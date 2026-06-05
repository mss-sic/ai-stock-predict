package repository

import (
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type BoardRepo struct{}

func (r *BoardRepo) GetTodayBoard() ([]model.AlgorithmPickDetail, error) {
	today := time.Now().Format("2006-01-02")
	return r.GetBoardByDate(today)
}

func (r *BoardRepo) GetBoardByDate(date string) ([]model.AlgorithmPickDetail, error) {
	var details []model.AlgorithmPickDetail
	err := db.PG.Where("pick_date = ?", date).Order("rank ASC").Find(&details).Error
	return details, err
}

func (r *BoardRepo) GetHeatmapData(from, to string) ([]model.AlgorithmPickDetail, error) {
	var details []model.AlgorithmPickDetail
	err := db.PG.Where("pick_date >= ? AND pick_date <= ?", from, to).
		Order("pick_date ASC, rank ASC").Find(&details).Error
	return details, err
}

func (r *BoardRepo) GetStockHeatmap(code string) ([]model.AlgorithmPickDetail, error) {
	var details []model.AlgorithmPickDetail
	err := db.PG.Where("stock_code = ?", code).
		Order("pick_date DESC").Limit(60).Find(&details).Error
	return details, err
}

func (r *BoardRepo) UpsertBoard(date time.Time, details []model.AlgorithmPickDetail) error {
	tx := db.PG.Begin()
	pick := model.AlgorithmPick{PickDate: date, TotalStocks: len(details)}
	if err := tx.Where("pick_date = ?", date).Assign(pick).FirstOrCreate(&pick).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, d := range details {
		d.PickDate = date
		if err := tx.Where("pick_date = ? AND stock_code = ?", date, d.StockCode).
			Assign(d).FirstOrCreate(&d).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
