package memory

import (
	"context"
	"sync"
	"time"
)

type lockEntry struct {
	expiresAt time.Time
}

type LockManager struct {
	mu    sync.RWMutex
	locks map[string]*lockEntry
	stop  chan struct{}
}

func NewLockManager() *LockManager {
	lm := &LockManager{
		locks: make(map[string]*lockEntry),
		stop:  make(chan struct{}),
	}
	go lm.cleanupExpiredLocks()
	return lm
}

func (lm *LockManager) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if entry, exists := lm.locks[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			return false, nil
		}
	}

	lm.locks[key] = &lockEntry{
		expiresAt: time.Now().Add(ttl),
	}
	return true, nil
}

func (lm *LockManager) ReleaseLock(ctx context.Context, key string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	delete(lm.locks, key)
	return nil
}

func (lm *LockManager) IsLocked(ctx context.Context, key string) (bool, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if entry, exists := lm.locks[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			return true, nil
		}
	}
	return false, nil
}

func (lm *LockManager) cleanupExpiredLocks() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lm.mu.Lock()
			now := time.Now()
			for key, entry := range lm.locks {
				if now.After(entry.expiresAt) {
					delete(lm.locks, key)
				}
			}
			lm.mu.Unlock()
		case <-lm.stop:
			return
		}
	}
}

func (lm *LockManager) Stop() {
	close(lm.stop)
}
