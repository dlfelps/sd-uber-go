package geo

import (
	"context"
	"testing"
)

func TestSpatialIndex_UpdateLocation(t *testing.T) {
	index := NewSpatialIndex(6)

	loc := index.UpdateLocation("driver-1", 37.7749, -122.4194)

	if loc.DriverID != "driver-1" {
		t.Errorf("Expected driver-1, got %s", loc.DriverID)
	}
	if loc.Location.Latitude != 37.7749 {
		t.Errorf("Expected lat 37.7749, got %f", loc.Location.Latitude)
	}
	if loc.Location.Longitude != -122.4194 {
		t.Errorf("Expected lon -122.4194, got %f", loc.Location.Longitude)
	}
	if loc.Geohash == "" {
		t.Error("Expected geohash to be set")
	}
}

func TestSpatialIndex_RemoveDriver(t *testing.T) {
	index := NewSpatialIndex(6)

	index.UpdateLocation("driver-1", 37.7749, -122.4194)

	if index.Count() != 1 {
		t.Errorf("Expected count 1, got %d", index.Count())
	}

	index.RemoveDriver("driver-1")

	if index.Count() != 0 {
		t.Errorf("Expected count 0 after removal, got %d", index.Count())
	}

	loc := index.GetDriverLocation("driver-1")
	if loc != nil {
		t.Error("Expected nil location after removal")
	}
}

func TestSpatialIndex_GetDriverLocation(t *testing.T) {
	index := NewSpatialIndex(6)

	// Non-existent driver
	loc := index.GetDriverLocation("driver-nonexistent")
	if loc != nil {
		t.Error("Expected nil for non-existent driver")
	}

	// Add and retrieve
	index.UpdateLocation("driver-1", 37.7749, -122.4194)
	loc = index.GetDriverLocation("driver-1")
	if loc == nil {
		t.Error("Expected location for driver-1")
	}
	if loc.DriverID != "driver-1" {
		t.Errorf("Expected driver-1, got %s", loc.DriverID)
	}
}

func TestSpatialIndex_FindNearbyDrivers(t *testing.T) {
	index := NewSpatialIndex(6)
	ctx := context.Background()

	// Add drivers at various distances from a central point
	// Central point: 37.7749, -122.4194 (San Francisco)
	// Geohash precision 6 = ~1.2km cells, so we place drivers within neighboring cells

	// Driver 1: Very close (same location)
	index.UpdateLocation("driver-1", 37.7749, -122.4194)

	// Driver 2: About 0.5km away (within same or neighbor geohash cell)
	index.UpdateLocation("driver-2", 37.7789, -122.4194)

	// Driver 3: About 1km away (should still be in neighbor cells)
	index.UpdateLocation("driver-3", 37.7839, -122.4194)

	// Driver 4: About 50km away (should not be found with 5km radius)
	index.UpdateLocation("driver-4", 38.2749, -122.4194)

	// Find within 5km
	nearby := index.FindNearbyDrivers(ctx, 37.7749, -122.4194, 5.0)

	if len(nearby) < 2 {
		t.Errorf("Expected at least 2 nearby drivers, got %d", len(nearby))
	}

	// Verify sorted by distance
	for i := 1; i < len(nearby); i++ {
		if nearby[i].Distance < nearby[i-1].Distance {
			t.Error("Results should be sorted by distance")
		}
	}

	// Check that closest driver is first
	if len(nearby) > 0 && nearby[0].Driver.DriverID != "driver-1" {
		t.Errorf("Expected driver-1 to be closest, got %s", nearby[0].Driver.DriverID)
	}

	// Driver 4 should not be found (50km away)
	for _, d := range nearby {
		if d.Driver.DriverID == "driver-4" {
			t.Error("Driver 4 should not be found (too far away)")
		}
	}
}

func TestSpatialIndex_FindNearbyDriverIDs(t *testing.T) {
	index := NewSpatialIndex(6)
	ctx := context.Background()

	index.UpdateLocation("driver-1", 37.7749, -122.4194)
	index.UpdateLocation("driver-2", 37.7759, -122.4184)

	ids := index.FindNearbyDriverIDs(ctx, 37.7749, -122.4194, 5.0)

	if len(ids) != 2 {
		t.Errorf("Expected 2 driver IDs, got %d", len(ids))
	}
}

func TestSpatialIndex_UpdateLocationMovesDriver(t *testing.T) {
	index := NewSpatialIndex(6)

	// Add driver at location 1
	index.UpdateLocation("driver-1", 37.7749, -122.4194)
	oldGeohash := index.GetDriverLocation("driver-1").Geohash

	// Move driver to a different location (different geohash cell)
	index.UpdateLocation("driver-1", 40.7128, -74.0060)
	newGeohash := index.GetDriverLocation("driver-1").Geohash

	// Geohash should be different
	if oldGeohash == newGeohash {
		t.Error("Geohash should change when driver moves significantly")
	}

	// Count should still be 1
	if index.Count() != 1 {
		t.Errorf("Expected count 1, got %d", index.Count())
	}
}

func TestSpatialIndex_Count(t *testing.T) {
	index := NewSpatialIndex(6)

	if index.Count() != 0 {
		t.Errorf("Expected count 0, got %d", index.Count())
	}

	index.UpdateLocation("driver-1", 37.7749, -122.4194)
	index.UpdateLocation("driver-2", 37.7759, -122.4184)
	index.UpdateLocation("driver-3", 37.7769, -122.4174)

	if index.Count() != 3 {
		t.Errorf("Expected count 3, got %d", index.Count())
	}

	index.RemoveDriver("driver-2")

	if index.Count() != 2 {
		t.Errorf("Expected count 2, got %d", index.Count())
	}
}

func BenchmarkFindNearbyDrivers(b *testing.B) {
	index := NewSpatialIndex(6)
	ctx := context.Background()

	// Add 1000 drivers
	for i := 0; i < 1000; i++ {
		lat := 37.0 + float64(i%100)*0.01
		lon := -122.0 + float64(i/100)*0.01
		index.UpdateLocation("driver-"+string(rune(i)), lat, lon)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.FindNearbyDrivers(ctx, 37.5, -122.0, 5.0)
	}
}
