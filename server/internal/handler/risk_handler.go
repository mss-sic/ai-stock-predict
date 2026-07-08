package handler

import (
	"fmt"
	"sort"
	"math"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type RiskHandler struct{}

func NewRiskHandler() *RiskHandler { return &RiskHandler{} }

// ── Legacy APIs (backward compat) ──

// List returns risk alerts for current user's holdings (legacy format).
func (h *RiskHandler) List(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	alerts, err := service.GetUserRiskAlerts(uid)
	if err != nil {
		response.InternalError(c, "获取风险预警失败: "+err.Error())
		return
	}
	response.Success(c, alerts)
}

// Scan triggers a full risk scan (admin only).
func (h *RiskHandler) Scan(c *gin.Context) {
	count, err := service.ScanUserHoldings()
	if err != nil {
		response.InternalError(c, "风险扫描失败: "+err.Error())
		return
	}
	log.Printf("[RiskHandler] admin triggered scan: %d alerts generated", count)
	response.Success(c, map[string]any{"alertsGenerated": count})
}

// Ignore marks a specific alert as ignored.
func (h *RiskHandler) Ignore(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := service.IgnoreRiskAlert(uid, uint(id)); err != nil {
		response.InternalError(c, "忽略预警失败: "+err.Error())
		return
	}
	response.SuccessMsg(c, "已忽略")
}

// ── New APIs ──

// Dashboard returns risk dashboard aggregate data.
func (h *RiskHandler) Dashboard(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}

	// Counts — filtered by user holdings (same logic as alert list)
	baseQuery := db.MySQL.Model(&model.RiskAlert{}).Where("(user_id = ? OR (user_id = 0 AND stock_code = '__MARKET__'))", uid).Where("DATE(hit_date) = CURRENT_DATE")
	var high, medium, low int64
	baseQuery.Where("status = 'active' AND level = 'high'").Count(&high)
	baseQuery.Where("status = 'active' AND level = 'medium'").Count(&medium)
	baseQuery.Where("status = 'active' AND level = 'low'").Count(&low)

	// Market risk level: weighted multi-factor scoring with breakdown
	breakdown := computeMarketRiskBreakdown(uid, high, medium, low)

	// Active alerts for user holdings
	alerts, _ := service.GetUserRiskAlerts(uid)

	response.Success(c, gin.H{
		"marketRiskLevel":     breakdown.Level,
		"marketRiskScore":     breakdown.Score,
		"marketRiskBreakdown": breakdown,
		"highAlerts":          high,
		"mediumAlerts":        medium,
		"lowAlerts":           low,
		"totalAlerts":         len(alerts),
		"circuitBreaker":      false,
	})
}

// ListAlerts returns paginated risk alerts with filters.
func (h *RiskHandler) ListAlerts(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	level := c.Query("level")
	dimension := c.Query("dimension")
	status := c.DefaultQuery("status", "active")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := db.MySQL.Model(&model.RiskAlert{}).
		Where("(user_id = ? OR (user_id = 0 AND stock_code = '__MARKET__'))", uid).
		Where("status = ?", status)
	if level != "" {
		query = query.Where("level = ?", level)
	}
	if dimension != "" {
		query = query.Where("dimension = ?", dimension)
	}

	var total int64
	query.Count(&total)

	var alerts []model.RiskAlert
	query.Order("FIELD(level, 'high','medium','low'), hit_date DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&alerts)

	// Enrich names
	enrichNames(alerts)

	response.Success(c, gin.H{
		"list":     alerts,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetAlertDetail returns a single alert with evidence.
func (h *RiskHandler) GetAlertDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var alert model.RiskAlert
	if err := db.MySQL.First(&alert, id).Error; err != nil {
		response.NotFound(c, "预警不存在")
		return
	}
	enrichNames([]model.RiskAlert{alert})
	response.Success(c, alert)
}

// AcknowledgeAlert marks an alert as acknowledged.
func (h *RiskHandler) AcknowledgeAlert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	db.MySQL.Model(&model.RiskAlert{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "acknowledged", "acknowledged_at": time.Now()})
	response.SuccessMsg(c, "已确认")
}

// ListRules returns all risk rule definitions.
func (h *RiskHandler) ListRules(c *gin.Context) {
	var rules []model.RiskRule
	db.MySQL.Order("FIELD(dimension, 'market','stock','portfolio','liquidity','event','behavior'), rule_key").Find(&rules)
	response.Success(c, rules)
}

// UpdateRule updates thresholds for a specific rule.
func (h *RiskHandler) UpdateRule(c *gin.Context) {
	key := c.Param("key")
	var body struct {
		Enabled    *bool          `json:"enabled"`
		Thresholds model.JSONMap  `json:"thresholds"`
		Weight     *float64       `json:"weight"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if body.Enabled != nil {
		updates["enabled"] = *body.Enabled
	}
	if body.Thresholds != nil {
		updates["thresholds"] = body.Thresholds
	}
	if body.Weight != nil {
		updates["weight"] = *body.Weight
	}
	if len(updates) == 0 {
		response.BadRequest(c, "无更新内容")
		return
	}
	if err := db.MySQL.Model(&model.RiskRule{}).Where("rule_key = ?", key).Updates(updates).Error; err != nil {
		response.InternalError(c, "更新失败: "+err.Error())
		return
	}
	response.SuccessMsg(c, "已更新")
}

// ListSnapshots returns historical risk snapshots for trend charts.
func (h *RiskHandler) ListSnapshots(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}
	var snaps []model.RiskSnapshot
	db.MySQL.Order("snapshot_date DESC").Limit(days).Find(&snaps)
	response.Success(c, snaps)
}

// CircuitBreakerStatus returns current circuit breaker status.
func (h *RiskHandler) CircuitBreakerStatus(c *gin.Context) {
	response.Success(c, gin.H{"active": false, "reason": "", "triggeredAt": nil})
}

// ── Helpers ──

func enrichNames(alerts []model.RiskAlert) {
	if len(alerts) == 0 {
		return
	}
	// Handle sentinel stock codes
	realCodes := make([]string, 0)
	for _, a := range alerts {
		if a.StockCode != "__MARKET__" && !strings.HasPrefix(a.StockCode, "__PORTFOLIO_") {
			realCodes = append(realCodes, a.StockCode)
		}
	}

	nameMap := make(map[string]string)
	if len(realCodes) > 0 {
		type NameRow struct {
			Code string `gorm:"column:code"`
			Name string `gorm:"column:name"`
		}
		var names []NameRow
		db.PG.Raw("SELECT code, name FROM stocks_basic WHERE code IN ?", realCodes).Scan(&names)
		for _, n := range names {
			nameMap[n.Code] = n.Name
		}
	}

	for i := range alerts {
		if alerts[i].StockCode == "__MARKET__" {
			alerts[i].StockName = "全市场"
		} else if strings.HasPrefix(alerts[i].StockCode, "__PORTFOLIO_") {
			alerts[i].StockName = "组合风险"
		} else {
			alerts[i].StockName = nameMap[alerts[i].StockCode]
		}
	}
}

// computeMarketRiskLevel calculates market-level risk using multi-factor weighted scoring.
func computeMarketRiskBreakdown(uid uint, high, medium, low int64) MarketRiskBreakdown {
	// Count market-dimension alerts
	var mktHigh, mktMedium, mktLow int64
	db.MySQL.Model(&model.RiskAlert{}).
		Where("status = 'active' AND dimension = 'market' AND DATE(hit_date) = CURRENT_DATE AND level = 'high'").
		Count(&mktHigh)
	db.MySQL.Model(&model.RiskAlert{}).
		Where("status = 'active' AND dimension = 'market' AND DATE(hit_date) = CURRENT_DATE AND level = 'medium'").
		Count(&mktMedium)
	db.MySQL.Model(&model.RiskAlert{}).
		Where("status = 'active' AND dimension = 'market' AND DATE(hit_date) = CURRENT_DATE AND level = 'low'").
		Count(&mktLow)

	// Count holdings
	var holdingCount int64
	db.MySQL.Model(&model.Holding{}).Where("user_id = ?", uid).Count(&holdingCount)
	if holdingCount == 0 {
		holdingCount = 1
	}

	// Factor 1: Market-level alerts (max 45)
	f1Score := math.Min(float64(mktHigh)*15+float64(mktMedium)*8+float64(mktLow)*2, 45)

	// Factor 2: High-risk stock ratio (max 30)
	f2Score := math.Min(float64(high)/float64(holdingCount)*30, 30)

	// Factor 3: Medium alert magnitude (max 15)
	f3Score := math.Min(float64(medium)/3*5, 15)

	// Factor 4: Total alert coverage (max 10)
	f4Score := math.Min(float64(high+medium+low)/float64(holdingCount)*10, 10)

	score := int(math.Min(f1Score+f2Score+f3Score+f4Score, 100))

	level := "low"
	switch {
	case score >= 75:
		level = "critical"
	case score >= 50:
		level = "high"
	case score >= 25:
		level = "medium"
	}

	// Build factors list
	factors := []RiskFactorItem{
		{Name: "市场预警", Score: f1Score, Max: 45, Detail: fmt.Sprintf("市场维度高:%d 中:%d 低:%d", mktHigh, mktMedium, mktLow)},
		{Name: "高风险占比", Score: f2Score, Max: 30, Detail: fmt.Sprintf("%d只高风险 / %d只持仓", high, holdingCount)},
		{Name: "中等风险量", Score: f3Score, Max: 15, Detail: fmt.Sprintf("%d条中等预警", medium)},
		{Name: "预警覆盖率", Score: f4Score, Max: 10, Detail: fmt.Sprintf("%d条预警 / %d只持仓", high+medium+low, holdingCount)},
	}

	// Get active market alerts for detail
	var mktAlerts []model.RiskAlert
	db.MySQL.Where("status = 'active' AND dimension = 'market' AND DATE(hit_date) = CURRENT_DATE").
		Order("FIELD(level, 'high','medium','low')").Find(&mktAlerts)
	activeAlerts := make([]RiskFactorAlert, 0, len(mktAlerts))
	for _, a := range mktAlerts {
		activeAlerts = append(activeAlerts, RiskFactorAlert{
			Type: a.Type, Description: a.Description, Level: a.Level,
		})
	}

	// Generate advice based on level and factors
	advice := generateRiskAdvice(level, f1Score, f2Score, high, holdingCount)

	return MarketRiskBreakdown{
		Level:        level,
		Score:        score,
		Factors:      factors,
		ActiveAlerts: activeAlerts,
		Advice:       advice,
	}
}

// generateRiskAdvice produces actionable advice based on risk factors.
func generateRiskAdvice(level string, mktScore, stockRatio float64, highCount, holdCount int64) string {
	var parts []string

	switch level {
	case "critical":
		parts = append(parts, "⚠️ 触发危险级别，建议暂停所有买入操作")
		parts = append(parts, "立即检查所有持仓的止损位是否有效")
	case "high":
		parts = append(parts, "⚠️ 高风险环境，建议减仓至50%以下")
	case "medium":
		parts = append(parts, "⚡ 中等风险，建议控制仓位在70%以内")
	case "low":
		parts = append(parts, "✅ 低风险环境，可正常操作")
	}

	if mktScore >= 30 {
		parts = append(parts, "市场系统性风险较高，优先降低β值高的持仓")
	}
	if stockRatio >= 20 {
		parts = append(parts, fmt.Sprintf("%d/%d只持仓触发高风险预警，逐一核查并考虑减仓", highCount, holdCount))
	}
	if highCount >= 3 {
		parts = append(parts, "多只持仓同时高风险，避免集中加仓任何一只")
	}

	if len(parts) == 0 {
		parts = append(parts, "保持当前策略，持续监控预警变化")
	}

	return strings.Join(parts, "；")
}

// MarketRiskBreakdown provides detailed factor-level market risk analysis.
type MarketRiskBreakdown struct {
	Level        string            `json:"level"`
	Score        int               `json:"score"`
	Factors      []RiskFactorItem  `json:"factors"`
	ActiveAlerts []RiskFactorAlert `json:"activeAlerts"`
	Advice       string            `json:"advice"`
}

// RiskFactorItem represents a single scoring factor.
type RiskFactorItem struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Max    float64 `json:"max"`
	Detail string  `json:"detail"`
}

// RiskFactorAlert represents an active market alert contributing to the score.
type RiskFactorAlert struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Level       string `json:"level"`
}

// StockRiskSummary represents aggregated risk for a single stock.
type StockRiskSummary struct {
	StockCode  string  `json:"stockCode"`
	StockName  string  `json:"stockName"`
	AlertCount int     `json:"alertCount"`
	HighCount  int     `json:"highCount"`
	RiskScore  int     `json:"riskScore"`  // 0-100 aggregate
	TopRule    string  `json:"topRule"`     // highest severity rule
	TopType    string  `json:"topType"`
	Dimensions []string `json:"dimensions"`
}

// ListAggregated returns per-stock aggregated risk scores for the current user.
func (h *RiskHandler) ListAggregated(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}

	var alerts []model.RiskAlert
	db.MySQL.Where("(user_id = ? OR (user_id = 0 AND stock_code = '__MARKET__'))", uid).
		Where("status = 'active'").
		Find(&alerts)

	// Group by stock code
	type stockGroup struct {
		alerts     []model.RiskAlert
		highCount  int
		totalScore int
		dimensions map[string]bool
	}
	groups := make(map[string]*stockGroup)
	for _, a := range alerts {
		if a.StockCode == "__MARKET__" || strings.HasPrefix(a.StockCode, "__PORTFOLIO_") {
			continue
		}
		if groups[a.StockCode] == nil {
			groups[a.StockCode] = &stockGroup{dimensions: make(map[string]bool)}
		}
		g := groups[a.StockCode]
		g.alerts = append(g.alerts, a)
		if a.Level == "high" {
			g.highCount++
		}
		g.totalScore += a.SeverityScore
		if a.Dimension != "" {
			g.dimensions[a.Dimension] = true
		}
	}

	// Build summaries
	summaries := make([]StockRiskSummary, 0, len(groups))
	for code, g := range groups {
		// Aggregate score: base on severity, amplify by count and dimensions
		baseScore := g.totalScore / len(g.alerts)
		countBonus := math.Min(float64(len(g.alerts))*5, 30)
		dimBonus := math.Min(float64(len(g.dimensions))*10, 30)
		aggScore := int(math.Min(float64(baseScore)+countBonus+dimBonus, 100))

		// Find top rule
		topAlert := g.alerts[0]
		for _, a := range g.alerts {
			if a.SeverityScore > topAlert.SeverityScore {
				topAlert = a
			}
		}

		dims := make([]string, 0, len(g.dimensions))
		for d := range g.dimensions {
			dims = append(dims, d)
		}

		summaries = append(summaries, StockRiskSummary{
			StockCode:  code,
			AlertCount: len(g.alerts),
			HighCount:  g.highCount,
			RiskScore:  aggScore,
			TopRule:    topAlert.RuleKey,
			TopType:    topAlert.Type,
			Dimensions: dims,
		})
	}

	// Sort by risk score descending
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RiskScore > summaries[j].RiskScore
	})

	// Enrich names
	for i := range summaries {
		type NameRow struct {
			Code string `gorm:"column:code"`
			Name string `gorm:"column:name"`
		}
		var names []NameRow
		db.PG.Raw("SELECT code, name FROM stocks_basic WHERE code IN ?", 
			[]string{summaries[i].StockCode}).Scan(&names)
		if len(names) > 0 {
			summaries[i].StockName = names[0].Name
		}
	}

	response.Success(c, summaries)
}
