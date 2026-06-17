package model

import "time"

// StockProfile stores AI-generated structured company profile (PostgreSQL)
type StockProfile struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Code          string    `gorm:"uniqueIndex;size:10" json:"code"`
	ProfileMarkdown string  `gorm:"type:text" json:"profileMarkdown"`   // 结构化 Markdown 简介
	ScoresJSON    string    `gorm:"type:text" json:"scoresJson"`        // 六维度评分 JSON
	AnalyzedAt    time.Time `json:"analyzedAt"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (StockProfile) TableName() string { return "stock_profiles" }
