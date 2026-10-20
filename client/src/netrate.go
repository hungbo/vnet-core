package main

import (
	"strings"
	"sync"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
)

// Băng thông lên/xuống của máy trạm cho bảng Hoạt động. Hệ điều hành chỉ đưa TỔNG số byte
// đã qua card mạng từ lúc bật máy; tốc độ là hiệu giữa hai lần đọc chia cho thời gian giữa
// chúng, nên cần nhớ lần đọc trước.

type netCounters struct{ recv, sent uint64 }

// rateBetween đổi hai lần đọc thành byte/giây. Bộ đếm giảm (card mạng bị tắt bật lại, driver
// đặt lại) thì coi là 0 chứ không phải số khổng lồ do số học không dấu quay vòng.
func rateBetween(prev, cur netCounters, dt time.Duration) (down, up int64) {
	if dt <= 0 {
		return 0, 0
	}
	secs := dt.Seconds()
	if cur.recv >= prev.recv {
		down = int64(float64(cur.recv-prev.recv) / secs)
	}
	if cur.sent >= prev.sent {
		up = int64(float64(cur.sent-prev.sent) / secs)
	}
	return down, up
}

type netRate struct {
	mu   sync.Mutex
	last netCounters
	at   time.Time
	ok   bool
}

// sample ghi lần đọc mới và trả tốc độ so với lần trước; ok=false ở lần đầu (chưa có gì để so).
func (r *netRate) sample(now time.Time, cur netCounters) (down, up int64, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	had, prev, prevAt := r.ok, r.last, r.at
	r.last, r.at, r.ok = cur, now, true
	if !had {
		return 0, 0, false
	}
	down, up = rateBetween(prev, cur, now.Sub(prevAt))
	return down, up, true
}

// readNetCounters cộng mọi card mạng thật, bỏ loopback: lưu lượng nội bộ giữa các tiến trình
// trong máy không phải băng thông ra mạng.
func readNetCounters() (netCounters, bool) {
	stats, err := psnet.IOCounters(true)
	if err != nil || len(stats) == 0 {
		return netCounters{}, false
	}
	var c netCounters
	for _, s := range stats {
		name := strings.ToLower(s.Name)
		if strings.Contains(name, "loopback") || name == "lo" {
			continue
		}
		c.recv += s.BytesRecv
		c.sent += s.BytesSent
	}
	return c, true
}
