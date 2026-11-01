<template>
	<!--
		Màn trả tiền bằng mã QR chuyển khoản, dùng chung cho nạp tiền (TopupDialog) và
		thanh toán đơn đã gọi (MyOrders). Máy chủ đã tạo mã; ở đây chỉ hiện mã, đếm
		ngược, và đổi sang "đã trả" khi điện thoại báo tiền ở quầy thấy tiền về.
	-->
	<div class="qr-screen">
		<template v-if="state === 'paid'">
			<div class="qr-result qr-paid">
				<strong>{{ paidTitle }} {{ money(qr.amount) }}</strong>
				<span>{{ paidText }}</span>
			</div>
			<el-button size="large" class="full-btn" @click="$emit('close')">Đóng</el-button>
		</template>
		<template v-else-if="state === 'expired'">
			<div class="qr-result qr-expired">
				<strong>Mã QR đã hết hạn</strong>
				<span>Bạn đã chuyển khoản rồi thì đừng chuyển lại, báo nhân viên quầy kiểm tra giúp.</span>
			</div>
			<el-button type="primary" size="large" class="full-btn" @click="$emit('retry')">Tạo mã mới</el-button>
		</template>
		<template v-else>
			<div class="qr-amount">{{ money(qr.amount) }}</div>
			<img :src="qr.qr_image" class="qr-img" alt="Mã QR chuyển khoản" draggable="false" />
			<dl class="qr-lines">
				<div><dt>Ngân hàng</dt><dd>{{ qr.bank_name || qr.bank }}</dd></div>
				<div><dt>Chủ tài khoản</dt><dd>{{ qr.account_name }}</dd></div>
				<div><dt>Số tài khoản</dt><dd>{{ qr.account_no }}</dd></div>
				<div><dt>Nội dung</dt><dd class="qr-code">{{ qr.code }}</dd></div>
			</dl>
			<p class="qr-countdown">Mã còn hiệu lực <strong>{{ conLai }}</strong></p>
			<p class="hint">
				Mở app ngân hàng và quét mã. Giữ nguyên số tiền và nội dung. Chuyển xong là
				tự xác nhận, không cần báo quầy.
			</p>
			<el-button size="large" class="full-btn" style="margin-top: 12px;" :loading="huying" @click="huy">
				{{ cancelText }}
			</el-button>
		</template>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { EventsOn } from '../../wailsjs/runtime/runtime'

declare const window: any
const api = () => window.go?.main?.App

// qr: trả lời của RequestTopupQR / PayOrderQR (request_id, order_id, code, amount,
// expires_at, bank_name, account_no, account_name, qr_image).
const props = defineProps<{
	qr: any
	paidTitle: string
	paidText: string
	cancelText: string
}>()
const emit = defineEmits<{
	paid: []
	close: []
	retry: []
	cancelled: []
}>()

const state = ref<'pending' | 'paid' | 'expired'>('pending')
const huying = ref(false)
const bayGio = ref(Date.now())
let dongHo: ReturnType<typeof setInterval> | undefined
let hoiLai: ReturnType<typeof setInterval> | undefined
const huySuKien: Array<() => void> = []

const conLai = computed(() => {
	const ms = new Date(props.qr.expires_at).getTime() - bayGio.value
	const giay = Math.max(0, Math.ceil(ms / 1000))
	return `${Math.floor(giay / 60)}:${String(giay % 60).padStart(2, '0')}`
})

function money(n: number) {
	return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(n || 0)
}

function dungTheoDoi() {
	clearInterval(dongHo)
	clearInterval(hoiLai)
	dongHo = hoiLai = undefined
}

function daTra() {
	if (state.value === 'paid') return
	state.value = 'paid'
	dungTheoDoi()
	emit('paid')
}

function hetHan() {
	if (state.value !== 'pending') return
	state.value = 'expired'
	dungTheoDoi()
}

// Lối dự phòng cho sự kiện WebSocket bị rơi (mạng chập, WebSocket đang nối lại):
// hỏi máy chủ mỗi 5 giây. Tiền đã về mà màn hình vẫn đứng chờ là khách ra quầy hỏi.
async function hoiTrangThai() {
	try {
		const r = JSON.parse(await api().GetQRRequest(props.qr.request_id))
		if (r?.status === 'paid') daTra()
		else if (r?.status === 'expired' || r?.status === 'cancelled') hetHan()
	} catch { /* lượt sau hỏi lại */ }
}

async function huy() {
	huying.value = true
	try {
		await api().CancelQRRequest(props.qr.request_id)
		dungTheoDoi()
		emit('cancelled')
	} catch (e) {
		// Không huỷ được thường là vì tiền vừa về: hỏi lại để màn hình nói đúng.
		ElMessage.error(String(e).replace(/^Error:\s*/, ''))
		hoiTrangThai()
	} finally {
		huying.value = false
	}
}

onMounted(() => {
	huySuKien.push(EventsOn('vnet:qr:paid', (d: any) => {
		if (d?.request_id === props.qr.request_id) daTra()
	}))
	huySuKien.push(EventsOn('vnet:qr:expired', (d: any) => {
		if (d?.request_id === props.qr.request_id) hetHan()
	}))
	// Nhân viên duyệt tay ở quầy đơn nạp của mã này cũng là đã trả.
	huySuKien.push(EventsOn('vnet:topup:confirmed', (d: any) => {
		if (d?.order_id && d.order_id === props.qr.order_id) daTra()
	}))
	dongHo = setInterval(() => {
		bayGio.value = Date.now()
		if (conLai.value === '0:00') hetHan()
	}, 1000)
	hoiLai = setInterval(hoiTrangThai, 5000)
})

onUnmounted(() => {
	dungTheoDoi()
	huySuKien.forEach(f => f())
})
</script>

<style scoped>
.qr-screen {
	display: flex;
	flex-direction: column;
	align-items: stretch;
}

.full-btn {
	width: 100%;
	/* Element Plus đẩy nút đứng sau một nút khác sang phải 12px. */
	margin-left: 0 !important;
}

.hint {
	font-size: 11px;
	color: var(--vnet-text-muted);
	text-align: center;
	margin-top: 8px;
}

.qr-amount {
	font-family: var(--vnet-font-display);
	font-size: 28px;
	font-weight: 700;
	text-align: center;
	color: var(--vnet-success);
	margin-bottom: 12px;
}

/* Nền trắng và viền trắng quanh mã: app ngân hàng quét mã tối trên nền tối kém hơn
   hẳn, nhất là camera điện thoại cũ. */
.qr-img {
	width: 240px;
	height: 240px;
	align-self: center;
	padding: 10px;
	background: #fff;
	border-radius: var(--vnet-radius-sm);
	image-rendering: pixelated;
}

.qr-lines {
	margin: 16px 0 4px;
	padding: 12px 16px;
	background: var(--vnet-surface-2);
	border: 1px solid var(--vnet-border);
	border-radius: 8px;
	font-size: 13px;
}

.qr-lines > div {
	display: flex;
	justify-content: space-between;
	gap: 12px;
	padding: 3px 0;
}

.qr-lines dt {
	color: var(--vnet-text-muted);
	flex: none;
}

.qr-lines dd {
	font-weight: 600;
	text-align: right;
	word-break: break-word;
}

.qr-code {
	font-family: var(--vnet-font-display);
	letter-spacing: 1px;
	color: var(--vnet-warning);
}

.qr-countdown {
	font-size: 13px;
	color: var(--vnet-text-muted);
	text-align: center;
	margin-top: 8px;
}

.qr-result {
	display: flex;
	flex-direction: column;
	gap: 6px;
	padding: 20px 16px;
	margin-bottom: 12px;
	border-radius: 8px;
	text-align: center;
	background: var(--vnet-surface-2);
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.qr-result strong {
	font-size: 18px;
}

.qr-paid strong {
	color: var(--vnet-success);
}

.qr-expired strong {
	color: var(--vnet-warning);
}
</style>
