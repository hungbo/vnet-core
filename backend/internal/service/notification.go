package service

import (
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type NotificationService struct {
	db    *gorm.DB
	audit *AuditService
	hub   *hub.Hub
}

func NewNotificationService(db *gorm.DB, audit *AuditService) *NotificationService {
	return &NotificationService{db: db, audit: audit}
}

// WithHub nối hub WebSocket vào để thông báo gửi riêng cho một hội viên được đẩy
// ngay. Dùng setter nối chuỗi thay vì thêm tham số vào constructor để mọi chỗ
// dựng service — kể cả trong test — không phải sửa.
func (s *NotificationService) WithHub(h *hub.Hub) *NotificationService {
	s.hub = h
	return s
}

func (s *NotificationService) List(memberID string, params pagination.Params) ([]model.MemberNotification, int64, int, int, error) {
	var total int64
	query := s.db.Model(&model.MemberNotification{}).Where("member_id = ?", memberID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var items []model.MemberNotification
	if err := pagination.Apply(query, &params).Find(&items).Error; err != nil {
		return nil, 0, 0, 0, err
	}
	return items, total, params.Page, params.PageSize, nil
}

func (s *NotificationService) UnreadCount(memberID string) (int64, error) {
	var count int64
	err := s.db.Model(&model.MemberNotification{}).Where("member_id = ? AND is_read = false", memberID).Count(&count).Error
	return count, err
}

func (s *NotificationService) MarkRead(id string, memberID string) error {
	return s.db.Model(&model.MemberNotification{}).
		Where("id = ? AND member_id = ?", id, memberID).
		Update("is_read", true).Error
}

func (s *NotificationService) MarkAllRead(memberID string) error {
	return s.db.Model(&model.MemberNotification{}).
		Where("member_id = ? AND is_read = false", memberID).
		Update("is_read", true).Error
}

func (s *NotificationService) Create(memberID, title, body string) (*model.MemberNotification, error) {
	n := &model.MemberNotification{
		MemberID: memberID,
		Title:    title,
		Body:     body,
	}
	if err := s.db.Create(n).Error; err != nil {
		return nil, err
	}

	// Đẩy tới MỌI thiết bị của hội viên đó. Bản cũ chỉ ghi xuống bảng: thông báo
	// gửi riêng cho một người nằm im trong hộp thư tới khi người đó tự mở ra.
	// Chỉ gửi id — nội dung lấy lại qua REST, đúng lối Dispatch đang làm.
	if s.hub != nil {
		s.hub.SendToUser(memberID, hub.Event{
			Type: "notification:new",
			Data: map[string]interface{}{"notification_id": n.ID},
		})
	}
	return n, nil
}
