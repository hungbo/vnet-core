//go:build windows

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/gin-gonic/gin"
	"golang.org/x/sys/windows/svc"
)

// Tên dịch vụ do bộ cài máy chủ (installer/vnet-server.nsi) đăng ký.
const serviceName = "VNETServer"

// lifetime trả về context kết thúc server và hàm main gọi sau khi dọn dẹp xong.
//
// Chạy dưới Service Control Manager thì SCM là thứ duy nhất ra lệnh dừng, và nó
// phải được báo STOPPED trước khi tiến trình thoát. Thoát trước khi báo thì SCM
// coi là sập và chạy lại dịch vụ ngay — nghĩa là chủ quán bấm Stop trong
// services.msc mà server cứ tự bật lại. Vì thế hàm trả về chờ svc.Run kết thúc.
func lifetime() (context.Context, func()) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		return ctx, stop
	}

	setupLogFile()
	loadServiceConfig()

	ctx, cancel := context.WithCancel(context.Background())
	cleanedUp := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		if err := svc.Run(serviceName, &serviceHandler{cancel: cancel, cleanedUp: cleanedUp}); err != nil {
			log.Printf("service: %v", err)
		}
		cancel()
		close(exited)
	}()
	return ctx, func() {
		close(cleanedUp)
		<-exited
	}
}

// loadServiceConfig nạp <thư mục cài>\data\config.env vào biến môi trường
// trước khi config.Load đọc chúng.
//
// Không để SCM truyền biến qua giá trị Environment trong registry: khoá
// HKLM\...\Services cho mọi tài khoản Users đọc, tức là lộ mật khẩu database
// và JWT_SECRET. Tệp này thì bộ cài đã khoá chỉ SYSTEM và Administrators đọc
// được. Sửa tệp xong chỉ cần khởi động lại dịch vụ.
func loadServiceConfig() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	root := filepath.Dir(exe)
	data := filepath.Join(root, "data")

	b, err := os.ReadFile(filepath.Join(data, "config.env"))
	if err != nil {
		log.Printf("config.env: %v", err)
	} else {
		// PowerShell 5.1 ghi UTF-8 kèm BOM; không bỏ thì khoá đầu tiên hỏng tên.
		text := strings.TrimPrefix(string(b), "\uFEFF")
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
			}
		}
	}

	// Đường dẫn luôn theo thư mục cài hiện tại, không lấy từ tệp.
	os.Setenv("UPLOAD_DIR", filepath.Join(data, "uploads"))
	os.Setenv("BACKUP_DIR", filepath.Join(data, "backups"))
	// Sao lưu/khôi phục gọi pg_dump/pg_restore theo PATH; bản đi kèm nằm ở pgsql\bin.
	os.Setenv("PATH", filepath.Join(root, "pgsql", "bin")+";"+os.Getenv("PATH"))
}

type serviceHandler struct {
	cancel    context.CancelFunc
	cleanedUp <-chan struct{}
}

func (h *serviceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for req := range r {
		switch req.Cmd {
		case svc.Interrogate:
			changes <- req.CurrentStatus
		case svc.Stop, svc.Shutdown:
			// main cho HTTP 15 giây để đóng rồi còn chờ các job nền.
			changes <- svc.Status{State: svc.StopPending, WaitHint: 30000}
			h.cancel()
			<-h.cleanedUp
			return false, 0
		}
	}
	return false, 0
}

// Dịch vụ không có console: không ghi ra tệp thì mọi dòng log, kể cả lý do
// server từ chối khởi động, đều biến mất. Ghi vào <thư mục cài>\logs\server.log.
//
// Mỗi yêu cầu HTTP là một dòng log, và mỗi máy trạm gửi heartbeat 15 giây một
// lần — quán 50 máy ra cỡ 30 MB mỗi ngày. Dịch vụ chạy hàng tháng không khởi
// động lại, nên phải xoay vòng trong lúc chạy chứ không chỉ lúc bật.
const logMaxBytes = 20 << 20

func setupLogFile() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	w := &rotatingFile{path: filepath.Join(dir, "server.log")}
	if err := w.open(); err != nil {
		return
	}
	log.SetOutput(w)
	gin.DefaultWriter = w
	gin.DefaultErrorWriter = w
}

type rotatingFile struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
}

func (w *rotatingFile) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, st.Size()
	return nil
}

// Write giữ đúng một bản cũ (server.log.1): đủ để xem chuyện vừa xảy ra mà
// không bao giờ lấp đầy ổ đĩa.
func (w *rotatingFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > logMaxBytes {
		w.f.Close()
		_ = os.Rename(w.path, w.path+".1")
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}
