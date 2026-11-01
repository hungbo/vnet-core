package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Nạp tiền bằng QR, tự xác nhận nhờ điện thoại ở quầy (app vnet-bank-notification).
//
//  1. Khách (hoặc quầy) xin nạp X đồng bằng QR: tạo đơn nạp chờ như bình thường,
//     kèm một mã nạp (VN + 6 ký tự) và mã VietQR có sẵn số tài khoản quán, số
//     tiền, nội dung là mã nạp.
//  2. Khách chuyển khoản. App ngân hàng trên điện thoại ở quầy hiện thông báo;
//     app vnet-bank-notification gửi nguyên văn về Ingest.
//  3. Ingest đọc số tiền + mã: khớp đúng một yêu cầu đang chờ và đúng số tiền
//     thì duyệt đơn bằng chính OrderService.Pay (cộng tiền, nạp tặng, báo số dư).
//     Không khớp thì nằm ở trang "Tiền vào" cho nhân viên xử lý.
//
// Không bao giờ tự cộng khi lệch số tiền hay không thấy mã.

const (
	QRPaymentGroup  = "qr_payment"
	QRDeviceKeyKey  = "device_key" // bí mật: không bao giờ trả qua API cài đặt
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
	// Điện thoại gửi nhịp mỗi 60 giây; quá 3 phút không thấy là mất kết nối.
	qrPhoneOfflineAfter = 3 * time.Minute
	// WebSocket của điện thoại: máy chủ ping mỗi 10 giây, 25 giây không nghe gì
	// là kết nối chết (điện thoại mất mạng không kịp đóng socket); rớt quá 10
	// giây mới báo mất kết nối.
	qrWSPingEvery = 10 * time.Second
	qrWSReadWait  = 25 * time.Second
	qrWSGrace     = 10 * time.Second
	qrPairCodeTTL = 5 * time.Minute
	// Mã ghép chỉ 6 số và route ghép không cần đăng nhập: sai bấy nhiêu lần là
	// huỷ mã, không thì người trong mạng quán dò hết 1 triệu mã trong 5 phút,
	// lấy được khoá điện thoại rồi tự "báo tiền về" cho mã QR của chính mình.
	qrPairMaxFails  = 5
	qrSignatureSkew = 5 * time.Minute
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
	if _, ok := in[QRDeviceKeyKey]; ok {
		return errors.New("khoá điện thoại chỉ đặt được bằng cách ghép điện thoại")
	}
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

type qrPhoneState struct {
	LastSeen     time.Time
	LastNotifAt  time.Time
	AppVersion   string
	Queue        int
	Battery      int
	Charging     bool
	ListenerOK   bool
	BankPackages []string
}

type QRPayService struct {
	db      *gorm.DB
	hub     *hub.Hub
	audit   *AuditService
	orders  *OrderService
	members *MemberService
	now     func() time.Time

	mu          sync.Mutex
	pairCode    string
	pairExpires time.Time
	pairFails   int
	phone       qrPhoneState
	wasOnline   bool

	// Kết nối WebSocket của điện thoại (ServeDevice). Có kết nối là online; rớt
	// thì sau qrWSGrace báo mất kết nối ngay, không đợi 3 phút không có nhịp.
	// wsCapable: điện thoại này từng nối WebSocket (app cũ chỉ có nhịp HTTP).
	wsConn    *websocket.Conn
	wsCapable bool
	wsLostAt  time.Time
}

func NewQRPayService(db *gorm.DB, h *hub.Hub, audit *AuditService, orders *OrderService, members *MemberService) *QRPayService {
	return &QRPayService{db: db, hub: h, audit: audit, orders: orders, members: members, now: time.Now}
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
	Paired       bool       `json:"paired"`
	PhoneOnline  bool       `json:"phone_online"`
	Ready        bool       `json:"ready"`
	AutoConfirm  bool       `json:"auto_confirm"`
	Bank         string     `json:"bank"`
	BankName     string     `json:"bank_name"`
	AccountName  string     `json:"account_name"`
	LastSeen     *time.Time `json:"last_seen"`
	LastNotifAt  *time.Time `json:"last_notification_at"`
	AppVersion   string     `json:"app_version"`
	Queue        int        `json:"queue"`
	Battery      int        `json:"battery"`
	Charging     bool       `json:"charging"`
	ListenerOK   bool       `json:"listener_ok"`
	ExpireMinute int64      `json:"expire_minutes"`
	WSConnected  bool       `json:"ws_connected"`
}

func (s *QRPayService) phoneOnline(now time.Time) bool {
	if s.wsConn != nil {
		return true
	}
	if s.wsCapable {
		// Rớt Wi-Fi vài giây hay máy chủ vừa khởi động lại: app tự nối lại ngay,
		// chưa coi là mất kết nối.
		return now.Sub(s.wsLostAt) < qrWSGrace
	}
	return !s.phone.LastSeen.IsZero() && now.Sub(s.phone.LastSeen) < qrPhoneOfflineAfter
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s *QRPayService) Status() QRStatus {
	cfg := s.config()
	paired := s.deviceKey() != ""
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	online := paired && s.phoneOnline(now)
	lastNotif := s.phone.LastNotifAt
	if lastNotif.IsZero() {
		// Máy chủ vừa khởi động lại: lấy từ sổ thay vì báo "chưa có".
		var t *time.Time
		s.db.Model(&model.BankNotification{}).Select("MAX(received_at)").Scan(&t)
		if t != nil {
			lastNotif = *t
		}
	}
	st := QRStatus{
		Enabled:      cfg.Enabled,
		Configured:   cfg.configured(),
		Paired:       paired,
		PhoneOnline:  online,
		Ready:        cfg.Enabled && cfg.configured() && online,
		AutoConfirm:  cfg.AutoConfirm,
		Bank:         cfg.Bank,
		BankName:     qrBanks[cfg.Bank].Name,
		AccountName:  cfg.AccountName,
		LastSeen:     timePtr(s.phone.LastSeen),
		LastNotifAt:  timePtr(lastNotif),
		AppVersion:   s.phone.AppVersion,
		Queue:        s.phone.Queue,
		Battery:      s.phone.Battery,
		Charging:     s.phone.Charging,
		ListenerOK:   s.phone.ListenerOK,
		WSConnected:  s.wsConn != nil,
		ExpireMinute: cfg.ExpireMinutes,
	}
	return st
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

// qrReady: cùng điều kiện cho mọi mã QR — đã bật, đã cài tài khoản, điện thoại online.
func (s *QRPayService) qrReady() (QRConfig, error) {
	cfg := s.config()
	if !cfg.Enabled || !cfg.configured() {
		return cfg, errors.New("quán chưa bật thanh toán bằng QR")
	}
	if !s.Status().PhoneOnline {
		return cfg, errors.New("điện thoại báo tiền ở quầy đang mất kết nối, tạm thời không thanh toán bằng QR được")
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
	s.watchPhone()
	return n, nil
}

// watchPhone báo trang quản lý và máy trạm khi điện thoại vừa mất/có lại kết nối.
func (s *QRPayService) watchPhone() {
	s.mu.Lock()
	online := s.phoneOnline(s.now())
	flipped := online != s.wasOnline
	s.wasOnline = online
	s.mu.Unlock()
	if flipped {
		s.changed("qr")
	}
}

// --- Ghép điện thoại -----------------------------------------------------------

func (s *QRPayService) deviceKey() string {
	var row model.SystemSetting
	if s.db.Where("group_name = ? AND key = ?", QRPaymentGroup, QRDeviceKeyKey).First(&row).Error != nil {
		return ""
	}
	return decodeSettingValue(row.Value)
}

// StartPairing: quầy bấm "Ghép điện thoại", màn hình hiện mã 6 số sống 5 phút.
func (s *QRPayService) StartPairing(actorID string) (string, time.Time, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", time.Time{}, err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	s.mu.Lock()
	s.pairCode, s.pairExpires, s.pairFails = code, s.now().Add(qrPairCodeTTL), 0
	expires := s.pairExpires
	s.mu.Unlock()
	s.audit.Log(&LogAuditRequest{Action: "qr_pairing_start", EntityType: "system_setting", UserID: optionalUUID(actorID)})
	return code, expires, nil
}

type QRPairResponse struct {
	DeviceKey string            `json:"device_key"`
	Banks     map[string]string `json:"banks"` // tên gói app → mã ngân hàng
}

// Pair: điện thoại gửi mã 6 số, nhận khoá riêng dùng ký mọi lần gửi về sau.
// Ghép lại là thay khoá: điện thoại cũ hết gửi được.
func (s *QRPayService) Pair(code string) (*QRPairResponse, error) {
	s.mu.Lock()
	ok := s.pairCode != "" && hmac.Equal([]byte(s.pairCode), []byte(strings.TrimSpace(code))) && s.now().Before(s.pairExpires)
	if ok {
		s.pairCode = ""
	} else if s.pairCode != "" {
		s.pairFails++
		if s.pairFails >= qrPairMaxFails {
			s.pairCode = ""
		}
	}
	s.mu.Unlock()
	if !ok {
		return nil, errors.New("mã ghép sai hoặc đã hết hạn — sai 5 lần thì phải bấm Ghép điện thoại để lấy mã mới")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	key := hex.EncodeToString(raw)
	value := `"` + key + `"`
	row := model.SystemSetting{GroupName: QRPaymentGroup, Key: QRDeviceKeyKey, Value: value}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group_name"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"value": value, "updated_at": s.now()}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	s.dropDevice()
	s.audit.Log(&LogAuditRequest{Action: "qr_pairing_done", EntityType: "system_setting"})
	s.changed("qr")
	banks := map[string]string{}
	for code, b := range qrBanks {
		banks[b.Package] = code
	}
	return &QRPairResponse{DeviceKey: key, Banks: banks}, nil
}

func (s *QRPayService) Unpair(actorID string) error {
	if err := s.db.Where("group_name = ? AND key = ?", QRPaymentGroup, QRDeviceKeyKey).
		Delete(&model.SystemSetting{}).Error; err != nil {
		return err
	}
	s.dropDevice()
	s.audit.Log(&LogAuditRequest{Action: "qr_unpair", EntityType: "system_setting", UserID: optionalUUID(actorID)})
	s.changed("qr")
	return nil
}

// dropDevice quên điện thoại cũ (ghép lại hoặc gỡ): xoá trạng thái, đóng kết nối
// WebSocket đang mở — khoá cũ hết hiệu lực thì kết nối của nó cũng vậy.
func (s *QRPayService) dropDevice() {
	s.mu.Lock()
	s.phone = qrPhoneState{}
	conn := s.wsConn
	s.wsConn, s.wsCapable = nil, false
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// ServeDevice giữ kết nối WebSocket của điện thoại cho tới khi nó rớt. Điện
// thoại gửi trạng thái (pin, hàng đợi...) theo nhịp; máy chủ ping để biết kết
// nối còn sống. Có kết nối và mất kết nối đều báo trang quản lý ngay.
func (s *QRPayService) ServeDevice(conn *websocket.Conn) {
	now := s.now()
	s.mu.Lock()
	old := s.wsConn
	s.wsConn, s.wsCapable = conn, true
	s.phone.LastSeen = now
	s.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	s.watchPhone()

	conn.SetReadLimit(64 << 10)
	_ = conn.SetReadDeadline(now.Add(qrWSReadWait))
	conn.SetPongHandler(func(string) error {
		s.mu.Lock()
		s.phone.LastSeen = s.now()
		s.mu.Unlock()
		return conn.SetReadDeadline(s.now().Add(qrWSReadWait))
	})
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(qrWSPingEvery)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
					return
				}
			}
		}
	}()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_ = conn.SetReadDeadline(s.now().Add(qrWSReadWait))
		var hb QRHeartbeat
		if json.Unmarshal(msg, &hb) == nil {
			s.Heartbeat(hb)
		}
	}
	close(done)
	_ = conn.Close()
	s.mu.Lock()
	if s.wsConn == conn {
		s.wsConn, s.wsLostAt = nil, s.now()
	}
	s.mu.Unlock()
	time.AfterFunc(qrWSGrace+time.Second, s.watchPhone)
}

var ErrQRBadSignature = errors.New("chữ ký không hợp lệ")

// errQROrderPaidElsewhere: đơn của mã đã được trả bằng cách khác (quầy thu tiền
// mặt, duyệt tay) trước khi tiền QR về — tiền này đang thừa, phải để nhân viên xử lý.
var errQROrderPaidElsewhere = errors.New("đơn đã được thanh toán bằng cách khác trước khi tiền QR về — tiền này chưa cộng cho ai")

// VerifyDevice kiểm chữ ký HMAC-SHA256(khoá, timestamp + "\n" + body) của điện
// thoại. Khoá không bao giờ đi trên mạng sau lúc ghép; timestamp chặn gửi lại.
func (s *QRPayService) VerifyDevice(timestamp, signature string, body []byte) error {
	key := s.deviceKey()
	if key == "" {
		return errors.New("chưa ghép điện thoại")
	}
	ms, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrQRBadSignature
	}
	if d := s.now().Sub(time.UnixMilli(ms)); d > qrSignatureSkew || d < -qrSignatureSkew {
		return errors.New("đồng hồ điện thoại lệch quá 5 phút")
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp + "\n"))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(signature))) {
		return ErrQRBadSignature
	}
	return nil
}

type QRHeartbeat struct {
	AppVersion   string   `json:"app_version"`
	Queue        int      `json:"queue"`
	Battery      int      `json:"battery"`
	Charging     bool     `json:"charging"`
	ListenerOK   bool     `json:"listener_ok"`
	BankPackages []string `json:"bank_packages"`
}

func (s *QRPayService) Heartbeat(in QRHeartbeat) QRStatus {
	s.mu.Lock()
	last := s.phone.LastNotifAt
	s.phone = qrPhoneState{
		LastSeen: s.now(), LastNotifAt: last, AppVersion: in.AppVersion, Queue: in.Queue,
		Battery: in.Battery, Charging: in.Charging, ListenerOK: in.ListenerOK, BankPackages: in.BankPackages,
	}
	s.mu.Unlock()
	s.watchPhone()
	return s.Status()
}

// --- Nhận thông báo ngân hàng ------------------------------------------------

type BankNotifyRequest struct {
	Package    string `json:"package"`
	Title      string `json:"title"`
	Text       string `json:"text"`
	PostedAt   int64  `json:"posted_at"`   // ms, đồng hồ điện thoại
	ReceivedAt int64  `json:"received_at"` // ms, lúc app VNET đọc được
}

type BankNotifyResult struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
	Code   string `json:"code,omitempty"`
}

func bankNotifHash(pkg, title, text string) string {
	sum := sha256.Sum256([]byte(pkg + "\n" + title + "\n" + text))
	return hex.EncodeToString(sum[:])
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// Ingest lưu một thông báo và, nếu được, khớp nó với một mã nạp đang chờ.
// Gửi lại cùng một thông báo (điện thoại thử lại sau lỗi mạng) trả "duplicate".
func (s *QRPayService) Ingest(in BankNotifyRequest) (*BankNotifyResult, error) {
	if strings.TrimSpace(in.Package) == "" || strings.TrimSpace(in.Text) == "" {
		return nil, errors.New("thiếu tên gói hoặc nội dung thông báo")
	}
	p := parseBankNotification(in.Package, in.Title, in.Text)
	now := s.now()
	n := model.BankNotification{
		ReceivedAt:    now,
		Package:       clip(in.Package, 100),
		Title:         in.Title,
		Text:          in.Text,
		Hash:          bankNotifHash(in.Package, in.Title, in.Text),
		Bank:          p.Bank,
		Amount:        p.Amount,
		Incoming:      p.Incoming,
		BankRef:       clip(p.BankRef, 60),
		SenderName:    clip(p.SenderName, 120),
		SenderAccount: clip(p.SenderAccount, 40),
		Content:       p.Content,
		Status:        model.BankNotifUnmatched,
	}
	if in.PostedAt > 0 {
		t := time.UnixMilli(in.PostedAt)
		n.PostedAt = &t
	}
	switch {
	case p.Bank == "":
		// Thông báo thử của chính app vnet-bank-notification (hoặc gói lạ): ghi lại để thấy
		// đường gửi chạy, nhưng không bao giờ thành tiền vào cộng được cho ai.
		n.Status, n.Note = model.BankNotifIgnored, "không phải app ngân hàng"
	case !p.Parsed:
		n.Status = model.BankNotifUnparsed
	case !p.Incoming:
		n.Status, n.Note = model.BankNotifIgnored, "tiền ra"
	}

	res := s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "hash"}}, DoNothing: true}).Create(&n)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return &BankNotifyResult{Status: "duplicate"}, nil
	}
	s.mu.Lock()
	s.phone.LastNotifAt = now
	s.mu.Unlock()

	if n.Status == model.BankNotifUnmatched {
		s.match(&n)
	}
	s.changed("bank-notifications")
	return &BankNotifyResult{ID: n.ID, Status: n.Status, Code: n.Code}, nil
}

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

func (s *QRPayService) List(f BankNotificationFilter, params pagination.Params) ([]BankNotificationRow, int64, error) {
	q := s.db.Table("bank_notifications AS n").
		Joins("LEFT JOIN members m ON m.id = n.member_id").
		Joins("LEFT JOIN orders o ON o.id = n.order_id").
		Joins("LEFT JOIN users u ON u.id = n.handled_by")
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
		return nil, 0, err
	}
	var rows []BankNotificationRow
	err := q.Select("n.*, COALESCE(NULLIF(m.username,''), m.full_name, '') AS member_name, COALESCE(o.order_code,'') AS order_code, COALESCE(u.full_name, u.username, '') AS handled_by_name").
		Order(pagination.OrderBy(&params, bankNotifSortColumns, "n.received_at DESC")).
		Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).
		Scan(&rows).Error
	return rows, total, err
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
