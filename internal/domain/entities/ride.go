package entities

import (
	"errors"
	"time"
)

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

func (r *Ride) TransitionTo(newStatus RideStatus) error {
	if !r.CanTransitionTo(newStatus) {
		return errors.New("invalid status transition from " + string(r.Status) + " to " + string(newStatus))
	}
	r.Status = newStatus
	r.UpdatedAt = time.Now()

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

func (r *Ride) AssignDriver(driverID string) {
	r.DriverID = driverID
	r.UpdatedAt = time.Now()
}

func (r *Ride) Request() error {
	return r.TransitionTo(RideStatusRequested)
}

func (r *Ride) StartMatching() error {
	return r.TransitionTo(RideStatusMatching)
}

func (r *Ride) Accept(driverID string) error {
	r.AssignDriver(driverID)
	return r.TransitionTo(RideStatusAccepted)
}

func (r *Ride) StartPickup() error {
	return r.TransitionTo(RideStatusPickingUp)
}

func (r *Ride) StartTrip() error {
	return r.TransitionTo(RideStatusInProgress)
}

func (r *Ride) Complete() error {
	return r.TransitionTo(RideStatusCompleted)
}

func (r *Ride) Cancel() error {
	return r.TransitionTo(RideStatusCancelled)
}

func (r *Ride) Fail() error {
	return r.TransitionTo(RideStatusFailed)
}
