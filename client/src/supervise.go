package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Chính sách bảo vệ và bộ giám sát giao diện.
//
// Nguyên tắc: dịch vụ nền (chạy dưới SYSTEM) và máy chủ là nơi QUYẾT ĐỊNH; giao
// diện chạy dưới tài khoản của khách nên chỉ báo cáo và làm theo. Phần quyết
// định ở tệp này là hàm thuần — không đụng Windows — để kiểm được ở bất kỳ đâu,
// đúng lối guard.go đã làm: đây là nơi một lỗi khiến cả phòng máy khởi động lại.

// clientPolicy là chính sách máy chủ gửi kèm phản hồi nhịp tim.
type clientPolicy struct {
	TamperAction         string `json:"tamper_action"`          // "restart" | "shutdown"
	OfflineLockSeconds   int64  `json:"offline_lock_seconds"`   // mất kết nối bấy lâu thì khoá
	OfflineRebootSeconds int64  `json:"offline_reboot_seconds"` // bấy lâu thì khởi động lại; 0 = không
	// Tài khoản quản trị máy trạm đặt trên trang quản trị. Có thì thay cho tài
	// khoản đặt lúc cài (config.json). Mật khẩu chỉ ở dạng băm.
	LocalAdminUsername string `json:"local_admin_username,omitempty"`
	LocalAdminHash     string `json:"local_admin_hash,omitempty"`
	// Ứng dụng cấm chạy trên mọi máy, "chrome,steam". Chuỗi chứ không phải mảng
	// để clientPolicy vẫn so sánh được bằng == (setPolicy dựa vào đó).
	BlockedApps string `json:"blocked_apps,omitempty"`
	// Tên lối tắt (không đuôi) bị xoá khỏi desktop mọi máy, "Game Menu,Riot Client";
	// xem shortcuts.go. Rỗng = tính năng tắt. Cũng là chuỗi, vì cùng lý do trên.
	HiddenShortcuts string `json:"hidden_shortcuts,omitempty"`
	// Màn hình khoá không ai đăng nhập bấy nhiêu phút thì tắt máy; 0 = không.
	// Mặc định 0: chưa nghe máy chủ nói gì thì không tự tắt máy của ai cả.
	IdleShutdownMinutes int64 `json:"idle_shutdown_minutes"`
}

func defaultClientPolicy() clientPolicy {
	return clientPolicy{TamperAction: "restart", OfflineLockSeconds: 60, OfflineRebootSeconds: 300}
}

// normalizePolicy ép chính sách về khoảng hợp lệ, cùng luật với máy chủ. Máy
// trạm không tin thẳng con số nhận về: một giá trị 0 lọt vào đây là khoá máy
// ngay ở nhịp tim rớt đầu tiên.
func normalizePolicy(p clientPolicy) clientPolicy {
	if p.TamperAction != "shutdown" {
		p.TamperAction = "restart"
	}
	if p.OfflineLockSeconds < 30 {
		p.OfflineLockSeconds = 30
	}
	if p.OfflineRebootSeconds < 0 {
		p.OfflineRebootSeconds = 0
	}
	if p.OfflineRebootSeconds != 0 && p.OfflineRebootSeconds < p.OfflineLockSeconds+60 {
		p.OfflineRebootSeconds = p.OfflineLockSeconds + 60
	}
	if p.IdleShutdownMinutes < 0 {
		p.IdleShutdownMinutes = 0
	}
	if p.IdleShutdownMinutes > 1440 {
		p.IdleShutdownMinutes = 1440
	}
	// Danh sách lối tắt cần xoá quyết định tệp nào bị xoá khỏi desktop của khách:
	// ép về dạng chuẩn ngay cổng vào chứ không tin thẳng chuỗi nhận về.
	p.HiddenShortcuts = chuanHoaLoiTat(p.HiddenShortcuts)
	// Nửa tài khoản (có tên không băm hoặc ngược lại) là không có tài khoản.
	if p.LocalAdminUsername == "" || p.LocalAdminHash == "" {
		p.LocalAdminUsername, p.LocalAdminHash = "", ""
	}
	return p
}

var (
	policyMu  sync.RWMutex
	policyNow = defaultClientPolicy()
)

func currentPolicy() clientPolicy {
	policyMu.RLock()
	defer policyMu.RUnlock()
	return policyNow
}

// setPolicy nhận chính sách mới; trả về true nếu nó khác bản đang dùng.
func setPolicy(p clientPolicy) bool {
	p = normalizePolicy(p)
	policyMu.Lock()
	defer policyMu.Unlock()
	if p == policyNow {
		return false
	}
	policyNow = p
	return true
}

// Chính sách gần nhất được lưu cạnh .exe để vẫn áp được khi mất mạng ngay từ
// lúc khởi động. Thư mục cài chỉ dịch vụ nền ghi được.
func policyFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "policy.json"
	}
	return filepath.Join(filepath.Dir(exe), "policy.json")
}

func loadPolicyFile() {
	data, err := os.ReadFile(policyFilePath())
	if err != nil {
		return
	}
	var p clientPolicy
	if json.Unmarshal(data, &p) == nil {
		// Tệp do bản cũ ghi còn băm mật khẩu quản trị: ghi đè để xoá nó khỏi
		// đĩa. Giao diện chạy dưới tài khoản khách không ghi được thư mục cài,
		// nên chỉ dịch vụ nền làm được việc này — lỗi ghi bỏ qua.
		if p.LocalAdminUsername != "" || p.LocalAdminHash != "" {
			savePolicyFile(p)
		}
		p.LocalAdminUsername, p.LocalAdminHash = "", ""
		setPolicy(p)
	}
}

// savePolicyFile KHÔNG lưu tài khoản quản trị máy trạm: tệp này ai trên máy
// cũng đọc được, và khách có băm là dò mật khẩu ngoại tuyến được. Giao diện
// nhận tài khoản qua kênh pipe đã xác thực; mất mạng ngay lúc khởi động thì
// dùng tài khoản đặt lúc cài (config.json) cho tới nhịp tim đầu tiên.
func savePolicyFile(p clientPolicy) {
	p.LocalAdminUsername, p.LocalAdminHash = "", ""
	data, _ := json.Marshal(p)
	_ = os.WriteFile(policyFilePath(), data, 0o644)
}

// uiStatus là báo cáo giao diện gửi cho dịch vụ nền mỗi hai giây.
type uiStatus struct {
	Locked bool `json:"locked"` // đang ở màn hình khoá (đăng nhập, khoá từ quầy, mất kết nối)
	Member bool `json:"member"` // có hội viên đang đăng nhập
	Admin  bool `json:"admin"`  // nhân viên đang đăng nhập
	Maint  bool `json:"maint"`  // chế độ bảo trì (tài khoản quản trị máy trạm, tài khoản nhân viên ngoại tuyến)
}

// svcReply là câu trả lời của dịch vụ nền cho mỗi báo cáo.
type svcReply struct {
	OfflineLock bool         `json:"offline_lock"`
	Policy      clientPolicy `json:"policy"`
}

type superviseAction int

const (
	actNone superviseAction = iota
	actRestartUI
	actReboot
	actShutdown
)

const (
	// Giao diện báo cáo mỗi 2 giây; im quá mức này là treo hoặc bị dừng.
	uiSilentLimit = 15 * time.Second
	// Lần đầu sau khi bật, giao diện cần thời gian dựng cửa sổ.
	uiStartGrace = 30 * time.Second
	// Phiên kết thúc thì giao diện phải về màn hình khoá trong quãng này.
	sessionEndLockLimit = 10 * time.Second
	// Bật lại giao diện tới lần thứ ba trong quãng này thì khởi động lại máy.
	uiRestartWindow = 5 * time.Minute
	uiRestartMax    = 3
)

// uiSupervisor gom những gì dịch vụ nền biết về giao diện và về kết nối máy chủ.
type uiSupervisor struct {
	mu sync.Mutex

	lastServerOK time.Time // lần cuối gọi máy chủ thành công
	uiSince      time.Time // giao diện bắt đầu được thấy đang chạy; zero = không chạy
	lastStatusAt time.Time
	status       uiStatus

	sessionEndedAt time.Time // zero = không có phiên nào vừa kết thúc cần theo dõi
	lockedSinceEnd bool

	idleSince time.Time // màn hình khoá không ai đăng nhập từ lúc này; zero = đang có người

	restarts  []time.Time
	rebooting bool
}

func newUISupervisor(now time.Time) *uiSupervisor {
	// Coi lúc khởi động là "vừa nói chuyện được với máy chủ": mạng thường lên
	// sau dịch vụ vài giây, và đếm từ zero thì máy khoá ngay lúc vừa bật.
	return &uiSupervisor{lastServerOK: now}
}

func (s *uiSupervisor) noteServerOK(now time.Time) {
	s.mu.Lock()
	s.lastServerOK = now
	s.mu.Unlock()
}

func (s *uiSupervisor) noteStatus(now time.Time, st uiStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastStatusAt = now
	s.status = st
	if st.Locked && !s.sessionEndedAt.IsZero() {
		s.lockedSinceEnd = true
	}
	if st.Locked && !st.Member && !st.Admin && !st.Maint {
		if s.idleSince.IsZero() {
			s.idleSince = now
		}
	} else {
		s.idleSince = time.Time{}
	}
}

func (s *uiSupervisor) noteSessionEnded(now time.Time) {
	s.mu.Lock()
	s.sessionEndedAt = now
	s.lockedSinceEnd = false
	s.mu.Unlock()
}

// noteUIRunning ghi lại giao diện có đang chạy không. Không chạy thì xoá mọi
// thứ đã biết về nó: báo cáo cũ của một tiến trình đã chết không nói lên gì.
func (s *uiSupervisor) noteUIRunning(now time.Time, running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !running {
		s.uiSince = time.Time{}
		s.lastStatusAt = time.Time{}
		s.status = uiStatus{}
		s.idleSince = time.Time{}
		return
	}
	if s.uiSince.IsZero() {
		s.uiSince = now
	}
}

// offlineLock: đã mất kết nối máy chủ đủ lâu để phải khoá màn hình chưa.
func (s *uiSupervisor) offlineLock(now time.Time, pol clientPolicy) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return now.Sub(s.lastServerOK) >= time.Duration(pol.OfflineLockSeconds)*time.Second
}

// decide chấm một lượt. maintFlag là tệp maintenance.flag cạnh .exe.
//
// Như guard.go: chỉ hành động khi CHẮC. Không có giao diện, đang bảo trì, nhân
// viên đang đăng nhập — đều không làm gì. Thà bỏ sót còn hơn khởi động lại máy
// của người đang chơi thật hoặc đang sửa máy.
func (s *uiSupervisor) decide(now time.Time, pol clientPolicy, maintFlag bool) superviseAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rebooting || s.uiSince.IsZero() {
		return actNone
	}
	maint := maintFlag || s.status.Maint

	// 1. Giao diện im lặng: treo, hoặc bị dừng.
	ref, limit := s.lastStatusAt, uiSilentLimit
	if ref.IsZero() {
		ref, limit = s.uiSince, uiStartGrace
	}
	if now.Sub(ref) > limit {
		if maintFlag {
			return actNone
		}
		return s.restartOrRebootLocked(now)
	}

	// 2. Phiên đã kết thúc mà giao diện không về màn hình khoá.
	if !s.sessionEndedAt.IsZero() && now.Sub(s.sessionEndedAt) > sessionEndLockLimit {
		locked := s.lockedSinceEnd
		s.sessionEndedAt = time.Time{}
		if !locked && !s.status.Admin && !maint {
			return s.restartOrRebootLocked(now)
		}
	}

	// 3. Mất kết nối máy chủ quá lâu trong lúc có hội viên đăng nhập. Máy không
	// ai đăng nhập thì đã ở màn hình khoá, khởi động lại không được gì — và
	// tránh cả phòng máy khởi động lại vòng tròn khi máy chủ tắt qua đêm.
	if pol.OfflineRebootSeconds > 0 && s.status.Member && !s.status.Admin && !maint &&
		now.Sub(s.lastServerOK) >= time.Duration(pol.OfflineRebootSeconds)*time.Second {
		s.rebooting = true
		return actReboot
	}

	// 4. Máy sẵn sàng mà không ai đăng nhập quá lâu: tắt cho đỡ điện. Đồng hồ
	// chỉ chạy khi giao diện CHÍNH NÓ báo đang ở màn hình khoá và không có ai —
	// hội viên, nhân viên hay bảo trì đều dừng nó lại, và một lượt im lặng thì
	// đã bị luật 1 bắt trước khi tới đây.
	if pol.IdleShutdownMinutes > 0 && !maint && !s.idleSince.IsZero() &&
		now.Sub(s.idleSince) >= time.Duration(pol.IdleShutdownMinutes)*time.Minute {
		s.rebooting = true
		return actShutdown
	}
	return actNone
}

// restartOrRebootLocked: bật lại giao diện, nhưng tới lần thứ ba trong năm phút
// thì khởi động lại máy — bật lại mãi một giao diện cứ treo là vô ích.
func (s *uiSupervisor) restartOrRebootLocked(now time.Time) superviseAction {
	kept := s.restarts[:0]
	for _, t := range s.restarts {
		if now.Sub(t) < uiRestartWindow {
			kept = append(kept, t)
		}
	}
	s.restarts = append(kept, now)

	// Giao diện sắp bị tắt: quên mọi thứ về nó để lượt sau không chấm lại.
	s.uiSince = time.Time{}
	s.lastStatusAt = time.Time{}
	s.status = uiStatus{}
	s.sessionEndedAt = time.Time{}
	s.idleSince = time.Time{}

	if len(s.restarts) >= uiRestartMax {
		s.rebooting = true
		return actReboot
	}
	return actRestartUI
}
