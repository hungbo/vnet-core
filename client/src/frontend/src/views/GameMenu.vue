<template>
	<div class="game-menu">
		<header class="menu-header">
			<div class="menu-brand">
				<!-- Logo và tên quán do quán đặt ở trang quản trị; thiếu thì chỉ còn chữ "Game". -->
				<img v-if="branding.logo" class="menu-logo" :src="branding.logo" alt="" />
				<h1>{{ branding.name || 'Game' }}</h1>
			</div>

			<!-- Tab theo nhóm, kiểu menu Kzone khách đã quen: ALL, ONLINE, STEAM, OFFLINE, TOOL
			     và các nhóm khác của quán. Tab không có game nào thì không hiện. -->
			<nav v-if="games.length" class="menu-tabs">
				<button
					v-for="t in tabs"
					:key="t"
					type="button"
					class="tab"
					:class="{ active: tab === t }"
					@click="tab = t"
				>
					{{ t || 'ALL' }}
				</button>
			</nav>

			<!-- Tìm không dấu, theo nhóm hoặc chữ cái đầu ("lmht"). Gõ phím bất kỳ ở đâu
			     trong cửa sổ cũng vào ô này; Esc xoá. -->
			<el-input
				v-if="games.length"
				ref="searchInput"
				v-model="query"
				class="menu-search"
				clearable
				placeholder="Tìm kiếm …"
				:prefix-icon="Search"
				@keydown.esc="query = ''"
			/>
		</header>

		<!-- Mất mạng giữa chừng thì giữ danh sách cũ và báo một dòng, đừng xoá
		     trắng màn hình đang có game chơi được. -->
		<div v-if="loadError && games.length" class="load-error">{{ loadError }} — đang thử lại</div>

		<main v-loading="loading" class="menu-body">
			<div v-if="!loading && !games.length" class="empty">
				<template v-if="loadError">
					<p>{{ loadError }}</p>
					<el-button @click="load">Thử lại</el-button>
				</template>
				<p v-else>Quán chưa có game nào</p>
			</div>

			<div v-else-if="!sections.length" class="empty">
				<p>Không có game nào khớp "{{ query.trim() || tab }}"</p>
				<el-button @click="clearFilters">Xoá bộ lọc</el-button>
			</div>

			<template v-else>
				<section v-for="s in sections" :key="s.name" class="group">
					<h2 class="group-title">{{ s.name }} <span>{{ s.items.length }}</span></h2>
					<div class="game-grid">
						<!-- Cả ô là nút: bấm vào đâu trên ô cũng mở game, và Tab/Enter dùng được. -->
						<button
							v-for="g in s.items"
							:key="gid(g)"
							type="button"
							class="tile"
							:class="{ ready: g.status === 'ready' && !g.problem, busy: launching === gid(g) }"
							:disabled="g.status !== 'ready' || !!g.problem || (!!launching && launching !== gid(g))"
							:title="g.status === 'ready' && !g.problem ? gameLabel(g) : `${gameLabel(g)} — ${badge(g).text}`"
							@click="play(g)"
						>
							<span v-if="g.status === 'ready' && !g.problem && g.label" class="tile-label" :class="'is-' + g.label">
								{{ g.label === 'hot' ? 'HOT' : 'MỚI' }}
							</span>
							<!-- Không có icon (chưa lấy được, hoặc tệp chạy không có) thì vẽ chữ cái đầu
							     trên nền màu suy ra từ tên, để ô nào cũng có hình mà không phải đợi. -->
							<div class="tile-icon" :style="icons[gid(g)] ? undefined : placeholderStyle(g)">
								<img
									v-if="icons[gid(g)]"
									:src="icons[gid(g)]"
									alt=""
									draggable="false"
									@error="icons[gid(g)] = ''"
								/>
								<span v-else class="game-initial" aria-hidden="true">{{ initial(g) }}</span>
								<span v-if="launching === gid(g)" class="tile-opening">
									<el-icon class="is-loading"><Loading /></el-icon>
								</span>
							</div>
							<span v-if="g.status !== 'ready' || g.problem" class="tile-status" :class="'is-' + badge(g).type">
								{{ badge(g).text }}
							</span>
							<div class="tile-name">{{ gameLabel(g) }}</div>
							<div v-if="g.status === 'downloading'" class="tile-progress">
								<i :style="{ width: (g.progress || 0) + '%' }" />
							</div>
						</button>
					</div>
				</section>
			</template>
		</main>

		<footer class="menu-footer">
			<button type="button" class="footer-btn" :disabled="refreshing" title="Làm mới" @click="load(true)">
				<el-icon :class="{ 'is-loading': refreshing }"><Refresh /></el-icon>
			</button>
		</footer>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading, Refresh, Search } from '@element-plus/icons-vue'
import { foldSearch, gameLabel, matchesSearch } from '../utils/gameName'

declare const window: any
const api = () => window.go?.main?.App

interface Game {
	// key: khoá của mục (máy chủ từ 1.1.15). Một thư mục game có thể ra nhiều mục
	// (LibreOffice → Word, Excel), nên name không còn là khoá.
	key?: string
	name: string
	display_name: string
	category: string
	launcher?: string
	launch_args?: string
	status: string
	progress: number
	version?: number
	icon?: string
	// "hot" (nhiều lượt mở gần đây) hoặc "new" (mới về quán).
	label?: string
	// Máy trạm tự đặt khi tệp chạy của mục sẵn sàng không dùng được: "missing_launcher" hoặc "bad_config".
	problem?: string
}

// Khoá của mục: máy chủ cũ không gửi key thì mỗi game là một mục, khoá là tên.
function gid(g: Game): string {
	return g.key || g.name
}

// Tiến độ tải nhích theo từng lượt làm mới, nên đọc lại thường xuyên khi cửa
// sổ đang hiện.
const REFRESH_MS = 10000

const games = ref<Game[]>([])
// Ô tìm game. Cửa sổ dựng lại mỗi lần mở (App.vue khoá theo lanMo) nên khách sau
// không thấy chữ khách trước gõ.
const query = ref('')
const searchInput = ref<{ focus: () => void } | null>(null)

// Nhóm của quán đổi sang tên tab ngắn như menu Kzone; nhóm khác giữ tên quán đặt, in hoa.
const GROUP_NAMES: Record<string, string> = {
	'game online': 'ONLINE',
	online: 'ONLINE',
	'game offline': 'OFFLINE',
	offline: 'OFFLINE',
	'cong cu': 'TOOL',
	tool: 'TOOL',
	tools: 'TOOL',
}
const GROUP_ORDER = ['ONLINE', 'STEAM', 'OFFLINE', 'TOOL']
const NO_GROUP = 'KHÁC'

function groupOf(g: Game): string {
	const c = (g.category || '').trim()
	if (!c) return NO_GROUP
	return GROUP_NAMES[foldSearch(c)] || c.toLocaleUpperCase('vi')
}

// STEAM là tab lọc, không phải nhóm: game Steam vẫn nằm trong nhóm của nó (thường là
// ONLINE). Menu không có trường "nguồn" nên nhận ra qua tên ("Naraka Steam") hoặc lệnh
// chạy (steam.exe, steam://…, -applaunch).
function isSteam(g: Game): boolean {
	const hay = [gameLabel(g), g.name, g.launcher || '', g.launch_args || ''].join(' ').toLowerCase()
	return hay.includes('steam') || hay.includes('-applaunch')
}

// Tab đang chọn ('' = ALL).
const tab = ref('')
const tabs = computed(() => {
	const present = new Set(games.value.map(groupOf))
	const known = GROUP_ORDER.filter(k => (k === 'STEAM' ? games.value.some(isSteam) : present.has(k)))
	const others = [...present]
		.filter(k => !GROUP_ORDER.includes(k) && k !== NO_GROUP)
		.sort((a, b) => a.localeCompare(b, 'vi'))
	return ['', ...known, ...others, ...(present.has(NO_GROUP) ? [NO_GROUP] : [])]
})

// Các nhóm đang hiện, theo thứ tự tab; trong nhóm giữ thứ tự máy chủ trả.
const sections = computed(() => {
	const list = games.value.filter(g => matchesSearch(g, query.value))
	if (tab.value === 'STEAM') {
		const items = list.filter(isSteam)
		return items.length ? [{ name: 'STEAM', items }] : []
	}
	const byGroup = new Map<string, Game[]>()
	for (const g of list) {
		const k = groupOf(g)
		if (tab.value && tab.value !== k) continue
		if (!byGroup.has(k)) byGroup.set(k, [])
		byGroup.get(k)!.push(g)
	}
	return tabs.value.filter(k => byGroup.has(k)).map(k => ({ name: k, items: byGroup.get(k)! }))
})

function clearFilters() {
	query.value = ''
	tab.value = ''
}
// Chỉ lần tải đầu mới che màn hình; các lượt sau cập nhật ngầm, nếu không cả
// lưới nháy mờ mỗi 10 giây.
const loading = ref(true)
const refreshing = ref(false)
const loadError = ref('')
// Khoá của mục đang được mở. Mở game mất vài giây mới thấy cửa sổ, khách hay bấm
// lần hai — và lần hai là hai bản game chạy chồng nhau.
const launching = ref('')

// Icon theo khoá của mục: data URI PNG, hoặc '' khi không có. Giữ qua các lượt làm mới
// để khỏi hỏi lại Go mỗi 10 giây.
const icons = ref<Record<string, string>>({})
// Lần hỏi icon gần nhất của từng game. Hỏi lại khi version/trạng thái đổi (game vừa
// cập nhật hoặc vừa cài xong thì tệp chạy mới có), hoặc khi lần trước không ra icon
// và đã quá ICON_RETRY_MS.
const iconAsked = new Map<string, { key: string; at: number }>()
const ICON_RETRY_MS = 5 * 60 * 1000

let timer: number | undefined
let inflight = false

function cleanError(e: unknown): string {
	return String(e).replace(/^Error:\s*/, '')
}

// force: nút "Làm mới" của khách bỏ qua bản nhớ và hỏi lại máy chủ. Các lượt tự động mỗi 10 giây dùng lại danh sách khi
// version của máy chủ không đổi, nên nhẹ hơn nhiều khi phòng máy đông.
async function load(force = false) {
	if (inflight) return
	inflight = true
	refreshing.value = true
	try {
		const call = force === true && typeof api()?.RefreshGames === 'function' ? api().RefreshGames : api().ListGames
		const data = JSON.parse(await call())
		games.value = Array.isArray(data?.items) ? data.items : []
		loadError.value = ''
		syncIcons(games.value)
		void loadBranding()
	} catch (e) {
		loadError.value = cleanError(e) || 'Không tải được danh sách game'
	} finally {
		inflight = false
		refreshing.value = false
		loading.value = false
	}
}

// Tên quán và logo ở đầu menu. Hỏi cùng nhịp với danh sách (Go dùng lại bản nhớ, logo chỉ tải một lần), lỗi thì giữ cái cũ.
const branding = ref({ name: '', logo: '' })

async function loadBranding() {
	if (typeof api()?.MenuBranding !== 'function') return
	try {
		const b = JSON.parse(await api().MenuBranding())
		branding.value = { name: typeof b?.name === 'string' ? b.name : '', logo: typeof b?.logo === 'string' ? b.logo : '' }
	} catch {
		// Giữ tên và logo đang hiện.
	}
}

function syncIcons(list: Game[]) {
	// Game bị gỡ khỏi menu thì bỏ icon của nó, đừng giữ ảnh vô thời hạn.
	const names = new Set(list.map(gid))
	for (const n of Object.keys(icons.value)) if (!names.has(n)) delete icons.value[n]
	for (const n of [...iconAsked.keys()]) if (!names.has(n)) iconAsked.delete(n)

	// Ngoài Wails (npm run dev) không có GameIcon: khỏi ghi là đã hỏi.
	if (typeof api()?.GameIcon !== 'function') return
	const now = Date.now()
	for (const g of list) {
		// Đổi tệp icon ở trang quản trị không đổi version: đưa nó vào khoá để hỏi lại.
		const key = `${g.version ?? 0}:${g.status === 'ready'}:${g.icon ?? ''}`
		const id = gid(g)
		const asked = iconAsked.get(id)
		if (asked && asked.key === key && (icons.value[id] || now - asked.at < ICON_RETRY_MS)) continue
		iconAsked.set(id, { key, at: now })
		loadIcon(id)
	}
}

async function loadIcon(id: string) {
	let uri = ''
	try {
		uri = await api().GameIcon(id)
	} catch {
		// Lỗi thì cứ để chữ cái đầu; lần thử lại sau ICON_RETRY_MS.
	}
	// Game có thể đã rời menu trong lúc chờ.
	if (iconAsked.has(id)) icons.value[id] = typeof uri === 'string' ? uri : ''
}

function initial(g: Game): string {
	// NFC để chữ có dấu tách rời (e + dấu mũ) gộp lại thành một ký tự; \p{L} khớp
	// theo điểm mã nên không cắt đôi cặp thay thế.
	const m = gameLabel(g).normalize('NFC').match(/[\p{L}\p{N}]/u)
	return m ? Array.from(m[0].toUpperCase())[0] : '?'
}

// Màu riêng của mỗi mục suy từ khoá (không đổi khi sửa tên hiển thị): cùng mục luôn
// cùng màu, các mục khác nhau thì tản đều quanh vòng màu.
function hue(g: Game): number {
	const id = gid(g)
	let h = 2166136261
	for (let i = 0; i < id.length; i++) h = Math.imul(h ^ id.charCodeAt(i), 16777619)
	return (h >>> 0) % 360
}

function placeholderStyle(g: Game) {
	const h = hue(g)
	return { background: `linear-gradient(135deg, hsl(${h} 55% 46%), hsl(${(h + 40) % 360} 55% 30%))` }
}

// Chỉ "ready" mới chơi được. Mọi trạng thái còn lại (queued, publishing,
// not_installed, và cả giá trị lạ máy chủ thêm sau này) đều là "chờ".
function badge(g: Game): { type: 'success' | 'primary' | 'info' | 'danger'; text: string } {
	// Máy chủ báo sẵn sàng nhưng tệp chạy không dùng được trên ổ game của máy này: nói rõ lý do thay vì để khách bấm rồi lỗi.
	if (g.status === 'ready' && g.problem) {
		return { type: 'danger', text: g.problem === 'missing_launcher' ? 'Thiếu tệp chạy' : 'Cấu hình lỗi' }
	}
	switch (g.status) {
		case 'ready': return { type: 'success', text: 'Sẵn sàng' }
		// Ngắn để vừa ô nhỏ.
		case 'downloading': return { type: 'primary', text: `Cập nhật ${g.progress}%` }
		case 'error': return { type: 'danger', text: 'Lỗi' }
		default: return { type: 'info', text: 'Chờ cập nhật' }
	}
}

async function play(g: Game) {
	if (launching.value) return
	launching.value = gid(g)
	try {
		// Chỉ gửi KHOÁ: Go tra lại danh sách của máy chủ để dựng đường dẫn.
		await api().LaunchGame(gid(g))
	} catch (e) {
		ElMessage.error(cleanError(e))
		launching.value = ''
		return
	}
	// Ẩn cửa sổ để game hiện lên trước mặt, thay vì nằm sau cửa sổ này.
	api()?.HidePanel?.()
	setTimeout(() => { launching.value = '' }, 3000)
}

// Gõ một phím chữ/số khi đang không ở ô nhập nào thì chuyển vào ô tìm: phím đó
// được gõ luôn vào ô, khách khỏi phải bấm chuột vào ô trước.
function onKeydown(e: KeyboardEvent) {
	if (e.key.length !== 1 || e.ctrlKey || e.altKey || e.metaKey) return
	const t = e.target as HTMLElement | null
	if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
	searchInput.value?.focus()
}

// Cửa sổ bị ẩn (bấm X, hoặc vừa mở game) thì thôi hỏi máy chủ; hiện lại thì hỏi
// ngay chứ không chờ hết nhịp.
function onVisibility() {
	if (document.visibilityState === 'visible') load()
}

onMounted(async () => {
	timer = window.setInterval(() => {
		if (document.visibilityState === 'visible') load()
	}, REFRESH_MS)
	document.addEventListener('visibilitychange', onVisibility)
	window.addEventListener('keydown', onKeydown)
	// Ô tìm chỉ hiện khi đã có danh sách: chờ lượt tải đầu rồi mới đặt con trỏ vào.
	await load()
	await nextTick()
	searchInput.value?.focus()
})

onUnmounted(() => {
	if (timer) clearInterval(timer)
	document.removeEventListener('visibilitychange', onVisibility)
	window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.game-menu {
	height: 100vh;
	display: flex;
	flex-direction: column;
	background:
		radial-gradient(900px 420px at 0% 0%, rgba(123, 92, 255, 0.12), transparent 60%),
		radial-gradient(900px 500px at 100% 100%, rgba(76, 125, 255, 0.1), transparent 60%),
		var(--vnet-bg);
}

/* Ba cột: tên quán bên trái, tab ở GIỮA cửa sổ, ô tìm bên phải. */
.menu-header {
	display: grid;
	grid-template-columns: 1fr auto 1fr;
	align-items: center;
	gap: 16px;
	padding: 12px 24px;
	background: rgba(17, 24, 51, 0.55);
	border-bottom: 1px solid var(--vnet-border);
	box-shadow: 0 1px 0 rgba(76, 125, 255, 0.18);
}

.menu-brand {
	display: flex;
	align-items: center;
	gap: 10px;
	min-width: 0;
}

.menu-logo {
	height: 32px;
	max-width: 96px;
	object-fit: contain;
	border-radius: 6px;
}

.menu-brand h1 {
	min-width: 0;
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
	font-family: var(--vnet-font-display);
	font-size: 22px;
	font-weight: 700;
	letter-spacing: 0.04em;
	text-transform: uppercase;
	background: var(--vnet-brand);
	-webkit-background-clip: text;
	background-clip: text;
	color: transparent;
}

.menu-tabs {
	display: flex;
	gap: 4px;
}

.tab {
	padding: 7px 20px;
	border: 1px solid transparent;
	border-radius: var(--vnet-radius-sm);
	background: transparent;
	color: var(--vnet-text-muted);
	font: inherit;
	font-size: 13px;
	font-weight: 600;
	letter-spacing: 0.04em;
	white-space: nowrap;
	cursor: pointer;
	transition: color 0.15s, background 0.15s, border-color 0.15s;
}

.tab:hover {
	color: var(--vnet-text);
}

.tab.active {
	color: var(--vnet-text);
	border-color: var(--vnet-border);
	background: linear-gradient(180deg, var(--vnet-surface-2), var(--vnet-surface));
	box-shadow: 0 4px 14px rgba(3, 6, 16, 0.5), inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.menu-search {
	justify-self: end;
	width: 220px;
}

.menu-search :deep(.el-input__wrapper) {
	border-radius: var(--vnet-radius-sm);
	background: var(--vnet-surface);
}

/* Cửa sổ hẹp (màn 1366): tab xuống hàng riêng thay vì chen giữa tên quán và ô tìm. */
@media (max-width: 960px) {
	.menu-header {
		grid-template-columns: 1fr auto;
	}

	.menu-tabs {
		grid-row: 2;
		grid-column: 1 / -1;
		justify-self: center;
	}
}

.load-error {
	margin: 8px 24px 0;
	padding: 6px 12px;
	border-radius: var(--vnet-radius-sm);
	font-size: 12px;
	color: var(--vnet-warning);
	background: rgba(255, 181, 71, 0.1);
}

.menu-body {
	flex: 1;
	min-height: 0;
	overflow-y: auto;
	padding: 4px 24px 16px;
}

.empty {
	height: 100%;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 12px;
	color: var(--vnet-text-muted);
}

.group-title {
	margin: 14px 0 14px;
	padding-bottom: 6px;
	border-bottom: 1px solid rgba(37, 48, 86, 0.7);
	font-size: 12px;
	font-weight: 700;
	letter-spacing: 0.06em;
	color: var(--vnet-text);
}

.group-title span {
	margin-left: 4px;
	font-weight: 600;
	color: var(--vnet-text-faint);
}

.game-grid {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
	gap: 12px;
}

/* Ô vuông nhỏ: icon ở giữa phía trên, tên tối đa hai dòng bên dưới. Không cắt tràn
   (overflow) vì nhãn HOT nhô lên khỏi mép trên. */
.tile {
	position: relative;
	display: flex;
	flex-direction: column;
	align-items: center;
	padding: 10px 6px 6px;
	border: 1px solid rgba(37, 48, 86, 0.8);
	border-radius: var(--vnet-radius-sm);
	background: var(--vnet-surface);
	color: inherit;
	font: inherit;
	cursor: default;
	transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
}

.tile.ready {
	cursor: pointer;
}

.tile.ready:hover,
.tile.ready:focus-visible,
.tile.busy {
	background: var(--vnet-surface-2);
	border-color: rgba(76, 125, 255, 0.7);
	box-shadow: 0 6px 18px rgba(3, 6, 16, 0.55), 0 0 0 1px rgba(76, 125, 255, 0.25);
	outline: none;
}

/* Game chưa sẵn sàng vẫn hiện: khách cần biết quán có game đó, chỉ là chưa
   chơi được. */
.tile:not(.ready) .tile-icon {
	filter: grayscale(0.85) brightness(0.7);
}

.tile:not(.ready) .tile-name {
	color: var(--vnet-text-faint);
}

.tile-icon {
	position: relative;
	flex: none;
	width: 48px;
	height: 48px;
	border-radius: 10px;
	overflow: hidden;
	display: flex;
	align-items: center;
	justify-content: center;
	transition: transform 0.15s ease;
}

.tile.ready:hover .tile-icon,
.tile.ready:focus-visible .tile-icon {
	transform: scale(1.08);
}

.tile-icon img {
	width: 100%;
	height: 100%;
	object-fit: contain;
	/* Ảnh là trang trí: không cho kéo thả ra khỏi ô. */
	user-select: none;
	pointer-events: none;
}

.game-initial {
	font-family: var(--vnet-font-display);
	font-size: 22px;
	font-weight: 700;
	line-height: 1;
	color: #fff;
	text-shadow: 0 1px 2px rgba(0, 0, 0, 0.35);
}

/* Đang mở game: vòng xoay phủ lên icon. */
.tile-opening {
	position: absolute;
	inset: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	font-size: 22px;
	color: #fff;
	background: rgba(10, 15, 30, 0.6);
}

.tile-name {
	width: 100%;
	margin-top: 6px;
	font-size: 12px;
	font-weight: 500;
	line-height: 1.25;
	min-height: 2.5em;
	text-align: center;
	/* Tên không có khoảng trắng (slug dài) cũng phải xuống dòng. */
	overflow-wrap: anywhere;
	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
	overflow: hidden;
}

/* Nhãn HOT / MỚI: viên nhỏ nhô lên ở góc trên phải (chỉ khi game sẵn sàng và không có vấn đề). */
.tile-label {
	position: absolute;
	top: -7px;
	right: 6px;
	z-index: 1;
	padding: 1px 7px;
	border-radius: 999px;
	font-size: 9px;
	font-weight: 700;
	letter-spacing: 0.04em;
	line-height: 14px;
	color: #fff;
	background: linear-gradient(135deg, rgb(255, 85, 110), rgb(220, 40, 70));
	box-shadow: 0 2px 6px rgba(220, 40, 70, 0.4);
}

.tile-label.is-new {
	background: linear-gradient(135deg, rgb(0, 188, 255), rgb(0, 130, 230));
	box-shadow: 0 2px 6px rgba(0, 130, 230, 0.4);
}

/* Trạng thái chưa chơi được: dải nhỏ đè lên mép dưới icon. */
.tile-status {
	position: absolute;
	top: 48px;
	left: 50%;
	max-width: calc(100% - 4px);
	transform: translateX(-50%);
	padding: 1px 4px;
	border-radius: 999px;
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
	font-size: 9.5px;
	font-weight: 600;
	line-height: 14px;
	color: var(--vnet-text-muted);
	background: rgba(10, 15, 30, 0.88);
	border: 1px solid var(--vnet-border);
}

.tile-status.is-primary {
	color: var(--vnet-primary);
}

.tile-status.is-danger {
	color: var(--vnet-danger);
}

.tile-progress {
	position: absolute;
	left: 6px;
	right: 6px;
	bottom: 3px;
	height: 3px;
	border-radius: 2px;
	overflow: hidden;
	background: rgba(255, 255, 255, 0.08);
}

.tile-progress i {
	display: block;
	height: 100%;
	background: var(--vnet-brand);
	transition: width 0.4s ease;
}

/* Dải dưới cùng: nút làm mới ở góc phải. */
.menu-footer {
	display: flex;
	justify-content: flex-end;
	align-items: center;
	padding: 4px 16px;
	border-top: 1px solid rgba(37, 48, 86, 0.6);
	background: rgba(17, 24, 51, 0.55);
}

.footer-btn {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 28px;
	height: 28px;
	border: none;
	border-radius: 6px;
	background: transparent;
	color: var(--vnet-text-muted);
	font-size: 16px;
	cursor: pointer;
	transition: color 0.15s, background 0.15s;
}

.footer-btn:hover:not(:disabled) {
	color: var(--vnet-text);
	background: var(--vnet-surface-2);
}

.footer-btn:disabled {
	cursor: default;
}
</style>
