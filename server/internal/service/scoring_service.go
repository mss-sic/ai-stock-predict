package service

import "github.com/ai-stock-predict/server/internal/model"

// ScoreResult is the final scoring output for one stock.
type ScoreResult struct {
	Code       string
	Name       string
	Price      float64
	TotalScore float64
	Rank       int
}

// ScoreDistribution captures the statistical distribution of scores.
type ScoreDistribution struct {
	Count  int
	Top1   float64
	Top5   float64
	Top10P float64
	Median float64
	Mean   float64
}

// AdaptiveMinScore computes a dynamic minimum score threshold.
// Uses top-quintile mean as reference, with hard floor at 0.30.
func (d ScoreDistribution) AdaptiveMinScore() float64 {
	if d.Count == 0 {
		return 0.30
	}
	gap := d.Top1 - d.Median
	dynamicMin := d.Median + gap*0.5
	if dynamicMin < 0.30 {
		dynamicMin = 0.30
	}
	return dynamicMin
}

// ScoringService provides multi-factor scoring for the backtest pipeline.
// Currently a thin wrapper. Full extraction from handler/scoring_engine.go
// will happen in Phase 3 (Backtest Engine Extraction).
type ScoringService struct {
	signalSvc *SignalService
}

// NewScoringService creates a new ScoringService.
func NewScoringService() *ScoringService {
	return &ScoringService{
		signalSvc: NewSignalService(),
	}
}

// ComputeDistribution calculates the score distribution from a list of results.
func (s *ScoringService) ComputeDistribution(results []ScoreResult) ScoreDistribution {
	n := len(results)
	if n == 0 {
		return ScoreDistribution{}
	}

	// Extract scores
	scores := make([]float64, n)
	var sum float64
	for i, r := range results {
		scores[i] = r.TotalScore
		sum += r.TotalScore
	}

	// Sort descending
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if scores[i] < scores[j] {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	top5Count := 5
	if n < 5 {
		top5Count = n
	}
	top10PCount := n / 10
	if top10PCount < 1 {
		top10PCount = 1
	}

	var top5Sum, top10PSum float64
	for i := 0; i < top5Count; i++ {
		top5Sum += scores[i]
	}
	for i := 0; i < top10PCount; i++ {
		top10PSum += scores[i]
	}

	return ScoreDistribution{
		Count:  n,
		Top1:   scores[0],
		Top5:   top5Sum / float64(top5Count),
		Top10P: top10PSum / float64(top10PCount),
		Median: scores[n/2],
		Mean:   sum / float64(n),
	}
}

// RankResults assigns ranks to results based on TotalScore (descending).
func (s *ScoringService) RankResults(results []ScoreResult) {
	for i := range results {
		results[i].Rank = i + 1
	}
}

// FilterByMinScore returns results with TotalScore >= minScore.
func (s *ScoringService) FilterByMinScore(results []ScoreResult, minScore float64) []ScoreResult {
	var filtered []ScoreResult
	for _, r := range results {
		if r.TotalScore >= minScore {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// ComputeWeightedScore calculates a single condition's weighted score.
// Binary mode (sigma <= 0): score = weight if condition passes, else 0.
// Fuzzy mode (sigma > 0): score = sigmoid((value - threshold) / sigma) * weight.
func (s *ScoringService) ComputeWeightedScore(
	value, threshold float64,
	operator string,
	weight, sigma float64,
) float64 {
	raw := s.signalSvc.ComputeFuzzyScore(value, threshold, operator, sigma)
	return raw * weight
}

// ConditionWeight is a convenience type for passing condition weights.
type ConditionWeight struct {
	Indicator string
	Operator  string
	Threshold float64
	Weight    float64
	Sigma     float64
}

// ComputeTotalScore computes the total weighted score for a stock given indicator values.
// Returns sum of weighted scores across all conditions.
func (s *ScoringService) ComputeTotalScore(
	conds []ConditionWeight,
	indicatorValues map[string]float64,
) float64 {
	var total float64
	for _, c := range conds {
		val, ok := indicatorValues[c.Indicator]
		if !ok {
			continue
		}
		total += s.ComputeWeightedScore(val, c.Threshold, c.Operator, c.Weight, c.Sigma)
	}
	return total
}

// BuildConditionWeights converts model conditions to ConditionWeight for scoring.
func (s *ScoringService) BuildConditionWeights(conds []model.StrategyCondition) []ConditionWeight {
	result := make([]ConditionWeight, 0, len(conds))
	for _, c := range conds {
		if !c.Enabled {
			continue
		}
		w := c.Weight
		if w <= 0 {
			w = 1.0
		}
		result = append(result, ConditionWeight{
			Indicator: c.Indicator,
			Operator:  c.Operator,
			Threshold: c.Value,
			Weight:    w,
			Sigma:     c.FuzzySigma,
		})
	}
	return result
}
