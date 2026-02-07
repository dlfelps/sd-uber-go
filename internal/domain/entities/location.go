package entities

import "time"

// Location represents a geographic coordinate pair (latitude/longitude).
//
// Go Learning Note — Value Types vs Reference Types:
// Location is a small, immutable data holder. NewLocation returns it by value
// (not a pointer), which is idiomatic for small structs. Value types are copied
// on assignment, which is fine here since Location is only 16 bytes (two float64s).
// By contrast, larger or mutable structs (like Driver, Ride) are returned as
// pointers to avoid expensive copies and allow shared mutation.
//
// Go Learning Note — Custom JSON Field Names:
// The struct tag `json:"lat"` makes this field serialize as "lat" instead of
// "Latitude" in JSON. The "omitempty" option (seen on other structs) omits the
// field from JSON output when it holds its zero value.
type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"long"`
}

// DriverLocation combines a driver's identity with their geographic position
// and a geohash encoding. The Geohash field enables O(1) lookups into the
// spatial index — instead of scanning all drivers, you only check drivers in
// the same geohash cell and its 8 neighbors.
type DriverLocation struct {
	DriverID  string    `json:"driver_id"`
	Location  Location  `json:"location"`
	Geohash   string    `json:"geohash"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewLocation creates a Location value from latitude and longitude.
func NewLocation(lat, long float64) Location {
	return Location{
		Latitude:  lat,
		Longitude: long,
	}
}

// NewDriverLocation creates a DriverLocation with the current timestamp.
// The geohash parameter should be pre-computed by the geo package.
func NewDriverLocation(driverID string, lat, long float64, geohash string) *DriverLocation {
	return &DriverLocation{
		DriverID: driverID,
		Location: Location{
			Latitude:  lat,
			Longitude: long,
		},
		Geohash:   geohash,
		UpdatedAt: time.Now(),
	}
}
