# Uber Clone Backend

A real-time ride-sharing backend built in Go with Gin, using in-memory storage for a monolith MVP.

## Features

- **Fare Estimation**: Calculate ride prices based on distance and time
- **Driver Location Tracking**: Real-time geospatial indexing with geohash
- **Async Ride Matching**: Background matching with driver timeouts
- **Ride Lifecycle Management**: Full state machine for ride status
- **Mock Authentication**: JWT-style auth for riders and drivers

## Project Structure

```
uber/
├── cmd/server/main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── handlers/               # HTTP handlers
│   │   ├── middleware/auth.go      # Mock JWT auth
│   │   └── routes.go               # Route registration
│   ├── config/config.go            # App configuration
│   ├── domain/entities/            # Domain models
│   ├── services/                   # Business logic
│   ├── repository/memory/          # In-memory storage
│   └── geo/                        # Geospatial utilities
├── pkg/utils/                      # Shared utilities
├── go.mod
├── Makefile
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.21 or later

### Installation

```bash
# Clone and enter directory
cd uber

# Download dependencies
go mod download

# Run the server
go run cmd/server/main.go
```

The server starts on `http://localhost:8080`.

## API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | None | Health check |
| `/ride/fair-estimate` | POST | Rider | Get price/ETA for route |
| `/ride/request` | PATCH | Rider | Start async matching |
| `/ride/:id` | GET | Any | Get ride details |
| `/location/update` | PATCH | Driver | Update driver position |
| `/ride/driver/accept` | PATCH | Driver | Accept/deny ride |
| `/ride/driver/update` | PATCH | Driver | Update ride status |

## Authentication

Use the `Authorization` header with format `Bearer <user-id>`:
- Riders: `Bearer rider-1`, `Bearer rider-2`, etc.
- Drivers: `Bearer driver-1`, `Bearer driver-2`, etc.

## Testing the API

### 1. Health Check
```bash
curl http://localhost:8080/health
```

### 2. Add Driver Location
```bash
curl -X PATCH http://localhost:8080/location/update \
  -H "Authorization: Bearer driver-1" \
  -H "Content-Type: application/json" \
  -d '{"lat":37.771,"long":-122.411}'
```

### 3. Get Fare Estimate
```bash
curl -X POST http://localhost:8080/ride/fair-estimate \
  -H "Authorization: Bearer rider-1" \
  -H "Content-Type: application/json" \
  -d '{"source":{"lat":37.77,"long":-122.41},"destination":{"lat":37.78,"long":-122.40}}'
```

### 4. Request Ride
```bash
curl -X PATCH http://localhost:8080/ride/request \
  -H "Authorization: Bearer rider-1" \
  -H "Content-Type: application/json" \
  -d '{"ride_id":"<ride-id-from-step-3>"}'
```

### 5. Driver Accepts
```bash
curl -X PATCH http://localhost:8080/ride/driver/accept \
  -H "Authorization: Bearer driver-1" \
  -H "Content-Type: application/json" \
  -d '{"ride_id":"<ride-id>","accept":true}'
```

### 6. Driver Updates Status
```bash
# Start pickup
curl -X PATCH http://localhost:8080/ride/driver/update \
  -H "Authorization: Bearer driver-1" \
  -H "Content-Type: application/json" \
  -d '{"ride_id":"<ride-id>","status":"picking_up"}'

# Start trip
curl -X PATCH http://localhost:8080/ride/driver/update \
  -H "Authorization: Bearer driver-1" \
  -H "Content-Type: application/json" \
  -d '{"ride_id":"<ride-id>","status":"in_progress"}'

# Complete trip
curl -X PATCH http://localhost:8080/ride/driver/update \
  -H "Authorization: Bearer driver-1" \
  -H "Content-Type: application/json" \
  -d '{"ride_id":"<ride-id>","status":"completed"}'
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

## Ride Status Flow

```
Estimate → Requested → Matching → Accepted → PickingUp → InProgress → Completed
                         ↓
                       Failed (no driver found)
```

## Configuration

Default configuration in `internal/config/config.go`:
- Server port: `:8080`
- Driver response timeout: 10 seconds
- Total matching timeout: 60 seconds
- Search radius: 5 km
- Geohash precision: 6

## Technical Highlights

### Geospatial Search
- Uses geohash encoding (precision 6, ~1.2km cells)
- Searches center cell + 8 neighbors
- Filters by Haversine distance

### Async Matching
- Background goroutine per ride request
- Locks drivers during request to prevent double-booking
- 10-second TTL for driver response
- Iterates through drivers by proximity

### Thread Safety
- All repositories use `sync.RWMutex`
- Lock manager with TTL for distributed locking
- Background cleanup of expired locks
