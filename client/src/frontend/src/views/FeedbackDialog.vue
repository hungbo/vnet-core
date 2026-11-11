<template>
	<div class="overlay" :class="{ standalone }" @click.self="!standalone && $emit('close')">
		<div class="card">
			<el-button v-if="!standalone" class="card-close" text circle @click="$emit('close')">
				<el-icon><Close /></el-icon>
			</el-button>

			<header class="card-head">
				<span class="head-icon"><el-icon><ChatDotRound /></el-icon></span>
				<div>
					<h2>Đánh giá dịch vụ</h2>
					<p>Góp ý của bạn giúp quán phục vụ tốt hơn.</p>
				</div>
			</header>

			<!-- Gửi xong: lời cảm ơn thay chỗ biểu mẫu, rồi cửa sổ tự ẩn (submit). -->
			<div v-if="daGui" class="thanks">
				<el-icon><CircleCheckFilled /></el-icon>
				<h3>Cảm ơn bạn!</h3>
				<p>Quầy đã nhận đánh giá của bạn.</p>
			</div>

			<template v-else>
				<div class="rating" @mouseleave="dangRe = 0">
					<div class="stars" role="radiogroup" aria-label="Số sao">
						<button
							v-for="n in 5"
							:key="n"
							type="button"
							role="radio"
							class="star"
							:class="{ on: n <= saoHien }"
							:aria-checked="n === rating"
							:aria-label="`${n} sao — ${RATE_TEXTS[n - 1]}`"
							@mouseenter="dangRe = n"
							@click="rating = n"
						>
							<el-icon><StarFilled /></el-icon>
						</button>
					</div>
					<div class="rate-text" :class="mucDo">{{ RATE_TEXTS[saoHien - 1] }}</div>
				</div>

				<el-input
					v-model="content"
					class="note"
					type="textarea"
					:rows="5"
					maxlength="500"
					show-word-limit
					resize="none"
					placeholder="Điều gì làm bạn hài lòng hoặc chưa hài lòng? (không bắt buộc)"
				/>

				<p class="hint">
					Đánh giá gắn với phiên chơi đang mở của bạn — quán không biết ai gửi nếu bạn không ghi tên.
				</p>

				<button type="button" class="action" :disabled="submitting || rating < 1" @click="submit">
					<el-icon v-if="submitting" class="is-loading"><Loading /></el-icon>
					Gửi đánh giá
				</button>
			</template>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ChatDotRound, CircleCheckFilled, Close, Loading, StarFilled } from '@element-plus/icons-vue'
import { useBaoSanSang } from '../utils/panelReady'

// SubmitFeedback đã có trong app.go nhưng phía Vue chưa có nút nào gọi tới.

declare const window: any
const api = () => window.go?.main?.App

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ standalone?: boolean }>()

const RATE_TEXTS = ['Rất tệ', 'Tệ', 'Bình thường', 'Tốt', 'Rất tốt']

// Không có gì phải tải: dựng xong là sẵn sàng.
onMounted(useBaoSanSang())

const rating = ref(5)
// Sao đang rê chuột tới: xem trước mức đó, rời chuột thì về số đã chọn.
const dangRe = ref(0)
const saoHien = computed(() => dangRe.value || rating.value)
const mucDo = computed(() => (saoHien.value <= 2 ? 'te' : saoHien.value === 3 ? 'vua' : 'tot'))
const content = ref('')
const submitting = ref(false)
const daGui = ref(false)

async function submit() {
	submitting.value = true
	try {
		// orderID để trống: đây là đánh giá cho cả phiên chơi, không cho một đơn.
		await api().SubmitFeedback(rating.value, content.value, '')
		daGui.value = true
		// Để khách thấy lời cảm ơn trước đã, rồi mới ẩn cửa sổ.
		setTimeout(() => {
			if (props.standalone) {
				rating.value = 5
				content.value = ''
				daGui.value = false
				api()?.HidePanel?.()
			} else emit('close')
		}, 1500)
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

/* Dạng cửa sổ riêng (tiến trình --window): không lớp phủ, thẻ nằm giữa cửa sổ —
   cùng kiểu với cửa sổ Điểm danh. */
.overlay.standalone {
	position: static;
	min-height: 100vh;
	padding: 24px;
	background:
		radial-gradient(ellipse at top, rgba(123, 92, 255, 0.16), transparent 60%),
		var(--vnet-bg);
	color: var(--vnet-text);
}

.card {
	position: relative;
	width: 100%;
	max-width: 540px;
	display: flex;
	flex-direction: column;
	gap: 18px;
	padding: 28px;
	background: var(--vnet-surface);
	border: 1px solid var(--vnet-border);
	border-radius: 18px;
	box-shadow: var(--vnet-shadow);
}

.card-close {
	position: absolute;
	top: 12px;
	right: 12px;
}

.card-head {
	display: flex;
	align-items: center;
	gap: 14px;
}

.head-icon {
	flex: none;
	display: grid;
	place-items: center;
	width: 48px;
	height: 48px;
	border-radius: 14px;
	background: var(--vnet-brand);
	color: #fff;
	font-size: 24px;
}

.card-head h2 {
	font-size: 20px;
	font-weight: 700;
}

.card-head p {
	margin-top: 2px;
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.rating {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 8px;
	padding: 18px 0 14px;
	border-radius: 14px;
	background: var(--vnet-surface-2);
}

.stars {
	display: flex;
	gap: 6px;
}

.star {
	display: grid;
	place-items: center;
	width: 56px;
	height: 56px;
	padding: 0;
	border: 0;
	border-radius: 12px;
	background: transparent;
	color: var(--vnet-text-faint);
	font-size: 40px;
	cursor: pointer;
	transition: color .12s, transform .12s;
}

.star:hover {
	transform: scale(1.12);
}

.star.on {
	color: var(--vnet-warning);
}

.rate-text {
	font-family: var(--vnet-font-display);
	font-size: 18px;
	font-weight: 700;
}

.rate-text.te { color: var(--vnet-danger); }
.rate-text.vua { color: var(--vnet-text-muted); }
.rate-text.tot { color: var(--vnet-success); }

.note :deep(.el-textarea__inner) {
	padding: 12px 14px;
	border-radius: 12px;
	background: var(--vnet-surface-2);
	font-size: 14px;
	line-height: 1.5;
}

.note :deep(.el-input__count) {
	background: transparent;
}

.hint {
	margin-top: -6px;
	font-size: 12px;
	color: var(--vnet-text-faint);
}

/* Cùng kiểu nút chính "Nạp tiền" ở thanh điều khiển. */
.action {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 8px;
	height: 52px;
	border: 0;
	border-radius: 12px;
	background: var(--vnet-brand);
	color: #fff;
	font-family: inherit;
	font-size: 16px;
	font-weight: 600;
	cursor: pointer;
	transition: filter .15s, transform .15s;
}

.action:hover:not(:disabled) {
	filter: brightness(1.1);
	transform: translateY(-1px);
}

.action:disabled {
	cursor: default;
	background: var(--vnet-surface-2);
	color: var(--vnet-text-faint);
}

.thanks {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 6px;
	padding: 28px 0 20px;
	text-align: center;
}

.thanks .el-icon {
	font-size: 56px;
	color: var(--vnet-success);
}

.thanks h3 {
	margin-top: 6px;
	font-size: 20px;
	font-weight: 700;
}

.thanks p {
	font-size: 14px;
	color: var(--vnet-text-muted);
}
</style>
