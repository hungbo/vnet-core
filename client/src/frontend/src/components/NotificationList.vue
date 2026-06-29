<template>
	<el-card v-if="visible" shadow="never" class="notif-panel">
		<template #header>
			<div class="notif-header">
				<span>Thông báo</span>
				<el-button v-if="unreadCount > 0" text size="small" @click="markAllRead">Đã đọc tất cả</el-button>
			</div>
		</template>
		<div v-if="items.length === 0" class="notif-empty">Không có thông báo</div>
		<div v-for="n in items" :key="n.id" class="notif-item" :class="{ unread: !n.is_read }" @click="markRead(n)">
			<div class="notif-title">{{ n.title }}</div>
			<div class="notif-body">{{ n.body }}</div>
			<div class="notif-time">{{ formatTime(n.created_at) }}</div>
		</div>
	</el-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'

declare const window: any
const api = () => window.go?.main?.App

const props = defineProps<{
	visible: boolean
}>()

const emit = defineEmits<{
	close: []
	'update:unreadCount': [count: number]
}>()

const items = ref<any[]>([])
const unreadCount = ref(0)

function formatTime(t: string) {
	if (!t) return ''
	const d = new Date(t)
	const now = new Date()
	const diff = now.getTime() - d.getTime()
	if (diff < 60000) return 'Vừa xong'
	if (diff < 3600000) return `${Math.floor(diff / 60000)} phút trước`
	const h = String(d.getHours()).padStart(2, '0')
	const m = String(d.getMinutes()).padStart(2, '0')
	return `${h}:${m}`
}

async function load() {
	try {
		const unreadStr = await api().GetUnreadNotificationCount()
		const unreadData = JSON.parse(unreadStr)
		unreadCount.value = unreadData?.data?.count || 0
		emit('update:unreadCount', unreadCount.value)

		const notifStr = await api().GetNotifications()
		const parsed = JSON.parse(notifStr)
		items.value = parsed?.data?.items || []
	} catch { /* ignore */ }
}

async function markRead(n: any) {
	if (n.is_read) return
	try {
		await api().MarkNotificationRead(n.id)
		n.is_read = true
		unreadCount.value = Math.max(0, unreadCount.value - 1)
		emit('update:unreadCount', unreadCount.value)
	} catch { /* ignore */ }
}

async function markAllRead() {
	try {
		await api().MarkAllNotificationsRead()
		items.value.forEach((n: any) => n.is_read = true)
		unreadCount.value = 0
		emit('update:unreadCount', 0)
	} catch { /* ignore */ }
}

defineExpose({ load })
</script>

<style scoped>
.notif-panel {
	width: 100%;
	max-width: 400px;
	max-height: 320px;
	overflow-y: auto;
	border-radius: 12px;
}

.notif-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	font-size: 14px;
	font-weight: 600;
}

.notif-empty {
	text-align: center;
	color: #909399;
	padding: 24px 0;
	font-size: 13px;
}

.notif-item {
	padding: 10px 0;
	border-bottom: 1px solid #f0f0f0;
	cursor: pointer;
}

.notif-item:last-child {
	border-bottom: none;
}

.notif-item.unread {
	background: #f5f7fa;
	margin: 0 -16px;
	padding: 10px 16px;
}

.notif-title {
	font-size: 14px;
	font-weight: 500;
	color: #303133;
}

.notif-body {
	font-size: 13px;
	color: #606266;
	margin-top: 2px;
}

.notif-time {
	font-size: 11px;
	color: #c0c4cc;
	margin-top: 4px;
}
</style>
