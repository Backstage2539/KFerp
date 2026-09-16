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
        <h3>提交代加工申请</h3>
        <p>成品配置以已发布的默认 BOM 为准。先预览可生产上限；提交时系统会再次校验，并立即预订客户原料和包材。</p>
        <form class="processing-request" @submit.prevent="submitProcessing">
          <div v-for="(item, index) in processing.items" :key="item.row_id" class="processing-line">
            <label
              >成品 / 规格<select v-model="item.target_key" required @change="applyProcessingTarget(item)">
                <option value="">选择已配置的代加工商品</option>
                <option v-for="target in processingTargets" :key="targetKey(target)" :value="targetKey(target)">
                  {{ processingTargetLabel(target) }}
                </option>
              </select></label
            ><label
              >申请数量<input v-model.number="item.qty" type="number" min="1" step="1" required @input="processingPreview = null"
            /></label>
            <button v-if="processing.items.length > 1" type="button" @click="removeProcessingLine(index)">删除</button>
          </div>
          <div class="links">
            <button type="button" @click="addProcessingLine">增加成品</button>
            <label>期望日期<input v-model="processing.expected_completion_date" type="date" /></label>
            <label>备注<input v-model.trim="processing.note" /></label>
          </div>
          <div class="links">
            <button type="button" :disabled="saving || !processingPayloadItems.length" @click="previewProcessingRequest">预览可生产量</button>
            <button :disabled="saving || !processingPreview?.can_submit">确认提交并预订原料</button>
          </div>
        </form>
        <div v-if="processingPreview" class="processing-preview" :class="{ blocked: !processingPreview.can_submit }">
          <strong>{{ processingPreview.can_submit ? '当前可提交' : '当前不能提交' }}</strong>
          <p v-if="!processingPreview.can_submit">请把申请数量调整到可生产上限以内；整张申请不会部分提交。</p>
          <table>
            <thead><tr><th>成品</th><th>申请</th><th>可生产上限</th><th>原料 / 包材</th></tr></thead>
            <tbody>
              <tr v-for="(row, index) in processingPreview.items || []" :key="row.id || index">
                <td>{{ row.product_name }} {{ row.spec_name }}</td><td>{{ row.qty }}</td><td>{{ row.max_producible_qty }}</td>
                <td>{{ (row.materials || []).map((material) => `${material.material_name}：需 ${material.required_g || material.required_units}，缺 ${material.shortage_g || material.shortage_units}`).join('；') || '无需领料' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <h3>代加工申请进度</h3>
        <table>
          <thead>
            <tr>
              <th>申请单号</th>
              <th>来源 / 客户</th>
              <th>成品</th>
              <th>状态</th>
              <th>申请 / 入库 / 可预订</th>
              <th>计划 / 工单</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in processingRequests" :key="row.id" class="clickable-row" @click="selectedProcessingRequest = row">
              <td><strong>{{ row.request_no }}</strong><small>{{ row.created_at }}</small></td>
              <td>客户工单<small>{{ row.customer_name || data.customer_name }}</small></td>
              <td>{{ (row.items || []).map((item) => `${item.product_name} ${item.spec_name || ''}`).join('、') }}</td>
              <td>{{ processingStatusLabel(row.status) }}</td>
              <td>{{ row.requested_qty || 0 }} / {{ row.actual_inbound_qty || 0 }} / {{ row.remaining_reservable_qty || 0 }}</td>
              <td>{{ (row.items || []).map((item) => item.work_order_no || (item.production_plan_id ? `计划 #${item.production_plan_id}` : '待计划')).join('、') }}</td>
            </tr>
            <tr v-if="!processingRequests.length"><td colspan="6">暂无代加工申请</td></tr>
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
    <div v-if="selectedProcessingRequest" class="drawer-mask" @click.self="selectedProcessingRequest = null">
      <aside class="drawer processing-detail-drawer">
        <div class="portal-head"><div><small>客户工单详情</small><h3>{{ selectedProcessingRequest.request_no }}</h3><p>{{ selectedProcessingRequest.customer_name || data.customer_name }} · {{ processingStatusLabel(selectedProcessingRequest.status) }}</p></div><button @click="selectedProcessingRequest = null">关闭</button></div>
        <section class="processing-timeline"><article v-for="step in customerProcessingTimeline(selectedProcessingRequest)" :key="step.key" :class="step.state"><span></span><strong>{{ step.label }}</strong><small>{{ step.time }}</small></article></section>
        <section class="metrics processing-detail-metrics"><article><span>申请数量</span><strong>{{ customerProcessingRequestProgress.requestedQty }}</strong></article><article><span>累计入库</span><strong>{{ customerProcessingRequestProgress.inboundQty }}</strong></article><article><span>订单占用</span><strong>{{ customerProcessingRequestProgress.orderOccupiedQty }}</strong></article><article><span>剩余可预订</span><strong>{{ customerProcessingRequestProgress.remainingReservableQty }}</strong></article></section>
        <article v-for="item in selectedProcessingRequest.items || []" :key="item.id" class="processing-detail-item">
          <div class="portal-head"><div><strong>{{ item.product_name }}</strong><small>{{ item.spec_name || `${item.spec_g}g` }} · BOM {{ item.bom_version_no || '-' }}</small></div><span>{{ processingStatusLabel(item.status) }}</span></div>
          <div class="processing-detail-grid"><span>目标仓库 <strong>{{ item.target_warehouse || '-' }}</strong></span><span>物料预订 <strong>{{ item.material_reserved_g || item.material_reserved_units || 0 }}</strong></span><span>生产计划 <strong>{{ item.production_plan_id || '-' }}</strong></span><span>生产工单 <strong>{{ item.work_order_no || '-' }}</strong></span></div>
          <button v-if="item.work_order_id" @click="navigateWorkOrder(item.work_order_id)">查看生产工单</button>
          <div v-for="order in item.related_orders || []" :key="order.order_id" class="related-order"><span>{{ order.order_no }}</span><span>预订 {{ order.reserved_qty }} · 已转库存 {{ order.converted_qty }} · 缺口 {{ order.shortfall_qty }}</span></div>
        </article>
      </aside>
    </div>
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
import { customerProcessingProgress, customerProcessingTimeline } from '../lib/customer-processing-trace'
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
  processingCatalog = ref({}),
  processingRequests = ref([]),
  selectedProcessingRequest = ref(null),
  processingPreview = ref(null),
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
let processingRowID = 0
function newProcessingLine() {
  processingRowID += 1
  return { row_id: processingRowID, target_key: '', product_id: 0, bom_spec_id: 0, bom_variant_id: 0, spec_g: 0, qty: 1 }
}
const processing = reactive({ items: [newProcessingLine()], expected_completion_date: '', note: '' })
const processingTargets = computed(() => processingCatalog.value.targets || [])
const customerProcessingRequestProgress = computed(() => {
  const rows = selectedProcessingRequest.value?.items || []
  return rows.reduce((total, item) => {
    const progress = customerProcessingProgress({ target_qty: item.qty, actual_inbound_qty: item.actual_inbound_qty, output_reserved_qty: item.output_reserved_qty, output_converted_qty: item.output_converted_qty })
    for (const key of Object.keys(total)) total[key] += progress[key]
    return total
  }, { requestedQty: 0, inboundQty: 0, orderOccupiedQty: 0, remainingReservableQty: 0 })
})
const processingPayloadItems = computed(() =>
  processing.items
    .filter((item) => item.product_id > 0 && Number(item.qty) > 0)
    .map(({ product_id, bom_spec_id, bom_variant_id, spec_g, qty }) => ({
      product_id,
      bom_spec_id,
      bom_variant_id,
      spec_g,
      qty: Number(qty)
    }))
)
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
function processingEndpointBase() {
  return props.customerAccountActor
    ? '/api/customer-processing/portal'
    : `/api/customer-processing/internal/${data.value.customer_id}`
}
function targetKey(target) {
  return [target.product_id || 0, target.bom_spec_id || 0, target.bom_variant_id || 0, target.spec_g || 0].join(':')
}
function processingProduct(target) {
  return (processingCatalog.value.products || []).find(
    (product) => Number(product.product_id) === Number(target.product_id) || Number(product.base_product_id) === Number(target.product_id)
  )
}
function processingTargetLabel(target) {
  const product = processingProduct(target)
  const name = product?.customer_product_display_name || product?.product_name || `商品 ${target.product_id}`
  return `${name} · ${target.spec_name || (target.spec_g ? `${target.spec_g}g` : target.inventory_unit || '默认规格')}`
}
function applyProcessingTarget(item) {
  const target = processingTargets.value.find((row) => targetKey(row) === item.target_key)
  Object.assign(item, {
    product_id: Number(target?.product_id || 0),
    bom_spec_id: Number(target?.bom_spec_id || 0),
    bom_variant_id: Number(target?.bom_variant_id || 0),
    spec_g: Number(target?.spec_g || 0)
  })
  processingPreview.value = null
}
function addProcessingLine() {
  processing.items.push(newProcessingLine())
  processingPreview.value = null
}
function removeProcessingLine(index) {
  processing.items.splice(index, 1)
  processingPreview.value = null
}
function processingStatusLabel(status) {
  return (
    {
      submitted: '待接单',
      draft: '待接单',
      awaiting_schedule: '待接单',
      planned: '待接单',
      released: '已接单',
      running: '开始生产',
      paused: '开始生产（暂停）',
      partially_completed: '开始生产（部分入库）',
      completed: '生产完成',
      cancelled: '已取消'
    }[String(status || '').trim()] || '待接单'
  )
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
function navigateWorkOrder(workOrderID) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'workOrders', params: { work_order_id: Number(workOrderID), customer_id: data.value.customer_id }, returnNavigation: { key: props.portalPage, params: { customer_id: data.value.customer_id }, label: '返回客户生产工单' } } }))
  selectedProcessingRequest.value = null
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
    if (props.portalPage === 'customerProcessing') await loadProcessingWorkspace()
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
    if (!processingPreview.value?.can_submit) throw new Error('请先预览并确认当前可生产量')
    await apiSend(`${processingEndpointBase()}/processing-requests`, {
      body: {
        idempotency_key: `erp-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`,
        items: processingPayloadItems.value,
        expected_completion_date: processing.expected_completion_date,
        note: processing.note
      }
    })
    message.value = '代加工申请已提交，原料和包材已预订'
    processing.items.splice(0, processing.items.length, newProcessingLine())
    processing.expected_completion_date = ''
    processing.note = ''
    processingPreview.value = null
    await loadProcessingWorkspace()
  } catch (e) {
    error.value = e.message
    if (e.status === 409) {
      try {
        await previewProcessingRequest()
        error.value = '原料或包材可用量已变化，请按最新预览调整'
      } catch {}
    }
  } finally {
    saving.value = false
  }
}
async function loadProcessingWorkspace() {
  const base = processingEndpointBase()
  const [catalog, requests] = await Promise.all([
    apiGet(`${base}/processing-catalog`),
    apiGet(`${base}/processing-requests?limit=100`)
  ])
  processingCatalog.value = catalog || {}
  processingRequests.value = requests.rows || []
  openRequestedProcessingRequest()
}
function openRequestedProcessingRequest() {
  const requestedID = Number(props.viewParams.processing_request_id || 0)
  const requestedNo = String(props.viewParams.processing_request_no || '').trim()
  if (!requestedID && !requestedNo) return
  selectedProcessingRequest.value = processingRequests.value.find((row) =>
    (requestedID > 0 && Number(row.id || 0) === requestedID) ||
    (requestedNo && String(row.request_no || '').trim() === requestedNo)
  ) || null
}
async function previewProcessingRequest() {
  if (!processingPayloadItems.value.length) throw new Error('请先选择成品规格和申请数量')
  error.value = ''
  processingPreview.value = await apiSend(`${processingEndpointBase()}/processing-requests/preview`, {
    body: { items: processingPayloadItems.value }
  })
  return processingPreview.value
}
watch(() => [props.portalPage, props.customerContextId, props.viewParams.customer_id], load, { immediate: true })
watch(() => [props.viewParams.processing_request_id, props.viewParams.processing_request_no], openRequestedProcessingRequest)
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
.processing-request,
.processing-preview {
  display: grid;
  gap: 12px;
  margin: 16px 0;
}
.processing-line {
  display: grid;
  grid-template-columns: minmax(260px, 2fr) minmax(120px, 1fr) auto;
  gap: 12px;
  align-items: end;
  padding: 12px;
  border: 1px solid #dce6de;
  border-radius: 8px;
  background: #f8faf8;
}
.processing-preview {
  padding: 14px;
  border: 1px solid #b9d8be;
  border-radius: 8px;
  background: #f1faf2;
}
.processing-preview.blocked {
  border-color: #efb2b2;
  background: #fff3f3;
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
.clickable-row{cursor:pointer}.clickable-row:hover{background:#f5faf6}.processing-detail-drawer{width:min(960px,96vw)}.processing-timeline{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;padding:18px;margin:18px 0;border:1px solid #e4e8e5;border-radius:10px}.processing-timeline article{display:grid;justify-items:center;gap:6px;text-align:center}.processing-timeline article>span{width:14px;height:14px;border-radius:50%;background:#cfd7d2}.processing-timeline article.done>span,.processing-timeline article.current>span{background:#2f7a50}.processing-timeline article.current strong{color:#2f7a50}.processing-detail-metrics{margin-bottom:16px}.processing-detail-item{display:grid;gap:12px;padding:16px;margin-bottom:12px;border:1px solid #e2e8e4;border-radius:10px}.processing-detail-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:10px}.processing-detail-grid span{display:grid;gap:5px;color:#66736c}.related-order{display:flex;justify-content:space-between;gap:12px;padding:10px;border-radius:7px;background:#f7f5f1}@media(max-width:720px){.processing-timeline,.processing-detail-grid{grid-template-columns:repeat(2,1fr)}}
.links {
  justify-content: flex-start;
  margin: 10px 0;
}
</style>
