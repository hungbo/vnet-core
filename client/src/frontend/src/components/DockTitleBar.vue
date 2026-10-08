<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import vnetLogo from '../assets/vnet-logo.svg'

// Thanh tiêu đề tự vẽ của thanh dọc mép phải.
//
// Cửa sổ chính không có viền hệ điều hành (main.go, Frameless) vì viền thì không bỏ
// được riêng nút tắt: chỉ làm mờ nó đi chứ không biến mất. Nên ở đây chỉ có hai
// thứ: vùng kéo cửa sổ và MỘT nút thu nhỏ. Không có nút tắt, và mọi lệnh đóng khác
// (Alt+F4, taskbar) cũng chỉ thu nhỏ — xem App.beforeDockClose bên Go.
//
// Thu nhỏ xong thì cửa sổ nằm ở taskbar; bấm nút trên taskbar hoặc lối tắt VNET
// ngoài desktop là hiện lại đúng chỗ cũ.
//
// Chỉ dựng khi đã đăng nhập (App.vue). Màn hình đăng nhập phủ kín màn hình và KHÔNG
// được thu nhỏ — nên ở đó không có thanh này; bên Go cũng từ chối (App.MinimiseDock).

declare const window: any
const api = () => window.go?.main?.App

// Lớp phủ khoá khi mất kết nối máy chủ (MachineLockOverlay) cũng là một lúc đang khoá
// dù khách vẫn "đăng nhập". Lớp phủ nằm đè lên cả thanh này, nhưng đừng dựa vào thứ tự
// lớp: lúc đó nút bị gỡ hẳn khỏi cây.
const dangKhoa = ref(false)
const huy: Array<() => void> = []

onMounted(() => {
	huy.push(EventsOn('vnet:machine:locked', () => { dangKhoa.value = true }))
	huy.push(EventsOn('vnet:machine:unlocked', () => { dangKhoa.value = false }))
})
onUnmounted(() => huy.forEach(f => f()))

function thuNho() {
	api()?.MinimiseDock?.()
}
</script>

<template>
	<header class="dock-bar">
		<img :src="vnetLogo" alt="VNET" class="dock-bar-logo" draggable="false" />
		<button
			v-if="!dangKhoa"
			type="button"
			class="dock-bar-min"
			title="Thu nhỏ xuống thanh tác vụ"
			aria-label="Thu nhỏ"
			@click="thuNho"
		>
			<svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
				<path d="M0 5.5h10" fill="none" stroke="currentColor" stroke-width="1" />
			</svg>
		</button>
	</header>
</template>

<style scoped>
.dock-bar {
	flex: none;
	display: flex;
	align-items: center;
	justify-content: space-between;
	height: var(--vnet-dock-bar-h);
	padding-left: var(--vnet-gap);
	background: var(--vnet-surface);
	border-bottom: 1px solid var(--vnet-border);
	user-select: none;
	/* Vùng kéo cửa sổ của Wails. Đây là thuộc tính tuỳ biến nên con cháu kế thừa luôn
	   — nút bên dưới phải tự ghi đè bằng no-drag, nếu không bấm vào nó là bắt đầu
	   kéo cửa sổ chứ không phải thu nhỏ. */
	--wails-draggable: drag;
}

.dock-bar-logo {
	height: 12px;
	width: auto;
	pointer-events: none;
}

/* Cao hết thanh, rộng cỡ nút thu nhỏ của Windows: vùng bấm lớn mà khách không cần
   ngắm cho trúng. */
.dock-bar-min {
	--wails-draggable: no-drag;
	display: grid;
	place-items: center;
	width: 46px;
	height: 100%;
	padding: 0;
	border: 0;
	background: transparent;
	color: var(--vnet-text-muted);
	cursor: pointer;
	transition: background 0.15s, color 0.15s;
}

.dock-bar-min:hover {
	background: var(--vnet-surface-2);
	color: var(--vnet-text);
}
</style>
