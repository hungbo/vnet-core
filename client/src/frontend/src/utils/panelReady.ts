import { inject, type InjectionKey } from 'vue'

// Màn chờ của cửa sổ phụ (App.vue): từ lúc đổi màn hình tới khi màn hình mới tải xong
// dữ liệu lần đầu, cửa sổ phủ một lớp "Đang mở …" thay vì để khách nhìn màn cũ hay một
// màn trống đang tải dở. Màn hình gọi hàm này khi đã sẵn sàng; màn nào không gọi thì
// lớp chờ tự gỡ sau một hạn chót.
export const PANEL_SAN_SANG: InjectionKey<() => void> = Symbol('panel-san-sang')

// Ngoài cửa sổ phụ (không ai cung cấp) thì là hàm rỗng.
export function useBaoSanSang(): () => void {
	return inject(PANEL_SAN_SANG, () => {})
}
