package auth

import (
	"testing"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/domain"
)

func TestTokenIssuer_AccessTokenRoundTrip(t *testing.T) {
	issuer := NewTokenIssuer("access-secret", "refresh-secret", 15*time.Minute, 168*time.Hour)

	token, expiresAt, err := issuer.IssueAccessToken("user-1", domain.RoleClinician, "doctor-1")
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	if expiresAt.Before(time.Now()) {
		t.Fatal("expected expiry to be in the future")
	}

	claims, err := issuer.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-1")
	}
	if claims.Role != domain.RoleClinician {
		t.Errorf("Role = %q, want %q", claims.Role, domain.RoleClinician)
	}
	if claims.DoctorID != "doctor-1" {
		t.Errorf("DoctorID = %q, want %q", claims.DoctorID, "doctor-1")
	}
}

func TestTokenIssuer_RejectsWrongSecret(t *testing.T) {
	issuer := NewTokenIssuer("access-secret", "refresh-secret", 15*time.Minute, 168*time.Hour)
	other := NewTokenIssuer("different-secret", "refresh-secret", 15*time.Minute, 168*time.Hour)

	token, _, err := issuer.IssueAccessToken("user-1", domain.RoleAdmin, "")
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	if _, err := other.ParseAccessToken(token); err == nil {
		t.Fatal("expected parsing a token signed with a different secret to fail")
	}
}

func TestTokenIssuer_RejectsExpiredToken(t *testing.T) {
	issuer := NewTokenIssuer("access-secret", "refresh-secret", -1*time.Minute, 168*time.Hour)

	token, _, err := issuer.IssueAccessToken("user-1", domain.RoleAdmin, "")
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	if _, err := issuer.ParseAccessToken(token); err == nil {
		t.Fatal("expected parsing an already-expired token to fail")
	}
}

func TestTokenIssuer_RefreshTokenIsHashedConsistently(t *testing.T) {
	issuer := NewTokenIssuer("access-secret", "refresh-secret", 15*time.Minute, 168*time.Hour)

	raw, hash, expiresAt, err := issuer.IssueRefreshToken()
	if err != nil {
		t.Fatalf("IssueRefreshToken() error = %v", err)
	}
	if raw == hash {
		t.Fatal("expected the raw token and its hash to differ")
	}
	if HashToken(raw) != hash {
		t.Fatal("expected HashToken(raw) to reproduce the stored hash")
	}
	if expiresAt.Before(time.Now()) {
		t.Fatal("expected refresh token expiry to be in the future")
	}
}
