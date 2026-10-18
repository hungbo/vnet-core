package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cùng mẫu với backend/internal/service/local_admin_test.go: chuỗi máy chủ
// băm cho "Quantri@2026" phải mở được máy trạm.
const localAdminVector = "pbkdf2-sha256$210000$AAECAwQFBgcICQoLDA0ODw$MKWV8XVcE7Dxum7d8y+ShLzMKHmnfcZUwtxeP2Byz8A"

func datChinhSach(t *testing.T, p clientPolicy) {
	t.Helper()
	setPolicy(p)
	t.Cleanup(func() { setPolicy(defaultClientPolicy()) })
}

// ghiConfig đặt config.json cạnh tệp test đang chạy — đúng chỗ loadFileConfig đọc.
func ghiConfig(t *testing.T, fc fileConfig) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(exe), "config.json")
	data, _ := json.Marshal(fc)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(path) })
}

func TestLocalAdmin_BamCuaMayChuMoDuocMayTram(t *testing.T) {
	if err := verifyPin(localAdminVector, "Quantri@2026"); err != nil {
		t.Fatalf("băm của máy chủ không khớp máy trạm: %v", err)
	}
}

func TestLocalAdmin_TuMayChu(t *testing.T) {
	datChinhSach(t, clientPolicy{LocalAdminUsername: "kythuat", LocalAdminHash: localAdminVector})

	if trung, dung := verifyLocalAdmin(" KyThuat ", "Quantri@2026"); !trung || !dung {
		t.Fatalf("đúng tài khoản mà không mở: trùng=%v đúng=%v", trung, dung)
	}
	if trung, dung := verifyLocalAdmin("kythuat", "sai-mat-khau"); !trung || dung {
		t.Fatalf("sai mật khẩu: trùng=%v đúng=%v", trung, dung)
	}
	if trung, dung := verifyLocalAdmin("khach1", "Quantri@2026"); trung || dung {
		t.Fatalf("tên khác: trùng=%v đúng=%v", trung, dung)
	}
}

func TestLocalAdmin_TuBoCaiKhiMayChuChuaDat(t *testing.T) {
	datChinhSach(t, defaultClientPolicy())
	ghiConfig(t, fileConfig{ServerURL: "http://x", LocalAdminUsername: "caidat", LocalAdminHash: localAdminVector})

	if _, dung := verifyLocalAdmin("caidat", "Quantri@2026"); !dung {
		t.Fatal("tài khoản đặt lúc cài không mở được máy")
	}

	// Máy chủ đặt tài khoản khác: tài khoản lúc cài hết hiệu lực.
	hash, err := hashPin("MatKhauMoi99")
	if err != nil {
		t.Fatal(err)
	}
	setPolicy(clientPolicy{LocalAdminUsername: "kythuat", LocalAdminHash: hash})
	if trung, _ := verifyLocalAdmin("caidat", "Quantri@2026"); trung {
		t.Fatal("máy chủ đã đổi tài khoản mà tài khoản lúc cài vẫn dùng được")
	}
	if _, dung := verifyLocalAdmin("kythuat", "MatKhauMoi99"); !dung {
		t.Fatal("tài khoản từ máy chủ không mở được máy")
	}
}

func TestLocalAdmin_ChuaDatThiKhongMo(t *testing.T) {
	datChinhSach(t, defaultClientPolicy())
	if trung, dung := verifyLocalAdmin("", ""); trung || dung {
		t.Fatal("chưa đặt tài khoản mà vẫn mở")
	}
}

func TestLocalAdmin_NuaTaiKhoanBiBo(t *testing.T) {
	p := normalizePolicy(clientPolicy{LocalAdminUsername: "kythuat"})
	if p.LocalAdminUsername != "" {
		t.Fatal("có tên mà không có băm vẫn được giữ")
	}
}

func TestLocalAdmin_KiemDauVaoLucCai(t *testing.T) {
	for _, c := range []struct{ user, pass string }{
		{"", "Quantri@2026"}, {"ky thuat", "Quantri@2026"}, {"kythuat", "12345"},
	} {
		if checkLocalAdminInput(c.user, c.pass) == nil {
			t.Errorf("chấp nhận %q/%q", c.user, c.pass)
		}
	}
	if err := checkLocalAdminInput("kythuat", "Quantri@2026"); err != nil {
		t.Error(err)
	}
}

// Nhật ký giao diện nằm trong thư mục của tài khoản khách: không được có mật khẩu.
func TestSafeBody_CheMatKhau(t *testing.T) {
	a := &App{}
	got := a.safeBody(LoginRequest{Username: "khach1", Password: "Bi-Mat-123", MachineCode: "MAY01"})
	if want := `{"machine_code":"MAY01","password":"***","username":"khach1"}`; got != want {
		t.Fatalf("safeBody = %s, mong %s", got, want)
	}
	got = a.safeBody(map[string]string{"old_password": "a", "new_password": "b", "card_code": "c"})
	if want := `{"card_code":"***","new_password":"***","old_password":"***"}`; got != want {
		t.Fatalf("safeBody = %s", got)
	}
}

func TestLocalAdmin_KhongLuuRaPolicyFile(t *testing.T) {
	path := policyFilePath()
	t.Cleanup(func() {
		os.Remove(path)
		setPolicy(defaultClientPolicy())
	})

	p := defaultClientPolicy()
	p.LocalAdminUsername, p.LocalAdminHash = "kythuat", localAdminVector
	savePolicyFile(p)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "kythuat") || strings.Contains(string(data), "pbkdf2") {
		t.Fatalf("policy.json chứa tài khoản quản trị: %s", data)
	}

	// Tệp do bản cũ ghi: nạp vào thì băm không vào bộ nhớ và bị xoá khỏi đĩa.
	old, _ := json.Marshal(p)
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	loadPolicyFile()
	if currentPolicy().LocalAdminHash != "" {
		t.Fatal("nạp policy.json cũ vẫn lấy băm mật khẩu")
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "pbkdf2") {
		t.Fatalf("policy.json cũ chưa được xoá băm: %s", data)
	}
}
