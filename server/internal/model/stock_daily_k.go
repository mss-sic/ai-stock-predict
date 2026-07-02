package model

import "time"

// StockDailyK — 日K线数据（腾讯前复权 + 实时行情补充）
// 单位规范: volume=股(不是手), amount=元, turnoverRate=%(百倍值,如0.26=0.26%)
type StockDailyK struct {
	Code         string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate    time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	Open         float64   `gorm:"type:numeric(12,4)" json:"open"`          // 开盘价(元)
	High         float64   `gorm:"type:numeric(12,4)" json:"high"`          // 最高价(元)
	Low          float64   `gorm:"type:numeric(12,4)" json:"low"`           // 最低价(元)
	Close        float64   `gorm:"type:numeric(12,4)" json:"close"`         // 收盘价(元,前复权)
	Volume       int64     `json:"volume"`                                   // 成交量(股,非手)
	Amount       float64   `gorm:"type:numeric(20,2)" json:"amount"`        // 成交额(元)
	TurnoverRate float64   `gorm:"type:numeric(10,4)" json:"turnoverRate"`  // 换手率(%,如0.26=0.26%)
}

func (StockDailyK) TableName() string { return "stocks_daily_k" }