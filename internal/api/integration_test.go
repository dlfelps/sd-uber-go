package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"uber/internal/api/handlers"
	"uber/internal/config"
	"uber/internal/geo"
	"uber/internal/repository/memory"
	"uber/internal/services"
)

func setupTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)

	cfg := config.NewDefaultConfig()
	cfg.Matching.DriverResponseTimeout = 1 * time.Second
	cfg.Matching.TotalMatchingTimeout = 3 * time.Second

	riderRepo := memory.NewRiderRepository()
	driverRepo := memory.NewDriverRepository()
	rideRepo := memory.NewRideRepository()
	locationRepo := memory.NewLocationRepository()
	lockManager := memory.NewLockManager()
	spatialIndex := geo.NewSpatialIndex(cfg.Geo.GeohashPrecision)

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

	rideHandler := handlers.NewRideHandler(rideService, matchingService)
	driverHandler := handlers.NewDriverHandler(rideService, matchingService, notificationService)
	locationHandler := handlers.NewLocationHandler(locationService)

	router := NewRouter(rideHandler, driverHandler, locationHandler)
	engine := gin.New()
	router.Setup(engine)

	return engine
}

func TestHealthEndpoint(t *testing.T) {
	engine := setupTestServer()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFareEstimateEndpoint(t *testing.T) {
	engine := setupTestServer()

	body := `{"source":{"lat":37.77,"long":-122.41},"destination":{"lat":37.78,"long":-122.40}}`
	req, _ := http.NewRequest("POST", "/ride/fair-estimate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer rider-1")

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["ride_id"] == nil {
		t.Error("Expected ride_id in response")
	}
	if response["fare"] == nil {
		t.Error("Expected fare in response")
	}
}

func TestLocationUpdateEndpoint(t *testing.T) {
	engine := setupTestServer()

	body := `{"lat":37.771,"long":-122.411}`
	req, _ := http.NewRequest("PATCH", "/location/update", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer driver-1")

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["driver_id"] != "driver-1" {
		t.Errorf("Expected driver_id driver-1, got %v", response["driver_id"])
	}
	if response["geohash"] == nil {
		t.Error("Expected geohash in response")
	}
}

func TestRideRequestEndpoint(t *testing.T) {
	engine := setupTestServer()

	// First, add a driver nearby
	driverBody := `{"lat":37.771,"long":-122.411}`
	driverReq, _ := http.NewRequest("PATCH", "/location/update", bytes.NewBufferString(driverBody))
	driverReq.Header.Set("Content-Type", "application/json")
	driverReq.Header.Set("Authorization", "Bearer driver-1")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, driverReq)

	// Create fare estimate
	estimateBody := `{"source":{"lat":37.77,"long":-122.41},"destination":{"lat":37.78,"long":-122.40}}`
	estimateReq, _ := http.NewRequest("POST", "/ride/fair-estimate", bytes.NewBufferString(estimateBody))
	estimateReq.Header.Set("Content-Type", "application/json")
	estimateReq.Header.Set("Authorization", "Bearer rider-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, estimateReq)

	var estimateResponse map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &estimateResponse)
	rideID := estimateResponse["ride_id"].(string)

	// Request the ride
	requestBody := `{"ride_id":"` + rideID + `"}`
	requestReq, _ := http.NewRequest("PATCH", "/ride/request", bytes.NewBufferString(requestBody))
	requestReq.Header.Set("Content-Type", "application/json")
	requestReq.Header.Set("Authorization", "Bearer rider-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, requestReq)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestDriverAcceptEndpoint(t *testing.T) {
	engine := setupTestServer()

	// Add driver
	driverBody := `{"lat":37.771,"long":-122.411}`
	driverReq, _ := http.NewRequest("PATCH", "/location/update", bytes.NewBufferString(driverBody))
	driverReq.Header.Set("Content-Type", "application/json")
	driverReq.Header.Set("Authorization", "Bearer driver-1")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, driverReq)

	// Create and request ride
	estimateBody := `{"source":{"lat":37.77,"long":-122.41},"destination":{"lat":37.78,"long":-122.40}}`
	estimateReq, _ := http.NewRequest("POST", "/ride/fair-estimate", bytes.NewBufferString(estimateBody))
	estimateReq.Header.Set("Content-Type", "application/json")
	estimateReq.Header.Set("Authorization", "Bearer rider-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, estimateReq)

	var estimateResponse map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &estimateResponse)
	rideID := estimateResponse["ride_id"].(string)

	requestBody := `{"ride_id":"` + rideID + `"}`
	requestReq, _ := http.NewRequest("PATCH", "/ride/request", bytes.NewBufferString(requestBody))
	requestReq.Header.Set("Content-Type", "application/json")
	requestReq.Header.Set("Authorization", "Bearer rider-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, requestReq)

	// Give matching time to start
	time.Sleep(100 * time.Millisecond)

	// Driver accepts
	acceptBody := `{"ride_id":"` + rideID + `","accept":true}`
	acceptReq, _ := http.NewRequest("PATCH", "/ride/driver/accept", bytes.NewBufferString(acceptBody))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq.Header.Set("Authorization", "Bearer driver-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, acceptReq)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestCompleteRideFlow(t *testing.T) {
	engine := setupTestServer()

	// 1. Driver updates location
	driverBody := `{"lat":37.771,"long":-122.411}`
	driverReq, _ := http.NewRequest("PATCH", "/location/update", bytes.NewBufferString(driverBody))
	driverReq.Header.Set("Content-Type", "application/json")
	driverReq.Header.Set("Authorization", "Bearer driver-1")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, driverReq)
	if w.Code != http.StatusOK {
		t.Fatalf("Driver location update failed: %d", w.Code)
	}

	// 2. Rider gets fare estimate
	estimateBody := `{"source":{"lat":37.77,"long":-122.41},"destination":{"lat":37.78,"long":-122.40}}`
	estimateReq, _ := http.NewRequest("POST", "/ride/fair-estimate", bytes.NewBufferString(estimateBody))
	estimateReq.Header.Set("Content-Type", "application/json")
	estimateReq.Header.Set("Authorization", "Bearer rider-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, estimateReq)
	if w.Code != http.StatusOK {
		t.Fatalf("Fare estimate failed: %d", w.Code)
	}

	var estimateResponse map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &estimateResponse)
	rideID := estimateResponse["ride_id"].(string)

	// 3. Rider requests ride
	requestBody := `{"ride_id":"` + rideID + `"}`
	requestReq, _ := http.NewRequest("PATCH", "/ride/request", bytes.NewBufferString(requestBody))
	requestReq.Header.Set("Content-Type", "application/json")
	requestReq.Header.Set("Authorization", "Bearer rider-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, requestReq)
	if w.Code != http.StatusAccepted {
		t.Fatalf("Ride request failed: %d", w.Code)
	}

	// Wait for matching to start
	time.Sleep(100 * time.Millisecond)

	// 4. Driver accepts
	acceptBody := `{"ride_id":"` + rideID + `","accept":true}`
	acceptReq, _ := http.NewRequest("PATCH", "/ride/driver/accept", bytes.NewBufferString(acceptBody))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq.Header.Set("Authorization", "Bearer driver-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, acceptReq)
	if w.Code != http.StatusOK {
		t.Fatalf("Driver accept failed: %d", w.Code)
	}

	// Wait for acceptance to process
	time.Sleep(200 * time.Millisecond)

	// 5. Driver starts picking up
	pickupBody := `{"ride_id":"` + rideID + `","status":"picking_up"}`
	pickupReq, _ := http.NewRequest("PATCH", "/ride/driver/update", bytes.NewBufferString(pickupBody))
	pickupReq.Header.Set("Content-Type", "application/json")
	pickupReq.Header.Set("Authorization", "Bearer driver-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, pickupReq)
	if w.Code != http.StatusOK {
		t.Fatalf("Pickup update failed: %d - %s", w.Code, w.Body.String())
	}

	// 6. Driver starts trip
	tripBody := `{"ride_id":"` + rideID + `","status":"in_progress"}`
	tripReq, _ := http.NewRequest("PATCH", "/ride/driver/update", bytes.NewBufferString(tripBody))
	tripReq.Header.Set("Content-Type", "application/json")
	tripReq.Header.Set("Authorization", "Bearer driver-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, tripReq)
	if w.Code != http.StatusOK {
		t.Fatalf("Trip start failed: %d - %s", w.Code, w.Body.String())
	}

	// 7. Driver completes trip
	completeBody := `{"ride_id":"` + rideID + `","status":"completed"}`
	completeReq, _ := http.NewRequest("PATCH", "/ride/driver/update", bytes.NewBufferString(completeBody))
	completeReq.Header.Set("Content-Type", "application/json")
	completeReq.Header.Set("Authorization", "Bearer driver-1")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, completeReq)
	if w.Code != http.StatusOK {
		t.Fatalf("Trip complete failed: %d - %s", w.Code, w.Body.String())
	}

	// Verify final state
	var completeResponse map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &completeResponse)
	if completeResponse["status"] != "completed" {
		t.Errorf("Expected status completed, got %v", completeResponse["status"])
	}
}

func TestUnauthorizedAccess(t *testing.T) {
	engine := setupTestServer()

	// Try to access rider endpoint without auth
	body := `{"source":{"lat":37.77,"long":-122.41},"destination":{"lat":37.78,"long":-122.40}}`
	req, _ := http.NewRequest("POST", "/ride/fair-estimate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestDriverAccessingRiderEndpoint(t *testing.T) {
	engine := setupTestServer()

	body := `{"source":{"lat":37.77,"long":-122.41},"destination":{"lat":37.78,"long":-122.40}}`
	req, _ := http.NewRequest("POST", "/ride/fair-estimate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer driver-1") // Driver trying to access rider endpoint

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestRiderAccessingDriverEndpoint(t *testing.T) {
	engine := setupTestServer()

	body := `{"lat":37.77,"long":-122.41}`
	req, _ := http.NewRequest("PATCH", "/location/update", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer rider-1") // Rider trying to access driver endpoint

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}
