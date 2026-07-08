package model

import "time"

// StockDailyK — 日K线数据（腾讯前复权 + Tushare 原始行情）
// 单位规范:
//   volume=股(非手), amount=元, turnoverRate=原始比率
//   buyVol/sellVol=股, changePct=%, amplitude=%, volumeRatio=倍
//   preClose=元(昨收), changeAmount=元(涨跌额)
// Tushare 字段映射:
//   vol(手)→volume(股)×100, amount(千元)→amount(元)×1000
type StockDailyK struct {
	Code         string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate    time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	Open         float64   `gorm:"type:numeric(12,4)" json:"open"`          // 开盘价(元)
	High         float64   `gorm:"type:numeric(12,4)" json:"high"`          // 最高价(元)
	Low          float64   `gorm:"type:numeric(12,4)" json:"low"`           // 最低价(元)
	Close        float64   `gorm:"type:numeric(12,4)" json:"close"`         // 收盘价(元)
	PreClose     float64   `gorm:"type:numeric(12,4)" json:"preClose"`      // 昨收价(元, Tushare pre_close)
	ChangeAmount float64   `gorm:"type:numeric(12,4)" json:"changeAmount"`  // 涨跌额(元, Tushare change)
	Volume       int64     `json:"volume"`                                   // 成交量(股)
	Amount       float64   `gorm:"type:numeric(20,2)" json:"amount"`        // 成交额(元)
	TurnoverRate float64   `gorm:"type:numeric(10,4)" json:"turnoverRate"`  // 换手率(原始比率,如0.0026=0.26%,即qt[38]/100)
	BuyVol       int64     `json:"buyVol"`                                   // 外盘(主动性买盘,股)
	SellVol      int64     `json:"sellVol"`                                  // 内盘(主动性卖盘,股)
	ChangePct    float64   `gorm:"type:numeric(8,4)" json:"changePct"`       // 涨跌幅(%,如0.31=0.31%)
	Amplitude    float64   `gorm:"type:numeric(8,4)" json:"amplitude"`       // 振幅(%,如2.10=2.10%)
	VolumeRatio  float64   `gorm:"type:numeric(8,4)" json:"volumeRatio"`     // 量比(相对5日均量)
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (StockDailyK) TableName() string { return "stocks_daily_k" }
