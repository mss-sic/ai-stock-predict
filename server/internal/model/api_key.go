package model

import "time"

// ApiKey stores API keys for external teams to import data (MySQL)
type ApiKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	KeyHash     string     `gorm:"uniqueIndex;size:64" json:"-"`
	KeyPrefix   string     `gorm:"size:12" json:"keyPrefix"`
	TeamName    string     `gorm:"size:100" json:"teamName"`
	Description string     `gorm:"size:255" json:"description"`
	Permissions string     `gorm:"type:text" json:"permissions"`
	IsActive    bool       `gorm:"default:true" json:"isActive"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (ApiKey) TableName() string { return "api_keys" }
