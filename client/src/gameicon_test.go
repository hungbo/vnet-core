package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Phần lấy icon bằng Win32 chỉ kiểm được trên Windows. Ở đây là phần thuần chạy
// được ở mọi nơi: đổi điểm ảnh GDI sang PNG, kiểm tệp nguồn và bộ nhớ đệm.

func TestIconToNRGBA_CoAlpha(t *testing.T) {
	// GDI trả BGRA. Điểm 0 đặc; điểm 1 bán trong suốt có màu lớn hơn alpha, nên
	// không thể là ảnh đã nhân alpha và phải giữ nguyên.
	p := &iconPixels{w: 2, h: 1, color: []byte{
		10, 20, 30, 255,
		200, 100, 50, 128,
	}}
	img, err := iconToNRGBA(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{30, 20, 10, 255, 50, 100, 200, 128}
	if got := img.Pix[:8]; !bytes.Equal(got, want) { // 2x1 nằm ở hàng trên của khung 2x2
		t.Errorf("Pix = %v, mong %v", got, want)
	}
}

func TestIconToNRGBA_DaNhanSanAlpha(t *testing.T) {
	// Mọi kênh màu <= alpha: ảnh đã nhân alpha, phải chia ngược lại.
	p := &iconPixels{w: 2, h: 1, color: []byte{
		32, 16, 8, 128,
		255, 0, 0, 255,
	}}
	img, err := iconToNRGBA(p)
	if err != nil {
		t.Fatal(err)
	}
	// c*255/128 làm tròn: 8->16, 16->32, 32->64.
	want := []byte{16, 32, 64, 128, 0, 0, 255, 255}
	if got := img.Pix[:8]; !bytes.Equal(got, want) {
		t.Errorf("Pix = %v, mong %v", got, want)
	}
}

func TestLooksPremultiplied(t *testing.T) {
	cases := []struct {
		ten  string
		bgra []byte
		want bool
	}{
		{"toàn đặc", []byte{9, 9, 9, 255}, false},
		{"bán trong suốt, màu <= alpha", []byte{10, 20, 30, 100}, true},
		{"một kênh vượt alpha", []byte{10, 20, 101, 100}, false},
		{"một điểm vi phạm trong nhiều điểm", []byte{1, 1, 1, 50, 200, 0, 0, 100}, false},
		{"alpha 0 mà còn màu", []byte{0, 0, 7, 0, 5, 5, 5, 90}, false},
		{"chỉ có điểm đặc và điểm trống", []byte{0, 0, 0, 0, 9, 9, 9, 255}, false},
	}
	for _, c := range cases {
		if got := looksPremultiplied(c.bgra); got != c.want {
			t.Errorf("%s: got %v, mong %v", c.ten, got, c.want)
		}
	}
}

func TestIconToNRGBA_IconCuDungMatNa(t *testing.T) {
	// Alpha = 0 ở mọi điểm là icon không có kênh alpha, KHÔNG phải icon trống.
	// Mặt nạ: đen = hiện, trắng = trong suốt.
	p := &iconPixels{
		w: 2, h: 1,
		color: []byte{10, 20, 30, 0, 40, 50, 60, 0},
		mask:  []byte{0, 0, 0, 0, 255, 255, 255, 0},
	}
	img, err := iconToNRGBA(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{30, 20, 10, 255, 0, 0, 0, 0}
	if got := img.Pix[:8]; !bytes.Equal(got, want) {
		t.Errorf("Pix = %v, mong %v", got, want)
	}
}

func TestIconToNRGBA_KhongAlphaKhongMatNaLaDac(t *testing.T) {
	for _, mask := range [][]byte{nil, {0, 0}} { // nil, và mặt nạ sai độ dài thì bỏ
		p := &iconPixels{w: 1, h: 1, color: []byte{1, 2, 3, 0}, mask: mask}
		img, err := iconToNRGBA(p)
		if err != nil {
			t.Fatal(err)
		}
		if want := []byte{3, 2, 1, 255}; !bytes.Equal(img.Pix, want) {
			t.Errorf("mask=%v: Pix = %v, mong %v", mask, img.Pix, want)
		}
	}
}

func TestIconToNRGBA_AlphaThatBoQuaMatNa(t *testing.T) {
	// Đã có alpha thật thì mặt nạ (kể cả trắng toàn bộ) không được xoá icon.
	p := &iconPixels{
		w: 1, h: 1,
		color: []byte{10, 20, 30, 255},
		mask:  []byte{255, 255, 255, 0},
	}
	img, err := iconToNRGBA(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{30, 20, 10, 255}; !bytes.Equal(img.Pix, want) {
		t.Errorf("Pix = %v, mong %v", img.Pix, want)
	}
}

func TestIconToNRGBA_KhongVuongDatGiua(t *testing.T) {
	p := &iconPixels{w: 1, h: 3, color: []byte{
		1, 1, 1, 255,
		2, 2, 2, 255,
		3, 3, 3, 255,
	}}
	img, err := iconToNRGBA(p)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 3 || b.Dy() != 3 {
		t.Fatalf("kích thước %v, mong 3x3", b)
	}
	// Cột giữa (x=1) mang icon; hai cột bên trong suốt.
	for y, v := range []uint8{1, 2, 3} {
		if got := img.NRGBAAt(1, y); got.R != v || got.A != 255 {
			t.Errorf("(1,%d) = %v, mong R=%d đặc", y, got, v)
		}
		if got := img.NRGBAAt(0, y); got.A != 0 {
			t.Errorf("(0,%d) = %v, mong trong suốt", y, got)
		}
	}
}

func TestIconToNRGBA_BiTuChoi(t *testing.T) {
	cases := []struct {
		ten string
		p   *iconPixels
	}{
		{"rộng 0", &iconPixels{w: 0, h: 1}},
		{"quá lớn", &iconPixels{w: maxIconSide + 1, h: 1}},
		{"sai độ dài", &iconPixels{w: 2, h: 2, color: make([]byte, 4)}},
	}
	for _, c := range cases {
		if _, err := iconToNRGBA(c.p); err == nil {
			t.Errorf("%s: mong lỗi", c.ten)
		}
	}

	// Trong suốt hoàn toàn (mặt nạ trắng hết) không có gì để vẽ: coi là không icon.
	_, err := iconToNRGBA(&iconPixels{w: 1, h: 1, color: []byte{1, 2, 3, 0}, mask: []byte{255, 255, 255, 0}})
	if !errors.Is(err, errNoGameIcon) {
		t.Errorf("err = %v, mong errNoGameIcon", err)
	}
}

func TestIconPixelsToDataURI_GiaiMaDuoc(t *testing.T) {
	p := &iconPixels{w: 2, h: 2, color: []byte{
		10, 20, 30, 255, 0, 0, 255, 255,
		0, 255, 0, 255, 255, 0, 0, 255,
	}}
	uri, err := iconPixelsToDataURI(p)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("thiếu tiền tố data URI: %.40s", uri)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 2 || b.Dy() != 2 {
		t.Fatalf("kích thước %v", b)
	}
	r, g, b, a := img.At(0, 0).RGBA()
	if r>>8 != 30 || g>>8 != 20 || b>>8 != 10 || a>>8 != 255 {
		t.Errorf("điểm (0,0) = %d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
}

// ---- Bộ nhớ đệm -----------------------------------------------------------

func newTestIconCache(now *time.Time) *iconCache {
	return &iconCache{entries: map[string]iconCacheEntry{}, now: func() time.Time { return *now }}
}

func TestIconCache_NhoCaKetQuaRong(t *testing.T) {
	now := time.Unix(1000, 0)
	c := newTestIconCache(&now)
	src := iconSource{key: "a"}
	calls := 0
	extract := func() string { calls++; return "" }

	for i := 0; i < 3; i++ {
		if got := c.get(src, extract); got != "" {
			t.Fatalf("got %q", got)
		}
	}
	if calls != 1 {
		t.Fatalf("trích %d lần, mong 1 (kết quả rỗng cũng phải được nhớ)", calls)
	}

	// Hết hạn thì thử lại: game vừa cài xong sẽ có tệp chạy.
	now = now.Add(gameIconMissTTL)
	c.get(src, extract)
	if calls != 2 {
		t.Fatalf("sau khi hết hạn trích %d lần, mong 2", calls)
	}
}

func TestIconCache_IconCoKhongHetHan(t *testing.T) {
	now := time.Unix(1000, 0)
	c := newTestIconCache(&now)
	src := iconSource{key: "a"}
	calls := 0
	extract := func() string { calls++; return "data:x" }

	c.get(src, extract)
	now = now.Add(24 * time.Hour)
	if got := c.get(src, extract); got != "data:x" || calls != 1 {
		t.Fatalf("got %q sau %d lần trích, mong data:x sau 1 lần", got, calls)
	}
}

func TestIconCache_ChanPhinhVoHan(t *testing.T) {
	now := time.Unix(1000, 0)
	c := newTestIconCache(&now)
	for i := 0; i < gameIconCacheMax+50; i++ {
		now = now.Add(time.Second)
		c.get(iconSource{key: fmt.Sprint("k", i)}, func() string { return "u" })
	}
	if n := len(c.entries); n > gameIconCacheMax {
		t.Fatalf("bộ nhớ đệm có %d mục, trần %d", n, gameIconCacheMax)
	}
	// Mục cũ nhất bị bỏ, mục mới nhất còn.
	if _, ok := c.entries["k0"]; ok {
		t.Error("k0 (cũ nhất) lẽ ra đã bị bỏ")
	}
	if _, ok := c.entries[fmt.Sprint("k", gameIconCacheMax+49)]; !ok {
		t.Error("mục mới nhất không được bị bỏ")
	}
}

func TestIconCache_DonKetQuaRongHetHanTruoc(t *testing.T) {
	now := time.Unix(1000, 0)
	c := newTestIconCache(&now)
	// Mục có icon CŨ NHẤT, rồi lấp đầy bằng kết quả rỗng.
	c.get(iconSource{key: "co-icon"}, func() string { return "u" })
	for i := 1; i < gameIconCacheMax; i++ {
		now = now.Add(time.Millisecond)
		c.get(iconSource{key: fmt.Sprint("rong", i)}, func() string { return "" })
	}
	// Mọi kết quả rỗng đã hết hạn: mục có icon phải được giữ lại dù cũ nhất.
	now = now.Add(gameIconMissTTL)
	c.get(iconSource{key: "moi"}, func() string { return "u" })
	if _, ok := c.entries["co-icon"]; !ok {
		t.Error("icon thật bị bỏ trong khi còn kết quả rỗng hết hạn để dọn")
	}
}

// ---- Tệp nguồn ------------------------------------------------------------

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckIconSource(t *testing.T) {
	root := t.TempDir()
	gameDir := filepath.Join(root, "lol")
	outside := filepath.Join(root, "ngoai")
	exe := filepath.Join(gameDir, "Bin", "Game.exe")
	writeFile(t, exe)
	writeFile(t, filepath.Join(outside, "evil.exe"))

	t.Run("tệp thường trong thư mục game", func(t *testing.T) {
		src, err := checkIconSource(gameDir, exe)
		if err != nil {
			t.Fatal(err)
		}
		if src.key == "" || src.path == "" {
			t.Errorf("thiếu khoá/đường dẫn: %+v", src)
		}
	})

	t.Run("không tồn tại", func(t *testing.T) {
		_, err := checkIconSource(gameDir, filepath.Join(gameDir, "khong-co.exe"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("err = %v, mong fs.ErrNotExist", err)
		}
	})

	t.Run("là thư mục", func(t *testing.T) {
		if _, err := checkIconSource(gameDir, filepath.Join(gameDir, "Bin")); err == nil {
			t.Error("thư mục phải bị từ chối")
		}
	})

	t.Run("nằm ngoài thư mục game", func(t *testing.T) {
		if _, err := checkIconSource(gameDir, filepath.Join(outside, "evil.exe")); err == nil {
			t.Error("tệp ngoài thư mục game phải bị từ chối")
		}
	})

	t.Run("symlink tới tệp ngoài", func(t *testing.T) {
		link := filepath.Join(gameDir, "link.exe")
		if err := os.Symlink(filepath.Join(outside, "evil.exe"), link); err != nil {
			t.Skip("không tạo được symlink:", err)
		}
		if _, err := checkIconSource(gameDir, link); err == nil {
			t.Error("symlink phải bị từ chối")
		}
	})

	t.Run("thư mục cha là symlink ra ngoài", func(t *testing.T) {
		if err := os.Symlink(outside, filepath.Join(gameDir, "Ra")); err != nil {
			t.Skip("không tạo được symlink:", err)
		}
		if _, err := checkIconSource(gameDir, filepath.Join(gameDir, "Ra", "evil.exe")); err == nil {
			t.Error("tệp qua thư mục liên kết ra ngoài phải bị từ chối")
		}
	})

	t.Run("thư mục tên bắt đầu giống thư mục game", func(t *testing.T) {
		// lol2 không nằm trong lol dù chuỗi bắt đầu giống nhau.
		sibling := filepath.Join(root, "lol2", "Game.exe")
		writeFile(t, sibling)
		if _, err := checkIconSource(gameDir, sibling); err == nil {
			t.Error("thư mục anh em phải bị từ chối")
		}
	})
}

func TestCheckIconSource_KhoaDoiKhiTepDoi(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "lol", "Game.exe")
	writeFile(t, exe)
	a, err := checkIconSource(filepath.Join(root, "lol"), exe)
	if err != nil {
		t.Fatal(err)
	}
	// Game cập nhật: nội dung và giờ sửa đổi -> khoá đổi -> icon được lấy lại.
	if err := os.WriteFile(exe, []byte("MZ-moi-dai-hon"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(exe, later, later); err != nil {
		t.Fatal(err)
	}
	b, err := checkIconSource(filepath.Join(root, "lol"), exe)
	if err != nil {
		t.Fatal(err)
	}
	if a.key == b.key {
		t.Error("khoá không đổi sau khi tệp đổi")
	}
}

// ---- Cả đường GameIcon ----------------------------------------------------

// thayTrichIcon thay hàm trích icon thật bằng hàm giả và bộ nhớ đệm mới cho một
// bài kiểm, rồi trả bộ đếm số lần được gọi.
func thayTrichIcon(t *testing.T, fn func(path string) (*iconPixels, error)) *int {
	t.Helper()
	oldExtract, oldCache := gameIconExtract, gameIcons
	calls := 0
	gameIconExtract = func(path string, size int) (*iconPixels, error) {
		calls++
		return fn(path)
	}
	gameIcons = &iconCache{entries: map[string]iconCacheEntry{}, now: time.Now}
	t.Cleanup(func() { gameIconExtract, gameIcons = oldExtract, oldCache })
	return &calls
}

func onePixelIcon() (*iconPixels, error) {
	return &iconPixels{w: 1, h: 1, color: []byte{1, 2, 3, 255}}, nil
}

func TestGameIconDataURI(t *testing.T) {
	root := t.TempDir()
	gameDir := filepath.Join(root, "lol")
	exe := filepath.Join(gameDir, "Game.exe")
	writeFile(t, exe)

	t.Run("lấy icon và nhớ lại", func(t *testing.T) {
		calls := thayTrichIcon(t, func(string) (*iconPixels, error) { return onePixelIcon() })
		for i := 0; i < 3; i++ {
			if got := gameIconDataURI(gameDir, exe); !strings.HasPrefix(got, "data:image/png;base64,") {
				t.Fatalf("lần %d: got %.30q", i, got)
			}
		}
		if *calls != 1 {
			t.Errorf("trích %d lần, mong 1", *calls)
		}
	})

	t.Run("tệp đổi thì lấy lại", func(t *testing.T) {
		calls := thayTrichIcon(t, func(string) (*iconPixels, error) { return onePixelIcon() })
		gameIconDataURI(gameDir, exe)
		later := time.Now().Add(2 * time.Hour)
		if err := os.Chtimes(exe, later, later); err != nil {
			t.Fatal(err)
		}
		gameIconDataURI(gameDir, exe)
		if *calls != 2 {
			t.Errorf("trích %d lần, mong 2", *calls)
		}
	})

	t.Run("không có icon thì rỗng, không lỗi", func(t *testing.T) {
		thayTrichIcon(t, func(string) (*iconPixels, error) { return nil, errNoGameIcon })
		if got := gameIconDataURI(gameDir, exe); got != "" {
			t.Errorf("got %.30q, mong rỗng", got)
		}
	})

	t.Run("tệp thiếu thì không gọi trích", func(t *testing.T) {
		calls := thayTrichIcon(t, func(string) (*iconPixels, error) { return onePixelIcon() })
		if got := gameIconDataURI(gameDir, filepath.Join(gameDir, "khong-co.exe")); got != "" || *calls != 0 {
			t.Errorf("got %.30q, %d lần trích", got, *calls)
		}
	})

	t.Run("symlink thì không gọi trích", func(t *testing.T) {
		calls := thayTrichIcon(t, func(string) (*iconPixels, error) { return onePixelIcon() })
		link := filepath.Join(gameDir, "link.exe")
		if err := os.Symlink(exe, link); err != nil {
			t.Skip("không tạo được symlink:", err)
		}
		if got := gameIconDataURI(gameDir, link); got != "" || *calls != 0 {
			t.Errorf("got %.30q, %d lần trích", got, *calls)
		}
	})
}

func TestGameIconTarget(t *testing.T) {
	m := &gameMenu{Root: `D:\Games`, Items: []gameMenuItem{
		{Name: "lol", Launcher: `Bin\Game.exe`},
		{Name: "hong", Launcher: `..\evil.exe`},
	}}

	exe, dir, ok := gameIconTarget(m, "lol")
	if !ok || exe != `D:\Games\lol\Bin\Game.exe` || dir != `D:\Games\lol` {
		t.Errorf("lol: %q %q %v", exe, dir, ok)
	}
	if _, _, ok := gameIconTarget(m, "khong-co"); ok {
		t.Error("tên không có trong menu phải bị từ chối")
	}
	if _, _, ok := gameIconTarget(m, "hong"); ok {
		t.Error("launcher thoát thư mục game phải bị từ chối")
	}
	m.Root = "Games"
	if _, _, ok := gameIconTarget(m, "lol"); ok {
		t.Error("gốc không hợp lệ phải bị từ chối")
	}
}

func TestGameIcon_DungMenuDaNho(t *testing.T) {
	lastGameMenu.Lock()
	oldMenu, oldAt := lastGameMenu.menu, lastGameMenu.at
	lastGameMenu.Unlock()
	t.Cleanup(func() {
		lastGameMenu.Lock()
		lastGameMenu.menu, lastGameMenu.at = oldMenu, oldAt
		lastGameMenu.Unlock()
	})

	rememberGameMenu(&gameMenu{Root: `Z:\vnet-khong-co`, Items: []gameMenuItem{{Name: "lol", Launcher: "Game.exe"}}})
	if recentGameMenu() == nil {
		t.Fatal("menu vừa nhớ phải còn dùng được")
	}
	// App rỗng: nếu GameIcon gọi HTTP thay vì dùng menu đã nhớ thì bài kiểm này hỏng.
	a := &App{}
	for _, name := range []string{"lol", "khong-co"} {
		if got, err := a.GameIcon(name); got != "" || err != nil {
			t.Errorf("GameIcon(%q) = %.30q, %v", name, got, err)
		}
	}

	lastGameMenu.Lock()
	lastGameMenu.at = time.Now().Add(-gameMenuMemoTTL - time.Second)
	lastGameMenu.Unlock()
	if recentGameMenu() != nil {
		t.Error("menu quá hạn không được dùng lại")
	}
}

// Một thư mục ra nhiều mục (khoá "<tên>/<số>"), và icon riêng thay cho launcher.
func TestGameIconTarget_KhoaVaIconRieng(t *testing.T) {
	m := &gameMenu{Root: `D:\Games`, Items: []gameMenuItem{
		{Key: "Riot Games", Name: "Riot Games", Launcher: `Riot Client\RiotClientServices.exe`, Icon: `League of Legends\LeagueClient.exe`},
		{Key: "LibreOffice/1", Name: "LibreOffice", Launcher: `program\swriter.exe`},
		{Key: "LibreOffice/2", Name: "LibreOffice", Launcher: `program\scalc.exe`},
		{Key: "hong/1", Name: "hong", Launcher: "a.exe", Icon: `..\evil.ico`},
	}}
	for _, c := range []struct{ key, exe, dir string }{
		{"Riot Games", `D:\Games\Riot Games\League of Legends\LeagueClient.exe`, `D:\Games\Riot Games`},
		{"LibreOffice/1", `D:\Games\LibreOffice\program\swriter.exe`, `D:\Games\LibreOffice`},
		{"LibreOffice/2", `D:\Games\LibreOffice\program\scalc.exe`, `D:\Games\LibreOffice`},
	} {
		exe, dir, ok := gameIconTarget(m, c.key)
		if !ok || exe != c.exe || dir != c.dir {
			t.Errorf("%s: %q %q %v", c.key, exe, dir, ok)
		}
	}
	if _, _, ok := gameIconTarget(m, "LibreOffice"); ok {
		t.Error("tên thư mục không phải khoá của mục nào khi game có mục riêng")
	}
	if _, _, ok := gameIconTarget(m, "hong/1"); ok {
		t.Error("icon thoát thư mục game phải bị từ chối")
	}
}

// Máy chủ cũ không gửi key: khoá là tên game.
func TestMenuKey_MayChuCu(t *testing.T) {
	m := &gameMenu{Items: []gameMenuItem{{Name: "lol"}, {Key: "LibreOffice/1", Name: "LibreOffice"}}}
	if g := m.findMenuItem("lol"); g == nil || g.Name != "lol" {
		t.Errorf("lol: %+v", g)
	}
	if g := m.findMenuItem("LibreOffice/1"); g == nil || g.Name != "LibreOffice" {
		t.Errorf("LibreOffice/1: %+v", g)
	}
	if g := m.findMenuItem("LibreOffice"); g != nil {
		t.Errorf("LibreOffice: %+v", g)
	}
}
