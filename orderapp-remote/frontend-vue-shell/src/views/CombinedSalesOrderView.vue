<template>
  <div class="combined-document-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>组合销售单</h2>
        <div class="actions">
          <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
          <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !canPreview">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
          <a v-if="latestDocument?.download_url" class="secondary link-button" :href="latestDocument.download_url" target="_blank" rel="noopener">下载本次 PDF</a>
          <button class="primary" type="button" @click="generate" :disabled="generating || !preview">{{ generating ? '生成中' : '确认生成 PDF' }}</button>
        </div>
      </div>
      <div v-if="error" class="notice error">{{ error }}</div>
      <div v-if="message" class="notice ok">{{ message }}</div>
      <div class="summary">
        <span>订单数：{{ normalizedOrderIDs.length }}</span>
        <span>客户：{{ preview?.snapshot?.customer_name || '-' }}</span>
        <span>关联订单：{{ relatedOrderNos || '-' }}</span>
        <span>下一版本：V{{ preview?.next_version_no || '-' }}</span>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>销售单预览 <span v-if="preview" class="version-tag">V{{ preview.next_version_no }}</span></h3>
        <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !canPreview">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
      </div>
      <div v-if="!preview" class="muted preview-empty">暂无预览</div>
      <PDFStampPreview
        v-else
        :pdf-url="salesOrderPreviewPDFUrl"
        :editable="false"
        preview-label="PREVIEW 预览版"
      />
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>关联订单</h3>
      </div>
      <table>
        <thead>
          <tr><th>订单号</th><th>单据日期</th><th>订单日期</th><th>商品数</th><th>商品金额</th><th>优惠</th><th>运费</th><th>应收</th></tr>
        </thead>
        <tbody>
          <tr v-for="group in preview?.snapshot?.groups || []" :key="group.order_id">
            <td>{{ group.order_no }}</td>
            <td>{{ group.document_date || '-' }}</td>
            <td>{{ group.order_date || '-' }}</td>
            <td>{{ group.items?.length || 0 }}</td>
            <td>{{ group.total_amount || '0.00' }}</td>
            <td>{{ group.discount || '0.00' }}</td>
            <td>{{ group.shipping || '0.00' }}</td>
            <td>{{ group.grand_total || '0.00' }}</td>
          </tr>
          <tr v-if="!(preview?.snapshot?.groups || []).length"><td colspan="8" class="muted">暂无关联订单</td></tr>
        </tbody>
      </table>
    </section>

    <section v-if="documents.length" class="panel">
      <div class="panel-head">
        <h3>已生成</h3>
      </div>
      <table>
        <thead>
          <tr><th>版本</th><th>关联订单</th><th>生成时间</th><th>操作人</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="doc in documents" :key="doc.id">
            <td>V{{ doc.version_no }}</td>
            <td>{{ (doc.order_nos || []).join('、') }}</td>
            <td>{{ doc.created_at }}</td>
            <td>{{ doc.created_by || '-' }}</td>
            <td>{{ doc.is_latest ? '最新版' : '历史版本' }}</td>
            <td><a class="text-link" :href="doc.download_url" target="_blank" rel="noopener">下载</a></td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import PDFStampPreview from '../components/PDFStampPreview.vue'
import { buildCombinedDocumentQuery } from '../lib/combined-order-documents'

const props = defineProps({
  orderIds: { type: Array, default: () => [] },
  embedded: { type: Boolean, default: false },
})

const emit = defineEmits(['close'])

const preview = ref(null)
const documents = ref([])
const latestDocument = ref(null)
const previewLoading = ref(false)
const generating = ref(false)
const error = ref('')
const message = ref('')
const previewPDFRefreshKey = ref(0)

const normalizedOrderIDs = computed(() => {
  const seen = new Set()
  return (props.orderIds || [])
    .map((id) => Number(id))
    .filter((id) => {
      if (!Number.isFinite(id) || id <= 0 || seen.has(id)) return false
      seen.add(id)
      return true
    })
})
const canPreview = computed(() => normalizedOrderIDs.value.length >= 2)
const query = computed(() => buildCombinedDocumentQuery(normalizedOrderIDs.value))
const salesOrderPreviewPDFUrl = computed(() => query.value ? `/api/orders/combined/sales-order-preview.pdf?${query.value}&v=${previewPDFRefreshKey.value}` : '')
const relatedOrderNos = computed(() => (preview.value?.order_nos || preview.value?.snapshot?.order_nos || []).join('、'))

async function loadPreview() {
  if (!canPreview.value) {
    preview.value = null
    error.value = '请选择同一客户的至少两个订单'
    return
  }
  previewLoading.value = true
  error.value = ''
  try {
    preview.value = await apiGet(`/api/orders/combined/sales-order-preview?${query.value}`)
    previewPDFRefreshKey.value += 1
  } catch (err) {
    preview.value = null
    error.value = err.message || '加载组合销售单预览失败'
  } finally {
    previewLoading.value = false
  }
}

async function generate() {
  if (!preview.value || generating.value) return
  generating.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend('/api/orders/combined/sales-orders', {
      body: { order_ids: normalizedOrderIDs.value },
    })
    latestDocument.value = data
    documents.value = [data, ...documents.value.filter((doc) => Number(doc.id) !== Number(data.id))]
    message.value = `已生成组合销售单 V${data.version_no}`
    await loadPreview()
  } catch (err) {
    error.value = err.message || '生成组合销售单失败'
  } finally {
    generating.value = false
  }
}

onMounted(loadPreview)
watch(normalizedOrderIDs, loadPreview)
</script>

<style scoped>
* { box-sizing: border-box; }
.combined-document-page { padding: 18px; color: #171717; display: grid; gap: 16px; }
.combined-document-page.embedded { padding: 14px; }
.panel { background: #fff; border: 1px solid #e6e0d8; border-radius: 8px; padding: 16px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2, h3 { margin: 0; }
h2 { font-size: 22px; }
h3 { font-size: 17px; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
button, .link-button { border: 1px solid #d7cec3; border-radius: 6px; background: #fff; padding: 8px 12px; font-size: 14px; color: #171717; text-decoration: none; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.primary { background: #1f6f4a; border-color: #1f6f4a; color: #fff; }
.secondary { background: #fff; }
.summary { display: flex; flex-wrap: wrap; gap: 10px; color: #555; font-size: 13px; }
.summary span { background: #f8f7f4; border: 1px solid #eee7df; border-radius: 6px; padding: 5px 8px; }
table { width: 100%; border-collapse: collapse; font-size: 14px; }
th, td { border-bottom: 1px solid #eee7df; padding: 9px 8px; text-align: left; vertical-align: top; }
th { color: #555; background: #faf8f4; font-weight: 600; }
.text-link { color: #1f6f4a; text-decoration: none; font-weight: 600; }
.version-tag { color: #1f6f4a; font-size: 13px; }
.notice { padding: 10px 12px; border-radius: 7px; margin-bottom: 12px; }
.notice.ok { background: #eef8f1; border: 1px solid #cfe8d4; color: #1f6f4a; }
.notice.error { background: #fff1f1; border: 1px solid #f0caca; color: #9d1c1c; }
.muted { color: #888; text-align: center; }
.preview-empty { padding: 20px 0; }
@media (max-width: 900px) {
  .combined-document-page { padding: 12px; }
  .panel-head, .actions { align-items: stretch; flex-direction: column; }
}
</style>
