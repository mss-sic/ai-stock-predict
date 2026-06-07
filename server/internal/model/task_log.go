package model

import "time"

type TaskLog struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	TaskID     uint       `gorm:"index" json:"taskId"`
	TaskName   string     `gorm:"size:100" json:"taskName"`
	Phase      string     `gorm:"size:50" json:"phase"`
	Status     string     `gorm:"size:20" json:"status"`           // running, success, failed
	TotalNew   int        `json:"totalNew"`
	TotalSkip  int        `json:"totalSkip"`
	TotalErr   int        `json:"totalErr"`
	Result     string     `gorm:"type:text" json:"result"`         // JSON summary
	ErrorMsg   string     `gorm:"type:text" json:"errorMsg"`
	DurationMs int64      `json:"durationMs"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}
