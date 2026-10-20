package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Kiểm tệp chạy của game TRƯỚC khi khách bấm Chơi: máy chủ chỉ biết game "sẵn sàng", còn máy trạm mới thấy được tệp có
// thật trên ổ game của mình không. Game mất tệp chạy (bị xoá, ổ chưa gắn, đổi ổ riêng sai) hiện "Thiếu tệp chạy" và không
// bấm được, thay vì để khách bấm rồi mới gặp lỗi.
//
// Chỉ stat tệp (không đọc) và nhớ kết quả launcherCheckTTL, nên 200 game không làm nặng ổ đĩa. Ổ mạng treo thì stat chờ
// tối đa launcherStatTimeout rồi bỏ qua (không biết thì không báo lỗi): không được làm đứng cả danh sách menu.

const (
	problemMissingLauncher = "missing_launcher"
	problemBadConfig       = "bad_config"

	launcherCheckTTL    = 5 * time.Minute
	launcherStatTimeout = 2 * time.Second
)

// statRegular nằm ở biến để bài kiểm thay được. known=false là hết hạn chờ mà chưa có câu trả lời.
var statRegular = func(path string, timeout time.Duration) (ok, known bool) {
	ch := make(chan bool, 1)
	go func() {
		st, err := os.Stat(path)
		ch <- err == nil && st.Mode().IsRegular()
	}()
	select {
	case r := <-ch:
		return r, true
	case <-time.After(timeout):
		return false, false
	}
}

type launcherResult struct {
	problem string
	at      time.Time
}

var launcherChecks struct {
	sync.Mutex
	byExe    map[string]launcherResult // theo đường dẫn tệp chạy
	reported map[string]bool           // key + vấn đề đã ghi nhật ký (mỗi cặp một lần mỗi lần chạy)
}

// launcherProblem trả vấn đề của tệp chạy (rỗng là tốt hoặc chưa biết), dùng kết quả cũ trong launcherCheckTTL.
func launcherProblem(exe string, now time.Time) string {
	launcherChecks.Lock()
	if r, ok := launcherChecks.byExe[exe]; ok && now.Sub(r.at) < launcherCheckTTL {
		launcherChecks.Unlock()
		return r.problem
	}
	launcherChecks.Unlock()

	ok, known := statRegular(exe, launcherStatTimeout)
	if !known {
		return "" // ổ chưa trả lời: không kết luận, lượt sau hỏi lại
	}
	problem := ""
	if !ok {
		problem = problemMissingLauncher
	}
	launcherChecks.Lock()
	if launcherChecks.byExe == nil {
		launcherChecks.byExe = map[string]launcherResult{}
	}
	launcherChecks.byExe[exe] = launcherResult{problem: problem, at: now}
	launcherChecks.Unlock()
	return problem
}

// annotateLaunchers ghi vấn đề tệp chạy vào các mục SẴN SÀNG của một bản sao menu (mục chưa sẵn sàng đã bị chặn vì lý do
// khác). Bản gốc, có thể đang nằm trong bộ nhớ đệm, không bị đổi.
func annotateLaunchers(m *gameMenu, now time.Time) *gameMenu {
	out := *m
	out.Items = append([]gameMenuItem(nil), m.Items...)
	for i := range out.Items {
		g := &out.Items[i]
		g.Problem = ""
		if g.Status != gameStatusReady {
			continue
		}
		exe, _, err := resolveGameLaunch(g.rootOf(m.Root), g.Name, g.Launcher)
		if err != nil {
			g.Problem = problemBadConfig
			noteLauncherProblem(g, fmt.Sprintf("cấu hình không hợp lệ: %v", err))
			continue
		}
		if g.Problem = launcherProblem(exe, now); g.Problem != "" {
			noteLauncherProblem(g, "không thấy tệp "+exe)
		}
	}
	return &out
}

// noteLauncherProblem ghi nhật ký và báo lỗi về máy chủ MỘT lần cho mỗi cặp (mục, vấn đề), để nhân viên thấy mà không bị
// dồn lỗi mỗi lượt làm mới 10 giây.
func noteLauncherProblem(g *gameMenuItem, detail string) {
	k := g.menuKey() + "|" + g.Problem
	launcherChecks.Lock()
	if launcherChecks.reported == nil {
		launcherChecks.reported = map[string]bool{}
	}
	first := !launcherChecks.reported[k]
	launcherChecks.reported[k] = true
	launcherChecks.Unlock()
	if first {
		log.Printf("[game] %s: tệp chạy không dùng được (%s): %s", g.menuKey(), g.Problem, detail)
		reportError("error", "game", fmt.Sprintf("game %s không có tệp chạy dùng được", g.Name), detail)
	}
}
