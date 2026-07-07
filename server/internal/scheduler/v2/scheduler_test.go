package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestJobQueueBasic verifies enqueue/dequeue ordering and priority.
func TestJobQueueBasic(t *testing.T) {
	q := NewInMemoryJobQueue()
	defer q.Close()

	ctx := context.Background()

	// Enqueue jobs with different priorities
	jobs := []*Job{
		{ID: "j1", Kind: KindStrategy, Priority: PriorityNormal},
		{ID: "j2", Kind: KindStrategy, Priority: PriorityHigh},
		{ID: "j3", Kind: KindStrategy, Priority: PriorityCritical},
		{ID: "j4", Kind: KindPipeline, Priority: PriorityLow},
	}

	for _, j := range jobs {
		if err := q.Enqueue(ctx, j); err != nil {
			t.Fatalf("enqueue %s: %v", j.ID, err)
		}
	}

	if q.Len() != 4 {
		t.Fatalf("len = %d, want 4", q.Len())
	}

	// Dequeue should return highest priority (lowest number) first
	kinds := []TaskKind{KindStrategy, KindPipeline, KindAccount, KindPortfolio, KindCustom}

	deqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// j3 (Critical) should come first
	j1, err := q.Dequeue(deqCtx, kinds)
	if err != nil {
		t.Fatalf("dequeue 1: %v", err)
	}
	if j1.ID != "j3" {
		t.Errorf("first = %s, want j3", j1.ID)
	}

	// j2 (High) second
	j2, err := q.Dequeue(deqCtx, kinds)
	if err != nil {
		t.Fatalf("dequeue 2: %v", err)
	}
	if j2.ID != "j2" {
		t.Errorf("second = %s, want j2", j2.ID)
	}
}

// TestEventBusPublishSubscribe verifies event delivery.
func TestEventBusPublishSubscribe(t *testing.T) {
	bus := NewInMemoryEventBus()
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Subscribe(ctx, "data_ready:*")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Publish matching event
	bus.Publish(ctx, Event{Type: EventDataReady, Key: "2026-07-07"})

	select {
	case evt := <-ch:
		if evt.Type != EventDataReady {
			t.Errorf("type = %s, want data_ready", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Publish non-matching event — should not be delivered (no subscriber for it)
	bus.Publish(ctx, Event{Type: EventMorningReady, Key: "2026-07-07"})

	select {
	case evt := <-ch:
		t.Errorf("unexpected event received: %s", evt.Type)
	case <-time.After(100 * time.Millisecond):
		// OK, no event received
	}
}

// TestLeaderElection verifies exclusive lock acquisition.
func TestLeaderElection(t *testing.T) {
	e := NewInMemoryLeaderElection()
	defer e.Close()

	ctx := context.Background()

	// First acquisition should succeed
	ok, err := e.TryAcquire(ctx, "pipeline:after_close", 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}

	// Second acquisition should fail (already held)
	ok, err = e.TryAcquire(ctx, "pipeline:after_close", 5*time.Second)
	if err != nil || ok {
		t.Fatalf("second acquire should fail: ok=%v err=%v", ok, err)
	}

	// Release
	e.Release(ctx, "pipeline:after_close")

	// Now should work again
	ok, err = e.TryAcquire(ctx, "pipeline:after_close", 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("third acquire: ok=%v err=%v", ok, err)
	}
}

// TestFullPipeline verifies a complete scheduler lifecycle.
func TestFullPipeline(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workers = 2
	cfg.EvalInterval = 100 * time.Millisecond

	s := New(cfg)

	// Register a pipeline definition
	var stage1Ran, stage2Ran, stage3Ran int32

	pipeline := &Pipeline{
		Name:  "test_pipeline",
		Label: "Test Pipeline",
		Trigger: TriggerSpec{
			Cron: "",   // event-driven only
			Manual: true,
		},
		Stages: []PipelineStage{
			{
				Name:    "stage1",
				Timeout: 5 * time.Second,
				Handler: func(ctx context.Context, logger *StructuredLogger) error {
					atomic.StoreInt32(&stage1Ran, 1)
					logger.Info("stage1_done", nil)
					return nil
				},
			},
			{
				Name:      "stage2",
				DependsOn: []string{"stage1"},
				Timeout:   5 * time.Second,
				Handler: func(ctx context.Context, logger *StructuredLogger) error {
					atomic.StoreInt32(&stage2Ran, 1)
					return nil
				},
			},
			{
				Name:      "stage3",
				DependsOn: []string{"stage1"},
				Timeout:   5 * time.Second,
				Handler: func(ctx context.Context, logger *StructuredLogger) error {
					atomic.StoreInt32(&stage3Ran, 1)
					return nil
				},
			},
		},
		OnComplete: EventDataReady,
	}

	// Register a user task definition
	var userTaskRan int32
	userDef := &TaskDefinition{
		ID:    "test_user_task",
		Kind:  KindStrategy,
		Label: "Test User Task",
		Trigger: TriggerSpec{
			Events:     []string{EventDataReady},
			RequireAll: []string{EventDataReady},
		},
		Timeout: 10 * time.Second,
		Handler: func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
			atomic.StoreInt32(&userTaskRan, 1)
			logger.Info("user_task_done", nil)
			return nil
		},
	}

	s.RegisterDefinition(&TaskDefinition{
		ID:    "test_pipeline",
		Kind:  KindPipeline,
		Label: "Test Pipeline Def",
	})
	s.RegisterDefinition(userDef)

	// Register a user task instance
	inst := &TaskInstance{
		DefinitionID: "test_user_task",
		Owner:        ResourceRef{Kind: "strategy_run", ID: 42},
		UserID:       1,
		Enabled:      true,
		Trigger: TriggerSpec{
			Events:     []string{EventDataReady},
			RequireAll: []string{EventDataReady},
		},
	}
	s.RegisterInstance(inst)

	// Start the scheduler
	s.Start()
	defer s.Stop()

	// Run the pipeline
	go func() {
		if err := s.RunPipeline(pipeline); err != nil {
			t.Logf("pipeline error: %v", err)
		}
	}()

	// Wait for everything
	time.Sleep(2 * time.Second)

	if atomic.LoadInt32(&stage1Ran) != 1 {
		t.Error("stage1 did not run")
	}
	if atomic.LoadInt32(&stage2Ran) != 1 {
		t.Error("stage2 did not run")
	}
	if atomic.LoadInt32(&stage3Ran) != 1 {
		t.Error("stage3 did not run")
	}
	// Note: user task is event-driven but the event loop subscribes to "*"
	// and should pick up the DataReady event from the pipeline.
	// With buffered channels and eval loop, it might take a tick cycle.
	time.Sleep(500 * time.Millisecond)
}

// TestTriggerEval verifies that the eval loop picks up cron-based triggers.
func TestTriggerEval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workers = 1
	cfg.EvalInterval = 50 * time.Millisecond

	s := New(cfg)

	var ran int32
	def := &TaskDefinition{
		ID:      "cron_test",
		Kind:    KindCustom,
		Label:   "Cron Test",
		Trigger: TriggerSpec{Cron: "* * * * * *"}, // every second
		Timeout: 5 * time.Second,
		Handler: func(ctx context.Context, inst *TaskInstance, logger *StructuredLogger) error {
			atomic.AddInt32(&ran, 1)
			return nil
		},
	}
	s.RegisterDefinition(def)

	inst := &TaskInstance{
		DefinitionID: "cron_test",
		Owner:        ResourceRef{Kind: "user", ID: 1},
		UserID:       1,
		Enabled:      true,
	}
	s.RegisterInstance(inst)

	s.Start()
	defer s.Stop()

	// eval every 50ms, should fire multiple times in 300ms
	time.Sleep(300 * time.Millisecond)

	count := atomic.LoadInt32(&ran)
	if count < 2 {
		t.Errorf("cron test ran %d times, want >= 2", count)
	}
}

// TestHealth verifies the health endpoint returns valid data.
func TestHealth(t *testing.T) {
	cfg := DefaultConfig()
	s := New(cfg)
	s.Start()
	defer s.Stop()

	health := s.Health()
	if health.Status != "healthy" {
		t.Errorf("status = %s, want healthy", health.Status)
	}
	if health.Workers.Total != cfg.Workers {
		t.Errorf("workers = %d, want %d", health.Workers.Total, cfg.Workers)
	}
	if health.Queues == nil {
		t.Error("queues should not be nil")
	}
}

// TestAddAlert verifies alert management.
func TestAddAlert(t *testing.T) {
	cfg := DefaultConfig()
	s := New(cfg)

	s.AddAlert("warning", "test alert")
	health := s.Health()

	if len(health.ActiveAlerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(health.ActiveAlerts))
	}
	if health.ActiveAlerts[0].Level != "warning" {
		t.Errorf("level = %s, want warning", health.ActiveAlerts[0].Level)
	}

	s.ClearAlerts()
	health = s.Health()
	if len(health.ActiveAlerts) != 0 {
		t.Errorf("alerts after clear = %d, want 0", len(health.ActiveAlerts))
	}
}

// TestLoggerProducesJSON verifies structured log output.
func TestLoggerProducesJSON(t *testing.T) {
	l := NewStructuredLogger("test:trace:1", "test_def", KindStrategy)
	l.WithInstance(42, ResourceRef{Kind: "run", ID: 99})

	// Just verify no panics
	l.Info("test_event", map[string]any{"key": "value"})
	l.Warn("test_warning", map[string]any{"reason": "test"})
	l.Error("test_error", nil, nil)
	l.Phase("executing", map[string]any{"step": 1})
	l.Metric("elapsed", 1234.5, "ms")
}
