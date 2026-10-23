<template>
  <button v-if="source?.key" type="button" class="production-return" :disabled="disabled" @click="goBack">← {{ source.label || '返回来源操作' }}</button>
</template>
<script setup>
function goBack() {
  const source = props.source
  if (props.disabled || !source?.key || props.beforeNavigate?.(source) === false) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: source.key, params: source.params || {}, viewContext: source.viewContext } }))
}
const props = defineProps({
  source: { type: Object, default: null },
  disabled: { type: Boolean, default: false },
  beforeNavigate: { type: Function, default: null },
})
</script>
<style scoped>
.production-return{display:block;justify-self:start;text-align:left;margin:0 0 14px;padding:0;border:0;background:none;color:#287bad;font:inherit;font-size:13px;cursor:pointer}
.production-return:disabled{opacity:.55;cursor:wait}
</style>
