package entities

import "time"

type DriverStatus string

const (
	DriverStatusAvailable DriverStatus = "available"
	DriverStatusInRide    DriverStatus = "in_ride"
	DriverStatusOffline   DriverStatus = "offline"
)

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

func (d *Driver) IsAvailable() bool {
	return d.Status == DriverStatusAvailable
}

func (d *Driver) SetStatus(status DriverStatus) {
	d.Status = status
	d.UpdatedAt = time.Now()
}

func (d *Driver) GoOnline() {
	d.SetStatus(DriverStatusAvailable)
}

func (d *Driver) GoOffline() {
	d.SetStatus(DriverStatusOffline)
}

func (d *Driver) StartRide() {
	d.SetStatus(DriverStatusInRide)
}

func (d *Driver) EndRide() {
	d.SetStatus(DriverStatusAvailable)
}
