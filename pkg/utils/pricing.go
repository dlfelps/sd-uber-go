package utils

import (
	"math"
)

// EarthRadiusKm is the mean radius of the Earth in kilometers, used by the
// Haversine formula to convert angular distance to linear distance.
const (
	EarthRadiusKm = 6371.0
)

// FareEstimate is a detailed fare breakdown returned to the rider. It shows
// each component of the fare separately so the UI can display a transparent
// breakdown.
type FareEstimate struct {
	DistanceKm    float64 `json:"distance_km"`
	DurationMins  float64 `json:"duration_mins"`
	BaseFare      float64 `json:"base_fare"`
	DistanceFare  float64 `json:"distance_fare"`
	TimeFare      float64 `json:"time_fare"`
	TotalFare     float64 `json:"total_fare"`
	SurgeMultiple float64 `json:"surge_multiple"`
}

// PricingCalculator computes ride fares using a standard formula:
// Total = (BaseFare + Distance*PerKmRate + Duration*PerMinuteRate) * SurgeMultiplier
// If the result is below MinimumFare, MinimumFare is charged instead.
type PricingCalculator struct {
	BaseFare      float64
	PerKmRate     float64
	PerMinuteRate float64
	MinimumFare   float64
}

// NewPricingCalculator creates a calculator with the given rate parameters.
func NewPricingCalculator(baseFare, perKmRate, perMinuteRate, minimumFare float64) *PricingCalculator {
	return &PricingCalculator{
		BaseFare:      baseFare,
		PerKmRate:     perKmRate,
		PerMinuteRate: perMinuteRate,
		MinimumFare:   minimumFare,
	}
}

// CalculateFare computes a fare estimate with a detailed breakdown. The
// surgeMultiple parameter allows dynamic pricing during high-demand periods
// (1.0 = no surge, 2.0 = double price).
//
// Go Learning Note — Rounding with math.Round:
// math.Round(x*100)/100 is the standard trick to round to 2 decimal places
// in Go. Go doesn't have a built-in "round to N decimals" function. Multiply
// by 10^N, round to nearest integer, then divide by 10^N. For financial
// calculations in production, use a decimal library like "shopspring/decimal"
// to avoid floating-point precision issues.
func (p *PricingCalculator) CalculateFare(distanceKm, durationMins, surgeMultiple float64) FareEstimate {
	distanceFare := distanceKm * p.PerKmRate
	timeFare := durationMins * p.PerMinuteRate

	subtotal := p.BaseFare + distanceFare + timeFare
	total := subtotal * surgeMultiple

	// Enforce minimum fare — short rides still cost at least MinimumFare.
	if total < p.MinimumFare {
		total = p.MinimumFare
	}

	return FareEstimate{
		DistanceKm:    math.Round(distanceKm*100) / 100,
		DurationMins:  math.Round(durationMins*100) / 100,
		BaseFare:      p.BaseFare,
		DistanceFare:  math.Round(distanceFare*100) / 100,
		TimeFare:      math.Round(timeFare*100) / 100,
		TotalFare:     math.Round(total*100) / 100,
		SurgeMultiple: surgeMultiple,
	}
}

// HaversineDistance calculates the great-circle distance between two points on
// Earth given their latitude and longitude in degrees. Returns distance in km.
//
// The Haversine formula accounts for Earth's curvature, making it more accurate
// than simple Euclidean distance for geographic coordinates. For short distances
// (<10 km, typical for ride-sharing), the difference is small, but Haversine is
// still preferred because it's correct at all scales.
//
// Go Learning Note — math Package:
// Go's standard library "math" provides all common math functions (Sin, Cos,
// Sqrt, Atan2, Pi, etc.). Unlike some languages, Go doesn't have a separate
// math package for float32 — all math functions work with float64. If you need
// float32 math, you must cast explicitly: float32(math.Sin(float64(x))).
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Convert degrees to radians (Go's math functions use radians).
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	// Haversine formula: a = sin²(Δlat/2) + cos(lat1) * cos(lat2) * sin²(Δlon/2)
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	// c is the angular distance in radians.
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKm * c
}

// EstimateDuration provides a rough travel time estimate based on distance,
// assuming an average urban speed of 30 km/h. Returns duration in minutes.
// In production, you'd use a routing API (Google Maps, OSRM) for accurate ETAs
// that account for traffic, road types, and turn-by-turn routing.
func EstimateDuration(distanceKm float64) float64 {
	averageSpeedKmH := 30.0
	return (distanceKm / averageSpeedKmH) * 60 // Convert hours to minutes.
}
