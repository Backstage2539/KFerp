<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>成本核算</h2>
        <div class="actions">
          <button class="secondary" type="button" :disabled="loading" @click="loadBeanList">刷新</button>
          <button class="primary" type="button" :disabled="saving || loading || !items.length" @click="createRun">保存试算</button>
          <button class="danger" type="button" :disabled="publishing || !runId" @click="publishRun">发布价格</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div class="metrics">
        <div>
          <span>商品数</span>
          <strong>{{ items.length }}</strong>
        </div>
        <div>
          <span>烘焙率</span>
          <strong>{{ percent(parameters?.roast_yield_rate) }}</strong>
        </div>
        <div>
          <span>kg/lb</span>
          <strong>{{ fixed(parameters?.kg_to_lb_factor, 3) }}</strong>
        </div>
        <div>
          <span>试算批次</span>
          <strong>{{ runId || '-' }}</strong>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">价格试算</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>生豆/kg</th>
              <th>熟豆成本/kg</th>
              <th>2-13磅/lb</th>
              <th>14-23磅/lb</th>
              <th>24-47磅/lb</th>
              <th>大于47磅/lb</th>
              <th>零售227g</th>
              <th>零售250g</th>
              <th>挂耳/袋</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in items" :key="item.product_id || item.name">
              <td class="name">{{ item.name }}</td>
              <td>{{ money(item.green_bean_cost_per_kg) }}</td>
              <td>{{ money(item.small_batch_cost_per_kg) }}</td>
              <td>{{ money(tierPrice(item, 0)) }}</td>
              <td>{{ money(tierPrice(item, 1)) }}</td>
              <td>{{ money(tierPrice(item, 2)) }}</td>
              <td>{{ money(tierPrice(item, 3)) }}</td>
              <td>{{ money(item.retail_227g_price) }}</td>
              <td>{{ money(item.retail_250g_price) }}</td>
              <td>{{ money(first(item.wholesale_drip_bag_prices)) }}</td>
            </tr>
            <tr v-if="!loading && !items.length">
              <td colspan="10" class="muted empty">暂无可试算商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">商用批发豆单</div>
      <div class="bean-grid">
        <article v-for="item in beanPreview" :key="item.product_id || item.name">
          <div class="bean-title">{{ item.name }}</div>
          <div v-if="item.flavor" class="bean-note">{{ item.flavor }}</div>
          <div class="bean-row" v-for="tier in item.commercial_wholesale_tiers || []" :key="tier.label">
            <span>{{ tier.label }}</span><strong>{{ money(tier.price_per_lb) }}/lb</strong>
          </div>
        </article>
        <div v-if="!beanPreview.length" class="muted empty-card">暂无豆单数据</div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">零售豆单</div>
      <div class="bean-grid">
        <article v-for="item in beanPreview" :key="`retail-${item.product_id || item.name}`">
          <div class="bean-title">{{ item.name }}</div>
          <div v-if="item.origin || item.process_method" class="bean-note">{{ compact([item.origin, item.process_method]) }}</div>
          <div v-if="item.flavor" class="bean-note">{{ item.flavor }}</div>
          <div class="bean-row"><span>零售</span><strong>{{ money(item.retail_227g_price) }}/227g</strong></div>
          <div class="bean-row"><span>250g</span><strong>{{ money(item.retail_250g_price) }}</strong></div>
          <div class="bean-row"><span>挂耳10袋</span><strong>{{ money(item.retail_drip_10_bag_price) }}</strong></div>
        </article>
        <div v-if="!beanPreview.length" class="muted empty-card">暂无豆单数据</div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const loading = ref(false)
const saving = ref(false)
const publishing = ref(false)
const error = ref('')
const message = ref('')
const parameters = ref(null)
const items = ref([])
const runId = ref(null)

const beanPreview = computed(() => items.value.slice(0, 24))

function first(values) {
  return Array.isArray(values) && values.length ? Number(values[0] || 0) : 0
}

function tierPrice(item, index) {
  const tier = item?.commercial_wholesale_tiers?.[index]
  if (tier) return Number(tier.price_per_lb || 0)
  return Array.isArray(item?.wholesale_lb_prices) ? Number(item.wholesale_lb_prices[index] || 0) : 0
}

function compact(values) {
  return values.filter(Boolean).join(' / ')
}

function fixed(value, digits = 2) {
  return Number(value || 0).toFixed(digits)
}

function money(value) {
  return fixed(value, 2)
}

function percent(value) {
  return `${fixed(Number(value || 0) * 100, 1)}%`
}

async function loadBeanList() {
  loading.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiGet('/api/costing/bean-list')
    parameters.value = data.parameters
    items.value = Array.isArray(data.items) ? data.items : []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function createRun() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend('/api/costing/runs')
    runId.value = data.id
    if (Array.isArray(data.items)) items.value = data.items
    message.value = `已保存试算批次 ${data.id}`
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function publishRun() {
  if (!runId.value) return
  publishing.value = true
  error.value = ''
  message.value = ''
  try {
    await apiSend(`/api/costing/runs/${runId.value}/publish`)
    message.value = `已发布试算批次 ${runId.value}`
  } catch (err) {
    error.value = err.message || '发布失败'
  } finally {
    publishing.value = false
  }
}

onMounted(loadBeanList)
</script>

<style scoped>
.page { padding: 18px; color: #171717; display: grid; gap: 16px; }
.panel { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fff; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.metrics > div { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fafafa; }
.metrics span, .muted { color: #666; font-size: 12px; }
.metrics span { display: block; margin-bottom: 6px; }
.metrics strong { font-size: 18px; }
.table-wrap { overflow: auto; margin-top: 10px; }
table { width: 100%; border-collapse: collapse; min-width: 1100px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 9px 10px; text-align: right; white-space: nowrap; }
th:first-child, td:first-child { text-align: left; }
th { color: #555; background: #fafafa; font-weight: 700; }
.name { font-weight: 650; }
.empty { text-align: center !important; padding: 18px; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; font: inherit; }
button:disabled { opacity: .45; cursor: not-allowed; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.danger { border: 1px solid #8b1e1e; background: #8b1e1e; color: #fff; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.bean-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; margin-top: 10px; }
article, .empty-card { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fafafa; }
.bean-title { font-weight: 700; min-height: 38px; margin-bottom: 8px; }
.bean-note { color: #555; font-size: 12px; min-height: 18px; margin: 0 0 8px; line-height: 1.45; }
.bean-row { display: flex; justify-content: space-between; align-items: center; gap: 8px; border-top: 1px solid #eee; padding: 7px 0; }
.bean-row span { color: #666; font-size: 12px; }
.bean-row strong { font-size: 13px; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: flex-start; flex-direction: column; }
  .actions { justify-content: flex-start; }
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .bean-grid { grid-template-columns: 1fr; }
}
</style>
