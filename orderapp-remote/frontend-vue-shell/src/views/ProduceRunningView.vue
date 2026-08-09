<template>
  <div class="page">
    <ProductionTopNav active-key="produceRunning" />

    <section class="toolbar">
      <div>
        <h2>生产中</h2>
        <p>{{ rows.length }} 个批次项目正在生产</p>
      </div>
      <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
    </section>

    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="error" class="error">
      <div>
        <strong>{{ finishErrorDetail.reason }}</strong>
        <span>{{ finishErrorDetail.affectedObject || error }}</span>
      </div>
      <button v-if="finishErrorDetail.actionKey" class="secondary small-button" type="button" :aria-label="finishErrorDetail.action || '打开库存作业'" @click="openErrorAction">
        {{ finishErrorDetail.action }}
      </button>
    </div>

    <section class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>开始时间</th>
            <th>批次</th>
            <th>产品</th>
            <th>订单号</th>
            <th>计划摘要</th>
            <th>主动作</th>
            <th>取消</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.started_at }}</td>
            <td>{{ row.batch_id }}</td>
            <td>{{ row.product_name }}</td>
            <td class="muted">{{ row.order_nos }}</td>
            <td>
              <div class="plan-summary">
                <span>规格 {{ row.outputs?.length ? '多规格' : row.spec_g }}</span>
                <span>需求 {{ row.need_g }}g</span>
                <span>投料 {{ row.input_g }}g</span>
              </div>
            </td>
            <td>
              <button class="primary" type="button" @click="openCompletionPanel(row)" :disabled="busyId === row.id">
                完成/部分完成
              </button>
            </td>
            <td>
              <button class="danger" type="button" @click="cancel(row)" :disabled="busyId === row.id">
                取消生产
              </button>
            </td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="7" class="empty">暂无生产中项目</td>
          </tr>
        </tbody>
      </table>
    </section>

    <section v-if="completion.open" class="completion-panel">
      <div class="completion-head">
        <div>
          <h3>{{ completion.partial ? '部分完成' : '完成生产' }}</h3>
          <p>{{ completion.row?.batch_id || '-' }} · {{ completion.row?.product_name || '-' }}</p>
        </div>
        <button class="secondary" type="button" @click="completion.open = false">关闭</button>
      </div>
      <div class="completion-grid">
        <label>
          <span>投料(g)</span>
          <input v-model.number="activeCompletionInput.consumed_input_g" min="0" type="number" />
        </label>
        <div v-if="activeCompletionInput.outputs?.length" class="multi-output-grid">
          <label v-for="output in activeCompletionInput.outputs" :key="output.spec_g">
            <span>{{ output.spec_g }}g 成品件数</span>
            <input v-model.number="output.finished_units" min="0" type="number" />
          </label>
          <label v-for="output in activeCompletionInput.outputs" :key="`${output.spec_g}-loose`">
            <span>{{ output.spec_g }}g 余料(g)</span>
            <input v-model.number="output.finished_loose_g" min="0" type="number" />
          </label>
        </div>
        <template v-else>
          <label>
            <span>成品件数</span>
            <input v-model.number="activeCompletionInput.finished_units" min="0" type="number" />
          </label>
          <label>
            <span>余料(g)</span>
            <input v-model.number="activeCompletionInput.finished_loose_g" min="0" type="number" />
          </label>
        </template>
        <label>
          <span>入库仓</span>
          <select v-model="activeCompletionInput.warehouse" aria-label="成品入库仓">
            <option v-for="wh in finishedWarehouses" :key="wh.code" :value="wh.code">{{ wh.name }}</option>
          </select>
        </label>
        <label class="partial-check completion-check">
          <input v-model="completion.partial" type="checkbox" />
          部分完工（保留剩余）
        </label>
        <label class="completion-note">
          <span>异常/备注</span>
          <input v-model.trim="completion.note" placeholder="可记录异常或交接说明；当前不改变后端完工口径" />
        </label>
        <div class="completion-yield">
          <span>实际产出率</span>
          <strong>{{ formatActualYield(completion.row, activeCompletionInput) }}</strong>
        </div>
      </div>
      <div class="completion-actions">
        <button class="secondary" type="button" @click="completion.open = false">取消</button>
        <button class="primary" type="button" @click="submitCompletionPanel" :disabled="busyId === completion.row?.id">
          {{ completion.partial ? '记录部分完成' : '完成并入库' }}
        </button>
      </div>
    </section>

    <div v-if="stockDrawerOpen" class="drawer-mask" @click.self="stockDrawerOpen = false">
      <aside class="stock-drawer" aria-label="库存作业">
        <div class="drawer-head">
          <div>
            <h3>{{ isWipInsufficientError ? 'WIP库存不足' : '库存作业' }}</h3>
            <p>先把原料从原料仓领到 WIP，再回到生产中完成生产。</p>
          </div>
          <button class="secondary" type="button" @click="stockDrawerOpen = false">关闭</button>
        </div>
        <StockOperationsView embedded initial-tab="stockEntries" :view-params="{ tab: 'stockEntries' }" />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet } from '../api/client'
import { cancelRunningProduction, fetchRunningProduction, finishRunningProduction } from '../api/production.js'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import { buildFinishInput, buildFinishPanelModel, formatActualYield, productionFinishErrorDetail } from '../lib/produce-running'
import StockOperationsView from './StockOperationsView.vue'

defineProps({
  viewParams: { type: Object, default: () => ({}) },
})

const loading = ref(false)
const busyId = ref(0)
const error = ref('')
const message = ref('')
const rows = ref([])
const finishedWarehouses = ref([{ code: 'finished_goods', name: '成品仓' }])
const finishInputs = reactive({})
const stockDrawerOpen = ref(false)
const completion = reactive({
  open: false,
  row: null,
  partial: false,
  note: '',
})
const finishErrorDetail = computed(() => productionFinishErrorDetail(error.value))
const isWipInsufficientError = computed(() => finishErrorDetail.value.actionKey === 'stockOperations')
const activeCompletionInput = computed(() => completion.row ? (finishInputs[completion.row.id] || buildFinishInput(completion.row)) : buildFinishInput({}))

function rowSignature(row) {
  const outputSignature = (row.outputs || []).map((output) => `${output.spec_g}:${output.plan_units}:${output.plan_loose_g}`).join('|')
  return [row.input_g, row.plan_units, row.plan_loose_g, outputSignature].map((v) => String(v || 0)).join(':')
}

function ensureInputs() {
  const activeIds = new Set(rows.value.map((row) => String(row.id)))
  for (const id of Object.keys(finishInputs)) {
    if (!activeIds.has(String(id))) delete finishInputs[id]
  }
  for (const row of rows.value) {
    const signature = rowSignature(row)
    if (!finishInputs[row.id] || finishInputs[row.id].source_signature !== signature) {
      finishInputs[row.id] = {
        ...buildFinishInput(row),
        source_signature: signature,
      }
    }
  }
}

function openStockDrawer() {
  stockDrawerOpen.value = true
}

function openErrorAction() {
  if (finishErrorDetail.value.actionKey === 'stockOperations') {
    openStockDrawer()
    return
  }
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: finishErrorDetail.value.actionKey } }))
}

function openCompletionPanel(row) {
  completion.open = true
  completion.row = row
  completion.partial = false
  completion.note = ''
  if (!finishInputs[row.id]) finishInputs[row.id] = buildFinishInput(row)
}

async function loadWarehouses() {
  try {
    const data = await apiGet('/api/stock/warehouses')
    const rows = (data.rows || []).filter((row) => row.kind === 'finished')
    if (rows.length) finishedWarehouses.value = rows
  } catch {
    finishedWarehouses.value = [{ code: 'finished_goods', name: '成品仓' }]
  }
}

function applyUrlState() {
  const params = new URL(window.location.href).searchParams
  if (params.get('ok') === '1') message.value = '操作已完成'
  if (params.get('err')) error.value = params.get('err')
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchRunningProduction()
    rows.value = data.rows || []
    ensureInputs()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function submitCompletionPanel() {
  const row = completion.row
  if (!row) return
  busyId.value = row.id
  error.value = ''
  message.value = ''
  try {
    const input = finishInputs[row.id] || {}
    input.partial = !!completion.partial
    const panel = buildFinishPanelModel(row, input, input.partial ? 'partial' : 'complete')
    await finishRunningProduction(panel.payload)
    completion.open = false
    message.value = input.partial ? '已记录部分完工' : '生产已完成'
    await load()
  } catch (err) {
    const errMessage = err.message || '完成失败'
    error.value = errMessage
  } finally {
    busyId.value = 0
  }
}

async function cancel(row) {
  busyId.value = row.id
  error.value = ''
  message.value = ''
  try {
    await cancelRunningProduction(row.id)
    message.value = '生产已取消'
    await load()
  } catch (err) {
    error.value = err.message || '取消失败'
  } finally {
    busyId.value = 0
  }
}

onMounted(() => {
  applyUrlState()
  loadWarehouses()
  load()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
h2 { margin: 0 0 4px; font-size: 20px; }
p { margin: 0; color: #666; }
button {
  height: 34px;
  border-radius: 6px;
  border: 1px solid #222;
  padding: 0 10px;
  font: inherit;
  cursor: pointer;
  white-space: nowrap;
}
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #202020; color: #fff; }
.secondary { background: #fff; color: #202020; }
.danger { background: #fff3f3; color: #8a1f1f; border-color: #d99; }
.notice, .error {
  border-radius: 6px;
  padding: 9px 10px;
  margin-bottom: 12px;
}
.notice { border: 1px solid #b7d9b7; background: #f0fff0; color: #246024; }
.error { display: flex; justify-content: space-between; align-items: center; gap: 10px; border: 1px solid #e0b0b0; background: #fff3f3; color: #8a1f1f; }
.small-button { height: 32px; padding: 0 10px; }
.table-wrap { overflow: auto; border: 1px solid #e6e0d8; border-radius: 8px; }
table { width: 100%; min-width: 860px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
input {
  width: 72px;
  height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 6px;
  padding: 6px 8px;
  font: inherit;
}
.input-g { width: 104px; }
.output-grid {
  display: grid;
  grid-template-columns: 72px 82px;
  gap: 6px;
}
.output-grid span {
  display: block;
  margin-bottom: 3px;
  color: #666;
  font-size: 12px;
}
.finish-grid {
  display: grid;
  grid-template-columns: 132px 150px 58px;
  gap: 6px;
  align-items: center;
}
.partial-check {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
  color: #333;
}
.partial-check input {
  width: 16px;
  height: 16px;
}
.partial-note { grid-column: 1 / -1; color: #666; font-size: 12px; line-height: 1.35; }
.plan-summary { display: flex; flex-wrap: wrap; gap: 6px; }
.plan-summary span {
  border: 1px solid #e6e0d8;
  border-radius: 999px;
  padding: 3px 7px;
  background: #fbfaf8;
  color: #555;
  font-size: 12px;
}
.completion-panel {
  border: 1px solid #d5d0c8;
  border-radius: 8px;
  background: #fff;
  padding: 14px;
  display: grid;
  gap: 12px;
}
.completion-head,
.completion-actions {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.completion-head h3 { margin: 0 0 4px; font-size: 18px; }
.completion-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  align-items: end;
}
.multi-output-grid { display: contents; }
.completion-grid label { display: grid; gap: 5px; color: #555; font-size: 13px; }
.completion-grid label span,
.completion-yield span { color: #666; font-size: 12px; }
.completion-grid input,
.completion-grid select { width: 100%; }
.completion-check { align-self: center; }
.completion-note { grid-column: span 2; }
.completion-yield {
  min-height: 38px;
  border: 1px solid #e6e0d8;
  border-radius: 8px;
  padding: 6px 8px;
  display: grid;
  gap: 2px;
  background: #fbfaf8;
}
.muted { color: #666; }
.empty { color: #666; text-align: center; }
.drawer-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  justify-content: flex-end;
  background: rgba(0, 0, 0, .24);
}
.stock-drawer {
  width: min(980px, calc(100vw - 28px));
  height: 100%;
  overflow: auto;
  background: #f8fafc;
  border-left: 1px solid #d1d5db;
  padding: 16px;
  box-shadow: -12px 0 28px rgba(15, 23, 42, .14);
}
.drawer-head {
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin: -16px -16px 14px;
  padding: 16px;
  border-bottom: 1px solid #e5e7eb;
  background: #f8fafc;
}
.drawer-head h3 { margin: 0 0 4px; font-size: 18px; }
.drawer-head p { margin: 0; color: #666; font-size: 13px; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .toolbar { align-items: flex-start; }
  table { min-width: 760px; }
  .error { align-items: flex-start; flex-direction: column; }
  .completion-head,
  .completion-actions { flex-direction: column; }
  .completion-grid { grid-template-columns: 1fr; }
  .completion-note { grid-column: auto; }
  .stock-drawer { width: 100vw; }
}
</style>
