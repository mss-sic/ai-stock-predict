package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type SSELine struct {
	Type    string       `json:"type"`
	Phase   string       `json:"phase,omitempty"`
	Message string       `json:"message,omitempty"`
	Level   string       `json:"level,omitempty"`
	Result  *PhaseResult `json:"result,omitempty"`
}

type sseWriter struct {
	mu      sync.Mutex
	buf     strings.Builder
	level   string
	writers []io.Writer
}

func (w *sseWriter) Write(p []byte) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[sseWriter] panic: %v", r)
		}
	}()
	w.mu.Lock()
	defer w.mu.Unlock()
	n = len(p)
	for _, b := range p {
		if b == '\n' {
			line := strings.TrimSpace(w.buf.String())
			w.buf.Reset()
			line = strings.TrimPrefix(line, "\r")
			if line != "" {
				data, _ := json.Marshal(SSELine{Type: "log", Message: line, Level: w.level})
				sseLine := fmt.Sprintf("data: %s\n\n", data)
				for _, wr := range w.writers {
					wr.Write([]byte(sseLine))
					if f, ok := wr.(http.Flusher); ok {
						f.Flush()
					}
				}
			}
		} else {
			w.buf.WriteByte(b)
		}
	}
	return
}

func (w *sseWriter) addWriter(wr io.Writer) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writers = append(w.writers, wr)
}

func (w *sseWriter) flushRemaining() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		line := strings.TrimSpace(w.buf.String())
		if line != "" {
			data, _ := json.Marshal(SSELine{Type: "log", Message: line, Level: w.level})
			sseLine := fmt.Sprintf("data: %s\n\n", data)
			for _, wr := range w.writers {
				wr.Write([]byte(sseLine))
			}
		}
		w.buf.Reset()
	}
}

// ---- Progress State ----

type PhaseResult struct {
	Phase      string `json:"phase"`
	Total      int    `json:"total"`
	New        int    `json:"new"`
	Skipped    int    `json:"skipped"`
	Errors     int    `json:"errors"`
	DurationMs int64  `json:"durationMs"`
}

type CollectionProgress struct {
	mu       sync.RWMutex
	Running  bool          `json:"running"`
	Phase    string        `json:"phase"`
	Current  int           `json:"current"`
	Total    int           `json:"total"`
	Message  string        `json:"message"`
	Results  []PhaseResult `json:"results"`
	Started  time.Time     `json:"started"`
	Finished *time.Time    `json:"finished"`
	LastRun  interface{}   `json:"lastRun"`
	Errors   []string      `json:"errors"`
	LastOutput time.Time   `json:"-"`
}

var (
	progress     = &CollectionProgress{}
	activeWriter *sseWriter
	writerMu     sync.Mutex
)

func GetProgress() *CollectionProgress {
	progress.mu.RLock()
	// Auto-reset only if truly stuck: no log output for 15+ minutes
	if progress.Running && !progress.LastOutput.IsZero() && time.Since(progress.LastOutput) > 15*time.Minute {
		progress.mu.RUnlock()
		progress.mu.Lock()
		if progress.Running && !progress.LastOutput.IsZero() && time.Since(progress.LastOutput) > 15*time.Minute {
			progress.Running = false
			progress.Phase = "done"
			now := time.Now()
			progress.Finished = &now
			log.Println("[collector] auto-reset stuck collection (no output for 15+ min)")
		}
		progress.mu.Unlock()
		progress.mu.RLock()
	}
	cp := *progress
	cp.Results = make([]PhaseResult, len(progress.Results))
	copy(cp.Results, progress.Results)
	cp.Errors = make([]string, len(progress.Errors))
	copy(cp.Errors, progress.Errors)
	progress.mu.RUnlock()
	return &cp
}

func sseSend(line SSELine) {
	progress.mu.Lock()
	progress.LastOutput = time.Now()
	progress.mu.Unlock()
	writerMu.Lock()
	w := activeWriter
	writerMu.Unlock()
	if w != nil {
		data, _ := json.Marshal(line)
		w.mu.Lock()
		for _, wr := range w.writers {
			fmt.Fprintf(wr, "data: %s\n\n", data)
			if f, ok := wr.(http.Flusher); ok {
				f.Flush()
			}
		}
		w.mu.Unlock()
	}
}

func runPythonStream(script string) error {
	return runPythonStreamWithArgs(script)
}

func runPythonStreamWithArgs(script string, args ...string) error {
	scriptPath := filepath.Join(scriptsRoot(), script)
	cmdArgs := append([]string{"-u", scriptPath}, args...)
	cmd := exec.Command("python3", cmdArgs...)
	cmd.Dir = scriptsRoot()
	cmd.Env = append(cmd.Environ(), "PYTHONUNBUFFERED=1", "PYTHONIOENCODING=utf-8")

	stdoutW := &sseWriter{level: "info"}
	stderrW := &sseWriter{level: "stderr"}
	writerMu.Lock()
	if activeWriter != nil {
		stdoutW.writers = append(stdoutW.writers, activeWriter.writers...)
		stderrW.writers = append(stderrW.writers, activeWriter.writers...)
	}
	writerMu.Unlock()

	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	err := cmd.Run()
	stdoutW.flushRemaining()
	stderrW.flushRemaining()
	return err
}

// RunManualCollection runs all phases. If phases is non-empty, only those phases run.
func RunManualCollection(phases []string) {
	progress.mu.Lock()
	if progress.Running {
		progress.mu.Unlock()
		log.Println("[collector] already running, skip")
		return
	}
	progress.Running = true
	progress.LastOutput = time.Now()
	progress.Phase = "starting"
	progress.Current = 0
	totalPhases := len(phases)
	if totalPhases == 0 {
		totalPhases = 14
	}
	progress.Total = totalPhases
	progress.Results = nil
	progress.Errors = nil
	progress.Started = time.Now()
	progress.Finished = nil
	progress.mu.Unlock()

	logEntry := model.CollectionLog{
		Status:    "running",
		StartedAt: time.Now(),
	}
	db.MySQL.Create(&logEntry)

	defer func() {
		now := time.Now()
		progress.mu.Lock()
		progress.Running = false
		progress.Phase = "done"
		progress.Finished = &now
		progress.Current = len(progress.Results)
		progress.mu.Unlock()

		totalNew, totalSkipped, totalErrors := 0, 0, 0
		for _, r := range progress.Results {
			totalNew += r.New
			totalSkipped += r.Skipped
			totalErrors += r.Errors
		}
		status := "success"
		if totalErrors > 0 {
			status = "partial"
		}
		durationMs := now.Sub(logEntry.StartedAt).Milliseconds()
		phasesJSON, _ := json.Marshal(progress.Results)
		db.MySQL.Model(&logEntry).Updates(map[string]interface{}{
			"phases": string(phasesJSON), "total_new": totalNew,
			"total_skipped": totalSkipped, "total_errors": totalErrors,
			"status": status, "duration_ms": durationMs, "finished_at": now,
		})
		sseSend(SSELine{Type: "done", Phase: "done", Level: "success",
			Message: fmt.Sprintf("采集完成: 新增 %d, 跳过 %d, 错误 %d, 耗时 %dms", totalNew, totalSkipped, totalErrors, durationMs)})
	}()

	shouldRun := func(p string) bool {
		if len(phases) == 0 {
			return true
		}
		for _, ph := range phases {
			if ph == p {
				return true
			}
		}
		return false
	}

	if shouldRun("full_sync") {
		appendResult(runFullSyncPhase())
	}
	if shouldRun("kline") {
		appendResult(runKLinePhase())
	}
	if shouldRun("indicator") {
		appendResult(runIndicatorPhase())
	}
	if shouldRun("industry") {
		appendResult(runIndustryPhase())
	}
	if shouldRun("shareholder") {
		appendResult(runShareholderPhase())
	}
	if shouldRun("financial") {
		appendResult(runFinancialPhase())
	}
	if shouldRun("news") {
		appendResult(runNewsPhase())
	}

	if shouldRun("reports") {
		appendResult(runReportsPhase())
	}

	// === Backfill phases ===
	if shouldRun("backfill_financial") {
		appendResult(runBackfillFinancialPhase())
	}
	if shouldRun("backfill_shareholder") {
		appendResult(runBackfillShareholderPhase())
	}
	if shouldRun("backfill_indicator") {
		appendResult(runBackfillIndicatorPhase())
	}
	if shouldRun("concept") {
		appendResult(runConceptPhase())
	}
	if shouldRun("profile") {
		appendResult(runProfilePhase())
	}
}

func runProfilePhase() PhaseResult {
	setPhase("profile", "AI 股票简介+六维评分...")
	sseSend(SSELine{Type: "phase", Phase: "profile", Message: "开始 AI 股票简介+六维评分采集...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockProfile{}).Count(&before)
	runPythonStreamWithArgs("stock_profile_collect.py", "--batch")
	phaseRes := PhaseResult{Phase: "profile", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockProfile{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "profile", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("AI简介: 新增 %d 份", after-before)})
	return phaseRes
}

func runConceptPhase() PhaseResult {
	setPhase("concept", "采集概念板块数据...")
	sseSend(SSELine{Type: "phase", Phase: "concept", Message: "开始采集东方财富概念板块...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockConcept{}).Count(&before)
	if err := CollectConcepts(); err != nil {
		phaseRes := PhaseResult{Phase: "concept", Errors: 1}
		phaseRes.DurationMs = time.Since(t0).Milliseconds()
		sseSend(SSELine{Type: "result", Phase: "concept", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("概念采集失败: %v", err)})
		return phaseRes
	}
	var after int64
	db.PG.Model(&model.StockConcept{}).Count(&after)
	phaseRes := PhaseResult{Phase: "concept", Total: int(after), New: int(after - before)}
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "concept", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("概念板块: %d 条关联", after)})
	return phaseRes
}

func runFullSyncPhase() PhaseResult {
	setPhase("full_sync", "同步A股股票列表...")
	sseSend(SSELine{Type: "phase", Phase: "full_sync", Message: "开始同步A股股票列表...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockBasic{}).Count(&before)
	runPythonStream("full_sync.py")
	phaseRes := PhaseResult{Phase: "full_sync", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockBasic{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "full_sync", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("列表同步: %d 只", after)})
	return phaseRes
}

func runKLinePhase() PhaseResult {
	setPhase("kline", "采集日K线数据...")
	sseSend(SSELine{Type: "phase", Phase: "kline", Message: "开始采集日K线数据...", Level: "info"})
	t0 := time.Now()
	var totalStocks int64
	db.PG.Model(&model.StockBasic{}).Count(&totalStocks)
	var stocksWithK int64
	if err := db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k").Scan(&stocksWithK).Error; err != nil {
	log.Printf("[collector] stocksWithK query failed: %v", err)
}

	// 检查数据新鲜度：最近交易日距今超过 3 天则重新采集
	needK := int(totalStocks - stocksWithK)
	var latestDate time.Time
	if err := db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k").Scan(&latestDate).Error; err != nil {
	log.Printf("[collector] latestDate query failed: %v", err)
}
	stale := time.Since(latestDate) > 72*time.Hour

	if needK <= 0 && !stale {
		pr := PhaseResult{Phase: "kline", Total: int(stocksWithK), Skipped: int(stocksWithK), DurationMs: time.Since(t0).Milliseconds()}
		sseSend(SSELine{Type: "result", Phase: "kline", Result: &pr, Level: "success", Message: fmt.Sprintf("K线已完整 (%d 只), 跳过", stocksWithK)})
		return pr
	}
	if stale && needK <= 0 {
		sseSend(SSELine{Type: "log", Message: fmt.Sprintf("K线数据过期 (最新: %s), 重新采集", latestDate.Format("2006-01-02")), Level: "info"})
	}
	sseSend(SSELine{Type: "log", Message: fmt.Sprintf("需采集K线: %d 只", needK), Level: "info"})
	runPythonStream("batch_collect.py")
	phaseRes := PhaseResult{Phase: "kline", Skipped: int(stocksWithK)}
	var after int64
	if err := db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k").Scan(&after).Error; err != nil {
	log.Printf("[collector] after count query failed: %v", err)
}
	phaseRes.Total = int(after)
	phaseRes.New = int(after - stocksWithK)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "kline", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("K线: %d 只", after)})
	return phaseRes
}

func runIndicatorPhase() PhaseResult {
	setPhase("indicator", "采集PE/PB指标...")
	sseSend(SSELine{Type: "phase", Phase: "indicator", Message: "开始采集PE/PB指标...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockDailyIndicator{}).Count(&before)
	runPythonStream("daily_indicator.py")
	phaseRes := PhaseResult{Phase: "indicator", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockDailyIndicator{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "indicator", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("PE/PB: 新增 %d", after-before)})
	return phaseRes
}

func runIndustryPhase() PhaseResult {
	setPhase("industry", "填充行业分类...")
	sseSend(SSELine{Type: "phase", Phase: "industry", Message: "开始填充行业分类...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockBasic{}).Where("industry IS NOT NULL AND industry != ''").Count(&before)
	runPythonStream("populate_industry.py")
	phaseRes := PhaseResult{Phase: "industry", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockBasic{}).Where("industry IS NOT NULL AND industry != ''").Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "industry", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("行业: 新增 %d", after-before)})
	return phaseRes
}



func runShareholderPhase() PhaseResult {
	setPhase("shareholder", "采集股东户数...")
	sseSend(SSELine{Type: "phase", Phase: "shareholder", Message: "开始采集股东户数...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockShareholder{}).Count(&before)
	runPythonStream("shareholder_collect.py")
	phaseRes := PhaseResult{Phase: "shareholder", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockShareholder{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "shareholder", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("股东: 新增 %d", after-before)})
	return phaseRes
}

func runFinancialPhase() PhaseResult {
	setPhase("financial", "采集财务数据...")
	sseSend(SSELine{Type: "phase", Phase: "financial", Message: "开始采集财务数据...", Level: "info"})
	t0 := time.Now()
	var totalStocks int64
	db.PG.Model(&model.StockBasic{}).Count(&totalStocks)
	var existing int64
	if err := db.PG.Model(&model.StockFinancial{}).Select("COUNT(DISTINCT code)").Scan(&existing).Error; err != nil {
	log.Printf("[collector] financial existing query failed: %v", err)
}
	need := totalStocks - existing
	if need <= 0 {
		pr := PhaseResult{Phase: "financial", Total: int(existing), Skipped: int(existing), DurationMs: time.Since(t0).Milliseconds()}
		sseSend(SSELine{Type: "result", Phase: "financial", Result: &pr, Level: "success", Message: fmt.Sprintf("财务数据完整 (%d 只), 跳过", existing)})
		return pr
	}
	sseSend(SSELine{Type: "log", Message: fmt.Sprintf("需采集财务: %d 只 (总计 %d, 已有 %d)", need, totalStocks, existing), Level: "info"})
	runPythonStream("financial_collect.py")
	phaseRes := PhaseResult{Phase: "financial", Skipped: int(existing)}
	var after int64
	if err := db.PG.Model(&model.StockFinancial{}).Select("COUNT(DISTINCT code)").Scan(&after).Error; err != nil {
	log.Printf("[collector] financial after query failed: %v", err)
}
	phaseRes.Total = int(after)
	phaseRes.New = int(after - existing)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "financial", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("财务: %d 只", after)})
	return phaseRes
}

func runNewsPhase() PhaseResult {
	setPhase("news", "采集资讯数据...")
	sseSend(SSELine{Type: "phase", Phase: "news", Message: "开始采集资讯数据...", Level: "info"})
	t0 := time.Now()
	var totalStocks int64
	db.PG.Model(&model.StockBasic{}).Count(&totalStocks)
	var existing int64
	if err := db.PG.Model(&model.StockNews{}).Select("COUNT(DISTINCT code)").Scan(&existing).Error; err != nil {
	log.Printf("[collector] news existing query failed: %v", err)
}
	need := totalStocks - existing
	if need <= 0 {
		pr := PhaseResult{Phase: "news", Total: int(existing), Skipped: int(existing), DurationMs: time.Since(t0).Milliseconds()}
		sseSend(SSELine{Type: "result", Phase: "news", Result: &pr, Level: "success", Message: fmt.Sprintf("资讯数据完整 (%d 只), 跳过", existing)})
		return pr
	}
	sseSend(SSELine{Type: "log", Message: fmt.Sprintf("需采集资讯: %d 只 (总计 %d, 已有 %d)", need, totalStocks, existing), Level: "info"})
	runPythonStream("news_collect.py")
	phaseRes := PhaseResult{Phase: "news", Skipped: int(existing)}
	var after int64
	if err := db.PG.Model(&model.StockNews{}).Select("COUNT(DISTINCT code)").Scan(&after).Error; err != nil {
	log.Printf("[collector] news after query failed: %v", err)
}
	phaseRes.Total = int(after)
	phaseRes.New = int(after - existing)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "news", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("资讯: %d 只", after)})
	return phaseRes
}

// RunStockCollection runs a single phase for a single stock code
func RunStockCollection(phase, code string) error {
	log.Printf("[collector] stock collection phase=%s code=%s", phase, code)
	switch phase {
	case "shareholder":
		return runPythonStreamWithArgs("shareholder_collect.py", code)
	case "financial":
		return runPythonStreamWithArgs("financial_collect.py", code)
	case "news":
		return runPythonStreamWithArgs("news_collect.py", code)
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}
}

func setPhase(phase, msg string) {
	progress.mu.Lock()
	progress.Phase = phase
	progress.Message = msg
	progress.mu.Unlock()
}

func appendResult(r PhaseResult) {
	progress.mu.Lock()
	progress.Results = append(progress.Results, r)
	progress.Current = len(progress.Results)
	progress.mu.Unlock()
}

func addError(msg string) {
	progress.mu.Lock()
	progress.Errors = append(progress.Errors, msg)
	progress.mu.Unlock()
	log.Printf("[collector] ERROR: %s", msg)
}

func RegisterSSEWriter(w io.Writer) {
	writerMu.Lock()
	defer writerMu.Unlock()
	if activeWriter == nil {
		activeWriter = &sseWriter{level: "info"}
	}
	activeWriter.addWriter(w)
}

func UnregisterSSEWriter(w io.Writer) {
	writerMu.Lock()
	defer writerMu.Unlock()
	if activeWriter == nil {
		return
	}
	activeWriter.mu.Lock()
	defer activeWriter.mu.Unlock()
	var kept []io.Writer
	for _, wr := range activeWriter.writers {
		if wr != w {
			kept = append(kept, wr)
		}
	}
	activeWriter.writers = kept
}

func runReportsPhase() PhaseResult {
	setPhase("reports", "采集研报数据...")
	sseSend(SSELine{Type: "phase", Phase: "reports", Message: "开始采集研报数据...", Level: "info"})
	t0 := time.Now()
	var existing int64
	db.PG.Model(&model.StockReport{}).Count(&existing)
	sseSend(SSELine{Type: "log", Message: fmt.Sprintf("当前研报: %d 篇, 开始增量拉取...", existing), Level: "info"})

	beginDate := "2024-01-01"
	endDate := time.Now().Format("2006-01-02")
	args := []string{beginDate, endDate}

	err := runPythonStreamWithArgs("report_collect.py", args...)
	if err != nil {
		pr := PhaseResult{Phase: "reports", Skipped: int(existing), Errors: 1, DurationMs: time.Since(t0).Milliseconds()}
		sseSend(SSELine{Type: "result", Phase: "reports", Result: &pr, Level: "error", Message: fmt.Sprintf("研报采集失败: %v", err)})
		return pr
	}

	var after int64
	db.PG.Model(&model.StockReport{}).Count(&after)
	phaseRes := PhaseResult{
		Phase:      "reports",
		Total:      int(after),
		New:        int(after - existing),
		Skipped:    int(existing),
		DurationMs: time.Since(t0).Milliseconds(),
	}
	sseSend(SSELine{Type: "result", Phase: "reports", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("研报: %d 篇 (新增 %d)", after, phaseRes.New)})
	return phaseRes
}

func runBackfillFinancialPhase() PhaseResult {
	setPhase("backfill_financial", "财报数据全量回填...")
	sseSend(SSELine{Type: "phase", Phase: "backfill_financial", Message: "开始全量回填财报数据（利润表+资产负债表）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockFinancial{}).Count(&before)
	runPythonStream("backfill_financial.py")
	phaseRes := PhaseResult{Phase: "backfill_financial", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockFinancial{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "backfill_financial", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("财报回填: +%d 条", after-before)})
	return phaseRes
}

func runBackfillShareholderPhase() PhaseResult {
	setPhase("backfill_shareholder", "股东数据全量回填...")
	sseSend(SSELine{Type: "phase", Phase: "backfill_shareholder", Message: "开始全量回填股东数据（股东户数+十大股东）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockShareholder{}).Count(&before)
	runPythonStream("backfill_shareholder.py")
	phaseRes := PhaseResult{Phase: "backfill_shareholder", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockShareholder{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "backfill_shareholder", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("股东回填: +%d 条", after-before)})
	return phaseRes
}

func runBackfillIndicatorPhase() PhaseResult {
	setPhase("backfill_indicator", "PE/PB指标历史回填...")
	sseSend(SSELine{Type: "phase", Phase: "backfill_indicator", Message: "开始计算历史PE/PB/PS指标（从K线×财报）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockDailyIndicator{}).Count(&before)
	runPythonStreamWithArgs("backfill_indicator.py", "2024-01-01")
	phaseRes := PhaseResult{Phase: "backfill_indicator", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockDailyIndicator{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	sseSend(SSELine{Type: "result", Phase: "backfill_indicator", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("指标回填: +%d 条", after-before)})
	return phaseRes
}
