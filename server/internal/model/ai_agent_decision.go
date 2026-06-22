package model

import "time"

// AIAgentDecision records each AI agent review/decision event.
type AIAgentDecision struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	StrategyID      uint      `gorm:"index" json:"strategyId"`
	BacktestTaskID  *uint     `gorm:"index" json:"backtestTaskId"` // 回测关联，实盘为 NULL
	TradeDate       string    `gorm:"size:10" json:"tradeDate"`
	MarketScore     float64   `json:"marketScore"`  // MarketSentiment.CompositeScore
	MarketBias      float64   `json:"marketBias"`   // AI 输出的风险偏好乘数
	CandidatesIn    int       `json:"candidatesIn"` // 引擎输入候选数
	CandidatesOut   int       `json:"candidatesOut"` // AI 确认后候选数
	Reasoning       string    `gorm:"type:text" json:"reasoning"`  // AI 推理过程
	Actions         JSONArray `gorm:"type:jsonb" json:"actions"`   // [{code, action, confidence, reason}]
	OverridesApplied bool     `json:"overridesApplied"` // 是否有风控覆盖
	CreatedAt       time.Time `json:"createdAt"`
}

func (AIAgentDecision) TableName() string { return "ai_agent_decisions" }
