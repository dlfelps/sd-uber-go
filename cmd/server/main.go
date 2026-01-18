package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"uber/internal/api"
	"uber/internal/api/handlers"
	"uber/internal/config"
	"uber/internal/geo"
	"uber/internal/repository/memory"
	"uber/internal/services"
)

func main() {
	// Load configuration
	cfg := config.NewDefaultConfig()

	// Initialize repositories
	riderRepo := memory.NewRiderRepository()
	driverRepo := memory.NewDriverRepository()
	rideRepo := memory.NewRideRepository()
	locationRepo := memory.NewLocationRepository()
	lockManager := memory.NewLockManager()

	// Initialize spatial index
	spatialIndex := geo.NewSpatialIndex(cfg.Geo.GeohashPrecision)

	// Initialize services
	notificationService := services.NewNotificationService()
	locationService := services.NewLocationService(spatialIndex, driverRepo, locationRepo)
	rideService := services.NewRideService(rideRepo, riderRepo, driverRepo, cfg)
	matchingService := services.NewMatchingService(
		cfg,
		rideService,
		locationService,
		notificationService,
		lockManager,
		driverRepo,
	)

	// Initialize handlers
	rideHandler := handlers.NewRideHandler(rideService, matchingService)
	driverHandler := handlers.NewDriverHandler(rideService, matchingService, notificationService)
	locationHandler := handlers.NewLocationHandler(locationService)

	// Setup router
	router := api.NewRouter(rideHandler, driverHandler, locationHandler)

	// Create Gin engine
	engine := gin.Default()
	router.Setup(engine)

	// Start server
	log.Printf("Starting Uber Clone server on %s", cfg.Server.Port)
	if err := engine.Run(cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
