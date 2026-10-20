//go:build windows

package main

import "golang.org/x/sys/windows"

// osForegroundApp trả tên tệp chạy của cửa sổ đang ở trên cùng của PHIÊN HIỆN TẠI, nên chỉ
// giao diện (chạy trong phiên của khách) gọi được; dịch vụ nền ở phiên 0 không thấy màn hình.
//
// ("", true): chưa có cửa sổ nào ở trên cùng (đang chuyển màn hình, hoặc màn hình khoá) —
// một giá trị biết chắc. ("", false): có cửa sổ nhưng không đọc được tiến trình của nó.
func osForegroundApp() (string, bool) {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return "", true
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil || pid == 0 {
		return "", false
	}
	// PROCESS_QUERY_LIMITED_INFORMATION đủ cho cả tiến trình chạy ở quyền cao hơn.
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(p)
	buf := make([]uint16, windows.MAX_PATH*2)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(p, 0, &buf[0], &size); err != nil {
		return "", false
	}
	return exeBaseName(windows.UTF16ToString(buf[:size])), true
}
