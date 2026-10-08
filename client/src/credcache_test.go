package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Mỗi bài kiểm dùng một thư mục riêng: bản đệm nằm trong thư mục cấu hình của
// người dùng, và ghi vào thư mục thật của người đang chạy test là làm bẩn máy họ.
func dungThuMucTam(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // Linux
	t.Setenv("HOME", t.TempDir())            // macOS
	t.Setenv("AppData", t.TempDir())         // Windows
}

func TestCredCache_LuuRoiMoDuocKhiMatMang(t *testing.T) {
	dungThuMucTam(t)

	if err := rememberStaff("quanly", "matkhau-dai-1234", "manager"); err != nil {
		t.Fatal(err)
	}

	c, err := verifyCachedStaff("quanly", "matkhau-dai-1234", time.Now())
	if err != nil {
		t.Fatalf("tài khoản đúng bị từ chối: %v", err)
	}
	if c.Role != "manager" {
		t.Errorf("Role = %q", c.Role)
	}

	if _, err := verifyCachedStaff("quanly", "sai-mat-khau", time.Now()); err == nil {
		t.Error("sai mật khẩu vẫn mở được")
	}
	if _, err := verifyCachedStaff("khong-co-ai", "matkhau-dai-1234", time.Now()); err == nil {
		t.Error("tài khoản không có trong bản đệm vẫn mở được")
	}
}

// Điều kiện quan trọng nhất: KHÔNG lưu hội viên. Lưu được nghĩa là khách chỉ
// cần rút dây mạng rồi tự mở máy — chơi không mất tiền, đúng thứ lớp khoá sinh
// ra để chặn.
func TestCredCache_KhongLuuHoiVien(t *testing.T) {
	dungThuMucTam(t)

	for _, vaiTro := range []string{"member", ""} {
		if err := rememberStaff("khach", "matkhau-dai-1234", vaiTro); err != nil {
			t.Fatal(err)
		}
	}
	if len(loadCredCache()) != 0 {
		t.Fatalf("đã lưu %d tài khoản — hội viên không được lưu", len(loadCredCache()))
	}
	if _, err := verifyCachedStaff("khach", "matkhau-dai-1234", time.Now()); err == nil {
		t.Fatal("hội viên mở được máy khi mất mạng")
	}
}

// Đuổi việc một nhân viên và đổi mật khẩu trên máy chủ mà bản đệm sống mãi thì
// họ vẫn mở được mọi máy trong quán.
func TestCredCache_QuaHanThiKhongMoDuoc(t *testing.T) {
	dungThuMucTam(t)

	if err := rememberStaff("nhanvien", "matkhau-dai-1234", "staff"); err != nil {
		t.Fatal(err)
	}

	vuaKip := time.Now().Add(credCacheTTL - time.Hour)
	if _, err := verifyCachedStaff("nhanvien", "matkhau-dai-1234", vuaKip); err != nil {
		t.Errorf("chưa quá hạn mà đã từ chối: %v", err)
	}

	quaHan := time.Now().Add(credCacheTTL + time.Hour)
	if _, err := verifyCachedStaff("nhanvien", "matkhau-dai-1234", quaHan); err == nil {
		t.Error("bản đệm quá hạn vẫn mở được")
	}
}

// Đổi mật khẩu trên máy chủ rồi đăng nhập một lần thì bản đệm phải theo kịp,
// và mật khẩu CŨ phải hết tác dụng.
func TestCredCache_DoiMatKhauThiBanDemTheoKip(t *testing.T) {
	dungThuMucTam(t)

	rememberStaff("quanly", "matkhau-cu-1234", "manager")
	rememberStaff("quanly", "matkhau-moi-5678", "manager")

	if _, err := verifyCachedStaff("quanly", "matkhau-moi-5678", time.Now()); err != nil {
		t.Errorf("mật khẩu mới không dùng được: %v", err)
	}
	if _, err := verifyCachedStaff("quanly", "matkhau-cu-1234", time.Now()); err == nil {
		t.Error("mật khẩu cũ vẫn mở được")
	}
	if n := len(loadCredCache()); n != 1 {
		t.Errorf("có %d bản ghi — cùng một tài khoản phải ghi đè, không cộng dồn", n)
	}
}

// Không giữ vô hạn: bản đệm là tệp trên máy khách, càng nhiều dòng càng nhiều
// thứ để dò.
func TestCredCache_ChiGiuMotSoTaiKhoanGanNhat(t *testing.T) {
	dungThuMucTam(t)

	for i := 0; i < credCacheMax+4; i++ {
		rememberStaff(string(rune('a'+i))+"-nhanvien", "matkhau-dai-1234", "staff")
	}
	if n := len(loadCredCache()); n > credCacheMax {
		t.Fatalf("giữ %d tài khoản, trần là %d", n, credCacheMax)
	}
}

// Tệp trên đĩa KHÔNG được chứa mật khẩu.
func TestCredCache_TepKhongChuaMatKhau(t *testing.T) {
	dungThuMucTam(t)

	rememberStaff("quanly", "matkhau-rat-de-nhan-ra", "manager")

	path, err := credCachePath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "matkhau-rat-de-nhan-ra") {
		t.Fatalf("tệp %s chứa mật khẩu dạng chữ thường", filepath.Base(path))
	}
}

// Máy chưa từng có ai đăng nhập thì không mở offline được — và phải nói rõ lý
// do, chứ không phải "sai mật khẩu" khiến nhân viên gõ lại mười lần.
func TestCredCache_ChuaCoAiDangNhap(t *testing.T) {
	dungThuMucTam(t)

	_, err := verifyCachedStaff("quanly", "matkhau-dai-1234", time.Now())
	if err == nil {
		t.Fatal("bản đệm rỗng vẫn mở được")
	}
	if err != errNoCachedCred {
		t.Errorf("lỗi = %v, mong thông báo riêng cho trường hợp chưa có ai đăng nhập", err)
	}
}

// Cặp admin/admin cứng trong bản build đã bỏ: máy chưa có ai đăng nhập thì
// đường vào là tài khoản quản trị máy trạm đặt lúc cài, không phải mật khẩu ai
// cũng đoán ra.
func TestCredCache_KhongConTaiKhoanMacDinh(t *testing.T) {
	dungThuMucTam(t)
	if _, err := verifyCachedStaff("admin", "admin", time.Now()); err == nil {
		t.Fatal("admin/admin vẫn mở được máy")
	}
}
