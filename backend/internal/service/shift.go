package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Lỗi có kiểu riêng để handler trả đúng mã HTTP (404/403) thay vì gộp hết vào 400.
var (
	ErrShiftNotFound  = errors.New("không tìm thấy ca")
	ErrShiftForbidden = errors.New("chỉ người mở ca hoặc chủ quán mới được thao tác trên ca này")
	errShiftClosed    = errors.New("ca đã đóng")
)

// uuidHopLe chặn id không phải UUID trước khi chạm DB. Không có nó, PostgreSQL
// trả lỗi thô "invalid input syntax for type uuid" và lỗi đó lọt nguyên văn ra
// giao diện (GET /shifts/current, đóng ca với id gõ tay...).
var uuidHopLe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ShiftActor là người đang gọi API, lấy từ token ở handler.
type ShiftActor struct {
	UserID string
	// IsOwner: quyền "*" (chủ quán) — được đóng ca và ghi thu/chi trên ca của
	// bất kỳ ai, ví dụ khi nhân viên về mà quên đóng ca.
	IsOwner bool
}

func (a ShiftActor) coTheThaoTac(shift *model.Shift) bool {
	return a.IsOwner || (a.UserID != "" && a.UserID == shift.UserID)
}

type ShiftService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewShiftService(db *gorm.DB, audit *AuditService) *ShiftService {
	return &ShiftService{db: db, audit: audit}
}

type ShiftResponse struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	// Cột "Người dùng" trên trang Ca làm việc đọc trường này. Không trả về thì
	// bảng hiện UUID thô — nhân viên không đọc được ai đã trực ca nào.
	UserName       string  `json:"user_name"`
	StartedAt      string  `json:"started_at"`
	EndedAt        *string `json:"ended_at"`
	Status         string  `json:"status"`
	OpeningBalance int64   `json:"opening_balance"`
	ClosingBalance *int64  `json:"closing_balance"`
	ExpectedTotal  *int64  `json:"expected_total"`
	Discrepancy    *int64  `json:"discrepancy"`
	Notes          string  `json:"notes"`
	CreatedAt      string  `json:"created_at"`
}

type OpenShiftRequest struct {
	OpeningBalance int64  `json:"opening_balance" binding:"gte=0"`
	Notes          string `json:"notes"`
}

type CloseShiftRequest struct {
	// Đóng ca với két rỗng là hợp lệ, nên 0 phải được chấp nhận — vì thế là con
	// trỏ: thiếu hẳn trường thì phải báo lỗi chứ không ngầm hiểu là két rỗng.
	ClosingBalance *int64 `json:"closing_balance" binding:"required,gte=0"`
	Notes          string `json:"notes"`
}

type HandoverRequest struct {
	Amount       int64  `json:"amount" binding:"gt=0"`
	HandoverType string `json:"handover_type" binding:"required,oneof=cash_in cash_out"`
	// Bắt buộc: tiền rút khỏi két không lý do thì cuối ca không ai giải thích
	// được khoản lệch.
	Reason string `json:"reason" binding:"required"`
}

// CashHandoverResponse là một lần thu/chi tiền mặt ngoài bán hàng trong ca.
type CashHandoverResponse struct {
	ID           string `json:"id"`
	ShiftID      string `json:"shift_id"`
	Amount       int64  `json:"amount"`
	HandoverType string `json:"handover_type"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
}

// userNames nạp tên nhân viên cho cả trang một lần thay vì mỗi dòng một truy vấn.
// Unscoped: nhân viên nghỉ việc rồi thì ca trực cũ vẫn phải hiện tên.
func (s *ShiftService) userNames(shifts []model.Shift) map[string]string {
	ids := make([]string, 0, len(shifts))
	for _, sh := range shifts {
		ids = append(ids, sh.UserID)
	}
	out := map[string]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []struct{ ID, Username, FullName string }
	s.db.Unscoped().Model(&model.User{}).Select("id, username, full_name").
		Where("id IN ?", ids).Find(&rows)
	for _, r := range rows {
		if r.FullName != "" {
			out[r.ID] = r.FullName
		} else {
			out[r.ID] = r.Username
		}
	}
	return out
}

func (s *ShiftService) userName(userID string) string {
	return s.userNames([]model.Shift{{UserID: userID}})[userID]
}

// findShift nạp ca theo id; khoá dòng khi lock = true (trong giao dịch) để
// đóng ca và ghi thu/chi không chen nhau.
func findShift(db *gorm.DB, id string, lock bool) (*model.Shift, error) {
	if !uuidHopLe.MatchString(id) {
		return nil, ErrShiftNotFound
	}
	q := db
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var shift model.Shift
	if err := q.Where("id = ?", id).First(&shift).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShiftNotFound
		}
		return nil, err
	}
	return &shift, nil
}

// List lọc theo trạng thái (open/closed, rỗng = tất cả — handler đã kiểm giá
// trị) và theo tên nhân viên.
func (s *ShiftService) List(params *pagination.Params, status string) (*pagination.Result, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.Sort == "" {
		params.Sort = "started_at"
	}
	if params.Order == "" {
		params.Order = "desc"
	}

	var shifts []model.Shift
	query := s.db
	if params.Search != "" {
		// Ô tìm kiếm là chữ tự do (tên hoặc tài khoản nhân viên). Trước đây so
		// thẳng với cột uuid user_id nên mọi lần gõ đều thành lỗi 500. Không lọc
		// deleted_at: ca của nhân viên đã nghỉ vẫn phải tìm ra.
		like := "%" + params.Search + "%"
		query = query.Where("user_id IN (SELECT id FROM users WHERE unaccent(username) ILIKE unaccent(?) OR unaccent(full_name) ILIKE unaccent(?))", like, like)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Model(&model.Shift{}).Count(&total)

	if err := pagination.Apply(query, params).Find(&shifts).Error; err != nil {
		return nil, err
	}

	items := make([]ShiftResponse, len(shifts))
	names := s.userNames(shifts)
	for i, sh := range shifts {
		items[i] = shiftToResponse(sh)
		items[i].UserName = names[sh.UserID]
	}

	return pagination.NewResult(items, total, params), nil
}

func (s *ShiftService) GetByID(id string) (*ShiftResponse, error) {
	shift, err := findShift(s.db, id, false)
	if err != nil {
		return nil, err
	}

	result := shiftToResponse(*shift)
	result.UserName = s.userName(shift.UserID)
	return &result, nil
}

// loiDangCoCaMo nêu tên người đang giữ két để nhân viên biết phải tìm ai.
func (s *ShiftService) loiDangCoCaMo(open *model.Shift) error {
	name := s.userName(open.UserID)
	if name == "" {
		name = "nhân viên khác"
	}
	return fmt.Errorf("đang có ca mở của %s — đóng ca đó trước khi mở ca mới", name)
}

// OpenShift mở ca mới. Quán chỉ có MỘT két tiền, nên cả quán chỉ được có một ca
// mở tại một thời điểm (không phải mỗi người một ca): hai ca chồng nhau sẽ cùng
// tính một khoản tiền mặt vào tiền dự kiến của cả hai. Người kế tiếp phải đóng
// (bàn giao) ca hiện tại rồi mới mở ca mới.
func (s *ShiftService) OpenShift(req *OpenShiftRequest, userID string) (*ShiftResponse, error) {
	if req.OpeningBalance < 0 {
		return nil, errors.New("tiền đầu ca không được âm")
	}

	var activeShift model.Shift
	err := s.db.Where("status = ?", "open").First(&activeShift).Error
	if err == nil {
		return nil, s.loiDangCoCaMo(&activeShift)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now()

	shift := model.Shift{
		UserID:         userID,
		StartedAt:      now,
		Status:         "open",
		OpeningBalance: req.OpeningBalance,
		Notes:          req.Notes,
	}

	if err := s.db.Create(&shift).Error; err != nil {
		// Hai người bấm mở ca cùng lúc đều qua được bước kiểm tra trên; chỉ chỉ
		// mục duy nhất uniq_shift_open trong DB mới chặn được người thứ hai.
		if isUniqueViolation(err) {
			if e := s.db.Where("status = ?", "open").First(&activeShift).Error; e == nil {
				return nil, s.loiDangCoCaMo(&activeShift)
			}
			return nil, errors.New("đang có ca mở — đóng ca đó trước khi mở ca mới")
		}
		return nil, err
	}

	result := shiftToResponse(shift)
	result.UserName = s.userName(userID)

	s.audit.Log(&LogAuditRequest{
		Action:     "open_shift",
		EntityType: "shift",
		EntityID:   shift.ID,
		UserID:     &userID,
		Metadata:   map[string]interface{}{"opening_cash": req.OpeningBalance},
	})

	return &result, nil
}

// expectedCash is what the drawer should hold on top of the opening float:
// only money that physically entered or left it during the shift.
//
// Balance-settled orders and session fees never touch the drawer, so they are
// deliberately excluded — the previous implementation summed every order with
// status "paid", a status this codebase never assigns, so the expected total
// was always zero and every shift reconciled against nothing.
//
// Orders carry no shift_id, so the window is the shift's own time range. That
// attribution is exact because the shop allows only one open shift at a time
// (one cash drawer; enforced by the uniq_shift_open index), so shift windows
// never overlap and no cash is counted into two shifts.
func expectedCash(db *gorm.DB, shift *model.Shift, until time.Time) (int64, error) {
	var cashPayments int64
	if err := db.Model(&model.Payment{}).
		Where("payment_method = ? AND status = ? AND paid_at >= ? AND paid_at <= ?",
			"cash", "completed", shift.StartedAt, until).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&cashPayments).Error; err != nil {
		return 0, err
	}

	// Top-ups add cash; combo purchases paid in cash do too, and are stored as
	// a negative ledger amount, hence the sign flip.
	var cashTopups int64
	if err := db.Model(&model.MemberTransaction{}).
		Where("payment_method = ? AND transaction_type IN ? AND created_at >= ? AND created_at <= ?",
			"cash", []string{"topup", "topup_bonus"}, shift.StartedAt, until).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&cashTopups).Error; err != nil {
		return 0, err
	}

	var cashCombos int64
	if err := db.Model(&model.MemberTransaction{}).
		Where("payment_method = ? AND transaction_type = ? AND created_at >= ? AND created_at <= ?",
			"cash", "combo_purchase", shift.StartedAt, until).
		Select("COALESCE(SUM(-amount), 0)").
		Scan(&cashCombos).Error; err != nil {
		return 0, err
	}

	var handoverIn int64
	if err := db.Model(&model.CashHandover{}).
		Where("shift_id = ? AND handover_type = ?", shift.ID, "cash_in").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&handoverIn).Error; err != nil {
		return 0, err
	}

	var handoverOut int64
	if err := db.Model(&model.CashHandover{}).
		Where("shift_id = ? AND handover_type = ?", shift.ID, "cash_out").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&handoverOut).Error; err != nil {
		return 0, err
	}

	// Hoàn tiền hội viên (MemberService.Refund) trừ số dư và ghi amount ÂM, không
	// có payment_method. Giả định: quán luôn trả lại khách bằng TIỀN MẶT lấy từ
	// két, nên khoản này làm két vơi đi. Chỉ tính "refund" (số dư thật khách đã
	// nạp); "refund_bonus" là điểm thưởng, không có tiền nào rời két.
	var cashRefunds int64
	if err := db.Model(&model.MemberTransaction{}).
		Where("transaction_type = ? AND created_at >= ? AND created_at <= ?",
			"refund", shift.StartedAt, until).
		Select("COALESCE(SUM(-amount), 0)").
		Scan(&cashRefunds).Error; err != nil {
		return 0, err
	}

	return cashPayments + cashTopups + cashCombos + handoverIn - handoverOut - cashRefunds, nil
}

func (s *ShiftService) CloseShift(id string, req *CloseShiftRequest, actor ShiftActor) (*ShiftResponse, error) {
	if req.ClosingBalance == nil {
		return nil, errors.New("nhập tiền cuối ca")
	}
	if *req.ClosingBalance < 0 {
		return nil, errors.New("tiền cuối ca không được âm")
	}
	closingBalance := *req.ClosingBalance

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Khoá dòng ca: một lần ghi chi tiền chen vào giữa lúc tính tiền dự kiến và
	// lúc ghi kết quả sẽ làm khoản lệch sai.
	shift, err := findShift(tx, id, true)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if shift.Status != "open" {
		tx.Rollback()
		return nil, errShiftClosed
	}
	if !actor.coTheThaoTac(shift) {
		tx.Rollback()
		return nil, ErrShiftForbidden
	}

	now := time.Now()
	expectedTotal, err := expectedCash(tx, shift, now)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	discrepancy := closingBalance - (shift.OpeningBalance + expectedTotal)

	// Chỉ nối ghi chú khi có — trước đây luôn thêm "\n" nên ghi chú cứ dài ra
	// bằng các dòng trống.
	notes := shift.Notes
	if extra := strings.TrimSpace(req.Notes); extra != "" {
		if notes != "" {
			notes += "\n"
		}
		notes += extra
	}

	updates := map[string]interface{}{
		"status":          "closed",
		"ended_at":        now,
		"closing_balance": closingBalance,
		"expected_total":  expectedTotal,
		"discrepancy":     discrepancy,
		"notes":           notes,
	}

	// Điều kiện status = 'open' làm việc đóng ca thành nguyên tử: hai lần bấm
	// đóng cùng lúc thì lần sau không ghi đè số liệu của lần trước.
	res := tx.Model(&model.Shift{}).Where("id = ? AND status = ?", shift.ID, "open").Updates(updates)
	if res.Error != nil {
		tx.Rollback()
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return nil, errShiftClosed
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// Dựng phản hồi từ đúng các giá trị vừa ghi, không đọc lại: tránh cặp tiền
	// cuối/khoản lệch lệch nhau với thứ đã lưu.
	shift.Status = "closed"
	shift.EndedAt = &now
	shift.ClosingBalance = &closingBalance
	shift.ExpectedTotal = &expectedTotal
	shift.Discrepancy = &discrepancy
	shift.Notes = notes
	result := shiftToResponse(*shift)
	result.UserName = s.userName(shift.UserID)

	s.audit.Log(&LogAuditRequest{
		Action:     "close_shift",
		EntityType: "shift",
		EntityID:   shift.ID,
		UserID:     optionalUUID(actor.UserID),
		Metadata:   map[string]interface{}{"closing_cash": closingBalance, "expected_total": expectedTotal, "discrepancy": discrepancy},
	})

	return &result, nil
}

// Handover ghi một lần thu/chi tiền mặt ngoài bán hàng (nộp tiền về, rút tiền
// lẻ...) vào ca đang mở.
func (s *ShiftService) Handover(shiftID string, req *HandoverRequest, actor ShiftActor) (*CashHandoverResponse, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("nhập lý do thu/chi")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Khoá dòng ca để hai lần rút tiền cùng lúc không cùng thấy đủ tiền trong
	// két, và để không ghi được vào ca đang bị đóng dở.
	shift, err := findShift(tx, shiftID, true)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if shift.Status != "open" {
		tx.Rollback()
		return nil, errors.New("ca đã đóng — không ghi thu/chi được nữa")
	}
	if !actor.coTheThaoTac(shift) {
		tx.Rollback()
		return nil, ErrShiftForbidden
	}

	if req.HandoverType == "cash_out" {
		expected, err := expectedCash(tx, shift, time.Now())
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		drawer := shift.OpeningBalance + expected
		if req.Amount > drawer {
			tx.Rollback()
			return nil, fmt.Errorf("không đủ tiền trong két: hiện chỉ có %d đ, không chi được %d đ", drawer, req.Amount)
		}
	}

	handover := model.CashHandover{
		ShiftID:      shift.ID,
		Amount:       req.Amount,
		HandoverType: req.HandoverType,
		Reason:       reason,
		CreatedBy:    optionalUUID(actor.UserID),
	}

	if err := tx.Create(&handover).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "handover",
		EntityType: "cash_handover",
		EntityID:   handover.ID,
		UserID:     optionalUUID(actor.UserID),
		Metadata:   map[string]interface{}{"type": req.HandoverType, "amount": req.Amount},
	})

	return &CashHandoverResponse{
		ID:           handover.ID,
		ShiftID:      handover.ShiftID,
		Amount:       handover.Amount,
		HandoverType: handover.HandoverType,
		Reason:       handover.Reason,
		CreatedAt:    handover.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func shiftToResponse(s model.Shift) ShiftResponse {
	var endedAt *string
	if s.EndedAt != nil {
		f := s.EndedAt.Format("2006-01-02T15:04:05Z07:00")
		endedAt = &f
	}

	return ShiftResponse{
		ID:             s.ID,
		UserID:         s.UserID,
		StartedAt:      s.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		EndedAt:        endedAt,
		Status:         s.Status,
		OpeningBalance: s.OpeningBalance,
		ClosingBalance: s.ClosingBalance,
		ExpectedTotal:  s.ExpectedTotal,
		Discrepancy:    s.Discrepancy,
		Notes:          s.Notes,
		CreatedAt:      s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
