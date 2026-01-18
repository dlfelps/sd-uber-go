package api

import (
	"github.com/gin-gonic/gin"
	"uber/internal/api/handlers"
	"uber/internal/api/middleware"
)

type Router struct {
	rideHandler     *handlers.RideHandler
	driverHandler   *handlers.DriverHandler
	locationHandler *handlers.LocationHandler
}

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

func (r *Router) Setup(engine *gin.Engine) {
	// Health check endpoint
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Protected routes
	api := engine.Group("/")
	api.Use(middleware.MockAuth())
	{
		// Rider endpoints
		riderRoutes := api.Group("/ride")
		riderRoutes.Use(middleware.RequireRider())
		{
			riderRoutes.POST("/fair-estimate", r.rideHandler.FareEstimate)
			riderRoutes.PATCH("/request", r.rideHandler.RequestRide)
		}

		// Driver endpoints
		driverRoutes := api.Group("/")
		driverRoutes.Use(middleware.RequireDriver())
		{
			driverRoutes.PATCH("/location/update", r.locationHandler.UpdateLocation)
			driverRoutes.PATCH("/ride/driver/accept", r.driverHandler.AcceptRide)
			driverRoutes.PATCH("/ride/driver/update", r.driverHandler.UpdateRideStatus)
		}

		// Shared endpoints (both rider and driver can access)
		api.GET("/ride/:id", r.rideHandler.GetRide)
	}

	// Debug endpoints (no auth for testing)
	debug := engine.Group("/debug")
	{
		debug.GET("/location/:driver_id", r.locationHandler.GetLocation)
	}
}
