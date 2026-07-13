package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
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
// Returns immediately; actual repair runs in background goroutine.
func (h *StockHandler) RepairKLine(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "缺少股票代码")
		return
	}
	log.Printf("[repair] triggered for %s", code)

	// Run repair in background to avoid blocking the HTTP request
	// and exhausting PostgreSQL shared memory under concurrent calls.
	go func() {
		log.Printf("[repair] starting for %s", code)
		if err := collector.RepairStock(code); err != nil {
			log.Printf("[repair] failed for %s: %v", code, err)
		} else {
			log.Printf("[repair] completed for %s", code)
		}
	}()

	response.Success(c, gin.H{"message": "数据修复已触发，请稍后刷新查看", "stockCode": code})
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

func (h *StockHandler) GetFundFlowMinute(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Error(c, 400, response.CodeBadRequest, "股票代码不能为空")
		return
	}
	market := "0"
	if strings.HasPrefix(code, "6") {
		market = "1"
	}
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/fflow/kline/get?secid=%s.%s&klt=1&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57", market, code)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		response.Success(c, []interface{}{})
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	req.Header.Set("Origin", "https://quote.eastmoney.com")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Minute API often unavailable; return empty gracefully
		response.Success(c, []interface{}{})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		response.Success(c, []interface{}{})
		return
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		response.Success(c, []interface{}{})
		return
	}
	data, _ := result["data"].(map[string]interface{})
	klines, _ := data["klines"].([]interface{})
	type MinuteFlow struct {
		Time     string  `json:"time"`
		MainNet  float64 `json:"mainNet"`
		SmallNet float64 `json:"smallNet"`
		MidNet   float64 `json:"midNet"`
		LargeNet float64 `json:"largeNet"`
		SuperNet float64 `json:"superNet"`
	}
	var rows []MinuteFlow
	for _, line := range klines {
		parts := strings.Split(line.(string), ",")
		if len(parts) >= 6 {
			mainNet, _ := strconv.ParseFloat(parts[1], 64)
			smallNet, _ := strconv.ParseFloat(parts[2], 64)
			midNet, _ := strconv.ParseFloat(parts[3], 64)
			largeNet, _ := strconv.ParseFloat(parts[4], 64)
			superNet, _ := strconv.ParseFloat(parts[5], 64)
			rows = append(rows, MinuteFlow{
				Time: parts[0],
				MainNet: mainNet / 10000,
				SmallNet: smallNet / 10000,
				MidNet: midNet / 10000,
				LargeNet: largeNet / 10000,
				SuperNet: superNet / 10000,
			})
		}
	}
	response.Success(c, rows)
}

func (h *StockHandler) GetStockFundFlow(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Error(c, 400, response.CodeBadRequest, "股票代码不能为空")
		return
	}
	// 并行查两个数据源
	fundFlow, _ := h.svc.GetStockFundFlow(code)
	buySellFlow, _ := h.svc.GetBuySellFlow(code, "30")

	if fundFlow == nil { fundFlow = []model.StockFundFlow{} }
	if buySellFlow == nil { buySellFlow = []model.BuySellFlowItem{} }

	response.Success(c, gin.H{
		"fundFlow":    fundFlow,
		"buySellFlow": buySellFlow,
		"hasFundFlow": len(fundFlow) > 0,
	})
}

func (h *StockHandler) GetAllAnnouncements(c *gin.Context) {
	limit := 200
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "200")); err == nil && l > 0 {
		limit = l
	}
	data, err := h.svc.GetAllAnnouncements(limit)
	if err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询公告失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetThsEpsForecast(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Error(c, 400, response.CodeBadRequest, "股票代码不能为空")
		return
	}
	data, err := h.svc.GetThsEpsForecast(code)
	if err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询一致预期失败: "+err.Error())
		return
	}
	
	response.Success(c, data)
}

func (h *StockHandler) GetMacroNews(c *gin.Context) {
	category := c.DefaultQuery("category", "")
	limit := 50
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && l > 0 {
		limit = l
	}
	data, err := h.svc.GetMacroNews(category, limit)
	if err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询宏观资讯失败: "+err.Error())
		return
	}
	
	response.Success(c, data)
}

func (h *StockHandler) GetMacroCategories(c *gin.Context) {
	data, err := h.svc.GetMacroCategories()
	if err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询资讯分类失败: "+err.Error())
		return
	}
	if data == nil { data = []string{} }
	response.Success(c, data)
}

func (h *StockHandler) GetThsHotConceptStats(c *gin.Context) {
	days := 7
	if d, err := strconv.Atoi(c.DefaultQuery("days", "7")); err == nil && d > 0 {
		days = d
	}
	data, err := h.svc.GetThsHotConceptStats(days)
	if err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询题材统计失败: "+err.Error())
		return
	}
	if data == nil {
		data = []map[string]interface{}{}
	}
	response.Success(c, data)
}

func (h *StockHandler) GetAllFutureUnlocks(c *gin.Context) {
	days := 90
	if d, err := strconv.Atoi(c.DefaultQuery("days", "90")); err == nil && d > 0 {
		days = d
	}
	data, err := h.svc.GetAllFutureUnlocks(days)
	if err != nil {
		response.Error(c, 500, response.CodeInternalError, "查询解禁数据失败: "+err.Error())
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

func (h *StockHandler) GetDailyDragonTigerList(c *gin.Context) {
	tradeDate := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	data, err := h.svc.GetDailyDragonTigerList(tradeDate)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}
func (h *StockHandler) GetDailyDragonTigerEnriched(c *gin.Context) {
	tradeDate := c.DefaultQuery("date", "")
	if tradeDate == "" {
		// Auto-detect latest available trading date from database
		db.PG.Raw("SELECT trade_date::text FROM dragon_tiger_list ORDER BY trade_date DESC LIMIT 1").Scan(&tradeDate)
		if tradeDate == "" {
			tradeDate = time.Now().Format("2006-01-02")
		}
	}
	data, err := h.svc.GetDailyDragonTigerEnriched(tradeDate)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}


func (h *StockHandler) GetDragonTigerSeats(c *gin.Context) {
	code := c.Param("code")
	tradeDate := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	data, err := h.svc.GetDragonTigerSeats(code, tradeDate)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}
