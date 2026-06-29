<template>
	<div class="app-container" :style="{ height: '100vh', background: session.loggedIn ? '#f0f2f5' : '#1a1a2e' }">
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

			<ServiceMenu
				v-else-if="currentView === 'order'"
				@back="currentView = 'dashboard'"
			/>

			<ChatWidget
				v-else-if="currentView === 'chat'"
				fullPage
				@back="currentView = 'dashboard'"
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
		</template>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { useSessionStore } from './stores/session.store'
import { useChatStore } from './stores/chat.store'
import LockScreen from './views/LockScreen.vue'
import Dashboard from './views/Dashboard.vue'
import ServiceMenu from './views/ServiceMenu.vue'
import SettingsPage from './views/SettingsPage.vue'
import TopupDialog from './views/TopupDialog.vue'
import ChatWidget from './components/ChatWidget.vue'

declare const window: any
const api = () => window.go?.main?.App

const session = useSessionStore()
const chat = useChatStore()
const currentView = ref('dashboard')
const showTopup = ref(false)

function navigateTo(view: string) {
	if (view === 'topup') {
		showTopup.value = true
		return
	}
	currentView.value = view
}

async function handleLogin(username: string, password: string) {
	try {
		await session.login(username, password)
		currentView.value = 'dashboard'
	} catch (e) {
		ElMessage.error(String(e))
	}
}

async function handleLogout() {
	await session.logout()
	currentView.value = 'dashboard'
	showTopup.value = false
}

onMounted(async () => {
	const savedUrl = localStorage.getItem('vnet_server_url')
	if (savedUrl) {
		api().SetServerURL(savedUrl)
	}

	const savedCode = localStorage.getItem('vnet_machine_code')
	if (savedCode) {
		api().SetMachineCode(savedCode)
	}

	await session.restore()

	EventsOn('vnet:session:updated', (data: string) => {
		try { session.updateSession(JSON.parse(data)) } catch {}
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
		try {
			const msg = JSON.parse(data)
			if (currentView.value !== 'chat') {
				ElMessage.info('Có tin nhắn mới trong hỗ trợ')
			}
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
}
</style>
