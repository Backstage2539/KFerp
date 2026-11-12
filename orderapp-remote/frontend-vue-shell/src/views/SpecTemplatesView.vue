<template>
  <BomView :view-params="viewParams" spec-template-page />
</template>

<script setup>
import BomView from './BomView.vue'
defineProps({ viewParams: { type: Object, default: () => ({}) } })
</script>
