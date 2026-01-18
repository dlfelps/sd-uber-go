package memory

import (
	"context"
	"sync"
	"uber/internal/domain/entities"
)

type LocationRepository struct {
	mu        sync.RWMutex
	locations map[string]*entities.DriverLocation
	geohashIndex map[string]map[string]*entities.DriverLocation
}

func NewLocationRepository() *LocationRepository {
	return &LocationRepository{
		locations:    make(map[string]*entities.DriverLocation),
		geohashIndex: make(map[string]map[string]*entities.DriverLocation),
	}
}

func (r *LocationRepository) UpdateDriverLocation(ctx context.Context, location *entities.DriverLocation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldLocation, exists := r.locations[location.DriverID]
	if exists && oldLocation.Geohash != location.Geohash {
		if geohashMap, ok := r.geohashIndex[oldLocation.Geohash]; ok {
			delete(geohashMap, location.DriverID)
			if len(geohashMap) == 0 {
				delete(r.geohashIndex, oldLocation.Geohash)
			}
		}
	}

	r.locations[location.DriverID] = location

	if _, exists := r.geohashIndex[location.Geohash]; !exists {
		r.geohashIndex[location.Geohash] = make(map[string]*entities.DriverLocation)
	}
	r.geohashIndex[location.Geohash][location.DriverID] = location

	return nil
}

func (r *LocationRepository) GetDriverLocation(ctx context.Context, driverID string) (*entities.DriverLocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	location, exists := r.locations[driverID]
	if !exists {
		return nil, nil
	}
	return location, nil
}

func (r *LocationRepository) RemoveDriverLocation(ctx context.Context, driverID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	location, exists := r.locations[driverID]
	if !exists {
		return nil
	}

	if geohashMap, ok := r.geohashIndex[location.Geohash]; ok {
		delete(geohashMap, driverID)
		if len(geohashMap) == 0 {
			delete(r.geohashIndex, location.Geohash)
		}
	}

	delete(r.locations, driverID)
	return nil
}

func (r *LocationRepository) GetDriversInGeohash(ctx context.Context, geohash string) ([]*entities.DriverLocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var locations []*entities.DriverLocation
	if geohashMap, exists := r.geohashIndex[geohash]; exists {
		for _, loc := range geohashMap {
			locations = append(locations, loc)
		}
	}
	return locations, nil
}

func (r *LocationRepository) GetAllGeohashes(ctx context.Context) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	geohashes := make([]string, 0, len(r.geohashIndex))
	for gh := range r.geohashIndex {
		geohashes = append(geohashes, gh)
	}
	return geohashes
}
