<template>
  <div class="invoice-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>发票</h2>
          <p>订单申请发票后，开票完成即可上传 PDF 或图片文件。</p>
        </div>
        <div class="actions">
          <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
          <a v-else class="secondary link-button" :href="appURL('/vue-shell?view=orders')">返回订单列表</a>
          <button class="secondary" type="button" @click="load" :disabled="loading || !orderID">{{ loading ? '刷新中' : '刷新' }}</button>
          <button class="primary" type="button" @click="requestInvoice" :disabled="requesting || !orderID || invoice.status === 'uploaded'">
            {{ requesting ? '申请中' : invoice.status ? '重新标记申请' : '申请发票' }}
          </button>
        </div>
      </div>
      <div v-if="error" class="notice error">{{ error }}</div>
      <div v-if="message" class="notice ok">{{ message }}</div>
      <div class="summary">
        <span>订单 ID：{{ orderID || '-' }}</span>
        <span>订单号：{{ invoice.order_no || '-' }}</span>
        <span :class="['status-pill', invoiceStatusTone(invoice.status)]">发票：{{ invoiceStatusLabel(invoice.status) }}</span>
        <span>申请人：{{ invoice.requested_by || '-' }}</span>
        <span>申请时间：{{ invoice.requested_at || '-' }}</span>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>发票文件</h3>
        <a v-if="invoice.asset?.url" class="secondary link-button" :href="invoice.asset.url" target="_blank" rel="noopener">打开文件</a>
      </div>
      <div class="file-current">
        <strong>{{ orderInvoiceAssetName(invoice.asset) }}</strong>
        <span v-if="invoice.asset">{{ invoice.asset.content_type }} · {{ formatBytes(invoice.asset.bytes) }} · {{ invoice.uploaded_at || '-' }}</span>
        <span v-else>支持 PDF、PNG、JPG、GIF、WebP</span>
      </div>
      <div class="upload-row">
        <label>
          <span>选择发票文件</span>
          <input type="file" :accept="orderInvoiceFileAccept" @change="handleFileChange" />
        </label>
        <button class="primary" type="button" @click="uploadInvoiceFile" :disabled="uploading || !selectedFile">
          {{ uploading ? '上传中' : '上传发票文件' }}
        </button>
      </div>
      <div v-if="selectedFile" class="selected-file">
        已选择：{{ selectedFile.name }}
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend, appURL } from '../api/client'
import {
  invoiceStatusLabel,
  invoiceStatusTone,
  orderInvoiceAssetName,
  orderInvoiceFileAccept,
  orderInvoiceFileAllowed,
} from '../lib/order-invoice'

const props = defineProps({
  orderId: { type: [Number, String], default: 0 },
  embedded: { type: Boolean, default: false },
})

const emit = defineEmits(['close', 'updated'])

const loading = ref(false)
const requesting = ref(false)
const uploading = ref(false)
const error = ref('')
const message = ref('')
const selectedFile = ref(null)
const invoice = reactive(emptyInvoice())

const orderID = computed(() => Number(props.orderId || new URL(window.location.href).searchParams.get('order_id') || 0))

function emptyInvoice() {
  return {
    order_id: 0,
    order_no: '',
    status: '',
    requested_at: '',
    requested_by: '',
    uploaded_at: '',
    uploaded_by: '',
    asset: null,
  }
}

function assignInvoice(data = {}) {
  Object.assign(invoice, emptyInvoice(), data || {})
}

async function load() {
  if (!orderID.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/orders/${orderID.value}/invoice`)
    assignInvoice(data)
  } catch (err) {
    error.value = err.message || '加载发票信息失败'
  } finally {
    loading.value = false
  }
}

async function requestInvoice() {
  if (!orderID.value) return
  requesting.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/orders/${orderID.value}/invoice-request`)
    assignInvoice(data)
    message.value = '已申请发票'
    emit('updated')
  } catch (err) {
    error.value = err.message || '申请发票失败'
  } finally {
    requesting.value = false
  }
}

function handleFileChange(event) {
  const file = event?.target?.files?.[0] || null
  selectedFile.value = null
  error.value = ''
  message.value = ''
  if (!file) return
  if (!orderInvoiceFileAllowed(file)) {
    error.value = '只支持 PDF、PNG、JPG、GIF、WebP 发票文件'
    event.target.value = ''
    return
  }
  selectedFile.value = file
}

async function uploadInvoiceFile() {
  if (!orderID.value || !selectedFile.value) return
  uploading.value = true
  error.value = ''
  message.value = ''
  try {
    const form = new FormData()
    form.append('file', selectedFile.value)
    const data = await apiSend(`/api/orders/${orderID.value}/invoice-file`, { body: form })
    assignInvoice(data)
    selectedFile.value = null
    message.value = '发票文件已上传'
    emit('updated')
  } catch (err) {
    error.value = err.message || '上传发票文件失败'
  } finally {
    uploading.value = false
  }
}

function formatBytes(bytes) {
  const n = Number(bytes || 0)
  if (n <= 0) return '-'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.invoice-page { padding: 18px; color: #171717; display: grid; gap: 16px; }
.invoice-page.embedded { padding: 14px; }
.panel { background: #fff; border: 1px solid #e6e0d8; border-radius: 8px; padding: 16px; }
.panel-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2, h3, p { margin: 0; }
h2 { font-size: 22px; }
h3 { font-size: 17px; }
p { color: #666; font-size: 13px; margin-top: 4px; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
button, .link-button { border: 1px solid #d7cec3; border-radius: 6px; background: #fff; padding: 8px 12px; font-size: 14px; color: #171717; text-decoration: none; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.primary { background: #1f6f4a; border-color: #1f6f4a; color: #fff; }
.secondary { background: #fff; }
.summary { display: flex; flex-wrap: wrap; gap: 10px; color: #555; font-size: 13px; }
.summary span { background: #f8f7f4; border: 1px solid #eee7df; border-radius: 6px; padding: 5px 8px; }
.status-pill.ok { background: #eef8f1; border-color: #cfe8d4; color: #1f6f4a; }
.status-pill.warn { background: #fff8e8; border-color: #ead9a8; color: #765a11; }
.status-pill.muted { color: #777; }
.file-current { border: 1px solid #eee7df; background: #faf8f4; border-radius: 8px; padding: 12px; display: grid; gap: 4px; margin-bottom: 12px; }
.file-current span { color: #666; font-size: 13px; }
.upload-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: end; }
label { display: grid; gap: 6px; font-size: 13px; color: #555; }
input { width: 100%; border: 1px solid #d8d1c8; border-radius: 6px; padding: 9px 10px; font: inherit; background: #fff; color: #171717; }
.selected-file { margin-top: 10px; color: #555; font-size: 13px; }
.notice { padding: 10px 12px; border-radius: 7px; margin-bottom: 12px; }
.notice.ok { background: #eef8f1; border: 1px solid #cfe8d4; color: #1f6f4a; }
.notice.error { background: #fff1f1; border: 1px solid #f0caca; color: #9d1c1c; }
@media (max-width: 900px) {
  .invoice-page { padding: 12px; }
  .panel-head, .actions, .upload-row { align-items: stretch; grid-template-columns: 1fr; flex-direction: column; }
}
</style>
