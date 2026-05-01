<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>公司设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>

      <div class="form-grid">
        <label><span>公司名称</span><input v-model.trim="form.company_name" required /></label>
        <label><span>联系电话</span><input v-model.trim="form.company_phone" /></label>
        <label class="wide"><span>公司地址</span><textarea v-model.trim="form.company_address" rows="3"></textarea></label>
        <div class="wide account-settings">
          <div class="account-head">
            <h3>公账收款设置</h3>
            <button class="secondary" type="button" @click="copyAccountInfo" :disabled="!hasAccountInfo">{{ accountCopied ? '已复制' : '一键复制公账收款信息' }}</button>
          </div>
          <div class="account-grid">
            <label><span>纳税人识别号</span><input v-model.trim="form.taxpayer_id" name="taxpayer_id" /></label>
            <label><span>户名</span><input v-model.trim="form.bank_account_name" name="bank_account_name" /></label>
            <label><span>开户行</span><input v-model.trim="form.bank_name" name="bank_name" /></label>
            <label><span>账号</span><input v-model.trim="form.bank_account_no" name="bank_account_no" /></label>
          </div>
        </div>
      </div>
      <div class="actions">
        <button class="primary" type="button" @click="save" :disabled="saving">{{ saving ? '保存中' : '保存公司设置' }}</button>
      </div>
      <details class="manual">
        <summary>公司设置手册</summary>
        <ul>
          <li>公司名称作为销售单抬头使用。</li>
          <li>公司地址、纳税人识别号和公账收款信息会进入新生成的销售单。</li>
          <li>“一键复制公账收款信息”用于快速发给客户，不会修改数据。</li>
          <li>销售单设置只维护销售单专用说明、收款方式、收款码和公章。</li>
        </ul>
      </details>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const accountCopied = ref(false)

const form = reactive({
  company_name: '',
  company_address: '',
  company_phone: '',
  taxpayer_id: '',
  bank_account_name: '',
  bank_name: '',
  bank_account_no: '',
})

function assign(data = {}) {
  form.company_name = data.company_name || ''
  form.company_address = data.company_address || ''
  form.company_phone = data.company_phone || ''
  form.taxpayer_id = data.taxpayer_id || ''
  form.bank_account_name = data.bank_account_name || ''
  form.bank_name = data.bank_name || ''
  form.bank_account_no = data.bank_account_no || ''
}

const hasAccountInfo = computed(() => Boolean(
  form.company_address ||
  form.taxpayer_id ||
  form.bank_account_name ||
  form.bank_name ||
  form.bank_account_no,
))

async function load() {
  loading.value = true
  error.value = ''
  try {
    assign(await apiGet('/api/company/profile'))
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
    assign(await apiSend('/api/company/profile', { method: 'POST', body: { ...form } }))
    ok.value = true
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function copyAccountInfo() {
  const lines = [
    form.bank_account_name ? `户名：${form.bank_account_name}` : '',
    form.taxpayer_id ? `纳税人识别号：${form.taxpayer_id}` : '',
    form.company_address ? `地址：${form.company_address}` : '',
    form.bank_name ? `开户行：${form.bank_name}` : '',
    form.bank_account_no ? `账号：${form.bank_account_no}` : '',
  ].filter(Boolean)
  if (!lines.length) return
  accountCopied.value = false
  try {
    await navigator.clipboard.writeText(lines.join('\n'))
    accountCopied.value = true
    window.setTimeout(() => {
      accountCopied.value = false
    }, 1600)
  } catch (err) {
    error.value = '复制失败，请手动选择内容复制'
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 960px; }
.panel-head, .actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 10px; margin-bottom: 12px; }
.wide { grid-column: 1 / -1; }
.account-settings { border: 1px solid #eee2d4; border-radius: 8px; padding: 12px; background: #fffdf9; }
.account-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 10px; }
.account-head h3 { margin: 0; font-size: 16px; }
.account-grid { display: grid; grid-template-columns: repeat(2, minmax(180px, 1fr)); gap: 10px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input { height: 38px; }
textarea { resize: vertical; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.manual { border-top: 1px solid #edf0f5; padding-top: 10px; margin-top: 12px; color: #4b5563; font-size: 13px; }
.manual summary { cursor: pointer; font-weight: 700; color: #111827; }
.manual ul { margin: 8px 0 0; padding-left: 18px; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid, .account-grid { grid-template-columns: 1fr; }
  .account-head { align-items: stretch; flex-direction: column; }
}
</style>
