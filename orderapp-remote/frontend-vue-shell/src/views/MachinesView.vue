<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>设备产能配置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>
    </section>

    <section class="panel">
      <div class="section-title">新增设备</div>
      <div class="form-grid">
        <label><span>名称</span><input v-model.trim="form.name" /></label>
        <label><span>容量(g)</span><input v-model.number="form.capacity_g" type="number" min="1" /></label>
        <label><span>最小烘焙(g)</span><input v-model.number="form.min_roast_g" type="number" min="1" /></label>
        <label><span>允许载量</span><input v-model.trim="form.allowed_specs" placeholder="1000,1500,2000" /></label>
        <label class="checkline"><input v-model="form.active" type="checkbox" />启用</label>
        <button class="primary" type="button" @click="saveNew" :disabled="saving">新增</button>
      </div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead><tr><th>ID</th><th>名称</th><th>容量(g)</th><th>最小烘焙(g)</th><th>允许载量</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.id }}</td>
              <td><input v-model.trim="row.name" /></td>
              <td><input v-model.number="row.capacity_g" type="number" /></td>
              <td><input v-model.number="row.min_roast_g" type="number" /></td>
              <td><input v-model.trim="row.allowed_specs" /></td>
              <td><input v-model="row.active" type="checkbox" /></td>
              <td><button class="secondary" type="button" @click="save(row)" :disabled="saving">保存</button></td>
            </tr>
            <tr v-if="!rows.length"><td colspan="7" class="muted">暂无设备</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const form = reactive({ name: '', capacity_g: 0, min_roast_g: 1000, allowed_specs: '', active: true })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/produce/machines')
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function saveNew() {
  await save(form)
  form.name = ''
  form.capacity_g = 0
  form.min_roast_g = 1000
  form.allowed_specs = ''
}

async function save(row) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await apiSend('/api/produce/machines', { body: row })
    ok.value = true
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
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
h2 { margin: 0; font-size: 20px; }
.section-title { font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: 1fr 110px 130px 1fr 90px 84px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input:not([type="checkbox"]) { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; }
.checkline { display: flex; align-items: center; gap: 6px; height: 38px; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 940px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfaf8; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) { .page { padding: 12px; } .form-grid { grid-template-columns: 1fr; } }
</style>
