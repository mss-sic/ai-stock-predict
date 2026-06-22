package handler

import (
	"fmt"
	"log"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// DecisionTreeEngine — 决策树引擎 (Sell/Reduce)
// ═══════════════════════════════════════════════════════════════
//
// 支持嵌套条件组和分支决策。
// 条件按 parent_id 构建树结构，后序遍历求值。
// 根节点为 true → 整个条件组触发。

// TreeNode represents a node in the decision tree.
type TreeNode struct {
	Condition *model.StrategyCondition
	Operator  string     // and / or / not
	Children  []*TreeNode
	IsLeaf    bool
	Evaluated bool
	Result    bool
}

// DecisionTreeEngine evaluates sell/reduce conditions using a tree structure.
type DecisionTreeEngine struct {
	conditions []model.StrategyCondition
	evalSingle func(model.StrategyCondition, string, string) bool
}

// NewDecisionTreeEngine creates a decision tree engine.
func NewDecisionTreeEngine(
	conditions []model.StrategyCondition,
	evalSingle func(model.StrategyCondition, string, string) bool,
) *DecisionTreeEngine {
	return &DecisionTreeEngine{
		conditions: conditions,
		evalSingle: evalSingle,
	}
}

// Evaluate evaluates the decision tree for a single stock on a given date.
// Returns (triggered bool, reason string).
func (dte *DecisionTreeEngine) Evaluate(code, date string) (bool, string) {
	roots := dte.buildTree()
	if len(roots) == 0 {
		return false, "无条件（空决策树）"
	}

	// Evaluate each root group. Any root triggering means the overall condition fires.
	reasons := make([]string, 0)
	anyTriggered := false

	for i, root := range roots {
		triggered, reason := dte.evaluateNode(root, code, date)
		if triggered {
			anyTriggered = true
			reasons = append(reasons, fmt.Sprintf("组%d触发: %s", i+1, reason))
		}
	}

	if anyTriggered {
		return true, joinActions(reasons)
	}
	return false, "条件未触发"
}

// EvaluateWithDetail returns detailed per-node evaluation results.
func (dte *DecisionTreeEngine) EvaluateWithDetail(code, date string) (bool, []TreeNodeResult) {
	roots := dte.buildTree()
	results := make([]TreeNodeResult, 0)
	anyTriggered := false

	for i, root := range roots {
		triggered, _ := dte.evaluateNodeDetail(root, code, date, i+1, &results)
		if triggered {
			anyTriggered = true
		}
	}
	return anyTriggered, results
}

// TreeNodeResult records evaluation detail for a node.
type TreeNodeResult struct {
	GroupID    int
	Indicator  string
	Operator   string
	TreeOp     string
	Value      float64
	Threshold  float64
	Passed     bool
	IsLeaf     bool
}

// buildTree constructs the decision tree from flat conditions.
func (dte *DecisionTreeEngine) buildTree() []*TreeNode {
	// Group by parent_id: root nodes have parent_id = NULL or 0
	childMap := make(map[uint][]model.StrategyCondition)
	rootConds := make([]model.StrategyCondition, 0)

	for _, c := range dte.conditions {
		if !c.Enabled {
			continue
		}
		if c.ParentID == nil || *c.ParentID == 0 {
			rootConds = append(rootConds, c)
		} else {
			childMap[*c.ParentID] = append(childMap[*c.ParentID], c)
		}
	}

	// Build tree recursively
	roots := make([]*TreeNode, 0, len(rootConds))
	for _, rc := range rootConds {
		node := &TreeNode{
			Condition: &rc,
			Operator:  rc.TreeOperator,
			IsLeaf:    len(childMap[rc.ID]) == 0,
		}
		// CRITICAL: capture rc by value to avoid loop variable aliasing
		condCopy := rc
		node.Condition = &condCopy
		dte.buildChildren(node, childMap)
		roots = append(roots, node)
	}

	return roots
}

// buildChildren recursively builds child nodes.
func (dte *DecisionTreeEngine) buildChildren(parent *TreeNode, childMap map[uint][]model.StrategyCondition) {
	children, ok := childMap[parent.Condition.ID]
	if !ok || len(children) == 0 {
		parent.IsLeaf = true
		return
	}

	parent.IsLeaf = false
	for _, cc := range children {
		ccCopy := cc
		child := &TreeNode{
			Condition: &ccCopy,
			Operator:  ccCopy.TreeOperator,
			IsLeaf:    len(childMap[cc.ID]) == 0,
		}
		dte.buildChildren(child, childMap)
		parent.Children = append(parent.Children, child)
	}
}

// evaluateNode recursively evaluates a tree node.
func (dte *DecisionTreeEngine) evaluateNode(node *TreeNode, code, date string) (bool, string) {
	if node.IsLeaf {
		passed := dte.evalSingle(*node.Condition, code, date)
		node.Evaluated = true
		node.Result = passed
		if passed {
			return true, fmt.Sprintf("%s ✓", node.Condition.Indicator)
		}
		return false, fmt.Sprintf("%s ✗", node.Condition.Indicator)
	}

	// Non-leaf: evaluate children and combine
	passedCount := 0
	reasons := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		childPassed, childReason := dte.evaluateNode(child, code, date)
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
		result = passedCount == len(node.Children) // default AND
	}

	node.Evaluated = true
	node.Result = result

	status := "✗"
	if result {
		status = "✓"
	}
	return result, fmt.Sprintf("组[%s] %s (%s)", node.Operator, status, joinActions(reasons))
}

// evaluateNodeDetail evaluates with detailed result recording.
func (dte *DecisionTreeEngine) evaluateNodeDetail(
	node *TreeNode,
	code, date string,
	groupID int,
	results *[]TreeNodeResult,
) (bool, string) {
	if node.IsLeaf {
		passed := dte.evalSingle(*node.Condition, code, date)
		node.Evaluated = true
		node.Result = passed
		*results = append(*results, TreeNodeResult{
			GroupID:   groupID,
			Indicator: node.Condition.Indicator,
			Operator:  node.Condition.Operator,
			TreeOp:    node.Operator,
			Value:     0, // value filled by evalSingle
			Threshold: node.Condition.Value,
			Passed:    passed,
			IsLeaf:    true,
		})
		if passed {
			return true, fmt.Sprintf("%s ✓", node.Condition.Indicator)
		}
		return false, fmt.Sprintf("%s ✗", node.Condition.Indicator)
	}

	passedCount := 0
	reasons := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		childPassed, childReason := dte.evaluateNodeDetail(child, code, date, groupID, results)
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

	node.Evaluated = true
	node.Result = result

	*results = append(*results, TreeNodeResult{
		GroupID:  groupID,
		TreeOp:   node.Operator,
		Passed:   result,
		IsLeaf:   false,
	})

	status := "✗"
	if result {
		status = "✓"
	}
	return result, fmt.Sprintf("组[%s] %s", node.Operator, status)
}

// BuildFlatConditions converts a tree back to flat conditions for backward compatibility.
// This is used when the engine needs to pass conditions back to the old evalConds system.
func (dte *DecisionTreeEngine) BuildFlatConditions() [][]model.StrategyCondition {
	roots := dte.buildTree()
	groups := make([][]model.StrategyCondition, 0)

	for _, root := range roots {
		group := make([]model.StrategyCondition, 0)
		dte.collectLeaves(root, &group)
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

// collectLeaves recursively collects leaf conditions.
func (dte *DecisionTreeEngine) collectLeaves(node *TreeNode, result *[]model.StrategyCondition) {
	if node.IsLeaf {
		*result = append(*result, *node.Condition)
		return
	}
	for _, child := range node.Children {
		dte.collectLeaves(child, result)
	}
}

// init registers the decision tree engine for import
func init() {
	log.Printf("[decision_tree_engine] registered")
}
