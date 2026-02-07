// Package memory provides in-memory implementations of the repository interfaces.
// These are suitable for development and testing. For production, you would
// create a "postgres" or "redis" package with the same method signatures that
// satisfies the same interfaces.
//
// Go Learning Note — Thread-Safe Maps:
// Go's built-in map is NOT safe for concurrent use. If multiple goroutines
// read and write a map simultaneously, you'll get a runtime panic. There are
// three common solutions:
//   1. sync.RWMutex (used here) — manual locking around map access
//   2. sync.Map — a concurrent map from the standard library (best for
//      append-only workloads with many reads)
//   3. Channel-based access — serialize all map operations through a goroutine
//
// sync.RWMutex is the most common choice because it gives you explicit control
// and works well with any access pattern.
package memory

import (
	"context"
	"errors"
	"sync"
	"uber/internal/domain/entities"
)

// ErrDriverNotFound is a sentinel error returned when a driver lookup fails.
//
// Go Learning Note — Sentinel Errors:
// Sentinel errors are package-level variables of type error. They allow callers
// to compare errors with == (e.g., `if err == ErrDriverNotFound`). This is a
// simple pattern used in the standard library (io.EOF, sql.ErrNoRows). For
// richer errors that carry context (like the missing ID), you'd define a custom
// error type that implements the error interface.
var ErrDriverNotFound = errors.New("driver not found")

// DriverRepository stores drivers in an in-memory map protected by a RWMutex.
type DriverRepository struct {
	mu      sync.RWMutex
	drivers map[string]*entities.Driver
}

// NewDriverRepository initializes an empty driver store.
func NewDriverRepository() *DriverRepository {
	return &DriverRepository{
		drivers: make(map[string]*entities.Driver),
	}
}

// Create adds a new driver. Uses a write lock since it modifies the map.
func (r *DriverRepository) Create(ctx context.Context, driver *entities.Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drivers[driver.ID] = driver
	return nil
}

// GetByID retrieves a driver by ID. Uses a read lock (RLock) since it only
// reads the map — multiple goroutines can read simultaneously.
func (r *DriverRepository) GetByID(ctx context.Context, id string) (*entities.Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.drivers[id]
	if !exists {
		return nil, ErrDriverNotFound
	}
	return driver, nil
}

// Update replaces a driver's data. Checks existence first to return a
// meaningful error rather than silently creating a new entry.
func (r *DriverRepository) Update(ctx context.Context, driver *entities.Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.drivers[driver.ID]; !exists {
		return ErrDriverNotFound
	}
	r.drivers[driver.ID] = driver
	return nil
}

// Delete removes a driver from the store.
//
// Go Learning Note — delete() Built-in:
// delete(map, key) removes a key from a map. It's a no-op if the key doesn't
// exist (no panic). Here we check existence first to return an error, but in
// some APIs a "delete if exists" (idempotent) approach is preferred.
func (r *DriverRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.drivers[id]; !exists {
		return ErrDriverNotFound
	}
	delete(r.drivers, id)
	return nil
}

// GetAvailableDrivers returns all drivers with status "available". This
// requires a full scan of the map — in production, you'd use a database index
// on the status column, or maintain a separate set of available driver IDs.
func (r *DriverRepository) GetAvailableDrivers(ctx context.Context) ([]*entities.Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []*entities.Driver
	for _, driver := range r.drivers {
		if driver.IsAvailable() {
			available = append(available, driver)
		}
	}
	return available, nil
}

// SetStatus updates only the driver's status field.
func (r *DriverRepository) SetStatus(ctx context.Context, id string, status entities.DriverStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	driver, exists := r.drivers[id]
	if !exists {
		return ErrDriverNotFound
	}
	driver.SetStatus(status)
	return nil
}

// GetOrCreate returns an existing driver or creates a new one with default
// data. This is a convenience for the MVP — real apps would require proper
// driver registration.
func (r *DriverRepository) GetOrCreate(ctx context.Context, id string) (*entities.Driver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if driver, exists := r.drivers[id]; exists {
		return driver, nil
	}

	driver := entities.NewDriver(id, "Driver "+id, id+"@example.com", "555-0000", "vehicle-"+id)
	driver.GoOnline()
	r.drivers[id] = driver
	return driver, nil
}
