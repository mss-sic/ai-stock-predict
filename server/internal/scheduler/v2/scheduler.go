package scheduler

import (
	"context"
	"strings"
	"fmt"
	"log"

	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/ai-stock-predict/server/internal/db"
)

// cronParser is a shared cron parser with seconds support.
var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)


// ── Global Accessor ──

var globalScheduler *UnifiedScheduler

// SetGlobal sets the global scheduler instance (called from main).
func SetGlobal(s *UnifiedScheduler) {
	globalScheduler = s
}

// GetGlobal returns the global scheduler instance.
func GetGlobal() *UnifiedScheduler {
	return globalScheduler
}

// ── Config ──

// Config controls scheduler behavior.
type Config struct {
	// Mode: "standalone" (default, in-memory) or "distributed" (Postgres-backed, future).
	Mode string

	// Workers: number of goroutine workers pulling from the job queue.
	Workers int

	// InstanceID uniquely identifies this scheduler instance (for distributed mode).
	InstanceID string

	// EvalInterval: how often to re-evaluate TriggerSpecs for cron-based tasks.
	EvalInterval time.Duration
}

// DefaultConfig returns a sensible standalone configuration.
func DefaultConfig() Config {
	return Config{
		Mode:         "standalone",
		Workers:      4,
		InstanceID:   "default",
		EvalInterval: 10 * time.Second,
	}
}

// ── UnifiedScheduler ──

// UnifiedScheduler is the central task scheduling engine.
// It manages Pipeline definitions, user TaskInstances, and orchestrates execution
// through a JobQueue and WorkerPool.
type UnifiedScheduler struct {
	cfg Config

	// Components
	cron   *cron.Cron
	queue  JobQueue
	events EventBus
	leader LeaderElection
	store  StateStore
	pool   *WorkerPool

	// Registry
	mu          sync.RWMutex
	definitions map[string]*TaskDefinition   // definitionID → def
	instances   map[uint]*TaskInstance       // instanceID → inst
	byDef       map[string][]uint            // definitionID → []instanceID
	byOwner     map[string]map[uint]struct{} // "kind:id" → instanceID set
	byBizTask   map[string]uint              // "definitionID:bizTaskID" → instanceID (for upsert)

	// State
	ctx       context.Context
	cancel    context.CancelFunc
	started   time.Time
	alerts    []HealthAlert
	alertsMu  sync.Mutex
}

// New creates a new UnifiedScheduler with the given config.
// For standalone mode, all backends are in-memory.
func New(cfg Config) *UnifiedScheduler {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.EvalInterval <= 0 {
		cfg.EvalInterval = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &UnifiedScheduler{
		cfg:         cfg,
		cron:        cron.New(cron.WithSeconds()),
		queue:       NewInMemoryJobQueue(),
		events:      NewInMemoryEventBus(),
		leader:      NewInMemoryLeaderElection(),
		store:       NewInMemoryStateStore(),
		definitions: make(map[string]*TaskDefinition),
		instances:   make(map[uint]*TaskInstance),
		byDef:       make(map[string][]uint),
		byOwner:     make(map[string]map[uint]struct{}),
		byBizTask:   make(map[string]uint),
		ctx:         ctx,
		cancel:      cancel,
	}
	s.pool = NewWorkerPool(s.queue, cfg.Workers)
	return s
}

// Start begins the scheduler: starts cron, worker pool, and the evaluation loop.
func (s *UnifiedScheduler) Start() {
	s.started = time.Now()

	s.cron.Start()
	s.pool.Start()

	// Main evaluation loop: periodically check all triggers
	go s.evalLoop()

	// Event-driven evaluation: when events arrive, re-check waiting tasks
	go s.eventLoop()

	log.Printf("[scheduler-v2] started (mode=%s workers=%d)", s.cfg.Mode, s.cfg.Workers)
}

// Stop gracefully shuts down the scheduler.
func (s *UnifiedScheduler) Stop() {
	s.cancel()
	s.cron.Stop()
	s.pool.Stop()
	s.queue.Close()
	s.events.Close()
	s.leader.Close()
	s.store.Close()
	log.Printf("[scheduler-v2] stopped")
}

// ── Definition Management ──

// RegisterDefinition adds a task definition to the registry.
func (s *UnifiedScheduler) RegisterDefinition(def *TaskDefinition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.definitions[def.ID] = def
	log.Printf("[scheduler-v2] registered definition: %s (kind=%s)", def.ID, def.Kind)
}

// GetDefinition returns a task definition by ID.
func (s *UnifiedScheduler) GetDefinition(id string) *TaskDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.definitions[id]
}

// ListDefinitions returns all registered definitions, optionally filtered by kind.
func (s *UnifiedScheduler) ListDefinitions(kind TaskKind) []*TaskDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TaskDefinition
	for _, def := range s.definitions {
		if kind == "" || def.Kind == kind {
			result = append(result, def)
		}
	}
	return result
}

// ── Instance Management ──

// RegisterInstance adds a task instance and starts scheduling. The instance must
// reference a valid definition ID.
func (s *UnifiedScheduler) RegisterInstance(inst *TaskInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.definitions[inst.DefinitionID]; !ok {
		return fmt.Errorf("unknown definition: %s", inst.DefinitionID)
	}

	// Assign an ID if not set (simple auto-increment for standalone)
	if inst.ID == 0 {
		maxID := uint(0)
		for id := range s.instances {
			if id > maxID {
				maxID = id
			}
		}
		inst.ID = maxID + 1
	}

	if _, exists := s.instances[inst.ID]; exists {
		return fmt.Errorf("instance %d already registered", inst.ID)
	}

	inst.CreatedAt = time.Now()
	inst.UpdatedAt = time.Now()
	s.instances[inst.ID] = inst

	s.byDef[inst.DefinitionID] = append(s.byDef[inst.DefinitionID], inst.ID)

	ownerKey := inst.Owner.Kind + ":" + fmt.Sprint(inst.Owner.ID)
	if s.byOwner[ownerKey] == nil {
		s.byOwner[ownerKey] = make(map[uint]struct{})
	}
	s.byOwner[ownerKey][inst.ID] = struct{}{}

	log.Printf("[scheduler-v2] registered instance: %d (def=%s owner=%s)", inst.ID, inst.DefinitionID, ownerKey)
	return nil
}

// UpdateInstance updates trigger, params, enabled status, and notification config.
func (s *UnifiedScheduler) UpdateInstance(id uint, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, ok := s.instances[id]
	if !ok {
		return fmt.Errorf("instance %d not found", id)
	}

	if v, ok := updates["trigger"]; ok {
		inst.Trigger = v.(TriggerSpec)
	}
	if v, ok := updates["params"]; ok {
		inst.Params = v.(map[string]any)
	}
	if v, ok := updates["enabled"]; ok {
		inst.Enabled = v.(bool)
	}
	if v, ok := updates["notifyOn"]; ok {
		inst.NotifyOn = v.([]string)
	}
	if v, ok := updates["notifyChannels"]; ok {
		inst.NotifyChannels = v.([]string)
	}
	inst.UpdatedAt = time.Now()
	return nil
}

// UnregisterInstance removes an instance and stops its scheduling.
func (s *UnifiedScheduler) UnregisterInstance(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst, ok := s.instances[id]
	if !ok {
		return fmt.Errorf("instance %d not found", id)
	}

	// Remove from byDef index
	if ids, ok2 := s.byDef[inst.DefinitionID]; ok2 {
		filtered := make([]uint, 0, len(ids))
		for _, i := range ids {
			if i != id {
				filtered = append(filtered, i)
			}
		}
		s.byDef[inst.DefinitionID] = filtered
	}

	// Remove from byOwner index
	ownerKey := inst.Owner.Kind + ":" + fmt.Sprint(inst.Owner.ID)
	if owners, ok2 := s.byOwner[ownerKey]; ok2 {
		delete(owners, id)
		if len(owners) == 0 {
			delete(s.byOwner, ownerKey)
		}
	}

	delete(s.instances, id)
	log.Printf("[scheduler-v2] unregistered instance: %d", id)
	return nil
}

// UnbindAll removes all instances owned by the given resource.
func (s *UnifiedScheduler) UnbindAll(owner ResourceRef) int {
	s.mu.Lock()
	ownerKey := owner.Kind + ":" + fmt.Sprint(owner.ID)
	ids := make([]uint, 0)
	if owners, ok := s.byOwner[ownerKey]; ok {
		for id := range owners {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()

	count := 0
	for _, id := range ids {
		if err := s.UnregisterInstance(id); err == nil {
			count++
		}
	}
	return count
}

// GetInstance returns an instance by ID.
func (s *UnifiedScheduler) GetInstance(id uint) *TaskInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instances[id]
}

// ListInstances returns all instances, optionally filtered by definition ID or owner.
func (s *UnifiedScheduler) ListInstances(defID string, owner *ResourceRef) []*TaskInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TaskInstance
	for _, inst := range s.instances {
		if defID != "" && inst.DefinitionID != defID {
			continue
		}
		if owner != nil && (inst.Owner.Kind != owner.Kind || inst.Owner.ID != owner.ID) {
			continue
		}
		instCopy := *inst
		result = append(result, &instCopy)
	}
	return result
}

// ── Trigger Evaluation ──

// evalLoop periodically evaluates all enabled instances and enqueues jobs that are due.
func (s *UnifiedScheduler) evalLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler-v2] evalLoop panic: %v", r)
		}
	}()
	log.Printf("[scheduler-v2] evalLoop started, interval=%v", s.cfg.EvalInterval)
	ticker := time.NewTicker(s.cfg.EvalInterval)
	defer ticker.Stop()

	// Fire first evaluation immediately
	s.evaluateAll()

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("[scheduler-v2] evalLoop stopped")
			return
		case <-ticker.C:
			s.evaluateAll()
		}
	}
}

func (s *UnifiedScheduler) evaluateAll() {
	s.mu.RLock()
	instances := make([]*TaskInstance, 0, len(s.instances))
	for _, inst := range s.instances {
		if inst.Enabled {
			instances = append(instances, inst)
		}
	}
	s.mu.RUnlock()

	now := time.Now()
	fired := 0
	waiting := 0
	for _, inst := range instances {
		def := s.GetDefinition(inst.DefinitionID)
		if def == nil {
			continue
		}

		result := s.evaluateTrigger(&def.Trigger, &inst.Trigger, now, inst)
		if result.Fired {
			s.enqueueFromInstance(inst, def, "cron")
			fired++
		} else if result.Reason != "" && result.Reason != "no trigger condition met" && result.Reason != "manual only" && result.Reason != "not a trading day" {
			waiting++
		}
	}
	if fired > 0 {
		log.Printf("[scheduler-v2] eval at %s: fired=%d, total=%d", now.Format("15:04:05"), fired, len(instances))
	}
}

// eventLoop listens for events and re-checks instances waiting on them.
func (s *UnifiedScheduler) eventLoop() {
	ch, err := s.events.Subscribe(s.ctx, "*")
	if err != nil {
		log.Printf("[scheduler-v2] event subscription failed: %v", err)
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			s.handleEvent(event)
		}
	}
}

func (s *UnifiedScheduler) handleEvent(event Event) {
	eventKey := formatEventKey(event.Type, event.Key)

	s.mu.RLock()
	var candidates []*TaskInstance
	for _, inst := range s.instances {
		if !inst.Enabled {
			continue
		}
		// Check if this instance is waiting for this event
		for _, evt := range inst.Trigger.RequireAll {
			if matchPattern(evt, eventKey) || matchPattern(evt, event.Type) {
				candidates = append(candidates, inst)
				break
			}
		}
		for _, evt := range inst.Trigger.Events {
			if matchPattern(evt, eventKey) || matchPattern(evt, event.Type) {
				candidates = append(candidates, inst)
				break
			}
		}
	}
	s.mu.RUnlock()

	now := time.Now()
	for _, inst := range candidates {
		def := s.GetDefinition(inst.DefinitionID)
		if def == nil {
			continue
		}

		result := s.evaluateTrigger(&def.Trigger, &inst.Trigger, now, inst)
		if result.Fired {
			s.enqueueFromInstance(inst, def, "event:"+event.Type)
		}
	}
}

// evaluateTrigger checks both the definition trigger and instance-specific overrides.
func (s *UnifiedScheduler) evaluateTrigger(defTrigger, instTrigger *TriggerSpec, now time.Time, inst *TaskInstance) TriggerResult {
	// Merge: instance trigger overrides definition trigger
	trigger := *defTrigger
	if instTrigger != nil {
		if instTrigger.Cron != "" {
			trigger.Cron = instTrigger.Cron
		}
		if len(instTrigger.Events) > 0 {
			trigger.Events = make([]string, len(instTrigger.Events))
			copy(trigger.Events, instTrigger.Events)
		}
		if len(instTrigger.RequireAll) > 0 {
			trigger.RequireAll = make([]string, len(instTrigger.RequireAll))
			copy(trigger.RequireAll, instTrigger.RequireAll)
		}
		if instTrigger.Deadline != "" {
			trigger.Deadline = instTrigger.Deadline
		}
		if instTrigger.MinInterval != "" {
			trigger.MinInterval = instTrigger.MinInterval
		}
	}

	result := TriggerResult{}

	// Check min interval since last run
	if inst.LastRunAt != nil && trigger.MinInterval != "" {
		minDur, _ := time.ParseDuration(trigger.MinInterval)
		if minDur > 0 && now.Sub(*inst.LastRunAt) < minDur {
			if trigger.Cron != "" {
				if sched, err := cronParser.Parse(trigger.Cron); err == nil {
					nextAt := sched.Next(*inst.LastRunAt)
					if nextAt.After(now) {
						result.WaitFor = append(result.WaitFor, "cron_next="+nextAt.Format("15:04:05"))
					}
				}
			}
			result.Reason = "min_interval not elapsed, last run: " + inst.LastRunAt.Format("15:04:05")
			return result
		}
	}

	// Check trading day requirement
	if trigger.TradingDay && !isTradingDay(now) {
		result.Reason = "not a trading day"
		return result
	}

	// Check require_all events — all must be satisfied
	requiredMet := true
	for _, evt := range trigger.RequireAll {
		exists := s.storeHas(s.ctx, "event:"+evt)
		if exists {
			continue
		}
		requiredMet = false
		result.WaitFor = append(result.WaitFor, evt)
	}
	if !requiredMet {
		result.Reason = "waiting for events: " + joinStrings(result.WaitFor)
		return result
	}

	// Check deadline
	if trigger.Deadline != "" {
		deadlineTime, err := parseTimeOfDay(now, trigger.Deadline)
		if err == nil && now.After(deadlineTime) {
			result.PastDeadline = true
			result.Reason = "past deadline " + trigger.Deadline
			result.Fired = true
			return result
		}
	}

	// Check if triggered by events (no cron needed)
	if len(trigger.Events) > 0 {
		for _, evt := range trigger.Events {
			exists := s.storeHas(s.ctx, "event:"+evt)
			if exists {
				result.Fired = true
				result.Reason = "event triggered: " + evt
				return result
			}
		}
		if trigger.Cron == "" {
			result.WaitFor = trigger.Events
			result.Reason = "waiting for event"
			return result
		}
	}

	// Check cron — parse-driven next-run evaluation
	if trigger.Cron != "" {
		schedule, err := cronParser.Parse(trigger.Cron)
		if err != nil {
			result.Reason = fmt.Sprintf("invalid cron: %s (%v)", trigger.Cron, err)
			return result
		}

		if inst.LastRunAt != nil && inst.LastRunAt.Before(now) {
			nextAfterLast := schedule.Next(*inst.LastRunAt)
			if !nextAfterLast.After(now) {
				result.Fired = true
				nextAfterNow := schedule.Next(now)
				inst.NextRunAt = &nextAfterNow
				result.Reason = fmt.Sprintf("cron fired, next at %s", nextAfterNow.Format("15:04:05"))
				return result
			}
			inst.NextRunAt = &nextAfterLast
			result.WaitFor = append(result.WaitFor, "cron_next="+nextAfterLast.Format("15:04:05"))
			result.Reason = fmt.Sprintf("waiting for cron, next at %s", nextAfterLast.Format("15:04:05"))
			return result
		}

		// First run: use a grace window of 2x eval interval
		nextAt := schedule.Next(now.Add(-2 * s.cfg.EvalInterval))
		if !nextAt.After(now) {
			result.Fired = true
			nextAfterNow := schedule.Next(now)
			inst.NextRunAt = &nextAfterNow
			result.Reason = fmt.Sprintf("cron fired (first run), next at %s", nextAfterNow.Format("15:04:05"))
			return result
		}
		inst.NextRunAt = &nextAt
		result.WaitFor = append(result.WaitFor, "cron_next="+nextAt.Format("15:04:05"))
		result.Reason = fmt.Sprintf("waiting for first cron, next at %s", nextAt.Format("15:04:05"))
		return result
	}

	// Manual-only tasks
	if trigger.Manual {
		result.Reason = "manual only"
		return result
	}

	result.Reason = "no trigger condition met"
	return result
}

// ── Job Creation ──

// TriggerNow manually triggers an instance immediately.
func (s *UnifiedScheduler) TriggerNow(instanceID uint) error {
	inst := s.GetInstance(instanceID)
	if inst == nil {
		return fmt.Errorf("instance %d not found", instanceID)
	}
	def := s.GetDefinition(inst.DefinitionID)
	if def == nil {
		return fmt.Errorf("definition %s not found", inst.DefinitionID)
	}
	s.enqueueFromInstance(inst, def, "manual")
	return nil
}

// enqueueFromInstance creates a Job from a TaskInstance and enqueues it.
func (s *UnifiedScheduler) enqueueFromInstance(inst *TaskInstance, def *TaskDefinition, source string) {
	// Idempotency guard: skip if already running or recently completed (< 5s ago)
	if inst.LastStatus == "running" {
		log.Printf("[scheduler-v2] %s instance %d skipped: already running", def.ID, inst.ID)
		return
	}
	if inst.LastRunAt != nil && time.Since(*inst.LastRunAt) < 5*time.Second {
		log.Printf("[scheduler-v2] %s instance %d skipped: completed < 5s ago", def.ID, inst.ID)
		return
	}

	// Check concurrency limit for this definition
	if def.MaxConcurrent > 0 {
		s.mu.RLock()
		active := 0
		for _, i := range s.instances {
			if i.DefinitionID == def.ID && i.LastStatus == "running" {
				active++
			}
		}
		s.mu.RUnlock()
		if active >= def.MaxConcurrent {
			log.Printf("[scheduler-v2] %s instance %d skipped: max concurrency %d reached", def.ID, inst.ID, def.MaxConcurrent)
			return
		}
	}

	traceID := NewTraceID(def.Kind, def.ID, inst.ID)
	jobID := fmt.Sprintf("job:%s:%d:%d", def.ID, inst.ID, time.Now().UnixNano())

	job := &Job{
		ID:           jobID,
		TraceID:      traceID,
		DefinitionID: def.ID,
		Kind:         def.Kind,
		InstanceID:   inst.ID,
		UserID:       inst.UserID,
		Priority:     PriorityNormal, // Pipelines set higher priority in their handler
		Timeout:      def.Timeout,
		Handler:      s.makeHandler(def, inst),
		Payload: map[string]any{
			"source": source,
			"params": inst.Params,
		},
	}

	if def.Kind == KindPipeline {
		job.Priority = PriorityHigh
	}

	log.Printf("[scheduler-v2] enqueue %s: trace=%s source=%s", def.ID, traceID, source)

	// Try leader election for pipeline singleton tasks
	if def.Kind == KindPipeline {
		lockKey := "pipeline:" + def.ID
		acquired, err := s.leader.TryAcquire(s.ctx, lockKey, def.Timeout+30*time.Second)
		if err != nil || !acquired {
			log.Printf("[scheduler-v2] %s: leader election failed, another instance is running", def.ID)
			return
		}
	}

	inst.LastStatus = "pending"
	inst.UpdatedAt = time.Now()

	if err := s.queue.Enqueue(s.ctx, job); err != nil {
		log.Printf("[scheduler-v2] enqueue %s failed: %v", def.ID, err)
	}
}

// makeHandler wraps a TaskDefinition's handler with logging, timeout, and degrade logic.
func (s *UnifiedScheduler) makeHandler(def *TaskDefinition, inst *TaskInstance) TaskHandler {
	return func(ctx context.Context, instArg *TaskInstance, logger *StructuredLogger) error {
		// Create trace logger
		traceID := NewTraceID(def.Kind, def.ID, inst.ID)
		l := NewStructuredLogger(traceID, def.ID, def.Kind)
		l.WithInstance(inst.ID, inst.Owner)
		l.Info("task_started", map[string]any{
			"timeout": def.Timeout.String(),
			"params":  inst.Params,
		})

		inst.LastStatus = "running"
		inst.NowRunning()

		start := time.Now()

		// Check if past deadline
		if inst.Trigger.Deadline != "" || def.Trigger.Deadline != "" {
			deadline := inst.Trigger.Deadline
			if deadline == "" {
				deadline = def.Trigger.Deadline
			}
			if deadlineTime, err := parseTimeOfDay(time.Now(), deadline); err == nil {
				if time.Now().After(deadlineTime) && def.DegradeFn != nil {
					l.Warn("deadline_exceeded", map[string]any{"deadline": deadline})
					inst.LastStatus = "degraded"
					return def.DegradeFn(ctx, inst, "deadline exceeded", l)
				}
			}
		}

		// Execute with retry
		var lastErr error
		for attempt := 0; attempt <= def.RetryPolicy.MaxRetries; attempt++ {
			if attempt > 0 {
				backoff := def.RetryPolicy.Backoff * time.Duration(1<<uint(attempt-1))
				if backoff > def.RetryPolicy.MaxBackoff && def.RetryPolicy.MaxBackoff > 0 {
					backoff = def.RetryPolicy.MaxBackoff
				}
				l.Info("retry_attempt", map[string]any{
					"attempt": attempt,
					"backoff": backoff.String(),
				})
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if def.Handler != nil {
		// Inject scheduler into context for handlers that need event emission
		ctx = context.WithValue(ctx, ctxKeyScheduler, s)
				lastErr = def.Handler(ctx, inst, l)
			}
			if lastErr == nil {
				break
			}
		}

		elapsed := time.Since(start)

		if lastErr != nil {
			inst.LastStatus = "failed"
			l.Error("task_failed", lastErr, map[string]any{
				"duration_ms": elapsed.Milliseconds(),
				"retries":     def.RetryPolicy.MaxRetries,
			})
			return lastErr
		}

		inst.LastStatus = "success"
		now := time.Now()
		inst.LastRunAt = &now
		l.Metric("duration_ms", float64(elapsed.Milliseconds()), "ms")
		l.Info("task_completed", map[string]any{"duration_ms": elapsed.Milliseconds()})

		// Emit completion event
		s.emitEvent(Event{
			ID:        NewEventID(EventTaskCompleted, def.ID),
			Type:      EventTaskCompleted,
			Key:       fmt.Sprintf("%s:%d", def.ID, inst.ID),
			Timestamp: time.Now(),
		})

		// Release leader lock for pipeline tasks
		if def.Kind == KindPipeline {
			s.leader.Release(s.ctx, "pipeline:"+def.ID)
		}

		return nil
	}
}

// ── Pipeline Execution ──

// RunPipeline executes a pipeline's stages respecting their DAG dependencies.
func (s *UnifiedScheduler) RunPipeline(p *Pipeline) error {
	logger := NewStructuredLogger(NewTraceID(KindPipeline, p.Name, 0), p.Name, KindPipeline)
	logger.Info("pipeline_started", map[string]any{"stages": len(p.Stages)})

	// Build dependency graph and execute in topological order
	completed := make(map[string]bool)
	failed := make(map[string]error)

	// Simple BFS: execute stages with all deps satisfied
	remaining := make([]PipelineStage, len(p.Stages))
	copy(remaining, p.Stages)

	for len(remaining) > 0 {
		// Collect stages whose dependencies are all met
		ready := make([]int, 0)
		for i, stage := range remaining {
			depsMet := true
			for _, dep := range stage.DependsOn {
				if !completed[dep] {
					depsMet = false
					break
				}
			}
			if depsMet {
				ready = append(ready, i)
			}
		}

		if len(ready) == 0 && len(remaining) > 0 {
			// Circular dependency or all remaining depend on failed stages
			break
		}

		// Execute ready stages concurrently
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, idx := range ready {
			wg.Add(1)
			go func(stage PipelineStage) {
				defer wg.Done()
				stageLogger := NewStructuredLogger(NewTraceID(KindPipeline, p.Name+"."+stage.Name, 0), p.Name, KindPipeline)
				stageLogger.Info("stage_started", map[string]any{"stage": stage.Name})

				var err error
				for attempt := 0; attempt <= stage.Retries; attempt++ {
					err = stage.Handler(s.ctx, stageLogger)
					if err == nil {
						break
					}
					if attempt < stage.Retries {
						time.Sleep(time.Duration(attempt+1) * time.Second)
					}
				}

				mu.Lock()
				if err != nil {
					failed[stage.Name] = err
					stageLogger.Error("stage_failed", err, map[string]any{"stage": stage.Name})
					s.emitEvent(Event{
						ID:        NewEventID(EventPhaseFailed, stage.Name),
						Type:      EventPhaseFailed,
						Key:       stage.Name,
						Timestamp: time.Now(),
						Payload:   map[string]any{"error": err.Error()},
					})
				} else {
					completed[stage.Name] = true
					stageLogger.Info("stage_completed", map[string]any{"stage": stage.Name})
					s.emitEvent(Event{
						ID:        NewEventID(EventPhaseCompleted, stage.Name),
						Type:      EventPhaseCompleted,
						Key:       stage.Name,
						Timestamp: time.Now(),
					})
				}
				mu.Unlock()
			}(remaining[idx])
		}
		wg.Wait()

		// Remove completed/failed stages
		newRemaining := make([]PipelineStage, 0)
		skipSet := make(map[int]bool)
		for _, idx := range ready {
			skipSet[idx] = true
		}
		for i, stage := range remaining {
			if !skipSet[i] {
				newRemaining = append(newRemaining, stage)
			}
		}
		remaining = newRemaining
	}

	if len(failed) > 0 {
		logger.Error("pipeline_failed", fmt.Errorf("%d stages failed", len(failed)), map[string]any{"failed_stages": len(failed)})
		if p.OnError != "" {
			s.emitEvent(Event{
				ID:        NewEventID(p.OnError, p.Name),
				Type:      p.OnError,
				Key:       p.Name,
				Timestamp: time.Now(),
			})
		}
		return fmt.Errorf("pipeline %s: %d stages failed", p.Name, len(failed))
	}

	logger.Info("pipeline_completed", map[string]any{"stages": len(completed)})
	if p.OnComplete != "" {
		s.emitEvent(Event{
			ID:        NewEventID(p.OnComplete, p.Name),
			Type:      p.OnComplete,
			Key:       p.Name,
			Timestamp: time.Now(),
		})
	}
	return nil
}


// SyncRunCron updates cron triggers on per-run TaskInstances.
// SyncRunCron updates cron triggers on per-run TaskInstances.
// normalizeCron converts user-friendly time formats to proper cron expressions.
// Supported inputs:
//   "HH:MM" → "0 MM HH * * 1-5" (weekdays at HH:MM)
//   Full cron with 5-6 fields → passed through
// Returns empty string if input is empty or invalid.
func normalizeCron(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	// Already a full cron expression (at least 5 space-separated fields)
	if strings.Count(input, " ") >= 4 {
		// Try to parse it to verify
		if _, err := cronParser.Parse(input); err == nil {
			return input
		}
		log.Printf("[scheduler-v2] invalid cron expression: %q, treating as time format", input)
	}
	// Try "HH:MM" format
	parts := strings.SplitN(input, ":", 2)
	if len(parts) == 2 {
		hour := strings.TrimSpace(parts[0])
		minute := strings.TrimSpace(parts[1])
		// Basic validation
		if len(hour) > 0 && len(minute) > 0 {
			return fmt.Sprintf("0 %s %s * * 1-5", minute, hour)
		}
	}
	log.Printf("[scheduler-v2] cannot normalize cron: %q", input)
	return ""
}

// SyncRunCron updates cron triggers on per-run TaskInstances.
func (s *UnifiedScheduler) SyncRunCron(runID uint, dailyCron, tradeExecCron string) {
	// Normalize user-friendly time formats to proper cron expressions
	dailyCron = normalizeCron(dailyCron)
	tradeExecCron = normalizeCron(tradeExecCron)

	s.mu.Lock()
	defer s.mu.Unlock()

	owner := ResourceRef{Kind: "strategy_run", ID: runID}
	ownerKey := owner.Kind + ":" + fmt.Sprint(owner.ID)
	idSet := s.byOwner[ownerKey]

	for id := range idSet {
		inst, ok := s.instances[id]
		if !ok {
			continue
		}
		if inst.DefinitionID == "live_daily_run" && dailyCron != "" {
			inst.Trigger.Cron = dailyCron
			inst.Label = fmt.Sprintf("策略执行 #%d", runID)
			inst.NextRunAt = nil       // reset so next evaluation picks up new cron
			inst.LastRunAt = nil       // reset so first run uses grace window
			inst.UpdatedAt = time.Now()
			log.Printf("[scheduler-v2] SyncRunCron: updated live_daily_run cron for run %d → %s", runID, dailyCron)
		}
		if inst.DefinitionID == "live_trade_exec" && tradeExecCron != "" {
			inst.Trigger.Cron = tradeExecCron
			inst.Label = fmt.Sprintf("交易执行 #%d", runID)
			inst.NextRunAt = nil       // reset so next evaluation picks up new cron
			inst.LastRunAt = nil       // reset so first run uses grace window
			inst.UpdatedAt = time.Now()
			log.Printf("[scheduler-v2] SyncRunCron: updated live_trade_exec cron for run %d → %s", runID, tradeExecCron)
		}
	}

	if len(idSet) == 0 && (dailyCron != "" || tradeExecCron != "") {
		log.Printf("[scheduler-v2] SyncRunCron: no existing instances for run %d, creating new", runID)
		// Release lock before calling RegisterInstance (which also acquires lock)
		s.mu.Unlock()
		if dailyCron != "" {
			s.RegisterInstance(&TaskInstance{
				DefinitionID: "live_daily_run",
				BizTaskID:    runID,
				Label:        fmt.Sprintf("策略执行 #%d", runID),
				Owner:        owner,
				Trigger:      TriggerSpec{Cron: dailyCron, TradingDay: true, MinInterval: "10m"},
				Enabled:      true,
			})
		}
		if tradeExecCron != "" {
			s.RegisterInstance(&TaskInstance{
				DefinitionID: "live_trade_exec",
				BizTaskID:    runID,
				Label:        fmt.Sprintf("交易执行 #%d", runID),
				Owner:        owner,
				Trigger:      TriggerSpec{Cron: tradeExecCron, TradingDay: true, MinInterval: "10m"},
				Enabled:      true,
			})
		}
		s.RegisterInstance(&TaskInstance{
			DefinitionID: "live_position_patrol",
			BizTaskID:    runID,
			Label:        fmt.Sprintf("持仓巡检 #%d", runID),
			Owner:        owner,
			Trigger:      TriggerSpec{Cron: "0 */30 9-15 * * 1-5", TradingDay: true, MinInterval: "5m"},
			Enabled:      true,
		})
		s.mu.Lock()
	}
}

// ── Per-StrategyRun Task Management ──

// RestoreLiveTradingTasks reads all active strategy runs from DB and re-creates
// their per-run TaskInstances. Called on server startup to recover from restarts.
func (s *UnifiedScheduler) RestoreLiveTradingTasks() {
	type runInfo struct {
		ID            uint
		UserID        uint
		AutoDailyCron string
	}
	var runs []runInfo
	if err := db.MySQL.Table("strategy_runs").Where("status IN ?", []string{"active", "paused"}).
		Select("id, user_id, auto_daily_cron").Scan(&runs).Error; err != nil {
		log.Printf("[scheduler-v2] RestoreLiveTradingTasks: query failed: %v", err)
		return
	}
	if len(runs) == 0 {
		log.Printf("[scheduler-v2] RestoreLiveTradingTasks: no active strategy runs found")
		return
	}

	restored := 0
	for _, run := range runs {
		dailyCron := normalizeCron(run.AutoDailyCron)
		tradeExecCron := normalizeCron("")
		if dailyCron == "" {
			dailyCron = "0 10 16 * * 1-5" // default
		}
		if tradeExecCron == "" {
			tradeExecCron = "0 30 8 * * 1-5" // default
		}

		// Check if instances already exist for this run
		owner := ResourceRef{Kind: "strategy_run", ID: run.ID}
		existing := s.ListInstances("", &owner)
		if len(existing) > 0 {
			// Already restored, skip
			continue
		}

		s.SyncRunCron(run.ID, dailyCron, tradeExecCron)
		restored++
	}
	log.Printf("[scheduler-v2] RestoreLiveTradingTasks: restored %d runs", restored)
}

// ── Per-StrategyRun Task Management ──

// RegisterStrategyRunTasks creates per-run TaskInstance entries for live trading.
// Called when a new StrategyRun is created or activated.
// Reads user-configured cron from StrategyRun.AutoDailyCron / AutoPreMarketCron.
func (s *UnifiedScheduler) RegisterStrategyRunTasks(runID uint, userID uint) {
	// Read user-configured crons from DB
	dailyCron := "0 10 16 * * 1-5"   // default
	tradeExecCron := "0 30 8 * * 1-5" // default

	var run struct {
		AutoDailyCron     string
		AutoPreMarketCron string
	}
	if err := db.MySQL.Table("strategy_runs").Where("id = ?", runID).
		Select("auto_daily_cron").Scan(&run).Error; err == nil {
		if run.AutoDailyCron != "" {
			dailyCron = run.AutoDailyCron
		}
	}

	defs := []struct {
		DefID      string
		Cron       string
		Label      string
		ReqAll     []string
		MinInterval string
	}{
		{"live_daily_run", dailyCron, fmt.Sprintf("策略执行 #%d", runID), []string{"data_ready"}, "10m"},
		{"live_trade_exec", tradeExecCron, fmt.Sprintf("交易执行 #%d", runID), nil, "10m"},
		{"live_position_patrol", "0 */30 9-15 * * 1-5", fmt.Sprintf("持仓巡检 #%d", runID), nil, "5m"},
		{"live_snapshot", "0 5 15 * * 1-5", fmt.Sprintf("盘后快照 #%d", runID), nil, "10m"},
	}

	owner := ResourceRef{Kind: "strategy_run", ID: runID}
	for _, d := range defs {
		inst := &TaskInstance{
			DefinitionID: d.DefID,
			BizTaskID:    runID, // bind to strategy run
			Label:        d.Label, // per-instance display name
			Owner:        owner,
			UserID:       userID,
			Trigger: TriggerSpec{
				Cron:        d.Cron,
				TradingDay:  true,
				RequireAll:  d.ReqAll,
				MinInterval: d.MinInterval,
			},
			Enabled: true,
			Params:  map[string]any{"runID": runID},
		}
		if err := s.RegisterInstance(inst); err != nil {
			log.Printf("[scheduler-v2] RegisterStrategyRunTasks: %v", err)
		}
	}
	log.Printf("[scheduler-v2] registered %d live trading tasks for run #%d (daily=%s pre_market=%s)",
		len(defs), runID, dailyCron, tradeExecCron)
}

// DisableStrategyRunTasks disables all per-run TaskInstances for a given run.
func (s *UnifiedScheduler) DisableStrategyRunTasks(runID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	owner := ResourceRef{Kind: "strategy_run", ID: runID}
	instances := s.findByOwner(owner)
	for _, inst := range instances {
		inst.Enabled = false
		inst.UpdatedAt = time.Now()
	}
	log.Printf("[scheduler-v2] disabled %d tasks for run #%d", len(instances), runID)
}

// EnableStrategyRunTasks enables all per-run TaskInstances for a given run.
func (s *UnifiedScheduler) EnableStrategyRunTasks(runID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	owner := ResourceRef{Kind: "strategy_run", ID: runID}
	instances := s.findByOwner(owner)
	for _, inst := range instances {
		inst.Enabled = true
		inst.UpdatedAt = time.Now()
	}
	log.Printf("[scheduler-v2] enabled %d tasks for run #%d", len(instances), runID)
}

func (s *UnifiedScheduler) findByOwner(owner ResourceRef) []*TaskInstance {
	ownerKey := owner.Kind + ":" + fmt.Sprint(owner.ID)
	idSet := s.byOwner[ownerKey]
	var result []*TaskInstance
	for id := range idSet {
		if inst, ok := s.instances[id]; ok {
			result = append(result, inst)
		}
	}
	return result
}

// ── Health ──

// Health returns the current scheduler health status.
func (s *UnifiedScheduler) Health() SchedulerHealth {
	queueStats := s.queue.Stats()
	busyWorkers := s.pool.BusyWorkers()

	s.alertsMu.Lock()
	alerts := make([]HealthAlert, len(s.alerts))
	copy(alerts, s.alerts)
	s.alertsMu.Unlock()

	// Determine overall status
	status := "healthy"
	avgBusyPct := float64(busyWorkers) / float64(s.pool.Total()) * 100
	if avgBusyPct > 90 {
		status = "degraded"
	}
	for _, q := range queueStats {
		if q.Depth > 20 || q.Oldest > 5*time.Minute {
			status = "degraded"
		}
	}
	if len(alerts) > 0 {
		hasCritical := false
		for _, a := range alerts {
			if a.Level == "critical" {
				hasCritical = true
				break
			}
		}
		if hasCritical {
			status = "unhealthy"
		}
	}

	return SchedulerHealth{
		Status: status,
		Uptime: time.Since(s.started),
		Workers: WorkerHealth{
			Total:      s.pool.Total(),
			Busy:       busyWorkers,
			Idle:       s.pool.IdleWorkers(),
			AvgBusyPct: avgBusyPct,
		},
		Queues:       queueStats,
		ActiveAlerts: alerts,
	}
}

// AddAlert appends a health alert.
func (s *UnifiedScheduler) AddAlert(level, message string) {
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()

	s.alerts = append(s.alerts, HealthAlert{
		Level:   level,
		Message: message,
		Since:   time.Now(),
	})
	// Keep only last 100 alerts
	if len(s.alerts) > 100 {
		s.alerts = s.alerts[len(s.alerts)-100:]
	}
}

// ClearAlerts removes all alerts.
func (s *UnifiedScheduler) ClearAlerts() {
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()
	s.alerts = nil
}

// ── TaskInstance lifecycle helpers ──

// NowRunning marks an instance as running and updates its timestamp.
func (i *TaskInstance) NowRunning() {
	i.LastStatus = "running"
	i.UpdatedAt = time.Now()
}

// ── Helpers ──

func parseTimeOfDay(base time.Time, hhmm string) (time.Time, error) {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		return time.Time{}, err
	}
	return time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, base.Location()), nil
}

func isTradingDay(t time.Time) bool {
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	return true
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	s := ss[0]
	for i := 1; i < len(ss); i++ {
		s += ", " + ss[i]
	}
	return s
}

// storeHas checks if a key exists in the store (ignoring TTL).
func (s *UnifiedScheduler) storeHas(ctx context.Context, key string) bool {
	err := s.store.Get(ctx, key, &struct{}{})
	return err == nil
}

// emitEvent publishes an event and persists it in the store for trigger evaluation.
func (s *UnifiedScheduler) emitEvent(event Event) {
	// Store for trigger evaluation (1 hour TTL for data_ready, 10 min for others)
	ttl := 10 * time.Minute
	if event.Type == EventDataReady || event.Type == "morning_partial" || event.Type == "morning_ready" {
		ttl = 1 * time.Hour
	}
	eventKey := formatEventKey(event.Type, event.Key)
	s.store.Set(s.ctx, "event:"+eventKey, event.Timestamp, ttl)

	// Publish for live subscribers
	s.events.Publish(s.ctx, event)
}

// ── Execution History ──

// TaskRunRecord stores a single task execution record.
type TaskRunRecord struct {
	ID           string        `json:"id"`
	TraceID      string        `json:"traceId"`
	DefinitionID string        `json:"definitionId"`
	Kind         TaskKind      `json:"kind"`
	InstanceID   uint          `json:"instanceId,omitempty"`
	BizTaskID    uint          `json:"bizTaskId,omitempty"`
	Label        string        `json:"label,omitempty"`
	Status       string        `json:"status"`
	StartedAt    time.Time     `json:"startedAt"`
	FinishedAt   time.Time     `json:"finishedAt"`
	Duration     time.Duration `json:"duration"`
	ErrorMsg     string        `json:"errorMsg,omitempty"`
}

var (
	historyMu      sync.RWMutex
	historyRecords []TaskRunRecord
	maxHistorySize = 500
)

// RecordExecution appends a task execution record.
func (s *UnifiedScheduler) RecordExecution(defID string, instID uint, bizTaskID uint, label string, status string, startedAt, finishedAt time.Time, errMsg string) {
	rec := TaskRunRecord{
		ID:           fmt.Sprintf("rec:%s:%d:%d", defID, instID, time.Now().UnixNano()),
		TraceID:      NewTraceID(KindPipeline, defID, instID),
		DefinitionID: defID,
		InstanceID:   instID,
		BizTaskID:    bizTaskID,
		Label:        label,
		Status:       status,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		Duration:     finishedAt.Sub(startedAt),
		ErrorMsg:     errMsg,
	}
	s.recordExecution(rec)
}

func (s *UnifiedScheduler) recordExecution(rec TaskRunRecord) {
	historyMu.Lock()
	defer historyMu.Unlock()

	historyRecords = append(historyRecords, rec)
	if len(historyRecords) > maxHistorySize {
		historyRecords = historyRecords[len(historyRecords)-maxHistorySize:]
	}

	if rec.Duration > 5*time.Minute {
		s.AddAlert("warning", fmt.Sprintf("慢任务: %s 耗时 %v", rec.DefinitionID, rec.Duration.Round(time.Second)))
	}
}

// GetTaskHistory returns recent task execution records.
func (s *UnifiedScheduler) GetTaskHistory(defID string, limit int) []TaskRunRecord {
	historyMu.RLock()
	defer historyMu.RUnlock()

	if limit <= 0 || limit > len(historyRecords) {
		limit = len(historyRecords)
	}

	var result []TaskRunRecord
	for i := len(historyRecords) - 1; i >= 0 && len(result) < limit; i-- {
		rec := historyRecords[i]
		if defID == "" || rec.DefinitionID == defID {
			result = append(result, rec)
		}
	}
	return result
}

// ── Distributed Execution Readiness ──

// DistributedConfig holds settings for distributed (multi-instance) deployment.
type DistributedConfig struct {
	// Enabled switches to distributed mode (requires Postgres or Redis backing).
	Enabled bool `json:"enabled"`

	// Backend: "postgres" or "redis" (future: "etcd").
	Backend string `json:"backend"`

	// InstanceID must be unique per process (e.g., pod name, hostname).
	InstanceID string `json:"instanceId"`

	// HeartbeatInterval is how often instances report liveness.
	HeartbeatInterval time.Duration `json:"heartbeatInterval"`

	// HeartbeatTTL is how long before a missing instance is considered dead.
	HeartbeatTTL time.Duration `json:"heartbeatTtl"`

	// LockTTL is the lease duration for leader election locks.
	LockTTL time.Duration `json:"lockTtl"`
}

// DefaultDistributedConfig returns sensible defaults for distributed mode.
func DefaultDistributedConfig() DistributedConfig {
	return DistributedConfig{
		Enabled:           false,
		Backend:           "postgres",
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTTL:      30 * time.Second,
		LockTTL:           15 * time.Second,
	}
}

// DistributedReadiness describes whether the instance is ready for distributed execution.
type DistributedReadiness struct {
	Mode             string `json:"mode"`
	InstanceID       string `json:"instanceId"`
	BackendConnected bool   `json:"backendConnected"`
	LeaderElectionOK bool   `json:"leaderElectionOk"`
	QueueBackendOK   bool   `json:"queueBackendOk"`
	EventBusOK       bool   `json:"eventBusOk"`
	StateStoreOK     bool   `json:"stateStoreOk"`
	ActiveInstances  int    `json:"activeInstances"`
	IsLeader         bool   `json:"isLeader"`
}

// DistributedReadinessCheck verifies that all distributed components are functional.
// In standalone mode, always returns healthy.
func (s *UnifiedScheduler) DistributedReadinessCheck() DistributedReadiness {
	if s.cfg.Mode != "distributed" {
		return DistributedReadiness{
			Mode:       "standalone",
			InstanceID: s.cfg.InstanceID,
		}
	}

	return DistributedReadiness{
		Mode:             "distributed",
		InstanceID:       s.cfg.InstanceID,
		BackendConnected: s.leader.IsConnected(),
		LeaderElectionOK: s.leader.IsActive(),
		QueueBackendOK:   s.queue.IsConnected(),
		EventBusOK:       s.events.IsConnected(),
		StateStoreOK:     s.store.IsConnected(),
		IsLeader:         s.leader.IsLeader(),
	}
}
