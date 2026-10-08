package main

import (
	"context"
	"testing"
)

// Bốn độ phân giải hay gặp trong quán, cộng hai trường hợp cực đoan.
var manHinhThuongGap = []struct {
	ten  string
	w, h int
}{
	{"1366x768", 1366, 768},
	{"1920x1080", 1920, 1080},
	{"2560x1440", 2560, 1440},
	{"3840x2160", 3840, 2160},
	{"1024x768 cũ", 1024, 768},
}

// Thanh dọc: dán mép phải vùng làm việc và cao HẾT vùng đó — chạm mép taskbar,
// không lòi ra ngoài.
//
// Taskbar dưới cao 48px là cấu hình mặc định của Windows 11 ở 100%.
func TestDockGeometry(t *testing.T) {
	const caoTaskbar = 48

	for _, m := range manHinhThuongGap {
		t.Run(m.ten, func(t *testing.T) {
			wx, wy, ww, wh := vungLamViec(m.w, m.h, m.w, m.h, vienTaskbar{duoi: caoTaskbar})
			w, h, x, y := dockGeometry(wx, wy, ww, wh)

			if x+w != m.w {
				t.Errorf("mép phải: x+w = %d, màn rộng %d", x+w, m.w)
			}
			if y != 0 {
				t.Errorf("y = %d — phải bắt đầu từ đỉnh màn hình", y)
			}
			if h != m.h-caoTaskbar {
				t.Errorf("cao %d, mong %d (hết màn trừ taskbar)", h, m.h-caoTaskbar)
			}
			if y+h > m.h-caoTaskbar {
				t.Errorf("đè lên taskbar: y+h = %d, mép taskbar ở %d", y+h, m.h-caoTaskbar)
			}
		})
	}
}

// Taskbar dựng ở mép phải: thanh dọc phải lùi vào, không nằm đè lên nó.
func TestDockGeometry_TaskbarBenPhai(t *testing.T) {
	const dayTaskbar = 62

	wx, wy, ww, wh := vungLamViec(1920, 1080, 1920, 1080, vienTaskbar{phai: dayTaskbar})
	w, h, x, y := dockGeometry(wx, wy, ww, wh)

	if x+w != 1920-dayTaskbar {
		t.Errorf("mép phải thanh ở %d, mép taskbar ở %d", x+w, 1920-dayTaskbar)
	}
	if h != 1080 || y != 0 {
		t.Errorf("taskbar dọc không ăn chiều cao: y=%d h=%d", y, h)
	}
}

// Màn hình phóng to 150%: viền taskbar đo bằng pixel vật lý phải được quy về
// pixel logic trước khi trừ, nếu không thanh dọc hụt đúng một nửa phần đó.
func TestVungLamViec_ManHinhPhongTo(t *testing.T) {
	// 1920x1080 vật lý ở 150% → Wails báo 1280x720 logic. Taskbar 72px vật lý
	// tương đương 48px logic.
	_, y, _, h := vungLamViec(1280, 720, 1920, 1080, vienTaskbar{duoi: 72})

	if y != 0 || h != 720-48 {
		t.Errorf("y=%d h=%d — mong y=0 h=%d", y, h, 720-48)
	}
}

// Viền vô lý (taskbar dày hơn cả màn hình) không được cho ra chiều cao 0: thanh
// đè lên taskbar còn hơn thanh tàng hình.
func TestVungLamViec_VienVoLy(t *testing.T) {
	x, y, w, h := vungLamViec(1920, 1080, 1920, 1080, vienTaskbar{duoi: 2000, phai: 3000})

	if x != 0 || y != 0 || w != 1920 || h != 1080 {
		t.Errorf("(%d,%d) %dx%d — phải lùi về nguyên màn hình", x, y, w, h)
	}
}

// Cửa sổ gọi món / hỗ trợ: 70% ngang, 80% dọc, nằm giữa.
func TestCenteredGeometry(t *testing.T) {
	const minW, minH = 640, 480

	for _, m := range manHinhThuongGap {
		t.Run(m.ten, func(t *testing.T) {
			w, h, x, y := centeredGeometry(m.w, m.h, minW, minH)

			if trai, phai := x, m.w-(x+w); trai != phai && trai != phai-1 {
				t.Errorf("không căn giữa ngang: trái %d, phải %d", trai, phai)
			}
			if tren, duoi := y, m.h-(y+h); tren != duoi && tren != duoi-1 {
				t.Errorf("không căn giữa dọc: trên %d, dưới %d", tren, duoi)
			}
			if x < 0 || y < 0 || x+w > m.w || y+h > m.h {
				t.Errorf("lòi ra ngoài màn hình: %dx%d tại (%d,%d) trên màn %dx%d",
					w, h, x, y, m.w, m.h)
			}
			if w < minW || h < minH {
				t.Errorf("nhỏ hơn kích thước tối thiểu: %dx%d", w, h)
			}
		})
	}
}

// Màn hình nhỏ hơn cả kích thước tối thiểu: cửa sổ phải bị kẹp về vừa màn hình
// chứ không được để toạ độ ra số âm — cửa sổ nằm ngoài màn hình thì không kéo
// lại được, và máy trạm thì không có ai ngồi cạnh để sửa.
func TestCenteredGeometry_ManHinhNhoHonKichThuocToiThieu(t *testing.T) {
	w, h, x, y := centeredGeometry(800, 600, 1280, 1024)

	if w != 800 || h != 600 {
		t.Errorf("kích thước %dx%d — phải kẹp về vừa màn hình 800x600", w, h)
	}
	if x != 0 || y != 0 {
		t.Errorf("toạ độ (%d,%d) — phải là (0,0)", x, y)
	}
}

// Thanh dọc rộng hơn màn hình (màn dọc rất hẹp) cũng không được cho toạ độ âm.
func TestDockGeometry_ManHinhHepHonThanhDoc(t *testing.T) {
	w, _, x, _ := dockGeometry(0, 0, 300, 1000)
	if w != 300 || x != 0 {
		t.Errorf("w=%d x=%d — màn hẹp hơn thanh thì phải kẹp về 300 và x=0", w, x)
	}
}

// Cửa sổ phụ không được nằm dưới thanh dọc: thanh dọc luôn nổi trên, và phần
// bị che có nút đóng của cửa sổ.
func TestChildGeometry_KhongDeThanhDoc(t *testing.T) {
	const minW, minH = 640, 480
	for _, m := range manHinhThuongGap {
		t.Run(m.ten, func(t *testing.T) {
			w, h, x, y := childGeometry(m.w, m.h, minW, minH)
			if m.w-dockWidth >= minW && x+w > m.w-dockWidth {
				t.Errorf("cửa sổ %dx%d tại x=%d lấn %dpx vào thanh dọc", w, h, x, x+w-(m.w-dockWidth))
			}
			if x < 0 || y < 0 || x+w > m.w || y+h > m.h {
				t.Errorf("lòi ra ngoài màn hình: %dx%d tại (%d,%d)", w, h, x, y)
			}
		})
	}
}

// Các trạng thái máy mà thanh điều khiển có thể đang ở, và việc nó có được thu nhỏ
// xuống taskbar hay không. Dùng chung cho hai test dưới.
func cacTrangThaiThuNho() []struct {
	ten  string
	app  func() *App
	duoc bool
} {
	dangNhap := func() *App {
		a := &App{token: "t"}
		a.verified.Store(true)
		return a
	}
	return []struct {
		ten  string
		app  func() *App
		duoc bool
	}{
		{"màn hình đăng nhập (chưa ai vào)", func() *App { return &App{} }, false},
		{"có token nhưng máy chủ chưa xác nhận", func() *App { return &App{token: "t"} }, false},
		{"đã đăng nhập", dangNhap, true},
		{"nhân viên đã đăng nhập", func() *App { a := dangNhap(); a.role = "admin"; return a }, true},
		{"đã đăng nhập nhưng mất kết nối máy chủ", func() *App {
			a := dangNhap()
			a.offlineLocked.Store(true)
			return a
		}, false},
		{"chế độ bảo trì (không token)", func() *App {
			a := &App{}
			a.baoTri.Store(true)
			return a
		}, true},
		{"cửa sổ phụ không phải thanh điều khiển", func() *App { a := dangNhap(); a.windowMode = "panel"; return a }, false},
	}
}

// Màn hình khoá không bao giờ thu nhỏ được: thu nhỏ nó là lộ desktop cho người
// chưa đăng nhập. Chốt chặn nằm ở Go chứ không ở nút, vì giao diện chỉ đề nghị.
func TestCoTheThuNho(t *testing.T) {
	for _, c := range cacTrangThaiThuNho() {
		t.Run(c.ten, func(t *testing.T) {
			if got := c.app().coTheThuNho(); got != c.duoc {
				t.Errorf("coTheThuNho = %v, mong %v", got, c.duoc)
			}
		})
	}
}

// Lệnh đóng cửa sổ (Alt+F4, nút Đóng trên taskbar, WM_CLOSE) phải LUÔN bị chặn: trả
// false là Wails thoát tiến trình, tức mất đồng hồ tính tiền và đường nhận lệnh khoá
// máy. Chạy cả với ctx rỗng (chưa dựng cửa sổ) lẫn mọi trạng thái khoá.
func TestBeforeDockClose_KhongBaoGioChoThoat(t *testing.T) {
	for _, c := range cacTrangThaiThuNho() {
		t.Run(c.ten, func(t *testing.T) {
			if !c.app().beforeDockClose(context.Background()) {
				t.Error("beforeDockClose trả false — Wails sẽ thoát tiến trình giao diện")
			}
		})
	}
}
