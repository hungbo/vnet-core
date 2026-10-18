package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// serviceName là tên dịch vụ trong services.msc. Bộ cài và lệnh gỡ đều dùng nó.
const (
	serviceName = "VNETClient"
	// Tên hiển thị: bộ lọc sự kiện của tác vụ canh chừng so theo chuỗi này, nên
	// đổi nó ở một chỗ mà quên chỗ kia là tác vụ im lặng không bao giờ kích.
	serviceDisplayName = "VNET Client Agent"
)

// runAgent là phần việc nền, KHÔNG phụ thuộc Windows.
//
// Trước đây toàn bộ những goroutine này khởi động trong App.Startup, tức là sống
// chết theo cửa sổ giao diện: khách tắt cửa sổ là máy im lặng hoàn toàn với máy
// chủ, không nhịp tim, không nhận lệnh nào. Tách ra đây để chúng chạy từ lúc bật
// máy, kể cả khi chưa ai đăng nhập.
//
// Trả về khi ctx bị huỷ.
func runAgent(ctx context.Context) {
	// Mốc dịch vụ bắt đầu, và báo cáo khởi động nạp lại nếu dịch vụ vừa được bật lại
	// giữa lần bật máy này (bootreport.go). Trước lượt đọc trước: lượt đó cũng ghi
	// vào báo cáo. Chỉ đọc một tệp nhỏ cạnh .exe.
	bootRec.dichVuBatDau(bootFilePath(), time.Now(), systemUptime())
	// Việc đầu tiên: đọc trước tệp giao diện sắp cần, trong vài giây trước khi
	// Windows tự đăng nhập và dịch vụ bật giao diện (prefetch.go).
	go runBootPrefetch(ctx)
	// Trước lần ngủ đầu tiên của Lightning Boot: thức dậy thẳng vào màn khoá VNET
	// chứ không qua màn khoá Windows (wakelock.go).
	go tatKhoaKhiThuc()

	cfg := LoadConfig()
	log.Printf("[agent] máy %s → %s", cfg.MachineCode, cfg.ServerURL)

	// Bộ giám sát phải có TRƯỚC khi nhịp tim và WebSocket chạy: cả hai báo vào nó.
	loadPolicyFile()

	// Danh sách chặn lấy từ chính sách đã lưu, nên áp ngay từ lúc bật máy kể cả
	// khi chưa nối được máy chủ. Nhịp tim mang bản mới về thì agent.go thay.
	appBlock = newAppBlocker()
	appBlock.setAll(currentPolicy().BlockedApps)
	go appBlock.run(ctx)
	agentSupervisor = newUISupervisor(supNow())
	// Dịch vụ khởi động lại giữa một lần bật máy (tự cập nhật) vẫn phải nhớ máy đã
	// có người dùng: cho ngủ chờ một máy còn dấu vết của khách trước là để lộ tài
	// khoản của họ cho người bấm nút tiếp theo (decide, luật 4).
	if mayDaDung(daDungFilePath(), time.Now(), systemUptime(), vuaTuCapNhat()) {
		agentSupervisor.danhDauDaDung()
	}

	go runTelemetry(ctx, cfg)
	go newWebBlocker(cfg).run(ctx)
	go runAgentWS(ctx, cfg)
	go runUILinkServer(ctx, agentSupervisor)
	go superviseUI(ctx, agentSupervisor)
	// Lối tắt VNET trên desktop và xoá lối tắt rác theo chính sách (shortcuts.go).
	go runShortcutKeeper(ctx)
	// Chép metadata Riot từ Game Disk vào ProgramData (riotmeta.go).
	go runRiotMetaKeeper(ctx)
	if osIsWindows {
		go runAutoUpdate(ctx, cfg)
	}

	<-ctx.Done()
}

// runAgentWS giữ kết nối WebSocket bằng KHOÁ MÁY, không phải token người dùng.
//
// Đây là phần vá lỗ hổng lớn nhất: bản cũ chỉ nối sau khi khách đăng nhập, bằng
// token của khách, nên máy không có ai ngồi thì không có kết nối nào và nhân
// viên KHÔNG tắt hay khoá được máy trống — đúng lúc cần nhất.
//
// Tiến trình nền chỉ xử lý những lệnh làm được ở session 0. Khoá màn hình và
// hiện thông báo là việc của giao diện: hook bàn phím và cửa sổ không tồn tại ở
// session 0. Nhờ hub gửi lệnh tới MỌI kết nối của một mã máy, hai bên cùng nhận
// và mỗi bên bỏ qua phần không thuộc mình.
func runAgentWS(ctx context.Context, cfg *Config) {
	if cfg.MachineCode == "" {
		log.Printf("[agent] chưa khai mã máy — bỏ qua WebSocket, chỉ gửi nhịp tim")
		return
	}

	ws := NewAgentWSClient(ctx, cfg)
	agentWS.Store(ws)
	// Phiên kết thúc vì bất kỳ lý do gì: ghi lại để vòng giám sát kiểm giao
	// diện có thật sự về màn hình khoá không.
	phienKetThuc := func(WSMessage) {
		if agentSupervisor != nil {
			agentSupervisor.noteSessionEnded(supNow())
		}
	}
	ws.On("session:ended", phienKetThuc)
	ws.On("session:auto-ended", phienKetThuc)
	ws.On("curfew:enforced", phienKetThuc)

	ws.On("remote:shutdown", func(WSMessage) { shutdownMachine() })
	ws.On("remote:restart", func(WSMessage) { restartMachine() })
	ws.Run()
}

// remoteField bóc một trường chuỗi khỏi payload lồng hai lớp
// {"data":{"payload":{...}}} mà máy chủ gửi xuống.
func remoteField(msg WSMessage, key string) string {
	var outer struct {
		Payload map[string]interface{} `json:"payload"`
	}
	if err := json.Unmarshal(msg.Payload, &outer); err != nil {
		return ""
	}
	if v, ok := outer.Payload[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// Nhịp của vòng giám sát giao diện.
const (
	// Giao diện đang chạy: hỏi bộ giám sát (decide) mỗi 2 giây.
	checkEvery = 2 * time.Second
	// Giao diện chưa chạy: dò dày để bật nó NGAY khi phiên đăng nhập tự động của
	// Windows có token. Mỗi nhịp chỉ là vài lời gọi Win32 rẻ.
	fastEvery = 200 * time.Millisecond
	// Khoảng cách tối thiểu giữa hai lần bật khi lần trước đã chạy được hoặc hỏng
	// vì lý do thật — để giao diện chết ngay sau khi bật không thành vòng lặp.
	launchEvery = 5 * time.Second
	// Chờ phiên đăng nhập có thể kéo dài cả chục giây: chỉ ghi nhật ký lần đầu,
	// rồi tối đa mỗi chừng này.
	waitLogEvery = 30 * time.Second
)

// errChuaCoPhien: chưa có phiên đăng nhập nào để mượn token. Máy vừa khởi động
// là trạng thái bình thường, không phải lỗi — nên khác mọi lỗi bật giao diện còn
// lại, nó được thử lại ở nhịp dày.
var errChuaCoPhien = errors.New("chưa ai đăng nhập vào Windows")

// nhipGiamSat quyết định một lượt của vòng giám sát: có bật giao diện ngay lượt
// này không, và bao lâu nữa thì kiểm lại.
//
//	dangChay:  giao diện đang chạy.
//	sauLanBat: thời gian kể từ lần bật gần nhất.
//	loiCuoi:   kết quả lần bật gần nhất (nil = đã bật được).
//
// Tách thành hàm thuần vì đây là chỗ cân hai điều ngược nhau: bật giao diện sớm
// nhất có thể, mà không để một giao diện chết liên tục kéo cả dịch vụ quay tít.
func nhipGiamSat(dangChay bool, sauLanBat time.Duration, loiCuoi error) (batNgay bool, cho time.Duration) {
	if dangChay {
		return false, checkEvery
	}
	if errors.Is(loiCuoi, errChuaCoPhien) {
		return true, fastEvery
	}
	return sauLanBat >= launchEvery, fastEvery
}

// superviseUI giữ cho giao diện luôn chạy trong phiên của người dùng.
//
// Đây là lý do tiến trình nền phải là dịch vụ chứ không phải một mục tự khởi
// động thường: khách tắt giao diện trong Task Manager thì phải có ai đó bật lại,
// và cái "ai đó" không được nằm cùng thuyền với nó.
//
// Ngoài việc bật lại giao diện đã chết, vòng này hỏi bộ giám sát mỗi hai giây:
// giao diện còn phản hồi không, phiên kết thúc rồi nó có về màn hình khoá
// không, và mất kết nối máy chủ đã quá ngưỡng chưa.
//
// Lượt đầu chạy ngay không chờ, và khi giao diện chưa chạy thì vòng dò dày (xem
// nhipGiamSat): máy trạm không ổ cứng khởi động xong là desktop hiện ra, mỗi
// giây bật giao diện chậm là thêm một giây khách thấy desktop trước màn hình khoá.
func superviseUI(ctx context.Context, sup *uiSupervisor) {
	var (
		lastLaunch  time.Time
		lastErr     error
		lastWaitLog time.Time
		wait        time.Duration // lượt đầu: 0 = kiểm ngay
		lastTick    time.Time
		lastAwake   time.Duration
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		now := time.Now()
		// Phát hiện máy vừa ngủ dậy, phòng khi thông báo nguồn của Windows
		// (service_windows.go) không tới. Bộ giám sát không cần việc này để chấm
		// đúng — nó chạy theo supNow, đồng hồ không tính lúc ngủ — nhưng WebSocket
		// và nhịp tim thì cần được giục (onResume).
		awake := awakeTime()
		if !lastTick.IsZero() && awake > 0 && lastAwake > 0 {
			if ngu := daNgu(now.Sub(lastTick), awake-lastAwake); ngu > 0 {
				log.Printf("[nguồn] máy đã ngủ khoảng %ds giữa hai lượt giám sát", int(ngu.Seconds()))
				onResume()
			}
		}
		lastTick, lastAwake = now, awake

		running := uiIsRunning()
		// Phần giám sát chỉ có nghĩa trên Windows: nơi khác uiIsRunning luôn
		// báo "đang chạy" mà không có giao diện nào báo cáo về.
		if osIsWindows {
			sup.noteUIRunning(supNow(), running)
		}

		launch, next := nhipGiamSat(running, now.Sub(lastLaunch), lastErr)
		wait = next
		if !running {
			if !launch {
				continue
			}
			lastLaunch = now
			lastErr = launchUIInUserSession()
			switch {
			case lastErr == nil:
			case errors.Is(lastErr, errChuaCoPhien):
				// Chưa ai đăng nhập vào Windows là trạng thái bình thường, không
				// phải lỗi — đừng làm ngập nhật ký ở nhịp 200 ms.
				if now.Sub(lastWaitLog) >= waitLogEvery {
					lastWaitLog = now
					log.Printf("[agent] chưa bật được giao diện: %v", lastErr)
				}
			default:
				log.Printf("[agent] chưa bật được giao diện: %v", lastErr)
			}
			continue
		}
		if !osIsWindows {
			continue
		}

		switch sup.decide(supNow(), currentPolicy(), maintenanceMode()) {
		case actRestartUI:
			log.Printf("[giám sát] giao diện không phản hồi hoặc không về màn hình khoá — bật lại")
			tatGiaoDienCu()
		case actReboot:
			log.Printf("[giám sát] khởi động lại máy: giao diện hỏng lặp lại, hoặc mất kết nối máy chủ quá ngưỡng")
			if err := restartMachine(); err != nil {
				log.Printf("[giám sát] không khởi động lại được: %v", err)
			}
		case actShutdown:
			log.Printf("[giám sát] không ai đăng nhập quá %d phút — tắt máy", currentPolicy().IdleShutdownMinutes)
			if err := shutdownMachine(); err != nil {
				log.Printf("[giám sát] không tắt được máy: %v", err)
			}
		case actSleep:
			log.Printf("[giám sát] không ai đăng nhập quá %d phút, từ lúc bật chưa ai dùng — cho máy ngủ chờ", currentPolicy().IdleShutdownMinutes)
			// SetSuspendState chỉ trả về khi máy đã thức dậy: chạy riêng để vòng
			// giám sát không đứng theo nó.
			go func() {
				if err := sleepMachine(); err != nil {
					log.Printf("[giám sát] không cho máy ngủ được: %v", err)
				}
			}()
		case actIdleRestart:
			log.Printf("[giám sát] không ai đăng nhập quá %d phút — khởi động lại cho sạch rồi mới ngủ chờ", currentPolicy().IdleShutdownMinutes)
			if err := restartMachine(); err != nil {
				log.Printf("[giám sát] không khởi động lại được: %v", err)
			}
		}
	}
}

// agentSupervisor chỉ có trong tiến trình DỊCH VỤ NỀN; ở giao diện nó là nil.
// Nhịp tim dùng chung cho cả hai tiến trình nên phải hỏi biến này mới biết mình
// đang chạy ở đâu.
var agentSupervisor *uiSupervisor

// agentWS: kết nối WebSocket bằng mã máy của dịch vụ nền, để onResume nối lại nó.
var agentWS atomic.Pointer[WSClient]

// sleepDetectMin: giữa hai lượt giám sát (cách 0,2–2 s), đồng hồ thường chạy hơn
// đồng hồ "thức" chừng này thì máy đã ngủ giữa chừng. Lệch do làm tròn chỉ cỡ ms.
const sleepDetectMin = 5 * time.Second

// daNgu: khoảng máy đã ngủ giữa hai lượt, 0 nếu không ngủ. dThuong đo bằng đồng hồ
// Go (tính cả lúc ngủ), dThuc bằng awakeTime (không tính). Khác với "hai lượt
// cách nhau lâu": tiến trình bị chậm hay treo thì cả hai đồng hồ cùng chạy.
func daNgu(dThuong, dThuc time.Duration) time.Duration {
	if d := dThuong - dThuc; d >= sleepDetectMin {
		return d
	}
	return 0
}

// Đồng hồ của bộ giám sát giao diện (supNow).
var (
	supClockMu    sync.Mutex
	supClockEpoch time.Time // zero = chưa lấy mốc
	supClockAwake bool      // đọc được đồng hồ thức không
	supClockLast  time.Duration
	awakeClock    = awakeTime // test thay được
)

// supNow là "bây giờ" của bộ giám sát giao diện: đồng hồ chỉ chạy khi máy THỨC.
//
// Đồng hồ Go trên Windows tính cả lúc ngủ. Lightning Boot (闪电开机) của Cloud
// Update cho máy boot sẵn rồi ngủ S3, có khi cả đêm; đo bằng đồng hồ Go thì lúc
// khách bấm nút, giao diện đã "im lặng" cả đêm (bị bật lại), máy "không ai đăng
// nhập" cả đêm (bị TẮT ngay trước mặt khách) và "mất máy chủ" cả đêm. Đo bằng
// đồng hồ thức thì giấc ngủ không có thật với bộ giám sát: không ai phải đặt lại
// mốc nào lúc thức dậy, nên cũng không có cuộc đua giữa thông báo nguồn của
// Windows và vòng giám sát. Mọi mốc đưa vào uiSupervisor phải lấy từ đây.
//
// Không đọc được đồng hồ thức (không phải Windows) thì là đồng hồ Go.
func supNow() time.Time {
	supClockMu.Lock()
	defer supClockMu.Unlock()
	a := awakeClock()
	if supClockEpoch.IsZero() {
		supClockAwake = a > 0
		supClockEpoch = time.Now().Add(-a)
	}
	if !supClockAwake {
		return time.Now()
	}
	if a < supClockLast { // đọc hỏng (0) thì đứng yên, không lùi
		a = supClockLast
	}
	supClockLast = a
	return supClockEpoch.Add(a)
}

var (
	resumeMu     sync.Mutex
	lastResumeAt time.Time
)

// onResume: máy vừa thức dậy từ chế độ ngủ. Hai đường cùng gọi vào đây: thông
// báo nguồn của Windows và vòng giám sát tự phát hiện (superviseUI); lần thứ
// hai trong 10 giây là cùng một lần thức, bỏ qua.
//
// Đồng hồ giám sát không cần đặt lại (supNow). Ở đây chỉ là những việc làm một
// lần lúc thức:
//   - giữ khoá ngoại tuyến tới khi máy chủ trả lời lại (noteResume): hội viên
//     còn đăng nhập từ trước lúc ngủ thì phiên có thể đã bị máy chủ đóng;
//   - nối lại WebSocket: máy chủ đã đóng kết nối cũ trong lúc máy ngủ;
//   - gửi nhịp tim ngay: máy chủ coi máy im quá 2 phút là đã rời và đóng mọi
//     phiên mở trên đó, kể cả phiên khách vừa đăng nhập sau khi thức.
func onResume() {
	resumeMu.Lock()
	now := time.Now()
	if !lastResumeAt.IsZero() && now.Sub(lastResumeAt) < 10*time.Second {
		resumeMu.Unlock()
		return
	}
	lastResumeAt = now
	if sup := agentSupervisor; sup != nil {
		sup.noteResume(supNow())
	}
	resumeMu.Unlock()

	log.Printf("[nguồn] máy vừa thức dậy từ chế độ ngủ — nối lại WebSocket, gửi nhịp tim ngay")
	if ws := agentWS.Load(); ws != nil {
		ws.dropConn()
	}
	giucNhipTim()
}

// vuaThucDay: máy thức dậy chưa tới d.
func vuaThucDay(d time.Duration) bool {
	resumeMu.Lock()
	defer resumeMu.Unlock()
	return !lastResumeAt.IsZero() && time.Since(lastResumeAt) < d
}

// runService là điểm vào của chế độ --service.
//
// Trên Windows nó chạy dưới Service Control Manager; ở nơi khác (chỉ dùng khi
// phát triển) nó chạy như một tiến trình thường để còn thử được logic nền.
func runService() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if runAsWindowsService(ctx, cancel) {
		return
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	runAgent(ctx)
}
