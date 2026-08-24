<template>
	<div class="lock-screen">
		<div class="lock-card">
			<div class="logo">
				<span class="logo-text">VNET</span>
				<span class="logo-sub">GAMING</span>
			</div>

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
								placeholder="Mật khẩu / PIN"
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
								Đăng nhập
							</el-button>
						</el-form-item>
					</el-form>
				</el-tab-pane>

				<el-tab-pane label="QR Code" name="qr">
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

			<div class="admin-link">
				<el-button v-if="coPin" text size="small" @click="hienPin = !hienPin">
					Mở khoá kỹ thuật
				</el-button>
			</div>

			<!--
				Đường vào DUY NHẤT khi mất mạng. Mọi cách đăng nhập phía trên đều
				gọi API, nên router hỏng là cả phòng máy đứng trước màn hình khoá
				phủ kín mà không có gì gõ vào được.
			-->
			<div v-if="hienPin" class="pin-box">
				<el-input
					v-model="pin"
					type="password"
					size="large"
					placeholder="PIN kỹ thuật"
					show-password
					@keyup.enter="moKhoaKyThuat"
				/>
				<el-button type="warning" size="large" :loading="dangMo" @click="moKhoaKyThuat">
					Mở khoá
				</el-button>
				<p class="pin-hint">
					Mở máy để sửa chữa. Không mở phiên chơi và không tính tiền.
				</p>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Camera } from '@element-plus/icons-vue'
import jsQR from 'jsqr'

declare const window: any
const api = () => window.go?.main?.App

const emit = defineEmits<{
	login: [username: string, password: string]
}>()

const coPin = ref(false)
const hienPin = ref(false)
const pin = ref('')
const dangMo = ref(false)

// Máy chưa đặt PIN thì không hiện nút: mời người ta gõ vào một cái không bao giờ
// đúng là cách chắc chắn nhất để họ tưởng máy hỏng.
onMounted(async () => {
	try { coPin.value = await api().HasMaintenancePin() } catch { coPin.value = false }
})

async function moKhoaKyThuat() {
	if (!pin.value) return
	dangMo.value = true
	try {
		await api().UnlockMaintenance(pin.value)
		pin.value = ''
		hienPin.value = false
	} catch (e) {
		ElMessage.error(String(e).replace(/^Error:\s*/, ''))
	} finally {
		dangMo.value = false
	}
}

defineProps<{
	loading?: boolean
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
	display: flex;
	align-items: center;
	justify-content: center;
	height: 100vh;
	/* Nền dựng từ token thay vì ba mã màu chép tay: đây là màn hình phủ kín và
	   đứng nguyên suốt thời gian máy trống, nên nó phải cùng một tông với phần
	   còn lại chứ không phải một hòn đảo riêng. */
	background: radial-gradient(circle at 50% 30%, var(--vnet-surface-2), var(--vnet-bg) 70%);
}

.lock-card {
	width: 380px;
	padding: 40px 32px 24px;
	/* Thẻ TRẮNG trên nền tối là mảnh sót lại từ trước khi máy trạm chuyển sang
	   tông tối. Không ai thấy nó vì màn hình khoá xưa nay nằm gọn trong thanh
	   360px; giờ nó phủ kín màn hình nên sai màu là sai to. */
	background: var(--vnet-surface);
	border: 1px solid var(--vnet-border);
	border-radius: 16px;
	box-shadow: var(--vnet-shadow);
}

.logo {
	text-align: center;
	margin-bottom: 32px;
}

.logo-text {
	font-size: 36px;
	font-weight: 800;
	background: linear-gradient(135deg, #667eea, #764ba2);
	-webkit-background-clip: text;
	background-clip: text;
	color: transparent;
	letter-spacing: 2px;
}

.logo-sub {
	display: block;
	font-size: 11px;
	color: var(--vnet-text-muted);
	letter-spacing: 4px;
	text-transform: uppercase;
	margin-top: 4px;
}

.login-tabs {
	margin-bottom: 16px;
}

.login-form .el-form-item {
	margin-bottom: 16px;
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
	padding: 32px 0;
}

.qr-hint {
	color: var(--vnet-text-muted);
	font-size: 13px;
	margin-top: 12px;
	text-align: center;
}

.pin-box {
	display: flex;
	flex-direction: column;
	gap: 10px;
	margin-top: 12px;
}

.pin-hint {
	font-size: 12px;
	color: var(--vnet-text-muted);
	text-align: center;
}

.admin-link {
	text-align: center;
	margin-top: 8px;
	border-top: 1px solid #eee;
	padding-top: 12px;
}
</style>
