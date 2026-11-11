<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dayjs from 'dayjs';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAuth } from '@/hooks/business/auth';
import { useRealtimeRefresh } from '@/hooks/common/realtime';
import { khongDau } from '@/utils/text';
import HubCatalogDialog from './modules/hub-catalog-dialog.vue';

const { t: $t } = useI18n();
const { hasAuth } = useAuth();

const loading = ref(false);
const rows = ref<any[]>([]);
const info = ref<{
  role: string;
  root: string;
  upstream_url: string;
  update_window: string;
  source: string;
  hub_url: string;
  has_key: boolean;
  key_hint: string;
  game_root: string;
  last_sync_at?: string;
  sync_error: string;
  key_status: string;
  key_expires_at?: string;
  key_grace_until?: string;
  events_connected: boolean;
  local_clash?: string[];
}>({
  role: '',
  root: '',
  upstream_url: '',
  update_window: '',
  source: '',
  hub_url: '',
  has_key: false,
  key_hint: '',
  game_root: '',
  sync_error: '',
  key_status: '',
  events_connected: false
});
// Ô riêng chứ không nằm trong info: vòng làm mới 3 giây ghi đè info, đang gõ dở sẽ mất chữ.
const clientRoot = ref('');
const updateWindow = ref('');
const hubUrl = ref('');
const hubKey = ref(''); // không bao giờ nạp từ máy chủ: máy chủ không trả key ra

// Tình trạng key do Hub báo (qua report). Sắp hết hạn: còn không quá 14 ngày.
const keyDaysLeft = computed(() => (info.value.key_expires_at ? dayjs(info.value.key_expires_at).diff(dayjs(), 'day') : 0));
const keyTagType = computed<'success' | 'warning' | 'danger' | 'info'>(() => {
  if (info.value.key_status === 'expired') return 'danger';
  if (info.value.key_status === 'grace') return 'warning';
  if (info.value.key_status === 'active') return keyDaysLeft.value <= 14 ? 'warning' : 'success';
  return 'info';
});
const keyLine = computed(() => {
  const exp = info.value.key_expires_at ? dayjs(info.value.key_expires_at).format('DD/MM/YYYY') : '';
  const grace = info.value.key_grace_until ? dayjs(info.value.key_grace_until).format('DD/MM/YYYY') : '';
  switch (info.value.key_status) {
    case 'active':
      return keyDaysLeft.value <= 14
        ? $t('vnetPages.games.keyLineExpiring', { date: exp, days: Math.max(keyDaysLeft.value, 0) })
        : $t('vnetPages.games.keyLineActive', { date: exp });
    case 'grace':
      return $t('vnetPages.games.keyLineGrace', { date: exp, grace });
    case 'expired':
      return $t('vnetPages.games.keyLineExpired', { date: exp });
    default:
      return '';
  }
});
const hubVisible = ref(false);
// Khung kết nối nằm trong hộp thoại: lỗi đọc catalog hay key sắp/đã hết hạn thì chấm đỏ trên nút mở để khỏi bị bỏ sót.
const hubAlert = computed(
  () =>
    info.value.source === 'settings' &&
    (Boolean(info.value.sync_error) || keyTagType.value === 'warning' || keyTagType.value === 'danger')
);
const isMaster = computed(() => info.value.role === 'master');
// Chưa bật: ô nào của khung kết nối còn trống, hoặc đã lưu đủ mà giá trị không còn dùng được.
const offStatus = computed(() => {
  const missing = [
    !info.value.hub_url && $t('vnetPages.games.hubUrl'),
    !info.value.has_key && $t('vnetPages.games.hubKey')
  ]
    .filter(Boolean)
    .join(', ');
  return missing ? $t('vnetPages.games.offMissing', { items: missing }) : $t('vnetPages.games.offSavedInvalid');
});
const isCafe = computed(() => info.value.role === 'cafe');
// Master phát hành game, không có game cục bộ. Quán không có thư mục game chung thì gõ thư mục chứa game ở hộp thoại.
const canAddLocal = computed(() => isCafe.value);
const isLocal = (row: any) => row.origin === 'local';

// Ở quán, bảng chỉ có game cục bộ và game Hub quán đã bấm Tải; game Hub khác nằm trong hộp "Tải game".
const coOQuan = (r: any) =>
  !isCafe.value || isLocal(r) || r.infohash || r.install_root || ['downloading', 'queued'].includes(r.status);
const search = ref('');
const shownRows = computed(() => {
  const q = khongDau(search.value.trim());
  return rows.value.filter(
    r =>
      coOQuan(r) &&
      (!q ||
        khongDau([r.custom?.display_name, r.display_name, r.name, r.category].filter(Boolean).join(' ')).includes(q))
  );
});

function formatSize(v: number) {
  if (!v) return '-';
  if (v >= 1024 ** 3) return `${(v / 1024 ** 3).toFixed(1)} GB`;
  return `${(v / 1024 ** 2).toFixed(1)} MB`;
}

const statusType: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'primary'> = {
  ready: 'success',
  downloading: 'primary',
  publishing: 'primary',
  queued: 'warning',
  error: 'danger',
  not_installed: 'info'
};

// Ở quán, "cần cập nhật" là khi master đã có bản mới hơn bản trên ổ.
function outdated(row: any) {
  return isCafe.value && row.remote_infohash && row.remote_infohash !== row.infohash;
}

let timer: ReturnType<typeof setTimeout> | undefined;

// Game đang tải ở hàng đợi của quán (cùng nguồn với hộp "Tải game"): tốc độ và số nguồn.
const speed = ref<{ name: string; bytes_per_sec: number; peers: number } | null>(null);

function bitRate(bps: number) {
  const bits = bps * 8;
  if (bits >= 1_000_000) return `${(bits / 1_000_000).toFixed(1)} Mbps`;
  return `${Math.round(bits / 1000)} kbps`;
}

// Thiếu tốc độ (master, hàng đợi lỗi) thì cột Trạng thái vẫn hiện % như cũ.
async function loadSpeed() {
  if (!isCafe.value || !rows.value.some(r => r.status === 'downloading')) {
    speed.value = null;
    return;
  }
  try {
    speed.value = ((await client.get('/games/queue')) as any)?.current || null;
  } catch {
    speed.value = null;
  }
}

// Tải lặng chỉ đổi ô nào CHƯA bị sửa (còn đúng giá trị máy chủ lần trước):
// người khác vừa đổi cài đặt thì ô theo kịp, còn ô đang gõ dở giữ nguyên chữ.
function syncFields(i: any, silent: boolean) {
  const cu: any = info.value;
  const fields: [typeof clientRoot, string, string][] = [
    [clientRoot, cu.client_root, i.client_root],
    [updateWindow, cu.update_window, i.update_window],
    [hubUrl, cu.hub_url, i.hub_url]
  ];
  for (const [field, before, now] of fields) {
    if (!silent || field.value === (before || '')) field.value = now || '';
  }
}

async function load(silent = false) {
  if (!silent) loading.value = true;
  try {
    const [i, list]: any = await Promise.all([client.get('/games/info'), client.get('/games')]);
    syncFields(i, silent);
    info.value = i;
    rows.value = list || [];
    await loadSpeed();
  } catch (e: any) {
    if (!silent) ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    loading.value = false;
  }
  // Đang tải thì cập nhật % mỗi 3 giây; xong hết thì thôi.
  clearTimeout(timer);
  if (rows.value.some(r => ['downloading', 'queued', 'publishing'].includes(r.status))) {
    timer = setTimeout(() => load(true), 3000);
  }
}

async function patch(row: any, body: Record<string, any>) {
  try {
    await client.put(`/games/${row.id}`, body);
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function publish(row: any) {
  row.publishing = true;
  try {
    await client.post(`/games/${row.id}/publish`, {});
    ElMessage.success($t('vnetPages.games.publishing', { name: row.display_name }));
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    row.publishing = false;
  }
}

async function syncNow(row: any, installRoot = '') {
  try {
    await client.post(`/games/${row.id}/sync`, installRoot ? { install_root: installRoot } : {});
    ElMessage.success($t('vnetPages.games.queued', { name: row.display_name }));
    load(true);
    return true;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    return false;
  }
}

// Hộp thoại Tải: game chưa tải bản nào thì quán chọn thư mục lưu trên máy chủ (chưa có thì máy chủ tự tạo). Gợi ý các
// thư mục quán đã dùng; game đã tải thì tải ngay vào chỗ cũ, không hỏi. Dòng từ hộp "Tải game" (hub: true) đi qua
// /games/hub-download để máy chủ chọn game trên Hub trước nếu cần.
const catalogVisible = ref(false);
const downloadRow = ref<any>(null);
const downloadRoot = ref('');
const downloading = ref(false);
const usedRoots = computed(() => {
  const seen = new Map<string, string>();
  for (const r of [...rows.value.map(x => x.install_root), info.value.root]) {
    if (r && !seen.has(r.toLowerCase())) seen.set(r.toLowerCase(), r);
  }
  return [...seen.values()];
});
const downloadPath = computed(() =>
  downloadRoot.value.trim() && downloadRow.value
    ? joinServerPath(downloadRoot.value.trim(), downloadRow.value.name)
    : ''
);
function suggestRoots(_q: string, cb: (items: { value: string }[]) => void) {
  cb(usedRoots.value.map(value => ({ value })));
}
function openDownload(row: any) {
  if (row.infohash) {
    syncNow(row);
    return;
  }
  downloadRow.value = row;
  downloadRoot.value = row.install_root || usedRoots.value[0] || '';
}
function openHubDownload(row: any) {
  downloadRow.value = { hub: true, name: row.name, display_name: row.display_name };
  downloadRoot.value = row.local?.install_root || usedRoots.value[0] || '';
}
async function hubDownload(row: any, installRoot: string) {
  try {
    await client.post('/games/hub-download', { name: row.name, install_root: installRoot });
    ElMessage.success($t('vnetPages.games.queued', { name: row.display_name }));
    load(true);
    return true;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    return false;
  }
}
async function submitDownload() {
  const row = downloadRow.value;
  const root = downloadRoot.value.trim();
  downloading.value = true;
  if (await (row.hub ? hubDownload(row, root) : syncNow(row, root))) downloadRow.value = null;
  downloading.value = false;
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.games.deleteConfirm', { name: row.display_name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/games/${row.id}`);
    ElMessage.success($t('vnetPages.common.deleted'));
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

const windowSaving = ref(false);
async function saveWindow() {
  windowSaving.value = true;
  try {
    await client.put('/settings/games', { update_window: updateWindow.value.trim() });
    ElMessage.success($t('vnetPages.common.success'));
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    windowSaving.value = false;
  }
}

const clientRootSaving = ref(false);
async function saveClientRoot() {
  clientRootSaving.value = true;
  try {
    await client.put('/settings/games', { client_root: clientRoot.value.trim() });
    ElMessage.success($t('vnetPages.common.success'));
    // Tải lại để ô hiện giá trị máy chủ thật sự dùng (đã chuẩn hoá, sai thì về mặc định).
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    clientRootSaving.value = false;
  }
}

const hubSaving = ref(false);
async function saveHub() {
  hubSaving.value = true;
  try {
    // Máy chủ đọc thử catalog bằng key này trước khi lưu: key sai hay địa chỉ sai báo lỗi ngay ở đây.
    await client.put('/games/hub', {
      hub_url: hubUrl.value.trim(),
      hub_key: hubKey.value.trim()
    });
    hubKey.value = '';
    ElMessage.success($t('vnetPages.games.hubConnected'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    hubSaving.value = false;
  }
}

const hubClearing = ref(false);
async function clearHub() {
  try {
    await ElMessageBox.confirm($t('vnetPages.games.hubDisconnectConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return;
  }
  hubClearing.value = true;
  try {
    await client.delete('/games/hub');
    hubKey.value = '';
    ElMessage.success($t('vnetPages.games.hubCleared'));
    load();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    hubClearing.value = false;
  }
}

const dialogVisible = ref(false);
const saving = ref(false);
// Một mục riêng trên menu máy trạm, cho thư mục có nhiều chương trình (LibreOffice → Word, Excel).
interface MenuEntry {
  display_name: string;
  launcher: string;
  launch_args: string;
  icon: string;
}
const emptyEntry = (): MenuEntry => ({ display_name: '', launcher: '', launch_args: '', icon: '' });
// Một bước chạy trước khi mở game (RunFirst).
interface RunStep {
  launcher: string;
  launch_args: string;
}
const emptyStep = (): RunStep => ({ launcher: '', launch_args: '' });
const cleanSteps = (list: RunStep[]) => list.filter(s => s.launcher.trim() !== '');
const emptyForm = () => ({
  name: '',
  // Thư mục game dưới "Thư mục chứa game" (game cục bộ). name để trống thì máy chủ lấy tên thư mục.
  folder_name: '',
  display_name: '',
  category: '',
  launcher: '',
  launch_args: '',
  icon: '',
  menu_entries: [] as MenuEntry[],
  run_first: [] as RunStep[],
  client_root: '',
  priority: 0
});
const form = ref(emptyForm());
// Hộp thêm/sửa game cục bộ: thư mục game chọn qua nhiều cấp thư mục của máy chủ; tệp chạy và icon chọn từ gợi ý (vẫn
// gõ tay được).
const localMode = ref(false);
// Các cấp của thư mục game đang chọn (giá trị của ElCascader), ví dụ ["E:\\", "E:\\Online Games", "E:\\Online Games\\VALORANT"].
const folderChain = ref<string[] | null>([]);
// Tăng mỗi lần mở hộp để ô chọn thư mục đọc lại thư mục từ đầu: thư mục vừa chép vào máy chủ cũng hiện ra.
const folderPickerKey = ref(0);
// Tên hiển thị tự điền theo thư mục đang chọn, cho tới khi người dùng tự sửa.
const autoDisplayName = ref('');
const localLaunchers = ref<string[]>([]);
const localIcons = ref<string[]>([]);
// Nhóm đã có để chọn lại; gõ tên mới thì thành nhóm mới.
const categories = computed(() =>
  [...new Set(rows.value.map(r => r.category).filter(Boolean) as string[])].sort((a, b) => a.localeCompare(b, 'vi'))
);

const trimSep = (p: string) => p.replace(/[\\/]+$/, '');
// Ghép thư mục trên máy chủ với tên con theo kiểu gạch của chính đường dẫn đó (máy chủ quán là Windows).
function joinServerPath(dir: string, name: string) {
  const sep = dir.includes('\\') || /^[A-Za-z]:/.test(dir) ? '\\' : '/';
  return trimSep(dir) + sep + name;
}
// Thư mục cha và tên của một đường dẫn trên máy chủ: "E:\\Online Games\\VALORANT" → ["E:\\Online Games", "VALORANT"].
function splitServerPath(p: string): [string, string] {
  const t = trimSep(p);
  const i = Math.max(t.lastIndexOf('\\'), t.lastIndexOf('/'));
  if (i < 0) return ['', t];
  const dir = t.slice(0, i);
  return [/^[A-Za-z]:$/.test(dir) ? `${dir}\\` : dir || '/', t.slice(i + 1)];
}
// Các cấp của một đường dẫn, đúng như máy chủ trả cho ô chọn thư mục (để ô hiện lại thư mục của game đang sửa).
function serverPathChain(p: string): string[] {
  const chain: string[] = [];
  for (let cur = p; cur; cur = splitServerPath(cur)[0]) chain.unshift(cur);
  return chain;
}
const localFolderPath = computed(() => (folderChain.value || []).slice(-1)[0] || '');
const localDir = computed(() => splitServerPath(localFolderPath.value)[0]);
// Gốc máy trạm máy chủ sẽ dùng khi để trống: thư mục chứa game, hoặc ổ chung khi đó chính là GAME_ROOT.
const clientRootDefault = computed(() =>
  trimSep(localDir.value).toLowerCase() === trimSep(info.value.root).toLowerCase()
    ? clientRoot.value
    : trimSep(localDir.value)
);
// Có id là đang sửa game đã có, rỗng là thêm mới. Thư mục game (name) là khoá nối
// master với các quán nên không đổi được.
const editingId = ref('');
// Sửa game cục bộ: đường dẫn thư mục lúc mở hộp; khác đường dẫn đang chọn thì gửi folder để trỏ game sang đó.
const editFolderPath = ref('');
const folderChanged = computed(
  () =>
    Boolean(editingId.value && localMode.value && localFolderPath.value) &&
    trimSep(localFolderPath.value).toLowerCase() !== trimSep(editFolderPath.value).toLowerCase()
);

function openCreate() {
  editingId.value = '';
  localMode.value = false;
  form.value = emptyForm();
  dialogVisible.value = true;
}

// Tệp chạy và tệp icon gợi ý trong thư mục game (gồm cả thư mục con). folder: tên thư mục trong GAME_ROOT hoặc đường dẫn
// đầy đủ trên máy chủ. quiet: không báo lỗi, chỉ là không có gợi ý.
const filesFor = ref('');
async function loadLocalFiles(folder: string, quiet = false) {
  filesFor.value = folder;
  localLaunchers.value = [];
  localIcons.value = [];
  if (!folder) return;
  try {
    const [launchers, icons] = await Promise.all(
      ['launcher', 'icon'].map(
        kind => client.get('/games/local/launchers', { params: { folder, kind } }) as Promise<any>
      )
    );
    localLaunchers.value = launchers || [];
    localIcons.value = icons || [];
  } catch (e: any) {
    if (!quiet) ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

// Ô chọn thư mục game: bấm vào thư mục là mở cấp dưới và chọn luôn thư mục đó, đi xuống tới thư mục game là xong. Cấp
// đầu là các ổ đĩa của máy chủ. Thư mục đã là game thì không chọn được, trừ thư mục cùng tên với game đang sửa (trỏ game
// sang bản ở chỗ khác). Mở cấp dưới khi rê chuột thì không dùng: danh sách giãn ra là tự dịch, chuột rê nhầm thư mục khác.
// Thư mục không chọn được (đã là game) theo lần đọc gần nhất: bấm vào chỉ mở cấp dưới.
const takenFolders = new Set<string>();
const folderProps = {
  lazy: true,
  checkStrictly: true,
  async lazyLoad(node: any, resolve: (nodes: any[]) => void) {
    const own = editingId.value ? splitServerPath(editFolderPath.value)[1].toLowerCase() : '';
    try {
      const dirs: any[] =
        (await client.get('/games/local/browse', { params: { path: node.level ? node.value : '' } })) || [];
      resolve(
        dirs.map(d => {
          const disabled = d.taken && d.name.toLowerCase() !== own;
          if (disabled) takenFolders.add(d.path);
          else takenFolders.delete(d.path);
          return { value: d.path, label: d.name, leaf: false, disabled };
        })
      );
    } catch (e: any) {
      ElMessage.error(e?.message || $t('vnetPages.common.error'));
      resolve([]);
    }
  }
};

function openCreateLocal() {
  editingId.value = '';
  localMode.value = true;
  form.value = emptyForm();
  editFolderPath.value = '';
  folderChain.value = [];
  folderPickerKey.value += 1;
  autoDisplayName.value = '';
  loadLocalFiles('');
  dialogVisible.value = true;
}

function onLocalFolderPicked() {
  const folder = splitServerPath(localFolderPath.value)[1];
  form.value.folder_name = folder;
  if (!folder) loadLocalFiles(''); // bấm xoá ô chọn
  if (editingId.value) {
    // Cùng game ở thư mục khác: giữ tệp chạy; gốc máy trạm để trống cho máy chủ lấy thư mục cha mới.
    form.value.client_root = '';
    return;
  }
  if (!form.value.display_name || form.value.display_name === autoDisplayName.value) {
    form.value.display_name = folder;
    autoDisplayName.value = folder;
  }
  form.value.launcher = '';
  form.value.icon = '';
}
function onFolderExpand(chain: any) {
  if (!takenFolders.has((chain as string[]).slice(-1)[0])) {
    folderChain.value = chain as string[];
    onLocalFolderPicked();
  }
  // Thư mục sâu nhiều cấp thì ô chọn cuộn ngang: đưa cấp vừa mở vào tầm nhìn.
  nextTick(() => {
    const panel = document.querySelector('.game-folder-picker .el-cascader-panel');
    if (panel) panel.scrollLeft = panel.scrollWidth;
  });
}
// Tệp chạy và icon đọc khi đóng ô chọn: đọc ở mỗi cấp đang đi qua (cả ổ E:\) thì máy chủ phải dò cả ổ.
function onFolderPanel(open: boolean) {
  if (!open && localFolderPath.value !== filesFor.value) loadLocalFiles(localFolderPath.value);
}

function openEdit(row: any) {
  editingId.value = row.id;
  localMode.value = isLocal(row);
  const folder = row.custom?.folder || row.name;
  if (localMode.value) {
    // Máy chủ không lưu đường dẫn của game thêm từ thư mục ngoài GAME_ROOT. Gốc máy trạm riêng của nó mặc định là thư
    // mục chứa game trên máy chủ nên thử ở đó; không thấy thì chỉ là không có gợi ý, vẫn gõ tay được.
    const cr = row.custom?.client_root;
    const base = cr || info.value.root;
    editFolderPath.value = base ? joinServerPath(base, folder) : '';
    folderChain.value = serverPathChain(editFolderPath.value);
    folderPickerKey.value += 1;
    loadLocalFiles(editFolderPath.value, Boolean(cr));
  }
  form.value = {
    name: row.name,
    folder_name: folder,
    display_name: row.display_name,
    category: row.category || '',
    launcher: row.launcher || '',
    launch_args: row.launch_args || '',
    icon: row.icon || '',
    menu_entries: (row.menu_entries || []).map((e: Partial<MenuEntry>) => ({ ...emptyEntry(), ...e })),
    run_first: (row.run_first || []).map((s: Partial<RunStep>) => ({ ...emptyStep(), ...s })),
    client_root: row.custom?.client_root || '',
    priority: row.priority || 0
  };
  dialogVisible.value = true;
}

// Hộp "Tuỳ chỉnh" của quán trên một game: tên, icon, gốc game và lệnh chạy trước riêng. Catalog không ghi đè.
const customVisible = ref(false);
const customSaving = ref(false);
const customRow = ref<any>(null);
const customForm = ref({
  display_name: '',
  icon: '',
  client_root: '',
  follow_run_first: true,
  run_first: [] as RunStep[]
});
const hasCustom = (row: any) => Boolean(row.custom && Object.keys(row.custom).length);

function openCustom(row: any) {
  customRow.value = row;
  const c = row.custom || {};
  customForm.value = {
    display_name: c.display_name || '',
    icon: c.icon || '',
    client_root: c.client_root || '',
    follow_run_first: !c.run_first,
    run_first: (c.run_first || []).map((s: Partial<RunStep>) => ({ ...emptyStep(), ...s }))
  };
  customVisible.value = true;
}

async function saveCustom() {
  const f = customForm.value;
  customSaving.value = true;
  try {
    await client.put(`/games/${customRow.value.id}/custom`, {
      display_name: f.display_name,
      icon: f.icon,
      client_root: f.client_root,
      ...(f.follow_run_first ? { reset_run_first: true } : { run_first: cleanSteps(f.run_first) })
    });
    ElMessage.success($t('vnetPages.common.success'));
    customVisible.value = false;
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    customSaving.value = false;
  }
}

// Tên trùng một game Hub quán chưa tải: hỏi trước, máy chủ chỉ thay khi có replace_hub. null là người dùng huỷ.
async function askReplaceHub(): Promise<boolean | null> {
  if (!localMode.value || editingId.value) return false;
  const name = form.value.name.trim() || form.value.folder_name;
  const hub = rows.value.find(r => r.name === name && !isLocal(r));
  if (!hub || hub.status !== 'not_installed' || hub.version) return false;
  try {
    await ElMessageBox.confirm($t('vnetPages.games.replaceHubConfirm', { name }), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
  } catch {
    return null;
  }
  return true;
}

async function submit() {
  const replaceHub = await askReplaceHub();
  if (replaceHub === null) return;
  saving.value = true;
  try {
    if (editingId.value) {
      const {
        name,
        display_name,
        category,
        launcher,
        launch_args,
        icon,
        menu_entries,
        run_first,
        client_root,
        priority
      } = form.value;
      await client.put(`/games/${editingId.value}`, {
        // Tên chỉ đổi được ở game cục bộ (nhường tên cho game Hub cùng tên).
        ...(localMode.value ? { name } : {}),
        display_name,
        category,
        launcher,
        launch_args,
        icon,
        menu_entries,
        run_first: cleanSteps(run_first),
        // Gốc game riêng chỉ đi cùng game cục bộ; game của Hub đặt ở hộp Tuỳ chỉnh.
        ...(localMode.value ? { client_root } : {}),
        ...(folderChanged.value ? { folder: localFolderPath.value } : {}),
        priority
      });
    } else {
      const { folder_name, ...body } = form.value;
      await client.post(localMode.value ? '/games/local' : '/games', {
        ...body,
        ...(localMode.value && folder_name ? { folder: localFolderPath.value } : {}),
        ...(replaceHub ? { replace_hub: true } : {}),
        run_first: cleanSteps(body.run_first)
      });
    }
    ElMessage.success($t('vnetPages.common.success'));
    dialogVisible.value = false;
    load(true);
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    saving.value = false;
  }
}

onMounted(() => load());
onBeforeUnmount(() => clearTimeout(timer));

// Trạng thái game đổi chủ yếu ở vòng nền (publish xong, tải xong hay lỗi, catalog
// của master có bản mới) — máy chủ báo "games". Khung giờ và gốc game nằm trong
// cài đặt ("settings"). Tải lặng (silent) để không ghi đè hai ô đang gõ dở; phần
// trăm đang tải vẫn hỏi mỗi 3 giây vì nó lấy thẳng từ engine, không ghi DB.
useRealtimeRefresh(['games', 'settings'], () => load(true));
</script>

<template>
  <div class="page-fill">
    <ElCard class="fill-card">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap">
          <div style="display: flex; align-items: center; gap: 8px">
            <ElTag :type="info.role ? 'success' : 'info'">
              {{ $t(`vnetPages.games.role_${info.role || 'off'}`) }}
            </ElTag>
            <span v-if="info.root" style="color: var(--el-text-color-secondary); font-size: 13px">
              {{ $t('vnetPages.games.root') }}: {{ info.root }}
            </span>
            <span v-if="isCafe" style="color: var(--el-text-color-secondary); font-size: 13px">
              · {{ $t('vnetPages.games.upstream') }}: {{ info.upstream_url }}
            </span>
          </div>
          <div style="display: flex; gap: 8px">
            <ElInput
              v-model="search"
              :placeholder="$t('vnetPages.games.searchPlaceholder')"
              :aria-label="$t('vnetPages.games.searchPlaceholder')"
              clearable
              class="w-220px"
            />
            <ElButton v-if="isCafe" type="primary" @click="catalogVisible = true">
              {{ $t('vnetPages.games.downloadGames') }}
            </ElButton>
            <ElBadge is-dot :hidden="!hubAlert">
              <ElButton @click="hubVisible = true">{{ $t('vnetPages.games.hubTitle') }}</ElButton>
            </ElBadge>
            <ElButton @click="load()">{{ $t('common.refresh') }}</ElButton>
            <ElButton v-if="isMaster && hasAuth('games.create')" type="primary" @click="openCreate">
              {{ $t('vnetPages.games.add') }}
            </ElButton>
            <ElButton v-if="canAddLocal && hasAuth('games.create')" type="primary" @click="openCreateLocal">
              {{ $t('vnetPages.games.addLocal') }}
            </ElButton>
          </div>
        </div>
      </template>

      <ElAlert v-if="!info.role" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.offHint') }}
        {{ offStatus }}
      </ElAlert>

      <ElTable v-loading="loading" :data="shownRows" border stripe class="fill-table" height="100%" style="width: 100%">
        <ElTableColumn
          :label="$t('vnetPages.games.name')"
          min-width="200"
          sortable
          :sort-by="(row: any) => row.custom?.display_name || row.display_name"
        >
          <template #default="{ row }">
            <div style="font-weight: 600">
              {{ row.custom?.display_name || row.display_name }}
              <ElTag v-if="isLocal(row)" size="small" type="warning" style="margin-left: 6px">
                {{ $t('vnetPages.games.localTag') }}
              </ElTag>
              <ElTag v-if="hasCustom(row)" size="small" type="info" style="margin-left: 6px">
                {{ $t('vnetPages.games.customTag') }}
              </ElTag>
            </div>
            <div style="color: var(--el-text-color-secondary); font-size: 12px">
              {{ row.install_root ? joinServerPath(row.install_root, row.name) : row.name }}
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="category" :label="$t('vnetPages.games.category')" width="120" sortable>
          <template #default="{ row }">{{ row.category || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="version" :label="$t('vnetPages.games.version')" width="120" sortable>
          <template #default="{ row }">
            <span>{{ row.version || '-' }}</span>
            <span v-if="outdated(row)" style="color: var(--el-color-warning)">→ {{ row.remote_version }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="size_bytes" :label="$t('vnetPages.games.size')" width="100" sortable>
          <template #default="{ row }">{{ formatSize(row.size_bytes) }}</template>
        </ElTableColumn>
        <!-- Số lượt khách bấm "Chơi" trên menu máy trạm (từ client 1.0.19). -->
        <ElTableColumn
          prop="launch_count"
          :label="$t('vnetPages.games.launchCount')"
          width="100"
          align="right"
          sortable
        >
          <template #default="{ row }">{{ (row.launch_count || 0).toLocaleString('vi-VN') }}</template>
        </ElTableColumn>
        <ElTableColumn prop="status" :label="$t('vnetPages.games.status')" min-width="190" sortable>
          <template #default="{ row }">
            <ElTag :type="statusType[row.status] || 'info'" size="small">
              {{ $t(`vnetPages.games.status_${row.status || 'not_installed'}`) }}
            </ElTag>
            <ElProgress v-if="row.status === 'downloading'" :percentage="row.progress" style="margin-top: 6px" />
            <div
              v-if="row.status === 'downloading' && speed && speed.name === row.name"
              class="mt-4px text-12px text-[var(--el-text-color-secondary)]"
            >
              {{ bitRate(speed.bytes_per_sec) }} · {{ $t('vnetPages.hubGames.peers', { n: speed.peers }) }}
            </div>
            <div
              v-if="row.status === 'error' && row.error"
              style="color: var(--el-color-danger); font-size: 12px; margin-top: 4px"
            >
              {{ row.error }}
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="auto_update" :label="$t('vnetPages.games.autoUpdate')" width="110" sortable>
          <template #default="{ row }">
            <span v-if="isLocal(row)" style="color: var(--el-text-color-secondary)">-</span>
            <ElSwitch
              v-else
              :model-value="row.auto_update"
              :disabled="!hasAuth('games.update')"
              @change="(v: any) => patch(row, { auto_update: v })"
            />
          </template>
        </ElTableColumn>
        <ElTableColumn prop="enabled" :label="$t('vnetPages.games.enabled')" width="90" sortable>
          <template #default="{ row }">
            <ElSwitch
              :model-value="row.enabled"
              :disabled="!hasAuth('games.update')"
              @change="(v: any) => patch(row, { enabled: v })"
            />
          </template>
        </ElTableColumn>
        <ElTableColumn prop="priority" :label="$t('vnetPages.games.priority')" width="120" sortable>
          <template #default="{ row }">
            <ElInputNumber
              :model-value="row.priority"
              size="small"
              controls-position="right"
              style="width: 90px"
              :disabled="!hasAuth('games.update')"
              @change="(v: any) => patch(row, { priority: Number(v) || 0 })"
            />
          </template>
        </ElTableColumn>
        <ElTableColumn prop="last_updated_at" :label="$t('vnetPages.games.lastUpdated')" width="150" sortable>
          <template #default="{ row }">
            {{ row.last_updated_at ? dayjs(row.last_updated_at).format('DD/MM/YYYY HH:mm') : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="250" fixed="right">
          <template #default="{ row }">
            <template v-if="isMaster">
              <ElButton v-if="hasAuth('games.update')" size="small" @click="openEdit(row)">
                {{ $t('vnetPages.common.edit') }}
              </ElButton>
              <ElButton
                v-if="hasAuth('games.update')"
                size="small"
                type="primary"
                plain
                :loading="row.publishing"
                :disabled="row.status === 'publishing'"
                @click="publish(row)"
              >
                {{ $t('vnetPages.games.publishNow') }}
              </ElButton>
              <ElButton v-if="hasAuth('games.delete')" size="small" type="danger" plain @click="handleDelete(row)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
            </template>
            <template v-else-if="isCafe && isLocal(row)">
              <ElButton v-if="hasAuth('games.update')" size="small" @click="openEdit(row)">
                {{ $t('vnetPages.common.edit') }}
              </ElButton>
              <ElButton v-if="hasAuth('games.delete')" size="small" type="danger" plain @click="handleDelete(row)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
            </template>
            <template v-else-if="isCafe">
              <ElButton v-if="hasAuth('games.update')" size="small" @click="openCustom(row)">
                {{ $t('vnetPages.games.customize') }}
              </ElButton>
              <ElButton
                v-if="hasAuth('games.update')"
                size="small"
                type="primary"
                plain
                :disabled="row.status === 'downloading' || !row.remote_infohash"
                @click="openDownload(row)"
              >
                {{ $t('vnetPages.games.syncNow') }}
              </ElButton>
            </template>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog v-model="hubVisible" :title="$t('vnetPages.games.hubTitle')" width="880px">
      <ElAlert
        v-if="info.local_clash && info.local_clash.length"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      >
        {{ $t('vnetPages.games.localClash', { names: info.local_clash.join(', ') }) }}
      </ElAlert>

      <ElAlert v-if="info.role" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ isMaster ? $t('vnetPages.games.masterHint') : $t('vnetPages.games.cafeHint') }}
      </ElAlert>

      <div class="mb-16px">
        <ElAlert v-if="info.source === 'env'" type="info" :closable="false" show-icon>
          {{ $t('vnetPages.games.hubEnvHint') }}
        </ElAlert>
        <template v-else>
          <div class="flex flex-wrap items-end gap-8px">
            <div>
              <div class="mb-2px text-12px text-gray-500">{{ $t('vnetPages.games.hubUrl') }}</div>
              <ElInput
                v-model="hubUrl"
                :placeholder="$t('vnetPages.games.hubUrlPlaceholder')"
                :aria-label="$t('vnetPages.games.hubUrl')"
                class="w-260px"
                clearable
              />
            </div>
            <div>
              <div class="mb-2px text-12px text-gray-500">{{ $t('vnetPages.games.hubKey') }}</div>
              <ElInput
                v-model="hubKey"
                type="password"
                show-password
                autocomplete="new-password"
                :placeholder="
                  info.has_key
                    ? $t('vnetPages.games.hubKeySaved', { hint: info.key_hint })
                    : 'VGK-XXXXX-XXXXX-XXXXX-XXXXX'
                "
                :aria-label="$t('vnetPages.games.hubKey')"
                class="w-300px"
              />
            </div>
            <ElButton v-if="hasAuth('games.hub')" type="primary" :loading="hubSaving" @click="saveHub">
              {{ $t('vnetPages.games.hubConnect') }}
            </ElButton>
            <ElButton
              v-if="(info.hub_url || info.has_key) && hasAuth('games.hub')"
              :loading="hubClearing"
              @click="clearHub"
            >
              {{ $t('vnetPages.games.hubDisconnect') }}
            </ElButton>
          </div>
          <div class="mt-4px text-12px text-gray-500">{{ $t('vnetPages.games.hubKeyKeepHint') }}</div>
          <div v-if="info.source === 'settings'" class="mt-4px text-12px">
            <span v-if="info.sync_error" style="color: var(--el-color-danger)">
              {{ $t('vnetPages.games.hubSyncError', { error: info.sync_error }) }}
            </span>
            <span v-else-if="info.last_sync_at" style="color: var(--el-color-success)">
              {{
                $t('vnetPages.games.hubSyncOk', { time: dayjs(info.last_sync_at).format('DD/MM/YYYY HH:mm:ss') })
              }}
            </span>
            <span v-else class="text-gray-500">{{ $t('vnetPages.games.hubConnecting') }}</span>
            <span class="ml-8px text-gray-500">
              · {{ info.events_connected ? $t('vnetPages.games.eventsOn') : $t('vnetPages.games.eventsOff') }}
            </span>
          </div>
          <div v-if="info.source === 'settings' && info.key_status" class="mt-4px flex items-center gap-8px text-12px">
            <ElTag size="small" :type="keyTagType">{{ $t(`vnetPages.games.keyStatus_${info.key_status}`) }}</ElTag>
            <span>{{ keyLine }}</span>
          </div>
        </template>
      </div>

      <div v-if="info.role && info.root" class="mb-16px flex flex-wrap items-center gap-8px">
        <span>{{ $t('vnetPages.games.clientRoot') }}</span>
        <ElInput v-model="clientRoot" placeholder="D:\Games" class="w-240px" clearable />
        <!-- Lưu vào PUT /settings/games → quyền settings.edit -->
        <ElButton v-if="hasAuth('settings.edit')" :loading="clientRootSaving" @click="saveClientRoot">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
        <span class="text-12px text-gray-500">{{ $t('vnetPages.games.clientRootHint') }}</span>
      </div>

      <div v-if="isCafe" style="display: flex; align-items: center; gap: 8px; margin-bottom: 16px">
        <span>{{ $t('vnetPages.games.updateWindow') }}</span>
        <ElInput v-model="updateWindow" placeholder="02:00-08:00" style="width: 160px" clearable />
        <ElButton v-if="hasAuth('settings.edit')" :loading="windowSaving" @click="saveWindow">
          {{ $t('vnetPages.common.save') }}
        </ElButton>
        <span style="color: var(--el-text-color-secondary); font-size: 12px">
          {{ $t('vnetPages.games.updateWindowHint') }}
        </span>
      </div>
      <template #footer>
        <ElButton @click="hubVisible = false">{{ $t('common.close') }}</ElButton>
      </template>
    </ElDialog>

    <HubCatalogDialog v-model:visible="catalogVisible" @download="openHubDownload" />

    <ElDialog
      :model-value="Boolean(downloadRow)"
      :title="
        $t('vnetPages.games.downloadTitle', { name: downloadRow?.custom?.display_name || downloadRow?.display_name })
      "
      width="560px"
      @update:model-value="downloadRow = null"
    >
      <ElForm label-position="top" @submit.prevent="submitDownload">
        <ElFormItem :label="$t('vnetPages.games.downloadRoot')">
          <ElAutocomplete
            v-model="downloadRoot"
            :fetch-suggestions="suggestRoots"
            placeholder="E:\Games"
            clearable
            class="w-full"
          />
          <div class="mt-4px text-12px text-gray-500">
            {{ downloadPath ? $t('vnetPages.games.downloadPath', { path: downloadPath }) : '' }}
            {{ $t('vnetPages.games.downloadRootHint') }}
          </div>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="downloadRow = null">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="downloading" :disabled="!downloadRoot.trim()" @click="submitDownload">
          {{ $t('vnetPages.games.syncNow') }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? $t('vnetPages.common.edit') : localMode ? $t('vnetPages.games.addLocal') : $t('vnetPages.games.add')"
      width="720px"
    >
      <ElAlert v-if="localMode" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.localHint', { root: info.root }) }}
      </ElAlert>
      <ElAlert v-else-if="!editingId" type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.addHint', { root: info.root }) }}
      </ElAlert>
      <ElForm label-width="150px">
        <ElFormItem v-if="localMode" :label="$t('vnetPages.games.localFolder')">
          <div class="w-full">
            <ElCascader
              :key="folderPickerKey"
              v-model="folderChain"
              :props="folderProps"
              clearable
              class="w-full"
              popper-class="game-folder-picker"
              :placeholder="$t('vnetPages.games.localFolderPick')"
              @change="onLocalFolderPicked"
              @expand-change="onFolderExpand"
              @visible-change="onFolderPanel"
            />
            <div class="text-12px text-gray-500">
              {{ localFolderPath ? $t('vnetPages.games.localFolderPath', { path: localFolderPath }) : '' }}
              {{ $t(editingId ? 'vnetPages.games.localFolderEditHint' : 'vnetPages.games.localFolderHint') }}
            </div>
          </div>
        </ElFormItem>
        <ElFormItem v-else :label="$t('vnetPages.games.folder')">
          <ElInput v-if="editingId" :model-value="form.folder_name" disabled />
          <ElInput v-else v-model="form.name" placeholder="League of Legends" />
        </ElFormItem>
        <ElFormItem v-if="localMode" :label="$t('vnetPages.games.localName')">
          <ElInput v-model="form.name" :placeholder="form.folder_name" />
          <div class="text-12px text-gray-500">
            {{ $t(editingId ? 'vnetPages.games.localRenameHint' : 'vnetPages.games.localNameHint') }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.displayName')">
          <ElInput v-model="form.display_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.category')">
          <ElSelect
            v-model="form.category"
            filterable
            allow-create
            default-first-option
            clearable
            class="w-full"
            :placeholder="$t('vnetPages.games.categoryPick')"
          >
            <ElOption v-for="c in categories" :key="c" :label="c" :value="c" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.launcher')">
          <ElSelect
            v-if="localMode"
            v-model="form.launcher"
            filterable
            allow-create
            default-first-option
            class="w-full"
            :placeholder="$t('vnetPages.games.localLauncherPick')"
          >
            <ElOption v-for="f in localLaunchers" :key="f" :label="f" :value="f" />
          </ElSelect>
          <ElInput v-else v-model="form.launcher" placeholder="Riot Client/RiotClientServices.exe" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.launchArgs')">
          <ElInput v-model="form.launch_args" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.icon')">
          <ElSelect
            v-if="localMode"
            v-model="form.icon"
            filterable
            allow-create
            default-first-option
            clearable
            class="w-full"
            :placeholder="$t('vnetPages.games.localIconPick')"
          >
            <ElOption v-for="f in localIcons" :key="f" :label="f" :value="f" />
          </ElSelect>
          <ElInput v-else v-model="form.icon" placeholder="League of Legends/LeagueClient.exe" />
          <div class="text-12px text-gray-500">{{ $t('vnetPages.games.iconHint') }}</div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.menuEntries')">
          <div class="w-full">
            <div class="text-12px text-gray-500">{{ $t('vnetPages.games.menuEntriesHint') }}</div>
            <div
              v-for="(e, i) in form.menu_entries"
              :key="i"
              class="grid grid-cols-[1fr_1fr_auto] mt-8px items-center gap-6px"
            >
              <ElInput v-model="e.display_name" :placeholder="$t('vnetPages.games.entryName')" />
              <ElInput v-model="e.launcher" placeholder="program/swriter.exe" />
              <ElButton type="danger" link @click="form.menu_entries.splice(i, 1)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
              <ElInput v-model="e.launch_args" :placeholder="$t('vnetPages.games.launchArgs')" />
              <ElInput v-model="e.icon" :placeholder="$t('vnetPages.games.icon')" />
            </div>
            <ElButton class="mt-8px" size="small" @click="form.menu_entries.push(emptyEntry())">
              {{ $t('vnetPages.games.addEntry') }}
            </ElButton>
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.runFirst')">
          <div class="w-full">
            <div class="text-12px text-gray-500">{{ $t('vnetPages.games.runFirstHint') }}</div>
            <div v-for="(s, i) in form.run_first" :key="i" class="grid grid-cols-[1fr_1fr_auto] mt-8px items-center gap-6px">
              <ElInput v-model="s.launcher" :placeholder="$t('vnetPages.games.stepLauncher')" />
              <ElInput v-model="s.launch_args" :placeholder="$t('vnetPages.games.stepArgs')" />
              <ElButton type="danger" link @click="form.run_first.splice(i, 1)">
                {{ $t('vnetPages.common.delete') }}
              </ElButton>
            </div>
            <ElButton v-if="form.run_first.length < 5" class="mt-8px" size="small" @click="form.run_first.push(emptyStep())">
              {{ $t('vnetPages.games.addStep') }}
            </ElButton>
          </div>
        </ElFormItem>
        <ElFormItem v-if="localMode" :label="$t('vnetPages.games.clientRootLocal')">
          <ElInput
            v-model="form.client_root"
            :placeholder="editingId && !folderChanged ? 'E:\\Games2' : clientRootDefault"
          />
          <div class="text-12px text-gray-500">
            {{
              $t(
                editingId && !folderChanged
                  ? 'vnetPages.games.clientRootLocalHint'
                  : 'vnetPages.games.clientRootFolderHint'
              )
            }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.priority')">
          <ElInputNumber v-model="form.priority" style="width: 100%" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="submit">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="customVisible" :title="$t('vnetPages.games.customTitle')" width="680px">
      <ElAlert type="info" :closable="false" show-icon style="margin-bottom: 16px">
        {{ $t('vnetPages.games.customHint') }}
      </ElAlert>
      <ElForm v-if="customRow" label-width="150px">
        <ElFormItem :label="$t('vnetPages.games.customDisplayName')">
          <ElInput v-model="customForm.display_name" :placeholder="customRow.display_name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.customIcon')">
          <ElInput v-model="customForm.icon" :placeholder="customRow.icon || customRow.launcher" />
          <div class="text-12px text-gray-500">{{ $t('vnetPages.games.iconHint') }}</div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.clientRootLocal')">
          <ElInput v-model="customForm.client_root" placeholder="E:\Games2" />
          <div class="text-12px text-gray-500">{{ $t('vnetPages.games.clientRootLocalHint') }}</div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.games.runFirst')">
          <div class="w-full">
            <ElCheckbox v-model="customForm.follow_run_first">{{ $t('vnetPages.games.customRunFirstFollow') }}</ElCheckbox>
            <template v-if="!customForm.follow_run_first">
              <div class="text-12px text-gray-500">{{ $t('vnetPages.games.runFirstHint') }}</div>
              <div
                v-for="(s, i) in customForm.run_first"
                :key="i"
                class="grid grid-cols-[1fr_1fr_auto] mt-8px items-center gap-6px"
              >
                <ElInput v-model="s.launcher" :placeholder="$t('vnetPages.games.stepLauncher')" />
                <ElInput v-model="s.launch_args" :placeholder="$t('vnetPages.games.stepArgs')" />
                <ElButton type="danger" link @click="customForm.run_first.splice(i, 1)">
                  {{ $t('vnetPages.common.delete') }}
                </ElButton>
              </div>
              <ElButton
                v-if="customForm.run_first.length < 5"
                class="mt-8px"
                size="small"
                @click="customForm.run_first.push(emptyStep())"
              >
                {{ $t('vnetPages.games.addStep') }}
              </ElButton>
            </template>
          </div>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="customVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="customSaving" @click="saveCustom">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style>
/* Ô chọn thư mục game nằm ngoài trang (popper): nhiều cấp thì cuộn ngang thay vì tràn khỏi màn hình. */
.game-folder-picker .el-cascader-panel {
  max-width: 90vw;
  overflow-x: auto;
}

/* Thư mục đã là game: mờ cả tên, không chỉ nút chọn (vẫn bấm vào để mở cấp dưới được). */
.game-folder-picker .el-cascader-node:has(.el-radio.is-disabled) .el-cascader-node__label {
  color: var(--el-text-color-placeholder);
}
</style>
