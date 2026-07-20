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
	
	log.Println("Starting FULL concept rebuild...")
	if err := collector.CollectConceptsFull(); err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	log.Println("Done!")
}
