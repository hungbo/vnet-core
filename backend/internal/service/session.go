package service

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

type SessionService struct {
	db    *gorm.DB
	hub   *hub.Hub
	audit *AuditService
}

func NewSessionService(db *gorm.DB, wsHub *hub.Hub, audit *AuditService) *SessionService {
	return &SessionService{db: db, hub: wsHub, audit: audit}
}

type StartRequest struct {
	MachineID       string `json:"machine_id" binding:"required"`
	MemberID        string `json:"member_id" binding:"required"`
	ComboPurchaseID string `json:"combo_purchase_id"`
}

type SwitchMachineRequest struct {
	NewMachineID string `json:"new_machine_id" binding:"required"`
}

type EndSessionResponse struct {
	SessionID       string         `json:"session_id"`
	MachineID       string         `json:"machine_id"`
	MachineCode     string         `json:"machine_code"`
	MemberID        string         `json:"member_id"`
	DurationMinutes int            `json:"duration_minutes"`
	TotalCost       int64          `json:"total_cost"`
	BalanceBefore   int64          `json:"balance_before"`
	BalanceAfter    int64          `json:"balance_after"`
	CostBreakdown   *CostBreakdown `json:"cost_breakdown,omitempty"`
}

type SessionDetail struct {
	ID               string     `json:"id"`
	MachineID        string     `json:"machine_id"`
	MachineCode      string     `json:"machine_code"`
	MemberID         *string    `json:"member_id"`
	MemberName       string     `json:"member_name"`
	ComboType        string     `json:"combo_type"`
	ComboID          *string    `json:"combo_id"`
	SlotEnd          *time.Time `json:"slot_end"`
	RemainingMinutes *int       `json:"remaining_minutes"`
	StartedAt        time.Time  `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
	DurationMinutes  *int       `json:"duration_minutes"`
	TotalCost        *int64     `json:"total_cost"`
	IsOvernight      bool       `json:"is_overnight"`
	StoreID          *string    `json:"store_id"`
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
}

type CostItem struct {
	Label  string `json:"label"`
	Amount int64  `json:"amount"`
}

type CostBreakdown struct {
	MachineGroupName    string     `json:"machine_group_name"`
	BasePricePerHour    int64      `json:"base_price_per_hour"`
	DurationMinutes     int        `json:"duration_minutes"`
	BilledMinutes       int        `json:"billed_minutes"`
	GraceMinutes        int        `json:"grace_minutes"`
	TierDiscountPercent float64    `json:"tier_discount_percent"`
	FinalCost           int64      `json:"final_cost"`
	BreakdownItems      []CostItem `json:"breakdown_items"`
}

func (s *SessionService) StartSession(req *StartRequest) (*SessionDetail, error) {
	var machine model.Machine
	if err := s.db.Where("id = ? AND is_active = ?", req.MachineID, true).First(&machine).Error; err != nil {
		return nil, errors.New("machine not found")
	}

	if machine.Status == "in_use" {
		return nil, errors.New("machine is already in use")
	}

	var member model.Member
	if err := s.db.Where("id = ? AND is_active = ?", req.MemberID, true).First(&member).Error; err != nil {
		return nil, errors.New("member not found or inactive")
	}

	tx := s.db.Begin()

	machine.Status = "in_use"
	if err := tx.Save(&machine).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	session := model.MachineSession{
		MachineID: machine.ID,
		MemberID:  &member.ID,
		StartedAt: utils.VietnamTime(),
		IsActive:  true,
		StoreID:   machine.StoreID,
	}

	if req.ComboPurchaseID != "" {
		var purchase model.ComboPurchase
		if err := tx.Where("id = ?", req.ComboPurchaseID).First(&purchase).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("combo purchase not found")
		}

		if !purchase.Activated {
			tx.Rollback()
			return nil, errors.New("combo purchase is not activated")
		}

		var combo model.Combo
		if err := tx.Where("id = ?", purchase.ComboID).First(&combo).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("combo not found")
		}

		session.ComboID = &purchase.ID
		session.ComboType = combo.Type

		if combo.SlotEnd != "" {
			now := utils.VietnamTime()
			parts := strings.Split(combo.SlotEnd, ":")
			if len(parts) >= 2 {
				h := 0
				m := 0
				for _, c := range parts[0] {
					if c >= '0' && c <= '9' {
						h = h*10 + int(c-'0')
					}
				}
				for _, c := range parts[1] {
					if c >= '0' && c <= '9' {
						m = m*10 + int(c-'0')
					}
				}
				slotEnd := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
				session.SlotEnd = &slotEnd
			}
		}

		session.RemainingMinutes = &purchase.RemainingMinutes

		tx.Model(&purchase).Update("current_session_id", session.ID)
	}

	if err := tx.Create(&session).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	resp := toSessionDetail(&session, machine.MachineCode, member.FullName)

	s.audit.Log(&LogAuditRequest{
		Action:     "start_session",
		EntityType: "machine_session",
		EntityID:   session.ID,
		Metadata: map[string]interface{}{
			"machine_id": session.MachineID,
			"member_id":  session.MemberID,
		},
	})

	if s.hub != nil {
		s.hub.Broadcast(hub.Event{
			Type: "session:started",
			Data: map[string]interface{}{
				"session_id":  session.ID,
				"machine_id":  session.MachineID,
				"member_id":   session.MemberID,
				"machine_code": machine.MachineCode,
				"member_name": member.FullName,
				"started_at":  session.StartedAt,
			},
		})
	}

	return resp, nil
}

func (s *SessionService) EndSession(id string) (*EndSessionResponse, error) {
	var session model.MachineSession
	if err := s.db.Where("id = ? AND is_active = ?", id, true).First(&session).Error; err != nil {
		return nil, errors.New("active session not found")
	}

	var machine model.Machine
	if err := s.db.Where("id = ?", session.MachineID).First(&machine).Error; err != nil {
		return nil, errors.New("machine not found")
	}

	now := utils.VietnamTime()
	duration := now.Sub(session.StartedAt)
	durationMinutes := int(duration.Minutes())

	var memberID string
	if session.MemberID != nil {
		memberID = *session.MemberID
	}

	costBreakdown, err := s.CalculateCost(machine.ID, memberID, durationMinutes)
	if err != nil {
		return nil, err
	}

	tx := s.db.Begin()

	session.EndedAt = &now
	session.DurationMinutes = &durationMinutes
	session.TotalCost = &costBreakdown.FinalCost
	session.IsActive = false
	if err := tx.Save(&session).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	machine.Status = "available"
	if err := tx.Save(&machine).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	balanceBefore := int64(0)
	balanceAfter := int64(0)

	if memberID != "" {
		var member model.Member
		if err := tx.Where("id = ?", memberID).First(&member).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("member not found")
		}

		balanceBefore = member.Balance
		member.Balance -= costBreakdown.FinalCost
		balanceAfter = member.Balance
		if err := tx.Save(&member).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		transaction := model.MemberTransaction{
			MemberID:        memberID,
			TransactionType: "session_fee",
			Amount:          -costBreakdown.FinalCost,
			BalanceBefore:   balanceBefore,
			BalanceAfter:    balanceAfter,
			ReferenceID:     &session.ID,
			Description:     "Session fee for " + machine.MachineCode,
			StoreID:         machine.StoreID,
		}
		if err := tx.Create(&transaction).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if session.ComboID != nil {
		var purchase model.ComboPurchase
		if err := tx.Where("id = ?", *session.ComboID).First(&purchase).Error; err == nil {
			remaining := purchase.RemainingMinutes - durationMinutes
			if remaining < 0 {
				remaining = 0
			}
			tx.Model(&purchase).Updates(map[string]interface{}{
				"current_session_id": nil,
				"remaining_minutes":  remaining,
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "end_session",
		EntityType: "machine_session",
		EntityID:   session.ID,
		Metadata: map[string]interface{}{
			"machine_id":     session.MachineID,
			"member_id":      memberID,
			"duration_min":   durationMinutes,
			"total_cost":     costBreakdown.FinalCost,
			"machine_code":   machine.MachineCode,
		},
	})

	if s.hub != nil {
		s.hub.Broadcast(hub.Event{
			Type: "session:ended",
			Data: map[string]interface{}{
				"session_id":   session.ID,
				"machine_id":   session.MachineID,
				"machine_code": machine.MachineCode,
				"member_id":    memberID,
				"duration":     durationMinutes,
				"total_cost":   costBreakdown.FinalCost,
			},
		})
	}

	return &EndSessionResponse{
		SessionID:       session.ID,
		MachineID:       machine.ID,
		MachineCode:     machine.MachineCode,
		MemberID:        memberID,
		DurationMinutes: durationMinutes,
		TotalCost:       costBreakdown.FinalCost,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		CostBreakdown:   costBreakdown,
	}, nil
}

func (s *SessionService) GetSession(id string) (*SessionDetail, error) {
	var session model.MachineSession
	if err := s.db.Where("id = ?", id).First(&session).Error; err != nil {
		return nil, err
	}

	machineCode := ""
	var machine model.Machine
	if err := s.db.Where("id = ?", session.MachineID).First(&machine).Error; err == nil {
		machineCode = machine.MachineCode
	}

	memberName := ""
	if session.MemberID != nil {
		var member model.Member
		if err := s.db.Where("id = ?", *session.MemberID).First(&member).Error; err == nil {
			memberName = member.FullName
		}
	}

	return toSessionDetail(&session, machineCode, memberName), nil
}

func (s *SessionService) SwitchMachine(sessionID, newMachineID string) (*SessionDetail, error) {
	_, err := s.EndSession(sessionID)
	if err != nil {
		return nil, err
	}

	var oldSession model.MachineSession
	if err := s.db.Where("id = ?", sessionID).First(&oldSession).Error; err != nil {
		return nil, err
	}

	startReq := &StartRequest{
		MachineID: newMachineID,
	}
	if oldSession.MemberID != nil {
		startReq.MemberID = *oldSession.MemberID
	} else {
		return nil, errors.New("session has no member")
	}

	if oldSession.ComboID != nil {
		startReq.ComboPurchaseID = *oldSession.ComboID
	}

	newSession, err := s.StartSession(startReq)
	if err != nil {
		return nil, err
	}

	if oldSession.ComboID != nil && oldSession.SlotEnd != nil {
		newSession.SlotEnd = oldSession.SlotEnd
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "switch_machine",
		EntityType: "machine_session",
		EntityID:   sessionID,
		Metadata: map[string]interface{}{
			"old_machine": oldSession.MachineID,
			"new_machine": newMachineID,
			"member_id":   oldSession.MemberID,
		},
	})

	return newSession, nil
}

func (s *SessionService) GetActiveSessions(storeID string) ([]SessionDetail, error) {
	var sessions []model.MachineSession
	query := s.db.Where("is_active = ?", true)
	if storeID != "" {
		query = query.Where("store_id = ?", storeID)
	}
	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}

	machineCache := make(map[string]string)
	memberCache := make(map[string]string)

	var responses []SessionDetail
	for _, session := range sessions {
		machineCode := ""
		if c, ok := machineCache[session.MachineID]; ok {
			machineCode = c
		} else {
			var m model.Machine
			if err := s.db.Where("id = ?", session.MachineID).First(&m).Error; err == nil {
				machineCode = m.MachineCode
				machineCache[session.MachineID] = machineCode
			}
		}

		memberName := ""
		if session.MemberID != nil {
			if n, ok := memberCache[*session.MemberID]; ok {
				memberName = n
			} else {
				var m model.Member
				if err := s.db.Where("id = ?", *session.MemberID).First(&m).Error; err == nil {
					memberName = m.FullName
					memberCache[*session.MemberID] = memberName
				}
			}
		}

		responses = append(responses, *toSessionDetail(&session, machineCode, memberName))
	}

	return responses, nil
}

func (s *SessionService) GetActiveSessionByMember(memberID string) (*SessionDetail, error) {
	var session model.MachineSession
	if err := s.db.Where("member_id = ? AND is_active = ?", memberID, true).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	machineCode := ""
	var m model.Machine
	if err := s.db.Where("id = ?", session.MachineID).First(&m).Error; err == nil {
		machineCode = m.MachineCode
	}

	memberName := ""
	if session.MemberID != nil {
		var mem model.Member
		if err := s.db.Where("id = ?", *session.MemberID).First(&mem).Error; err == nil {
			memberName = mem.FullName
		}
	}

	return toSessionDetail(&session, machineCode, memberName), nil
}

func (s *SessionService) CalculateCost(machineID string, memberID string, durationMinutes int) (*CostBreakdown, error) {
	var machine model.Machine
	if err := s.db.Where("id = ?", machineID).First(&machine).Error; err != nil {
		return nil, errors.New("machine not found")
	}

	var groupName string
	if machine.GroupID != nil {
		var group model.MachineGroup
		if err := s.db.Where("id = ?", *machine.GroupID).First(&group).Error; err == nil {
			groupName = group.Name
		}
	}

	var discountPercent float64
	var memberGroupID string
	if memberID != "" {
		var member model.Member
		if err := s.db.Where("id = ?", memberID).First(&member).Error; err == nil && member.GroupID != nil {
			memberGroupID = *member.GroupID
			var group model.MemberGroup
			if err := s.db.Where("id = ?", *member.GroupID).First(&group).Error; err == nil {
				discountPercent = group.DiscountPercent
			}
		}
	}

	var basePrice int64
	var machinePrice model.MachinePrice
	priceQuery := s.db.Where("(machine_group_id = ? OR machine_group_id IS NULL)", machine.GroupID)
	if memberGroupID != "" {
		priceQuery = priceQuery.Where("(member_group_id = ? OR member_group_id IS NULL)", memberGroupID)
	} else {
		priceQuery = priceQuery.Where("member_group_id IS NULL")
	}
	priceQuery = priceQuery.Where("effective_from <= CURRENT_DATE").
		Where("(effective_to IS NULL OR effective_to >= CURRENT_DATE)").
		Order("member_group_id NULLS LAST, effective_from DESC")
	if err := priceQuery.First(&machinePrice).Error; err == nil {
		basePrice = machinePrice.PricePerHour
	}

	var timeBasedPrice int64
	if machine.GroupID != nil {
		now := utils.VietnamTime()
		currentDay := int(now.Weekday())
		currentTime := now.Format("15:04")
		var timeBased model.TimeBasedPricing
		if err := s.db.Where("machine_group_id = ? AND day_of_week = ? AND start_time <= ? AND end_time > ? AND is_active = ?",
			*machine.GroupID, currentDay, currentTime, currentTime, true).
			First(&timeBased).Error; err == nil {
			timeBasedPrice = timeBased.PricePerHour
		}
	}

	effectivePrice := basePrice
	if timeBasedPrice > 0 {
		effectivePrice = timeBasedPrice
	}

	graceMinutes := 0
	var graceSetting model.SystemSetting
	if err := s.db.Where("group_name = ? AND key = ?", "session", "grace_period_minutes").First(&graceSetting).Error; err == nil {
		graceMinutes = parseIntFromJSON(graceSetting.Value)
	}

	roundingMode := "none"
	var roundSetting model.SystemSetting
	if err := s.db.Where("group_name = ? AND key = ?", "session", "rounding_mode").First(&roundSetting).Error; err == nil {
		roundingMode = strings.Trim(roundSetting.Value, "\"")
	}

	billedMinutes := durationMinutes
	if durationMinutes <= graceMinutes {
		billedMinutes = 0
	} else if graceMinutes > 0 {
		billedMinutes = durationMinutes - graceMinutes
	}

	switch roundingMode {
	case "up_15":
		if billedMinutes > 0 {
			billedMinutes = int(math.Ceil(float64(billedMinutes)/15) * 15)
		}
	case "up_30":
		if billedMinutes > 0 {
			billedMinutes = int(math.Ceil(float64(billedMinutes)/30) * 30)
		}
	case "up_60":
		if billedMinutes > 0 {
			billedMinutes = int(math.Ceil(float64(billedMinutes)/60) * 60)
		}
	case "nearest_15":
		if billedMinutes > 0 {
			billedMinutes = int(math.Round(float64(billedMinutes)/15) * 15)
		}
	case "nearest_30":
		if billedMinutes > 0 {
			billedMinutes = int(math.Round(float64(billedMinutes)/30) * 30)
		}
	case "nearest_60":
		if billedMinutes > 0 {
			billedMinutes = int(math.Round(float64(billedMinutes)/60) * 60)
		}
	}

	cost := int64(0)
	if billedMinutes > 0 {
		cost = int64(math.Ceil(float64(billedMinutes) * float64(effectivePrice) / 60.0))
	}

	discountAmount := int64(0)
	if discountPercent > 0 {
		discountAmount = int64(math.Ceil(float64(cost) * discountPercent / 100.0))
	}

	finalCost := cost - discountAmount

	items := []CostItem{
		{Label: "Base price", Amount: cost},
	}
	if graceMinutes > 0 && billedMinutes < durationMinutes {
		items = append(items, CostItem{Label: "Grace period", Amount: 0})
	}
	if discountAmount > 0 {
		items = append(items, CostItem{Label: "Tier discount", Amount: -discountAmount})
	}

	return &CostBreakdown{
		MachineGroupName:    groupName,
		BasePricePerHour:    effectivePrice,
		DurationMinutes:     durationMinutes,
		BilledMinutes:       billedMinutes,
		GraceMinutes:        graceMinutes,
		TierDiscountPercent: discountPercent,
		FinalCost:           finalCost,
		BreakdownItems:      items,
	}, nil
}

func toSessionDetail(s *model.MachineSession, machineCode string, memberName string) *SessionDetail {
	return &SessionDetail{
		ID:               s.ID,
		MachineID:        s.MachineID,
		MachineCode:      machineCode,
		MemberID:         s.MemberID,
		MemberName:       memberName,
		ComboType:        s.ComboType,
		ComboID:          s.ComboID,
		SlotEnd:          s.SlotEnd,
		RemainingMinutes: s.RemainingMinutes,
		StartedAt:        s.StartedAt,
		EndedAt:          s.EndedAt,
		DurationMinutes:  s.DurationMinutes,
		TotalCost:        s.TotalCost,
		IsOvernight:      s.IsOvernight,
		StoreID:          s.StoreID,
		IsActive:         s.IsActive,
		CreatedAt:        s.CreatedAt,
	}
}

func parseIntFromJSON(val string) int {
	val = strings.TrimSpace(val)
	val = strings.Trim(val, "\"")
	n := 0
	for _, c := range val {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
