<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  createDirectShipBatch,
  createProcessingRequest,
  fetchServicePage,
  type ServicePageResponse,
} from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { normalizeServiceKey, serviceTitle, visibleServiceSections, type ServiceKey } from '../../utils/servicePage'

const session = useSessionStore()
const serviceKey = ref<ServiceKey>('beanList')
const page = ref<ServicePageResponse | null>(null)
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')

const directShipForm = ref({ source_name: '', total_rows: 0, note: '' })
const processingForm = ref({
  input_material_id: 0,
  input_qty_g: 0,
  target_product_id: 0,
  target_spec_g: 454,
  target_qty: 1,
  note: '',
})

const title = computed(() => page.value?.title || serviceTitle(serviceKey.value))
const summary = computed(() => page.value?.summary || [])
const sections = computed(() => (page.value ? visibleServiceSections(page.value) : []))

async function loadPage() {
  if (!session.token) {
    uni.redirectTo({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    page.value = await fetchServicePage(session.token, serviceKey.value)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '服务数据加载失败'
  } finally {
    loading.value = false
  }
}

async function submitDirectShipBatch() {
  if (!directShipForm.value.source_name.trim()) {
    errorMessage.value = '请填写批次名称'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await createDirectShipBatch(session.token, {
      source_name: directShipForm.value.source_name,
      total_rows: Number(directShipForm.value.total_rows) || 0,
      note: directShipForm.value.note,
    })
    directShipForm.value = { source_name: '', total_rows: 0, note: '' }
    uni.showToast({ title: '已提交', icon: 'success' })
    await loadPage()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

async function submitProcessingRequest() {
  const payload = {
    input_material_id: Number(processingForm.value.input_material_id) || 0,
    input_qty_g: Number(processingForm.value.input_qty_g) || 0,
    target_product_id: Number(processingForm.value.target_product_id) || 0,
    target_spec_g: Number(processingForm.value.target_spec_g) || 0,
    target_qty: Number(processingForm.value.target_qty) || 0,
    note: processingForm.value.note,
  }
  if (!payload.input_material_id || !payload.input_qty_g || !payload.target_product_id || !payload.target_spec_g || !payload.target_qty) {
    errorMessage.value = '请填写完整加工信息'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await createProcessingRequest(session.token, payload)
    processingForm.value = {
      input_material_id: 0,
      input_qty_g: 0,
      target_product_id: 0,
      target_spec_g: 454,
      target_qty: 1,
      note: '',
    }
    uni.showToast({ title: '已提交', icon: 'success' })
    await loadPage()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

onLoad((query) => {
  serviceKey.value = normalizeServiceKey(String(query?.key || 'beanList'))
})

onShow(() => {
  void loadPage()
})
</script>

<template>
  <view class="page">
    <view class="header">
      <text class="eyebrow">服务入口</text>
      <text class="title">{{ title }}</text>
      <text class="subtitle">{{ page?.current_customer_name || session.currentCustomerName || '客户中心' }}</text>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else>
      <view v-if="errorMessage" class="state error">
        <text>{{ errorMessage }}</text>
      </view>

      <view v-if="summary.length" class="metrics">
        <view v-for="item in summary" :key="item.label" class="metric">
          <text class="metric-value">{{ item.value }}</text>
          <text class="metric-label">{{ item.label }}</text>
        </view>
      </view>

      <view v-if="serviceKey === 'directShip'" class="panel">
        <text class="panel-title">新建代发批次</text>
        <input v-model="directShipForm.source_name" class="input" placeholder="批次名称，例如 5月直播订单" />
        <input v-model.number="directShipForm.total_rows" class="input" type="number" placeholder="订单行数" />
        <textarea v-model="directShipForm.note" class="textarea" placeholder="备注" />
        <button class="primary" :disabled="submitting" @tap="submitDirectShipBatch">提交批次</button>
      </view>

      <view v-if="serviceKey === 'processing'" class="panel">
        <text class="panel-title">提交加工申请</text>
        <input v-model.number="processingForm.input_material_id" class="input" type="number" placeholder="生豆物料ID" />
        <input v-model.number="processingForm.input_qty_g" class="input" type="number" placeholder="投入生豆克重" />
        <input v-model.number="processingForm.target_product_id" class="input" type="number" placeholder="目标产品ID" />
        <input v-model.number="processingForm.target_spec_g" class="input" type="number" placeholder="规格克重" />
        <input v-model.number="processingForm.target_qty" class="input" type="number" placeholder="目标件数" />
        <textarea v-model="processingForm.note" class="textarea" placeholder="加工要求" />
        <button class="primary" :disabled="submitting" @tap="submitProcessingRequest">提交申请</button>
      </view>

      <view v-if="sections.length" class="section-list">
        <view v-for="section in sections" :key="section.title" class="section-row">
          <text class="section-title">{{ section.title }}</text>
          <text class="section-count">{{ section.count }}</text>
        </view>
      </view>

      <view v-if="page?.bean_lists?.length" class="panel">
        <text class="panel-title">豆单</text>
        <view v-for="item in page.bean_lists" :key="item.id" class="list-row">
          <text class="row-main">{{ item.list_type }} {{ item.version_no }}</text>
          <text class="row-sub">{{ item.published_at }}</text>
        </view>
      </view>

      <view v-if="page?.products?.length" class="panel">
        <text class="panel-title">现货商品</text>
        <view v-for="item in page.products" :key="item.id" class="list-row">
          <text class="row-main">{{ item.name }}</text>
          <text class="row-sub">{{ item.roast_level }} / 默认 ¥{{ item.default_price }}</text>
        </view>
      </view>

      <view v-if="page?.orders?.length" class="panel">
        <text class="panel-title">订单 / 物流</text>
        <view v-for="item in page.orders" :key="item.id" class="list-row">
          <view class="row-head">
            <text class="row-main">{{ item.order_no || '未编号订单' }}</text>
            <text class="price">¥{{ item.grand_total || '0.00' }}</text>
          </view>
          <text class="row-sub">{{ item.order_date || '未填写日期' }} / {{ item.process_status || '生产待处理' }} / {{ item.pay_status || '未收款' }} / {{ item.ship_status || '待发货' }}</text>
          <view v-if="item.items?.length" class="order-items">
            <view v-for="line in item.items" :key="line.id" class="item-line">
              <text>{{ line.item_name }} {{ line.spec }}</text>
              <text>{{ line.qty }}{{ line.unit }} x ¥{{ line.unit_price }} = ¥{{ line.line_total }}</text>
            </view>
          </view>
          <text class="row-sub">运费：¥{{ item.shipping_amount || '0.00' }}</text>
          <text class="row-sub">物流：{{ item.ship_tracking_no || '暂无单号' }}</text>
        </view>
      </view>

      <view v-if="page?.direct_ship_batches?.length" class="panel">
        <text class="panel-title">一件代发批次</text>
        <view v-for="item in page.direct_ship_batches" :key="item.id" class="list-row">
          <text class="row-main">{{ item.batch_no }}</text>
          <text class="row-sub">{{ item.source_name }} / {{ item.status }} / {{ item.total_rows }} 单</text>
        </view>
      </view>

      <view v-if="page?.inventory?.length" class="panel">
        <text class="panel-title">库存</text>
        <view v-for="item in page.inventory" :key="item.id" class="list-row">
          <text class="row-main">{{ item.item_name }}</text>
          <text class="row-sub">{{ item.warehouse }} / {{ item.qty_g }}g / {{ item.qty_units }} 件</text>
        </view>
      </view>

      <view v-if="page?.processing_requests?.length" class="panel">
        <text class="panel-title">加工申请</text>
        <view v-for="item in page.processing_requests" :key="item.id" class="list-row">
          <text class="row-main">{{ item.request_no }}</text>
          <text class="row-sub">{{ item.status }} / {{ item.input_qty_g }}g -> {{ item.target_qty }} 件</text>
        </view>
      </view>

      <view v-if="page?.fee_items?.length" class="panel">
        <text class="panel-title">费用明细</text>
        <view v-for="item in page.fee_items" :key="item.id" class="list-row">
          <text class="row-main">{{ item.fee_type }} ¥{{ item.amount }}</text>
          <text class="row-sub">{{ item.status }} / {{ item.occurred_at }}</text>
        </view>
      </view>

      <view v-if="page?.settlement_batches?.length" class="panel">
        <text class="panel-title">结算单</text>
        <view v-for="item in page.settlement_batches" :key="item.id" class="list-row">
          <text class="row-main">{{ item.settlement_no }} ¥{{ item.total_amount }}</text>
          <text class="row-sub">{{ item.status }} / {{ item.period_from }} 至 {{ item.period_to }}</text>
        </view>
      </view>

      <view v-if="page && !sections.length" class="state">
        <text>暂无数据</text>
      </view>
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 32rpx;
  background: #f6f6f6;
  box-sizing: border-box;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 24rpx 0 32rpx;
}

.eyebrow {
  color: #6f5d2e;
  font-size: 24rpx;
  font-weight: 600;
}

.title {
  color: #171717;
  font-size: 42rpx;
  font-weight: 700;
}

.subtitle {
  color: #666666;
  font-size: 26rpx;
}

.metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
  margin-bottom: 20rpx;
}

.metric,
.panel,
.section-row {
  background: #ffffff;
  border: 1rpx solid #e8e8e8;
  border-radius: 8rpx;
}

.metric {
  min-height: 110rpx;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8rpx;
  padding: 20rpx;
  box-sizing: border-box;
}

.metric-value {
  color: #171717;
  font-size: 36rpx;
  font-weight: 700;
}

.metric-label,
.row-sub {
  color: #666666;
  font-size: 24rpx;
  line-height: 1.5;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  padding: 24rpx;
  margin-bottom: 20rpx;
  box-sizing: border-box;
}

.panel-title {
  color: #171717;
  font-size: 30rpx;
  font-weight: 700;
}

.input,
.textarea {
  width: 100%;
  min-height: 76rpx;
  padding: 0 20rpx;
  background: #f8f8f8;
  border: 1rpx solid #e2e2e2;
  border-radius: 8rpx;
  color: #171717;
  font-size: 26rpx;
  box-sizing: border-box;
}

.textarea {
  min-height: 132rpx;
  padding-top: 18rpx;
}

.primary {
  width: 100%;
  min-height: 82rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #171717;
  color: #ffffff;
  border-radius: 8rpx;
  font-size: 28rpx;
}

.section-list {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  margin-bottom: 20rpx;
}

.section-row {
  min-height: 86rpx;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24rpx;
}

.section-title,
.row-main {
  color: #171717;
  font-size: 28rpx;
  font-weight: 600;
}

.row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
}

.price {
  color: #171717;
  font-size: 28rpx;
  font-weight: 700;
  white-space: nowrap;
}

.order-items {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 12rpx;
  background: #f8f8f8;
  border-radius: 8rpx;
}

.item-line {
  display: flex;
  justify-content: space-between;
  gap: 16rpx;
  color: #333333;
  font-size: 24rpx;
  line-height: 1.45;
}

.section-count {
  color: #6f5d2e;
  font-size: 30rpx;
  font-weight: 700;
}

.list-row {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 18rpx 0;
  border-top: 1rpx solid #eeeeee;
}

.list-row:first-of-type {
  border-top: 0;
}

.state {
  min-height: 160rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666666;
  font-size: 28rpx;
}

.error {
  color: #b42318;
}
</style>
