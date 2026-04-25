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
              <th>进货价</th>
              <th>销售价</th>
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
              <td><input type="number" min="0" step="0.01" v-model.number="row.purchase_price" /></td>
              <td><input type="number" min="0" step="0.01" v-model.number="row.sale_price" /></td>
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
              <td colspan="12" class="muted">暂无物料</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="muted footer">保存单行会记录到操作日志，可在“操作日志”中按物料查看变更字段。</div>
    </section>
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

function normalizeRow(row) {
  return {
    id: Number(row.ID ?? row.id ?? 0),
    code: row.Code ?? row.code ?? '',
    name: row.Name ?? row.name ?? '',
    kind: row.Kind ?? row.kind ?? 'other',
    unit: row.Unit ?? row.unit ?? 'g',
    purchase_price: Number(row.PurchasePrice ?? row.purchase_price ?? 0),
    sale_price: Number(row.SalePrice ?? row.sale_price ?? 0),
    onhand_g: Number(row.OnhandG ?? row.onhand_g ?? 0),
    onhand_units: Number(row.OnhandUnits ?? row.onhand_units ?? 0),
    min_level_g: Number(row.MinLevelG ?? row.min_level_g ?? 0),
    min_level_units: Number(row.MinLevelUnits ?? row.min_level_units ?? 0),
    updated_at: row.UpdatedAt ?? row.updated_at ?? '',
  }
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
        purchase_price: Number(row.purchase_price || 0),
        sale_price: Number(row.sale_price || 0),
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
table { width: 100%; border-collapse: collapse; min-width: 1280px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 8px; text-align: left; vertical-align: middle; }
input, select, button { font: inherit; }
input, select { width: 100%; border: 1px solid #ddd; border-radius: 6px; padding: 8px; min-height: 36px; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.footer { margin-top: 10px; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
}
</style>
