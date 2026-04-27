<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>物料档案/库存</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存，操作日志已记录</div>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input v-model.trim="q" placeholder="名称/编码" @keyup.enter="load" />
        </label>
        <button class="primary" type="button" @click="load" :disabled="loading">查询</button>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">物料列表</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>编码</th>
              <th>名称</th>
              <th>类型</th>
              <th>单位</th>
              <th>批次号</th>
              <th>进货价</th>
              <th>销售价</th>
              <th>咖啡豆信息</th>
              <th>库存(g)</th>
              <th>库存(个)</th>
              <th>警戒线(g)</th>
              <th>警戒线(个)</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td><input v-model.trim="row.code" /></td>
              <td><input v-model.trim="row.name" /></td>
              <td>
                <select v-model="row.kind">
                  <option value="bean">生豆</option>
                  <option value="pack">包材</option>
                  <option value="other">其他</option>
                </select>
              </td>
              <td>
                <select v-model="row.unit">
                  <option value="g">g</option>
                  <option value="kg">kg</option>
                  <option value="unit">个/张</option>
                  <option value="个">个</option>
                </select>
              </td>
              <td><input v-model.trim="row.batch_no" /></td>
              <td><input type="number" min="0" step="0.01" v-model.number="row.purchase_price" /></td>
              <td><input type="number" min="0" step="0.01" v-model.number="row.sale_price" /></td>
              <td class="profile-cell">
                <div v-if="row.kind === 'bean'" class="profile-summary">
                  <div>
                    <strong>{{ profileTitle(row) }}</strong>
                    <span>{{ profileSubtitle(row) }}</span>
                  </div>
                  <button class="secondary compact" type="button" @click="openBeanProfileDialog(row)">设置</button>
                </div>
                <span v-else class="muted">非咖啡豆物料</span>
              </td>
              <td><input type="number" min="0" step="1" v-model.number="row.onhand_g" /></td>
              <td><input type="number" min="0" step="1" v-model.number="row.onhand_units" /></td>
              <td><input type="number" min="0" step="1" v-model.number="row.min_level_g" /></td>
              <td><input type="number" min="0" step="1" v-model.number="row.min_level_units" /></td>
              <td class="muted">{{ row.updated_at }}</td>
              <td>
                <button class="secondary" type="button" @click="saveMaterial(row)" :disabled="savingId === row.id">保存</button>
              </td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="14" class="muted">暂无物料</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="muted footer">保存单行会记录到操作日志，可在“操作日志”中按物料查看变更字段。</div>
    </section>

    <div v-if="profileModal.open" class="profile-modal" @click.self="closeBeanProfileDialog">
      <section class="modal-panel">
        <div class="modal-head">
          <div>
            <h3>咖啡豆信息</h3>
            <p>{{ profileModal.row?.name || '' }}</p>
          </div>
          <button class="icon-button" type="button" @click="closeBeanProfileDialog" aria-label="关闭">×</button>
        </div>
        <div class="modal-grid">
          <label><span>产地</span><input v-model.trim="profileModal.draft.origin" /></label>
          <label><span>处理站</span><input v-model.trim="profileModal.draft.processing_station" /></label>
          <label><span>品种</span><input v-model.trim="profileModal.draft.variety" /></label>
          <label><span>处理法</span><input v-model.trim="profileModal.draft.process_method" /></label>
          <label><span>等级</span><input v-model.trim="profileModal.draft.grade" /></label>
          <label><span>海拔</span><input v-model.trim="profileModal.draft.altitude" /></label>
          <label class="wide"><span>风味</span><textarea v-model.trim="profileModal.draft.flavor" rows="3"></textarea></label>
          <label class="wide"><span>豆单备注</span><input v-model.trim="profileModal.draft.bean_list_note" /></label>
        </div>
        <div class="modal-actions">
          <button class="secondary" type="button" @click="closeBeanProfileDialog">取消</button>
          <button class="primary" type="button" :disabled="savingId === profileModal.row?.id" @click="saveBeanProfileDialog">保存咖啡豆信息</button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'

const rows = ref([])
const q = ref('')
const loading = ref(false)
const savingId = ref(null)
const error = ref('')
const ok = ref(false)
const profileModal = ref({
  open: false,
  row: null,
  draft: emptyBeanProfile(),
})

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

function cloneBeanProfile(profile = {}) {
  return {
    ...emptyBeanProfile(),
    origin: profile.origin || '',
    processing_station: profile.processing_station || '',
    variety: profile.variety || '',
    process_method: profile.process_method || '',
    grade: profile.grade || '',
    altitude: profile.altitude || '',
    flavor: profile.flavor || '',
    bean_list_note: profile.bean_list_note || '',
  }
}

function normalizeRow(row) {
  const profile = row.BeanProfile ?? row.bean_profile ?? {}
  return {
    id: Number(row.ID ?? row.id ?? 0),
    code: row.Code ?? row.code ?? '',
    name: row.Name ?? row.name ?? '',
    kind: row.Kind ?? row.kind ?? 'other',
    unit: row.Unit ?? row.unit ?? 'g',
    batch_no: row.BatchNo ?? row.batch_no ?? '',
    purchase_price: Number(row.PurchasePrice ?? row.purchase_price ?? 0),
    sale_price: Number(row.SalePrice ?? row.sale_price ?? 0),
    bean_profile: cloneBeanProfile({
      origin: profile.Origin ?? profile.origin ?? row.Origin ?? row.origin ?? '',
      processing_station: profile.ProcessingStation ?? profile.processing_station ?? row.ProcessingStation ?? row.processing_station ?? '',
      variety: profile.Variety ?? profile.variety ?? row.Variety ?? row.variety ?? '',
      process_method: profile.ProcessMethod ?? profile.process_method ?? row.ProcessMethod ?? row.process_method ?? '',
      grade: profile.Grade ?? profile.grade ?? row.Grade ?? row.grade ?? '',
      altitude: profile.Altitude ?? profile.altitude ?? row.Altitude ?? row.altitude ?? '',
      flavor: profile.Flavor ?? profile.flavor ?? row.Flavor ?? row.flavor ?? '',
      bean_list_note: profile.BeanListNote ?? profile.bean_list_note ?? row.BeanListNote ?? row.bean_list_note ?? '',
    }),
    onhand_g: Number(row.OnhandG ?? row.onhand_g ?? 0),
    onhand_units: Number(row.OnhandUnits ?? row.onhand_units ?? 0),
    min_level_g: Number(row.MinLevelG ?? row.min_level_g ?? 0),
    min_level_units: Number(row.MinLevelUnits ?? row.min_level_units ?? 0),
    updated_at: row.UpdatedAt ?? row.updated_at ?? '',
  }
}

function profileTitle(row) {
  const p = row.bean_profile || {}
  return [p.origin, p.processing_station].filter(Boolean).join(' / ') || '未设置'
}

function profileSubtitle(row) {
  const p = row.bean_profile || {}
  return [p.process_method, p.variety, p.flavor].filter(Boolean).join(' · ') || '点击设置产地、处理法、风味'
}

function openBeanProfileDialog(row) {
  profileModal.value = {
    open: true,
    row,
    draft: cloneBeanProfile(row.bean_profile),
  }
}

function closeBeanProfileDialog() {
  profileModal.value = {
    open: false,
    row: null,
    draft: emptyBeanProfile(),
  }
}

async function saveBeanProfileDialog() {
  const row = profileModal.value.row
  if (!row) return
  row.bean_profile = cloneBeanProfile(profileModal.value.draft)
  await saveMaterial(row)
  if (!error.value) closeBeanProfileDialog()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/materials', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    const res = await fetch(url)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    rows.value = (data.rows || []).map(normalizeRow)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function saveMaterial(row) {
  savingId.value = row.id
  error.value = ''
  ok.value = false
  try {
    const res = await fetch(`/api/materials/${row.id}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        code: row.code,
        name: row.name,
        kind: row.kind,
        unit: row.unit,
        batch_no: row.batch_no,
        purchase_price: Number(row.purchase_price || 0),
        sale_price: Number(row.sale_price || 0),
        bean_profile: row.kind === 'bean' ? row.bean_profile : null,
        onhand_g: Number(row.onhand_g || 0),
        onhand_units: Number(row.onhand_units || 0),
        min_level_g: Number(row.min_level_g || 0),
        min_level_units: Number(row.min_level_units || 0),
      }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    const next = normalizeRow(data)
    const idx = rows.value.findIndex((item) => item.id === row.id)
    if (idx >= 0) rows.value[idx] = next
    ok.value = true
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    savingId.value = null
  }
}

onMounted(() => {
  q.value = new URL(window.location.href).searchParams.get('q') || ''
  load()
})
</script>

<style scoped>
.page { padding: 16px; display: grid; gap: 16px; }
.panel { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fff; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.filters { display: grid; grid-template-columns: minmax(220px, 1fr) 100px; gap: 12px; align-items: end; max-width: 560px; }
.filters label { display: flex; flex-direction: column; gap: 6px; }
.filters span, .muted { color: #666; font-size: 12px; }
.table-wrap { overflow: auto; margin-top: 10px; }
table { width: 100%; border-collapse: collapse; min-width: 1500px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 8px; text-align: left; vertical-align: middle; }
input, select, textarea, button { font: inherit; }
input, select, textarea { width: 100%; border: 1px solid #ddd; border-radius: 6px; padding: 8px; min-height: 36px; }
textarea { resize: vertical; line-height: 1.45; }
.profile-cell { min-width: 320px; }
.profile-summary { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.profile-summary div { display: grid; gap: 3px; min-width: 0; }
.profile-summary strong, .profile-summary span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.profile-summary strong { font-size: 13px; }
.profile-summary span { color: #666; font-size: 12px; max-width: 220px; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; }
.compact { padding: 7px 10px; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.icon-button { border: 1px solid #ddd; background: #fff; color: #111; width: 36px; height: 36px; padding: 0; font-size: 22px; line-height: 1; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.footer { margin-top: 10px; }
.profile-modal { position: fixed; inset: 0; z-index: 60; background: rgba(0,0,0,.28); display: flex; align-items: center; justify-content: center; padding: 18px; }
.modal-panel { width: min(760px, 100%); max-height: calc(100vh - 36px); overflow: auto; background: #fff; border: 1px solid #e5e5e5; border-radius: 8px; box-shadow: 0 18px 50px rgba(0,0,0,.18); padding: 16px; }
.modal-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.modal-head h3 { margin: 0; font-size: 18px; }
.modal-head p { margin: 4px 0 0; color: #666; font-size: 13px; }
.modal-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.modal-grid label { display: flex; flex-direction: column; gap: 6px; }
.modal-grid span { color: #555; font-size: 12px; font-weight: 600; }
.modal-grid .wide { grid-column: span 2; }
.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 16px; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
  .modal-grid { grid-template-columns: 1fr; }
  .modal-grid .wide { grid-column: span 1; }
}
</style>
