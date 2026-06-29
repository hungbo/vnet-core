import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { EventsOn } from '../../wailsjs/runtime/runtime'

declare const window: any
const api = () => window.go?.main?.App

function mapMessage(raw: any, userId: string) {
  return {
    _id: raw.id,
    content: raw.message,
    senderId: raw.sender_type === 'member' ? userId : '0',
    username: raw.sender_type === 'member' ? 'Tôi' : 'Admin',
    createdAt: raw.created_at,
    date: raw.created_at ? new Date(raw.created_at).toLocaleDateString('vi-VN') : '',
    timestamp: raw.created_at ? new Date(raw.created_at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '',
    saved: true,
    distributed: raw.status === 'delivered' || raw.status === 'read',
    seen: raw.status === 'read',
  }
}

function sortMessages(msgs: any[]) {
  return msgs.sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime())
}

export const useChatStore = defineStore('chat', () => {
  const convId = ref('')
  const messages = ref<any[]>([])
  const screenshotting = ref(false)
  const messageText = ref('')
  const userId = ref('')

  function initWsHandlers(uid: string) {
    userId.value = uid
    EventsOn('vnet:chat:message', (data: string) => {
      try {
        const msg = JSON.parse(data)
        if (msg.sender_type !== 'member') markDelivered(msg)
        appendMessage(msg)
      } catch {}
    })
    EventsOn('vnet:message:status:updated', (data: string) => {
      try {
        const parsed = JSON.parse(data)
        updateMessageStatus(parsed.id, parsed.status)
      } catch {}
    })
  }

  async function loadSupportConv(uid?: string) {
    if (uid) userId.value = uid
    try {
      const data = await api().GetConversations()
      const list = data ? JSON.parse(data) : []
      if (list.length > 0) {
        convId.value = list[0].id
      } else {
        const uid = userId.value || ''
        const created = await api().CreateConversation('Hỗ trợ', uid, 'member')
        const parsed = JSON.parse(created)
        convId.value = parsed.id || parsed
      }
      await loadMessages()
    } catch (e) {
      console.error('loadSupportConv error:', e)
    }
  }

  async function loadMessages() {
    if (!convId.value) return
    try {
      const data = await api().GetChatMessages(convId.value)
      const parsed = data ? JSON.parse(data) : []
      const items = Array.isArray(parsed) ? parsed : []
      messages.value = sortMessages(items.map(m => mapMessage(m, userId.value)))
    } catch { messages.value = [] }
  }

  function appendMessage(raw: any) {
    if (raw.conversation_id !== convId.value) return
    if (!messages.value.some(m => m._id === raw.id)) {
      messages.value = sortMessages([...messages.value, mapMessage(raw, userId.value)])
    }
  }

  async function sendMessage() {
    if (!messageText.value.trim() || !convId.value) return
    try {
      const raw = await api().SendChatMessage(convId.value, messageText.value)
      if (raw) {
        const msg = JSON.parse(raw)
        appendMessage(msg)
      }
      messageText.value = ''
    } catch (e) {
      ElMessage.error(String(e))
    }
  }

  async function sendScreenshot() {
    if (!convId.value) return
    screenshotting.value = true
    try {
      const imgData = await api().TakeScreenshot()
      if (!imgData) {
        ElMessage.warning('Chụp màn hình không khả dụng trên nền tảng này')
        return
      }
      await api().SendScreenshotMessage(convId.value, imgData)
    } catch (e) {
      ElMessage.error(String(e))
    } finally {
      screenshotting.value = false
    }
  }

  function updateMessageStatus(id: string, status: string) {
    const msg = messages.value.find(m => m._id === id)
    if (msg) msg.distributed = status === 'delivered' || status === 'read'
    if (msg) msg.seen = status === 'read'
  }

  async function markDelivered(msg: any) {
    if (msg.status === 'sent') {
      try { await api().MarkMessageDelivered(msg.id) } catch {}
    }
  }

  async function markRead(msg: any) {
    if (msg.status !== 'read') {
      try { await api().MarkMessageRead(msg.id) } catch {}
    }
  }

  async function markConversationRead() {
    if (!convId.value) return
    try { await api().MarkConversationMessagesRead(convId.value) } catch {}
  }

  function clearAll() {
    convId.value = ''
    messages.value = []
  }

  return {
    convId, messages, screenshotting, messageText,
    initWsHandlers,
    loadSupportConv, loadMessages, appendMessage, sendMessage, sendScreenshot,
    updateMessageStatus, markDelivered, markRead, markConversationRead,
    clearAll,
  }
})
