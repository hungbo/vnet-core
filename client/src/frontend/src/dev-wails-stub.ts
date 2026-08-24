/**
 * Lớp giả cho runtime của Wails, CHỈ nạp khi chạy `npm run dev` ngoài Wails.
 *
 * Không có nó thì giao diện máy khách không mở được trong trình duyệt: mọi
 * component gọi `runtime.EventsOn` lúc mounted sẽ ném lỗi và cả cây ngừng vẽ.
 * Mà máy khách chỉ chạy thật trên Windows, nên đây là cách DUY NHẤT để xem lại
 * bố cục và màu sắc từ máy khác.
 *
 * Dữ liệu dưới đây là hàng giả có chủ đích: đủ để dựng màn hình, không giả vờ
 * nói chuyện được với máy chủ.
 */
declare const window: any

const DEV_SERVER = 'http://localhost:8080'

const DEV_CATEGORIES = [
	{ id: 'c-mi', name: 'Mì' },
	{ id: 'c-com', name: 'Cơm' },
	{ id: 'c-nuoc', name: 'Nước uống' },
	{ id: 'c-vat', name: 'Đồ ăn vặt' },
]

const DEV_MENU = [
	{ id: 'p1', category_id: 'c-mi', name: 'Mì Hảo Hảo tôm chua cay', price: 15000, image_url: `${DEV_SERVER}/uploads/seed/mi-01.svg` },
	{ id: 'p2', category_id: 'c-mi', name: 'Mì xào bò', price: 45000, image_url: `${DEV_SERVER}/uploads/seed/mi-11.svg` },
	{ id: 'p3', category_id: 'c-mi', name: 'Hủ tiếu Nam Vang ăn liền', price: 25000, image_url: `${DEV_SERVER}/uploads/seed/mi-20.svg` },
	{ id: 'p4', category_id: 'c-com', name: 'Cơm rang dưa bò', price: 45000, image_url: `${DEV_SERVER}/uploads/seed/com-01.svg` },
	{ id: 'p5', category_id: 'c-com', name: 'Cơm sườn trứng ốp la', price: 55000, image_url: `${DEV_SERVER}/uploads/seed/com-07.svg` },
	{ id: 'p6', category_id: 'c-nuoc', name: 'Coca-Cola lon 330ml', price: 15000, image_url: `${DEV_SERVER}/uploads/seed/nuoc-uong-03.svg` },
	{ id: 'p7', category_id: 'c-nuoc', name: 'Trà sữa trân châu', price: 30000, image_url: `${DEV_SERVER}/uploads/seed/nuoc-uong-26.svg` },
	{ id: 'p8', category_id: 'c-vat', name: 'Khoai tây chiên', price: 30000, image_url: `${DEV_SERVER}/uploads/seed/do-an-vat-14.svg` },
	{ id: 'p9', category_id: 'c-vat', name: 'Xúc xích Đức Việt nướng', price: 15000, image_url: `${DEV_SERVER}/uploads/seed/do-an-vat-10.svg` },
	// Một món cố tình sai đường dẫn: chỗ duy nhất nhìn được đường lui khi ảnh 404.
	{ id: 'p10', category_id: 'c-vat', name: 'Món thiếu ảnh', price: 9000, image_url: `${DEV_SERVER}/uploads/seed/khong-ton-tai.svg` },
]

let nguoiDangDangNhap = { id: 'dev', username: 'khach', full_name: 'Khách Thử', role: 'member' }

export function installDevWailsStub() {
	if (window.runtime || window.go) return

	// Giữ lại handler thay vì nuốt đi, và mở một cửa để bắn sự kiện từ Console:
	//
	//   __vnetEmit('vnet:balance:updated', { balance: 120000 })
	//
	// Không có nó thì phần realtime của giao diện máy trạm — số dư tụt mỗi phút,
	// tin nhắn tới, phiên bị đóng — không xem được ở đâu ngoài Windows.
	const handlers: Record<string, Function[]> = {}
	window.__vnetEmit = (event: string, data: unknown) => {
		(handlers[event] || []).forEach(fn => fn(data))
		return (handlers[event] || []).length
	}

	window.runtime = {
		// EventsOnMultiple là chỗ ĐĂNG KÝ THẬT: wailsjs/runtime/runtime.js cho
		// EventsOn gọi vòng qua nó. Chỉ cài EventsOn thôi thì lớp giả này im
		// lặng không nhận gì — đúng cái bẫy mà nó sinh ra để tránh.
		EventsOnMultiple: (event: string, fn: Function) => {
			(handlers[event] ||= []).push(fn)
			return () => {
				handlers[event] = (handlers[event] || []).filter(f => f !== fn)
			}
		},
		EventsOn: (event: string, fn: Function) =>
			window.runtime.EventsOnMultiple(event, fn, -1),
		EventsOff: (event: string) => { delete handlers[event] },
		EventsEmit: () => {},
		WindowShow: () => {},
		WindowHide: () => {},
		WindowCenter: () => {},
		WindowSetAlwaysOnTop: () => {},
		WindowUnminimise: () => {},
		LogInfo: () => {},
	}

	const ok = <T>(v: T) => Promise.resolve(v)

	window.go = {
		main: {
			App: {
				GetWindowMode: () => ok(new URLSearchParams(location.search).get('window') || ''),
				GetMachineCode: () => ok('KT-01'),
				GetServerURL: () => ok('http://localhost:8080'),
				// Trả về đúng người vừa đăng nhập, không phải một chuỗi cứng:
				// Dashboard đọc hàm này để dựng màn hình, nên nếu nó luôn nói
				// "khách" thì không bao giờ xem được màn hình nhân viên.
				GetUserInfo: () => ok(JSON.stringify(nguoiDangDangNhap)),
				GetMemberInfo: () => ok(JSON.stringify({ balance: 120000, bonus_balance: 5000 })),
				GetSession: () =>
					ok(
						JSON.stringify({
							started_at: new Date(Date.now() - 30 * 60000).toISOString(),
							price_per_hour: 20000,
							charged_amount: 10000,
							affordable_until: new Date(Date.now() + 90 * 60000).toISOString(),
						})
					),
				GetFeatureFlags: () => ok(JSON.stringify({ attendance_enabled: true, feedback_enabled: true })),
				GetTopupPresets: () => ok(JSON.stringify([10000, 20000, 50000, 100000])),
				GetNotifications: () => ok(JSON.stringify([])),
				GetHardware: () => ok(JSON.stringify({ machine_code: 'KT-01', server_url: 'http://localhost:8080' })),
				GetVersion: () => ok('dev'),
				GetProducts: () => ok(JSON.stringify([])),
				GetCategories: () => ok(JSON.stringify(DEV_CATEGORIES)),
				// Ảnh trỏ thẳng vào máy chủ đang chạy vì đó là điều thật sự cần
				// nhìn thấy: ngoài Wails không có gì tự đổi đường dẫn tương đối
				// thành tuyệt đối hộ (việc đó do GetMenu bên Go làm).
				GetMenu: (categoryId: string) =>
					ok(JSON.stringify(DEV_MENU.filter(p => !categoryId || p.category_id === categoryId))),
				RestoreSession: () => ok(''),
				SetLoggedIn: () => ok(undefined),
				HasMaintenancePin: () => ok(true),
				HasCachedStaff: () => ok(true),
				HasBuiltinAdmin: () => ok(true),
				// Một ô cho cả hai loại, đúng như máy chủ làm: tên tài khoản
				// quyết định đường đi.
				//   quanly  → nhân viên   · khach → hội viên   · offline → bản lưu
				Login: (u: string, p: string) => {
					if (p !== '123456') return Promise.reject(new Error('sai tài khoản hoặc mật khẩu'))
					if (u === 'offline') return ok(JSON.stringify({ offline: true }))
					const staff = u === 'quanly'
					nguoiDangDangNhap = {
						id: 'dev', username: u,
						full_name: staff ? 'Quản Lý Thử' : 'Khách Thử',
						role: staff ? 'admin' : 'member',
					}
					return ok(JSON.stringify({
						access_token: 'dev-token',
						kind: staff ? 'staff' : 'member',
						user: {
							id: 'dev', username: u,
							full_name: staff ? 'Quản Lý Thử' : 'Khách Thử',
							role: staff ? 'manager' : 'member',
						},
					}))
				},
				// PIN thử: 246810. Lớp giả không băm gì cả — nó chỉ để xem giao
				// diện, phần kiểm PIN thật nằm ở pin_test.go.
				UnlockMaintenance: (pin: string) =>
					pin === '246810' ? ok(undefined) : Promise.reject(new Error('PIN không đúng')),
				// Nút "Đăng nhập" phải bấm được, nếu không thì lớp giả này chỉ
				// xem được đúng màn hình khoá — mọi thứ phía sau vẫn khuất.

				SetServerURL: () => ok(''),
				OpenWindow: (mode: string) => {
					window.open(`${location.pathname}?window=${mode}`, '_blank')
					return ok(undefined)
				},
				ShowWindow: () => ok(undefined),
				Logout: () => ok(''),
			},
		},
	}
}
