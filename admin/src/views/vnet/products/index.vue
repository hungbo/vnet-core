<script setup lang="ts">
import { h, nextTick, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useI18n } from 'vue-i18n';
import client from '@/api/client';
import { useUIPaginatedTable } from '@/hooks/common/table';
import { vnetTransform } from '@/hooks/common/vnet-table';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';
import ImageUpload from '@/components/common/ImageUpload.vue';

const { t: $t } = useI18n();

const saving = ref(false);
const search = ref('');
const filterRetail = ref<boolean | null>(null);
const categories = ref<any[]>([]);
const suppliers = ref<any[]>([]);
const searchResults = ref<any[]>([]);
const searchLoading = ref(false);
const searchPage = ref(1);
const searchTotal = ref(0);
const searchKeyword = ref('');
let searchTimer: ReturnType<typeof setTimeout> | null = null;
let selectScrollEl: HTMLElement | null = null;
const dialogVisible = ref(false);
const isEdit = ref(false);
const formRef = ref<any>(null);
const productIngredients = ref<any[]>([]);
const units = ref<any[]>([]);

const form = ref<any>({
  name: '',
  category_id: null,
  is_retail: true,
  unit_id: 'cai',
  min_stock: 0,
  supplier_id: null,
  price: 0,
  description: '',
  image_url: '',
  has_stock: false,
  current_stock: 0,
  options: []
});

const newIngredient = ref({ ingredient_id: '', quantity: 1 });
const newOption = ref({ ingredient_id: null, quantity: 1 });

const rules = {
  name: [{ required: true, message: $t('vnetPages.products.form.nameRequired'), trigger: 'blur' }]
};

const stockDialog = ref(false);
const isStockIn = ref(true);
const stockSaving = ref(false);
const stockTargetProduct = ref<any>(null);
const stockFormRef = ref<any>(null);
const stockForm = ref({ quantity: 1, note: '', unit_price: 0 });
const stockRules = {
  quantity: [{ required: true, message: $t('vnetPages.products.form.quantityRequired'), trigger: 'blur' }]
};

function formatPrice(price: number) {
  return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(price || 0);
}

function getCategoryName(id: number) {
  const c = categories.value.find(c => c.id === id);
  return c ? c.name : id;
}

function getUnitName(id: string) {
  const u = units.value.find(u => u.id === id);
  return u ? u.name : id;
}

const { columns, columnChecks, data, getData, loading, mobilePagination } = useUIPaginatedTable({
  api: ({ page, pageSize }) =>
    client.get('/products', {
      params: {
        page,
        page_size: pageSize,
        search: search.value || undefined,
        is_retail: filterRetail.value !== null ? filterRetail.value : undefined
      }
    }),
  transform: vnetTransform,
  columns: () => [
    { prop: 'name', label: $t('vnetPages.products.name'), minWidth: 160 },
    {
      prop: 'category_id',
      label: $t('vnetPages.products.category'),
      width: 120,
      formatter: (row: any) => (row.category_id ? getCategoryName(row.category_id) : '-')
    },
    {
      prop: 'price',
      label: $t('vnetPages.products.price'),
      width: 120,
      formatter: (row: any) => formatPrice(row.price)
    },
    {
      prop: 'unit_id',
      label: $t('vnetPages.products.unit'),
      width: 80,
      formatter: (row: any) => (row.unit_id ? getUnitName(row.unit_id) : '-')
    },
    {
      prop: 'supplier_name',
      label: $t('vnetPages.products.supplier'),
      width: 130,
      formatter: (row: any) => row.supplier_name || '-'
    },
    {
      prop: 'is_retail',
      label: $t('vnetPages.products.isRetail'),
      width: 90,
      align: 'center',
      formatter: (row: any) =>
        h(ElTag, { type: row.is_retail ? 'success' : 'info', size: 'small' }, () =>
          row.is_retail ? $t('vnetPages.common.yes') : $t('vnetPages.common.no')
        )
    },
    {
      prop: 'current_stock',
      label: $t('vnetPages.products.currentStock'),
      width: 100,
      align: 'center',
      formatter: (row: any) => (row.has_stock ? String(row.current_stock ?? '') : '-')
    }
  ]
});

async function fetchCategories() {
  try {
    const res: any = await client.get('/categories', { params: { page_size: 1000 } });
    categories.value = Array.isArray(res) ? res : res?.items || [];
  } catch (_) {}
}

async function loadProductResults(keyword: string, append = false) {
  const page = append ? searchPage.value + 1 : 1;
  searchLoading.value = true;
  try {
    const params: any = { page, page_size: 10 };
    if (keyword) params.search = keyword;
    const res: any = await client.get('/products', { params });
    const items = res?.items || [];
    searchTotal.value = res?.total || 0;
    searchResults.value = append ? [...searchResults.value, ...items] : items;
    searchPage.value = page;
  } catch (_) {
    if (!append) searchResults.value = [];
  } finally {
    searchLoading.value = false;
  }
}

function onSelectOpen() {
  if (selectScrollEl) {
    selectScrollEl.removeEventListener('scroll', onSelectScroll);
    selectScrollEl = null;
  }
  if (searchResults.value.length === 0 && !searchLoading.value) {
    loadProductResults(searchKeyword.value, false);
  }
  nextTick(() => {
    const wraps = document.querySelectorAll('.ingredient-popper .el-scrollbar__wrap');
    for (const w of wraps) {
      const popper = w.closest('.ingredient-popper') as HTMLElement;
      if (popper && popper.style.display !== 'none' && w.scrollHeight > 0) {
        selectScrollEl = w as HTMLElement;
        break;
      }
    }
    if (selectScrollEl) {
      selectScrollEl.addEventListener('scroll', onSelectScroll);
    }
  });
}

function onSelectClose() {
  if (selectScrollEl) {
    selectScrollEl.removeEventListener('scroll', onSelectScroll);
    selectScrollEl = null;
  }
  searchTimer = null;
}

function onSelectScroll() {
  if (!selectScrollEl || searchLoading.value) return;
  if (selectScrollEl.scrollTop + selectScrollEl.clientHeight >= selectScrollEl.scrollHeight - 10) {
    if (searchResults.value.length < searchTotal.value) {
      loadProductResults(searchKeyword.value, true);
    }
  }
}

function onSelectSearch(keyword: string) {
  searchKeyword.value = keyword;
  if (searchTimer) clearTimeout(searchTimer);
  if (!keyword) {
    searchResults.value = [];
    searchTotal.value = 0;
    return;
  }
  searchTimer = setTimeout(() => {
    loadProductResults(keyword, false);
  }, 300);
}

async function fetchSuppliers() {
  try {
    const res: any = await client.get('/suppliers', { params: { page_size: 1000 } });
    suppliers.value = Array.isArray(res) ? res : res?.items || [];
  } catch (_) {}
}

async function fetchUnits() {
  try {
    const res: any = await client.get('/units');
    units.value = Array.isArray(res) ? res : [];
  } catch (_) {}
}

onMounted(() => {
  fetchCategories();
  fetchUnits();
  fetchSuppliers();
});

async function fetchProductIngredients(productId: string) {
  try {
    const res: any = await client.get(`/products/${productId}/ingredients`);
    productIngredients.value = Array.isArray(res) ? res : [];
  } catch (_) {
    productIngredients.value = [];
  }
}

function resetForm() {
  form.value = {
    name: '',
    category_id: null,
    is_retail: true,
    unit_id: 'cai',
    min_stock: 0,
    supplier_id: null,
    price: 0,
    description: '',
    image_url: '',
    has_stock: false,
    current_stock: 0,
    options: []
  };
  productIngredients.value = [];
  newIngredient.value = { ingredient_id: '', quantity: 1 };
  newOption.value = { ingredient_id: null, quantity: 1 };
  delete form.value.id;
}

function addOption() {
  if (!newOption.value.ingredient_id) return;
  const product = searchResults.value.find((p: any) => p.id === newOption.value.ingredient_id);
  if (!product) return;
  form.value.options.push({
    ingredient_id: newOption.value.ingredient_id,
    ingredient_name: product.name,
    name: product.name,
    quantity: newOption.value.quantity
  });
  newOption.value = { ingredient_id: null, quantity: 1 };
}

function removeOption(idx: number) {
  form.value.options.splice(idx, 1);
}

function handleCreate() {
  isEdit.value = false;
  resetForm();
  dialogVisible.value = true;
  searchKeyword.value = '';
  loadProductResults('', false);
}

function handleEdit(row: any) {
  isEdit.value = true;
  form.value = {
    id: row.id,
    name: row.name,
    category_id: row.category_id || null,
    is_retail: row.is_retail ?? true,
    unit_id: row.unit_id || 'cai',
    min_stock: row.min_stock ?? 0,
    supplier_id: row.supplier_id || null,
    price: row.price ?? 0,
    description: row.description || '',
    image_url: row.image_url || '',
    has_stock: row.has_stock || false,
    current_stock: row.current_stock ?? 0,
    options: (row.options || []).map((o: any) => ({
      ingredient_id: o.ingredient_id || null,
      ingredient_name: o.ingredient_name || o.name || '',
      name: o.name || o.ingredient_name || '',
      quantity: o.quantity || 1
    }))
  };
  dialogVisible.value = true;
  searchKeyword.value = '';
  loadProductResults('', false);
  if (row.is_retail) {
    fetchProductIngredients(row.id);
  }
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    let product: any;
    if (isEdit.value) {
      product = await client.put(`/products/${form.value.id}`, form.value);
      ElMessage.success($t('vnetPages.products.messages.editSuccess'));
    } else {
      product = await client.post('/products', form.value);
      ElMessage.success($t('vnetPages.products.messages.addSuccess'));
      if (productIngredients.value.length > 0) {
        for (const ing of productIngredients.value) {
          await client.post(`/products/${product.id}/ingredients`, {
            ingredient_id: ing.ingredient_id,
            quantity: ing.quantity
          });
        }
      }
    }
    dialogVisible.value = false;
    await getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.products.messages.saveError'));
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm($t('vnetPages.products.messages.deleteConfirm'), $t('vnetPages.common.confirm'), {
      type: 'warning'
    });
    await client.delete(`/products/${row.id}`);
    ElMessage.success($t('vnetPages.products.messages.deleteSuccess'));
    await getData();
  } catch (_) {}
}

async function addIngredient() {
  if (!newIngredient.value.ingredient_id) return;
  const product = searchResults.value.find((p: any) => p.id === newIngredient.value.ingredient_id);
  if (!product) return;
  if (isEdit.value) {
    saving.value = true;
    try {
      await client.post(`/products/${form.value.id}/ingredients`, { ...newIngredient.value });
      ElMessage.success($t('vnetPages.products.messages.addIngredientSuccess'));
      newIngredient.value = { ingredient_id: '', quantity: 1 };
      await fetchProductIngredients(form.value.id);
    } catch (e: any) {
      ElMessage.error(e.message || $t('vnetPages.products.messages.saveError'));
    } finally {
      saving.value = false;
    }
  } else {
    productIngredients.value.push({
      ingredient_id: newIngredient.value.ingredient_id,
      ingredient_name: product.name,
      quantity: newIngredient.value.quantity,
      unit_name: product.unit_name || ''
    });
    newIngredient.value = { ingredient_id: '', quantity: 1 };
  }
}

async function removeIngredient(index: number) {
  const ing = productIngredients.value[index];
  if (!ing) return;
  if (isEdit.value && ing?.id) {
    try {
      await client.delete(`/products/${form.value.id}/ingredients/${ing.id}`);
      productIngredients.value.splice(index, 1);
      ElMessage.success($t('vnetPages.products.messages.deleteIngredientSuccess'));
    } catch (e: any) {
      ElMessage.error(e.message || $t('vnetPages.products.messages.saveError'));
    }
  } else {
    productIngredients.value.splice(index, 1);
  }
}

function handleStockIn(row: any) {
  stockTargetProduct.value = row;
  isStockIn.value = true;
  stockForm.value = { quantity: 1, note: '', unit_price: 0 };
  stockDialog.value = true;
}

function handleStockOut(row: any) {
  stockTargetProduct.value = row;
  isStockIn.value = false;
  stockForm.value = { quantity: 1, note: '', unit_price: 0 };
  stockDialog.value = true;
}

async function handleStockSave() {
  const valid = await stockFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  stockSaving.value = true;
  try {
    const target = stockTargetProduct.value;
    if (target.has_stock) {
      await client.post('/stock-transactions', {
        product_id: target.id,
        transaction_type: isStockIn.value ? 'inbound' : 'outbound',
        quantity: stockForm.value.quantity,
        unit_price: stockForm.value.unit_price,
        reference_id: target.id,
        description:
          stockForm.value.note ||
          $t(isStockIn.value ? 'vnetPages.products.stockInDescription' : 'vnetPages.products.stockOutDescription')
      });
    } else {
      ElMessage.error($t('vnetPages.products.messages.noIngredients'));
      stockSaving.value = false;
      return;
    }
    ElMessage.success(
      isStockIn.value
        ? $t('vnetPages.products.messages.stockInSuccess')
        : $t('vnetPages.products.messages.stockOutSuccess')
    );
    stockDialog.value = false;
    await getData();
  } catch (e: any) {
    ElMessage.error(e.message || $t('vnetPages.products.messages.saveError'));
  } finally {
    stockSaving.value = false;
  }
}
</script>

<template>
  <div>
    <ElCard>
      <template #header>
        <div class="flex items-center justify-between">
          <span>{{ $t('vnetPages.products.title') }}</span>
          <TableHeaderOperation
            v-model:columns="columnChecks"
            :loading="loading"
            @add="handleCreate"
            @refresh="getData"
          >
            <template #prefix>
              <ElRadioGroup v-model="filterRetail" @change="getData">
                <ElRadio :value="null">{{ $t('vnetPages.common.all') }}</ElRadio>
                <ElRadio :value="true">{{ $t('vnetPages.products.isRetail') }}</ElRadio>
                <ElRadio :value="false">{{ $t('vnetPages.products.nonRetail') }}</ElRadio>
              </ElRadioGroup>
              <ElInput
                v-model="search"
                :placeholder="$t('vnetPages.products.searchPlaceholder')"
                clearable
                style="width: 200px"
                @input="getData"
              />
            </template>
          </TableHeaderOperation>
        </div>
      </template>
      <ElTable v-loading="loading" :data="data" style="width: 100%">
        <ElTableColumn v-for="col in columns" :key="col.prop" v-bind="col" />
        <ElTableColumn :label="$t('vnetPages.products.image')" width="90" align="center">
          <template #default="{ row }">
            <ElImage
              v-if="row.image_url"
              :src="row.image_url"
              style="width: 40px; height: 40px; border-radius: 4px; cursor: pointer"
              fit="cover"
              :preview-src-list="[row.image_url]"
              preview-teleported
            />
            <span v-else>-</span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="$t('vnetPages.common.action')" width="300" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" @click="handleEdit(row)">{{ $t('vnetPages.common.edit') }}</ElButton>
            <ElButton size="small" :disabled="!row.has_stock" @click="handleStockIn(row)">
              {{ $t('vnetPages.products.stockIn') }}
            </ElButton>
            <ElButton size="small" :disabled="!row.has_stock" @click="handleStockOut(row)">
              {{ $t('vnetPages.products.stockOut') }}
            </ElButton>
            <ElButton size="small" type="danger" @click="handleDelete(row)">
              {{ $t('vnetPages.common.delete') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
      <div class="mt-16px flex justify-end">
        <ElPagination
          v-if="mobilePagination.total"
          layout="total, sizes, prev, pager, next"
          v-bind="mobilePagination"
          @current-change="mobilePagination['current-change']"
          @size-change="mobilePagination['size-change']"
        />
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="isEdit ? $t('vnetPages.products.edit') : $t('vnetPages.products.add')"
      width="600px"
      :close-on-click-modal="false"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" :label-width="110">
        <ElFormItem :label="$t('vnetPages.products.name')" prop="name">
          <ElInput v-model="form.name" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.isRetail')" prop="is_retail">
          <ElSwitch v-model="form.is_retail" />
          <span style="margin-left: 8px; color: #666">
            {{ form.is_retail ? $t('vnetPages.products.isRetail') : $t('vnetPages.products.nonRetail') }}
          </span>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.unit')" prop="unit_id">
          <ElSelect v-model="form.unit_id" :placeholder="$t('vnetPages.products.unit')" style="width: 100%">
            <ElOption v-for="u in units" :key="u.id" :label="u.name" :value="u.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.category')" prop="category_id">
          <ElSelect
            v-model="form.category_id"
            :placeholder="$t('vnetPages.products.form.categoryPlaceholder')"
            clearable
            style="width: 100%"
          >
            <ElOption v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.price')" prop="price">
          <ElInputNumber v-model="form.price" :min="0" :precision="0" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.description')" prop="description">
          <ElInput v-model="form.description" type="textarea" :rows="2" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.image')" prop="image_url">
          <ImageUpload v-model="form.image_url" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.supplier')" prop="supplier_id">
          <ElSelect
            v-model="form.supplier_id"
            :placeholder="$t('vnetPages.products.supplier')"
            clearable
            style="width: 100%"
          >
            <ElOption v-for="s in suppliers" :key="s.id" :label="s.name" :value="s.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.hasStock')" prop="has_stock">
          <ElSwitch
            v-model="form.has_stock"
            @change="
              v => {
                if (!v) form.min_stock = 0;
              }
            "
          />
          <span v-if="form.has_stock" style="margin-left: 8px; color: #666">
            {{ $t('vnetPages.products.currentStock') }}: {{ form.current_stock ?? 0 }}
          </span>
        </ElFormItem>
        <template v-if="form.has_stock">
          <ElFormItem :label="$t('vnetPages.products.minStock')" prop="min_stock">
            <ElInputNumber v-model="form.min_stock" :min="0" style="width: 100%" />
          </ElFormItem>
        </template>
        <template v-if="form.is_retail">
          <ElFormItem :label="$t('vnetPages.products.ingredients')">
            <div style="width: 100%">
              <ElTable :data="productIngredients" size="small" max-height="200" style="margin-bottom: 8px">
                <ElTableColumn prop="ingredient_name" :label="$t('vnetPages.products.ingredients')" />
                <ElTableColumn prop="quantity" :label="$t('vnetPages.products.stockQuantity')" width="100" />
                <ElTableColumn prop="unit_name" :label="$t('vnetPages.products.unit')" width="80" />
                <ElTableColumn width="80">
                  <template #default="{ row, $index }">
                    <ElButton size="small" type="danger" link @click="removeIngredient($index)">
                      {{ $t('vnetPages.common.delete') }}
                    </ElButton>
                  </template>
                </ElTableColumn>
              </ElTable>
              <div style="display: flex; gap: 8px">
                <ElSelect
                  v-model="newIngredient.ingredient_id"
                  :placeholder="$t('vnetPages.products.addIngredient')"
                  filterable
                  remote
                  :remote-method="onSelectSearch"
                  :loading="searchLoading"
                  popper-class="ingredient-popper"
                  clearable
                  style="width: 200px"
                  @visible-change="v => (v ? onSelectOpen() : onSelectClose())"
                >
                  <ElOption v-for="p in searchResults" :key="p.id" :label="p.name" :value="p.id" />
                </ElSelect>
                <ElInputNumber v-model="newIngredient.quantity" :min="0" :precision="0" style="width: 120px" />
                <ElButton type="primary" @click="addIngredient">{{ $t('vnetPages.common.add') }}</ElButton>
              </div>
            </div>
          </ElFormItem>
        </template>
        <template v-if="form.is_retail">
          <ElDivider />
          <ElFormItem :label="$t('vnetPages.products.optionGroups')">
            <div style="width: 100%">
              <ElTable :data="form.options" size="small" max-height="200" style="margin-bottom: 8px">
                <ElTableColumn prop="name" :label="$t('vnetPages.products.optionItem')" />
                <ElTableColumn prop="quantity" :label="$t('vnetPages.products.stockQuantity')" width="100" />
                <ElTableColumn width="80">
                  <template #default="{ row, $index }">
                    <ElButton size="small" type="danger" link @click="removeOption($index)">
                      {{ $t('vnetPages.common.delete') }}
                    </ElButton>
                  </template>
                </ElTableColumn>
              </ElTable>
              <div style="display: flex; gap: 8px">
                <ElSelect
                  v-model="newOption.ingredient_id"
                  :placeholder="$t('vnetPages.products.addOption')"
                  filterable
                  remote
                  :remote-method="onSelectSearch"
                  :loading="searchLoading"
                  popper-class="ingredient-popper"
                  clearable
                  style="width: 200px"
                  @visible-change="v => (v ? onSelectOpen() : onSelectClose())"
                >
                  <ElOption v-for="p in searchResults" :key="p.id" :label="p.name" :value="p.id" />
                </ElSelect>
                <ElInputNumber v-model="newOption.quantity" :min="0" :precision="0" style="width: 120px" />
                <ElButton type="primary" @click="addOption">{{ $t('vnetPages.common.add') }}</ElButton>
              </div>
            </div>
          </ElFormItem>
        </template>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="saving" @click="handleSave">{{ $t('vnetPages.common.save') }}</ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="stockDialog"
      :title="isStockIn ? $t('vnetPages.products.stockIn') : $t('vnetPages.products.stockOut')"
      width="450px"
    >
      <ElForm ref="stockFormRef" :model="stockForm" :rules="stockRules" :label-width="100">
        <ElFormItem :label="$t('vnetPages.products.stockQuantity')" prop="quantity">
          <ElInputNumber v-model="stockForm.quantity" :min="1" style="width: 100%" />
        </ElFormItem>
        <ElFormItem :label="$t('vnetPages.products.stockNote')" prop="note">
          <ElInput v-model="stockForm.note" type="textarea" :rows="2" />
        </ElFormItem>
        <ElFormItem v-if="isStockIn" :label="$t('vnetPages.products.stockUnitPrice')" prop="unit_price">
          <ElInputNumber v-model="stockForm.unit_price" :min="0" :precision="0" style="width: 100%" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="stockDialog = false">{{ $t('vnetPages.common.cancel') }}</ElButton>
        <ElButton type="primary" :loading="stockSaving" @click="handleStockSave">
          {{ $t('vnetPages.common.confirm') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>
