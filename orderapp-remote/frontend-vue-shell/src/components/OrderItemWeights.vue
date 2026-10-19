<script setup>
import { computed } from 'vue'
import { orderWeightLines } from '../lib/logistics-weight.js'
const props = defineProps({ item: { type: Object, default: () => ({}) } })
const lines = computed(() => orderWeightLines(props.item))
</script>
<template><div class="order-item-weights"><span v-for="line in lines" :key="line.label">{{ line.label }}：{{ line.value }}</span></div></template>
<style scoped>.order-item-weights{display:grid!important;gap:3px!important;font-size:12px;color:#627166;line-height:1.5;min-width:0;overflow-wrap:anywhere}.order-item-weights span{font-size:inherit;color:inherit}</style>
