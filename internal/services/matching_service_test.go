package services

import (
	"context"
	"testing"
	"time"
	"uber/internal/config"
	"uber/internal/domain/entities"
	"uber/internal/geo"
	"uber/internal/repository/memory"
)

func setupMatchingService() (*MatchingService, *RideService, *LocationService, *memory.DriverRepository) {
	cfg := config.NewDefaultConfig()
	cfg.Matching.DriverResponseTimeout = 2 * time.Second
	cfg.Matching.TotalMatchingTimeout = 5 * time.Second

	rideRepo := memory.NewRideRepository()
	riderRepo := memory.NewRiderRepository()
	driverRepo := memory.NewDriverRepository()
	locationRepo := memory.NewLocationRepository()
	lockManager := memory.NewLockManager()
	spatialIndex := geo.NewSpatialIndex(cfg.Geo.GeohashPrecision)

	notificationService := NewNotificationService()
	locationService := NewLocationService(spatialIndex, driverRepo, locationRepo)
	rideService := NewRideService(rideRepo, riderRepo, driverRepo, cfg)
	matchingService := NewMatchingService(
		cfg,
		rideService,
		locationService,
		notificationService,
		lockManager,
		driverRepo,
	)

	return matchingService, rideService, locationService, driverRepo
}

func TestMatchingService_StartMatching_NoDrivers(t *testing.T) {
	matchingService, rideService, _, _ := setupMatchingService()
	ctx := context.Background()

	// Create a ride
	estimate, _ := rideService.CreateFareEstimate(ctx, "rider-1", FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	})

	ride, _ := rideService.RequestRide(ctx, "rider-1", estimate.RideID)

	// Start matching with no drivers
	resultChan := matchingService.StartMatching(ctx, ride)
	result := <-resultChan

	if result.Success {
		t.Error("Expected matching to fail with no drivers")
	}
}

func TestMatchingService_StartMatching_DriverAccepts(t *testing.T) {
	matchingService, rideService, locationService, driverRepo := setupMatchingService()
	ctx := context.Background()

	// Create and position a driver
	driverRepo.GetOrCreate(ctx, "driver-1")
	locationService.UpdateDriverLocation(ctx, "driver-1", 37.771, -122.411)

	// Create a ride
	estimate, _ := rideService.CreateFareEstimate(ctx, "rider-1", FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	})

	ride, _ := rideService.RequestRide(ctx, "rider-1", estimate.RideID)

	// Start matching
	resultChan := matchingService.StartMatching(ctx, ride)

	// Give matching time to start and send notification
	time.Sleep(100 * time.Millisecond)

	// Driver accepts
	matchingService.SubmitDriverResponse("driver-1", ride.ID, true)

	result := <-resultChan

	if !result.Success {
		t.Error("Expected matching to succeed when driver accepts")
	}
	if result.DriverID != "driver-1" {
		t.Errorf("Expected driver-1, got %s", result.DriverID)
	}
}

func TestMatchingService_StartMatching_DriverDeclines(t *testing.T) {
	matchingService, rideService, locationService, driverRepo := setupMatchingService()
	ctx := context.Background()

	// Create and position a single driver
	driverRepo.GetOrCreate(ctx, "driver-1")
	locationService.UpdateDriverLocation(ctx, "driver-1", 37.771, -122.411)

	// Create a ride
	estimate, _ := rideService.CreateFareEstimate(ctx, "rider-1", FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	})

	ride, _ := rideService.RequestRide(ctx, "rider-1", estimate.RideID)

	// Start matching
	resultChan := matchingService.StartMatching(ctx, ride)

	// Give matching time to start
	time.Sleep(100 * time.Millisecond)

	// Driver declines
	matchingService.SubmitDriverResponse("driver-1", ride.ID, false)

	result := <-resultChan

	// Should fail since only driver declined
	if result.Success {
		t.Error("Expected matching to fail when only driver declines")
	}
}

func TestMatchingService_StartMatching_SecondDriverAccepts(t *testing.T) {
	matchingService, rideService, locationService, driverRepo := setupMatchingService()
	ctx := context.Background()

	// Create and position two drivers (first one closer)
	driverRepo.GetOrCreate(ctx, "driver-1")
	driverRepo.GetOrCreate(ctx, "driver-2")
	locationService.UpdateDriverLocation(ctx, "driver-1", 37.771, -122.411)  // Closest
	locationService.UpdateDriverLocation(ctx, "driver-2", 37.775, -122.415)  // Second closest

	// Create a ride
	estimate, _ := rideService.CreateFareEstimate(ctx, "rider-1", FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	})

	ride, _ := rideService.RequestRide(ctx, "rider-1", estimate.RideID)

	// Start matching
	resultChan := matchingService.StartMatching(ctx, ride)

	// Give matching time to start
	time.Sleep(100 * time.Millisecond)

	// First driver declines
	matchingService.SubmitDriverResponse("driver-1", ride.ID, false)

	// Wait for second driver to be contacted
	time.Sleep(100 * time.Millisecond)

	// Second driver accepts
	matchingService.SubmitDriverResponse("driver-2", ride.ID, true)

	result := <-resultChan

	if !result.Success {
		t.Error("Expected matching to succeed when second driver accepts")
	}
	if result.DriverID != "driver-2" {
		t.Errorf("Expected driver-2, got %s", result.DriverID)
	}
}

func TestMatchingService_DriverTimeout(t *testing.T) {
	matchingService, rideService, locationService, driverRepo := setupMatchingService()
	ctx := context.Background()

	// Create and position a driver
	driverRepo.GetOrCreate(ctx, "driver-1")
	locationService.UpdateDriverLocation(ctx, "driver-1", 37.771, -122.411)

	// Create a ride
	estimate, _ := rideService.CreateFareEstimate(ctx, "rider-1", FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	})

	ride, _ := rideService.RequestRide(ctx, "rider-1", estimate.RideID)

	// Start matching - driver will timeout (2 second timeout in test config)
	resultChan := matchingService.StartMatching(ctx, ride)

	// Don't submit any driver response - wait for timeout
	result := <-resultChan

	// Should fail since driver timed out
	if result.Success {
		t.Error("Expected matching to fail when driver times out")
	}
}
