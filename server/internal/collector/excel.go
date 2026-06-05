package collector

import (
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/xuri/excelize/v2"
)

type ImportResult struct {
	FileName       string   `json:"fileName"`
	DatesImported  int      `json:"datesImported"`
	PicksImported  int      `json:"picksImported"`
	SignalsImported int     `json:"signalsImported"`
	StocksCreated  int      `json:"stocksCreated"`
	Errors         []string `json:"errors"`
	Previews       []string `json:"previews"`
}

func ParseAndImportExcel(file multipart.File, fileName string) (*ImportResult, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("无法打开 Excel 文件: %w", err)
	}
	defer f.Close()

	result := &ImportResult{FileName: fileName}

	// ── Parse sheet2: algorithm picks ──
	if err := importSheet2(f, result); err != nil {
		result.Errors = append(result.Errors, "sheet2(榜单): "+err.Error())
	}

	// ── Parse sheet1: stock signals ──
	if err := importSheet1(f, result); err != nil {
		result.Errors = append(result.Errors, "sheet1(信号): "+err.Error())
	}

	// ── Log to MySQL ──
	if db.MySQL != nil {
		status := "success"
		if len(result.Errors) > 0 {
			status = "partial"
		}
		db.MySQL.Create(&model.ImportLog{
			FileName:     fileName,
			RowsImported: result.PicksImported + result.SignalsImported,
			Status:       status,
			ImportedAt:   time.Now(),
		})
	}

	return result, nil
}

// ── importSheet2 parses the algorithm picks (50 stocks × N trading days) ──
func importSheet2(f *excelize.File, result *ImportResult) error {
	rows, err := f.GetRows("sheet2")
	if err != nil {
		return fmt.Errorf("找不到 sheet2: %w", err)
	}
	if len(rows) < 2 {
		return fmt.Errorf("sheet2 数据为空")
	}

	// Row 0: date headers (every other column has a date: 20260603, 20260602, ...)
	header := rows[0]
	type daySlot struct {
		date time.Time
	}
	var days []daySlot
	for i := 0; i < len(header); i += 2 {
		val := strings.TrimPrefix(strings.TrimSpace(header[i]), "'")
		val = strings.Trim(val, `"`)
		t, err := time.Parse("20060102", val)
		if err != nil {
			continue
		}
		days = append(days, daySlot{date: t})
	}
	if len(days) == 0 {
		return fmt.Errorf("sheet2 未找到有效日期")
	}

	// Parse data rows: row 1 = rank 1, row 2 = rank 2, etc.
	// Each row: [code0, name0, code1, name1, ...]
	type dayPick struct {
		date time.Time
		code string
		name string
		rank int
	}
	var allPicks []dayPick
	stockNames := map[string]string{}

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		cells := rows[rowIdx]
		rank := rowIdx // row 1 = rank 1
		for colIdx := 0; colIdx+1 < len(cells) && colIdx/2 < len(days); colIdx += 2 {
			code := strings.TrimSpace(cells[colIdx])
			code = strings.TrimPrefix(code, "'")
			code = strings.Trim(code, `"`)
			if code == "" || len(code) < 6 {
				continue
			}
			name := ""
			if colIdx+1 < len(cells) {
				name = strings.TrimSpace(cells[colIdx+1])
			}
			stockNames[code] = name
			dayIdx := colIdx / 2
			if dayIdx < len(days) {
				allPicks = append(allPicks, dayPick{date: days[dayIdx].date, code: code, name: name, rank: rank})
			}
		}
	}

	if len(allPicks) == 0 {
		return fmt.Errorf("sheet2 未找到上榜数据")
	}

	// Ensure stock basics exist for all codes
	for code, name := range stockNames {
		res := db.PG.Where("code = ?", code).FirstOrCreate(&model.StockBasic{
			Code: code,
			Name: name,
		})
		if res.RowsAffected > 0 {
			result.StocksCreated++
		}
	}

	// Group picks by date
	type dateGroup struct {
		date  time.Time
		picks []dayPick
	}
	dateMap := map[string]*dateGroup{}
	for _, p := range allPicks {
		key := p.date.Format("2006-01-02")
		if dateMap[key] == nil {
			dateMap[key] = &dateGroup{date: p.date}
		}
		dateMap[key].picks = append(dateMap[key].picks, p)
	}

	// Import each date's picks
	for _, dg := range dateMap {
		// Upsert algorithm_picks header
		db.PG.Where("pick_date = ?", dg.date).Assign(model.AlgorithmPick{
			PickDate:    dg.date,
			TotalStocks: len(dg.picks),
			GeneratedAt: time.Now(),
		}).FirstOrCreate(&model.AlgorithmPick{})

		// Upsert detail rows with correct rank
		for _, p := range dg.picks {
			detail := model.AlgorithmPickDetail{
				PickDate:  p.date,
				StockCode: p.code,
				Rank:      p.rank,
				Suggestion: "hold",
				RiskLevel:  "medium",
			}
			db.PG.Where("pick_date = ? AND stock_code = ?", p.date, p.code).
				Assign(detail).FirstOrCreate(&detail)
		}
	}

	result.DatesImported = len(dateMap)
	result.PicksImported = len(allPicks)
	result.Previews = append(result.Previews,
		fmt.Sprintf("榜单: %d 个交易日, %d 条上榜记录, %d 只个股", len(dateMap), len(allPicks), len(stockNames)))

	return nil
}

// ── importSheet1 parses stock signals (code + signal value) ──
func importSheet1(f *excelize.File, result *ImportResult) error {
	rows, err := f.GetRows("sheet1")
	if err != nil {
		return fmt.Errorf("找不到 sheet1: %w", err)
	}
	if len(rows) < 2 {
		return fmt.Errorf("sheet1 数据为空")
	}

	count := 0
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		code := strings.TrimSpace(row[0])
		code = strings.TrimPrefix(code, "'")
		if code == "" || len(code) < 6 {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err != nil {
			continue
		}

		db.PG.Where("code = ?", code).Assign(model.StockSignal{
			Code:        code,
			SignalValue: val,
			Source:      "excel_import",
			UpdatedAt:   time.Now(),
		}).FirstOrCreate(&model.StockSignal{Code: code})
		count++
	}

	result.SignalsImported = count
	result.Previews = append(result.Previews,
		fmt.Sprintf("信号: %d 只个股信号值已导入", count))
	return nil
}

func init() {
	_ = strconv.Itoa(0) // ensure import
}
