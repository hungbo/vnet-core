//go:build !windows

package service

import "golang.org/x/sys/unix"

// thongTinODia: máy chủ không phải Windows chỉ có "/" (chạy thử, CI). Không có tên ổ.
func thongTinODia(root string) (label string, free, total uint64) {
	var st unix.Statfs_t
	if unix.Statfs(root, &st) != nil {
		return "", 0, 0
	}
	return "", st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize)
}
