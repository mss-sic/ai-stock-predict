package service

import (
	"log"

	"github.com/ai-stock-predict/server/internal/repository"
)

// DataLoaderService handles batch data loading and caching for backtest execution.
// Encapsulates the K-line preloading, forward-filling, and cache construction
// that was previously embedded in strategy_handler's runBacktestAsync.
type DataLoaderService struct {
	klineRepo     *repository.KlineRepo
	indicatorRepo *repository.IndicatorRepo
}

// NewDataLoaderService creates a new DataLoaderService.
func NewDataLoaderService() *DataLoaderService {
	return &DataLoaderService{
		klineRepo:     repository.NewKlineRepo(),
		indicatorRepo: repository.NewIndicatorRepo(),
	}
}

// KlineCache stores preloaded close/open prices for fast O(1) lookup during backtest.
type KlineCache struct {
	Dates    []string
	DateIdx  map[string]int
	CloseMap map[string][]float64 // code -> []close per date (forward-filled)
	OpenMap  map[string][]float64 // code -> []open per date (forward-filled)
}

// GetClose returns the close price for a stock on a given date (O(1) lookup).
func (kc *KlineCache) GetClose(code, date string) float64 {
	arr, ok := kc.CloseMap[code]
	if !ok {
		return 0
	}
	idx, ok := kc.DateIdx[date]
	if !ok {
		return 0
	}
	return arr[idx]
}

// LoadKlineCache loads all K-line data for the given stock codes and builds the cache.
// Performs forward-filling to handle gaps (suspended stocks, missing data).
func (s *DataLoaderService) LoadKlineCache(codes []string, startDate, endDate string) (*KlineCache, error) {
	kc := &KlineCache{
		DateIdx:  make(map[string]int),
		CloseMap: make(map[string][]float64, len(codes)),
		OpenMap:  make(map[string][]float64, len(codes)),
	}

	// 1. Get all trading dates
	dates, err := s.klineRepo.GetTradingDates(startDate, endDate)
	if err != nil {
		log.Printf("[data_loader] GetTradingDates failed: %v", err)
		return kc, err
	}
	kc.Dates = dates
	for i, d := range dates {
		kc.DateIdx[d] = i
	}

	if len(dates) == 0 {
		return kc, nil
	}

	// 2. Bulk load close + open prices
	rows, err := s.klineRepo.BulkLoadCloses(codes, startDate, endDate)
	if err != nil {
		log.Printf("[data_loader] BulkLoadCloses failed: %v", err)
		return kc, err
	}

	// 3. Initialize arrays
	nDays := len(dates)
	for _, c := range codes {
		kc.CloseMap[c] = make([]float64, nDays)
		kc.OpenMap[c] = make([]float64, nDays)
	}

	// 4. Fill prices
	for _, r := range rows {
		if idx, ok := kc.DateIdx[r.Date]; ok {
			kc.CloseMap[r.Code][idx] = r.Close
			kc.OpenMap[r.Code][idx] = r.Open
		}
	}

	// 5. Forward-fill close and open prices
	for _, c := range codes {
		arr := kc.CloseMap[c]
		var last float64
		for i := 0; i < nDays; i++ {
			if arr[i] > 0 {
				last = arr[i]
			} else {
				arr[i] = last
			}
		}
		arrO := kc.OpenMap[c]
		var lastO float64
		for i := 0; i < nDays; i++ {
			if arrO[i] > 0 {
				lastO = arrO[i]
			} else {
				arrO[i] = lastO
			}
		}
	}

	log.Printf("[data_loader] Loaded K-line cache: %d codes, %d dates", len(codes), nDays)
	return kc, nil
}

// LoadStockUniverse loads the stock universe for backtesting.
// If stockCodes is provided, filters to those codes (minus ST stocks).
// Otherwise, loads all stocks with data in the date range (up to limit).
func (s *DataLoaderService) LoadStockUniverse(stockCodes []string, startDate, endDate string, limit int) ([]StockInfo, error) {
	if len(stockCodes) > 0 {
		rows, err := s.klineRepo.GetStockUniverseFiltered(stockCodes)
		if err != nil {
			return nil, err
		}
		result := make([]StockInfo, len(rows))
		for i, r := range rows {
			result[i] = StockInfo{Code: r.Code, Name: r.Name}
		}
		return result, nil
	}

	rows, err := s.klineRepo.GetStockUniverse(startDate, endDate, limit)
	if err != nil {
		return nil, err
	}
	result := make([]StockInfo, len(rows))
	for i, r := range rows {
		result[i] = StockInfo{Code: r.Code, Name: r.Name}
	}
	return result, nil
}

// StockInfo represents a stock in the backtest universe.
type StockInfo struct {
	Code string
	Name string
}
