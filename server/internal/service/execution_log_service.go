package service

import (
	"encoding/json"
	"log"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// ExecutionLogService handles persistence of run execution logs.
type ExecutionLogService struct{}

func NewExecutionLogService() *ExecutionLogService {
	return &ExecutionLogService{}
}

// LogEntry represents a single log line to be persisted.
type LogEntry struct {
	Level     string `json:"level,omitempty"`
	StockCode string `json:"stockCode,omitempty"`
	StockName string `json:"stockName,omitempty"`
	Message   string `json:"message"`
}

// SaveRunLogs persists a batch of log lines for a specific run+date+type.
// Existing logs for the same (run_id, trade_date, log_type) are replaced.
func (s *ExecutionLogService) SaveRunLogs(runID uint, tradeDate string, logType string, lines []string) {
	if len(lines) == 0 {
		return
	}

	// Delete existing logs for this run+date+type to avoid duplicates
	db.MySQL.Where("run_id = ? AND trade_date = ? AND log_type = ?", runID, tradeDate, logType).Delete(&model.RunExecutionLog{})

	// Batch insert
	for _, line := range lines {
		entry := model.RunExecutionLog{
			RunID:     runID,
			TradeDate: tradeDate,
			LogType:   logType,
			Level:     "info",
			Message:   line,
		}
		db.MySQL.Create(&entry)
	}

	log.Printf("[exec_log] saved %d %s logs for run=%d date=%s", len(lines), logType, runID, tradeDate)
}

// LoadRunLogs loads logs for a specific run+date+type.
func (s *ExecutionLogService) LoadRunLogs(runID uint, tradeDate string, logType string) ([]string, error) {
	var entries []model.RunExecutionLog
	db.MySQL.Where("run_id = ? AND trade_date = ? AND log_type = ?", runID, tradeDate, logType).
		Order("id ASC").Find(&entries)

	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.Message
	}
	return lines, nil
}

// LoadRunLogsJSON loads all log types for a run+date, grouped by type, as JSON.
func (s *ExecutionLogService) LoadRunLogsJSON(runID uint, tradeDate string) (map[string][]string, error) {
	var entries []model.RunExecutionLog
	db.MySQL.Where("run_id = ? AND trade_date = ?", runID, tradeDate).
		Order("log_type ASC, id ASC").Find(&entries)

	result := make(map[string][]string)
	for _, e := range entries {
		result[e.LogType] = append(result[e.LogType], e.Message)
	}
	return result, nil
}

// LastExecutionTime returns the latest created_at for a run+date+type.
func (s *ExecutionLogService) LastExecutionTime(runID uint, tradeDate string, logType string) string {
	var entry model.RunExecutionLog
	db.MySQL.Where("run_id = ? AND trade_date = ? AND log_type = ?", runID, tradeDate, logType).
		Order("id DESC").First(&entry)
	if entry.ID == 0 {
		return ""
	}
	return entry.CreatedAt.Format("15:04:05")
}

// GetAvailableDates returns distinct trade_dates that have logs for a run.
func (s *ExecutionLogService) GetAvailableDates(runID uint) ([]string, error) {
	var dates []string
	db.MySQL.Model(&model.RunExecutionLog{}).
		Where("run_id = ?", runID).
		Distinct("trade_date").
		Order("trade_date DESC").
		Pluck("trade_date", &dates)
	return dates, nil
}

// buildLogsJSON is a helper to marshal log lines for the old last_run_log field.
func buildLogsJSON(lines []string) string {
	b, _ := json.Marshal(lines)
	return string(b)
}
