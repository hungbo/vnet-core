package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type HeartbeatPayload struct {
	MachineCode string  `json:"machine_code"`
	CPUTemp     float64 `json:"cpu_temp"`
	GPUTemp     float64 `json:"gpu_temp"`
	IP          string  `json:"ip"`
	MAC         string  `json:"mac"`
	CPUUsage    float64 `json:"cpu_usage"`
	RAMUsage    float64 `json:"ram_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	Timestamp   string  `json:"timestamp"`
}

type SnapshotPayload struct {
	MachineCode string  `json:"machine_code"`
	CPUUsage    float64 `json:"cpu_usage"`
	RAMUsage    float64 `json:"ram_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	CPUTemp     float64 `json:"cpu_temp"`
	GPUTemp     float64 `json:"gpu_temp"`
	Uptime      uint64  `json:"uptime"`
	Timestamp   string  `json:"timestamp"`
}

func runHeartbeat(ctx context.Context, cfg *Config) {
	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ip, mac := getNetworkInfo()
			payload := HeartbeatPayload{
				MachineCode: cfg.MachineCode,
				CPUTemp:     getCPUTemp(),
				GPUTemp:     getGPUTemp(),
				IP:          ip,
				MAC:         mac,
				CPUUsage:    getCPUUsage(),
				RAMUsage:    getRAMUsage(),
				DiskUsage:   getDiskUsage(),
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
			}
			data, _ := json.Marshal(payload)
			url := fmt.Sprintf("%s/api/machines/by-code/%s/heartbeat", cfg.ServerURL, cfg.MachineCode)
			resp, err := httpClient.Post(url, "application/json", bytes.NewReader(data))
			if err != nil {
				log.Printf("heartbeat: %v", err)
			} else {
				resp.Body.Close()
			}
		}
	}
}

func runMonitor(ctx context.Context, cfg *Config) {
	ticker := time.NewTicker(cfg.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload := SnapshotPayload{
				MachineCode: cfg.MachineCode,
				CPUUsage:    getCPUUsage(),
				RAMUsage:    getRAMUsage(),
				DiskUsage:   getDiskUsage(),
				CPUTemp:     getCPUTemp(),
				GPUTemp:     getGPUTemp(),
				Uptime:      getUptime(),
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
			}
			data, _ := json.Marshal(payload)
			url := fmt.Sprintf("%s/api/machines/by-code/%s/snapshots", cfg.ServerURL, cfg.MachineCode)
			resp, err := httpClient.Post(url, "application/json", bytes.NewReader(data))
			if err != nil {
				log.Printf("monitor: %v", err)
			} else {
				resp.Body.Close()
			}
		}
	}
}

type Watchdog struct {
	locker       *ScreenLocker
	blockedApps  []string
	lockRequired bool
}

func runWatchdog(ctx context.Context, cfg *Config, locker *ScreenLocker, blockedApps *[]string, lockRequired *bool) {
	ticker := time.NewTicker(cfg.WatchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if *lockRequired && cfg.ScreenLockEnabled {
				log.Println("watchdog: locking screen")
				locker.Lock()
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

func getGPUTemp() float64 {
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
	stat, err := disk.Usage("/")
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
