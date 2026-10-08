package main

import (
	"testing"
	"time"
)

func TestShouldShowCurtain(t *testing.T) {
	cases := []struct {
		ten          string
		uptime       time.Duration
		baoTri       bool
		dockDangChay bool
		want         bool
	}{
		{"máy vừa bật", 20 * time.Second, false, false, true},
		{"sát ngưỡng 5 phút", 5*time.Minute - time.Second, false, false, true},
		{"đúng 5 phút: hết hạn", 5 * time.Minute, false, false, false},
		{"giao diện bị bật lại giữa buổi", 3 * time.Hour, false, false, false},
		{"có maintenance.flag: nhân viên đang sửa máy", 20 * time.Second, true, false, false},
		{"thanh điều khiển đã chạy: bản thứ hai không được chớp màn che", 20 * time.Second, false, true, false},
		{"cả hai cờ", 20 * time.Second, true, true, false},
		{"đồng hồ máy báo số âm", -time.Second, false, false, false},
	}
	for _, c := range cases {
		if got := shouldShowCurtain(c.uptime, c.baoTri, c.dockDangChay); got != c.want {
			t.Errorf("%s: shouldShowCurtain(%v, %v, %v) = %v, mong %v",
				c.ten, c.uptime, c.baoTri, c.dockDangChay, got, c.want)
		}
	}
}

func TestCurtainLayout(t *testing.T) {
	const m = curtainMargin
	cases := []struct {
		ten               string
		vx, vy, vw, vh    int
		pw, ph            int
		wantWin, wantText curtainRect
	}{
		{
			"một màn hình",
			0, 0, 1920, 1080, 1920, 1080,
			curtainRect{-m, -m, 1920 + m, 1080 + m},
			curtainRect{m, m, m + 1920, m + 1080},
		},
		{
			// 1920x1080 ở 150% khi tiến trình chưa khai DPI: Windows báo 1280x720.
			"tỉ lệ 150%: số đo đã co",
			0, 0, 1280, 720, 1280, 720,
			curtainRect{-m, -m, 1280 + m, 720 + m},
			curtainRect{m, m, m + 1280, m + 720},
		},
		{
			// Màn phụ 1280x1024 nằm bên TRÁI màn chính 1920x1080: gốc ảo âm.
			"màn phụ bên trái: gốc âm, chữ vẫn ở màn chính",
			-1280, 0, 3200, 1080, 1920, 1080,
			curtainRect{-1280 - m, -m, 1920 + m, 1080 + m},
			curtainRect{1280 + m, m, 1280 + m + 1920, m + 1080},
		},
		{
			"màn phụ phía trên",
			0, -1080, 1920, 2160, 1920, 1080,
			curtainRect{-m, -1080 - m, 1920 + m, 1080 + m},
			curtainRect{m, 1080 + m, m + 1920, 1080 + m + 1080},
		},
		{
			"Windows không báo màn hình ảo: lùi về màn hình chính",
			0, 0, 0, 0, 1366, 768,
			curtainRect{-m, -m, 1366 + m, 768 + m},
			curtainRect{m, m, m + 1366, m + 768},
		},
	}
	for _, c := range cases {
		win, text := curtainLayout(c.vx, c.vy, c.vw, c.vh, c.pw, c.ph)
		if win != c.wantWin {
			t.Errorf("%s: cửa sổ %+v, mong %+v", c.ten, win, c.wantWin)
		}
		if text != c.wantText {
			t.Errorf("%s: chữ %+v, mong %+v", c.ten, text, c.wantText)
		}
		// Bất biến: màn che phải phủ kín màn hình ảo, chừa lề ở mọi mép.
		if c.vw > 0 && (win.left > c.vx-m || win.top > c.vy-m || win.right < c.vx+c.vw+m || win.bottom < c.vy+c.vh+m) {
			t.Errorf("%s: cửa sổ %+v không phủ hết màn hình ảo", c.ten, win)
		}
	}
}
