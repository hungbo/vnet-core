<template>
	<div class="game-menu">
		<header class="menu-header">
			<div class="menu-title">
				<!-- Logo và tên quán do quán đặt ở trang quản trị; thiếu thì chỉ còn chữ "Game". -->
				<img v-if="branding.logo" class="menu-logo" :src="branding.logo" alt="" />
				<h3>{{ branding.name || 'Game' }}</h3>
				<span v-if="games.length" class="menu-count">{{ games.length }} game</span>
			</div>
			<!-- Tìm không dấu, theo nhóm hoặc chữ cái đầu ("lmht"). Gõ phím bất kỳ ở đâu
			     trong cửa sổ cũng vào ô này; Esc xoá. -->
			<el-input
				v-if="games.length"
				ref="searchInput"
				v-model="query"
				class="menu-search"
				clearable
				placeholder="Tìm game"
				:prefix-icon="Search"
				@keydown.esc="query = ''"
			/>
			<el-button text circle :loading="refreshing" title="Làm mới" @click="load(true)">
				<el-icon><Refresh /></el-icon>
			</el-button>
		</header>

		<!-- Lọc theo nhóm: chỉ hiện khi quán chia game thành từ hai nhóm trở lên. -->
		<nav v-if="categories.length > 1" class="menu-chips">
			<button type="button" class="chip" :class="{ active: !category }" @click="category = ''">Tất cả</button>
			<button
				v-for="c in categories"
				:key="c"
				type="button"
				class="chip"
				:class="{ active: category === c }"
				@click="category = category === c ? '' : c"
			>
				{{ c }}
			</button>
		</nav>

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

			<div v-else-if="!shown.length" class="empty">
				<p>Không có game nào khớp "{{ query.trim() || category }}"</p>
				<el-button @click="clearFilters">Xoá bộ lọc</el-button>
			</div>

			<div v-else class="game-grid">
				<!-- Cả ô là nút: bấm vào đâu trên ô cũng mở game, và Tab/Enter dùng được. -->
				<button
					v-for="g in shown"
					:key="gid(g)"
					type="button"
					class="tile"
					:class="{ ready: g.status === 'ready' && !g.problem, busy: launching === gid(g) }"
					:disabled="g.status !== 'ready' || !!g.problem || (!!launching && launching !== gid(g))"
					:title="gameLabel(g)"
					@click="play(g)"
				>
					<div class="tile-art" :style="artStyle(g)">
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
						</div>
						<span v-if="g.status !== 'ready' || g.problem" class="tile-status" :class="'is-' + badge(g).type">
							{{ badge(g).text }}
						</span>
						<span v-else-if="g.label" class="tile-label" :class="'is-' + g.label">
							{{ g.label === 'hot' ? 'Hot' : 'Mới' }}
						</span>
						<div v-if="g.status === 'ready' && !g.problem" class="tile-play">
							<el-icon v-if="launching !== gid(g)"><VideoPlay /></el-icon>
							<el-icon v-else class="is-loading"><Loading /></el-icon>
							<span>{{ launching === gid(g) ? 'Đang mở…' : 'Chơi' }}</span>
						</div>
						<div v-if="g.status === 'downloading'" class="tile-progress">
							<i :style="{ width: (g.progress || 0) + '%' }" />
						</div>
					</div>
					<div class="tile-body">
						<div class="tile-name">{{ gameLabel(g) }}</div>
						<div class="tile-category">{{ g.category || '\u00a0' }}</div>
					</div>
				</button>
			</div>
		</main>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading, Refresh, Search, VideoPlay } from '@element-plus/icons-vue'
import { gameLabel, matchesSearch } from '../utils/gameName'

declare const window: any
const api = () => window.go?.main?.App

interface Game {
	// key: khoá của mục (máy chủ từ 1.1.15). Một thư mục game có thể ra nhiều mục
	// (LibreOffice → Word, Excel), nên name không còn là khoá.
	key?: string
	name: string
	display_name: string
	category: string
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
// Nhóm đang lọc ('' = tất cả). Nhóm lấy từ chính danh sách game của quán.
const category = ref('')
const categories = computed(() =>
	[...new Set(games.value.map(g => (g.category || '').trim()).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'vi')),
)
const shown = computed(() =>
	games.value.filter(g => matchesSearch(g, query.value) && (!category.value || (g.category || '').trim() === category.value)),
)

function clearFilters() {
	query.value = ''
	category.value = ''
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

// Nền phía sau icon: vầng sáng cùng màu của mục, để lưới có màu sắc mà icon
// vẫn nổi.
function artStyle(g: Game) {
	const h = hue(g)
	return {
		background: `radial-gradient(circle at 50% 45%, hsl(${h} 70% 50% / 0.38), transparent 68%), linear-gradient(160deg, hsl(${h} 45% 22%), var(--vnet-surface-2))`,
	}
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
		case 'downloading': return { type: 'primary', text: `Đang cập nhật ${g.progress}%` }
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
		radial-gradient(1200px 400px at 10% -10%, rgba(76, 125, 255, 0.16), transparent 60%),
		radial-gradient(900px 360px at 100% 0%, rgba(123, 92, 255, 0.12), transparent 60%),
		var(--vnet-bg);
}

.menu-header {
	display: flex;
	align-items: center;
	gap: 16px;
	padding: 16px 20px 12px;
}

.menu-title {
	display: flex;
	align-items: baseline;
	gap: 10px;
	margin-right: auto;
}

.menu-logo {
	align-self: center;
	height: 32px;
	max-width: 96px;
	object-fit: contain;
	border-radius: 6px;
}

.menu-title h3 {
	font-family: var(--vnet-font-display);
	font-size: 24px;
	font-weight: 700;
	letter-spacing: 0.02em;
	background: var(--vnet-brand);
	-webkit-background-clip: text;
	background-clip: text;
	color: transparent;
}

.menu-count {
	font-size: 13px;
	color: var(--vnet-text-muted);
}

.menu-search {
	width: 320px;
}

.menu-search :deep(.el-input__wrapper) {
	border-radius: 999px;
	background: var(--vnet-surface);
}

.menu-chips {
	display: flex;
	flex-wrap: wrap;
	gap: 8px;
	padding: 0 20px 8px;
}

.chip {
	padding: 5px 14px;
	border: 1px solid var(--vnet-border);
	border-radius: 999px;
	background: var(--vnet-surface);
	color: var(--vnet-text-muted);
	font: inherit;
	font-size: 13px;
	cursor: pointer;
	transition: color 0.15s, border-color 0.15s, background 0.15s;
}

.chip:hover {
	color: var(--vnet-text);
	border-color: var(--vnet-primary);
}

.chip.active {
	color: #fff;
	border-color: transparent;
	background: var(--vnet-brand);
}

.load-error {
	margin: 0 20px 8px;
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
	padding: 8px 20px 20px;
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

.game-grid {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
	gap: 16px;
	/* Lưới cao hết cửa sổ thì hàng duy nhất giãn theo; giữ ô cao đúng bằng nội dung. */
	align-content: start;
}

.tile {
	display: flex;
	flex-direction: column;
	padding: 0;
	border: 1px solid var(--vnet-border);
	border-radius: 14px;
	overflow: hidden;
	background: var(--vnet-surface);
	color: inherit;
	font: inherit;
	text-align: left;
	cursor: default;
	transition: transform 0.18s ease, border-color 0.18s, box-shadow 0.18s;
}

.tile.ready {
	cursor: pointer;
}

.tile.ready:hover,
.tile.ready:focus-visible,
.tile.busy {
	transform: translateY(-4px);
	border-color: var(--vnet-primary);
	box-shadow: 0 12px 28px rgba(3, 6, 16, 0.6), 0 0 0 1px rgba(76, 125, 255, 0.35);
	outline: none;
}

/* Game chưa sẵn sàng vẫn hiện: khách cần biết quán có game đó, chỉ là chưa
   chơi được. */
.tile:not(.ready) .tile-icon {
	filter: grayscale(0.85) brightness(0.75);
}

.tile-art {
	position: relative;
	aspect-ratio: 16 / 11;
	display: flex;
	align-items: center;
	justify-content: center;
}

.tile-icon {
	width: 76px;
	height: 76px;
	border-radius: 18px;
	overflow: hidden;
	display: flex;
	align-items: center;
	justify-content: center;
	filter: drop-shadow(0 6px 14px rgba(0, 0, 0, 0.45));
	transition: transform 0.18s ease;
}

.tile.ready:hover .tile-icon,
.tile.ready:focus-visible .tile-icon,
.tile.busy .tile-icon {
	transform: translateY(-12px) scale(1.04);
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
	font-size: 34px;
	font-weight: 700;
	line-height: 1;
	color: #fff;
	text-shadow: 0 1px 2px rgba(0, 0, 0, 0.35);
}

.tile-status {
	position: absolute;
	top: 8px;
	left: 8px;
	padding: 2px 8px;
	border-radius: 999px;
	font-size: 11px;
	font-weight: 600;
	color: var(--vnet-text-muted);
	background: rgba(10, 15, 30, 0.72);
}

.tile-status.is-primary {
	color: var(--vnet-primary);
}

.tile-status.is-danger {
	color: var(--vnet-danger);
}

/* Nhãn Hot / Mới: cùng chỗ với trạng thái (chỉ hiện khi game sẵn sàng và không có vấn đề). */
.tile-label {
	position: absolute;
	top: 8px;
	left: 8px;
	padding: 2px 8px;
	border-radius: 999px;
	font-size: 11px;
	font-weight: 700;
	color: #fff;
	background: linear-gradient(135deg, rgb(255, 105, 135), rgb(255, 60, 90));
}

.tile-label.is-new {
	background: linear-gradient(135deg, rgb(0, 188, 255), rgb(0, 130, 230));
}

/* Nút Chơi hiện ở đáy hình khi rê chuột (icon nhích lên nhường chỗ): hình
   thoáng lúc xem, rõ ràng lúc chọn. */
.tile-play {
	position: absolute;
	left: 50%;
	bottom: 10px;
	display: flex;
	align-items: center;
	gap: 6px;
	padding: 5px 16px;
	border-radius: 999px;
	white-space: nowrap;
	font-family: var(--vnet-font-display);
	font-size: 14px;
	font-weight: 700;
	color: #fff;
	background: var(--vnet-brand);
	box-shadow: 0 6px 16px rgba(76, 125, 255, 0.45);
	opacity: 0;
	transform: translate(-50%, 6px);
	transition: opacity 0.18s, transform 0.18s;
}

.tile-play .el-icon {
	font-size: 16px;
}

.tile.ready:hover .tile-play,
.tile.ready:focus-visible .tile-play,
.tile.busy .tile-play {
	opacity: 1;
	transform: translate(-50%, 0);
}

.tile-progress {
	position: absolute;
	left: 0;
	right: 0;
	bottom: 0;
	height: 4px;
	background: rgba(255, 255, 255, 0.08);
}

.tile-progress i {
	display: block;
	height: 100%;
	background: var(--vnet-brand);
	transition: width 0.4s ease;
}

/* min-width: 0 để tên dài co lại được trong ô, không đẩy ô tràn khỏi lưới. */
.tile-body {
	min-width: 0;
	padding: 10px 12px 12px;
}

.tile-name {
	font-size: 14px;
	font-weight: 600;
	line-height: 1.35;
	min-height: 2.7em;
	/* Tên không có khoảng trắng (slug dài) cũng phải xuống dòng. */
	overflow-wrap: anywhere;
	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
	overflow: hidden;
}

.tile-category {
	margin-top: 2px;
	font-size: 12px;
	color: var(--vnet-text-muted);
}
</style>
