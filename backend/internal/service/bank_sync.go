package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm/clause"
)

// Giao dịch tiền vào đến từ ThueAPI: ngân hàng → ThueAPI → webhook của Hub → quán.
// Máy chủ quán chỉ chạy trong mạng nội bộ nên không nhận webhook được. Hub giữ giao
// dịch, báo quán qua kênh sự kiện (KickBankSync), quán đọc theo số thứ tự tăng dần.
// Quán tắt hay rớt mạng thì lần đọc sau bắt bù, không mất tiền nào.

const (
	bankSyncGroup     = "bank_sync"
	bankSyncCursorKey = "cursor"
	bankProviderName  = "thueapi"
	// Đọc định kỳ phòng khi lỡ tin báo của Hub.
	bankSyncEvery = 15 * time.Second
	// Lỡ 3 lần đọc liên tiếp là mất kết nối: mã QR mới sẽ không xác nhận được tiền.
	qrSourceOfflineAfter = 50 * time.Second
)

// Không đi theo chuyển hướng: key quán chỉ gửi đúng tới địa chỉ Hub đã cài.
var bankSyncClient = &http.Client{
	Timeout:       10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

var (
	// Mã nạp: VN + 6 ký tự, bỏ 0/O, 1/I/L cho khỏi đọc nhầm (xem taoMaNap).
	reQRCode     = regexp.MustCompile(`VN[2-9A-HJKMNP-Z]{6}`)
	reSeparators = regexp.MustCompile(`[\s.\-_/]`)
	bankTimezone = time.FixedZone("ICT", 7*60*60)
)

// Mã ngân hàng của ThueAPI → mã trong qrBanks.
var bankFromGateway = map[string]string{"MBB": "mb", "VCB": "vcb"}

type hubBankEvent struct {
	Seq             int64  `json:"seq"`
	TxID            string `json:"tx_id"`
	Bank            string `json:"bank"`
	BankRef         string `json:"bank_ref"`
	Amount          int64  `json:"amount"`
	Content         string `json:"content"`
	Direction       string `json:"direction"`
	TransactionDate string `json:"transaction_date"`
}

// qrCodesIn trả về mọi chuỗi có dạng mã nạp trong nội dung, cả khi ngân hàng
// người gửi chèn dấu cách hay dấu chấm vào giữa. Có thể lẫn chuỗi không phải
// mã (vd. đuôi tên người): nơi gọi chỉ nhận chuỗi trùng một yêu cầu đang chờ.
func qrCodesIn(content string) []string {
	upper := strings.ToUpper(content)
	seen := map[string]bool{}
	var out []string
	for _, s := range []string{upper, reSeparators.ReplaceAllString(upper, "")} {
		for _, c := range reQRCode.FindAllString(s, -1) {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func bankEventHash(txID string) string {
	sum := sha256.Sum256([]byte(bankProviderName + "\n" + txID))
	return hex.EncodeToString(sum[:])
}

func (s *QRPayService) notificationFrom(e hubBankEvent) model.BankNotification {
	n := model.BankNotification{
		ReceivedAt: s.now(),
		Package:    bankProviderName,
		Text:       e.Content,
		Hash:       bankEventHash(e.TxID),
		Bank:       bankFromGateway[e.Bank],
		Amount:     e.Amount,
		Incoming:   true,
		BankRef:    clip(e.BankRef, 60),
		Content:    e.Content,
		Status:     model.BankNotifUnmatched,
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", e.TransactionDate, bankTimezone); err == nil {
		n.PostedAt = &t
	}
	return n
}

// ingestBankEvent lưu một giao dịch tiền vào và, nếu được, khớp nó với một mã nạp
// đang chờ. Đọc lại cùng giao dịch (Hash) thì bỏ qua, nên không bao giờ cộng hai lần.
func (s *QRPayService) ingestBankEvent(e hubBankEvent) error {
	if e.Direction != "IN" {
		return nil
	}
	n := s.notificationFrom(e)
	// Khoản đã ghi từ hồi còn đọc thông báo điện thoại có cùng nội dung và số tiền: không ghi lần nữa.
	var seen int64
	if err := s.db.Model(&model.BankNotification{}).
		Where("package <> ? AND amount = ? AND content = ?", bankProviderName, n.Amount, n.Content).Count(&seen).Error; err != nil {
		return err
	}
	if seen > 0 {
		return nil
	}
	res := s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "hash"}}, DoNothing: true}).Create(&n)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}
	s.match(&n)
	s.changed("bank-notifications")
	return nil
}

// fetchBankEvents đọc từ Hub các giao dịch sau số thứ tự after.
func (s *QRPayService) fetchBankEvents(ctx context.Context, after int64) ([]hubBankEvent, error) {
	if s.hubAccess == nil {
		return nil, errors.New("quán chưa nối Hub")
	}
	base, key := s.hubAccess()
	if base == "" || key == "" {
		return nil, errors.New("quán chưa nối Hub")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/bank-events?after="+strconv.FormatInt(after, 10), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(GameCatalogKeyHeader, key)
	res, err := bankSyncClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil, fmt.Errorf("Hub trả %d", res.StatusCode)
	}
	var body struct {
		Events []hubBankEvent `json:"events"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Events, nil
}

func (s *QRPayService) bankCursor() int64 {
	return settingInt(settingsGroup(s.db, bankSyncGroup), bankSyncCursorKey, 0)
}

func (s *QRPayService) saveBankCursor(seq int64) error {
	value := strconv.FormatInt(seq, 10)
	row := model.SystemSetting{GroupName: bankSyncGroup, Key: bankSyncCursorKey, Value: value}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group_name"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"value": value, "updated_at": s.now()}),
	}).Create(&row).Error
}

// syncBankEvents đọc hết giao dịch mới. Lỗi ở một giao dịch thì dừng lại, không
// nhích số thứ tự, lần sau đọc lại đúng giao dịch đó.
func (s *QRPayService) syncBankEvents(ctx context.Context) error {
	cursor := s.bankCursor()
	for {
		events, err := s.fetchBankEvents(ctx, cursor)
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.lastSync = s.now()
		s.mu.Unlock()
		if len(events) == 0 {
			return nil
		}
		for _, e := range events {
			if err := s.ingestBankEvent(e); err != nil {
				return fmt.Errorf("giao dịch %d: %w", e.Seq, err)
			}
			cursor = e.Seq
			if err := s.saveBankCursor(cursor); err != nil {
				return err
			}
		}
	}
}

// KickBankSync: Hub vừa báo có giao dịch mới.
func (s *QRPayService) KickBankSync() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// watchSource báo trang quản lý và máy trạm khi vừa mất hay có lại nguồn giao dịch.
func (s *QRPayService) watchSource() {
	s.mu.Lock()
	online := s.sourceOnline(s.now())
	flipped := online != s.wasOnline
	s.wasOnline = online
	s.mu.Unlock()
	if flipped {
		s.changed("qr")
	}
}

// RunBankSync chạy cho tới khi ctx kết thúc.
func (s *QRPayService) RunBankSync(ctx context.Context) {
	// Khoá của điện thoại đã bỏ. GET /settings/:group mở cho mọi tài khoản nên không để nó nằm lại.
	s.db.Where("group_name = ? AND key = ?", QRPaymentGroup, "device_key").Delete(&model.SystemSetting{})
	t := time.NewTicker(bankSyncEvery)
	defer t.Stop()
	lastErr := ""
	for {
		msg := ""
		if err := s.syncBankEvents(ctx); err != nil && ctx.Err() == nil {
			msg = err.Error()
		}
		if msg != "" && msg != lastErr {
			log.Printf("[qr] đọc giao dịch ngân hàng: %s", msg)
		}
		lastErr = msg
		s.watchSource()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-s.kick:
		}
	}
}
