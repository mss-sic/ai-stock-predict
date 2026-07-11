package db

import (
	"log"
	"os"

	"github.com/ai-stock-predict/server/internal/model"
)

// AutoMigrate runs all pending versioned migrations in order.
// Set SKIP_AUTO_MIGRATE=true to disable (use standalone migrate command instead).
//
// Safe for production: each migration is idempotent (IF NOT EXISTS / IF EXISTS),
// and previously applied versions are tracked in schema_migrations.
//
// To add a new migration:
//  1. Open migrations_data.go
//  2. Add a Register(Migration{Version: N, Description: "...", Up: func() error { ... }})
//  3. Always use safeExec/safeExecMysql for raw SQL (handles nil DB)
//  4. Always write idempotent SQL (CREATE IF NOT EXISTS, ALTER with IF NOT EXISTS, etc.)
func AutoMigrate() {
	if os.Getenv("SKIP_AUTO_MIGRATE") == "true" {
		log.Println("[migrate] SKIP_AUTO_MIGRATE=true, 跳过自动迁移（请使用 migrate 命令）")
		return
	}

	if err := RunMigrations(); err != nil {
		log.Printf("[migrate] FATAL: %v", err)
		log.Println("[migrate] 迁移失败！请使用 migrate 命令排查修复。")
	}
}

// EnsureManualTables is kept for backward compatibility.
// New manual tables should be added as versioned migrations in migrations_data.go.
func EnsureManualTables() {
	// Lightweight tables auto-migrated via GORM (for dedup/audit logging)
	MySQL.AutoMigrate(&model.NotificationLog{})
	log.Println("[migrate] EnsureManualTables: done")
}
