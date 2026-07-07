package config

import "os"

type Config struct {
	Port        string
	PostgresDSN string
	MySQLDSN    string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		PostgresDSN: getEnv("POSTGRES_DSN", "host=localhost user=stock password=stock123 dbname=stock_predict port=5432 sslmode=disable TimeZone=Asia/Shanghai"),
		MySQLDSN:    getEnv("MYSQL_DSN", "stock:stock123@tcp(127.0.0.1:3307)/stock_predict?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
