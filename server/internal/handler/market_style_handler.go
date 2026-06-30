package handler

import (
	"log"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// MarketStyleHandler handles market style and review endpoints
type MarketStyleHandler struct {
	svc *service.MarketStyleService
}

// NewMarketStyleHandler creates a new handler
func NewMarketStyleHandler() *MarketStyleHandler {
	return &MarketStyleHandler{svc: service.NewMarketStyleService()}
}

// GetStyleCurve returns market style timeline for a date range
func (h *MarketStyleHandler) GetStyleCurve(c *gin.Context) {
	from := c.DefaultQuery("from", "")
	to := c.DefaultQuery("to", "")
	if from == "" || to == "" {
		response.BadRequest(c, "from 和 to 参数必填")
		return
	}
	rows, err := h.svc.GetStyleCurve(from, to)
	if err != nil {
		response.InternalError(c, "获取风格曲线失败: "+err.Error())
		return
	}
	if rows == nil {
		rows = []service.StyleRow{}
	}
	response.Success(c, rows)
}

// GetDailyReview returns full daily review data
func (h *MarketStyleHandler) GetDailyReview(c *gin.Context) {
	date := c.DefaultQuery("date", "")
	if date == "" {
		response.BadRequest(c, "date 参数必填")
		return
	}
	review, err := h.svc.GetDailyReview(date)
	if err != nil {
		response.InternalError(c, "获取复盘数据失败: "+err.Error())
		return
	}
	response.Success(c, review)
}

// GetLatestStyle returns the most recent market style
func (h *MarketStyleHandler) GetLatestStyle(c *gin.Context) {
	row, err := h.svc.GetLatestStyle()
	if err != nil {
		response.InternalError(c, "获取最新风格失败: "+err.Error())
		return
	}
	response.Success(c, row)
}

// ComputeStyle triggers style detection + storage for a date
func (h *MarketStyleHandler) ComputeStyle(c *gin.Context) {
	date := c.DefaultQuery("date", "")
	if date == "" {
		response.BadRequest(c, "date 参数必填")
		return
	}
	if err := h.svc.ComputeAndStore(date); err != nil {
		response.InternalError(c, "计算风格失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"message": "风格计算完成", "date": date})
}

// BulkCompute computes market style for all dates in market_sentiment that are missing
func (h *MarketStyleHandler) BulkCompute(c *gin.Context) {
	// Clean zero-filled rows first (caused by missing source data at time of initial insert)
	db.PG.Exec(`DELETE FROM market_style_daily WHERE up_ratio = 0 AND total_amount = 0`)

	var dates []string
	if err := db.PG.Raw(`
		SELECT trade_date::text FROM market_sentiment ms
		WHERE ms.trade_date NOT IN (SELECT trade_date FROM market_style_daily)
		   OR ms.trade_date IN (SELECT trade_date FROM market_style_daily WHERE style_confidence = 0)
		ORDER BY trade_date
	`).Pluck("trade_date", &dates).Error; err != nil {
		response.InternalError(c, "查询日期失败: "+err.Error())
		return
	}

	total := len(dates)
	success := 0
	fail := 0
	for i, date := range dates {
		if err := h.svc.ComputeAndStore(date); err != nil {
			fail++
			log.Printf("[market_style] bulk compute FAIL %s: %v", date, err)
		} else {
			success++
		}
		if (i+1)%20 == 0 || i == total-1 {
			log.Printf("[market_style] bulk progress: %d/%d (ok=%d fail=%d)", i+1, total, success, fail)
		}
	}
	response.Success(c, gin.H{
		"message": "批量计算完成",
		"total":   total,
		"success": success,
		"fail":    fail,
	})
}
