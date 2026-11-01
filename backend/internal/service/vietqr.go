package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"strconv"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

// Ngân hàng nhận tiền QR mà điện thoại ở quầy đọc được thông báo. BIN do Ngân
// hàng Nhà nước cấp, nằm trong mã VietQR; Package là tên gói app trên Android,
// điện thoại dùng để biết thông báo nào là của ngân hàng.
type qrBank struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	BIN     string `json:"bin"`
	Package string `json:"package"`
}

var qrBanks = map[string]qrBank{
	"mb":  {Code: "mb", Name: "MB Bank", BIN: "970422", Package: "com.mbmobile"},
	"vcb": {Code: "vcb", Name: "Vietcombank", BIN: "970436", Package: "com.VCB"},
}

// tlv ghép một trường EMVCo: mã 2 chữ số, độ dài 2 chữ số, giá trị.
func tlv(id, value string) string {
	return fmt.Sprintf("%s%02d%s", id, len(value), value)
}

// crc16CCITT là CRC-16/CCITT-FALSE (đa thức 0x1021, khởi tạo 0xFFFF) mà chuẩn
// EMVCo dùng cho trường 63. Giá trị kiểm chuẩn: "123456789" → 0x29B1.
func crc16CCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// vietQRPayload dựng chuỗi VietQR động (chuẩn NAPAS trên nền EMVCo): chuyển
// nhanh 24/7 tới tài khoản, có sẵn số tiền và nội dung. Thứ tự trường lấy theo
// các thư viện VietQR công khai; phải quét thử bằng app ngân hàng thật.
func vietQRPayload(bin, accountNo string, amount int64, info string) string {
	beneficiary := tlv("00", bin) + tlv("01", accountNo)
	merchant := tlv("00", "A000000727") + tlv("01", beneficiary) + tlv("02", "QRIBFTTA")
	s := tlv("00", "01") +
		tlv("01", "12") + // 12 = mã động, dùng một lần
		tlv("38", merchant) +
		tlv("53", "704") + // VND
		tlv("54", strconv.FormatInt(amount, 10)) +
		tlv("58", "VN") +
		tlv("62", tlv("08", info)) +
		"6304"
	return s + fmt.Sprintf("%04X", crc16CCITT([]byte(s)))
}

// qrPNGDataURL vẽ chuỗi thành ảnh QR (data URL) để máy trạm và trang quản lý
// hiện thẳng, không cần thư viện vẽ QR ở phía giao diện.
func qrPNGDataURL(payload string, size int) (string, error) {
	code, err := qr.Encode(payload, qr.M, qr.Auto)
	if err != nil {
		return "", err
	}
	scaled, err := barcode.Scale(code, size, size)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
