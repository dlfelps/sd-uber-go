package services

import (
	"context"
	"errors"
	"uber/internal/config"
	"uber/internal/domain/entities"
	"uber/internal/repository/memory"
	"uber/pkg/utils"
)

var (
	ErrRideNotFound      = errors.New("ride not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNotAuthorized     = errors.New("not authorized to perform this action")
	ErrActiveRideExists  = errors.New("rider already has an active ride")
)

type RideService struct {
	rideRepo    *memory.RideRepository
	riderRepo   *memory.RiderRepository
	driverRepo  *memory.DriverRepository
	config      *config.Config
	calculator  *utils.PricingCalculator
}

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

type FareEstimateRequest struct {
	Source      entities.Location `json:"source"`
	Destination entities.Location `json:"destination"`
}

type FareEstimateResponse struct {
	RideID       string            `json:"ride_id"`
	Source       entities.Location `json:"source"`
	Destination  entities.Location `json:"destination"`
	DistanceKm   float64           `json:"distance_km"`
	DurationMins float64           `json:"duration_mins"`
	Fare         utils.FareEstimate `json:"fare"`
}

// CreateFareEstimate creates a new ride estimate
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

// RequestRide transitions a ride from estimate to requested
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

// UpdateRideStatus updates ride status (for driver actions)
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

// AcceptRide allows a driver to accept or deny a ride
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
