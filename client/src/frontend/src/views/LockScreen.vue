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
								placeholder="Tài khoản hội viên"
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
						<el-icon :size="64" color="#909399"><Camera /></el-icon>
						<p class="qr-hint">Quét mã QR trên ứng dụng VNET Mobile</p>
						<el-button type="primary" size="large" @click="startCamera" style="margin-top: 16px;">
							Mở camera
						</el-button>
					</div>
				</el-tab-pane>
			</el-tabs>

			<div class="admin-link">
				<el-button text size="small" @click="$emit('adminLogin')">
					Đăng nhập quản trị
				</el-button>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { Camera } from '@element-plus/icons-vue'
import jsQR from 'jsqr'

const emit = defineEmits<{
	login: [username: string, password: string]
	adminLogin: []
}>()

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
	background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
}

.lock-card {
	width: 380px;
	padding: 40px 32px 24px;
	background: rgba(255, 255, 255, .95);
	border-radius: 16px;
	box-shadow: 0 20px 60px rgba(0, 0, 0, .3);
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
	color: #909399;
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
	color: #909399;
	font-size: 13px;
	margin-top: 12px;
	text-align: center;
}

.admin-link {
	text-align: center;
	margin-top: 8px;
	border-top: 1px solid #eee;
	padding-top: 12px;
}
</style>
