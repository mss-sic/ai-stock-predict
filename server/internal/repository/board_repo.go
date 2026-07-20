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

func (r *BoardRepo) GetEnrichedHeatmap(from, to string) ([]model.HeatmapEnriched, error) {
	var rows []model.HeatmapEnriched
	err := db.PG.Raw(`
		SELECT 
			a.pick_date,
			a.stock_code,
			COALESCE(s.name, a.stock_code) AS stock_name,
			a.rank,
			COALESCE(a.score, 0) AS score,
			COALESCE(k.open, 0) AS open,
			COALESCE(k.close, 0) AS close,
			CASE WHEN k.open > 0 THEN ROUND(((k.close - k.open) / k.open * 100)::numeric, 2) ELSE 0 END AS chg_pct,
			COALESCE(today.chg_pct, 0) AS today_chg_pct
		FROM algorithm_pick_details a
		LEFT JOIN stocks_basic s ON s.code = a.stock_code
		LEFT JOIN stocks_daily_k k ON k.code = a.stock_code AND k.trade_date = a.pick_date
		LEFT JOIN LATERAL (
			SELECT change_pct AS chg_pct
			FROM stocks_daily_k
			WHERE code = a.stock_code
			ORDER BY trade_date DESC LIMIT 1
		) today ON true
		WHERE a.pick_date >= ? AND a.pick_date <= ?
		ORDER BY a.pick_date ASC, a.rank ASC
	`, from, to).Scan(&rows).Error
	return rows, err
}
