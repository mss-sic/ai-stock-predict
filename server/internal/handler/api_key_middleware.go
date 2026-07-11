package handler

import (
	"crypto/rand"
	"path/filepath"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/gin-gonic/gin"
)

// ── API Key generation ──

// GenerateAPIKey creates a new random API key string (prefix + random).
// Returns the full key to show once (stored as SHA-256 hash).
func GenerateAPIKey() (fullKey string, keyHash string, keyPrefix string) {
	b := make([]byte, 32)
	rand.Read(b)
	fullKey = "ak-" + hex.EncodeToString(b)
	h := sha256.Sum256([]byte(fullKey))
	keyHash = hex.EncodeToString(h[:])
	keyPrefix = fullKey[:11] // "ak-" + 8 hex chars
	return
}

// ── API Key Middleware ──

// APIKeyAuth validates X-API-Key header against stored keys.
// Sets "teamName" and "permissions" in context on success.
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			response.Unauthorized(c, "缺少 API Key (X-API-Key header)")
			c.Abort()
			return
		}

		h := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(h[:])

		var key model.ApiKey
		if err := db.MySQL.Where("key_hash = ? AND is_active = true", keyHash).First(&key).Error; err != nil {
			log.Printf("[api-key] invalid key attempt from %s", c.ClientIP())
			response.Unauthorized(c, "API Key 无效或已停用")
			c.Abort()
			return
		}

		// Update last used timestamp
		now := time.Now()
		db.MySQL.Model(&key).Update("last_used_at", now)

		// Parse permissions
		var perms []string
		if key.Permissions != "" {
			json.Unmarshal([]byte(key.Permissions), &perms)
		}

		c.Set("apiTeamName", key.TeamName)
		c.Set("apiPermissions", perms)
		c.Next()
	}
}

// ── API Key Admin CRUD ──

// ListAPIKeys returns all API keys (without hash) for the admin panel.
func ListAPIKeys(c *gin.Context) {
	var keys []model.ApiKey
	db.MySQL.Order("created_at DESC").Find(&keys)
	response.Success(c, keys)
}

type createAPIKeyBody struct {
	TeamName    string   `json:"teamName"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// CreateAPIKey generates a new API key and returns it once.
func CreateAPIKey(c *gin.Context) {
	var body createAPIKeyBody
	if err := c.ShouldBindJSON(&body); err != nil || body.TeamName == "" {
		response.BadRequest(c, "团队名称(teamName)必填")
		return
	}

	if len(body.Permissions) == 0 {
		body.Permissions = []string{"prediction", "indicator", "kline", "profile"}
	}

	permsJSON, _ := json.Marshal(body.Permissions)

	fullKey, keyHash, keyPrefix := GenerateAPIKey()

	apiKey := model.ApiKey{
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		TeamName:    body.TeamName,
		Description: body.Description,
		Permissions: string(permsJSON),
		IsActive:    true,
	}

	if err := db.MySQL.Create(&apiKey).Error; err != nil {
		response.InternalError(c, "创建 API Key 失败")
		return
	}

	log.Printf("[api-key] created key for team=%s id=%d", body.TeamName, apiKey.ID)

	response.Created(c, gin.H{
		"id":          apiKey.ID,
		"apiKey":      fullKey,
		"keyPrefix":   keyPrefix,
		"teamName":    apiKey.TeamName,
		"description": apiKey.Description,
		"permissions": body.Permissions,
		"warning":     "请立即复制保存，此密钥仅显示一次",
	})
}

// UpdateAPIKey toggles active status or updates permissions.
func UpdateAPIKey(c *gin.Context) {
	var body struct {
		IsActive    *bool    `json:"isActive"`
		Permissions []string `json:"permissions"`
		Description *string  `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	id := c.Param("id")
	var key model.ApiKey
	if err := db.MySQL.First(&key, id).Error; err != nil {
		response.NotFound(c, "API Key 不存在")
		return
	}

	updates := map[string]interface{}{}
	if body.IsActive != nil {
		updates["is_active"] = *body.IsActive
	}
	if body.Description != nil {
		updates["description"] = *body.Description
	}
	if body.Permissions != nil {
		pJSON, _ := json.Marshal(body.Permissions)
		updates["permissions"] = string(pJSON)
	}

	if len(updates) > 0 {
		db.MySQL.Model(&key).Updates(updates)
	}

	response.SuccessMsg(c, "更新成功")
}

// DeleteAPIKey permanently removes an API key.
func DeleteAPIKey(c *gin.Context) {
	id := c.Param("id")
	if err := db.MySQL.Delete(&model.ApiKey{}, id).Error; err != nil {
		response.InternalError(c, "删除失败")
		return
	}
	response.SuccessMsg(c, "已删除")
}

// ── Generic Data Import Endpoint ──

// dataImportRequest is the unified JSON import request structure.
type dataImportRequest struct {
	// Type specifies the data category: "prediction", "kline", "indicator", "profile", "signal"
	Type string `json:"type" binding:"required"`
	// Source identifies the origin system/team (auto-filled from API key team name)
	Source string `json:"source"`
	// Data is the actual payload — structure varies by type
	Data json.RawMessage `json:"data" binding:"required"`
	// Options carries type-specific import options
	Options json.RawMessage `json:"options"`
}

// DataImport handles data import authenticated by API key.
// Supports two modes:
//   1. JSON body:   Content-Type: application/json  → {"type":"prediction","data":[...]}
//   2. File upload: Content-Type: multipart/form-data → form field "type" + file field "file"
func DataImport(c *gin.Context) {
	teamName, _ := c.Get("apiTeamName")
	perms, _ := c.Get("apiPermissions")

	var req dataImportRequest
	var logFileName string
	contentType := c.GetHeader("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// ── File upload mode ──
		dataType := c.PostForm("type")
		file, err := c.FormFile("file")
		if err != nil {
			response.BadRequest(c, "请上传文件（form field: file）")
			return
		}
		logFileName = fmt.Sprintf("api:%s:%s", teamName, file.Filename)
		ext := strings.ToLower(filepath.Ext(file.Filename))
		f, err := file.Open()
		if err != nil { response.InternalError(c, "无法读取上传文件"); return }
		defer f.Close()

		// Excel: 榜单数据 → use existing collector parser (data management compatible)
		if ext == ".xlsx" || ext == ".xlsm" {
			result, err := collector.ParseAndImportExcel(f, file.Filename)
			if err != nil { response.InternalError(c, fmt.Sprintf("Excel导入失败: %v", err)); return }
			logImport(teamName, logFileName, result.PicksImported+result.SignalsImported, true)
			response.Success(c, gin.H{
				"datesImported": result.DatesImported, "picksImported": result.PicksImported,
				"signalsImported": result.SignalsImported, "stocksCreated": result.StocksCreated,
				"source": fmt.Sprintf("api:%v", teamName), "team": teamName,
			})
			return
		}

		// CSV: K线数据 → use existing CSV parser (data management compatible)
		if ext == ".csv" {
			result, err := parseKlineCSV(f, file.Filename)
			if err != nil { response.InternalError(c, fmt.Sprintf("CSV导入失败: %v", err)); return }
			logImport(teamName, logFileName, result.ImportedKline+result.ImportedIndic, true)
			response.Success(c, gin.H{
				"imported": result.ImportedKline + result.ImportedIndic,
				"importedKline": result.ImportedKline, "importedIndic": result.ImportedIndic,
				"skipped": result.Skipped, "totalRows": result.TotalRows,
				"tradeDate": result.TradeDate, "source": fmt.Sprintf("api:%v", teamName), "team": teamName,
			})
			return
		}

		// JSON: 预测/指标/研报/信号
		if dataType == "" {
			response.BadRequest(c, "JSON文件请提供 type 表单字段（prediction/kline/indicator/profile/signal）")
			return
		}
		fileData, err := io.ReadAll(f)
		if err != nil { response.InternalError(c, "读取文件内容失败"); return }
		req.Type = dataType
		req.Data = json.RawMessage(fileData)
		req.Source = c.PostForm("source")
		log.Printf("[data-import] team=%v type=%s file=%s size=%d", teamName, dataType, file.Filename, len(fileData))
	} else {
		// ── JSON body mode ──
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求格式错误: type 和 data 为必填字段。也支持 multipart/form-data 文件上传模式。")
			return
		}
		logFileName = fmt.Sprintf("api:%s:%s", teamName, req.Type)
		log.Printf("[data-import] team=%v type=%s size=%d", teamName, req.Type, len(req.Data))
	}

	// Check permission
	if perms != nil {
		allowed := false
		for _, p := range perms.([]string) {
			if p == req.Type || p == "*" { allowed = true; break }
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": fmt.Sprintf("该 API Key 没有导入 %s 类型数据的权限", req.Type)})
			return
		}
	}

	if req.Source == "" { req.Source = fmt.Sprintf("api:%v", teamName) }

	var result gin.H
	var err error
	switch req.Type {
	case "prediction": result, err = importPredictionJSON(req)
	case "kline":       result, err = importKlineJSON(req)
	case "indicator":   result, err = importIndicatorJSON(req)
	case "profile":     result, err = importProfileJSON(req)
	case "signal":      result, err = importSignalJSON(req)
	default:
		response.BadRequest(c, fmt.Sprintf("不支持的数据类型: %s，支持: prediction, kline, indicator, profile, signal", req.Type))
		return
	}

	if err != nil {
		log.Printf("[data-import] error: %v", err)
		response.InternalError(c, fmt.Sprintf("导入失败: %v", err))
		return
	}

	logImport(teamName, logFileName, getImported(result), false)

	result["source"] = req.Source
	result["team"] = teamName
	response.Success(c, result)
}

// ── Type-specific import handlers ──

// predictionImportUnit matches the algorithm team JSON format (same as /internal/predictions/sync).
type predictionImportUnit struct {
	Index            int         `json:"index"`
	StockCode        string      `json:"stock_code"`
	StockName        string      `json:"stock_name"`
	Confidence       json.Number `json:"confidence"`
	TodayWave        json.Number `json:"today_wave"`
	TodayTradeMoney  json.Number `json:"today_trade_money"`
	TodayTradeRate   json.Number `json:"today_trade_rate"`
	RealWave         []float64   `json:"real_wave"`
	KdistributedData [][]float64 `json:"kdistributed_data"`
}

func importPredictionJSON(req dataImportRequest) (gin.H, error) {
	var input struct {
		TotalUnitsNumber int                    `json:"total_units_number"`
		Kdis             int                    `json:"kdis"`
		MaxPredictDay    int                    `json:"max_predict_day"`
		DataUnits        []predictionImportUnit `json:"data_units"`
	}
	if err := json.Unmarshal(req.Data, &input); err != nil {
		return nil, fmt.Errorf("预测数据 JSON 格式错误: %w", err)
	}

	if len(input.DataUnits) == 0 {
		return nil, fmt.Errorf("data_units 为空")
	}

	today := time.Now().Truncate(24 * time.Hour)

	// Batch load lastClose for all codes
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
		}
	}

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

		if len(unit.KdistributedData) < 1 {
			skipped++
			continue
		}

		// Build prediction records from kdistributed_data
		for ki, kdCurve := range unit.KdistributedData {
			modelName := fmt.Sprintf("model%d", ki+1)
			for day, kdVal := range kdCurve {
				predPrice := lastClose * (1 + kdVal/100)
				predictDate := today.AddDate(0, 0, day+1).Format("2006-01-02")
				preds = append(preds, predRec{
					Code: code, ModelName: modelName, PredictDate: predictDate,
					PredictedPrice: predPrice, Upper: predPrice * 1.02, Lower: predPrice * 0.98,
				})
			}
		}

		// KD data for chart overlay
		kdJSON, _ := json.Marshal(unit.KdistributedData)
		kds = append(kds, kdRec{Code: code, KDData: string(kdJSON)})

		// Signal from confidence + wave + trade data
		conf, _ := unit.Confidence.Float64()
		tw, _ := unit.TodayWave.Float64()
		tm, _ := unit.TodayTradeMoney.Float64()
		tr, _ := unit.TodayTradeRate.Float64()
		sigs = append(sigs, sigRec{Code: code, SignalValue: conf + tw + tm + tr})
	}

	imported := len(preds)

	// Batch insert predictions
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
			log.Printf("[data-import] batch insert predictions failed at batch %d: %v", i/batchSize, err)
		}
	}

	// Batch upsert kdist
	now := time.Now()
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
				fmt.Sprintf("($%d,$%d,$%d)", base+1, base+2, base+3))
			valueArgs = append(valueArgs, k.Code, k.KDData, now)
		}
		query := fmt.Sprintf(`
			INSERT INTO prediction_kdist (code, kd_data, updated_at)
			VALUES %s
			ON CONFLICT (code) DO UPDATE SET kd_data = EXCLUDED.kd_data, updated_at = NOW()
		`, strings.Join(valueStrings, ","))
		if err := db.PG.Exec(query, valueArgs...).Error; err != nil {
			log.Printf("[data-import] batch insert kdist failed at batch %d: %v", i/batchSize, err)
		}
	}

	// Batch upsert signals
	for i := 0; i < len(sigs); i += batchSize {
		end := i + batchSize
		if end > len(sigs) {
			end = len(sigs)
		}
		batch := sigs[i:end]
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*2)
		for j, s := range batch {
			base := j * 4
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4))
			valueArgs = append(valueArgs, s.Code, s.SignalValue, req.Source, time.Now())
		}
		query := fmt.Sprintf(`
			INSERT INTO stock_signals (code, signal_value, source, updated_at)
			VALUES %s
			ON CONFLICT (code) DO UPDATE SET signal_value = EXCLUDED.signal_value, source = EXCLUDED.source, updated_at = NOW()
		`, strings.Join(valueStrings, ","))
		if err := db.PG.Exec(query, valueArgs...).Error; err != nil {
			log.Printf("[data-import] batch insert signals failed at batch %d: %v", i/batchSize, err)
		}
	}

	log.Printf("[data-import] prediction done: %d preds, %d kdist, %d signals (%d skipped)", imported, len(kds), len(sigs), skipped)

	return gin.H{"imported": imported, "skipped": skipped, "total": len(input.DataUnits)}, nil
}

// klineEntry is the JSON structure for K-line data import.
type klineEntry struct {
	Code   string       `json:"code"`
	KLines []klineDay   `json:"klines"`
}

type klineDay struct {
	Date         string  `json:"date"`
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	Close        float64 `json:"close"`
	Volume       float64 `json:"volume"`
	Amount       float64 `json:"amount"`
	TurnoverRate float64 `json:"turnoverRate"`
}

func importKlineJSON(req dataImportRequest) (gin.H, error) {
	var entries []klineEntry
	if err := json.Unmarshal(req.Data, &entries); err != nil {
		return nil, fmt.Errorf("K线数据 JSON 格式错误: %w", err)
	}

	imported := 0
	batchSize := 500

	type rec struct {
		Code, Date                                         string
		Open, High, Low, Close, Volume, Amount, TurnoverRate float64
	}
	var batch []rec

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*9)
		for j, r := range batch {
			base := j * 9
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
					base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9))
			valueArgs = append(valueArgs, r.Code, r.Date, r.Open, r.High, r.Low, r.Close, r.Volume, r.Amount, r.TurnoverRate)
		}
		query := fmt.Sprintf(`
			INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, volume, amount, turnover_rate)
			VALUES %s
			ON CONFLICT (code, trade_date) DO UPDATE SET
				open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low, close = EXCLUDED.close,
				volume = EXCLUDED.volume, amount = EXCLUDED.amount, turnover_rate = EXCLUDED.turnover_rate
		`, strings.Join(valueStrings, ","))
		return db.PG.Exec(query, valueArgs...).Error
	}

	for _, entry := range entries {
		code := strings.TrimSpace(entry.Code)
		if len(code) != 6 {
			continue
		}
		for _, k := range entry.KLines {
			batch = append(batch, rec{code, k.Date, k.Open, k.High, k.Low, k.Close, k.Volume, k.Amount, k.TurnoverRate})
			imported++
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					return nil, fmt.Errorf("批量写入K线数据失败: %w", err)
				}
				batch = batch[:0]
			}
		}
	}

	if len(batch) > 0 {
		if err := flush(); err != nil {
			return nil, fmt.Errorf("批量写入K线数据失败: %w", err)
		}
	}

	return gin.H{"imported": imported, "stocks": len(entries)}, nil
}

// indicatorEntry for technical indicator import.
type indicatorEntry struct {
	Code       string              `json:"code"`
	Date       string              `json:"date"`
	Indicators []indicatorValue    `json:"indicators"`
}

type indicatorValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

func importIndicatorJSON(req dataImportRequest) (gin.H, error) {
	var entries []indicatorEntry
	if err := json.Unmarshal(req.Data, &entries); err != nil {
		return nil, fmt.Errorf("指标数据 JSON 格式错误: %w", err)
	}

	imported := 0
	batchSize := 500

	type rec struct {
		Code, Date, Name string
		Value             float64
	}
	var batch []rec

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		for _, r := range batch {
			query := fmt.Sprintf(`
				INSERT INTO stocks_daily_indicator (code, trade_date, %s)
				VALUES ($1, $2, $3)
				ON CONFLICT (code, trade_date) DO UPDATE SET %s = EXCLUDED.%s
			`, r.Name, r.Name, r.Name)
			if err := db.PG.Exec(query, r.Code, r.Date, r.Value).Error; err != nil {
				log.Printf("[data-import] indicator upsert error: code=%s date=%s name=%s err=%v", r.Code, r.Date, r.Name, err)
			}
		}
		return nil
	}

	for _, entry := range entries {
		code := strings.TrimSpace(entry.Code)
		if len(code) != 6 {
			continue
		}
		for _, ind := range entry.Indicators {
			batch = append(batch, rec{code, entry.Date, ind.Name, ind.Value})
			imported++
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					return nil, fmt.Errorf("批量写入指标数据失败: %w", err)
				}
				batch = batch[:0]
			}
		}
	}

	if len(batch) > 0 {
		if err := flush(); err != nil {
			return nil, fmt.Errorf("批量写入指标数据失败: %w", err)
		}
	}

	return gin.H{"imported": imported, "stocks": len(entries)}, nil
}

// apiProfileEntry matches the console file-import JSON format (same as /import/profile endpoint).
type apiProfileEntry struct {
	StockCode       string `json:"stock_code"`
	RawCode         string `json:"raw_code"`
	CompanyName     string `json:"company_name"`
	RawName         string `json:"raw_name"`
	Market          string `json:"market"`
	AnalysisDate    string `json:"analysis_date"`
	AnalysisContent string `json:"analysis_content"`
}

func importProfileJSON(req dataImportRequest) (gin.H, error) {
	var entries []apiProfileEntry
	if err := json.Unmarshal(req.Data, &entries); err != nil {
		return nil, fmt.Errorf("研报数据 JSON 格式错误: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("数据为空")
	}

	imported := 0
	updated := 0
	errors := 0

	for _, e := range entries {
		code := strings.TrimSpace(e.RawCode)
		if code == "" && e.StockCode != "" {
			code = strings.Split(e.StockCode, ".")[0]
		}
		code = strings.TrimLeft(code, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		if len(code) != 6 {
			errors++
			continue
		}

		analyzedAt := time.Now()
		if e.AnalysisDate != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", e.AnalysisDate); err == nil {
				analyzedAt = t
			} else if t, err := time.Parse("2006-01-02", e.AnalysisDate); err == nil {
				analyzedAt = t
			}
		}

		profile := model.StockProfile{
			Code:            code,
			ProfileMarkdown: e.AnalysisContent,
			Source:          req.Source,
			AnalyzedAt:      analyzedAt,
		}

		var existing model.StockProfile
		if err := db.PG.Where("code = ?", code).First(&existing).Error; err == nil {
			db.PG.Model(&existing).Updates(map[string]interface{}{
				"profile_markdown": e.AnalysisContent,
				"source":           req.Source,
				"analyzed_at":      analyzedAt,
				"updated_at":       time.Now(),
			})
			updated++
		} else {
			db.PG.Create(&profile)
			imported++
		}
	}

	return gin.H{"imported": imported, "updated": updated, "errors": errors, "total": len(entries)}, nil
}

// signalEntry for stock signal import.
type signalEntry struct {
	Code        string  `json:"code"`
	SignalValue float64 `json:"signalValue"`
}

func importSignalJSON(req dataImportRequest) (gin.H, error) {
	var entries []signalEntry
	if err := json.Unmarshal(req.Data, &entries); err != nil {
		return nil, fmt.Errorf("信号数据 JSON 格式错误: %w", err)
	}

	imported := 0
	for _, e := range entries {
		code := strings.TrimSpace(e.Code)
		if len(code) != 6 {
			continue
		}
		query := `
			INSERT INTO stock_signals (code, signal_value, source, updated_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (code) DO UPDATE SET signal_value = EXCLUDED.signal_value, source = EXCLUDED.source, updated_at = NOW()
		`
		if err := db.PG.Exec(query, code, e.SignalValue, req.Source).Error; err != nil {
			log.Printf("[data-import] signal upsert error: code=%s err=%v", code, err)
			continue
		}
		imported++
	}

	return gin.H{"imported": imported, "total": len(entries)}, nil
}

// logImport records an import event to the import history.
func logImport(teamName interface{}, fileName string, rows int, success bool) {
	status := "success"
	if !success { status = "failed" }
	importLog := model.ImportLog{
		FileName:     fileName,
		RowsImported: rows,
		Status:       status,
		ImportedAt:   time.Now(),
	}
	if db.MySQL != nil { db.MySQL.Create(&importLog) } else { db.PG.Create(&importLog) }
}

// getImported extracts the imported count from a result map.
func getImported(result gin.H) int {
	if v, ok := result["imported"]; ok {
		if n, ok := v.(int); ok { return n }
	}
	return 0
}

