package model

import "time"

// StockDailyIndicator — 日频估值指标（从腾讯行情 qt 字段提取）
// 单位规范: marketCap=元(非亿), PE/PB/PS=原始倍数
type StockDailyIndicator struct {
	Code                 string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate            time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	PE                   float64   `gorm:"type:numeric(14,4)" json:"pe"`                 // 市盈率(TTM,原始倍数)
	PB                   float64   `gorm:"type:numeric(14,4)" json:"pb"`                 // 市净率(原始倍数)
	PS                   float64   `gorm:"type:numeric(14,4)" json:"ps"`                 // 市销率
	TotalMarketCap       float64   `gorm:"type:numeric(20,2)" json:"totalMarketCap"`     // 总市值(亿)
	CirculatingMarketCap float64   `gorm:"type:numeric(20,2)" json:"circulatingMarketCap"` // 流通市值(亿)
}

func (StockDailyIndicator) TableName() string { return "stocks_daily_indicator" }