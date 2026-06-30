package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// SentimentHandler handles market sentiment API requests.
type SentimentHandler struct{}

// GetLatest returns the most recent market sentiment composite score and sub-indicators.
func (h *SentimentHandler) GetLatest(c *gin.Context) {
	s, err := service.GetLatestSentiment()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取市场情绪数据失败: "+err.Error())
		return
	}
	response.Success(c, s)
}

// GetHistory returns sentiment history for the last N days (default 90).
func (h *SentimentHandler) GetHistory(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "90")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		days = 90
	}

	list, err := service.GetSentimentHistory(days)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取历史情绪数据失败: "+err.Error())
		return
	}
	response.Success(c, list)
}

// GetDetail returns sentiment detail for a specific date.
func (h *SentimentHandler) GetDetail(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		response.Error(c, http.StatusBadRequest, 400, "缺少日期参数 ?date=2026-01-01")
		return
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "日期格式错误, 请使用 YYYY-MM-DD")
		return
	}

	s, err := service.GetSentimentByDate(t)
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "该日期无情绪数据")
		return
	}
	response.Success(c, s)
}

// GetRange returns sentiment data for a date range.
func (h *SentimentHandler) GetRange(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		response.Error(c, http.StatusBadRequest, 400, "缺少日期范围参数 ?start=&end=")
		return
	}
	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "start 日期格式错误")
		return
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "end 日期格式错误")
		return
	}

	list, err := service.GetSentimentDateRange(start, end)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取区间数据失败: "+err.Error())
		return
	}
	response.Success(c, list)
}

// GetNorthbound returns recent daily northbound flow from the view.
func (h *SentimentHandler) GetNorthbound(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, _ := strconv.Atoi(daysStr)
	if days < 1 || days > 365 {
		days = 30
	}

	list, err := service.GetNorthboundHistory(days)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取北向数据失败: "+err.Error())
		return
	}
	response.Success(c, list)
}

// GetNorthboundMinute returns minute-level northbound data for the latest trading day.
func (h *SentimentHandler) GetNorthboundMinute(c *gin.Context) {
	list, err := service.GetNorthboundMinuteHistory(1)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取北向分钟数据失败: "+err.Error())
		return
	}
	response.Success(c, list)
}

// GetReturnDistribution returns the return distribution histogram for the latest trading day.
func (h *SentimentHandler) GetReturnDistribution(c *gin.Context) {
	type Bucket struct {
		Label string `json:"label"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
		Count int    `json:"count"`
	}

	type row struct {
		Bucket string `json:"label"`
		Cnt    int    `json:"count"`
	}
	var rows []row

	err := db.PG.Raw(`
		WITH returns AS (
			SELECT k.code,
				(k.close - kp.close) / NULLIF(kp.close, 0) as ret,
				b.board_type, COALESCE(b.is_st, false) as is_st
			FROM stocks_daily_k k
			JOIN LATERAL (
				SELECT k2.close FROM stocks_daily_k k2
				WHERE k2.code = k.code AND k2.trade_date < k.trade_date
				ORDER BY k2.trade_date DESC LIMIT 1
			) kp ON TRUE
			JOIN stocks_basic b ON b.code = k.code
			WHERE k.trade_date = (SELECT MAX(trade_date) FROM market_daily_agg)
			  AND k.close > 0 AND kp.close > 0
		)
		SELECT
			CASE
				WHEN board_type IN ('kc','cy') AND ret >= 0.1999 THEN '涨停'
				WHEN board_type = 'bj' AND ret >= 0.2999 THEN '涨停'
				WHEN is_st AND ret >= 0.0499  THEN '涨停'
				WHEN ret >= 0.0999 THEN '涨停'
				WHEN board_type IN ('kc','cy') AND ret <= -0.1999 THEN '跌停'
				WHEN board_type = 'bj' AND ret <= -0.2999 THEN '跌停'
				WHEN is_st AND ret <= -0.0499 THEN '跌停'
				WHEN ret <= -0.0999 THEN '跌停'
				WHEN ret <= -0.08 THEN '< -8%'
				WHEN ret < -0.06 THEN '-8%~-6%'
				WHEN ret < -0.04 THEN '-6%~-4%'
				WHEN ret < -0.02 THEN '-4%~-2%'
				WHEN ret < -0.001 THEN '-2%~0'
				WHEN ret <= 0.001 THEN '0%'
				WHEN ret < 0.02 THEN '0~2%'
				WHEN ret < 0.04 THEN '2%~4%'
				WHEN ret < 0.06 THEN '4%~6%'
				WHEN ret < 0.08 THEN '6%~8%'
				ELSE '> 8%'
			END as bucket,
			COUNT(*) as cnt
		FROM returns
		GROUP BY 1
		ORDER BY
			CASE
				WHEN MIN(ret) <= -0.0999 THEN -999
				WHEN AVG(ret) <= -0.08 THEN -8
				WHEN AVG(ret) < -0.06 THEN -7
				WHEN AVG(ret) < -0.04 THEN -5
				WHEN AVG(ret) < -0.02 THEN -3
				WHEN AVG(ret) < 0 THEN -1
				WHEN AVG(ret) < 0.02 THEN 1
				WHEN AVG(ret) < 0.04 THEN 3
				WHEN AVG(ret) < 0.06 THEN 5
				WHEN AVG(ret) < 0.08 THEN 7
				WHEN AVG(ret) < 0.0999 THEN 9
				ELSE 999
			END
	`).Scan(&rows).Error

	buckets := make([]Bucket, len(rows))
	for i, r := range rows {
		buckets[i] = Bucket{Label: r.Bucket, Count: r.Cnt}
	}

	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取涨跌分布失败: "+err.Error())
		return
	}

	response.Success(c, buckets)
}

// GetMarketTurnover returns latest market turnover with previous day comparison.
func (h *SentimentHandler) GetMarketTurnover(c *gin.Context) {
	type TurnoverData struct {
		TradeDate  string  `json:"tradeDate"`
		Amount     float64 `json:"amount"`     // 亿元
		PrevAmount float64 `json:"prevAmount"` // 上一交易日 亿元
		Change     float64 `json:"change"`     // 变化额 亿元
		ChangePct  float64 `json:"changePct"`  // 变化百分比
	}

	var result TurnoverData

	var tradeDate time.Time
	var amount, prevAmount float64
	db.PG.Raw(`
		SELECT a.trade_date,
			COALESCE((SELECT SUM(amount) FROM stocks_daily_k WHERE trade_date = a.trade_date AND code !~ '^IDX'), 0)  / 1e8,
			COALESCE((SELECT SUM(amount) FROM stocks_daily_k WHERE trade_date = b.trade_date AND code !~ '^IDX'), 0)  / 1e8
		FROM market_daily_agg a
		LEFT JOIN market_daily_agg b ON b.trade_date = (
			SELECT MAX(trade_date) FROM market_daily_agg WHERE trade_date < a.trade_date
		)
		ORDER BY a.trade_date DESC LIMIT 1
	`).Row().Scan(&tradeDate, &amount, &prevAmount)

	result.TradeDate = tradeDate.Format("2006-01-02")
	result.Amount = amount
	result.PrevAmount = prevAmount
	if amount > 0 {
		result.Change = amount - prevAmount
		if prevAmount > 0 {
			result.ChangePct = (amount - prevAmount) / prevAmount * 100
		}
	}

	response.Success(c, result)
}

// GetIndexKLine returns K-line data for a market index (public, no auth).
func (h *SentimentHandler) GetIndexKLine(c *gin.Context) {
	code := c.Param("code")
	from := c.Query("from")
	to := c.Query("to")

	type KLineRow struct {
		TradeDate string  `json:"tradeDate"`
		Open      float64 `json:"open"`
		Close     float64 `json:"close"`
		High      float64 `json:"high"`
		Low       float64 `json:"low"`
		Volume    int64   `json:"volume"`
		Amount    float64 `json:"amount"`
	}

	var rows []KLineRow
	query := "SELECT trade_date, open, close, high, low, volume, amount FROM stocks_daily_k WHERE code = ?"
	args := []interface{}{code}
	if from != "" {
		query += " AND trade_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND trade_date <= ?"
		args = append(args, to)
	}
	query += " ORDER BY trade_date ASC"

	if err := db.PG.Raw(query, args...).Scan(&rows).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "查询K线失败")
		return
	}
	if rows == nil {
		rows = []KLineRow{}
	}
	response.Success(c, rows)
}

// GetLimitStats returns daily limit-up/down statistics for the last N days.
func (h *SentimentHandler) GetLimitStats(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "60")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		days = 60
	}

	list, err := service.GetLimitStatsHistory(days)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取涨跌停统计失败: "+err.Error())
		return
	}
	if list == nil {
		list = []service.LimitStats{}
	}
	response.Success(c, list)
}
