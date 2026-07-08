package service

// ═══════════════════════════════════════════════════════════════
// Trading Agent v2: Objective Analysis + Strategy-Bridged Decision
//
// Architecture:
//   Phase 1: 3 Analysts (objective, no strategy bias)
//   Phase 2: Portfolio Manager (bridges strategy intent ↔ objective data)
//
// Removed: bull/bear debate, research manager, risk analyst triad
// Rationale: forced adversarial debate creates noise, not insight.
//   PM alone bridges strategy mandate with objective analysis.
// ═══════════════════════════════════════════════════════════════

// TradingAgentRole represents a specialized agent role.
type TradingAgentRole string

const (
	TARoleTechnicalAnalyst   TradingAgentRole = "technical_analyst"
	TARoleFundamentalAnalyst TradingAgentRole = "fundamental_analyst"
	TARoleMarketAnalyst      TradingAgentRole = "market_analyst" // sentiment + macro context
	TARolePortfolioManager   TradingAgentRole = "portfolio_manager"
)

// TradingAgentContext carries all data an agent needs.
type TradingAgentContext struct {
	StockCode    string
	StockName    string
	TradeDate    string
	CurrentPrice float64

	RecentPrices []PricePoint
	Indicators   map[string]float64

	PE        float64
	PB        float64
	PS        float64
	MarketCap float64

	SocialSentiment float64
	NewsSentiment   float64
	NewsHeadlines   []string

	// Financial data (from stock_financials)
	NetProfit        float64 // 净利润（最近报告期）
	TotalRevenue     float64 // 营业总收入
	TotalAssets      float64 // 总资产
	TotalLiabilities float64 // 总负债
	DebtRatio        float64 // 资产负债率
	EPS              float64 // 每股收益
	BVPS             float64 // 每股净资产
	IndustryPE       float64 // 行业平均PE
	IndustryPB       float64 // 行业平均PB

	BuyConditions  []string
	SellConditions []string
	StopProfit     float64
	StopLoss       float64

	CurrentCash      float64
	CurrentPositions []PositionSnapshot
	TotalEquity      float64

	// ── Strategy Profile (NEW) ──
	Strategy StrategyProfile
}

// StrategyProfile captures the strategy's intent and risk parameters.
// This is the bridge between "what the strategy wants" and "what the market offers."
type StrategyProfile struct {
	Name            string // "3日高动量追击"
	Style           string // momentum_chaser / swing_trader / trend_follower / value_hunter / dip_buyer
	HoldDays        int    // 目标持仓天数
	RiskProfile     string // aggressive / balanced / conservative
	Thesis          string // 用户自述策略理念
	StopLoss        float64
	StopProfit      float64
	PositionSizing  string
	BuyPositionPct  float64
	MaxHoldings     int
}

// TradingAgentMessage is the output of a single agent run.
type TradingAgentMessage struct {
	Role    TradingAgentRole `json:"role"`
	Content string           `json:"content"`
}

// TradingAgentDecision is a structured trading decision.
type TradingAgentDecision struct {
	Action           string  `json:"action"`
	Confidence       float64 `json:"confidence"`
	Amount           float64 `json:"amount"`
	Price            float64 `json:"price"`
	SuggestedPremium float64 `json:"suggested_premium"`
	OrderPriceLimit  float64 `json:"order_price_limit"`
	StopLoss         float64 `json:"stop_loss"`
	StopProfit       float64 `json:"stop_profit"`
	Reasoning        string  `json:"reasoning"`
	RiskLevel        string  `json:"risk_level"`
	HorizonDays      int     `json:"horizon_days"`
	SuggestedQty     int     `json:"suggested_qty"`
	OpenDeviation    float64 `json:"open_deviation"`
	DecisionRule     string  `json:"decision_rule"`
}

// TradingAgentResult is the final output of the pipeline.
type TradingAgentResult struct {
	FinalDecision   TradingAgentDecision  `json:"final_decision"`
	AnalystReports  []TradingAgentMessage `json:"analyst_reports"`
	PMReasoning     string                `json:"pm_reasoning"`
	TotalTokensUsed int                   `json:"total_tokens_used"`
}

// ── Analyst Prompts (Objective, No Strategy Bias) ──

const TATechnicalPrompt = `You are a Technical Analyst for Chinese A-share stocks. 
Answer ONLY based on the price/volume data provided. Do NOT recommend buy/sell.

Analyze:
1. Trend — direction, strength, phase (early/mid/late)
2. Support/Resistance — nearest key levels with price distance
3. Volume — abnormal? climax? drying up? relationship to price
4. Momentum — accelerating or decelerating? divergence?

Output format:
## Technical Analysis
### Trend: [direction] [strength] [phase]
### Key Levels: Support ¥[S] ([-X]%), Resistance ¥[R] ([+Y]%)
### Volume: [description]
### Momentum: [description]
### Risk Signals: [any technical warnings — gap risk, breakdown risk, overextension]`

const TAFundamentalPrompt = `You are a Fundamental Analyst for Chinese A-share stocks.
Answer ONLY based on the financial data provided. Do NOT recommend buy/sell.

Analyze:
1. Valuation — PE/PB vs industry and historical range
2. Financial health — profitability, debt, cash flow indicators
3. Market cap context — liquidity, institutional coverage implications
4. Growth trajectory — revenue/earnings trend

Output format:
## Fundamental Analysis
### Valuation: PE=[X] (industry [Y]), PB=[Z]
### Market Cap: [value] — [liquidity assessment]
### Financial Health: [assessment]
### Growth: [trajectory description]`

const TAMarketContextPrompt = `You are a Market Context Analyst for Chinese A-share stocks.
Answer ONLY based on the macro/sentiment/flow data provided. Do NOT recommend buy/sell.

Analyze:
1. Market sentiment — risk appetite, fear/greed indicators
2. Sector/Industry flow — capital rotation, sector strength
3. News catalysts — positive/negative headlines and their materiality
4. Northbound flow — foreign capital direction

Output format:
## Market Context
### Sentiment: [risk-on/neutral/risk-off] — [evidence]
### Sector Position: [leading/lagging/neutral] in current market
### News Impact: [catalyst description or "no material news"]
### Capital Flow: [description]`

// ── PM Prompt (Strategy Bridge) ──

const TAPMStrategyBridgePrompt = `You are a Portfolio Manager for a Chinese A-share quantitative strategy.

## YOUR STRATEGY MANDATE
- Strategy Name: {strategy_name}
- Strategy Style: {strategy_style}
- Strategy Thesis: {strategy_thesis}
- Target Holding Period: {hold_days} days
- Risk Profile: {risk_profile}
- Stop Loss: {stop_loss}% | Stop Profit: {stop_profit}%
- Position Sizing: {position_sizing}, {buy_pct}% per trade

## THE SIGNAL
- Stock: {stock_name} ({stock_code})
- Signal: {signal_action}
- Trigger Reason: {signal_reason}
- Signal Price: ¥{signal_price}

## OBJECTIVE ANALYSIS
{analyst_reports}

## PORTFOLIO STATE
{portfolio_state}

## STRATEGY-STYLE TOLERANCE FRAMEWORK

Your judgment standards MUST align with the strategy style. Below are the EXPECTED and FATAL conditions per style:

### 动量追击 (momentum_chaser) — 追涨强势股，1-3天短线
| 指标 | 预期范围(不构成驳回) | 警告范围(需关注) | 致命范围(可驳回) |
|------|---------------------|-----------------|-----------------|
| RSI(14) | 60-85 | 85-95 | >95（极端超买） |
| 乖离率(MA20) | +3%~+12% | +12%~+18% | >+20% 或 <-5% |
| 换手率 | 5%-25% | 25%-40% | <3%（无资金关注） |
| 成交量比(20日均) | 1.2x-4x | 4x-8x | <0.8x（缩量） |
| 近5日涨幅 | +5%~+25% | +25%~+40% | <+3%（动能不足） |
| 关键原则 | 高位 + 高量 = 动量确认，非风险 | 缩量新高 = 警示 | 基本面恶化 = 无关 |

### 波段交易 (swing_trader) — 高抛低吸，3-10天
| 指标 | 预期范围 | 警告范围 | 致命范围 |
|------|---------|---------|---------|
| RSI(14) | 30-70 | >75 或 <25 | >85 或 <15（极端） |
| 乖离率(MA20) | -5%~+8% | +8%~+15% 或 -8%~-15% | >+18% 或 <-18% |
| 布林带位置 | 中轨附近 | 触及上/下轨 | 持续沿轨运行(趋势) |
| 波动率 | 中等(20-40%年化) | >50% | <15%(无波段空间) |
| 关键原则 | 均值回归是核心逻辑 | 趋势行情不适合波段 | 突发消息不否定波段 |

### 趋势跟随 (trend_follower) — 顺势而为，10-30天
| 指标 | 预期范围 | 警告范围 | 致命范围 |
|------|---------|---------|---------|
| MA排列 | 多头排列(短期>长期) | 均线粘合 | 空头排列 |
| ADX | >25 | 20-25 | <20（无趋势） |
| MACD | 金叉/零轴上 | 死叉但未破位 | 零轴下+死叉 |
| 回撤幅度 | <10% | 10%-15% | >15% 或破关键均线 |
| 关键原则 | 趋势不破则持有 | 震荡不否定趋势 | 基本面变化可否定 |

### 价值挖掘 (value_hunter) — 低估标的，20-60天
| 指标 | 预期范围 | 警告范围 | 致命范围 |
|------|---------|---------|---------|
| PE(ttm) | <行业均值70% | 亏损（周期底部） | 亏损+无改善预期 |
| PB | <1.5 | <0.8（可能有雷） | 持续恶化 |
| 股息率 | >2% | 1%-2% | <1%+高负债 |
| 短期走势 | 弱势/横盘 | 急跌>15% | 急跌+基本面恶化 |
| 关键原则 | 低估 ≠ 买入信号 | 等待催化剂 | 价值陷阱 = 驳回 |

### 抄底反弹 (dip_buyer) — 超跌买入，1-5天
| 指标 | 预期范围 | 警告范围 | 致命范围 |
|------|---------|---------|---------|
| 近10日跌幅 | -10%~-25% | -25%~-40% | >-40% 或 <-10%（不够超跌） |
| RSI(14) | 20-35 | 15-20 或 35-40 | >45（反弹已启动） |
| 乖离率(MA20) | -8%~-20% | -20%~-30% | >-5% 或 <-35% |
| 成交量 | 缩量止跌或放量反弹 | 持续缩量阴跌 | 放量暴跌 |
| 关键原则 | 超跌≠抄底，需要止跌信号 | 利空出尽 = 加分 | 财务造假 = 一票否决 |

### 网格做T (grid_trader) — 震荡市低买高卖，1-10天
| 指标 | 预期范围 | 警告范围 | 致命范围 |
|------|---------|---------|---------|
| 波动率 | 15%-35%年化 | 35%-50% | <12%(无网格空间) |
| ADX | <20 | 20-25 | >30(强趋势不适合网格) |
| 振幅(日均) | 2%-6% | 6%-10% | <1.5% |
| 布林带宽度 | 收缩/正常 | 扩张初期 | 持续扩张(趋势中) |
| 关键原则 | 震荡确认 = 可行 | 突破区间 = 暂停 | 单边趋势 = 驳回 |

## DECISION FRAMEWORK

**Step 1 — Strategy Alignment (use tolerance table above)**
Check analyst findings against your strategy's tolerance table:
- All indicators in "预期范围" → STRONG ALIGNMENT (confidence 80+)
- Some in "警告范围", none in "致命" → MODERATE ALIGNMENT (confidence 50-79)
- Any in "致命范围" → FATAL CONTRADICTION → REJECT
- Strategy-irrelevant warnings (e.g. PE for momentum) → IGNORE

**Step 2 — Risk/Reward Under Strategy Constraints**
Given {hold_days}-day horizon and {risk_profile} risk profile:
- Upside: based on technical resistance levels + strategy thesis
- Downside: based on support levels, capped by stop_loss={stop_loss}%
- R/R ratio: acceptable for your strategy style?

**Step 3 — Execution Decision**
- STRONG ALIGNMENT + acceptable R/R → CONFIRM with high confidence
- MODERATE ALIGNMENT + acceptable R/R → CONFIRM with moderate confidence
- Any FATAL CONTRADICTION → REJECT with specific reason
- Insufficient data to judge → REJECT with "INSUFFICIENT_DATA"

**Principles:**
1. Your job is to EXECUTE the strategy, not override it with generic investing wisdom.
2. A strategy's "expected behavior" is NOT a risk — it's confirmation the strategy is working.
3. Only FATAL contradictions (core thesis broken) warrant rejection.
4. When in doubt between "warning" and "fatal", lean toward confirming — the strategy's stop-loss protects against actual losses.

Output ONLY the final JSON decision (no markdown, no explanation outside JSON):
{
  "action": "{signal_action}|hold",
  "confidence": 0-100,
  "amount": planned_amount,
  "price": execution_price,
  "suggested_premium": premium_pct,
  "order_price_limit": limit_price,
  "stop_loss": stop_loss_price,
  "stop_profit": take_profit_price,
  "reasoning": "decision rationale in Chinese, reference which tolerance indicators drove the decision",
  "risk_level": "low|medium|high",
  "horizon_days": {hold_days},
  "suggested_qty": shares,
  "open_deviation": pct_from_open,
  "decision_rule": "STRATEGY_ALIGNED|TOLERABLE_CONCERN|FATAL_CONTRADICTION|INSUFFICIENT_DATA"
}

Pricing guide:
- suggested_premium: buy +0.5~2% (momentum: higher end, value: lower end), sell 0~-1%, hold=0
- order_price_limit: buy=max acceptable price, sell=min acceptable price
- decision_rule: STRATEGY_ALIGNED=all indicators in expected range; TOLERABLE_CONCERN=some warnings but no fatal; FATAL_CONTRADICTION=core premise broken; INSUFFICIENT_DATA=not enough data to decide`

// ── Supporting Types ──
type PricePoint struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}
