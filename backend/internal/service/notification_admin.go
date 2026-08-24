package service

import (
	"errors"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type NotificationAdminService struct {
	db    *gorm.DB
	hub   *hub.Hub
	audit *AuditService
}

func NewNotificationAdminService(db *gorm.DB, wsHub *hub.Hub, audit *AuditService) *NotificationAdminService {
	return &NotificationAdminService{db: db, hub: wsHub, audit: audit}
}

type NotificationResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type CreateNotificationRequest struct {
	Type    string `json:"type" binding:"required"`
	Title   string `json:"title" binding:"required"`
	Content string `json:"content"`
}

type UpdateNotificationRequest struct {
	Type    *string `json:"type"`
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

func (s *NotificationAdminService) List(params pagination.Params) ([]NotificationResponse, int64, int, int, error) {
	var total int64
	query := s.db.Model(&model.Notification{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}
	var items []model.Notification
	if err := pagination.Apply(query, &params).Order("created_at desc").Find(&items).Error; err != nil {
		return nil, 0, 0, 0, err
	}
	result := make([]NotificationResponse, len(items))
	for i, n := range items {
		result[i] = notificationToResponse(n)
	}
	return result, total, params.Page, params.PageSize, nil
}

func (s *NotificationAdminService) GetByID(id string) (*NotificationResponse, error) {
	var n model.Notification
	if err := s.db.First(&n, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("notification not found")
		}
		return nil, err
	}
	r := notificationToResponse(n)
	return &r, nil
}

func (s *NotificationAdminService) Create(req *CreateNotificationRequest) (*NotificationResponse, error) {
	n := model.Notification{
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
	}
	if err := s.db.Create(&n).Error; err != nil {
		return nil, err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "notification",
		EntityID:   n.ID,
		Metadata:   map[string]interface{}{"title": n.Title, "type": n.Type},
	})
	r := notificationToResponse(n)
	return &r, nil
}

func (s *NotificationAdminService) Update(id string, req *UpdateNotificationRequest) (*NotificationResponse, error) {
	var n model.Notification
	if err := s.db.First(&n, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("notification not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if len(updates) > 0 {
		if err := s.db.Model(&n).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "notification",
		EntityID:   id,
		Metadata:   updates,
	})
	s.db.First(&n, "id = ?", id)
	r := notificationToResponse(n)
	return &r, nil
}

func (s *NotificationAdminService) Delete(id string) error {
	var n model.Notification
	if err := s.db.First(&n, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("notification not found")
		}
		return err
	}
	// Delete all recipient records first
	s.db.Where("notification_id = ?", id).Delete(&model.NotificationRecipient{})
	if err := s.db.Delete(&n).Error; err != nil {
		return err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "notification",
		EntityID:   n.ID,
		Metadata:   map[string]interface{}{"title": n.Title},
	})
	return nil
}

func (s *NotificationAdminService) Dispatch(notificationID string) (int, error) {
	var n model.Notification
	if err := s.db.First(&n, "id = ?", notificationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("notification not found")
		}
		return 0, err
	}
	var memberIDs []string
	if err := s.db.Model(&model.Member{}).Pluck("id", &memberIDs).Error; err != nil {
		return 0, err
	}
	if len(memberIDs) == 0 {
		return 0, nil
	}
	// Two rows per member, deliberately: NotificationRecipient is the dispatch
	// ledger (who this was sent to), MemberNotification is the member's own
	// copy and is what GET /api/notifications reads. Dispatch used to write
	// only the former, which nothing reads, so members never saw anything.
	batchSize := 500
	inserted := 0
	for i := 0; i < len(memberIDs); i += batchSize {
		end := i + batchSize
		if end > len(memberIDs) {
			end = len(memberIDs)
		}

		chunk := memberIDs[i:end]
		recipients := make([]model.NotificationRecipient, 0, len(chunk))
		inbox := make([]model.MemberNotification, 0, len(chunk))
		for _, mid := range chunk {
			recipients = append(recipients, model.NotificationRecipient{
				NotificationID: notificationID,
				RecipientID:    mid,
			})
			inbox = append(inbox, model.MemberNotification{
				MemberID: mid,
				Title:    n.Title,
				Body:     n.Content,
			})
		}

		tx := s.db.Begin()
		if err := tx.Create(&recipients).Error; err != nil {
			tx.Rollback()
			return inserted, err
		}
		if err := tx.Create(&inbox).Error; err != nil {
			tx.Rollback()
			return inserted, err
		}
		if err := tx.Commit().Error; err != nil {
			return inserted, err
		}
		inserted += len(chunk)
	}

	// The desktop client already listens for this; nothing ever sent it.
	s.hub.Broadcast(hub.Event{
		Type: "notification:new",
		Data: map[string]interface{}{
			"notification_id": notificationID,
			"title":           n.Title,
			"body":            n.Content,
		},
	})
	s.audit.Log(&LogAuditRequest{
		Action:     "dispatch",
		EntityType: "notification",
		EntityID:   notificationID,
		Metadata:   map[string]interface{}{"title": n.Title, "recipients": inserted},
	})
	return inserted, nil
}

func notificationToResponse(n model.Notification) NotificationResponse {
	return NotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Content:   n.Content,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
