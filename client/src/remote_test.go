package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Payload dưới đây được chép nguyên văn từ chuỗi backend gửi thật, bắt được
// bằng một máy trạm giả nối vào /api/ws/client. Lồng hai lớp là chỗ dễ sai:
// "data" của sự kiện lại chứa một khoá "payload" nữa bên trong.
const capturedLockEvent = `{"type":"remote:lock","data":{"action":"lock","machine_code":"RC-01","machine_id":"79376323-4f1d-4fb7-b995-8a33e5c8b2dd","payload":{"reason":"Hết giờ chơi, vui lòng ra quầy"}}}`

const capturedMessageEvent = `{"type":"remote:message","data":{"action":"message","machine_code":"RC-01","machine_id":"79376323-4f1d-4fb7-b995-8a33e5c8b2dd","payload":{"message":"Còn 5 phút","title":"VNET"}}}`

const capturedUnlockEvent = `{"type":"remote:unlock","data":{"action":"unlock","machine_code":"RC-01","machine_id":"79376323-4f1d-4fb7-b995-8a33e5c8b2dd","payload":{}}}`

func parse(t *testing.T, raw string) WSMessage {
	t.Helper()
	var msg WSMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("không đọc được sự kiện: %v", err)
	}
	return msg
}

func TestRemoteStringField(t *testing.T) {
	lock := parse(t, capturedLockEvent)
	if lock.Type != "remote:lock" {
		t.Errorf("loại sự kiện = %q, mong remote:lock", lock.Type)
	}
	if got := remoteStringField(lock, "reason"); got != "Hết giờ chơi, vui lòng ra quầy" {
		t.Errorf("lý do khoá = %q", got)
	}

	msg := parse(t, capturedMessageEvent)
	if got := remoteStringField(msg, "title"); got != "VNET" {
		t.Errorf("tiêu đề = %q", got)
	}
	if got := remoteStringField(msg, "message"); got != "Còn 5 phút" {
		t.Errorf("nội dung = %q", got)
	}
}

func TestRemoteStringField_MissingIsEmptyNotPanic(t *testing.T) {
	// Lệnh mở khoá không mang trường nào; đọc trường vắng mặt phải trả rỗng.
	unlock := parse(t, capturedUnlockEvent)
	if got := remoteStringField(unlock, "reason"); got != "" {
		t.Errorf("trường vắng mặt trả %q, mong rỗng", got)
	}

	// Dữ liệu hỏng cũng không được làm sập máy khách.
	broken := WSMessage{Type: "remote:lock", Payload: json.RawMessage(`{lỗi`)}
	if got := remoteStringField(broken, "reason"); got != "" {
		t.Errorf("payload hỏng trả %q, mong rỗng", got)
	}

	// Trường có kiểu sai (số thay vì chuỗi) cũng vậy.
	wrongType := WSMessage{Type: "remote:lock", Payload: json.RawMessage(`{"payload":{"reason":42}}`)}
	if got := remoteStringField(wrongType, "reason"); got != "" {
		t.Errorf("trường sai kiểu trả %q, mong rỗng", got)
	}
}

// Danh sách lệnh máy khách đăng ký phải khớp remoteActions bên backend. Lệch
// nhau là cả chuỗi đứt lặng lẽ — đúng lỗi mà đợt này đang sửa.
func TestRegisteredRemoteHandlersMatchBackend(t *testing.T) {
	backendActions := []string{"lock", "unlock", "shutdown", "restart", "message", "block-app", "unblock-app"}

	c := &WSClient{handlers: make(map[string]WSHandler)}
	(&App{locker: NewScreenLocker()}).registerRemoteHandlers(c)

	for _, a := range backendActions {
		if _, ok := c.handlers["remote:"+a]; !ok {
			t.Errorf("máy khách thiếu handler cho remote:%s", a)
		}
	}
	for name := range c.handlers {
		found := false
		for _, a := range backendActions {
			if "remote:"+a == name {
				found = true
			}
		}
		if !found {
			t.Errorf("máy khách đăng ký %q mà backend không gửi", name)
		}
	}
}

// parsePresetValues phải nhận cả hai dạng lưu: `{"values": [...]}` mà seed đang
// dùng, và mảng trần. Bản cũ chỉ mong mảng trần nên luôn rơi về danh sách cứng.
func TestParsePresetValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []int64
	}{
		{"dạng seed", `{"values": [5000, 10000, 20000]}`, []int64{5000, 10000, 20000}},
		{"mảng trần", `[1000, 2000]`, []int64{1000, 2000}},
		{"rỗng", `{"values": []}`, nil},
		{"hỏng", `khong-phai-json`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parsePresetValues(c.raw)
			if len(got) != len(c.want) {
				t.Fatalf("số phần tử: nhận %v, mong %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("phần tử %d: nhận %d, mong %d", i, got[i], c.want[i])
				}
			}
		})
	}
}

// GetTopupPresets phải hỏi ĐÚNG nhóm/khoá máy chủ đang lưu. Bản cũ hỏi nhóm
// "general" khoá "topup_presets" nên luôn rơi về danh sách cứng — quán đổi mệnh
// giá trong trang Cài đặt mà máy khách vẫn hiện mức cũ.
func TestGetTopupPresetsReadsServerValue(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"message":"success","data":[
			{"group_name":"topup","key":"presets","value":"{\"values\": [7000, 21000]}"}
		]}`))
	}))
	defer srv.Close()

	app := &App{cfg: &Config{ServerURL: srv.URL}}
	out, err := app.GetTopupPresets()
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if gotPath != "/api/settings/topup" {
		t.Fatalf("gọi nhầm đường dẫn: %s", gotPath)
	}
	if out != `[7000,21000]` {
		t.Fatalf("nhận %s, mong [7000,21000]", out)
	}
}

// Máy chủ không có nhóm topup thì vẫn phải có mệnh giá để khách bấm.
func TestGetTopupPresetsFallsBackWhenMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"message":"success","data":[]}`))
	}))
	defer srv.Close()

	app := &App{cfg: &Config{ServerURL: srv.URL}}
	out, _ := app.GetTopupPresets()
	if !strings.Contains(out, "5000") {
		t.Fatalf("không rơi về mặc định: %s", out)
	}
}
