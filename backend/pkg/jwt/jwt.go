package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token types. A token minted for refreshing must never be accepted as an
// access token, so the type is carried inside the signed payload.
const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

// Subject kinds. Staff tokens come from the admin login flows; member tokens
// come from the client-facing login flows and must not reach admin endpoints.
const (
	KindStaff  = "staff"
	KindMember = "member"
)

type Claims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	RoleID      string   `json:"role_id"`
	Permissions []string `json:"permissions"`
	TokenType   string   `json:"typ"`
	Kind        string   `json:"knd"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

func New(secret string, accessTTL, refreshTTL time.Duration, issuer string) *Manager {
	return &Manager{
		secret:     secret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     issuer,
	}
}

func (m *Manager) GenerateAccessToken(userID, username, role, roleID, kind string, permissions []string) (string, error) {
	claims := &Claims{
		UserID:      userID,
		Username:    username,
		Role:        role,
		RoleID:      roleID,
		Permissions: permissions,
		TokenType:   TypeAccess,
		Kind:        kind,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    m.issuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secret))
}

func (m *Manager) GenerateRefreshToken(userID, kind string) (string, error) {
	claims := &Claims{
		UserID:    userID,
		TokenType: TypeRefresh,
		Kind:      kind,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    m.issuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secret))
}

func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
