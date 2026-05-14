<template>
  <div class="page contracts-page">
    <section class="panel contract-toolbar">
      <div>
        <h2>合同盖章</h2>
        <p>上传 PDF 或 DOCX 合同，选择公章后在多页拖动位置并保存盖章 PDF。</p>
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
            <h3>合同</h3>
            <p>{{ contracts.length }} 份</p>
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
          <em v-if="item.latest_stamped">已盖章 V{{ item.latest_stamped.version_no }}</em>
        </button>
        <div v-if="!contracts.length" class="muted empty">暂无合同</div>
      </aside>

      <main class="panel contract-workspace">
        <div class="workspace-head">
          <div>
            <h3>{{ selectedContract?.title || '选择合同' }}</h3>
            <p v-if="selectedContract">源文件：{{ selectedContract.source_filename }} · PDF {{ formatBytes(selectedContract.pdf_bytes) }}</p>
          </div>
          <div class="seal-tools">
            <label>
              <span>公章</span>
              <select v-model.number="selectedSealID">
                <option :value="0">请选择公章</option>
                <option v-for="seal in seals" :key="seal.id" :value="seal.id">{{ seal.filename || `公章 ${seal.id}` }}</option>
              </select>
            </label>
            <button class="secondary" type="button" :disabled="!selectedContract || !selectedSeal" @click="stampAllPages">全部页加盖</button>
            <button class="primary" type="button" :disabled="saving || !canSave" @click="saveStampedPDF">{{ saving ? '保存中' : '保存盖章PDF' }}</button>
            <a v-if="latestStampedURL" class="secondary button-link" :href="appURL(latestStampedURL)">下载已盖章PDF</a>
          </div>
        </div>

        <div v-if="selectedContract && !selectedSeal" class="notice">请先在销售单设置中上传公章，或从已有公章中选择一个。</div>
        <div v-if="rendering" class="status">PDF 加载中</div>
        <div v-else-if="selectedContract && !pages.length" class="status">未加载到 PDF 页面</div>
        <div v-else class="pdf-pages">
          <div
            v-for="page in pages"
            :key="page.pageNumber"
            class="pdf-page-shell">
            <div class="page-title">
              <span>第 {{ page.pageNumber }} 页</span>
              <button class="secondary small" type="button" :disabled="!selectedSeal" @click="addStamp(page)">本页加盖</button>
            </div>
            <div class="pdf-page" :style="{ width: `${page.displayWidth}px`, height: `${page.displayHeight}px` }">
              <img :src="page.dataUrl" alt="合同PDF页面" draggable="false" />
              <div
                v-for="(placement, index) in placementsForPage(page.pageNumber)"
                :key="`${page.pageNumber}-${index}`"
                class="stamp-overlay"
                :style="contractStampOverlayStyle(placement, page.displayScale)"
                title="拖动调整公章位置"
                @pointerdown.prevent="startDrag($event, placement)">
                <img v-if="selectedSeal?.url" :src="assetURL(selectedSeal.url)" alt="公章" draggable="false" />
                <span v-else>公章</span>
              </div>
            </div>
          </div>
        </div>
      </main>
    </section>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import * as pdfjsLib from 'pdfjs-dist'
import pdfWorker from 'pdfjs-dist/build/pdf.worker.mjs?url'
import { apiFetch, apiGet, apiSend, appURL } from '../api/client'
import {
  contractStampOverlayStyle,
  contractStampPayload,
  createStampedContractPDF,
  defaultContractStampPlacement,
  moveContractStampPlacement,
  normalizeContractUploadKind,
} from '../lib/contract-stamp'

pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorker

const fileInput = ref(null)
const contracts = ref([])
const seals = ref([])
const uploadFile = ref(null)
const selectedContractID = ref(0)
const selectedSealID = ref(0)
const pages = ref([])
const placements = ref([])
const currentPDFBytes = ref(null)
const loading = ref(false)
const uploading = ref(false)
const rendering = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const dragState = ref(null)

const selectedContract = computed(() => contracts.value.find((item) => item.id === selectedContractID.value) || null)
const selectedSeal = computed(() => seals.value.find((item) => item.id === selectedSealID.value) || null)
const latestStampedURL = computed(() => selectedContract.value?.latest_stamped?.download_url || '')
const canSave = computed(() => selectedContract.value && selectedSeal.value && placements.value.length > 0 && currentPDFBytes.value)

onMounted(loadAll)
onBeforeUnmount(stopDrag)

function assetURL(url) {
  return appURL(url || '')
}

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
    if (!selectedContractID.value && contracts.value.length) {
      await selectContract(contracts.value[0])
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
  message.value = ''
  if (!item?.pdf_url) return
  await loadPDF(item.pdf_url)
}

async function loadPDF(url) {
  rendering.value = true
  error.value = ''
  try {
    const res = await apiFetch(url)
    if (!res.ok) throw new Error('PDF 下载失败')
    const buffer = await res.arrayBuffer()
    currentPDFBytes.value = buffer
    await renderPDF(buffer)
  } catch (err) {
    error.value = err.message || 'PDF 加载失败'
  } finally {
    rendering.value = false
  }
}

async function renderPDF(buffer) {
  const pdf = await pdfjsLib.getDocument({ data: buffer.slice(0) }).promise
  const out = []
  for (let i = 1; i <= pdf.numPages; i += 1) {
    const page = await pdf.getPage(i)
    const base = page.getViewport({ scale: 1 })
    const displayScale = Math.min(1.25, 760 / base.width)
    const viewport = page.getViewport({ scale: displayScale })
    const canvas = document.createElement('canvas')
    canvas.width = Math.ceil(viewport.width)
    canvas.height = Math.ceil(viewport.height)
    const context = canvas.getContext('2d')
    await page.render({ canvasContext: context, viewport }).promise
    out.push({
      pageNumber: i,
      width: base.width,
      height: base.height,
      displayWidth: canvas.width,
      displayHeight: canvas.height,
      displayScale,
      dataUrl: canvas.toDataURL('image/png'),
    })
  }
  pages.value = out
}

function addStamp(page) {
  if (!page || !selectedSeal.value) return
  placements.value = [
    ...placements.value.filter((item) => item.page_number !== page.pageNumber),
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

function placementsForPage(pageNumber) {
  return placements.value.filter((placement) => placement.page_number === pageNumber)
}

function startDrag(event, placement) {
  const index = placements.value.indexOf(placement)
  const page = pages.value.find((item) => item.pageNumber === placement.page_number)
  if (index < 0 || !page) return
  dragState.value = {
    index,
    startX: event.clientX,
    startY: event.clientY,
    original: { ...placement },
    displayScale: page.displayScale,
  }
  window.addEventListener('pointermove', moveDrag)
  window.addEventListener('pointerup', stopDrag)
}

function moveDrag(event) {
  const state = dragState.value
  if (!state) return
  placements.value[state.index] = moveContractStampPlacement(state.original, {
    deltaX: event.clientX - state.startX,
    deltaY: event.clientY - state.startY,
    displayScale: state.displayScale,
  })
}

function stopDrag() {
  if (!dragState.value) return
  dragState.value = null
  window.removeEventListener('pointermove', moveDrag)
  window.removeEventListener('pointerup', stopDrag)
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
.contract-layout { display: grid; grid-template-columns: minmax(240px, 300px) minmax(0, 1fr); gap: 14px; align-items: start; }
.contract-list { display: grid; gap: 8px; align-content: start; }
.panel-head.compact { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.contract-item { text-align: left; border: 1px solid #e5e7eb; background: #fff; border-radius: 8px; padding: 10px; display: grid; gap: 4px; cursor: pointer; }
.contract-item.active { border-color: #2563eb; box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.12); }
.contract-item span { color: #6b7280; font-size: 12px; }
.contract-item em { color: #047857; font-style: normal; font-size: 12px; }
.empty { padding: 12px; }
.contract-workspace { min-width: 0; display: grid; gap: 12px; }
.workspace-head { display: flex; justify-content: space-between; gap: 12px; align-items: end; }
.seal-tools { display: flex; gap: 8px; align-items: end; flex-wrap: wrap; justify-content: flex-end; }
.seal-tools label { display: grid; gap: 4px; min-width: 180px; }
.button-link { text-decoration: none; display: inline-flex; align-items: center; justify-content: center; }
.notice { border: 1px solid #facc15; background: #fefce8; color: #854d0e; border-radius: 8px; padding: 10px; }
.pdf-pages { display: grid; gap: 18px; justify-items: center; overflow-x: auto; padding-bottom: 6px; }
.pdf-page-shell { display: grid; gap: 8px; }
.page-title { display: flex; justify-content: space-between; align-items: center; gap: 8px; color: #374151; font-size: 13px; }
.pdf-page { position: relative; background: white; border: 1px solid #d1d5db; box-shadow: 0 8px 20px rgba(15, 23, 42, 0.12); overflow: hidden; }
.pdf-page > img { display: block; width: 100%; height: 100%; user-select: none; pointer-events: none; }
.stamp-overlay { position: absolute; border: 1px dashed rgba(220, 38, 38, 0.75); cursor: move; touch-action: none; display: flex; align-items: center; justify-content: center; color: #dc2626; font-weight: 700; background: rgba(254, 242, 242, 0.2); }
.stamp-overlay img { width: 100%; height: 100%; object-fit: contain; pointer-events: none; user-select: none; }
.small { padding: 5px 8px; font-size: 12px; }
@media (max-width: 920px) {
  .contract-toolbar, .workspace-head { align-items: stretch; flex-direction: column; }
  .contract-layout { grid-template-columns: 1fr; }
  .seal-tools { justify-content: flex-start; }
}
</style>
