<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>{{ printMode ? '报价导出' : '商品档案' }}</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>烘焙度</th>
              <th>默认价</th>
              <th>100g</th>
              <th>200g</th>
              <th>227g</th>
              <th>250g</th>
              <th>阶梯价</th>
              <th v-if="!printMode">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.name }}</td>
              <td>{{ row.roast_level }}</td>
              <td>{{ money(row.default_price) }}</td>
              <td>{{ money(row.retail_price_100g) }}</td>
              <td>{{ money(row.retail_price_200g) }}</td>
              <td>{{ money(row.retail_price_227g) }}</td>
              <td>{{ money(row.retail_price_250g) }}</td>
              <td class="tiers">{{ tierText(row.tiers) }}</td>
              <td v-if="!printMode"><a :href="`/products/${row.id}?legacy=1`">编辑价格</a></td>
            </tr>
            <tr v-if="!rows.length">
              <td :colspan="printMode ? 8 : 9" class="muted">暂无商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'

const props = defineProps({
  viewKey: { type: String, default: 'products' },
})

const rows = ref([])
const loading = ref(false)
const error = ref('')
const printMode = computed(() => props.viewKey === 'quotePrint')

function money(value) {
  const n = Number(value || 0)
  return n > 0 ? n.toFixed(2) : ''
}

function tierText(tiers) {
  return (tiers || []).map((tier) => {
    const max = tier.max_qty ? `-${tier.max_qty}` : '+'
    return `${tier.spec_g}g ${tier.min_qty}${max}: ${money(tier.unit_price)}`
  }).join('\n')
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch('/api/products')
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
h2 { margin: 0; font-size: 20px; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
.secondary { background: #fff; color: #1f1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1040px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
a { color: #1f4f82; text-decoration: none; }
.tiers { white-space: pre-wrap; min-width: 260px; }
.muted { color: #666; text-align: center; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-top: 12px; color: #8a1f1f; }
@media (max-width: 900px) { .page { padding: 12px; } table { min-width: 900px; } }
</style>
