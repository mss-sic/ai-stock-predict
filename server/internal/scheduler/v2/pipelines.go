package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/service"
)

// ── System Pipeline Definitions ──

// AfterCloseDataPipeline defines the post-market data collection DAG.
// Sequential: tushare_kline → market_daily_agg → limit_stats → market_sentiment → market_style
// (Sequential to avoid collector.Running global lock contention; will be parallelized after collector refactor)
var AfterCloseDataPipeline = Pipeline{
	Name:  "after_close_data",
	Label: "盘后数据采集流水线",
	Trigger: TriggerSpec{
		Cron:       "40 10 16 * * 1-5",
		TradingDay: true,
	},
	Stages: []PipelineStage{
		{
			Name:    "tushare_kline",
			Timeout: 45 * time.Minute,
			Retries: 1,
			Handler: wrapCollectorPhase("tushare_kline", "Tushare日K线采集"),
		},
		{
			Name:      "market_daily_agg",
			DependsOn: []string{"tushare_kline"},
			Timeout:   10 * time.Minute,
			Handler:   wrapCollectorPhase("market_daily_agg", "市场日聚合"),
		},
		{
			Name:      "limit_stats",
			DependsOn: []string{"market_daily_agg"},
			Timeout:   5 * time.Minute,
			Handler:   wrapCollectorPhase("limit_stats", "涨跌停统计"),
		},
		{
			Name:      "market_sentiment",
			DependsOn: []string{"limit_stats"},
			Timeout:   5 * time.Minute,
			Handler:   wrapCollectorPhase("market_sentiment", "市场情绪计算"),
		},
		{
			Name:      "market_style",
			DependsOn: []string{"market_sentiment"},
			Timeout:   5 * time.Minute,
			Handler:   wrapCollectorPhase("market_style", "市场风格计算"),
		},
	},
	OnComplete: EventDataReady,
}

// PreMarketDataPipeline defines the pre-market data collection.
// concept + cninfo run in parallel (different data sources, no shared lock).
var PreMarketDataPipeline = Pipeline{
	Name:  "pre_market_data",
	Label: "盘前数据采集流水线",
	Trigger: TriggerSpec{
		Cron:       "0 0 8 * * 1-5",
		TradingDay: true,
	},
	Stages: []PipelineStage{
		{
			Name:    "concept",
			Timeout: 10 * time.Minute,
			Handler: wrapCollectorPhase("concept", "概念板块采集"),
		},
		{
			Name:    "cninfo",
			Timeout: 10 * time.Minute,
			Handler: wrapCollectorPhase("cninfo", "巨潮公告采集"),
		},
	},
	OnComplete: "morning_partial",
}

// ── Standalone Task Definitions ──

// SystemTaskDefs returns all system-level TaskDefinitions not in pipelines.
func SystemTaskDefs() []*TaskDefinition {
	defs := []*TaskDefinition{
		// ── 盘中实时 ──
		{
			ID: "quote", Kind: KindPipeline, Label: "实时行情监控",
			Trigger: TriggerSpec{Cron: "0 */5 9-15 * * 1-5", TradingDay: true},
			Timeout: time.Minute, MaxConcurrent: 1,
			Handler: makeTaskHandler("quote", "实时行情"),
		},
		{
			ID: "kline_intraday", Kind: KindPipeline, Label: "盘中日K采集",
			Trigger: TriggerSpec{Cron: "0 */30 9-16 * * 1-5", TradingDay: true},
			Timeout: 5 * time.Minute, MaxConcurrent: 1,
			Handler: makeTaskHandler("kline", "盘中K线"),
		},
		{
			ID: "news", Kind: KindPipeline, Label: "资讯数据采集",
			Trigger: TriggerSpec{Cron: "0 */30 * * * *"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("news", "资讯"),
		},
		{
			ID: "macro_news", Kind: KindPipeline, Label: "宏观资讯采集",
			Trigger: TriggerSpec{Cron: "0 */30 * * * *"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("macro_news", "宏观资讯"),
		},

		// ── 盘后独立 ──
		{
			ID: "ths_hot", Kind: KindPipeline, Label: "同花顺热点采集",
			Trigger: TriggerSpec{Cron: "0 0 16 * * *"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("ths_hot", "同花顺热点"),
		},
		{
			ID: "northbound", Kind: KindPipeline, Label: "北向资金采集",
			Trigger: TriggerSpec{Cron: "0 30 15 * * 1-5", TradingDay: true},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("northbound", "北向资金"),
		},
		{
			ID: "dragon_tiger", Kind: KindPipeline, Label: "龙虎榜采集",
			Trigger: TriggerSpec{Cron: "0 0 17 * * *"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("dragon_tiger", "龙虎榜"),
		},
		{
			ID: "shareholder", Kind: KindPipeline, Label: "股东数据采集",
			Trigger: TriggerSpec{Cron: "0 0 17 * * *"},
			Timeout: 10 * time.Minute,
			Handler: makeTaskHandler("shareholder", "股东数据"),
		},
		{
			ID: "reports", Kind: KindPipeline, Label: "研报数据采集",
			Trigger: TriggerSpec{Cron: "0 0 18 * * *"},
			Timeout: 10 * time.Minute,
			Handler: makeTaskHandler("reports", "研报"),
		},
		{
			ID: "block_trade", Kind: KindPipeline, Label: "大宗交易采集",
			Trigger: TriggerSpec{Cron: "0 0 18 * * *"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("block_trade", "大宗交易"),
		},
		{
			ID: "tushare_indicator", Kind: KindPipeline, Label: "Tushare技术指标采集",
			Trigger: TriggerSpec{Cron: "0 0 16-20 * * 1-5", TradingDay: true},
			Timeout: 15 * time.Minute,
			Handler: makeTaskHandler("tushare_indicator", "技术指标"),
		},

		// ── 盘前独立 ──
		{
			ID: "margin", Kind: KindPipeline, Label: "融资融券采集",
			Trigger: TriggerSpec{Cron: "0 0 9 * * *"},
			Timeout: 5 * time.Minute,
			// On completion, emit morning_ready for live_trade_exec tasks
			Handler: makeTaskHandlerWithEvent("margin", "融资融券", EventMorningReady, ""),
		},


		// ── 实盘交易调度: 定义在此, 实例由 per-run RegisterStrategyRunTasks 创建 ──
		// Trigger.Cron 由 StrategyRun.AutoDailyCron / AutoTradeExecCron 覆盖
		// MinInterval 防止短时间内重复触发（cron解析容错机制）
		{
			ID: "live_daily_run", Kind: KindStrategy, Label: "盘后策略执行(信号生成)",
			Timeout: 30 * time.Minute, MaxConcurrent: 1,
			Handler:  makeLiveDailyRunHandler(),
		},
		{
			ID: "live_trade_exec", Kind: KindStrategy, Label: "交易执行",
			Timeout: 30 * time.Minute, MaxConcurrent: 1,
			Handler:  makeLiveTradeExecHandler(),
		},
		{
			ID: "live_position_patrol", Kind: KindStrategy, Label: "持仓巡检(止损止盈)",
			Timeout: 5 * time.Minute, MaxConcurrent: 1,
			Handler: makeLivePositionPatrolHandler(),
		},
		{
			ID: "order_sync", Kind: KindPipeline, Label: "订单状态同步(委托查询)",
			Trigger: TriggerSpec{Cron: "0 */30 9-15 * * 1-5", TradingDay: true},
			Timeout: 5 * time.Minute, MaxConcurrent: 1,
			Handler:  makeOrderSyncHandler(),
		},
		{
			ID: "live_snapshot", Kind: KindStrategy, Label: "盘后快照(净值记录)",
			Timeout: 5 * time.Minute, MaxConcurrent: 1,
			Handler: makeLiveSnapshotHandler(),
		},
		{
			ID: "live_position_refresh", Kind: KindStrategy, Label: "持仓市值刷新",
			Trigger: TriggerSpec{Cron: "0 */5 9-15 * * 1-5", TradingDay: true},
			Timeout: 5 * time.Minute, MaxConcurrent: 1,
			Handler: makeLivePositionRefreshHandler(),
		},
		{
			ID: "daily_t1_unlock", Kind: KindStrategy, Label: "T+1解锁(开盘前)",
			Trigger: TriggerSpec{Cron: "0 25 9 * * 1-5", TradingDay: true},
			Timeout: 1 * time.Minute, MaxConcurrent: 1,
			Handler: makeDailyT1UnlockHandler(),
		},
		// ── 周度/月度 ──
		{
			ID: "industry", Kind: KindPipeline, Label: "行业分类采集",
			Trigger: TriggerSpec{Cron: "0 0 2 * * 1"},
			Timeout: 10 * time.Minute,
			Handler: makeTaskHandler("industry", "行业分类"),
		},
		{
			ID: "full_sync", Kind: KindPipeline, Label: "股票列表同步",
			Trigger: TriggerSpec{Cron: "0 0 3 * * 1"},
			Timeout: 30 * time.Minute,
			Handler: makeTaskHandler("full_sync", "股票同步"),
		},
		{
			ID: "financial", Kind: KindPipeline, Label: "财务数据采集",
			Trigger: TriggerSpec{Cron: "0 0 4 * * 0"},
			Timeout: 15 * time.Minute,
			Handler: makeTaskHandler("financial", "财务数据"),
		},
		{
			ID: "concept_full", Kind: KindPipeline, Label: "概念全量重建",
			Trigger: TriggerSpec{Cron: "0 0 6 * * 0"},
			Timeout: 30 * time.Minute,
			Handler: makeTaskHandler("concept_full", "概念全量"),
		},
		{
			ID: "backfill_financial", Kind: KindPipeline, Label: "财报全量回填",
			Trigger: TriggerSpec{Cron: "0 0 3 1 * *"},
			Timeout: 60 * time.Minute,
			Handler: makeTaskHandler("backfill_financial", "财报回填"),
		},
		{
			ID: "backfill_shareholder", Kind: KindPipeline, Label: "股东全量回填",
			Trigger: TriggerSpec{Cron: "0 0 4 1 * *"},
			Timeout: 60 * time.Minute,
			Handler: makeTaskHandler("backfill_shareholder", "股东回填"),
		},
		{
			ID: "risk_scan", Kind: KindPipeline, Label: "风险扫描",
			Trigger: TriggerSpec{Cron: "0 5 * * * *"},
			Timeout: 5 * time.Minute, MaxConcurrent: 1,
			Handler: makeRiskScanHandler(),
		},
		{
			ID: "ai_score", Kind: KindPipeline, Label: "AI评分更新",
			Trigger: TriggerSpec{Cron: "0 0 20 * * 0"},
			Timeout: 30 * time.Minute,
			Handler: makeTaskHandler("ai_score", "AI评分"),
		},
		{
			ID: "unlock", Kind: KindPipeline, Label: "解禁数据采集",
			Trigger: TriggerSpec{Cron: "0 0 7 * * 1"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("unlock", "解禁数据"),
		},
		{
			ID: "dividend", Kind: KindPipeline, Label: "分红数据采集",
			Trigger: TriggerSpec{Cron: "0 0 5 * * 1"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("dividend", "分红数据"),
		},
		{
			ID: "ths_eps", Kind: KindPipeline, Label: "一致预期采集",
			Trigger: TriggerSpec{Cron: "0 0 6 * * 1"},
			Timeout: 5 * time.Minute,
			Handler: makeTaskHandler("ths_eps", "一致预期"),
		},
	}
	return defs
}

// ── Collector Adapters ──

func wrapCollectorPhase(phase, label string) func(ctx context.Context, logger *StructuredLogger) error {
	return func(ctx context.Context, logger *StructuredLogger) error {
		logger.Phase("running_collector", map[string]any{"phase": phase, "label": label})
		if err := collector.RunManualCollection([]string{phase}); err != nil {
			logger.Warn("collector_skip", map[string]any{"phase": phase, "reason": err.Error()})
			return nil
		}
		logger.Phase("collector_done", map[string]any{"phase": phase})
		return nil
	}
}

func makeTaskHandler(phase, label string) TaskHandler {
	return makeTaskHandlerWithEvent(phase, label, "", "")
}

// makeTaskHandlerWithEvent creates a handler that runs a collector phase, then emits a completion event.
func makeTaskHandlerWithEvent(phase, label, eventType, eventKey string) TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		logger.Phase("running", map[string]any{"phase": phase, "label": label})

		// Special handling for risk_scan (calls service directly, not collector)
		if phase == "risk_scan" {
			count, err := service.ScanUserHoldings()
			if err != nil {
				logger.Error("risk_scan_failed", err, nil)
				return err
			}
			logger.Info("risk_scan_complete", map[string]any{"alerts": count})
			return nil
		}

		if err := collector.RunManualCollection([]string{phase}); err != nil {
			logger.Warn("collector_busy", map[string]any{"phase": phase, "reason": err.Error()})
			return nil
		}
		// Emit completion event if specified
		if eventType != "" {
			key := eventKey
			if key == "" {
				key = phase
			}
			s := SchedulerFromContext(ctx)
			if s != nil {
				s.emitEvent(Event{
					ID:        NewEventID(eventType, key),
					Type:      eventType,
					Key:       key,
					Timestamp: time.Now(),
				})
			}
		}

		logger.Info("completed", map[string]any{"phase": phase})
		return nil
	}
}

// makeRiskScanHandler creates a handler for risk scanning.
func makeRiskScanHandler() TaskHandler {
	return makeTaskHandlerWithEvent("risk_scan", "风险扫描", "", "")
}

// ── Context helper ──

func SchedulerFromContext(ctx context.Context) *UnifiedScheduler {
	if s, ok := ctx.Value(ctxKeyScheduler).(*UnifiedScheduler); ok {
		return s
	}
	return nil
}


// ── Live Trading Handlers ──

func makeLiveDailyRunHandler() TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		runID := uint(0)
		if v, ok := inst.Params["runID"]; ok {
			switch id := v.(type) {
			case float64:
				runID = uint(id)
			case uint:
				runID = id
			case int:
				runID = uint(id)
			}
		}
		if runID == 0 {
			err := fmt.Errorf("runID is required but not provided in task params")
			logger.Error("live_daily_run_failed", err, nil)
			return err
		}
		logger.Phase("live_daily_run_start", map[string]any{"label": "盘后策略执行", "runID": runID})
		svc := service.NewLiveTradingService()
		tradeDate := time.Now().Format("2006-01-02")
		result, err := svc.RunDaily(tradeDate, "after_close", runID)
		if err != nil {
			logger.Error("live_daily_run_failed", err, nil)
			return err
		}
		logger.Info("live_daily_run_complete", map[string]any{
			"strategies": result.StrategiesRan,
			"signals":    result.SignalsGenerated,
			"trades":     result.TradesExecuted,
			"errors":     len(result.Errors),
		})

		// Emit data_ready event so downstream tasks know signals are ready
		s := SchedulerFromContext(ctx)
		if s != nil {
			s.emitEvent(Event{
				ID:        NewEventID(EventDataReady, "live_signals"),
				Type:      EventDataReady,
				Key:       "live_signals",
				Timestamp: time.Now(),
			})
		}
		return nil
	}
}

func makeLiveTradeExecHandler() TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		runID := uint(0)
		if v, ok := inst.Params["runID"]; ok {
			switch id := v.(type) {
			case float64:
				runID = uint(id)
			case uint:
				runID = id
			case int:
				runID = uint(id)
			}
		}
		logger.Phase("live_trade_exec_start", map[string]any{"label": "交易执行", "runID": runID})
		svc := service.NewPreMarketService(service.NewAIService())
		tradeDate := time.Now().Format("2006-01-02")
		result, err := svc.FinalizePreMarketForRun(tradeDate, runID)
		if err != nil {
			logger.Error("live_trade_exec_failed", err, nil)
			return err
		}
		logger.Info("live_trade_exec_complete", map[string]any{
			"confirmed": result.Confirmed,
			"rejected":  result.Rejected,
			"modified":  result.Modified,
			"total":     result.TotalSignals,
		})
		return nil
	}
}

func makeLivePositionPatrolHandler() TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		logger.Phase("position_patrol_start", map[string]any{"label": "持仓巡检"})
		svc := service.NewPreMarketService(service.NewAIService())
		tradeDate := time.Now().Format("2006-01-02")
		patrolResult := svc.PositionPatrol(tradeDate)
		logger.Info("position_patrol_complete", map[string]any{"alerts": len(patrolResult)})
		return nil
	}
}

func makeLiveSnapshotHandler() TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		logger.Phase("snapshot_start", map[string]any{"label": "盘后快照"})
		svc := service.NewLiveTradingService()
		tradeDate := time.Now().Format("2006-01-02")
		svc.TakeAllDailySnapshots(tradeDate)
		logger.Info("snapshot_complete", nil)
		return nil
	}
}

func makeOrderSyncHandler() TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		logger.Phase("order_sync_start", map[string]any{"label": "订单状态同步"})
		svc := service.NewOrderSyncService()
		result, err := svc.SyncAllPendingOrders(0)
		if err != nil {
			logger.Error("order_sync_failed", err, nil)
			return err
		}
		logger.Info("order_sync_complete", map[string]any{
			"scanned":        result.TotalScanned,
			"updated":        result.Updated,
			"executed":       result.Executed,
			"partialFilled":  result.PartialFilled,
			"cancelled":      result.Cancelled,
			"failed":         result.Failed,
			"skipped":        result.Skipped,
		})
		// Log detailed per-order results
		for _, l := range result.Logs {
			logger.Info("order_sync_detail", map[string]any{"msg": l})
		}
		return nil
	}
}

// ── Registration Helper ──

// RegisterSystemPipelines registers all system pipeline definitions and instances.
func RegisterSystemPipelines(s *UnifiedScheduler) {
	// After-close pipeline
	s.RegisterDefinition(&TaskDefinition{
		ID:      AfterCloseDataPipeline.Name,
		Kind:    KindPipeline,
		Label:   AfterCloseDataPipeline.Label,
		Trigger: AfterCloseDataPipeline.Trigger,
		Timeout: 60 * time.Minute,
		Handler: func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
			return s.RunPipeline(&AfterCloseDataPipeline)
		},
	})
	s.RegisterInstance(&TaskInstance{
		DefinitionID: AfterCloseDataPipeline.Name,
		Owner:        ResourceRef{Kind: "system", ID: 0},
		Enabled:      true,
		Trigger:      AfterCloseDataPipeline.Trigger,
	})

	// Pre-market pipeline
	s.RegisterDefinition(&TaskDefinition{
		ID:      PreMarketDataPipeline.Name,
		Kind:    KindPipeline,
		Label:   PreMarketDataPipeline.Label,
		Trigger: PreMarketDataPipeline.Trigger,
		Timeout: 30 * time.Minute,
		Handler: func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
			return s.RunPipeline(&PreMarketDataPipeline)
		},
	})
	s.RegisterInstance(&TaskInstance{
		DefinitionID: PreMarketDataPipeline.Name,
		Owner:        ResourceRef{Kind: "system", ID: 0},
		Enabled:      true,
		Trigger:      PreMarketDataPipeline.Trigger,
	})

	// Standalone tasks
	// Skip live trading defs — instances created per-run by RegisterStrategyRunTasks
	liveDefs := map[string]bool{"live_daily_run": true, "live_trade_exec": true, "live_position_patrol": true, "live_snapshot": true}
	for _, def := range SystemTaskDefs() {
		s.RegisterDefinition(def)
		if !liveDefs[def.ID] {
			s.RegisterInstance(&TaskInstance{
				DefinitionID: def.ID,
				Owner:        ResourceRef{Kind: "system", ID: 0},
				Enabled:      true,
				Trigger:      def.Trigger,
			})
		}
	}

	fmt.Printf("[scheduler-v2] system: %d pipelines + %d standalone tasks registered\n",
		2, len(SystemTaskDefs()))
}

func makeLivePositionRefreshHandler() TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		logger.Phase("position_refresh_start", map[string]any{"label": "持仓市值刷新"})
		svc := service.NewLiveTradingService()
		if err := svc.RefreshLivePositions(); err != nil {
			logger.Error("position_refresh_failed", err, nil)
			return err
		}
		logger.Info("position_refresh_complete", nil)
		return nil
	}
}

func makeDailyT1UnlockHandler() TaskHandler {
	return func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
		logger.Phase("t1_unlock_start", map[string]any{"label": "T+1解锁"})
		svc := service.NewLiveTradingService()
		if err := svc.ResetDailyBuyLock(); err != nil {
			logger.Error("t1_unlock_failed", err, nil)
			return err
		}
		logger.Info("t1_unlock_complete", nil)
		return nil
	}
}
