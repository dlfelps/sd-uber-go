package services

import (
	"context"
	"uber/internal/domain/entities"
	"uber/internal/geo"
	"uber/internal/repository/memory"
)

// LocationService manages real-time driver location tracking. It coordinates
// between the spatial index (for fast proximity queries) and the location
// repository (for persistent storage). Both are updated on every location ping.
type LocationService struct {
	spatialIndex *geo.SpatialIndex
	driverRepo   *memory.DriverRepository
	locationRepo *memory.LocationRepository
}

// NewLocationService creates a LocationService with its dependencies.
func NewLocationService(
	spatialIndex *geo.SpatialIndex,
	driverRepo *memory.DriverRepository,
	locationRepo *memory.LocationRepository,
) *LocationService {
	return &LocationService{
		spatialIndex: spatialIndex,
		driverRepo:   driverRepo,
		locationRepo: locationRepo,
	}
}

// UpdateDriverLocation processes a driver's GPS location ping. It auto-creates
// the driver if needed (for the MVP) and automatically marks offline drivers
// as available when they start sending location updates — the assumption being
// that a driver sending their location means they're ready to accept rides.
func (s *LocationService) UpdateDriverLocation(ctx context.Context, driverID string, lat, lon float64) (*entities.DriverLocation, error) {
	// Ensure driver exists (creates with default data if not).
	driver, err := s.driverRepo.GetOrCreate(ctx, driverID)
	if err != nil {
		return nil, err
	}

	// Automatically set driver to available when they start sending location.
	if driver.Status == entities.DriverStatusOffline {
		driver.GoOnline()
		if err := s.driverRepo.Update(ctx, driver); err != nil {
			return nil, err
		}
	}

	// Update spatial index — this computes the geohash and moves the driver
	// to the correct cell.
	location := s.spatialIndex.UpdateLocation(driverID, lat, lon)

	// Also persist to the location repository for historical/debug queries.
	if err := s.locationRepo.UpdateDriverLocation(ctx, location); err != nil {
		return nil, err
	}

	return location, nil
}

// GetDriverLocation retrieves a driver's last known location.
func (s *LocationService) GetDriverLocation(ctx context.Context, driverID string) (*entities.DriverLocation, error) {
	return s.locationRepo.GetDriverLocation(ctx, driverID)
}

// FindNearbyAvailableDrivers finds drivers that are both geographically nearby
// AND have a status of "available." The spatial index provides the coarse
// proximity filter, then we check each driver's status against the driver
// repository.
//
// Go Learning Note — Filtering Pattern:
// The pattern of "query a broad set, then filter" is common in Go. Here we get
// all nearby drivers from the spatial index, then filter to only available ones.
// The alternative (only indexing available drivers) would couple location
// tracking with driver status, which is harder to maintain.
func (s *LocationService) FindNearbyAvailableDrivers(ctx context.Context, lat, lon float64, radiusKm float64) ([]geo.DriverWithDistance, error) {
	// Get all nearby drivers from spatial index (regardless of status).
	nearbyDrivers := s.spatialIndex.FindNearbyDrivers(ctx, lat, lon, radiusKm)

	// Filter to only available drivers by checking each driver's current status.
	var availableDrivers []geo.DriverWithDistance
	for _, dwd := range nearbyDrivers {
		driver, err := s.driverRepo.GetByID(ctx, dwd.Driver.DriverID)
		if err != nil {
			continue // Driver might have been deleted; skip them.
		}
		if driver.IsAvailable() {
			availableDrivers = append(availableDrivers, dwd)
		}
	}

	return availableDrivers, nil
}

// RemoveDriverLocation removes a driver from both the spatial index and the
// location repository (e.g., when they go offline).
func (s *LocationService) RemoveDriverLocation(ctx context.Context, driverID string) error {
	s.spatialIndex.RemoveDriver(driverID)
	return s.locationRepo.RemoveDriverLocation(ctx, driverID)
}
