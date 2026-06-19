package model

import "time"

// NorthboundMinute stores minute-level northbound cumulative flow.
type NorthboundMinute struct {
	TradeDate     time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	Time          string    `gorm:"primaryKey;size:5" json:"time"`
	HgtCumulative float64   `gorm:"type:numeric(12,4)" json:"hgtCumulative"`
	SgtCumulative float64   `gorm:"type:numeric(12,4)" json:"sgtCumulative"`
}

func (NorthboundMinute) TableName() string { return "northbound_minute" }

// NorthboundDailyView maps to northbound_daily_view (aggregated from minute data).
type NorthboundDailyView struct {
	TradeDate time.Time `gorm:"type:date" json:"tradeDate"`
	HgtNet    float64   `gorm:"type:numeric(12,4)" json:"hgtNet"`
	SgtNet    float64   `gorm:"type:numeric(12,4)" json:"sgtNet"`
	TotalNet  float64   `gorm:"type:numeric(12,4)" json:"totalNet"`
}

func (NorthboundDailyView) TableName() string { return "northbound_daily_view" }
