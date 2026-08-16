<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  createProcessingRequest,
  fetchProcessingCatalog,
  fetchProcessingRequests,
  previewProcessingRequest,
  type DirectShipCatalog,
  type EmployeeOrderProductFamily,
  type EmployeeOrderProductSpec,
  type ProcessingRequest,
  type ProcessingRequestPreview,
} from '../api/customerPortal'
import {
  mergeProcessingTargetLines,
  productionStatusLabel,
  productionSubmissionBlockReason,
} from '../utils/customerFulfillment'
import {
  normalizeProcessingPrefillItems,
  resolveProcessingPrefillLines,
  type ProcessingPrefillDraftLine,
  type ProcessingPrefillItem,
} from '../utils/customerInventory'
import { productSpecLabel, productSpecWeightG } from '../utils/employeeOrder'
import CustomerProductSelector from './CustomerProductSelector.vue'

const props = withDefaults(defineProps<{
  token: string
  customerId: number
  prefillProductId?: number
  prefillSpecG?: number
  prefillBomSpecId?: number
  prefillBomVariantId?: number
  prefillInventoryUnit?: string
  prefillItems?: ProcessingPrefillItem[]
}>(), {
  prefillProductId: 0,
  prefillSpecG: 0,
  prefillBomSpecId: 0,
  prefillBomVariantId: 0,
  prefillInventoryUnit: '',
  prefillItems: () => [],
})

const emit = defineEmits<{ prefillConsumed: [] }>()
type DraftLine = ProcessingPrefillDraftLine
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const prefillWarning = ref('')
const catalog = ref<DirectShipCatalog>({ current_customer_id: 0, product_families: [] })
const requests = ref<ProcessingRequest[]>([])
const lines = ref<DraftLine[]>([])
const note = ref('')
const preview = ref<ProcessingRequestPreview | null>(null)
let previewTimer: ReturnType<typeof setTimeout> | null = null
let previewVersion = 0
let prefillApplied = false
const submissionBlock = computed(() => productionSubmissionBlockReason(preview.value ? {
  complete: preview.value.complete ?? preview.value.items.every((item) => Number(item.bom_version_id || 0) > 0),
  canSubmit: preview.value.can_submit,
  materials: preview.value.materials,
} : null))

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [catalogValue, requestValue] = await Promise.all([
      fetchProcessingCatalog(props.token),
      fetchProcessingRequests(props.token),
    ])
    catalog.value = catalogValue
    requests.value = requestValue.rows || []
    applyPrefill()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '生产工单加载失败'
  } finally {
    loading.value = false
  }
}

function applyPrefill() {
  if (prefillApplied) return
  const requested = normalizeProcessingPrefillItems(
    props.prefillItems?.length
      ? props.prefillItems
      : (props.prefillProductId > 0 ? [{
          product_id: props.prefillProductId,
          bom_spec_id: props.prefillBomSpecId,
          bom_variant_id: props.prefillBomVariantId,
          spec_g: props.prefillSpecG,
          inventory_unit: props.prefillInventoryUnit,
          product_name: `商品 ${props.prefillProductId}`,
        }] : []),
  )
  if (!requested.length) return
  prefillApplied = true
  const resolved = resolveProcessingPrefillLines(requested, catalog.value.product_families || [])
  lines.value.push(...resolved.lines.map((item) => ({ ...item, qty: 0 })))
  if (resolved.unavailable.length) {
    prefillWarning.value = `以下商品当前不可生产（商品不可用或未配置有效 BOM），无法添加生产工单：${resolved.unavailable
      .map((item) => item.product_name || item.sku_code || `商品 ${item.product_id}`)
      .join('、')}`
  }
  emit('prefillConsumed')
}

function addProduct(value: { family: EmployeeOrderProductFamily; spec: EmployeeOrderProductSpec }) {
  const bomSpecID = Number(value.spec.bom_spec_id || 0)
  const bomVariantID = Number(value.spec.bom_variant_id || 0)
  const canonical = bomSpecID > 0 || bomVariantID > 0
  const productID = canonical
    ? Number(value.family.parent_product_id || value.spec.product_id || 0)
    : Number(value.spec.sku_id || value.spec.product_id || 0)
  const specG = canonical ? 0 : productSpecWeightG(value.spec)
  const existing = lines.value.find((item) => (
    item.product_id === productID
    && Number(item.bom_spec_id || 0) === bomSpecID
    && Number(item.bom_variant_id || 0) === bomVariantID
    && item.spec_g === specG
  ))
  if (existing) existing.qty += 1
  else lines.value.push({
    product_id: productID,
    bom_spec_id: canonical ? bomSpecID : undefined,
    bom_variant_id: canonical ? bomVariantID : undefined,
    product_name: value.spec.sku_name || value.family.customer_product_display_name || value.family.name,
    spec_g: specG,
    spec_label: productSpecLabel(value.spec),
    inventory_unit: canonical ? String(value.spec.inventory_unit || '').trim() : undefined,
    qty: 1,
  })
  schedulePreview()
}

function removeLine(index: number) {
  lines.value.splice(index, 1)
  schedulePreview()
}

function schedulePreview() {
  previewVersion += 1
  preview.value = null
  errorMessage.value = ''
  if (previewTimer) clearTimeout(previewTimer)
  if (!lines.value.length || lines.value.some((item) => Number(item.qty || 0) <= 0)) return
  previewTimer = setTimeout(() => { void runPreview() }, 300)
}

function payload() {
  return {
    items: mergeProcessingTargetLines(lines.value.map(({
      product_id,
      bom_spec_id,
      bom_variant_id,
      inventory_unit,
      spec_g,
      qty,
    }) => ({
      product_id,
      bom_spec_id,
      bom_variant_id,
      inventory_unit,
      spec_g,
      qty: Number(qty || 0),
    }))),
    note: note.value.trim(),
  }
}

function targetSpecLabel(item: {
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

function targetQtyLabel(item: { qty?: number; inventory_unit?: string; bom_spec_id?: number }): string {
  const unit = Number(item.bom_spec_id || 0) > 0 ? String(item.inventory_unit || '').trim() : '件'
  return `${Number(item.qty || 0)} ${unit || '件'}`
}

async function runPreview() {
  const requestVersion = ++previewVersion
  if (!payload().items.length) { errorMessage.value = '请选择至少一个目标商品规格'; return }
  errorMessage.value = ''
  try {
    const value = await previewProcessingRequest(props.token, payload())
    if (requestVersion !== previewVersion) return
    preview.value = value
    errorMessage.value = productionSubmissionBlockReason({
      complete: preview.value.complete ?? preview.value.items.every((item) => Number(item.bom_version_id || 0) > 0),
      canSubmit: preview.value.can_submit,
      materials: preview.value.materials,
    })
  } catch (error) {
    if (requestVersion !== previewVersion) return
    errorMessage.value = error instanceof Error ? error.message : 'BOM 试算失败'
  }
}

async function submit() {
  if (!preview.value || !preview.value.can_submit || submissionBlock.value) {
    errorMessage.value = submissionBlock.value || '请先完成 BOM 试算'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await createProcessingRequest(props.token, payload())
    lines.value = []
    preview.value = null
    note.value = ''
    uni.showToast({ title: '生产工单申请已提交', icon: 'success' })
    await load()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '生产工单提交失败'
  } finally {
    submitting.value = false
  }
}

onMounted(() => { void load() })
onBeforeUnmount(() => { if (previewTimer) clearTimeout(previewTimer) })
</script>

<template>
  <view class="workspace">
    <view class="panel">
      <text class="title">提交生产工单</text>
      <text class="hint">只选择目标 SKU、规格和数量；系统按当前有效 BOM 自动汇总物料需求。</text>
      <text v-if="prefillWarning" class="warning">{{ prefillWarning }}</text>
      <CustomerProductSelector :families="catalog.product_families" :customer-id="customerId" @select="addProduct" />
      <view v-for="(line, index) in lines" :key="`${line.product_id}:${line.bom_spec_id || 0}:${line.bom_variant_id || 0}:${line.spec_g}`" class="line">
        <view class="line-copy"><text class="line-name">{{ line.product_name }}</text><text class="hint">{{ line.spec_label }}</text></view>
        <input v-model.number="line.qty" class="qty" type="number" @input="schedulePreview" />
        <button class="remove" @tap="removeLine(index)">删除</button>
      </view>
      <textarea v-model="note" class="textarea" placeholder="生产要求（可选）" />
      <button class="secondary" :disabled="loading" @tap="runPreview">BOM 试算</button>

      <view v-if="preview" class="preview">
        <text class="subtitle">目标商品</text>
        <view v-for="item in preview.items" :key="item.line_no" class="preview-row">
          <text>{{ item.product_name }} · {{ targetSpecLabel(item) }} · {{ targetQtyLabel(item) }}</text>
          <text class="hint">BOM {{ item.bom_version_no || '未配置' }}{{ item.bom_inherited ? '（继承父商品）' : '' }} · 最大可生产 {{ item.max_producible_qty }} {{ item.inventory_unit || '件' }}</text>
        </view>
        <text class="subtitle">汇总物料</text>
        <view v-for="item in preview.materials" :key="`${item.material_id}:${item.component_type}`" class="preview-row" :class="{ shortage: item.shortage_g > 0 || item.shortage_units > 0 }">
          <text>{{ item.material_name }} · 需求 {{ item.required_g || item.required_units }}{{ item.required_g ? 'g' : item.unit }}</text>
          <text class="hint">客户库存 {{ item.customer_inventory_g || item.customer_inventory_units }}；客户在制品 {{ item.customer_wip_g || item.customer_wip_units }}</text>
          <text class="hint">工厂库存 {{ item.factory_inventory_g || item.factory_inventory_units }}；工厂在制品 {{ item.factory_wip_g || item.factory_wip_units }}；已有预留 {{ item.reserved_g || item.reserved_units }}</text>
          <text v-if="item.shortage_g > 0 || item.shortage_units > 0" class="danger">缺口 {{ item.shortage_g || item.shortage_units }}</text>
        </view>
      </view>
      <button class="primary" :disabled="submitting || Boolean(submissionBlock)" @tap="submit">提交生产工单</button>
    </view>

    <view class="panel">
      <text class="title">工单列表</text>
      <view v-for="item in requests" :key="item.id" class="request">
        <view class="request-head"><text class="line-name">{{ item.request_no }}</text><text class="status">{{ productionStatusLabel(item.status) }}</text></view>
        <text class="hint">{{ item.created_at }}</text>
        <view v-for="target in item.items || []" :key="target.id || target.line_no" class="preview-row">
          <text>{{ target.product_name }} · {{ targetSpecLabel(target) }} · {{ targetQtyLabel(target) }}</text>
          <text class="hint">{{ productionStatusLabel(target.status) }}{{ target.work_order_no ? ` · ${target.work_order_no}` : '' }}</text>
        </view>
      </view>
      <text v-if="!loading && !requests.length" class="hint">暂无生产工单</text>
    </view>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.preview,.request,.preview-row,.line-copy{display:flex;flex-direction:column;gap:15rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}.title{font-size:30rpx;font-weight:900}.subtitle,.line-name{font-size:27rpx;font-weight:800}.hint{color:#707070;font-size:23rpx;line-height:1.5}.warning{display:block;padding:14rpx 16rpx;border-radius:8rpx;background:#fff4e5;color:#8a4b08;font-size:23rpx;line-height:1.5}.line,.request-head{display:flex;align-items:center;gap:12rpx}.line{padding:14rpx 0;border-top:1rpx solid #eee}.line-copy{flex:1;gap:4rpx}.qty{width:120rpx;min-height:72rpx;padding:0 16rpx;border:1rpx solid #ddd;border-radius:8rpx;box-sizing:border-box}.remove{margin:0;color:#a22;background:#fff;border:1rpx solid #e5caca}.textarea{min-height:112rpx;padding:16rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa;box-sizing:border-box}.primary,.secondary{min-height:76rpx;margin:0;border-radius:8rpx;font-size:25rpx}.primary{background:#2b2118;color:#fff}.secondary{background:#fff;border:1rpx solid #d8d8d8}.preview,.request{padding:16rpx;border:1rpx solid #eee;border-radius:8rpx}.preview-row{gap:6rpx;padding:12rpx 0;border-top:1rpx solid #eee}.preview-row.shortage{background:#fff6f4}.request-head{justify-content:space-between}.status{color:#28624a;font-weight:800}.danger,.error{color:#b42318}.error{display:block;padding:18rpx}
</style>
