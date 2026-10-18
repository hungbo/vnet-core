//go:build windows

package main

import (
	"log"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// tatKhoaKhiThuc tắt yêu cầu đăng nhập Windows khi máy thức dậy (xem wakelock.go).
// Chạy một lần lúc dịch vụ bắt đầu: máy không ổ cứng mất cài đặt này mỗi lần khởi động
// lại, và dịch vụ lên trước lần ngủ đầu tiên (Cloud Update cho ngủ khoảng 38 giây sau
// khi boot). Hai đường độc lập, lỗi đường nào chỉ ghi nhật ký.
func tatKhoaKhiThuc() {
	if k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, consoleLockPolicyKey,
		registry.SET_VALUE|registry.QUERY_VALUE); err != nil {
		log.Printf("[nguồn] không mở được khoá chính sách đăng nhập khi thức: %v", err)
	} else {
		for _, ten := range []string{"ACSettingIndex", "DCSettingIndex"} {
			if v, _, err := k.GetIntegerValue(ten); err == nil && v == 0 {
				continue
			}
			if err := k.SetDWordValue(ten, 0); err != nil {
				log.Printf("[nguồn] không ghi được %s: %v", ten, err)
			}
		}
		k.Close()
	}

	for _, args := range wakeLockPowercfgArgs() {
		cmd := exec.Command("powercfg", args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[nguồn] powercfg %v lỗi: %v %s", args, err, out)
			return
		}
	}
	log.Printf("[nguồn] đã tắt yêu cầu đăng nhập Windows khi máy thức dậy")
}
