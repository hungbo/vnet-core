package main

import (
	"strings"
	"testing"
)

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDanhSachHinhNenMay(t *testing.T) {
	got := danhSachHinhNen("/uploads/a.jpg\r\n/uploads/a.jpg\n/uploads/../x.jpg\n/uploads/b.PNG\nhttp://x/y.jpg\n/uploads/c.gif")
	want := []string{"/uploads/a.jpg", "/uploads/b.PNG"}
	if !sameStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(danhSachHinhNen(strings.Repeat("/uploads/a.jpg\n", 3))) != 1 {
		t.Error("trùng phải bị bỏ")
	}
	var many []string
	for i := 0; i < 30; i++ {
		many = append(many, "/uploads/"+string(rune('a'+i%26))+string(rune('a'+i/26))+".jpg")
	}
	if n := len(danhSachHinhNen(strings.Join(many, "\n"))); n != maxWallpapers {
		t.Errorf("tối đa %d ảnh, được %d", maxWallpapers, n)
	}
}

func TestKhoangTrinhChieuMay(t *testing.T) {
	for in, want := range map[int64]int64{-1: 0, 0: 0, 1: 10, 300: 300, 99999999: 86400} {
		if got := khoangTrinhChieuMay(in); got != want {
			t.Errorf("khoangTrinhChieuMay(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestUngDungKhoiDongHopLeMay(t *testing.T) {
	good := []string{`Tools\a.exe`, `Tools/b/c.EXE`, `C:\Program Files\X\x.exe`, `c:\program files (x86)\Y\y.exe`}
	for _, in := range good {
		if _, err := ungDungKhoiDongHopLeMay(in); err != nil {
			t.Errorf("%q phải hợp lệ: %v", in, err)
		}
	}
	bad := []string{"", `a.bat`, `a.exe /s`, `..\a.exe`, `a\..\b.exe`, `C:\Windows\cmd.exe`, `\\srv\x.exe`, `\a.exe`, `a&b.exe`, `%TEMP%\a.exe`, strings.Repeat("a", 300) + ".exe"}
	for _, in := range bad {
		if _, err := ungDungKhoiDongHopLeMay(in); err == nil {
			t.Errorf("%q phải bị từ chối", in)
		}
	}
	got := danhSachUngDungKhoiDong("Tools\\a.exe\nbad.bat\ntools\\A.EXE\nTools\\b.exe")
	if !sameStrings(got, []string{`Tools\a.exe`, `Tools\b.exe`}) {
		t.Errorf("got %v", got)
	}
}

func TestResolveStartupApp(t *testing.T) {
	pf := []string{`C:\Program Files`, `C:\Program Files (x86)`}
	exe, dir, err := resolveStartupApp(`D:\Games`, `Tools\Garena\garena.exe`, pf)
	if err != nil || exe != `D:\Games\Tools\Garena\garena.exe` || dir != `D:\Games\Tools\Garena` {
		t.Errorf("tương đối: %q %q %v", exe, dir, err)
	}
	exe, _, err = resolveStartupApp(`D:\Games`, `C:\Program Files\X\x.exe`, pf)
	if err != nil || exe != `C:\Program Files\X\x.exe` {
		t.Errorf("tuyệt đối: %q %v", exe, err)
	}
	// "C:\Program Files" chỉ đúng khi nó thật là thư mục Program Files của máy: Windows cài ở ổ khác thì không.
	if _, _, err := resolveStartupApp(`D:\Games`, `C:\Program Files\X\x.exe`, []string{`E:\Program Files`}); err == nil {
		t.Error("Program Files giả (khác ổ với Program Files thật) phải bị từ chối")
	}
	for _, bad := range []string{`..\evil.exe`, `Tools\..\..\evil.exe`, `C:\Windows\System32\cmd.exe`, `Tools \x.exe`} {
		if _, _, err := resolveStartupApp(`D:\Games`, bad, pf); err == nil {
			t.Errorf("%q phải bị từ chối", bad)
		}
	}
	if _, _, err := resolveStartupApp(`không-phải-ổ-đĩa`, `Tools\a.exe`, pf); err == nil {
		t.Error("gốc game sai phải bị từ chối")
	}
}

func TestChonDeDong(t *testing.T) {
	apps := []openApp{
		{1, "explorer"}, {2, "chrome"}, {3, "VALORANT-Win64-Shipping.exe"}, {4, "discord"}, {5, "vnet-client"},
		{6, "chrome"}, {2, "chrome"}, {7, "Steam.exe"}, {8, "ctfmon"}, {0, "x"}, {9, ""}, {100, "tenvnet"},
	}
	got := chonDeDong(apps, danhSachLoaiTruDong("Discord.exe, steam"), 100, "tenvnet.exe")
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	want := []string{"chrome", "VALORANT-Win64-Shipping.exe", "chrome"}
	if !sameStrings(names, want) {
		t.Errorf("đóng %v, mong %v", names, want)
	}
	for _, a := range got {
		if a.PID == 100 || a.PID == 5 || a.PID == 1 {
			t.Errorf("không được đóng tiến trình %v", a)
		}
	}
}

func TestDanhSachLoaiTruDong(t *testing.T) {
	got := danhSachLoaiTruDong("Discord.exe, STEAM\nbad name!,;chrome")
	for _, n := range []string{"discord", "steam", "chrome"} {
		if !got[n] {
			t.Errorf("thiếu %q trong %v", n, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("tên sai phải bị bỏ: %v", got)
	}
}

func TestUngDungKhoiDongHaiDauGachNguocMay(t *testing.T) {
	if _, err := ungDungKhoiDongHopLeMay(`Tools\\Garena\\garena.exe`); err == nil {
		t.Error("hai dấu gạch ngược liền nhau phải bị từ chối")
	}
}
