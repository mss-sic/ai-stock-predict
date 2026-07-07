package model

import "time"

// NotificationConfig stores per-user notification channel settings.
type NotificationConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"userId"`
	Channel   string    `gorm:"size:30" json:"channel"`   // dingtalk_bot / feishu_bot / wecom_bot / email
	Name      string    `gorm:"size:100" json:"name"`     // display name, e.g. "企微通知群"
	Config    JSONMap   `gorm:"type:json" json:"config"`  // {"webhook_url": "...", "secret": "..."} | {"smtp_host":...,"to":"..."}
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (NotificationConfig) TableName() string { return "notification_configs" }

// Notification records a sent notification for audit trail.
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"userId"`
	Channel   string    `gorm:"size:30" json:"channel"`
	Title     string    `gorm:"size:200" json:"title"`
	Body      string    `gorm:"type:text" json:"body"`
	Status    string    `gorm:"size:15;default:sent" json:"status"` // sent / failed
	ErrorMsg  string    `gorm:"size:500" json:"errorMsg"`
	CreatedAt time.Time `json:"createdAt"`
}

func (Notification) TableName() string { return "notifications" }
