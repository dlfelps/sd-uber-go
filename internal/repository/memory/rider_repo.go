package memory

import (
	"context"
	"errors"
	"sync"
	"uber/internal/domain/entities"
)

var ErrRiderNotFound = errors.New("rider not found")

// RiderRepository is the in-memory rider store. Its structure mirrors
// DriverRepository — all in-memory repos in this package follow the same
// pattern: a map guarded by sync.RWMutex.
type RiderRepository struct {
	mu     sync.RWMutex
	riders map[string]*entities.Rider
}

func NewRiderRepository() *RiderRepository {
	return &RiderRepository{
		riders: make(map[string]*entities.Rider),
	}
}

func (r *RiderRepository) Create(ctx context.Context, rider *entities.Rider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.riders[rider.ID] = rider
	return nil
}

func (r *RiderRepository) GetByID(ctx context.Context, id string) (*entities.Rider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rider, exists := r.riders[id]
	if !exists {
		return nil, ErrRiderNotFound
	}
	return rider, nil
}

func (r *RiderRepository) Update(ctx context.Context, rider *entities.Rider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.riders[rider.ID]; !exists {
		return ErrRiderNotFound
	}
	r.riders[rider.ID] = rider
	return nil
}

func (r *RiderRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.riders[id]; !exists {
		return ErrRiderNotFound
	}
	delete(r.riders, id)
	return nil
}

// GetOrCreate returns an existing rider or auto-creates one with placeholder
// data. This simplifies the MVP by not requiring a separate registration flow.
func (r *RiderRepository) GetOrCreate(ctx context.Context, id string) (*entities.Rider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rider, exists := r.riders[id]; exists {
		return rider, nil
	}

	rider := entities.NewRider(id, "Rider "+id, id+"@example.com", "555-0000")
	r.riders[id] = rider
	return rider, nil
}
