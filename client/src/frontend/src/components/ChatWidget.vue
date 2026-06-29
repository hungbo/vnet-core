<template>
	<!-- Full Page Mode -->
	<div v-if="fullPage" class="chat-fullpage">
		<header class="chat-header">
			<el-button text @click="$emit('back')">
				<el-icon>
					<ArrowLeft />
				</el-icon>
			</el-button>
			<h3>Hỗ trợ</h3>
		</header>
		<vue-advanced-chat v-show="!showClearedUI" :current-user-id="currentUserId" :rooms="roomsJson"
			:messages="messagesJson" :room-id="store.roomId" :messages-loaded="messagesLoaded" :show-add-room="false"
			:show-search="false" :show-files="false" :show-audio="false" :show-emojis="false"
			:show-reaction-emojis="false" :show-new-messages-divider="false" :show-rooms="false"
			:auto-scroll="autoScrollJson" :styles="stylesJson" rooms-loaded height="100%"
			@fetch-messages="onFetchMessages" @send-message="onSendMessage" />
		<div v-show="showClearedUI" class="cleared-overlay">
			<p>Tất cả phòng đã được xoá</p>
			<div class="cleared-actions">
				<el-button type="primary" @click="onCreateNewRoom">+ Tạo phòng mới</el-button>
				<el-button @click="onBackToMain">← Trở về màn hình chính</el-button>
			</div>
		</div>
	</div>

	<!-- Floating Mode -->
	<teleport to="body" v-else>
		<div v-if="isOpen" class="chat-widget-overlay" @click.self="isOpen = false" />
		<div v-if="isOpen" class="chat-widget-panel" :style="{ left: posX + 'px', top: posY + 'px' }">
			<div class="chat-widget-handle" @mousedown="startDrag">
				<span>Chat hỗ trợ</span>
				<el-button size="small" text @click="isOpen = false" style="cursor:pointer">─</el-button>
			</div>
			<vue-advanced-chat v-show="!showClearedUI" :current-user-id="currentUserId" :rooms="roomsJson"
				:messages="messagesJson" :room-id="store.roomId" :messages-loaded="messagesLoaded" :show-add-room="false"
				:show-search="false" :show-files="false" :show-audio="false" :show-emojis="false"
				:show-reaction-emojis="false" :show-new-messages-divider="false" :show-rooms="false"
				:auto-scroll="autoScrollJson" :styles="stylesJson" rooms-loaded height="100%"
				@fetch-messages="onFetchMessages" @send-message="onSendMessage" />
			<div v-show="showClearedUI" class="cleared-overlay">
				<p>Tất cả phòng đã được xoá</p>
				<div class="cleared-actions">
					<el-button type="primary" @click="onCreateNewRoom">+ Tạo phòng mới</el-button>
					<el-button @click="onBackToMain">← Trở về màn hình chính</el-button>
				</div>
			</div>
		</div>

		<el-button v-if="!isOpen" class="chat-widget-bubble" type="primary" circle @click="openWidget">
			<el-icon style="font-size:22px">
				<ChatDotRound />
			</el-icon>
			<span v-if="unreadCount" class="chat-widget-badge">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
		</el-button>
	</teleport>
</template>

<script setup lang="ts">
import { register } from 'vue-advanced-chat'
register()
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElButton, ElIcon } from 'element-plus'
import { ArrowLeft, ChatDotRound } from '@element-plus/icons-vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useSessionStore } from '../stores/session.store'
import { useChatStore } from '../stores/chat.store'

declare const window: any
const api = () => window.go?.main?.App

const props = withDefaults(defineProps<{ fullPage?: boolean }>(), { fullPage: false })
const emit = defineEmits<{ back: [] }>()

const store = useChatStore()
const session = useSessionStore()

const currentUserId = computed(() => session.userID || '0')
const isOpen = ref(false)
const unreadCount = ref(0)
const posX = ref(20)
const posY = ref(80)
const isDragging = ref(false)
const dragOffset = { x: 0, y: 0 }

const rooms = ref<any[]>([])
const messagesLoaded = ref(false)
const showClearedUI = ref(false)

const roomsJson = computed(() => JSON.stringify(rooms.value))
const messagesJson = computed(() => JSON.stringify(store.messages))
const autoScrollJson = computed(() => JSON.stringify({
	send: { new: true, newAfterScrollUp: false },
	receive: { new: false, newAfterScrollUp: true }
}))
const stylesJson = computed(() => JSON.stringify({
	general: { color: '#333', borderStyle: '1px solid #e4e7ed' },
	footer: { background: '#fff' }
}))

function openWidget() {
	isOpen.value = true
	loadRoom()
}

function startDrag(e: MouseEvent) {
	isDragging.value = true
	dragOffset.x = e.clientX - posX.value
	dragOffset.y = e.clientY - posY.value
	document.addEventListener('mousemove', onDrag)
	document.addEventListener('mouseup', stopDrag)
}

function onDrag(e: MouseEvent) {
	if (!isDragging.value) return
	posX.value = e.clientX - dragOffset.x
	posY.value = Math.max(0, e.clientY - dragOffset.y)
}

function stopDrag() {
	isDragging.value = false
	document.removeEventListener('mousemove', onDrag)
	document.removeEventListener('mouseup', stopDrag)
}

async function loadRoom() {
	try {
		await store.loadSupportConv(session.userID)
		if (store.roomId) {
			rooms.value = [{
				roomId: store.roomId,
				roomName: 'Hỗ trợ',
				unreadCount: 0,
				users: [{ _id: currentUserId.value, username: 'Tôi' }]
			}]
		}
		messagesLoaded.value = true
	} catch { rooms.value = [] }
}

async function onFetchMessages($event: any) {
	const raw = $event?.detail ?? $event
	const payload = Array.isArray(raw) ? raw[0] : raw
	if (!payload?.room?.roomId) return
	store.roomId = payload.room.roomId
	messagesLoaded.value = false
	await store.loadMessages()
	messagesLoaded.value = true
}

async function onSendMessage($event: any) {
	const raw = $event?.detail ?? $event
	const payload = Array.isArray(raw) ? raw[0] : raw
	const roomId = payload?.roomId || store.roomId
	const content = payload?.content
	if (!content || !roomId) return
	try {
		const resp = await api().SendRoomMessage(roomId, content)
		if (resp) {
			store.appendMessage(JSON.parse(resp))
		}
	} catch (e) {
		console.error('[Chat] onSendMessage error:', e)
	}
}

let cleanup: (() => void) | null = null

async function onCreateNewRoom() {
	showClearedUI.value = false
	await loadRoom()
}

function onBackToMain() {
	showClearedUI.value = false
	if (props.fullPage) {
		emit('back')
	} else {
		isOpen.value = false
	}
}

onMounted(async () => {
	cleanup = EventsOn('vnet:rooms:cleared', () => {
		store.clearAll()
		rooms.value = []
		showClearedUI.value = true
	})

	if (props.fullPage) {
		await loadRoom()
		await nextTick()
		messagesLoaded.value = false
		await nextTick()
		messagesLoaded.value = true
		nextTick(() => {
			const el = document.querySelector('.chat-fullpage > vue-advanced-chat')
			if (el?.shadowRoot) {
				const style = document.createElement('style')
				style.textContent = `
					.vac-rooms-container { display: none !important; }
					.vac-toggle-button { display: none !important; }
					.vac-room-header { display: none !important; height: 0 !important; min-height: 0 !important; padding: 0 !important; }
					.vac-col-messages .vac-container-scroll { margin-top: 0 !important; }
				`
				el.shadowRoot.appendChild(style)
			}
		})
	}
})

onBeforeUnmount(() => {
	if (cleanup) cleanup()
})
</script>

<style scoped>
.chat-widget-overlay {
	position: fixed;
	inset: 0;
	z-index: 1000;
	background: transparent;
}

.chat-widget-bubble {
	position: fixed !important;
	bottom: 20px !important;
	right: 20px !important;
	z-index: 999;
	width: 52px;
	height: 52px;
	font-size: 22px;
	box-shadow: 0 4px 12px rgba(0, 0, 0, .2);
}

.chat-widget-badge {
	position: absolute;
	top: -4px;
	right: -4px;
	min-width: 20px;
	height: 20px;
	border-radius: 10px;
	background: #f56c6c;
	color: #fff;
	font-size: 11px;
	font-weight: 600;
	display: flex;
	align-items: center;
	justify-content: center;
	padding: 0 5px;
}

.chat-widget-panel {
	position: fixed;
	z-index: 1001;
	width: 380px;
	height: 520px;
	background: #fff;
	border-radius: 12px;
	box-shadow: 0 8px 24px rgba(0, 0, 0, .15);
	display: flex;
	flex-direction: column;
	overflow: hidden;
}

.chat-widget-handle {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 8px 12px;
	background: #f5f7fa;
	cursor: grab;
	user-select: none;
	font-size: 13px;
	font-weight: 500;
	border-bottom: 1px solid #e4e7ed;
	flex-shrink: 0;
}

.chat-widget-handle:active {
	cursor: grabbing;
}

.chat-widget-panel :deep(vue-advanced-chat) {
	flex: 1;
	min-height: 0;
}

.chat-fullpage {
	position: relative;
	height: 100vh;
	display: flex;
	flex-direction: column;
	background: #fff;
}

.chat-fullpage .chat-header {
	display: flex;
	align-items: center;
	padding: 12px 16px;
	box-shadow: 0 1px 4px rgba(0, 0, 0, .08);
	flex-shrink: 0;
}

.chat-fullpage .chat-header h3 {
	font-size: 16px;
	font-weight: 600;
	flex: 1;
	text-align: center;
}

.chat-fullpage :deep(vue-advanced-chat) {
	flex: 1;
	min-height: 0;
}

.cleared-overlay {
	position: absolute;
	inset: 0;
	z-index: 10;
	background: #fff;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 16px;
	padding: 24px;
	color: #666;
	font-size: 14px;
}

.cleared-actions {
	display: flex;
	flex-direction: column;
	gap: 10px;
	width: 100%;
	max-width: 240px;
}
</style>
