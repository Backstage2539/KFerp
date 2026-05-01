<template>
  <div class="page">
    <section class="toolbar">
      <div>
        <h2>生产中</h2>
        <p>{{ rows.length }} 个批次项目正在生产</p>
      </div>
      <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
    </section>

    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="error" class="error">{{ error }}</div>

    <section class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>开始时间</th>
            <th>批次</th>
            <th>产品</th>
            <th>规格(g)</th>
            <th>订单号</th>
            <th>需求(g)</th>
            <th>投料(g)</th>
            <th>BOM出品率</th>
            <th>成品数</th>
            <th>实际出品率</th>
            <th>操作人</th>
            <th>完成生产</th>
            <th>取消</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.started_at }}</td>
            <td>{{ row.batch_id }}</td>
            <td>{{ row.product_name }}</td>
            <td>{{ row.spec_g }}</td>
            <td class="muted">{{ row.order_nos }}</td>
            <td>{{ row.need_g }}</td>
            <td>
              <input
                v-model.number="finishInputs[row.id].consumed_input_g"
                class="input-g"
                min="0"
                type="number"
                aria-label="投料(g)"
                @input="markRowEdited(row)"
              />
            </td>
            <td>{{ percent(row.bom_yield_rate) }}</td>
            <td>
              <div class="output-grid">
                <label>
                  <span>件</span>
                  <input
                    v-model.number="finishInputs[row.id].finished_units"
                    min="0"
                    type="number"
                    aria-label="完成件数"
                    @input="markRowEdited(row)"
                  />
                </label>
                <label>
                  <span>余g</span>
                  <input
                    v-model.number="finishInputs[row.id].finished_loose_g"
                    min="0"
                    type="number"
                    aria-label="散装余量"
                    @input="markRowEdited(row)"
                  />
                </label>
              </div>
            </td>
            <td><strong>{{ formatActualYield(row, finishInputs[row.id]) }}</strong></td>
            <td>{{ row.started_by }}</td>
            <td>
              <div class="finish-grid">
                <select v-model="finishInputs[row.id].warehouse" aria-label="成品入库仓">
                  <option v-for="wh in finishedWarehouses" :key="wh.code" :value="wh.code">{{ wh.name }}</option>
                </select>
                <label class="partial-check">
                  <input v-model="finishInputs[row.id].partial" type="checkbox" />
                  部分完工（保留剩余）
                </label>
                <div v-if="finishInputs[row.id].partial" class="partial-note">本次入库并扣本次投料，剩余继续留在生产中。</div>
                <button class="primary" type="button" @click="finish(row)" :disabled="busyId === row.id">
                  完成
                </button>
              </div>
            </td>
            <td>
              <button class="danger" type="button" @click="cancel(row)" :disabled="busyId === row.id">
                取消生产
              </button>
            </td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="13" class="empty">暂无生产中项目</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet } from '../api/client'
import { cancelRunningProduction, fetchRunningProduction, finishRunningProduction } from '../api/production.js'
import { buildFinishInput, buildFinishPayload, formatActualYield, markYieldDirty } from '../lib/produce-running'

const loading = ref(false)
const busyId = ref(0)
const error = ref('')
const message = ref('')
const rows = ref([])
const finishedWarehouses = ref([{ code: 'finished_goods', name: '成品仓' }])
const finishInputs = reactive({})

function percent(v) {
  return `${(Number(v || 0) * 100).toFixed(2)}%`
}

function rowSignature(row) {
  return [row.input_g, row.plan_units, row.plan_loose_g, row.bom_yield_rate].map((v) => String(v || 0)).join(':')
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

function markRowEdited(row) {
  markYieldDirty(finishInputs[row.id])
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

async function finish(row) {
  busyId.value = row.id
  error.value = ''
  message.value = ''
  try {
    const input = finishInputs[row.id] || {}
    await finishRunningProduction(buildFinishPayload(row, input))
    message.value = input.partial ? '已记录部分完工' : '生产已完成'
    await load()
  } catch (err) {
    error.value = err.message || '完成失败'
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
.error { border: 1px solid #e0b0b0; background: #fff3f3; color: #8a1f1f; }
.table-wrap { overflow: auto; border: 1px solid #e6e0d8; border-radius: 8px; }
table { width: 100%; min-width: 1380px; border-collapse: collapse; }
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
.muted { color: #666; }
.empty { color: #666; text-align: center; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .toolbar { align-items: flex-start; }
  table { min-width: 1220px; }
}
</style>
