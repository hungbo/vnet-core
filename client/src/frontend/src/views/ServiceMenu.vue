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

		<!-- Khách cần xem lại món đã gọi và quầy đã duyệt chưa, ngay trong cửa sổ gọi món. -->
		<div class="menu-tabs">
			<el-radio-group v-model="tab">
				<el-radio-button value="menu">Thực đơn</el-radio-button>
				<el-radio-button value="orders">Đơn đã gọi</el-radio-button>
			</el-radio-group>
			<!--
				Mặc định chỉ món còn hàng: khách khỏi lướt qua cả dãy món mờ không bấm được.
				"Tất cả" hiện thêm món hết hàng, để khách biết quán có bán, chỉ là tạm hết.
			-->
			<el-radio-group v-show="tab === 'menu'" v-model="locKho">
				<el-radio-button value="con-hang">Còn hàng</el-radio-button>
				<el-radio-button value="tat-ca">Tất cả</el-radio-button>
			</el-radio-group>
		</div>

		<MyOrders v-if="tab === 'orders'" />

		<div v-show="tab === 'menu'" class="menu-categories">
			<el-radio-group v-model="activeCategory" @change="loadProducts">
				<el-radio-button value="">Tất cả</el-radio-button>
				<el-radio-button v-for="cat in categories" :key="cat.id" :value="cat.id">{{ cat.name }}</el-radio-button>
			</el-radio-group>
		</div>

		<!-- Lưới món và giỏ hàng nằm cạnh nhau: chọn món và nhìn giỏ là một việc. -->
		<div v-show="tab === 'menu'" class="menu-content">
			<main class="menu-grid">
				<ProductCard
					v-for="p in monHienThi" :key="p.id"
					:product="p"
					:color="p.color || 'food'"
					@select="order.addToCart"
				/>
				<p v-if="products.length && !monHienThi.length" class="menu-trong">
					Mục này đang hết hàng — chọn "Tất cả" để xem các món.
				</p>
			</main>

			<CartPanel />
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onActivated } from 'vue'
import { ArrowLeft } from '@element-plus/icons-vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useOrderStore } from '../stores/order.store'
import { hetHang } from '../utils/stock'
import { useBaoSanSang } from '../utils/panelReady'
import ProductCard from '../components/ProductCard.vue'
import CartPanel from '../components/CartPanel.vue'
import MyOrders from '../components/MyOrders.vue'

declare const window: any
const api = () => window.go?.main?.App

defineProps<{ standalone?: boolean }>()
const emit = defineEmits<{ back: [] }>()

const order = useOrderStore()
const categories = ref<any[]>([])
const products = ref<any[]>([])
const activeCategory = ref('')
const tab = ref<'menu' | 'orders'>('menu')
const locKho = ref<'con-hang' | 'tat-ca'>('con-hang')
const monHienThi = computed(() =>
	locKho.value === 'tat-ca' ? products.value : products.value.filter(p => !hetHang(p)))

async function loadProducts() {
	try {
		const data = await api().GetMenu(activeCategory.value)
		if (data) products.value = JSON.parse(data)
	} catch { products.value = [] }
}

// Màn này được giữ nguyên giữa các lần mở (KeepAlive ở App.vue): lần đầu báo sẵn sàng khi
// thực đơn tải xong, các lần sau báo ngay lúc được bật lại.
const baoSanSang = useBaoSanSang()
let daTai = false
onActivated(() => { if (daTai) baoSanSang() })

onMounted(async () => {
	try {
		const data = await api().GetCategories()
		if (data) categories.value = JSON.parse(data)
	} catch {}
	await loadProducts()
	daTai = true
	baoSanSang()

	// Máy khác trong quán đặt món cuối, hoặc nhân viên nhập kho — thực đơn phải
	// đổi theo mà không cần khách bấm gì.
	//
	// Không nạp lại ngay sau khi khách đặt: kho bị trừ lúc nhân viên XÁC NHẬN
	// đơn, không phải lúc đặt, nên nạp lúc đó chỉ đọc lại đúng con số cũ. Sự
	// kiện dưới đây mới về đúng thời điểm.
	//
	// Nạp lại cả danh sách chứ không vá từng món theo id trong gói tin: tồn kho
	// mà máy chủ trả về là giá trị suy diễn từ nguyên liệu, nên trừ một nguyên
	// liệu làm đổi số suất của nhiều món khác nhau. Hỏi lại thì luôn đúng.
	EventsOn('vnet:stock:changed', () => loadProducts())
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

.menu-tabs {
	display: flex;
	justify-content: space-between;
	flex-wrap: wrap;
	gap: 8px;
	padding: 12px 16px 0;
	background: var(--vnet-surface);
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
	/* Ô rộng hơn bản cũ (140px) để ảnh món to, nhìn rõ. */
	grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
	gap: 12px;
	/* Lưới là khối co giãn cao hết cửa sổ, nên hàng duy nhất giãn theo và mỗi ô
	   cao bằng cả màn hình. align-content: start giữ ô cao đúng bằng nội dung. */
	align-content: start;
}

.menu-trong {
	grid-column: 1 / -1;
	padding: 32px 0;
	text-align: center;
	color: var(--vnet-text-muted);
}
</style>
