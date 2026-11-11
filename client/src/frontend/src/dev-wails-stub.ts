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

const DEV_SERVER = 'http://localhost:20800'

const DEV_CATEGORIES = [
	{ id: 'c-mi', name: 'Mì' },
	{ id: 'c-com', name: 'Cơm' },
	{ id: 'c-nuoc', name: 'Nước uống' },
	{ id: 'c-vat', name: 'Đồ ăn vặt' },
]

const DEV_MENU = [
	{ id: 'p1', category_id: 'c-mi', name: 'Mì Hảo Hảo tôm chua cay', price: 15000, image_url: `${DEV_SERVER}/uploads/seed/mi-01.svg` },
	{ id: 'p2', category_id: 'c-mi', name: 'Mì xào bò', price: 45000, has_stock: true, current_stock: 0, image_url: `${DEV_SERVER}/uploads/seed/mi-11.svg` },
	{ id: 'p3', category_id: 'c-mi', name: 'Hủ tiếu Nam Vang ăn liền', price: 25000, image_url: `${DEV_SERVER}/uploads/seed/mi-20.svg` },
	// Cơm hết sạch: thấy được dòng nhắc chọn "Tất cả" khi cả mục đều hết hàng.
	{ id: 'p4', category_id: 'c-com', name: 'Cơm rang dưa bò', price: 45000, has_stock: true, current_stock: 0, image_url: `${DEV_SERVER}/uploads/seed/com-01.svg` },
	{ id: 'p5', category_id: 'c-com', name: 'Cơm sườn trứng ốp la', price: 55000, has_stock: true, current_stock: 0, image_url: `${DEV_SERVER}/uploads/seed/com-07.svg` },
	{ id: 'p6', category_id: 'c-nuoc', name: 'Coca-Cola lon 330ml', price: 15000, has_stock: true, current_stock: 12, image_url: `${DEV_SERVER}/uploads/seed/nuoc-uong-03.svg` },
	{ id: 'p7', category_id: 'c-nuoc', name: 'Trà sữa trân châu', price: 30000, image_url: `${DEV_SERVER}/uploads/seed/nuoc-uong-26.svg` },
	{ id: 'p8', category_id: 'c-vat', name: 'Khoai tây chiên', price: 30000, image_url: `${DEV_SERVER}/uploads/seed/do-an-vat-14.svg` },
	{ id: 'p9', category_id: 'c-vat', name: 'Xúc xích Đức Việt nướng', price: 15000, image_url: `${DEV_SERVER}/uploads/seed/do-an-vat-10.svg` },
	// Một món cố tình sai đường dẫn: chỗ duy nhất nhìn được đường lui khi ảnh 404.
	{ id: 'p10', category_id: 'c-vat', name: 'Món thiếu ảnh', price: 9000, image_url: `${DEV_SERVER}/uploads/seed/khong-ton-tai.svg` },
]

// Đủ các trạng thái menu game phải vẽ được. Tiến độ của game đang tải nhích theo
// đồng hồ để thấy lượt làm mới mỗi 10 giây có tác dụng. Nhóm như menu của quán
// (Game online / Game offline / Công cụ); Dota 2 chỉ nhận ra là Steam qua lệnh chạy.
const ONL = 'Game online'
const DEV_GAMES: Record<string, string>[] = [
	{ name: 'roblox', display_name: 'Roblox', category: ONL, status: 'ready', label: 'hot' },
	{ name: 'fo4', display_name: 'FIFA Online 4', category: ONL, status: 'ready', label: 'hot' },
	{ name: 'lol', display_name: 'Liên Minh Huyền Thoại', category: ONL, status: 'ready', label: 'hot' },
	{ name: 'valorant', display_name: 'Valorant', category: ONL, status: 'ready', label: 'new' },
	{ name: 'cs-old', display_name: 'Game mất tệp chạy', category: ONL, status: 'ready', problem: 'missing_launcher' },
	{ name: 'pubg', display_name: 'PUBG: Battlegrounds', category: ONL, status: 'downloading' },
	{ name: 'fc-online', display_name: 'FC Online', category: ONL, status: 'queued' },
	{ name: 'csgo2', display_name: 'Counter-Strike 2', category: ONL, status: 'error' },
	...[
		'Đột Kích', 'Free Fire', 'GTA5VN', 'Delta Force Garena', 'The Isle', 'Genshin Impact', 'PUBG Mobile',
		'Zenless Zone Zero', 'ZingSpeed Mobile', 'Naraka VNG', 'GTA V Epic', 'Honkai Star Rail', 'FIFA 24',
		'Truy Kích', 'Overwatch 2', 'World of Tanks ASIA', 'Wuthering Waves', 'Naraka Steam', 'ROS Legacy',
		'GTA V Steam', 'Audition', 'Palworld Steam', 'Arena Breakout Infinite', 'Phi Đội 3 VTC', 'Black Desert',
		'Onmyoji Steam', 'Elden Ring', 'Fortnite', 'Võ Lâm Truyền Kỳ I', 'Honkai Impact 3', 'Silkroad Origin',
		'Mecha BREAK Steam', 'Age of Empires IV Steam', 'Võ Lâm Truyền Kỳ II',
	].map(n => ({ name: n.toLowerCase().replace(/\s+/g, '-'), display_name: n, category: ONL, status: 'ready' })),
	{ name: 'dota2', display_name: 'DotA 2', category: ONL, status: 'ready', launcher: 'steam.exe', launch_args: '-applaunch 570' },
	...['Steam', 'Garena', 'BlueStacks 5', 'LDPlayer', 'GPlay', 'OBS Portable', 'Nox', 'EGOPlay', 'Vanguard']
		.map(n => ({ name: n.toLowerCase().replace(/\s+/g, '-'), display_name: n, category: 'Công cụ', status: 'ready' })),
	{ name: 'gta-v', display_name: 'GTA V', category: 'Game offline', status: 'ready', label: 'hot' },
	{ name: 'tft-ten-rat-dai-de-thu-xem-hai-dong-co-bi-cat-hay-khong', display_name: 'Đấu Trường Chân Lý với một cái tên rất dài để thử xem hai dòng có bị cắt hay không', category: '', status: 'ready' },
]

let nguoiDangDangNhap = { id: 'dev', username: 'khach', full_name: 'Khách Thử', role: 'member' }

// Nạp QR giả. ?qr=off → quán chưa bật QR (chỉ còn nút trả tại quầy). Mã đang hiện thì
// __vnetQr('paid') / __vnetQr('expired') trong Console giả tiền về / hết hạn. Ảnh chỉ
// là một ô vuông có hoa văn giống mã QR — mã thật do máy chủ vẽ (vietqr.go).
let devQr: any = null
const phutTruoc = (m: number) => new Date(Date.now() - m * 60000).toISOString()
const devOrders: any[] = [
	{ id: 'o1', order_code: 'ORD-00042', status: 'pending', order_type: 'food', created_at: phutTruoc(2), final_amount: 30000,
		items: [{ id: 'i1', quantity: 1, product_name: 'Trà sữa trân châu', subtotal: 30000 }] },
	{ id: 'o2', order_code: 'ORD-00041', status: 'confirmed', order_type: 'food', created_at: phutTruoc(25), final_amount: 60000,
		items: [{ id: 'i2', quantity: 1, product_name: 'Cơm sườn trứng ốp la', subtotal: 55000 }, { id: 'i3', quantity: 1, product_name: 'Xúc xích Đức Việt nướng', subtotal: 5000 }] },
	{ id: 'o3', order_code: 'ORD-00037', status: 'completed', order_type: 'food', created_at: phutTruoc(90), final_amount: 15000,
		items: [{ id: 'i4', quantity: 1, product_name: 'Coca-Cola lon 330ml', subtotal: 15000 }] },
	{ id: 'o4', order_code: 'TOP-00012', status: 'completed', order_type: 'topup', created_at: phutTruoc(120), final_amount: 50000, items: [] },
]
function anhQrGia() {
	const n = 29, o: string[] = []
	let x = 7
	const ngauNhien = () => ((x = (x * 1103515245 + 12345) % 2147483648) % 100) < 45
	const goc = (cx: number, cy: number) =>
		o.push(`<path d="M${cx} ${cy}h7v7h-7zM${cx + 1} ${cy + 1}v5h5v-5z" fill-rule="evenodd"/><rect x="${cx + 2}" y="${cy + 2}" width="3" height="3"/>`)
	for (let r = 0; r < n; r++)
		for (let c = 0; c < n; c++) {
			const trongGoc = (r < 8 && c < 8) || (r < 8 && c > n - 9) || (r > n - 9 && c < 8)
			if (!trongGoc && ngauNhien()) o.push(`<rect x="${c}" y="${r}" width="1" height="1"/>`)
		}
	goc(0, 0); goc(n - 7, 0); goc(0, n - 7)
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${n} ${n}" shape-rendering="crispEdges">${o.join('')}</svg>`
	return 'data:image/svg+xml;utf8,' + encodeURIComponent(svg)
}

// Hàng chờ hộp thông báo của quầy (notice.go) — cùng luật với Go: trả mới nhất trước, và
// "Đã hiểu" chỉ gạt những cái có `at` tới mốc khách đã đọc. Xem hộp ở ?window=notice;
// bắn thêm một thông báo lúc hộp đang mở bằng __vnetNotice('nội dung') trong Console.
let devNotices = [
	{ title: 'Thông báo', body: 'Quầy sẽ bảo trì máy chủ lúc 03:00 sáng mai, khoảng 15 phút. Bạn nhớ lưu game trước nhé.', at: Date.now() - 120000 },
]

// Màn hình & chuột giả cho trang Cài đặt. Đủ độ phân giải có nhiều mức Hz (lọc Hz theo
// độ phân giải) và một độ phân giải chỉ có một mức. Sắp như Go trả: rộng, cao, Hz giảm dần.
const DEV_DISPLAY_MODES = [
	{ width: 2560, height: 1440, hz: 144 },
	{ width: 2560, height: 1440, hz: 60 },
	{ width: 1920, height: 1080, hz: 240 },
	{ width: 1920, height: 1080, hz: 144 },
	{ width: 1920, height: 1080, hz: 60 },
	{ width: 1280, height: 720, hz: 60 },
]
// Điểm danh giả: chuỗi 4 ngày, mốc thưởng mỗi 7 ngày. ?diemdanh=roi để xem lúc đã điểm
// danh hôm nay, ?diemdanh=dut cho chuỗi vừa đứt (bắt đầu lại từ ngày 1).
const devDiemDanh = (() => {
	const q = new URLSearchParams(location.search).get('diemdanh')
	if (q === 'roi') return { checked_in_today: true, streak_days: 5, next_reward: 2000, streak_every: 7, last_checkin: '2026-11-03' }
	if (q === 'dut') return { checked_in_today: false, streak_days: 0, next_reward: 2000, streak_every: 7, last_checkin: '2026-10-28' }
	return { checked_in_today: false, streak_days: 4, next_reward: 2000, streak_every: 7, last_checkin: '2026-11-02' }
})()

const DEV_DISPLAY_BASELINE = { width: 1920, height: 1080, hz: 60 }
const devDisplay = {
	current: { ...DEV_DISPLAY_BASELINE },
	// Chế độ trước khi áp dụng; khác null nghĩa là đang chờ Giữ/Hoàn tác. Lớp giả KHÔNG
	// tự hoàn tác sau 20 giây như Go — để thử được trường hợp đếm ngược của giao diện.
	previous: null as { width: number; height: number; hz: number } | null,
	mouseSpeed: 10,
	mousePrecision: true,
}

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

	window.__vnetQr = (kq: 'paid' | 'expired' = 'paid') => {
		if (!devQr) return 'chưa có mã QR nào đang hiện'
		devQr.status = kq
		const o = devOrders.find(x => x.id === devQr.order_id)
		if (o && kq === 'paid') o.status = 'completed'
		return window.__vnetEmit(`vnet:qr:${kq}`, { request_id: devQr.request_id, order_id: devQr.order_id, amount: devQr.amount })
	}

	window.__vnetNotice = (body: string, title = 'Thông báo') => {
		devNotices.push({ title, body, at: Date.now() })
		window.__vnetEmit('vnet:panel:mode', 'notice')
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
				GetServerURL: () => ok('http://localhost:20800'),
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
				GetAttendanceStatus: () => ok(JSON.stringify({ code: 0, data: devDiemDanh })),
				CheckinAttendance: () => {
					const thuong = devDiemDanh.next_reward
					devDiemDanh.checked_in_today = true
					devDiemDanh.streak_days++
					devDiemDanh.last_checkin = new Date().toISOString().slice(0, 10)
					return ok(JSON.stringify({ code: 0, data: { streak_days: devDiemDanh.streak_days, reward_amount: thuong } }))
				},
				SubmitFeedback: () => ok(JSON.stringify({ code: 0, data: { id: 'dev-feedback' } })),
				GetFeatureFlags: () => ok(JSON.stringify({ attendance_enabled: true, feedback_enabled: true })),
				GetTopupPresets: () => ok(JSON.stringify([10000, 20000, 50000, 100000])),
				GetQRStatus: () => ok(JSON.stringify({ ready: new URLSearchParams(location.search).get('qr') !== 'off' })),
				RequestTopupQR: (amount: number) => {
					devQr = {
						request_id: 'dev-qr-' + Date.now(), order_id: 'dev-order', order_code: 'TOP-DEV',
						code: 'VN7K2Q9M', amount, status: 'pending',
						expires_at: new Date(Date.now() + 10 * 60000).toISOString(),
						bank: 'MB', bank_name: 'MB Bank', account_no: '0123456789', account_name: 'NGUYEN VAN A',
						qr_payload: '', qr_image: anhQrGia(),
					}
					return ok(JSON.stringify(devQr))
				},
				// Đủ các trạng thái tab "Đơn đã gọi" phải vẽ: chờ duyệt, đã xác nhận chưa trả
				// (có nút Thanh toán bằng QR), hoàn thành, yêu cầu nạp tiền.
				GetMyOrders: () => ok(JSON.stringify(devOrders)),
				PayOrderQR: (id: string) => {
					const o = devOrders.find(x => x.id === id)
					if (!o || o.status !== 'confirmed') return Promise.reject('đơn không còn chờ thanh toán')
					devQr = {
						request_id: 'dev-qr-' + Date.now(), order_id: o.id, order_code: o.order_code,
						code: 'VN4HQ8TR', amount: o.final_amount, status: 'pending',
						expires_at: new Date(Date.now() + 10 * 60000).toISOString(),
						bank: 'MB', bank_name: 'MB Bank', account_no: '0123456789', account_name: 'NGUYEN VAN A',
						qr_payload: '', qr_image: anhQrGia(),
					}
					return ok(JSON.stringify(devQr))
				},
				GetQRRequest: (id: string) =>
					ok(JSON.stringify({ id, status: devQr?.request_id === id ? devQr.status : 'expired' })),
				CancelQRRequest: () => { if (devQr) devQr.status = 'cancelled'; return ok('') },
				GetNotifications: () => ok(JSON.stringify([])),
				GetHardware: () => ok(JSON.stringify({ machine_code: 'KT-01', server_url: 'http://localhost:20800' })),
				GetVersion: () => ok('dev'),
				GetProducts: () => ok(JSON.stringify([])),
				GetCategories: () => ok(JSON.stringify(DEV_CATEGORIES)),
				// Ảnh trỏ thẳng vào máy chủ đang chạy vì đó là điều thật sự cần
				// nhìn thấy: ngoài Wails không có gì tự đổi đường dẫn tương đối
				// thành tuyệt đối hộ (việc đó do GetMenu bên Go làm).
				GetMenu: (categoryId: string) =>
					ok(JSON.stringify(DEV_MENU.filter(p => !categoryId || p.category_id === categoryId))),
				RestoreSession: () => ok(JSON.stringify({ id: 'dev-member', username: 'khach', full_name: 'Khách thử', role: 'member' })),
				SetLoggedIn: () => ok(undefined),
				// Ngoài Wails không có màn khoá native nào đăng nhập hộ trang.
				CurrentLogin: () => ok(''),
				// Một ô cho cả hai loại, đúng như máy chủ làm: tên tài khoản
				// quyết định đường đi.
				//   quanly  → nhân viên   · khach → hội viên
				//   kythuat → tài khoản quản trị máy trạm (chế độ bảo trì)
				Login: (u: string, p: string) => {
					if (p !== '123456') return Promise.reject(new Error('sai tài khoản hoặc mật khẩu'))
					if (u === 'kythuat') return ok(JSON.stringify({ offline: true, local_admin: true }))
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
				// Nút "Đăng nhập" phải bấm được, nếu không thì lớp giả này chỉ
				// xem được đúng màn hình khoá — mọi thứ phía sau vẫn khuất.

				ListGames: () =>
					ok(JSON.stringify({
						root: 'D:\\Games',
						items: DEV_GAMES.map(g => ({
							launcher: 'Game.exe', launch_args: '', ...g, version: 1,
							progress: g.status === 'downloading' ? 20 + (Math.floor(Date.now() / 10000) % 80) : 0,
						})),
					})),
				RefreshGames: () => (window as any).go.main.App.ListGames(),
				MenuBranding: () => ok(JSON.stringify({ name: 'Net Mia Gaming', logo: '' })),
				// Valorant cố tình thất bại: nút Chơi của game "ready" là đường duy nhất
				// tới được thông báo lỗi.
				LaunchGame: (name: string) =>
					name === 'valorant'
						? Promise.reject('không tìm thấy tệp chạy Valorant trên ổ game, vui lòng báo nhân viên')
						: ok(undefined),

				OpenWindow: (mode: string) => {
					// Cửa sổ khởi động sẵn (prewarm) bên Go là cửa sổ ẩn: mở nó thành một tab
					// sau mỗi lần đăng nhập chỉ che mất trang đang xem.
					if (mode === 'prewarm') return ok(undefined)
					window.open(`${location.pathname}?window=${mode}`, '_blank')
					return ok(undefined)
				},
				ShowWindow: () => ok(undefined),
				GetNotices: () => ok(JSON.stringify([...devNotices].reverse())),
				DismissNotices: (upTo: number) => {
					devNotices = devNotices.filter(n => n.at > upTo)
					return ok(JSON.stringify([...devNotices].reverse()))
				},
				// Nút tắt của DockTitleBar. Ngoài Wails không có cửa sổ nào để ẩn xuống khay:
				// chỉ ghi log để thấy nút đã được nối dây.
				HideDock: () => { console.info('[dev-stub] HideDock'); return ok(undefined) },
				Logout: () => ok(''),
				GetBootTime: () => ok(1700000000),

				// ?display=unsupported → supported=false (phần bị ẩn) · ?display=error → lỗi
				GetDisplayInfo: () => {
					const mode = new URLSearchParams(location.search).get('display')
					if (mode === 'error') return Promise.reject('không đọc được thông tin màn hình')
					if (mode === 'unsupported') return ok(JSON.stringify({ supported: false }))
					return ok(JSON.stringify({
						supported: true,
						modes: DEV_DISPLAY_MODES,
						current: devDisplay.current,
						baseline: DEV_DISPLAY_BASELINE,
						mouse_speed: devDisplay.mouseSpeed,
						mouse_precision: devDisplay.mousePrecision,
					}))
				},
				ApplyDisplayMode: (width: number, height: number, hz: number) => {
					if (!DEV_DISPLAY_MODES.some(m => m.width === width && m.height === height && m.hz === hz))
						return Promise.reject(`màn hình không hỗ trợ chế độ ${width}×${height} ${hz} Hz`)
					devDisplay.previous ||= devDisplay.current
					devDisplay.current = { width, height, hz }
					return ok(undefined)
				},
				ConfirmDisplayMode: () => {
					if (!devDisplay.previous) return Promise.reject('không có thay đổi màn hình nào đang chờ xác nhận')
					devDisplay.previous = null
					return ok(undefined)
				},
				RevertDisplayMode: () => {
					if (devDisplay.previous) devDisplay.current = devDisplay.previous
					devDisplay.previous = null
					return ok(undefined)
				},
				SetMouseSpeed: (speed: number) => {
					devDisplay.mouseSpeed = Math.min(20, Math.max(1, speed))
					return ok(undefined)
				},
				SetMousePrecision: (on: boolean) => { devDisplay.mousePrecision = on; return ok(undefined) },
				ResetDisplayAndMouse: () => {
					devDisplay.current = { ...DEV_DISPLAY_BASELINE }
					devDisplay.previous = null
					devDisplay.mouseSpeed = 10
					devDisplay.mousePrecision = true
					return ok(undefined)
				},
			},
		},
	}
}
