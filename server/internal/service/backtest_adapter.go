package service

import (
	"encoding/json"
	"log"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// KlineCacheAdapter adapts the handler's KlineCache + IndicatorCache to BacktestDataProvider.
// Uses Go structural typing — the handler's KlineCache and IndicatorCache implement these interfaces.
type KlineCacheAdapter struct {
	KC interface {
		GetClose(string, string) float64
		GetOpen(string, string) float64
		GetDailyChange(string, string) float64
	}
	IC interface {
		Get(string, string, string) (float64, bool)
	}
	dates   []string
	dateIdx map[string]int
}

// NewKlineCacheAdapter creates a new adapter wrapping handler caches.
func NewKlineCacheAdapter(
	kc interface {
		GetClose(string, string) float64
		GetOpen(string, string) float64
		GetDailyChange(string, string) float64
	},
	ic interface {
		Get(string, string, string) (float64, bool)
	},
	dates []string,
	dateIdx map[string]int,
) *KlineCacheAdapter {
	return &KlineCacheAdapter{KC: kc, IC: ic, dates: dates, dateIdx: dateIdx}
}

func (a *KlineCacheAdapter) GetClose(code, date string) float64 { return a.KC.GetClose(code, date) }
func (a *KlineCacheAdapter) GetOpen(code, date string) float64  { return a.KC.GetOpen(code, date) }
func (a *KlineCacheAdapter) GetDailyChange(code, date string) float64 {
	return a.KC.GetDailyChange(code, date)
}
func (a *KlineCacheAdapter) GetIndicatorValue(ind, code, date string) (float64, bool) {
	return a.IC.Get(ind, code, date)
}
func (a *KlineCacheAdapter) GetNextDate(date string) string {
	idx := a.dateIdx[date]
	if idx >= 0 && idx+1 < len(a.dates) {
		return a.dates[idx+1]
	}
	return date
}
func (a *KlineCacheAdapter) Dates() []string { return a.dates }

// DBPersister implements BacktestPersister using MySQL for a specific task.
type DBPersister struct {
	TaskID     uint
	StrategyID uint
	UserID     uint
}

func NewDBPersister(taskID, strategyID, userID uint) *DBPersister {
	return &DBPersister{TaskID: taskID, StrategyID: strategyID, UserID: userID}
}

func (p *DBPersister) SaveSignal(sig *BacktestSignalRecord) error {
	record := model.BacktestSignal{
		TaskID:        p.TaskID,
		StrategyID:    p.StrategyID,
		UserID:        p.UserID,
		SignalDate:    sig.SignalDate,
		ExecDate:      sig.ExecDate,
		StockCode:     sig.StockCode,
		StockName:     sig.StockName,
		ActionType:    sig.ActionType,
		PlannedPrice:  sig.PlannedPrice,
		PlannedQty:    sig.PlannedQty,
		PlannedAmount: sig.PlannedAmount,
		ExecPrice:     sig.ExecPrice,
		ExecQty:       sig.ExecQty,
		ExecAmount:    sig.ExecAmount,
		Status:        sig.Status,
		SkipReason:    sig.SkipReason,
		Reason:        sig.Reason,
		Pnl:           sig.Pnl,
		PnlPct:        sig.PnlPct,
	}
	return db.MySQL.Save(&record).Error
}

func (p *DBPersister) SaveSnapshot(snap *BacktestSnapshot) error {
	posJSON := "[]"
	if len(snap.Positions) > 0 {
		b, _ := json.Marshal(snap.Positions)
		posJSON = string(b)
	}
	record := model.BacktestDailySnapshot{
		TaskID:           p.TaskID,
		StrategyID:       p.StrategyID,
		UserID:           p.UserID,
		Date:             snap.Date,
		DayIndex:         snap.DayIndex,
		Cash:             snap.Cash,
		TotalEquity:      snap.TotalEquity,
		DailyReturn:      snap.DailyReturn,
		CumulativeReturn: snap.CumulativeReturn,
		PositionCount:    snap.PositionCount,
		Positions:        posJSON,
		MaxDrawdown:      snap.MaxDrawdown,
	}
	return db.MySQL.Create(&record).Error
}

func (p *DBPersister) SaveLog(entry *BacktestLogEntry) error {
	detailJSON := "{}"
	if entry.Detail != nil {
		b, _ := json.Marshal(entry.Detail)
		detailJSON = string(b)
	}
	record := model.BacktestExecutionLog{
		TaskID:     p.TaskID,
		StrategyID: p.StrategyID,
		UserID:     p.UserID,
		Date:       entry.Date,
		Seq:        entry.Seq,
		LogType:    entry.LogType,
		Level:      entry.Level,
		StockCode:  entry.StockCode,
		StockName:  entry.StockName,
		Message:    entry.Message,
		Detail:     detailJSON,
	}
	return db.MySQL.Create(&record).Error
}

func (p *DBPersister) UpdateProgress(day, total int, phase string) {
	pct := 0.0
	if total > 0 {
		pct = float64(day) / float64(total) * 100
	}
	db.MySQL.Model(&model.BacktestTask{}).Where("id = ?", p.TaskID).Updates(map[string]interface{}{
		"current_day":  day,
		"progress_pct": pct,
		"phase":        phase,
	})
}

func (p *DBPersister) IsCancelled() bool {
	var task model.BacktestTask
	if err := db.MySQL.First(&task, p.TaskID).Error; err != nil {
		return true
	}
	return task.Status == "cancelled"
}

// BuildEngineConfig converts a Strategy model to BacktestConfig.
func BuildEngineConfig(s *model.Strategy) BacktestConfig {
	cfg := DefaultBacktestConfig()
	if s.InitialCapital > 0 {
		cfg.InitialCapital = s.InitialCapital
	}
	if s.MaxHoldings > 0 {
		cfg.MaxHoldings = s.MaxHoldings
	}
	if s.BuyPositionPct > 0 {
		cfg.BuyPositionPct = s.BuyPositionPct
	}
	if s.AddPositionPct > 0 {
		cfg.AddPositionPct = s.AddPositionPct
	}
	if s.ReducePositionPct > 0 {
		cfg.ReducePositionPct = s.ReducePositionPct
	}
	cfg.StopProfit = s.StopProfit
	cfg.StopLoss = s.StopLoss
	cfg.EnableDipBuy = s.EnableDipBuy
	cfg.DipBuyThreshold = s.DipBuyThreshold
	cfg.DipBuyAmountPct = s.DipBuyAmountPct
	cfg.DipTargetReturn = s.DipTargetReturn
	cfg.DipMaxHoldDays = s.DipMaxHoldDays
	cfg.DipCooldownDays = s.DipCooldownDays
	cfg.EnableGrid = s.EnableGrid
	cfg.GridLevels = s.GridLevels
	cfg.GridLotPct = s.GridLotPct
cfg.EnableDynamicSizing = s.EnableDynamicSizing
cfg.MaxTotalPosition = s.MaxTotalPosition
cfg.DailyBuyLimit = s.DailyBuyLimit
cfg.PositionConcentrationLimit = s.PositionConcentrationLimit
cfg.AggressiveThreshold = s.AggressiveThreshold
cfg.DefensiveThreshold = s.DefensiveThreshold
cfg.MarketCompositeMin = s.MarketCompositeMin
cfg.MarketPositionBias = s.MarketPositionBias
cfg.ScoringConfig = s.ScoringConfig
	return cfg
}

// ConvertToConditionDefs converts model conditions to engine ConditionDefs filtered by type.
func ConvertToConditionDefs(conds []model.StrategyCondition, condType string) []ConditionDef {
	result := make([]ConditionDef, 0, len(conds))
	for _, c := range conds {
		if c.CondType == condType && c.Enabled {
			lg := c.LogicGroup
			if lg == 0 {
				lg = 1
			}
			result = append(result, ConditionDef{
				Indicator:  c.Indicator,
				Operator:   c.Operator,
				Value:      c.Value,
				LogicGroup: lg,
			})
		}
	}
	return result
}

func init() {
	log.Printf("[backtest_engine] registered")
}
