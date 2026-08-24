<template>
	<div class="service-menu">
		<header class="menu-header">
			<!-- Cửa sổ riêng thì không có chỗ nào để "quay lại" — đóng cửa sổ là xong. -->
			<el-button v-if="!standalone" text @click="$emit('back')">
				<el-icon><ArrowLeft /></el-icon>
				Quay lại
			</el-button>
			<span v-else class="header-spacer" />
			<h3>Đồ ăn &amp; Nước uống</h3>
			<span class="header-spacer" />
		</header>

		<div class="menu-categories">
			<el-radio-group v-model="activeCategory" @change="loadProducts">
				<el-radio-button value="">Tất cả</el-radio-button>
				<el-radio-button v-for="cat in categories" :key="cat.id" :value="cat.id">{{ cat.name }}</el-radio-button>
			</el-radio-group>
		</div>

		<!-- Lưới món và giỏ hàng nằm cạnh nhau: chọn món và nhìn giỏ là một việc. -->
		<div class="menu-content">
			<main class="menu-grid">
				<ProductCard
					v-for="p in products" :key="p.id"
					:product="p"
					:color="p.color || 'food'"
					@select="order.addToCart"
				/>
			</main>

			<CartPanel />
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ArrowLeft } from '@element-plus/icons-vue'
import { useOrderStore } from '../stores/order.store'
import ProductCard from '../components/ProductCard.vue'
import CartPanel from '../components/CartPanel.vue'

declare const window: any
const api = () => window.go?.main?.App

defineProps<{ standalone?: boolean }>()
const emit = defineEmits<{ back: [] }>()

const order = useOrderStore()
const categories = ref<any[]>([])
const products = ref<any[]>([])
const activeCategory = ref('')

async function loadProducts() {
	try {
		const data = await api().GetMenu(activeCategory.value)
		if (data) products.value = JSON.parse(data)
	} catch { products.value = [] }
}

onMounted(async () => {
	try {
		const data = await api().GetCategories()
		if (data) categories.value = JSON.parse(data)
	} catch {}
	loadProducts()
})
</script>

<style scoped>
.service-menu {
	height: 100vh;
	display: flex;
	flex-direction: column;
	background: var(--vnet-bg);
}

.menu-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 12px 16px;
	background: var(--vnet-surface);
	box-shadow: 0 1px 4px rgba(0,0,0,.08);
}

.menu-header h3 {
	font-size: 16px;
	font-weight: 600;
}

.menu-categories {
	padding: 12px 16px;
	background: var(--vnet-surface);
	overflow-x: auto;
	white-space: nowrap;
}

.menu-content {
	flex: 1;
	display: flex;
	min-height: 0;
}

.menu-grid {
	flex: 1;
	overflow-y: auto;
	padding: 16px;
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
	gap: 12px;
	/* Lưới là khối co giãn cao hết cửa sổ, nên hàng duy nhất giãn theo và mỗi ô
	   cao bằng cả màn hình. align-content: start giữ ô cao đúng bằng nội dung. */
	align-content: start;
}
</style>
