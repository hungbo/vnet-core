package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PromotionService struct {
	db    *gorm.DB
	audit *AuditService
	hub   *hub.Hub
}

func NewPromotionService(db *gorm.DB, audit *AuditService) *PromotionService {
	return &PromotionService{db: db, audit: audit}
}

// WithHub nối hub WebSocket vào để thưởng khuyến mãi báo số dư mới cho máy trạm
// ngay lúc nó đổi. Không nối thì mọi thứ vẫn chạy đúng, chỉ là màn hình khách
// giữ số cũ cho tới khi tự tải lại.
//
// Nối rời thay vì thêm tham số cho constructor: giữ nguyên chữ ký thì mọi test
// dựng service không phải sửa, và hub vẫn nil được trong test.
func (s *PromotionService) WithHub(h *hub.Hub) *PromotionService {
	s.hub = h
	return s
}

type PromotionListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`
	Search   string `form:"search"`
	Type     string `form:"type"`
	IsActive *bool  `form:"is_active"`
}

type PromotionResponse struct {
	ID          string                       `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Type        string                       `json:"type"`
	Priority    int                          `json:"priority"`
	IsActive    bool                         `json:"is_active"`
	ValidFrom   *string                      `json:"valid_from"`
	ValidTo     *string                      `json:"valid_to"`
	Conditions  []PromotionConditionResponse `json:"conditions"`
	Rewards     []PromotionRewardResponse    `json:"rewards"`
	CreatedAt   string                       `json:"created_at"`
}

type PromotionConditionResponse struct {
	ID             string          `json:"id"`
	ConditionKey   string          `json:"condition_key"`
	ConditionValue json.RawMessage `json:"condition_value"`
}

type PromotionRewardResponse struct {
	ID          string          `json:"id"`
	RewardType  string          `json:"reward_type"`
	RewardValue json.RawMessage `json:"reward_value"`
}

type CreatePromotionRequest struct {
	Name        string                     `json:"name" binding:"required"`
	Description string                     `json:"description"`
	Type        string                     `json:"type" binding:"required"`
	Priority    int                        `json:"priority"`
	IsActive    bool                       `json:"is_active"`
	ValidFrom   *string                    `json:"valid_from"`
	ValidTo     *string                    `json:"valid_to"`
	Conditions  []CreatePromotionCondition `json:"conditions"`
	Rewards     []CreatePromotionReward    `json:"rewards"`
}

type CreatePromotionCondition struct {
	ConditionKey   string          `json:"condition_key" binding:"required"`
	ConditionValue json.RawMessage `json:"condition_value" binding:"required"`
}

type CreatePromotionReward struct {
	RewardType  string          `json:"reward_type" binding:"required"`
	RewardValue json.RawMessage `json:"reward_value" binding:"required"`
}

type UpdatePromotionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Priority    *int   `json:"priority"`
	IsActive    *bool  `json:"is_active"`
	// RawMessage thay vì *string để phân biệt "không gửi" (giữ nguyên) với
	// null hoặc "" (xoá mốc ngày). Với *string cả hai đều ra nil, nên một khuyến
	// mãi đã đặt ngày hết hạn không bao giờ bỏ hạn được nữa.
	ValidFrom  json.RawMessage            `json:"valid_from" swaggertype:"string"`
	ValidTo    json.RawMessage            `json:"valid_to" swaggertype:"string"`
	Conditions []CreatePromotionCondition `json:"conditions"`
	Rewards    []CreatePromotionReward    `json:"rewards"`
}

// PromotionTypes là các loại khuyến mãi trang quản trị cho chọn. Bộ máy không
// đọc cột type (mức giảm do phần thưởng quyết định) — đây là nhãn phân loại —
// nhưng ô nhập tự do cũ cho lưu mọi chuỗi gõ sai, nên lọc theo loại không khớp.
var PromotionTypes = []string{"percentage", "fixed", "combo"}

// validatePromotionHeader kiểm phần "vỏ" của khuyến mãi. Chạy trước khi chạm DB
// để lỗi trả về là câu tiếng Việt rõ ràng, không phải lỗi PostgreSQL thô.
func validatePromotionHeader(typ string, priority int, validFrom, validTo *time.Time) error {
	if typ != "" && !containsString(PromotionTypes, typ) {
		return fmt.Errorf("loại khuyến mãi %q không hợp lệ (chấp nhận: %s)", typ, strings.Join(PromotionTypes, ", "))
	}
	if priority < 0 {
		return errors.New("độ ưu tiên không được âm")
	}
	if validFrom != nil && validTo != nil && validFrom.After(*validTo) {
		return errors.New("ngày bắt đầu hiệu lực phải trước hoặc bằng ngày hết hiệu lực")
	}
	return nil
}

// parseNgayHieuLuc đọc một mốc ngày trong payload cập nhật.
// present=false: không gửi trường này. present=true, t=nil: xoá mốc ngày.
func parseNgayHieuLuc(raw json.RawMessage, tenTruong string) (t *time.Time, present bool, err error) {
	if raw == nil {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return nil, true, fmt.Errorf("%s không đúng định dạng", tenTruong)
	}
	if strings.TrimSpace(str) == "" {
		return nil, true, nil
	}
	v, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return nil, true, fmt.Errorf("%s không đúng định dạng", tenTruong)
	}
	return &v, true, nil
}

type LuckySpinRewardResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	RewardType  string          `json:"reward_type"`
	RewardValue json.RawMessage `json:"reward_value"`
	// Amount là con số bên trong RewardValue, bóc sẵn cho giao diện. Bắt màn
	// hình tự đọc jsonb là cách chắc chắn sinh ra ô trống khi khoá đổi tên.
	Amount      int64   `json:"amount"`
	Probability float64 `json:"probability"`
	MaxPerDay   int     `json:"max_per_day"`
	IsActive    bool    `json:"is_active"`
}

// Chỉ hai loại này thực sự cấp được phần thưởng (applyReward). free_minutes
// chưa có ví phút để tiêu nên bị từ chối ngay lúc tạo, thay vì để quán cấu hình
// một ô thưởng không bao giờ trả gì.
var luckySpinRewardTypes = map[string]bool{
	"bonus_points": true,
	"balance":      true,
}

type LuckySpinRewardRequest struct {
	Name       string `json:"name" binding:"required"`
	RewardType string `json:"reward_type" binding:"required"`
	// Amount thay cho jsonb thô: cả hai loại được hỗ trợ đều tiêu đúng một số.
	Amount      int64   `json:"amount"`
	Probability float64 `json:"probability"`
	// MaxPerDay khai trên từng ô nhưng được dùng như giới hạn lượt quay MỖI
	// HỘI VIÊN MỖI NGÀY của cả bàn quay: Spin lấy số lớn nhất trong các ô đang
	// bật, tối thiểu 1. Xem spinLimit.
	MaxPerDay int   `json:"max_per_day"`
	IsActive  *bool `json:"is_active"`
}

type SpinRequest struct {
	MemberID string `json:"member_id" binding:"required"`
}

type SpinResponse struct {
	IsWin      bool                     `json:"is_win"`
	Reward     *LuckySpinRewardResponse `json:"reward,omitempty"`
	DailySpins int                      `json:"daily_spins"`
	MaxPerDay  int                      `json:"max_per_day"`
}

func (s *PromotionService) List(req *PromotionListRequest) (*pagination.Result, error) {
	p := &pagination.Params{
		Page:     req.Page,
		PageSize: req.PageSize,
		Sort:     req.Sort,
		Order:    req.Order,
		Search:   req.Search,
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.Sort == "" {
		p.Sort = "created_at"
	}
	if p.Order == "" {
		p.Order = "desc"
	}

	var promotions []model.Promotion
	query := s.db
	if p.Search != "" {
		// Bỏ dấu: tên khuyến mãi là chữ tiếng Việt có dấu ("Giảm 10% cuối tuần")
		// còn nhân viên gõ không dấu. Cùng khuôn với các ô tìm đã sửa trước đó.
		query = query.Where("unaccent(name) ILIKE unaccent(?)", "%"+p.Search+"%")
	}
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.IsActive != nil {
		query = query.Where("is_active = ?", *req.IsActive)
	}

	var total int64
	query.Model(&model.Promotion{}).Count(&total)

	if err := pagination.Apply(query, p).Find(&promotions).Error; err != nil {
		return nil, err
	}

	items := make([]PromotionResponse, len(promotions))
	for i, promo := range promotions {
		items[i] = s.promotionToResponse(&promo)
	}

	return pagination.NewResult(items, total, p), nil
}

func (s *PromotionService) GetByID(id string) (*PromotionResponse, error) {
	var promo model.Promotion
	if err := s.db.Where("id = ?", id).First(&promo).Error; err != nil {
		return nil, err
	}
	result := s.promotionToResponse(&promo)
	return &result, nil
}

func (s *PromotionService) Create(req *CreatePromotionRequest) (*PromotionResponse, error) {
	// Khoá gõ sai phải lộ ra ngay: một điều kiện không hiểu được sẽ khiến bộ
	// máy bỏ qua khuyến mãi, và người tạo không có cách nào biết vì sao.
	if err := validateRuleRequest(req.Conditions, req.Rewards); err != nil {
		return nil, err
	}

	if len(req.Rewards) == 0 {
		return nil, errors.New("khuyến mãi phải có ít nhất một phần thưởng")
	}
	if req.Type == "" {
		return nil, errors.New("vui lòng chọn loại khuyến mãi")
	}

	var validFrom, validTo *time.Time
	// "" coi như không đặt mốc, giống null — ô ngày bị xoá trên giao diện gửi "".
	if req.ValidFrom != nil && strings.TrimSpace(*req.ValidFrom) != "" {
		t, err := time.Parse(time.RFC3339, *req.ValidFrom)
		if err != nil {
			return nil, errors.New("ngày bắt đầu hiệu lực không đúng định dạng")
		}
		validFrom = &t
	}
	if req.ValidTo != nil && strings.TrimSpace(*req.ValidTo) != "" {
		t, err := time.Parse(time.RFC3339, *req.ValidTo)
		if err != nil {
			return nil, errors.New("ngày hết hiệu lực không đúng định dạng")
		}
		validTo = &t
	}
	if err := validatePromotionHeader(req.Type, req.Priority, validFrom, validTo); err != nil {
		return nil, err
	}

	promo := model.Promotion{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Priority:    req.Priority,
		IsActive:    req.IsActive,
		ValidFrom:   validFrom,
		ValidTo:     validTo,
	}

	tx := s.db.Begin()

	if err := tx.Create(&promo).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	// Cột is_active có `default:true`, và GORM (1.25) thay giá trị zero của cột
	// có default bằng chính default đó khi INSERT — Select cũng không cứu được.
	// Vì vậy "tạo ở trạng thái tắt" luôn ra bật. Ghi lại false ngay trong cùng
	// giao dịch, để không có khoảnh khắc nào khuyến mãi tắt lại đang chạy.
	if !req.IsActive {
		if err := tx.Model(&promo).Update("is_active", false).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		promo.IsActive = false
	}

	for _, c := range req.Conditions {
		cond := model.PromotionCondition{
			PromotionID:    promo.ID,
			ConditionKey:   c.ConditionKey,
			ConditionValue: string(c.ConditionValue),
		}
		if err := tx.Create(&cond).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	for _, r := range req.Rewards {
		reward := model.PromotionReward{
			PromotionID: promo.ID,
			RewardType:  r.RewardType,
			RewardValue: string(r.RewardValue),
		}
		if err := tx.Create(&reward).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	tx.Commit()

	result := s.promotionToResponse(&promo)
	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "promotion",
		EntityID:   promo.ID,
		Metadata: map[string]interface{}{
			"name": promo.Name,
			"type": promo.Type,
		},
	})
	return &result, nil
}

func (s *PromotionService) Update(id string, req *UpdatePromotionRequest) (*PromotionResponse, error) {
	// Khoá gõ sai phải lộ ra ngay: một điều kiện không hiểu được sẽ khiến bộ
	// máy bỏ qua khuyến mãi, và người tạo không có cách nào biết vì sao.
	if err := validateRuleRequest(req.Conditions, req.Rewards); err != nil {
		return nil, err
	}

	if req.Rewards != nil && len(req.Rewards) == 0 {
		return nil, errors.New("khuyến mãi phải có ít nhất một phần thưởng")
	}

	var promo model.Promotion
	if err := s.db.Where("id = ?", id).First(&promo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy khuyến mãi")
		}
		return nil, err
	}

	validFrom, fromSet, err := parseNgayHieuLuc(req.ValidFrom, "ngày bắt đầu hiệu lực")
	if err != nil {
		return nil, err
	}
	validTo, toSet, err := parseNgayHieuLuc(req.ValidTo, "ngày hết hiệu lực")
	if err != nil {
		return nil, err
	}
	// So khoảng ngày SAU khi gộp với giá trị đang lưu: chỉ sửa một đầu vẫn có
	// thể làm đầu kia nằm sai phía.
	effFrom, effTo := promo.ValidFrom, promo.ValidTo
	if fromSet {
		effFrom = validFrom
	}
	if toSet {
		effTo = validTo
	}
	// Loại chỉ kiểm khi đổi: khuyến mãi cũ mang nhãn ngoài danh sách vẫn sửa
	// được các trường khác mà không bị chặn.
	typeToCheck := ""
	if req.Type != "" && req.Type != promo.Type {
		typeToCheck = req.Type
	}
	priority := 0
	if req.Priority != nil {
		priority = *req.Priority
	}
	if err := validatePromotionHeader(typeToCheck, priority, effFrom, effTo); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	// Map giữ được nil, nên null thật sự ghi NULL xuống cột.
	if fromSet {
		updates["valid_from"] = validFrom
	}
	if toSet {
		updates["valid_to"] = validTo
	}

	tx := s.db.Begin()

	if len(updates) > 0 {
		if err := tx.Model(&promo).Updates(updates).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if req.Conditions != nil {
		tx.Where("promotion_id = ?", id).Delete(&model.PromotionCondition{})
		for _, c := range req.Conditions {
			cond := model.PromotionCondition{
				PromotionID:    id,
				ConditionKey:   c.ConditionKey,
				ConditionValue: string(c.ConditionValue),
			}
			if err := tx.Create(&cond).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	if req.Rewards != nil {
		tx.Where("promotion_id = ?", id).Delete(&model.PromotionReward{})
		for _, r := range req.Rewards {
			reward := model.PromotionReward{
				PromotionID: id,
				RewardType:  r.RewardType,
				RewardValue: string(r.RewardValue),
			}
			if err := tx.Create(&reward).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	tx.Commit()

	s.db.First(&promo, "id = ?", id)
	result := s.promotionToResponse(&promo)
	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "promotion",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"name": promo.Name,
			"type": promo.Type,
		},
	})
	return &result, nil
}

// Delete xoá một chương trình khuyến mãi cùng điều kiện và phần thưởng của nó.
//
// Điều kiện và phần thưởng là con SỞ HỮU — Update (cùng file) đã xoá rồi tạo lại
// chúng theo đúng lối này. Còn orders.promotion_id thì chặn: báo cáo hiệu quả
// khuyến mãi đọc cột đó, xoá chương trình là báo cáo cũ mất tên.
func (s *PromotionService) Delete(id string) error {
	var promo model.Promotion
	if err := s.db.Where("id = ?", id).First(&promo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy khuyến mãi")
		}
		return err
	}
	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.Order{}, Cot: "promotion_id", Nhan: "đơn hàng đã áp dụng"},
	}, "hãy tắt khuyến mãi (bỏ đang chạy) thay vì xoá"); err != nil {
		return err
	}
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("promotion_id = ?", id).Delete(&model.PromotionCondition{}).Error; err != nil {
			return err
		}
		if err := tx.Where("promotion_id = ?", id).Delete(&model.PromotionReward{}).Error; err != nil {
			return err
		}
		return tx.Model(&promo).Update("deleted_at", &now).Error
	}); err != nil {
		return err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "promotion",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"name": promo.Name,
			"type": promo.Type,
		},
	})
	return nil
}

// GetLuckySpinRewards liệt kê ô thưởng. includeInactive dành cho màn quản trị:
// ô đã tắt vẫn phải nhìn thấy để bật lại, còn vòng quay chỉ lấy ô đang bật.
func (s *PromotionService) GetLuckySpinRewards(includeInactive bool) ([]LuckySpinRewardResponse, error) {
	var rewards []model.LuckySpinReward
	q := s.db.Order("created_at")
	if !includeInactive {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&rewards).Error; err != nil {
		return nil, err
	}

	items := make([]LuckySpinRewardResponse, len(rewards))
	for i, r := range rewards {
		items[i] = luckySpinToResponse(r)
	}
	return items, nil
}

func luckySpinToResponse(r model.LuckySpinReward) LuckySpinRewardResponse {
	return LuckySpinRewardResponse{
		ID:          r.ID,
		Name:        r.Name,
		RewardType:  r.RewardType,
		RewardValue: json.RawMessage(r.RewardValue),
		Amount:      luckySpinAmount(r.RewardValue),
		Probability: r.Probability,
		MaxPerDay:   r.MaxPerDay,
		IsActive:    r.IsActive,
	}
}

func luckySpinAmount(rewardValue string) int64 {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rewardValue), &m); err != nil {
		return 0
	}
	if v, ok := m["amount"].(float64); ok {
		return int64(v)
	}
	return 0
}

// validateLuckySpinReward kiểm cả dòng lẫn tổng xác suất của bàn quay.
//
// excludeID để trống khi tạo mới, và mang ID của chính dòng đang sửa khi cập
// nhật — nếu không, sửa một dòng sẽ tự tính trùng chính nó và luôn báo vượt.
// active là trạng thái SAU khi lưu: bật một ô đang tắt phải kiểm lại tổng.
func (s *PromotionService) validateLuckySpinReward(req *LuckySpinRewardRequest, excludeID string, active bool) error {
	if !luckySpinRewardTypes[req.RewardType] {
		return fmt.Errorf("loại phần thưởng %q chưa cấp được; chỉ hỗ trợ bonus_points và balance", req.RewardType)
	}
	if req.Amount <= 0 {
		return errors.New("giá trị phần thưởng phải lớn hơn 0")
	}
	// Cột probability là decimal(5,4): dưới 0.0001 (0,01%) bị làm tròn về 0 khi
	// lưu, tức một ô "đang bật" nhưng không bao giờ trúng.
	if req.Probability < 0.0001 || req.Probability > 1 {
		return errors.New("xác suất phải từ 0,01% đến 100%")
	}
	if req.MaxPerDay < 0 {
		return errors.New("số lượt tối đa mỗi ngày không được âm")
	}

	if !active {
		return nil
	}

	// Tổng xác suất các ô đang bật không được vượt 1: phần còn thiếu chính là
	// tỉ lệ quay trượt. Cho vượt 1 thì weightedSelect phải chuẩn hoá lại và ô
	// hiếm bỗng trúng thường xuyên hơn con số quán đã đặt.
	var rows []model.LuckySpinReward
	q := s.db.Where("is_active = ?", true)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	total := req.Probability
	for _, r := range rows {
		total += r.Probability
	}
	if total > 1.0001 { // biên nhỏ cho sai số dấu phẩy động
		// Nói bằng phần trăm: giao diện nhập theo %, báo lỗi bằng phân số
		// bắt người dùng tự quy đổi mới hiểu con số vừa nhập sai ở đâu.
		return fmt.Errorf("tổng xác suất các ô đang bật là %.2f%%, vượt quá 100%%", total*100)
	}
	return nil
}

func (s *PromotionService) CreateLuckySpinReward(req *LuckySpinRewardRequest) (*LuckySpinRewardResponse, error) {
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	if err := s.validateLuckySpinReward(req, "", active); err != nil {
		return nil, err
	}

	reward := model.LuckySpinReward{
		Name:        req.Name,
		RewardType:  req.RewardType,
		RewardValue: fmt.Sprintf(`{"amount": %d}`, req.Amount),
		Probability: req.Probability,
		MaxPerDay:   req.MaxPerDay,
		IsActive:    active,
	}
	// Cùng bẫy với Promotion.IsActive: GORM thay false bằng `default:true` khi
	// INSERT, nên ô "tạo ở trạng thái tắt" ra bật — và lọt qua kiểm tổng xác
	// suất vừa bỏ qua cho ô tắt. Ghi lại false trong cùng giao dịch.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&reward).Error; err != nil {
			return err
		}
		if !active {
			if err := tx.Model(&reward).Update("is_active", false).Error; err != nil {
				return err
			}
			reward.IsActive = false
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "lucky_spin_reward",
		EntityID:   reward.ID,
		Metadata: map[string]interface{}{
			"name":        reward.Name,
			"reward_type": reward.RewardType,
			"amount":      req.Amount,
			"probability": reward.Probability,
		},
	})

	resp := luckySpinToResponse(reward)
	return &resp, nil
}

func (s *PromotionService) UpdateLuckySpinReward(id string, req *LuckySpinRewardRequest) (*LuckySpinRewardResponse, error) {
	var reward model.LuckySpinReward
	if err := s.db.Where("id = ?", id).First(&reward).Error; err != nil {
		return nil, errors.New("không tìm thấy ô thưởng")
	}
	active := reward.IsActive
	if req.IsActive != nil {
		active = *req.IsActive
	}
	if err := s.validateLuckySpinReward(req, id, active); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"name":         req.Name,
		"reward_type":  req.RewardType,
		"reward_value": fmt.Sprintf(`{"amount": %d}`, req.Amount),
		"probability":  req.Probability,
		"max_per_day":  req.MaxPerDay,
		"is_active":    active,
	}
	if err := s.db.Model(&reward).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "lucky_spin_reward",
		EntityID:   reward.ID,
		Metadata:   updates,
	})

	s.db.Where("id = ?", id).First(&reward)
	resp := luckySpinToResponse(reward)
	return &resp, nil
}

func (s *PromotionService) DeleteLuckySpinReward(id string) error {
	var reward model.LuckySpinReward
	if err := s.db.Where("id = ?", id).First(&reward).Error; err != nil {
		return errors.New("không tìm thấy ô thưởng")
	}
	// LuckySpinLog.RewardID là ON DELETE SET NULL nên lịch sử quay vẫn còn,
	// chỉ mất tên ô thưởng. Xoá hẳn là đúng: bảng này là cấu hình, không phải
	// chứng từ.
	if err := s.db.Delete(&reward).Error; err != nil {
		return err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "lucky_spin_reward",
		EntityID:   id,
		Metadata:   map[string]interface{}{"name": reward.Name},
	})
	return nil
}

// ErrSpinMemberNotFound: hội viên không tồn tại hoặc đã xoá. Handler trả 404.
var ErrSpinMemberNotFound = errors.New("không tìm thấy hội viên")

// uuidHoiVien chặn member_id không phải UUID trước khi chạm DB — nếu không,
// lỗi thô "invalid input syntax for type uuid" của PostgreSQL lọt ra người dùng.
var uuidHoiVien = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// spinLimit là số lượt quay mỗi hội viên mỗi ngày của cả bàn quay.
//
// max_per_day được khai trên TỪNG Ô nhưng không có nghĩa "ô này trúng tối đa
// N lần/ngày": giới hạn tính chung cho hội viên, bằng số lớn nhất trong các ô
// đang bật, tối thiểu 1 (để 0 ở mọi ô nghĩa là 1 lượt/ngày, không phải vô hạn).
func spinLimit(rewards []model.LuckySpinReward) int {
	limit := 1
	for _, r := range rewards {
		if r.MaxPerDay > limit {
			limit = r.MaxPerDay
		}
	}
	return limit
}

// Spin quay cho một hội viên thật: ghi lịch sử và cộng thưởng.
//
// Cả lượt quay chạy trong MỘT giao dịch, khoá dòng hội viên (FOR UPDATE) ngay
// đầu. Bản cũ đếm lượt, ghi log và cộng tiền bằng các câu rời rạc: hai lượt
// bấm cùng lúc đều đếm thấy 0 lượt nên vượt max_per_day, rồi cùng ghi
// balance = <số dư đọc lúc trước> + thưởng — một lượt thưởng mất, và còn ghi
// đè cả tiền phiên chơi vừa trừ song song. Giữ khoá thì lượt thứ hai phải chờ,
// đếm lại thấy lượt thứ nhất, và số dư trước/sau trong sổ là số dư thật.
func (s *PromotionService) Spin(req *SpinRequest) (*SpinResponse, error) {
	if !uuidHoiVien.MatchString(req.MemberID) {
		return nil, errors.New("mã hội viên không hợp lệ")
	}

	var (
		selected   *model.LuckySpinReward
		dailyCount int64
		maxPerDay  int
		log        model.LuckySpinLog
		member     model.Member
		rewardLog  *LogAuditRequest
	)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.MemberID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSpinMemberNotFound
			}
			return err
		}
		if !member.IsActive {
			return errors.New("tài khoản hội viên đang bị khoá, không quay được")
		}

		var rewards []model.LuckySpinReward
		if err := tx.Where("is_active = ?", true).Find(&rewards).Error; err != nil {
			return err
		}

		if len(rewards) > 0 {
			maxPerDay = spinLimit(rewards)
			// "Hôm nay" theo giờ quán: Truncate(24h) cắt theo UTC, nên ngày
			// quay ở Việt Nam bắt đầu lúc 7 giờ sáng.
			todayStart := utils.StartOfDay(time.Now())
			if err := tx.Model(&model.LuckySpinLog{}).
				Where("member_id = ? AND spun_at >= ?", req.MemberID, todayStart).
				Count(&dailyCount).Error; err != nil {
				return err
			}
			if int(dailyCount) >= maxPerDay {
				return errors.New("đã hết lượt quay trong ngày")
			}
			selected = weightedSelect(rewards)
		}

		log = model.LuckySpinLog{MemberID: req.MemberID, SpunAt: time.Now()}
		if selected != nil {
			log.IsWin = true
			log.RewardID = &selected.ID
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		if selected != nil {
			var err error
			rewardLog, err = applyReward(tx, &member, selected)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var rewardResp *LuckySpinRewardResponse
	if selected != nil {
		resp := luckySpinToResponse(*selected)
		rewardResp = &resp
		// Báo số dư sau khi commit: báo trong giao dịch thì máy khách có thể
		// nhận số mới rồi giao dịch lại rollback.
		phatSoDuMoi(s.hub, member.ID, member.Balance, member.BonusBalance)
	}
	// Nhật ký ghi sau commit, qua kết nối riêng: ghi trong giao dịch thì một
	// lượt quay bị rollback vẫn để lại dòng "đã cộng thưởng".
	if rewardLog != nil {
		s.audit.Log(rewardLog)
	}

	spinMetadata := map[string]interface{}{
		"member_id": req.MemberID,
		"is_win":    selected != nil,
	}
	if selected != nil {
		spinMetadata["reward_id"] = selected.ID
		spinMetadata["reward_name"] = selected.Name
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "spin",
		EntityType: "lucky_spin_log",
		EntityID:   log.ID,
		Metadata:   spinMetadata,
	})
	resp := &SpinResponse{
		IsWin:     selected != nil,
		Reward:    rewardResp,
		MaxPerDay: maxPerDay,
	}
	if maxPerDay > 0 {
		resp.DailySpins = int(dailyCount) + 1
	}
	return resp, nil
}

// SimulateSpin quay thử để kiểm cấu hình: chọn ô theo đúng xác suất của Spin
// nhưng KHÔNG ghi lịch sử, KHÔNG cộng tiền cho ai. Nút "Quay thử" trên trang
// quản trị từng gọi Spin thật cho hội viên mới nhất — mỗi lần nhân viên bấm
// thử là một khách lạ được cộng tiền thật.
func (s *PromotionService) SimulateSpin() (*SpinResponse, error) {
	var rewards []model.LuckySpinReward
	if err := s.db.Where("is_active = ?", true).Find(&rewards).Error; err != nil {
		return nil, err
	}
	resp := &SpinResponse{}
	if len(rewards) == 0 {
		return resp, nil
	}
	resp.MaxPerDay = spinLimit(rewards)
	if selected := weightedSelect(rewards); selected != nil {
		r := luckySpinToResponse(*selected)
		resp.IsWin = true
		resp.Reward = &r
	}
	return resp, nil
}

// applyReward cộng thưởng cho hội viên ĐÃ ĐƯỢC KHOÁ trong tx.
//
// member là dòng đọc dưới FOR UPDATE nên số dư trước/sau ghi vào sổ là số
// thật; cột vẫn được cập nhật bằng biểu thức (balance + x) chứ không ghi một
// con số tuyệt đối, để không bao giờ đè lên thay đổi nào khác. member được
// cập nhật tại chỗ để bên gọi báo số dư mới sau khi commit. Trả về dòng nhật
// ký để bên gọi ghi SAU khi commit.
func applyReward(tx *gorm.DB, member *model.Member, reward *model.LuckySpinReward) (*LogAuditRequest, error) {
	var valueMap map[string]interface{}
	if err := json.Unmarshal([]byte(reward.RewardValue), &valueMap); err != nil {
		return nil, nil
	}

	switch reward.RewardType {
	case "bonus_points", "balance":
		amountF, ok := valueMap["amount"].(float64)
		if !ok {
			return nil, nil
		}
		amount := int64(amountF)
		col, txType := "bonus_balance", "lucky_spin_bonus"
		before := member.BonusBalance
		if reward.RewardType == "balance" {
			col, txType = "balance", "lucky_spin_balance"
			before = member.Balance
		}
		after := before + amount
		trans := model.MemberTransaction{
			MemberID:        member.ID,
			TransactionType: txType,
			Amount:          amount,
			BalanceBefore:   before,
			BalanceAfter:    after,
			ReferenceID:     &reward.ID,
			Description:     fmt.Sprintf("Lucky spin reward: %s", reward.Name),
			CreatedAt:       time.Now(),
		}
		if err := tx.Create(&trans).Error; err != nil {
			return nil, err
		}
		if err := tx.Model(&model.Member{}).Where("id = ?", member.ID).
			Update(col, gorm.Expr(col+" + ?", amount)).Error; err != nil {
			return nil, err
		}
		if col == "balance" {
			member.Balance = after
		} else {
			member.BonusBalance = after
		}
		return &LogAuditRequest{
			Action:     "apply_reward",
			EntityType: "member",
			EntityID:   member.ID,
			Metadata: map[string]interface{}{
				"reward_type":      reward.RewardType,
				"amount":           amount,
				"balance_before":   before,
				"balance_after":    after,
				"transaction_type": txType,
			},
		}, nil
	case "free_minutes":
		// Hệ thống chưa có ví phút miễn phí để tiêu. Bản cũ cộng số phút vào
		// total_played_hours — vừa sai đơn vị (phút vào cột giờ), vừa sai bản
		// chất (đó là cột thống kê, không phải số dư tiêu được), nên phần
		// thưởng không dùng được mà lại làm hỏng số liệu.
		//
		// Ghi nhận rõ là chưa hỗ trợ, hơn là ghi sai một cách âm thầm.
		if minutes, ok := valueMap["minutes"].(float64); ok {
			return &LogAuditRequest{
				Action:     "apply_reward_unsupported",
				EntityType: "member",
				EntityID:   member.ID,
				Metadata: map[string]interface{}{
					"reward_type": "free_minutes",
					"minutes":     int(minutes),
					"reason":      "chưa có ví phút miễn phí; phần thưởng không được cấp",
				},
			}, nil
		}
	}
	return nil, nil
}

func (s *PromotionService) promotionToResponse(promo *model.Promotion) PromotionResponse {
	resp := PromotionResponse{
		ID:          promo.ID,
		Name:        promo.Name,
		Description: promo.Description,
		Type:        promo.Type,
		Priority:    promo.Priority,
		IsActive:    promo.IsActive,
		CreatedAt:   promo.CreatedAt.Format(time.RFC3339),
	}

	if promo.ValidFrom != nil {
		s := promo.ValidFrom.Format(time.RFC3339)
		resp.ValidFrom = &s
	}
	if promo.ValidTo != nil {
		s := promo.ValidTo.Format(time.RFC3339)
		resp.ValidTo = &s
	}

	var conditions []model.PromotionCondition
	s.db.Where("promotion_id = ?", promo.ID).Find(&conditions)
	resp.Conditions = make([]PromotionConditionResponse, len(conditions))
	for i, c := range conditions {
		resp.Conditions[i] = PromotionConditionResponse{
			ID:             c.ID,
			ConditionKey:   c.ConditionKey,
			ConditionValue: json.RawMessage(c.ConditionValue),
		}
	}

	var rewards []model.PromotionReward
	s.db.Where("promotion_id = ?", promo.ID).Find(&rewards)
	resp.Rewards = make([]PromotionRewardResponse, len(rewards))
	for i, r := range rewards {
		resp.Rewards[i] = PromotionRewardResponse{
			ID:          r.ID,
			RewardType:  r.RewardType,
			RewardValue: json.RawMessage(r.RewardValue),
		}
	}

	return resp
}

// weightedSelect chọn ô thưởng theo xác suất tuyệt đối, KHÔNG chuẩn hoá.
//
// Bản cũ chia cho tổng xác suất, nên một bàn quay đặt "iPhone 1%" mà chỉ có
// đúng ô đó thì lượt nào cũng trúng iPhone: phần xác suất còn lại (99%) đáng
// lẽ là quay trượt đã bị chuẩn hoá mất, và nhánh `selected == nil` cùng cột
// LuckySpinLog.IsWin trở thành mã chết. CreateLuckySpinReward đã chặn tổng
// vượt 1, nên phần thiếu ở đây chính là tỉ lệ trượt quán đã cố ý để lại.
func weightedSelect(rewards []model.LuckySpinReward) *model.LuckySpinReward {
	roll := rand.Float64()
	cumulative := 0.0
	for _, r := range rewards {
		cumulative += r.Probability
		if roll < cumulative {
			return &r
		}
	}
	return nil
}

// PreviewForOrder cho phép giao diện xem trước mức giảm trước khi tạo đơn.
func (s *PromotionService) PreviewForOrder(pctx PromotionContext) *AppliedPromotion {
	return BestPromotionForOrder(s.db, pctx)
}
