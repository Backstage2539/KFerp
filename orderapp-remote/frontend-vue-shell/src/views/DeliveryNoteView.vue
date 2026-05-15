<template>
  <div class="delivery-note-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>出库单</h2>
        <div class="actions">
          <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
          <a v-else class="secondary link-button" :href="appURL('/vue-shell?view=orders')">返回订单列表</a>
          <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !orderID">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
          <a v-if="documents.length" class="secondary link-button" :href="deliveryNoteDownloadUrl(orderID)" target="_blank" rel="noopener">下载最新版</a>
          <button class="secondary" type="button" @click="openSettingsDrawer">公章设置</button>
          <button class="secondary" type="button" @click="shareDeliveryNote" :disabled="shareLoading || !orderID || !documents.length">{{ shareLoading ? '分享中' : '分享到微信' }}</button>
          <button class="primary" type="button" @click="confirmGenerateDeliveryNote" :disabled="generating || !orderID || !preview">{{ generating ? '生成中' : '确认生成 PDF' }}</button>
        </div>
      </div>
      <div v-if="error" class="notice error">{{ error }}</div>
      <div v-if="message" class="notice ok">{{ message }}</div>
      <div class="summary">
        <span>订单 ID：{{ orderID || '-' }}</span>
        <span>订单号：{{ orderSummary.order_no || '-' }}</span>
        <span>客户：{{ orderSummary.customer?.name || '-' }}</span>
        <span>发货状态：{{ orderSummary.ship_status || '-' }}</span>
        <span>版本数：{{ documents.length }}</span>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>出库维护</h3>
        <button class="primary" type="button" @click="saveForm" :disabled="saving || !orderID">{{ saving ? '保存中' : '保存出库信息' }}</button>
      </div>
      <form class="form-grid" @submit.prevent="saveForm">
        <label>
          <span>出库日期</span>
          <input v-model.trim="form.posting_date" type="text" inputmode="numeric" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>出库仓库</span>
          <select v-model="form.source_warehouse">
            <option v-for="item in warehouseOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
        <label>
          <span>发货方式</span>
          <input v-model.trim="form.delivery_method" />
        </label>
        <label>
          <span>快递单号（可多个）</span>
          <textarea v-model.trim="form.tracking_no" rows="2" placeholder="多个单号可用换行、逗号或分号分隔"></textarea>
        </label>
        <label class="wide">
          <span>备注</span>
          <textarea v-model.trim="form.note" rows="3"></textarea>
        </label>
      </form>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>出库单预览 <span v-if="preview" class="version-tag">V{{ preview.next_version_no }}</span></h3>
        <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !orderID">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
      </div>
      <div v-if="preview?.snapshot?.seal" class="preview-tools">
        <label class="seal-size-slider">
          <span>公章大小</span>
          <input v-model.number="previewSealWidthMM" type="range" :min="salesOrderSealMinWidthMM" :max="salesOrderSealMaxWidthMM" step="1" :disabled="sealDragSaving" @change="savePreviewSealSize" />
          <output>{{ previewSealWidthMM }}mm</output>
        </label>
      </div>
      <div v-if="!preview" class="muted preview-empty">暂无预览</div>
      <PDFStampPreview
        v-else
        :pdf-url="deliveryNotePreviewPDFUrl"
        :placements="deliveryNotePreviewPlacements"
        :seal-url="previewSealUrl"
        seal-label="公章"
        preview-label="PREVIEW 预览版"
        @loaded="onPreviewPDFLoaded"
        @placement-commit="savePDFPreviewSealPosition"
      />
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>历史版本</h3>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <table>
        <thead>
          <tr><th>版本</th><th>订单号</th><th>生成时间</th><th>操作人</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="doc in documents" :key="doc.id">
            <td>V{{ doc.version_no }}</td>
            <td>{{ doc.order_no }}</td>
            <td>{{ doc.created_at }}</td>
            <td>{{ doc.created_by || '-' }}</td>
            <td>{{ doc.is_latest ? '最新版' : '历史版本' }}</td>
            <td><a class="text-link" :href="doc.download_url" target="_blank" rel="noopener">下载</a></td>
          </tr>
          <tr v-if="!documents.length"><td colspan="6" class="muted">暂无出库单版本</td></tr>
        </tbody>
      </table>
    </section>

    <div v-if="settingsDrawerOpen" class="settings-drawer-mask" @click.self="closeSettingsDrawer">
      <aside class="settings-drawer" aria-label="公章设置">
        <CompanySealSettingsView embedded @close="closeSettingsDrawer" @updated="loadPreview" />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend, appURL } from '../api/client'
import { deliveryNoteDownloadUrl } from '../lib/delivery-note'
import { buildShareResourcePayload, shareResourceToWechat } from '../lib/external-share'
import { pdfPlacementToSalesSealMM, salesSealMMToPDFPlacement } from '../lib/document-pdf-stamp'
import { salesOrderSealMaxWidthMM, salesOrderSealMinWidthMM } from '../lib/sales-order-seal'
import PDFStampPreview from '../components/PDFStampPreview.vue'
import CompanySealSettingsView from './CompanySealSettingsView.vue'

const props = defineProps({
  orderId: { type: [Number, String], default: 0 },
  embedded: { type: Boolean, default: false },
})

const emit = defineEmits(['close'])

const loading = ref(false)
const previewLoading = ref(false)
const saving = ref(false)
const generating = ref(false)
const error = ref('')
const message = ref('')
const documents = ref([])
const preview = ref(null)
const shareLoading = ref(false)
const settingsDrawerOpen = ref(false)
const sealDragSaving = ref(false)
const previewPDFPages = ref([])
const previewPDFRefreshKey = ref(0)
const previewSealAspectRatio = ref(1)
const orderSummary = reactive({ order_id: 0, order_no: '', ship_status: '', customer: {} })
const form = reactive(emptyForm())

const warehouseOptions = [
  { value: 'finished_goods', label: '成品仓' },
  { value: 'finished_shop', label: '门店成品仓' },
]

const orderID = computed(() => Number(props.orderId || new URL(window.location.href).searchParams.get('order_id') || 0))
const deliveryNotePreviewBasePDFUrl = computed(() => orderID.value ? `/api/orders/${orderID.value}/delivery-note-preview.pdf` : '')
const deliveryNotePreviewPDFUrl = computed(() => deliveryNotePreviewBasePDFUrl.value ? `${deliveryNotePreviewBasePDFUrl.value}?v=${previewPDFRefreshKey.value}` : '')
const previewSealUrl = computed(() => preview.value?.snapshot?.seal?.url || '')
const previewSealWidthMM = computed({
  get() {
    return clampSealWidthMM(preview.value?.snapshot?.seal?.width_mm ?? preview.value?.snapshot?.seal?.seal_width_mm ?? 36)
  },
  set(value) {
    const seal = preview.value?.snapshot?.seal
    if (!seal) return
    const width = clampSealWidthMM(value)
    seal.width_mm = width
    seal.seal_width_mm = width
  },
})
const deliveryNotePreviewPlacements = computed(() => {
  const seal = preview.value?.snapshot?.seal
  const page = previewPDFPages.value[0]
  if (!seal || !page) return []
  return [salesSealMMToPDFPlacement(seal, page, { sealAspectRatio: previewSealAspectRatio.value })]
})

let previewSealAspectToken = 0
watch(previewSealUrl, loadPreviewSealAspectRatio, { immediate: true })

function emptyForm() {
  return {
    posting_date: '',
    source_warehouse: 'finished_goods',
    delivery_method: '',
    tracking_no: '',
    note: '',
  }
}

function assignForm(data = {}) {
  Object.assign(form, {
    posting_date: data.posting_date || '',
    source_warehouse: data.source_warehouse || 'finished_goods',
    delivery_method: data.delivery_method || '',
    tracking_no: data.tracking_no || '',
    note: data.note || '',
  })
}

function assignOrder(data = {}) {
  Object.assign(orderSummary, {
    order_id: Number(data.order_id || 0),
    order_no: data.order_no || '',
    ship_status: data.ship_status || '',
    customer: data.customer || {},
  })
}

async function load() {
  if (!orderID.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/orders/${orderID.value}/delivery-notes`)
    documents.value = data.rows || []
    assignOrder(data.order || {})
    assignForm(data.form || {})
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadPage() {
  await load()
  await loadPreview()
}

async function saveForm() {
  if (!orderID.value) return
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/orders/${orderID.value}/delivery-note`, {
      body: {
        posting_date: form.posting_date,
        source_warehouse: form.source_warehouse,
        delivery_method: form.delivery_method,
        tracking_no: form.tracking_no,
        note: form.note,
      },
    })
    assignForm(data || {})
    message.value = '出库信息已保存'
    await loadPreview()
  } catch (err) {
    error.value = err.message || '保存出库信息失败'
  } finally {
    saving.value = false
  }
}

async function loadPreview() {
  if (!orderID.value) return
  previewLoading.value = true
  error.value = ''
  try {
    preview.value = await apiGet(`/api/orders/${orderID.value}/delivery-note-preview`)
    previewPDFRefreshKey.value += 1
  } catch (err) {
    preview.value = null
    error.value = err.message || '加载出库单预览失败'
  } finally {
    previewLoading.value = false
  }
}

async function confirmGenerateDeliveryNote() {
  if (!orderID.value || !preview.value) return
  generating.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/orders/${orderID.value}/delivery-notes`)
    message.value = `已生成 V${data.version_no}`
    await load()
    await loadPreview()
  } catch (err) {
    error.value = err.message || '生成失败'
  } finally {
    generating.value = false
  }
}

async function shareDeliveryNote() {
  if (!orderID.value || shareLoading.value) return
  shareLoading.value = true
  error.value = ''
  message.value = ''
  try {
    const share = await apiSend('/api/share-resources', {
      body: buildShareResourcePayload('delivery_note_pdf', orderID.value),
    })
    const result = await shareResourceToWechat(share)
    if (result === 'file-shared') {
      message.value = '已打开系统分享面板，请选择微信发送文件'
    } else if (result === 'unsupported') {
      message.value = '当前浏览器不支持直接分享文件，请下载最新版后手动发送到微信'
    } else {
      message.value = '无法直接分享文件，请下载最新版后手动发送到微信'
    }
    await load()
  } catch (err) {
    error.value = err.message || '分享到微信失败'
  } finally {
    shareLoading.value = false
  }
}

function openSettingsDrawer() {
  settingsDrawerOpen.value = true
}

async function closeSettingsDrawer() {
  settingsDrawerOpen.value = false
  await loadPreview()
}

function onPreviewPDFLoaded(pages) {
  previewPDFPages.value = pages || []
}

function loadPreviewSealAspectRatio(url) {
  previewSealAspectRatio.value = 1
  const token = ++previewSealAspectToken
  if (!url || typeof Image === 'undefined') return
  const img = new Image()
  img.onload = () => {
    if (token !== previewSealAspectToken) return
    if (img.naturalWidth > 0 && img.naturalHeight > 0) {
      previewSealAspectRatio.value = img.naturalHeight / img.naturalWidth
    }
  }
  img.onerror = () => {
    if (token === previewSealAspectToken) previewSealAspectRatio.value = 1
  }
  img.src = url
}

function clampSealWidthMM(value) {
  const n = Number(value)
  const width = Number.isFinite(n) && n > 0 ? Math.round(n) : 36
  return Math.min(salesOrderSealMaxWidthMM, Math.max(salesOrderSealMinWidthMM, width))
}

function currentSealPositionPayload() {
  const seal = preview.value?.snapshot?.seal || {}
  return {
    seal_x_mm: Math.round(Number(seal.x_mm ?? seal.seal_x_mm ?? 32)),
    seal_y_mm: Math.round(Number(seal.y_mm ?? seal.seal_y_mm ?? 5)),
    seal_width_mm: clampSealWidthMM(seal.width_mm ?? seal.seal_width_mm ?? 36),
  }
}

async function savePDFPreviewSealPosition(placement) {
  const seal = preview.value?.snapshot?.seal
  const page = previewPDFPages.value.find((item) => Number(item.pageNumber) === Number(placement?.page_number || 1))
  if (!seal || !page || sealDragSaving.value) return
  const next = pdfPlacementToSalesSealMM(placement, page)
  sealDragSaving.value = true
  error.value = ''
  try {
    await apiSend('/api/settings/sales-order/seal-position', {
      body: next,
    })
    seal.x_mm = next.seal_x_mm
    seal.seal_x_mm = next.seal_x_mm
    seal.y_mm = next.seal_y_mm
    seal.seal_y_mm = next.seal_y_mm
    seal.width_mm = next.seal_width_mm
    seal.seal_width_mm = next.seal_width_mm
    message.value = '公章位置已保存，请重新生成出库单 PDF 后下载'
  } catch (err) {
    error.value = err.message || '保存公章位置失败'
  } finally {
    sealDragSaving.value = false
  }
}

async function savePreviewSealSize() {
  const seal = preview.value?.snapshot?.seal
  if (!seal || sealDragSaving.value) return
  const next = currentSealPositionPayload()
  sealDragSaving.value = true
  error.value = ''
  try {
    await apiSend('/api/settings/sales-order/seal-position', {
      body: next,
    })
    seal.x_mm = next.seal_x_mm
    seal.seal_x_mm = next.seal_x_mm
    seal.y_mm = next.seal_y_mm
    seal.seal_y_mm = next.seal_y_mm
    seal.width_mm = next.seal_width_mm
    seal.seal_width_mm = next.seal_width_mm
    message.value = '公章大小已保存，请重新生成出库单 PDF 后下载'
  } catch (err) {
    error.value = err.message || '保存公章大小失败'
  } finally {
    sealDragSaving.value = false
  }
}

onMounted(loadPage)
</script>

<style scoped>
* { box-sizing: border-box; }
.delivery-note-page { padding: 18px; color: #171717; display: grid; gap: 16px; }
.delivery-note-page.embedded { padding: 14px; }
.panel { background: #fff; border: 1px solid #e6e0d8; border-radius: 8px; padding: 16px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.preview-tools { display: flex; justify-content: flex-end; margin: -4px 0 12px; }
.seal-size-slider { min-width: min(340px, 100%); display: grid; grid-template-columns: auto minmax(160px, 1fr) auto; gap: 10px; align-items: center; color: #4b5563; font-size: 13px; }
.seal-size-slider input { width: 100%; accent-color: #1f6f4a; }
.seal-size-slider output { min-width: 46px; text-align: right; color: #171717; font-variant-numeric: tabular-nums; }
h2, h3 { margin: 0; }
h2 { font-size: 22px; }
h3 { font-size: 17px; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
button, .link-button { border: 1px solid #d7cec3; border-radius: 6px; background: #fff; padding: 8px 12px; font-size: 14px; color: #171717; text-decoration: none; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.primary { background: #1f6f4a; border-color: #1f6f4a; color: #fff; }
.secondary { background: #fff; }
.text-link { color: #1f6f4a; text-decoration: none; font-weight: 600; }
.summary { display: flex; flex-wrap: wrap; gap: 10px; color: #555; font-size: 13px; }
.summary span { background: #f8f7f4; border: 1px solid #eee7df; border-radius: 6px; padding: 5px 8px; }
.form-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
label { display: grid; gap: 6px; font-size: 13px; color: #555; }
input, select, textarea { width: 100%; border: 1px solid #d8d1c8; border-radius: 6px; padding: 9px 10px; font: inherit; background: #fff; color: #171717; }
textarea { resize: vertical; }
.wide { grid-column: 1 / -1; }
.preview-empty { padding: 20px 0; text-align: center; }
.preview-box { border: 1px solid #e3dccf; border-radius: 8px; padding: 18px; background: #fffdf9; }
.preview-title { position: relative; display: flex; justify-content: space-between; gap: 12px; align-items: baseline; border-bottom: 1px solid #d8d1c8; padding-bottom: 10px; margin-bottom: 12px; min-height: 46px; }
.preview-title strong { font-size: 20px; }
.preview-title span { font-size: 16px; font-weight: 600; }
.seal-stamp-preview { position: absolute; object-fit: contain; opacity: .86; cursor: move; touch-action: none; z-index: 1; }
.preview-meta { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px 14px; font-size: 13px; color: #3d3d3d; margin-bottom: 12px; }
table { width: 100%; border-collapse: collapse; font-size: 14px; }
th, td { border-bottom: 1px solid #eee7df; padding: 9px 8px; text-align: left; vertical-align: top; }
th { color: #555; background: #faf8f4; font-weight: 600; }
.preview-note { margin-top: 12px; display: grid; gap: 5px; color: #333; }
.preview-note p { margin: 0; white-space: pre-wrap; }
.version-tag { color: #1f6f4a; font-size: 13px; }
.notice { padding: 10px 12px; border-radius: 7px; margin-bottom: 12px; }
.notice.ok { background: #eef8f1; border: 1px solid #cfe8d4; color: #1f6f4a; }
.notice.error { background: #fff1f1; border: 1px solid #f0caca; color: #9d1c1c; }
.muted { color: #888; }
.settings-drawer-mask { position: fixed; inset: 0; z-index: 45; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.settings-drawer { width: min(720px, calc(100vw - 28px)); height: 100%; overflow: auto; background: #f8f7f4; border-left: 1px solid #e6e0d8; box-shadow: -10px 0 24px rgba(0, 0, 0, .14); }
@media (max-width: 900px) {
  .delivery-note-page { padding: 12px; }
  .panel-head, .actions { align-items: stretch; flex-direction: column; }
  .preview-tools { justify-content: flex-start; }
  .seal-size-slider { grid-template-columns: 1fr; }
  .form-grid, .preview-meta { grid-template-columns: 1fr; }
  .settings-drawer { width: 100vw; }
}
</style>
