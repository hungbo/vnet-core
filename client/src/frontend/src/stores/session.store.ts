import { defineStore } from 'pinia'
import { ref } from 'vue'

declare const window: any
const api = () => window.go?.main?.App

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

	async function login(username: string, password: string) {
		loginLoading.value = true
		try {
			const result = await api().Login(username, password)
			const data = JSON.parse(result)

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
				api().SetLoggedIn(true)
				localStorage.setItem('vnet_session', JSON.stringify({
					token: data.access_token,
					userId: data.user?.id,
					role: role.value,
					displayName: data.user?.full_name || data.user?.username || ''
				}))
			}
			return data
		} finally {
			loginLoading.value = false
		}
	}

	async function logout() {
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
		if (!isForCurrentMember(data)) return
		// GỘP chứ không gán đè. Payload session:started chỉ có 6 khoá, nên gán đè
		// là xoá sạch affordable_until, price_per_hour, combo_type... và đồng hồ
		// đếm ngược tụt về nhánh "Đã chơi" ngay khi có một sự kiện WebSocket.
		session.value = { ...(session.value || {}), ...data }
	}

	// Phiên bị máy chủ tự đóng (hết tiền, hết khung giờ, giới nghiêm). Không xử
	// lý thì đồng hồ trên máy vẫn chạy tiếp như chưa có gì xảy ra.
	async function clearSession(data: any) {
		if (!isForCurrentMember(data)) return
		session.value = null
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
		// lượt tính kế tiếp (có thể gần một phút sau). Tính lại tại chỗ bằng đơn
		// giá đã có sẵn trong phiên.
		const price = Number(session.value?.price_per_hour || 0)
		if (session.value && price > 0 && memberInfo.value) {
			const money = Number(memberInfo.value.balance || 0) + Number(memberInfo.value.bonus_balance || 0)
			if (money > 0) {
				const minutes = Math.floor((money * 60) / price)
				session.value.affordable_until = new Date(Date.now() + minutes * 60000).toISOString()
			}
		}
	}

	async function restore() {
		const saved = localStorage.getItem('vnet_session')
		if (!saved) return false
		try {
			const s = JSON.parse(saved)
			if (!s.token) return false
			await api().RestoreSession(s.token, s.userId || '', '', s.displayName || '', s.role || 'member')
			token.value = s.token
			userID.value = s.userId || ''
			role.value = s.role || 'member'
			displayName.value = s.displayName || ''
			loggedIn.value = true
			api().SetLoggedIn(true)
			return true
		} catch {
			localStorage.removeItem('vnet_session')
			return false
		}
	}

	return { loggedIn, token, userID, machineCode, role, displayName, machineStatus, session, memberInfo, loginLoading, login, logout, loadData, updateSession, clearSession, updateBalance, restore }
})
