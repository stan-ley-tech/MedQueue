package domain

// Page carries pagination input shared by every list filter. Limit and
// Offset are normalized by NormalizeDefaults before hitting the database.
type Page struct {
	Limit  int
	Offset int
}

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

func (p *Page) NormalizeDefaults() {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
}

// PagedResult wraps a page of items with the metadata clients need to
// fetch subsequent pages.
type PagedResult[T any] struct {
	Items      []T `json:"items"`
	Total      int `json:"total"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
}
