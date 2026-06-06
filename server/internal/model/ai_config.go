package model

import "time"

// AIConfig stores AI provider settings (MySQL)
type AIConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Provider  string    `gorm:"size:50;default:deepseek" json:"provider"`
	APIKey    string    `gorm:"size:512" json:"apiKey"`
	ModelName string    `gorm:"size:100;default:deepseek-chat" json:"modelName"`
	BaseURL   string    `gorm:"size:255;default:https://api.deepseek.com" json:"baseUrl"`
	IsActive  bool      `gorm:"default:false" json:"isActive"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AIConfig) TableName() string { return "ai_configs" }
