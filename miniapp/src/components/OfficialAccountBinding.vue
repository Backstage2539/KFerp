<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import { useSessionStore } from '../stores/session'
import {
  fetchOfficialBindings,
  createOfficialBindingCode,
  updateOfficialBinding,
  type OfficialBinding,
} from '../api/officialAccount'
const session = useSessionStore(),
  rows = ref<OfficialBinding[]>([]),
  enabled = ref(false),
  code = ref(''),
  customerName = ref(''),
  expiresAt = ref(0),
  error = ref(''),
  busy = ref(false)
async function load() {
  code.value = ''
  if (!session.token) return
  try {
    const result = await fetchOfficialBindings(session.token)
    rows.value = result.rows
    enabled.value = result.enabled
  } catch (e) {
    error.value = e instanceof Error ? e.message : '无法读取绑定'
  }
}
async function createCode() {
  busy.value = true
  error.value = ''
  try {
    const data = await createOfficialBindingCode(session.token)
    code.value = data.code
    customerName.value = data.customer_name
    expiresAt.value = Date.now() + data.expires_in * 1000
  } catch (e) {
    error.value = e instanceof Error ? e.message : '无法获取绑定码'
  } finally {
    busy.value = false
  }
}
function copy() {
  if (Date.now() > expiresAt.value) {
    code.value = ''
    error.value = '绑定码已过期，请重新获取'
    return
  }
  uni.setClipboardData({ data: `绑定 ${code.value}` })
}
async function change(
  row: OfficialBinding,
  event: { detail: { value: string | number } },
) {
  const binding = session.bindings[Number(event.detail.value)]
  if (!binding) return
  await update(row, binding.customer_id, false)
}
async function update(
  row: OfficialBinding,
  customerID: number,
  remove: boolean,
) {
  busy.value = true
  error.value = ''
  try {
    await updateOfficialBinding(session.token, row.openid, customerID, remove)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '更新失败'
  } finally {
    busy.value = false
  }
}
onShow(load)
onMounted(load)
onHide(() => {
  code.value = ''
  rows.value = []
})
</script>
<template>
  <view class="official">
    <text class="title">公众号订单查询</text>
    <text class="hint">
      认证后，公众号可回复您的最近订单。多个客户账号可单独选择查询客户。
    </text>
    <text v-if="!enabled" class="hint">公众号尚未启用</text>
    <view v-for="row in rows" :key="row.openid" class="binding">
      <text>
        {{ row.active ? '已绑定' : '已解绑' }} ·
        {{ row.customer_name || '请选择查询客户' }}
      </text>
      <picker
        v-if="row.active"
        :range="session.bindings.map((b) => b.customer_name)"
        :value="
          Math.max(
            0,
            session.bindings.findIndex(
              (b) => b.customer_id === row.customer_id,
            ),
          )
        "
        @change="change(row, $event)"
      >
        <view class="picker">
          公众号查询客户：{{ row.customer_name || '请选择' }} ▾
        </view>
      </picker>
      <button v-if="row.active" :disabled="busy" @tap="update(row, 0, true)">
        解除公众号绑定
      </button>
    </view>
    <button :disabled="busy || !enabled" @tap="createCode">
      获取一次性绑定码
    </button>
    <view v-if="code" class="code">
      <text>{{ customerName }} · 绑定码有效期 5 分钟</text>
      <text class="digits">{{ code }}</text>
      <button @tap="copy">复制绑定指令</button>
      <text>复制后发送到棵凡公众号，完成关联。请勿转发给他人。</text>
    </view>
    <text v-if="error" class="error">{{ error }}</text>
  </view>
</template>
<style scoped>
.official {
  background: white;
  border-radius: 24rpx;
  padding: 30rpx;
  display: flex;
  flex-direction: column;
  gap: 22rpx;
  margin: 24rpx 0;
}
.title {
  font-size: 32rpx;
  font-weight: 700;
}
.hint,
.code {
  font-size: 24rpx;
  color: #58645a;
  line-height: 1.7;
}
.binding {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  border-top: 1rpx solid #eee;
  padding-top: 20rpx;
}
.picker {
  padding: 20rpx;
  background: #f2f7f3;
  border-radius: 12rpx;
}
.code {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
  text-align: center;
}
.digits {
  font-size: 40rpx;
  font-weight: 800;
  color: #254f34;
  letter-spacing: 4rpx;
}
button {
  font-size: 26rpx;
  width: 100%;
}
.error {
  color: #aa3030;
  font-size: 24rpx;
}
</style>
