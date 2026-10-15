import { createApp } from 'vue';
import './plugins/assets';
import './plugins/ui';
import {
  setupAppErrorHandle,
  setupAppVersionNotification,
  setupDayjs,
  setupIconifyOffline,
  setupLoading,
  setupNProgress,
  setupWebSocket
} from './plugins';
import { setupStore } from './store';
import { setupRouter } from './router';
import { setupI18n } from './locales';
import App from './App.vue';

async function setupApp() {
  setupLoading();

  setupNProgress();

  setupIconifyOffline();

  setupDayjs();

  const app = createApp(App);

  setupAppErrorHandle(app);

  setupStore(app);

  await setupRouter(app);

  setupWebSocket();

  setupI18n(app);

  setupAppVersionNotification();

  app.mount('#app');
}

setupApp();
