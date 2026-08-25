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
	"sync/atomic"
	"time"

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
	// Đang ở chế độ bảo trì: mở bằng PIN kỹ thuật, không có phiên và không tính
	// tiền. Lớp chống phá đứng yên trong lúc này.
	baoTri atomic.Bool
	wsClient *WSClient
	wsCtx    context.Context
	wsCancel context.CancelFunc
	cancel   context.CancelFunc

	// windowMode rỗng nghĩa là thanh điều khiển chính; "order"/"support" là cửa
	// sổ phụ chạy trong tiến trình riêng. Frontend đọc giá trị này để render
	// thẳng màn hình tương ứng thay vì dựng cả thanh.
	windowMode string
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
	go runWatchdog(aCtx, a.cfg, a.locker, &[]string{}, new(bool), a.LockScreen)
	go newWebBlocker(a.cfg).run(aCtx)
	// Lớp canh dịch vụ nền chạy Ở ĐÂY chứ không ở tiến trình dịch vụ: dịch vụ
	// đã canh giao diện, đây là chiều còn lại.
	go runGuard(aCtx, a.cfg, a.baoTri.Load)

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
	if loggedIn {
		a.baoTri.Store(false)
	}
	a.applyLoginLock(!loggedIn)
}

// HasMaintenancePin cho màn hình khoá biết có nên hiện ô PIN hay không. Máy
// chưa đặt PIN mà vẫn hiện ô là mời người ta gõ vào một cái không bao giờ đúng.
func (a *App) HasMaintenancePin() bool {
	return a.cfg.MaintenancePinHash != ""
}

// UnlockMaintenance mở khoá máy bằng PIN kỹ thuật, KHÔNG hỏi máy chủ.
//
// Đây là đường vào duy nhất khi mất mạng. Mọi cách đăng nhập khác — hội viên,
// quét QR, tài khoản quản trị — đều gọi API, nên router hỏng là cả phòng máy
// đứng trước màn hình khoá phủ kín mà không có gì gõ vào được.
//
// Mở kiểu này KHÔNG mở phiên và KHÔNG tính tiền: nó dành cho nhân viên kỹ thuật,
// không phải cho khách. Lớp chống phá cũng đứng yên, vì việc đầu tiên người sửa
// máy làm thường là tắt dịch vụ.
func (a *App) UnlockMaintenance(pin string) error {
	if err := verifyPin(a.cfg.MaintenancePinHash, pin); err != nil {
		// Chậm lại một nhịp: máy đứng ngay trước mặt người muốn dò, và dò một
		// mã sáu chữ số qua giao diện là chuyện của vài phút nếu không có nó.
		time.Sleep(time.Second)
		log.Printf("[bảo trì] PIN sai")
		return err
	}

	log.Printf("[bảo trì] mở khoá bằng PIN kỹ thuật")
	a.baoTri.Store(true)
	a.locker.Unlock()
	a.applyLoginLock(false)
	wailsruntime.WindowMinimise(a.ctx)
	return nil
}

// unlockWithCachedStaff mở máy bằng tài khoản nhân viên đã lưu lúc còn mạng.
//
// KHÔNG mở phiên và KHÔNG tính tiền — giống hệt PIN kỹ thuật. Đây là đường để
// vào sửa máy hoặc gỡ máy trạm, không phải một cách đăng nhập thay thế.
func (a *App) unlockWithCachedStaff(username, password string) error {
	c, err := verifyCachedStaff(username, password, time.Now())
	if err != nil {
		time.Sleep(time.Second)
		log.Printf("[offline] từ chối %q: %v", username, err)
		return fmt.Errorf("không kết nối được máy chủ, và %v", err)
	}

	if c.Builtin {
		log.Printf("[offline] mở khoá bằng TÀI KHOẢN MẶC ĐỊNH của bản cài — " +
			"máy này chưa từng có nhân viên nào đăng nhập qua máy chủ")
	} else {
		log.Printf("[offline] mở khoá bằng tài khoản %q (%s) đã lưu ngày %s",
			c.Username, c.Role, c.SavedAt.Format("02/01/2006"))
	}
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

// HasBuiltinAdmin: máy này còn mở được bằng tài khoản mặc định của bản cài hay
// không. Giao diện đọc hàm này để cảnh báo mỗi lần có nhân viên đăng nhập —
// một cửa sau mà không ai biết nó còn mở thì tệ hơn là không có cửa nào.
func (a *App) HasBuiltinAdmin() bool {
	return builtinAdminActive()
}

func (a *App) SetServerURL(url string) {
	a.cfg.ServerURL = url
}

func (a *App) GetServerURL() string {
	if a.cfg.ServerURL == "" {
		return "http://localhost:8080"
	}
	return a.cfg.ServerURL
}

func (a *App) doRequest(method, path string, body interface{}) (json.RawMessage, error) {
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

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		log.Printf("[HTTP] new request error: %v", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[HTTP] do error: %v", err)
		// Bọc bằng sentinel: "không với tới máy chủ" và "sai mật khẩu" phải
		// phân biệt được, vì chỉ trường hợp đầu mới được rơi sang bản đệm
		// offline. Nhầm hai cái là sai mật khẩu cũng mở được máy.
		return nil, fmt.Errorf("%w: %v", errServerUnreachable, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
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
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

func (a *App) connectWS() {
	if a.wsCancel != nil {
		a.wsCancel()
	}
	a.wsCtx, a.wsCancel = context.WithCancel(context.Background())
	a.wsClient = NewWSClient(a.ctx, a.GetServerURL(), a.token, a.machineCode)
	a.registerRemoteHandlers(a.wsClient)
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

	a.connectWS()

	return string(data), nil
}

func (a *App) Logout() error {
	a.locker.Unlock()
	a.token = ""
	a.userID = ""
	a.username = ""
	a.fullName = ""
	a.role = ""
	if a.wsCancel != nil {
		a.wsCancel()
	}
	return nil
}

func (a *App) RestoreSession(token, userID, username, fullName, role string) error {
	a.token = token
	a.userID = userID
	a.username = username
	a.fullName = fullName
	a.role = role
	a.connectWS()
	return nil
}

// GetWindowMode cho frontend biết nó đang chạy trong cửa sổ nào.
func (a *App) GetWindowMode() string {
	return a.windowMode
}

// ShowWindow kéo cửa sổ lên trước mặt người dùng.
//
// Dùng khi có tin nhắn tới lúc khách đang chơi game toàn màn hình, và khi một
// tiến trình khác cố mở cửa sổ đã chạy (SingleInstanceLock gọi vào đây).
// AlwaysOnTop được bật rồi tắt: bật vĩnh viễn thì cửa sổ hỗ trợ che game mãi.
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

// OpenWindow mở một cửa sổ phụ trong tiến trình riêng.
//
// Wails v2 không tạo được cửa sổ thứ hai trong cùng tiến trình, nên "cửa sổ
// riêng" là chạy lại chính tệp .exe này với cờ --window. Cửa sổ đã chạy thì
// SingleInstanceLock bên đó nhận cú gõ cửa và tự đưa mình lên trước.
//
// Token đưa qua STDIN chứ không qua tham số dòng lệnh: tham số hiện nguyên văn
// trong Task Manager, ai ngồi máy cũng đọc được.
func (a *App) OpenWindow(mode string) error {
	if mode != "order" && mode != "support" && mode != "topup" {
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
	go cmd.Wait()
	return nil
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
	return captureScreen()
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
	c.On("remote:lock", func(msg WSMessage) {
		reason := remoteStringField(msg, "reason")
		if err := a.LockScreen(reason); err != nil {
			log.Printf("[remote] khoá máy thất bại: %v", err)
		}
	})

	c.On("remote:unlock", func(msg WSMessage) {
		if err := a.UnlockScreen(); err != nil {
			log.Printf("[remote] mở khoá thất bại: %v", err)
		}
	})

	c.On("remote:shutdown", func(msg WSMessage) {
		if _, err := a.ShutdownMachine(); err != nil {
			log.Printf("[remote] tắt máy thất bại: %v", err)
		}
	})

	c.On("remote:restart", func(msg WSMessage) {
		if _, err := a.RestartMachine(); err != nil {
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

	c.On("remote:block-app", func(msg WSMessage) {
		name := remoteStringField(msg, "process")
		if name == "" {
			log.Printf("[remote] chặn ứng dụng: thiếu tên tiến trình")
			return
		}
		if err := a.BlockApp(name); err != nil {
			log.Printf("[remote] chặn %s thất bại: %v", name, err)
		}
	})

	c.On("remote:unblock-app", func(msg WSMessage) {
		name := remoteStringField(msg, "process")
		if name == "" {
			log.Printf("[remote] bỏ chặn ứng dụng: thiếu tên tiến trình")
			return
		}
		if err := a.UnblockApp(name); err != nil {
			log.Printf("[remote] bỏ chặn %s thất bại: %v", name, err)
		}
	})

	// Ba lệnh giám sát chạy Ở GIAO DIỆN, không ở dịch vụ nền: chụp màn hình cần
	// desktop của khách, mà dịch vụ chạy ở session 0 không thấy desktop nào.
	c.On("remote:screenshot", func(msg WSMessage) {
		reqID := remoteRequestID(msg)
		img, err := captureScreen()
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

// LockScreen phủ kín màn hình và chặn các phím thoát ra.
//
// Hai nửa tách nhau có chủ đích: cửa sổ là việc của Wails, chặn phím là việc
// của ScreenLocker. Nếu hook bàn phím không cài được (thiếu quyền chẳng hạn)
// thì vẫn phủ màn hình — rào cản yếu hơn còn hơn không có gì — nhưng lỗi được
// trả về để chỗ gọi biết.
func (a *App) LockScreen(reason string) error {
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowFullscreen(a.ctx)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wailsruntime.EventsEmit(a.ctx, "vnet:machine:locked", reason)

	if err := a.locker.Lock(); err != nil {
		return fmt.Errorf("đã phủ màn hình nhưng không chặn được phím: %w", err)
	}
	return nil
}

func (a *App) UnlockScreen() error {
	a.locker.Unlock()
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	wailsruntime.WindowUnfullscreen(a.ctx)
	wailsruntime.EventsEmit(a.ctx, "vnet:machine:unlocked")
	return nil
}

func (a *App) IsLocked() bool {
	return a.locker.Locked()
}

func (a *App) ShutdownMachine() (string, error) {
	if err := shutdownMachine(); err != nil {
		return "", err
	}
	return "ok", nil
}

func (a *App) RestartMachine() (string, error) {
	if err := restartMachine(); err != nil {
		return "", err
	}
	return "ok", nil
}

func (a *App) ShowMessage(title, message string) (string, error) {
	a.locker.ShowMessage(title, message)
	return "ok", nil
}

func (a *App) ExecuteCommand(command string) (string, error) {
	cmd := exec.Command("cmd", "/C", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), fmt.Errorf("exec error: %w: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (a *App) BlockApp(processName string) error   { return blockAppByName(processName) }
func (a *App) UnblockApp(processName string) error { return unblockAppByName(processName) }
