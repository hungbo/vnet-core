//go:build windows

package main

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procQueryUnbiasedInterruptTime = kernel32.NewProc("QueryUnbiasedInterruptTime")
	procSetSuspendState            = windows.NewLazySystemDLL("powrprof.dll").NewProc("SetSuspendState")
)

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

// sleepMachine cho máy ngủ (S3) chứ không ngủ đông, giữ các sự kiện đánh thức (nút
// nguồn, Wake-on-LAN). Chỉ trả về khi máy đã thức dậy. Cần quyền SeShutdownPrivilege
// đang BẬT: tài khoản SYSTEM có quyền đó nhưng mặc định để tắt.
func sleepMachine() error {
	// Không có powrprof.dll thì Call panic ngay trong goroutine, kéo cả dịch vụ chết.
	if err := procSetSuspendState.Find(); err != nil {
		return err
	}
	if err := batQuyenTatMay(); err != nil {
		return err
	}
	// SetSuspendState(bHibernate, bForce, bWakeupEventsDisabled) = (FALSE, FALSE, FALSE).
	// Kết quả là BOOLEAN một byte: các byte cao của thanh ghi không xác định.
	if r, _, err := procSetSuspendState.Call(0, 0, 0); byte(r) == 0 {
		return err
	}
	return nil
}

func batQuyenTatMay() error {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &tok); err != nil {
		return err
	}
	defer tok.Close()
	ten, err := windows.UTF16PtrFromString("SeShutdownPrivilege")
	if err != nil {
		return err
	}
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, ten, &luid); err != nil {
		return err
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	return windows.AdjustTokenPrivileges(tok, false, &tp, 0, nil, nil)
}
