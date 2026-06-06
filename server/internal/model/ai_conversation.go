package model

import "time"

// AIConversation stores chat history per stock (PostgreSQL)
type AIConversation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"index;size:10" json:"code"`
	Role      string    `gorm:"size:10" json:"role"` // user / ai
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func (AIConversation) TableName() string { return "ai_conversations" }
