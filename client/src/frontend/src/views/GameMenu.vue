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

		<main v-loading="loading" class="menu-body">
			<div v-if="!loading && !games.length" class="empty">
				<template v-if="loadError">
					<p>{{ loadError }}</p>
					<el-button @click="load">Thử lại</el-button>
				</template>
				<p v-else>Quán chưa có game nào</p>
			</div>

			<div v-else class="game-grid">
				<article v-for="g in games" :key="g.name" class="game-card" :class="{ ready: g.status === 'ready' }">
					<div class="game-info">
						<div class="game-name" :title="g.display_name">{{ g.display_name }}</div>
						<div v-if="g.category" class="game-category">{{ g.category }}</div>
					</div>
					<div class="game-foot">
						<el-tag :type="badge(g).type" effect="plain" round size="small">{{ badge(g).text }}</el-tag>
						<el-button
							type="primary"
							:disabled="g.status !== 'ready' || (!!launching && launching !== g.name)"
							:loading="launching === g.name"
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
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'

declare const window: any
const api = () => window.go?.main?.App

interface Game {
	name: string
	display_name: string
	category: string
	status: string
	progress: number
}

// Tiến độ tải nhích theo từng lượt làm mới, nên đọc lại thường xuyên khi cửa
// sổ đang hiện.
const REFRESH_MS = 10000

const games = ref<Game[]>([])
// Chỉ lần tải đầu mới che màn hình; các lượt sau cập nhật ngầm, nếu không cả
// lưới nháy mờ mỗi 10 giây.
const loading = ref(true)
const refreshing = ref(false)
const loadError = ref('')
// Tên game đang được mở. Mở game mất vài giây mới thấy cửa sổ, khách hay bấm
// lần hai — và lần hai là hai bản game chạy chồng nhau.
const launching = ref('')

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
	} catch (e) {
		loadError.value = cleanError(e) || 'Không tải được danh sách game'
	} finally {
		inflight = false
		refreshing.value = false
		loading.value = false
	}
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
	launching.value = g.name
	try {
		// Chỉ gửi TÊN: Go tra lại danh sách của máy chủ để dựng đường dẫn.
		await api().LaunchGame(g.name)
	} catch (e) {
		ElMessage.error(cleanError(e))
		launching.value = ''
		return
	}
	// Ẩn cửa sổ để game hiện lên trước mặt, thay vì nằm sau cửa sổ này.
	api()?.HidePanel?.()
	setTimeout(() => { launching.value = '' }, 3000)
}

// Cửa sổ bị ẩn (bấm X, hoặc vừa mở game) thì thôi hỏi máy chủ; hiện lại thì hỏi
// ngay chứ không chờ hết nhịp.
function onVisibility() {
	if (document.visibilityState === 'visible') load()
}

onMounted(() => {
	load()
	timer = window.setInterval(() => {
		if (document.visibilityState === 'visible') load()
	}, REFRESH_MS)
	document.addEventListener('visibilitychange', onVisibility)
})

onUnmounted(() => {
	if (timer) clearInterval(timer)
	document.removeEventListener('visibilitychange', onVisibility)
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
	grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
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

.game-name {
	font-size: 15px;
	font-weight: 600;
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
