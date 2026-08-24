package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Thẻ nạp và thẻ quà tặng.
//
// Hai loại khác nhau, không gộp được:
//
//   - Thẻ nạp: dùng MỘT LẦN. Nạp xong thì mệnh giá vào số dư hội viên và thẻ
//     chết. Giống thẻ cào điện thoại.
//   - Thẻ quà tặng: có số dư riêng, tiêu DẦN qua nhiều đơn hàng cho tới hết.
//
// Ba nguyên tắc an toàn xuyên suốt file này:
//
//  1. Phần bí mật lưu băm, không lưu thô. Mã thô chỉ hiện đúng một lần trong
//     phản hồi lúc sinh thẻ. Lưu thô nghĩa là ai đọc được database là tiêu được
//     toàn bộ thẻ chưa dùng — và đó là tiền thật.
//  2. Mỗi thẻ có SERI công khai để tra cứu, tách khỏi phần bí mật. Nhân viên hỗ
//     trợ khách bằng seri, không cần biết mã.
//  3. Nạp/tiêu luôn khoá dòng thẻ trong giao dịch. Hai người nạp cùng một thẻ
//     cùng lúc phải có đúng một người thắng.

type CardService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewCardService(db *gorm.DB, audit *AuditService) *CardService {
	return &CardService{db: db, audit: audit}
}

// Bảng chữ cái sinh mã: bỏ 0/O/1/I/L để nhân viên đọc số điện thoại cho khách
// không bị nhầm.
const cardAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

const (
	cardSecretLen = 16 // 16 ký tự × log2(31) ≈ 79 bit — dò mù là không khả thi
	cardSerialLen = 10
)

const (
	CardStatusActive    = "active"
	CardStatusUsed      = "used"
	CardStatusCancelled = "cancelled"
)

// GeneratedCard là thứ DUY NHẤT chứa mã thô, và chỉ tồn tại trong phản hồi của
// lệnh sinh thẻ. In ra hoặc xuất file ngay — không lấy lại được.
type GeneratedCard struct {
	ID     string `json:"id"`
	Serial string `json:"serial"`
	Secret string `json:"secret"`
	Value  int64  `json:"value"`
}

func randomString(n int) (string, error) {
	max := big.NewInt(int64(len(cardAlphabet)))
	var b strings.Builder
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(cardAlphabet[v.Int64()])
	}
	return b.String(), nil
}

func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(s))))
	return hex.EncodeToString(sum[:])
}

// secretMatches so sánh theo thời gian hằng định. Ở đây tra theo seri rồi mới
// so mã, nên rò rỉ thời gian là nhỏ — nhưng so sánh hằng định không tốn gì.
func secretMatches(stored, given string) bool {
	return subtle.ConstantTimeCompare([]byte(stored), []byte(hashSecret(given))) == 1
}

func normalizeSerial(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// --- thẻ nạp ---------------------------------------------------------------

type GenerateTopupCardsRequest struct {
	Count      int    `json:"count" binding:"required,min=1,max=500"`
	FaceValue  int64  `json:"face_value" binding:"required,gt=0"`
	BonusValue int64  `json:"bonus_value" binding:"omitempty,min=0"`
	ExpiresAt  string `json:"expires_at"`
}

func (s *CardService) GenerateTopupCards(req *GenerateTopupCardsRequest, actorID string) ([]GeneratedCard, error) {
	expires, err := parseOptionalTime(req.ExpiresAt)
	if err != nil {
		return nil, err
	}

	out := make([]GeneratedCard, 0, req.Count)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < req.Count; i++ {
			serial, err := randomString(cardSerialLen)
			if err != nil {
				return err
			}
			secret, err := randomString(cardSecretLen)
			if err != nil {
				return err
			}
			card := model.TopupCard{
				Code:       "TC" + serial,
				Pin:        hashSecret(secret),
				FaceValue:  req.FaceValue,
				BonusValue: req.BonusValue,
				Status:     CardStatusActive,
				ExpiresAt:  expires,
			}
			if err := tx.Create(&card).Error; err != nil {
				return err
			}
			out = append(out, GeneratedCard{
				ID: card.ID, Serial: card.Code, Secret: secret, Value: req.FaceValue,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.log("generate_topup_cards", "topup_card", "", actorID, map[string]interface{}{
		"count": req.Count, "face_value": req.FaceValue, "bonus_value": req.BonusValue,
	})
	return out, nil
}

type RedeemTopupCardRequest struct {
	Serial   string `json:"serial" binding:"required"`
	Secret   string `json:"secret" binding:"required"`
	MemberID string `json:"member_id"`
}

type RedeemResult struct {
	Serial       string `json:"serial"`
	FaceValue    int64  `json:"face_value"`
	BonusValue   int64  `json:"bonus_value"`
	BalanceAfter int64  `json:"balance_after"`
	BonusAfter   int64  `json:"bonus_after"`
}

// errCardInvalid dùng chung cho MỌI lý do thẻ không dùng được: sai seri, sai
// mã, đã dùng, đã huỷ, hết hạn.
//
// Cố ý không nói rõ lý do: phân biệt "sai seri" với "sai mã" cho phép dò từng
// seri hợp lệ rồi mới tấn công phần bí mật.
var errCardInvalid = errors.New("thẻ không hợp lệ hoặc đã được sử dụng")

// RedeemTopupCard nạp thẻ vào số dư hội viên.
func (s *CardService) RedeemTopupCard(req *RedeemTopupCardRequest, actorID string) (*RedeemResult, error) {
	if req.MemberID == "" {
		return nil, errors.New("thiếu mã hội viên")
	}

	// Lý do thất bại được GOM lại ở đây rồi mới ghi nhật ký SAU giao dịch.
	// Ghi bên trong thì rollback xoá luôn bản ghi — và đúng dấu vết cần nhất
	// (ai đang dò mã) là thứ biến mất.
	var failSerial, failReason string

	var res RedeemResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var card model.TopupCard
		// Khoá dòng thẻ NGAY: hai người nạp cùng một thẻ cùng lúc thì người
		// thứ hai phải chờ, thấy status đã là "used" và bị từ chối.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ?", normalizeSerial(req.Serial)).First(&card).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				failSerial, failReason = normalizeSerial(req.Serial), "không tìm thấy seri"
				return errCardInvalid
			}
			return err
		}

		if !secretMatches(card.Pin, req.Secret) {
			failSerial, failReason = card.Code, "sai mã bí mật"
			return errCardInvalid
		}
		if card.Status != CardStatusActive {
			failSerial, failReason = card.Code, "trạng thái "+card.Status
			return errCardInvalid
		}
		if card.ExpiresAt != nil && card.ExpiresAt.Before(time.Now()) {
			failSerial, failReason = card.Code, "hết hạn"
			return errCardInvalid
		}

		var member model.Member
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", req.MemberID).First(&member).Error; err != nil {
			return errors.New("không tìm thấy hội viên")
		}

		balanceAfter := member.Balance + card.FaceValue
		bonusAfter := member.BonusBalance + card.BonusValue

		trans := model.MemberTransaction{
			MemberID:        member.ID,
			TransactionType: "topup_card",
			Amount:          card.FaceValue,
			BalanceBefore:   member.Balance,
			BalanceAfter:    balanceAfter,
			BonusBefore:     member.BonusBalance,
			BonusAfter:      bonusAfter,
			PaymentMethod:   "topup_card",
			ReferenceID:     &card.ID,
			Description:     "Nạp thẻ " + card.Code,
		}
		if actorID != "" {
			trans.CreatedBy = &actorID
		}
		if err := tx.Create(&trans).Error; err != nil {
			return err
		}
		if err := tx.Model(&member).Updates(map[string]interface{}{
			"balance": balanceAfter, "bonus_balance": bonusAfter,
		}).Error; err != nil {
			return err
		}

		now := time.Now()
		usedBy := member.ID
		if err := tx.Model(&card).Updates(map[string]interface{}{
			"status": CardStatusUsed, "used_by": &usedBy, "used_at": &now,
		}).Error; err != nil {
			return err
		}

		res = RedeemResult{
			Serial: card.Code, FaceValue: card.FaceValue, BonusValue: card.BonusValue,
			BalanceAfter: balanceAfter, BonusAfter: bonusAfter,
		}
		return nil
	})
	if err != nil {
		if failReason != "" {
			s.logFailedRedeem("topup", failSerial, actorID, failReason)
		}
		return nil, err
	}

	s.log("redeem_topup_card", "topup_card", "", actorID, map[string]interface{}{
		"serial": res.Serial, "member_id": req.MemberID,
		"face_value": res.FaceValue, "bonus_value": res.BonusValue,
	})
	return &res, nil
}

// --- thẻ quà tặng ----------------------------------------------------------

type GenerateGiftCardsRequest struct {
	Count     int    `json:"count" binding:"required,min=1,max=500"`
	Value     int64  `json:"value" binding:"required,gt=0"`
	ExpiresAt string `json:"expires_at"`
}

func (s *CardService) GenerateGiftCards(req *GenerateGiftCardsRequest, actorID string) ([]GeneratedCard, error) {
	expires, err := parseOptionalTime(req.ExpiresAt)
	if err != nil {
		return nil, err
	}

	out := make([]GeneratedCard, 0, req.Count)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < req.Count; i++ {
			serial, err := randomString(cardSerialLen)
			if err != nil {
				return err
			}
			secret, err := randomString(cardSecretLen)
			if err != nil {
				return err
			}
			card := model.GiftCard{
				Serial:         "GC" + serial,
				Code:           hashSecret(secret),
				Balance:        req.Value,
				InitialBalance: req.Value,
				Status:         CardStatusActive,
				ExpiresAt:      expires,
			}
			if err := tx.Create(&card).Error; err != nil {
				return err
			}
			out = append(out, GeneratedCard{
				ID: card.ID, Serial: card.Serial, Secret: secret, Value: req.Value,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.log("generate_gift_cards", "gift_card", "", actorID, map[string]interface{}{
		"count": req.Count, "value": req.Value,
	})
	return out, nil
}

type GiftCardBalanceResult struct {
	Serial    string     `json:"serial"`
	Balance   int64      `json:"balance"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// CheckGiftCard tra số dư. Vẫn đòi mã bí mật: nếu chỉ cần seri là xem được số
// dư thì ai cầm tấm thẻ chụp ảnh cũng dò được.
func (s *CardService) CheckGiftCard(serial, secret string) (*GiftCardBalanceResult, error) {
	var card model.GiftCard
	if err := s.db.Where("serial = ?", normalizeSerial(serial)).First(&card).Error; err != nil {
		return nil, errCardInvalid
	}
	if !secretMatches(card.Code, secret) {
		return nil, errCardInvalid
	}
	return &GiftCardBalanceResult{
		Serial: card.Serial, Balance: card.Balance,
		Status: card.Status, ExpiresAt: card.ExpiresAt,
	}, nil
}

// spendGiftCard trừ tiền thẻ quà tặng trong CHÍNH giao dịch thanh toán đơn.
//
// Là hàm cấp gói để OrderService gọi thẳng bằng giao dịch đang mở — trừ thẻ và
// chốt đơn phải cùng sống hoặc cùng chết, không được nửa nọ nửa kia.
func spendGiftCard(tx *gorm.DB, serial, secret string, amount int64, orderID string) error {
	if amount <= 0 {
		return errors.New("số tiền phải lớn hơn 0")
	}

	var card model.GiftCard
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("serial = ?", normalizeSerial(serial)).First(&card).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errCardInvalid
		}
		return err
	}
	if !secretMatches(card.Code, secret) {
		return errCardInvalid
	}
	if card.Status != CardStatusActive {
		return errCardInvalid
	}
	if card.ExpiresAt != nil && card.ExpiresAt.Before(time.Now()) {
		return errCardInvalid
	}
	if card.Balance < amount {
		// Đây là lỗi nói rõ được: khách cầm thẻ trong tay, biết còn bao nhiêu
		// không giúp gì cho kẻ tấn công.
		return fmt.Errorf("thẻ quà tặng không đủ số dư (còn %d, cần %d)", card.Balance, amount)
	}

	after := card.Balance - amount
	trans := model.GiftCardTransaction{
		GiftCardID:    card.ID,
		Amount:        -amount,
		BalanceBefore: card.Balance,
		BalanceAfter:  after,
	}
	if orderID != "" {
		trans.OrderID = &orderID
	}
	if err := tx.Create(&trans).Error; err != nil {
		return err
	}

	updates := map[string]interface{}{"balance": after}
	if after == 0 {
		updates["status"] = CardStatusUsed
	}
	return tx.Model(&card).Updates(updates).Error
}

// --- chung -----------------------------------------------------------------

// CancelCard huỷ thẻ chưa dùng. Thẻ đã dùng thì không huỷ được — tiền đã sang
// tay, huỷ chỉ làm sổ sách sai.
func (s *CardService) CancelCard(kind, id, actorID string) error {
	var (
		table  string
		status string
	)
	switch kind {
	case "topup":
		var card model.TopupCard
		if err := s.db.First(&card, "id = ?", id).Error; err != nil {
			return errors.New("không tìm thấy thẻ")
		}
		table, status = "topup_cards", card.Status
	case "gift":
		var card model.GiftCard
		if err := s.db.First(&card, "id = ?", id).Error; err != nil {
			return errors.New("không tìm thấy thẻ")
		}
		table, status = "gift_cards", card.Status
	default:
		return errors.New("loại thẻ không hợp lệ")
	}

	if status == CardStatusUsed {
		return errors.New("thẻ đã được sử dụng, không huỷ được")
	}
	if status == CardStatusCancelled {
		return errors.New("thẻ đã bị huỷ")
	}

	if err := s.db.Table(table).Where("id = ?", id).
		Update("status", CardStatusCancelled).Error; err != nil {
		return err
	}
	s.log("cancel_card", kind+"_card", id, actorID, map[string]interface{}{"kind": kind})
	return nil
}

// --- bán thẻ ở quầy -----------------------------------------------------------
//
// Hai cột sold_to/sold_at khai từ đầu nhưng KHÔNG endpoint nào ghi. Chúng khác
// used_by/used_at: bán là lúc quán đưa tấm thẻ giấy cho khách, nạp là lúc ai đó
// gõ mã vào tài khoản — người mua và người nạp có thể là hai người khác nhau
// (mua tặng). Không ghi lại người mua thì lúc khách báo mất thẻ, quán không có
// gì đối chiếu.

type SellTopupCardRequest struct {
	MemberID string `json:"member_id" binding:"required"`
}

func (s *CardService) SellTopupCard(id, memberID, actorID string) (*model.TopupCard, error) {
	var card model.TopupCard
	if err := s.db.First(&card, "id = ?", id).Error; err != nil {
		return nil, errors.New("không tìm thấy thẻ")
	}
	if card.Status != CardStatusActive {
		return nil, errors.New("chỉ bán được thẻ còn dùng được")
	}
	if card.SoldTo != nil {
		return nil, errors.New("thẻ đã bán cho người khác")
	}

	var member model.Member
	if err := s.db.Select("id, full_name").First(&member, "id = ?", memberID).Error; err != nil {
		return nil, errors.New("không tìm thấy hội viên")
	}

	now := time.Now()
	if err := s.db.Model(&card).Updates(map[string]interface{}{
		"sold_to": memberID,
		"sold_at": now,
	}).Error; err != nil {
		return nil, err
	}
	card.SoldTo = &memberID
	card.SoldAt = &now

	s.log("sell_card", "topup_card", id, actorID, map[string]interface{}{
		"serial":      card.Code,
		"member_id":   memberID,
		"member_name": member.FullName,
		"face_value":  card.FaceValue,
	})
	return &card, nil
}

type CardListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Search   string `form:"search"`
	Status   string `form:"status"`
}

func (s *CardService) ListTopupCards(req *CardListRequest) (*pagination.Result, error) {
	q := s.db.Model(&model.TopupCard{})
	if req.Status != "" {
		q = q.Where("status = ?", req.Status)
	}
	if req.Search != "" {
		q = q.Where("code ILIKE ?", "%"+normalizeSerial(req.Search)+"%")
	}
	var rows []model.TopupCard
	return paginateCards(q.Order("created_at desc"), req, &rows)
}

func (s *CardService) ListGiftCards(req *CardListRequest) (*pagination.Result, error) {
	q := s.db.Model(&model.GiftCard{})
	if req.Status != "" {
		q = q.Where("status = ?", req.Status)
	}
	if req.Search != "" {
		q = q.Where("serial ILIKE ?", "%"+normalizeSerial(req.Search)+"%")
	}
	var rows []model.GiftCard
	return paginateCards(q.Order("created_at desc"), req, &rows)
}

func paginateCards(q *gorm.DB, req *CardListRequest, rows interface{}) (*pagination.Result, error) {
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := q.Offset((page - 1) * size).Limit(size).Find(rows).Error; err != nil {
		return nil, err
	}
	return &pagination.Result{Items: rows, Total: total, Page: page, PageSize: size}, nil
}

func parseOptionalTime(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, errors.New("hạn sử dụng sai định dạng")
	}
	return &t, nil
}

func (s *CardService) log(action, entity, id, actorID string, meta map[string]interface{}) {
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action: action, EntityType: entity, EntityID: id,
		UserID: actor, Metadata: meta,
	})
}

// logFailedRedeem ghi lại mọi lần nạp hụt. Một loạt bản ghi này trên cùng một
// tài khoản là dấu hiệu có người đang dò mã.
func (s *CardService) logFailedRedeem(kind, serial, actorID, reason string) {
	s.log("redeem_failed", kind+"_card", "", actorID, map[string]interface{}{
		"serial": serial, "reason": reason,
	})
}
