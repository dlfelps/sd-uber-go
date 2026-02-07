package memory

import (
	"context"
	"sync"
	"uber/internal/domain/entities"
)

// LocationRepository stores driver locations with a secondary geohash index
// for spatial queries. It maintains two data structures:
//   - locations: driverID → DriverLocation (primary lookup by driver)
//   - geohashIndex: geohash → driverID → DriverLocation (spatial lookup)
//
// This dual-index pattern is common when you need fast lookups by two different
// keys. The tradeoff is that both indices must be kept in sync on every write.
type LocationRepository struct {
	mu           sync.RWMutex
	locations    map[string]*entities.DriverLocation            // driverID → location
	geohashIndex map[string]map[string]*entities.DriverLocation // geohash → driverID → location
}

func NewLocationRepository() *LocationRepository {
	return &LocationRepository{
		locations:    make(map[string]*entities.DriverLocation),
		geohashIndex: make(map[string]map[string]*entities.DriverLocation),
	}
}

// UpdateDriverLocation upserts a driver's location, maintaining both indices.
// If the driver moved to a different geohash cell, the old cell's entry is
// cleaned up first to prevent stale references.
func (r *LocationRepository) UpdateDriverLocation(ctx context.Context, location *entities.DriverLocation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If driver has a previous location in a different geohash cell, remove
	// the old entry from the geohash index.
	oldLocation, exists := r.locations[location.DriverID]
	if exists && oldLocation.Geohash != location.Geohash {
		if geohashMap, ok := r.geohashIndex[oldLocation.Geohash]; ok {
			delete(geohashMap, location.DriverID)
			if len(geohashMap) == 0 {
				delete(r.geohashIndex, oldLocation.Geohash) // Clean up empty cells.
			}
		}
	}

	// Update primary index.
	r.locations[location.DriverID] = location

	// Update geohash index.
	if _, exists := r.geohashIndex[location.Geohash]; !exists {
		r.geohashIndex[location.Geohash] = make(map[string]*entities.DriverLocation)
	}
	r.geohashIndex[location.Geohash][location.DriverID] = location

	return nil
}

// GetDriverLocation returns a driver's current location, or (nil, nil) if
// they haven't sent a location update yet.
func (r *LocationRepository) GetDriverLocation(ctx context.Context, driverID string) (*entities.DriverLocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	location, exists := r.locations[driverID]
	if !exists {
		return nil, nil
	}
	return location, nil
}

// RemoveDriverLocation removes a driver from both indices.
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

// GetDriversInGeohash returns all drivers in a specific geohash cell.
// This is an O(1) cell lookup + O(k) iteration where k is drivers in that cell.
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

// GetAllGeohashes returns all geohash cells that currently have drivers.
//
// Go Learning Note — make() with Length 0 and Capacity:
// make([]string, 0, len(r.geohashIndex)) creates a slice with length 0 but
// pre-allocated capacity. This is the pattern when you want to use append()
// but know approximately how many elements you'll add. It avoids the repeated
// slice growing that happens when append exceeds capacity (which copies the
// entire underlying array each time it doubles).
func (r *LocationRepository) GetAllGeohashes(ctx context.Context) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	geohashes := make([]string, 0, len(r.geohashIndex))
	for gh := range r.geohashIndex {
		geohashes = append(geohashes, gh)
	}
	return geohashes
}
