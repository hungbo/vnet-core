//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Phía Windows của shortcuts.go: tìm các thư mục desktop và giữ lối tắt VNET.

// publicDesktopDir là C:\Users\Public\Desktop. Hỏi hệ điều hành trước; hỏi không
// được mới rơi về %PUBLIC%.
func publicDesktopDir() string {
	if dir, err := windows.KnownFolderPath(windows.FOLDERID_PublicDesktop, 0); err == nil {
		return dir
	}
	public := os.Getenv("PUBLIC")
	if public == "" {
		public = `C:\Users\Public`
	}
	return filepath.Join(public, "Desktop")
}

// usersRootDir là C:\Users.
func usersRootDir() string {
	if dir, err := windows.KnownFolderPath(windows.FOLDERID_UserProfiles, 0); err == nil {
		return dir
	}
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	return drive + `\Users`
}

// desktopDirs: Desktop chung cùng Desktop của mọi hồ sơ người dùng.
func desktopDirs() []string {
	return append([]string{publicDesktopDir()}, userDesktopDirs(usersRootDir())...)
}

var (
	// lnkChecked: đã đối chiếu đích của VNET.lnk một lần kể từ lúc dịch vụ bật.
	// Sau đó chỉ tạo lại khi tệp mất — không so đích mỗi 10 giây, kẻo một đường
	// dẫn mà Windows ghi khác đi (tên 8.3...) thành vòng ghi lại mãi.
	lnkChecked bool
	// lnkLastErr: lỗi tạo gần nhất đã ghi nhật ký, để vòng 10 giây không lặp lại.
	lnkLastErr string
)

// ensureVNETShortcut tạo C:\Users\Public\Desktop\VNET.lnk trỏ tới chính tệp .exe
// này nếu chưa có, và sửa nếu lúc dịch vụ bật nó trỏ sang chỗ khác (cài lại vào
// thư mục khác). Đã trỏ đúng thì không ghi gì.
//
// Máy diskless mất tệp này sau mỗi lần khởi động, nên dịch vụ phải tự dựng lại
// — bộ cài chỉ tạo được một lần.
func ensureVNETShortcut() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	lnk := filepath.Join(publicDesktopDir(), vnetShortcutName+".lnk")
	fi, statErr := os.Lstat(lnk)
	if statErr == nil && !fi.Mode().IsRegular() {
		// Liên kết mềm, junction hay thư mục đặt tên VNET.lnk: ghi đè bằng quyền SYSTEM
		// sẽ ghi xuyên qua nó. Gỡ chính nó đi rồi dựng tệp thường.
		if err := os.Remove(lnk); err != nil {
			return
		}
		statErr = os.ErrNotExist
		lnkChecked = false
	}
	if statErr == nil && lnkChecked {
		return
	}

	// COM gắn với luồng hệ điều hành: khoá luồng suốt từ lúc khởi tạo tới lúc gỡ.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == syscall.Errno(1) { // 1 = S_FALSE
		defer windows.CoUninitialize()
	}

	if statErr == nil {
		if target, err := readLinkTarget(lnk); err == nil && strings.EqualFold(filepath.Clean(target), filepath.Clean(exe)) {
			lnkChecked = true
			return
		}
	}
	if err := writeLink(lnk, exe, filepath.Dir(exe), vnetShortcutName); err != nil {
		if msg := err.Error(); msg != lnkLastErr {
			lnkLastErr = msg
			log.Printf("[shortcut] không tạo được %s: %v", lnk, err)
		}
		return
	}
	lnkChecked, lnkLastErr = true, ""
	log.Printf("[shortcut] đã ghi %s → %s", lnk, exe)
}

// Lối tắt .lnk là đối tượng COM ShellLink; không dùng PowerShell hay thư viện
// ngoài mà gọi thẳng qua bảng hàm ảo (vtable). Số thứ tự phương thức theo
// shobjidl_core.h (IShellLinkW) và objidl.h (IPersistFile), tính cả ba hàm của
// IUnknown ở đầu.
const (
	lnkQueryInterface = 0 // IUnknown::QueryInterface
	lnkRelease        = 2 // IUnknown::Release

	lnkGetPath         = 3  // IShellLinkW::GetPath
	lnkSetDescription  = 7  // IShellLinkW::SetDescription
	lnkSetWorkingDir   = 9  // IShellLinkW::SetWorkingDirectory
	lnkSetIconLocation = 17 // IShellLinkW::SetIconLocation
	lnkSetPath         = 20 // IShellLinkW::SetPath
	lnkPersistFileLoad = 5  // IPersistFile::Load
	lnkPersistFileSave = 6  // IPersistFile::Save
)

var (
	lnkCoCreateInstance = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")

	lnkCLSIDShellLink = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	lnkIIDShellLinkW  = windows.GUID{Data1: 0x000214F9, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	lnkIIDPersistFile = windows.GUID{Data1: 0x0000010B, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// lnkCall gọi phương thức số slot của một đối tượng COM và đổi HRESULT lỗi thành
// error. Chỉ thị bên dưới giữ sống và không cho dời khỏi stack những biến mà nơi
// gọi đổi sang uintptr để truyền vào — thiếu nó, stack nở ra giữa chừng là COM
// ghi vào một địa chỉ đã cũ.
//
//go:uintptrescapes
func lnkCall(obj unsafe.Pointer, slot uintptr, args ...uintptr) error {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, slot*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	if int32(r) < 0 {
		return fmt.Errorf("HRESULT 0x%08X", uint32(r))
	}
	return nil
}

// lnkOpen dựng một ShellLink rỗng cùng giao diện IPersistFile của nó. Người gọi
// phải thả cả hai bằng lnkClose. COM phải đã được khởi tạo trên luồng này.
func lnkOpen() (link, file unsafe.Pointer, err error) {
	r, _, _ := lnkCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&lnkCLSIDShellLink)), 0, windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&lnkIIDShellLinkW)), uintptr(unsafe.Pointer(&link)))
	if int32(r) < 0 {
		return nil, nil, fmt.Errorf("CoCreateInstance(ShellLink): HRESULT 0x%08X", uint32(r))
	}
	if err := lnkCall(link, lnkQueryInterface,
		uintptr(unsafe.Pointer(&lnkIIDPersistFile)), uintptr(unsafe.Pointer(&file))); err != nil {
		_ = lnkCall(link, lnkRelease)
		return nil, nil, fmt.Errorf("QueryInterface(IPersistFile): %w", err)
	}
	return link, file, nil
}

func lnkClose(link, file unsafe.Pointer) {
	_ = lnkCall(file, lnkRelease)
	_ = lnkCall(link, lnkRelease)
}

// readLinkTarget đọc đích của một tệp .lnk có sẵn.
func readLinkTarget(lnk string) (string, error) {
	link, file, err := lnkOpen()
	if err != nil {
		return "", err
	}
	defer lnkClose(link, file)

	const stgmRead = 0
	if err := lnkCall(file, lnkPersistFileLoad, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(lnk))), stgmRead); err != nil {
		return "", fmt.Errorf("Load: %w", err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	if err := lnkCall(link, lnkGetPath, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0, 0); err != nil {
		return "", fmt.Errorf("GetPath: %w", err)
	}
	return windows.UTF16ToString(buf), nil
}

// writeLink ghi tệp .lnk trỏ tới target, chạy ở workDir, lấy biểu tượng của
// chính target. Ghi đè nếu tệp đã có.
func writeLink(lnk, target, workDir, desc string) error {
	link, file, err := lnkOpen()
	if err != nil {
		return err
	}
	defer lnkClose(link, file)

	if err := lnkCall(link, lnkSetPath, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(target)))); err != nil {
		return fmt.Errorf("SetPath: %w", err)
	}
	if err := lnkCall(link, lnkSetWorkingDir, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(workDir)))); err != nil {
		return fmt.Errorf("SetWorkingDirectory: %w", err)
	}
	if err := lnkCall(link, lnkSetDescription, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(desc)))); err != nil {
		return fmt.Errorf("SetDescription: %w", err)
	}
	if err := lnkCall(link, lnkSetIconLocation, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(target))), 0); err != nil {
		return fmt.Errorf("SetIconLocation: %w", err)
	}
	const fRemember = 1
	if err := lnkCall(file, lnkPersistFileSave, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(lnk))), fRemember); err != nil {
		return fmt.Errorf("Save: %w", err)
	}
	return nil
}
