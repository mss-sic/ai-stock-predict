package db

import (
	"log"

	"github.com/ai-stock-predict/server/internal/model"
)

func AutoMigrate() {
	if PG != nil {
		// Create stock_signals manually (in case AutoMigrate ordering fails)
		PG.Exec(`CREATE TABLE IF NOT EXISTS stock_signals (
			id SERIAL PRIMARY KEY,
			code VARCHAR(10) UNIQUE,
			signal_value NUMERIC(12,6),
			source VARCHAR(50) DEFAULT 'excel_import',
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`)

		if err := PG.AutoMigrate(
			&model.StockBasic{},
			&model.StockDailyK{},
			&model.StockDailyIndicator{},
			&model.AlgorithmPick{},
			&model.AlgorithmPickDetail{},
			&model.StockSignal{},
		); err != nil {
			log.Printf("PG migrate warning: %v", err)
		}
	}

	if MySQL != nil {
		if err := MySQL.AutoMigrate(
			&model.User{},
			&model.Watchlist{},
			&model.Strategy{},
			&model.BacktestResult{},
			&model.Holding{},
			&model.RiskAlert{},
			&model.ImportLog{},
		); err != nil {
			log.Printf("MySQL migrate warning: %v", err)
		}
	}
	log.Println("AutoMigrate completed")
}
