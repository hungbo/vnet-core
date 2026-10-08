//go:build windows

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Kênh nội bộ giữa dịch vụ nền và giao diện: một named pipe do DỊCH VỤ tạo.
//
// Giao diện gửi báo cáo trạng thái mỗi hai giây, dịch vụ trả lời từng báo cáo.
// Hỏi-đáp tuần tự chứ không đọc ghi song song: handle đồng bộ của Windows xếp
// hàng mọi thao tác, một lệnh đọc đang chờ sẽ chặn luôn lệnh ghi trên cùng handle.
//
// Nhờ kênh này dịch vụ biết giao diện còn PHẢN HỒI hay không, chứ không chỉ còn
// tồn tại: một tiến trình bị treo vẫn "đang chạy" với mọi phép kiểm mã thoát.
const uiPipeName = `\\.\pipe\vnet-client-ui`

// Quyền trên pipe: SYSTEM và quản trị toàn quyền; người dùng tương tác chỉ được
// đọc và ghi dữ liệu. Cố ý KHÔNG cấp GENERIC_WRITE: nó kèm quyền tạo thêm
// instance của pipe, tức là một tiến trình khác dựng được "dịch vụ giả" cùng tên.
const (
	uiPipeSDDL         = "D:(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x120183;;;IU)"
	uiPipeClientAccess = 0x1 | 0x2 | 0x80 | 0x100000 // đọc, ghi, đọc thuộc tính, SYNCHRONIZE
)

func createUIPipe(first bool) (windows.Handle, error) {
	sd, err := windows.SecurityDescriptorFromString(uiPipeSDDL)
	if err != nil {
		return 0, err
	}
	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	flags := uint32(windows.PIPE_ACCESS_DUPLEX)
	if first {
		// Tên này đã có ai tạo trước thì thất bại thay vì nhập chung với họ.
		flags |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	mode := uint32(windows.PIPE_TYPE_MESSAGE | windows.PIPE_READMODE_MESSAGE |
		windows.PIPE_WAIT | windows.PIPE_REJECT_REMOTE_CLIENTS)
	name, _ := windows.UTF16PtrFromString(uiPipeName)
	return windows.CreateNamedPipe(name, flags, mode, windows.PIPE_UNLIMITED_INSTANCES, 4096, 4096, 0, sa)
}

// runUILinkServer chạy trong dịch vụ nền.
func runUILinkServer(ctx context.Context, sup *uiSupervisor) {
	first := true
	for ctx.Err() == nil {
		h, err := createUIPipe(first)
		if err != nil {
			log.Printf("[kênh giao diện] không tạo được pipe: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		first = false
		if err := windows.ConnectNamedPipe(h, nil); err != nil && err != windows.ERROR_PIPE_CONNECTED {
			windows.CloseHandle(h)
			continue
		}
		go serveUIPipe(h, sup)
	}
}

func serveUIPipe(h windows.Handle, sup *uiSupervisor) {
	defer windows.CloseHandle(h)
	defer windows.DisconnectNamedPipe(h)

	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(h, &pid); err != nil || !laGiaoDienVNET(pid) {
		log.Printf("[kênh giao diện] từ chối tiến trình %d: không phải giao diện VNET đã cài", pid)
		return
	}

	buf := make([]byte, 4096)
	for {
		var n uint32
		if err := windows.ReadFile(h, buf, &n, nil); err != nil {
			return
		}
		var st uiStatus
		if json.Unmarshal(buf[:n], &st) != nil {
			return
		}
		now := time.Now()
		sup.noteStatus(now, st)
		pol := currentPolicy()
		out, _ := json.Marshal(svcReply{OfflineLock: sup.offlineLock(now, pol), Policy: pol})
		if err := windows.WriteFile(h, out, &n, nil); err != nil {
			return
		}
	}
}

// laGiaoDienVNET: tiến trình ở đầu kia có chạy từ ĐÚNG tệp .exe đã cài không.
// Thư mục cài chỉ quản trị ghi được, nên đường dẫn tệp là thứ tin được.
func laGiaoDienVNET(pid uint32) bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(p)
	buf := make([]uint16, windows.MAX_PATH*2)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(p, 0, &buf[0], &size); err != nil {
		return false
	}
	return strings.EqualFold(windows.UTF16ToString(buf[:size]), self)
}

// runUILinkClient chạy trong giao diện: báo trạng thái, nhận chỉ thị.
//
// Không có dịch vụ (bản chạy rời, máy đang phát triển) thì pipe không tồn tại;
// vòng lặp chỉ thử lại, không coi đó là lỗi.
func runUILinkClient(ctx context.Context, a *App) {
	name, _ := windows.UTF16PtrFromString(uiPipeName)
	for ctx.Err() == nil {
		h, err := windows.CreateFile(name, uiPipeClientAccess, 0, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			sleepCtx(ctx, 2*time.Second)
			continue
		}
		uiLinkLoop(ctx, h, a)
		windows.CloseHandle(h)
		sleepCtx(ctx, 2*time.Second)
	}
}

func uiLinkLoop(ctx context.Context, h windows.Handle, a *App) {
	buf := make([]byte, 4096)
	for ctx.Err() == nil {
		out, _ := json.Marshal(a.linkStatus())
		var n uint32
		if err := windows.WriteFile(h, out, &n, nil); err != nil {
			return
		}
		if err := windows.ReadFile(h, buf, &n, nil); err != nil {
			return
		}
		dichVuTraLoiLuc.Store(time.Now().UnixNano())
		var reply svcReply
		if json.Unmarshal(buf[:n], &reply) == nil {
			a.applyServiceReply(reply)
		}
		sleepCtx(ctx, 2*time.Second)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// consoleUserIsAdmin: tài khoản đang đăng nhập phiên màn hình có thuộc nhóm
// Administrators không. Trả nil khi chưa ai đăng nhập Windows, hoặc khi tiến
// trình này không phải dịch vụ nền (chỉ SYSTEM hỏi được token của phiên khác).
//
// Xét SỰ CÓ MẶT của nhóm trong token, kể cả dạng "chỉ để từ chối" mà UAC gắn
// cho token đã lọc: tài khoản quản trị chạy không nâng quyền vẫn là tài khoản
// quản trị — nó chỉ cách quyền thật một hộp thoại bấm "Yes".
func consoleUserIsAdmin() *bool {
	sessionID := windows.WTSGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return nil
	}
	var tok windows.Token
	if err := windows.WTSQueryUserToken(sessionID, &tok); err != nil {
		return nil
	}
	defer tok.Close()

	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return nil
	}
	groups, err := tok.GetTokenGroups()
	if err != nil {
		return nil
	}
	laAdmin := false
	for _, g := range groups.AllGroups() {
		if g.Sid.Equals(adminSID) {
			laAdmin = true
			break
		}
	}
	return &laAdmin
}
