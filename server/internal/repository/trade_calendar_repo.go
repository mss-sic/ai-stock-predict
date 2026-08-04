package repository

import (
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// TradeCalendarRepo provides trading day queries.
type TradeCalendarRepo struct{}

// NewTradeCalendarRepo creates a new TradeCalendarRepo.
func NewTradeCalendarRepo() *TradeCalendarRepo {
	return &TradeCalendarRepo{}
}

// LatestTradeDate returns the most recent trading day on or before today.
func (r *TradeCalendarRepo) LatestTradeDate() (time.Time, error) {
	var t time.Time
	err := db.PG.Model(&model.TradeCalendar{}).
		Where("is_trading_day = true AND trade_date <= CURRENT_DATE").
		Order("trade_date DESC").
		Limit(1).
		Pluck("trade_date", &t).Error
	return t, err
}

// PrevTradeDate returns the previous trading day before the given date.
func (r *TradeCalendarRepo) PrevTradeDate(before time.Time) (time.Time, error) {
	var t time.Time
	err := db.PG.Model(&model.TradeCalendar{}).
		Where("is_trading_day = true AND trade_date < ?", before).
		Order("trade_date DESC").
		Limit(1).
		Pluck("trade_date", &t).Error
	return t, err
}

// NextTradeDate returns the next trading day after the given date.
func (r *TradeCalendarRepo) NextTradeDate(after time.Time) (time.Time, error) {
	var t time.Time
	err := db.PG.Model(&model.TradeCalendar{}).
		Where("is_trading_day = true AND trade_date > ?", after).
		Order("trade_date ASC").
		Limit(1).
		Pluck("trade_date", &t).Error
	return t, err
}

// IsTradeDay checks whether a given date is a trading day.
func (r *TradeCalendarRepo) IsTradeDay(date time.Time) (bool, error) {
	var count int64
	err := db.PG.Model(&model.TradeCalendar{}).
		Where("trade_date = ? AND is_trading_day = true", date).
		Count(&count).Error
	return count > 0, err
}

// TradeDaysBetween returns all trading days in [start, end] (inclusive).
func (r *TradeCalendarRepo) TradeDaysBetween(start, end time.Time) ([]time.Time, error) {
	var dates []time.Time
	err := db.PG.Model(&model.TradeCalendar{}).
		Where("is_trading_day = true AND trade_date >= ? AND trade_date <= ?", start, end).
		Order("trade_date ASC").
		Pluck("trade_date", &dates).Error
	return dates, err
}

// CountTradeDays returns the number of trading days in [start, end].
func (r *TradeCalendarRepo) CountTradeDays(start, end time.Time) (int64, error) {
	var count int64
	err := db.PG.Model(&model.TradeCalendar{}).
		Where("is_trading_day = true AND trade_date >= ? AND trade_date <= ?", start, end).
		Count(&count).Error
	return count, err
}
