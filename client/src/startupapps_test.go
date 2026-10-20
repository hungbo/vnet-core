package main

import (
	"errors"
	"testing"
)

func newTestStartup() (*startupRunner, *[]string) {
	var launched []string
	return &startupRunner{
		root:         func() (string, error) { return `D:\Games`, nil },
		programFiles: func() []string { return []string{`C:\Program Files`} },
		exists:       func(string) bool { return true },
		running:      func() map[string]bool { return map[string]bool{} },
		launch:       func(exe, dir string) error { launched = append(launched, exe); return nil },
	}, &launched
}

func TestStartupRunner_ChayMotLanMoiLanDangNhap(t *testing.T) {
	r, launched := newTestStartup()
	p := clientPolicy{StartupApps: "Tools\\a.exe\nTools\\b.exe"}

	if n := r.tick(p, false); n != 0 {
		t.Errorf("chưa có hội viên mà chạy %d tệp", n)
	}
	if n := r.tick(p, true); n != 2 {
		t.Fatalf("hội viên vừa đăng nhập phải chạy 2 tệp, chạy %d", n)
	}
	if n := r.tick(p, true); n != 0 {
		t.Errorf("phiên đang chạy không chạy lại, chạy %d", n)
	}
	r.tick(p, false) // đăng xuất
	if n := r.tick(p, true); n != 2 {
		t.Errorf("đăng nhập lần nữa phải chạy lại, chạy %d", n)
	}
	want := []string{`D:\Games\Tools\a.exe`, `D:\Games\Tools\b.exe`, `D:\Games\Tools\a.exe`, `D:\Games\Tools\b.exe`}
	if !sameStrings(*launched, want) {
		t.Errorf("đã chạy %v", *launched)
	}
}

func TestStartupRunner_TinhNangTat(t *testing.T) {
	r, launched := newTestStartup()
	r.tick(clientPolicy{}, true)
	if len(*launched) != 0 {
		t.Errorf("chính sách rỗng mà chạy %v", *launched)
	}
}

func TestStartupRunner_BoQuaTepLoiVaDangChay(t *testing.T) {
	r, launched := newTestStartup()
	r.exists = func(p string) bool { return p != `D:\Games\Tools\thieu.exe` }
	r.running = func() map[string]bool { return map[string]bool{"discord": true} }
	r.launch = func(exe, dir string) error {
		if exe == `D:\Games\Tools\hong.exe` {
			return errors.New("lỗi")
		}
		*launched = append(*launched, exe)
		return nil
	}
	p := clientPolicy{StartupApps: "Tools\\thieu.exe\nTools\\Discord.exe\nTools\\hong.exe\nTools\\tot.exe"}
	if n := r.tick(p, true); n != 1 {
		t.Errorf("chỉ tot.exe được chạy, chạy %d", n)
	}
	if !sameStrings(*launched, []string{`D:\Games\Tools\tot.exe`}) {
		t.Errorf("đã chạy %v", *launched)
	}
}

func TestStartupRunner_KhongBietThuMucGame(t *testing.T) {
	r, launched := newTestStartup()
	r.root = func() (string, error) { return "", errors.New("mất mạng") }
	p := clientPolicy{StartupApps: "Tools\\a.exe\nC:\\Program Files\\X\\x.exe"}
	r.tick(p, true)
	// Đường dẫn tuyệt đối trong Program Files không cần thư mục game nên vẫn chạy.
	if !sameStrings(*launched, []string{`C:\Program Files\X\x.exe`}) {
		t.Errorf("đã chạy %v", *launched)
	}
}
