// Package services contains the core business logic. Services orchestrate
// domain entities, repository operations, and cross-cutting concerns (pricing,
// notifications) to implement use cases like "request a ride" or "accept a ride."
//
// Go Learning Note — Service Layer Pattern:
// In a layered architecture, services sit between handlers (HTTP) and
// repositories (data access). They contain the business rules and workflows.
// A service method typically: validates preconditions, loads entities from
// repositories, calls domain methods on those entities, persists changes, and
// returns results. Services should be the ONLY place business logic lives —
// handlers should be thin, and repositories should have no business rules.
package services

import (
	"context"
	"errors"
	"uber/internal/config"
	"uber/internal/domain/entities"
	"uber/internal/repository/memory"
	"uber/pkg/utils"
)

// Sentinel errors for the ride service. These are checked by handlers to map
// to appropriate HTTP status codes.
//
// Go Learning Note — Error Design:
// There are three levels of error sophistication in Go:
//   1. Sentinel errors (used here): var ErrFoo = errors.New("message")
//      Simple, comparable with ==, but carry no dynamic context.
//   2. Custom error types: type NotFoundError struct { ID string }
//      Carry context and can be checked with errors.As().
//   3. Wrapped errors: fmt.Errorf("loading user %s: %w", id, err)
//      Chain errors with context and can be unwrapped with errors.Is/As.
//
// For an MVP, sentinel errors are sufficient. As the app grows, wrapping
// errors with %w provides better debugging context.
var (
	ErrRideNotFound      = errors.New("ride not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNotAuthorized     = errors.New("not authorized to perform this action")
	ErrActiveRideExists  = errors.New("rider already has an active ride")
)

// RideService manages the ride lifecycle: fare estimation, requesting, status
// transitions, and driver assignment. It coordinates between ride, rider, and
// driver repositories.
type RideService struct {
	rideRepo   *memory.RideRepository
	riderRepo  *memory.RiderRepository
	driverRepo *memory.DriverRepository
	config     *config.Config
	calculator *utils.PricingCalculator
}

// NewRideService creates a RideService. The PricingCalculator is initialized
// from the config's pricing parameters — this keeps pricing configuration in
// one place rather than scattered through service methods.
func NewRideService(
	rideRepo *memory.RideRepository,
	riderRepo *memory.RiderRepository,
	driverRepo *memory.DriverRepository,
	cfg *config.Config,
) *RideService {
	return &RideService{
		rideRepo:   rideRepo,
		riderRepo:  riderRepo,
		driverRepo: driverRepo,
		config:     cfg,
		calculator: utils.NewPricingCalculator(
			cfg.Pricing.BaseFare,
			cfg.Pricing.PerKmRate,
			cfg.Pricing.PerMinuteRate,
			cfg.Pricing.MinimumFare,
		),
	}
}

// FareEstimateRequest contains the pickup and dropoff locations for a fare estimate.
type FareEstimateRequest struct {
	Source      entities.Location `json:"source"`
	Destination entities.Location `json:"destination"`
}

// FareEstimateResponse contains the computed fare breakdown, distance, and
// duration. The RideID can be used to later request this ride.
type FareEstimateResponse struct {
	RideID       string             `json:"ride_id"`
	Source       entities.Location  `json:"source"`
	Destination  entities.Location  `json:"destination"`
	DistanceKm   float64            `json:"distance_km"`
	DurationMins float64            `json:"duration_mins"`
	Fare         utils.FareEstimate `json:"fare"`
}

// CreateFareEstimate calculates the fare for a trip and creates a Ride entity
// in the Estimate state. The rider can later confirm this estimate to request
// an actual ride.
func (s *RideService) CreateFareEstimate(ctx context.Context, riderID string, req FareEstimateRequest) (*FareEstimateResponse, error) {
	// Ensure rider exists
	_, err := s.riderRepo.GetOrCreate(ctx, riderID)
	if err != nil {
		return nil, err
	}

	// Calculate distance and duration
	distanceKm := utils.HaversineDistance(
		req.Source.Latitude, req.Source.Longitude,
		req.Destination.Latitude, req.Destination.Longitude,
	)
	durationMins := utils.EstimateDuration(distanceKm)

	// Calculate fare (no surge for MVP)
	fare := s.calculator.CalculateFare(distanceKm, durationMins, 1.0)

	// Create ride entity
	rideID := utils.GenerateID()
	ride := entities.NewRide(
		rideID,
		riderID,
		req.Source,
		req.Destination,
		fare.TotalFare,
		distanceKm,
		durationMins,
	)

	// Save ride
	if err := s.rideRepo.Create(ctx, ride); err != nil {
		return nil, err
	}

	return &FareEstimateResponse{
		RideID:       rideID,
		Source:       req.Source,
		Destination:  req.Destination,
		DistanceKm:   distanceKm,
		DurationMins: durationMins,
		Fare:         fare,
	}, nil
}

// RequestRide transitions a ride from Estimate to Requested. This is the
// rider confirming they want the ride. It checks authorization (is this the
// rider's ride?) and idempotency (does the rider already have an active ride?).
func (s *RideService) RequestRide(ctx context.Context, riderID, rideID string) (*entities.Ride, error) {
	// Check for existing active ride
	activeRide, _ := s.rideRepo.GetActiveRideByRiderID(ctx, riderID)
	if activeRide != nil && activeRide.ID != rideID {
		return nil, ErrActiveRideExists
	}

	ride, err := s.rideRepo.GetByID(ctx, rideID)
	if err != nil {
		return nil, ErrRideNotFound
	}

	if ride.RiderID != riderID {
		return nil, ErrNotAuthorized
	}

	if err := ride.Request(); err != nil {
		return nil, ErrInvalidTransition
	}

	if err := s.rideRepo.Update(ctx, ride); err != nil {
		return nil, err
	}

	return ride, nil
}

// GetRide retrieves a ride by ID
func (s *RideService) GetRide(ctx context.Context, rideID string) (*entities.Ride, error) {
	return s.rideRepo.GetByID(ctx, rideID)
}

// UpdateRideStatus advances a ride through its lifecycle (driver-side).
// It also keeps the driver's status in sync — when a ride starts, the driver
// is marked as InRide; when it completes or is cancelled, the driver becomes
// Available again. This dual-update is a business rule: ride state and driver
// state must always be consistent.
func (s *RideService) UpdateRideStatus(ctx context.Context, driverID, rideID string, newStatus entities.RideStatus) (*entities.Ride, error) {
	ride, err := s.rideRepo.GetByID(ctx, rideID)
	if err != nil {
		return nil, ErrRideNotFound
	}

	if ride.DriverID != driverID {
		return nil, ErrNotAuthorized
	}

	if err := ride.TransitionTo(newStatus); err != nil {
		return nil, ErrInvalidTransition
	}

	// Update driver status based on ride status
	driver, err := s.driverRepo.GetByID(ctx, driverID)
	if err == nil {
		switch newStatus {
		case entities.RideStatusPickingUp, entities.RideStatusInProgress:
			driver.StartRide()
		case entities.RideStatusCompleted, entities.RideStatusCancelled:
			driver.EndRide()
		}
		s.driverRepo.Update(ctx, driver)
	}

	if err := s.rideRepo.Update(ctx, ride); err != nil {
		return nil, err
	}

	return ride, nil
}

// AcceptRide allows a driver to accept or deny a ride. If accepted, the
// ride transitions to Accepted and the driver is marked as InRide. If denied,
// the ride state is unchanged (the matching service will try the next driver).
func (s *RideService) AcceptRide(ctx context.Context, driverID, rideID string, accept bool) (*entities.Ride, error) {
	ride, err := s.rideRepo.GetByID(ctx, rideID)
	if err != nil {
		return nil, ErrRideNotFound
	}

	if !accept {
		// Driver denied, don't change ride state
		return ride, nil
	}

	if err := ride.Accept(driverID); err != nil {
		return nil, ErrInvalidTransition
	}

	// Update driver status
	driver, err := s.driverRepo.GetByID(ctx, driverID)
	if err == nil {
		driver.StartRide()
		s.driverRepo.Update(ctx, driver)
	}

	if err := s.rideRepo.Update(ctx, ride); err != nil {
		return nil, err
	}

	return ride, nil
}

// StartMatching transitions ride to matching status
func (s *RideService) StartMatching(ctx context.Context, ride *entities.Ride) error {
	if err := ride.StartMatching(); err != nil {
		return err
	}
	return s.rideRepo.Update(ctx, ride)
}

// FailMatching marks a ride as failed to find a driver
func (s *RideService) FailMatching(ctx context.Context, rideID string) error {
	ride, err := s.rideRepo.GetByID(ctx, rideID)
	if err != nil {
		return err
	}
	if err := ride.Fail(); err != nil {
		return err
	}
	return s.rideRepo.Update(ctx, ride)
}
