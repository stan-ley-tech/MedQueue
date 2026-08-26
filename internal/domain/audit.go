package domain

import "time"

// AuditLog is an immutable record of a state-changing action, written by
// the audit service on the same transaction as the change it describes
// wherever practical, and always before the HTTP response is sent.
type AuditLog struct {
	ID         string
	ActorID    string
	ActorRole  Role
	Action     string
	EntityType string
	EntityID   string
	Metadata   map[string]any
	CreatedAt  time.Time
}

type AuditFilter struct {
	EntityType string
	EntityID   string
	ActorID    string
	Page
}
