package config

import (
	"time"
)

type Config struct {
	Server   ServerConfig
	Matching MatchingConfig
	Geo      GeoConfig
	Pricing  PricingConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type MatchingConfig struct {
	DriverResponseTimeout time.Duration
	TotalMatchingTimeout  time.Duration
	SearchRadiusKm        float64
}

type GeoConfig struct {
	GeohashPrecision int
}

type PricingConfig struct {
	BaseFare       float64
	PerKmRate      float64
	PerMinuteRate  float64
	MinimumFare    float64
	SurgePriceMax  float64
}

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
