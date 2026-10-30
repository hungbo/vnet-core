package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

type SettingsService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewSettingsService(db *gorm.DB, audit *AuditService) *SettingsService {
	return &SettingsService{db: db, audit: audit}
}

type SettingResponse struct {
	GroupName   string `json:"group_name"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// decodeSettingValue turns the stored JSON back into what the caller wrote.
// A JSON string is handed back as plain text so a form shows `abc`, not `"abc"`;
// anything structured is passed through as JSON for the caller to parse.
func decodeSettingValue(stored string) string {
	var asString string
	if err := json.Unmarshal([]byte(stored), &asString); err == nil {
		return asString
	}
	return stored
}

// PaymentMethods trả về danh sách phương thức thanh toán đã cài (kể cả mục
// đang tắt, để bảng lịch sử vẫn hiện được tên).
func (s *SettingsService) PaymentMethods() []PaymentMethodOption {
	return PaymentMethods(s.db)
}

func (s *SettingsService) List() (map[string][]SettingResponse, error) {
	var settings []model.SystemSetting
	if err := s.db.Order("group_name, key").Find(&settings).Error; err != nil {
		return nil, err
	}

	grouped := make(map[string][]SettingResponse)
	for _, setting := range settings {
		if isSecretSetting(setting.GroupName, setting.Key) {
			continue
		}
		grouped[setting.GroupName] = append(grouped[setting.GroupName], SettingResponse{
			GroupName:   setting.GroupName,
			Key:         setting.Key,
			Value:       decodeSettingValue(setting.Value),
			Description: setting.Description,
		})
	}

	return grouped, nil
}

func (s *SettingsService) GetByGroup(groupName string) ([]SettingResponse, error) {
	var settings []model.SystemSetting
	if err := s.db.Where("group_name = ?", groupName).Order("key").Find(&settings).Error; err != nil {
		return nil, err
	}
	// Nhóm chưa có dòng nào là trạng thái LẦN ĐẦU bình thường — trang Cài đặt
	// mở ra trắng rồi lần Lưu đầu tiên tạo dòng. Trả 404 ở đây bắt giao diện
	// phải đi một nhánh riêng và để lại một lỗi đỏ trong console mỗi lần mở tab
	// chưa dùng tới.
	result := make([]SettingResponse, 0, len(settings))
	for _, setting := range settings {
		if isSecretSetting(setting.GroupName, setting.Key) {
			continue
		}
		result = append(result, SettingResponse{
			GroupName:   setting.GroupName,
			Key:         setting.Key,
			Value:       decodeSettingValue(setting.Value),
			Description: setting.Description,
		})
	}

	return result, nil
}

func (s *SettingsService) Update(groupName string, settings map[string]interface{}) ([]SettingResponse, error) {
	if groupName == ClientGroup {
		var err error
		if err = kiemTraCaiDatMayTramThem(settings); err != nil {
			return nil, err
		}
		if settings, err = prepareClientSettings(settingsGroup(s.db, ClientGroup), settings); err != nil {
			return nil, err
		}
	}
	if groupName == GeneralGroup {
		if err := kiemTraLogoMenu(settings); err != nil {
			return nil, err
		}
	}
	if groupName == PaymentGroup {
		var err error
		if settings, err = preparePaymentSettings(settings); err != nil {
			return nil, err
		}
	}
	if groupName == GamesGroup {
		if err := kiemTraCaiDatGame(settings); err != nil {
			return nil, err
		}
	}
	if groupName == TopupGroup {
		if raw, ok := settings[TopupBonusTiersKey]; ok {
			tiers, err := chuanHoaMocNapTang(raw)
			if err != nil {
				return nil, err
			}
			settings[TopupBonusTiersKey] = tiers
		}
	}
	var settingKeys []string
	for key, value := range settings {
		settingKeys = append(settingKeys, key)
		// The column is jsonb, so the value has to be marshalled rather than
		// printed: a bare "abc" is not valid JSON and PostgreSQL rejected every
		// text setting that was ever saved.
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("setting %q has a value that cannot be stored: %w", key, err)
		}
		strValue := string(raw)

		var existing model.SystemSetting
		err = s.db.Where("group_name = ? AND key = ?", groupName, key).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				setting := model.SystemSetting{
					GroupName: groupName,
					Key:       key,
					Value:     strValue,
				}
				if err := s.db.Create(&setting).Error; err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		} else {
			if err := s.db.Model(&existing).Update("value", strValue).Error; err != nil {
				return nil, err
			}
		}
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "upsert",
		EntityType: "system_setting",
		EntityID:   "",
		UserID:     nil,
		Metadata: map[string]interface{}{
			"group_name":   groupName,
			"setting_keys": settingKeys,
		},
		IPAddress: "",
	})
	return s.GetByGroup(groupName)
}

// --- đọc cài đặt từ các service khác -----------------------------------------
//
// Ba nhóm cài đặt (`invoice`, `limits`, `printer`) từng lưu được nhưng không
// service nào đọc: người vận hành điền vào form, bấm Lưu, thấy "thành công", và
// không có gì thay đổi. Hai hàm dưới đây là đường đọc dùng chung để mỗi nơi cần
// cấu hình không phải tự viết lại truy vấn.

// settingsGroup trả về toàn bộ cặp key/value của một nhóm, đã bóc lớp JSON.
func settingsGroup(db *gorm.DB, group string) map[string]string {
	out := map[string]string{}
	var rows []model.SystemSetting
	if err := db.Where("group_name = ?", group).Find(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[r.Key] = strings.TrimSpace(decodeSettingValue(r.Value))
	}
	return out
}

// settingInt đọc một số nguyên; thiếu khoá, để trống hoặc không phải số đều trả
// về def. Quy ước xuyên suốt: 0 nghĩa là "không giới hạn", nên nơi gọi phải tự
// quyết định 0 có ý nghĩa gì với mình.
func settingInt(m map[string]string, key string, def int64) int64 {
	v, ok := m[key]
	if !ok || v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// FeaturesGroup là nhóm cài đặt chứa các công tắc bật/tắt tính năng, đọc được cả
// từ máy trạm vì GET /api/settings/:group cố ý không gắn StaffOnly.
const FeaturesGroup = "features"

// FeatureEnabled cho biết một tính năng có đang bật hay không.
//
// Mặc định BẬT: nhóm "features" chưa tồn tại cho tới lần Lưu đầu tiên ở trang
// quản trị, và GetByGroup trả 404 khi nhóm rỗng. Mặc định tắt sẽ làm cả điểm
// danh lẫn đánh giá biến mất ở mọi quán chưa từng mở tab đó.
func FeatureEnabled(db *gorm.DB, key string) bool {
	return settingBool(settingsGroup(db, FeaturesGroup), key, true)
}

// settingBool đọc một công tắc. Giá trị lưu trong cột jsonb được
// decodeSettingValue trả về dạng CHUỖI, nên "false" là một chuỗi khác rỗng —
// kiểm bằng truthiness sẽ luôn ra "bật". Phải parse thật.
func settingBool(m map[string]string, key string, def bool) bool {
	v, ok := m[key]
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

// ClientGroup là nhóm cài đặt chính sách bảo vệ của máy trạm.
const ClientGroup = "client"

// ClientPolicy: máy trạm làm gì khi phần mềm bị can thiệp hoặc mất kết nối với
// máy chủ. Máy trạm lưu lại bản gần nhất để vẫn áp được lúc mất mạng.
type ClientPolicy struct {
	// TamperAction: "restart" hoặc "shutdown". Quán diskless chọn restart — máy
	// khởi động lại là về bản gốc, khách ngồi lại dùng tiếp được.
	TamperAction string `json:"tamper_action"`
	// Mất kết nối bấy nhiêu giây thì khoá màn hình.
	OfflineLockSeconds int64 `json:"offline_lock_seconds"`
	// Mất kết nối bấy nhiêu giây thì khởi động lại. 0 là không bao giờ.
	OfflineRebootSeconds int64 `json:"offline_reboot_seconds"`
	// Tài khoản quản trị máy trạm; chỉ gửi khi đã đặt đủ tên và mật khẩu. Có
	// thì máy trạm dùng nó thay cho tài khoản đặt lúc cài.
	LocalAdminUsername string `json:"local_admin_username,omitempty"`
	LocalAdminHash     string `json:"local_admin_hash,omitempty"`
	// BlockedApps: tên tiến trình cấm chạy trên MỌI máy trạm, đã chuẩn hoá và
	// nối bằng dấu phẩy ("chrome,steam"). Luôn gửi, kể cả rỗng: rỗng nghĩa là
	// bỏ chặn hết, còn thiếu trường thì máy trạm không phân biệt được hai việc đó.
	BlockedApps string `json:"blocked_apps"`
	// HiddenShortcuts: tên lối tắt (không đuôi) mà dịch vụ máy trạm xoá khỏi màn
	// hình desktop, đã chuẩn hoá và nối bằng dấu phẩy ("Game Menu,Teamfight
	// Tactics"). Luôn gửi, kể cả rỗng: rỗng là tắt tính năng.
	HiddenShortcuts string `json:"hidden_shortcuts"`
	// RiotMetadataSource: thư mục trên Game Disk chứa bản metadata Riot mà dịch
	// vụ máy trạm chép vào C:\ProgramData\Riot Games. Luôn gửi, kể cả rỗng:
	// rỗng là tắt; máy chủ tự điền khi admin để trống cài đặt (xem
	// MachineService.ClientPolicy), cài đặt `off` là tắt hẳn.
	RiotMetadataSource string `json:"riot_metadata_source"`
	// Máy ở màn hình khoá, không ai đăng nhập bấy nhiêu phút thì tự tắt. 0 là
	// không bao giờ. Luôn gửi: máy chủ cũ không có trường này thì máy trạm hiểu
	// là 0, tức là tắt tính năng — không bao giờ tắt máy theo một mặc định
	// máy chủ không hề biết.
	IdleShutdownMinutes int64 `json:"idle_shutdown_minutes"`
	// Hình nền máy trạm (C1): các đường dẫn /uploads/... nối bằng xuống dòng, và số giây giữa hai ảnh khi trình chiếu
	// (0 là đứng yên ở ảnh đầu). Luôn gửi, kể cả rỗng: rỗng là tắt, máy trạm không đổi hình nền của ai.
	Wallpapers               string `json:"wallpapers"`
	WallpaperIntervalSeconds int64  `json:"wallpaper_interval_seconds"`
	// StartupApps (C2): tệp .exe chạy mỗi khi khách đăng nhập, nối bằng xuống dòng; rỗng là tắt.
	StartupApps string `json:"startup_apps"`
	// CloseHotkey (C3): Alt+K trên máy trạm đóng mọi ứng dụng đang hiện cửa sổ, trừ CloseExclude (tên tiến trình, dấu phẩy).
	CloseHotkey  bool   `json:"close_hotkey"`
	CloseExclude string `json:"close_exclude"`
	// IdleAction: hết thời gian trên thì làm gì. "shutdown" là tắt máy. "standby"
	// dành cho Lightning Boot của Cloud Update: máy chưa ai đăng nhập từ lúc bật thì
	// cho ngủ chờ, đã có người dùng thì khởi động lại cho sạch rồi mới ngủ. Luôn
	// gửi; máy trạm cũ (trước 1.0.16) không biết trường này thì vẫn tắt máy.
	IdleAction string `json:"idle_action"`
}

// IdleShutdownKey: số phút máy sẵn sàng mà không ai đăng nhập thì tắt máy.
const IdleShutdownKey = "idle_shutdown_minutes"

// IdleActionKey: hết thời gian IdleShutdownKey thì "shutdown" hay "standby".
const IdleActionKey = "idle_action"

// BlockedAppsKey là danh sách ứng dụng bị chặn trong nhóm cài đặt máy trạm,
// mỗi dòng (hoặc mỗi dấu phẩy) một tên.
const BlockedAppsKey = "blocked_apps"

// tenUngDungHopLe: chỉ tên tiến trình, không đường dẫn, không ký tự lệnh — cùng
// luật với safeProcessName ở máy trạm, nơi kiểm lại trước khi tắt bất cứ thứ gì.
var tenUngDungHopLe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// chuanHoaUngDungChan: "Chrome.EXE\n steam, ,chrome" → "chrome,steam".
func chuanHoaUngDungChan(raw string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ';' }) {
		name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(part)), ".exe")
		if !tenUngDungHopLe.MatchString(name) || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// RiotMetadataSourceKey là thư mục metadata Riot trong nhóm cài đặt máy trạm.
const RiotMetadataSourceKey = "riot_metadata_source"

// RiotMetadataOff: giá trị cài đặt tắt hẳn tính năng (không phân biệt hoa thường). Để trống thì
// máy chủ tự điền đường dẫn (xem MachineService.ClientPolicy).
const RiotMetadataOff = "off"

// chuanHoaNguonRiot: chỉ nhận đường dẫn ổ đĩa tuyệt đối ("D:\Games\_riot"). Máy
// trạm đem đường dẫn này đi đọc bằng quyền SYSTEM nên tương đối, UNC, ký tự điều
// khiển hay đoạn ".." đều bị bỏ (trả ""), cùng luật với chuanHoaNguonRiot của client.
func chuanHoaNguonRiot(raw string) string {
	p := strings.TrimSpace(raw)
	if len(p) < 3 || len(p) > 260 {
		return ""
	}
	drive := p[0]
	if !(drive >= 'A' && drive <= 'Z' || drive >= 'a' && drive <= 'z') || p[1] != ':' || (p[2] != '\\' && p[2] != '/') {
		return ""
	}
	for _, r := range p {
		if unicode.IsControl(r) || strings.ContainsRune(`*?<>"|`, r) {
			return ""
		}
	}
	for _, part := range strings.FieldsFunc(p[3:], func(r rune) bool { return r == '\\' || r == '/' }) {
		if part == ".." {
			return ""
		}
	}
	return p
}

// HiddenShortcutsKey là danh sách lối tắt cần ẩn trong nhóm cài đặt máy trạm,
// mỗi dòng (hoặc mỗi dấu phẩy) một tên không có đuôi.
const HiddenShortcutsKey = "hidden_shortcuts"

const (
	maxHiddenShortcutName  = 100
	maxHiddenShortcutCount = 50
)

// chuanHoaLoiTatAn: "Game Menu.lnk\n game menu, ..\evil, Riot Client" →
// "Game Menu,Riot Client". Giữ chữ hoa và thứ tự gõ đầu tiên (máy trạm so tên
// không phân biệt hoa thường). Tên này được dịch vụ máy trạm đem đi XOÁ FILE nên
// thứ gì giống đường dẫn, ký tự đại diện hay quá dài thì bỏ hẳn chứ không cắt:
// cắt tên dài ra có thể trùng với một lối tắt khác không ai định xoá.
func chuanHoaLoiTatAn(raw string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' }) {
		name := strings.TrimSpace(part)
		// Chỉ cắt khi 4 byte cuối đúng là đuôi ASCII, để không cắt giữa một ký tự.
		// Lặp tới khi hết đuôi để chuẩn hoá lại không đổi giá trị (máy trạm cũng lặp).
		for n := len(name); n >= 4; n = len(name) {
			ext := strings.ToLower(name[n-4:])
			if ext != ".lnk" && ext != ".url" {
				break
			}
			name = strings.TrimSpace(name[:n-4])
		}
		key := strings.ToLower(name)
		if !tenLoiTatHopLe(name) || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
		if len(out) == maxHiddenShortcutCount {
			break
		}
	}
	return strings.Join(out, ",")
}

// tenLoiTatHopLe: một tên file thường, không đường dẫn, không ký tự đại diện.
func tenLoiTatHopLe(name string) bool {
	if name == "" || name == "." || name == ".." || utf8.RuneCountInString(name) > maxHiddenShortcutName {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`\/:*?<>"|`, r) {
			return false
		}
	}
	return true
}

const (
	defaultOfflineLockSeconds   = 60
	defaultOfflineRebootSeconds = 300
	// Dưới mức này thì một nhịp tim rớt (15 giây một nhịp) cũng đủ khoá máy.
	minOfflineLockSeconds      = 30
	defaultIdleShutdownMinutes = 5
	// Một ngày. Lớn hơn nữa thì cũng như tắt tính năng, chỉ khó hiểu hơn.
	maxIdleShutdownMinutes = 1440
)

func clientPolicyFrom(m map[string]string) ClientPolicy {
	p := ClientPolicy{
		TamperAction:         "restart",
		OfflineLockSeconds:   settingInt(m, "offline_lock_seconds", defaultOfflineLockSeconds),
		OfflineRebootSeconds: settingInt(m, "offline_reboot_seconds", defaultOfflineRebootSeconds),
		IdleAction:           "shutdown",
	}
	if u, h := m[LocalAdminUserKey], m[LocalAdminHashKey]; u != "" && h != "" {
		p.LocalAdminUsername, p.LocalAdminHash = u, h
	}
	if m["tamper_action"] == "shutdown" {
		p.TamperAction = "shutdown"
	}
	if m[IdleActionKey] == "standby" {
		p.IdleAction = "standby"
	}
	p.BlockedApps = chuanHoaUngDungChan(m[BlockedAppsKey])
	p.Wallpapers = strings.Join(chuanHoaHinhNen(m[WallpaperURLsKey]), "\n")
	p.WallpaperIntervalSeconds = khoangTrinhChieu(settingInt(m, WallpaperIntervalKey, 0))
	p.StartupApps = strings.Join(chuanHoaUngDungKhoiDong(m[StartupAppsKey]), "\n")
	p.CloseHotkey = settingBool(m, CloseHotkeyKey, false)
	p.CloseExclude = chuanHoaUngDungChan(m[CloseExcludeKey])
	p.HiddenShortcuts = chuanHoaLoiTatAn(m[HiddenShortcutsKey])
	p.RiotMetadataSource = chuanHoaNguonRiot(m[RiotMetadataSourceKey])
	p.IdleShutdownMinutes = settingInt(m, IdleShutdownKey, defaultIdleShutdownMinutes)
	if p.IdleShutdownMinutes < 0 {
		p.IdleShutdownMinutes = 0
	}
	if p.IdleShutdownMinutes > maxIdleShutdownMinutes {
		p.IdleShutdownMinutes = maxIdleShutdownMinutes
	}
	if p.OfflineLockSeconds < minOfflineLockSeconds {
		p.OfflineLockSeconds = minOfflineLockSeconds
	}
	if p.OfflineRebootSeconds < 0 {
		p.OfflineRebootSeconds = 0
	}
	// Khởi động lại trước cả khi khoá là vô nghĩa: phải cách bậc khoá ít nhất
	// một phút để máy chủ khởi động lại nhanh không kéo theo cả phòng máy.
	if p.OfflineRebootSeconds != 0 && p.OfflineRebootSeconds < p.OfflineLockSeconds+60 {
		p.OfflineRebootSeconds = p.OfflineLockSeconds + 60
	}
	return p
}
