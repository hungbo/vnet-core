<template>
	<div class="dashboard">
		<header class="dash-header">
			<div class="header-left">
				<span class="machine-tag" :class="session.machineStatus">{{ session.machineCode }}</span>
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
				<button v-for="a in visibleActions" :key="a.key" class="action-btn" @click="a.run()">
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
import { Setting, Bell, Timer, Coin, Chicken, ChatDotSquare, Calendar, Star } from '@element-plus/icons-vue'
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
	height: 100vh;
	background: var(--vnet-bg);
}

.dash-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 10px var(--vnet-gap);
	background: var(--vnet-surface);
	border-bottom: 1px solid var(--vnet-border);
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

.dash-main {
	flex: 1;
	overflow-y: auto;
	padding: var(--vnet-gap);
	display: flex;
	flex-direction: column;
	/* stretch chứ không center: thanh chỉ rộng 360px, căn giữa rồi để thẻ cố định
	   320px là thừa hai mép và các khối rộng khác nhau. */
	align-items: stretch;
	gap: var(--vnet-gap);
}

.actions-grid {
	display: grid;
	/* Hai cột cho thanh dọc 360px. auto-fit sẽ để một ô lẻ giãn hết hàng khi số
	   ô là số lẻ, nên cố định cột và cho ô vuông là cách duy nhất giữ chúng đều
	   nhau ở mọi vai trò. */
	grid-template-columns: repeat(2, 1fr);
	gap: var(--vnet-gap);
	width: 100%;
	/* Mục lưới mặc định là stretch, nghĩa là chiều cao lấy từ hàng — và khi chiều
	   cao đã xác định thì aspect-ratio của ô bị bỏ qua. Phải để start thì ô mới
	   thật sự vuông. */
	align-items: start;
}

.action-btn {
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 8px;
	/* Ô VUÔNG. Bản cũ để chiều cao chạy theo nội dung: mọi nút tình cờ bằng nhau
	   vì đều một icon một dòng chữ, nhưng không có ràng buộc nào — thêm một nhãn
	   dài hơn là hàng đó cao hơn hàng kia. */
	aspect-ratio: 1;
	padding: 8px;
	border: 1px solid var(--vnet-border);
	border-radius: var(--vnet-radius);
	cursor: pointer;
	font-size: 13px;
	font-weight: 500;
	color: var(--vnet-text);
	transition: background .15s, border-color .15s, transform .15s;
	background: var(--vnet-surface);
}

.action-btn:hover {
	transform: translateY(-2px);
	border-color: var(--vnet-primary);
	background: var(--vnet-surface-2);
}

.action-btn:active {
	transform: none;
}

.dash-footer {
	text-align: center;
	padding: 8px 0;
}

.notif-badge .el-badge__content {
	top: 8px;
	right: 6px;
}
</style>
