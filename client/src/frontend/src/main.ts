import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
// Bảng biến màu tối của Element Plus phải nạp SAU index.css, và tokens.css nạp
// sau cùng để ghi đè cả hai.
import 'element-plus/theme-chalk/dark/css-vars.css'
import './styles/tokens.css'
import App from './App.vue'

// Ngoài Wails (npm run dev trong trình duyệt) không có runtime nào, và mọi
// component gọi EventsOn lúc mounted sẽ ném lỗi làm ngừng vẽ cả cây. Lớp giả
// dưới đây KHÔNG vào bản dựng production: import.meta.env.DEV bị loại lúc build.
if (import.meta.env.DEV) {
	const { installDevWailsStub } = await import('./dev-wails-stub')
	installDevWailsStub()
}

const app = createApp(App)

app.use(createPinia())
app.use(ElementPlus)
app.mount('#app')
