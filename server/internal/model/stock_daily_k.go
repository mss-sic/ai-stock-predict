package model

import "time"

// StockDailyK — 日K线数据（腾讯前复权 + Tushare 原始行情）
// 单位规范:
//
//	volume=股(非手), amount=元, turnoverRate=原始比率
//	buyVol/sellVol=股, changePct=%, amplitude=%, volumeRatio=倍
//	preClose=元(昨收), changeAmount=元(涨跌额)
//	adjFactor=前复权因子(不复权价格×adjFactor=前复权价格)
//
// Tushare 字段映射:
//
//	vol(手)→volume(股)×100, amount(千元)→amount(元)×1000
type StockDailyK struct {
	Code         string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate    time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	Open         float64   `gorm:"type:numeric(12,4)" json:"open"`
	High         float64   `gorm:"type:numeric(12,4)" json:"high"`
	Low          float64   `gorm:"type:numeric(12,4)" json:"low"`
	Close        float64   `gorm:"type:numeric(12,4)" json:"close"`
	PreClose     float64   `gorm:"type:numeric(12,4)" json:"preClose"`
	ChangeAmount float64   `gorm:"type:numeric(12,4)" json:"changeAmount"`
	Volume       int64     `json:"volume"`
	Amount       float64   `gorm:"type:numeric(20,2)" json:"amount"`
	TurnoverRate float64   `gorm:"type:numeric(10,4)" json:"turnoverRate"`
	BuyVol       int64     `json:"buyVol"`
	SellVol      int64     `json:"sellVol"`
	ChangePct    float64   `gorm:"type:numeric(8,4)" json:"changePct"`
	Amplitude    float64   `gorm:"type:numeric(8,4)" json:"amplitude"`
	VolumeRatio  float64   `gorm:"type:numeric(8,4)" json:"volumeRatio"`

	// 数据源与元信息
	DataSource     string  `gorm:"type:varchar(20);default:'tencent'" json:"dataSource"`
	SourcePriority int     `gorm:"default:0" json:"sourcePriority"`
	DataQuality    string  `gorm:"type:varchar(20);default:'ok'" json:"dataQuality"`
	AdjFactor      float64 `gorm:"type:numeric(12,8);default:1.0" json:"adjFactor"`

	// 涨跌停与状态
	HighLimit float64 `gorm:"type:numeric(12,4)" json:"highLimit"`
	LowLimit  float64 `gorm:"type:numeric(12,4)" json:"lowLimit"`
	AvgPrice  float64 `gorm:"type:numeric(12,4)" json:"avgPrice"`
	IsPaused  bool    `gorm:"default:false" json:"isPaused"`

	// 预计算技术指标 (用于全市场扫描)
	Ema12   float64 `gorm:"type:numeric(12,4)" json:"ema12"`
	Ema26   float64 `gorm:"type:numeric(12,4)" json:"ema26"`
	MacdBar float64 `gorm:"type:numeric(12,4)" json:"macdBar"`

	// macd_dif 和 macd_dea 已由早期迁移创建，此处仅为模型映射
	MacdDif float64 `gorm:"column:macd_dif;type:numeric(12,4)" json:"macdDif"`
	MacdDea float64 `gorm:"column:macd_dea;type:numeric(12,4)" json:"macdDea"`

	UpdatedAt time.Time `json:"updatedAt"`
}

func (StockDailyK) TableName() string { return "stocks_daily_k" }
