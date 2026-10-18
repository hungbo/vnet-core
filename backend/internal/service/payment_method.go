package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Danh sách phương thức thanh toán quán nhận tiền từ khách, chỉnh ở tab
// "Thanh toán" của trang Cài đặt và lưu thành một mảng JSON trong
// system_settings (nhóm "payment", khoá "methods").
//
// Trước đây mỗi màn hình tự viết cứng danh sách riêng và mỗi API tự kiểm một
// danh sách khác: nạp tiền nhận ví điện tử, thanh toán đơn thì không, mua gói thì
// nhận bất kỳ chuỗi nào. Mọi nơi giờ đọc chung một nguồn này.
const (
	PaymentGroup      = "payment"
	PaymentMethodsKey = "methods"

	// PaymentMethodCash luôn bật: đối soát két cuối ca dựa vào nó.
	PaymentMethodCash = "cash"

	// Mã sinh ra phải vừa cột ngắn nhất đang lưu phương thức
	// (combo_purchases... sold_payment_method varchar(20)).
	maxPaymentCodeLen = 20
)

type PaymentMethodOption struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

var defaultPaymentMethods = []PaymentMethodOption{
	{Code: PaymentMethodCash, Name: "Tiền mặt", Enabled: true},
	{Code: "transfer", Name: "Chuyển khoản", Enabled: true},
	{Code: "ewallet", Name: "Ví điện tử", Enabled: true},
}

// Các mã này là cách TRỪ tiền nội bộ (số dư, tiền thưởng), không phải tiền
// khách đưa vào quán. Mỗi nơi dùng được tự thêm chúng; cho đặt trùng tên ở cài
// đặt thì báo cáo sẽ tính nhầm thành tiền vào két. "gift_card" và "topup_card"
// thuộc chức năng thẻ đã gỡ, nhưng đơn và giao dịch cũ vẫn mang hai mã này nên
// vẫn không cho đặt lại.
var reservedPaymentCodes = map[string]bool{
	PaymentMethodBalance: true,
	"bonus_balance":      true,
	"gift_card":          true,
	"topup_card":         true,
}

// PaymentMethods trả về danh sách đã cài, hoặc danh sách mặc định khi quán
// chưa từng lưu tab này.
func PaymentMethods(db *gorm.DB) []PaymentMethodOption {
	raw := settingsGroup(db, PaymentGroup)[PaymentMethodsKey]
	var list []PaymentMethodOption
	if raw == "" || json.Unmarshal([]byte(raw), &list) != nil || len(list) == 0 {
		return append([]PaymentMethodOption(nil), defaultPaymentMethods...)
	}
	return list
}

// kiemTraPhuongThucThanhToan từ chối mã không có trong cài đặt hoặc đang tắt.
// Tiền mặt luôn hợp lệ nên không tốn truy vấn.
func kiemTraPhuongThucThanhToan(db *gorm.DB, code string) error {
	if code == PaymentMethodCash {
		return nil
	}
	for _, m := range PaymentMethods(db) {
		if m.Code == code {
			if !m.Enabled {
				return fmt.Errorf("phương thức thanh toán %q đang tắt", m.Name)
			}
			return nil
		}
	}
	return fmt.Errorf("phương thức thanh toán %q không có trong cài đặt", code)
}

// preparePaymentSettings chuẩn hoá danh sách trước khi lưu: sinh mã cho dòng
// mới, chặn mã dành riêng, bỏ trùng và giữ tiền mặt luôn bật.
func preparePaymentSettings(in map[string]interface{}) (map[string]interface{}, error) {
	rawList, ok := in[PaymentMethodsKey]
	if !ok {
		return in, nil
	}
	b, err := json.Marshal(rawList)
	if err != nil {
		return nil, err
	}
	var list []PaymentMethodOption
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, errors.New("danh sách phương thức thanh toán không hợp lệ")
	}

	seen := map[string]bool{}
	out := make([]PaymentMethodOption, 0, len(list)+1)
	for _, m := range list {
		m.Name = strings.TrimSpace(m.Name)
		if m.Name == "" {
			return nil, errors.New("phương thức thanh toán phải có tên")
		}
		m.Code = strings.TrimSpace(m.Code)
		if m.Code == "" {
			m.Code = paymentCodeFromName(m.Name, seen)
		}
		if reservedPaymentCodes[m.Code] {
			return nil, fmt.Errorf("mã %q dành riêng cho hệ thống", m.Code)
		}
		if seen[m.Code] {
			return nil, fmt.Errorf("trùng mã phương thức %q", m.Code)
		}
		seen[m.Code] = true
		if m.Code == PaymentMethodCash {
			m.Enabled = true
		}
		out = append(out, m)
	}
	if !seen[PaymentMethodCash] {
		out = append([]PaymentMethodOption{defaultPaymentMethods[0]}, out...)
	}
	in[PaymentMethodsKey] = out
	return in, nil
}

// paymentCodeFromName: "Quẹt thẻ POS" -> "quet_the_pos".
func paymentCodeFromName(name string, taken map[string]bool) string {
	var b strings.Builder
	for _, r := range strings.ToLower(foldVietnamese(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteRune('_')
		}
	}
	base := strings.Trim(b.String(), "_")
	if base == "" {
		base = "pm"
	}
	if len(base) > maxPaymentCodeLen-3 {
		base = strings.Trim(base[:maxPaymentCodeLen-3], "_")
	}
	code := base
	for i := 2; taken[code] || reservedPaymentCodes[code]; i++ {
		code = fmt.Sprintf("%s_%d", base, i)
	}
	return code
}
