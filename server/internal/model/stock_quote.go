package model

import "time"

// StockQuote stores real-time trading data from mootdx
type StockQuote struct {
	Code      string    `gorm:"primaryKey;size:10" json:"code"`
	Price     float64   `gorm:"type:numeric(12,4)" json:"price"`
	Open      float64   `gorm:"type:numeric(12,4)" json:"open"`
	High      float64   `gorm:"type:numeric(12,4)" json:"high"`
	Low       float64   `gorm:"type:numeric(12,4)" json:"low"`
	Volume    int64     `json:"volume"`
	Amount    float64   `gorm:"type:numeric(20,2)" json:"amount"`
	BidVol    int64     `json:"bidVol"`     // 外盘
	AskVol    int64     `json:"askVol"`     // 内盘
	Turnover  float64   `gorm:"type:numeric(10,4)" json:"turnover"` // 换手率%
	UpdatedAt time.Time `json:"updatedAt"`
}

func (StockQuote) TableName() string { return "stock_quotes" }
