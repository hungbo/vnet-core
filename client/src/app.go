package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type LoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	MachineCode string `json:"machine_code,omitempty"`
}

type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	User         *UserInfo `json:"user"`
	SessionID    string    `json:"session_id,omitempty"`
}

// ClientLoginResponse là phản hồi của /api/auth/client-login: giống đăng nhập
// hội viên, thêm một trường cho biết ai vừa vào.
type ClientLoginResponse struct {
	LoginResponse
	Kind string `json:"kind"` // "staff" hoặc "member"
}

type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type OrderItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type OrderRequest struct {
	OrderType   string             `json:"order_type"`
	MemberID    string             `json:"member_id"`
	MachineCode string             `json:"machine_code"`
	Items       []OrderItemRequest `json:"items"`
}

type PaginatedData struct {
	Items    json.RawMessage `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type App struct {
	ctx         context.Context
	cfg         *Config
	token       string
	userID      string
	username    string
	fullName    string
	role        string
	machineCode string
	locker      *ScreenLocker
	// Đang ở chế độ bảo trì: mở bằng tài khoản quản trị máy trạm, không có phiên và không tính
	// tiền. Lớp chống phá đứng yên trong lúc này.
	baoTri atomic.Bool
	// verified: máy chủ đã xác nhận token đang giữ. Chỉ khi đó giao diện mới
	// được phép mở khoá — frontend xin mở mà chưa có xác nhận thì bị từ chối.
	verified atomic.Bool
	// Lý do phủ màn hình ngoài chuyện chưa đăng nhập: mất kết nối máy chủ quá
	// ngưỡng.
	offlineLocked atomic.Bool
	wsClient *WSClient
	wsCtx    context.Context
	wsCancel context.CancelFunc
	cancel   context.CancelFunc

	// windowMode rỗng nghĩa là thanh điều khiển chính; "order"/"support" là cửa
	// sổ phụ chạy trong tiến trình riêng. Frontend đọc giá trị này để render
	// thẳng màn hình tương ứng thay vì dựng cả thanh.
	windowMode string
	// Cửa sổ phụ dùng CHUNG một tiến trình cho mọi màn hình (gọi món, hỗ trợ,
	// nạp tiền, điểm danh, đánh giá): panelMode là màn hình đang hiện. Tiến
	// trình được dựng sẵn ẩn ngay khi khách đăng nhập, bấm nút chỉ đổi màn hình
	// rồi hiện lên — không phải chạy lại .exe và dựng WebView2 mỗi lần.
	panelMu   sync.Mutex
	panelMode string
	// thoat: cửa sổ phụ đang tắt thật (hết phiên). Bấm X thì chỉ ẩn.
	thoat atomic.Bool
	// prewarmRetry: số lần liên tiếp cửa sổ dựng sẵn chết lúc khởi động.
	prewarmRetry atomic.Int32

	// Màn khoá native (curtain*.go) giữ máy từ lúc thanh điều khiển bật tới lần mở
	// khoá đầu tiên. lockMu: chỗ nào suy ra "khoá hay mở" từ trạng thái, hoặc đổi
	// trạng thái rồi áp, đều giữ khoá này từ lúc đọc tới lúc áp xong — DomReady chạy
	// chồng với một lần đăng nhập mà không giữ là hook chặn phím ở lại trên desktop.
	lockMu sync.Mutex
	// dockHidden: cửa sổ Wails dựng ẩn sau màn khoá native và chưa hiện lần nào.
	// SetLoggedIn(true) hiện nó đúng một lần. Chỉ đọc ghi dưới lockMu.
	dockHidden bool
	// nativeDangGui: một lần Vào máy của màn khoá native đang gọi Login. Chỉ đọc ghi
	// dưới lockMu. Nó và một lần RestoreSession không bao giờ chạy chồng nhau: bên nào
	// tới trước thì bên kia nhường (nativeSubmit, RestoreSession).
	nativeDangGui bool
	// startupDone đóng ở cuối Startup. chinhSachSan đóng ở lượt trả lời đầu tiên của
	// dịch vụ nền, chậm nhất nativeChoChinhSach sau Startup. nativeSubmit chờ cả hai.
	startupDone  chan struct{}
	chinhSachSan chan struct{}
	chinhSachMot sync.Once
}

func NewApp() *App {
	cfg := LoadConfig()
	return &App{
		cfg:          cfg,
		machineCode:  cfg.MachineCode,
		locker:       NewScreenLocker(),
		startupDone:  make(chan struct{}),
		chinhSachSan: make(chan struct{}),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Cửa sổ phụ (gọi món, hỗ trợ) là một TIẾN TRÌNH RIÊNG dùng chung struct
	// này. Không chặn ở đây thì mở thực đơn là dựng thêm một bộ vòng lặp nền
	// nữa: nhịp tim gửi đôi, tệp hosts bị hai nơi cùng ghi, và cửa sổ gọi món
	// tự phủ kín màn hình vì tưởng chưa ai đăng nhập.
	if a.windowMode != "" {
		return
	}

	// Bản cũ để lại băm mật khẩu nhân viên trong thư mục của khách: xoá.
	purgeCredCache()

	// Mốc đo khởi động: Wails gọi OnStartup ngay sau Navigate, nên tới đây WebView2 đã
	// dựng xong và trang bắt đầu nạp. Khúc từ đây tới "DOM sẵn sàng" là phần của trang.
	log.Printf("[ui] Startup sau %dms (WebView2 đã dựng, trang đang nạp)", time.Since(processStart).Milliseconds())
	bootRec.webViewSan(systemUptime())
	// Trang tự gửi các mốc nạp của nó lúc nạp xong (main.ts).
	wailsruntime.EventsOn(ctx, "vnet:ui:moc", ghiMocNapTrang)

	aCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	go runTelemetry(aCtx, a.cfg)
	go runWatchdog(aCtx, a.cfg, a.locker, &[]string{})
	go newWebBlocker(a.cfg).run(aCtx)
	// Lớp canh dịch vụ nền chạy Ở ĐÂY chứ không ở tiến trình dịch vụ: dịch vụ
	// đã canh giao diện, đây là chiều còn lại.
	go runGuard(aCtx, a.cfg, a.baoTri.Load)
	// Chính sách gần nhất dịch vụ nền đã lưu, rồi kênh báo cáo với dịch vụ.
	loadPolicyFile()
	go runUILinkClient(aCtx, a)
	// Màn hình khoá phải phủ kín cả khi game nhả chế độ màn hình riêng của nó (display_app.go).
	go a.giuManHinhKhoaPhuKin(aCtx)

	// Chưa ai đăng nhập thì máy phải bị khoá NGAY, trước cả khi frontend kịp
	// dựng xong. Đây là mặc định an toàn: mọi đường dẫn tới màn hình desktop
	// đều phải đi qua một lần đăng nhập thành công.
	//
	// Màn khoá native quá nativeLockReadyWait mà vẫn đang dựng (choNativeLockHien) thì
	// tới đây thôi chờ nó: từ giờ cửa sổ Wails phủ màn hình, cửa sổ native xong muộn hơn
	// tự huỷ. dockHidden chỉ đọc ghi dưới lockMu.
	a.lockMu.Lock()
	if a.dockHidden && nativeLockAbandon() {
		log.Printf("[ui] tới Startup màn khoá native vẫn chưa hiện — dùng màn hình khoá web")
		a.dockHidden = false
	}
	a.applyLoginLock(true)
	a.lockMu.Unlock()

	// Ô đăng nhập native chờ tới đây mới gọi Login (nativeSubmit).
	time.AfterFunc(nativeChoChinhSach, a.coChinhSach)
	close(a.startupDone)
}

// applyLoginLock chuyển cửa sổ giữa hai hình dạng.
//
// khoá: phủ kín màn hình, luôn nổi, chặn phím thoát — chỉ còn ô đăng nhập.
// mở:  thu về thanh dọc mép phải như thường.
//
// Đây KHÔNG dính gì tới đăng nhập/khoá máy của Windows. Quán thường để Windows
// tự đăng nhập vào một tài khoản dùng chung, nên khoá của Windows không bảo vệ
// được gì; lớp khoá này mới là thứ quyết định ai được dùng máy.
func (a *App) applyLoginLock(khoa bool) {
	// Cửa sổ phụ (gọi món, hỗ trợ) là tiến trình RIÊNG nhưng dùng chung struct
	// này, và hình dạng của nó do canhGiuaManHinh quyết định. Không chặn ở đây
	// thì frontend gọi SetLoggedIn lúc khôi phục phiên là dockToRightEdge kéo
	// cửa sổ vừa căn giữa xong về đúng chỗ thanh điều khiển — thực đơn hiện ra
	// giữa màn hình rồi nhảy vào góc phải, rộng 360px.
	//
	// Nó còn bật AlwaysOnTop vĩnh viễn cho cửa sổ phụ, đúng thứ mà ShowWindow
	// cố tình bật-rồi-tắt để cửa sổ hỗ trợ không che game mãi.
	if a.windowMode != "" || a.ctx == nil {
		return
	}
	if khoa {
		// Trả độ phân giải và chuột về mức ban đầu TRƯỚC khi phủ màn hình: người ngồi
		// sau không thừa hưởng cài đặt của khách trước, và màn hình khoá phải đo theo
		// chế độ cuối cùng chứ không phải chế độ khách đã chọn.
		a.resetDisplayForNextUser()
		// Màn khoá native còn giữ máy thì nó đã tự phủ kín, tự nổi trên cùng và tự giữ
		// tiêu điểm. Cửa sổ Wails phải nằm yên phía sau: hiện nó lúc này là thêm một ô
		// đăng nhập thứ hai giành phím với ô của khách.
		if !nativeLockOwns() {
			wailsruntime.WindowShow(a.ctx)
			wailsruntime.WindowUnminimise(a.ctx)
			wailsruntime.WindowFullscreen(a.ctx)
			wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
		}
		if err := a.locker.Lock(); err != nil {
			log.Printf("[login] đã phủ màn hình nhưng không chặn được phím: %v", err)
		}
		return
	}

	// Gỡ hook chặn phím: đăng nhập xong khách dùng máy bình thường, Alt+Tab giữa
	// game với Discord/trình duyệt, Alt+F4 tắt game. Hook chỉ để màn hình khoá
	// không bị thoát ra; về lại màn hình khoá thì nhánh trên bật lại.
	a.locker.Unlock()
	wailsruntime.WindowUnfullscreen(a.ctx)
	dockToRightEdge(a.ctx)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	// Lần mở khoá đầu tiên kể từ khi giao diện bật: hạ màn khoá native SAU CÙNG. Cửa sổ
	// đang có bàn phím mà biến mất thì Windows trao bàn phím cho cửa sổ kế bên dưới, nên
	// thanh điều khiển SetLoggedIn vừa hiện phải lên xong trước: WindowShow chỉ xếp việc
	// vào hàng của luồng giao diện Wails, còn dockToRightEdge ở trên hỏi màn hình qua
	// đúng hàng đó và chờ trả lời. Đã hạ rồi (hoặc chưa từng dựng) thì không làm gì.
	nativeLockClose()
	// Mốc mở khoá đầu tiên của lần bật máy (bootreport.go).
	bootRec.moKhoa(systemUptime())
}

// ApplyWindowState đặt lại hình dạng cửa sổ theo trạng thái hiện tại.
//
// Gọi từ OnDomReady. Wails chạy OnStartup trong goroutine song song với
// f.WindowCenter() của chính nó, nên lần đặt vị trí trong Startup có thể bị ghi
// đè; đây là lần chốt hạ, chạy ngay trước khi cửa sổ được hiện lên.
//
// Màn khoá native KHÔNG bao giờ hạ ở đây, chỉ ở một lần mở khoá thật (nativeSubmit,
// SetLoggedIn(true), hai đường bảo trì): RestoreSession gán token trước khi giao diện
// hỏi xong phiên còn hay không, và DomReady rơi đúng quãng đó là hạ màn khoá cho một
// phiên có thể đã hết. Chế độ bảo trì cũng là đã mở: kỹ thuật viên mở khoá ở màn khoá
// native trước DomReady thì DomReady không được khoá lại máy.
func (a *App) ApplyWindowState() {
	if a.windowMode != "" || a.ctx == nil {
		return
	}
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	a.applyLoginLock(khoaKhiApTrangThai(nativeLockOwns(), a.loggedIn(), a.baoTri.Load()))
}

// khoaKhiApTrangThai: ApplyWindowState khoá hay mở. Hàm thuần để kiểm bằng bảng
// (app_nativelock_test.go): màn khoá native còn giữ thì luôn khoá; không thì khoá khi
// chưa ai đăng nhập, trừ lúc đang bảo trì.
func khoaKhiApTrangThai(nativeGiu, dangNhap, baoTri bool) bool {
	return nativeGiu || (!dangNhap && !baoTri)
}

// loggedIn suy ra từ token: frontend gọi SetLoggedIn mỗi lần trạng thái đổi,
// nhưng OnDomReady có thể chạy TRƯỚC lần gọi đầu tiên đó.
func (a *App) loggedIn() bool {
	return a.token != ""
}

// SetLoggedIn được frontend gọi mỗi khi trạng thái đăng nhập đổi: đăng nhập
// xong, đăng xuất, hoặc khôi phục phiên cũ lúc khởi động.
func (a *App) SetLoggedIn(loggedIn bool) {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	// Frontend chỉ ĐỀ NGHỊ; mở khoá hay không do Go quyết theo xác nhận của máy
	// chủ. Không có chốt này thì bất cứ thứ gì chạy được trong giao diện cũng
	// gọi được hàm này để mở máy.
	if loggedIn && !a.verified.Load() {
		log.Printf("[login] từ chối mở khoá: máy chủ chưa xác nhận phiên đăng nhập")
		a.applyLoginLock(true)
		return
	}
	if loggedIn {
		a.baoTri.Store(false)
	}
	// Thanh điều khiển dựng ẩn sau màn khoá native: hiện nó ở đây, khi giao diện đã
	// ở trạng thái đăng nhập — không bao giờ là một màn hình khoá bị bóp vào 360px.
	// Hiện TRƯỚC khi màn khoá native đóng (applyLoginLock(false) đóng nó sau cùng) để
	// bàn phím sang thanh điều khiển chứ không sang game hay Discord phía sau: phiên
	// khôi phục xong lúc khách còn gõ dở trên màn khoá thì phần còn lại không được rơi
	// vào ô chat của game. Màn khoá thôi giữ khoá trước, không thì nhịp 150 ms của nó
	// giành bàn phím lại; thanh vào mép phải trước khi hiện, không thì nó chớp giữa
	// màn hình.
	if loggedIn && a.dockHidden && a.ctx != nil {
		a.dockHidden = false
		nativeLockRelease()
		dockToRightEdge(a.ctx)
		wailsruntime.WindowShow(a.ctx)
	}
	a.applyLoginLock(!loggedIn)
}

// unlockLocalAdmin mở khoá máy bằng tài khoản quản trị máy trạm, KHÔNG hỏi
// máy chủ.
//
// Đây là đường vào chắc chắn có khi mất mạng: mọi cách đăng nhập khác đều gọi
// API, nên router hỏng là cả phòng máy đứng trước màn hình khoá phủ kín.
//
// Mở kiểu này KHÔNG mở phiên và KHÔNG tính tiền: nó dành cho nhân viên kỹ thuật,
// không phải cho khách. Lớp chống phá cũng đứng yên, vì việc đầu tiên người sửa
// máy làm thường là tắt dịch vụ.
func (a *App) unlockLocalAdmin(username string) {
	log.Printf("[bảo trì] mở khoá bằng tài khoản quản trị máy trạm %q", username)
	a.lockMu.Lock()
	a.baoTri.Store(true)
	a.locker.Unlock()
	a.applyLoginLock(false)
	a.lockMu.Unlock()
	wailsruntime.WindowMinimise(a.ctx)
}

func (a *App) GetServerURL() string {
	if a.cfg.ServerURL == "" {
		return "http://localhost:20800"
	}
	return a.cfg.ServerURL
}

// loiTraLoiLa: máy chủ có trả lời nhưng không theo dạng của máy chủ VNET.
const loiTraLoiLa = "máy chủ trả lời không hợp lệ, có thể sai địa chỉ máy chủ — vui lòng báo nhân viên"

func (a *App) doRequest(method, path string, body interface{}) (json.RawMessage, error) {
	return a.doRequestWith(a.token, uiTimeout, method, path, body)
}

// doRequestWith là doRequest với token và hạn chót do người gọi chọn.
// RestoreSession hỏi bằng một token CHƯA được tin nên không thể gán nó vào
// a.token trước; Logout cần hạn chót ngắn hơn lời gọi thường.
func (a *App) doRequestWith(token string, timeout time.Duration, method, path string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	fullURL := a.GetServerURL() + path
	log.Printf("[HTTP] %s %s body=%s", method, fullURL, a.safeBody(body))

	// Hạn chót gắn vào context để bao cả phần đọc thân trả lời: máy chủ gửi
	// header rồi treo giữa chừng cũng không giữ được lời gọi.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		log.Printf("[HTTP] new request error: %v", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[HTTP] do error: %v", err)
		// Bọc bằng sentinel: "không với tới máy chủ" và "sai mật khẩu" phải
		// phân biệt được, vì chỉ trường hợp đầu mới được rơi sang bản đệm
		// offline. Nhầm hai cái là sai mật khẩu cũng mở được máy.
		return nil, fmt.Errorf("%w: %v", errServerUnreachable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[HTTP] read error: %v", err)
		return nil, fmt.Errorf("%w: %v", errServerUnreachable, err)
	}
	// Thân trả lời KHÔNG được ghi: ui.log nằm trong thư mục của tài khoản Windows của
	// khách, khách sau ngồi cùng máy đọc được. Trả lời đăng nhập mang access/refresh
	// token (dùng được phiên và số dư của người trước), các trả lời khác mang thông tin
	// hội viên và nội dung chat. Lỗi thì logBody ghi bản đã che, ở hai nhánh dưới.
	log.Printf("[HTTP] %s %s → %d (%d byte)", method, fullURL, resp.StatusCode, len(bodyBytes))

	var apiResp APIResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		log.Printf("[HTTP] parse error: %v body=%s", err, logBody(bodyBytes))
		// Máy chủ VNET luôn trả JSON, kể cả "sai mật khẩu", nên trả lời không đọc được
		// nghĩa là thứ đang trả lời không phải nó (địa chỉ trỏ nhầm sang router, IIS,
		// cổng đăng nhập wifi; proxy báo 502): với Login cũng như không với tới máy chủ, và
		// bản lưu nhân viên vẫn được thử — mật khẩu của nó vẫn kiểm tại máy. Câu lỗi
		// hiện thẳng cho khách nên phải là tiếng Việt.
		return nil, fmt.Errorf("%w: %s", errServerUnreachable, loiTraLoiLa)
	}

	if apiResp.Code != 0 {
		log.Printf("[HTTP] api error: code=%d msg=%s", apiResp.Code, apiResp.Message)
		return nil, fmt.Errorf("%s", apiResp.Message)
	}

	return apiResp.Data, nil
}

func (a *App) safeBody(body interface{}) string {
	if body == nil {
		return ""
	}
	b, _ := json.Marshal(body)
	// ui.log nằm trong thư mục của tài khoản Windows đang ngồi máy — tài khoản
	// của KHÁCH. Ghi nguyên thân yêu cầu là khách sau đọc được mật khẩu khách
	// trước, kể cả mật khẩu nhân viên.
	var m map[string]interface{}
	if json.Unmarshal(b, &m) == nil {
		for k := range m {
			if isSecretField(k) || isPrivateText(k) {
				m[k] = "***"
			}
		}
		b, _ = json.Marshal(m)
	}
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// isSecretField: trường không bao giờ được ghi ra nhật ký.
func isSecretField(key string) bool {
	k := strings.ToLower(key)
	if k == "machine_code" {
		return false
	}
	for _, w := range []string{"password", "pin", "token", "secret", "code"} {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}

func (a *App) connectWS() {
	if a.wsCancel != nil {
		a.wsCancel()
	}
	a.wsCtx, a.wsCancel = context.WithCancel(context.Background())
	a.wsClient = NewWSClient(a.ctx, a.GetServerURL(), a.token, a.machineCode)
	// Lệnh điều khiển từ quầy (khoá, nhắn tin, chụp màn hình, tiến trình...)
	// chỉ thanh chính xử lý. Cửa sổ phụ dựng sẵn sống suốt phiên và cũng nối
	// WebSocket (cần cho chat, số dư); để nó xử lý nữa là mỗi lệnh chạy HAI
	// lần — khách thấy hai hộp thông báo, quầy nhận hai ảnh chụp.
	if a.windowMode == "" {
		a.registerRemoteHandlers(a.wsClient)
		a.khoaKhiHetPhienChuaNhan(a.wsClient)
	}
	go func() {
		if err := a.wsClient.Connect(a.wsCtx); err != nil {
			log.Printf("ws client: %v", err)
		}
	}()
}

// khoaKhiHetPhienChuaNhan: phiên trên máy này kết thúc (session:ended,
// session:auto-ended, curfew:enforced) trong lúc giao diện web chưa nhận lần đăng nhập
// ở màn khoá native — trang còn nạp sau lưng, hoặc trình hiển thị WebView2 đã chết. ws.go
// chỉ báo cho trang, mà trang chưa đăng nhập thì bỏ qua sự kiện: không có gì ở đây là
// desktop mở cho một máy không còn phiên, tới khi dịch vụ nền bật lại giao diện. Trang
// vẫn nhận sự kiện như cũ (để hiện lý do khoá); Go tự đăng xuất, như trang sẽ làm
// (clearSession → logout).
func (a *App) khoaKhiHetPhienChuaNhan(c *WSClient) {
	for _, loai := range []string{"session:ended", "session:auto-ended", "curfew:enforced"} {
		baoTrang := c.handlers[loai]
		c.On(loai, func(msg WSMessage) {
			if baoTrang != nil {
				baoTrang(msg)
			}
			if a.hetPhienTruocKhiTrangNhan(msg) {
				log.Printf("[login] phiên kết thúc trước khi giao diện nhận lần đăng nhập ở màn khoá native — khoá máy")
				a.Logout()
			}
		})
	}
}

// hetPhienTruocKhiTrangNhan: sự kiện hết phiên này là của hội viên Go đang giữ, và giao
// diện chưa nhận lần đăng nhập đó — dockHidden: thanh điều khiển chưa hiện lần nào, mà
// SetLoggedIn(true) của trang mới hiện nó. Màn khoá native còn giữ khoá thì lần đăng
// nhập là một lần RestoreSession trang đang tự lo (hỏi phiên còn không), không phải của
// màn khoá native. Lọc như clearSession của trang: sự kiện ghi hội viên khác, hoặc nhân
// viên đang đăng nhập, thì bỏ qua.
func (a *App) hetPhienTruocKhiTrangNhan(msg WSMessage) bool {
	var d struct {
		MemberID string `json:"member_id"`
	}
	_ = json.Unmarshal(msg.Payload, &d) // hỏng thì như trang: coi như không ghi hội viên nào
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	return a.dockHidden && !nativeLockOwns() && a.loggedIn() && a.role != "admin" &&
		(d.MemberID == "" || d.MemberID == a.userID)
}

// Login là đường đăng nhập DUY NHẤT của máy trạm.
//
// Trước đây có hai binding và màn hình khoá có hai ô: một ô cho hội viên, một
// nút "Đăng nhập quản trị" nằm khuất bên dưới. Nhân viên phải nhớ mình thuộc
// loại nào trước khi gõ, mà gõ nhầm ô thì máy chủ báo "sai mật khẩu" — sai chỗ
// để đi tìm. Giờ máy chủ nhìn tên tài khoản rồi tự chọn đường.
//
// Nhân viên đăng nhập thì KHÔNG mở phiên và không tính tiền: họ vào để trông
// máy, không phải để chơi.
func (a *App) Login(username, password string) (string, error) {
	// Tài khoản quản trị máy trạm kiểm ngay tại máy, trước khi hỏi máy chủ:
	// nó phải mở được máy cả lúc mất mạng. Sai mật khẩu thì vẫn đi tiếp lên
	// máy chủ — có thể đó là tài khoản nhân viên trùng tên.
	if _, dung := verifyLocalAdmin(username, password); dung && a.windowMode == "" {
		a.unlockLocalAdmin(strings.TrimSpace(username))
		return `{"offline":true,"local_admin":true}`, nil
	}

	req := LoginRequest{
		Username:    username,
		Password:    password,
		MachineCode: a.machineCode,
	}

	data, err := a.doRequest("POST", "/api/auth/client-login", req)
	if err != nil {
		// Không có đường mở máy offline nào khác ngoài tài khoản quản trị máy trạm,
		// đã kiểm ở trên (xem credcache.go: vì sao bỏ bản đệm nhân viên).
		if errors.Is(err, errServerUnreachable) {
			time.Sleep(time.Second)
			return "", errors.New("Không kết nối được máy chủ. Khi mất mạng chỉ tài khoản quản trị máy trạm mở được máy")
		}
		return "", err
	}

	var resp ClientLoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", errors.New(loiTraLoiLa)
	}
	if resp.User == nil {
		return "", fmt.Errorf("máy chủ trả về phản hồi thiếu thông tin tài khoản")
	}

	a.token = resp.AccessToken
	a.userID = resp.User.ID
	a.username = resp.User.Username
	a.fullName = resp.User.FullName
	// Nhân viên mang vai trò "owner"/"manager"/"staff" trong bảng users, nhưng
	// giao diện máy trạm chỉ có ba loại màn hình. Quy về 'admin' NGAY TẠI ĐÂY,
	// vì GetUserInfo là thứ Dashboard đọc lúc dựng màn hình — để nguyên thì chủ
	// quán đăng nhập lại thấy màn hình của khách, kèm nút Nạp tiền và Điểm danh.
	a.role = resp.User.Role
	if resp.Kind == "staff" {
		a.role = "admin"
	}

	a.verified.Store(true)
	a.connectWS()
	a.refreshGameShortcuts()

	return string(data), nil
}

// nativeChoChinhSach: ô đăng nhập native chờ lượt trả lời đầu tiên của dịch vụ nền
// tối đa chừng này sau Startup. Tài khoản quản trị máy trạm đặt ở trang quản trị chỉ
// tới qua lượt đó (policy.json cố ý không giữ nó); gọi Login sớm hơn là kỹ thuật viên
// gõ đúng tài khoản mà máy đi hỏi máy chủ rồi báo sai mật khẩu.
const nativeChoChinhSach = 2 * time.Second

// coChinhSach mở cửa cho nativeSubmit: dịch vụ nền đã trả lời, hoặc đã chờ đủ.
func (a *App) coChinhSach() {
	a.chinhSachMot.Do(func() { close(a.chinhSachSan) })
}

// nativeSubmit là nút "Vào máy" của màn khoá native (curtain*.go), chạy trên goroutine
// riêng của nó. Đi đúng đường Login của màn hình khoá web: nhân viên hay hội viên, tài
// khoản quản trị máy trạm, bản lưu nhân viên khi mất mạng, câu lỗi của máy chủ — tất cả
// nằm ở Login, không chép lại ở đây.
//
// Chờ Startup trước: Login cần a.ctx (WebSocket, thu nhỏ cửa sổ), và kênh báo cáo với
// dịch vụ nền phải đang chạy. Cái giá là vài giây cho người bấm Vào máy ngay lúc máy
// vừa lên — trước đây họ chờ màn hình khoá web còn lâu hơn.
//
// Trả nil là máy đã mở khoá VÀ màn khoá native đã hạ (applyLoginLock(false) hạ nó). Trả
// lỗi thì màn khoá native hiện nguyên văn câu lỗi và nhận lần bấm kế.
func (a *App) nativeSubmit(username, password string) error {
	<-a.startupDone
	<-a.chinhSachSan
	// Giao diện web đang khôi phục phiên cũ (giao diện bật lại giữa buổi): phiên đó
	// thắng. Login không từ chối khi máy đã có token, nên cho chạy song song là hai lần
	// đăng nhập giành một máy — Go giữ người này, giao diện hiện người kia.
	a.lockMu.Lock()
	if nativeLockDangKhoiPhuc() {
		a.lockMu.Unlock()
		return errors.New("Máy đang khôi phục phiên trước, vui lòng chờ giây lát")
	}
	a.nativeDangGui = true
	a.lockMu.Unlock()

	_, err := a.Login(username, password)

	a.lockMu.Lock()
	a.nativeDangGui = false
	var khoa, mo bool
	khoa, mo, err = sauVaoMayNative(err, a.baoTri.Load(), a.loggedIn(), a.verified.Load())
	if khoa {
		a.applyLoginLock(true)
	}
	if mo {
		a.applyLoginLock(false)
	}
	a.lockMu.Unlock()
	if mo {
		// Desktop dùng được ngay; thanh điều khiển hiện khi giao diện nhận lần đăng nhập
		// này (CurrentLogin). Trang chưa nạp xong thì tiếng gõ cửa rơi mất, không sao: nó
		// tự hỏi lúc dựng.
		wailsruntime.EventsEmit(a.ctx, "vnet:login:native")
	}
	return err
}

// sauVaoMayNative: Login của màn khoá native đã trả về (loiLogin), máy phải về thế nào.
// Đọc dưới lockMu. Hàm thuần để kiểm bằng bảng (app_nativelock_test.go).
//
//	khoa: khoá lại (applyLoginLock(true)).
//	mo:   mở máy (applyLoginLock(false), hạ màn khoá native) rồi gõ cửa giao diện.
//	err:  trả cho màn khoá native; nil là máy đã mở.
func sauVaoMayNative(loiLogin error, baoTri, dangNhap, daXacNhan bool) (khoa, mo bool, err error) {
	switch {
	case loiLogin != nil:
		return false, false, loiLogin
	case baoTri:
		// Tài khoản quản trị máy trạm và bản lưu nhân viên đã tự mở khoá bên trong Login.
		return false, false, nil
	case !dangNhap || !daXacNhan:
		// Một lần đăng xuất chạy chồng (Logout không giữ lockMu lúc gọi máy chủ) đã dọn
		// mất lần đăng nhập vừa xong: không mở máy cho một máy không còn ai đăng nhập.
		return true, false, errors.New("Đăng nhập chưa xong, vui lòng thử lại")
	}
	return false, true, nil
}

func (a *App) Logout() error {
	// Cửa sổ phụ không có đăng xuất của riêng nó: phiên hết thì nó đóng lại.
	// Không chặn ở đây thì cửa sổ thực đơn cũng gọi trả máy thay cho khách.
	if a.windowMode != "" {
		if a.ctx != nil {
			a.thoat.Store(true)
			wailsruntime.Quit(a.ctx)
		}
		return nil
	}

	// Đăng xuất của HỘI VIÊN là trả máy. Bản cũ chỉ xoá token cục bộ nên phiên
	// trên máy chủ chạy tiếp: khách về rồi vẫn bị trừ tiền từng phút, và máy
	// kẹt ở "đang dùng" nên người sau không mở được. Máy chủ treo thì hết hạn
	// chót cũng đi tiếp — dọn trạng thái cục bộ bên dưới KHÔNG được chờ mãi.
	// Không với tới máy chủ thì phiên VẪN cần được trả: máy gửi nhịp tim đều nên
	// máy chủ không coi là mất máy. Giữ bản sao token để thử lại ở nền (traMayONen)
	// sau khi trạng thái cục bộ đã dọn xong.
	chuaTra := ""
	if a.token != "" && a.role != "admin" {
		if _, err := a.doRequestWith(a.token, logoutTimeout, "POST", "/api/sessions/me/end", nil); err != nil {
			log.Printf("[logout] không trả được máy trên máy chủ: %v", err)
			if errors.Is(err, errServerUnreachable) {
				chuaTra = a.token
			}
		}
	}
	a.closeChildWindows()
	a.resetDisplayForNextUser()

	// Hook chặn phím GIỮ NGUYÊN: đăng xuất là quay về màn hình khoá, và màn
	// hình khoá cần chính cái hook đó. Bản cũ gỡ hook ở đây, nên từ lần đăng
	// xuất đầu tiên trở đi màn hình khoá không còn chặn được Alt+Tab nữa.
	if a.baoTri.Load() {
		a.locker.Unlock()
	}
	a.verified.Store(false)
	a.token = ""
	a.userID = ""
	a.username = ""
	a.fullName = ""
	a.role = ""
	if a.wsCancel != nil {
		a.wsCancel()
	}
	// Khoá lại dưới lockMu SAU khi đã dọn trạng thái: lần mở khoá nào chen vào giữa lúc
	// đọc token ở trên và lúc dọn xong (Vào máy ở màn khoá native, giao diện nhận lần
	// đăng nhập Go đang giữ) cũng không được để desktop mở cho một máy không còn ai đăng
	// nhập. Phiên khôi phục dở (nếu có) cũng hết ở đây: màn khoá native nhận Vào máy lại.
	a.lockMu.Lock()
	nativeLockRestoring(false)
	if !a.baoTri.Load() {
		a.applyLoginLock(true)
	}
	a.lockMu.Unlock()
	if chuaTra != "" {
		go a.traMayONen(chuaTra, lichTraMay{endRetryBackoff, time.Now().Add(endRetryFor), logoutTimeout})
	}
	return nil
}

// lichTraMay là lịch thử lại việc trả máy, chốt lúc Logout để vòng nền không đọc biến toàn cục.
type lichTraMay struct {
	buoc    []time.Duration // chờ trước mỗi lần thử; phần tử cuối lặp mãi
	hetHan  time.Time       // quá mốc này thì bỏ cuộc
	hanChot time.Duration   // hạn chót mỗi lời gọi
}

// traMayONen thử lại việc trả máy mà Logout không với tới máy chủ được. Chạy sau khi
// trạng thái cục bộ đã dọn nên dùng BẢN SAO token và không đụng tới a.token. Dừng khi
// máy chủ đã trả lời (xong, hết phiên, token hết hạn...), khi khách vừa đăng nhập lại,
// hoặc quá lich.hetHan.
func (a *App) traMayONen(token string, lich lichTraMay) {
	for i := 0; time.Now().Before(lich.hetHan); i++ {
		time.Sleep(lich.buoc[min(i, len(lich.buoc)-1)])
		// Hội viên đã đăng nhập lại (cùng người thì máy chủ nối lại ĐÚNG phiên cũ): kết thúc nó bây giờ là
		// đuổi họ khỏi máy giữa chừng. Nhân viên vào xem máy thì không ảnh hưởng gì.
		if a.loggedIn() && a.role != "admin" {
			return
		}
		if a.traMayMotLan(token, lich.hanChot) {
			return
		}
	}
}

// traMayMotLan thử trả máy một lần; true là dừng hẳn. /sessions/me/end kết thúc phiên đang chạy của
// hội viên Ở BẤT KỲ MÁY NÀO, mà sau vài phút phiên cũ có thể đã được máy chủ tự đóng và hội viên
// đang chơi ở máy khác — nên hỏi trước và chỉ kết thúc phiên nằm đúng trên máy này.
func (a *App) traMayMotLan(token string, hanChot time.Duration) bool {
	data, err := a.doRequestWith(token, hanChot, "GET", "/api/sessions/me", nil)
	if err != nil {
		return !errors.Is(err, errServerUnreachable)
	}
	var phien struct {
		MachineCode string `json:"machine_code"`
	}
	if json.Unmarshal(data, &phien) != nil || phien.MachineCode != a.machineCode {
		return true // hết phiên (null) hoặc phiên đã sang máy khác
	}
	_, err = a.doRequestWith(token, hanChot, "POST", "/api/sessions/me/end", nil)
	return !errors.Is(err, errServerUnreachable)
}

// errDangVaoMayNative: RestoreSession lui vì một lần Vào máy ở màn khoá native đang gọi
// máy chủ. Khác "đã đăng nhập bằng cách khác": lần đó còn có thể hỏng (sai mật khẩu),
// và lúc đó phiên cũ vẫn phải khôi phục được. session.store.ts (restore) nhận ra đúng
// câu này để GIỮ phiên đã lưu và hỏi lại — đổi câu thì đổi cả bên đó.
var errDangVaoMayNative = errors.New("đang vào máy ở màn khoá native, khôi phục phiên sau")

// RestoreSession khôi phục đăng nhập sau khi giao diện tự khởi động lại.
//
// Chỉ nhận token; MỌI thứ khác (ai, vai trò gì) hỏi lại máy chủ. Bản cũ nhận cả
// vai trò từ nơi frontend tự lưu và không hỏi ai: sửa giá trị lưu đó thành
// nhân viên là có một máy mở khoá, không phiên, không tính tiền. Không hỏi được
// máy chủ thì không khôi phục — máy ở lại màn hình khoá, an toàn hơn là mở.
//
// Token CHỈ được gán vào a.token SAU khi máy chủ xác nhận. Giao diện chạy hàm
// này ở nền (màn hình khoá hiện ngay, không chờ), nên trong lúc máy chủ chưa
// trả lời người ở máy có thể đăng nhập tay: thất bại hay hết hạn chót ở đây mà
// động tới a.token là xoá mất phiên vừa đăng nhập của họ — và giữ token của
// khách trước trong lúc chưa ai xác nhận nó thì làm máy trông như đã mở.
func (a *App) RestoreSession(token string) (string, error) {
	// Màn khoá native còn giữ máy thì báo "đang khôi phục" trên nó và khoá nút Vào máy
	// cho tới khi xong: SetLoggedIn(true) hạ nó, còn Logout hay thất bại dưới đây thì mở
	// lại nút. Khách đã bấm Vào máy trước thì lần đó đi trước: lui bằng
	// errDangVaoMayNative để giao diện giữ phiên đã lưu và hỏi lại khi lần đó xong.
	a.lockMu.Lock()
	if a.nativeDangGui {
		a.lockMu.Unlock()
		return "", errDangVaoMayNative
	}
	nativeLockRestoring(true)
	a.lockMu.Unlock()
	xong := false
	defer func() {
		if !xong {
			nativeLockRestoring(false)
		}
	}()

	data, err := a.doRequestWith(token, uiTimeout, "GET", "/api/auth/me", nil)
	if err != nil {
		return "", err
	}
	var me struct {
		UserInfo
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &me); err != nil || me.ID == "" || me.Kind == "" {
		return "", errors.New("máy chủ không xác nhận được phiên đăng nhập")
	}
	// Máy chủ trả lời chậm, người ở máy đã tự vào (đăng nhập, hoặc mở khoá bảo
	// trì) trong lúc chờ: phiên của họ thắng, phiên cũ bị bỏ.
	if a.token != "" || a.baoTri.Load() {
		return "", errors.New("máy đã được đăng nhập bằng cách khác")
	}
	a.token = token
	a.userID = me.ID
	a.username = me.Username
	a.fullName = me.FullName
	a.role = me.Role
	if me.Kind == "staff" {
		a.role = "admin"
	}
	a.verified.Store(true)
	a.connectWS()
	a.refreshGameShortcuts()
	xong = true

	out, _ := json.Marshal(map[string]string{
		"id": a.userID, "username": a.username, "full_name": a.fullName, "role": a.role,
	})
	return string(out), nil
}

// CurrentLogin trả lần đăng nhập Go đang giữ, cùng dạng với Login, để giao diện nhận
// lấy: khách vào máy ở màn khoá native trong lúc trang còn nạp phía sau, hoặc trang vừa
// nạp lại. Chuỗi rỗng khi chưa có lần đăng nhập nào được máy chủ xác nhận. Gọi bao
// nhiêu lần cũng được.
func (a *App) CurrentLogin() string {
	if !a.loggedIn() || !a.verified.Load() {
		return ""
	}
	kind := "member"
	if a.role == "admin" {
		kind = "staff"
	}
	out, _ := json.Marshal(ClientLoginResponse{
		LoginResponse: LoginResponse{
			AccessToken: a.token,
			User:        &UserInfo{ID: a.userID, Username: a.username, FullName: a.fullName, Role: a.role},
		},
		Kind: kind,
	})
	return string(out)
}

// offlineReason hiện trên lớp phủ khi mất kết nối máy chủ.
const offlineReason = "Mất kết nối máy chủ — đang thử lại"

// linkStatus là báo cáo gửi dịch vụ nền mỗi hai giây.
func (a *App) linkStatus() uiStatus {
	maint := a.baoTri.Load()
	dangNhap := a.loggedIn() && a.verified.Load()
	return uiStatus{
		Locked: !maint && (!dangNhap || a.offlineLocked.Load()),
		Member: dangNhap && a.role != "admin",
		Admin:  dangNhap && a.role == "admin",
		Maint:  maint,
	}
}

// applyServiceReply làm theo chỉ thị của dịch vụ nền.
func (a *App) applyServiceReply(r svcReply) {
	setPolicy(r.Policy)
	a.coChinhSach()
	a.setOfflineLock(r.OfflineLock)
}

// setOfflineLock phủ màn hình khi mất kết nối máy chủ, và gỡ ra khi có lại.
//
// Máy chủ ngừng tính tiền sau hai phút không nghe thấy máy; không khoá ở đây
// thì quãng sau đó là dùng máy không ai tính. Chỉ áp cho hội viên: máy chưa ai
// đăng nhập đã ở màn hình khoá sẵn, còn nhân viên và chế độ bảo trì là người
// đang sửa máy — nhiều khi chính vì mạng hỏng.
func (a *App) setOfflineLock(on bool) {
	if a.windowMode != "" || a.ctx == nil {
		return
	}
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if on {
		// Màn khoá native còn giữ máy thì máy đang khoá sẵn, và cửa sổ Wails không được
		// hiện đè lên nó. Hạ nó rồi mà vẫn mất kết nối thì lượt trả lời kế khoá lại.
		if a.offlineLocked.Load() || a.baoTri.Load() || !a.loggedIn() || a.role == "admin" || nativeLockOwns() {
			return
		}
		a.offlineLocked.Store(true)
		log.Printf("[offline] mất kết nối máy chủ quá ngưỡng — khoá màn hình")
		a.closeChildWindows()
		wailsruntime.WindowShow(a.ctx)
		wailsruntime.WindowUnminimise(a.ctx)
		wailsruntime.WindowFullscreen(a.ctx)
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
		wailsruntime.EventsEmit(a.ctx, "vnet:machine:locked", offlineReason)
		if err := a.locker.Lock(); err != nil {
			log.Printf("[offline] đã phủ màn hình nhưng không chặn được phím: %v", err)
		}
		return
	}

	if !a.offlineLocked.Load() {
		return
	}
	a.offlineLocked.Store(false)
	log.Printf("[offline] đã nối lại máy chủ")
	wailsruntime.EventsEmit(a.ctx, "vnet:machine:unlocked")
	a.applyLoginLock(!a.loggedIn())
	a.dungLaiCuaSoPhu()
	// Frontend hỏi lại máy chủ ngay: phiên còn thì chơi tiếp, phiên đã bị đóng
	// trong lúc mất mạng thì về màn hình đăng nhập.
	wailsruntime.EventsEmit(a.ctx, "vnet:offline:cleared")
}

// GetWindowMode cho frontend biết nó đang chạy trong cửa sổ nào.
func (a *App) GetWindowMode() string {
	a.panelMu.Lock()
	defer a.panelMu.Unlock()
	if a.panelMode != "" {
		return a.panelMode
	}
	return a.windowMode
}

var panelTitles = map[string]string{
	"order":      "VNET · Gọi món",
	"support":    "VNET · Hỗ trợ",
	"topup":      "VNET · Nạp tiền",
	"attendance": "VNET · Điểm danh",
	"feedback":   "VNET · Đánh giá",
	"games":      "VNET · Game",
}

// moMenuGameTuLoiTat: khách bấm lối tắt "Menu Game" trên desktop (gameshortcuts.go).
// Chưa ai đăng nhập thì menu Game không có gì để mở: làm như lối tắt VNET.
func (a *App) moMenuGameTuLoiTat() {
	if !a.loggedIn() {
		a.ShowWindow()
		return
	}
	if err := a.OpenWindow("games"); err != nil {
		log.Printf("[window] không mở được menu Game từ lối tắt: %v", err)
	}
}

// switchPanel đổi cửa sổ phụ đang chạy sang màn hình khác rồi đưa nó lên.
func (a *App) switchPanel(mode string) {
	title, ok := panelTitles[mode]
	if !ok || a.ctx == nil {
		return
	}
	a.panelMu.Lock()
	a.panelMode = mode
	a.panelMu.Unlock()
	wailsruntime.WindowSetTitle(a.ctx, title)
	wailsruntime.EventsEmit(a.ctx, "vnet:panel:mode", mode)
	// Đo lại màn hình mỗi lần hiện: cửa sổ này dựng sẵn ẩn từ lúc khách đăng nhập và
	// chỉ được căn lúc đó, nên khách đổi độ phân giải rồi mở Gọi món sẽ thấy nó nằm
	// nguyên chỗ cũ — nửa dưới (nút thanh toán) tràn ra ngoài màn hình nhỏ hơn.
	canhGiuaManHinh(a.ctx, 640, 480)
	a.ShowWindow()
}

// HidePanel ẩn cửa sổ phụ thay vì tắt, để lần mở sau hiện ngay.
func (a *App) HidePanel() {
	if a.windowMode != "" && a.ctx != nil {
		wailsruntime.WindowHide(a.ctx)
	}
}

// ShowWindow kéo cửa sổ lên trước mặt người dùng.
//
// Dùng khi có tin nhắn tới lúc khách đang chơi game toàn màn hình, và khi một
// tiến trình khác cố mở cửa sổ đã chạy (SingleInstanceLock gọi vào đây).
// AlwaysOnTop được bật rồi tắt: bật vĩnh viễn thì cửa sổ hỗ trợ che game mãi.
//
// Với thanh điều khiển chính, đây là việc lối tắt VNET trên desktop làm: WindowShow
// khôi phục cửa sổ đang thu nhỏ rồi đưa lên trước. Đang khoá thì không có gì để
// "khôi phục" — màn hình khoá phủ kín và không bao giờ thu nhỏ được; WindowUnminimise
// bỏ qua khi đang toàn màn hình, nên lệnh này không đổi trạng thái khoá.
//
// Màn khoá native còn giữ máy thì cửa sổ Wails phải nằm ẩn sau nó: chỉ đưa ô đăng nhập
// native lên trước.
func (a *App) ShowWindow() {
	if nativeLockOwns() {
		nativeLockFocus()
		return
	}
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	go func() {
		time.Sleep(1200 * time.Millisecond)
		if a.ctx != nil && a.windowMode != "" {
			wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
		}
	}()
}

// coTheThuNho: lúc này thanh điều khiển có được thu xuống thanh tác vụ không.
//
// Dùng đúng định nghĩa "đang khoá" mà dịch vụ nền nhận qua linkStatus, để hai bên
// không lệch nhau: màn hình đăng nhập, khoá vì mất kết nối máy chủ, phiên chưa được
// máy chủ xác nhận — đều KHÔNG được thu nhỏ, vì thu nhỏ màn hình khoá là lộ desktop.
// Chế độ bảo trì thì được: nhân viên kỹ thuật cần desktop, và hai đường mở khoá bảo
// trì ở trên vốn đã tự thu nhỏ cửa sổ.
func (a *App) coTheThuNho() bool {
	return a.windowMode == "" && !a.linkStatus().Locked
}

// MinimiseDock thu thanh điều khiển xuống thanh tác vụ — việc duy nhất nút thu nhỏ
// trên thanh tiêu đề tự vẽ (DockTitleBar.vue) làm. Nút tắt không tồn tại.
//
// Frontend chỉ ĐỀ NGHỊ (như SetLoggedIn): màn hình khoá không được thu nhỏ kể cả khi
// ai đó gọi thẳng hàm này từ giao diện, nên chốt chặn nằm ở đây chứ không ở nút.
// Thu xuống rồi thì nút trên taskbar vẫn còn; bấm nó, hoặc bấm lối tắt VNET trên
// desktop (ShowWindow), là thanh hiện lại đúng chỗ cũ.
func (a *App) MinimiseDock() {
	if a.ctx == nil || !a.coTheThuNho() {
		return
	}
	wailsruntime.WindowMinimise(a.ctx)
}

// beforeDockClose là OnBeforeClose của thanh điều khiển chính (runDock): lệnh đóng
// nào cũng thành thu nhỏ — hoặc bị bỏ qua nếu đang khoá — và KHÔNG BAO GIỜ cho thoát.
// Trả true là bảo Wails dừng Quit(). Lý do và ranh giới với tắt máy ở main.go.
func (a *App) beforeDockClose(context.Context) (prevent bool) {
	a.MinimiseDock()
	return true
}

// OpenWindow mở một cửa sổ phụ trong tiến trình riêng.
//
// Wails v2 không tạo được cửa sổ thứ hai trong cùng tiến trình, nên "cửa sổ
// riêng" là chạy lại chính tệp .exe này với cờ --window. Cửa sổ đã chạy thì
// SingleInstanceLock bên đó nhận cú gõ cửa và tự đưa mình lên trước.
//
// Token đưa qua STDIN chứ không qua tham số dòng lệnh: tham số hiện nguyên văn
// trong Task Manager, ai ngồi máy cũng đọc được.
func (a *App) OpenWindow(mode string) error {
	// Cửa sổ phụ không tự dựng thêm cửa sổ phụ.
	if a.windowMode != "" {
		return nil
	}
	switch mode {
	case "order", "support", "topup", "attendance", "feedback", "games", "prewarm":
	default:
		return fmt.Errorf("cửa sổ %q không hợp lệ", mode)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "--window", mode)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	handoff, _ := json.Marshal(map[string]string{
		"token":        a.token,
		"user_id":      a.userID,
		"username":     a.username,
		"full_name":    a.fullName,
		"role":         a.role,
		"machine_code": a.machineCode,
		"server_url":   a.cfg.ServerURL,
	})
	stdin.Write(append(handoff, '\n'))
	stdin.Close()

	proc := cmd.Process
	childMu.Lock()
	childProcs[proc.Pid] = proc
	childMu.Unlock()
	batDau := time.Now()
	go func() {
		err := cmd.Wait()
		childMu.Lock()
		// Còn trong danh sách nghĩa là nó TỰ chết; closeChildWindows giết thì
		// đã xoá trước rồi.
		_, tuChet := childProcs[proc.Pid]
		delete(childProcs, proc.Pid)
		childMu.Unlock()

		// WebView2 thỉnh thoảng không dựng được khung hiển thị lúc vừa có mạng
		// lại (CreateCoreWebView2Controller lỗi) và thư viện thoát luôn. Cửa sổ
		// dựng sẵn chết sớm như vậy thì dựng lại, tối đa ba lần liên tiếp.
		// err == nil là lần gõ cửa vào cửa sổ đang chạy — thoát bình thường.
		if mode == "prewarm" && tuChet && err != nil && time.Since(batDau) < 15*time.Second {
			if n := a.prewarmRetry.Add(1); n <= 3 && a.loggedIn() {
				log.Printf("[window] cửa sổ phụ chết khi khởi động (%v) — dựng lại lần %d", err, n)
				time.Sleep(3 * time.Second)
				_ = a.OpenWindow("prewarm")
			}
			return
		}
		if mode == "prewarm" {
			a.prewarmRetry.Store(0)
		}
	}()
	return nil
}

// Cửa sổ phụ do thanh điều khiển mở. Giữ lại để đóng chúng khi phiên kết thúc:
// để sót thì cửa sổ hỗ trợ còn nằm trên màn hình khoá, và cửa sổ thực đơn vẫn
// gọi món bằng token của người vừa trả máy.
var (
	childMu    sync.Mutex
	childProcs = map[int]*os.Process{}
)

func (a *App) closeChildWindows() {
	childMu.Lock()
	defer childMu.Unlock()
	for pid, p := range childProcs {
		if err := p.Kill(); err != nil {
			log.Printf("[window] không đóng được cửa sổ phụ %d: %v", pid, err)
		}
		delete(childProcs, pid)
	}
}

// GetBootTime trả về thời điểm Windows khởi động (giây Unix). Giao diện lưu nó
// lúc đăng nhập để nhận ra máy đã khởi động lại: khi đó KHÔNG được khôi phục
// đăng nhập cũ, vì phiên của nó đã bị máy chủ chốt tại lúc reboot.
func (a *App) GetBootTime() int64 {
	t, err := host.BootTime()
	if err != nil {
		return 0
	}
	return int64(t)
}

func (a *App) IsLoggedIn() bool {
	return a.token != ""
}

func (a *App) GetHardware() (string, error) {
	data, err := a.doRequest("GET", "/api/machines/by-code/"+a.machineCode, nil)
	if err != nil {
		info := map[string]interface{}{
			"machine_code": a.machineCode,
			"server_url":   a.cfg.ServerURL,
		}
		data, _ := json.Marshal(info)
		return string(data), nil
	}
	return string(data), nil
}

func (a *App) GetUserInfo() (string, error) {
	info := map[string]string{
		"id":        a.userID,
		"username":  a.username,
		"full_name": a.fullName,
		"role":      a.role,
	}
	data, _ := json.Marshal(info)
	return string(data), nil
}

func (a *App) GetMachineCode() string {
	return a.machineCode
}

func (a *App) GetMemberInfo() (string, error) {
	data, err := a.doRequest("GET", "/api/members/"+a.userID, nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SubmitFeedback gửi đánh giá dịch vụ.
//
// Không gửi kèm mã máy: máy chủ suy ra từ phiên đang chơi của chính hội viên.
// Nếu để máy khách khai máy thì ai sửa được request cũng gán được nhận xét xấu
// cho máy bất kỳ.
func (a *App) SubmitFeedback(rating int, content, orderID string) (string, error) {
	body := map[string]interface{}{"rating": rating, "content": content}
	if orderID != "" {
		body["order_id"] = orderID
	}
	data, err := a.doRequest("POST", "/api/feedback", body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// --- Điểm danh hằng ngày ---------------------------------------------------
//
// Máy chủ ép member_id về chính người gọi khi token là của hội viên, nên hai
// hàm này không điểm danh hộ tài khoản khác được.

func (a *App) GetAttendanceStatus() (string, error) {
	data, err := a.doRequest("GET", "/api/attendance/status", nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) CheckinAttendance() (string, error) {
	data, err := a.doRequest("POST", "/api/attendance/checkin", map[string]interface{}{})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetSession() (string, error) {
	data, err := a.doRequest("GET", "/api/sessions/me", nil)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (a *App) GetMenu(categoryId string) (string, error) {
	// Không truyền page_size thì máy chủ mặc định 20, mà thực đơn không có nút
	// sang trang — quán bán quá hai mươi món là khách không thấy phần còn lại.
	// Không gửi sort: /api/products tự sắp theo sort_order rồi tên, và nó bỏ qua
	// tham số sort — gửi lên chỉ tạo cảm giác an toàn giả.
	q := url.Values{"page_size": {"500"}}
	if categoryId != "" {
		q.Set("category_id", categoryId)
	}
	data, err := a.doRequest("GET", "/api/products?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}

	var paginated PaginatedData
	if err := json.Unmarshal(data, &paginated); err != nil {
		return string(data), nil
	}
	return string(a.absoluteImageURLs(paginated.Items)), nil
}

// absoluteImageURLs đổi image_url tương đối thành địa chỉ đầy đủ.
//
// Máy chủ trả về "/uploads/...", đúng cho trang quản trị vì trang đó cùng nguồn.
// Giao diện máy khách thì không: nó chạy từ máy chủ tài nguyên nhúng của Wails,
// nên đường dẫn tương đối trỏ vào chính nó và mọi ảnh món ăn đều hỏng.
func (a *App) absoluteImageURLs(items json.RawMessage) json.RawMessage {
	var products []map[string]any
	if err := json.Unmarshal(items, &products); err != nil {
		return items
	}
	base := strings.TrimRight(a.GetServerURL(), "/")
	for _, p := range products {
		if u, ok := p["image_url"].(string); ok && strings.HasPrefix(u, "/") {
			p["image_url"] = base + u
		}
	}
	out, err := json.Marshal(products)
	if err != nil {
		return items
	}
	return out
}

func (a *App) GetCategories() (string, error) {
	data, err := a.doRequest("GET", "/api/categories", nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) PlaceOrder(itemsJSON string) (string, error) {
	var items []OrderItemRequest
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return "", fmt.Errorf("invalid items format")
	}

	req := OrderRequest{
		OrderType:   "machine_order",
		MemberID:    a.userID,
		MachineCode: a.machineCode,
		Items:       items,
	}

	data, err := a.doRequest("POST", "/api/orders", req)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetMyOrders trả mảng JSON các đơn gần nhất của hội viên đang đăng nhập, mới
// nhất trước — để khách xem lại món đã gọi và đơn đã được quầy duyệt chưa.
func (a *App) GetMyOrders() (string, error) {
	if a.userID == "" {
		return "[]", nil
	}
	data, err := a.doRequest("GET", "/api/members/"+url.PathEscape(a.userID)+"/orders?page_size=30", nil)
	if err != nil {
		return "", err
	}
	var paginated PaginatedData
	if err := json.Unmarshal(data, &paginated); err != nil || paginated.Items == nil {
		return "[]", nil
	}
	return string(paginated.Items), nil
}

// GetTopupPresets trả về mảng mệnh giá nạp, dạng JSON `[5000, 10000, ...]`.
//
// Bản cũ lệch với máy chủ cả ba chiều — tìm nhóm "general" thay vì "topup",
// khoá "topup_presets" thay vì "presets", và mong một mảng trong khi giá trị
// lưu là `{"values": [...]}`. Hệ quả: luôn rơi về danh sách cứng, quán không
// đổi được mệnh giá khách nhìn thấy.
// GetFeatureFlags trả về các công tắc bật/tắt tính năng do quán đặt ở trang
// quản trị, dạng {"attendance_enabled": true, ...}.
//
// Mặc định BẬT HẾT khi không đọc được: nhóm cài đặt "features" chưa tồn tại cho
// tới lần Lưu đầu tiên và máy chủ trả 404 khi nhóm rỗng. Mặc định tắt sẽ làm
// điểm danh và đánh giá biến mất ở mọi quán chưa từng mở tab đó.
func (a *App) GetFeatureFlags() (string, error) {
	type settingItem struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	defaults := map[string]bool{"attendance_enabled": true, "feedback_enabled": true}
	fallback := func() string {
		out, _ := json.Marshal(defaults)
		return string(out)
	}

	data, err := a.doRequest("GET", "/api/settings/features", nil)
	if err != nil {
		return fallback(), nil
	}

	var settings []settingItem
	if err := json.Unmarshal(data, &settings); err != nil {
		return fallback(), nil
	}

	flags := map[string]bool{}
	for k, v := range defaults {
		flags[k] = v
	}
	for _, item := range settings {
		// Giá trị về từ cột jsonb là CHUỖI: "false" là chuỗi khác rỗng, nên phải
		// parse thật chứ không kiểm bằng chuỗi rỗng hay không.
		b, err := strconv.ParseBool(strings.TrimSpace(item.Value))
		if err != nil {
			continue
		}
		flags[item.Key] = b
	}

	out, _ := json.Marshal(flags)
	return string(out), nil
}

func (a *App) GetTopupPresets() (string, error) {
	type settingItem struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	data, err := a.doRequest("GET", "/api/settings/topup", nil)
	if err != nil {
		return a.defaultPresets(), nil
	}

	var settings []settingItem
	if err := json.Unmarshal(data, &settings); err != nil {
		return a.defaultPresets(), nil
	}

	for _, s := range settings {
		if s.Key != "presets" || s.Value == "" {
			continue
		}
		if values := parsePresetValues(s.Value); len(values) > 0 {
			out, _ := json.Marshal(values)
			return string(out), nil
		}
	}

	return a.defaultPresets(), nil
}

// parsePresetValues nhận cả hai dạng: `{"values": [...]}` (dạng seed đang lưu)
// và mảng trần `[...]`. Nhận cả hai để bản máy khách cũ/mới cùng chạy được với
// một database, thay vì im lặng rơi về mặc định.
func parsePresetValues(raw string) []int64 {
	var wrapped struct {
		Values []int64 `json:"values"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil && len(wrapped.Values) > 0 {
		return wrapped.Values
	}
	var bare []int64
	if err := json.Unmarshal([]byte(raw), &bare); err == nil {
		return bare
	}
	return nil
}

func (a *App) defaultPresets() string {
	defaults := []int64{5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000}
	jsonData, _ := json.Marshal(defaults)
	return string(jsonData)
}

func (a *App) RequestTopup(amount int64) (string, error) {
	reqBody := map[string]interface{}{
		"amount":       amount,
		"member_id":    a.userID,
		"machine_code": a.machineCode,
	}

	data, err := a.doRequest("POST", "/api/orders/topup-request", reqBody)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) ChangePin(oldPin, newPin string) (string, error) {
	reqBody := map[string]string{
		"old_password": oldPin,
		"new_password": newPin,
	}

	data, err := a.doRequest("PUT", "/api/auth/change-password", reqBody)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) SendRoomMessage(roomID, message string) (string, error) {
	reqBody := map[string]string{
		"room_id":      roomID,
		"sender_type":  "member",
		"sender_id":    a.userID,
		"message":      message,
		"message_type": "text",
	}

	data, err := a.doRequest("POST", "/api/chat/messages", reqBody)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetRoomMessages(roomID string) (string, error) {
	data, err := a.doRequest("GET", "/api/chat/rooms/"+roomID+"/messages", nil)
	if err != nil {
		return "", err
	}

	var paginated PaginatedData
	if err := json.Unmarshal(data, &paginated); err != nil {
		return string(data), nil
	}
	return string(paginated.Items), nil
}

func (a *App) CreateRoom(title, participantID, participantType string) (string, error) {
	reqBody := map[string]string{
		"title":            title,
		"participant_id":   participantID,
		"participant_type": participantType,
	}
	data, err := a.doRequest("POST", "/api/chat/rooms", reqBody)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetRooms() (string, error) {
	data, err := a.doRequest("GET", "/api/chat/rooms", nil)
	if err != nil {
		return "", err
	}

	var paginated PaginatedData
	if err := json.Unmarshal(data, &paginated); err != nil {
		return string(data), nil
	}
	return string(paginated.Items), nil
}

func (a *App) GetNotifications() (string, error) {
	data, err := a.doRequest("GET", "/api/notifications", nil)
	if err != nil {
		return "", err
	}

	var paginated PaginatedData
	if err := json.Unmarshal(data, &paginated); err != nil {
		return string(data), nil
	}
	return string(paginated.Items), nil
}

func (a *App) GetUnreadNotificationCount() (string, error) {
	data, err := a.doRequest("GET", "/api/notifications/unread-count", nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) MarkNotificationRead(id string) (string, error) {
	_, err := a.doRequest("PUT", "/api/notifications/"+id+"/read", nil)
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func (a *App) MarkAllNotificationsRead() (string, error) {
	_, err := a.doRequest("PUT", "/api/notifications/read-all", nil)
	if err != nil {
		return "", err
	}
	return "ok", nil
}

// TakeScreenshot chụp màn hình và trả về data URI để gửi kèm tin nhắn chat.
//
// Cùng captureScreen với lệnh remote:screenshot; khác ở chỗ này là khách tự
// bấm gửi, còn lệnh remote là nhân viên chụp từ xa.
func (a *App) TakeScreenshot() (string, error) {
	return captureScreen(chupChoChat)
}

func (a *App) SendScreenshotMessage(roomID, imageData string) (string, error) {
	reqBody := map[string]string{
		"room_id":      roomID,
		"sender_type":  "member",
		"sender_id":    a.userID,
		"message":      imageData,
		"message_type": "screenshot",
	}

	data, err := a.doRequest("POST", "/api/chat/messages", reqBody)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) MarkMessageDelivered(messageID string) (string, error) {
	data, err := a.doRequest("PUT", "/api/chat/messages/"+messageID+"/deliver", nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) MarkMessageRead(messageID string) (string, error) {
	data, err := a.doRequest("PUT", "/api/chat/messages/"+messageID+"/read", nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) MarkRoomMessagesRead(roomID string) (string, error) {
	data, err := a.doRequest("PUT", "/api/chat/rooms/"+roomID+"/read", nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// --- Điều khiển từ xa ------------------------------------------------------
//
// Máy chủ đẩy sự kiện "remote:<lệnh>" qua WebSocket. Trước đây client không
// đăng ký handler nào cho chúng: lệnh tới nơi, ghi log "no handler" rồi bị bỏ —
// cả chuỗi điều khiển từ xa chưa từng chạy.
//
// Danh sách lệnh ở đây phải khớp remoteActions bên backend
// (internal/service/machine.go). Cố ý không có lệnh chạy câu lệnh tuỳ ý.

func (a *App) registerRemoteHandlers(c *WSClient) {
	c.On("remote:shutdown", func(msg WSMessage) {
		if err := shutdownMachine(); err != nil {
			log.Printf("[remote] tắt máy thất bại: %v", err)
		}
	})

	c.On("remote:restart", func(msg WSMessage) {
		if err := restartMachine(); err != nil {
			log.Printf("[remote] khởi động lại thất bại: %v", err)
		}
	})

	c.On("remote:message", func(msg WSMessage) {
		title := remoteStringField(msg, "title")
		if title == "" {
			title = "Thông báo"
		}
		body := remoteStringField(msg, "message")
		if body == "" {
			return
		}
		a.locker.ShowMessage(title, body)
	})

	// Ba lệnh giám sát chạy Ở GIAO DIỆN, không ở dịch vụ nền: chụp màn hình cần
	// desktop của khách, mà dịch vụ chạy ở session 0 không thấy desktop nào.
	c.On("remote:screenshot", func(msg WSMessage) {
		reqID := remoteRequestID(msg)
		img, err := captureScreen(chupTuXa)
		if err != nil {
			log.Printf("[remote] chụp màn hình thất bại: %v", err)
			return
		}
		if err := baoCaoLen(a.cfg, "screenshot", map[string]string{
			"request_id": reqID, "image": img,
		}); err != nil {
			log.Printf("[remote] gửi ảnh lên thất bại: %v", err)
		}
	})

	c.On("remote:process-list", func(msg WSMessage) {
		reqID := remoteRequestID(msg)
		if err := baoCaoLen(a.cfg, "processes", map[string]interface{}{
			"request_id": reqID, "processes": likeTienTrinh(), "killed": -1,
		}); err != nil {
			log.Printf("[remote] gửi danh sách tiến trình thất bại: %v", err)
		}
	})

	c.On("remote:process-kill", func(msg WSMessage) {
		reqID := remoteRequestID(msg)
		name := remoteStringField(msg, "process")
		killed := 0
		if name != "" {
			killed = tatTienTrinh(name)
		}
		// Gửi kèm danh sách mới để trang quản trị vẽ lại bảng ngay, thấy tiến
		// trình vừa tắt đã biến mất.
		if err := baoCaoLen(a.cfg, "processes", map[string]interface{}{
			"request_id": reqID, "processes": likeTienTrinh(), "killed": killed,
		}); err != nil {
			log.Printf("[remote] gửi kết quả tắt tiến trình thất bại: %v", err)
		}
	})
}

// remoteRequestID bóc request_id nằm NGANG HÀNG với payload (không nằm trong
// payload): backend đặt nó ở vỏ ngoài để không làm lệch các trường payload mà
// những lệnh cũ đang đọc.
func remoteRequestID(msg WSMessage) string {
	var envelope struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(msg.Payload, &envelope); err != nil {
		return ""
	}
	return envelope.RequestID
}

// remoteStringField bóc một trường chuỗi khỏi payload lồng hai lớp mà backend
// gửi: {"machine_id":…, "payload": {…}}.
func remoteStringField(msg WSMessage, field string) string {
	var envelope struct {
		Payload map[string]interface{} `json:"payload"`
	}
	if err := json.Unmarshal(msg.Payload, &envelope); err != nil {
		return ""
	}
	if v, ok := envelope.Payload[field].(string); ok {
		return v
	}
	return ""
}

// dungLaiCuaSoPhu dựng lại cửa sổ phụ sau khi máy được mở khoá. Lúc khoá vì mất
// kết nối, cửa sổ phụ bị tắt; khách vẫn còn đăng nhập mà không dựng
// lại thì lần mở Đồ ăn kế tiếp lại chậm như trước khi có cửa sổ dựng sẵn.
func (a *App) dungLaiCuaSoPhu() {
	if !a.loggedIn() {
		return
	}
	if err := a.OpenWindow("prewarm"); err != nil {
		log.Printf("[window] không dựng lại được cửa sổ phụ: %v", err)
	}
}

func (a *App) IsLocked() bool {
	return a.locker.Locked()
}

func (a *App) ShowMessage(title, message string) (string, error) {
	a.locker.ShowMessage(title, message)
	return "ok", nil
}


