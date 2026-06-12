package model

import "time"

// AISystemConfig stores per-scene system prompt and parameters (PostgreSQL)
type AISystemConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Scene        string    `gorm:"uniqueIndex;size:50" json:"scene"`    // chat_analysis / stock_scoring
	Name         string    `gorm:"size:100" json:"name"`               // 场景名称（中文）
	SystemPrompt string    `gorm:"type:text" json:"systemPrompt"`       // 系统提示词模板，支持 %s 占位符
	ModelName    string    `gorm:"size:100" json:"modelName"`           // 模型名，空=使用用户配置
	Temperature  float64   `gorm:"default:0.7" json:"temperature"`
	MaxTokens    int       `gorm:"default:2048" json:"maxTokens"`
	EnableSearch bool      `gorm:"default:true" json:"enableSearch"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (AISystemConfig) TableName() string { return "ai_system_configs" }
