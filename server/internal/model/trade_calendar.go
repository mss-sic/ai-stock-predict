package model

import "time"

// TradeCalendar stores trading day information for efficient lookup.
// Replaces expensive SELECT MAX(trade_date) FROM stocks_daily_k queries.
type TradeCalendar struct {
	TradeDate    time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	IsTradingDay bool      `gorm:"default:true" json:"isTradingDay"`
	HolidayName  string    `gorm:"type:varchar(50)" json:"holidayName"`
	DataSource   string    `gorm:"type:varchar(20);default:'tushare'" json:"dataSource"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (TradeCalendar) TableName() string { return "trade_calendar" }
