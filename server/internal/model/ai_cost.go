package model

import "time"

// AICostLog records every AI API call with token usage and cost.
// Stored in MySQL.
type AICostLog struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"index:idx_cost_user_date;default:0" json:"userId"`
	Username         string    `gorm:"size:50;default:''" json:"username"`
	Module           string    `gorm:"size:30;index:idx_cost_module_date" json:"module"`
	ModelName        string    `gorm:"size:100" json:"modelName"`
	Function         string    `gorm:"size:50;default:''" json:"function"`
	PromptTokens     int       `gorm:"default:0" json:"promptTokens"`
	CompletionTokens int       `gorm:"default:0" json:"completionTokens"`
	TotalTokens      int       `gorm:"default:0" json:"totalTokens"`
	PromptCacheHit   int       `gorm:"default:0" json:"promptCacheHit"`
	PromptCacheMiss  int       `gorm:"default:0" json:"promptCacheMiss"`
	CostAmount       float64   `gorm:"type:decimal(10,6);default:0" json:"costAmount"`
	DurationMs       int       `gorm:"default:0" json:"durationMs"`
	Success          bool      `gorm:"default:1" json:"success"`
	ErrorMsg         string    `gorm:"size:500;default:''" json:"errorMsg"`
	RequestContent   string    `gorm:"type:mediumtext" json:"requestContent"`
	ResponseContent  string    `gorm:"type:mediumtext" json:"responseContent"`
	CreatedAt        time.Time `gorm:"index:idx_cost_user_date" json:"createdAt"`
}

func (AICostLog) TableName() string { return "ai_cost_logs" }

// ModelPrice stores per-model token pricing for cost calculation.
type ModelPrice struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ModelName      string    `gorm:"size:100;uniqueIndex" json:"modelName"`
	DisplayName    string    `gorm:"size:100;default:''" json:"displayName"`
	InputPrice     float64   `gorm:"type:decimal(10,6);default:0" json:"inputPrice"`
	OutputPrice    float64   `gorm:"type:decimal(10,6);default:0" json:"outputPrice"`
	CacheHitPrice  float64   `gorm:"type:decimal(10,6);default:0" json:"cacheHitPrice"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (ModelPrice) TableName() string { return "model_prices" }

// UsageInfo is parsed from the AI API response "usage" field.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptCacheHit   int `json:"prompt_cache_hit_tokens"`
	PromptCacheMiss  int `json:"prompt_cache_miss_tokens"`
}

// defaultModelPrices returns the initial seed data from DeepSeek official pricing (2026).
func DefaultModelPrices() []ModelPrice {
	return []ModelPrice{
		{ModelName: "deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash", InputPrice: 1.00, OutputPrice: 2.00, CacheHitPrice: 0.02},
		{ModelName: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", InputPrice: 3.00, OutputPrice: 6.00, CacheHitPrice: 0.025},
	}
}
