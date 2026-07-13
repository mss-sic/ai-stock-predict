package db

import (
	"log"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var PG *gorm.DB

func InitPostgres(dsn string) {
	var err error
	PG, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}
	sqlDB, _ := PG.DB()
	// Limit concurrent connections to prevent PostgreSQL shared memory exhaustion
	// (work_mem × connections + lock table overhead). Default 8, configurable via PG_MAX_OPEN_CONNS.
	maxOpen := 8
	if v := os.Getenv("PG_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxOpen = n
		}
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxOpen / 2)
	log.Printf("PostgreSQL connected (max_open_conns=%d, max_idle=%d)", maxOpen, maxOpen/2)
}
