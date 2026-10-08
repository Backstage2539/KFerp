<script setup>
defineProps({
  data: { type: Object, required: true },
  images: { type: Object, default: () => ({}) },
});
</script>
<template>
  <article class="page-content">
    <h1>{{ data.document.name }}</h1>
    <template v-if="data.document.kind === 'article'">
      <template
        v-for="(block, index) in data.document.blocks || []"
        :key="index"
      >
        <h2 v-if="block.kind === 'heading'">{{ block.text }}</h2>
        <p v-else-if="block.kind === 'text'" class="paragraph">
          {{ block.text }}
        </p>
        <figure v-else-if="block.kind === 'image'">
          <img
            v-if="images[block.asset_id]"
            :src="images[block.asset_id]"
            :alt="block.caption || '页面图片'"
          />
          <figcaption>{{ block.caption }}</figcaption>
        </figure>
      </template>
    </template>
    <template v-else-if="data.document.kind === 'price' && data.publication">
      <img
        v-if="data.publication.logo_image"
        class="logo"
        :src="data.publication.logo_image"
        alt="品牌标志"
      />
      <h2>{{ data.publication.title }}</h2>
      <p>{{ data.publication.subtitle }}</p>
      <p class="version">
        {{ data.publication.version_no }} · {{ data.publication.published_at }}
      </p>
      <p class="paragraph">{{ data.publication.brand_intro }}</p>
      <section v-for="(group, gi) in data.publication.groups || []" :key="gi">
        <h2>{{ group.category }}</h2>
        <div v-for="(item, i) in group.items" :key="i" class="product">
          <h3>
            {{ item.name }} <small>{{ item.badge_label }}</small>
          </h3>
          <p v-if="item.code">{{ item.code }}</p>
          <p v-if="item.flavor">风味：{{ item.flavor }}</p>
          <p v-if="item.recommended_use">
            出品建议：{{ item.recommended_use }}
          </p>
          <p v-if="item.description">{{ item.description }}</p>
          <div v-for="(price, j) in item.prices || []" :key="j" class="price">
            <span>{{ price.label }}</span
            ><strong :class="{ red: price.red }">{{ price.value }}</strong>
          </div>
        </div>
      </section>
    </template>
    <p v-else-if="data.document.kind === 'function'">
      此入口将在小程序打开对应功能，权限沿用该功能。
    </p>
    <footer>棵凡咖啡</footer>
  </article>
</template>
<style scoped>
.logo {
  max-width: 160px;
  max-height: 100px;
  object-fit: contain;
  margin: auto;
}
.page-content {
  max-width: 680px;
  margin: auto;
  padding: 24px;
  background: #fff;
  color: #233c30;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.page-content h1 {
  font-size: 26px;
  line-height: 1.4;
}
.page-content h2 {
  font-size: 20px;
}
.paragraph {
  white-space: pre-wrap;
}
.version,
figcaption,
footer {
  font-size: 13px;
  color: #68786c;
}
figure {
  margin: 20px 0;
}
img {
  width: 100%;
  height: auto;
  display: block;
  border-radius: 8px;
}
.product {
  border: 1px solid #dce4df;
  border-radius: 12px;
  padding: 16px;
  margin: 16px 0;
}
.product h3 {
  margin: 0;
}
.product p {
  font-size: 14px;
}
.price {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  border-top: 1px solid #edf0ed;
  padding: 9px 0;
  white-space: pre-wrap;
}
.red,
small {
  color: #a03931;
}
footer {
  text-align: center;
  margin-top: 36px;
}
</style>
