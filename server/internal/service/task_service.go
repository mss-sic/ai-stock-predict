package service

// DEPRECATED: TaskManager cron scheduling is disabled.
// All scheduling is now handled by scheduler/v2 UnifiedScheduler.
// The ScheduledTask model and TaskHandler API remain for backward compatibility.
// Manual triggers via RunTaskNow still work by calling collector directly.

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/robfig/cron/v3"
)

// Default task definitions
var DefaultTasks = []struct {
	Name     string
	Phase    string
	CronExpr string
}{
	{"概念板块采集", "concept", "0 0 8 * * *"},        // daily 08:00
	{"概念全量重建", "concept_full", "0 0 6 * * 0"},        // weekly Sun 06:00
	{"实时行情监控", "quote", "0 */5 9-15 * * 1-5"},  // every 5 min during trading hours Mon-Fri
	{"日K线采集", "kline", "0 */30 9-16 * * 1-5"},    // every 30min on weekdays 9:00-16:30
	{"Tushare日K采集", "tushare_kline", "0 10 16 * * 1-5"},  // 交易日 16:10 盘后
	{"Tushare技术指标采集", "tushare_indicator", "0 0 16-20 * * 1-5"},  // 交易日 16:00-20:00 每小时
	{"行业分类采集", "industry", "0 0 2 * * 1"},         // weekly Mon 2:00
	{"股票列表同步", "full_sync", "0 0 3 * * 1"},        // weekly Mon 3:00
	{"股东数据采集", "shareholder", "0 0 17 * * *"},     // daily 17:00
	{"财务数据采集", "financial", "0 0 4 * * 0"},        // weekly Sun 4:00
	{"资讯数据采集", "news", "0 */30 * * * *"},           // every 30 min
	{"研报数据采集", "reports", "0 0 18 * * *"},          // daily 18:00
	{"财报全量回填", "backfill_financial", "0 0 3 1 * *"}, // monthly 1st
	{"股东全量回填", "backfill_shareholder", "0 0 4 1 * *"}, // monthly 1st
	{"风险扫描", "risk_scan", "0 5 * * * *"},             // hourly at :05
{"市场日聚合", "market_daily_agg", "40 10 16 * * *"},     // daily 16:10 (after K-line completes)
{"市场情绪计算", "market_sentiment", "10 0 17 * * *"},   // daily 17:00
{"市场风格计算", "market_style", "0 10,40 17 * * 1-5"},  // 交易日 17:10 + 17:40 盘后多次更新
{"AI评分更新", "ai_score", "0 0 20 * * 0"},
	{"龙虎榜采集", "dragon_tiger", "0 0 17 * * *"},        // daily 17:00 (盘后龙虎榜更新)
	{"融资融券采集", "margin", "0 0 9 * * *"},             // daily 09:00 (盘前更新)
	{"大宗交易采集", "block_trade", "0 0 18 * * *"},       // daily 18:00
	{"解禁数据采集", "unlock", "0 0 7 * * 1"},             // weekly Mon 07:00
	{"同花顺热点采集", "ths_hot", "0 0 16 * * *"},          // daily 16:00 (收盘后)
	{"分红数据采集", "dividend", "0 0 5 * * 1"},            // weekly Mon 05:00
	{"一致预期采集", "ths_eps", "0 0 6 * * 1"},             // weekly Mon 06:00
	{"巨潮公告采集", "cninfo", "0 0 8 * * *"},              // daily 08:00
	{"宏观资讯采集", "macro_news", "0 */30 * * * *"},       // every 30 min (7×24滚动)
	{"北向资金采集", "northbound", "0 30 15 * * 1-5"},       // 交易日 15:30 盘后
	{"涨跌停统计更新", "limit_stats", "0 5 16 * * 1-5"},    // 交易日 16:05 (after K-line completes)             // weekly Sun 20:00

}

type TaskManager struct {
	cron  *cron.Cron
	mu    sync.Mutex
	jobs  map[uint]cron.EntryID // taskID -> cron entry ID
}

var taskManager *TaskManager
var taskExtraArgs sync.Map // taskID → []string

// InitTaskManager boots the legacy cron scheduler for scheduled_tasks.
// v2 UnifiedScheduler handles strategy_run tasks; legacy handles collector/data tasks.
func InitTaskManager() *TaskManager { return initTaskManagerX() }

func initTaskManagerX() *TaskManager {
	tm := &TaskManager{
		cron: cron.New(cron.WithSeconds()),
		jobs: make(map[uint]cron.EntryID),
	}
	taskManager = tm

	// Reset any tasks stuck in "running" state from previous crash
	db.MySQL.Model(&model.ScheduledTask{}).Where("last_status = ?", "running").
		Update("last_status", "unknown")
	// Also finalize stuck TaskLog entries
	db.MySQL.Model(&model.TaskLog{}).Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status": "unknown", "finished_at": time.Now(),
			"error_msg": "服务重启，任务中断",
		})

	// Load existing tasks from DB and schedule them
	var tasks []model.ScheduledTask
	db.MySQL.Find(&tasks)
	for i := range tasks {
		if tasks[i].Enabled {
			tm.ScheduleTask(&tasks[i])
		}
	}

	tm.cron.Start()
	log.Printf("[TaskManager] started with %d scheduled tasks", len(tm.jobs))
	return tm
}

func GetTaskManager() *TaskManager { return taskManager }

// ScheduleTask is deprecated. Cron scheduling handled by v2 UnifiedScheduler.
func (tm *TaskManager) ScheduleTask(task *model.ScheduledTask) {
	if tm == nil {
		return
	}
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Remove existing job if any
	if entryID, ok := tm.jobs[task.ID]; ok {
		tm.cron.Remove(entryID)
		delete(tm.jobs, task.ID)
	}

	if !task.Enabled {
		return
	}

	// Normalize cron: add seconds if missing
	expr := task.CronExpr
	parts := strings.Fields(expr)
	if len(parts) == 5 {
		expr = "0 " + expr
	}

	taskID := task.ID
	phase := task.Phase
	name := task.Name

	entryID, err := tm.cron.AddFunc(expr, func() {
		executeTaskStandalone(taskID, phase, name)
	})
	if err != nil {
		log.Printf("[TaskManager] failed to schedule %s: %v", name, err)
		return
	}

	tm.jobs[task.ID] = entryID

	// Calculate next run (P0-4: check cron entry exists)
	entry := tm.cron.Entry(entryID)
	nextRun := entry.Next
	if nextRun.IsZero() {
		// Cron entry.Next may be zero if computed asynchronously;
		// compute from the schedule directly
		if entry.Schedule != nil {
			nextRun = entry.Schedule.Next(time.Now())
		} else {
			nextRun = time.Now().Add(24 * time.Hour)
		}
	}
	db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", task.ID).
		Update("next_run", nextRun)

	log.Printf("[TaskManager] scheduled: %s (%s) next=%s", name, expr, nextRun.Format("01-02 15:04"))
}

func executeTaskStandalone(taskID uint, phase, name string) {
	// Panic recovery — also finalize TaskLog
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[TaskManager] PANIC in task %s (phase=%s): %v", name, phase, r)
			now := time.Now()
			db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", taskID).
				Updates(map[string]interface{}{"last_status": "failed", "last_run": now})
			// Finalize any running TaskLog for this task
			db.MySQL.Model(&model.TaskLog{}).Where("task_id = ? AND status = ?", taskID, "running").
				Updates(map[string]interface{}{
					"status": "failed", "finished_at": now,
					"error_msg": fmt.Sprintf("PANIC: %v", r),
				})
		}
	}()

	// Create log entry
	logEntry := model.TaskLog{
		TaskID:    taskID,
		TaskName:  name,
		Phase:     phase,
		Status:    "running",
		StartedAt: time.Now(),
	}
	db.MySQL.Create(&logEntry)

	// Update task status
	db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{"last_run": time.Now(), "last_status": "running"})

	if phase == "risk_scan" {
		count, err := ScanUserHoldings()
		finishTaskLog(&logEntry, count, 0, 0, err)
	} else if phase == "market_style" {
		// Market style computation — use latest trading day, not wall clock
		log.Printf("[market_style] ⏰ 定时任务触发")
		svc := NewMarketStyleService()
		var date string
		db.PG.Raw("SELECT trade_date::text FROM market_sentiment ORDER BY trade_date DESC LIMIT 1").Scan(&date)
		log.Printf("[market_style] 📊 market_sentiment最新日期: %q", date)
		if date == "" {
			db.PG.Raw("SELECT trade_date::text FROM market_daily_agg ORDER BY trade_date DESC LIMIT 1").Scan(&date)
			log.Printf("[market_style] ⚠️  market_sentiment 无数据，降级到 market_daily_agg: %q", date)
		}
		var latestStyleDate string
		db.PG.Raw("SELECT trade_date::text FROM market_style_daily ORDER BY trade_date DESC LIMIT 1").Scan(&latestStyleDate)
		log.Printf("[market_style] 📋 market_style_daily最新日期: %q", latestStyleDate)
		if date == "" {
			log.Printf("[market_style] ❌ 无可用交易日期，跳过本次")
			finishTaskLog(&logEntry, 0, 0, 0, fmt.Errorf("无可用交易日期"))
		} else if date == latestStyleDate {
			log.Printf("[market_style] ✅ 数据已是最新 (market_style_daily=%s = market_sentiment=%s)，跳过", latestStyleDate, date)
			finishTaskLog(&logEntry, 0, 0, 0, nil)
		} else {
			// Collect dates to compute: missing dates + always recompute latest
			var dates []string
			db.PG.Raw(`SELECT trade_date::text FROM market_sentiment
				WHERE trade_date NOT IN (SELECT trade_date FROM market_style_daily)
				ORDER BY trade_date`).Pluck("trade_date", &dates)
			// Always include the latest date for recompute (overwrite stale data)
			hasLatest := false
			for _, d := range dates {
				if d == date { hasLatest = true; break }
			}
			if !hasLatest && date != "" {
				dates = append(dates, date)
			}
			log.Printf("[market_style] 📝 待计算日期: %d 个 (最新=%s, 包含重新计算=%v)", len(dates), date, !hasLatest)
			if len(dates) == 0 {
				log.Printf("[market_style] ⚠️  无待计算日期")
				finishTaskLog(&logEntry, 0, 0, 0, fmt.Errorf("无可用日期"))
			} else {
				success, fail := 0, 0
				for _, d := range dates {
					log.Printf("[market_style] 🔄 计算 %s (%d/%d) | 最新数据日期=%s", d, success+fail+1, len(dates), date)
					if err := svc.ComputeAndStore(d); err != nil {
						fail++
						log.Printf("[market_style] scheduled FAIL %s: %v", d, err)
					} else {
						success++
					}
				}
				var taskErr error
				if fail > 0 {
					taskErr = fmt.Errorf("%d/%d 失败", fail, success+fail)
				}
				log.Printf("[market_style] 🏁 任务完成: 成功=%d 失败=%d 最新数据日期=%s", success, fail, date)
				finishTaskLog(&logEntry, success, fail, 0, taskErr)
			}
		}
	} else {
		// Run collector phase (Python scripts: kline, indicator, market_sentiment, market_daily_agg, etc.)
		phaseName := phase
		if phase == "ai_score" {
			phaseName = "score" // ai_score maps to score phase
		}
		phases := []string{phaseName}
		// Read extra args (range selection) if any
		var extraArgs []string
		if v, ok := taskExtraArgs.LoadAndDelete(taskID); ok {
			extraArgs = v.([]string)
		}
		collector.RunManualCollection(phases, extraArgs...)

		// Collect results
		prog := collector.GetProgress()
		totalNew := 0
		totalSkip := 0
		totalErr := 0
		errMsgs := []string{}
		for _, r := range prog.Results {
			totalNew += r.New
			totalSkip += r.Skipped
			totalErr += r.Errors
			if r.Errors > 0 {
				errMsgs = append(errMsgs, fmt.Sprintf("[%s] errors=%d", r.Phase, r.Errors))
			}
		}

		var err error
		if len(prog.Errors) > 0 {
			allErrs := append(prog.Errors, errMsgs...)
			err = fmt.Errorf("%s", strings.Join(allErrs, "; "))
		} else if len(errMsgs) > 0 {
			err = fmt.Errorf("%s", strings.Join(errMsgs, "; "))
		}

		finishTaskLog(&logEntry, totalNew, totalSkip, totalErr, err)
		if err != nil {
			db.MySQL.Model(&model.TaskLog{}).Where("id = ?", logEntry.ID).
				Update("error_msg", err.Error())
		}
	}

	db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"last_run":    logEntry.StartedAt,
			"last_status": logEntry.Status,
		})
	// Next run update: cron scheduling managed by v2 UnifiedScheduler

}

func finishTaskLog(logEntry *model.TaskLog, totalNew, totalSkip, totalErr int, err error) {
	now := time.Now()
	status := "success"
	if err != nil {
		status = "failed"
	}
	updates := map[string]interface{}{
		"finished_at": now,
		"total_new":   totalNew,
		"total_skip":  totalSkip,
		"total_err":   totalErr,
		"duration_ms": now.Sub(logEntry.StartedAt).Milliseconds(),
		"status":      status,
	}
	if err != nil {
		updates["error_msg"] = err.Error()
	}
	logEntry.Status = status
	db.MySQL.Model(&model.TaskLog{}).Where("id = ?", logEntry.ID).Updates(updates)
}

// InitializeDefaultTasks creates missing default tasks (checks by name, safe for incremental runs)
func InitializeDefaultTasks() (int, error) {
	created := 0
	for _, dt := range DefaultTasks {
		var existing model.ScheduledTask
		err := db.MySQL.Where("name = ?", dt.Name).First(&existing).Error
		if err == nil {
			// Already exists — update cron/phase if changed
			updated := false
			if existing.Phase != dt.Phase {
				existing.Phase = dt.Phase
				updated = true
			}
			if existing.CronExpr != dt.CronExpr {
				existing.CronExpr = dt.CronExpr
				updated = true
			}
			if updated {
				db.MySQL.Save(&existing)
				if tm := GetTaskManager(); tm != nil { tm.ScheduleTask(&existing) }
			}
			continue
		}
		task := model.ScheduledTask{
			Name:     dt.Name,
			Phase:    dt.Phase,
			CronExpr: dt.CronExpr,
			Enabled:  true,
		}
		if err := db.MySQL.Create(&task).Error; err != nil {
			log.Printf("[TaskManager] failed to create default task %s: %v", dt.Name, err)
			continue
		}
		created++
		if tm := GetTaskManager(); tm != nil { tm.ScheduleTask(&task) }
	}
	// Cleanup: disable tasks that are no longer in DefaultTasks
	defaultNames := make(map[string]bool)
	for _, dt := range DefaultTasks {
		defaultNames[dt.Name] = true
	}
	var allTasks []model.ScheduledTask
	db.MySQL.Find(&allTasks)
	for _, t := range allTasks {
		if !defaultNames[t.Name] && t.Enabled {
			db.MySQL.Model(&t).Update("enabled", false)
			// removed from schedule by disabling
			log.Printf("[TaskManager] disabled stale task: %s (phase=%s)", t.Name, t.Phase)
		}
	}

	log.Printf("[TaskManager] initialized %d default tasks", created)
	return created, nil
}

// ── CRUD helpers ──

func ListTasks() ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	err := db.MySQL.Order("id ASC").Find(&tasks).Error
	return tasks, err
}

func CreateTask(name, phase, cronExpr string, enabled bool) (*model.ScheduledTask, error) {
	task := model.ScheduledTask{
		Name:     name,
		Phase:    phase,
		CronExpr: cronExpr,
		Enabled:  enabled,
	}
	if err := db.MySQL.Create(&task).Error; err != nil {
		return nil, err
	}
	if enabled {
		if tm := GetTaskManager(); tm != nil { tm.ScheduleTask(&task) }
	}
	return &task, nil
}

func UpdateTask(id uint, name, phase, cronExpr string, enabled bool) (*model.ScheduledTask, error) {
	var task model.ScheduledTask
	if err := db.MySQL.First(&task, id).Error; err != nil {
		return nil, err
	}
	if name != "" {
		task.Name = name
	}
	if phase != "" {
		task.Phase = phase
	}
	if cronExpr != "" {
		task.CronExpr = cronExpr
	}
	task.Enabled = enabled
	if err := db.MySQL.Save(&task).Error; err != nil {
		return nil, err
	}
	if tm := GetTaskManager(); tm != nil { tm.ScheduleTask(&task) }
	return &task, nil
}

func DeleteTask(id uint) error {
	tm := GetTaskManager()
	if tm != nil {
		tm.mu.Lock()
		if eid, ok := tm.jobs[id]; ok {
			tm.cron.Remove(eid)
			delete(tm.jobs, id)
		}
		tm.mu.Unlock()
	}
	return db.MySQL.Delete(&model.ScheduledTask{}, id).Error
}

func RunTaskNow(id uint, args []string) error {
	var task model.ScheduledTask
	if err := db.MySQL.First(&task, id).Error; err != nil {
		return err
	}
	if task.LastStatus == "running" {
		// Allow re-run if last run was more than 10 minutes ago (stuck)
		if task.LastRun != nil && time.Since(*task.LastRun) < 10*time.Minute {
			return fmt.Errorf("任务 %s 正在运行中，请等待完成", task.Name)
		}
	}
	// Mark as running immediately
	db.MySQL.Model(&task).Update("last_status", "running")
	if len(args) > 0 {
		taskExtraArgs.Store(id, args)
	}
	go executeTaskStandalone(task.ID, task.Phase, task.Name)
	return nil
}

// ResetTaskStatus clears a stuck "running" status
func ResetTaskStatus(id uint) error {
	return db.MySQL.Model(&model.ScheduledTask{}).Where("id = ? AND last_status = ?", id, "running").
		Update("last_status", "unknown").Error
}

func ListTaskLogs(taskID uint, limit int) ([]model.TaskLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var logs []model.TaskLog
	q := db.MySQL.Order("started_at DESC").Limit(limit)
	if taskID > 0 {
		q = q.Where("task_id = ?", taskID)
	}
	err := q.Find(&logs).Error
	return logs, err
}

// RepairTask runs a task in repair mode with optional date range.
func RepairTask(id uint, from, to string, all bool) error {
	var task model.ScheduledTask
	if err := db.MySQL.First(&task, id).Error; err != nil {
		return err
	}
	// For market_style, run bulk compute with support for --all / --from/--to
	if task.Phase == "market_style" {
		var dates []string
		if all {
			// --all: recompute all dates from market_sentiment
			if err := db.PG.Raw(`SELECT trade_date::text FROM market_sentiment ORDER BY trade_date`).
				Pluck("trade_date", &dates).Error; err != nil {
				return fmt.Errorf("查询全部日期失败: %w", err)
			}
			if len(dates) == 0 {
				return fmt.Errorf("market_sentiment 无数据，请先采集市场情绪")
			}
			log.Printf("[market_style] repair --all: %d dates (%s ~ %s)", len(dates), dates[0], dates[len(dates)-1])
		} else if from != "" && to != "" {
			// Date range: recompute dates in range
			if err := db.PG.Raw(`SELECT trade_date::text FROM market_sentiment
				WHERE trade_date >= ? AND trade_date <= ? ORDER BY trade_date`, from, to).
				Pluck("trade_date", &dates).Error; err != nil {
				return fmt.Errorf("查询日期范围失败: %w", err)
			}
			if len(dates) == 0 {
				return fmt.Errorf("%s ~ %s 范围内无 market_sentiment 数据", from, to)
			}
			log.Printf("[market_style] repair range: %d dates (%s ~ %s)", len(dates), dates[0], dates[len(dates)-1])
		} else {
			// Default: fill only missing dates
			db.PG.Exec(`DELETE FROM market_style_daily WHERE up_ratio = 0 AND total_amount = 0`)
			if err := db.PG.Raw(`SELECT trade_date::text FROM market_sentiment
				WHERE trade_date NOT IN (SELECT trade_date FROM market_style_daily)
				ORDER BY trade_date`).Pluck("trade_date", &dates).Error; err != nil {
				return fmt.Errorf("查询缺失日期失败: %w", err)
			}
			if len(dates) == 0 {
				return fmt.Errorf("没有缺失日期需要修复（market_style_daily 已是最新）")
			}
			log.Printf("[market_style] repair missing: %d dates", len(dates))
		}

		// Prevent re-entry: same task must run serially
		if task.LastStatus == "running" && task.LastRun != nil && time.Since(*task.LastRun) < 10*time.Minute {
			return fmt.Errorf("任务 %s 正在运行中，请等待完成", task.Name)
		}

		// Create TaskLog and mark as running so frontend can track progress
		now := time.Now()
		logEntry := model.TaskLog{
			TaskID:    task.ID,
			TaskName:  task.Name,
			Phase:     task.Phase,
			Status:    "running",
			StartedAt: now,
		}
		db.MySQL.Create(&logEntry)
		db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", task.ID).
			Updates(map[string]interface{}{"last_run": now, "last_status": "running"})

		svc := NewMarketStyleService()
		success, fail := 0, 0
		for i, date := range dates {
			log.Printf("[market_style] repair [%d/%d] %s", i+1, len(dates), date)
			if err := svc.ComputeAndStore(date); err != nil {
				fail++
				log.Printf("[market_style] ❌ 修复失败 %s: %v", date, err)
			} else {
				success++
			}
		}
		log.Printf("[market_style] repair done: %d dates, %d ok, %d fail", len(dates), success, fail)

		var repairErr error
		if fail > 0 {
			repairErr = fmt.Errorf("市场风格修复: %d/%d 成功, %d 失败", success, len(dates), fail)
		}
		finishTaskLog(&logEntry, success, 0, fail, repairErr)
		// Restore last_status so the task can be re-triggered later
		db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", task.ID).
			Updates(map[string]interface{}{"last_status": logEntry.Status})
		return repairErr
	}
	args := []string{"--repair"}
	if all {
		args = append(args, "--all")
	} else if from != "" && to != "" {
		args = append(args, "--from", from, "--to", to)
	}
	return RunTaskNow(id, args)
}

// ToggleTask toggles the enabled status of a task
func ToggleTask(id uint) (*model.ScheduledTask, error) {
	var task model.ScheduledTask
	if err := db.MySQL.First(&task, id).Error; err != nil {
		return nil, err
	}
	task.Enabled = !task.Enabled
	if err := db.MySQL.Save(&task).Error; err != nil {
		return nil, err
	}
	// Re-schedule: if disabled, ScheduleTask will remove from cron;
	// if enabled, it will add to cron. Running tasks won't be interrupted.
	if tm := GetTaskManager(); tm != nil { tm.ScheduleTask(&task) }
	return &task, nil
}
