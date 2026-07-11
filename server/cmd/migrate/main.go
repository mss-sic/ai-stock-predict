// Command migrate runs database migrations as a standalone CLI.
// Usage:
//   migrate                    # run all pending migrations
//   migrate --dry-run           # show pending migrations without executing
//   migrate --force v90          # re-run a specific version (dangerous, use only for repair)
//
// Environment variables (same as server):
//   POSTGRES_DSN   PostgreSQL connection string
//   MYSQL_DSN       MySQL connection string
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/ai-stock-predict/server/internal/config"
	"github.com/ai-stock-predict/server/internal/db"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "show pending migrations without executing")
	force := flag.Int("force", 0, "force re-run a specific migration version (repair mode)")
	flag.Parse()

	cfg := config.Load()
	log.SetFlags(log.LstdFlags)

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  智策投研 — 数据库迁移工具")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	// Connect to databases
	log.Println("[migrate] connecting to PostgreSQL...")
	db.InitPostgres(cfg.PostgresDSN)
	log.Println("[migrate] PostgreSQL connected ✓")

	log.Println("[migrate] connecting to MySQL...")
	db.InitMySQL(cfg.MySQLDSN)
	log.Println("[migrate] MySQL connected ✓")
	fmt.Println()

	if *force > 0 {
		runForceMigration(*force)
		return
	}

	if *dryRun {
		runDryRun()
		return
	}

	runAllMigrations()
}

func runAllMigrations() {
	log.Println("[migrate] running all pending migrations...")
	fmt.Println()

	if err := db.RunMigrations(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ 迁移失败: %v\n", err)
		fmt.Fprintln(os.Stderr, "\n请根据上方错误日志修复后重新执行 migrate 命令。")
		os.Exit(1)
	}

	fmt.Println()
	log.Println("[migrate] ✅ 所有迁移执行成功")
}

func runDryRun() {
	pending := db.PendingMigrations()
	if len(pending) == 0 {
		fmt.Println("✅ 没有待执行的迁移，数据库已是最新状态。")
		return
	}

	fmt.Printf("📋 待执行迁移: %d 个\n\n", len(pending))
	sort.Slice(pending, func(i, j int) bool { return pending[i].Version < pending[j].Version })
	for _, m := range pending {
		fmt.Printf("  v%-4d  %s\n", m.Version, m.Description)
	}
	fmt.Println()
	fmt.Println("使用不带参数的命令执行以上迁移。")
}

func runForceMigration(version int) {
	fmt.Printf("⚠️  强制重跑迁移 v%d（修复模式）\n\n", version)
	if err := db.ForceMigration(version); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ 强制迁移 v%d 失败: %v\n", version, err)
		os.Exit(1)
	}
	fmt.Printf("✅ 迁移 v%d 执行成功\n", version)
}
