package service

import (
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/repository"
)

type QuoteService struct {
	repo *repository.QuoteRepo
}

func NewQuoteService() *QuoteService { return &QuoteService{repo: &repository.QuoteRepo{}} }

func (s *QuoteService) GetQuote(code string) (*model.StockQuote, error) {
	return s.repo.GetByCode(code)
}
