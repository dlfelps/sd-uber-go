package services

import (
	"context"
	"uber/internal/domain/entities"
	"uber/internal/geo"
	"uber/internal/repository/memory"
)

type LocationService struct {
	spatialIndex   *geo.SpatialIndex
	driverRepo     *memory.DriverRepository
	locationRepo   *memory.LocationRepository
}

func NewLocationService(
	spatialIndex *geo.SpatialIndex,
	driverRepo *memory.DriverRepository,
	locationRepo *memory.LocationRepository,
) *LocationService {
	return &LocationService{
		spatialIndex:   spatialIndex,
		driverRepo:     driverRepo,
		locationRepo:   locationRepo,
	}
}

// UpdateDriverLocation updates a driver's current location
func (s *LocationService) UpdateDriverLocation(ctx context.Context, driverID string, lat, lon float64) (*entities.DriverLocation, error) {
	// Ensure driver exists (creates if not)
	driver, err := s.driverRepo.GetOrCreate(ctx, driverID)
	if err != nil {
		return nil, err
	}

	// Make driver available when they update location
	if driver.Status == entities.DriverStatusOffline {
		driver.GoOnline()
		if err := s.driverRepo.Update(ctx, driver); err != nil {
			return nil, err
		}
	}

	// Update spatial index
	location := s.spatialIndex.UpdateLocation(driverID, lat, lon)

	// Update location repository
	if err := s.locationRepo.UpdateDriverLocation(ctx, location); err != nil {
		return nil, err
	}

	return location, nil
}

// GetDriverLocation retrieves a driver's current location
func (s *LocationService) GetDriverLocation(ctx context.Context, driverID string) (*entities.DriverLocation, error) {
	return s.locationRepo.GetDriverLocation(ctx, driverID)
}

// FindNearbyAvailableDrivers finds available drivers within radius
func (s *LocationService) FindNearbyAvailableDrivers(ctx context.Context, lat, lon float64, radiusKm float64) ([]geo.DriverWithDistance, error) {
	// Get all nearby drivers from spatial index
	nearbyDrivers := s.spatialIndex.FindNearbyDrivers(ctx, lat, lon, radiusKm)

	// Filter to only available drivers
	var availableDrivers []geo.DriverWithDistance
	for _, dwd := range nearbyDrivers {
		driver, err := s.driverRepo.GetByID(ctx, dwd.Driver.DriverID)
		if err != nil {
			continue
		}
		if driver.IsAvailable() {
			availableDrivers = append(availableDrivers, dwd)
		}
	}

	return availableDrivers, nil
}

// RemoveDriverLocation removes a driver from location tracking
func (s *LocationService) RemoveDriverLocation(ctx context.Context, driverID string) error {
	s.spatialIndex.RemoveDriver(driverID)
	return s.locationRepo.RemoveDriverLocation(ctx, driverID)
}
