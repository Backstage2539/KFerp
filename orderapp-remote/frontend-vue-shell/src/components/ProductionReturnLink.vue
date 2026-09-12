<template>
  <button v-if="source?.key" type="button" class="production-return" @click="goBack">← {{ source.label || '返回来源操作' }}</button>
</template>
<script setup>
function goBack() {
  const source = props.source
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: source.key, params: source.params || {} } }))
}
const props = defineProps({ source: { type: Object, default: null } })
</script>
<style scoped>
.production-return{display:block;margin:0 0 14px;padding:0;border:0;background:none;color:#287bad;font:inherit;font-size:13px;cursor:pointer}
</style>
