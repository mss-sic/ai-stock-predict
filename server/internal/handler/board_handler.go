package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/ai-stock-predict/server/pkg/response"
)

type BoardHandler struct {
	repo *repository.BoardRepo
}

func NewBoardHandler() *BoardHandler {
	return &BoardHandler{repo: &repository.BoardRepo{}}
}

// Internal row types for parallel queries
type KLineRow struct {
	Code     string
	Close    float64
	Open     float64
	PreClose float64
}

type NextKLineRow struct {
	Code  string
	Open  float64
	Close float64
}

type LatestCloseRow struct {
	Code  string
	Close float64
}

type IndicatorRow struct {
	Code string
	PE   float64
	PB   float64
	MCap float64
}

type AIScoreRow struct {
	Code       string
	RiskLevel  string
	Suggestion string
}

type EnrichedBoardItem struct {
	ID          uint    `json:"id"`
	PickDate    string  `json:"pickDate"`
	StockCode   string  `json:"stockCode"`
	StockName   string  `json:"stockName"`
	Rank        int     `json:"rank"`
	Score       float64 `json:"score"`
	RiskLevel   string  `json:"riskLevel"`
	Suggestion  string  `json:"suggestion"`
	SignalTags  string  `json:"signalTags"`
	Open        float64 `json:"open"`
	Close       float64 `json:"close"`
	PreClose    float64 `json:"preClose"`
	ChgPct      float64 `json:"chgPct"`
	PE          float64 `json:"pe"`
	PB          float64 `json:"pb"`
	Industry    string  `json:"industry"`
	MarketCap   float64 `json:"marketCap"`
	// Next-day performance
	NextDate   string  `json:"nextDate"`
	NextOpen   float64 `json:"nextOpen"`
	NextClose  float64 `json:"nextClose"`
	NextChgPct float64 `json:"nextChgPct"`
	// Cumulative return from pick-date close to latest close
	CumuChgPct float64 `json:"cumuChgPct"`
	// Board streak stats (last 20 trading days)
	StreakCount     int `json:"streakCount"`
	AppearanceCount int `json:"appearanceCount"`
}

func (h *BoardHandler) Today(c *gin.Context) {
	data, dateStr, err := h.getEnrichedBoard("")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []EnrichedBoardItem{}, "date": "", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "date": dateStr})
}

func (h *BoardHandler) History(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date required"})
		return
	}
	data, _, err := h.getEnrichedBoard(dateStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []EnrichedBoardItem{}, "date": dateStr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "date": dateStr})
}

func (h *BoardHandler) getEnrichedBoard(dateStr string) ([]EnrichedBoardItem, string, error) {
	if dateStr == "" {
		var latest model.AlgorithmPickDetail
		if err := db.PG.Order("pick_date DESC").First(&latest).Error; err != nil {
			return nil, "", err
		}
		dateStr = latest.PickDate.Format("2006-01-02")
	}

	var picks []model.AlgorithmPickDetail
	if err := db.PG.Where("pick_date = ?", dateStr).Order("rank ASC").Find(&picks).Error; err != nil {
		return nil, dateStr, err
	}

	if len(picks) == 0 {
		return []EnrichedBoardItem{}, dateStr, nil
	}

	codes := make([]string, len(picks))
	for i, p := range picks {
		codes[i] = p.StockCode
	}

	// ── Run 6 independent lookups in parallel ──
	var (
		stockMap      map[string]model.StockBasic
		klineMap      map[string]KLineRow
		indMap        map[string]IndicatorRow
		aiScoreMap    map[string]AIScoreRow
		latestCloseMap map[string]float64
		nextKlineMap  map[string]NextKLineRow
		nextDateMap   map[string]string
		streakMap     map[string][2]int // [streak, appearance]
		wg            sync.WaitGroup
	)

	wg.Add(7)

	go func() {
		defer wg.Done()
		var stocks []model.StockBasic
		db.PG.Where("code IN ?", codes).Find(&stocks)
		m := make(map[string]model.StockBasic, len(stocks))
		for _, s := range stocks { m[s.Code] = s }
		stockMap = m
	}()

	inClause := db.CodesToInClause(codes)
	go func() {
		defer wg.Done()
		var klines []KLineRow
		if err := db.PG.Raw(fmt.Sprintf(`SELECT k.code, k.close, k.open,
			COALESCE(LAG(k.close) OVER (PARTITION BY k.code ORDER BY k.trade_date), k.open) AS pre_close
			FROM stocks_daily_k k WHERE k.code IN (%s) AND k.trade_date = ?`, inClause), dateStr).Scan(&klines).Error; err != nil {
			log.Printf("[board] klines query failed: %v", err)
			return
		}
		m := make(map[string]KLineRow, len(klines))
		for _, k := range klines { m[k.Code] = k }
		klineMap = m
	}()

	go func() {
		defer wg.Done()
		var indicators []IndicatorRow
		if err := db.PG.Raw(fmt.Sprintf(`SELECT i.code, i.pe, i.pb, i.total_market_cap
			FROM stocks_daily_indicator i
			INNER JOIN (SELECT code, MAX(trade_date) as max_date FROM stocks_daily_indicator WHERE code IN (%s) GROUP BY code) latest
			ON i.code = latest.code AND i.trade_date = latest.max_date`, inClause)).Scan(&indicators).Error; err != nil {
			log.Printf("[board] indicators query failed: %v", err)
			return
		}
		m := make(map[string]IndicatorRow, len(indicators))
		for _, ind := range indicators { m[ind.Code] = ind }
		indMap = m
	}()

	go func() {
		defer wg.Done()
		var aiScores []AIScoreRow
		if err := db.PG.Raw(fmt.Sprintf(`SELECT s.code, s.risk_level, s.suggestion
			FROM ai_stock_scores s
			INNER JOIN (SELECT code, MAX(analyzed_at) AS max_at FROM ai_stock_scores WHERE code IN (%s) GROUP BY code) latest
			ON s.code = latest.code AND s.analyzed_at = latest.max_at`, inClause)).Scan(&aiScores).Error; err != nil {
			log.Printf("[board] ai_scores query failed: %v", err)
			return
		}
		m := make(map[string]AIScoreRow, len(aiScores))
		for _, a := range aiScores { m[a.Code] = a }
		aiScoreMap = m
	}()

	go func() {
		defer wg.Done()
		var latestCloses []LatestCloseRow
		if err := db.PG.Raw(fmt.Sprintf(`SELECT l.code, l.close FROM stocks_daily_k l
			INNER JOIN (SELECT code, MAX(trade_date) AS max_date FROM stocks_daily_k WHERE code IN (%s) GROUP BY code) latest
			ON l.code = latest.code AND l.trade_date = latest.max_date`, inClause)).Scan(&latestCloses).Error; err != nil {
			log.Printf("[board] latest_closes query failed: %v", err)
			return
		}
		m := make(map[string]float64, len(latestCloses))
		for _, lc := range latestCloses { m[lc.Code] = lc.Close }
		latestCloseMap = m
	}()

	go func() {
		defer wg.Done()
		// Combined next-day lookup: single query using ROW_NUMBER window function
		type NextDayRow struct {
			Code     string
			NextDate string
			Open     float64
			Close    float64
		}
		var nextRows []NextDayRow
		if err := db.PG.Raw(fmt.Sprintf(`SELECT code, TO_CHAR(trade_date, 'YYYY-MM-DD') AS next_date, open, close FROM (
			SELECT code, trade_date, open, close,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date) AS rn
			FROM stocks_daily_k
			WHERE code IN (%s) AND trade_date > ?
		) sub WHERE rn = 1`, inClause), dateStr).Scan(&nextRows).Error; err != nil {
			log.Printf("[board] next-day kline query failed: %v", err)
		}
		ndm := make(map[string]string, len(nextRows))
		nkm := make(map[string]NextKLineRow, len(nextRows))
		for _, n := range nextRows {
			ndm[n.Code] = n.NextDate
			nkm[n.Code] = NextKLineRow{Code: n.Code, Open: n.Open, Close: n.Close}
		}
		nextDateMap = ndm
		nextKlineMap = nkm
	}()

	go func() {
		defer wg.Done()
		// Compute streak count and appearance count (last 20 trading days)
		type BoardDateRow struct {
			StockCode string
			PickDate  string
		}
		var rows []BoardDateRow
		if err := db.PG.Raw(fmt.Sprintf(`SELECT stock_code, TO_CHAR(pick_date, 'YYYY-MM-DD HH24:MI:SS')
			FROM algorithm_pick_details
			WHERE stock_code IN (%s) AND pick_date <= ? AND pick_date > (?::date - INTERVAL '30 days')
			ORDER BY stock_code, pick_date DESC`, inClause), dateStr, dateStr).Scan(&rows).Error; err != nil {
			log.Printf("[board] pick_date query failed: %v", err)
		}

		m := make(map[string][2]int, len(codes))
		// Group by code
		codeDates := make(map[string][]string)
		for _, r := range rows {
			if len(r.PickDate) >= 10 {
				codeDates[r.StockCode] = append(codeDates[r.StockCode], r.PickDate[:10])
			}
		}
		for code, dates := range codeDates {
			appearance := len(dates)
			streak := 0
			// Compute consecutive streak backwards from dateStr
			// dates are sorted DESC, so check consecutive trading days
			if len(dates) > 0 && dates[0] == dateStr {
				streak = 1
				prev := dateStr
				for i := 1; i < len(dates); i++ {
					// Check if dates[i] is the previous trading day of prev
					if isPrevTradingDay(dates[i], prev) {
						streak++
						prev = dates[i]
					} else {
						break
					}
				}
			}
			m[code] = [2]int{streak, appearance}
		}
		streakMap = m
	}()

	wg.Wait()

	result := make([]EnrichedBoardItem, 0, len(picks))
	for _, p := range picks {
		item := EnrichedBoardItem{
			ID:         p.ID,
			PickDate:   dateStr,
			StockCode:  p.StockCode,
			Rank:       p.Rank,
			Score:      p.Score,
			RiskLevel:  p.RiskLevel,
			Suggestion: p.Suggestion,
		}

		if s, ok := stockMap[p.StockCode]; ok {
			item.StockName = s.Name
			item.Industry = s.Industry
		}
		if a, ok := aiScoreMap[p.StockCode]; ok {
			item.RiskLevel = a.RiskLevel
			item.Suggestion = a.Suggestion
		} else {
			item.RiskLevel = ""
			item.Suggestion = ""
		}
		if k, ok := klineMap[p.StockCode]; ok {
			item.Open = k.Open
			item.Close = k.Close
			item.PreClose = k.PreClose
			if k.PreClose > 0 {
				item.ChgPct = ((k.Close - k.PreClose) / k.PreClose) * 100
			}
		}
		if ind, ok := indMap[p.StockCode]; ok {
			item.PE = ind.PE
			item.PB = ind.PB
			item.MarketCap = ind.MCap
		}
		// Next-day performance
		if n, ok := nextKlineMap[p.StockCode]; ok {
			item.NextDate = nextDateMap[p.StockCode]
			item.NextOpen = n.Open
			item.NextClose = n.Close
		}
		// Fix: compute NextChgPct from pick-day close, not next-day pre_close
		if item.NextClose > 0 && item.Close > 0 {
			item.NextChgPct = ((item.NextClose - item.Close) / item.Close) * 100
		}
		// Cumulative return: (latest_close - pick_close) / pick_close * 100
		if lc, ok := latestCloseMap[p.StockCode]; ok && lc > 0 && item.Close > 0 {
			item.CumuChgPct = ((lc - item.Close) / item.Close) * 100
		}
		// Streak & appearance count
		if sa, ok := streakMap[p.StockCode]; ok {
			item.StreakCount = sa[0]
			item.AppearanceCount = sa[1]
		}

		result = append(result, item)
	}

	return result, dateStr, nil
}

// isPrevTradingDay checks if d1 is the immediate previous trading day of d2
func isPrevTradingDay(d1, d2 string) bool {
	t1, err1 := time.Parse("2006-01-02", d1)
	t2, err2 := time.Parse("2006-01-02", d2)
	if err1 != nil || err2 != nil {
		return false
	}
	// Walk backwards from d2 looking for the previous trading day
	prev := t2.AddDate(0, 0, -1)
	for prev.Weekday() == time.Saturday || prev.Weekday() == time.Sunday {
		prev = prev.AddDate(0, 0, -1)
	}
	return t1.Equal(prev)
}

func (h *BoardHandler) Heatmap(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	now := time.Now()
	if from == "" {
		from = now.AddDate(0, 0, -20).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	data, err := h.repo.GetHeatmapData(from, to)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []model.AlgorithmPickDetail{}})
		return
	}
	response.Success(c, data)
}

// ── Concept Board APIs ──

// StockConcepts returns concept tags for a stock
func (h *BoardHandler) StockConcepts(c *gin.Context) {
	code := c.Param("code")
	var concepts []model.StockConcept
	db.PG.Where("code = ?", code).Order("concept_name ASC").Find(&concepts)
	
	if len(concepts) == 0 {
		// Fallback: get from stocks_basic industry
		var stock model.StockBasic
		if db.PG.Where("code = ?", code).First(&stock).Error == nil && stock.Industry != "" {
			concepts = append(concepts, model.StockConcept{
				Code:        stock.Code,
				ConceptCode: "IND_" + stock.Industry,
				ConceptName: stock.Industry,
				ConceptType: "industry",
				StockName:   stock.Name,
			})
		}
	}
	
	response.Success(c, concepts)
}

// ConceptBoards returns all concept boards
func (h *BoardHandler) ConceptBoards(c *gin.Context) {
	conceptType := c.DefaultQuery("type", "")
	var boards []model.ConceptBoard
	q := db.PG.Order("stock_count DESC")
	if conceptType != "" {
		q = q.Where("concept_type = ?", conceptType)
	}
	q.Find(&boards)
	response.Success(c, boards)
}

// ConceptBoardStocks returns stocks in a concept board with summary
func (h *BoardHandler) ConceptBoardStocks(c *gin.Context) {
	conceptCode := c.Param("code")
	
	type StockSummary struct {
		model.StockConcept
		Close    float64 `json:"close"`
		ChgPct   float64 `json:"chgPct"`
		MarketCap float64 `json:"marketCap"`
	}
	
	var stocks []StockSummary
	db.PG.Raw(`
		SELECT sc.code, sc.concept_code, sc.concept_name, sc.concept_type, sc.stock_name,
			COALESCE(k.close, 0) as close,
			CASE WHEN k.pre_close > 0 THEN ((k.close - k.pre_close) / k.pre_close * 100) ELSE 0 END as chg_pct,
			COALESCE(i.total_market_cap, 0) as market_cap
		FROM stock_concepts sc
		LEFT JOIN LATERAL (
			SELECT close, LAG(close) OVER (ORDER BY trade_date) as pre_close
			FROM stocks_daily_k WHERE code = sc.code ORDER BY trade_date DESC LIMIT 1
		) k ON true
		LEFT JOIN LATERAL (
			SELECT total_market_cap FROM stocks_daily_indicator 
			WHERE code = sc.code ORDER BY trade_date DESC LIMIT 1
		) i ON true
		WHERE sc.concept_code = ?
		ORDER BY sc.code
	`, conceptCode).Scan(&stocks)
	
	// Board info
	var board model.ConceptBoard
	db.PG.Where("concept_code = ?", conceptCode).First(&board)
	
	// Summary stats
	var upCount, downCount int
	var avgChg float64
	for _, s := range stocks {
		if s.ChgPct > 0 {
			upCount++
		} else if s.ChgPct < 0 {
			downCount++
		}
		avgChg += s.ChgPct
	}
	if len(stocks) > 0 {
		avgChg = avgChg / float64(len(stocks))
	}
	
	response.Success(c, gin.H{
		"board":     board,
		"stocks":    stocks,
		"upCount":   upCount,
		"downCount": downCount,
		"avgChgPct": avgChg,
		"total":     len(stocks),
	})
}

// ConceptHeatmap returns heatmap data for concept boards
func (h *BoardHandler) ConceptHeatmap(c *gin.Context) {
	var items []model.ConceptHeatmapItem
	
	rows, err := db.PG.Raw(`
		WITH latest_prices AS (
			SELECT code, close,
				LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k
			WHERE trade_date >= (SELECT MAX(trade_date) FROM stocks_daily_k) - INTERVAL '3 days'
		),
		stock_changes AS (
			SELECT code, close, prev_close
			FROM latest_prices
			WHERE rn = 1
		)
		SELECT cb.concept_code, cb.concept_name, cb.concept_type, cb.stock_count,
			COALESCE(AVG(CASE WHEN sc2.prev_close > 0 THEN (sc2.close - sc2.prev_close) / sc2.prev_close * 100 END), 0) as avg_chg_pct,
			COUNT(CASE WHEN sc2.close > sc2.prev_close THEN 1 END) as up_count,
			COUNT(CASE WHEN sc2.close < sc2.prev_close THEN 1 END) as down_count
		FROM concept_boards cb
		LEFT JOIN stock_concepts sc ON sc.concept_code = cb.concept_code
		LEFT JOIN stock_changes sc2 ON sc2.code = sc.code
		GROUP BY cb.concept_code, cb.concept_name, cb.concept_type, cb.stock_count
		ORDER BY cb.stock_count DESC
	`).Rows()
	
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item model.ConceptHeatmapItem
			rows.Scan(&item.ConceptCode, &item.ConceptName, &item.ConceptType, &item.StockCount,
				&item.AvgChgPct, &item.UpCount, &item.DownCount)
			items = append(items, item)
		}
	}
	
	response.Success(c, items)
}


func (h *BoardHandler) HeatmapEnriched(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	now := time.Now()
	if from == "" {
		from = now.AddDate(0, 0, -20).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	rows, err := h.repo.GetEnrichedHeatmap(from, to)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []model.HeatmapEnriched{}})
		return
	}
	response.Success(c, rows)
}

func (h *BoardHandler) StockHeatmap(c *gin.Context) {
	code := c.Param("code")
	data, err := h.repo.GetStockHeatmap(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []model.AlgorithmPickDetail{}})
		return
	}
	response.Success(c, data)
}

func (h *BoardHandler) Dates(c *gin.Context) {
	var dates []string
	db.PG.Raw("SELECT DISTINCT pick_date::date FROM algorithm_pick_details ORDER BY pick_date DESC LIMIT 30").
		Scan(&dates)
	response.Success(c, dates)
}

