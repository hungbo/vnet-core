package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Những việc tác động lên chính cỗ máy, viết dưới dạng hàm tự do vì có HAI nơi
// gọi: giao diện (qua binding cho frontend) và tiến trình nền (qua WebSocket
// bằng khoá máy). Hai bản sao của cùng một lệnh là hai chỗ để lệch nhau.

func shutdownMachine() error {
	return exec.Command("shutdown", "/s", "/t", "5").Run()
}

func restartMachine() error {
	return exec.Command("shutdown", "/r", "/t", "5").Run()
}

// blockAppByName chặn một ứng dụng bằng luật tường lửa theo TÊN TIẾN TRÌNH.
// Cố ý không nhận câu lệnh tuỳ ý — xem ghi chú về ExecuteCommand ở phía máy chủ.
func blockAppByName(processName string) error {
	name, err := safeProcessName(processName)
	if err != nil {
		return err
	}
	return exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=VNET_Block_%s", name),
		"dir=out",
		fmt.Sprintf("program=%%ProgramFiles%%\\%s.exe", name),
		"action=block",
		"enable=yes",
	).Run()
}

func unblockAppByName(processName string) error {
	name, err := safeProcessName(processName)
	if err != nil {
		return err
	}
	return exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=VNET_Block_%s", name),
	).Run()
}

// safeProcessName chặn những ký tự có thể lái tên thành đường dẫn hoặc tham số
// khác cho netsh. Tên tiến trình tới từ máy chủ qua WebSocket, nên không được
// tin nó là lành.
func safeProcessName(raw string) (string, error) {
	name := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ".exe"))
	if name == "" {
		return "", errors.New("thiếu tên ứng dụng")
	}
	for _, r := range name {
		ok := r == '-' || r == '_' || r == '.' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !ok {
			return "", fmt.Errorf("tên ứng dụng %q có ký tự không hợp lệ", raw)
		}
	}
	return name, nil
}
