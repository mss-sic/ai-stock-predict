package service

import "strings"

// StockPoolItem represents a single stock in a pool.
type StockPoolItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// StockPoolGroup represents a group of stocks (e.g., watchlist, portfolio, all).
type StockPoolGroup struct {
	Key   string          `json:"key"`
	Label string          `json:"label"`
	Count int             `json:"count"`
	Items []StockPoolItem `json:"items,omitempty"`
}

// StockPoolService manages stock pool resolution for strategy backtesting.
type StockPoolService struct{}

// NewStockPoolService creates a new StockPoolService.
func NewStockPoolService() *StockPoolService {
	return &StockPoolService{}
}

// PoolKey constants for standard pool types.
const (
	PoolKeyAll       = "all"
	PoolKeyPortfolio = "portfolio"
	PoolKeyCodes     = "codes"
)

// IsWatchlistPool returns true if the pool key refers to a watchlist group.
func (s *StockPoolService) IsWatchlistPool(key string) bool {
	return strings.HasPrefix(key, "watchlist_")
}

// ResolvePoolLabel returns a human-readable label for a pool key.
func (s *StockPoolService) ResolvePoolLabel(key string, stockCount int) string {
	var label string
	switch {
	case key == PoolKeyAll:
		label = "全部股票"
	case s.IsWatchlistPool(key):
		label = "自选组"
	case key == PoolKeyPortfolio:
		label = "我的持仓"
	case key == PoolKeyCodes:
		label = "自选代码"
	default:
		label = key
	}
	if stockCount > 0 {
		return label + " (" + itoa(stockCount) + "只)"
	}
	return label
}

// ShouldScanAll returns true if the pool key means "scan all stocks".
// A nil or empty stockCodes list means scan all.
func (s *StockPoolService) ShouldScanAll(key string, codes []string) bool {
	if key == PoolKeyAll {
		return true
	}
	return len(codes) == 0 && key == ""
}

// ValidatePoolSize checks if the pool has sufficient stocks for meaningful backtesting.
func (s *StockPoolService) ValidatePoolSize(count int) (minCount int, ok bool) {
	minCount = 5
	return minCount, count >= minCount
}

// FilterByIndustry filters stock items by industry name.
func (s *StockPoolService) FilterByIndustry(items []StockPoolItem, industry string) []StockPoolItem {
	// Industry filtering is done at DB level - this is a placeholder
	// for future in-memory filtering when industry data is preloaded
	if industry == "" {
		return items
	}
	return items // actual filtering deferred to DB query
}

// BuildPoolLabel constructs a label from key and params for display.
func (s *StockPoolService) BuildPoolLabel(key, label string, codes []string) string {
	count := len(codes)
	if count > 0 {
		return s.ResolvePoolLabel(key, count)
	}
	return s.ResolvePoolLabel(key, 0)
}

// itoa is a simple int-to-string converter without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
