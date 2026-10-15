//go:build windows

package main

import (
	"time"
	"unsafe"
)

var procQueryUnbiasedInterruptTime = kernel32.NewProc("QueryUnbiasedInterruptTime")

// awakeTime: thời gian máy THỨC kể từ lúc bật — không tính lúc ngủ (S3) hay ngủ
// đông. Đồng hồ Go và GetTickCount64 thì tính cả lúc ngủ, nên hiệu của hai đồng
// hồ giữa hai lượt chính là khoảng máy đã ngủ. 0 nếu không đọc được.
func awakeTime() time.Duration {
	var t uint64 // đơn vị 100 ns
	r, _, _ := procQueryUnbiasedInterruptTime.Call(uintptr(unsafe.Pointer(&t)))
	if r == 0 {
		return 0
	}
	return time.Duration(t) * 100
}
