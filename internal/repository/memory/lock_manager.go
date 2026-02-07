package memory

import (
	"context"
	"sync"
	"time"
)

// lockEntry represents a single lock with an expiration time (TTL).
// The TTL ensures that locks held by crashed processes eventually expire
// rather than being held forever (preventing deadlocks).
type lockEntry struct {
	expiresAt time.Time
}

// LockManager provides in-memory distributed locking with TTL-based expiration.
// In the matching service, it prevents two matching goroutines from offering the
// same ride to the same driver simultaneously (double-booking prevention).
//
// In production, this would be replaced by Redis SETNX with TTL or etcd leases,
// which work across multiple server instances. This in-memory version only
// works for a single-instance deployment.
//
// Go Learning Note — Channels for Signaling:
// The `stop` field is a `chan struct{}` — an empty struct channel used purely
// for signaling. `struct{}` occupies zero bytes, making it the most efficient
// signal type. The pattern is: close(stop) to signal all goroutines listening
// on this channel to exit. A closed channel returns immediately on receive,
// so `<-lm.stop` in the select will trigger once Stop() is called.
type LockManager struct {
	mu    sync.RWMutex
	locks map[string]*lockEntry
	stop  chan struct{}
}

// NewLockManager creates a LockManager and starts a background goroutine to
// clean up expired locks.
//
// Go Learning Note — Background Goroutines:
// The `go lm.cleanupExpiredLocks()` starts a long-running goroutine that
// periodically sweeps expired locks. This is a common pattern for housekeeping
// tasks (cache eviction, metric flushing, health checks). The goroutine exits
// when Stop() is called via the stop channel. Always provide a way to stop
// background goroutines to prevent goroutine leaks in tests.
func NewLockManager() *LockManager {
	lm := &LockManager{
		locks: make(map[string]*lockEntry),
		stop:  make(chan struct{}),
	}
	go lm.cleanupExpiredLocks()
	return lm
}

// AcquireLock attempts to acquire a named lock with a time-to-live duration.
// Returns (true, nil) if the lock was acquired, (false, nil) if it's already
// held by someone else. If the existing lock has expired, it's treated as free.
//
// This is the Go equivalent of Redis's `SET key value NX EX ttl`.
func (lm *LockManager) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if entry, exists := lm.locks[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			return false, nil // Lock is still held — acquisition fails.
		}
		// Lock has expired — fall through to acquire it.
	}

	lm.locks[key] = &lockEntry{
		expiresAt: time.Now().Add(ttl),
	}
	return true, nil
}

// ReleaseLock explicitly releases a lock before its TTL expires.
func (lm *LockManager) ReleaseLock(ctx context.Context, key string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	delete(lm.locks, key)
	return nil
}

// IsLocked checks whether a lock is currently held (and not expired).
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

// cleanupExpiredLocks runs in a background goroutine and periodically removes
// locks that have passed their TTL.
//
// Go Learning Note — time.NewTicker:
// time.NewTicker creates a channel that receives a value at regular intervals.
// Unlike time.After (one-shot), a ticker repeats forever until stopped. Always
// call ticker.Stop() when done (via defer) to release the underlying timer
// resources. Without Stop(), the ticker goroutine and its channel will leak.
//
// Go Learning Note — select Statement:
// select is like switch but for channel operations. It blocks until one of the
// cases can proceed. If multiple cases are ready, one is chosen at random.
// Here it waits for either the ticker (do cleanup) or the stop signal (exit).
// This is the idiomatic pattern for a cancellable periodic task.
//
// Go Learning Note — Safe Map Deletion During Iteration:
// In Go, it's safe to delete map keys during a for-range loop over that map.
// The Go spec explicitly guarantees this behavior. This isn't true in all
// languages (Java's HashMap throws ConcurrentModificationException).
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

// Stop signals the background cleanup goroutine to exit.
// Call this during graceful shutdown to prevent goroutine leaks.
func (lm *LockManager) Stop() {
	close(lm.stop)
}
