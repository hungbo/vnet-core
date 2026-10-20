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

// Nguồn của một game ở quán. Game "hub" đến từ catalog (master hoặc Hub), được đồng bộ và cập nhật tự động.
// Game "local" do quán tự tải và tự khai: có trên menu máy trạm nhưng nằm ngoài vòng đồng bộ, catalog không bao giờ
// ghi đè hay xoá nó, và Hub không biết nó có.
const (
	GameOriginHub   = "hub"
	GameOriginLocal = "local"
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
	// Icon: tệp lấy icon cho menu máy trạm (.exe/.dll/.ico, tương đối trong thư mục
	// game như Launcher). Rỗng là lấy từ Launcher. Cần khi launcher là trình mở chung,
	// ví dụ Riot Client mở Liên Minh.
	Icon string `gorm:"type:varchar(500)" json:"icon"`
	// MenuEntries: nhiều mục trên menu máy trạm cho cùng một thư mục game (LibreOffice
	// → Word và Excel). Rỗng là một mục theo DisplayName/Launcher/LaunchArgs/Icon.
	MenuEntries GameMenuEntries `json:"menu_entries"`
	Enabled     bool            `gorm:"default:true" json:"enabled"`
	AutoUpdate  bool            `gorm:"default:true" json:"auto_update"`
	// Priority: số lớn tải trước.
	Priority int `gorm:"default:0" json:"priority"`
	// RunFirst: lệnh chạy trước khi mở game (catalog của Hub, hoặc do quản trị viên đặt ở master và game cục bộ).
	RunFirst GameRunSteps `gorm:"type:jsonb" json:"run_first"`
	// Custom: tuỳ chỉnh riêng của quán, catalog không ghi đè (xem GameCustom).
	Custom GameCustom `gorm:"type:jsonb" json:"custom"`
	// Origin: GameOriginHub (mặc định, mọi game có từ trước) hoặc GameOriginLocal.
	Origin string `gorm:"type:varchar(10);not null;default:'hub'" json:"origin"`

	Version   int    `gorm:"default:0" json:"version"`
	Infohash  string `gorm:"type:varchar(64)" json:"infohash"`
	SizeBytes int64  `gorm:"default:0" json:"size_bytes"`
	// Fingerprint (master): dấu vân tay thư mục của bản đã publish.
	Fingerprint string `gorm:"type:varchar(64)" json:"-"`

	RemoteVersion  int    `gorm:"default:0" json:"remote_version"`
	RemoteInfohash string `gorm:"type:varchar(64)" json:"remote_infohash"`
	// RemoteWebseeds: nơi tải dữ liệu do catalog chỉ định (Hub: máy master khác máy catalog, URL đã ký cho quán này).
	// Rỗng là suy ra từ địa chỉ upstream như master cũ. Không trả ra API: URL mang token của quán.
	RemoteWebseeds StringArray `gorm:"type:jsonb" json:"-"`
	// RemoteTrackers: tracker WebRTC do catalog chỉ định (Hub; URL mang token của quán). Có tracker mà RemoteWebseeds
	// rỗng nghĩa là master chỉ phát qua P2P, không suy webseed từ địa chỉ upstream. Không trả ra API vì mang token.
	RemoteTrackers StringArray `gorm:"type:jsonb" json:"-"`
	// RemoteRelays: địa chỉ relay của Hub đã ký cho quán này (catalog `relay_webseeds`): đường cuối cùng khi P2P và webseed
	// đều không có dữ liệu. Mang token nên không trả ra API.
	RemoteRelays StringArray `gorm:"type:jsonb" json:"-"`

	Status   string `gorm:"type:varchar(20);default:'not_installed'" json:"status"`
	Progress int    `gorm:"default:0" json:"progress"` // phần trăm, 0–100
	Error    string `gorm:"type:text" json:"error"`

	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`

	// LaunchCount: số lượt khách bấm "Chơi" trên menu máy trạm (bảng
	// game_launches). Không phải cột: GameService.List đếm khi tải danh sách.
	LaunchCount int64 `gorm:"-" json:"launch_count"`
}

// GameLaunch là một lượt bấm "Chơi" trên menu game của máy trạm, ghi sau khi
// game đã mở được. Một dòng mỗi lượt để sau này đếm được theo ngày, theo máy.
type GameLaunch struct {
	ID     string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GameID string `gorm:"type:uuid;not null;index" json:"game_id"`
	// MenuKey: "<tên thư mục>" hoặc "<tên thư mục>/<i>" khi game có nhiều mục
	// menu (LibreOffice → Word, Excel).
	MenuKey     string    `gorm:"type:varchar(200)" json:"menu_key"`
	MemberID    *string   `gorm:"type:uuid;index" json:"member_id"`
	MachineCode string    `gorm:"type:varchar(256)" json:"machine_code"`
	CreatedAt   time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
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
