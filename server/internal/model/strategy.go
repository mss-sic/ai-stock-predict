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

// Strategy represents a user's trading strategy
type Strategy struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint      `gorm:"index" json:"userId"`
	Name         string    `gorm:"size:100" json:"name"`
	Description  string    `gorm:"size:500" json:"description"`
	SortOrder    int       `gorm:"default:0" json:"sortOrder"`
	IsDefault    bool      `gorm:"default:false" json:"isDefault"`
	StopProfit   float64   `gorm:"default:0" json:"stopProfit"`    // 止盈%, 0=不设
	StopLoss     float64   `gorm:"default:0" json:"stopLoss"`      // 止损% (负数), 0=不设
	MaxHoldings  int       `gorm:"default:20" json:"maxHoldings"`  // 最大同时持股数

	// Position sizing (仓位管理)
	BuyPositionPct    float64 `gorm:"default:15" json:"buyPositionPct"`    // 初始买入仓位 (占总资金%)
	AddPositionPct    float64 `gorm:"default:10" json:"addPositionPct"`    // 加仓仓位 (占总资金%)
	ReducePositionPct float64 `gorm:"default:50" json:"reducePositionPct"` // 减仓比例 (占持仓%)

	// Position sizing method
	PositionSizing string `gorm:"size:15;default:fixed_pct" json:"positionSizing"` // fixed_pct / equal_weight / kelly

	// Investment plan
	InitialCapital  float64 `gorm:"default:100000" json:"initialCapital"` // 初始资金
	InvestmentType  string  `gorm:"size:10;default:lump" json:"investmentType"` // lump=一次性 / regular=定投
	RegularAmount   float64 `gorm:"default:0" json:"regularAmount"`       // 定投金额 (每次追加)
	RegularInterval string  `gorm:"size:10;default:monthly" json:"regularInterval"` // daily/weekly/monthly

	// Stock universe for backtest
	StockCodes string `gorm:"size:2000" json:"stockCodes"` // 逗号分隔的自选股票池, 空=使用榜单

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// StrategyCondition defines a single factor condition in a strategy
type StrategyCondition struct {
	ID         uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	StrategyID uint   `gorm:"index" json:"strategyId"`
	CondType   string `gorm:"size:10" json:"condType"`    // buy / add / sell / reduce
	Indicator  string `gorm:"size:30" json:"indicator"`    // streak_count / algo_score / ai_score / ma_cross etc
	Operator   string `gorm:"size:15" json:"operator"`     // gt / lt / gte / lte / eq / cross_up / cross_down
	Value      float64 `json:"value"`                      // threshold
	Enabled    bool    `gorm:"default:true" json:"enabled"` // soft enable/disable without deleting
	LogicGroup int    `gorm:"default:1" json:"logicGroup"` // AND group
	SortOrder  int    `gorm:"default:0" json:"sortOrder"`
}
