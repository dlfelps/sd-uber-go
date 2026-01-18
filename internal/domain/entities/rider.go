package entities

import "time"

type Rider struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
}

func NewRider(id, name, email, phone string) *Rider {
	return &Rider{
		ID:        id,
		Name:      name,
		Email:     email,
		Phone:     phone,
		CreatedAt: time.Now(),
	}
}
