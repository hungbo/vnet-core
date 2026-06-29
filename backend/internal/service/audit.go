package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

type AuditLogParams struct {
	Action     string `form:"action"`
	EntityType string `form:"entity_type"`
	UserID     string `form:"user_id"`
	DateFrom   string `form:"date_from"`
	DateTo     string `form:"date_to"`
}

type LogAuditRequest struct {
	Action     string      `json:"action" binding:"required"`
	EntityType string      `json:"entity_type" binding:"required"`
	EntityID   string      `json:"entity_id"`
	UserID     *string     `json:"user_id"`
	Metadata   interface{} `json:"metadata"`
	IPAddress  string      `json:"ip_address"`
}

type AuditLogResponse struct {
	ID          string    `json:"id"`
	Action      string    `json:"action"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	UserID      *string   `json:"user_id"`
	UserName    string    `json:"user_name"`
	Description string    `json:"description"`
	Metadata    string    `json:"metadata"`
	IPAddress   string    `json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *AuditService) List(params pagination.Params, filters AuditLogParams) ([]AuditLogResponse, int64, int, int, error) {
	query := s.db.Table("audit_logs").
		Select(`audit_logs.id, audit_logs.action, audit_logs.entity_type, audit_logs.entity_id,
			audit_logs.user_id, COALESCE(users.full_name, '') as user_name,
			audit_logs.description, audit_logs.metadata, audit_logs.ip_address, audit_logs.created_at`).
		Joins("LEFT JOIN users ON users.id = audit_logs.user_id::uuid")

	if filters.Action != "" {
		query = query.Where("audit_logs.action = ?", filters.Action)
	}
	if filters.EntityType != "" {
		query = query.Where("audit_logs.entity_type = ?", filters.EntityType)
	}
	if filters.UserID != "" {
		query = query.Where("audit_logs.user_id = ?", filters.UserID)
	}
	if filters.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", filters.DateFrom); err == nil {
			query = query.Where("audit_logs.created_at >= ?", t)
		}
	}
	if filters.DateTo != "" {
		if t, err := time.Parse("2006-01-02", filters.DateTo); err == nil {
			query = query.Where("audit_logs.created_at <= ?", t.Add(24*time.Hour))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var logs []AuditLogResponse
	params.Sort = "audit_logs.created_at"
	if err := pagination.Apply(query, &params).Find(&logs).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	return logs, total, params.Page, params.PageSize, nil
}

func (s *AuditService) GetByID(id string) (*model.AuditLog, error) {
	var log model.AuditLog
	if err := s.db.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (s *AuditService) Log(req *LogAuditRequest) error {
	metadata := ""
	if req.Metadata != nil {
		metadata = toString(req.Metadata)
	}

	description := buildDescription(req.Action, req.EntityType, req.Metadata)

	log := model.AuditLog{
		Action:      req.Action,
		EntityType:  req.EntityType,
		EntityID:    req.EntityID,
		UserID:      req.UserID,
		Description: description,
		Metadata:    metadata,
		IPAddress:   req.IPAddress,
	}

	return s.db.Create(&log).Error
}

func buildDescription(action, entityType string, metadata interface{}) string {
	entityLabel := entityLabel(entityType)
	actionLabel := actionLabel(action)

	var meta map[string]interface{}
	if metadata != nil {
		if m, ok := metadata.(map[string]interface{}); ok {
			meta = m
		} else if s, ok := metadata.(string); ok {
			json.Unmarshal([]byte(s), &meta)
		}
	}

	switch action {
	case "create":
		name := extractName(meta)
		if name != "" {
			return fmt.Sprintf("%s %s: %s", actionLabel, entityLabel, name)
		}
		return fmt.Sprintf("%s %s", actionLabel, entityLabel)

	case "update":
		changes := extractChanges(meta)
		if changes != "" {
			return fmt.Sprintf("%s %s: %s", actionLabel, entityLabel, changes)
		}
		return fmt.Sprintf("%s %s", actionLabel, entityLabel)

	case "delete":
		name := extractName(meta)
		if name != "" {
			return fmt.Sprintf("%s %s: %s", actionLabel, entityLabel, name)
		}
		return fmt.Sprintf("%s %s", actionLabel, entityLabel)

	case "topup", "refund", "purchase", "pay":
		amount := extractAmount(meta)
		name := extractName(meta)
		if name != "" && amount != "" {
			return fmt.Sprintf("%s %s %s cho %s", actionLabel, amount, entityLabel, name)
		}
		if amount != "" {
			return fmt.Sprintf("%s %s %s", actionLabel, amount, entityLabel)
		}
		return fmt.Sprintf("%s %s", actionLabel, entityLabel)

	case "heartbeat":
		return fmt.Sprintf("Cập nhật trạng thái %s", entityLabel)

	case "start_session":
		return fmt.Sprintf("Bắt đầu phiên chơi %s", entityLabel)

	case "end_session":
		cost := extractAmount(meta)
		if cost != "" {
			return fmt.Sprintf("Kết thúc phiên chơi - phí %s", cost)
		}
		return "Kết thúc phiên chơi"

	case "change_password":
		return "Đổi mật khẩu"

	case "upsert":
		return fmt.Sprintf("Cập nhật %s", entityLabel)

	default:
		if name := extractName(meta); name != "" {
			return fmt.Sprintf("%s %s: %s", actionLabel, entityLabel, name)
		}
		return fmt.Sprintf("%s %s", actionLabel, entityLabel)
	}
}

func entityLabel(entityType string) string {
	labels := map[string]string{
		"product_material": "nguyên liệu sản phẩm",
		"member":           "hội viên",
		"member_group":     "nhóm hội viên",
		"machine":          "máy",
		"machine_group":    "nhóm máy",
		"machine_price":    "giá máy",
		"machine_asset":    "thiết bị máy",
		"combo":            "gói dịch vụ",
		"combo_purchase":   "gói đã mua",
		"machine_session":  "phiên chơi",
		"machine_booking":  "đặt chỗ",
		"promotion":        "khuyến mãi",
		"lucky_spin_log":   "vòng quay",
		"order":            "đơn hàng",
		"product":          "sản phẩm",
		"category":         "danh mục",
		"material":         "nguyên liệu",
		"supplier":         "nhà cung cấp",
		"warehouse":        "kho",
		"stock_transaction": "nhập xuất kho",
		"shift":            "ca làm việc",
		"cash_handover":    "bàn giao tiền",
		"store":            "cửa hàng",
		"printer_config":   "máy in",
		"system_setting":   "cài đặt",
		"backup_log":       "sao lưu",
		"curfew_policy":    "giới hạn tuổi",
		"user":             "người dùng",
		"role":             "vai trò",
		"chat_room": "phòng",
		"chat_message":     "tin nhắn",
	}
	if label, ok := labels[entityType]; ok {
		return label
	}
	return entityType
}

func actionLabel(action string) string {
	labels := map[string]string{
		"create":         "Tạo",
		"update":         "Cập nhật",
		"delete":         "Xóa",
		"topup":          "Nạp tiền",
		"refund":         "Hoàn tiền",
		"purchase":       "Mua",
		"activate":       "Kích hoạt",
		"start_session":  "Bắt đầu phiên",
		"end_session":    "Kết thúc phiên",
		"switch_machine": "Chuyển máy",
		"check_in":       "Check-in",
		"cancel":         "Hủy",
		"no_show":        "Không đến",
		"spin":           "Quay thưởng",
		"apply_reward":   "Nhận thưởng",
		"update_status":  "Cập nhật trạng thái",
		"split":          "Tách đơn",
		"pay":            "Thanh toán",
		"heartbeat":      "Heartbeat",
		"batch_delete":   "Xóa hàng loạt",
		"open_shift":     "Mở ca",
		"close_shift":    "Đóng ca",
		"handover":       "Bàn giao ca",
		"upsert":         "Cập nhật",
		"restore":        "Phục hồi",
		"override":       "Ghi đè",
		"change_password": "Đổi mật khẩu",
		"create_room": "Tạo phòng",
		"send_message":   "Gửi tin nhắn",
		"mark_read":      "Đánh dấu đã đọc",
	}
	if label, ok := labels[action]; ok {
		return label
	}
	return action
}

func extractName(meta map[string]interface{}) string {
	for _, key := range []string{"name", "username", "full_name", "member_name", "combo_name", "machine_code", "machine_name", "order_code"} {
		if v, ok := meta[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func extractAmount(meta map[string]interface{}) string {
	if v, ok := meta["amount"]; ok {
		switch n := v.(type) {
		case float64:
			return fmt.Sprintf("%.0f₫", n)
		case int64:
			return fmt.Sprintf("%d₫", n)
		case int:
			return fmt.Sprintf("%d₫", n)
		case string:
			return n
		}
	}
	return ""
}

func extractChanges(meta map[string]interface{}) string {
	skipKeys := map[string]bool{"updated_at": true, "name": true, "username": true}
	var parts []string
	labelMap := map[string]string{
		"full_name":      "tên",
		"phone":          "SĐT",
		"email":          "email",
		"password_hash":  "mật khẩu",
		"is_active":      "trạng thái",
		"group_id":       "nhóm",
		"notes":          "ghi chú",
		"address":        "địa chỉ",
		"price":          "giá",
		"description":    "mô tả",
		"status":         "trạng thái",
	}
	for k, v := range meta {
		if skipKeys[k] {
			continue
		}
		label := labelMap[k]
		if label == "" {
			label = k
		}
		parts = append(parts, fmt.Sprintf("%s=%v", label, v))
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return ""
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
