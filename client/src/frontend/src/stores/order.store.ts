import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'

declare const window: any
const api = () => window.go?.main?.App

export const useOrderStore = defineStore('order', () => {
	const cart = ref<{ id: string; name: string; price: number; qty: number }[]>([])
	const ordering = ref(false)
	const showCart = ref(false)

	const cartTotal = computed(() => cart.value.reduce((sum, i) => sum + i.price * i.qty, 0))
	const cartCount = computed(() => cart.value.reduce((sum, i) => sum + i.qty, 0))

	function addToCart(p: { id: string; name: string; price: number }) {
		const existing = cart.value.find(i => i.id === p.id)
		if (existing) {
			existing.qty++
		} else {
			cart.value.push({ id: p.id, name: p.name, price: p.price, qty: 1 })
		}
		ElMessage.success({ message: `Đã thêm ${p.name}`, duration: 1000 })
	}

	function removeFromCart(index: number) {
		cart.value.splice(index, 1)
	}

	function updateQty(id: string, delta: number) {
		const item = cart.value.find(i => i.id === id)
		if (item) {
			item.qty = Math.max(1, item.qty + delta)
		}
	}

	function clearCart() {
		cart.value = []
	}

	async function placeOrder() {
		if (!cart.value.length) return
		ordering.value = true
		try {
			const items = cart.value.map(i => ({ product_id: i.id, quantity: i.qty }))
			await api().PlaceOrder(JSON.stringify(items))
			ElMessage.success('Đã gửi đơn hàng!')
			cart.value = []
			showCart.value = false
		} catch (e) {
			ElMessage.error(String(e))
		} finally {
			ordering.value = false
		}
	}

	return { cart, ordering, showCart, cartTotal, cartCount, addToCart, removeFromCart, updateQty, clearCart, placeOrder }
})
