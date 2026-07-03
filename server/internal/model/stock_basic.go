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
	SwL1        string    `gorm:"column:sw_l1;size:50" json:"swL1"`
	SwL2        string    `gorm:"column:sw_l2;size:50" json:"swL2"`
	SwL2Dc      string    `gorm:"column:sw_l2_dc;size:50" json:"swL2Dc"`
	ConceptTags JSONArray `gorm:"type:jsonb" json:"conceptTags"`
	ListedDate  *time.Time `json:"listedDate"`
	TotalShares int64     `json:"totalShares"`
	BoardType   string    `gorm:"size:5" json:"boardType"`   // sh/kc/sz/cy/bj
	IsST        bool      `gorm:"default:false" json:"isST"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (StockBasic) TableName() string { return "stocks_basic" }
