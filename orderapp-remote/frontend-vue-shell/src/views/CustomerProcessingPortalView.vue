<template>
  <div class="page customer-processing-portal">
    <header class="portal-head">
      <select
        v-if="!customerAccountActor && !customerContextId && !viewParams.customer_id"
        v-model.number="internalCustomerID"
        @change="load"
      >
        <option :value="0">选择履约客户</option>
        <option v-for="row in internalCustomers" :key="row.id" :value="row.id">{{ row.name }}</option>
      </select>
      <div>
        <h2>{{ title }}</h2>
        <p>{{ data.customer_name || customerContextLabel }}</p>
      </div>
      <button type="button" @click="load" :disabled="loading">刷新</button>
    </header>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p v-if="message" class="notice ok">{{ message }}</p>
    <p v-if="loading">加载中…</p>
    <template v-if="!loading && data.customer_id">
      <template v-if="portalPage === 'customerProcessingPortal'">
        <section class="metrics">
          <article v-for="metric in metrics" :key="metric.label">
            <span>{{ metric.label }}</span
            ><strong>{{ metric.value }}</strong>
          </article>
        </section>
        <section class="panel">
          <h3>常用入口</h3>
          <div class="links">
            <button v-for="item in menuItems" :key="item.key" type="button" @click="navigate(item.key)">
              {{ item.label }}
            </button>
          </div>
        </section>
      </template>
      <OrderEntryView
        v-else-if="['customerDirectShip', 'customerProductOrder'].includes(portalPage)"
        :key="portalPage"
        :customer-portal="true"
        :fulfillment-mode="true"
        :portal-service="capability"
        :customer-context-id="data.customer_id"
        :customer-context-label="data.customer_name"
        workspace-mode="customer"
        @saved="navigate('customerOrders')"
      />
      <section v-else-if="portalPage === 'customerPriceTables'" class="panel">
        <p>选择价格表后点击预览查看商品、规格和阶梯价。</p>
        <table>
          <thead>
            <tr>
              <th>价格表</th>
              <th>版本</th>
              <th>发布日期</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="list in data.price_lists || []" :key="list.id"
              ><tr>
                <td>{{ list.name }}</td>
                <td>{{ list.version_no }}</td>
                <td>{{ list.published_at }}</td>
                <td>
                  <button type="button" @click="preview(list.id)">{{ previewID === list.id ? '收起' : '预览' }}</button>
                </td>
              </tr>
              <tr v-if="previewID === list.id">
                <td colspan="4">
                  <p v-if="previewLoading">加载中…</p>
                  <table v-else class="price-preview">
                    <thead>
                      <tr>
                        <th>商品</th>
                        <th>规格</th>
                        <th>数量阶梯</th>
                        <th>计价单位</th>
                        <th>单价</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(row, i) in previewRows" :key="i">
                        <td>{{ row.product_name }}</td>
                        <td>{{ row.spec }}</td>
                        <td>
                          {{ row.min_qty }}–{{ row.max_qty === '' ? '不限' : row.max_qty }} {{ row.quantity_unit }}
                        </td>
                        <td>{{ row.price_unit }}</td>
                        <td>{{ money(row.unit_price) }}</td>
                      </tr>
                      <tr v-if="!previewRows.length">
                        <td colspan="5">暂无价格明细</td>
                      </tr>
                    </tbody>
                  </table>
                </td>
              </tr>
            </template>
            <tr v-if="!data.price_lists?.length">
              <td colspan="4">暂无已发布价格表</td>
            </tr>
          </tbody>
        </table>
      </section>
      <section v-else-if="portalPage === 'customerOrders'" class="panel">
        <h3>履约客户订单</h3>
        <form class="links" @submit.prevent="loadOrders(1)">
          <input v-model.trim="query" placeholder="搜索订单号或商品" /><button>查询</button
          ><button
            v-if="!customerAccountActor"
            type="button"
            :disabled="selectedIDs.length < 2"
            @click="salesIDs = [...selectedIDs]"
          >
            合并销售单
          </button>
        </form>
        <table>
          <thead>
            <tr>
              <th v-if="!customerAccountActor">
                <input
                  type="checkbox"
                  :checked="orders.length > 0 && selectedIDs.length === orders.length"
                  :indeterminate="selectedIDs.length > 0 && selectedIDs.length < orders.length"
                  @change="selectedIDs = $event.target.checked ? orders.map((r) => r.id) : []"
                />
              </th>
              <th>订单号 / 日期</th>
              <th>金额</th>
              <th>付款 / 发货</th>
              <th>收件信息</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in orders" :key="row.id">
              <td v-if="!customerAccountActor"><input v-model="selectedIDs" type="checkbox" :value="row.id" /></td>
              <td>
                {{ row.order_no }}<small>{{ row.order_date }}</small>
              </td>
              <td>{{ money(row.grand_total) }}</td>
              <td>{{ row.pay_status }} / {{ row.ship_status }}</td>
              <td>
                <span v-if="!completeRecipient(row)" class="warning">待补收件信息</span
                ><span v-else
                  >{{ row.receiver_name }}<small>{{ row.receiver_phone }}</small></span
                >
              </td>
              <td>
                <button type="button" @click="openDetail(row)">明细 / 收件信息</button
                ><button type="button" @click="salesIDs = [row.id]">销售单</button
                ><button
                  :disabled="!['已发货', '已出库', '已签收', '已收货', '已完成'].includes(row.ship_status)"
                  @click="deliveryID = row.id"
                >
                  出库单
                </button>
              </td>
            </tr>
            <tr v-if="!orders.length">
              <td colspan="6">暂无订单</td>
            </tr>
          </tbody>
        </table>
        <PaginationControls
          :page="page"
          :page-size="pageSize"
          :total="total"
          :disabled="loading"
          @change="
            ({ page: next, pageSize: size }) => {
              pageSize = size
              loadOrders(next)
            }
          "
        />
      </section>
      <template v-else-if="portalPage === 'customerInventory'">
        <section class="panel">
          <h3>原料 / 包材库存</h3>
          <table>
            <thead>
              <tr>
                <th>物料</th>
                <th>规格</th>
                <th>数量</th>
                <th>重量(g)</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, i) in data.custody_balances || []" :key="i">
                <td>{{ row.item_name }}</td>
                <td>{{ row.spec }}</td>
                <td>{{ row.quantity_units }}</td>
                <td>{{ row.quantity_g }}</td>
              </tr>
              <tr v-if="!data.custody_balances?.length">
                <td colspan="4">暂无托管库存</td>
              </tr>
            </tbody>
          </table>
        </section>
        <section class="panel">
          <h3>成品库存</h3>
          <table>
            <thead>
              <tr>
                <th>商品</th>
                <th>规格</th>
                <th>仓库</th>
                <th>数量</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, i) in data.finished_goods || []" :key="i">
                <td>{{ row.product_name }}</td>
                <td>{{ row.spec_g }}g</td>
                <td>{{ row.warehouse }}</td>
                <td>{{ row.quantity_units }}</td>
              </tr>
              <tr v-if="!data.finished_goods?.length">
                <td colspan="4">暂无成品库存</td>
              </tr>
            </tbody>
          </table>
        </section>
      </template>
      <section v-else-if="portalPage === 'customerProcessing'" class="panel">
        <h3>提交加工工单</h3>
        <form class="fields" @submit.prevent="submitProcessing">
          <label
            >成品<select v-model.number="processing.product_id" required>
              <option :value="0">选择商品</option>
              <option v-for="row in options.customer_skus || []" :key="row.product_id" :value="row.product_id">
                {{ row.product_name }}
              </option>
            </select></label
          ><label
            >原料<select v-model.number="processing.raw_bean_item_id" required>
              <option :value="0">选择托管原料</option>
              <option v-for="row in options.custody_items || []" :key="row.item_id" :value="row.item_id">
                {{ row.item_name }}
              </option>
            </select></label
          ><label>投料克重<input v-model.number="processing.input_quantity_g" type="number" min="1" required /></label
          ><label
            >计划产量<input v-model.number="processing.planned_output_units" type="number" min="1" required /></label
          ><label>期望日期<input v-model="processing.expected_date" type="date" /></label
          ><label>备注<input v-model="processing.note" /></label><button :disabled="saving">提交工单</button>
        </form>
        <table>
          <thead>
            <tr>
              <th>工单号</th>
              <th>成品</th>
              <th>状态</th>
              <th>数量</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in data.processing_orders || []" :key="row.work_order_no">
              <td>{{ row.work_order_no }}</td>
              <td>{{ row.product_name }}</td>
              <td>{{ row.status }}</td>
              <td>{{ row.units }}</td>
            </tr>
          </tbody>
        </table>
      </section>
      <section v-else-if="portalPage === 'customerSettlement'" class="panel">
        <h3>结算中心</h3>
        <div class="links">
          <button @click="navigate('financeExpenses')">费用明细</button
          ><button @click="navigate('financeReport')">经营报告</button
          ><button @click="navigate('financeClosing')">结账相关</button>
        </div>
        <table>
          <thead>
            <tr>
              <th>结算单</th>
              <th>期间</th>
              <th>金额</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, i) in data.settlements || []" :key="i">
              <td>{{ row.settlement_no }}</td>
              <td>{{ row.period_start }} ~ {{ row.period_end }}</td>
              <td>{{ money(row.total_amount) }}</td>
              <td>{{ row.status }}</td>
            </tr>
            <tr v-if="!data.settlements?.length">
              <td colspan="4">暂无结算单</td>
            </tr>
          </tbody>
        </table>
      </section>
      <CustomerMallView v-else-if="portalPage === 'customerMall'" :customer-id="data.customer_id" />
    </template>
    <div v-if="detail" class="drawer-mask" @click.self="detail = null">
      <aside class="drawer">
        <button @click="detail = null">关闭</button>
        <h3>{{ detail.order_no }} · 订单明细</h3>
        <p>价格表版本：{{ detail.bean_list_version_no || '按明细' }}</p>
        <p>
          订单金额 {{ money(detail.grand_total) }} · 运费 {{ money(detail.shipping_amount) }} · 优惠
          {{ money(detail.discount_amount) }}
        </p>
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>规格</th>
              <th>数量</th>
              <th>金额</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, i) in detail.items || []" :key="i">
              <td>{{ row.customer_product_display_name_snapshot || row.product_name || row.name }}</td>
              <td>
                {{ row.bom_spec_name || row.spec }}<small>{{ row.bean_list_version_no }}</small>
              </td>
              <td>{{ row.qty }}</td>
              <td>{{ money(row.line_total) }}</td>
            </tr>
          </tbody>
        </table>
        <form class="fields" @submit.prevent="saveRecipient">
          <label>收件人<input v-model.trim="recipient.receiver_name" required /></label
          ><label>电话<input v-model.trim="recipient.receiver_phone" required /></label
          ><label>地址<input v-model.trim="recipient.receiver_address" required /></label>
          <p v-if="!canEditRecipient">已发货或作废订单不能修改收件信息。</p>
          <button :disabled="saving || !canEditRecipient">保存收件信息</button>
        </form>
      </aside>
    </div>
    <div v-if="deliveryID" class="drawer-mask" @click.self="deliveryID = 0">
      <aside class="drawer"><DeliveryNoteView :order-id="deliveryID" embedded @close="deliveryID = 0" /></aside>
    </div>
    <div v-if="salesIDs.length" class="drawer-mask" @click.self="salesIDs = []">
      <aside class="drawer">
        <SalesOrderView
          :order-id="salesIDs.length === 1 ? salesIDs[0] : 0"
          :order-ids="salesIDs.length > 1 ? salesIDs : []"
          embedded
          @close="salesIDs = []"
        />
      </aside>
    </div>
  </div>
</template>
<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import OrderEntryView from './OrderEntryView.vue'
import SalesOrderView from './SalesOrderView.vue'
import PaginationControls from '../components/PaginationControls.vue'
import DeliveryNoteView from './DeliveryNoteView.vue'
import { fetchCustomerFulfillmentOrders, fetchCustomerFulfillmentOrderDetail } from '../api/customer-fulfillment'
import CustomerMallView from './CustomerMallView.vue'
import { customerWorkspaceMenu, customerWorkspaceCapability, customerWorkspacePages } from '../lib/customer-workspace'
const props = defineProps({
  portalPage: { type: String, default: 'customerProcessingPortal' },
  viewParams: { type: Object, default: () => ({}) },
  customerContextId: { type: [String, Number], default: 0 },
  customerContextLabel: { type: String, default: '' },
  customerAccountActor: { type: Boolean, default: false }
})
const internalCustomerID = ref(0),
  internalCustomers = ref([]),
  pageSize = ref(10),
  deliveryID = ref(0)
const loading = ref(false),
  saving = ref(false),
  error = ref(''),
  message = ref(''),
  data = ref({}),
  options = ref({}),
  previewID = ref(0),
  previewRows = ref([]),
  previewLoading = ref(false),
  orders = ref([]),
  query = ref(''),
  page = ref(1),
  total = ref(0),
  selectedIDs = ref([]),
  detail = ref(null),
  salesIDs = ref([])
const recipient = reactive({ receiver_name: '', receiver_phone: '', receiver_address: '' })
const processing = reactive({
  product_id: 0,
  raw_bean_item_id: 0,
  input_quantity_g: '',
  planned_output_units: '',
  expected_date: '',
  note: ''
})
const capability = computed(() => customerWorkspaceCapability(props.portalPage))
const title = computed(() =>
  props.portalPage === 'customerProcessingPortal'
    ? '首页'
    : props.portalPage === 'customerOrders'
      ? '我的订单'
      : customerWorkspacePages.find((p) => p[0] === props.portalPage)?.[1] || '客户中心'
)
const menuItems = computed(() =>
  customerWorkspaceMenu(data.value.capabilities)
    .flatMap((g) => g.items)
    .filter((i) => i.key !== 'customerProcessingPortal')
)
const metrics = computed(() =>
  [
    ['orders', '订单数量'],
    ['pending_shipment', '待发货'],
    ['missing_recipient', '待补收件信息'],
    ['warehouses', '托管仓库'],
    ['finished_goods_units', '成品库存数量'],
    ['finished_goods_kg', '成品库存(kg)'],
    ['pending_settlement_amount', '待结算金额']
  ]
    .filter(([k]) => data.value[k] !== undefined)
    .map(([k, label]) => ({ label, value: k.includes('amount') ? money(data.value[k]) : data.value[k] }))
)
function money(value) {
  return Number(value || 0).toFixed(2)
}
function completeRecipient(row) {
  return ['receiver_name', 'receiver_phone', 'receiver_address'].every((k) => String(row[k] || '').trim())
}
const canEditRecipient = computed(
  () =>
    detail.value &&
    !detail.value.is_void &&
    !['已发货', '已出库', '已签收', '已收货', '已完成'].includes(detail.value.ship_status)
)
function scopedURL(path, params = {}) {
  const query = new URLSearchParams(params)
  if (!props.customerAccountActor) {
    const id = Number(
      internalCustomerID.value || props.customerContextId || props.viewParams.customer_id || data.value.customer_id || 0
    )
    if (id) query.set('customer_id', String(id))
  }
  return path + (query.size ? '?' + query : '')
}
function navigate(key) {
  window.dispatchEvent(
    new CustomEvent('kferp:navigate-view', {
      detail: {
        key,
        params: { customer_id: data.value.customer_id },
        returnNavigation: { key: props.portalPage, params: { customer_id: data.value.customer_id }, label: title.value }
      }
    })
  )
}
let loadVersion = 0
async function load() {
  const version = ++loadVersion
  loading.value = true
  error.value = ''
  previewID.value = 0
  selectedIDs.value = []
  detail.value = null
  try {
    if (
      !props.customerAccountActor &&
      !internalCustomerID.value &&
      !props.customerContextId &&
      !props.viewParams.customer_id
    ) {
      internalCustomers.value = (await apiGet('/api/customer-fulfillment/customers?limit=200')).customers || []
      return
    }
    const section =
      props.portalPage === 'customerProcessingPortal'
        ? 'home'
        : props.portalPage === 'customerOrders'
          ? 'context'
          : capability.value
    const result = await apiGet(scopedURL('/api/customer-processing/portal/workspace', { page: section }))
    if (version !== loadVersion) return
    data.value = result
    if (props.portalPage === 'customerOrders') await loadOrders(1)
    if (props.portalPage === 'customerProcessing')
      options.value = await apiGet(
        props.customerAccountActor
          ? '/api/customer-processing/portal/options'
          : `/api/customer-processing/internal/${data.value.customer_id}/options`
      )
  } catch (e) {
    if (version === loadVersion) error.value = e.message
  } finally {
    if (version === loadVersion) loading.value = false
  }
}
async function preview(id) {
  if (previewID.value === id) {
    previewID.value = 0
    return
  }
  previewID.value = id
  previewLoading.value = true
  previewRows.value = []
  error.value = ''
  try {
    const result = await apiGet(
      scopedURL('/api/customer-processing/portal/workspace', { page: 'bean_list', publication_id: id })
    )
    if (previewID.value === id) previewRows.value = result.rows || []
  } catch (e) {
    error.value = e.message
  } finally {
    if (previewID.value === id) previewLoading.value = false
  }
}
async function loadOrders(next) {
  error.value = ''
  try {
    const result = await fetchCustomerFulfillmentOrders(data.value.customer_id, {
      page: next,
      limit: pageSize.value,
      q: query.value
    })
    orders.value = result.rows || []
    total.value = result.total || result.summary?.orders || 0
    page.value = next
    selectedIDs.value = []
  } catch (e) {
    error.value = e.message
  }
}
async function openDetail(row) {
  error.value = ''
  try {
    const result = await fetchCustomerFulfillmentOrderDetail(row.id)
    detail.value = {
      ...row,
      ...(result.edit_data || result.order || result),
      items: result.edit_data?.items || result.items || result.order?.items || []
    }
    Object.assign(recipient, Object.fromEntries(Object.keys(recipient).map((k) => [k, detail.value[k] || ''])))
  } catch (e) {
    error.value = e.message
  }
}
async function saveRecipient() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    await apiSend(scopedURL(`/api/customer-processing/portal/orders/${detail.value.id}/recipient`), {
      method: 'PATCH',
      body: recipient
    })
    message.value = '收件信息已保存'
    detail.value = null
    await loadOrders(page.value)
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
async function submitProcessing() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    const product = (options.value.customer_skus || []).find((r) => r.product_id === processing.product_id)
    const raw = (options.value.custody_items || []).find((r) => r.item_id === processing.raw_bean_item_id)
    const path = props.customerAccountActor
      ? '/api/customer-processing/portal/work-orders'
      : `/api/customer-fulfillment/${data.value.customer_id}/work-orders`
    await apiSend(path, {
      body: { ...processing, product_name: product?.product_name || '', raw_bean_name: raw?.item_name || '' }
    })
    message.value = '加工工单已提交'
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
watch(() => [props.portalPage, props.customerContextId, props.viewParams.customer_id], load, { immediate: true })
</script>
<style scoped>
.page {
  display: grid;
  gap: 16px;
}
.portal-head,
.links {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.panel {
  padding: 18px;
  background: white;
  border: 1px solid #e2e8e4;
  border-radius: 10px;
  overflow: auto;
}
.metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}
.metrics article {
  display: grid;
  gap: 12px;
  padding: 20px;
  background: #f1f6f2;
  border: 1px solid #dce6de;
  border-radius: 10px;
}
.metrics strong {
  font-size: 28px;
}
.fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 14px;
  margin: 16px 0;
}
label {
  display: grid;
  gap: 6px;
}
table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}
th,
td {
  padding: 12px;
  border-bottom: 1px solid #e6ebe7;
}
th {
  white-space: nowrap;
}
small {
  display: block;
  color: #66736c;
}
button {
  cursor: pointer;
  border: 1px solid #cbd9ce;
  border-radius: 6px;
  padding: 8px 12px;
  background: #f6f9f6;
  color: #284731;
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
input,
select {
  padding: 8px;
  border: 1px solid #cad5cc;
  border-radius: 5px;
  max-width: 100%;
}
.notice {
  padding: 12px;
  border-radius: 6px;
}
.error {
  color: #a32d2d;
  background: #fff0f0;
}
.ok {
  color: #285b32;
  background: #edf8ee;
}
.warning {
  color: #a35a16;
}
.drawer-mask {
  position: fixed;
  inset: 0;
  background: #0005;
  z-index: 60;
  display: flex;
  justify-content: flex-end;
}
.drawer {
  width: min(1100px, 95vw);
  background: #fff;
  padding: 24px;
  overflow: auto;
}
.price-preview {
  background: #f8faf8;
}
.links {
  justify-content: flex-start;
  margin: 10px 0;
}
</style>
