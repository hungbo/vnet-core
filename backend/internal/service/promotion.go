package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
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
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Type        string                     `json:"type"`
	Priority    *int                       `json:"priority"`
	IsActive    *bool                      `json:"is_active"`
	ValidFrom   *string                    `json:"valid_from"`
	ValidTo     *string                    `json:"valid_to"`
	Conditions  []CreatePromotionCondition `json:"conditions"`
	Rewards     []CreatePromotionReward    `json:"rewards"`
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
	MaxPerDay   int     `json:"max_per_day"`
	IsActive    *bool   `json:"is_active"`
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
		query = query.Where("name ILIKE ?", "%"+p.Search+"%")
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

	var validFrom, validTo *time.Time
	if req.ValidFrom != nil {
		t, err := time.Parse(time.RFC3339, *req.ValidFrom)
		if err != nil {
			return nil, errors.New("invalid valid_from format")
		}
		validFrom = &t
	}
	if req.ValidTo != nil {
		t, err := time.Parse(time.RFC3339, *req.ValidTo)
		if err != nil {
			return nil, errors.New("invalid valid_to format")
		}
		validTo = &t
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

	var promo model.Promotion
	if err := s.db.Where("id = ?", id).First(&promo).Error; err != nil {
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
	if req.ValidFrom != nil {
		t, err := time.Parse(time.RFC3339, *req.ValidFrom)
		if err != nil {
			return nil, errors.New("invalid valid_from format")
		}
		updates["valid_from"] = t
	}
	if req.ValidTo != nil {
		t, err := time.Parse(time.RFC3339, *req.ValidTo)
		if err != nil {
			return nil, errors.New("invalid valid_to format")
		}
		updates["valid_to"] = t
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
func (s *PromotionService) validateLuckySpinReward(req *LuckySpinRewardRequest, excludeID string) error {
	if !luckySpinRewardTypes[req.RewardType] {
		return fmt.Errorf("loại phần thưởng %q chưa cấp được; chỉ hỗ trợ bonus_points và balance", req.RewardType)
	}
	if req.Amount <= 0 {
		return errors.New("giá trị phần thưởng phải lớn hơn 0")
	}
	if req.Probability <= 0 || req.Probability > 1 {
		return errors.New("xác suất phải nằm trong khoảng 0 đến 1")
	}
	if req.MaxPerDay < 0 {
		return errors.New("số lượt tối đa mỗi ngày không được âm")
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
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
	if err := s.validateLuckySpinReward(req, ""); err != nil {
		return nil, err
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	reward := model.LuckySpinReward{
		Name:        req.Name,
		RewardType:  req.RewardType,
		RewardValue: fmt.Sprintf(`{"amount": %d}`, req.Amount),
		Probability: req.Probability,
		MaxPerDay:   req.MaxPerDay,
		IsActive:    active,
	}
	if err := s.db.Create(&reward).Error; err != nil {
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
	if err := s.validateLuckySpinReward(req, id); err != nil {
		return nil, err
	}

	active := reward.IsActive
	if req.IsActive != nil {
		active = *req.IsActive
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

func (s *PromotionService) Spin(req *SpinRequest) (*SpinResponse, error) {
	var rewards []model.LuckySpinReward
	if err := s.db.Where("is_active = ?", true).Find(&rewards).Error; err != nil {
		return nil, err
	}

	if len(rewards) == 0 {
		now := time.Now()
		log := model.LuckySpinLog{
			MemberID: req.MemberID,
			IsWin:    false,
			SpunAt:   now,
		}
		s.db.Create(&log)
		s.audit.Log(&LogAuditRequest{
			Action:     "spin",
			EntityType: "lucky_spin_log",
			EntityID:   log.ID,
			Metadata: map[string]interface{}{
				"member_id": req.MemberID,
				"is_win":    false,
			},
		})
		return &SpinResponse{
			IsWin:      false,
			DailySpins: 0,
			MaxPerDay:  0,
		}, nil
	}

	todayStart := time.Now().Truncate(24 * time.Hour)
	var dailyCount int64
	s.db.Model(&model.LuckySpinLog{}).Where("member_id = ? AND spun_at >= ?", req.MemberID, todayStart).Count(&dailyCount)

	maxPerDay := 1
	for _, r := range rewards {
		if r.MaxPerDay > maxPerDay {
			maxPerDay = r.MaxPerDay
		}
	}

	if int(dailyCount) >= maxPerDay && maxPerDay > 0 {
		return nil, errors.New("daily spin limit reached")
	}

	selected := weightedSelect(rewards)

	now := time.Now()
	log := model.LuckySpinLog{
		MemberID: req.MemberID,
		SpunAt:   now,
	}

	if selected != nil {
		log.IsWin = true
		log.RewardID = &selected.ID
	}

	if err := s.db.Create(&log).Error; err != nil {
		return nil, err
	}

	var rewardResp *LuckySpinRewardResponse
	if selected != nil {
		s.applyReward(req.MemberID, selected)
		rewardResp = &LuckySpinRewardResponse{
			ID:          selected.ID,
			Name:        selected.Name,
			RewardType:  selected.RewardType,
			RewardValue: json.RawMessage(selected.RewardValue),
			Probability: selected.Probability,
			MaxPerDay:   selected.MaxPerDay,
			IsActive:    selected.IsActive,
		}
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
	return &SpinResponse{
		IsWin:      selected != nil,
		Reward:     rewardResp,
		DailySpins: int(dailyCount) + 1,
		MaxPerDay:  maxPerDay,
	}, nil
}

func (s *PromotionService) applyReward(memberID string, reward *model.LuckySpinReward) {
	var member model.Member
	if err := s.db.First(&member, "id = ?", memberID).Error; err != nil {
		return
	}

	var valueMap map[string]interface{}
	if err := json.Unmarshal([]byte(reward.RewardValue), &valueMap); err != nil {
		return
	}

	switch reward.RewardType {
	case "bonus_points":
		if amount, ok := valueMap["amount"].(float64); ok {
			trans := model.MemberTransaction{
				MemberID:        memberID,
				TransactionType: "lucky_spin_bonus",
				Amount:          int64(amount),
				BalanceBefore:   member.BonusBalance,
				BalanceAfter:    member.BonusBalance + int64(amount),
				ReferenceID:     &reward.ID,
				Description:     fmt.Sprintf("Lucky spin reward: %s", reward.Name),
				CreatedAt:       time.Now(),
			}
			s.db.Create(&trans)
			s.db.Model(&member).Update("bonus_balance", member.BonusBalance+int64(amount))
			phatSoDuMoi(s.hub, memberID, member.Balance, member.BonusBalance+int64(amount))
			s.audit.Log(&LogAuditRequest{
				Action:     "apply_reward",
				EntityType: "member",
				EntityID:   memberID,
				Metadata: map[string]interface{}{
					"reward_type":      "bonus_points",
					"amount":           int64(amount),
					"balance_before":   member.BonusBalance,
					"balance_after":    member.BonusBalance + int64(amount),
					"transaction_type": "lucky_spin_bonus",
				},
			})
		}
	case "balance":
		if amount, ok := valueMap["amount"].(float64); ok {
			trans := model.MemberTransaction{
				MemberID:        memberID,
				TransactionType: "lucky_spin_balance",
				Amount:          int64(amount),
				BalanceBefore:   member.Balance,
				BalanceAfter:    member.Balance + int64(amount),
				ReferenceID:     &reward.ID,
				Description:     fmt.Sprintf("Lucky spin reward: %s", reward.Name),
				CreatedAt:       time.Now(),
			}
			s.db.Create(&trans)
			s.db.Model(&member).Update("balance", member.Balance+int64(amount))
			phatSoDuMoi(s.hub, memberID, member.Balance+int64(amount), member.BonusBalance)
			s.audit.Log(&LogAuditRequest{
				Action:     "apply_reward",
				EntityType: "member",
				EntityID:   memberID,
				Metadata: map[string]interface{}{
					"reward_type":      "balance",
					"amount":           int64(amount),
					"balance_before":   member.Balance,
					"balance_after":    member.Balance + int64(amount),
					"transaction_type": "lucky_spin_balance",
				},
			})
		}
	case "free_minutes":
		// Hệ thống chưa có ví phút miễn phí để tiêu. Bản cũ cộng số phút vào
		// total_played_hours — vừa sai đơn vị (phút vào cột giờ), vừa sai bản
		// chất (đó là cột thống kê, không phải số dư tiêu được), nên phần
		// thưởng không dùng được mà lại làm hỏng số liệu.
		//
		// Ghi nhận rõ là chưa hỗ trợ, hơn là ghi sai một cách âm thầm.
		if minutes, ok := valueMap["minutes"].(float64); ok {
			s.audit.Log(&LogAuditRequest{
				Action:     "apply_reward_unsupported",
				EntityType: "member",
				EntityID:   memberID,
				Metadata: map[string]interface{}{
					"reward_type": "free_minutes",
					"minutes":     int(minutes),
					"reason":      "chưa có ví phút miễn phí; phần thưởng không được cấp",
				},
			})
		}
	}
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
