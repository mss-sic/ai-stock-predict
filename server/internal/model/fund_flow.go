package model

import "time"

// StockFundFlow stores daily fund flow data (主力/大单/中单/小单/超大单净流入).
type StockFundFlow struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string    `gorm:"size:10;index" json:"code"`
	TradeDate string    `gorm:"size:10;index" json:"tradeDate"`
	MainNet   float64   `json:"mainNet"`
	SmallNet  float64   `json:"smallNet"`
	MidNet    float64   `json:"midNet"`
	LargeNet  float64   `json:"largeNet"`
	SuperNet  float64   `json:"superNet"`
	CreatedAt time.Time `json:"createdAt"`
}

func (StockFundFlow) TableName() string { return "stock_fund_flow" }


// BuySellFlowItem 内外盘资金流（fund_flow 的 fallback）
type BuySellFlowItem struct {
	TradeDate    string  `json:"tradeDate"`
	BuyVol       int64   `json:"buyVol"`
	SellVol      int64   `json:"sellVol"`
	NetFlow      int64   `json:"netFlow"`
	NetFlowRatio float64 `json:"netFlowRatio"`
}
