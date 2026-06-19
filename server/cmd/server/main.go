package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ai-stock-predict/server/internal/config"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/handler"
	"github.com/ai-stock-predict/server/internal/scheduler"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	log.Printf("智策投研 server starting on :%s", cfg.Port)

	db.InitPostgres(cfg.PostgresDSN)
	db.InitMySQL(cfg.MySQLDSN)
	db.AutoMigrate()
	db.EnsureManualTables()
	handler.EnsureAdminUser()

	// Clean orphaned backtest tasks from previous server run
	db.MySQL.Exec("UPDATE backtest_tasks SET status='cancelled', phase='服务器重启, 任务已中断', completed_at=NOW() WHERE status IN ('running','pending')")

	sched := scheduler.New(cfg.CronExpr)
	sched.Start()
	defer sched.Stop()

	// Initialize task manager with default scheduled tasks
	service.InitTaskManager()
	service.InitializeDefaultTasks()

	r := gin.Default()
	r.MaxMultipartMemory = 100 << 20 // 100MB for large file imports
	r.Use(corsMiddleware())

	// ── Public routes (no auth) ──
	authH := handler.NewAuthHandler()
	costH := handler.NewCostHandler()
	// Internal API for algo team sync (no auth, internal only)
	internalH := handler.NewInternalHandler()
	r.POST("/api/v1/internal/predictions/sync", internalH.SyncPredictions)


	r.POST("/api/v1/auth/login", authH.Login)
	r.POST("/api/v1/auth/refresh", authH.Refresh)
	r.GET("/api/v1/indices", handler.GetIndices) // public index data

	// ── Concept Board routes ──
	boardH := handler.NewBoardHandler()
	r.GET("/api/v1/concept-boards", boardH.ConceptBoards)
	r.GET("/api/v1/concept-boards/:code/stocks", boardH.ConceptBoardStocks)
	r.GET("/api/v1/concept-boards/heatmap", boardH.ConceptHeatmap)
	r.GET("/api/v1/stocks/:code/concept-tags", boardH.StockConcepts)

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
			admin.GET("/data-stats", handler.GetDataStats)
			admin.GET("/data-stats/:type/detail", handler.GetDataDetail)
			admin.POST("/risks/scan", handler.NewRiskHandler().Scan)

		// Scheduled Tasks
		taskH := handler.NewTaskHandler()
			admin.GET("/scheduled-tasks", taskH.ListTasks)
			admin.POST("/scheduled-tasks", taskH.CreateTask)
			admin.PUT("/scheduled-tasks/:id", taskH.UpdateTask)
			admin.DELETE("/scheduled-tasks/:id", taskH.DeleteTask)
			admin.POST("/scheduled-tasks/:id/run", taskH.RunTaskNow)
			admin.POST("/scheduled-tasks/:id/reset", taskH.ResetTask)
			admin.POST("/scheduled-tasks/:id/toggle", taskH.ToggleTask)
			admin.POST("/scheduled-tasks/init-defaults", taskH.InitDefaults)
			admin.GET("/task-logs", taskH.ListLogs)
		}

		// Stocks
		stockH := handler.NewStockHandler()
		api.GET("/stocks", stockH.List)
		api.GET("/stocks/:code", stockH.GetDetail)
		api.GET("/stocks/:code/kline", stockH.GetKLine)
		api.GET("/stocks/:code/indicator", stockH.GetIndicator)
		api.GET("/stocks/:code/signal", stockH.GetSignal)
		api.GET("/stocks/:code/financials", stockH.GetFinancials)
		api.GET("/stocks/:code/shareholders", stockH.GetShareholders)
		api.GET("/stocks/:code/news", stockH.GetNews)
		api.GET("/stocks/:code/reports", stockH.GetReports)
		api.POST("/stocks/:code/repair", stockH.RepairKLine)
		api.GET("/reports/industry", stockH.GetIndustryReports)
		api.GET("/reports/pdf", handler.ServeReportPDF)

		// Board
		boardH := handler.NewBoardHandler()
		api.GET("/board/today", boardH.Today)
		api.GET("/board/dates", boardH.Dates)
		api.GET("/board/history", boardH.History)
		api.GET("/board/heatmap", boardH.Heatmap)
		api.GET("/board/heatmap-enriched", boardH.HeatmapEnriched)
		api.GET("/board/heatmap/:code", boardH.StockHeatmap)
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
		api.PUT("/holdings/account", holdingH.UpdateAccount)
		api.GET("/holdings/trades", holdingH.TradeRecords)

		// Risk alerts
		riskH := handler.NewRiskHandler()
		api.GET("/risks", riskH.List)
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
		api.DELETE("/strategies/backtest-history/:id", strategyH.DeleteBacktestResult)
		api.GET("/strategies/stock-pool", strategyH.StockPool)

		// Collector
		collectorH := handler.NewCollectorHandler(sched)
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
		sched.Stop()
		os.Exit(0)
	}()

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
