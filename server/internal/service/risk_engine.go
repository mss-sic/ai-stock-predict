package service

import (
	"strings"
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"gorm.io/gorm/clause"
)

// RiskRule evaluates risk conditions for given stock codes.
// Returns: new/updated alerts, alerts to resolve (condition no longer holds).
type RiskRule interface {
	// Key returns the rule key matching risk_rules.rule_key.
	Key() string
	// Evaluate scans the given stock codes (or nil for market-level) and returns alerts.
	Evaluate(ctx context.Context, stockCodes []string, allHoldings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error)
}

// RiskEngine orchestrates all risk rules and handles dedup/upsert/resolve.
type RiskEngine struct {
	mu    sync.RWMutex
	rules map[string]RiskRule // keyed by rule_key

	// seen hashes for within-scan dedup (event-type rules)
	seenHashes map[string]bool
}

var defaultEngine = &RiskEngine{
	rules:      make(map[string]RiskRule),
	seenHashes: make(map[string]bool),
}

// RegisterRule adds a rule to the engine.
func RegisterRule(r RiskRule) {
	defaultEngine.mu.Lock()
	defer defaultEngine.mu.Unlock()
	defaultEngine.rules[r.Key()] = r
	// (log suppressed: use GET /risk/rules to list all)
}

// Rules returns all registered rule keys.
func (e *RiskEngine) Rules() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	keys := make([]string, 0, len(e.rules))
	for k := range e.rules {
		keys = append(keys, k)
	}
	return keys
}

// GetEngine returns the global risk engine instance.
func GetEngine() *RiskEngine { return defaultEngine }

// ScanAll runs all enabled rules for all user holdings.
// Returns total alerts generated (new + updated).
func (e *RiskEngine) ScanAll(ctx context.Context) (int, error) {
	var holdings []model.Holding
	db.MySQL.Find(&holdings)
	if len(holdings) == 0 {
		return 0, nil
	}

	// Build user→stock mapping for per-user stock alerts
	userStocks := make(map[uint]map[string]bool)
	codeSet := make(map[string]bool)
	for _, h := range holdings {
		codeSet[h.StockCode] = true
		if userStocks[h.UserID] == nil {
			userStocks[h.UserID] = make(map[string]bool)
		}
		userStocks[h.UserID][h.StockCode] = true
	}
	codes := make([]string, 0, len(codeSet))
	for c := range codeSet {
		codes = append(codes, c)
	}

	var ruleDefs []model.RiskRule
	db.MySQL.Where("enabled = true").Find(&ruleDefs)
	if len(ruleDefs) == 0 {
		return 0, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	e.seenHashes = make(map[string]bool)

	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	totalCount := 0

	for _, def := range ruleDefs {
		rule, ok := e.rules[def.RuleKey]
		if !ok {
			continue
		}

		alerts, toResolve, err := rule.Evaluate(ctx, codes, holdings)
		if err != nil {
			log.Printf("[RiskEngine] rule %s error: %v", def.RuleKey, err)
			continue
		}

		for _, a := range toResolve {
			q := db.MySQL.Model(&model.RiskAlert{}).
				Where("rule_key = ? AND stock_code = ? AND status = 'active'", a.RuleKey, a.StockCode)
			if a.UserID > 0 {
				q = q.Where("user_id = ?", a.UserID)
			}
			q.Updates(map[string]interface{}{"status": "resolved", "resolved_at": now})
		}

		for _, a := range alerts {
			if a.Dimension == "market" || a.UserID > 0 {
				c, _ := e.upsertAlert(&a, now)
				totalCount += c
			} else {
				// Stock-level: create per-user copies for each real user holding this stock
				for uid, stocks := range userStocks {
					if uid == 0 {
						continue
					}
					if stocks[a.StockCode] {
						ua := a
						ua.UserID = uid
						c, _ := e.upsertAlert(&ua, now)
						totalCount += c
					}
				}
			}
		}
	}

	// Auto-resolve: mark alerts as resolved if condition no longer holds
	totalResolved := e.autoResolve(codes, now)
	if totalResolved > 0 {
		log.Printf("[RiskEngine] auto-resolved %d alerts", totalResolved)
	}

	// Cross-rule correlation: detect dangerous combinations
	totalCorrelated := e.detectCorrelations(codes, now, userStocks)
	if totalCorrelated > 0 {
		log.Printf("[RiskEngine] generated %d correlation alerts", totalCorrelated)
	}

	return totalCount, nil
}

// autoResolve marks threshold-type alerts as resolved when condition no longer holds.
// Strategy: re-run each rule that has active alerts, compare results, resolve stale ones.
func (e *RiskEngine) autoResolve(codes []string, now time.Time) int {
	// Group active alerts by rule_key → stock_codes
	type activeGroup struct {
		ruleKey string
		codes   map[string]bool
		userIDs map[uint]bool
	}
	groups := make(map[string]*activeGroup)

	var allActive []model.RiskAlert
	db.MySQL.Where("status = 'active' AND dimension IN ?", []string{"stock", "liquidity", "event"}).Find(&allActive)
	for _, a := range allActive {
		if groups[a.RuleKey] == nil {
			groups[a.RuleKey] = &activeGroup{ruleKey: a.RuleKey, codes: make(map[string]bool), userIDs: make(map[uint]bool)}
		}
		groups[a.RuleKey].codes[a.StockCode] = true
		groups[a.RuleKey].userIDs[a.UserID] = true
	}

	resolvedCount := 0
	for _, g := range groups {
		rule, ok := e.rules[g.ruleKey]
		if !ok {
			continue
		}
		// Collect stock codes for this rule
		ruleCodes := make([]string, 0, len(g.codes))
		for c := range g.codes {
			ruleCodes = append(ruleCodes, c)
		}
		// Re-run rule once for all codes
		newAlerts, _, err := rule.Evaluate(context.Background(), ruleCodes, nil)
		if err != nil {
			continue
		}
		// Build set of codes that still trigger
		stillActive := make(map[string]bool)
		for _, na := range newAlerts {
			stillActive[na.StockCode] = true
		}
		// Resolve alerts whose code is no longer triggering
		for c := range g.codes {
			if !stillActive[c] {
				db.MySQL.Model(&model.RiskAlert{}).
					Where("rule_key = ? AND stock_code = ? AND status = 'active'", g.ruleKey, c).
					Updates(map[string]interface{}{"status": "resolved", "resolved_at": now})
				resolvedCount++
			}
		}
	}

	// Auto-resolve market alerts
	var mktAlerts []model.RiskAlert
	db.MySQL.Where("status = 'active' AND dimension = 'market'").Find(&mktAlerts)
	for _, a := range mktAlerts {
		rule, ok := e.rules[a.RuleKey]
		if !ok {
			continue
		}
		newAlerts, _, err := rule.Evaluate(context.Background(), nil, nil)
		if err != nil {
			continue
		}
		if len(newAlerts) == 0 {
			db.MySQL.Model(&model.RiskAlert{}).
				Where("rule_key = ? AND status = 'active'", a.RuleKey).
				Updates(map[string]interface{}{"status": "resolved", "resolved_at": now})
			resolvedCount++
		}
	}

	return resolvedCount
}

// detectCorrelations checks for dangerous rule combinations and generates synthetic alerts.
func (e *RiskEngine) detectCorrelations(codes []string, now time.Time, userStocks map[uint]map[string]bool) int {
	// Get today's active alerts for correlation analysis
	var todayAlerts []model.RiskAlert
	db.MySQL.Where("status = 'active' AND DATE(hit_date) = ? AND stock_code IN ?", 
		now.Format("2006-01-02"), codes).Find(&todayAlerts)
	if len(todayAlerts) == 0 {
		return 0
	}

	// Group alerts by stock code
	stockRules := make(map[string]map[string]bool)
	for _, a := range todayAlerts {
		if stockRules[a.StockCode] == nil {
			stockRules[a.StockCode] = make(map[string]bool)
		}
		stockRules[a.StockCode][a.RuleKey] = true
	}

	// Known dangerous combinations: [ruleA, ruleB] → correlation alert
	correlations := []struct {
		rules      []string
		level      string
		corrType   string
		description string
		severity   int
	}{
		{[]string{"pe_extreme", "major_reduction"}, "high", "估值+减持双杀", "高估值叠加股东减持，恐慌抛售风险极高", 95},
		{[]string{"ma_bearish_alignment", "profit_decline"}, "high", "趋势+基本面恶化", "均线空头排列叠加业绩下滑，趋势向下确认", 85},
		{[]string{"heavy_volume_drop", "major_outflow_streak"}, "high", "放量+资金流出", "放量下跌叠加资金持续流出，主力撤退信号", 90},
		{[]string{"macd_divergence", "rsi_overbought"}, "high", "背离+超买共振", "MACD顶背离叠加RSI超买，技术面极度危险", 85},
		{[]string{"limit_down_locked", "margin_collapse"}, "high", "跌停+杠杆崩塌", "跌停封板叠加融资盘崩塌，流动性危机", 95},
		{[]string{"pe_extreme", "shrinking_rebound"}, "medium", "高估+假反弹", "高估值下的缩量反弹，诱多陷阱风险", 70},
		{[]string{"volume_too_low", "turnover_decay"}, "medium", "流动性枯竭", "成交额低且换手率持续衰减，僵尸股风险", 65},
		{[]string{"ma20_breakdown", "profit_decline"}, "medium", "破位+业绩降", "跌破关键均线伴随业绩下滑，下行加速风险", 75},
	}

	count := 0
	for code, rules := range stockRules {
		for _, corr := range correlations {
			allMatch := true
			for _, rk := range corr.rules {
				if !rules[rk] {
					allMatch = false
					break
				}
			}
			if !allMatch {
				continue
			}
			// Generate correlation alert
			alert := model.RiskAlert{
				StockCode:     code,
				Level:         corr.level,
				Type:          corr.corrType,
				Description:   fmt.Sprintf("%s: %s", code, corr.description),
				RuleKey:       "correlation_" + strings.ReplaceAll(corr.corrType, "+", "_"),
				Dimension:     "stock",
				SeverityScore: corr.severity,
				Evidence: model.JSONMap{
					"rules":       corr.rules,
					"correlation": corr.corrType,
				},
				HitDate: now,
			}
			// Create per-user copies
			for uid := range userStocks {
				if uid == 0 { continue }
				if userStocks[uid][code] {
					ua := alert
					ua.UserID = uid
					e.upsertAlert(&ua, now)
					count++
				}
			}
		}
	}

	return count
}

// upsertAlert inserts or updates a risk alert with dedup.
func (e *RiskEngine) upsertAlert(a *model.RiskAlert, now time.Time) (int, error) {
	// Scenario A (threshold-type): UNIQUE index handles dedup
	// Scenario B/C (event-type): check seenHashes
	if a.RuleKey != "" && a.StockCode != "" && a.Dimension != "" &&
		a.Dimension != "market" && a.Evidence != nil {
		hash := eventHashWithUser(a.StockCode, a.RuleKey, a.Evidence, a.UserID)
		if e.seenHashes[hash] {
			return 0, nil
		}
		e.seenHashes[hash] = true
	}

	// Fill defaults
	if a.Status == "" {
		a.Status = "active"
	}
	if a.HitDate.IsZero() {
		a.HitDate = now
	}
	// Truncate to date-only for dedup index
	a.HitDate = time.Date(a.HitDate.Year(), a.HitDate.Month(), a.HitDate.Day(), 0, 0, 0, 0, a.HitDate.Location())
	a.UpdatedAt = now

	// Upsert using GORM OnConflict
	res := db.MySQL.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "stock_code"},
			{Name: "rule_key"},
			{Name: "hit_date"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"severity_score", "evidence", "description", "level", "type", "status", "updated_at",
		}),
	}).Create(a)

	if res.Error != nil {
		return 0, res.Error
	}

	return 1, nil
}

// ScanUserHoldingsWithEngine runs the new RiskEngine (replaces old logic).
// Signature matches old ScanUserHoldings for backward compat.
func ScanUserHoldingsWithEngine() (int, error) {
	ctx := context.Background()
	return GetEngine().ScanAll(ctx)
}

// eventHash creates a deterministic hash for event-type dedup.
func eventHash(code, ruleKey string, evidence model.JSONMap) string {
	evidenceStr := ""
	if evidence != nil {
		evidenceStr = fmt.Sprintf("%v", evidence)
	}
	raw := fmt.Sprintf("%s:%s:%s", code, ruleKey, evidenceStr)
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

// eventHashWithUser creates a hash including user_id for per-user dedup.
func eventHashWithUser(code, ruleKey string, evidence model.JSONMap, userID uint) string {
	evidenceStr := ""
	if evidence != nil {
		evidenceStr = fmt.Sprintf("%v", evidence)
	}
	raw := fmt.Sprintf("%s:%s:%s:%d", code, ruleKey, evidenceStr, userID)
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

// ScanDimension runs only rules of a specific dimension.
func (e *RiskEngine) ScanDimension(dimension string) (int, error) {
	var holdings []model.Holding
	db.MySQL.Find(&holdings)
	if len(holdings) == 0 {
		return 0, nil
	}

	userStocks := make(map[uint]map[string]bool)
	codeSet := make(map[string]bool)
	for _, h := range holdings {
		codeSet[h.StockCode] = true
		if userStocks[h.UserID] == nil {
			userStocks[h.UserID] = make(map[string]bool)
		}
		userStocks[h.UserID][h.StockCode] = true
	}
	codes := make([]string, 0, len(codeSet))
	for c := range codeSet {
		codes = append(codes, c)
	}

	var ruleDefs []model.RiskRule
	db.MySQL.Where("enabled = true AND dimension = ?", dimension).Find(&ruleDefs)
	if len(ruleDefs) == 0 {
		return 0, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	e.seenHashes = make(map[string]bool)

	ctx := context.Background()
	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	totalCount := 0

	for _, def := range ruleDefs {
		rule, ok := e.rules[def.RuleKey]
		if !ok {
			continue
		}
		alerts, _, err := rule.Evaluate(ctx, codes, holdings)
		if err != nil {
			log.Printf("[RiskEngine] rule %s error: %v", def.RuleKey, err)
			continue
		}
		for _, a := range alerts {
			if a.Dimension == "market" || a.UserID > 0 {
				c, _ := e.upsertAlert(&a, now)
				totalCount += c
			} else {
				for uid, stocks := range userStocks {
					if uid == 0 {
						continue
					}
					if stocks[a.StockCode] {
						ua := a
						ua.UserID = uid
						c, _ := e.upsertAlert(&ua, now)
						totalCount += c
					}
				}
			}
		}
	}
	return totalCount, nil
}
