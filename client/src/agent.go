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
}

// runTelemetry báo cáo định kỳ: giữ máy ở trạng thái online, cập nhật nhiệt độ
// và địa chỉ mạng, đồng thời ghi một dòng vào lịch sử phần cứng.
//
// Một vòng lặp, không phải hai. Bản cũ chạy runHeartbeat 15 giây và runMonitor
// 60 giây, cả hai gửi tới cùng một endpoint với hai gói tin khác nhau — tức là
// năm dòng lịch sử mỗi phút cho mỗi máy, và một lỗi xoá trắng IP.
func runTelemetry(ctx context.Context, cfg *Config) {
	// Cấu hình máy đo MỘT lần: trên Windows phải gọi ra ngoài hệ thống mới lấy
	// được tên GPU, làm việc đó mỗi 15 giây là tự bắn vào chân mình.
	specs := readMachineSpecs()
	log.Printf("cấu hình máy: CPU=%q GPU=%q RAM=%dGB Ổ đĩa=%dGB",
		specs.CPUName, specs.GPUName, specs.RAMGB, specs.StorageGB)

	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
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
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				log.Printf("báo cáo: máy chủ trả %d — kiểm tra VNET_MACHINE_CODE", resp.StatusCode)
			}
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

// runWatchdog nhận lockScreen thay vì tự gọi locker.Lock(): khoá bây giờ gồm
// hai nửa — phủ màn hình và chặn phím. Gọi mỗi locker.Lock() sẽ khoá bàn phím
// mà không hiện gì lên, khách sẽ ngồi trước một máy không gõ được và không hiểu
// vì sao.
//
// Lưu ý: lockRequired và blockedApps hiện chưa có gì ghi vào (Startup truyền
// con trỏ tới giá trị rỗng), nên hai nhánh dùng chúng chưa bao giờ chạy.
func runWatchdog(ctx context.Context, cfg *Config, locker *ScreenLocker, blockedApps *[]string, lockRequired *bool, lockScreen func(reason string) error) {
	ticker := time.NewTicker(cfg.WatchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if *lockRequired && cfg.ScreenLockEnabled {
				log.Println("watchdog: locking screen")
				if err := lockScreen("watchdog"); err != nil {
					log.Printf("watchdog: khoá máy thất bại: %v", err)
				}
			}
			for _, app := range *blockedApps {
				terminateIfRunning(app)
			}
			cpuTemp := getCPUTemp()
			gpuTemp := getGPUTemp()
			if cpuTemp > cfg.HighTempThreshold {
				log.Printf("watchdog: high CPU temp: %.1f°C", cpuTemp)
				locker.ShowMessage("VNET Alert",
					fmt.Sprintf("CPU temperature is too high (%.1f°C). Please check cooling.", cpuTemp))
			}
			if gpuTemp > cfg.HighTempThreshold {
				log.Printf("watchdog: high GPU temp: %.1f°C", gpuTemp)
				locker.ShowMessage("VNET Alert",
					fmt.Sprintf("GPU temperature is too high (%.1f°C). Please check cooling.", gpuTemp))
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
