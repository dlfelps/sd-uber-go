package repository

import (
	"context"
	"time"
	"uber/internal/domain/entities"
)

type RiderRepository interface {
	Create(ctx context.Context, rider *entities.Rider) error
	GetByID(ctx context.Context, id string) (*entities.Rider, error)
	Update(ctx context.Context, rider *entities.Rider) error
	Delete(ctx context.Context, id string) error
}

type DriverRepository interface {
	Create(ctx context.Context, driver *entities.Driver) error
	GetByID(ctx context.Context, id string) (*entities.Driver, error)
	Update(ctx context.Context, driver *entities.Driver) error
	Delete(ctx context.Context, id string) error
	GetAvailableDrivers(ctx context.Context) ([]*entities.Driver, error)
	SetStatus(ctx context.Context, id string, status entities.DriverStatus) error
}

type RideRepository interface {
	Create(ctx context.Context, ride *entities.Ride) error
	GetByID(ctx context.Context, id string) (*entities.Ride, error)
	Update(ctx context.Context, ride *entities.Ride) error
	Delete(ctx context.Context, id string) error
	GetByRiderID(ctx context.Context, riderID string) ([]*entities.Ride, error)
	GetByDriverID(ctx context.Context, driverID string) ([]*entities.Ride, error)
	GetActiveRideByRiderID(ctx context.Context, riderID string) (*entities.Ride, error)
}

type LocationRepository interface {
	UpdateDriverLocation(ctx context.Context, location *entities.DriverLocation) error
	GetDriverLocation(ctx context.Context, driverID string) (*entities.DriverLocation, error)
	RemoveDriverLocation(ctx context.Context, driverID string) error
	GetDriversInGeohash(ctx context.Context, geohash string) ([]*entities.DriverLocation, error)
}

type LockManager interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
	IsLocked(ctx context.Context, key string) (bool, error)
}
