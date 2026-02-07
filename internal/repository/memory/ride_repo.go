package memory

import (
	"context"
	"errors"
	"sync"
	"uber/internal/domain/entities"
)

var ErrRideNotFound = errors.New("ride not found")

// RideRepository stores rides in memory. It includes query methods for finding
// rides by rider or driver, and for checking if a rider has an active ride
// (to prevent double-booking).
type RideRepository struct {
	mu    sync.RWMutex
	rides map[string]*entities.Ride
}

func NewRideRepository() *RideRepository {
	return &RideRepository{
		rides: make(map[string]*entities.Ride),
	}
}

func (r *RideRepository) Create(ctx context.Context, ride *entities.Ride) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rides[ride.ID] = ride
	return nil
}

func (r *RideRepository) GetByID(ctx context.Context, id string) (*entities.Ride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ride, exists := r.rides[id]
	if !exists {
		return nil, ErrRideNotFound
	}
	return ride, nil
}

func (r *RideRepository) Update(ctx context.Context, ride *entities.Ride) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rides[ride.ID]; !exists {
		return ErrRideNotFound
	}
	r.rides[ride.ID] = ride
	return nil
}

func (r *RideRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rides[id]; !exists {
		return ErrRideNotFound
	}
	delete(r.rides, id)
	return nil
}

// GetByRiderID returns all rides for a given rider (history + active).
// This is an O(n) scan — in production, you'd index rides by riderID.
func (r *RideRepository) GetByRiderID(ctx context.Context, riderID string) ([]*entities.Ride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var rides []*entities.Ride
	for _, ride := range r.rides {
		if ride.RiderID == riderID {
			rides = append(rides, ride)
		}
	}
	return rides, nil
}

// GetByDriverID returns all rides for a given driver.
func (r *RideRepository) GetByDriverID(ctx context.Context, driverID string) ([]*entities.Ride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var rides []*entities.Ride
	for _, ride := range r.rides {
		if ride.DriverID == driverID {
			rides = append(rides, ride)
		}
	}
	return rides, nil
}

// GetActiveRideByRiderID returns a ride that is currently in progress for
// a given rider, or nil if none exists. A ride is "active" if it's in any
// non-terminal state (not completed, cancelled, or failed). This prevents
// riders from requesting a new ride while they already have one in progress.
//
// Go Learning Note — Multiple Return Values:
// Returning (nil, nil) means "no active ride found, and that's not an error."
// This is a common Go pattern: nil error means success, nil pointer means
// "not found." The caller checks both: if ride != nil, there's an active ride.
// This is different from GetByID which returns an error for "not found" —
// the distinction is that having no active ride is a normal case, not an error.
func (r *RideRepository) GetActiveRideByRiderID(ctx context.Context, riderID string) (*entities.Ride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ride := range r.rides {
		if ride.RiderID == riderID {
			switch ride.Status {
			case entities.RideStatusRequested,
				entities.RideStatusMatching,
				entities.RideStatusAccepted,
				entities.RideStatusPickingUp,
				entities.RideStatusInProgress:
				return ride, nil
			}
		}
	}
	return nil, nil
}
