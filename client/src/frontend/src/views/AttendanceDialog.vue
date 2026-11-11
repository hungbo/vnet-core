<template>
	<div class="overlay" :class="{ standalone }" @click.self="!standalone && $emit('close')">
		<div class="card" v-loading="loading">
			<el-button v-if="!standalone" class="card-close" text circle @click="$emit('close')">
				<el-icon><Close /></el-icon>
			</el-button>

			<header class="card-head">
				<span class="head-icon"><el-icon><Calendar /></el-icon></span>
				<div>
					<h2>Điểm danh hằng ngày</h2>
					<p>Mỗi ngày một lần, liên tiếp để nhận thêm thưởng chuỗi.</p>
				</div>
			</header>

			<div class="streak">
				<span class="streak-num">{{ status?.streak_days ?? 0 }}</span>
				<span class="streak-label">ngày liên tiếp</span>
			</div>

			<!--
				Dải ngày của chuỗi hiện tại, ô cuối là mốc thưởng chuỗi. Chỉ vẽ khi mốc
				ngắn đủ để xếp vừa một hàng; mốc dài hơn thì dòng chữ bên dưới đã đủ.
			-->
			<ol v-if="oNgay.length" class="track">
				<li
					v-for="o in oNgay"
					:key="o.ngay"
					class="day"
					:class="{ xong: o.xong, 'hom-nay': o.homNay, moc: o.moc }"
				>
					<span class="day-label">{{ o.homNay ? 'Hôm nay' : `Ngày ${o.ngay}` }}</span>
					<span class="day-mark">
						<el-icon v-if="o.xong"><Check /></el-icon>
						<el-icon v-else-if="o.moc"><Present /></el-icon>
						<template v-else>{{ o.ngay }}</template>
					</span>
					<span v-if="o.homNay && !daDiemDanh && thuongHomNay > 0" class="day-reward">
						+{{ formatMoney(thuongHomNay) }}
					</span>
				</li>
			</ol>

			<p v-if="dongMoc" class="milestone">{{ dongMoc }}</p>

			<div v-if="daDiemDanh" class="state done">
				<el-icon><CircleCheckFilled /></el-icon>
				<span>Hôm nay bạn đã điểm danh. Hẹn gặp lại ngày mai!</span>
			</div>
			<div v-else-if="thuongHomNay > 0" class="state reward">
				<span>Thưởng hôm nay</span>
				<strong>+{{ formatMoney(thuongHomNay) }}</strong>
			</div>

			<button
				type="button"
				class="action"
				:disabled="daDiemDanh || submitting || !status"
				@click="checkin"
			>
				<el-icon v-if="submitting" class="is-loading"><Loading /></el-icon>
				{{ daDiemDanh ? 'Đã điểm danh hôm nay' : 'Điểm danh ngay' }}
			</button>

			<p v-if="status?.last_checkin" class="foot">
				Lần điểm danh gần nhất: {{ formatDate(status.last_checkin) }}
			</p>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { Calendar, Check, CircleCheckFilled, Close, Loading, Present } from '@element-plus/icons-vue'
import { useSessionStore } from '../stores/session.store'
import { useBaoSanSang } from '../utils/panelReady'

// GetAttendanceStatus/CheckinAttendance đã có trong app.go từ trước nhưng phía
// Vue không gọi hàm nào — khách không có đường nào để điểm danh.

declare const window: any
const api = () => window.go?.main?.App

defineEmits<{ close: [] }>()
defineProps<{ standalone?: boolean }>()

const session = useSessionStore()
const baoSanSang = useBaoSanSang()
const loading = ref(false)
const submitting = ref(false)
const status = ref<any>(null)

// Mốc chuỗi dài hơn thế này thì một hàng ô không còn đọc được.
const SO_O_TOI_DA = 10

const daDiemDanh = computed(() => !!status.value?.checked_in_today)
// next_reward của máy chủ đã gồm cả thưởng chuỗi nếu hôm nay chạm mốc.
const thuongHomNay = computed(() => Number(status.value?.next_reward || 0))
const moc = computed(() => Number(status.value?.streak_every || 0))

// Số ngày đã xong trong vòng chuỗi hiện tại. Đã điểm danh hôm nay mà vừa chạm mốc
// thì vòng đó đầy (không quay về 0), để khách thấy trọn dải ô vừa hoàn thành.
const daXongTrongVong = computed(() => {
	const n = Number(status.value?.streak_days || 0)
	if (moc.value <= 0) return 0
	return daDiemDanh.value ? ((n - 1) % moc.value) + 1 : n % moc.value
})

const oNgay = computed(() => {
	if (!status.value || moc.value < 2 || moc.value > SO_O_TOI_DA) return []
	const homNay = daDiemDanh.value ? daXongTrongVong.value : daXongTrongVong.value + 1
	return Array.from({ length: moc.value }, (_, i) => {
		const ngay = i + 1
		return { ngay, xong: ngay <= daXongTrongVong.value, homNay: ngay === homNay, moc: ngay === moc.value }
	})
})

const dongMoc = computed(() => {
	if (!status.value || moc.value <= 0) return ''
	const conLai = moc.value - daXongTrongVong.value
	if (daDiemDanh.value) {
		return conLai === 0
			? `Bạn vừa hoàn thành chuỗi ${moc.value} ngày!`
			: `Còn ${conLai} ngày nữa tới mốc thưởng chuỗi ${moc.value} ngày.`
	}
	return conLai === 1
		? `Hôm nay là ngày thứ ${moc.value} — điểm danh để nhận thưởng chuỗi!`
		: `Điểm danh đủ ${moc.value} ngày liên tiếp để nhận thêm thưởng chuỗi.`
})

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

onMounted(async () => {
	await load()
	baoSanSang()
})
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

/* Dạng cửa sổ riêng (tiến trình --window): không lớp phủ, thẻ nằm giữa cửa sổ. */
.overlay.standalone {
	position: static;
	min-height: 100vh;
	padding: 24px;
	background:
		radial-gradient(ellipse at top, rgba(123, 92, 255, 0.16), transparent 60%),
		var(--vnet-bg);
	color: var(--vnet-text);
}

.card {
	position: relative;
	width: 100%;
	max-width: 540px;
	display: flex;
	flex-direction: column;
	gap: 18px;
	padding: 28px;
	background: var(--vnet-surface);
	border: 1px solid var(--vnet-border);
	border-radius: 18px;
	box-shadow: var(--vnet-shadow);
}

.card-close {
	position: absolute;
	top: 12px;
	right: 12px;
}

.card-head {
	display: flex;
	align-items: center;
	gap: 14px;
}

.head-icon {
	flex: none;
	display: grid;
	place-items: center;
	width: 48px;
	height: 48px;
	border-radius: 14px;
	background: var(--vnet-brand);
	color: #fff;
	font-size: 24px;
}

.card-head h2 {
	font-size: 20px;
	font-weight: 700;
}

.card-head p {
	margin-top: 2px;
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.streak {
	display: flex;
	flex-direction: column;
	align-items: center;
	padding: 6px 0 2px;
}

.streak-num {
	font-family: var(--vnet-font-display);
	font-size: 72px;
	font-weight: 700;
	line-height: 1;
	background: var(--vnet-brand);
	-webkit-background-clip: text;
	background-clip: text;
	color: transparent;
}

.streak-label {
	margin-top: 4px;
	font-size: 14px;
	color: var(--vnet-text-muted);
}

.track {
	list-style: none;
	display: grid;
	grid-auto-flow: column;
	grid-auto-columns: 1fr;
	gap: 8px;
}

.day {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 6px;
	padding: 10px 2px;
	border: 1px solid var(--vnet-border);
	border-radius: 12px;
	background: var(--vnet-surface-2);
	color: var(--vnet-text-faint);
}

.day-label {
	font-size: 11px;
	white-space: nowrap;
}

.day-mark {
	display: grid;
	place-items: center;
	width: 30px;
	height: 30px;
	border-radius: 50%;
	font-family: var(--vnet-font-display);
	font-weight: 700;
	font-size: 15px;
	background: var(--vnet-bg);
}

.day.moc .day-mark {
	color: var(--vnet-warning);
	font-size: 17px;
}

.day.xong {
	border-color: rgba(61, 214, 140, 0.45);
	color: var(--vnet-success);
}

.day.xong .day-mark {
	background: var(--vnet-success);
	color: var(--vnet-bg);
	font-size: 17px;
}

/* Ô của hôm nay khi chưa điểm danh: viền thương hiệu, nhấp nháy nhẹ để mắt tìm tới. */
.day.hom-nay:not(.xong) {
	border-color: var(--vnet-primary);
	background: var(--vnet-primary-soft);
	color: var(--vnet-text);
	animation: nhip 1.8s ease-in-out infinite;
}

.day-reward {
	font-size: 11px;
	font-weight: 600;
	color: var(--vnet-warning);
	white-space: nowrap;
}

@keyframes nhip {
	50% { box-shadow: 0 0 0 4px rgba(76, 125, 255, 0.18); }
}

.milestone {
	text-align: center;
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.state {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 10px;
	padding: 14px 16px;
	border-radius: 12px;
	font-size: 14px;
}

.state.reward {
	justify-content: space-between;
	background: rgba(255, 181, 71, 0.1);
	border: 1px solid rgba(255, 181, 71, 0.35);
	color: var(--vnet-text-muted);
}

.state.reward strong {
	font-family: var(--vnet-font-display);
	font-size: 24px;
	color: var(--vnet-warning);
}

.state.done {
	background: rgba(61, 214, 140, 0.1);
	border: 1px solid rgba(61, 214, 140, 0.35);
	color: var(--vnet-success);
}

.state.done .el-icon {
	font-size: 20px;
}

/* Cùng kiểu nút chính "Nạp tiền" ở thanh điều khiển. */
.action {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 8px;
	height: 52px;
	border: 0;
	border-radius: 12px;
	background: var(--vnet-brand);
	color: #fff;
	font-family: inherit;
	font-size: 16px;
	font-weight: 600;
	cursor: pointer;
	transition: filter .15s, transform .15s;
}

.action:hover:not(:disabled) {
	filter: brightness(1.1);
	transform: translateY(-1px);
}

.action:disabled {
	cursor: default;
	background: var(--vnet-surface-2);
	color: var(--vnet-text-faint);
}

.foot {
	text-align: center;
	font-size: 12px;
	color: var(--vnet-text-faint);
}
</style>
