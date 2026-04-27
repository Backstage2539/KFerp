<template>
  <div class="page">
    <section class="order-hero">
      <div>
        <p class="eyebrow">订单销售</p>
        <h2>{{ form.edit_id ? '编辑订单' : '录单' }}</h2>
      </div>
      <div class="hero-actions">
        <div class="total-pill">
          <span>商品合计</span>
          <strong>{{ money(itemsTotal) }}</strong>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="error" class="notice error">{{ error }}</div>
    <div v-if="ok" class="notice ok">订单已保存：{{ ok }}</div>

    <section class="panel order-fields">
      <div class="section-title">订单信息</div>
      <div class="form-grid">
        <label>
          <span>订单日期</span>
          <input v-model.trim="form.order_date" type="date" />
        </label>

        <label class="customer-combobox combobox">
          <span>客户</span>
          <input
            v-model.trim="customerQuery"
            type="search"
            placeholder="输入客户名/拼音"
            autocomplete="off"
            @focus="customerOpen = true"
            @input="form.customer_id = 0; customerOpen = true"
            @keydown.down.prevent="customerOpen = true"
          />
          <div v-if="customerOpen" class="combo-menu">
            <button
              v-for="item in filteredCustomers"
              :key="item.id"
              type="button"
              class="combo-option"
              @mousedown.prevent="chooseCustomer(item)"
            >
              <strong>{{ item.name }}</strong>
              <small v-if="item.default_source_id || item.default_order_type_id">
                {{ optionName(sources, item.default_source_id) || '默认来源' }} / {{ optionName(orderTypes, item.default_order_type_id) || '默认类型' }}
              </small>
            </button>
            <div v-if="!filteredCustomers.length" class="combo-empty">没有匹配客户</div>
          </div>
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
            <option v-for="item in payStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>

        <label>
          <span>发货状态</span>
          <select v-model.number="form.ship_status_id">
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
      <div class="section-row">
        <div class="section-title">商品明细</div>
        <button class="secondary" type="button" @click="addRow">新增明细</button>
      </div>
      <div class="line-list">
        <article v-for="(row, idx) in rows" :key="row.key" class="line-item">
          <label class="product-combobox combobox product-cell">
            <span>商品</span>
            <input
              v-model.trim="row.product_query"
              type="search"
              placeholder="选择商品"
              autocomplete="off"
              @focus="row.product_open = true"
              @input="clearProduct(row)"
              @keydown.down.prevent="row.product_open = true"
            />
            <div v-if="row.product_open" class="combo-menu">
              <button
                v-for="product in productOptions(row)"
                :key="product.id"
                type="button"
                class="combo-option"
                @mousedown.prevent="chooseProduct(row, product)"
              >
                <strong>{{ product.name }}</strong>
                <small v-if="product.tiers?.length">{{ product.tiers.length }} 个价格梯度</small>
              </button>
              <div v-if="!productOptions(row).length" class="combo-empty">没有匹配商品</div>
            </div>
          </label>

          <label>
            <span>规格</span>
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
                placeholder="克数"
                @input="syncPrice(row)"
              />
            </div>
          </label>

          <label>
            <span>数量</span>
            <input v-model.number="row.qty" type="number" min="1" step="1" @input="syncPrice(row)" />
          </label>

          <label>
            <span>单价</span>
            <div class="price-control">
              <input
                v-model.trim="row.unit_price"
                type="number"
                min="0"
                step="0.01"
                placeholder="0.00"
                @input="markManualPrice(row)"
              />
              <button class="icon-button" type="button" title="恢复自动价格" @click="resetAutoPrice(row)">↺</button>
            </div>
          </label>

          <div class="line-total">
            <span>小计</span>
            <strong>{{ money(rowTotal(row)) }}</strong>
            <small>{{ row.manual_price ? '手动价' : autoPriceLabel(row) }}</small>
          </div>

          <button class="secondary danger" type="button" @click="removeRow(idx)" :disabled="rows.length === 1">删除</button>

          <div v-if="tierRows(row).length" class="tier-prices">
            <button
              v-for="tier in tierRows(row)"
              :key="tier.id || `${tier.specG}-${tier.rangeLabel}`"
              type="button"
              class="tier-price-chip"
              :class="{ active: String(row.spec_mode) === String(tier.specG) && String(row.tier_id) === tier.id }"
              @click="selectTier(row, tier)"
            >
              <span>{{ tier.specLabel }} {{ tier.rangeLabel }}</span>
              <strong>{{ money(tier.unitPrice) }}/件</strong>
            </button>
          </div>
        </article>
      </div>
    </section>

    <section class="panel footer-panel">
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
      <div class="save-row">
        <div class="grand-line">
          <span>商品合计</span>
          <strong>{{ money(itemsTotal) }}</strong>
        </div>
        <button class="primary" type="button" @click="save" :disabled="saving">保存订单</button>
      </div>
      <details class="manual">
        <summary>录单手册</summary>
        <ul>
          <li>客户和商品输入框支持名称、拼音和首字母搜索。</li>
          <li>选择客户后会带入客户档案中的默认来源和订单类型。</li>
          <li>常用规格：36g、80g、100g、227g、454g、500g、1000g、2.5kg。</li>
          <li>新订单默认已付款、未发货；商品单价会随规格和数量匹配价格梯度。</li>
          <li>需要临时改价时直接修改单价，点击 ↺ 恢复自动梯度价。</li>
        </ul>
      </details>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  CUSTOM_SPEC_VALUE,
  buildOrderPayload,
  defaultWholesaleSpec,
  defaultStatusID,
  filterOptions,
  lineTotal,
  normalizeSpecG,
  retailPackagePrice,
  retailSpecOptions,
  syncWholesaleTierPrice,
  toInt,
  toNumber,
  wholesaleTierPriceRows,
  wholesaleSpecOptions,
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
const customerQuery = ref('')
const customerOpen = ref(false)

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
    product_open: false,
    product_id: 0,
    product_name: '',
    tier_id: 'auto',
    unit_price: '',
    manual_price: false,
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
const filteredCustomers = computed(() => filterOptions(customers.value, customerQuery.value).slice(0, 20))

function productByID(id) {
  return products.value.find((item) => Number(item.id) === Number(id)) || null
}

function optionName(options, id) {
  return (options || []).find((item) => Number(item.id) === Number(id))?.name || ''
}

function chooseCustomer(item) {
  form.customer_id = Number(item.id || 0)
  customerQuery.value = item.name || ''
  customerOpen.value = false
  if (Number(item.default_source_id || 0) > 0) form.source_id = Number(item.default_source_id)
  if (Number(item.default_order_type_id || 0) > 0) {
    form.order_type_id = Number(item.default_order_type_id)
    syncRowsForType()
  }
}

function productOptions(row) {
  return filterOptions(products.value, row.product_query).slice(0, 30)
}

function clearProduct(row) {
  row.product_open = true
  row.product_id = 0
  row.product_name = ''
  row.tier_id = 'auto'
  row.unit_price = ''
  row.manual_price = false
}

function chooseProduct(row, product) {
  row.product_id = Number(product?.id || 0)
  row.product_name = product?.name || ''
  row.product_query = product?.name || ''
  row.product_open = false
  row.manual_price = false
  if (retailOrder.value) {
    const options = retailSpecOptions(product, true)
    row.spec_mode = options[0]?.value || CUSTOM_SPEC_VALUE
  } else {
    row.spec_mode = defaultWholesaleSpec(product)
  }
  syncPrice(row, { force: true })
}

function specOptions(row) {
  const product = productByID(row.product_id)
  if (retailOrder.value) return retailSpecOptions(product, true)
  return wholesaleSpecOptions(product)
}

function syncRowsForType() {
  rows.value.forEach((row) => {
    if (!row.product_id) return
    row.manual_price = false
    const options = specOptions(row)
    if (!options.some((option) => option.value === row.spec_mode)) {
      row.spec_mode = retailOrder.value ? (options[0]?.value || '') : defaultWholesaleSpec(productByID(row.product_id))
      row.custom_spec_g = ''
    }
    syncPrice(row, { force: true })
  })
}

function syncPrice(row, options = {}) {
  const product = productByID(row.product_id)
  if (!product) {
    row.unit_price = ''
    return
  }
  if (row.manual_price && !options.force) return
  if (retailOrder.value) {
    row.tier_id = 'auto'
    row.unit_price = String(retailPackagePrice(product, normalizeSpecG(row)) || '')
    row.manual_price = false
    return
  }
  const price = syncWholesaleTierPrice(product, row)
  row.tier_id = price.tierID
  row.unit_price = price.unitPrice
  row.manual_price = false
}

function markManualPrice(row) {
  row.manual_price = true
  row.tier_id = 'manual'
}

function resetAutoPrice(row) {
  row.manual_price = false
  syncPrice(row, { force: true })
}

function tierRows(row) {
  if (retailOrder.value) return []
  return wholesaleTierPriceRows(productByID(row.product_id))
}

function selectTier(row, tier) {
  row.spec_mode = String(tier.specG || '')
  row.custom_spec_g = ''
  row.manual_price = false
  syncPrice(row, { force: true })
}

function autoPriceLabel(row) {
  if (retailOrder.value) return '零售价'
  if (row.tier_id && row.tier_id !== 'auto' && row.tier_id !== 'manual') return `梯度 ${row.tier_id}`
  return '自动价'
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

function applyDefaultSelections(data) {
  if (!form.order_type_id && orderTypes.value.length) form.order_type_id = Number(orderTypes.value[0].id)
  if (!form.source_id && sources.value.length) form.source_id = Number(sources.value[0].id)
  if (!form.pay_status_id) {
    form.pay_status_id = defaultStatusID(payStatuses.value, ['已付款', '已收款']) || Number(payStatuses.value[0]?.id || 0)
  }
  if (!form.ship_status_id) {
    form.ship_status_id = defaultStatusID(shipStatuses.value, ['未发货']) || Number(shipStatuses.value[0]?.id || 0)
  }
  form.order_date = data.today || form.order_date
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
  customerQuery.value = optionName(customers.value, form.customer_id)
  rows.value = (data.items || []).map((item) => {
    const spec = String(item.spec || '').replace(/g$/i, '')
    const product = productByID(item.product_id)
    const retailSpecs = (product?.retail_specs || []).map(toInt)
    const shouldUseCustomSpec = retailOrder.value && !retailSpecs.includes(toInt(spec))
    return {
      ...newRow(),
      product_id: Number(item.product_id || 0),
      product_name: item.product_name || '',
      product_query: item.product_name || '',
      tier_id: item.tier_id || 'auto',
      unit_price: item.unit_price || '',
      manual_price: item.tier_id === 'manual',
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
    applyDefaultSelections(data)
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
.page { min-height: 100%; padding: 18px; display: grid; gap: 14px; background: #f6f7f9; color: #15171a; }
.order-hero, .panel { background: #fff; border: 1px solid #e7e9ee; border-radius: 8px; box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04); }
.order-hero { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 20px; }
.eyebrow { margin: 0 0 4px; color: #6b7280; font-size: 12px; }
.order-hero h2, .section-title { margin: 0; font-size: 20px; font-weight: 700; letter-spacing: 0; }
.section-title { font-size: 17px; }
.hero-actions, .section-row, .save-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.total-pill { display: grid; gap: 2px; min-width: 132px; padding: 8px 12px; border: 1px solid #e5e7eb; border-radius: 8px; background: #fafafa; }
.total-pill span, .grand-line span, label span, .line-total span, .combo-option small { color: #667085; font-size: 12px; }
.total-pill strong, .grand-line strong { font-size: 20px; }
.panel { padding: 16px; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(190px, 1fr)); gap: 14px; }
.form-grid.compact { grid-template-columns: repeat(4, minmax(150px, 1fr)); }
label { position: relative; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
input, select, textarea, button { font: inherit; }
input, select, textarea { width: 100%; border: 1px solid #d7dbe3; border-radius: 6px; padding: 8px 10px; min-height: 38px; background: #fff; box-sizing: border-box; }
input:focus, select:focus, textarea:focus { outline: 2px solid #9cc2ff; border-color: #4f8df7; }
textarea { resize: vertical; }
button { border-radius: 7px; padding: 8px 12px; cursor: pointer; white-space: nowrap; }
button:disabled { cursor: not-allowed; opacity: 0.5; }
.primary { border: 1px solid #111827; background: #111827; color: #fff; }
.secondary { border: 1px solid #c9ced8; background: #fff; color: #111827; }
.danger { color: #9f1239; }
.notes { margin-top: 14px; }
.notice { border-radius: 8px; padding: 10px 12px; }
.notice.ok { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.notice.ok a { color: #0f3d99; font-weight: 700; text-decoration: none; }
.error { background: #fff1f2; border: 1px solid #fecdd3; color: #9f1239; }
.ok { background: #ecfdf3; border: 1px solid #bbf7d0; color: #166534; }
.warn { background: #fffbeb; border: 1px solid #fde68a; color: #92400e; }
.combobox { z-index: 2; }
.combo-menu { position: absolute; top: calc(100% + 4px); left: 0; right: 0; z-index: 20; max-height: 280px; overflow: auto; border: 1px solid #d7dbe3; border-radius: 8px; background: #fff; box-shadow: 0 14px 30px rgba(15, 23, 42, 0.16); padding: 6px; }
.combo-option { width: 100%; display: grid; gap: 2px; text-align: left; border: 0; background: transparent; padding: 8px; border-radius: 6px; }
.combo-option:hover { background: #f3f6fb; }
.combo-empty { padding: 12px; color: #667085; font-size: 13px; }
.line-list { display: grid; gap: 10px; margin-top: 12px; }
.line-item { display: grid; grid-template-columns: minmax(260px, 1.5fr) minmax(180px, 0.9fr) minmax(110px, 0.55fr) minmax(150px, 0.75fr) minmax(110px, 0.6fr) auto; align-items: end; gap: 12px; padding: 12px; border: 1px solid #edf0f5; border-radius: 8px; background: #fcfcfd; }
.product-cell { z-index: 3; }
.spec-control, .price-control { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: center; }
.spec-control input { min-width: 86px; }
.icon-button { width: 38px; height: 38px; padding: 0; display: inline-grid; place-items: center; border: 1px solid #c9ced8; background: #fff; }
.line-total { display: grid; gap: 3px; padding-bottom: 2px; }
.line-total strong { font-size: 18px; }
.line-total small { color: #667085; font-size: 12px; }
.tier-prices { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 8px; padding-top: 2px; }
.tier-price-chip { display: grid; grid-template-columns: auto auto; align-items: center; gap: 8px; min-height: 32px; border: 1px solid #d7dbe3; background: #fff; color: #344054; border-radius: 7px; padding: 6px 8px; font-size: 12px; }
.tier-price-chip strong { color: #111827; }
.tier-price-chip.active { border-color: #4f8df7; background: #eef5ff; color: #174ea6; }
.checkline { flex-direction: row; align-items: center; gap: 8px; padding-top: 25px; }
.checkline input { width: auto; min-height: auto; }
.footer-panel { display: grid; gap: 12px; }
.grand-line { display: grid; gap: 2px; }
.manual { border-top: 1px solid #edf0f5; padding-top: 10px; color: #4b5563; font-size: 13px; }
.manual summary { cursor: pointer; font-weight: 700; color: #111827; }
.manual ul { margin: 8px 0 0; padding-left: 18px; }

@media (max-width: 1100px) {
  .line-item { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 760px) {
  .page { padding: 12px; }
  .order-hero, .section-row, .save-row { align-items: stretch; flex-direction: column; }
  .form-grid, .form-grid.compact, .line-item { grid-template-columns: 1fr; }
  .hero-actions { width: 100%; }
}
</style>
