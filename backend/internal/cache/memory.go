package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var ErrKeyNotFound = errors.New("cache: key not found")

// sweepInterval bounds how often Set walks the whole map. Rate limit keys are written once
// per client IP and may never be read again, so expiring them only on read would let them pile up.
const sweepInterval = time.Minute

type entry struct {
	value     []byte
	expiresAt time.Time
}

func (e entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

// MemoryCache is the API's in-process cache. A TTL of zero or less keeps the entry until it is forgotten.
type MemoryCache struct {
	mu        sync.Mutex
	data      map[string]entry
	lastSweep time.Time
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{data: make(map[string]entry), lastSweep: time.Now()}
}

func (m *MemoryCache) Get(_ context.Context, key string, dest interface{}) error {
	m.mu.Lock()
	e, ok := m.data[key]
	if ok && e.expired(time.Now()) {
		delete(m.data, key)
		ok = false
	}
	m.mu.Unlock()
	if !ok {
		return ErrKeyNotFound
	}
	return json.Unmarshal(e.value, dest)
}

func (m *MemoryCache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	now := time.Now()
	e := entry{value: b}
	if ttl > 0 {
		e.expiresAt = now.Add(ttl)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = e
	if now.Sub(m.lastSweep) >= sweepInterval {
		for k, old := range m.data {
			if old.expired(now) {
				delete(m.data, k)
			}
		}
		m.lastSweep = now
	}
	return nil
}

func (m *MemoryCache) Remember(ctx context.Context, key string, ttl time.Duration, callback func() (interface{}, error), dest interface{}) error {
	err := m.Get(ctx, key, dest)
	if err == nil {
		return nil
	}
	if err != ErrKeyNotFound {
		return err
	}
	value, err := callback()
	if err != nil {
		return err
	}
	if err := m.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (m *MemoryCache) Forget(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}
