package scheduler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// InMemoryEventBus provides publish/subscribe with glob-style pattern matching.
type InMemoryEventBus struct {
	mu     sync.RWMutex
	subs   map[uint64]*subscription
	next   uint64
	done   chan struct{}
	closed bool
}

type subscription struct {
	pattern string
	ch      chan Event
	closed  bool
}

func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{
		subs: make(map[uint64]*subscription),
		done: make(chan struct{}),
	}
}

func (b *InMemoryEventBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subs {
		if sub.closed {
			continue
		}
		if matchPattern(sub.pattern, formatEventKey(event.Type, event.Key)) ||
			matchPattern(sub.pattern, event.Type) {
			select {
			case sub.ch <- event:
			default:
			}
		}
	}
	return nil
}

func (b *InMemoryEventBus) Subscribe(ctx context.Context, pattern string) (<-chan Event, error) {
	ch := make(chan Event, 32)

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(ch)
		return ch, nil
	}
	id := b.next
	b.next++
	sub := &subscription{pattern: pattern, ch: ch}
	b.subs[id] = sub
	b.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
		case <-b.done:
		}
		b.mu.Lock()
		if !sub.closed {
			sub.closed = true
			close(sub.ch)
			delete(b.subs, id)
		}
		b.mu.Unlock()
	}()

	return ch, nil
}

func (b *InMemoryEventBus) Unsubscribe(ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, sub := range b.subs {
		if sub.ch == ch && !sub.closed {
			sub.closed = true
			close(sub.ch)
			delete(b.subs, id)
			return
		}
	}
}

func (b *InMemoryEventBus) IsConnected() bool { return true }

func (b *InMemoryEventBus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	close(b.done)

	// Give goroutines a moment to clean up, then force-close remaining
	time.Sleep(50 * time.Millisecond)

	b.mu.Lock()
	defer b.mu.Unlock()
	for id, sub := range b.subs {
		if !sub.closed {
			sub.closed = true
			close(sub.ch)
			delete(b.subs, id)
		}
	}
	return nil
}

func formatEventKey(eventType, key string) string {
	if key == "" {
		return eventType
	}
	return eventType + ":" + key
}

func matchPattern(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	matched, err := filepath.Match(pattern, s)
	if err != nil {
		return pattern == s
	}
	if matched {
		return true
	}
	if strings.Contains(pattern, ":*") {
		baseType := strings.TrimSuffix(pattern, ":*")
		if s == baseType {
			return true
		}
		parts := strings.SplitN(s, ":", 2)
		if len(parts) > 0 && parts[0] == baseType {
			return true
		}
	}
	return false
}

// ── InMemoryLeaderElection ──

type InMemoryLeaderElection struct {
	mu    sync.Mutex
	locks map[string]*heldLock
}

type heldLock struct {
	expiresAt time.Time
}

func NewInMemoryLeaderElection() *InMemoryLeaderElection {
	return &InMemoryLeaderElection{locks: make(map[string]*heldLock)}
}

func (e *InMemoryLeaderElection) TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	if held, ok := e.locks[key]; ok && held.expiresAt.After(now) {
		return false, nil
	}
	e.locks[key] = &heldLock{expiresAt: now.Add(ttl)}
	return true, nil
}

func (e *InMemoryLeaderElection) Renew(ctx context.Context, key string, ttl time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if held, ok := e.locks[key]; ok {
		held.expiresAt = time.Now().Add(ttl)
		return nil
	}
	return fmt.Errorf("lock %s not held", key)
}

func (e *InMemoryLeaderElection) Release(ctx context.Context, key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.locks, key)
	return nil
}

func (e *InMemoryLeaderElection) IsHeld(key string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	held, ok := e.locks[key]
	return ok && held.expiresAt.After(time.Now())
}

func (e *InMemoryLeaderElection) IsConnected() bool { return true }
func (e *InMemoryLeaderElection) IsActive() bool { return true }
func (e *InMemoryLeaderElection) IsLeader() bool { return true }

func (e *InMemoryLeaderElection) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.locks = nil
	return nil
}

// ── InMemoryStateStore ──

type InMemoryStateStore struct {
	mu   sync.RWMutex
	data map[string]stateEntry
}

type stateEntry struct {
	val      any
	expireAt time.Time
}

func NewInMemoryStateStore() *InMemoryStateStore {
	return &InMemoryStateStore{data: make(map[string]stateEntry)}
}

func (s *InMemoryStateStore) Get(ctx context.Context, key string, dest any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}
	if !entry.expireAt.IsZero() && time.Now().After(entry.expireAt) {
		return fmt.Errorf("key expired: %s", key)
	}
	if dp, ok2 := dest.(*any); ok2 {
		*dp = entry.val
	}
	return nil
}

func (s *InMemoryStateStore) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = stateEntry{val: val, expireAt: time.Now().Add(ttl)}
	return nil
}

func (s *InMemoryStateStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *InMemoryStateStore) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var keys []string
	now := time.Now()
	for k, entry := range s.data {
		if !entry.expireAt.IsZero() && now.After(entry.expireAt) {
			continue
		}
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *InMemoryStateStore) IsConnected() bool { return true }

func (s *InMemoryStateStore) Close() error {
	return nil
}
