package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

func init() {
	RegisterRule(&MajorReductionRule{})
	RegisterRule(&LitigationViolationRule{})
	RegisterRule(&DividendExNearRule{})
}

// ── E1: Major Reduction ──

type MajorReductionRule struct{}

func (r *MajorReductionRule) Key() string { return "major_reduction" }
func (r *MajorReductionRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	lookback := 30
	keywords := "减持,股份变动,权益变动,转让"
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["lookback_days"].(float64); ok {
			lookback = int(v)
		}
		if v, ok := def.Thresholds["keywords"].(string); ok {
			keywords = v
		}
	}
	kwList := strings.Split(keywords, ",")
	inClause := codesToInClause(codes)

	type Row struct {
		Code  string `gorm:"column:code"`
		Title string `gorm:"column:title"`
		Date  string `gorm:"column:ann_date"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, title, ann_date::text
		FROM cninfo_announcements
		WHERE code IN (%s) AND ann_date >= CURRENT_DATE - INTERVAL '%d days'
	`, inClause, lookback)).Scan(&rows)

	now := time.Now()
	seen := make(map[string]bool)
	var alerts []model.RiskAlert
	for _, row := range rows {
		for _, kw := range kwList {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(row.Title, kw) {
				hash := row.Code + ":" + row.Date + ":" + kw
				if seen[hash] {
					continue
				}
				seen[hash] = true
				alerts = append(alerts, model.RiskAlert{
					StockCode:   row.Code,
					Level:       "high",
					Type:        "大股东减持公告",
					Description: fmt.Sprintf("%s: %s", row.Date, row.Title),
					RuleKey:     "major_reduction",
					Dimension:   "event",
					SeverityScore: 75,
					Evidence: model.JSONMap{
						"title":      row.Title,
						"ann_date":   row.Date,
						"keyword":    kw,
					},
					HitDate: now,
				})
				break
			}
		}
	}
	return alerts, nil, nil
}

// ── E2: Litigation / Violation ──

type LitigationViolationRule struct{}

func (r *LitigationViolationRule) Key() string { return "litigation_violation" }
func (r *LitigationViolationRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	lookback := 30
	keywords := "诉讼,违规,处罚,立案,调查"
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["lookback_days"].(float64); ok {
			lookback = int(v)
		}
		if v, ok := def.Thresholds["keywords"].(string); ok {
			keywords = v
		}
	}
	kwList := strings.Split(keywords, ",")
	inClause := codesToInClause(codes)

	type Row struct {
		Code  string `gorm:"column:code"`
		Title string `gorm:"column:title"`
		Date  string `gorm:"column:ann_date"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, title, ann_date::text
		FROM cninfo_announcements
		WHERE code IN (%s) AND ann_date >= CURRENT_DATE - INTERVAL '%d days'
	`, inClause, lookback)).Scan(&rows)

	now := time.Now()
	seen := make(map[string]bool)
	var alerts []model.RiskAlert
	for _, row := range rows {
		for _, kw := range kwList {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(row.Title, kw) {
				hash := row.Code + ":" + row.Date + ":" + kw
				if seen[hash] {
					continue
				}
				seen[hash] = true
				alerts = append(alerts, model.RiskAlert{
					StockCode:   row.Code,
					Level:       "high",
					Type:        "重大诉讼违规",
					Description: fmt.Sprintf("%s: %s", row.Date, row.Title),
					RuleKey:     "litigation_violation",
					Dimension:   "event",
					SeverityScore: 80,
					Evidence: model.JSONMap{
						"title":    row.Title,
						"ann_date": row.Date,
						"keyword":  kw,
					},
					HitDate: now,
				})
				break
			}
		}
	}
	return alerts, nil, nil
}

// ── E3: Dividend Ex Near ──

type DividendExNearRule struct{}

func (r *DividendExNearRule) Key() string { return "dividend_ex_near" }
func (r *DividendExNearRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	lookahead := 5
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["lookahead_days"].(float64); ok {
			lookahead = int(v)
		}
	}
	inClause := codesToInClause(codes)

	type Row struct {
		Code          string `gorm:"column:code"`
		ExDividendDate string `gorm:"column:ex_dividend_date"`
		CashDiv       float64 `gorm:"column:cash_div"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, ex_dividend_date::text, cash_div
		FROM dividend_history
		WHERE code IN (%s)
		  AND ex_dividend_date BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '%d days'
	`, inClause, lookahead)).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, row := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:   row.Code,
			Level:       "low",
			Type:        "分红除权临近",
			Description: fmt.Sprintf("%s除权除息，每股%.2f元，注意价格跳空", row.ExDividendDate, row.CashDiv),
			RuleKey:     "dividend_ex_near",
			Dimension:   "event",
			SeverityScore: 10,
			Evidence: model.JSONMap{
				"ex_date":  row.ExDividendDate,
				"cash_div": row.CashDiv,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}
