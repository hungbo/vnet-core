package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Icon game trong menu: lấy từ chính tệp chạy của game trên Game Disk rồi gửi
// cho giao diện dạng data URI PNG. Lấy ở máy trạm chứ không để máy chủ lưu ảnh
// riêng, vì quản trị viên không phải tải icon lên cho từng game và icon luôn
// khớp với bản game đang cài.

const (
	// Cạnh icon yêu cầu. Thẻ game vẽ ô ~56px; 128 vẫn nét ở màn hình tỉ lệ 200%
	// mà mỗi PNG chỉ vài chục KB.
	gameIconSize = 128

	// Menu tải gần nhất được dùng lại cho các lời gọi GameIcon. Giao diện làm mới
	// menu mỗi 10 giây, nên bản này gần như luôn còn mới; hết hạn thì mới phải
	// hỏi lại máy chủ.
	gameMenuMemoTTL = 2 * time.Minute

	// Kết quả "không có icon" chỉ nhớ ngắn: game vừa cài xong hoặc vừa cập nhật
	// thì tệp chạy mới xuất hiện, phải thử lại được mà không cần đổi tệp.
	gameIconMissTTL = 3 * time.Minute

	// Trần số mục trong bộ nhớ đệm. Quán không có tới ngần này game; trần chỉ để
	// khoá (đường dẫn + kích thước + giờ sửa) của các bản game cũ không ở lại mãi.
	gameIconCacheMax = 256
)

// errNoGameIcon: tệp không có icon dùng được. Bình thường (launcher .bat, game
// mới cài chưa đủ tệp), nên không ghi nhật ký.
var errNoGameIcon = errors.New("tệp không có icon")

// iconPixels là điểm ảnh thô của một icon, cùng dạng GDI trả về.
type iconPixels struct {
	w, h  int
	color []byte // BGRA, w*h*4, hàng từ trên xuống
	// Mặt nạ AND cùng kích thước, 32 bpp: điểm trắng là trong suốt. nil nếu
	// không đọc được; chỉ dùng cho icon cũ không có kênh alpha.
	mask []byte
}

// gameIconExtract đọc icon cỡ gần gameIconSize của một tệp (extractIconPixels ở
// gameicon_windows.go). Gán qua biến để bài kiểm thay được trên máy không có Win32.
var gameIconExtract = extractIconPixels

// ---- Menu tải gần nhất ----------------------------------------------------

// lastGameMenu ở cấp gói vì struct App nằm ở app.go. Mỗi tiến trình có đúng một
// App nên không lẫn.
var lastGameMenu struct {
	sync.Mutex
	menu *gameMenu
	at   time.Time
}

// rememberGameMenu ghi lại menu vừa tải để GameIcon khỏi gọi HTTP cho từng icon.
// fetchGameMenu gọi hàm này; nơi khác chỉ đọc menu nên dùng chung con trỏ được.
func rememberGameMenu(m *gameMenu) {
	lastGameMenu.Lock()
	lastGameMenu.menu, lastGameMenu.at = m, time.Now()
	lastGameMenu.Unlock()
}

func recentGameMenu() *gameMenu {
	lastGameMenu.Lock()
	defer lastGameMenu.Unlock()
	if lastGameMenu.menu != nil && time.Since(lastGameMenu.at) < gameMenuMemoTTL {
		return lastGameMenu.menu
	}
	return nil
}

// ---- API cho giao diện ----------------------------------------------------

// GameIcon trả icon của game dạng "data:image/png;base64,...", hoặc "" nếu không
// lấy được (giao diện khi đó vẽ chữ cái đầu của tên). "" không phải lỗi của
// khách nên không trả error; error chỉ dành cho việc không tải nổi menu từ máy
// chủ, để giao diện thử lại ở lượt làm mới sau.
//
// Cũng như LaunchGame, giao diện CHỈ gửi tên game: đường dẫn dựng lại từ menu
// của máy chủ, không tin chuỗi nào từ phía WebView.
func (a *App) GameIcon(name string) (string, error) {
	m := recentGameMenu()
	if m == nil {
		var err error
		if m, err = a.fetchGameMenu(); err != nil {
			return "", err
		}
	}
	exe, gameDir, ok := gameIconTarget(m, name)
	if !ok {
		return "", nil
	}
	return gameIconDataURI(gameDir, exe), nil
}

// gameIconTarget tra game trong menu và dựng đường dẫn tệp chạy cùng thư mục
// game, bằng đúng hàm LaunchGame dùng. Cấu hình hỏng thì không có icon.
func gameIconTarget(m *gameMenu, name string) (exe, gameDir string, ok bool) {
	for i := range m.Items {
		g := &m.Items[i]
		if g.Name != name {
			continue
		}
		exe, _, err := resolveGameLaunch(m.Root, g.Name, g.Launcher)
		if err != nil {
			return "", "", false
		}
		base, err := gameRootBase(m.Root)
		if err != nil {
			return "", "", false
		}
		return exe, base + `\` + g.Name, true
	}
	return "", "", false
}

// gameIconDataURI lấy icon của exe (nằm trong gameDir; cả hai là đường dẫn của
// hệ điều hành đang chạy), qua bộ nhớ đệm. "" nếu không có icon.
func gameIconDataURI(gameDir, exe string) string {
	src, err := checkIconSource(gameDir, exe)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("[game] bỏ qua icon %q: %v", exe, err)
		}
		return ""
	}
	return gameIcons.get(src, func() string {
		px, err := gameIconExtract(src.path, gameIconSize)
		if err == nil {
			var uri string
			if uri, err = iconPixelsToDataURI(px); err == nil {
				return uri
			}
		}
		if !errors.Is(err, errNoGameIcon) {
			log.Printf("[game] không lấy được icon %q: %v", exe, err)
		}
		return ""
	})
}

// ---- Kiểm tệp nguồn -------------------------------------------------------

type iconSource struct {
	path string // đường dẫn thật (đã giải symlink) để đọc
	key  string // khoá bộ nhớ đệm
}

// checkIconSource chỉ cho đọc icon từ một tệp thường nằm thật sự trong thư mục
// game. resolveGameLaunch đã chặn ".." trên chữ, nhưng chữ không thấy được
// symlink/junction: Bin -> C:\Windows\System32 sẽ lọt qua. Lstat bắt tệp là
// symlink hay thư mục; EvalSymlinks bắt thư mục cha là liên kết đi ra ngoài.
// Thư mục game được giải liên kết trước khi so, nên game cố ý đặt junction ở
// chính thư mục game vẫn dùng được.
//
// Tệp không tồn tại trả lỗi bọc fs.ErrNotExist (trường hợp thường: game chưa cài xong).
func checkIconSource(gameDir, exe string) (iconSource, error) {
	fi, err := os.Lstat(exe)
	if err != nil {
		return iconSource{}, err
	}
	if !fi.Mode().IsRegular() {
		return iconSource{}, errors.New("không phải tệp thường")
	}
	realExe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return iconSource{}, err
	}
	realDir, err := filepath.EvalSymlinks(gameDir)
	if err != nil {
		return iconSource{}, err
	}
	if !insideDir(filepath.ToSlash(realDir), filepath.ToSlash(realExe)) {
		return iconSource{}, errors.New("tệp nằm ngoài thư mục game")
	}
	// Kích thước + giờ sửa: game cập nhật thì tệp đổi và icon được lấy lại.
	key := fmt.Sprintf("%s|%d|%d|%d", strings.ToLower(realExe), gameIconSize, fi.Size(), fi.ModTime().UnixNano())
	return iconSource{path: realExe, key: key}, nil
}

// ---- Bộ nhớ đệm -----------------------------------------------------------

type iconCacheEntry struct {
	uri string // "" = không có icon
	at  time.Time
}

type iconCache struct {
	mu      sync.Mutex
	entries map[string]iconCacheEntry
	// Chỉ một lần trích icon chạy một lúc: giao diện xin cả chục icon cùng lúc,
	// và hai lời gọi cho cùng một tệp không nên cùng đọc nó.
	extractMu sync.Mutex
	now       func() time.Time
}

var gameIcons = &iconCache{entries: map[string]iconCacheEntry{}, now: time.Now}

// get trả icon trong bộ nhớ đệm, hoặc gọi extract (một lần, tuần tự) rồi nhớ kết
// quả — kể cả "" để game không có icon không bị trích lại mỗi lần giao diện hỏi.
func (c *iconCache) get(src iconSource, extract func() string) string {
	if uri, ok := c.lookup(src.key); ok {
		return uri
	}
	c.extractMu.Lock()
	defer c.extractMu.Unlock()
	// Lời gọi trước có thể vừa trích xong chính tệp này trong lúc ta chờ khoá.
	if uri, ok := c.lookup(src.key); ok {
		return uri
	}
	uri := extract()
	c.store(src.key, uri)
	return uri
}

func (c *iconCache) lookup(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || e.uri == "" && c.now().Sub(e.at) >= gameIconMissTTL {
		return "", false
	}
	return e.uri, true
}

func (c *iconCache) store(key, uri string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= gameIconCacheMax {
		c.evict()
	}
	c.entries[key] = iconCacheEntry{uri: uri, at: c.now()}
}

// evict dọn bớt khi đầy: bỏ trước các kết quả "không có icon" đã hết hạn, vẫn đầy
// thì bỏ mục cũ nhất. Gọi khi đang giữ c.mu.
func (c *iconCache) evict() {
	now := c.now()
	var oldestKey string
	var oldest time.Time
	for k, e := range c.entries {
		if e.uri == "" && now.Sub(e.at) >= gameIconMissTTL {
			delete(c.entries, k)
			continue
		}
		if oldestKey == "" || e.at.Before(oldest) {
			oldestKey, oldest = k, e.at
		}
	}
	if len(c.entries) >= gameIconCacheMax {
		delete(c.entries, oldestKey)
	}
}

// ---- Điểm ảnh -> PNG ------------------------------------------------------

// maxIconSide chặn kích thước do Win32 báo: icon thật không quá 256, còn số lạ
// thì không đáng cấp phát bộ nhớ theo.
const maxIconSide = 512

// iconPixelsToDataURI đổi điểm ảnh icon sang PNG vuông, trả dạng data URI.
func iconPixelsToDataURI(p *iconPixels) (string, error) {
	img, err := iconToNRGBA(p)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// iconToNRGBA đổi BGRA (kèm mặt nạ) của GDI sang ảnh NRGBA vuông.
//
// Alpha có ba nguồn, theo thứ tự:
//   - Icon hiện đại có kênh alpha thật: giữ nguyên.
//   - Icon cũ không có alpha: GDI trả alpha = 0 ở MỌI điểm (đừng nhầm thành
//     "trong suốt hết"), độ trong suốt nằm ở mặt nạ AND.
//   - Không có mặt nạ: coi như đặc.
func iconToNRGBA(p *iconPixels) (*image.NRGBA, error) {
	w, h := p.w, p.h
	if w <= 0 || h <= 0 || w > maxIconSide || h > maxIconSide {
		return nil, fmt.Errorf("kích thước icon %dx%d không hợp lệ", w, h)
	}
	n := w * h * 4
	if len(p.color) != n {
		return nil, fmt.Errorf("điểm ảnh icon có %d byte, cần %d", len(p.color), n)
	}
	mask := p.mask
	if len(mask) != n {
		mask = nil
	}

	hasAlpha := false
	for i := 3; i < n; i += 4 {
		if p.color[i] != 0 {
			hasAlpha = true
			break
		}
	}
	unpremultiply := hasAlpha && looksPremultiplied(p.color)

	// Icon không vuông thì đặt giữa khung vuông, giao diện vẽ ô vuông.
	side := w
	if h > side {
		side = h
	}
	offX, offY := (side-w)/2, (side-h)/2
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	visible := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := (y*w + x) * 4
			b, g, r, a := p.color[s], p.color[s+1], p.color[s+2], p.color[s+3]
			switch {
			case hasAlpha:
				if unpremultiply && a != 0 && a != 0xFF {
					r, g, b = unpremul(r, a), unpremul(g, a), unpremul(b, a)
				}
			case mask != nil && mask[s] >= 0x80:
				a = 0
			default:
				a = 0xFF
			}
			if a == 0 {
				// Điểm trong suốt không cần màu; để 0 cho PNG nén nhỏ hơn.
				continue
			}
			visible = true
			d := img.PixOffset(x+offX, y+offY)
			img.Pix[d], img.Pix[d+1], img.Pix[d+2], img.Pix[d+3] = r, g, b, a
		}
	}
	if !visible {
		return nil, fmt.Errorf("%w: icon trong suốt hoàn toàn", errNoGameIcon)
	}
	return img, nil
}

// looksPremultiplied đoán điểm ảnh đã nhân sẵn alpha hay chưa. Màu đã nhân alpha
// thì không bao giờ lớn hơn alpha, nên chỉ cần một điểm bán trong suốt có kênh
// màu vượt alpha là biết ảnh KHÔNG nhân sẵn. Ngược lại, mọi điểm bán trong suốt
// đều thoả (và có ít nhất một điểm như vậy) thì coi là đã nhân: nếu đoán nhầm cho
// icon tối màu, viền chỉ sáng lên chút ít; còn nhầm theo hướng kia thì viền icon
// sáng màu bị một vòng tối.
func looksPremultiplied(bgra []byte) bool {
	semi := false
	for i := 0; i+3 < len(bgra); i += 4 {
		a := bgra[i+3]
		if a == 0xFF {
			continue
		}
		if bgra[i] > a || bgra[i+1] > a || bgra[i+2] > a {
			return false
		}
		if a != 0 {
			semi = true
		}
	}
	return semi
}

// unpremul đưa một kênh màu đã nhân alpha về màu thẳng (có làm tròn). a > 0.
func unpremul(c, a uint8) uint8 {
	v := (uint32(c)*255 + uint32(a)/2) / uint32(a)
	if v > 255 {
		v = 255
	}
	return uint8(v)
}
