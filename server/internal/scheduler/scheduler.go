package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron    *cron.Cron
	mu      sync.Mutex
	running bool
	lastRun time.Time
	status  string
}

func New(cronExpr string) *Scheduler {
	s := &Scheduler{status: "idle"}
	s.cron = cron.New(cron.WithSeconds())
	s.cron.AddFunc(cronExpr, func() {
		s.mu.Lock()
		if s.running {
			s.mu.Unlock()
			log.Println("[scheduler] previous job still running, skipping")
			return
		}
		s.running = true
		s.status = "running"
		s.mu.Unlock()

		collector.RunFullCollection()

		s.mu.Lock()
		s.running = false
		s.lastRun = time.Now()
		s.status = "idle"
		s.mu.Unlock()
	})
	return s
}

func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("[scheduler] started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("[scheduler] stopped")
}

func (s *Scheduler) Trigger() {
	go func() {
		log.Println("[scheduler] manual trigger")
		collector.RunFullCollection()
		s.mu.Lock()
		s.lastRun = time.Now()
		s.mu.Unlock()
	}()
}

func (s *Scheduler) Status() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{
		"running": s.running,
		"lastRun": s.lastRun,
		"status":  s.status,
	}
}
