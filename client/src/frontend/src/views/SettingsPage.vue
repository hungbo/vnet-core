<template>
	<div class="settings-page">
		<!--
			Ba cột thật, không phải hai nút rồi chèn một <div width:80px> cho cân —
			cái chèn đó lệch ngay khi nhãn nút đổi độ dài.
		-->
		<header class="settings-header">
			<el-button text @click="$emit('back')">
				<el-icon><ArrowLeft /></el-icon>
				Quay lại
			</el-button>
			<h3>Cài đặt</h3>
			<span />
		</header>

		<div class="settings-body">
			<!--
				KHÔNG dùng el-card ở đây. Element Plus đặt sẵn
				`.el-card__body { flex-grow: 1; overflow: auto }`, nên trong một
				cột flex mỗi thẻ tự thành MỘT vùng cuộn riêng: trang Cài đặt hoá
				ra bốn thanh cuộn lồng nhau, và cái ở ngoài thì không bao giờ
				cuộn. Khối thường thì cả trang cuộn chung một lần.
			-->
			<section class="settings-card">
				<h4>Đổi mật khẩu</h4>
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
					<el-button type="primary" :loading="changingPin" @click="changePin">
						Đổi mật khẩu
					</el-button>
				</el-form>
			</section>

			<!--
				Chỉ quản trị mới đổi được địa chỉ máy chủ. Gõ nhầm một ký tự là
				máy trạm mất liên lạc, và người sửa được chuyện đó lại không ngồi
				trước máy đó.

				Mã máy KHÔNG có ở đây: nó do bộ cài ghi vào config.json, và dịch
				vụ nền cũng đọc từ đó. Cho sửa ở giao diện nghĩa là hai nguồn cho
				cùng một giá trị, rồi máy chủ thấy một mã còn dịch vụ khai một mã.
			-->
			<section v-if="laQuanTri" class="settings-card">
				<h4>Cấu hình thiết bị</h4>
				<el-form label-position="top">
					<el-form-item label="Địa chỉ server">
						<el-input :model-value="serverUrl" readonly />
					</el-form-item>
					<el-form-item label="Mã máy">
						<el-input :model-value="maMay" readonly />
					</el-form-item>
				</el-form>
				<!--
					Chỉ xem, không sửa ở đây. Địa chỉ máy chủ có đúng một nguồn là
					config.json, nơi dịch vụ nền cũng đọc. Bản cũ cho sửa ở đây và
					lưu vào trình duyệt: giao diện và dịch vụ nền nói chuyện với hai
					máy chủ khác nhau, và ô này còn hiện cứng localhost chứ không
					phải địa chỉ đang dùng.
				-->
				<p class="update-notes">
					Muốn đổi thì sửa tệp config.json cạnh vnet-client.exe (cần quyền quản trị) rồi khởi động lại máy.
				</p>
			</section>

			<DisplaySettings />

			<section class="settings-card">
				<h4>Cập nhật ứng dụng</h4>

				<p class="update-notes">Đang dùng bản {{ version || '—' }}</p>

				<el-alert v-if="update?.has_update" type="warning" :closable="false" show-icon
					:title="`Có bản mới: ${update.version}`" style="margin-bottom: 12px" />
				<el-alert v-else-if="checked" type="success" :closable="false" show-icon
					title="Đang dùng bản mới nhất" style="margin-bottom: 12px" />

				<p v-if="update?.changelog" class="update-notes">{{ update.changelog }}</p>
				<p v-if="update?.has_update" class="update-notes">
					Máy tự cài bản mới trong vòng 10 phút, vào lúc không có khách ngồi.
				</p>

				<div class="update-actions">
					<el-button :loading="checking" @click="checkUpdate">Kiểm tra cập nhật</el-button>
				</div>
			</section>

			<div class="logout-section">
				<el-button type="danger" size="large" @click="$emit('logout')">
					Đăng xuất
				</el-button>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { ArrowLeft } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useSessionStore } from '../stores/session.store'
import DisplaySettings from '../components/DisplaySettings.vue'

declare const window: any
const api = () => window.go?.main?.App

const emit = defineEmits<{
	back: []
	logout: []
}>()

const changingPin = ref(false)

// GetVersion/CheckUpdate đã có trong update.go nhưng phía Vue
// chưa gọi hàm nào, và ô "Phiên bản" ở trên là chuỗi cứng 1.0.0 — số hiển thị
// không liên quan gì tới bản đang chạy.
const version = ref('')
const checking = ref(false)
const checked = ref(false)
const update = ref<any>(null)

async function checkUpdate() {
	checking.value = true
	try {
		update.value = JSON.parse(await api().CheckUpdate())
		checked.value = true
	} catch (e) {
		ElMessage.error(String(e))
	} finally {
		checking.value = false
	}
}

const serverUrl = ref('')
const maMay = ref('')

// Chỉ quản trị mới thấy phần Cấu hình thiết bị. Nhân viên đăng nhập trên máy
// trạm mang vai trò 'admin' (App.Login quy về), hội viên thì không.
const session = useSessionStore()
const laQuanTri = computed(() => session.role === 'admin')
const pinForm = ref({
	oldPin: '',
	newPin: '',
	confirmPin: '',
})

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

onMounted(() => {
	api().GetServerURL().then((u: string) => { serverUrl.value = u }).catch(() => {})
	api().GetMachineCode().then((c: string) => { maMay.value = c }).catch(() => {})
	api().GetVersion().then((v: string) => { version.value = v }).catch(() => {})
})
</script>

<style scoped>
.update-notes {
	font-size: 13px;
	color: var(--vnet-text-muted);
	margin-bottom: 12px;
	white-space: pre-wrap;
}

.update-actions {
	display: flex;
	gap: 8px;
}

.settings-page {
	/* Lấp phần còn lại dưới DockTitleBar, không phải cả màn hình (100vh sẽ tràn). */
	flex: 1;
	min-height: 0;
	display: flex;
	flex-direction: column;
	background: var(--vnet-bg);
}

.settings-header {
	display: grid;
	grid-template-columns: 1fr auto 1fr;
	align-items: center;
	padding: 10px var(--vnet-gap);
	background: var(--vnet-surface);
	border-bottom: 1px solid var(--vnet-border);
}

.settings-header h3 {
	font-size: 15px;
	font-weight: 600;
	text-align: center;
	white-space: nowrap;
}

.settings-body {
	flex: 1;
	overflow-y: auto;
	padding: var(--vnet-gap);
	/* Không kẹp 480px và không căn giữa: thanh chỉ rộng 360px, cột hẹp giữa màn
	   hình rộng là bố cục của trang web chứ không phải của thanh công cụ. */
	width: 100%;
	display: flex;
	flex-direction: column;
	gap: var(--vnet-gap);
}

.settings-card {
	background: var(--vnet-surface);
	border: 1px solid var(--vnet-border);
	border-radius: var(--vnet-radius);
	padding: 16px;
}

.settings-card h4 {
	font-size: 14px;
	font-weight: 600;
	color: var(--vnet-text);
	margin-bottom: 14px;
}

.logout-section {
	display: flex;
	justify-content: center;
	padding: 16px 0;
}
</style>
