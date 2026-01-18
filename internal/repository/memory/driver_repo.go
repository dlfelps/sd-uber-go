package memory

import (
	"context"
	"errors"
	"sync"
	"uber/internal/domain/entities"
)

var ErrDriverNotFound = errors.New("driver not found")

type DriverRepository struct {
	mu      sync.RWMutex
	drivers map[string]*entities.Driver
}

func NewDriverRepository() *DriverRepository {
	return &DriverRepository{
		drivers: make(map[string]*entities.Driver),
	}
}

func (r *DriverRepository) Create(ctx context.Context, driver *entities.Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drivers[driver.ID] = driver
	return nil
}

func (r *DriverRepository) GetByID(ctx context.Context, id string) (*entities.Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.drivers[id]
	if !exists {
		return nil, ErrDriverNotFound
	}
	return driver, nil
}

func (r *DriverRepository) Update(ctx context.Context, driver *entities.Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.drivers[driver.ID]; !exists {
		return ErrDriverNotFound
	}
	r.drivers[driver.ID] = driver
	return nil
}

func (r *DriverRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.drivers[id]; !exists {
		return ErrDriverNotFound
	}
	delete(r.drivers, id)
	return nil
}

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
