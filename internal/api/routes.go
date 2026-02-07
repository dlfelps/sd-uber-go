// Package api wires together HTTP routes, middleware, and handlers.
//
// Go Learning Note — Package Naming:
// Go packages are named after what they provide, not what they contain. "api"
// is a good name because this package provides the API layer. Avoid names like
// "utils", "common", or "helpers" which are vague — if you must have utilities,
// place them in a package named after their domain (e.g., "pricing", "geo").
package api

import (
	"github.com/gin-gonic/gin"
	"uber/internal/api/handlers"
	"uber/internal/api/middleware"
)

// Router holds references to all HTTP handlers and configures URL routing.
// It acts as the composition root for the HTTP layer.
type Router struct {
	rideHandler     *handlers.RideHandler
	driverHandler   *handlers.DriverHandler
	locationHandler *handlers.LocationHandler
}

// NewRouter creates a Router with all required handler dependencies.
func NewRouter(
	rideHandler *handlers.RideHandler,
	driverHandler *handlers.DriverHandler,
	locationHandler *handlers.LocationHandler,
) *Router {
	return &Router{
		rideHandler:     rideHandler,
		driverHandler:   driverHandler,
		locationHandler: locationHandler,
	}
}

// Setup registers all routes and middleware on the Gin engine.
//
// Go Learning Note — Route Groups in Gin:
// engine.Group() creates a route group that shares a common path prefix and/or
// middleware. This is how you apply authentication to a subset of routes.
// The braces { } after .Use() are purely cosmetic — they're just Go block
// scoping for readability, not Gin syntax. They help visually group routes
// that share middleware.
//
// Go Learning Note — gin.H:
// gin.H is shorthand for map[string]interface{} (or map[string]any in Go 1.18+).
// It's used for building JSON responses inline. For typed responses, define a
// struct instead — it's more maintainable and gives you compile-time field checks.
//
// Go Learning Note — HTTP Methods:
// REST convention: POST for creation, GET for retrieval, PATCH for partial
// updates, PUT for full replacement, DELETE for removal. This API uses PATCH
// for ride state transitions since they modify specific fields, not the full
// resource. POST is used for fare estimates since they create a new ride entity.
func (r *Router) Setup(engine *gin.Engine) {
	// Health check endpoint — no authentication required.
	// Load balancers and orchestrators (Kubernetes, ECS) call this to verify
	// the server is running before routing traffic to it.
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Protected routes — all routes in this group require authentication.
	api := engine.Group("/")
	api.Use(middleware.MockAuth())
	{
		// Rider endpoints — only authenticated riders can access these.
		// Middleware is applied in order: MockAuth runs first (set by the
		// parent group), then RequireRider checks the user type.
		riderRoutes := api.Group("/ride")
		riderRoutes.Use(middleware.RequireRider())
		{
			riderRoutes.POST("/fair-estimate", r.rideHandler.FareEstimate)
			riderRoutes.PATCH("/request", r.rideHandler.RequestRide)
		}

		// Driver endpoints — only authenticated drivers can access these.
		driverRoutes := api.Group("/")
		driverRoutes.Use(middleware.RequireDriver())
		{
			driverRoutes.PATCH("/location/update", r.locationHandler.UpdateLocation)
			driverRoutes.PATCH("/ride/driver/accept", r.driverHandler.AcceptRide)
			driverRoutes.PATCH("/ride/driver/update", r.driverHandler.UpdateRideStatus)
		}

		// Shared endpoints — both rider and driver can access.
		// No additional role middleware is applied here; MockAuth alone suffices.
		api.GET("/ride/:id", r.rideHandler.GetRide)
	}

	// Debug endpoints — no authentication, only for testing and development.
	// In production, these would be removed or moved behind an admin auth layer.
	debug := engine.Group("/debug")
	{
		debug.GET("/location/:driver_id", r.locationHandler.GetLocation)
	}
}
