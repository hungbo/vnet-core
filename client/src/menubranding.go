package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Tên quán và logo trên menu game (roadmap C4). Máy chủ trả cafe_name và logo_url trong danh sách menu; logo là ảnh
// công khai /uploads/... mà trang web của máy trạm không với tới trực tiếp (nó chạy ở địa chỉ nội bộ của Wails), nên phần
// Go tải về và đưa cho giao diện dạng data URI — cùng cách đã làm với icon game.

const logoMaxBytes = 1 << 20

var logoExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// logoHopLeMay: đường dẫn ảnh của chính máy chủ (/uploads/...), đuôi ảnh, không thoát ra ngoài.
func logoHopLeMay(raw string) (string, bool) {
	p := strings.TrimSpace(raw)
	if !strings.HasPrefix(p, "/uploads/") || strings.ContainsAny(p, "?#\\") || strings.Contains(p, "..") || len(p) > 200 || path.Clean(p) != p {
		return "", false
	}
	for _, r := range p {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", false
		}
	}
	return p, logoExts[strings.ToLower(path.Ext(p))]
}

// logoMime nhận ảnh theo nội dung chứ không theo đuôi: đuôi do quản trị viên chọn, còn nội dung mới nói lên thứ sẽ được
// đưa vào thẻ <img>. SVG cố ý không nhận (có thể mang script).
func logoMime(body []byte) (string, bool) {
	switch mt := http.DetectContentType(body); mt {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return mt, true
	}
	return "", false
}

// fetchLogoDataURI tải logo và đổi thành data URI.
func fetchLogoDataURI(serverURL, urlPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(serverURL, "/")+urlPath, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("máy chủ trả %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, logoMaxBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > logoMaxBytes {
		return "", fmt.Errorf("logo lớn hơn %d MB", logoMaxBytes>>20)
	}
	mt, ok := logoMime(body)
	if !ok {
		return "", fmt.Errorf("không phải ảnh png, jpeg, gif hay webp")
	}
	return "data:" + mt + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

// logoCache nhớ logo đã tải theo đường dẫn: đổi logo là đổi đường dẫn (mỗi lần tải lên một tên mới), nên khoá theo đường
// dẫn đã đủ và không cần hạn dùng. Lần lỗi được nhớ một phút để menu không gọi lại máy chủ mỗi 10 giây.
type logoCache struct {
	mu      sync.Mutex
	path    string
	dataURI string
	failAt  time.Time
}

var brandingLogo logoCache

func (c *logoCache) get(serverURL, urlPath string, now time.Time) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == urlPath {
		if c.dataURI != "" || now.Sub(c.failAt) < time.Minute {
			return c.dataURI
		}
	}
	uri, err := fetchLogoDataURI(serverURL, urlPath)
	c.path, c.dataURI = urlPath, uri
	if err != nil {
		c.failAt = now
		return ""
	}
	return uri
}

// MenuBranding cho giao diện biết tên quán và logo (data URI) để hiện ở đầu menu game; rỗng là không có.
func (a *App) MenuBranding() (string, error) {
	m, err := gameMenuCache.get(a.menuSource())
	if err != nil {
		return "", err
	}
	out := struct {
		Name string `json:"name"`
		Logo string `json:"logo"`
	}{Name: strings.TrimSpace(m.CafeName)}
	if p, ok := logoHopLeMay(m.LogoURL); ok {
		out.Logo = brandingLogo.get(a.cfg.ServerURL, p, time.Now())
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}
