package main

import (
	"context"
	"log"
	"time"
)

// Lớp chống phá giờ chơi.
//
// Cách gian lận: giết dịch vụ nền và giao diện, rồi dùng máy mà không đăng nhập
// — không có phiên nào nên máy chủ không tính tiền. Dịch vụ đã canh giao diện
// (superviseUI) từ trước; đây là chiều còn lại, giao diện canh dịch vụ.
//
// Phần QUYẾT ĐỊNH tách khỏi phần quan sát có chủ đích: mọi thứ đụng tới Windows
// nằm trong guard_windows.go, còn luật "khi nào được tắt máy" là hàm thuần nên
// kiểm chứng được ở bất kỳ đâu. Đây là nơi một lỗi khiến cả quán tắt máy giữa
// giờ cao điểm, nên nó phải kiểm được mà không cần một cỗ máy Windows.

// guardObservation là những gì đọc được trong MỘT lượt kiểm.
type guardObservation struct {
	ServiceInstalled   bool // dịch vụ có được đăng ký trong Windows không
	ServiceQueryOK     bool // có đọc được trạng thái dịch vụ không
	ServiceRunning     bool
	StartOK            bool // lượt này có bật lại được dịch vụ không
	SystemShuttingDown bool // Windows đang tắt/khởi động lại
	MaintenanceMode    bool // có tệp cờ bảo trì cạnh .exe
	UIUptime           time.Duration
}

type guardPolicy struct {
	Every  time.Duration // nhịp kiểm
	Grace  time.Duration // im lặng bấy nhiêu lâu sau khi giao diện khởi động
	Strike int           // số lượt xấu LIÊN TIẾP trước khi tắt máy
}

func defaultGuardPolicy() guardPolicy {
	return guardPolicy{
		Every: 5 * time.Second,
		// Giao diện khởi động cùng Windows, thường TRƯỚC khi dịch vụ kịp chạy.
		// Không có quãng im lặng này thì mỗi lần bật máy là một lần tắt máy.
		Grace:  90 * time.Second,
		Strike: 12, // 12 × 5 giây = một phút liên tục không cứu được dịch vụ
	}
}

type guardAction int

const (
	guardNothing guardAction = iota
	guardWarn
	guardShutdown
)

type guardState struct {
	strikes int
}

// step chấm điểm một lượt quan sát.
//
// Nguyên tắc: CHỈ đếm khi chắc chắn. Bất cứ điều gì khiến ta không phán được —
// chưa cài dịch vụ, không đọc nổi trạng thái, Windows đang tắt, đang bảo trì,
// giao diện vừa mới khởi động — đều XOÁ bộ đếm chứ không cộng thêm. Thà bỏ sót
// một kẻ gian còn hơn tắt máy của một người khách đang chơi thật.
func (g *guardState) step(o guardObservation, p guardPolicy) guardAction {
	khongPhanDuoc := o.MaintenanceMode ||
		o.SystemShuttingDown ||
		o.UIUptime < p.Grace ||
		!o.ServiceInstalled ||
		!o.ServiceQueryOK

	if khongPhanDuoc {
		g.strikes = 0
		return guardNothing
	}

	// Dịch vụ đang chạy, hoặc vừa bật lại được: mọi thứ ổn.
	if o.ServiceRunning || o.StartOK {
		g.strikes = 0
		return guardNothing
	}

	g.strikes++
	if g.strikes >= p.Strike {
		return guardShutdown
	}
	return guardWarn
}

// runGuard chạy trong tiến trình GIAO DIỆN. Dịch vụ nền không chạy hàm này —
// nó đã có superviseUI lo chiều ngược lại.
func runGuard(ctx context.Context, cfg *Config, dangBaoTri func() bool) {
	if !cfg.GuardEnabled {
		log.Printf("[guard] đã tắt bằng cấu hình")
		return
	}
	if dangBaoTri == nil {
		dangBaoTri = func() bool { return false }
	}

	p := defaultGuardPolicy()
	st := &guardState{}
	batDau := time.Now()

	ticker := time.NewTicker(p.Every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		o := observeGuard(time.Since(batDau))
		// Nhân viên kỹ thuật đã mở máy bằng PIN thì việc đầu tiên họ làm thường
		// là tắt dịch vụ. Đọc đó thành phá hoại là tắt máy ngay giữa lúc sửa.
		if dangBaoTri() {
			o.MaintenanceMode = true
		}

		switch st.step(o, p) {
		case guardWarn:
			log.Printf("[guard] dịch vụ nền không chạy và không bật lại được (%d/%d lượt)",
				st.strikes, p.Strike)
		case guardShutdown:
			log.Printf("[guard] dịch vụ nền chết hẳn sau %d lượt — tắt máy để không ai chơi chùa",
				st.strikes)
			if err := shutdownMachine(); err != nil {
				log.Printf("[guard] không tắt được máy: %v", err)
				// Tắt không được thì đừng gọi lại mỗi 5 giây: đặt lại bộ đếm để
				// còn một phút nữa mới thử tiếp, và nhật ký không bị ngập.
				st.strikes = 0
			}
			return
		}
	}
}
