// Tồn kho của một món trong thực đơn, dùng chung cho thẻ món và bộ lọc "Còn hàng".
//
// CHỈ xét tồn kho khi has_stock bật.
//
// Món nấu từ nguyên liệu mà không bật cờ này luôn được máy chủ trả về
// current_stock = 0 (xem loadProductStock bên backend). Lấy số 0 đó ra mà khoá
// là khoá nhầm gần hết thực đơn trong khi kho vẫn đầy.
export function theoDoiKho(p: any): boolean {
	return p?.has_stock === true
}

export function soConLai(p: any): number {
	return Math.floor(Number(p?.current_stock ?? 0))
}

export function hetHang(p: any): boolean {
	return theoDoiKho(p) && soConLai(p) <= 0
}
