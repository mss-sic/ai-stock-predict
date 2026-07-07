package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// ── Priority Queue (min-heap by Priority, then CreatedAt) ──

type jobHeap []*Job

func (h jobHeap) Len() int { return len(h) }
func (h jobHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].CreatedAt.Before(h[j].CreatedAt)
}
func (h jobHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *jobHeap) Push(x any)   { *h = append(*h, x.(*Job)) }
func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	j := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return j
}

// ── InMemoryJobQueue ──

// InMemoryJobQueue is a single-process, priority-ordered job queue.
// Uses a min-heap + channel-based signal for reliable blocking dequeue.
// Safe for concurrent use. Suitable for standalone mode.
type InMemoryJobQueue struct {
	mu      sync.Mutex
	heap    jobHeap
	pending map[string]*Job
	signal  chan struct{} // sent when a new job is enqueued
	closed  bool
	done    chan struct{}

	stats map[TaskKind]*queueStats
}

type queueStats struct {
	totalEnqueued  int64
	totalDequeued  int64
	waitTimes      []time.Duration
	waitIdx        int
}

func NewInMemoryJobQueue() *InMemoryJobQueue {
	q := &InMemoryJobQueue{
		heap:    make(jobHeap, 0),
		pending: make(map[string]*Job),
		signal:  make(chan struct{}, 1), // buffered to avoid blocking enqueue
		done:    make(chan struct{}),
		stats:   make(map[TaskKind]*queueStats),
	}
	heap.Init(&q.heap)
	return q
}

func (q *InMemoryJobQueue) getStats(kind TaskKind) *queueStats {
	s, ok := q.stats[kind]
	if !ok {
		s = &queueStats{waitTimes: make([]time.Duration, 60)}
		q.stats[kind] = s
	}
	return s
}

func (q *InMemoryJobQueue) signalNonBlocking() {
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

func (q *InMemoryJobQueue) Enqueue(ctx context.Context, job *Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue closed")
	}

	if _, exists := q.pending[job.ID]; exists {
		return fmt.Errorf("duplicate job: %s", job.ID)
	}

	job.CreatedAt = time.Now()
	q.pending[job.ID] = job

	if job.Priority == PriorityCritical {
		job.CreatedAt = time.Time{}
	}

	heap.Push(&q.heap, job)
	q.getStats(job.Kind).totalEnqueued++
	q.signalNonBlocking()

	return nil
}

func (q *InMemoryJobQueue) Dequeue(ctx context.Context, kinds []TaskKind) (*Job, error) {
	kindSet := make(map[TaskKind]bool, len(kinds))
	for _, k := range kinds {
		kindSet[k] = true
	}

	for {
		// Try to find a matching job
		q.mu.Lock()
		job := q.dequeueMatchingLocked(kindSet)
		if job != nil {
			q.mu.Unlock()
			return job, nil
		}

		if q.closed {
			q.mu.Unlock()
			return nil, fmt.Errorf("queue closed")
		}
		q.mu.Unlock()

		// Block until a new job arrives or context is cancelled
		select {
		case <-q.signal:
			// A job was enqueued, retry
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-q.done:
			return nil, fmt.Errorf("queue closed")
		}
	}
}

func (q *InMemoryJobQueue) dequeueMatchingLocked(kindSet map[TaskKind]bool) *Job {
	for i, j := range q.heap {
		if kindSet[j.Kind] && q.pending[j.ID] != nil {
			job := heap.Remove(&q.heap, i).(*Job)
			delete(q.pending, job.ID)

			s := q.getStats(job.Kind)
			s.totalDequeued++
			waitTime := time.Since(job.CreatedAt)
			s.waitTimes[s.waitIdx%len(s.waitTimes)] = waitTime
			s.waitIdx++

			return job
		}
	}
	return nil
}

func (q *InMemoryJobQueue) TryDequeue(kinds []TaskKind) *Job {
	q.mu.Lock()
	defer q.mu.Unlock()

	kindSet := make(map[TaskKind]bool, len(kinds))
	for _, k := range kinds {
		kindSet[k] = true
	}
	return q.dequeueMatchingLocked(kindSet)
}

func (q *InMemoryJobQueue) Ack(ctx context.Context, jobID string) error {
	// Job already removed from pending during dequeue
	return nil
}

func (q *InMemoryJobQueue) Nack(ctx context.Context, jobID string, retryAfter time.Duration) error {
	// For in-memory: caller should enqueue a new job with incremented retry count
	return nil
}

func (q *InMemoryJobQueue) Stats() map[TaskKind]QueueHealth {
	q.mu.Lock()
	defer q.mu.Unlock()

	result := make(map[TaskKind]QueueHealth)
	counts := make(map[TaskKind]int)
	oldestTimes := make(map[TaskKind]time.Time)

	for _, j := range q.pending {
		counts[j.Kind]++
		if t, ok := oldestTimes[j.Kind]; !ok || j.CreatedAt.Before(t) {
			oldestTimes[j.Kind] = j.CreatedAt
		}
	}

	for kind, count := range counts {
		h := QueueHealth{Depth: count}
		if t, ok := oldestTimes[kind]; ok {
			h.Oldest = time.Since(t)
		}
		if s, ok := q.stats[kind]; ok {
			var sum time.Duration
			n := 0
			for _, wt := range s.waitTimes {
				if wt > 0 {
					sum += wt
					n++
				}
			}
			if n > 0 {
				h.AvgWait = sum / time.Duration(n)
			}
		}
		result[kind] = h
	}
	return result
}

func (q *InMemoryJobQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.heap)
}

func (q *InMemoryJobQueue) Close() error {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	close(q.done)
	return nil
}

func (q *InMemoryJobQueue) IsConnected() bool { return true }
