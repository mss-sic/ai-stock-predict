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
}

type TaskManager struct {
	cron  *cron.Cron
	mu    sync.Mutex
	jobs  map[uint]cron.EntryID // taskID -> cron entry ID
}

var taskManager *TaskManager

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

	// Calculate next run
	entry := tm.cron.Entry(entryID)
	nextRun := entry.Next
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
	} else {
		// Run collector phase
		phases := []string{phase}
		collector.RunManualCollection(phases)

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

// InitializeDefaultTasks creates default tasks if none exist
func InitializeDefaultTasks() (int, error) {
	var count int64
	db.MySQL.Model(&model.ScheduledTask{}).Count(&count)
	if count > 0 {
		return 0, nil
	}

	created := 0
	for _, dt := range DefaultTasks {
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

func RunTaskNow(id uint) error {
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
