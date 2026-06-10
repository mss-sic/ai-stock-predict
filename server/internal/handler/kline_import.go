package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm/clause"
)

type klineImportResult struct {
	FileName    string   `json:"fileName"`
	TotalRows   int      `json:"totalRows"`
	Imported    int      `json:"imported"`
	Skipped     int      `json:"skipped"`
	Errors      []string `json:"errors"`
	TradeDate   string   `json:"tradeDate"`
}

func parseKlineCSV(f multipart.File, fileName string) (*klineImportResult, error) {
	result := &klineImportResult{FileName: fileName}

	// Decode GBK
	reader := transform.NewReader(f, simplifiedchinese.GBK.NewDecoder())
	csvReader := csv.NewReader(reader)
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1 // allow variable fields

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("读取CSV头部失败: %v", err)
	}

	// Build column index map
	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.TrimSpace(col)] = i
	}

	// Required columns
	requiredCols := []string{"股票代码", "交易日期", "开盘价", "最高价", "最低价", "收盘价", "成交量", "成交额", "换手率"}
	for _, col := range requiredCols {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("缺少必要列: %s", col)
		}
	}

	// Read all rows into batches
	batchSize := 500
	batch := make([]model.StockDailyK, 0, batchSize)
	var tradeDate string

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("行读取错误: %v", err))
			result.Skipped++
			continue
		}

		record, rowDate, rowErr := parseKlineRow(row, colIdx)
		if rowErr != "" {
			result.Errors = append(result.Errors, rowErr)
			result.Skipped++
			continue
		}

		if tradeDate == "" {
			tradeDate = rowDate
		}

		batch = append(batch, record)
		result.TotalRows++

		if len(batch) >= batchSize {
			if err := upsertKlineBatch(batch); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("批量写入失败: %v", err))
				result.Skipped += len(batch)
			} else {
				result.Imported += len(batch)
			}
			batch = batch[:0]
		}
	}

	// Flush remaining
	if len(batch) > 0 {
		if err := upsertKlineBatch(batch); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("批量写入失败: %v", err))
			result.Skipped += len(batch)
		} else {
			result.Imported += len(batch)
		}
	}

	result.TradeDate = tradeDate

	// Record import log in MySQL
	status := "success"
	if result.Skipped > 0 {
		status = "partial"
	}
	if result.Imported == 0 {
		status = "failed"
	}
	db.MySQL.Create(&model.ImportLog{
		FileName:     fileName,
		RowsImported: result.Imported,
		Status:       status,
		ImportedAt:   time.Now(),
	})

	return result, nil
}

func parseKlineRow(row []string, colIdx map[string]int) (model.StockDailyK, string, string) {
	var r model.StockDailyK

	getStr := func(col string) string {
		if idx, ok := colIdx[col]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}
	getFloat := func(col string) (float64, bool) {
		s := getStr(col)
		if s == "" || s == "-" || s == "--" {
			return 0, false
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	getInt64 := func(col string) (int64, bool) {
		s := getStr(col)
		if s == "" || s == "-" || s == "--" {
			return 0, false
		}
		// Handle float-like volume (e.g., "761836.0")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f)), true
	}

	code := getStr("股票代码")
	// Strip exchange prefix (sh/sz/bj)
	code = strings.TrimPrefix(code, "sh")
	code = strings.TrimPrefix(code, "sz")
	code = strings.TrimPrefix(code, "bj")
	if code == "" {
		return r, "", fmt.Sprintf("缺少股票代码: %v", row)
	}

	dateStr := getStr("交易日期")
	if dateStr == "" {
		return r, "", fmt.Sprintf("股票%s: 缺少交易日期", code)
	}
	tradeDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return r, "", fmt.Sprintf("股票%s: 日期格式错误: %s", code, dateStr)
	}

	open, _ := getFloat("开盘价")
	high, _ := getFloat("最高价")
	low, _ := getFloat("最低价")
	close_, _ := getFloat("收盘价")
	volume, _ := getInt64("成交量")
	amount, _ := getFloat("成交额")
	turnover, _ := getFloat("换手率")

	r.Code = code
	r.TradeDate = tradeDate
	r.Open = open
	r.High = high
	r.Low = low
	r.Close = close_
	r.Volume = volume
	r.Amount = amount
	r.TurnoverRate = turnover

	return r, dateStr, ""
}

func upsertKlineBatch(batch []model.StockDailyK) error {
	if db.PG == nil {
		return fmt.Errorf("PostgreSQL 未连接")
	}
	return db.PG.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "amount", "turnover_rate"}),
	}).CreateInBatches(batch, 500).Error
}
