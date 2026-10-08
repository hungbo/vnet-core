//go:build !windows

package main

import "context"

// Kênh dịch vụ ↔ giao diện chỉ có trên Windows. Bản dựng ở nơi khác (để phát
// triển giao diện) không có dịch vụ nền nào để nói chuyện.
func runUILinkServer(ctx context.Context, sup *uiSupervisor) {}

func runUILinkClient(ctx context.Context, a *App) {}

func consoleUserIsAdmin() *bool { return nil }
