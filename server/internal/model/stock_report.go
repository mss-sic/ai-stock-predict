package model

import (
	"encoding/json"
	"time"
)

type StockReport struct {
	ID                     uint            `gorm:"primaryKey" json:"id"`
	InfoCode               string          `gorm:"uniqueIndex;size:30" json:"infoCode"`
	Title                  string          `gorm:"size:500" json:"title"`
	StockCode              string          `gorm:"index;size:10" json:"stockCode"`
	StockName              string          `gorm:"size:50" json:"stockName"`
	OrgName                string          `gorm:"size:200" json:"orgName"`
	OrgSName               string          `gorm:"column:org_sname;size:100" json:"orgSname"`
	PublishDate            string          `gorm:"index;size:10" json:"publishDate"`
	Rating                 string          `gorm:"size:20" json:"rating"`
	RatingChange           string          `gorm:"size:20" json:"ratingChange"`
	PredictThisYearEPS     float64         `json:"predictThisYearEps"`
	PredictThisYearPE      float64         `json:"predictThisYearPe"`
	PredictNextYearEPS     float64         `json:"predictNextYearEps"`
	PredictNextYearPE      float64         `json:"predictNextYearPe"`
	PredictNextTwoYearEPS  float64         `json:"predictNextTwoYearEps"`
	PredictNextTwoYearPE   float64         `json:"predictNextTwoYearPe"`
	Author                 json.RawMessage `gorm:"type:jsonb" json:"author"`
	Researcher             string          `gorm:"size:200" json:"researcher"`
	IndustryName           string          `gorm:"index;size:100" json:"industryName"`
	PDFUrl                 string          `gorm:"size:200" json:"pdfUrl"`
	AttachSize             int             `json:"attachSize"`
	AttachPages            int             `json:"attachPages"`
	CreatedAt              time.Time       `json:"createdAt"`
}

func (StockReport) TableName() string { return "stock_reports" }
