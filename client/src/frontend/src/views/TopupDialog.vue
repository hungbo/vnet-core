<template>
	<!--
		Một thân, hai lớp vỏ. Mở từ thanh dock thì là hộp thoại; mở thành cửa sổ
		riêng thì el-dialog là sai — một hộp thoại nổi giữa cửa sổ trống, lại còn
		nút X của riêng nó bên cạnh nút X của hệ điều hành.

		Dùng <component :is> chứ không chép đôi khối markup: hai bản chép rồi sẽ
		lệch nhau, mà lệch ở đây là màn hình nạp tiền của khách.
	-->
	<component :is="standalone ? 'div' : 'el-dialog'" v-bind="voBoc" @close="$emit('close')">
		<h2 v-if="standalone" class="standalone-title">Nạp tiền</h2>
		<div class="balance-info" v-if="memberInfo">
			<div class="info-row">
				<span>Số dư hiện tại</span>
				<strong style="color: var(--vnet-success);">{{ formatCurrency(memberInfo.balance) }}</strong>
			</div>
			<div class="info-row" v-if="memberInfo.bonus_balance">
				<span>KM</span>
				<strong style="color: var(--vnet-warning);">{{ formatCurrency(memberInfo.bonus_balance) }}</strong>
			</div>
		</div>

		<div class="preset-grid">
			<button
				v-for="amount in presets"
				:key="amount"
				:class="['preset-btn', { active: selectedAmount === amount }]"
				@click="selectedAmount = amount"
			>
				{{ formatCurrency(amount) }}
			</button>
		</div>

		<div class="custom-input">
			<span class="custom-label">Hoặc nhập số tiền</span>
			<el-input-number
				v-model="customAmount"
				:min="1000"
				:step="10000"
				:precision="0"
				style="width: 100%;"
				controls-position="right"
			/>
		</div>

		<div class="total-display">
			<span>Nạp</span>
			<strong>{{ formatCurrency(finalAmount) }}</strong>
		</div>

		<template v-if="daGui">
			<div class="da-gui">Đã gửi yêu cầu nạp {{ formatCurrency(finalAmount) }}</div>
			<p class="hint">Nhân viên quầy sẽ xác nhận sau khi nhận tiền. Đóng cửa sổ này được rồi.</p>
		</template>
		<template v-else>
			<el-button
				type="warning"
				size="large"
				style="width: 100%; margin-top: 8px;"
				:loading="sending"
				@click="handleRequest"
			>
				Gửi yêu cầu nạp tiền
			</el-button>
			<p class="hint">Nhân viên quầy sẽ xác nhận sau khi nhận tiền</p>
		</template>
	</component>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'

declare const window: any
const api = () => window.go?.main?.App

const props = defineProps<{ standalone?: boolean }>()
const emit = defineEmits<{ close: [] }>()

const visible = ref(true)

// Thuộc tính của lớp vỏ, tách ra để thẻ <component> không phải mang theo cả
// modelValue lẫn title khi nó chỉ là một thẻ div.
const voBoc = computed(() =>
	props.standalone
		? { class: 'topup-standalone' }
		: { modelValue: visible.value, title: 'Nạp tiền', width: '380px', closeOnClickModal: false }
)
const presets = ref<number[]>([])
const selectedAmount = ref<number | null>(null)
const customAmount = ref(50000)
const sending = ref(false)
// Chỉ có ý nghĩa ở dạng cửa sổ riêng: hộp thoại gửi xong là đóng, còn cửa sổ thì
// nằm nguyên đó với cái nút vẫn bấm được — bấm hai lần là hai yêu cầu nạp tiền.
const daGui = ref(false)
const memberInfo = ref<any>(null)

const finalAmount = computed(() => selectedAmount.value || customAmount.value)

function formatCurrency(n: number) {
	return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(n || 0)
}

async function handleRequest() {
	if (!finalAmount.value || finalAmount.value < 1000) {
		ElMessage.warning('Chọn số tiền nạp')
		return
	}
	sending.value = true
	try {
		await api().RequestTopup(finalAmount.value)
		ElMessage.success('Yêu cầu nạp tiền đã gửi! Admin sẽ xác nhận sau.')
		daGui.value = true
		emit('close')
	} catch (e) {
		ElMessage.error(String(e))
	} finally {
		sending.value = false
	}
}

onMounted(async () => {
	try {
		const data = await api().GetMemberInfo()
		if (data) memberInfo.value = JSON.parse(data)
	} catch {}

	try {
		const data = await api().GetTopupPresets()
		if (data) presets.value = JSON.parse(data)
	} catch {
		presets.value = [5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000]
	}
})
</script>

<style scoped>
/* Dạng cửa sổ riêng: căn giữa một cột hẹp thay vì kéo giãn form ra hết bề
   ngang màn hình — mấy ô chọn mệnh giá mà dài 1300px thì không bấm nổi. */
.topup-standalone {
	min-height: 100vh;
	display: flex;
	flex-direction: column;
	justify-content: center;
	gap: 4px;
	max-width: 420px;
	margin: 0 auto;
	padding: 24px;
	background: var(--vnet-bg);
	color: var(--vnet-text);
}

.standalone-title {
	margin: 0 0 12px;
	font-size: 20px;
	font-weight: 600;
	text-align: center;
}

.da-gui {
	margin-top: 8px;
	padding: 14px;
	border-radius: 8px;
	text-align: center;
	font-weight: 600;
	color: var(--vnet-success);
	background: var(--vnet-surface-2);
}

.balance-info {
	background: #f8f9fa;
	border-radius: 8px;
	padding: 12px 16px;
	margin-bottom: 16px;
}

.info-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	font-size: 14px;
}

.preset-grid {
	display: grid;
	grid-template-columns: repeat(4, 1fr);
	gap: 8px;
	margin-bottom: 16px;
}

.preset-btn {
	padding: 10px 4px;
	border: 1px solid var(--vnet-border);
	border-radius: 8px;
	background: var(--vnet-surface);
	cursor: pointer;
	font-size: 12px;
	font-weight: 500;
	color: var(--vnet-text);
	transition: all .2s;
}

.preset-btn:hover {
	border-color: var(--vnet-primary);
	color: var(--vnet-primary);
}

.preset-btn.active {
	background: var(--vnet-primary);
	color: var(--vnet-surface);
	border-color: var(--vnet-primary);
}

.custom-input {
	margin-bottom: 16px;
}

.custom-label {
	display: block;
	font-size: 12px;
	color: var(--vnet-text-muted);
	margin-bottom: 6px;
}

.total-display {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 12px 0;
	font-size: 18px;
	border-top: 1px solid #f0f0f0;
}

.hint {
	font-size: 11px;
	color: var(--vnet-text-muted);
	text-align: center;
	margin-top: 8px;
}
</style>
