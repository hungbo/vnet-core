package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Nạp tặng tự động: mỗi lần hội viên nạp tiền thật, quán tặng thêm vào số dư
// khuyến mãi theo bảng mốc ở Cài đặt > Nạp tiền.
//
// Áp cho nạp ở quầy (MemberService.Topup) và đơn nạp khách gửi từ máy trạm khi
// nhân viên duyệt (OrderService.processTopupOrder). Không áp cho nút "Tặng tiền"
// (đường nội bộ bonus_balance): tặng trên tiền tặng là cộng dồn vô hạn.
//
// Tiền tặng ghi thành một dòng topup_bonus riêng với phương thức bonus_balance,
// nên báo cáo doanh thu và két cuối ca (lọc đúng chuỗi "cash") không đếm nó.

const (
	TopupGroup          = "topup"
	TopupBonusTiersKey  = "bonus_tiers"
	TopupBonusFixed     = "fixed"   // tặng đúng Value đồng
	TopupBonusPercent   = "percent" // tặng Value% số tiền nạp
	maxTopupBonusTiers  = 20
	maxTopupBonusPct    = 1000
	minTopupBonusAmount = 1000
)

// TopupBonusTier là một mốc: nạp từ Min đồng trở lên thì tặng theo Type/Value.
type TopupBonusTier struct {
	Min   int64  `json:"min"`
	Type  string `json:"type"`
	Value int64  `json:"value"`
}

// chuanHoaMocNapTang kiểm và sắp xếp bảng mốc trước khi lưu. Bảng rỗng là tắt
// tính năng.
func chuanHoaMocNapTang(raw interface{}) ([]TopupBonusTier, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, errors.New("bảng nạp tặng không đúng dạng")
	}
	var tiers []TopupBonusTier
	if string(b) != "null" {
		if err := json.Unmarshal(b, &tiers); err != nil {
			return nil, errors.New("bảng nạp tặng không đúng dạng")
		}
	}
	if len(tiers) > maxTopupBonusTiers {
		return nil, fmt.Errorf("bảng nạp tặng tối đa %d mốc", maxTopupBonusTiers)
	}
	seen := map[int64]bool{}
	for _, t := range tiers {
		if t.Min < minTopupBonusAmount {
			return nil, fmt.Errorf("mốc nạp phải từ %sđ trở lên", formatMoney(minTopupBonusAmount))
		}
		if seen[t.Min] {
			return nil, fmt.Errorf("mốc nạp %sđ bị trùng", formatMoney(t.Min))
		}
		seen[t.Min] = true
		switch t.Type {
		case TopupBonusFixed:
			if t.Value <= 0 {
				return nil, fmt.Errorf("mốc %sđ: số tiền tặng phải lớn hơn 0", formatMoney(t.Min))
			}
		case TopupBonusPercent:
			if t.Value <= 0 || t.Value > maxTopupBonusPct {
				return nil, fmt.Errorf("mốc %sđ: phần trăm tặng phải từ 1 đến %d", formatMoney(t.Min), maxTopupBonusPct)
			}
		default:
			return nil, fmt.Errorf("mốc %sđ: kiểu tặng phải là số tiền hoặc phần trăm", formatMoney(t.Min))
		}
	}
	sort.Slice(tiers, func(i, j int) bool { return tiers[i].Min < tiers[j].Min })
	if tiers == nil {
		tiers = []TopupBonusTier{}
	}
	return tiers, nil
}

// docMocNapTang đọc bảng mốc đã lưu. Lỗi đọc hay dữ liệu hỏng đều coi như không
// có mốc: thiếu tiền tặng còn hơn chặn luôn việc nạp tiền.
func docMocNapTang(db *gorm.DB) []TopupBonusTier {
	raw := settingsGroup(db, TopupGroup)[TopupBonusTiersKey]
	if raw == "" {
		return nil
	}
	var tiers []TopupBonusTier
	if err := json.Unmarshal([]byte(raw), &tiers); err != nil {
		return nil
	}
	return tiers
}

// tinhTienTangNap chọn mốc cao nhất mà số tiền nạp đạt tới và trả số tiền tặng.
// Không đạt mốc nào thì trả 0 và nil.
func tinhTienTangNap(tiers []TopupBonusTier, amount int64) (int64, *TopupBonusTier) {
	var best *TopupBonusTier
	for i := range tiers {
		if tiers[i].Min <= amount && (best == nil || tiers[i].Min > best.Min) {
			best = &tiers[i]
		}
	}
	if best == nil {
		return 0, nil
	}
	switch best.Type {
	case TopupBonusFixed:
		return best.Value, best
	case TopupBonusPercent:
		return amount * best.Value / 100, best
	}
	return 0, nil
}

// ghiTienTangNap ghi dòng topup_bonus trong transaction của lần nạp. Người gọi
// tự cộng số trả về vào bonus_balance trong cùng câu UPDATE hội viên.
func ghiTienTangNap(tx *gorm.DB, memberID string, amount, balanceAfter, bonusBefore int64, tier *TopupBonusTier, bonus int64, createdBy *string) error {
	moc := formatMoney(tier.Value) + "đ"
	if tier.Type == TopupBonusPercent {
		moc = fmt.Sprintf("%d%%", tier.Value)
	}
	return tx.Create(&model.MemberTransaction{
		MemberID:        memberID,
		TransactionType: "topup_bonus",
		Amount:          bonus,
		BalanceBefore:   balanceAfter,
		BalanceAfter:    balanceAfter,
		BonusBefore:     bonusBefore,
		BonusAfter:      bonusBefore + bonus,
		PaymentMethod:   "bonus_balance",
		Description: fmt.Sprintf("Nạp tặng: nạp %sđ, mốc từ %sđ tặng %s",
			formatMoney(amount), formatMoney(tier.Min), moc),
		CreatedBy: createdBy,
	}).Error
}
