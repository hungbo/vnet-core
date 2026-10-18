//go:build darwin || linux

package main

import "sync/atomic"

// Bản không-Windows chỉ giữ trạng thái để phần chung biên dịch và chạy được khi
// phát triển trên máy Mac/Linux. Không có hook bàn phím ở đây; máy trạm thật
// luôn là Windows.

type ScreenLocker struct {
	locked atomic.Bool
}

func NewScreenLocker() *ScreenLocker {
	return &ScreenLocker{}
}

func (l *ScreenLocker) Locked() bool { return l.locked.Load() }

func (l *ScreenLocker) Lock() error {
	l.locked.Store(true)
	return nil
}

func (l *ScreenLocker) Unlock() {
	l.locked.Store(false)
}

func (l *ScreenLocker) ShowMessage(title, message string) error {
	return nil
}
