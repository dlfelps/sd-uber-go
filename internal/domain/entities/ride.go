package entities

import (
	"errors"
	"time"
)

// RideStatus represents the current lifecycle state of a ride.
//
// Go Learning Note — State Machines in Go:
// This file implements a finite state machine (FSM) using a map of valid
// transitions. This is a clean pattern for modeling entities with well-defined
// lifecycles (orders, payments, rides, etc.). The ride's lifecycle is:
//
//	Estimate → Requested → Matching → Accepted → PickingUp → InProgress → Completed
//	                           ↘ Failed
//	     (any non-terminal state can also transition to Cancelled)
type RideStatus string

const (
	RideStatusEstimate   RideStatus = "estimate"
	RideStatusRequested  RideStatus = "requested"
	RideStatusMatching   RideStatus = "matching"
	RideStatusAccepted   RideStatus = "accepted"
	RideStatusPickingUp  RideStatus = "picking_up"
	RideStatusInProgress RideStatus = "in_progress"
	RideStatusCompleted  RideStatus = "completed"
	RideStatusCancelled  RideStatus = "cancelled"
	RideStatusFailed     RideStatus = "failed"
)

// validTransitions defines which status changes are allowed from each state.
// Terminal states (Completed, Cancelled, Failed) have empty slices — no
// transitions out. This map IS the state machine — CanTransitionTo() simply
// looks up the current status and checks if the target is in the slice.
//
// Go Learning Note — Package-Level Variables:
// The lowercase "var" makes validTransitions unexported (private to this
// package). It's initialized at package load time. In Go, package-level vars
// are initialized before main() runs, in dependency order.
var validTransitions = map[RideStatus][]RideStatus{
	RideStatusEstimate:   {RideStatusRequested, RideStatusCancelled},
	RideStatusRequested:  {RideStatusMatching, RideStatusCancelled},
	RideStatusMatching:   {RideStatusAccepted, RideStatusFailed, RideStatusCancelled},
	RideStatusAccepted:   {RideStatusPickingUp, RideStatusCancelled},
	RideStatusPickingUp:  {RideStatusInProgress, RideStatusCancelled},
	RideStatusInProgress: {RideStatusCompleted, RideStatusCancelled},
	RideStatusCompleted:  {},
	RideStatusCancelled:  {},
	RideStatusFailed:     {},
}

// Ride is the central domain entity. It tracks a ride from fare estimate through
// completion, including the assigned driver, timestamps for each phase, and fares.
//
// Go Learning Note — "omitempty" Struct Tag:
// Fields tagged with `json:"...,omitempty"` are excluded from JSON output when
// they hold their zero value. For strings that's "", for numbers 0, for
// time.Time it's the zero time. This keeps API responses clean — DriverID won't
// appear in the JSON until a driver is assigned, and ActualFare won't appear
// until the ride is completed.
type Ride struct {
	ID            string     `json:"id"`
	RiderID       string     `json:"rider_id"`
	DriverID      string     `json:"driver_id,omitempty"`
	Status        RideStatus `json:"status"`
	Source        Location   `json:"source"`
	Destination   Location   `json:"destination"`
	EstimatedFare float64    `json:"estimated_fare"`
	ActualFare    float64    `json:"actual_fare,omitempty"`
	DistanceKm    float64    `json:"distance_km"`
	DurationMins  float64    `json:"duration_mins"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	AcceptedAt    time.Time  `json:"accepted_at,omitempty"`
	PickedUpAt    time.Time  `json:"picked_up_at,omitempty"`
	CompletedAt   time.Time  `json:"completed_at,omitempty"`
}

// NewRide creates a Ride starting in the Estimate state. No driver is assigned
// yet — that happens later when a driver accepts during the matching phase.
func NewRide(id, riderID string, source, destination Location, estimatedFare, distanceKm, durationMins float64) *Ride {
	now := time.Now()
	return &Ride{
		ID:            id,
		RiderID:       riderID,
		Status:        RideStatusEstimate,
		Source:        source,
		Destination:   destination,
		EstimatedFare: estimatedFare,
		DistanceKm:    distanceKm,
		DurationMins:  durationMins,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CanTransitionTo checks if moving to newStatus is a valid state change.
//
// Go Learning Note — Comma-ok Idiom:
// The pattern `value, exists := someMap[key]` is the "comma-ok" idiom. The
// second return value (exists) is a boolean indicating whether the key was
// found. Without the second variable, accessing a missing key returns the
// zero value silently, which can cause subtle bugs.
func (r *Ride) CanTransitionTo(newStatus RideStatus) bool {
	allowedStatuses, exists := validTransitions[r.Status]
	if !exists {
		return false
	}
	for _, s := range allowedStatuses {
		if s == newStatus {
			return true
		}
	}
	return false
}

// TransitionTo attempts to move the ride to newStatus. Returns an error if the
// transition is not allowed by the state machine. On success, it also records
// phase-specific timestamps (AcceptedAt, PickedUpAt, CompletedAt).
//
// Go Learning Note — Error Handling:
// Go functions signal failure by returning an error as the last return value.
// There are no exceptions or try/catch. Callers must check `if err != nil`
// after every call that can fail. This makes error paths explicit and visible
// in the code. The errors.New() function creates a simple error with a message.
// For richer errors, you can define custom error types or use fmt.Errorf with
// the %w verb for error wrapping (Go 1.13+).
func (r *Ride) TransitionTo(newStatus RideStatus) error {
	if !r.CanTransitionTo(newStatus) {
		return errors.New("invalid status transition from " + string(r.Status) + " to " + string(newStatus))
	}
	r.Status = newStatus
	r.UpdatedAt = time.Now()

	// Record timestamps for specific lifecycle milestones.
	switch newStatus {
	case RideStatusAccepted:
		r.AcceptedAt = time.Now()
	case RideStatusPickingUp:
		r.PickedUpAt = time.Now()
	case RideStatusCompleted:
		r.CompletedAt = time.Now()
		r.ActualFare = r.EstimatedFare
	}

	return nil
}

// AssignDriver records which driver is handling this ride.
func (r *Ride) AssignDriver(driverID string) {
	r.DriverID = driverID
	r.UpdatedAt = time.Now()
}

// The following methods are convenience wrappers around TransitionTo. They
// make calling code more readable: ride.Request() is clearer than
// ride.TransitionTo(RideStatusRequested). This is a common Go pattern for
// domain entities — expose intent-revealing method names that delegate to a
// general-purpose internal method.

// Request transitions the ride from Estimate to Requested (rider confirms).
func (r *Ride) Request() error {
	return r.TransitionTo(RideStatusRequested)
}

// StartMatching transitions to the Matching state (system is finding a driver).
func (r *Ride) StartMatching() error {
	return r.TransitionTo(RideStatusMatching)
}

// Accept assigns a driver and transitions to Accepted.
func (r *Ride) Accept(driverID string) error {
	r.AssignDriver(driverID)
	return r.TransitionTo(RideStatusAccepted)
}

// StartPickup transitions to PickingUp (driver is en route to rider).
func (r *Ride) StartPickup() error {
	return r.TransitionTo(RideStatusPickingUp)
}

// StartTrip transitions to InProgress (rider is in the car).
func (r *Ride) StartTrip() error {
	return r.TransitionTo(RideStatusInProgress)
}

// Complete transitions to Completed (ride finished successfully).
func (r *Ride) Complete() error {
	return r.TransitionTo(RideStatusCompleted)
}

// Cancel transitions to Cancelled (rider or driver cancelled).
func (r *Ride) Cancel() error {
	return r.TransitionTo(RideStatusCancelled)
}

// Fail transitions to Failed (no driver found during matching).
func (r *Ride) Fail() error {
	return r.TransitionTo(RideStatusFailed)
}
