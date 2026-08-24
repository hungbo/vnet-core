<template>
	<button class="product-card" :class="[color]" @click="$emit('select', product)">
		<!--
			Bản cũ chỉ hiện product.icon, mà bảng products không có cột icon nào —
			ô ảnh vì thế luôn rỗng ở mọi món. Ưu tiên ảnh thật, còn icon giữ lại làm
			đường lui cho những chỗ tự dựng danh sách món (ví dụ mệnh giá nạp tiền).
		-->
		<img
			v-if="product.image_url && !broken"
			class="product-image"
			:src="product.image_url"
			:alt="product.name"
			loading="lazy"
			@error="broken = true"
		>
		<div v-if="!product.image_url || broken" class="product-icon">{{ product.icon || '🍽️' }}</div>
		<div class="product-name">{{ product.name }}</div>
		<div class="product-price" v-if="product.price">{{ formatCurrency(product.price) }}</div>
	</button>
</template>

<script setup lang="ts">
import { ref } from 'vue'

// Ảnh hỏng thì rơi về icon chứ không để lại một ô trắng: máy trạm hay chạy khi
// máy chủ vừa đổi địa chỉ, và lúc đó mọi ảnh đều 404.
const broken = ref(false)

defineProps<{
	product: any
	color?: string
}>()

const emit = defineEmits<{
	select: [product: any]
}>()

function formatCurrency(n: number) {
	return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(n || 0)
}
</script>

<style scoped>
.product-card {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 6px;
	padding: 20px 12px;
	border: 1px solid var(--vnet-border);
	border-radius: 12px;
	cursor: pointer;
	background: var(--vnet-surface);
	transition: all .2s;
	font-family: inherit;
}

.product-card:hover {
	transform: translateY(-2px);
	box-shadow: 0 4px 16px rgba(0,0,0,.1);
	border-color: var(--vnet-primary);
}

.product-card.topup { color: var(--vnet-warning); }
.product-card.food { color: var(--vnet-success); }
.product-card.drink { color: var(--vnet-primary); }
.product-card.snack { color: var(--vnet-text-muted); }

.product-icon {
	font-size: 28px;
	line-height: 1;
	/* Bằng đúng khung ảnh, nếu không thì hàng nào rơi về icon sẽ thấp hơn hẳn
	   hàng có ảnh và cả lưới bị so le. */
	height: 72px;
	display: flex;
	align-items: center;
}

.product-image {
	width: 72px;
	height: 72px;
	object-fit: cover;
	border-radius: 10px;
}

.product-name {
	font-size: 13px;
	font-weight: 500;
	color: var(--vnet-text);
}

.product-price {
	font-size: 12px;
	color: var(--vnet-warning);
	font-weight: 600;
}
</style>
