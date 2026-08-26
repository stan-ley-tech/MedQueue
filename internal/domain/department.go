package domain

import "time"

type Department struct {
	ID          string
	Name        string
	Description string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DepartmentFilter struct {
	Search string
	Page
}
