package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Dấu "đã có người dùng" phải sống qua một lần dịch vụ khởi động lại trong CÙNG lần
// bật máy, và không được dính sang lần bật máy sau (máy có ổ cứng giữ được tệp).
func TestMayDaDung(t *testing.T) {
	tep := filepath.Join(t.TempDir(), "used.json")
	batMay := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	// Mốc "bây giờ" và thời gian máy đã chạy, sau khi bật máy một khoảng up.
	luc := func(up time.Duration) time.Time { return batMay.Add(up) }

	if mayDaDung(tep, luc(40*time.Second), 40*time.Second, false) {
		t.Fatal("chưa có dấu, dịch vụ bật sớm: máy chưa ai dùng")
	}
	ghiDaDung(tep, luc(3*time.Minute), 3*time.Minute)
	if !mayDaDung(tep, luc(4*time.Minute), 4*time.Minute, false) {
		t.Fatal("dịch vụ khởi động lại trong cùng lần bật máy phải đọc được dấu")
	}

	// Lần bật máy sau, hai tiếng nữa: tệp còn đó (máy có ổ cứng) nhưng không phải của lần này.
	sau := batMay.Add(2 * time.Hour)
	if mayDaDung(tep, sau.Add(40*time.Second), 40*time.Second, false) {
		t.Fatal("dấu của lần bật máy trước không được tính")
	}

	// Không dấu mà dịch vụ bật muộn: không biết trước đó ra sao, coi như đã dùng.
	if !mayDaDung(filepath.Join(t.TempDir(), "khong-co.json"), batMay.Add(6*time.Minute), 6*time.Minute, false) {
		t.Fatal("dịch vụ bật quá curtainMaxUptime mà không có dấu thì phải coi là đã dùng")
	}

	// Tệp hỏng thì như không có dấu.
	hong := filepath.Join(t.TempDir(), "hong.json")
	_ = os.WriteFile(hong, []byte("{"), 0o644)
	if mayDaDung(hong, batMay.Add(time.Minute), time.Minute, false) {
		t.Fatal("tệp hỏng, dịch vụ bật sớm: máy chưa ai dùng")
	}
}

// Đồng hồ hệ thống bị chỉnh vài giây giữa lúc ghi và lúc đọc vẫn là cùng một lần bật
// máy; đúng mốc curtainMaxUptime là đã muộn; bản cũ vừa tự cập nhật lên thì không tin
// là máy sạch dù không có dấu.
func TestMayDaDung_Bien(t *testing.T) {
	tep := filepath.Join(t.TempDir(), "used.json")
	batMay := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	ghiDaDung(tep, batMay.Add(time.Minute), time.Minute)
	// Giờ hệ thống bị kéo lùi 20 giây (đồng bộ giờ), uptime vẫn chạy đều.
	if !mayDaDung(tep, batMay.Add(2*time.Minute-20*time.Second), 2*time.Minute, false) {
		t.Fatal("lệch 20 giây vẫn là cùng một lần bật máy")
	}
	khong := filepath.Join(t.TempDir(), "khong-co.json")
	if !mayDaDung(khong, batMay.Add(curtainMaxUptime), curtainMaxUptime, false) {
		t.Fatal("dịch vụ bật đúng curtainMaxUptime sau khi bật máy: coi như đã dùng")
	}
	if mayDaDung(khong, batMay.Add(curtainMaxUptime-time.Second), curtainMaxUptime-time.Second, false) {
		t.Fatal("dưới curtainMaxUptime, không dấu, không cập nhật: máy chưa ai dùng")
	}
	if !mayDaDung(khong, batMay.Add(100*time.Second), 100*time.Second, true) {
		t.Fatal("vừa tự cập nhật từ bản cũ giữa lần bật máy: coi như đã dùng")
	}
}
