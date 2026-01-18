package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"uber/internal/api/middleware"
	"uber/internal/domain/entities"
	"uber/internal/services"
)

type DriverHandler struct {
	rideService         *services.RideService
	matchingService     *services.MatchingService
	notificationService *services.NotificationService
}

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

type AcceptRideRequest struct {
	RideID string `json:"ride_id" binding:"required"`
	Accept bool   `json:"accept"`
}

// AcceptRide handles PATCH /ride/driver/accept
func (h *DriverHandler) AcceptRide(c *gin.Context) {
	var req AcceptRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	driverID := middleware.GetUserID(c)

	// Submit response to matching service
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

type UpdateRideStatusRequest struct {
	RideID string `json:"ride_id" binding:"required"`
	Status string `json:"status" binding:"required"`
}

// UpdateRideStatus handles PATCH /ride/driver/update
func (h *DriverHandler) UpdateRideStatus(c *gin.Context) {
	var req UpdateRideStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	driverID := middleware.GetUserID(c)

	// Map status string to enum
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

	// Send appropriate notifications
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
