<template>
	<button
		class="product-card"
		:class="[color, { 'het-hang': hetHang }]"
		:disabled="hetHang"
		@click="$emit('select', product)"
	>
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
		<div v-if="hetHang" class="nhan-het">Hết hàng</div>
		<div v-else-if="theoDoiKho" class="con-lai">còn {{ soConLai }}</div>
	</button>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

// Ảnh hỏng thì rơi về icon chứ không để lại một ô trắng: máy trạm hay chạy khi
// máy chủ vừa đổi địa chỉ, và lúc đó mọi ảnh đều 404.
const broken = ref(false)

const props = defineProps<{
	product: any
	color?: string
}>()

// CHỈ xét tồn kho khi has_stock bật.
//
// Món nấu từ nguyên liệu mà không bật cờ này luôn được máy chủ trả về
// current_stock = 0 (xem loadProductStock bên backend). Lấy số 0 đó ra mà khoá
// là khoá nhầm gần hết thực đơn trong khi kho vẫn đầy.
const theoDoiKho = computed(() => props.product?.has_stock === true)
const soConLai = computed(() => Math.floor(Number(props.product?.current_stock ?? 0)))
const hetHang = computed(() => theoDoiKho.value && soConLai.value <= 0)

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

/* Hết hàng vẫn hiện chứ không ẩn: khách cần biết quán có bán món đó, chỉ là
   hôm nay tạm hết. Ẩn đi thì món biến mất rồi hiện lại luân phiên, trông như
   lỗi giao diện. */
.product-card.het-hang {
	opacity: .45;
	cursor: not-allowed;
}

.product-card.het-hang:hover {
	transform: none;
	box-shadow: none;
	border-color: var(--vnet-border);
}

.nhan-het {
	font-size: 11px;
	font-weight: 600;
	color: var(--vnet-danger, #f56c6c);
	border: 1px solid currentColor;
	border-radius: 999px;
	padding: 1px 8px;
}

.con-lai {
	font-size: 11px;
	color: var(--vnet-text-muted);
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
