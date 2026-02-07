// Package entities defines the core domain models for the ride-sharing system.
// These structs represent the business concepts (Driver, Rider, Ride, Location)
// and live in the innermost layer of the architecture — they have no dependencies
// on databases, HTTP, or external services.
//
// Go Learning Note — "internal/" directory:
// Packages under internal/ cannot be imported by code outside this module. Go
// enforces this at the compiler level. This is how Go provides encapsulation
// at the package level — it prevents external code from depending on your
// internal implementation details.
package entities

import "time"

// DriverStatus is a typed string enum representing the driver's current state.
//
// Go Learning Note — Type Aliases for Enums:
// Go doesn't have a native enum keyword. The idiomatic pattern is to define a
// named type (usually based on string or int) and then declare constants of that
// type using const + iota (for ints) or explicit string values (for strings).
// String-based enums are preferred when the value will be serialized to JSON or
// stored in a database, because they're human-readable.
type DriverStatus string

const (
	DriverStatusAvailable DriverStatus = "available"
	DriverStatusInRide    DriverStatus = "in_ride"
	DriverStatusOffline   DriverStatus = "offline"
)

// Driver represents a driver in the ride-sharing system.
//
// Go Learning Note — Struct Tags:
// The `json:"id"` annotations are called struct tags. They control how
// encoding/json (and other encoding packages) serialize/deserialize the field.
// Tags are metadata attached to struct fields — they're accessed via the
// "reflect" package at runtime. Common tags include `json`, `xml`, `db`,
// `yaml`, and `binding` (used by Gin for request validation).
type Driver struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Email     string       `json:"email"`
	Phone     string       `json:"phone"`
	Status    DriverStatus `json:"status"`
	VehicleID string       `json:"vehicle_id"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// NewDriver creates a Driver with initial status set to Offline.
// Drivers must explicitly go online before they can receive ride requests.
//
// Go Learning Note — Pointer vs Value Receivers:
// NewDriver returns *Driver (a pointer). This is standard for constructors of
// mutable objects — the caller and all downstream code share the same Driver
// instance. If you returned a Driver value, each assignment would create a copy,
// and mutations wouldn't be visible to other holders.
func NewDriver(id, name, email, phone, vehicleID string) *Driver {
	now := time.Now()
	return &Driver{
		ID:        id,
		Name:      name,
		Email:     email,
		Phone:     phone,
		Status:    DriverStatusOffline,
		VehicleID: vehicleID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsAvailable checks whether the driver can accept new ride requests.
func (d *Driver) IsAvailable() bool {
	return d.Status == DriverStatusAvailable
}

// SetStatus updates the driver's status and records the change timestamp.
//
// Go Learning Note — Methods with Pointer Receivers:
// The (d *Driver) receiver means this method can mutate the Driver. If the
// receiver were (d Driver) (value receiver), Go would pass a copy and changes
// would be lost. Rule of thumb: use pointer receivers when the method modifies
// the receiver, or when the struct is large and you want to avoid copying.
func (d *Driver) SetStatus(status DriverStatus) {
	d.Status = status
	d.UpdatedAt = time.Now()
}

// GoOnline marks the driver as available to receive ride requests.
func (d *Driver) GoOnline() {
	d.SetStatus(DriverStatusAvailable)
}

// GoOffline marks the driver as unavailable.
func (d *Driver) GoOffline() {
	d.SetStatus(DriverStatusOffline)
}

// StartRide marks the driver as currently on a ride and unavailable for new ones.
func (d *Driver) StartRide() {
	d.SetStatus(DriverStatusInRide)
}

// EndRide marks the driver as available again after completing or cancelling a ride.
func (d *Driver) EndRide() {
	d.SetStatus(DriverStatusAvailable)
}
