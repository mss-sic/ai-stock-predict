package db

import (
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var MySQL *gorm.DB

func InitMySQL(dsn string) {
	var err error
	MySQL, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect mysql: %v", err)
	}
	sqlDB, _ := MySQL.DB()
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(3)
	log.Println("MySQL connected")
}
