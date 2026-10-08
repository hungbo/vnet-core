package main

import (
	"crypto/des"
	"encoding/json"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
)

// Remote desktop dùng TightVNC Server cài trên máy trạm. Máy chủ VNET là bên
// DUY NHẤT nối vào cổng 5900 (nó làm cầu nối WebSocket cho noVNC ở trang quản
// trị), nên tường lửa chỉ mở cổng cho IP máy chủ. Mật khẩu VNC do máy chủ cấp
// qua phản hồi nhịp tim.

const (
	vncServiceName = "tvnserver"
	vncMSIName     = "tightvnc.msi"
	vncRuleName    = "VNET_VNC"
	vncPort        = "5900"
)

// vncDESKey là khoá DES cố định VNC dùng để làm rối mật khẩu lưu trong registry:
// {23,82,107,6,35,78,88,7} của d3des, đảo bit từng byte cho DES chuẩn.
var vncDESKey = []byte{0xe8, 0x4a, 0xd6, 0x60, 0xc4, 0x72, 0x1a, 0xe0}

// vncObfuscate trả 8 byte TightVNC lưu ở giá trị Password. VNC chỉ dùng 8 ký tự
// đầu; thiếu thì đệm 0.
func vncObfuscate(password string) []byte {
	plain := make([]byte, 8)
	copy(plain, password)
	block, _ := des.NewCipher(vncDESKey) // khoá đúng 8 byte: không bao giờ lỗi
	out := make([]byte, 8)
	block.Encrypt(out, plain)
	return out
}

// vncFirewallArgs dựng lệnh netsh mở cổng VNC chỉ cho remoteIP.
func vncFirewallArgs(remoteIP string) []string {
	return []string{"advfirewall", "firewall", "add", "rule",
		"name=" + vncRuleName, "dir=in", "action=allow", "protocol=TCP",
		"localport=" + vncPort, "remoteip=" + remoteIP}
}

// vncMSIArgs dựng lệnh cài im lặng TightVNC: chỉ phần Server, chạy như dịch vụ,
// không tự mở tường lửa (VNET tự mở, giới hạn theo IP máy chủ). Mật khẩu ghi
// sau qua registry nên không nằm trên dòng lệnh.
func vncMSIArgs(msiPath string) []string {
	return []string{"/i", msiPath, "/quiet", "/norestart",
		"ADDLOCAL=Server",
		"SERVER_REGISTER_AS_SERVICE=1",
		"SERVER_ADD_FIREWALL_EXCEPTION=0",
		"SERVER_ALLOW_SAS=1",
	}
}

// vncRemoteIP là giá trị remoteip cho luật tường lửa: các IPv4 của máy chủ VNET
// lấy từ server_url. Không phân giải được (hoặc máy chủ là chính máy này) thì
// lùi về LocalSubnet — vẫn chặn được mọi thứ ngoài mạng quán.
func vncRemoteIP(serverURL string) string {
	u, err := url.Parse(serverURL)
	if err != nil || u.Hostname() == "" {
		return "LocalSubnet"
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return "LocalSubnet"
	}
	var out []string
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() {
			out = append(out, ip4.String())
		}
	}
	if len(out) == 0 {
		return "LocalSubnet"
	}
	return strings.Join(out, ",")
}

var (
	vncMu       sync.Mutex
	vncDaApDung string // "mật khẩu|remoteip" đã áp lần gần nhất
)

// applyVNCFromHeartbeat đọc mật khẩu VNC trong phản hồi nhịp tim và cấu hình
// TightVNC khi mật khẩu hoặc IP máy chủ đổi. Chỉ dịch vụ nền (SYSTEM) làm việc
// này: giao diện không có quyền cài phần mềm hay ghi HKLM.
func applyVNCFromHeartbeat(cfg *Config, body []byte) {
	if !osIsWindows || agentSupervisor == nil {
		return
	}
	var resp struct {
		Data struct {
			VNCPassword string `json:"vnc_password"`
		} `json:"data"`
	}
	// Máy chủ bản cũ không gửi mật khẩu: không đụng vào VNC.
	if json.Unmarshal(body, &resp) != nil || resp.Data.VNCPassword == "" {
		return
	}
	if !vncMu.TryLock() {
		return // lần áp trước (có thể đang cài MSI) chưa xong
	}
	go func() {
		defer vncMu.Unlock()
		remoteIP := vncRemoteIP(cfg.ServerURL)
		key := resp.Data.VNCPassword + "|" + remoteIP
		if key == vncDaApDung {
			return
		}
		if err := applyVNC(resp.Data.VNCPassword, remoteIP); err != nil {
			log.Printf("[VNC] cấu hình thất bại: %v", err)
			return
		}
		vncDaApDung = key
		log.Printf("[VNC] đã cấu hình TightVNC, chỉ nhận kết nối từ %s", remoteIP)
	}()
}
