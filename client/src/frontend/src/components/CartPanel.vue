<template>
	<!--
		Cột thường, KHÔNG phải el-drawer. Ngăn kéo nghĩa là khách phải bấm vào
		biểu tượng giỏ mới thấy mình đã chọn gì và hết bao nhiêu tiền — mà đó
		đúng là hai thứ người ta muốn nhìn trong lúc chọn món.
	-->
	<aside class="cart-panel">
		<h4 class="cart-title">Giỏ hàng<span v-if="store.cartCount" class="cart-count">{{ store.cartCount }}</span></h4>

		<div class="cart-balance" v-if="balance !== null">
			<span class="cart-balance-label">Số dư:</span>
			<span class="cart-balance-value">{{ formatCurrency(balance) }}</span>
		</div>

		<div v-if="store.cart.length === 0" class="cart-empty">Giỏ hàng trống</div>
		<div v-for="(item, idx) in store.cart" :key="item.id" class="cart-item">
			<div class="cart-item-info">
				<div class="cart-item-name">{{ item.name }}</div>
				<div class="cart-item-price">{{ formatCurrency(item.price * item.qty) }}</div>
			</div>
			<div class="cart-item-actions">
				<el-button circle size="small" @click="store.updateQty(item.id, -1)" :disabled="item.qty <= 1">-</el-button>
				<span class="cart-qty">{{ item.qty }}</span>
				<el-button circle size="small" @click="store.updateQty(item.id, 1)">+</el-button>
				<el-button text type="danger" size="small" @click="store.removeFromCart(idx)">Xoá</el-button>
			</div>
		</div>
		<div class="cart-total">
			<span>Tổng cộng:</span>
			<span class="cart-total-value">{{ formatCurrency(store.cartTotal) }}</span>
		</div>
		<el-button
			type="primary"
			size="large"
			style="width:100%;margin-top:12px"
			:loading="store.ordering"
			:disabled="store.cart.length === 0"
			@click="store.placeOrder"
		>
			Gọi món
		</el-button>
	</aside>
</template>

<script setup lang="ts">
import { useOrderStore } from '../stores/order.store'
import { useSessionStore } from '../stores/session.store'

const store = useOrderStore()
const session = useSessionStore()

const balance = session.memberInfo?.balance ?? null

function formatCurrency(n: number) {
	return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(n || 0)
}
</script>

<style scoped>
.cart-panel {
	width: 320px;
	flex-shrink: 0;
	display: flex;
	flex-direction: column;
	padding: 16px;
	background: var(--vnet-surface);
	border-left: 1px solid var(--vnet-border);
	overflow-y: auto;
}

.cart-title {
	font-size: 15px;
	font-weight: 600;
	color: var(--vnet-text);
	margin-bottom: 8px;
}

.cart-count {
	margin-left: 8px;
	padding: 1px 8px;
	border-radius: 10px;
	font-size: 12px;
	background: var(--vnet-primary);
	color: #fff;
}

.cart-empty {
	text-align: center;
	color: var(--vnet-text-muted);
	padding: 40px 0;
}

.cart-balance {
	display: flex;
	justify-content: space-between;
	padding: 12px 0;
	border-bottom: 1px solid var(--vnet-border);
	margin-bottom: 8px;
}

.cart-balance-label {
	font-size: 14px;
	color: var(--vnet-text-muted);
}

.cart-balance-value {
	font-size: 14px;
	font-weight: 600;
	color: var(--vnet-success);
}

.cart-item {
	padding: 12px 0;
	border-bottom: 1px solid var(--vnet-border);
}

.cart-item-info {
	display: flex;
	justify-content: space-between;
	margin-bottom: 8px;
}

.cart-item-name {
	font-size: 14px;
	font-weight: 500;
}

.cart-item-price {
	font-size: 14px;
	color: var(--vnet-warning);
	font-weight: 600;
}

.cart-item-actions {
	display: flex;
	align-items: center;
	gap: 8px;
}

.cart-qty {
	font-size: 14px;
	font-weight: 500;
	min-width: 20px;
	text-align: center;
}

.cart-total {
	display: flex;
	justify-content: space-between;
	padding: 16px 0;
	font-size: 16px;
	font-weight: 600;
}

.cart-total-value {
	color: var(--vnet-warning);
}
</style>
