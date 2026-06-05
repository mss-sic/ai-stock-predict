package repository

import (
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type StockRepo struct{}

func (r *StockRepo) List(industry string, keyword string, offset, limit int) ([]model.StockBasic, int64, error) {
	var stocks []model.StockBasic
	var total int64
	q := db.PG.Model(&model.StockBasic{})
	if industry != "" {
		q = q.Where("industry = ?", industry)
	}
	if keyword != "" {
		q = q.Where("code LIKE ? OR name LIKE ?", keyword+"%", "%"+keyword+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Offset(offset).Limit(limit).Find(&stocks).Error; err != nil {
		return nil, 0, err
	}
	return stocks, total, nil
}

func (r *StockRepo) GetByCode(code string) (*model.StockBasic, error) {
	var stock model.StockBasic
	err := db.PG.Where("code = ?", code).First(&stock).Error
	return &stock, err
}

func (r *StockRepo) GetKLine(code string, from, to time.Time) ([]model.StockDailyK, error) {
	var klines []model.StockDailyK
	q := db.PG.Where("code = ?", code)
	if !from.IsZero() {
		q = q.Where("trade_date >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("trade_date <= ?", to)
	}
	err := q.Order("trade_date ASC").Find(&klines).Error
	return klines, err
}

func (r *StockRepo) GetIndicator(code string, date time.Time) (*model.StockDailyIndicator, error) {
	var ind model.StockDailyIndicator
	err := db.PG.Where("code = ? AND trade_date = ?", code, date).First(&ind).Error
	return &ind, err
}

func (r *StockRepo) UpsertBasic(stock *model.StockBasic) error {
	return db.PG.Where("code = ?", stock.Code).Assign(stock).FirstOrCreate(stock).Error
}

func (r *StockRepo) UpsertDailyK(k *model.StockDailyK) error {
	return db.PG.Where("code = ? AND trade_date = ?", k.Code, k.TradeDate).Assign(k).FirstOrCreate(k).Error
}

func (r *StockRepo) GetSignal(code string) (*model.StockSignal, error) {
	var signal model.StockSignal
	err := db.PG.Where("code = ?", code).First(&signal).Error
	return &signal, err
}
