package service

import (
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"
)

// Mọi lời gọi từ máy chủ tới Hub (thử key lúc dán, đọc catalog, tải .torrent) đi qua MỘT http.Client dựng ở
// đây. Địa chỉ Hub do quản trị viên gõ vào trang Game, nên không có rào này thì ô "địa chỉ Hub" biến máy chủ
// thành chỗ để dò mạng nội bộ: trỏ tới http://127.0.0.1:... hay http://169.254.169.254/ (dịch vụ metadata của
// đám mây), rồi đọc thông báo lỗi trả về.
//
// Kiểm ở lúc NỐI (Dialer.Control), không ở lúc phân giải tên: Control nhận đúng địa chỉ IP sắp được nối, nên
// một tên miền trả địa chỉ ngoài lúc kiểm rồi đổi sang địa chỉ nội bộ lúc nối (DNS rebinding) vẫn bị chặn.
// Chỉ chặn địa chỉ của CHÍNH máy này và địa chỉ không đi đâu được; dải riêng LAN (10.x, 192.168.x) vẫn nối được
// vì quán có thể đặt Hub hay master trong mạng nội bộ.

var errHubDiaChiNoiBo = errors.New("địa chỉ này là địa chỉ nội bộ của chính máy chủ (loopback, link-local hoặc không xác định): Hub phải là một máy khác")

// awsMetadataIPv6: dịch vụ metadata của AWS qua IPv6 nằm trong dải ULA, không bị IsLinkLocal* bắt.
var awsMetadataIPv6 = net.ParseIP("fd00:ec2::254")

// blockedHubIP: máy chủ không được nối tới địa chỉ này khi gọi Hub.
func blockedHubIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 { // 0.0.0.0/8: "mạng này", Linux nối vào đó là nối vào chính máy
			return true
		}
		ip = ip4
	}
	return ip.IsLoopback() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		ip.Equal(awsMetadataIPv6)
}

// hubDialControl chạy ngay trước mỗi lần nối, với địa chỉ IP đã phân giải ("ip:cổng").
func hubDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	// Địa chỉ có zone ("fe80::1%eth0") ParseIP không đọc được: cũng là link-local, chặn luôn.
	if ip := net.ParseIP(host); ip == nil || blockedHubIP(ip) {
		return errHubDiaChiNoiBo
	}
	return nil
}

// hubCheckRedirect không đi theo chuyển hướng nào: Hub chuyển sang địa chỉ khác là địa chỉ đó chưa qua
// bước kiểm của quản trị viên. Trả nguyên phản hồi 3xx để checkHub báo "nhập thẳng địa chỉ cuối cùng".
func hubCheckRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// newHubHTTPClient dựng http.Client cho mọi lời gọi tới Hub.
func newHubHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: hubDialControl}
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: hubCheckRedirect,
		Transport: &http.Transport{
			// Không dùng proxy trong môi trường: qua proxy thì địa chỉ được kiểm là của proxy chứ không
			// phải của Hub.
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}
