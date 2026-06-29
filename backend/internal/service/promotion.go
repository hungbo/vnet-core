package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type PromotionService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewPromotionService(db *gorm.DB, audit *AuditService) *PromotionService {
	return &PromotionService{db: db, audit: audit}
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
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        string              `json:"type"`
	Priority    int                 `json:"priority"`
	IsActive    bool                `json:"is_active"`
	ValidFrom   *string              `json:"valid_from"`
	ValidTo     *string              `json:"valid_to"`
	Conditions  []PromotionConditionResponse `json:"conditions"`
	Rewards     []PromotionRewardResponse    `json:"rewards"`
	CreatedAt   string              `json:"created_at"`
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
	Name        string                   `json:"name" binding:"required"`
	Description string                   `json:"description"`
	Type        string                   `json:"type" binding:"required"`
	Priority    int                      `json:"priority"`
	IsActive    bool                     `json:"is_active"`
	ValidFrom   *string                  `json:"valid_from"`
	ValidTo     *string                  `json:"valid_to"`
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
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Type        string                   `json:"type"`
	Priority    *int                     `json:"priority"`
	IsActive    *bool                    `json:"is_active"`
	ValidFrom   *string                  `json:"valid_from"`
	ValidTo     *string                  `json:"valid_to"`
	Conditions  []CreatePromotionCondition `json:"conditions"`
	Rewards     []CreatePromotionReward    `json:"rewards"`
}

type LuckySpinRewardResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	RewardType  string          `json:"reward_type"`
	RewardValue json.RawMessage `json:"reward_value"`
	Probability float64         `json:"probability"`
	MaxPerDay   int             `json:"max_per_day"`
	IsActive    bool            `json:"is_active"`
}

type SpinRequest struct {
	MemberID string `json:"member_id" binding:"required"`
}

type SpinResponse struct {
	IsWin    bool                     `json:"is_win"`
	Reward   *LuckySpinRewardResponse `json:"reward,omitempty"`
	DailySpins int                    `json:"daily_spins"`
	MaxPerDay  int                    `json:"max_per_day"`
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
	query := s.db.Where("deleted_at IS NULL")
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
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&promo).Error; err != nil {
		return nil, err
	}
	result := s.promotionToResponse(&promo)
	return &result, nil
}

func (s *PromotionService) Create(req *CreatePromotionRequest) (*PromotionResponse, error) {
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
	var promo model.Promotion
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&promo).Error; err != nil {
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

func (s *PromotionService) Delete(id string) error {
	var promo model.Promotion
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&promo).Error; err != nil {
		return err
	}
	now := time.Now()
	if err := s.db.Model(&promo).Update("deleted_at", &now).Error; err != nil {
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

func (s *PromotionService) GetLuckySpinRewards() ([]LuckySpinRewardResponse, error) {
	var rewards []model.LuckySpinReward
	if err := s.db.Where("is_active = ?", true).Find(&rewards).Error; err != nil {
		return nil, err
	}

	items := make([]LuckySpinRewardResponse, len(rewards))
	for i, r := range rewards {
		items[i] = LuckySpinRewardResponse{
			ID:          r.ID,
			Name:        r.Name,
			RewardType:  r.RewardType,
			RewardValue: json.RawMessage(r.RewardValue),
			Probability: r.Probability,
			MaxPerDay:   r.MaxPerDay,
			IsActive:    r.IsActive,
		}
	}
	return items, nil
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
			IsWin:       false,
			DailySpins:  0,
			MaxPerDay:   0,
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
		IsWin:       selected != nil,
		Reward:      rewardResp,
		DailySpins:  int(dailyCount) + 1,
		MaxPerDay:   maxPerDay,
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
		if minutes, ok := valueMap["minutes"].(float64); ok {
			s.db.Model(&member).Update("total_played_hours", member.TotalPlayedHours+int(minutes))
			s.audit.Log(&LogAuditRequest{
				Action:     "apply_reward",
				EntityType: "member",
				EntityID:   memberID,
				Metadata: map[string]interface{}{
					"reward_type": "free_minutes",
					"minutes":     int(minutes),
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

func weightedSelect(rewards []model.LuckySpinReward) *model.LuckySpinReward {
	totalProb := 0.0
	for _, r := range rewards {
		totalProb += r.Probability
	}

	if totalProb <= 0 {
		return nil
	}

	roll := rand.Float64() * totalProb
	cumulative := 0.0
	for _, r := range rewards {
		cumulative += r.Probability
		if roll < cumulative {
			return &r
		}
	}

	return nil
}
