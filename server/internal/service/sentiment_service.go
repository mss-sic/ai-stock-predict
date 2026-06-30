package service

import (
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// GetLatestSentiment returns the most recent market sentiment record.
func GetLatestSentiment() (*model.MarketSentiment, error) {
	var s model.MarketSentiment
	err := db.PG.Order("trade_date DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSentimentHistory returns sentiment records for the last N trading days.
func GetSentimentHistory(days int) ([]model.MarketSentiment, error) {
	var list []model.MarketSentiment
	err := db.PG.Order("trade_date DESC").Limit(days).Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// GetSentimentByDate returns sentiment for a specific date.
func GetSentimentByDate(date time.Time) (*model.MarketSentiment, error) {
	var s model.MarketSentiment
	err := db.PG.Where("trade_date = ?", date.Format("2006-01-02")).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSentimentDateRange returns sentiment records between start and end dates.
func GetSentimentDateRange(start, end time.Time) ([]model.MarketSentiment, error) {
	var list []model.MarketSentiment
	err := db.PG.Where("trade_date >= ? AND trade_date <= ?",
		start.Format("2006-01-02"), end.Format("2006-01-02")).
		Order("trade_date ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetNorthboundMinuteHistory returns recent minute-level northbound data.
func GetNorthboundMinuteHistory(days int) ([]model.NorthboundMinute, error) {
	var list []model.NorthboundMinute
	err := db.PG.Where("trade_date >= CURRENT_DATE - ?::integer", days).
		Order("trade_date DESC, time DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetNorthboundHistory returns recent daily northbound flow from the view.
func GetNorthboundHistory(days int) ([]model.NorthboundDailyView, error) {
	var list []model.NorthboundDailyView
	err := db.PG.Table("northbound_daily_view").Order("trade_date DESC").Limit(days).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// LimitStats represents daily limit-up/down statistics.
type LimitStats struct {
	TradeDate    string  `gorm:"column:trade_date" json:"tradeDate"`
	UpCount      int     `gorm:"column:up_count" json:"upCount"`
	DownCount    int     `gorm:"column:down_count" json:"downCount"`
	RiseCount    int     `gorm:"column:rise_count" json:"riseCount"`
	FallCount    int     `gorm:"column:fall_count" json:"fallCount"`
	BoardBreak   int     `gorm:"column:board_break" json:"boardBreak"`
	MaxStreak    int     `gorm:"column:max_streak" json:"maxStreak"`
	TotalStocks  int     `gorm:"column:total_stocks" json:"totalStocks"`
}

// GetLimitStatsHistory returns daily limit-up/down statistics for the last N days.
// Reads from pre-computed limit_stats_daily table (populated by collect_limit_stats.py).
func GetLimitStatsHistory(days int) ([]LimitStats, error) {
	var list []LimitStats
	err := db.PG.Raw(`
		SELECT trade_date::text, up_count, down_count, rise_count, fall_count,
		       board_break, 0 as max_streak, total_stocks
		FROM limit_stats_daily
		ORDER BY trade_date DESC
		LIMIT ?
	`, days).Scan(&list).Error
	if err != nil {
		return nil, err
	}
	// Reverse to ascending for chart display
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}
