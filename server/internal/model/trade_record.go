package model

import "time"

// TradeRecord logs every buy/sell transaction for audit and P&L analysis.
type TradeRecord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"userId"`
	StockCode  string    `gorm:"size:10" json:"stockCode"`
	StockName  string    `gorm:"size:50" json:"stockName"`
	TradeType  string    `gorm:"size:10" json:"tradeType"`   // buy / sell
	TradeDate  string    `gorm:"size:10;index" json:"tradeDate"` // YYYY-MM-DD
	Price      float64   `gorm:"type:numeric(12,4)" json:"price"`
	Quantity   int       `json:"quantity"`
	Amount     float64   `gorm:"type:numeric(16,2)" json:"amount"`      // price * quantity
	Pnl        float64   `gorm:"type:numeric(16,2);default:0" json:"pnl"`      // realized P&L (sell only)
	PnlPct     float64   `gorm:"type:numeric(10,4);default:0" json:"pnlPct"`   // realized P&L % (sell only)
	HoldDays   int       `gorm:"default:0" json:"holdDays"`                     // holding period (sell only)
	HoldingID  *uint     `json:"holdingId"`                                     // FK to holdings for buys
	CreatedAt  time.Time `json:"createdAt"`
}

func (TradeRecord) TableName() string { return "trade_records" }
