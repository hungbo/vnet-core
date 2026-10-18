import type { App } from 'vue'
import { ReportUIError } from '../../wailsjs/go/main/App'

// Lỗi JS của giao diện máy trạm đi qua Go (App.ReportUIError, errorreport.go)
// về server và hiện trong trang "Nhật ký lỗi" của admin. Console của WebView2
// không được lưu ở đâu cả, nên trước đây lỗi giao diện trên máy khách là vô hình.

/** Một lỗi lặp trong vòng lặp không được dội hàng nghìn lần lên server. */
const MAX_PER_LOAD = 20
/** Cùng một lỗi: một lần mỗi phút là đủ, server tự đếm số lần. */
const DEDUPE_MS = 60_000

let sentCount = 0
const lastSent = new Map<string, number>()

export function reportUIError(err: unknown, context = '') {
	const message = err instanceof Error ? err.message : String(err ?? '')
	if (!message || message.includes('ResizeObserver loop')) return
	const where = window.location.hash || window.location.pathname
	const key = `${message}\n${where}`
	const now = Date.now()
	if (sentCount >= MAX_PER_LOAD || now - (lastSent.get(key) ?? 0) < DEDUPE_MS) return
	sentCount += 1
	lastSent.set(key, now)

	const stack = err instanceof Error && err.stack ? err.stack : ''
	try {
		ReportUIError(message.slice(0, 2000), [context, stack].filter(Boolean).join('\n').slice(0, 16000), where).catch(() => {})
	} catch {
		// Ngoài Wails (npm run dev) không có window.go nên lệnh gọi ném lỗi — và
		// ném trong bộ bắt lỗi là sinh thêm lỗi để bắt.
	}
}

export function setupErrorReporting(app: App) {
	app.config.errorHandler = (err, _vm, info) => {
		console.error(err)
		reportUIError(err, `vue: ${info}`)
	}
	window.addEventListener('error', e => {
		reportUIError(e.error ?? e.message, `${e.filename}:${e.lineno}:${e.colno}`)
	})
	window.addEventListener('unhandledrejection', e => {
		reportUIError(e.reason, 'unhandledrejection')
	})
}
