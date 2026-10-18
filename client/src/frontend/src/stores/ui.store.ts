import { defineStore } from 'pinia'
import { ref } from 'vue'

declare const window: any
const api = () => window.go?.main?.App

/**
 * Công tắc bật/tắt tính năng, do quán đặt ở trang Cài đặt của trang quản trị.
 *
 * Mặc định BẬT hết. Nhóm cài đặt "features" chưa tồn tại cho tới lần Lưu đầu
 * tiên và máy chủ trả 404 khi nhóm rỗng — mặc định tắt sẽ làm điểm danh và đánh
 * giá biến mất ở mọi quán chưa từng mở tab đó.
 *
 * Ẩn nút chỉ là phép lịch sự với người dùng; máy chủ vẫn từ chối nếu ai đó gọi
 * thẳng API, nên hai bên không thể lệch nhau về mặt hiệu lực.
 */
export const useUiStore = defineStore('ui', () => {
	const features = ref<Record<string, boolean>>({})
	const loaded = ref(false)

	function enabled(key: string): boolean {
		return features.value[key] !== false
	}

	async function load() {
		try {
			const raw = await api()?.GetFeatureFlags()
			if (!raw) return
			const parsed = JSON.parse(raw)
			const next: Record<string, boolean> = {}
			Object.keys(parsed || {}).forEach(k => {
				// Máy chủ lưu công tắc trong cột jsonb và trả về dạng CHUỖI, nên
				// "false" là chuỗi khác rỗng: so bằng truthiness sẽ luôn ra "bật".
				next[k] = parsed[k] !== false && parsed[k] !== 'false'
			})
			features.value = next
		} catch {
			// Không đọc được thì giữ nguyên "bật hết" — mất mạng không phải lý do
			// để khách mất chức năng.
		} finally {
			loaded.value = true
		}
	}

	return { features, loaded, enabled, load }
})
