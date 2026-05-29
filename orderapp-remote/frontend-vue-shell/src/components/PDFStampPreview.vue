<template>
  <div class="pdf-stamp-preview">
    <div v-if="error" class="pdf-stamp-status error">{{ error }}</div>
    <div v-else-if="loading" class="pdf-stamp-status">PDF 加载中</div>
    <div v-else-if="!pages.length" class="pdf-stamp-status">暂无 PDF 预览</div>
    <div v-else class="pdf-stamp-pages">
      <div v-for="page in pages" :key="page.pageNumber" class="pdf-stamp-page-shell">
        <div class="pdf-stamp-page-title">
          <span>第 {{ page.pageNumber }} 页</span>
          <slot name="page-actions" :page="page"></slot>
        </div>
        <div class="pdf-stamp-page" :style="{ width: `${page.displayWidth}px`, height: `${page.displayHeight}px` }">
          <img :src="page.dataUrl" alt="PDF页面" draggable="false" />
          <span v-if="previewLabel" class="pdf-stamp-preview-label">{{ previewLabel }}</span>
          <div
            v-for="(placement, index) in placementsForPage(page.pageNumber)"
            :key="`${page.pageNumber}-${index}`"
            class="pdf-stamp-overlay"
            :class="overlayClass(placement)"
            :style="contractStampOverlayStyle(placement, page.displayScale)"
            :title="overlayTitle(placement)"
            @pointerdown.prevent="startDrag($event, placement, page)">
            <img v-if="overlayImageUrl(placement)" :src="overlayImageUrl(placement)" :alt="overlayLabel(placement)" draggable="false" />
            <span v-else>{{ overlayLabel(placement) }}</span>
            <i
              v-if="canResize(placement)"
              class="pdf-stamp-resize-handle"
              title="拖动调整大小"
              @pointerdown.stop.prevent="startResize($event, placement, page)"
            ></i>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, ref, watch } from 'vue'
import * as pdfjsLib from 'pdfjs-dist'
import pdfWorker from 'pdfjs-dist/build/pdf.worker.mjs?url'
import { apiFetch } from '../api/client'
import { contractStampOverlayStyle } from '../lib/contract-stamp'
import { movePDFStampPlacement, movePDFStampPlacementAcrossPages, resizePDFStampPlacement } from '../lib/document-pdf-stamp'

pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorker

const props = defineProps({
  pdfUrl: { type: String, default: '' },
  pdfBytes: { type: [ArrayBuffer, Uint8Array], default: null },
  placements: { type: Array, default: () => [] },
  sealUrl: { type: String, default: '' },
  sealLabel: { type: String, default: '' },
  previewLabel: { type: String, default: '' },
  editable: { type: Boolean, default: true },
  maxDisplayWidth: { type: Number, default: 760 },
})

const emit = defineEmits(['loaded', 'placement-change', 'placement-commit'])

const pages = ref([])
const localPlacements = ref([])
const loading = ref(false)
const error = ref('')
const dragState = ref(null)

watch(() => props.placements, (items) => {
  localPlacements.value = (items || []).map((item) => ({ ...item }))
}, { deep: true, immediate: true })

watch(() => [props.pdfUrl, props.pdfBytes], loadPDF, { immediate: true })
onBeforeUnmount(stopDrag)

function placementsForPage(pageNumber) {
  return localPlacements.value.filter((placement) => Number(placement.page_number || 1) === Number(pageNumber))
}

async function loadPDF() {
  pages.value = []
  error.value = ''
  if (!props.pdfUrl && !props.pdfBytes) return
  loading.value = true
  try {
    const bytes = props.pdfBytes || await fetchPDFBytes(props.pdfUrl)
    await renderPDF(bytes)
  } catch (err) {
    error.value = err.message || 'PDF 预览加载失败'
  } finally {
    loading.value = false
  }
}

async function fetchPDFBytes(url) {
  const res = await apiFetch(url)
  if (!res.ok) throw new Error('PDF 下载失败')
  return res.arrayBuffer()
}

async function renderPDF(buffer) {
  const pdf = await pdfjsLib.getDocument({ data: buffer.slice(0) }).promise
  const out = []
  for (let i = 1; i <= pdf.numPages; i += 1) {
    const page = await pdf.getPage(i)
    const base = page.getViewport({ scale: 1 })
    const displayScale = Math.min(1.25, Number(props.maxDisplayWidth || 760) / base.width)
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
      pageWidth: base.width,
      pageHeight: base.height,
      displayWidth: canvas.width,
      displayHeight: canvas.height,
      displayScale,
      dataUrl: canvas.toDataURL('image/png'),
    })
  }
  pages.value = out
  emit('loaded', out)
}

function startDrag(event, placement, page) {
  if (!canDrag(placement)) return
  const index = localPlacements.value.indexOf(placement)
  if (index < 0 || !page) return
  dragState.value = {
    mode: 'move',
    index,
    startX: event.clientX,
    startY: event.clientY,
    original: { ...placement },
    displayScale: page.displayScale,
  }
  event.currentTarget?.setPointerCapture?.(event.pointerId)
  window.addEventListener('pointermove', moveDrag)
  window.addEventListener('pointerup', stopDrag)
}

function startResize(event, placement, page) {
  if (!canResize(placement)) return
  const index = localPlacements.value.indexOf(placement)
  if (index < 0 || !page) return
  dragState.value = {
    mode: 'resize',
    index,
    startX: event.clientX,
    startY: event.clientY,
    original: { ...placement },
    displayScale: page.displayScale,
  }
  event.currentTarget?.setPointerCapture?.(event.pointerId)
  window.addEventListener('pointermove', moveDrag)
  window.addEventListener('pointerup', stopDrag)
}

function moveDrag(event) {
  const state = dragState.value
  if (!state) return
  const movement = {
    deltaX: event.clientX - state.startX,
    deltaY: event.clientY - state.startY,
    displayScale: state.displayScale,
  }
  const next = state.mode === 'resize'
    ? resizePDFStampPlacement(state.original, {
      ...movement,
      minWidth: state.original.min_width || 24,
      minHeight: state.original.min_height || 24,
      lockAspectRatio: Boolean(state.original.lock_aspect_ratio),
    })
    : state.original.cross_page_drag
      ? movePDFStampPlacementAcrossPages(state.original, { ...movement, pages: pages.value })
      : movePDFStampPlacement(state.original, movement)
  localPlacements.value[state.index] = next
  emit('placement-change', next)
}

function stopDrag() {
  const state = dragState.value
  if (!state) return
  const next = localPlacements.value[state.index]
  dragState.value = null
  window.removeEventListener('pointermove', moveDrag)
  window.removeEventListener('pointerup', stopDrag)
  if (next) emit('placement-commit', next)
}

function canDrag(placement) {
  return props.editable && placement?.draggable !== false
}

function canResize(placement) {
  return props.editable && placement?.resizable === true
}

function overlayImageUrl(placement) {
  if (placement?.image_url) return placement.image_url
  if (placement?.use_seal_image === false) return ''
  if (placement?.kind && placement.kind !== 'seal') return ''
  return props.sealUrl || ''
}

function overlayLabel(placement) {
  return placement?.label || props.sealLabel || '公章'
}

function overlayTitle(placement) {
  if (canResize(placement)) return '拖动调整位置，右下角调整大小'
  return `拖动调整${overlayLabel(placement)}位置`
}

function overlayClass(placement) {
  const kind = String(placement?.kind || 'seal').replace(/[^a-z0-9_-]/gi, '-')
  return {
    editable: canDrag(placement),
    resizable: canResize(placement),
    [`kind-${kind}`]: true,
  }
}
</script>

<style scoped>
.pdf-stamp-preview { display: grid; gap: 12px; min-width: 0; }
.pdf-stamp-status { border: 1px solid #e5e7eb; border-radius: 8px; padding: 14px; color: #4b5563; background: #fff; text-align: center; }
.pdf-stamp-status.error { border-color: #fca5a5; color: #991b1b; background: #fef2f2; }
.pdf-stamp-pages { display: grid; gap: 18px; justify-items: center; overflow-x: auto; padding-bottom: 6px; }
.pdf-stamp-page-shell { display: grid; gap: 8px; }
.pdf-stamp-page-title { display: flex; justify-content: space-between; align-items: center; gap: 8px; color: #374151; font-size: 13px; }
.pdf-stamp-page { position: relative; background: #fff; border: 1px solid #d1d5db; box-shadow: 0 8px 20px rgba(15, 23, 42, 0.12); overflow: hidden; }
.pdf-stamp-page > img { display: block; width: 100%; height: 100%; user-select: none; pointer-events: none; }
.pdf-stamp-preview-label { position: absolute; top: 10px; right: 12px; border: 1px solid rgba(220, 38, 38, .55); color: #b91c1c; background: rgba(255, 255, 255, .82); padding: 4px 8px; font-size: 12px; font-weight: 700; z-index: 2; }
.pdf-stamp-overlay { position: absolute; border: 1px dashed rgba(220, 38, 38, 0.75); display: flex; align-items: center; justify-content: center; color: #dc2626; font-weight: 700; background: rgba(254, 242, 242, 0.2); z-index: 3; }
.pdf-stamp-overlay.editable { cursor: move; touch-action: none; }
.pdf-stamp-overlay img { max-width: 100%; max-height: 100%; width: auto; height: auto; object-fit: contain; pointer-events: none; user-select: none; }
.pdf-stamp-overlay.kind-payment_text,
.pdf-stamp-overlay.kind-payment_code { border-color: rgba(37, 99, 235, .8); background: rgba(239, 246, 255, .28); color: #1d4ed8; font-size: 13px; text-align: center; padding: 4px; }
.pdf-stamp-overlay.kind-payment_code { border-color: rgba(5, 150, 105, .82); background: rgba(236, 253, 245, .3); color: #047857; }
.pdf-stamp-resize-handle { position: absolute; right: -5px; bottom: -5px; width: 12px; height: 12px; border: 2px solid currentColor; border-radius: 999px; background: #fff; cursor: nwse-resize; touch-action: none; }
</style>
