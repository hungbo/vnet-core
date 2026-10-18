package config

import (
	"testing"
	"time"

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

func TestGameConfigValidate(t *testing.T) {
	ok := GameConfig{Role: GameRoleCafe, Root: "/srv/games", CatalogKey: "0123456789abcdef", UpstreamURL: "http://master:20800"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("cấu hình quán hợp lệ bị từ chối: %v", err)
	}
	if err := (GameConfig{}).Validate(); err != nil {
		t.Fatalf("tắt tính năng thì không cần gì: %v", err)
	}
	bad := []GameConfig{
		{Role: "slave", Root: "/srv/games", CatalogKey: "0123456789abcdef"},
		{Role: GameRoleCafe, Root: "games", CatalogKey: "0123456789abcdef", UpstreamURL: "http://m"},
		{Role: GameRoleCafe, Root: "/srv/games", CatalogKey: "short", UpstreamURL: "http://m"},
		{Role: GameRoleCafe, Root: "/srv/games", CatalogKey: "0123456789abcdef"},
		{Role: GameRoleMaster, Root: "/srv/games", CatalogKey: "0123456789abcdef"},
	}
	for i, c := range bad {
		if c.Validate() == nil {
			t.Errorf("trường hợp %d phải bị từ chối: %+v", i, c)
		}
	}
}

func TestLoad_GameStallTimeout(t *testing.T) {
	assert.Equal(t, 10*time.Minute, Load().Game.StallTimeout, "mặc định")

	t.Setenv("GAME_STALL_TIMEOUT", "3m")
	assert.Equal(t, 3*time.Minute, Load().Game.StallTimeout)

	// Không có chế độ tắt: 0, số âm hay giá trị sai đều về mặc định.
	for _, bad := range []string{"0", "-5m", "abc"} {
		t.Setenv("GAME_STALL_TIMEOUT", bad)
		assert.Equal(t, 10*time.Minute, Load().Game.StallTimeout, bad)
	}
}
