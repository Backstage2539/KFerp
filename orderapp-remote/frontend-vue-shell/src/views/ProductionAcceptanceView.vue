<template><WorkOrdersView :embedded="embedded" :view-params="viewParams" /></template>
<script setup>
import WorkOrdersView from './WorkOrdersView.vue'
defineProps({ embedded: Boolean, viewParams: { type: Object, default: () => ({}) } })
</script>
