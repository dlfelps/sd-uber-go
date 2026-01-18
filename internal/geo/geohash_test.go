package geo

import (
	"math"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name      string
		lat       float64
		lon       float64
		precision int
		want      string
	}{
		{
			name:      "San Francisco",
			lat:       37.7749,
			lon:       -122.4194,
			precision: 6,
			want:      "9q8yyk",
		},
		{
			name:      "New York",
			lat:       40.7128,
			lon:       -74.0060,
			precision: 6,
			want:      "dr5reg",
		},
		{
			name:      "London",
			lat:       51.5074,
			lon:       -0.1278,
			precision: 6,
			want:      "gcpvj0",
		},
		{
			name:      "Default precision",
			lat:       37.7749,
			lon:       -122.4194,
			precision: 0,
			want:      "9q8yyk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Encode(tt.lat, tt.lon, tt.precision)
			if got != tt.want {
				t.Errorf("Encode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		wantLat  float64
		wantLon  float64
		tolerance float64
	}{
		{
			name:      "San Francisco",
			hash:      "9q8yyk",
			wantLat:   37.7749,
			wantLon:   -122.4194,
			tolerance: 0.01,
		},
		{
			name:      "New York",
			hash:      "dr5reg",
			wantLat:   40.7128,
			wantLon:   -74.0060,
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLat, gotLon := Decode(tt.hash)
			if math.Abs(gotLat-tt.wantLat) > tt.tolerance {
				t.Errorf("Decode() lat = %v, want %v", gotLat, tt.wantLat)
			}
			if math.Abs(gotLon-tt.wantLon) > tt.tolerance {
				t.Errorf("Decode() lon = %v, want %v", gotLon, tt.wantLon)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	testCases := []struct {
		lat float64
		lon float64
	}{
		{37.7749, -122.4194},
		{40.7128, -74.0060},
		{51.5074, -0.1278},
		{-33.8688, 151.2093},
		{35.6762, 139.6503},
	}

	for _, tc := range testCases {
		hash := Encode(tc.lat, tc.lon, 8)
		decodedLat, decodedLon := Decode(hash)

		tolerance := 0.001
		if math.Abs(decodedLat-tc.lat) > tolerance {
			t.Errorf("Round trip failed for lat: original %v, decoded %v", tc.lat, decodedLat)
		}
		if math.Abs(decodedLon-tc.lon) > tolerance {
			t.Errorf("Round trip failed for lon: original %v, decoded %v", tc.lon, decodedLon)
		}
	}
}

func TestNeighbor(t *testing.T) {
	center := "9q8yyk"

	north := Neighbor(center, "n")
	south := Neighbor(center, "s")
	east := Neighbor(center, "e")
	west := Neighbor(center, "w")

	if north == center {
		t.Error("North neighbor should be different from center")
	}
	if south == center {
		t.Error("South neighbor should be different from center")
	}
	if east == center {
		t.Error("East neighbor should be different from center")
	}
	if west == center {
		t.Error("West neighbor should be different from center")
	}

	// Verify neighbors are valid geohashes (same length)
	if len(north) != len(center) {
		t.Errorf("North neighbor length %d != center length %d", len(north), len(center))
	}
}

func TestAllNeighbors(t *testing.T) {
	center := "9q8yyk"
	neighbors := AllNeighbors(center)

	if len(neighbors) != 9 {
		t.Errorf("Expected 9 neighbors (including center), got %d", len(neighbors))
	}

	// First should be center
	if neighbors[0] != center {
		t.Errorf("First neighbor should be center, got %s", neighbors[0])
	}

	// Check for uniqueness (except center might appear in edge cases)
	seen := make(map[string]bool)
	for _, n := range neighbors {
		if seen[n] {
			// This could happen at edges, but let's log it
			t.Logf("Duplicate neighbor found: %s", n)
		}
		seen[n] = true
	}
}

func BenchmarkEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Encode(37.7749, -122.4194, 6)
	}
}

func BenchmarkDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Decode("9q8yyk")
	}
}
