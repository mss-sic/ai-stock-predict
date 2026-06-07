package model

import "time"

type LoginLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"userId"`
	Username   string    `gorm:"size:50" json:"username"`
	Action     string    `gorm:"size:20" json:"action"` // login / logout / failed / kicked
	Success    bool      `json:"success"`
	IPAddress  string    `gorm:"size:50" json:"ipAddress"`
	DeviceInfo string    `gorm:"size:255" json:"deviceInfo"`
	DeviceFp   string    `gorm:"size:64" json:"-"`
	FailReason string    `gorm:"size:200" json:"failReason,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (LoginLog) TableName() string { return "login_logs" }
