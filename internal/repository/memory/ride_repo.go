package memory

import (
	"context"
	"errors"
	"sync"
	"uber/internal/domain/entities"
)

var ErrRideNotFound = errors.New("ride not found")

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
