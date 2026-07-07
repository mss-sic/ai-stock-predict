package service

import (
	"fmt"
	"strings"

	"github.com/ai-stock-predict/server/internal/model"
)

// DecisionTreeNode represents a node in the decision tree.
type DecisionTreeNode struct {
	Condition *model.StrategyCondition
	Operator  string // and / or / not
	Children  []*DecisionTreeNode
	IsLeaf    bool
}

// TreeNodeResult records evaluation detail for a single node.
type TreeNodeResult struct {
	GroupID   int
	Indicator string
	Operator  string
	TreeOp    string
	Value     float64
	Threshold float64
	Passed    bool
	IsLeaf    bool
}

// EvalFunc evaluates a single condition for a stock on a given date.
type EvalFunc func(cond model.StrategyCondition, code, date string) bool

// DecisionTreeService evaluates sell/reduce conditions using a tree structure.
// Pure business logic — condition evaluation is delegated via EvalFunc.
type DecisionTreeService struct{}

// NewDecisionTreeService creates a new DecisionTreeService.
func NewDecisionTreeService() *DecisionTreeService {
	return &DecisionTreeService{}
}

// BuildTree constructs a decision tree from flat conditions.
// Root nodes are conditions with ParentID == nil or 0.
func (s *DecisionTreeService) BuildTree(conditions []model.StrategyCondition) []*DecisionTreeNode {
	childMap := make(map[uint][]model.StrategyCondition)
	rootConds := make([]model.StrategyCondition, 0)

	for _, c := range conditions {
		if !c.Enabled {
			continue
		}
		if c.ParentID == nil || *c.ParentID == 0 {
			rootConds = append(rootConds, c)
		} else {
			childMap[*c.ParentID] = append(childMap[*c.ParentID], c)
		}
	}

	roots := make([]*DecisionTreeNode, 0, len(rootConds))
	for _, rc := range rootConds {
		condCopy := rc
		node := &DecisionTreeNode{
			Condition: &condCopy,
			Operator:  condCopy.TreeOperator,
			IsLeaf:    len(childMap[condCopy.ID]) == 0,
		}
		s.buildChildren(node, childMap)
		roots = append(roots, node)
	}
	return roots
}

// buildChildren recursively builds child nodes.
func (s *DecisionTreeService) buildChildren(parent *DecisionTreeNode, childMap map[uint][]model.StrategyCondition) {
	children, ok := childMap[parent.Condition.ID]
	if !ok || len(children) == 0 {
		parent.IsLeaf = true
		return
	}
	parent.IsLeaf = false
	for _, cc := range children {
		ccCopy := cc
		child := &DecisionTreeNode{
			Condition: &ccCopy,
			Operator:  ccCopy.TreeOperator,
			IsLeaf:    len(childMap[cc.ID]) == 0,
		}
		s.buildChildren(child, childMap)
		parent.Children = append(parent.Children, child)
	}
}

// Evaluate evaluates the decision tree for a single stock/date combination.
// Returns (triggered, reason).
func (s *DecisionTreeService) Evaluate(roots []*DecisionTreeNode, evalFn EvalFunc, code, date string) (bool, string) {
	if len(roots) == 0 {
		return false, "无条件（空决策树）"
	}

	reasons := make([]string, 0)
	anyTriggered := false

	for i, root := range roots {
		triggered, reason := s.evaluateNode(root, evalFn, code, date)
		if triggered {
			anyTriggered = true
			reasons = append(reasons, fmt.Sprintf("组%d触发: %s", i+1, reason))
		}
	}

	if anyTriggered {
		return true, strings.Join(reasons, "; ")
	}
	return false, "条件未触发"
}

// EvaluateWithDetail evaluates and returns per-node results.
func (s *DecisionTreeService) EvaluateWithDetail(
	roots []*DecisionTreeNode,
	evalFn EvalFunc,
	code, date string,
) (bool, []TreeNodeResult) {
	results := make([]TreeNodeResult, 0)
	anyTriggered := false

	for i, root := range roots {
		triggered, _ := s.evaluateNodeDetail(root, evalFn, code, date, i+1, &results)
		if triggered {
			anyTriggered = true
		}
	}
	return anyTriggered, results
}

// evaluateNode recursively evaluates a tree node.
func (s *DecisionTreeService) evaluateNode(
	node *DecisionTreeNode,
	evalFn EvalFunc,
	code, date string,
) (bool, string) {
	if node.IsLeaf {
		passed := evalFn(*node.Condition, code, date)
		if passed {
			return true, node.Condition.Indicator + " ✓"
		}
		return false, node.Condition.Indicator + " ✗"
	}

	passedCount := 0
	reasons := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		childPassed, childReason := s.evaluateNode(child, evalFn, code, date)
		if childPassed {
			passedCount++
		}
		reasons = append(reasons, childReason)
	}

	var result bool
	switch node.Operator {
	case "and":
		result = passedCount == len(node.Children)
	case "or":
		result = passedCount > 0
	case "not":
		result = passedCount == 0
	default:
		result = passedCount == len(node.Children) // default to AND
	}

	if result {
		return true, fmt.Sprintf("(%s)", strings.Join(reasons, " "+node.Operator+" "))
	}
	return false, fmt.Sprintf("(%s)", strings.Join(reasons, " "+node.Operator+" "))
}

// evaluateNodeDetail evaluates with per-node results collection.
func (s *DecisionTreeService) evaluateNodeDetail(
	node *DecisionTreeNode,
	evalFn EvalFunc,
	code, date string,
	groupID int,
	results *[]TreeNodeResult,
) (bool, string) {
	if node.IsLeaf {
		passed := evalFn(*node.Condition, code, date)
		*results = append(*results, TreeNodeResult{
			GroupID:   groupID,
			Indicator: node.Condition.Indicator,
			Operator:  node.Condition.Operator,
			TreeOp:    node.Operator,
			Value:     0,
			Threshold: node.Condition.Value,
			Passed:    passed,
			IsLeaf:    true,
		})
		if passed {
			return true, node.Condition.Indicator + " ✓"
		}
		return false, node.Condition.Indicator + " ✗"
	}

	passedCount := 0
	reasons := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		childPassed, childReason := s.evaluateNodeDetail(child, evalFn, code, date, groupID, results)
		if childPassed {
			passedCount++
		}
		reasons = append(reasons, childReason)
	}

	var result bool
	switch node.Operator {
	case "and":
		result = passedCount == len(node.Children)
	case "or":
		result = passedCount > 0
	case "not":
		result = passedCount == 0
	default:
		result = passedCount == len(node.Children)
	}

	if result {
		return true, fmt.Sprintf("(%s)", strings.Join(reasons, " "+node.Operator+" "))
	}
	return false, fmt.Sprintf("(%s)", strings.Join(reasons, " "+node.Operator+" "))
}

// FilterByEnabled returns only enabled conditions from a list.
func (s *DecisionTreeService) FilterByEnabled(conds []model.StrategyCondition) []model.StrategyCondition {
	out := make([]model.StrategyCondition, 0, len(conds))
	for _, c := range conds {
		if c.Enabled {
			out = append(out, c)
		}
	}
	return out
}
