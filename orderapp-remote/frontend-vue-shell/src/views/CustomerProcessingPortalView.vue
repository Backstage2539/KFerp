<template>
  <div class="page customer-processing-portal">
    <section class="portal-head">
      <div>
        <h2>{{ overview.customer_name || '客户履约工作台' }}</h2>
        <p>客户登录 · 查看数据、提交工单和履约订单信息</p>
      </div>
      <div class="head-actions">
        <label v-if="!internalCustomerID && !customerAccountActor" class="customer-picker-field">
          <span>选择客户</span>
          <SearchableSelect
            v-model="adminCustomerValue"
            :options="adminCustomerOptions"
            :option-label="adminCustomerOptionLabel"
            :option-meta="adminCustomerOptionMeta"
            :option-value="optionNumericValue"
            placeholder="搜索客户名/公司/联系人"
            empty-text="没有匹配客户"
            :disabled="loading"
            @select="selectAdminCustomer" />
        </label>
        <button class="secondary" type="button" @click="loadOverview" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="ok" class="ok">{{ ok }}</div>

    <section class="metric-grid">
      <div v-for="item in metrics" :key="item.label" class="metric">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
      </div>
    </section>

    <section class="form-grid">
      <form v-if="canSubmitProcessing" class="panel" @submit.prevent="submitProcessing">
        <div class="panel-head">
          <h3>提交加工工单</h3>
        </div>
        <div class="fields">
          <label>
            <span>成品名称</span>
            <SearchableSelect
              v-model="processingProductValue"
              :options="customerSKUOptions"
              :option-label="productOptionLabel"
              :option-meta="productOptionMeta"
              :option-value="productOptionValue"
              empty-value=""
              placeholder="搜索该客户 SKU"
              empty-text="没有匹配 SKU"
              :disabled="loading"
              @select="selectProcessingProduct" />
          </label>
          <label>
            <span>原料名称</span>
            <SearchableSelect
              v-model="processingRawBeanValue"
              :options="rawBeanOptions"
              :option-label="custodyOptionLabel"
              :option-meta="custodyOptionMeta"
              :option-value="custodyOptionValue"
              empty-value=""
              placeholder="搜索托管原料"
              empty-text="没有匹配原料"
              :disabled="loading"
              @select="selectProcessingRawBean" />
          </label>
          <label>
            <span>投豆克重</span>
            <input v-model.number="processingForm.input_quantity_g" type="number" min="1" required />
          </label>
          <label>
            <span>计划产量</span>
            <input v-model.number="processingForm.planned_output_units" type="number" min="1" required />
          </label>
          <label>
            <span>期望日期</span>
            <input v-model="processingForm.expected_date" type="date" />
          </label>
          <label class="wide">
            <span>备注</span>
            <input v-model.trim="processingForm.note" placeholder="加工要求" />
          </label>
        </div>
        <button class="primary" type="submit" :disabled="loading">提交工单</button>
      </form>

      <form v-if="canDirectShip" class="panel" @submit.prevent="submitDirectShip">
        <div class="panel-head">
          <h3>{{ submitCopy.formTitle }}</h3>
        </div>
        <div class="direct-ship-form">
          <div class="direct-ship-recipient">
            <label class="recipient-paste">
              <span>粘贴收件信息</span>
              <textarea v-model="recipientPasteText" rows="2" placeholder="粘贴姓名、电话、地址" @paste.prevent="pasteRecipientInfo"></textarea>
            </label>
            <button class="secondary parse-button" type="button" @click="applyRecipientParse()" :disabled="loading">解析收件信息</button>
            <label>
              <span>历史收件信息</span>
              <SearchableSelect
                v-model="recipientHistoryValue"
                :options="recipientOptions"
                :option-label="recipientOptionLabel"
                :option-meta="recipientOptionMeta"
                :option-value="recipientOptionValue"
                empty-value=""
                placeholder="搜索姓名/电话/地址"
                empty-text="没有历史收件信息"
                :disabled="loading"
                @select="selectRecipientHistory" />
            </label>
            <label>
              <span>收件人</span>
              <input v-model.trim="directShipForm.receiver_name" required />
            </label>
            <label>
              <span>电话</span>
              <input v-model.trim="directShipForm.receiver_phone" required />
            </label>
            <label class="recipient-address">
              <span>地址</span>
              <input v-model.trim="directShipForm.receiver_address" required />
            </label>
          </div>

          <section class="direct-ship-items">
            <div class="direct-ship-items-head">
              <span class="muted">一个收件信息可添加多行商品</span>
            </div>
            <div class="table-wrap">
              <table class="order-lines-table">
                <thead>
                  <tr>
                    <th>商品</th>
                    <th>规格(g)</th>
                    <th>数量</th>
                    <th>单价</th>
                    <th>小计</th>
                    <th>阶梯价</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, idx) in directShipItems" :key="row.key">
                    <td class="product-cell">
                      <SearchableSelect
                        v-model="row.product_value"
                        :options="directShipProductOptions"
                        :option-label="productOptionLabel"
                        :option-meta="productOptionMeta"
                        :option-value="productOptionValue"
                        empty-value=""
                        placeholder="搜索客户 SKU/公共 SKU"
                        empty-text="没有匹配商品"
                        :disabled="loading"
                        @select="(option) => selectDirectShipItemProduct(row, option)" />
                    </td>
                    <td><input v-model.number="row.spec_g" type="number" min="1" step="1" @input="syncDirectShipItemPrice(row)" /></td>
                    <td><input v-model.number="row.qty" type="number" min="1" step="1" @input="syncDirectShipItemPrice(row)" /></td>
                    <td class="price-cell">
                      <input :value="row.unit_price || ''" type="text" disabled />
                      <small>{{ priceUnitLabel(row) }}</small>
                    </td>
                    <td class="subtotal-cell">{{ money(rowLineTotal(row)) }}</td>
                    <td>
                      <div v-if="rowTierRows(row).length" class="tier-chips">
                        <span
                          v-for="tier in rowTierRows(row)"
                          :key="`${row.key}-${tier.id}-${tier.rangeLabel}`"
                          class="tier-chip"
                          :class="{ active: rowTierActive(row, tier) }">
                          {{ tier.rangeLabel }} {{ money(tier.unitPrice) }}{{ tier.priceUnit.suffix }}
                        </span>
                      </div>
                      <span v-else class="muted">-</span>
                    </td>
                    <td>
                      <button class="secondary danger" type="button" :disabled="directShipItems.length <= 1" @click="removeDirectShipItem(idx)">删除</button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <div class="direct-ship-footer">
            <div class="line-total grand-total">
              <span>订单合计</span>
              <strong>{{ money(directShipGrandTotal) }}</strong>
            </div>
            <label class="note-field">
              <span>备注</span>
              <textarea v-model.trim="directShipForm.note" rows="2" :placeholder="submitCopy.notePlaceholder" />
            </label>
          </div>
        </div>
        <button class="primary" type="submit" :disabled="loading">{{ submitCopy.submitButton }}</button>
      </form>
    </section>

    <section class="grid-2">
      <div v-if="canViewInventory" class="panel">
        <div class="panel-head"><h3>原料托管库存</h3></div>
        <table>
          <thead><tr><th>类型</th><th>名称</th><th>规格</th><th>克重</th><th>件数</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.custody_balances || []" :key="`${row.item_type}-${row.item_name}-${row.spec}`">
              <td>{{ custodyTypeLabel(row.item_type) }}</td>
              <td>{{ row.item_name }}</td>
              <td>{{ row.spec || '-' }}</td>
              <td>{{ formatG(row.quantity_g) }}</td>
              <td>{{ row.quantity_units || 0 }}</td>
            </tr>
            <tr v-if="!(overview.custody_balances || []).length"><td colspan="5" class="muted">暂无库存</td></tr>
          </tbody>
        </table>
      </div>

      <div v-if="canViewInventory" class="panel">
        <div class="panel-head"><h3>成品库存</h3></div>
        <table>
          <thead><tr><th>产品</th><th>规格</th><th>仓库</th><th>克重</th><th>件数</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.finished_goods || []" :key="`${row.product_id}-${row.product_name}-${row.spec_g}-${row.warehouse}`">
              <td>{{ row.product_name }}</td>
              <td>{{ row.spec_g ? `${row.spec_g}g` : '-' }}</td>
              <td>{{ row.warehouse || '-' }}</td>
              <td>{{ formatG(row.quantity_g) }}</td>
              <td>{{ row.quantity_units || 0 }}</td>
            </tr>
            <tr v-if="!(overview.finished_goods || []).length"><td colspan="5" class="muted">暂无成品库存</td></tr>
          </tbody>
        </table>
      </div>

      <div v-if="canSubmitProcessing" class="panel">
        <div class="panel-head"><h3>加工进度</h3></div>
        <table>
          <thead><tr><th>工单号</th><th>产品</th><th>状态</th><th>投豆</th><th>产量</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.processing_orders || []" :key="row.work_order_no">
              <td>{{ row.work_order_no }}</td>
              <td>{{ row.product_name }}</td>
              <td>{{ statusLabel(row.status) }}</td>
              <td>{{ formatG(row.quantity_g) }}</td>
              <td>{{ row.units || 0 }}</td>
            </tr>
            <tr v-if="!(overview.processing_orders || []).length"><td colspan="5" class="muted">暂无工单</td></tr>
          </tbody>
        </table>
      </div>

    </section>

    <section class="panel fulfillment-orders-panel">
      <div class="panel-head">
        <div>
          <h3>履约客户订单</h3>
          <p>按当前履约账户同步 ERP 后台订单列表，销售单和出库单在订单明细中处理。</p>
        </div>
        <div class="head-actions">
          <span class="muted">共 {{ fulfillmentOrdersSummary.orders || 0 }} 单</span>
          <button class="secondary" type="button" @click="loadFulfillmentOrders(fulfillmentOrdersPage)" :disabled="fulfillmentOrdersLoading || !overviewCustomerId">
            刷新订单
          </button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="fulfillment-orders-table">
          <thead>
            <tr>
              <th>订单号</th>
              <th>日期</th>
              <th>客户 / 类型</th>
              <th>收件信息</th>
              <th>订单费用</th>
              <th>快递信息</th>
              <th>订单状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in fulfillmentOrders" :key="row.id">
              <td>
                <button class="order-link" type="button" @click="openFulfillmentOrderDetail(row)">{{ row.order_no }}</button>
                <div class="cell-meta">{{ row.portal_service_code || row.order_type || '-' }}</div>
              </td>
              <td>{{ row.order_date || '-' }}</td>
              <td>
                <div class="stacked-text">
                  <strong>{{ row.customer || overview.customer_name || '-' }}</strong>
                  <span>{{ row.order_type || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="stacked-text receiver-cell">
                  <strong>{{ row.receiver_name || '-' }} {{ row.receiver_phone || '' }}</strong>
                  <span>{{ row.receiver_address || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="fee-stack">
                  <div v-for="fee in orderFeeLines(row)" :key="`${row.id}-${fee.label}`" class="fee-line" :class="{ emphasized: fee.emphasized }">
                    <span>{{ fee.label }}</span>
                    <strong>{{ fee.value }}</strong>
                  </div>
                </div>
              </td>
              <td>
                <div class="stacked-text">
                  <strong>{{ row.sender_label || row.sender_name || '-' }}</strong>
                  <span>{{ row.ship_tracking_no || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="status-stack">
                  <span>收款：{{ row.pay_status || '-' }}</span>
                  <span>发货：{{ row.ship_status || '-' }}</span>
                  <span>生产：{{ row.process_status || '-' }}</span>
                  <span>发票：{{ row.invoice_status || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="actions-inline">
                  <button class="link-button" type="button" @click="openSalesOrderDrawer(row)">销售单</button>
                  <button class="link-button" type="button" @click="openDeliveryNoteDrawer(row)" :disabled="!isShipped(row)">出库单</button>
                </div>
              </td>
            </tr>
            <tr v-if="!fulfillmentOrders.length">
              <td colspan="8" class="muted empty-row">{{ fulfillmentOrdersLoading ? '加载中...' : '暂无履约订单' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <PaginationControls
        :page="fulfillmentOrdersPage"
        :page-size="fulfillmentOrdersLimit"
        :total="Number(fulfillmentOrdersSummary.orders || 0)"
        :disabled="fulfillmentOrdersLoading"
        @change="handleFulfillmentOrdersPagination"
      />
    </section>

    <div v-if="orderDetailDrawerOpen" class="order-detail-drawer-mask" @click.self="closeOrderDetailDrawer">
      <aside class="order-detail-drawer" aria-label="履约订单详情">
        <div class="drawer-head">
          <div>
            <h3>{{ activeOrderSummary?.order_no || '订单详情' }}</h3>
            <p>{{ activeOrderSummary?.customer || overview.customer_name || '-' }} · {{ activeOrderSummary?.order_date || '-' }}</p>
          </div>
          <button class="secondary" type="button" @click="closeOrderDetailDrawer">关闭</button>
        </div>

        <div v-if="orderDetailError" class="error">{{ orderDetailError }}</div>

        <div v-if="activeOrderSummary" class="drawer-body">
          <section class="drawer-section">
            <h4>收件与快递</h4>
            <div class="detail-grid">
              <span>收件人：{{ activeOrderSummary.receiver_name || '-' }}</span>
              <span>电话：{{ activeOrderSummary.receiver_phone || '-' }}</span>
              <span class="wide-item">地址：{{ activeOrderSummary.receiver_address || '-' }}</span>
              <span>寄件人：{{ activeOrderSummary.sender_label || activeOrderSummary.sender_name || '-' }}</span>
              <span>运单号：{{ activeOrderSummary.ship_tracking_no || '-' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <h4>订单费用</h4>
            <div class="detail-grid">
              <span>商品金额：{{ activeOrderDetail?.total_amount || activeOrderSummary.total_amount || '0.00' }}</span>
              <span>运费：{{ activeOrderDetail?.shipping_amount || activeOrderSummary.shipping_amount || '0.00' }}</span>
              <span>优惠：{{ activeOrderDetail?.discount_amount || activeOrderSummary.discount_amount || '0.00' }}</span>
              <span>应收：{{ activeOrderDetail?.grand_total || activeOrderSummary.grand_total || '0.00' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <h4>订单信息</h4>
            <div class="detail-grid">
              <span>类型：{{ activeOrderSummary.order_type || '-' }}</span>
              <span>来源：{{ activeOrderSummary.portal_service_code || '-' }}</span>
              <span>收款：{{ activeOrderSummary.pay_status || '-' }}</span>
              <span>发货：{{ activeOrderSummary.ship_status || '-' }}</span>
              <span>生产：{{ activeOrderSummary.process_status || '-' }}</span>
              <span>发票：{{ activeOrderSummary.invoice_status || '-' }}</span>
              <span class="wide-item">备注：{{ activeOrderDetail?.notes || activeOrderSummary.notes || '-' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <div class="drawer-actions">
              <button class="secondary" type="button" @click="openSalesOrderDrawer(activeOrderSummary)">销售单</button>
              <button class="secondary" type="button" @click="openDeliveryNoteDrawer(activeOrderSummary)" :disabled="!isShipped(activeOrderSummary)">出库单</button>
            </div>
          </section>

          <section class="drawer-section">
            <h4>商品明细</h4>
            <div v-if="orderDetailLoading" class="muted">订单明细加载中...</div>
            <div v-else-if="!activeOrderDetail?.items?.length" class="muted">暂无商品明细</div>
            <div v-else class="table-wrap drawer-table-wrap">
              <table class="drawer-table">
                <thead>
                  <tr><th>商品</th><th>规格</th><th>数量</th><th>单价</th><th>小计</th><th>豆单版本</th><th>备注</th></tr>
                </thead>
                <tbody>
                  <tr v-for="(item, idx) in activeOrderDetail?.items || []" :key="`${activeOrderSummary.id}-${idx}`">
                    <td>{{ item.product_name || '-' }}</td>
                    <td>{{ item.spec || '-' }}</td>
                    <td>{{ item.qty || '-' }}{{ item.unit || '' }}</td>
                    <td>{{ item.unit_price || '-' }}</td>
                    <td>{{ item.line_total || '-' }}</td>
                    <td>{{ item.bean_list_version_no || '未记录' }}</td>
                    <td>{{ item.note || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </aside>
    </div>

    <div v-if="salesOrderDrawerOpen" class="sales-order-drawer-mask" @click.self="closeSalesOrderDrawer">
      <aside class="sales-order-drawer" aria-label="销售单">
        <SalesOrderView :order-id="activeSalesOrderID" embedded @close="closeSalesOrderDrawer" />
      </aside>
    </div>

    <div v-if="deliveryNoteDrawerOpen" class="delivery-note-drawer-mask" @click.self="closeDeliveryNoteDrawer">
      <aside class="delivery-note-drawer" aria-label="出库单">
        <DeliveryNoteView :order-id="activeDeliveryNoteID" embedded @close="closeDeliveryNoteDrawer" />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import PaginationControls from '../components/PaginationControls.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import DeliveryNoteView from './DeliveryNoteView.vue'
import SalesOrderView from './SalesOrderView.vue'
import { apiGet } from '../api/client'
import {
  fetchCustomerFulfillmentOrderDetail,
  fetchCustomerFulfillmentOrders,
  fetchCustomerProcessingPortalOverview,
  fetchCustomerProcessingPortalOptions,
  fetchInternalCustomerProcessingPortalOverview,
  fetchInternalCustomerProcessingPortalOptions,
  submitCustomerDirectShipOrder,
  submitCustomerProcessingWorkOrder,
} from '../api/customer-fulfillment'
import { customerFulfillmentOrderFees, customerFulfillmentSubmitCopy } from '../lib/customer-fulfillment'
import { lineTotal, syncWholesaleTierPrice, toInt, wholesalePriceUnit, wholesaleTierPriceRows } from '../lib/order-entry'
import { parseRecipientText } from '../lib/customer-recipient'
import { normalizePageSize } from '../lib/pagination'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
  customerAccountActor: { type: Boolean, default: false },
})

const internalCustomerID = computed(() => {
  if (adminSelectedCustomerId.value > 0) return adminSelectedCustomerId.value
  if (props.customerAccountActor) return 0
  const fromProps = Number(props.customerContextId || 0)
  if (fromProps > 0) return fromProps
  const fromParams = Number(props.viewParams?.customer_id || 0)
  return fromParams > 0 ? fromParams : 0
})
const isInternalContext = computed(() => internalCustomerID.value > 0)

// Admin customer picker
const adminCustomerValue = ref('')
const adminSelectedCustomerId = ref(0)
const adminCustomerOptions = ref([])
const adminCustomersLoaded = ref(false)

async function fetchAdminCustomers() {
  if (adminCustomersLoaded.value) return
  try {
    const data = await apiGet('/api/customers?limit=200&active=true')
    adminCustomerOptions.value = (data?.rows || []).filter(row => row.active !== false)
    adminCustomersLoaded.value = true
  } catch {
    // Silently ignore - customers can still use workspace mode
  }
}

function selectAdminCustomer(option) {
  const customerID = Number(option?.id || 0)
  adminSelectedCustomerId.value = customerID
  if (customerID > 0) {
    loadOverview()
  }
}

function adminCustomerOptionLabel(customer) {
  return customer?.name || ''
}

function adminCustomerOptionMeta(customer) {
  const parts = []
  if (customer?.company_name && customer.company_name !== customer?.name) parts.push(customer.company_name)
  if (customer?.contact) parts.push(customer.contact)
  if (customer?.phone || customer?.company_phone) parts.push(customer.phone || customer.company_phone)
  return parts.join(' / ')
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

const loading = ref(false)
const error = ref('')
const ok = ref('')
const overview = ref({})
const fulfillmentOptions = ref({})
const fulfillmentOrders = ref([])
const fulfillmentOrdersSummary = ref({})
const fulfillmentOrdersPage = ref(1)
const fulfillmentOrdersLimit = ref(10)
const fulfillmentOrdersHasPrev = ref(false)
const fulfillmentOrdersHasNext = ref(false)
const fulfillmentOrdersLoading = ref(false)
const orderDetailDrawerOpen = ref(false)
const activeOrderSummary = ref(null)
const activeOrderDetail = ref(null)
const orderDetailLoading = ref(false)
const orderDetailError = ref('')
const salesOrderDrawerOpen = ref(false)
const activeSalesOrderID = ref(0)
const deliveryNoteDrawerOpen = ref(false)
const activeDeliveryNoteID = ref(0)
const processingProductValue = ref('')
const processingRawBeanValue = ref('')
const recipientHistoryValue = ref('')
const recipientPasteText = ref('')
const directShipItems = ref([newDirectShipItem()])
const processingForm = reactive({
  product_id: 0,
  product_name: '',
  raw_bean_item_id: 0,
  raw_bean_name: '',
  input_quantity_g: '',
  planned_output_units: '',
  expected_date: '',
  note: '',
})
const directShipForm = reactive({
  receiver_name: '',
  receiver_phone: '',
  receiver_address: '',
  note: '',
})

const capabilities = computed(() => overview.value.capabilities || [])
const hasScopedCapabilities = computed(() => capabilities.value.length > 0)
const overviewCustomerId = computed(() => Number(overview.value?.customer_id || 0))
const canSubmitProcessing = computed(() => hasCapability('processing'))
const canDirectShip = computed(() => hasCapability('direct_ship') || hasCapability('product_order'))
const submitCopy = computed(() => customerFulfillmentSubmitCopy(capabilities.value))
const canViewInventory = computed(() => hasCapability('inventory_custody') || hasCapability('processing'))
const customerSKUOptions = computed(() => fulfillmentOptions.value?.customer_skus || [])
const custodyItemOptions = computed(() => fulfillmentOptions.value?.custody_items || [])
const rawBeanOptions = computed(() => custodyItemOptions.value.filter((row) => row.item_type === 'raw_bean'))
const recipientOptions = computed(() => fulfillmentOptions.value?.recipients || [])
const finishedGoodsProductOptions = computed(() => (overview.value?.finished_goods || []).map((row) => ({
  product_id: row.product_id,
  product_name: row.product_name,
  spec: row.spec_g ? `${row.spec_g}g` : '',
  warehouse: row.warehouse,
  quantity_units: row.quantity_units,
  quantity_g: row.quantity_g,
  source: 'finished_goods',
})))
const directShipProductOptions = computed(() => uniqueProductOptions([
  ...customerSKUOptions.value,
  ...finishedGoodsProductOptions.value,
]))
const directShipItemsTotal = computed(() => directShipItems.value.reduce((sum, row) => sum + rowLineTotal(row), 0))
const directShipGrandTotal = computed(() => directShipItemsTotal.value)
const metrics = computed(() => [
  canViewInventory.value ? { label: '原料库存', value: (overview.value.custody_balances || []).length } : null,
  canSubmitProcessing.value ? { label: '加工工单', value: (overview.value.processing_orders || []).length } : null,
  canViewInventory.value ? { label: '成品库存', value: (overview.value.finished_goods || []).length } : null,
  canDirectShip.value ? { label: '履约订单', value: fulfillmentOrdersSummary.value.orders || fulfillmentOrders.value.length } : null,
  canDirectShip.value ? { label: '待结算金额', value: money(fulfillmentOrdersSummary.value.pending_settlement_amount || 0) } : null,
].filter(Boolean))

onMounted(() => {
  fetchAdminCustomers()
  loadOverview()
})

async function loadOverview() {
  loading.value = true
  fulfillmentOrdersLoading.value = true
  error.value = ''
  try {
    const [overviewData, optionsData] = await Promise.all([
      isInternalContext.value
        ? fetchInternalCustomerProcessingPortalOverview(internalCustomerID.value)
        : fetchCustomerProcessingPortalOverview(),
      isInternalContext.value
        ? fetchInternalCustomerProcessingPortalOptions(internalCustomerID.value)
        : fetchCustomerProcessingPortalOptions(),
    ])
    overview.value = overviewData || {}
    fulfillmentOptions.value = optionsData || {}
    fulfillmentOrdersPage.value = 1
    if (overviewCustomerId.value) {
      assignFulfillmentOrders(await loadFulfillmentOrdersData(overviewCustomerId.value, fulfillmentOrdersPage.value))
    } else {
      assignFulfillmentOrders({})
    }
  } catch (err) {
    error.value = err.message || '加载代加工数据失败'
  } finally {
    loading.value = false
    fulfillmentOrdersLoading.value = false
  }
}

async function submitProcessing() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await submitCustomerProcessingWorkOrder({
      product_id: Number(processingForm.product_id || 0),
      product_name: processingForm.product_name,
      raw_bean_item_id: Number(processingForm.raw_bean_item_id || 0),
      raw_bean_name: processingForm.raw_bean_name,
      input_quantity_g: Number(processingForm.input_quantity_g || 0),
      planned_output_units: Number(processingForm.planned_output_units || 0),
      expected_date: processingForm.expected_date,
      note: processingForm.note,
    })
    ok.value = `已提交工单 ${row.work_order_no || ''}`
    processingProductValue.value = ''
    processingRawBeanValue.value = ''
    processingForm.product_id = 0
    processingForm.product_name = ''
    processingForm.raw_bean_item_id = 0
    processingForm.raw_bean_name = ''
    processingForm.input_quantity_g = ''
    processingForm.planned_output_units = ''
    processingForm.expected_date = ''
    processingForm.note = ''
    await loadOverview()
  } catch (err) {
    error.value = err.message || '提交工单失败'
  } finally {
    loading.value = false
  }
}

async function submitDirectShip() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const items = directShipItems.value
      .map((row) => ({
        product_id: Number(row.product_id || 0),
        customer_product_alias_id: Number(row.customer_product_alias_id || 0),
        customer_product_display_name_snapshot: String(row.customer_product_display_name || row.product_name || '').trim(),
        customer_item_code_snapshot: String(row.customer_item_code || '').trim(),
        product_code_snapshot: String(row.product_code || '').trim(),
        product_name_snapshot: String(row.product_record_name || '').trim(),
        product_name: String(row.product_name || '').trim(),
        spec: row.spec_g ? `${Number(row.spec_g)}g` : '',
        spec_g: Number(row.spec_g || 0),
        sales_unit: String(row.sales_unit || '').trim(),
        quantity_units: Number(row.qty || 0),
      }))
      .filter((row) => row.product_id > 0 && row.spec_g > 0 && row.quantity_units > 0)
    const row = await submitCustomerDirectShipOrder({
      receiver_name: directShipForm.receiver_name,
      receiver_phone: directShipForm.receiver_phone,
      receiver_address: directShipForm.receiver_address,
      items,
      note: directShipForm.note,
    })
    ok.value = `${submitCopy.value.successPrefix} ${row.order_no || ''}`
    recipientHistoryValue.value = ''
    recipientPasteText.value = ''
    directShipForm.receiver_name = ''
    directShipForm.receiver_phone = ''
    directShipForm.receiver_address = ''
    directShipForm.note = ''
    directShipItems.value = [newDirectShipItem()]
    fulfillmentOrdersPage.value = 1
    await loadOverview()
  } catch (err) {
    error.value = err.message || submitCopy.value.errorFallback
  } finally {
    loading.value = false
  }
}

async function loadFulfillmentOrders(page = fulfillmentOrdersPage.value) {
  if (!overviewCustomerId.value) return
  fulfillmentOrdersLoading.value = true
  try {
    const data = await loadFulfillmentOrdersData(overviewCustomerId.value, page)
    assignFulfillmentOrders(data)
  } catch (err) {
    error.value = err.message || '加载履约客户订单失败'
  } finally {
    fulfillmentOrdersLoading.value = false
  }
}

function loadFulfillmentOrdersData(customerId, page = 1) {
  return fetchCustomerFulfillmentOrders(customerId, { page, limit: fulfillmentOrdersLimit.value })
}

function handleFulfillmentOrdersPagination({ page, pageSize }) {
  fulfillmentOrdersLimit.value = normalizePageSize(pageSize)
  loadFulfillmentOrders(page)
}

function assignFulfillmentOrders(data = {}) {
  fulfillmentOrders.value = Array.isArray(data?.rows) ? data.rows : []
  fulfillmentOrdersSummary.value = data?.summary || {}
  fulfillmentOrdersPage.value = Number(data?.page || fulfillmentOrdersPage.value || 1)
  fulfillmentOrdersLimit.value = normalizePageSize(data?.limit || fulfillmentOrdersLimit.value)
  fulfillmentOrdersHasPrev.value = Boolean(data?.has_prev)
  fulfillmentOrdersHasNext.value = Boolean(data?.has_next)
  const currentID = Number(activeOrderSummary.value?.id || 0)
  if (currentID > 0) {
    const refreshed = fulfillmentOrders.value.find((row) => Number(row.id) === currentID)
    if (refreshed) activeOrderSummary.value = { ...refreshed }
  }
}

async function openFulfillmentOrderDetail(row) {
  const orderId = Number(row?.id || 0)
  if (!orderId) return
  activeOrderSummary.value = { ...row }
  activeOrderDetail.value = null
  orderDetailError.value = ''
  orderDetailDrawerOpen.value = true
  orderDetailLoading.value = true
  try {
    const data = await fetchCustomerFulfillmentOrderDetail(orderId)
    activeOrderDetail.value = data?.edit_data || null
  } catch (err) {
    orderDetailError.value = err.message || '加载订单明细失败'
  } finally {
    orderDetailLoading.value = false
  }
}

function closeOrderDetailDrawer() {
  orderDetailDrawerOpen.value = false
  activeOrderSummary.value = null
  activeOrderDetail.value = null
  orderDetailError.value = ''
  orderDetailLoading.value = false
}

function openSalesOrderDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeSalesOrderID.value = id
  salesOrderDrawerOpen.value = true
}

function closeSalesOrderDrawer() {
  salesOrderDrawerOpen.value = false
  activeSalesOrderID.value = 0
}

function openDeliveryNoteDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeDeliveryNoteID.value = id
  deliveryNoteDrawerOpen.value = true
}

function closeDeliveryNoteDrawer() {
  deliveryNoteDrawerOpen.value = false
  activeDeliveryNoteID.value = 0
}

function hasCapability(code) {
  if (!hasScopedCapabilities.value) return true
  return capabilities.value.includes(code)
}

function selectProcessingProduct(option) {
  processingForm.product_id = Number(option?.product_id || 0)
  processingForm.product_name = String(option?.product_name || '').trim()
}

function selectProcessingRawBean(option) {
  processingForm.raw_bean_item_id = Number(option?.item_id || 0)
  processingForm.raw_bean_name = String(option?.item_name || '').trim()
}

function selectDirectShipItemProduct(row, option) {
  row.product_id = Number(option?.product_id || 0)
  row.customer_product_alias_id = Number(option?.customer_product_alias_id || 0)
  row.customer_product_display_name = String(option?.customer_product_display_name || option?.product_name || '').trim()
  row.customer_item_code = String(option?.customer_item_code || option?.sku_code || '').trim()
  row.product_code = String(option?.product_code || '').trim()
  row.product_record_name = String(option?.product_record_name || '').trim()
  row.product_name = String(option?.product_name || '').trim()
  row.product_value = productOptionValue(option)
  row.spec_g = parseSpecG(option?.spec) || firstTierSpecG(option) || 454
  row.sales_unit = String(Array.isArray(option?.sales_units) ? option.sales_units[0] || '' : option?.sales_unit || '').trim()
  if (!toInt(row.qty)) row.qty = 1
  syncDirectShipItemPrice(row)
  ensureSingleTrailingEmptyRow()
}

function pasteRecipientInfo(event) {
  const text = event?.clipboardData?.getData('text') || ''
  recipientPasteText.value = text
  applyRecipientParse(text)
}

function applyRecipientParse(text = recipientPasteText.value) {
  applyRecipientFields(parseRecipientText(text))
}

function selectRecipientHistory(option) {
  const snapshot = [option?.receiver_name, option?.receiver_phone, option?.receiver_address].filter(Boolean).join(' ')
  recipientPasteText.value = snapshot
  const parsed = parseRecipientText(snapshot)
  applyRecipientFields({
    recipient_name: option?.receiver_name || parsed.recipient_name,
    phone: option?.receiver_phone || parsed.phone,
    address: option?.receiver_address || parsed.address,
  })
}

function applyRecipientFields(parsed) {
  if (parsed?.recipient_name) directShipForm.receiver_name = parsed.recipient_name
  if (parsed?.phone) directShipForm.receiver_phone = parsed.phone
  if (parsed?.address) directShipForm.receiver_address = parsed.address
}

function productOptionLabel(option) {
  return option?.product_name || ''
}

function productOptionMeta(option) {
  return [
    productSourceLabel(option?.source),
    option?.sku_code,
    option?.spec,
    option?.roast_degree,
    option?.warehouse,
    option?.quantity_units ? `${option.quantity_units}件` : '',
  ].filter(Boolean).join(' / ')
}

function productOptionValue(option) {
  const aliasID = Number(option?.customer_product_alias_id || 0)
  if (aliasID > 0) return `alias:${aliasID}:${option?.spec || ''}:${option?.warehouse || ''}:${option?.source || ''}`
  if (Number(option?.product_id || 0) > 0) {
    return `product:${option.product_id}:${option?.spec || ''}:${option?.warehouse || ''}:${option?.source || ''}`
  }
  return `product:${option?.product_name || ''}:${option?.spec || ''}`
}

function custodyOptionLabel(option) {
  return option?.item_name || ''
}

function custodyOptionMeta(option) {
  return [
    custodyTypeLabel(option?.item_type),
    option?.spec,
    option?.quantity_g ? `${option.quantity_g}g` : '',
    option?.quantity_units ? `${option.quantity_units}件` : '',
  ].filter(Boolean).join(' / ')
}

function custodyOptionValue(option) {
  return `custody:${option?.item_type || ''}:${option?.item_id || option?.item_name || ''}:${option?.spec || ''}`
}

function recipientOptionLabel(option) {
  return [option?.receiver_name, option?.receiver_phone].filter(Boolean).join(' ') || option?.receiver_address || ''
}

function recipientOptionMeta(option) {
  return [option?.receiver_address, option?.last_order_no, option?.last_used_at].filter(Boolean).join(' / ')
}

function recipientOptionValue(option) {
  return [option?.receiver_phone, option?.receiver_address, option?.last_order_no].filter(Boolean).join('|')
}

function uniqueProductOptions(rows) {
  const out = []
  const seen = new Set()
  for (const row of rows || []) {
    const name = String(row?.product_name || '').trim()
    if (!name) continue
    const normalized = { ...row, product_name: name, spec: String(row?.spec || '').trim() }
    const aliasID = Number(normalized.customer_product_alias_id || 0)
    const key = aliasID > 0
      ? `alias:${aliasID}|${normalized.spec}|${normalized.warehouse || ''}|${normalized.source || ''}`
      : `${normalized.product_id || 0}|${normalized.product_name}|${normalized.spec}|${normalized.warehouse || ''}|${normalized.source || ''}`
    if (seen.has(key)) continue
    seen.add(key)
    out.push(normalized)
  }
  return out
}

function newDirectShipItem() {
  return {
    key: `${Date.now()}-${Math.random()}`,
    product_id: 0,
    customer_product_alias_id: 0,
    customer_product_display_name: '',
    customer_item_code: '',
    product_code: '',
    product_record_name: '',
    product_name: '',
    product_value: '',
    spec_g: 454,
    sales_unit: '',
    qty: 1,
    tier_id: 'auto',
    unit_price: '',
  }
}

function removeDirectShipItem(idx) {
  if (directShipItems.value.length <= 1) return
  directShipItems.value.splice(idx, 1)
  ensureSingleTrailingEmptyRow()
}

function productByID(id) {
  return directShipProductOptions.value.find((item) => Number(item.product_id || 0) === Number(id)) || null
}

function productForDirectShipRow(row) {
  const aliasID = Number(row?.customer_product_alias_id || 0)
  if (aliasID > 0) {
    const byAlias = directShipProductOptions.value.find((item) => Number(item.customer_product_alias_id || 0) === aliasID)
    if (byAlias) return byAlias
  }
  return productByID(row?.product_id)
}

function syncDirectShipItemPrice(row) {
  const product = productForDirectShipRow(row)
  if (!product) {
    row.tier_id = 'auto'
    row.unit_price = ''
    return
  }
  const price = syncWholesaleTierPrice(product, row)
  row.tier_id = price.tierID
  row.unit_price = price.unitPrice
}

function rowLineTotal(row) {
  const base = lineTotal(productForDirectShipRow(row), row, false)
  return base > 0 ? base : 0
}

function priceUnitLabel(row) {
  return wholesalePriceUnit(row).label
}

function rowTierRows(row) {
  return wholesaleTierPriceRows(productForDirectShipRow(row), row)
}

function rowTierActive(row, tier) {
  return String(row?.tier_id || '') === String(tier?.id || '')
}

function isDirectShipItemFilled(row) {
  return Number(row?.product_id || 0) > 0
}

function ensureSingleTrailingEmptyRow() {
  const filled = directShipItems.value.filter((row) => isDirectShipItemFilled(row))
  const firstEmpty = directShipItems.value.find((row) => !isDirectShipItemFilled(row))
  directShipItems.value = [...filled, firstEmpty || newDirectShipItem()]
}

function parseSpecG(spec) {
  const text = String(spec || '').trim().toLowerCase().replace(/g$/, '')
  const n = Number.parseInt(text, 10)
  return Number.isFinite(n) && n > 0 ? n : 0
}

function firstTierSpecG(option) {
  const first = Array.isArray(option?.tiers) ? option.tiers.find((tier) => Number(tier?.spec_g || 0) > 0) : null
  return Number(first?.spec_g || 0)
}

function custodyTypeLabel(value) {
  return { raw_bean: '生豆', packaging: '包材', product: '成品' }[value] || value || '-'
}

function productSourceLabel(value) {
  return {
    public_sku: '公共SKU',
    public_sku_alias: '客户专属名',
    customer_sku_import: '客户SKU',
    customer_product: '客户商品',
    finished_goods: '成品库存',
  }[value] || ''
}

function statusLabel(value) {
  return {
    submitted: '已提交',
    accepted: '已受理',
    planned: '已排产',
    running: '生产中',
    finished: '已完成',
    settled: '已结算',
    draft: '草稿',
  }[value] || value || '-'
}

function orderFeeLines(row) {
  return customerFulfillmentOrderFees(row)
}

function isShipped(row) {
  return String(row?.ship_status || '').includes('已发货')
}

function formatG(value) {
  const n = Number(value || 0)
  if (!n) return '0'
  if (Math.abs(n) >= 1000) return `${(n / 1000).toFixed(2)} kg`
  return `${n} g`
}

function money(value) {
  return Number(value || 0).toFixed(2)
}
</script>

<style scoped>
.customer-processing-portal {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.portal-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 16px;
  background: #fff;
}

.portal-head h2,
.portal-head p,
.panel h3 {
  margin: 0;
}

.portal-head p {
  margin-top: 4px;
  color: #64748b;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.metric,
.panel {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  background: #fff;
}

.metric {
  padding: 12px;
}

.metric span {
  display: block;
  color: #64748b;
  font-size: 13px;
}

.metric strong {
  display: block;
  margin-top: 6px;
  font-size: 22px;
}

.form-grid,
.grid-2 {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 14px;
}

.panel {
  padding: 14px;
  min-width: 0;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.wide {
  grid-column: span 2;
}

label {
  display: grid;
  gap: 4px;
  color: #475569;
  font-size: 13px;
}

input,
textarea {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 8px;
  font: inherit;
}

textarea {
  resize: vertical;
}

button {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 12px;
  background: #fff;
  cursor: pointer;
}

button:disabled {
  cursor: not-allowed;
  opacity: .6;
}

.primary {
  border-color: #0f766e;
  background: #0f766e;
  color: #fff;
}

.secondary {
  background: #f8fafc;
}

.direct-ship-form {
  display: grid;
  gap: 12px;
}

.direct-ship-recipient {
  display: grid;
  grid-template-columns: minmax(220px, 1.4fr) auto repeat(3, minmax(130px, 1fr));
  gap: 10px;
  align-items: end;
}

.recipient-paste {
  grid-column: 1 / span 1;
}

.recipient-address {
  grid-column: 1 / -1;
}

.parse-button {
  min-width: 120px;
}

.direct-ship-items {
  display: grid;
  gap: 8px;
  position: relative;
  z-index: 5;
  overflow: visible;
}

.direct-ship-items-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.order-lines-table {
  min-width: 920px;
  table-layout: fixed;
}

.order-lines-table th:nth-child(1) { width: 260px; }
.order-lines-table th:nth-child(2) { width: 100px; }
.order-lines-table th:nth-child(3) { width: 90px; }
.order-lines-table th:nth-child(4) { width: 130px; }
.order-lines-table th:nth-child(5) { width: 100px; }
.order-lines-table th:nth-child(6) { width: 240px; }
.order-lines-table th:nth-child(7) { width: 80px; }

.order-lines-table td {
  vertical-align: top;
}

.order-lines-table td :deep(.searchable-select) {
  width: 100%;
}

.order-lines-table td input {
  width: 100%;
}

.price-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.price-cell small {
  color: #64748b;
  font-size: 12px;
  white-space: nowrap;
}

.line-total {
  display: grid;
  gap: 2px;
  color: #475569;
  font-size: 12px;
}

.line-total strong {
  color: #0f172a;
  font-size: 14px;
}

.subtotal-cell {
  font-weight: 700;
  color: #0f172a;
}

.tier-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tier-chip {
  border: 1px solid #cbd5e1;
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 12px;
  color: #475569;
  background: #fff;
}

.tier-chip.active {
  border-color: #0f766e;
  color: #0f766e;
  font-weight: 600;
}

.direct-ship-footer {
  display: grid;
  grid-template-columns: minmax(120px, 180px) minmax(280px, 1fr);
  gap: 12px;
  align-items: end;
}

.grand-total {
  align-self: center;
}

.note-field {
  min-width: 0;
}

.danger {
  border-color: #fecaca;
  color: #b91c1c;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

th,
td {
  border-bottom: 1px solid #e2e8f0;
  padding: 8px;
  text-align: left;
  vertical-align: top;
}

th {
  color: #475569;
  background: #f8fafc;
}

.muted {
  color: #64748b;
}

.error,
.ok {
  padding: 8px 10px;
  border-radius: 6px;
}

.error {
  background: #fef2f2;
  color: #b91c1c;
}

.ok {
  background: #ecfdf5;
  color: #047857;
}

.head-actions,
.actions-inline,
.drawer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.table-wrap {
  width: 100%;
  overflow: visible;
}

.fulfillment-orders-panel {
  padding: 0;
  overflow: hidden;
}

.fulfillment-orders-panel .panel-head {
  margin: 0;
  padding: 14px;
  border-bottom: 1px solid #e2e8f0;
}

.fulfillment-orders-panel .panel-head p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.fulfillment-orders-table {
  min-width: 1080px;
  table-layout: fixed;
}

.fulfillment-orders-table th:nth-child(1) { width: 150px; }
.fulfillment-orders-table th:nth-child(2) { width: 96px; }
.fulfillment-orders-table th:nth-child(3) { width: 150px; }
.fulfillment-orders-table th:nth-child(4) { width: 250px; }
.fulfillment-orders-table th:nth-child(5) { width: 150px; }
.fulfillment-orders-table th:nth-child(6) { width: 140px; }
.fulfillment-orders-table th:nth-child(7) { width: 150px; }
.fulfillment-orders-table th:nth-child(8) { width: 110px; }

.stacked-text,
.status-stack,
.fee-stack {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.stacked-text strong,
.stacked-text span,
.status-stack span,
.receiver-cell span {
  min-width: 0;
  overflow-wrap: anywhere;
}

.cell-meta {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.order-link,
.link-button {
  min-height: auto;
  border: 0;
  padding: 0;
  background: transparent;
  color: #0f766e;
  font: inherit;
  text-align: left;
}

.fee-line {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.fee-line span {
  color: #64748b;
}

.fee-line.emphasized strong {
  color: #0f766e;
}

.empty-row {
  text-align: center;
}

.order-detail-drawer-mask,
.sales-order-drawer-mask,
.delivery-note-drawer-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  justify-content: flex-end;
  background: rgba(15, 23, 42, .36);
}

.order-detail-drawer,
.sales-order-drawer,
.delivery-note-drawer {
  width: min(920px, 96vw);
  height: 100%;
  overflow: auto;
  background: #fff;
  box-shadow: -16px 0 32px rgba(15, 23, 42, .18);
}

.drawer-head {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
  border-bottom: 1px solid #e2e8f0;
  background: #fff;
}

.drawer-head h3,
.drawer-head p,
.drawer-section h4 {
  margin: 0;
}

.drawer-head p {
  margin-top: 4px;
  color: #64748b;
}

.drawer-body {
  display: grid;
  gap: 14px;
  padding: 14px;
}

.drawer-section {
  display: grid;
  gap: 10px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 12px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
  color: #334155;
}

.detail-grid span {
  overflow-wrap: anywhere;
}

.wide-item {
  grid-column: 1 / -1;
}

.drawer-table {
  min-width: 720px;
}

@media (max-width: 720px) {
  .portal-head {
    align-items: stretch;
    flex-direction: column;
  }

  .form-grid,
  .grid-2 {
    grid-template-columns: 1fr;
  }

  .wide {
    grid-column: auto;
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }

  .direct-ship-recipient,
  .direct-ship-footer {
    grid-template-columns: 1fr;
  }

  .recipient-paste,
  .recipient-address {
    grid-column: auto;
  }

  .direct-ship-items-head {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
