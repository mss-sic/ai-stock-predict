package service

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
	{"日K线采集", "kline", "0 0 16 * * *"},             // daily 16:00
	{"PE/PB指标采集", "indicator", "0 30 16 * * *"},     // daily 16:30
	{"行业分类采集", "industry", "0 0 2 * * 1"},         // weekly Mon 2:00
	{"股票列表同步", "full_sync", "0 0 3 * * 1"},        // weekly Mon 3:00
	{"股东数据采集", "shareholder", "0 0 17 * * *"},     // daily 17:00
	{"财务数据采集", "financial", "0 0 4 * * 0"},        // weekly Sun 4:00
	{"资讯数据采集", "news", "0 */30 * * * *"},           // every 30 min
	{"研报数据采集", "reports", "0 0 18 * * *"},          // daily 18:00
	{"财报全量回填", "backfill_financial", "0 0 3 1 * *"}, // monthly 1st
	{"股东全量回填", "backfill_shareholder", "0 0 4 1 * *"}, // monthly 1st
	{"PE/PB历史回填", "backfill_indicator", "0 0 5 1 * *"}, // monthly 1st
	{"风险扫描", "risk_scan", "0 5 * * * *"},             // hourly at :05
{"市场日聚合", "market_daily_agg", "40 0 16 * * *"},     // daily 16:00
{"市场情绪计算", "market_sentiment", "10 0 17 * * *"},   // daily 17:00
{"市场风格计算", "market_style", "0 30 17 * * *"},       // daily 17:30
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
	{"涨跌停统计更新", "limit_stats", "0 0 16 * * 1-5"},    // 交易日 16:00 盘后             // weekly Sun 20:00

}

type TaskManager struct {
	cron  *cron.Cron
	mu    sync.Mutex
	jobs  map[uint]cron.EntryID // taskID -> cron entry ID
}

var taskManager *TaskManager
var taskExtraArgs sync.Map // taskID → []string

func InitTaskManager() *TaskManager {
	tm := &TaskManager{
		cron: cron.New(cron.WithSeconds()),
		jobs: make(map[uint]cron.EntryID),
	}
	taskManager = tm

	// Reset any tasks stuck in "running" state from previous crash
	db.MySQL.Model(&model.ScheduledTask{}).Where("last_status = ?", "running").
		Update("last_status", "unknown")

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

func (tm *TaskManager) ScheduleTask(task *model.ScheduledTask) {
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
		tm.executeTask(taskID, phase, name)
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
		log.Printf("[TaskManager] cron entry %d has zero next run, using default", entryID)
		nextRun = time.Now().Add(24 * time.Hour)
	}
	db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", task.ID).
		Update("next_run", nextRun)

	log.Printf("[TaskManager] scheduled: %s (%s) next=%s", name, expr, nextRun.Format("01-02 15:04"))
}

func (tm *TaskManager) executeTask(taskID uint, phase, name string) {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[TaskManager] PANIC in task %s (phase=%s): %v", name, phase, r)
			db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", taskID).
				Updates(map[string]interface{}{"last_status": "failed"})
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
		svc := NewMarketStyleService()
		var date string
		db.PG.Raw("SELECT trade_date::text FROM market_sentiment ORDER BY trade_date DESC LIMIT 1").Scan(&date)
		if date == "" {
			db.PG.Raw("SELECT trade_date::text FROM market_daily_agg ORDER BY trade_date DESC LIMIT 1").Scan(&date)
		}
		if date == "" {
			finishTaskLog(&logEntry, 0, 0, 0, fmt.Errorf("无可用交易日期"))
		} else {
			// Also fill any missing dates between last computed and latest
			var missing []string
			db.PG.Raw(`SELECT trade_date::text FROM market_sentiment
				WHERE trade_date NOT IN (SELECT trade_date FROM market_style_daily)
				ORDER BY trade_date`).Pluck("trade_date", &missing)
			success, fail := 0, 0
			for _, d := range missing {
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
			finishTaskLog(&logEntry, success, fail, 0, taskErr)
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
		}

		var err error
		if len(prog.Errors) > 0 {
			err = fmt.Errorf(strings.Join(prog.Errors, "; "))
			errMsgs = prog.Errors
		}

		finishTaskLog(&logEntry, totalNew, totalSkip, totalErr, err)
		if len(errMsgs) > 0 {
			db.MySQL.Model(&model.TaskLog{}).Where("id = ?", logEntry.ID).
				Update("error_msg", strings.Join(errMsgs, "; "))
		}
	}

	db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"last_run":    logEntry.StartedAt,
			"last_status": logEntry.Status,
		})
	// Update next run
	tm.mu.Lock()
	eid, eok := tm.jobs[taskID]
	tm.mu.Unlock()
	if eok {
		entry := tm.cron.Entry(eid)
		db.MySQL.Model(&model.ScheduledTask{}).Where("id = ?", taskID).
			Update("next_run", entry.Next)
	}

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
				GetTaskManager().ScheduleTask(&existing)
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
		GetTaskManager().ScheduleTask(&task)
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
		GetTaskManager().ScheduleTask(&task)
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
	GetTaskManager().ScheduleTask(&task)
	return &task, nil
}

func DeleteTask(id uint) error {
	var entryID cron.EntryID
	tm := GetTaskManager()
	tm.mu.Lock()
	if eid, ok := tm.jobs[id]; ok {
		entryID = eid
		delete(tm.jobs, id)
	}
	tm.mu.Unlock()
	if entryID != 0 {
		tm.cron.Remove(entryID)
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
	go GetTaskManager().executeTask(task.ID, task.Phase, task.Name)
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
	// For market_style, run bulk compute: clean zero rows + fill missing dates
	if task.Phase == "market_style" {
		// Clean zero-filled rows first (from failed previous computations)
		db.PG.Exec(`DELETE FROM market_style_daily WHERE up_ratio = 0 AND total_amount = 0`)
		var dates []string
		if err := db.PG.Raw(`
			SELECT trade_date::text FROM market_sentiment
			WHERE trade_date NOT IN (SELECT trade_date FROM market_style_daily)
			ORDER BY trade_date
		`).Pluck("trade_date", &dates).Error; err != nil {
			return fmt.Errorf("查询缺失日期失败: %w", err)
		}
		svc := NewMarketStyleService()
		success, fail := 0, 0
		for _, date := range dates {
			if err := svc.ComputeAndStore(date); err != nil {
				fail++
				log.Printf("[market_style] repair FAIL %s: %v", date, err)
			} else {
				success++
			}
		}
		log.Printf("[market_style] repair done: %d dates, %d ok, %d fail", len(dates), success, fail)
		if fail > 0 {
			return fmt.Errorf("市场风格修复: %d/%d 成功, %d 失败", success, len(dates), fail)
		}
		if success == 0 && len(dates) == 0 {
			return fmt.Errorf("没有缺失日期需要修复（market_style_daily 已是最新）")
		}
		return nil
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
	GetTaskManager().ScheduleTask(&task)
	return &task, nil
}
