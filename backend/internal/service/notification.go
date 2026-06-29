package service

import (
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type NotificationService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewNotificationService(db *gorm.DB, audit *AuditService) *NotificationService {
	return &NotificationService{db: db, audit: audit}
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
	return n, nil
}
