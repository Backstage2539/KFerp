<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { MiniRequestError, miniRequest } from '../api/client'
import { useSessionStore } from '../stores/session'
import { orderNotificationChoices } from '../utils/orderNotificationChoices'
import { notificationPromptCoordinator, onNotificationPageVisibility, promptSelection, type PromptChoice, type PromptKey } from '../utils/notificationPrompt'

type Preference = { employee_id: number; customer_id: number; active?: boolean; subscribed: boolean; enabled: boolean; events: string[] }
type PromptResponse = { enabled: boolean; prompt_required?: boolean; prompt_version?: number; mini_user_id: number; customer_id: number; employee_id: number; choice?: PromptChoice | null; rows: Preference[] }

const session = useSessionStore()
const visible = ref(false)
const busy = ref(false)
const error = ref('')
const events = ref<string[]>([])
const previousHint = ref('')
const contextLabel = computed(() => session.accountType === 'employee' ? `当前员工：${session.employeeName || '员工账号'}` : `当前客户：${session.currentCustomerName || '客户账号'}`)
let pageVisible = false
let generation = 0
let activeKey: PromptKey | null = null
let requiresBinding = false
let ownerPage: ReturnType<typeof getCurrentPages>[number] | undefined
let unsubscribe = () => {}

function identity() {
  return `${session.token}:${session.miniUserID}:${session.currentCustomerID}:${session.employeeID}:${session.isCustomerPreview}`
}

async function check() {
  const sequence = ++generation
  if (activeKey) notificationPromptCoordinator.release(activeKey)
  activeKey = null
  visible.value = false
  if (!pageVisible || !session.token || !session.miniUserID || session.isCustomerPreview) return
  const pages = getCurrentPages()
  if (pages[pages.length - 1]?.route === 'pages/order-notifications/order-notifications') return
  const current = identity()
  try {
    const result = await miniRequest<PromptResponse>('/api/mini/order-notifications', { token: session.token })
    if (sequence !== generation || !pageVisible || identity() !== current || result.mini_user_id !== session.miniUserID || result.customer_id !== session.currentCustomerID || result.employee_id !== session.employeeID || !result.enabled || !result.prompt_required || !Number.isInteger(result.prompt_version) || !result.prompt_version) return
    const key = { user: session.miniUserID, customer: session.currentCustomerID, employee: session.employeeID, version: result.prompt_version }
    if (!notificationPromptCoordinator.claim(key)) return
    activeKey = key
    const previous = (result.rows || []).find(row => session.accountType === 'employee' ? row.employee_id === session.employeeID : row.customer_id === session.currentCustomerID && row.employee_id === 0)
    const selection = promptSelection(result.choice || null, previous || null)
    events.value = selection.events
    previousHint.value = (result.choice || previous) ? (selection.enabled ? '以下为当前设置，请确认或调整。' : '当前通知已关闭，以下为保留的选择；开启后才会接收。') : ''
    requiresBinding = !previous || previous.active === false || !previous.subscribed
    error.value = ''
    visible.value = true
  } catch {
    // An unavailable notification service must never prevent normal app use.
  }
}

function toggle(key: string) {
  events.value = events.value.includes(key) ? events.value.filter(item => item !== key) : [...events.value, key]
}

function postpone() {
  if (busy.value || !activeKey) return
  notificationPromptCoordinator.dismiss(activeKey)
  activeKey = null
  visible.value = false
}

async function save(enabled: boolean) {
  if (busy.value || !activeKey || (enabled && events.value.length === 0)) return
  const key = activeKey
  const current = identity()
  busy.value = true
  error.value = ''
  try {
    await miniRequest('/api/mini/order-notifications/prompt', { token: session.token, method: 'PUT', data: { version: key.version, mini_user_id: key.user, customer_id: key.customer, employee_id: key.employee, enabled, events: enabled ? events.value : [] } })
    if (activeKey !== key || current !== identity()) return
    notificationPromptCoordinator.release(key)
    activeKey = null
    visible.value = false
    if (enabled && requiresBinding) {
      uni.showModal({ title: '关注服务号', content: '已保存通知选择，关注并绑定服务号后，可接收系统已开通的通知。', confirmText: '去关注', cancelText: '稍后', success: result => {
        if (result.confirm && current === identity()) uni.navigateTo({ url: '/pages/order-notifications/order-notifications' })
      } })
    }
  } catch (cause) {
    if (cause instanceof MiniRequestError && cause.statusCode === 409 && activeKey === key) {
      await check()
      if (visible.value) error.value = '通知选项已更新，请重新选择。'
    } else {
      error.value = cause instanceof Error ? cause.message : '保存失败，请重试'
    }
  } finally {
    busy.value = false
  }
}

watch(() => [session.token, session.miniUserID, session.currentCustomerID, session.employeeID, session.isCustomerPreview], () => { if (pageVisible) void check() })
function stop() { pageVisible = false; generation++; if (activeKey) notificationPromptCoordinator.release(activeKey); activeKey = null; visible.value = false }
function showIfCurrent() {
  if (typeof getCurrentPages !== 'function') return
  const pages = getCurrentPages()
  if (ownerPage && pages[pages.length - 1] === ownerPage) { pageVisible = true; void check() }
  else stop()
}
onMounted(() => {
  if (typeof getCurrentPages !== 'function') return
  const pages = getCurrentPages()
  ownerPage = pages[pages.length - 1]
  unsubscribe = onNotificationPageVisibility(show => { if (show) showIfCurrent(); else stop() })
  showIfCurrent()
})
onUnmounted(() => { unsubscribe(); stop() })
</script>

<template>
  <view v-if="visible" class="notification-overlay" @touchmove.stop.prevent>
    <view class="notification-dialog">
      <text class="notification-title">选择订单通知</text>
      <text class="notification-hint">{{ contextLabel }}</text>
      <text class="notification-hint">选择希望通过服务号接收的通知，可在“我的 → 订单通知”随时修改。</text>
      <text class="notification-hint">部分通知尚未开通，选中后仍以服务号模板和系统启用情况为准。</text>
      <text v-if="previousHint" class="notification-hint">{{ previousHint }}</text>
      <scroll-view scroll-y class="notification-options">
        <label v-for="[key, label] in orderNotificationChoices" :key="key" class="notification-option" @tap="toggle(key)">
          <checkbox :checked="events.includes(key)" :disabled="busy" />{{ label }}
        </label>
      </scroll-view>
      <text v-if="error" class="notification-error">{{ error }}</text>
      <button class="notification-confirm" :disabled="busy || events.length === 0" @tap="save(true)">开启并保存所选通知</button>
      <button class="notification-secondary" :disabled="busy" @tap="save(false)">暂不接收通知</button>
      <button class="notification-later" :disabled="busy" @tap="postpone">稍后再说</button>
    </view>
  </view>
</template>

<style scoped>
.notification-overlay { position: fixed; inset: 0; z-index: 9999; display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,.48); padding: 32rpx; box-sizing: border-box; }
.notification-dialog { display: flex; flex-direction: column; gap: 18rpx; width: 100%; max-width: 640rpx; padding: 32rpx; box-sizing: border-box; border-radius: 20rpx; background: #fff; }
.notification-title { font-size: 36rpx; font-weight: 600; color: #18251c; }
.notification-hint { font-size: 25rpx; line-height: 1.5; color: #66736a; }
.notification-options { max-height: 480rpx; }
.notification-option { display: flex; align-items: center; gap: 16rpx; padding: 16rpx 0; font-size: 28rpx; }
.notification-error { color: #b42323; font-size: 25rpx; }
.notification-dialog button { width: 100%; font-size: 28rpx; line-height: 2.5; }
.notification-confirm { color: #fff; background: #28553a; }
.notification-secondary { color: #28553a; background: #f1f6f2; }
.notification-later { color: #59645d; background: #fff; }
</style>
