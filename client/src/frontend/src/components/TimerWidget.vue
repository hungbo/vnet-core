<template>
	<div class="timer-section" v-if="role !== 'admin'">
		<div v-if="session" class="timer-card">
			<div class="timer-value" :class="{ warn: timerWarn }">{{ formattedTime }}</div>
			<div class="timer-label">{{ timerLabel }}</div>
			<div v-if="session.combo_name" class="combo-badge">
				<el-tag type="success" size="small">{{ session.combo_name }}</el-tag>
			</div>
		</div>
		<div v-else class="timer-card idle">
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
	return 'Đã chơi'
})

const timerWarn = computed(() => {
	if (!props.session) return false
	if (props.session.combo_type === 'fixed_slot' && props.session.slot_end) {
		const remain = new Date(props.session.slot_end).getTime() - props.now
		return remain > 0 && remain < 300000
	}
	if (props.session.remaining_minutes) {
		const remainSec = getRemainingSeconds()
		return remainSec > 0 && remainSec < 300
	}
	return false
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

	const start = new Date(props.session.started_at).getTime()
	const diff = Math.floor((props.now - start) / 1000)
	return formatDuration(diff)
})
</script>

<style scoped>
.timer-card {
	width: 320px;
	padding: 24px;
	background: #fff;
	border-radius: 16px;
	text-align: center;
	box-shadow: 0 2px 12px rgba(0,0,0,.06);
}

.timer-card.idle {
	opacity: .6;
}

.timer-card.admin-card {
	background: linear-gradient(135deg, #667eea, #764ba2);
	color: #fff;
}

.timer-value {
	font-size: 48px;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	letter-spacing: 2px;
}

.timer-value.warn {
	color: #e6a23c;
}

.timer-label {
	font-size: 13px;
	color: #909399;
	margin-top: 4px;
}

.admin-card .timer-label {
	color: rgba(255,255,255,.7);
}

.combo-badge {
	margin-top: 8px;
}
</style>
