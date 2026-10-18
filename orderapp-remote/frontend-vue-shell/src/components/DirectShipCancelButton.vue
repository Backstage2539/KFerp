<script setup>
import { ref } from 'vue'
import { apiSend } from '../api/client'
const props = defineProps({ customerId: { type: Number, required: true }, orderId: { type: Number, required: true } })
const emit = defineEmits(['cancelled'])
const cancelling = ref(false)
const error = ref('')
async function cancel() {
  if (cancelling.value || props.customerId <= 0 || props.orderId <= 0) return
  if (!window.confirm('确认取消本次代发？系统将释放预订并冲回未结算代发费用。')) return
  cancelling.value = true; error.value = ''
  try { await apiSend(`/api/customer-fulfillment/${props.customerId}/direct-ship/orders/${props.orderId}/cancel`, { body: {} }); emit('cancelled', props.orderId) }
  catch (cause) { error.value = cause.message || '取消代发失败' }
  finally { cancelling.value = false }
}
</script>
<template><span class="cancel-direct-ship"><button type="button" :disabled="cancelling || customerId <= 0 || orderId <= 0" @click.stop="cancel">{{ cancelling ? '取消中…' : '取消代发' }}</button><span v-if="error" class="error" role="alert">{{ error }}</span></span></template>
<style scoped>.cancel-direct-ship{display:inline-flex;flex-direction:column;gap:5px;align-items:flex-start}button{padding:6px 10px;border:1px solid #dcc7be;border-radius:6px;background:#fff;color:#9b3d30;cursor:pointer}button:disabled{opacity:.5;cursor:default}.error{max-width:240px;color:#b42318;font-size:12px;white-space:normal}</style>
