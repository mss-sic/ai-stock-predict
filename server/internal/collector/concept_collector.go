package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// CollectConcepts fetches all concept/industry boards from EastMoney and their constituent stocks
func CollectConcepts() error {
	log.Println("[概念采集] 开始采集东方财富概念板块数据")

	// Step 1: Fetch all concept boards
	boards, err := fetchConceptBoards()
	if err != nil {
		return fmt.Errorf("获取概念板块列表失败: %w", err)
	}
	log.Printf("[概念采集] 获取到 %d 个概念板块", len(boards))

	// Step 2: Fetch constituent stocks for each board
	totalStocks := 0
	for i, board := range boards {
		stocks, err := fetchBoardStocks(board.Code, board.Name, board.Type)
		if err != nil {
			log.Printf("[概念采集] %s(%s) 获取成分股失败: %v", board.Name, board.Code, err)
			continue
		}

		// Upsert concept board metadata
		db.PG.Where("concept_code = ?", board.Code).Assign(model.ConceptBoard{
			ConceptCode: board.Code,
			ConceptName: board.Name,
			ConceptType: board.Type,
			StockCount:  len(stocks),
			UpdatedAt:   time.Now(),
		}).FirstOrCreate(&model.ConceptBoard{})

		// Upsert stock-concept mappings in batches
		batchSize := 200
		for j := 0; j < len(stocks); j += batchSize {
			end := j + batchSize
			if end > len(stocks) {
				end = len(stocks)
			}
			batch := stocks[j:end]
			for _, sc := range batch {
				db.PG.Where("code = ? AND concept_code = ?", sc.Code, sc.ConceptCode).
					Assign(sc).FirstOrCreate(&model.StockConcept{})
			}
		}

		totalStocks += len(stocks)

		if (i+1)%50 == 0 {
			log.Printf("[概念采集] 进度: %d/%d, 已入库 %d 条关联", i+1, len(boards), totalStocks)
		}

		// Rate limit
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("[概念采集] 完成: %d 板块, %d 条股票-概念关联", len(boards), totalStocks)
	return nil
}

type conceptBoard struct {
	Code string
	Name string
	Type string // concept / industry
}

// fetchConceptBoards gets all concept and industry boards from EastMoney
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
		Diff []emBoardItem `json:"diff"`
		Total int          `json:"total"`
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

func fetchBoardStocks(boardCode, boardName, boardType string) ([]model.StockConcept, error) {
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

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
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

	return io.ReadAll(resp.Body)
}
