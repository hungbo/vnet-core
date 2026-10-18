package main

import "testing"

// Hai lệnh đặt CONSOLELOCK về 0 (cắm điện và pin) rồi áp lại lược đồ: thiếu một
// trong ba là máy vẫn hỏi đăng nhập khi thức ở một chế độ nguồn nào đó.
func TestWakeLockPowercfgArgs(t *testing.T) {
	cmds := wakeLockPowercfgArgs()
	if len(cmds) != 3 {
		t.Fatalf("cần 3 lệnh powercfg, có %d", len(cmds))
	}
	want := [][]string{
		{"/setacvalueindex", "SCHEME_CURRENT", "SUB_NONE", "CONSOLELOCK", "0"},
		{"/setdcvalueindex", "SCHEME_CURRENT", "SUB_NONE", "CONSOLELOCK", "0"},
		{"/setactive", "SCHEME_CURRENT"},
	}
	for i := range want {
		if len(cmds[i]) != len(want[i]) {
			t.Fatalf("lệnh %d: %v, muốn %v", i, cmds[i], want[i])
		}
		for j := range want[i] {
			if cmds[i][j] != want[i][j] {
				t.Fatalf("lệnh %d: %v, muốn %v", i, cmds[i], want[i])
			}
		}
	}
}

func TestConsoleLockPolicyKey(t *testing.T) {
	got := consoleLockPolicyKey
	want := `SOFTWARE\Policies\Microsoft\Power\PowerSettings\0e796bdb-100d-47d6-a2d5-f7d2daa51f51`
	if got != want {
		t.Fatalf("khoá chính sách %q, muốn %q", got, want)
	}
}
