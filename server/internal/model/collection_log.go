package model

import "time"

type CollectionLog struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Phases        string     `gorm:"type:text" json:"phases"`
	TotalNew      int        `json:"totalNew"`
	TotalSkipped  int        `json:"totalSkipped"`
	TotalErrors   int        `json:"totalErrors"`
	Status        string     `gorm:"size:20;default:running" json:"status"`
	ErrorMsg      string     `gorm:"type:text" json:"errorMsg"`
	BehaviorStats string     `gorm:"type:text" json:"behaviorStats"`
	DurationMs    int64      `json:"durationMs"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}
