<template>
	<!--
		Cửa sổ phụ (thực đơn, hỗ trợ) chạy trong TIẾN TRÌNH RIÊNG vì Wails v2 không
		tạo được cửa sổ thứ hai. Ở đó không dựng thanh điều khiển, chỉ render đúng
		một màn hình.
	-->
	<div v-if="windowMode" class="app-container child-window">
		<ServiceMenu v-if="windowMode === 'order'" standalone />
		<ChatWidget v-else fullPage standalone />
	</div>

	<div v-else class="app-container">
		<MachineLockOverlay />

		<LockScreen
			v-if="!session.loggedIn"
			@login="handleLogin"
			:loading="session.loginLoading"
		/>

		<template v-else>
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

			<TopupDialog
				v-if="showTopup"
				@close="showTopup = false"
			/>

			<AttendanceDialog
				v-if="showAttendance"
				@close="showAttendance = false"
			/>

			<FeedbackDialog
				v-if="showFeedback"
				@close="showFeedback = false"
			/>
		</template>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { useSessionStore } from './stores/session.store'
import { useChatStore } from './stores/chat.store'
import LockScreen from './views/LockScreen.vue'
import Dashboard from './views/Dashboard.vue'
import ServiceMenu from './views/ServiceMenu.vue'
import SettingsPage from './views/SettingsPage.vue'
import TopupDialog from './views/TopupDialog.vue'
import AttendanceDialog from './views/AttendanceDialog.vue'
import FeedbackDialog from './views/FeedbackDialog.vue'
import ChatWidget from './components/ChatWidget.vue'
import MachineLockOverlay from './components/MachineLockOverlay.vue'

declare const window: any
const api = () => window.go?.main?.App

const session = useSessionStore()
const chat = useChatStore()
const currentView = ref('dashboard')
const windowMode = ref('')
const showTopup = ref(false)
const showAttendance = ref(false)
const showFeedback = ref(false)

// Ba mục này là hộp thoại chồng lên màn hình chính chứ không phải trang riêng,
// nên không đổi currentView.
const OVERLAYS: Record<string, (v: boolean) => void> = {
	topup: v => (showTopup.value = v),
	attendance: v => (showAttendance.value = v),
	feedback: v => (showFeedback.value = v),
}

// Hai màn hình này mở thành CỬA SỔ HỆ ĐIỀU HÀNH RIÊNG: thực đơn cần rộng hơn
// thanh 360px, và cửa sổ hỗ trợ phải nổi lên trên được khi khách đang chơi game
// toàn màn hình. Wails v2 không tạo được cửa sổ thứ hai nên đây là một tiến
// trình .exe nữa; bấm lần hai chỉ đánh thức cửa sổ đang chạy.
const EXTERNAL_WINDOWS: Record<string, string> = { order: 'order', chat: 'support' }

function navigateTo(view: string) {
	const overlay = OVERLAYS[view]
	if (overlay) {
		overlay(true)
		return
	}
	const external = EXTERNAL_WINDOWS[view]
	if (external) {
		api()?.OpenWindow(external)?.catch?.((e: any) => ElMessage.error(String(e)))
		return
	}
	currentView.value = view
}

async function handleLogin(username: string, password: string) {
	try {
		const data = await session.login(username, password)
		currentView.value = 'dashboard'
		if (data?.kind === 'staff') await canhBaoTaiKhoanMacDinh()
	} catch (e) {
		// Bỏ tiền tố "Error:" mà JS gắn vào: thông báo từ máy chủ đã là câu
		// tiếng Việt hoàn chỉnh, dán thêm chữ đó vào chỉ làm khách tưởng máy hỏng.
		ElMessage.error(String(e).replace(/^Error:\s*/, ''))
	}
}

// Cảnh báo tài khoản mặc định, chỉ hiện khi nó CÒN mở được.
//
// Đặt ở lúc nhân viên đăng nhập chứ không phải lúc khởi động: khách không làm
// gì được với thông tin này, còn nhân viên thì làm được — họ chỉ cần đăng nhập
// một lần khi có mạng là cửa đó đóng lại.
//
// duration 0 = không tự tắt. Một cảnh báo bảo mật trôi qua sau ba giây thì
// chẳng khác gì không có.
async function canhBaoTaiKhoanMacDinh() {
	try {
		if (!(await api().HasBuiltinAdmin())) return
	} catch {
		return
	}
	ElNotification({
		title: 'Máy này còn mở được bằng tài khoản mặc định',
		message:
			'admin / admin vẫn dùng được để mở khoá máy khi mất mạng, vì máy chưa từng ' +
			'có nhân viên nào đăng nhập qua máy chủ. Mọi máy cài từ cùng bản này đều ' +
			'giống nhau. Đăng nhập một lần khi có mạng là cửa đó tự đóng.',
		type: 'warning',
		duration: 0,
		position: 'bottom-right',
	})
}

async function handleLogout() {
	await session.logout()
	currentView.value = 'dashboard'
	showTopup.value = false
	showAttendance.value = false
	showFeedback.value = false
}

onMounted(async () => {
	// Biết mình là cửa sổ nào TRƯỚC khi dựng gì: cửa sổ phụ không có thanh điều
	// khiển, không có màn hình khoá.
	try { windowMode.value = (await api()?.GetWindowMode()) || '' } catch { /* ngoài Wails */ }

	const savedUrl = localStorage.getItem('vnet_server_url')
	if (savedUrl) {
		api().SetServerURL(savedUrl)
	}

	await session.restore()

	EventsOn('vnet:session:updated', (data: string) => {
		try { session.updateSession(JSON.parse(data)) } catch {}
	})

	EventsOn('vnet:session:ended', (data: string) => {
		try { session.clearSession(JSON.parse(data)) } catch {}
	})

	EventsOn('vnet:balance:updated', (data: any) => {
		session.updateBalance(data)
	})

	EventsOn('vnet:notification:new', () => {
		session.loadData()
	})

	EventsOn('vnet:topup:confirmed', () => {
		session.loadData()
		ElMessage.success('Nạp tiền thành công!')
	})

	chat.initWsHandlers(session.userID)

	EventsOn('vnet:chat:message', (data: string) => {
		// Cửa sổ hỗ trợ tự lo tin nhắn của nó; thanh điều khiển chỉ có việc kéo
		// cửa sổ đó lên trước mặt khách.
		if (windowMode.value) return
		try {
			const msg = JSON.parse(data)
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
	font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
	overflow: hidden;
}

.app-container {
	width: 100vw;
	height: 100vh;
	background: var(--vnet-bg);
	color: var(--vnet-text);
}

/* Cửa sổ phụ là cửa sổ thường, có viền hệ điều hành — không dán mép nên cứ để
   nội dung chiếm hết. */
.child-window {
	overflow: hidden;
}
</style>
