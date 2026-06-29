package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/repository"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type StockHandler struct {
	svc *service.StockService
}

func NewStockHandler() *StockHandler { return &StockHandler{svc: service.NewStockService()} }

func (h *StockHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	industry := c.Query("industry")
	keyword := c.Query("keyword")
	boardType := c.Query("boardType")
	sortBy := c.Query("sortBy")
	sortDir := c.DefaultQuery("sortDir", "desc")

	stocks, total, err := h.svc.List(industry, keyword, boardType, sortBy, sortDir, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stocks, "total": total, "page": page, "pageSize": pageSize})
}

// MarketSnapshot returns aggregate market overview for the latest trading day.
func (h *StockHandler) MarketSnapshot(c *gin.Context) {
	snap, err := h.svc.GetMarketSnapshot()
	if err != nil {
		response.InternalError(c, "获取市场快照失败: "+err.Error())
		return
	}
	response.Success(c, snap)
}

// Ranking returns top stocks sorted by the given field.
func (h *StockHandler) Ranking(c *gin.Context) {
	boardType := c.Query("boardType")
	sortBy := c.DefaultQuery("sortBy", "chgPct")
	ascStr := c.DefaultQuery("asc", "false")
	limitStr := c.DefaultQuery("limit", "50")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 200 {
		limit = 50
	}
	asc := ascStr == "true"

	rows, err := h.svc.GetRanking(boardType, sortBy, limit, asc)
	if err != nil {
		response.InternalError(c, "获取排行失败: "+err.Error())
		return
	}
	if rows == nil {
		rows = []repository.StockListRow{}
	}
	response.Success(c, rows)
}

// Unusual returns stocks with unusual activity.
func (h *StockHandler) Unusual(c *gin.Context) {
	boardType := c.Query("boardType")
	limitStr := c.DefaultQuery("limit", "20")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	rows, err := h.svc.GetUnusual(boardType, limit)
	if err != nil {
		response.InternalError(c, "获取异动数据失败: "+err.Error())
		return
	}
	if rows == nil {
		rows = []repository.UnusualRow{}
	}
	response.Success(c, rows)
}

// BoardTypeCounts returns stock counts per board type.
func (h *StockHandler) BoardTypeCounts(c *gin.Context) {
	counts, err := h.svc.GetBoardTypeCounts()
	if err != nil {
		response.InternalError(c, "获取板块统计失败: "+err.Error())
		return
	}
	response.Success(c, counts)
}

func (h *StockHandler) GetDetail(c *gin.Context) {
	code := c.Param("code")
	stock, err := h.svc.GetDetail(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stock not found"})
		return
	}
	response.Success(c, stock)
}

func (h *StockHandler) GetKLine(c *gin.Context) {
	code := c.Param("code")
	from := c.Query("from")
	to := c.Query("to")
	klines, err := h.svc.GetKLine(code, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.Success(c, klines)
}

func (h *StockHandler) GetIndicator(c *gin.Context) {
	code := c.Param("code")
	ind, err := h.svc.GetIndicator(code)
	if err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, ind)
}

func (h *StockHandler) GetSignal(c *gin.Context) {
	code := c.Param("code")
	signal, err := h.svc.GetSignal(code)
	if err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, signal)
}

func (h *StockHandler) GetFinancials(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetFinancials(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetShareholders(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetShareholders(code)
	if err != nil {
		log.Printf("[Shareholders] query error for %s: %v", code, err)
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	log.Printf("[Shareholders] got %d rows for %s", len(data), code)
	response.Success(c, data)
}

func (h *StockHandler) GetNews(c *gin.Context) {
	code := c.Param("code")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := h.svc.GetNews(code, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetReports(c *gin.Context) {
	code := c.Param("code")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := h.svc.GetReports(code, limit)
	if err != nil {
		log.Printf("[Reports] query error for %s: %v", code, err)
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetIndustryReports(c *gin.Context) {
	industry := c.Query("industry")
	if industry == "" {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := h.svc.GetIndustryReports(industry, limit)
	if err != nil {
		log.Printf("[IndustryReports] query error for %s: %v", industry, err)
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func GetDataStats(c *gin.Context) {
	stats := repository.GetDataStats()
	response.Success(c, stats)
}

func GetDataDetail(c *gin.Context) {
	typ := c.Param("type")
	results := repository.GetDataDetail(typ)
	response.Success(c, results)
}


// AppearanceStats returns stocks ranked by how often they appeared in top-N daily gainers.
func (h *StockHandler) AppearanceStats(c *gin.Context) {
	topNStr := c.DefaultQuery("topN", "50")
	limitStr := c.DefaultQuery("limit", "100")

	topN, err := strconv.Atoi(topNStr)
	if err != nil || topN < 10 || topN > 200 {
		topN = 50
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 200 {
		limit = 100
	}

	rows, err := h.svc.GetAppearanceStats(topN, limit)
	if err != nil {
		response.InternalError(c, "获取上榜统计失败: "+err.Error())
		return
	}
	if rows == nil {
		rows = []repository.AppearanceRow{}
	}
	response.Success(c, rows)
}
// RepairKLine triggers full data repair for a stock (async).
func (h *StockHandler) RepairKLine(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "缺少股票代码")
		return
	}
	// Run repair in background (can take ~1-5 seconds)
	go func() {
		log.Printf("[repair] starting for %s", code)
		if err := collector.RepairStock(code); err != nil {
			log.Printf("[repair] failed for %s: %v", code, err)
		} else {
			log.Printf("[repair] completed for %s", code)
		}
	}()
	response.Success(c, gin.H{"message": "数据修复已触发", "stockCode": code})
}

func (h *StockHandler) GetDragonTigerList(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetDragonTigerList(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetBlockTrades(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetBlockTrades(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetCninfoAnnouncements(c *gin.Context) {
	code := c.Param("code")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := h.svc.GetCninfoAnnouncements(code, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetRestrictedUnlocks(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetRestrictedUnlocks(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}
