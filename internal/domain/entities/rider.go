package entities

import "time"

// Rider represents a passenger who requests rides.
// This is a simple value object with no status tracking — rider state is
// managed through their active Ride entities instead.
//
// Go Learning Note — Exported vs Unexported:
// In Go, identifiers starting with an uppercase letter (Rider, ID, Name) are
// exported (public) and accessible from other packages. Lowercase identifiers
// are unexported (package-private). There are no keywords like public/private —
// capitalization IS the access modifier. This applies to types, functions,
// methods, struct fields, and variables.
type Rider struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
}

// NewRider constructs a Rider with the creation timestamp set to now.
func NewRider(id, name, email, phone string) *Rider {
	return &Rider{
		ID:        id,
		Name:      name,
		Email:     email,
		Phone:     phone,
		CreatedAt: time.Now(),
	}
}
