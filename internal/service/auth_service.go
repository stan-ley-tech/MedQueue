package service

import (
	"context"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/auth"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

type AuthService struct {
	users         repository.UserRepository
	doctors       repository.DoctorRepository
	refreshTokens repository.RefreshTokenRepository
	tokens        *auth.TokenIssuer
	audit         *AuditService
}

func NewAuthService(users repository.UserRepository, doctors repository.DoctorRepository, refreshTokens repository.RefreshTokenRepository, tokens *auth.TokenIssuer, audit *AuditService) *AuthService {
	return &AuthService{users: users, doctors: doctors, refreshTokens: refreshTokens, tokens: tokens, audit: audit}
}

type Session struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	User                  *domain.User
}

func (s *AuthService) Register(ctx context.Context, email, password, name string, role domain.Role) (*domain.User, error) {
	if !role.Valid() {
		return nil, apperr.Validation("invalid role", map[string]string{"role": "must be admin, front_desk, or clinician"})
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	u := &domain.User{Email: email, PasswordHash: hash, Name: name, Role: role}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	s.audit.Record(ctx, u.ID, role, "user.registered", "user", u.ID, map[string]any{"role": role})
	return u, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*Session, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if appErr, ok := apperr.As(err); ok && appErr.Code == apperr.CodeNotFound {
			return nil, apperr.Unauthorized("invalid email or password")
		}
		return nil, err
	}
	if !u.Active {
		return nil, apperr.Unauthorized("this account has been deactivated")
	}
	if !auth.VerifyPassword(u.PasswordHash, password) {
		return nil, apperr.Unauthorized("invalid email or password")
	}

	return s.issueSession(ctx, u)
}

func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken string) (*Session, error) {
	hash := auth.HashToken(rawRefreshToken)
	userID, err := s.refreshTokens.GetActiveByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !u.Active {
		return nil, apperr.Unauthorized("this account has been deactivated")
	}

	// Rotate: the presented refresh token is single-use.
	if err := s.refreshTokens.Revoke(ctx, hash); err != nil {
		return nil, err
	}

	return s.issueSession(ctx, u)
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	return s.refreshTokens.Revoke(ctx, auth.HashToken(rawRefreshToken))
}

func (s *AuthService) issueSession(ctx context.Context, u *domain.User) (*Session, error) {
	var doctorID string
	if u.Role == domain.RoleClinician {
		if d, err := s.doctors.GetByUserID(ctx, u.ID); err == nil {
			doctorID = d.ID
		}
	}

	access, accessExp, err := s.tokens.IssueAccessToken(u.ID, u.Role, doctorID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	refresh, refreshHash, refreshExp, err := s.tokens.IssueRefreshToken()
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if err := s.refreshTokens.Create(ctx, u.ID, refreshHash, refreshExp); err != nil {
		return nil, err
	}

	return &Session{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExp,
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshExp,
		User:                  u,
	}, nil
}
