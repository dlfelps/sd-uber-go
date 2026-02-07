package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"uber/internal/api/middleware"
	"uber/internal/services"
)

// LocationHandler manages driver location tracking endpoints. In a real
// ride-sharing app, drivers send location updates every few seconds while
// online. These updates feed the spatial index used for matching riders with
// nearby drivers.
type LocationHandler struct {
	locationService *services.LocationService
}

// NewLocationHandler creates a LocationHandler with the location service.
func NewLocationHandler(locationService *services.LocationService) *LocationHandler {
	return &LocationHandler{
		locationService: locationService,
	}
}

// UpdateLocationRequest is the JSON body for a driver location ping.
type UpdateLocationRequest struct {
	Lat  float64 `json:"lat" binding:"required"`
	Long float64 `json:"long" binding:"required"`
}

// UpdateLocation handles PATCH /location/update.
// Called frequently by driver apps to report their current GPS position.
// The response includes the computed geohash, which is useful for debugging
// spatial index behavior.
func (h *LocationHandler) UpdateLocation(c *gin.Context) {
	var req UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	driverID := middleware.GetUserID(c)

	location, err := h.locationService.UpdateDriverLocation(c.Request.Context(), driverID, req.Lat, req.Long)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"driver_id": location.DriverID,
		"location": gin.H{
			"lat":  location.Location.Latitude,
			"long": location.Location.Longitude,
		},
		"geohash":    location.Geohash,
		"updated_at": location.UpdatedAt,
	})
}

// GetLocation handles GET /location/:driver_id (debug endpoint, no auth).
// Useful for verifying that driver locations are being tracked correctly.
func (h *LocationHandler) GetLocation(c *gin.Context) {
	driverID := c.Param("driver_id")

	location, err := h.locationService.GetDriverLocation(c.Request.Context(), driverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if location == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "driver location not found"})
		return
	}

	c.JSON(http.StatusOK, location)
}
