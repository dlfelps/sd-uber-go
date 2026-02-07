// Package utils provides shared utility functions used across the application.
//
// Go Learning Note — "pkg/" Directory Convention:
// Code under pkg/ is intended to be importable by external projects (unlike
// internal/ which is compiler-enforced private). This is a community convention,
// not a Go language feature. Some Go projects avoid pkg/ entirely and put
// importable packages at the module root. Use pkg/ when you want to clearly
// signal "these packages are part of the public API."
package utils

import (
	"github.com/google/uuid"
)

// GenerateID creates a new UUID v4 string for use as an entity identifier.
//
// Go Learning Note — "github.com/google/uuid":
// This library generates RFC 4122 UUIDs. uuid.New() creates a v4 (random) UUID
// like "550e8400-e29b-41d4-a716-446655440000". UUIDs are good for distributed
// systems because they can be generated without coordination (no central counter).
// The collision probability is astronomically low (1 in 2^122).
//
// For shorter IDs, alternatives include:
//   - github.com/rs/xid — 20-char, sortable, URL-safe
//   - github.com/oklog/ulid — 26-char, sortable, compatible with UUID
//   - nanoid — configurable length, URL-safe
func GenerateID() string {
	return uuid.New().String()
}
