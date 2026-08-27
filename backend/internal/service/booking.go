package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BookingService struct {
	db    *gorm.DB
	audit *AuditService
	hub   *hub.Hub
}

func NewBookingService(db *gorm.DB, audit *AuditService) *BookingService {
	return &BookingService{db: db, audit: audit}
}

// WithHub nối hub WebSocket vào để thanh toán và huỷ đặt chỗ báo số dư mới cho máy trạm
// ngay lúc nó đổi. Không nối thì mọi thứ vẫn chạy đúng, chỉ là màn hình khách
// giữ số cũ cho tới khi tự tải lại.
//
// Nối rời thay vì thêm tham số cho constructor: giữ nguyên chữ ký thì mọi test
// dựng service không phải sửa, và hub vẫn nil được trong test.
func (s *BookingService) WithHub(h *hub.Hub) *BookingService {
	s.hub = h
	return s
}

type BookingListRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Sort      string `form:"sort"`
	Order     string `form:"order"`
	Search    string `form:"search"`
	Status    string `form:"status"`
	MachineID string `form:"machine_id"`
	DateFrom  string `form:"date_from"`
	DateTo    string `form:"date_to"`
}

type BookingResponse struct {
	ID        string `json:"id"`
	MachineID string `json:"machine_id"`
	// Cột "Mã máy" trên trang Đặt chỗ đọc trường này. Không trả về thì ô đó
	// luôn trống, và câu xác nhận check-in thành "tại máy ." — nhân viên không
	// biết đang giữ máy nào.
	MachineCode          string  `json:"machine_code"`
	MemberID             *string `json:"member_id"`
	CustomerName         string  `json:"customer_name"`
	CustomerPhone        string  `json:"customer_phone"`
	BookedFrom           string  `json:"booked_from"`
	BookedTo             string  `json:"booked_to"`
	DepositAmount        int64   `json:"deposit_amount"`
	DepositTransactionID *string `json:"deposit_transaction_id"`
	Status               string  `json:"status"`
	CancelAt             *string `json:"cancel_at"`
	Notes                string  `json:"notes"`
	CreatedBy            *string `json:"created_by"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type CreateBookingRequest struct {
	MachineID     string `json:"machine_id" binding:"required"`
	MemberID      string `json:"member_id"`
	CustomerName  string `json:"customer_name" binding:"required"`
	CustomerPhone string `json:"customer_phone" binding:"required"`
	BookedFrom    string `json:"booked_from" binding:"required"`
	BookedTo      string `json:"booked_to" binding:"required"`
	DepositAmount int64  `json:"deposit_amount"`
	Notes         string `json:"notes"`
	// IdempotencyKey do phía gọi sinh ra. Gửi lại cùng một khoá nghĩa là cùng
	// MỘT ý định, không phải hai lần thao tác. Bỏ trống là không tham gia.
	IdempotencyKey string `json:"idempotency_key"`
}

type UpdateBookingRequest struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	BookedFrom    string `json:"booked_from"`
	BookedTo      string `json:"booked_to"`
	DepositAmount int64  `json:"deposit_amount"`
	Notes         string `json:"notes"`
}

func (s *BookingService) List(req *BookingListRequest) (*pagination.Result, error) {
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

	var bookings []model.MachineBooking
	query := s.db

	if p.Search != "" {
		query = query.Where("customer_name ILIKE ? OR customer_phone ILIKE ?", "%"+p.Search+"%", "%"+p.Search+"%")
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.MachineID != "" {
		query = query.Where("machine_id = ?", req.MachineID)
	}
	if req.DateFrom != "" {
		if t, err := time.Parse(time.RFC3339, req.DateFrom); err == nil {
			query = query.Where("booked_from >= ?", t)
		}
	}
	if req.DateTo != "" {
		if t, err := time.Parse(time.RFC3339, req.DateTo); err == nil {
			query = query.Where("booked_to <= ?", t)
		}
	}

	var total int64
	query.Model(&model.MachineBooking{}).Count(&total)

	if err := pagination.Apply(query, p).Find(&bookings).Error; err != nil {
		return nil, err
	}

	items := make([]BookingResponse, len(bookings))
	codes := s.machineCodes(bookings)
	for i, b := range bookings {
		items[i] = bookingToResponse(b)
		items[i].MachineCode = codes[b.MachineID]
	}

	return pagination.NewResult(items, total, p), nil
}

// checkLimits áp hai trần trong nhóm cài đặt "limits". Cả hai để 0 nghĩa là
// không giới hạn — đó cũng là hành vi trước đây, khi hai ô này lưu được nhưng
// không nơi nào đọc.
//
// Đếm theo ngày của khung giờ đặt, KHÔNG phải ngày tạo phiếu: quán giới hạn số
// chỗ giữ trong một ngày kinh doanh, không giới hạn số lần nhân viên gõ máy.
func (s *BookingService) checkLimits(req *CreateBookingRequest, bookedFrom time.Time) error {
	limits := settingsGroup(s.db, "limits")

	perDay := settingInt(limits, "max_bookings_per_day", 0)
	perMember := settingInt(limits, "max_bookings_per_member", 0)
	if perDay <= 0 && perMember <= 0 {
		return nil
	}

	dayStart := time.Date(bookedFrom.Year(), bookedFrom.Month(), bookedFrom.Day(), 0, 0, 0, 0, bookedFrom.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	live := "status NOT IN ('cancelled', 'no_show')"

	if perDay > 0 {
		var n int64
		s.db.Model(&model.MachineBooking{}).
			Where(live+" AND booked_from >= ? AND booked_from < ?", dayStart, dayEnd).
			Count(&n)
		if n >= perDay {
			return fmt.Errorf("ngày %s đã đủ %d lượt đặt chỗ — trần mỗi ngày là %d",
				dayStart.Format("02/01/2006"), n, perDay)
		}
	}

	if perMember > 0 && req.MemberID != "" {
		var n int64
		s.db.Model(&model.MachineBooking{}).
			Where(live+" AND member_id = ? AND booked_from >= ? AND booked_from < ?",
				req.MemberID, dayStart, dayEnd).
			Count(&n)
		if n >= perMember {
			return fmt.Errorf("hội viên đã đặt %d chỗ trong ngày %s — trần mỗi người là %d",
				n, dayStart.Format("02/01/2006"), perMember)
		}
	}
	return nil
}

func (s *BookingService) GetByID(id string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ?", id).First(&booking).Error; err != nil {
		return nil, err
	}
	result := bookingToResponse(booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	return &result, nil
}

func (s *BookingService) Create(req *CreateBookingRequest, userID string) (*BookingResponse, error) {
	bookedFrom, err := time.Parse(time.RFC3339, req.BookedFrom)
	if err != nil {
		return nil, errors.New("invalid booked_from format, use RFC3339")
	}
	bookedTo, err := time.Parse(time.RFC3339, req.BookedTo)
	if err != nil {
		return nil, errors.New("invalid booked_to format, use RFC3339")
	}

	if bookedTo.Before(bookedFrom) || bookedTo.Equal(bookedFrom) {
		return nil, errors.New("booked_to must be after booked_from")
	}

	var existing int64
	s.db.Model(&model.MachineBooking{}).
		Where("machine_id = ? AND status NOT IN ('cancelled', 'completed') AND booked_from < ? AND booked_to > ?",
			req.MachineID, bookedTo, bookedFrom).
		Count(&existing)
	if existing > 0 {
		return nil, errors.New("machine already booked for this time period")
	}

	if err := s.checkLimits(req, bookedFrom); err != nil {
		return nil, err
	}

	var memberID *string
	if req.MemberID != "" {
		memberID = &req.MemberID
	}

	booking := model.MachineBooking{
		MachineID:     req.MachineID,
		MemberID:      memberID,
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		BookedFrom:    bookedFrom,
		BookedTo:      bookedTo,
		DepositAmount: req.DepositAmount,
		Status:        "pending",
		Notes:         req.Notes,
	}

	if userID != "" {
		booking.CreatedBy = &userID
	}

	// The booking and its deposit are one unit of work. Previously the deposit
	// was only written onto the row while cancelling refunded it for real,
	// which turned "book then cancel" into free money.
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := giuKhoaIdempotency(tx, "booking.create", req.IdempotencyKey); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Số dư sau khi trừ cọc, mang ra ngoài khối để còn báo cho máy trạm sau khi
	// commit. Báo bên trong transaction là hứa một con số có thể bị rollback.
	var hvSoDu string
	var soDuSau, thuongSau int64

	var depositTx *model.MemberTransaction
	if booking.DepositAmount > 0 && memberID != nil {
		var member model.Member
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&member, "id = ?", *memberID).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("member not found")
		}
		if member.Balance < booking.DepositAmount {
			tx.Rollback()
			return nil, errors.New("insufficient balance for the deposit")
		}

		balanceAfter := member.Balance - booking.DepositAmount
		depositTx = &model.MemberTransaction{
			MemberID:        *memberID,
			TransactionType: "booking_deposit",
			Amount:          -booking.DepositAmount,
			BalanceBefore:   member.Balance,
			BalanceAfter:    balanceAfter,
			Description:     "Deposit for machine booking",
			CreatedAt:       time.Now(),
		}
		if userID != "" {
			depositTx.CreatedBy = &userID
		}
		if err := tx.Create(depositTx).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Model(&member).Update("balance", balanceAfter).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		hvSoDu, soDuSau, thuongSau = *memberID, balanceAfter, member.BonusBalance
		booking.DepositTransactionID = &depositTx.ID
	}

	if err := tx.Create(&booking).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	phatSoDuMoi(s.hub, hvSoDu, soDuSau, thuongSau)

	result := bookingToResponse(booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	var uid *string
	if userID != "" {
		uid = &userID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_booking",
		EntityID:   booking.ID,
		UserID:     uid,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"deposit_amount": booking.DepositAmount,
			"status":         booking.Status,
			"booked_from":    booking.BookedFrom.Format(time.RFC3339),
			"booked_to":      booking.BookedTo.Format(time.RFC3339),
		},
	})
	return &result, nil
}

func (s *BookingService) Update(id string, req *UpdateBookingRequest, userID string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ?", id).First(&booking).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.CustomerName != "" {
		updates["customer_name"] = req.CustomerName
	}
	if req.CustomerPhone != "" {
		updates["customer_phone"] = req.CustomerPhone
	}
	if req.BookedFrom != "" {
		t, err := time.Parse(time.RFC3339, req.BookedFrom)
		if err != nil {
			return nil, errors.New("invalid booked_from format")
		}
		updates["booked_from"] = t
	}
	if req.BookedTo != "" {
		t, err := time.Parse(time.RFC3339, req.BookedTo)
		if err != nil {
			return nil, errors.New("invalid booked_to format")
		}
		updates["booked_to"] = t
	}
	if req.DepositAmount > 0 {
		updates["deposit_amount"] = req.DepositAmount
	}
	updates["notes"] = req.Notes
	updates["updated_at"] = time.Now()

	if err := s.db.Model(&booking).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.db.First(&booking, "id = ?", id)
	result := bookingToResponse(booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	var uid *string
	if userID != "" {
		uid = &userID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "machine_booking",
		EntityID:   id,
		UserID:     uid,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"status":         booking.Status,
		},
	})
	return &result, nil
}

// Delete xoá một lịch đặt máy.
//
// Không bảng nào tham chiếu machine_bookings, nên chặn ở đây là chặn theo
// TRẠNG THÁI: lịch đã nhận máy là một lượt chơi đã xảy ra, và lịch đã thu cọc
// là tiền đang giữ của khách. Cả hai phải đi qua đường HUỶ — Cancel hoàn cọc
// tử tế, còn xoá thì tiền nằm lại trong két mà không có chứng từ nào giải thích.
func (s *BookingService) Delete(id string) error {
	var booking model.MachineBooking
	if err := s.db.Where("id = ?", id).First(&booking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy lịch đặt máy")
		}
		return err
	}

	if booking.Status == "checked_in" {
		return chanVi("lịch đặt đã nhận máy, không xoá được — hãy dùng chức năng huỷ")
	}
	if booking.DepositAmount > 0 && booking.DepositTransactionID != nil {
		return chanVi("lịch đặt đã thu cọc %d đ — hãy huỷ để hoàn cọc thay vì xoá", booking.DepositAmount)
	}

	now := time.Now()
	if err := s.db.Model(&booking).Update("deleted_at", &now).Error; err != nil {
		return err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"status":         booking.Status,
		},
	})
	return nil
}

func (s *BookingService) CheckIn(id string) (*BookingResponse, error) {
	result, err := s.updateStatus(id, "checked_in", nil, "pending")
	if err != nil {
		return nil, err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "check_in",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"status": "checked_in",
		},
	})
	return result, nil
}

func (s *BookingService) Cancel(id string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ?", id).First(&booking).Error; err != nil {
		return nil, err
	}

	if booking.Status == "cancelled" {
		return nil, errors.New("booking already cancelled")
	}
	if booking.Status == "checked_in" {
		return nil, errors.New("cannot cancel a checked-in booking")
	}
	if booking.Status == "no_show" {
		return nil, errors.New("cannot cancel a no-show booking")
	}

	// cancel_before_minutes (nhóm "limits"): huỷ sát giờ thì quán không kịp bán
	// lại chỗ đó, nên chỉ được huỷ khi còn cách giờ giữ máy đủ xa. Để 0 là huỷ
	// lúc nào cũng được — đúng như hành vi trước đây.
	if minutes := settingInt(settingsGroup(s.db, "limits"), "cancel_before_minutes", 0); minutes > 0 {
		deadline := booking.BookedFrom.Add(-time.Duration(minutes) * time.Minute)
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("chỉ huỷ được trước giờ giữ máy ít nhất %d phút (hạn huỷ: %s)",
				minutes, deadline.Format("02/01/2006 15:04"))
		}
	}

	tx := s.db.Begin()

	// Kiểm tra ở trên chạy trên bản đọc ngoài transaction. Hai lệnh huỷ song song
	// đều thấy trạng thái cũ và đều hoàn cọc — khách được trả cọc hai lần. Khoá
	// rồi đọc lại là chỗ duy nhất chặn được.
	var locked model.MachineBooking
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&locked).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if locked.Status != "pending" {
		tx.Rollback()
		return nil, fmt.Errorf("lịch đặt đang ở trạng thái %s, không huỷ được", locked.Status)
	}
	booking = locked

	var hvSoDu string
	var soDuSau, thuongSau int64

	// Refund only what was actually taken. DepositTransactionID is set when the
	// deposit was charged to a member's balance; a cash deposit from a walk-in
	// never touched the ledger and is settled at the counter.
	if booking.DepositAmount > 0 && booking.DepositTransactionID != nil && booking.MemberID != nil {
		var member model.Member
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&member, "id = ?", *booking.MemberID).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("member not found")
		}

		balanceAfter := member.Balance + booking.DepositAmount
		trans := model.MemberTransaction{
			MemberID:        *booking.MemberID,
			TransactionType: "deposit_refund",
			Amount:          booking.DepositAmount,
			BalanceBefore:   member.Balance,
			BalanceAfter:    balanceAfter,
			ReferenceID:     &booking.ID,
			Description:     "Deposit refund for cancelled booking",
			CreatedAt:       time.Now(),
		}
		if err := tx.Create(&trans).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Model(&member).Update("balance", balanceAfter).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		hvSoDu, soDuSau, thuongSau = *booking.MemberID, balanceAfter, member.BonusBalance
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":     "cancelled",
		"cancel_at":  &now,
		"updated_at": now,
	}
	if err := tx.Model(&booking).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()

	phatSoDuMoi(s.hub, hvSoDu, soDuSau, thuongSau)
	phatLichDatDoi(s.hub, booking.ID, "cancelled")

	booking.Status = "cancelled"
	booking.CancelAt = &now
	result := bookingToResponse(booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	refundAmount := int64(0)
	if booking.DepositAmount > 0 && booking.DepositTransactionID != nil && booking.MemberID != nil {
		refundAmount = booking.DepositAmount
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "cancel",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"status":         "cancelled",
			"deposit_amount": booking.DepositAmount,
			"refund_amount":  refundAmount,
			"member_id":      booking.MemberID,
		},
	})
	return &result, nil
}

func (s *BookingService) NoShow(id string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ?", id).First(&booking).Error; err != nil {
		return nil, err
	}

	if booking.Status == "no_show" {
		return nil, errors.New("booking already marked as no-show")
	}
	if booking.Status == "cancelled" {
		return nil, errors.New("cannot mark cancelled booking as no-show")
	}

	result, err := s.updateStatus(id, "no_show", nil, "pending")
	if err != nil {
		return nil, err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "no_show",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"status": "no_show",
		},
	})
	return result, nil
}

// phatLichDatDoi báo cho mọi thiết bị quản trị rằng một lịch đặt vừa đổi trạng
// thái. Không có sự kiện nào cho đặt chỗ, nên nhận máy ở quầy xong thì danh sách
// trên điện thoại của cùng nhân viên đó vẫn hiện lịch cũ.
func phatLichDatDoi(h *hub.Hub, bookingID, status string) {
	if h == nil || bookingID == "" {
		return
	}
	h.BroadcastToType(hub.Event{
		Type: "booking:updated",
		Data: map[string]interface{}{"booking_id": bookingID, "status": status},
	}, hub.ClientTypeAdmin)
}

// updateStatus đổi trạng thái lịch đặt. chapNhanTu liệt kê những trạng thái được
// phép chuyển đi; để trống là chấp nhận mọi trạng thái.
//
// Việc đọc và ghi nằm trong cùng một transaction với hàng đã khoá. Trước đây hàm
// đọc trần rồi ghi đè vô điều kiện, nên gọi hai lần — hai nhân viên, hay một
// người trên hai thiết bị — đều thành công và mỗi lần lại ghi một lượt audit.
func (s *BookingService) updateStatus(id, status string, cancelAt *time.Time, chapNhanTu ...string) (*BookingResponse, error) {
	tx := s.db.Begin()

	var booking model.MachineBooking
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&booking).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if len(chapNhanTu) > 0 && !contains(chapNhanTu, booking.Status) {
		tx.Rollback()
		return nil, fmt.Errorf("lịch đặt đang ở trạng thái %s, không thể chuyển sang %s", booking.Status, status)
	}

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if cancelAt != nil {
		updates["cancel_at"] = cancelAt
	}

	if err := tx.Model(&booking).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	phatLichDatDoi(s.hub, booking.ID, status)

	booking.Status = status
	if cancelAt != nil {
		booking.CancelAt = cancelAt
	}
	result := bookingToResponse(booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	return &result, nil
}

// machineCodes nạp mã máy cho cả trang một lần thay vì mỗi dòng một truy vấn.
func (s *BookingService) machineCodes(bookings []model.MachineBooking) map[string]string {
	ids := make([]string, 0, len(bookings))
	for _, b := range bookings {
		ids = append(ids, b.MachineID)
	}
	out := map[string]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []struct {
		ID          string
		MachineCode string
	}
	// Unscoped: máy đã gỡ khỏi danh sách vẫn phải hiện mã trên lịch đặt cũ.
	s.db.Unscoped().Model(&model.Machine{}).Select("id, machine_code").Where("id IN ?", ids).Find(&rows)
	for _, r := range rows {
		out[r.ID] = r.MachineCode
	}
	return out
}

func (s *BookingService) machineCode(id string) string {
	var m model.Machine
	if err := s.db.Unscoped().Select("machine_code").Where("id = ?", id).First(&m).Error; err != nil {
		return ""
	}
	return m.MachineCode
}

func bookingToResponse(b model.MachineBooking) BookingResponse {
	resp := BookingResponse{
		ID:            b.ID,
		MachineID:     b.MachineID,
		MemberID:      b.MemberID,
		CustomerName:  b.CustomerName,
		CustomerPhone: b.CustomerPhone,
		BookedFrom:    b.BookedFrom.Format(time.RFC3339),
		BookedTo:      b.BookedTo.Format(time.RFC3339),
		DepositAmount: b.DepositAmount,
		Status:        b.Status,
		Notes:         b.Notes,
		CreatedBy:     b.CreatedBy,
		CreatedAt:     b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     b.UpdatedAt.Format(time.RFC3339),
	}
	if b.DepositTransactionID != nil {
		resp.DepositTransactionID = b.DepositTransactionID
	}
	if b.CancelAt != nil {
		s := b.CancelAt.Format(time.RFC3339)
		resp.CancelAt = &s
	}
	return resp
}
