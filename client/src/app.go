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
}

func NewApp() *App {
	cfg := LoadConfig()
	return &App{
		cfg:         cfg,
		machineCode: cfg.MachineCode,
		locker:      NewScreenLocker(),
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

	// Nhớ mốc độ phân giải và chuột NGAY bây giờ, trước khi khách nào kịp đụng tới
	// (và trước khi màn hình khoá đo kích thước), chứ không đợi lần đầu có người mở
	// Cài đặt — lúc đó game có thể đang giữ một độ phân giải tạm.
	displays.startup()

	// Chưa ai đăng nhập thì máy phải bị khoá NGAY, trước cả khi frontend kịp
	// dựng xong. Đây là mặc định an toàn: mọi đường dẫn tới màn hình desktop
	// đều phải đi qua một lần đăng nhập thành công.
	a.applyLoginLock(true)
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
		wailsruntime.WindowShow(a.ctx)
		wailsruntime.WindowUnminimise(a.ctx)
		wailsruntime.WindowFullscreen(a.ctx)
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
		if err := a.locker.Lock(); err != nil {
			log.Printf("[login] đã phủ màn hình nhưng không chặn được phím: %v", err)
		}
		return
	}

	// KHÔNG gỡ hook chặn phím ở đây. Đăng nhập xong là chuyển từ "phủ kín màn
	// hình" sang "thanh dọc mép phải", nhưng khách đang chơi vẫn không được
	// thoát ra desktop. Chỉ đường mở khoá kỹ thuật mới gỡ hook.
	wailsruntime.WindowUnfullscreen(a.ctx)
	dockToRightEdge(a.ctx)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
}

// ApplyWindowState đặt lại hình dạng cửa sổ theo trạng thái hiện tại.
//
// Gọi từ OnDomReady. Wails chạy OnStartup trong goroutine song song với
// f.WindowCenter() của chính nó, nên lần đặt vị trí trong Startup có thể bị ghi
// đè; đây là lần chốt hạ, chạy ngay trước khi cửa sổ được hiện lên.
func (a *App) ApplyWindowState() {
	if a.windowMode != "" || a.ctx == nil {
		return
	}
	a.applyLoginLock(!a.loggedIn())
}

// loggedIn suy ra từ token: frontend gọi SetLoggedIn mỗi lần trạng thái đổi,
// nhưng OnDomReady có thể chạy TRƯỚC lần gọi đầu tiên đó.
func (a *App) loggedIn() bool {
	return a.token != ""
}

// SetLoggedIn được frontend gọi mỗi khi trạng thái đăng nhập đổi: đăng nhập
// xong, đăng xuất, hoặc khôi phục phiên cũ lúc khởi động.
func (a *App) SetLoggedIn(loggedIn bool) {
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
	a.baoTri.Store(true)
	a.locker.Unlock()
	a.applyLoginLock(false)
	wailsruntime.WindowMinimise(a.ctx)
}

// unlockWithCachedStaff mở máy bằng tài khoản nhân viên đã lưu lúc còn mạng.
//
// KHÔNG mở phiên và KHÔNG tính tiền — giống tài khoản quản trị máy trạm. Đây là đường để
// vào sửa máy hoặc gỡ máy trạm, không phải một cách đăng nhập thay thế.
func (a *App) unlockWithCachedStaff(username, password string) error {
	c, err := verifyCachedStaff(username, password, time.Now())
	if err != nil {
		time.Sleep(time.Second)
		log.Printf("[offline] từ chối %q: %v", username, err)
		return fmt.Errorf("Không kết nối được máy chủ. Khi mất mạng chỉ tài khoản nhân viên từng đăng nhập trên máy này mới mở được máy")
	}

	log.Printf("[offline] mở khoá bằng tài khoản %q (%s) đã lưu ngày %s",
		c.Username, c.Role, c.SavedAt.Format("02/01/2006"))
	a.baoTri.Store(true)
	a.locker.Unlock()
	a.applyLoginLock(false)
	wailsruntime.WindowMinimise(a.ctx)
	return nil
}

// HasCachedStaff cho màn hình khoá biết máy này có đường vào offline hay chưa.
func (a *App) HasCachedStaff() bool {
	return len(loadCredCache()) > 0
}

func (a *App) GetServerURL() string {
	if a.cfg.ServerURL == "" {
		return "http://localhost:20800"
	}
	return a.cfg.ServerURL
}

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
	log.Printf("[HTTP] %s %s → %d body=%s", method, fullURL, resp.StatusCode, string(bodyBytes))

	var apiResp APIResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		log.Printf("[HTTP] parse error: %v", err)
		return nil, fmt.Errorf("invalid response")
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
			if isSecretField(k) {
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
	}
	go func() {
		if err := a.wsClient.Connect(a.wsCtx); err != nil {
			log.Printf("ws client: %v", err)
		}
	}()
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
		// Chỉ rơi sang bản lưu offline khi KHÔNG với tới máy chủ. Sai mật khẩu
		// trả về lỗi bình thường; nhầm hai cái là sai mật khẩu cũng mở được máy.
		if errors.Is(err, errServerUnreachable) {
			if err := a.unlockWithCachedStaff(username, password); err != nil {
				return "", err
			}
			return `{"offline":true}`, nil
		}
		return "", err
	}

	var resp ClientLoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("invalid response")
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

	if resp.Kind == "staff" {
		// Lưu để lần sau mất mạng vẫn vào được máy mà sửa hoặc gỡ. Chỉ nhân
		// viên; hội viên lưu được nghĩa là khách rút dây mạng là tự mở máy.
		if err := rememberStaff(username, password, "staff"); err != nil {
			log.Printf("[offline] không lưu được tài khoản nhân viên: %v", err)
		}
	}

	// Hook chặn phím bật cho CẢ hai loại: khách đang chơi vẫn không được thoát
	// ra desktop. Nhân viên cần toàn quyền thì dùng đường mở khoá kỹ thuật.
	if err := a.locker.Lock(); err != nil {
		log.Printf("lock screen error: %v", err)
	}

	a.verified.Store(true)
	a.connectWS()

	return string(data), nil
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

	out, _ := json.Marshal(map[string]string{
		"id": a.userID, "username": a.username, "full_name": a.fullName, "role": a.role,
	})
	return string(out), nil
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
	if on {
		if a.offlineLocked.Load() || a.baoTri.Load() || !a.loggedIn() || a.role == "admin" {
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
func (a *App) ShowWindow() {
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


