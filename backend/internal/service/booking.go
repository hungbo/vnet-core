package service

import (
	"errors"
	"fmt"
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

// Ba mốc giờ của một lịch đặt, dùng chung cho nhận máy tay, đánh vắng tay, tác
// vụ nền bookings:expire và chốt giữ máy trong StartSession. Mỗi chỗ tự đặt
// một con số là cách chắc chắn nhất để nút "Đánh vắng" bấm được ở phút thứ 5
// trong khi tác vụ nền đợi tới phút 15, hay máy trạm chặn khách lạ lâu hơn hạn
// đánh vắng.
const (
	// BookingCheckInEarly: khách tới sớm tối đa bao lâu thì nhận máy được. Sớm
	// hơn nữa thì máy có thể còn đang bán cho người khác.
	BookingCheckInEarly = 30 * time.Minute
	// BookingNoShowGrace: quá giờ bắt đầu bao lâu thì coi là không đến — mất
	// cọc, nhả máy. Cũng là hạn giữ máy cho riêng chủ lịch.
	BookingNoShowGrace = 15 * time.Minute
	// bookingPastGrace: chấp nhận giờ bắt đầu lùi về quá khứ chừng này, vì
	// nhân viên chọn "bây giờ" trên ô giờ rồi mất vài phút gõ tên khách.
	bookingPastGrace = 5 * time.Minute
)

// trangThaiDatCho là toàn bộ trạng thái backend thật sự ghi. Lọc theo một chữ
// ngoài danh sách này trước đây trả về danh sách rỗng, nên gõ nhầm "noshow"
// trông y như "không có lịch nào".
var trangThaiDatCho = map[string]string{
	"pending":    "đang chờ",
	"checked_in": "đã nhận máy",
	"completed":  "đã hoàn tất",
	"cancelled":  "đã huỷ",
	"no_show":    "không đến",
}

func tenTrangThaiDatCho(st string) string {
	if v, ok := trangThaiDatCho[st]; ok {
		return v
	}
	return st
}

// ErrDatChoKhongHopLe đánh dấu lỗi do dữ liệu người gọi gửi lên, để handler trả
// 400 thay vì 500.
var ErrDatChoKhongHopLe = errors.New("dữ liệu đặt chỗ không hợp lệ")

// loiDatCho mang câu tiếng Việt hiện cho người dùng, đồng thời vẫn khớp được
// errors.Is với sentinel gốc (ErrDatChoKhongHopLe, gorm.ErrRecordNotFound).
type loiDatCho struct {
	msg string
	goc error
}

func (e *loiDatCho) Error() string { return e.msg }
func (e *loiDatCho) Unwrap() error { return e.goc }

func datChoSai(format string, a ...interface{}) error {
	return &loiDatCho{msg: fmt.Sprintf(format, a...), goc: ErrDatChoKhongHopLe}
}

func khongThayLichDat() error {
	return &loiDatCho{msg: "không tìm thấy lịch đặt máy", goc: gorm.ErrRecordNotFound}
}

var mauUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// laUUID kiểm tra trước khi đưa chuỗi vào câu SQL: cột là uuid nên "abc" làm
// PostgreSQL ném lỗi thô "invalid input syntax for type uuid", và lỗi đó lọt
// nguyên văn ra màn hình (hoặc thành 500 ở trang danh sách).
func laUUID(s string) bool { return mauUUID.MatchString(s) }

func gioVN(t time.Time) string { return t.In(utils.VietnamLocation()).Format("15:04") }

func ngayGioVN(t time.Time) string {
	return t.In(utils.VietnamLocation()).Format("02/01/2006 15:04")
}

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

// UpdateBookingRequest: chuỗi rỗng / trường vắng mặt nghĩa là GIỮ NGUYÊN.
//
// Notes là con trỏ vì "xoá ghi chú" (gửi "") phải khác "không gửi" — bản cũ
// ghi đè notes bằng "" mỗi lần PUT thiếu trường này.
//
// DepositAmount chỉ còn để phát hiện người gọi cố đổi cọc: cọc đã trừ khỏi ví
// lúc tạo, sửa con số trên lịch không đụng tới ví, mà huỷ lại hoàn theo con số
// đã sửa — đặt 25.000đ rồi sửa thành 500.000đ là rút được 475.000đ.
type UpdateBookingRequest struct {
	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone"`
	BookedFrom    string  `json:"booked_from"`
	BookedTo      string  `json:"booked_to"`
	DepositAmount *int64  `json:"deposit_amount"`
	Notes         *string `json:"notes"`
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
		// Ô nhập trên trang Đặt chỗ ghi "Tìm theo tên / mã máy", nhưng chỗ này
		// chỉ tra tên và số điện thoại — gõ đúng mã máy lại ra rỗng. Và tên
		// khách là chữ tiếng Việt có dấu trong khi nhân viên gõ không dấu, nên
		// ILIKE trần cũng không khớp. Tra mã máy bằng truy vấn con để không
		// join (join làm hỏng Count ở dưới khi một dòng khớp cả hai vế).
		tuKhoa := "%" + p.Search + "%"
		query = query.Where(
			"unaccent(customer_name) ILIKE unaccent(?) OR customer_phone ILIKE ? OR machine_id IN (SELECT id FROM machines WHERE machine_code ILIKE ? AND deleted_at IS NULL)",
			tuKhoa, tuKhoa, tuKhoa,
		)
	}
	// Bộ lọc sai phải báo sai. Bản cũ lặng lẽ bỏ qua ngày sai định dạng (trả về
	// MỌI lịch, trông như lọc đúng), lọc trạng thái lạ ra danh sách rỗng, còn
	// machine_id không phải uuid làm PostgreSQL ném lỗi thành 500.
	if req.Status != "" {
		if _, ok := trangThaiDatCho[req.Status]; !ok {
			return nil, datChoSai("trạng thái %q không hợp lệ — chỉ nhận: pending, checked_in, completed, cancelled, no_show", req.Status)
		}
		query = query.Where("status = ?", req.Status)
	}
	if req.MachineID != "" {
		if !laUUID(req.MachineID) {
			return nil, datChoSai("machine_id %q không phải mã máy hợp lệ", req.MachineID)
		}
		query = query.Where("machine_id = ?", req.MachineID)
	}
	if req.DateFrom != "" {
		t, err := time.Parse(time.RFC3339, req.DateFrom)
		if err != nil {
			return nil, datChoSai("date_from %q không đúng định dạng RFC3339 (vd 2026-10-04T00:00:00+07:00)", req.DateFrom)
		}
		query = query.Where("booked_from >= ?", t)
	}
	if req.DateTo != "" {
		t, err := time.Parse(time.RFC3339, req.DateTo)
		if err != nil {
			return nil, datChoSai("date_to %q không đúng định dạng RFC3339 (vd 2026-10-04T23:59:59+07:00)", req.DateTo)
		}
		query = query.Where("booked_to <= ?", t)
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
//
// "Ngày" là ngày GIỜ VIỆT NAM. Trang quản trị gửi giờ dạng UTC (toISOString),
// nên lấy ngày theo múi của chuỗi gửi lên thì lịch 06:00 sáng giờ ta rơi vào
// ngày hôm trước và bị đếm chung với ngày đó.
//
// limits đọc trước ở ngoài transaction; tx là transaction đang khoá máy để
// đếm và chèn lịch đi liền nhau.
func checkLimits(tx *gorm.DB, limits map[string]string, memberID string, bookedFrom time.Time) error {
	perDay := settingInt(limits, "max_bookings_per_day", 0)
	perMember := settingInt(limits, "max_bookings_per_member", 0)
	if perDay <= 0 && perMember <= 0 {
		return nil
	}

	dayStart := utils.StartOfDay(bookedFrom)
	dayEnd := dayStart.AddDate(0, 0, 1)
	live := "status NOT IN ('cancelled', 'no_show')"

	if perDay > 0 {
		var n int64
		tx.Model(&model.MachineBooking{}).
			Where(live+" AND booked_from >= ? AND booked_from < ?", dayStart, dayEnd).
			Count(&n)
		if n >= perDay {
			return fmt.Errorf("ngày %s đã đủ %d lượt đặt chỗ — trần mỗi ngày là %d",
				dayStart.Format("02/01/2006"), n, perDay)
		}
	}

	if perMember > 0 && memberID != "" {
		var n int64
		tx.Model(&model.MachineBooking{}).
			Where(live+" AND member_id = ? AND booked_from >= ? AND booked_from < ?",
				memberID, dayStart, dayEnd).
			Count(&n)
		if n >= perMember {
			return fmt.Errorf("hội viên đã đặt %d chỗ trong ngày %s — trần mỗi người là %d",
				n, dayStart.Format("02/01/2006"), perMember)
		}
	}
	return nil
}

// kiemKhungGio là luật chung của Create và Update cho một khoảng giờ giữ máy.
func kiemKhungGio(from, to time.Time) error {
	if !to.After(from) {
		return errors.New("giờ kết thúc phải sau giờ bắt đầu")
	}
	if from.Before(time.Now().Add(-bookingPastGrace)) {
		return fmt.Errorf("giờ bắt đầu %s đã qua — chỉ đặt được từ bây giờ trở đi", ngayGioVN(from))
	}
	return nil
}

// trungLich đếm lịch còn giữ máy trong khoảng [from, to). no_show đã nhả máy,
// cancelled và completed đã xong — cả ba không được chặn khung giờ. Bản cũ quên
// no_show nên một khách không đến vẫn giữ chặt máy tới hết khung giờ.
func trungLich(tx *gorm.DB, machineID string, from, to time.Time, boQuaID string) (bool, error) {
	q := tx.Model(&model.MachineBooking{}).
		Where("machine_id = ? AND status NOT IN ('cancelled', 'completed', 'no_show') AND booked_from < ? AND booked_to > ?",
			machineID, to, from)
	if boQuaID != "" {
		q = q.Where("id <> ?", boQuaID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// khoaMayDeDat khoá dòng máy trong transaction.
//
// Khoá máy là thứ biến "đếm trùng rồi chèn" thành một bước: hai nhân viên đặt
// cùng máy cùng giờ trước đây đều đếm thấy 0 rồi cùng chèn. Khoá theo máy
// (không theo lịch) vì lịch mới chưa có dòng nào để khoá.
func khoaMayDeDat(tx *gorm.DB, machineID string) (*model.Machine, error) {
	var machine model.Machine
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", machineID).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy")
		}
		return nil, err
	}
	return &machine, nil
}

func kiemMayHoatDong(machine *model.Machine) error {
	if !machine.IsActive {
		return fmt.Errorf("máy %s đang bị khoá, không đặt chỗ được", machine.MachineCode)
	}
	return nil
}

// docLichDat đọc một lịch, kiểm id trước để "abc" không thành lỗi SQL thô.
func (s *BookingService) docLichDat(id string) (*model.MachineBooking, error) {
	if !laUUID(id) {
		return nil, khongThayLichDat()
	}
	var booking model.MachineBooking
	if err := s.db.Where("id = ?", id).First(&booking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, khongThayLichDat()
		}
		return nil, err
	}
	return &booking, nil
}

// khoaLichDat đọc lại lịch dưới khoá trong transaction — mọi kiểm tra trạng
// thái phải làm trên bản này, bản đọc ngoài transaction có thể đã cũ.
func khoaLichDat(tx *gorm.DB, id string) (*model.MachineBooking, error) {
	var booking model.MachineBooking
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&booking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, khongThayLichDat()
		}
		return nil, err
	}
	return &booking, nil
}

func (s *BookingService) GetByID(id string) (*BookingResponse, error) {
	booking, err := s.docLichDat(id)
	if err != nil {
		return nil, err
	}
	result := bookingToResponse(*booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	return &result, nil
}

func (s *BookingService) Create(req *CreateBookingRequest, userID string) (*BookingResponse, error) {
	bookedFrom, err := time.Parse(time.RFC3339, req.BookedFrom)
	if err != nil {
		return nil, errors.New("giờ bắt đầu không đúng định dạng, phải theo RFC3339")
	}
	bookedTo, err := time.Parse(time.RFC3339, req.BookedTo)
	if err != nil {
		return nil, errors.New("giờ kết thúc không đúng định dạng, phải theo RFC3339")
	}
	if err := kiemKhungGio(bookedFrom, bookedTo); err != nil {
		return nil, err
	}
	if req.DepositAmount < 0 {
		return nil, errors.New("tiền cọc không được âm")
	}
	req.MemberID = strings.TrimSpace(req.MemberID)
	// Cọc chỉ thu bằng ví hội viên. Khách vãng lai đưa tiền mặt ở quầy thì cọc
	// không để lại chứng từ nào: không vào sổ ví, không vào ca, xoá lịch là
	// tiền nằm trong két mà không ai giải thích được.
	if req.DepositAmount > 0 && req.MemberID == "" {
		return nil, errors.New("khách vãng lai không đặt cọc được — cọc chỉ trừ từ ví hội viên; hãy chọn hội viên hoặc để cọc 0")
	}
	if !laUUID(req.MachineID) {
		return nil, errors.New("không tìm thấy máy")
	}
	if req.MemberID != "" && !laUUID(req.MemberID) {
		return nil, errors.New("không tìm thấy hội viên")
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

	// Đọc cài đặt trước khi mở transaction: không cần khoá, và một câu đọc hỏng
	// bên trong sẽ làm hỏng cả transaction.
	limits := settingsGroup(s.db, "limits")

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

	// Thứ tự khoá trên toàn hệ thống: hội viên → máy → lịch đặt. StartSession
	// khoá đúng thứ tự này; khoá ngược thì hai thao tác chờ nhau mãi.
	var member model.Member
	if memberID != nil {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&member, "id = ?", *memberID).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("không tìm thấy hội viên")
		}
		if !member.IsActive {
			tx.Rollback()
			return nil, errors.New("tài khoản hội viên đã bị khoá, không đặt chỗ được")
		}
	}

	machine, err := khoaMayDeDat(tx, req.MachineID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := kiemMayHoatDong(machine); err != nil {
		tx.Rollback()
		return nil, err
	}
	trung, err := trungLich(tx, req.MachineID, bookedFrom, bookedTo, "")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if trung {
		tx.Rollback()
		return nil, errors.New("máy đã có người đặt trong khung giờ này")
	}
	if err := checkLimits(tx, limits, req.MemberID, bookedFrom); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Create(&booking).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Số dư sau khi trừ cọc, mang ra ngoài khối để còn báo cho máy trạm sau khi
	// commit. Báo bên trong transaction là hứa một con số có thể bị rollback.
	var hvSoDu string
	var soDuSau, thuongSau int64

	if booking.DepositAmount > 0 && memberID != nil {
		if member.Balance < booking.DepositAmount {
			tx.Rollback()
			return nil, errors.New("số dư không đủ để đặt cọc")
		}

		balanceAfter := member.Balance - booking.DepositAmount
		// Ghi SAU khi lịch đã có ID để giao dịch cọc trỏ ngược về lịch
		// (reference_id) — sổ ví không còn những dòng "Deposit" mồ côi.
		depositTx := &model.MemberTransaction{
			MemberID:        *memberID,
			TransactionType: "booking_deposit",
			Amount:          -booking.DepositAmount,
			BalanceBefore:   member.Balance,
			BalanceAfter:    balanceAfter,
			ReferenceID:     &booking.ID,
			Description:     fmt.Sprintf("Đặt cọc giữ máy từ %s", ngayGioVN(bookedFrom)),
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
		if err := tx.Model(&booking).Update("deposit_transaction_id", depositTx.ID).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		hvSoDu, soDuSau, thuongSau = *memberID, balanceAfter, member.BonusBalance
		booking.DepositTransactionID = &depositTx.ID
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	phatSoDuMoi(s.hub, hvSoDu, soDuSau, thuongSau)

	result := bookingToResponse(booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_booking",
		EntityID:   booking.ID,
		UserID:     uidHoacNil(userID),
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

func uidHoacNil(userID string) *string {
	if userID == "" {
		return nil
	}
	return &userID
}

// kiemHanHuy áp cancel_before_minutes (nhóm "limits"): huỷ sát giờ thì quán
// không kịp bán lại chỗ đó. Để 0 là lúc nào cũng được — đúng như trước đây.
//
// Dùng chung cho HUỶ và ĐỔI GIỜ: dời một lịch sát giờ sang hôm khác cũng nhả
// chỗ ngay lúc đó y như huỷ, nên bản cũ — đổi giờ không kiểm gì — là lối đi
// vòng qua hạn huỷ.
func (s *BookingService) kiemHanHuy(bookedFrom time.Time, hanhDong string) error {
	minutes := settingInt(settingsGroup(s.db, "limits"), "cancel_before_minutes", 0)
	if minutes <= 0 {
		return nil
	}
	deadline := bookedFrom.Add(-time.Duration(minutes) * time.Minute)
	if time.Now().After(deadline) {
		return fmt.Errorf("chỉ %s được trước giờ giữ máy ít nhất %d phút (hạn %s: %s)",
			hanhDong, minutes, hanhDong, ngayGioVN(deadline))
	}
	return nil
}

// Update sửa một lịch còn đang chờ. Chạy lại đúng các luật của Create cho phần
// bị đổi: bản cũ ghi thẳng mọi trường, nên sửa giờ là lách được cả kiểm trùng
// lịch, trần mỗi ngày, hạn huỷ, và sửa được cả lịch đã huỷ hay đã nhận máy.
func (s *BookingService) Update(id string, req *UpdateBookingRequest, userID string) (*BookingResponse, error) {
	booking, err := s.docLichDat(id)
	if err != nil {
		return nil, err
	}

	if req.DepositAmount != nil && *req.DepositAmount != booking.DepositAmount {
		return nil, errors.New("không sửa được tiền cọc sau khi đã tạo lịch — muốn đổi cọc hãy huỷ lịch (hoàn cọc) rồi đặt lại")
	}

	newFrom, newTo := booking.BookedFrom, booking.BookedTo
	if req.BookedFrom != "" {
		t, err := time.Parse(time.RFC3339, req.BookedFrom)
		if err != nil {
			return nil, errors.New("giờ bắt đầu không đúng định dạng")
		}
		newFrom = t
	}
	if req.BookedTo != "" {
		t, err := time.Parse(time.RFC3339, req.BookedTo)
		if err != nil {
			return nil, errors.New("giờ kết thúc không đúng định dạng")
		}
		newTo = t
	}
	doiGio := !newFrom.Equal(booking.BookedFrom) || !newTo.Equal(booking.BookedTo)

	limits := map[string]string{}
	if doiGio {
		if err := s.kiemHanHuy(booking.BookedFrom, "đổi giờ"); err != nil {
			return nil, err
		}
		if err := kiemKhungGio(newFrom, newTo); err != nil {
			return nil, err
		}
		limits = settingsGroup(s.db, "limits")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Máy trước rồi tới lịch — cùng thứ tự hội viên → máy → lịch của cả hệ thống.
	machine, err := khoaMayDeDat(tx, booking.MachineID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// Máy bị khoá thì không dời lịch sang giờ khác trên nó, nhưng vẫn sửa được
	// tên, số điện thoại, ghi chú của lịch sẵn có.
	if doiGio {
		if err := kiemMayHoatDong(machine); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	locked, err := khoaLichDat(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if locked.Status != "pending" {
		tx.Rollback()
		return nil, fmt.Errorf("lịch đặt %s, chỉ sửa được lịch đang chờ", tenTrangThaiDatCho(locked.Status))
	}
	booking = locked

	updates := map[string]interface{}{}
	if req.CustomerName != "" {
		updates["customer_name"] = req.CustomerName
	}
	if req.CustomerPhone != "" {
		updates["customer_phone"] = req.CustomerPhone
	}
	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}
	if doiGio {
		trung, err := trungLich(tx, booking.MachineID, newFrom, newTo, booking.ID)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if trung {
			tx.Rollback()
			return nil, errors.New("máy đã có người đặt trong khung giờ này")
		}
		// Đổi sang ngày khác thì lịch này thành một lượt mới của ngày đó.
		if !utils.StartOfDay(newFrom).Equal(utils.StartOfDay(booking.BookedFrom)) {
			memberID := ""
			if booking.MemberID != nil {
				memberID = *booking.MemberID
			}
			if err := checkLimits(tx, limits, memberID, newFrom); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		updates["booked_from"] = newFrom
		updates["booked_to"] = newTo
	}
	updates["updated_at"] = time.Now()

	if err := tx.Model(booking).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.db.First(booking, "id = ?", id)
	result := bookingToResponse(*booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "machine_booking",
		EntityID:   id,
		UserID:     uidHoacNil(userID),
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"status":         booking.Status,
			"booked_from":    booking.BookedFrom.Format(time.RFC3339),
			"booked_to":      booking.BookedTo.Format(time.RFC3339),
		},
	})
	return &result, nil
}

// Delete xoá một lịch đặt máy.
//
// Không bảng nào tham chiếu machine_bookings, nên chặn ở đây là chặn theo
// TRẠNG THÁI, và chỉ chặn khi xoá làm mất dấu tiền hay một lượt chơi đang diễn ra:
//   - đang chờ mà đã thu cọc: tiền khách còn đang giữ — phải đi qua Huỷ để hoàn.
//   - đã nhận máy: khách đang ngồi trong khung giờ đó; hết giờ lịch tự sang
//     "hoàn tất" và lúc đó xoá được.
//
// Lịch đã huỷ / không đến / hoàn tất thì cọc đã xong xuôi (hoàn lại hoặc mất) và
// giao dịch ví vẫn nằm trong sổ, nên xoá được. Bản cũ chặn mọi lịch có cọc bất
// kể trạng thái, nên lịch không đến hay đã huỷ có cọc nằm vĩnh viễn trên trang.
func (s *BookingService) Delete(id, userID string) error {
	booking, err := s.docLichDat(id)
	if err != nil {
		return err
	}

	switch booking.Status {
	case "checked_in":
		return chanVi("lịch đặt đã nhận máy và còn trong giờ giữ máy, không xoá được — hết giờ lịch tự chuyển sang hoàn tất, khi đó mới xoá")
	case "pending":
		if booking.DepositTransactionID != nil {
			return chanVi("lịch đặt đã thu cọc %d đ — hãy huỷ để hoàn cọc thay vì xoá", booking.DepositAmount)
		}
	}

	now := time.Now()
	if err := s.db.Model(booking).Update("deleted_at", &now).Error; err != nil {
		return err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_booking",
		EntityID:   id,
		UserID:     uidHoacNil(userID),
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"status":         booking.Status,
		},
	})
	return nil
}

// cocDaThu trả về số cọc THẬT SỰ đã trừ khỏi ví, đọc từ chính giao dịch cọc.
//
// Không bao giờ đọc deposit_amount trên lịch: đó là con số hiển thị, và trước
// đây sửa được — hoàn theo nó là hoàn theo con số người dùng tự gõ.
func cocDaThu(tx *gorm.DB, booking *model.MachineBooking) (int64, error) {
	if booking.DepositTransactionID == nil || booking.MemberID == nil {
		return 0, nil
	}
	var giaoDich model.MemberTransaction
	if err := tx.Where("id = ?", *booking.DepositTransactionID).First(&giaoDich).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("không tìm thấy giao dịch đặt cọc của lịch này — kiểm tra lại sổ ví trước khi hoàn")
		}
		return 0, err
	}
	if giaoDich.Amount >= 0 {
		return 0, nil
	}
	return -giaoDich.Amount, nil
}

// hoanCocVaoVi trả cọc về ví hội viên (member đã được khoá trong tx) và ghi
// một dòng deposit_refund trỏ về lịch.
func hoanCocVaoVi(tx *gorm.DB, booking *model.MachineBooking, member *model.Member, soTien int64, moTa, userID string) error {
	balanceAfter := member.Balance + soTien
	trans := model.MemberTransaction{
		MemberID:        member.ID,
		TransactionType: "deposit_refund",
		Amount:          soTien,
		BalanceBefore:   member.Balance,
		BalanceAfter:    balanceAfter,
		ReferenceID:     &booking.ID,
		Description:     moTa,
		CreatedBy:       uidHoacNil(userID),
		CreatedAt:       time.Now(),
	}
	if err := tx.Create(&trans).Error; err != nil {
		return err
	}
	if err := tx.Model(member).Update("balance", balanceAfter).Error; err != nil {
		return err
	}
	member.Balance = balanceAfter
	return nil
}

// khoaHoiVienCuaLich khoá hội viên của lịch nếu lịch đang giữ cọc của họ. Gọi
// TRƯỚC khi khoá lịch để đúng thứ tự hội viên → máy → lịch.
func khoaHoiVienCuaLich(tx *gorm.DB, booking *model.MachineBooking) (*model.Member, error) {
	if booking.MemberID == nil || booking.DepositTransactionID == nil {
		return nil, nil
	}
	var member model.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&member, "id = ?", *booking.MemberID).Error; err != nil {
		return nil, errors.New("không tìm thấy hội viên")
	}
	return &member, nil
}

// nhanMayTrongTx chuyển lịch sang checked_in và trả cọc vào ví: khách đã tới,
// cọc thành tiền chơi. Bản cũ chỉ đổi trạng thái nên cọc biến mất khỏi ví mà
// không đi đâu cả. Trả về số đã hoàn.
func nhanMayTrongTx(tx *gorm.DB, booking *model.MachineBooking, member *model.Member, userID string) (int64, error) {
	hoan, err := cocDaThu(tx, booking)
	if err != nil {
		return 0, err
	}
	if hoan > 0 && member != nil {
		moTa := fmt.Sprintf("Hoàn cọc khi nhận máy (lịch đặt %s) — cọc thành tiền chơi", ngayGioVN(booking.BookedFrom))
		if err := hoanCocVaoVi(tx, booking, member, hoan, moTa, userID); err != nil {
			return 0, err
		}
	} else {
		hoan = 0
	}
	if err := tx.Model(booking).Updates(map[string]interface{}{
		"status":     "checked_in",
		"updated_at": time.Now(),
	}).Error; err != nil {
		return 0, err
	}
	booking.Status = "checked_in"
	return hoan, nil
}

// CheckIn nhận máy cho khách. Chỉ trong cửa sổ [giờ bắt đầu − 30 phút, giờ
// kết thúc): sớm hơn thì máy có thể đang bán cho người khác, còn hết giờ rồi
// thì không còn gì để nhận. Thực tế tác vụ nền đã đánh vắng lịch sau giờ bắt
// đầu 15 phút, nên cửa sổ hữu hiệu là tới mốc đó.
func (s *BookingService) CheckIn(id, userID string) (*BookingResponse, error) {
	booking, err := s.docLichDat(id)
	if err != nil {
		return nil, err
	}

	tx := s.db.Begin()
	member, err := khoaHoiVienCuaLich(tx, booking)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	locked, err := khoaLichDat(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if locked.Status != "pending" {
		tx.Rollback()
		return nil, fmt.Errorf("lịch đặt %s, không nhận máy được", tenTrangThaiDatCho(locked.Status))
	}
	now := time.Now()
	if moCua := locked.BookedFrom.Add(-BookingCheckInEarly); now.Before(moCua) {
		tx.Rollback()
		return nil, fmt.Errorf("chưa tới giờ nhận máy — nhận được từ %s (%d phút trước giờ đặt)",
			ngayGioVN(moCua), int(BookingCheckInEarly.Minutes()))
	}
	if !now.Before(locked.BookedTo) {
		tx.Rollback()
		return nil, fmt.Errorf("lịch đặt đã hết giờ lúc %s, không nhận máy được nữa", ngayGioVN(locked.BookedTo))
	}

	hoan, err := nhanMayTrongTx(tx, locked, member, userID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	if member != nil && hoan > 0 {
		phatSoDuMoi(s.hub, member.ID, member.Balance, member.BonusBalance)
	}
	PhatLichDatDoi(s.hub, locked.ID, "checked_in")

	result := bookingToResponse(*locked)
	result.MachineCode = s.machineCode(locked.MachineID)
	s.audit.Log(&LogAuditRequest{
		Action:     "check_in",
		EntityType: "machine_booking",
		EntityID:   id,
		UserID:     uidHoacNil(userID),
		Metadata: map[string]interface{}{
			"status":         "checked_in",
			"refund_amount":  hoan,
			"member_id":      locked.MemberID,
			"deposit_amount": locked.DepositAmount,
		},
	})
	return &result, nil
}

func (s *BookingService) Cancel(id, userID string) (*BookingResponse, error) {
	booking, err := s.docLichDat(id)
	if err != nil {
		return nil, err
	}

	if booking.Status != "pending" {
		return nil, fmt.Errorf("lịch đặt %s, không huỷ được", tenTrangThaiDatCho(booking.Status))
	}
	if err := s.kiemHanHuy(booking.BookedFrom, "huỷ"); err != nil {
		return nil, err
	}

	tx := s.db.Begin()

	// Hội viên trước, lịch sau — cùng thứ tự với StartSession.
	member, err := khoaHoiVienCuaLich(tx, booking)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// Kiểm tra ở trên chạy trên bản đọc ngoài transaction. Hai lệnh huỷ song song
	// đều thấy trạng thái cũ và đều hoàn cọc — khách được trả cọc hai lần. Khoá
	// rồi đọc lại là chỗ duy nhất chặn được.
	locked, err := khoaLichDat(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if locked.Status != "pending" {
		tx.Rollback()
		return nil, fmt.Errorf("lịch đặt %s, không huỷ được", tenTrangThaiDatCho(locked.Status))
	}
	booking = locked

	// Hoàn đúng số đã thu, đọc từ giao dịch cọc. Lịch không có giao dịch cọc
	// (khách vãng lai, hoặc không cọc) thì không có gì để hoàn.
	hoan, err := cocDaThu(tx, booking)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if hoan > 0 && member != nil {
		if err := hoanCocVaoVi(tx, booking, member, hoan, "Hoàn cọc do huỷ lịch đặt máy", userID); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		hoan = 0
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":     "cancelled",
		"cancel_at":  &now,
		"updated_at": now,
	}
	if err := tx.Model(booking).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	if hoan > 0 {
		phatSoDuMoi(s.hub, member.ID, member.Balance, member.BonusBalance)
	}
	PhatLichDatDoi(s.hub, booking.ID, "cancelled")

	booking.Status = "cancelled"
	booking.CancelAt = &now
	result := bookingToResponse(*booking)
	result.MachineCode = s.machineCode(booking.MachineID)
	s.audit.Log(&LogAuditRequest{
		Action:     "cancel",
		EntityType: "machine_booking",
		EntityID:   id,
		UserID:     uidHoacNil(userID),
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"status":         "cancelled",
			"deposit_amount": booking.DepositAmount,
			"refund_amount":  hoan,
			"member_id":      booking.MemberID,
		},
	})
	return &result, nil
}

// NoShow đánh vắng một lịch: khách MẤT cọc, máy được nhả.
//
// Chỉ bấm được sau giờ bắt đầu BookingNoShowGrace — cùng mốc với tác vụ nền
// bookings:expire, để nút ở quầy không đánh vắng sớm hơn luật tự động.
//
// Mất cọc không đụng tới ví lần nữa: tiền đã rời ví từ lúc đặt (dòng
// booking_deposit). Báo cáo doanh thu tính theo tiền VÀO QUÁN nên khoản này đã
// nằm trong doanh thu từ lần nạp ví sinh ra nó — xem txRevenueQuery.
func (s *BookingService) NoShow(id, userID string) (*BookingResponse, error) {
	booking, err := s.docLichDat(id)
	if err != nil {
		return nil, err
	}
	if booking.Status != "pending" {
		return nil, fmt.Errorf("lịch đặt %s, không đánh vắng được", tenTrangThaiDatCho(booking.Status))
	}

	tx := s.db.Begin()
	locked, err := khoaLichDat(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if locked.Status != "pending" {
		tx.Rollback()
		return nil, fmt.Errorf("lịch đặt %s, không đánh vắng được", tenTrangThaiDatCho(locked.Status))
	}
	if moc := locked.BookedFrom.Add(BookingNoShowGrace); time.Now().Before(moc) {
		tx.Rollback()
		return nil, fmt.Errorf("chỉ đánh vắng được từ %s (%d phút sau giờ đặt) — trước đó khách vẫn còn thời gian tới",
			ngayGioVN(moc), int(BookingNoShowGrace.Minutes()))
	}
	matCoc, err := cocDaThu(tx, locked)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Model(locked).Updates(map[string]interface{}{
		"status":     "no_show",
		"updated_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	locked.Status = "no_show"

	PhatLichDatDoi(s.hub, locked.ID, "no_show")

	result := bookingToResponse(*locked)
	result.MachineCode = s.machineCode(locked.MachineID)
	s.audit.Log(&LogAuditRequest{
		Action:     "no_show",
		EntityType: "machine_booking",
		EntityID:   id,
		UserID:     uidHoacNil(userID),
		Metadata: map[string]interface{}{
			"status":            "no_show",
			"member_id":         locked.MemberID,
			"forfeited_deposit": matCoc,
		},
	})
	return &result, nil
}

// PhatLichDatDoi báo cho mọi thiết bị quản trị rằng một lịch đặt vừa đổi trạng
// thái. Không có sự kiện nào cho đặt chỗ, nên nhận máy ở quầy xong thì danh sách
// trên điện thoại của cùng nhân viên đó vẫn hiện lịch cũ.
//
// Xuất ra ngoài vì tác vụ nền bookings:expire cũng đổi trạng thái hàng loạt.
func PhatLichDatDoi(h *hub.Hub, bookingID, status string) {
	if h == nil || bookingID == "" {
		return
	}
	h.BroadcastToType(hub.Event{
		Type: "booking:updated",
		Data: map[string]interface{}{"booking_id": bookingID, "status": status},
	}, hub.ClientTypeAdmin)
}

// nhanMayTuDong là dấu vết một lần StartSession tự nhận máy cho chủ lịch, để
// báo số dư và sự kiện SAU khi transaction của phiên commit.
type nhanMayTuDong struct {
	bookingID string
	memberID  string
	hoan      int64
}

// giuMayTheoLichDat là chốt giữ máy cho lịch đặt, gọi trong transaction mở
// phiên sau khi đã khoá hội viên và máy (đúng thứ tự hội viên → máy → lịch).
//
//   - Từ giờ bắt đầu tới hạn đánh vắng (bắt đầu + BookingNoShowGrace), máy chỉ
//     dành cho chủ lịch. Người khác — kể cả nhân viên mở máy hộ — bị từ chối.
//     Lịch khách vãng lai không có chủ là hội viên nên chặn mọi người; nhân
//     viên nhận máy (check-in) xong thì lịch hết giữ.
//   - Chủ lịch ngồi vào trong cửa sổ nhận máy thì coi như đã tới: nhận máy luôn
//     và trả cọc vào ví. Không làm vậy thì khách đang ngồi chơi mà 15 phút sau
//     tác vụ nền vẫn đánh vắng và nuốt cọc.
//   - Trước giờ bắt đầu, người khác vẫn ngồi được (máy chưa tới giờ giữ).
func giuMayTheoLichDat(tx *gorm.DB, machine *model.Machine, member *model.Member, now time.Time) (*nhanMayTuDong, error) {
	var lich []model.MachineBooking
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("machine_id = ? AND status = ? AND booked_from <= ? AND booked_from > ?",
			machine.ID, "pending", now.Add(BookingCheckInEarly), now.Add(-BookingNoShowGrace)).
		Order("booked_from").Limit(1).Find(&lich).Error; err != nil {
		return nil, err
	}
	if len(lich) == 0 {
		return nil, nil
	}
	b := &lich[0]

	if b.MemberID != nil && *b.MemberID == member.ID {
		hoan, err := nhanMayTrongTx(tx, b, member, "")
		if err != nil {
			return nil, err
		}
		return &nhanMayTuDong{bookingID: b.ID, memberID: member.ID, hoan: hoan}, nil
	}
	if now.Before(b.BookedFrom) {
		return nil, nil
	}
	return nil, fmt.Errorf("máy %s đã được đặt trước đến %s — hãy chọn máy khác",
		machine.MachineCode, gioVN(b.BookedFrom.Add(BookingNoShowGrace)))
}

// bao gửi số dư mới, sự kiện đổi lịch và nhật ký cho lần tự nhận máy.
func (n *nhanMayTuDong) bao(h *hub.Hub, audit *AuditService, member *model.Member) {
	if n == nil {
		return
	}
	if n.hoan > 0 {
		phatSoDuMoi(h, n.memberID, member.Balance, member.BonusBalance)
	}
	PhatLichDatDoi(h, n.bookingID, "checked_in")
	if audit != nil {
		_ = audit.Log(&LogAuditRequest{
			Action:     "check_in",
			EntityType: "machine_booking",
			EntityID:   n.bookingID,
			Metadata: map[string]interface{}{
				"status":        "checked_in",
				"auto":          true,
				"refund_amount": n.hoan,
				"member_id":     n.memberID,
			},
		})
	}
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

// bookingToResponse trả mọi mốc giờ theo giờ Việt Nam. Create trước đây trả
// nguyên múi người gọi gửi lên (UTC từ trang quản trị) trong khi GET trả
// +07:00 — cùng một lịch mà hai lần đọc ra hai chuỗi giờ khác nhau.
func bookingToResponse(b model.MachineBooking) BookingResponse {
	vn := utils.VietnamLocation()
	resp := BookingResponse{
		ID:            b.ID,
		MachineID:     b.MachineID,
		MemberID:      b.MemberID,
		CustomerName:  b.CustomerName,
		CustomerPhone: b.CustomerPhone,
		BookedFrom:    b.BookedFrom.In(vn).Format(time.RFC3339),
		BookedTo:      b.BookedTo.In(vn).Format(time.RFC3339),
		DepositAmount: b.DepositAmount,
		Status:        b.Status,
		Notes:         b.Notes,
		CreatedBy:     b.CreatedBy,
		CreatedAt:     b.CreatedAt.In(vn).Format(time.RFC3339),
		UpdatedAt:     b.UpdatedAt.In(vn).Format(time.RFC3339),
	}
	if b.DepositTransactionID != nil {
		resp.DepositTransactionID = b.DepositTransactionID
	}
	if b.CancelAt != nil {
		s := b.CancelAt.In(vn).Format(time.RFC3339)
		resp.CancelAt = &s
	}
	return resp
}
