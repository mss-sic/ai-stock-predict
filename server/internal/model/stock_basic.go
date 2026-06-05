package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type JSONArray []string

func (j JSONArray) Value() (driver.Value, error) { return json.Marshal(j) }
func (j *JSONArray) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok { return errors.New("type assertion to []byte failed") }
	return json.Unmarshal(bytes, j)
}

type StockBasic struct {
	Code        string    `gorm:"primaryKey;size:10" json:"code"`
	Name        string    `gorm:"size:50" json:"name"`
	Industry    string    `gorm:"size:50" json:"industry"`
	ConceptTags JSONArray `gorm:"type:jsonb" json:"conceptTags"`
	ListedDate  *time.Time `json:"listedDate"`
	TotalShares int64     `json:"totalShares"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (StockBasic) TableName() string { return "stocks_basic" }
