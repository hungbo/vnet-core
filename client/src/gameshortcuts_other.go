//go:build !windows

package main

// Desktop và lối tắt .lnk chỉ có trên máy trạm Windows. Bản này để phần còn lại
// biên dịch được khi phát triển trên Mac/Linux: thư mục rỗng nghĩa là không có
// desktop nào để đối chiếu.
func userDesktopDir() (string, error) { return "", nil }

func syncDesktopShortcuts(dir string, desired []gameShortcutSpec) ([]string, error) {
	return nil, nil
}
