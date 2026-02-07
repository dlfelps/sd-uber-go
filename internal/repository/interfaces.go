// Package repository defines interfaces for data access. The actual storage
// implementations live in sub-packages (e.g., repository/memory).
//
// Go Learning Note — Interfaces:
// Go interfaces are one of the language's most distinctive features. Unlike
// Java or C#, Go interfaces are satisfied implicitly — a type implements an
// interface simply by having the right methods, with no "implements" keyword.
// This is called "structural typing" (or "duck typing" at compile time).
//
// Interfaces in Go are typically small (1-3 methods). The standard library's
// io.Reader (one method) and io.Writer (one method) are prime examples. Larger
// interfaces like these repository ones are acceptable when they represent a
// coherent set of CRUD operations.
//
// Go Learning Note — Interface Segregation:
// Defining interfaces in the package that USES them (not the one that implements
// them) is idiomatic Go. This is the opposite of Java's convention. Here the
// interfaces are in the repository package, and the memory package implements
// them. An alternative Go pattern is defining the interface in the service
// that depends on it, keeping each service's required interface minimal.
//
// Go Learning Note — context.Context:
// Every repository method takes context.Context as its first parameter. This
// is a Go convention for any function that might be long-running or need
// cancellation. The context carries deadlines, cancellation signals, and
// request-scoped values down the call chain. Even if the in-memory
// implementation doesn't use it, the interface includes it so that a real
// database implementation (which would use ctx for query timeouts) can be
// swapped in without changing the interface.
package repository

import (
	"context"
	"time"
	"uber/internal/domain/entities"
)

// RiderRepository defines CRUD operations for rider entities.
type RiderRepository interface {
	Create(ctx context.Context, rider *entities.Rider) error
	GetByID(ctx context.Context, id string) (*entities.Rider, error)
	Update(ctx context.Context, rider *entities.Rider) error
	Delete(ctx context.Context, id string) error
}

// DriverRepository extends basic CRUD with driver-specific queries.
type DriverRepository interface {
	Create(ctx context.Context, driver *entities.Driver) error
	GetByID(ctx context.Context, id string) (*entities.Driver, error)
	Update(ctx context.Context, driver *entities.Driver) error
	Delete(ctx context.Context, id string) error
	GetAvailableDrivers(ctx context.Context) ([]*entities.Driver, error)
	SetStatus(ctx context.Context, id string, status entities.DriverStatus) error
}

// RideRepository provides ride persistence with query methods for looking up
// rides by rider or driver.
type RideRepository interface {
	Create(ctx context.Context, ride *entities.Ride) error
	GetByID(ctx context.Context, id string) (*entities.Ride, error)
	Update(ctx context.Context, ride *entities.Ride) error
	Delete(ctx context.Context, id string) error
	GetByRiderID(ctx context.Context, riderID string) ([]*entities.Ride, error)
	GetByDriverID(ctx context.Context, driverID string) ([]*entities.Ride, error)
	GetActiveRideByRiderID(ctx context.Context, riderID string) (*entities.Ride, error)
}

// LocationRepository manages driver GPS positions with geohash-based indexing.
type LocationRepository interface {
	UpdateDriverLocation(ctx context.Context, location *entities.DriverLocation) error
	GetDriverLocation(ctx context.Context, driverID string) (*entities.DriverLocation, error)
	RemoveDriverLocation(ctx context.Context, driverID string) error
	GetDriversInGeohash(ctx context.Context, geohash string) ([]*entities.DriverLocation, error)
}

// LockManager provides distributed locking to prevent double-booking drivers.
// In production, this would be backed by Redis (SETNX with TTL) or etcd.
type LockManager interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
	IsLocked(ctx context.Context, key string) (bool, error)
}
