package service

import (
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/repository"
)

type WatchlistService struct {
	repo *repository.WatchlistRepo
}

func NewWatchlistService() *WatchlistService { return &WatchlistService{repo: &repository.WatchlistRepo{}} }

type WatchlistItem struct {
	StockCode string  `json:"stockCode"`
	StockName string  `json:"stockName"`
	Industry  string  `json:"industry"`
	Price     float64 `json:"price"`
	ChgPct    float64 `json:"chgPct"`
	PE        float64 `json:"pe"`
	PB        float64 `json:"pb"`
	AddedAt   string  `json:"addedAt"`
}

func (s *WatchlistService) List(userID uint) ([]WatchlistItem, error) {
	items, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}

	result := make([]WatchlistItem, 0, len(items))
	for _, item := range items {
		wi := WatchlistItem{StockCode: item.StockCode, AddedAt: item.AddedAt.Format("2006-01-02")}

		// Get stock basic info
		var basic model.StockBasic
		if err := db.PG.Where("code = ?", item.StockCode).First(&basic).Error; err == nil {
			wi.StockName = basic.Name
			wi.Industry = basic.Industry
		}

		// Get latest K-line for price & chg%
		var kline struct {
			Close float64
			Open  float64
		}
		if err := db.PG.Table("stocks_daily_k").
			Select("close, open").
			Where("code = ?", item.StockCode).
			Order("trade_date DESC").Limit(1).Scan(&kline).Error; err == nil && kline.Close > 0 {
			wi.Price = kline.Close
			if kline.Open > 0 {
				wi.ChgPct = (kline.Close - kline.Open) / kline.Open * 100
			}
		}

		// Get indicator
		var ind model.StockDailyIndicator
		if err := db.PG.Where("code = ?", item.StockCode).Order("trade_date DESC").First(&ind).Error; err == nil {
			wi.PE = ind.PE
			wi.PB = ind.PB
		}

		result = append(result, wi)
	}
	return result, nil
}

func (s *WatchlistService) Add(userID uint, stockCode string) error {
	return s.repo.Add(userID, stockCode)
}

func (s *WatchlistService) Remove(userID uint, stockCode string) error {
	return s.repo.Remove(userID, stockCode)
}

