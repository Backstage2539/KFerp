<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>发货人设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>
      <div class="form-grid">
        <label><span>姓名</span><input v-model.trim="form.sender_name" /></label>
        <label><span>电话</span><input v-model.trim="form.sender_phone" /></label>
        <label><span>公司</span><input v-model.trim="form.sender_company" /></label>
        <label><span>货品名</span><input v-model.trim="form.sender_goods" /></label>
        <label><span>顺丰业务类型</span><input v-model.trim="form.sf_biz_type" /></label>
        <label class="wide"><span>地址</span><input v-model.trim="form.sender_addr" /></label>
      </div>
      <button class="primary" type="button" @click="save" :disabled="saving">保存</button>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const form = reactive({
  sender_name: '',
  sender_phone: '',
  sender_addr: '',
  sender_company: '',
  sender_goods: '茶叶',
  sf_biz_type: '',
})

function assignProfile(profile) {
  for (const key of Object.keys(form)) {
    form[key] = profile?.[key] || ''
  }
  if (!form.sender_goods) form.sender_goods = '茶叶'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch('/api/settings/sender')
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    assignProfile(data.profile)
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
    const res = await fetch('/api/settings/sender', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    ok.value = true
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
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; max-width: 980px; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 10px; margin-bottom: 12px; }
.wide { grid-column: 1 / -1; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) { .page { padding: 12px; } .form-grid { grid-template-columns: 1fr; } }
</style>
