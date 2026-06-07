package model

import "time"

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"uniqueIndex;size:50" json:"username"`
	Nickname     string     `gorm:"size:50" json:"nickname"`
	PasswordHash string     `gorm:"size:255" json:"-"`
	Role         string     `gorm:"size:20;default:user" json:"role"` // admin / user
	IsActive     bool       `gorm:"default:true" json:"isActive"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	LastLoginIP  string     `gorm:"size:50" json:"lastLoginIp"`
	LastDeviceFp string     `gorm:"size:64" json:"-"`
	Require2FA   bool       `gorm:"default:false" json:"require2fa"`
	TOTPSecret   string     `gorm:"size:64" json:"-"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// Session stores refresh tokens with device info
type Session struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	UserID        uint       `gorm:"index" json:"userId"`
	RefreshToken  string     `gorm:"uniqueIndex;size:512" json:"-"`
	AccessToken   string     `gorm:"size:512" json:"-"`
	DeviceFp      string     `gorm:"size:64" json:"-"`
	DeviceInfo    string     `gorm:"size:255" json:"deviceInfo"`
	IPAddress     string     `gorm:"size:50" json:"ipAddress"`
	IsActive      bool       `gorm:"default:true" json:"isActive"`
	IsOnline      bool       `gorm:"default:true" json:"isOnline"`
	LastHeartbeat *time.Time `json:"lastHeartbeat"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

func (Session) TableName() string { return "sessions" }
