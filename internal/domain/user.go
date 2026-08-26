package domain

import "time"

// Role is a coarse-grained permission group. RBAC middleware checks the
// role attached to the authenticated request context against the roles
// a route allows.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleFrontDesk Role = "front_desk"
	RoleClinician Role = "clinician"
)

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleFrontDesk, RoleClinician:
		return true
	}
	return false
}

// User is an account that can authenticate against the API. Clinicians
// have a linked Doctor row (DoctorID); front-desk and admin users do not.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	Role         Role
	DoctorID     *string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
