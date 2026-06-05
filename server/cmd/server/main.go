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

	// Initialize databases
	db.InitPostgres(cfg.PostgresDSN)
	db.InitMySQL(cfg.MySQLDSN)
	db.AutoMigrate()

	// Initialize scheduler
	sched := scheduler.New(cfg.CronExpr)
	sched.Start()
	defer sched.Stop()

	// Setup Gin
	r := gin.Default()
	r.Use(corsMiddleware())

	api := r.Group("/api/v1")
	{
		// Stock endpoints
		stockH := handler.NewStockHandler()
		api.GET("/stocks", stockH.List)
		api.GET("/stocks/:code", stockH.GetDetail)
		api.GET("/stocks/:code/kline", stockH.GetKLine)
		api.GET("/stocks/:code/indicator", stockH.GetIndicator)

		// Board endpoints
		boardH := handler.NewBoardHandler()
		api.GET("/board/today", boardH.Today)
		api.GET("/board/history", boardH.History)
		api.GET("/board/heatmap", boardH.Heatmap)
		api.GET("/board/heatmap/:code", boardH.StockHeatmap)

		// Import
		importH := handler.NewImportHandler()
		api.POST("/import/excel", importH.Upload)

		// Collector
		collectorH := handler.NewCollectorHandler(sched)
		api.POST("/collector/trigger", collectorH.Trigger)
		api.GET("/collector/status", collectorH.Status)
		api.PUT("/collector/schedule", collectorH.UpdateSchedule)

		// Forecast & AI
		forecastH := handler.NewForecastHandler()
		api.GET("/forecast/:code", forecastH.Predict)

		aiH := handler.NewAIHandler()
		api.POST("/ai/analyze", aiH.Analyze)
	}

	// Graceful shutdown
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
