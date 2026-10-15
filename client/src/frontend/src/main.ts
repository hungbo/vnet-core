import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
// Bảng biến màu tối của Element Plus phải nạp SAU index.css, và tokens.css nạp
// sau cùng để ghi đè cả hai.
import 'element-plus/theme-chalk/dark/css-vars.css'
// Phông nhúng sẵn trong bản build: quán không chắc có Internet, và phông mặc
// định của Windows không có dáng của logo. Chỉ nạp những độ đậm thật sự dùng.
import '@fontsource/be-vietnam-pro/400.css'
import '@fontsource/be-vietnam-pro/500.css'
import '@fontsource/be-vietnam-pro/600.css'
import '@fontsource/chakra-petch/600.css'
import '@fontsource/chakra-petch/700.css'
import '@fontsource/chakra-petch/700-italic.css'
import './styles/tokens.css'
import App from './App.vue'
import { EventsEmit } from '../wailsjs/runtime/runtime'
import { setupErrorReporting } from './utils/errorReport'

// Ngoài Wails (npm run dev trong trình duyệt) không có runtime nào, và mọi
// component gọi EventsOn lúc mounted sẽ ném lỗi làm ngừng vẽ cả cây. Lớp giả
// dưới đây KHÔNG vào bản dựng production: import.meta.env.DEV bị loại lúc build.
if (import.meta.env.DEV) {
	const { installDevWailsStub } = await import('./dev-wails-stub')
	installDevWailsStub()
}

const app = createApp(App)
setupErrorReporting(app)

app.use(createPinia())
app.use(ElementPlus)
app.mount('#app')
performance.mark('vnet:mount')
const hienLucMount = document.visibilityState

// Mốc nạp trang gửi về Go ghi vào ui.log (curtain.go, ghiMocNapTrang): khúc 9–15 giây
// từ lúc WebView2 dựng xong tới lúc hạ màn che phải tách được ra từng phần. Số tính
// bằng ms từ lúc trang bắt đầu nạp; Go tự quy về mốc tiến trình. Màn hình khoá dựng
// ngay trong mount (không chờ gì), nên "mount" ≈ màn hình khoá có DOM. "hiển thị" là
// document.visibilityState lúc mount / lúc load: cửa sổ nằm dưới màn che, Chromium coi
// trang là hidden thì "vẽ đầu" trống hoặc trễ — đó là dấu hiệu, không phải lỗi vẽ.
window.addEventListener('load', () => setTimeout(() => {
	const ms = (v?: number) => (v ? Math.round(v) + 'ms' : '-')
	const nav = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined
	const tep = performance.getEntriesByType('resource') as PerformanceResourceTiming[]
	const xong = (duoi: RegExp) => Math.max(0, ...tep.filter(r => duoi.test(r.name)).map(r => r.responseEnd))
	EventsEmit('vnet:ui:moc', performance.now(), [
		`html ${ms(nav?.responseEnd)}`,
		`js ${ms(xong(/\.js(\?|$)/))}`,
		`css ${ms(xong(/\.css(\?|$)/))}`,
		`phông ${ms(xong(/\.woff2?(\?|$)/))}`,
		`domInteractive ${ms(nav?.domInteractive)}`,
		`DOMContentLoaded ${ms(nav?.domContentLoadedEventStart)}`,
		`mount ${ms(performance.getEntriesByName('vnet:mount')[0]?.startTime)}`,
		`vẽ đầu ${ms(performance.getEntriesByName('first-contentful-paint')[0]?.startTime)}`,
		`load ${ms(nav?.loadEventStart)}`,
		`hiển thị ${hienLucMount}/${document.visibilityState}`,
	].join(', '))
}), { once: true })
