package model

import (
	"encoding/json"
	"time"
)

// StockShareholder stores shareholder data per stock
type StockShareholder struct {
	ID              uint            `gorm:"primaryKey" json:"id"`
	Code            string          `gorm:"index:idx_sh_code_date;size:10" json:"code"`
	ReportDate      string          `gorm:"index:idx_sh_code_date;size:10" json:"reportDate"` // 报告期 2026-03-31
	TotalHolders    int64           `json:"totalHolders"`     // 股东总户数
	HolderChange    float64         `json:"holderChange"`     // 环比变化%
	Top10Holders    json.RawMessage `gorm:"type:jsonb" json:"top10Holders"`    // [{name, shares, ratio}]
	Top10Float      json.RawMessage `gorm:"type:jsonb" json:"top10Float"`      // 十大流通股东
	InstHoldRatio   float64         `json:"instHoldRatio"`   // 机构持股比例%
	AvgHolding      int64           `json:"avgHolding"`       // 户均持股
	CreatedAt       time.Time       `json:"createdAt"`
}

func (StockShareholder) TableName() string { return "stock_shareholders" }
