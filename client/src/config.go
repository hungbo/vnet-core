package main

import (
	"os"
	"time"
)

type Config struct {
	ServerURL          string
	MachineCode        string
	HeartbeatInterval  time.Duration
	ScreenLockEnabled  bool
	HighTempThreshold  float64
	SnapshotInterval   time.Duration
	WatchdogInterval   time.Duration
}

func LoadConfig() *Config {
	cfg := &Config{
		ServerURL:         getEnv("VNET_SERVER_URL", "http://localhost:8080"),
		MachineCode:       getEnv("VNET_MACHINE_CODE", ""),
		HeartbeatInterval: 15 * time.Second,
		ScreenLockEnabled: true,
		HighTempThreshold: 85.0,
		SnapshotInterval:  60 * time.Second,
		WatchdogInterval:  30 * time.Second,
	}
	if cfg.MachineCode == "" {
		hostname, err := os.Hostname()
		if err == nil {
			cfg.MachineCode = hostname
		} else {
			cfg.MachineCode = "unknown"
		}
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
