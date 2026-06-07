package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"context"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/ai-stock-predict/server/pkg/response"
)

type PredictionHandler struct{}

func NewPredictionHandler() *PredictionHandler { return &PredictionHandler{} }

var predictScriptsDir string

func init() {
	_, f, _, _ := runtime.Caller(0)
	base := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(f))))
	predictScriptsDir = filepath.Join(base, "scripts", "predict")
}

var models = []string{"gru", "lstm", "xgb", "arima", "transformer", "prophet"}

func runPredictScript(modelName string, args ...string) ([]map[string]float64, error) {
	scriptPath := filepath.Join(predictScriptsDir, modelName+"_predict.py")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.CommandContext(ctx, "python3", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %s", err.Error(), string(exitErr.Stderr))
		}
		return nil, err
	}
	var result []map[string]float64
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w, output: %s", err, string(out))
	}
	return result, nil
}

// RunAll runs all 6 prediction models for a stock
func (h *PredictionHandler) RunAll(c *gin.Context) {
	code := c.Param("code")
	horizon := c.DefaultQuery("horizon", "10")

	type ModelResult struct {
		Model      string                `json:"model"`
		Success    bool                  `json:"success"`
		Error      string                `json:"error,omitempty"`
		Prediction []map[string]float64 `json:"prediction,omitempty"`
	}

	results := make([]ModelResult, len(models))
	var wg sync.WaitGroup
	for i, m := range models {
		wg.Add(1)
		go func(idx int, modelName string) {
			defer wg.Done()
			pred, err := runPredictScript(modelName, code, horizon)
			if err != nil {
				results[idx] = ModelResult{Model: modelName, Success: false, Error: err.Error()}
				return
			}
			results[idx] = ModelResult{Model: modelName, Success: true, Prediction: pred}
			// Save to DB
			savePredictions(code, modelName, pred)
		}(i, m)
	}
	wg.Wait()

	response.Success(c, gin.H{"predictions": results, "stockCode": code})
}

// Batch runs predictions for all stocks in today's board
func (h *PredictionHandler) Batch(c *gin.Context) {
	var body struct {
		Codes   []string `json:"codes"`
		Horizon int      `json:"horizon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if body.Horizon == 0 {
		body.Horizon = 10
	}
	horizon := fmt.Sprintf("%d", body.Horizon)

	success := 0
	fails := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, code := range body.Codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			for _, m := range models {
				pred, err := runPredictScript(m, c, horizon)
				if err != nil {
					log.Printf("predict %s %s: %v", c, m, err)
					mu.Lock()
					fails++
					mu.Unlock()
					continue
				}
				savePredictions(c, m, pred)
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(code)
	}
	wg.Wait()

	c.JSON(http.StatusOK, gin.H{"success": success, "fails": fails, "total": success + fails})
}

// GetResult returns stored predictions for a stock
// HitRate calculates real hit rates by comparing historical predictions vs actual prices
func (h *PredictionHandler) HitRate(c *gin.Context) {
	code := c.Param("code")
	type ModelHitRate struct {
		ModelName string  `json:"modelName"`
		HitRate   float64 `json:"hitRate"`
		Total     int     `json:"total"`
		Hits      int     `json:"hits"`
	}
	var results []ModelHitRate
	models := []string{"model1", "model2", "model3", "model4", "model5", "model6", "model7"}
	for _, model := range models {
		var total, hits int
		rows, err := db.PG.Raw(`SELECT p.predict_date, p.predicted_price, k.close AS actual_close, k.trade_date
			FROM predictions p
			LEFT JOIN LATERAL (
				SELECT close, trade_date FROM stocks_daily_k
				WHERE code = p.code AND trade_date = p.predict_date
				LIMIT 1
			) k ON true
			WHERE p.code = ? AND p.model_name = ? AND p.predict_date <= CURRENT_DATE
			ORDER BY p.predict_date`, code, model).Rows()
		if err != nil {
			continue
		}
		var prevPrice, prevPred, prevClose float64
		for rows.Next() {
			var predictDate, tradeDate time.Time
			var predPrice, actualClose float64
			rows.Scan(&predictDate, &predPrice, &actualClose, &tradeDate)
			if actualClose == 0 {
				continue
			}
			// Compare direction: predicted direction vs actual direction from previous day
			if !tradeDate.IsZero() {
				prevClose = actualClose
			}
			// Direction: prediction says up if predicted > last known close
			// We use the previous row close as reference
			if prevPrice > 0 {
				total++
				predUp := predPrice > prevPrice
				actualUp := actualClose > prevClose
				if predUp == actualUp || (predPrice == prevPrice && actualClose == prevClose) {
					hits++
				}
			}
			prevPrice = actualClose
			if prevPred == 0 {
				prevPred = predPrice
			}
			_ = prevPred
		}
		rows.Close()
		hitRate := 0.0
		if total > 0 {
			hitRate = float64(hits) / float64(total)
		}
		results = append(results, ModelHitRate{
			ModelName: model,
			HitRate:   math.Round(hitRate*10000) / 10000,
			Total:     total,
			Hits:      hits,
		})
	}
	response.Success(c, gin.H{"hitRates": results, "stockCode": code})
}


func (h *PredictionHandler) GetResult(c *gin.Context) {
	code := c.Param("code")
	var predictions []model.Prediction
	db.PG.Where("code = ?", code).Order("predict_date ASC").Find(&predictions)

	// Include kdistributed_data for chart overlay
	var kdist model.PredictionKDist
	db.PG.Where("code = ?", code).First(&kdist)

	response.Success(c, gin.H{
		"predictions": predictions,
		"stockCode":   code,
		"kdData":      kdist.KDData,
	})
}

func savePredictions(code, modelName string, pred []map[string]float64) {
	for _, p := range pred {
		day := int(p["day"])
		predictDate := time.Now().AddDate(0, 0, day)
		rec := model.Prediction{
			Code:           code,
			ModelName:      modelName,
			PredictDate:    predictDate,
			PredictedPrice: p["price"],
			UpperBound:     p["upper"],
			LowerBound:     p["lower"],
		}
		db.PG.Where("code = ? AND model_name = ? AND predict_date = ?", code, modelName, predictDate.Format("2006-01-02")).
			Assign(rec).FirstOrCreate(&rec)
	}
}
