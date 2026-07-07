package model

import "time"

// NotificationLog records sent notifications for dedup and audit.
// Dedup key: (event_type, event_date, run_id) — one notification per run per day.
type NotificationLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"userId"`
	RunID     uint      `gorm:"index:idx_notify_dedup,priority:3;default:0" json:"runId"`
	EventType string    `gorm:"size:50;index:idx_notify_dedup,priority:1" json:"eventType"` // "pre_market_report"
	EventDate string    `gorm:"size:10;index:idx_notify_dedup,priority:2" json:"eventDate"` // "2026-07-07"
	Title     string    `gorm:"size:200" json:"title"`
	SentAt    time.Time `json:"sentAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func (NotificationLog) TableName() string { return "notification_logs" }
