<template>
  <section class="recipient-editor" aria-label="修改代发收件信息">
    <h2>修改收件信息</h2>
    <p class="muted">{{ order.fulfillment_mode === 'stock' ? '库存一件代发' : '无库存一件代发' }}订单处于可编辑状态时，可以修改本单收件信息。商品、数量、费用及库存占用沿用原单；如需更换商品，请取消未发货代发后重新下单。</p>
    <Teleport :to="recipientTarget || 'body'" :disabled="!recipientTarget" defer>
      <div class="recipient-fields">
        <CustomerRecipientPicker :customer-id="Number(order.customer_id)" :current-recipient="recipient" @select="selectRecipient" />
        <label class="wide"><span>粘贴收件信息</span><textarea v-model="pasted" rows="2" placeholder="姓名、电话、地址" :disabled="saving || !!disabledReason" /></label>
        <button type="button" class="secondary" @click="parseRecipient" :disabled="saving || !!disabledReason">解析收件信息</button>
        <label><span>收件人</span><input v-model.trim="recipient.receiver_name" :disabled="saving || !!disabledReason" /></label>
        <label><span>电话</span><input v-model.trim="recipient.receiver_phone" :disabled="saving || !!disabledReason" /></label>
        <label><span>公司（选填）</span><input v-model.trim="recipient.receiver_company" :disabled="saving || !!disabledReason" /></label>
        <label class="wide"><span>地址</span><input v-model.trim="recipient.receiver_address" :disabled="saving || !!disabledReason" /></label>
        <p v-if="disabledReason" class="notice error wide" role="alert">{{ disabledReason }}</p>
        <p v-if="error" class="notice error wide" role="alert">{{ error }}</p>
        <p v-if="saved" class="notice ok wide" role="status">收件信息已保存</p>
        <button class="primary" type="button" @click="save" :disabled="saving || saved || !!disabledReason">{{ saving ? '保存中' : '保存收件信息' }}</button>
      </div>
    </Teleport>
    <div class="stored-items" aria-label="原单商品明细">
      <div v-for="(item, index) in order.items || []" :key="item.item_id || index">
        <strong>{{ item.product_name || '原单商品' }}</strong>
        <span>{{ item.bom_spec_name || item.spec || '原单规格' }} × {{ item.qty }} {{ item.unit || '' }}</span>
      </div>
    </div>
  </section>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import { apiSend } from '../api/client'
import { parseRecipientText } from '../lib/customer-recipient'
import CustomerRecipientPicker from './CustomerRecipientPicker.vue'

const props = defineProps({
  order: { type: Object, required: true },
  recipientTarget: { type: String, default: '' },
  disabledReason: { type: String, default: '' },
})
const emit = defineEmits(['saved'])
const recipient = reactive({ receiver_name: '', receiver_phone: '', receiver_address: '', receiver_company: '' })
const pasted = ref('')
const saving = ref(false)
const saved = ref(false)
const error = ref('')
watch(() => props.order, order => {
  for (const key of Object.keys(recipient)) recipient[key] = String(order?.[key] || '')
  pasted.value = ''
  error.value = ''
  saved.value = false
}, { immediate: true })

function selectRecipient(snapshot) {
  if (saving.value || props.disabledReason) return
  for (const key of Object.keys(recipient)) if (snapshot?.[key] !== undefined) recipient[key] = String(snapshot[key] || '')
}
function parseRecipient() {
  if (saving.value || props.disabledReason) return
  const parsed = parseRecipientText(pasted.value)
  if (parsed.recipient_name) recipient.receiver_name = parsed.recipient_name
  if (parsed.phone) recipient.receiver_phone = parsed.phone
  if (parsed.address) recipient.receiver_address = parsed.address
}
async function save() {
  if (saving.value || saved.value || props.disabledReason) return
  error.value = ''
  if (!recipient.receiver_name.trim() || !recipient.receiver_phone.trim() || !recipient.receiver_address.trim()) {
    error.value = '请填写收件人、电话和地址'
    return
  }
  saving.value = true
  try {
    const data = await apiSend(`/api/orders/${Number(props.order.edit_id)}/recipient`, {
      method: 'PATCH', body: { edit_revision: props.order.edit_revision, ...recipient },
    })
    saved.value = true
    emit('saved', { ...data, edited: true })
  } catch (err) {
    error.value = err.message || '收件信息保存失败'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.recipient-editor h2 { margin: 0 0 8px; }
.muted { color: #66756f; line-height: 1.65; }
.recipient-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.recipient-fields label { display: grid; gap: 6px; min-width: 0; }
.recipient-fields .wide { grid-column: 1 / -1; }
.recipient-fields input, .recipient-fields textarea { box-sizing: border-box; width: 100%; min-width: 0; padding: 9px 11px; border: 1px solid #d5ddd9; border-radius: 8px; font: inherit; }
.recipient-fields button, .recipient-fields :deep(.address-book-button) { padding: 9px 14px; border: 1px solid #c8d8ce; border-radius: 8px; cursor: pointer; justify-self: start; align-self: end; font: inherit; }
.recipient-fields .primary { background: #28624a; color: #fff; }
.recipient-fields .secondary, .recipient-fields :deep(.address-book-button) { background: #fff; color: #315844; }
.recipient-fields button:disabled { opacity: .6; cursor: default; }
.stored-items { display: grid; gap: 10px; margin-top: 16px; }
.stored-items > div { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 8px; padding: 12px; background: #f4f7f5; border-radius: 8px; }
@media (max-width: 640px) { .recipient-fields { grid-template-columns: minmax(0, 1fr); } }
</style>
