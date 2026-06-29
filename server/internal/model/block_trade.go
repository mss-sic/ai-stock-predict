package model

import "time"

// BlockTrade stores block trade (大宗交易) records.
type BlockTrade struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string    `gorm:"size:10;index" json:"code"`
	TradeDate  string    `gorm:"size:10;index" json:"tradeDate"`
	DealPrice  float64   `json:"dealPrice"`
	ClosePrice float64   `json:"closePrice"`
	PremiumPct float64   `json:"premiumPct"`
	DealVolume float64   `json:"dealVolume"`
	DealAmt    float64   `json:"dealAmt"`
	BuyerName  string    `gorm:"size:100" json:"buyerName"`
	SellerName string    `gorm:"size:100" json:"sellerName"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (BlockTrade) TableName() string { return "block_trade" }
