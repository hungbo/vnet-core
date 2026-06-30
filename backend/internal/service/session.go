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
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`

	// Snapshot fields (populated when session ends)
	MachineGroupID   *string `json:"machine_group_id,omitempty"`
	MemberGroupID    *string `json:"member_group_id,omitempty"`
	MachineGroupName string  `json:"machine_group_name,omitempty"`
	PricePerHour     int64   `json:"price_per_hour,omitempty"`
	BilledMinutes    int     `json:"billed_minutes,omitempty"`
}

type CostBreakdown struct {
	MachineGroupName string `json:"machine_group_name"`
	PricePerHour     int64  `json:"price_per_hour"`
	DurationMinutes  int    `json:"duration_minutes"`
	BilledMinutes    int    `json:"billed_minutes"`
	FinalCost        int64  `json:"final_cost"`
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

	// Snapshot pricing data at end time for audit
	session.MachineCode = machine.MachineCode
	session.MachineGroupID = machine.GroupID
	session.MachineGroupName = costBreakdown.MachineGroupName
	session.PricePerHour = costBreakdown.PricePerHour
	session.BilledMinutes = costBreakdown.BilledMinutes
	if memberID != "" {
		var member model.Member
		if err := tx.Where("id = ?", memberID).First(&member).Error; err == nil {
			session.MemberGroupID = member.GroupID
		}
	}

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

func (s *SessionService) GetActiveSessions() ([]SessionDetail, error) {
	var sessions []model.MachineSession
	if err := s.db.Where("is_active = ?", true).Find(&sessions).Error; err != nil {
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

	pricePerHour := int64(0)
	groupName := ""
	if machine.GroupID != nil {
		var group model.MachineGroup
		if err := s.db.Where("id = ?", *machine.GroupID).First(&group).Error; err == nil {
			groupName = group.Name
			pricePerHour = group.PricePerHour
		}
	}

	billedMinutes := durationMinutes
	cost := int64(0)
	if billedMinutes > 0 {
		cost = int64(math.Ceil(float64(billedMinutes) * float64(pricePerHour) / 60.0))
	}

	return &CostBreakdown{
		MachineGroupName: groupName,
		PricePerHour:     pricePerHour,
		DurationMinutes:  durationMinutes,
		BilledMinutes:    billedMinutes,
		FinalCost:        cost,
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
		IsActive:         s.IsActive,
		CreatedAt:        s.CreatedAt,

		MachineGroupID:   s.MachineGroupID,
		MemberGroupID:    s.MemberGroupID,
		MachineGroupName: s.MachineGroupName,
		PricePerHour:     s.PricePerHour,
		BilledMinutes:    s.BilledMinutes,
	}
}


