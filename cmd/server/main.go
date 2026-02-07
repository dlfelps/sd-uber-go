// Package main is the entry point for the ride-sharing server.
//
// Go Learning Note — "cmd/" directory convention:
// In idiomatic Go projects, executables live under cmd/<name>/main.go.
// This keeps the project root clean and allows multiple binaries in one repo
// (e.g., cmd/server/, cmd/worker/, cmd/cli/). Each subdirectory under cmd/
// must be package main with a main() function.
//
// Go Learning Note — Dependency Injection:
// Go does not have a built-in DI framework like Java's Spring. Instead,
// dependencies are wired manually in main(). This is intentional — Go favors
// explicit, readable code over "magic." You construct each layer (repos →
// services → handlers → router) and pass dependencies as constructor arguments.
// This makes the dependency graph visible and easy to test.
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
	// Load configuration.
	// Go Learning Note — No config files in MVP:
	// A common Go pattern is to start with hardcoded defaults via a constructor
	// like NewDefaultConfig(), then layer on environment variables or config files
	// later. Libraries like "github.com/spf13/viper" or "github.com/kelseyhightower/envconfig"
	// are popular for production config management.
	cfg := config.NewDefaultConfig()

	// Initialize repositories (data access layer).
	// Go Learning Note — The Repository Pattern:
	// Each repository encapsulates data access for one domain entity. Using
	// in-memory maps here makes the MVP simple, but the pattern allows swapping
	// to PostgreSQL, Redis, etc. later without changing service code — as long as
	// the repository satisfies the same interface.
	riderRepo := memory.NewRiderRepository()
	driverRepo := memory.NewDriverRepository()
	rideRepo := memory.NewRideRepository()
	locationRepo := memory.NewLocationRepository()
	lockManager := memory.NewLockManager()

	// Initialize spatial index for fast geolocation queries.
	// The precision parameter (6) means geohash cells of ~1.2 km — a good
	// tradeoff between search accuracy and the number of cells to scan.
	spatialIndex := geo.NewSpatialIndex(cfg.Geo.GeohashPrecision)

	// Initialize services (business logic layer).
	// Go Learning Note — Layered Architecture:
	// Services depend on repositories (never the reverse). Handlers depend on
	// services. This unidirectional flow makes the code testable: you can test
	// services by providing mock repositories, and test handlers by providing
	// mock services.
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

	// Initialize handlers (HTTP transport layer).
	// Handlers translate HTTP requests into service calls and service responses
	// into HTTP responses. They should contain no business logic themselves.
	rideHandler := handlers.NewRideHandler(rideService, matchingService)
	driverHandler := handlers.NewDriverHandler(rideService, matchingService, notificationService)
	locationHandler := handlers.NewLocationHandler(locationService)

	// Setup router — wires handlers to URL paths with middleware.
	router := api.NewRouter(rideHandler, driverHandler, locationHandler)

	// Create Gin engine with default middleware (logger + recovery).
	// Go Learning Note — gin.Default() vs gin.New():
	// gin.Default() includes Logger and Recovery middleware automatically.
	// gin.New() gives you a bare engine. Recovery middleware catches panics in
	// handlers and returns a 500 instead of crashing the server — essential for
	// production.
	engine := gin.Default()
	router.Setup(engine)

	// Start server.
	// Go Learning Note — log.Fatalf:
	// log.Fatalf calls os.Exit(1) after logging, so deferred functions won't run.
	// For graceful shutdown in production, use http.Server with signal handling
	// and server.Shutdown(ctx) instead of engine.Run().
	log.Printf("Starting Uber Clone server on %s", cfg.Server.Port)
	if err := engine.Run(cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
