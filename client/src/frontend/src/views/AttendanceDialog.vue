<template>
	<div class="overlay" @click.self="$emit('close')">
		<div class="panel">
			<header class="panel-head">
				<h3>Điểm danh hằng ngày</h3>
				<el-button text circle @click="$emit('close')">
					<el-icon><Close /></el-icon>
				</el-button>
			</header>

			<div v-loading="loading" class="panel-body">
				<div class="streak">
					<div class="streak-num">{{ status?.streak_days ?? 0 }}</div>
					<div class="streak-label">ngày liên tiếp</div>
				</div>

				<el-alert
					v-if="status?.checked_in_today"
					type="success"
					:closable="false"
					show-icon
					title="Hôm nay bạn đã điểm danh rồi"
				/>
				<el-alert
					v-else
					type="info"
					:closable="false"
					show-icon
					:title="`Điểm danh hôm nay để nhận ${formatMoney(status?.next_reward)}`"
				/>

				<p v-if="status?.streak_every" class="hint">
					Cứ {{ status.streak_every }} ngày liên tiếp sẽ có thêm phần thưởng chuỗi.
				</p>
				<p v-if="status?.last_checkin" class="hint">
					Lần điểm danh gần nhất: {{ formatDate(status.last_checkin) }}
				</p>

				<el-button
					type="primary"
					size="large"
					class="checkin-btn"
					:loading="submitting"
					:disabled="status?.checked_in_today"
					@click="checkin"
				>
					{{ status?.checked_in_today ? 'Đã điểm danh' : 'Điểm danh ngay' }}
				</el-button>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { Close } from '@element-plus/icons-vue'
import { useSessionStore } from '../stores/session.store'

// GetAttendanceStatus/CheckinAttendance đã có trong app.go từ trước nhưng phía
// Vue không gọi hàm nào — khách không có đường nào để điểm danh.

declare const window: any
const api = () => window.go?.main?.App

defineEmits<{ close: [] }>()

const session = useSessionStore()
const loading = ref(false)
const submitting = ref(false)
const status = ref<any>(null)

function formatMoney(v: number | undefined | null) {
	return `${new Intl.NumberFormat('vi-VN').format(v || 0)}₫`
}

function formatDate(v: string) {
	const d = new Date(v)
	return Number.isNaN(d.getTime()) ? v : d.toLocaleDateString('vi-VN')
}

// Máy chủ trả phong bì { code, message, data } nên bóc một lớp trước khi dùng.
function unwrap(raw: string) {
	const parsed = JSON.parse(raw)
	return parsed?.data ?? parsed
}

async function load() {
	loading.value = true
	try {
		status.value = unwrap(await api().GetAttendanceStatus())
	} catch (e) {
		ElMessage.error(String(e))
	} finally {
		loading.value = false
	}
}

async function checkin() {
	submitting.value = true
	try {
		const result = unwrap(await api().CheckinAttendance())
		ElNotification({
			type: 'success',
			title: `Điểm danh ngày thứ ${result.streak_days}`,
			message: result.reward_amount > 0
				? `Đã cộng ${formatMoney(result.reward_amount)} vào điểm thưởng`
				: 'Đã ghi nhận điểm danh hôm nay',
		})
		// Số dư thưởng vừa đổi — nạp lại để màn hình chính hiện đúng.
		session.loadData()
		await load()
	} catch (e) {
		ElMessage.error(String(e))
	} finally {
		submitting.value = false
	}
}

onMounted(load)
</script>

<style scoped>
.overlay {
	position: fixed;
	inset: 0;
	background: rgba(0, 0, 0, 0.55);
	display: flex;
	align-items: center;
	justify-content: center;
	z-index: 2000;
}

.panel {
	width: 420px;
	max-width: 92vw;
	background: var(--vnet-surface);
	border-radius: 12px;
	overflow: hidden;
}

.panel-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 16px 20px;
	border-bottom: 1px solid var(--vnet-border);
}

.panel-head h3 {
	font-size: 16px;
	font-weight: 600;
}

.panel-body {
	padding: 20px;
	display: flex;
	flex-direction: column;
	gap: 12px;
}

.streak {
	text-align: center;
	padding: 8px 0 4px;
}

.streak-num {
	font-size: 48px;
	font-weight: 700;
	line-height: 1;
	color: var(--vnet-primary);
}

.streak-label {
	margin-top: 4px;
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.hint {
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.checkin-btn {
	margin-top: 4px;
	width: 100%;
}
</style>
