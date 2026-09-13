package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	Host           string
	Port           int
	Mode           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxUploadSize  int64
	MaxFileSize    int64
	UploadDir      string
	BackupDir      string
	AllowedOrigins []string
	// Số ngày giữ lại lịch sử phần cứng. 0 = giữ tất cả.
	HardwareHistoryDays int
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	LogLevel        string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
}

// ModeRelease is the GIN_MODE value that turns on the production guardrails.
const ModeRelease = "release"

// DefaultJWTSecret is the placeholder shipped in .env.example. Running in
// release mode with this value still set is refused at startup.
const DefaultJWTSecret = "change-me-in-production"

// MinJWTSecretLen is the shortest secret accepted in release mode.
const MinJWTSecretLen = 32

func Load() *Config {
	mode := getEnv("GIN_MODE", "debug")

	// Wildcard CORS is convenient for local development but must be an
	// explicit choice in production, never a default.
	defaultOrigins := []string{"*"}
	if mode == ModeRelease {
		defaultOrigins = nil
	}

	return &Config{
		Server: ServerConfig{
			Host:          getEnv("SERVER_HOST", "0.0.0.0"),
			Port:          getEnvInt("SERVER_PORT", 20800),
			Mode:          mode,
			ReadTimeout:   getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:  getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			MaxUploadSize: getEnvInt64("MAX_UPLOAD_SIZE", 10<<20),
			MaxFileSize:   getEnvInt64("MAX_FILE_SIZE", 5<<20),
			UploadDir:     getEnv("UPLOAD_DIR", "uploads"),
			// Bảng lịch sử phần cứng nhận khoảng 5.800 dòng mỗi ngày cho mỗi
			// máy. Bảy ngày là đủ để truy "máy nào hay nóng" mà không để bảng
			// phình vô hạn; đặt 0 nếu muốn giữ tất cả.
			HardwareHistoryDays: getEnvInt("HARDWARE_HISTORY_DAYS", 7),
			BackupDir:           getEnv("BACKUP_DIR", "backups"),
			AllowedOrigins:      getEnvSlice("ALLOWED_ORIGINS", defaultOrigins),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "vnet"),
			Password:        getEnv("DB_PASSWORD", "vnet"),
			Name:            getEnv("DB_NAME", "vnet"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 100),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", time.Hour),
			LogLevel:        getEnv("DB_LOG_LEVEL", "warn"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", DefaultJWTSecret),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TTL", 24*time.Hour),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			Issuer:          getEnv("JWT_ISSUER", "vnet"),
		},
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Ho_Chi_Minh",
		c.Database.Host, c.Database.Port, c.Database.User, c.Database.Password,
		c.Database.Name, c.Database.SSLMode,
	)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		return splitAndTrim(v, ",")
	}
	return fallback
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range split(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func split(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		result = append(result, s[start:])
	}
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// Validate refuses to start a production server on development defaults.
// Every check here is a setting that is safe locally and dangerous in release.
func (c *Config) Validate() error {
	if c.Server.Mode != ModeRelease {
		return nil
	}

	if c.JWT.Secret == DefaultJWTSecret {
		return errors.New("JWT_SECRET is still the development placeholder; set a real secret before running in release mode")
	}
	if len(c.JWT.Secret) < MinJWTSecretLen {
		return fmt.Errorf("JWT_SECRET must be at least %d characters in release mode, got %d", MinJWTSecretLen, len(c.JWT.Secret))
	}

	if len(c.Server.AllowedOrigins) == 0 {
		return errors.New("ALLOWED_ORIGINS must list the admin origins explicitly in release mode")
	}
	for _, origin := range c.Server.AllowedOrigins {
		if origin == "*" {
			return errors.New("ALLOWED_ORIGINS must not contain \"*\" in release mode")
		}
	}

	return nil
}
