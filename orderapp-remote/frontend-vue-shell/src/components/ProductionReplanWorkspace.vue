<template>
  <section class="replan-workspace">
    <header class="replan-header">
      <button type="button" class="back" @click="$emit('back')">← 返回原生产计划</button>
      <div class="title-row">
        <div><span>撤回未开工需求</span><h1>撤回并重新安排</h1><p>{{ preview?.production_plan_no || '原生产计划' }} · 原安排在最终确认前保持有效</p></div>
        <span class="state">创建新草稿前核对</span>
      </div>
      <div class="summary">
        <article><span>原需求</span><strong>{{ originalQuantity }}</strong><small>{{ preview?.original_demands?.length || 0 }} 项商品需求</small></article>
        <article><span>新增合并</span><strong>{{ additionalQuantity }}</strong><small>{{ selectedKeys.length }} 项待计划需求</small></article>
        <article><span>新计划合计</span><strong>{{ totalQuantity }}</strong><small>新草稿可重新选择工位与批次</small></article>
      </div>
    </header>

    <div v-if="error" class="notice error">{{ error }}</div>
    <div class="body">
      <main>
        <section class="panel">
          <div class="section-head"><div><b>01</b><span><h2>原计划撤回范围</h2><p>系统会一并撤回共用的未开工上游任务，不直接抽走单个设备批次。</p></span></div></div>
          <div v-if="preview?.scope_expanded" class="scope-notice">检测到共用上游，关联商品需求已自动加入本次撤回和新草稿。</div>
          <div class="compact-table">
            <div class="table-row header"><span>商品 / 规格</span><span>数量</span><span>订单</span></div>
            <div v-for="row in preview?.original_demands || []" :key="row.production_plan_item_id" class="table-row"><strong>{{ row.product_name }}<small>{{ row.spec_label || '-' }}</small></strong><span>{{ quantity(row.quantity, row.unit, row.quantity_g) }}</span><span>{{ row.order_nos || '-' }}</span></div>
          </div>
          <details v-if="preview?.work_orders?.length" class="affected"><summary>将撤回 {{ preview.work_orders.length }} 张未开工工单</summary><p v-for="row in preview.work_orders" :key="row.id">{{ row.work_order_no }} · {{ row.workstation || '工位待确认' }} · {{ row.batch_count }} 个工序任务</p></details>
        </section>

        <section class="panel">
          <div class="section-head"><div><b>02</b><span><h2>合并新增待计划需求</h2><p>可不选择；勾选后会与原需求合计到同一张新草稿。</p></span></div><input v-model.trim="keyword" type="search" placeholder="搜索商品或订单" /></div>
          <div class="compact-table selectable">
            <div class="table-row header"><span>选择</span><span>商品 / 规格</span><span>待生产</span><span>订单</span></div>
            <label v-for="row in filteredRows" :key="selectionKey(row)" class="table-row" :class="{ selected: selectedKeys.includes(selectionKey(row)) }">
              <input type="checkbox" :checked="selectedKeys.includes(selectionKey(row))" @change="$emit('toggle', selectionKey(row), $event.target.checked)" />
              <strong>{{ row.product || row.product_name }}<small>{{ row.spec_label || row.spec || '-' }}</small></strong>
              <span>{{ demandQuantity(row) }}</span><span>{{ row.order_nos || '-' }}</span>
            </label>
            <p v-if="!filteredRows.length" class="empty">当前没有可合并的新增待计划需求</p>
          </div>
        </section>
      </main>

      <aside class="check-panel">
        <div class="check-title"><h2>提交前核对</h2><span :class="{ ready: preview?.can_replan }">{{ preview?.can_replan ? '可以重排' : '存在阻断' }}</span></div>
        <article v-for="reason in preview?.blocking_reasons || []" :key="reason" class="blocking"><b>!</b><p>{{ reason }}</p></article>
        <article v-if="preview?.can_replan" class="ready-card"><b>✓</b><p>所选任务均未开工，可原子撤回并创建新草稿。</p></article>
        <div class="wip-note"><strong>已领 WIP 怎么处理</strong><p>{{ preview?.wip_notice || '实际库存位置保持不变，新草稿会重新核对来源。' }}</p></div>
      </aside>
    </div>

    <footer>
      <div><strong>尚未执行撤回</strong><span>最终确认成功后，原工单标记“已撤回重排”，并直接打开新草稿。</span></div>
      <div><button type="button" class="secondary" :disabled="loading || saving" @click="$emit('preview')">{{ loading ? '正在核对…' : '重新核对' }}</button><button type="button" class="primary" :disabled="loading || saving || !preview?.can_replan" @click="$emit('commit')">{{ saving ? '正在创建…' : '撤回并创建新草稿' }}</button></div>
    </footer>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { summarizeReplanQuantities } from '../lib/production-replan.js'

const props = defineProps({
  preview: { type: Object, default: null },
  availableRows: { type: Array, default: () => [] },
  selectedKeys: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  saving: { type: Boolean, default: false },
  error: { type: String, default: '' },
})
defineEmits(['back', 'toggle', 'preview', 'commit'])
const keyword = ref('')
function selectionKey(row) { return String(row.selection_id || row.selection_key || '').trim() }
function quantity(value, unit, grams = 0) { if (Number(value) > 0 && unit) return `${Number(value)} ${unit}`; return `${Number(grams || 0) / 1000} kg` }
function demandQuantity(row) { return quantity(row.gap_inventory_qty || row.need_inventory_qty, row.inventory_unit, row.gap_g || row.need_g) }
const available = computed(() => props.availableRows.filter(row => row.demand_status === 'unplanned' && row.demand_selectable !== false && (Number(row.gap_g || 0) > 0 || Number(row.gap_inventory_qty || 0) > 0) && selectionKey(row)))
const filteredRows = computed(() => { const key = keyword.value.toLocaleLowerCase('zh-CN'); return key ? available.value.filter(row => [row.product, row.product_name, row.order_nos, row.spec_label].some(v => String(v || '').toLocaleLowerCase('zh-CN').includes(key))) : available.value })
const originalQuantity = computed(() => summarizeReplanQuantities(props.preview?.original_demands || []))
const additionalQuantity = computed(() => summarizeReplanQuantities(props.preview?.additional_demands || []))
const totalQuantity = computed(() => summarizeReplanQuantities([...(props.preview?.original_demands || []), ...(props.preview?.additional_demands || [])]))
</script>

<style scoped>
.replan-workspace{--ink:#162e27;--green:#238653;--soft:#eaf6ef;--line:#dde6e1;min-height:100vh;padding-bottom:84px;background:#f7f9f8;color:var(--ink)}.replan-header{padding:22px 28px 18px;background:#fff;border-bottom:1px solid var(--line)}.back{border:0;background:none;color:var(--green);font-weight:750;padding:0 0 14px;cursor:pointer}.title-row,.section-head,.section-head>div,.check-title,footer,footer>div{display:flex}.title-row{justify-content:space-between;gap:20px;align-items:start}.title-row span:first-child{font-size:12px;color:#728079}.title-row h1{margin:3px 0;font-size:27px}.title-row p,.section-head p{margin:0;color:#6c7973;font-size:13px}.state{padding:7px 11px;border-radius:999px;background:#fff0d9;color:#8b570d;font-size:12px;font-weight:800}.summary{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-top:18px}.summary article{display:grid;gap:4px;padding:14px;border:1px solid #dfe9e3;border-radius:10px;background:linear-gradient(90deg,#edf8f1,#fbfdfc)}.summary span,.summary small{font-size:12px;color:#68766f}.summary strong{font-size:19px}.body{display:grid;grid-template-columns:minmax(0,1fr) 310px;gap:20px;padding:22px 28px;max-width:1480px;margin:auto}.body main{display:grid;gap:18px}.panel,.check-panel{background:#fff;border:1px solid var(--line);border-radius:13px;padding:19px}.section-head{justify-content:space-between;gap:16px;margin-bottom:14px}.section-head>div{gap:10px}.section-head b{display:grid;place-items:center;width:31px;height:25px;border-radius:6px;background:var(--soft);color:var(--green);font-size:12px}.section-head h2{font-size:18px;margin:0 0 4px}.section-head input{width:220px;height:36px;border:1px solid #ccd7d1;border-radius:8px;padding:0 10px}.compact-table{border:1px solid var(--line);border-radius:9px;overflow:hidden}.table-row{display:grid;grid-template-columns:1.2fr .6fr 1fr;gap:12px;align-items:center;padding:11px 13px;border-top:1px solid #edf1ef;font-size:13px}.table-row.header{border:0;background:#f2f5f3;color:#617069;font-size:12px;font-weight:750}.table-row strong{display:grid;gap:3px}.table-row small{font-weight:400;color:#76827c}.selectable .table-row{grid-template-columns:46px 1.2fr .6fr 1fr}.selectable label{cursor:pointer}.selectable label.selected{background:#edf8f1}.scope-notice{margin:0 0 12px;padding:10px 12px;border:1px solid #c5dfcf;border-radius:8px;background:#edf8f1;color:#1d6849;font-size:12px}.affected{margin-top:12px;color:#53645d;font-size:12px}.affected summary{color:var(--green);font-weight:750;cursor:pointer}.affected p{margin:7px 0}.check-panel{align-self:start;position:sticky;top:18px}.check-title{justify-content:space-between;align-items:center}.check-title h2{font-size:18px}.check-title span{padding:5px 9px;border-radius:999px;background:#fff0dc;color:#945c12;font-size:12px;font-weight:800}.check-title span.ready{background:var(--soft);color:var(--green)}.blocking,.ready-card,.wip-note{margin-top:12px;padding:12px;border-radius:9px}.blocking{display:flex;gap:9px;background:#fff3ec;color:#923d21}.blocking b,.ready-card b{display:grid;place-items:center;width:22px;height:22px;border-radius:50%;background:#b45b2e;color:#fff}.blocking p,.ready-card p,.wip-note p{margin:0;font-size:12px;line-height:1.55}.ready-card{display:flex;gap:9px;background:var(--soft);color:#1d6849}.ready-card b{background:var(--green)}.wip-note{background:#f2f5f3}.wip-note p{margin-top:5px;color:#65736d}.notice{margin:14px 28px;padding:11px;border-radius:8px}.notice.error{border:1px solid #efb6a5;background:#fff2ed;color:#943d27}.empty{padding:22px;text-align:center;color:#75827c}footer{position:fixed;left:0;right:0;bottom:0;z-index:20;justify-content:space-between;align-items:center;gap:20px;padding:13px 28px;background:#fff;border-top:1px solid var(--line);box-shadow:0 -6px 20px rgba(20,50,39,.08)}footer>div{gap:10px;align-items:center}footer>div:first-child{display:grid;gap:3px}footer span{font-size:12px;color:#6c7973}button.primary,button.secondary{height:39px;padding:0 15px;border-radius:8px;font-weight:750;cursor:pointer}.primary{border:1px solid var(--green);background:var(--green);color:#fff}.secondary{border:1px solid #cbd7d1;background:#fff;color:var(--ink)}button:disabled{opacity:.45;cursor:not-allowed}@media(max-width:900px){.body{grid-template-columns:1fr}.check-panel{position:static;order:-1}.summary{grid-template-columns:1fr}.table-row,.selectable .table-row{grid-template-columns:36px 1fr}.table-row.header span:nth-child(n+3),.table-row>span:nth-child(n+3){display:none}.compact-table:not(.selectable) .table-row{grid-template-columns:1fr 1fr}.compact-table:not(.selectable) .table-row span:last-child{display:none}footer{position:sticky;flex-direction:column;align-items:stretch}.replan-workspace{padding-bottom:0}.replan-header,.body{padding-left:16px;padding-right:16px}}
.scope-notice{margin:0 0 12px;padding:10px 12px;border:1px solid #c5dfcf;border-radius:8px;background:#edf8f1;color:#1d6849;font-size:12px}
</style>
