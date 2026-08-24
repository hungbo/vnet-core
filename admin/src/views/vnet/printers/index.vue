<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, ElNotification, ElTag } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUITable } from '@/hooks/common/table';
import { vnetSimpleTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';

const { t: $t } = useI18n();

const { columns, columnChecks, data, getData, loading } = useUITable({
  api: async () => {
    const res: any = await client.get('/printers');
    return { items: Array.isArray(res) ? res : res?.items || [] };
  },
  transform: vnetSimpleTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.printers.name'), minWidth: 140 },
    { prop: 'printer_type', label: $t('vnetPages.printers.type'), width: 110 },
    {
      prop: 'ip_address',
      label: $t('vnetPages.printers.address'),
      minWidth: 150,
      formatter: (row: any) => (row.ip_address ? `${row.ip_address}:${row.port}` : '-')
    },
    {
      prop: 'chars_per_line',
      label: $t('vnetPages.printers.paper'),
      width: 110,
      formatter: (row: any) => (row.chars_per_line >= 48 ? '80mm' : '58mm')
    },
    {
      prop: 'encoding',
      label: $t('vnetPages.printers.encoding'),
      width: 130,
      formatter: (row: any) =>
        row.encoding === 'cp1258' ? $t('vnetPages.printers.encodingVi') : $t('vnetPages.printers.encodingAscii')
    },
    {
      prop: 'is_default',
      label: $t('vnetPages.printers.isDefault'),
      width: 110,
      formatter: (row: any) =>
        row.is_default ? h(ElTag, { type: 'success', size: 'small' }, () => $t('vnetPages.printers.defaultTag')) : '-'
    }
  ]
});

// --- Thêm / sửa --------------------------------------------------------------

const dialogVisible = ref(false);
const submitting = ref(false);
const editingId = ref('');
const formRef = ref<FormInstance>();
const form = ref({
  name: '',
  printer_type: 'thermal',
  ip_address: '',
  port: 9100,
  is_default: false,
  chars_per_line: 32,
  encoding: 'ascii',
  code_page: 0
});

const rules: FormRules = {
  name: [
    {
      required: true,
      message: $t('vnetPages.printers.nameRequired'),
      trigger: 'blur'
    }
  ],
  ip_address: [
    {
      required: true,
      message: $t('vnetPages.printers.addressRequired'),
      trigger: 'blur'
    }
  ]
};

function openCreate() {
  editingId.value = '';
  form.value = {
    name: '',
    printer_type: 'thermal',
    ip_address: '',
    port: 9100,
    is_default: false,
    chars_per_line: 32,
    encoding: 'ascii',
    code_page: 0
  };
  dialogVisible.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  form.value = {
    name: row.name || '',
    printer_type: row.printer_type || 'thermal',
    ip_address: row.ip_address || '',
    port: row.port || 9100,
    is_default: Boolean(row.is_default),
    chars_per_line: row.chars_per_line || 32,
    encoding: row.encoding || 'ascii',
    code_page: row.code_page || 0
  };
  dialogVisible.value = true;
}

async function submit() {
  const ok = await formRef.value?.validate().catch(() => false);
  if (!ok) return;
  submitting.value = true;
  try {
    if (editingId.value) {
      await client.put(`/printers/${editingId.value}`, form.value);
    } else {
      await client.post('/printers', form.value);
    }
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.common.saved')
    });
    dialogVisible.value = false;
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    submitting.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(
      $t('vnetPages.printers.deleteConfirm', { name: row.name }),
      $t('vnetPages.common.confirm'),
      { type: 'warning' }
    );
  } catch {
    return;
  }
  try {
    await client.delete(`/printers/${row.id}`);
    ElMessage.success($t('vnetPages.common.deleted'));
    getData();
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  }
}

async function handleTest(row: any) {
  try {
    await client.post(`/printers/${row.id}/test`);
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.printers.testSent', { name: row.name })
    });
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.printers.testFailed'));
  }
}

// --- Phân luồng món ----------------------------------------------------------
// Bảng product_printer_mappings quyết định món nào ra máy in nào. Món không gán
// cho máy nào thì không có phiếu chế biến — backend trả về danh sách đó khi in.

const mapVisible = ref(false);
const mapLoading = ref(false);
const mapSaving = ref(false);
const mapPrinter = ref<any>(null);
const products = ref<any[]>([]);
const selectedProducts = ref<string[]>([]);
const productFilter = ref('');

const filteredProducts = computed(() => {
  const q = productFilter.value.trim().toLowerCase();
  if (!q) return products.value;
  return products.value.filter(p => (p.name || '').toLowerCase().includes(q));
});

async function openMapping(row: any) {
  mapPrinter.value = row;
  mapVisible.value = true;
  mapLoading.value = true;
  productFilter.value = '';
  try {
    const [prodRes, mapRes] = await Promise.all([
      client.get('/products', { params: { page_size: 500 } }) as Promise<any>,
      client.get(`/printers/${row.id}/products`) as Promise<any>
    ]);
    const list: any[] = Array.isArray(prodRes) ? prodRes : prodRes?.items || [];
    products.value = list.filter(p => p.is_retail);
    selectedProducts.value = mapRes?.product_ids || [];
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
    products.value = [];
    selectedProducts.value = [];
  } finally {
    mapLoading.value = false;
  }
}

async function saveMapping() {
  mapSaving.value = true;
  try {
    await client.put(`/printers/${mapPrinter.value.id}/products`, {
      product_ids: selectedProducts.value
    });
    ElNotification({
      type: 'success',
      title: $t('vnetPages.common.success'),
      message: $t('vnetPages.printers.mappingSaved', {
        count: selectedProducts.value.length
      })
    });
    mapVisible.value = false;
  } catch (e: any) {
    ElMessage.error(e?.message || $t('vnetPages.common.error'));
  } finally {
    mapSaving.value = false;
  }
}

onMounted(getData);
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span style="color: #909399; font-size: 14px">{{ $t('vnetPages.printers.hint') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            :show-delete="false"
            @add="openCreate"
            @refresh="getData"
          />
        </div>
      </template>

      <ElTable v-loading="loading" :data="data" border stripe style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.common.action')" width="330" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="openMapping(row)">{{ $t('vnetPages.printers.mapping') }}</ElButton>
            <ElButton size="small" @click="handleTest(row)">{{ $t('vnetPages.printers.test') }}</ElButton>
            <ElButton size="small" @click="openEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? $t('vnetPages.printers.edit') : $t('vnetPages.printers.add')"
      width="520px"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="160px">
        <ElFormItem :label="$t('vnetPages.printers.name')" prop="name">
          <ElInput v-model="form.name" :placeholder="$t('vnetPages.printers.namePlaceholder')" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.printers.type')">
          <ElSelect v-model="form.printer_type" style="width: 100%">
            <ElOption label="thermal" value="thermal" />
            <ElOption label="label" value="label" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.printers.ip')" prop="ip_address">
          <ElInput v-model="form.ip_address" placeholder="192.168.1.50" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.printers.port')">
          <ElInputNumber v-model="form.port" :min="1" :max="65535" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.printers.paper')">
          <ElRadioGroup v-model="form.chars_per_line">
            <ElRadioButton :value="32">58mm</ElRadioButton>
            <ElRadioButton :value="48">80mm</ElRadioButton>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.printers.encoding')">
          <ElSelect v-model="form.encoding" style="width: 100%">
            <ElOption :label="$t('vnetPages.printers.encodingAscii')" value="ascii" />
            <ElOption :label="$t('vnetPages.printers.encodingVi')" value="cp1258" />
          </ElSelect>
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 4px">
            {{ $t('vnetPages.printers.encodingHint') }}
          </div>
        </ElFormItem>
        <ElFormItem v-if="form.encoding === 'cp1258'" :label="$t('vnetPages.printers.codePage')">
          <ElInputNumber v-model="form.code_page" :min="0" :max="255" />
          <div style="color: #909399; font-size: 12px; line-height: 1.5; margin-top: 4px">
            {{ $t('vnetPages.printers.codePageHint') }}
          </div>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.printers.isDefault')">
          <ElSwitch v-model="form.is_default" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submit">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="mapVisible"
      :title="$t('vnetPages.printers.mappingTitle', { name: mapPrinter?.name })"
      width="560px"
    >
      <div v-loading="mapLoading">
        <div style="color: #909399; font-size: 13px; margin-bottom: 12px">
          {{ $t('vnetPages.printers.mappingHint') }}
        </div>
        <ElInput
          v-model="productFilter"
          :placeholder="$t('vnetPages.printers.searchProduct')"
          clearable
          style="margin-bottom: 12px"
        />
        <ElCheckboxGroup v-model="selectedProducts" style="max-height: 340px; overflow-y: auto; display: block">
          <div v-for="p in filteredProducts" :key="p.id" style="padding: 4px 0">
            <ElCheckbox :value="p.id">{{ p.name }}</ElCheckbox>
          </div>
        </ElCheckboxGroup>
        <div v-if="!mapLoading && !filteredProducts.length" style="color: #909399; text-align: center; padding: 24px 0">
          {{ $t('vnetPages.printers.noProduct') }}
        </div>
      </div>
      <template #footer>
        <span style="float: left; color: #909399; font-size: 13px; line-height: 32px">
          {{
            $t('vnetPages.printers.selectedCount', {
              count: selectedProducts.length
            })
          }}
        </span>
        <ElButton @click="mapVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="mapSaving" @click="saveMapping">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
