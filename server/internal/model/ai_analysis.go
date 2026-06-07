package model

import "time"

// AIAnalysis stores AI-generated analysis results (PostgreSQL)
type AIAnalysis struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Code       string    `gorm:"index;size:10" json:"code"`
	PickDate   string    `gorm:"index;size:10" json:"pickDate"`
	Model      string    `gorm:"size:50" json:"model"`
	RiskLevel  string    `gorm:"size:20" json:"riskLevel"`
	Suggestion string    `gorm:"size:20" json:"suggestion"`
	Summary    string    `gorm:"type:text" json:"summary"`
	Signals    JSONArray `gorm:"type:jsonb" json:"signals"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (AIAnalysis) TableName() string { return "ai_analyses" }

// Prediction stores model prediction results (PostgreSQL)
type Prediction struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Code           string    `gorm:"index;size:10" json:"code"`
	ModelName      string    `gorm:"index;size:30" json:"modelName"`
	PredictDate    time.Time `gorm:"index;type:date" json:"predictDate"`
	PredictedPrice float64   `gorm:"type:numeric(12,4)" json:"predictedPrice"`
	UpperBound     float64   `gorm:"type:numeric(12,4)" json:"upperBound"`
	LowerBound     float64   `gorm:"type:numeric(12,4)" json:"lowerBound"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (Prediction) TableName() string { return "predictions" }

// PredictionKDist stores algorithm team K-distribution data for chart overlay
type PredictionKDist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"uniqueIndex;size:10" json:"code"`
	KDData    string    `gorm:"type:jsonb" json:"kdData"` // JSON: [[float*20]*7]
	UpdatedAt time.Time `json:"updatedAt"`
}

func (PredictionKDist) TableName() string { return "prediction_kdist" }
