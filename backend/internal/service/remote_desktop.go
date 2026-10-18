package service

import (
	"errors"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

// ErrNoMachineIP: máy chưa báo địa chỉ IP qua nhịp tim nên không biết nối VNC vào đâu.
var ErrNoMachineIP = errors.New("máy chưa báo địa chỉ IP, chưa thể điều khiển từ xa")

// VNCPassword trả mật khẩu TightVNC của máy, sinh mới nếu chưa có. VNC chỉ
// dùng 8 ký tự đầu của mật khẩu, nên sinh đúng 8 ký tự hex.
func (s *MachineService) VNCPassword(machineID string) (string, error) {
	var m model.Machine
	if err := s.db.Select("id, vnc_password").First(&m, "id = ?", machineID).Error; err != nil {
		return "", err
	}
	if m.VNCPassword != "" {
		return m.VNCPassword, nil
	}
	pw, err := utils.GenerateRandomToken(4)
	if err != nil {
		return "", err
	}
	// Chỉ ghi khi vẫn còn trống: hai nhịp tim cùng lúc không được sinh hai
	// mật khẩu khác nhau. Bên thua đọc lại bản bên thắng đã ghi.
	res := s.db.Model(&model.Machine{}).
		Where("id = ? AND (vnc_password IS NULL OR vnc_password = '')", machineID).
		Update("vnc_password", pw)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		if err := s.db.Select("id, vnc_password").First(&m, "id = ?", machineID).Error; err != nil {
			return "", err
		}
		return m.VNCPassword, nil
	}
	return pw, nil
}

// RemoteDesktopTarget trả máy cần điều khiển, kèm IP, nếu máy đang bật.
func (s *MachineService) RemoteDesktopTarget(id string) (*model.Machine, error) {
	var m model.Machine
	if err := s.db.Select("id, machine_code, status, ip_address").First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy")
		}
		return nil, err
	}
	if m.Status == "offline" {
		return nil, ErrMachineOffline
	}
	if m.IPAddress == "" {
		return nil, ErrNoMachineIP
	}
	return &m, nil
}

// LogRemoteDesktop ghi nhật ký mở/đóng phiên remote desktop.
func (s *MachineService) LogRemoteDesktop(m *model.Machine, actorID string, started bool) {
	action := "remote_desktop_stop"
	if started {
		action = "remote_desktop_start"
	}
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     action,
		EntityType: "machine",
		EntityID:   m.ID,
		UserID:     actor,
		Metadata:   map[string]interface{}{"machine_code": m.MachineCode, "ip_address": m.IPAddress},
	})
}
