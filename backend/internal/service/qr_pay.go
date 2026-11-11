package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

// Nạp tiền bằng QR, tự xác nhận nhờ giao dịch ThueAPI báo qua Hub (bank_sync.go).
//
//  1. Khách (hoặc quầy) xin nạp X đồng bằng QR: tạo đơn nạp chờ như bình thường,
//     kèm một mã nạp (VN + 6 ký tự) và mã VietQR có sẵn số tài khoản quán, số
//     tiền, nội dung là mã nạp.
//  2. Khách chuyển khoản. ThueAPI gọi webhook của Hub, Hub báo quán, quán đọc giao dịch mới.
//  3. match đọc số tiền + mã: khớp đúng một yêu cầu đang chờ và đúng số tiền
//     thì duyệt đơn bằng chính OrderService.Pay (cộng tiền, nạp tặng, báo số dư).
//     Không khớp thì nằm ở trang "Tiền vào" cho nhân viên xử lý.
//
// Không bao giờ tự cộng khi lệch số tiền hay không thấy mã.

const (
	QRPaymentGroup  = "qr_payment"
	PaymentMethodQR = "qr"

	qrKeyEnabled       = "enabled"
	qrKeyBank          = "bank"
	qrKeyAccountNo     = "account_no"
	qrKeyAccountName   = "account_name"
	qrKeyExpireMinutes = "expire_minutes"
	qrKeyAutoConfirm   = "auto_confirm"

	qrDefaultExpireMinutes = 10
	qrMinAmount            = 1000
	qrMaxAmount            = 50_000_000
	// Mã nạp lấy từ bảng chữ không có 0/O, 1/I/L.
	qrCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
)

type QRConfig struct {
	Enabled       bool   `json:"enabled"`
	Bank          string `json:"bank"`
	AccountNo     string `json:"account_no"`
	AccountName   string `json:"account_name"`
	ExpireMinutes int64  `json:"expire_minutes"`
	AutoConfirm   bool   `json:"auto_confirm"`
}

func (c QRConfig) configured() bool {
	_, ok := qrBanks[c.Bank]
	return ok && c.AccountNo != ""
}

func qrConfigFrom(m map[string]string) QRConfig {
	c := QRConfig{
		Enabled:       settingBool(m, qrKeyEnabled, false),
		Bank:          m[qrKeyBank],
		AccountNo:     m[qrKeyAccountNo],
		AccountName:   m[qrKeyAccountName],
		ExpireMinutes: settingInt(m, qrKeyExpireMinutes, qrDefaultExpireMinutes),
		// Mặc định chỉ gợi ý: những ngày đầu nhân viên tự bấm duyệt và đối chiếu sao kê.
		AutoConfirm: settingBool(m, qrKeyAutoConfirm, false),
	}
	if c.ExpireMinutes <= 0 {
		c.ExpireMinutes = qrDefaultExpireMinutes
	}
	return c
}

var reAccountNo = regexp.MustCompile(`^[0-9]{6,19}$`)

// kiemTraCaiDatQR kiểm nhóm cài đặt qr_payment trước khi lưu.
func kiemTraCaiDatQR(in map[string]interface{}) error {
	if v, ok := in[qrKeyBank]; ok {
		if s, _ := v.(string); s != "" {
			if _, known := qrBanks[s]; !known {
				return errors.New("ngân hàng phải là MB hoặc Vietcombank")
			}
		}
	}
	if v, ok := in[qrKeyAccountNo]; ok {
		s, _ := v.(string)
		s = strings.TrimSpace(s)
		if s != "" && !reAccountNo.MatchString(s) {
			return errors.New("số tài khoản chỉ gồm 6–19 chữ số")
		}
		in[qrKeyAccountNo] = s
	}
	if v, ok := in[qrKeyAccountName]; ok {
		s, _ := v.(string)
		if len([]rune(s)) > 50 {
			return errors.New("tên chủ tài khoản tối đa 50 ký tự")
		}
	}
	if v, ok := in[qrKeyExpireMinutes]; ok {
		n, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(v)), 10, 64)
		if err != nil || n < 3 || n > 60 {
			return errors.New("thời hạn mã QR phải từ 3 đến 60 phút")
		}
		in[qrKeyExpireMinutes] = n
	}
	return nil
}

type QRPayService struct {
	db      *gorm.DB
	hub     *hub.Hub
	audit   *AuditService
	orders  *OrderService
	members *MemberService
	now     func() time.Time

	// hubAccess trả địa chỉ Hub và key của quán; trống là quán chưa nối Hub.
	hubAccess func() (url, key string)
	kick      chan struct{} // có tin Hub báo giao dịch mới: đọc ngay, không đợi tới lượt

	mu        sync.Mutex
	lastSync  time.Time // lần cuối đọc được Hub
	wasOnline bool
}

func NewQRPayService(db *gorm.DB, h *hub.Hub, audit *AuditService, orders *OrderService, members *MemberService) *QRPayService {
	return &QRPayService{db: db, hub: h, audit: audit, orders: orders, members: members, now: time.Now, kick: make(chan struct{}, 1)}
}

// WithHubAccess cho biết đọc giao dịch ngân hàng ở đâu. GameService giữ cấu hình Hub của quán.
func (s *QRPayService) WithHubAccess(f func() (url, key string)) *QRPayService {
	s.hubAccess = f
	return s
}

func (s *QRPayService) config() QRConfig { return qrConfigFrom(settingsGroup(s.db, QRPaymentGroup)) }

func (s *QRPayService) changed(topic string) {
	if s.hub != nil {
		s.hub.Changed(topic, nil)
	}
}

// --- Trạng thái -------------------------------------------------------------

type QRStatus struct {
	Enabled      bool       `json:"enabled"`
	Configured   bool       `json:"configured"`
	SourceOnline bool       `json:"source_online"` // đọc được giao dịch từ Hub
	Ready        bool       `json:"ready"`
	AutoConfirm  bool       `json:"auto_confirm"`
	Bank         string     `json:"bank"`
	BankName     string     `json:"bank_name"`
	AccountName  string     `json:"account_name"`
	LastSeen     *time.Time `json:"last_seen"`
	LastNotifAt  *time.Time `json:"last_notification_at"`
	ExpireMinute int64      `json:"expire_minutes"`
}

func (s *QRPayService) sourceOnline(now time.Time) bool {
	return !s.lastSync.IsZero() && now.Sub(s.lastSync) < qrSourceOfflineAfter
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s *QRPayService) Status() QRStatus {
	cfg := s.config()
	var lastNotif *time.Time
	s.db.Unscoped().Model(&model.BankNotification{}).Select("MAX(received_at)").Scan(&lastNotif)
	s.mu.Lock()
	defer s.mu.Unlock()
	online := s.sourceOnline(s.now())
	return QRStatus{
		Enabled:      cfg.Enabled,
		Configured:   cfg.configured(),
		SourceOnline: online,
		Ready:        cfg.Enabled && cfg.configured() && online,
		AutoConfirm:  cfg.AutoConfirm,
		Bank:         cfg.Bank,
		BankName:     qrBanks[cfg.Bank].Name,
		AccountName:  cfg.AccountName,
		LastSeen:     timePtr(s.lastSync),
		LastNotifAt:  lastNotif,
		ExpireMinute: cfg.ExpireMinutes,
	}
}

// --- Tạo mã nạp ---------------------------------------------------------------

type CreateQRTopupRequest struct {
	MemberID    string `json:"member_id"`
	Amount      int64  `json:"amount"`
	MachineCode string `json:"machine_code"`
}

type QRTopupResponse struct {
	RequestID   string    `json:"request_id"`
	OrderID     string    `json:"order_id"`
	OrderCode   string    `json:"order_code"`
	Code        string    `json:"code"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	Bank        string    `json:"bank"`
	BankName    string    `json:"bank_name"`
	AccountNo   string    `json:"account_no"`
	AccountName string    `json:"account_name"`
	Payload     string    `json:"qr_payload"`
	Image       string    `json:"qr_image"`
}

func taoMaNap() (string, error) {
	var b strings.Builder
	b.WriteString("VN")
	max := big.NewInt(int64(len(qrCodeAlphabet)))
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(qrCodeAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// CreateTopup tạo đơn nạp chờ + mã QR cho một hội viên.
func (s *QRPayService) CreateTopup(req CreateQRTopupRequest, createdBy string) (*QRTopupResponse, error) {
	cfg, err := s.qrReady()
	if err != nil {
		return nil, err
	}
	if req.Amount < qrMinAmount || req.Amount > qrMaxAmount {
		return nil, fmt.Errorf("số tiền nạp QR phải từ %sđ đến %sđ", formatMoney(qrMinAmount), formatMoney(qrMaxAmount))
	}
	if !laUUID(req.MemberID) {
		return nil, errors.New("thiếu hội viên")
	}
	var member model.Member
	if err := s.db.Select("id").Where("id = ?", req.MemberID).First(&member).Error; err != nil {
		return nil, errors.New("không tìm thấy hội viên")
	}

	order, err := s.orders.createTopupOrder(CreateTopupOrderRequest{
		MemberID: req.MemberID, Amount: req.Amount, MachineCode: req.MachineCode,
	}, createdBy, PaymentMethodQR, false)
	if err != nil {
		return nil, err
	}

	r, err := s.newRequest(cfg, order.ID, req.MemberID, order.MachineID, req.Amount, createdBy)
	if err != nil {
		return nil, err
	}
	s.audit.Log(&LogAuditRequest{
		Action: "qr_topup_create", EntityType: "order", EntityID: order.ID, UserID: optionalUUID(createdBy),
		Metadata: map[string]interface{}{"order_code": order.OrderCode, "code": r.Code, "amount": req.Amount},
	})
	s.changed("orders")
	return s.respond(cfg, r, order.OrderCode)
}

// qrReady: cùng điều kiện cho mọi mã QR — đã bật, đã cài tài khoản, đang đọc được giao dịch.
func (s *QRPayService) qrReady() (QRConfig, error) {
	cfg := s.config()
	if !cfg.Enabled || !cfg.configured() {
		return cfg, errors.New("quán chưa bật thanh toán bằng QR")
	}
	if !s.Status().SourceOnline {
		return cfg, errors.New("máy chủ quán đang mất kết nối tới Hub, tạm thời không thanh toán bằng QR được")
	}
	return cfg, nil
}

// newRequest tạo một mã QR chờ tiền cho một đơn (đơn nạp, hoặc đơn món đã xác nhận).
func (s *QRPayService) newRequest(cfg QRConfig, orderID, memberID string, machineID *string, amount int64, createdBy string) (*model.QRRequest, error) {
	r := model.QRRequest{
		OrderID:   orderID,
		MemberID:  &memberID,
		MachineID: machineID,
		Amount:    amount,
		Bank:      cfg.Bank,
		AccountNo: cfg.AccountNo,
		Status:    model.QRRequestPending,
		ExpiresAt: s.now().Add(time.Duration(cfg.ExpireMinutes) * time.Minute),
		CreatedBy: optionalUUID(createdBy),
	}
	// Mã trùng một yêu cầu cũ (khả năng rất nhỏ) thì unique index từ chối: thử mã khác.
	var err error
	for i := 0; ; i++ {
		if r.Code, err = taoMaNap(); err != nil {
			return nil, err
		}
		if err = s.db.Create(&r).Error; err == nil {
			return &r, nil
		}
		if i == 4 {
			return nil, err
		}
	}
}

func (s *QRPayService) respond(cfg QRConfig, r *model.QRRequest, orderCode string) (*QRTopupResponse, error) {
	bank := qrBanks[cfg.Bank]
	payload := vietQRPayload(bank.BIN, r.AccountNo, r.Amount, r.Code)
	image, err := qrPNGDataURL(payload, 512)
	if err != nil {
		return nil, err
	}
	return &QRTopupResponse{
		RequestID: r.ID, OrderID: r.OrderID, OrderCode: orderCode, Code: r.Code, Amount: r.Amount,
		Status: r.Status, ExpiresAt: r.ExpiresAt, Bank: bank.Code, BankName: bank.Name,
		AccountNo: r.AccountNo, AccountName: cfg.AccountName, Payload: payload, Image: image,
	}, nil
}

// PayOrder: hội viên trả bằng QR một đơn món của chính mình đã được quầy xác
// nhận mà chưa thanh toán. Số tiền là của đơn, không nhận từ máy trạm. Đơn đang
// có mã còn hạn thì trả lại đúng mã đó, không sinh mã thứ hai cho cùng một đơn.
func (s *QRPayService) PayOrder(orderID, memberID string) (*QRTopupResponse, error) {
	cfg, err := s.qrReady()
	if err != nil {
		return nil, err
	}
	var order model.Order
	if !laUUID(orderID) || s.db.Where("id = ?", orderID).First(&order).Error != nil {
		return nil, errors.New("không tìm thấy đơn hàng")
	}
	if order.MemberID == nil || *order.MemberID != memberID {
		return nil, errors.New("không phải đơn của bạn")
	}
	if order.OrderType == model.OrderTypeTopup {
		return nil, errors.New("đơn nạp tiền dùng nút Nạp tiền")
	}
	if order.Status != "confirmed" {
		return nil, errors.New("chỉ trả bằng QR được đơn đã xác nhận mà chưa thanh toán")
	}
	if order.FinalAmount < qrMinAmount || order.FinalAmount > qrMaxAmount {
		return nil, fmt.Errorf("đơn %sđ không trả bằng QR được (từ %sđ đến %sđ)", formatMoney(order.FinalAmount), formatMoney(qrMinAmount), formatMoney(qrMaxAmount))
	}

	var r model.QRRequest
	err = s.db.Where("order_id = ? AND status = ? AND expires_at > ? AND amount = ?", order.ID, model.QRRequestPending, s.now(), order.FinalAmount).
		Order("created_at DESC").First(&r).Error
	if err != nil {
		created, err := s.newRequest(cfg, order.ID, memberID, order.MachineID, order.FinalAmount, memberID)
		if err != nil {
			return nil, err
		}
		r = *created
		s.audit.Log(&LogAuditRequest{
			Action: "qr_order_pay_create", EntityType: "order", EntityID: order.ID, UserID: optionalUUID(memberID),
			Metadata: map[string]interface{}{"order_code": order.OrderCode, "code": r.Code, "amount": r.Amount},
		})
	}
	return s.respond(cfg, &r, order.OrderCode)
}

// GetRequest: máy trạm hỏi lại trạng thái khi lỡ sự kiện WebSocket.
func (s *QRPayService) GetRequest(id string) (*model.QRRequest, error) {
	var r model.QRRequest
	if !laUUID(id) || s.db.Where("id = ?", id).First(&r).Error != nil {
		return nil, errors.New("không tìm thấy mã nạp")
	}
	return &r, nil
}

// Cancel: khách đóng hộp QR, hoặc quầy huỷ. Chỉ huỷ được khi tiền chưa về.
func (s *QRPayService) Cancel(id, actorID string) error {
	r, err := s.GetRequest(id)
	if err != nil {
		return err
	}
	res := s.db.Model(&model.QRRequest{}).Where("id = ? AND status = ?", r.ID, model.QRRequestPending).
		Updates(map[string]interface{}{"status": model.QRRequestCancelled, "updated_at": s.now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("mã nạp không còn chờ tiền")
	}
	return s.cancelOrder(r.OrderID, actorID)
}

// cancelOrder huỷ đơn nạp của một mã QR. Có người bấm thì đi đường huỷ đơn
// thường (ghi tên người huỷ); hệ thống tự huỷ (mã hết hạn) thì không có ai để
// ghi vào updated_by, nên chỉ đổi trạng thái — đơn nạp chờ chưa động tới kho
// hay tiền nên không có gì phải hoàn.
func (s *QRPayService) cancelOrder(orderID, actorID string) error {
	// Đơn món (khách trả đơn đã xác nhận bằng QR) không bị huỷ theo mã: huỷ hay
	// hết hạn mã chỉ là khách chưa trả, đơn vẫn chờ thanh toán ở quầy.
	var order model.Order
	if err := s.db.Select("id", "order_type").Where("id = ?", orderID).First(&order).Error; err != nil {
		return err
	}
	if order.OrderType != model.OrderTypeTopup {
		return nil
	}
	if actorID != "" {
		_, err := s.orders.UpdateStatus(orderID, actorID, UpdateStatusRequest{Status: "cancelled"})
		return err
	}
	err := s.db.Model(&model.Order{}).
		Where("id = ? AND status = ? AND order_type = ?", orderID, "pending", model.OrderTypeTopup).
		Update("status", "cancelled").Error
	if err == nil {
		s.changed("orders")
	}
	return err
}

// ExpireDue chạy mỗi phút: mã quá hạn thì huỷ cùng đơn nạp. Tiền tới muộn vẫn
// được ghi ở "Tiền vào" kèm gợi ý đúng hội viên.
func (s *QRPayService) ExpireDue() (int, error) {
	var due []model.QRRequest
	if err := s.db.Where("status = ? AND expires_at < ?", model.QRRequestPending, s.now()).Find(&due).Error; err != nil {
		return 0, err
	}
	n := 0
	for _, r := range due {
		// Giành từng mã: tiền về đúng lúc này (payRequest) thì bên đó thắng.
		res := s.db.Model(&model.QRRequest{}).Where("id = ? AND status = ?", r.ID, model.QRRequestPending).
			Updates(map[string]interface{}{"status": model.QRRequestExpired, "updated_at": s.now()})
		if res.Error != nil || res.RowsAffected == 0 {
			continue
		}
		n++
		if err := s.cancelOrder(r.OrderID, ""); err != nil {
			log.Printf("[qr] huỷ đơn nạp %s của mã hết hạn %s: %v", r.OrderID, r.Code, err)
		}
		if r.MemberID != nil && s.hub != nil {
			s.hub.SendToUser(*r.MemberID, hub.Event{Type: "qr:expired", Data: map[string]interface{}{"request_id": r.ID}})
		}
	}
	if n > 0 {
		// Nhật ký mã QR ở trang "Tiền vào"; hết hạn không đi qua yêu cầu HTTP nào.
		s.changed("qr")
	}
	return n, nil
}

// errQROrderPaidElsewhere: đơn của mã đã được trả bằng cách khác (quầy thu tiền
// mặt, duyệt tay) trước khi tiền QR về — tiền này đang thừa, phải để nhân viên xử lý.
var errQROrderPaidElsewhere = errors.New("đơn đã được thanh toán bằng cách khác trước khi tiền QR về — tiền này chưa cộng cho ai")

// match tìm mã nạp trong nội dung và quyết định: duyệt, gợi ý, hay để nhân viên.
func (s *QRPayService) match(n *model.BankNotification) {
	codes := qrCodesIn(n.Content)
	if len(codes) == 0 {
		return
	}
	var reqs []model.QRRequest
	if s.db.Where("code IN ?", codes).Order("created_at DESC").Find(&reqs).Error != nil || len(reqs) == 0 {
		return
	}
	r := reqs[0]
	for _, x := range reqs {
		if x.Status == model.QRRequestPending {
			r = x
			break
		}
	}
	updates := map[string]interface{}{
		"code": r.Code, "qr_request_id": r.ID, "order_id": r.OrderID, "member_id": r.MemberID, "updated_at": s.now(),
	}
	n.Code, n.QRRequestID, n.OrderID, n.MemberID = r.Code, &r.ID, &r.OrderID, r.MemberID

	switch {
	case r.Status != model.QRRequestPending:
		n.Note = map[string]string{
			model.QRRequestExpired:   "mã đã hết hạn, tiền tới muộn",
			model.QRRequestCancelled: "mã đã huỷ",
			model.QRRequestPaid:      "mã này đã được trả một lần",
		}[r.Status]
	case n.Amount != r.Amount:
		n.Status = model.BankNotifMismatch
		n.Note = fmt.Sprintf("chuyển %sđ, mã nạp %sđ", formatMoney(n.Amount), formatMoney(r.Amount))
	case !s.config().AutoConfirm:
		n.Status = model.BankNotifSuggested
	default:
		if err := s.payRequest(&r, n, ""); errors.Is(err, errQROrderPaidElsewhere) {
			n.Note = err.Error()
		} else if err != nil {
			n.Status, n.Note = model.BankNotifSuggested, "tự duyệt lỗi: "+err.Error()
		} else {
			n.Status = model.BankNotifMatched
		}
	}
	updates["status"], updates["note"] = n.Status, n.Note
	s.db.Model(&model.BankNotification{}).Where("id = ?", n.ID).Updates(updates)
}

// payRequest giành mã nạp (chỉ một bên thắng nếu tự duyệt và nhân viên bấm cùng
// lúc) rồi duyệt đơn bằng đường thanh toán đơn nạp có sẵn.
func (s *QRPayService) payRequest(r *model.QRRequest, n *model.BankNotification, actorID string) error {
	now := s.now()
	res := s.db.Model(&model.QRRequest{}).Where("id = ? AND status = ?", r.ID, model.QRRequestPending).
		Updates(map[string]interface{}{"status": model.QRRequestPaid, "paid_at": now, "bank_notification_id": n.ID, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("mã nạp không còn chờ tiền")
	}
	_, err := s.orders.pay(r.OrderID, PayRequest{PaymentMethod: PaymentMethodQR, Amount: r.Amount, ReferenceCode: clip(n.BankRef+" "+r.Code, 60)}, actorID, true)
	if err != nil && strings.Contains(err.Error(), "đã hoàn tất") {
		s.db.Model(&model.QRRequest{}).Where("id = ?", r.ID).
			Updates(map[string]interface{}{"status": model.QRRequestCancelled, "paid_at": nil, "bank_notification_id": nil})
		return errQROrderPaidElsewhere
	}
	if err != nil {
		// Trả mã về chờ để nhân viên xử lý được.
		s.db.Model(&model.QRRequest{}).Where("id = ? AND status = ?", r.ID, model.QRRequestPaid).
			Updates(map[string]interface{}{"status": model.QRRequestPending, "paid_at": nil, "bank_notification_id": nil})
		return err
	}
	if s.hub != nil {
		ev := hub.Event{Type: "qr:paid", Data: map[string]interface{}{"request_id": r.ID, "order_id": r.OrderID, "amount": r.Amount}}
		if r.MemberID != nil {
			s.hub.SendToUser(*r.MemberID, ev)
		}
		s.hub.BroadcastToType(ev, hub.ClientTypeAdmin)
	}
	s.audit.Log(&LogAuditRequest{
		Action: "qr_topup_paid", EntityType: "order", EntityID: r.OrderID, UserID: optionalUUID(actorID),
		Metadata: map[string]interface{}{"code": r.Code, "amount": r.Amount, "bank_ref": n.BankRef, "auto": actorID == ""},
	})
	return nil
}

// --- Trang "Tiền vào" ---------------------------------------------------------

func (s *QRPayService) claimNotification(id string, from []string, to, actorID string) (*model.BankNotification, error) {
	var n model.BankNotification
	if !laUUID(id) || s.db.Where("id = ?", id).First(&n).Error != nil {
		return nil, errors.New("không tìm thấy khoản tiền vào")
	}
	res := s.db.Model(&model.BankNotification{}).Where("id = ? AND status IN ?", id, from).
		Updates(map[string]interface{}{"status": to, "handled_by": optionalUUID(actorID), "updated_at": s.now()})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, errors.New("khoản này đã được xử lý")
	}
	return &n, nil
}

// Confirm: tự duyệt đang tắt (hoặc tự duyệt lỗi), nhân viên xem rồi bấm duyệt.
func (s *QRPayService) Confirm(id, actorID string) error {
	n, err := s.claimNotification(id, []string{model.BankNotifSuggested}, model.BankNotifMatched, actorID)
	if err != nil {
		return err
	}
	var r model.QRRequest
	if n.QRRequestID == nil || s.db.Where("id = ?", *n.QRRequestID).First(&r).Error != nil {
		err = errors.New("không tìm thấy mã nạp của khoản này")
	} else {
		err = s.payRequest(&r, n, actorID)
	}
	if err != nil {
		// Đơn đã trả bằng cách khác: khoản tiền này thành "chưa khớp" để cộng tay hoặc hoàn lại.
		back := model.BankNotifSuggested
		if errors.Is(err, errQROrderPaidElsewhere) {
			back = model.BankNotifUnmatched
		}
		s.db.Model(&model.BankNotification{}).Where("id = ?", id).Updates(map[string]interface{}{"status": back, "note": err.Error()})
		s.changed("bank-notifications")
		return err
	}
	s.changed("bank-notifications")
	return nil
}

// Assign: cộng một khoản chưa khớp (hoặc lệch số tiền) cho hội viên, đúng số
// tiền đã chuyển. Mã nạp còn chờ của khoản đó bị huỷ để không cộng hai lần.
func (s *QRPayService) Assign(id, memberID, actorID string) error {
	if !laUUID(memberID) {
		return errors.New("chọn hội viên để cộng tiền")
	}
	n, err := s.claimNotification(id, []string{model.BankNotifUnmatched, model.BankNotifMismatch}, model.BankNotifAssigned, actorID)
	if err != nil {
		return err
	}
	if !n.Incoming || n.Amount <= 0 {
		s.db.Model(&model.BankNotification{}).Where("id = ?", id).Update("status", n.Status)
		return errors.New("khoản này không phải tiền vào")
	}
	desc := strings.TrimSpace("Tiền vào QR " + n.BankRef + " " + n.Code)
	if _, err := s.members.Topup(memberID, &TopupRequest{Amount: n.Amount, PaymentMethod: PaymentMethodQR, Description: desc, IdempotencyKey: "bank-notif-" + n.ID}, actorID); err != nil {
		s.db.Model(&model.BankNotification{}).Where("id = ?", id).Update("status", n.Status)
		return err
	}
	if n.QRRequestID != nil {
		_ = s.Cancel(*n.QRRequestID, actorID)
	}
	s.db.Model(&model.BankNotification{}).Where("id = ?", id).Update("member_id", memberID)
	s.changed("bank-notifications")
	return nil
}

func (s *QRPayService) Ignore(id, actorID, note string) error {
	_, err := s.claimNotification(id, []string{model.BankNotifUnmatched, model.BankNotifMismatch, model.BankNotifUnparsed, model.BankNotifSuggested}, model.BankNotifIgnored, actorID)
	if err != nil {
		return err
	}
	if note = strings.TrimSpace(note); note != "" {
		s.db.Model(&model.BankNotification{}).Where("id = ?", id).Update("note", clip(note, 500))
	}
	s.changed("bank-notifications")
	return nil
}

// DeleteNotification ẩn một dòng khỏi trang Tiền vào. Khoản đã cộng cho hội viên thì
// không xoá được: xoá không lấy lại được tiền, chỉ làm mất dấu để đối chiếu sao kê.
func (s *QRPayService) DeleteNotification(id, actorID string) error {
	var n model.BankNotification
	if !laUUID(id) || s.db.Where("id = ?", id).First(&n).Error != nil {
		return errors.New("không tìm thấy khoản tiền vào")
	}
	res := s.db.Where("id = ? AND status NOT IN ?", id, []string{model.BankNotifMatched, model.BankNotifAssigned}).
		Delete(&model.BankNotification{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("khoản này đã cộng tiền cho hội viên, không xoá được")
	}
	s.audit.Log(&LogAuditRequest{
		Action: "bank_notification_delete", EntityType: "bank_notification", EntityID: id, UserID: optionalUUID(actorID),
		Description: fmt.Sprintf("Xoá khoản tiền vào %s: %s", formatMoney(n.Amount), clip(n.Content, 120)),
		Metadata: map[string]interface{}{"amount": n.Amount, "incoming": n.Incoming, "status": n.Status, "bank_ref": n.BankRef, "content": n.Content},
	})
	s.changed("bank-notifications")
	return nil
}

type BankNotificationFilter struct {
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Status   string `form:"status"`
}

type BankNotificationRow struct {
	model.BankNotification
	MemberName    string `json:"member_name"`
	OrderCode     string `json:"order_code"`
	HandledByName string `json:"handled_by_name"`
}

var bankNotifSortColumns = map[string]string{
	"received_at": "n.received_at",
	"amount":      "n.amount",
	"status":      "n.status",
	"code":        "n.code",
	"member_name": "member_name",
}

// List trả thêm tổng tiền vào của mọi dòng khớp bộ lọc, không chỉ trang đang xem.
func (s *QRPayService) List(f BankNotificationFilter, params pagination.Params) ([]BankNotificationRow, int64, int64, error) {
	q := s.db.Table("bank_notifications AS n").
		Joins("LEFT JOIN members m ON m.id = n.member_id").
		Joins("LEFT JOIN orders o ON o.id = n.order_id").
		Joins("LEFT JOIN users u ON u.id = n.handled_by").
		Where("n.deleted_at IS NULL")
	q = locKhoangNgay(q, "n.received_at", f.DateFrom, f.DateTo)
	if f.Status != "" {
		q = q.Where("n.status = ?", f.Status)
	}
	if kw := strings.TrimSpace(params.Search); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("unaccent(n.text) ILIKE unaccent(?) OR n.code ILIKE ? OR m.username ILIKE ? OR unaccent(COALESCE(m.full_name,'')) ILIKE unaccent(?)", like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}
	// Chạy trên bản sao (Session): Scan thẳng trên q làm câu lấy dòng bên dưới lặp JOIN.
	var totalIn int64
	if err := q.Session(&gorm.Session{}).Select("COALESCE(SUM(n.amount) FILTER (WHERE n.incoming), 0)").Scan(&totalIn).Error; err != nil {
		return nil, 0, 0, err
	}
	var rows []BankNotificationRow
	err := q.Select("n.*, COALESCE(NULLIF(m.username,''), m.full_name, '') AS member_name, COALESCE(o.order_code,'') AS order_code, COALESCE(u.full_name, u.username, '') AS handled_by_name").
		Order(pagination.OrderBy(&params, bankNotifSortColumns, "n.received_at DESC")).
		Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).
		Scan(&rows).Error
	return rows, total, totalIn, err
}

// --- Nhật ký mã QR đã tạo -------------------------------------------------------

type QRRequestFilter struct {
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Status   string `form:"status"`
}

type QRRequestRow struct {
	model.QRRequest
	MemberName    string `json:"member_name"`
	MachineName   string `json:"machine_name"`
	OrderCode     string `json:"order_code"`
	OrderType     string `json:"order_type"`
	CreatedByName string `json:"created_by_name"`
	// Lúc máy chủ nhận thông báo tiền về của mã này (khác paid_at khi nhân viên duyệt tay).
	BankReceivedAt *time.Time `json:"bank_received_at"`
	// Giây từ lúc tạo mã tới lúc cộng tiền; null khi chưa trả.
	DurationSeconds *int64 `json:"duration_seconds"`
}

// Mã chưa trả (paid_at rỗng) luôn nằm cuối, sắp tăng hay giảm cũng vậy.
var qrRequestSortColumns = map[string]string{
	"created_at":       "r.created_at",
	"amount":           "r.amount",
	"status":           "r.status",
	"code":             "r.code",
	"member_name":      "member_name",
	"paid_at":          "r.paid_at IS NULL, r.paid_at",
	"duration_seconds": "r.paid_at IS NULL, r.paid_at - r.created_at",
}

// ListRequests: mọi mã QR đã đưa cho khách, để xem từ lúc tạo tới lúc tiền vào mất bao lâu.
func (s *QRPayService) ListRequests(f QRRequestFilter, params pagination.Params) ([]QRRequestRow, int64, error) {
	q := s.db.Table("qr_requests AS r").
		Joins("LEFT JOIN members m ON m.id = r.member_id").
		Joins("LEFT JOIN machines mc ON mc.id = r.machine_id").
		Joins("LEFT JOIN orders o ON o.id = r.order_id").
		Joins("LEFT JOIN users u ON u.id = r.created_by").
		Joins("LEFT JOIN bank_notifications b ON b.id = r.bank_notification_id")
	q = locKhoangNgay(q, "r.created_at", f.DateFrom, f.DateTo)
	if f.Status != "" {
		q = q.Where("r.status = ?", f.Status)
	}
	if kw := strings.TrimSpace(params.Search); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("r.code ILIKE ? OR o.order_code ILIKE ? OR m.username ILIKE ? OR unaccent(COALESCE(m.full_name,'')) ILIKE unaccent(?)", like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []QRRequestRow
	err := q.Select("r.*, COALESCE(NULLIF(m.username,''), m.full_name, '') AS member_name, COALESCE(mc.machine_code,'') AS machine_name, " +
		"COALESCE(o.order_code,'') AS order_code, COALESCE(o.order_type,'') AS order_type, " +
		"COALESCE(NULLIF(u.full_name,''), u.username, '') AS created_by_name, b.received_at AS bank_received_at, " +
		"EXTRACT(EPOCH FROM r.paid_at - r.created_at)::bigint AS duration_seconds").
		Order(pagination.OrderBy(&params, qrRequestSortColumns, "r.created_at DESC")).
		Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).
		Scan(&rows).Error
	return rows, total, err
}
