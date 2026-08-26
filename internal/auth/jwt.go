package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

// Claims is the payload embedded in access tokens. DoctorID is only set
// for clinician accounts and lets handlers resolve "my queue" without an
// extra database round trip on every request.
type Claims struct {
	UserID   string      `json:"uid"`
	Role     domain.Role `json:"role"`
	DoctorID string      `json:"doctor_id,omitempty"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	issuer        string
}

func NewTokenIssuer(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		issuer:        "medqueue",
	}
}

func (t *TokenIssuer) IssueAccessToken(userID string, role domain.Role, doctorID string) (string, time.Time, error) {
	expiresAt := time.Now().Add(t.accessTTL)
	claims := Claims{
		UserID:   userID,
		Role:     role,
		DoctorID: doctorID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.accessSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (t *TokenIssuer) ParseAccessToken(raw string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return t.accessSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("auth: invalid access token")
	}
	return claims, nil
}

// IssueRefreshToken returns an opaque, high-entropy token to hand to the
// client, plus its SHA-256 hash and expiry to persist server-side. Only
// the hash is stored so a leaked database never exposes usable tokens.
func (t *TokenIssuer) IssueRefreshToken() (raw string, hash string, expiresAt time.Time, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", time.Time{}, fmt.Errorf("auth: generate refresh token: %w", err)
	}
	raw = hex.EncodeToString(buf)
	hash = HashToken(raw)
	expiresAt = time.Now().Add(t.refreshTTL)
	return raw, hash, expiresAt, nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
