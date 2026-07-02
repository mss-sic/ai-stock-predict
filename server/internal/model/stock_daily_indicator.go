package model

import "time"

// StockDailyIndicator — 日频估值与技术指标（Tushare daily_basic）
// 单位规范:
//   pe/pb/ps=原始倍数, turnover_rate=%(如0.47=0.47%), volume_ratio=倍
//   dv_ratio=%(如5.89=5.89%), total_share/float_share/free_share=万股
//   total_market_cap/circulating_market_cap=元(Tushare返回万元,脚本×10000)
type StockDailyIndicator struct {
	Code                 string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate            time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	PE                   float64   `gorm:"type:numeric(14,4)" json:"pe"`
	PETTM                float64   `gorm:"type:numeric(14,4)" json:"peTtm"`
	PB                   float64   `gorm:"type:numeric(14,4)" json:"pb"`
	PS                   float64   `gorm:"type:numeric(14,4)" json:"ps"`
	PSTTM                float64   `gorm:"type:numeric(14,4)" json:"psTtm"`
	TurnoverRate         float64   `gorm:"type:numeric(10,4)" json:"turnoverRate"`
	TurnoverRateF        float64   `gorm:"type:numeric(10,4)" json:"turnoverRateF"`
	VolumeRatio          float64   `gorm:"type:numeric(10,4)" json:"volumeRatio"`
	DividendYield        float64   `gorm:"column:dv_ratio;type:numeric(10,4)" json:"dvRatio"`
	DividendYieldTTM     float64   `gorm:"column:dv_ttm;type:numeric(10,4)" json:"dvTtm"`
	TotalShare           float64   `gorm:"type:numeric(20,4)" json:"totalShare"`
	FloatShare           float64   `gorm:"type:numeric(20,4)" json:"floatShare"`
	FreeShare            float64   `gorm:"type:numeric(20,4)" json:"freeShare"`
	TotalMarketCap       float64   `gorm:"type:numeric(20,2)" json:"totalMarketCap"`
	CirculatingMarketCap float64   `gorm:"type:numeric(20,2)" json:"circulatingMarketCap"`
	DataSource           string    `gorm:"size:20;default:'tushare'" json:"dataSource"`
}

func (StockDailyIndicator) TableName() string { return "stocks_daily_indicator" }
