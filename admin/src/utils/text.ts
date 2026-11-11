/** Chữ thường, bỏ dấu tiếng Việt: "Liên Minh" → "lien minh". Dùng cho ô tìm kiếm không dấu. */
export function khongDau(s: string) {
  return s
    .normalize('NFD')
    .replace(/[\u0300-\u036F]/g, '')
    .replace(/đ/g, 'd')
    .replace(/Đ/g, 'D')
    .toLowerCase();
}
