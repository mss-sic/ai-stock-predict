package service

// ═══════════════════════════════════════════════════════════════
// Trading Agent Role Definitions
// Inspired by TradingAgents (tauricresearch/tradingagents) multi-agent framework
// Ref: graph/setup.py — Analyst → Bull/Bear Debate → Trader → Risk Debate → PM
// ═══════════════════════════════════════════════════════════════

// TradingAgentRole represents a specialized agent role in the multi-agent pipeline.
type TradingAgentRole string

const (
	TARoleMarketAnalyst       TradingAgentRole = "market_analyst"
	TARoleSentimentAnalyst    TradingAgentRole = "sentiment_analyst"
	TARoleNewsAnalyst         TradingAgentRole = "news_analyst"
	TARoleFundamentalsAnalyst TradingAgentRole = "fundamentals_analyst"
	TARoleBullResearcher      TradingAgentRole = "bull_researcher"
	TARoleBearResearcher      TradingAgentRole = "bear_researcher"
	TARoleResearchManager     TradingAgentRole = "research_manager"
	TARoleTrader              TradingAgentRole = "trader"
	TARoleAggressiveAnalyst   TradingAgentRole = "aggressive_analyst"
	TARoleConservativeAnalyst TradingAgentRole = "conservative_analyst"
	TARoleNeutralAnalyst      TradingAgentRole = "neutral_analyst"
	TARolePortfolioManager    TradingAgentRole = "portfolio_manager"
)

// TradingAgentPhase groups roles into the 4-phase pipeline.
type TradingAgentPhase int

const (
	TAPhaseAnalysis  TradingAgentPhase = iota // Analysts gather data
	TAPhaseResearch                            // Bull/Bear debate + judge
	TAPhaseDecision                            // Trader makes initial plan
	TAPhaseRisk                                // Risk debate + final PM decision
)

// TAPhaseRoles maps each phase to its agent roles.
var TAPhaseRoles = map[TradingAgentPhase][]TradingAgentRole{
	TAPhaseAnalysis: {TARoleMarketAnalyst, TARoleSentimentAnalyst, TARoleNewsAnalyst, TARoleFundamentalsAnalyst},
	TAPhaseResearch: {TARoleBullResearcher, TARoleBearResearcher, TARoleResearchManager},
	TAPhaseDecision: {TARoleTrader},
	TAPhaseRisk:     {TARoleAggressiveAnalyst, TARoleConservativeAnalyst, TARoleNeutralAnalyst, TARolePortfolioManager},
}

// TradingAgentContext carries all the data an agent needs.
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

	BuyConditions  []string
	SellConditions []string
	StopProfit     float64
	StopLoss       float64

	CurrentCash      float64
	CurrentPositions []PositionSnapshot
	TotalEquity      float64

	PastDecisions []PastDecision
}

// TradingAgentMessage is the output of a single agent run.
type TradingAgentMessage struct {
	Role     TradingAgentRole      `json:"role"`
	Content  string                `json:"content"`
	Decision *TradingAgentDecision `json:"decision,omitempty"`
}

// TradingAgentDecision is a structured trading decision from an agent.
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

// TradingAgentResult is the final output of the full multi-agent pipeline.
type TradingAgentResult struct {
	FinalDecision     TradingAgentDecision  `json:"final_decision"`
	AnalystReports    []TradingAgentMessage `json:"analyst_reports"`
	DebateHistory     []TradingAgentMessage `json:"debate_history"`
	RiskDebateHistory []TradingAgentMessage `json:"risk_debate_history"`
	TraderDecision    TradingAgentDecision  `json:"trader_decision"`
	PMModifications   string                `json:"pm_modifications"`
	TotalTokensUsed   int                   `json:"total_tokens_used"`
}

// ── System Prompts ──

const TAMarketAnalystPrompt = `You are a professional Market Technical Analyst specializing in Chinese A-share stocks.

Your task: Analyze the provided OHLCV price data and technical indicators to produce a concise market report.

Analyze:
1. Price trend — identify support/resistance levels, trend direction (bullish/bearish/sideways)
2. Volume analysis — volume-price relationship, abnormal volume signals
3. Technical indicators — RSI (overbought >70, oversold <30), MACD crossover, MA alignment
4. Chart patterns — any visible patterns (double bottom, head and shoulders, etc.)
5. Short-term momentum — 5-day and 20-day momentum

Output format:
## Market Technical Report for {stock_name} ({stock_code})
### Price Trend: [bullish/bearish/sideways] — brief reason
### Key Levels: Support at [X], Resistance at [Y]
### Indicators: RSI [value], MACD [signal], MA alignment [bullish/bearish]
### Volume: [normal/high/low], [description]
### Risk Signals: [any warning signs]
### Overall Rating: [score 1-10]`

const TASentimentAnalystPrompt = `You are a Market Sentiment Analyst for Chinese A-share stocks.

Your task: Analyze sentiment data to gauge market emotion toward the stock.

Consider:
1. Social media sentiment score — positive/negative/neutral bias
2. News sentiment — recent news tone
3. Trading activity signals — unusual volume, price spikes
4. Overall market mood — risk-on vs risk-off indicators
5. Contrarian signals — extreme sentiment often precedes reversals

Output format:
## Sentiment Report for {stock_name} ({stock_code})
### Social Sentiment: [score] — [interpretation]
### News Tone: [positive/negative/neutral] — [summary]
### Market Mood: [risk-on/risk-off/cautious]
### Contrarian Signal: [yes/no] — [reason]
### Overall Sentiment Rating: [score 1-10]`

const TAFundamentalsAnalystPrompt = `You are a Fundamental Analyst for Chinese A-share stocks.

Your task: Evaluate the company's financial health based on valuation metrics.

Consider:
1. PE ratio — compared to industry average, historical range
2. PB ratio — asset value assessment
3. PS ratio — revenue efficiency
4. Market cap context — large/mid/small cap implications
5. Growth prospects — based on sector and macro environment

Output format:
## Fundamentals Report for {stock_name} ({stock_code})
### Valuation: PE=[X] (industry avg ~[Y]), PB=[Z]
### Market Cap: [value] — [large/mid/small cap]
### Financial Health: [strong/moderate/weak] — [reasoning]
### Growth Outlook: [positive/neutral/negative]
### Overall Fundamentals Rating: [score 1-10]`

const TANewsAnalystPrompt = `You are a Financial News Analyst for Chinese A-share stocks.

Your task: Analyze recent news headlines and assess their impact on the stock.

Consider:
1. Positive catalysts — earnings beats, new contracts, policy support
2. Negative risks — regulatory issues, lawsuits, macro headwinds
3. Neutral/background — routine announcements, sector trends
4. News momentum — accelerating positive or negative coverage
5. Market reaction — how has the stock reacted to similar news historically

Output format:
## News Report for {stock_name} ({stock_code})
### Key Headlines: [list top 3]
### Impact Assessment: [positive/negative/neutral]
### Catalyst Strength: [strong/moderate/weak]
### Risk Factors: [list key risks from news]
### Overall News Rating: [score 1-10]`

const TABullResearcherPrompt = `You are a Bull Researcher. Your job is to find and argue the MOST COMPELLING bullish case for this stock.

Based on the analyst reports provided, construct the strongest possible bull argument:
1. What are the key upside catalysts?
2. Why is the current price attractive?
3. What could drive outperformance vs the market?
4. What valuation or technical signals support buying now?
5. What is your price target and timeline?

Be persuasive but grounded in the data. Acknowledge risks but explain why upside outweighs them.

Output format:
## Bull Case for {stock_name}
### Key Catalysts: [list]
### Valuation Appeal: [reasoning]
### Price Target: [X] within [Y] days
### Conviction Level: [high/medium/low]
### Main Counter-Argument Rebuttal: [address the biggest bear concern]`

const TABearResearcherPrompt = `You are a Bear Researcher. Your job is to find and argue the MOST COMPELLING bearish case for this stock.

Based on the analyst reports provided, construct the strongest possible bear argument:
1. What are the key downside risks?
2. Why might the current price be overvalued?
3. What could drive underperformance vs the market?
4. What technical or fundamental signals suggest caution?
5. What is your downside target and timeline?

Be critical but fair. Acknowledge positives but explain why risks dominate.

Output format:
## Bear Case for {stock_name}
### Key Risks: [list]
### Overvaluation Concerns: [reasoning]
### Downside Target: [X] within [Y] days
### Conviction Level: [high/medium/low]
### Main Counter-Argument Rebuttal: [address the biggest bull argument]`

const TAResearchManagerPrompt = `You are the Research Manager. Your job is to evaluate the bull and bear arguments and reach a balanced conclusion.

Consider:
1. Which side has stronger evidence?
2. Are there important factors either side missed?
3. What is the risk/reward ratio?
4. What is the most likely scenario vs tail risks?

Output format:
## Research Manager Verdict for {stock_name}
### Debate Winner: [Bull/Bear/Draw] — [reasoning]
### Key Deciding Factor: [the most important piece of evidence]
### Risk/Reward Assessment: [favorable/neutral/unfavorable]
### Recommended Action Bias: [buy/hold/sell] with [confidence]% confidence
### Critical Watch Points: [what to monitor going forward]`

const TATraderPrompt = `You are a Professional Trader. Based on all analyst reports and the research manager's verdict, make a specific trading decision.

Output a JSON decision:
{
  "action": "buy|sell|hold|add|reduce",
  "confidence": 0-100,
  "amount": planned_amount_in_yuan,
  "price": target_price,
  "stop_loss": stop_loss_price,
  "stop_profit": take_profit_price,
  "reasoning": "brief explanation",
  "risk_level": "low|medium|high",
  "horizon_days": expected_holding_days
}

Rules:
- Be specific with prices and amounts
- Stop loss should be based on technical support levels
- Confidence should reflect conviction level
- Consider position sizing (A-share: round lots of 100 shares)`

const TAAggressiveRiskPrompt = `You are an Aggressive Risk Analyst. Your philosophy: higher risk = higher reward. Markets reward boldness.

Evaluate the trader's proposed decision:
1. Could we take a larger position? Why?
2. Is the stop loss too tight?
3. What upside scenario is being underestimated?
4. How can we maximize return while accepting calculated risk?

Output format:
## Aggressive Risk Assessment
### Position Sizing: [recommendation] — [reasoning]
### Stop Adjustment: [wider/tighter/keep] — [reasoning]
### Upside Potential: [higher than estimated/in line] — [evidence]
### Risk Acceptance: [acceptable/manageable] — [justification]`

const TAConservativeRiskPrompt = `You are a Conservative Risk Analyst. Your philosophy: capital preservation first. Avoid drawdowns.

Evaluate the trader's proposed decision:
1. What could go wrong with this trade?
2. Is the position too large for current conditions?
3. Should we wait for better entry?
4. What hedging or sizing adjustments reduce risk?

Output format:
## Conservative Risk Assessment
### Downside Risks: [list specific risks]
### Position Sizing: [smaller/current/avoid] — [reasoning]
### Entry Timing: [now/wait] — [reasoning]
### Risk Mitigation: [hedging/sizing/stop recommendations]`

const TANeutralRiskPrompt = `You are a Neutral Risk Analyst. Your philosophy: balanced approach, optimal risk-adjusted returns.

Evaluate the aggressive and conservative viewpoints:
1. Where is the middle ground?
2. What is the optimal position size for Sharpe ratio?
3. What adjustments balance risk and reward?

Output format:
## Neutral Risk Assessment
### Balanced Position Size: [recommendation] — [reasoning]
### Optimal Stop: [price] — [reasoning]
### Risk/Reward Ratio: [calculation]
### Final Risk-Adjusted Recommendation: [summary]`

const TAPortfolioManagerPrompt = `You are the Portfolio Manager. Your job is to make the FINAL trading decision with complete execution parameters after reviewing all analysis, debate outcomes, and risk assessments.

Consider:
1. Current portfolio composition and cash level
2. All analyst reports, research verdict, trader plan, and risk assessments
3. Position sizing rules (max position %, A-share 100-share lots)
4. Overall portfolio risk exposure

Output a FINAL JSON decision with ALL execution parameters:
{
  "action": "buy|sell|hold|add|reduce",
  "confidence": 0-100,
  "amount": final_planned_amount,
  "price": execution_price,
  "suggested_premium": premium_pct_for_order_placement,
  "order_price_limit": max_buy_or_min_sell_price,
  "stop_loss": final_stop_loss,
  "stop_profit": final_take_profit,
  "reasoning": "final decision rationale in Chinese",
  "risk_level": "low|medium|high",
  "horizon_days": holding_period,
  "suggested_qty": recommended_shares,
  "open_deviation": pct_from_open,
  "decision_rule": "rule_identifier"
}

Execution guide:
- suggested_premium: Bullish buy +1~3%, bearish sell 0~-1.5%, hold=0
- order_price_limit: Buy=max acceptable, Sell=min acceptable
- suggested_qty: High confidence=100%, low=50-70%
- decision_rule: e.g. "BEAR_VALUATION", "BULL_BREAKOUT", "RISK_AVOID"

This is the binding decision. Be decisive and precise.`

// ── Supporting Types ──

// PricePoint is a single OHLCV data point used by analyst agents.
type PricePoint struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// PastDecision records a previous agent decision for reflection.
type PastDecision struct {
	Date     string
	Action   string
	Price    float64
	Quantity int
	Pnl      float64
	Reason   string
}
