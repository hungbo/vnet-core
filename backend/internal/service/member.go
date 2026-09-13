package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MemberService struct {
	db    *gorm.DB
	audit *AuditService
	hub   *hub.Hub
}

func NewMemberService(db *gorm.DB, audit *AuditService) *MemberService {
	return &MemberService{db: db, audit: audit}
}

// WithHub nối hub WebSocket vào để nạp tiền, hoàn tiền báo số dư mới cho máy trạm
// ngay lúc nó đổi. Không nối thì mọi thứ vẫn chạy đúng, chỉ là màn hình khách
// giữ số cũ cho tới khi tự tải lại.
//
// Nối rời thay vì thêm tham số cho constructor: giữ nguyên chữ ký thì mọi test
// dựng service không phải sửa, và hub vẫn nil được trong test.
func (s *MemberService) WithHub(h *hub.Hub) *MemberService {
	s.hub = h
	return s
}

type CreateMemberRequest struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	IDCardNumber string `json:"id_card_number"`
	// Hai đường dẫn ảnh dưới đây lưu được nhưng trước nay KHÔNG DTO nào nhận,
	// nên cột luôn rỗng. Giấy đồng ý của phụ huynh gắn thẳng với giới nghiêm:
	// không có nó thì quán không chứng minh được vì sao cho trẻ vị thành niên
	// ngồi máy.
	IDCardImageURL       string         `json:"id_card_image_url"`
	ParentConsentFileURL string         `json:"parent_consent_file_url"`
	DateOfBirth          utils.JSONDate `json:"date_of_birth"`
	GroupID              string         `json:"group_id"`
	Notes                string         `json:"notes"`
}

type UpdateMemberRequest struct {
	FullName             string         `json:"full_name"`
	Phone                string         `json:"phone"`
	Email                string         `json:"email"`
	Password             string         `json:"password"`
	IDCardNumber         string         `json:"id_card_number"`
	IDCardImageURL       string         `json:"id_card_image_url"`
	ParentConsentFileURL string         `json:"parent_consent_file_url"`
	AvatarURL            string         `json:"avatar_url"`
	DateOfBirth          utils.JSONDate `json:"date_of_birth"`
	GroupID              string         `json:"group_id"`
	Notes                string         `json:"notes"`
	IsActive             *bool          `json:"is_active"`
}

type TopupRequest struct {
	Amount int64 `json:"amount"`
	// Chốt ca đếm tiền mặt bằng cách lọc đúng chuỗi "cash". Để trường này tự do
	// thì một giá trị gõ sai làm khoản tiền mặt đó biến mất khỏi số tiền phải có
	// trong két — két thừa mà không ai truy ra vì sao. Danh sách này khớp đúng
	// ô chọn trên giao diện, cộng "bonus_balance" là đường nội bộ để tặng số dư.
	PaymentMethod string `json:"payment_method" binding:"required,oneof=cash transfer ewallet bonus_balance"`
	Description   string `json:"description"`
	// IdempotencyKey do phía gọi sinh ra. Gửi lại cùng một khoá nghĩa là cùng
	// MỘT ý định, không phải hai lần thao tác. Bỏ trống là không tham gia.
	IdempotencyKey string `json:"idempotency_key"`
}

type RefundRequest struct {
	Amount      int64  `json:"amount"`
	IsBonus     bool   `json:"is_bonus"`
	Description string `json:"description"`
	// IdempotencyKey do phía gọi sinh ra. Gửi lại cùng một khoá nghĩa là cùng
	// MỘT ý định, không phải hai lần thao tác. Bỏ trống là không tham gia.
	IdempotencyKey string `json:"idempotency_key"`
}

// Hai mức này chỉ được giao diện kẹp lại, còn API thì nhận tuốt: gọi thẳng
// endpoint tạo được nhóm giảm 150% hoặc -20%, và mức chi tiêu tối thiểu âm.
// Giá phiên chơi có tự kẹp về 100 nên tiền không âm, nhưng bảng quản trị vẫn
// hiện "150%" cho người vận hành đọc, và mọi chỗ dùng về sau đều phải tự nhớ
// kẹp lại. Chặn ngay ở cửa vào.
type CreateGroupRequest struct {
	Name            string  `json:"name"`
	MinSpent        int64   `json:"min_spent" binding:"omitempty,min=0"`
	DiscountPercent float64 `json:"discount_percent" binding:"omitempty,min=0,max=100"`
	IsDefault       bool    `json:"is_default"`
}

type UpdateGroupRequest struct {
	Name            string  `json:"name"`
	MinSpent        int64   `json:"min_spent" binding:"omitempty,min=0"`
	DiscountPercent float64 `json:"discount_percent" binding:"omitempty,min=0,max=100"`
	IsDefault       bool    `json:"is_default"`
}

type MemberResponse struct {
	ID                   string         `json:"id"`
	Username             string         `json:"username"`
	FullName             string         `json:"full_name"`
	Phone                string         `json:"phone"`
	Email                string         `json:"email"`
	IDCardNumber         string         `json:"id_card_number"`
	IDCardImageURL       string         `json:"id_card_image_url"`
	AvatarURL            string         `json:"avatar_url"`
	DateOfBirth          *time.Time     `json:"date_of_birth"`
	Balance              int64          `json:"balance"`
	BonusBalance         int64          `json:"bonus_balance"`
	TotalSpent           int64          `json:"total_spent"`
	TotalPlayedMinutes   int            `json:"total_played_minutes"`
	Group                *GroupResponse `json:"group"`
	GroupID              *string        `json:"group_id"`
	Notes                string         `json:"notes"`
	ParentConsentFileURL string         `json:"parent_consent_file_url"`
	IsActive             bool           `json:"is_active"`
	LastVisitAt          *time.Time     `json:"last_visit_at"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
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
		ID:                   m.ID,
		Username:             m.Username,
		FullName:             m.FullName,
		Phone:                m.Phone,
		Email:                m.Email,
		IDCardNumber:         m.IDCardNumber,
		IDCardImageURL:       m.IDCardImageURL,
		AvatarURL:            m.AvatarURL,
		DateOfBirth:          m.DateOfBirth,
		Balance:              m.Balance,
		BonusBalance:         m.BonusBalance,
		TotalSpent:           m.TotalSpent,
		TotalPlayedMinutes:   m.TotalPlayedMinutes,
		GroupID:              m.GroupID,
		Group:                groupResp,
		Notes:                m.Notes,
		ParentConsentFileURL: m.ParentConsentFileURL,
		IsActive:             m.IsActive,
		LastVisitAt:          m.LastVisitAt,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
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
		ReferenceID: func() string {
			if t.ReferenceID != nil {
				return *t.ReferenceID
			}
			return ""
		}(),
		Description: t.Description,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
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
		IsActive:         s.IsActive,
		CreatedAt:        s.CreatedAt,
	}
}

func (s *MemberService) List(params pagination.Params) ([]*MemberResponse, int64, int, int, error) {
	query := s.db.Model(&model.Member{})

	if params.Search != "" {
		search := "%" + params.Search + "%"
		// Nhân viên quầy gõ không dấu ("Nguyen Van") nhưng họ tên lưu có dấu
		// ("Nguyễn Văn Anh"), nên ILIKE trần không khớp gì cả. Trang Sản phẩm
		// đã bỏ dấu khi tìm (product.go), ở đây thì chưa — cùng một thao tác mà
		// hai trang cho kết quả khác nhau. Số điện thoại không có dấu nên giữ nguyên.
		query = query.Where(
			"unaccent(full_name) ILIKE unaccent(?) OR phone ILIKE ? OR unaccent(username) ILIKE unaccent(?)",
			search, search, search,
		)
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

func (s *MemberService) GetByID(id string) (*MemberResponse, error) {
	var member model.Member
	if err := s.db.Where("id = ?", id).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy hội viên")
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
		return nil, errors.New("phải nhập mật khẩu")
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
		Username:             req.Username,
		PasswordHash:         hash,
		FullName:             req.FullName,
		Phone:                req.Phone,
		Email:                req.Email,
		IDCardNumber:         req.IDCardNumber,
		IDCardImageURL:       req.IDCardImageURL,
		ParentConsentFileURL: req.ParentConsentFileURL,
		DateOfBirth:          req.DateOfBirth.Time,
		Notes:                req.Notes,
		IsActive:             true,
	}

	if req.GroupID != "" {
		member.GroupID = &req.GroupID
	} else {
		var defaultGroup model.MemberGroup
		if err := s.db.Where("is_default = ?", true).First(&defaultGroup).Error; err != nil {
			return nil, errors.New("chưa có hạng hội viên mặc định; hãy tạo ít nhất một hạng trước")
		}
		member.GroupID = &defaultGroup.ID
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
			return nil, errors.New("không tìm thấy hội viên")
		}
		return nil, err
	}

	updates := map[string]interface{}{}

	if req.FullName != "" {
		updates["full_name"] = req.FullName
	}
	if req.Phone != "" {
		if !utils.IsValidPhone(req.Phone) {
			return nil, errors.New("số điện thoại không hợp lệ")
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
	if req.IDCardImageURL != "" {
		updates["id_card_image_url"] = req.IDCardImageURL
	}
	if req.ParentConsentFileURL != "" {
		updates["parent_consent_file_url"] = req.ParentConsentFileURL
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.DateOfBirth.Time != nil {
		updates["date_of_birth"] = req.DateOfBirth.Time
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

// Delete xoá một hội viên.
//
// Giao dịch số dư, đơn hàng, phiên chơi, lượt mua combo đều là chứng từ tiền:
// xoá hội viên mà bỏ lại chúng thì báo cáo doanh thu và đối soát ca trỏ tới một
// người không còn tồn tại. Hội viên đã tiêu tiền thì chỉ khoá được (is_active),
// không xoá.
//
// lucky_spin_logs, member_notifications, member_attendances KHÔNG chặn: nhật ký
// và hộp thư, không phải chứng từ. topup_cards.used_by/sold_to cũng không —
// tấm thẻ đã dùng vẫn tự giữ mệnh giá và thời điểm.
func (s *MemberService) Delete(id string) error {
	var member model.Member
	if err := s.db.Where("id = ?", id).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy hội viên")
		}
		return err
	}
	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.MemberTransaction{}, Cot: "member_id", Nhan: "giao dịch số dư"},
		{Bang: &model.MachineSession{}, Cot: "member_id", Nhan: "phiên chơi"},
		{Bang: &model.Order{}, Cot: "member_id", Nhan: "đơn hàng"},
		{Bang: &model.ComboPurchase{}, Cot: "member_id", Nhan: "lượt mua combo"},
		{Bang: &model.MachineBooking{}, Cot: "member_id", Nhan: "lịch đặt máy"},
	}, "hãy khoá tài khoản thay vì xoá — lịch sử tiêu dùng và số dư phải giữ lại"); err != nil {
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
			return errors.New("không tìm thấy hội viên")
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

func (s *MemberService) Topup(id string, req *TopupRequest, userID string) (*MemberResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("số tiền phải lớn hơn 0")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := giuKhoaIdempotency(tx, "member.topup", req.IdempotencyKey); err != nil {
		tx.Rollback()
		return nil, err
	}

	var member model.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&member).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy hội viên")
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

	// Nhân viên nạp ở quầy, khách đang nhìn màn hình máy trạm. Không có hai
	// dòng này thì con số trên máy khách đứng yên cho tới lần tự tải lại — mà
	// khách chưa vào phiên thì không có nhịp nào để mà tải lại.
	phatSoDuMoi(s.hub, id, balanceAfter, bonusAfter)
	phatNapTien(s.hub, id, req.Amount)

	return s.GetByID(id)
}

func (s *MemberService) Refund(id string, req *RefundRequest, userID string) (*MemberResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("số tiền phải lớn hơn 0")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := giuKhoaIdempotency(tx, "member.refund", req.IdempotencyKey); err != nil {
		tx.Rollback()
		return nil, err
	}

	var member model.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&member).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy hội viên")
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
			return nil, errors.New("số dư điểm thưởng không đủ")
		}
		bonusAfter = bonusBefore - req.Amount
		balanceAfter = balanceBefore
		transactionType = "refund_bonus"
	} else {
		if balanceBefore < req.Amount {
			tx.Rollback()
			return nil, errors.New("số dư không đủ")
		}
		balanceAfter = balanceBefore - req.Amount
		bonusAfter = bonusBefore
		transactionType = "refund"
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

	// Rút tiền ra thì không bắn toast "nạp tiền" — chỉ vẽ lại số dư.
	phatSoDuMoi(s.hub, id, balanceAfter, bonusAfter)

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

	comboIDs := make([]string, len(purchases))
	for i, p := range purchases {
		comboIDs[i] = p.ComboID
	}
	comboMap := make(map[string]string, len(comboIDs))
	if len(comboIDs) > 0 {
		var combos []model.Combo
		s.db.Where("id IN ?", comboIDs).Find(&combos)
		for _, c := range combos {
			comboMap[c.ID] = c.Name
		}
	}

	result := make([]ComboPurchaseResponse, len(purchases))
	for i := range purchases {
		result[i] = purchaseToResponse(purchases[i], comboMap[purchases[i].ComboID])
	}

	return result, total, params.Page, params.PageSize, nil
}

// RefreshTiers xếp lại hạng hội viên theo tổng chi tiêu.
//
// Chạy nền mỗi 15 phút, nhưng quán còn cần chạy NGAY sau khi sửa ngưỡng chi
// tiêu của một hạng — nếu không thì mọi người vẫn ở hạng cũ tới 15 phút, và
// nhân viên không có cách nào biết hàm này có chạy hay không.
//
// Trả về số hội viên đã đổi hạng.
func (s *MemberService) RefreshTiers() (int, error) {
	var groups []model.MemberGroup
	if err := s.db.Order("min_spent DESC").Find(&groups).Error; err != nil {
		return 0, err
	}
	if len(groups) == 0 {
		return 0, nil
	}

	moved := 0
	for _, g := range groups {
		// Ngưỡng cao nhất trước, để mỗi người rơi vào hạng tốt nhất họ đủ điều
		// kiện và hạng thấp hơn không giành lại được sau đó.
		res := s.db.Model(&model.Member{}).
			Where("total_spent >= ?", g.MinSpent).
			Where("group_id IS NULL OR group_id <> ?", g.ID).
			Where("group_id IS NULL OR group_id NOT IN (?)",
				s.db.Model(&model.MemberGroup{}).Select("id").Where("min_spent > ?", g.MinSpent)).
			Update("group_id", g.ID)
		if res.Error != nil {
			return moved, res.Error
		}
		moved += int(res.RowsAffected)
	}

	if moved > 0 {
		s.audit.Log(&LogAuditRequest{
			Action:     "refresh_tiers",
			EntityType: "member",
			Metadata:   map[string]interface{}{"moved": moved},
		})
	}
	return moved, nil
}

// GetGroups lọc theo tên nhóm khi có từ khoá.
//
// Trang quản trị vẫn gửi ?search= từ trước, nhưng hàm này bỏ qua hẳn: gõ gì vào
// ô tìm kiếm cũng ra đủ danh sách, không lỗi, không dấu hiệu gì — đúng kiểu nút
// bấm vào không làm gì. Bỏ dấu khi so khớp cho giống ô tìm hội viên.
func (s *MemberService) GetGroups(search string) ([]*GroupResponse, error) {
	query := s.db.Model(&model.MemberGroup{})
	if search != "" {
		query = query.Where("unaccent(name) ILIKE unaccent(?)", "%"+search+"%")
	}

	var groups []model.MemberGroup
	if err := query.Find(&groups).Error; err != nil {
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
		return nil, errors.New("phải nhập tên nhóm")
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
			return nil, errors.New("không tìm thấy nhóm")
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
			return errors.New("không tìm thấy nhóm")
		}
		return err
	}

	// Nhóm mặc định kiểm TRƯỚC: hội viên mới rơi vào đó, xoá là hỏng luồng tạo
	// tài khoản, và lý do đó không sửa được bằng cách chuyển hội viên đi.
	if group.IsDefault {
		return chanVi("không xoá được nhóm mặc định — hãy đặt nhóm khác làm mặc định trước")
	}

	// Bản cũ đếm hội viên nhưng KHÔNG kiểm lỗi của Count: database hỏng thì đọc
	// ra 0 và nhóm bị xoá kèm theo. Và nó bỏ sót bảng giá theo hạng.
	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.Member{}, Cot: "group_id", Nhan: "hội viên"},
		{Bang: &model.MachinePrice{}, Cot: "member_group_id", Nhan: "dòng bảng giá theo hạng"},
	}, "hãy chuyển họ sang nhóm khác trước"); err != nil {
		return err
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
