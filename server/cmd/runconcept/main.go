package main

import (
	"log"
	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/config"
	"github.com/ai-stock-predict/server/internal/db"
)

func main() {
	cfg := config.Load()
	log.Println("Connecting to databases...")
	db.InitPostgres(cfg.PostgresDSN)
	db.InitMySQL(cfg.MySQLDSN)
	
	log.Println("Starting concept collection...")
	if err := collector.CollectConcepts(); err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	var boardCount, stockCount int64
	db.PG.Raw("SELECT COUNT(*) FROM concept_boards").Scan(&boardCount)
	db.PG.Raw("SELECT COUNT(*) FROM stock_concepts").Scan(&stockCount)
	log.Printf("Done! Boards: %d, Stock mappings: %d", boardCount, stockCount)
}
