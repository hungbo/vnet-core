// Âm báo của máy trạm: có tin nhắn từ quầy, có thông báo mới.
//
// Chỉ thanh chính phát (App.vue, windowMode rỗng). Cửa sổ phụ cũng nhận cùng
// sự kiện; để nó phát nữa là mỗi tin kêu hai lần.
//
// Âm tự tổng hợp bằng ffmpeg (sóng sin), không vướng bản quyền. Đổi âm: thay
// tệp trong assets/sounds/, giữ nguyên tên.
import tinNhan from '../assets/sounds/tin-nhan.mp3'
import thongBao from '../assets/sounds/thong-bao.mp3'

const am = { 'tin-nhan': tinNhan, 'thong-bao': thongBao } as const
type TenAm = keyof typeof am

// Quầy gửi dồn mấy tin một lúc thì chỉ kêu một lần, không thành một tràng chuông.
const KHOANG_CACH_MS = 1500
const lanCuoi: Partial<Record<TenAm, number>> = {}

export function phatAm(ten: TenAm) {
	const bayGio = Date.now()
	if (bayGio - (lanCuoi[ten] ?? 0) < KHOANG_CACH_MS) return
	lanCuoi[ten] = bayGio
	const a = new Audio(am[ten])
	a.volume = 0.8
	a.play().catch(e => console.warn('[âm báo] không phát được', e))
}
