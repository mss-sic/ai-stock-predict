package db

import (
	"log"

	"github.com/ai-stock-predict/server/internal/model"
)

func AutoMigrate() {
	if PG != nil {
		if err := PG.AutoMigrate(
			&model.StockBasic{},
			&model.StockDailyK{},
			&model.StockDailyIndicator{},
			&model.AlgorithmPick{},
			&model.AlgorithmPickDetail{},
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
