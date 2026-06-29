<template>
	<div class="settings-page">
		<header class="settings-header">
			<el-button text @click="$emit('back')">
				<el-icon><ArrowLeft /></el-icon>
				Quay lại
			</el-button>
			<h3>Cài đặt</h3>
			<div style="width: 80px;"></div>
		</header>

		<div class="settings-body">
			<el-card shadow="never" class="settings-card">
				<template #header>
					<span>Đổi mật khẩu / PIN</span>
				</template>
				<el-form :model="pinForm" label-position="top">
					<el-form-item label="Mật khẩu cũ">
						<el-input v-model="pinForm.oldPin" type="password" show-password />
					</el-form-item>
					<el-form-item label="Mật khẩu mới">
						<el-input v-model="pinForm.newPin" type="password" show-password />
					</el-form-item>
					<el-form-item label="Xác nhận mật khẩu mới">
						<el-input v-model="pinForm.confirmPin" type="password" show-password />
					</el-form-item>
					<el-button
						type="primary"
						:loading="changingPin"
						@click="changePin"
					>
						Đổi mật khẩu
					</el-button>
				</el-form>
			</el-card>

			<el-card shadow="never" class="settings-card">
				<template #header>
					<span>Cấu hình thiết bị</span>
				</template>
				<el-form label-position="top">
					<el-form-item label="Mã máy">
						<el-input v-model="machineCode" @change="saveMachineCode" placeholder="VD: M01" />
					</el-form-item>
					<el-form-item label="Địa chỉ server">
						<el-input v-model="serverUrl" @change="saveServerUrl" placeholder="http://localhost:8080" />
					</el-form-item>
				</el-form>
			</el-card>

			<el-card shadow="never" class="settings-card">
				<template #header>
					<span>Thông tin</span>
				</template>
				<el-descriptions :column="1" border size="small">
					<el-descriptions-item label="Mã máy chủ">{{ hardware?.machine_code || '—' }}</el-descriptions-item>
					<el-descriptions-item label="Server">{{ hardware?.server_url || '—' }}</el-descriptions-item>
					<el-descriptions-item v-if="hardware?.cpu_name" label="CPU">{{ hardware.cpu_name }}</el-descriptions-item>
					<el-descriptions-item v-if="hardware?.gpu_name" label="GPU">{{ hardware.gpu_name }}</el-descriptions-item>
					<el-descriptions-item v-if="hardware?.ram_gb" label="RAM">{{ hardware.ram_gb }} GB</el-descriptions-item>
					<el-descriptions-item v-if="hardware?.storage_gb" label="Ổ cứng">{{ hardware.storage_gb }} GB</el-descriptions-item>
					<el-descriptions-item v-if="hardware?.cpu_temp" label="Nhiệt CPU">{{ hardware.cpu_temp }}°C</el-descriptions-item>
					<el-descriptions-item v-if="hardware?.gpu_temp" label="Nhiệt GPU">{{ hardware.gpu_temp }}°C</el-descriptions-item>
					<el-descriptions-item label="Phiên bản">1.0.0</el-descriptions-item>
					<el-descriptions-item label="Nền tảng">VNET Desktop</el-descriptions-item>
				</el-descriptions>
			</el-card>

			<div class="logout-section">
				<el-button type="danger" size="large" @click="$emit('logout')">
					Đăng xuất
				</el-button>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ArrowLeft } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

declare const window: any
const api = () => window.go?.main?.App

const emit = defineEmits<{
	back: []
	logout: []
}>()

const changingPin = ref(false)
const serverUrl = ref('http://localhost:8080')
const machineCode = ref('')
const hardware = ref<any>(null)
const pinForm = ref({
	oldPin: '',
	newPin: '',
	confirmPin: '',
})

function saveServerUrl() {
	localStorage.setItem('vnet_server_url', serverUrl.value)
	api().SetServerURL(serverUrl.value)
}

function saveMachineCode() {
	localStorage.setItem('vnet_machine_code', machineCode.value)
	api().SetMachineCode(machineCode.value)
}

async function changePin() {
	if (!pinForm.value.oldPin || !pinForm.value.newPin) {
		ElMessage.warning('Nhập đầy đủ thông tin')
		return
	}
	if (pinForm.value.newPin !== pinForm.value.confirmPin) {
		ElMessage.warning('Mật khẩu mới không khớp')
		return
	}
	changingPin.value = true
	try {
		await api().ChangePin(pinForm.value.oldPin, pinForm.value.newPin)
		ElMessage.success('Đã đổi mật khẩu')
		pinForm.value = { oldPin: '', newPin: '', confirmPin: '' }
	} catch (e) {
		ElMessage.error(String(e))
	} finally {
		changingPin.value = false
	}
}

async function loadHardware() {
	try {
		const hw = await api().GetHardware()
		const parsed = JSON.parse(hw)
		if (parsed?.data) {
			hardware.value = parsed.data
		} else if (parsed?.machine_code) {
			hardware.value = parsed
		}
	} catch { /* ignore */ }
}

onMounted(() => {
	const savedUrl = localStorage.getItem('vnet_server_url')
	if (savedUrl) serverUrl.value = savedUrl
	const savedCode = localStorage.getItem('vnet_machine_code')
	if (savedCode) machineCode.value = savedCode
	loadHardware()
})
</script>

<style scoped>
.settings-page {
	height: 100vh;
	display: flex;
	flex-direction: column;
	background: #f0f2f5;
}

.settings-header {
	display: flex;
	align-items: center;
	padding: 12px 16px;
	background: #fff;
	box-shadow: 0 1px 4px rgba(0,0,0,.08);
}

.settings-header h3 {
	font-size: 16px;
	font-weight: 600;
	flex: 1;
	text-align: center;
}

.settings-body {
	flex: 1;
	overflow-y: auto;
	padding: 16px;
	max-width: 480px;
	margin: 0 auto;
	width: 100%;
	display: flex;
	flex-direction: column;
	gap: 16px;
}

.settings-card {
	border-radius: 12px;
}

.logout-section {
	display: flex;
	justify-content: center;
	padding: 16px 0;
}
</style>
