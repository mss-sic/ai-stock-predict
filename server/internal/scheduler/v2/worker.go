package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerPool manages a fixed number of goroutine workers that pull jobs from the queue.
type WorkerPool struct {
	queue   JobQueue
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc

	// Metrics
	busyWorkers int64 // atomic counter
	jobsDone    int64
	jobsFailed  int64
}

// NewWorkerPool creates a worker pool with n workers pulling from the given queue.
func NewWorkerPool(queue JobQueue, n int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		queue:   queue,
		workers: n,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start launches all workers. Each worker blocks on Dequeue.
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(i)
	}
}

// Stop gracefully shuts down workers by cancelling their context.
func (p *WorkerPool) Stop() {
	p.cancel()
	p.wg.Wait()
}

// BusyWorkers returns the number of currently executing workers.
func (p *WorkerPool) BusyWorkers() int {
	return int(atomic.LoadInt64(&p.busyWorkers))
}

// IdleWorkers returns workers - busy.
func (p *WorkerPool) IdleWorkers() int {
	return p.workers - p.BusyWorkers()
}

// Total returns the worker count.
func (p *WorkerPool) Total() int {
	return p.workers
}

// JobsDone returns total completed jobs.
func (p *WorkerPool) JobsDone() int64 {
	return atomic.LoadInt64(&p.jobsDone)
}

// JobsFailed returns total failed jobs.
func (p *WorkerPool) JobsFailed() int64 {
	return atomic.LoadInt64(&p.jobsFailed)
}

func (p *WorkerPool) runWorker(id int) {
	defer p.wg.Done()

	// Workers listen for all task kinds
	kinds := []TaskKind{KindPipeline, KindStrategy, KindAccount, KindPortfolio, KindCustom}

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		job, err := p.queue.Dequeue(p.ctx, kinds)
		if err != nil {
			// Queue closed or context cancelled
			return
		}

		atomic.AddInt64(&p.busyWorkers, 1)

		logger := NewStructuredLogger(job.TraceID, job.DefinitionID, job.Kind)
		logger.WithInstance(job.InstanceID, ResourceRef{}) // owner set by caller
		logger.Info("job_started", map[string]any{
			"worker_id":   id,
			"job_id":      job.ID,
			"queue_depth": p.queue.Len(),
		})

		// Execute with timeout
		execCtx := job.Context()
		cancel := func() {}
		if job.Timeout > 0 {
			execCtx, cancel = context.WithTimeout(p.ctx, job.Timeout)
		}
		execCtx = context.WithValue(execCtx, ctxKeyWorkerID, id)

		start := time.Now()
		err = job.Handler(execCtx, nil, logger) // instance will be passed by the handler itself
		cancel()

		elapsed := time.Since(start)
		atomic.AddInt64(&p.busyWorkers, -1)

		status := "success"
		errMsg := ""
		if err != nil {
			atomic.AddInt64(&p.jobsFailed, 1)
			status = "failed"
			errMsg = err.Error()
			logger.Error("job_failed", err, map[string]any{
				"duration_ms": elapsed.Milliseconds(),
				"worker_id":   id,
			})
		} else {
			atomic.AddInt64(&p.jobsDone, 1)
			logger.Info("job_completed", map[string]any{
				"duration_ms": elapsed.Milliseconds(),
				"worker_id":   id,
			})
		}

		// Record execution history
		if sched := GetGlobal(); sched != nil {
			bizTaskID := uint(0)
			label := ""
			if inst := sched.GetInstance(job.InstanceID); inst != nil {
				bizTaskID = inst.BizTaskID
				label = inst.Label
			}
			sched.RecordExecution(job.DefinitionID, job.InstanceID, bizTaskID, label, status, start, time.Now(), errMsg)
		}
	}
}

// ── Context helpers ──

type ctxKey string

const ctxKeyWorkerID ctxKey = "worker_id"
const ctxKeyScheduler ctxKey = "scheduler"

// WorkerIDFromContext returns the worker ID from a context.
func WorkerIDFromContext(ctx context.Context) int {
	id, _ := ctx.Value(ctxKeyWorkerID).(int)
	return id
}

// ── Job.Do helper ──

// Context returns a fresh context for the job's execution, with scheduler attached.
func (j *Job) Context() context.Context {
	return context.Background()
}
