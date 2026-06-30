package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/service"
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

		collector.RunManualCollection(nil)

		s.mu.Lock()
		s.running = false
		s.lastRun = time.Now()
		s.status = "idle"
		s.mu.Unlock()
	})

	// Risk scan: every hour at minute 5
	s.cron.AddFunc("0 5 * * * *", func() {
		count, err := service.ScanUserHoldings()
		if err != nil {
			log.Printf("[scheduler] risk scan error: %v", err)
		} else {
			log.Printf("[scheduler] risk scan: %d alerts", count)
		}
	})

	// Sentiment computation: after market close every trading day (Mon-Fri 15:45)
	s.cron.AddFunc("0 45 15 * * 1-5", func() {
		log.Println("[scheduler] computing market sentiment...")
		collector.RunSentimentComputation()
	})

	// Northbound flow collection: after market close Mon-Fri 15:30
	s.cron.AddFunc("0 30 15 * * 1-5", func() {
		log.Println("[scheduler] collecting northbound flow...")
		collector.RunManualCollection([]string{"northbound"})
	})

	// Limit stats pre-computation: after market close Mon-Fri 16:00
	s.cron.AddFunc("0 0 16 * * 1-5", func() {
		log.Println("[scheduler] computing limit stats...")
		collector.RunManualCollection([]string{"limit_stats"})
	})

	return s
}

func (s *Scheduler) Start() { s.cron.Start(); log.Println("[scheduler] started") }
func (s *Scheduler) Stop()  { s.cron.Stop(); log.Println("[scheduler] stopped") }

func (s *Scheduler) Trigger(phases []string) {
	go func() {
		log.Printf("[scheduler] manual trigger phases=%v", phases)
		collector.RunManualCollection(phases)
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
