package model

import "time"

// CninfoAnnouncement stores cninfo (巨潮) announcement records.
type CninfoAnnouncement struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code         string    `gorm:"size:10;index" json:"code"`
	Title        string    `gorm:"size:500" json:"title"`
	AnnType      string    `gorm:"size:50" json:"annType"`
	AnnDate      string    `gorm:"size:10;index" json:"annDate"`
	AnnURL       string    `gorm:"size:500" json:"annUrl"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (CninfoAnnouncement) TableName() string { return "cninfo_announcements" }
