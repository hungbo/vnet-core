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
