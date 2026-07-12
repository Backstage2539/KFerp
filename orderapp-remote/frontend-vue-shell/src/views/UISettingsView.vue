<template>
  <div class="system-settings-page">
    <header class="page-head">
      <div>
        <h2>系统设置</h2>
        <p>维护 ERP 全局入口与通知规则。</p>
      </div>
    </header>

    <nav class="settings-tabs" role="tablist" aria-label="系统设置">
      <button type="button" role="tab" :aria-selected="activeTab === 'base'" :class="{ active: activeTab === 'base' }" @click="activeTab = 'base'">系统基础设置</button>
      <button type="button" role="tab" :aria-selected="activeTab === 'notifications'" :class="{ active: activeTab === 'notifications' }" @click="activeTab = 'notifications'">通知设置</button>
    </nav>

    <section v-if="activeTab === 'base'" class="page" role="tabpanel">
      <div class="panel">
        <div class="panel-head">
          <div>
            <h2>系统基础设置</h2>
            <p>维护跨商品、生产和库存共用的系统级显示规则。</p>
          </div>
          <button class="secondary" type="button" :disabled="loading" @click="load">刷新</button>
        </div>

        <label class="toggle-row">
          <input v-model="form.hide_customer_account_fulfillment" type="checkbox" />
          <span>
            <strong>客户账户模式隐藏履约运营台</strong>
            <small>开启后，客户账户模式不显示内部履约运营台入口；页面和接口仍保留。</small>
          </span>
        </label>

        <div class="actions">
          <button class="primary" type="button" :disabled="saving || loading" @click="save">{{ saving ? '保存中' : '保存设置' }}</button>
          <span v-if="ok" class="ok">{{ ok }}</span>
          <span v-if="error" class="error">{{ error }}</span>
        </div>
      </div>
    </section>

    <NotificationSettingsView v-else />
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { fetchUISettings, saveUISettings } from '../api/ui-settings'
import NotificationSettingsView from './NotificationSettingsView.vue'

const activeTab = ref('base')
const loading = ref(false)
const saving = ref(false)
const ok = ref('')
const error = ref('')
const form = reactive({ hide_customer_account_fulfillment: true })

function assignSettings(data) {
  const settings = data?.settings || data || {}
  form.hide_customer_account_fulfillment = settings.hide_customer_account_fulfillment !== false
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    assignSettings(await fetchUISettings())
  } catch (err) {
    error.value = err.message || '加载系统设置失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    assignSettings(await saveUISettings({ hide_customer_account_fulfillment: !!form.hide_customer_account_fulfillment }))
    ok.value = '已保存系统设置'
  } catch (err) {
    error.value = err.message || '保存系统设置失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.system-settings-page { min-height: 100%; background: #f7f8fa; color: #171717; }
.page-head { padding: 18px 18px 12px; background: #fff; border-bottom: 1px solid #e6e8eb; }
.page-head h2, .panel h2 { margin: 0; font-size: 22px; }
p { margin: 5px 0 0; color: #666; font-size: 13px; }
.settings-tabs { display: flex; gap: 6px; padding: 12px 18px 0; background: #fff; border-bottom: 1px solid #e6e8eb; }
.settings-tabs button { min-height: 42px; border: 0; border-bottom: 3px solid transparent; background: transparent; padding: 8px 14px; color: #555; cursor: pointer; }
.settings-tabs button.active { border-bottom-color: #111827; color: #111827; font-weight: 700; }
.page { padding: 18px; }
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; max-width: 1080px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 16px; }
.toggle-row { display: flex; gap: 10px; align-items: flex-start; padding: 12px; border: 1px solid #e6eaf0; border-radius: 8px; background: #fbfcfe; cursor: pointer; }
.toggle-row input { margin-top: 3px; width: 18px; height: 18px; }
.toggle-row strong { display: block; font-size: 15px; }
.toggle-row small { display: block; color: #666; font-size: 13px; margin-top: 4px; }
.actions { display: flex; align-items: center; gap: 10px; margin-top: 14px; flex-wrap: wrap; }
button { border: 1px solid #d7dde6; border-radius: 6px; background: #fff; padding: 8px 12px; cursor: pointer; }
button.primary { background: #111827; color: #fff; border-color: #111827; }
button:disabled { opacity: .55; cursor: not-allowed; }
.ok { color: #0f766e; font-size: 13px; }
.error { color: #b91c1c; font-size: 13px; }
@media (max-width: 760px) {
  .page-head, .page { padding-left: 12px; padding-right: 12px; }
  .settings-tabs { overflow-x: auto; padding-left: 12px; padding-right: 12px; }
  .panel-head { flex-direction: column; }
}
</style>
