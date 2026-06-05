package collector

import (
	"log"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

func RunFullCollection() {
	log.Println("[collector] Starting full collection...")
	start := time.Now()

	// Phase 1: Update stock basics
	basicResult := RunBasicTask()
	log.Printf("[collector] Basic info: %d/%d success, %d failed",
		basicResult.Success, basicResult.Total, basicResult.Failed)

	// Phase 2: Fetch daily K-line
	kResult := RunDailyKTask()
	log.Printf("[collector] Daily K: %d/%d success, %d failed",
		kResult.Success, kResult.Total, kResult.Failed)

	// Log to import_logs
	logEntry := model.ImportLog{
		FileName:     "auto_collection",
		RowsImported: kResult.Success,
		Status:       "success",
		ImportedAt:   time.Now(),
	}
	if kResult.Failed > 0 || basicResult.Failed > 0 {
		logEntry.Status = "partial"
	}
	db.MySQL.Create(&logEntry)

	log.Printf("[collector] Full collection completed in %s", time.Since(start))
}
