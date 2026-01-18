package services

import (
	"context"
	"testing"
	"uber/internal/config"
	"uber/internal/domain/entities"
	"uber/internal/repository/memory"
)

func setupRideService() (*RideService, *memory.RideRepository, *memory.RiderRepository, *memory.DriverRepository) {
	rideRepo := memory.NewRideRepository()
	riderRepo := memory.NewRiderRepository()
	driverRepo := memory.NewDriverRepository()
	cfg := config.NewDefaultConfig()

	service := NewRideService(rideRepo, riderRepo, driverRepo, cfg)
	return service, rideRepo, riderRepo, driverRepo
}

func TestRideService_CreateFareEstimate(t *testing.T) {
	service, _, _, _ := setupRideService()
	ctx := context.Background()

	req := FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	}

	estimate, err := service.CreateFareEstimate(ctx, "rider-1", req)
	if err != nil {
		t.Fatalf("CreateFareEstimate failed: %v", err)
	}

	if estimate.RideID == "" {
		t.Error("Expected ride ID to be set")
	}
	if estimate.DistanceKm <= 0 {
		t.Error("Expected positive distance")
	}
	if estimate.DurationMins <= 0 {
		t.Error("Expected positive duration")
	}
	if estimate.Fare.TotalFare <= 0 {
		t.Error("Expected positive fare")
	}
}

func TestRideService_RequestRide(t *testing.T) {
	service, _, _, _ := setupRideService()
	ctx := context.Background()

	// Create estimate first
	req := FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	}
	estimate, _ := service.CreateFareEstimate(ctx, "rider-1", req)

	// Request the ride
	ride, err := service.RequestRide(ctx, "rider-1", estimate.RideID)
	if err != nil {
		t.Fatalf("RequestRide failed: %v", err)
	}

	if ride.Status != entities.RideStatusRequested {
		t.Errorf("Expected status requested, got %s", ride.Status)
	}
}

func TestRideService_RequestRide_NotAuthorized(t *testing.T) {
	service, _, _, _ := setupRideService()
	ctx := context.Background()

	// Create estimate for rider-1
	req := FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	}
	estimate, _ := service.CreateFareEstimate(ctx, "rider-1", req)

	// Try to request as different rider
	_, err := service.RequestRide(ctx, "rider-2", estimate.RideID)
	if err != ErrNotAuthorized {
		t.Errorf("Expected ErrNotAuthorized, got %v", err)
	}
}

func TestRideService_RequestRide_ActiveRideExists(t *testing.T) {
	service, _, _, _ := setupRideService()
	ctx := context.Background()

	// Create and request first ride
	req := FareEstimateRequest{
		Source: entities.Location{
			Latitude:  37.77,
			Longitude: -122.41,
		},
		Destination: entities.Location{
			Latitude:  37.78,
			Longitude: -122.40,
		},
	}
	estimate1, _ := service.CreateFareEstimate(ctx, "rider-1", req)
	_, err := service.RequestRide(ctx, "rider-1", estimate1.RideID)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	// Create second estimate
	estimate2, _ := service.CreateFareEstimate(ctx, "rider-1", req)

	// Try to request second ride
	_, err = service.RequestRide(ctx, "rider-1", estimate2.RideID)
	if err != ErrActiveRideExists {
		t.Errorf("Expected ErrActiveRideExists, got %v", err)
	}
}

func TestRideService_UpdateRideStatus(t *testing.T) {
	service, rideRepo, riderRepo, driverRepo := setupRideService()
	ctx := context.Background()

	// Create rider and driver
	riderRepo.GetOrCreate(ctx, "rider-1")
	driverRepo.GetOrCreate(ctx, "driver-1")

	// Create a ride in accepted state
	ride := entities.NewRide("ride-1", "rider-1",
		entities.Location{Latitude: 37.77, Longitude: -122.41},
		entities.Location{Latitude: 37.78, Longitude: -122.40},
		10.00, 1.5, 5.0)
	ride.Request()
	ride.StartMatching()
	ride.Accept("driver-1")
	rideRepo.Create(ctx, ride)

	// Update to picking_up
	updatedRide, err := service.UpdateRideStatus(ctx, "driver-1", "ride-1", entities.RideStatusPickingUp)
	if err != nil {
		t.Fatalf("UpdateRideStatus failed: %v", err)
	}

	if updatedRide.Status != entities.RideStatusPickingUp {
		t.Errorf("Expected status picking_up, got %s", updatedRide.Status)
	}
}

func TestRideService_UpdateRideStatus_InvalidTransition(t *testing.T) {
	service, rideRepo, riderRepo, driverRepo := setupRideService()
	ctx := context.Background()

	riderRepo.GetOrCreate(ctx, "rider-1")
	driverRepo.GetOrCreate(ctx, "driver-1")

	// Create a ride in accepted state
	ride := entities.NewRide("ride-1", "rider-1",
		entities.Location{Latitude: 37.77, Longitude: -122.41},
		entities.Location{Latitude: 37.78, Longitude: -122.40},
		10.00, 1.5, 5.0)
	ride.Request()
	ride.StartMatching()
	ride.Accept("driver-1")
	rideRepo.Create(ctx, ride)

	// Try invalid transition (accepted -> completed without picking_up and in_progress)
	_, err := service.UpdateRideStatus(ctx, "driver-1", "ride-1", entities.RideStatusCompleted)
	if err != ErrInvalidTransition {
		t.Errorf("Expected ErrInvalidTransition, got %v", err)
	}
}

func TestRideService_AcceptRide(t *testing.T) {
	service, rideRepo, riderRepo, driverRepo := setupRideService()
	ctx := context.Background()

	riderRepo.GetOrCreate(ctx, "rider-1")
	driverRepo.GetOrCreate(ctx, "driver-1")

	// Create a ride in matching state
	ride := entities.NewRide("ride-1", "rider-1",
		entities.Location{Latitude: 37.77, Longitude: -122.41},
		entities.Location{Latitude: 37.78, Longitude: -122.40},
		10.00, 1.5, 5.0)
	ride.Request()
	ride.StartMatching()
	rideRepo.Create(ctx, ride)

	// Accept the ride
	acceptedRide, err := service.AcceptRide(ctx, "driver-1", "ride-1", true)
	if err != nil {
		t.Fatalf("AcceptRide failed: %v", err)
	}

	if acceptedRide.Status != entities.RideStatusAccepted {
		t.Errorf("Expected status accepted, got %s", acceptedRide.Status)
	}
	if acceptedRide.DriverID != "driver-1" {
		t.Errorf("Expected driver-1, got %s", acceptedRide.DriverID)
	}
}
