<template>
	<div class="notice" role="alertdialog" aria-labelledby="notice-title" @keydown.esc="dismiss">
		<div class="notice-bar" aria-hidden="true" />

		<header class="notice-head">
			<span class="notice-icon" aria-hidden="true">
				<el-icon :size="22"><BellFilled /></el-icon>
			</span>
			<div class="notice-heading">
				<h2 id="notice-title">Thông báo từ quầy</h2>
				<span v-if="items.length > 1" class="notice-count">{{ items.length }} thông báo mới</span>
			</div>
		</header>

		<main class="notice-list">
			<article v-for="n in items" :key="n.at" class="notice-item">
				<h3 v-if="hasTitle(n)">{{ n.title }}</h3>
				<p class="notice-body">{{ n.body }}</p>
				<time class="notice-time">{{ formatTime(n.at) }}</time>
			</article>
		</main>

		<footer class="notice-foot">
			<button ref="okBtn" class="notice-ok" type="button" @click="dismiss">Đã hiểu</button>
		</footer>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'
import { BellFilled } from '@element-plus/icons-vue'
import { phatAm } from '../utils/sound'

// Thông báo quầy gửi tới máy (lệnh remote:message), hiện thay hộp xám của Windows.
// Cửa sổ này là một chế độ của cửa sổ phụ: thanh chính gõ cửa kèm nội dung, Go giữ hàng chờ
// (notice.go) và màn hình này đọc nó lúc dựng — App.vue dựng MỚI mỗi lần có thông báo mới.

declare const window: any
const api = () => window.go?.main?.App

interface Notice { title: string; body: string; at: number }

const items = ref<Notice[]>([])
const okBtn = ref<HTMLButtonElement | null>(null)

// Quầy không điền tiêu đề thì Go đặt "Thông báo": trùng với tiêu đề cửa sổ, bỏ đi cho gọn.
const hasTitle = (n: Notice) => !!n.title && n.title !== 'Thông báo'

function formatTime(at: number) {
	const d = new Date(at)
	return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function parse(raw: unknown): Notice[] {
	try {
		const list = typeof raw === 'string' ? JSON.parse(raw) : raw
		return Array.isArray(list) ? list : []
	} catch { return [] }
}

// Bấm "Đã hiểu" chỉ gạt những cái đang hiện: cái quầy gửi thêm giữa chừng vẫn còn trong hàng
// chờ, Go trả lại để vẽ tiếp. Hết thì Go ẩn cửa sổ.
async function dismiss() {
	const upTo = Math.max(0, ...items.value.map(n => n.at))
	try {
		items.value = parse(await api()?.DismissNotices(upTo))
	} catch { items.value = [] }
	if (items.value.length) focusOk()
}

function focusOk() {
	nextTick(() => okBtn.value?.focus())
}

onMounted(async () => {
	try {
		items.value = parse(await api()?.GetNotices())
	} catch { /* không đọc được thì coi như rỗng */ }
	// Cửa sổ vừa được đánh thức mà không có gì để hiện (đã bị gạt đi): ẩn đi, đừng để hộp trống.
	if (!items.value.length) {
		api()?.DismissNotices(Number.MAX_SAFE_INTEGER)
		return
	}
	phatAm('thong-bao')
	focusOk()
})
</script>

<style scoped>
.notice {
	position: relative;
	display: flex;
	flex-direction: column;
	height: 100vh;
	padding: 22px 24px 20px;
	background:
		radial-gradient(120% 60% at 100% 0%, rgba(76, 125, 255, 0.14), transparent 70%),
		var(--vnet-bg);
	color: var(--vnet-text);
	overflow: hidden;
}

/* Vạch màu logo ở mép trên, cùng kiểu với thẻ đăng nhập. */
.notice-bar {
	position: absolute;
	inset: 0 0 auto 0;
	height: 3px;
	background: var(--vnet-brand);
}

.notice-head {
	display: flex;
	align-items: center;
	gap: 12px;
	flex-shrink: 0;
	margin-bottom: 14px;
}

.notice-icon {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 40px;
	height: 40px;
	flex-shrink: 0;
	border-radius: var(--vnet-radius-sm);
	background: var(--vnet-primary-soft);
	color: var(--vnet-primary);
}

.notice-heading h2 {
	font-family: var(--vnet-font-display);
	font-size: 18px;
	font-weight: 700;
	line-height: 1.2;
}

.notice-count {
	font-size: 12px;
	color: var(--vnet-text-muted);
}

.notice-list {
	flex: 1;
	min-height: 0;
	display: flex;
	flex-direction: column;
	gap: 10px;
	overflow-y: auto;
}

.notice-item {
	padding: 12px 14px;
	border: 1px solid var(--vnet-border);
	border-radius: var(--vnet-radius-sm);
	background: var(--vnet-surface);
}

.notice-item h3 {
	margin-bottom: 4px;
	font-size: 14px;
	font-weight: 600;
}

/* Quầy gõ tay có xuống dòng: giữ nguyên. Chữ dài không có khoảng trắng thì bẻ chứ không tràn ngang. */
.notice-body {
	font-size: 15px;
	line-height: 1.55;
	white-space: pre-wrap;
	overflow-wrap: anywhere;
}

.notice-time {
	display: block;
	margin-top: 6px;
	font-size: 11px;
	color: var(--vnet-text-faint);
}

.notice-foot {
	flex-shrink: 0;
	margin-top: 16px;
}

.notice-ok {
	width: 100%;
	height: 44px;
	border: none;
	border-radius: var(--vnet-radius-sm);
	background: var(--vnet-brand);
	color: #fff;
	font-family: var(--vnet-font);
	font-size: 15px;
	font-weight: 600;
	cursor: pointer;
	transition: filter 0.15s;
}

.notice-ok:hover {
	filter: brightness(1.1);
}
</style>
