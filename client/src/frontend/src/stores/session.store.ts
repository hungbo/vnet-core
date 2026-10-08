import { defineStore } from 'pinia'
import { ref } from 'vue'

declare const window: any
const api = () => window.go?.main?.App

const LY_DO: Record<string, string> = {
	out_of_balance: 'Hết số dư — vui lòng nạp thêm để chơi tiếp',
	slot_ended: 'Đã hết khung giờ của gói',
	minutes_exhausted: 'Đã hết số phút của gói',
	curfew_window: 'Đã tới giờ giới nghiêm',
	max_minor_hours: 'Đã chơi đủ số giờ cho phép trong ngày',
}

function lyDoKetThuc(reason?: string): string {
	return (reason && LY_DO[reason]) || 'Phiên chơi đã kết thúc'
}

// Hạn chót cho mỗi lời gọi Go lúc khôi phục phiên. Bên Go đã tự chặn ở 15 giây
// (uiTimeout, http.go); đây là chốt thứ hai và cố ý DÀI HƠN, để lỗi của Go luôn
// tới trước còn giao diện không bao giờ đứng chờ một lời gọi không trả về.
const HAN_KHOI_PHUC_MS = 20000

function voiHan<T>(p: Promise<T>, ms: number): Promise<T> {
	let timer: ReturnType<typeof setTimeout>
	const het = new Promise<never>((_, reject) => {
		timer = setTimeout(() => reject(new Error('hết thời gian chờ máy chủ')), ms)
	})
	return Promise.race([p, het]).finally(() => clearTimeout(timer))
}

export const useSessionStore = defineStore('session', () => {
	const loggedIn = ref(false)
	const token = ref('')
	const userID = ref('')
	const machineCode = ref('')
	const role = ref('member')
	const displayName = ref('')
	const machineStatus = ref('available')
	const session = ref<any>(null)
	const memberInfo = ref<any>(null)
	const loginLoading = ref(false)
	// restore() chạy nền: màn hình khoá hiện ngay và báo "đang khôi phục" trong lúc
	// chờ máy chủ xác nhận phiên cũ.
	const dangKhoiPhuc = ref(false)
	// Tăng mỗi lần người ở máy đăng nhập xong bằng tay. restore() có thể xong SAU
	// lần đăng nhập đó; số này cho nó biết mình đã bị vượt mặt và phải lui.
	let luotDangNhap = 0
	// Lý do máy vừa về màn hình khoá (hết tiền, quầy trả máy...). Màn hình khoá
	// hiện dòng này để khách biết vì sao, thay vì một lớp phủ không có ô đăng
	// nhập nào.
	const lockReason = ref('')
	let xoaLyDo: ReturnType<typeof setTimeout> | undefined
	// Lý do chỉ dành cho người vừa bị đưa về màn hình khoá. Để mãi thì khách
	// kế tiếp ngồi vào cũng đọc "Hết số dư" của người trước.
	function datLyDo(reason: string) {
		lockReason.value = reason
		if (xoaLyDo) clearTimeout(xoaLyDo)
		if (reason) xoaLyDo = setTimeout(() => { lockReason.value = '' }, 120000)
	}

	async function login(username: string, password: string) {
		loginLoading.value = true
		try {
			const result = await api().Login(username, password)
			const data = JSON.parse(result)
			luotDangNhap++

			// Mất mạng nhưng đúng tài khoản nhân viên đã lưu: máy đã được mở
			// khoá bên Go rồi. KHÔNG dựng phiên ở đây — không có phiên nào cả.
			if (data?.offline) return data

			if (data?.access_token) {
				token.value = data.access_token
				userID.value = data.user?.id || ''
				// Nhân viên (bảng users) mang vai trò "owner"/"manager"/"staff",
				// nhưng giao diện máy trạm chỉ phân biệt ba loại màn hình. Quy
				// hết về 'admin' — nếu không, chủ quán đăng nhập lại thấy màn
				// hình của khách, kèm nút Nạp tiền và Điểm danh.
				role.value = data.kind === 'staff' ? 'admin' : (data.user?.role || 'member')
				displayName.value = data.user?.full_name || data.user?.username || ''
				loggedIn.value = true
				// Đổi hình dạng cửa sổ NGAY tại đây, không để component tự nhớ:
				// có ba đường dẫn tới trạng thái đăng nhập (đăng nhập, đăng
				// xuất, khôi phục phiên) và bỏ sót một đường là máy kẹt ở màn
				// hình khoá hoặc ngược lại, mở toang cho ai cũng dùng được.
				datLyDo('')
				api().SetLoggedIn(true)
				let bootTime = 0
				try { bootTime = await api().GetBootTime() } catch { /* bản Go cũ */ }
				localStorage.setItem('vnet_session', JSON.stringify({
					token: data.access_token,
					userId: data.user?.id,
					role: role.value,
					displayName: data.user?.full_name || data.user?.username || '',
					bootTime,
				}))
				dungSanCuaSoPhu()
			}
			return data
		} finally {
			loginLoading.value = false
		}
	}

	async function logout(reason = '') {
		datLyDo(reason)
		localStorage.removeItem('vnet_session')
		api().SetLoggedIn(false)
		loggedIn.value = false
		token.value = ''
		userID.value = ''
		machineCode.value = ''
		role.value = 'member'
		displayName.value = ''
		session.value = null
		memberInfo.value = null
		try { await api().Logout() } catch { /* ignore */ }
	}

	async function loadData() {
		machineCode.value = await api().GetMachineCode()
		const userInfoStr = await api().GetUserInfo()
		if (userInfoStr) {
			const info = JSON.parse(userInfoStr)
			role.value = info.role || 'member'
			displayName.value = info.full_name || info.username || ''
		}
		if (role.value !== 'admin') {
			const memberStr = await api().GetMemberInfo()
			if (memberStr) memberInfo.value = JSON.parse(memberStr)
			const sessionStr = await api().GetSession()
			if (sessionStr && sessionStr !== 'null') session.value = JSON.parse(sessionStr)
		}
	}

	// Máy chủ phát các sự kiện này cho mọi máy đang kết nối, nên nếu không lọc
	// thì hội viên ở máy khác mở phiên sẽ ghi đè bộ đếm giờ của máy này, và
	// người khác nạp tiền sẽ làm nhảy số dư hiển thị ở đây.
	function isForCurrentMember(data: any): boolean {
		const target = data?.member_id
		if (!target) return true
		return !userID.value || target === userID.value
	}

	async function updateSession(data: any) {
		// Máy đang ở màn hình khoá thì không có phiên nào của ai để cập nhật:
		// quầy mở máy trước cho khách là chuyện thường, và gọi máy chủ lúc này
		// là gọi không có token.
		if (!loggedIn.value) return
		if (!isForCurrentMember(data)) return
		// GỘP chứ không gán đè. Payload session:started chỉ có 6 khoá, nên gán đè
		// là xoá sạch affordable_until, price_per_hour, combo_type... và đồng hồ
		// đếm ngược tụt về nhánh "Đã chơi" ngay khi có một sự kiện WebSocket.
		session.value = { ...(session.value || {}), ...data }
		// Phiên do quầy mở thì sự kiện không mang mốc hết tiền: đọc lại phiên
		// đầy đủ, nếu không đồng hồ đếm LÊN thay vì đếm ngược tới lúc hết tiền.
		if (!data?.affordable_until && role.value !== 'admin') {
			try {
				const s = await api().GetSession()
				if (s && s !== 'null') session.value = JSON.parse(s)
			} catch { /* mất mạng: giữ nguyên */ }
		}
	}

	// Phiên kết thúc vì bất kỳ lý do gì: hết tiền, hết gói, giới nghiêm, quầy
	// trả máy. Máy phải về màn hình khoá có ô đăng nhập. Bản cũ chỉ xoá đồng hồ
	// và để khách ngồi trên desktop mở với tài khoản vẫn đăng nhập — chơi tiếp
	// không tính tiền.
	async function clearSession(data: any) {
		if (!isForCurrentMember(data)) return
		const reason = lyDoKetThuc(data?.reason)
		if (!loggedIn.value) {
			// Hai sự kiện cho cùng một lần kết thúc (session:ended rồi
			// curfew:enforced): giữ lý do cụ thể hơn.
			if (data?.reason) datLyDo(reason)
			return
		}
		if (role.value === 'admin') return
		await logout(reason)
	}

	// Kiểm lại với máy chủ: tài khoản hội viên đang đăng nhập mà không còn phiên
	// nào thì về màn hình khoá. Chốt chặn cho mọi sự kiện WebSocket bị rơi.
	async function verifySession() {
		if (!loggedIn.value || role.value === 'admin') return
		let s: string
		try {
			s = await api().GetSession()
		} catch {
			return // không với tới máy chủ: không phán gì
		}
		if (!s || s === 'null') await logout('Phiên chơi đã kết thúc')
	}

	async function updateBalance(data: any) {
		if (!isForCurrentMember(data)) return
		if (memberInfo.value) {
			if (data.balance !== undefined) memberInfo.value.balance = data.balance
			if (data.bonus_balance !== undefined) memberInfo.value.bonus_balance = data.bonus_balance
		}
		// Mốc hết tiền: dùng số máy chủ gửi kèm nếu có, vì nó đã tính cả thời
		// lượng tối thiểu và gói khung giờ — hai thứ mà ước lượng dưới đây không
		// biết. Lượt trừ tiền mỗi phút gửi kèm mốc này; sự kiện nạp tiền thì không.
		if (session.value && data.affordable_until) {
			session.value.affordable_until = data.affordable_until
			return
		}

		// Nạp tiền xong khách phải thấy đồng hồ dài ra NGAY, không phải chờ tới
		// lượt tính kế tiếp. Hỏi lại máy chủ chứ không tự ước lượng: ước lượng ở
		// đây không biết phần tiền tối thiểu đã trả trước và không neo theo mốc
		// phút của phiên, nên đồng hồ hụt đi rồi nhảy ngược lên.
		if (session.value && role.value !== 'admin') {
			try {
				const s = await api().GetSession()
				if (s && s !== 'null') {
					const moi = JSON.parse(s)
					if (moi?.affordable_until) session.value.affordable_until = moi.affordable_until
				}
			} catch { /* mất mạng: giữ nguyên */ }
		}
	}

	// Dựng sẵn cửa sổ phụ (ẩn) ngay sau khi token đã lưu, để bấm Đồ ăn, Hỗ trợ...
	// là hiện ngay thay vì chờ chạy lại .exe và dựng WebView2. Cửa sổ phụ tự gọi
	// hàm này khi khôi phục phiên thì Go bỏ qua.
	function dungSanCuaSoPhu() {
		try { api()?.OpenWindow('prewarm')?.catch?.(() => {}) } catch { /* ngoài Wails */ }
	}

	async function restore() {
		const saved = localStorage.getItem('vnet_session')
		if (!saved) return false
		const luot = luotDangNhap
		try {
			const s = JSON.parse(saved)
			if (!s.token) return false
			// Máy đã khởi động lại kể từ lúc đăng nhập: máy chủ chốt phiên cũ tại
			// lúc reboot, nên khôi phục đăng nhập là mở desktop cho một tài khoản
			// không còn phiên — chơi không tính tiền. Chỉ khôi phục khi giao diện
			// tự khởi động lại mà máy vẫn chạy (bị tắt nhầm, bị treo).
			let bootNow = 0
			try { bootNow = await api().GetBootTime() } catch { /* bản Go cũ */ }
			if (!s.bootTime || !bootNow || Math.abs(bootNow - s.bootTime) > 5) {
				localStorage.removeItem('vnet_session')
				return false
			}
			// Chỉ đưa token; AI và VAI TRÒ GÌ do máy chủ trả lời, không lấy từ bản
			// lưu ở đây. Máy chủ không xác nhận (kể cả không với tới) thì hàm này
			// ném lỗi và máy ở lại màn hình khoá.
			dangKhoiPhuc.value = true
			const me = JSON.parse(await voiHan(api().RestoreSession(s.token), HAN_KHOI_PHUC_MS))
			dangKhoiPhuc.value = false
			// Người ở máy đã tự đăng nhập trong lúc chờ (bên Go cũng đang giữ phiên của
			// họ): phiên của họ thắng, không ghi đè.
			if (luot !== luotDangNhap) return false
			token.value = s.token
			userID.value = me.id || ''
			role.value = me.role || 'member'
			displayName.value = me.full_name || me.username || ''
			loggedIn.value = true
			if (role.value !== 'admin') {
				let cur = ''
				try { cur = await voiHan(api().GetSession(), HAN_KHOI_PHUC_MS) } catch { cur = 'unknown' }
				if (!cur || cur === 'null') {
					await logout('Phiên chơi đã kết thúc')
					return false
				}
			}
			api().SetLoggedIn(true)
			dungSanCuaSoPhu()
			return true
		} catch {
			// Hết hạn chót hay bị từ chối đều như nhau: về màn hình khoá và bỏ token
			// cũ để khách đăng nhập lại — KHÔNG giữ token của người trước. Trừ khi đã
			// có người đăng nhập tay: bản lưu lúc này là của họ.
			if (luot === luotDangNhap) localStorage.removeItem('vnet_session')
			return false
		} finally {
			dangKhoiPhuc.value = false
		}
	}

	return { loggedIn, token, userID, machineCode, role, displayName, machineStatus, session, memberInfo, loginLoading, dangKhoiPhuc, lockReason, login, logout, loadData, updateSession, clearSession, verifySession, updateBalance, restore }
})
