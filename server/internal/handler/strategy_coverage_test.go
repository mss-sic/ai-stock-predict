package handler

import (
	"fmt"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// 量化策略案例定义 & 条件指标覆盖度验证
// ═══════════════════════════════════════════════════════════════

// QuantStrategy 定义一个量化策略及其所需的条件指标
type QuantStrategy struct {
	Name        string   // 策略名称
	Type        string   // 止损型 / 趋势型 / 均值回归 / 动量 / 多因子 / 突破
	Description string   // 策略描述
	BuyConds    []string // 买入条件所需指标
	SellConds   []string // 卖出条件所需指标
	StopLoss    float64  // 止损比例（负数）
	StopProfit  float64  // 止盈比例
}

// stratExec 评估一个策略是否可以被当前系统完整表达
type StratCheckResult struct {
	Strategy       string
	Supported      bool
	IndicatorsOK   int      // 已覆盖指标数
	IndicatorsNeed int      // 总需求指标数
	Missing        []string // 缺失指标
	MissingNote    string   // 缺失说明
}

// allStrategies 定义8个经典量化策略
func allStrategies() []QuantStrategy {
	return []QuantStrategy{
		// ═══ 策略1：双均线金死叉 ═══
		{
			Name:        "双均线金死叉策略",
			Type:        "趋势跟踪",
			Description: "MA5上穿MA20买入，MA5下穿MA20卖出，经典趋势跟踪",
			BuyConds:    []string{"ma_cross"},   // cross_up MA5/MA20
			SellConds:   []string{"ma_cross"},   // cross_down MA5/MA20
			StopLoss:    -8.0,
			StopProfit:  15.0,
		},

		// ═══ 策略2：MACD金叉+成交量确认 ═══
		{
			Name:        "MACD量价共振策略",
			Type:        "趋势跟踪",
			Description: "MACD金叉(DIF上穿DEA)且成交量放量(>1.5倍5日均量)买入；MACD死叉卖出",
			BuyConds:    []string{"macd", "volume_ratio"},     // cross_up + volume_ratio > 1.5
			SellConds:   []string{"macd"},                     // cross_down
			StopLoss:    -5.0,
			StopProfit:  10.0,
		},

		// ═══ 策略3：布林带均值回归 ═══
		{
			Name:        "布林带均值回归策略",
			Type:        "均值回归",
			Description: "价格触及布林下轨(位置<20)买入，回到中轨以上或触及上轨(>80)卖出",
			BuyConds:    []string{"boll_position", "rsi"},         // boll_position < 20, rsi < 30
			SellConds:   []string{"boll_position"},                // boll_position > 50 (回中轨) or > 80
			StopLoss:    -5.0,
			StopProfit:  8.0,
		},

		// ═══ 策略4：RSI超卖反弹 ═══
		{
			Name:        "RSI超卖反弹策略",
			Type:        "均值回归",
			Description: "RSI(14)<30且单日跌幅>3%买入，RSI>70或涨超10%卖出",
			BuyConds:    []string{"rsi", "daily_change"},          // rsi < 30, daily_change < -3
			SellConds:   []string{"rsi"},                          // rsi > 70
			StopLoss:    -5.0,
			StopProfit:  10.0,
		},

		// ═══ 策略5：多因子价值投资 ═══
		{
			Name:        "价值多因子策略",
			Type:        "多因子",
			Description: "低PE(<15)+低PB(<2)+高ROE(>15%)+利润正增长买入；PE>30或ROE<5%卖出",
			BuyConds:    []string{"pe", "pb", "roe", "profit_growth"},
			SellConds:   []string{"pe", "roe"},
			StopLoss:    -10.0,
			StopProfit:  30.0,
		},

		// ═══ 策略6：动量+趋势强度 ═══
		{
			Name:        "动量趋势策略",
			Type:        "动量",
			Description: "20日动量>5%+趋势强度>0.6+ADX>25买入；动量<-3%或趋势强度<0.4卖出",
			BuyConds:    []string{"momentum_20", "trend_strength", "adx"},
			SellConds:   []string{"momentum_20", "trend_strength"},
			StopLoss:    -8.0,
			StopProfit:  20.0,
		},

		// ═══ 策略7：筹码集中度+股东数据 ═══
		{
			Name:        "筹码集中策略",
			Type:        "资金面",
			Description: "股东户数下降>5%(筹码集中)+机构持股>30%买入；股东户数上升>10%卖出",
			BuyConds:    []string{"shareholder_change", "inst_hold_ratio"},
			SellConds:   []string{"shareholder_change"},
			StopLoss:    -10.0,
			StopProfit:  25.0,
		},

		// ═══ 策略8：KDJ超卖金叉 ═══
		{
			Name:        "KDJ超卖金叉策略",
			Type:        "均值回归",
			Description: "KDJ-J<0(超卖)且K上穿D买入；KDJ-J>100或K<D卖出",
			BuyConds:    []string{"kdj_j", "kdj_k", "kdj_d"}, // J<0 + K cross_up D
			SellConds:   []string{"kdj_j"},                   // J>100
			StopLoss:    -5.0,
			StopProfit:  12.0,
		},

		// ═══ 策略9：VWAP偏离+大盘强度 ═══
		{
			Name:        "VWAP相对强度策略",
			Type:        "趋势跟踪",
			Description: "VWAP偏离<-2%(低于均价)且跑赢大盘>5%买入；VWAP偏离>3%卖出",
			BuyConds:    []string{"vwap_deviation", "index_relative"},
			SellConds:   []string{"vwap_deviation"},
			StopLoss:    -5.0,
			StopProfit:  10.0,
		},

		// ═══ 策略10：布林收口突破 ═══
		{
			Name:        "布林收口突破策略",
			Type:        "突破",
			Description: "布林挤压<5(极度收口)+ATR/价格%偏低+20日新高买入；ATR扩张过大卖出",
			BuyConds:    []string{"boll_squeeze", "atr_pct", "new_high_20"},
			SellConds:   []string{"atr_pct"},
			StopLoss:    -5.0,
			StopProfit:  15.0,
		},

		// ═══ 策略11：PSYCHE心理线 ═══
		{
			Name:        "PSY心理线策略",
			Type:        "均值回归",
			Description: "PSY(12)<25(极度恐慌)买入；PSY>75(过度乐观)卖出",
			BuyConds:    []string{"psy_12"},
			SellConds:   []string{"psy_12"},
			StopLoss:    -5.0,
			StopProfit:  10.0,
		},

		// ═══ 策略12：多指标综合策略（最复杂） ═══
		{
			Name:        "多维度综合策略",
			Type:        "多因子",
			Description: "同时满足：技术面(ADX>25趋势明确)+资金面(量比>1.2放量)+基本面(ROE>12%)+估值(PE<25)买入",
			BuyConds:    []string{"adx", "volume_ma_ratio", "roe", "pe"},
			SellConds:   []string{"adx", "pe"},
			StopLoss:    -8.0,
			StopProfit:  25.0,
		},
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 1: 策略指标覆盖度 — 每个策略所需的指标是否在注册表中
// ═══════════════════════════════════════════════════════════════

func TestStrategyCoverage(t *testing.T) {
	strategies := allStrategies()
	var results []StratCheckResult

	for _, s := range strategies {
		result := StratCheckResult{
			Strategy: s.Name,
		}

		// 检查买入条件指标
		for _, ind := range s.BuyConds {
			result.IndicatorsNeed++
			if m := GetIndicatorMeta(ind); m != nil {
				result.IndicatorsOK++
			} else {
				result.Missing = append(result.Missing, ind)
			}
		}
		// 检查卖出条件指标
		for _, ind := range s.SellConds {
			result.IndicatorsNeed++
			if m := GetIndicatorMeta(ind); m != nil {
				result.IndicatorsOK++
			} else {
				result.Missing = append(result.Missing, ind)
			}
		}

		result.Supported = len(result.Missing) == 0
		results = append(results, result)
	}

	// 打印报告 & 断言
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  量化策略指标覆盖度报告")
	fmt.Println(strings.Repeat("=", 70))

	totalSupported := 0
	for _, r := range results {
		status := "✅"
		note := ""
		if !r.Supported {
			status = "❌"
			note = fmt.Sprintf("  缺失指标: %v", r.Missing)
			r.MissingNote = strings.Join(r.Missing, ", ")
		} else {
			totalSupported++
		}
		fmt.Printf("%s %s (%s)\n", status, r.Strategy, allStrategies()[indexOf(results, r)].Type)
		fmt.Printf("   指标覆盖: %d/%d\n", r.IndicatorsOK, r.IndicatorsNeed)
		if note != "" {
			fmt.Println(note)
		}
	}

	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("总计: %d/%d 策略完整支持\n", totalSupported, len(results))
	fmt.Println(strings.Repeat("=", 70))

	if totalSupported != len(results) {
		t.Errorf("Only %d/%d strategies fully supported — missing indicators above",
			totalSupported, len(results))
	}
}

func indexOf(results []StratCheckResult, target StratCheckResult) int {
	for i, r := range results {
		if r.Strategy == target.Strategy {
			return i
		}
	}
	return -1
}

// ═══════════════════════════════════════════════════════════════
// Test 2: 指标按策略类型的分类覆盖 — 每种量化模式需要哪些指标
// ═══════════════════════════════════════════════════════════════

func TestStrategyTypeCoverage(t *testing.T) {
	type TypeRequirement struct {
		Type         string
		NeedIndicators []string
		Description  string
	}

	requirements := []TypeRequirement{
		{
			Type: "趋势跟踪必备指标",
			NeedIndicators: []string{
				"ma_cross", "macd", "adx", "dmi_plus", "dmi_minus",
				"trend_strength", "ema_cross", "ma_deviation",
			},
			Description: "判断趋势方向的指标集合",
		},
		{
			Type: "均值回归必备指标",
			NeedIndicators: []string{
				"rsi", "kdj_k", "kdj_d", "kdj_j", "boll_position",
				"boll_width", "boll_squeeze", "cci", "williams_r",
				"psy_12", "psy_ma",
			},
			Description: "识别超买超卖状态的指标集合",
		},
		{
			Type: "动量策略必备指标",
			NeedIndicators: []string{
				"momentum_5", "momentum_20", "daily_change",
				"price_position_20", "price_position_60",
				"new_high_20", "index_relative",
			},
			Description: "度量价格动量强弱",
		},
		{
			Type: "波动率策略必备指标",
			NeedIndicators: []string{
				"atr", "atr_pct", "boll_width", "boll_squeeze",
				"high_low_range", "ma_convergence",
			},
			Description: "度量波动及变盘信号",
		},
		{
			Type: "资金面策略必备指标",
			NeedIndicators: []string{
				"volume_ratio", "volume_ma_ratio", "volume_trend",
				"turnover_rate", "mfi", "vwap_deviation",
				"shareholder_change", "inst_hold_ratio",
			},
			Description: "资金流量和筹码分布",
		},
		{
			Type: "价值投资必备指标",
			NeedIndicators: []string{
				"pe", "pb", "ps", "pe_percentile", "pb_percentile",
				"roe", "revenue_growth", "profit_growth",
				"gross_margin", "net_margin", "debt_ratio", "eps",
			},
			Description: "基本面估值指标",
		},
		{
			Type: "形态/结构必备指标",
			NeedIndicators: []string{
				"drawdown_20", "up_days_ratio", "consecutive_days",
				"gap_pct", "high_low_range",
			},
			Description: "K线形态与市场结构",
		},
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  按策略类型分类的指标覆盖度")
	fmt.Println(strings.Repeat("=", 70))

	allCovered := true
	for _, req := range requirements {
		missing := []string{}
		for _, ind := range req.NeedIndicators {
			if GetIndicatorMeta(ind) == nil {
				missing = append(missing, ind)
			}
		}
		status := "✅"
		if len(missing) > 0 {
			status = "❌"
			allCovered = false
		}
		fmt.Printf("%s %s (%s)\n", status, req.Type, req.Description)
		fmt.Printf("   需求 %d 个指标", len(req.NeedIndicators))
		if len(missing) > 0 {
			fmt.Printf(", 缺失: %v\n", missing)
		} else {
			fmt.Printf(", 全部覆盖\n")
		}
		// Show the indicator list
		fmt.Printf("   指标: %s\n", strings.Join(req.NeedIndicators, ", "))
	}

	if !allCovered {
		t.Error("Some strategy type indicators are missing from registry")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 3: 策略条件构建 — 验证具体条件配置能否被系统正确表达
// ═══════════════════════════════════════════════════════════════

func TestBuildStrategyConditions(t *testing.T) {
	type StrategyConditionConfig struct {
		Strategy   string
		CondType   string
		Indicator  string
		Operator   string
		Value      float64
		ShouldWork bool // 是否应该被回测引擎支持
	}

	tests := []StrategyConditionConfig{
		// 双均线
		{Strategy: "双均线金死叉", CondType: "buy", Indicator: "ma_cross", Operator: "cross_up", Value: 5.020, ShouldWork: true},
		{Strategy: "双均线金死叉", CondType: "sell", Indicator: "ma_cross", Operator: "cross_down", Value: 5.020, ShouldWork: true},

		// MACD量价共振
		{Strategy: "MACD量价共振", CondType: "buy", Indicator: "macd", Operator: "cross_up", Value: 0, ShouldWork: true},
		{Strategy: "MACD量价共振", CondType: "buy", Indicator: "volume_ratio", Operator: "gt", Value: 1.5, ShouldWork: true},
		{Strategy: "MACD量价共振", CondType: "sell", Indicator: "macd", Operator: "cross_down", Value: 0, ShouldWork: true},

		// 布林带均值回归
		{Strategy: "布林带均值回归", CondType: "buy", Indicator: "boll_position", Operator: "lt", Value: 20, ShouldWork: true},
		{Strategy: "布林带均值回归", CondType: "sell", Indicator: "boll_position", Operator: "gt", Value: 80, ShouldWork: true},

		// RSI超卖反弹
		{Strategy: "RSI超卖反弹", CondType: "buy", Indicator: "rsi", Operator: "lt", Value: 30, ShouldWork: true},
		{Strategy: "RSI超卖反弹", CondType: "sell", Indicator: "rsi", Operator: "gt", Value: 70, ShouldWork: true},

		// 价值多因子
		{Strategy: "价值多因子", CondType: "buy", Indicator: "pe", Operator: "lt", Value: 15, ShouldWork: true},
		{Strategy: "价值多因子", CondType: "buy", Indicator: "pb", Operator: "lt", Value: 2, ShouldWork: true},
		{Strategy: "价值多因子", CondType: "buy", Indicator: "roe", Operator: "gt", Value: 15, ShouldWork: true},
		{Strategy: "价值多因子", CondType: "sell", Indicator: "pe", Operator: "gt", Value: 30, ShouldWork: true},

		// 动量趋势
		{Strategy: "动量趋势", CondType: "buy", Indicator: "momentum_20", Operator: "gt", Value: 5, ShouldWork: true},
		{Strategy: "动量趋势", CondType: "buy", Indicator: "trend_strength", Operator: "gt", Value: 0.6, ShouldWork: true},
		{Strategy: "动量趋势", CondType: "buy", Indicator: "adx", Operator: "gt", Value: 25, ShouldWork: true},

		// PSY心理线 (回归测试 — 之前有Bug)
		{Strategy: "PSY心理线", CondType: "buy", Indicator: "psy_12", Operator: "lt", Value: 25, ShouldWork: true},
		{Strategy: "PSY心理线", CondType: "sell", Indicator: "psy_12", Operator: "gt", Value: 75, ShouldWork: true},

		// 不能用于回测的指标
		{Strategy: "预测策略(不可回测)", CondType: "buy", Indicator: "prediction_upside", Operator: "gt", Value: 10, ShouldWork: false},
		{Strategy: "AI评分策略(不可回测)", CondType: "buy", Indicator: "ai_score", Operator: "gt", Value: 6, ShouldWork: false},
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  策略条件构建验证")
	fmt.Println(strings.Repeat("=", 70))

	for _, tc := range tests {
		// 检查指标存在
		meta := GetIndicatorMeta(tc.Indicator)
		if meta == nil {
			t.Errorf("❌ %s: 指标 %s 不在注册表中", tc.Strategy, tc.Indicator)
			continue
		}

		// 检查 operator 是否支持
		opSupported := false
		for _, op := range meta.Operators {
			if op == tc.Operator {
				opSupported = true
				break
			}
		}
		if !opSupported {
			t.Errorf("❌ %s: 指标 %s 不支持操作符 %s (支持: %v)",
				tc.Strategy, tc.Indicator, tc.Operator, meta.Operators)
			continue
		}

		// 检查回测安全性
		isSafe := IsBacktestSafe(tc.Indicator)
		if tc.ShouldWork && !isSafe {
			t.Errorf("⚠️ %s: 指标 %s 应该可用于回测但标记为 unsafe",
				tc.Strategy, tc.Indicator)
		}

		status := "✅"
		note := ""
		if !tc.ShouldWork && isSafe {
			note = " (警告: 实际应在回测中禁用)"
		}
		if !isSafe {
			note = fmt.Sprintf(" (回测不可用: %s)", meta.DataNote)
		}
		fmt.Printf("%s %s | %s.%s %s %.1f%s\n",
			status, tc.Strategy, tc.CondType, tc.Indicator, tc.Operator, tc.Value, note)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 4: 策略间指标对比 — 哪些指标最常用
// ═══════════════════════════════════════════════════════════════

func TestIndicatorUsageFrequency(t *testing.T) {
	strategies := allStrategies()
	freq := make(map[string]int)

	for _, s := range strategies {
		for _, ind := range s.BuyConds {
			freq[ind]++
		}
		for _, ind := range s.SellConds {
			freq[ind]++
		}
	}

	// Sort by frequency
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}
	// Simple sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Value > sorted[i].Value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  指标使用频率排行（12个策略中）")
	fmt.Println(strings.Repeat("=", 70))

	for _, kv := range sorted {
		bar := strings.Repeat("█", kv.Value)
		meta := GetIndicatorMeta(kv.Key)
		label := kv.Key
		if meta != nil {
			label = fmt.Sprintf("%s (%s)", kv.Key, meta.Label)
		}
		fmt.Printf("  %-35s %dx %s\n", label, kv.Value, bar)
	}

	// Assert core indicators are present
	coreIndicators := []string{"ma_cross", "rsi", "pe", "macd", "boll_position", "adx"}
	for _, ind := range coreIndicators {
		if GetIndicatorMeta(ind) == nil {
			t.Errorf("Core indicator %s missing from registry", ind)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 5: 组合条件验证 — AND组内/OR组间逻辑
// ═══════════════════════════════════════════════════════════════

func TestMultiConditionGroups(t *testing.T) {
	// 模拟 "MACD量价共振" — 2个buy条件在同一logicGroup(AND)
	// 外加一个独立条件组(OR)做备选买入
	conds := []struct {
		Indicator  string
		Operator   string
		Value      float64
		LogicGroup int
		CondType   string
	}{
		// 组1 (AND): MACD金叉 + 放量 → 必须同时满足
		{Indicator: "macd", Operator: "cross_up", Value: 0, LogicGroup: 1, CondType: "buy"},
		{Indicator: "volume_ratio", Operator: "gt", Value: 1.5, LogicGroup: 1, CondType: "buy"},
		// 组2 (OR备选): RSI超卖反弹
		{Indicator: "rsi", Operator: "lt", Value: 30, LogicGroup: 2, CondType: "buy"},
		{Indicator: "daily_change", Operator: "lt", Value: -3, LogicGroup: 2, CondType: "buy"},
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  组合条件逻辑验证")
	fmt.Println(strings.Repeat("=", 70))

	groups := make(map[int][]string)
	for _, c := range conds {
		if GetIndicatorMeta(c.Indicator) == nil {
			t.Errorf("Indicator %s not in registry", c.Indicator)
			continue
		}
		groups[c.LogicGroup] = append(groups[c.LogicGroup],
			fmt.Sprintf("%s %s %.1f", c.Indicator, c.Operator, c.Value))
	}

	for gid, condStrs := range groups {
		fmt.Printf("LogicGroup %d (AND):\n", gid)
		for _, s := range condStrs {
			fmt.Printf("  • %s\n", s)
		}
	}
	fmt.Println("组间关系: OR — 任一组满足即触发买入")
	fmt.Println("说明: 即 MACD金叉+放量 或 RSI超卖反弹，任一组合满足即可买入")

	if len(groups) != 2 {
		t.Errorf("Expected 2 logic groups, got %d", len(groups))
	}
	if len(groups[1]) != 2 {
		t.Errorf("Expected 2 conditions in group 1, got %d", len(groups[1]))
	}
}
