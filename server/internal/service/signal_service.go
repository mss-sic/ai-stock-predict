package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/ai-stock-predict/server/internal/model"
)

// SignalService provides condition evaluation and signal generation logic.
// Pure business logic — no direct database access.
type SignalService struct{}

// NewSignalService creates a new SignalService.
func NewSignalService() *SignalService { return &SignalService{} }

// SignalResult describes the outcome of evaluating buy/sell conditions for a stock.
type ConditionEvalResult struct {
	Passed  bool
	Details []string
}

// GroupConditions groups strategy conditions by their LogicGroup.
func (s *SignalService) GroupConditions(conds []model.StrategyCondition) map[int][]model.StrategyCondition {
	groups := make(map[int][]model.StrategyCondition)
	for _, c := range conds {
		if c.Enabled {
			groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
		}
	}
	return groups
}

// EvaluateGroup checks if all conditions in a logic group are met (AND logic).
func (s *SignalService) EvaluateGroup(group []model.StrategyCondition, indicatorValues map[string]float64) (bool, []string) {
	details := make([]string, 0, len(group))
	for _, c := range group {
		val, ok := indicatorValues[c.Indicator]
		if !ok {
			details = append(details, c.Indicator+"=N/A")
			return false, details
		}
		passed := checkOperator(val, c.Operator, c.Value)
		detail := fmt.Sprintf("%s %s=%.2f %s %.2f",
			boolIcon(passed), c.Indicator, val, c.Operator, c.Value)
		details = append(details, detail)
		if !passed {
			return false, details
		}
	}
	return true, details
}

// EvaluateAnyGroup checks groups with OR logic between groups.
// Returns true if ANY group fully passes (all conditions within group pass).
func (s *SignalService) EvaluateAnyGroup(groups map[int][]model.StrategyCondition, indicatorValues map[string]float64) ConditionEvalResult {
	for _, group := range groups {
		passed, details := s.EvaluateGroup(group, indicatorValues)
		if passed {
			return ConditionEvalResult{Passed: true, Details: details}
		}
	}
	return ConditionEvalResult{Passed: false, Details: []string{"no group matched"}}
}

// FilterByType filters conditions by CondType (buy/add/sell/reduce).
func (s *SignalService) FilterByType(conds []model.StrategyCondition, condType string) []model.StrategyCondition {
	var out []model.StrategyCondition
	for _, c := range conds {
		if c.CondType == condType && c.Enabled {
			out = append(out, c)
		}
	}
	return out
}

// ComputeFuzzyScore computes a sigmoid-based score for how well a value meets a condition.
// sigma controls steepness; 0 = binary (exact match), >0 = smooth gradient.
func (s *SignalService) ComputeFuzzyScore(value, threshold float64, operator string, sigma float64) float64 {
	if sigma <= 0 {
		if checkOperator(value, operator, threshold) {
			return 1.0
		}
		return 0.0
	}
	diff := (value - threshold) / sigma
	return 1.0 / (1.0 + math.Exp(-diff))
}

// checkOperator compares a value against a threshold using the given operator.
func checkOperator(val float64, op string, threshold float64) bool {
	switch op {
	case "gte":
		return val >= threshold
	case "lte":
		return val <= threshold
	case "gt":
		return val > threshold
	case "lt":
		return val < threshold
	case "eq":
		return val == threshold
	case "cross_up":
		return val > 0
	case "cross_down":
		return val < 0
	}
	return false
}

func boolIcon(passed bool) string {
	if passed {
		return "✓"
	}
	return "✗"
}

// trimFloat formats a float for display, trimming trailing zeros.
func trimFloat(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
