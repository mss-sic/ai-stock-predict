package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) { return json.Marshal(j) }
func (j *JSONMap) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok { return errors.New("type assertion to []byte failed") }
	return json.Unmarshal(bytes, j)
}

type Strategy struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"userId"`
	Name      string    `gorm:"size:100" json:"name"`
	Params    JSONMap   `gorm:"type:json" json:"params"`
	CreatedAt time.Time `json:"createdAt"`
}
