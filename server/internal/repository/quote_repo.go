package repository

import (
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type QuoteRepo struct{}

func (r *QuoteRepo) GetByCode(code string) (*model.StockQuote, error) {
	var q model.StockQuote
	err := db.PG.Where("code = ?", code).First(&q).Error
	return &q, err
}
