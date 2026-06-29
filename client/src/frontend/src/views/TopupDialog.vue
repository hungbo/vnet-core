<template>
	<el-dialog v-model="visible" title="Nạp tiền" width="380px" :close-on-click-modal="false" @close="$emit('close')">
		<div class="balance-info" v-if="memberInfo">
			<div class="info-row">
				<span>Số dư hiện tại</span>
				<strong style="color: #67c23a;">{{ formatCurrency(memberInfo.balance) }}</strong>
			</div>
			<div class="info-row" v-if="memberInfo.bonus_balance">
				<span>KM</span>
				<strong style="color: #e6a23c;">{{ formatCurrency(memberInfo.bonus_balance) }}</strong>
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
	</el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'

declare const window: any
const api = () => window.go?.main?.App

const emit = defineEmits<{ close: [] }>()

const visible = ref(true)
const presets = ref<number[]>([])
const selectedAmount = ref<number | null>(null)
const customAmount = ref(50000)
const sending = ref(false)
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
	border: 1px solid #dcdfe6;
	border-radius: 8px;
	background: #fff;
	cursor: pointer;
	font-size: 12px;
	font-weight: 500;
	color: #303133;
	transition: all .2s;
}

.preset-btn:hover {
	border-color: #409eff;
	color: #409eff;
}

.preset-btn.active {
	background: #409eff;
	color: #fff;
	border-color: #409eff;
}

.custom-input {
	margin-bottom: 16px;
}

.custom-label {
	display: block;
	font-size: 12px;
	color: #909399;
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
	color: #909399;
	text-align: center;
	margin-top: 8px;
}
</style>
