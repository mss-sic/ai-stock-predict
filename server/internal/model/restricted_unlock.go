package model

import "time"

// RestrictedShareUnlock stores restricted share unlock (限售解禁) records.
type RestrictedShareUnlock struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string    `gorm:"size:10;index" json:"code"`
	FreeDate   string    `gorm:"size:10;index" json:"freeDate"`
	StockType  string    `gorm:"size:100" json:"stockType"`
	Shares     float64   `json:"shares"`
	Ratio      float64   `json:"ratio"`
	IsHistory  bool      `gorm:"default:true" json:"isHistory"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (RestrictedShareUnlock) TableName() string { return "restricted_share_unlock" }
