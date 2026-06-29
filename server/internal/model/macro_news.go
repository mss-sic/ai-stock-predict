package model

import "time"

// MacroNews stores global macro news / 7x24 financial news.
type MacroNews struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"size:500" json:"title"`
	Summary   string    `gorm:"type:text" json:"summary"`
	NewsTime  string    `gorm:"size:30" json:"newsTime"`
	Category  string    `gorm:"size:30;index;default:general" json:"category"`
	CreatedAt time.Time `json:"createdAt"`
}

func (MacroNews) TableName() string { return "macro_news" }
