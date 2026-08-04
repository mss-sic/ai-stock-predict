package model

import "time"

// StockDailyIndicators — JSONB 指标缓存表，每日盘后预计算全部 84 个指标值。
// Hot columns 用于跨股筛选/排序/聚合；其余指标存储于 JSONB indicators 列。
type StockDailyIndicators struct {
	Code           string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate      time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	DailyChange    float64   `gorm:"type:numeric(8,4)" json:"dailyChange"`
	PE             float64   `gorm:"type:numeric(12,4)" json:"pe"`
	PB             float64   `gorm:"type:numeric(12,4)" json:"pb"`
	RSI            float64   `gorm:"type:numeric(8,2)" json:"rsi"`
	VolumeRatio    float64   `gorm:"type:numeric(8,4)" json:"volumeRatio"`
	TurnoverRate   float64   `gorm:"type:numeric(8,4)" json:"turnoverRate"`
	TotalMarketCap float64   `gorm:"type:numeric(20,2)" json:"totalMarketCap"`
	AlgoScore      float64   `gorm:"type:numeric(8,2)" json:"algoScore"`
	Indicators     string    `gorm:"type:jsonb;default:'{}'" json:"indicators"`
	AdjFactor      float64   `gorm:"type:numeric(12,8);default:1.0" json:"adjFactor"`
	DataQuality    string    `gorm:"type:varchar(10);default:'ok'" json:"dataQuality"`
	ComputedAt     time.Time `json:"computedAt"`
}

func (StockDailyIndicators) TableName() string { return "stock_daily_indicators" }
