// Package handlers contains HTTP handler functions that translate between
// HTTP requests/responses and service-layer calls.
//
// Go Learning Note — Handler Responsibility:
// Handlers should only do three things:
//   1. Parse and validate the incoming request (JSON binding, path params)
//   2. Call the appropriate service method
//   3. Map the service result to an HTTP response (status code + body)
//
// Business logic belongs in the services layer, not here. This separation
// makes handlers thin and easy to test — you can test services independently
// without HTTP concerns.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"uber/internal/api/middleware"
	"uber/internal/domain/entities"
	"uber/internal/services"
)

// RideHandler groups all ride-related HTTP endpoints. It depends on RideService
// for business logic and MatchingService to trigger async driver matching.
type RideHandler struct {
	rideService     *services.RideService
	matchingService *services.MatchingService
}

// NewRideHandler creates a RideHandler with its required service dependencies.
func NewRideHandler(rideService *services.RideService, matchingService *services.MatchingService) *RideHandler {
	return &RideHandler{
		rideService:     rideService,
		matchingService: matchingService,
	}
}

// FareEstimateRequest is the expected JSON body for the fare estimate endpoint.
//
// Go Learning Note — Gin Binding Tags:
// The `binding:"required"` tag tells Gin's ShouldBindJSON to return an error
// if the field is missing or has its zero value. Gin uses the "go-playground/validator"
// library under the hood, so you can also use tags like `binding:"min=1,max=100"`
// or `binding:"email"` for more complex validation. This keeps validation
// declarative and out of your handler logic.
type FareEstimateRequest struct {
	Source      LocationRequest `json:"source" binding:"required"`
	Destination LocationRequest `json:"destination" binding:"required"`
}

// LocationRequest represents a lat/long pair in the API request.
// Note: this is separate from entities.Location because API request types
// and domain types should evolve independently (the API might use "lat/long"
// while the domain uses "Latitude/Longitude").
type LocationRequest struct {
	Lat  float64 `json:"lat" binding:"required"`
	Long float64 `json:"long" binding:"required"`
}

// FareEstimate handles POST /ride/fair-estimate.
// It calculates the estimated fare for a trip without committing the rider.
//
// Go Learning Note — c.ShouldBindJSON:
// ShouldBindJSON reads the request body, unmarshals the JSON into the struct,
// and runs validation. It returns an error if binding or validation fails.
// The "Should" prefix means it doesn't automatically write a 400 response —
// you handle the error yourself. The alternative c.BindJSON() auto-aborts with
// 400 on failure, giving you less control over the error format.
//
// Go Learning Note — c.Request.Context():
// c.Request.Context() returns the standard library context.Context from the
// HTTP request. This is the idiomatic way to pass cancellation signals and
// deadlines through Go code. When the client disconnects, the context is
// cancelled, and well-behaved code will stop work early.
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

// RequestRideRequest is the JSON body for confirming a ride request.
type RequestRideRequest struct {
	RideID string `json:"ride_id" binding:"required"`
}

// RequestRide handles PATCH /ride/request.
// It confirms a previously estimated ride and kicks off async driver matching.
//
// Go Learning Note — Error Mapping Pattern:
// The switch statement maps domain errors (ErrRideNotFound, ErrNotAuthorized)
// to appropriate HTTP status codes. This is a common Go pattern — define
// sentinel errors in the service layer, then map them to HTTP codes in handlers.
// This keeps HTTP concerns out of business logic. In Go 1.13+, you can also
// use errors.Is() for wrapped errors: `if errors.Is(err, services.ErrRideNotFound)`.
//
// Go Learning Note — HTTP 202 Accepted:
// Returning 202 (not 200) signals that the request was accepted for processing
// but not yet completed. The client should poll GET /ride/:id to check the
// matching status. This is the standard REST pattern for async operations.
//
// Go Learning Note — Goroutines:
// The `go func() { ... }()` launches a new goroutine — a lightweight concurrent
// function (not an OS thread). Goroutines are Go's core concurrency primitive.
// They cost only ~2 KB of stack space and are multiplexed onto OS threads by
// the Go runtime scheduler. Here we use one to run matching in the background
// so the HTTP response returns immediately.
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

	// Start async matching process in a separate goroutine.
	// The HTTP response returns immediately with 202 Accepted while matching
	// continues in the background.
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

// GetRide handles GET /ride/:id.
//
// Go Learning Note — URL Path Parameters:
// c.Param("id") extracts the ":id" path parameter from the URL. In Gin,
// path parameters are defined with a colon prefix in route registration
// (e.g., "/ride/:id"). This is handled by Gin's radix tree router, which
// is faster than regex-based routers.
func (h *RideHandler) GetRide(c *gin.Context) {
	rideID := c.Param("id")

	ride, err := h.rideService.GetRide(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		return
	}

	c.JSON(http.StatusOK, ride)
}
