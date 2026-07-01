package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type StrategyHandler struct {
	aiSvc *service.AIService
}

func NewStrategyHandler() *StrategyHandler { return &StrategyHandler{aiSvc: service.NewAIService()} }

// ── Strategy CRUD ──

func (h *StrategyHandler) List(c *gin.Context) {
	uid := getUID(c)
	var strategies []model.Strategy
	db.MySQL.Where("user_id = ?", uid).Order("sort_order ASC, id ASC").Find(&strategies)

	// If exclude_pk=true, filter out strategies already in active PK events
	if c.Query("exclude_pk") == "true" {
		var activeSids []uint
		db.MySQL.Model(&model.PkEntry{}).
			Joins("JOIN pk_events ON pk_events.id = pk_entries.event_id").
			Where("pk_entries.user_id = ? AND pk_events.status IN (?)", uid, []string{"enrolling", "running"}).
			Pluck("pk_entries.strategy_id", &activeSids)
		if len(activeSids) > 0 {
			filtered := make([]model.Strategy, 0)
			activeSet := make(map[uint]bool)
			for _, sid := range activeSids {
				activeSet[sid] = true
			}
			for _, s := range strategies {
				if !activeSet[s.ID] {
					filtered = append(filtered, s)
				}
			}
			strategies = filtered
		}
	}

	log.Printf("[strategy] list uid=%d count=%d", uid, len(strategies))
	response.Success(c, strategies)
}

func (h *StrategyHandler) Create(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.BadRequest(c, "策略名称不能为空")
		return
	}
	var count int64
	db.MySQL.Model(&model.Strategy{}).Where("user_id = ?", uid).Count(&count)
	s := model.Strategy{
		UserID:    uid,
		Name:      body.Name,
		SortOrder: int(count),
	}
	if count == 0 {
		s.IsDefault = true
	}
	if err := db.MySQL.Create(&s).Error; err != nil {
		log.Printf("[strategy] create error: %v", err)
		response.InternalError(c, "创建失败")
		return
	}
	log.Printf("[strategy] created id=%d name=%s uid=%d", s.ID, s.Name, uid)
	response.Created(c, s)
}


func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64: return val, true
	case float32: return float64(val), true
	case int: return float64(val), true
	case int64: return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

func (h *StrategyHandler) Update(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	// Use raw map first to detect which fields were actually sent
	raw := make(map[string]interface{})
	if err := c.ShouldBindJSON(&raw); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if v, ok := raw["name"]; ok && v != "" { updates["name"] = v }
	if _, ok := raw["description"]; ok { updates["description"] = raw["description"] }
	if _, ok := raw["stopProfit"]; ok { updates["stop_profit"] = raw["stopProfit"] }
	if _, ok := raw["stopLoss"]; ok { updates["stop_loss"] = raw["stopLoss"] }
	if v, ok := raw["maxHoldings"]; ok { if n, _ := toFloat64(v); n > 0 { updates["max_holdings"] = int(n) } }
	if v, ok := raw["initialCapital"]; ok { if n, _ := toFloat64(v); n > 0 { updates["initial_capital"] = n } }
	if v, ok := raw["buyPositionPct"]; ok { if n, _ := toFloat64(v); n > 0 { updates["buy_position_pct"] = n } }
	if v, ok := raw["addPositionPct"]; ok { if n, _ := toFloat64(v); n > 0 { updates["add_position_pct"] = n } }
	if v, ok := raw["reducePositionPct"]; ok { if n, _ := toFloat64(v); n > 0 { updates["reduce_position_pct"] = n } }
	if v, ok := raw["positionConcentrationLimit"]; ok { if n, _ := toFloat64(v); n > 0 { updates["position_concentration_limit"] = n } }
	if v, ok := raw["maxDailyLoss"]; ok { if n, _ := toFloat64(v); n < 0 { updates["max_daily_loss"] = n } }
	// Trailing stop
	if _, ok := raw["enableTrailingStop"]; ok { updates["enable_trailing_stop"] = raw["enableTrailingStop"] }
	if _, ok := raw["trailingStopActivation"]; ok { updates["trailing_stop_activation"] = raw["trailingStopActivation"] }
	if _, ok := raw["trailingStopDrawdown"]; ok { updates["trailing_stop_drawdown"] = raw["trailingStopDrawdown"] }
	// Dip buy
	if _, ok := raw["enableDipBuy"]; ok { updates["enable_dip_buy"] = raw["enableDipBuy"] }
	if _, ok := raw["dipBuyThreshold"]; ok { updates["dip_buy_threshold"] = raw["dipBuyThreshold"] }
	if _, ok := raw["dipBuyAmountPct"]; ok { updates["dip_buy_amount_pct"] = raw["dipBuyAmountPct"] }
	if _, ok := raw["dipTargetReturn"]; ok { updates["dip_target_return"] = raw["dipTargetReturn"] }
	if _, ok := raw["dipMaxHoldDays"]; ok { updates["dip_max_hold_days"] = raw["dipMaxHoldDays"] }
	if _, ok := raw["dipCooldownDays"]; ok { updates["dip_cooldown_days"] = raw["dipCooldownDays"] }
	// Grid trading
	if _, ok := raw["enableGrid"]; ok { updates["enable_grid"] = raw["enableGrid"] }
	if _, ok := raw["gridTriggerSqueeze"]; ok { updates["grid_trigger_squeeze"] = raw["gridTriggerSqueeze"] }
	if _, ok := raw["gridLevels"]; ok { updates["grid_levels"] = raw["gridLevels"] }
	if _, ok := raw["gridLotPct"]; ok { updates["grid_lot_pct"] = raw["gridLotPct"] }
	if v, ok := raw["investmentType"]; ok && v != "" { updates["investment_type"] = v }
	if _, ok := raw["regularAmount"]; ok { updates["regular_amount"] = raw["regularAmount"] }
	if v, ok := raw["regularInterval"]; ok && v != "" { updates["regular_interval"] = v }
	if _, ok := raw["stockCodes"]; ok { updates["stock_codes"] = raw["stockCodes"] }
	if _, ok := raw["enableMarketContext"]; ok { updates["enable_market_context"] = raw["enableMarketContext"] }
	if _, ok := raw["defensiveThreshold"]; ok { updates["defensive_threshold"] = raw["defensiveThreshold"] }
	if _, ok := raw["aggressiveThreshold"]; ok { updates["aggressive_threshold"] = raw["aggressiveThreshold"] }
	if _, ok := raw["marketCompositeMin"]; ok { updates["market_composite_min"] = raw["marketCompositeMin"] }
	db.MySQL.Model(&model.Strategy{}).Where("id = ? AND user_id = ?", id, uid).Updates(updates)
	response.SuccessMsg(c, "更新成功")
}

func (h *StrategyHandler) Delete(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.Strategy{})
	db.MySQL.Where("strategy_id = ?", id).Delete(&model.StrategyCondition{})
	response.SuccessMsg(c, "已删除")
}

func (h *StrategyHandler) Reorder(c *gin.Context) {
	uid := getUID(c)
	var body struct{ IDs []uint `json:"ids"` }
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for i, id := range body.IDs {
		db.MySQL.Model(&model.Strategy{}).Where("id = ? AND user_id = ?", id, uid).Update("sort_order", i)
	}
	response.SuccessMsg(c, "ok")
}

// ── Conditions ──

func (h *StrategyHandler) ListConditions(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	// Verify ownership
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}
	var conds []model.StrategyCondition
	db.MySQL.Where("strategy_id = ?", sid).Order("cond_type, logic_group, sort_order").Find(&conds)
	response.Success(c, conds)
}

func (h *StrategyHandler) SaveConditions(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}
	var body struct {
		Conditions []model.StrategyCondition `json:"conditions"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// Replace all conditions
	db.MySQL.Where("strategy_id = ?", sid).Delete(&model.StrategyCondition{})
	for i := range body.Conditions {
		body.Conditions[i].ID = 0
		body.Conditions[i].StrategyID = uint(sid)
		if body.Conditions[i].LogicGroup == 0 {
			body.Conditions[i].LogicGroup = 1
		}
		db.MySQL.Create(&body.Conditions[i])
	}
	response.SuccessMsg(c, "条件保存成功")
}

// ── AI Generate Strategy ──

func (h *StrategyHandler) AIGenerate(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Style       string `json:"style"` // aggressive / moderate / conservative
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	style := body.Style
	if style == "" {
		style = "moderate"
	}

	cfg, err := h.aiSvc.GetConfig(uid)
	if err != nil || cfg.APIKey == "" {
		response.Error(c, 400, response.CodeAIConfigMissing, "请先配置AI模型")
		return
	}

	// Dynamically generated indicator list from Registry for AI prompt
	indicators := buildIndicatorReference()

	prompt := h.buildAIGeneratePrompt(indicators, body.Name, body.Description, style)

	reply, err := h.aiSvc.ChatCompletionWithTokensModule(uid, prompt, nil, 4096, "strategy_gen")
	if err != nil {
		response.Error(c, 500, response.CodeAIModelError, "AI生成失败: "+err.Error())
		return
	}
	if reply == "" {
		response.Error(c, 500, response.CodeAIModelError, "AI返回空内容，请检查模型配置或稍后重试")
		return
	}

	// Parse JSON response with flexible value types
	reply = cleanJSON(reply)
	if reply == "" {
		response.Error(c, 500, response.CodeAIModelError, "AI返回内容无法解析，可能未按要求返回JSON格式")
		return
	}
	var rawResult struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		StopProfit  float64 `json:"stopProfit"`
		StopLoss    float64 `json:"stopLoss"`
		MaxHoldings int     `json:"maxHoldings"`
		Conditions  []struct {
			CondType   string      `json:"condType"`
			Indicator  string      `json:"indicator"`
			Operator   string      `json:"operator"`
			Value      interface{} `json:"value"` // accept both string and number
			LogicGroup int         `json:"logicGroup"`
			SortOrder  int         `json:"sortOrder"`
		} `json:"conditions"`
	}
	if err := json.Unmarshal([]byte(reply), &rawResult); err != nil {
		preview := reply
		if len(preview) > 300 { preview = preview[:300] + "..." }
		response.Error(c, 500, response.CodeAIModelError, "AI返回解析失败: "+err.Error()+" | 原文: "+preview)
		return
	}

	// Convert conditions with flexible values
	result := struct {
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		StopProfit  float64                  `json:"stopProfit"`
		StopLoss    float64                  `json:"stopLoss"`
		MaxHoldings int                      `json:"maxHoldings"`
		Conditions  []model.StrategyCondition `json:"conditions"`
	}{
		Name:        rawResult.Name,
		Description: rawResult.Description,
		StopProfit:  rawResult.StopProfit,
		StopLoss:    rawResult.StopLoss,
		MaxHoldings: rawResult.MaxHoldings,
	}
	// Backward compatibility: map old indicator names to new ones
	oldToNew := map[string]string{
		"pe_level": "pe_percentile",
	}
	for _, rc := range rawResult.Conditions {
		if newName, ok := oldToNew[rc.Indicator]; ok {
			rc.Indicator = newName
		}
		v := parseValue(rc.Value, rc.Indicator, rc.Operator)
		result.Conditions = append(result.Conditions, model.StrategyCondition{
			CondType:   rc.CondType,
			Indicator:  rc.Indicator,
			Operator:   rc.Operator,
			Value:      v,
			LogicGroup: rc.LogicGroup,
			SortOrder:  rc.SortOrder,
		})
	}

	response.Success(c, result)
}

// buildIndicatorReference generates a detailed, structured indicator reference table
// for AI prompt context. Groups indicators by category with full metadata:
// field name, label, type, valid operators, value range, description, and usage suggestion.
func buildIndicatorReference() string {
	type catGroup struct {
		name    string
		entries []*IndicatorMeta
	}
	groups := make(map[string]*catGroup)
	categoryOrder := []string{
		"榜单与评分", "AI评分",
		"技术面-趋势", "技术面-趋势系统",
		"技术面-超买超卖",
		"技术面-量价", "技术面-波动", "技术面-形态",
		"估值", "基本面", "资金面", "预测",
	}

	for _, m := range IndicatorRegistry {
		group, ok := groups[m.Category]
		if !ok {
			group = &catGroup{name: m.Category}
			groups[m.Category] = group
		}
		group.entries = append(group.entries, m)
	}

	opSymbol := func(op string) string {
		switch op {
		case "gte": return "≥"
		case "lte": return "≤"
		case "gt": return ">"
		case "lt": return "<"
		case "eq": return "="
		case "cross_up": return "↑上穿"
		case "cross_down": return "↓下穿"
		default: return op
		}
	}
	opsJoin := func(ops []string) string {
		parts := make([]string, len(ops))
		for i, o := range ops {
			parts[i] = opSymbol(o)
		}
		return strings.Join(parts, "/")
	}

	var sb strings.Builder
	sb.WriteString("## 可用指标参考（共 ")
	sb.WriteString(strconv.Itoa(len(IndicatorRegistry)))
	sb.WriteString(" 项）\n\n")

	for _, cat := range categoryOrder {
		g, ok := groups[cat]
		if !ok || len(g.entries) == 0 {
			continue
		}
		// Sort entries within category by key
		sort.Slice(g.entries, func(i, j int) bool {
			return g.entries[i].Key < g.entries[j].Key
		})
		sb.WriteString("### ")
		sb.WriteString(g.name)
		sb.WriteString("\n\n")
		sb.WriteString("| 字段名 | 名称 | 类型 | 可用操作符 | 值域 | 用途 | 说明 |\n")
		sb.WriteString("|--------|------|------|-----------|------|------|------|\n")
		for _, m := range g.entries {
			typeStr := m.Type
			valueRange := buildValueRangeHint(m)
			useFor := ""
			switch m.UseFor {
			case "buy": useFor = "买入"
			case "sell": useFor = "卖出"
			case "both": useFor = "买卖"
			}
			sb.WriteString("| `")
			sb.WriteString(m.Key)
			sb.WriteString("` | ")
			sb.WriteString(m.Label)
			sb.WriteString(" | ")
			sb.WriteString(typeStr)
			sb.WriteString(" | ")
			sb.WriteString(opsJoin(m.Operators))
			sb.WriteString(" | ")
			sb.WriteString(valueRange)
			sb.WriteString(" | ")
			sb.WriteString(useFor)
			sb.WriteString(" | ")
			sb.WriteString(m.Desc)
			if m.Suggestion != "" {
				sb.WriteString("。")
				sb.WriteString(m.Suggestion)
			}
			if !m.BacktestSafe {
				sb.WriteString("（回测禁用）")
			}
			sb.WriteString(" |\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildValueRangeHint returns a concise value range hint from indicator metadata
func buildValueRangeHint(m *IndicatorMeta) string {
	switch m.Key {
	// Score types
	case "algo_score", "ai_score", "ai_fundamental", "ai_technical",
		"ai_valuation", "ai_growth", "ai_industry", "ai_capital":
		return "0-10"
	// RSI family
	case "rsi", "rsi_6", "rsi_12", "rsi_24":
		return "0-100（>70超买/<30超卖）"
	// KDJ family
	case "kdj_k", "kdj_d", "kdj_j":
		return "0-100"
	// Bollinger
	case "boll_position":
		return "0-100（>80上轨/<20下轨）"
	case "boll_width":
		return "%（变大=波动加剧）"
	case "boll_squeeze":
		return "%（越小=即将突破）"
	case "boll_upper", "boll_middle", "boll_lower":
		return "元（股价相关）"
	// CCI
	case "cci":
		return "-300~300（>100超买/<-100超卖）"
	// Williams
	case "williams_r":
		return "-100~0（>-20超买/<-80超卖）"
	// MFI
	case "mfi":
		return "0-100（>80超买/<20超卖）"
	// Volume
	case "volume_ratio":
		return ">0（>2放量/<0.5缩量）"
	case "volume_ma_ratio":
		return ">0（>1.2放量）"
	case "turnover_rate":
		return "%（>5活跃/<1冷清）"
	// ADX/DMI
	case "adx":
		return "0-100（>25趋势强）"
	case "dmi_plus", "dmi_minus":
		return "0-100（PDI>MDI=多头）"
	// Cross types
	case "ma_cross", "ema_cross":
		return "短/长周期，如 5/20"
	case "macd":
		return "金叉/死叉信号（仅cross_up/cross_down）"
	// MA values
	case "ma_5", "ma_10", "ma_20", "ma_30", "ma_60":
		return "元（股价均线值）"
	// MACD components
	case "macd_dif", "macd_dea":
		return "元（MACD指标值）"
	// Percentile
	case "pe_percentile", "pb_percentile":
		return "0-100（<30低估/>70高估）"
	// MA derived
	case "ma_convergence":
		return "%（越小=均线粘合）"
	case "ma_deviation":
		return "%（正=股价高于均线）"
	// Daily change / momentum
	case "daily_change", "momentum_5", "momentum_20":
		return "%（正=上涨）"
	// Gap
	case "gap_pct":
		return "%（正=向上跳空）"
	// Drawdown
	case "drawdown_20":
		return "%（负=回撤，越小越好）"
	// Price position
	case "price_position_20", "price_position_60":
		return "%（100=最高价附近）"
	// High-low range
	case "high_low_range":
		return "%（日内振幅）"
	// New high
	case "new_high_20":
		return "0/1（1=创20日新高）"
	// Consecutive
	case "consecutive_days":
		return "天（正=连涨/负=连跌）"
	// Up days ratio
	case "up_days_ratio":
		return "0-1（>0.6偏强）"
	// Trend strength
	case "trend_strength":
		return "0-1（>0.6趋势明确）"
	// Index relative
	case "index_relative":
		return "%（正=跑赢大盘）"
	// Volume trend
	case "volume_trend":
		return "0-1（>0.6量能向上）"
	// VWAP
	case "vwap_deviation":
		return "%（正=高于加权均价）"
	// ATR
	case "atr":
		return "元（波动绝对值）"
	case "atr_pct":
		return "%（波动率，>3高波动）"
	// PE/PB/PS
	case "pe":
		return "倍（>0，<20低估）"
	case "pb":
		return "倍（>0，<2低估）"
	case "ps":
		return "倍"
	// Fundamentals
	case "roe":
		return "%（>15优秀）"
	case "eps":
		return "元"
	case "revenue_growth":
		return "%（>20高增长）"
	case "profit_growth":
		return "%（>20高增长）"
	case "gross_margin":
		return "%（>40优秀）"
	case "net_margin":
		return "%（>15优秀）"
	case "debt_ratio":
		return "%（<60安全）"
	// Market cap
	case "total_market_cap":
		return "元（大盘>1e11）"
	// Shareholder
	case "shareholder_change":
		return "%（负=减少=筹码集中）"
	case "inst_hold_ratio":
		return "%（>30机构看好）"
	// Prediction
	case "prediction_upside":
		return "%（>10看涨）"
	case "prediction_consensus":
		return "0-1（>0.6看涨）"
	// Streak / signal
	case "streak_count":
		return "次数（≥3持续关注）"
	case "signal_value":
		return "比值（>0.5偏多）"
	// PSY
	case "psy_12", "psy_ma":
		return "0-100（>75超买/<25超卖）"
	default:
		if m.Type == "cross" {
			return "信号（仅cross_up/cross_down）"
		}
		if m.Unit == "%" {
			return "%"
		}
		if m.Unit == "元" {
			return "元"
		}
		if m.Unit == "倍" {
			return "倍"
		}
		return m.Unit
	}
}

// GetIndicatorList returns all indicator metadata sorted by category+key (for API/tool use)
func GetIndicatorList() []*IndicatorMeta {
	result := make([]*IndicatorMeta, 0, len(IndicatorRegistry))
	for _, m := range IndicatorRegistry {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].Key < result[j].Key
	})
	return result
}
// loadSystemConfig loads AI system config for a scene, returning defaults if not found
func (h *StrategyHandler) loadSystemConfig(scene string) model.AISystemConfig {
	var cfg model.AISystemConfig
	if err := db.PG.Where("scene = ?", scene).First(&cfg).Error; err != nil {
		return model.AISystemConfig{
			Scene: scene,
			SystemPrompt: "",
			Temperature: 0.7,
			MaxTokens: 4096,
		}
	}
	return cfg
}

// buildAIGeneratePrompt constructs the full AI prompt with indicator context.
func (h *StrategyHandler) buildAIGeneratePrompt(indicators, name, description, style string) string {
	cfg := h.loadSystemConfig("strategy_gen")
	basePrompt := cfg.SystemPrompt
	if basePrompt == "" {
		basePrompt = `你是量化策略专家。根据用户描述生成A股策略JSON。
返回纯JSON（无markdown，不要markdown代码块，只返回JSON对象）：
{
  "name": "策略名称",
  "description": "策略描述",
  "stopProfit": 15,
  "stopLoss": -8,
  "maxHoldings": 10,
  "conditions": [
    {"condType": "buy", "indicator": "algo_score", "operator": "gte", "value": 6, "logicGroup": 1, "sortOrder": 0}
  ]
}`
	}
	
	indicatorRules := `__INDICATORS__

## 条件构建规范

请严格根据上方「可用指标参考」表构建每一条条件：

### 字段映射规则
- ` + "`indicator`" + ` → 必须使用参考表「字段名」列的值（如 ` + "`algo_score`" + `、` + "`rsi`" + `、` + "`daily_change`" + `）
- ` + "`operator`" + ` → 必须使用参考表「可用操作符」列中的某一个，英文映射为：
  ≥→gte, ≤→lte, >→gt, <→lt, =→eq, ↑上穿→cross_up, ↓下穿→cross_down
- ` + "`value`" + ` → 必须使用参考表「值域」列建议的数值范围
- ` + "`condType`" + ` → 参考表「用途」列：买入→buy, 卖出→sell, 买卖→两者均可

### 类型特殊规则
- **cross 类型**（ma_cross/ema_cross/macd）：operator 只能用 cross_up 或 cross_down，value 为 "短/长" 如 "5/20"
- **评分类型**（algo_score/ai_*）：value 0-10，建议买入 ≥6、卖出 ≤3
- **RSI/KDJ**：value 0-100，超买>70 卖出、超卖<30 买入
- **pe/pb**：单位是倍，<20低估可买入，>50高估考虑卖出
- **% 单位指标**：value 直接写数字如 5 表示 5%
- **元单位指标**（ma_*/boll_*/macd_*）：value 是股价绝对值
- **信号/比值型**（new_high_20/volume_trend）：value 为 0 或 1

### 数量与组织规则
- 最多生成 12 条条件（买入+卖出合计）
- 同一 logicGroup 内条件为 AND 关系，不同 logicGroup 为 OR 关系
- 根据投资风格调整阈值：aggressive(激进) 放宽阈值，conservative(保守) 收紧阈值

### 输出格式（纯JSON，无markdown代码块）
{
  "name": "策略名称",
  "description": "策略描述（≤50字）",
  "stopProfit": 15,
  "stopLoss": -8,
  "maxHoldings": 10,
  "conditions": [
    {"condType": "buy", "indicator": "algo_score", "operator": "gte", "value": 6, "logicGroup": 1, "sortOrder": 0},
    {"condType": "buy", "indicator": "rsi", "operator": "lt", "value": 40, "logicGroup": 1, "sortOrder": 1},
    {"condType": "sell", "indicator": "daily_change", "operator": "lt", "value": -5, "logicGroup": 2, "sortOrder": 2}
  ]
}`
	
	fullPrompt := basePrompt + "\n\n" + indicatorRules + "\n\n用户策略名: __STRATEGY_NAME__\n用户描述: __STRATEGY_DESC__\n风险偏好: __STRATEGY_STYLE__"
	vars := map[string]string{
		"INDICATORS": indicators,
		"STRATEGY_NAME": name,
		"STRATEGY_DESC": description,
		"STRATEGY_STYLE": style,
	}
	return renderPrompt(fullPrompt, vars)
}

// ── Prompt Optimizer ──

func (h *StrategyHandler) OptimizePrompt(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Prompt string `json:"prompt"`
		Style  string `json:"style"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Prompt == "" {
		response.BadRequest(c, "请输入描述要求")
		return
	}
	cfg, err := h.aiSvc.GetConfig(uid)
	if err != nil || cfg.APIKey == "" {
		response.Error(c, 400, response.CodeAIConfigMissing, "请先配置AI模型")
		return
	}
	style := body.Style
	if style == "" { style = "moderate" }
	sysCfg := h.loadSystemConfig("strategy_opt")
	optPrompt := sysCfg.SystemPrompt
	if optPrompt == "" {
		optPrompt = "请将以下用户要求优化为结构化的策略描述，包含：投资风格、选股偏好、买入时机、卖出时机、仓位管理、风险控制等方面。\n\n用户原始要求：__USER_PROMPT__\n风险偏好：__STRATEGY_STYLE__\n\n优化后的策略描述："
	}
	prompt := renderPrompt(optPrompt, map[string]string{"USER_PROMPT": body.Prompt, "STRATEGY_STYLE": style})
	reply, err := h.aiSvc.ChatCompletionWithTokensModule(uid, prompt, nil, 4096, "strategy_opt")
	if err != nil {
		response.Error(c, 500, response.CodeAIModelError, "AI优化失败: "+err.Error())
		return
	}
	response.Success(c, map[string]string{"optimized": reply})
}


// ── Orchestration Endpoints ──

func (h *StrategyHandler) GetOrchestration(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var s model.Strategy
	if err := db.MySQL.Where("id = ? AND user_id = ?", id, uid).First(&s).Error; err != nil { response.NotFound(c, "策略不存在"); return }
	response.Success(c, map[string]interface{}{
		"orchestrationMode": s.OrchestrationMode, "enableMarketContext": s.EnableMarketContext,
		"marketCompositeMin": s.MarketCompositeMin, "marketPositionBias": s.MarketPositionBias,
		"enableAIAgent": s.EnableAIAgent, "aiAgentMode": s.AIAgentMode,
		"aiAgentReviewScope": s.AIAgentReviewScope, "aiAgentMaxDailyTrades": s.AIAgentMaxDailyTrades,
		"industryFilter": s.IndustryFilter, "enableSectorRotation": s.EnableSectorRotation,
		"policyMode": s.PolicyMode,
		"defensiveThreshold": s.DefensiveThreshold,
		"policyAggressive": s.PolicyAggressive, "policyDefensive": s.PolicyDefensive, "policyCash": s.PolicyCash,
	})
}

func (h *StrategyHandler) SaveOrchestration(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil { response.BadRequest(c, "参数错误"); return }
	updates := map[string]interface{}{}
	if v, ok := body["orchestrationMode"]; ok { updates["orchestration_mode"] = v }
	if v, ok := body["enableMarketContext"]; ok { updates["enable_market_context"] = v }
	if v, ok := body["marketCompositeMin"]; ok { updates["market_composite_min"] = v }
	if v, ok := body["marketPositionBias"]; ok { updates["market_position_bias"] = v }
	if v, ok := body["enableAIAgent"]; ok { updates["enable_ai_agent"] = v }
	if v, ok := body["aiAgentMode"]; ok { updates["ai_agent_mode"] = v }
	if v, ok := body["aiAgentReviewScope"]; ok { updates["ai_agent_review_scope"] = v }
	if v, ok := body["aiAgentMaxDailyTrades"]; ok { updates["ai_agent_max_daily_trades"] = v }
	if v, ok := body["industryFilter"]; ok { updates["industry_filter"] = v }
	if v, ok := body["enableSectorRotation"]; ok { updates["enable_sector_rotation"] = v }
	if v, ok := body["policyMode"]; ok { updates["policy_mode"] = v }
	if v, ok := body["aggressiveThreshold"]; ok { updates["aggressive_threshold"] = v }
	if v, ok := body["defensiveThreshold"]; ok { updates["defensive_threshold"] = v }
	if v, ok := body["policyAggressive"]; ok { if b, err := json.Marshal(v); err == nil { updates["policy_aggressive"] = string(b) } }
	if v, ok := body["policyDefensive"]; ok { if b, err := json.Marshal(v); err == nil { updates["policy_defensive"] = string(b) } }
	if v, ok := body["policyCash"]; ok { if b, err := json.Marshal(v); err == nil { updates["policy_cash"] = string(b) } }
	db.MySQL.Model(&model.Strategy{}).Where("id = ? AND user_id = ?", id, uid).Updates(updates)
	response.SuccessMsg(c, "编排配置已保存")
}

func (h *StrategyHandler) ListTemplates(c *gin.Context) {
	uid := getUID(c)
	var templates []model.ConditionTemplate
	db.MySQL.Where("is_system = ? OR created_by = ?", true, uid).Order("is_system DESC, id ASC").Find(&templates)
	response.Success(c, templates)
}

func (h *StrategyHandler) CreateTemplate(c *gin.Context) {
	uid := getUID(c)
	var body struct { Name string `json:"name"`; Description string `json:"description"`; Category string `json:"category"`; CondType string `json:"condType"` }
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" { response.BadRequest(c, "模板名称不能为空"); return }
	tmpl := model.ConditionTemplate{Name: body.Name, Description: body.Description, Category: body.Category, CondType: body.CondType, CreatedBy: uid}
	if err := db.MySQL.Create(&tmpl).Error; err != nil { response.InternalError(c, "创建模板失败"); return }
	response.Created(c, tmpl)
}

func (h *StrategyHandler) ListAIDecisions(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var decisions []model.AIAgentDecision
	db.PG.Where("strategy_id = ?", id).Order("created_at DESC").Limit(50).Find(&decisions)
	response.Success(c, decisions)
}

func (h *StrategyHandler) AIReview(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var s model.Strategy
	if err := db.MySQL.Where("id = ? AND user_id = ?", id, uid).First(&s).Error; err != nil { response.NotFound(c, "策略不存在"); return }
	if !s.EnableAIAgent { response.BadRequest(c, "该策略未启用AI代理"); return }
	response.SuccessMsg(c, "AI审查请求已提交（异步）")
}


// ── Backtest ──

// ═══════════════════════════════════════════════════════════════
// 异步回测引擎
// ═══════════════════════════════════════════════════════════════

type backtestTrade struct {
	Date       string  `json:"date"`
	SignalDate string  `json:"signalDate"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Action     string  `json:"action"`
	Price      float64 `json:"price"`
	Quantity   int     `json:"quantity"`
	Reason     string  `json:"reason"`
	Pnl        float64 `json:"pnl"`
	PnlPct     float64 `json:"pnlPct"`
}

// map of running tasks per strategy: strategyID -> map[taskID]context.CancelFunc
var runningBacktests = make(map[uint]map[uint]context.CancelFunc)

func init() {
	// Initialize outer map lazily; access protected by getRunningMap
}

func getRunningMap(sid uint) map[uint]context.CancelFunc {
	if runningBacktests[sid] == nil {
		runningBacktests[sid] = make(map[uint]context.CancelFunc)
	}
	return runningBacktests[sid]
}

// ── Start async backtest ──

// resolveStockPool converts a pool key + fallback codes into actual stock code list
func resolveStockPool(uid uint, poolKey string, fallbackCodes []string) []string {
	if poolKey == "" {
		return fallbackCodes
	}
	switch {
	case poolKey == "all":
		return nil // nil signals "all stocks" to runBacktestAsync
	case strings.HasPrefix(poolKey, "watchlist_"):
		gidStr := strings.TrimPrefix(poolKey, "watchlist_")
		gid, err := strconv.Atoi(gidStr)
		if err != nil {
			return fallbackCodes
		}
		var codes []string
		db.MySQL.Raw("SELECT stock_code FROM watchlists WHERE user_id = ? AND group_id = ?", uid, gid).Pluck("stock_code", &codes)
		return codes
	case poolKey == "portfolio":
		var codes []string
		db.MySQL.Raw("SELECT DISTINCT stock_code FROM holdings WHERE user_id = ?", uid).Pluck("stock_code", &codes)
		return codes
	}
	return fallbackCodes
}

// resolveStockPoolLabel converts a pool key to a human-readable display label.
// Keys: "all", "watchlist_N", "portfolio", "codes"
// Uses stockPoolParams JSON to derive the count.
func resolveStockPoolLabel(poolKey, poolParamsJSON string) string {
	count := 0
	if poolParamsJSON != "" {
		var codes []string
		if json.Unmarshal([]byte(poolParamsJSON), &codes) == nil {
			count = len(codes)
		}
	}
	var label string
	switch {
	case poolKey == "all":
		label = "全部股票"
	case strings.HasPrefix(poolKey, "watchlist_"):
		label = "自选组"
	case poolKey == "portfolio":
		label = "我的持仓"
	case poolKey == "codes":
		label = "自选代码"
	default:
		label = poolKey // fallback for legacy data
	}
	if count > 0 {
		return fmt.Sprintf("%s (%d只)", label, count)
	}
	return label
}

func (h *StrategyHandler) StartBacktest(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		StartDate  string   `json:"startDate"`
		EndDate    string   `json:"endDate"`
		StockCodes []string `json:"stockCodes"`
		StockPool  string   `json:"stockPool"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Resolve stock pool to actual stock codes
	resolvedCodes := resolveStockPool(uid, body.StockPool, body.StockCodes)

	// Load strategy
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}

	// Validate date range
	startDate, _ := time.Parse("2006-01-02", body.StartDate)
	endDate, _ := time.Parse("2006-01-02", body.EndDate)
	if startDate.After(endDate) {
		response.BadRequest(c, "开始日期不能晚于结束日期")
		return
	}

	// Count trading days for estimate
	var totalDays int
	if err := db.PG.Raw(`SELECT COUNT(DISTINCT trade_date) FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ?`, body.StartDate, body.EndDate).Scan(&totalDays).Error; err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询交易日数据失败: "+err.Error())
		return
	}
	if totalDays == 0 {
		response.BadRequest(c, "所选时间段无交易日数据")
		return
	}

	// Serialize params
	paramsBytes, _ := json.Marshal(map[string]interface{}{
		"startDate":   body.StartDate,
		"endDate":     body.EndDate,
		"stockCodes":  resolvedCodes,
		"stockPool":   body.StockPool,
	})

	// Create task
	task := model.BacktestTask{
		UserID:     uid,
		StrategyID: uint(sid),
		Status:     "pending",
		Phase:      "排队中",
		TotalDays:  totalDays,
		Params:     string(paramsBytes),
	}
	db.MySQL.Create(&task)

	// Launch goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	getRunningMap(uint(sid))[task.ID] = cancel

	go h.runBacktestAsync(ctx, &task, &s, body.StartDate, body.EndDate, resolvedCodes)

	response.Created(c, map[string]interface{}{
		"taskId":    task.ID,
		"totalDays": totalDays,
		"status":    "pending",
	})
}

// ── Poll task status ──

func (h *StrategyHandler) BacktestStatus(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	resp := map[string]interface{}{
		"taskId":     task.ID,
		"status":     task.Status,
		"phase":      task.Phase,
		"currentDay": task.CurrentDay,
		"totalDays":  task.TotalDays,
		"progressPct": task.ProgressPct,
		"errorMsg":   task.ErrorMsg,
		"resultId":   task.ResultID,
		"startedAt":  task.StartedAt,
		"completedAt": task.CompletedAt,
	}

	// Include current positions snapshot if available
	if task.CurrentPositions != "" {
		var positions interface{}
		if err := json.Unmarshal([]byte(task.CurrentPositions), &positions); err == nil {
			resp["currentPositions"] = positions
		}
	}

	response.Success(c, resp)
}

// ── SSE stream for live viewing (reconnectable) ──

func (h *StrategyHandler) BacktestStream(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, 500, response.CodeInternalError, "不支持流式输出")
		return
	}

	sendSSE := func(typ string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: {\"type\":\"%s\",\"payload\":%s}\n\n", typ, string(b))
		flusher.Flush()
	}

	// Send current snapshot immediately
	sendSSE("status", map[string]interface{}{
		"status":      task.Status,
		"phase":       task.Phase,
		"currentDay":  task.CurrentDay,
		"totalDays":   task.TotalDays,
		"progressPct": task.ProgressPct,
	})

	if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
		if task.Status == "completed" && task.ResultID != nil {
			var bt model.BacktestResult
			db.MySQL.First(&bt, *task.ResultID)
			metrics := map[string]interface{}{
				"totalReturn": bt.TotalReturn,
				"sharpeRatio": bt.SharpeRatio,
				"maxDrawdown": bt.MaxDrawdown,
				"winRate":     bt.WinRate,
				"tradeCount":  bt.TradeCount,
				"resultId":    bt.ID,
			}
			sendSSE("metric", metrics)
			// Send trades and equity curve
			if tradesData, ok := bt.Trades["data"]; ok {
				sendSSE("trades", tradesData)
			}
			if equityData, ok := bt.EquityCurve["data"]; ok {
				sendSSE("equity", equityData)
			}
		}
		sendSSE("done", map[string]string{"message": "回测" + task.Status})
		return
	}

	// Poll task status periodically
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()
	lastDay := 0
	lastPositions := ""

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			db.MySQL.First(&task, tid)
			// Send phase updates
			if task.Phase != "" {
				sendSSE("phase", map[string]string{"phase": "", "message": task.Phase})
			}
			// Send position updates
			if task.CurrentDay != lastDay || task.CurrentPositions != lastPositions {
				lastDay = task.CurrentDay
				lastPositions = task.CurrentPositions
				sendSSE("position", map[string]interface{}{
					"day":       task.CurrentDay,
					"totalDays": task.TotalDays,
				})
				// If positions JSON available, parse and send
				if task.CurrentPositions != "" {
					var posData map[string]interface{}
					if json.Unmarshal([]byte(task.CurrentPositions), &posData) == nil {
						sendSSE("position", posData)
						// Emit individual trade events
						if trades, ok := posData["recentTrades"].([]interface{}); ok {
							for _, t := range trades {
								sendSSE("trade", t)
							}
						}
					}
				}
			}
			// Check terminal states
			if task.Status == "completed" {
				if task.ResultID != nil {
					var bt model.BacktestResult
					db.MySQL.First(&bt, *task.ResultID)
					metrics := map[string]interface{}{
						"totalReturn": bt.TotalReturn,
						"sharpeRatio": bt.SharpeRatio,
						"maxDrawdown": bt.MaxDrawdown,
						"winRate":     bt.WinRate,
						"tradeCount":  bt.TradeCount,
						"resultId":    bt.ID,
					}
					sendSSE("metric", metrics)
					if tradesData, ok := bt.Trades["data"]; ok {
						sendSSE("trades", tradesData)
					}
					if equityData, ok := bt.EquityCurve["data"]; ok {
						sendSSE("equity", equityData)
					}
				}
				sendSSE("done", map[string]string{"message": "回测完成"})
				return
			}
			if task.Status == "failed" {
				sendSSE("error", map[string]string{"message": task.ErrorMsg})
				sendSSE("done", map[string]string{"message": "回测失败"})
				return
			}
			if task.Status == "cancelled" {
				sendSSE("done", map[string]string{"message": "回测已取消"})
				return
			}
		}
	}
}

// ── Cancel running task ──

func (h *StrategyHandler) CancelBacktest(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// Only cancel active tasks; allow force-cancel for orphaned ones too
	canForce := task.Status == "running" || task.Status == "pending"

	// Try graceful cancel via context
	cancelled := false
	if task.Status == "running" || task.Status == "pending" {
		rm := getRunningMap(uint(sid))
		if cancel, ok := rm[uint(tid)]; ok {
			cancel()
			delete(rm, uint(tid))
			cancelled = true
		}
	}

	// Always update DB status (handles orphaned tasks after server restart)
	db.MySQL.Model(&task).Updates(map[string]interface{}{
		"status": "cancelled",
		"phase":  "已取消",
	})
	now := time.Now()
	db.MySQL.Model(&task).Update("completed_at", now)

	if cancelled {
		response.SuccessMsg(c, "已取消运行中的任务")
	} else if canForce {
		response.SuccessMsg(c, "已强制取消（任务可能已丢失上下文）")
	} else {
		response.SuccessMsg(c, "已取消")
	}
}

// ── List backtest tasks ──

func (h *StrategyHandler) BacktestTasks(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))

	var tasks []model.BacktestTask
	db.MySQL.Where("strategy_id = ? AND user_id = ?", sid, uid).
		Order("created_at DESC").Limit(50).Find(&tasks)
	response.Success(c, tasks)
}

// DeleteBacktestTask deletes a backtest task and all related child records.
// Cascades: execution_logs, daily_snapshots, and the linked backtest_result.
func (h *StrategyHandler) DeleteBacktestTask(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// Cancel if running
	if task.Status == "running" {
		rm := getRunningMap(uint(sid))
		if cancel, ok := rm[uint(tid)]; ok {
			cancel()
			delete(rm, uint(tid))
		}
	}

	// Cascade delete child tables
	db.MySQL.Where("task_id = ?", tid).Delete(&model.BacktestExecutionLog{})
	db.MySQL.Where("task_id = ?", tid).Delete(&model.BacktestDailySnapshot{})
	db.MySQL.Where("task_id = ?", tid).Delete(&model.BacktestResult{})

	// Delete the task itself
	db.MySQL.Delete(&task)

	log.Printf("[backtest] task %d deleted by user %d", tid, uid)
	response.SuccessMsg(c, "已删除")
}


// BacktestStockAnalysis returns per-stock profit analysis for a backtest task.
func (h *StrategyHandler) BacktestStockAnalysis(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// Query all executed signals for this task, grouped by stock
	type StockTrade struct {
		SignalDate string  `json:"signalDate"`
		ExecDate   string  `json:"execDate"`
		ActionType string  `json:"actionType"`
		ExecPrice  float64 `json:"execPrice"`
		ExecQty    int     `json:"execQty"`
		ExecAmount float64 `json:"execAmount"`
		Pnl        float64 `json:"pnl"`
		PnlPct     float64 `json:"pnlPct"`
		Reason     string  `json:"reason"`
	}

	type StockAnalysis struct {
		StockCode  string       `json:"stockCode"`
		StockName  string       `json:"stockName"`
		TotalPnl   float64      `json:"totalPnl"`
		TotalPnlPct float64     `json:"totalPnlPct"`
		BuyCount   int          `json:"buyCount"`
		SellCount  int          `json:"sellCount"`
		Trades     []StockTrade `json:"trades"`
	}

	var signals []model.BacktestSignal
	db.MySQL.Where("task_id = ? AND status = ?", tid, "executed").
		Order("stock_code, signal_date ASC").
		Find(&signals)

	// Group by stock
	stockMap := make(map[string]*StockAnalysis)
	for _, s := range signals {
		sa, ok := stockMap[s.StockCode]
		if !ok {
			sa = &StockAnalysis{
				StockCode: s.StockCode,
				StockName: s.StockName,
			}
			stockMap[s.StockCode] = sa
		}
		trade := StockTrade{
			SignalDate: s.SignalDate,
			ExecDate:   s.ExecDate,
			ActionType: s.ActionType,
			ExecPrice:  s.ExecPrice,
			ExecQty:    s.ExecQty,
			ExecAmount: s.ExecAmount,
			Pnl:        s.Pnl,
			PnlPct:     s.PnlPct,
			Reason:     s.Reason,
		}
		sa.Trades = append(sa.Trades, trade)

		switch s.ActionType {
		case "buy", "add":
			sa.BuyCount++
		case "sell", "reduce", "stop":
			sa.SellCount++
			sa.TotalPnl += s.Pnl
		}
	}

	// Calculate total PnlPct based on initial capital
	initialCapital := task.InitialCapital
	if initialCapital <= 0 {
		initialCapital = 100000
	}
	for _, sa := range stockMap {
		if sa.TotalPnl != 0 {
			sa.TotalPnlPct = sa.TotalPnl / initialCapital * 100
		}
	}

	// Convert to sorted slice (by abs totalPnl desc)
	result := make([]StockAnalysis, 0, len(stockMap))
	for _, sa := range stockMap {
		result = append(result, *sa)
	}
	sort.Slice(result, func(i, j int) bool {
		return math.Abs(result[i].TotalPnl) > math.Abs(result[j].TotalPnl)
	})

	response.Success(c, map[string]interface{}{
		"stocks": result,
		"total":  len(result),
	})
}
// BacktestTaskLogs returns execution logs for a task.
// Supports incremental polling: ?afterSeq=N returns only logs with seq > N.
func (h *StrategyHandler) BacktestTaskLogs(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	afterSeq := c.Query("afterSeq")
	q := db.MySQL.Where("task_id = ?", tid).Order("date ASC, seq ASC")
	if afterSeq != "" {
		q = q.Where("id > ?", afterSeq)
	}

	var logs []model.BacktestExecutionLog
	q.Find(&logs)

	// Return the max log id as cursor for next poll
	maxID := uint(0)
	if len(logs) > 0 {
		maxID = logs[len(logs)-1].ID
	}

	response.Success(c, map[string]interface{}{
		"logs":   logs,
		"cursor": maxID,
		"total":  len(logs),
	})
}

// BacktestTaskSnapshots returns daily snapshots for a task.
// Supports ?limit=N for latest N snapshots (default all).
func (h *StrategyHandler) BacktestTaskSnapshots(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	limitStr := c.DefaultQuery("limit", "0")
	limit, _ := strconv.Atoi(limitStr)

	q := db.MySQL.Where("task_id = ?", tid).Order("date ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}

	var snapshots []model.BacktestDailySnapshot
	q.Find(&snapshots)

	// Parse positions JSON for each snapshot
	type SnapshotOut struct {
		model.BacktestDailySnapshot
		PositionsData []map[string]interface{} `json:"positionsData"`
	}

	out := make([]SnapshotOut, len(snapshots))
	for i, s := range snapshots {
		out[i] = SnapshotOut{BacktestDailySnapshot: s}
		if s.Positions != "" {
			var pd []map[string]interface{}
			if json.Unmarshal([]byte(s.Positions), &pd) == nil {
				out[i].PositionsData = pd
			}
		}
	}

	response.Success(c, out)
}

// ═══════════════════════════════════════════════════════════════
// K-line cache — preloads close prices to avoid N+1 DB queries
// ═══════════════════════════════════════════════════════════════

type KlineCache struct {
	dates    []string
	dateIdx  map[string]int
	closeMap map[string][]float64 // code -> []close per date (forward-filled)
	openMap  map[string][]float64 // code -> []open per date (forward-filled)
}

// preloadKline loads all close prices for the given stock codes within date range.
// Forward-fills gaps (suspended stocks keep previous close).
func preloadKline(codes []string, startDate, endDate string) *KlineCache {
	kc := &KlineCache{
		dateIdx:  make(map[string]int),
		closeMap: make(map[string][]float64, len(codes)),
		openMap:  make(map[string][]float64, len(codes)),
	}

	// 1. Get all trading days
	if err := db.PG.Raw(`SELECT DISTINCT TO_CHAR(trade_date, 'YYYY-MM-DD') as d FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ? ORDER BY d`,
		startDate, endDate).Scan(&kc.dates).Error; err != nil {
		log.Printf("[backtest] preloadKline dates query failed: %v", err)
		return kc
	}

	for i, d := range kc.dates {
		kc.dateIdx[d] = i
	}

	if len(kc.dates) == 0 {
		return kc
	}

	// 2. Bulk load close + open prices in ONE query
	type KCRow struct {
		Code  string
		Date  string
		Close float64
		Open  float64
	}
	var rows []KCRow
	err := db.PG.Table("stocks_daily_k").
		Select("code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, close, open").
		Where("code IN ?", codes).
		Where("trade_date >= ?", startDate).
		Where("trade_date <= ?", endDate).
		Order("code, trade_date").
		Scan(&rows).Error
	if err != nil {
		log.Printf("[backtest] preloadKline close query failed: %v", err)
		return kc
	}

	// 3. Initialize arrays with zeros
	nDays := len(kc.dates)
	for _, c := range codes {
		kc.closeMap[c] = make([]float64, nDays)
		kc.openMap[c] = make([]float64, nDays)
	}

	// 4. Fill prices
	for _, r := range rows {
		if idx, ok := kc.dateIdx[r.Date]; ok {
			kc.closeMap[r.Code][idx] = r.Close
			kc.openMap[r.Code][idx] = r.Open
		}
	}

	// 5. Forward-fill close prices (gaps → previous close)
	for _, c := range codes {
		arr := kc.closeMap[c]
		var last float64
		for i := 0; i < nDays; i++ {
			if arr[i] > 0 {
				last = arr[i]
			} else {
				arr[i] = last
			}
		}
		// Forward-fill open prices too
		arrO := kc.openMap[c]
		var lastO float64
		for i := 0; i < nDays; i++ {
			if arrO[i] > 0 {
				lastO = arrO[i]
			} else {
				arrO[i] = lastO
			}
		}
	}

	return kc
}

// GetClose returns the close price for a stock on a given date (O(1) lookup).
func (kc *KlineCache) GetClose(code, date string) float64 {
	arr, ok := kc.closeMap[code]
	if !ok {
		return 0
	}
	idx, ok := kc.dateIdx[date]
	if !ok {
		return 0
	}
	return arr[idx]
}

// GetOpen returns the open price for a stock on a given date (O(1) lookup).
// GetDailyChange returns the daily change % for a stock on a given date.
func (kc *KlineCache) GetDailyChange(code, date string) float64 {
	cur := kc.GetClose(code, date)
	prev := getPrevClose(kc, code, date)
	if prev > 0 { return (cur - prev) / prev * 100 }
	return 0
}

func (kc *KlineCache) GetOpen(code, date string) float64 {
	arr, ok := kc.openMap[code]
	if !ok {
		return 0
	}
	idx, ok := kc.dateIdx[date]
	if !ok {
		return 0
	}
	return arr[idx]
}

// GetNextOpen returns the open price on the next trading day after the given date.
// Returns 0 if date is the last in the cache (no next day).
func (kc *KlineCache) GetNextOpen(code, date string) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx+1 >= len(kc.dates) {
		return 0
	}
	nextDate := kc.dates[idx+1]
	return kc.GetOpen(code, nextDate)
}



// getNextDate returns the next trading day after the given date.
// Returns empty string if date is the last in cache.
func getNextDate(kc *KlineCache, date string) string {
	idx, ok := kc.dateIdx[date]
	if !ok || idx+1 >= len(kc.dates) {
		return ""
	}
	return kc.dates[idx+1]
}

// tradingDaysBetween returns the number of trading days between date1 and date2.
// Returns -1 if either date is not in cache.
func (kc *KlineCache) tradingDaysBetween(date1, date2 string) int {
	idx1, ok1 := kc.dateIdx[date1]
	idx2, ok2 := kc.dateIdx[date2]
	if !ok1 || !ok2 {
		return -1
	}
	diff := idx2 - idx1
	if diff < 0 {
		return -diff
	}
	return diff
}

// GetNextClose returns the close price on the next trading day after the given date.
func (kc *KlineCache) GetNextClose(code, date string) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx+1 >= len(kc.dates) {
		return 0
	}
	nextDate := kc.dates[idx+1]
	return kc.GetClose(code, nextDate)
}

// checkOp evaluates a comparison between a float value and threshold.
func checkOp(val float64, op string, threshold float64) bool {
	switch op {
	case "gte": return val >= threshold
	case "lte": return val <= threshold
	case "gt": return val > threshold
	case "lt": return val < threshold
	case "eq": return val == threshold
	case "cross_up": return val > 0
	case "cross_down": return val < 0
	}
	return false
}

// getPrevClose returns the close price one trading day before the given date.
func getPrevClose(kc *KlineCache, code, date string) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx == 0 {
		return 0
	}
	arr, ok := kc.closeMap[code]
	if !ok {
		return 0
	}
	return arr[idx-1]
}

// getCloseNDaysAgo returns the close price N trading days before the given date.
func getCloseNDaysAgo(kc *KlineCache, code, date string, n int) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx < n {
		return 0
	}
	arr, ok := kc.closeMap[code]
	if !ok {
		return 0
	}
	return arr[idx-n]
}

// ── Indicator batch preloader ──


// IndicatorValue holds a preloaded indicator value for a stock on a date.
type IndicatorValue struct {
	Code  string
	Date  string
	Value float64
}

// IndicatorCache stores preloaded indicator values: map[indicatorName]map[code|date]value
type IndicatorCache struct {
	data              map[string]map[string]float64 // key: indicator, inner key: "code|date"
	hasIndicatorData  map[string]map[string]bool    // indicator -> code -> has any data
}

func newIndicatorCache() *IndicatorCache {
	return &IndicatorCache{
		data:             make(map[string]map[string]float64),
		hasIndicatorData: make(map[string]map[string]bool),
	}
}

func (ic *IndicatorCache) set(indicator, code, date string, val float64) {
	key := indicator
	if _, ok := ic.data[key]; !ok {
		ic.data[key] = make(map[string]float64)
	}
	ic.data[key][code+"|"+date] = val
}

func (ic *IndicatorCache) get(indicator, code, date string) (float64, bool) {
	m, ok := ic.data[indicator]
	if !ok {
		return 0, false
	}
	v, ok := m[code+"|"+date]
	return v, ok
}

// batchScan runs a query and populates the cache for a given indicator.
func (ic *IndicatorCache) batchScan(indicator string, query string, args ...interface{}) {
	var rows []IndicatorValue
	if err := db.PG.Raw(query, args...).Scan(&rows).Error; err != nil {
		log.Printf("[backtest] batch scan %s failed: %v", indicator, err)
		return
	}
	for _, r := range rows {
		ic.set(indicator, r.Code, r.Date, r.Value)
		ic.markHasData(indicator, r.Code)
	}
}

// markHasData records that a stock has this indicator type of data (at least one date).
func (ic *IndicatorCache) markHasData(indicator, code string) {
	if _, ok := ic.hasIndicatorData[indicator]; !ok {
		ic.hasIndicatorData[indicator] = make(map[string]bool)
	}
	ic.hasIndicatorData[indicator][code] = true
}

// HasData returns true if the stock has any records for this indicator.
func (ic *IndicatorCache) HasData(indicator, code string) bool {
	m, ok := ic.hasIndicatorData[indicator]
	if !ok {
		return false
	}
	return m[code]
}

// batchScanWithCodes builds an IN clause from codes and injects it via fmt.Sprintf at %%s.
func (ic *IndicatorCache) batchScanWithCodes(indicator string, codes []string, queryFmt string, args ...interface{}) {
	inClause := db.CodesToInClause(codes)
	query := fmt.Sprintf(queryFmt, inClause)
	ic.batchScan(indicator, query, args...)
}

// preloadIndicators batch-loads all indicator values needed by the strategy for the given universe.
func preloadIndicators(conds []model.StrategyCondition, codes []string, startDate, endDate string, kcache *KlineCache) *IndicatorCache {
	cache := newIndicatorCache()

	// Collect unique indicator names to preload (use Registry for safety check)
	needPreload := make(map[string]bool)
	for _, c := range conds {
		ind := c.Indicator
		// Skip non-backtest-safe indicators
		if !IsBacktestSafe(ind) {
			continue
		}
		// Skip indicators we can compute from close prices in Go
		switch ind {
		case "daily_change", "momentum_5", "momentum_20":
			continue // computed from kcache.GetClose
		}
		// Skip indicators that require complex per-stock computation (loaded separately below)
		switch {
		case ind == "pe_percentile", ind == "pb_percentile":
			continue
		case ind == "shareholder_change", ind == "inst_hold_ratio":
			continue
		// Note: AI scores, financial metrics, algo_score, signal_value are batch-preloaded below
		}
		needPreload[ind] = true
	}

	// Batch preload streak counts (cumulative, code-level, not date-dependent)
	// Preload volume data for volume-related indicators
	needVolume := false
	for ind := range needPreload {
		if strings.HasPrefix(ind, "volume") || ind == "turnover_rate" || ind == "mfi" {
			needVolume = true
			break
		}
	}
	_ = needVolume // we'll handle volume inline

	// Batch preload: DMI/ADX (most expensive, preload all three at once)
	if needPreload["dmi_plus"] || needPreload["dmi_minus"] || needPreload["adx"] {
		log.Printf("[backtest] batch preloading DMI/ADX for %d stocks...", len(codes))
		cache.batchScanWithCodes("dmi_plus", codes,
			`WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k
				WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			), tr_calc AS (
				SELECT code, date,
					GREATEST(high - LAG(high) OVER (PARTITION BY code ORDER BY date), 0) as up_move,
					GREATEST(LAG(low) OVER (PARTITION BY code ORDER BY date) - low, 0) as down_move,
					GREATEST(high-low,
						ABS(high-prev_close),
						ABS(low-prev_close)
					) as tr
				FROM klines
			), dmi14 AS (
				SELECT code, date,
					AVG(CASE WHEN up_move > 0 AND up_move > down_move THEN up_move ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_up,
					AVG(CASE WHEN down_move > 0 AND down_move > up_move THEN down_move ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_down,
					AVG(tr) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_tr
				FROM tr_calc
			), dmi AS (
				SELECT code, date,
					CASE WHEN avg_tr > 0 THEN avg_up/avg_tr*100 ELSE 0 END as pdi,
					CASE WHEN avg_tr > 0 THEN avg_down/avg_tr*100 ELSE 0 END as mdi
				FROM dmi14
			), adx_calc AS (
				SELECT code, date, pdi, mdi,
					CASE WHEN pdi+mdi > 0 THEN ABS(pdi-mdi)/(pdi+mdi)*100 ELSE 0 END as dx
				FROM dmi
			)
			SELECT code, date, 
				AVG(dx) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as value
			FROM adx_calc`,
			startDate, endDate)
		delete(needPreload, "adx")

		cache.batchScanWithCodes("dmi_minus", codes,
			`WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			), tr_calc AS (
				SELECT code, date,
					GREATEST(high - LAG(high) OVER (PARTITION BY code ORDER BY date), 0) as up_move,
					GREATEST(LAG(low) OVER (PARTITION BY code ORDER BY date) - low, 0) as down_move,
					GREATEST(high-low, ABS(high-prev_close), ABS(low-prev_close)) as tr
				FROM klines
			), dmi14 AS (
				SELECT code, date,
					AVG(CASE WHEN up_move > 0 AND up_move > down_move THEN up_move ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_up,
					AVG(CASE WHEN down_move > 0 AND down_move > up_move THEN down_move ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_down,
					AVG(tr) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_tr
				FROM tr_calc
			)
			SELECT code, date,
				CASE WHEN avg_tr > 0 THEN avg_down/avg_tr*100 ELSE 0 END as value
			FROM dmi14`,
			startDate, endDate)
		delete(needPreload, "dmi_minus")

		cache.batchScanWithCodes("dmi_plus", codes,
			`WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			), tr_calc AS (
				SELECT code, date,
					GREATEST(high - LAG(high) OVER (PARTITION BY code ORDER BY date), 0) as up_move,
					GREATEST(LAG(low) OVER (PARTITION BY code ORDER BY date) - low, 0) as down_move,
					GREATEST(high-low, ABS(high-prev_close), ABS(low-prev_close)) as tr
				FROM klines
			), dmi14 AS (
				SELECT code, date,
					AVG(CASE WHEN up_move > 0 AND up_move > down_move THEN up_move ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_up,
					AVG(CASE WHEN down_move > 0 AND down_move > up_move THEN down_move ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_down,
					AVG(tr) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_tr
				FROM tr_calc
			)
			SELECT code, date,
				CASE WHEN avg_tr > 0 THEN avg_up/avg_tr*100 ELSE 0 END as value
			FROM dmi14`,
			startDate, endDate)
		delete(needPreload, "dmi_plus")
	}

	// Batch preload: RSI
	if needPreload["rsi"] {
		log.Printf("[backtest] batch preloading RSI for %d stocks...", len(codes))
		cache.batchScanWithCodes("rsi", codes,
			`WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, close,
					close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as chg
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			), gains AS (
				SELECT code, date,
					AVG(CASE WHEN chg > 0 THEN chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_gain,
					AVG(CASE WHEN chg < 0 THEN -chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_loss
				FROM klines
			)
			SELECT code, date,
				CASE WHEN avg_loss > 0 THEN 100 - 100/(1 + avg_gain/NULLIF(avg_loss,0)) ELSE 100 END as value
			FROM gains`,
			startDate, endDate)
		delete(needPreload, "rsi")
	}

	// Batch preload: MACD
	if needPreload["macd"] || needPreload["macd_dif"] || needPreload["macd_dea"] {
		log.Printf("[backtest] batch preloading MACD/DIF/DEA (real EMA) for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		type CloseRow struct {
			Code  string
			Date  string
			Close float64
		}
		var closes []CloseRow
		closeQuery := fmt.Sprintf(
			"SELECT code, TO_CHAR(trade_date,'YYYY-MM-DD') as date, close FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ? ORDER BY code, trade_date",
			inClause)
		if err := db.PG.Raw(closeQuery, startDate, endDate).Scan(&closes).Error; err != nil {
			log.Printf("[backtest] MACD close fetch failed: %v", err)
		} else {
			codeCloses := map[string][]struct{ date string; close float64 }{}
			for _, c := range closes {
				codeCloses[c.Code] = append(codeCloses[c.Code], struct{ date string; close float64 }{c.Date, c.Close})
			}
			const alpha12, alpha26, alphaDea = 0.1538, 0.0741, 0.2
			for code, rows := range codeCloses {
				if len(rows) < 12 { continue }
				var ema12, ema26, dea float64
				ema12, ema26 = rows[0].close, rows[0].close
				for i, r := range rows {
					if i == 0 {
						ema12, ema26 = r.close, r.close
					} else {
						ema12 = alpha12*r.close + (1-alpha12)*ema12
						ema26 = alpha26*r.close + (1-alpha26)*ema26
					}
					dif := ema12 - ema26
					if i == 0 { dea = dif } else { dea = alphaDea*dif + (1-alphaDea)*dea }
					hist := dif - dea
					if needPreload["macd"] { cache.set("macd", code, r.date, hist) }
					if needPreload["macd_dif"] { cache.set("macd_dif", code, r.date, dif) }
					if needPreload["macd_dea"] { cache.set("macd_dea", code, r.date, dea) }
				}
			}
		}
		delete(needPreload, "macd")
		delete(needPreload, "macd_dif")
		delete(needPreload, "macd_dea")
	}

	// Batch preload: KDJ (preload K, D, J)
	if needPreload["kdj_k"] || needPreload["kdj_d"] || needPreload["kdj_j"] {
		log.Printf("[backtest] batch preloading KDJ for %d stocks...", len(codes))
		inClauseKDJ := db.CodesToInClause(codes)
		queryKDJ := fmt.Sprintf(`
			WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, high, low, close FROM stocks_daily_k
				WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			), rsv AS (
				SELECT code, date,
					CASE WHEN MAX(high) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) -
						MIN(low) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) > 0
					THEN (close - MIN(low) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW)) /
						(MAX(high) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) -
						MIN(low) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW)) * 100
					ELSE 50 END as rsv_val
				FROM klines
			), k_calc AS (
				SELECT code, date,
					(2.0/3.0) * COALESCE(LAG(rsv_val) OVER (PARTITION BY code ORDER BY date), 50) + (1.0/3.0) * rsv_val as k_val
				FROM rsv
			), kdj AS (
				SELECT code, date, k_val,
					AVG(k_val) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) as d_val
				FROM k_calc
			)
			SELECT code, date, k_val, d_val, 3*k_val - 2*d_val as j_val
			FROM kdj
		`, inClauseKDJ)
		type KDJRow struct {
			Code string
			Date string
			K    float64
			D    float64
			J    float64
		}
		var kdjRows []KDJRow
		if err := db.PG.Raw(queryKDJ, startDate, endDate).Scan(&kdjRows).Error; err != nil {
			log.Printf("[backtest] KDJ preload failed: %v", err)
		} else {
			for _, r := range kdjRows {
				if needPreload["kdj_k"] { cache.set("kdj_k", r.Code, r.Date, r.K) }
				if needPreload["kdj_d"] { cache.set("kdj_d", r.Code, r.Date, r.D) }
				if needPreload["kdj_j"] { cache.set("kdj_j", r.Code, r.Date, r.J) }
			}
		}
		delete(needPreload, "kdj_k")
		delete(needPreload, "kdj_d")
		delete(needPreload, "kdj_j")
	}

	// Batch preload: simple volume-related from stocks_daily_k
	if needPreload["volume_ratio"] || needPreload["volume_ma_ratio"] {
		log.Printf("[backtest] batch preloading volume data for %d stocks...", len(codes))
		if needPreload["volume_ratio"] {
			cache.batchScanWithCodes("volume_ratio", codes,
				`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, 
					COALESCE(volume / NULLIF(AVG(volume) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 4 PRECEDING AND 1 PRECEDING), 0), 0) as value
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?`,
				startDate, endDate)
			delete(needPreload, "volume_ratio")
		}
		if needPreload["volume_ma_ratio"] {
			cache.batchScanWithCodes("volume_ma_ratio", codes,
				`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, 
					COALESCE(volume / NULLIF(AVG(volume) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND 1 PRECEDING), 0), 0) as value
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?`,
				startDate, endDate)
			delete(needPreload, "volume_ma_ratio")
		}
	}

	// Batch preload: turnover_rate if available
	if needPreload["turnover_rate"] {
		log.Printf("[backtest] batch preloading turnover_rate for %d stocks...", len(codes))
		cache.batchScanWithCodes("turnover_rate", codes,
			`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, COALESCE(turnover_rate, 0) as value
			FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?`,
			startDate, endDate)
		delete(needPreload, "turnover_rate")
	}

	// Batch preload: ATR
	if needPreload["atr"] || needPreload["atr_pct"] {
		log.Printf("[backtest] batch preloading ATR for %d stocks...", len(codes))
		cache.batchScanWithCodes("atr", codes,
			`WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			)
			SELECT code, date,
				AVG(GREATEST(high-low, ABS(high-prev_close), ABS(low-prev_close)))
					OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as value
			FROM klines`,
			startDate, endDate)
		delete(needPreload, "atr")
	}

	// Batch preload: CCI
	if needPreload["cci"] {
		log.Printf("[backtest] batch preloading CCI for %d stocks...", len(codes))
		cache.batchScanWithCodes("cci", codes,
			`WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, high, low, close FROM stocks_daily_k
				WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			), tp AS (
				SELECT code, date, (high+low+close)/3 as typical FROM klines
			)
			SELECT code, date,
				(typical - AVG(typical) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW)) /
				NULLIF(0.015 * AVG(ABS(typical - AVG(typical) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW)))
					OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW), 0) as value
			FROM tp`,
			startDate, endDate)
		delete(needPreload, "cci")
	}

	// Batch preload: Bollinger Bands (upper/middle/lower)
	if needPreload["boll_upper"] || needPreload["boll_middle"] || needPreload["boll_lower"] {
		log.Printf("[backtest] batch preloading Bollinger Bands for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		query := fmt.Sprintf(`
			WITH bb AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, close,
					AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
					STDDEV_SAMP(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as stddev
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			)
			SELECT code, date,
				mid + 2 * COALESCE(stddev, 0) as upper,
				mid as middle,
				mid - 2 * COALESCE(stddev, 0) as lower
			FROM bb
		`, inClause)
		type BBRow struct {
			Code   string
			Date   string
			Upper  float64
			Middle float64
			Lower  float64
		}
		var rows []BBRow
		if err := db.PG.Raw(query, startDate, endDate).Scan(&rows).Error; err != nil {
			log.Printf("[backtest] BB preload failed: %v", err)
		} else {
			for _, r := range rows {
				if needPreload["boll_upper"]  { cache.set("boll_upper",  r.Code, r.Date, r.Upper)  }
				if needPreload["boll_middle"] { cache.set("boll_middle", r.Code, r.Date, r.Middle) }
				if needPreload["boll_lower"]  { cache.set("boll_lower",  r.Code, r.Date, r.Lower)  }
			}
		}
		delete(needPreload, "boll_upper")
		delete(needPreload, "boll_middle")
		delete(needPreload, "boll_lower")
	}

	// Batch preload: PSY/PSYMA psychological line
	if needPreload["psy_12"] || needPreload["psy_ma"] {
		log.Printf("[backtest] batch preloading PSY/PSYMA for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		query := fmt.Sprintf(`
			WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			),
			psy_calc AS (
				SELECT code, date,
					SUM(CASE WHEN close > prev_close THEN 1 ELSE 0 END) 
						OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 11 PRECEDING AND CURRENT ROW) * 100.0 / 12 as psy
				FROM klines
			)
			SELECT code, date, psy,
				AVG(psy) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 5 PRECEDING AND CURRENT ROW) as psyma
			FROM psy_calc
		`, inClause)
		type PSYRow struct {
			Code  string
			Date  string
			PSY   float64
			PSYMA float64
		}
		var rows []PSYRow
		if err := db.PG.Raw(query, startDate, endDate).Scan(&rows).Error; err != nil {
			log.Printf("[backtest] PSY preload failed: %v", err)
		} else {
			for _, r := range rows {
				if needPreload["psy_12"] { cache.set("psy_12", r.Code, r.Date, r.PSY) }
				if needPreload["psy_ma"] { cache.set("psy_ma", r.Code, r.Date, r.PSYMA) }
			}
		}
		delete(needPreload, "psy_12")
		delete(needPreload, "psy_ma")
	}

	// Batch preload: RSI multi-period (rsi_6/12/24)
	if needPreload["rsi_6"] || needPreload["rsi_12"] || needPreload["rsi_24"] {
		log.Printf("[backtest] batch preloading multi-period RSI for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		query := fmt.Sprintf(`
			WITH klines AS (
				SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, close,
					close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as chg
				FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
			)
			SELECT code, date,
				CASE WHEN avg_loss_6 > 0 THEN 100 - 100/(1 + avg_gain_6/NULLIF(avg_loss_6,0)) ELSE 100 END as rsi6,
				CASE WHEN avg_loss_12 > 0 THEN 100 - 100/(1 + avg_gain_12/NULLIF(avg_loss_12,0)) ELSE 100 END as rsi12,
				CASE WHEN avg_loss_24 > 0 THEN 100 - 100/(1 + avg_gain_24/NULLIF(avg_loss_24,0)) ELSE 100 END as rsi24
			FROM (
				SELECT code, date,
					AVG(CASE WHEN chg > 0 THEN chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 5 PRECEDING AND CURRENT ROW) as avg_gain_6,
					AVG(CASE WHEN chg < 0 THEN -chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 5 PRECEDING AND CURRENT ROW) as avg_loss_6,
					AVG(CASE WHEN chg > 0 THEN chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 11 PRECEDING AND CURRENT ROW) as avg_gain_12,
					AVG(CASE WHEN chg < 0 THEN -chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 11 PRECEDING AND CURRENT ROW) as avg_loss_12,
					AVG(CASE WHEN chg > 0 THEN chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 23 PRECEDING AND CURRENT ROW) as avg_gain_24,
					AVG(CASE WHEN chg < 0 THEN -chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 23 PRECEDING AND CURRENT ROW) as avg_loss_24
				FROM klines
			) gains
		`, inClause)
		type RSIRow struct {
			Code  string
			Date  string
			RSI6  float64
			RSI12 float64
			RSI24 float64
		}
		var rows []RSIRow
		if err := db.PG.Raw(query, startDate, endDate).Scan(&rows).Error; err != nil {
			log.Printf("[backtest] multi-RSI preload failed: %v", err)
		} else {
			for _, r := range rows {
				if needPreload["rsi_6"]  { cache.set("rsi_6",  r.Code, r.Date, r.RSI6)  }
				if needPreload["rsi_12"] { cache.set("rsi_12", r.Code, r.Date, r.RSI12) }
				if needPreload["rsi_24"] { cache.set("rsi_24", r.Code, r.Date, r.RSI24) }
			}
		}
		delete(needPreload, "rsi_6")
		delete(needPreload, "rsi_12")
		delete(needPreload, "rsi_24")
	}

	// Batch preload: MA lines (ma_5/10/20/30/60)
	if needPreload["ma_5"] || needPreload["ma_10"] || needPreload["ma_20"] || needPreload["ma_30"] || needPreload["ma_60"] {
		log.Printf("[backtest] batch preloading MA lines for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		query := fmt.Sprintf(`
			SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) as ma5,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 9 PRECEDING AND CURRENT ROW) as ma10,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 29 PRECEDING AND CURRENT ROW) as ma30,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) as ma60
			FROM stocks_daily_k WHERE code IN (%s) AND trade_date BETWEEN ? AND ?
		`, inClause)
		type MARow struct {
			Code string
			Date string
			MA5  float64
			MA10 float64
			MA20 float64
			MA30 float64
			MA60 float64
		}
		var rows []MARow
		if err := db.PG.Raw(query, startDate, endDate).Scan(&rows).Error; err != nil {
			log.Printf("[backtest] MA preload failed: %v", err)
		} else {
			for _, r := range rows {
				if needPreload["ma_5"]  { cache.set("ma_5",  r.Code, r.Date, r.MA5)  }
				if needPreload["ma_10"] { cache.set("ma_10", r.Code, r.Date, r.MA10) }
				if needPreload["ma_20"] { cache.set("ma_20", r.Code, r.Date, r.MA20) }
				if needPreload["ma_30"] { cache.set("ma_30", r.Code, r.Date, r.MA30) }
				if needPreload["ma_60"] { cache.set("ma_60", r.Code, r.Date, r.MA60) }
			}
		}
		delete(needPreload, "ma_5")
		delete(needPreload, "ma_10")
		delete(needPreload, "ma_20")
		delete(needPreload, "ma_30")
		delete(needPreload, "ma_60")
	}

	if len(needPreload) > 0 {
		keys := make([]string, 0, len(needPreload))
		for k := range needPreload {
			keys = append(keys, k)
		}
		log.Printf("[backtest] unbatched indicators (fallback to per-stock): %v", keys)
	}

	// Batch preload: AI scores (7 dimensions from ai_stock_scores)
	// Take latest score per stock up to endDate; these are weekly-refresh data so exact date match not needed
	if needPreload["ai_score"] || needPreload["ai_fundamental"] || needPreload["ai_technical"] || needPreload["ai_valuation"] || needPreload["ai_growth"] || needPreload["ai_industry"] || needPreload["ai_capital"] {
		log.Printf("[backtest] batch preloading AI scores for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		query := fmt.Sprintf(`
			SELECT code,
				COALESCE(composite_score, 0) as ai_score,
				COALESCE(fundamental_score, 0) as ai_fundamental,
				COALESCE(technical_score, 0) as ai_technical,
				COALESCE(valuation_score, 0) as ai_valuation,
				COALESCE(growth_score, 0) as ai_growth,
				COALESCE(industry_score, 0) as ai_industry,
				COALESCE(capital_score, 0) as ai_capital
			FROM ai_stock_scores
			WHERE code IN (%s) AND created_at <= ?::date
		`, inClause)
		type AIRow struct {
			Code           string
			AiScore        float64
			AiFundamental  float64
			AiTechnical    float64
			AiValuation    float64
			AiGrowth       float64
			AiIndustry     float64
			AiCapital      float64
		}
		var rows []AIRow
		if err := db.PG.Raw(query, endDate).Scan(&rows).Error; err != nil {
			log.Printf("[backtest] AI scores preload failed: %v", err)
		} else {
			// AI scores are date-independent (one row per stock), store with empty date
			for _, r := range rows {
				if needPreload["ai_score"]       { cache.set("ai_score",       r.Code, "", r.AiScore) }
				if needPreload["ai_fundamental"]  { cache.set("ai_fundamental",  r.Code, "", r.AiFundamental) }
				if needPreload["ai_technical"]    { cache.set("ai_technical",    r.Code, "", r.AiTechnical) }
				if needPreload["ai_valuation"]    { cache.set("ai_valuation",    r.Code, "", r.AiValuation) }
				if needPreload["ai_growth"]       { cache.set("ai_growth",       r.Code, "", r.AiGrowth) }
				if needPreload["ai_industry"]     { cache.set("ai_industry",     r.Code, "", r.AiIndustry) }
				if needPreload["ai_capital"]      { cache.set("ai_capital",      r.Code, "", r.AiCapital) }
			}
		}
		delete(needPreload, "ai_score")
		delete(needPreload, "ai_fundamental")
		delete(needPreload, "ai_technical")
		delete(needPreload, "ai_valuation")
		delete(needPreload, "ai_growth")
		delete(needPreload, "ai_industry")
		delete(needPreload, "ai_capital")
	}

	// Batch preload: Financial metrics from stock_financials (latest report before endDate)
	if needPreload["roe"] || needPreload["revenue_growth"] || needPreload["profit_growth"] || needPreload["gross_margin"] || needPreload["net_margin"] || needPreload["debt_ratio"] || needPreload["eps"] {
		log.Printf("[backtest] batch preloading financials for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		query := fmt.Sprintf(`
			SELECT DISTINCT ON (code) code,
				COALESCE(roe, 0) as roe,
				COALESCE(revenue_growth, 0) as revenue_growth,
				COALESCE(profit_growth, 0) as profit_growth,
				COALESCE(gross_margin, 0) as gross_margin,
				COALESCE(net_margin, 0) as net_margin,
				COALESCE(debt_ratio, 0) as debt_ratio,
				COALESCE(eps, 0) as eps
			FROM stock_financials
			WHERE code IN (%s) AND report_date <= ?
			ORDER BY code, report_date DESC
		`, inClause)
		type FinRow struct {
			Code          string
			ROE           float64
			RevenueGrowth float64
			ProfitGrowth  float64
			GrossMargin   float64
			NetMargin     float64
			DebtRatio     float64
			EPS           float64
		}
		var rows []FinRow
		if err := db.PG.Raw(query, endDate).Scan(&rows).Error; err != nil {
			log.Printf("[backtest] financials preload failed: %v", err)
		} else {
			for _, r := range rows {
				if needPreload["roe"]            { cache.set("roe",            r.Code, "", r.ROE) }
				if needPreload["revenue_growth"] { cache.set("revenue_growth", r.Code, "", r.RevenueGrowth) }
				if needPreload["profit_growth"]  { cache.set("profit_growth",  r.Code, "", r.ProfitGrowth) }
				if needPreload["gross_margin"]   { cache.set("gross_margin",   r.Code, "", r.GrossMargin) }
				if needPreload["net_margin"]     { cache.set("net_margin",     r.Code, "", r.NetMargin) }
				if needPreload["debt_ratio"]     { cache.set("debt_ratio",     r.Code, "", r.DebtRatio) }
				if needPreload["eps"]            { cache.set("eps",            r.Code, "", r.EPS) }
			}
		}
		delete(needPreload, "roe")
		delete(needPreload, "revenue_growth")
		delete(needPreload, "profit_growth")
		delete(needPreload, "gross_margin")
		delete(needPreload, "net_margin")
		delete(needPreload, "debt_ratio")
		delete(needPreload, "eps")
	}

	// Batch preload: algo_score from algorithm_pick_details
	if needPreload["algo_score"] {
		log.Printf("[backtest] batch preloading algo_score for %d stocks...", len(codes))
		cache.batchScanWithCodes("algo_score", codes,
			`SELECT stock_code as code, TO_CHAR(pick_date, 'YYYY-MM-DD') as date, COALESCE(score, 0) as value
			FROM algorithm_pick_details WHERE stock_code IN (%s) AND pick_date BETWEEN ? AND ?`,
			startDate, endDate)
		delete(needPreload, "algo_score")
	}

	// Batch preload: signal_value from stock_signals (one row per stock, date-independent)
	if needPreload["signal_value"] {
		log.Printf("[backtest] batch preloading signal_value for %d stocks...", len(codes))
		inClause := db.CodesToInClause(codes)
		query := fmt.Sprintf(`SELECT code, COALESCE(signal_value, 0) as value FROM stock_signals WHERE code IN (%s)`, inClause)
		cache.batchScan("signal_value", query)
		delete(needPreload, "signal_value")
	}

	// Batch preload: PE/PB/PS/market_cap from stocks_daily_indicator
	if needPreload["pe"] || needPreload["pb"] || needPreload["ps"] || needPreload["total_market_cap"] {
		log.Printf("[backtest] batch preloading PE/PB/PS/市值 for %d stocks...", len(codes))
		if needPreload["pe"] {
			cache.batchScanWithCodes("pe", codes,
				`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, pe as value
				FROM stocks_daily_indicator WHERE code IN (%s) AND trade_date BETWEEN ? AND ? AND pe > 0`,
				startDate, endDate)
			delete(needPreload, "pe")
		}
		if needPreload["pb"] {
			cache.batchScanWithCodes("pb", codes,
				`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, pb as value
				FROM stocks_daily_indicator WHERE code IN (%s) AND trade_date BETWEEN ? AND ? AND pb > 0`,
				startDate, endDate)
			delete(needPreload, "pb")
		}
		if needPreload["ps"] {
			cache.batchScanWithCodes("ps", codes,
				`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, ps as value
				FROM stocks_daily_indicator WHERE code IN (%s) AND trade_date BETWEEN ? AND ? AND ps > 0`,
				startDate, endDate)
			delete(needPreload, "ps")
		}
		if needPreload["total_market_cap"] {
			cache.batchScanWithCodes("total_market_cap", codes,
				`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, COALESCE(total_market_cap, 0) as value
				FROM stocks_daily_indicator WHERE code IN (%s) AND trade_date BETWEEN ? AND ?`,
				startDate, endDate)
			delete(needPreload, "total_market_cap")
		}
	}

	return cache
}


// ═══════════════════════════════════════════════════════════════
// ConceptRankCache — 概念板块强度排名缓存
// ═══════════════════════════════════════════════════════════════

type ConceptRankCache struct {
    // date -> concept_name -> rank percentile (0.0=worst, 1.0=best)
    dateRanks map[string]map[string]float64
    // code -> concept_names (preloaded stock→concept mapping)
    codeConcepts map[string][]string
}

// GetMultiplier returns a score multiplier based on the stock's concept rankings.
// Top 20% concepts: 1.3x, 20-50%: 1.0x, 50-80%: 0.7x, bottom 20%: 0.4x
func (crc *ConceptRankCache) GetMultiplier(code, date string) float64 {
    if crc == nil {
        return 1.0
    }
    concepts, ok := crc.codeConcepts[code]
    if !ok || len(concepts) == 0 {
        return 1.0 // no concept data, neutral
    }
    
    dateRanks, ok := crc.dateRanks[date]
    if !ok {
        return 1.0
    }
    
    // Average rank percentile across all concepts this stock belongs to
    var sumRank float64
    var count int
    for _, c := range concepts {
        if rank, ok := dateRanks[c]; ok {
            sumRank += rank
            count++
        }
    }
    if count == 0 {
        return 1.0
    }
    
    avgRank := sumRank / float64(count)
    
    // Map percentile to multiplier
    switch {
    case avgRank >= 0.80:
        return 1.30 // top 20% concepts
    case avgRank >= 0.50:
        return 1.00 // middle
    case avgRank >= 0.20:
        return 0.70 // bottom 50-80%
    default:
        return 0.40 // bottom 20% concepts
    }
}

// preloadStreakCounts batch-loads streak_count for all codes over the date range.
// Replaces per-stock N+1 queries in getStreakCount.
func preloadStreakCounts(codes []string, startDate, endDate string) map[string]map[string]float64 {
	result := make(map[string]map[string]float64)
	for _, c := range codes {
		result[c] = make(map[string]float64)
	}

	type streakRow struct {
		Code  string
		Date  string
		Count float64
	}
	var rows []streakRow
	// Batch query: for each (code, date), count consecutive streak
	err := db.PG.Raw(`
		WITH stock_dates AS (
			SELECT DISTINCT apd.stock_code AS code, apd.pick_date AS date
			FROM algorithm_pick_details apd
			JOIN algorithm_picks ap ON ap.pick_date = apd.pick_date
			WHERE apd.stock_code = ANY(?) AND apd.pick_date >= ?::date AND apd.pick_date <= ?::date
		),
		ranked AS (
			SELECT code, date,
				date - (ROW_NUMBER() OVER (PARTITION BY code ORDER BY date DESC))::int AS grp
			FROM stock_dates
		),
		streaks AS (
			SELECT code, date,
				COUNT(*) OVER (PARTITION BY code, grp ORDER BY date DESC) AS streak
			FROM ranked
		)
		SELECT code, TO_CHAR(date, 'YYYY-MM-DD') as date, streak::float as count FROM streaks
	`, codes, startDate, endDate).Scan(&rows).Error

	if err != nil {
		log.Printf("[streak_count] batch preload failed: %v", err)
		return result
	}

	for _, r := range rows {
		result[r.Code][r.Date] = r.Count
	}
	log.Printf("[streak_count] preloaded %d rows for %d codes", len(rows), len(codes))
	return result
}

// preloadPickCounts batch-loads pick counts for all codes over the date range.
func preloadPickCounts(codes []string, startDate, endDate string) map[string]map[int]float64 {
	result := make(map[string]map[int]float64)
	if len(codes) == 0 {
		return result
	}
	for _, c := range codes {
		result[c] = make(map[int]float64)
	}

	type pickRow struct {
		StockCode string
		PickDate  string
	}
	var rows []pickRow
	if err := db.PG.Raw(`
		SELECT apd.stock_code, apd.pick_date::text
		FROM algorithm_pick_details apd
		JOIN algorithm_picks ap ON ap.pick_date = apd.pick_date
		WHERE apd.stock_code = ANY(?) AND apd.pick_date >= ?::date AND apd.pick_date <= ?::date
		ORDER BY apd.stock_code, apd.pick_date DESC
	`, codes, startDate, endDate).Scan(&rows).Error; err != nil {
		log.Printf("[pick_count] batch preload failed: %v", err)
		return result
	}

	// Count appearances in 5d and 20d windows for each stock
	for _, r := range rows {
		if result[r.StockCode] == nil {
			result[r.StockCode] = make(map[int]float64)
		}
		result[r.StockCode][5]++
		result[r.StockCode][20]++
	}
	log.Printf("[pick_count] preloaded %d pick-dates for %d codes", len(rows), len(codes))
	return result
}

// preloadConceptRanks precomputes concept daily performance rankings.
func preloadConceptRanks(codes []string, startDate, endDate string) *ConceptRankCache {
    crc := &ConceptRankCache{
        dateRanks:    make(map[string]map[string]float64),
        codeConcepts: make(map[string][]string),
    }
    
    // 1. Preload stock→concept mappings
    type conceptRow struct {
        Code        string
        ConceptName string
    }
    var mappings []conceptRow
    inClause1 := db.CodesToInClause(codes)
    query1 := fmt.Sprintf(`SELECT code, concept_name FROM stock_concepts 
        WHERE concept_type = 'concept' AND code IN (%s)`, inClause1)
    if err := db.PG.Raw(query1).Scan(&mappings).Error; err != nil {
        log.Printf("[concept_cache] stock→concept preload failed: %v", err)
        return crc
    }
    for _, m := range mappings {
        crc.codeConcepts[m.Code] = append(crc.codeConcepts[m.Code], m.ConceptName)
    }
    log.Printf("[concept_cache] loaded %d stock→concept mappings for %d stocks", len(mappings), len(crc.codeConcepts))
    
    // 2. Compute per-date per-concept average daily_change
    type conceptPerf struct {
        TradeDate   string
        ConceptName string
        AvgChg      float64
    }
    var perfs []conceptPerf
    inClause2 := db.CodesToInClause(codes)
    query2 := fmt.Sprintf(`
        WITH daily_chg AS (
            SELECT code, trade_date,
                   (close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date)) 
                   / NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date), 0) * 100 as chg
            FROM stocks_daily_k
            WHERE trade_date >= ? AND trade_date <= ?
              AND code IN (%s)
        )
        SELECT TO_CHAR(dc.trade_date, 'YYYY-MM-DD') as trade_date,
               sc.concept_name,
               AVG(dc.chg) as avg_chg
        FROM daily_chg dc
        JOIN stock_concepts sc ON sc.code = dc.code AND sc.concept_type = 'concept'
        WHERE dc.chg IS NOT NULL AND dc.chg > -50 AND dc.chg < 50
        GROUP BY dc.trade_date, sc.concept_name
        HAVING COUNT(*) >= 3
        ORDER BY dc.trade_date
    `, inClause2)
    if err := db.PG.Raw(query2, startDate, endDate).Scan(&perfs).Error; err != nil {
        log.Printf("[concept_cache] concept performance query failed: %v", err)
        return crc
    }
    log.Printf("[concept_cache] loaded %d concept-day performance records", len(perfs))
    
    // 3. Group by date and compute rank percentiles
    datePerfs := make(map[string][]float64)
    dateConcepts := make(map[string][]string)
    for _, p := range perfs {
        datePerfs[p.TradeDate] = append(datePerfs[p.TradeDate], p.AvgChg)
        dateConcepts[p.TradeDate] = append(dateConcepts[p.TradeDate], p.ConceptName)
    }
    
    for date, perfs_ := range datePerfs {
        concepts := dateConcepts[date]
        n := len(perfs_)
        if n < 3 {
            continue
        }
        // Create sorted copy for ranking
        sorted := make([]float64, n)
        copy(sorted, perfs_)
        sort.Float64s(sorted)
        
        crc.dateRanks[date] = make(map[string]float64)
        for i, p := range perfs_ {
            // Find rank percentile: position in sorted / total
            rank := float64(sort.SearchFloat64s(sorted, p)) / float64(n-1)
            crc.dateRanks[date][concepts[i]] = rank
        }
    }
    
    log.Printf("[concept_cache] computed ranks for %d dates", len(crc.dateRanks))
    return crc
}
// ═══════════════════════════════════════════════════════════════
// MarketStyleEngine — 市场风格识别引擎 (20日滚动多因子)
// ═══════════════════════════════════════════════════════════════

type MarketStyle string

const (
    StyleBullRally    MarketStyle = "bull_rally"    // 🟢 牛市普涨
    StyleMildBull     MarketStyle = "mild_bull"     // 🟢 温和上涨
    StyleRecovery     MarketStyle = "recovery"      // 🟡 回暖修复
    StyleStructural   MarketStyle = "structural"    // 🟠 结构分化
    StyleRotation     MarketStyle = "rotation"      // 🟡 震荡轮动
    StyleBottoming    MarketStyle = "bottoming"     // 🟤 底部磨底
    StyleBear         MarketStyle = "bear"          // 🔴 熊市下跌
    StyleCrash        MarketStyle = "crash"         // ⚫ 恐慌暴跌
    StyleTransitional MarketStyle = "transitional"  // ⬜ 过渡
)

// StyleParams holds the strategy parameter adjustments for a market style.
type StyleParams struct {
    BuyPct          float64 // 单票仓位%
    AddPct          float64 // 加仓仓位%
    BuyLogic        string  // "and" or "or"
    AllowAdd        bool
    AllowBuy        bool
    SellPctMult     float64 // 卖出加速倍数
    ConceptTopPct   float64 // 概念池范围(0-1), 0=全部
    PositionBias    float64 // 仓位乘数
    StopProfitAdj   float64 // 止盈调整(加法)
    StopLossAdj     float64 // 止损调整(加法，负值=更紧)
    TrailingStopDrawdown float64 // 移动止盈回撤%(0=默认)
}

// defaultStyleParams returns hardcoded optimal parameters per market style.
func defaultStyleParams(style MarketStyle) StyleParams {
    switch style {
    case StyleBullRally, StyleMildBull:
        return StyleParams{
            BuyPct: 20, AddPct: 15, BuyLogic: "or",
            AllowBuy: true, AllowAdd: true,
            ConceptTopPct: 0.50, PositionBias: 1.2,
            StopProfitAdj: 5, StopLossAdj: -2,
            TrailingStopDrawdown: 10, // 牛市允许更大回撤
        }
    case StyleRecovery:
        return StyleParams{
            BuyPct: 15, AddPct: 10, BuyLogic: "and",
            AllowBuy: true, AllowAdd: true,
            ConceptTopPct: 0.40, PositionBias: 1.0,
            StopProfitAdj: 0, StopLossAdj: 0,
            TrailingStopDrawdown: 8,
        }
    case StyleStructural:
        return StyleParams{
            BuyPct: 12, AddPct: 10, BuyLogic: "and",
            AllowBuy: true, AllowAdd: true,
            ConceptTopPct: 0.20, PositionBias: 1.0,
            StopProfitAdj: 0, StopLossAdj: 0,
            TrailingStopDrawdown: 8,
        }
    case StyleRotation:
        return StyleParams{
            BuyPct: 6, AddPct: 0, BuyLogic: "and",
            AllowBuy: true, AllowAdd: false,
            ConceptTopPct: 0.30, PositionBias: 0.6,
            StopProfitAdj: -5, StopLossAdj: 2, // tighter stops
            TrailingStopDrawdown: 5, // 轮动收紧回撤
        }
    case StyleBottoming:
        return StyleParams{
            BuyPct: 4, AddPct: 0, BuyLogic: "and",
            AllowBuy: true, AllowAdd: false,
            ConceptTopPct: 0.15, PositionBias: 0.4,
            StopProfitAdj: -5, StopLossAdj: 3,
            TrailingStopDrawdown: 4, // 磨底收紧回撤
        }
    case StyleBear, StyleCrash:
        return StyleParams{
            BuyPct: 0, AddPct: 0, BuyLogic: "and",
            AllowBuy: false, AllowAdd: false,
            ConceptTopPct: 0, PositionBias: 0,
            StopProfitAdj: 0, StopLossAdj: 0,
            SellPctMult: 2.0,
            TrailingStopDrawdown: 0,
        }
    default: // transitional
        return StyleParams{
            BuyPct: 10, AddPct: 5, BuyLogic: "and",
            AllowBuy: true, AllowAdd: false,
            ConceptTopPct: 0.50, PositionBias: 0.8,
            StopProfitAdj: 0, StopLossAdj: 0,
            TrailingStopDrawdown: 0,
        }
    }
}

func styleName(style MarketStyle) string {
    names := map[MarketStyle]string{
        StyleBullRally: "🟢 牛市普涨", StyleMildBull: "🟢 温和上涨",
        StyleRecovery: "🟡 回暖修复", StyleStructural: "🟠 结构分化",
        StyleRotation: "🟡 震荡轮动", StyleBottoming: "🟤 底部磨底",
        StyleBear: "🔴 熊市下跌", StyleCrash: "⚫ 恐慌暴跌",
        StyleTransitional: "⬜ 过渡整理",
    }
    return names[style]
}

// MarketStyleEngine detects the current market regime using 20-day rolling statistics.
type MarketStyleEngine struct {
    cache map[string]MarketStyle
}

func NewMarketStyleEngine() *MarketStyleEngine {
    return &MarketStyleEngine{cache: make(map[string]MarketStyle)}
}

// DetectStyle classifies the market regime for a given date using multi-factor rolling analysis.
func (mse *MarketStyleEngine) DetectStyle(date string) MarketStyle {
    if s, ok := mse.cache[date]; ok {
        return s
    }

    // Query rolling 20-day stats from market_sentiment
    var row struct {
        AvgScore    float64
        AvgUpRatio  float64
        AvgDiff     float64
        AvgVol      float64
        ScoreTrend  float64 // 10-day slope of composite_score
    }
    err := db.PG.Raw(`
        WITH rolling AS (
            SELECT trade_date, composite_score,
                   up_count::float / NULLIF(total_stocks,0) as up_ratio,
                   sector_diffusion, volatility,
                   ROW_NUMBER() OVER (ORDER BY trade_date) as rn
            FROM market_sentiment
            WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '30 days')
        ),
        stats AS (
            SELECT 
                AVG(composite_score) as avg_score,
                AVG(up_ratio) as avg_up_ratio,
                AVG(sector_diffusion) as avg_diff,
                AVG(volatility) as avg_vol
            FROM rolling
            WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '20 days')
        ),
        trend AS (
            SELECT REGR_SLOPE(composite_score, rn::float) as slope
            FROM rolling
            WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '10 days')
        )
        SELECT s.*, COALESCE(t.slope, 0) as score_trend FROM stats s, trend t
    `, date, date, date, date, date, date).Scan(&row).Error

    if err != nil {
        log.Printf("[market_style] query failed for %s: %v", date, err)
        return StyleTransitional
    }

    // Multi-factor classification
    s20 := row.AvgScore
    u20 := row.AvgUpRatio
    d20 := row.AvgDiff
    v20 := row.AvgVol
    trend := row.ScoreTrend

    var style MarketStyle

    // ⚫ 恐慌: 极低分 + 高波动
    if s20 < 18 || (s20 < 25 && v20 > 0.18) {
        style = StyleCrash
    } else if trend < 0 && u20 < 0.30 && s20 < 32 {
        style = StyleBear // 🔴 熊市
    } else if s20 < 30 && u20 < 0.35 && trend >= -0.5 && trend <= 0.5 {
        style = StyleBottoming // 🟤 磨底
    } else if trend > 0.5 && u20 < 0.48 {
        style = StyleRecovery // 🟡 回暖
    } else if u20 > 0.48 && d20 > 0.45 && trend > 0.3 {
        style = StyleBullRally // 🟢 普涨
    } else if u20 > 0.45 && trend > 0.2 {
        style = StyleMildBull // 🟢 温和
    } else if u20 < 0.35 && d20 < 0.30 {
        style = StyleStructural // 🟠 结构
    } else if 0.30 <= u20 && u20 < 0.50 && d20 >= 0.30 {
        style = StyleRotation // 🟡 轮动
    } else {
        style = StyleTransitional
    }

    mse.cache[date] = style
    return style
}

// GetStyleParams returns the trading parameters for the detected style.
func (mse *MarketStyleEngine) GetStyleParams(date string) StyleParams {
    return defaultStyleParams(mse.DetectStyle(date))
}
// ── The async backtest runner (runs in goroutine) ──

func (h *StrategyHandler) runBacktestAsync(ctx context.Context, task *model.BacktestTask, s *model.Strategy, startDate, endDate string, stockCodes []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[backtest] PANIC in task %d: %v", task.ID, r)
			db.MySQL.Model(task).Updates(map[string]interface{}{
				"status": "failed",
				"phase":  "回测异常",
				"error_msg": fmt.Sprintf("panic: %v", r),
			})
		}
	}()
	// Transaction cost constants (A-share market)
	const (
		STAMP_TAX_RATE  = 0.0005  // 卖出印花税 0.05%
		COMMISSION_RATE = 0.00025 // 券商佣金 万分之2.5
		MIN_COMMISSION  = 5.0     // 最低佣金 5元
	)

	// Mark as running
	now := time.Now()
	db.MySQL.Model(task).Updates(map[string]interface{}{
		"status":     "running",
		"phase":      "正在初始化...",
		"started_at": now,
	})

	updateProgress := func(day, total int, phase string, positions string) {
		pct := 0.0
		if total > 0 {
			pct = float64(day) / float64(total) * 100
		}
		updates := map[string]interface{}{
			"current_day": day,
			"progress_pct": pct,
		}
		if phase != "" {
			updates["phase"] = phase
		}
		if positions != "" {
			updates["current_positions"] = positions
		}
		db.MySQL.Model(task).Updates(updates)
	}

	// Load conditions
	var conds []model.StrategyCondition
	db.MySQL.Where("strategy_id = ?", task.StrategyID).Find(&conds)
	buyConds := filterConds(conds, "buy")
	addConds := filterConds(conds, "add")
	sellConds := filterConds(conds, "sell")
	reduceConds := filterConds(conds, "reduce")

	buyPct := s.BuyPositionPct
	if buyPct <= 0 { buyPct = 15 }
	addPct := s.AddPositionPct
	if addPct <= 0 { addPct = 10 }
	reducePct := s.ReducePositionPct
	if reducePct <= 0 { reducePct = 50 }

	capital := s.InitialCapital
	if capital <= 0 { capital = 100000 }
	remainingCash := capital
	maxHold := s.MaxHoldings

	// Record initial capital on task
	db.MySQL.Model(task).Update("initial_capital", capital)
	if maxHold <= 0 { maxHold = 20 }
	// Default to V2 (hybrid) for all strategies unless explicitly set to legacy
	useV2 := s.OrchestrationMode != "legacy"

	updateProgress(0, task.TotalDays, fmt.Sprintf("初始化: 资金¥%.0f | 最大持股%d只", capital, maxHold), "")

	// Determine stock universe
	type StockInfo struct {
		Code string
		Name string
	}
	var universe []StockInfo
	if len(stockCodes) > 0 {
		// P0-2: Filter ST stocks even in explicit stock code list
		if err := db.PG.Table("stocks_basic").
			Select("code, COALESCE(name,'') as name").
			Where("code IN ?", stockCodes).
			Where("is_st IS NULL OR is_st = false").
			Scan(&universe).Error; err != nil {
			log.Printf("[backtest] universe query (stockCodes) failed: %v", err)
		}
	} else {
		// Stock pool "all" — sample up to 3000 stocks for performance
		// P0-2: Filter out ST/suspended stocks at DB level
		err := db.PG.Table("stocks_daily_k k").
			Select("k.code, COALESCE(s.name, k.code) as name").
			Joins("LEFT JOIN stocks_basic s ON s.code = k.code").
			Where("k.trade_date >= ?", startDate).
			Where("k.trade_date <= ?", endDate).
			Where("s.is_st IS NULL OR s.is_st = false").
			Group("k.code, s.name").
			Order("code ASC").Limit(3000).  // deterministic for reproducibility
			Scan(&universe).Error
		if err != nil {
			log.Printf("[backtest] universe query (all) failed: %v", err)
		} else {
			log.Printf("[backtest] universe query (all): %d stocks found for %s~%s", len(universe), startDate, endDate)
		}
	}

	if len(universe) == 0 {
		db.MySQL.Model(task).Updates(map[string]interface{}{
			"status":  "failed",
			"phase":   "无可用股票数据",
			"error_msg": "所选时间段无可用股票数据",
		})
		return
	}

	// Get all stock codes for preload
	universeCodes := make([]string, len(universe))
	for i, s := range universe {
		universeCodes[i] = s.Code
	}

	// P1: Extend kline preloading for indicator lookback
	extStart, _ := time.Parse("2006-01-02", startDate)
	extEnd, _ := time.Parse("2006-01-02", endDate)
	preloadFromStart := extStart.AddDate(0, 0, -100)
	preloadFromEnd := extEnd.AddDate(0, 0, -80)
	preloadStart := preloadFromEnd.Format("2006-01-02")
	if preloadFromStart.Before(preloadFromEnd) {
		preloadStart = preloadFromStart.Format("2006-01-02")
	}
	kcache := preloadKline(universeCodes, preloadStart, endDate)
	allDates := kcache.dates
	// Debug: verify preload
	loadedCount := 0
	if len(allDates) > 0 {
		for _, c := range universeCodes {
			if kcache.GetClose(c, allDates[0]) > 0 || kcache.GetClose(c, allDates[len(allDates)-1]) > 0 {
				loadedCount++
			}
		}
	}
	log.Printf("[backtest] kcache preload: %d codes, %d dates, %d have data on first/last day", len(universeCodes), len(allDates), loadedCount)

	// Preload indicator values in batch (one query per indicator instead of N+1 per stock)
	icache := preloadIndicators(conds, universeCodes, preloadStart, endDate, kcache)

	// Preload concept strength rankings for scoring multiplier
	conceptCache := preloadConceptRanks(universeCodes, startDate, endDate)

	// Create market style engine (cached across days)
	styleEngine := service.NewMarketStyleService()

	// Local evaluateSingleCondition that uses the preloaded cache
	evalSingle := func(cond model.StrategyCondition, code, date string) bool {
		ind := cond.Indicator
		// Try cache first with exact date
		if val, ok := icache.get(ind, code, date); ok {
			return checkOp(val, cond.Operator, cond.Value)
		}
		// AI scores and financials are stored with empty date (date-independent)
		if val, ok := icache.get(ind, code, ""); ok {
			return checkOp(val, cond.Operator, cond.Value)
		}
		// PE/PB/PS/市值 are preloaded but sparse — try previous dates
		if ind == "pe" || ind == "pb" || ind == "ps" || ind == "total_market_cap" {
			dates := kcache.dates
			idx := -1
			for i, d := range dates {
				if d == date { idx = i; break }
			}
			for i := idx; i >= 0; i-- {
				if val, ok := icache.get(ind, code, dates[i]); ok {
					return checkOp(val, cond.Operator, cond.Value)
				}
			}
		}
		// Compute from close cache for simple momentum indicators
		switch ind {
		case "daily_change":
			cur := kcache.GetClose(code, date)
			prev := getPrevClose(kcache, code, date)
			if prev > 0 {
				return checkOp((cur-prev)/prev*100, cond.Operator, cond.Value)
			}
			return false
		case "momentum_5":
			cur := kcache.GetClose(code, date)
			prev := getCloseNDaysAgo(kcache, code, date, 5)
			if prev > 0 {
				return checkOp((cur-prev)/prev*100, cond.Operator, cond.Value)
			}
			return false
		case "momentum_20":
			cur := kcache.GetClose(code, date)
			prev := getCloseNDaysAgo(kcache, code, date, 20)
			if prev > 0 {
				return checkOp((cur-prev)/prev*100, cond.Operator, cond.Value)
			}
			return false
		// PE/PB/PS indicators: check data availability before falling back
		// Missing data should NOT silently pass the condition
		case "pe", "pb", "ps", "total_market_cap":
			if !icache.HasData(ind, code) {
				return false // No indicator data for this stock
			}
		case "pe_percentile":
			if !icache.HasData("pe", code) {
				return false // No PE data → cannot compute percentile
			}
		case "pb_percentile":
			if !icache.HasData("pb", code) {
				return false // No PB data → cannot compute percentile
			}
		}
		// Fallback to original per-stock SQL query (handles forward-fill)
		return evaluateSingleCondition(cond, code, date)
	}

	// Local evaluateConditions that uses the cached evalSingle
	evalConds := func(conds_ []model.StrategyCondition, code, date string) bool {
		if len(conds_) == 0 { return false }
		groups := make(map[int][]model.StrategyCondition)
		for _, c := range conds_ {
			groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
		}
		for _, groupConds := range groups {
			allMet := true
			for _, c := range groupConds {
				if !evalSingle(c, code, date) {
					allMet = false
					break
				}
			}
			if allMet { return true }
		}
		return false
	}
	// evalSingleWithDetail evaluates a single condition and returns (passed, detail_string).
	evalSingleWithDetail := func(cond model.StrategyCondition, code, date string) (bool, string) {
		ind := cond.Indicator
		var val float64
		if v, ok := icache.get(ind, code, date); ok {
			val = v
		} else if v, ok := icache.get(ind, code, ""); ok {
			// AI scores and financials stored with empty date
			val = v
		} else if ind == "pe" || ind == "pb" || ind == "ps" || ind == "total_market_cap" {
			// PE/PB sparse data — try previous dates from kcache
			dates := kcache.dates
			idx := -1
			for i, d := range dates {
				if d == date { idx = i; break }
			}
			for i := idx; i >= 0; i-- {
				if v, ok := icache.get(ind, code, dates[i]); ok {
					val = v; break
				}
			}
		} else {
			switch ind {
			case "daily_change":
				cur := kcache.GetClose(code, date)
				prev := getPrevClose(kcache, code, date)
				if prev > 0 { val = (cur - prev) / prev * 100 }
			case "momentum_5":
				cur := kcache.GetClose(code, date)
				prev := getCloseNDaysAgo(kcache, code, date, 5)
				if prev > 0 { val = (cur - prev) / prev * 100 }
			case "momentum_20":
				cur := kcache.GetClose(code, date)
				prev := getCloseNDaysAgo(kcache, code, date, 20)
				if prev > 0 { val = (cur - prev) / prev * 100 }
			default:
				val = getIndicatorValue(cond, code, date)
			}
		}
		passed := checkOp(val, cond.Operator, cond.Value)
		detail := fmt.Sprintf("%s=%.2f %s %.2f", ind, val, cond.Operator, cond.Value)
		return passed, detail
	}

	// evalCondsWithDetail evaluates conditions grouped by LogicGroup and returns
	// (passed, detail_string) where detail explains which group matched.
	evalCondsWithDetail := func(conds_ []model.StrategyCondition, code, date string) (bool, string) {
		if len(conds_) == 0 { return false, "无条件" }
		groups := make(map[int][]model.StrategyCondition)
		for _, c := range conds_ {
			groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
		}
		for gid, groupConds := range groups {
			allMet := true
			details := make([]string, 0, len(groupConds))
			for _, c := range groupConds {
				ok, d := evalSingleWithDetail(c, code, date)
				details = append(details, d)
				if !ok {
					allMet = false
					break
				}
			}
			if allMet {
				return true, fmt.Sprintf("组%d通过: %s", gid, strings.Join(details, "; "))
			}
		}
		return false, "无满足的条件组"
	}

	_ = evalCondsWithDetail

	if len(allDates) == 0 {
		db.MySQL.Model(task).Updates(map[string]interface{}{
			"status":  "failed",
			"phase":   "无交易日数据",
			"error_msg": "所选时间段无交易日数据",
		})
		return
	}

	// P1: Only evaluate dates within [startDate, endDate], not all cached dates
	evalDates := make([]string, 0, len(allDates))
	for _, d := range allDates {
		if d >= startDate && d <= endDate {
			evalDates = append(evalDates, d)
		}
	}
	allDates = evalDates
	totalDays := len(allDates)
	db.MySQL.Model(task).Update("total_days", totalDays)
	updateProgress(0, totalDays, fmt.Sprintf("回测区间: %s ~ %s | 共 %d 个交易日", startDate, endDate, totalDays), "")

	// Regular investment schedule
	regDates := make(map[string]bool)
	if s.InvestmentType == "regular" && s.RegularAmount > 0 && s.RegularInterval != "" {
		lastReg := ""
		for _, d := range allDates {
			add := false
			switch s.RegularInterval {
			case "daily": add = true
			case "weekly": add = lastReg == "" || d > lastReg
			case "monthly": add = lastReg == "" || d[:7] != lastReg[:7]
			}
			if add { regDates[d] = true; lastReg = d }
		}
	}


	// ────────────────────────────────────────────────────────────
	// Signal Generation & Execution helpers
	// ────────────────────────────────────────────────────────────
	var positions map[string]*dcPosition

	// generateSignals evaluates conditions and creates pending BacktestSignal records.
	// Called at T day close — signals will be executed at T+1 day open.
	generateSignals := func(date string, cash float64, isLastDay bool) []model.BacktestSignal {
		var signals []model.BacktestSignal

		// --- Last day: skip all buy/add signals; forced liquidation handles sells ---
		// Stop-loss signals are also skipped because all positions will be force-sold below.
		if isLastDay {
			return signals
		}

		// --- Stop-profit / Stop-loss checks ---
		stopCodeSet := make(map[string]bool)
		for _, pos := range positions {
			closePrice := kcache.GetClose(pos.Code, date)
			if closePrice <= 0 {
				continue
			}
			chgPct := (closePrice - pos.BuyPrice) / pos.BuyPrice * 100

			// Stop-loss: T+1 check — cannot sell if bought today
			if pos.BuyDate == date {
				continue
			}

			if s.StopLoss < 0 && chgPct <= s.StopLoss {
				signals = append(signals, model.BacktestSignal{
					TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
					SignalDate: date, ExecDate: getNextDate(kcache, date),
					StockCode: pos.Code, StockName: pos.Name,
					ActionType: "stop",
					PlannedPrice: closePrice, PlannedQty: pos.Quantity,
					PlannedAmount: closePrice * float64(pos.Quantity),
					Status: "pending",
					Reason: fmt.Sprintf("止损触发 %.1f%% ≤ %.1f%%", chgPct, s.StopLoss),
				})
				stopCodeSet[pos.Code] = true
				continue
			}

			if s.StopProfit > 0 && chgPct >= s.StopProfit {
				signals = append(signals, model.BacktestSignal{
					TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
					SignalDate: date, ExecDate: getNextDate(kcache, date),
					StockCode: pos.Code, StockName: pos.Name,
					ActionType: "stop",
					PlannedPrice: closePrice, PlannedQty: pos.Quantity,
					PlannedAmount: closePrice * float64(pos.Quantity),
					Status: "pending",
					Reason: fmt.Sprintf("止盈触发 %.1f%% ≥ %.1f%%", chgPct, s.StopProfit),
				})
				stopCodeSet[pos.Code] = true
			}
		}

		// --- Sell/Reduce checks ---
		for _, pos := range positions {
			// Skip if already has a stop signal today
			if stopCodeSet[pos.Code] {
				continue
			}
			// T+1: cannot sell if bought today
			if pos.BuyDate == date {
				continue
			}

			if ok, detail := evalCondsWithDetail(sellConds, pos.Code, date); ok {
				reason := "满足卖出条件"
				if detail != "" { reason = reason + " | " + detail }
				signals = append(signals, model.BacktestSignal{
					TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
					SignalDate: date, ExecDate: getNextDate(kcache, date),
					StockCode: pos.Code, StockName: pos.Name,
					ActionType: "sell",
					PlannedPrice: kcache.GetClose(pos.Code, date),
					PlannedQty: pos.Quantity,
					PlannedAmount: kcache.GetClose(pos.Code, date) * float64(pos.Quantity),
					Status: "pending",
					Reason: reason,
				})
			} else if ok, detail := evalCondsWithDetail(reduceConds, pos.Code, date); ok {
				// Cooldown guard: skip reduce if already reduced within cooldown
				if pos.LastReduceDate != "" && kcache.tradingDaysBetween(pos.LastReduceDate, date) <= REDUCE_COOLDOWN_DAYS {
					// skip — within cooldown
				} else {
					reduceQty := int(float64(pos.Quantity) * reducePct / 100)
					if reduceQty > 0 && pos.Quantity-reduceQty < MIN_REDUCE_QTY && pos.Quantity-reduceQty > 0 {
						// Convert to full sell to avoid fragmentation
						reduceQty = pos.Quantity
						signals = append(signals, model.BacktestSignal{
							TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
							SignalDate: date, ExecDate: getNextDate(kcache, date),
							StockCode: pos.Code, StockName: pos.Name,
							ActionType: "sell",
							PlannedPrice: kcache.GetClose(pos.Code, date),
							PlannedQty: reduceQty,
							PlannedAmount: kcache.GetClose(pos.Code, date) * float64(reduceQty),
							Status: "pending",
							Reason: fmt.Sprintf("满足减仓条件 | %s (%.0f%%, 碎片化转清仓)", detail, reducePct),
						})
					} else if reduceQty >= MIN_REDUCE_QTY {
						signals = append(signals, model.BacktestSignal{
							TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
							SignalDate: date, ExecDate: getNextDate(kcache, date),
							StockCode: pos.Code, StockName: pos.Name,
							ActionType: "reduce",
							PlannedPrice: kcache.GetClose(pos.Code, date),
							PlannedQty: reduceQty,
							PlannedAmount: kcache.GetClose(pos.Code, date) * float64(reduceQty),
							Status: "pending",
							Reason: fmt.Sprintf("满足减仓条件 | %s (%.0f%%)", detail, reducePct),
						})
					}
				}
			}
		}

		// --- Buy checks ---
		slotCount := maxHold - len(positions)
		if slotCount > 0 && cash > 0 {
			buyAmountPerStock := cash * buyPct / 100
			boughtThisRound := 0

			for _, si := range universe {
				if boughtThisRound >= slotCount {
					break
				}
				if _, held := positions[si.Code]; held {
					continue
				}
				ok, detail := evalCondsWithDetail(buyConds, si.Code, date)
				if !ok {
					continue
				}

				closePrice := kcache.GetClose(si.Code, date)
				if closePrice <= 0 {
					continue
				}
				plannedQty := int(buyAmountPerStock / closePrice / 100) * 100
				if plannedQty < 100 && buyAmountPerStock >= closePrice*100 {
					plannedQty = 100
				}
				if plannedQty <= 0 {
					continue
				}

				signals = append(signals, model.BacktestSignal{
					TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
					SignalDate: date, ExecDate: getNextDate(kcache, date),
					StockCode: si.Code, StockName: si.Name,
					ActionType: "buy",
					PlannedPrice: closePrice, PlannedQty: plannedQty,
					PlannedAmount: closePrice * float64(plannedQty),
					Status: "pending",
					Reason: fmt.Sprintf("满足买入条件 | %s", detail),
				})
				boughtThisRound++
			}
		}

		// --- Add checks ---
		for _, pos := range positions {
			if ok, detail := evalCondsWithDetail(addConds, pos.Code, date); ok {
				closePrice := kcache.GetClose(pos.Code, date)
				if closePrice <= 0 {
					continue
				}
				addAmount := cash * addPct / 100
				addQty := int(addAmount / closePrice / 100) * 100
				if addQty <= 0 {
					continue
				}

				signals = append(signals, model.BacktestSignal{
					TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
					SignalDate: date, ExecDate: getNextDate(kcache, date),
					StockCode: pos.Code, StockName: pos.Name,
					ActionType: "add",
					PlannedPrice: closePrice, PlannedQty: addQty,
					PlannedAmount: closePrice * float64(addQty),
					Status: "pending",
					Reason: fmt.Sprintf("满足加仓条件 | %s", detail),
				})
			}
		}

		return signals
	}

	// executeSignal executes a pending signal at the given open price.
	// Updates positions, cash, and marks the signal as executed/skipped.
	executeSignal := func(sig *model.BacktestSignal, openPrice float64, cash *float64) *backtestTrade {
		if openPrice <= 0 {
			sig.Status = "skipped"
			sig.SkipReason = "停牌或无开盘价"
			return nil
		}

		switch sig.ActionType {
		case "buy":
			// T+1 re-calc: actual qty based on open price
			actualQty := int(sig.PlannedAmount / openPrice / 100) * 100
			if actualQty <= 0 {
				sig.Status = "skipped"
				sig.SkipReason = "开盘价过高，可买数量不足1手"
				return nil
			}
			actualAmount := openPrice * float64(actualQty)
			if actualAmount > *cash {
				actualQty = int(*cash / openPrice / 100) * 100
				actualAmount = openPrice * float64(actualQty)
			}
			if actualQty <= 0 {
				sig.Status = "skipped"
				sig.SkipReason = "资金不足"
				return nil
			}

			*cash -= actualAmount
			// Deduct buy commission (no stamp tax on buy)
			buyCommission := math.Max(actualAmount*COMMISSION_RATE, MIN_COMMISSION)
			*cash -= buyCommission
			positions[sig.StockCode] = &dcPosition{
				Code: sig.StockCode, Name: sig.StockName,
				BuyPrice: openPrice, Quantity: actualQty, BuyDate: sig.ExecDate,
			}
			sig.ExecPrice = openPrice
			sig.ExecQty = actualQty
			sig.ExecAmount = actualAmount
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: "buy", Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: actualQty,
				Reason: sig.Reason,
			}

		case "add":
			actualQty := int(sig.PlannedAmount / openPrice / 100) * 100
			if actualQty <= 0 {
				sig.Status = "skipped"
				sig.SkipReason = "开盘价过高，加仓数量不足"
				return nil
			}
			actualAmount := openPrice * float64(actualQty)
			if actualAmount > *cash {
				actualQty = int(*cash / openPrice / 100) * 100
				actualAmount = openPrice * float64(actualQty)
			}
			if actualQty <= 0 {
				sig.Status = "skipped"
				sig.SkipReason = "资金不足"
				return nil
			}
			*cash -= actualAmount
			// Deduct buy commission (no stamp tax on buy)
			addCommission := math.Max(actualAmount*COMMISSION_RATE, MIN_COMMISSION)
			*cash -= addCommission
			if pos, ok := positions[sig.StockCode]; ok {
				totalCost := pos.BuyPrice*float64(pos.Quantity) + actualAmount
				pos.Quantity += actualQty
				if pos.Quantity > 0 {
					pos.BuyPrice = totalCost / float64(pos.Quantity)
				}
			}
			sig.ExecPrice = openPrice
			sig.ExecQty = actualQty
			sig.ExecAmount = actualAmount
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: "add", Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: actualQty,
				Reason: sig.Reason,
			}

		case "sell", "stop":
			pos, ok := positions[sig.StockCode]
			if !ok {
				sig.Status = "skipped"
				sig.SkipReason = "无持仓"
				return nil
			}
			// T+1 check: bought today, cannot sell
			if pos.BuyDate == sig.ExecDate {
				sig.Status = "skipped"
				sig.SkipReason = "T+1限制：当日买入不可卖出"
				return nil
			}
			sellQty := pos.Quantity
			pnl := (openPrice - pos.BuyPrice) * float64(sellQty)
			pnlPct := 0.0
			if pos.BuyPrice > 0 {
				pnlPct = (openPrice - pos.BuyPrice) / pos.BuyPrice * 100
			}
			*cash += openPrice * float64(sellQty)
			// Deduct transaction costs
			sellAmount := openPrice * float64(sellQty)
			commission := math.Max(sellAmount*COMMISSION_RATE, MIN_COMMISSION)
			stampTax := sellAmount * STAMP_TAX_RATE
			*cash -= commission + stampTax
			delete(positions, sig.StockCode)

			sig.ExecPrice = openPrice
			sig.ExecQty = sellQty
			sig.ExecAmount = openPrice * float64(sellQty)
			sig.Pnl = math.Round(pnl*100) / 100
			sig.PnlPct = math.Round(pnlPct*100) / 100
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: sig.ActionType, Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: sellQty,
				Reason: sig.Reason, Pnl: sig.Pnl, PnlPct: sig.PnlPct,
			}

		case "reduce":
			pos, ok := positions[sig.StockCode]
			if !ok {
				sig.Status = "skipped"
				sig.SkipReason = "无持仓"
				return nil
			}
			if pos.BuyDate == sig.ExecDate {
				sig.Status = "skipped"
				sig.SkipReason = "T+1限制：当日买入不可减持"
				return nil
			}
			reduceQty := sig.PlannedQty
			if reduceQty >= pos.Quantity {
				reduceQty = pos.Quantity
			}
			pnl := (openPrice - pos.BuyPrice) * float64(reduceQty)
			pnlPct := 0.0
			if pos.BuyPrice > 0 {
				pnlPct = (openPrice - pos.BuyPrice) / pos.BuyPrice * 100
			}
			*cash += openPrice * float64(reduceQty)
			// Deduct transaction costs
			reduceAmount := openPrice * float64(reduceQty)
			commission := math.Max(reduceAmount*COMMISSION_RATE, MIN_COMMISSION)
			stampTax := reduceAmount * STAMP_TAX_RATE
			*cash -= commission + stampTax
			pos.Quantity -= reduceQty
			pos.LastReduceDate = sig.ExecDate // mark cooldown start
			if pos.Quantity <= 0 {
				delete(positions, sig.StockCode)
			}

			sig.ExecPrice = openPrice
			sig.ExecQty = reduceQty
			sig.ExecAmount = openPrice * float64(reduceQty)
			sig.Pnl = math.Round(pnl*100) / 100
			sig.PnlPct = math.Round(pnlPct*100) / 100
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: "reduce", Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: reduceQty,
				Reason: sig.Reason, Pnl: sig.Pnl, PnlPct: sig.PnlPct,
			}

		case "dip_buy":
			actualQty := int(sig.PlannedAmount / openPrice / 100) * 100
			if actualQty <= 0 || actualQty < 100 {
				sig.Status = "skipped"
				sig.SkipReason = "抄底数量不足"
				return nil
			}
			actualAmount := openPrice * float64(actualQty)
			if actualAmount > *cash {
				actualQty = int(*cash / openPrice / 100) * 100
				actualAmount = openPrice * float64(actualQty)
			}
			if actualQty <= 0 {
				sig.Status = "skipped"
				sig.SkipReason = "资金不足"
				return nil
			}
			*cash -= actualAmount
			dipCommission := math.Max(actualAmount*COMMISSION_RATE, MIN_COMMISSION)
			*cash -= dipCommission
			if pos, ok := positions[sig.StockCode]; ok {
				pos.DipLot = &DipLot{Qty: actualQty, BuyPrice: openPrice, BuyDate: sig.ExecDate}
				pos.LastDipDate = sig.ExecDate
			}
			sig.ExecPrice = openPrice
			sig.ExecQty = actualQty
			sig.ExecAmount = actualAmount
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: "dip_buy", Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: actualQty,
				Reason: sig.Reason,
			}

		case "dip_sell":
			pos, ok := positions[sig.StockCode]
			if !ok || pos.DipLot == nil || pos.DipLot.Qty <= 0 {
				sig.Status = "skipped"
				sig.SkipReason = "无抄底持仓"
				return nil
			}
			if pos.DipLot.BuyDate == sig.ExecDate {
				sig.Status = "skipped"
				sig.SkipReason = "T+1限制: 当日抄底不可卖出"
				return nil
			}
			sellQty := pos.DipLot.Qty
			dipPnl := (openPrice - pos.DipLot.BuyPrice) * float64(sellQty)
			dipPnlPct := 0.0
			if pos.DipLot.BuyPrice > 0 {
				dipPnlPct = (openPrice - pos.DipLot.BuyPrice) / pos.DipLot.BuyPrice * 100
			}
			*cash += openPrice * float64(sellQty)
			dipSellAmt := openPrice * float64(sellQty)
			commission := math.Max(dipSellAmt*COMMISSION_RATE, MIN_COMMISSION)
			stampTax := dipSellAmt * STAMP_TAX_RATE
			*cash -= commission + stampTax
			pos.DipLot = nil
			sig.ExecPrice = openPrice
			sig.ExecQty = sellQty
			sig.ExecAmount = dipSellAmt
			sig.Pnl = math.Round(dipPnl*100) / 100
			sig.PnlPct = math.Round(dipPnlPct*100) / 100
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: "dip_sell", Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: sellQty,
				Reason: sig.Reason, Pnl: sig.Pnl, PnlPct: sig.PnlPct,
			}

		case "grid_buy":
			pos, ok := positions[sig.StockCode]
			if !ok || !pos.GridActive {
				sig.Status = "skipped"
				sig.SkipReason = "网格未激活"
				return nil
			}
			actualQty := int(sig.PlannedAmount / openPrice / 100) * 100
			if actualQty <= 0 || actualQty < 100 {
				sig.Status = "skipped"
				sig.SkipReason = "网格数量不足"
				return nil
			}
			actualAmount := openPrice * float64(actualQty)
			if actualAmount > *cash {
				actualQty = int(*cash / openPrice / 100) * 100
				actualAmount = openPrice * float64(actualQty)
			}
			if actualQty <= 0 {
				sig.Status = "skipped"
				sig.SkipReason = "资金不足"
				return nil
			}
			*cash -= actualAmount
			gridComm := math.Max(actualAmount*COMMISSION_RATE, MIN_COMMISSION)
			*cash -= gridComm
			// Find the level from the reason string (format: "网格买入 L0 @...")
			level := 0
			// Store grid lot
			pos.GridLots = append(pos.GridLots, GridLot{Qty: actualQty, BuyPrice: openPrice, Level: level})
			sig.ExecPrice = openPrice
			sig.ExecQty = actualQty
			sig.ExecAmount = actualAmount
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: "grid_buy", Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: actualQty,
				Reason: sig.Reason,
			}

		case "grid_sell":
			pos, ok := positions[sig.StockCode]
			if !ok {
				sig.Status = "skipped"
				sig.SkipReason = "无持仓"
				return nil
			}
			sellQty := sig.PlannedQty
			// Remove matching grid lot
			for i, lot := range pos.GridLots {
				if lot.Qty == sellQty {
					pos.GridLots = append(pos.GridLots[:i], pos.GridLots[i+1:]...)
					break
				}
			}
			gridPnl := (openPrice - sig.PlannedPrice) * float64(sellQty) // plannedPrice used as cost basis
			gridPnlPct := 0.0
			if sig.PlannedPrice > 0 {
				gridPnlPct = (openPrice - sig.PlannedPrice) / sig.PlannedPrice * 100
			}
			*cash += openPrice * float64(sellQty)
			gsAmt := openPrice * float64(sellQty)
			commission := math.Max(gsAmt*COMMISSION_RATE, MIN_COMMISSION)
			stampTax := gsAmt * STAMP_TAX_RATE
			*cash -= commission + stampTax
			sig.ExecPrice = openPrice
			sig.ExecQty = sellQty
			sig.ExecAmount = gsAmt
			sig.Pnl = math.Round(gridPnl*100) / 100
			sig.PnlPct = math.Round(gridPnlPct*100) / 100
			sig.Status = "executed"
			return &backtestTrade{
				Date: sig.ExecDate, SignalDate: sig.SignalDate,
				Action: "grid_sell", Code: sig.StockCode,
				Name: sig.StockName, Price: openPrice, Quantity: sellQty,
				Reason: sig.Reason, Pnl: sig.Pnl, PnlPct: sig.PnlPct,
			}
		}

		sig.Status = "skipped"
		sig.SkipReason = fmt.Sprintf("未知操作类型: %s", sig.ActionType)
		return nil
	}

	// countPendingForDate counts pending signals for a given exec date
	countPendingForDate := func(signals *[]model.BacktestSignal, date string) int {
		count := 0
		for _, sig := range *signals {
			if sig.ExecDate == date && sig.Status == "pending" {
				count++
			}
		}
		return count
	}

	positions = make(map[string]*dcPosition)
	var allTrades []backtestTrade
	var equityPoints []map[string]interface{}
	var dailyReturns []float64
	prevDayEquity := capital

	// Pre-load pending signals from DB (for resumed tasks)
	var pendingSignals []model.BacktestSignal
	db.MySQL.Where("task_id = ? AND status = 'pending'", task.ID).Find(&pendingSignals)

	for di, date := range allDates {
		// Day start log
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 0,
			"system", "info", "", "",
			fmt.Sprintf("━━ 第%d天 %s ━ 持仓%d只 现金¥%.0f", di+1, date, len(positions), remainingCash),
			nil)
		// Update progress so frontend sees current day immediately
		updateProgress(di+1, totalDays,
			fmt.Sprintf("执行信号: 第%d天 %s (%d条待执行)", di+1, date, countPendingForDate(&pendingSignals, date)),
			"")

		// P1: Check cancellation via DB status (not context)
		// Context cancellation is unreliable — use explicit status check
		db.MySQL.First(task, task.ID)
		if task.Status == "cancelled" {
			return
		}

		if regDates[date] {
			remainingCash += s.RegularAmount
		}

		// ============================================================
		// PHASE 1: Execute pending signals (T+1 open)
		// ============================================================
		var todayTrades []backtestTrade
		logSeq := 100

		// Execute signals with exec_date == today
		for i := 0; i < len(pendingSignals); i++ {
			sig := &pendingSignals[i]
			if sig.ExecDate != date {
				continue
			}
			if sig.Status != "pending" {
				continue
			}

			// Last day: skip pending buy/add signals (only execute sells)
			if di == len(allDates)-1 && (sig.ActionType == "buy" || sig.ActionType == "add") {
				sig.Status = "skipped"
				sig.SkipReason = "最后交易日跳过买入"
				db.MySQL.Save(sig)
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, logSeq,
					"signal", "warn", sig.StockCode, sig.StockName,
					fmt.Sprintf("⏭ [%s] %s 信号跳过: 最后交易日不执行买入", sig.ActionType, sig.StockCode),
					nil)
				logSeq++
				continue
			}

			openPrice := kcache.GetOpen(sig.StockCode, date)
			trade := executeSignal(sig, openPrice, &remainingCash)

			// Save signal update to DB
			db.MySQL.Save(sig)

			if trade != nil {
				todayTrades = append(todayTrades, *trade)
				// Log the execution
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, logSeq,
					"trade", "info", sig.StockCode, sig.StockName,
					fmt.Sprintf("📌 [%s] %s %s %d股 @¥%.2f %s (信号:%s)",
						trade.Action, trade.Code, trade.Name, trade.Quantity, trade.Price, trade.Reason, sig.SignalDate),
					map[string]interface{}{
						"action": trade.Action, "price": trade.Price, "quantity": trade.Quantity,
						"reason": trade.Reason, "pnl": trade.Pnl, "pnlPct": trade.PnlPct,
						"signalDate": sig.SignalDate,
					})
				logSeq++
			} else if sig.Status == "skipped" {
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, logSeq,
					"signal", "warn", sig.StockCode, sig.StockName,
					fmt.Sprintf("⏭ [%s] %s 信号跳过: %s", sig.ActionType, sig.StockCode, sig.SkipReason),
					nil)
				logSeq++
			}
		}

		allTrades = append(allTrades, todayTrades...)

		// Update progress: signal execution done, moving to generation
		updateProgress(di+1, totalDays,
			fmt.Sprintf("生成信号: 第%d天 %s (扫描%d只)", di+1, date, len(universe)),
			"")

		// ============================================================
		// PHASE 2b: Last day — force liquidate all positions at close
		// ============================================================
		isLastDay := di == len(allDates)-1
		if isLastDay {
			// Force-sell all remaining holdings at today's close price
			for code, pos := range positions {
				closePrice := kcache.GetClose(code, date)
				if closePrice <= 0 {
					// No price data — sell at cost price as fallback
					closePrice = pos.BuyPrice
				}

				// Check if stock is limit-down (cannot sell)
				if dailyChange := kcache.GetDailyChange(code, date); dailyChange <= -9.8 {
					// Limit-down: cannot liquidate, keep position record
					log.Printf("[backtest] date=%s code=%s limit-down, skip forced liquidation", date, code)
					continue
				}
				sellAmount := closePrice * float64(pos.Quantity)
				pnl := (closePrice - pos.BuyPrice) * float64(pos.Quantity)
				pnlPct := 0.0
				if pos.BuyPrice > 0 {
					pnlPct = (closePrice - pos.BuyPrice) / pos.BuyPrice * 100
				}

				// Create executed signal for the forced sell
				forceSig := model.BacktestSignal{
					TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
					SignalDate: date, ExecDate: date,
					StockCode: pos.Code, StockName: pos.Name,
					ActionType: "sell",
					PlannedPrice: closePrice, PlannedQty: pos.Quantity,
					PlannedAmount: sellAmount,
					ExecPrice: closePrice, ExecQty: pos.Quantity, ExecAmount: sellAmount,
					Pnl: math.Round(pnl*100)/100, PnlPct: math.Round(pnlPct*100)/100,
					Status: "executed",
					SkipReason: "最后交易日强制清仓",
					Reason: "最后交易日强制清仓",
				}
				db.MySQL.Create(&forceSig)

				trade := backtestTrade{
					Date: date, SignalDate: date,
					Code: pos.Code, Name: pos.Name,
					Action: "sell", Price: closePrice, Quantity: pos.Quantity,
					Reason: "最后交易日强制清仓",
					Pnl: math.Round(pnl*100)/100,
					PnlPct: math.Round(pnlPct*100)/100,
				}
				todayTrades = append(todayTrades, trade)
				allTrades = append(allTrades, trade)

				remainingCash += sellAmount

				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, logSeq,
					"trade", "info", pos.Code, pos.Name,
					fmt.Sprintf("🏁 [强制清仓] %s %s %d股 @¥%.2f 盈亏¥%.2f (%.2f%%)",
						pos.Code, pos.Name, pos.Quantity, closePrice, pnl, pnlPct),
					map[string]interface{}{
						"action": "sell", "price": closePrice, "quantity": pos.Quantity,
						"reason": "最后交易日强制清仓", "pnl": math.Round(pnl*100)/100,
						"pnlPct": math.Round(pnlPct*100)/100, "signalDate": date,
					})
				logSeq++
			}
			// Clear all positions
			positions = make(map[string]*dcPosition)

			insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, logSeq,
				"system", "info", "", "",
				fmt.Sprintf("🏁 最后交易日强制清仓完成 剩余现金¥%.0f", remainingCash),
				nil)
		}

		// ============================================================
		// PHASE 2: Generate new signals (T day close)
		// ============================================================
		var newSignals []model.BacktestSignal
		if useV2 {
			// Convert universe from StockInfo to dcStockInfo
			v2universe := make([]dcStockInfo, len(universe))
			for i, si := range universe { v2universe[i] = dcStockInfo{Code: si.Code, Name: si.Name} }
			newSignals = generateSignalsV2(date, remainingCash, isLastDay,
				positions, v2universe, task, s,
				buyConds, sellConds, addConds, reduceConds,
				evalSingle, kcache, icache, getNextDate, evalConds, conceptCache, styleEngine)
		} else {
			newSignals = generateSignals(date, remainingCash, isLastDay)
		}

		// Batch save new signals
		for i := range newSignals {
			ns := &newSignals[i]
			if ns.ExecDate == "" {
				// Last day — no next day to execute, skip signal
				continue
			}
			db.MySQL.Create(ns)
			pendingSignals = append(pendingSignals, *ns)

			// Log signal generation
			insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 70,
				"signal", "info", ns.StockCode, ns.StockName,
				fmt.Sprintf("🔔 [%s] %s %s %d股 预估¥%.2f → 计划%s执行 %s",
					ns.ActionType, ns.StockCode, ns.StockName, ns.PlannedQty,
					ns.PlannedPrice, ns.ExecDate, ns.Reason),
				nil)
		}

		// ── Daily signal summary ──
		buyCnt, sellCnt, addCnt, reduceCnt, stopCnt := 0, 0, 0, 0, 0
		for _, ns := range newSignals {
			switch ns.ActionType {
			case "buy": buyCnt++
			case "sell": sellCnt++
			case "add": addCnt++
			case "reduce": reduceCnt++
			case "stop": stopCnt++
			}
		}
		if len(newSignals) > 0 {
			insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 75,
				"system", "info", "", "",
				fmt.Sprintf("▸ 信号输出: 买入%d 卖出%d 加仓%d 减仓%d 止损止盈%d | %d只→%d个信号",
					buyCnt, sellCnt, addCnt, reduceCnt, stopCnt, len(universe), buyCnt+sellCnt+addCnt+reduceCnt+stopCnt),
				nil)
		}

		// ── Daily snapshot & progress update ──
		posList := make([]map[string]interface{}, 0)
		totalEquity := remainingCash
		for _, pos := range positions {
			cp := kcache.GetClose(pos.Code, date)
			mv := cp * float64(pos.Quantity)
			pnl := (cp - pos.BuyPrice) * float64(pos.Quantity)
			pnlPct := 0.0
			if pos.BuyPrice > 0 { pnlPct = (cp - pos.BuyPrice) / pos.BuyPrice * 100 }
			// Track highest price for trailing stop
			if cp > pos.HighestPrice { pos.HighestPrice = cp }
			posList = append(posList, map[string]interface{}{
				"code": pos.Code, "name": pos.Name, "qty": pos.Quantity,
				"price": cp, "costPrice": pos.BuyPrice,
				"marketVal": math.Round(mv*100)/100,
				"pnl": math.Round(pnl*100)/100, "pnlPct": math.Round(pnlPct*100)/100,
			})
			totalEquity += mv
		}

		// Append today's sold stocks to posList for display
		soldList := make([]map[string]interface{}, 0)
		for _, t := range todayTrades {
			if t.Action == "sell" || t.Action == "reduce" || t.Action == "stop" {
				soldList = append(soldList, map[string]interface{}{
					"code": t.Code, "name": t.Name, "qty": 0,
					"soldQty": t.Quantity,
					"price": t.Price, "costPrice": 0,
					"marketVal": 0,
					"pnl": math.Round(t.Pnl*100)/100,
					"pnlPct": math.Round(t.PnlPct*100)/100,
					"sold": true,
				})
			}
		}
		allPositions := append(posList, soldList...)

		posData := map[string]interface{}{
			"date": date, "day": di+1, "totalDays": totalDays,
			"cash": math.Round(remainingCash*100)/100,
			"totalEquity": math.Round(totalEquity*100)/100,
			"totalReturn": math.Round((totalEquity-capital)/capital*10000)/100,
			"positions": allPositions,
			"positionCount": len(positions),
			"soldCount": len(soldList),
			"recentTrades": todayTrades,
		}
		posBytes, _ := json.Marshal(posData)
		updateProgress(di+1, totalDays, "", string(posBytes))

		// Equity point
		equityPoints = append(equityPoints, map[string]interface{}{
			"date": date, "equity": math.Round(totalEquity*100) / 100,
		})

		// ── Insert daily snapshot ──
		dailyRet := 0.0
		if di > 0 {
			prevEquity := capital
			if len(equityPoints) >= 2 {
				prevEquity = equityPoints[len(equityPoints)-2]["equity"].(float64)
			}
			if prevEquity > 0 {
				dailyRet = (totalEquity - prevEquity) / prevEquity * 100
			}
		}
		dailyReturns = append(dailyReturns, dailyRet)
		cumRet := (totalEquity - capital) / capital * 100
		peak := capital
		for _, eq := range equityPoints {
			e := eq["equity"].(float64)
			if e > peak { peak = e }
		}
		currentDD := 0.0
		if peak > 0 { currentDD = (peak - totalEquity) / peak * 100 }
		insertDailySnapshot(task.ID, task.StrategyID, task.UserID, date, di+1,
			remainingCash, totalEquity, dailyRet, cumRet, currentDD, len(positions), posList)


		// Day end summary
		dailyPnl := totalEquity - prevDayEquity
		prevDayEquity = totalEquity
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 999,
			"system", "info", "", "",
			fmt.Sprintf("▸ 日终结算: 权益¥%.0f 日盈亏%+.0f 累计%+.1f%% 持仓%d只", totalEquity, dailyPnl, cumRet, len(positions)),
			nil)
	}

	// Calculate final metrics
	winCount := 0
	for _, t := range allTrades {
		if (t.Action == "sell" || t.Action == "reduce" || t.Action == "stop" || t.Action == "dip_sell" || t.Action == "grid_sell") && t.Pnl > 0 { winCount++ }
	}
	sellCount := 0
	for _, t := range allTrades {
		if t.Action == "sell" || t.Action == "reduce" || t.Action == "stop" || t.Action == "dip_sell" || t.Action == "grid_sell" { sellCount++ }
	}
	winRate := 0.0
	if sellCount > 0 { winRate = float64(winCount) / float64(sellCount) * 100 }

	finalEquity := equityPoints[len(equityPoints)-1]["equity"].(float64)
	totalReturn := (finalEquity - capital) / capital * 100

	peak := capital
	maxDD := 0.0
	for _, eq := range equityPoints {
		e := eq["equity"].(float64)
		if e > peak { peak = e }
		dd := (peak - e) / peak * 100
		if dd > maxDD { maxDD = dd }
	}

	// Sharpe Ratio: annualized (mean daily return / std dev of daily returns) * sqrt(252)
	sharpe := 0.0
	if len(dailyReturns) > 1 {
		mean := 0.0
		for _, r := range dailyReturns {
			mean += r
		}
		mean /= float64(len(dailyReturns))
		variance := 0.0
		for _, r := range dailyReturns {
			diff := r - mean
			variance += diff * diff
		}
		stdDev := math.Sqrt(variance / float64(len(dailyReturns)-1))
		if stdDev > 1e-9 {
			sharpe = (mean / stdDev) * math.Sqrt(252)
		}
	}

	// Compute indicator coverage
	usedIndicators := make(map[string]bool)
	allConds := append(append(append(buyConds, addConds...), sellConds...), reduceConds...)
	for _, c := range allConds {
		usedIndicators[c.Indicator] = true
	}
	var coverageStats []map[string]interface{}
	klineSafe := 0
	klineUnsafe := 0
	for ind := range usedIndicators {
		isKlineDerived := strings.HasPrefix(ind, "daily_change") || strings.HasPrefix(ind, "momentum") ||
			ind == "ma_deviation" || ind == "ma_cross" || ind == "macd" || ind == "ema_cross" ||
			ind == "rsi" || strings.HasPrefix(ind, "kdj") || strings.HasPrefix(ind, "boll") ||
			strings.HasPrefix(ind, "volume") || ind == "turnover_rate" || ind == "atr" ||
			strings.HasPrefix(ind, "drawdown") || strings.HasPrefix(ind, "new_high") ||
			ind == "up_days_ratio" || strings.HasPrefix(ind, "price_position") ||
			ind == "adx" || strings.HasPrefix(ind, "dmi_") ||
			ind == "cci" || ind == "williams_r" || ind == "mfi" ||
			ind == "atr_pct" || ind == "ma_convergence" || ind == "trend_strength" ||
			ind == "consecutive_days" || ind == "gap_pct" || ind == "high_low_range" ||
			ind == "vwap_deviation" || ind == "index_relative"
		if isKlineDerived {
			klineSafe++
		} else {
			klineUnsafe++
		}
		coverageStats = append(coverageStats, map[string]interface{}{
			"indicator": ind,
			"klineSafe": isKlineDerived,
		})
	}
	coveragePct := 0.0
	if len(usedIndicators) > 0 {
		coveragePct = float64(klineSafe) / float64(len(usedIndicators)) * 100
	}

	// Determine pool key (short enum) from task params
	poolKey := "all"
	if task.Params != "" {
		var params map[string]interface{}
		if json.Unmarshal([]byte(task.Params), &params) == nil {
			if pool, ok := params["stockPool"].(string); ok && pool != "" {
				poolKey = pool
			}
		}
	}

	// Build stock pool params for replay
	poolParamsBytes, _ := json.Marshal(universeCodes)
	poolParamsJSON := string(poolParamsBytes)

	// Save result
	bt := model.BacktestResult{
		TaskID:          task.ID,
		UserID:          task.UserID,
		StrategyID:      task.StrategyID,
		StockPool:       poolKey,
		StockPoolParams: poolParamsJSON,
		StartDate:       parseDate(startDate),
		EndDate:         parseDate(endDate),
		InitialCapital:  capital,
		FinalEquity:     math.Round(finalEquity*100) / 100,
		TotalReturn:     math.Round(totalReturn*100) / 100,
		SharpeRatio:     math.Round(sharpe*100) / 100,
		MaxDrawdown:     math.Round(maxDD*100) / 100,
		WinRate:         math.Round(winRate*100) / 100,
		TradeCount:      sellCount, // completed (sell+reduce), not all actions
		Trades:          model.JSONMap{"data": allTrades},
		EquityCurve:     model.JSONMap{"data": equityPoints},
		Coverage:        model.JSONMap{"stats": coverageStats, "klineSafe": klineSafe, "klineUnsafe": klineUnsafe, "coveragePct": coveragePct},
	}
	if err := db.MySQL.Create(&bt).Error; err != nil {
		log.Printf("[backtest] failed to save result for task %d: %v", task.ID, err)
		db.MySQL.Model(task).Updates(map[string]interface{}{
			"status":    "failed",
			"phase":     "保存结果失败",
			"error_msg": fmt.Sprintf("保存结果失败: %v", err),
		})
		rm := getRunningMap(task.StrategyID)
		delete(rm, task.ID)
		return
	}

	// Final system log
	insertBacktestLog(task.ID, task.StrategyID, task.UserID, "", 0,
		"system", "info", "", "",
		fmt.Sprintf("回测完成: 收益率%.2f%%, 夏普%.2f, 最大回撤%.2f%%, 胜率%.2f%%, 交易%d次",
			totalReturn, sharpe, maxDD, winRate, len(allTrades)),
		map[string]interface{}{
			"totalReturn": math.Round(totalReturn*100)/100,
			"sharpe": math.Round(sharpe*100)/100,
			"maxDrawdown": math.Round(maxDD*100)/100,
			"winRate": math.Round(winRate*100)/100,
			"tradeCount": len(allTrades),
		})

	now2 := time.Now()
	db.MySQL.Model(task).Updates(map[string]interface{}{
		"status":        "completed",
		"phase":         "回测完成",
		"result_id":     bt.ID,
		"final_equity":  math.Round(finalEquity*100) / 100,
		"total_return":  math.Round(totalReturn*100) / 100,
		"progress_pct":  100,
		"completed_at":  now2,
	})

	// Clean up running map
	rm := getRunningMap(task.StrategyID)
	delete(rm, task.ID)
}

// ── Legacy SSE backtest (kept for backward compat, delegates to async) ──

func (h *StrategyHandler) RunBacktest(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		StartDate  string   `json:"startDate"`
		EndDate    string   `json:"endDate"`
		StockCodes []string `json:"stockCodes"`
		StockPool  string   `json:"stockPool"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, 500, response.CodeInternalError, "不支持流式输出")
		return
	}

	sendSSE := func(typ string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: {\"type\":\"%s\",\"payload\":%s}\n\n", typ, string(b))
		flusher.Flush()
	}

	// Load strategy
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		sendSSE("error", map[string]string{"message": "策略不存在"})
		return
	}

	// Count trading days
	var totalDays int
	if err := db.PG.Raw(`SELECT COUNT(DISTINCT trade_date) FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ?`, body.StartDate, body.EndDate).Scan(&totalDays).Error; err != nil {
		sendSSE("error", map[string]string{"message": "查询交易日数据失败: " + err.Error()})
		return
	}
	if totalDays == 0 {
		sendSSE("error", map[string]string{"message": "所选时间段无交易日数据"})
		return
	}

	resolvedCodes := resolveStockPool(uid, body.StockPool, body.StockCodes)

	// Create task
	paramsBytes, _ := json.Marshal(map[string]interface{}{
		"startDate":  body.StartDate,
		"endDate":    body.EndDate,
		"stockCodes": resolvedCodes,
		"stockPool":  body.StockPool,
	})
	task := model.BacktestTask{
		UserID:     uid,
		StrategyID: uint(sid),
		Status:     "pending",
		Phase:      "初始化",
		TotalDays:  totalDays,
		Params:     string(paramsBytes),
	}
	db.MySQL.Create(&task)

	sendSSE("taskId", map[string]interface{}{"taskId": task.ID})

	// Run inline synchronously
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	getRunningMap(uint(sid))[task.ID] = cancel

	h.runBacktestAsync(ctx, &task, &s, body.StartDate, body.EndDate, resolvedCodes)

	// Reload task for final status
	db.MySQL.First(&task, task.ID)
	if task.Status == "completed" && task.ResultID != nil {
		var bt model.BacktestResult
		db.MySQL.First(&bt, *task.ResultID)
		metrics := map[string]interface{}{
			"totalReturn": bt.TotalReturn,
			"sharpeRatio": bt.SharpeRatio,
			"maxDrawdown": bt.MaxDrawdown,
			"winRate":     bt.WinRate,
			"tradeCount":  bt.TradeCount,
		}
		sendSSE("metric", metrics)
		sendSSE("phase", map[string]string{"phase": "saved", "message": fmt.Sprintf("回测结果已保存 (ID: %d)", bt.ID)})
	} else if task.Status == "failed" {
		sendSSE("error", map[string]string{"message": task.ErrorMsg})
	}
	sendSSE("done", map[string]string{"message": "回测" + task.Status})
}
func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func (h *StrategyHandler) BacktestHistory(c *gin.Context) {
	uid := getUID(c)
	sid := c.Query("strategyId")
	var results []model.BacktestResult
	q := db.MySQL.Where("user_id = ?", uid)
	if sid != "" {
		q = q.Where("strategy_id = ?", sid)
	}
	q.Order("created_at DESC").Limit(20).Find(&results)

	// Resolve pool labels for display
	for i := range results {
		results[i].StockPoolLabel = resolveStockPoolLabel(results[i].StockPool, results[i].StockPoolParams)
	}

	response.Success(c, results)
}

// DeleteBacktestResult deletes a single backtest result by ID
func (h *StrategyHandler) DeleteBacktestResult(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	result := db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.BacktestResult{})
	if result.RowsAffected == 0 {
		response.NotFound(c, "记录不存在")
		return
	}
	response.SuccessMsg(c, "已删除")
}

// GetBacktestResult returns a single backtest result by ID
func (h *StrategyHandler) GetBacktestResult(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var result model.BacktestResult
	if db.MySQL.Where("id = ? AND user_id = ?", id, uid).First(&result).Error != nil {
		response.NotFound(c, "记录不存在")
		return
	}
	result.StockPoolLabel = resolveStockPoolLabel(result.StockPool, result.StockPoolParams)
	response.Success(c, result)
}

// StockPool returns available stock pools for backtest selection
func (h *StrategyHandler) StockPool(c *gin.Context) {
	uid := getUID(c)
	
	type PoolItem struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	type PoolGroup struct {
		Key   string     `json:"key"`
		Label string     `json:"label"`
		Count int        `json:"count"`
		Items []PoolItem `json:"items,omitempty"`
	}

	pools := []PoolGroup{}

	// 1. All stocks
	var allCount int64
	if err := db.PG.Raw("SELECT COUNT(*) FROM stocks_basic").Scan(&allCount).Error; err != nil {
		log.Printf("[strategy] stock count query failed: %v", err)
	}
	var allStocks []PoolItem
	if err := db.PG.Raw("SELECT code, COALESCE(name,'') as name FROM stocks_basic ORDER BY code LIMIT 5000").Scan(&allStocks).Error; err != nil {
		log.Printf("[strategy] all stocks query failed: %v", err)
	}
	pools = append(pools, PoolGroup{
		Key:   "all",
		Label: "全部股票",
		Count: int(allCount),
		Items: allStocks,
	})

	// 2. Watchlist groups (MySQL)
	type WLGroup struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	var wlGroups []WLGroup
	if err := db.MySQL.Raw("SELECT id, name FROM watchlist_groups WHERE user_id = ? ORDER BY sort_order", uid).Scan(&wlGroups).Error; err != nil {
		log.Printf("[strategy] watchlist groups query failed: %v", err)
	}
	for _, g := range wlGroups {
		// Query watchlist stocks from MySQL, then join with PG for names
		type WLRaw struct {
			StockCode string
		}
		var wlRaw []WLRaw
		if err := db.MySQL.Raw("SELECT stock_code FROM watchlists WHERE user_id = ? AND group_id = ? ORDER BY stock_code", uid, g.ID).Scan(&wlRaw).Error; err != nil {
			log.Printf("[strategy] watchlist items query failed for group %d: %v", g.ID, err)
			continue
		}
		codes := make([]string, len(wlRaw))
		for i, w := range wlRaw {
			codes[i] = w.StockCode
		}
		var items []PoolItem
		if len(codes) > 0 {
			if err := db.PG.Raw(fmt.Sprintf("SELECT code, COALESCE(name,'') as name FROM stocks_basic WHERE code IN (%s) ORDER BY code", db.CodesToInClause(codes))).Scan(&items).Error; err != nil {
			log.Printf("[strategy] watchlist stock names query failed: %v", err)
			continue
		}
		}
		pools = append(pools, PoolGroup{
			Key:   fmt.Sprintf("watchlist_%d", g.ID),
			Label: "自选组: " + g.Name,
			Count: len(items),
			Items: items,
		})
	}

	// 3. Portfolio holdings (MySQL → PG join)
	type HoldingRaw struct {
		Code string
	}
	var holdingCodes []HoldingRaw
	if err := db.MySQL.Raw("SELECT DISTINCT stock_code as code FROM holdings WHERE user_id = ? ORDER BY stock_code", uid).Scan(&holdingCodes).Error; err != nil {
		log.Printf("[strategy] holdings query failed: %v", err)
	}
	if len(holdingCodes) > 0 {
		codes := make([]string, len(holdingCodes))
		for i, h := range holdingCodes {
			codes[i] = h.Code
		}
		var holdings []PoolItem
		if err := db.PG.Raw(fmt.Sprintf("SELECT code, COALESCE(name,'') as name FROM stocks_basic WHERE code IN (%s) ORDER BY code", db.CodesToInClause(codes))).Scan(&holdings).Error; err != nil {
			log.Printf("[strategy] holdings stock names query failed: %v", err)
		}
		if len(holdings) > 0 {
			pools = append(pools, PoolGroup{
				Key:   "portfolio",
				Label: "我的持仓",
				Count: len(holdings),
				Items: holdings,
			})
		}
	}

	response.Success(c, pools)
}

// ── Available indicators list ──

func buildIndicatorList() []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(IndicatorRegistry))
	for _, m := range IndicatorRegistry {
		result = append(result, map[string]interface{}{
			"key":          m.Key,
			"label":        m.Label,
			"category":     m.Category,
			"unit":         m.Unit,
			"type":         m.Type,
			"operators":    m.Operators,
			"desc":         m.Desc,
			"backtestSafe": m.BacktestSafe,
			"dataNote":     m.DataNote,
			"suggestion":   m.Suggestion,
			"useFor":       m.UseFor,
			"dataSource":   m.DataSource,
		})
	}
	return result
}

// buildIndicatorListLegacy was the old hardcoded list; replaced by Registry. Keep for reference.
func buildIndicatorListLegacy() []map[string]interface{} {
	return []map[string]interface{}{
		// ═══ 榜单与评分 ═══
		{"key": "streak_count", "label": "连榜次数", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "该股票在榜单连续出现的交易日数", "backtestSafe": true, "dataNote": "依赖榜单数据覆盖", "suggestion": "买入建议 ≥ 3 天，连续上榜说明持续受关注"},
		{"key": "algo_score", "label": "算法评分", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "算法团队给出的综合评分 (0-10)", "backtestSafe": true, "dataNote": "仅榜单日期有值", "suggestion": "买入建议 ≥ 6 分，算法团队评分越高越好"},
		{"key": "signal_value", "label": "原始信号值", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "算法团队原始信号值（越大越强）", "backtestSafe": false, "dataNote": "⚠️ 单点快照无历史，回测禁用", "suggestion": "买入建议 > 0.5，正值表示信号偏多"},

		// ═══ AI六维评分 ═══
		{"key": "ai_score", "label": "AI综合评分", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI六维综合评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，AI综合评估偏高"},
		{"key": "ai_fundamental", "label": "AI基本面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI基本面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，基本面扎实"},
		{"key": "ai_technical", "label": "AI技术面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI技术面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，技术形态良好"},
		{"key": "ai_valuation", "label": "AI估值", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI估值评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，估值合理偏低"},
		{"key": "ai_growth", "label": "AI成长性", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI成长性评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，成长性突出"},
		{"key": "ai_industry", "label": "AI行业面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI行业面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，行业景气度高"},
		{"key": "ai_capital", "label": "AI资金面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI资金面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，资金面积极"},

		// ═══ 技术面 — 趋势类 (100% K线数据覆盖) ═══
		{"key": "daily_change", "label": "单日涨跌幅", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日涨跌幅 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 2% 或 < -5% 超跌反弹"},
		{"key": "momentum_5", "label": "5日动量", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近5个交易日累计涨跌幅 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 3%，短期趋势向上"},
		{"key": "momentum_20", "label": "20日动量", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20个交易日累计涨跌幅 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 5%，中期趋势确立"},
		{"key": "ma_deviation", "label": "均线偏离", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价偏离MA20的百分比", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -5% 超跌，卖出建议 > 10% 超涨"},
		{"key": "ma_5", "label": "MA5均线", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "5日收盘均价", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "收盘价>MA5短线偏多，<MA5偏空"},
		{"key": "ma_10", "label": "MA10均线", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "10日收盘均价", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "MA5>MA10短线金叉"},
		{"key": "ma_20", "label": "MA20均线", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "20日收盘均价(月线)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "收盘价>MA20中线偏多，常用止损位"},
		{"key": "ma_30", "label": "MA30均线", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "30日收盘均价", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "中期趋势判断位"},
		{"key": "ma_60", "label": "MA60均线", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "60日收盘均价(季线)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "收盘价>MA60长线偏多，重要支撑压力位"},
		{"key": "ma_cross", "label": "MA均线交叉", "type": "cross", "operators": []string{"cross_up", "cross_down"}, "desc": "value填均线周期如5/20表示MA5上穿/下穿MA20", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "上穿买入，如 MA5↑MA20 为短线金叉"},
		{"key": "macd", "label": "MACD信号", "type": "cross", "operators": []string{"cross_up", "cross_down", "eq"}, "desc": "MACD(12,26,9)金叉=1/死叉=-1/无交叉=0", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "eq=1 买入(金叉), eq=-1 卖出(死叉), 零轴上方金叉更可靠"},
		{"key": "macd_dif", "label": "MACD DIF", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "cross_up", "cross_down"}, "desc": "MACD快线DIF值 (EMA12-EMA26)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "DIF>0多头，DIF>DEA金叉看涨"},
		{"key": "macd_dea", "label": "MACD DEA", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "MACD慢线DEA值 (DIF的9日EMA)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "DEA>0多头区域，DIF上穿DEA金叉"},

		// ═══ 技术面 — 超买超卖 (100% K线数据覆盖) ═══
		{"key": "rsi", "label": "RSI(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "相对强弱指数，>70超买 <30超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 30 超卖，卖出建议 > 70 超买"},
		{"key": "kdj_k", "label": "KDJ-K", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "KDJ指标K值(9,3,3)，>80超买 <20超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 20 超卖，卖出建议 > 80 超买"},
		{"key": "kdj_d", "label": "KDJ-D", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "KDJ指标D值", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 30，卖出建议 > 70"},
		{"key": "kdj_j", "label": "KDJ-J", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "KDJ指标J值，>100极度超买 <0极度超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0 极度超卖，卖出建议 > 100 极度超买"},
		{"key": "boll_position", "label": "布林带位置", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "价格在布林带(20,2)中的位置%，>80上轨 <20下轨", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0.1 触下轨，卖出建议 > 0.9 触上轨"},
		{"key": "boll_width", "label": "布林带宽度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "布林带(上轨-下轨)/中轨，衡量波动率", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 5 收口将变盘，卖出建议 > 15 宽口波动大"},

		// ═══ 技术面 — 量价 (100% K线数据覆盖) ═══
		{"key": "volume_ratio", "label": "量比(5日)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日成交量与前5日均量之比", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 1.5 放量，量价配合更可靠"},
		{"key": "volume_ma_ratio", "label": "量比(20日)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日成交量与20日均量之比，>1.5放量", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 1.2，20日均量以上为活跃"},
		{"key": "turnover_rate", "label": "换手率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日换手率 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 3-10%，过高需警惕出货"},
		{"key": "atr", "label": "ATR(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "平均真实波幅，衡量波动率", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "波动率指标，值越大波动越大，无固定阈值"},

		// ═══ 技术面 — 形态与强度 (100% K线数据覆盖) ═══
		{"key": "drawdown_20", "label": "20日最大回撤", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20个交易日最大回撤 (%)，负值代表回撤", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 15% 超跌反弹机会，结合基本面判断"},
		{"key": "new_high_20", "label": "20日新高", "type": "number", "operators": []string{"eq"}, "desc": "当日收盘是否为20日新高，1=是 0=否", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 = 1 创新高，强势突破信号"},
		{"key": "up_days_ratio", "label": "上涨天数占比", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20个交易日上涨天数占比，>0.5偏强", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 0.6，阳线居多说明多头主导"},
		{"key": "price_position_20", "label": "20日价格位置", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价在近20日最高最低之间的位置%，>80高位", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0.3 低位，> 0.7 高位注意风险"},
		{"key": "price_position_60", "label": "60日价格位置", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价在近60日最高最低之间的位置%", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0.3 中长期低位区域"},


		// ═══ 技术面 — 进阶：趋势系统 ═══
		{"key": "adx", "label": "ADX(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "平均趋向指数，>25有趋势 >50强趋势", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 25 趋势明确，> 40 强趋势"},
		{"key": "dmi_plus", "label": "DMI+ (PDI)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "上升方向线，PDI>MDI多头占优", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "PDI > MDI 多头占优，差值越大越强"},
		{"key": "dmi_minus", "label": "DMI- (MDI)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "下降方向线，MDI>PDI空头占优", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "MDI > PDI 空头占优"},
		{"key": "ema_cross", "label": "EMA交叉", "type": "cross", "operators": []string{"cross_up", "cross_down"}, "desc": "value填12/26表示EMA12上穿/下穿EMA26", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖"},

		// ═══ 技术面 — 进阶：超买超卖扩展 ═══
		{"key": "cci", "label": "CCI(20)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "商品通道指数，>100超买 <-100超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -100 超卖，卖出建议 > 100 超买"},
		{"key": "williams_r", "label": "威廉%R(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "威廉指标，>-20超买 <-80超卖（负值）", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -80 超卖，卖出建议 > -20 超买"},
		{"key": "mfi", "label": "MFI(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "资金流量指标(量价RSI)，>80超买 <20超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 20 资金超卖，卖出建议 > 80 资金超买"},

		// ═══ 技术面 — 进阶：波动与结构 ═══
		{"key": "boll_squeeze", "label": "布林挤压", "type": "number", "operators": []string{"gte", "lte"}, "desc": "布林带宽度处于N日最低的百分位，<10预示变盘", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 5 极度收口，变盘在即"},
		{"key": "atr_pct", "label": "ATR/价格%", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "ATR(14)/收盘价*100，标准化波动率", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议结合趋势，高波动率需设宽止损"},
		{"key": "ma_convergence", "label": "均线粘合度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "MA5/10/20/60变异系数，<3%高度粘合预示变盘", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 2 均线粘合，即将选择方向"},
		{"key": "trend_strength", "label": "趋势强度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20日收盘>MA20的天数占比，>0.7强多头", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 0.6，趋势评分越高方向越明确"},

		// ═══ 技术面 — 进阶：形态与量价 ═══
		{"key": "consecutive_days", "label": "连续涨跌", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "连续涨跌天数，正=连涨 负=连跌", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 3 连续阳线强势，< -3 超跌"},
		{"key": "gap_pct", "label": "跳空缺口%", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日开盘相对昨收的跳空幅度%", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -3% 跳空缺口可关注回补"},
		{"key": "high_low_range", "label": "日内振幅%", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "(最高-最低)/昨收*100，衡量日内波动", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "振幅指标，日内振幅大适合短线，无固定阈值"},
		{"key": "vwap_deviation", "label": "VWAP偏离", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价偏离当日VWAP的%，正值强于均价", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -2% 低于均价可能反弹"},
		{"key": "volume_trend", "label": "量能趋势", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "成交量MA5/MA20-1，>0放量趋势", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 0 放量趋势，量涨价增更可靠"},
		{"key": "index_relative", "label": "大盘相对强度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "个股20日收益-上证20日收益，正值跑赢大盘", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 5 跑赢大盘，说明个股强势"},
		// ═══ 估值 (依赖indicator表，仅2天历史) ═══
		{"key": "pe", "label": "市盈率PE", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当前市盈率", "backtestSafe": true, "dataNote": "📊 ~3500只股票覆盖，2024-07起有历史数据", "suggestion": "买入建议 < 15 低估值，< 10 极度低估"},
		{"key": "pb", "label": "市净率PB", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当前市净率", "backtestSafe": true, "dataNote": "📊 ~3500只股票覆盖，2024-07起有历史数据", "suggestion": "买入建议 < 1.5 低市净率，金融股可放宽"},
		{"key": "ps", "label": "市销率PS", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当前市销率", "backtestSafe": true, "dataNote": "📊 ~3500只股票覆盖，2024-07起有历史数据", "suggestion": "买入建议 < 2，成长股可适当放宽"},
		{"key": "pe_percentile", "label": "PE历史分位", "type": "number", "operators": []string{"gte", "lte"}, "desc": "当前PE在历史数据中的百分位，<30低估 >70高估", "backtestSafe": true, "dataNote": "📊 基于 ~580个交易日历史PE计算，2024-07起可用", "suggestion": "买入建议 < 30 历史低位，> 70 历史高位"},
		{"key": "pb_percentile", "label": "PB历史分位", "type": "number", "operators": []string{"gte", "lte"}, "desc": "当前PB在历史数据中的百分位，<30低估 >70高估", "backtestSafe": true, "dataNote": "📊 基于 ~580个交易日历史PB计算，2024-07起可用", "suggestion": "买入建议 < 30 历史低位，> 70 历史高位"},

		// ═══ 基本面 (30只股票，8报告期) ═══
		{"key": "roe", "label": "ROE", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "净资产收益率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 15%，ROE越高盈利能力越强"},
		{"key": "revenue_growth", "label": "营收增长率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "营收同比增长率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 10%，持续增长为佳"},
		{"key": "profit_growth", "label": "利润增长率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "归母净利润同比增长率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 15%，利润增速高更有价值"},
		{"key": "gross_margin", "label": "毛利率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "销售毛利率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 30%，高毛利率有定价权"},
		{"key": "net_margin", "label": "净利率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "销售净利率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 10%，净利率越高盈利质量越好"},
		{"key": "debt_ratio", "label": "资产负债率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "资产负债率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 < 60%，过高财务风险大"},
		{"key": "eps", "label": "每股收益", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "基本每股收益 (元)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 0.5 元且持续增长"},

		// ═══ 资金面/市场面 ═══
		{"key": "total_market_cap", "label": "总市值(亿)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "总市值 (亿元)", "backtestSafe": false, "dataNote": "⚠️ 仅最近2天数据，回测取最近可用值", "suggestion": "无固定阈值，大盘股 > 1000亿 稳健，小盘股 < 100亿 弹性大"},
		{"key": "shareholder_change", "label": "股东户数变化", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "股东户数环比变化%，负值表示筹码集中", "backtestSafe": true, "dataNote": "📊 53只股票覆盖，回测取最近报告", "suggestion": "买入建议 < -5% 筹码集中，> 10% 筹码分散警惕"},
		{"key": "inst_hold_ratio", "label": "机构持股比", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "机构持股占流通股比例 (%)", "backtestSafe": true, "dataNote": "📊 53只股票覆盖，回测取最近报告", "suggestion": "买入建议 > 30%，机构持股比例高更受认可"},

		// ═══ 预测 (纯未来数据) ═══
		{"key": "prediction_upside", "label": "预测上涨空间", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "多模型预测均价相对现价的上涨空间 (%)", "backtestSafe": false, "dataNote": "🚫 预测为未来数据，回测不可用", "suggestion": "买入建议 > 10%，预测上涨空间越大越好"},
		{"key": "prediction_consensus", "label": "预测一致性", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "预测看涨的模型占比 (0-1)，>0.5多数看涨", "backtestSafe": false, "dataNote": "🚫 预测为未来数据，回测不可用", "suggestion": "买入建议 > 0.6，多数模型看涨为佳"},
	}
}

func (h *StrategyHandler) Indicators(c *gin.Context) {
	response.Success(c, buildIndicatorList())
}

// IndicatorGuide returns a comprehensive indicator guide for frontend consumption,
// organized by category with full metadata for building interactive indicator forms.
func (h *StrategyHandler) IndicatorGuide(c *gin.Context) {
	type IndicatorGuideItem struct {
		Key          string   `json:"key"`
		Label        string   `json:"label"`
		Category     string   `json:"category"`
		Unit         string   `json:"unit"`
		Type         string   `json:"type"`         // number / cross
		Operators    []string `json:"operators"`    // available operators for this indicator
		Desc         string   `json:"desc"`
		BacktestSafe bool     `json:"backtestSafe"`
		DataNote     string   `json:"dataNote"`
		Suggestion   string   `json:"suggestion"`
		UseFor       string   `json:"useFor"`       // buy / sell / both
		DataSource   string   `json:"dataSource"`
		ValueType    string   `json:"valueType"`    // "number" | "cross"
		ValueExample string   `json:"valueExample"` // example value for UI hint
		ValueMin     *float64 `json:"valueMin,omitempty"`
		ValueMax     *float64 `json:"valueMax,omitempty"`
	}

	type CategoryGroup struct {
		Category string               `json:"category"`
		Label    string               `json:"label"`
		Items    []IndicatorGuideItem `json:"items"`
	}

	// Build categories map
	catMap := make(map[string]*CategoryGroup)
	categoryOrder := []string{"榜单与评分", "AI评分", "技术面-趋势", "技术面-超买超卖", "技术面-量价", "技术面-形态", "估值", "基本面", "资金面", "预测"}

	for _, m := range IndicatorRegistry {
		item := IndicatorGuideItem{
			Key:          m.Key,
			Label:        m.Label,
			Category:     m.Category,
			Unit:         m.Unit,
			Type:         m.Type,
			Operators:    m.Operators,
			Desc:         m.Desc,
			BacktestSafe: m.BacktestSafe,
			DataNote:     m.DataNote,
			Suggestion:   m.Suggestion,
			UseFor:       m.UseFor,
			DataSource:   m.DataSource,
			ValueType:    m.Type,
			ValueExample: getValueExample(m),
		}
		// Set value range hints for number types
		if m.Type == "number" {
			if min, max, ok := getValueRange(m.Key); ok {
				item.ValueMin = min
				item.ValueMax = max
			}
		}
		if m.Type == "cross" {
			item.ValueExample = "5/20"
		}

		if _, ok := catMap[m.Category]; !ok {
			catMap[m.Category] = &CategoryGroup{Category: m.Category, Label: m.Category, Items: []IndicatorGuideItem{}}
		}
		catMap[m.Category].Items = append(catMap[m.Category].Items, item)
	}

	// Build ordered response
	result := make([]CategoryGroup, 0, len(categoryOrder))
	for _, cat := range categoryOrder {
		if g, ok := catMap[cat]; ok {
			result = append(result, *g)
		}
	}
	// Append any categories not in the ordered list
	for _, g := range catMap {
		found := false
		for _, c := range categoryOrder {
			if c == g.Category {
				found = true
				break
			}
		}
		if !found {
			result = append(result, *g)
		}
	}

	// Also return flat operator enum for dropdowns
	operatorEnum := []map[string]string{
		{"key": "gte", "label": "≥ 大于等于", "appliesTo": "number"},
		{"key": "lte", "label": "≤ 小于等于", "appliesTo": "number"},
		{"key": "gt", "label": "> 大于", "appliesTo": "number"},
		{"key": "lt", "label": "< 小于", "appliesTo": "number"},
		{"key": "eq", "label": "= 等于", "appliesTo": "number"},
		{"key": "cross_up", "label": "↑ 上穿", "appliesTo": "cross"},
		{"key": "cross_down", "label": "↓ 下穿", "appliesTo": "cross"},
	}

	response.Success(c, map[string]interface{}{
		"categories":  result,
		"operators":   operatorEnum,
		"totalCount":  len(IndicatorRegistry),
	})
}

// getValueExample returns a human-readable example value for an indicator.
func getValueExample(m *IndicatorMeta) string {
	switch m.Key {
	case "algo_score", "ai_score", "ai_fundamental", "ai_technical", "ai_valuation", "ai_growth", "ai_industry", "ai_capital":
		return "6"
	case "streak_count":
		return "3"
	case "pick_count_5d":
		return "2"
	case "pick_count_20d":
		return "5"
	case "signal_value":
		return "0.5"
	case "daily_change":
		return "2"
	case "momentum_5":
		return "3"
	case "momentum_20":
		return "5"
	case "ma_deviation":
		return "5"
	case "ma_cross", "ema_cross":
		return "5/20"
	case "macd", "macd_dif", "macd_dea":
		return "0"
	case "rsi", "rsi_6":
		return "70"
	case "rsi_12", "rsi_24":
		return "60"
	case "kdj_k":
		return "80"
	case "kdj_d":
		return "70"
	case "kdj_j":
		return "90"
	case "boll_position":
		return "80"
	case "boll_width":
		return "10"
	case "boll_squeeze":
		return "1"
	case "volume_ratio":
		return "2"
	case "volume_ma_ratio":
		return "1.5"
	case "turnover_rate":
		return "5"
	case "atr":
		return "0.5"
	case "atr_pct":
		return "3"
	case "adx":
		return "25"
	case "dmi_plus", "dmi_minus":
		return "20"
	case "cci":
		return "100"
	case "williams_r":
		return "-80"
	case "mfi":
		return "80"
	case "psy_12":
		return "60"
	case "psy_ma":
		return "50"
	case "drawdown_20":
		return "-10"
	case "new_high_20":
		return "1"
	case "up_days_ratio":
		return "60"
	case "price_position_20", "price_position_60":
		return "70"
	case "gap_pct":
		return "3"
	case "high_low_range":
		return "5"
	case "ma_convergence":
		return "3"
	case "trend_strength":
		return "2"
	case "consecutive_days":
		return "3"
	case "vwap_deviation":
		return "2"
	case "volume_trend":
		return "1"
	case "index_relative":
		return "2"
	case "pe":
		return "20"
	case "pb":
		return "2"
	case "ps":
		return "2"
	case "pe_percentile", "pb_percentile":
		return "30"
	case "roe":
		return "15"
	case "revenue_growth":
		return "10"
	case "profit_growth":
		return "15"
	case "gross_margin":
		return "30"
	case "net_margin":
		return "10"
	case "debt_ratio":
		return "60"
	case "eps":
		return "0.5"
	case "total_market_cap":
		return "100000000000"
	case "shareholder_change":
		return "-5"
	case "inst_hold_ratio":
		return "30"
	case "prediction_upside":
		return "10"
	case "prediction_consensus":
		return "0.6"
	default:
		return "0"
	}
}

// getValueRange returns min/max range hints for number-type indicators.
func getValueRange(key string) (*float64, *float64, bool) {
	f := func(v float64) *float64 { return &v }
	switch key {
	case "algo_score", "ai_score", "ai_fundamental", "ai_technical", "ai_valuation", "ai_growth", "ai_industry", "ai_capital":
		return f(0), f(10), true
	case "rsi", "rsi_6", "rsi_12", "rsi_24":
		return f(0), f(100), true
	case "kdj_k", "kdj_d", "kdj_j":
		return f(0), f(100), true
	case "boll_position":
		return f(0), f(100), true
	case "williams_r":
		return f(-100), f(0), true
	case "mfi":
		return f(0), f(100), true
	case "adx":
		return f(0), f(100), true
	case "dmi_plus", "dmi_minus":
		return f(0), f(100), true
	case "pe_percentile", "pb_percentile":
		return f(0), f(100), true
	case "volume_ratio", "volume_ma_ratio":
		return f(0), f(10), true
	case "turnover_rate":
		return f(0), f(50), true
	case "streak_count", "pick_count_5d":
		return f(0), f(20), true
	case "pick_count_20d":
		return f(0), f(60), true
	case "signal_value":
		return f(-1), f(1), true
	case "daily_change":
		return f(-10), f(10), true
	case "momentum_5":
		return f(-30), f(30), true
	case "momentum_20":
		return f(-50), f(50), true
	case "ma_deviation":
		return f(-20), f(20), true
	case "debt_ratio":
		return f(0), f(100), true
	case "prediction_consensus":
		return f(0), f(1), true
	default:
		return nil, nil, false
	}
}

// buildIndicatorListLegacy closing brace is above. End of legacy function.

// parseValue converts flexible AI values to float64
func parseValue(v interface{}, indicator string, op string) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		// Try parse as number
		f, err := strconv.ParseFloat(val, 64)
		if err == nil { return f }
		// Handle "5/20" format for ma_cross → encode as 5.020
		if indicator == "ma_cross" && strings.Contains(val, "/") {
			parts := strings.Split(val, "/")
			if len(parts) == 2 {
				ma1, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
				ma2, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if ma1 > 0 && ma2 > 0 {
					return ma1 + ma2/1000
				}
			}
		}
		// For cross operators: direction encoded
		if op == "cross_up" { return 1 }
		if op == "cross_down" { return -1 }
		return 0
	default:
		return 0
	}
}

// truncate returns first n chars
func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "..."
}

// ── Helpers ──

func filterConds(conds []model.StrategyCondition, condType string) []model.StrategyCondition {
	var out []model.StrategyCondition
	for _, c := range conds {
		if c.CondType == condType && c.Enabled {
			out = append(out, c)
		}
	}
	return out
}

// evaluateConditions checks if conditions of a type are met for a stock on a date
// Uses OR between logic groups, AND within each group
func evaluateConditions(conds []model.StrategyCondition, code, date string) bool {
	if len(conds) == 0 {
		return false
	}

	// Group conditions by logicGroup
	groups := make(map[int][]model.StrategyCondition)
	for _, c := range conds {
		groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
	}

	for _, groupConds := range groups {
		allMet := true
		for _, c := range groupConds {
			if !evaluateSingleCondition(c, code, date) {
				allMet = false
				break
			}
		}
		if allMet {
			return true
		}
	}
	return false
}


// ── Backtest execution logging helpers ──

// insertBacktestLog inserts a single execution log entry during backtest.
// Use batch insert for better performance when many logs are generated per day.
func insertBacktestLog(taskID, strategyID, userID uint, date string, seq int,
	logType, level, stockCode, stockName, message string, detail interface{}) {
	detailJSON := "{}"
	if detail != nil {
		b, _ := json.Marshal(detail)
		detailJSON = string(b)
	}
	entry := model.BacktestExecutionLog{
		TaskID:     taskID,
		StrategyID: strategyID,
		UserID:     userID,
		Date:       date,
		Seq:        seq,
		LogType:    logType,
		Level:      level,
		StockCode:  stockCode,
		StockName:  stockName,
		Message:    message,
		Detail:     detailJSON,
	}
	if err := db.MySQL.Create(&entry).Error; err != nil {
		log.Printf("[backtest] insertLog failed: %v", err)
	}
}

// insertDailySnapshot records a portfolio snapshot for one trading day.
func insertDailySnapshot(taskID, strategyID, userID uint, date string, dayIndex int,
	cash, totalEquity, dailyReturn, cumulativeReturn, maxDD float64,
	positionCount int, positions []map[string]interface{}) {
	posJSON := "[]"
	if len(positions) > 0 {
		b, _ := json.Marshal(positions)
		posJSON = string(b)
	}
	snap := model.BacktestDailySnapshot{
		TaskID:           taskID,
		StrategyID:       strategyID,
		UserID:           userID,
		Date:             date,
		DayIndex:         dayIndex,
		Cash:             math.Round(cash*100) / 100,
		TotalEquity:      math.Round(totalEquity*100) / 100,
		DailyReturn:      math.Round(dailyReturn*100) / 100,
		CumulativeReturn: math.Round(cumulativeReturn*100) / 100,
		PositionCount:    positionCount,
		Positions:        posJSON,
		MaxDrawdown:      math.Round(maxDD*100) / 100,
	}
	if err := db.MySQL.Create(&snap).Error; err != nil {
		log.Printf("[backtest] insertSnapshot failed: %v", err)
	}
}

func evaluateSingleCondition(cond model.StrategyCondition, code, date string) bool {
	val := getIndicatorValue(cond, code, date)
	switch cond.Operator {
	case "gte":
		return val >= cond.Value
	case "lte":
		return val <= cond.Value
	case "gt":
		return val > cond.Value
	case "lt":
		return val < cond.Value
	case "eq":
		return val == cond.Value
	case "cross_up":
		return val > 0 // 1 means cross up
	case "cross_down":
		return val < 0 // -1 means cross down
	}
	return false
}

func getIndicatorValue(cond model.StrategyCondition, code, date string) float64 {
	switch cond.Indicator {
	// ── 榜单与评分 ──
	case "streak_count":
		return getStreakCount(code, date)
	case "pick_count_5d":
		return getPickCount(code, date, 5)
	case "pick_count_20d":
		return getPickCount(code, date, 20)
	case "algo_score":
		var score float64
		db.PG.Raw("SELECT COALESCE(score,0) FROM algorithm_pick_details WHERE stock_code = ? AND pick_date = ?", code, date).Scan(&score)
		return score
	case "signal_value":
		var sig float64
		db.PG.Raw("SELECT COALESCE(signal_value,0) FROM stock_signals WHERE code = ?", code).Scan(&sig)
		return sig

	// ── AI六维评分 ──
	case "ai_score":
		return getAIScore(code, "composite_score", date)
	case "ai_fundamental":
		return getAIScore(code, "fundamental_score", date)
	case "ai_technical":
		return getAIScore(code, "technical_score", date)
	case "ai_valuation":
		return getAIScore(code, "valuation_score", date)
	case "ai_growth":
		return getAIScore(code, "growth_score", date)
	case "ai_industry":
		return getAIScore(code, "industry_score", date)
	case "ai_capital":
		return getAIScore(code, "capital_score", date)

	// ── 技术面 — 趋势类 ──
	case "daily_change":
		return getMomentum(code, date, 1)
	case "momentum_5":
		return getMomentum(code, date, 5)
	case "momentum_20":
		return getMomentum(code, date, 20)
	case "ma_deviation":
		return getMADeviation(code, date, 20)
	case "ma_5":
		return getSMA(code, date, 5)
	case "ma_10":
		return getSMA(code, date, 10)
	case "ma_20":
		return getSMA(code, date, 20)
	case "ma_30":
		return getSMA(code, date, 30)
	case "ma_60":
		return getSMA(code, date, 60)
	case "ma_cross":
		ma1 := int(cond.Value)
		ma2 := int(math.Round((cond.Value - float64(ma1)) * 1000))
		if ma1 < 1 { ma1 = 5 }
		if ma2 < 1 { ma2 = 20 }
		return checkMACross(code, date, ma1, ma2)
	case "macd":
		return checkMACD(code, date)
	case "macd_dif":
		return getMACDDIF(code, date)
	case "macd_dea":
		return getMACDDEA(code, date)

	// ── 技术面 — 超买超卖 ──
	case "rsi":
		return getRSI(code, date, 14)
	case "rsi_6":
		return getRSI(code, date, 6)
	case "rsi_12":
		return getRSI(code, date, 12)
	case "rsi_24":
		return getRSI(code, date, 24)
	case "kdj_k":
		k, _, _ := getKDJ(code, date)
		return k
	case "kdj_d":
		_, d, _ := getKDJ(code, date)
		return d
	case "kdj_j":
		_, _, j := getKDJ(code, date)
		return j
	case "boll_position":
		return getBollPosition(code, date)
	case "boll_width":
		return getBollWidth(code, date)
	case "boll_upper":
		return getBollUpper(code, date)
	case "boll_middle":
		return getSMA(code, date, 20) // Bollinger middle = MA20
	case "boll_lower":
		return getBollLower(code, date)

	// ── 技术面 — 量价 ──
	case "volume_ratio":
		return getVolumeRatio(code, date, 5)
	case "volume_ma_ratio":
		return getVolumeRatio(code, date, 20)
	case "psy_12":
		return getPSY(code, date, 12)
	case "psy_ma":
		return getPSYMA(code, date)
	case "turnover_rate":
		return getTurnoverRate(code, date)
	case "atr":
		return getATR(code, date, 14)

	// ── 技术面 — 形态与强度 ──
	case "drawdown_20":
		return getMaxDrawdown(code, date, 20)
	case "new_high_20":
		return getNewHigh(code, date, 20)
	case "up_days_ratio":
		return getUpDaysRatio(code, date, 20)
	case "price_position_20":
		return getPricePosition(code, date, 20)
	case "price_position_60":
		return getPricePosition(code, date, 60)

	// ── 估值 ──
	case "pe":
		return getIndicator(code, date, "pe")
	case "pb":
		return getIndicator(code, date, "pb")
	case "ps":
		return getIndicator(code, date, "ps")
	case "pe_percentile":
		return getPEPercentile(code, date)
	case "pb_percentile":
		return getPBPercentile(code, date)

	// ── 基本面 ──
	case "roe":
		return getFinancialMetric(code, date, "roe")
	case "revenue_growth":
		return getFinancialMetric(code, date, "revenue_growth")
	case "profit_growth":
		return getFinancialMetric(code, date, "profit_growth")
	case "gross_margin":
		return getFinancialMetric(code, date, "gross_margin")
	case "net_margin":
		return getFinancialMetric(code, date, "net_margin")
	case "debt_ratio":
		return getFinancialMetric(code, date, "debt_ratio")
	case "eps":
		return getFinancialMetric(code, date, "eps")

	// ── 资金面/市场面 ──
	case "total_market_cap":
		return getIndicator(code, date, "total_market_cap")
	case "shareholder_change":
		var chg float64
		db.PG.Raw("SELECT COALESCE(holder_change,0) FROM stock_shareholders WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", code, date).Scan(&chg)
		return chg
	case "inst_hold_ratio":
		var ratio float64
		db.PG.Raw("SELECT COALESCE(inst_hold_ratio,0) FROM stock_shareholders WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", code, date).Scan(&ratio)
		return ratio

	// ── 技术面 — 进阶：趋势系统 ──
	case "adx":
		return getADX(code, date, 14)
	case "dmi_plus":
		p, _, _ := getDMI(code, date, 14)
		return p
	case "dmi_minus":
		_, m, _ := getDMI(code, date, 14)
		return m
	case "ema_cross":
		ma1 := int(cond.Value)
		ma2 := int(math.Round((cond.Value - float64(ma1)) * 1000))
		if ma1 < 1 { ma1 = 12 }
		if ma2 < 1 { ma2 = 26 }
		return checkEMACross(code, date, ma1, ma2)

	// ── 技术面 — 进阶：超买超卖扩展 ──
	case "cci":
		return getCCI(code, date, 20)
	case "williams_r":
		return getWilliamsR(code, date, 14)
	case "mfi":
		return getMFI(code, date, 14)

	// ── 技术面 — 进阶：波动与结构 ──
	case "boll_squeeze":
		return getBollSqueeze(code, date, 100)
	case "atr_pct":
		return getATRPct(code, date, 14)
	case "ma_convergence":
		return getMAConvergence(code, date)
	case "trend_strength":
		return getTrendStrength(code, date, 20)

	// ── 技术面 — 进阶：形态与量价 ──
	case "consecutive_days":
		return getConsecutiveDays(code, date)
	case "gap_pct":
		return getGapPct(code, date)
	case "high_low_range":
		return getHighLowRange(code, date)
	case "vwap_deviation":
		return getVWAPDeviation(code, date)
	case "volume_trend":
		return getVolumeTrend(code, date)
	case "index_relative":
		return getIndexRelative(code, date, 20)

	// ── 预测 ──
	case "prediction_upside":
		return getPredictionUpside(code, date)
	case "prediction_consensus":
		return getPredictionConsensus(code, date)
	}
	return 0
}

// ═══════════════════════════════════════════════════════════════
// 通用辅助函数
// ═══════════════════════════════════════════════════════════════

// getPickCount returns how many of the last N pick-dates the stock appeared in.
func getPickCount(code, date string, days int) float64 {
	var count int
	db.PG.Raw(`
		WITH recent_pick_dates AS (
			SELECT DISTINCT pick_date FROM algorithm_pick_details
			WHERE pick_date <= ?::date
			ORDER BY pick_date DESC
			LIMIT ?
		)
		SELECT COUNT(DISTINCT apd.pick_date)
		FROM algorithm_pick_details apd
		JOIN recent_pick_dates rpd ON apd.pick_date = rpd.pick_date
		WHERE apd.stock_code = ?
	`, date, days, code).Scan(&count)
	return float64(count)
}

func getStreakCount(code, date string) float64 {
	// Count consecutive trading days the stock appeared in algorithm picks
	// ending on or before the given date (actual consecutive streak)
	var streak int
	db.PG.Raw(`
		WITH dates AS (
			SELECT DISTINCT apd.pick_date 
			FROM algorithm_pick_details apd
			JOIN algorithm_picks ap ON ap.pick_date = apd.pick_date
			WHERE apd.stock_code = ? AND apd.pick_date <= ?::date
			ORDER BY pick_date DESC
		),
		consecutive AS (
			SELECT pick_date,
				pick_date - (ROW_NUMBER() OVER (ORDER BY pick_date DESC))::int as grp
			FROM dates
		)
		SELECT COUNT(*) FROM consecutive WHERE grp = (
			SELECT grp FROM consecutive ORDER BY pick_date DESC LIMIT 1
		)
	`, code, date).Scan(&streak)
	return float64(streak)
}

func getClosePrice(code, date string) float64 {
	var close float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&close)
	return close
}

func getClosePriceOn(code, date string) float64 {
	var close float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?::date LIMIT 1", code, date).Scan(&close)
	return close
}

func getAIScore(code, field, date string) float64 {
	var score float64
	db.PG.Raw(fmt.Sprintf("SELECT COALESCE(%s,0) FROM ai_stock_scores WHERE code = ? AND created_at <= ?::date ORDER BY created_at DESC LIMIT 1", field), code, date).Scan(&score)
	return score
}

func getIndicator(code, date, field string) float64 {
	var val float64
	// Use most recent indicator data available on or before the backtest date
	db.PG.Raw(fmt.Sprintf("SELECT COALESCE(%s,0) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1", field), code, date).Scan(&val)
	return val
}

// hasIndicatorData checks if any indicator data exists for this stock up to the given date
func hasIndicatorData(code, date string) bool {
	var count int
	db.PG.Raw("SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date", code, date).Scan(&count)
	return count > 0
}

func getFinancialMetric(code, date, field string) float64 {
	var val float64
	// Use latest financial report available on or before the backtest date
	db.PG.Raw(fmt.Sprintf("SELECT COALESCE(%s,0) FROM stock_financials WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", field), code, date).Scan(&val)
	return val
}

// hasFinancialData checks if any financial data exists for this stock up to given date
func hasFinancialData(code, date string) bool {
	var count int
	db.PG.Raw("SELECT COUNT(*) FROM stock_financials WHERE code = ? AND report_date <= ?", code, date).Scan(&count)
	return count > 0
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 趋势类
// ═══════════════════════════════════════════════════════════════

func getMomentum(code, date string, days int) float64 {
	var chg float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) /
		NULLIF((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1 OFFSET ?), 0) - 1, 0) * 100`,
		code, date, code, date, days).Scan(&chg)
	return chg
}

// getSMA returns simple moving average for fallback (cache miss).
func getSMA(code, date string, days int) float64 {
	var ma float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT AVG(close) FROM (
			SELECT close FROM stocks_daily_k 
			WHERE code = ? AND trade_date <= ?::date 
			ORDER BY trade_date DESC LIMIT ?
		) sub), 0)
	`, code, date, days).Scan(&ma)
	return ma
}

func getMADeviation(code, date string, maPeriod int) float64 {
	var dev float64
	db.PG.Raw(`SELECT COALESCE(
		((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) /
		NULLIF((SELECT AVG(close) FROM (SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub), 0) - 1) * 100, 0)`,
		code, date, code, date, maPeriod).Scan(&dev)
	return dev
}

func checkMACross(code, date string, ma1, ma2 int) float64 {
	var cross int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ma AS (
		SELECT trade_date,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ma_short,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ma_long
		FROM klines
	)
	SELECT CASE
		WHEN ma_short > ma_long AND LAG(ma_short) OVER (ORDER BY trade_date ASC) <= LAG(ma_long) OVER (ORDER BY trade_date ASC) THEN 1
		WHEN ma_short < ma_long AND LAG(ma_short) OVER (ORDER BY trade_date ASC) >= LAG(ma_long) OVER (ORDER BY trade_date ASC) THEN -1
		ELSE 0 END
	FROM ma ORDER BY trade_date DESC LIMIT 1`, code, date, ma1, ma2).Scan(&cross)
	return float64(cross)
}

func checkMACD(code, date string) float64 {
	var cross int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close,
			row_number() OVER (ORDER BY trade_date ASC) as rn
		FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ema_calc AS (
		SELECT trade_date, close, rn,
			close AS ema12, close AS ema26
		FROM klines WHERE rn = 1
		UNION ALL
		SELECT k.trade_date, k.close, k.rn,
			0.1538 * k.close + 0.8462 * e.ema12,
			0.0741 * k.close + 0.9259 * e.ema26
		FROM ema_calc e
		JOIN klines k ON k.rn = e.rn + 1
	), macd AS (
		SELECT trade_date,
			ema12 - ema26 as dif,
			0.2 * (ema12 - ema26) + 0.8 * COALESCE(LAG(ema12 - ema26) OVER (ORDER BY trade_date ASC), ema12 - ema26) as dea
		FROM ema_calc
	)
	SELECT CASE
		WHEN dif > dea AND LAG(dif) OVER (ORDER BY trade_date ASC) <= LAG(dea) OVER (ORDER BY trade_date ASC) THEN 1
		WHEN dif < dea AND LAG(dif) OVER (ORDER BY trade_date ASC) >= LAG(dea) OVER (ORDER BY trade_date ASC) THEN -1
		ELSE 0 END
	FROM macd ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&cross)
	return float64(cross)
}

func getMACDDIF(code, date string) float64 {
	var dif float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close,
			row_number() OVER (ORDER BY trade_date ASC) as rn
		FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ema_calc AS (
		SELECT trade_date, close, rn,
			close AS ema12, close AS ema26
		FROM klines WHERE rn = 1
		UNION ALL
		SELECT k.trade_date, k.close, k.rn,
			0.1538 * k.close + 0.8462 * e.ema12,
			0.0741 * k.close + 0.9259 * e.ema26
		FROM ema_calc e
		JOIN klines k ON k.rn = e.rn + 1
	)
	SELECT COALESCE(ema12 - ema26, 0) FROM ema_calc ORDER BY trade_date DESC LIMIT 1
	`, code, date).Scan(&dif)
	return dif
}

func getMACDDEA(code, date string) float64 {
	var dea float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close,
			row_number() OVER (ORDER BY trade_date ASC) as rn
		FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ema_calc AS (
		SELECT trade_date, close, rn,
			close AS ema12, close AS ema26
		FROM klines WHERE rn = 1
		UNION ALL
		SELECT k.trade_date, k.close, k.rn,
			0.1538 * k.close + 0.8462 * e.ema12,
			0.0741 * k.close + 0.9259 * e.ema26
		FROM ema_calc e
		JOIN klines k ON k.rn = e.rn + 1
	), macd AS (
		SELECT trade_date,
			ema12 - ema26 as dif,
			0.2 * (ema12 - ema26) + 0.8 * COALESCE(LAG(ema12 - ema26) OVER (ORDER BY trade_date ASC), ema12 - ema26) as dea
		FROM ema_calc
	)
	SELECT COALESCE(dea, 0) FROM macd ORDER BY trade_date DESC LIMIT 1
	`, code, date).Scan(&dea)
	return dea
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 超买超卖
// ═══════════════════════════════════════════════════════════════

func getRSI(code, date string, period int) float64 {
	var rsi float64
	db.PG.Raw(`SELECT COALESCE(
		100 - 100 / (1 + (avg_gain / NULLIF(avg_loss,0))), 50)
	FROM (
		SELECT
			AVG(CASE WHEN chg > 0 THEN chg ELSE 0 END) as avg_gain,
			AVG(CASE WHEN chg < 0 THEN -chg ELSE 0 END) as avg_loss
		FROM (
			SELECT close - LAG(close) OVER (ORDER BY trade_date) as chg
			FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date
			ORDER BY trade_date DESC LIMIT ?
		) changes
	) avgs`, code, date, period+1).Scan(&rsi)
	return rsi
}

func getKDJ(code, date string) (k, d, j float64) {
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), rsv_calc AS (
		SELECT trade_date, close,
			MIN(low) OVER (ORDER BY trade_date ASC ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) as l9,
			MAX(high) OVER (ORDER BY trade_date ASC ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) as h9
		FROM klines
	), k_calc AS (
		SELECT trade_date,
			(close - l9) / NULLIF((h9 - l9), 0) * 100 as rsv,
			AVG((close - l9) / NULLIF((h9 - l9), 0) * 100) OVER (ORDER BY trade_date ASC ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) as k
		FROM rsv_calc
	), d_calc AS (
		SELECT trade_date, k,
			AVG(k) OVER (ORDER BY trade_date ASC ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) as d
		FROM k_calc
	)
	SELECT COALESCE(k,50), COALESCE(d,50)
	FROM d_calc ORDER BY trade_date DESC LIMIT 1`, code, date).Row().Scan(&k, &d)
	if k > 0 || d > 0 {
		j = 3*k - 2*d
	}
	return
}

func getBollPosition(code, date string) float64 {
	var pos float64
	db.PG.Raw(`SELECT COALESCE(
		(cl - (mid - 2*std)) / NULLIF((mid + 2*std) - (mid - 2*std), 0) * 100, 50)
	FROM (
		SELECT close as cl,
			AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
			STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date
		ORDER BY trade_date DESC LIMIT 1
	) boll`, code, date).Scan(&pos)
	return pos
}

// getBollUpper returns Bollinger upper band for fallback.
func getBollUpper(code, date string) float64 {
	var val float64
	db.PG.Raw(`SELECT (mid + 2 * COALESCE(stddev, 0)) FROM (
		SELECT AVG(close) as mid, STDDEV_SAMP(close) as stddev FROM (
			SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 20
		) sub
	) bb`, code, date).Scan(&val)
	return val
}

// getBollLower returns Bollinger lower band for fallback.
func getBollLower(code, date string) float64 {
	var val float64
	db.PG.Raw(`SELECT (mid - 2 * COALESCE(stddev, 0)) FROM (
		SELECT AVG(close) as mid, STDDEV_SAMP(close) as stddev FROM (
			SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 20
		) sub
	) bb`, code, date).Scan(&val)
	return val
}

func getBollWidth(code, date string) float64 {
	var w float64
	db.PG.Raw(`SELECT COALESCE(
		((mid + 2*std) - (mid - 2*std)) / NULLIF(mid, 0) * 100, 0)
	FROM (
		SELECT close as cl,
			AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
			STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date
		ORDER BY trade_date DESC LIMIT 1
	) boll`, code, date).Scan(&w)
	return w
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 量价
// ═══════════════════════════════════════════════════════════════

func getVolumeRatio(code, date string, days int) float64 {
	var vr float64
	db.PG.Raw(`SELECT COALESCE(
		vol::float / NULLIF(avg_vol, 0), 0)
	FROM (
		SELECT 
			(SELECT volume FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) as vol,
			(SELECT AVG(volume) FROM (SELECT volume FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub) as avg_vol
	) t`, code, date, code, date, days).Scan(&vr)
	return vr
}

func getTurnoverRate(code, date string) float64 {
	var tr float64
	db.PG.Raw("SELECT COALESCE(turnover_rate, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&tr)
	return tr
}

func getATR(code, date string, period int) float64 {
	var atr float64
	db.PG.Raw(`SELECT COALESCE(AVG(tr),0) FROM (
		SELECT GREATEST(high-low, ABS(high-LAG(close) OVER (ORDER BY trade_date)), ABS(low-LAG(close) OVER (ORDER BY trade_date))) as tr
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?
	) sub`, code, date, period+1).Scan(&atr)
	return atr
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 形态与强度
// ═══════════════════════════════════════════════════════════════

// getMaxDrawdown returns the max drawdown as a positive magnitude (percentage).
// e.g., 7.6 means a 7.6% drawdown from the N-day high. Use operator "gt" to compare.
func getMaxDrawdown(code, date string, days int) float64 {
	var dd float64
	db.PG.Raw(`SELECT COALESCE(ABS(MIN(drawdown)), 0) FROM (
		SELECT (close - MAX(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW))
			/ NULLIF(MAX(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW), 0) * 100 as drawdown
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC LIMIT ?
	) sub`, code, date, days).Scan(&dd)
	return dd
}

func getNewHigh(code, date string, days int) float64 {
	var isHigh int
	db.PG.Raw(`SELECT CASE WHEN
		(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) >=
		(SELECT COALESCE(MAX(close),0) FROM (SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub)
		THEN 1 ELSE 0 END`, code, date, code, date, days).Scan(&isHigh)
	return float64(isHigh)
}

func getUpDaysRatio(code, date string, days int) float64 {
	var ratio float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM (SELECT close, open FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub WHERE close > open)::float / ?, 0)`,
		code, date, days, float64(days)).Scan(&ratio)
	return ratio
}

func getPricePosition(code, date string, days int) float64 {
	var pos float64
	db.PG.Raw(`SELECT COALESCE(
		(cl - low_n) / NULLIF(high_n - low_n, 0) * 100, 50)
	FROM (
		SELECT
			(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) as cl,
			(SELECT MIN(low) FROM (SELECT low FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub) as low_n,
			(SELECT MAX(high) FROM (SELECT high FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub) as high_n
	) pos_calc`, code, date, code, date, days, code, date, days).Scan(&pos)
	return pos
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：趋势系统 (ADX/DMI/EMA交叉)
// ═══════════════════════════════════════════════════════════════

func getADX(code, date string, period int) float64 {
	var adx float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), tr_calc AS (
		SELECT trade_date,
			high - LAG(high) OVER (ORDER BY trade_date ASC) as up_move,
			LAG(low) OVER (ORDER BY trade_date ASC) - low as down_move,
			GREATEST(high-low,
				ABS(high-LAG(close) OVER (ORDER BY trade_date ASC)),
				ABS(low-LAG(close) OVER (ORDER BY trade_date ASC))
			) as tr
		FROM klines
	), dmi AS (
		SELECT trade_date,
			AVG(CASE WHEN up_move > 0 AND up_move > down_move THEN up_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as pdi,
			AVG(CASE WHEN down_move > 0 AND down_move > up_move THEN down_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as mdi
		FROM tr_calc
	)
	SELECT COALESCE(ABS(pdi-mdi)/NULLIF(pdi+mdi,0)*100, 0)
	FROM dmi ORDER BY trade_date DESC LIMIT 1`, code, date, period, period, period, period).Scan(&adx)
	return adx
}

func getDMI(code, date string, period int) (pdi, mdi, adx float64) {
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), tr_calc AS (
		SELECT trade_date,
			high - LAG(high) OVER (ORDER BY trade_date ASC) as up_move,
			LAG(low) OVER (ORDER BY trade_date ASC) - low as down_move,
			GREATEST(high-low,
				ABS(high-LAG(close) OVER (ORDER BY trade_date ASC)),
				ABS(low-LAG(close) OVER (ORDER BY trade_date ASC))
			) as tr
		FROM klines
	), dmi AS (
		SELECT trade_date,
			AVG(CASE WHEN up_move > 0 AND up_move > down_move THEN up_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as pdi,
			AVG(CASE WHEN down_move > 0 AND down_move > up_move THEN down_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as mdi
		FROM tr_calc
	)
	SELECT COALESCE(pdi, 25), COALESCE(mdi, 25),
		COALESCE(ABS(pdi-mdi)/NULLIF(pdi+mdi,0)*100, 0)
	FROM dmi ORDER BY trade_date DESC LIMIT 1`, code, date, period, period, period, period).Row().Scan(&pdi, &mdi, &adx)
	return
}

func checkEMACross(code, date string, ma1, ma2 int) float64 {
	var cross int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ema AS (
		SELECT trade_date,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ema1,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ema2
		FROM klines
	)
	SELECT CASE
		WHEN ema1 > ema2 AND LAG(ema1) OVER (ORDER BY trade_date ASC) <= LAG(ema2) OVER (ORDER BY trade_date ASC) THEN 1
		WHEN ema1 < ema2 AND LAG(ema1) OVER (ORDER BY trade_date ASC) >= LAG(ema2) OVER (ORDER BY trade_date ASC) THEN -1
		ELSE 0 END
	FROM ema ORDER BY trade_date DESC LIMIT 1`, code, date, ma1, ma2).Scan(&cross)
	return float64(cross)
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：超买超卖扩展 (CCI/Williams%R/MFI)
// ═══════════════════════════════════════════════════════════════

// getPSY computes N-day psychological line (% of up days in last N days).
// Uses proper rolling window — up days / N * 100 for the most recent period days.
func getPSY(code, date string, period int) float64 {
	var psy float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close,
			LAG(close) OVER (ORDER BY trade_date ASC) as prev_close
		FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date
		ORDER BY trade_date ASC
	)
	SELECT COALESCE(
		SUM(CASE WHEN close > prev_close THEN 1 ELSE 0 END)::float /
		NULLIF(COUNT(*) - 1, 0) * 100, 50)
	FROM (SELECT * FROM klines WHERE prev_close IS NOT NULL ORDER BY trade_date DESC LIMIT ?) sub`,
		code, date, period).Scan(&psy)
	return psy
}

// getPSYMA computes 6-day SMA of PSY(12).
// Computes PSY(12) rolling window first, then averages the last 6 PSY values.
func getPSYMA(code, date string) float64 {
	var psyma float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close,
			LAG(close) OVER (ORDER BY trade_date ASC) as prev_close
		FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date
		ORDER BY trade_date ASC
	), psy_calc AS (
		SELECT trade_date,
			SUM(CASE WHEN close > prev_close THEN 1 ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN 11 PRECEDING AND CURRENT ROW) * 100.0 / 12 as psy
		FROM klines WHERE prev_close IS NOT NULL
	)
	SELECT COALESCE(AVG(psy), 50) FROM (
		SELECT psy FROM psy_calc ORDER BY trade_date DESC LIMIT 6
	) sub`, code, date).Scan(&psyma)
	return psyma
}

func getCCI(code, date string, period int) float64 {
	var cci float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), tp_dev AS (
		SELECT trade_date,
			(high + low + close) / 3 as tp,
			AVG((high + low + close) / 3) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as avg_tp
		FROM klines
	), dev_calc AS (
		SELECT trade_date, tp, avg_tp,
			AVG(ABS(tp - avg_tp)) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as avg_dev
		FROM tp_dev
	)
	SELECT COALESCE((tp - avg_tp) / NULLIF(0.015 * avg_dev, 0), 0)
	FROM dev_calc ORDER BY trade_date DESC LIMIT 1`, code, date, period, period).Scan(&cci)
	return cci
}

func getWilliamsR(code, date string, period int) float64 {
	var wr float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), wr_calc AS (
		SELECT trade_date,
			MAX(high) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as hn,
			MIN(low) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ln,
			close
		FROM klines
	)
	SELECT COALESCE(-(hn - close) / NULLIF(hn - ln, 0) * 100, -50)
	FROM wr_calc ORDER BY trade_date DESC LIMIT 1`, code, date, period, period).Scan(&wr)
	return wr
}

func getMFI(code, date string, period int) float64 {
	var mfi float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close, volume FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), mf_calc AS (
		SELECT trade_date,
			(high + low + close) / 3.0 as tp,
			((high + low + close) / 3.0 - LAG((high + low + close) / 3.0) OVER (ORDER BY trade_date ASC)) * volume as mf
		FROM klines
	)
	SELECT COALESCE(100.0 - 100.0 / NULLIF(1.0 + pos_mf / NULLIF(neg_mf, 0), 0), 50.0)
	FROM (
		SELECT
			SUM(CASE WHEN mf > 0 THEN mf ELSE 0 END) as pos_mf,
			SUM(CASE WHEN mf < 0 THEN -mf ELSE 0 END) as neg_mf
		FROM (SELECT mf FROM mf_calc ORDER BY trade_date DESC LIMIT ?) recent
	) mfi_calc`, code, date, period).Scan(&mfi)
	return mfi
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：波动与结构
// ═══════════════════════════════════════════════════════════════

func getBollSqueeze(code, date string, lookback int) float64 {
	var squeeze float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM (
			SELECT (mid + 2*std - (mid - 2*std)) / NULLIF(mid, 0) as bw
			FROM (
				SELECT close,
					AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
					STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
				FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?
			) boll WHERE mid > 0
		) hist WHERE bw > (
			SELECT (mid + 2*std - (mid - 2*std)) / NULLIF(mid, 0)
			FROM (
				SELECT close,
					AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
					STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
				FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1
			) curr WHERE mid > 0
		))::float / NULLIF(?, 0) * 100, 50)`,
		code, date, lookback+20, code, date, float64(lookback+20)).Scan(&squeeze)
	return squeeze
}

func getATRPct(code, date string, period int) float64 {
	atr := getATR(code, date, period)
	price := getClosePrice(code, date)
	if price > 0 {
		return atr / price * 100
	}
	return 0
}

func getMAConvergence(code, date string) float64 {
	var cv float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), mas AS (
		SELECT
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) as ma5,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 9 PRECEDING AND CURRENT ROW) as ma10,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) as ma60
		FROM klines
		ORDER BY trade_date DESC LIMIT 1
	), stats AS (
		SELECT ma5, ma10, ma20, ma60,
			(ma5 + ma10 + ma20 + ma60) / 4.0 as avg_ma,
			(ma5 + ma10 + ma20 + ma60) / 4.0 as mean_val
		FROM mas
	)
	SELECT COALESCE(
		SQRT(
			(POWER(ma5 - mean_val, 2) + POWER(ma10 - mean_val, 2) + 
			 POWER(ma20 - mean_val, 2) + POWER(ma60 - mean_val, 2)) / 3.0
		) / NULLIF(avg_ma, 0) * 100, 100)
	FROM stats`, code, date).Scan(&cv)
	return cv
}

func getTrendStrength(code, date string, days int) float64 {
	var strength float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ma AS (
		SELECT trade_date, close,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20
		FROM klines
	)
	SELECT COALESCE(
		SUM(CASE WHEN close > ma20 THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0.5)
	FROM (SELECT close, ma20 FROM ma ORDER BY trade_date DESC LIMIT ?) sub`, code, date, days).Scan(&strength)
	return strength
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：形态与量价
// ═══════════════════════════════════════════════════════════════

func getConsecutiveDays(code, date string) float64 {
	var days int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ranked AS (
		SELECT close, LAG(close) OVER (ORDER BY trade_date ASC) as prev_close, trade_date
		FROM klines
	), grouped AS (
		SELECT *, SUM(CASE WHEN (close > prev_close) != (LAG(close) OVER (ORDER BY trade_date ASC) > LAG(prev_close) OVER (ORDER BY trade_date ASC)) THEN 1 ELSE 0 END) OVER (ORDER BY trade_date ASC) as grp
		FROM ranked WHERE prev_close IS NOT NULL
	)
	SELECT COUNT(*) * CASE
		WHEN (SELECT close > prev_close FROM grouped ORDER BY trade_date DESC LIMIT 1) THEN 1 ELSE -1 END
	FROM (SELECT * FROM grouped ORDER BY trade_date DESC LIMIT 20) g WHERE grp = 0`, code, date).Scan(&days)
	return float64(days)
}

func getGapPct(code, date string) float64 {
	var gap float64
	db.PG.Raw(`SELECT COALESCE(
		(open - LAG(close) OVER (ORDER BY trade_date)) / NULLIF(LAG(close) OVER (ORDER BY trade_date), 0) * 100, 0)
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&gap)
	return gap
}

func getHighLowRange(code, date string) float64 {
	var rng float64
	db.PG.Raw(`SELECT COALESCE(
		(high - low) / NULLIF(LAG(close) OVER (ORDER BY trade_date), 0) * 100, 0)
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&rng)
	return rng
}

func getVWAPDeviation(code, date string) float64 {
	var dev float64
	db.PG.Raw(`SELECT COALESCE(
		(close - vwap) / NULLIF(vwap, 0) * 100, 0)
	FROM (
		SELECT close, SUM(close * volume) / NULLIF(SUM(volume), 0) as vwap
		FROM stocks_daily_k WHERE code = ? AND trade_date = ?::date
	) vwap_calc`, code, date).Scan(&dev)
	return dev
}

func getVolumeTrend(code, date string) float64 {
	var trend float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, volume FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), vol AS (
		SELECT trade_date,
			AVG(volume) OVER (ORDER BY trade_date ASC ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) as vol_ma5,
			AVG(volume) OVER (ORDER BY trade_date ASC ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as vol_ma20
		FROM klines
	)
	SELECT COALESCE(vol_ma5 / NULLIF(vol_ma20, 0) - 1, 0)
	FROM vol ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&trend)
	return trend
}

func getIndexRelative(code, date string, days int) float64 {
	// Compare stock return vs Shanghai index (000001) return
	var rel float64
	db.PG.Raw(`SELECT COALESCE(
		(s1.close / NULLIF(s2.close, 0) - i1.close / NULLIF(i2.close, 0)) * 100, 0)
	FROM
		(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) s1,
		(SELECT close FROM (SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub) s2,
		(SELECT close FROM stocks_daily_k WHERE code = '000001' AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) i1,
		(SELECT close FROM (SELECT close FROM stocks_daily_k WHERE code = '000001' AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub) i2`,
		code, date, code, date, days, date, date, days).Scan(&rel)
	return rel
}


// ═══════════════════════════════════════════════════════════════
// 估值
// ═══════════════════════════════════════════════════════════════

func getPEPercentile(code, date string) float64 {
	var pct float64
	// Compute PE percentile using only data available up to the backtest date
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pe > 0 AND pe < COALESCE((SELECT pe FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1), 0))::float /
		NULLIF((SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pe > 0), 0) * 100, 50)`,
		code, date, code, date, code, date).Scan(&pct)
	return pct
}

func getPBPercentile(code, date string) float64 {
	var pct float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pb > 0 AND pb < COALESCE((SELECT pb FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1), 0))::float /
		NULLIF((SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pb > 0), 0) * 100, 50)`,
		code, date, code, date, code, date).Scan(&pct)
	return pct
}

// ═══════════════════════════════════════════════════════════════
// 预测
// ═══════════════════════════════════════════════════════════════

func getPredictionUpside(code, date string) float64 {
	var upside float64
	price := getClosePrice(code, date)
	if price <= 0 { return 0 }
	// Predictions are forward-looking; only use if predict_date > backtest_date
	db.PG.Raw(`SELECT COALESCE((AVG(predicted_price) - ?) / ? * 100, 0)
		FROM predictions WHERE code = ? AND predict_date > ?::date
		AND created_at = (SELECT MAX(created_at) FROM predictions WHERE code = ? AND predict_date > ?::date)`,
		price, price, code, date, code, date).Scan(&upside)
	return upside
}

func getPredictionConsensus(code, date string) float64 {
	var consensus float64
	price := getClosePrice(code, date)
	if price <= 0 { return 0 }
	db.PG.Raw(`SELECT COALESCE(
		SUM(CASE WHEN predicted_price > ? THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0)
		FROM predictions WHERE code = ? AND predict_date > ?::date
		AND created_at = (SELECT MAX(created_at) FROM predictions WHERE code = ? AND predict_date > ?::date)`,
		price, code, date, code, date).Scan(&consensus)
	return consensus
}

func cleanJSON(s string) string {
	// Trim whitespace and newlines
	s = trimSpace(s)
	// Remove markdown fences
	s = trimPrefixes(s, "```json", "```")
	s = trimSuffixes(s, "```")
	s = trimSpace(s)
	// If still not starting with {, find the first { and last }
	if len(s) == 0 || s[0] != '{' {
		start := findChar(s, '{')
		end := findLastChar(s, '}')
		if start >= 0 && end > start {
			s = s[start : end+1]
		}
	}
	return s
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func findChar(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c { return i }
	}
	return -1
}

func findLastChar(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c { return i }
	}
	return -1
}

func trimPrefixes(s string, prefixes ...string) string {
	for _, p := range prefixes {
		if len(s) > len(p) && s[:len(p)] == p {
			s = s[len(p):]
		}
	}
	return s
}

func trimSuffixes(s string, suffixes ...string) string {
	for _, sf := range suffixes {
		if len(s) > len(sf) && s[len(s)-len(sf):] == sf {
			s = s[:len(s)-len(sf)]
		}
	}
	return s
}

// ── Test Indicator ──

type TestIndicatorReq struct {
	StockCode string      `json:"stockCode"`
	Date      string      `json:"date"`
	Indicator string      `json:"indicator"`
	Operator  string      `json:"operator"`
	Value     json.Number `json:"value"`
}

type TestIndicatorResp struct {
	StockCode      string  `json:"stockCode"`
	StockName      string  `json:"stockName"`
	Date           string  `json:"date"`
	Indicator      string  `json:"indicator"`
	IndicatorLabel string  `json:"indicatorLabel"`
	Operator       string  `json:"operator"`
	OperatorLabel  string  `json:"operatorLabel"`
	Threshold      float64 `json:"threshold"`
	ComputedValue  float64 `json:"computedValue"`
	ConditionMet   bool    `json:"conditionMet"`
	DataSource     string  `json:"dataSource"`
	DataNote       string  `json:"dataNote"`
	Error          string  `json:"error,omitempty"`
	HasData        bool    `json:"hasData"`
}

// ── Indicator Management (Enable/Disable by strategy) ──

// StrategyIndicatorView represents an indicator with its enable status per condType.
type StrategyIndicatorView struct {
	Key          string            `json:"key"`
	Label        string            `json:"label"`
	Category     string            `json:"category"`
	Unit         string            `json:"unit"`
	Type         string            `json:"type"`
	Desc         string            `json:"desc"`
	BacktestSafe bool              `json:"backtestSafe"`
	DataNote     string            `json:"dataNote"`
	Suggestion   string            `json:"suggestion"`
	Enabled      map[string]bool   `json:"enabled"` // condType -> enabled
	Conditions   []CondSummary     `json:"conditions"` // existing conditions for this indicator
}

// CondSummary is a lightweight view of a condition.
type CondSummary struct {
	ID         uint    `json:"id"`
	CondType   string  `json:"condType"`
	Operator   string  `json:"operator"`
	Value      float64 `json:"value"`
	Enabled    bool    `json:"enabled"`
	LogicGroup int     `json:"logicGroup"`
}

// ListStrategyIndicators returns all indicators with enable status for the strategy.
func (h *StrategyHandler) ListStrategyIndicators(c *gin.Context) {
	uid := getUID(c)
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "策略ID错误")
		return
	}

	// Verify ownership
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}

	// Load existing conditions
	var conds []model.StrategyCondition
	db.MySQL.Where("strategy_id = ?", sid).Find(&conds)

	// Build condition index: indicator -> condType -> enabled
	condByIndicator := make(map[string]map[string][]CondSummary)
	for _, c := range conds {
		if _, ok := condByIndicator[c.Indicator]; !ok {
			condByIndicator[c.Indicator] = make(map[string][]CondSummary)
		}
		condByIndicator[c.Indicator][c.CondType] = append(condByIndicator[c.Indicator][c.CondType], CondSummary{
			ID:         c.ID,
			CondType:   c.CondType,
			Operator:   c.Operator,
			Value:      c.Value,
			Enabled:    c.Enabled,
			LogicGroup: c.LogicGroup,
		})
	}

	// Build response: all registered indicators with their status
	result := make([]StrategyIndicatorView, 0, len(IndicatorRegistry))
	for _, m := range IndicatorRegistry {
		enabled := make(map[string]bool)
		var summaries []CondSummary

		if condMap, ok := condByIndicator[m.Key]; ok {
			for condType, summaries_ := range condMap {
				allEnabled := false
				for _, s_ := range summaries_ {
					if s_.Enabled {
						allEnabled = true
						break
					}
				}
				enabled[condType] = allEnabled
				summaries = append(summaries, summaries_...)
			}
		}

		result = append(result, StrategyIndicatorView{
			Key:          m.Key,
			Label:        m.Label,
			Category:     m.Category,
			Unit:         m.Unit,
			Type:         m.Type,
			Desc:         m.Desc,
			BacktestSafe: m.BacktestSafe,
			DataNote:     m.DataNote,
			Suggestion:   m.Suggestion,
			Enabled:      enabled,
			Conditions:   summaries,
		})
	}

	response.Success(c, result)
}

// ToggleIndicatorCondition toggles the enabled state of a specific condition.
type ToggleIndicatorReq struct {
	CondID  uint `json:"condId"`  // toggle specific condition
	Enabled *bool `json:"enabled"` // nil = toggle, true = enable, false = disable
}

func (h *StrategyHandler) ToggleIndicatorCondition(c *gin.Context) {
	uid := getUID(c)
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "策略ID错误")
		return
	}

	// Verify ownership
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}

	var req ToggleIndicatorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var cond model.StrategyCondition
	if db.MySQL.Where("id = ? AND strategy_id = ?", req.CondID, sid).First(&cond).Error != nil {
		response.NotFound(c, "条件不存在")
		return
	}

	// Toggle or set
	if req.Enabled != nil {
		cond.Enabled = *req.Enabled
	} else {
		cond.Enabled = !cond.Enabled
	}

	db.MySQL.Save(&cond)
	log.Printf("[strategy] indicator toggle cond=%d indicator=%s enabled=%v uid=%d", cond.ID, cond.Indicator, cond.Enabled, uid)
	response.SuccessMsg(c, "已更新")
}

// BulkToggleIndicator toggles all conditions for an indicator type within a strategy.
type BulkToggleReq struct {
	Indicator string `json:"indicator"`
	CondType  string `json:"condType"` // buy/sell/add/reduce
	Enabled   bool   `json:"enabled"`
}

func (h *StrategyHandler) BulkToggleIndicator(c *gin.Context) {
	uid := getUID(c)
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "策略ID错误")
		return
	}

	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}

	var req BulkToggleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	db.MySQL.Model(&model.StrategyCondition{}).
		Where("strategy_id = ? AND indicator = ? AND cond_type = ?", sid, req.Indicator, req.CondType).
		Update("enabled", req.Enabled)

	log.Printf("[strategy] bulk toggle sid=%d indicator=%s condType=%s enabled=%v uid=%d", sid, req.Indicator, req.CondType, req.Enabled, uid)
	response.SuccessMsg(c, fmt.Sprintf("已%s %s 的 %s 指标", map[bool]string{true: "启用", false: "禁用"}[req.Enabled], req.CondType, req.Indicator))
}

func (h *StrategyHandler) TestIndicator(c *gin.Context) {
	var req TestIndicatorReq
	if err := c.ShouldBindJSON(&req); err != nil || req.StockCode == "" || req.Date == "" || req.Indicator == "" || req.Operator == "" {
		response.BadRequest(c, "参数错误：stockCode/date/indicator/operator 必填")
		return
	}

	// Parse value (accepts both "5" and 5)
	valFloat, err := req.Value.Float64()
	if err != nil {
		response.BadRequest(c, "参数错误：value 必须是数字")
		return
	}

	// Look up indicator metadata
	indicatorInfo := getIndicatorMeta(req.Indicator)
	indicatorLabel := req.Indicator
	indicatorDataNote := ""
	if indicatorInfo != nil {
		indicatorLabel = indicatorInfo["label"].(string)
		if dn, ok := indicatorInfo["dataNote"]; ok {
			indicatorDataNote = dn.(string)
		}
	}

	// Look up stock name
	stockName := getStockName(req.StockCode)

	// Get data source
	dataSource := getIndicatorDataSource(req.Indicator)

	resp := TestIndicatorResp{
		StockCode:      req.StockCode,
		StockName:      stockName,
		Date:           req.Date,
		Indicator:      req.Indicator,
		IndicatorLabel: indicatorLabel,
		Operator:       req.Operator,
		OperatorLabel:  getOperatorLabel(req.Operator),
		Threshold:      valFloat,
		DataSource:     dataSource,
		DataNote:       indicatorDataNote,
	}

	// Check if data exists for this stock/date
	if !hasAnyData(req.StockCode, req.Date, req.Indicator) {
		resp.Error = "该股票在指定日期无对应数据"
		resp.HasData = false
		response.Success(c, resp)
		return
	}

	// Create a fake condition to reuse the evaluation logic
	cond := model.StrategyCondition{
		Indicator: req.Indicator,
		Operator:  req.Operator,
		Value:     valFloat,
	}

	computedValue := getIndicatorValue(cond, req.StockCode, req.Date)
	resp.ComputedValue = math.Round(computedValue*100) / 100
	resp.HasData = true
	resp.ConditionMet = evaluateSingleCondition(cond, req.StockCode, req.Date)

	response.Success(c, resp)
}

func getIndicatorMeta(key string) map[string]interface{} {
	m := GetIndicatorMeta(key)
	if m == nil {
		return nil
	}
	return map[string]interface{}{
		"key": m.Key, "label": m.Label, "category": m.Category,
		"unit": m.Unit, "type": m.Type, "operators": m.Operators,
		"desc": m.Desc, "backtestSafe": m.BacktestSafe,
		"dataNote": m.DataNote, "suggestion": m.Suggestion,
		"useFor": m.UseFor, "dataSource": m.DataSource,
	}
}

func getOperatorLabel(op string) string {
	switch op {
	case "gte": return "≥ (大于等于)"
	case "lte": return "≤ (小于等于)"
	case "gt": return "> (大于)"
	case "lt": return "< (小于)"
	case "eq": return "= (等于)"
	case "cross_up": return "↑ 上穿"
	case "cross_down": return "↓ 下穿"
	}
	return op
}

func getIndicatorDataSource(indicator string) string {
	return GetIndicatorDataSource(indicator)
}

// getIndicatorDataSourceLegacy kept for reference.
func getIndicatorDataSourceLegacy(indicator string) string {
	switch {
	// K线衍生
	case indicator == "daily_change", indicator == "momentum_5", indicator == "momentum_20",
		indicator == "ma_5", indicator == "ma_10", indicator == "ma_20", indicator == "ma_30", indicator == "ma_60", indicator == "ma_deviation", indicator == "ma_cross", indicator == "macd",
		indicator == "ema_cross", indicator == "macd_dif", indicator == "macd_dea", indicator == "rsi", indicator == "rsi_6", indicator == "rsi_12", indicator == "rsi_24", indicator == "kdj_k", indicator == "kdj_d", indicator == "kdj_j",
		indicator == "boll_position", indicator == "boll_width", indicator == "boll_squeeze", indicator == "boll_upper", indicator == "boll_middle", indicator == "boll_lower",
		indicator == "volume_ratio", indicator == "volume_ma_ratio", indicator == "turnover_rate",
		indicator == "atr", indicator == "atr_pct", indicator == "drawdown_20", indicator == "new_high_20",
		indicator == "up_days_ratio", indicator == "price_position_20", indicator == "price_position_60",
		indicator == "adx", indicator == "dmi_plus", indicator == "dmi_minus",
		indicator == "cci", indicator == "psy_12", indicator == "psy_ma", indicator == "williams_r", indicator == "mfi",
		indicator == "ma_convergence", indicator == "trend_strength",
		indicator == "consecutive_days", indicator == "gap_pct", indicator == "high_low_range",
		indicator == "vwap_deviation", indicator == "volume_trend", indicator == "index_relative":
		return "stocks_daily_k"
	// 估值指标
	case indicator == "pe", indicator == "pb", indicator == "ps",
		indicator == "pe_percentile", indicator == "pb_percentile",
		indicator == "total_market_cap":
		return "stocks_daily_indicator"
	// 财务数据
	case indicator == "roe", indicator == "revenue_growth", indicator == "profit_growth",
		indicator == "gross_margin", indicator == "net_margin", indicator == "debt_ratio",
		indicator == "eps":
		return "stock_financials"
	// AI评分
	case strings.HasPrefix(indicator, "ai_"):
		return "ai_stock_scores"
	// 榜单
	case indicator == "streak_count", indicator == "algo_score":
		return "algorithm_pick_details"
	// 信号
	case indicator == "signal_value":
		return "stock_signals"
	// 股东
	case indicator == "shareholder_change", indicator == "inst_hold_ratio":
		return "stock_shareholders"
	// 预测
	case indicator == "prediction_upside", indicator == "prediction_consensus":
		return "ai_stock_predictions"
	}
	return "unknown"
}

func hasAnyData(code, date, indicator string) bool {
	switch {
	case indicator == "daily_change", strings.HasPrefix(indicator, "momentum"),
		indicator == "ma_5", indicator == "ma_10", indicator == "ma_20", indicator == "ma_30", indicator == "ma_60", indicator == "ma_deviation", indicator == "ma_cross", indicator == "macd",
		indicator == "ema_cross", indicator == "macd_dif", indicator == "macd_dea", indicator == "rsi", indicator == "rsi_6", indicator == "rsi_12", indicator == "rsi_24", strings.HasPrefix(indicator, "kdj"),
		strings.HasPrefix(indicator, "boll"), indicator == "boll_squeeze",
		strings.HasPrefix(indicator, "volume"), indicator == "turnover_rate",
		indicator == "atr", indicator == "atr_pct", strings.HasPrefix(indicator, "drawdown"),
		strings.HasPrefix(indicator, "new_high"), indicator == "up_days_ratio",
		strings.HasPrefix(indicator, "price_position"),
		indicator == "adx", strings.HasPrefix(indicator, "dmi_"),
		indicator == "cci", indicator == "psy_12", indicator == "psy_ma", indicator == "williams_r", indicator == "mfi",
		indicator == "ma_convergence", indicator == "trend_strength",
		indicator == "consecutive_days", indicator == "gap_pct", indicator == "high_low_range",
		indicator == "vwap_deviation", indicator == "volume_trend", indicator == "index_relative":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&count)
		return count > 0
	case indicator == "pe", indicator == "pb", indicator == "ps",
		indicator == "pe_percentile", indicator == "pb_percentile",
		indicator == "total_market_cap":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?", code, date).Scan(&count)
		return count > 0
	case indicator == "roe", indicator == "revenue_growth", indicator == "profit_growth",
		indicator == "gross_margin", indicator == "net_margin", indicator == "debt_ratio",
		indicator == "eps":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stock_financials WHERE code = ? AND report_date <= ?", code, date).Scan(&count)
		return count > 0
	case strings.HasPrefix(indicator, "ai_"):
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM ai_stock_scores WHERE code = ?", code).Scan(&count)
		return count > 0
	case indicator == "streak_count", indicator == "algo_score":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM algorithm_pick_details WHERE stock_code = ? AND pick_date <= ?", code, date).Scan(&count)
		return count > 0
	case indicator == "signal_value":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stock_signals WHERE code = ?", code).Scan(&count)
		return count > 0
	case indicator == "shareholder_change", indicator == "inst_hold_ratio":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stock_shareholders WHERE code = ? AND report_date <= ?", code, date).Scan(&count)
		return count > 0
	case indicator == "prediction_upside", indicator == "prediction_consensus":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM ai_stock_predictions WHERE code = ? AND predict_date = ?", code, date).Scan(&count)
		return count > 0
	}
	return false
}

func getStockName(code string) string {
	var name string
	if err := db.PG.Raw("SELECT COALESCE(name,'') FROM stocks_basic WHERE code = ? LIMIT 1", code).Scan(&name).Error; err != nil {
		log.Printf("[backtest] stock name query failed for %s: %v", code, err)
	}
	return name
}
