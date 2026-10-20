package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Giảm số lần kéo danh sách game: giao diện hỏi mỗi 10 giây trên mọi máy trạm, mà danh sách ít khi đổi. Hỏi trước một chuỗi
// version rẻ (GET /api/game-menu/version); version không đổi thì dùng lại danh sách đã có. Danh sách vẫn được kéo lại:
//   - khi có game đang tải (tiến độ không nằm trong version);
//   - sau menuMaxAge, để lượt mở và nhãn Hot/New (đổi chậm, không nằm trong version) cũng theo kịp;
//   - khi máy chủ không có /version (máy chủ cũ): hỏi như trước, thử lại version sau menuVersionRetry.
//
// LaunchGame và GameIcon vẫn gọi fetchGameMenu trực tiếp: mở game phải dựa trên danh sách mới nhất của máy chủ.

const (
	menuMaxAge       = 5 * time.Minute
	menuVersionRetry = 10 * time.Minute
)

// menuSource gom ba nguồn dữ liệu để bài kiểm thay được.
type menuSource struct {
	version func() (string, error)
	menu    func() (*gameMenu, error)
	now     func() time.Time
}

// menuCache là danh sách đã kéo gần nhất. Một khoá chung giữ suốt lượt hỏi: hai lượt cùng lúc không kéo hai lần.
type menuCache struct {
	mu             sync.Mutex
	menu           *gameMenu
	version        string // version đọc TRƯỚC khi kéo danh sách: thay đổi xen giữa thì lượt sau thấy version mới và kéo lại
	at             time.Time
	downloading    bool
	noVersionUntil time.Time
}

func (c *menuCache) reset() {
	c.mu.Lock()
	c.menu, c.version, c.at, c.downloading = nil, "", time.Time{}, false
	c.mu.Unlock()
}

// get trả danh sách menu, kéo lại từ máy chủ khi cần (xem đầu tệp).
func (c *menuCache) get(src menuSource) (*gameMenu, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := src.now()
	canAsk := !now.Before(c.noVersionUntil)
	if c.menu != nil && c.version != "" && !c.downloading && now.Sub(c.at) < menuMaxAge && canAsk {
		v, err := src.version()
		if err == nil && v == c.version {
			return c.menu, nil
		}
		if err != nil {
			c.noVersionUntil = now.Add(menuVersionRetry)
			canAsk = false
		}
	}
	var version string
	if canAsk {
		if v, err := src.version(); err == nil {
			version = v
		} else {
			c.noVersionUntil = now.Add(menuVersionRetry)
		}
	}
	m, err := src.menu()
	if err != nil {
		return nil, err
	}
	c.menu, c.version, c.at = m, version, now
	c.downloading = false
	for i := range m.Items {
		if m.Items[i].Status == "downloading" {
			c.downloading = true
			break
		}
	}
	return m, nil
}

var gameMenuCache menuCache

// fetchMenuVersion hỏi dấu version của dữ liệu menu.
func (a *App) fetchMenuVersion() (string, error) {
	data, err := a.doRequest("GET", "/api/game-menu/version", nil)
	if err != nil {
		return "", err
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &v); err != nil || v.Version == "" {
		return "", fmt.Errorf("version menu không đọc được: %v", err)
	}
	return v.Version, nil
}

func (a *App) menuSource() menuSource {
	return menuSource{version: a.fetchMenuVersion, menu: a.fetchGameMenu, now: time.Now}
}
