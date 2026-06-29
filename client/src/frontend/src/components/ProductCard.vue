<template>
	<button class="product-card" :class="[color]" @click="$emit('select', product)">
		<div class="product-icon">{{ product.icon }}</div>
		<div class="product-name">{{ product.name }}</div>
		<div class="product-price" v-if="product.price">{{ formatCurrency(product.price) }}</div>
	</button>
</template>

<script setup lang="ts">
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
	border: 1px solid #ebeef5;
	border-radius: 12px;
	cursor: pointer;
	background: #fff;
	transition: all .2s;
	font-family: inherit;
}

.product-card:hover {
	transform: translateY(-2px);
	box-shadow: 0 4px 16px rgba(0,0,0,.1);
	border-color: #409eff;
}

.product-card.topup { color: #e6a23c; }
.product-card.food { color: #67c23a; }
.product-card.drink { color: #409eff; }
.product-card.snack { color: #909399; }

.product-icon {
	font-size: 28px;
	line-height: 1;
}

.product-name {
	font-size: 13px;
	font-weight: 500;
	color: #303133;
}

.product-price {
	font-size: 12px;
	color: #e6a23c;
	font-weight: 600;
}
</style>
