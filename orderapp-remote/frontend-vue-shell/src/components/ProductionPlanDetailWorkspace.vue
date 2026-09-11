<template>
  <section class="production-plan-workspace" aria-label="生产计划详情工作区">
    <header class="workspace-header">
      <button class="back-button" type="button" @click="$emit('back')"><span aria-hidden="true">←</span> 返回生产计划</button>
      <div class="header-main">
        <div>
          <div class="eyebrow">生产计划</div>
          <div class="title-row">
            <h1>{{ detail.plan_no || '-' }}</h1>
            <span :class="['status-pill', `status-${statusTone}`]">{{ statusLabel }}</span>
            <span v-if="dirty" class="unsaved-pill">有未保存修改</span>
          </div>
          <p>创建于 {{ detail.created_at || '-' }} · {{ detail.created_by || '未记录创建人' }}</p>
        </div>
        <div v-if="isDraft" class="header-actions">
          <button class="secondary" type="button" @click="$emit('refresh')">刷新供应</button>
          <button class="secondary" type="button" @click="$emit('edit-splits')">查看工序拆分</button>
        </div>
      </div>
      <div class="summary-grid">
        <article>
          <span>计划产品</span>
          <strong>{{ productSummary }}</strong>
          <small>{{ productKinds }} 个品种</small>
        </article>
        <article>
          <span>生产任务</span>
          <strong>{{ detail.items?.length || 0 }} 项</strong>
          <small>{{ stages.length }} 个连续阶段</small>
        </article>
        <article>
          <span>关联订单</span>
          <strong>{{ orderNos.length }} 张</strong>
          <small>{{ orderNos.slice(0, 2).join('、') || '备货计划' }}</small>
        </article>
        <article>
          <span>提交状态</span>
          <strong>{{ readinessText }}</strong>
          <small>{{ detail.readiness?.blocking_count || 0 }} 项需要处理</small>
        </article>
      </div>
    </header>

    <div v-if="loading" class="workspace-loading">正在读取生产计划详情…</div>
    <div v-else-if="error" class="workspace-error">{{ error }}</div>
    <div v-else class="workspace-body">
      <main class="workspace-main">
        <section class="content-section schedule-section">
          <div class="section-heading">
            <div>
              <span class="section-index">01</span>
              <div><h2>本次生产安排</h2><p>按物料流向从上游到成品排列，每张任务卡保留自己的订单、客户、规格和冻结版本。</p></div>
            </div>
            <button v-if="isDraft" class="text-button" type="button" @click="$emit('edit-splits')">编辑工位与批次</button>
          </div>

          <div v-if="stages.length" class="stage-list">
            <article v-for="(stage, stageIndex) in stages" :key="stage.key" class="stage-card">
              <div class="stage-rail"><span>{{ stageIndex + 1 }}</span><i v-if="stageIndex < stages.length - 1" /></div>
              <div class="stage-content">
                <div class="stage-head"><span>阶段 {{ stageIndex + 1 }}</span><h3>{{ stage.title }}</h3><small>{{ stage.tasks.length }} 项任务</small></div>
                <div class="task-grid">
                  <article v-for="task in stage.tasks" :id="`plan-item-${task.id}`" :key="task.id" class="task-card">
                    <div class="task-top">
                      <div><span class="task-kind">{{ task.kind_label }}</span><h4>{{ task.item.output_name || task.item.product_name || '-' }}</h4></div>
                      <strong>{{ task.quantity_label }}</strong>
                    </div>
                    <div class="task-meta">
                      <span>{{ task.item.spec_label || quantitySummary(task.item) }}</span>
                      <span>{{ bomSourceLabel(task.item) }}</span>
                    </div>
                    <label class="warehouse-field">
                      <span>完工入库</span>
                      <select v-if="isDraft" :value="task.item.target_warehouse" @change="$emit('warehouse-change', task.id, $event.target.value)">
                        <option value="">请选择目标仓库</option>
                        <option v-for="warehouse in availableWarehouses(task.item)" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}（{{ warehouse.code }}）</option>
                      </select>
                      <strong v-else>{{ warehouseLabel(task.item.target_warehouse) }}</strong>
                    </label>
                    <div v-if="task.order_nos.length" class="trace-row"><span>订单</span><div><b v-for="orderNo in task.order_nos" :key="orderNo">{{ orderNo }}</b></div></div>
                    <div v-if="task.dependencies.length" class="dependency-row"><span>等待</span><div v-for="link in task.dependencies" :key="link.item.id">{{ link.item.output_name || link.item.product_name }} {{ link.quantity_label }}</div></div>
                    <div v-if="task.supplies.length" class="dependency-row supplies"><span>供应</span><div v-for="link in task.supplies" :key="link.item.id">{{ link.item.output_name || link.item.product_name }} {{ link.quantity_label }}</div></div>
                  </article>
                </div>
              </div>
            </article>
          </div>
          <div v-else class="empty-card">暂无生产任务</div>
        </section>

        <section id="material-sources" class="content-section materials-section">
          <div class="section-heading">
            <div>
              <span class="section-index">02</span>
              <div><h2>用料与来源</h2><p>数量来自上方冻结任务；同一仓库与货主的需求会在提交时合并核对可用量。</p></div>
            </div>
          </div>
          <div class="material-summary">
            <article v-for="material in detail.material_summary || []" :key="`${material.name}-${material.unit}-${material.component_type}`">
              <span>{{ materialTypeLabel(material) }}</span>
              <strong>{{ material.name }}</strong>
              <b>{{ quantity(material.quantity, material.unit) }}</b>
            </article>
          </div>
          <div v-if="detail.component_sources?.length" class="source-list">
            <article v-for="source in detail.component_sources" :id="`component-source-${source.id}`" :key="sourceIdentity(source)" :class="['source-card', { invalid: !source.selected || sourceShort(source) }]">
              <div class="source-identity">
                <span>{{ source.component_type === 'packaging' ? '包材' : source.component_type === 'product' ? '半成品' : '原料' }}</span>
                <div><h3>{{ source.component_name }}</h3><p>{{ planItemName(source.production_plan_item_id) }}</p></div>
              </div>
              <div class="source-need"><span>本项需求</span><strong>{{ sourceRequired(source) }}</strong></div>
              <label>
                <span>来源仓库与货主</span>
                <select v-if="isDraft" :value="sourceOptionKey(source)" @change="$emit('source-change', source, $event.target.value)">
                  <option value="">请选择来源</option>
                  <option v-for="option in source.options || []" :key="optionKey(option)" :value="optionKey(option)">{{ optionLabel(option, source) }}</option>
                </select>
                <strong v-else>{{ selectedSourceLabel(source) }}</strong>
              </label>
              <div :class="['source-state', { short: sourceShort(source) }]">
                <span>{{ source.selected ? '可用量' : '状态' }}</span>
                <strong>{{ source.selected ? sourceAvailable(source) : '待选择' }}</strong>
              </div>
            </article>
          </div>
          <div v-else class="empty-card">本计划没有需要从库存领用的组件</div>
        </section>

        <section class="content-section detail-section">
          <div class="section-heading compact"><div><span class="section-index">03</span><div><h2>单据追溯</h2><p>查看冻结配置、订单来源、供应关系和提交后生成的工单。</p></div></div></div>
          <details open>
            <summary>冻结 BOM 与工艺路线</summary>
            <div class="frozen-grid">
              <article v-for="item in detail.items || []" :key="`frozen-${item.id}`">
                <strong>{{ item.output_name || item.product_name }}</strong>
                <span>{{ bomSourceLabel(item) }}</span>
                <span>{{ processRouteLabel(item) }}</span>
              </article>
            </div>
          </details>
          <details>
            <summary>订单与客户追溯</summary>
            <div class="trace-table">
              <article v-for="item in detail.items || []" :key="`trace-${item.id}`"><strong>{{ item.output_name || item.product_name }}</strong><span>{{ item.order_nos || '备货计划' }}</span><span>{{ item.customer_name || (item.customer_id ? `客户 #${item.customer_id}` : '工厂') }}</span></article>
            </div>
          </details>
          <details>
            <summary>供应依赖</summary>
            <div class="trace-table">
              <article v-for="edge in detail.manufacturing_plan?.edges || []" :key="`${edge.consumer_plan_item_id}-${edge.supplier_plan_item_id}`"><strong>{{ planItemName(edge.supplier_plan_item_id) }}</strong><span>供应 {{ planItemName(edge.consumer_plan_item_id) }}</span><span>{{ edgeRequired(edge) }}</span></article>
              <p v-if="!detail.manufacturing_plan?.edges?.length" class="muted">没有计划内上下游依赖</p>
            </div>
          </details>
          <details>
            <summary>已生成工单（{{ detail.related_work_orders?.length || 0 }}）</summary>
            <div class="work-order-list">
              <article v-for="workOrder in detail.related_work_orders || []" :key="workOrder.id">
                <div><strong>{{ workOrder.work_order_no }}</strong><span>{{ workOrder.output_name || workOrder.product_name }}</span></div>
                <span>{{ workOrderStatus(workOrder.status) }}</span>
                <button type="button" @click="$emit('navigate', 'workOrders', { work_order_id: workOrder.id })">打开工单</button>
              </article>
              <p v-if="!detail.related_work_orders?.length" class="muted">提交计划后，生成的工单会出现在这里。</p>
            </div>
          </details>
        </section>
      </main>

      <aside class="readiness-panel">
        <div class="readiness-title">
          <div><span class="section-index">核对</span><h2>提交前核对</h2></div>
          <span :class="['readiness-count', { ready: detail.readiness?.can_submit }]">{{ detail.readiness?.can_submit ? '已通过' : `${detail.readiness?.blocking_count || 0} 项待处理` }}</span>
        </div>
        <p>{{ detail.readiness?.can_submit ? '仓库、用料和工序安排均已核对，可提交生成工单。' : '请完成下面的必填项。保存草稿后系统会重新计算。' }}</p>
        <div v-if="detail.readiness?.issues?.length" class="issue-list">
          <article v-for="issue in detail.readiness.issues" :key="`${issue.code}-${issue.production_plan_item_id}-${issue.component_source_id}`">
            <span class="issue-icon" aria-hidden="true">!</span>
            <div><strong>{{ issueTitle(issue.category) }}</strong><p>{{ issue.message }}</p><button v-if="issue.action" type="button" @click="focusIssue(issue)">{{ issue.action }} →</button></div>
          </article>
        </div>
        <div v-else class="ready-card"><span aria-hidden="true">✓</span><strong>没有阻断项</strong><p>当前内容已满足提交条件。</p></div>
        <div class="readiness-note"><strong>数量口径</strong><p>商品按冻结销售规格计数，用料按 BOM 净需求加一次损耗。页面汇总、来源核对和工单使用同一份数量。</p></div>
      </aside>
    </div>

    <footer class="workspace-footer">
      <div>
        <strong>{{ footerTitle }}</strong>
        <span>{{ footerHint }}</span>
      </div>
      <div v-if="isDraft" class="footer-actions">
        <button class="danger-link" type="button" @click="$emit('cancel')">撤销草稿</button>
        <button class="secondary" type="button" :disabled="saving || !dirty" @click="$emit('save')">{{ saving ? '正在保存…' : '保存草稿' }}</button>
        <button class="primary" type="button" :disabled="saving || dirty || !detail.readiness?.can_submit" @click="$emit('submit')">提交生成工单</button>
      </div>
      <button v-else-if="detail.related_work_orders?.length" class="primary" type="button" @click="$emit('navigate', 'workOrders', { production_plan_id: detail.id })">查看生成的工单</button>
    </footer>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import {
  buildProductionPlanStages,
  productionPlanItemBomSourceLabel,
  productionPlanItemQuantitySummary,
  productionPlanStatusLabel,
  productionPlanStatusTone,
} from '../lib/produce-plan'

const props = defineProps({
  detail: { type: Object, required: true },
  warehouses: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  saving: { type: Boolean, default: false },
  dirty: { type: Boolean, default: false },
})
const emit = defineEmits(['back', 'save', 'submit', 'edit-splits', 'refresh', 'cancel', 'navigate', 'source-change', 'warehouse-change'])

const stages = computed(() => buildProductionPlanStages(props.detail))
const isDraft = computed(() => String(props.detail?.status || '') === 'draft')
const statusLabel = computed(() => productionPlanStatusLabel(props.detail?.status))
const statusTone = computed(() => productionPlanStatusTone(props.detail?.status))
const productNames = computed(() => [...new Set((props.detail?.items || []).filter((item) => String(item.output_type || 'product') !== 'material').map((item) => item.output_name || item.product_name).filter(Boolean))])
const productSummary = computed(() => productNames.value.slice(0, 2).join('、') || '备货计划')
const productKinds = computed(() => productNames.value.length)
const orderNos = computed(() => [...new Set((props.detail?.items || []).flatMap((item) => String(item.order_nos || '').split(',').map((value) => value.trim())).filter(Boolean))])
const readinessText = computed(() => props.detail?.readiness?.can_submit ? '可以提交' : isDraft.value ? '需要完善' : '已冻结')
const footerTitle = computed(() => isDraft.value ? (props.dirty ? '草稿内容尚未保存' : props.detail?.readiness?.can_submit ? '草稿已保存，可以提交' : '草稿已保存，仍有阻断项') : `${statusLabel.value} · 单据内容只读`)
const footerHint = computed(() => isDraft.value ? (props.dirty ? '先保存本页修改，系统会重新核对数量、来源和工序。' : '提交后将冻结本页配置并生成生产工单。') : `工单 ${props.detail?.related_work_orders?.length || 0} 张 · 工序卡 ${props.detail?.job_card_count || 0} 张`)

function quantity(value, unit = '') { const number = Number(value || 0); return `${Number.isInteger(number) ? number : Number(number.toFixed(6))} ${unit || ''}`.trim() }
function quantitySummary(item) { return productionPlanItemQuantitySummary(item) }
function bomSourceLabel(item) { return productionPlanItemBomSourceLabel(item) }
function materialTypeLabel(item) { const type = String(item.component_type || ''); return type === 'packaging' ? '包材' : type === 'product' || type === 'finished_product' ? '半成品' : '生产原料' }
function availableWarehouses(item) { return props.warehouses.filter((row) => !Number(row.customer_id || 0) || Number(row.customer_id) === Number(item.customer_id || 0)) }
function warehouseLabel(code) { const row = props.warehouses.find((item) => String(item.code) === String(code)); return row ? `${row.name}（${row.code}）` : code || '-' }
function planItemName(id) { const item = (props.detail.items || []).find((row) => Number(row.id) === Number(id)); return item?.output_name || item?.product_name || `任务 #${id}` }
function sourceIdentity(source) { return [source.production_plan_item_id, source.component_type, source.component_id, source.component_bom_spec_id, source.component_spec_g].join(':') }
function optionKey(option) { return `${String(option.warehouse || '').trim()}\u001f${Number(option.owner_customer_id || 0)}` }
function sourceOptionKey(source) { return source.source_warehouse ? `${String(source.source_warehouse).trim()}\u001f${Number(source.source_owner_customer_id || 0)}` : '' }
function optionLabel(option, source) { const warehouse = option.warehouse_name ? `${option.warehouse_name}（${option.warehouse}）` : option.warehouse; const owner = option.owner_name || (option.owner_customer_id ? `客户 #${option.owner_customer_id}` : '工厂'); const available = Number(source.required_g || 0) > 0 ? `${option.available_g || 0}g` : `${option.available_units || 0}${source.unit || '件'}`; return `${warehouse} · ${owner} · 可用 ${available}` }
function selectedSourceLabel(source) { const option = (source.options || []).find((row) => optionKey(row) === sourceOptionKey(source)); return option ? optionLabel(option, source) : source.source_warehouse || '-' }
function sourceRequired(source) { return Number(source.required_g || 0) > 0 ? `${source.required_g}g` : `${source.required_units || 0}${source.unit || '件'}` }
function sourceAvailable(source) { return Number(source.required_g || 0) > 0 ? `${source.available_g_snapshot || 0}g` : `${source.available_units_snapshot || 0}${source.unit || '件'}` }
function sourceShort(source) { return Number(source.shortage_g || 0) > 0 || Number(source.shortage_units || 0) > 0 }
function processRouteLabel(item) { try { const raw = item.process_snapshot_json; const snapshot = typeof raw === 'object' ? raw : JSON.parse(raw || '{}'); return snapshot.name || (snapshot.operations || []).map((row) => row.operation).filter(Boolean).join(' → ') || '未设置工艺路线' } catch (_) { return '工艺快照待核对' } }
function edgeRequired(edge) { return Number(edge.required_g || 0) > 0 ? `${Number((Number(edge.required_g) / 1000).toFixed(6))} kg` : quantity(edge.required_units || edge.required_qty, edge.required_units ? '件' : '') }
function workOrderStatus(status) { return ({ draft: '草稿', released: '待开工', running: '生产中', completed: '已完成', cancelled: '已取消' })[status] || status || '-' }
function issueTitle(category) { return ({ quantity: '数量需要核对', source: '来源仓库需要完善', supply: '供应存在缺口', operation: '工序拆分需要完善' })[category] || '计划需要完善' }
function focusIssue(issue) {
  if (issue.category === 'operation') { emit('edit-splits'); return }
  const id = issue.component_source_id ? `component-source-${issue.component_source_id}` : issue.production_plan_item_id ? `plan-item-${issue.production_plan_item_id}` : 'material-sources'
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
</script>

<style scoped>
.production-plan-workspace{--ink:#18352e;--muted:#657771;--line:#dfe7e3;--green:#1e6a50;--green-soft:#eaf5ef;--paper:#f7f9f7;min-height:100vh;background:var(--paper);color:var(--ink);padding-bottom:88px}.workspace-header{position:sticky;top:0;z-index:12;padding:20px 28px 18px;background:rgba(255,255,255,.97);border-bottom:1px solid var(--line);box-shadow:0 6px 24px rgba(24,53,46,.06)}.back-button,.text-button,.danger-link{border:0;background:none;color:var(--green);font-weight:700;cursor:pointer;padding:0}.back-button{margin-bottom:13px}.header-main,.title-row,.header-actions,.section-heading>div,.readiness-title,.readiness-title>div{display:flex;align-items:center}.header-main{justify-content:space-between;gap:24px}.eyebrow,.section-index{font-size:12px;font-weight:800;letter-spacing:.08em;color:#7b8c86;text-transform:uppercase}.title-row{gap:12px;margin:3px 0}.title-row h1{font-size:25px;margin:0}.header-main p,.section-heading p{margin:0;color:var(--muted);font-size:13px}.header-actions,.footer-actions{display:flex;gap:10px}.status-pill,.unsaved-pill,.readiness-count{border-radius:999px;padding:5px 10px;font-size:12px;font-weight:800}.status-draft,.unsaved-pill{background:#fff1d9;color:#925b08}.status-submitted,.status-in-progress,.readiness-count.ready{background:var(--green-soft);color:var(--green)}.status-completed{background:#e8f2ff;color:#1a5f9d}.status-cancelled{background:#eef0ef;color:#68736f}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-top:18px}.summary-grid article{display:grid;gap:3px;padding:13px 15px;background:#f5f8f6;border:1px solid #e5ebe8;border-radius:10px}.summary-grid span,.summary-grid small{font-size:12px;color:var(--muted)}.summary-grid strong{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.workspace-body{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:22px;max-width:1480px;margin:0 auto;padding:24px 28px}.workspace-main{display:grid;gap:22px}.content-section,.readiness-panel{background:#fff;border:1px solid var(--line);border-radius:14px;box-shadow:0 8px 28px rgba(24,53,46,.045)}.content-section{padding:22px}.section-heading{display:flex;justify-content:space-between;gap:20px;margin-bottom:20px}.section-heading>div{align-items:flex-start;gap:12px}.section-heading h2{font-size:19px;margin:0 0 5px}.section-index{display:inline-grid;place-items:center;min-width:34px;height:25px;background:var(--green-soft);border-radius:6px;color:var(--green)}.stage-list{display:grid;gap:4px}.stage-card{display:grid;grid-template-columns:34px minmax(0,1fr);gap:12px}.stage-rail{display:flex;flex-direction:column;align-items:center}.stage-rail span{display:grid;place-items:center;width:28px;height:28px;border-radius:50%;background:var(--green);color:#fff;font-size:12px;font-weight:800}.stage-rail i{width:2px;flex:1;min-height:28px;background:#c8ddd3}.stage-content{padding-bottom:16px}.stage-head{display:flex;align-items:baseline;gap:9px;margin:3px 0 12px}.stage-head span,.stage-head small{color:var(--muted);font-size:12px}.stage-head h3{font-size:16px;margin:0}.task-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.task-card{padding:16px;border:1px solid var(--line);border-radius:12px;background:#fcfdfc}.task-top{display:flex;justify-content:space-between;gap:16px}.task-top h4{font-size:16px;margin:5px 0}.task-top>strong{font-size:19px;color:var(--green);white-space:nowrap}.task-kind{font-size:11px;color:var(--muted);font-weight:700}.task-meta{display:flex;flex-wrap:wrap;gap:6px;margin:4px 0 13px}.task-meta span{padding:4px 7px;border-radius:5px;background:#eef3f0;color:#52645e;font-size:11px}.warehouse-field,.source-card label{display:grid;gap:5px}.warehouse-field>span,.source-card label>span,.source-need>span,.source-state>span{font-size:11px;color:var(--muted)}select{width:100%;min-height:36px;border:1px solid #cfdad5;border-radius:7px;background:#fff;padding:6px 9px;color:var(--ink)}.trace-row,.dependency-row{display:flex;gap:8px;margin-top:11px;font-size:12px}.trace-row>span,.dependency-row>span{color:var(--muted);min-width:36px}.trace-row b{display:inline-block;margin:0 5px 4px 0;padding:3px 6px;border-radius:5px;background:#f0f3f2;font-weight:600}.dependency-row div{color:#695b31}.dependency-row.supplies div{color:var(--green)}.material-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px;margin-bottom:16px}.material-summary article{display:grid;grid-template-columns:1fr auto;gap:4px;padding:12px;border-radius:9px;background:#f5f8f6}.material-summary span{grid-column:1/-1;color:var(--muted);font-size:11px}.material-summary b{color:var(--green)}.source-list{display:grid;gap:9px}.source-card{display:grid;grid-template-columns:minmax(190px,1.1fr) 110px minmax(240px,1.2fr) 120px;align-items:center;gap:14px;padding:13px 15px;border:1px solid var(--line);border-radius:10px}.source-card.invalid{border-color:#eac99c;background:#fffaf2}.source-identity{display:flex;gap:10px;align-items:center}.source-identity>span{padding:4px 7px;background:#edf3f0;border-radius:5px;font-size:11px}.source-identity h3,.source-identity p{margin:0}.source-identity p{font-size:11px;color:var(--muted);margin-top:3px}.source-need,.source-state{display:grid;gap:4px}.source-state.short strong{color:#a84c20}.detail-section details{border-top:1px solid var(--line);padding:13px 2px}.detail-section summary{cursor:pointer;font-weight:750}.frozen-grid,.trace-table,.work-order-list{display:grid;gap:8px;margin-top:12px}.frozen-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.frozen-grid article,.trace-table article,.work-order-list article{display:flex;justify-content:space-between;gap:12px;padding:11px;border-radius:8px;background:#f5f8f6;font-size:12px}.frozen-grid article{display:grid}.frozen-grid span,.trace-table span,.work-order-list span{color:var(--muted)}.work-order-list article{align-items:center}.work-order-list article>div{display:grid;gap:3px}.work-order-list button,.issue-list button{border:0;background:none;color:var(--green);font-weight:700;cursor:pointer}.readiness-panel{position:sticky;top:202px;align-self:start;padding:20px}.readiness-title{justify-content:space-between;gap:10px}.readiness-title>div{gap:9px}.readiness-title h2{font-size:18px;margin:0}.readiness-count{background:#fff0df;color:#9a5914}.readiness-panel>p{font-size:13px;color:var(--muted);line-height:1.55}.issue-list{display:grid;gap:9px;margin-top:16px}.issue-list article{display:flex;gap:10px;padding:12px;border:1px solid #ecd5b5;border-radius:9px;background:#fffaf2}.issue-icon{display:grid;place-items:center;flex:0 0 22px;height:22px;border-radius:50%;background:#b96620;color:#fff;font-weight:900}.issue-list strong{font-size:13px}.issue-list p{margin:4px 0;font-size:12px;line-height:1.5;color:#6a5f54}.issue-list button{font-size:12px;padding:0}.ready-card{text-align:center;padding:22px 12px;border-radius:10px;background:var(--green-soft)}.ready-card>span{display:grid;place-items:center;width:32px;height:32px;margin:0 auto 8px;border-radius:50%;background:var(--green);color:#fff}.ready-card p{margin:5px 0;color:var(--muted);font-size:12px}.readiness-note{margin-top:16px;padding:13px;border-radius:9px;background:#f3f6f4}.readiness-note p{margin:5px 0 0;color:var(--muted);font-size:12px;line-height:1.55}.workspace-footer{position:sticky;bottom:0;z-index:14;display:flex;justify-content:space-between;align-items:center;gap:24px;padding:14px 28px;background:rgba(255,255,255,.98);border-top:1px solid var(--line);box-shadow:0 -8px 24px rgba(24,53,46,.08)}.workspace-footer>div:first-child{display:grid;gap:3px}.workspace-footer span{font-size:12px;color:var(--muted)}button.primary,button.secondary{min-height:38px;padding:0 15px;border-radius:8px;font-weight:750;cursor:pointer}button.primary{border:1px solid var(--green);background:var(--green);color:#fff}button.secondary{border:1px solid #cbd8d2;background:#fff;color:var(--ink)}button:disabled{opacity:.45;cursor:not-allowed}.danger-link{color:#a03d2e}.workspace-loading,.workspace-error,.empty-card{padding:32px;text-align:center;color:var(--muted)}.workspace-error{color:#a03d2e}.muted{color:var(--muted);font-size:12px}
@media(max-width:1100px){.workspace-body{grid-template-columns:1fr}.readiness-panel{position:relative;top:auto;order:-1}.task-grid{grid-template-columns:1fr}.source-card{grid-template-columns:1fr 1fr}.summary-grid{grid-template-columns:repeat(2,1fr)}}
@media(max-width:720px){.workspace-header,.workspace-body,.workspace-footer{padding-left:16px;padding-right:16px}.header-main,.workspace-footer{align-items:flex-start;flex-direction:column}.summary-grid,.material-summary,.frozen-grid,.source-card{grid-template-columns:1fr}.footer-actions{width:100%;flex-wrap:wrap}.footer-actions .primary{flex:1}.production-plan-workspace{padding-bottom:130px}}
</style>
