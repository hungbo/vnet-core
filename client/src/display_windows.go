//go:build windows

package main

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Đổi độ phân giải và chuột — phần Win32. Lý do, phạm vi (chỉ màn hình chính) và
// quyết định ở display.go.
//
// Số đo ở đây là pixel VẬT LÝ của card đồ hoạ, không phụ thuộc DPI của tiến trình
// (khác GetSystemMetrics, xem curtain_windows.go). Mọi thay đổi là ĐỘNG: cờ
// ChangeDisplaySettingsEx bằng 0, không ghi registry, nên khởi động lại là về mặc
// định của Windows — khỏi lo máy kẹt ở chế độ khách chọn nhầm.

var (
	displayUser32                = windows.NewLazySystemDLL("user32.dll")
	procEnumDisplayDevicesW      = displayUser32.NewProc("EnumDisplayDevicesW")
	procEnumDisplaySettingsExW   = displayUser32.NewProc("EnumDisplaySettingsExW")
	procChangeDisplaySettingsExW = displayUser32.NewProc("ChangeDisplaySettingsExW")
)

const (
	enumCurrentSettings  = 0xFFFFFFFF // ENUM_CURRENT_SETTINGS = (DWORD)-1
	enumRegistrySettings = 0xFFFFFFFE // ENUM_REGISTRY_SETTINGS = (DWORD)-2

	// Bit của dmFields: trường nào của DEVMODE đang có giá trị.
	dmFieldDisplayOrientation = 0x00000080
	dmFieldBitsPerPel         = 0x00040000
	dmFieldPelsWidth          = 0x00080000
	dmFieldPelsHeight         = 0x00100000
	dmFieldDisplayFlags       = 0x00200000
	dmFieldDisplayFrequency   = 0x00400000
	dmInterlaced              = 0x2 // bit của dmDisplayFlags (DM_INTERLACED)

	cdsTest = 0x00000002 // CDS_TEST: chỉ hỏi "đổi được không", chưa đổi

	ddPrimaryDevice = 0x00000004 // DISPLAY_DEVICE_PRIMARY_DEVICE

	spiGetMouse      = 0x0003
	spiSetMouse      = 0x0004
	spiGetMouseSpeed = 0x0070
	spiSetMouseSpeed = 0x0071

	// dmSize của DEVMODEW: 220 byte trên cả 32 lẫn 64 bit (cấu trúc không có con
	// trỏ nên không đệm theo kiến trúc).
	devModeSize       = 220
	displayDeviceSize = 840
)

// devModeW khớp bố cục DEVMODEW của Win32. Hai union (cấu hình in ấn / vị trí màn
// hình) được trải phẳng theo nhánh màn hình: dmPosition, dmDisplayOrientation,
// dmDisplayFixedOutput chiếm đúng 16 byte của union ở offset 76. Tên trường giữ
// nguyên tên Win32 để đối chiếu với wingdi.h.
type devModeW struct {
	dmDeviceName         [32]uint16 // 0
	dmSpecVersion        uint16     // 64
	dmDriverVersion      uint16     // 66
	dmSize               uint16     // 68
	dmDriverExtra        uint16     // 70
	dmFields             uint32     // 72
	dmPositionX          int32      // 76
	dmPositionY          int32      // 80
	dmDisplayOrientation uint32     // 84
	dmDisplayFixedOutput uint32     // 88
	dmColor              int16      // 92
	dmDuplex             int16      // 94
	dmYResolution        int16      // 96
	dmTTOption           int16      // 98
	dmCollate            int16      // 100
	dmFormName           [32]uint16 // 102
	dmLogPixels          uint16     // 166
	dmBitsPerPel         uint32     // 168
	dmPelsWidth          uint32     // 172
	dmPelsHeight         uint32     // 176
	dmDisplayFlags       uint32     // 180
	dmDisplayFrequency   uint32     // 184
	dmICMMethod          uint32     // 188
	dmICMIntent          uint32     // 192
	dmMediaType          uint32     // 196
	dmDitherType         uint32     // 200
	dmReserved1          uint32     // 204
	dmReserved2          uint32     // 208
	dmPanningWidth       uint32     // 212
	dmPanningHeight      uint32     // 216
}

// displayDeviceW khớp DISPLAY_DEVICEW.
type displayDeviceW struct {
	cb           uint32      // 0
	deviceName   [32]uint16  // 4
	deviceString [128]uint16 // 68
	stateFlags   uint32      // 324
	deviceID     [128]uint16 // 328
	deviceKey    [128]uint16 // 584
}

// Bố cục kiểm LÚC BIÊN DỊCH: lệch một byte là build hỏng ngay tại máy dev, không phải
// một chế độ màn hình sai chỉ lộ ra trên máy khách. Kích thước thì so độ dài mảng; mỗi
// offset là chỉ số vào mảng một phần tử, nên chỉ bằng đúng giá trị Win32 mới biên
// dịch được (lệch thì báo "index out of bounds" hoặc "overflows").
type layoutCheck [1]struct{}

var (
	_ [devModeSize]byte       = [unsafe.Sizeof(devModeW{})]byte{}
	_ [displayDeviceSize]byte = [unsafe.Sizeof(displayDeviceW{})]byte{}

	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmFields)-72]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmPositionX)-76]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmDisplayOrientation)-84]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmDisplayFixedOutput)-88]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmFormName)-102]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmBitsPerPel)-168]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmPelsWidth)-172]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmPelsHeight)-176]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmDisplayFlags)-180]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmDisplayFrequency)-184]
	_ = layoutCheck{}[unsafe.Offsetof(devModeW{}.dmPanningHeight)-216]

	_ = layoutCheck{}[unsafe.Offsetof(displayDeviceW{}.deviceString)-68]
	_ = layoutCheck{}[unsafe.Offsetof(displayDeviceW{}.stateFlags)-324]
	_ = layoutCheck{}[unsafe.Offsetof(displayDeviceW{}.deviceKey)-584]
)

func osDisplayHW() displayHW {
	return displayHW{
		modes:          winDisplayModes,
		setMode:        winSetDisplayMode,
		registryMode:   winRegistryMode,
		restoreDefault: winRestoreDefault,
		getMouse:       winGetMouse,
		setSpeed:       winSetMouseSpeed,
		setPrecision:   winSetMousePrecision,
	}
}

// primaryDisplayName tìm tên thiết bị (\\.\DISPLAY1, ...) của màn hình chính. Hỏi
// lại mỗi lần chứ không nhớ: cắm/rút màn hình có thể đổi màn nào là chính.
func primaryDisplayName() ([32]uint16, error) {
	for i := uintptr(0); ; i++ {
		var dd displayDeviceW
		dd.cb = displayDeviceSize
		ok, _, _ := procEnumDisplayDevicesW.Call(0, i, uintptr(unsafe.Pointer(&dd)), 0)
		if ok == 0 {
			return [32]uint16{}, errors.New("Không tìm thấy màn hình chính")
		}
		if dd.stateFlags&ddPrimaryDevice != 0 {
			return dd.deviceName, nil
		}
	}
}

// enumDisplaySettings đọc chế độ số mode của thiết bị (hoặc enumCurrentSettings).
//
// dmSize phải được đặt TRƯỚC khi gọi: Windows từ chối cấu trúc không khai cỡ.
// dmDriverExtra bằng 0 báo "sau cấu trúc không có vùng đệm cho dữ liệu riêng của
// driver" — đúng, vì devModeW không có đuôi — nên Windows chỉ điền phần chuẩn và
// kết quả đưa thẳng lại cho ChangeDisplaySettingsEx được.
func enumDisplaySettings(device *uint16, mode uintptr) (dm devModeW, ok bool) {
	dm.dmSize = devModeSize
	r, _, _ := procEnumDisplaySettingsExW.Call(
		uintptr(unsafe.Pointer(device)), mode, uintptr(unsafe.Pointer(&dm)), 0)
	return dm, r != 0
}

// rawFromDevMode đổi một DEVMODE sang dòng thô. Trường nào dmFields không khai thì
// KHÔNG tin giá trị của nó: hướng xoay lấy theo orientation truyền vào, xen kẽ là false.
func rawFromDevMode(dm *devModeW, orientation int) rawDisplayMode {
	r := rawDisplayMode{
		DisplayMode: DisplayMode{
			Width:  int(dm.dmPelsWidth),
			Height: int(dm.dmPelsHeight),
			Hz:     int(dm.dmDisplayFrequency),
		},
		BitsPerPel:  int(dm.dmBitsPerPel),
		Orientation: orientation,
	}
	if dm.dmFields&dmFieldDisplayOrientation != 0 {
		r.Orientation = int(dm.dmDisplayOrientation)
	}
	if dm.dmFields&dmFieldDisplayFlags != 0 {
		r.Interlaced = dm.dmDisplayFlags&dmInterlaced != 0
	}
	return r
}

// winDisplayModes liệt kê mọi chế độ của màn hình chính. Không truyền EDS_ROTATEDMODE
// nên Windows chỉ trả chế độ cùng hướng xoay hiện tại.
func winDisplayModes() ([]rawDisplayMode, rawDisplayMode, error) {
	dev, err := primaryDisplayName()
	if err != nil {
		return nil, rawDisplayMode{}, err
	}
	dm, ok := enumDisplaySettings(&dev[0], enumCurrentSettings)
	if !ok {
		return nil, rawDisplayMode{}, errors.New("Không đọc được chế độ màn hình hiện tại")
	}
	cur := rawFromDevMode(&dm, 0)

	var raw []rawDisplayMode
	for i := uintptr(0); ; i++ {
		dm, ok := enumDisplaySettings(&dev[0], i)
		if !ok {
			break
		}
		raw = append(raw, rawFromDevMode(&dm, cur.Orientation))
	}
	return raw, cur, nil
}

// winSetDisplayMode đổi sang m: hỏi CDS_TEST trước, được thì mới đổi thật.
//
// Lấy chế độ HIỆN TẠI làm nền rồi chỉ ghi đè bốn trường (bề rộng, cao, tần số, màu):
// vị trí, hướng xoay và kiểu co giãn của màn hình giữ nguyên như đang chạy, và
// dmFields chỉ khai bốn trường đó nên Windows không đụng gì ngoài chúng.
func winSetDisplayMode(m DisplayMode) error {
	dev, err := primaryDisplayName()
	if err != nil {
		return err
	}
	dm, ok := enumDisplaySettings(&dev[0], enumCurrentSettings)
	if !ok {
		return errors.New("Không đọc được chế độ màn hình hiện tại")
	}
	dm.dmSize, dm.dmDriverExtra = devModeSize, 0
	dm.dmFields = dmFieldBitsPerPel | dmFieldPelsWidth | dmFieldPelsHeight | dmFieldDisplayFrequency
	dm.dmBitsPerPel = 32
	dm.dmPelsWidth, dm.dmPelsHeight, dm.dmDisplayFrequency = uint32(m.Width), uint32(m.Height), uint32(m.Hz)

	if err := changeDisplaySettings(&dev[0], &dm, cdsTest); err != nil {
		return err
	}
	return changeDisplaySettings(&dev[0], &dm, 0)
}

// winRegistryMode đọc chế độ Windows lưu trong registry cho màn hình chính. Không kiểm
// dmFields: nơi dùng (staleDisplayMode) đã bỏ qua kích thước vô lý và coi tần số 0/1
// là "mặc định của card", nên một trường chưa khai (ra 0) không gây hại gì.
func winRegistryMode() (DisplayMode, error) {
	dev, err := primaryDisplayName()
	if err != nil {
		return DisplayMode{}, err
	}
	dm, ok := enumDisplaySettings(&dev[0], enumRegistrySettings)
	if !ok {
		return DisplayMode{}, errors.New("Không đọc được chế độ màn hình lưu trong registry")
	}
	return rawFromDevMode(&dm, 0).DisplayMode, nil
}

// winRestoreDefault: DEVMODE rỗng cùng cờ 0 là cách Windows ghi rõ để "về lại chế độ
// trong registry sau một lần đổi động" — không cần biết chế độ đó là gì.
func winRestoreDefault() error {
	dev, err := primaryDisplayName()
	if err != nil {
		return err
	}
	return changeDisplaySettings(&dev[0], nil, 0)
}

func changeDisplaySettings(device *uint16, dm *devModeW, flags uintptr) error {
	r, _, _ := procChangeDisplaySettingsExW.Call(
		uintptr(unsafe.Pointer(device)), uintptr(unsafe.Pointer(dm)), 0, flags, 0)
	return displayChangeError(int32(r))
}

// Chuột. Không dùng SPIF_UPDATEINIFILE (fWinIni = 0): đổi cho phiên hiện tại,
// không ghi vào hồ sơ người dùng — người ngồi sau không thừa hưởng, kể cả khi
// ResetDisplayAndMouse không kịp chạy.

func winGetMouse() (speed int, precision bool, err error) {
	var s uint32
	if r, _, e := procSystemParametersInfoW.Call(spiGetMouseSpeed, 0, uintptr(unsafe.Pointer(&s)), 0); r == 0 {
		return 0, false, fmt.Errorf("Không đọc được tốc độ chuột: %w", e)
	}
	var p [3]int32
	if r, _, e := procSystemParametersInfoW.Call(spiGetMouse, 0, uintptr(unsafe.Pointer(&p)), 0); r == 0 {
		return 0, false, fmt.Errorf("Không đọc được độ chính xác con trỏ: %w", e)
	}
	return int(s), mousePrecisionOn(p), nil
}

func winSetMouseSpeed(speed int) error {
	// SPI_SETMOUSESPEED nhận chính tốc độ trong pvParam (ép sang con trỏ), không phải
	// địa chỉ của nó — khác SPI_GETMOUSESPEED.
	if r, _, e := procSystemParametersInfoW.Call(spiSetMouseSpeed, 0, uintptr(speed), 0); r == 0 {
		return fmt.Errorf("Không đổi được tốc độ chuột: %w", e)
	}
	return nil
}

func winSetMousePrecision(on bool) error {
	var p [3]int32
	if r, _, e := procSystemParametersInfoW.Call(spiGetMouse, 0, uintptr(unsafe.Pointer(&p)), 0); r == 0 {
		return fmt.Errorf("Không đọc được độ chính xác con trỏ: %w", e)
	}
	p = withMousePrecision(p, on)
	if r, _, e := procSystemParametersInfoW.Call(spiSetMouse, 0, uintptr(unsafe.Pointer(&p)), 0); r == 0 {
		return fmt.Errorf("Không đổi được độ chính xác con trỏ: %w", e)
	}
	return nil
}
