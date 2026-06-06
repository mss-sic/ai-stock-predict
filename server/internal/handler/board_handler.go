package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/repository"
	"github.com/gin-gonic/gin"
)

type BoardHandler struct {
	repo *repository.BoardRepo
}

func NewBoardHandler() *BoardHandler {
	return &BoardHandler{repo: &repository.BoardRepo{}}
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
	if err := db.PG.Where("pick_date = ?", dateStr).Order("score DESC").Find(&picks).Error; err != nil {
		return nil, dateStr, err
	}

	if len(picks) == 0 {
		return []EnrichedBoardItem{}, dateStr, nil
	}

	codes := make([]string, len(picks))
	for i, p := range picks {
		codes[i] = p.StockCode
	}

	// Stock names + industries
	var stocks []model.StockBasic
	db.PG.Where("code IN ?", codes).Find(&stocks)
	stockMap := make(map[string]model.StockBasic)
	for _, s := range stocks {
		stockMap[s.Code] = s
	}

	// K-line for pick date
	type KLineRow struct {
		Code     string
		Close    float64
		Open     float64
		PreClose float64
	}
	var klines []KLineRow
	db.PG.Raw(`
		SELECT k.code, k.close, k.open,
			COALESCE(LAG(k.close) OVER (PARTITION BY k.code ORDER BY k.trade_date), k.open) AS pre_close
		FROM stocks_daily_k k
		WHERE k.code IN ? AND k.trade_date = ?
	`, codes, dateStr).Scan(&klines)
	klineMap := make(map[string]KLineRow)
	for _, k := range klines {
		klineMap[k.Code] = k
	}

	// Next-day K-line
	type NextDateRow struct {
		Code     string
		NextDate string
	}
	var nextDates []NextDateRow
	db.PG.Raw(`
		SELECT code, MIN(trade_date)::text AS next_date
		FROM stocks_daily_k
		WHERE code IN ? AND trade_date > ?
		GROUP BY code
	`, codes, dateStr).Scan(&nextDates)

	nextDateMap := make(map[string]string)
	for _, nd := range nextDates {
		nextDateMap[nd.Code] = nd.NextDate
	}

	type NextKLineRow struct {
		Code     string
		Open     float64
		Close    float64
		PreClose float64
	}
	nextKlineMap := make(map[string]NextKLineRow)
	if len(nextDates) > 0 {
		var conditions []string
		var args []interface{}
		for _, nd := range nextDates {
			conditions = append(conditions, "(code = ? AND trade_date = ?)")
			args = append(args, nd.Code, nd.NextDate)
		}
		sql := fmt.Sprintf(`
			SELECT n.code, n.open, n.close,
				COALESCE(LAG(n.close) OVER (PARTITION BY n.code ORDER BY n.trade_date), n.open) AS pre_close
			FROM stocks_daily_k n
			WHERE %s
		`, joinStrings(conditions, " OR "))
		var nkl []NextKLineRow
		db.PG.Raw(sql, args...).Scan(&nkl)
		for _, n := range nkl {
			nextKlineMap[n.Code] = n
		}
	}

	// Latest close price for each stock (cumulative return)
	type LatestCloseRow struct {
		Code  string
		Close float64
	}
	var latestCloses []LatestCloseRow
	db.PG.Raw(`
		SELECT l.code, l.close
		FROM stocks_daily_k l
		INNER JOIN (
			SELECT code, MAX(trade_date) AS max_date
			FROM stocks_daily_k
			WHERE code IN ?
			GROUP BY code
		) latest ON l.code = latest.code AND l.trade_date = latest.max_date
	`, codes).Scan(&latestCloses)
	latestCloseMap := make(map[string]float64)
	for _, lc := range latestCloses {
		latestCloseMap[lc.Code] = lc.Close
	}

	// PE/PB
	type IndicatorRow struct {
		Code string
		PE   float64
		PB   float64
		MCap float64
	}
	var indicators []IndicatorRow
	db.PG.Raw(`
		SELECT i.code, i.pe, i.pb, i.total_market_cap
		FROM stocks_daily_indicator i
		INNER JOIN (
			SELECT code, MAX(trade_date) as max_date
			FROM stocks_daily_indicator
			WHERE code IN ?
			GROUP BY code
		) latest ON i.code = latest.code AND i.trade_date = latest.max_date
	`, codes).Scan(&indicators)
	indMap := make(map[string]IndicatorRow)
	for _, ind := range indicators {
		indMap[ind.Code] = ind
	}

	// AI Stock Scores — override risk/suggestion with AI analysis results
	type AIScoreRow struct {
		Code       string
		RiskLevel  string
		Suggestion string
	}
	var aiScores []AIScoreRow
	db.PG.Raw(`
		SELECT s.code, s.risk_level, s.suggestion
		FROM ai_stock_scores s
		INNER JOIN (
			SELECT code, MAX(analyzed_at) AS max_at
			FROM ai_stock_scores
			WHERE code IN ?
			GROUP BY code
		) latest ON s.code = latest.code AND s.analyzed_at = latest.max_at
	`, codes).Scan(&aiScores)
	aiScoreMap := make(map[string]AIScoreRow)
	for _, a := range aiScores {
		aiScoreMap[a.Code] = a
	}

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
		// Override with AI analysis results
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
		if n, ok := nextKlineMap[p.StockCode]; ok {
			item.NextDate = nextDateMap[p.StockCode]
			item.NextOpen = n.Open
			item.NextClose = n.Close
			if n.PreClose > 0 {
				item.NextChgPct = ((n.Close - n.PreClose) / n.PreClose) * 100
			}
		}
		// Cumulative return: (latest_close - pick_close) / pick_close * 100
		if latestClose, ok := latestCloseMap[p.StockCode]; ok && latestClose > 0 && item.Close > 0 {
			item.CumuChgPct = ((latestClose - item.Close) / item.Close) * 100
		}

		result = append(result, item)
	}

	return result, dateStr, nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	r := strs[0]
	for i := 1; i < len(strs); i++ {
		r += sep + strs[i]
	}
	return r
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
	c.JSON(http.StatusOK, gin.H{"data": data})
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
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *BoardHandler) StockHeatmap(c *gin.Context) {
	code := c.Param("code")
	data, err := h.repo.GetStockHeatmap(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []model.AlgorithmPickDetail{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *BoardHandler) Dates(c *gin.Context) {
	var dates []string
	db.PG.Raw("SELECT DISTINCT pick_date::date FROM algorithm_pick_details ORDER BY pick_date DESC LIMIT 30").
		Scan(&dates)
	c.JSON(http.StatusOK, gin.H{"data": dates})
}
