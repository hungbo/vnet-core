package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
)

// Menu game trong phòng máy: khách đăng nhập, mở "Game", thấy các game quán đã
// phát hành và bấm Chơi — game nằm sẵn trên Game Disk của máy trạm.

// gameStatusReady khớp model.GameStatusReady phía máy chủ. Máy trạm chỉ cần
// phân biệt một trạng thái: sẵn sàng thì chạy được, còn lại thì chờ.
const gameStatusReady = "ready"

// gameMenuItem là một game trong phản hồi của GET /api/game-menu.
type gameMenuItem struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	Version     int    `json:"version"`
}

// gameMenu: Root là đường dẫn GAME_ROOT NHƯ MÁY TRẠM THẤY (ổ Game Disk),
// không phải đường dẫn trên máy chủ quán.
type gameMenu struct {
	Root  string         `json:"root"`
	Items []gameMenuItem `json:"items"`
}

func (a *App) fetchGameMenu() (*gameMenu, error) {
	data, err := a.doRequest("GET", "/api/game-menu", nil)
	if err != nil {
		return nil, err
	}
	var m gameMenu
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("danh sách game không đọc được: %w", err)
	}
	// Giao diện lặp qua items: null thì v-for không hỏng nhưng "length" thì có.
	if m.Items == nil {
		m.Items = []gameMenuItem{}
	}
	return &m, nil
}

// ListGames trả JSON {root, items} các game quán đã phát hành cho phòng máy.
func (a *App) ListGames() (string, error) {
	m, err := a.fetchGameMenu()
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(m)
	return string(out), nil
}

// LaunchGame chạy một game từ Game Disk.
//
// Giao diện CHỈ gửi tên game, không gửi đường dẫn: tên được tra lại trong danh
// sách máy chủ vừa trả, và đường dẫn dựng từ chính danh sách đó. Nếu tin đường
// dẫn từ giao diện thì ai mở được DevTools cũng chạy được tệp bất kỳ trên máy.
func (a *App) LaunchGame(name string) error {
	if !a.loggedIn() {
		return errors.New("chưa đăng nhập")
	}

	m, err := a.fetchGameMenu()
	if err != nil {
		return err
	}
	var g *gameMenuItem
	for i := range m.Items {
		if m.Items[i].Name == name {
			g = &m.Items[i]
			break
		}
	}
	if g == nil {
		return errors.New("game này không còn trong danh sách của quán")
	}
	// Máy chủ đánh dấu ready khi bản trên Game Disk đã đủ. Đang tải dở mà chạy
	// thì game báo thiếu tệp, hoặc tệ hơn là chạy bản hỏng.
	if g.Status != gameStatusReady {
		return fmt.Errorf("%s chưa sẵn sàng, vui lòng chờ cập nhật xong", g.DisplayName)
	}

	exe, dir, err := resolveGameLaunch(m.Root, g.Name, g.Launcher)
	if err != nil {
		log.Printf("[game] từ chối %q: %v", g.Name, err)
		return errors.New("cấu hình chạy game không hợp lệ, vui lòng báo nhân viên")
	}
	if st, err := os.Stat(exe); err != nil || !st.Mode().IsRegular() {
		log.Printf("[game] không thấy tệp chạy %q (%s): %v", g.Name, exe, err)
		return fmt.Errorf("không tìm thấy tệp chạy %s trên ổ game, vui lòng báo nhân viên", g.DisplayName)
	}

	if err := startGame(exe, dir, g.LaunchArgs); err != nil {
		log.Printf("[game] không mở được %q (%s): %v", g.Name, exe, err)
		return fmt.Errorf("không mở được %s: %v", g.DisplayName, err)
	}
	log.Printf("[game] %s mở %q (%s)", a.username, g.Name, exe)
	return nil
}

// resolveGameLaunch dựng đường dẫn tệp chạy game: root\name\launcher.
//
// Hàm thuần, và cố ý tự xử lý dấu `\` thay vì dùng path/filepath: đường dẫn
// ở đây luôn là của Windows, mà bài kiểm chạy được cả trên Mac/Linux.
//
// launcher là chuỗi do quản trị viên nhập ở trang quản trị, và nó đi thẳng vào
// lệnh chạy tệp nên phải coi là không tin cậy: chỉ nhận đường dẫn TƯƠNG ĐỐI,
// gồm các thành phần bình thường, không "..", không ổ đĩa, không UNC.
func resolveGameLaunch(root, name, launcher string) (exe, dir string, err error) {
	base, err := gameRootBase(root)
	if err != nil {
		return "", "", err
	}
	if err := checkGameComponent(name); err != nil {
		return "", "", fmt.Errorf("tên game %q: %w", name, err)
	}

	launcher = strings.TrimSpace(launcher)
	if launcher == "" {
		return "", "", errors.New("game chưa khai báo tệp chạy")
	}
	rel := strings.ReplaceAll(launcher, "/", `\`)
	// Đường dẫn tuyệt đối và UNC (\\máy\thư mục, \\?\...) đều mở đầu bằng dấu
	// gạch; kiểu ổ đĩa ("C:\...", "C:tệp") bị bắt ở dấu ":" trong từng thành phần.
	if rel[0] == '\\' {
		return "", "", fmt.Errorf("tệp chạy %q là đường dẫn tuyệt đối hoặc UNC", launcher)
	}
	parts := strings.Split(rel, `\`)
	for _, p := range parts {
		if err := checkGameComponent(p); err != nil {
			return "", "", fmt.Errorf("tệp chạy %q: %w", launcher, err)
		}
	}

	// path.Clean chạy trên dạng dấu "/": cùng một nguyên tắc với filepath.Clean
	// của Windows, nhưng không phụ thuộc hệ điều hành đang chạy bài kiểm.
	gameDir := path.Clean(strings.ReplaceAll(base, `\`, "/") + "/" + name)
	cleaned := path.Clean(gameDir + "/" + strings.Join(parts, "/"))
	// Kiểm lần nữa sau khi làm sạch, độc lập với luật từng thành phần ở trên:
	// luật đó mà sau này nới ra thì đây vẫn chặn được việc thoát khỏi root\name.
	if !insideDir(gameDir, cleaned) {
		return "", "", fmt.Errorf("tệp chạy %q nằm ngoài thư mục game", launcher)
	}

	exe = strings.ReplaceAll(cleaned, "/", `\`)
	return exe, exe[:strings.LastIndex(exe, `\`)], nil
}

// gameRootBase kiểm tra gốc game có dạng "X:\..." rồi trả về nó không còn dấu
// gạch ở cuối (ổ gốc "D:\" thành "D:", để ghép "\tên" vào vẫn đúng).
//
// Máy chủ đã chuẩn hoá cài đặt này và trả ổ gốc là "D:" (không có gạch), nên "D:"
// trơn cũng phải nhận; "D:Games" thì không (đường dẫn tương đối theo ổ).
// Đây là điểm cuối trước khi chạy tệp nên không tin lại: gốc mà có ".." thì mọi
// game đều trượt ra ngoài ổ game.
func gameRootBase(root string) (string, error) {
	r := strings.ReplaceAll(strings.TrimSpace(root), "/", `\`)
	if len(r) < 2 || !isASCIILetter(r[0]) || r[1] != ':' || len(r) > 2 && r[2] != '\\' {
		return "", fmt.Errorf("thư mục gốc game %q không phải đường dẫn dạng X:\\...", root)
	}
	if len(r) == 2 {
		return r, nil
	}
	rest := strings.TrimRight(r[3:], `\`)
	if rest == "" {
		return r[:2], nil
	}
	for _, p := range strings.Split(rest, `\`) {
		if err := checkGameComponent(p); err != nil {
			return "", fmt.Errorf("thư mục gốc game %q: %w", root, err)
		}
	}
	return r[:3] + rest, nil
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// checkGameComponent nhận một thành phần đường dẫn đơn: tên thư mục hoặc tên tệp.
func checkGameComponent(c string) error {
	if c == "" {
		return errors.New("có thành phần rỗng")
	}
	for _, r := range c {
		// ":" chặn cả ổ đĩa lẫn luồng dữ liệu phụ NTFS ("Game.exe:luồng").
		if r < 0x20 || strings.ContainsRune(`<>:"|?*\/`, r) {
			return fmt.Errorf("ký tự %q không hợp lệ", r)
		}
	}
	// Windows bỏ dấu chấm và khoảng trắng ở cuối thành phần, nên ".. ", "..." hay
	// "Bin " mơ hồ không kém gì "..": không cần đoán xem hệ điều hành hiểu ra sao.
	if strings.Trim(c, ". ") == "" {
		return fmt.Errorf("thành phần %q không hợp lệ", c)
	}
	if strings.HasSuffix(c, " ") || strings.HasSuffix(c, ".") {
		return fmt.Errorf("thành phần %q kết thúc bằng dấu chấm hoặc khoảng trắng", c)
	}
	return nil
}

// insideDir: p nằm BÊN TRONG dir (không tính chính dir). Cả hai là dạng dấu "/"
// đã làm sạch. So không phân biệt hoa thường như Windows, và phải so cả dấu "/"
// ở cuối tên thư mục — nếu không "D:/Games/lol2/x" bị tính là nằm trong "D:/Games/lol".
func insideDir(dir, p string) bool {
	prefix := dir + "/"
	return len(p) > len(prefix) && strings.EqualFold(p[:len(prefix)], prefix)
}
