import { getCurrentInstance, onBeforeMount, unref } from 'vue';
import type { SetupContext } from 'vue';
import { ElOption } from 'element-plus/es';

/** global table column align */
ElTable.TableColumn.props.align = {
  type: String,
  default: 'center'
};

/** Đo chữ tiêu đề cột: đậm 600, 14px, cùng font với trang (một canvas dùng chung). */
let thuocDo: CanvasRenderingContext2D | null = null;
function rongChu(text: string) {
  thuocDo ??= document.createElement('canvas').getContext('2d');
  if (!thuocDo) return 0;
  thuocDo.font = `600 14px ${getComputedStyle(document.body).fontFamily}`;
  return Math.ceil(thuocDo.measureText(text).width);
}

/**
 * Tiêu đề cột không xuống dòng: cột hẹp hơn chữ tiêu đề (cộng mũi tên sắp xếp, nút lọc) thì nới ra vừa một dòng —
 * cột cố định nới `width`, cột co giãn nới `min-width`. Element Plus để cấu hình cột ở `instance.columnConfig`, dựng
 * trong onBeforeMount của nó; hàm dưới đây đăng ký sau nên chạy sau, trước khi cột vào bảng ở onMounted.
 */
const TableColumn = ElTable.TableColumn as unknown as { setup: (props: any, ctx: SetupContext<any>) => any };
const setupColumn = TableColumn.setup;
TableColumn.setup = (props, ctx) => {
  const render = setupColumn(props, ctx);
  const instance: any = getCurrentInstance();
  onBeforeMount(() => {
    const col = instance?.columnConfig?.value;
    if (!col || col.type !== 'default' || typeof col.label !== 'string' || !col.label) return;
    // 1px viền ô + 16px chừa hai bên; ô tiêu đề không có lề ngang.
    const can = rongChu(col.label) + (col.sortable ? 24 : 0) + (col.filterable ? 16 : 0) + 17;
    if (col.width !== undefined) {
      if (Number(col.width) < can) {
        col.width = can;
        col.realWidth = can;
      }
    } else if (Number(col.minWidth) < can) {
      col.minWidth = can;
      col.realWidth = can;
    }
  });
  return render;
};

/** global ElCard shadow */
ElCard.props.shadow = {
  type: String,
  default: 'never'
};

/** global ElForm require asterisk position */
ElForm.props.requireAsteriskPosition = {
  type: String,
  default: 'right'
};

/** Chữ thường, bỏ dấu tiếng Việt: "Vàng Đỏ" → "vang do". */
function khongDau(s: string) {
  return s.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'D').toLowerCase();
}

/**
 * Ô select có tìm kiếm (filterable) lọc không dấu: gõ "vang" ra "Vàng". Element Plus gọi updateOption(query) của
 * từng ElOption để ẩn/hiện nó, mặc định so bằng RegExp phân biệt dấu; thay hàm đó ở một chỗ cho mọi select. Select
 * có filter-method hoặc remote-method riêng thì Element Plus không gọi hàm này.
 */
const setupOption = ElOption.setup!;
ElOption.setup = (props: any, ctx: SetupContext<any>) => {
  const bindings: any = setupOption(props, ctx);
  if (typeof bindings?.updateOption === 'function' && bindings.states) {
    bindings.updateOption = (query: string) => {
      bindings.states.visible =
        khongDau(String(unref(bindings.currentLabel))).includes(khongDau(query)) || props.created;
    };
  }
  return bindings;
};
