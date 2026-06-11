package handler

import (
	"log"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/gin-gonic/gin"
)

type InternalHandler struct{}

func NewInternalHandler() *InternalHandler { return &InternalHandler{} }

// SyncPredictions accepts algorithm team's JSON and stores 7 KD curves as model1-model7
func (h *InternalHandler) SyncPredictions(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "read body failed"})
		return
	}

	var input struct {
		TotalUnitsNumber int `json:"total_units_number"`
		Kdis             int `json:"kdis"`
		MaxPredictDay    int `json:"max_predict_day"`
		DataUnits        []struct {
			Index            int         `json:"index"`
			StockCode        string      `json:"stock_code"`
			StockName        string      `json:"stock_name"`
			Confidence       json.Number `json:"confidence"`
			TodayWave        json.Number `json:"today_wave"`
			TodayTradeMoney  json.Number `json:"today_trade_money"`
			TodayTradeRate   json.Number `json:"today_trade_rate"`
			RealWave         []float64   `json:"real_wave"`
			KdistributedData [][]float64 `json:"kdistributed_data"`
		} `json:"data_units"`
	}

	if err := json.Unmarshal(body, &input); err != nil {
		c.JSON(400, gin.H{"error": "invalid json: " + err.Error()})
		return
	}

	if len(input.DataUnits) == 0 {
		c.JSON(400, gin.H{"error": "empty data_units"})
		return
	}

	today := time.Now().Truncate(24 * time.Hour)

	// Clear ALL old prediction data (ignore errors if tables are empty)
	db.PG.Exec("DELETE FROM predictions")
	db.PG.Exec("DELETE FROM prediction_kdist")
	db.PG.Exec("DELETE FROM stock_signals WHERE source = 'algo_team'")

	imported := 0
	skipped := 0

	for _, unit := range input.DataUnits {
		code := unit.StockCode
		if code == "" {
			skipped++
			continue
		}

		// Get latest close price
		var lastClose float64
		if err := db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&lastClose).Error; err != nil {
			log.Printf("[internal] lastClose query failed for %s: %v", code, err)
		}
		if lastClose <= 0 {
			skipped++
			continue
		}

		// Store 7 KD curves as model1-model7
		if len(unit.KdistributedData) < 7 {
			skipped++
			continue
		}

		for ki, kdCurve := range unit.KdistributedData {
			modelName := fmt.Sprintf("model%d", ki+1)
			for day, kdVal := range kdCurve {
				predPrice := lastClose * (1 + kdVal/100)
				predictDate := today.AddDate(0, 0, day+1)

				rec := model.Prediction{
					Code:           code,
					ModelName:      modelName,
					PredictDate:    predictDate,
					PredictedPrice: predPrice,
					UpperBound:     predPrice * 1.02,
					LowerBound:     predPrice * 0.98,
				}

				// Use Assign + FirstOrCreate for proper upsert
				db.PG.Where("code = ? AND model_name = ? AND predict_date = ?",
					code, modelName, predictDate.Format("2006-01-02")).
					Assign(rec).FirstOrCreate(&rec)
				imported++
			}
		}

		// Store kdistributed_data as JSON
		kdJSON, _ := json.Marshal(unit.KdistributedData)
		db.PG.Exec(`INSERT INTO prediction_kdist (code, kd_data, updated_at)
			VALUES (?, ?, NOW())
			ON CONFLICT (code) DO UPDATE SET kd_data = ?, updated_at = NOW()`,
			code, string(kdJSON), string(kdJSON))

		// Store signal
		conf, _ := unit.Confidence.Float64()
		tw, _ := unit.TodayWave.Float64()
		tm, _ := unit.TodayTradeMoney.Float64()
		tr, _ := unit.TodayTradeRate.Float64()

		db.PG.Exec(`INSERT INTO stock_signals (code, signal_value, source, updated_at)
			VALUES (?, ?, 'algo_team', NOW())
			ON CONFLICT (code) DO UPDATE SET signal_value = ?, source = 'algo_team', updated_at = NOW()`,
			code, conf+tw+tm+tr, conf+tw+tm+tr)
	}

	// Write import log
	fileName := c.Query("filename")
	if fileName == "" {
		fileName = "prediction_import"
	}
	status := "success"
	if skipped > 0 && imported == 0 {
		status = "failed"
	} else if skipped > 0 {
		status = "partial"
	}
	log := model.ImportLog{
		FileName:     fileName,
		RowsImported: imported,
		Status:       status,
		ImportedAt:   time.Now(),
	}
	if db.MySQL != nil {
		db.MySQL.Create(&log)
	} else {
		db.PG.Create(&log)
	}

	c.JSON(200, gin.H{
		"success":  true,
		"imported": imported,
		"skipped":  skipped,
		"total":    len(input.DataUnits),
	})
}
