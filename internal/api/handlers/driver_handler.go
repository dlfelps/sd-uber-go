package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"uber/internal/api/middleware"
	"uber/internal/domain/entities"
	"uber/internal/services"
)

// DriverHandler groups all driver-facing HTTP endpoints. Drivers use these
// to accept/decline ride requests and update ride status (picking up, in
// progress, completed).
type DriverHandler struct {
	rideService         *services.RideService
	matchingService     *services.MatchingService
	notificationService *services.NotificationService
}

// NewDriverHandler creates a DriverHandler with its required service dependencies.
func NewDriverHandler(
	rideService *services.RideService,
	matchingService *services.MatchingService,
	notificationService *services.NotificationService,
) *DriverHandler {
	return &DriverHandler{
		rideService:         rideService,
		matchingService:     matchingService,
		notificationService: notificationService,
	}
}

// AcceptRideRequest is the JSON body for a driver's accept/decline response.
// Note that Accept is a bool without `binding:"required"` — in Go, an omitted
// bool defaults to false, which conveniently means "decline" if not specified.
type AcceptRideRequest struct {
	RideID string `json:"ride_id" binding:"required"`
	Accept bool   `json:"accept"`
}

// AcceptRide handles PATCH /ride/driver/accept.
// The driver's response is submitted asynchronously to the matching service
// via a channel, which is waiting for this driver's reply. The HTTP response
// returns immediately — the actual ride state transition happens in the
// matching goroutine.
func (h *DriverHandler) AcceptRide(c *gin.Context) {
	var req AcceptRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	driverID := middleware.GetUserID(c)

	// Submit response to matching service via the driver response channel.
	h.matchingService.SubmitDriverResponse(driverID, req.RideID, req.Accept)

	if req.Accept {
		c.JSON(http.StatusOK, gin.H{
			"message": "ride acceptance submitted",
			"ride_id": req.RideID,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"message": "ride declined",
			"ride_id": req.RideID,
		})
	}
}

// UpdateRideStatusRequest is the JSON body for advancing a ride through its
// lifecycle. Drivers call this to signal pickup, trip start, and completion.
type UpdateRideStatusRequest struct {
	RideID string `json:"ride_id" binding:"required"`
	Status string `json:"status" binding:"required"`
}

// UpdateRideStatus handles PATCH /ride/driver/update.
// It maps the API status string to the domain RideStatus type and delegates
// to the service layer for the actual state transition. After a successful
// transition, it triggers the appropriate rider notification.
//
// Go Learning Note — Switch Statements:
// Go's switch is more powerful than C/Java's: cases don't fall through by
// default (no "break" needed), and you can switch on strings, types, or
// complex expressions. The "default" case handles unexpected values, providing
// a safety net. If you do want fallthrough, Go has an explicit `fallthrough`
// keyword, but it's rarely used.
func (h *DriverHandler) UpdateRideStatus(c *gin.Context) {
	var req UpdateRideStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	driverID := middleware.GetUserID(c)

	// Map the raw status string from the API to a typed RideStatus enum.
	// This boundary validation ensures only known statuses enter the domain layer.
	var newStatus entities.RideStatus
	switch req.Status {
	case "picking_up":
		newStatus = entities.RideStatusPickingUp
	case "in_progress":
		newStatus = entities.RideStatusInProgress
	case "completed":
		newStatus = entities.RideStatusCompleted
	case "cancelled":
		newStatus = entities.RideStatusCancelled
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	ride, err := h.rideService.UpdateRideStatus(c.Request.Context(), driverID, req.RideID, newStatus)
	if err != nil {
		switch err {
		case services.ErrRideNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		case services.ErrNotAuthorized:
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized"})
		case services.ErrInvalidTransition:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status transition"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// Send appropriate notifications based on the new ride state.
	switch newStatus {
	case entities.RideStatusPickingUp:
		h.notificationService.NotifyRiderOfDriverArriving(ride.RiderID, driverID, ride.ID)
	case entities.RideStatusInProgress:
		h.notificationService.NotifyRiderOfTripStarted(ride.RiderID, ride.ID)
	case entities.RideStatusCompleted:
		h.notificationService.NotifyRiderOfTripCompleted(ride.RiderID, ride.ID, ride.ActualFare)
	}

	c.JSON(http.StatusOK, ride)
}
