package main

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"unicode/utf16"
)

// XML sai một ký tự thì schtasks từ chối, và lúc đó máy mất luôn lớp bật lại
// dịch vụ mà không ai biết — bộ cài chỉ ghi một dòng nhật ký rồi đi tiếp.
func TestTaskXML_LaXMLHopLe(t *testing.T) {
	for ten, x := range map[string]string{
		"theo nhịp":    taskXMLEvery(`C:\Program Files\VNET Client\vnet-client.exe`),
		"theo sự kiện": taskXMLOnStop(`C:\Program Files\VNET Client\vnet-client.exe`, serviceDisplayName),
	} {
		t.Run(ten, func(t *testing.T) {
			if err := docThuXML(x); err != nil {
				t.Fatalf("không phải XML hợp lệ: %v", err)
			}
		})
	}
}

// Chạy dưới SYSTEM, không phải dưới tài khoản người đang cài: tác vụ phải sống
// cả khi không có ai đăng nhập Windows, và không được để khách dừng.
func TestTaskXML_ChayDuoiSystem(t *testing.T) {
	x := taskXMLEvery("vnet-client.exe")
	if !strings.Contains(x, "<UserId>S-1-5-18</UserId>") {
		t.Error("không chạy dưới SYSTEM (S-1-5-18)")
	}
	if !strings.Contains(x, "<Arguments>--ensure-service</Arguments>") {
		t.Error("không gọi --ensure-service")
	}
}

// Nhịp một phút và lặp vô hạn. StopAtDurationEnd=true là tác vụ tự tắt sau một
// ngày, và lỗi đó chỉ lộ ra sau khi máy đã chạy được 24 tiếng.
func TestTaskXML_LapMoiPhutVoHan(t *testing.T) {
	x := taskXMLEvery("vnet-client.exe")
	if !strings.Contains(x, "<Interval>PT1M</Interval>") {
		t.Error("nhịp lặp không phải một phút")
	}
	if !strings.Contains(x, "<StopAtDurationEnd>false</StopAtDurationEnd>") {
		t.Error("tác vụ sẽ tự dừng sau một khoảng — phải lặp vô hạn")
	}
	if !strings.Contains(x, "<BootTrigger>") {
		t.Error("không kích lúc khởi động máy")
	}
}

// Bộ lọc sự kiện phải nêu ĐÍCH DANH dịch vụ này. Sự kiện 7036 phát ra cho mọi
// dịch vụ trên máy; thiếu bộ lọc là tác vụ chạy hàng trăm lần mỗi ngày.
func TestTaskXMLOnStop_LocDungDichVu(t *testing.T) {
	x := taskXMLOnStop("vnet-client.exe", serviceDisplayName)

	for _, phai := range []string{
		"Service Control Manager",
		"EventID=7036",
		serviceDisplayName,
		"'stopped'",
	} {
		if !strings.Contains(x, phai) {
			t.Errorf("bộ lọc thiếu %q", phai)
		}
	}
	// Truy vấn nằm trong một phần tử XML nên dấu ngoặc nhọn phải được thoát.
	if strings.Contains(x, "<QueryList>") {
		t.Error("truy vấn chưa thoát ký tự — schtasks sẽ đọc nó thành thẻ XML")
	}
}

// Tên dịch vụ có ký tự cần thoát thì XML vẫn phải hợp lệ.
func TestTaskXMLOnStop_ThoatKyTuTrongTen(t *testing.T) {
	x := taskXMLOnStop(`C:\a & b\vnet.exe`, `VNET <Agent> & Co`)

	if err := docThuXML(x); err != nil {
		t.Fatalf("tên có ký tự đặc biệt làm hỏng XML: %v", err)
	}
	if strings.Contains(x, "<Agent>") {
		t.Error("tên chưa được thoát — đọc thành thẻ XML")
	}
}

// schtasks /Create /XML từ chối tệp UTF-8 trên nhiều bản Windows, và thông báo
// lỗi không nhắc gì tới mã hoá.
func TestUTF16BOM(t *testing.T) {
	b := utf16BOMBytes("AB")

	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xFE {
		t.Fatalf("thiếu BOM little-endian: % x", b[:2])
	}
	if got := string(utf16.Decode(giaiMaLE(b[2:]))); got != "AB" {
		t.Errorf("giải mã ra %q", got)
	}

	// Ký tự ngoài BMP phải thành cặp surrogate, không được cắt cụt.
	b = utf16BOMBytes("🍜")
	if len(b)-2 != 4 {
		t.Errorf("emoji ra %d byte, mong 4 (cặp surrogate)", len(b)-2)
	}
}

// docThuXML phân tích chuỗi để bắt lỗi cú pháp.
//
// Phải tự cấp CharsetReader: XML khai encoding="UTF-16" vì schtasks đòi thế,
// nhưng chuỗi ở đây vẫn là UTF-8 trong bộ nhớ — nó chỉ thành UTF-16 lúc
// utf16BOMBytes ghi ra tệp. Không có hàm này thì bộ phân tích của Go từ chối
// ngay ở dòng khai báo và bài kiểm đỏ vì một lý do chẳng liên quan gì.
func docThuXML(x string) error {
	d := xml.NewDecoder(strings.NewReader(x))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		_, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func giaiMaLE(b []byte) []uint16 {
	out := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		out = append(out, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return out
}
