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
func GetLimitStatsHistory(days int) ([]LimitStats, error) {
	sql := `
		WITH daily_ret AS (
			SELECT k.trade_date::text, k.code, b.board_type, COALESCE(b.is_st, false) as is_st,
				(k.close - kp.close) / NULLIF(kp.close, 0) as ret,
				-- Estimate board break: hit limit-up but closed far from limit
				CASE
					WHEN b.board_type IN ('kc','cy') AND (k.close - kp.close) / NULLIF(kp.close, 0) >= 0.1999
						AND (k.high - k.close) / NULLIF(k.high, 0) > 0.02 THEN 1
					WHEN b.board_type = 'bj' AND (k.close - kp.close) / NULLIF(kp.close, 0) >= 0.2999
						AND (k.high - k.close) / NULLIF(k.high, 0) > 0.02 THEN 1
					WHEN is_st AND (k.close - kp.close) / NULLIF(kp.close, 0) >= 0.0499
						AND (k.high - k.close) / NULLIF(k.high, 0) > 0.02 THEN 1
					WHEN (k.close - kp.close) / NULLIF(kp.close, 0) >= 0.0999
						AND (k.high - k.close) / NULLIF(k.high, 0) > 0.02 THEN 1
					ELSE 0
				END as board_break
			FROM stocks_daily_k k
			JOIN LATERAL (
				SELECT kp2.close FROM stocks_daily_k kp2
				WHERE kp2.code = k.code AND kp2.trade_date < k.trade_date
				ORDER BY kp2.trade_date DESC LIMIT 1
			) kp ON TRUE
			JOIN stocks_basic b ON b.code = k.code
			WHERE k.trade_date >= (
				SELECT MAX(trade_date) FROM market_daily_agg
			) - ?::integer
			  AND k.close > 0 AND kp.close > 0
		)
		SELECT trade_date,
			COUNT(*) FILTER (
				WHERE (board_type IN ('kc','cy') AND ret >= 0.1999)
				   OR (board_type = 'bj' AND ret >= 0.2999)
				   OR (is_st AND ret >= 0.0499)
				   OR (ret >= 0.0999)
			) as up_count,
			COUNT(*) FILTER (
				WHERE (board_type IN ('kc','cy') AND ret <= -0.1999)
				   OR (board_type = 'bj' AND ret <= -0.2999)
				   OR (is_st AND ret <= -0.0499)
				   OR (ret <= -0.0999)
			) as down_count,
			COUNT(*) FILTER (WHERE ret > 0) as rise_count,
			COUNT(*) FILTER (WHERE ret < 0) as fall_count,
			SUM(board_break) as board_break,
			0 as max_streak,
			COUNT(*) as total_stocks
		FROM daily_ret
		GROUP BY trade_date
		ORDER BY trade_date ASC
	`

	var list []LimitStats
	if err := db.PG.Raw(sql, days).Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
