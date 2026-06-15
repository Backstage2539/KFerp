<template>
  <div class="page">
    <ProductionTopNav active-key="qualityInspections" />

    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>生产质检</h2>
          <p>原料、生产工单和成品批次的检查结果</p>
        </div>
        <div class="head-actions">
          <button class="secondary" type="button" @click="openTargetDrawer(form.scope)">{{ qualityTargetActionLabel(form.scope) }}</button>
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="message" class="notice">{{ message }}</div>
      <div v-if="error" class="error">{{ error }}</div>
    </section>

    <div class="workspace">
      <aside class="panel type-panel">
        <div class="panel-title">质检类型</div>
        <button
          v-for="tab in qualityTargetTabs"
          :key="tab.scope"
          class="type-button"
          :class="{ active: form.scope === tab.scope }"
          type="button"
          @click="switchScope(tab.scope)">
          <strong>{{ tab.label }}</strong>
          <small>{{ typeHint(tab.scope) }}</small>
        </button>
      </aside>

      <section class="panel form-panel">
        <div class="section-head">
          <div>
            <div class="panel-title">新增质检记录</div>
            <p>{{ scopeLabel(form.scope) }} · {{ form.reference_no || '未选择对象' }}</p>
          </div>
          <button class="secondary" type="button" @click="openTargetDrawer(form.scope)">{{ qualityTargetActionLabel(form.scope) }}</button>
        </div>

        <div v-if="selectedTarget" class="target-summary">
          <div><span>对象</span><strong>{{ form.reference_no }}</strong></div>
          <div><span>名称</span><strong>{{ form.item_name || '-' }}</strong></div>
          <div><span>来源</span><strong>{{ scopeLabel(form.scope) }}</strong></div>
        </div>

        <div class="form-grid">
          <label>
            <span>质检范围</span>
            <select v-model="form.scope" @change="switchScope(form.scope)">
              <option value="work_order">生产工单</option>
              <option value="raw_material">原料</option>
              <option value="finished_batch">成品批次</option>
            </select>
          </label>
          <label>
            <span>单据/批次号</span>
            <input v-model.trim="form.reference_no" placeholder="WO-0000000020 / MB-0000000007 / FP-0000000042" />
          </label>
          <label>
            <span>物料/产品</span>
            <input v-model.trim="form.item_name" placeholder="孟连水洗5T批次" />
          </label>
          <label>
            <span>检查结果</span>
            <select v-model="form.result">
              <option value="pass">通过</option>
              <option value="hold">待处理</option>
              <option value="reject">不合格</option>
            </select>
          </label>
        </div>
        <div class="note-grid">
          <label v-if="form.scope === 'raw_material'">
            <span>工厂风味描述</span>
            <input v-model.trim="form.factory_flavor_description" placeholder="杯测后工厂确认的风味" />
          </label>
          <label v-if="form.scope === 'raw_material'">
            <span>水分</span>
            <input v-model.trim="form.moisture" placeholder="10.8%" />
          </label>
          <label v-if="form.scope === 'raw_material'">
            <span>密度</span>
            <input v-model.trim="form.density" placeholder="780g/L" />
          </label>
          <label>
            <span>指标 JSON</span>
            <textarea v-model.trim="form.metrics_json" rows="3" placeholder='{"色值":"正常"}'></textarea>
          </label>
          <label>
            <span>备注</span>
            <textarea v-model.trim="form.note" rows="3" placeholder="首锅杯测通过"></textarea>
          </label>
        </div>
        <div class="actions">
          <button class="primary" type="button" @click="save" :disabled="saving">保存质检</button>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="section-head">
        <div class="panel-title">质检记录</div>
        <div class="filters">
          <select v-model="filters.scope" @change="load">
            <option value="">全部范围</option>
            <option value="raw_material">原料</option>
            <option value="work_order">生产工单</option>
            <option value="finished_batch">成品批次</option>
          </select>
          <select v-model="filters.result" @change="load">
            <option value="">全部结果</option>
            <option value="pass">通过</option>
            <option value="hold">待处理</option>
            <option value="reject">不合格</option>
          </select>
        </div>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>时间</th>
              <th>范围</th>
              <th>单据/批次号</th>
              <th>物料/产品</th>
              <th>结果</th>
              <th>指标</th>
              <th>备注</th>
              <th>操作人</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.created_at }}</td>
              <td>{{ scopeLabel(row.scope) }}</td>
              <td>{{ row.reference_no }}</td>
              <td>{{ row.item_name }}</td>
              <td><span class="quality-pill" :class="resultClass(row.result)">{{ resultLabel(row.result) }}</span></td>
              <td class="mono">{{ row.metrics_json }}</td>
              <td>{{ row.note }}</td>
              <td>{{ row.operator }}</td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="8" class="empty">暂无质检记录</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <div v-if="targetDrawerOpen" class="drawer-mask" @click.self="targetDrawerOpen = false">
      <aside class="drawer wide">
        <div class="drawer-head">
          <h3>{{ qualityTargetDrawerTitle(activeTargetScope) }}</h3>
          <button class="secondary" type="button" @click="targetDrawerOpen = false">关闭</button>
        </div>

        <div class="drawer-search">
          <label>
            <span>搜索</span>
            <input v-model.trim="targetQ" :placeholder="qualityTargetSearchPlaceholder(activeTargetScope)" @keyup.enter="loadTargets" />
          </label>
          <button class="primary" type="button" @click="loadTargets" :disabled="targetLoading">查询</button>
        </div>

        <div v-if="targetError" class="error">{{ targetError }}</div>
        <div class="drawer-summary">
          <div><span>类型</span><strong>{{ scopeLabel(activeTargetScope) }}</strong></div>
          <div><span>候选</span><strong>{{ filteredTargets.length }}</strong></div>
          <div><span>已选</span><strong>{{ form.reference_no || '-' }}</strong></div>
        </div>

        <div class="table-wrap drawer-table">
          <table>
            <thead>
              <tr>
                <th>对象</th>
                <th>名称</th>
                <th>状态</th>
                <th>参考</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in filteredTargets" :key="targetKey(row)">
                <td><strong>{{ targetPrimary(row) }}</strong><small>{{ row.order_nos || '' }}</small></td>
                <td>{{ targetName(row) }}</td>
                <td>
                  <span v-if="activeTargetScope === 'work_order'" class="status-pill">{{ workOrderQualityStatusLabel(row.status) }}</span>
                  <span v-else class="quality-pill" :class="qualityClass(qualityTargetStatus(row))">{{ qualityLabel(qualityTargetStatus(row)) }}</span>
                </td>
                <td>{{ targetMeta(row) || '-' }}</td>
                <td><button class="link" type="button" @click="selectTarget(row)">选择</button></td>
              </tr>
              <tr v-if="!filteredTargets.length">
                <td colspan="5" class="empty">暂无候选对象</td>
              </tr>
            </tbody>
          </table>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import {
  filterQualityTargets,
  qualityInspectionErrorMessage,
  qualityTargetActionLabel,
  qualityTargetAPIPath,
  qualityTargetDrawerTitle,
  qualityTargetFromRow,
  qualityTargetMeta,
  qualityTargetName,
  qualityTargetPrimary,
  qualityTargetSearchPlaceholder,
  qualityTargetStatus,
  qualityTargetTabs,
  workOrderQualityStatusLabel,
} from '../lib/quality-inspections'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const rows = ref([])
const targetDrawerOpen = ref(false)
const activeTargetScope = ref('work_order')
const targetRows = ref([])
const targetQ = ref('')
const targetLoading = ref(false)
const targetError = ref('')
const selectedTarget = ref(null)

const filters = reactive({
  scope: '',
  result: '',
})

const form = reactive({
  scope: 'work_order',
  reference_type: 'work_order',
  reference_no: '',
  item_name: '',
  result: 'pass',
  metrics_json: '',
  factory_flavor_description: '',
  moisture: '',
  density: '',
  note: '',
})

const filteredTargets = computed(() => filterQualityTargets(activeTargetScope.value, targetRows.value, targetQ.value))

function scopeLabel(value) {
  return {
    raw_material: '原料',
    work_order: '生产工单',
    finished_batch: '成品批次',
  }[value] || value
}

function typeHint(scope) {
  return {
    work_order: '按生产工单记录首锅、过程或完工检查',
    raw_material: '按原料批次记录入库、复检或冻结检查',
    finished_batch: '按成品批次记录入库后检查',
  }[scope] || ''
}

function resultLabel(value) {
  return {
    pass: '通过',
    hold: '待处理',
    reject: '不合格',
  }[value] || value
}

function qualityLabel(status) {
  return {
    pass: '通过',
    hold: '待处理',
    reject: '不通过',
    unchecked: '未检',
  }[status || 'unchecked'] || '未检'
}

function qualityClass(status) {
  return `quality-${status || 'unchecked'}`
}

function resultClass(result) {
  return qualityClass(result === 'reject' ? 'reject' : result)
}

function switchScope(scope) {
  form.scope = scope
  form.reference_type = scope
  activeTargetScope.value = scope
  selectedTarget.value = null
  targetRows.value = []
  targetError.value = ''
  form.reference_no = ''
  form.item_name = ''
  form.factory_flavor_description = ''
  form.moisture = ''
  form.density = ''
}

function targetPrimary(row) {
  return qualityTargetPrimary(activeTargetScope.value, row)
}

function targetName(row) {
  return qualityTargetName(activeTargetScope.value, row)
}

function targetMeta(row) {
  return qualityTargetMeta(activeTargetScope.value, row)
}

function targetKey(row) {
  return `${activeTargetScope.value}-${targetPrimary(row)}-${row.id || row.batch_id || ''}`
}

function openTargetDrawer(scope) {
  activeTargetScope.value = scope || form.scope
  targetRows.value = []
  targetError.value = ''
  targetDrawerOpen.value = true
  loadTargets()
}

async function loadTargets() {
  targetLoading.value = true
  targetError.value = ''
  try {
    const url = new URL(qualityTargetAPIPath(activeTargetScope.value), window.location.origin)
    if (targetQ.value) url.searchParams.set('q', targetQ.value)
    const data = await apiGet(url)
    targetRows.value = data.rows || []
  } catch (err) {
    targetError.value = err.message || '加载质检对象失败'
  } finally {
    targetLoading.value = false
  }
}

function selectTarget(row) {
  const selected = qualityTargetFromRow(activeTargetScope.value, row)
  Object.assign(form, selected)
  selectedTarget.value = { ...row, scope: activeTargetScope.value }
  targetDrawerOpen.value = false
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams()
    if (filters.scope) params.set('scope', filters.scope)
    if (filters.result) params.set('result', filters.result)
    const qs = params.toString()
    const data = await apiGet(`/api/produce/quality-inspections${qs ? `?${qs}` : ''}`)
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    if (!String(form.scope || '').trim() || !String(form.reference_no || '').trim() || !String(form.result || '').trim()) {
      error.value = qualityInspectionErrorMessage('scope, reference_no and result required')
      return
    }
    await apiSend('/api/produce/quality-inspections', {
      body: {
        scope: form.scope,
        reference_type: form.reference_type || form.scope,
        reference_no: form.reference_no,
        item_name: form.item_name,
        result: form.result,
        metrics_json: form.metrics_json || '{}',
        factory_flavor_description: form.factory_flavor_description,
        moisture: form.moisture,
        density: form.density,
        note: form.note,
      },
    })
    message.value = '质检记录已保存'
    selectedTarget.value = null
    form.reference_no = ''
    form.item_name = ''
    form.metrics_json = ''
    form.factory_flavor_description = ''
    form.moisture = ''
    form.density = ''
    form.note = ''
    await load()
  } catch (err) {
    error.value = qualityInspectionErrorMessage(err.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
*{box-sizing:border-box}.page{padding:16px;color:#171717;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head,.section-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.panel-head h2{margin:0 0 4px;font-size:18px}.panel-head p,.section-head p{margin:0;color:#6b7280;font-size:13px}.head-actions{display:flex;gap:8px;align-items:center}.panel-title{font-weight:700;margin-bottom:10px}.workspace{display:grid;grid-template-columns:260px minmax(0,1fr);gap:16px}.type-panel{align-self:start}.type-button{width:100%;text-align:left;border:1px solid #e5e7eb;background:#fff;border-radius:8px;padding:9px;margin-bottom:8px}.type-button strong{display:block}.type-button small{display:block;color:#6b7280;margin-top:3px;line-height:1.35}.type-button.active{border-color:#111;background:#111;color:#fff}.type-button.active small{color:#e5e7eb}.target-summary,.drawer-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin:12px 0}.target-summary div,.drawer-summary div{border:1px solid #e5e7eb;border-radius:8px;padding:10px}.target-summary span,.drawer-summary span{display:block;color:#6b7280;font-size:12px;margin-bottom:4px}.target-summary strong,.drawer-summary strong{font-size:16px}.form-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.note-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-top:10px}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}input,select,textarea,button{font:inherit}input,select,textarea{width:100%;border:1px solid #d1d5db;border-radius:6px;padding:8px 9px;background:#fff}textarea{resize:vertical}button{min-height:36px;border-radius:6px;cursor:pointer;white-space:nowrap}button:disabled{cursor:not-allowed;opacity:.55}.primary{border:1px solid #111;background:#111;color:#fff;padding:8px 12px}.secondary{border:1px solid #9ca3af;background:#fff;color:#111;padding:8px 12px}.link{border:0;background:transparent;color:#111;text-decoration:underline;padding:0;min-height:0}.actions,.filters{display:flex;gap:8px;flex-wrap:wrap}.filters select{max-width:180px}.notice,.error{border-radius:8px;padding:9px 10px}.notice{border:1px solid #b7d9b7;background:#f0fff0;color:#246024}.error{border:1px solid #ffb9b9;background:#ffecec;color:#8a1f1f}.table-wrap{overflow:auto}table{width:100%;min-width:1080px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb;position:sticky;top:0}td small{display:block;color:#6b7280;margin-top:3px}.mono{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px}.empty{color:#666;text-align:center}.quality-pill,.status-pill{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb;white-space:nowrap}.quality-pass{border-color:#bbf7d0;background:#f0fdf4;color:#166534}.quality-hold{border-color:#fde68a;background:#fffbeb;color:#92400e}.quality-reject{border-color:#fecaca;background:#fef2f2;color:#991b1b}.quality-unchecked{border-color:#d1d5db;background:#f9fafb;color:#4b5563}.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.22);display:flex;justify-content:flex-end;z-index:40}.drawer{width:min(520px,100%);height:100%;background:#fff;border-left:1px solid #d1d5db;padding:16px;overflow:auto}.drawer.wide{width:min(820px,100%)}.drawer-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}.drawer h3{margin:0;font-size:18px}.drawer-search{display:grid;grid-template-columns:1fr 84px;gap:10px;align-items:end;margin-bottom:12px}.drawer-table table{min-width:720px}.form-panel{min-width:0}
@media (max-width:900px){.page{padding:12px}.panel-head,.section-head{display:grid}.head-actions,.filters{width:100%}.workspace,.form-grid,.note-grid,.target-summary,.drawer-summary,.drawer-search{grid-template-columns:1fr}}
</style>
