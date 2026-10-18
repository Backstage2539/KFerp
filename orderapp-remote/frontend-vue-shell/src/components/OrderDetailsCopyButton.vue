<script setup>
import { ref, watch, onBeforeUnmount, onMounted, nextTick } from 'vue'
import { fetchOrderDetailsText } from '../api/order-details-copy.js'
import { createOrderDetailsCopyAction } from '../lib/order-details-copy.js'
const props = defineProps({ orderId: { type: [Number, String], required: true }, customerPortal: Boolean, customerId: { type: [Number, String], default: 0 } })
const busy = ref(false)
const message = ref('')
const manualText = ref('')
const failed = ref(false)
const copyButton = ref(null)
const textArea = ref(null)
let timer
let alive = true
const source = Symbol('order-copy')
const copyStarted = event => { if (event.detail !== source) reset() }
const context = () => JSON.stringify([alive, props.orderId, props.customerId, props.customerPortal, typeof window === 'undefined' ? '' : window.localStorage?.getItem('auth_token') || ''])
const action = createOrderDetailsCopyAction({
  context,
  fetchDetails: id => fetchOrderDetailsText(id, props.customerPortal, props.customerId),
  writeText: text => {
    if (!globalThis.navigator?.clipboard?.writeText) return Promise.reject(new Error('clipboard unavailable'))
    return globalThis.navigator.clipboard.writeText(text)
  },
})
function reset() { action.invalidate(); busy.value = false; message.value = ''; manualText.value = ''; clearTimeout(timer) }
watch(() => [props.orderId, props.customerId, props.customerPortal], reset, { flush: 'sync' })
onMounted(() => { if (typeof window !== 'undefined') window.addEventListener('kferp:order-copy-start', copyStarted) })
onBeforeUnmount(() => { alive = false; reset(); if (typeof window !== 'undefined') window.removeEventListener('kferp:order-copy-start', copyStarted) })
watch(manualText, async value => { await nextTick(); if (value) textArea.value?.focus(); else if (alive) copyButton.value?.focus() })
async function copy() {
  if (busy.value) return
  if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('kferp:order-copy-start', { detail: source }))
  busy.value = true
  message.value = ''
  manualText.value = ''
  const scope = context()
  const result = await action.copy(Number(props.orderId))
  busy.value = action.busy
  if (scope !== context() || !alive) return
  if (result.status === 'manual') manualText.value = result.text
  if (result.status === 'copied' || result.status === 'error') {
    failed.value = result.status === 'error'
    message.value = failed.value ? result.message : '订单详情已复制'
    clearTimeout(timer)
    timer = setTimeout(() => { message.value = '' }, 4000)
  }
}
</script>
<template>
  <span class="order-copy-control" @click.stop>
    <button ref="copyButton" type="button" class="order-copy-button" :disabled="busy || !Number(orderId)" :aria-busy="busy" title="复制已保存的订单详情文字，粘贴到生产群" @click.stop="copy">{{ busy ? '正在复制…' : '复制订单详情' }}</button>
    <Teleport to="body">
      <div v-if="message" class="order-copy-toast" :class="{ failed }" :role="failed ? 'alert' : 'status'">{{ message }}</div>
      <div v-if="manualText" class="order-copy-mask" @click.self="manualText = ''" @keydown.esc="manualText = ''">
        <section class="order-copy-dialog" role="dialog" aria-modal="true" aria-label="手动复制订单详情">
          <header><h3>复制订单详情</h3><button type="button" @click="manualText = ''">关闭</button></header>
          <p>浏览器未能自动复制，请选中下方文字后复制。</p>
          <textarea ref="textArea" readonly :value="manualText" aria-label="订单详情文字" @focus="$event.target.select()" @click="$event.target.select()" />
        </section>
      </div>
    </Teleport>
  </span>
</template>
<style scoped>
.order-copy-control{display:inline-flex;max-width:100%}.order-copy-button{background:transparent;color:#28624a;border:1px solid #cbded3;border-radius:6px;padding:5px 9px;font-size:12px;line-height:1.5;white-space:nowrap;cursor:pointer}.order-copy-button:disabled{opacity:.55;cursor:wait}.order-copy-toast{position:fixed;bottom:28px;left:50%;transform:translateX(-50%);z-index:12000;max-width:calc(100vw - 32px);background:#28624a;color:#fff;border-radius:8px;padding:12px 20px;box-sizing:border-box}.order-copy-toast.failed{background:#9c3434}.order-copy-mask{position:fixed;inset:0;z-index:12001;background:#0006;display:flex;align-items:center;justify-content:center;padding:16px}.order-copy-dialog{width:560px;max-width:100%;max-height:85vh;overflow:auto;background:#fff;border-radius:12px;padding:20px;box-sizing:border-box}.order-copy-dialog header{display:flex;align-items:center;justify-content:space-between;gap:12px}.order-copy-dialog h3{margin:0}.order-copy-dialog p{color:#52665b}.order-copy-dialog textarea{display:block;box-sizing:border-box;width:100%;height:40vh;min-height:180px;resize:vertical;padding:12px;border:1px solid #b8c9bf;border-radius:6px;line-height:1.7;color:#183b2b;font-size:14px}
</style>
