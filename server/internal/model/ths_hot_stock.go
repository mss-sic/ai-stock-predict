package model

import "time"

// ThsHotStock stores daily THS (同花顺) hot stock / strong stock list with reason tags.
type ThsHotStock struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string    `gorm:"size:10;index" json:"code"`
	Name           string    `gorm:"size:50" json:"name"`
	TradeDate      string    `gorm:"size:10;index" json:"tradeDate"`
	ClosePrice     float64   `json:"closePrice"`
	ChangeAmount   float64   `json:"changeAmount"`
	ChangePct      float64   `json:"changePct"`
	TurnoverPct    float64   `json:"turnoverPct"`
	Volume         float64   `json:"volume"`
	Amount         float64   `json:"amount"`
	DdeNetAmount   float64   `json:"ddeNetAmount"`
	ReasonTags     string    `gorm:"type:text" json:"reasonTags"`
	Market         string    `gorm:"size:5" json:"market"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (ThsHotStock) TableName() string { return "ths_hot_stocks" }
