package service

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionService struct {
	db     *gorm.DB
	hub    *hub.Hub
	audit  *AuditService
	curfew *CurfewService
}

func NewSessionService(db *gorm.DB, wsHub *hub.Hub, audit *AuditService) *SessionService {
	return &SessionService{db: db, hub: wsHub, audit: audit}
}

// WithCurfew attaches the curfew rules used to gate session starts. It is a
// setter rather than a constructor argument because both services need the
// audit service, and existing callers construct sessions without curfew.
func (s *SessionService) WithCurfew(c *CurfewService) *SessionService {
	s.curfew = c
	return s
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
	BonusUsed       int64          `json:"bonus_used"`
	AmountUnpaid    int64          `json:"amount_unpaid"`
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

	// ChargedAmount là tiền ĐÃ trừ tới lúc này; AffordableUntil là thời điểm số
	// dư cạn theo đơn giá đang áp dụng. Cả hai nằm sẵn trên dòng phiên nên mọi
	// màn hình có được miễn phí, không phát sinh truy vấn nào.
	ChargedAmount   int64      `json:"charged_amount"`
	AffordableUntil *time.Time `json:"affordable_until"`

	// Snapshot fields (populated when session ends).
	// PricePerHour là ngoại lệ: có từ lúc mở máy nên KHÔNG omitempty — phiên
	// đang chạy cần đơn giá để màn hình tính lại khi khách vừa nạp tiền.
	MachineGroupID   *string `json:"machine_group_id,omitempty"`
	MemberGroupID    *string `json:"member_group_id,omitempty"`
	MachineGroupName string  `json:"machine_group_name,omitempty"`
	PricePerHour     int64   `json:"price_per_hour"`
	BilledMinutes    int     `json:"billed_minutes,omitempty"`
}

type CostBreakdown struct {
	MachineGroupName string `json:"machine_group_name"`
	// NoPricing bật khi máy không tra ra được giá nào (chưa gán nhóm, hoặc nhóm
	// để giá 0). Phiên vẫn mở được — có quán cố ý để máy miễn phí — nhưng quầy
	// phải nhìn thấy điều đó trước khi bấm, không phải phát hiện lúc cuối tháng
	// thấy doanh thu hụt.
	NoPricing       bool    `json:"no_pricing"`
	PricePerHour    int64   `json:"price_per_hour"`
	DurationMinutes int     `json:"duration_minutes"`
	BilledMinutes   int     `json:"billed_minutes"`
	GrossCost       int64   `json:"gross_cost"`
	DiscountPercent float64 `json:"discount_percent"`
	DiscountAmount  int64   `json:"discount_amount"`
	FinalCost       int64   `json:"final_cost"`
}

// CheckMemberMayPlay gom mọi điều kiện để một hội viên được ngồi máy.
//
// Hàm này phải dùng chung cho MỌI cửa vào máy, không riêng /sessions/start.
// Trước đây luật chỉ nằm trong StartSession — đường mà NHÂN VIÊN mở máy từ trang
// quản trị — còn /auth/member-login, tức là màn hình khoá mà KHÁCH tự gõ mật
// khẩu vào, không kiểm gì cả. Khách hết tiền vẫn đăng nhập được và dùng máy
// bình thường, chỉ là không có phiên nào tính tiền.
//
// hasCombo = true khi phiên đi kèm gói trả trước: gói tự mang thời gian nên
// không cần số dư.
func CheckMemberMayPlay(db *gorm.DB, curfew *CurfewService, member *model.Member, hasCombo bool) error {
	// Phiên chỉ chốt tiền lúc trả máy nên có thể để lại khoản nợ. Đây là chỗ
	// duy nhất chặn nợ chồng nợ, thứ làm mô hình "trả sau" an toàn.
	//
	// max_debt (nhóm cài đặt "limits") là trần nợ quán chấp nhận. Trước đây ô
	// này lưu được nhưng không nơi nào đọc, và luật cứng là "nợ 1 đồng cũng
	// không cho chơi". Để 0 giữ nguyên hành vi cũ.
	if member.Balance < 0 {
		maxDebt := settingInt(settingsGroup(db, "limits"), "max_debt", 0)
		if -member.Balance > maxDebt {
			return fmt.Errorf("hội viên đang nợ %d₫, vượt trần nợ %d₫ — cần nạp thêm trước khi mở máy",
				-member.Balance, maxDebt)
		}
	}

	// A prepaid combo carries its own time, so it does not need a balance.
	if !hasCombo && member.Balance == 0 && member.BonusBalance == 0 {
		return errors.New("hội viên không còn số dư; hãy nạp tiền hoặc dùng gói cước")
	}

	if curfew != nil {
		if err := curfew.CheckStart(member, utils.VietnamTime()); err != nil {
			return err
		}
	}
	return nil
}

// affordableUntil trả về thời điểm số dư cạn theo đơn giá đang áp dụng, hoặc
// nil nếu không có giới hạn (máy chưa có giá, hoặc gói cước đang phủ).
//
// Tính ở máy chủ để mọi màn hình chỉ việc đếm ngược tới một mốc: máy trạm không
// phải tự tra giá, trang Phiên không phải nạp số dư của từng hội viên.
func affordableUntil(session *model.MachineSession, member *model.Member, at time.Time) *time.Time {
	if member == nil || session.PricePerHour <= 0 {
		return nil
	}
	// Gói khung giờ chạy tới hết khung, số dư không liên quan.
	if session.ComboID != nil && session.ComboType == "fixed_slot" {
		return nil
	}

	// Tiền còn dùng được = số dư + điểm thưởng, trừ đi phần đã trừ cho phiên này
	// nhưng chưa phản ánh vào số dư thì không có — ChargedAmount đã trừ rồi.
	remaining := member.Balance + member.BonusBalance
	if remaining <= 0 {
		return &at
	}

	minutes := remaining * 60 / session.PricePerHour
	// Phút gói cước còn phủ được cộng thêm: khách chưa phải tiêu đồng nào cho
	// quãng đó.
	if session.RemainingMinutes != nil {
		elapsed := int64(at.Sub(session.StartedAt).Minutes())
		covered := int64(*session.RemainingMinutes) - elapsed
		if covered > 0 {
			minutes += covered
		}
	}
	until := at.Add(time.Duration(minutes) * time.Minute)
	return &until
}

// billableMinutes trả về số phút PHẢI TRẢ TIỀN của một phiên sau khi trừ phần
// gói cước đã phủ.
//
// Dùng chung cho cả lượt tính mỗi phút lẫn lúc trả máy — hai công thức riêng là
// hai chỗ để lệch nhau, và lệch ở đây là lệch tiền của khách.
//
// session.RemainingMinutes là ảnh chụp số phút gói còn lại lúc MỞ máy, nên phép
// trừ này đúng suốt phiên mà không cần đọc lại ComboPurchase.
func billableMinutes(session *model.MachineSession, elapsedMinutes int) int {
	// Gói khung giờ đã trả tiền trọn khung: khách mua rồi thì không trả thêm
	// đồng nào theo giá lẻ. Gói loại này có TotalMinutes = 0 nên
	// RemainingMinutes = 0, và phép trừ bên dưới sẽ tính tiền TOÀN BỘ thời gian
	// — khách bị thu hai lần cho cùng một khung giờ.
	if session.ComboID != nil && session.ComboType == "fixed_slot" {
		return 0
	}
	if session.RemainingMinutes != nil {
		elapsedMinutes -= *session.RemainingMinutes
	}
	if elapsedMinutes < 0 {
		return 0
	}
	return elapsedMinutes
}

// chargeResult mô tả một lượt trừ tiền.
type chargeResult struct {
	Charged       int64 // số tiền vừa trừ trong lượt này
	BalanceBefore int64
	BalanceAfter  int64
	BonusUsed     int64
	AmountUnpaid  int64 // phần nợ (số dư âm) sau lượt này
}

// chargeSessionTo trừ tiền cho tới khi tổng đã thu của phiên bằng target.
//
// Đây là phép TÍCH LUỸ TỚI ĐÍCH, không phải cộng dồn từng lượt: mỗi lần gọi hỏi
// "tới giờ đáng lẽ đã thu bao nhiêu" rồi chỉ thu phần chênh. Ba hệ quả, tất cả
// đều là lý do chọn cách này:
//
//   - Idempotent. Gọi lại, khởi động lại máy chủ, hai tiến trình chạy chồng —
//     tổng thu không bao giờ vượt target.
//   - Không lệch do làm tròn. math.Ceil trong CalculateCost áp lên TỔNG, nên
//     tiền cuối phiên khớp từng đồng với cách tính một lần lúc trả máy. Cộng dồn
//     từng phút thì Ceil chạy 60 lần mỗi giờ và lệch tới +60₫/giờ.
//   - min_duration tự đúng. CalculateCost đã nâng số phút tính tiền lên mức tối
//     thiểu, nên ngay lượt đầu target đã bằng trọn mức đó.
//
// target chỉ được phép tăng: giá tụt giữa phiên (ra khỏi khung cao điểm) thì
// không hoàn tiền, và cũng không thu lại lần nữa.
//
// Sổ giao dịch giữ ĐÚNG MỘT dòng session_fee cho cả phiên, lớn dần theo thời
// gian. Mỗi phút một dòng thì 100 máy sinh ~144.000 dòng/ngày và trang Giao dịch
// của mỗi khách ngập.
//
// Người gọi phải đang ở trong transaction và đã khoá dòng phiên.
func chargeSessionTo(tx *gorm.DB, session *model.MachineSession, target int64, machineCode string, addPlayedMinutes int) (*chargeResult, error) {
	res := &chargeResult{}
	if session.MemberID == nil || *session.MemberID == "" {
		return res, nil
	}
	memberID := *session.MemberID

	delta := target - session.ChargedAmount
	if delta < 0 {
		delta = 0
	}

	var member model.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", memberID).First(&member).Error; err != nil {
		return nil, errors.New("không tìm thấy hội viên")
	}

	res.BalanceBefore = member.Balance
	bonusBefore := member.BonusBalance

	// Trừ tiền không bao giờ được từ chối. Trả máy phải luôn thành công để máy
	// được giải phóng; phần thiếu thành nợ, và CheckMemberMayPlay chặn ở cửa vào
	// nên nợ không phình mãi.
	//
	// Điểm thưởng tiêu trước, tiền thật tiêu sau.
	owed := delta
	if member.BonusBalance > 0 && owed > 0 {
		res.BonusUsed = member.BonusBalance
		if res.BonusUsed > owed {
			res.BonusUsed = owed
		}
		member.BonusBalance -= res.BonusUsed
		owed -= res.BonusUsed
	}
	member.Balance -= owed
	if member.Balance < 0 {
		res.AmountUnpaid = -member.Balance
	}
	res.BalanceAfter = member.Balance
	res.Charged = delta

	// Tổng số phút đã chơi: cột này khai từ đầu nhưng không nơi nào cộng, nên
	// trang hội viên luôn hiện 0 dù khách đã ngồi hàng trăm giờ.
	if addPlayedMinutes > 0 {
		member.TotalPlayedMinutes += addPlayedMinutes
	}

	// total_spent quyết định hạng hội viên nhưng trước đây CHỈ tăng khi mua gói
	// cước, nên khách tiêu hàng triệu tiền giờ vẫn nằm hạng thấp nhất và cả nút
	// "Xếp lại hạng" lẫn tác vụ nền đều vô nghĩa. Cộng đúng số tiền THẬT SỰ đã
	// trừ (delta), kể cả phần trả bằng điểm thưởng — khách vẫn tiêu ngần ấy.
	member.TotalSpent += delta

	if err := tx.Save(&member).Error; err != nil {
		return nil, err
	}

	session.ChargedAmount = target

	// Một dòng cho cả phiên: tìm theo reference_id rồi cộng vào, chưa có thì tạo.
	var txn model.MemberTransaction
	err := tx.Where("reference_id = ? AND transaction_type = ?", session.ID, "session_fee").
		First(&txn).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if delta == 0 {
			// Chưa trừ đồng nào thì đừng tạo dòng 0₫ làm rác sổ.
			return res, nil
		}
		txn = model.MemberTransaction{
			MemberID:        memberID,
			TransactionType: "session_fee",
			// Ghi số tiền THẬT SỰ đã trừ, không phải target.
			//
			// Với phiên bình thường hai con số bằng nhau vì tổng các delta chính
			// là target. Chúng lệch nhau đúng một trường hợp: phiên đã chạy dở
			// lúc triển khai, được đánh dấu "đã thu tới thời điểm triển khai" nên
			// charged_amount khác 0 mà chưa có đồng nào rời khỏi số dư. Lấy
			// target thì dòng sổ đầu tiên ghi -15.000₫ trong khi số dư chỉ giảm
			// 833₫ — bất biến của sổ cái vỡ ngay ở dòng đầu.
			Amount:        -delta,
			BalanceBefore: res.BalanceBefore,
			BalanceAfter:  res.BalanceAfter,
			// BonusBefore/BonusAfter trước đây bị bỏ trống, nên khi khách tiêu
			// điểm thưởng thì BalanceAfter - BalanceBefore != Amount và bất biến
			// của sổ cái vỡ đúng ở loại giao dịch này.
			BonusBefore: bonusBefore,
			BonusAfter:  member.BonusBalance,
			ReferenceID: &session.ID,
			Description: "Session fee for " + machineCode,
		}
		if err := tx.Create(&txn).Error; err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		if delta == 0 {
			return res, nil
		}
		if err := tx.Model(&txn).Updates(map[string]interface{}{
			"amount":        txn.Amount - delta,
			"balance_after": res.BalanceAfter,
			"bonus_after":   member.BonusBalance,
		}).Error; err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (s *SessionService) StartSession(req *StartRequest) (*SessionDetail, error) {
	var machine model.Machine
	if err := s.db.Where("id = ? AND is_active = ?", req.MachineID, true).First(&machine).Error; err != nil {
		return nil, errors.New("không tìm thấy máy")
	}

	if machine.Status == "in_use" {
		return nil, errors.New("máy đang có người dùng")
	}

	var member model.Member
	if err := s.db.Where("id = ? AND is_active = ?", req.MemberID, true).First(&member).Error; err != nil {
		return nil, errors.New("không tìm thấy hội viên hoặc tài khoản đã bị khoá")
	}

	// Một tài khoản chỉ được chơi một máy tại một thời điểm. Thông báo phải nói
	// rõ MÁY NÀO để nhân viên xử lý được ngay, và bằng tiếng Việt vì nó hiện
	// thẳng lên màn hình khoá của khách.
	var current model.MachineSession
	err := s.db.Where("member_id = ? AND is_active = ?", req.MemberID, true).First(&current).Error
	if err == nil {
		otherCode := current.MachineCode
		if otherCode == "" {
			s.db.Model(&model.Machine{}).Where("id = ?", current.MachineID).
				Pluck("machine_code", &otherCode)
		}
		if otherCode != "" {
			return nil, fmt.Errorf("tài khoản đang chơi ở máy %s — hãy trả máy đó trước", otherCode)
		}
		return nil, errors.New("tài khoản đang chơi ở một máy khác — hãy trả máy đó trước")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := CheckMemberMayPlay(s.db, s.curfew, &member, req.ComboPurchaseID != ""); err != nil {
		return nil, err
	}

	tx := s.db.Begin()

	// Mọi kiểm tra ở trên chạy trên bản đọc NGOÀI transaction. Hai lệnh mở phiên
	// song song — nhân viên bấm ở quầy đúng lúc khách tự đăng nhập ở máy trạm —
	// đều thấy máy còn trống và tài khoản chưa chơi ở đâu, rồi cả hai cùng mở
	// phiên. Khoá và kiểm lại ở đây là chỗ duy nhất chặn được.
	//
	// Thứ tự khoá là hội viên TRƯỚC rồi mới tới máy, khớp với EndSessionAt
	// (phiên → hội viên → máy), nếu không hai hàm sẽ khoá chéo nhau.
	var lockedMember model.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", member.ID).First(&lockedMember).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("không tìm thấy hội viên hoặc tài khoản đã bị khoá")
	}
	var dangChoi int64
	if err := tx.Model(&model.MachineSession{}).
		Where("member_id = ? AND is_active = ?", member.ID, true).Count(&dangChoi).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if dangChoi > 0 {
		tx.Rollback()
		return nil, errors.New("tài khoản đang chơi ở một máy khác — hãy trả máy đó trước")
	}

	var lockedMachine model.Machine
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", machine.ID).First(&lockedMachine).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("không tìm thấy máy")
	}
	// Đếm phiên đang chạy chứ KHÔNG tin cột status.
	//
	// status là cột bị nhiều thứ ghi đè, trong đó có tác vụ nền
	// machines:mark-offline: máy nào ngừng gửi nhịp tim quá hạn thì bị đặt về
	// "offline", kể cả khi khách đang ngồi chơi (máy trạm treo, rớt mạng, agent
	// bị tắt). Khi đó chốt chặn cũ không còn nổ và quầy mở được phiên THỨ HAI
	// trên cùng một máy: hai người bị tính tiền cho một chỗ ngồi. Bảng phiên mới
	// là nguồn sự thật — phía hội viên ngay bên trên cũng đếm theo cách này.
	var dangDung int64
	if err := tx.Model(&model.MachineSession{}).
		Where("machine_id = ? AND is_active = ?", lockedMachine.ID, true).Count(&dangDung).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if dangDung > 0 {
		tx.Rollback()
		return nil, fmt.Errorf("máy %s đang có người chơi — hãy kết thúc phiên đó trước", lockedMachine.MachineCode)
	}

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

	// Giữ con trỏ tới gói để gắn phiên vào nó SAU khi phiên đã được tạo và có ID.
	var dungGoi *model.ComboPurchase

	if req.ComboPurchaseID != "" {
		var purchase model.ComboPurchase
		if err := tx.Where("id = ?", req.ComboPurchaseID).First(&purchase).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("không tìm thấy lượt mua gói cước")
		}

		if !purchase.Activated {
			tx.Rollback()
			return nil, errors.New("gói cước này chưa được kích hoạt")
		}

		var combo model.Combo
		if err := tx.Where("id = ?", purchase.ComboID).First(&combo).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("không tìm thấy gói cước")
		}

		session.ComboID = &purchase.ID
		session.ComboType = combo.Type

		if slotClock := clockValue(combo.SlotEnd); slotClock != "" {
			now := utils.VietnamTime()
			parts := strings.Split(slotClock, ":")
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
		dungGoi = &purchase
	}

	// Đơn giá và mốc hết tiền phải có NGAY từ lúc mở máy, không phải chờ lượt
	// tính đầu tiên: khách vừa ngồi xuống đã nhìn đồng hồ đếm ngược. PricePerHour
	// vốn chỉ được ghi lúc trả máy nên phiên đang chạy luôn hiện 0.
	if cost, err := s.CalculateCost(machine.ID, member.ID, utils.VietnamTime(), 0); err == nil {
		session.PricePerHour = cost.PricePerHour
		session.AffordableUntil = affordableUntil(&session, &member, session.StartedAt)
	}

	if err := tx.Create(&session).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Gắn phiên vào gói SAU khi phiên đã có ID.
	//
	// Lệnh này vốn nằm ngay chỗ đọc gói, tức TRƯỚC tx.Create ở trên, nên
	// session.ID còn rỗng và PostgreSQL từ chối "" cho cột uuid. Lỗi lại không
	// được kiểm, nên transaction hỏng âm thầm và mọi lệnh sau đó trả về
	// "current transaction is aborted (SQLSTATE 25P02)" — người dùng nhận đúng
	// chuỗi đó, còn mở phiên bằng gói trả trước thì không bao giờ thành công.
	if dungGoi != nil {
		if err := tx.Model(dungGoi).Update("current_session_id", session.ID).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Lần ghé gần nhất: cột khai từ đầu nhưng không nơi nào ghi, nên không cách
	// nào biết khách nào lâu rồi không tới.
	if err := tx.Model(&member).Update("last_visit_at", session.StartedAt).Error; err != nil {
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
		// Chỉ quản trị và ĐÚNG máy đang chơi được nhận. Broadcast thì tiện nhưng
		// đẩy member_name của khách này sang màn hình máy của khách khác.
		s.hub.SendToAdminsAndMachine(machine.MachineCode, hub.Event{
			Type: "session:started",
			Data: map[string]interface{}{
				"session_id":   session.ID,
				"machine_id":   session.MachineID,
				"member_id":    session.MemberID,
				"machine_code": machine.MachineCode,
				"member_name":  member.FullName,
				"started_at":   session.StartedAt,
			},
		})
	}

	return resp, nil
}

// BackfillChargedAmount đánh dấu phần đã chơi TRƯỚC khi bản trừ-tiền-theo-phút
// được triển khai là "đã thu", mà không trừ đồng nào.
//
// Không có bước này, lượt tính đầu tiên sau khi triển khai sẽ thấy một phiên đã
// chạy ba tiếng với charged_amount = 0 và trừ một cục ba tiếng tiền — khách có
// thể bị đá ra máy ngay lập tức vì không đủ số dư.
//
// Ngưỡng hai phút phân biệt "phiên có từ trước bản này" với phiên vừa mở. Chạy
// lại ở những lần khởi động sau vô hại: phiên đã có charged_amount > 0.
func (s *SessionService) BackfillChargedAmount() (int, error) {
	cutoff := utils.VietnamTime().Add(-2 * time.Minute)

	var sessions []model.MachineSession
	if err := s.db.Where("is_active = ? AND charged_amount = 0 AND started_at < ?", true, cutoff).
		Find(&sessions).Error; err != nil {
		return 0, err
	}

	done := 0
	for i := range sessions {
		sess := &sessions[i]
		memberID := ""
		if sess.MemberID != nil {
			memberID = *sess.MemberID
		}
		elapsed := int(utils.VietnamTime().Sub(sess.StartedAt).Minutes())
		cost, err := s.CalculateCost(sess.MachineID, memberID, sess.StartedAt, billableMinutes(sess, elapsed))
		if err != nil {
			continue
		}
		if err := s.db.Model(sess).Update("charged_amount", cost.FinalCost).Error; err != nil {
			continue
		}
		done++
	}
	return done, nil
}

// ChargeTickResult cho người gọi biết có phải dừng phiên hay không.
type ChargeTickResult struct {
	Charged   int64
	OutOfCash bool // số dư đã cạn, phải trả máy
}

// ChargeTick trừ tiền cho một phiên tới thời điểm hiện tại.
//
// Gọi mỗi phút từ scheduler. Khoá dòng phiên bằng FOR UPDATE trong suốt lượt
// tính: trước đây MachineSession không có khoá nào, nên nhân viên bấm "Trả máy"
// đúng lúc lượt tính chạy là cả hai cùng trừ tiền.
//
// Không cộng TotalPlayedMinutes ở đây — số phút đã chơi chỉ cộng một lần lúc trả
// máy, cộng mỗi phút rồi lại cộng trọn phiên là đếm gấp đôi.
func (s *SessionService) ChargeTick(sessionID string) (*ChargeTickResult, error) {
	res := &ChargeTickResult{}

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var session model.MachineSession
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND is_active = ?", sessionID, true).First(&session).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return res, nil // phiên vừa được đóng bởi đường khác
		}
		return nil, err
	}

	var machine model.Machine
	if err := tx.Where("id = ?", session.MachineID).First(&machine).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	memberID := ""
	if session.MemberID != nil {
		memberID = *session.MemberID
	}

	now := utils.VietnamTime()
	elapsed := int(now.Sub(session.StartedAt).Minutes())
	if elapsed < 0 {
		elapsed = 0
	}

	cost, err := s.CalculateCost(machine.ID, memberID, session.StartedAt, billableMinutes(&session, elapsed))
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	charge, err := chargeSessionTo(tx, &session, cost.FinalCost, machine.MachineCode, 0)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	res.Charged = charge.Charged

	// Đơn giá được cập nhật mỗi lượt: phiên vắt qua ranh giới khung giờ cao điểm
	// thì màn hình phải đổi theo giá đang thật sự áp dụng.
	session.PricePerHour = cost.PricePerHour
	var balanceAfter, bonusAfter int64
	coMember := false
	if memberID != "" {
		var member model.Member
		if err := tx.Where("id = ?", memberID).First(&member).Error; err == nil {
			coMember = true
			balanceAfter, bonusAfter = member.Balance, member.BonusBalance
			session.AffordableUntil = affordableUntil(&session, &member, now)
			// Hết tiền là hết giờ. Không đợi tới lúc trả máy mới lộ ra khoản nợ.
			if session.PricePerHour > 0 && member.Balance+member.BonusBalance <= 0 &&
				billableMinutes(&session, elapsed+1) > billableMinutes(&session, elapsed) {
				res.OutOfCash = true
			}
		}
	}

	if err := tx.Save(&session).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// Báo cho màn hình biết tiền vừa đi đâu.
	//
	// Trước đây lượt tính này im lặng tuyệt đối: máy trạm chỉ đọc số dư một lần
	// lúc đăng nhập, nên con số trên màn hình khách đứng yên trong khi tiền
	// thật rút đi mỗi phút. Chơi ba tiếng ở giá 20.000₫/giờ là màn hình báo dư
	// 60.000₫ so với thực tế, rồi máy khoá "đột ngột" ở đúng lúc khách đang tin
	// mình vẫn còn tiền.
	//
	// Gửi SAU khi commit, không phải trong giao dịch: gửi trước thì một lần
	// rollback sẽ để lại con số không có thật trên màn hình.
	//
	// Gửi giá trị TUYỆT ĐỐI chứ không phải phần chênh — nhờ vậy một sự kiện rơi
	// mất (máy trạm mất mạng chốc lát) tự lành ở nhịp kế tiếp, không cần đồng bộ
	// lại gì cả.
	if s.hub != nil && coMember && charge.Charged > 0 {
		data := map[string]interface{}{
			"member_id":     memberID,
			"balance":       balanceAfter,
			"bonus_balance": bonusAfter,
		}
		// Mốc hết tiền do máy chủ tính, đã tính cả thời lượng tối thiểu và gói
		// khung giờ. Máy trạm tự ước lượng được, nhưng ước lượng đó không biết
		// hai thứ vừa kể.
		if session.AffordableUntil != nil {
			data["affordable_until"] = session.AffordableUntil
		}
		// Chỉ gửi tới đúng máy đó, KHÔNG phát kèm cho trang quản trị. Sự kiện
		// nạp tiền có gửi cho quản trị, nhưng nó hiếm; cái này bắn mỗi phút cho
		// mỗi phiên — một quán 50 máy là 3.000 gói tin mỗi giờ vào mỗi trình
		// duyệt quản trị, mà bên đó không có gì nghe.
		s.hub.SendToMachine(machine.MachineCode, hub.Event{
			Type: "balance:updated",
			Data: data,
		})
	}
	return res, nil
}

func (s *SessionService) EndSession(id string) (*EndSessionResponse, error) {
	return s.EndSessionAt(id, utils.VietnamTime())
}

// rebootStaleMargin: phiên bắt đầu trước mốc khởi động máy quá ngần này thì coi
// là phiên còn sót từ trước lần reboot. Chỉ để nuốt độ lệch đồng hồ vài giây —
// bình thường máy khởi động RỒI khách mới đăng nhập, nên booted_at luôn nằm
// trước started_at cả phút; chỉ khi reboot thật booted_at mới nhảy vượt qua.
const rebootStaleMargin = 2 * time.Minute

// sessionStaleAfterReboot: phiên có bắt đầu TRƯỚC lần khởi động hiện tại của máy
// không (quá ngưỡng lệch đồng hồ).
//
// Bình thường máy khởi động rồi khách mới đăng nhập, nên bootedAt luôn nằm
// trước startedAt cả phút — trả về false, không đụng phiên đang chạy. Chỉ khi
// máy reboot GIỮA phiên, bootedAt mới nhảy vượt qua startedAt. uptime chỉ giảm
// khi reboot nên không có false dương: không có cách nào bootedAt tự trôi tới
// hiện tại mà máy không thật sự bật lại.
func sessionStaleAfterReboot(startedAt, bootedAt time.Time) bool {
	return bootedAt.After(startedAt.Add(rebootStaleMargin))
}

// EndSessionsStaleAfterReboot đóng mọi phiên đang chạy mà máy của nó đã khởi
// động lại SAU khi phiên bắt đầu.
//
// Đây là hệ quả của đĩa đóng băng (diskless): reboot đưa máy về màn hình khoá,
// nhưng phiên vẫn sống trên máy chủ và VẪN tính tiền. Không đóng thì máy nằm
// không ở màn hình khoá mà đồng hồ vẫn chạy, và khách kế tiếp không đăng nhập
// được vì máy đang "in_use". Chốt sổ tại đúng lúc reboot (booted_at), không
// phải bây giờ, để không tính phần máy nằm không giữa lúc bật lại.
//
// machineID rỗng: quét mọi máy — tác vụ định kỳ dùng cho trường hợp reboot rồi
// bỏ đó không ai ngồi. Có machineID: chỉ máy đó — đường đăng nhập dùng để phiên
// mới mở được ngay thay vì báo "máy đang bận".
func (s *SessionService) EndSessionsStaleAfterReboot(machineID string) (int, error) {
	type joined struct {
		SessionID string
		StartedAt time.Time
		BootedAt  time.Time
	}

	q := s.db.Model(&model.MachineSession{}).
		Select("machine_sessions.id AS session_id, machine_sessions.started_at AS started_at, machines.booted_at AS booted_at").
		Joins("JOIN machines ON machines.id = machine_sessions.machine_id").
		Where("machine_sessions.is_active = ? AND machines.booted_at IS NOT NULL", true)
	if machineID != "" {
		q = q.Where("machine_sessions.machine_id = ?", machineID)
	}

	var rows []joined
	if err := q.Scan(&rows).Error; err != nil {
		return 0, err
	}

	closed := 0
	for _, r := range rows {
		if !sessionStaleAfterReboot(r.StartedAt, r.BootedAt) {
			continue
		}
		if _, err := s.EndSessionAt(r.SessionID, r.BootedAt); err != nil {
			log.Printf("[session] không đóng được phiên sót sau reboot %s: %v", r.SessionID, err)
			continue
		}
		closed++
	}
	return closed, nil
}

// EndSessionsOnOfflineMachines đóng phiên trên máy đã MẤT TÍN HIỆU quá timeout.
//
// Máy tắt (hoặc rút mạng) mà không đăng xuất thì heartbeat ngừng, nhưng tiền do
// máy chủ tính nên phiên vẫn chạy tới khi hết sạch số dư — máy đã tắt vẫn bị
// tính hàng chục tiếng. Đây là chốt chặn: không nghe thấy máy quá timeout thì
// coi như khách đã rời, đóng phiên.
//
// Chốt sổ tại NHỊP TIM CUỐI, không phải bây giờ: khách không bị tính phần máy
// đã tắt. Chỉ xét máy TỪNG báo cáo (last_heartbeat khác NULL) — máy chưa cài
// máy trạm bao giờ (nhân viên tự mở phiên) thì để yên, không phải việc của lớp
// này.
//
// KHÔNG đụng máy vừa reboot: máy đó đang báo cáo lại nên last_heartbeat mới
// tinh, không rơi vào diện này; lớp reboot lo nó.
func (s *SessionService) EndSessionsOnOfflineMachines(timeout time.Duration) (int, error) {
	cutoff := utils.VietnamTime().Add(-timeout)

	type joined struct {
		SessionID     string
		LastHeartbeat time.Time
	}
	var rows []joined
	err := s.db.Model(&model.MachineSession{}).
		Select("machine_sessions.id AS session_id, machines.last_heartbeat AS last_heartbeat").
		Joins("JOIN machines ON machines.id = machine_sessions.machine_id").
		Where("machine_sessions.is_active = ? AND machines.last_heartbeat IS NOT NULL AND machines.last_heartbeat < ?",
			true, cutoff).
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}

	closed := 0
	for _, r := range rows {
		if _, err := s.EndSessionAt(r.SessionID, r.LastHeartbeat); err != nil {
			log.Printf("[session] không đóng được phiên máy mất tín hiệu %s: %v", r.SessionID, err)
			continue
		}
		closed++
	}
	return closed, nil
}

// EndSessionAt đóng phiên và chốt sổ TỚI thời điểm endedAt, không phải tới bây
// giờ. Đóng phiên bình thường dùng "bây giờ"; đóng phiên còn sót sau khi máy
// reboot thì chốt tại đúng lúc reboot, để khách không bị tính phần máy nằm ở
// màn hình khoá giữa lúc bật lại và lúc máy chủ nhận ra.
func (s *SessionService) EndSessionAt(id string, now time.Time) (*EndSessionResponse, error) {
	var session model.MachineSession
	if err := s.db.Where("id = ? AND is_active = ?", id, true).First(&session).Error; err != nil {
		return nil, errors.New("không tìm thấy phiên đang chạy")
	}

	var machine model.Machine
	if err := s.db.Where("id = ?", session.MachineID).First(&machine).Error; err != nil {
		return nil, errors.New("không tìm thấy máy")
	}

	// endedAt không được sớm hơn lúc phiên bắt đầu: đồng hồ lệch có thể cho ra
	// mốc reboot nằm trước started_at vài giây, và thời lượng âm thì vô nghĩa.
	if now.Before(session.StartedAt) {
		now = session.StartedAt
	}
	duration := now.Sub(session.StartedAt)
	durationMinutes := int(duration.Minutes())

	var memberID string
	if session.MemberID != nil {
		memberID = *session.MemberID
	}

	// Bản cũ mở một transaction chỉ để khoá ComboPurchase rồi commit ngay TRƯỚC
	// khi trừ đồng nào — khoá đó không bảo vệ được gì. Nay số phút phải trả tiền
	// tính từ ảnh chụp trên chính dòng phiên nên không cần đọc bảng gói.
	costBreakdown, err := s.CalculateCost(machine.ID, memberID, session.StartedAt, billableMinutes(&session, durationMinutes))
	if err != nil {
		return nil, err
	}

	tx := s.db.Begin()

	// Khoá phiên rồi đọc lại. Bản đọc ở đầu hàm nằm ngoài transaction, nên hai
	// lệnh kết thúc song song đều thấy is_active=true, đều chạy chargeSessionTo
	// và khách bị trừ tiền hai lần cho cùng số phút.
	var lockedSession model.MachineSession
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND is_active = ?", id, true).First(&lockedSession).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("không tìm thấy phiên đang chạy")
	}
	// chargeSessionTo khấu trừ phần đã thu dọc đường từ chính con số này.
	session.ChargedAmount = lockedSession.ChargedAmount

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

	// Chốt sổ đi qua ĐÚNG hàm mà lượt tính mỗi phút dùng: target là tiền của
	// trọn phiên, phần đã trừ dọc đường được khấu trừ. Không có đường tính tiền
	// thứ hai để mà lệch, và khách không bị thu hai lần cho cùng số phút.
	//
	// Phải chạy TRƯỚC khi lưu phiên: nó đặt session.ChargedAmount, chạy sau thì
	// con số đó không được ghi xuống.
	charge, err := chargeSessionTo(tx, &session, costBreakdown.FinalCost, machine.MachineCode, durationMinutes)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	balanceBefore := charge.BalanceBefore
	balanceAfter := charge.BalanceAfter
	bonusUsed := charge.BonusUsed
	amountUnpaid := charge.AmountUnpaid

	// Phiên đã đóng thì không còn mốc hết tiền nào để đếm ngược.
	session.AffordableUntil = nil

	if err := tx.Save(&session).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	machine.Status = "available"
	if err := tx.Save(&machine).Error; err != nil {
		tx.Rollback()
		return nil, err
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
			"machine_id":   session.MachineID,
			"member_id":    memberID,
			"duration_min": durationMinutes,
			"total_cost":   costBreakdown.FinalCost,
			"machine_code": machine.MachineCode,
		},
	})

	if s.hub != nil {
		// Như session:started — total_cost là tiền của một người, không phải tin
		// tức chung cho cả quán.
		s.hub.SendToAdminsAndMachine(machine.MachineCode, hub.Event{
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
		BonusUsed:       bonusUsed,
		AmountUnpaid:    amountUnpaid,
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
	// Unscoped: xem lại một phiên trên máy đã gỡ khỏi danh sách vẫn phải thấy
	// máy nào — phiên là chứng từ, không phải thao tác trên máy còn sống.
	if err := s.db.Unscoped().Where("id = ?", session.MachineID).First(&machine).Error; err == nil {
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
	var newMachine model.Machine
	if err := s.db.Where("id = ? AND is_active = ?", newMachineID, true).First(&newMachine).Error; err != nil {
		return nil, errors.New("không tìm thấy máy mới")
	}
	if newMachine.Status == "in_use" {
		return nil, errors.New("máy mới đang có người dùng")
	}

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
			// Unscoped: máy vừa bị gỡ khỏi danh sách mà khách còn đang ngồi thì
			// phiên vẫn chạy — bỏ trống mã máy làm nhân viên không biết đuổi ai ở đâu.
			if err := s.db.Unscoped().Where("id = ?", session.MachineID).First(&m).Error; err == nil {
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

// CalculateCost tính tiền cho khoảng [batDau, batDau+durationMinutes).
//
// batDau là bắt buộc vì giá có thể đổi GIỮA phiên: bảng giá theo khung giờ cho
// phép quán đặt giá cao điểm 22:00–02:00, và một phiên bình thường của quán net
// vắt qua mốc đó. Bản cũ chỉ nhận số phút rồi tra MỘT đơn giá tại thời điểm
// tính tiền, nên toàn bộ thời gian đã chơi bị tính lại theo giá mới — khách
// ngồi từ 20:00 tới 23:00 bị thu giá cao điểm cho cả ba tiếng.
func (s *SessionService) CalculateCost(machineID string, memberID string, batDau time.Time, durationMinutes int) (*CostBreakdown, error) {
	var machine model.Machine
	if err := s.db.Where("id = ?", machineID).First(&machine).Error; err != nil {
		return nil, errors.New("không tìm thấy máy")
	}

	groupName := ""
	pricePerHour := int64(0)
	if machine.GroupID != nil {
		var group model.MachineGroup
		if err := s.db.Where("id = ?", *machine.GroupID).First(&group).Error; err == nil {
			groupName = group.Name
			pricePerHour = group.PricePerHour
		}
	}

	var member *model.Member
	if memberID != "" {
		var m model.Member
		if err := s.db.Where("id = ?", memberID).First(&m).Error; err == nil {
			member = &m
		}
	}

	// Rates are resolved most-specific first: a member-tier rate for this
	// machine group beats the time-of-day rate, which beats the group's base
	// rate.
	minBilledMinutes := 0
	coGiaTheoHang := false
	if machine.GroupID != nil {
		if mp, ok := s.memberGroupRate(*machine.GroupID, member); ok {
			pricePerHour = mp.PricePerHour
			minBilledMinutes = mp.MinDuration
			coGiaTheoHang = true
		}
	}

	billedMinutes := durationMinutes
	// MinDuration là số phút tối thiểu được tính tiền của bảng giá theo hạng:
	// khách ngồi 10 phút ở bảng giá tối thiểu 30 phút vẫn trả tiền 30 phút.
	// Chỉ nâng khi khách thực sự có chơi — phiên 0 phút không được sinh tiền.
	if billedMinutes > 0 && minBilledMinutes > billedMinutes {
		billedMinutes = minBilledMinutes
	}

	gross := int64(0)
	switch {
	case coGiaTheoHang:
		// Giá theo hạng hội viên không đổi theo giờ nên không cần cắt đoạn.
		if billedMinutes > 0 {
			gross = int64(math.Ceil(float64(billedMinutes) * float64(pricePerHour) / 60.0))
		}
	case machine.GroupID != nil:
		khung := s.bangGiaKhungGio(*machine.GroupID)
		gross = tinhTienTheoDoan(khung, pricePerHour, batDau, billedMinutes)
		// Đơn giá đem đi hiển thị và chụp vào dòng phiên là giá ĐANG áp dụng ở
		// cuối khoảng, tức giá của phút sắp chơi tiếp. Tiền đã tính vẫn giữ
		// nguyên theo từng đoạn; con số này chỉ để màn hình nói đúng "bây giờ
		// đang tính bao nhiêu một giờ".
		pricePerHour = giaTaiThoiDiem(khung, pricePerHour, batDau.Add(time.Duration(billedMinutes)*time.Minute))
	}

	// The member-tier discount applies to whatever rate was resolved above.
	discountPercent := float64(0)
	if member != nil && member.GroupID != nil {
		var mg model.MemberGroup
		if err := s.db.Where("id = ?", *member.GroupID).First(&mg).Error; err == nil {
			discountPercent = mg.DiscountPercent
		}
	}
	discountAmount := int64(0)
	if discountPercent > 0 && gross > 0 {
		if discountPercent > 100 {
			discountPercent = 100
		}
		discountAmount = int64(math.Floor(float64(gross) * discountPercent / 100.0))
	}

	return &CostBreakdown{
		MachineGroupName: groupName,
		NoPricing:        pricePerHour <= 0,
		PricePerHour:     pricePerHour,
		DurationMinutes:  durationMinutes,
		BilledMinutes:    billedMinutes,
		GrossCost:        gross,
		DiscountPercent:  discountPercent,
		DiscountAmount:   discountAmount,
		FinalCost:        gross - discountAmount,
	}, nil
}

// memberGroupRate returns the machine-group x member-group price row in effect
// today. Trả về cả dòng chứ không chỉ giá vì người gọi còn cần MinDuration.
func (s *SessionService) memberGroupRate(machineGroupID string, member *model.Member) (model.MachinePrice, bool) {
	if member == nil || member.GroupID == nil {
		return model.MachinePrice{}, false
	}

	today := utils.VietnamTime().Format("2006-01-02")
	var mp model.MachinePrice
	err := s.db.
		Where("machine_group_id = ? AND member_group_id = ?", machineGroupID, *member.GroupID).
		Where("effective_from <= ?", today).
		// effective_to là cột date: so với chuỗi rỗng làm PostgreSQL báo lỗi cú
		// pháp, và lỗi đó bị nuốt ở nhánh `err != nil` bên dưới — giá theo hạng
		// hội viên sẽ im lặng không bao giờ áp dụng. Để trống nay là NULL.
		Where("effective_to IS NULL OR effective_to >= ?", today).
		Order("effective_from DESC").
		First(&mp).Error
	if err != nil {
		return model.MachinePrice{}, false
	}
	return mp, true
}

// bangGiaKhungGio đọc mọi khung giờ đang bật của nhóm máy, KHÔNG lọc theo thứ
// trong tuần: một phiên vắt qua nửa đêm nằm trên hai ngày khác nhau, lọc sẵn ở
// SQL theo thứ hôm nay thì phần sau nửa đêm mất giá của nó.
func (s *SessionService) bangGiaKhungGio(machineGroupID string) []model.TimeBasedPricing {
	var rows []model.TimeBasedPricing
	if err := s.db.
		Where("machine_group_id = ? AND is_active = ?", machineGroupID, true).
		Find(&rows).Error; err != nil {
		return nil
	}
	return rows
}

// giaTaiThoiDiem trả về giá áp dụng đúng một thời điểm, giaGoc là giá của nhóm
// máy khi không khung nào phủ.
//
// So khung giờ trong Go chứ không trong SQL, vì điều kiện
// `start_time <= now AND end_time > now` KHÔNG khớp được khung vắt qua nửa đêm
// — một khung cao điểm 22:00–02:00 sẽ không bao giờ áp dụng. Quán net chạy
// xuyên đêm nên đó là khung phổ biến nhất.
func giaTaiThoiDiem(khung []model.TimeBasedPricing, giaGoc int64, at time.Time) int64 {
	gio := at.Format("15:04")
	thu := int(at.Weekday())
	for _, tp := range khung {
		if tp.DayOfWeek != thu {
			continue
		}
		// withinCurfew là so khung giờ trong ngày có xử lý vắt qua nửa đêm.
		if withinCurfew(gio, clockHHMM(tp.StartTime), clockHHMM(tp.EndTime)) {
			return tp.PricePerHour
		}
	}
	return giaGoc
}

// tinhTienTheoDoan cộng tiền của từng phút, mỗi phút giữ giá của khung giờ mà
// nó thuộc về.
//
// Đi theo từng phút chứ không dựng danh sách đoạn: một phiên dài nhất cũng chỉ
// vài trăm vòng lặp trên dữ liệu đã nằm sẵn trong bộ nhớ, đổi lại không phải
// viết logic cắt đoạn — thứ dễ sai đúng ở chỗ ranh giới và ở khung vắt nửa đêm.
//
// Làm tròn lên ở TỔNG chứ không ở từng phút, giữ đúng cách bản cũ làm tròn để
// một phiên không đổi khung giờ vẫn ra đúng con số cũ.
func tinhTienTheoDoan(khung []model.TimeBasedPricing, giaGoc int64, batDau time.Time, soPhut int) int64 {
	if soPhut <= 0 {
		return 0
	}
	tong := 0.0
	for i := 0; i < soPhut; i++ {
		gia := giaTaiThoiDiem(khung, giaGoc, batDau.Add(time.Duration(i)*time.Minute))
		tong += float64(gia) / 60.0
	}
	return int64(math.Ceil(tong))
}

// activeDuration trả về số phút đã chơi.
//
// Cột duration_minutes chỉ được ghi lúc TRẢ máy, nên phiên đang chạy luôn trả
// null: cột "Thời gian" ở Bảng điều khiển và trang Phiên bỏ trống đúng lúc cần
// nhất — con số quan trọng nhất của quán net là khách đang ngồi bao lâu rồi.
func activeDuration(s *model.MachineSession) *int {
	if !s.IsActive {
		return s.DurationMinutes
	}
	elapsed := int(utils.VietnamTime().Sub(s.StartedAt).Minutes())
	if elapsed < 0 {
		elapsed = 0
	}
	return &elapsed
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
		DurationMinutes:  activeDuration(s),
		TotalCost:        s.TotalCost,
		IsOvernight:      s.IsOvernight,
		IsActive:         s.IsActive,
		CreatedAt:        s.CreatedAt,

		ChargedAmount:   s.ChargedAmount,
		AffordableUntil: s.AffordableUntil,

		MachineGroupID:   s.MachineGroupID,
		MemberGroupID:    s.MemberGroupID,
		MachineGroupName: s.MachineGroupName,
		PricePerHour:     s.PricePerHour,
		BilledMinutes:    s.BilledMinutes,
	}
}
