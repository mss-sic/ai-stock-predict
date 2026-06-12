package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/gin-gonic/gin"
)

type InternalHandler struct{}

func NewInternalHandler() *InternalHandler { return &InternalHandler{} }

// SyncPredictions accepts algorithm team's JSON and stores 7 KD curves as model1-model7.
// Batch-optimized for large files (5000+ stocks).
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
	fileName := c.Query("filename")
	if fileName == "" {
		fileName = "prediction_import"
	}

	// ── Step 1: Batch load all lastClose in ONE query ──
	codes := make([]string, 0, len(input.DataUnits))
	codeSet := make(map[string]bool)
	for _, u := range input.DataUnits {
		if u.StockCode != "" && !codeSet[u.StockCode] {
			codes = append(codes, u.StockCode)
			codeSet[u.StockCode] = true
		}
	}

	closeMap := make(map[string]float64, len(codes))
	if len(codes) > 0 {
		// Use string_to_array+unnest for safe large IN queries with GORM
		codesStr := strings.Join(codes, ",")
		rows, err := db.PG.Raw(`
			SELECT code, close FROM (
				SELECT DISTINCT ON (code) code, close
				FROM stocks_daily_k
				WHERE code IN (SELECT unnest(string_to_array(?, ',')))
				ORDER BY code, trade_date DESC
			) sub
		`, codesStr).Rows()
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code string
				var close float64
				rows.Scan(&code, &close)
				closeMap[code] = close
			}
		} else {
			log.Printf("[internal] batch close query failed: %v", err)
		}
	}

	// ── Step 2: Build all records in memory, then bulk insert ──
	type predRec struct {
		Code, ModelName, PredictDate string
		PredictedPrice, Upper, Lower float64
	}
	var preds []predRec
	type kdRec struct {
		Code, KDData string
	}
	var kds []kdRec
	type sigRec struct {
		Code        string
		SignalValue float64
	}
	var sigs []sigRec

	skipped := 0

	for _, unit := range input.DataUnits {
		code := unit.StockCode
		if code == "" {
			skipped++
			continue
		}

		lastClose, ok := closeMap[code]
		if !ok || lastClose <= 0 {
			skipped++
			continue
		}

		if len(unit.KdistributedData) < 7 {
			skipped++
			continue
		}

		// Prediction records
		for ki, kdCurve := range unit.KdistributedData {
			modelName := fmt.Sprintf("model%d", ki+1)
			for day, kdVal := range kdCurve {
				predPrice := lastClose * (1 + kdVal/100)
				predictDate := today.AddDate(0, 0, day+1).Format("2006-01-02")
				preds = append(preds, predRec{
					Code:           code,
					ModelName:      modelName,
					PredictDate:    predictDate,
					PredictedPrice: predPrice,
					Upper:          predPrice * 1.02,
					Lower:          predPrice * 0.98,
				})
			}
		}

		// KD data
		kdJSON, _ := json.Marshal(unit.KdistributedData)
		kds = append(kds, kdRec{Code: code, KDData: string(kdJSON)})

		// Signal
		conf, _ := unit.Confidence.Float64()
		tw, _ := unit.TodayWave.Float64()
		tm, _ := unit.TodayTradeMoney.Float64()
		tr, _ := unit.TodayTradeRate.Float64()
		sigs = append(sigs, sigRec{Code: code, SignalValue: conf + tw + tm + tr})
	}

	imported := len(preds)

	// ── Step 3: Clear old data ──
	db.PG.Exec("DELETE FROM predictions")
	db.PG.Exec("DELETE FROM prediction_kdist")
	db.PG.Exec("DELETE FROM stock_signals WHERE source = 'algo_team'")

	// ── Step 4: Bulk insert predictions ──
	batchSize := 2000
	for i := 0; i < len(preds); i += batchSize {
		end := i + batchSize
		if end > len(preds) {
			end = len(preds)
		}
		batch := preds[i:end]
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*6)
		for j, p := range batch {
			base := j * 6
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5, base+6))
			valueArgs = append(valueArgs, p.Code, p.ModelName, p.PredictDate,
				p.PredictedPrice, p.Upper, p.Lower)
		}
		query := fmt.Sprintf(`
			INSERT INTO predictions (code, model_name, predict_date, predicted_price, upper_bound, lower_bound)
			VALUES %s
			ON CONFLICT (code, model_name, predict_date) DO UPDATE SET
				predicted_price = EXCLUDED.predicted_price,
				upper_bound = EXCLUDED.upper_bound,
				lower_bound = EXCLUDED.lower_bound
		`, strings.Join(valueStrings, ","))
		if err := db.PG.Exec(query, valueArgs...).Error; err != nil {
			log.Printf("[internal] batch insert predictions failed at batch %d: %v", i/batchSize, err)
		}
	}

	// ── Step 5: Bulk upsert kdist ──
	for i := 0; i < len(kds); i += batchSize {
		end := i + batchSize
		if end > len(kds) {
			end = len(kds)
		}
		batch := kds[i:end]
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*3)
		for j, k := range batch {
			base := j * 3
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,NOW())", base+1, base+2))
			valueArgs = append(valueArgs, k.Code, k.KDData)
		}
		query := fmt.Sprintf(`
			INSERT INTO prediction_kdist (code, kd_data, updated_at)
			VALUES %s
			ON CONFLICT (code) DO UPDATE SET kd_data = EXCLUDED.kd_data, updated_at = NOW()
		`, strings.Join(valueStrings, ","))
		if err := db.PG.Exec(query, valueArgs...).Error; err != nil {
			log.Printf("[internal] batch insert kdist failed at batch %d: %v", i/batchSize, err)
		}
	}

	// ── Step 6: Bulk upsert signals ──
	for i := 0; i < len(sigs); i += batchSize {
		end := i + batchSize
		if end > len(sigs) {
			end = len(sigs)
		}
		batch := sigs[i:end]
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*2)
		for j, s := range batch {
			base := j * 2
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d)", base+1, base+2))
			valueArgs = append(valueArgs, s.Code, s.SignalValue)
		}
		query := fmt.Sprintf(`
			INSERT INTO stock_signals (code, signal_value, source, updated_at)
			VALUES %s
			ON CONFLICT (code) DO UPDATE SET signal_value = EXCLUDED.signal_value, source = 'algo_team', updated_at = NOW()
		`, strings.Join(valueStrings, ","))
		if err := db.PG.Exec(query, valueArgs...).Error; err != nil {
			log.Printf("[internal] batch insert signals failed at batch %d: %v", i/batchSize, err)
		}
	}

	// ── Step 7: Import log ──
	status := "success"
	if skipped > 0 && imported == 0 {
		status = "failed"
	} else if skipped > 0 {
		status = "partial"
	}
	importLog := model.ImportLog{
		FileName:     fileName,
		RowsImported: imported,
		Status:       status,
		ImportedAt:   time.Now(),
	}
	if db.MySQL != nil {
		db.MySQL.Create(&importLog)
	} else {
		db.PG.Create(&importLog)
	}

	log.Printf("[internal] SyncPredictions done: %d predictions, %d kdist, %d signals (%d skipped)", imported, len(kds), len(sigs), skipped)

	c.JSON(200, gin.H{
		"success":  true,
		"imported": imported,
		"skipped":  skipped,
		"total":    len(input.DataUnits),
	})
}
