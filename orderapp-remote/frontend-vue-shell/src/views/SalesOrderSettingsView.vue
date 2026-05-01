<template>
  <div class="page" :class="{ 'embedded-page': props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>销售单设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>

      <div class="form-grid">
        <label class="wide"><span>收款方式</span><textarea v-model.trim="form.payment_text" rows="2"></textarea></label>
        <label class="wide"><span>个性化说明</span><textarea v-model.trim="form.note" rows="3"></textarea></label>
      </div>
      <div class="actions">
        <button class="primary" type="button" @click="save" :disabled="saving">保存设置</button>
      </div>
      <details class="manual">
        <summary>销售单设置手册</summary>
        <ul>
          <li>公司名称在“公司设置”里维护；本页只维护销售单说明、收款方式、收款码和公章。</li>
          <li>公账收款信息在“公司设置”里维护；为空时销售单不展示公账信息。</li>
          <li>收款码支持多个，名称和说明会随 PDF 一起展示。</li>
          <li>公章可上传图片后点击“去除背景”生成透明 PNG；也可拖动调整盖在公司名称上的位置，并调整公章大小，调整后只影响新生成版本。</li>
        </ul>
      </details>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>收款码</h3>
      </div>
      <div class="upload-row">
        <input v-model.trim="paymentForm.label" placeholder="名称，如微信/支付宝" />
        <input v-model.trim="paymentForm.description" placeholder="说明，如扫码付款" />
        <input v-model.number="paymentForm.sort" type="number" placeholder="排序" />
        <input type="file" accept="image/*" @change="handlePaymentFile" />
        <button class="primary" type="button" @click="uploadPaymentCode" :disabled="uploadingPayment || !paymentFile">上传</button>
      </div>
      <table>
        <thead>
          <tr><th>名称</th><th>说明</th><th>文件</th><th>排序</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="code in settings.payment_codes || []" :key="code.id">
            <td>{{ code.label }}</td>
            <td>{{ code.description || '-' }}</td>
            <td>{{ code.asset?.filename || '-' }}</td>
            <td>{{ code.sort }}</td>
            <td><button class="secondary" type="button" @click="deletePaymentCode(code)" :disabled="saving">停用</button></td>
          </tr>
          <tr v-if="!(settings.payment_codes || []).length"><td colspan="5" class="muted">暂无收款码</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>公章</h3>
      </div>
      <div class="upload-row seal-row">
        <div class="current-seal">当前：{{ settings.seal?.filename || '未设置' }}</div>
        <input type="file" accept="image/*" @change="handleSealFile" />
        <button class="primary" type="button" @click="uploadSeal" :disabled="uploadingSeal || !sealFile">上传公章</button>
        <button class="secondary" type="button" @click="removeSealBackground" :disabled="removingSealBackground || !settings.seal">{{ removingSealBackground ? '处理中' : '去除背景' }}</button>
      </div>
      <div class="seal-position">
        <div
          ref="sealStage"
          class="seal-position-stage"
          @pointerdown="startSealDrag"
        >
          <div class="company-line">公司：公司名称</div>
          <img
            v-if="settings.seal?.url"
            class="seal-drag-image"
            :src="settings.seal.url"
            alt="公章"
            :style="sealDragStyle"
          />
          <div v-else class="seal-placeholder" :style="sealDragStyle">公章</div>
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
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const props = defineProps({
  embedded: { type: Boolean, default: false },
})

const loading = ref(false)
const saving = ref(false)
const uploadingPayment = ref(false)
const uploadingSeal = ref(false)
const removingSealBackground = ref(false)
const error = ref('')
const ok = ref(false)
const settings = ref({})
const paymentFile = ref(null)
const sealFile = ref(null)
const sealStage = ref(null)

const form = reactive({
  note: '',
  payment_text: '',
  seal_x_mm: 32,
  seal_y_mm: 22,
  seal_width_mm: 42,
})

const paymentForm = reactive({
  label: '',
  description: '',
  sort: 0,
})

function assignSettings(data) {
  settings.value = data || {}
  form.note = data?.note || ''
  form.payment_text = data?.payment_text || ''
  form.seal_x_mm = Number(data?.seal_x_mm || 32)
  form.seal_y_mm = Number(data?.seal_y_mm || 22)
  form.seal_width_mm = Number(data?.seal_width_mm || 42)
}

const sealDragStyle = computed(() => {
  const scale = 2.2
  const width = Math.max(20, Number(form.seal_width_mm || 42)) * scale
  return {
    left: `${Math.max(0, Number(form.seal_x_mm || 0)) * scale}px`,
    top: `${Math.max(0, Number(form.seal_y_mm || 0)) * scale}px`,
    width: `${width}px`,
    height: `${width * 0.62}px`,
  }
})

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
  if (!sealStage.value) return
  const stage = sealStage.value
  const rect = stage.getBoundingClientRect()
  const scaleX = 210 / rect.width
  const scaleY = 84 / rect.height
  const update = (clientX, clientY) => {
    form.seal_x_mm = Math.max(0, Math.round((clientX - rect.left) * scaleX))
    form.seal_y_mm = Math.max(0, Math.round((clientY - rect.top) * scaleY))
  }
  update(event.clientX, event.clientY)
  const move = (moveEvent) => update(moveEvent.clientX, moveEvent.clientY)
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up, { once: true })
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    assignSettings(await apiSend('/api/settings/sales-order', { body: { ...form } }))
    ok.value = true
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

function handlePaymentFile(event) {
  paymentFile.value = event.target.files?.[0] || null
}

function handleSealFile(event) {
  sealFile.value = event.target.files?.[0] || null
}

async function uploadPaymentCode() {
  if (!paymentFile.value) return
  uploadingPayment.value = true
  error.value = ''
  try {
    const body = new FormData()
    body.append('file', paymentFile.value)
    body.append('label', paymentForm.label || paymentFile.value.name)
    body.append('description', paymentForm.description)
    body.append('sort', String(paymentForm.sort || 0))
    await apiSend('/api/settings/sales-order/payment-codes', { body })
    paymentFile.value = null
    paymentForm.label = ''
    paymentForm.description = ''
    paymentForm.sort = 0
    await load()
  } catch (err) {
    error.value = err.message || '上传失败'
  } finally {
    uploadingPayment.value = false
  }
}

async function deletePaymentCode(code) {
  if (!code?.id) return
  saving.value = true
  error.value = ''
  try {
    await apiSend(`/api/settings/sales-order/payment-codes/${code.id}`, { method: 'DELETE' })
    await load()
  } catch (err) {
    error.value = err.message || '停用失败'
  } finally {
    saving.value = false
  }
}

async function uploadSeal() {
  if (!sealFile.value) return
  uploadingSeal.value = true
  error.value = ''
  try {
    const body = new FormData()
    body.append('file', sealFile.value)
    await apiSend('/api/settings/sales-order/seal', { body })
    sealFile.value = null
    await load()
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
.page { padding: 18px; color: #171717; }
.embedded-page { padding: 0; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 1180px; }
.embedded-page .panel { max-width: none; }
.panel-head, .actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
.manual { border-top: 1px solid #edf0f5; padding-top: 10px; margin-top: 12px; color: #4b5563; font-size: 13px; }
.manual summary { cursor: pointer; font-weight: 700; color: #111827; }
.manual ul { margin: 8px 0 0; padding-left: 18px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 10px; margin-bottom: 12px; }
.wide { grid-column: 1 / -1; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input { height: 38px; }
textarea { resize: vertical; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.upload-row { display: grid; grid-template-columns: 180px 1fr 100px minmax(220px, 1fr) 90px; gap: 10px; align-items: center; margin-bottom: 12px; }
.seal-row { grid-template-columns: 1fr minmax(220px, 1fr) 110px 110px; }
.current-seal { color: #555; }
.seal-position { display: grid; grid-template-columns: minmax(320px, 462px) 1fr; gap: 12px; align-items: start; }
.seal-position-stage { position: relative; width: 100%; aspect-ratio: 2.5 / 1; border: 1px dashed #d2c8bc; border-radius: 8px; background: #fffdf9; overflow: hidden; cursor: crosshair; }
.company-line { position: absolute; left: 35px; top: 48px; font-weight: 700; }
.seal-drag-image, .seal-placeholder { position: absolute; user-select: none; pointer-events: none; object-fit: contain; opacity: .86; }
.seal-placeholder { border: 2px solid #b91c1c; border-radius: 999px; color: #b91c1c; display: grid; place-items: center; font-weight: 800; }
.seal-position-fields { display: grid; grid-template-columns: repeat(3, minmax(80px, 1fr)); gap: 10px; }
.seal-size-control { grid-column: 1 / -1; }
.seal-size-inputs { display: grid; grid-template-columns: minmax(180px, 1fr) 88px; gap: 10px; align-items: center; }
.seal-size-inputs input[type="range"] { padding: 0; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; }
th { background: #fbfaf8; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid, .upload-row, .seal-row, .seal-position { grid-template-columns: 1fr; }
  .seal-position-fields { grid-template-columns: 1fr; }
}
</style>
