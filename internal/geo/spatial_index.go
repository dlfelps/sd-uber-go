package geo

import (
	"context"
	"sort"
	"sync"
	"uber/internal/domain/entities"
	"uber/pkg/utils"
)

// DriverWithDistance pairs a driver's location with their computed distance
// from a search point. Used to return sorted proximity results.
type DriverWithDistance struct {
	Driver   *entities.DriverLocation
	Distance float64
}

// SpatialIndex is an in-memory geospatial data structure that enables fast
// "find nearby drivers" queries. It organizes drivers into geohash cells,
// so a proximity search only needs to check the center cell and its 8 neighbors
// (9 cells total) instead of scanning every driver in the system.
//
// Go Learning Note — sync.RWMutex:
// RWMutex provides read-write locking. Multiple goroutines can hold a read lock
// simultaneously (RLock), but a write lock (Lock) is exclusive. This is perfect
// for data structures with many readers and few writers — like a spatial index
// that's queried constantly but only updated when drivers move. Using a plain
// sync.Mutex would serialize all reads unnecessarily.
//
// Go Learning Note — Nested Maps:
// The drivers field is map[string]map[string]*DriverLocation — a two-level map.
// The outer key is the geohash string (which cell), the inner key is driverID
// (which driver in that cell). This gives O(1) lookup by both cell and driver.
// Go maps must be initialized with make() before use; a nil map will panic on
// write (but reads return the zero value).
type SpatialIndex struct {
	mu        sync.RWMutex
	precision int
	drivers   map[string]map[string]*entities.DriverLocation // geohash -> driverID -> location
}

// NewSpatialIndex creates an empty spatial index with the given geohash precision.
func NewSpatialIndex(precision int) *SpatialIndex {
	return &SpatialIndex{
		precision: precision,
		drivers:   make(map[string]map[string]*entities.DriverLocation),
	}
}

// UpdateLocation updates a driver's position in the spatial index. If the driver
// has moved to a different geohash cell, they're removed from the old cell and
// added to the new one. This is called every time a driver sends a location ping.
//
// Go Learning Note — defer:
// `defer s.mu.Unlock()` schedules the unlock to run when the function returns,
// regardless of how it returns (normal return, early return, or even panic).
// This prevents forgetting to unlock — a common source of deadlocks. The defer
// pattern is idiomatic for any resource that needs cleanup: file handles,
// database connections, mutexes, etc.
func (s *SpatialIndex) UpdateLocation(driverID string, lat, lon float64) *entities.DriverLocation {
	s.mu.Lock()
	defer s.mu.Unlock()

	geohash := Encode(lat, lon, s.precision)

	// Remove the driver from their previous geohash cell (if any).
	// We iterate all cells because we don't track which cell the driver was in.
	// With a secondary index (driverID → geohash), this could be O(1) instead
	// of O(n) — a good optimization for production.
	for gh, drivers := range s.drivers {
		if _, exists := drivers[driverID]; exists {
			delete(drivers, driverID)
			if len(drivers) == 0 {
				delete(s.drivers, gh) // Clean up empty cells to prevent memory leaks.
			}
			break
		}
	}

	// Add to the new geohash cell, creating the cell map if needed.
	if _, exists := s.drivers[geohash]; !exists {
		s.drivers[geohash] = make(map[string]*entities.DriverLocation)
	}

	location := entities.NewDriverLocation(driverID, lat, lon, geohash)
	s.drivers[geohash][driverID] = location

	return location
}

// RemoveDriver removes a driver from the spatial index entirely (e.g., when
// they go offline).
func (s *SpatialIndex) RemoveDriver(driverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for gh, drivers := range s.drivers {
		if _, exists := drivers[driverID]; exists {
			delete(drivers, driverID)
			if len(drivers) == 0 {
				delete(s.drivers, gh)
			}
			return
		}
	}
}

// GetDriverLocation returns the current location of a driver, or nil if not
// found in the index.
func (s *SpatialIndex) GetDriverLocation(driverID string) *entities.DriverLocation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, drivers := range s.drivers {
		if loc, exists := drivers[driverID]; exists {
			return loc
		}
	}
	return nil
}

// FindNearbyDrivers finds all drivers within a given radius (in km) from a point.
//
// Strategy: Coarse filter → Fine filter
//  1. Coarse: Compute the geohash of the search point, then get all 9 cells
//     (center + 8 neighbors). Only scan drivers in those cells.
//  2. Fine: For each candidate, compute the exact Haversine distance and
//     filter to those within the radius.
//  3. Sort results by distance (nearest first).
//
// This two-phase approach is dramatically faster than computing distances to
// every driver in the system.
//
// Go Learning Note — sort.Slice:
// sort.Slice sorts a slice in-place using a provided less function. The less
// function takes two indices and returns true if element i should come before
// element j. This is more flexible than sort.Sort (which requires implementing
// the sort.Interface with Len/Less/Swap methods on a named type).
func (s *SpatialIndex) FindNearbyDrivers(ctx context.Context, lat, lon float64, radiusKm float64) []DriverWithDistance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	centerGeohash := Encode(lat, lon, s.precision)
	neighborGeohashes := AllNeighbors(centerGeohash)

	var candidates []DriverWithDistance

	for _, gh := range neighborGeohashes {
		if drivers, exists := s.drivers[gh]; exists {
			for _, driver := range drivers {
				distance := utils.HaversineDistance(lat, lon, driver.Location.Latitude, driver.Location.Longitude)
				if distance <= radiusKm {
					candidates = append(candidates, DriverWithDistance{
						Driver:   driver,
						Distance: distance,
					})
				}
			}
		}
	}

	// Sort by distance so the matching service can try the nearest driver first.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Distance < candidates[j].Distance
	})

	return candidates
}

// FindNearbyDriverIDs returns just the driver IDs within range, sorted by distance.
// This is a convenience wrapper when you only need IDs, not full location data.
//
// Go Learning Note — make() with Length:
// make([]string, len(nearby)) creates a slice with both length and capacity set
// to len(nearby). This pre-allocates exactly the right amount of memory, avoiding
// repeated growth via append(). Use make with a known length when you know the
// exact size upfront. Use make([]T, 0, capacity) when you want to append but
// know the approximate size.
func (s *SpatialIndex) FindNearbyDriverIDs(ctx context.Context, lat, lon float64, radiusKm float64) []string {
	nearby := s.FindNearbyDrivers(ctx, lat, lon, radiusKm)
	ids := make([]string, len(nearby))
	for i, d := range nearby {
		ids[i] = d.Driver.DriverID
	}
	return ids
}

// Count returns the total number of drivers in the index.
func (s *SpatialIndex) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, drivers := range s.drivers {
		count += len(drivers)
	}
	return count
}
