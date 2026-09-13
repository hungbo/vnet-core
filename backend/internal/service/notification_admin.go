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
			return nil, errors.New("không tìm thấy thông báo")
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
			return nil, errors.New("không tìm thấy thông báo")
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
			return errors.New("không tìm thấy thông báo")
		}
		return err
	}
	// Sổ người nhận là con SỞ HỮU của thông báo. Bản cũ xoá nó rồi BỎ QUA lỗi
	// và xoá tiếp thông báo ở một câu lệnh khác: hỏng ở bước đầu là để lại một
	// đống dòng người nhận trỏ tới thông báo không còn tồn tại.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("notification_id = ?", id).Delete(&model.NotificationRecipient{}).Error; err != nil {
			return err
		}
		return tx.Delete(&n).Error
	}); err != nil {
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
			return 0, errors.New("không tìm thấy thông báo")
		}
		return 0, err
	}
	// Bỏ qua hội viên đã nhận thông báo này: bấm Gửi lần hai — hoặc hai người
	// trực cùng bấm — trước đây nhét thêm một bản sao vào hộp thư của TẤT CẢ
	// hội viên, không có gì chặn. Sổ người nhận sinh ra để ghi "ai đã nhận",
	// nên lấy đúng nó làm mốc chống trùng; hội viên đăng ký sau lần gửi đầu
	// vẫn nhận được ở lần bấm sau.
	var memberIDs []string
	if err := s.db.Model(&model.Member{}).
		Where(`NOT EXISTS (SELECT 1 FROM notification_recipients nr WHERE nr.notification_id = ? AND nr.recipient_id = members.id)`, notificationID).
		Pluck("id", &memberIDs).Error; err != nil {
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
	//
	// Gửi theo danh sách người nhận chứ không Broadcast: Broadcast đánh thức cả
	// kết nối quản trị lẫn máy trạm chưa có ai đăng nhập, mà những chỗ đó không
	// có hộp thư nào để mà mở. SendToUsers phủ MỌI thiết bị của từng hội viên —
	// một người mở cả máy trạm lẫn điện thoại thì cả hai cùng sáng đèn.
	s.hub.SendToUsers(memberIDs, hub.Event{
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
