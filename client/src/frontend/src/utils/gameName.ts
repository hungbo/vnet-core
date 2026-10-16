// Tên game hiển thị cho khách.
//
// Có game trong cơ sở dữ liệu bị lưu display_name hỏng: dấu tiếng Việt rụng lúc
// tạo game qua đường mã hoá sai, chỉ còn ký tự thay thế U+FFFD ("Li�n Minh ...").
// Hiện nguyên chuỗi ấy thì khách thấy ô vuông giữa chữ, và lỗi này nằm ở DỮ LIỆU
// nên đổi font không cứu được. Slug `name` luôn là ASCII sạch, nên dùng nó thay
// cho tới khi quản trị viên sửa lại tên.
export function gameLabel(g: { name: string; display_name?: string | null }): string {
	const shown = (g.display_name ?? '').trim()
	return shown && !shown.includes('�') ? shown : g.name
}

// foldSearch chuẩn hoá chữ để tìm: chữ thường, bỏ dấu tiếng Việt (đ → d), gộp
// khoảng trắng. Khách gõ "lien minh" hay "LIÊN MINH" đều ra cùng một chuỗi.
export function foldSearch(s: string): string {
	return s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().replace(/đ/g, 'd').replace(/\s+/g, ' ').trim()
}

// matchesSearch: mọi từ khách gõ đều phải có trong tên hiển thị, nhóm, tên thư
// mục hoặc chữ cái đầu các từ của tên ("lmht" ra Liên Minh Huyền Thoại). Ô tìm
// trống thì khớp tất cả.
export function matchesSearch(g: { name: string; display_name?: string | null; category?: string | null }, query: string): boolean {
	const words = foldSearch(query).split(' ').filter(Boolean)
	if (!words.length) return true
	const label = foldSearch(gameLabel(g))
	const initials = label.split(/[^a-z0-9]+/).filter(Boolean).map(w => w[0]).join('')
	const hay = [label, foldSearch(g.category ?? ''), foldSearch(g.name), initials].join(' ')
	return words.every(w => hay.includes(w))
}
