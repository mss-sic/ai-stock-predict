package model

import "time"

type ImportLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	FileName     string    `gorm:"size:255" json:"fileName"`
	RowsImported int       `json:"rowsImported"`
	Status       string    `gorm:"size:20;default:pending" json:"status"`
	ErrorMsg     string    `gorm:"type:text" json:"errorMsg"`
	ImportedAt   time.Time `json:"importedAt"`
}
