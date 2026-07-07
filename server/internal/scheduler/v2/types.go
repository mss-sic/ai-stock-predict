// Package scheduler provides the v2 unified task scheduling engine.
// Supports pipelines (system-level data collection), strategy tasks (per-StrategyRun),
// account tasks (per-TradingAccount), portfolio tasks (cross-account), and custom user tasks.
package scheduler

import (
	"context"
	"time"
)

// ── TaskKind ──

// TaskKind classifies tasks for isolation and resource-limiter policies.
type TaskKind string

const (
	KindPipeline  TaskKind = "pipeline"  // system-level collection pipelines
	KindStrategy  TaskKind = "strategy"  // per-StrategyRun (signal gen, AI decision)
	KindAccount   TaskKind = "account"   // per-TradingAccount (balance, margin)
	KindPortfolio TaskKind = "portfolio" // cross-account (rebalance, weekly report)
	KindCustom    TaskKind = "custom"    // user-defined (price alert, webhook)
)

// ── Priority ──

// Priority determines execution order when multiple jobs compete for workers.
type Priority int

const (
	PriorityCritical Priority = 0 // must run immediately (quote, risk)
	PriorityHigh     Priority = 1 // blocks downstream (kline, daily_run)
	PriorityNormal   Priority = 2 // standard tasks (sentiment, AI decision)
	PriorityLow      Priority = 3 // best-effort (backfill, weekly reports)
)

// ── TriggerSpec ──

// TriggerSpec defines when a task should fire.
// A task fires when: (cron matches OR event arrives OR manual trigger) AND all require_all events are present.
// If a deadline is set and the task hasn't started by that time, the degrade handler is invoked.
type TriggerSpec struct {
	Cron   string   `json:"cron,omitempty"`    // cron expression, e.g. "40 10 16 * * 1-5"
	Events []string `json:"events,omitempty"`  // fire on these events, e.g. ["data_ready:2026-07-07"]
	Manual bool     `json:"manual,omitempty"`  // allow manual trigger via API

	RequireAll  []string `json:"requireAll,omitempty"`  // must wait for these events before firing
	Deadline    string   `json:"deadline,omitempty"`     // "HH:MM" cutoff; past deadline → degrade
	TradingDay  bool     `json:"tradingDay,omitempty"`   // only fire on trading days
	MinInterval string   `json:"minInterval,omitempty"`  // min interval between runs, e.g. "5m"
}

// TriggerResult captures the outcome of evaluating a TriggerSpec.
type TriggerResult struct {
	Fired       bool     // whether the task should execute now
	WaitFor     []string // events still needed
	PastDeadline bool    // deadline has passed → degrade
	Reason      string   // human-readable explanation
}

// ── ResourceRef ──

// ResourceRef identifies the owner resource of a TaskInstance.
type ResourceRef struct {
	Kind string `json:"kind"` // "strategy_run" / "trading_account" / "user"
	ID   uint   `json:"id"`
}

// ── TaskDefinition ──

// TaskDefinition is a reusable template for tasks.
// System definitions (pipelines) are hardcoded; user definitions are created via API.
type TaskDefinition struct {
	ID          string    `json:"id"`          // unique identifier, e.g. "live_daily_run" / "after_close"
	Kind        TaskKind  `json:"kind"`
	Label       string    `json:"label"`       // human-readable name
	Description string    `json:"description"`

	Trigger     TriggerSpec     `json:"trigger"`
	Timeout     time.Duration   `json:"timeout"`       // per-execution timeout
	RetryPolicy RetryPolicy     `json:"retryPolicy"`
	DegradeFn   DegradeHandler  `json:"-"`              // called on deadline miss or timeout
	Handler     TaskHandler     `json:"-"`              // the actual work function

	// Resource limits
	MaxConcurrent int   `json:"maxConcurrent"` // max concurrent instances of this def (0=unlimited)
	TokenBudget   int64 `json:"tokenBudget"`   // AI token budget per execution (0=unlimited)

	// For Kind=Custom: parameter schema exposed to users
	ParamSchema []ParamDef `json:"paramSchema,omitempty"`
}

// TaskHandler is the function that executes a task.
type TaskHandler func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error

// DegradeHandler is called when a task cannot execute within its deadline.
type DegradeHandler func(ctx context.Context, inst *TaskInstance, reason string, logger *StructuredLogger) error

// RetryPolicy controls automatic retry behavior.
type RetryPolicy struct {
	MaxRetries int           `json:"maxRetries"` // 0 = no retry
	Backoff    time.Duration `json:"backoff"`    // initial backoff, doubles each retry
	MaxBackoff time.Duration `json:"maxBackoff"` // cap on backoff
}

// ParamDef describes a single custom task parameter.
type ParamDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string" / "number" / "boolean" / "select"
	Label       string `json:"label"`
	Default     any    `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Options     []any  `json:"options,omitempty"` // for type=select
}

// ── TaskInstance ──

// TaskInstance is a concrete instance of a TaskDefinition bound to a resource.
type TaskInstance struct {
	ID           uint        `json:"id"`
	DefinitionID string      `json:"definitionId"`
	BizTaskID    uint        `json:"bizTaskId"` // 0=system task, >0=business entity ID (e.g. strategy_run.id)
	Label        string      `json:"label,omitempty"` // per-instance display label override
	Owner        ResourceRef `json:"owner"`
	UserID       uint        `json:"userId"`

	// Overrides for the definition's trigger (user-configurable)
	Trigger TriggerSpec `json:"trigger"`
	Params  map[string]any `json:"params,omitempty"` // runtime parameter overrides
	Enabled bool          `json:"enabled"`

	// Execution state
	LastRunAt  *time.Time `json:"lastRunAt"`
	LastStatus string     `json:"lastStatus"` // pending / running / success / failed / degraded
	NextRunAt  *time.Time `json:"nextRunAt"`

	// Notifications
	NotifyOn       []string `json:"notifyOn,omitempty"`       // event types: "success" / "failure" / "degraded"
	NotifyChannels []string `json:"notifyChannels,omitempty"` // channel IDs for delivery

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ── Job ──

// Job is an in-flight task execution request placed on the JobQueue.
type Job struct {
	ID           string       `json:"id"`       // unique job ID (instance-scoped trace)
	TraceID      string       `json:"traceId"`
	DefinitionID string       `json:"definitionId"`
	Kind         TaskKind     `json:"kind"`
	InstanceID   uint         `json:"instanceId"`
	UserID       uint         `json:"userId"`
	Priority     Priority     `json:"priority"`
	Handler      TaskHandler  `json:"-"`
	Timeout      time.Duration `json:"timeout"`
	CreatedAt    time.Time    `json:"createdAt"`

	// Context for the handler
	Payload map[string]any `json:"payload,omitempty"`
}

// ── Event ──

// Event is an internal event propagated through the EventBus.
type Event struct {
	ID        string    `json:"id"`        // unique event ID
	Type      string    `json:"type"`      // "data_ready" / "task_completed" / "phase_failed"
	Key       string    `json:"key"`       // scoping key, e.g. "2026-07-07" / "live_daily_run:42"
	Payload   any       `json:"payload,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Common event types
const (
	EventDataReady      = "data_ready"      // downstream data collection complete
	EventMorningReady   = "morning_ready"   // pre-market data ready
	EventTaskCompleted  = "task_completed"  // a TaskInstance execution finished
	EventTaskFailed     = "task_failed"     // a TaskInstance execution failed
	EventTaskDegraded   = "task_degraded"   // a TaskInstance was degraded
	EventPhaseCompleted = "phase_completed" // a Pipeline stage finished
	EventPhaseFailed    = "phase_failed"    // a Pipeline stage failed
	EventMarketOpen     = "market_open"     // market opened
	EventMarketClose    = "market_close"    // market closed
)

// ── JobStatus ──

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSuccess   JobStatus = "success"
	JobStatusFailed    JobStatus = "failed"
	JobStatusDegraded  JobStatus = "degraded"
	JobStatusCancelled JobStatus = "cancelled"
)

// ── Health ──

// SchedulerHealth summarizes the scheduler's state.
type SchedulerHealth struct {
	Status   string        `json:"status"` // "healthy" / "degraded" / "unhealthy"
	Uptime   time.Duration `json:"uptime"`
	Workers  WorkerHealth  `json:"workers"`
	Queues   map[TaskKind]QueueHealth `json:"queues"`
	ActiveAlerts []HealthAlert `json:"activeAlerts"`
}

type WorkerHealth struct {
	Total       int     `json:"total"`
	Busy        int     `json:"busy"`
	Idle        int     `json:"idle"`
	AvgBusyPct  float64 `json:"avgBusyPct"` // 5-min average
}

type QueueHealth struct {
	Depth    int           `json:"depth"`    // pending jobs
	Oldest   time.Duration `json:"oldest"`   // oldest job wait time
	AvgWait  time.Duration `json:"avgWait"`  // average wait time (5-min window)
}

type HealthAlert struct {
	Level   string    `json:"level"`   // "warning" / "critical"
	Message string    `json:"message"`
	Since   time.Time `json:"since"`
}

