package model

import "time"

// Trạng thái của một game trên máy chủ quán (ở master luôn là ready hoặc error).
const (
	GameStatusNotInstalled = "not_installed"
	GameStatusQueued       = "queued"
	GameStatusDownloading  = "downloading"
	GameStatusReady        = "ready"
	GameStatusPublishing   = "publishing" // master đang băm để publish
	GameStatusError        = "error"
)

// Game là một thư mục game được phân phối. Name trùng tên thư mục con trong
// GAME_ROOT, và là khoá nối giữa master với các quán.
//
// Ở master: Version/Infohash là bản đã publish gần nhất. Ở quán: Remote* là bản
// master đang có, còn Version/Infohash là bản đã nằm trọn trên ổ của quán.
type Game struct {
	ID          string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
	DisplayName string `gorm:"type:varchar(200);not null" json:"display_name"`
	Category    string `gorm:"type:varchar(50)" json:"category"`
	// Launcher: đường dẫn tương đối trong thư mục game, ví dụ "Bin/Game.exe".
	Launcher   string `gorm:"type:varchar(500)" json:"launcher"`
	LaunchArgs string `gorm:"type:varchar(500)" json:"launch_args"`
	Enabled    bool   `gorm:"default:true" json:"enabled"`
	AutoUpdate bool   `gorm:"default:true" json:"auto_update"`
	// Priority: số lớn tải trước.
	Priority int `gorm:"default:0" json:"priority"`

	Version   int    `gorm:"default:0" json:"version"`
	Infohash  string `gorm:"type:varchar(64)" json:"infohash"`
	SizeBytes int64  `gorm:"default:0" json:"size_bytes"`
	// Fingerprint (master): dấu vân tay thư mục của bản đã publish.
	Fingerprint string `gorm:"type:varchar(64)" json:"-"`

	RemoteVersion  int    `gorm:"default:0" json:"remote_version"`
	RemoteInfohash string `gorm:"type:varchar(64)" json:"remote_infohash"`

	Status   string `gorm:"type:varchar(20);default:'not_installed'" json:"status"`
	Progress int    `gorm:"default:0" json:"progress"` // phần trăm, 0–100
	Error    string `gorm:"type:text" json:"error"`

	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`
}

// GamePublish lưu tệp .torrent của từng bản đã publish (chỉ dùng ở master).
type GamePublish struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GameID    string    `gorm:"type:uuid;not null;index" json:"game_id"`
	Version   int       `gorm:"not null" json:"version"`
	Infohash  string    `gorm:"type:varchar(64);not null" json:"infohash"`
	Torrent   []byte    `gorm:"type:bytea;not null" json:"-"`
	FileCount int       `gorm:"default:0" json:"file_count"`
	SizeBytes int64     `gorm:"default:0" json:"size_bytes"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
