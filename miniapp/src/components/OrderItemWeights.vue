<script setup lang="ts">
import { computed } from 'vue'
import { orderWeightLines, type OrderWeightSnapshot } from '../utils/orderWeights'
const props = defineProps<{ item?: OrderWeightSnapshot }>()
const lines = computed(() => orderWeightLines(props.item))
</script>
<template><view class="order-item-weights"><text v-for="line in lines" :key="line.label">{{ line.label }}：{{ line.value }}</text></view></template>
<style scoped>.order-item-weights{display:flex;flex-direction:column;gap:5rpx;color:#65776c;font-size:22rpx;line-height:1.5;overflow-wrap:anywhere}</style>
