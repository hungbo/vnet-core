//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func comForTest(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == syscall.Errno(1) { // 1 = S_FALSE
		t.Cleanup(windows.CoUninitialize)
	}
}

// Số thứ tự các phương thức Get*/SetArguments thêm cho lối tắt game không kiểm
// được ngoài Windows: sai một số là gọi nhầm hàm của đối tượng COM.
func TestLnk_ThamSoVaMoTa(t *testing.T) {
	comForTest(t)
	dir := t.TempDir()
	exe := filepath.Join(dir, "Game.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lnk := filepath.Join(dir, "Game.lnk")

	const args = `--launch-product=league_of_legends --launch-patchline=live "C:\a b\c"`
	const desc = "VNET: Liên Minh Huyền Thoại"
	if err := writeLinkArgs(lnk, exe, args, dir, desc); err != nil {
		t.Fatalf("writeLinkArgs: %v", err)
	}
	info, err := readLinkInfo(lnk)
	if err != nil {
		t.Fatalf("readLinkInfo: %v", err)
	}
	if info.Args != args || info.Desc != desc {
		t.Errorf("đọc lại args=%q desc=%q", info.Args, info.Desc)
	}
	// Chỉ so tên cuối: thư mục tạm có thể ở dạng 8.3 (RUNNER~1) khi ghi và dạng dài khi đọc.
	if !strings.EqualFold(filepath.Base(info.Target), "Game.exe") || !strings.EqualFold(filepath.Base(info.WorkDir), filepath.Base(dir)) {
		t.Errorf("đọc lại target=%q workdir=%q", info.Target, info.WorkDir)
	}

	// writeLink cũ (không tham số) vẫn ra lối tắt không tham số, kể cả khi ghi đè lên bản có tham số.
	if err := writeLink(lnk, exe, dir, "VNET"); err != nil {
		t.Fatal(err)
	}
	if info, err = readLinkInfo(lnk); err != nil || info.Args != "" || info.Desc != "VNET" {
		t.Errorf("sau writeLink: %+v, %v", info, err)
	}
}

// Đi trọn vòng thật: tạo, nhận ra lối tắt của VNET, xoá cái không còn mong muốn mà
// không đụng lối tắt của người khác.
func TestSyncDesktopShortcuts(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) gameShortcutSpec {
		exe := filepath.Join(dir, name+".exe")
		if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return gameShortcutSpec{File: name + ".lnk", Desc: gameLnkPrefix + name, Target: exe, Args: "-x", WorkDir: dir}
	}
	a, b := mk("Alpha"), mk("Beta")

	comForTest(t)
	foreign := filepath.Join(dir, "Của khách.lnk")
	if err := writeLink(foreign, a.Target, dir, "lối tắt của khách"); err != nil {
		t.Fatal(err)
	}

	// syncDesktopShortcuts tự khởi tạo COM trên luồng của nó; chạy trong goroutine để
	// COM của bài kiểm (STA) không dính vào.
	run := func(desired []gameShortcutSpec) {
		t.Helper()
		done := make(chan error, 1)
		go func() { _, err := syncDesktopShortcuts(dir, desired); done <- err }()
		if err := <-done; err != nil {
			t.Fatalf("syncDesktopShortcuts: %v", err)
		}
	}
	names := func() map[string]bool {
		t.Helper()
		got, err := scanDesktopLnks(dir)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, l := range got {
			out[l.File] = l.Ours
		}
		return out
	}

	run([]gameShortcutSpec{a, b})
	if got := names(); !got["Alpha.lnk"] || !got["Beta.lnk"] || got["Của khách.lnk"] {
		t.Errorf("sau lần 1: %v (mong Alpha, Beta là của VNET; lối tắt khách không)", got)
	}

	run([]gameShortcutSpec{a})
	got := names()
	if _, ok := got["Beta.lnk"]; ok {
		t.Error("Beta không còn mong muốn mà vẫn còn")
	}
	if !got["Alpha.lnk"] {
		t.Error("Alpha bị mất")
	}
	if _, ok := got["Của khách.lnk"]; !ok {
		t.Error("lối tắt của khách bị xoá")
	}
}
