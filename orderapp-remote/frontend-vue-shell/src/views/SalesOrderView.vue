<template>
  <div class="page" :class="{ 'embedded-page': props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>销售单</h2>
        <div class="actions">
          <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
          <a v-else class="secondary link-button" :href="appURL('/vue-shell?view=orders')">返回订单列表</a>
          <button class="secondary" type="button" @click="openSettingsDrawer">销售单设置</button>
          <button class="secondary" type="button" @click="openCustomerDrawer" :disabled="!customerSummary.id">客户信息</button>
          <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !orderID">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
          <a v-if="documents.length" class="secondary link-button" :href="salesOrderDownloadUrl(orderID)" target="_blank" rel="noopener">下载最新版 PDF</a>
          <a v-if="imageDocuments.length" class="secondary link-button" :href="salesOrderImageDownloadUrl(orderID)" target="_blank" rel="noopener">下载最新版图片</a>
          <button class="secondary" type="button" @click="shareLatestResource('sales_order_pdf')" :disabled="shareLoading || !orderID || !documents.length">{{ shareLoading === 'sales_order_pdf' ? '分享中' : '分享PDF到微信' }}</button>
          <button class="secondary" type="button" @click="shareLatestResource('sales_order_image')" :disabled="shareLoading || !orderID || !imageDocuments.length">{{ shareLoading === 'sales_order_image' ? '分享中' : '分享图片到微信' }}</button>
          <button class="primary" type="button" @click="generate" :disabled="generating || !orderID || !preview">{{ generating ? '生成中' : '确认生成 PDF' }}</button>
          <button class="primary" type="button" @click="generateImage" :disabled="imageGenerating || !orderID || !preview">{{ imageGenerating ? '生成图片中' : '确认生成图片' }}</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div class="summary">
        <span>订单 ID：{{ orderID || '-' }}</span>
        <span>客户：{{ customerSummary.name || '-' }}</span>
        <span>公司：{{ customerSummary.company_name || customerSummary.name || '-' }}</span>
        <span>PDF版本：{{ documents.length }}</span>
        <span>图片版本：{{ imageDocuments.length }}</span>
      </div>
      <details class="manual">
        <summary>销售单手册</summary>
        <ul>
          <li>首次生成销售单 PDF 或图片都从 V1 开始，同一订单再次生成会创建新版本，不覆盖旧文件。</li>
          <li>生成前先查看“销售单预览”；客户信息调整后可刷新预览，确认内容后再生成 PDF 或图片。</li>
          <li>公账收款信息在“公司设置”维护；预览会展示纳税人识别号、公司地址、户名、开户行和账号。</li>
          <li>客户公司地址过长时会自动换行；收款码会按数量自动放大或排列，减少空白区域。</li>
          <li>预览里的公章可直接拖动，松开后保存位置；位置只影响之后生成的新销售单。</li>
          <li>销售单内容按生成时的订单和设置保存快照，后续修改设置不会改动旧版本。</li>
          <li>需要给客户最新文件时使用“下载最新版 PDF”或“下载最新版图片”，需要追溯时下载指定历史版本。</li>
          <li>“分享到微信”会调起系统分享面板直接发送 PDF 或图片文件；浏览器不支持文件分享时，请下载最新版后手动发送。</li>
        </ul>
      </details>
    </section>

    <section class="panel sales-order-note-panel">
      <div class="panel-head">
        <h3>销售单备注</h3>
        <button class="secondary" type="button" @click="saveSalesOrderNote" :disabled="noteSaving || !orderID">{{ noteSaving ? '保存中' : '保存备注' }}</button>
      </div>
      <textarea v-model.trim="salesOrderNote" rows="2" placeholder="只显示在销售单最后一行，不影响订单列表内部备注"></textarea>
    </section>

    <section class="panel preview-panel">
      <div class="panel-head">
        <h3>销售单预览 <span v-if="preview" class="version-tag">V{{ preview.next_version_no }}</span></h3>
        <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !orderID">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
      </div>
      <div v-if="preview?.snapshot" class="preview-tools">
        <div class="layout-drag-hint">拖动“文字位置和大小”“收款码位置和大小”边框调整位置，拖右下角圆点调整大小。</div>
        <label v-if="preview?.snapshot?.seal" class="seal-size-slider">
          <span>公章大小</span>
          <input v-model.number="previewSealWidthMM" type="range" :min="salesOrderSealMinWidthMM" :max="salesOrderSealMaxWidthMM" step="1" :disabled="sealDragSaving" @change="savePreviewSealSize" />
          <output>{{ previewSealWidthMM }}mm</output>
        </label>
      </div>
      <div v-if="!preview" class="muted preview-empty">暂无预览</div>
      <PDFStampPreview
        v-else
        :pdf-url="salesOrderPreviewPDFUrl"
        :placements="salesOrderPreviewPlacements"
        :seal-url="previewSealUrl"
        seal-label="公章"
        preview-label="PREVIEW 预览版"
        @loaded="onPreviewPDFLoaded"
        @placement-commit="savePDFPreviewPlacement"
      />
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>PDF版本</h3>
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
          <tr v-if="!documents.length"><td colspan="6" class="muted">暂无销售单版本</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>图片版本</h3>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <table>
        <thead>
          <tr><th>版本</th><th>订单号</th><th>生成时间</th><th>操作人</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="doc in imageDocuments" :key="doc.id">
            <td>V{{ doc.version_no }}</td>
            <td>{{ doc.order_no }}</td>
            <td>{{ doc.created_at }}</td>
            <td>{{ doc.created_by || '-' }}</td>
            <td>{{ doc.is_latest ? '最新版' : '历史版本' }}</td>
            <td><a class="text-link" :href="doc.download_url" target="_blank" rel="noopener">下载图片</a></td>
          </tr>
          <tr v-if="!imageDocuments.length"><td colspan="6" class="muted">暂无销售单图片版本</td></tr>
        </tbody>
      </table>
    </section>

    <div v-if="drawerOpen" class="drawer-mask" @click.self="closeCustomerDrawer">
      <aside class="drawer" aria-label="客户信息">
        <div class="drawer-head">
          <h3>客户信息</h3>
          <button class="secondary" type="button" @click="closeCustomerDrawer">关闭</button>
        </div>
        <form class="drawer-form" @submit.prevent="saveCustomer">
          <label>
            <span>客户名</span>
            <input v-model.trim="customerForm.name" required />
          </label>
          <label>
            <span>公司名称</span>
            <input v-model.trim="customerForm.company_name" name="company_name" placeholder="不填则默认客户名" />
          </label>
          <label>
            <span>联系电话</span>
            <input v-model.trim="customerForm.company_phone" name="company_phone" />
          </label>
          <label>
            <span>联系人</span>
            <input v-model.trim="customerForm.contact" />
          </label>
          <label>
            <span>收货电话</span>
            <input v-model.trim="customerForm.phone" />
          </label>
          <label>
            <span>公司地址</span>
            <textarea v-model.trim="customerForm.company_address" name="company_address" rows="3"></textarea>
          </label>
          <label>
            <span>收货地址</span>
            <textarea v-model.trim="customerForm.address" rows="3"></textarea>
          </label>
          <div class="drawer-actions">
            <button class="primary" type="submit" :disabled="savingCustomer">{{ savingCustomer ? '保存中' : '保存客户信息' }}</button>
          </div>
        </form>
      </aside>
    </div>

    <div v-if="settingsDrawerOpen" class="drawer-mask settings-drawer-mask" @click.self="closeSettingsDrawer">
      <aside class="drawer settings-drawer" aria-label="销售单设置">
        <div class="drawer-head">
          <h3>销售单设置</h3>
          <button class="secondary" type="button" @click="closeSettingsDrawer">关闭</button>
        </div>
        <SalesOrderSettingsView embedded />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend, appURL } from '../api/client'
import { salesOrderDownloadUrl, salesOrderImageDownloadUrl } from '../lib/sales-order'
import {
  pdfPlacementToSalesLayoutBox,
  pdfPlacementToSalesSealMM,
  salesLayoutBoxMMToPDFPlacement,
  salesSealMMToPDFPlacement,
} from '../lib/document-pdf-stamp'
import { salesOrderSealMaxWidthMM, salesOrderSealMinWidthMM } from '../lib/sales-order-seal'
import { buildShareResourcePayload, shareResourceToWechat } from '../lib/external-share'
import PDFStampPreview from '../components/PDFStampPreview.vue'
import SalesOrderSettingsView from './SalesOrderSettingsView.vue'

const props = defineProps({
  orderId: { type: [Number, String], default: 0 },
  embedded: { type: Boolean, default: false },
})

const emit = defineEmits(['close'])

const loading = ref(false)
const previewLoading = ref(false)
const generating = ref(false)
const imageGenerating = ref(false)
const error = ref('')
const message = ref('')
const documents = ref([])
const imageDocuments = ref([])
const preview = ref(null)
const drawerOpen = ref(false)
const settingsDrawerOpen = ref(false)
const savingCustomer = ref(false)
const noteSaving = ref(false)
const sealDragSaving = ref(false)
const layoutDragSaving = ref(false)
const shareLoading = ref('')
const previewPDFPages = ref([])
const previewPDFRefreshKey = ref(0)
const previewSealAspectRatio = ref(1)
const customerSummary = reactive(emptyCustomer())
const customerForm = reactive(emptyCustomer())
const salesOrderNote = ref('')

const orderID = computed(() => Number(props.orderId || new URL(window.location.href).searchParams.get('order_id') || 0))
const salesOrderPreviewBasePDFUrl = computed(() => orderID.value ? `/api/orders/${orderID.value}/sales-order-preview.pdf` : '')
const salesOrderPreviewPDFUrl = computed(() => salesOrderPreviewBasePDFUrl.value ? `${salesOrderPreviewBasePDFUrl.value}?v=${previewPDFRefreshKey.value}` : '')
const previewSealUrl = computed(() => assetURL(preview.value?.snapshot?.seal || {}))
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
const salesOrderPreviewPlacements = computed(() => {
  const snapshot = preview.value?.snapshot
  const page = previewPDFPages.value[0]
  if (!snapshot || !page) return []
  const placements = []
  if (snapshot.payment_text_box) {
    placements.push(salesLayoutBoxMMToPDFPlacement(snapshot.payment_text_box, page, {
      kind: 'payment_text',
      label: '文字位置和大小',
      resizable: true,
      use_seal_image: false,
      min_width: 80,
      min_height: 36,
    }))
  }
  if (snapshot.payment_code_box) {
    placements.push(salesLayoutBoxMMToPDFPlacement(snapshot.payment_code_box, page, {
      kind: 'payment_code',
      label: '收款码位置和大小',
      resizable: true,
      use_seal_image: false,
      min_width: 80,
      min_height: 80,
    }))
  }
  if (snapshot.seal) {
    placements.push(salesSealMMToPDFPlacement(snapshot.seal, page, {
      kind: 'seal',
      label: '公章',
      resizable: false,
      sealAspectRatio: previewSealAspectRatio.value,
    }))
  }
  return placements
})

let previewSealAspectToken = 0
watch(previewSealUrl, loadPreviewSealAspectRatio, { immediate: true })

async function load() {
  if (!orderID.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/orders/${orderID.value}/sales-orders`)
    documents.value = data.rows || []
    imageDocuments.value = data.image_rows || []
    assignCustomer(customerSummary, data.order?.customer || {})
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

async function loadPreview() {
  if (!orderID.value) return
  previewLoading.value = true
  error.value = ''
  try {
    preview.value = await apiGet(`/api/orders/${orderID.value}/sales-order-preview`)
    salesOrderNote.value = preview.value?.snapshot?.sales_order_note || ''
    previewPDFRefreshKey.value += 1
  } catch (err) {
    preview.value = null
    error.value = err.message || '加载销售单预览失败'
  } finally {
    previewLoading.value = false
  }
}

async function saveSalesOrderNote() {
  if (!orderID.value) return
  noteSaving.value = true
  error.value = ''
  message.value = ''
  try {
    preview.value = await apiSend(`/api/orders/${orderID.value}/sales-order-note`, {
      method: 'PUT',
      body: { note: salesOrderNote.value },
    })
    salesOrderNote.value = preview.value?.snapshot?.sales_order_note || ''
    previewPDFRefreshKey.value += 1
    message.value = '销售单备注已保存，请重新生成 PDF 或图片后下载'
  } catch (err) {
    error.value = err.message || '保存销售单备注失败'
  } finally {
    noteSaving.value = false
  }
}

async function generate() {
  if (!orderID.value || !preview.value) return
  generating.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/orders/${orderID.value}/sales-orders`)
    message.value = `已生成 V${data.version_no}`
    await load()
    await loadPreview()
  } catch (err) {
    error.value = err.message || '生成失败'
  } finally {
    generating.value = false
  }
}

async function generateImage() {
  if (!orderID.value || !preview.value) return
  imageGenerating.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/orders/${orderID.value}/sales-order-images`)
    message.value = `已生成图片 V${data.version_no}`
    await load()
    await loadPreview()
  } catch (err) {
    error.value = err.message || '生成图片失败'
  } finally {
    imageGenerating.value = false
  }
}

async function shareLatestResource(resourceType) {
  if (!orderID.value || shareLoading.value) return
  shareLoading.value = resourceType
  error.value = ''
  message.value = ''
  try {
    const share = await apiSend('/api/share-resources', {
      body: buildShareResourcePayload(resourceType, orderID.value),
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
    shareLoading.value = ''
  }
}

function emptyCustomer() {
  return {
    id: 0,
    name: '',
    raw_name: '',
    company_name: '',
    company_address: '',
    company_phone: '',
    contact: '',
    phone: '',
    address: '',
    default_source_id: null,
    default_order_type_id: null,
    active: true,
  }
}

function assignCustomer(target, data = {}) {
  Object.assign(target, {
    id: Number(data.id || 0),
    name: data.name || '',
    raw_name: data.raw_name || '',
    company_name: data.company_name || '',
    company_address: data.company_address || '',
    company_phone: data.company_phone || '',
    contact: data.contact || '',
    phone: data.phone || '',
    address: data.address || '',
    default_source_id: data.default_source_id || null,
    default_order_type_id: data.default_order_type_id || null,
    active: data.active !== false,
  })
}

function assetURL(ref = {}) {
  if (ref.url) return ref.url
  return ref.object_key ? `/assets/${ref.object_key}` : ''
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
    message.value = '公章位置已保存，请重新生成图片或 PDF 后下载'
  } catch (err) {
    error.value = err.message || '保存公章位置失败'
  } finally {
    sealDragSaving.value = false
  }
}

async function savePDFPreviewPlacement(placement) {
  if (placement?.kind === 'payment_text' || placement?.kind === 'payment_code') {
    await savePDFPreviewLayoutBox(placement)
    return
  }
  await savePDFPreviewSealPosition(placement)
}

async function savePDFPreviewLayoutBox(placement) {
  const snapshot = preview.value?.snapshot
  const page = previewPDFPages.value.find((item) => Number(item.pageNumber) === Number(placement?.page_number || 1))
  if (!snapshot || !page || layoutDragSaving.value) return
  const nextBox = pdfPlacementToSalesLayoutBox(placement, page)
  if (placement.kind === 'payment_text') {
    snapshot.payment_text_box = nextBox
  } else if (placement.kind === 'payment_code') {
    snapshot.payment_code_box = nextBox
  } else {
    return
  }
  layoutDragSaving.value = true
  error.value = ''
  try {
    const textBox = normalizeLayoutBox(snapshot.payment_text_box, {
      x_mm: 16,
      y_mm: 118,
      width_mm: 104,
      height_mm: 78,
    })
    const codeBox = normalizeLayoutBox(snapshot.payment_code_box, {
      x_mm: 126,
      y_mm: 106,
      width_mm: 72,
      height_mm: 122,
    })
    await apiSend('/api/settings/sales-order/payment-layout', {
      body: {
        payment_text_x_mm: textBox.x_mm,
        payment_text_y_mm: textBox.y_mm,
        payment_text_width_mm: textBox.width_mm,
        payment_text_height_mm: textBox.height_mm,
        payment_code_x_mm: codeBox.x_mm,
        payment_code_y_mm: codeBox.y_mm,
        payment_code_width_mm: codeBox.width_mm,
        payment_code_height_mm: codeBox.height_mm,
      },
    })
    previewPDFRefreshKey.value += 1
    message.value = '销售单文字和收款码版式已保存，请重新生成图片或 PDF 后下载'
  } catch (err) {
    error.value = err.message || '保存销售单版式失败'
  } finally {
    layoutDragSaving.value = false
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
    message.value = '公章大小已保存，请重新生成图片或 PDF 后下载'
  } catch (err) {
    error.value = err.message || '保存公章大小失败'
  } finally {
    sealDragSaving.value = false
  }
}

function normalizeLayoutBox(box = {}, fallback = {}) {
  return {
    x_mm: Math.round(Number(box.x_mm ?? fallback.x_mm ?? 0)),
    y_mm: Math.round(Number(box.y_mm ?? fallback.y_mm ?? 0)),
    width_mm: Math.round(Number(box.width_mm ?? fallback.width_mm ?? 1)),
    height_mm: Math.round(Number(box.height_mm ?? fallback.height_mm ?? 1)),
  }
}

async function openCustomerDrawer() {
  if (!customerSummary.id) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/customers/${customerSummary.id}`)
    assignCustomer(customerForm, data.customer || {})
    drawerOpen.value = true
  } catch (err) {
    error.value = err.message || '加载客户信息失败'
  } finally {
    loading.value = false
  }
}

function closeCustomerDrawer() {
  drawerOpen.value = false
}

function openSettingsDrawer() {
  settingsDrawerOpen.value = true
}

async function closeSettingsDrawer() {
  settingsDrawerOpen.value = false
  await loadPreview()
}

async function saveCustomer() {
  if (!customerForm.id) return
  savingCustomer.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/customers/${customerForm.id}`, {
      method: 'PUT',
      body: {
        name: customerForm.name,
        raw_name: customerForm.raw_name,
        company_name: customerForm.company_name,
        company_address: customerForm.company_address,
        company_phone: customerForm.company_phone,
        contact: customerForm.contact,
        phone: customerForm.phone,
        address: customerForm.address,
        default_source_id: customerForm.default_source_id,
        default_order_type_id: customerForm.default_order_type_id,
        active: customerForm.active,
      },
    })
    assignCustomer(customerForm, data.customer || {})
    assignCustomer(customerSummary, data.customer || {})
    message.value = '客户信息已保存'
    await loadPreview()
  } catch (err) {
    error.value = err.message || '保存客户信息失败'
  } finally {
    savingCustomer.value = false
  }
}

onMounted(loadPage)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.embedded-page { padding: 14px; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 1180px; }
.embedded-page .panel { max-width: none; }
.preview-panel { overflow: auto; }
.panel-head, .actions, .summary { display: flex; align-items: center; gap: 12px; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
.sales-order-note-panel textarea { width: 100%; min-height: 76px; resize: vertical; border: 1px solid #d6cec3; border-radius: 6px; padding: 10px 12px; font: inherit; line-height: 1.6; background: #fff; }
.actions { flex-wrap: wrap; justify-content: flex-end; }
.preview-tools { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin: -4px 0 12px; }
.layout-drag-hint { color: #4b5563; font-size: 13px; line-height: 1.5; }
.seal-size-slider { min-width: min(340px, 100%); display: grid; grid-template-columns: auto minmax(160px, 1fr) auto; gap: 10px; align-items: center; color: #4b5563; font-size: 13px; }
.seal-size-slider input { width: 100%; accent-color: #1f1f1f; }
.seal-size-slider output { min-width: 46px; text-align: right; color: #171717; font-variant-numeric: tabular-nums; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
button, .link-button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 8px 12px; font: inherit; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.summary { color: #555; }
.manual { border-top: 1px solid #edf0f5; padding-top: 10px; margin-top: 12px; color: #4b5563; font-size: 13px; }
.manual summary { cursor: pointer; font-weight: 700; color: #111827; }
.manual ul { margin: 8px 0 0; padding-left: 18px; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; }
th { background: #fbfaf8; }
.text-link { color: #1f4f82; text-decoration: none; }
.muted { color: #666; text-align: center; }
.version-tag { display: inline-block; margin-left: 6px; border: 1px solid #d0d0d0; border-radius: 999px; padding: 1px 7px; color: #555; font-size: 12px; vertical-align: middle; }
.preview-empty { padding: 18px 0; }
.preview-box { width: 1240px; min-height: 1754px; border: 1px solid #e5ded3; border-radius: 8px; padding: 0; background: #fff; position: relative; color: #171717; }
.preview-title { display: flex; justify-content: space-between; align-items: flex-start; gap: 24px; height: 58px; border-bottom: 2px solid #1f1f1f; padding: 0 0 28px; margin: 62px 70px 42px; }
.preview-title strong { font-size: 24px; line-height: 28px; font-weight: 700; }
.preview-title span { font-size: 22px; line-height: 28px; font-weight: 500; }
.seal-stamp-preview { position: absolute; object-fit: contain; opacity: .86; cursor: move; touch-action: none; z-index: 2; }
.preview-meta { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px 14px; margin: 0 70px 30px; color: #555; font-size: 20px; line-height: 28px; }
.preview-meta span { min-width: 0; overflow-wrap: anywhere; line-height: 28px; }
.preview-box table { width: calc(100% - 140px); margin: 0 70px; table-layout: fixed; }
.preview-box th, .preview-box td { font-size: 20px; padding: 16px 8px; }
.preview-box th { font-size: 21px; }
.preview-total { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 22px; margin: 0 70px; padding: 20px 0 18px; border-bottom: 2px solid #1f1f1f; font-size: 22px; }
.preview-total strong { font-size: 22px; }
.preview-notes { border-top: 1px solid #eee2d4; margin-top: 12px; padding-top: 10px; color: #555; }
.preview-notes p { margin: 4px 0; white-space: pre-line; }
.payment-info-grid { display: grid; grid-template-columns: minmax(0, 650px) minmax(260px, 360px); gap: 18px; margin: 28px 70px 0; padding-top: 0; }
.payment-text-panel { display: grid; gap: 10px; align-content: start; }
.payment-code-panel { align-self: start; }
.compact-notes { border-top: 0; margin-top: 0; padding-top: 0; }
.account-payment-preview p { margin: 2px 0; }
.payment-code-preview-list { display: grid; grid-template-columns: 1fr; justify-content: stretch; gap: 14px; }
.payment-code-preview { display: grid; grid-template-columns: 132px minmax(0, 1fr); gap: 12px; align-items: center; justify-items: start; text-align: left; }
.payment-code-preview img { width: 132px; height: 132px; object-fit: contain; border: 1px solid #eee2d4; border-radius: 6px; background: #fff; }
.single-payment-code .payment-code-preview { grid-template-columns: 1fr; justify-items: center; text-align: center; }
.single-payment-code .payment-code-preview img { width: 168px; height: 168px; }
.payment-code-preview strong, .payment-code-preview span { display: block; }
.payment-code-preview span { color: #666; margin-top: 4px; white-space: pre-line; font-size: 12px; line-height: 1.35; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
.drawer-mask { position: fixed; inset: 0; z-index: 40; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.drawer { width: min(520px, 100vw); height: 100%; overflow: auto; background: #fff; border-left: 1px solid #e6e0d8; padding: 16px; box-shadow: -10px 0 24px rgba(0, 0, 0, .12); }
.settings-drawer-mask { z-index: 60; }
.settings-drawer { width: min(760px, 100vw); background: #f8f7f4; }
.drawer-head, .drawer-actions { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 14px; }
.drawer-form { display: grid; gap: 12px; }
.drawer-form label span { display: block; color: #555; font-size: 12px; margin-bottom: 5px; }
.drawer-form input, .drawer-form textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 8px 10px; font: inherit; background: #fff; }
.drawer-form input { height: 38px; }
.drawer-form textarea { min-height: 82px; resize: vertical; }
.drawer-actions { justify-content: flex-end; margin: 4px 0 0; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: stretch; flex-direction: column; }
  .actions { justify-content: flex-start; }
  .preview-tools { justify-content: flex-start; }
  .seal-size-slider { grid-template-columns: 1fr; }
  table { min-width: 760px; }
  .panel { overflow: auto; }
  .preview-meta { grid-template-columns: 1fr; }
  .payment-info-grid { grid-template-columns: 1fr; }
  .drawer { width: 100vw; }
}
</style>
