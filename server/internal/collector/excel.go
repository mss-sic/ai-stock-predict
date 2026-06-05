package collector

import (
	"fmt"
	"mime/multipart"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/xuri/excelize/v2"
)

type ImportResult struct {
	DatesImported  int
	StocksImported int
	Errors         []string
}

func ParseAndImportExcel(file multipart.File) (*ImportResult, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel: %w", err)
	}
	defer f.Close()

	result := &ImportResult{}

	// Try sheet2 first (the algorithm picks sheet)
	sheetName := "sheet2"
	rows, err := f.GetRows(sheetName)
	if err != nil {
		sheetName = "sheet1"
		rows, err = f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("no workable sheet found")
		}
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("empty sheet")
	}

	// Parse header (row 0): pairs of (date, empty) for each trading day
	header := rows[0]
	type daySlot struct {
		date time.Time
	}
	var days []daySlot
	for i := 0; i < len(header); i += 2 {
		val := strings.TrimPrefix(strings.TrimSpace(header[i]), "=")
		val = strings.Trim(val, `"`)
		t, err := time.Parse("20060102", val)
		if err != nil {
			continue
		}
		days = append(days, daySlot{date: t})
	}

	// Parse data rows: each row is a stock with (code, name) pairs per day
	// Row format: [code0, name0, code1, name1, ...]
	type dayPick struct {
		date time.Time
		code string
		name string
	}
	var allPicks []dayPick
	stockNames := map[string]string{}

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		cells := rows[rowIdx]
		for colIdx := 0; colIdx+1 < len(cells) && colIdx/2 < len(days); colIdx += 2 {
			code := strings.TrimPrefix(strings.TrimSpace(cells[colIdx]), "=")
			code = strings.Trim(code, `"`)
			name := ""
			if colIdx+1 < len(cells) {
				name = strings.TrimSpace(cells[colIdx+1])
			}
			if code == "" || len(code) < 6 {
				continue
			}
			stockNames[code] = name
			dayIdx := colIdx / 2
			if dayIdx < len(days) {
				allPicks = append(allPicks, dayPick{date: days[dayIdx].date, code: code, name: name})
			}
		}
	}

	if len(allPicks) == 0 {
		return nil, fmt.Errorf("no stock picks found in file")
	}

	// Group by date
	dateGroups := map[string][]dayPick{}
	for _, p := range allPicks {
		key := p.date.Format("2006-01-02")
		dateGroups[key] = append(dateGroups[key], p)
	}

	// Ensure stock basics exist
	for code, name := range stockNames {
		db.PG.Where("code = ?", code).FirstOrCreate(&model.StockBasic{
			Code: code,
			Name: name,
		})
	}

	// Import each date's picks
	for dateStr, picks := range dateGroups {
		pickDate, _ := time.Parse("2006-01-02", dateStr)

		// Sort by code for deterministic ranking
		sort.Slice(picks, func(i, j int) bool { return picks[i].code < picks[j].code })

		// Upsert algorithm_picks
		db.PG.Where("pick_date = ?", pickDate).Assign(model.AlgorithmPick{
			PickDate:    pickDate,
			TotalStocks: len(picks),
		}).FirstOrCreate(&model.AlgorithmPick{})

		// Upsert details
		for rank, p := range picks {
			detail := model.AlgorithmPickDetail{
				PickDate:  pickDate,
				StockCode: p.code,
				Rank:      rank + 1,
			}
			db.PG.Where("pick_date = ? AND stock_code = ?", pickDate, p.code).
				Assign(detail).FirstOrCreate(&detail)
		}
		result.DatesImported = len(dateGroups)
		result.StocksImported += len(picks)
	}

	return result, nil
}

func init() {
	_ = strconv.Itoa(0) // ensure import
}
