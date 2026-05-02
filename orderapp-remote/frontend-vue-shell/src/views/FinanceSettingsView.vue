<template>
  <div class="page">
    <section class="panel head">
      <div>
        <h2>财务设置</h2>
        <p>{{ companyTypeLabel(form.company_type) }} · {{ taxpayerTypeLabel(form.taxpayer_type) }}</p>
      </div>
      <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
    </section>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="ok" class="ok">已保存</div>

    <section class="panel">
      <div class="section-title">经营与纳税身份</div>
      <div class="form-grid identity">
        <label>
          <span>企业类型</span>
          <select v-model="form.company_type">
            <option value="coffee_roaster">咖啡烘焙厂</option>
            <option value="coffee_trader">咖啡贸易商</option>
            <option value="coffee_processor">咖啡壳豆加工厂</option>
            <option value="combined">综合咖啡工厂</option>
          </select>
        </label>
        <label>
          <span>纳税人类型</span>
          <select v-model="form.taxpayer_type">
            <option value="small_scale">小规模纳税人</option>
            <option value="general">一般纳税人</option>
          </select>
        </label>
        <label>
          <span>申报周期</span>
          <select v-model="form.declaration_period">
            <option value="monthly">按月</option>
            <option value="quarterly">按季</option>
          </select>
        </label>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">税率参数</div>
      <div class="form-grid">
        <label>
          <span>小规模增值税率 %</span>
          <input v-model="form.small_scale_vat_rate" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>小规模免税阈值</span>
          <input v-model="form.small_scale_vat_threshold" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>一般纳税人销项 %</span>
          <input v-model="form.general_output_vat_rate" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>默认进项 %</span>
          <input v-model="form.default_input_vat_rate" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>附加税率 %</span>
          <input v-model="form.surtax_rate" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>企业所得税率 %</span>
          <input v-model="form.cit_standard_rate" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>小微有效税率 %</span>
          <input v-model="form.small_low_profit_effective_rate" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>小微年利润上限</span>
          <input v-model="form.small_low_profit_annual_profit_limit" type="number" min="0" step="0.01" />
        </label>
        <label class="check">
          <input v-model="form.small_low_profit_enabled" type="checkbox" />
          <span>启用小微优惠估算</span>
        </label>
      </div>
      <div class="footer">
        <button type="button" @click="save" :disabled="saving">保存设置</button>
      </div>
    </section>

    <section v-if="settings.can_manage_close_mode" class="panel private">
      <div class="section-title">锁账模式</div>
      <div class="mode-row">
        <button
          type="button"
          :class="{ active: settings.closing_mode === 'strong_lock' }"
          @click="switchMode('strong_lock')"
          :disabled="saving">
          强锁账
        </button>
        <button
          type="button"
          :class="{ active: settings.closing_mode === 'light_confirmation' }"
          @click="switchMode('light_confirmation')"
          :disabled="saving">
          轻确认
        </button>
        <span>{{ closingModeLabel(settings.closing_mode) }}</span>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { fetchFinanceSettings, saveFinanceSettings, switchFinanceClosingMode } from '../api/finance.js'
import {
  closingModeLabel,
  companyTypeLabel,
  rateFromPercent,
  rateToPercent,
  taxpayerTypeLabel,
} from '../lib/finance.js'

const settings = ref({})
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const form = reactive({
  company_type: 'coffee_roaster',
  taxpayer_type: 'small_scale',
  declaration_period: 'monthly',
  small_scale_vat_rate: '3',
  small_scale_vat_threshold: 100000,
  general_output_vat_rate: '13',
  default_input_vat_rate: '13',
  surtax_rate: '12',
  cit_standard_rate: '25',
  small_low_profit_enabled: true,
  small_low_profit_effective_rate: '5',
  small_low_profit_annual_profit_limit: 3000000,
})

function assignForm(data) {
  form.company_type = data.company_type || 'coffee_roaster'
  form.taxpayer_type = data.taxpayer_type || 'small_scale'
  form.declaration_period = data.declaration_period || 'monthly'
  form.small_scale_vat_rate = rateToPercent(data.small_scale_vat_rate ?? 0.03)
  form.small_scale_vat_threshold = Number(data.small_scale_vat_threshold ?? 100000)
  form.general_output_vat_rate = rateToPercent(data.general_output_vat_rate ?? 0.13)
  form.default_input_vat_rate = rateToPercent(data.default_input_vat_rate ?? 0.13)
  form.surtax_rate = rateToPercent(data.surtax_rate ?? 0.12)
  form.cit_standard_rate = rateToPercent(data.cit_standard_rate ?? 0.25)
  form.small_low_profit_enabled = data.small_low_profit_enabled !== false
  form.small_low_profit_effective_rate = rateToPercent(data.small_low_profit_effective_rate ?? 0.05)
  form.small_low_profit_annual_profit_limit = Number(data.small_low_profit_annual_profit_limit ?? 3000000)
}

function payload() {
  return {
    company_type: form.company_type,
    taxpayer_type: form.taxpayer_type,
    declaration_period: form.declaration_period,
    closing_mode: settings.value.closing_mode || 'strong_lock',
    small_scale_vat_rate: rateFromPercent(form.small_scale_vat_rate),
    small_scale_vat_threshold: Number(form.small_scale_vat_threshold || 0),
    general_output_vat_rate: rateFromPercent(form.general_output_vat_rate),
    default_input_vat_rate: rateFromPercent(form.default_input_vat_rate),
    surtax_rate: rateFromPercent(form.surtax_rate),
    cit_standard_rate: rateFromPercent(form.cit_standard_rate),
    small_low_profit_enabled: !!form.small_low_profit_enabled,
    small_low_profit_effective_rate: rateFromPercent(form.small_low_profit_effective_rate),
    small_low_profit_annual_profit_limit: Number(form.small_low_profit_annual_profit_limit || 0),
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchFinanceSettings()
    settings.value = data || {}
    assignForm(settings.value)
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
    const data = await saveFinanceSettings(payload())
    settings.value = data || {}
    assignForm(settings.value)
    ok.value = true
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function switchMode(mode) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    const data = await switchFinanceClosingMode(mode)
    settings.value = data || {}
    assignForm(settings.value)
    ok.value = true
  } catch (err) {
    error.value = err.message || '切换失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.panel { border: 1px solid #e3e3e3; border-radius: 8px; background: #fff; padding: 14px; }
.head { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
h2 { margin: 0 0 4px; font-size: 20px; }
p { margin: 0; color: #666; }
.section-title { font-weight: 800; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; align-items: end; }
.identity { grid-template-columns: repeat(3, minmax(0, 1fr)); }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfcfcf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
.check { display: flex; align-items: center; gap: 8px; min-height: 38px; }
.check input { width: 18px; height: 18px; }
.check span { margin: 0; color: #171717; font-size: 14px; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; background: #1f1f1f; color: #fff; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary, .mode-row button { background: #fff; color: #1f1f1f; }
.mode-row button.active { background: #1f1f1f; color: #fff; }
.footer { display: flex; justify-content: flex-end; margin-top: 12px; }
.mode-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.mode-row span { color: #666; }
.private { border-style: dashed; }
.error, .ok { border-radius: 6px; padding: 10px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .head, .form-grid, .identity { display: grid; grid-template-columns: 1fr; }
  .footer { justify-content: stretch; }
  .footer button { width: 100%; }
}
</style>
