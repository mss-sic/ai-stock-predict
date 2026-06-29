package model

import "time"

// DragonTigerList stores daily all-market dragon tiger board summary records.
type DragonTigerList struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string    `gorm:"size:10;index" json:"code"`
	Name        string    `gorm:"size:50" json:"name"`
	TradeDate   string    `gorm:"size:10;index" json:"tradeDate"`
	Reason      string    `gorm:"size:200" json:"reason"`
	ClosePrice  float64   `json:"closePrice"`
	ChangePct   float64   `json:"changePct"`
	NetBuyAmt   float64   `json:"netBuyAmt"`
	BuyAmt      float64   `json:"buyAmt"`
	SellAmt     float64   `json:"sellAmt"`
	TurnoverPct float64   `json:"turnoverPct"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (DragonTigerList) TableName() string { return "dragon_tiger_list" }

// DragonTigerDetail stores seat-level detail for individual stocks on the dragon tiger board.
type DragonTigerDetail struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code          string    `gorm:"size:10;index" json:"code"`
	TradeDate     string    `gorm:"size:10;index" json:"tradeDate"`
	SeatName      string    `gorm:"size:100" json:"seatName"`
	SeatCode      string    `gorm:"size:20" json:"seatCode"`
	Side          string    `gorm:"size:5" json:"side"`
	BuyAmt        float64   `json:"buyAmt"`
	SellAmt       float64   `json:"sellAmt"`
	NetAmt        float64   `json:"netAmt"`
	IsInstitution bool      `gorm:"default:false" json:"isInstitution"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (DragonTigerDetail) TableName() string { return "dragon_tiger_detail" }
