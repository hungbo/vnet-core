<template>
	<div class="balance-section" v-if="memberInfo && role !== 'admin'">
		<div class="balance-card">
			<div class="balance-cell">
				<span class="balance-label">Số dư</span>
				<span class="balance-value primary">{{ formatCurrency(memberInfo.balance) }}</span>
			</div>
			<div class="balance-cell">
				<span class="balance-label">Khuyến mãi</span>
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
	width: 100%;
}

/* Hai số cạnh nhau thay vì hai hàng: đọc một lượt là biết còn bao nhiêu. */
.balance-card {
	display: grid;
	grid-template-columns: 1fr 1fr;
	border: 1px solid var(--vnet-border);
	border-radius: var(--vnet-radius);
	background: var(--vnet-surface);
}

.balance-cell {
	display: flex;
	flex-direction: column;
	gap: 2px;
	padding: 12px 14px;
	min-width: 0;
}

.balance-cell + .balance-cell {
	border-left: 1px solid var(--vnet-border);
}

.balance-label {
	font-size: 12px;
	color: var(--vnet-text-muted);
}

.balance-value {
	font-family: var(--vnet-font-display);
	font-weight: 600;
	font-size: 18px;
	color: var(--vnet-text);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.balance-value.primary {
	color: var(--vnet-success);
}
</style>
