import { unref } from 'vue';
import type { SetupContext } from 'vue';
import { ElOption } from 'element-plus/es';

/** global table column align */
ElTable.TableColumn.props.align = {
  type: String,
  default: 'center'
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
