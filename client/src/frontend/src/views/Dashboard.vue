<template>
	<div class="dashboard">
		<header class="dash-header">
			<div class="header-left">
				<span class="machine-tag" :class="session.machineStatus">{{ session.machineCode }}</span>
				<span class="user-name">{{ session.displayName }}</span>
				<el-tag v-if="session.role === 'admin'" type="warning" size="small" effect="dark">Quản trị</el-tag>
				<el-tag v-else-if="session.role === 'combo'" type="info" size="small" effect="dark">Combo</el-tag>
			</div>
			<div class="header-right">
				<el-badge :value="unreadCount" :hidden="unreadCount === 0" class="notif-badge">
					<el-button text circle @click="toggleNotif">
						<el-icon><Bell /></el-icon>
					</el-button>
				</el-badge>
				<el-button text circle @click="$emit('navigate', 'settings')">
					<el-icon><Setting /></el-icon>
				</el-button>
			</div>
		</header>

		<main class="dash-main">
			<TimerWidget :role="session.role" :session="session.session" :now="now" />
			<BalanceDisplay :role="session.role" :memberInfo="session.memberInfo" />

			<NotificationList ref="notifRef" :visible="showNotif" @update:unreadCount="unreadCount = $event" />

			<div class="actions-grid">
				<button
					v-if="session.role === 'member'"
					class="action-btn topup"
					@click="$emit('navigate', 'topup')"
				>
					<el-icon :size="28"><Coin /></el-icon>
					<span>Nạp tiền</span>
				</button>

				<button
					v-if="session.role === 'admin'"
					class="action-btn playtime"
					@click="handlePlaytime"
				>
					<el-icon :size="28"><Timer /></el-icon>
					<span>Giờ chơi</span>
				</button>

				<button
					v-if="session.role !== 'admin'"
					class="action-btn food"
					@click="$emit('navigate', 'order')"
				>
					<el-icon :size="28"><Chicken /></el-icon>
					<span>Đồ ăn</span>
				</button>

				<button
					class="action-btn chat"
					@click="$emit('navigate', 'chat')"
				>
					<el-icon :size="28"><ChatDotSquare /></el-icon>
					<span>Hỗ trợ</span>
				</button>
			</div>
		</main>

		<footer class="dash-footer">
			<el-button text type="danger" size="small" @click="$emit('logout')">
				Đăng xuất
			</el-button>
		</footer>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Setting, Bell, Timer, Coin, Chicken, ChatDotSquare } from '@element-plus/icons-vue'
import { useSessionStore } from '../stores/session.store'
import TimerWidget from '../components/TimerWidget.vue'
import BalanceDisplay from '../components/BalanceDisplay.vue'
import NotificationList from '../components/NotificationList.vue'

const emit = defineEmits<{
	navigate: [view: string]
	logout: []
}>()

const session = useSessionStore()
const now = ref(Date.now())
const showNotif = ref(false)
const unreadCount = ref(0)
const notifRef = ref<InstanceType<typeof NotificationList> | null>(null)

let timer: number

function toggleNotif() {
	showNotif.value = !showNotif.value
	if (showNotif.value) {
		notifRef.value?.load()
	}
}

function handlePlaytime() {
	ElMessage.info('Phiên quản trị - không tính giờ')
}

onMounted(() => {
	session.loadData()
	timer = window.setInterval(() => {
		now.value = Date.now()
	}, 1000)
})

onUnmounted(() => {
	if (timer) clearInterval(timer)
})
</script>

<style scoped>
.dashboard {
	display: flex;
	flex-direction: column;
	height: 100vh;
	background: #f0f2f5;
}

.dash-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 12px 20px;
	background: #fff;
	box-shadow: 0 1px 4px rgba(0,0,0,.08);
}

.header-left {
	display: flex;
	align-items: center;
	gap: 10px;
}

.machine-tag {
	padding: 4px 12px;
	border-radius: 4px;
	font-weight: 600;
	font-size: 14px;
}

.machine-tag.available {
	background: #e8f5e9;
	color: #2e7d32;
}

.user-name {
	font-weight: 500;
	font-size: 15px;
}

.dash-main {
	flex: 1;
	overflow-y: auto;
	padding: 20px;
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 20px;
}

.actions-grid {
	display: grid;
	grid-template-columns: repeat(3, 1fr);
	gap: 16px;
	width: 100%;
	max-width: 400px;
}

.action-btn {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 8px;
	padding: 24px 8px;
	border: none;
	border-radius: 16px;
	cursor: pointer;
	font-size: 13px;
	font-weight: 500;
	color: #303133;
	transition: all .2s;
	background: #fff;
	box-shadow: 0 2px 8px rgba(0,0,0,.06);
}

.action-btn:hover {
	transform: translateY(-2px);
	box-shadow: 0 4px 16px rgba(0,0,0,.1);
}

.action-btn.topup { color: #e6a23c; }
.action-btn.playtime { color: #667eea; }
.action-btn.food { color: #67c23a; }
.action-btn.chat { color: #409eff; }

.dash-footer {
	text-align: center;
	padding: 8px 0;
}

.notif-badge .el-badge__content {
	top: 8px;
	right: 6px;
}
</style>
