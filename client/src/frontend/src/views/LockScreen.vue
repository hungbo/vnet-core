<template>
	<div class="lock-screen">
		<section class="seat">
			<img :src="vnetLogo" alt="VNET" class="seat-logo" />
			<!-- Ba vệt tốc độ của logo, phóng to, chạy từ mép màn hình vào tên máy. -->
			<div class="speed" aria-hidden="true"><i /><i /><i /></div>
			<!--
				Tên máy là thứ to nhất trên màn hình: khách báo quầy "máy nào" thì
				đọc ngay ở đây, nhân viên đối được với trang Máy.
			-->
			<p v-if="tenMay" class="seat-label">Bạn đang ngồi máy</p>
			<h1 v-if="tenMay" class="seat-name" :title="tenMay" :style="{ '--len': Math.max(tenMay.length, 5) }">{{ tenMay }}</h1>
			<p class="seat-clock">
				<span class="clock-time">{{ gio }}</span>
				<span class="clock-date">{{ ngay }}</span>
			</p>
		</section>

		<section class="login-panel">
			<h2 class="login-title">Đăng nhập để chơi</h2>
			<div v-if="reason" class="lock-reason">{{ reason }}</div>

			<el-tabs v-model="activeTab" class="login-tabs" stretch>
				<el-tab-pane label="Tài khoản" name="pin">
					<el-form @submit.prevent="handleLogin" class="login-form">
						<el-form-item>
							<el-input
								v-model="username"
								placeholder="Tài khoản"
								size="large"
								clearable
							/>
						</el-form-item>
						<el-form-item>
							<el-input
								v-model="password"
								type="password"
								placeholder="Mật khẩu"
								size="large"
								show-password
								@keyup.enter="handleLogin"
							/>
						</el-form-item>
						<el-form-item>
							<el-button
								type="primary"
								size="large"
								:loading="loading"
								@click="handleLogin"
								style="width: 100%;"
							>
								Vào máy
							</el-button>
						</el-form-item>
					</el-form>
				</el-tab-pane>

				<el-tab-pane label="Quét QR" name="qr">
					<div class="qr-scanner" v-if="cameraActive">
						<video ref="videoRef" class="qr-video" autoplay playsinline />
						<canvas ref="canvasRef" class="qr-canvas" />
						<p class="qr-hint">Đưa mã QR vào khung hình</p>
					</div>
					<div class="qr-placeholder" v-else>
						<el-icon :size="64" color="var(--vnet-text-muted)"><Camera /></el-icon>
						<p class="qr-hint">Quét mã QR trên ứng dụng VNET Mobile</p>
						<el-button type="primary" size="large" @click="startCamera" style="margin-top: 16px;">
							Mở camera
						</el-button>
					</div>
				</el-tab-pane>
			</el-tabs>
		</section>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Camera } from '@element-plus/icons-vue'
import jsQR from 'jsqr'
import vnetLogo from '../assets/vnet-logo.svg'

declare const window: any
const api = () => window.go?.main?.App

const emit = defineEmits<{
	login: [username: string, password: string]
}>()

// Tên máy (mã máy) — chính là tên máy Windows, cũng là mã trên trang Máy.
const tenMay = ref('')

// Đồng hồ trên màn hình khoá: máy trống thì đây là thứ duy nhất đổi trên màn hình.
const bayGio = ref(new Date())
let dongHo: number | null = null
const gio = computed(() => bayGio.value.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }))
const ngay = computed(() => bayGio.value.toLocaleDateString('vi-VN', { weekday: 'long', day: 'numeric', month: 'numeric' }))
onMounted(() => { dongHo = window.setInterval(() => { bayGio.value = new Date() }, 15000) })
onUnmounted(() => { if (dongHo) clearInterval(dongHo) })
onMounted(async () => {
	try { tenMay.value = await api().GetMachineCode() } catch { tenMay.value = '' }
})

defineProps<{
	loading?: boolean
	reason?: string
}>()

const activeTab = ref('pin')
const username = ref('')
const password = ref('')
const cameraActive = ref(false)
const videoRef = ref<HTMLVideoElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)

let stream: MediaStream | null = null
let scanInterval: number | null = null

function handleLogin() {
	if (!username.value || !password.value) return
	emit('login', username.value, password.value)
}

async function startCamera() {
	try {
		stream = await navigator.mediaDevices.getUserMedia({
			video: { facingMode: 'environment', width: { ideal: 640 }, height: { ideal: 480 } },
		})
		cameraActive.value = true

		await new Promise<void>((resolve) => {
			if (videoRef.value) {
				videoRef.value.srcObject = stream
				videoRef.value.onloadedmetadata = () => resolve()
			}
		})

		scanInterval = window.setInterval(scanQR, 500)
	} catch {
		cameraActive.value = false
	}
}

function stopCamera() {
	if (scanInterval) {
		clearInterval(scanInterval)
		scanInterval = null
	}
	if (stream) {
		stream.getTracks().forEach(t => t.stop())
		stream = null
	}
	cameraActive.value = false
}

function scanQR() {
	if (!videoRef.value || !canvasRef.value) return
	const video = videoRef.value
	const canvas = canvasRef.value

	if (video.readyState !== video.HAVE_ENOUGH_DATA) return

	canvas.width = video.videoWidth
	canvas.height = video.videoHeight
	const ctx = canvas.getContext('2d')
	if (!ctx) return

	ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
	const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
	const code = jsQR(imageData.data, imageData.width, imageData.height)

	if (code) {
		try {
			const data = JSON.parse(code.data)
			if (data.username && data.password) {
				emit('login', data.username, data.password)
				stopCamera()
			}
		} catch {
			// QR might contain raw token format
		}
	}
}

onUnmounted(() => {
	stopCamera()
})
</script>

<style scoped>
.lock-screen {
	position: relative;
	display: grid;
	grid-template-columns: minmax(0, 1fr) 400px;
	align-items: center;
	gap: 64px;
	height: 100vh;
	padding: 0 8vw;
	overflow: hidden;
	/* Luồng sáng xanh-tím chéo từ góc trái, cùng hướng nghiêng với logo. */
	background:
		radial-gradient(120% 90% at 0% 100%, rgba(123, 92, 255, 0.18), transparent 55%),
		radial-gradient(90% 70% at 15% 10%, rgba(76, 125, 255, 0.16), transparent 60%),
		var(--vnet-bg);
}

/* Ba vệt tốc độ: dải màu của logo, nghiêng đúng góc chữ VNET, dừng ngay trước
   chữ cái đầu của tên máy — như chính logo, nơi ba gạch dẫn vào chữ V. */
.speed {
	position: absolute;
	right: calc(100% + 20px);
	bottom: 92px;
	display: flex;
	flex-direction: column;
	align-items: flex-end;
	gap: 16px;
	transform: skewX(-18deg);
	pointer-events: none;
}

.speed i {
	display: block;
	height: 8px;
	border-radius: 2px;
	background: var(--vnet-brand);
}

.speed i:nth-child(1) { width: 6vw; opacity: 0.35; }
.speed i:nth-child(2) { width: 10vw; opacity: 0.6; }
.speed i:nth-child(3) { width: 5vw; opacity: 0.25; }

.seat {
	position: relative;
	min-width: 0;
}

.seat-logo {
	display: block;
	width: 160px;
	height: auto;
	margin-bottom: 18vh;
}

.seat-label {
	margin: 0 0 4px;
	font-size: 18px;
	color: var(--vnet-text-muted);
}

/* Điểm nhấn duy nhất của màn hình: tên máy, to, nghiêng như logo. */
.seat-name {
	margin: 0;
	font-family: var(--vnet-font-display);
	font-style: italic;
	font-weight: 700;
	/* Cỡ chữ co theo độ dài tên để tên máy Windows thường gặp (15 ký tự) vẫn
	   hiện đủ: bề ngang cột trái chia cho số ký tự, mỗi ký tự rộng ~0,64em.
	   Tên rất dài thì chạm cỡ tối thiểu rồi mới cắt bằng dấu ba chấm. */
	font-size: clamp(32px, calc((84vw - 464px) / (var(--len) * 0.64)), 132px);
	line-height: 1.05;
	color: var(--vnet-text);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
	/* Chữ nghiêng bị cắt mép phải khi ẩn phần tràn; chừa chỗ cho nét cuối. */
	padding-right: 0.12em;
}

.seat-clock {
	display: flex;
	align-items: baseline;
	gap: 14px;
	margin: 20px 0 0;
}

.clock-time {
	font-family: var(--vnet-font-display);
	font-weight: 600;
	font-size: 36px;
	font-variant-numeric: tabular-nums;
	color: var(--vnet-text);
}

.clock-date {
	font-size: 16px;
	color: var(--vnet-text-muted);
}

.clock-date::first-letter {
	text-transform: uppercase;
}

.login-panel {
	position: relative;
	padding: 32px 28px 20px;
	background: rgba(17, 24, 51, 0.82);
	border: 1px solid var(--vnet-border);
	border-radius: 14px;
	box-shadow: var(--vnet-shadow);
	backdrop-filter: blur(12px);
}

/* Viền trên mang màu logo: thứ nối khung đăng nhập với vệt tốc độ. */
.login-panel::before {
	content: '';
	position: absolute;
	left: 24px;
	right: 24px;
	top: -2px;
	height: 3px;
	border-radius: 3px;
	background: var(--vnet-brand);
}

.login-title {
	margin: 0 0 16px;
	font-size: 20px;
	font-weight: 600;
	color: var(--vnet-text);
}

.lock-reason {
	margin: 0 0 16px;
	padding: 10px 12px;
	border-radius: 8px;
	background: rgba(255, 181, 71, 0.12);
	border: 1px solid rgba(255, 181, 71, 0.4);
	color: var(--vnet-warning);
	font-size: 14px;
}

.login-tabs {
	margin-bottom: 4px;
}

.login-form .el-form-item {
	margin-bottom: 14px;
}

.login-form :deep(.el-input__wrapper) {
	background: var(--vnet-bg);
	box-shadow: 0 0 0 1px var(--vnet-border) inset;
}

.login-form :deep(.el-input__wrapper.is-focus) {
	box-shadow: 0 0 0 1px var(--vnet-primary) inset;
}

.login-form :deep(.el-button--primary) {
	height: 48px;
	font-size: 16px;
	font-weight: 600;
	border: none;
	background: var(--vnet-brand);
}

.qr-scanner {
	display: flex;
	flex-direction: column;
	align-items: center;
	padding: 16px 0;
}

.qr-video {
	width: 280px;
	height: 210px;
	border-radius: 12px;
	object-fit: cover;
	background: #000;
}

.qr-canvas {
	display: none;
}

.qr-placeholder {
	display: flex;
	flex-direction: column;
	align-items: center;
	padding: 24px 0;
}

.qr-hint {
	color: var(--vnet-text-muted);
	font-size: 13px;
	margin-top: 12px;
	text-align: center;
}

/* Màn hình hẹp hoặc dọc: xếp chồng, tên máy lên trên. */
@media (max-width: 900px) {
	.lock-screen {
		grid-template-columns: 1fr;
		align-content: center;
		gap: 32px;
		padding: 0 24px;
	}
	.seat-logo { margin-bottom: 24px; }
	.seat-name { font-size: clamp(32px, calc((100vw - 48px) / (var(--len) * 0.64)), 96px); }
	.login-panel { max-width: 420px; }
}
</style>
