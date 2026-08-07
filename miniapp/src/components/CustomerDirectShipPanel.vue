<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  cancelDirectShipRequest,
  createDirectShipRequest,
  fetchDirectShipCatalog,
  fetchDirectShipRequests,
  parseEmployeeCustomerRecipient,
  previewDirectShipRequest,
  type DirectShipCatalog,
  type DirectShipPreview,
  type DirectShipRequest,
  type EmployeeOrderProductFamily,
  type EmployeeOrderProductSpec,
} from '../api/customerPortal'
import { productSpecLabel, productSpecWeightG } from '../utils/employeeOrder'
import { directShipStatusLabel } from '../utils/customerFulfillment'
import CustomerProductSelector from './CustomerProductSelector.vue'

const props = withDefaults(defineProps<{ token: string; customerId: number; showCreate?: boolean }>(), { showCreate: true })
type DraftLine = { product_id: number; product_name: string; spec_g: number; spec_label: string; qty: number }

const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const catalog = ref<DirectShipCatalog>({ current_customer_id: 0, product_families: [] })
const requests = ref<DirectShipRequest[]>([])
const pastedRecipient = ref('')
const recipientName = ref('')
const recipientPhone = ref('')
const province = ref('')
const city = ref('')
const district = ref('')
const detailAddress = ref('')
const recipientCompany = ref('')
const note = ref('')
const lines = ref<DraftLine[]>([])
const preview = ref<DirectShipPreview | null>(null)
const idempotencyKey = ref(newIdempotencyKey())

function newIdempotencyKey(): string {
  return `mini-ds-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    const jobs: Promise<unknown>[] = []
    if (props.showCreate) {
      jobs.push(fetchDirectShipCatalog(props.token).then((value) => { catalog.value = value }))
    } else {
      jobs.push(fetchDirectShipRequests(props.token).then((value) => { requests.value = value.rows || [] }))
    }
    await Promise.all(jobs)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '发货信息加载失败'
  } finally {
    loading.value = false
  }
}

async function parseRecipient() {
  if (!pastedRecipient.value.trim()) return
  errorMessage.value = ''
  try {
    const parsed = await parseEmployeeCustomerRecipient(props.token, pastedRecipient.value)
    recipientName.value = parsed.recipient_name || ''
    recipientPhone.value = parsed.phone || ''
    province.value = parsed.province || ''
    city.value = parsed.city || ''
    district.value = parsed.district || ''
    detailAddress.value = parsed.detail_address || parsed.address || ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '收货信息解析失败'
  }
}

function addProduct(value: { family: EmployeeOrderProductFamily; spec: EmployeeOrderProductSpec }) {
  const productID = Number(value.spec.sku_id || value.spec.product_id || 0)
  const specG = productSpecWeightG(value.spec)
  const existing = lines.value.find((item) => item.product_id === productID && item.spec_g === specG)
  if (existing) existing.qty += 1
  else lines.value.push({
    product_id: productID,
    product_name: value.spec.sku_name || value.family.customer_product_display_name || value.family.name,
    spec_g: specG,
    spec_label: productSpecLabel(value.spec),
    qty: 1,
  })
  preview.value = null
}

function removeLine(index: number) {
  lines.value.splice(index, 1)
  preview.value = null
}

function payload() {
  return {
    idempotency_key: idempotencyKey.value,
    recipient_name: recipientName.value.trim(),
    recipient_phone: recipientPhone.value.trim(),
    province: province.value.trim(),
    city: city.value.trim(),
    district: district.value.trim(),
    detail_address: detailAddress.value.trim(),
    recipient_company: recipientCompany.value.trim(),
    items: lines.value.map(({ product_id, spec_g, qty }) => ({ product_id, spec_g, qty: Number(qty || 0) })),
    note: note.value.trim(),
  }
}

function validate(): string {
  if (!recipientName.value.trim() || !recipientPhone.value.trim() || !detailAddress.value.trim()) return '请确认收件人、电话和详细地址'
  if (!lines.value.length || lines.value.some((item) => Number(item.qty || 0) <= 0)) return '请至少选择一个商品规格并填写数量'
  return ''
}

async function previewRequest() {
  const validation = validate()
  if (validation) { errorMessage.value = validation; return }
  try {
    preview.value = await previewDirectShipRequest(props.token, payload())
    errorMessage.value = preview.value.can_submit ? '' : '当前客户仓库存不足，无法提交发货'
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '发货预览失败'
  }
}

async function submitRequest() {
  const validation = validate()
  if (validation) { errorMessage.value = validation; return }
  submitting.value = true
  errorMessage.value = ''
  try {
    const checked = await previewDirectShipRequest(props.token, payload())
    preview.value = checked
    if (!checked.can_submit) throw new Error('当前客户仓库存不足，无法提交发货')
    await createDirectShipRequest(props.token, payload())
    lines.value = []
    preview.value = null
    note.value = ''
    idempotencyKey.value = newIdempotencyKey()
    uni.showToast({ title: '发货申请已提交', icon: 'success' })
    await load()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '发货提交失败'
  } finally {
    submitting.value = false
  }
}

async function cancelRequest(item: DirectShipRequest) {
  try {
    await cancelDirectShipRequest(props.token, item.id)
    uni.showToast({ title: '已取消并释放库存', icon: 'success' })
    await load()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '取消失败'
  }
}

onMounted(() => { void load() })
</script>

<template>
  <view class="workspace">
    <view v-if="showCreate" class="panel">
      <text class="title">新建发货</text>
      <textarea v-model="pastedRecipient" class="textarea" placeholder="粘贴收货信息，例如：张三 13800138000 云南省普洱市思茅区咖啡路88号" />
      <button class="secondary" @tap="parseRecipient">一键解析地址</button>
      <input v-model="recipientName" class="input" placeholder="收件人" />
      <input v-model="recipientPhone" class="input" placeholder="联系电话" />
      <view class="region-row">
        <input v-model="province" class="input" placeholder="省" />
        <input v-model="city" class="input" placeholder="市" />
        <input v-model="district" class="input" placeholder="区/县" />
      </view>
      <input v-model="detailAddress" class="input" placeholder="详细地址" />
      <input v-model="recipientCompany" class="input" placeholder="公司/门店（可选）" />
      <text class="subtitle">选择当前客户成品仓内的商品</text>
      <CustomerProductSelector :families="catalog.product_families" :customer-id="customerId" @select="addProduct" />
      <view v-for="(line, index) in lines" :key="`${line.product_id}:${line.spec_g}`" class="line">
        <view class="line-copy"><text class="line-name">{{ line.product_name }}</text><text class="muted">{{ line.spec_label }}</text></view>
        <input v-model.number="line.qty" class="qty" type="number" @input="preview = null" />
        <button class="remove" @tap="removeLine(index)">删除</button>
      </view>
      <textarea v-model="note" class="textarea" placeholder="发货备注（可选）" />
      <view v-if="preview" class="preview"><text>系统将按先进先出自动分配批次；跨仓时拆成 {{ preview.warehouses?.length || 0 }} 个包裹。</text></view>
      <view class="actions"><button class="secondary" @tap="previewRequest">预览分配</button><button class="primary" :disabled="submitting" @tap="submitRequest">提交发货</button></view>
    </view>

    <view v-if="!showCreate" class="panel">
      <text class="title">发货中心</text>
      <text v-if="loading" class="muted">加载中...</text>
      <view v-for="item in requests" :key="item.id" class="request">
        <view class="request-head"><text class="line-name">{{ item.request_no }}</text><text class="status">{{ directShipStatusLabel(item.status) }}</text></view>
        <text class="muted">{{ item.recipient_name }} {{ item.recipient_phone }} · {{ item.province }}{{ item.city }}{{ item.district }}{{ item.detail_address }}</text>
        <text v-for="line in item.items || []" :key="`${line.product_id}:${line.spec_g}`" class="muted">{{ line.product_name || `商品 ${line.product_id}` }}{{ line.sku_code ? `（${line.sku_code}）` : '' }} · {{ line.spec_label || `${line.spec_g}g` }} · {{ line.qty }} 件</text>
        <view v-for="pkg in item.packages || []" :key="pkg.id" class="package">
          <text>{{ pkg.order_no }} · {{ pkg.warehouse }} · {{ directShipStatusLabel(pkg.status) }}</text>
          <text class="muted">{{ pkg.carrier_name || '物流待录入' }} {{ pkg.tracking_no || '' }}</text>
          <view v-for="(event, index) in pkg.events || []" :key="`${pkg.id}:event:${index}`" class="event">
            <text class="muted">{{ event.time || '' }} {{ event.description || directShipStatusLabel(event.status) }}</text>
            <text v-if="event.location" class="muted">{{ event.location }}</text>
          </view>
        </view>
        <button v-if="['pending','reserved','待处理','待发货'].includes(item.status)" class="secondary compact" @tap="cancelRequest(item)">取消发货</button>
      </view>
      <text v-if="!loading && !requests.length" class="muted">暂无发货记录</text>
    </view>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.request,.package,.line-copy,.event{display:flex;flex-direction:column;gap:16rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}.title{font-size:30rpx;font-weight:900}.subtitle,.line-name{font-size:27rpx;font-weight:800}.input,.textarea,.qty{min-height:76rpx;padding:0 18rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa;box-sizing:border-box}.textarea{min-height:120rpx;padding-top:16rpx}.region-row,.actions,.line,.request-head{display:flex;gap:12rpx;align-items:center}.region-row .input,.actions button{flex:1}.line{padding:14rpx 0;border-top:1rpx solid #eee}.line-copy{flex:1;gap:4rpx}.qty{width:120rpx}.remove{margin:0;color:#a22;background:#fff;border:1rpx solid #e5caca}.primary,.secondary{min-height:76rpx;margin:0;border-radius:8rpx;font-size:25rpx}.primary{background:#2b2118;color:#fff}.secondary{background:#fff;color:#333;border:1rpx solid #d8d8d8}.compact{min-height:64rpx}.request,.package,.preview{padding:16rpx;border:1rpx solid #eee;border-radius:8rpx}.event{gap:4rpx;padding-top:8rpx;border-top:1rpx dashed #ddd}.request-head{justify-content:space-between}.muted{color:#707070;font-size:23rpx;line-height:1.5}.status{color:#28624a;font-size:23rpx;font-weight:800}.error{display:block;padding:18rpx;color:#b42318}
</style>
