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

// MatchingRequest represents a request to find a driver for a ride.
type MatchingRequest struct {
	RideID   string
	RiderID  string
	Response chan MatchingResult
}

// MatchingResult is the outcome of a matching attempt — either a driver
// was found (Success=true, DriverID set) or matching failed.
type MatchingResult struct {
	Success  bool
	DriverID string
	Error    error
}

// DriverResponse represents a driver's accept/decline response to a ride offer.
type DriverResponse struct {
	DriverID string
	RideID   string
	Accept   bool
}

// MatchingService is the async ride-driver matching engine. When a rider
// requests a ride, this service runs a goroutine that:
//  1. Finds nearby available drivers sorted by distance
//  2. Offers the ride to each driver in order (nearest first)
//  3. Waits for each driver to accept or times out after DriverResponseTimeout
//  4. If no driver accepts within TotalMatchingTimeout, the ride fails
//
// Go Learning Note — Channel-Based Architecture:
// This service uses channels extensively for async communication:
//   - driverResponses: drivers send accept/decline via HTTP → channel
//   - pendingMatches: maps each ride to a per-ride response channel
//   - resultChan: returns the matching outcome to the caller
//
// This is a classic Go concurrency pattern: use channels to communicate between
// goroutines rather than sharing memory. The processDriverResponses goroutine
// acts as a "router" that dispatches incoming driver responses to the correct
// matching goroutine based on rideID.
//
// Go Learning Note — Buffered vs Unbuffered Channels:
// make(chan DriverResponse, 100) creates a buffered channel with capacity 100.
// Buffered channels allow sends without blocking until the buffer is full.
// Unbuffered channels (make(chan T)) block the sender until a receiver is ready.
// Use buffered channels when:
//   - The sender and receiver operate at different speeds
//   - You want fire-and-forget semantics (within buffer limits)
//   - You need to prevent goroutine deadlocks from slow consumers
type MatchingService struct {
	config              *config.Config
	rideService         *RideService
	locationService     *LocationService
	notificationService *NotificationService
	lockManager         *memory.LockManager
	driverRepo          *memory.DriverRepository

	// driverResponses receives all driver accept/decline responses from the HTTP
	// handler. The processDriverResponses goroutine routes each response to the
	// correct matching goroutine.
	driverResponses chan DriverResponse

	// pendingMatches maps rideID → per-ride channel. Each matching goroutine
	// registers its ride here so driver responses can be routed to it.
	pendingMatches map[string]chan DriverResponse
	pendingMu      sync.RWMutex
}

// NewMatchingService creates and starts the matching service. It launches a
// background goroutine to route driver responses.
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

	// Start the response router goroutine.
	go ms.processDriverResponses()

	return ms
}

// processDriverResponses is a long-running goroutine that reads from the
// global driverResponses channel and routes each response to the per-ride
// channel in pendingMatches. This decouples the HTTP handler (which receives
// the driver's response) from the matching goroutine (which is waiting for it).
//
// Go Learning Note — for-range on Channels:
// `for resp := range s.driverResponses` reads from the channel until it's
// closed. This is the idiomatic way to consume all values from a channel.
// The loop blocks when the channel is empty and resumes when a new value arrives.
//
// Go Learning Note — Non-Blocking Send:
// The `select { case ch <- resp: default: }` pattern attempts to send on the
// channel but falls through to `default` if the channel's buffer is full. This
// prevents the router from blocking if a matching goroutine is slow to consume.
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

// StartMatching begins the async matching process for a ride. It returns a
// channel that will receive exactly one MatchingResult when matching completes
// (either successfully or not).
//
// Go Learning Note — Receive-Only Channels:
// The return type `<-chan MatchingResult` is a receive-only channel — the caller
// can only read from it, not write. This is a Go idiom for returning "futures"
// or async results. The caller does `result := <-resultChan` to block until
// the result is ready.
func (s *MatchingService) StartMatching(ctx context.Context, ride *entities.Ride) <-chan MatchingResult {
	resultChan := make(chan MatchingResult, 1)

	go s.matchingLoop(ctx, ride, resultChan)

	return resultChan
}

// matchingLoop is the core matching algorithm. It runs in its own goroutine
// for each ride request. The algorithm:
//  1. Register a per-ride response channel in pendingMatches
//  2. Transition ride to Matching state
//  3. Find nearby available drivers (sorted by distance)
//  4. For each driver: acquire lock → notify → wait for response/timeout
//  5. On accept: transition ride to Accepted, notify rider, return success
//  6. On decline/timeout: release lock, try next driver
//  7. If all drivers exhausted or total timeout: mark ride as Failed
//
// Go Learning Note — time.After:
// time.After(d) returns a channel that receives a value after duration d.
// Used in select statements for timeouts. Note: each call creates a new timer
// that isn't garbage collected until it fires, so avoid using it in tight loops.
// For cancellable/resettable timers, use time.NewTimer instead.
//
// Go Learning Note — chan<- (Send-Only Channel):
// The parameter `resultChan chan<- MatchingResult` is send-only — this
// goroutine can write to it but not read. This enforces the direction of
// communication at compile time.
func (s *MatchingService) matchingLoop(ctx context.Context, ride *entities.Ride, resultChan chan<- MatchingResult) {
	defer close(resultChan)

	// Register a per-ride channel so driver responses can be routed here.
	responseChan := make(chan DriverResponse, 10)
	s.pendingMu.Lock()
	s.pendingMatches[ride.ID] = responseChan
	s.pendingMu.Unlock()

	// Clean up when done: remove from pendingMatches and close the channel.
	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingMatches, ride.ID)
		s.pendingMu.Unlock()
		close(responseChan)
	}()

	// Transition ride from Requested → Matching.
	if err := s.rideService.StartMatching(ctx, ride); err != nil {
		resultChan <- MatchingResult{Success: false, Error: err}
		return
	}

	// Set an overall deadline for the entire matching process.
	totalTimeout := time.After(s.config.Matching.TotalMatchingTimeout)

	// Find nearby available drivers, sorted by distance (nearest first).
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

	// Try each driver in order of proximity (nearest first).
	for _, dwd := range nearbyDrivers {
		// Check if we've exceeded the total timeout or the context was cancelled
		// before trying the next driver.
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
			// No timeout yet — proceed to try this driver.
		}

		driverID := dwd.Driver.DriverID

		// Re-check driver availability (they might have been matched to another
		// ride while we were trying other drivers).
		driver, err := s.driverRepo.GetByID(ctx, driverID)
		if err != nil || !driver.IsAvailable() {
			continue
		}

		// Acquire a distributed lock on this driver to prevent double-booking.
		// If another matching goroutine already locked this driver, skip them.
		lockKey := "driver:" + driverID
		acquired, err := s.lockManager.AcquireLock(ctx, lockKey, s.config.Matching.DriverResponseTimeout)
		if err != nil || !acquired {
			log.Printf("[MATCHING] Could not acquire lock for driver %s", driverID)
			continue
		}

		log.Printf("[MATCHING] Requesting driver %s (%.2f km away) for ride %s",
			driverID, dwd.Distance, ride.ID)

		// Notify the driver about the ride request (in production, this would
		// be a push notification via FCM/APNs).
		s.notificationService.NotifyDriverOfRideRequest(driverID, ride)

		// Wait for this specific driver to respond, or timeout.
		driverTimeout := time.After(s.config.Matching.DriverResponseTimeout)

		select {
		case resp := <-responseChan:
			if resp.DriverID == driverID && resp.Accept {
				// Driver accepted the ride.
				log.Printf("[MATCHING] Driver %s accepted ride %s", driverID, ride.ID)
				s.lockManager.ReleaseLock(ctx, lockKey)

				_, err := s.rideService.AcceptRide(ctx, driverID, ride.ID, true)
				if err != nil {
					log.Printf("[MATCHING] Error accepting ride: %v", err)
					continue
				}

				s.notificationService.NotifyRiderOfDriverAccepted(ride.RiderID, driverID, ride.ID)
				resultChan <- MatchingResult{Success: true, DriverID: driverID}
				return
			} else {
				// Driver declined — release lock and try next driver.
				log.Printf("[MATCHING] Driver %s denied ride %s", driverID, ride.ID)
				s.lockManager.ReleaseLock(ctx, lockKey)
			}

		case <-driverTimeout:
			// Driver didn't respond within the timeout window.
			log.Printf("[MATCHING] Driver %s timed out for ride %s", driverID, ride.ID)
			s.notificationService.NotifyDriverOfRideTimeout(driverID, ride.ID)
			s.lockManager.ReleaseLock(ctx, lockKey)

		case <-totalTimeout:
			// Overall matching timeout exceeded while waiting for this driver.
			s.lockManager.ReleaseLock(ctx, lockKey)
			log.Printf("[MATCHING] Total timeout exceeded for ride %s", ride.ID)
			s.rideService.FailMatching(ctx, ride.ID)
			s.notificationService.NotifyRiderOfNoDriversAvailable(ride.RiderID, ride.ID)
			resultChan <- MatchingResult{Success: false}
			return
		}
	}

	// All nearby drivers were tried and none accepted.
	log.Printf("[MATCHING] No driver accepted ride %s", ride.ID)
	s.rideService.FailMatching(ctx, ride.ID)
	s.notificationService.NotifyRiderOfNoDriversAvailable(ride.RiderID, ride.ID)
	resultChan <- MatchingResult{Success: false}
}

// SubmitDriverResponse is called by the HTTP handler when a driver accepts or
// declines a ride. It sends the response through the driverResponses channel,
// which is consumed by processDriverResponses and routed to the matching loop.
func (s *MatchingService) SubmitDriverResponse(driverID, rideID string, accept bool) {
	s.driverResponses <- DriverResponse{
		DriverID: driverID,
		RideID:   rideID,
		Accept:   accept,
	}
}
