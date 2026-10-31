<script setup lang="ts">
// Lệnh cho nhiều máy một lúc: thông báo lên màn hình, khởi động lại, tắt máy, bật
// máy. Dùng chung cho Bảng điều khiển và trang Máy, như machine-remote-actions.vue
// cho từng máy.
//
// Không có API hàng loạt: mỗi máy đi qua đúng API của lệnh đơn lẻ
// (POST /machines/:id/remote/:action, /machines/:id/wake), nên nhật ký, chốt tiền
// phiên khi tắt/khởi động lại và mã 409 "máy chưa kết nối" giống hệt bấm từng máy.
// Gửi song song: mỗi máy một phiên, một hội viên riêng, không khoá chung dòng nào.
import { computed, ref } from 'vue';
import { ElMessageBox, ElNotification } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useAuth } from '@/hooks/business/auth';

defineOptions({ name: 'MachineBatchActions' });

interface MayChon {
  id: string;
  machine_code: string;
  status?: string;
}

// selected: các máy đang tích. online: danh sách máy đang bật cho nút "Thông báo tất
// cả máy đang bật" khi chưa tích máy nào — mảng thì hiện luôn số máy, hàm thì tải
// lúc bấm (trang Máy chỉ có một trang dữ liệu trong tay). wake: hiện nút Bật máy.
// Nút riêng của trang (Gán nhóm) đặt vào slot, đứng trước Bỏ chọn.
const props = defineProps<{
  selected: MayChon[];
  online?: MayChon[] | (() => Promise<MayChon[]>);
  wake?: boolean;
}>();
const emit = defineEmits<{ clear: []; done: [] }>();

const { t: $t } = useI18n();
const { hasAuth } = useAuth();
const remote = computed(() => hasAuth('machines.remote'));
const busy = ref(false);

function danhSachMa(list: MayChon[]) {
  const ma = list.map(m => m.machine_code);
  return ma.length > 12 ? `${ma.slice(0, 12).join(', ')}… (+${ma.length - 12})` : ma.join(', ');
}

// Máy lỗi phải hiện đủ từng mã: một dòng "8/10" không cho biết hai máy nào chưa
// nhận lệnh. 409 tách riêng: lệnh đúng, chỉ là máy đang tắt (hoặc đang bật, với
// lệnh bật máy) — không phải sự cố.
async function chay(list: MayChon[], call: (m: MayChon) => Promise<unknown>, nhan409: string) {
  busy.value = true;
  const kq = await Promise.allSettled(list.map(m => call(m)));
  busy.value = false;
  const ma409: MayChon[] = [];
  const loi: string[] = [];
  kq.forEach((r, i) => {
    if (r.status === 'fulfilled') return;
    const e: any = r.reason;
    if (e?.status === 409) ma409.push(list[i]);
    else loi.push(`${list[i].machine_code}: ${e?.message || $t('vnetPages.common.error')}`);
  });
  const tom = $t('vnetPages.machines.bulk.done', { ok: list.length - ma409.length - loi.length, total: list.length });
  if (!ma409.length && !loi.length) {
    ElNotification({ type: 'success', title: $t('vnetPages.common.success'), message: tom });
  } else {
    const dong = [];
    if (ma409.length) dong.push($t(nhan409, { codes: danhSachMa(ma409) }));
    if (loi.length) dong.push($t('vnetPages.machines.bulk.failedList', { list: loi.join('; ') }));
    ElMessageBox.alert(dong.join('\n'), tom, { type: 'warning', customStyle: { whiteSpace: 'pre-line' } });
  }
  emit('done');
}

async function thongBao(list: MayChon[]) {
  if (!list.length) return;
  let value: string;
  try {
    ({ value } = await ElMessageBox.prompt(
      $t('vnetPages.machines.remote.messagePrompt'),
      $t('vnetPages.machines.bulk.messageTitle', { n: list.length }),
      { inputPattern: /\S/, inputErrorMessage: $t('vnetPages.machines.remote.messageRequired') }
    ));
  } catch {
    return;
  }
  await chay(
    list,
    m => client.post(`/machines/${m.id}/remote/message`, { title: 'VNET', message: value }),
    'vnetPages.machines.bulk.offlineList'
  );
}

async function thongBaoMayDangBat() {
  let list: MayChon[];
  if (typeof props.online === 'function') {
    busy.value = true;
    try {
      list = await props.online();
    } finally {
      busy.value = false;
    }
  } else {
    list = props.online || [];
  }
  await thongBao(list);
}

async function nguon(action: 'restart' | 'shutdown') {
  const list = props.selected;
  const dangChoi = list.filter(m => m.status === 'in_use').length;
  const dong = [
    $t('vnetPages.machines.bulk.confirmPower', {
      action: $t(`vnetPages.machines.bulk.${action}`),
      n: list.length,
      codes: danhSachMa(list)
    })
  ];
  if (dangChoi) dong.push($t('vnetPages.machines.bulk.inUseWarn', { n: dangChoi }));
  try {
    await ElMessageBox.confirm(dong.join('\n'), $t('vnetPages.common.confirm'), {
      type: 'warning',
      customStyle: { whiteSpace: 'pre-line' }
    });
  } catch {
    return;
  }
  await chay(list, m => client.post(`/machines/${m.id}/remote/${action}`, {}), 'vnetPages.machines.bulk.offlineList');
}

async function batMay() {
  const list = props.selected;
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.machines.bulk.wakeConfirm', { n: list.length, codes: danhSachMa(list) }),
      $t('vnetPages.common.confirm'),
      { type: 'info' }
    );
  } catch {
    return;
  }
  await chay(list, m => client.post(`/machines/${m.id}/wake`), 'vnetPages.machines.bulk.alreadyOnList');
}
</script>

<template>
  <div v-if="selected.length" class="machine-batch">
    <span class="machine-batch__count">{{ $t('vnetPages.machines.bulk.selected', { n: selected.length }) }}</span>
    <template v-if="remote">
      <ElButton size="small" type="primary" :loading="busy" @click="thongBao(selected)">
        {{ $t('vnetPages.machines.bulk.message') }}
      </ElButton>
      <ElButton size="small" type="warning" plain :disabled="busy" @click="nguon('restart')">
        {{ $t('vnetPages.machines.bulk.restart') }}
      </ElButton>
      <ElButton size="small" type="danger" plain :disabled="busy" @click="nguon('shutdown')">
        {{ $t('vnetPages.machines.bulk.shutdown') }}
      </ElButton>
      <ElButton v-if="wake" size="small" :disabled="busy" @click="batMay">
        {{ $t('vnetPages.machines.bulk.wake') }}
      </ElButton>
    </template>
    <slot />
    <ElButton size="small" link @click="emit('clear')">{{ $t('vnetPages.machines.bulk.clear') }}</ElButton>
  </div>
  <ElButton
    v-else-if="online && remote"
    size="small"
    type="primary"
    plain
    :loading="busy"
    :disabled="Array.isArray(online) && !online.length"
    @click="thongBaoMayDangBat"
  >
    {{
      Array.isArray(online)
        ? $t('vnetPages.machines.bulk.notifyOnline', { n: online.length })
        : $t('vnetPages.machines.bulk.notifyOnlineAll')
    }}
  </ElButton>
</template>

<style scoped>
.machine-batch {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.machine-batch :deep(.el-button + .el-button) {
  margin-left: 0;
}

.machine-batch__count {
  font-size: 13px;
  font-weight: 600;
}
</style>
