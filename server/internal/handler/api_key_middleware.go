package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
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

// DataImport handles generic JSON data import authenticated by API key.
func DataImport(c *gin.Context) {
	teamName, _ := c.Get("apiTeamName")
	perms, _ := c.Get("apiPermissions")

	var req dataImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误: type 和 data 为必填字段")
		return
	}

	// Check permission: team must have access to this data type
	if perms != nil {
		allowed := false
		for _, p := range perms.([]string) {
			if p == req.Type || p == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": fmt.Sprintf("该 API Key 没有导入 %s 类型数据的权限", req.Type),
			})
			return
		}
	}

	// Auto-set source from API key team name
	if req.Source == "" {
		req.Source = fmt.Sprintf("api:%v", teamName)
	}

	log.Printf("[data-import] team=%v type=%s size=%d", teamName, req.Type, len(req.Data))

	var result gin.H
	var err error
	switch req.Type {
	case "prediction":
		result, err = importPredictionJSON(req)
	case "kline":
		result, err = importKlineJSON(req)
	case "indicator":
		result, err = importIndicatorJSON(req)
	case "profile":
		result, err = importProfileJSON(req)
	case "signal":
		result, err = importSignalJSON(req)
	default:
		response.BadRequest(c, fmt.Sprintf("不支持的数据类型: %s，支持的类型: prediction, kline, indicator, profile, signal", req.Type))
		return
	}

	if err != nil {
		log.Printf("[data-import] error: %v", err)
		response.InternalError(c, fmt.Sprintf("导入失败: %v", err))
		return
	}

	// Record import log
	status := "success"
	importLog := model.ImportLog{
		FileName:     fmt.Sprintf("api:%s:%s", teamName, req.Type),
		RowsImported: 0,
		Status:       status,
		ImportedAt:   time.Now(),
	}
	if v, ok := result["imported"]; ok {
		if n, ok := v.(int); ok {
			importLog.RowsImported = n
		}
	}
	if db.MySQL != nil {
		db.MySQL.Create(&importLog)
	} else {
		db.PG.Create(&importLog)
	}

	result["source"] = req.Source
	result["team"] = teamName
	response.Success(c, result)
}

// ── Type-specific import handlers ──

// predictionEntry is the JSON structure for prediction data import.
type predictionEntry struct {
	StockCode string      `json:"stockCode"`
	Models    []predModel `json:"models"`
}

type predModel struct {
	ModelName string          `json:"modelName"`
	Prices    []predPriceDay  `json:"prices"`
}

type predPriceDay struct {
	Day   int     `json:"day"`
	Price float64 `json:"price"`
	Upper float64 `json:"upper"`
	Lower float64 `json:"lower"`
}

func importPredictionJSON(req dataImportRequest) (gin.H, error) {
	var entries []predictionEntry
	if err := json.Unmarshal(req.Data, &entries); err != nil {
		return nil, fmt.Errorf("预测数据 JSON 格式错误: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("预测数据为空")
	}

	now := time.Now()
	imported := 0
	batchSize := 500

	type rec struct {
		Code, Model, Date string
		Price, Upper, Lower float64
	}
	var batch []rec

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*6)
		for j, r := range batch {
			base := j * 6
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5, base+6))
			valueArgs = append(valueArgs, r.Code, r.Model, r.Date, r.Price, r.Upper, r.Lower)
		}
		query := fmt.Sprintf(`
			INSERT INTO predictions (code, model_name, predict_date, predicted_price, upper_bound, lower_bound)
			VALUES %s
			ON CONFLICT (code, model_name, predict_date) DO UPDATE SET
				predicted_price = EXCLUDED.predicted_price,
				upper_bound = EXCLUDED.upper_bound,
				lower_bound = EXCLUDED.lower_bound
		`, strings.Join(valueStrings, ","))
		return db.PG.Exec(query, valueArgs...).Error
	}

	for _, entry := range entries {
		code := strings.TrimSpace(entry.StockCode)
		if len(code) != 6 {
			continue
		}
		for _, m := range entry.Models {
			for _, p := range m.Prices {
				date := now.AddDate(0, 0, p.Day).Format("2006-01-02")
				batch = append(batch, rec{code, m.ModelName, date, p.Price, p.Upper, p.Lower})
				imported++
				if len(batch) >= batchSize {
					if err := flush(); err != nil {
						return nil, fmt.Errorf("批量写入预测数据失败: %w", err)
					}
					batch = batch[:0]
				}
			}
		}
	}

	if len(batch) > 0 {
		if err := flush(); err != nil {
			return nil, fmt.Errorf("批量写入预测数据失败: %w", err)
		}
	}

	return gin.H{"imported": imported, "stocks": len(entries)}, nil
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

// apiProfileEntry for stock profile import via API.
type apiProfileEntry struct {
	Code            string `json:"code"`
	ProfileMarkdown string `json:"profileMarkdown"`
}

func importProfileJSON(req dataImportRequest) (gin.H, error) {
	var entries []apiProfileEntry
	if err := json.Unmarshal(req.Data, &entries); err != nil {
		return nil, fmt.Errorf("研报数据 JSON 格式错误: %w", err)
	}

	imported := 0
	updated := 0

	for _, e := range entries {
		code := strings.TrimSpace(e.Code)
		if len(code) != 6 {
			continue
		}

		profile := model.StockProfile{
			Code:            code,
			ProfileMarkdown: e.ProfileMarkdown,
			Source:          req.Source,
			AnalyzedAt:      time.Now(),
		}

		var existing model.StockProfile
		if err := db.PG.Where("code = ?", code).First(&existing).Error; err == nil {
			db.PG.Model(&existing).Updates(map[string]interface{}{
				"profile_markdown": e.ProfileMarkdown,
				"source":           req.Source,
				"analyzed_at":      time.Now(),
				"updated_at":       time.Now(),
			})
			updated++
		} else {
			db.PG.Create(&profile)
			imported++
		}
	}

	return gin.H{"imported": imported, "updated": updated, "total": len(entries)}, nil
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
