<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>界面设置</h2>
          <p>控制不同账号模式下的 ERP 页面入口。</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>

      <label class="toggle-row">
        <input v-model="form.hide_customer_account_fulfillment" type="checkbox" />
        <span>
          <strong>客户账户模式隐藏履约运营台</strong>
          <small>开启后，客户账户模式不显示内部履约运营台入口；页面和接口仍保留。</small>
        </span>
      </label>

      <div class="actions">
        <button class="primary" type="button" @click="save" :disabled="saving || loading">
          {{ saving ? '保存中' : '保存设置' }}
        </button>
        <span v-if="ok" class="ok">{{ ok }}</span>
        <span v-if="error" class="error">{{ error }}</span>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { fetchUISettings, saveUISettings } from '../api/ui-settings'

const loading = ref(false)
const saving = ref(false)
const ok = ref('')
const error = ref('')
const form = reactive({
  hide_customer_account_fulfillment: true,
})

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
    error.value = err.message || '加载界面设置失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    assignSettings(await saveUISettings({
      hide_customer_account_fulfillment: !!form.hide_customer_account_fulfillment,
    }))
    ok.value = '已保存界面设置'
  } catch (err) {
    error.value = err.message || '保存界面设置失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; max-width: 820px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 16px; }
h2 { margin: 0; font-size: 22px; }
p { margin: 5px 0 0; color: #666; font-size: 13px; }
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
</style>
