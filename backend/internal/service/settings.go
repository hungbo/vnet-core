package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

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

func (s *SettingsService) List() (map[string][]SettingResponse, error) {
	var settings []model.SystemSetting
	if err := s.db.Order("group_name, key").Find(&settings).Error; err != nil {
		return nil, err
	}

	grouped := make(map[string][]SettingResponse)
	for _, setting := range settings {
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
	result := make([]SettingResponse, len(settings))
	for i, setting := range settings {
		result[i] = SettingResponse{
			GroupName:   setting.GroupName,
			Key:         setting.Key,
			Value:       decodeSettingValue(setting.Value),
			Description: setting.Description,
		}
	}

	return result, nil
}

func (s *SettingsService) Update(groupName string, settings map[string]interface{}) ([]SettingResponse, error) {
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
