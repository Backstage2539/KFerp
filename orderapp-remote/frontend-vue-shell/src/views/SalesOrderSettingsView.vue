<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>销售单设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>

      <div class="form-grid">
        <label><span>公司名称</span><input v-model.trim="form.company_name" /></label>
        <label><span>收款方式</span><input v-model.trim="form.payment_text" /></label>
        <label class="wide"><span>个性化说明</span><textarea v-model.trim="form.note" rows="3"></textarea></label>
      </div>
      <div class="actions">
        <button class="primary" type="button" @click="save" :disabled="saving">保存设置</button>
      </div>
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
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'

const loading = ref(false)
const saving = ref(false)
const uploadingPayment = ref(false)
const uploadingSeal = ref(false)
const error = ref('')
const ok = ref(false)
const settings = ref({})
const paymentFile = ref(null)
const sealFile = ref(null)

const form = reactive({
  company_name: '',
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
  form.company_name = data?.company_name || ''
  form.note = data?.note || ''
  form.payment_text = data?.payment_text || ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch('/api/settings/sales-order')
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    assignSettings(data)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    const res = await fetch('/api/settings/sales-order', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    assignSettings(data)
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
    const res = await fetch('/api/settings/sales-order/payment-codes', { method: 'POST', body })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '上传失败')
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
    const res = await fetch(`/api/settings/sales-order/payment-codes/${code.id}`, { method: 'DELETE' })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '停用失败')
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
    const res = await fetch('/api/settings/sales-order/seal', { method: 'POST', body })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '上传失败')
    sealFile.value = null
    await load()
  } catch (err) {
    error.value = err.message || '上传失败'
  } finally {
    uploadingSeal.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 1180px; }
.panel-head, .actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
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
.seal-row { grid-template-columns: 1fr minmax(220px, 1fr) 110px; }
.current-seal { color: #555; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; }
th { background: #fbfaf8; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid, .upload-row, .seal-row { grid-template-columns: 1fr; }
}
</style>
