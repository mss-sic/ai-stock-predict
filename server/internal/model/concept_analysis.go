package model

import "time"

// ConceptAnalysis stores AI-generated concept analysis (PG)
type ConceptAnalysis struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ConceptCode string    `gorm:"uniqueIndex;size:20" json:"conceptCode"`
	Content     string    `gorm:"type:text" json:"content"`
	GeneratedAt time.Time `json:"generatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (ConceptAnalysis) TableName() string { return "concept_analyses" }
