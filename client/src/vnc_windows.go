//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/mgr"
)

// applyVNC cài TightVNC nếu chưa có, ghi mật khẩu, khởi động lại dịch vụ để
// nhận cấu hình mới, rồi mở tường lửa cổng 5900 chỉ cho remoteIP.
func applyVNC(password, remoteIP string) error {
	installed, err := vncInstalled()
	if err != nil {
		return err
	}
	if !installed {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		msi := filepath.Join(filepath.Dir(exe), vncMSIName)
		if _, err := os.Stat(msi); err != nil {
			return fmt.Errorf("chưa có TightVNC và không thấy %s cạnh vnet-client.exe", vncMSIName)
		}
		if out, err := exec.Command("msiexec", vncMSIArgs(msi)...).CombinedOutput(); err != nil {
			return fmt.Errorf("cài TightVNC thất bại: %v %s", err, out)
		}
	}

	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\TightVNC\Server`, registry.SET_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return fmt.Errorf("mở registry TightVNC: %w", err)
	}
	defer k.Close()
	if err := errors.Join(
		k.SetBinaryValue("Password", vncObfuscate(password)),
		k.SetDWordValue("UseVncAuthentication", 1),
		k.SetDWordValue("AcceptRfbConnections", 1),
		k.SetDWordValue("RfbPort", 5900),
		// Cổng 5800 (trình xem Java qua HTTP) không dùng tới: tắt.
		k.SetDWordValue("AcceptHttpConnections", 0),
	); err != nil {
		return fmt.Errorf("ghi registry TightVNC: %w", err)
	}

	// TightVNC chỉ đọc registry lúc khởi động.
	_ = exec.Command("net", "stop", vncServiceName).Run()
	if out, err := exec.Command("net", "start", vncServiceName).CombinedOutput(); err != nil {
		return fmt.Errorf("khởi động dịch vụ TightVNC: %v %s", err, out)
	}

	_ = exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name="+vncRuleName).Run()
	if out, err := exec.Command("netsh", vncFirewallArgs(remoteIP)...).CombinedOutput(); err != nil {
		return fmt.Errorf("mở tường lửa cho VNC: %v %s", err, out)
	}
	return nil
}

func vncInstalled() (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()
	s, err := m.OpenService(vncServiceName)
	if err != nil {
		return false, nil
	}
	s.Close()
	return true, nil
}
