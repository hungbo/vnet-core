<template>
	<div class="game-menu">
		<header class="menu-header">
			<span class="header-spacer" />
			<h3>Game</h3>
			<div class="header-spacer">
				<el-button text circle :loading="refreshing" title="Làm mới" @click="load">
					<el-icon><Refresh /></el-icon>
				</el-button>
			</div>
		</header>

		<!-- Mất mạng giữa chừng thì giữ danh sách cũ và báo một dòng, đừng xoá
		     trắng màn hình đang có game chơi được. -->
		<div v-if="loadError && games.length" class="load-error">{{ loadError }} — đang thử lại</div>

		<!-- Tìm không dấu, theo nhóm hoặc chữ cái đầu ("lmht"). Gõ phím bất kỳ ở đâu
		     trong cửa sổ cũng vào ô này; Esc xoá. -->
		<div v-if="games.length" class="search-bar">
			<el-input
				ref="searchInput"
				v-model="query"
				clearable
				placeholder="Tìm game"
				:prefix-icon="Search"
				@keydown.esc="query = ''"
			/>
		</div>

		<main v-loading="loading" class="menu-body">
			<div v-if="!loading && !games.length" class="empty">
				<template v-if="loadError">
					<p>{{ loadError }}</p>
					<el-button @click="load">Thử lại</el-button>
				</template>
				<p v-else>Quán chưa có game nào</p>
			</div>

			<div v-else-if="!shown.length" class="empty">
				<p>Không có game nào khớp "{{ query.trim() }}"</p>
				<el-button @click="query = ''">Xoá tìm kiếm</el-button>
			</div>

			<div v-else class="game-grid">
				<article v-for="g in shown" :key="gid(g)" class="game-card" :class="{ ready: g.status === 'ready' }">
					<div class="game-head">
						<!-- Không có icon (chưa lấy được, hoặc tệp chạy không có) thì vẽ chữ cái đầu
						     trên nền màu suy ra từ tên, để thẻ nào cũng có hình mà không phải đợi. -->
						<div class="game-icon" :style="icons[gid(g)] ? undefined : placeholderStyle(g)">
							<img
								v-if="icons[gid(g)]"
								:src="icons[gid(g)]"
								alt=""
								draggable="false"
								@error="icons[gid(g)] = ''"
							/>
							<span v-else class="game-initial" aria-hidden="true">{{ initial(g) }}</span>
						</div>
						<div class="game-info">
							<div class="game-name" :title="gameLabel(g)">{{ gameLabel(g) }}</div>
							<div v-if="g.category" class="game-category">{{ g.category }}</div>
						</div>
					</div>
					<div class="game-foot">
						<el-tag :type="badge(g).type" effect="plain" round size="small">{{ badge(g).text }}</el-tag>
						<el-button
							type="primary"
							:disabled="g.status !== 'ready' || (!!launching && launching !== gid(g))"
							:loading="launching === gid(g)"
							@click="play(g)"
						>
							Chơi
						</el-button>
					</div>
				</article>
			</div>
		</main>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
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
const shown = computed(() => games.value.filter(g => matchesSearch(g, query.value)))
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

async function load() {
	if (inflight) return
	inflight = true
	refreshing.value = true
	try {
		const data = JSON.parse(await api().ListGames())
		games.value = Array.isArray(data?.items) ? data.items : []
		loadError.value = ''
		syncIcons(games.value)
	} catch (e) {
		loadError.value = cleanError(e) || 'Không tải được danh sách game'
	} finally {
		inflight = false
		refreshing.value = false
		loading.value = false
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

// Màu nền chữ cái đầu suy từ khoá (không đổi khi sửa tên hiển thị): cùng mục luôn
// cùng màu, các mục khác nhau thì tản đều quanh vòng màu.
function placeholderStyle(g: Game) {
	const id = gid(g)
	let h = 2166136261
	for (let i = 0; i < id.length; i++) h = Math.imul(h ^ id.charCodeAt(i), 16777619)
	const hue = (h >>> 0) % 360
	return { background: `linear-gradient(135deg, hsl(${hue} 55% 46%), hsl(${(hue + 40) % 360} 55% 30%))` }
}

// Chỉ "ready" mới chơi được. Mọi trạng thái còn lại (queued, publishing,
// not_installed, và cả giá trị lạ máy chủ thêm sau này) đều là "chờ".
function badge(g: Game): { type: 'success' | 'primary' | 'info' | 'danger'; text: string } {
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
	background: var(--vnet-bg);
}

.menu-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 12px 16px;
	background: var(--vnet-surface);
	box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.menu-header h3 {
	font-size: 16px;
	font-weight: 600;
}

/* Hai đầu cùng bề rộng để tiêu đề nằm đúng giữa; nút làm mới dựa phải. */
.header-spacer {
	width: 40px;
	display: flex;
	justify-content: flex-end;
}

.search-bar {
	padding: 12px 16px 0;
}

.load-error {
	padding: 6px 16px;
	font-size: 12px;
	color: var(--vnet-warning);
	background: var(--vnet-surface);
}

.menu-body {
	flex: 1;
	min-height: 0;
	overflow-y: auto;
	padding: 16px;
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
	grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
	gap: 12px;
	/* Lưới cao hết cửa sổ thì hàng duy nhất giãn theo; giữ ô cao đúng bằng nội dung. */
	align-content: start;
}

.game-card {
	display: flex;
	flex-direction: column;
	justify-content: space-between;
	gap: 14px;
	padding: 16px;
	border: 1px solid var(--vnet-border);
	border-radius: var(--vnet-radius);
	background: var(--vnet-surface);
	/* Game chưa sẵn sàng vẫn hiện: khách cần biết quán có game đó, chỉ là
	   chưa chơi được. */
	opacity: 0.7;
	transition: border-color 0.15s, opacity 0.15s;
}

.game-card.ready {
	opacity: 1;
}

.game-card.ready:hover {
	border-color: var(--vnet-primary);
}

.game-head {
	display: flex;
	align-items: center;
	gap: 12px;
}

.game-icon {
	flex: none;
	width: 56px;
	height: 56px;
	border-radius: 12px;
	overflow: hidden;
	display: flex;
	align-items: center;
	justify-content: center;
	background: var(--vnet-surface-2);
}

.game-icon img {
	width: 100%;
	height: 100%;
	object-fit: contain;
	/* Ảnh là trang trí: không cho kéo thả ra khỏi thẻ. */
	user-select: none;
	pointer-events: none;
}

.game-initial {
	font-family: var(--vnet-font-display);
	font-size: 26px;
	font-weight: 700;
	line-height: 1;
	color: #fff;
	text-shadow: 0 1px 2px rgba(0, 0, 0, 0.35);
}

/* min-width: 0 để cột tên co lại được trong hàng flex, nếu không tên dài đẩy
   thẻ tràn ra ngoài lưới. */
.game-info {
	flex: 1;
	min-width: 0;
}

.game-name {
	font-size: 15px;
	font-weight: 600;
	/* Tên không có khoảng trắng (slug dài) cũng phải xuống dòng. */
	overflow-wrap: anywhere;
	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
	overflow: hidden;
}

.game-category {
	margin-top: 4px;
	font-size: 12px;
	color: var(--vnet-text-muted);
}

.game-foot {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
}
</style>
