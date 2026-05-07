<template>
  <div class="seal-settings-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>公章设置</h2>
        <div class="actions">
          <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="notice error">{{ error }}</div>
      <div v-if="ok" class="notice ok">已保存</div>
      <div class="upload-row seal-row">
        <div class="current-seal">当前：{{ settings.seal?.filename || '未设置' }}</div>
        <input type="file" accept="image/*" @change="handleSealFile" />
        <button class="primary" type="button" @click="uploadSeal" :disabled="uploadingSeal || !sealFile">上传公章</button>
        <button class="secondary" type="button" @click="removeSealBackground" :disabled="removingSealBackground || !settings.seal">{{ removingSealBackground ? '处理中' : '去除背景' }}</button>
      </div>
      <div class="seal-position">
        <div class="seal-position-stage">
          <div class="company-line">{{ settings.company_name || '公司名称' }}</div>
          <img
            v-if="settings.seal?.url"
            class="seal-drag-image"
            :src="settings.seal.url"
            alt="公章"
            :style="sealDragStyle"
            title="拖动调整公章位置，松手自动保存"
            @pointerdown.stop="startSealDrag"
          />
          <div v-else class="seal-placeholder" :style="sealDragStyle" title="拖动调整公章位置，松手自动保存" @pointerdown.stop="startSealDrag">公章</div>
        </div>
        <div class="seal-position-fields">
          <label><span>X(mm)</span><input v-model.number="form.seal_x_mm" type="number" min="0" step="1" /></label>
          <label><span>Y(mm)</span><input v-model.number="form.seal_y_mm" type="number" min="0" step="1" /></label>
          <label class="seal-size-control">
            <span>公章大小(mm)</span>
            <div class="seal-size-inputs">
              <input v-model.number="form.seal_width_mm" type="range" min="20" max="80" step="1" />
              <input v-model.number="form.seal_width_mm" type="number" min="20" max="80" step="1" />
            </div>
          </label>
        </div>
      </div>
      <div class="actions right">
        <button class="primary" type="button" @click="saveSealPosition" :disabled="sealPositionSaving">{{ sealPositionSaving ? '保存中' : '保存公章位置' }}</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { beginSalesOrderSealDrag, moveSalesOrderSealDrag, salesOrderSealPreviewScale, salesOrderSealStyle } from '../lib/sales-order-seal'

const props = defineProps({
  embedded: { type: Boolean, default: false },
})

const emit = defineEmits(['close', 'updated'])

const loading = ref(false)
const uploadingSeal = ref(false)
const removingSealBackground = ref(false)
const sealPositionSaving = ref(false)
const error = ref('')
const ok = ref(false)
const settings = ref({})
const sealFile = ref(null)

const form = reactive({
  seal_x_mm: 32,
  seal_y_mm: 5,
  seal_width_mm: 36,
})

const sealDragStyle = computed(() => salesOrderSealStyle(form, salesOrderSealPreviewScale))

function assignSettings(data = {}) {
  settings.value = data || {}
  form.seal_x_mm = Number(data.seal_x_mm ?? data.seal?.x_mm ?? 32)
  form.seal_y_mm = Number(data.seal_y_mm ?? data.seal?.y_mm ?? 5)
  form.seal_width_mm = Number(data.seal_width_mm ?? data.seal?.width_mm ?? 36)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    assignSettings(await apiGet('/api/settings/sales-order'))
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function startSealDrag(event) {
  if (sealPositionSaving.value) return
  event.preventDefault()
  const drag = beginSalesOrderSealDrag({
    seal: form,
    clientX: event.clientX,
    clientY: event.clientY,
    scale: salesOrderSealPreviewScale,
  })
  const update = (clientX, clientY) => {
    const next = moveSalesOrderSealDrag(drag, { clientX, clientY })
    form.seal_x_mm = next.x_mm
    form.seal_y_mm = next.y_mm
    form.seal_width_mm = next.width_mm
  }
  const move = (moveEvent) => update(moveEvent.clientX, moveEvent.clientY)
  const up = async () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    await saveSealPosition()
  }
  event.currentTarget?.setPointerCapture?.(event.pointerId)
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up, { once: true })
}

async function saveSealPosition() {
  sealPositionSaving.value = true
  error.value = ''
  ok.value = false
  try {
    assignSettings(await apiSend('/api/settings/sales-order/seal-position', {
      body: {
        seal_x_mm: Number(form.seal_x_mm || 32),
        seal_y_mm: Number(form.seal_y_mm || 5),
        seal_width_mm: Number(form.seal_width_mm || 36),
      },
    }))
    ok.value = true
    emit('updated')
  } catch (err) {
    error.value = err.message || '保存公章位置失败'
  } finally {
    sealPositionSaving.value = false
  }
}

function handleSealFile(event) {
  sealFile.value = event.target.files?.[0] || null
}

async function uploadSeal() {
  if (!sealFile.value) return
  uploadingSeal.value = true
  error.value = ''
  ok.value = false
  try {
    const body = new FormData()
    body.append('file', sealFile.value)
    await apiSend('/api/settings/sales-order/seal', { body })
    sealFile.value = null
    await load()
    ok.value = true
    emit('updated')
  } catch (err) {
    error.value = err.message || '上传失败'
  } finally {
    uploadingSeal.value = false
  }
}

async function removeSealBackground() {
  if (!settings.value?.seal) return
  removingSealBackground.value = true
  error.value = ''
  ok.value = false
  try {
    await apiSend('/api/settings/sales-order/seal/remove-background')
    await load()
    ok.value = true
    emit('updated')
  } catch (err) {
    error.value = err.message || '去除公章背景失败'
  } finally {
    removingSealBackground.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.seal-settings-page { padding: 18px; color: #171717; }
.seal-settings-page.embedded { padding: 14px; }
.panel { background: #fff; border: 1px solid #e6e0d8; border-radius: 8px; padding: 16px; }
.panel-head, .actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.actions.right { justify-content: flex-end; margin: 12px 0 0; }
h2 { margin: 0; font-size: 20px; }
button { border: 1px solid #d7cec3; border-radius: 6px; background: #fff; padding: 8px 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.primary { background: #1f6f4a; border-color: #1f6f4a; color: #fff; }
.secondary { background: #fff; color: #171717; }
input { width: 100%; border: 1px solid #d8d1c8; border-radius: 6px; padding: 9px 10px; font: inherit; background: #fff; color: #171717; }
.upload-row { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(180px, 1fr) auto auto; gap: 10px; align-items: center; margin-bottom: 12px; }
.current-seal { color: #555; font-size: 13px; }
.seal-position { display: grid; grid-template-columns: minmax(320px, 462px) 1fr; gap: 12px; align-items: start; }
.seal-position-stage { position: relative; width: 100%; aspect-ratio: 2.5 / 1; border: 1px dashed #d2c8bc; border-radius: 8px; background: #fffdf9; overflow: hidden; }
.company-line { position: absolute; left: 16px; top: 14px; font-size: 18px; font-weight: 700; color: #222; }
.seal-drag-image, .seal-placeholder { position: absolute; object-fit: contain; opacity: .86; cursor: move; touch-action: none; }
.seal-placeholder { display: grid; place-items: center; border: 2px solid #b3261e; border-radius: 999px; color: #b3261e; font-weight: 700; }
.seal-position-fields { display: grid; grid-template-columns: repeat(3, minmax(80px, 1fr)); gap: 10px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.seal-size-control { grid-column: 1 / -1; }
.seal-size-inputs { display: grid; grid-template-columns: 1fr 86px; gap: 8px; align-items: center; }
.notice { padding: 10px 12px; border-radius: 7px; margin-bottom: 12px; }
.notice.ok { background: #eef8f1; border: 1px solid #cfe8d4; color: #1f6f4a; }
.notice.error { background: #fff1f1; border: 1px solid #f0caca; color: #9d1c1c; }
@media (max-width: 900px) {
  .seal-settings-page { padding: 12px; }
  .panel-head, .actions { align-items: stretch; flex-direction: column; }
  .upload-row, .seal-position, .seal-position-fields { grid-template-columns: 1fr; }
}
</style>
