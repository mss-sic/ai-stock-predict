package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
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
	// Stats contains key-value statistics from collection (e.g. "records": 1234)
	Stats map[string]int64 `json:"stats,omitempty"`
	// ProgressCurrent / ProgressTotal track per-phase sub-progress
	ProgressCurrent int `json:"progressCurrent,omitempty"`
	ProgressTotal   int `json:"progressTotal,omitempty"`
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
				w.emitLine(line)
			}
		} else {
			w.buf.WriteByte(b)
		}
	}
	return
}

// emitLine parses a single output line and sends the appropriate SSE message.
// Recognizes STAT:key=val and PROGRESS:current/total prefixes for structured data.
func (w *sseWriter) emitLine(line string) {
	// STAT: lines are for silent metric accumulation only — not emitted to SSE.
	// The human-readable behavior descriptions come from regular print() lines.
	if strings.HasPrefix(line, "STAT:") {
		statPart := strings.TrimPrefix(line, "STAT:")
		pairs := strings.Split(statPart, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(parts) == 2 {
				if v, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					progress.mu.Lock()
					if progress.BehaviorStats == nil {
						progress.BehaviorStats = make(map[string]int64)
					}
					progress.BehaviorStats[parts[0]] += v
					progress.LastOutput = time.Now()
					progress.mu.Unlock()
				}
			}
		}
		return
	}

	// Check for per-phase progress: PROGRESS:current/total
	if strings.HasPrefix(line, "PROGRESS:") {
		progPart := strings.TrimPrefix(line, "PROGRESS:")
		parts := strings.SplitN(progPart, "/", 2)
		curr, total := 0, 0
		if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			curr = v
		}
		if len(parts) == 2 {
			if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				total = v
			}
		}
		data, _ := json.Marshal(SSELine{
			Type:            "progress",
			Level:           "info",
			Message:         progPart,
			ProgressCurrent: curr,
			ProgressTotal:   total,
		})
		sseLine := fmt.Sprintf("data: %s\n\n", data)
		for _, wr := range w.writers {
			wr.Write([]byte(sseLine))
			if f, ok := wr.(http.Flusher); ok {
				f.Flush()
			}
		}
		progress.mu.Lock()
		progress.PhaseCurrent = curr
		progress.PhaseTotal = total
		progress.LastOutput = time.Now()
		// Also update per-phase progress
		if ps, ok := progress.ActivePhases[progress.Phase]; ok {
			ps.PhaseCurrent = curr
			ps.PhaseTotal = total
			ps.Current = curr
			ps.Total = total
		}
		progress.mu.Unlock()
		return
	}

	// Regular log line
	data, _ := json.Marshal(SSELine{Type: "log", Message: line, Level: w.level})
	sseLine := fmt.Sprintf("data: %s\n\n", data)
	for _, wr := range w.writers {
		wr.Write([]byte(sseLine))
		if f, ok := wr.(http.Flusher); ok {
			f.Flush()
		}
	}
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
			w.emitLine(line)
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

// PhaseState tracks per-phase running state for concurrent collection.
type PhaseState struct {
	Running      bool      `json:"running"`
	Phase        string    `json:"phase"`
	Current      int       `json:"current"`
	Total        int       `json:"total"`
	Started      time.Time `json:"started"`
	PhaseCurrent int       `json:"phaseCurrent"`
	PhaseTotal   int       `json:"phaseTotal"`
}
type CollectionProgress struct {
	mu         sync.RWMutex
	Running    bool          `json:"running"`
	ExtraArgs  []string      `json:"extraArgs,omitempty"`
	Phase      string        `json:"phase"`
	Current    int           `json:"current"`
	Total      int           `json:"total"`
	Message    string        `json:"message"`
	Results    []PhaseResult `json:"results"`
	Started    time.Time     `json:"started"`
	Finished   *time.Time    `json:"finished"`
	LastRun    interface{}   `json:"lastRun"`
	Errors     []string      `json:"errors"`
	LastOutput time.Time     `json:"-"`
	// Per-phase sub-progress
	PhaseCurrent int                    `json:"phaseCurrent"`
	PhaseTotal   int                    `json:"phaseTotal"`
	ActivePhases map[string]*PhaseState `json:"activePhases"` // per-phase concurrency tracking
	// Accumulated behavior stats across all phases
	BehaviorStats map[string]int64 `json:"behaviorStats"`
}

var (
	progress     = &CollectionProgress{ActivePhases: make(map[string]*PhaseState)}
	activeWriter *sseWriter
	writerMu     sync.Mutex
)

func GetProgress() *CollectionProgress {
	progress.mu.RLock()
	// Auto-reset per-phase: any phase stuck >15 min without output gets cleared
	hasRunning := false
	for name, ps := range progress.ActivePhases {
		if ps.Running && !progress.LastOutput.IsZero() && time.Since(progress.LastOutput) > 15*time.Minute {
			progress.mu.RUnlock()
			progress.mu.Lock()
			if p, ok := progress.ActivePhases[name]; ok && p.Running {
				p.Running = false
				log.Printf("[collector] auto-reset stuck phase %s (no output for 15+ min)", name)
			}
			progress.mu.Unlock()
			progress.mu.RLock()
		}
		if ps.Running {
			hasRunning = true
		}
	}
	progress.Running = hasRunning
	if !hasRunning {
		progress.Phase = "done"
	}
	cp := CollectionProgress{
		Running:       progress.Running,
		ExtraArgs:     append([]string(nil), progress.ExtraArgs...),
		Phase:         progress.Phase,
		Current:       progress.Current,
		Total:         progress.Total,
		Message:       progress.Message,
		Started:       progress.Started,
		Finished:      progress.Finished,
		LastRun:       progress.LastRun,
		LastOutput:    progress.LastOutput,
		PhaseCurrent:  progress.PhaseCurrent,
		PhaseTotal:    progress.PhaseTotal,
		BehaviorStats: make(map[string]int64, len(progress.BehaviorStats)),
	}
	cp.Results = make([]PhaseResult, len(progress.Results))
	copy(cp.Results, progress.Results)
	cp.Errors = make([]string, len(progress.Errors))
	copy(cp.Errors, progress.Errors)
	cp.ActivePhases = make(map[string]*PhaseState, len(progress.ActivePhases))
	for k, v := range progress.ActivePhases {
		cp.ActivePhases[k] = &PhaseState{Running: v.Running, Phase: v.Phase, Current: v.Current, Total: v.Total, Started: v.Started, PhaseCurrent: v.PhaseCurrent, PhaseTotal: v.PhaseTotal}
	}
	for k, v := range progress.BehaviorStats {
		cp.BehaviorStats[k] = v
	}
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

	// Capture Python output for logging when no SSE connection is active
	var logBuf strings.Builder
	hasSSE := false
	writerMu.Lock()
	if activeWriter == nil {
		activeWriter = &sseWriter{level: "info"}
	} else {
		hasSSE = true
	}
	aw := activeWriter // shared reference for both stdout and stderr
	writerMu.Unlock()

	// Tee output: always write to sseWriter, also capture for server log
	teeWriter := io.MultiWriter(aw, &logBuf)
	cmd.Stdout = teeWriter
	cmd.Stderr = teeWriter
	err := cmd.Run()
	// Flush remaining data through the shared writer
	aw.flushRemaining()

	// When running as background task (no SSE), log Python output to Go log
	if !hasSSE && logBuf.Len() > 0 {
		log.Printf("[collector:%s] Python output:\n%s", script, logBuf.String())
	}
	if err != nil {
		log.Printf("[collector:%s] Python error: %v", script, err)
	}
	return err
}

func runQuotePhase() PhaseResult {
	setPhase("quote", "实时行情监控...")
	sseSend(SSELine{Type: "phase", Phase: "quote", Message: "开始实时行情监控 (自选+持仓+榜单)...", Level: "info"})
	t0 := time.Now()

	err := runPythonStreamWithArgs("realtime_quotes.py", "--all")

	var count int64
	db.PG.Model(&model.StockRealtimeQuote{}).Count(&count)
	phaseRes := PhaseResult{
		Phase:      "quote",
		Total:      int(count),
		New:        int(count),
		DurationMs: time.Since(t0).Milliseconds(),
	}
	if err != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "quote", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("实时行情失败: %v", err)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "quote", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("实时行情: %d 只已更新", count)})
	}
	return phaseRes
}

// RunManualCollection runs specified phases. Phases must be non-empty.
func RunManualCollection(phases []string, extraArgs ...string) error {
	progress.mu.Lock()
	if len(phases) == 0 {
		progress.mu.Unlock()
		return fmt.Errorf("采集任务列表为空")
	}

	// Per-phase lock: reject if ANY requested phase is already running
	var conflicts []string
	for _, p := range phases {
		if ps, ok := progress.ActivePhases[p]; ok && ps.Running {
			conflicts = append(conflicts, p)
		}
	}
	if len(conflicts) > 0 {
		progress.mu.Unlock()
		return fmt.Errorf("采集任务 [%s] 正在执行中，请等待完成", strings.Join(conflicts, ", "))
	}

	// Mark all requested phases as running
	now := time.Now()
	progress.Running = true
	progress.LastOutput = time.Now()
	if len(extraArgs) > 0 {
		progress.ExtraArgs = extraArgs
	}
	progress.Phase = "starting"
	progress.Current = 0
	totalPhases := len(phases)
	progress.Total = totalPhases
	if len(progress.ActivePhases) <= len(phases) {
		progress.Results = nil
		progress.Errors = nil
		progress.Started = now
	}
	for _, p := range phases {
		progress.ActivePhases[p] = &PhaseState{Running: true, Phase: p, Started: now}
	}
	progress.Phase = phases[0]
	progress.LastRun = progress.Started
	progress.Finished = nil
	progress.PhaseCurrent = 0
	progress.PhaseTotal = 0
	if progress.BehaviorStats == nil {
		progress.BehaviorStats = make(map[string]int64)
	}
	progress.mu.Unlock()

	logEntry := model.CollectionLog{
		Status:    "running",
		StartedAt: time.Now(),
	}
	db.MySQL.Create(&logEntry)

	defer func() {
		now := time.Now()
		progress.mu.Lock()
		// Clear only our phases; other concurrent phases stay running
		for _, p := range phases {
			delete(progress.ActivePhases, p)
		}
		if len(progress.ActivePhases) == 0 {
			progress.Running = false
			progress.Phase = "done"
		} else {
			progress.Running = true
			// Keep Phase as first remaining active phase
			for _, ps := range progress.ActivePhases {
				progress.Phase = ps.Phase
				break
			}
		}
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
		statsJSON, _ := json.Marshal(progress.BehaviorStats)
		db.MySQL.Model(&logEntry).Updates(map[string]interface{}{
			"phases":         string(phasesJSON),
			"total_new":      totalNew,
			"total_skipped":  totalSkipped,
			"total_errors":   totalErrors,
			"status":         status,
			"duration_ms":    durationMs,
			"finished_at":    now,
			"behavior_stats": string(statsJSON),
		})
		sseSend(SSELine{Type: "done", Phase: "done", Level: "success",
			Stats:   progress.BehaviorStats,
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
	if shouldRun("kline_youzi") {
		appendResult(runKLineYouziPhase())
	}
	if shouldRun("tushare_kline") {
		appendResult(runTushareKLinePhase())
	}
	if shouldRun("tushare_indicator") {
		appendResult(runTushareIndicatorPhase())
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
	if shouldRun("concept") {
		appendResult(runConceptPhase())
	}
	if shouldRun("concept_full") {
		appendResult(runConceptFullPhase())
	}
	if shouldRun("market_daily_agg") {
		appendResult(runMarketDailyAggPhase())
	}
	if shouldRun("market_sentiment") {
		appendResult(runMarketSentimentPhase())
	}
	if shouldRun("market_style") {
		appendResult(runMarketStylePhase())
	}
	if shouldRun("profile") {
		appendResult(runProfilePhase())
	}
	if shouldRun("score") {
		appendResult(runScorePhase())
	}
	if shouldRun("quote") {
		appendResult(runQuotePhase())
	}
	if shouldRun("dragon_tiger") {
		appendResult(runDragonTigerPhase())
	}
	if shouldRun("margin") {
		appendResult(runMarginPhase())
	}
	if shouldRun("block_trade") {
		appendResult(runBlockTradePhase())
	}
	if shouldRun("unlock") {
		appendResult(runUnlockPhase())
	}
	if shouldRun("ths_hot") {
		appendResult(runThsHotPhase())
	}
	if shouldRun("dividend") {
		appendResult(runDividendPhase())
	}
	if shouldRun("ths_eps") {
		appendResult(runThsEpsPhase())
	}
	if shouldRun("cninfo") {
		appendResult(runCninfoPhase())
	}
	if shouldRun("macro_news") {
		appendResult(runMacroNewsPhase())
	}
	if shouldRun("fund_flow") {
		appendResult(runFundFlowPhase())
	}
	if shouldRun("northbound") {
		appendResult(runNorthboundPhase())
	}
	if shouldRun("limit_stats") {
		appendResult(runLimitStatsPhase())
	}
	return nil
}

// runStatsPhase runs a Python script and returns PhaseResult populated from STAT: output.
// STAT:records_new=X,records_skip=Y,records_err=Z lines from the script drive the counts.
func runStatsPhase(phase, label, script string, args ...string) PhaseResult {
	setPhase(phase, label+"...")
	sseSend(SSELine{Type: "phase", Phase: phase, Message: "开始采集" + label + "...", Level: "info"})

	// Snapshot BehaviorStats before
	progress.mu.RLock()
	snapBefore := make(map[string]int64)
	for k, v := range progress.BehaviorStats {
		snapBefore[k] = v
	}
	progress.mu.RUnlock()

	t0 := time.Now()
	var pyErr error
	if len(args) > 0 {
		pyErr = runPythonStreamWithArgs(script, args...)
	} else {
		pyErr = runPythonStream(script)
	}

	// Read STAT results
	progress.mu.RLock()
	newRec := progress.BehaviorStats["records_new"] - snapBefore["records_new"]
	skipRec := progress.BehaviorStats["records_skip"] - snapBefore["records_skip"]
	errRec := progress.BehaviorStats["records_err"] - snapBefore["records_err"]
	progress.mu.RUnlock()

	// Fallback: if STAT not emitted AND Python failed, report error
	if newRec == 0 && skipRec == 0 && errRec == 0 {
		if pyErr != nil {
			errRec = 1
			log.Printf("[collector:%s] %s Python failed: %v", script, label, pyErr)
		} else {
			newRec = 1 // at least signal completion
		}
	}

	phaseRes := PhaseResult{
		Phase:      phase,
		New:        int(newRec),
		Skipped:    int(skipRec),
		Errors:     int(errRec),
		DurationMs: time.Since(t0).Milliseconds(),
	}
	sseSend(SSELine{Type: "result", Phase: phase, Result: &phaseRes, Level: ternary(errRec > 0, "error", "success"),
		Message: fmt.Sprintf("%s: 新增 %d, 跳过 %d, 错误 %d", label, newRec, skipRec, errRec)})
	return phaseRes
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// ── 新增数据采集 phase ──────────────────────────────────────

func runDragonTigerPhase() PhaseResult {
	return runStatsPhase("dragon_tiger", "龙虎榜", "collect_dragon_tiger.py")
}

func runMarginPhase() PhaseResult {
	return runStatsPhase("margin", "融资融券", "collect_margin.py")
}

func runBlockTradePhase() PhaseResult {
	return runStatsPhase("block_trade", "大宗交易", "collect_block_trade.py")
}

func runUnlockPhase() PhaseResult {
	return runStatsPhase("unlock", "限售解禁", "collect_unlock.py")
}

func runThsHotPhase() PhaseResult {
	return runStatsPhase("ths_hot", "同花顺热点", "collect_ths_hot.py")
}

func runDividendPhase() PhaseResult {
	return runStatsPhase("dividend", "分红送转", "collect_dividend.py")
}

func runThsEpsPhase() PhaseResult {
	return runStatsPhase("ths_eps", "一致预期EPS", "collect_ths_eps.py")
}

func runCninfoPhase() PhaseResult {
	return runStatsPhase("cninfo", "巨潮公告", "collect_cninfo.py")
}

func runFundFlowPhase() PhaseResult {
	return runStatsPhase("fund_flow", "资金流向", "collect_fund_flow.py")
}

func runMacroNewsPhase() PhaseResult {
	return runStatsPhase("macro_news", "宏观资讯", "collect_macro_news.py")
}

func runScorePhase() PhaseResult {
	setPhase("score", "AI 六维评分...")
	sseSend(SSELine{Type: "phase", Phase: "score", Message: "开始 AI 六维评分采集...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.AIStockScore{}).Where("DATE(analyzed_at) = CURRENT_DATE").Count(&before)
	err := runPythonStreamWithArgs("stock_score_collect.py", "--batch")
	phaseRes := PhaseResult{Phase: "score", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.AIStockScore{}).Where("DATE(analyzed_at) = CURRENT_DATE").Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if err != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "score", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("六维评分失败: %v", err)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "score", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("六维评分: 新增 %d 条", after-before)})
	}
	return phaseRes
}

func runProfilePhase() PhaseResult {
	setPhase("profile", "AI 股票简介+六维评分...")
	sseSend(SSELine{Type: "phase", Phase: "profile", Message: "开始 AI 股票简介+六维评分采集...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockProfile{}).Count(&before)
	err := runPythonStreamWithArgs("stock_profile_collect.py", "--batch")
	phaseRes := PhaseResult{Phase: "profile", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockProfile{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if err != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "profile", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("AI简介失败: %v", err)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "profile", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("AI简介: 新增 %d 份", after-before)})
	}
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

func runConceptFullPhase() PhaseResult {
	setPhase("concept_full", "概念板块全量重建...")
	sseSend(SSELine{Type: "phase", Phase: "concept_full", Message: "开始全量重建概念板块数据（股票→概念反向采集,预计30分钟）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockConcept{}).Where("concept_type = ?", "concept").Count(&before)
	if err := CollectConceptsFull(); err != nil {
		phaseRes := PhaseResult{Phase: "concept_full", Errors: 1}
		phaseRes.DurationMs = time.Since(t0).Milliseconds()
		sseSend(SSELine{Type: "result", Phase: "concept_full", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("全量重建失败: %v", err)})
		return phaseRes
	}
	var after int64
	db.PG.Model(&model.StockConcept{}).Where("concept_type = ?", "concept").Count(&after)
	phaseRes := PhaseResult{Phase: "concept_full", Total: int(after), New: int(after - before), Skipped: int(before), DurationMs: time.Since(t0).Milliseconds()}
	sseSend(SSELine{Type: "result", Phase: "concept_full", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("全量重建: %d 条概念关联", after)})
	return phaseRes
}

func runFullSyncPhase() PhaseResult {
	setPhase("full_sync", "同步A股股票列表...")
	sseSend(SSELine{Type: "phase", Phase: "full_sync", Message: "开始同步A股股票列表...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockBasic{}).Count(&before)
	errFullSync := runPythonStream("full_sync.py")
	phaseRes := PhaseResult{Phase: "full_sync", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockBasic{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errFullSync != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "full_sync", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("列表同步失败: %v", errFullSync)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "full_sync", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("列表同步: %d 只", after)})
	}
	return phaseRes
}

func runKLinePhase() PhaseResult {
	setPhase("kline", "采集日K线+大盘指数...")
	sseSend(SSELine{Type: "phase", Phase: "kline", Message: "开始采集日K线数据（个股+大盘指数+国债ETF）...", Level: "info"})
	t0 := time.Now()
	var totalStocks int64
	db.PG.Model(&model.StockBasic{}).Count(&totalStocks)
	var stocksWithK int64
	if err := db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k WHERE code NOT LIKE 'IDX%%' AND code NOT IN ('511010','511090','511520')").Scan(&stocksWithK).Error; err != nil {
		log.Printf("[collector] stocksWithK query failed: %v", err)
	}

	// Always run K-line collection — script handles incremental per-stock,
	// fetching recent trading days and filling gaps via ON CONFLICT DO UPDATE.
	indexCollected := false
	sseSend(SSELine{Type: "log", Message: fmt.Sprintf("开始采集个股K线 (%d 只已有数据)", stocksWithK), Level: "info"})
	runPythonStreamWithArgs("batch_collect.py", "--last", "10")

	// Always collect index + bond ETF K-line (5 indices + 3 bonds, very fast)
	sseSend(SSELine{Type: "log", Message: "采集大盘指数+国债ETF K线...", Level: "info"})
	indexT0 := time.Now()
	if err := runPythonStream("collect_index_kline.py"); err != nil {
		log.Printf("[collector] index kline collection failed: %v", err)
	} else {
		indexCollected = true
		sseSend(SSELine{Type: "log", Message: fmt.Sprintf("大盘指数+国债ETF K线更新完成 (%.1fs)", time.Since(indexT0).Seconds()), Level: "info"})
	}

	var after int64
	if err := db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k").Scan(&after).Error; err != nil {
		log.Printf("[collector] after count query failed: %v", err)
	}
	phaseRes := PhaseResult{Phase: "kline", Total: int(after), New: int(after - stocksWithK), Skipped: int(stocksWithK), DurationMs: time.Since(t0).Milliseconds()}
	idxMsg := ""
	if indexCollected {
		idxMsg = " +大盘指数"
	}
	sseSend(SSELine{Type: "result", Phase: "kline", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("K线: %d 只%s", after, idxMsg)})
	return phaseRes
}

func runTushareKLinePhase() PhaseResult {
	setPhase("tushare_kline", "Tushare日K采集...")
	sseSend(SSELine{Type: "phase", Phase: "tushare_kline", Message: "开始从 Tushare API 采集全市场日K线（支持指定日期）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k WHERE data_source = 'tushare'").Scan(&before)
	errTK := runPythonStream("tushare_kline.py")
	phaseRes := PhaseResult{Phase: "tushare_kline", Skipped: int(before)}
	var after int64
	db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k WHERE data_source = 'tushare'").Scan(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errTK != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "tushare_kline", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("Tushare日K采集失败: %v", errTK)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "tushare_kline", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("Tushare日K: %d 只", after)})
	}
	return phaseRes
}

func runTushareIndicatorPhase() PhaseResult {
	setPhase("tushare_indicator", "Tushare技术指标采集...")
	sseSend(SSELine{Type: "phase", Phase: "tushare_indicator", Message: "开始从 Tushare daily_basic 采集技术指标（PE/PB/PS/股息率/换手率/股本/市值）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_indicator WHERE data_source = 'tushare'").Scan(&before)
	errTI := runPythonStream("tushare_indicator.py")
	phaseRes := PhaseResult{Phase: "tushare_indicator", Skipped: int(before)}
	var after int64
	db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_indicator WHERE data_source = 'tushare'").Scan(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errTI != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "tushare_indicator", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("Tushare技术指标采集失败: %v", errTI)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "tushare_indicator", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("Tushare技术指标: %d 只", after)})
	}
	return phaseRes
}

func runIndustryPhase() PhaseResult {
	setPhase("industry", "填充行业分类...")
	sseSend(SSELine{Type: "phase", Phase: "industry", Message: "开始填充行业分类...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockBasic{}).Where("industry IS NOT NULL AND industry != ''").Count(&before)
	errIndu := runPythonStream("populate_industry.py")
	phaseRes := PhaseResult{Phase: "industry", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockBasic{}).Where("industry IS NOT NULL AND industry != ''").Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errIndu != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "industry", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("行业分类失败: %v", errIndu)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "industry", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("行业: 新增 %d", after-before)})
	}
	return phaseRes
}

func runShareholderPhase() PhaseResult {
	setPhase("shareholder", "采集股东户数...")
	sseSend(SSELine{Type: "phase", Phase: "shareholder", Message: "开始采集股东户数...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockShareholder{}).Count(&before)
	errSH := runPythonStream("shareholder_collect.py")
	phaseRes := PhaseResult{Phase: "shareholder", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockShareholder{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errSH != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "shareholder", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("股东采集失败: %v", errSH)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "shareholder", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("股东: 新增 %d", after-before)})
	}
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
	errFin := runPythonStream("financial_collect.py")
	phaseRes := PhaseResult{Phase: "financial", Skipped: int(existing)}
	var after int64
	if err := db.PG.Model(&model.StockFinancial{}).Select("COUNT(DISTINCT code)").Scan(&after).Error; err != nil {
		log.Printf("[collector] financial after query failed: %v", err)
	}
	phaseRes.Total = int(after)
	phaseRes.New = int(after - existing)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errFin != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "financial", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("财务采集失败: %v", errFin)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "financial", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("财务: %d 只", after)})
	}
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
	errNews := runPythonStream("news_collect.py")
	phaseRes := PhaseResult{Phase: "news", Skipped: int(existing)}
	var after int64
	if err := db.PG.Model(&model.StockNews{}).Select("COUNT(DISTINCT code)").Scan(&after).Error; err != nil {
		log.Printf("[collector] news after query failed: %v", err)
	}
	phaseRes.Total = int(after)
	phaseRes.New = int(after - existing)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errNews != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "news", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("资讯采集失败: %v", errNews)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "news", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("资讯: %d 只", after)})
	}
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
	case "dragon_tiger":
		return runPythonStreamWithArgs("collect_dragon_tiger.py", time.Now().Format("2006-01-02"))
	case "block_trade":
		return runPythonStreamWithArgs("collect_block_trade.py", code)
	case "announcements":
		return runPythonStreamWithArgs("collect_cninfo.py", code)
	case "unlocks":
		return runPythonStreamWithArgs("collect_unlock.py", code)
	case "fund_flow":
		if code != "" {
			return runPythonStreamWithArgs("collect_fund_flow.py", code)
		}
		return runPythonStreamWithArgs("collect_fund_flow.py", "--last", "30")
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}
}

func setPhase(phase, msg string) {
	progress.mu.Lock()
	progress.Phase = phase
	progress.Message = msg
	if ps, ok := progress.ActivePhases[phase]; ok {
		ps.Phase = phase
	}
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

func runMarketDailyAggPhase() PhaseResult {
	setPhase("market_daily_agg", "市场日聚合...")
	sseSend(SSELine{Type: "phase", Phase: "market_daily_agg", Message: "开始计算市场日聚合数据（涨跌比、成交额、MA20站上数等）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Raw("SELECT COUNT(*) FROM market_daily_agg").Scan(&before)
	errMDA := runPythonWithRepair("precompute_aggs.py", getExtraArgsOrDefault("--last", "60")...)
	phaseRes := PhaseResult{Phase: "market_daily_agg", Skipped: int(before)}
	var after int64
	db.PG.Raw("SELECT COUNT(*) FROM market_daily_agg").Scan(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errMDA != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "market_daily_agg", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("市场日聚合失败: %v", errMDA)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "market_daily_agg", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("市场日聚合: %d 天 (%+d)", after, after-before)})
	}
	return phaseRes
}

func runMarketSentimentPhase() PhaseResult {
	setPhase("market_sentiment", "市场情绪计算...")
	sseSend(SSELine{Type: "phase", Phase: "market_sentiment", Message: "开始计算市场情绪指数（11项子指标）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Raw("SELECT COUNT(*) FROM market_sentiment").Scan(&before)
	err := runPythonWithRepair("compute_sentiment.py", getExtraArgsOrDefault("--last", "60")...)
	phaseRes := PhaseResult{Phase: "market_sentiment", Skipped: int(before)}
	var after int64
	db.PG.Raw("SELECT COUNT(*) FROM market_sentiment").Scan(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if err != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "market_sentiment", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("市场情绪计算失败: %v", err)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "market_sentiment", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("市场情绪: %d 天 (%+d)", after, after-before)})
	}
	return phaseRes
}

// runPythonWithRepair detects repair mode (--repair --from X --to Y) and runs
// the script once per trading date in range. Falls back to default args otherwise.
func runPythonWithRepair(script string, defaultArgs ...string) error {
	args := getExtraArgs()
	if len(args) >= 5 && args[0] == "--repair" && args[1] == "--from" && args[3] == "--to" {
		from := args[2]
		to := args[4]
		var dates []string
		db.PG.Raw(`SELECT DISTINCT trade_date::text FROM stocks_daily_k
			WHERE trade_date >= ? AND trade_date <= ?
			ORDER BY trade_date`, from, to).Pluck("trade_date", &dates)
		if len(dates) == 0 {
			return fmt.Errorf("repair: no trading dates in %s ~ %s", from, to)
		}
		log.Printf("[collector] repair %s: %d dates (%s ~ %s)", script, len(dates), from, to)
		for i, date := range dates {
			log.Printf("[collector] repair %s [%d/%d] %s", script, i+1, len(dates), date)
			if err := runPythonStreamWithArgs(script, date); err != nil {
				return fmt.Errorf("repair %s date=%s: %w", script, date, err)
			}
		}
		return nil
	}
	return runPythonStreamWithArgs(script, defaultArgs...)
}

func getExtraArgs() []string {
	progress.mu.RLock()
	defer progress.mu.RUnlock()
	return progress.ExtraArgs
}

// getExtraArgsOrDefault returns stored extra args, or the provided defaults if none.
func getExtraArgsOrDefault(defaults ...string) []string {
	args := getExtraArgs()
	if len(args) > 0 {
		return args
	}
	return defaults
}

func runBackfillFinancialPhase() PhaseResult {
	setPhase("backfill_financial", "财报数据全量回填...")
	sseSend(SSELine{Type: "phase", Phase: "backfill_financial", Message: "开始全量回填财报数据（利润表+资产负债表）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockFinancial{}).Count(&before)
	errBF := runPythonStream("backfill_financial.py")
	phaseRes := PhaseResult{Phase: "backfill_financial", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockFinancial{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errBF != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "backfill_financial", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("财报回填失败: %v", errBF)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "backfill_financial", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("财报回填: +%d 条", after-before)})
	}
	return phaseRes
}

func runBackfillShareholderPhase() PhaseResult {
	setPhase("backfill_shareholder", "股东数据全量回填...")
	sseSend(SSELine{Type: "phase", Phase: "backfill_shareholder", Message: "开始全量回填股东数据（股东户数+十大股东）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Model(&model.StockShareholder{}).Count(&before)
	errBS := runPythonStream("backfill_shareholder.py")
	phaseRes := PhaseResult{Phase: "backfill_shareholder", Skipped: int(before)}
	var after int64
	db.PG.Model(&model.StockShareholder{}).Count(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errBS != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "backfill_shareholder", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("股东回填失败: %v", errBS)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "backfill_shareholder", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("股东回填: +%d 条", after-before)})
	}
	return phaseRes
}
func runKLineYouziPhase() PhaseResult {
	setPhase("kline_youzi", "柚子K线采集...")
	sseSend(SSELine{Type: "phase", Phase: "kline_youzi", Message: "开始从柚子大数据API采集全市日K线（含北交所）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k WHERE data_source = 'youzi'").Scan(&before)
	errKY := runPythonStream("collect_kline_youzi.py")
	phaseRes := PhaseResult{Phase: "kline_youzi"}
	var after int64
	db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k WHERE data_source = 'youzi'").Scan(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if errKY != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "kline_youzi", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("柚子K线采集失败: %v", errKY)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "kline_youzi", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("柚子K线: %d 只", after)})
	}
	return phaseRes
}

// RepairStock runs the repair_kline.py script to delete + refetch + recalc all data for a stock.

// ComputeAllIndicatorsBatch computes all 84 technical indicators for all stocks
// and writes to the stock_daily_indicators JSONB cache table.
func ComputeAllIndicatorsBatch(days int) error {
	return runPythonStreamWithArgs("compute_all_indicators.py", "--days", fmt.Sprintf("%d", days))
}

func RepairStock(code string) error {
	if err := runPythonStreamWithArgs("repair_kline.py", code); err != nil {
		return err
	}
	// Also compute indicators after K-line repair
	return runPythonStreamWithArgs("compute_all_indicators.py", "--code", code, "--days", "250")
}

func runNorthboundPhase() PhaseResult {
	return runStatsPhase("northbound", "北向资金", "collect_northbound.py")
}

// runMarketStylePhase computes market style for today via internal API call.
func runMarketStylePhase() PhaseResult {
	setPhase("market_style", "市场风格计算...")
	sseSend(SSELine{Type: "phase", Phase: "market_style", Message: "开始计算市场风格（趋势/波动/结构）...", Level: "info"})
	t0 := time.Now()
	var before int64
	db.PG.Raw("SELECT COUNT(*) FROM market_style_daily").Scan(&before)
	err := runPythonWithRepair("compute_market_style.py")
	phaseRes := PhaseResult{Phase: "market_style", Skipped: int(before)}
	var after int64
	db.PG.Raw("SELECT COUNT(*) FROM market_style_daily").Scan(&after)
	phaseRes.Total = int(after)
	phaseRes.New = int(after - before)
	phaseRes.DurationMs = time.Since(t0).Milliseconds()
	if err != nil {
		phaseRes.Errors = 1
		sseSend(SSELine{Type: "result", Phase: "market_style", Result: &phaseRes, Level: "error", Message: fmt.Sprintf("市场风格计算失败: %v", err)})
	} else {
		sseSend(SSELine{Type: "result", Phase: "market_style", Result: &phaseRes, Level: "success", Message: fmt.Sprintf("市场风格: %d 天 (%+d)", after, after-before)})
	}
	return phaseRes
}

func runLimitStatsPhase() PhaseResult {
	return runStatsPhase("limit_stats", "涨跌停统计", "collect_limit_stats.py")
}

// ── Exported wrappers for external callers (handler, service) ──

// SetPhase sets the current collection phase.
func SetPhase(phase, msg string) { setPhase(phase, msg) }

// SSESend sends an SSE line.
func SSESend(line SSELine) { sseSend(line) }

// AppendResult appends a phase result.
func AppendResult(r PhaseResult) { appendResult(r) }
