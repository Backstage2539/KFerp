<template>
  <section class="notification-settings">
    <header class="view-head">
      <div>
        <h2>通知配置</h2>
        <p>按业务事件配置接收人和通道，外部 IM 通道先进入投递队列。</p>
      </div>
      <button type="button" class="primary" @click="load">刷新</button>
    </header>

    <div class="settings-grid">
      <form class="rule-form" @submit.prevent="save">
        <label><span>规则编码</span><input v-model.trim="form.code" required placeholder="order_shipped_customer_im" /></label>
        <label><span>规则名称</span><input v-model.trim="form.name" placeholder="已发货通知客户" /></label>
        <label><span>事件类型</span><input v-model.trim="form.event_type" required placeholder="order.shipped" /></label>
        <label><span>主题</span><input v-model.trim="form.topic" placeholder="orders" /></label>
        <label><span>来源类型</span><input v-model.trim="form.source_type" placeholder="order" /></label>
        <label>
          <span>通知通道</span>
          <select v-model="form.channel">
            <option value="erp_platform">ERP站内</option>
            <option value="external_im">外部IM</option>
            <option value="wechat_service_account">微信服务号</option>
            <option value="enterprise_wechat">企业微信</option>
          </select>
        </label>
        <label>
          <span>接收人类型</span>
          <select v-model="form.target_type">
            <option value="role">角色用户</option>
            <option value="permission">权限用户</option>
            <option value="employee">指定员工</option>
            <option value="order_responsible">订单负责人</option>
            <option value="order_customer">订单客户</option>
            <option value="broadcast">所有人</option>
          </select>
        </label>
        <label><span>接收人键</span><input v-model.trim="form.target_key" placeholder="sales / production / customer" /></label>
        <label><span>员工ID</span><input v-model.number="form.target_employee_id" type="number" min="0" /></label>
        <label><span>模板键</span><input v-model.trim="form.template_key" placeholder="order_shipped_customer" /></label>
        <label><span>适配器</span><input v-model.trim="form.adapter_key" placeholder="enterprise_wechat" /></label>
        <label>
          <span>状态</span>
          <select v-model="form.enabled">
            <option :value="true">启用</option>
            <option :value="false">停用</option>
          </select>
        </label>
        <label class="wide">
          <span>Payload 条件(JSON)</span>
          <textarea v-model.trim="payloadMatchText" rows="4" placeholder='{"new_status":"已发货"}'></textarea>
        </label>
        <button type="submit" class="primary" :disabled="saving">{{ saving ? '保存中' : '保存规则' }}</button>
        <div v-if="message" class="notice ok">{{ message }}</div>
        <div v-if="error" class="notice error">{{ error }}</div>
      </form>

      <div class="rule-list">
        <table>
          <thead>
            <tr>
              <th>状态</th>
              <th>事件</th>
              <th>通道</th>
              <th>接收人</th>
              <th>模板</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in rules" :key="rule.code" @click="edit(rule)">
              <td><span class="badge" :class="{ off: !rule.enabled }">{{ rule.enabled ? '启用' : '停用' }}</span></td>
              <td><strong>{{ rule.event_type }}</strong><span>{{ rule.name || rule.code }}</span></td>
              <td>{{ rule.channel }}</td>
              <td>{{ rule.target_type }}: {{ rule.target_key || rule.target_employee_id || '-' }}</td>
              <td>{{ rule.template_key || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { fetchNotificationRules, saveNotificationRule } from '../api/message-center.js'

const rules = ref([])
const saving = ref(false)
const message = ref('')
const error = ref('')
const payloadMatchText = ref('')
const form = reactive(defaultForm())

function defaultForm() {
  return {
    code: '',
    name: '',
    enabled: true,
    topic: 'orders',
    event_type: '',
    source_type: 'order',
    channel: 'erp_platform',
    target_type: 'role',
    target_key: '',
    target_employee_id: 0,
    template_key: '',
    adapter_key: '',
  }
}

function reset(next = {}) {
  Object.assign(form, defaultForm(), next)
  payloadMatchText.value = next.payload_match && Object.keys(next.payload_match).length ? JSON.stringify(next.payload_match, null, 2) : ''
}

async function load() {
  error.value = ''
  const data = await fetchNotificationRules()
  rules.value = data.rules || []
}

function edit(rule) {
  reset({ ...rule })
  message.value = ''
  error.value = ''
}

async function save() {
  saving.value = true
  message.value = ''
  error.value = ''
  try {
    let payloadMatch = {}
    if (payloadMatchText.value) payloadMatch = JSON.parse(payloadMatchText.value)
    await saveNotificationRule({ ...form, payload_match: payloadMatch })
    message.value = '已保存通知规则'
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
.notification-settings { display: grid; gap: 16px; }
.view-head { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; }
.view-head h2 { margin: 0 0 4px; font-size: 22px; }
.view-head p { margin: 0; color: #666; font-size: 13px; }
.settings-grid { display: grid; grid-template-columns: minmax(280px, 360px) minmax(0, 1fr); gap: 16px; align-items: start; }
.rule-form { display: grid; gap: 10px; padding: 14px; border: 1px solid #e3e6ea; border-radius: 6px; background: #fff; }
.rule-form label { display: grid; gap: 4px; font-size: 13px; color: #555; }
.rule-form input, .rule-form select, .rule-form textarea { width: 100%; box-sizing: border-box; border: 1px solid #d7dce2; border-radius: 4px; padding: 8px; font: inherit; }
.rule-form textarea { resize: vertical; }
.rule-list { overflow: auto; border: 1px solid #e3e6ea; border-radius: 6px; background: #fff; }
table { width: 100%; border-collapse: collapse; font-size: 13px; }
th, td { padding: 10px; border-bottom: 1px solid #edf0f3; text-align: left; vertical-align: top; }
tbody tr { cursor: pointer; }
tbody tr:hover { background: #f6f8fa; }
td strong, td span { display: block; }
td span { color: #666; margin-top: 3px; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 999px; color: #166534; background: #dcfce7; }
.badge.off { color: #666; background: #eee; }
.primary { border: 0; border-radius: 4px; padding: 8px 12px; background: #1f6feb; color: #fff; cursor: pointer; }
.primary:disabled { opacity: .65; cursor: not-allowed; }
.notice { padding: 8px 10px; border-radius: 4px; font-size: 13px; }
.notice.ok { color: #166534; background: #dcfce7; }
.notice.error { color: #b91c1c; background: #fee2e2; }
@media (max-width: 900px) {
  .settings-grid { grid-template-columns: 1fr; }
  .view-head { align-items: stretch; flex-direction: column; }
}
</style>
