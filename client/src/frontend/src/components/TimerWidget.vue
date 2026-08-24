<template>
	<div class="timer-section" v-if="role !== 'admin'">
		<div v-if="session" class="timer-card">
			<!--
				Tên nằm NGAY TRÊN đồng hồ chứ không ở thanh tiêu đề: khách nhìn
				vào con số đếm ngược, và cần thấy ngay nó đang đếm cho ai — máy
				dùng chung thì đăng nhập nhầm tài khoản là chuyện thường.
			-->
			<div v-if="name" class="timer-name">{{ name }}</div>
			<div class="timer-value" :class="{ warn: timerWarn }">{{ formattedTime }}</div>
			<div class="timer-label">{{ timerLabel }}</div>
			<div v-if="session.combo_name" class="combo-badge">
				<el-tag type="success" size="small">{{ session.combo_name }}</el-tag>
			</div>
		</div>
		<div v-else class="timer-card idle">
			<div v-if="name" class="timer-name">{{ name }}</div>
			<div class="timer-value">--:--:--</div>
			<div class="timer-label">Chưa có phiên chơi</div>
		</div>
	</div>

	<div v-if="role === 'admin'" class="timer-card admin-card">
		<div class="timer-value">Không tính giờ</div>
		<div class="timer-label">Chế độ quản trị</div>
	</div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
	role: string
	session: any
	now: number
	name?: string
}>()

function formatDuration(seconds: number): string {
	if (seconds < 0) seconds = 0
	const h = String(Math.floor(seconds / 3600)).padStart(2, '0')
	const m = String(Math.floor((seconds % 3600) / 60)).padStart(2, '0')
	const s = String(seconds % 60).padStart(2, '0')
	return `${h}:${m}:${s}`
}

const timerLabel = computed(() => {
	if (!props.session) return ''
	if (props.session.combo_type === 'fixed_slot' && props.session.slot_end) return 'Khung giờ kết thúc sau'
	if (props.session.combo_type === 'prepaid' && props.session.remaining_minutes) return 'Còn lại'
	// Số dư còn mua được bao nhiêu thời gian. Máy chủ đã tính sẵn mốc hết tiền
	// nên ở đây chỉ việc đếm ngược tới đó.
	if (props.session.affordable_until) return 'Số dư còn chơi được'
	return 'Đã chơi'
})

const timerWarn = computed(() => {
	if (!props.session) return false
	if (props.session.combo_type === 'fixed_slot' && props.session.slot_end) {
		const remain = new Date(props.session.slot_end).getTime() - props.now
		return remain > 0 && remain < 300000
	}
	// Nhánh trả theo giờ trước đây KHÔNG BAO GIỜ cảnh báo vì getRemainingSeconds
	// trả 0 — khách hết tiền mà không hề được báo trước.
	const remainSec = getRemainingSeconds()
	return remainSec > 0 && remainSec < 300
})

function getRemainingSeconds(): number {
	if (!props.session?.started_at) return 0
	const start = new Date(props.session.started_at).getTime()
	const elapsed = Math.floor((props.now - start) / 1000)

	if (props.session.combo_type === 'fixed_slot' && props.session.slot_end) {
		return Math.floor((new Date(props.session.slot_end).getTime() - props.now) / 1000)
	}

	if (props.session.combo_type === 'prepaid' && props.session.remaining_minutes) {
		return props.session.remaining_minutes * 60 - elapsed
	}

	if (props.session.affordable_until) {
		return Math.floor((new Date(props.session.affordable_until).getTime() - props.now) / 1000)
	}

	return 0
}

const formattedTime = computed(() => {
	if (!props.session?.started_at) return '--:--:--'

	if (props.session.combo_type === 'fixed_slot' && props.session.slot_end) {
		const remain = Math.floor((new Date(props.session.slot_end).getTime() - props.now) / 1000)
		return formatDuration(remain)
	}

	if (props.session.combo_type === 'prepaid' && props.session.remaining_minutes) {
		const remain = Math.max(0, props.session.remaining_minutes * 60 - Math.floor((props.now - new Date(props.session.started_at).getTime()) / 1000))
		return formatDuration(remain)
	}

	if (props.session.affordable_until) {
		return formatDuration(Math.max(0, getRemainingSeconds()))
	}

	// Máy không có giá (chưa gán nhóm) thì không có gì để đếm ngược — quay về
	// đếm lên như cũ.
	const start = new Date(props.session.started_at).getTime()
	const diff = Math.floor((props.now - start) / 1000)
	return formatDuration(diff)
})
</script>

<style scoped>
.timer-card {
	/* Rộng bằng thanh, không cố định 320px: thanh chỉ 360px nên con số cứng đó
	   vừa tràn vừa lệch với lưới nút bên dưới. */
	width: 100%;
	padding: 24px;
	background: var(--vnet-surface);
	border-radius: 16px;
	text-align: center;
	box-shadow: var(--vnet-shadow);
}

.timer-card.idle {
	opacity: .6;
}

.timer-card.admin-card {
	background: linear-gradient(135deg, #667eea, #764ba2);
	color: var(--vnet-surface);
}

.timer-name {
	font-size: 15px;
	font-weight: 600;
	color: var(--vnet-text);
	margin-bottom: 4px;
	/* Tên dài phải cắt bằng ba chấm, không được đẩy đồng hồ xuống dòng. */
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.timer-value {
	font-size: 48px;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	letter-spacing: 2px;
}

.timer-value.warn {
	color: var(--vnet-warning);
}

.timer-label {
	font-size: 13px;
	color: var(--vnet-text-muted);
	margin-top: 4px;
}

.admin-card .timer-label {
	color: rgba(255,255,255,.7);
}

.combo-badge {
	margin-top: 8px;
}
</style>
