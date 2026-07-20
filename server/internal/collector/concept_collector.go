package collector

import (
	"crypto/tls"

	"golang.org/x/text/encoding/simplifiedchinese"
	"strconv"
	"strings"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm/clause"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// CollectConcepts fetches concept/industry boards from 新浪 (primary) or EastMoney (fallback).
func CollectConcepts() error {
	log.Println("[概念采集] 开始采集概念板块数据")

	// Check if full rebuild was done today — skip incremental to avoid data regression
	var lastFull time.Time
	if err := db.PG.Raw("SELECT MAX(updated_at) FROM concept_boards WHERE concept_type='concept' AND updated_at > NOW() - INTERVAL '1 day'").Scan(&lastFull).Error; err == nil && !lastFull.IsZero() {
		var totalRows int64
		db.PG.Model(&model.StockConcept{}).Where("concept_type = ?", "concept").Count(&totalRows)
		if totalRows > 50000 {
			log.Printf("[概念采集] 检测到近期全量重建 (%d 条关联), 跳过增量采集避免数据退化", totalRows)
			return nil
		}
	}

	// Try 新浪 first (more reliable from China networks)
	boards, err := fetchSinaConceptBoards()
	if err != nil || len(boards) == 0 {
		if err != nil {
			log.Printf("[概念采集] 新浪API不可用: %v, 尝试东方财富...", err)
		}
		// Try EastMoney
		boards, err = fetchEMConceptBoards()
		if err != nil || len(boards) == 0 {
			if err != nil {
				log.Printf("[概念采集] 东方财富API也不可用: %v", err)
			}
			log.Println("[概念采集] 所有远程API失败，使用stocks_basic行业种子")
			return seedFromStocksBasic()
		}
	}

	log.Printf("[概念采集] 获取到 %d 个概念板块", len(boards))

	// Fetch constituent stocks for each board
	totalStocks := 0
	for i, board := range boards {
		var stocks []model.StockConcept
		var stockErr error

		// Try 新浪 for constituent stocks first
		stocks, stockErr = fetchSinaBoardStocks(board.Code, board.Name, board.Type)
		if stockErr != nil || len(stocks) == 0 {
			// Fallback to EastMoney
			if board.Type == "concept" {
				stocks, stockErr = fetchEMBoardStocks(board.Code, board.Name, board.Type)
			}
			if stockErr != nil {
				log.Printf("[概念采集] %s(%s) 成分股获取失败: %v", board.Name, board.Code, stockErr)
			}
		}

		// Upsert board metadata
		bcount := len(stocks)
		if bcount == 0 && board.StockCount > 0 {
			bcount = board.StockCount
		}
		if err := db.PG.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "concept_code"}},
			DoUpdates: clause.AssignmentColumns([]string{"concept_name", "concept_type", "stock_count", "updated_at"}),
		}).Create(&model.ConceptBoard{
			ConceptCode: board.Code,
			ConceptName: board.Name,
			ConceptType: board.Type,
			StockCount:  bcount,
			UpdatedAt:   time.Now(),
		}).Error; err != nil {
			log.Printf("[概念采集] 板块%s 入库失败: %v", board.Code, err)
		}

		// Upsert stock mappings
		if len(stocks) > 0 {
			if err := db.PG.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "code"}, {Name: "concept_code"}},
				DoUpdates: clause.AssignmentColumns([]string{"concept_name", "concept_type", "stock_name", "updated_at"}),
			}).CreateInBatches(stocks, 500).Error; err != nil {
				log.Printf("[概念采集] 板块%s 成分股写入失败: %v", board.Code, err)
			}
		}

		totalStocks += len(stocks)

		if (i+1)%50 == 0 {
			log.Printf("[概念采集] 进度: %d/%d, 已入库 %d 条关联", i+1, len(boards), totalStocks)
		}
		time.Sleep(150 * time.Millisecond)
	}

	log.Printf("[概念采集] 完成: %d 板块, %d 条股票-概念关联", len(boards), totalStocks)
	return nil
}

type conceptBoard struct {
	Code       string
	Name       string
	Type       string // concept / industry
	StockCount int    // known count from API metadata
}

// ── 新浪 concept board source ──

// fetchSinaConceptBoards fetches concept boards from 新浪 financial data.
func fetchSinaConceptBoards() ([]conceptBoard, error) {
	url := "http://money.finance.sina.com.cn/q/view/newFLJK.php?param=class"
	body, err := httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("新浪概念列表请求失败: %w", err)
	}

	// Extract JSON from JS variable: var S_Finance_bankuai_class = {...}
	start := strings.Index(string(body), "S_Finance_bankuai_class")
	if start < 0 {
		return nil, fmt.Errorf("未找到概念数据")
	}
	jsonStart := strings.Index(string(body)[start:], "{")
	if jsonStart < 0 {
		return nil, fmt.Errorf("概念JSON起始未找到")
	}
	// Find matching closing brace (depth-based)
	braceDepth := 0
	jsonEnd := -1
	searchStart := start + jsonStart
	for i := searchStart; i < len(body); i++ {
		if body[i] == '{' {
			braceDepth++
		} else if body[i] == '}' {
			braceDepth--
			if braceDepth == 0 {
				jsonEnd = i
				break
			}
		}
	}
	if jsonEnd < 0 {
		return nil, fmt.Errorf("概念JSON结束未找到")
	}
	jsonStr := string(body)[searchStart : jsonEnd+1]

	var data map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("概念JSON解析失败: %w", err)
	}

	boards := make([]conceptBoard, 0, len(data))
	for key, val := range data {
		parts := strings.Split(val, ",")
		if len(parts) < 3 {
			continue
		}
		count := 0
		if n, err := strconv.Atoi(parts[2]); err == nil {
			count = n
		}
		btype := "concept"
		if strings.HasPrefix(key, "hy_") {
			btype = "industry"
		}
		boards = append(boards, conceptBoard{
			Code:       parts[0],
			Name:       parts[1],
			Type:       btype,
			StockCount: count,
		})
	}
	return boards, nil
}

// fetchSinaBoardStocks fetches constituent stocks for a concept board from 新浪.
func fetchSinaBoardStocks(boardCode, boardName, boardType string) ([]model.StockConcept, error) {
	url := fmt.Sprintf("http://money.finance.sina.com.cn/q/view/newFLJK.php?param=%s", boardCode)
	body, err := httpGet(url)
	if err != nil {
		return nil, err
	}

	// Parse stocks from JS variable
	text := string(body)
	var raw map[string]string
	// Try to find JSON object after S_Finance_bankuai_XXX =
	searchKey := fmt.Sprintf("S_Finance_bankuai_%s", boardCode)
	start := strings.Index(text, searchKey)
	if start < 0 {
		return nil, fmt.Errorf("未找到板块%s的成分股数据", boardCode)
	}
	jsonStart := strings.Index(text[start:], "{")
	if jsonStart < 0 {
		return nil, nil // empty board
	}
	// Find matching closing brace
	braceStart := start + jsonStart
	depth := 0
	braceEnd := -1
	for i := braceStart; i < len(text); i++ {
		if text[i] == '{' {
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 {
				braceEnd = i
				break
			}
		}
	}
	if braceEnd < 0 {
		return nil, fmt.Errorf("成分股JSON提取失败")
	}
	jsonStr := text[braceStart : braceEnd+1]

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("成分股JSON解析: %w", err)
	}

	// 新浪 board detail returns sub-board metadata, not constituent stocks.
	// Board metadata (count) is already known from the list API.
	// Constituent stocks will be filled via EastMoney fallback or future enhancement.
	return nil, nil
}

// ── EastMoney concept board source (kept as fallback) ──

// fetchEMConceptBoards gets all concept and industry boards from EastMoney.
func fetchEMConceptBoards() ([]conceptBoard, error) {
	return fetchConceptBoards()
}


// fetchConceptBoards gets all concept and industry boards from EastMoney (legacy, used as fallback)
func fetchConceptBoards() ([]conceptBoard, error) {
	var allBoards []conceptBoard

	// Industry boards (m:90+t:2)
	industryBoards, err := fetchBoardList("m:90+t:2", "industry")
	if err != nil {
		log.Printf("[概念采集] 行业板块获取失败: %v", err)
	} else {
		allBoards = append(allBoards, industryBoards...)
	}

	// Concept boards (m:90+t:3)
	conceptBoards, err := fetchBoardList("m:90+t:3", "concept")
	if err != nil {
		log.Printf("[概念采集] 概念板块获取失败: %v", err)
	} else {
		allBoards = append(allBoards, conceptBoards...)
	}

	return allBoards, nil
}

type emBoardItem struct {
	Code string `json:"f12"`
	Name string `json:"f14"`
}

type emBoardResp struct {
	Data struct {
		Diff  []emBoardItem `json:"diff"`
		Total int           `json:"total"`
	} `json:"data"`
}

func fetchBoardList(fs string, boardType string) ([]conceptBoard, error) {
	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?fs=%s&fid=f3&po=1&pz=500&np=1&fltt=2&invt=2&fields=f12,f14",
		fs,
	)

	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}

	var result emBoardResp
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("解析板块列表失败: %w", err)
	}

	boards := make([]conceptBoard, 0, len(result.Data.Diff))
	for _, item := range result.Data.Diff {
		if item.Code != "" && item.Name != "" {
			boards = append(boards, conceptBoard{Code: item.Code, Name: item.Name, Type: boardType})
		}
	}

	return boards, nil
}

type emStockItem struct {
	Code   string  `json:"f12"`
	Name   string  `json:"f14"`
	Price  float64 `json:"f2"`
	ChgPct float64 `json:"f3"`
}

type emStockResp struct {
	Data struct {
		Diff  []emStockItem `json:"diff"`
		Total int           `json:"total"`
	} `json:"data"`
}

func fetchEMBoardStocks(boardCode, boardName, boardType string) ([]model.StockConcept, error) {
	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?fs=b:%s&fid=f3&po=1&pz=300&np=1&fltt=2&invt=2&fields=f12,f14,f2,f3",
		boardCode,
	)

	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}

	var result emStockResp
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("解析成分股失败: %w", err)
	}

	now := time.Now()
	stocks := make([]model.StockConcept, 0, len(result.Data.Diff))
	for _, item := range result.Data.Diff {
		if item.Code == "" {
			continue
		}
		// EastMoney codes include market prefix (1.600519), strip it
		code := item.Code
		if len(code) > 6 && (code[0] == '0' || code[0] == '1') && code[1] == '.' {
			code = code[2:]
		}
		stocks = append(stocks, model.StockConcept{
			Code:        code,
			ConceptCode: boardCode,
			ConceptName: boardName,
			ConceptType: boardType,
			StockName:   item.Name,
			UpdatedAt:   now,
		})
	}

	return stocks, nil
}

// seedFromStocksBasic seeds industry data from stocks_basic.sw_l1/sw_l2
// when the external API is unavailable.
func seedFromStocksBasic() error {
	log.Println("[概念采集] 外部API不可用，从stocks_basic行业字段种子化...")

	type industryRow struct {
		Industry string
		Cnt      int
	}

	var industries []industryRow
	if err := db.PG.Raw(
		"SELECT industry, count(*) as cnt FROM stocks_basic WHERE industry != '' GROUP BY industry ORDER BY cnt DESC",
	).Scan(&industries).Error; err != nil {
		return fmt.Errorf("查询stocks_basic行业分布失败: %w", err)
	}

	log.Printf("[概念采集] 发现 %d 个行业", len(industries))

	totalStocks := 0
	for _, ind := range industries {
		code := strings.TrimPrefix(ind.Industry, "IND_")
		// Upsert concept board
		if err := db.PG.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "concept_code"}},
			DoUpdates: clause.AssignmentColumns([]string{"concept_name", "concept_type", "stock_count", "updated_at"}),
		}).Create(&model.ConceptBoard{
			ConceptCode: code,
			ConceptName: code,
			ConceptType: "industry",
			StockCount:  ind.Cnt,
			UpdatedAt:   time.Now(),
		}).Error; err != nil {
			log.Printf("[概念采集] 行业板块 %s 入库失败: %v", code, err)
			continue
		}

		// Upsert stock-concept mappings
		var stocks []struct {
			Code string
			Name string
		}
		if err := db.PG.Raw(
			"SELECT code, name FROM stocks_basic WHERE industry = ?",
			code,
		).Scan(&stocks).Error; err != nil {
			log.Printf("[概念采集] 行业 %s 查询股票失败: %v", code, err)
			continue
		}

		// Batch upsert stock-concept mappings
		scs := make([]model.StockConcept, 0, len(stocks))
		for _, s := range stocks {
			scs = append(scs, model.StockConcept{
				Code:        s.Code,
				ConceptCode: code,
				ConceptName: code,
				ConceptType: "industry",
				StockName:   s.Name,
				UpdatedAt:   time.Now(),
			})
		}
		if len(scs) > 0 {
			if err := db.PG.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "code"}, {Name: "concept_code"}},
				DoUpdates: clause.AssignmentColumns([]string{"concept_name", "concept_type", "stock_name", "updated_at"}),
			}).CreateInBatches(scs, 500).Error; err != nil {
				log.Printf("[概念采集] 行业%s 批量写入失败: %v", code, err)
			}
		}
		totalStocks += len(stocks)
	}

	log.Printf("[概念采集] 完成(种子): %d 板块, %d 条股票-行业关联", len(industries), totalStocks)
	return nil
}

func httpGet(url string) ([]byte, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: tr}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Convert GBK to UTF-8 for 新浪 responses (EastMoney uses UTF-8 already)
	if strings.Contains(url, "sina.com.cn") || strings.Contains(url, "sinajs") {
		utf8Body, err := simplifiedchinese.GBK.NewDecoder().Bytes(body)
		if err == nil {
			return utf8Body, nil
		}
		// If conversion fails, return raw (might already be UTF-8)
	}

	return body, nil
}


// CollectConceptsFull runs a full rebuild using the Python rebuild_concepts.py script.
// This mirrors scripts/collector/rebuild_concepts.py logic: stock-centric via 东财 slist API.
// It DELETEs all concept-type data first, then rebuilds from scratch.
// This should be used for periodic full refresh (weekly/monthly), not daily.
func CollectConceptsFull() error {
	log.Println("[概念采集] 开始全量重建概念板块数据 (stock-centric)")
	
	var totalStocks int64
	db.PG.Model(&model.StockBasic{}).Count(&totalStocks)
	log.Printf("[概念采集] 全量重建: %d 只股票, 预计耗时 %d 分钟", totalStocks, totalStocks*400/1000/60)
	
	// Run the proven Python rebuild script
	err := runPythonStreamWithArgs("rebuild_concepts.py")
	if err != nil {
		log.Printf("[概念采集] 全量重建失败: %v", err)
		return err
	}
	
	var afterCount int64
	db.PG.Model(&model.StockConcept{}).Where("concept_type = ?", "concept").Count(&afterCount)
	log.Printf("[概念采集] 全量重建完成: %d 条概念关联", afterCount)
	return nil
}
