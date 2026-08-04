package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
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

	// ── Step 3: Clear old data in a transaction ──
	tx := db.PG.Begin()
	tx.Exec("DELETE FROM predictions")
	tx.Exec("DELETE FROM prediction_kdist")
	tx.Exec("DELETE FROM stock_signals WHERE source = 'algo_team'")
	if err := tx.Commit().Error; err != nil {
		log.Printf("[internal] clear old data transaction failed: %v", err)
		c.JSON(500, gin.H{"error": "clear old data failed"})
		return
	}

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
		now := time.Now()
		for j, k := range batch {
			base := j * 3
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d)", base+1, base+2, base+3))
			valueArgs = append(valueArgs, k.Code, k.KDData, now)
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

	// ── Step 6.5: Precompute KD factors for prediction_factors table ──
	type factorRec struct {
		Code                                    string
		ConsensusD5, ConsensusD10, ConsensusD20 int
		ExpReturnD5, ExpReturnD10, ExpReturnD20 float64
		MomentumD5, MomentumD10, MomentumD20    float64
		StddevD20                               float64
	}
	var factors []factorRec
	for _, unit := range input.DataUnits {
		if len(unit.KdistributedData) < 7 {
			continue
		}
		var d5, d10, d20, moms []float64
		var cons5, cons10, cons20 int
		for _, c := range unit.KdistributedData {
			if len(c) < 20 {
				continue
			}
			d5v, d10v, d20v := c[4], c[9], c[19]
			if d5v > 0 {
				cons5++
			}
			if d10v > 0 {
				cons10++
			}
			if d20v > 0 {
				cons20++
			}
			d5 = append(d5, d5v)
			d10 = append(d10, d10v)
			d20 = append(d20, d20v)
			// momentum: last5 avg - first5 avg
			first5, last5 := 0.0, 0.0
			for j := 0; j < 5; j++ {
				first5 += c[j]
			}
			for j := 15; j < 20; j++ {
				last5 += c[j]
			}
			moms = append(moms, last5/5.0-first5/5.0)
		}
		f := factorRec{Code: unit.StockCode, ConsensusD5: cons5, ConsensusD10: cons10, ConsensusD20: cons20}
		if len(d5) > 0 {
			sum := 0.0
			for _, v := range d5 {
				sum += v
			}
			f.ExpReturnD5 = sum / float64(len(d5))
			sum = 0.0
			for _, v := range d10 {
				sum += v
			}
			f.ExpReturnD10 = sum / float64(len(d10))
			sum = 0.0
			for _, v := range d20 {
				sum += v
			}
			f.ExpReturnD20 = sum / float64(len(d20))
			// stddev d20
			sq := 0.0
			for _, v := range d20 {
				d := v - f.ExpReturnD20
				sq += d * d
			}
			if len(d20) > 1 {
				f.StddevD20 = math.Sqrt(sq / float64(len(d20)))
			}
		}
		if len(moms) > 0 {
			sum := 0.0
			for _, v := range moms {
				sum += v
			}
			avg := sum / float64(len(moms))
			f.MomentumD5, f.MomentumD10, f.MomentumD20 = avg, avg, avg
		}
		factors = append(factors, f)
	}

	// Batch upsert into prediction_factors
	factorBatchSize := 2000
	for i := 0; i < len(factors); i += factorBatchSize {
		end := i + factorBatchSize
		if end > len(factors) {
			end = len(factors)
		}
		batch := factors[i:end]
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*11)
		for j, f := range batch {
			base := j * 11
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,NOW())",
					base+1, base+2, base+3, base+4, base+5, base+6,
					base+7, base+8, base+9, base+10, base+11))
			valueArgs = append(valueArgs, f.Code,
				f.ConsensusD5, f.ExpReturnD5, f.MomentumD5,
				f.ConsensusD10, f.ExpReturnD10, f.MomentumD10,
				f.ConsensusD20, f.ExpReturnD20, f.MomentumD20,
				f.StddevD20)
		}
		query := fmt.Sprintf(`
			INSERT INTO prediction_factors (code, consensus_d5, exp_return_d5, momentum_d5,
				consensus_d10, exp_return_d10, momentum_d10,
				consensus_d20, exp_return_d20, momentum_d20,
				stddev_d20, updated_at)
			VALUES %s
			ON CONFLICT (code) DO UPDATE SET
				consensus_d5 = EXCLUDED.consensus_d5, exp_return_d5 = EXCLUDED.exp_return_d5, momentum_d5 = EXCLUDED.momentum_d5,
				consensus_d10 = EXCLUDED.consensus_d10, exp_return_d10 = EXCLUDED.exp_return_d10, momentum_d10 = EXCLUDED.momentum_d10,
				consensus_d20 = EXCLUDED.consensus_d20, exp_return_d20 = EXCLUDED.exp_return_d20, momentum_d20 = EXCLUDED.momentum_d20,
				stddev_d20 = EXCLUDED.stddev_d20, updated_at = NOW()
		`, strings.Join(valueStrings, ","))
		if err := db.PG.Exec(query, valueArgs...).Error; err != nil {
			log.Printf("[internal] batch insert factors failed at batch %d: %v", i/factorBatchSize, err)
			response.InternalError(c, "预测因子写入失败")
			return
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
