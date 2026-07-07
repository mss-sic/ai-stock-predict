package service

import (
	"strings"
)

// PromptBuilderService builds structured prompts for AI-driven strategy generation.
// Encapsulates the template construction logic previously embedded in handler methods.
type PromptBuilderService struct{}

// NewPromptBuilderService creates a new PromptBuilderService.
func NewPromptBuilderService() *PromptBuilderService {
	return &PromptBuilderService{}
}

// PromptContext holds all parameters for building a strategy generation prompt.
type PromptContext struct {
	SystemPrompt  string // from ai_system_config
	Indicators    string // indicator reference table
	StrategyName  string
	Description   string
	Style         string // aggressive / moderate / conservative
}

// IndicatorRules returns the standard condition construction rules for AI prompts.
func (s *PromptBuilderService) IndicatorRules() string {
	return `## 条件构建规范

请严格根据「可用指标参考」表构建每一条条件：

### 字段映射规则
- indicator → 必须使用参考表「字段名」列的值（如 algo_score、rsi、daily_change）
- operator → 必须使用参考表「可用操作符」列中的某一个，英文映射为：
  ≥→gte, ≤→lte, >→gt, <→lt, =→eq, ↑上穿→cross_up, ↓下穿→cross_down
- value → 必须使用参考表「值域」列建议的数值范围
- condType → 参考表「用途」列：买入→buy, 卖出→sell, 买卖→两者均可

### 类型特殊规则
- cross 类型（ma_cross/ema_cross/macd）：operator 只能用 cross_up 或 cross_down，value 为 "短/长" 如 "5/20"
- 评分类型（algo_score/ai_*）：value 0-10，建议买入 ≥6、卖出 ≤3
- RSI/KDJ：value 0-100，超买>70 卖出、超卖<30 买入
- pe/pb：单位是倍，<20低估可买入，>50高估考虑卖出
- % 单位指标：value 直接写数字如 5 表示 5%
- 元单位指标（ma_*/boll_*/macd_*）：value 是股价绝对值
- 信号/比值型（new_high_20/volume_trend）：value 为 0 或 1

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
}

// DefaultSystemPrompt returns the default system prompt for strategy generation.
func (s *PromptBuilderService) DefaultSystemPrompt() string {
	return `你是量化策略专家。根据用户描述生成A股策略JSON。
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

// BuildStrategyPrompt constructs the full AI prompt for strategy generation.
func (s *PromptBuilderService) BuildStrategyPrompt(ctx PromptContext) string {
	basePrompt := ctx.SystemPrompt
	if basePrompt == "" {
		basePrompt = s.DefaultSystemPrompt()
	}

	fullPrompt := basePrompt + "\n\n" + s.IndicatorRules() +
		"\n\n用户策略名: __STRATEGY_NAME__\n用户描述: __STRATEGY_DESC__\n风险偏好: __STRATEGY_STYLE__"

	return s.renderTemplate(fullPrompt, ctx)
}

// BuildAnalysisPrompt constructs a prompt for AI stock analysis.
func (s *PromptBuilderService) BuildAnalysisPrompt(systemPrompt, code, contextData, question string) string {
	prompt := systemPrompt
	if prompt == "" {
		prompt = "你是专业的股票分析师，请根据提供的股票数据进行分析。"
	}
	return prompt + "\n\n股票代码: " + code + "\n" + contextData + "\n\n用户问题: " + question
}

// BuildReviewPrompt constructs a prompt for AI backtest review.
func (s *PromptBuilderService) BuildReviewPrompt(systemPrompt string, strategyName string, performanceSummary string) string {
	prompt := systemPrompt
	if prompt == "" {
		prompt = "你是量化策略评审专家，请对策略回测结果进行专业评审。"
	}
	return prompt + "\n\n策略名称: " + strategyName + "\n\n回测表现:\n" + performanceSummary +
		"\n\n请从胜率、最大回撤、夏普比率、持仓集中度等维度进行分析，给出改进建议。"
}

// renderTemplate replaces placeholders in a template string.
func (s *PromptBuilderService) renderTemplate(template string, ctx PromptContext) string {
	result := template
	result = strings.ReplaceAll(result, "__INDICATORS__", ctx.Indicators)
	result = strings.ReplaceAll(result, "__STRATEGY_NAME__", ctx.StrategyName)
	result = strings.ReplaceAll(result, "__STRATEGY_DESC__", ctx.Description)
	result = strings.ReplaceAll(result, "__STRATEGY_STYLE__", ctx.Style)
	return result
}
