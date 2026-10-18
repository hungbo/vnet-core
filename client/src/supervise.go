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
	// Thư mục (ổ đĩa tuyệt đối) chứa bản metadata Riot để chép vào
	// C:\ProgramData\Riot Games; xem riotmeta.go. Rỗng = tính năng tắt.
	RiotMetadataSource string `json:"riot_metadata_source,omitempty"`
	// Màn hình khoá không ai đăng nhập bấy nhiêu phút thì tắt máy; 0 = không.
	// Mặc định 0: chưa nghe máy chủ nói gì thì không tự tắt máy của ai cả.
	IdleShutdownMinutes int64 `json:"idle_shutdown_minutes"`
	// Hết giờ rảnh ở trên thì làm gì: "shutdown" (tắt máy) hoặc "standby" — máy
	// chưa ai đăng nhập từ lúc bật thì cho ngủ chờ, đã có người dùng thì khởi động
	// lại cho sạch (decide, luật 4). Máy chủ cũ không gửi trường này: tắt máy.
	IdleAction string `json:"idle_action"`
}

func defaultClientPolicy() clientPolicy {
	return clientPolicy{TamperAction: "restart", OfflineLockSeconds: 60, OfflineRebootSeconds: 300, IdleAction: "shutdown"}
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
	if p.IdleAction != "standby" {
		p.IdleAction = "shutdown"
	}
	// Danh sách lối tắt cần xoá quyết định tệp nào bị xoá khỏi desktop của khách:
	// ép về dạng chuẩn ngay cổng vào chứ không tin thẳng chuỗi nhận về.
	p.HiddenShortcuts = chuanHoaLoiTat(p.HiddenShortcuts)
	p.RiotMetadataSource = chuanHoaNguonRiot(p.RiotMetadataSource)
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

	// Các mốc khởi động giao diện đã ghi (bootreport.go); dịch vụ gộp vào báo cáo của nó.
	Boot *bootGUI `json:"boot,omitempty"`
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
	actSleep       // ngủ chờ (S3): máy chưa ai dùng từ lúc bật
	actIdleRestart // khởi động lại cho sạch rồi mới ngủ chờ: máy đã có người dùng
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
	// Máy vừa thức dậy (noteResume): khoá ngoại tuyến cho tới khi giao diện đã
	// được báo khoá ít nhất một lần VÀ máy chủ trả lời sau lần báo đó.
	choMayChu    bool
	khoaBaoLuc   time.Time // lần đầu báo khoá cho giao diện sau khi thức; zero = chưa báo
	uiSince      time.Time // giao diện bắt đầu được thấy đang chạy; zero = không chạy
	lastStatusAt time.Time
	status       uiStatus

	sessionEndedAt time.Time // zero = không có phiên nào vừa kết thúc cần theo dõi
	lockedSinceEnd bool

	idleSince time.Time // màn hình khoá không ai đăng nhập từ lúc này; zero = đang có người
	// daDung: từ lúc bật máy đã có người đăng nhập (hội viên, nhân viên hay bảo
	// trì). Máy như vậy còn dấu vết của người trước — tài khoản trình duyệt, game
	// đang mở — nên không được cho ngủ chờ, phải khởi động lại cho sạch.
	daDung bool

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

// noteResume: máy vừa thức dậy từ chế độ ngủ (onResume, service.go).
//
// Các mốc đo bằng supNow, đồng hồ không tính lúc ngủ, nên không cần đặt lại để
// tránh "quá hạn cả đêm". Hai việc:
//   - Khoá ngoại tuyến cho tới lần đầu máy chủ trả lời lại. Hội viên còn đăng
//     nhập từ trước lúc ngủ thì trong lúc ngủ máy chủ đã đóng phiên của họ (máy
//     im quá 2 phút); không khoá thì người bấm nút tiếp theo dùng luôn desktop
//     và tài khoản của họ cho tới khi giao diện tự hỏi lại. Máy chủ trả lời thì
//     giao diện mở khoá và hỏi lại phiên ngay (vnet:offline:cleared). Màn hình
//     khoá không ai đăng nhập thì giao diện bỏ qua khoá ngoại tuyến.
//   - Đếm lại "không ai đăng nhập" từ lúc thức: có người vừa bấm nút nguồn, cho
//     họ đủ thời gian đăng nhập chứ không chỉ phần còn lại từ trước lúc ngủ.
//   - Giao diện chưa báo cáo lần nào thì bật lại NGAY. Cloud Update cho máy ngủ
//     ngay khi vào tới desktop, tức đúng lúc giao diện đang dựng WebView2. Thử
//     thật trên MAY1 (15/10): WebView2 dựng dở qua giấc ngủ thì không bao giờ
//     xong — giao diện đứng ở màn khoá native, hết 30 giây ân hạn mới bị bật lại,
//     đúng lúc khách đang gõ. Bật lại ngay thì vài giây sau khi bấm nút đã có
//     màn khoá đầy đủ.
func (s *uiSupervisor) noteResume(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.choMayChu = true
	s.khoaBaoLuc = time.Time{}
	if !s.idleSince.IsZero() {
		s.idleSince = now
	}
	if !s.uiSince.IsZero() && s.lastStatusAt.IsZero() {
		s.uiSince = now.Add(-uiStartGrace - time.Second)
	}
}

// noteStatus ghi báo cáo của giao diện. Trả về true đúng lần đầu thấy có người
// đăng nhập kể từ lúc bật máy, để bên gọi ghi dấu ra đĩa (ghiDaDung).
func (s *uiSupervisor) noteStatus(now time.Time, st uiStatus) (moiDung bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastStatusAt = now
	s.status = st
	if (st.Member || st.Admin || st.Maint) && !s.daDung {
		s.daDung = true
		moiDung = true
	}
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
	return moiDung
}

// danhDauDaDung: lần bật máy này đã có người dùng — dịch vụ nền vừa khởi động lại
// giữa chừng và đọc được dấu đã ghi, hoặc không biết chắc (mayDaDung).
func (s *uiSupervisor) danhDauDaDung() {
	s.mu.Lock()
	s.daDung = true
	s.mu.Unlock()
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
		// Giao diện mất đi mà bộ giám sát không bảo nó tắt (restartOrRebootLocked xoá
		// uiSince trước khi tắt): bị ai đó tắt hoặc tự chết, và trong lúc đó desktop
		// lộ ra. Coi như máy đã có người dùng — đoán nhầm chỉ tốn một lần khởi động lại.
		if !s.uiSince.IsZero() {
			s.daDung = true
		}
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

// offlineLock: đã mất kết nối máy chủ đủ lâu để phải khoá màn hình chưa. Chỉ
// kênh nối với giao diện gọi hàm này, mỗi lần trả lời một báo cáo của nó.
func (s *uiSupervisor) offlineLock(now time.Time, pol clientPolicy) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.choMayChu {
		switch {
		case s.khoaBaoLuc.IsZero():
			// Lần báo đầu tiên sau khi thức luôn là khoá, kể cả khi nhịp tim đã kịp
			// thành công: giao diện phải thấy khoá một lần thì lúc mở khoá mới hỏi
			// lại phiên. Nhả ngay thì hội viên bỏ quên từ trước lúc ngủ vẫn còn đó.
			s.khoaBaoLuc = now
			return true
		case s.lastServerOK.After(s.khoaBaoLuc):
			s.choMayChu = false
		default:
			return true
		}
	}
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

	// Có tệp bảo trì là có người (kỹ thuật) dùng máy theo cách VNET không thấy: coi
	// như đã dùng, xoá tệp đi rồi cũng không được cho ngủ chờ.
	if maintFlag {
		s.daDung = true
	}
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
	//
	// Chế độ "standby" là cho Lightning Boot của Cloud Update. Tắt máy thì Cloud
	// Update chỉ đưa máy về ngủ chờ sau khoảng offline đặt trên Console, nên VNET tự
	// làm:
	//   - chưa ai đăng nhập từ lúc bật: máy vẫn sạch như vừa khởi động, cho ngủ
	//     luôn. Thử thật trên MAY1 (15/10): Cloud Update không đánh thức máy do
	//     Windows cho ngủ. Đồng hồ đếm lại: thức dậy mà vẫn không ai đăng nhập thì
	//     đủ thời gian nữa mới ngủ lại, và ngủ không được thì cũng chừng ấy nữa
	//     mới thử lại.
	//   - đã có người dùng: khởi động lại cho sạch. Lên lại là máy chưa ai dùng,
	//     đủ thời gian nữa vẫn không ai đăng nhập thì lượt sau cho ngủ.
	if pol.IdleShutdownMinutes > 0 && !maint && !s.idleSince.IsZero() &&
		now.Sub(s.idleSince) >= time.Duration(pol.IdleShutdownMinutes)*time.Minute {
		if pol.IdleAction == "standby" && !s.daDung {
			s.idleSince = now
			return actSleep
		}
		s.rebooting = true
		if pol.IdleAction == "standby" {
			return actIdleRestart
		}
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
