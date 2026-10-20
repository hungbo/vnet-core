package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Hình nền máy trạm và trình chiếu (roadmap C1). Chạy trong GIAO DIỆN (phiên của khách): hình nền là thiết lập theo
// người dùng, dịch vụ nền ở phiên 0 không đặt được cho khách. Quán diskless khởi động lại là hình nền về bản gốc, nên mỗi
// lần giao diện lên là tải lại ảnh (bộ nhớ đệm nằm ở thư mục của khách, cũng mất khi khởi động lại) và đặt lại.
//
// Danh sách ảnh và thời gian đổi ảnh đến từ chính sách (clientPolicy.Wallpapers), nên đổi ở trang quản trị là mọi máy đổi
// theo ở nhịp tim kế tiếp. Chính sách rỗng nghĩa là tắt: máy trạm không đụng vào hình nền của ai.

const (
	wallpaperMaxBytes   = 25 << 20
	wallpaperRetryAfter = time.Minute
	wallpaperTick       = 5 * time.Second
)

// hasImageMagic: đầu tệp đúng JPEG hoặc PNG. Máy chủ trả trang lỗi HTML với mã 200 (proxy, trang đăng nhập của quán net)
// thì không được ghi vào đĩa rồi đặt làm hình nền.
func hasImageMagic(head []byte) bool {
	return bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}) || bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n"))
}

// wallpaperFileName: tên tệp trong bộ nhớ đệm, suy từ đường dẫn trên máy chủ nên cùng ảnh luôn cùng tên và ảnh khác không
// đè nhau. Đuôi giữ nguyên để Windows biết định dạng.
func wallpaperFileName(urlPath string) string {
	sum := sha256.Sum256([]byte(urlPath))
	return hex.EncodeToString(sum[:8]) + strings.ToLower(path.Ext(urlPath))
}

// wallpaperRunner giữ trạng thái của vòng hình nền; mọi việc chạm vào hệ thống đi qua ba hàm để kiểm thử thay được.
type wallpaperRunner struct {
	dir   string
	fetch func(urlPath, dest string) error // tải một ảnh về dest
	set   func(file string) error          // đặt làm hình nền

	key     string // danh sách ảnh đang chạy, nối lại: đổi là làm lại từ đầu
	files   []string
	idx     int
	applied bool
	nextAt  time.Time
	retryAt time.Time
}

// tick áp chính sách p tại thời điểm now. Gọi định kỳ; mỗi lần làm tối đa một việc có thể chậm (tải ảnh).
func (w *wallpaperRunner) tick(p clientPolicy, now time.Time) {
	urls := danhSachHinhNen(p.Wallpapers)
	if len(urls) == 0 {
		w.key, w.files, w.idx, w.applied = "", nil, 0, false
		return
	}
	if key := strings.Join(urls, "\n"); key != w.key {
		w.key, w.files, w.idx, w.applied, w.retryAt = key, nil, 0, false, time.Time{}
		log.Printf("[hình nền] chính sách có %d ảnh, bắt đầu tải về %s", len(urls), w.dir)
	}
	// Tải ảnh còn thiếu. Một ảnh lỗi thì bỏ qua và thử lại sau một phút; các ảnh khác vẫn chạy.
	if len(w.files) < len(urls) && !now.Before(w.retryAt) {
		var files []string
		failed := false
		for _, u := range urls {
			dest := filepath.Join(w.dir, wallpaperFileName(u))
			if _, err := os.Stat(dest); err != nil {
				if err := w.fetch(u, dest); err != nil {
					log.Printf("[hình nền] không tải được %s: %v", u, err)
					failed = true
					continue
				}
			}
			files = append(files, dest)
		}
		w.files = files
		if failed {
			w.retryAt = now.Add(wallpaperRetryAfter)
		}
		// Có ảnh mới tới thì đặt lại từ ảnh đầu.
		w.applied = false
	}
	if len(w.files) == 0 {
		return
	}
	interval := time.Duration(khoangTrinhChieuMay(p.WallpaperIntervalSeconds)) * time.Second
	switch {
	case !w.applied:
		w.idx = 0
		w.apply(now, interval)
	case interval > 0 && len(w.files) > 1 && !now.Before(w.nextAt):
		w.idx = (w.idx + 1) % len(w.files)
		w.apply(now, interval)
	}
}

func (w *wallpaperRunner) apply(now time.Time, interval time.Duration) {
	if err := w.set(w.files[w.idx]); err != nil {
		log.Printf("[hình nền] không đặt được %s: %v", filepath.Base(w.files[w.idx]), err)
		w.nextAt = now.Add(wallpaperRetryAfter)
		return
	}
	w.applied = true
	w.nextAt = now.Add(interval)
	log.Printf("[hình nền] đã đặt %s (ảnh %d/%d)", filepath.Base(w.files[w.idx]), w.idx+1, len(w.files))
}

// fetchWallpaper tải ảnh từ máy chủ (đường dẫn /uploads/... công khai, không cần token) về dest.
func fetchWallpaper(serverURL, urlPath, dest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(serverURL, "/")+urlPath, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("máy chủ trả %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, wallpaperMaxBytes+1))
	if err != nil {
		return err
	}
	if len(body) > wallpaperMaxBytes {
		return fmt.Errorf("ảnh lớn hơn %d MB", wallpaperMaxBytes>>20)
	}
	if !hasImageMagic(body) {
		return fmt.Errorf("không phải ảnh JPEG hoặc PNG")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	log.Printf("[hình nền] đã tải %s (%d byte)", urlPath, len(body))
	return nil
}

// runWallpaper là vòng nền của giao diện.
func runWallpaper(ctx context.Context, cfg *Config) {
	defer ghiPanic("hình nền")
	w := &wallpaperRunner{
		dir:   wallpaperDir(),
		fetch: func(u, dest string) error { return fetchWallpaper(cfg.ServerURL, u, dest) },
		set:   osSetWallpaper,
	}
	t := time.NewTicker(wallpaperTick)
	defer t.Stop()
	for {
		w.tick(currentPolicy(), time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
