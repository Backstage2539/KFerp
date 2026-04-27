<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>物料档案/库存</h2>
          <p>基础档案字段锁定，变更编码、名称、价格等信息时复制为新物料。</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input v-model.trim="q" placeholder="名称/编码/批次号" @keyup.enter="load" />
        </label>
        <button class="primary" type="button" @click="load" :disabled="loading">查询</button>
      </div>
    </section>

    <div class="materials-layout">
      <section class="panel material-list-panel">
        <div class="panel-title">物料列表</div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>物料类别</th>
                <th>物料名称</th>
                <th>批次号</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in rows"
                :key="row.id"
                :class="{ active: selected?.id === row.id }"
                @click="selectMaterial(row)">
                <td><span class="pill">{{ kindLabel(row.kind) }}</span></td>
                <td>
                  <strong>{{ row.name }}</strong>
                  <small>{{ profileSummary(row) }}</small>
                </td>
                <td>{{ row.batch_no || '-' }}</td>
              </tr>
              <tr v-if="!rows.length">
                <td colspan="3" class="muted">暂无物料</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel material-detail-panel">
        <div class="detail-head">
          <div>
            <div class="panel-title">物料详情</div>
            <p v-if="selected">{{ draftMode ? '复制新物料' : '查看与维护库存/属性' }}</p>
            <p v-if="selected">保存、复制和废弃会记录到操作日志。</p>
          </div>
          <div class="actions" v-if="selected">
            <button v-if="!draftMode" class="secondary" type="button" @click="openStockBackfill" :disabled="loading">库存补录</button>
            <button class="secondary" type="button" @click="copySelectedMaterial">复制新物料</button>
            <button v-if="!draftMode" class="danger" type="button" @click="deprecateSelectedMaterial" :disabled="loading">废弃物料</button>
          </div>
        </div>

        <div v-if="!selected" class="empty muted">请选择左侧物料</div>

        <form v-else class="detail-form" @submit.prevent="saveMaterial">
          <section class="form-section">
            <div class="section-title">基础信息</div>
            <div class="form-grid">
              <label><span>编码</span><input v-model.trim="draft.code" :disabled="!draftMode" /></label>
              <label><span>名称</span><input v-model.trim="draft.name" :disabled="!draftMode" /></label>
              <label>
                <span>类型</span>
                <select v-model="draft.kind" :disabled="!draftMode">
                  <option value="bean">生豆</option>
                  <option value="pack">包材</option>
                  <option value="other">其他</option>
                </select>
              </label>
              <label>
                <span>单位</span>
                <select v-model="draft.unit" :disabled="!draftMode">
                  <option value="g">g</option>
                  <option value="kg">kg</option>
                  <option value="unit">个/张</option>
                  <option value="个">个</option>
                </select>
              </label>
              <label><span>批次号</span><input v-model.trim="draft.batch_no" :disabled="!draftMode" /></label>
              <label><span>进货价</span><input type="number" min="0" step="0.01" v-model.number="draft.purchase_price" :disabled="!draftMode" /></label>
              <label><span>销售价</span><input type="number" min="0" step="0.01" v-model.number="draft.sale_price" :disabled="!draftMode" /></label>
              <label><span>更新时间</span><input :value="draft.updated_at || '-'" disabled /></label>
            </div>
          </section>

          <section class="form-section">
            <div class="section-title">库存</div>
            <div class="form-grid">
              <label><span>库存(g)</span><input type="number" :value="draft.onhand_g" disabled /></label>
              <label><span>库存(个)</span><input type="number" :value="draft.onhand_units" disabled /></label>
              <label><span>警戒线(g)</span><input type="number" min="0" step="1" v-model.number="draft.min_level_g" /></label>
              <label><span>警戒线(个)</span><input type="number" min="0" step="1" v-model.number="draft.min_level_units" /></label>
            </div>
          </section>

          <section v-if="draft.kind === 'bean'" class="form-section">
            <div class="section-title">咖啡生豆属性</div>
            <div class="form-grid">
              <label><span>产地</span><input v-model.trim="draft.bean_profile.origin" /></label>
              <label><span>处理站</span><input v-model.trim="draft.bean_profile.processing_station" /></label>
              <label><span>品种</span><input v-model.trim="draft.bean_profile.variety" /></label>
              <label><span>处理法</span><input v-model.trim="draft.bean_profile.process_method" /></label>
              <label><span>等级</span><input v-model.trim="draft.bean_profile.grade" /></label>
              <label><span>海拔</span><input v-model.trim="draft.bean_profile.altitude" /></label>
              <label class="wide"><span>风味</span><textarea v-model.trim="draft.bean_profile.flavor" rows="3"></textarea></label>
              <label class="wide"><span>豆单备注</span><input v-model.trim="draft.bean_profile.bean_list_note" /></label>
            </div>
          </section>

          <section v-else-if="draft.kind === 'pack'" class="form-section">
            <div class="section-title">包材属性</div>
            <div class="form-grid">
              <label><span>大小规格</span><input v-model.trim="draft.pack_profile.size_spec" placeholder="例如 227g袋" /></label>
              <label><span>尺寸</span><input v-model.trim="draft.pack_profile.dimensions" placeholder="例如 12x20cm" /></label>
              <label><span>材质</span><input v-model.trim="draft.pack_profile.material" /></label>
              <label><span>容量</span><input v-model.trim="draft.pack_profile.capacity" /></label>
              <label><span>颜色</span><input v-model.trim="draft.pack_profile.color" /></label>
              <label class="wide"><span>备注</span><input v-model.trim="draft.pack_profile.note" /></label>
            </div>
          </section>

          <section v-else class="form-section">
            <div class="section-title">物料属性</div>
            <div class="empty muted">当前类型暂无专属属性。</div>
          </section>

          <div class="form-actions">
            <button class="primary" type="submit" :disabled="loading">{{ draftMode ? '保存新物料' : '保存警戒线/属性' }}</button>
          </div>
        </form>
      </section>
    </div>

    <div v-if="stockBackfill.open" class="modal-mask" @click.self="closeStockBackfill">
      <section class="modal-panel">
        <div class="modal-head">
          <div>
            <h3>库存补录</h3>
            <p>{{ selected?.name || '-' }} · {{ selected?.batch_no || '-' }}</p>
          </div>
          <button class="secondary" type="button" @click="closeStockBackfill">关闭</button>
        </div>
        <form class="detail-form" @submit.prevent="submitStockBackfill">
          <div class="form-grid">
            <label><span>当前库存(g)</span><input type="number" :value="selected?.onhand_g || 0" disabled /></label>
            <label><span>当前库存(个)</span><input type="number" :value="selected?.onhand_units || 0" disabled /></label>
            <label><span>目标库存(g)</span><input type="number" min="0" step="1" v-model.number="stockBackfill.target_g" /></label>
            <label><span>目标库存(个)</span><input type="number" min="0" step="1" v-model.number="stockBackfill.target_units" /></label>
            <label class="wide"><span>补录说明</span><textarea v-model.trim="stockBackfill.reason" rows="3" required></textarea></label>
          </div>
          <div class="form-actions">
            <button class="secondary" type="button" @click="closeStockBackfill">取消</button>
            <button class="primary" type="submit" :disabled="loading">提交补录</button>
          </div>
        </form>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'

const rows = ref([])
const q = ref('')
const loading = ref(false)
const error = ref('')
const ok = ref('')
const selected = ref(null)
const draft = ref(null)
const draftMode = ref(false)
const stockBackfill = ref({
  open: false,
  target_g: 0,
  target_units: 0,
  reason: '',
})

const selectedID = computed(() => selected.value?.id || 0)

function emptyBeanProfile() {
  return {
    origin: '',
    processing_station: '',
    variety: '',
    process_method: '',
    grade: '',
    altitude: '',
    flavor: '',
    bean_list_note: '',
  }
}

function emptyPackProfile() {
  return {
    size_spec: '',
    dimensions: '',
    material: '',
    capacity: '',
    color: '',
    note: '',
  }
}

function cloneBeanProfile(profile = {}) {
  return { ...emptyBeanProfile(), ...profile }
}

function clonePackProfile(profile = {}) {
  return { ...emptyPackProfile(), ...profile }
}

function normalizeRow(row) {
  const beanProfile = row.BeanProfile ?? row.bean_profile ?? {}
  const packProfile = row.PackProfile ?? row.pack_profile ?? {}
  return {
    id: Number(row.ID ?? row.id ?? 0),
    code: row.Code ?? row.code ?? '',
    name: row.Name ?? row.name ?? '',
    kind: row.Kind ?? row.kind ?? 'other',
    unit: row.Unit ?? row.unit ?? 'g',
    batch_no: row.BatchNo ?? row.batch_no ?? '',
    purchase_price: Number(row.PurchasePrice ?? row.purchase_price ?? 0),
    sale_price: Number(row.SalePrice ?? row.sale_price ?? 0),
    onhand_g: Number(row.OnhandG ?? row.onhand_g ?? 0),
    onhand_units: Number(row.OnhandUnits ?? row.onhand_units ?? 0),
    min_level_g: Number(row.MinLevelG ?? row.min_level_g ?? 0),
    min_level_units: Number(row.MinLevelUnits ?? row.min_level_units ?? 0),
    bean_profile: cloneBeanProfile({
      origin: beanProfile.Origin ?? beanProfile.origin ?? '',
      processing_station: beanProfile.ProcessingStation ?? beanProfile.processing_station ?? '',
      variety: beanProfile.Variety ?? beanProfile.variety ?? '',
      process_method: beanProfile.ProcessMethod ?? beanProfile.process_method ?? '',
      grade: beanProfile.Grade ?? beanProfile.grade ?? '',
      altitude: beanProfile.Altitude ?? beanProfile.altitude ?? '',
      flavor: beanProfile.Flavor ?? beanProfile.flavor ?? '',
      bean_list_note: beanProfile.BeanListNote ?? beanProfile.bean_list_note ?? '',
    }),
    pack_profile: clonePackProfile({
      size_spec: packProfile.SizeSpec ?? packProfile.size_spec ?? '',
      dimensions: packProfile.Dimensions ?? packProfile.dimensions ?? '',
      material: packProfile.Material ?? packProfile.material ?? '',
      capacity: packProfile.Capacity ?? packProfile.capacity ?? '',
      color: packProfile.Color ?? packProfile.color ?? '',
      note: packProfile.Note ?? packProfile.note ?? '',
    }),
    updated_at: row.UpdatedAt ?? row.updated_at ?? '',
    deprecated_at: row.DeprecatedAt ?? row.deprecated_at ?? '',
  }
}

function cloneMaterial(row) {
  return normalizeRow(JSON.parse(JSON.stringify(row)))
}

function kindLabel(kind) {
  return ({ bean: '生豆', pack: '包材', other: '其他' })[kind] || kind || '其他'
}

function profileSummary(row) {
  if (row.kind === 'bean') {
    const p = row.bean_profile || {}
    return [p.origin, p.process_method, p.flavor].filter(Boolean).join(' · ') || '未设置咖啡生豆属性'
  }
  if (row.kind === 'pack') {
    const p = row.pack_profile || {}
    return [p.size_spec, p.dimensions, p.material].filter(Boolean).join(' · ') || '未设置包材属性'
  }
  return '无专属属性'
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = new URL('/api/materials', window.location.origin)
    url.searchParams.set('limit', '500')
    if (q.value) url.searchParams.set('q', q.value)
    const res = await fetch(url)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    rows.value = (data.rows || []).map(normalizeRow)
    if (selectedID.value) {
      const next = rows.value.find((item) => item.id === selectedID.value)
      if (next) selectMaterial(next, { quiet: true })
      else clearSelection()
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function selectMaterial(row, options = {}) {
  selected.value = row
  draft.value = cloneMaterial(row)
  draftMode.value = false
  closeStockBackfill()
  if (!options.quiet) {
    error.value = ''
    ok.value = ''
  }
}

function clearSelection() {
  selected.value = null
  draft.value = null
  draftMode.value = false
  closeStockBackfill()
}

function copySelectedMaterial() {
  if (!selected.value) return
  const next = cloneMaterial(selected.value)
  next.id = 0
  next.code = `${next.code}-copy`
  next.name = `${next.name} 副本`
  next.onhand_g = 0
  next.onhand_units = 0
  next.updated_at = ''
  next.deprecated_at = ''
  selected.value = next
  draft.value = next
  draftMode.value = true
  ok.value = ''
  error.value = ''
}

function payloadFromDraft() {
  const sourceStock = draftMode.value ? { onhand_g: 0, onhand_units: 0 } : (selected.value || draft.value)
  return {
    code: draft.value.code,
    name: draft.value.name,
    kind: draft.value.kind,
    unit: draft.value.unit,
    batch_no: draft.value.batch_no,
    purchase_price: Number(draft.value.purchase_price || 0),
    sale_price: Number(draft.value.sale_price || 0),
    onhand_g: Number(sourceStock.onhand_g || 0),
    onhand_units: Number(sourceStock.onhand_units || 0),
    min_level_g: Number(draft.value.min_level_g || 0),
    min_level_units: Number(draft.value.min_level_units || 0),
    bean_profile: draft.value.kind === 'bean' ? draft.value.bean_profile : null,
    pack_profile: draft.value.kind === 'pack' ? draft.value.pack_profile : null,
  }
}

async function saveMaterial() {
  if (!draft.value) return
  await mutate(async () => {
    const url = draftMode.value ? '/api/materials' : `/api/materials/${draft.value.id}`
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payloadFromDraft()),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    const row = normalizeRow(data)
    ok.value = draftMode.value ? '已保存新物料' : '已保存库存/属性'
    draftMode.value = false
    await load()
    const next = rows.value.find((item) => item.id === row.id) || row
    selectMaterial(next, { quiet: true })
  })
}

function openStockBackfill() {
  if (!selected.value || draftMode.value) return
  stockBackfill.value = {
    open: true,
    target_g: Number(selected.value.onhand_g || 0),
    target_units: Number(selected.value.onhand_units || 0),
    reason: '',
  }
  error.value = ''
  ok.value = ''
}

function closeStockBackfill() {
  if (!stockBackfill.value.open) return
  stockBackfill.value.open = false
}

async function submitStockBackfill() {
  if (!selected.value || draftMode.value) return
  if (!stockBackfill.value.reason) {
    error.value = '补录说明必填'
    return
  }
  await mutate(async () => {
    const res = await fetch('/api/stock/adjustments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        item_type: 'material',
        item_id: selected.value.id,
        spec_g: 0,
        warehouse: '',
        target_g: Number(stockBackfill.value.target_g || 0),
        target_units: Number(stockBackfill.value.target_units || 0),
        reason: stockBackfill.value.reason,
      }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '库存补录失败')
    stockBackfill.value.open = false
    await load()
    const next = rows.value.find((item) => item.id === selectedID.value)
    if (next) selectMaterial(next, { quiet: true })
    ok.value = `库存补录已提交：#${data.adjustment_id || '-'}`
  })
}

async function deprecateSelectedMaterial() {
  if (!selected.value || draftMode.value) return
  if (!window.confirm(`废弃物料：${selected.value.name}？`)) return
  await mutate(async () => {
    const res = await fetch(`/api/materials/${selected.value.id}/deprecate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '废弃失败')
    ok.value = `已废弃：${data.name || selected.value.name}`
    clearSelection()
    await load()
  })
}

async function mutate(action) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await action()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  q.value = new URL(window.location.href).searchParams.get('q') || ''
  load()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters, .detail-head, .actions, .form-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
.panel-head h2 { margin: 0; font-size: 20px; }
.panel-head p, .detail-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
.panel-title { font-size: 16px; font-weight: 700; }
.materials-layout { display: grid; grid-template-columns: minmax(360px, .85fr) minmax(520px, 1.15fr); gap: 14px; align-items: start; }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; min-width: 520px; }
th, td { border-bottom: 1px solid #eee8df; padding: 10px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.material-list-panel tbody tr { cursor: pointer; }
tbody tr.active { background: #f3f7fb; }
td strong, td small { display: block; }
td small { color: #666; margin-top: 4px; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pill { display: inline-flex; height: 24px; align-items: center; border: 1px solid #d8d0c7; border-radius: 999px; padding: 0 8px; background: #fbfaf8; font-size: 12px; }
.detail-head { justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
.detail-form { display: grid; gap: 12px; }
.form-section { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; }
.section-title { font-size: 14px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; min-height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input:disabled, select:disabled { background: #f6f4f1; color: #555; }
textarea { resize: vertical; line-height: 1.45; }
.wide { grid-column: span 2; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.danger { border-color: #b23b3b; background: #fff; color: #9b2020; }
.form-actions { justify-content: flex-end; }
.muted { color: #666; text-align: center; }
.empty { padding: 22px; border: 1px dashed #d8d0c7; border-radius: 8px; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.modal-mask { position: fixed; inset: 0; z-index: 50; display: grid; place-items: center; padding: 18px; background: rgba(0,0,0,.28); }
.modal-panel { width: min(640px, 100%); max-height: calc(100vh - 36px); overflow: auto; border-radius: 8px; background: #fff; border: 1px solid #d8d0c7; padding: 16px; box-shadow: 0 18px 50px rgba(0,0,0,.18); }
.modal-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 14px; }
.modal-head h3 { margin: 0; font-size: 18px; }
.modal-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
@media (max-width: 1100px) { .materials-layout { grid-template-columns: 1fr; } }
@media (max-width: 760px) { .page { padding: 12px; } .form-grid { grid-template-columns: 1fr; } .wide { grid-column: span 1; } }
</style>
