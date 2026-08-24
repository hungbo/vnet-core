<template>
	<div class="balance-section" v-if="memberInfo && role !== 'admin'">
		<div class="balance-card">
			<div class="balance-row">
				<span class="balance-label">Số dư</span>
				<span class="balance-value primary">{{ formatCurrency(memberInfo.balance) }}</span>
			</div>
			<div class="balance-row bonus">
				<span class="balance-label">KM</span>
				<span class="balance-value">{{ formatCurrency(memberInfo.bonus_balance) }}</span>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
defineProps<{
	role: string
	memberInfo: any
}>()

function formatCurrency(n: number) {
	return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(n || 0)
}
</script>

<style scoped>
.balance-section {
	/* Rộng bằng thanh, không cố định 320px: thanh chỉ 360px nên con số cứng đó
	   vừa tràn vừa lệch với lưới nút bên dưới. */
	width: 100%;
}

.balance-card {
	background: var(--vnet-surface);
	border-radius: 12px;
	padding: 16px 20px;
	box-shadow: var(--vnet-shadow);
}

.balance-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 6px 0;
}

.balance-row.bonus {
	border-top: 1px solid #f0f0f0;
	margin-top: 4px;
	padding-top: 10px;
}

.balance-label {
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.balance-value {
	font-weight: 600;
	font-size: 16px;
	color: var(--vnet-text);
}

.balance-value.primary {
	color: var(--vnet-success);
	font-size: 20px;
}
</style>
