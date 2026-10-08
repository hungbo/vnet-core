// @unocss-include
import { getColorPalette, getRgb } from '@sa/color';
import { DARK_CLASS } from '@/constants/app';
import { localStg } from '@/utils/storage';
import { toggleHtmlClass } from '@/utils/common';
import { $t } from '@/locales';

export function setupLoading() {
  const themeColor = localStg.get('themeColor') || '#646cff';
  const darkMode = localStg.get('darkMode') || false;
  const palette = getColorPalette(themeColor);

  const { r, g, b } = getRgb(themeColor);

  const primaryColor = `--primary-color: ${r} ${g} ${b}`;

  const svgCssVars = Array.from(palette.entries())
    .map(([key, value]) => `--logo-color-${key}: ${value}`)
    .join(';');

  const cssVars = `${primaryColor}; ${svgCssVars}`;

  if (darkMode) {
    toggleHtmlClass(DARK_CLASS).add();
  }

  const loadingClasses = [
    'left-0 top-0',
    'left-0 bottom-0 animate-delay-500',
    'right-0 top-0 animate-delay-1000',
    'right-0 bottom-0 animate-delay-1500'
  ];

  const dot = loadingClasses
    .map(item => {
      return `<div class="absolute w-16px h-16px bg-primary rounded-8px animate-pulse ${item}"></div>`;
    })
    .join('\n');

  const loading = `
<div class="fixed-center flex-col bg-layout" style="${cssVars}">
  <div class="w-128px h-128px">
    ${getLogoSvg()}
  </div>
  <div class="w-56px h-56px my-36px">
    <div class="relative h-full animate-spin">
      ${dot}
    </div>
  </div>
  <h2 class="text-28px font-500 text-primary">${$t('system.title')}</h2>
</div>`;

  const app = document.getElementById('app');

  if (app) {
    app.innerHTML = loading;
  }
}

function getLogoSvg() {
  // Cùng hình với branding/vnet-mark.svg. Viết thẳng ở đây vì màn hình chờ dựng
  // trước khi Vue và bộ nạp tài nguyên chạy.
  return `<svg width="100%" height="100%" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="VNET">
<defs>
    <linearGradient id="vnet-loading-grad" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#4C7DFF"/>
      <stop offset="1" stop-color="#7B5CFF"/>
    </linearGradient>
  </defs>
  <rect x="8" y="8" width="240" height="240" rx="56" fill="#0D1017"/>
  <rect x="8" y="8" width="240" height="240" rx="56" fill="none" stroke="url(#vnet-loading-grad)" stroke-width="10"/>
  <path transform="translate(24 0) skewX(-12)" fill="url(#vnet-loading-grad)"
        d="M48 56 H96 L128 158 L160 56 H208 L152 204 H104 Z"/>
</svg>`;
}
