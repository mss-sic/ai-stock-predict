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
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	log.Printf("智策投研 server starting on :%s", cfg.Port)

	db.InitPostgres(cfg.PostgresDSN)
	db.InitMySQL(cfg.MySQLDSN)
	db.AutoMigrate()
	db.EnsureManualTables()

	sched := scheduler.New(cfg.CronExpr)
	sched.Start()
	defer sched.Stop()

	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api/v1")
	{
		stockH := handler.NewStockHandler()
		api.GET("/stocks", stockH.List)
		api.GET("/stocks/:code", stockH.GetDetail)
		api.GET("/stocks/:code/kline", stockH.GetKLine)
		api.GET("/stocks/:code/indicator", stockH.GetIndicator)
		api.GET("/stocks/:code/signal", stockH.GetSignal)
		api.GET("/stocks/:code/quote", stockH.GetQuote)
		api.GET("/stocks/:code/financials", stockH.GetFinancials)
		api.GET("/stocks/:code/shareholders", stockH.GetShareholders)
		api.GET("/stocks/:code/news", stockH.GetNews)
		api.GET("/stocks/:code/reports", stockH.GetReports)
		api.GET("/reports/industry", stockH.GetIndustryReports)
		api.GET("/indices", handler.GetIndices)
		api.GET("/reports/pdf", handler.ServeReportPDF)

		boardH := handler.NewBoardHandler()
		api.GET("/board/today", boardH.Today)
		api.GET("/board/dates", boardH.Dates)
		api.GET("/board/history", boardH.History)
		api.GET("/board/heatmap", boardH.Heatmap)
		api.GET("/board/heatmap-enriched", boardH.HeatmapEnriched)
		api.GET("/board/heatmap/:code", boardH.StockHeatmap)

		importH := handler.NewImportHandler()
		api.POST("/import/excel", importH.Upload)
		api.GET("/import/history", importH.History)

		watchH := handler.NewWatchlistHandler()
		api.GET("/watchlist", watchH.List)
		api.POST("/watchlist", watchH.Add)
		api.DELETE("/watchlist/:code", watchH.Remove)

		collectorH := handler.NewCollectorHandler(sched)
		api.GET("/collector/stream", collectorH.Stream)
		api.GET("/collector/history", collectorH.History)
		api.POST("/collector/trigger", collectorH.Trigger)
		api.GET("/collector/status", collectorH.Status)
		api.PUT("/collector/schedule", collectorH.UpdateSchedule)
		api.POST("/collector/stock/:code", collectorH.StockCollect)
		api.GET("/collector/reports/:code", collectorH.CollectReports)

		forecastH := handler.NewForecastHandler()
		api.GET("/forecast/:code", forecastH.Predict)

		// AI — fixed routes first, then parameterized
		aiH := handler.NewAIHandler()
		api.POST("/ai/analyze", aiH.Analyze)
		api.POST("/ai/analyze/stream", aiH.AnalyzeStream)
		api.GET("/ai/history/:code", aiH.GetHistory)
		api.DELETE("/ai/history/:code", aiH.ClearHistory)
		api.GET("/ai/score/:code", aiH.GetScore)
		api.POST("/ai/score/:code", aiH.RunScore)

		settingsH := handler.NewSettingsHandler()
		api.GET("/settings/ai", settingsH.GetAIConfig)
		api.PUT("/settings/ai", settingsH.SaveAIConfig)
		api.POST("/settings/ai/test", settingsH.TestAIConnection)
		api.POST("/settings/ai/models", settingsH.ListModels)

		predH := handler.NewPredictionHandler()
		api.POST("/prediction/:code", predH.RunAll)
		api.POST("/prediction/batch", predH.Batch)
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
