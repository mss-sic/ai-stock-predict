package main

import (
	"log"
	"os"
	"time"
	"os/signal"
	"syscall"

	"github.com/ai-stock-predict/server/internal/config"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/handler"
	schedv2 "github.com/ai-stock-predict/server/internal/scheduler/v2"
	"github.com/ai-stock-predict/server/internal/ws"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	// Release mode unless GIN_DEBUG=true (reduces 200+ route log lines)
	if os.Getenv("GIN_DEBUG") != "true" {
		gin.SetMode(gin.ReleaseMode)
	}
	cfg := config.Load()
	log.Printf("[startup] 智策投研 server starting on :%s", cfg.Port)

	db.InitPostgres(cfg.PostgresDSN)
	db.InitMySQL(cfg.MySQLDSN)
	db.AutoMigrate()
	db.EnsureManualTables()
	handler.EnsureAdminUser()

	// Clean orphaned backtest tasks from previous server run
	db.MySQL.Exec("UPDATE backtest_tasks SET status='cancelled', phase='服务器重启, 任务已中断', completed_at=NOW() WHERE status IN ('running','pending')")

	// Old scheduler deprecated — all scheduling migrated to v2 UnifiedScheduler

	// ── v2 Unified Scheduler (兼容模式，与旧调度器共存) ──
	schedV2 := schedv2.New(schedv2.Config{
		Mode:         "standalone",
		Workers:       4,
		InstanceID:   "stock-server",
		EvalInterval: 10 * time.Second,
	})
	// Legacy cron scheduler for scheduled_tasks (collector/data tasks)
	service.InitTaskManager()

	schedv2.RegisterSystemPipelines(schedV2)
	schedV2.Start()
	schedv2.SetGlobal(schedV2)
	schedV2.RestoreLiveTradingTasks()
	defer schedV2.Stop()

	r := gin.Default()
	r.MaxMultipartMemory = 100 << 20 // 100MB for large file imports
	r.Use(corsMiddleware())

	// ── Public routes (no auth) ──
	authH := handler.NewAuthHandler()
	costH := handler.NewCostHandler()
	// Internal API for algo team sync (no auth, internal only)
	internalH := handler.NewInternalHandler()
	r.POST("/api/v1/internal/predictions/sync", internalH.SyncPredictions)

	// Public data import API for external teams (API key auth)
	r.POST("/api/v1/data/import", handler.APIKeyAuth(), handler.DataImport)

	r.POST("/api/v1/auth/login", authH.Login)
	r.POST("/api/v1/auth/refresh", authH.Refresh)
	r.GET("/api/v1/indices", handler.GetIndices) // public index data

	// Market style compute (public for internal collector pipeline)
	marketPubH := handler.NewMarketStyleHandler()
	r.POST("/api/v1/market/compute-style", marketPubH.ComputeStyle)
	r.POST("/api/v1/market/bulk-compute", marketPubH.BulkCompute)
	r.POST("/api/v1/market/ai-interpretation", marketPubH.GenerateAIInterpretation)

	// Market sentiment (public)
	sentimentH := &handler.SentimentHandler{}
	capitalH := &handler.CapitalFlowHandler{}

	r.GET("/api/v1/sentiment/latest", sentimentH.GetLatest)
	r.GET("/api/v1/sentiment/history", sentimentH.GetHistory)
	r.GET("/api/v1/sentiment/detail", sentimentH.GetDetail)
	r.GET("/api/v1/sentiment/range", sentimentH.GetRange)
	r.GET("/api/v1/northbound", sentimentH.GetNorthbound)
	r.GET("/api/v1/northbound/minute", sentimentH.GetNorthboundMinute)
	r.GET("/api/v1/sentiment/index-kline/:code", sentimentH.GetIndexKLine)
	r.GET("/api/v1/sentiment/distribution", sentimentH.GetReturnDistribution)

	// Capital flow dashboard
	r.GET("/api/v1/capital-flow/summary", capitalH.GetSummary)
	r.GET("/api/v1/capital-flow/fund-top", capitalH.GetFundFlowTop)
	r.GET("/api/v1/capital-flow/northbound-trend", capitalH.GetNorthboundTrend)
	r.GET("/api/v1/capital-flow/daily", capitalH.GetFundFlowDaily)
	r.GET("/api/v1/capital-flow/margin-trend", capitalH.GetMarginTrend)
	r.GET("/api/v1/capital-flow/margin-top", capitalH.GetMarginTop)
	r.GET("/api/v1/capital-flow/stock-rank", capitalH.GetStockCapitalRank)
	r.GET("/api/v1/sentiment/turnover", sentimentH.GetMarketTurnover)
	r.GET("/api/v1/sentiment/limit-stats", sentimentH.GetLimitStats)
	r.GET("/api/v1/sentiment/fear-greed", sentimentH.GetFearGreedLatest)
	r.GET("/api/v1/sentiment/fear-greed/history", sentimentH.GetFearGreedHistory)


	// ── Concept Board routes ──
	boardH := handler.NewBoardHandler()
	r.GET("/api/v1/concept-boards", boardH.ConceptBoards)
			r.GET("/api/v1/concept-boards/:code/kline", boardH.ConceptBoardKline)
			r.GET("/api/v1/concept-boards/:code/stocks", boardH.ConceptBoardStocks)
	r.GET("/api/v1/concept-boards/heatmap", boardH.ConceptHeatmap)
	r.GET("/api/v1/industry/heatmap", boardH.IndustryHeatmap)
	r.GET("/api/v1/stocks/:code/concept-tags", boardH.StockConcepts)

	// ── Industry comparison routes ──
	industryH := handler.NewIndustryHandler()
	r.GET("/api/v1/industries", industryH.List)
	r.GET("/api/v1/industries/:name/stocks", industryH.Stocks)

	// ── Agent auto-trading routes (public, agent_token auth) ──
	agentHub := ws.NewHub()
	testMgr := ws.NewTestManager(agentHub)
	commander := ws.NewCommander()
	r.GET("/api/v1/ws/signals", ws.HandleAgentWS(agentHub))

	agentH := handler.NewAgentHandler(testMgr, commander)
	agentGroup := r.Group("/api/v1/live")
	agentGroup.GET("/pending-auto-signals", agentH.GetPendingAutoSignals)
	agentGroup.POST("/signals/:id/claim", agentH.ClaimSignal)
	agentGroup.POST("/signals/:id/report-result", agentH.ReportResult)
	agentGroup.GET("/signals/:id/detail", agentH.GetSignalDetail)
	agentGroup.GET("/agent/account-summary", agentH.GetAccountSummary)
	agentGroup.POST("/test-agent", agentH.TestAgent)
	agentGroup.POST("/agent-test-response", agentH.AgentTestResponse)
	agentGroup.GET("/agent-status", agentH.CheckAgentStatus)
	agentGroup.GET("/agent/account", agentH.GetAccount)
	agentGroup.POST("/agent/commands/:requestId/response", agentH.PostCommandResponse)
	agentGroup.POST("/agent/positions/sync", agentH.SyncPositions)
	agentGroup.POST("/agent/orders/sync", agentH.SyncOrders)

	// ── Protected routes ──
	api := r.Group("/api/v1")
	api.Use(handler.AuthMiddleware())
	{
		// Auth self-service
		api.POST("/auth/logout", authH.Logout)
		api.GET("/auth/me", authH.Me)
		api.POST("/auth/change-password", authH.ChangePassword)
		api.PUT("/auth/profile", authH.UpdateProfile)
		api.POST("/auth/heartbeat", authH.Heartbeat)
		api.GET("/auth/sessions", authH.GetSessions)
		api.DELETE("/auth/sessions/:id", authH.RevokeSession)

		// User: AI cost analysis
		api.GET("/cost/logs", costH.GetUserCostLogs)
		api.GET("/cost/summary", costH.GetUserCostSummary)
		api.GET("/cost/daily", costH.GetUserCostDaily)

		// Admin: user management
		admin := api.Group("/admin")
		admin.Use(handler.AdminMiddleware())
		{
			admin.GET("/users", authH.ListUsers)
			admin.POST("/users", authH.CreateUser)
			admin.POST("/users/reset-password", authH.ResetPassword)
			admin.POST("/users/toggle", authH.ToggleUser)
			admin.POST("/users/kick", authH.KickUser)
			admin.GET("/login-logs", authH.ListLoginLogs)
			admin.GET("/cost-logs", costH.GetCostLogs)
			admin.GET("/cost-summary", costH.GetCostSummary)
			admin.GET("/model-prices", costH.GetModelPrices)
			admin.PUT("/model-prices/:model_name", costH.UpdateModelPrice)
			// API Key management
		admin.GET("/api-keys", handler.ListAPIKeys)
		admin.POST("/api-keys", handler.CreateAPIKey)
		admin.PUT("/api-keys/:id", handler.UpdateAPIKey)
		admin.DELETE("/api-keys/:id", handler.DeleteAPIKey)

		admin.GET("/data-stats", handler.GetDataStats)
			admin.GET("/data-stats/:type/detail", handler.GetDataDetail)
			admin.POST("/risks/scan", handler.NewRiskHandler().Scan)

		// Scheduled Tasks
		schedV2Handler := schedv2.NewHandler(schedV2)
		schedV2Handler.RegisterRoutes(admin.Group("/scheduler/v2"))
		taskH := handler.NewTaskHandler()
			admin.GET("/scheduled-tasks", taskH.ListTasks)
			admin.POST("/scheduled-tasks", taskH.CreateTask)
			admin.PUT("/scheduled-tasks/:id", taskH.UpdateTask)
			admin.DELETE("/scheduled-tasks/:id", taskH.DeleteTask)
			admin.POST("/scheduled-tasks/:id/run", taskH.RunTaskNow)
			admin.POST("/scheduled-tasks/:id/repair", taskH.RepairTask)
			admin.POST("/scheduled-tasks/:id/reset", taskH.ResetTask)
			admin.POST("/scheduled-tasks/:id/toggle", taskH.ToggleTask)
			admin.POST("/scheduled-tasks/init-defaults", taskH.InitDefaults)
			admin.GET("/task-logs", taskH.ListLogs)

		// Scheduler execution history
		schedLogH := handler.NewSchedulerLogHandler()
		admin.GET("/scheduler/logs", schedLogH.ListLogs)
		admin.GET("/scheduler/logs/:id", schedLogH.GetLog)
		admin.GET("/scheduler/stats", schedLogH.GetStats)
		}

		// Stocks
		stockH := handler.NewStockHandler()
		api.GET("/stocks", stockH.List)
		api.GET("/stocks/market-snapshot", stockH.MarketSnapshot)
		api.GET("/stocks/ranking", stockH.Ranking)
		api.GET("/stocks/unusual", stockH.Unusual)
		api.GET("/stocks/appearance-stats", stockH.AppearanceStats)
		api.GET("/stocks/board-type-counts", stockH.BoardTypeCounts)
		api.GET("/stocks/:code", stockH.GetDetail)
		api.GET("/stocks/:code/kline", stockH.GetKLine)
		api.GET("/stocks/:code/indicator", stockH.GetIndicator)
		api.GET("/stocks/:code/signal", stockH.GetSignal)
		api.GET("/stocks/:code/financials", stockH.GetFinancials)
		api.GET("/stocks/:code/shareholders", stockH.GetShareholders)
		api.GET("/stocks/:code/news", stockH.GetNews)
		api.GET("/stocks/:code/reports", stockH.GetReports)
		api.GET("/stocks/:code/dragon-tiger", stockH.GetDragonTigerList)
		api.GET("/dragon-tiger", stockH.GetDailyDragonTigerList)
		api.GET("/dragon-tiger/enriched", stockH.GetDailyDragonTigerEnriched)
		api.GET("/dragon-tiger/:code/seats", stockH.GetDragonTigerSeats)
		api.GET("/stocks/:code/block-trades", stockH.GetBlockTrades)
		api.GET("/stocks/:code/announcements", stockH.GetCninfoAnnouncements)
		api.GET("/announcements", stockH.GetAllAnnouncements)
		api.GET("/stocks/:code/fund-flow-minute", stockH.GetFundFlowMinute)
		api.GET("/stocks/:code/fund-flow", stockH.GetStockFundFlow)
		api.GET("/stocks/:code/eps-forecast", stockH.GetThsEpsForecast)
		api.GET("/macro-news", stockH.GetMacroNews)
		api.GET("/macro-news/categories", stockH.GetMacroCategories)
		api.GET("/ths-hot-concepts", stockH.GetThsHotConceptStats)
		api.GET("/unlocks", stockH.GetAllFutureUnlocks)
		api.GET("/stocks/:code/unlocks", stockH.GetRestrictedUnlocks)
		api.POST("/stocks/:code/repair", stockH.RepairKLine)
		api.GET("/reports/industry", stockH.GetIndustryReports)
		api.GET("/reports/pdf", handler.ServeReportPDF)

		// Board

		// Market Style
		marketH := handler.NewMarketStyleHandler()
		api.GET("/market/style-curve", marketH.GetStyleCurve)
		api.GET("/market/daily-review", marketH.GetDailyReview)
		api.GET("/market/latest-style", marketH.GetLatestStyle)



		boardH := handler.NewBoardHandler()
		api.GET("/board/today", boardH.Today)
		api.GET("/board/dates", boardH.Dates)
		api.GET("/board/history", boardH.History)
		api.GET("/board/heatmap", boardH.Heatmap)
		api.GET("/board/heatmap-enriched", boardH.HeatmapEnriched)
		api.GET("/board/heatmap/:code", boardH.StockHeatmap)
			api.GET("/concept-boards/:code/analysis", boardH.GetConceptAnalysis)
			api.PUT("/concept-boards/analysis-prompt", boardH.UpdateConceptAnalysisPrompt)
		// Watchlist
		watchH := handler.NewWatchlistHandler()
		api.GET("/watchlist/groups", watchH.ListGroups)
		api.POST("/watchlist/groups", watchH.CreateGroup)
		api.PUT("/watchlist/groups/:id", watchH.RenameGroup)
		api.DELETE("/watchlist/groups/:id", watchH.DeleteGroup)
		api.PUT("/watchlist/groups/reorder", watchH.ReorderGroups)
		api.GET("/watchlist", watchH.ListStocks)
		api.POST("/watchlist", watchH.Add)
		api.DELETE("/watchlist/:code", watchH.Remove)
		api.DELETE("/watchlist", watchH.Clear)
		api.PUT("/watchlist/:code/move", watchH.MoveStock)

		// Holdings + Account
		holdingH := handler.NewHoldingHandler()
		api.GET("/holdings/summary", holdingH.Summary)
		api.GET("/holdings", holdingH.List)
		api.POST("/holdings", holdingH.Create)
		api.PUT("/holdings/:id", holdingH.Update)
		api.DELETE("/holdings/:id", holdingH.Delete)
		api.GET("/holdings/account", holdingH.Account)
		api.GET("/holdings/accounts-overview", holdingH.AccountsOverview)
		api.PUT("/holdings/account", holdingH.UpdateAccount)
		api.GET("/holdings/trades", holdingH.TradeRecords)

		// Risk alerts
		riskH := handler.NewRiskHandler()
		api.GET("/risks", riskH.List)
		api.GET("/risk/dashboard", riskH.Dashboard)
		api.GET("/risk/aggregated", riskH.ListAggregated)
		api.GET("/risk/alerts", riskH.ListAlerts)
		api.GET("/risk/alerts/:id", riskH.GetAlertDetail)
		api.PUT("/risk/alerts/:id/acknowledge", riskH.AcknowledgeAlert)
		api.GET("/risk/rules", riskH.ListRules)
		api.PUT("/risk/rules/:key", riskH.UpdateRule)
		api.GET("/risk/snapshots", riskH.ListSnapshots)
		api.GET("/risk/circuit-breaker", riskH.CircuitBreakerStatus)
		api.PUT("/risks/:id/ignore", riskH.Ignore)

		// Strategy
		strategyH := handler.NewStrategyHandler()
		handler.SetDefaultStrategyHandler(strategyH)

		// PK Events
		pkH := handler.NewPkHandler()
		api.POST("/pk/events", pkH.CreateEvent)
		api.GET("/pk/events", pkH.ListEvents)
		api.GET("/pk/events/:id", pkH.GetEvent)
		api.PUT("/pk/events/:id", pkH.UpdateEvent)
		api.POST("/pk/events/:id/start", pkH.StartEvent)
		api.POST("/pk/events/:id/close", pkH.CloseEvent)
		api.DELETE("/pk/events/:id", pkH.DeleteEvent)
		api.POST("/pk/events/:id/join", pkH.JoinEvent)
		api.GET("/pk/entries/:entryId/detail", pkH.EntryDetail)
		api.GET("/pk/active-notice", pkH.ActiveNotice)
		api.GET("/strategies", strategyH.List)
		api.POST("/strategies", strategyH.Create)
		api.PUT("/strategies/:id", strategyH.Update)
		api.DELETE("/strategies/:id", strategyH.Delete)
		api.PUT("/strategies/reorder", strategyH.Reorder)
		api.GET("/strategies/:id/conditions", strategyH.ListConditions)
		api.PUT("/strategies/:id/conditions", strategyH.SaveConditions)
		api.POST("/strategies/ai-generate", strategyH.AIGenerate)
		api.POST("/strategies/optimize-prompt", strategyH.OptimizePrompt)
		api.GET("/strategies/indicators", strategyH.Indicators)
		api.GET("/strategies/indicator-guide", strategyH.IndicatorGuide)
		api.POST("/strategies/test-indicator", strategyH.TestIndicator)
		api.GET("/strategies/:id/indicators", strategyH.ListStrategyIndicators)
		api.PUT("/strategies/:id/indicators/toggle", strategyH.ToggleIndicatorCondition)
		api.PUT("/strategies/:id/indicators/bulk-toggle", strategyH.BulkToggleIndicator)
		api.POST("/strategies/:id/backtest", strategyH.RunBacktest)
		api.POST("/strategies/:id/backtest/start", strategyH.StartBacktest)
		api.GET("/strategies/:id/backtest/status/:taskId", strategyH.BacktestStatus)
		api.GET("/strategies/:id/backtest/stream/:taskId", strategyH.BacktestStream)
		api.POST("/strategies/:id/backtest/cancel/:taskId", strategyH.CancelBacktest)
		api.GET("/strategies/:id/backtest/tasks", strategyH.BacktestTasks)
		api.DELETE("/strategies/:id/backtest/tasks/:taskId", strategyH.DeleteBacktestTask)
		api.GET("/strategies/:id/backtest/tasks/:taskId/logs", strategyH.BacktestTaskLogs)
		api.GET("/strategies/:id/backtest/tasks/:taskId/snapshots", strategyH.BacktestTaskSnapshots)
		api.GET("/strategies/:id/backtest/tasks/:taskId/stock-analysis", strategyH.BacktestStockAnalysis)
		api.GET("/strategies/backtest-history", strategyH.BacktestHistory)
		api.GET("/strategies/backtest-history/:id", strategyH.GetBacktestResult)
		api.DELETE("/strategies/backtest-history/:id", strategyH.DeleteBacktestResult)
		api.GET("/strategies/:id/orchestration", strategyH.GetOrchestration)
		api.PUT("/strategies/:id/orchestration", strategyH.SaveOrchestration)
		api.GET("/strategies/templates", strategyH.ListTemplates)
		api.POST("/strategies/templates", strategyH.CreateTemplate)
		api.GET("/strategies/:id/ai-decisions", strategyH.ListAIDecisions)
		api.POST("/strategies/:id/ai-review", strategyH.AIReview)
		api.GET("/strategies/stock-pool", strategyH.StockPool)

		// Agent WebSocket hub (local auto-trading)


	// Live Trading (实盘交易)
		liveH := handler.NewLiveTradingHandlerWithHub(agentHub)
		handler.RegisterLiveTradingRoutes(api.Group("/live"), liveH)

		// Inject hub+commander into BrokerService for lobster support
		brokerSvc := service.NewBrokerService()
		brokerSvc.SetHubAndCommander(agentHub, commander)
		service.SetGlobalBrokerService(brokerSvc)
		liveH.SetBrokerService(brokerSvc)

		// Pre-Market Finalization (盘前决策) + Notifications
		preMarketH := handler.NewPreMarketHandler(service.NewAIService())
		preMarketGroup := api.Group("/live")
		preMarketGroup.POST("/trade-exec", preMarketH.FinalizePreMarket)
		preMarketGroup.GET("/trade-exec/tasks/latest", preMarketH.GetLatestTask)
		preMarketGroup.GET("/trade-exec/tasks/:id", preMarketH.GetTaskStatus)
		preMarketGroup.GET("/trade-exec/decisions", preMarketH.GetPreMarketDecisions)
		preMarketGroup.GET("/notification-configs", preMarketH.ListNotificationConfigs)
		preMarketGroup.POST("/notification-configs", preMarketH.CreateNotificationConfig)
		preMarketGroup.PUT("/notification-configs/:id", preMarketH.UpdateNotificationConfig)
		preMarketGroup.DELETE("/notification-configs/:id", preMarketH.DeleteNotificationConfig)
		preMarketGroup.POST("/notification-configs/:id/test", preMarketH.TestNotificationConfig)
		// Agent auto-trading REST API (local agent polling endpoints)



		// Collector
		collectorH := handler.NewCollectorHandler()
		api.GET("/collector/stream", collectorH.Stream)
		api.GET("/collector/history", collectorH.History)
		api.DELETE("/collector/history/clear", collectorH.ClearHistory)
		api.POST("/collector/trigger", collectorH.Trigger)
		api.GET("/collector/status", collectorH.Status)
		api.PUT("/collector/schedule", collectorH.UpdateSchedule)
		api.POST("/collector/stock/:code", collectorH.StockCollect)
		api.GET("/collector/stock/:code/:phase", collectorH.CollectStockPhaseSSE)
		api.POST("/collector/realtime", collectorH.RealtimeQuotes)
		api.POST("/collector/realtime/:code", collectorH.RealtimeQuoteSingle)
		api.GET("/collector/reports/:code", collectorH.CollectReports)

		// Forecast
		forecastH := handler.NewForecastHandler()
		api.GET("/forecast/:code", forecastH.Predict)

		// AI
		aiH := handler.NewAIHandler()
		api.POST("/ai/analyze", aiH.Analyze)
		api.POST("/ai/analyze/stream", aiH.AnalyzeStream)
		api.GET("/ai/history/:code", aiH.GetHistory)
		api.DELETE("/ai/history/:code", aiH.ClearHistory)
		api.GET("/ai/score/:code", aiH.GetScore)
		api.POST("/ai/score/:code", aiH.RunScore)
		api.GET("/ai/profile/:code", aiH.GetProfile)
		api.POST("/ai/profile/:code", aiH.RunProfile)
		api.POST("/ai/profile-batch", aiH.RunProfileBatch)
		api.GET("/ai/system-configs", aiH.GetSystemConfigs)
		api.GET("/ai/system-config-vars", aiH.GetSystemConfigVars)
		api.GET("/ai/system-configs/:scene", aiH.GetSystemConfig)
		api.PUT("/ai/system-configs/:scene", aiH.UpdateSystemConfig)

		// Import
		importH := handler.NewImportHandler(aiH)
		api.POST("/import/excel", importH.Upload)
		api.POST("/import/kline", importH.UploadKline)
		api.POST("/import/profile", importH.UploadProfile)
		api.GET("/import/history", importH.History)

		// Settings (per-user)
		settingsH := handler.NewSettingsHandler()
		api.GET("/settings/ai", settingsH.GetAIConfig)
		api.PUT("/settings/ai", settingsH.SaveAIConfig)
		api.POST("/settings/ai/test", settingsH.TestAIConnection)
		api.POST("/settings/ai/models", settingsH.ListModels)

		// Predictions
		predH := handler.NewPredictionHandler()
		// Prediction screening (cross-sectional ranking)
		api.GET("/prediction/screening", handler.PredictionScreening)

		api.POST("/prediction/:code", predH.RunAll)
		api.POST("/prediction/batch", predH.Batch)
		api.GET("/prediction/:code/hitrate", predH.HitRate)
		api.GET("/prediction/:code", predH.GetResult)
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	log.Printf("[startup] ready — :%s | postgres+mysql | scheduler-v2 | %d risk rules",
		cfg.Port, len(service.GetEngine().Rules()))
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
