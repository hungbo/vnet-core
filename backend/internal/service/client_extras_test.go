package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHinhNenHopLe(t *testing.T) {
	ok := []string{"/uploads/2026/10/20/1.jpg", "/uploads/a/b.PNG", " /uploads/x.jpeg "}
	for _, in := range ok {
		_, good := hinhNenHopLe(in)
		assert.True(t, good, in)
	}
	bad := []string{
		"", "uploads/x.jpg", "/other/x.jpg", "/uploads/../etc/passwd.jpg", "/uploads/x.gif", "/uploads/x.webp",
		"/uploads/x.jpg?cb=1", "/uploads/a b.jpg", `/uploads/a\b.jpg`, "http://evil/uploads/x.jpg", "/uploads//x.jpg",
	}
	for _, in := range bad {
		_, good := hinhNenHopLe(in)
		assert.False(t, good, in)
	}
	got := chuanHoaHinhNen("/uploads/a.jpg\r\n/uploads/a.jpg\n/uploads/../x.jpg\n/uploads/b.png\n\n")
	assert.Equal(t, []string{"/uploads/a.jpg", "/uploads/b.png"}, got, "bỏ trùng và dòng sai")
}

func TestUngDungKhoiDongHopLe(t *testing.T) {
	good := map[string]string{
		`Tools\Garena\garena.exe`:            `Tools\Garena\garena.exe`,
		`Tools/Discord/Discord.EXE`:          `Tools\Discord\Discord.EXE`,
		`  C:\Program Files\Foo\foo.exe `:    `C:\Program Files\Foo\foo.exe`,
		`c:\program files (x86)\Bar\bar.exe`: `c:\program files (x86)\Bar\bar.exe`,
	}
	for in, want := range good {
		got, err := ungDungKhoiDongHopLe(in)
		assert.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	bad := map[string]string{
		"":                                "trống",
		`tools\run.bat`:                   "không phải .exe",
		`tools\a.exe /s`:                  "tham số",
		`..\evil.exe`:                     "lên thư mục trên",
		`tools\..\evil.exe`:               "lên thư mục trên",
		`C:\Windows\System32\cmd.exe`:     "tuyệt đối ngoài Program Files",
		`\\server\share\x.exe`:            "đường dẫn mạng",
		`\tools\x.exe`:                    "bắt đầu bằng gạch ngược",
		`tools\a&calc.exe`:                "ký tự lệnh",
		`tools\%TEMP%\x.exe`:              "biến môi trường",
		`tools\"x".exe`:                   "dấu nháy",
		strings.Repeat("a", 300) + ".exe": "quá dài",
	}
	for in, why := range bad {
		_, err := ungDungKhoiDongHopLe(in)
		assert.Error(t, err, why)
	}
}

func TestKiemTraCaiDatMayTramThem(t *testing.T) {
	assert.NoError(t, kiemTraCaiDatMayTramThem(map[string]interface{}{
		StartupAppsKey: "Tools\\a.exe\n\nTools\\b.exe", WallpaperURLsKey: "/uploads/a.jpg",
	}))
	err := kiemTraCaiDatMayTramThem(map[string]interface{}{StartupAppsKey: "ok.exe\nbad.bat"})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "dòng 2", "nói đúng dòng sai")
	}
	assert.Error(t, kiemTraCaiDatMayTramThem(map[string]interface{}{WallpaperURLsKey: "http://x/y.jpg"}))
	assert.Error(t, kiemTraCaiDatMayTramThem(map[string]interface{}{StartupAppsKey: 5}))
	many := strings.Repeat("a.exe\n", maxStartupApps+1)
	assert.Error(t, kiemTraCaiDatMayTramThem(map[string]interface{}{StartupAppsKey: many}))
}

func TestKhoangTrinhChieu(t *testing.T) {
	assert.Zero(t, khoangTrinhChieu(0))
	assert.Zero(t, khoangTrinhChieu(-5))
	assert.Equal(t, int64(minWallpaperInterval), khoangTrinhChieu(1), "đổi ảnh nhanh quá thì kẹp về mức tối thiểu")
	assert.Equal(t, int64(maxWallpaperInterval), khoangTrinhChieu(10_000_000))
	assert.Equal(t, int64(300), khoangTrinhChieu(300))
}

func TestClientPolicyFrom_CaiDatThem(t *testing.T) {
	p := clientPolicyFrom(map[string]string{
		WallpaperURLsKey: "/uploads/a.jpg\n/uploads/bad.gif", WallpaperIntervalKey: "5",
		StartupAppsKey: "Tools\\a.exe\nevil.bat", CloseHotkeyKey: "true", CloseExcludeKey: "Discord.exe, steam",
	})
	assert.Equal(t, "/uploads/a.jpg", p.Wallpapers)
	assert.Equal(t, int64(minWallpaperInterval), p.WallpaperIntervalSeconds)
	assert.Equal(t, `Tools\a.exe`, p.StartupApps)
	assert.True(t, p.CloseHotkey)
	assert.Equal(t, "discord,steam", p.CloseExclude)

	empty := clientPolicyFrom(map[string]string{})
	assert.Empty(t, empty.Wallpapers)
	assert.False(t, empty.CloseHotkey, "mặc định TẮT: không tự đóng ứng dụng của khách khi chưa ai bật")
}

func TestLogoMenu(t *testing.T) {
	_, ok := logoMenuHopLe("/uploads/2026/10/20/1.webp")
	assert.True(t, ok)
	for _, bad := range []string{"", "/x/a.png", "/uploads/a.svg", "/uploads/../a.png", "http://x/a.png"} {
		_, ok := logoMenuHopLe(bad)
		assert.False(t, ok, bad)
	}
	assert.NoError(t, kiemTraLogoMenu(map[string]interface{}{MenuLogoKey: ""}))
	assert.Error(t, kiemTraLogoMenu(map[string]interface{}{MenuLogoKey: "/uploads/a.svg"}))
}

func TestUngDungKhoiDongHaiDauGachNguoc(t *testing.T) {
	_, err := ungDungKhoiDongHopLe(`Tools\\Garena\\garena.exe`)
	assert.Error(t, err, "chuỗi đã escape dán nguyên: máy trạm sẽ từ chối, nên máy chủ từ chối ngay lúc lưu")
}
