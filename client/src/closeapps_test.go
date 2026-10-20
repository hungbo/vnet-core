package main

import (
	"errors"
	"testing"
	"time"
)

func newTestClose(apps []openApp) (*closeAppsState, *[]uint32) {
	var killed []uint32
	return &closeAppsState{
		selfPID: 100, selfName: "vnet-client.exe",
		list: func() []openApp { return apps },
		terminate: func(a openApp) error {
			if a.Name == "cung-dau" {
				return errors.New("từ chối")
			}
			killed = append(killed, a.PID)
			return nil
		},
	}, &killed
}

func TestClosePress(t *testing.T) {
	apps := []openApp{{1, "explorer"}, {2, "chrome"}, {3, "cung-dau"}, {4, "discord"}, {100, "vnet-client"}, {5, "game"}}
	st, killed := newTestClose(apps)
	now := time.Now()
	on := clientPolicy{CloseHotkey: true, CloseExclude: "discord"}

	if n := st.press(clientPolicy{CloseHotkey: false}, false, now); n != 0 {
		t.Errorf("tắt tính năng mà vẫn đóng %d", n)
	}
	if n := st.press(on, true, now); n != 0 {
		t.Errorf("màn hình khoá mà vẫn đóng %d", n)
	}
	if n := st.press(on, false, now); n != 2 {
		t.Errorf("phải đóng chrome và game (cung-dau từ chối), đóng %d", n)
	}
	if !sameU32(*killed, []uint32{2, 5}) {
		t.Errorf("đã đóng PID %v", *killed)
	}
	// Bấm dồn trong thời gian chờ chỉ tính một lần.
	if n := st.press(on, false, now.Add(time.Second)); n != 0 {
		t.Errorf("bấm dồn mà đóng %d", n)
	}
	if n := st.press(on, false, now.Add(5*time.Second)); n != 2 {
		t.Errorf("hết thời gian chờ phải đóng lại, đóng %d", n)
	}
}

func sameU32(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
