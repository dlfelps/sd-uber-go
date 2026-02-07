// Package config centralizes all application configuration into typed structs.
//
// Go Learning Note — Configuration Management:
// Go projects typically manage configuration in one of these ways:
//   1. Struct literals with defaults (used here — simplest for MVPs)
//   2. Environment variables via os.Getenv() or "github.com/kelseyhightower/envconfig"
//   3. Config files (YAML/TOML) via "github.com/spf13/viper"
//   4. Command-line flags via the standard "flag" package
//
// Using typed structs (not raw strings/maps) gives you compile-time safety
// and IDE autocompletion. This is strongly preferred in Go over untyped config.
package config

import (
	"time"
)

// Config is the top-level configuration container. Grouping related settings
// into sub-structs keeps the config organized as the application grows.
//
// Go Learning Note — Struct Composition:
// Go doesn't have classes or inheritance. Instead, you compose structs by
// embedding or nesting them. Here Config "has a" ServerConfig, MatchingConfig,
// etc. This is composition over inheritance — a core Go design principle.
type Config struct {
	Server   ServerConfig
	Matching MatchingConfig
	Geo      GeoConfig
	Pricing  PricingConfig
}

// ServerConfig holds HTTP server settings.
//
// Go Learning Note — time.Duration:
// Go uses time.Duration (an int64 of nanoseconds) instead of raw integers for
// timeouts and intervals. This prevents unit confusion — you write
// "10 * time.Second" which is self-documenting, rather than guessing whether
// "10" means seconds, milliseconds, or something else.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// MatchingConfig controls the async ride-driver matching engine.
type MatchingConfig struct {
	DriverResponseTimeout time.Duration // How long to wait for one driver to respond
	TotalMatchingTimeout  time.Duration // Max total time to find any driver
	SearchRadiusKm        float64       // Geospatial search radius in kilometers
}

// GeoConfig controls geohash encoding precision. Precision 6 ≈ 1.2 km cells,
// precision 7 ≈ 150 m cells. Higher precision means smaller cells and more
// accurate proximity queries, but requires scanning more neighboring cells.
type GeoConfig struct {
	GeohashPrecision int
}

// PricingConfig defines the fare calculation parameters.
// Fare = (BaseFare + DistanceKm*PerKmRate + DurationMins*PerMinuteRate) * SurgeMultiplier
// Result is clamped to at least MinimumFare.
type PricingConfig struct {
	BaseFare      float64
	PerKmRate     float64
	PerMinuteRate float64
	MinimumFare   float64
	SurgePriceMax float64
}

// NewDefaultConfig returns a Config populated with sensible defaults.
//
// Go Learning Note — Constructor Functions:
// Go has no constructors. By convention, New<Type>() functions serve the same
// purpose. They return a pointer (*Config) so the caller gets a reference to
// shared, mutable state. Returning a value type would copy the struct on every
// assignment, which is fine for small immutable data but wasteful for large
// config objects that get passed around.
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         ":8080",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Matching: MatchingConfig{
			DriverResponseTimeout: 10 * time.Second,
			TotalMatchingTimeout:  60 * time.Second,
			SearchRadiusKm:        5.0,
		},
		Geo: GeoConfig{
			GeohashPrecision: 6,
		},
		Pricing: PricingConfig{
			BaseFare:      2.50,
			PerKmRate:     1.50,
			PerMinuteRate: 0.25,
			MinimumFare:   5.00,
			SurgePriceMax: 3.0,
		},
	}
}
