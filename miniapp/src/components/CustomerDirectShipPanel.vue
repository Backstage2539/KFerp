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
} from '../api/customerPortal'
import { employeeOrderProductFamilyKey, productSpecLabel } from '../utils/employeeOrder'
import { directShipStatusLabel } from '../utils/customerFulfillment'
import {
  buildDirectShipDraftItems,
  createDirectShipDraftLine,
  directShipDraftValidation,
  selectDirectShipDraftProduct,
  selectDirectShipDraftSpec,
  type DirectShipDraftLine,
} from '../utils/directShipDraft'
import {
  directShipDatePresetRange,
  directShipRequestTitle,
  normalizeDirectShipDateRange,
  type DirectShipDatePreset,
} from '../utils/directShipFilters'
import ProductFamilyPickerSheet from './ProductFamilyPickerSheet.vue'

const props = withDefaults(defineProps<{ token: string; customerId: number; showCreate?: boolean }>(), { showCreate: true })
type PickerChangeEvent = { detail?: { value?: string | number } }

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
const lines = ref<DirectShipDraftLine[]>([createDirectShipDraftLine()])
const preview = ref<DirectShipPreview | null>(null)
const productSelectorOpen = ref(false)
const editingLineKey = ref('')
const idempotencyKey = ref(newIdempotencyKey())
const shipmentQueryInput = ref('')
const shipmentQuery = ref('')
const shippedFrom = ref('')
const shippedTo = ref('')
const activeDatePreset = ref<DirectShipDatePreset | ''>('')
const currentPage = ref(1)
const pageLimit = ref(10)
const totalRows = ref(0)
const totalPages = ref(1)
const jumpPage = ref('1')
const pageLimitOptions = [10, 20, 50]
const pageLimitLabels = pageLimitOptions.map((value) => `每页 ${value} 条`)
let loadVersion = 0
let previewVersion = 0

function newIdempotencyKey(): string {
  return `mini-ds-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

async function load() {
  const version = ++loadVersion
  loading.value = true
  errorMessage.value = ''
  try {
    if (props.showCreate) {
      const value = await fetchDirectShipCatalog(props.token)
      if (version !== loadVersion) return
      catalog.value = value
    } else {
      const value = await fetchDirectShipRequests(props.token, {
        q: shipmentQuery.value,
        shipped_from: shippedFrom.value,
        shipped_to: shippedTo.value,
        page: currentPage.value,
        limit: pageLimit.value,
      })
      if (version !== loadVersion) return
      requests.value = value.rows || []
      totalRows.value = Math.max(0, Number(value.total ?? requests.value.length) || 0)
      pageLimit.value = Math.max(1, Number(value.limit ?? pageLimit.value) || pageLimit.value)
      totalPages.value = Math.max(1, Number(value.total_pages) || Math.ceil(totalRows.value / pageLimit.value) || 1)
      currentPage.value = Math.min(totalPages.value, Math.max(1, Number(value.page ?? currentPage.value) || 1))
      jumpPage.value = String(currentPage.value)
    }
  } catch (error) {
    if (version !== loadVersion) return
    errorMessage.value = error instanceof Error ? error.message : '发货信息加载失败'
  } finally {
    if (version === loadVersion) loading.value = false
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
    invalidatePreview()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '收货信息解析失败'
  }
}

function invalidatePreview() {
  previewVersion += 1
  preview.value = null
}

function addLine() {
  if (submitting.value) return
  lines.value.push(createDirectShipDraftLine())
  invalidatePreview()
}

function removeLine(index: number) {
  if (submitting.value) return
  if (lines.value.length === 1) {
    lines.value.splice(0, 1, createDirectShipDraftLine(lines.value[0]?.key))
    invalidatePreview()
    return
  }
  lines.value.splice(index, 1)
  invalidatePreview()
}

function familyForLine(line: DirectShipDraftLine): EmployeeOrderProductFamily | undefined {
  return catalog.value.product_families.find(
    (family) => employeeOrderProductFamilyKey(family) === line.product_family_key,
  )
}

function specLabelsForLine(line: DirectShipDraftLine): string[] {
  return familyForLine(line)?.specs.map(productSpecLabel) || []
}

function selectedSpecIndexForLine(line: DirectShipDraftLine): number {
  const family = familyForLine(line)
  return Math.max(0, family?.specs.findIndex(
    (spec) => Number(line.bom_spec_id || 0) > 0
      ? Number(spec.bom_spec_id || 0) === Number(line.bom_spec_id)
        && Number(spec.bom_variant_id || 0) === Number(line.bom_variant_id)
      : Number(spec.sku_id || spec.product_id || 0) === Number(line.product_id || 0),
  ) ?? 0)
}

function directShipItemSpecLabel(item: {
  bom_spec_id?: number
  spec_name?: string
  bom_spec_name?: string
  spec_label?: string
  inventory_unit?: string
  spec_g?: number
}): string {
  if (Number(item.bom_spec_id || 0) > 0) {
    return String(item.spec_name || item.bom_spec_name || item.spec_label || '').trim() || '当前 BOM 规格'
  }
  return String(item.spec_label || '').trim() || `${Number(item.spec_g || 0)}g`
}

function directShipItemQtyLabel(item: { qty?: number; bom_spec_id?: number; inventory_unit?: string }): string {
  const unit = Number(item.bom_spec_id || 0) > 0 ? String(item.inventory_unit || '').trim() : '件'
  return `${Number(item.qty || 0)} ${unit || '件'}`
}

function openProductSelector(lineKey: string) {
  if (loading.value || submitting.value) return
  editingLineKey.value = lineKey
  productSelectorOpen.value = true
}

function closeProductSelector() {
  productSelectorOpen.value = false
  editingLineKey.value = ''
}

function chooseProduct(family: EmployeeOrderProductFamily) {
  const line = lines.value.find((item) => item.key === editingLineKey.value)
  if (!line) return
  const selected = selectDirectShipDraftProduct(line, family)
  if (!selected) {
    uni.showToast({ title: '该商品暂无可选规格', icon: 'none' })
    return
  }
  Object.assign(line, selected)
  closeProductSelector()
  invalidatePreview()
}

function chooseSpec(line: DirectShipDraftLine, event: PickerChangeEvent) {
  if (submitting.value) return
  const family = familyForLine(line)
  const spec = family?.specs[Number(event.detail?.value || 0)]
  if (!spec) return
  Object.assign(line, selectDirectShipDraftSpec(line, spec))
  invalidatePreview()
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
    items: buildDirectShipDraftItems(lines.value),
    note: note.value.trim(),
  }
}

function validate(): string {
  if (!recipientName.value.trim() || !recipientPhone.value.trim() || !detailAddress.value.trim()) return '请确认收件人、电话和详细地址'
  return directShipDraftValidation(lines.value)
}

async function previewRequest() {
  const validation = validate()
  if (validation) { errorMessage.value = validation; return }
  const version = ++previewVersion
  const command = payload()
  try {
    const checked = await previewDirectShipRequest(props.token, command)
    if (version !== previewVersion) return
    preview.value = checked
    errorMessage.value = preview.value.can_submit ? '' : '当前客户仓库存不足，无法提交发货'
  } catch (error) {
    if (version !== previewVersion) return
    errorMessage.value = error instanceof Error ? error.message : '发货预览失败'
  }
}

async function submitRequest() {
  if (submitting.value) return
  const validation = validate()
  if (validation) { errorMessage.value = validation; return }
  const version = ++previewVersion
  const command = payload()
  submitting.value = true
  errorMessage.value = ''
  try {
    const checked = await previewDirectShipRequest(props.token, command)
    if (version !== previewVersion) throw new Error('发货内容已变更，请重新预览后提交')
    preview.value = checked
    if (!checked.can_submit) throw new Error('当前客户仓库存不足，无法提交发货')
    await createDirectShipRequest(props.token, command)
    lines.value = [createDirectShipDraftLine()]
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

async function applyShipmentFilters() {
  shipmentQuery.value = shipmentQueryInput.value.trim()
  const range = normalizeDirectShipDateRange(shippedFrom.value, shippedTo.value)
  shippedFrom.value = range.shipped_from || ''
  shippedTo.value = range.shipped_to || ''
  activeDatePreset.value = ''
  currentPage.value = 1
  await load()
}

async function applyDatePreset(preset: DirectShipDatePreset) {
  shipmentQuery.value = shipmentQueryInput.value.trim()
  const range = directShipDatePresetRange(preset)
  shippedFrom.value = range.shipped_from
  shippedTo.value = range.shipped_to
  activeDatePreset.value = preset
  currentPage.value = 1
  await load()
}

function setShippedFrom(event: PickerChangeEvent) {
  shippedFrom.value = String(event.detail?.value || '')
  activeDatePreset.value = ''
}

function setShippedTo(event: PickerChangeEvent) {
  shippedTo.value = String(event.detail?.value || '')
  activeDatePreset.value = ''
}

async function clearShipmentFilters() {
  shipmentQueryInput.value = ''
  shipmentQuery.value = ''
  shippedFrom.value = ''
  shippedTo.value = ''
  activeDatePreset.value = ''
  currentPage.value = 1
  await load()
}

async function goToPage(value: number | string) {
  const target = Math.min(totalPages.value, Math.max(1, Math.trunc(Number(value) || 1)))
  jumpPage.value = String(target)
  if (target === currentPage.value) return
  currentPage.value = target
  await load()
}

async function changePageLimit(event: PickerChangeEvent) {
  const index = Math.max(0, Number(event.detail?.value || 0))
  pageLimit.value = pageLimitOptions[index] || pageLimitOptions[0]
  currentPage.value = 1
  await load()
}

onMounted(() => { void load() })
</script>

<template>
  <view class="workspace">
    <view v-if="showCreate" class="panel">
      <text class="title">新建发货</text>
      <textarea v-model="pastedRecipient" class="textarea" :disabled="submitting" placeholder="粘贴收货信息，例如：张三 13800138000 云南省普洱市思茅区咖啡路88号" />
      <button class="secondary" :disabled="submitting" @tap="parseRecipient">一键解析地址</button>
      <input v-model="recipientName" class="input" :disabled="submitting" placeholder="收件人" @input="invalidatePreview" />
      <input v-model="recipientPhone" class="input" :disabled="submitting" placeholder="联系电话" @input="invalidatePreview" />
      <view class="region-row">
        <input v-model="province" class="input" :disabled="submitting" placeholder="省" @input="invalidatePreview" />
        <input v-model="city" class="input" :disabled="submitting" placeholder="市" @input="invalidatePreview" />
        <input v-model="district" class="input" :disabled="submitting" placeholder="区/县" @input="invalidatePreview" />
      </view>
      <input v-model="detailAddress" class="input" :disabled="submitting" placeholder="详细地址" @input="invalidatePreview" />
      <input v-model="recipientCompany" class="input" :disabled="submitting" placeholder="公司/门店（可选）" @input="invalidatePreview" />
      <text class="subtitle">选择当前客户成品仓内的商品</text>
      <view v-for="(line, index) in lines" :key="line.key" class="line">
        <view class="line-head">
          <text class="line-name">商品 {{ index + 1 }}</text>
          <button class="remove" @tap="removeLine(index)">删除本行</button>
        </view>
        <text class="line-label">商品</text>
        <view class="selector-field" @tap="openProductSelector(line.key)">
          <text :class="{ muted: !line.product_name }">{{ line.product_name || '搜索并选择商品' }}</text>
          <text class="chevron">›</text>
        </view>
        <text class="line-label">规格</text>
        <picker
          mode="selector"
          :range="specLabelsForLine(line)"
          :value="selectedSpecIndexForLine(line)"
          :disabled="submitting || !familyForLine(line)"
          @change="chooseSpec(line, $event)"
        >
          <view class="selector-field" :class="{ muted: !familyForLine(line) }">
            <text>{{ line.spec_label || (familyForLine(line) ? '选择该商品的规格' : '请先选择商品') }}</text>
            <text class="chevron">›</text>
          </view>
        </picker>
        <text class="line-label">数量</text>
        <input v-model.number="line.qty" class="qty" type="number" :disabled="submitting" placeholder="填写数量" @input="invalidatePreview" />
      </view>
      <button class="secondary add-line" :disabled="submitting" @tap="addLine">新增商品</button>
      <textarea v-model="note" class="textarea" :disabled="submitting" placeholder="发货备注（可选）" @input="invalidatePreview" />
      <view v-if="preview" class="preview"><text>系统将按先进先出自动分配批次；跨仓时拆成 {{ preview.warehouses?.length || 0 }} 个包裹。</text></view>
      <view class="actions"><button class="secondary" :disabled="submitting" @tap="previewRequest">预览分配</button><button class="primary" :disabled="submitting" @tap="submitRequest">提交发货</button></view>
    </view>

    <view v-if="!showCreate" class="panel">
      <text class="title">发货中心</text>
      <view class="filters">
        <input
          v-model="shipmentQueryInput"
          class="input"
          confirm-type="search"
          placeholder="搜索收件客户/公司、收件人、电话、目的地"
          @confirm="applyShipmentFilters"
        />
        <text class="filter-label">按实际发货时间</text>
        <view class="date-presets">
          <button class="chip" :class="{ active: activeDatePreset === 'today' }" @tap="applyDatePreset('today')">当天</button>
          <button class="chip" :class="{ active: activeDatePreset === 'last3' }" @tap="applyDatePreset('last3')">三天内</button>
          <button class="chip" :class="{ active: activeDatePreset === 'last7' }" @tap="applyDatePreset('last7')">一周内</button>
          <button class="chip" :class="{ active: activeDatePreset === 'month' }" @tap="applyDatePreset('month')">当月</button>
        </view>
        <view class="date-range">
          <picker mode="date" :value="shippedFrom" @change="setShippedFrom">
            <view class="picker-field">{{ shippedFrom || '发货开始日期' }}</view>
          </picker>
          <picker mode="date" :value="shippedTo" @change="setShippedTo">
            <view class="picker-field">{{ shippedTo || '发货结束日期' }}</view>
          </picker>
        </view>
        <view class="filter-actions">
          <button class="secondary compact" @tap="clearShipmentFilters">清除</button>
          <button class="primary compact" @tap="applyShipmentFilters">查询</button>
        </view>
      </view>
      <text v-if="loading" class="muted">加载中...</text>
      <view v-for="item in requests" :key="item.id" class="request">
        <view class="request-head"><text class="line-name">{{ directShipRequestTitle(item) }}</text><text class="status">{{ directShipStatusLabel(item.status) }}</text></view>
        <text v-if="item.recipient_company" class="muted">收件客户/公司：{{ item.recipient_company }}</text>
        <text class="muted">{{ item.recipient_phone }} · {{ item.province }}{{ item.city }}{{ item.district }}{{ item.detail_address }}</text>
        <text v-for="line in item.items || []" :key="`${line.product_id}:${line.bom_spec_id || 0}:${line.bom_variant_id || 0}:${line.spec_g}`" class="muted">{{ line.product_name || `商品 ${line.product_id}` }}{{ line.sku_code ? `（${line.sku_code}）` : '' }} · {{ directShipItemSpecLabel(line) }} · {{ directShipItemQtyLabel(line) }}</text>
        <view v-for="pkg in item.packages || []" :key="pkg.id" class="package">
          <text>{{ pkg.order_no }} · {{ pkg.warehouse }} · {{ directShipStatusLabel(pkg.status) }}</text>
          <text v-if="pkg.shipped_at" class="muted">发货时间：{{ pkg.shipped_at }}</text>
          <text class="muted">{{ pkg.carrier_name || '物流待录入' }} {{ pkg.tracking_no || '' }}</text>
          <view v-for="(event, index) in pkg.events || []" :key="`${pkg.id}:event:${index}`" class="event">
            <text class="muted">{{ event.time || '' }} {{ event.description || directShipStatusLabel(event.status) }}</text>
            <text v-if="event.location" class="muted">{{ event.location }}</text>
          </view>
        </view>
        <button v-if="['pending','reserved','待处理','待发货'].includes(item.status)" class="secondary compact" @tap="cancelRequest(item)">取消发货</button>
      </view>
      <text v-if="!loading && !requests.length" class="muted">暂无发货记录</text>
      <view class="pagination">
        <text class="muted">共 {{ totalRows }} 条 · 共 {{ totalPages }} 页</text>
        <view class="page-actions">
          <button class="secondary compact" :disabled="currentPage <= 1 || loading" @tap="goToPage(currentPage - 1)">上一页</button>
          <text class="page-current">第 {{ currentPage }} / {{ totalPages }} 页</text>
          <button class="secondary compact" :disabled="currentPage >= totalPages || loading" @tap="goToPage(currentPage + 1)">下一页</button>
        </view>
        <view class="page-settings">
          <input v-model="jumpPage" class="jump-input" type="number" placeholder="页码" />
          <button class="secondary compact" :disabled="loading" @tap="goToPage(jumpPage)">跳页</button>
          <picker mode="selector" :range="pageLimitLabels" :value="Math.max(0, pageLimitOptions.indexOf(pageLimit))" @change="changePageLimit">
            <view class="picker-field page-limit">每页 {{ pageLimit }} 条</view>
          </picker>
        </view>
      </view>
    </view>
    <ProductFamilyPickerSheet
      :visible="productSelectorOpen"
      :families="catalog.product_families"
      :customer-id="customerId"
      :loading="loading"
      customer-facing-names
      @close="closeProductSelector"
      @select="chooseProduct"
    />
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.request,.package,.event,.filters,.pagination,.line{display:flex;flex-direction:column;gap:16rpx}
.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}
.title{font-size:30rpx;font-weight:900}
.subtitle,.line-name{font-size:27rpx;font-weight:800}
.input,.textarea,.qty,.jump-input,.selector-field{min-height:76rpx;padding:0 18rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa;box-sizing:border-box}
.textarea{min-height:120rpx;padding-top:16rpx}
.region-row,.actions,.request-head,.date-presets,.date-range,.filter-actions,.page-actions,.page-settings,.line-head,.selector-field{display:flex;gap:12rpx;align-items:center}
.region-row .input,.actions button,.date-range picker,.filter-actions button{flex:1}
.line{padding:20rpx;border:1rpx solid #e6e0d8;border-radius:10rpx;background:#fffdf9}
.line-head{justify-content:space-between}
.line-label{color:#555;font-size:23rpx;font-weight:750}
.selector-field{justify-content:space-between;width:100%}
.chevron{color:#8a938e;font-size:34rpx}
.qty{width:100%}
.remove{width:auto;min-height:58rpx;margin:0;padding:0 18rpx;color:#a22;background:#fff;border:1rpx solid #e5caca;font-size:22rpx}
.add-line{width:100%;border-style:dashed}
.primary,.secondary,.chip{min-height:76rpx;margin:0;border-radius:8rpx;font-size:25rpx}
.primary{background:#2b2118;color:#fff}
.secondary,.chip{background:#fff;color:#333;border:1rpx solid #d8d8d8}
.compact{min-height:64rpx}
.request,.package,.preview,.filters,.pagination{padding:16rpx;border:1rpx solid #eee;border-radius:8rpx}
.event{gap:4rpx;padding-top:8rpx;border-top:1rpx dashed #ddd}
.request-head{justify-content:space-between;align-items:flex-start}
.request-head .line-name{flex:1}
.muted{color:#707070;font-size:23rpx;line-height:1.5}
.status{flex-shrink:0;color:#28624a;font-size:23rpx;font-weight:800}
.error{display:block;padding:18rpx;color:#b42318}
.filter-label{color:#555;font-size:24rpx;font-weight:700}
.date-presets{flex-wrap:wrap}
.chip{min-height:60rpx;padding:0 18rpx;font-size:23rpx}
.chip.active{background:#2b2118;color:#fff}
.picker-field{min-height:68rpx;padding:0 16rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa;color:#555;font-size:23rpx;line-height:68rpx;box-sizing:border-box}
.pagination{margin-top:4rpx}
.page-actions{justify-content:space-between}
.page-actions button{flex:1}
.page-current{flex:1.2;text-align:center;color:#555;font-size:23rpx}
.page-settings{justify-content:flex-end}
.jump-input{width:120rpx;min-height:64rpx}
.page-limit{min-width:180rpx}
.secondary[disabled]{opacity:.45}
</style>
