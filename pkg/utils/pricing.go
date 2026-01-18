package utils

import (
	"math"
)

const (
	EarthRadiusKm = 6371.0
)

type FareEstimate struct {
	DistanceKm    float64 `json:"distance_km"`
	DurationMins  float64 `json:"duration_mins"`
	BaseFare      float64 `json:"base_fare"`
	DistanceFare  float64 `json:"distance_fare"`
	TimeFare      float64 `json:"time_fare"`
	TotalFare     float64 `json:"total_fare"`
	SurgeMultiple float64 `json:"surge_multiple"`
}

type PricingCalculator struct {
	BaseFare      float64
	PerKmRate     float64
	PerMinuteRate float64
	MinimumFare   float64
}

func NewPricingCalculator(baseFare, perKmRate, perMinuteRate, minimumFare float64) *PricingCalculator {
	return &PricingCalculator{
		BaseFare:      baseFare,
		PerKmRate:     perKmRate,
		PerMinuteRate: perMinuteRate,
		MinimumFare:   minimumFare,
	}
}

func (p *PricingCalculator) CalculateFare(distanceKm, durationMins, surgeMultiple float64) FareEstimate {
	distanceFare := distanceKm * p.PerKmRate
	timeFare := durationMins * p.PerMinuteRate

	subtotal := p.BaseFare + distanceFare + timeFare
	total := subtotal * surgeMultiple

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

// HaversineDistance calculates the distance between two points in kilometers
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKm * c
}

// EstimateDuration estimates travel time based on distance
// Assumes average speed of 30 km/h in urban areas
func EstimateDuration(distanceKm float64) float64 {
	averageSpeedKmH := 30.0
	return (distanceKm / averageSpeedKmH) * 60 // Convert to minutes
}
