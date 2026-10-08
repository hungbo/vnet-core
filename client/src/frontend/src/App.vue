<template>
	<!--
		Cửa sổ phụ (thực đơn, hỗ trợ) chạy trong TIẾN TRÌNH RIÊNG vì Wails v2 không
		tạo được cửa sổ thứ hai. Ở đó không dựng thanh điều khiển, chỉ render đúng
		một màn hình.
	-->
	<div v-if="windowMode" class="app-container child-window">
		<!--
			Chờ khôi phục đăng nhập xong mới dựng: component con chạy onMounted
			TRƯỚC onMounted của App, nên dựng sớm là cửa sổ Nạp tiền hỏi số dư khi
			chưa có token — và hiện ra không có dòng "Số dư hiện tại".
		-->
		<!--
			Một cửa sổ cho mọi màn hình phụ, dựng sẵn ẩn từ lúc đăng nhập. KeepAlive
			giữ nguyên từng màn hình khi đổi qua lại: giỏ hàng đang chọn dở không
			mất khi khách mở Hỗ trợ rồi quay lại.
		-->
		<template v-if="!childReady || windowMode === 'blank'" />
		<!--
			Chỉ giữ nguyên Đồ ăn (giỏ hàng chọn dở) và Hỗ trợ (cuộc trò chuyện).
			Nạp tiền, Điểm danh, Đánh giá là biểu mẫu một lần: dựng MỚI mỗi lần
			mở (khoá theo lanMo), nếu không màn hình Nạp tiền kẹt ở "Đã gửi yêu
			cầu" và khách không gửi được yêu cầu thứ hai. Game cũng dựng mới để
			mỗi lần mở là một danh sách vừa tải.
		-->
		<KeepAlive v-else :include="['ServiceMenu', 'ChatWidget']">
			<ServiceMenu v-if="windowMode === 'order'" key="order" standalone />
			<TopupDialog v-else-if="windowMode === 'topup'" :key="'topup' + lanMo" standalone />
			<AttendanceDialog v-else-if="windowMode === 'attendance'" :key="'attendance' + lanMo" standalone />
			<FeedbackDialog v-else-if="windowMode === 'feedback'" :key="'feedback' + lanMo" standalone />
			<GameMenu v-else-if="windowMode === 'games'" :key="'games' + lanMo" />
			<ChatWidget v-else key="support" fullPage standalone />
		</KeepAlive>
	</div>

	<div v-else class="app-container dock-window">
		<MachineLockOverlay />

		<LockScreen
			v-if="!session.loggedIn"
			@login="handleLogin"
			:loading="session.loginLoading"
			:reason="session.dangKhoiPhuc ? 'Đang khôi phục phiên…' : session.lockReason"
		/>

		<template v-else>
			<!--
				Chỉ khi đã đăng nhập: màn hình đăng nhập phủ kín màn hình và không được
				thu nhỏ, nên không có thanh này ở đó.
			-->
			<DockTitleBar />

			<Dashboard
				v-if="currentView === 'dashboard'"
				@navigate="navigateTo"
				@logout="handleLogout"
			/>

			<SettingsPage
				v-else-if="currentView === 'settings'"
				@back="currentView = 'dashboard'"
				@logout="handleLogout"
			/>
		</template>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { phatAm } from './utils/sound'
import { useSessionStore } from './stores/session.store'
import { useChatStore } from './stores/chat.store'
import LockScreen from './views/LockScreen.vue'
import Dashboard from './views/Dashboard.vue'
import ServiceMenu from './views/ServiceMenu.vue'
import SettingsPage from './views/SettingsPage.vue'
import TopupDialog from './views/TopupDialog.vue'
import AttendanceDialog from './views/AttendanceDialog.vue'
import FeedbackDialog from './views/FeedbackDialog.vue'
import GameMenu from './views/GameMenu.vue'
import ChatWidget from './components/ChatWidget.vue'
import MachineLockOverlay from './components/MachineLockOverlay.vue'
import DockTitleBar from './components/DockTitleBar.vue'

declare const window: any
const api = () => window.go?.main?.App

const session = useSessionStore()
const chat = useChatStore()
const currentView = ref('dashboard')
const windowMode = ref('')
// Đếm số lần cửa sổ phụ được mở, để dựng mới các biểu mẫu một lần (xem template).
const lanMo = ref(0)
const childReady = ref(false)

// Hai màn hình này mở thành CỬA SỔ HỆ ĐIỀU HÀNH RIÊNG: thực đơn cần rộng hơn
// thanh 360px, và cửa sổ hỗ trợ phải nổi lên trên được khi khách đang chơi game
// toàn màn hình. Wails v2 không tạo được cửa sổ thứ hai nên đây là một tiến
// trình .exe nữa; bấm lần hai chỉ đánh thức cửa sổ đang chạy.
const EXTERNAL_WINDOWS: Record<string, string> = {
	order: 'order', chat: 'support', topup: 'topup', attendance: 'attendance', feedback: 'feedback', games: 'games',
}

function navigateTo(view: string) {
	const external = EXTERNAL_WINDOWS[view]
	if (external) {
		api()?.OpenWindow(external)?.catch?.((e: any) => ElMessage.error(String(e)))
		return
	}
	currentView.value = view
}

async function handleLogin(username: string, password: string) {
	try {
		await session.login(username, password)
		currentView.value = 'dashboard'
	} catch (e) {
		// Bỏ tiền tố "Error:" mà JS gắn vào: thông báo từ máy chủ đã là câu
		// tiếng Việt hoàn chỉnh, dán thêm chữ đó vào chỉ làm khách tưởng máy hỏng.
		ElMessage.error(String(e).replace(/^Error:\s*/, ''))
	}
}

async function handleLogout() {
	await session.logout()
	currentView.value = 'dashboard'
}

onMounted(async () => {
	// Biết mình là cửa sổ nào TRƯỚC khi dựng gì: cửa sổ phụ không có thanh điều
	// khiển, không có màn hình khoá.
	try { windowMode.value = (await api()?.GetWindowMode()) || '' } catch { /* ngoài Wails */ }

	// Địa chỉ máy chủ chỉ có MỘT nguồn: config.json cạnh vnet-client.exe, nơi
	// dịch vụ nền cũng đọc. Bản cũ để một bản lưu trong trình duyệt ghi đè nó,
	// và bản lưu đó sống sót qua cả gỡ-cài-lại: giao diện nói chuyện với một
	// máy chủ cũ trong khi dịch vụ nền nói chuyện với máy chủ mới.
	localStorage.removeItem('vnet_server_url')

	// Khôi phục đăng nhập chạy NỀN, không chờ: máy chủ treo thì màn hình khoá và ô
	// đăng nhập vẫn phải hiện ngay — cho khách, và cho kỹ thuật viên cần vào máy
	// lúc mất mạng. session.restore tự có hạn chót và về màn hình khoá khi hỏng.
	// Chỉ cửa sổ phụ phải chờ nó xong mới dựng (childReady). Các handler bên dưới
	// đăng ký NGAY: để sau lời chờ này thì máy chủ treo là mất luôn sự kiện hết
	// phiên, nối lại mạng và chốt kiểm 20 giây.
	session.restore().finally(() => {
		childReady.value = true
		// Cần userID của phiên vừa khôi phục.
		chat.initWsHandlers(session.userID)
	})

	// Cửa sổ phụ dựng sẵn: thanh chính bấm nút nào thì đổi sang màn hình đó.
	EventsOn('vnet:panel:mode', (m: string) => { windowMode.value = m; lanMo.value++ })

	// Chốt chặn cho sự kiện WebSocket bị rơi: hội viên đăng nhập mà máy chủ
	// không còn phiên nào thì về màn hình khoá.
	if (!windowMode.value) setInterval(() => session.verifySession(), 20000)

	EventsOn('vnet:session:updated', (data: string) => {
		try { session.updateSession(JSON.parse(data)) } catch {}
	})

	EventsOn('vnet:session:ended', (data: string) => {
		let d: any = {}
		try { d = JSON.parse(data) } catch {}
		// Cửa sổ phụ không có màn hình khoá: phiên hết thì nó tự đóng.
		if (windowMode.value) {
			api()?.Logout()
			return
		}
		session.clearSession(d)
		currentView.value = 'dashboard'
	})

	// Vừa nối lại máy chủ sau khi bị khoá vì mất kết nối: hỏi ngay phiên còn
	// không, đừng chờ lượt kiểm 20 giây.
	EventsOn('vnet:offline:cleared', () => {
		session.verifySession()
	})

	EventsOn('vnet:balance:updated', (data: any) => {
		session.updateBalance(data)
	})

	EventsOn('vnet:notification:new', () => {
		session.loadData()
		if (!windowMode.value) phatAm('thong-bao')
	})

	EventsOn('vnet:topup:confirmed', () => {
		session.loadData()
		// Cửa sổ Nạp tiền tự báo kết quả đổi thẻ kèm số tiền; báo thêm dòng này
		// là hai thông báo chồng nhau cho cùng một lần nạp.
		if (windowMode.value === 'topup') return
		ElMessage.success('Nạp tiền thành công!')
	})

	EventsOn('vnet:chat:message', (data: string) => {
		// Cửa sổ hỗ trợ tự lo tin nhắn của nó; thanh điều khiển chỉ có việc kéo
		// cửa sổ đó lên trước mặt khách.
		if (windowMode.value) return
		try {
			const msg = JSON.parse(data)
			// Tin do chính khách gửi cũng vọng về đây: không bật cửa sổ, không
			// thông báo, không kêu chuông cho câu khách vừa tự gõ.
			if (msg?.sender_type === 'member') return
			phatAm('tin-nhan')
			// Nhân viên nhắn lúc khách đang chơi game toàn màn hình thì một dòng
			// toast sau lưng game là vô hình. Mở/đánh thức cửa sổ hỗ trợ mới là
			// thứ khách thấy được.
			api()?.OpenWindow('support')
			const who = msg?.sender_username ? `${msg.sender_type} - ${msg.sender_username}` : 'Hỗ trợ'
			ElMessage.info(`${who}: ${String(msg?.message ?? '').slice(0, 60)}`)
		} catch {}
	})
})
</script>

<style>
* {
	margin: 0;
	padding: 0;
	box-sizing: border-box;
}

body {
	font-family: var(--vnet-font);
	overflow: hidden;
}

.app-container {
	width: 100vw;
	height: 100vh;
	background: var(--vnet-bg);
	color: var(--vnet-text);
}

/* Thanh dọc không có viền hệ điều hành: thanh tiêu đề tự vẽ ở trên, phần còn lại chia
   cho màn hình đang mở (Dashboard, SettingsPage lấp đầy bằng flex: 1). */
.dock-window {
	display: flex;
	flex-direction: column;
}

/* Cửa sổ phụ là cửa sổ thường, có viền hệ điều hành — không dán mép nên cứ để
   nội dung chiếm hết. */
.child-window {
	overflow: hidden;
}
</style>
