package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func releaseConfig() *Config {
	c := &Config{}
	c.Server.Mode = ModeRelease
	c.Server.AllowedOrigins = []string{"https://admin.example.com"}
	c.JWT.Secret = "a-real-secret-that-is-long-enough-32"
	return c
}

func TestValidate_AcceptsProperReleaseConfig(t *testing.T) {
	assert.NoError(t, releaseConfig().Validate())
}

func TestValidate_SkipsChecksOutsideRelease(t *testing.T) {
	c := releaseConfig()
	c.Server.Mode = "debug"
	c.JWT.Secret = DefaultJWTSecret
	c.Server.AllowedOrigins = []string{"*"}

	assert.NoError(t, c.Validate())
}

func TestValidate_RejectsPlaceholderSecret(t *testing.T) {
	c := releaseConfig()
	c.JWT.Secret = DefaultJWTSecret

	assert.ErrorContains(t, c.Validate(), "JWT_SECRET")
}

func TestValidate_RejectsShortSecret(t *testing.T) {
	c := releaseConfig()
	c.JWT.Secret = "short"

	assert.ErrorContains(t, c.Validate(), "at least 32 characters")
}

func TestValidate_RejectsWildcardOrigin(t *testing.T) {
	c := releaseConfig()
	c.Server.AllowedOrigins = []string{"https://admin.example.com", "*"}

	assert.ErrorContains(t, c.Validate(), "ALLOWED_ORIGINS")
}

func TestValidate_RejectsEmptyOrigins(t *testing.T) {
	c := releaseConfig()
	c.Server.AllowedOrigins = nil

	assert.ErrorContains(t, c.Validate(), "ALLOWED_ORIGINS")
}

func TestLoad_NoWildcardOriginDefaultInRelease(t *testing.T) {
	t.Setenv("GIN_MODE", ModeRelease)

	assert.Empty(t, Load().Server.AllowedOrigins)
}

func TestLoad_WildcardOriginDefaultInDebug(t *testing.T) {
	t.Setenv("GIN_MODE", "debug")

	assert.Equal(t, []string{"*"}, Load().Server.AllowedOrigins)
}
