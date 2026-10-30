import {
  computed,
  effectScope,
  getCurrentInstance,
  nextTick,
  onActivated,
  onMounted,
  onScopeDispose,
  reactive,
  shallowRef,
  watch
} from 'vue';
import type { Ref } from 'vue';
import { useRoute } from 'vue-router';
import { useDebounceFn, useEventListener } from '@vueuse/core';
import type { PaginationEmits, PaginationProps } from 'element-plus';
import { useBoolean, useTable } from '@sa/hooks';
import type { PaginationData, TableColumnCheck, UseTableOptions } from '@sa/hooks';
import type { FlatResponseData } from '@sa/axios';
import { jsonClone } from '@sa/utils';
import { useAppStore } from '@/store/modules/app';
import { $t } from '@/locales';

type RemoveReadonly<T> = {
  -readonly [key in keyof T]: T[key];
};

export type UseUITableOptions<ResponseData, ApiData, Pagination extends boolean> = Omit<
  UseTableOptions<ResponseData, ApiData, UI.TableColumn<ApiData>, Pagination>,
  'pagination' | 'getColumnChecks' | 'getColumns'
> & {
  /**
   * get column visible
   *
   * @param column
   *
   * @default true
   *
   * @returns true if the column is visible, false otherwise
   */
  getColumnVisible?: (column: UI.TableColumn<ApiData>) => boolean;
};

const SELECTION_KEY = '__selection__';

const EXPAND_KEY = '__expand__';

const INDEX_KEY = '__index__';

export function useUITable<ResponseData, ApiData>(options: UseUITableOptions<ResponseData, ApiData, false>) {
  const scope = effectScope();
  const appStore = useAppStore();

  const result = useTable<ResponseData, ApiData, UI.TableColumn<ApiData>, false>({
    ...options,
    getColumnChecks: cols => getColumnChecks(cols, options.getColumnVisible),
    getColumns
  });

  // calculate the total width of the table this is used for horizontal scrolling
  const scrollX = computed(() => {
    return result.columns.value.reduce((acc, column) => {
      return acc + Number(column.width ?? column.minWidth ?? 120);
    }, 0);
  });

  scope.run(() => {
    watch(
      () => appStore.locale,
      () => {
        result.reloadColumns();
      }
    );
  });

  onScopeDispose(() => {
    scope.stop();
  });

  return {
    ...result,
    scrollX
  };
}

/** Chiều cao một dòng khi trang chưa từng đo (ô 23px + đệm 8px trên dưới + viền). */
const DEFAULT_ROW_HEIGHT = 40;

const MIN_FIT_ROWS = 5;

const ROW_HEIGHT_STORAGE_PREFIX = 'vnet-table-row-height:';

/**
 * Chờ bảng xếp xong cột rồi mới đo dòng. Ngay sau khi gán dữ liệu, Element Plus chưa chia lại độ rộng cột nên
 * các nút ở cột Thao tác tạm xuống dòng: trang Hội viên đo ra 71px thay vì 42px và chỉ lấy 7 dòng.
 */
const LAYOUT_SETTLE_MS = 300;

/**
 * Số dòng mỗi trang vừa khít chiều cao bảng `.fill-table` (styles/css/page-fill.css), để bảng không
 * còn khoảng trống dưới dòng cuối. Chiều cao một dòng đo ở lần tải đầu rồi nhớ theo trang
 * (localStorage), nên lần sau mở trang là tải đúng số dòng ngay. Người dùng tự chọn số dòng/trang
 * thì thôi tự khớp; điện thoại (bảng cao theo nội dung) không áp dụng.
 */
export function useFitPageSize(options: {
  getPageSize: () => number;
  setPageSize: (size: number) => void;
  /** true khi người dùng đã tự chọn số dòng/trang. */
  isManual?: () => boolean;
}) {
  const instance = getCurrentInstance();
  const route = useRoute();
  const appStore = useAppStore();
  const storageKey = ROW_HEIGHT_STORAGE_PREFIX + String(route.name);
  let measured = false;

  function bodyWrapper() {
    const root = instance?.proxy?.$el;
    return root instanceof HTMLElement ? root.querySelector<HTMLElement>('.fill-table .el-table__body-wrapper') : null;
  }

  function storedRowHeight() {
    try {
      return Number(localStorage.getItem(storageKey)) || DEFAULT_ROW_HEIGHT;
    } catch {
      return DEFAULT_ROW_HEIGHT;
    }
  }

  /** Chiều cao trung bình của các dòng đang hiện; 0 khi bảng trống. */
  function rowHeightNow() {
    const rows = Array.from(bodyWrapper()?.querySelectorAll<HTMLElement>('.el-table__row') ?? []);
    return rows.length ? rows.reduce((sum, row) => sum + row.offsetHeight, 0) / rows.length : 0;
  }

  /** Đổi số dòng/trang cho vừa bảng; trả về true nếu đã đổi (nơi gọi tự tải lại qua setPageSize). */
  function fit(rowHeight = rowHeightNow() || storedRowHeight()) {
    const body = bodyWrapper();
    if (options.isManual?.() || appStore.isMobile || !body?.clientHeight) return false;

    const size = Math.max(MIN_FIT_ROWS, Math.floor(body.clientHeight / rowHeight));
    if (size === options.getPageSize()) return false;

    options.setPageSize(size);
    return true;
  }

  /** Gọi sau mỗi lần tải: lần đầu có dòng thì đo chiều cao dòng thật, nhớ lại và khớp lại nếu lệch. */
  async function afterLoad() {
    if (measured) return;
    await nextTick();
    await new Promise(resolve => {
      setTimeout(resolve, LAYOUT_SETTLE_MS);
    });
    const rowHeight = rowHeightNow();
    if (!rowHeight) return;

    measured = true;
    try {
      localStorage.setItem(storageKey, rowHeight.toFixed(1));
    } catch {}
    fit(rowHeight);
  }

  useEventListener(
    window,
    'resize',
    useDebounceFn(() => fit(), 300)
  );
  // KeepAlive: quay lại trang sau khi cửa sổ đã đổi cỡ.
  onActivated(() => fit());

  return { fit, afterLoad };
}

type PaginationParams = Pick<PaginationProps, 'currentPage' | 'pageSize'>;

const SORT_ORDER = { ascending: 'asc', descending: 'desc' } as const;

/** Tham số useUIPaginatedTable truyền cho api mỗi lần tải: trang, cỡ trang và cột đang sắp xếp. */
type PaginatedApiParams = { page: number; pageSize: number; sort?: string; order?: 'asc' | 'desc' };

type UseUIPaginatedTableOptions<ResponseData, ApiData> = Omit<UseUITableOptions<ResponseData, ApiData, true>, 'api'> & {
  api: (params: PaginatedApiParams) => Promise<ResponseData>;
  paginationProps?: Partial<Omit<PaginationProps, 'total'>>;
  /**
   * whether to show the total count of the table
   *
   * @default true
   */
  showTotal?: boolean;
  onPaginationParamsChange?: (params: PaginationParams) => void | Promise<void>;
};

export function useUIPaginatedTable<ResponseData, ApiData>(options: UseUIPaginatedTableOptions<ResponseData, ApiData>) {
  const scope = effectScope();
  const appStore = useAppStore();

  const isMobile = computed(() => appStore.isMobile);

  let sizePickedByUser = false;

  const pagination: Partial<RemoveReadonly<PaginationProps & PaginationEmits>> = reactive({
    currentPage: 1,
    pageSize: 10,
    total: 0,
    pageSizes: [10, 15, 20, 25, 30],
    'current-change': (page: number) => {
      pagination.currentPage = page;

      return true;
    },
    'size-change': (pageSize: number) => {
      sizePickedByUser = true;
      pagination.currentPage = 1;
      pagination.pageSize = pageSize;

      return true;
    },
    ...options.paginationProps
  }) as PaginationProps;

  // this is for mobile, if the system does not support mobile, you can use `pagination` directly
  const mobilePagination = computed(() => {
    const p: Partial<RemoveReadonly<PaginationProps & PaginationEmits>> = {
      ...pagination,
      // Element Plus chỉ nhận pagerCount lẻ trong khoảng 5..21; giá trị 3 bị
      // validator loại và rơi về mặc định 7 — tức trên điện thoại dải số trang
      // lại dài hơn chứ không ngắn đi, kèm một cảnh báo Vue ở mọi bảng.
      pagerCount: isMobile.value ? 5 : 9,
      // Số dòng tự khớp (useFitPageSize) phải có trong ô chọn thì ô mới hiện đúng "13/trang".
      pageSizes: [...new Set([...(pagination.pageSizes ?? []), pagination.pageSize!])].sort((a, b) => a - b)
    };

    return p;
  });

  const paginationParams = computed(() => {
    const { currentPage, pageSize } = pagination;

    return {
      currentPage,
      pageSize
    };
  });

  // Sắp xếp phía server: cột khai báo sortable: 'custom', ElTable gắn @sort-change="onSortChange".
  // api nhận thêm sort (prop của cột) và order ('asc' | 'desc'); bỏ sắp xếp thì cả hai là undefined.
  const sort: { prop?: string; order?: 'asc' | 'desc' } = reactive({});

  const apiWrapper = () =>
    (options.api as any)({
      page: pagination.currentPage,
      pageSize: pagination.pageSize,
      sort: sort.prop,
      order: sort.order
    });

  const fitter = useFitPageSize({
    getPageSize: () => pagination.pageSize!,
    setPageSize: size => {
      pagination.currentPage = 1;
      pagination.pageSize = size;
    },
    isManual: () => sizePickedByUser
  });

  const result = useTable<ResponseData, ApiData, UI.TableColumn<ApiData>, true>({
    ...options,
    api: apiWrapper,
    pagination: true,
    // Tải lần đầu ở onMounted, sau khi đã khớp số dòng với chiều cao bảng.
    immediate: false,
    getColumnChecks: cols => getColumnChecks(cols, options.getColumnVisible),
    getColumns,
    onFetched: async data => {
      pagination.total = data.total;
      await fitter.afterLoad();
    }
  });

  onMounted(() => {
    // fit() đổi số dòng thì watcher paginationParams bên dưới tự tải.
    if (options.immediate !== false && !fitter.fit()) result.getData();
  });

  async function getDataByPage(page: number = 1) {
    if (page !== pagination.currentPage) {
      pagination.currentPage = page;

      return;
    }

    await result.getData();
  }

  function onSortChange({ prop, order }: { prop: string | null; order: 'ascending' | 'descending' | null }) {
    sort.prop = order && prop ? prop : undefined;
    sort.order = order ? SORT_ORDER[order] : undefined;
    getDataByPage(1);
  }

  scope.run(() => {
    watch(
      () => appStore.locale,
      () => {
        result.reloadColumns();
      }
    );

    watch(paginationParams, async newVal => {
      await options.onPaginationParamsChange?.(newVal);

      await result.getData();
    });
  });

  onScopeDispose(() => {
    scope.stop();
  });

  return {
    ...result,
    getDataByPage,
    onSortChange,
    pagination,
    mobilePagination
  };
}

export function useTableOperate<TableData>(
  data: Ref<TableData[]>,
  idKey: keyof TableData,
  getData: () => Promise<void>
) {
  const { bool: drawerVisible, setTrue: openDrawer, setFalse: closeDrawer } = useBoolean();

  const operateType = shallowRef<UI.TableOperateType>('add');

  function handleAdd() {
    operateType.value = 'add';
    openDrawer();
  }

  /** the editing row data */
  const editingData = shallowRef<TableData | null>(null);

  function handleEdit(id: TableData[keyof TableData]) {
    operateType.value = 'edit';
    const findItem = data.value.find(item => item[idKey] === id) || null;
    editingData.value = jsonClone(findItem);

    openDrawer();
  }

  /** the checked row keys of table */
  const checkedRowKeys = shallowRef<string[]>([]);

  /** the hook after the batch delete operation is completed */
  async function onBatchDeleted() {
    window.$message?.success($t('common.deleteSuccess'));

    checkedRowKeys.value = [];

    await getData();
  }

  /** the hook after the delete operation is completed */
  async function onDeleted() {
    window.$message?.success($t('common.deleteSuccess'));

    await getData();
  }

  return {
    drawerVisible,
    openDrawer,
    closeDrawer,
    operateType,
    handleAdd,
    editingData,
    handleEdit,
    checkedRowKeys,
    onBatchDeleted,
    onDeleted
  };
}

export function defaultTransform<ApiData>(
  response: FlatResponseData<any, Api.Common.PaginatingQueryRecord<ApiData>>
): PaginationData<ApiData> {
  const { data, error } = response;

  if (!error) {
    const { records, current, size, total } = data;

    return {
      data: records,
      pageNum: current,
      pageSize: size,
      total
    };
  }

  return {
    data: [],
    pageNum: 1,
    pageSize: 10,
    total: 0
  };
}

function getColumnChecks<Column extends UI.TableColumn<any>>(
  cols: Column[],
  getColumnVisible?: (column: Column) => boolean
) {
  const checks: TableColumnCheck[] = [];

  cols.forEach(column => {
    if (column.type === 'selection') {
      checks.push({
        prop: SELECTION_KEY,
        label: $t('common.check'),
        checked: true,
        visible: getColumnVisible?.(column) ?? false
      });
    } else if (column.type === 'expand') {
      checks.push({
        prop: EXPAND_KEY,
        label: $t('common.expandColumn'),
        checked: true,
        visible: getColumnVisible?.(column) ?? false
      });
    } else if (column.type === 'index') {
      checks.push({
        prop: INDEX_KEY,
        label: $t('common.index'),
        checked: true,
        visible: getColumnVisible?.(column) ?? false
      });
    } else {
      checks.push({
        prop: column.prop as string,
        label: column.label as string,
        checked: true,
        visible: getColumnVisible?.(column) ?? true
      });
    }
  });

  return checks;
}

function getColumns<Column extends UI.TableColumn<any>>(cols: Column[], checks: TableColumnCheck[]) {
  const columnMap = new Map<string, Column>();

  cols.forEach(column => {
    if (column.type === 'selection') {
      columnMap.set(SELECTION_KEY, column);
    } else if (column.type === 'expand') {
      columnMap.set(EXPAND_KEY, column);
    } else if (column.type === 'index') {
      columnMap.set(INDEX_KEY, column);
    } else {
      columnMap.set(column.prop as string, column);
    }
  });

  const filteredColumns = checks.filter(item => item.checked).map(check => columnMap.get(check.prop) as Column);

  return filteredColumns;
}
