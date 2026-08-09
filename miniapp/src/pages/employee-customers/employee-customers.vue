<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { onLoad, onPullDownRefresh } from '@dcloudio/uni-app'
import EmployeeCustomerEditor from '../../components/EmployeeCustomerEditor.vue'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import {
  fetchEmployeeCustomers,
  type EmployeeCustomer,
  type EmployeeCustomersResponse,
} from '../../api/customerPortal'
import { isAuthenticationExpiredRequestError } from '../../api/client'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const context = ref<EmployeeCustomersResponse>()
const query = ref('')
const loading = ref(false)
const loadingMore = ref(false)
const errorMessage = ref('')
const editorOpen = ref(false)
const editingCustomerID = ref(0)
const page = ref(1)
const canMaintainCustomers = computed(() => session.permissions.includes('customers.read')
  && session.permissions.includes('customers.write'))
let customerListSequence = 0
let customerSearchTimer: ReturnType<typeof setTimeout> | undefined

const customerRows = () => context.value?.rows || []

function goToLogin() {
  session.clearSession()
  uni.reLaunch({ url: '/pages/login/login' })
}

async function loadCustomers(append = false) {
  const sequence = ++customerListSequence
  const nextPage = append ? page.value + 1 : 1
  if (append) loadingMore.value = true
  else loading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchEmployeeCustomers(session.token, {
      q: query.value,
      page: nextPage,
      limit: 100,
    })
    if (sequence !== customerListSequence) return
    if (append && context.value) {
      const rows = [...context.value.rows]
      const known = new Set(rows.map((row) => Number(row.id)))
      for (const row of response.rows || []) {
        if (!known.has(Number(row.id))) rows.push(row)
      }
      context.value = { ...response, rows }
    } else {
      context.value = response
    }
    page.value = nextPage
  } catch (cause) {
    if (sequence !== customerListSequence) return
    if (isAuthenticationExpiredRequestError(cause)) {
      goToLogin()
      return
    }
    errorMessage.value = cause instanceof Error ? cause.message : '客户资料加载失败'
  } finally {
    if (sequence === customerListSequence) {
      loading.value = false
      loadingMore.value = false
      uni.stopPullDownRefresh()
    }
  }
}

function reloadCustomers() {
  void loadCustomers(false)
}

function openCreate() {
  if (!canMaintainCustomers.value || !context.value || loading.value) {
    uni.showToast({ title: canMaintainCustomers.value ? '客户资料仍在加载' : '当前账号没有客户维护权限', icon: 'none' })
    return
  }
  editingCustomerID.value = 0
  editorOpen.value = true
}

function openEdit(customer: EmployeeCustomer) {
  if (!canMaintainCustomers.value || !context.value) {
    uni.showToast({ title: '当前账号没有客户维护权限', icon: 'none' })
    return
  }
  if (customer.can_maintain === false && !context.value.is_admin) {
    uni.showToast({ title: '只能维护自己负责的客户', icon: 'none' })
    return
  }
  editingCustomerID.value = Number(customer.id)
  editorOpen.value = true
}

function closeEditor() {
  editorOpen.value = false
}

function customerSaved(customer: EmployeeCustomer) {
  if (!context.value) return
  const index = context.value.rows.findIndex((row) => Number(row.id) === Number(customer.id))
  if (index >= 0) context.value.rows.splice(index, 1, customer)
  else {
    context.value.rows.unshift(customer)
    context.value.total = Number(context.value.total || 0) + 1
  }
  uni.showToast({ title: '客户已保存', icon: 'success' })
  void loadCustomers(false)
}

watch(query, () => {
  if (customerSearchTimer) clearTimeout(customerSearchTimer)
  customerSearchTimer = setTimeout(reloadCustomers, 300)
})

onLoad(reloadCustomers)
onPullDownRefresh(reloadCustomers)
</script>

<template>
  <view class="page pull-up-brand-page">
    <EnvironmentBadge />
    <view class="head">
      <view>
        <text class="title">客户维护</text>
        <text class="subtitle">{{ context?.is_admin ? '可维护全部客户' : '仅显示并维护本人负责的客户' }}</text>
      </view>
      <button
        v-if="canMaintainCustomers"
        class="add"
        :disabled="!context || loading"
        @tap="openCreate"
      >
        新增客户
      </button>
    </view>

    <input v-model="query" class="search" confirm-type="search" placeholder="搜索全部客户 / 公司 / 联系人" @confirm="reloadCustomers" />

    <view v-if="loading" class="state">正在加载客户...</view>
    <view v-else-if="errorMessage" class="state error">
      <text>{{ errorMessage }}</text>
      <button class="retry" @tap="reloadCustomers">重试</button>
    </view>
    <view v-else-if="!customerRows().length" class="state">暂无可维护客户</view>
    <view v-else class="list">
      <view v-for="customer in customerRows()" :key="customer.id" class="customer-card">
        <view class="customer-main">
          <text class="customer-name">{{ customer.name }}</text>
          <text class="customer-meta">{{ customer.company_name || customer.contact || '未填写公司或联系人' }}</text>
          <text v-if="context?.is_admin" class="customer-meta">负责人：{{ customer.responsible_employee_name || '未设置' }}</text>
        </view>
        <button
          class="edit"
          :disabled="!canMaintainCustomers || (customer.can_maintain === false && !context?.is_admin)"
          @tap="openEdit(customer)"
        >
          修改
        </button>
      </view>
      <button
        v-if="context?.has_next"
        class="load-more"
        :loading="loadingMore"
        :disabled="loadingMore"
        @tap="loadCustomers(true)"
      >
        加载更多客户
      </button>
    </view>

    <PullUpBrandFooter />

    <EmployeeCustomerEditor
      :visible="editorOpen"
      :token="session.token"
      :customer-id="editingCustomerID"
      :context="context"
      @close="closeEditor"
      @saved="customerSaved"
    />
  </view>
</template>

<style scoped>
.page { min-height: 100vh; padding: 28rpx; background: #f5f7f6; box-sizing: border-box; }
.head { display: flex; align-items: center; justify-content: space-between; gap: 20rpx; margin-bottom: 22rpx; }
.title { display: block; color: #172c22; font-size: 38rpx; font-weight: 850; }
.subtitle { display: block; margin-top: 8rpx; color: #718078; font-size: 23rpx; }
.add { flex: 0 0 auto; margin: 0; padding: 0 22rpx; min-height: 68rpx; line-height: 68rpx; background: #28624a; color: #fff; font-size: 25rpx; }
.search { width: 100%; min-height: 82rpx; margin-bottom: 20rpx; padding: 0 22rpx; border: 2rpx solid #b8ccc0; border-radius: 12rpx; box-sizing: border-box; background: #fff; }
.list { display: flex; flex-direction: column; gap: 16rpx; }
.customer-card { display: flex; align-items: center; justify-content: space-between; gap: 18rpx; padding: 24rpx; border: 1rpx solid #e0e8e3; border-radius: 16rpx; background: #fff; }
.customer-main { min-width: 0; }
.customer-name { display: block; overflow: hidden; color: #172c22; font-size: 29rpx; font-weight: 750; text-overflow: ellipsis; white-space: nowrap; }
.customer-meta { display: block; overflow: hidden; margin-top: 8rpx; color: #718078; font-size: 23rpx; text-overflow: ellipsis; white-space: nowrap; }
.edit { flex: 0 0 auto; margin: 0; padding: 0 22rpx; min-height: 62rpx; line-height: 62rpx; border: 1rpx solid #28624a; background: #fff; color: #28624a; font-size: 24rpx; }
.load-more { width: 100%; margin: 8rpx 0 0; border: 1rpx solid #b8ccc0; background: #fff; color: #28624a; font-size: 25rpx; }
.state { display: flex; flex-direction: column; align-items: center; gap: 20rpx; padding: 90rpx 20rpx; color: #718078; text-align: center; }
.error { color: #a7352a; }
.retry { margin: 0; padding: 0 24rpx; min-height: 62rpx; line-height: 62rpx; background: #28624a; color: #fff; font-size: 24rpx; }
</style>
