package model

import "time"

// ThsEpsForecast stores THS (同花顺) consensus EPS forecast data.
type ThsEpsForecast struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code            string    `gorm:"size:10;index" json:"code"`
	Year            string    `gorm:"size:10" json:"year"`
	InstitutionCount int      `json:"institutionCount"`
	EpsMin          float64   `json:"epsMin"`
	EpsAvg          float64   `json:"epsAvg"`
	EpsMax          float64   `json:"epsMax"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (ThsEpsForecast) TableName() string { return "ths_eps_forecast" }
