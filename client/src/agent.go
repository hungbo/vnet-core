package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// Một gói tin duy nhất cho mọi lần báo cáo.
//
// Trước đây có HAI struct gửi tới cùng một endpoint: gói nhịp tim có ip/mac
// nhưng không có uptime, gói giám sát thì ngược lại. Máy chủ ghi đè vô điều
// kiện, nên cứ mỗi phút gói giám sát lại xoá trắng IP và MAC của máy. Hai lỗi
// đó không phải hai lỗi riêng — chúng là cùng một lỗi: hai hình dạng dữ liệu
// cho cùng một việc.
type TelemetryPayload struct {
	MachineCode string  `json:"machine_code"`
	CPUTemp     float64 `json:"cpu_temp"`
	GPUTemp     float64 `json:"gpu_temp"`
	IP          string  `json:"ip"`
	MAC         string  `json:"mac"`
	CPUUsage    float64 `json:"cpu_usage"`
	RAMUsage    float64 `json:"ram_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	Uptime      uint64  `json:"uptime"`
	Timestamp   string  `json:"timestamp"`

	// Cấu hình máy, đo một lần lúc khởi động rồi gửi kèm mỗi lần báo.
	// Gửi kèm chứ không gửi một lần duy nhất: gửi một lần thì máy chủ đang tắt
	// lúc đó là mất luôn, và nâng RAM xong sẽ không có gì cập nhật lại.
	CPUName   string `json:"cpu_name,omitempty"`
	GPUName   string `json:"gpu_name,omitempty"`
	RAMGB     int    `json:"ram_gb,omitempty"`
	StorageGB int    `json:"storage_gb,omitempty"`
	OSInfo    string `json:"os_info,omitempty"`
	// Thiết bị ngoại vi đang cắm, cũng đo một lần lúc khởi động.
	Peripherals []string `json:"peripherals,omitempty"`

	// Hai dấu hiệu chỉ DỊCH VỤ NỀN khai (giao diện để nil): tài khoản Windows
	// đang đăng nhập có quyền quản trị không, và card mạng xuất hiện sau khi máy
	// đã chạy. Hai tiến trình cùng khai là hai bản không khớp nhau đè lên nhau.
	UserIsAdmin  *bool   `json:"user_is_admin,omitempty"`
	ExtraNetwork *string `json:"extra_network,omitempty"`
}

// runTelemetry báo cáo định kỳ: giữ máy ở trạng thái online, cập nhật nhiệt độ
// và địa chỉ mạng, đồng thời ghi một dòng vào lịch sử phần cứng.
//
// Một vòng lặp, không phải hai. Bản cũ chạy runHeartbeat 15 giây và runMonitor
// 60 giây, cả hai gửi tới cùng một endpoint với hai gói tin khác nhau — tức là
// năm dòng lịch sử mỗi phút cho mỗi máy, và một lỗi xoá trắng IP.
// dichVuTraLoiLuc: lần cuối dịch vụ nền trả lời giao diện qua kênh nối (Unix
// nano). Giao diện dùng nó để biết có nên tự gửi số đo hay không.
var dichVuTraLoiLuc atomic.Int64

// dangNoiDichVu: giao diện vừa nói chuyện được với dịch vụ nền trong 10 giây.
func dangNoiDichVu() bool {
	t := dichVuTraLoiLuc.Load()
	return t != 0 && time.Since(time.Unix(0, t)) < 10*time.Second
}

func runTelemetry(ctx context.Context, cfg *Config) {
	// Cấu hình máy đo MỘT lần: trên Windows phải gọi ra ngoài hệ thống mới lấy
	// được tên GPU, làm việc đó mỗi 15 giây là tự bắn vào chân mình.
	specs := readMachineSpecs()
	log.Printf("cấu hình máy: CPU=%q GPU=%q RAM=%dGB Ổ đĩa=%dGB HĐH=%q ngoại vi=%d",
		specs.CPUName, specs.GPUName, specs.RAMGB, specs.StorageGB, specs.OSInfo, len(specs.Peripherals))
	// Một dòng để kiểm nhanh trên máy thật: 0 nghĩa là Windows không cung cấp
	// cảm biến đó cho tiến trình này (máy ảo, driver cũ, bo mạch không báo).
	log.Printf("nhiệt độ lần đầu: CPU=%.1f°C GPU=%.1f°C", getCPUTemp(), getGPUTemp())

	nics := &nicWatcher{}

	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Dịch vụ nền (SYSTEM) và giao diện cùng chạy vòng này. Cả hai cùng
			// gửi là mỗi 15 giây hai dòng số đo, và vì giao diện chạy bằng quyền
			// của khách nên không đọc được cảm biến ACPI: cột nhiệt độ nhảy qua
			// lại giữa số thật và 0. Còn nối được dịch vụ nền thì để nó gửi;
			// dịch vụ chết thì giao diện tự gửi như cũ.
			if agentSupervisor == nil && dangNoiDichVu() {
				continue
			}
			ip, mac := getNetworkInfo()
			payload := TelemetryPayload{
				MachineCode: cfg.MachineCode,
				CPUTemp:     getCPUTemp(),
				GPUTemp:     getGPUTemp(),
				IP:          ip,
				MAC:         mac,
				CPUUsage:    getCPUUsage(),
				RAMUsage:    getRAMUsage(),
				DiskUsage:   getDiskUsage(),
				Uptime:      getUptime(),
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				CPUName:     specs.CPUName,
				GPUName:     specs.GPUName,
				RAMGB:       specs.RAMGB,
				StorageGB:   specs.StorageGB,
				OSInfo:      specs.OSInfo,
				Peripherals: specs.Peripherals,
			}
			if agentSupervisor != nil {
				payload.UserIsAdmin = consoleUserIsAdmin()
				payload.ExtraNetwork = nics.extra(listActiveNICs())
			}
			data, err := json.Marshal(payload)
			if err != nil {
				log.Printf("báo cáo: không đóng gói được dữ liệu: %v", err)
				continue
			}
			resp, err := postHeartbeat(cfg, data)
			if err != nil {
				log.Printf("báo cáo: %v", err)
				continue
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				log.Printf("báo cáo: máy chủ trả %d — kiểm tra mã máy trong config.json", resp.StatusCode)
				continue
			}
			nics.setBaselineOnce(listActiveNICs())
			onHeartbeatOK(body)
			applyVNCFromHeartbeat(cfg, body)
		}
	}
}

func postHeartbeat(cfg *Config, body []byte) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/machines/by-code/%s/heartbeat", cfg.ServerURL, cfg.MachineCode)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return httpClient.Do(req)
}

type Watchdog struct {
	locker       *ScreenLocker
	blockedApps  []string
	lockRequired bool
}

// runWatchdog cảnh báo nhiệt độ cao lên màn hình khách.
//
// Lưu ý: blockedApps hiện chưa có gì ghi vào (Startup truyền con trỏ tới giá trị
// rỗng), nên nhánh dùng nó chưa bao giờ chạy — chặn ứng dụng thật nằm ở dịch
// vụ nền (appblock.go).
func runWatchdog(ctx context.Context, cfg *Config, locker *ScreenLocker, blockedApps *[]string) {
	ticker := time.NewTicker(cfg.WatchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, app := range *blockedApps {
				terminateIfRunning(app)
			}
			cpuTemp := getCPUTemp()
			gpuTemp := getGPUTemp()
			if cpuTemp > cfg.HighTempThreshold {
				log.Printf("watchdog: high CPU temp: %.1f°C", cpuTemp)
				locker.ShowMessage("VNET — Cảnh báo",
					fmt.Sprintf("CPU đang quá nóng (%.1f°C). Báo quầy kiểm tra tản nhiệt.", cpuTemp))
			}
			if gpuTemp > cfg.HighTempThreshold {
				log.Printf("watchdog: high GPU temp: %.1f°C", gpuTemp)
				locker.ShowMessage("VNET — Cảnh báo",
					fmt.Sprintf("Card đồ hoạ đang quá nóng (%.1f°C). Báo quầy kiểm tra tản nhiệt.", gpuTemp))
			}
		}
	}
}

func terminateIfRunning(processName string) {
	name := strings.TrimSuffix(processName, ".exe")
	procs, err := process.Processes()
	if err != nil {
		return
	}
	for _, p := range procs {
		pname, err := p.Name()
		if err != nil {
			continue
		}
		if strings.EqualFold(strings.TrimSuffix(pname, ".exe"), name) {
			p.Kill()
		}
	}
}

func getCPUTemp() float64 {
	stat, err := host.SensorsTemperatures()
	if err != nil {
		return 0
	}
	for _, s := range stat {
		if s.Temperature > 0 {
			return s.Temperature
		}
	}
	return 0
}

// getGPUTemp tìm đúng cảm biến GPU. Bản cũ sao chép nguyên getCPUTemp nên
// nhiệt độ GPU luôn bằng nhiệt độ CPU. Không tìm thấy cảm biến thì trả 0 —
// thà không có số còn hơn báo một con số của bộ phận khác.
func getGPUTemp() float64 {
	if t := readGPUTempD3DKMT(); t > 0 {
		return t
	}
	stat, err := host.SensorsTemperatures()
	if err != nil {
		return 0
	}
	for _, s := range stat {
		key := strings.ToLower(s.SensorKey)
		isGPU := strings.Contains(key, "gpu") ||
			strings.Contains(key, "amdgpu") ||
			strings.Contains(key, "radeon") ||
			strings.Contains(key, "nouveau") ||
			strings.Contains(key, "nvidia")
		if isGPU && s.Temperature > 0 {
			return s.Temperature
		}
	}
	return 0
}

func getCPUUsage() float64 {
	percent, err := cpu.Percent(100*time.Millisecond, false)
	if err != nil || len(percent) == 0 {
		return 0
	}
	return math.Round(percent[0]*100) / 100
}

func getRAMUsage() float64 {
	stat, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return math.Round(stat.UsedPercent*100) / 100
}

func getDiskUsage() float64 {
	// systemDiskRoot() chứ không phải "/": trên Windows đường dẫn đó không trỏ
	// tới đâu cả, nên phần trăm ổ đĩa gửi lên có thể luôn bằng 0 mà không báo lỗi.
	stat, err := disk.Usage(systemDiskRoot())
	if err != nil {
		return 0
	}
	return math.Round(stat.UsedPercent*100) / 100
}

func getUptime() uint64 {
	uptime, err := host.Uptime()
	if err != nil {
		return 0
	}
	return uptime
}

func getNetworkInfo() (ip, mac string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
				continue
			}
			return ipnet.IP.String(), iface.HardwareAddr.String()
		}
	}
	return "", ""
}

// onHeartbeatOK xử lý phản hồi nhịp tim thành công: ghi nhận còn nói chuyện được
// với máy chủ, và nhận chính sách bảo vệ máy chủ gửi kèm.
func onHeartbeatOK(body []byte) {
	if agentSupervisor != nil {
		agentSupervisor.noteServerOK(time.Now())
	}
	var resp struct {
		Data struct {
			Policy *clientPolicy `json:"policy"`
		} `json:"data"`
	}
	// Máy chủ bản cũ không gửi chính sách: giữ nguyên bản đang có.
	if json.Unmarshal(body, &resp) != nil || resp.Data.Policy == nil {
		return
	}
	if setPolicy(*resp.Data.Policy) && agentSupervisor != nil {
		p := currentPolicy()
		log.Printf("[chính sách] can thiệp→%s, khoá sau %ds, khởi động lại sau %ds, tắt máy khi không ai dùng sau %d phút, chặn ứng dụng [%s], ẩn lối tắt [%s]",
			p.TamperAction, p.OfflineLockSeconds, p.OfflineRebootSeconds, p.IdleShutdownMinutes, p.BlockedApps, p.HiddenShortcuts)
		savePolicyFile(p)
		if appBlock != nil {
			appBlock.setAll(p.BlockedApps)
		}
	}
}
