package model

import "time"

// ConditionTemplate is a reusable composite condition template.
// System templates are pre-built; users can create custom ones.
type ConditionTemplate struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	Category    string    `gorm:"size:10" json:"category"`  // buy / sell / both
	CondType    string    `gorm:"size:10" json:"condType"`  // buy / add / sell / reduce
	IsSystem    bool      `gorm:"default:false" json:"isSystem"`
	CreatedBy   uint      `gorm:"default:0" json:"createdBy"` // 0 = system
	CreatedAt   time.Time `json:"createdAt"`
}

func (ConditionTemplate) TableName() string { return "condition_templates" }
