package domain

import "time"

type Patient struct {
	ID                  string
	MedicalRecordNumber string
	FirstName           string
	LastName            string
	DateOfBirth         time.Time
	Sex                 string
	Phone               string
	Email               string
	Address             string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type PatientFilter struct {
	Search string // matches name, MRN, phone
	Page
}
