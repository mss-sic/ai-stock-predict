package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) { return json.Marshal(j) }
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		// Try string (some drivers return JSON as string)
		if s, ok2 := value.(string); ok2 {
			bytes = []byte(s)
		} else {
			return errors.New("type assertion to []byte failed")
		}
	}
	if len(bytes) == 0 || string(bytes) == "null" {
		*j = nil
		return nil
	}
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

	// Risk limits
	PositionConcentrationLimit float64 `gorm:"default:0.25" json:"positionConcentrationLimit"` // 单票最大仓位占比 (0-1)
	MaxDailyLoss               float64 `gorm:"default:-0.05" json:"maxDailyLoss"`               // 单日最大亏损比例 (负数)

	// ── Trailing Stop (移动止盈) ──
	EnableTrailingStop     bool    `gorm:"default:false" json:"enableTrailingStop"`
	TrailingStopActivation float64 `gorm:"default:15" json:"trailingStopActivation"` // 激活阈值%
	TrailingStopDrawdown   float64 `gorm:"default:8" json:"trailingStopDrawdown"`     // 回撤%

	// ── Dip Buy (抄底反弹) ──
	EnableDipBuy      bool    `gorm:"default:false" json:"enableDipBuy"`
	DipBuyThreshold   float64 `gorm:"default:-15" json:"dipBuyThreshold"`    // 触发跌幅%
	DipBuyAmountPct   float64 `gorm:"default:10" json:"dipBuyAmountPct"`    // 抄底仓位%
	DipTargetReturn   float64 `gorm:"default:5" json:"dipTargetReturn"`     // 目标收益%
	DipMaxHoldDays    int     `gorm:"default:3" json:"dipMaxHoldDays"`      // 最大持有天数
	DipCooldownDays   int     `gorm:"default:10" json:"dipCooldownDays"`    // 冷却期

	// Investment plan
	InitialCapital  float64 `gorm:"default:100000" json:"initialCapital"` // 初始资金
	InvestmentType  string  `gorm:"size:10;default:lump" json:"investmentType"` // lump=一次性 / regular=定投
	RegularAmount   float64 `gorm:"default:0" json:"regularAmount"`       // 定投金额 (每次追加)
	RegularInterval string  `gorm:"size:10;default:monthly" json:"regularInterval"` // daily/weekly/monthly

	// Stock universe for backtest
	StockCodes string `gorm:"size:2000" json:"stockCodes"` // 逗号分隔的自选股票池, 空=使用榜单

	// ── Strategy Orchestration v2 ──
	OrchestrationMode string `gorm:"size:20;default:hybrid" json:"orchestrationMode"` // scoring / decision_tree / hybrid

	// Market context
	EnableMarketContext bool    `gorm:"default:false" json:"enableMarketContext"`
	MarketCompositeMin  float64 `gorm:"default:-2.0" json:"marketCompositeMin"`   // 市场情绪综合分低于此值禁止开仓
	MarketPositionBias  float64 `gorm:"default:1.0" json:"marketPositionBias"`    // 仓位比例市场乘数（0.5-1.5）

	// AI Agent
	EnableAIAgent          bool   `gorm:"default:false" json:"enableAIAgent"`
	AIAgentMode            string `gorm:"size:20;default:advisory" json:"aiAgentMode"`              // advisory / auto
	AIAgentReviewScope     string `gorm:"size:20;default:all" json:"aiAgentReviewScope"`           // all / buy_only / sell_only
	AIAgentMaxDailyTrades  int    `gorm:"default:5" json:"aiAgentMaxDailyTrades"`                  // AI 单日最大成交笔数

	// Industry
	IndustryFilter        string `gorm:"size:500" json:"industryFilter"`              // 逗号分隔行业白名单，空=全部
	EnableSectorRotation  bool   `gorm:"default:false" json:"enableSectorRotation"`   // 板块轮动

	// ── Policy Manager v3 ──
	PolicyMode           string  `gorm:"size:20;default:rule" json:"policyMode"`           // rule / ai_driven / manual
	AggressiveThreshold  float64 `gorm:"default:1.5" json:"aggressiveThreshold"`           // 进攻模式阈值
	DefensiveThreshold   float64 `gorm:"default:0.0" json:"defensiveThreshold"`            // 防御模式阈值
	PolicyAggressive     JSONMap `gorm:"type:json" json:"policyAggressive"`                // 进攻模式自定义参数
	PolicyDefensive      JSONMap `gorm:"type:json" json:"policyDefensive"`                 // 防御模式自定义参数
	PolicyCash           JSONMap `gorm:"type:json" json:"policyCash"`                      // 空仓模式自定义参数

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
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

	// ── Strategy Orchestration v2 ──
	Weight       float64 `gorm:"default:1.0" json:"weight"`                    // 条件权重 0-5（加权评分模式）
	FuzzySigma   float64 `gorm:"default:0" json:"fuzzySigma"`                  // >0 启用模糊评分，sigma 控制衰减速度
	LookbackDays int     `gorm:"default:1" json:"lookbackDays"`                // 回溯天数
	ConsecutiveDays int  `gorm:"default:1" json:"consecutiveDays"`             // 需连续满足 N 天
	TrendDirection string `gorm:"size:10;default:none" json:"trendDirection"`  // none / improving / deteriorating
	IndustryRelative   bool    `gorm:"default:false" json:"industryRelative"`   // 阈值相对于行业中位数
	IndustryPercentile float64 `gorm:"default:50" json:"industryPercentile"`   // 行业分位数（PE/PB 等估值指标）
	Timeframe      string `gorm:"size:10;default:daily" json:"timeframe"`      // daily / weekly / monthly
	ParentID       *uint  `gorm:"index" json:"parentId"`                       // 父条件 ID（NULL=根条件）
	CompositeType  string `gorm:"size:30" json:"compositeType"`                // 系统模板名，空=用户自定义
	TreeOperator   string `gorm:"size:10;default:and" json:"treeOperator"`     // and / or / not（子条件之间的逻辑）
}
