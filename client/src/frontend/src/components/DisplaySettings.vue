<template>
	<!--
		Ẩn hẳn khi Go báo supported=false (không phải Windows, hoặc không liệt kê
		được màn hình) hoặc cuộc gọi lỗi: một phần Cài đặt không chỉnh được gì thì
		chỉ là bốn điều khiển chết.

		Class settings-card lấy kiểu từ SettingsPage: gốc của component con luôn
		nhận scoped CSS của cha. Các thẻ con bên trong thì không, nên h4 phải khai
		lại ở dưới.
	-->
	<section v-if="info" class="settings-card">
		<h4>Màn hình &amp; chuột</h4>

		<div class="block">
			<div class="field-row">
				<span class="field-label">Tốc độ chuột</span>
				<span class="field-value">{{ mouseSpeed }}</span>
			</div>
			<el-slider
				v-model="mouseSpeed"
				:min="1"
				:max="20"
				:show-tooltip="false"
				:disabled="khoa"
				aria-label="Tốc độ chuột"
				@input="onSpeedInput"
			/>

			<div class="field-row switch-row">
				<span class="field-label">Tăng độ chính xác con trỏ</span>
				<el-switch
					v-model="precision"
					:loading="dangDoiChinhXac"
					:disabled="khoa"
					aria-label="Tăng độ chính xác con trỏ"
					@change="onPrecisionChange"
				/>
			</div>
		</div>

		<div class="block">
			<div class="mode-grid">
				<div>
					<span class="field-label">Độ phân giải</span>
					<el-select
						v-model="selRes"
						:disabled="khoa"
						aria-label="Độ phân giải"
						@change="onResolutionChange"
					>
						<el-option v-for="r in resolutions" :key="r.key" :label="r.label" :value="r.key" />
					</el-select>
				</div>
				<div>
					<span class="field-label">Tần số quét</span>
					<el-select v-model="selHz" :disabled="khoa" aria-label="Tần số quét">
						<el-option v-for="hz in hzOptions" :key="hz" :label="`${hz} Hz`" :value="hz" />
					</el-select>
				</div>
			</div>
			<el-button type="primary" :loading="dangApDung" :disabled="khoa || !coThayDoi" @click="apply">
				Áp dụng
			</el-button>
		</div>

		<div class="block">
			<el-button :loading="dangKhoiPhuc" :disabled="khoa" @click="reset">Khôi phục mặc định</el-button>
			<p class="hint">
				Về {{ info.baseline.width }} × {{ info.baseline.height }} · {{ info.baseline.hz }} Hz và
				cài đặt chuột lúc đầu. Máy cũng tự khôi phục khi khoá máy hoặc đăng xuất.
			</p>
		</div>

		<!--
			Hộp này KHÔNG có lối thoát vô ý: không nút đóng, không đóng khi bấm nền
			hay nhấn Esc. Độ phân giải sai có thể làm màn hình đen, và lúc đó cái
			duy nhất cứu được là đếm ngược tự hoàn tác — một cú nhấn nhầm không được
			phép làm nó biến mất.

			Hoàn tác đứng trước Giữ để nhận tiêu điểm đầu tiên: nhấn Enter vội thì
			rơi vào lựa chọn an toàn.
		-->
		<el-dialog
			:model-value="hopXacNhan"
			title="Giữ cài đặt màn hình này?"
			width="min(320px, calc(100vw - 24px))"
			align-center
			append-to-body
			:show-close="false"
			:close-on-click-modal="false"
			:close-on-press-escape="false"
		>
			<p class="confirm-mode">{{ cheDoMoi }}</p>
			<p class="confirm-count">
				Tự hoàn tác sau <strong>{{ giayConLai }}</strong> giây
			</p>
			<template #footer>
				<el-button :disabled="dangChot" @click="settle(false)">Hoàn tác</el-button>
				<el-button type="primary" :loading="dangChot" @click="settle(true)">Giữ</el-button>
			</template>
		</el-dialog>
	</section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'

declare const window: any
const api = () => window.go?.main?.App

// Chỉ xử lý MÀN HÌNH CHÍNH — Go cũng chỉ liệt kê và đổi màn hình chính. Máy trạm
// quán net hiếm khi có hai màn hình; thêm chọn màn hình là thêm một thứ để sai.

interface Mode { width: number; height: number; hz: number }
interface DisplayInfo {
	supported: boolean
	modes: Mode[]
	current: Mode
	baseline: Mode
	mouse_speed: number
	mouse_precision: boolean
}

// Go cũng tự hoàn tác ở giây thứ 20 phòng khi giao diện chết giữa chừng; 15 giây ở
// đây chừa 5 giây để lệnh hoàn tác kịp tới nơi trước khi hai bên giẫm chân nhau.
const GIAY_DEM_NGUOC = 15

const info = ref<DisplayInfo | null>(null)
const mouseSpeed = ref(10)
const precision = ref(false)
const selRes = ref('')
const selHz = ref(0)

const dangApDung = ref(false)
const dangKhoiPhuc = ref(false)
const dangDoiChinhXac = ref(false)
const hopXacNhan = ref(false)
const dangChot = ref(false)
const giayConLai = ref(GIAY_DEM_NGUOC)
const cheDoMoi = ref('')

// Hộp xác nhận đang mở thì mọi điều khiển phải đứng yên, kể cả khi người dùng
// vẫn với tới được chúng phía sau lớp phủ.
const khoa = computed(
	() => dangApDung.value || dangKhoiPhuc.value || hopXacNhan.value
)

const keyOf = (m: Mode) => `${m.width}x${m.height}`

// Go đã sắp sẵn (rộng, cao, Hz giảm dần) và loại trùng; ở đây chỉ gộp theo cặp
// rộng × cao, giữ nguyên thứ tự đó.
const resolutions = computed(() => {
	const seen = new Set<string>()
	const out: { key: string; label: string }[] = []
	for (const m of info.value?.modes ?? []) {
		const key = keyOf(m)
		if (seen.has(key)) continue
		seen.add(key)
		out.push({ key, label: `${m.width} × ${m.height}` })
	}
	return out
})

const hzOptions = computed(() =>
	(info.value?.modes ?? []).filter(m => keyOf(m) === selRes.value).map(m => m.hz)
)

const coThayDoi = computed(() => {
	const cur = info.value?.current
	return !!cur && (keyOf(cur) !== selRes.value || cur.hz !== selHz.value)
})

function gan(list: number[], target: number) {
	return list.reduce((best, hz) => (Math.abs(hz - target) < Math.abs(best - target) ? hz : best), list[0] ?? 0)
}

function chonTheoHienTai() {
	const cur = info.value!.current
	selRes.value = resolutions.value.some(r => r.key === keyOf(cur)) ? keyOf(cur) : (resolutions.value[0]?.key ?? '')
	selHz.value = gan(hzOptions.value, cur.hz)
}

// Bỏ tiền tố "Error:" mà JS gắn vào: câu báo lỗi từ Go đã là tiếng Việt hoàn chỉnh.
const loi = (e: unknown) => String(e).replace(/^Error:\s*/, '')

// Mỗi lần gọi đánh số lại; kết quả của lần cũ về muộn thì bỏ, nếu không nó ghi
// đè lên trạng thái mới hơn (kéo thanh trượt liên tục sinh ra nhiều lần làm mới).
let lanLamMoi = 0

// resync=false cho thanh trượt và công tắc: làm mới số liệu nhưng không giật
// lại ô chọn độ phân giải mà người dùng đang chọn dở.
async function refresh(resync = true) {
	const lan = ++lanLamMoi
	try {
		const r: DisplayInfo = JSON.parse(await api().GetDisplayInfo())
		if (lan !== lanLamMoi) return
		if (!r.supported) {
			info.value = null
			return
		}
		info.value = r
		// Đang có lệnh tốc độ chờ gửi thì giá trị trên thanh trượt mới hơn Go.
		if (speedTimer === undefined) mouseSpeed.value = r.mouse_speed
		precision.value = r.mouse_precision
		if (resync) chonTheoHienTai()
	} catch {
		if (lan === lanLamMoi) info.value = null
	}
}

// --- Chuột ---

let speedTimer: ReturnType<typeof setTimeout> | undefined

// Thanh trượt bắn `input` liên tục khi kéo; mỗi lần là một lệnh gọi Windows. Chỉ
// gửi giá trị cuối sau khi dừng ~150 ms.
function onSpeedInput() {
	clearTimeout(speedTimer)
	speedTimer = setTimeout(sendSpeed, 150)
}

async function sendSpeed() {
	speedTimer = undefined
	try {
		await api().SetMouseSpeed(mouseSpeed.value)
	} catch (e) {
		ElMessage.error(loi(e))
	}
	await refresh(false)
}

async function onPrecisionChange(on: boolean | string | number) {
	dangDoiChinhXac.value = true
	try {
		await api().SetMousePrecision(!!on)
	} catch (e) {
		ElMessage.error(loi(e))
	} finally {
		dangDoiChinhXac.value = false
	}
	await refresh(false)
}

// --- Màn hình ---

function onResolutionChange() {
	selHz.value = gan(hzOptions.value, info.value!.current.hz)
}

let tick: ReturnType<typeof setInterval> | undefined

function dungDem() {
	clearInterval(tick)
	tick = undefined
}

function batDauDem() {
	// Tính theo mốc giờ chứ không đếm số lần tick: webview bị làm chậm timer khi
	// ở nền, mà hạn chót hoàn tác thì không chờ ai.
	const han = Date.now() + GIAY_DEM_NGUOC * 1000
	giayConLai.value = GIAY_DEM_NGUOC
	tick = setInterval(() => {
		giayConLai.value = Math.max(0, Math.ceil((han - Date.now()) / 1000))
		if (giayConLai.value === 0) settle(false, true)
	}, 250)
}

async function apply() {
	const [width, height] = selRes.value.split('x').map(Number)
	dangApDung.value = true
	try {
		await api().ApplyDisplayMode(width, height, selHz.value)
		cheDoMoi.value = `${width} × ${height} · ${selHz.value} Hz`
		hopXacNhan.value = true
		batDauDem()
	} catch (e) {
		// Giữ nguyên ô đang chọn để người dùng chỉnh lại thay vì chọn lại từ đầu.
		ElMessage.error(loi(e))
		await refresh(false)
	} finally {
		dangApDung.value = false
	}
}

// Giữ hoặc hoàn tác. Hộp luôn đóng và số liệu luôn được đọc lại dù lệnh có lỗi:
// để hộp treo là để người dùng kẹt sau một lớp phủ không còn nút nào hoạt động.
async function settle(giu: boolean, hetGio = false) {
	if (dangChot.value) return
	dangChot.value = true
	dungDem()
	try {
		if (giu) {
			await api().ConfirmDisplayMode()
			ElMessage.success('Đã giữ cài đặt màn hình')
		} else {
			await api().RevertDisplayMode()
			ElMessage[hetGio ? 'warning' : 'info'](
				hetGio ? 'Hết thời gian xác nhận, đã hoàn tác cài đặt màn hình' : 'Đã hoàn tác cài đặt màn hình'
			)
		}
	} catch (e) {
		ElMessage.error(loi(e))
	} finally {
		hopXacNhan.value = false
		dangChot.value = false
	}
	await refresh()
}

async function reset() {
	// Lệnh tốc độ đang chờ mà bắn sau khi khôi phục sẽ đè lại giá trị mặc định.
	clearTimeout(speedTimer)
	speedTimer = undefined
	dangKhoiPhuc.value = true
	try {
		await api().ResetDisplayAndMouse()
		ElMessage.success('Đã khôi phục cài đặt mặc định')
	} catch (e) {
		ElMessage.error(loi(e))
	} finally {
		dangKhoiPhuc.value = false
	}
	await refresh()
}

onMounted(() => refresh())

// Rời trang trong 150 ms sau khi thả thanh trượt: gửi nốt giá trị cuối thay vì để
// nó rơi mất. Hộp xác nhận đang mở thì Go tự hoàn tác sau 20 giây, không cần lo.
onBeforeUnmount(() => {
	dungDem()
	if (speedTimer !== undefined) {
		clearTimeout(speedTimer)
		api()?.SetMouseSpeed(mouseSpeed.value)?.catch?.(() => {})
	}
})
</script>

<style scoped>
.settings-card h4 {
	font-size: 14px;
	font-weight: 600;
	color: var(--vnet-text);
	margin-bottom: 14px;
}

.block + .block {
	margin-top: 16px;
	padding-top: 16px;
	border-top: 1px solid var(--vnet-border);
}

.field-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
}

.switch-row {
	margin-top: 8px;
}

.field-label {
	display: block;
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.field-value {
	font-family: var(--vnet-font-display);
	font-weight: 700;
	font-size: 15px;
	color: var(--vnet-text);
	font-variant-numeric: tabular-nums;
}

/* Hai ô chọn cạnh nhau: bên trong thẻ 360px còn ~296px, đủ cho "2560 × 1440". Cột
   phân giải rộng hơn vì nhãn của nó dài hơn "144 Hz". */
.mode-grid {
	display: grid;
	grid-template-columns: 3fr 2fr;
	gap: 8px;
	margin-bottom: 12px;
}

.mode-grid .field-label {
	margin-bottom: 6px;
}

.hint {
	font-size: 13px;
	color: var(--vnet-text-muted);
	margin-top: 10px;
}

.confirm-mode {
	font-family: var(--vnet-font-display);
	font-weight: 700;
	font-size: 18px;
	color: var(--vnet-text);
	margin-bottom: 8px;
}

.confirm-count {
	font-size: 14px;
	color: var(--vnet-text-muted);
}

/* Hổ phách là màu dành cho "sắp hết giờ" (tokens.css) — đúng nghĩa của con số này. */
.confirm-count strong {
	font-family: var(--vnet-font-display);
	font-size: 18px;
	color: var(--vnet-warning);
	font-variant-numeric: tabular-nums;
}
</style>
