package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Game     GameConfig
}

// Vai trò của máy chủ trong việc phân phối game.
const (
	GameRoleOff    = ""       // không chạy gì liên quan tới cập nhật game
	GameRoleMaster = "master" // giữ bản gốc, publish và phát cho các quán
	GameRoleCafe   = "cafe"   // tải game từ master về ổ game của quán
)

// GameConfig cấu hình cập nhật game. Mặc định tắt: một máy chủ quán bình thường
// không tự nhiên mở cổng BitTorrent hay ghi vào ổ đĩa nào.
type GameConfig struct {
	Role string
	// Root: thư mục chứa các game, mỗi game một thư mục con trùng tên game.
	// Ở quán đây là Game Disk của Cloud Update, ví dụ D:\Games.
	Root string
	// DataDir: trạng thái riêng của engine BitTorrent, không chứa dữ liệu game.
	// Trống thì dùng GAME_ROOT/.vnet-gameupdate — dịch vụ Windows chạy với thư
	// mục làm việc là System32, nên một đường dẫn tương đối ở đây là sai chỗ.
	DataDir string
	// UpstreamURL: địa chỉ vnet-server của master, chỉ dùng ở quán.
	UpstreamURL string
	// PublicURL: địa chỉ các quán dùng để gọi master; webseed nằm ở PublicURL/seed/.
	PublicURL string
	// CatalogKey: khoá dùng chung giữa master và các quán để đọc catalog.
	CatalogKey   string
	ListenPort   int
	PollInterval time.Duration
	// StableFor: thư mục game phải đứng yên bấy lâu mới publish, tránh phát ra
	// một game đang được launcher vá dở.
	StableFor time.Duration
	// StallTimeout: một lượt tải mà bấy lâu không có thêm piece nào được xác thực đúng hash thì bị
	// hủy, game bị đánh dấu lỗi và việc tải chuyển sang game khác (rồi thử lại sau, giãn dần).
	// Tính theo piece ĐÃ XÁC THỰC chứ không theo byte nhận về: master đổi tệp sau khi publish thì
	// dữ liệu vẫn chảy nhưng không piece nào khớp. Piece dài tối đa 4 MB nên đường truyền chậm hơn
	// ~7 kB/s sẽ bị coi là treo oan — khi đó tăng giá trị này.
	StallTimeout time.Duration
}

// DefaultGameStallTimeout là GAME_STALL_TIMEOUT khi không đặt, đặt sai hoặc <= 0 (không có chế độ
// "tắt": đồng hồ này là thứ ngăn một lượt tải treo giữ chặn cả quán).
const DefaultGameStallTimeout = 10 * time.Minute

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

	stall := getEnvDuration("GAME_STALL_TIMEOUT", DefaultGameStallTimeout)
	if stall <= 0 {
		stall = DefaultGameStallTimeout
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
		Game: GameConfig{
			Role:         getEnv("GAME_ROLE", GameRoleOff),
			Root:         getEnv("GAME_ROOT", ""),
			DataDir:      getEnv("GAME_DATA_DIR", ""),
			UpstreamURL:  strings.TrimRight(getEnv("GAME_UPSTREAM_URL", ""), "/"),
			PublicURL:    strings.TrimRight(getEnv("GAME_PUBLIC_URL", ""), "/"),
			CatalogKey:   getEnv("GAME_CATALOG_KEY", ""),
			ListenPort:   getEnvInt("GAME_LISTEN_PORT", 19999),
			PollInterval: getEnvDuration("GAME_POLL_INTERVAL", 5*time.Minute),
			StableFor:    getEnvDuration("GAME_STABLE_FOR", 10*time.Minute),
			StallTimeout: stall,
		},
	}
}

// Validate kiểm cấu hình game ở mọi chế độ: sai ở đây là xoá nhầm thư mục
// hoặc phát game ra ngoài không khoá.
func (g GameConfig) Validate() error {
	switch g.Role {
	case GameRoleOff:
		return nil
	case GameRoleMaster, GameRoleCafe:
	default:
		return fmt.Errorf("GAME_ROLE must be empty, %q or %q, got %q", GameRoleMaster, GameRoleCafe, g.Role)
	}
	if g.Root == "" || !filepath.IsAbs(g.Root) {
		return errors.New("GAME_ROOT must be an absolute path when GAME_ROLE is set")
	}
	if len(g.CatalogKey) < 16 {
		return errors.New("GAME_CATALOG_KEY must be at least 16 characters when GAME_ROLE is set")
	}
	if g.Role == GameRoleCafe && g.UpstreamURL == "" {
		return errors.New("GAME_UPSTREAM_URL is required when GAME_ROLE=cafe")
	}
	if g.Role == GameRoleMaster && g.PublicURL == "" {
		return errors.New("GAME_PUBLIC_URL is required when GAME_ROLE=master")
	}
	return nil
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
	if err := c.Game.Validate(); err != nil {
		return err
	}
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
