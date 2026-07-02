package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/scheduler"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/ai-stock-predict/server/pkg/response"
)

type CollectorHandler struct {
	sched *scheduler.Scheduler
}

func NewCollectorHandler(sched *scheduler.Scheduler) *CollectorHandler {
	return &CollectorHandler{sched: sched}
}

func (h *CollectorHandler) Trigger(c *gin.Context) {
	var body struct {
		Phases []string `json:"phases"`
	}
	c.ShouldBindJSON(&body)
	log.Printf("[collector] Trigger received phases=%v (len=%d)", body.Phases, len(body.Phases))

	// Filter Go-native phases (market_style) from old scheduler;
	// they are handled by TaskManager.executeTask instead.
	var schedPhases []string
	for _, phase := range body.Phases {
		if phase == "market_style" {
			log.Printf("[collector] routing market_style to Go-native handler")
			go runMarketStyleCollection()
		} else {
			schedPhases = append(schedPhases, phase)
		}
	}

	// Only trigger scheduler if there are non-market_style phases;
	// empty slice triggers ALL phases in RunManualCollection.
	if len(schedPhases) > 0 {
		log.Printf("[collector] triggering scheduler with phases=%v", schedPhases)
		h.sched.Trigger(schedPhases)
	} else {
		log.Printf("[collector] no non-market_style phases, skipping scheduler")
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "采集已触发",
		"data":    collector.GetProgress(),
	})
}

// runMarketStyleCollection runs market style computation and reports via SSE
func runMarketStyleCollection() {
	svc := service.NewMarketStyleService()
	var dates []string
	db.PG.Raw(`SELECT trade_date::text FROM market_sentiment
		WHERE trade_date NOT IN (SELECT trade_date FROM market_style_daily)
		ORDER BY trade_date`).Pluck("trade_date", &dates)

	// Always include the latest date for recompute (overwrite stale data)
	var latestDate string
	db.PG.Raw("SELECT trade_date::text FROM market_sentiment ORDER BY trade_date DESC LIMIT 1").Scan(&latestDate)
	if latestDate != "" {
		hasLatest := false
		for _, d := range dates {
			if d == latestDate { hasLatest = true; break }
		}
		if !hasLatest {
			dates = append(dates, latestDate)
		}
	}

	collector.SetPhase("market_style", "市场风格计算...")

	if len(dates) == 0 {
		collector.SSESend(collector.SSELine{Type: "phase", Phase: "market_style", Message: "市场风格已是最新，无需计算", Level: "info"})
		pr := collector.PhaseResult{Phase: "market_style", New: 0, Total: 0}
		collector.SSESend(collector.SSELine{Type: "result", Phase: "market_style", Result: &pr, Level: "success", Message: "已是最新"})
		collector.SSESend(collector.SSELine{Type: "done", Phase: "done", Level: "success", Message: "市场风格已是最新"})
		collector.AppendResult(pr)
		return
	}

	collector.SSESend(collector.SSELine{Type: "phase", Phase: "market_style", Message: fmt.Sprintf("开始计算市场风格 (%d 个缺失日期)", len(dates)), Level: "info"})

	success, fail := 0, 0
	for _, d := range dates {
		log.Printf("[market_style] computing %s (%d/%d)", d, success+fail+1, len(dates))
		if err := svc.ComputeAndStore(d); err != nil {
			fail++
			log.Printf("[market_style] collection FAIL %s: %v", d, err)
		} else {
			success++
		}
	}

	pr := collector.PhaseResult{Phase: "market_style", New: success, Errors: fail, Total: len(dates)}
	level := "success"
	msg := fmt.Sprintf("市场风格: %d/%d 完成", success, len(dates))
	if fail > 0 {
		level = "error"
		msg += fmt.Sprintf(", %d 失败", fail)
	}
	collector.SSESend(collector.SSELine{Type: "result", Phase: "market_style", Result: &pr, Level: level, Message: msg})
	collector.SSESend(collector.SSELine{Type: "done", Phase: "done", Level: level, Message: msg})
	collector.AppendResult(pr)
}

func (h *CollectorHandler) Status(c *gin.Context) {
	prog := collector.GetProgress()
	prog.LastRun = h.sched.Status()["lastRun"]
	response.Success(c, prog)
}

func (h *CollectorHandler) UpdateSchedule(c *gin.Context) {
	var body struct {
		CronExpr string `json:"cronExpr"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "schedule updated", "cronExpr": body.CronExpr})
}

func (h *CollectorHandler) Stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	collector.RegisterSSEWriter(c.Writer)
	defer collector.UnregisterSSEWriter(c.Writer)
	fmt.Fprintf(c.Writer, "data: {\"type\":\"connected\"}\n\n")
	c.Writer.Flush()
	ctx := c.Request.Context()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		}
	}
}

func (h *CollectorHandler) History(c *gin.Context) {
	var logs []model.CollectionLog
	db.MySQL.Order("started_at DESC").Limit(50).Find(&logs)
	response.Success(c, logs)
}

// ClearHistory deletes stuck or old collection logs (type=stuck|errors|all, default stuck)
func (h *CollectorHandler) ClearHistory(c *gin.Context) {
	clearType := c.DefaultQuery("type", "stuck")

	var runningDeleted, errorDeleted int64

	if clearType == "stuck" || clearType == "all" {
		cutoff := time.Now().Add(-30 * time.Minute)
		result := db.MySQL.Where("status = ? AND started_at < ?", "running", cutoff).Delete(&model.CollectionLog{})
		runningDeleted = result.RowsAffected
	}

	if clearType == "errors" || clearType == "all" {
		cutoff := time.Now().Add(-24 * time.Hour)
		result := db.MySQL.Where("status IN ? AND started_at < ?", []string{"error", "failed"}, cutoff).Delete(&model.CollectionLog{})
		errorDeleted = result.RowsAffected
	}

	totalDeleted := runningDeleted + errorDeleted
	msg := fmt.Sprintf("已清除 %d 条记录", totalDeleted)
	if runningDeleted > 0 && errorDeleted > 0 {
		msg = fmt.Sprintf("已清除 %d 条卡住记录 + %d 条错误记录", runningDeleted, errorDeleted)
	} else if runningDeleted > 0 {
		msg = fmt.Sprintf("已清除 %d 条卡住的采集记录", runningDeleted)
	} else if errorDeleted > 0 {
		msg = fmt.Sprintf("已清除 %d 条错误采集记录", errorDeleted)
	}

	response.Success(c, map[string]interface{}{
		"deleted":      totalDeleted,
		"runningDeleted": runningDeleted,
		"errorDeleted":  errorDeleted,
		"message":      msg,
	})
}

func (h *CollectorHandler) StockCollect(c *gin.Context) {
	code := c.Param("code")
	var body struct {
		Phases []string `json:"phases"`
	}
	c.ShouldBindJSON(&body)
	if len(body.Phases) == 0 {
		body.Phases = []string{"shareholder", "financial", "news"}
	}
	go func() {
		for _, phase := range body.Phases {
			collector.RunStockCollection(phase, code)
		}
	}()
	response.Success(c, gin.H{"message": "单股采集已触发", "stockCode": code, "phases": body.Phases})
}

// CollectStockPhaseSSE runs single-stock collection synchronously with SSE feedback.
// Unlike StockCollect (async), this blocks until the collection completes.
func (h *CollectorHandler) CollectStockPhaseSSE(c *gin.Context) {
	code := c.Param("code")
	phase := c.Param("phase")
	if phase == "" || code == "" {
		c.JSON(400, gin.H{"error": "missing stock code or phase"})
		return
	}

	phaseNames := map[string]string{
		"shareholder": "股东数据", "financial": "财务数据", "news": "资讯数据", "dragon_tiger": "龙虎榜", "block_trade": "大宗交易", "announcements": "公告", "unlocks": "解禁",
		"fund_flow": "资金流",
	}
	phaseName := phaseNames[phase]
	if phaseName == "" {
		phaseName = phase
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	emit := func(typ, msg, level string) {
		data, _ := json.Marshal(map[string]string{"type": typ, "message": msg, "level": level})
		select {
		case <-ctx.Done():
			return
		default:
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	}

	emit("log", fmt.Sprintf("开始拉取 %s %s 数据...", code, phaseName), "info")

	collector.RegisterSSEWriter(c.Writer)
	err := collector.RunStockCollection(phase, code)
	if err != nil {
		emit("error", fmt.Sprintf("%s 采集失败: %v", phaseName, err), "error")
		return
	}

	collector.UnregisterSSEWriter(c.Writer)

	var count int64
	switch phase {
	case "shareholder":
		db.PG.Raw("SELECT count(*) FROM stock_shareholders WHERE code = ?", code).Scan(&count)
	case "financial":
		db.PG.Raw("SELECT count(*) FROM stock_financials WHERE code = ?", code).Scan(&count)
	case "news":
		db.PG.Raw("SELECT count(*) FROM stock_news WHERE code = ?", code).Scan(&count)
	case "dragon_tiger":
		db.PG.Raw("SELECT count(*) FROM dragon_tiger_list WHERE code = ?", code).Scan(&count)
	case "block_trade":
		db.PG.Raw("SELECT count(*) FROM block_trade WHERE code = ?", code).Scan(&count)
	case "announcements":
		db.PG.Raw("SELECT count(*) FROM cninfo_announcements WHERE code = ?", code).Scan(&count)
	case "unlocks":
		db.PG.Raw("SELECT count(*) FROM restricted_share_unlock WHERE code = ?", code).Scan(&count)
	case "fund_flow":
		db.PG.Raw("SELECT count(*) FROM stock_fund_flow WHERE code = ?", code).Scan(&count)
	}

	if count > 0 {
		emit("complete", fmt.Sprintf("成功拉取 %d 条 %s", count, phaseName), "success")
	} else {
		emit("complete", fmt.Sprintf("该股票暂无%s数据", phaseName), "warn")
	}
}

// CollectReports triggers per-stock report collection via Python script


func (h *CollectorHandler) CollectReports(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(400, gin.H{"error": "missing stock code"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	emit := func(typ, msg, level string) {
		data, _ := json.Marshal(map[string]string{"type": typ, "message": msg, "level": level})
		select {
		case <-ctx.Done():
			return
		default:
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	}

	emit("log", "开始拉取 "+code+" 研报数据...", "info")

	cmd := exec.CommandContext(ctx, "python3", "-u", "scripts/collector/report_collect.py", "2024-01-01", "2026-12-31", code)
	cmd.Dir = "/Users/admin/Documents/ai-stock-predict"
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		emit("error", "启动采集失败: "+err.Error(), "error")
		return
	}

	if err := cmd.Start(); err != nil {
		emit("error", "启动采集进程失败: "+err.Error(), "error")
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			emit("log", line, "info")
		}
	}

	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			errMsg := stderr.String()
			if len(errMsg) > 500 {
				errMsg = errMsg[:500] + "..."
			}
			emit("error", "采集异常: "+errMsg, "error")
		}
	}

	var count int64
	if err := db.PG.Raw("SELECT count(*) FROM stock_reports WHERE stock_code = ?", code).Scan(&count).Error; err != nil {
		log.Printf("[collector] report count query failed for %s: %v", code, err)
	}
	if count > 0 {
		emit("complete", fmt.Sprintf("成功拉取 %d 篇研报", count), "success")
	} else {
		emit("complete", "该股票暂无机构研报（近2年无券商覆盖）", "warn")
	}
}

// RealtimeQuotes triggers real-time quote fetch for relevant stocks
// (board picks last 2 days + user's strategy stock pool + holdings).
func (h *CollectorHandler) RealtimeQuotes(c *gin.Context) {
	uidVal, _ := c.Get("userId"); userID := uidVal.(uint)

	// 1. Collect target stock codes
	codeSet := make(map[string]bool)

	// Board picks from last 2 trading days
	var boardCodes []string
	db.PG.Raw(`
		SELECT DISTINCT code FROM board_picks
		WHERE trade_date >= (SELECT MAX(trade_date) FROM board_picks WHERE trade_date < CURRENT_DATE)
		ORDER BY code
	`).Scan(&boardCodes)
	for _, code := range boardCodes {
		codeSet[code] = true
	}

	// User's strategy stock pool
	var stockCodesStr string
	db.MySQL.Raw("SELECT stock_codes FROM strategies WHERE user_id = ? AND stock_codes != '' LIMIT 1", userID).Scan(&stockCodesStr)
	for _, code := range strings.Split(stockCodesStr, ",") {
		code = strings.TrimSpace(code)
		if len(code) == 6 {
			codeSet[code] = true
		}
	}

	// User's current holdings (stocks with non-zero position)
	var holdingCodes []string
	db.MySQL.Raw(`
		SELECT DISTINCT stock_code FROM trade_records
		WHERE user_id = ? AND trade_type = 'buy'
		AND stock_code NOT IN (
			SELECT stock_code FROM trade_records WHERE user_id = ? AND trade_type = 'sell'
			GROUP BY stock_code HAVING SUM(quantity) >= (
				SELECT SUM(quantity) FROM trade_records t2 WHERE t2.user_id = ? AND t2.stock_code = trade_records.stock_code AND t2.trade_type = 'buy'
			)
		)
	`, userID, userID, userID).Scan(&holdingCodes)
	for _, code := range holdingCodes {
		codeSet[code] = true
	}

	if len(codeSet) == 0 {
		response.Success(c, map[string]interface{}{
			"message": "没有需要更新的股票（榜单/自选/持仓均为空）",
			"count":   0,
		})
		return
	}

	codes := make([]string, 0, len(codeSet))
	for code := range codeSet {
		codes = append(codes, code)
	}

	log.Printf("[collector] realtime quotes: %d stocks (board=%d, strategy=%d, holdings=%d)",
		len(codes), len(boardCodes), len(strings.Split(stockCodesStr, ",")), len(holdingCodes))

	// 2. Run Python script
	scriptPath := filepath.Join(collector.ScriptsRoot(), "realtime_quotes.py")
	cmd := exec.Command("python3", "-u", scriptPath, strings.Join(codes, ","))
	cmd.Dir = collector.ScriptsRoot()
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[collector] realtime quotes failed: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "实时行情采集失败: "+string(output))
		return
	}

	// 3. Count updated records
	var count int64
	db.PG.Model(&model.StockRealtimeQuote{}).Count(&count)

	response.Success(c, map[string]interface{}{
		"message": fmt.Sprintf("实时行情更新完成，共 %d 只股票", count),
		"count":   count,
		"output":  string(output),
	})
}

// RealtimeQuoteSingle refreshes real-time quote for a single stock.
func (h *CollectorHandler) RealtimeQuoteSingle(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Error(c, http.StatusBadRequest, 400, "missing stock code")
		return
	}

	scriptPath := filepath.Join(collector.ScriptsRoot(), "realtime_quotes.py")
	cmd := exec.Command("python3", "-u", scriptPath, code)
	cmd.Dir = collector.ScriptsRoot()
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[collector] realtime single %s failed: %v", code, err)
		response.Error(c, http.StatusInternalServerError, 500, "行情刷新失败: "+string(output))
		return
	}

	response.Success(c, map[string]interface{}{
		"message": fmt.Sprintf("%s 行情已刷新", code),
		"output":  string(output),
	})
}
