package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"
)

// Ứng dụng tự chạy khi khách đăng nhập (roadmap C2): Discord, Garena, trình gõ tiếng Việt, công cụ riêng của quán...
// Danh sách do quán đặt ở trang Cài đặt > Máy trạm và đến qua chính sách. Chạy trong giao diện (phiên của khách) đúng lúc
// một hội viên VỪA đăng nhập, không phải khi nhân viên hay chế độ bảo trì vào máy: công cụ của quán không nên bật theo mỗi
// lần kỹ thuật viên sửa máy.
//
// Hẹp có chủ ý, cùng tinh thần với RunFirst của game: chỉ tệp .exe, không tham số, tương đối thì tính từ thư mục game,
// tuyệt đối thì chỉ trong Program Files thật của máy (resolveStartupApp). Tệp lỗi chỉ ghi nhật ký, không chặn việc đăng nhập.

const startupPollEvery = 2 * time.Second

// startupRunner là phần quyết định, tách khỏi Windows để kiểm thử.
type startupRunner struct {
	root         func() (string, error) // thư mục game trên máy trạm
	programFiles func() []string
	exists       func(string) bool
	running      func() map[string]bool // tên tiến trình đang chạy, chữ thường, không đuôi
	launch       func(exe, dir string) error

	wasMember bool
}

// tick nhận trạng thái đăng nhập hiện tại; trả về số tệp vừa chạy. Chỉ chạy ở lần CHUYỂN từ "chưa có hội viên" sang
// "có hội viên", nên một phiên dài không chạy lại mỗi vài giây.
func (r *startupRunner) tick(p clientPolicy, member bool) int {
	edge := member && !r.wasMember
	r.wasMember = member
	if !edge {
		return 0
	}
	apps := danhSachUngDungKhoiDong(p.StartupApps)
	if len(apps) == 0 {
		return 0
	}
	root, rootErr := r.root()
	var running map[string]bool
	started := 0
	for _, entry := range apps {
		relative := !(len(entry) >= 2 && entry[1] == ':')
		if relative && rootErr != nil {
			log.Printf("[khởi động] bỏ %q: chưa biết thư mục game (%v)", entry, rootErr)
			continue
		}
		exe, dir, err := resolveStartupApp(root, entry, r.programFiles())
		if err != nil {
			log.Printf("[khởi động] từ chối %q: %v", entry, err)
			reportError("warn", "startup", "ứng dụng tự chạy không hợp lệ", err.Error())
			continue
		}
		if !r.exists(exe) {
			log.Printf("[khởi động] không thấy tệp %s", exe)
			continue
		}
		if running == nil {
			running = r.running()
		}
		// Tách theo dấu gạch ngược bằng tay: filepath.Base chỉ hiểu dấu của hệ điều hành đang chạy (bài kiểm chạy cả trên macOS).
		name := strings.ToLower(strings.TrimSuffix(strings.ToLower(exe[strings.LastIndex(exe, `\`)+1:]), ".exe"))
		if running[name] {
			continue // đã chạy sẵn (khách vừa đăng nhập lại): không mở bản thứ hai
		}
		if err := r.launch(exe, dir); err != nil {
			log.Printf("[khởi động] chạy %s lỗi: %v", exe, err)
			reportError("warn", "startup", "không chạy được ứng dụng tự chạy", exe+": "+err.Error())
			continue
		}
		running[name] = true
		started++
	}
	return started
}

// runStartupApps là vòng nền của giao diện.
func runStartupApps(ctx context.Context, a *App) {
	r := &startupRunner{
		root: func() (string, error) {
			m, err := gameMenuCache.get(a.menuSource())
			if err != nil {
				return "", err
			}
			return m.Root, nil
		},
		programFiles: osProgramFiles,
		exists:       func(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() },
		running:      osRunningProcessNames,
		launch:       osLaunchDetached,
	}
	t := time.NewTicker(startupPollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st := a.linkStatus()
			if n := r.tick(currentPolicy(), st.Member && !st.Locked); n > 0 {
				log.Printf("[khởi động] đã chạy %d ứng dụng tự chạy", n)
			}
		}
	}
}
