package services

import (
	"log"
	"uber/internal/domain/entities"
)

type NotificationService struct {
	// In a real implementation, this would have push notification clients
}

func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

// NotifyDriverOfRideRequest sends a push notification to driver about a new ride request
func (s *NotificationService) NotifyDriverOfRideRequest(driverID string, ride *entities.Ride) {
	log.Printf("[NOTIFICATION] Driver %s: New ride request %s from (%.4f, %.4f) to (%.4f, %.4f). Estimated fare: $%.2f",
		driverID,
		ride.ID,
		ride.Source.Latitude, ride.Source.Longitude,
		ride.Destination.Latitude, ride.Destination.Longitude,
		ride.EstimatedFare,
	)
}

// NotifyRiderOfDriverAccepted sends notification to rider that driver accepted
func (s *NotificationService) NotifyRiderOfDriverAccepted(riderID, driverID, rideID string) {
	log.Printf("[NOTIFICATION] Rider %s: Driver %s has accepted your ride %s",
		riderID, driverID, rideID)
}

// NotifyRiderOfDriverArriving sends notification that driver is arriving
func (s *NotificationService) NotifyRiderOfDriverArriving(riderID, driverID, rideID string) {
	log.Printf("[NOTIFICATION] Rider %s: Driver %s is arriving for ride %s",
		riderID, driverID, rideID)
}

// NotifyRiderOfTripStarted sends notification that trip has started
func (s *NotificationService) NotifyRiderOfTripStarted(riderID, rideID string) {
	log.Printf("[NOTIFICATION] Rider %s: Your trip %s has started",
		riderID, rideID)
}

// NotifyRiderOfTripCompleted sends notification that trip is complete
func (s *NotificationService) NotifyRiderOfTripCompleted(riderID, rideID string, fare float64) {
	log.Printf("[NOTIFICATION] Rider %s: Your trip %s has been completed. Fare: $%.2f",
		riderID, rideID, fare)
}

// NotifyRiderOfNoDriversAvailable sends notification that no drivers were found
func (s *NotificationService) NotifyRiderOfNoDriversAvailable(riderID, rideID string) {
	log.Printf("[NOTIFICATION] Rider %s: No drivers available for ride %s. Please try again later.",
		riderID, rideID)
}

// NotifyDriverOfRideTimeout sends notification to driver that response timed out
func (s *NotificationService) NotifyDriverOfRideTimeout(driverID, rideID string) {
	log.Printf("[NOTIFICATION] Driver %s: Your response time for ride %s has expired",
		driverID, rideID)
}
