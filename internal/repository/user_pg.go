package repository

import (
	"context"
	"fmt"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type UserPG struct{ pgStore }

func NewUserRepository(pool *db.Pool) *UserPG {
	return &UserPG{pgStore{pool}}
}

func (r *UserPG) Create(ctx context.Context, u *domain.User) error {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, role, active)
		VALUES ($1, $2, $3, $4, TRUE)
		RETURNING id, created_at, updated_at`,
		u.Email, u.PasswordHash, u.Name, u.Role,
	)
	if err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("a user with this email already exists")
		}
		return fmt.Errorf("user_pg: create: %w", err)
	}
	u.Active = true
	return nil
}

func (r *UserPG) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT id, email, password_hash, name, role, active, created_at, updated_at
		FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *UserPG) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT id, email, password_hash, name, role, active, created_at, updated_at
		FROM users WHERE email = $1`, email)
	return scanUser(row)
}

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user_pg: scan: %w", err)
	}
	return &u, nil
}
