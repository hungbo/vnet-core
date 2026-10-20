package main

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Alt+K đóng ứng dụng (roadmap C3): khách (hay nhân viên) bấm Alt+K để đóng mọi ứng dụng đang hiện cửa sổ, ví dụ khi game
// treo hoặc khách rời máy mà quên thoát. Quán bật ở Cài đặt > Máy trạm và liệt kê tiến trình KHÔNG bị đóng (Discord, công cụ
// riêng...). Mặc định TẮT: phím nóng toàn cục lấy mất Alt+K của mọi game, nên chỉ đăng ký khi quán bật.
//
// Chỉ đóng tiến trình có cửa sổ hiện trên màn hình của khách; dịch vụ nền, tiến trình hệ thống và khung desktop thì không
// (protectedApps), và chính máy trạm VNET cũng không. Không làm gì khi màn hình đang khoá.

const (
	closePollEvery = 5 * time.Second
	closeCooldown  = 3 * time.Second
)

// closeAppsState là phần quyết định, tách khỏi Windows để kiểm thử.
type closeAppsState struct {
	mu       sync.Mutex
	last     time.Time
	selfPID  uint32
	selfName string

	list      func() []openApp
	terminate func(openApp) error
}

// press xử lý một lần bấm phím; trả về số tiến trình đã đóng. Bấm dồn trong closeCooldown chỉ tính một lần.
func (s *closeAppsState) press(p clientPolicy, locked bool, now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !p.CloseHotkey || locked || now.Sub(s.last) < closeCooldown {
		return 0
	}
	s.last = now
	victims := chonDeDong(s.list(), danhSachLoaiTruDong(p.CloseExclude), s.selfPID, s.selfName)
	closed := 0
	for _, v := range victims {
		if err := s.terminate(v); err != nil {
			log.Printf("[alt+k] không đóng được %s (%d): %v", v.Name, v.PID, err)
			continue
		}
		closed++
	}
	log.Printf("[alt+k] đã đóng %d trong %d ứng dụng", closed, len(victims))
	return closed
}

// runCloseApps là vòng nền của giao diện: đăng ký phím nóng khi quán bật, gỡ khi quán tắt.
func runCloseApps(ctx context.Context, a *App) {
	exe, _ := os.Executable()
	selfName := exe[strings.LastIndexAny(exe, `\/`)+1:]
	st := &closeAppsState{
		selfPID: uint32(os.Getpid()), selfName: selfName,
		list: osListOpenApps, terminate: osTerminateApp,
	}
	var stop func()
	defer func() {
		if stop != nil {
			stop()
		}
	}()
	t := time.NewTicker(closePollEvery)
	defer t.Stop()
	for {
		want := currentPolicy().CloseHotkey
		switch {
		case want && stop == nil:
			stop = osRegisterCloseHotkey(func() {
				st.press(currentPolicy(), a.linkStatus().Locked, time.Now())
			})
		case !want && stop != nil:
			stop()
			stop = nil
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
