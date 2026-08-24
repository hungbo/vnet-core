//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type vnetService struct {
	cancel context.CancelFunc
	ctx    context.Context
}

func (s *vnetService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}
	go runAgent(s.ctx)
	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	for req := range r {
		switch req.Cmd {
		case svc.Interrogate:
			changes <- req.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			s.cancel()
			return false, 0
		}
	}
	return false, 0
}

// runAsWindowsService chạy dưới Service Control Manager nếu đang được SCM gọi.
// Trả về false khi chạy từ dòng lệnh, để người gọi rơi về chế độ tiến trình
// thường (tiện lúc thử tay).
func runAsWindowsService(ctx context.Context, cancel context.CancelFunc) bool {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false
	}
	if err := svc.Run(serviceName, &vnetService{ctx: ctx, cancel: cancel}); err != nil {
		log.Printf("[service] thoát: %v", err)
	}
	return true
}

func installService() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("không mở được Service Control Manager (cần quyền quản trị): %w", err)
	}
	defer m.Disconnect()

	if existing, err := m.OpenService(serviceName); err == nil {
		existing.Close()
		return fmt.Errorf("dịch vụ %s đã tồn tại — gỡ trước bằng --uninstall-service", serviceName)
	}

	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName: serviceDisplayName,
		Description: "Giữ kết nối với máy chủ VNET và trông chừng giao diện máy trạm.",
		StartType:   mgr.StartAutomatic,
	}, "--service")
	if err != nil {
		return err
	}
	defer s.Close()

	// Giao cho chính Windows việc bật lại dịch vụ khi nó chết. Đây là lớp bảo
	// vệ mạnh nhất và rẻ nhất: Service Control Manager dựng lại tiến trình sau
	// vài giây, không cần ai canh ai. Lớp canh trong giao diện (guard.go) chỉ
	// là chốt cuối, cho trường hợp có người chuyển dịch vụ sang Disabled hoặc
	// gỡ luôn phần khôi phục này.
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
	}, 86400); err != nil {
		// Không đặt được thì vẫn cài tiếp: dịch vụ chạy được là chuyện chính,
		// và chốt cuối bên giao diện vẫn còn đó.
		log.Printf("[service] không đặt được chế độ tự khôi phục: %v", err)
	}

	// Cho người dùng thường quyền BẬT dịch vụ (không phải dừng, không phải sửa).
	// Không có nó thì lớp canh bên giao diện — chạy dưới tài khoản khách — không
	// bao giờ cứu nổi dịch vụ, và mọi lần cứu hụt đều dẫn tới tắt máy oan.
	if err := grantServiceStartToUsers(); err != nil {
		log.Printf("[service] không mở được quyền bật dịch vụ cho người dùng: %v", err)
	}

	// Nới chế độ khôi phục sang cả trường hợp dừng với mã thoát khác 0. KHÔNG
	// phủ được `sc stop` — dừng sạch sẽ thì SCM không bao giờ coi là hỏng — nên
	// mới có hai tác vụ theo lịch bên dưới.
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		log.Printf("[service] không đặt được cờ khôi phục: %v", err)
	}

	if err := registerWatchdogTasks(exe); err != nil {
		log.Printf("[service] không đăng ký được tác vụ canh chừng: %v", err)
	}

	return s.Start()
}

// registerWatchdogTasks đăng ký hai tác vụ theo lịch bằng schtasks.
//
// Tác vụ theo nhịp đăng ký TRƯỚC và lỗi của nó là lỗi thật; tác vụ theo sự kiện
// chỉ để rút thời gian phản ứng, hỏng thì ghi nhật ký rồi đi tiếp — bộ lọc XPath
// của nó là chỗ dễ sai, và mất nó vẫn còn lớp mỗi phút.
func registerWatchdogTasks(exe string) error {
	if err := createTask(watchdogTaskEvery, taskXMLEvery(exe)); err != nil {
		return err
	}
	if err := createTask(watchdogTaskEvent, taskXMLOnStop(exe, serviceDisplayName)); err != nil {
		log.Printf("[service] tác vụ theo sự kiện không đăng ký được (vẫn còn lớp mỗi phút): %v", err)
	}
	return nil
}

// createTask ghi XML ra tệp tạm rồi gọi schtasks /Create /XML.
//
// Qua tệp chứ không qua tham số dòng lệnh: XML có dấu ngoặc kép và ký tự xuống
// dòng, truyền thẳng vào cmd là hỏng.
func createTask(name, xml string) error {
	f, err := os.CreateTemp("", "vnet-task-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	// UTF-16 kèm BOM: schtasks /Create /XML từ chối tệp UTF-8 trên nhiều bản
	// Windows, và thông báo lỗi thì không nói gì về mã hoá.
	if _, err := f.Write(utf16BOMBytes(xml)); err != nil {
		f.Close()
		return err
	}
	f.Close()

	out, err := exec.Command("schtasks", "/Create", "/TN", name, "/XML", f.Name(), "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks /Create %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeWatchdogTasks() {
	for _, name := range []string{watchdogTaskEvery, watchdogTaskEvent} {
		if out, err := exec.Command("schtasks", "/Delete", "/TN", name, "/F").CombinedOutput(); err != nil {
			log.Printf("[service] không xoá được tác vụ %s: %v: %s",
				name, err, strings.TrimSpace(string(out)))
		}
	}
}

// ensureServiceRunning là thân của chế độ --ensure-service: tác vụ theo lịch gọi
// nó, nó bật dịch vụ nếu dịch vụ đang không chạy.
func ensureServiceRunning() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("không mở được Service Control Manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("dịch vụ %s chưa được đăng ký: %w", serviceName, err)
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("không đọc được trạng thái dịch vụ: %w", err)
	}
	if status.State == svc.Running || status.State == svc.StartPending {
		return nil
	}

	log.Printf("[watchdog] dịch vụ đang ở trạng thái %v — bật lại", status.State)
	return s.Start()
}

// grantServiceStartToUsers nới quyền trên dịch vụ: giữ nguyên mô tả bảo mật mặc
// định rồi thêm cho nhóm Authenticated Users (AU) quyền đọc trạng thái và bật.
//
// Dùng sc.exe vì mô tả bảo mật của dịch vụ phải viết bằng SDDL, và tự dựng SDDL
// bằng tay trong Go là một cách rất tốn công để gõ sai một ký tự rồi khoá luôn
// dịch vụ khỏi chính mình.
//
//	RP = start, WP = stop, DT = pause  →  chỉ cấp RP và quyền đọc.
func grantServiceStartToUsers() error {
	out, err := exec.Command("sc", "sdshow", serviceName).Output()
	if err != nil {
		return fmt.Errorf("sc sdshow: %w", err)
	}
	sddl := strings.TrimSpace(string(out))
	if sddl == "" {
		return errors.New("sc sdshow trả về rỗng")
	}
	if strings.Contains(sddl, "(A;;CCLCSWRPLO;;;AU)") {
		return nil // đã cấp rồi
	}

	// Chèn ngay sau phần D: — thứ tự các mục trong DACL là có ý nghĩa.
	i := strings.Index(sddl, "D:")
	if i < 0 {
		return errors.New("không tìm thấy phần DACL trong SDDL")
	}
	j := strings.Index(sddl[i:], "(")
	if j < 0 {
		return errors.New("DACL rỗng")
	}
	moi := sddl[:i+j] + "(A;;CCLCSWRPLO;;;AU)" + sddl[i+j:]

	return exec.Command("sc", "sdset", serviceName, moi).Run()
}

func uninstallService() error {
	// Xoá tác vụ trước: gỡ dịch vụ xong mà tác vụ còn thì cứ mỗi phút nó lại
	// thử bật một dịch vụ không còn tồn tại.
	removeWatchdogTasks()

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("không mở được Service Control Manager (cần quyền quản trị): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("dịch vụ %s chưa được đăng ký", serviceName)
	}
	defer s.Close()

	_, _ = s.Control(svc.Stop)
	return s.Delete()
}

// uiProc giữ tiến trình giao diện mà dịch vụ vừa bật lên, để biết khi nào nó chết.
var uiProc *os.Process

func uiIsRunning() bool {
	if uiProc == nil {
		return false
	}
	// Trên Windows, FindProcess luôn thành công nên phải hỏi mã thoát.
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(uiProc.Pid))
	if err != nil {
		uiProc = nil
		return false
	}
	defer windows.CloseHandle(h)

	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		uiProc = nil
		return false
	}
	const stillActive = 259
	if code != stillActive {
		uiProc = nil
		return false
	}
	return true
}

// launchUIInUserSession bật giao diện trong phiên đăng nhập đang hoạt động.
//
// Dịch vụ chạy ở session 0, nơi KHÔNG cửa sổ nào hiện được. Muốn có giao diện thì
// phải mượn token của người đang đăng nhập rồi tạo tiến trình trong phiên của họ
// — đó là toàn bộ lý do đoạn Win32 dưới đây tồn tại.
//
// Chưa ai đăng nhập vào Windows là trạng thái bình thường (máy vừa khởi động,
// đang ở màn hình khoá), không phải lỗi.
func launchUIInUserSession() error {
	sessionID := windows.WTSGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return fmt.Errorf("chưa có phiên người dùng nào")
	}

	var userToken windows.Token
	if err := windows.WTSQueryUserToken(sessionID, &userToken); err != nil {
		return fmt.Errorf("chưa ai đăng nhập vào Windows: %w", err)
	}
	defer userToken.Close()

	var dup windows.Token
	if err := windows.DuplicateTokenEx(userToken, windows.MAXIMUM_ALLOWED, nil,
		windows.SecurityIdentification, windows.TokenPrimary, &dup); err != nil {
		return err
	}
	defer dup.Close()

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, dup, false); err == nil {
		defer windows.DestroyEnvironmentBlock(env)
	}

	si := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	// Giao diện phải nằm trên desktop tương tác của người dùng, không thì nó chạy
	// vô hình ở một desktop khác.
	desktop, _ := syscall.UTF16PtrFromString(`winsta0\default`)
	si.Desktop = desktop

	var pi windows.ProcessInformation
	cmdline, _ := syscall.UTF16PtrFromString(`"` + exe + `"`)
	dir, _ := syscall.UTF16PtrFromString(filepath.Dir(exe))

	const createUnicodeEnvironment = 0x00000400
	if err := windows.CreateProcessAsUser(dup, nil, cmdline, nil, nil, false,
		createUnicodeEnvironment, env, dir, &si, &pi); err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)

	if p, err := os.FindProcess(int(pi.ProcessId)); err == nil {
		uiProc = p
	}
	log.Printf("[agent] đã bật giao diện, PID %d", pi.ProcessId)
	return nil
}

var _ = strings.TrimSpace
