package model

import "time"

// MarginTrading stores daily margin trading (rz/rq) data per stock.
type MarginTrading struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string    `gorm:"size:10;index" json:"code"`
	TradeDate string    `gorm:"size:10;uniqueIndex:idx_margin_code_date" json:"tradeDate"`
	Rzye      float64   `json:"rzye"`
	Rzmre     float64   `json:"rzmre"`
	Rzche     float64   `json:"rzche"`
	Rqye      float64   `json:"rqye"`
	Rqmcl     float64   `json:"rqmcl"`
	Rqchl     float64   `json:"rqchl"`
	Rzrqye    float64   `json:"rzrqye"`
	CreatedAt time.Time `json:"createdAt"`
}

func (MarginTrading) TableName() string { return "margin_trading" }
