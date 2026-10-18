package main

import (
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// Chặn ứng dụng trên máy trạm.
//
// Bản cũ thêm một luật tường lửa "program=%ProgramFiles%\<tên>.exe": đường dẫn
// không ứng dụng nào nằm ở đó, và kể cả đúng đường dẫn thì đó là chặn MẠNG chứ
// không phải cấm CHẠY như trang quản trị mô tả. Kiểm trên máy thật: bấm chặn,
// luật được tạo, ứng dụng vẫn chạy bình thường.
//
// Giờ dịch vụ nền (chạy dưới SYSTEM, thấy mọi phiên người dùng) giữ danh sách
// và tắt tiến trình trùng tên mỗi vài giây. Danh sách là cài đặt CHUNG cho mọi
// máy, đến trong chính sách kèm phản hồi nhịp tim (clientPolicy.BlockedApps).
// Chính sách được lưu ở policy.json nên danh sách sống qua khởi động lại và
// vẫn áp khi mất mạng.
type appBlocker struct {
	mu    sync.Mutex
	names map[string]bool
}

// appBlock chỉ có trong tiến trình dịch vụ nền. Giao diện không chặn gì: nó
// chạy dưới tài khoản khách, không tắt được tiến trình của phiên khác.
var appBlock *appBlocker

// Những tiến trình không bao giờ được tắt: tắt chúng là treo hoặc sập Windows,
// hoặc tự tắt chính máy trạm.
var khongDuocChan = map[string]bool{
	"vnet-client": true, "explorer": true, "csrss": true, "winlogon": true,
	"wininit": true, "lsass": true, "services": true, "smss": true,
	"svchost": true, "dwm": true, "system": true, "registry": true,
	"fontdrvhost": true, "logonui": true, "sihost": true, "ctfmon": true,
	"msedgewebview2": true,
}

func newAppBlocker() *appBlocker {
	return &appBlocker{names: map[string]bool{}}
}

// chuanHoaTenApp: "Chrome.EXE " → "chrome". So khớp không phân biệt hoa thường.
func chuanHoaTenApp(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	return strings.TrimSuffix(name, ".exe")
}

// setAll thay toàn bộ danh sách bằng csv ("chrome,steam") từ chính sách. Tên
// không hợp lệ, tiến trình hệ thống và chính máy trạm bị bỏ qua — máy chủ đã
// lọc, nhưng đây là chỗ cuối cùng trước khi tắt tiến trình thật.
func (b *appBlocker) setAll(csv string) {
	names := map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		name, err := safeProcessName(raw)
		if err != nil {
			log.Printf("[chặn ứng dụng] bỏ qua %q: %v", raw, err)
			continue
		}
		name = chuanHoaTenApp(name)
		if khongDuocChan[name] {
			log.Printf("[chặn ứng dụng] bỏ qua %q: tiến trình hệ thống hoặc chính máy trạm", name)
			continue
		}
		names[name] = true
	}
	b.mu.Lock()
	b.names = names
	b.mu.Unlock()
	b.enforce()
}

func (b *appBlocker) list() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.names))
	for n := range b.names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// enforce tắt mọi tiến trình đang chạy có tên trong danh sách.
func (b *appBlocker) enforce() {
	names := b.list()
	if len(names) == 0 {
		return
	}
	procs, err := process.Processes()
	if err != nil {
		return
	}
	self := int32(os.Getpid())
	for _, p := range procs {
		if p.Pid == self {
			continue
		}
		pname, err := p.Name()
		if err != nil {
			continue
		}
		n := chuanHoaTenApp(pname)
		for _, blocked := range names {
			if n == blocked {
				if err := p.Kill(); err == nil {
					log.Printf("[chặn ứng dụng] đã tắt %s (PID %d)", pname, p.Pid)
				}
				break
			}
		}
	}
}

func (b *appBlocker) run(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		b.enforce()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
