<template>
	<el-drawer v-model="store.showCart" title="Giỏ hàng" size="320px">
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
		<el-button type="primary" size="large" style="width:100%;margin-top:12px" :loading="store.ordering" @click="store.placeOrder">
			Gọi món
		</el-button>
	</el-drawer>
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
.cart-empty {
	text-align: center;
	color: #909399;
	padding: 40px 0;
}

.cart-balance {
	display: flex;
	justify-content: space-between;
	padding: 12px 0;
	border-bottom: 1px solid #f0f0f0;
	margin-bottom: 8px;
}

.cart-balance-label {
	font-size: 14px;
	color: #606266;
}

.cart-balance-value {
	font-size: 14px;
	font-weight: 600;
	color: #67c23a;
}

.cart-item {
	padding: 12px 0;
	border-bottom: 1px solid #f0f0f0;
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
	color: #e6a23c;
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
	color: #e6a23c;
}
</style>
