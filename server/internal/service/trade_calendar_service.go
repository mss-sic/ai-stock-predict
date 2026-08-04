package service

import (
	"fmt"
	"time"

	"github.com/ai-stock-predict/server/internal/repository"
)

// TradeCalendarService provides business logic for trading day operations.
type TradeCalendarService struct {
	repo *repository.TradeCalendarRepo
}

// NewTradeCalendarService creates a new TradeCalendarService.
func NewTradeCalendarService() *TradeCalendarService {
	return &TradeCalendarService{
		repo: repository.NewTradeCalendarRepo(),
	}
}

// LatestTradeDate returns the most recent trading day.
func (s *TradeCalendarService) LatestTradeDate() (time.Time, error) {
	t, err := s.repo.LatestTradeDate()
	if err != nil {
		return t, fmt.Errorf("get latest trade date: %w", err)
	}
	if t.IsZero() {
		return t, fmt.Errorf("no trading days found in trade_calendar table")
	}
	return t, nil
}

// NPrevTradeDate returns the Nth previous trading day (n=1 means previous trading day).
func (s *TradeCalendarService) NPrevTradeDate(after time.Time, n int) (time.Time, error) {
	cur := after
	for i := 0; i < n; i++ {
		var err error
		cur, err = s.repo.PrevTradeDate(cur)
		if err != nil {
			return cur, fmt.Errorf("get prev trade date (n=%d): %w", n, err)
		}
		if cur.IsZero() {
			return cur, fmt.Errorf("not enough trading days before %s", after.Format("2006-01-02"))
		}
	}
	return cur, nil
}

// TradingDaysAgo returns the date N trading days ago.
func (s *TradeCalendarService) TradingDaysAgo(n int) (time.Time, error) {
	// Start from latest trading day if today is not a trading day
	latest, err := s.repo.LatestTradeDate()
	if err != nil {
		return latest, err
	}
	return s.NPrevTradeDate(latest, n)
}
