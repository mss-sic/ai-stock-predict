package service

import (
	"time"

	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/repository"
)

type StockService struct {
	repo *repository.StockRepo
}

func NewStockService() *StockService { return &StockService{repo: &repository.StockRepo{}} }

func (s *StockService) List(industry, keyword string, page, pageSize int) ([]model.StockBasic, int64, error) {
	offset := (page - 1) * pageSize
	return s.repo.List(industry, keyword, offset, pageSize)
}

func (s *StockService) GetDetail(code string) (*model.StockBasic, error) {
	return s.repo.GetByCode(code)
}

func (s *StockService) GetKLine(code string, from, to string) ([]model.StockDailyK, error) {
	var f, t time.Time
	if from != "" { f, _ = time.Parse("2006-01-02", from) }
	if to != "" { t, _ = time.Parse("2006-01-02", to) }
	if f.IsZero() { f = time.Now().AddDate(0, 0, -90) }
	return s.repo.GetKLine(code, f, t)
}

func (s *StockService) GetIndicator(code string) (*model.StockDailyIndicator, error) {
	return s.repo.GetIndicator(code, time.Now())
}

func (s *StockService) GetSignal(code string) (*model.StockSignal, error) {
	return s.repo.GetSignal(code)
}
