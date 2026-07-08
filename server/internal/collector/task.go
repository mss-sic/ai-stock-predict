package collector

import (
	"fmt"
	"log"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type TaskResult struct {
	Total   int
	Success int
	Failed  int
	Errors  []string
}

// RunDailyKTask fetches daily K-line for all tracked stocks
func RunDailyKTask() *TaskResult {
	result := &TaskResult{}
	var stocks []model.StockBasic
	if err := db.PG.Find(&stocks).Error; err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	result.Total = len(stocks)

	for _, stock := range stocks {
		log.Printf("[collector] fetching kline for %s %s", stock.Code, stock.Name)
		klines, err := FetchKLine(stock.Code, 365)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", stock.Code, err))
			continue
		}

		for _, k := range klines {
			tradeDate, parseErr := time.Parse("2006-01-02", k.Date)
			if parseErr != nil { continue }
			record := model.StockDailyK{
				Code:      stock.Code,
				TradeDate: tradeDate,
				Open:      k.Open,
				High:      k.High,
				Low:       k.Low,
				Close:     k.Close,
				Volume:    k.Volume,
				Amount:    k.Close * float64(k.Volume) / 100,
				UpdatedAt: time.Now(),
			}
			db.PG.Where("code = ? AND trade_date = ?", stock.Code, tradeDate).
				Assign(record).FirstOrCreate(&record)
		}
		result.Success++
		time.Sleep(200 * time.Millisecond) // rate limit
	}
	return result
}

// RunBasicTask updates stock basic info
func RunBasicTask() *TaskResult {
	result := &TaskResult{}
	var stocks []model.StockBasic
	if err := db.PG.Find(&stocks).Error; err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	result.Total = len(stocks)

	for _, stock := range stocks {
		basic, err := FetchBasic(stock.Code)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", stock.Code, err))
			continue
		}
		stock.Name = basic.Name
		stock.UpdatedAt = time.Now()
		db.PG.Save(&stock)
		result.Success++
		time.Sleep(1200 * time.Millisecond) // 东财限流
	}
	return result
}
