package handler

// IndicatorMeta holds all metadata for a single indicator.
type IndicatorMeta struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Category     string   `json:"category"`     // 榜单/技术面-趋势/技术面-超买超卖/技术面-量价/技术面-形态/估值/基本面/资金面/AI评分/预测
	Unit         string   `json:"unit"`         // %, 元, 亿, 分, 次数, 比值, 率
	Type         string   `json:"type"`         // number, cross
	Operators    []string `json:"operators"`
	Desc         string   `json:"desc"`
	BacktestSafe bool     `json:"backtestSafe"`
	DataNote     string   `json:"dataNote"`
	Suggestion   string   `json:"suggestion"`
	UseFor       string   `json:"useFor"`       // buy, sell, both
	DataSource   string   `json:"dataSource"`   // stocks_daily_k, stocks_daily_indicator, etc.
}

// IndicatorRegistry is the single source of truth for all trading indicators.
// Codex: DO NOT add indicators in scattered switch statements; register them here.
var IndicatorRegistry = map[string]*IndicatorMeta{
	// ═══ 榜单与评分 ═══
	"streak_count": {
		Key: "streak_count", Label: "连榜次数", Category: "榜单与评分", Unit: "次数", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "该股票在榜单连续出现的交易日数", BacktestSafe: true, DataNote: "依赖榜单数据覆盖",
		Suggestion: "买入建议 ≥ 3 天，连续上榜说明持续受关注", UseFor: "buy", DataSource: "algorithm_pick_details",
	},
	"algo_score": {
		Key: "algo_score", Label: "算法评分", Category: "榜单与评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "算法团队给出的综合评分 (0-10)", BacktestSafe: true, DataNote: "仅榜单日期有值",
		Suggestion: "买入建议 ≥ 6 分，算法团队评分越高越好", UseFor: "buy", DataSource: "algorithm_pick_details",
	},
	"signal_value": {
		Key: "signal_value", Label: "原始信号值", Category: "榜单与评分", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "算法团队原始信号值（越大越强）", BacktestSafe: false, DataNote: "⚠️ 单点快照无历史，回测禁用",
		Suggestion: "买入建议 > 0.5，正值表示信号偏多", UseFor: "buy", DataSource: "stock_signals",
	},

	// ═══ AI六维评分 ═══
	"ai_score": {
		Key: "ai_score", Label: "AI综合评分", Category: "AI评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "AI六维综合评分 (0-10)", BacktestSafe: false, DataNote: "⚠️ 仅少量股票有AI评分",
		Suggestion: "买入建议 ≥ 6 分，AI综合评估偏高", UseFor: "buy", DataSource: "ai_stock_scores",
	},
	"ai_fundamental": {
		Key: "ai_fundamental", Label: "AI基本面", Category: "AI评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "AI基本面评分 (0-10)", BacktestSafe: false, DataNote: "⚠️ 仅少量股票有AI评分",
		Suggestion: "买入建议 ≥ 6 分，基本面扎实", UseFor: "buy", DataSource: "ai_stock_scores",
	},
	"ai_technical": {
		Key: "ai_technical", Label: "AI技术面", Category: "AI评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "AI技术面评分 (0-10)", BacktestSafe: false, DataNote: "⚠️ 仅少量股票有AI评分",
		Suggestion: "买入建议 ≥ 6 分，技术形态良好", UseFor: "buy", DataSource: "ai_stock_scores",
	},
	"ai_valuation": {
		Key: "ai_valuation", Label: "AI估值", Category: "AI评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "AI估值评分 (0-10)", BacktestSafe: false, DataNote: "⚠️ 仅少量股票有AI评分",
		Suggestion: "买入建议 ≥ 6 分，估值合理偏低", UseFor: "buy", DataSource: "ai_stock_scores",
	},
	"ai_growth": {
		Key: "ai_growth", Label: "AI成长性", Category: "AI评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "AI成长性评分 (0-10)", BacktestSafe: false, DataNote: "⚠️ 仅少量股票有AI评分",
		Suggestion: "买入建议 ≥ 6 分，成长性突出", UseFor: "buy", DataSource: "ai_stock_scores",
	},
	"ai_industry": {
		Key: "ai_industry", Label: "AI行业面", Category: "AI评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "AI行业面评分 (0-10)", BacktestSafe: false, DataNote: "⚠️ 仅少量股票有AI评分",
		Suggestion: "买入建议 ≥ 6 分，行业景气度高", UseFor: "buy", DataSource: "ai_stock_scores",
	},
	"ai_capital": {
		Key: "ai_capital", Label: "AI资金面", Category: "AI评分", Unit: "分", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "eq"},
		Desc: "AI资金面评分 (0-10)", BacktestSafe: false, DataNote: "⚠️ 仅少量股票有AI评分",
		Suggestion: "买入建议 ≥ 6 分，资金面积极", UseFor: "buy", DataSource: "ai_stock_scores",
	},

	// ═══ 技术面 — 趋势类 ═══
	"daily_change": {
		Key: "daily_change", Label: "单日涨跌幅", Category: "技术面-趋势", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当日涨跌幅 (%)", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 2% 或 < -5% 超跌反弹", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"momentum_5": {
		Key: "momentum_5", Label: "5日动量", Category: "技术面-趋势", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "近5个交易日累计涨跌幅 (%)", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 3%，短期趋势向上", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"momentum_20": {
		Key: "momentum_20", Label: "20日动量", Category: "技术面-趋势", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "近20个交易日累计涨跌幅 (%)", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 5%，中期趋势确立", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_deviation": {
		Key: "ma_deviation", Label: "均线偏离", Category: "技术面-趋势", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "收盘价偏离MA20的百分比", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < -5% 超跌，卖出建议 > 10% 超涨", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_5": {
		Key: "ma_5", Label: "MA5均线", Category: "技术面-趋势", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "5日收盘均价", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "收盘价>MA5短线偏多，<MA5偏空", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_10": {
		Key: "ma_10", Label: "MA10均线", Category: "技术面-趋势", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "10日收盘均价", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "MA5>MA10短线金叉", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_20": {
		Key: "ma_20", Label: "MA20均线", Category: "技术面-趋势", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "20日收盘均价", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "收盘价>MA20中线偏多，<MA20偏空", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_30": {
		Key: "ma_30", Label: "MA30均线", Category: "技术面-趋势", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "30日收盘均价", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "收盘价>MA30趋势偏多", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_60": {
		Key: "ma_60", Label: "MA60均线", Category: "技术面-趋势", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "60日收盘均价", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "收盘价>MA60长线多头排列", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_cross": {
		Key: "ma_cross", Label: "MA金叉/死叉", Category: "技术面-趋势", Unit: "信号", Type: "cross",
		Operators: []string{"cross_up", "cross_down"},
		Desc: "value填5.020表示MA5上穿/下穿MA20（简化版SMA交叉）", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 cross_up 金叉信号，卖出建议 cross_down", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"macd": {
		Key: "macd", Label: "MACD交叉(简化)", Category: "技术面-趋势", Unit: "信号", Type: "cross",
		Operators: []string{"cross_up", "cross_down"},
		Desc: "简化版MACD金叉/死叉（SMA12-SMA26的DIF与DEA交叉），非标准EMA实现", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 cross_up 金叉，卖出建议 cross_down 死叉", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"macd_dif": {
		Key: "macd_dif", Label: "MACD DIF(简化)", Category: "技术面-趋势", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "简化版DIF = SMA12 - SMA26，非标准EMA", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "DIF>0 偏多，DIF<0 偏空", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"macd_dea": {
		Key: "macd_dea", Label: "MACD DEA(简化)", Category: "技术面-趋势", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "简化版DEA = DIF的9日SMA，非标准EMA", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "DIF>DEA 偏多，DIF<DEA 偏空", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ema_cross": {
		Key: "ema_cross", Label: "EMA交叉(简化)", Category: "技术面-趋势", Unit: "信号", Type: "cross",
		Operators: []string{"cross_up", "cross_down"},
		Desc: "简化版EMA12/26交叉（实际使用SMA计算），非标准EMA", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 cross_up 短均线上穿长均线", UseFor: "both", DataSource: "stocks_daily_k",
	},

	// ═══ 技术面 — 超买超卖 ═══
	"rsi": {
		Key: "rsi", Label: "RSI(14)", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "14日相对强弱指数 (0-100)，>70超买 <30超卖", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 30 超卖，卖出建议 > 70 超买", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"rsi_6": {
		Key: "rsi_6", Label: "RSI(6)", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "6日相对强弱指数，更灵敏", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 20 超卖，卖出建议 > 80 超买", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"rsi_12": {
		Key: "rsi_12", Label: "RSI(12)", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "12日相对强弱指数", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 30 超卖，卖出建议 > 70 超买", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"rsi_24": {
		Key: "rsi_24", Label: "RSI(24)", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "24日相对强弱指数，更稳定", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 30 超卖，卖出建议 > 70 超买", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"kdj_k": {
		Key: "kdj_k", Label: "KDJ-K", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt", "cross_up", "cross_down"},
		Desc: "KDJ随机指标K值 (0-100)，>80超买 <20超卖", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 K<20 超卖区，K上穿D为金叉", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"kdj_d": {
		Key: "kdj_d", Label: "KDJ-D", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "KDJ随机指标D值 (0-100)，K的3日平均", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "D值辅助判断，K>D偏多", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"kdj_j": {
		Key: "kdj_j", Label: "KDJ-J", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "KDJ随机指标J值，=3K-2D，>100超买 <0超卖", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 J<0 超卖，J>100 超买", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"boll_position": {
		Key: "boll_position", Label: "布林位置", Category: "技术面-超买超卖", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "收盘价在布林带中的位置 (0-100)，>80上轨 <20下轨", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 20 触及下轨，卖出建议 > 80 触及上轨", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"boll_width": {
		Key: "boll_width", Label: "布林带宽", Category: "技术面-超买超卖", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "布林带宽度 (上轨-下轨)/中轨*100，值小预示变盘", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 5 带宽收窄变盘在即", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"boll_squeeze": {
		Key: "boll_squeeze", Label: "布林挤压", Category: "技术面-超买超卖", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte"}, Desc: "布林带宽度处于N日最低的百分位，<10预示变盘",
		BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 5 极度收口，变盘在即", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"boll_upper": {
		Key: "boll_upper", Label: "布林上轨", Category: "技术面-超买超卖", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "布林带上轨 = MA20 + 2*标准差", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "收盘价突破上轨可能超买，回踩上轨有支撑", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"boll_middle": {
		Key: "boll_middle", Label: "布林中轨", Category: "技术面-超买超卖", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "布林带中轨 = MA20", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "收盘价>中轨偏多，<中轨偏空", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"boll_lower": {
		Key: "boll_lower", Label: "布林下轨", Category: "技术面-超买超卖", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "布林带下轨 = MA20 - 2*标准差", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "收盘价跌破下轨可能超卖，有反弹预期", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"psy_12": {
		Key: "psy_12", Label: "PSY心理线", Category: "技术面-超买超卖", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "12日心理线 = 上涨天数/12*100，>75超买 <25超卖", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 25 超卖恐慌，卖出建议 > 75 过度乐观", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"psy_ma": {
		Key: "psy_ma", Label: "PSY均线", Category: "技术面-超买超卖", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "PSY的6日均线，用于判断趋势", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "PSY>PSYMA偏多，PSY<PSYMA偏空", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"cci": {
		Key: "cci", Label: "CCI(20)", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "20日商品通道指数，>100超买 <-100超卖", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < -100 超卖区，卖出建议 > 100 超买", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"williams_r": {
		Key: "williams_r", Label: "威廉指标", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "14日威廉指标 (-100~0)，<-80超卖 >-20超买", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < -80 超卖，卖出建议 > -20 超买", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"mfi": {
		Key: "mfi", Label: "MFI资金流量", Category: "技术面-超买超卖", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "14日资金流量指数 (0-100)，结合价量判断", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 20 资金流出超卖，卖出建议 > 80 过热", UseFor: "both", DataSource: "stocks_daily_k",
	},

	// ═══ 技术面 — 量价 ═══
	"volume_ratio": {
		Key: "volume_ratio", Label: "量比(5日)", Category: "技术面-量价", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当日成交量与5日均量之比", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 1.5 放量，< 0.5 缩量", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"volume_ma_ratio": {
		Key: "volume_ma_ratio", Label: "量比(20日)", Category: "技术面-量价", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当日成交量与20日均量之比，>1.5放量", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 1.2，20日均量以上为活跃", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"turnover_rate": {
		Key: "turnover_rate", Label: "换手率", Category: "技术面-量价", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当日换手率 (%)", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 3-10% 活跃，>20% 警惕出货", UseFor: "both", DataSource: "stocks_daily_k",
	},

	// ═══ 技术面 — 波动与结构 ═══
	"atr": {
		Key: "atr", Label: "ATR(14)", Category: "技术面-波动", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "14日平均真实波幅，衡量绝对波动", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议结合价格判断，高ATR需宽止损", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"atr_pct": {
		Key: "atr_pct", Label: "ATR/价格%", Category: "技术面-波动", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "ATR(14)/收盘价*100，标准化波动率", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议结合趋势，高波动率需设宽止损", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"ma_convergence": {
		Key: "ma_convergence", Label: "均线粘合度", Category: "技术面-波动", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "MA5/10/20/60变异系数，<3%高度粘合预示变盘", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 2 均线粘合，即将选择方向", UseFor: "buy", DataSource: "stocks_daily_k",
	},
	"trend_strength": {
		Key: "trend_strength", Label: "趋势强度", Category: "技术面-波动", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "近20日收盘>MA20的天数占比，>0.7强多头", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 0.6，趋势评分越高方向越明确", UseFor: "both", DataSource: "stocks_daily_k",
	},

	// ═══ 技术面 — 趋势系统 ═══
	"adx": {
		Key: "adx", Label: "ADX(14)", Category: "技术面-趋势系统", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "14日平均趋向指数，>25强趋势 <20无趋势", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 25 趋势明确，> 40 极强趋势", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"dmi_plus": {
		Key: "dmi_plus", Label: "DMI+PDI", Category: "技术面-趋势系统", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "14日上升方向线，PDI>MDI多头占优", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 PDI > MDI 多头占优", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"dmi_minus": {
		Key: "dmi_minus", Label: "DMI-MDI", Category: "技术面-趋势系统", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "14日下降方向线，MDI>PDI空头占优", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "卖出建议 MDI > PDI 空头占优", UseFor: "both", DataSource: "stocks_daily_k",
	},

	// ═══ 技术面 — 形态与量价 ═══
	"drawdown_20": {
		Key: "drawdown_20", Label: "20日回撤", Category: "技术面-形态", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "近20个交易日内最大回撤 (%)，负数表示下跌幅度", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < -10% 深度回撤可能反弹", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"new_high_20": {
		Key: "new_high_20", Label: "20日新高", Category: "技术面-形态", Unit: "信号", Type: "number",
		Operators: []string{"gte", "eq"},
		Desc: "当日收盘是否为20日新高，1=是 0=否", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 = 1 突破新高强势", UseFor: "buy", DataSource: "stocks_daily_k",
	},
	"up_days_ratio": {
		Key: "up_days_ratio", Label: "上涨天数占比", Category: "技术面-形态", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "近N日上涨天数/N，>0.6偏多", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 0.6 多头氛围浓", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"price_position_20": {
		Key: "price_position_20", Label: "价格位置(20)", Category: "技术面-形态", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "收盘价在近20日价格区间的相对位置 (0-100)，>80高位", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 20 低位，卖出建议 > 80 高位", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"price_position_60": {
		Key: "price_position_60", Label: "价格位置(60)", Category: "技术面-形态", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "收盘价在近60日价格区间的相对位置 (0-100)", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < 30 中低位，卖出建议 > 80 高位", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"consecutive_days": {
		Key: "consecutive_days", Label: "连续涨跌", Category: "技术面-形态", Unit: "天", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "连续涨跌天数，正=连涨 负=连跌", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 3 连续阳线强势，< -3 超跌", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"gap_pct": {
		Key: "gap_pct", Label: "跳空缺口%", Category: "技术面-形态", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当日开盘相对昨收的跳空幅度%", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < -3% 跳空缺口可关注回补", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"high_low_range": {
		Key: "high_low_range", Label: "日内振幅%", Category: "技术面-形态", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "(最高-最低)/昨收*100，衡量日内波动", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "振幅指标，日内振幅大适合短线，无固定阈值", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"vwap_deviation": {
		Key: "vwap_deviation", Label: "VWAP偏离", Category: "技术面-形态", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "收盘价偏离当日VWAP的%，正值强于均价", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 < -2% 低于均价可能反弹", UseFor: "both", DataSource: "stocks_daily_k",
	},
	"volume_trend": {
		Key: "volume_trend", Label: "量能趋势", Category: "技术面-形态", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "成交量MA5/MA20-1，>0放量趋势", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 0 放量趋势，量涨价增更可靠", UseFor: "buy", DataSource: "stocks_daily_k",
	},
	"index_relative": {
		Key: "index_relative", Label: "大盘相对强度", Category: "技术面-形态", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "个股20日收益-上证20日收益，正值跑赢大盘", BacktestSafe: true, DataNote: "✅ K线衍生，全量历史覆盖",
		Suggestion: "买入建议 > 5 跑赢大盘，说明个股强势", UseFor: "both", DataSource: "stocks_daily_k",
	},

	// ═══ 估值 ═══
	"pe": {
		Key: "pe", Label: "市盈率", Category: "估值", Unit: "倍", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当前市盈率", BacktestSafe: true, DataNote: "📊 ~3500只股票覆盖，2024-07起有历史数据",
		Suggestion: "买入建议 < 20，成长股可适当放宽", UseFor: "both", DataSource: "stocks_daily_indicator",
	},
	"pb": {
		Key: "pb", Label: "市净率", Category: "估值", Unit: "倍", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当前市净率", BacktestSafe: true, DataNote: "📊 ~3500只股票覆盖，2024-07起有历史数据",
		Suggestion: "买入建议 < 3，金融股可参考PB", UseFor: "both", DataSource: "stocks_daily_indicator",
	},
	"ps": {
		Key: "ps", Label: "市销率", Category: "估值", Unit: "倍", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "当前市销率", BacktestSafe: true, DataNote: "📊 ~3500只股票覆盖，2024-07起有历史数据",
		Suggestion: "买入建议 < 2，成长股可适当放宽", UseFor: "both", DataSource: "stocks_daily_indicator",
	},
	"pe_percentile": {
		Key: "pe_percentile", Label: "PE历史分位", Category: "估值", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte"},
		Desc: "当前PE在历史数据中的百分位，<30低估 >70高估", BacktestSafe: true, DataNote: "📊 基于 ~580个交易日历史PE计算，2024-07起可用",
		Suggestion: "买入建议 < 30 历史低位，> 70 历史高位", UseFor: "both", DataSource: "stocks_daily_indicator",
	},
	"pb_percentile": {
		Key: "pb_percentile", Label: "PB历史分位", Category: "估值", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte"},
		Desc: "当前PB在历史数据中的百分位，<30低估 >70高估", BacktestSafe: true, DataNote: "📊 基于 ~580个交易日历史PB计算，2024-07起可用",
		Suggestion: "买入建议 < 30 历史低位，> 70 历史高位", UseFor: "both", DataSource: "stocks_daily_indicator",
	},

	// ═══ 基本面 ═══
	"roe": {
		Key: "roe", Label: "ROE", Category: "基本面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "净资产收益率 (%)", BacktestSafe: true, DataNote: "📊 30只股票覆盖，回测取最近财报",
		Suggestion: "买入建议 > 15%，ROE越高盈利能力越强", UseFor: "buy", DataSource: "stock_financials",
	},
	"revenue_growth": {
		Key: "revenue_growth", Label: "营收增长率", Category: "基本面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "营收同比增长率 (%)", BacktestSafe: true, DataNote: "📊 30只股票覆盖，回测取最近财报",
		Suggestion: "买入建议 > 10%，持续增长为佳", UseFor: "buy", DataSource: "stock_financials",
	},
	"profit_growth": {
		Key: "profit_growth", Label: "利润增长率", Category: "基本面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "归母净利润同比增长率 (%)", BacktestSafe: true, DataNote: "📊 30只股票覆盖，回测取最近财报",
		Suggestion: "买入建议 > 15%，利润增速高更有价值", UseFor: "buy", DataSource: "stock_financials",
	},
	"gross_margin": {
		Key: "gross_margin", Label: "毛利率", Category: "基本面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "销售毛利率 (%)", BacktestSafe: true, DataNote: "📊 30只股票覆盖，回测取最近财报",
		Suggestion: "买入建议 > 30%，高毛利率有定价权", UseFor: "buy", DataSource: "stock_financials",
	},
	"net_margin": {
		Key: "net_margin", Label: "净利率", Category: "基本面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "销售净利率 (%)", BacktestSafe: true, DataNote: "📊 30只股票覆盖，回测取最近财报",
		Suggestion: "买入建议 > 10%，净利率越高盈利质量越好", UseFor: "buy", DataSource: "stock_financials",
	},
	"debt_ratio": {
		Key: "debt_ratio", Label: "资产负债率", Category: "基本面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "资产负债率 (%)", BacktestSafe: true, DataNote: "📊 30只股票覆盖，回测取最近财报",
		Suggestion: "买入建议 < 60%，过高财务风险大", UseFor: "buy", DataSource: "stock_financials",
	},
	"eps": {
		Key: "eps", Label: "每股收益", Category: "基本面", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "基本每股收益 (元)", BacktestSafe: true, DataNote: "📊 30只股票覆盖，回测取最近财报",
		Suggestion: "买入建议 > 0.5 元且持续增长", UseFor: "buy", DataSource: "stock_financials",
	},

	// ═══ 资金面/市场面 ═══
	"total_market_cap": {
		Key: "total_market_cap", Label: "总市值", Category: "资金面", Unit: "元", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "总市值 (元，数据库原始单位)", BacktestSafe: false, DataNote: "⚠️ 仅最近2天数据，回测取最近可用值",
		Suggestion: "无固定阈值，大盘股 > 1000亿 稳健，小盘股 < 100亿 弹性大", UseFor: "both", DataSource: "stocks_daily_indicator",
	},
	"shareholder_change": {
		Key: "shareholder_change", Label: "股东户数变化", Category: "资金面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "股东户数环比变化%，负值表示筹码集中", BacktestSafe: true, DataNote: "📊 53只股票覆盖，回测取最近报告",
		Suggestion: "买入建议 < -5% 筹码集中，> 10% 筹码分散警惕", UseFor: "buy", DataSource: "stock_shareholders",
	},
	"inst_hold_ratio": {
		Key: "inst_hold_ratio", Label: "机构持股比", Category: "资金面", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "机构持股占流通股比例 (%)", BacktestSafe: true, DataNote: "📊 53只股票覆盖，回测取最近报告",
		Suggestion: "买入建议 > 30%，机构持股比例高更受认可", UseFor: "buy", DataSource: "stock_shareholders",
	},

	// ═══ 预测 (纯未来数据) ═══
	"prediction_upside": {
		Key: "prediction_upside", Label: "预测上涨空间", Category: "预测", Unit: "%", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "多模型预测均价相对现价的上涨空间 (%)", BacktestSafe: false, DataNote: "🚫 预测为未来数据，回测不可用",
		Suggestion: "买入建议 > 10%，预测上涨空间越大越好", UseFor: "buy", DataSource: "ai_stock_predictions",
	},
	"prediction_consensus": {
		Key: "prediction_consensus", Label: "预测一致性", Category: "预测", Unit: "比值", Type: "number",
		Operators: []string{"gte", "lte", "gt", "lt"},
		Desc: "预测看涨的模型占比 (0-1)，>0.5多数看涨", BacktestSafe: false, DataNote: "🚫 预测为未来数据，回测不可用",
		Suggestion: "买入建议 > 0.6，多数模型看涨为佳", UseFor: "buy", DataSource: "ai_stock_predictions",
	},
}

// GetIndicatorMeta returns metadata for a given indicator key.
func GetIndicatorMeta(key string) *IndicatorMeta {
	return IndicatorRegistry[key]
}

// IsBacktestSafe returns whether the indicator can be used in backtesting.
func IsBacktestSafe(key string) bool {
	m, ok := IndicatorRegistry[key]
	return ok && m.BacktestSafe
}

// AllIndicators returns all indicators, optionally filtered by category.
func AllIndicators(category string) []*IndicatorMeta {
	result := make([]*IndicatorMeta, 0, len(IndicatorRegistry))
	for _, m := range IndicatorRegistry {
		if category == "" || m.Category == category {
			result = append(result, m)
		}
	}
	return result
}

// GetIndicatorsByUseFor returns indicators filtered by use case (buy/sell/both).
func GetIndicatorsByUseFor(useFor string) []*IndicatorMeta {
	result := make([]*IndicatorMeta, 0)
	for _, m := range IndicatorRegistry {
		if m.UseFor == useFor || m.UseFor == "both" {
			result = append(result, m)
		}
	}
	return result
}

// GetIndicatorDataSource returns the data source table for an indicator.
func GetIndicatorDataSource(indicator string) string {
	if m, ok := IndicatorRegistry[indicator]; ok {
		return m.DataSource
	}
	return "unknown"
}

// IsKlineDerived returns whether the indicator is derived from daily K-line data.
func IsKlineDerived(indicator string) bool {
	if m, ok := IndicatorRegistry[indicator]; ok {
		return m.DataSource == "stocks_daily_k"
	}
	return false
}

// IsIndicatorInRegistry returns true if the indicator key is registered.
func IsIndicatorInRegistry(key string) bool {
	_, ok := IndicatorRegistry[key]
	return ok
}
