import zhCN from './langs/zh-cn';
import enUS from './langs/en-us';
import viVN from './langs/vi-vn';

const locales: Record<App.I18n.LangType, App.I18n.Schema> = {
  'zh-CN': zhCN,
  'en-US': enUS,
  'vi-VN': viVN
};

export default locales;
