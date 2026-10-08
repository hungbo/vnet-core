//go:build windows

package main

import (
	"testing"
	"unsafe"
)

// Chạy được trên máy Windows có màn hình thật; không có màn hình để đọc (phiên dịch vụ,
// máy chủ không đầu) thì bỏ qua chứ không báo hỏng.
//
// Mọi thao tác GHI ở đây đều ghi lại ĐÚNG giá trị đang có, nên chạy trên máy dev không
// làm đổi màn hình hay chuột của ai.

// Bố cục DEVMODEW đã bị chặn lúc biên dịch (display_windows.go); bài này giữ lại để một
// người đọc kết quả test thấy ngay hai con số quan trọng nhất.
func TestDevModeLayout(t *testing.T) {
	if got := unsafe.Sizeof(devModeW{}); got != devModeSize {
		t.Errorf("sizeof(DEVMODEW) = %d, mong %d", got, devModeSize)
	}
	if got := unsafe.Sizeof(displayDeviceW{}); got != displayDeviceSize {
		t.Errorf("sizeof(DISPLAY_DEVICEW) = %d, mong %d", got, displayDeviceSize)
	}
}

func TestWinDisplayModes_DocDuocMoiThu(t *testing.T) {
	raw, cur, err := winDisplayModes()
	if err != nil {
		t.Skipf("không đọc được màn hình chính: %v", err)
	}
	if cur.Width < displayMinWidth || cur.Height < displayMinHeight || cur.BitsPerPel == 0 {
		t.Fatalf("chế độ hiện tại vô lý: %+v", cur)
	}
	if len(raw) == 0 {
		t.Fatal("EnumDisplaySettings không trả chế độ nào")
	}
	modes := filterDisplayModes(raw, cur.Orientation)
	if cur.BitsPerPel == 32 && !cur.Interlaced {
		if err := validateDisplayMode(modes, cur.DisplayMode); err != nil {
			t.Errorf("chế độ đang chạy phải nằm trong danh sách chọn được: %v", err)
		}
	}
}

// Đổi sang CHÍNH chế độ đang chạy: đi qua CDS_TEST rồi ChangeDisplaySettingsEx thật mà
// màn hình không đổi gì.
func TestWinSetDisplayMode_CheDoHienTai(t *testing.T) {
	_, cur, err := winDisplayModes()
	if err != nil {
		t.Skipf("không đọc được màn hình chính: %v", err)
	}
	if cur.BitsPerPel != 32 || cur.Interlaced {
		t.Skip("chế độ hiện tại không phải 32 bit quét liên tục — đặt lại sẽ làm đổi nó")
	}
	if err := winSetDisplayMode(cur.DisplayMode); err != nil {
		t.Fatalf("đặt lại chế độ đang chạy %s: %v", cur.DisplayMode, err)
	}
	_, after, err := winDisplayModes()
	if err != nil || after.DisplayMode != cur.DisplayMode {
		t.Errorf("sau khi đặt lại: %+v, %v (trước: %+v)", after, err, cur)
	}
}

// ENUM_REGISTRY_SETTINGS là chỗ duy nhất startup() dựa vào để biết máy có đang kẹt ở một
// chế độ động hay không. Bài này chỉ ĐỌC và in hai con số cạnh nhau (go test -v) để người
// chạy trên máy thật thấy registry có khớp chế độ đang chạy không. Không có bài cho
// winRestoreDefault: nó đổi màn hình nếu máy đang ở chế độ động.
func TestWinRegistryMode_DocDuoc(t *testing.T) {
	reg, err := winRegistryMode()
	if err != nil {
		t.Skipf("không đọc được registry màn hình: %v", err)
	}
	_, cur, err := winDisplayModes()
	if err != nil {
		t.Skipf("không đọc được màn hình chính: %v", err)
	}
	t.Logf("registry %s, đang chạy %s, lệch mặc định: %v", reg, cur.DisplayMode, staleDisplayMode(reg, cur.DisplayMode))
}

func TestWinMouse_GhiLaiDungGiaTriCu(t *testing.T) {
	speed, precision, err := winGetMouse()
	if err != nil {
		t.Skipf("không đọc được chuột: %v", err)
	}
	if speed < mouseSpeedMin || speed > mouseSpeedMax {
		t.Fatalf("tốc độ chuột ngoài thang 1..20: %d", speed)
	}
	if err := winSetMouseSpeed(speed); err != nil {
		t.Fatalf("ghi lại tốc độ: %v", err)
	}
	if err := winSetMousePrecision(precision); err != nil {
		t.Fatalf("ghi lại độ chính xác: %v", err)
	}
	s2, p2, err := winGetMouse()
	if err != nil || s2 != speed || p2 != precision {
		t.Errorf("sau khi ghi lại: tốc độ %d (trước %d), độ chính xác %v (trước %v), lỗi %v", s2, speed, p2, precision, err)
	}
}
