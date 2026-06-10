package db

import (
	"fmt"
	"log"
	"sort"
	"time"

	"gorm.io/gorm"
)

// Migration represents a versioned database migration.
type Migration struct {
	Version     int
	Description string
	Up          func() error
}

var migrations []Migration

// Register adds a migration to the global registry.
func Register(m Migration) {
	migrations = append(migrations, m)
}

// RunMigrations executes all unapplied migrations in version order.
// It uses the schema_migrations table (in PG) to track which versions have been applied.
func RunMigrations() error {
	if PG == nil {
		log.Println("[migrate] PG not available, skipping migrations")
		return nil
	}

	// Ensure tracking table exists
	if err := PG.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INT PRIMARY KEY,
		description VARCHAR(255) NOT NULL,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Read applied versions
	var applied []int
	if err := PG.Raw("SELECT version FROM schema_migrations ORDER BY version").Scan(&applied).Error; err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	appliedSet := make(map[int]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	for _, m := range migrations {
		if appliedSet[m.Version] {
			log.Printf("[migrate] skip v%d %s (already applied)", m.Version, m.Description)
			continue
		}
		log.Printf("[migrate] running v%d %s ...", m.Version, m.Description)
		if err := m.Up(); err != nil {
			return fmt.Errorf("migration v%d %s: %w", m.Version, m.Description, err)
		}
		// Record success
		if err := PG.Exec("INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)",
			m.Version, m.Description, time.Now()).Error; err != nil {
			// If race condition (another instance ran it), ignore duplicate
			if isDuplicateKeyErr(err) {
				log.Printf("[migrate] v%d already recorded (race), continuing", m.Version)
				continue
			}
			return fmt.Errorf("record migration v%d: %w", m.Version, err)
		}
		log.Printf("[migrate] v%d %s ✓", m.Version, m.Description)
	}

	log.Printf("[migrate] done (%d migrations, %d new)", len(migrations), len(migrations)-len(applied))
	return nil
}

func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	// PG duplicate key error code
	errStr := err.Error()
	return contains(errStr, "duplicate key") || contains(errStr, "23505")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// safeExec runs SQL on PG only if the PG connection is available.
func safeExec(sql string, args ...interface{}) {
	if PG != nil {
		if err := PG.Exec(sql, args...).Error; err != nil {
			log.Printf("[migrate] WARN: %v", err)
		}
	}
}

// safeExecMysql runs SQL on MySQL only if the MySQL connection is available.
func safeExecMysql(sql string, args ...interface{}) {
	if MySQL != nil {
		if err := MySQL.Exec(sql, args...).Error; err != nil {
			log.Printf("[migrate] WARN MySQL: %v", err)
		}
	}
}

// gormAutoMigrate runs GORM AutoMigrate and logs warnings.
func gormAutoMigrate(db *gorm.DB, models ...interface{}) {
	if db == nil {
		return
	}
	if err := db.AutoMigrate(models...); err != nil {
		log.Printf("[migrate] GORM AutoMigrate warning: %v", err)
	}
}
