package services

import (
	"context"
	"log"
	"sync"
	"time"
	"uber/internal/config"
	"uber/internal/domain/entities"
	"uber/internal/repository/memory"
)

type MatchingRequest struct {
	RideID   string
	RiderID  string
	Response chan MatchingResult
}

type MatchingResult struct {
	Success  bool
	DriverID string
	Error    error
}

type DriverResponse struct {
	DriverID string
	RideID   string
	Accept   bool
}

type MatchingService struct {
	config              *config.Config
	rideService         *RideService
	locationService     *LocationService
	notificationService *NotificationService
	lockManager         *memory.LockManager
	driverRepo          *memory.DriverRepository

	// Channel for driver responses
	driverResponses chan DriverResponse

	// Pending matches - rideID -> response channel
	pendingMatches map[string]chan DriverResponse
	pendingMu      sync.RWMutex
}

func NewMatchingService(
	cfg *config.Config,
	rideService *RideService,
	locationService *LocationService,
	notificationService *NotificationService,
	lockManager *memory.LockManager,
	driverRepo *memory.DriverRepository,
) *MatchingService {
	ms := &MatchingService{
		config:              cfg,
		rideService:         rideService,
		locationService:     locationService,
		notificationService: notificationService,
		lockManager:         lockManager,
		driverRepo:          driverRepo,
		driverResponses:     make(chan DriverResponse, 100),
		pendingMatches:      make(map[string]chan DriverResponse),
	}

	go ms.processDriverResponses()

	return ms
}

// processDriverResponses routes driver responses to the correct matching goroutine
func (s *MatchingService) processDriverResponses() {
	for resp := range s.driverResponses {
		s.pendingMu.RLock()
		ch, exists := s.pendingMatches[resp.RideID]
		s.pendingMu.RUnlock()

		if exists {
			select {
			case ch <- resp:
			default:
				log.Printf("[MATCHING] Response channel full for ride %s", resp.RideID)
			}
		}
	}
}

// StartMatching begins the async matching process for a ride
func (s *MatchingService) StartMatching(ctx context.Context, ride *entities.Ride) <-chan MatchingResult {
	resultChan := make(chan MatchingResult, 1)

	go s.matchingLoop(ctx, ride, resultChan)

	return resultChan
}

func (s *MatchingService) matchingLoop(ctx context.Context, ride *entities.Ride, resultChan chan<- MatchingResult) {
	defer close(resultChan)

	// Create response channel for this ride
	responseChan := make(chan DriverResponse, 10)
	s.pendingMu.Lock()
	s.pendingMatches[ride.ID] = responseChan
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingMatches, ride.ID)
		s.pendingMu.Unlock()
		close(responseChan)
	}()

	// Update ride status to matching
	if err := s.rideService.StartMatching(ctx, ride); err != nil {
		resultChan <- MatchingResult{Success: false, Error: err}
		return
	}

	// Set total timeout
	totalTimeout := time.After(s.config.Matching.TotalMatchingTimeout)

	// Find nearby available drivers
	nearbyDrivers, err := s.locationService.FindNearbyAvailableDrivers(
		ctx,
		ride.Source.Latitude,
		ride.Source.Longitude,
		s.config.Matching.SearchRadiusKm,
	)

	if err != nil {
		log.Printf("[MATCHING] Error finding drivers for ride %s: %v", ride.ID, err)
		s.rideService.FailMatching(ctx, ride.ID)
		s.notificationService.NotifyRiderOfNoDriversAvailable(ride.RiderID, ride.ID)
		resultChan <- MatchingResult{Success: false, Error: err}
		return
	}

	if len(nearbyDrivers) == 0 {
		log.Printf("[MATCHING] No drivers found for ride %s", ride.ID)
		s.rideService.FailMatching(ctx, ride.ID)
		s.notificationService.NotifyRiderOfNoDriversAvailable(ride.RiderID, ride.ID)
		resultChan <- MatchingResult{Success: false}
		return
	}

	log.Printf("[MATCHING] Found %d nearby drivers for ride %s", len(nearbyDrivers), ride.ID)

	// Try each driver in order of distance
	for _, dwd := range nearbyDrivers {
		select {
		case <-totalTimeout:
			log.Printf("[MATCHING] Total timeout exceeded for ride %s", ride.ID)
			s.rideService.FailMatching(ctx, ride.ID)
			s.notificationService.NotifyRiderOfNoDriversAvailable(ride.RiderID, ride.ID)
			resultChan <- MatchingResult{Success: false}
			return
		case <-ctx.Done():
			resultChan <- MatchingResult{Success: false, Error: ctx.Err()}
			return
		default:
		}

		driverID := dwd.Driver.DriverID

		// Check if driver is still available
		driver, err := s.driverRepo.GetByID(ctx, driverID)
		if err != nil || !driver.IsAvailable() {
			continue
		}

		// Try to acquire lock on driver
		lockKey := "driver:" + driverID
		acquired, err := s.lockManager.AcquireLock(ctx, lockKey, s.config.Matching.DriverResponseTimeout)
		if err != nil || !acquired {
			log.Printf("[MATCHING] Could not acquire lock for driver %s", driverID)
			continue
		}

		log.Printf("[MATCHING] Requesting driver %s (%.2f km away) for ride %s",
			driverID, dwd.Distance, ride.ID)

		// Send notification to driver
		s.notificationService.NotifyDriverOfRideRequest(driverID, ride)

		// Wait for driver response or timeout
		driverTimeout := time.After(s.config.Matching.DriverResponseTimeout)

		select {
		case resp := <-responseChan:
			if resp.DriverID == driverID && resp.Accept {
				// Driver accepted
				log.Printf("[MATCHING] Driver %s accepted ride %s", driverID, ride.ID)
				s.lockManager.ReleaseLock(ctx, lockKey)

				// Accept the ride
				_, err := s.rideService.AcceptRide(ctx, driverID, ride.ID, true)
				if err != nil {
					log.Printf("[MATCHING] Error accepting ride: %v", err)
					continue
				}

				s.notificationService.NotifyRiderOfDriverAccepted(ride.RiderID, driverID, ride.ID)
				resultChan <- MatchingResult{Success: true, DriverID: driverID}
				return
			} else {
				// Driver denied
				log.Printf("[MATCHING] Driver %s denied ride %s", driverID, ride.ID)
				s.lockManager.ReleaseLock(ctx, lockKey)
			}

		case <-driverTimeout:
			log.Printf("[MATCHING] Driver %s timed out for ride %s", driverID, ride.ID)
			s.notificationService.NotifyDriverOfRideTimeout(driverID, ride.ID)
			s.lockManager.ReleaseLock(ctx, lockKey)

		case <-totalTimeout:
			s.lockManager.ReleaseLock(ctx, lockKey)
			log.Printf("[MATCHING] Total timeout exceeded for ride %s", ride.ID)
			s.rideService.FailMatching(ctx, ride.ID)
			s.notificationService.NotifyRiderOfNoDriversAvailable(ride.RiderID, ride.ID)
			resultChan <- MatchingResult{Success: false}
			return
		}
	}

	// No driver accepted
	log.Printf("[MATCHING] No driver accepted ride %s", ride.ID)
	s.rideService.FailMatching(ctx, ride.ID)
	s.notificationService.NotifyRiderOfNoDriversAvailable(ride.RiderID, ride.ID)
	resultChan <- MatchingResult{Success: false}
}

// SubmitDriverResponse allows a driver to respond to a ride request
func (s *MatchingService) SubmitDriverResponse(driverID, rideID string, accept bool) {
	s.driverResponses <- DriverResponse{
		DriverID: driverID,
		RideID:   rideID,
		Accept:   accept,
	}
}
