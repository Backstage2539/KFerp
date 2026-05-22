<template>
  <div class="page" :class="{ 'embedded-page': props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>销售单设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>

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
          <li>公司名称和公账收款信息在“公司设置”里维护；本页只维护销售单说明、收款方式、收款码和公章资产。</li>
          <li>文字位置和大小、收款码位置和大小请在销售单预览中拖动调整；拖右下角圆点可调整文本框或收款码区域大小。</li>
          <li>个性化说明会优先显示在公账收款前面，公账信息太长时优先裁掉后面的公账信息。</li>
          <li>上传公章时会自动裁掉图片白边；旧公章可点击“去除背景”重新生成透明 PNG。</li>
          <li>销售单、出库单和合同盖章共用这一套公章资产；上传后可在各页面选择要使用的公章。</li>
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
          <tr><th>名称</th><th>说明</th><th>文件</th><th>排序</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="code in settings.payment_codes || []" :key="code.id" :class="{ 'payment-code-inactive': !code.active }">
            <td>{{ code.label }}</td>
            <td>{{ code.description || '-' }}</td>
            <td>{{ code.asset?.filename || '-' }}</td>
            <td>{{ code.sort }}</td>
            <td><span class="status-pill" :class="{ inactive: !code.active }">{{ code.active ? '启用' : '停用' }}</span></td>
            <td>
              <div class="row-actions">
                <button v-if="code.active" class="secondary" type="button" @click="deactivatePaymentCode(code)" :disabled="saving">停用</button>
                <button v-else class="secondary" type="button" @click="activatePaymentCode(code)" :disabled="saving">启用</button>
                <button class="secondary danger" type="button" @click="removePaymentCode(code)" :disabled="saving">删除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!(settings.payment_codes || []).length"><td colspan="6" class="muted">暂无收款码</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>公章</h3>
      </div>
      <div class="seal-toolbar">
        <label>
          <span>当前公章</span>
          <select v-model.number="selectedSealID" :disabled="loading || !seals.length" @change="selectSeal">
            <option :value="0">未选择</option>
            <option v-for="seal in seals" :key="seal.id" :value="seal.id">{{ seal.filename || `公章 ${seal.id}` }}</option>
          </select>
        </label>
        <div class="current-seal">当前：{{ settings.seal?.filename || '未设置' }}</div>
        <input type="file" accept="image/*" @change="handleSealFile" />
        <button class="primary" type="button" @click="uploadSeal" :disabled="uploadingSeal || !sealFile">上传公章</button>
        <button class="secondary" type="button" @click="removeSealBackground" :disabled="removingSealBackground || !settings.seal">{{ removingSealBackground ? '处理中' : '去除背景' }}</button>
      </div>
      <div v-if="seals.length" class="seal-list">
        <div v-for="seal in seals" :key="seal.id" class="seal-card" :class="{ active: Number(seal.id) === Number(selectedSealID) }">
          <img v-if="seal.url" :src="seal.url" alt="公章预览" />
          <div>
            <strong>{{ seal.filename || `公章 ${seal.id}` }}</strong>
            <span>{{ seal.created_at || '-' }}</span>
          </div>
        </div>
      </div>
      <div v-else class="muted">暂无公章，请先上传。</div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
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
const ok = ref('')
const settings = ref({})
const seals = ref([])
const selectedSealID = ref(0)
const paymentFile = ref(null)
const sealFile = ref(null)

const form = reactive({
  note: '',
  payment_text: '',
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
  selectedSealID.value = Number(data?.seal?.id || selectedSealID.value || 0)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [settingsData, sealData] = await Promise.all([
      apiGet('/api/settings/sales-order'),
      apiGet('/api/settings/sales-order/seals'),
    ])
    seals.value = sealData.rows || []
    selectedSealID.value = Number(sealData.current_id || settingsData?.seal?.id || 0)
    assignSettings(settingsData)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    assignSettings(await apiSend('/api/settings/sales-order', { body: preservedSettingsPayload() }))
    ok.value = '已保存'
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

function preservedSettingsPayload() {
  const data = settings.value || {}
  return {
    note: form.note,
    payment_text: form.payment_text,
    seal_x_mm: Number(data.seal_x_mm || 32),
    seal_y_mm: Number(data.seal_y_mm || 5),
    seal_width_mm: Number(data.seal_width_mm || 36),
    payment_text_x_mm: Number(data.payment_text_x_mm || 16),
    payment_text_y_mm: Number(data.payment_text_y_mm || 118),
    payment_text_width_mm: Number(data.payment_text_width_mm || 104),
    payment_text_height_mm: Number(data.payment_text_height_mm || 78),
    payment_code_x_mm: Number(data.payment_code_x_mm || 126),
    payment_code_y_mm: Number(data.payment_code_y_mm || 106),
    payment_code_width_mm: Number(data.payment_code_width_mm || 72),
    payment_code_height_mm: Number(data.payment_code_height_mm || 122),
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

async function deactivatePaymentCode(code) {
  if (!code?.id) return
  saving.value = true
  error.value = ''
  try {
    await apiSend(`/api/settings/sales-order/payment-codes/${code.id}/deactivate`, { method: 'POST' })
    ok.value = '收款码已停用'
    await load()
  } catch (err) {
    error.value = err.message || '停用失败'
  } finally {
    saving.value = false
  }
}

async function removePaymentCode(code) {
  if (!code?.id) return
  if (!window.confirm(`删除收款码“${code.label || code.id}”？`)) return
  saving.value = true
  error.value = ''
  try {
    await apiSend(`/api/settings/sales-order/payment-codes/${code.id}`, { method: 'DELETE' })
    ok.value = '收款码已删除'
    await load()
  } catch (err) {
    error.value = err.message || '删除失败'
  } finally {
    saving.value = false
  }
}

async function activatePaymentCode(code) {
  if (!code?.id) return
  saving.value = true
  error.value = ''
  try {
    await apiSend(`/api/settings/sales-order/payment-codes/${code.id}/activate`, { method: 'POST' })
    ok.value = '收款码已启用'
    await load()
  } catch (err) {
    error.value = err.message || '启用失败'
  } finally {
    saving.value = false
  }
}

async function selectSeal() {
  if (!selectedSealID.value) return
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    assignSettings(await apiSend('/api/settings/sales-order/seal/select', {
      body: { asset_id: selectedSealID.value },
    }))
    ok.value = '公章已切换'
    await load()
  } catch (err) {
    error.value = err.message || '选择公章失败'
  } finally {
    saving.value = false
  }
}

async function uploadSeal() {
  if (!sealFile.value) return
  uploadingSeal.value = true
  error.value = ''
  ok.value = ''
  try {
    const body = new FormData()
    body.append('file', sealFile.value)
    await apiSend('/api/settings/sales-order/seal', { body })
    sealFile.value = null
    await load()
    ok.value = '公章已上传并裁掉图片白边'
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
  ok.value = ''
  try {
    await apiSend('/api/settings/sales-order/seal/remove-background')
    await load()
    ok.value = '已保存'
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
input, select, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input, select { height: 38px; }
textarea { resize: vertical; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.upload-row { display: grid; grid-template-columns: 180px 1fr 100px minmax(220px, 1fr) 90px; gap: 10px; align-items: center; margin-bottom: 12px; }
.seal-toolbar { display: grid; grid-template-columns: minmax(190px, 1fr) minmax(160px, 1fr) minmax(220px, 1fr) auto auto; gap: 10px; align-items: end; margin-bottom: 12px; }
.current-seal { color: #555; align-self: center; }
.seal-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 10px; }
.seal-card { display: grid; grid-template-columns: 54px minmax(0, 1fr); gap: 10px; align-items: center; border: 1px solid #eee8df; border-radius: 8px; padding: 8px; background: #fffdf9; }
.seal-card.active { border-color: #1f1f1f; box-shadow: 0 0 0 1px #1f1f1f inset; }
.seal-card img { width: 54px; height: 54px; object-fit: contain; }
.seal-card strong, .seal-card span { display: block; min-width: 0; overflow-wrap: anywhere; }
.seal-card span { color: #666; font-size: 12px; margin-top: 3px; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; }
th { background: #fbfaf8; }
.payment-code-inactive { color: #777; background: #fcfcfc; }
.status-pill { display: inline-flex; align-items: center; min-height: 24px; padding: 2px 8px; border-radius: 999px; background: #e7f8ed; color: #166534; font-size: 13px; font-weight: 600; }
.status-pill.inactive { background: #f2f4f7; color: #667085; }
.row-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.danger { border-color: #b91c1c; color: #b91c1c; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid, .upload-row, .seal-toolbar { grid-template-columns: 1fr; }
}
</style>
