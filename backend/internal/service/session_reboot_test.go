package service

import (
	"testing"
	"time"
)

// Đóng nhầm phiên của khách đang chơi là lỗi tệ hơn bỏ sót một kẻ phá, nên hàm
// quyết định này phải chắc từng trường hợp.
func TestSessionStaleAfterReboot(t *testing.T) {
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		ten         string
		startedAt   time.Time
		bootedAt    time.Time
		muonDongLai bool
	}{
		{
			ten:         "bình thường: máy bật rồi khách mới đăng nhập",
			startedAt:   base,
			bootedAt:    base.Add(-3 * time.Minute), // boot trước login 3 phút
			muonDongLai: false,
		},
		{
			ten:         "reboot giữa phiên: bật lại sau khi đã chơi một giờ",
			startedAt:   base,
			bootedAt:    base.Add(1 * time.Hour),
			muonDongLai: true,
		},
		{
			ten:         "lệch đồng hồ vài giây: boot ngay sau login một chút",
			startedAt:   base,
			bootedAt:    base.Add(20 * time.Second), // trong ngưỡng 2 phút
			muonDongLai: false,
		},
		{
			ten:         "ngay tại ngưỡng: chưa đủ để coi là reboot",
			startedAt:   base,
			bootedAt:    base.Add(rebootStaleMargin),
			muonDongLai: false,
		},
		{
			ten:         "vừa qua ngưỡng một giây: là reboot",
			startedAt:   base,
			bootedAt:    base.Add(rebootStaleMargin + time.Second),
			muonDongLai: true,
		},
		{
			ten:         "boot rất lâu trước phiên: máy chạy liên tục nhiều ngày",
			startedAt:   base,
			bootedAt:    base.Add(-72 * time.Hour),
			muonDongLai: false,
		},
	}

	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got := sessionStaleAfterReboot(c.startedAt, c.bootedAt); got != c.muonDongLai {
				t.Errorf("sessionStaleAfterReboot = %v, mong %v", got, c.muonDongLai)
			}
		})
	}
}

// Ngưỡng phải đủ rộng để nuốt lệch đồng hồ mạng, và đủ hẹp để reboot thật (bật
// lại mất một hai phút) vẫn vượt qua được.
func TestRebootStaleMargin_HopLy(t *testing.T) {
	if rebootStaleMargin < time.Minute {
		t.Errorf("ngưỡng %v quá hẹp — lệch đồng hồ vài giây có thể đóng nhầm phiên", rebootStaleMargin)
	}
	if rebootStaleMargin > 5*time.Minute {
		t.Errorf("ngưỡng %v quá rộng — máy reboot xong khách đăng nhập lại vẫn bị nối phiên cũ", rebootStaleMargin)
	}
}
