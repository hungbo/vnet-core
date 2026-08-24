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

// Tài khoản mặc định của bản cài: máy vừa dựng, chưa từng nối được máy chủ, vẫn
// phải có đường vào để gỡ hoặc sửa.
func TestBuiltinAdmin_MayMoiThiMoDuoc(t *testing.T) {
	dungThuMucTam(t)

	if !builtinAdminActive() {
		t.Fatal("máy chưa có tài khoản nào mà tài khoản mặc định đã tắt")
	}

	c, err := verifyCachedStaff(builtinAdminUser, builtinAdminPass, time.Now())
	if err != nil {
		t.Fatalf("tài khoản mặc định không mở được: %v", err)
	}
	if !c.Builtin {
		t.Error("không đánh dấu là tài khoản mặc định — nhật ký và cảnh báo dựa vào cờ này")
	}

	if _, err := verifyCachedStaff(builtinAdminUser, "sai", time.Now()); err == nil {
		t.Error("sai mật khẩu vẫn mở được")
	}
	if _, err := verifyCachedStaff("nguoi-khac", builtinAdminPass, time.Now()); err == nil {
		t.Error("tên khác vẫn mở được bằng mật khẩu mặc định")
	}
}

// Điều kiện quan trọng nhất: có một lần đăng nhập nhân viên thật là cửa mặc
// định ĐÓNG LẠI. Không đóng thì mọi máy dựng từ cùng bản build đều mở được
// bằng đúng một cặp ai cũng đoán ra, mãi mãi.
func TestBuiltinAdmin_TuTatSauLanDangNhapThat(t *testing.T) {
	dungThuMucTam(t)

	if err := rememberStaff("quanly", "matkhau-that-1234", "staff"); err != nil {
		t.Fatal(err)
	}

	if builtinAdminActive() {
		t.Fatal("đã có tài khoản thật mà tài khoản mặc định vẫn còn hiệu lực")
	}
	if _, err := verifyCachedStaff(builtinAdminUser, builtinAdminPass, time.Now()); err == nil {
		t.Fatal("tài khoản mặc định vẫn mở được sau khi đã có tài khoản thật")
	}
	// Và tài khoản thật thì vẫn dùng được.
	if _, err := verifyCachedStaff("quanly", "matkhau-that-1234", time.Now()); err != nil {
		t.Fatalf("tài khoản thật không dùng được: %v", err)
	}
}

// Đăng nhập bằng chính tên "admin" khi có mạng thì mật khẩu thật thay chỗ mật
// khẩu mặc định — đúng nghĩa "cập nhật lại sau".
func TestBuiltinAdmin_MatKhauThatThayChoMacDinh(t *testing.T) {
	dungThuMucTam(t)

	if err := rememberStaff("admin", "matkhau-that-5678", "staff"); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyCachedStaff("admin", builtinAdminPass, time.Now()); err == nil {
		t.Fatal("mật khẩu mặc định vẫn mở được sau khi đã đồng bộ mật khẩu thật")
	}
	if _, err := verifyCachedStaff("admin", "matkhau-that-5678", time.Now()); err != nil {
		t.Fatalf("mật khẩu thật không dùng được: %v", err)
	}
}
