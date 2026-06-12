package model

import "time"

// TradingAccount represents a user's virtual trading account for portfolio tracking.
type TradingAccount struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"uniqueIndex" json:"userId"`
	InitialCapital float64   `gorm:"type:numeric(16,2);default:0" json:"initialCapital"` // 初始本金
	AvailableCash  float64   `gorm:"type:numeric(16,2);default:0" json:"availableCash"`   // 当前可用余额
	FrozenCash     float64   `gorm:"type:numeric(16,2);default:0" json:"frozenCash"`      // 冻结资金
	TotalDeposit   float64   `gorm:"type:numeric(16,2);default:0" json:"totalDeposit"`    // 累计入金
	TotalWithdraw  float64   `gorm:"type:numeric(16,2);default:0" json:"totalWithdraw"`   // 累计出金
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (TradingAccount) TableName() string { return "trading_accounts" }
