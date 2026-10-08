// Báo thời gian còn lại: đọc file âm thanh dựng sẵn ở 30, 15, 10 phút, và mỗi
// phút một lần từ 5 phút trở xuống.
//
// File dựng sẵn chứ không dùng giọng đọc của Windows: máy trạm hầu như không cài
// giọng tiếng Việt, và giọng tiếng Anh đọc câu tiếng Việt thì không ai nghe ra.
// Đổi giọng: thay các tệp assets/sounds/con-<số>-phut.mp3, giữ nguyên tên.

export const MOC_PHUT = [30, 15, 10, 5, 4, 3, 2, 1]

const amThanh = import.meta.glob('../assets/sounds/con-*-phut.mp3', {
	eager: true,
	import: 'default',
}) as Record<string, string>

// mocVuaQua trả về mốc (phút) vừa đi qua giữa hai lần đo, hoặc 0.
//
// Tính theo GIÂY còn lại chứ không theo phút làm tròn: đồng hồ có thể nhảy
// (máy chủ đẩy mốc hết tiền mới, máy ngủ rồi thức) và vượt qua vài mốc một lúc
// — khi đó chỉ báo mốc nhỏ nhất, đọc dồn ba câu liền nhau thì không ai nghe kịp.
//
// truoc = null là lần đo đầu của phiên: chỉ báo nếu đã ở dưới 5 phút, để khách
// đăng nhập lúc còn ít tiền vẫn được nhắc ngay.
export function mocVuaQua(truoc: number | null, bayGio: number): number {
	if (bayGio <= 0) return 0
	if (truoc === null) {
		const phut = Math.ceil(bayGio / 60)
		return phut <= 5 ? phut : 0
	}
	let moc = 0
	for (const m of MOC_PHUT) {
		if (truoc > m * 60 && bayGio <= m * 60) moc = m
	}
	return moc
}

export function phatThongBao(phut: number) {
	const url = amThanh[`../assets/sounds/con-${phut}-phut.mp3`]
	if (!url) return
	new Audio(url).play().catch(e => console.warn('[báo giờ] không phát được âm thanh', e))
}
