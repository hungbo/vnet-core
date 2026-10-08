<template>
  <!--
    Logo VNET: chữ V trong khung bo góc, cùng bộ với icon máy trạm và favicon.
    Tệp gốc nằm ở branding/vnet-mark.svg; đổi logo thì đổi ở đó rồi chép sang.
  -->
  <div class="app-logo">
    <svg
      width="100%"
      height="100%"
      viewBox="0 0 256 256"
      xmlns="http://www.w3.org/2000/svg"
      role="img"
      aria-label="VNET"
    >
      <defs>
        <linearGradient id="vnet-logo-grad" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stop-color="#4C7DFF" />
          <stop offset="1" stop-color="#7B5CFF" />
        </linearGradient>
      </defs>
      <rect x="8" y="8" width="240" height="240" rx="56" fill="#0D1017" />
      <rect x="8" y="8" width="240" height="240" rx="56" fill="none" stroke="url(#vnet-logo-grad)" stroke-width="10" />
      <path
        transform="translate(24 0) skewX(-12)"
        fill="url(#vnet-logo-grad)"
        d="M48 56 H96 L128 158 L160 56 H208 L152 204 H104 Z"
      />
    </svg>
  </div>
</template>
