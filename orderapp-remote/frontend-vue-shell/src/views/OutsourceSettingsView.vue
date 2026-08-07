<template>
  <div class="page">
    <section v-if="templateAccessDenied" class="panel permission-note">
      <strong>当前账号没有代加工模板维护权限</strong>
      <p class="muted">仍可继续使用已开通的账单功能；如需维护模板，请联系管理员开通对应权限。</p>
      <button class="secondary" type="button" @click="loadTemplates">重新检查</button>
    </section>

    <section v-else class="panel">
      <div class="panel-head">
        <div>
          <h2>代加工费用模板</h2>
          <p class="muted">每次保存都会发布不可修改的新版本；历史工单账单继续使用生成时的版本和计算快照。</p>
        </div>
        <button class="secondary" type="button" @click="loadTemplates" :disabled="loading">刷新</button>
      </div>
      <div v-if="templateError" class="error">{{ templateError }}</div>
      <div v-if="templateOK" class="ok">已发布模板新版本</div>

      <div class="form-grid">
        <label>
          <span>模板名称</span>
          <input v-model.trim="form.name" placeholder="代加工默认模板" />
        </label>
        <label class="check">
          <input v-model="form.is_default" type="checkbox" />
          <span>设为默认模板</span>
        </label>
      </div>

      <div class="rule-head">
        <div>
          <strong>费用规则</strong>
          <span class="muted">只按真实完工数据计费</span>
        </div>
        <button class="secondary" type="button" @click="addRule">增加规则</button>
      </div>
      <div class="rules">
        <div v-for="(rule, index) in form.rules" :key="rule.local_id" class="rule-row">
          <label>
            <span>费用名称</span>
            <input v-model.trim="rule.name" placeholder="例如：烘焙费" />
          </label>
          <label>
            <span>费用类型</span>
            <select v-model="rule.fee_type">
              <option value="roasting">烘焙费</option>
              <option value="labor">人工费</option>
              <option value="material">物料费</option>
              <option value="processing">代加工费</option>
              <option value="packaging">包装费</option>
              <option value="storage">仓储费</option>
            </select>
          </label>
          <label>
            <span>计费来源</span>
            <select v-model="rule.basis">
              <option v-for="option in basisOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label>
            <span>单价/倍率</span>
            <input v-model.number="rule.unit_price" type="number" min="0" step="0.01" />
          </label>
          <button class="danger compact" type="button" @click="removeRule(index)">删除</button>
        </div>
        <div v-if="!form.rules.length" class="empty">请至少增加一条费用规则。</div>
      </div>

      <details class="legacy-fields">
        <summary>旧模板兼容价格字段</summary>
        <div class="form-grid legacy-grid">
          <label><span>烘焙单价</span><input v-model.number="form.roast_unit_price" type="number" min="0" step="0.01" /></label>
          <label><span>咖啡豆包装费单价</span><input v-model.number="form.bean_pack_unit_price" type="number" min="0" step="0.01" /></label>
          <label><span>挂耳包装费单价</span><input v-model.number="form.drip_pack_unit_price" type="number" min="0" step="0.01" /></label>
          <label><span>SC挂靠费单价</span><input v-model.number="form.sc_unit_price" type="number" min="0" step="0.01" /></label>
        </div>
      </details>
      <button class="primary" type="button" @click="saveTemplate" :disabled="saving || !form.rules.length">保存并发布新版本</button>
    </section>

    <section v-if="!templateAccessDenied" class="panel">
      <div class="section-title">现有模板与当前发布版本</div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>模板</th><th>默认</th><th>当前发布版本</th><th>规则</th></tr></thead>
          <tbody>
            <tr v-for="row in templates" :key="row.id">
              <td>{{ row.name }}</td>
              <td>{{ row.is_default ? '是' : '否' }}</td>
              <td>{{ row.current_version_no ? `V${row.current_version_no}` : '历史模板' }}</td>
              <td>
                <div v-if="row.rules?.length" class="rule-tags">
                  <span v-for="rule in row.rules" :key="rule.id">{{ rule.name }} · {{ basisLabel(rule.basis) }} · {{ money(rule.unit_price) }}</span>
                </div>
                <span v-else class="muted">暂无可计费规则</span>
              </td>
            </tr>
            <tr v-if="!templates.length"><td colspan="4" class="muted">暂无模板</td></tr>
          </tbody>
        </table>
      </div>
    </section>

    <section v-if="!billingAccessDenied" class="panel">
      <div class="panel-head">
        <div>
          <h2>客户生产工单账单</h2>
          <p class="muted">只允许选择已完工的客户真实工单；预览不会写入费用，确认后推送到账单。</p>
        </div>
        <button class="secondary" type="button" @click="loadBillingCandidates" :disabled="billingLoading || !billing.customer_id">刷新工单</button>
      </div>
      <div v-if="billingError" class="error">{{ billingError }}</div>
      <div v-if="billingOK" class="ok">{{ billingOK }}</div>
      <div class="form-grid">
        <label>
          <span>选择客户</span>
          <select v-model.number="billing.customer_id" @change="onCustomerChange">
            <option :value="0">请选择客户</option>
            <option v-for="row in customers" :key="row.customer_id || row.id" :value="Number(row.customer_id || row.id)">{{ row.customer_name || row.name }}</option>
          </select>
        </label>
        <label>
          <span>选择费用模板</span>
          <select v-model.number="billing.template_id" @change="clearPreview">
            <option :value="0">请选择模板</option>
            <option v-for="row in billableTemplates" :key="row.id" :value="Number(row.id)">{{ row.name }} / V{{ row.current_version_no }}</option>
          </select>
        </label>
      </div>

      <div class="section-title">选择已完工工单</div>
      <div class="candidate-list">
        <label v-for="row in candidates" :key="row.work_order_id" class="candidate" :class="{ billed: row.already_billed }">
          <input v-model="billing.work_order_ids" type="checkbox" :value="Number(row.work_order_id)" :disabled="row.already_billed" @change="clearPreview" />
          <span>
            <strong>{{ row.work_order_no }}</strong>
            <small>{{ row.product_name }}<template v-if="row.spec_g"> · {{ row.spec_g }}g</template> · {{ row.completed_at }}</small>
          </span>
          <em>{{ row.already_billed ? '已计费' : '可计费' }}</em>
        </label>
        <div v-if="billing.customer_id && !candidates.length" class="empty">该客户暂无已完工生产工单。</div>
      </div>
      <div class="action-row">
        <button class="secondary" type="button" @click="previewBill" :disabled="billingLoading || !canPreview">预览账单</button>
        <button class="primary" type="button" @click="confirmBill" :disabled="billingLoading || !preview">确认生成并推送账单</button>
      </div>

      <div v-if="preview" class="preview">
        <div class="preview-head">
          <strong>{{ preview.customer_name }} · {{ preview.template_name }} V{{ preview.template_version_no }}</strong>
          <strong>合计 ¥{{ money(preview.total_amount) }}</strong>
        </div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>工单</th><th>费用</th><th>计算来源</th><th>实际数量</th><th>单价/倍率</th><th>金额</th></tr></thead>
            <tbody>
              <tr v-for="(line, index) in preview.lines || []" :key="`${line.work_order_id}-${line.rule_id}-${index}`">
                <td>{{ line.work_order_no }}</td><td>{{ line.fee_name }}</td><td>{{ basisLabel(line.basis) }}</td>
                <td>{{ quantity(line.base_quantity) }}</td><td>{{ money(line.unit_price) }}</td><td>¥{{ money(line.amount) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="billing-lifecycle">
        <div class="panel-head lifecycle-head">
          <div>
            <h3>账单生命周期</h3>
            <p class="muted">付款、冲销和调整都会留痕；原账单计算快照不会被修改。</p>
          </div>
          <button class="secondary" type="button" @click="loadBillingRuns" :disabled="lifecycleLoading || !billing.customer_id">刷新账单</button>
        </div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>账单</th><th>类型</th><th>状态</th><th>金额</th><th>关联</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="row in billingRuns" :key="row.id">
                <td>{{ row.settlement_no }}</td>
                <td>{{ runKindLabel(row.run_kind) }}</td>
                <td>{{ runStatusLabel(row.status) }}</td>
                <td>¥{{ money(row.total_amount) }}</td>
                <td>{{ row.source_billing_run_id ? `原账单 #${row.source_billing_run_id}` : `${row.work_order_count || 0} 张工单` }}</td>
                <td>
                  <div class="row-actions">
                    <button v-if="row.status === 'confirmed'" class="secondary compact-action" type="button" @click="payBill(row)">登记付款</button>
                    <button v-if="canCorrectBill(row)" class="danger compact-action" type="button" @click="reverseBill(row)">冲销账单</button>
                    <button v-if="canCorrectBill(row)" class="secondary compact-action" type="button" @click="openAdjustment(row)">新增调整单</button>
                  </div>
                </td>
              </tr>
              <tr v-if="billing.customer_id && !billingRuns.length"><td colspan="6" class="muted">该客户暂无已确认账单。</td></tr>
            </tbody>
          </table>
        </div>

        <div v-if="adjustment.billing_run_id" class="adjustment-form">
          <div class="section-title">新增调整单 · 原账单 #{{ adjustment.billing_run_id }}</div>
          <div class="form-grid">
            <label><span>调整原因</span><input v-model.trim="adjustment.reason" placeholder="必填，说明补收或退费原因" /></label>
            <label>
              <span>费用类型</span>
              <select v-model="adjustment.fee_type">
                <option value="roasting">烘焙费</option><option value="labor">人工费</option><option value="material">物料费</option>
                <option value="packaging">包装费</option><option value="processing">代加工费</option><option value="storage">仓储费</option>
              </select>
            </label>
            <label><span>费用名称</span><input v-model.trim="adjustment.fee_name" placeholder="例如：补收人工费" /></label>
            <label><span>关联工单 ID（可选）</span><input v-model.number="adjustment.work_order_id" type="number" min="0" /></label>
            <label><span>调整金额（退费填负数）</span><input v-model.number="adjustment.amount" type="number" step="0.01" /></label>
          </div>
          <div class="action-row">
            <button class="primary" type="button" @click="submitAdjustment" :disabled="lifecycleLoading">确认新增调整单</button>
            <button class="secondary" type="button" @click="closeAdjustment">取消</button>
          </div>
        </div>
      </div>
    </section>

    <section v-else class="panel permission-note">
      <strong>当前账号没有代加工账单查看权限</strong>
      <p class="muted">模板维护仍可继续使用；如需预览或处理账单，请联系管理员开通财务权限。</p>
      <button class="secondary" type="button" @click="loadBillingOptions">重新检查</button>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const basisOptions = [
  { value: 'actual_input_kg', label: '实际投入（kg）' },
  { value: 'actual_output_kg', label: '实际产出（kg）' },
  { value: 'actual_minutes', label: '实际工序分钟' },
  { value: 'actual_units', label: '实际成品件数' },
  { value: 'fixed_per_work_order', label: '每工单固定费' },
  { value: 'factory_material_actual_cost', label: '工厂物料实际成本（倍率）' },
]

const loading = ref(false)
const saving = ref(false)
const templateError = ref('')
const templateOK = ref(false)
const templates = ref([])
const templateAccessDenied = ref(false)
let nextRuleID = 1
const form = reactive({
  name: '', is_default: false,
  roast_unit_price: 0, bean_pack_unit_price: 0, drip_pack_unit_price: 0, sc_unit_price: 0,
  rules: [],
})

const customers = ref([])
const billingTemplates = ref([])
const candidates = ref([])
const preview = ref(null)
const billingLoading = ref(false)
const billingError = ref('')
const billingOK = ref('')
const billingAccessDenied = ref(false)
const billing = reactive({ customer_id: 0, template_id: 0, work_order_ids: [] })
const billingRuns = ref([])
const lifecycleLoading = ref(false)
const adjustment = reactive({ billing_run_id: 0, reason: '', work_order_id: 0, fee_type: 'labor', fee_name: '', amount: 0 })
const billableTemplates = computed(() => billingTemplates.value.filter((row) => Number(row.current_version_id) > 0 && row.rules?.length))
const canPreview = computed(() => Number(billing.customer_id) > 0 && Number(billing.template_id) > 0 && billing.work_order_ids.length > 0)

function money(value) { return Number(value || 0).toFixed(2) }
function quantity(value) { return Number(value || 0).toFixed(4).replace(/\.?(0+)$/, '') || '0' }
function basisLabel(value) { return basisOptions.find((row) => row.value === value)?.label || value || '-' }
function runKindLabel(value) { return ({ standard: '正式账单', adjustment: '调整单', reversal: '冲销单' })[value] || value || '-' }
function runStatusLabel(value) { return ({ confirmed: '已确认', paid: '已付款', reversed: '已冲销' })[value] || value || '-' }
function canCorrectBill(row) { return ['confirmed', 'paid'].includes(row.status) && row.run_kind !== 'reversal' }
function addRule() {
  form.rules.push({ local_id: nextRuleID++, fee_type: 'processing', name: '', basis: 'actual_output_kg', unit_price: 0, sort_order: form.rules.length * 10 + 10 })
}
function removeRule(index) { form.rules.splice(index, 1) }
function resetForm() {
  form.name = ''; form.is_default = false
  form.roast_unit_price = 0; form.bean_pack_unit_price = 0; form.drip_pack_unit_price = 0; form.sc_unit_price = 0
  form.rules = []
  addRule()
}
function clearPreview() { preview.value = null; billingOK.value = '' }

async function loadTemplates() {
  loading.value = true; templateError.value = ''; templateAccessDenied.value = false
  try {
    const data = await apiGet('/api/outsource/templates')
    templates.value = data.rows || []
  } catch (err) {
    templateAccessDenied.value = err.status === 403
    templates.value = []
    if (!templateAccessDenied.value) templateError.value = err.message || '加载模板失败'
  }
  finally { loading.value = false }
}

async function loadBillingOptions() {
  billingLoading.value = true; billingError.value = ''; billingAccessDenied.value = false
  try {
    const data = await apiGet('/api/finance/customer-processing-billing/options')
    billingTemplates.value = data.options?.templates || []
    customers.value = data.options?.customers || []
    if (!billableTemplates.value.some((row) => Number(row.id) === Number(billing.template_id))) {
      billing.template_id = billableTemplates.value.length ? Number(billableTemplates.value[0].id) : 0
      clearPreview()
    }
  } catch (err) {
    billingAccessDenied.value = err.status === 403
    billingTemplates.value = []
    customers.value = []
    billing.template_id = 0
    if (!billingAccessDenied.value) billingError.value = err.message || '加载客户与计费模板失败'
  }
  finally { billingLoading.value = false }
}

async function saveTemplate() {
  saving.value = true; templateError.value = ''; templateOK.value = false
  try {
    await apiSend('/api/outsource/templates', { body: {
      name: form.name, is_default: Boolean(form.is_default),
      roast_unit_price: Number(form.roast_unit_price || 0), bean_pack_unit_price: Number(form.bean_pack_unit_price || 0),
      drip_pack_unit_price: Number(form.drip_pack_unit_price || 0), sc_unit_price: Number(form.sc_unit_price || 0),
      rules: form.rules.map((rule, index) => ({ fee_type: rule.fee_type, name: rule.name, basis: rule.basis, unit_price: Number(rule.unit_price || 0), sort_order: (index + 1) * 10 })),
    } })
    templateOK.value = true; resetForm(); await loadTemplates()
    if (!billingAccessDenied.value) await loadBillingOptions()
  } catch (err) { templateError.value = err.message || '发布模板失败' }
  finally { saving.value = false }
}

async function onCustomerChange() {
	billing.work_order_ids = []; billingRuns.value = []; closeAdjustment(); clearPreview()
	await Promise.all([loadBillingCandidates(), loadBillingRuns()])
}
async function loadBillingCandidates() {
  candidates.value = []
  if (!Number(billing.customer_id)) return
  billingLoading.value = true; billingError.value = ''
  try {
    const data = await apiGet(`/api/finance/customer-processing-billing/candidates?customer_id=${encodeURIComponent(billing.customer_id)}`)
    candidates.value = data.rows || []
  } catch (err) { billingError.value = err.message || '加载已完工工单失败' }
  finally { billingLoading.value = false }
}
async function previewBill() {
  billingLoading.value = true; billingError.value = ''; billingOK.value = ''
  try {
    const data = await apiSend('/api/finance/customer-processing-billing/preview', { body: {
      customer_id: Number(billing.customer_id), template_id: Number(billing.template_id), work_order_ids: billing.work_order_ids.map(Number),
    } })
    preview.value = data.preview
  } catch (err) { billingError.value = err.message || '预览账单失败'; preview.value = null }
  finally { billingLoading.value = false }
}
async function confirmBill() {
  if (!preview.value) return
  billingLoading.value = true; billingError.value = ''; billingOK.value = ''
  try {
    const data = await apiSend('/api/finance/customer-processing-billing/confirm', { body: {
      customer_id: Number(billing.customer_id), template_version_id: Number(preview.value.template_version_id), work_order_ids: billing.work_order_ids.map(Number),
    } })
    billingOK.value = `已生成并推送账单 ${data.result?.settlement_no || ''}`
    preview.value = null; billing.work_order_ids = []; await Promise.all([loadBillingCandidates(), loadBillingRuns()])
  } catch (err) { billingError.value = err.message || '确认账单失败' }
  finally { billingLoading.value = false }
}

async function loadBillingRuns() {
  billingRuns.value = []
  if (!Number(billing.customer_id)) return
  lifecycleLoading.value = true; billingError.value = ''
  try {
    const data = await apiGet(`/api/finance/customer-processing-billing/runs?customer_id=${encodeURIComponent(billing.customer_id)}`)
    billingRuns.value = data.rows || []
  } catch (err) { billingError.value = err.message || '加载账单生命周期失败' }
  finally { lifecycleLoading.value = false }
}

async function payBill(row) {
  if (!window.confirm(`确认登记账单 ${row.settlement_no} 已付款？`)) return
  lifecycleLoading.value = true; billingError.value = ''; billingOK.value = ''
  try {
    await apiSend(`/api/finance/customer-processing-billing/runs/${Number(row.id)}/pay`, { body: { note: 'ERP 登记付款' } })
    billingOK.value = `账单 ${row.settlement_no} 已登记付款`
    await loadBillingRuns()
  } catch (err) { billingError.value = err.message || '登记付款失败' }
  finally { lifecycleLoading.value = false }
}

async function reverseBill(row) {
  const reason = window.prompt(`请输入冲销账单 ${row.settlement_no} 的原因`)
  if (!String(reason || '').trim()) return
  lifecycleLoading.value = true; billingError.value = ''; billingOK.value = ''
  try {
    await apiSend(`/api/finance/customer-processing-billing/runs/${Number(row.id)}/reverse`, { body: { reason: String(reason).trim() } })
    billingOK.value = `账单 ${row.settlement_no} 已冲销，已生成负数冲销单`
    await loadBillingRuns()
  } catch (err) { billingError.value = err.message || '冲销账单失败' }
  finally { lifecycleLoading.value = false }
}

function openAdjustment(row) {
  adjustment.billing_run_id = Number(row.id)
  adjustment.reason = ''; adjustment.work_order_id = 0; adjustment.fee_type = 'labor'; adjustment.fee_name = ''; adjustment.amount = 0
}
function closeAdjustment() { adjustment.billing_run_id = 0; adjustment.reason = ''; adjustment.work_order_id = 0; adjustment.fee_name = ''; adjustment.amount = 0 }
async function submitAdjustment() {
  lifecycleLoading.value = true; billingError.value = ''; billingOK.value = ''
  try {
    const sourceID = Number(adjustment.billing_run_id)
    await apiSend(`/api/finance/customer-processing-billing/runs/${sourceID}/adjustments`, { body: {
      reason: adjustment.reason,
      lines: [{ work_order_id: Number(adjustment.work_order_id || 0), fee_type: adjustment.fee_type, fee_name: adjustment.fee_name, amount: Number(adjustment.amount || 0) }],
    } })
    billingOK.value = `原账单 #${sourceID} 已新增调整单`
    closeAdjustment(); await loadBillingRuns()
  } catch (err) { billingError.value = err.message || '新增调整单失败' }
  finally { lifecycleLoading.value = false }
}

onMounted(async () => { resetForm(); await Promise.all([loadTemplates(), loadBillingOptions()]) })
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 1180px; }
.panel-head, .rule-head, .preview-head, .action-row { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.panel-head { margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; } p { margin: 5px 0 0; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)); gap: 10px; margin-bottom: 12px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
.check { display: flex; align-items: center; gap: 8px; min-height: 38px; padding-top: 18px; }
.check input { width: 18px; height: 18px; }.check span { margin: 0; color: #222; font-size: 14px; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }.primary { background: #1f1f1f; color: #fff; }.secondary { background: #fff; color: #1f1f1f; }.danger { background: #fff; color: #9b2525; border-color: #d7aaaa; }.compact { align-self: end; }
.rule-head { margin: 8px 0; }.rule-head div { display: flex; align-items: baseline; gap: 10px; }
.rules { display: grid; gap: 8px; margin-bottom: 12px; }.rule-row { display: grid; grid-template-columns: 1.3fr 1fr 1.4fr .8fr auto; gap: 8px; padding: 10px; border: 1px solid #eee8df; border-radius: 7px; background: #fbfaf8; }
.legacy-fields { margin: 10px 0 12px; }.legacy-fields summary { cursor: pointer; color: #666; }.legacy-grid { margin-top: 10px; }
.section-title { font-weight: 700; margin-bottom: 10px; }.table-wrap { overflow: auto; } table { width: 100%; border-collapse: collapse; min-width: 760px; } th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; } th { background: #fbfaf8; }
.rule-tags { display: flex; flex-wrap: wrap; gap: 5px; }.rule-tags span { padding: 3px 7px; border-radius: 999px; background: #f2eee8; font-size: 12px; }
.candidate-list { display: grid; gap: 7px; margin: 8px 0 12px; }.candidate { display: grid; grid-template-columns: auto 1fr auto; gap: 10px; align-items: center; padding: 10px; border: 1px solid #e7e0d8; border-radius: 7px; }.candidate input { width: 18px; height: 18px; }.candidate span { color: #222; }.candidate small { display: block; margin-top: 3px; color: #666; }.candidate em { font-style: normal; color: #397039; }.candidate.billed { opacity: .65; }.candidate.billed em { color: #777; }
.action-row { justify-content: flex-start; margin-bottom: 12px; }.preview { border-top: 1px solid #eee8df; padding-top: 12px; }.preview-head { margin-bottom: 10px; }
.billing-lifecycle { border-top: 1px solid #eee8df; margin-top: 16px; padding-top: 14px; }.lifecycle-head h3 { margin: 0; }.row-actions { display: flex; flex-wrap: wrap; gap: 6px; }.compact-action { min-height: 30px; padding: 0 8px; font-size: 12px; }.adjustment-form { margin-top: 14px; padding: 12px; border: 1px solid #ded6cc; border-radius: 7px; background: #fbfaf8; }
.empty { color: #777; padding: 12px; border: 1px dashed #d7d0c8; border-radius: 6px; }.muted { color: #666; }.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
.permission-note { display: grid; justify-items: start; gap: 8px; }
@media (max-width: 900px) { .page { padding: 12px; }.form-grid, .rule-row { grid-template-columns: 1fr; }.check { padding-top: 0; }.compact { justify-self: start; }.panel-head { align-items: flex-start; } }
</style>
