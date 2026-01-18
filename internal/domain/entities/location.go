package entities

import "time"

type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"long"`
}

type DriverLocation struct {
	DriverID  string    `json:"driver_id"`
	Location  Location  `json:"location"`
	Geohash   string    `json:"geohash"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewLocation(lat, long float64) Location {
	return Location{
		Latitude:  lat,
		Longitude: long,
	}
}

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
