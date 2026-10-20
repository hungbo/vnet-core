package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Lệnh chạy trước khi mở game (RunFirst): quán hoặc master khai vài chương trình trong thư mục game cần chạy xong trước
// (dựng cấu hình, bật dịch vụ phụ…). Máy trạm chạy lần lượt từng bước và chờ nó thoát, mỗi bước có hạn riêng và cả
// chuỗi có hạn chung, để một bước treo không làm khách đứng nhìn nút Chơi mãi.
//
// Đường dẫn dựng bằng đúng resolveGameLaunch của tệp chạy chính nên chịu cùng luật: tương đối trong thư mục game, không
// "..", không ổ đĩa, không UNC. Danh sách đến từ máy chủ, và nó đi thẳng vào lệnh chạy tệp.

const (
	// runFirstStepTimeout: một bước chạy quá lâu thì bỏ chờ (không tắt nó) và sang bước sau.
	runFirstStepTimeout = 30 * time.Second
	// runFirstTotalBudget: tổng thời gian chờ mọi bước.
	runFirstTotalBudget = 60 * time.Second
	// maxRunFirstSteps khớp maxGameRunFirst phía máy chủ; máy chủ gửi nhiều hơn thì bỏ phần dư.
	maxRunFirstSteps = 5
)

// gameRunStep: một bước chạy trước, khớp model.GameRunStep phía máy chủ.
type gameRunStep struct {
	Launcher   string `json:"launcher"`
	LaunchArgs string `json:"launch_args"`
}

// resolvedStep là một bước đã dựng đường dẫn đầy đủ.
type resolvedStep struct {
	Exe, Dir, Args string
}

// planRunFirst dựng đường dẫn cho từng bước. Bước hỏng (đường dẫn không hợp lệ) bị bỏ và trả lỗi riêng, các bước còn lại
// vẫn chạy.
func planRunFirst(root, name string, steps []gameRunStep) (plan []resolvedStep, problems []string) {
	for i, st := range steps {
		if i >= maxRunFirstSteps {
			problems = append(problems, fmt.Sprintf("bỏ %d bước thừa (tối đa %d)", len(steps)-i, maxRunFirstSteps))
			break
		}
		exe, dir, err := resolveGameLaunch(root, name, st.Launcher)
		if err != nil {
			problems = append(problems, fmt.Sprintf("bước %d (%s): %v", i+1, st.Launcher, err))
			continue
		}
		plan = append(plan, resolvedStep{Exe: exe, Dir: dir, Args: strings.TrimSpace(st.LaunchArgs)})
	}
	return plan, problems
}

// runPlan chạy lần lượt các bước đã dựng. run chạy một bước và chờ tối đa wait; exists kiểm tệp có thật. Trả về lỗi của
// từng bước (nếu có) để báo cho nhân viên. Hết hạn chung thì các bước còn lại bị bỏ.
func runPlan(plan []resolvedStep, run func(exe, dir, args string, wait time.Duration) error, exists func(string) bool,
	perStep, total time.Duration, now func() time.Time) (problems []string) {
	start := now()
	for i, st := range plan {
		left := total - now().Sub(start)
		if left <= 0 {
			problems = append(problems, fmt.Sprintf("hết %s chờ chung, bỏ %d bước còn lại", total, len(plan)-i))
			break
		}
		if !exists(st.Exe) {
			problems = append(problems, fmt.Sprintf("không thấy tệp %s", st.Exe))
			continue
		}
		wait := perStep
		if left < wait {
			wait = left
		}
		if err := run(st.Exe, st.Dir, st.Args, wait); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", st.Exe, err))
		}
	}
	return problems
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// runFirstBeforeGame chạy các bước chuẩn bị của một mục menu. Chỉ ghi log và báo lỗi, không bao giờ chặn việc mở game:
// khách bấm Chơi thì game vẫn mở, nhân viên đọc báo lỗi để sửa cấu hình.
func (a *App) runFirstBeforeGame(root string, g *gameMenuItem) {
	if len(g.RunFirst) == 0 {
		return
	}
	plan, problems := planRunFirst(root, g.Name, g.RunFirst)
	problems = append(problems, runPlan(plan, runRunFirstStep, fileExists, runFirstStepTimeout, runFirstTotalBudget, time.Now)...)
	log.Printf("[game] %s: chạy %d/%d bước chuẩn bị, %d lỗi", g.menuKey(), len(plan), len(g.RunFirst), len(problems))
	if len(problems) > 0 {
		reportError("error", "game", fmt.Sprintf("lệnh chạy trước của game %s có lỗi", g.Name), strings.Join(problems, "; "))
	}
}
