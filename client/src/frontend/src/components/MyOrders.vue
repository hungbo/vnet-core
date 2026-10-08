<template>
	<div class="my-orders">
		<div class="orders-toolbar">
			<span class="orders-hint">Đơn mới nhất ở trên. Trạng thái đổi khi quầy duyệt đơn.</span>
			<el-button size="small" :loading="loading" @click="load">Làm mới</el-button>
		</div>

		<div v-if="!loading && !orders.length" class="orders-empty">Bạn chưa gọi đơn nào.</div>

		<article v-for="o in orders" :key="o.id" class="order-card">
			<header class="order-head">
				<span class="order-code">{{ o.order_code }}</span>
				<span class="order-time">{{ formatTime(o.created_at) }}</span>
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
		</article>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useOrderStore } from '../stores/order.store'

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

onMounted(load)
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
</style>
