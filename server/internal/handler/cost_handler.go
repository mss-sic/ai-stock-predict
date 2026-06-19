package handler

import (
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type CostHandler struct{}

func NewCostHandler() *CostHandler { return &CostHandler{} }

// GetCostLogs returns paginated AI cost logs with filters
func (h *CostHandler) GetCostLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	userID := c.Query("userId")
	module := c.Query("module")
	start := c.Query("start")
	end := c.Query("end")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := db.MySQL.Model(&model.AICostLog{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if module != "" {
		query = query.Where("module = ?", module)
	}
	if start != "" {
		query = query.Where("created_at >= ?", start)
	}
	if end != "" {
		query = query.Where("created_at <= ?", end+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var logs []model.AICostLog
	query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	response.Success(c, gin.H{
		"list":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetCostSummary returns aggregated cost statistics
func (h *CostHandler) GetCostSummary(c *gin.Context) {
	start := c.DefaultQuery("start", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	end := c.DefaultQuery("end", time.Now().Format("2006-01-02"))

	type Summary struct {
		TotalCost  float64 `json:"totalCost"`
		TotalCalls int64   `json:"totalCalls"`
		TotalTokens int64  `json:"totalTokens"`
		TodayCost  float64 `json:"todayCost"`
		MonthCost  float64 `json:"monthCost"`
	}

	var s Summary

	db.MySQL.Model(&model.AICostLog{}).
		Where("success = 1 AND created_at >= ? AND created_at <= ?", start, end+" 23:59:59").
		Select("COALESCE(SUM(cost_amount),0) as total_cost, COUNT(*) as total_calls, COALESCE(SUM(total_tokens),0) as total_tokens").
		Scan(&s)

	today := time.Now().Format("2006-01-02")
	db.MySQL.Model(&model.AICostLog{}).
		Where("success = 1 AND DATE(created_at) = ?", today).
		Select("COALESCE(SUM(cost_amount),0)").Scan(&s.TodayCost)

	monthStart := time.Now().Format("2006-01") + "-01"
	db.MySQL.Model(&model.AICostLog{}).
		Where("success = 1 AND created_at >= ?", monthStart).
		Select("COALESCE(SUM(cost_amount),0)").Scan(&s.MonthCost)

	response.Success(c, s)
}

// GetModelPrices returns all model price configs
func (h *CostHandler) GetModelPrices(c *gin.Context) {
	var prices []model.ModelPrice
	db.MySQL.Order("id ASC").Find(&prices)
	response.Success(c, prices)
}

// UpdateModelPrice updates a single model's pricing
func (h *CostHandler) UpdateModelPrice(c *gin.Context) {
	modelName := c.Param("model_name")
	var body struct {
		InputPrice    float64 `json:"inputPrice"`
		OutputPrice   float64 `json:"outputPrice"`
		CacheHitPrice float64 `json:"cacheHitPrice"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := db.MySQL.Model(&model.ModelPrice{}).Where("model_name = ?", modelName).
		Updates(map[string]interface{}{
			"input_price":     body.InputPrice,
			"output_price":    body.OutputPrice,
			"cache_hit_price": body.CacheHitPrice,
		}).Error; err != nil {
		response.Error(c, 500, 0, "更新失败")
		return
	}
	response.SuccessMsg(c, "价格已更新")
}

// ── User-scoped cost APIs ──

// GetUserCostLogs returns paginated AI cost logs for the current user
func (h *CostHandler) GetUserCostLogs(c *gin.Context) {
	uid := getUID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	module := c.Query("module")
	start := c.Query("start")
	end := c.Query("end")

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }

	if db.MySQL == nil {
		response.Success(c, gin.H{"list": []model.AICostLog{}, "total": 0, "page": page, "pageSize": pageSize})
		return
	}

	query := db.MySQL.Model(&model.AICostLog{}).Where("user_id = ?", uid)
	if module != "" {
		query = query.Where("module = ?", module)
	}
	if start != "" {
		query = query.Where("created_at >= ?", start)
	}
	if end != "" {
		query = query.Where("created_at <= ?", end+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var logs []model.AICostLog
	query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
	if logs == nil { logs = []model.AICostLog{} }

	response.Success(c, gin.H{
		"list": logs, "total": total, "page": page, "pageSize": pageSize,
	})
}

// GetUserCostSummary returns aggregated cost statistics for the current user
func (h *CostHandler) GetUserCostSummary(c *gin.Context) {
	uid := getUID(c)

	s := struct {
		TotalCost   float64 `json:"totalCost"`
		TotalCalls  int64   `json:"totalCalls"`
		TotalTokens int64   `json:"totalTokens"`
		TodayCost   float64 `json:"todayCost"`
		TodayTokens int64   `json:"todayTokens"`
		ModelCalls  int64   `json:"modelCalls"`
	}{}

	if db.MySQL == nil {
		response.Success(c, s)
		return
	}

	row := db.MySQL.Model(&model.AICostLog{}).
		Where("user_id = ? AND success = 1", uid).
		Select("COALESCE(SUM(cost_amount),0), COUNT(*), COALESCE(SUM(total_tokens),0)").
		Row()
	if row != nil {
		row.Scan(&s.TotalCost, &s.TotalCalls, &s.TotalTokens)
	}

	today := time.Now().Format("2006-01-02")
	row = db.MySQL.Model(&model.AICostLog{}).
		Where("user_id = ? AND success = 1 AND DATE(created_at) = ?", uid, today).
		Select("COALESCE(SUM(cost_amount),0), COALESCE(SUM(total_tokens),0)").
		Row()
	if row != nil {
		row.Scan(&s.TodayCost, &s.TodayTokens)
	}

	db.MySQL.Model(&model.AICostLog{}).
		Where("user_id = ? AND success = 1", uid).
		Select("COUNT(DISTINCT model_name)").Row().Scan(&s.ModelCalls)

	response.Success(c, s)
}

// GetUserCostDaily returns daily cost breakdown grouped by module for the current user
func (h *CostHandler) GetUserCostDaily(c *gin.Context) {
	uid := getUID(c)
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	type DailyItem struct {
		Date   string  `json:"date"`
		Module string  `json:"module"`
		Cost   float64 `json:"cost"`
		Tokens int64   `json:"tokens"`
	}

	if db.MySQL == nil {
		response.Success(c, []DailyItem{})
		return
	}

	var items []DailyItem
	db.MySQL.Model(&model.AICostLog{}).
		Where("user_id = ? AND success = 1 AND DATE_FORMAT(created_at, '%Y-%m') = ?", uid, month).
		Select("DATE_FORMAT(created_at, '%Y-%m-%d') as date, module, COALESCE(SUM(cost_amount),0) as cost, COALESCE(SUM(total_tokens),0) as tokens").
		Group("date, module").
		Order("date ASC, module ASC").
		Scan(&items)

	if items == nil { items = []DailyItem{} }
	response.Success(c, items)
}
