<template>
	<div class="overlay" @click.self="$emit('close')">
		<div class="panel">
			<header class="panel-head">
				<h3>Đánh giá dịch vụ</h3>
				<el-button text circle @click="$emit('close')">
					<el-icon><Close /></el-icon>
				</el-button>
			</header>

			<div class="panel-body">
				<div class="rate-row">
					<el-rate v-model="rating" :max="5" size="large" show-text :texts="RATE_TEXTS" />
				</div>

				<el-input
					v-model="content"
					type="textarea"
					:rows="4"
					maxlength="500"
					show-word-limit
					placeholder="Điều gì làm bạn hài lòng hoặc chưa hài lòng? (không bắt buộc)"
				/>

				<p class="hint">
					Đánh giá gắn với phiên chơi đang mở của bạn — quán không biết ai gửi nếu bạn không ghi tên.
				</p>

				<el-button
					type="primary"
					size="large"
					class="submit-btn"
					:loading="submitting"
					:disabled="rating < 1"
					@click="submit"
				>
					Gửi đánh giá
				</el-button>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { Close } from '@element-plus/icons-vue'

// SubmitFeedback đã có trong app.go nhưng phía Vue chưa có nút nào gọi tới.

declare const window: any
const api = () => window.go?.main?.App

const emit = defineEmits<{ close: [] }>()

const RATE_TEXTS = ['Rất tệ', 'Tệ', 'Bình thường', 'Tốt', 'Rất tốt']

const rating = ref(5)
const content = ref('')
const submitting = ref(false)

async function submit() {
	submitting.value = true
	try {
		// orderID để trống: đây là đánh giá cho cả phiên chơi, không cho một đơn.
		await api().SubmitFeedback(rating.value, content.value, '')
		ElNotification({
			type: 'success',
			title: 'Cảm ơn bạn',
			message: 'Đã gửi đánh giá tới quầy',
		})
		emit('close')
	} catch (e) {
		// Máy chủ chỉ nhận một đánh giá mỗi đơn/phiên — hiện nguyên văn lý do.
		ElMessage.error(String(e))
	} finally {
		submitting.value = false
	}
}
</script>

<style scoped>
.overlay {
	position: fixed;
	inset: 0;
	background: rgba(0, 0, 0, 0.55);
	display: flex;
	align-items: center;
	justify-content: center;
	z-index: 2000;
}

.panel {
	width: 460px;
	max-width: 92vw;
	background: var(--vnet-surface);
	border-radius: 12px;
	overflow: hidden;
}

.panel-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 16px 20px;
	border-bottom: 1px solid var(--vnet-border);
}

.panel-head h3 {
	font-size: 16px;
	font-weight: 600;
}

.panel-body {
	padding: 20px;
	display: flex;
	flex-direction: column;
	gap: 14px;
}

.rate-row {
	display: flex;
	justify-content: center;
	padding: 4px 0;
}

.hint {
	font-size: 12px;
	color: var(--vnet-text-muted);
}

.submit-btn {
	width: 100%;
}
</style>
