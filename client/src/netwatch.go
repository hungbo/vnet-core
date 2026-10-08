package main

import (
	"net"
	"sort"
	"strings"
)

// Theo dõi card mạng xuất hiện SAU khi máy đã chạy ổn định (Wi-Fi, điện thoại
// phát mạng qua USB). Chỉ báo về máy chủ để chủ quán biết; không tự ngắt gì.
type nicInfo struct {
	Name string
	IP   string
}

// listActiveNICs: các card đang bật và có địa chỉ IPv4 dùng được.
func listActiveNICs() []nicInfo {
	var out []nicInfo
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			// 169.254.x.x là địa chỉ Windows tự gán cho card KHÔNG nối được
			// đâu cả — không phải một đường mạng.
			if !ok || ipnet.IP.To4() == nil || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, nicInfo{Name: iface.Name, IP: ipnet.IP.String()})
			break
		}
	}
	return out
}

type nicWatcher struct {
	baseline map[string]bool
}

// setBaselineOnce chốt danh sách card "của máy". Gọi sau nhịp tim thành công
// đầu tiên chứ không phải lúc dịch vụ vừa chạy: lúc đó card chính có thể chưa
// kịp nhận IP, chốt sớm là chính nó bị báo thành card lạ.
func (w *nicWatcher) setBaselineOnce(now []nicInfo) {
	if w.baseline != nil {
		return
	}
	w.baseline = map[string]bool{}
	for _, n := range now {
		w.baseline[n.Name] = true
	}
}

// extra trả về các card ngoài danh sách đã chốt, dạng "tên (IP), ...". nil khi
// chưa chốt — chưa biết thì không khai, để máy chủ giữ nguyên giá trị đang có.
func (w *nicWatcher) extra(now []nicInfo) *string {
	if w.baseline == nil {
		return nil
	}
	var la []string
	for _, n := range now {
		if !w.baseline[n.Name] {
			la = append(la, n.Name+" ("+n.IP+")")
		}
	}
	sort.Strings(la)
	s := strings.Join(la, ", ")
	return &s
}
