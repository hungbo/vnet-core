package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// Máy trạm ghi nhật ký ra TỆP.
//
// Bản build dùng -H windowsgui nên không có cửa sổ console, và log.Printf rơi
// vào stderr không ai đọc. Ra quán thật thì mọi dòng "[guard] ...", "[agent]
// ..." mà hướng dẫn bảo "xem nhật ký" đều không tồn tại ở đâu cả.
//
//	dịch vụ nền: <thư mục cài>\logs\service.log   (ghi được vì chạy dưới SYSTEM)
//	giao diện:   %LOCALAPPDATA%\VNET\logs\ui.log    (tài khoản khách ghi được)
//
// Quá 5 MB thì đổi tên sang .1 lúc khởi động — giữ đúng một bản cũ.
const logMaxBytes = 5 << 20

func setupLogFile(service bool, windowMode string) {
	// Chạy cả khi không mở được tệp nhật ký bên dưới: gửi lỗi về server không
	// cần đến tệp nào.
	go runErrorReporter()

	var dir string
	if service {
		exe, err := os.Executable()
		if err != nil {
			return
		}
		dir = filepath.Join(filepath.Dir(exe), "logs")
	} else {
		base, err := os.UserCacheDir() // %LOCALAPPDATA% trên Windows
		if err != nil {
			return
		}
		dir = filepath.Join(base, "VNET", "logs")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	name := "ui.log"
	switch {
	case service:
		name = "service.log"
	case windowMode != "":
		name = "ui-" + windowMode + ".log"
	}
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err == nil && st.Size() > logMaxBytes {
		_ = os.Rename(path, path+".1")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	log.SetFlags(log.LstdFlags)

	// Panic không ai recover (heartbeat, WS, watchdog…) in stack ra stderr, mà
	// -H windowsgui không có stderr: tiến trình chết, dịch vụ bật lại, không còn
	// dấu vết vì sao. Ghi stack vào một tệp crash riêng: tệp đó còn nội dung lúc
	// khởi động nghĩa là lần trước đã sập, và nó được gửi về server.
	crashPath := path + ".crash"
	reportPreviousCrash(crashPath, strings.TrimSuffix(name, ".log"))
	if cf, err := os.Create(crashPath); err == nil {
		_ = debug.SetCrashOutput(cf, debug.CrashOptions{})
		cf.Close() // SetCrashOutput giữ bản sao file descriptor của nó
	}
}
