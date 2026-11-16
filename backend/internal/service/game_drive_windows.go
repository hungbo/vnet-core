//go:build windows

package service

import "golang.org/x/sys/windows"

// thongTinODia đọc tên ổ và dung lượng của ổ root ("E:\"). Đọc không được (ổ đĩa quang trống, ổ mạng mất kết nối) thì
// để trống: ổ vẫn hiện trong danh sách, chỉ thiếu số liệu.
func thongTinODia(root string) (label string, free, total uint64) {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "", 0, 0
	}
	name := make([]uint16, windows.MAX_PATH+1)
	if windows.GetVolumeInformation(p, &name[0], uint32(len(name)), nil, nil, nil, nil, 0) == nil {
		label = windows.UTF16ToString(name)
	}
	var avail, all, totalFree uint64
	if windows.GetDiskFreeSpaceEx(p, &avail, &all, &totalFree) == nil {
		free, total = avail, all
	}
	return label, free, total
}
