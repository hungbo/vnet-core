<template>
	<div class="dashboard">
		<header class="dash-header">
			<div class="header-left">
				<span class="machine-tag" :title="session.machineCode">{{ session.machineCode }}</span>
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
			<TimerWidget :role="session.role" :session="session.session" :now="now" :name="session.displayName" />
			<BalanceDisplay :role="session.role" :memberInfo="session.memberInfo" />

			<NotificationList ref="notifRef" :visible="showNotif" @update:unreadCount="unreadCount = $event" />

			<div class="actions-grid">
				<button
					v-for="a in visibleActions"
					:key="a.key"
					class="action-btn"
					:class="{ primary: a.key === 'topup' }"
					@click="a.run()"
				>
					<el-icon :size="26" :style="{ color: a.color }"><component :is="a.icon" /></el-icon>
					<span>{{ a.label }}</span>
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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Setting, Bell, Timer, Coin, Chicken, ChatDotSquare, Calendar, Star, Trophy } from '@element-plus/icons-vue'
import { useSessionStore } from '../stores/session.store'
import { useUiStore } from '../stores/ui.store'
import TimerWidget from '../components/TimerWidget.vue'
import BalanceDisplay from '../components/BalanceDisplay.vue'
import NotificationList from '../components/NotificationList.vue'

const emit = defineEmits<{
	navigate: [view: string]
	logout: []
}>()

const session = useSessionStore()
const ui = useUiStore()
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

/**
 * Một bảng duy nhất mô tả mọi ô chức năng.
 *
 * Bản cũ viết tay sáu nút với bốn điều kiện v-if khác nhau, nên số ô đổi theo
 * vai trò mà lưới thì cố định ba cột: hội viên thấy 6 ô kín, vai trò combo thấy
 * 3 ô, quản trị chỉ 2 ô nằm lệch hẳn sang trái. Gom về một mảng thì số ô và điều
 * kiện hiện nằm cạnh nhau, nhìn một chỗ là biết vai trò nào thấy gì.
 *
 * `feature` nối vào công tắc bật/tắt ở trang quản trị; để trống nghĩa là luôn có.
 */
const ACTIONS = [
	{ key: 'topup', icon: Coin, label: 'Nạp tiền', color: '#d9a441', roles: ['member'], run: () => emit('navigate', 'topup') },
	{ key: 'playtime', icon: Timer, label: 'Giờ chơi', color: '#4c7dff', roles: ['admin'], run: handlePlaytime },
	{ key: 'order', icon: Chicken, label: 'Đồ ăn', color: '#f0603d', notRoles: ['admin'], run: () => emit('navigate', 'order') },
	{ key: 'games', icon: Trophy, label: 'Game', color: '#7b5cff', notRoles: ['admin'], run: () => emit('navigate', 'games') },
	{ key: 'chat', icon: ChatDotSquare, label: 'Hỗ trợ', color: '#3fb950', run: () => emit('navigate', 'chat') },
	{ key: 'attendance', icon: Calendar, label: 'Điểm danh', color: '#4c7dff', roles: ['member'], feature: 'attendance_enabled', run: () => emit('navigate', 'attendance') },
	{ key: 'feedback', icon: Star, label: 'Đánh giá', color: '#d9a441', notRoles: ['admin'], feature: 'feedback_enabled', run: () => emit('navigate', 'feedback') },
] as const

const visibleActions = computed(() =>
	ACTIONS.filter(a => {
		const role = session.role
		if ('roles' in a && a.roles && !a.roles.includes(role as never)) return false
		if ('notRoles' in a && a.notRoles && a.notRoles.includes(role as never)) return false
		if ('feature' in a && a.feature && !ui.enabled(a.feature)) return false
		return true
	})
)

onMounted(() => {
	session.loadData()
	ui.load()
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
	/* Lấp phần còn lại dưới DockTitleBar, không phải cả màn hình (100vh sẽ tràn). */
	flex: 1;
	min-height: 0;
	background:
		radial-gradient(140% 40% at 100% 0%, rgba(76, 125, 255, 0.12), transparent 70%),
		var(--vnet-bg);
}

.dash-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 10px 8px 10px var(--vnet-gap);
	border-bottom: 1px solid var(--vnet-border);
}

.header-left {
	display: flex;
	align-items: center;
	gap: 8px;
	min-width: 0;
}

/* Tên máy mang dáng chữ logo: nghiêng, vát góc. Không còn ô nền xanh lá
   sáng — mảnh sót của bản giao diện sáng. */
.machine-tag {
	font-family: var(--vnet-font-display);
	font-style: italic;
	font-weight: 700;
	font-size: 18px;
	color: var(--vnet-text);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.dash-main {
	flex: 1;
	overflow-y: auto;
	padding: var(--vnet-gap);
	display: flex;
	flex-direction: column;
	align-items: stretch;
	gap: var(--vnet-gap);
}

.actions-grid {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 8px;
	width: 100%;
}

/* Nút dạng hàng ngang: biểu tượng trái, chữ phải. Thanh chỉ rộng 360px nên
   ô vuông to làm danh sách dài quá màn hình; hàng thấp đọc lướt nhanh hơn. */
.action-btn {
	display: flex;
	align-items: center;
	gap: 10px;
	height: 56px;
	padding: 0 14px;
	border: 1px solid var(--vnet-border);
	border-radius: var(--vnet-radius-sm);
	background: var(--vnet-surface);
	color: var(--vnet-text);
	font-family: var(--vnet-font);
	font-size: 14px;
	font-weight: 500;
	text-align: left;
	cursor: pointer;
	transition: background 0.15s, border-color 0.15s;
}

.action-btn:hover {
	border-color: var(--vnet-primary);
	background: var(--vnet-surface-2);
}

/* Nạp tiền là việc khách cần nhất khi sắp hết giờ: chiếm trọn một hàng và
   mang màu logo. */
.action-btn.primary {
	grid-column: 1 / -1;
	justify-content: center;
	border: none;
	background: var(--vnet-brand);
	font-weight: 600;
	font-size: 15px;
}

.action-btn.primary :deep(.el-icon) {
	color: #fff !important;
}

.dash-footer {
	text-align: center;
	padding: 8px 0;
	border-top: 1px solid var(--vnet-border);
}

.notif-badge .el-badge__content {
	top: 8px;
	right: 6px;
}
</style>
