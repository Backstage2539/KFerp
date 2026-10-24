<template><ProductionRecordsView :embedded="embedded" :view-params="viewParams" /></template>
<script setup>
import ProductionRecordsView from './ProductionRecordsView.vue'
defineProps({embedded:Boolean,viewParams:{type:Object,default:()=>({})}})
</script>
