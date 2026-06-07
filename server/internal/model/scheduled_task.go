package model

import "time"

type ScheduledTask struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Name      string     `gorm:"size:100" json:"name"`
	Phase     string     `gorm:"size:50;index" json:"phase"`      // 采集阶段标识：kline, indicator, quote, risk_scan...
	CronExpr  string     `gorm:"size:50" json:"cronExpr"`          // cron 表达式
	Enabled   bool       `gorm:"default:true" json:"enabled"`
	LastRun   *time.Time `json:"lastRun"`
	LastStatus string    `gorm:"size:20" json:"lastStatus"`       // success, failed, running
	NextRun   *time.Time `json:"nextRun"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
