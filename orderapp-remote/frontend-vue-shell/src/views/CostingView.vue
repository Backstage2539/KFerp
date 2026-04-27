<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>成本核算</h2>
        <div class="actions">
          <button class="secondary" type="button" :disabled="loading" @click="loadBeanList">刷新</button>
          <button class="secondary" type="button" @click="settingsOpen = true">参数设置</button>
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
              <th>商用批发梯度</th>
              <th>零售豆单价</th>
              <th>挂耳/袋</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in items" :key="item.product_id || item.name">
              <td class="name">{{ item.name }}</td>
              <td>{{ costMoney(item.green_bean_cost_per_kg) }}</td>
              <td>{{ costMoney(item.small_batch_cost_per_kg) }}</td>
              <td class="tiers-cell">
                <div class="tier-list">
                  <span v-for="tier in item.commercial_wholesale_tiers || []" :key="tier.label" class="tier-chip">
                    <b>{{ tier.label }}</b>{{ price(tierPriceValue(tier)) }}/{{ tierUnit(tier) }}
                  </span>
                </div>
              </td>
              <td class="tiers-cell">
                <div class="tier-list">
                  <span v-for="tier in item.retail_bean_tiers || []" :key="tier.label" class="tier-chip">
                    <b>{{ tier.label }}</b>{{ price(tier.price_per_unit) }}
                  </span>
                </div>
              </td>
              <td>{{ price(first(item.wholesale_drip_bag_prices)) }}</td>
            </tr>
            <tr v-if="!loading && !items.length">
              <td colspan="6" class="muted empty">暂无可试算商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">商用批发豆单</div>
      <div class="bean-groups">
        <section v-for="group in commercialGroups" :key="group.category" class="bean-group">
          <h3>{{ group.category }}</h3>
          <div class="bean-grid">
            <article v-for="item in group.items" :key="item.product_id || item.name">
              <div class="bean-heading">
                <span class="bean-code">{{ beanMeta(item, 'commercial_bean_list').code }}</span>
                <div>
                  <div class="bean-title">{{ beanName(item, 'commercial_bean_list') }}</div>
                  <div v-if="beanMeta(item, 'commercial_bean_list').recommended_use" class="bean-use">
                    {{ beanMeta(item, 'commercial_bean_list').recommended_use }}
                  </div>
                </div>
              </div>
              <div v-if="beanFlavor(item, 'commercial_bean_list')" class="bean-note">{{ beanFlavor(item, 'commercial_bean_list') }}</div>
              <div v-if="beanDescription(item, 'commercial_bean_list')" class="bean-desc">{{ beanDescription(item, 'commercial_bean_list') }}</div>
              <div class="bean-row" v-for="tier in item.commercial_wholesale_tiers || []" :key="tier.label">
                <span>{{ tier.label }}</span><strong>{{ price(tierPriceValue(tier)) }}/{{ tierUnit(tier) }}</strong>
              </div>
            </article>
          </div>
        </section>
        <div v-if="!commercialGroups.length" class="muted empty-card">暂无豆单数据</div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">零售豆单</div>
      <div class="bean-groups">
        <section v-for="group in retailGroups" :key="group.category" class="bean-group">
          <h3>{{ group.category }}</h3>
          <div class="bean-grid">
            <article v-for="item in group.items" :key="`retail-${item.product_id || item.name}`">
              <div class="bean-heading">
                <span class="bean-code">{{ beanMeta(item, 'retail_bean_list').code }}</span>
                <div>
                  <div class="bean-title">{{ beanName(item, 'retail_bean_list') }}</div>
                  <div v-if="beanMeta(item, 'retail_bean_list').recommended_use" class="bean-use">
                    {{ beanMeta(item, 'retail_bean_list').recommended_use }}
                  </div>
                </div>
              </div>
              <div v-if="beanFlavor(item, 'retail_bean_list')" class="bean-note">{{ beanFlavor(item, 'retail_bean_list') }}</div>
              <div v-if="beanDescription(item, 'retail_bean_list')" class="bean-desc">{{ beanDescription(item, 'retail_bean_list') }}</div>
              <div class="bean-row" v-for="tier in item.retail_bean_tiers || []" :key="`retail-tier-${tier.label}`">
                <span>{{ tier.label }}</span><strong>{{ price(tier.price_per_unit) }}</strong>
              </div>
              <div class="bean-row"><span>挂耳10袋</span><strong>{{ price(item.retail_drip_10_bag_price) }}</strong></div>
            </article>
          </div>
        </section>
        <div v-if="!retailGroups.length" class="muted empty-card">暂无豆单数据</div>
      </div>
    </section>

    <div v-if="settingsOpen" class="drawer-backdrop" @click.self="settingsOpen = false">
      <aside class="settings-drawer" aria-label="快速成本参数设置">
        <div class="drawer-head">
          <div>
            <h3>快速成本参数设置</h3>
            <p>保存单个参数后，当前成本试算会自动刷新。</p>
          </div>
          <button class="secondary" type="button" @click="settingsOpen = false">关闭</button>
        </div>
        <CostingSettingsPanel compact :show-header="false" @saved="handleSettingSaved" />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import CostingSettingsPanel from '../components/CostingSettingsPanel.vue'

const loading = ref(false)
const saving = ref(false)
const publishing = ref(false)
const settingsOpen = ref(false)
const error = ref('')
const message = ref('')
const parameters = ref(null)
const items = ref([])
const runId = ref(null)

const commercialGroups = computed(() => groupBeanItems('commercial_bean_list'))
const retailGroups = computed(() => groupBeanItems('retail_bean_list'))

function first(values) {
  return Array.isArray(values) && values.length ? Number(values[0] || 0) : 0
}

function tierPriceValue(tier) {
  return Number(tier?.price_per_unit || tier?.price_per_lb || 0)
}

function tierUnit(tier) {
  const specG = Number(tier?.spec_g || 454)
  if (specG === 1000) return 'kg'
  if (specG === 227) return '227g'
  return '包'
}

function beanMeta(item, key) {
  return item?.[key] || {}
}

function beanName(item, key) {
  return beanMeta(item, key).display_name || item?.name || ''
}

function beanFlavor(item, key) {
  return beanMeta(item, key).flavor || item?.flavor || ''
}

function beanDescription(item, key) {
  return beanMeta(item, key).description || item?.bean_list_note || ''
}

function groupBeanItems(key) {
  const groups = new Map()
  items.value
    .filter((item) => beanMeta(item, key).code)
    .slice()
    .sort((a, b) => compareBeanCodes(beanMeta(a, key).code, beanMeta(b, key).code))
    .forEach((item) => {
      const meta = beanMeta(item, key)
      const category = meta.category || '未分类'
      if (!groups.has(category)) {
        groups.set(category, { category, items: [] })
      }
      groups.get(category).items.push(item)
    })
  return Array.from(groups.values())
}

function compareBeanCodes(a, b) {
  const aa = String(a || '').split('.').map((v) => Number(v) || 0)
  const bb = String(b || '').split('.').map((v) => Number(v) || 0)
  const len = Math.max(aa.length, bb.length)
  for (let i = 0; i < len; i += 1) {
    if ((aa[i] || 0) !== (bb[i] || 0)) return (aa[i] || 0) - (bb[i] || 0)
  }
  return String(a || '').localeCompare(String(b || ''))
}

function fixed(value, digits = 2) {
  return Number(value || 0).toFixed(digits)
}

function money(value) {
  return fixed(value, 2)
}

function price(value) {
  return fixed(value, 0)
}

function costMoney(value) {
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

async function handleSettingSaved() {
  await loadBeanList()
  message.value = '成本参数已保存，当前试算已刷新'
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
.tiers-cell { min-width: 360px; text-align: left; white-space: normal; }
.tier-list { display: flex; flex-wrap: wrap; gap: 6px; justify-content: flex-start; }
.tier-chip { border: 1px solid #ddd; border-radius: 8px; background: #fff; padding: 5px 7px; color: #222; font-size: 12px; line-height: 1.2; }
.tier-chip b { margin-right: 5px; font-weight: 700; color: #111; }
.empty { text-align: center !important; padding: 18px; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; font: inherit; }
button:disabled { opacity: .45; cursor: not-allowed; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.danger { border: 1px solid #8b1e1e; background: #8b1e1e; color: #fff; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.drawer-backdrop { position: fixed; inset: 0; z-index: 80; background: rgba(0,0,0,.25); display: flex; justify-content: flex-end; }
.settings-drawer { width: min(620px, 100vw); height: 100vh; overflow: auto; background: #f7f7f7; border-left: 1px solid #d9d9d9; padding: 14px; box-shadow: -18px 0 36px rgba(0,0,0,.18); }
.drawer-head { position: sticky; top: 0; z-index: 2; background: #f7f7f7; display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; padding-bottom: 12px; margin-bottom: 4px; }
.drawer-head h3 { margin: 0; font-size: 18px; }
.drawer-head p { margin: 4px 0 0; color: #666; font-size: 12px; line-height: 1.45; }
.bean-groups { display: grid; gap: 14px; margin-top: 10px; }
.bean-group { display: grid; gap: 8px; }
.bean-group h3 { margin: 0; padding: 8px 10px; border-left: 4px solid #111; background: #f5f5f5; font-size: 15px; }
.bean-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
article, .empty-card { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fafafa; }
.bean-heading { display: grid; grid-template-columns: auto 1fr; gap: 8px; align-items: start; min-height: 52px; margin-bottom: 8px; }
.bean-code { display: inline-flex; align-items: center; justify-content: center; min-width: 34px; height: 28px; border: 1px solid #ddd; border-radius: 8px; background: #fff; font-size: 12px; font-weight: 700; color: #333; }
.bean-title { font-weight: 700; line-height: 1.25; }
.bean-use { color: #111; font-size: 12px; line-height: 1.35; margin-top: 3px; white-space: pre-line; }
.bean-note { color: #555; font-size: 12px; min-height: 18px; margin: 0 0 8px; line-height: 1.45; }
.bean-desc { color: #777; font-size: 12px; line-height: 1.45; margin: 0 0 8px; }
.bean-row { display: flex; justify-content: space-between; align-items: center; gap: 8px; border-top: 1px solid #eee; padding: 7px 0; }
.bean-row span { color: #666; font-size: 12px; }
.bean-row strong { font-size: 13px; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: flex-start; flex-direction: column; }
  .actions { justify-content: flex-start; }
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .bean-grid { grid-template-columns: 1fr; }
  .settings-drawer { width: 100vw; }
}
</style>
