package model

import "time"

// StockFinancial stores financial statement data per stock per report period
type StockFinancial struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Code            string    `gorm:"index:idx_fin_code_date;size:10" json:"code"`
	ReportDate      string    `gorm:"index:idx_fin_code_date;size:10" json:"reportDate"` // 2026-03-31
	ReportType      string    `gorm:"size:10" json:"reportType"` // 一季报/中报/三季报/年报
	
	// Income Statement (利润表) key metrics — in 万元
	TotalRevenue    float64   `json:"totalRevenue"`     // 营业总收入
	NetProfit       float64   `json:"netProfit"`        // 归母净利润
	RevenueGrowth   float64   `json:"revenueGrowth"`    // 营收同比%
	ProfitGrowth    float64   `json:"profitGrowth"`     // 净利润同比%
	
	// Balance Sheet (资产负债表) key metrics — in 万元
	TotalAssets     float64   `json:"totalAssets"`      // 总资产
	TotalLiabilities float64  `json:"totalLiabilities"` // 总负债
	NetAssets       float64   `json:"netAssets"`        // 净资产
	
	// Key ratios
	ROE             float64   `json:"roe"`              // 净资产收益率%
	EPS             float64   `json:"eps"`              // 每股收益
	BPS             float64   `json:"bps"`              // 每股净资产
	GrossMargin     float64   `json:"grossMargin"`      // 毛利率%
	NetMargin       float64   `json:"netMargin"`        // 净利率%
	DebtRatio       float64   `json:"debtRatio"`        // 资产负债率%
	
	CreatedAt       time.Time `json:"createdAt"`
}

func (StockFinancial) TableName() string { return "stock_financials" }
