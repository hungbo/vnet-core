//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	lockWorkStation  = user32.NewProc("LockWorkStation")
	messageBoxW      = user32.NewProc("MessageBoxW")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procSendMessageW = user32.NewProc("SendMessageW")
)

type ScreenLocker struct{}

func NewScreenLocker() *ScreenLocker {
	return &ScreenLocker{}
}

func (l *ScreenLocker) Lock() error {
	ret, _, err := lockWorkStation.Call()
	if ret == 0 {
		return fmt.Errorf("LockWorkStation failed: %v", err)
	}
	return nil
}

func (l *ScreenLocker) Unlock() {
	hwnd, _, _ := procFindWindowW.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("WindowsShell"))),
		0,
	)
	if hwnd != 0 {
		procSendMessageW.Call(hwnd, 0x0112, 0xF170, 2)
	}
}

func (l *ScreenLocker) ShowMessage(title, message string) error {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	ret, _, err := messageBoxW.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), 0)
	if ret == 0 {
		return fmt.Errorf("MessageBox failed: %v", err)
	}
	return nil
}
