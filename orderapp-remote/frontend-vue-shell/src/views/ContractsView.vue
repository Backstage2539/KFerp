<template>
  <div class="page contracts-page">
    <section class="panel contract-toolbar">
      <div>
        <h2>合同盖章</h2>
        <p>上传 PDF 或 DOCX 合同，保存合同信息后在 PDF 预览中加盖公章。</p>
      </div>
      <div class="upload-row">
        <input ref="fileInput" type="file" accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document" @change="onFileChange" />
        <button class="primary" type="button" :disabled="uploading || !uploadFile" @click="uploadContract">{{ uploading ? '上传中' : '上传合同' }}</button>
      </div>
    </section>

    <section v-if="error" class="panel error">{{ error }}</section>
    <section v-if="message" class="panel ok">{{ message }}</section>

    <section class="contract-layout">
      <aside class="panel contract-list">
        <div class="panel-head compact">
          <div>
            <h3>合同库</h3>
            <p>{{ contracts.length }} 份有效合同</p>
          </div>
          <button class="secondary" type="button" :disabled="loading" @click="loadAll">刷新</button>
        </div>
        <button
          v-for="item in contracts"
          :key="item.id"
          class="contract-item"
          :class="{ active: selectedContractID === item.id }"
          type="button"
          @click="selectContract(item)">
          <strong>{{ item.title || item.source_filename }}</strong>
          <span>{{ item.source_kind?.toUpperCase() }} · {{ item.created_at || '-' }}</span>
          <small v-if="item.note">{{ item.note }}</small>
          <em v-if="item.latest_stamped">已盖章 V{{ item.latest_stamped.version_no }}</em>
        </button>
        <div v-if="!contracts.length" class="muted empty">暂无合同</div>
      </aside>

      <main class="panel contract-workspace">
        <div class="workspace-head">
          <div>
            <h3>{{ selectedContract?.title || '选择合同' }}</h3>
            <p v-if="selectedContract">源文件：{{ selectedContract.source_filename }} · PDF {{ formatBytes(selectedContract.pdf_bytes) }}</p>
            <p v-else>从左侧合同库选择，或先上传一个 PDF/DOCX 合同。</p>
          </div>
          <div class="workspace-actions">
            <button class="secondary" type="button" :disabled="!selectedContract || savingMetadata" @click="saveContractMetadata">{{ savingMetadata ? '保存中' : '保存合同' }}</button>
            <button class="danger" type="button" :disabled="!selectedContract || deleting" @click="deleteContract">{{ deleting ? '删除中' : '删除合同' }}</button>
          </div>
        </div>

        <div v-if="selectedContract" class="contract-form">
          <label>
            <span>合同标题</span>
            <input v-model.trim="contractForm.title" type="text" placeholder="合同标题" />
          </label>
          <label>
            <span>合同备注</span>
            <textarea v-model.trim="contractForm.note" rows="3" placeholder="记录客户、用途或版本说明"></textarea>
          </label>
        </div>

        <div class="stamp-toolbar">
          <label>
            <span>公章</span>
            <select v-model.number="selectedSealID" :disabled="!selectedContract">
              <option :value="0">请选择公章</option>
              <option v-for="seal in seals" :key="seal.id" :value="seal.id">{{ seal.filename || `公章 ${seal.id}` }}</option>
            </select>
          </label>
          <button class="secondary" type="button" :disabled="!selectedContract || !selectedSeal || !pages.length" @click="stampAllPages">全部页加盖</button>
          <button class="primary" type="button" :disabled="saving || !canSave" @click="saveStampedPDF">{{ saving ? '保存中' : '保存盖章PDF' }}</button>
          <a v-if="latestStampedURL" class="secondary button-link" :href="appURL(latestStampedURL)">下载已盖章PDF</a>
        </div>

        <div v-if="selectedContract && !selectedSeal" class="notice">请先在销售单设置中上传公章，或从已有公章中选择一个。</div>
        <div v-if="rendering" class="status">PDF 文件准备中</div>
        <div v-else-if="selectedContract && currentPDFBytes" class="pdf-preview-wrap">
          <PDFStampPreview
            :pdf-bytes="currentPDFBytes"
            :placements="placements"
            :seal-url="selectedSealURL"
            seal-label="公章"
            :editable="Boolean(selectedSeal)"
            :max-display-width="820"
            @loaded="onPDFPreviewLoaded"
            @placement-change="updatePlacement"
            @placement-commit="updatePlacement">
            <template #page-actions="{ page }">
              <button class="secondary small" type="button" :disabled="!selectedSeal" @click="addStamp(page)">本页加盖</button>
            </template>
          </PDFStampPreview>
        </div>
        <div v-else class="status">请选择合同后预览 PDF</div>
      </main>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import PDFStampPreview from '../components/PDFStampPreview.vue'
import { apiFetch, apiGet, apiSend, appURL } from '../api/client'
import {
  contractStampPayload,
  createStampedContractPDF,
  defaultContractStampPlacement,
  normalizeContractUploadKind,
} from '../lib/contract-stamp'

const fileInput = ref(null)
const contracts = ref([])
const seals = ref([])
const uploadFile = ref(null)
const selectedContractID = ref(0)
const selectedSealID = ref(0)
const pages = ref([])
const placements = ref([])
const currentPDFBytes = ref(null)
const contractForm = ref({ title: '', note: '' })
const loading = ref(false)
const uploading = ref(false)
const rendering = ref(false)
const saving = ref(false)
const savingMetadata = ref(false)
const deleting = ref(false)
const error = ref('')
const message = ref('')

const selectedContract = computed(() => contracts.value.find((item) => item.id === selectedContractID.value) || null)
const selectedSeal = computed(() => seals.value.find((item) => item.id === selectedSealID.value) || null)
const selectedSealURL = computed(() => selectedSeal.value?.url ? appURL(selectedSeal.value.url) : '')
const latestStampedURL = computed(() => selectedContract.value?.latest_stamped?.download_url || '')
const canSave = computed(() => selectedContract.value && selectedSeal.value && placements.value.length > 0 && currentPDFBytes.value)

onMounted(loadAll)

function onFileChange(event) {
  const file = event.target.files?.[0] || null
  uploadFile.value = file
  error.value = ''
  if (file && !normalizeContractUploadKind({ filename: file.name, contentType: file.type })) {
    error.value = '只支持 PDF 或 DOCX 合同'
    uploadFile.value = null
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [contractData, sealData] = await Promise.all([
      apiGet('/api/contracts'),
      apiGet('/api/settings/sales-order/seals'),
    ])
    contracts.value = contractData.rows || []
    seals.value = sealData.rows || []
    if (!selectedSealID.value && sealData.current_id) selectedSealID.value = Number(sealData.current_id)

    const current = selectedContractID.value
      ? contracts.value.find((item) => Number(item.id) === Number(selectedContractID.value))
      : null
    if (current) {
      syncContractForm(current)
    } else if (contracts.value.length) {
      await selectContract(contracts.value[0])
    } else {
      clearSelectedContract()
    }
  } catch (err) {
    error.value = err.message || '加载合同失败'
  } finally {
    loading.value = false
  }
}

async function uploadContract() {
  if (!uploadFile.value) return
  uploading.value = true
  error.value = ''
  message.value = ''
  try {
    const body = new FormData()
    body.append('file', uploadFile.value)
    const doc = await apiSend('/api/contracts', { body })
    uploadFile.value = null
    if (fileInput.value) fileInput.value.value = ''
    message.value = doc.source_kind === 'docx' ? 'DOCX 已转换为 PDF' : 'PDF 已上传'
    await loadAll()
    await selectContract(doc)
  } catch (err) {
    error.value = err.message || '上传合同失败'
  } finally {
    uploading.value = false
  }
}

async function selectContract(item) {
  selectedContractID.value = Number(item.id || 0)
  placements.value = []
  pages.value = []
  currentPDFBytes.value = null
  message.value = ''
  syncContractForm(item)
  if (!item?.pdf_url) return
  await loadPDF(item.pdf_url)
}

function syncContractForm(item) {
  contractForm.value = {
    title: item?.title || item?.source_filename || '',
    note: item?.note || '',
  }
}

function clearSelectedContract() {
  selectedContractID.value = 0
  placements.value = []
  pages.value = []
  currentPDFBytes.value = null
  contractForm.value = { title: '', note: '' }
}

async function loadPDF(url) {
  rendering.value = true
  error.value = ''
  try {
    const res = await apiFetch(url)
    if (!res.ok) throw new Error('PDF 下载失败')
    currentPDFBytes.value = await res.arrayBuffer()
  } catch (err) {
    error.value = err.message || 'PDF 加载失败'
  } finally {
    rendering.value = false
  }
}

function onPDFPreviewLoaded(items) {
  pages.value = items || []
}

function addStamp(page) {
  if (!page || !selectedSeal.value) return
  placements.value = [
    ...placements.value.filter((item) => Number(item.page_number) !== Number(page.pageNumber)),
    defaultContractStampPlacement({ pageNumber: page.pageNumber, pageWidth: page.width, pageHeight: page.height }),
  ].sort((a, b) => a.page_number - b.page_number)
}

function stampAllPages() {
  if (!selectedSeal.value || !pages.value.length) return
  placements.value = pages.value.map((page) => defaultContractStampPlacement({
    pageNumber: page.pageNumber,
    pageWidth: page.width,
    pageHeight: page.height,
  }))
}

function updatePlacement(next) {
  if (!next) return
  const pageNumber = Number(next.page_number || 1)
  const existing = placements.value.findIndex((item) => Number(item.page_number || 1) === pageNumber)
  const updated = { ...next, page_number: pageNumber }
  if (existing >= 0) {
    placements.value = placements.value.map((item, index) => index === existing ? updated : item)
  } else {
    placements.value = [...placements.value, updated].sort((a, b) => a.page_number - b.page_number)
  }
}

async function saveContractMetadata() {
  if (!selectedContract.value) return
  const title = contractForm.value.title.trim()
  if (!title) {
    error.value = '合同标题不能为空'
    return
  }
  savingMetadata.value = true
  error.value = ''
  message.value = ''
  try {
    const doc = await apiSend(`/api/contracts/${selectedContractID.value}`, {
      method: 'PUT',
      body: { title, note: contractForm.value.note.trim() },
    })
    contracts.value = contracts.value.map((item) => Number(item.id) === Number(doc.id) ? doc : item)
    selectedContractID.value = Number(doc.id)
    syncContractForm(doc)
    message.value = '合同已保存'
  } catch (err) {
    error.value = err.message || '保存合同失败'
  } finally {
    savingMetadata.value = false
  }
}

async function deleteContract() {
  if (!selectedContract.value) return
  const title = selectedContract.value.title || selectedContract.value.source_filename || '当前合同'
  if (!window.confirm(`确认删除“${title}”？删除后将从合同库隐藏。`)) return
  deleting.value = true
  error.value = ''
  message.value = ''
  try {
    await apiSend(`/api/contracts/${selectedContractID.value}`, { method: 'DELETE' })
    clearSelectedContract()
    message.value = '合同已删除'
    await loadAll()
  } catch (err) {
    error.value = err.message || '删除合同失败'
  } finally {
    deleting.value = false
  }
}

async function saveStampedPDF() {
  if (!canSave.value) return
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    const sealRes = await apiFetch(selectedSeal.value.url)
    if (!sealRes.ok) throw new Error('公章下载失败')
    const sealBytes = await sealRes.arrayBuffer()
    const stamped = await createStampedContractPDF({
      pdfBytes: currentPDFBytes.value.slice(0),
      sealBytes,
      sealContentType: selectedSeal.value.content_type || 'image/png',
      placements: placements.value,
    })
    const payload = contractStampPayload({
      contractID: selectedContractID.value,
      sealAssetID: selectedSealID.value,
      placements: placements.value,
    })
    const body = new FormData()
    body.append('file', new Blob([stamped], { type: 'application/pdf' }), `${selectedContract.value.title || 'contract'}-stamped.pdf`)
    body.append('seal_asset_id', String(payload.seal_asset_id))
    body.append('placements', JSON.stringify(payload.placements))
    const version = await apiSend(`/api/contracts/${selectedContractID.value}/stamped`, { body })
    message.value = `盖章 PDF 已保存：V${version.version_no || 1}`
    await loadAll()
    selectedContractID.value = Number(payload.contract_id)
  } catch (err) {
    error.value = err.message || '保存盖章 PDF 失败'
  } finally {
    saving.value = false
  }
}

function formatBytes(bytes) {
  const n = Number(bytes || 0)
  if (n >= 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${n} B`
}
</script>

<style scoped>
.contracts-page { display: grid; gap: 14px; }
.contract-toolbar { display: flex; justify-content: space-between; gap: 16px; align-items: center; }
.contract-toolbar h2, .workspace-head h3, .contract-list h3 { margin: 0; }
.contract-toolbar p, .workspace-head p, .contract-list p { margin: 4px 0 0; color: #6b7280; }
.upload-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.contract-layout { display: grid; grid-template-columns: minmax(260px, 320px) minmax(0, 1fr); gap: 14px; align-items: start; }
.contract-list { display: grid; gap: 8px; align-content: start; max-height: calc(100vh - 190px); overflow: auto; }
.panel-head.compact { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.contract-item { text-align: left; border: 1px solid #e5e7eb; background: #fff; border-radius: 8px; padding: 10px; display: grid; gap: 4px; cursor: pointer; }
.contract-item.active { border-color: #2563eb; box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.12); }
.contract-item span { color: #6b7280; font-size: 12px; }
.contract-item small { color: #4b5563; line-height: 1.4; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.contract-item em { color: #047857; font-style: normal; font-size: 12px; }
.empty { padding: 12px; }
.contract-workspace { min-width: 0; display: grid; gap: 12px; }
.workspace-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; border-bottom: 1px solid #e5e7eb; padding-bottom: 12px; }
.workspace-actions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.contract-form { display: grid; grid-template-columns: minmax(220px, 1fr) minmax(260px, 2fr); gap: 10px; align-items: start; }
.contract-form label, .stamp-toolbar label { display: grid; gap: 5px; }
.contract-form span, .stamp-toolbar span { color: #4b5563; font-size: 12px; }
.contract-form textarea { resize: vertical; min-height: 78px; }
.stamp-toolbar { display: flex; gap: 8px; align-items: end; flex-wrap: wrap; border-bottom: 1px solid #e5e7eb; padding-bottom: 12px; }
.stamp-toolbar label { min-width: 190px; }
.button-link { text-decoration: none; display: inline-flex; align-items: center; justify-content: center; }
.danger { border: 1px solid #fecaca; background: #fff1f2; color: #b91c1c; }
.danger:disabled { color: #9ca3af; background: #f9fafb; border-color: #e5e7eb; }
.notice { border: 1px solid #facc15; background: #fefce8; color: #854d0e; border-radius: 8px; padding: 10px; }
.pdf-preview-wrap { min-width: 0; }
.small { padding: 5px 8px; font-size: 12px; }
@media (max-width: 920px) {
  .contract-toolbar, .workspace-head { align-items: stretch; flex-direction: column; }
  .contract-layout, .contract-form { grid-template-columns: 1fr; }
  .workspace-actions, .stamp-toolbar { justify-content: flex-start; }
}
</style>
