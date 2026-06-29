<template>
	<div class="service-menu">
		<header class="menu-header">
			<el-button text @click="$emit('back')">
				<el-icon><ArrowLeft /></el-icon>
				Quay lại
			</el-button>
			<h3>Đồ ăn &amp; Nước uống</h3>
			<el-badge :value="order.cartCount" :hidden="order.cartCount === 0">
				<el-button text circle @click="order.showCart = true">
					<el-icon><ShoppingCart /></el-icon>
				</el-button>
			</el-badge>
		</header>

		<div class="menu-categories">
			<el-radio-group v-model="activeCategory" @change="loadProducts">
				<el-radio-button value="">Tất cả</el-radio-button>
				<el-radio-button v-for="cat in categories" :key="cat.id" :value="cat.id">{{ cat.name }}</el-radio-button>
			</el-radio-group>
		</div>

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
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ArrowLeft, ShoppingCart } from '@element-plus/icons-vue'
import { useOrderStore } from '../stores/order.store'
import ProductCard from '../components/ProductCard.vue'
import CartPanel from '../components/CartPanel.vue'

declare const window: any
const api = () => window.go?.main?.App

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
	background: #f0f2f5;
}

.menu-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 12px 16px;
	background: #fff;
	box-shadow: 0 1px 4px rgba(0,0,0,.08);
}

.menu-header h3 {
	font-size: 16px;
	font-weight: 600;
}

.menu-categories {
	padding: 12px 16px;
	background: #fff;
	overflow-x: auto;
	white-space: nowrap;
}

.menu-grid {
	flex: 1;
	overflow-y: auto;
	padding: 16px;
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
	gap: 12px;
}
</style>
