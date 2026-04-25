<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>{{ form.edit_id ? '编辑订单' : '录单' }}</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">订单已保存：{{ ok }}</div>

      <div class="form-grid">
        <label>
          <span>订单日期</span>
          <input v-model.trim="form.order_date" type="date" />
        </label>
        <label>
          <span>客户</span>
          <select v-model.number="form.customer_id">
            <option :value="0">选择客户</option>
            <option v-for="item in customers" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>来源</span>
          <select v-model.number="form.source_id">
            <option :value="0">选择来源</option>
            <option v-for="item in sources" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>订单类型</span>
          <select v-model.number="form.order_type_id" @change="syncRowsForType">
            <option :value="0">选择类型</option>
            <option v-for="item in orderTypes" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>付款状态</span>
          <select v-model.number="form.pay_status_id">
            <option :value="0">默认</option>
            <option v-for="item in payStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>发货状态</span>
          <select v-model.number="form.ship_status_id">
            <option :value="0">默认</option>
            <option v-for="item in shipStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
      </div>

      <label class="notes">
        <span>备注</span>
        <textarea v-model.trim="form.notes" rows="2"></textarea>
      </label>
    </section>

    <section class="panel">
      <div class="section-title">明细</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>规格(g)</th>
              <th>数量(件)</th>
              <th>单价</th>
              <th>小计</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, idx) in rows" :key="row.key">
              <td>
                <input v-model.trim="row.product_query" class="product-filter" placeholder="搜索商品" />
                <select v-model.number="row.product_id" @change="selectProduct(row)">
                  <option :value="0">选择商品</option>
                  <option v-for="product in productOptions(row)" :key="product.id" :value="product.id">
                    {{ product.name }}
                  </option>
                </select>
              </td>
              <td>
                <div class="spec-control">
                  <select v-model="row.spec_mode" @change="syncPrice(row)">
                    <option value="">选择规格</option>
                    <option v-for="option in specOptions(row)" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>
                  <input
                    v-if="row.spec_mode === CUSTOM_SPEC_VALUE"
                    v-model.number="row.custom_spec_g"
                    type="number"
                    min="1"
                    step="1"
                    placeholder="自定义克数"
                    @input="syncPrice(row)"
                  />
                </div>
              </td>
              <td><input v-model.number="row.qty" type="number" min="1" step="1" @input="syncPrice(row)" /></td>
              <td>{{ money(unitPackagePrice(row)) }}/件</td>
              <td><strong>{{ money(rowTotal(row)) }}</strong></td>
              <td><button class="secondary" type="button" @click="removeRow(idx)" :disabled="rows.length === 1">删除</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="actions">
        <button class="secondary" type="button" @click="addRow">新增明细</button>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">费用</div>
      <div class="form-grid compact">
        <label>
          <span>运费</span>
          <input v-model.trim="form.shipping_amount" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>优惠</span>
          <input v-model.trim="form.discount_amount" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>快递费备注</span>
          <input v-model.trim="form.express_fee" />
        </label>
        <label class="checkline">
          <input v-model="form.round_to_int" type="checkbox" />
          <span>合计取整</span>
        </label>
      </div>
      <div class="total-line">商品合计：{{ money(itemsTotal) }}</div>
      <button class="primary" type="button" @click="save" :disabled="saving">保存订单</button>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  CUSTOM_SPEC_VALUE,
  buildOrderPayload,
  lineTotal,
  normalizeSpecG,
  retailPackagePrice,
  retailSpecOptions,
  toInt,
  toNumber,
} from '../lib/order-entry'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const customers = ref([])
const sources = ref([])
const shipStatuses = ref([])
const payStatuses = ref([])
const orderTypes = ref([])
const products = ref([])
const rows = ref([newRow()])

const form = reactive({
  edit_id: 0,
  order_date: '',
  customer_id: 0,
  source_id: 0,
  order_type_id: 0,
  pay_status_id: 0,
  ship_status_id: 0,
  ship_method: '',
  ship_tracking_no: '',
  notes: '',
  shipping_amount: '',
  discount_amount: '',
  round_to_int: false,
  express_fee: '',
  outsource_material_fee: '',
  outsource_roast_fee: '',
  outsource_packaging_fee: '',
  outsource_manual_fee: '',
  outsource_tax_fee: '',
  outsource_other_fee: '',
})

function newRow() {
  return {
    key: `${Date.now()}-${Math.random()}`,
    product_query: '',
    product_id: 0,
    product_name: '',
    tier_id: 'auto',
    unit_price: '',
    spec_mode: '',
    custom_spec_g: '',
    qty: 1,
    unit: '件',
  }
}

function selectedOrderType() {
  return orderTypes.value.find((item) => Number(item.id) === Number(form.order_type_id)) || null
}

const retailOrder = computed(() => {
  const name = selectedOrderType()?.name || ''
  return name.includes('零售') || name.toLowerCase().includes('retail')
})

const itemsTotal = computed(() => rows.value.reduce((sum, row) => sum + rowTotal(row), 0))

function productByID(id) {
  return products.value.find((item) => Number(item.id) === Number(id)) || null
}

function productOptions(row) {
  const q = String(row.product_query || '').trim().toLowerCase()
  if (!q) return products.value
  return products.value.filter((product) => {
    const haystack = `${product.name || ''} ${product.py || ''} ${product.pyi || ''}`.toLowerCase()
    return haystack.includes(q)
  })
}

function selectProduct(row) {
  const product = productByID(row.product_id)
  row.product_name = product?.name || ''
  row.product_query = ''
  if (retailOrder.value) {
    const options = retailSpecOptions(product, true)
    row.spec_mode = options[0]?.value || CUSTOM_SPEC_VALUE
  } else {
    const options = specOptions(row)
    row.spec_mode = options[0]?.value || '454'
  }
  syncPrice(row)
}

function specOptions(row) {
  const product = productByID(row.product_id)
  if (retailOrder.value) return retailSpecOptions(product, true)
  const specs = new Set([227, 454])
  for (const tier of product?.tiers || []) {
    if (toInt(tier.spec_g) > 0) specs.add(toInt(tier.spec_g))
  }
  return [...specs].sort((a, b) => a - b).map((spec) => ({ label: `${spec}g`, value: String(spec) }))
}

function syncRowsForType() {
  rows.value.forEach((row) => {
    if (!row.product_id) return
    const options = specOptions(row)
    if (!options.some((option) => option.value === row.spec_mode)) {
      row.spec_mode = options[0]?.value || ''
      row.custom_spec_g = ''
    }
    syncPrice(row)
  })
}

function syncPrice(row) {
  const product = productByID(row.product_id)
  if (!product) {
    row.unit_price = ''
    return
  }
  if (retailOrder.value) {
    row.unit_price = String(retailPackagePrice(product, normalizeSpecG(row)) || '')
    return
  }
  const specG = normalizeSpecG(row)
  const qty = Math.max(1, toInt(row.qty))
  const tier = (product.tiers || [])
    .filter((item) => toInt(item.spec_g) === specG && toNumber(item.min) <= qty && (!item.max || toNumber(item.max) >= qty))
    .sort((a, b) => toNumber(b.min) - toNumber(a.min))[0]
  if (tier) {
    row.tier_id = String(tier.id)
    row.unit_price = String(tier.price || 0)
  } else {
    row.tier_id = 'auto'
    row.unit_price = ''
  }
}

function unitPackagePrice(row) {
  const product = productByID(row.product_id)
  if (!product) return 0
  if (retailOrder.value) return retailPackagePrice(product, normalizeSpecG(row))
  return toNumber(row.unit_price)
}

function rowTotal(row) {
  return lineTotal(productByID(row.product_id), row, retailOrder.value)
}

function money(value) {
  return Number(value || 0).toFixed(2)
}

function addRow() {
  rows.value.push(newRow())
}

function removeRow(idx) {
  if (rows.value.length <= 1) return
  rows.value.splice(idx, 1)
}

function applyEditData(data) {
  if (!data) return
  Object.assign(form, {
    edit_id: Number(data.edit_id || form.edit_id || 0),
    order_date: data.order_date || form.order_date,
    customer_id: Number(data.customer_id || 0),
    source_id: Number(data.source_id || 0),
    order_type_id: Number(data.order_type_id || 0),
    pay_status_id: Number(data.pay_status_id || 0),
    ship_status_id: Number(data.ship_status_id || 0),
    ship_method: data.ship_method || '',
    ship_tracking_no: data.ship_tracking_no || '',
    notes: data.notes || '',
    shipping_amount: data.shipping_amount || '',
    discount_amount: data.discount_amount || '',
    round_to_int: !!data.round_to_int,
    express_fee: data.express_fee || '',
    outsource_material_fee: data.outsource_material_fee || '',
    outsource_roast_fee: data.outsource_roast_fee || '',
    outsource_packaging_fee: data.outsource_packaging_fee || '',
    outsource_manual_fee: data.outsource_manual_fee || '',
    outsource_tax_fee: data.outsource_tax_fee || '',
    outsource_other_fee: data.outsource_other_fee || '',
  })
  rows.value = (data.items || []).map((item) => {
    const spec = String(item.spec || '').replace(/g$/i, '')
    const product = productByID(item.product_id)
    const retailSpecs = (product?.retail_specs || []).map(toInt)
    const shouldUseCustomSpec = retailOrder.value && !retailSpecs.includes(toInt(spec))
    return {
      ...newRow(),
      product_id: Number(item.product_id || 0),
      product_name: item.product_name || '',
      tier_id: item.tier_id || 'auto',
      unit_price: item.unit_price || '',
      spec_mode: shouldUseCustomSpec ? CUSTOM_SPEC_VALUE : spec,
      custom_spec_g: shouldUseCustomSpec ? spec : '',
      qty: Number(item.qty || 1),
      unit: item.unit || '件',
    }
  })
  if (!rows.value.length) rows.value = [newRow()]
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = new URL('/api/order/form', window.location.origin)
    const editID = new URL(window.location.href).searchParams.get('edit_id')
    if (editID) url.searchParams.set('edit_id', editID)
    const res = await fetch(url)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    customers.value = data.customers || []
    sources.value = data.sources || []
    shipStatuses.value = data.ship_statuses || []
    payStatuses.value = data.pay_statuses || []
    orderTypes.value = data.order_types || []
    products.value = data.products || []
    form.order_date = data.today || form.order_date
    if (!form.order_type_id && orderTypes.value.length) form.order_type_id = Number(orderTypes.value[0].id)
    if (!form.source_id && sources.value.length) form.source_id = Number(sources.value[0].id)
    if (data.edit_mode) {
      form.edit_id = Number(data.edit_id || 0)
      applyEditData({ ...data.edit_data, edit_id: data.edit_id })
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    const payload = buildOrderPayload({ form, rows: rows.value })
    if (!payload.customer_id) throw new Error('请选择客户')
    if (!payload.product_id.length) throw new Error('请至少录入一条有效明细')
    const res = await fetch('/api/order', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    ok.value = data.order_no || '成功'
    if (data.redirect_url) window.location.href = data.redirect_url
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page { padding: 16px; display: grid; gap: 16px; }
.panel { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fff; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)); gap: 12px; }
.form-grid.compact { grid-template-columns: repeat(4, minmax(140px, 1fr)); }
label { display: flex; flex-direction: column; gap: 6px; }
label span, .muted { color: #666; font-size: 12px; }
input, select, textarea, button { font: inherit; }
input, select, textarea { width: 100%; border: 1px solid #ddd; border-radius: 6px; padding: 8px; min-height: 36px; background: #fff; }
textarea { resize: vertical; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.notes { margin-top: 12px; }
.table-wrap { overflow: auto; margin-top: 10px; }
table { width: 100%; border-collapse: collapse; min-width: 980px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 8px; text-align: left; vertical-align: middle; }
th { font-weight: 700; }
.product-filter { margin-bottom: 6px; }
.spec-control { display: grid; grid-template-columns: minmax(150px, 1fr) minmax(140px, 1fr); gap: 8px; }
.actions { display: flex; gap: 10px; margin-top: 12px; }
.checkline { flex-direction: row; align-items: center; gap: 8px; padding-top: 24px; }
.checkline input { width: auto; min-height: auto; }
.total-line { margin: 12px 0; font-weight: 700; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid, .form-grid.compact { grid-template-columns: 1fr; }
  .spec-control { grid-template-columns: 1fr; }
}
</style>
