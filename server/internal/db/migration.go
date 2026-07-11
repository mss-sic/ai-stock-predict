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

	skipped := 0
	for _, m := range migrations {
		if appliedSet[m.Version] {
			skipped++
			continue
		}
		log.Printf("[migrate] v%d %s ...", m.Version, m.Description)
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

	log.Printf("[migrate] done: %d total, %d skipped, %d new", len(migrations), skipped, len(migrations)-skipped)
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

// seedRiskRules inserts the 34 default risk detection rules on first run.
func seedRiskRules(db *gorm.DB) {
	if db == nil {
		return
	}
	// Check if already seeded
	var count int64
	db.Table("risk_rules").Count(&count)
	if count > 0 {
		return
	}

	rules := []map[string]interface{}{
		// ── Market (4) ──
		{"rule_key": "fear_greed_overheat", "name": "恐贪指数过热", "dimension": "market", "default_level": "high", "enabled": true, "thresholds": `{"threshold":80,"consecutive_days":3}`, "description": "市场恐贪指数连续高于阈值，全线减仓预警", "weight": 0.15},
		{"rule_key": "market_breadth_decay", "name": "市场宽度恶化", "dimension": "market", "default_level": "medium", "enabled": true, "thresholds": `{"threshold":0.30}`, "description": "全市场站上MA20股票占比低于阈值", "weight": 0.10},
		{"rule_key": "northbound_outflow_streak", "name": "北向资金连续流出", "dimension": "market", "default_level": "medium", "enabled": true, "thresholds": `{"threshold_days":5}`, "description": "北向资金连续净流出超阈值天数", "weight": 0.10},
		{"rule_key": "volatility_spike", "name": "波动率飙升", "dimension": "market", "default_level": "medium", "enabled": true, "thresholds": `{"percentile":0.90}`, "description": "全市场振幅中位数突破历史分位", "weight": 0.10},

		// ── Stock (17) ──
		{"rule_key": "heavy_volume_drop", "name": "放量下跌", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{"drop_pct":-5,"volume_ratio":2.0}`, "description": "单日跌幅超阈值且量比放大", "weight": 0.08},
		{"rule_key": "shrinking_rebound", "name": "连续缩量反弹", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"rebound_days":3}`, "description": "反弹但量能递减，假反弹风险", "weight": 0.05},
		{"rule_key": "ma_bearish_alignment", "name": "均线空头排列", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"buffer_pct":0.02}`, "description": "MA5<MA10<MA20<MA60收盘确认", "weight": 0.06},
		{"rule_key": "rsi_overbought", "name": "RSI超买", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"threshold":80,"period":14}`, "description": "RSI突破超买线", "weight": 0.04},
		{"rule_key": "macd_divergence", "name": "MACD顶背离", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{"lookback":60}`, "description": "价格新高MACD未新高，动能衰竭", "weight": 0.08},
		{"rule_key": "bollinger_squeeze", "name": "布林带收窄", "dimension": "stock", "default_level": "low", "enabled": true, "thresholds": `{"percentile":0.20,"period":20}`, "description": "带宽收窄至历史低位，变盘预警", "weight": 0.03},
		{"rule_key": "turnover_abnormal", "name": "换手率异常", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"high":20,"low":0.1}`, "description": "换手率超常高或异常低", "weight": 0.04},
		{"rule_key": "major_outflow_streak", "name": "主力资金连续流出", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"days":5}`, "description": "主力净流出连续天数", "weight": 0.06},
		{"rule_key": "margin_collapse", "name": "融资余额骤降", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{"days":5,"drop_pct":-10}`, "description": "融资余额降幅超阈值", "weight": 0.08},
		{"rule_key": "block_discount", "name": "大宗折价交易", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"discount_pct":-8}`, "description": "大宗成交折价超阈值", "weight": 0.05},
		{"rule_key": "dragon_institution_sell", "name": "龙虎榜机构净卖出", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{"sell_buy_ratio":2.0}`, "description": "机构席位净卖出超买入倍数", "weight": 0.08},
		{"rule_key": "st_delist_risk", "name": "ST退市风险", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{}`, "description": "ST标识或面值低于1.5元", "weight": 0.10},
		{"rule_key": "ai_score_crash", "name": "AI评分骤降", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"days":3,"drop":2.0}`, "description": "AI综合评分在窗口期内下降超阈值", "weight": 0.06},
		{"rule_key": "sharp_decline", "name": "近期大跌", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{"days":5,"drop_pct":-8,"confirm_volume":true}`, "description": "近N日累计跌幅超阈值", "weight": 0.08},
		{"rule_key": "ma20_breakdown", "name": "跌破均线", "dimension": "stock", "default_level": "medium", "enabled": true, "thresholds": `{"buffer_pct":0.02}`, "description": "收盘下穿MA20，昨日在上方", "weight": 0.06},
		{"rule_key": "pe_extreme", "name": "估值极高", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{"pe_high":200,"pe_warn":100}`, "description": "市盈率远超行业合理范围", "weight": 0.08},
		{"rule_key": "profit_decline", "name": "业绩下滑", "dimension": "stock", "default_level": "high", "enabled": true, "thresholds": `{"decline_pct":-50,"warn_pct":-30}`, "description": "最新财报净利润同比降幅超阈值", "weight": 0.08},

		// ── Portfolio (4) ──
		{"rule_key": "industry_concentration", "name": "行业集中度过高", "dimension": "portfolio", "default_level": "medium", "enabled": true, "thresholds": `{"max_pct":0.40}`, "description": "单一行业仓位占比超阈值", "weight": 0.08},
		{"rule_key": "correlation_high", "name": "持仓相关性过高", "dimension": "portfolio", "default_level": "medium", "enabled": true, "thresholds": `{"threshold":0.70,"min_history_days":60}`, "description": "组合内股票收益率相关系数均值超阈值", "weight": 0.06},
		{"rule_key": "var_breach", "name": "VaR在险价值超限", "dimension": "portfolio", "default_level": "high", "enabled": true, "thresholds": `{"confidence":0.95,"max_var_pct":0.05,"min_history_days":90}`, "description": "历史模拟法VaR超总资产阈值", "weight": 0.10},
		{"rule_key": "position_overlimit", "name": "总仓位超限", "dimension": "portfolio", "default_level": "high", "enabled": true, "thresholds": `{"max_total_pct":0.80}`, "description": "总持仓占比超策略上限", "weight": 0.08},

		// ── Liquidity (3) ──
		{"rule_key": "volume_too_low", "name": "日成交额过低", "dimension": "liquidity", "default_level": "medium", "enabled": true, "thresholds": `{"avg_days":5,"min_amount_small_cap":20000000,"min_amount_large_cap":10000000}`, "description": "近N日均成交低于阈值", "weight": 0.06},
		{"rule_key": "limit_down_locked", "name": "跌停封板无流动性", "dimension": "liquidity", "default_level": "high", "enabled": true, "thresholds": `{}`, "description": "当日跌停且未开板", "weight": 0.10},
		{"rule_key": "turnover_decay", "name": "换手率持续衰减", "dimension": "liquidity", "default_level": "low", "enabled": true, "thresholds": `{"days":30,"min_turnover":0.005}`, "description": "30日换手率趋势向下且跌破阈值", "weight": 0.03},

		// ── Event (3) ──
		{"rule_key": "major_reduction", "name": "大股东减持公告", "dimension": "event", "default_level": "high", "enabled": true, "thresholds": `{"lookback_days":30,"keywords":"减持,股份变动,权益变动,转让"}`, "description": "近N天有减持相关公告", "weight": 0.10},
		{"rule_key": "litigation_violation", "name": "重大诉讼违规", "dimension": "event", "default_level": "high", "enabled": true, "thresholds": `{"lookback_days":30,"keywords":"诉讼,违规,处罚,立案,调查"}`, "description": "近N天有诉讼/违规/处罚相关公告", "weight": 0.10},
		{"rule_key": "dividend_ex_near", "name": "分红除权临近", "dimension": "event", "default_level": "low", "enabled": true, "thresholds": `{"lookahead_days":5}`, "description": "未来N日有除权除息", "weight": 0.03},

		// ── Behavioral (3) ──
		{"rule_key": "overtrading", "name": "频繁交易", "dimension": "behavior", "default_level": "low", "enabled": true, "thresholds": `{"max_trades_per_day":5}`, "description": "当日交易笔数超阈值", "weight": 0.03},
		{"rule_key": "stop_loss_missed", "name": "止损未执行", "dimension": "behavior", "default_level": "high", "enabled": true, "thresholds": `{"tolerance_pct":0.02}`, "description": "持仓亏损超策略止损线但未触发卖出", "weight": 0.10},
		{"rule_key": "live_backtest_divergence", "name": "实盘回测偏离", "dimension": "behavior", "default_level": "medium", "enabled": true, "thresholds": `{"max_divergence_pct":0.15}`, "description": "实盘收益偏离回测基准超阈值", "weight": 0.06},
	}

	for _, r := range rules {
		db.Exec(`INSERT INTO risk_rules (rule_key, name, dimension, default_level, enabled, thresholds, description, weight) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			r["rule_key"], r["name"], r["dimension"], r["default_level"], r["enabled"], r["thresholds"], r["description"], r["weight"])
	}
	log.Printf("[migrate] seeded %d risk rules", len(rules))
}

// PendingMigrations returns all unapplied migrations (for dry-run display).
func PendingMigrations() []Migration {
	if PG == nil {
		return nil
	}

	// Read applied versions
	var applied []int
	PG.Raw("SELECT version FROM schema_migrations ORDER BY version").Scan(&applied)
	appliedSet := make(map[int]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	var pending []Migration
	// Sort migrations by version before filtering
	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version < sorted[j].Version })

	for _, m := range sorted {
		if !appliedSet[m.Version] {
			pending = append(pending, m)
		}
	}
	return pending
}

// ForceMigration re-runs a specific migration version (repair mode).
// It deletes the tracking record first, then executes the migration.
func ForceMigration(version int) error {
	if PG == nil {
		return fmt.Errorf("PostgreSQL not available")
	}

	PG.Exec("DELETE FROM schema_migrations WHERE version = ?", version)

	// Find and run the migration
	for _, m := range migrations {
		if m.Version == version {
			log.Printf("[migrate] force re-run v%d %s ...", m.Version, m.Description)
			if err := m.Up(); err != nil {
				return fmt.Errorf("migration v%d %s: %w", m.Version, m.Description, err)
			}
			// Record success
			PG.Exec("INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)",
				m.Version, m.Description, time.Now())
			log.Printf("[migrate] v%d %s ✓ (force)", m.Version, m.Description)
			return nil
		}
	}
	return fmt.Errorf("migration v%d not found", version)
}
