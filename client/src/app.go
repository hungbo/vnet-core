package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type LoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	MachineID string `json:"machine_id,omitempty"`
}

type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	User         *UserInfo `json:"user"`
	SessionID    string    `json:"session_id,omitempty"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	StoreID  string `json:"store_id"`
}

type SessionInfo struct {
	ID               string     `json:"id"`
	MachineID        string     `json:"machine_id"`
	MachineCode      string     `json:"machine_code"`
	MemberID         string     `json:"member_id"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
	DurationMinutes  *int       `json:"duration_minutes"`
	RemainingMinutes *int       `json:"remaining_minutes"`
	ComboType        string     `json:"combo_type"`
	SlotEnd          *time.Time `json:"slot_end"`
	TotalCost        *int64     `json:"total_cost"`
	IsActive         bool       `json:"is_active"`
	ComboName        string     `json:"combo_name"`
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
	storeID     string
	machineCode string
	locker      *ScreenLocker
	wsClient    *WSClient
	wsCtx       context.Context
	wsCancel    context.CancelFunc
	watchProc   *os.Process
	cancel      context.CancelFunc
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

	aCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	go runHeartbeat(aCtx, a.cfg)
	go runMonitor(aCtx, a.cfg)
	go runWatchdog(aCtx, a.cfg, a.locker, &[]string{}, new(bool))

	a.spawnWatch()
}

func (a *App) spawnWatch() {
	cmd := exec.Command(os.Args[0], "--watch",
		fmt.Sprintf("%d", os.Getpid()),
		a.machineCode,
		a.cfg.ServerURL,
	)
	if err := cmd.Start(); err != nil {
		log.Printf("spawn watch: %v", err)
		return
	}
	a.watchProc = cmd.Process
}

func (a *App) killWatch() {
	if a.watchProc != nil {
		a.watchProc.Kill()
		a.watchProc = nil
	}
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
		return nil, fmt.Errorf("request failed: %v", err)
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
	go func() {
		if err := a.wsClient.Connect(a.wsCtx); err != nil {
			log.Printf("ws client: %v", err)
		}
	}()
}

func (a *App) Login(username, password string) (string, error) {
	req := LoginRequest{
		Username:  username,
		Password:  password,
		MachineID: a.machineCode,
	}

	data, err := a.doRequest("POST", "/api/auth/member-login", req)
	if err != nil {
		return "", err
	}

	var resp LoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("invalid response")
	}

	a.token = resp.AccessToken
	a.userID = resp.User.ID
	a.username = resp.User.Username
	a.fullName = resp.User.FullName
	a.role = resp.User.Role
	a.storeID = resp.User.StoreID

	if err := a.locker.Lock(); err != nil {
		log.Printf("lock screen error: %v", err)
	}

	a.connectWS()

	return string(data), nil
}

func (a *App) LoginAdmin(username, password string) (string, error) {
	req := LoginRequest{
		Username:  username,
		Password:  password,
		MachineID: a.machineCode,
	}

	data, err := a.doRequest("POST", "/api/auth/login", req)
	if err != nil {
		return "", err
	}

	var resp LoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("invalid response")
	}

	a.token = resp.AccessToken
	a.userID = resp.User.ID
	a.username = resp.User.Username
	a.fullName = resp.User.FullName
	a.role = resp.User.Role
	a.storeID = resp.User.StoreID

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
	a.storeID = ""
	if a.wsCancel != nil {
		a.wsCancel()
	}
	return nil
}

func (a *App) RestoreSession(token, userID, username, fullName, role, storeID string) error {
	a.token = token
	a.userID = userID
	a.username = username
	a.fullName = fullName
	a.role = role
	a.storeID = storeID
	a.connectWS()
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

func (a *App) SetMachineCode(code string) {
	a.machineCode = code
}

func (a *App) GetMemberInfo() (string, error) {
	data, err := a.doRequest("GET", "/api/members/"+a.userID, nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetSession() (string, error) {
	data, err := a.doRequest("GET", "/api/sessions/active", nil)
	if err != nil {
		return "", err
	}

	var paginated PaginatedData
	if err := json.Unmarshal(data, &paginated); err != nil {
		return string(data), nil
	}

	var sessions []SessionInfo
	if err := json.Unmarshal(paginated.Items, &sessions); err != nil {
		return string(paginated.Items), nil
	}

	for _, s := range sessions {
		if s.MemberID == a.userID && s.IsActive {
			sessionData, _ := json.Marshal(s)
			return string(sessionData), nil
		}
	}

	return "null", nil
}

func (a *App) GetMenu(categoryId string) (string, error) {
	path := "/api/products"
	if categoryId != "" {
		path += "?category_id=" + categoryId
	}
	data, err := a.doRequest("GET", path, nil)
	if err != nil {
		return "", err
	}

	var paginated PaginatedData
	if err := json.Unmarshal(data, &paginated); err != nil {
		return string(data), nil
	}
	return string(paginated.Items), nil
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

func (a *App) GetTopupPresets() (string, error) {
	type settingItem struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	data, err := a.doRequest("GET", "/api/settings/general", nil)
	if err != nil {
		return a.defaultPresets(), nil
	}

	var settings []settingItem
	if err := json.Unmarshal(data, &settings); err != nil {
		return a.defaultPresets(), nil
	}

	for _, s := range settings {
		if s.Key == "topup_presets" && s.Value != "" {
			return s.Value, nil
		}
	}

	return a.defaultPresets(), nil
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
		"room_id":         roomID,
		"sender_type":     "member",
		"sender_id":       a.userID,
		"message":         message,
		"message_type":    "text",
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

func (a *App) TakeScreenshot() (string, error) {
	return "", nil
}

func (a *App) SendScreenshotMessage(roomID, imageData string) (string, error) {
	reqBody := map[string]string{
		"room_id": roomID,
		"sender_type":     "member",
		"sender_id":       a.userID,
		"message":         imageData,
		"message_type":    "screenshot",
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

func (a *App) ShutdownMachine() (string, error) {
	if err := exec.Command("shutdown", "/s", "/t", "5").Run(); err != nil {
		return "", err
	}
	return "ok", nil
}

func (a *App) RestartMachine() (string, error) {
	if err := exec.Command("shutdown", "/r", "/t", "5").Run(); err != nil {
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

func (a *App) BlockApp(processName string) error {
	name := strings.TrimSuffix(processName, ".exe")
	ruleName := fmt.Sprintf("VNET_Block_%s", name)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", ruleName),
		"dir=out",
		fmt.Sprintf("program=%%ProgramFiles%%\\%s.exe", name),
		"action=block",
		"enable=yes",
	)
	return cmd.Run()
}

func (a *App) UnblockApp(processName string) error {
	name := strings.TrimSuffix(processName, ".exe")
	ruleName := fmt.Sprintf("VNET_Block_%s", name)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s", ruleName),
	)
	return cmd.Run()
}
