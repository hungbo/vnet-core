package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *Manager {
	return New("test-secret", 1*time.Hour, 168*time.Hour, "vnet-test")
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	m := newTestManager(t)

	token, err := m.GenerateAccessToken("user-1", "admin", "R_SUPER", "role-1", []string{"*", "read", "write"})
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := m.ValidateToken(token)
	require.NoError(t, err)

	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "admin", claims.Username)
	assert.Equal(t, "R_SUPER", claims.Role)
	assert.Equal(t, "role-1", claims.RoleID)
	assert.Equal(t, []string{"*", "read", "write"}, claims.Permissions)
	assert.Equal(t, "vnet-test", claims.Issuer)
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.IssuedAt)
}

func TestGenerateAndValidateRefreshToken(t *testing.T) {
	m := newTestManager(t)

	token, err := m.GenerateRefreshToken("user-1")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := m.ValidateToken(token)
	require.NoError(t, err)

	assert.Equal(t, "user-1", claims.UserID)
	assert.Empty(t, claims.Username)
	assert.Empty(t, claims.Role)
	assert.Empty(t, claims.RoleID)
	assert.Empty(t, claims.Permissions)
}

func TestValidateToken_Expired(t *testing.T) {
	m := New("test-secret", -1*time.Hour, -1*time.Hour, "vnet-test")

	token, err := m.GenerateAccessToken("user-1", "admin", "R_SUPER", "", nil)
	require.NoError(t, err)

	_, err = m.ValidateToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	m1 := New("secret-1", 1*time.Hour, 168*time.Hour, "vnet")
	m2 := New("secret-2", 1*time.Hour, 168*time.Hour, "vnet")

	token, err := m1.GenerateAccessToken("user-1", "admin", "R_SUPER", "", nil)
	require.NoError(t, err)

	_, err = m2.ValidateToken(token)
	assert.Error(t, err)
}

func TestValidateToken_Malformed(t *testing.T) {
	m := newTestManager(t)

	_, err := m.ValidateToken("not-a-valid-token")
	assert.Error(t, err)
}

func TestValidateToken_Empty(t *testing.T) {
	m := newTestManager(t)

	_, err := m.ValidateToken("")
	assert.Error(t, err)
}

func TestDifferentInstancesSameSecret(t *testing.T) {
	m1 := New("shared-secret", 1*time.Hour, 168*time.Hour, "vnet")
	m2 := New("shared-secret", 1*time.Hour, 168*time.Hour, "vnet")

	token, err := m1.GenerateAccessToken("user-1", "admin", "R_SUPER", "", []string{"read"})
	require.NoError(t, err)

	claims, err := m2.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "admin", claims.Username)
}
