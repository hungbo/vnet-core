package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// Một tệp .exe, bốn chế độ:
//
//	vnet-client.exe                    thanh điều khiển dán mép phải màn hình
//	vnet-client.exe --window order     cửa sổ thực đơn, tách rời
//	vnet-client.exe --window support   cửa sổ hỗ trợ, tách rời
//	vnet-client.exe --window topup     cửa sổ nạp tiền, tách rời
//	vnet-client.exe --service          tiến trình nền chạy như dịch vụ Windows
//
// Cửa sổ phụ phải là tiến trình riêng vì Wails v2 KHÔNG hỗ trợ đa cửa sổ: mọi
// hàm runtime.Window* nhận đúng một context của một cửa sổ duy nhất. Đây không
// phải lựa chọn kiến trúc, đây là giới hạn của thư viện.
//
// Chế độ --watch cũ đã bỏ. Nó gọi endpoint bằng mã máy trong khi route nhận
// UUID, lại không gắn token nên bị chặn 401, và httpPost nuốt lỗi — im lặng thất
// bại 100% số lần. Dịch vụ nền thay thế nó và làm được việc đó thật.
func main() {
	var (
		windowMode  = flag.String("window", "", "mở cửa sổ phụ: order | support")
		serviceMode = flag.Bool("service", false, "chạy như tiến trình nền")
		install     = flag.Bool("install-service", false, "đăng ký dịch vụ Windows")
		uninstall   = flag.Bool("uninstall-service", false, "gỡ dịch vụ Windows")
		setPin      = flag.String("set-pin", "", "đặt PIN kỹ thuật cho máy này")
		ensureSvc   = flag.Bool("ensure-service", false, "bật lại dịch vụ nếu nó đang dừng (tác vụ theo lịch gọi)")
	)
	flag.Parse()

	switch {
	case *ensureSvc:
		if err := ensureServiceRunning(); err != nil {
			log.Fatalf("không bảo đảm được dịch vụ đang chạy: %v", err)
		}
		return
	case *setPin != "":
		if err := luuPinKyThuat(*setPin); err != nil {
			log.Fatalf("đặt PIN thất bại: %v", err)
		}
		log.Printf("đã đặt PIN kỹ thuật cho máy này")
		return
	case *install:
		if err := installService(); err != nil {
			log.Fatalf("đăng ký dịch vụ thất bại: %v", err)
		}
		return
	case *uninstall:
		if err := uninstallService(); err != nil {
			log.Fatalf("gỡ dịch vụ thất bại: %v", err)
		}
		return
	case *serviceMode:
		runService()
		return
	}

	if *windowMode != "" {
		runChildWindow(*windowMode)
		return
	}

	runDock()
}

// runDock chạy thanh điều khiển chính: cột hẹp dán mép phải, luôn nổi trên các
// cửa sổ khác để khách vẫn thấy giờ và số dư khi đang chơi game toàn màn hình.
func runDock() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "VNET",
		Width:  dockWidth,
		Height: 900,
		// Có viền để lấy đúng ba nút của Windows ở góc trên: thu nhỏ và tắt là
		// thứ ai cũng biết bấm, không phải học một nút tự vẽ.
		//
		// Vẫn không cho đổi kích thước: đây là thanh công cụ, không phải cửa sổ
		// tài liệu. Kéo giãn được thì khách kéo lệch khỏi mép và thanh mất luôn
		// ý nghĩa. DisableResize cũng làm mờ luôn nút phóng to (winc gọi
		// EnableMaxButton(!DisableResize)), nên chỉ còn lại thu nhỏ và tắt.
		Frameless:     false,
		DisableResize: true,
		AlwaysOnTop:   true,
		// Bấm dấu X là giấu đi chứ không thoát: thoát hẳn là mất đồng hồ tính
		// tiền và mất đường nhận lệnh khoá máy. Giấu rồi thì chạy lại lối tắt
		// là hiện ra — SingleInstanceLock ngay dưới gọi ShowWindow.
		HideWindowOnClose: true,
		AssetServer:       &assetserver.Options{Assets: assets},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "vnet-client-dock",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.ShowWindow()
			},
		},
		OnStartup: func(ctx context.Context) {
			// KHÔNG gọi dockToRightEdge ở đây. Startup tự quyết hình dạng cửa
			// sổ theo trạng thái đăng nhập, và lúc khởi động thì trạng thái đó
			// là "chưa ai đăng nhập" → phủ kín màn hình. Dán mép phải ngay sau
			// đó là gỡ luôn cái khoá vừa đặt.
			app.Startup(ctx)
		},
		// Đặt lại hình dạng cửa sổ MỘT LẦN NỮA ở đây, và đây mới là lần ăn thua.
		//
		// Wails gọi OnStartup trong một goroutine (internal/frontend/desktop/
		// windows/frontend.go), song song với luồng chính đang gọi
		// f.WindowCenter() của chính nó. Ai xong sau thì thắng — nên vị trí đặt
		// trong OnStartup hay bị ghi đè bằng vị trí căn giữa tính theo kích
		// thước khai trong options, và cửa sổ hiện ra lệch hẳn.
		//
		// OnDomReady chạy trong navigationCompleted, tức là SAU khi WindowCenter
		// đã chạy xong từ lâu và ngay trước lúc cửa sổ được hiện lên.
		OnDomReady: func(ctx context.Context) {
			app.ApplyWindowState()
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// dockWidth là bề rộng thanh điều khiển. Bố cục lưới hai cột trong Dashboard
// được tính theo con số này.
const dockWidth = 360

// Tỉ lệ so với màn hình, dùng cho cửa sổ phụ. Thanh dọc KHÔNG dùng tỉ lệ nữa:
// nó cao hết vùng làm việc, đo bằng doVienTaskbar.
const (
	childWidthRatio  = 0.70 // cửa sổ gọi món / hỗ trợ
	childHeightRatio = 0.80
)

// vienTaskbar là phần bị thanh taskbar chiếm ở mỗi mép màn hình.
//
// Bốn mép chứ không phải một con số chiều cao: taskbar dựng được ở cả trên,
// dưới, trái và phải, mà máy quán thì cấu hình kiểu gì cũng có.
type vienTaskbar struct {
	trai, tren, phai, duoi int
}

// manHinhHienTai trả về màn hình đang chứa cửa sổ. Quán có đủ loại độ phân giải
// và có máy hai màn hình, nên không được lấy bừa cái đầu tiên.
func manHinhHienTai(ctx context.Context) (runtime.Screen, bool) {
	screens, err := runtime.ScreenGetAll(ctx)
	if err != nil || len(screens) == 0 {
		return runtime.Screen{}, false
	}

	screen := screens[0]
	for _, s := range screens {
		if s.IsCurrent {
			screen = s
			break
		}
	}
	if screen.Size.Width <= 0 || screen.Size.Height <= 0 {
		return runtime.Screen{}, false
	}
	return screen, true
}

// dockToRightEdge dán thanh điều khiển vào mép phải VÙNG LÀM VIỆC và kéo cao
// hết vùng đó — chạm đúng mép thanh taskbar, không đè lên nó.
//
// Bản trước cao 70% và căn giữa: đó là cách né taskbar bằng cách đoán, vì hồi
// đó chưa hỏi Windows xem taskbar nằm đâu. Đo được rồi thì không cần chừa nữa.
//
// Phải làm lúc chạy chứ không khai sẵn trong options: kích thước màn hình chỉ
// biết được sau khi cửa sổ đã tạo.
func dockToRightEdge(ctx context.Context) {
	screen, ok := manHinhHienTai(ctx)
	if !ok {
		return
	}

	wx, wy, ww, wh := vungLamViec(screen.Size.Width, screen.Size.Height,
		screen.PhysicalSize.Width, screen.PhysicalSize.Height, doVienTaskbar())
	w, h, x, y := dockGeometry(wx, wy, ww, wh)
	runtime.WindowSetSize(ctx, w, h)
	runtime.WindowSetPosition(ctx, x, y)
}

// vungLamViec quy vùng làm việc về ĐƠN VỊ LOGIC — cùng hệ toạ độ mà Wails dùng
// cho WindowSetSize/WindowSetPosition.
//
// Phải quy đổi vì hai bên nói hai thứ tiếng: doVienTaskbar hỏi thẳng Win32 nên
// trả pixel VẬT LÝ, còn Wails nhận pixel LOGIC (Screen.Size = PhysicalSize đã
// chia cho tỉ lệ phóng to). Máy để 150% mà bỏ bước này thì trừ dư gấp rưỡi, và
// lỗi đó chỉ lộ ra trên máy có DPI khác 100%.
//
// Vùng tính ra mà rỗng hoặc âm thì trả về nguyên màn hình: thà thanh đè lên
// taskbar còn hơn thanh có chiều cao 0 và khách không thấy gì.
func vungLamViec(logicW, logicH, vatLyW, vatLyH int, vien vienTaskbar) (x, y, w, h int) {
	quyDoi := func(v, logic, vatLy int) int {
		if vatLy <= 0 {
			return v
		}
		return v * logic / vatLy
	}

	x = quyDoi(vien.trai, logicW, vatLyW)
	y = quyDoi(vien.tren, logicH, vatLyH)
	w = logicW - x - quyDoi(vien.phai, logicW, vatLyW)
	h = logicH - y - quyDoi(vien.duoi, logicH, vatLyH)

	if w <= 0 {
		x, w = 0, logicW
	}
	if h <= 0 {
		y, h = 0, logicH
	}
	return x, y, w, h
}

// dockGeometry tách riêng phần tính toán khỏi phần gọi Wails.
//
// Đây là thứ duy nhất trong cả nhóm cửa sổ kiểm chứng được mà không cần một cỗ
// máy Windows — và cũng là chỗ dễ sai nhất: lệch một phép chia là cửa sổ nằm
// ngoài màn hình, mà lỗi đó chỉ lộ ra khi đã cài lên máy thật.
func dockGeometry(workX, workY, workW, workH int) (w, h, x, y int) {
	w = dockWidth
	if w > workW {
		w = workW
	}
	return w, workH, workX + workW - w, workY
}

// canhGiuaManHinh đặt cửa sổ phụ vào GIỮA màn hình với kích thước theo tỉ lệ.
//
// Tự tính thay vì gọi runtime.WindowCenter: cửa sổ phụ do thanh điều khiển ở mép
// phải sinh ra, và WindowCenter căn theo màn hình mà cửa sổ ĐANG nằm — nên nó
// hay dừng lại ngay trên chính thanh điều khiển thay vì ra giữa.
func canhGiuaManHinh(ctx context.Context, minW, minH int) {
	screen, ok := manHinhHienTai(ctx)
	if !ok {
		return
	}

	w, h, x, y := centeredGeometry(screen.Size.Width, screen.Size.Height, minW, minH)
	runtime.WindowSetSize(ctx, w, h)
	runtime.WindowSetPosition(ctx, x, y)
}

// centeredGeometry tính kích thước theo tỉ lệ rồi căn giữa.
func centeredGeometry(screenW, screenH, minW, minH int) (w, h, x, y int) {
	w = int(float64(screenW) * childWidthRatio)
	h = int(float64(screenH) * childHeightRatio)

	// Màn hình nhỏ thì tỉ lệ có thể ra nhỏ hơn kích thước tối thiểu của cửa sổ;
	// khi đó hệ điều hành kéo cửa sổ to lại nhưng toạ độ đã tính theo số cũ, và
	// cửa sổ lệch hẳn sang một bên.
	if w < minW {
		w = minW
	}
	if h < minH {
		h = minH
	}
	// Vẫn không được vượt quá màn hình, nếu không thì x hoặc y ra số âm.
	if w > screenW {
		w = screenW
	}
	if h > screenH {
		h = screenH
	}

	return w, h, (screenW - w) / 2, (screenH - h) / 2
}

// runChildWindow mở một cửa sổ phụ (thực đơn hoặc hỗ trợ) trong tiến trình
// riêng của nó.
//
// SingleInstanceLock làm hai việc cùng lúc: chặn mở hai cửa sổ thực đơn, và cho
// tiến trình khác "gõ cửa" để đánh thức cửa sổ đang chạy. Đó chính là cơ chế
// hiện cửa sổ hỗ trợ lên trên khi có tin nhắn tới — không phải viết IPC riêng.
func runChildWindow(mode string) {
	title := "VNET · Gọi món"
	switch mode {
	case "support":
		title = "VNET · Hỗ trợ"
	case "topup":
		title = "VNET · Nạp tiền"
	}

	app := NewApp()
	app.windowMode = mode

	err := wails.Run(&options.App{
		Title: title,
		// Kích thước thật do canhGiuaManHinh đặt lại theo tỉ lệ màn hình ngay khi
		// cửa sổ dựng xong; mấy con số này chỉ là chỗ dựa trước lần đầu vẽ.
		Width:       960,
		Height:      680,
		MinWidth:    640,
		MinHeight:   480,
		AssetServer: &assetserver.Options{Assets: assets},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "vnet-client-window-" + mode,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.ShowWindow()
			},
		},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			canhGiuaManHinh(ctx, 640, 480)
		},
		// Xem ghi chú ở runDock: OnStartup đua với WindowCenter của Wails, nên
		// lần đặt vị trí ăn thua là lần này.
		OnDomReady: func(ctx context.Context) {
			canhGiuaManHinh(ctx, 640, 480)
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// luuPinKyThuat băm PIN rồi ghi vào config.json cạnh .exe.
//
// Ghi bằng một lệnh riêng chứ không để bộ cài tự ghi: bộ cài Inno Setup không
// băm được, mà ghi PIN trần vào tệp thì khách đọc được ngay — tệp nằm trong
// Program Files, chặn ghi chứ không chặn đọc.
//
// Đọc tệp cũ rồi ghi lại để giữ nguyên những trường khác; ghi đè cả tệp là mất
// địa chỉ máy chủ và mã máy.
func luuPinKyThuat(pin string) error {
	hash, err := hashPin(pin)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(exe), "config.json")

	fc := loadFileConfig()
	fc.MaintenancePin = hash

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	// 0600: chỉ chủ sở hữu đọc được. Trên Windows quyền thật do ACL của thư mục
	// quyết định, nhưng bản chạy trên Linux/macOS lúc phát triển thì có tác dụng.
	return os.WriteFile(path, data, 0o600)
}
