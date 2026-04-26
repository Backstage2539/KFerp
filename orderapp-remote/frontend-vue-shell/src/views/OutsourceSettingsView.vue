<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>代加工模板设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>

      <div class="form-grid">
        <label>
          <span>模板名称</span>
          <input v-model.trim="form.name" placeholder="代加工默认模板" />
        </label>
        <label>
          <span>烘焙单价</span>
          <input v-model.number="form.roast_unit_price" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>咖啡豆包装费单价</span>
          <input v-model.number="form.bean_pack_unit_price" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>挂耳包装费单价</span>
          <input v-model.number="form.drip_pack_unit_price" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>SC挂靠费单价</span>
          <input v-model.number="form.sc_unit_price" type="number" min="0" step="0.01" />
        </label>
        <label class="check">
          <input v-model="form.is_default" type="checkbox" />
          <span>设为默认模板</span>
        </label>
      </div>
      <button class="primary" type="button" @click="save" :disabled="saving">保存模板</button>
    </section>

    <section class="panel">
      <div class="section-title">现有模板</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>模板</th>
              <th>默认</th>
              <th>烘焙</th>
              <th>豆包装</th>
              <th>挂耳包装</th>
              <th>SC挂靠费</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.name }}</td>
              <td>{{ row.is_default ? '是' : '否' }}</td>
              <td>{{ money(row.roast_unit_price) }}</td>
              <td>{{ money(row.bean_pack_unit_price) }}</td>
              <td>{{ money(row.drip_pack_unit_price) }}</td>
              <td>{{ money(row.sc_unit_price) }}</td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="6" class="muted">暂无模板</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const rows = ref([])
const form = reactive({
  name: '',
  is_default: false,
  roast_unit_price: 0,
  bean_pack_unit_price: 0,
  drip_pack_unit_price: 0,
  sc_unit_price: 0,
})

function money(value) {
  return Number(value || 0).toFixed(2)
}

function resetForm() {
  form.name = ''
  form.is_default = false
  form.roast_unit_price = 0
  form.bean_pack_unit_price = 0
  form.drip_pack_unit_price = 0
  form.sc_unit_price = 0
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/outsource/templates')
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
  ok.value = false
  try {
    await apiSend('/api/outsource/templates', {
      body: {
        name: form.name,
        is_default: Boolean(form.is_default),
        roast_unit_price: Number(form.roast_unit_price || 0),
        bean_pack_unit_price: Number(form.bean_pack_unit_price || 0),
        drip_pack_unit_price: Number(form.drip_pack_unit_price || 0),
        sc_unit_price: Number(form.sc_unit_price || 0),
      },
    })
    ok.value = true
    resetForm()
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 1120px; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)); gap: 10px; margin-bottom: 12px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; }
.check { display: flex; align-items: center; gap: 8px; min-height: 38px; padding-top: 18px; }
.check input { width: 18px; height: 18px; }
.check span { margin: 0; color: #222; font-size: 14px; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.section-title { font-weight: 700; margin-bottom: 10px; }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; min-width: 720px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfaf8; }
.muted { color: #666; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid { grid-template-columns: 1fr; }
  .check { padding-top: 0; }
}
</style>
