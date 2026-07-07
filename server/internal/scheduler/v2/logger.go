package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// StructuredLogger produces JSON log lines with trace_id correlation.
// Each task execution gets its own logger with a unique TraceID.
type StructuredLogger struct {
	TraceID    string    `json:"trace_id"`
	TaskDef    string    `json:"task_def"`
	TaskKind   TaskKind  `json:"task_kind"`
	InstanceID uint      `json:"instance_id,omitempty"`
	Owner      string    `json:"owner,omitempty"` // "run:42" / "account:7"
	mu         sync.Mutex
	out        *log.Logger
	fields     map[string]any // extra fields added to every log line
}

// NewStructuredLogger creates a logger writing to os.Stdout.
func NewStructuredLogger(traceID, taskDef string, kind TaskKind) *StructuredLogger {
	return &StructuredLogger{
		TraceID:  traceID,
		TaskDef:  taskDef,
		TaskKind: kind,
		out:      log.New(os.Stdout, "", 0),
		fields:   make(map[string]any),
	}
}

// WithInstance attaches instance metadata.
func (l *StructuredLogger) WithInstance(instanceID uint, owner ResourceRef) *StructuredLogger {
	l.InstanceID = instanceID
	if owner.Kind != "" {
		l.Owner = fmt.Sprintf("%s:%d", owner.Kind, owner.ID)
	}
	return l
}

// WithField adds a persistent field to every subsequent log line.
func (l *StructuredLogger) WithField(key string, val any) *StructuredLogger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fields[key] = val
	return l
}

// Info logs an informational event.
func (l *StructuredLogger) Info(event string, extra map[string]any) {
	l.emit("INFO", event, extra)
}

// Warn logs a warning event.
func (l *StructuredLogger) Warn(event string, extra map[string]any) {
	l.emit("WARN", event, extra)
}

// Error logs an error event. If err is non-nil, it is included.
func (l *StructuredLogger) Error(event string, err error, extra map[string]any) {
	if extra == nil {
		extra = make(map[string]any)
	}
	if err != nil {
		extra["error"] = err.Error()
	}
	l.emit("ERROR", event, extra)
}

// Phase marks a phase transition within a task.
func (l *StructuredLogger) Phase(phase string, extra map[string]any) {
	if extra == nil {
		extra = make(map[string]any)
	}
	extra["phase"] = phase
	l.emit("PHASE", "phase_transition", extra)
}

// Metric logs a numeric metric (duration, count, etc).
func (l *StructuredLogger) Metric(name string, value float64, unit string) {
	l.emit("METRIC", "metric", map[string]any{
		"metric_name":  name,
		"metric_value": value,
		"metric_unit":  unit,
	})
}

func (l *StructuredLogger) emit(level, event string, extra map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := make(map[string]any, 10+len(l.fields)+len(extra))
	entry["ts"] = time.Now().Format(time.RFC3339Nano)
	entry["level"] = level
	entry["event"] = event
	entry["trace_id"] = l.TraceID
	entry["task_def"] = l.TaskDef
	entry["task_kind"] = l.TaskKind

	if l.InstanceID > 0 {
		entry["instance_id"] = l.InstanceID
	}
	if l.Owner != "" {
		entry["owner"] = l.Owner
	}

	for k, v := range l.fields {
		entry[k] = v
	}
	for k, v := range extra {
		entry[k] = v
	}

	b, _ := json.Marshal(entry)
	l.out.Println(string(b))
}

// ── TraceID generation ──

var traceSeq struct {
	mu  sync.Mutex
	seq int64
}

// NewTraceID generates a unique trace ID for a task execution.
// Format: "kind:taskdef:instance:timestamp:seq"
func NewTraceID(kind TaskKind, taskDef string, instanceID uint) string {
	traceSeq.mu.Lock()
	traceSeq.seq++
	seq := traceSeq.seq
	traceSeq.mu.Unlock()

	return fmt.Sprintf("%s:%s:%d:%s:%04x",
		kind, taskDef, instanceID,
		time.Now().Format("01021504"),
		seq&0xFFFF,
	)
}

// NewEventID generates a unique event ID.
func NewEventID(eventType, key string) string {
	traceSeq.mu.Lock()
	traceSeq.seq++
	seq := traceSeq.seq
	traceSeq.mu.Unlock()

	return fmt.Sprintf("evt:%s:%s:%d:%04x",
		eventType, key,
		time.Now().UnixMilli(),
		seq&0xFFFF,
	)
}
