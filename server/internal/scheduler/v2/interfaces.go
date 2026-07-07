package scheduler

import (
	"context"
	"time"
)

// JobQueue manages pending job execution with priority ordering.
type JobQueue interface {
	// Enqueue adds a job to the queue. Returns immediately; does not block.
	Enqueue(ctx context.Context, job *Job) error

	// Dequeue blocks until a job matching any of the given kinds is available,
	// or the context is cancelled. Returns the dequeued job.
	Dequeue(ctx context.Context, kinds []TaskKind) (*Job, error)

	// TryDequeue is non-blocking; returns nil if no job is available.
	TryDequeue(kinds []TaskKind) *Job

	// Ack confirms successful completion and removes the job from tracking.
	Ack(ctx context.Context, jobID string) error

	// Nack signals failure and optionally re-enqueues after a delay.
	Nack(ctx context.Context, jobID string, retryAfter time.Duration) error

	// Stats returns per-kind queue depth and timing metrics.
	Stats() map[TaskKind]QueueHealth

	// Len returns the total number of pending jobs across all kinds.
	Len() int

	// Close shuts down the queue; any blocked Dequeue calls return.
	Close() error

	// IsConnected returns whether the queue backend is reachable.
	IsConnected() bool
}

// EventBus provides publish/subscribe for internal scheduler events.
type EventBus interface {
	// Publish sends an event to all subscribers matching the pattern.
	Publish(ctx context.Context, event Event) error

	// Subscribe returns a channel that receives events matching the pattern.
	// Pattern format: "data_ready:*" / "task_completed:live_daily_run:*" / "*"
	Subscribe(ctx context.Context, pattern string) (<-chan Event, error)

	// Unsubscribe removes a subscription channel.
	Unsubscribe(ch <-chan Event)

	// Close shuts down the event bus.
	Close() error

	// IsConnected returns whether the event bus backend is reachable.
	IsConnected() bool
}

// LeaderElection ensures at-most-one execution of singleton tasks across instances.
type LeaderElection interface {
	// TryAcquire attempts to acquire a lease for the given key.
	// Returns true if acquired, false if already held by another instance.
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Renew extends the TTL of a held lease.
	Renew(ctx context.Context, key string, ttl time.Duration) error

	// Release immediately releases a held lease.
	Release(ctx context.Context, key string) error

	// IsHeld returns whether this instance currently holds the lease.
	IsHeld(key string) bool

	// Close releases all held leases and shuts down.
	Close() error

	// IsConnected returns whether the leader election backend is reachable.
	IsConnected() bool

	// IsActive returns whether the leader election mechanism is functioning.
	IsActive() bool

	// IsLeader returns whether this instance is the current leader.
	IsLeader() bool
}

// StateStore provides shared state access for the scheduler.
type StateStore interface {
	// Get retrieves a value by key.
	Get(ctx context.Context, key string, dest any) error

	// Set stores a value with an optional TTL.
	Set(ctx context.Context, key string, val any, ttl time.Duration) error

	// Delete removes a key.
	Delete(ctx context.Context, key string) error

	// List returns all keys matching a prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Close shuts down the store.
	Close() error

	// IsConnected returns whether the state store backend is reachable.
	IsConnected() bool
}

// ── Pipeline interfaces ──

// PipelineStage represents a single stage in a collection pipeline.
type PipelineStage struct {
	Name      string
	Handler   func(ctx context.Context, logger *StructuredLogger) error
	DependsOn []string                   // stage names this stage depends on
	Timeout   time.Duration
	Retries   int
	Priority  Priority
}

// Pipeline is a system-level collection pipeline with a DAG of stages.
type Pipeline struct {
	Name       string
	Label      string
	Trigger    TriggerSpec
	Stages     []PipelineStage
	OnComplete string // event to emit on success, e.g. EventDataReady
	OnError    string // event to emit on failure
}
