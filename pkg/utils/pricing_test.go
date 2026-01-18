package utils

import (
	"math"
	"testing"
)

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lon1     float64
		lat2     float64
		lon2     float64
		expected float64
		tolerance float64
	}{
		{
			name:     "Same location",
			lat1:     37.7749,
			lon1:     -122.4194,
			lat2:     37.7749,
			lon2:     -122.4194,
			expected: 0,
			tolerance: 0.001,
		},
		{
			name:     "SF to Oakland",
			lat1:     37.7749,
			lon1:     -122.4194,
			lat2:     37.8044,
			lon2:     -122.2712,
			expected: 13.0, // approximately 13 km
			tolerance: 1.0,
		},
		{
			name:     "NYC to LA",
			lat1:     40.7128,
			lon1:     -74.0060,
			lat2:     34.0522,
			lon2:     -118.2437,
			expected: 3940, // approximately 3940 km
			tolerance: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HaversineDistance(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(result-tt.expected) > tt.tolerance {
				t.Errorf("HaversineDistance() = %v, expected %v (+/- %v)", result, tt.expected, tt.tolerance)
			}
		})
	}
}

func TestEstimateDuration(t *testing.T) {
	tests := []struct {
		name       string
		distanceKm float64
		minMinutes float64
		maxMinutes float64
	}{
		{
			name:       "Short trip 1km",
			distanceKm: 1.0,
			minMinutes: 1.5,
			maxMinutes: 3.0,
		},
		{
			name:       "Medium trip 5km",
			distanceKm: 5.0,
			minMinutes: 8.0,
			maxMinutes: 12.0,
		},
		{
			name:       "Long trip 20km",
			distanceKm: 20.0,
			minMinutes: 35.0,
			maxMinutes: 45.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateDuration(tt.distanceKm)
			if result < tt.minMinutes || result > tt.maxMinutes {
				t.Errorf("EstimateDuration(%v) = %v, expected between %v and %v",
					tt.distanceKm, result, tt.minMinutes, tt.maxMinutes)
			}
		})
	}
}

func TestPricingCalculator_CalculateFare(t *testing.T) {
	calc := NewPricingCalculator(2.50, 1.50, 0.25, 5.00)

	tests := []struct {
		name          string
		distanceKm    float64
		durationMins  float64
		surgeMultiple float64
		minFare       float64
		maxFare       float64
	}{
		{
			name:          "Short trip no surge",
			distanceKm:    1.0,
			durationMins:  5.0,
			surgeMultiple: 1.0,
			minFare:       5.00, // minimum fare
			maxFare:       6.00,
		},
		{
			name:          "Medium trip no surge",
			distanceKm:    5.0,
			durationMins:  15.0,
			surgeMultiple: 1.0,
			minFare:       12.0,
			maxFare:       15.0,
		},
		{
			name:          "Medium trip with surge",
			distanceKm:    5.0,
			durationMins:  15.0,
			surgeMultiple: 2.0,
			minFare:       24.0,
			maxFare:       30.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.CalculateFare(tt.distanceKm, tt.durationMins, tt.surgeMultiple)
			if result.TotalFare < tt.minFare || result.TotalFare > tt.maxFare {
				t.Errorf("CalculateFare() = %v, expected between %v and %v",
					result.TotalFare, tt.minFare, tt.maxFare)
			}
		})
	}
}

func TestPricingCalculator_MinimumFare(t *testing.T) {
	calc := NewPricingCalculator(2.50, 1.50, 0.25, 5.00)

	// Very short trip that would normally be less than minimum
	result := calc.CalculateFare(0.1, 1.0, 1.0)

	if result.TotalFare < 5.00 {
		t.Errorf("Expected minimum fare of 5.00, got %v", result.TotalFare)
	}
}

func TestFareEstimate_Fields(t *testing.T) {
	calc := NewPricingCalculator(2.50, 1.50, 0.25, 5.00)
	result := calc.CalculateFare(5.0, 15.0, 1.5)

	if result.DistanceKm != 5.0 {
		t.Errorf("Expected DistanceKm 5.0, got %v", result.DistanceKm)
	}
	if result.DurationMins != 15.0 {
		t.Errorf("Expected DurationMins 15.0, got %v", result.DurationMins)
	}
	if result.BaseFare != 2.50 {
		t.Errorf("Expected BaseFare 2.50, got %v", result.BaseFare)
	}
	if result.SurgeMultiple != 1.5 {
		t.Errorf("Expected SurgeMultiple 1.5, got %v", result.SurgeMultiple)
	}
}

func BenchmarkHaversineDistance(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HaversineDistance(37.7749, -122.4194, 37.8044, -122.2712)
	}
}

func BenchmarkCalculateFare(b *testing.B) {
	calc := NewPricingCalculator(2.50, 1.50, 0.25, 5.00)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateFare(5.0, 15.0, 1.5)
	}
}
