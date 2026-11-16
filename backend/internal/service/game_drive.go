package service

import (
	"os"
	"regexp"
	"strings"

	"github.com/vnet/core/internal/config"
)

// GameDrive là một ổ đĩa của máy chủ trong hộp Tải game và Thêm game cục bộ. Máy trạm thấy ổ đĩa của máy chủ dưới
// cùng chữ ổ, nên quán chỉ chọn ổ; thư mục trong ổ theo loại game (thuMucTheoLoai).
type GameDrive struct {
	Path       string `json:"path"`  // "E:\" (Windows) hoặc "/"
	Label      string `json:"label"` // tên ổ đặt trong Windows, ví dụ "GAMES"
	FreeBytes  uint64 `json:"free_bytes"`
	TotalBytes uint64 `json:"total_bytes"`
	System     bool   `json:"system"` // ổ cài Windows: lưu game ở đây dễ đầy ổ hệ thống
}

// Drives liệt kê các ổ đĩa của máy chủ kèm tên ổ và dung lượng còn trống.
func (s *GameService) Drives() ([]GameDrive, error) {
	if s.conf().Role == config.GameRoleMaster {
		return nil, ErrGameLocalOnMaster
	}
	sys := strings.ToUpper(os.Getenv("SystemDrive")) // "C:"
	out := []GameDrive{}
	for _, d := range oDiaMayChu() {
		label, free, total := thongTinODia(d.Path)
		out = append(out, GameDrive{
			Path: d.Path, Label: label, FreeBytes: free, TotalBytes: total,
			System: sys != "" && strings.HasPrefix(strings.ToUpper(d.Path), sys),
		})
	}
	return out, nil
}

// thuMucTheoLoai: thư mục trong ổ quán chọn lúc Tải. Công cụ vào <ổ>\Tools, game (online, offline, nhóm khác) vào
// <ổ>\Games. Nhận nhóm theo cùng cách menu máy trạm gộp tab TOOL (client GameMenu.vue GROUP_NAMES).
func thuMucTheoLoai(category string) string {
	switch strings.ToLower(foldVietnamese(strings.TrimSpace(category))) {
	case "cong cu", "tool", "tools":
		return "Tools"
	}
	return "Games"
}

var chuODia = regexp.MustCompile(`^[A-Za-z]:\\?$`)

// gocTheoODia đổi ổ quán chọn ("E:" hay "E:\") thành thư mục lưu của game thuộc nhóm category, ví dụ E:\Games.
// false khi không phải chữ ổ: ô này chỉ nhận chữ ổ, đường dẫn đầy đủ đi qua install_root.
func gocTheoODia(drive, category string) (string, bool) {
	d := strings.TrimSpace(drive)
	if !chuODia.MatchString(d) {
		return "", false
	}
	return strings.ToUpper(d[:1]) + `:\` + thuMucTheoLoai(category), true
}
