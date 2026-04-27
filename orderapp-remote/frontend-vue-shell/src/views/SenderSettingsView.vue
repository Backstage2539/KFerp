<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>发货人设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>

      <div class="sender-layout">
        <div class="sender-list">
          <div class="sender-list-head">
            <h3>寄件人列表</h3>
            <button class="secondary" type="button" @click="newProfile">新增寄件人</button>
          </div>
          <button
            v-for="profile in profiles"
            :key="profile.sender_id"
            type="button"
            class="sender-card"
            :class="{ active: Number(form.sender_id) === Number(profile.sender_id) }"
            @click="editProfile(profile)"
          >
            <strong>{{ profile.sender_label || profile.sender_name || '寄件人' }}</strong>
            <span>{{ profile.sender_name || '-' }} / {{ profile.sender_phone || '-' }}</span>
            <small v-if="profile.is_default">默认寄件人</small>
          </button>
          <div v-if="!profiles.length" class="muted">暂无寄件人</div>
        </div>

        <div class="editor">
          <div class="form-grid">
            <label><span>名称</span><input v-model.trim="form.sender_label" placeholder="如：仓库/门店" /></label>
            <label><span>姓名</span><input v-model.trim="form.sender_name" /></label>
            <label><span>电话</span><input v-model.trim="form.sender_phone" /></label>
            <label><span>公司</span><input v-model.trim="form.sender_company" /></label>
            <label><span>货品名</span><input v-model.trim="form.sender_goods" /></label>
            <label><span>顺丰业务类型</span><input v-model.trim="form.sf_biz_type" /></label>
            <label class="wide"><span>地址</span><input v-model.trim="form.sender_addr" /></label>
            <label class="check-row"><input v-model="form.is_default" type="checkbox" /> <span>设为默认</span></label>
            <label class="check-row"><input v-model="form.active" type="checkbox" /> <span>启用</span></label>
          </div>
          <div class="actions">
            <button class="primary" type="button" @click="save" :disabled="saving">保存</button>
            <button class="secondary" type="button" @click="markDefault" :disabled="saving || !form.sender_id">设为默认</button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const profiles = ref([])

const form = reactive(emptyProfile())

function emptyProfile() {
  return {
    sender_id: 0,
    sender_label: '',
    sender_name: '',
    sender_phone: '',
    sender_addr: '',
    sender_company: '',
    sender_goods: '茶叶',
    sf_biz_type: '',
    is_default: false,
    active: true,
  }
}

function assignProfile(profile) {
  Object.assign(form, emptyProfile(), profile || {})
  if (!form.sender_goods) form.sender_goods = '茶叶'
  form.active = profile?.active !== false
}

function newProfile() {
  ok.value = false
  assignProfile(null)
}

function editProfile(profile) {
  ok.value = false
  assignProfile(profile)
}

function markDefault() {
  form.is_default = true
  save()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch('/api/settings/sender')
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    profiles.value = data.profiles || []
    assignProfile(data.profile || profiles.value[0] || null)
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
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; max-width: 1180px; }
.panel-head, .sender-list-head, .actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
.sender-layout { display: grid; grid-template-columns: 320px 1fr; gap: 14px; align-items: start; }
.sender-list { border-right: 1px solid #eee8df; padding-right: 14px; }
.sender-card { width: 100%; height: auto; display: grid; gap: 4px; text-align: left; border-color: #e0d8ce; margin-bottom: 8px; padding: 10px; background: #fff; }
.sender-card.active { border-color: #1f1f1f; background: #fbfaf8; }
.sender-card span { color: #555; font-size: 13px; }
.sender-card small { color: #1f6b38; font-size: 12px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 10px; margin-bottom: 12px; }
.wide { grid-column: 1 / -1; }
.check-row { flex-direction: row; align-items: center; gap: 8px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; }
.check-row input { width: 16px; height: 16px; padding: 0; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.muted { color: #666; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .sender-layout, .form-grid { grid-template-columns: 1fr; }
  .sender-list { border-right: 0; border-bottom: 1px solid #eee8df; padding-right: 0; padding-bottom: 12px; }
}
</style>
