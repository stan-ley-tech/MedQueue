package service

import "github.com/stan-ley-tech/medqueue/internal/domain"

// Actor identifies who is performing a service call. Handlers build one
// from the authenticated request context and pass it into every
// state-changing service method so audit entries always know who to
// attribute the change to.
type Actor struct {
	UserID   string
	Role     domain.Role
	DoctorID string
}
