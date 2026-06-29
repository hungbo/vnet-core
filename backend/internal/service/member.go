package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MemberService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewMemberService(db *gorm.DB, audit *AuditService) *MemberService {
	return &MemberService{db: db, audit: audit}
}

type CreateMemberRequest struct {
	Username     string     `json:"username"`
	Password     string     `json:"password"`
	FullName     string     `json:"full_name"`
	Phone        string     `json:"phone"`
	Email        string     `json:"email"`
	IDCardNumber string     `json:"id_card_number"`
	DateOfBirth  *time.Time `json:"date_of_birth"`
	GroupID      string     `json:"group_id"`
	StoreID      string     `json:"store_id"`
	Notes        string     `json:"notes"`
}

type UpdateMemberRequest struct {
	FullName     string     `json:"full_name"`
	Phone        string     `json:"phone"`
	Email        string     `json:"email"`
	Password     string     `json:"password"`
	IDCardNumber string     `json:"id_card_number"`
	AvatarURL    string     `json:"avatar_url"`
	DateOfBirth  *time.Time `json:"date_of_birth"`
	GroupID      string     `json:"group_id"`
	Notes        string     `json:"notes"`
	IsActive     *bool      `json:"is_active"`
}

type TopupRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	Description   string `json:"description"`
}

type RefundRequest struct {
	Amount      int64  `json:"amount"`
	IsBonus     bool   `json:"is_bonus"`
	Description string `json:"description"`
}

type CreateGroupRequest struct {
	Name            string  `json:"name"`
	MinSpent        int64   `json:"min_spent"`
	DiscountPercent float64 `json:"discount_percent"`
	IsDefault       bool    `json:"is_default"`
}

type UpdateGroupRequest struct {
	Name            string  `json:"name"`
	MinSpent        int64   `json:"min_spent"`
	DiscountPercent float64 `json:"discount_percent"`
	IsDefault       bool    `json:"is_default"`
}

type MemberResponse struct {
	ID                  string       `json:"id"`
	Username            string       `json:"username"`
	FullName            string       `json:"full_name"`
	Phone               string       `json:"phone"`
	Email               string       `json:"email"`
	IDCardNumber        string       `json:"id_card_number"`
	IDCardImageURL      string       `json:"id_card_image_url"`
	AvatarURL           string       `json:"avatar_url"`
	DateOfBirth         *time.Time   `json:"date_of_birth"`
	Balance             int64        `json:"balance"`
	BonusBalance        int64        `json:"bonus_balance"`
	TotalSpent          int64        `json:"total_spent"`
	TotalPlayedHours    int          `json:"total_played_hours"`
	Group               *GroupResponse `json:"group"`
	GroupID             *string      `json:"group_id"`
	StoreID             *string      `json:"store_id"`
	Notes               string       `json:"notes"`
	ParentConsentFileURL string     `json:"parent_consent_file_url"`
	IsActive            bool         `json:"is_active"`
	LastVisitAt         *time.Time   `json:"last_visit_at"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

type GroupResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	MinSpent        int64     `json:"min_spent"`
	DiscountPercent float64   `json:"discount_percent"`
	IsDefault       bool      `json:"is_default"`
	CreatedAt       time.Time `json:"created_at"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=1"`
}

type MemberTransactionResponse struct {
	ID              string    `json:"id"`
	MemberID        string    `json:"member_id"`
	TransactionType string    `json:"transaction_type"`
	Amount          int64     `json:"amount"`
	BalanceBefore   int64     `json:"balance_before"`
	BalanceAfter    int64     `json:"balance_after"`
	BonusBefore     int64     `json:"bonus_before"`
	BonusAfter      int64     `json:"bonus_after"`
	PaymentMethod   string    `json:"payment_method"`
	ReferenceID     string    `json:"reference_id"`
	Description     string    `json:"description"`
	StoreID         *string   `json:"store_id"`
	CreatedBy       *string   `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

type SessionResponse struct {
	ID               string     `json:"id"`
	MachineID        string     `json:"machine_id"`
	MemberID         *string    `json:"member_id"`
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

func (s *MemberService) loadGroup(groupID *string) *GroupResponse {
	if groupID == nil {
		return nil
	}
	var group model.MemberGroup
	if err := s.db.Where("id = ?", *groupID).First(&group).Error; err != nil {
		return nil
	}
	return &GroupResponse{
		ID:              group.ID,
		Name:            group.Name,
		MinSpent:        group.MinSpent,
		DiscountPercent: group.DiscountPercent,
		CreatedAt:       group.CreatedAt,
	}
}

func toMemberResponse(m *model.Member, groupResp *GroupResponse) *MemberResponse {
	return &MemberResponse{
		ID:                  m.ID,
		Username:            m.Username,
		FullName:            m.FullName,
		Phone:               m.Phone,
		Email:               m.Email,
		IDCardNumber:        m.IDCardNumber,
		IDCardImageURL:      m.IDCardImageURL,
		AvatarURL:           m.AvatarURL,
		DateOfBirth:         m.DateOfBirth,
		Balance:             m.Balance,
		BonusBalance:        m.BonusBalance,
		TotalSpent:          m.TotalSpent,
		TotalPlayedHours:    m.TotalPlayedHours,
		GroupID:             m.GroupID,
		Group:               groupResp,
		StoreID:             m.StoreID,
		Notes:               m.Notes,
		ParentConsentFileURL: m.ParentConsentFileURL,
		IsActive:            m.IsActive,
		LastVisitAt:         m.LastVisitAt,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}

func toGroupResponse(g *model.MemberGroup) *GroupResponse {
	return &GroupResponse{
		ID:              g.ID,
		Name:            g.Name,
		MinSpent:        g.MinSpent,
		DiscountPercent: g.DiscountPercent,
		IsDefault:       g.IsDefault,
		CreatedAt:       g.CreatedAt,
	}
}

func toTransactionResponse(t *model.MemberTransaction) *MemberTransactionResponse {
	return &MemberTransactionResponse{
		ID:              t.ID,
		MemberID:        t.MemberID,
		TransactionType: t.TransactionType,
		Amount:          t.Amount,
		BalanceBefore:   t.BalanceBefore,
		BalanceAfter:    t.BalanceAfter,
		BonusBefore:     t.BonusBefore,
		BonusAfter:      t.BonusAfter,
		PaymentMethod:   t.PaymentMethod,
		ReferenceID:     func() string { if t.ReferenceID != nil { return *t.ReferenceID }; return "" }(),
		Description:     t.Description,
		StoreID:         t.StoreID,
		CreatedBy:       t.CreatedBy,
		CreatedAt:       t.CreatedAt,
	}
}

func toSessionResponse(s *model.MachineSession) *SessionResponse {
	return &SessionResponse{
		ID:               s.ID,
		MachineID:        s.MachineID,
		MemberID:         s.MemberID,
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

func (s *MemberService) List(params pagination.Params, storeID string) ([]*MemberResponse, int64, int, int, error) {
	query := s.db.Model(&model.Member{}).Where("store_id = ?", storeID)

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("full_name ILIKE ? OR phone ILIKE ? OR username ILIKE ?", search, search, search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var members []model.Member
	if err := pagination.Apply(query, &params).Find(&members).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]*MemberResponse, len(members))
	for i := range members {
		groupResp := s.loadGroup(members[i].GroupID)
		result[i] = toMemberResponse(&members[i], groupResp)
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *MemberService) ListByStore(storeID string, params pagination.Params) ([]*MemberResponse, int64, int, int, error) {
	query := s.db.Model(&model.Member{}).Where("store_id = ?", storeID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var members []model.Member
	if err := pagination.Apply(query, &params).Find(&members).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]*MemberResponse, len(members))
	for i := range members {
		groupResp := s.loadGroup(members[i].GroupID)
		result[i] = toMemberResponse(&members[i], groupResp)
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *MemberService) GetByID(id string) (*MemberResponse, error) {
	var member model.Member
	if err := s.db.Where("id = ?", id).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("member not found")
		}
		return nil, err
	}
	groupResp := s.loadGroup(member.GroupID)
	return toMemberResponse(&member, groupResp), nil
}

func (s *MemberService) Create(req *CreateMemberRequest) (*MemberResponse, error) {
	if req.Username == "" {
		return nil, errors.New("username is required")
	}
	if req.Password == "" {
		return nil, errors.New("password is required")
	}

	var existing model.Member
	if err := s.db.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		return nil, errors.New("username already exists")
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	member := model.Member{
		Username:     req.Username,
		PasswordHash: hash,
		FullName:     req.FullName,
		Phone:        req.Phone,
		Email:        req.Email,
		IDCardNumber: req.IDCardNumber,
		DateOfBirth:  req.DateOfBirth,
		Notes:        req.Notes,
		IsActive:     true,
	}

	if req.GroupID != "" {
		member.GroupID = &req.GroupID
	} else {
		var defaultGroup model.MemberGroup
		if err := s.db.Where("is_default = ?", true).First(&defaultGroup).Error; err != nil {
			return nil, errors.New("no default member group found. Please create at least one group first.")
		}
		member.GroupID = &defaultGroup.ID
	}

	if req.StoreID != "" {
		member.StoreID = &req.StoreID
	}

	if err := s.db.Create(&member).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "member",
		EntityID:   member.ID,
		Metadata:   map[string]string{"username": member.Username},
	})

	return s.GetByID(member.ID)
}

func (s *MemberService) Update(id string, req *UpdateMemberRequest) (*MemberResponse, error) {
	var member model.Member
	if err := s.db.Where("id = ?", id).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("member not found")
		}
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.FullName != "" {
		updates["full_name"] = req.FullName
	}
	if req.Phone != "" {
		if !utils.IsValidPhone(req.Phone) {
			return nil, errors.New("invalid phone number")
		}
		updates["phone"] = req.Phone
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		updates["password_hash"] = hash
	}
	if req.IDCardNumber != "" {
		updates["id_card_number"] = req.IDCardNumber
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.DateOfBirth != nil {
		updates["date_of_birth"] = req.DateOfBirth
	}
	if req.GroupID != "" {
		updates["group_id"] = req.GroupID
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := s.db.Model(&member).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "member",
		EntityID:   id,
		Metadata:   updates,
	})

	return s.GetByID(id)
}

func (s *MemberService) Delete(id string) error {
	var member model.Member
	if err := s.db.Where("id = ?", id).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("member not found")
		}
		return err
	}
	if err := s.db.Delete(&member).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "member",
		EntityID:   id,
		Metadata:   map[string]string{"username": member.Username},
	})

	return nil
}

func (s *MemberService) ResetPassword(id string, newPassword string) error {
	var member model.Member
	if err := s.db.Where("id = ?", id).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("member not found")
		}
		return err
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.db.Model(&member).Update("password_hash", hash).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "reset_password",
		EntityType: "member",
		EntityID:   id,
		Metadata:   map[string]string{"username": member.Username},
	})

	return nil
}

func (s *MemberService) Topup(id string, req *TopupRequest, userID string, storeID string) (*MemberResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var member model.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&member).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("member not found")
		}
		return nil, err
	}

	balanceBefore := member.Balance
	bonusBefore := member.BonusBalance
	var balanceAfter int64
	var bonusAfter int64
	var transactionType string

	if req.PaymentMethod == "bonus_balance" {
		bonusAfter = bonusBefore + req.Amount
		balanceAfter = balanceBefore
		transactionType = "topup_bonus"
	} else {
		balanceAfter = balanceBefore + req.Amount
		bonusAfter = bonusBefore
		transactionType = "topup"
	}

	var storeIDPtr *string
	if storeID != "" {
		storeIDPtr = &storeID
	}

	transaction := model.MemberTransaction{
		MemberID:        member.ID,
		TransactionType: transactionType,
		Amount:          req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		BonusBefore:     bonusBefore,
		BonusAfter:      bonusAfter,
		PaymentMethod:   req.PaymentMethod,
		Description:     req.Description,
		StoreID:         storeIDPtr,
		CreatedBy:       &userID,
	}

	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Model(&member).Updates(map[string]interface{}{
		"balance":       balanceAfter,
		"bonus_balance": bonusAfter,
		"updated_at":    time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "topup",
		EntityType: "member",
		EntityID:   id,
		UserID:     &userID,
		Metadata:   map[string]interface{}{"amount": req.Amount, "method": req.PaymentMethod},
	})

	return s.GetByID(id)
}

func (s *MemberService) Refund(id string, req *RefundRequest, userID string, storeID string) (*MemberResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var member model.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&member).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("member not found")
		}
		return nil, err
	}

	balanceBefore := member.Balance
	bonusBefore := member.BonusBalance
	var balanceAfter int64
	var bonusAfter int64
	var transactionType string

	if req.IsBonus {
		if bonusBefore < req.Amount {
			tx.Rollback()
			return nil, errors.New("insufficient bonus balance")
		}
		bonusAfter = bonusBefore - req.Amount
		balanceAfter = balanceBefore
		transactionType = "refund_bonus"
	} else {
		if balanceBefore < req.Amount {
			tx.Rollback()
			return nil, errors.New("insufficient balance")
		}
		balanceAfter = balanceBefore - req.Amount
		bonusAfter = bonusBefore
		transactionType = "refund"
	}

	var storeIDPtr *string
	if storeID != "" {
		storeIDPtr = &storeID
	}

	transaction := model.MemberTransaction{
		MemberID:        member.ID,
		TransactionType: transactionType,
		Amount:          -req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		BonusBefore:     bonusBefore,
		BonusAfter:      bonusAfter,
		Description:     req.Description,
		StoreID:         storeIDPtr,
		CreatedBy:       &userID,
	}

	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Model(&member).Updates(map[string]interface{}{
		"balance":       balanceAfter,
		"bonus_balance": bonusAfter,
		"updated_at":    time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "refund",
		EntityType: "member",
		EntityID:   id,
		UserID:     &userID,
		Metadata:   map[string]interface{}{"amount": req.Amount, "is_bonus": req.IsBonus},
	})

	return s.GetByID(id)
}

func (s *MemberService) GetTransactions(id string, params pagination.Params) ([]*MemberTransactionResponse, int64, int, int, error) {
	query := s.db.Model(&model.MemberTransaction{}).Where("member_id = ?", id)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var transactions []model.MemberTransaction
	if err := pagination.Apply(query, &params).Find(&transactions).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]*MemberTransactionResponse, len(transactions))
	for i := range transactions {
		result[i] = toTransactionResponse(&transactions[i])
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *MemberService) GetSessions(id string, params pagination.Params) ([]*SessionResponse, int64, int, int, error) {
	query := s.db.Model(&model.MachineSession{}).Where("member_id = ?", id)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var sessions []model.MachineSession
	if err := pagination.Apply(query, &params).Find(&sessions).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]*SessionResponse, len(sessions))
	for i := range sessions {
		result[i] = toSessionResponse(&sessions[i])
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *MemberService) GetCombos(id string, params pagination.Params) ([]ComboPurchaseResponse, int64, int, int, error) {
	query := s.db.Model(&model.ComboPurchase{}).Where("member_id = ?", id)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var purchases []model.ComboPurchase
	if err := pagination.Apply(query, &params).Find(&purchases).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]ComboPurchaseResponse, len(purchases))
	for i := range purchases {
		result[i] = purchaseToResponse(purchases[i])
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *MemberService) GetGroups() ([]*GroupResponse, error) {
	var groups []model.MemberGroup
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, err
	}

	result := make([]*GroupResponse, len(groups))
	for i := range groups {
		result[i] = toGroupResponse(&groups[i])
	}

	return result, nil
}

func (s *MemberService) CreateGroup(req *CreateGroupRequest) (*GroupResponse, error) {
	if req.Name == "" {
		return nil, errors.New("group name is required")
	}

	if req.IsDefault {
		s.db.Model(&model.MemberGroup{}).Where("is_default = ?", true).Update("is_default", false)
	}

	group := model.MemberGroup{
		Name:            req.Name,
		MinSpent:        req.MinSpent,
		DiscountPercent: req.DiscountPercent,
		IsDefault:       req.IsDefault,
	}

	if err := s.db.Create(&group).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "member_group",
		EntityID:   group.ID,
		Metadata:   map[string]string{"name": group.Name},
	})

	return toGroupResponse(&group), nil
}

func (s *MemberService) UpdateGroup(id string, req *UpdateGroupRequest) (*GroupResponse, error) {
	var group model.MemberGroup
	if err := s.db.Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("group not found")
		}
		return nil, err
	}

	if req.IsDefault && !group.IsDefault {
		s.db.Model(&model.MemberGroup{}).Where("is_default = ?", true).Update("is_default", false)
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.MinSpent != 0 {
		updates["min_spent"] = req.MinSpent
	}
	if req.DiscountPercent != 0 {
		updates["discount_percent"] = req.DiscountPercent
	}
	if req.IsDefault != group.IsDefault {
		updates["is_default"] = req.IsDefault
	}

	if len(updates) > 0 {
		if err := s.db.Model(&group).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "member_group",
		EntityID:   id,
		Metadata:   updates,
	})

	s.db.Where("id = ?", id).First(&group)
	return toGroupResponse(&group), nil
}

func (s *MemberService) DeleteGroup(id string) error {
	var group model.MemberGroup
	if err := s.db.Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("group not found")
		}
		return err
	}

	var memberCount int64
	s.db.Model(&model.Member{}).Where("group_id = ?", id).Count(&memberCount)
	if memberCount > 0 {
		return errors.New("cannot delete group with active members")
	}

	if group.IsDefault {
		return errors.New("cannot delete the default group")
	}

	if err := s.db.Delete(&group).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "member_group",
		EntityID:   id,
		Metadata:   map[string]string{"name": group.Name},
	})

	return nil
}
