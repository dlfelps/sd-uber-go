package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"uber/internal/api/middleware"
	"uber/internal/domain/entities"
	"uber/internal/services"
)

type RideHandler struct {
	rideService     *services.RideService
	matchingService *services.MatchingService
}

func NewRideHandler(rideService *services.RideService, matchingService *services.MatchingService) *RideHandler {
	return &RideHandler{
		rideService:     rideService,
		matchingService: matchingService,
	}
}

type FareEstimateRequest struct {
	Source      LocationRequest `json:"source" binding:"required"`
	Destination LocationRequest `json:"destination" binding:"required"`
}

type LocationRequest struct {
	Lat  float64 `json:"lat" binding:"required"`
	Long float64 `json:"long" binding:"required"`
}

// FareEstimate handles POST /ride/fair-estimate
func (h *RideHandler) FareEstimate(c *gin.Context) {
	var req FareEstimateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	riderID := middleware.GetUserID(c)

	estimate, err := h.rideService.CreateFareEstimate(c.Request.Context(), riderID, services.FareEstimateRequest{
		Source: entities.Location{
			Latitude:  req.Source.Lat,
			Longitude: req.Source.Long,
		},
		Destination: entities.Location{
			Latitude:  req.Destination.Lat,
			Longitude: req.Destination.Long,
		},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, estimate)
}

type RequestRideRequest struct {
	RideID string `json:"ride_id" binding:"required"`
}

// RequestRide handles PATCH /ride/request
func (h *RideHandler) RequestRide(c *gin.Context) {
	var req RequestRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	riderID := middleware.GetUserID(c)

	ride, err := h.rideService.RequestRide(c.Request.Context(), riderID, req.RideID)
	if err != nil {
		switch err {
		case services.ErrRideNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		case services.ErrNotAuthorized:
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized"})
		case services.ErrActiveRideExists:
			c.JSON(http.StatusConflict, gin.H{"error": "active ride already exists"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// Start async matching process
	go func() {
		resultChan := h.matchingService.StartMatching(c.Request.Context(), ride)
		result := <-resultChan
		if result.Success {
			// Matching succeeded - ride is now accepted
		} else {
			// Matching failed - ride status updated to failed
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"ride_id": ride.ID,
		"status":  ride.Status,
		"message": "matching in progress",
	})
}

// GetRide handles GET /ride/:id
func (h *RideHandler) GetRide(c *gin.Context) {
	rideID := c.Param("id")

	ride, err := h.rideService.GetRide(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		return
	}

	c.JSON(http.StatusOK, ride)
}
