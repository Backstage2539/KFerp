<template>
  <div class="page">
    <section class="panel">
      <div class="eyebrow">MIGRATION</div>
      <h2>{{ title }}</h2>
      <p>这个页面还没有迁入 Vue/Vite。当前架构不再把旧 HTML 模板嵌入 Vue shell。</p>
      <a v-if="legacyUrl" class="legacy-link" :href="legacyUrl">打开旧页面</a>
    </section>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, default: '页面迁移中' },
  legacyUrl: { type: String, default: '' },
})
</script>

<style scoped>
.page { padding: 18px; color: #151515; }
.panel {
  border: 1px solid #e6e0d8;
  border-radius: 8px;
  background: #fff;
  padding: 18px;
  max-width: 720px;
}
.eyebrow { font-size: 12px; font-weight: 700; color: #666; margin-bottom: 8px; }
h2 { margin: 0 0 10px; font-size: 20px; }
p { margin: 0 0 14px; color: #555; line-height: 1.55; }
.legacy-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding: 0 12px;
  border: 1px solid #222;
  border-radius: 6px;
  color: #fff;
  background: #222;
  text-decoration: none;
}
</style>

