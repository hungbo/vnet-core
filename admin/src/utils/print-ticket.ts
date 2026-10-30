// Khổ 80mm, vùng in 72mm. Không đặt chiều cao trang: để trình điều khiển máy in
// tự cắt theo nội dung như khi in một trang web bình thường.
const TICKET_CSS = `
@page { margin: 0; }
* { box-sizing: border-box; }
body { margin: 0; width: 72mm; padding: 2mm 3mm 4mm; font-family: Arial, Helvetica, sans-serif;
  font-size: 11pt; line-height: 1.35; color: #000; }
.center { text-align: center; }
.shop { font-size: 14pt; font-weight: 700; }
.title { font-size: 12pt; font-weight: 700; margin-top: 2mm; }
.sep { border-top: 1px dashed #000; margin: 2mm 0; }
.label { font-size: 10pt; margin-top: 1mm; }
.big { font-size: 20pt; font-weight: 700; letter-spacing: 1px; word-break: break-all; }
.row { display: flex; justify-content: space-between; gap: 2mm; }
.row span:last-child { text-align: right; font-weight: 700; }
.foot { margin-top: 3mm; font-size: 10pt; }
`;

/**
 * In một phiếu khổ giấy in nhiệt qua hộp thoại in của trình duyệt.
 *
 * Nút in của máy chủ gửi lệnh ESC/POS tới máy in mạng (IP:cổng). Máy in ở quầy
 * thường cắm USB vào máy thu ngân (vd Xprinter XP-80) nên máy chủ không với tới
 * được; in qua trình duyệt thì máy in nào đã cài trên Windows cũng dùng được.
 * Phiếu nằm trong một iframe ẩn để không in cả trang quản trị.
 */
export function printTicket(title: string, bodyHtml: string) {
  const iframe = document.createElement('iframe');
  iframe.setAttribute('aria-hidden', 'true');
  iframe.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0;visibility:hidden';
  document.body.appendChild(iframe);

  const win = iframe.contentWindow;
  const doc = iframe.contentDocument;
  if (!win || !doc) {
    iframe.remove();
    return;
  }
  doc.open();
  doc.write(
    `<!doctype html><html><head><meta charset="utf-8"><title>${escapeHtml(title)}</title>` +
      `<style>${TICKET_CSS}</style></head><body>${bodyHtml}</body></html>`
  );
  doc.close();

  win.addEventListener('afterprint', () => setTimeout(() => iframe.remove(), 500));
  // Đợi trình duyệt dựng xong trang trong iframe, in sớm quá thì ra tờ trắng.
  setTimeout(() => {
    win.focus();
    win.print();
  }, 150);
}

export function escapeHtml(s: unknown): string {
  return String(s ?? '').replace(
    /[&<>"']/g,
    c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string
  );
}
