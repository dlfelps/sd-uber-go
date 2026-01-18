package geo

import (
	"context"
	"sort"
	"sync"
	"uber/internal/domain/entities"
	"uber/pkg/utils"
)

type DriverWithDistance struct {
	Driver   *entities.DriverLocation
	Distance float64
}

type SpatialIndex struct {
	mu        sync.RWMutex
	precision int
	drivers   map[string]map[string]*entities.DriverLocation // geohash -> driverID -> location
}

func NewSpatialIndex(precision int) *SpatialIndex {
	return &SpatialIndex{
		precision: precision,
		drivers:   make(map[string]map[string]*entities.DriverLocation),
	}
}

// UpdateLocation updates a driver's location in the spatial index
func (s *SpatialIndex) UpdateLocation(driverID string, lat, lon float64) *entities.DriverLocation {
	s.mu.Lock()
	defer s.mu.Unlock()

	geohash := Encode(lat, lon, s.precision)

	// Remove from old geohash if exists
	for gh, drivers := range s.drivers {
		if _, exists := drivers[driverID]; exists {
			delete(drivers, driverID)
			if len(drivers) == 0 {
				delete(s.drivers, gh)
			}
			break
		}
	}

	// Add to new geohash
	if _, exists := s.drivers[geohash]; !exists {
		s.drivers[geohash] = make(map[string]*entities.DriverLocation)
	}

	location := entities.NewDriverLocation(driverID, lat, lon, geohash)
	s.drivers[geohash][driverID] = location

	return location
}

// RemoveDriver removes a driver from the spatial index
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

// GetDriverLocation returns the current location of a driver
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

// FindNearbyDrivers finds all drivers within a given radius (in km) from a point
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

	// Sort by distance
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Distance < candidates[j].Distance
	})

	return candidates
}

// FindNearbyDriverIDs returns just the driver IDs within range, sorted by distance
func (s *SpatialIndex) FindNearbyDriverIDs(ctx context.Context, lat, lon float64, radiusKm float64) []string {
	nearby := s.FindNearbyDrivers(ctx, lat, lon, radiusKm)
	ids := make([]string, len(nearby))
	for i, d := range nearby {
		ids[i] = d.Driver.DriverID
	}
	return ids
}

// Count returns the total number of drivers in the index
func (s *SpatialIndex) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, drivers := range s.drivers {
		count += len(drivers)
	}
	return count
}
