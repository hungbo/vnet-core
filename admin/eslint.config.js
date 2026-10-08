import { defineConfig } from '@soybeanjs/eslint-config';

export default defineConfig(
  { vue: true, unocss: true },
  {
    rules: {
      'vue/multi-word-component-names': [
        'warn',
        {
          ignores: ['index', 'App', 'Register', '[id]', '[url]']
        }
      ],
      'vue/component-name-in-template-casing': [
        'warn',
        'PascalCase',
        {
          registeredComponentsOnly: false,
          // Web component (vue-advanced-chat) phải giữ nguyên tên gạch nối: vite chỉ coi
          // đúng tên đó là custom element. Để --fix đổi sang PascalCase thì Vue không
          // nhận ra nó và khung chat của quản trị trắng trơn, không một lỗi nào.
          ignores: ['/^icon-/', 'vue-advanced-chat']
        }
      ],
      'unocss/order-attributify': 'off'
    }
  }
);
