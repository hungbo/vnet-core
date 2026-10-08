package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	winoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
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
		adminUser   = flag.String("set-admin-user", "", "đặt tài khoản quản trị máy trạm (đi kèm --set-admin-pass)")
		adminPass   = flag.String("set-admin-pass", "", "mật khẩu tài khoản quản trị máy trạm")
		ensureSvc   = flag.Bool("ensure-service", false, "bật lại dịch vụ nếu nó đang dừng (tác vụ theo lịch gọi)")
	)
	flag.Parse()

	// Lệnh cài/gỡ/đặt tài khoản chạy tay trên console: để nhật ký ra màn hình như cũ.
	setAdmin := *adminUser != "" || *adminPass != ""
	if *windowMode != "" || *serviceMode || (!setAdmin && !*install && !*uninstall && !*ensureSvc) {
		setupLogFile(*serviceMode, *windowMode)
	}

	switch {
	case *ensureSvc:
		if err := ensureServiceRunning(); err != nil {
			log.Fatalf("không bảo đảm được dịch vụ đang chạy: %v", err)
		}
		return
	case setAdmin:
		if err := luuTaiKhoanQuanTri(*adminUser, *adminPass); err != nil {
			log.Fatalf("đặt tài khoản quản trị máy trạm thất bại: %v", err)
		}
		log.Printf("đã đặt tài khoản quản trị máy trạm %q", strings.TrimSpace(*adminUser))
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
	// Việc đầu tiên, trước cả NewApp: dựng màn che lúc mới bật máy để khách không
	// thấy desktop trong lúc WebView2 khởi động lạnh. Hạ xuống ở OnDomReady.
	startCurtain()
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "VNET",
		Width:  dockWidth,
		Height: 900,
		// KHÔNG viền: thanh điều khiển không có nút tắt, chỉ có nút thu nhỏ.
		//
		// Viền hệ điều hành thì không làm được điều đó. Ba nút của Windows chung một
		// kiểu WS_SYSMENU: bỏ nút tắt là mất luôn thu nhỏ, còn DeleteMenu(SC_CLOSE)
		// chỉ làm nó MỜ đi chứ vẫn vẽ ra — khách vẫn thấy một nút X bấm không ăn,
		// tệ hơn không có. Nên bỏ hẳn viền: thanh tiêu đề do giao diện tự vẽ
		// (components/DockTitleBar.vue) với đúng hai thứ, vùng kéo và nút thu nhỏ.
		//
		// Cửa sổ không viền thì vùng vẽ web = TOÀN BỘ hình chữ nhật cửa sổ (Wails trả
		// 0 cho WM_NCCALCSIZE). Hình chữ nhật NGOÀI mà dockGeometry tính không đổi một
		// pixel; chỉ có phần trong là to ra đúng bằng chỗ thanh tiêu đề và viền cũ
		// từng chiếm, và DockTitleBar lấy lại chừng đó chiều cao.
		//
		// Vẫn không cho đổi kích thước: đây là thanh công cụ, không phải cửa sổ
		// tài liệu. Kéo giãn được thì khách kéo lệch khỏi mép và thanh mất luôn
		// ý nghĩa.
		Frameless:     true,
		DisableResize: true,
		AlwaysOnTop:   true,
		Windows: &winoptions.Options{
			// Không bóng đổ, không bo góc: thanh dán sát mép màn hình và mép taskbar,
			// bo góc chỉ để lộ hai khe nhỏ ở đó.
			DisableFramelessWindowDecorations: true,
			// Có khối Windows thì Wails ghi đè cài đặt thu phóng của WebView2 bằng giá
			// trị trong khối này (mặc định false). Giữ nguyên mặc định cũ của WebView2.
			IsZoomControlEnabled: true,
		},
		// Mặc định của Wails là NỀN TRẮNG cho cả cửa sổ lẫn WebView2, nên giữa lúc
		// cửa sổ hiện và lúc trang vẽ khung đầu tiên khách thấy một nháy trắng —
		// ngay trên màn che khởi động đang cố giữ màu tối. Cùng màu --vnet-bg
		// (styles/tokens.css) và curtain_windows.go.
		BackgroundColour: &options.RGBA{R: 0x0a, G: 0x0f, B: 0x1e, A: 255},
		// Mọi lệnh ĐÓNG (Alt+F4, "Đóng cửa sổ" trên hình thu nhỏ của taskbar, menu hệ
		// thống, WM_CLOSE từ bất kỳ đâu) đều thành THU NHỎ, không bao giờ ẩn hay thoát:
		// thoát hẳn là mất đồng hồ tính tiền và mất đường nhận lệnh khoá máy.
		//
		// Phải là OnBeforeClose chứ KHÔNG phải HideWindowOnClose. Wails (Windows) bắt
		// WM_CLOSE rồi bắn sự kiện OnClose, và nhánh HideWindowOnClose gọi WindowHide
		// thẳng, bỏ qua OnBeforeClose — tức là bản cũ giấu mất cửa sổ, và không có gì
		// ngoài lối tắt đưa nó về. Để false thì OnClose đi qua Quit() → OnBeforeClose,
		// trả true là Quit() dừng lại.
		//
		// Tắt máy, khởi động lại, đăng xuất KHÔNG đi qua đây: Windows chỉ gửi
		// WM_QUERYENDSESSION / WM_ENDSESSION, không gửi WM_CLOSE. Wails và ta đều không
		// xử lý hai thông điệp đó nên DefWindowProc trả "đồng ý", và dịch vụ nền vẫn
		// tắt/khởi động lại máy được. Đừng thêm xử lý cho chúng ở đây.
		OnBeforeClose: app.beforeDockClose,
		AssetServer:   &assetserver.Options{Assets: assets},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "vnet-client-dock",
			// Lối tắt VNET trên desktop chạy một bản thứ hai: nó gõ cửa bản đang chạy
			// rồi thoát. ShowWindow đưa thanh về từ trạng thái thu nhỏ; màn hình khoá
			// đang phủ kín thì giữ nguyên (xem ShowWindow).
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
			// TRƯỚC ApplyWindowState: màn che thôi tự nâng mình, để cửa sổ khoá mà
			// nó đưa lên đỉnh nhóm cửa sổ nổi không bị màn che đè lại.
			curtainStopRaise()
			app.ApplyWindowState()
			// Cửa sổ vừa được hiện: màn che hết việc (hạ sau một nhịp ngắn).
			curtainDomReady()
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

	w, h, x, y := childGeometry(screen.Size.Width, screen.Size.Height, minW, minH)
	runtime.WindowSetSize(ctx, w, h)
	runtime.WindowSetPosition(ctx, x, y)
}

// childGeometry căn cửa sổ phụ giữa phần màn hình BÊN TRÁI thanh dọc.
//
// Căn giữa cả màn hình thì cửa sổ rộng 70% lấn sang phải và nằm dưới thanh dọc
// (thanh dọc luôn nổi trên): kiểm trên máy thật, nút đóng của cửa sổ Nạp tiền
// bị thanh dọc che mất. Màn hình quá hẹp để chừa chỗ thì mới căn cả màn hình.
func childGeometry(screenW, screenH, minW, minH int) (w, h, x, y int) {
	if avail := screenW - dockWidth; avail >= minW {
		return centeredGeometry(avail, screenH, minW, minH)
	}
	return centeredGeometry(screenW, screenH, minW, minH)
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
	app := NewApp()
	// Mọi màn hình phụ chạy chung MỘT tiến trình (xem App.panelMode). Lần đầu
	// được gọi là "prewarm": dựng ẩn ngay khi khách đăng nhập, chờ sẵn.
	app.windowMode = "panel"
	title := panelTitles[mode]
	hidden := false
	if mode == "prewarm" {
		app.panelMode = "blank"
		title = "VNET"
		hidden = true
	} else {
		app.panelMode = mode
	}

	err := wails.Run(&options.App{
		Title: title,
		// Kích thước thật do canhGiuaManHinh đặt lại theo tỉ lệ màn hình ngay khi
		// cửa sổ dựng xong; mấy con số này chỉ là chỗ dựa trước lần đầu vẽ.
		Width:       960,
		Height:      680,
		MinWidth:    640,
		MinHeight:   480,
		StartHidden: hidden,
		AssetServer: &assetserver.Options{Assets: assets},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "vnet-client-panel",
			// Bấm nút trên thanh: tiến trình mới chỉ gõ cửa rồi thoát, cửa sổ
			// đang chạy đổi sang đúng màn hình và hiện lên.
			OnSecondInstanceLaunch: func(d options.SecondInstanceData) {
				if m := windowArg(d.Args); m != "" && m != "prewarm" {
					app.switchPanel(m)
				}
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
		// Bấm X chỉ ẩn: tắt hẳn là lần mở sau phải dựng lại từ đầu. Hết phiên
		// thì thanh chính giết tiến trình, hoặc Logout đặt cờ thoát.
		OnBeforeClose: func(ctx context.Context) bool {
			if app.thoat.Load() {
				return false
			}
			app.HidePanel()
			return true
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// windowArg lấy giá trị của --window trong dòng lệnh của lần gõ cửa.
func windowArg(args []string) string {
	for i, a := range args {
		if a == "--window" && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--window="); ok {
			return v
		}
	}
	return ""
}

// luuTaiKhoanQuanTri băm mật khẩu rồi ghi tài khoản vào config.json cạnh .exe.
//
// Ghi bằng một lệnh riêng chứ không để bộ cài tự ghi: bộ cài Inno Setup không
// băm được, mà ghi mật khẩu trần vào tệp thì khách đọc được ngay — tệp nằm trong
// Program Files, chặn ghi chứ không chặn đọc.
//
// Đọc tệp cũ rồi ghi lại để giữ nguyên những trường khác; ghi đè cả tệp là mất
// địa chỉ máy chủ.
func luuTaiKhoanQuanTri(user, pass string) error {
	user = strings.TrimSpace(user)
	if err := checkLocalAdminInput(user, pass); err != nil {
		return err
	}
	hash, err := hashPin(pass)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(exe), "config.json")

	fc := loadFileConfig()
	fc.LocalAdminUsername = user
	fc.LocalAdminHash = hash

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	// 0600: chỉ chủ sở hữu đọc được. Trên Windows quyền thật do ACL của thư mục
	// quyết định, nhưng bản chạy trên Linux/macOS lúc phát triển thì có tác dụng.
	return os.WriteFile(path, data, 0o600)
}
