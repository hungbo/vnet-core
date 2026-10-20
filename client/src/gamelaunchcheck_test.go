package main

import (
	"testing"
	"time"
)

func resetLauncherChecks() {
	launcherChecks.Lock()
	launcherChecks.byExe, launcherChecks.reported = nil, nil
	launcherChecks.Unlock()
}

func TestAnnotateLaunchers(t *testing.T) {
	resetLauncherChecks()
	old := statRegular
	defer func() { statRegular = old }()
	existing := map[string]bool{`D:\Games\Ok\ok.exe`: true, `E:\G2\Own\own.exe`: true}
	calls := map[string]int{}
	statRegular = func(p string, _ time.Duration) (bool, bool) {
		calls[p]++
		if p == `D:\Games\Slow\slow.exe` {
			return false, false // ổ không trả lời kịp
		}
		return existing[p], true
	}
	m := &gameMenu{Root: `D:\Games`, Items: []gameMenuItem{
		{Key: "Ok", Name: "Ok", Launcher: "ok.exe", Status: "ready"},
		{Key: "Missing", Name: "Missing", Launcher: "x.exe", Status: "ready"},
		{Key: "Own", Name: "Own", Launcher: "own.exe", Status: "ready", Root: `E:\G2`},
		{Key: "Bad", Name: "Bad", Launcher: `..\evil.exe`, Status: "ready"},
		{Key: "Down", Name: "Down", Launcher: "d.exe", Status: "downloading"},
		{Key: "Slow", Name: "Slow", Launcher: "slow.exe", Status: "ready"},
	}}
	now := time.Unix(5000, 0)
	out := annotateLaunchers(m, now)
	got := map[string]string{}
	for _, g := range out.Items {
		got[g.Key] = g.Problem
	}
	equal(t, map[string]string{"Ok": "", "Missing": problemMissingLauncher, "Own": "", "Bad": problemBadConfig, "Down": "", "Slow": ""}, got, "vấn đề từng mục")
	equal(t, "", m.Items[1].Problem, "bản gốc trong bộ nhớ đệm không bị đổi")
	equal(t, 0, calls[`D:\Games\Down\d.exe`], "mục chưa sẵn sàng không bị stat")

	// trong hạn thì không stat lại; ổ không trả lời thì hỏi lại lượt sau
	annotateLaunchers(m, now.Add(time.Minute))
	equal(t, 1, calls[`D:\Games\Ok\ok.exe`], "dùng kết quả cũ trong launcherCheckTTL")
	equal(t, 2, calls[`D:\Games\Slow\slow.exe`], "chưa biết thì lượt sau hỏi lại")
	existing[`D:\Games\Missing\x.exe`] = true
	out = annotateLaunchers(m, now.Add(launcherCheckTTL+time.Second))
	equal(t, "", out.Items[1].Problem, "hết hạn thì kiểm lại và thấy tệp đã về")
}

func TestStatRegular_ThatSu(t *testing.T) {
	dir := t.TempDir()
	if ok, known := statRegular(dir, time.Second); !known || ok {
		t.Fatalf("thư mục không phải tệp thường: ok=%v known=%v", ok, known)
	}
	if ok, known := statRegular(dir+"/khong-co.exe", time.Second); !known || ok {
		t.Fatalf("tệp không có: ok=%v known=%v", ok, known)
	}
}
