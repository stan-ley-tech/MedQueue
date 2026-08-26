package domain

import "time"

type Doctor struct {
	ID           string
	UserID       *string
	Name         string
	Specialty    string
	DepartmentID string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DoctorFilter struct {
	DepartmentID string
	Active       *bool
	Page
}
