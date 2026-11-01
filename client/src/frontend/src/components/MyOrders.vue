<template>
	<!--
		Đang trả một đơn bằng QR thì màn mã thay chỗ danh sách: khách đang cầm điện
		thoại quét, không cần thấy các đơn khác.
	-->
	<div v-if="dangTra" class="my-orders">
		<div class="pay-column">
			<h4 class="pay-title">Thanh toán đơn {{ dangTra.order.order_code }}</h4>
			<QrPayPanel
				:key="dangTra.qr.request_id"
				:qr="dangTra.qr"
				paid-title="Đã thanh toán"
				paid-text="Đơn đã được thanh toán. Cảm ơn bạn!"
				cancel-text="Huỷ, quay lại danh sách"
				@paid="load"
				@close="veDanhSach"
				@retry="traQR(dangTra.order)"
				@cancelled="veDanhSach"
			/>
		</div>
	</div>

	<div v-else class="my-orders">
		<div class="orders-toolbar">
			<span class="orders-hint">
				Đơn mới nhất ở trên. Trạng thái đổi khi quầy duyệt đơn.
				<template v-if="qrReady">Đơn quầy đã xác nhận thì thanh toán được bằng QR ngay tại máy.</template>
			</span>
			<el-button size="small" :loading="loading" @click="load">Làm mới</el-button>
		</div>

		<div v-if="!loading && !orders.length" class="orders-empty">Bạn chưa gọi đơn nào.</div>

		<article v-for="o in orders" :key="o.id" :class="['order-card', { unpaid: chuaTra(o) }]">
			<header class="order-head">
				<span class="order-code">{{ o.order_code }}</span>
				<span class="order-time">{{ formatTime(o.created_at) }}</span>
				<el-tag v-if="chuaTra(o)" type="warning" size="small" effect="dark">Chưa thanh toán</el-tag>
				<el-tag :type="statusOf(o.status).type" size="small" effect="dark">{{ statusOf(o.status).label }}</el-tag>
			</header>
			<ul v-if="o.items?.length" class="order-items">
				<li v-for="it in o.items" :key="it.id">
					<span>{{ it.quantity }}× {{ it.product_name }}</span>
					<span class="item-price">{{ money(it.subtotal) }}</span>
				</li>
			</ul>
			<div v-else-if="o.order_type === 'topup'" class="order-items">Yêu cầu nạp tiền</div>
			<footer class="order-total">
				<span>Tổng</span>
				<strong>{{ money(o.final_amount) }}</strong>
			</footer>
			<!--
				Trên máy chỉ trả được bằng QR: trừ số dư thì đụng tiền giờ chơi, nên vẫn
				là việc của quầy. Điện thoại báo tiền ở quầy mất kết nối thì không có nút,
				vì chuyển tiền xong sẽ không ai xác nhận.
			-->
			<div v-if="chuaTra(o)" class="order-pay">
				<el-button v-if="qrReady" type="primary" :loading="dangTao === o.id" @click="traQR(o)">
					Thanh toán bằng QR
				</el-button>
				<span v-else class="orders-hint">Thanh toán tại quầy</span>
			</div>
		</article>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useOrderStore } from '../stores/order.store'
import QrPayPanel from './QrPayPanel.vue'

declare const window: any
const api = () => window.go?.main?.App

const orders = ref<any[]>([])
const loading = ref(false)
const order = useOrderStore()

const STATUS: Record<string, { label: string; type: 'info' | 'primary' | 'success' | 'danger' }> = {
	pending: { label: 'Chờ duyệt', type: 'info' },
	confirmed: { label: 'Đã xác nhận', type: 'primary' },
	completed: { label: 'Hoàn thành', type: 'success' },
	cancelled: { label: 'Đã huỷ', type: 'danger' },
}
function statusOf(s: string) {
	return STATUS[s] || { label: s, type: 'info' as const }
}

function money(v: number) {
	return `${new Intl.NumberFormat('vi-VN').format(v || 0)}đ`
}

function formatTime(iso: string) {
	const d = new Date(iso)
	const p = (n: number) => String(n).padStart(2, '0')
	return `${p(d.getHours())}:${p(d.getMinutes())} ${p(d.getDate())}/${p(d.getMonth() + 1)}`
}

// Quầy đã xác nhận (đã làm món) mà chưa thanh toán: trả xong thì đơn thành "completed".
function chuaTra(o: any) {
	return o.status === 'confirmed' && o.order_type !== 'topup'
}

const qrReady = ref(false)
// Đơn đang trả bằng QR và mã của nó; null là đang xem danh sách.
const dangTra = ref<{ order: any; qr: any } | null>(null)
const dangTao = ref('')

async function traQR(o: any) {
	dangTao.value = o.id
	try {
		dangTra.value = { order: o, qr: JSON.parse(await api().PayOrderQR(o.id)) }
	} catch (e) {
		ElMessage.error(String(e).replace(/^Error:\s*/, ''))
		dangTra.value = null
		load()
	} finally {
		dangTao.value = ''
	}
}

function veDanhSach() {
	dangTra.value = null
	load()
}

async function load() {
	loading.value = true
	try {
		const data = await api().GetMyOrders()
		orders.value = data ? JSON.parse(data) : []
	} catch {
		orders.value = []
	} finally {
		loading.value = false
	}
}

// Vừa gửi đơn xong thì đơn đó phải có ngay trong danh sách.
watch(() => order.ordering, (dang, truoc) => {
	if (truoc && !dang) load()
})

// Quầy xác nhận / thu tiền / huỷ một đơn của khách thì danh sách tự đổi, khách không
// phải bấm Làm mới mới thấy nút trả bằng QR.
let huyNghe: (() => void) | undefined
onUnmounted(() => huyNghe?.())

onMounted(() => {
	huyNghe = EventsOn('vnet:order:updated', () => load())
	load()
	api()?.GetQRStatus?.()
		.then((s: string) => { qrReady.value = !!JSON.parse(s)?.ready })
		.catch(() => { /* máy chủ cũ chưa có QR: trả tại quầy */ })
})
defineExpose({ load })
</script>

<style scoped>
.my-orders {
	flex: 1;
	overflow-y: auto;
	padding: 16px;
	display: flex;
	flex-direction: column;
	gap: 12px;
}

.orders-toolbar {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
}

.orders-hint {
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.orders-empty {
	text-align: center;
	color: var(--vnet-text-muted);
	padding: 60px 0;
}

.order-card {
	background: var(--vnet-surface);
	border: 1px solid var(--vnet-border);
	border-radius: var(--vnet-radius);
	padding: 12px 16px;
}

.order-head {
	display: flex;
	align-items: center;
	gap: 12px;
}

.order-code {
	font-weight: 600;
	color: var(--vnet-text);
}

.order-time {
	flex: 1;
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.order-items {
	list-style: none;
	margin: 10px 0 0;
	padding: 0;
	font-size: 14px;
	color: var(--vnet-text);
}

.order-items li {
	display: flex;
	justify-content: space-between;
	padding: 3px 0;
}

.item-price {
	color: var(--vnet-text-muted);
}

.order-total {
	display: flex;
	justify-content: space-between;
	margin-top: 10px;
	padding-top: 10px;
	border-top: 1px solid var(--vnet-border);
	font-size: 14px;
	color: var(--vnet-text-muted);
}

.order-total strong {
	color: var(--vnet-warning);
}

.order-card.unpaid {
	border-left: 3px solid var(--vnet-warning);
}

.order-pay {
	display: flex;
	justify-content: flex-end;
	margin-top: 10px;
}

.pay-column {
	width: 100%;
	max-width: 420px;
	margin: 0 auto;
}

.pay-title {
	font-size: 16px;
	font-weight: 600;
	text-align: center;
	margin-bottom: 12px;
}
</style>
