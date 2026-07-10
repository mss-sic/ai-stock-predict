package service

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ai-stock-predict/server/internal/db"
)

// Indicators that may legitimately return N/A for some stocks
var naIndicators = map[string]bool{
	"pe": true, "pb": true, "algo_score": true, "ma_cross": true, "macd": true,
}

func TestAllIndicators(t *testing.T) {
	pgDSN := os.Getenv("POSTGRES_DSN")
	if pgDSN == "" {
		pgDSN = "host=localhost user=stock password=stock123 dbname=stock_predict sslmode=disable"
	}
	db.InitPostgres(pgDSN)

	date := "2026-07-10"
	stocks := []struct{ code, name string }{
		{"002141", "贤丰控股"},
		{"601162", "天风证券"},
		{"600519", "贵州茅台"},
		{"000001", "平安银行"},
	}

	indicators := []struct {
		indicator string
		category  string
	}{
		{"close", "daily_k_column"},
		{"open", "daily_k_column"},
		{"high", "daily_k_column"},
		{"low", "daily_k_column"},
		{"volume", "daily_k_column"},
		{"amount", "daily_k_column"},
		{"turnover_rate", "daily_k_column"},
		{"change_pct", "daily_k_column"},
		{"amplitude", "daily_k_column"},
		{"volume_ratio", "daily_k_column"},
		{"pre_close", "daily_k_column"},
		{"change_amount", "daily_k_column"},
		{"avg_price", "daily_k_column"},
		{"high_limit", "daily_k_column"},
		{"low_limit", "daily_k_column"},
		{"buy_vol", "daily_k_column"},
		{"sell_vol", "daily_k_column"},
		{"macd_dif", "daily_k_column"},
		{"macd_dea", "daily_k_column"},
		{"ema12", "daily_k_column"},
		{"ema26", "daily_k_column"},
		{"daily_change", "alias"},
		{"pct_chg", "alias"},
		{"change", "alias"},
		{"vol", "alias"},
		{"turnover", "alias"},
		{"CLOSE", "alias_case"},
		{"VOLUME", "alias_case"},
		{"AMOUNT", "alias_case"},
		{"TURNOVER", "alias_case"},
		{"PCT_CHG", "alias_case"},
		{"roe", "financial"},
		{"eps", "financial"},
		{"bps", "financial"},
		{"net_profit", "financial"},
		{"total_revenue", "financial"},
		{"revenue_growth", "financial"},
		{"profit_growth", "financial"},
		{"gross_margin", "financial"},
		{"net_margin", "financial"},
		{"debt_ratio", "financial"},
		{"total_assets", "financial"},
		{"net_assets", "financial"},
		{"pe", "computed"},
		{"pb", "computed"},
		{"rsi", "computed"},
		{"RSI", "computed_case"},
		{"streak_count", "computed"},
		{"algo_score", "computed"},
		{"ma5", "computed_pattern"},
		{"ma10", "computed_pattern"},
		{"ma20", "computed_pattern"},
		{"chg_5d", "computed_pattern"},
		{"chg_20d", "computed_pattern"},
		{"atr_14_pct", "computed_pattern"},
		{"momentum_5", "computed_pattern"},
		{"momentum_20", "computed_pattern"},
		{"drawdown_20", "computed_pattern"},
		{"volume_ma_ratio", "computed_pattern"},
		{"boll_position", "computed_pattern"},
		{"ma_cross", "cross"},
		{"macd", "cross"},
		{"nonexistent_xyz", "unknown"},
	}

	var failures, expectedNA []string
	total, okCount, naCount := 0, 0, 0

	for _, stock := range stocks {
		for _, ind := range indicators {
			total++
			_, ok := GetIndicatorValue(ind.indicator, stock.code, date)
			if ok {
				okCount++
			} else if naIndicators[ind.indicator] {
				naCount++
				expectedNA = append(expectedNA, fmt.Sprintf("  ⚠ %s(%s) %s = N/A (expected)", stock.name, stock.code, ind.indicator))
			} else if ind.category != "unknown" {
				failures = append(failures, fmt.Sprintf("  ✗ %s(%s) %s [%s] failed", stock.name, stock.code, ind.indicator, ind.category))
			}
		}
	}

	fmt.Printf("\n══════════════════════════════════════\n")
	fmt.Printf("Indicator Test Summary\n")
	fmt.Printf("══════════════════════════════════════\n")
	fmt.Printf("Date: %s | Stocks: %d | Indicators: %d\n", date, len(stocks), len(indicators))
	fmt.Printf("Tests: %d | OK: %d | N/A: %d | Fail: %d (%.1f%%)\n",
		total, okCount, naCount, len(failures),
		float64(okCount+naCount)/float64(total)*100)

	if len(expectedNA) > 0 {
		fmt.Printf("\n── Expected N/A (no data for this stock) ──\n")
		for _, s := range expectedNA {
			fmt.Println(s)
		}
	}

	if len(failures) > 0 {
		fmt.Printf("\n── Unexpected Failures ──\n")
		for _, s := range failures {
			fmt.Println(s)
		}
		t.Errorf("%d unexpected failure(s)", len(failures))
	} else {
		fmt.Printf("\n✅ All indicators pass!\n")
	}
}

func TestIndicatorDetail(t *testing.T) {
	pgDSN := os.Getenv("POSTGRES_DSN")
	if pgDSN == "" {
		pgDSN = "host=localhost user=stock password=stock123 dbname=stock_predict sslmode=disable"
	}
	db.InitPostgres(pgDSN)

	date := "2026-07-10"
	stocks := []struct{ code, name string }{
		{"002141", "贤丰控股"},
		{"601162", "天风证券"},
		{"600519", "贵州茅台"},
	}

	keyIndicators := []string{
		"close", "open", "high", "low",
		"volume", "amount", "turnover_rate",
		"change_pct", "daily_change",
		"volume_ratio", "amplitude",
		"pe", "pb", "roe", "eps", "bps",
		"rsi", "ma5", "ma10", "ma20",
		"chg_5d", "chg_20d", "momentum_5",
		"drawdown_20", "boll_position",
	}

	for _, stock := range stocks {
		fmt.Printf("\n── %s(%s) @ %s ──\n", stock.name, stock.code, date)
		for _, ind := range keyIndicators {
			val, ok := GetIndicatorValue(ind, stock.code, date)
			if ok {
				fmt.Printf("  %-20s = %12.4f\n", ind, val)
			} else {
				fmt.Printf("  %-20s = %12s (N/A)\n", ind, "—")
			}
		}
	}
}

func TestIndicatorColumnSafety(t *testing.T) {
	tests := []struct {
		col  string
		safe bool
	}{
		{"close", true},
		{"change_pct", true},
		{"DROP TABLE", false},
		{"close; DROP TABLE", false},
		{"close--", false},
		{"close ", false},
		{"\"close\"", false},
		{"", false},
		{strings.Repeat("a", 65), false},
	}
	for _, tt := range tests {
		if got := isSafeColumn(tt.col); got != tt.safe {
			t.Errorf("isSafeColumn(%q) = %v, want %v", tt.col, got, tt.safe)
		}
	}
}
