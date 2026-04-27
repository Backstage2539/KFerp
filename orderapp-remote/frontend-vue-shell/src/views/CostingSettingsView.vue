<template>
  <div class="page">
    <CostingSettingsPanel />
  </div>
</template>

<script setup>
import CostingSettingsPanel from '../components/CostingSettingsPanel.vue'
</script>

<style scoped>
.page { padding: 16px; }

@media (max-width: 900px) {
  .page { padding: 12px; }
}
</style>
