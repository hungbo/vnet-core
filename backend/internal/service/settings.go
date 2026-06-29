package service

import (
	"errors"
	"fmt"

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
			Value:       setting.Value,
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

	result := make([]SettingResponse, len(settings))
	for i, setting := range settings {
		result[i] = SettingResponse{
			GroupName:   setting.GroupName,
			Key:         setting.Key,
			Value:       setting.Value,
			Description: setting.Description,
		}
	}

	return result, nil
}

func (s *SettingsService) Update(groupName string, settings map[string]interface{}) ([]SettingResponse, error) {
	var settingKeys []string
	for key, value := range settings {
		settingKeys = append(settingKeys, key)
		var strValue string
		switch v := value.(type) {
		case string:
			strValue = v
		case map[string]interface{}:
			strValue = fmt.Sprintf("%v", v)
		default:
			strValue = fmt.Sprintf("%v", v)
		}

		var existing model.SystemSetting
		err := s.db.Where("group_name = ? AND key = ?", groupName, key).First(&existing).Error
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
		IPAddress:  "",
	})
	return s.GetByGroup(groupName)
}
