package model

import "time"

// StockRealtimeQuote stores the latest real-time quote snapshot for a stock.
// Updated by scripts/collector/realtime_quotes.py during trading hours.
type StockRealtimeQuote struct {
	Code              string    `gorm:"primaryKey;size:10" json:"code"`
	Name              string    `gorm:"size:50" json:"name"`
	Price             float64   `json:"price"`             // 当前价
	PrevClose         float64   `json:"prevClose"`         // 昨收
	Open              float64   `json:"open"`              // 今开
	High              float64   `json:"high"`              // 最高
	Low               float64   `json:"low"`               // 最低
	Volume            int64     `json:"volume"`            // 成交量(手)
	Amount            float64   `json:"amount"`            // 成交额(万元)
	ChangePct         float64   `json:"changePct"`         // 涨跌幅%
	TurnoverRate      float64   `json:"turnoverRate"`      // 换手率%
	PE                float64   `json:"pe"`                // 市盈率
	PB                float64   `json:"pb"`                // 市净率
	TotalMarketCap    float64   `json:"totalMarketCap"`    // 总市值(亿)
	CirculatingMcap   float64   `json:"circulatingMcap"`   // 流通市值(亿)
	Amplitude         float64   `json:"amplitude"`         // 振幅%
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (StockRealtimeQuote) TableName() string { return "stock_realtime_quote" }
