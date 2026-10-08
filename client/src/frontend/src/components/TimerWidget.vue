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
import { computed, watch } from 'vue'
import { ElNotification } from 'element-plus'
import { mocVuaQua, phatThongBao } from '../utils/timeAnnounce'

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

// Báo giờ còn lại cho khách (không báo cho nhân viên). Nhớ số giây của lần đo
// trước theo từng phiên: đổi phiên thì bắt đầu lại; nạp thêm tiền làm thời gian
// dài ra thì các mốc tự được báo lại khi đi xuống lần nữa.
let phienDangDo = ''
let giayTruoc: number | null = null
watch(() => props.now, () => {
	const id = props.session?.id || props.session?.session_id || ''
	if (props.role === 'admin' || !props.session?.started_at || !coDemNguoc()) {
		phienDangDo = ''
		giayTruoc = null
		return
	}
	if (id !== phienDangDo) {
		phienDangDo = id
		giayTruoc = null
	}
	const giay = getRemainingSeconds()
	const moc = mocVuaQua(giayTruoc, giay)
	giayTruoc = giay
	if (!moc) return
	phatThongBao(moc)
	ElNotification({
		title: `Còn ${moc} phút sử dụng`,
		message: moc <= 5 ? 'Vui lòng nạp thêm tiền nếu muốn chơi tiếp.' : '',
		type: moc <= 5 ? 'warning' : 'info',
		duration: 8000,
	})
})

// Chỉ những phiên CÓ mốc kết thúc mới báo; máy chưa có giá thì đồng hồ đếm lên.
function coDemNguoc(): boolean {
	const s = props.session
	return Boolean((s.combo_type === 'fixed_slot' && s.slot_end) ||
		(s.combo_type === 'prepaid' && s.remaining_minutes) || s.affordable_until)
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
/* Đồng hồ không đóng khung như mọi khối khác: nó là thứ khách nhìn nhiều nhất,
   nên đứng riêng trên nền với một vạch màu logo ở mép trái. Sắp hết giờ thì
   vạch và số chuyển sang hổ phách. */
.timer-card {
	position: relative;
	width: 100%;
	padding: 18px 16px 18px 22px;
	border-radius: var(--vnet-radius);
	background: linear-gradient(90deg, rgba(76, 125, 255, 0.12), transparent 80%);
}

.timer-card::before {
	content: '';
	position: absolute;
	left: 0;
	top: 14px;
	bottom: 14px;
	width: 4px;
	border-radius: 4px;
	background: var(--vnet-brand);
}

.timer-card:has(.warn) {
	background: linear-gradient(90deg, rgba(255, 181, 71, 0.14), transparent 80%);
}

.timer-card:has(.warn)::before {
	background: var(--vnet-warning);
}

.timer-card.idle {
	opacity: 0.6;
}

.timer-card.admin-card {
	background: linear-gradient(90deg, rgba(123, 92, 255, 0.18), transparent 80%);
}

.timer-name {
	font-size: 14px;
	font-weight: 500;
	color: var(--vnet-text-muted);
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.timer-value {
	font-family: var(--vnet-font-display);
	font-weight: 700;
	font-size: 52px;
	line-height: 1.1;
	font-variant-numeric: tabular-nums;
	color: var(--vnet-text);
}

.timer-value.warn {
	color: var(--vnet-warning);
}

.admin-card .timer-value {
	font-size: 30px;
}

.timer-label {
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.combo-badge {
	margin-top: 8px;
}
</style>
