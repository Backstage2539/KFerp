<template>
  <section class="production-plan-workspace" aria-label="生产计划详情工作区">
    <header class="workspace-header">
      <button class="back-button" type="button" @click="$emit('back')"><span aria-hidden="true">←</span> 返回生产计划</button>
      <div class="header-main">
        <div>
          <div class="eyebrow">生产计划</div>
          <div class="title-row">
            <h1>生产计划 {{ detail.plan_no || '#' + detail.id }}</h1>
            <span :class="['status-pill', `status-${statusTone}`]">{{ statusLabel }}</span>
            <span v-if="dirty" class="unsaved-pill">有未保存修改</span>
          </div>
          <p v-if="isDraft" class="draft-edit-label">正在编辑草稿 · 修改订单需求请撤销后重新创建</p>
          <p>创建于 {{ detail.created_at || '-' }} · {{ detail.created_by || '未记录创建人' }}</p>
        </div>
        <details v-if="isDraft && !loading" class="header-actions more-actions">
          <summary>更多 <IconChevronDown :size="15" aria-hidden="true" /></summary>
          <div class="more-menu">
            <button type="button" :disabled="saving" @click="$emit('refresh')">刷新库存与在产供应</button>
            <button type="button" :disabled="saving" @click="$emit('reload')">重新加载详情</button>
            <button class="danger-link" type="button" :disabled="saving" @click="$emit('cancel')">撤销草稿</button>
          </div>
        </details>
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

    <div v-if="notice" role="status" class="workspace-notice">{{ notice }}</div>
    <div v-if="loading" class="workspace-loading">正在读取生产计划详情…</div>
    <div v-if="error" class="workspace-error" role="alert"><span>{{ error }}</span><button class="secondary" :disabled="saving || loading" type="button" @click="$emit('reload')">重新加载详情</button><small>已创建的草稿会保留，请重新加载本单据。</small></div>
    <div v-if="!loading && detail.readiness" class="workspace-body">
      <main class="workspace-main">
        <section class="content-section schedule-section">
          <div class="section-heading">
            <div>
              <span class="section-index">01</span>
              <div><h2>本次生产安排</h2><p>按物料流向从上游到成品排列，每张任务卡保留自己的订单、客户、规格和冻结版本。</p></div>
            </div>
            <button v-if="isDraft" class="text-button" type="button" @click="$emit('edit-splits')">编辑工位与批次</button>
          </div>

          <p class="preparation-guidance">每个规格组紧接展示本组备料总量；展开订单任务可查看各自订单、来源仓、货主和批次。</p>
          <div v-if="stages.length" id="plan-stages" class="stage-list">
            <article v-for="(stage, stageIndex) in stages" :key="stage.key" class="stage-card">
              <div class="stage-rail"><span>{{ stageIndex + 1 }}</span><i v-if="stageIndex < stages.length - 1" /></div>
              <div class="stage-content">
                <div class="stage-head"><span>阶段 {{ stageIndex + 1 }}</span><h3>{{ stage.title }}</h3><small>{{ stage.groups.length }} 个规格组 · {{ stage.tasks.length }} 项任务</small></div>
                <div class="plan-groups">
                  <article v-for="(group, groupIndex) in stage.groups" :id="group.anchor_id" :key="group.key" class="plan-task-group">
                    <header class="plan-group-heading">
                      <div><span>同规格生产组 · {{ group.tasks[0]?.kind_label }} · {{ group.spec_label }}</span><h4>{{ group.tasks[0]?.item.output_name || group.tasks[0]?.item.product_name || '-' }}</h4></div>
                      <strong>{{ group.quantity_label }}</strong>
                      <small>{{ group.tasks.length }} 项任务 · {{ group.orders.length }} 张订单</small>
                    </header>

                    <section class="group-preparation" :aria-label="`${group.spec_label}本组备料`">
                      <div class="group-preparation-heading"><strong>本组备料</strong><span v-if="group.preparations.length">总需求按任务来源汇总</span></div>
                      <div v-if="group.preparations.length" class="group-preparation-list">
                        <article v-for="preparation in group.preparations" :key="preparation.key" :class="['group-preparation-row', { shortage: preparation.status_key === 'shortage' }]">
                          <div class="group-preparation-name"><span>{{ materialTypeLabel(preparation) }}</span><strong>{{ preparation.component_name || `物料 #${preparation.component_id}` }}</strong><small v-if="preparationSpecLabel(preparation)">{{ preparationSpecLabel(preparation) }}</small></div>
                          <dl>
                            <div><dt>需求</dt><dd>{{ preparationAmount(preparation, 'demand') }}</dd></div>
                            <div><dt>WIP 已有</dt><dd>{{ preparationAmount(preparation, 'wip_covered') }}</dd></div>
                            <div><dt>待领</dt><dd>{{ preparationAmount(preparation, 'transfer') }}</dd></div>
                            <div><dt>上游供给</dt><dd>{{ preparationAmount(preparation, 'upstream') }}</dd></div>
                            <div><dt>真实缺口</dt><dd :class="{ missing: preparation.shortage_g || preparation.shortage_units }">{{ preparationAmount(preparation, 'shortage') }}</dd></div>
                          </dl>
                          <span :class="['group-preparation-state', { shortage: preparation.status_key === 'shortage' }]">{{ preparation.state_label }}</span>
                          <small v-if="preparationOrigins(preparation).length" class="group-preparation-origin">{{ preparationOrigins(preparation).join('；') }}</small>
                        </article>
                      </div>
                      <p v-else-if="group.tasks.some((task) => task.dependencies.length)" class="muted">旧版计划保留在订单任务中的上游供给关系；未生成的备料来源不会按名称猜测。</p>
                      <p v-else-if="!detail.picking_version && detail.material_summary?.length" class="muted">旧版备料汇总没有任务归属，暂列为未关联备料。</p>
                      <p v-else class="muted">本组暂无备料来源。</p>
                    </section>

                    <details class="group-task-details" :open="group.tasks.length === 1">
                      <summary>{{ group.tasks.length === 1 ? '生产任务明细' : `展开 ${group.tasks.length} 项订单任务` }}<span v-if="group.orders.length"> · {{ group.orders.join('、') }}</span></summary>
                      <div class="task-grid">
                        <article v-for="task in group.tasks" :id="`plan-item-${task.id}`" :key="task.id" class="task-card">
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
                          <ProductionPreparation v-if="detail.picking_version && task.component_sources.length" :sources="task.component_sources" :items="detail.items || []" :supply-allocations="detail.supply_allocations || []" :work-orders="detail.related_work_orders || []" :editable="isDraft" :saving="saving" :show-note="false" @adjust="(source, allocations) => $emit('adjust-source', source, allocations)" @issue="issueWorkOrder" />
                          <div v-else-if="task.component_sources.length" class="legacy-source-list">
                            <article v-for="source in task.component_sources" :id="`component-source-${source.id}`" :key="sourceIdentity(source)" class="legacy-source-row">
                              <div><strong>{{ source.component_name }}</strong><small>本项需求 {{ sourceRequired(source) }} · {{ source.source_warehouse || '待选择来源' }} · {{ source.source_owner_customer_id ? `客户 #${source.source_owner_customer_id}` : '工厂' }}</small></div>
                              <select v-if="isDraft" :value="sourceOptionKey(source)" @change="$emit('source-change', source, $event.target.value)"><option value="">请选择来源</option><option v-for="option in source.options || []" :key="optionKey(option)" :value="optionKey(option)">{{ optionLabel(option, source) }}</option></select>
                              <strong v-else>{{ selectedSourceLabel(source) }}</strong>
                            </article>
                          </div>
                          <p v-else-if="task.dependencies.length" class="muted">上游供给按阶段依赖显示；旧版计划没有对应库存领料行。</p>
                          <p v-else class="muted">本任务暂无可追溯的备料来源。</p>
                          <div v-if="!isDraft && task.item.replan_status === 'withdrawn'" class="replan-state"><span>已撤回重排</span><button v-if="task.item.replaced_by_plan_id" type="button" @click="$emit('navigate', 'producePlan', { production_plan_id: task.item.replaced_by_plan_id })">查看新计划</button></div>
                          <button v-else-if="!isDraft && task.item.output_type !== 'material'" class="replan-button" type="button" @click="$emit('replan', task.item)">撤回并重新安排</button>
                        </article>
                      </div>
                    </details>
                  </article>
                </div>
              </div>
            </article>
          </div>
          <div v-else class="empty-card">暂无生产任务</div>

          <section v-if="unlinkedSources.length || (!detail.picking_version && detail.material_summary?.length)" class="unlinked-preparation">
            <h3>未关联备料</h3>
            <p>这些旧版汇总或来源没有可核对的任务编号，暂不分摊到生产安排；请重新加载详情核对。</p>
            <article v-for="source in unlinkedSources" :id="`component-source-${source.id}`" :key="`unlinked-${source.id}`" class="unlinked-source-row">
              <strong>{{ source.component_name || `物料 #${source.component_id}` }}</strong><span>需求 {{ sourceRequired(source) }}</span><span>来源 {{ source.source_warehouse || '待落实' }}</span><span>{{ source.source_owner_customer_id ? `客户 #${source.source_owner_customer_id}` : '货主以来源分配明细为准' }}</span>
            </article>
            <article v-if="!detail.picking_version" v-for="material in detail.material_summary || []" :key="`legacy-summary-${material.name}-${material.unit}-${material.component_type}`" class="unlinked-source-row">
              <strong>{{ material.name }}</strong><span>{{ materialTypeLabel(material) }}</span><span>{{ quantity(material.qty, material.unit) }}</span><span>旧版汇总，无任务归属</span>
            </article>
          </section>
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
          <div><span class="section-index">核对</span><h2>{{ isDraft ? '提交前核对' : '执行提示' }}</h2></div>
          <span :class="['readiness-count', { ready: detail.readiness?.can_submit }]">{{ !isDraft ? '已冻结' : detail.readiness?.can_submit ? '已通过' : `${detail.readiness?.blocking_count || 0} 项待处理` }}</span>
        </div>
        <p>{{ !isDraft ? '计划供给已冻结。按工单领料，物料到 WIP 且齐套后可开工。' : detail.readiness?.can_submit ? '仓库、用料和工序安排均已核对，可提交生成工单。' : '请完成下面的必填项。保存草稿后系统会重新计算。' }}</p>
        <div v-if="detail.readiness?.issues?.length" class="issue-list">
          <article v-for="issue in detail.readiness.issues" :key="`${issue.code}-${issue.production_plan_item_id}-${issue.component_source_id}`">
            <span class="issue-icon" aria-hidden="true">!</span>
            <div><strong>{{ issueTitle(issue.category) }}</strong><p>{{ issue.message }}</p><button v-if="issue.action" type="button" @click="focusIssue(issue)">{{ issue.action }} →</button></div>
          </article>
        </div>
        <div v-else-if="isDraft" class="ready-card"><span aria-hidden="true">✓</span><strong>没有阻断项</strong><p>当前内容已满足提交条件。</p></div>
        <div class="readiness-note"><strong>数量口径</strong><p>商品按冻结销售规格计数，用料按 BOM 净需求加一次损耗。页面汇总、来源核对和工单使用同一份数量。</p></div>
      </aside>
    </div>

    <footer class="workspace-footer">
      <div>
        <strong>{{ footerTitle }}</strong>
        <span>{{ footerHint }}</span>
      </div>
      <div v-if="isDraft" class="footer-actions">
        <button class="secondary" type="button" :disabled="loading || saving || !dirty" @click="$emit('save')">{{ saving ? '正在保存…' : '保存草稿' }}</button>
        <button class="primary" type="button" :disabled="loading || saving || !!error || dirty || !detail.readiness?.can_submit" @click="$emit('submit')">提交生成工单</button>
      </div>
      <button v-else-if="detail.related_work_orders?.length" class="primary" type="button" @click="$emit('navigate', 'workOrders', { production_plan_id: detail.id })">查看生产工单</button>
    </footer>
  </section>
</template>

<script setup>
import { computed, nextTick } from 'vue'
import ProductionPreparation from './ProductionPreparation.vue'
import { IconChevronDown } from '@tabler/icons-vue'
import {
  buildProductionPlanStages,
  buildProductionPlanTaskGroups,
  findUnlinkedProductionPlanComponentSources,
  expandProductionPlanIssueTarget,
  productionPlanItemBomSourceLabel,
  productionPlanItemQuantitySummary,
  productionPlanIssueTargetId,
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
  notice: { type: String, default: '' },
})
const emit = defineEmits(['back', 'reload', 'save', 'submit', 'edit-splits', 'refresh', 'cancel', 'replan', 'navigate', 'source-change', 'warehouse-change', 'adjust-source'])

const stages = computed(() => buildProductionPlanStages(props.detail).map((stage) => ({
  ...stage,
  groups: buildProductionPlanTaskGroups(stage, props.detail).map((group, index) => ({
    ...group,
    anchor_id: `plan-group-${stage.level}-${index}`,
  })),
})))
const unlinkedSources = computed(() => findUnlinkedProductionPlanComponentSources(props.detail))
const isDraft = computed(() => String(props.detail?.status || '') === 'draft')
const statusLabel = computed(() => productionPlanStatusLabel(props.detail?.status))
const statusTone = computed(() => productionPlanStatusTone(props.detail?.status))
const productNames = computed(() => [...new Set((props.detail?.items || []).filter((item) => props.detail?.source_type === 'stock' || String(item.output_type || 'product') !== 'material').map((item) => item.output_name || item.product_name).filter(Boolean))])
const productSummary = computed(() => productNames.value.slice(0, 2).join('、') || '备货计划')
const productKinds = computed(() => productNames.value.length)
const orderNos = computed(() => [...new Set((props.detail?.items || []).flatMap((item) => String(item.order_nos || '').split(',').map((value) => value.trim())).filter(Boolean))])
const readinessText = computed(() => props.detail?.readiness?.can_submit ? '可以提交' : isDraft.value ? '需要完善' : '已冻结')
const footerTitle = computed(() => isDraft.value ? (props.dirty ? '有未保存修改' : props.detail?.readiness?.can_submit ? '已具备提交条件' : '草稿已保存，有待处理项') : `${statusLabel.value} · 单据内容只读`)
const footerHint = computed(() => isDraft.value ? (props.dirty ? '先保存本页修改，系统会重新核对数量、来源和工序。' : '提交后将冻结本页配置并生成生产工单。') : `工单 ${props.detail?.related_work_orders?.length || 0} 张 · 工序卡 ${props.detail?.job_card_count || 0} 张`)

function issueWorkOrder(order) { emit('navigate', 'stockOperations', {tab:'stockEntries',action:'supplement',return_source:'work_order',work_order_id:order.id,production_plan_id:props.detail?.id}) }
function quantity(value, unit = '') { const number = Number(value || 0); return `${Number.isInteger(number) ? number : Number(number.toFixed(6))} ${unit || ''}`.trim() }
function quantitySummary(item) { return item.output_type === 'material' ? '按物料单位' : productionPlanItemQuantitySummary(item) }
function bomSourceLabel(item) { return productionPlanItemBomSourceLabel(item) }
function materialTypeLabel(item) { const type = String(item.component_type || ''); return type === 'packaging' ? '包材' : type === 'product' || type === 'finished_product' ? '半成品' : '生产用料' }
function preparationSpecLabel(preparation) {
  const specG = Number(preparation.component_spec_g || 0)
  if (specG > 0) return `规格 ${specG}g`
  const bomSpecID = Number(preparation.component_bom_spec_id || 0)
  if (bomSpecID > 0) return `BOM规格 #${bomSpecID}`
  const variantID = Number(preparation.component_bom_variant_id || 0)
  return variantID > 0 ? `配方变体 #${variantID}` : ''
}
function preparationAmount(preparation, key) {
  const grams = Number(preparation[`${key}_g`] || 0)
  const units = Number(preparation[`${key}_units`] || 0)
  if (grams > 0 || ['g', 'kg', '克', '千克', '公斤'].includes(String(preparation.unit || '').toLowerCase())) {
    const unit = ['kg', '千克', '公斤'].includes(String(preparation.unit || '').toLowerCase()) ? 'kg' : 'g'
    return quantity(unit === 'kg' ? grams / 1000 : grams, unit)
  }
  return quantity(units, preparation.unit || '件')
}
function preparationOrigins(preparation) {
  const labels = []
  for (const member of preparation.members || []) {
    const source = member.source || {}
    const componentType = String(source.component_type || '')
    const outputType = ['product', 'finished_product'].includes(componentType) ? 'product' : componentType
    if (!['product', 'material'].includes(outputType)) continue
    const supplier = (member.task.dependencies || []).find((link) => {
      const item = link.item || {}
      const supplierType = String(item.output_type || 'product')
      const supplierID = supplierType === 'material'
        ? Number(item.output_material_id || 0)
        : Number(item.output_product_id || item.product_id || 0)
      return supplierType === outputType && supplierID === Number(source.component_id || 0)
    })
    if (!supplier) continue
    const level = Number(supplier.item._source_stage_level || 0) + 1
    labels.push(`来自阶段 ${level} · ${supplier.item.output_name || supplier.item.product_name} ${supplier.quantity_label}`)
  }
  return [...new Set(labels)]
}
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
function issueTitle(category) { return ({ quantity: '数量需要核对', source: '备料供给需要补齐', supply: '供应存在缺口', operation: '工序拆分需要完善' })[category] || '计划需要完善' }
async function focusIssue(issue) {
  if (issue.action === '刷新供应') { emit('refresh'); return }
  if (issue.category === 'operation') { emit('edit-splits'); return }
  const node = expandProductionPlanIssueTarget(document.getElementById(productionPlanIssueTargetId(issue)))
  await nextTick()
  node?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
</script>

<style scoped>
.production-plan-workspace{--ink:#18352e;--muted:#657771;--line:#dfe7e3;--green:#1e6a50;--green-soft:#eaf5ef;--paper:#f7f9f7;min-height:100vh;background:var(--paper);color:var(--ink);padding-bottom:88px}.workspace-header{position:sticky;top:0;z-index:12;padding:20px 28px 18px;background:rgba(255,255,255,.97);border-bottom:1px solid var(--line);box-shadow:0 6px 24px rgba(24,53,46,.06)}.back-button,.text-button,.danger-link{border:0;background:none;color:var(--green);font-weight:700;cursor:pointer;padding:0}.back-button{margin-bottom:13px}.header-main,.title-row,.header-actions,.section-heading>div,.readiness-title,.readiness-title>div{display:flex;align-items:center}.header-main{justify-content:space-between;gap:24px}.eyebrow,.section-index{font-size:12px;font-weight:800;letter-spacing:.08em;color:#7b8c86;text-transform:uppercase}.title-row{gap:12px;margin:3px 0}.title-row h1{font-size:25px;margin:0}.header-main p,.section-heading p{margin:0;color:var(--muted);font-size:13px}.header-actions,.footer-actions{display:flex;gap:10px}.status-pill,.unsaved-pill,.readiness-count{border-radius:999px;padding:5px 10px;font-size:12px;font-weight:800}.status-draft,.unsaved-pill{background:#fff1d9;color:#925b08}.status-submitted,.status-in-progress,.readiness-count.ready{background:var(--green-soft);color:var(--green)}.status-completed{background:#e8f2ff;color:#1a5f9d}.status-cancelled{background:#eef0ef;color:#68736f}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-top:18px}.summary-grid article{display:grid;gap:3px;padding:13px 15px;background:#f5f8f6;border:1px solid #e5ebe8;border-radius:10px}.summary-grid span,.summary-grid small{font-size:12px;color:var(--muted)}.summary-grid strong{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.workspace-body{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:22px;max-width:1480px;margin:0 auto;padding:24px 28px}.workspace-main{display:grid;gap:22px}.content-section,.readiness-panel{background:#fff;border:1px solid var(--line);border-radius:14px;box-shadow:0 8px 28px rgba(24,53,46,.045)}.content-section{padding:22px}.section-heading{display:flex;justify-content:space-between;gap:20px;margin-bottom:20px}.section-heading>div{align-items:flex-start;gap:12px}.section-heading h2{font-size:19px;margin:0 0 5px}.section-index{display:inline-grid;place-items:center;min-width:34px;height:25px;background:var(--green-soft);border-radius:6px;color:var(--green)}.stage-list{display:grid;gap:4px}.stage-card{display:grid;grid-template-columns:34px minmax(0,1fr);gap:12px}.stage-rail{display:flex;flex-direction:column;align-items:center}.stage-rail span{display:grid;place-items:center;width:28px;height:28px;border-radius:50%;background:var(--green);color:#fff;font-size:12px;font-weight:800}.stage-rail i{width:2px;flex:1;min-height:28px;background:#c8ddd3}.stage-content{padding-bottom:16px}.stage-head{display:flex;align-items:baseline;gap:9px;margin:3px 0 12px}.stage-head span,.stage-head small{color:var(--muted);font-size:12px}.stage-head h3{font-size:16px;margin:0}.task-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.task-card{padding:16px;border:1px solid var(--line);border-radius:12px;background:#fcfdfc}.task-top{display:flex;justify-content:space-between;gap:16px}.task-top h4{font-size:16px;margin:5px 0}.task-top>strong{font-size:19px;color:var(--green);white-space:nowrap}.task-kind{font-size:11px;color:var(--muted);font-weight:700}.task-meta{display:flex;flex-wrap:wrap;gap:6px;margin:4px 0 13px}.task-meta span{padding:4px 7px;border-radius:5px;background:#eef3f0;color:#52645e;font-size:11px}.warehouse-field,.source-card label{display:grid;gap:5px}.warehouse-field>span,.source-card label>span,.source-need>span,.source-state>span{font-size:11px;color:var(--muted)}select{width:100%;min-height:36px;border:1px solid #cfdad5;border-radius:7px;background:#fff;padding:6px 9px;color:var(--ink)}.trace-row,.dependency-row{display:flex;gap:8px;margin-top:11px;font-size:12px}.trace-row>span,.dependency-row>span{color:var(--muted);min-width:36px}.trace-row b{display:inline-block;margin:0 5px 4px 0;padding:3px 6px;border-radius:5px;background:#f0f3f2;font-weight:600}.dependency-row div{color:#695b31}.dependency-row.supplies div{color:var(--green)}.material-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px;margin-bottom:16px}.material-summary article{display:grid;grid-template-columns:1fr auto;gap:4px;padding:12px;border-radius:9px;background:#f5f8f6}.material-summary span{grid-column:1/-1;color:var(--muted);font-size:11px}.material-summary b{color:var(--green)}.source-list{display:grid;gap:9px}.source-card{display:grid;grid-template-columns:minmax(190px,1.1fr) 110px minmax(240px,1.2fr) 120px;align-items:center;gap:14px;padding:13px 15px;border:1px solid var(--line);border-radius:10px}.source-card.invalid{border-color:#eac99c;background:#fffaf2}.source-identity{display:flex;gap:10px;align-items:center}.source-identity>span{padding:4px 7px;background:#edf3f0;border-radius:5px;font-size:11px}.source-identity h3,.source-identity p{margin:0}.source-identity p{font-size:11px;color:var(--muted);margin-top:3px}.source-need,.source-state{display:grid;gap:4px}.source-state.short strong{color:#a84c20}.detail-section details{border-top:1px solid var(--line);padding:13px 2px}.detail-section summary{cursor:pointer;font-weight:750}.frozen-grid,.trace-table,.work-order-list{display:grid;gap:8px;margin-top:12px}.frozen-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.frozen-grid article,.trace-table article,.work-order-list article{display:flex;justify-content:space-between;gap:12px;padding:11px;border-radius:8px;background:#f5f8f6;font-size:12px}.frozen-grid article{display:grid}.frozen-grid span,.trace-table span,.work-order-list span{color:var(--muted)}.work-order-list article{align-items:center}.work-order-list article>div{display:grid;gap:3px}.work-order-list button,.issue-list button{border:0;background:none;color:var(--green);font-weight:700;cursor:pointer}.readiness-panel{position:sticky;top:202px;align-self:start;padding:20px}.readiness-title{justify-content:space-between;gap:10px}.readiness-title>div{gap:9px}.readiness-title h2{font-size:18px;margin:0}.readiness-count{background:#fff0df;color:#9a5914}.readiness-panel>p{font-size:13px;color:var(--muted);line-height:1.55}.issue-list{display:grid;gap:9px;margin-top:16px}.issue-list article{display:flex;gap:10px;padding:12px;border:1px solid #ecd5b5;border-radius:9px;background:#fffaf2}.issue-icon{display:grid;place-items:center;flex:0 0 22px;height:22px;border-radius:50%;background:#b96620;color:#fff;font-weight:900}.issue-list strong{font-size:13px}.issue-list p{margin:4px 0;font-size:12px;line-height:1.5;color:#6a5f54}.issue-list button{font-size:12px;padding:0}.ready-card{text-align:center;padding:22px 12px;border-radius:10px;background:var(--green-soft)}.ready-card>span{display:grid;place-items:center;width:32px;height:32px;margin:0 auto 8px;border-radius:50%;background:var(--green);color:#fff}.ready-card p{margin:5px 0;color:var(--muted);font-size:12px}.readiness-note{margin-top:16px;padding:13px;border-radius:9px;background:#f3f6f4}.readiness-note p{margin:5px 0 0;color:var(--muted);font-size:12px;line-height:1.55}.workspace-footer{position:sticky;bottom:0;z-index:14;display:flex;justify-content:space-between;align-items:center;gap:24px;padding:14px 28px;background:rgba(255,255,255,.98);border-top:1px solid var(--line);box-shadow:0 -8px 24px rgba(24,53,46,.08)}.workspace-footer>div:first-child{display:grid;gap:3px}.workspace-footer span{font-size:12px;color:var(--muted)}button.primary,button.secondary{min-height:38px;padding:0 15px;border-radius:8px;font-weight:750;cursor:pointer}button.primary{border:1px solid var(--green);background:var(--green);color:#fff}button.secondary{border:1px solid #cbd8d2;background:#fff;color:var(--ink)}button:disabled{opacity:.45;cursor:not-allowed}.danger-link{color:#a03d2e}.workspace-loading,.workspace-error,.empty-card{padding:32px;text-align:center;color:var(--muted)}.workspace-error{color:#a03d2e}.muted{color:var(--muted);font-size:12px}
@media(max-width:1100px){.workspace-body{grid-template-columns:1fr}.readiness-panel{position:relative;top:auto;order:-1}.task-grid{grid-template-columns:1fr}.source-card{grid-template-columns:1fr 1fr}.summary-grid{grid-template-columns:repeat(2,1fr)}}
@media(max-width:720px){.workspace-header,.workspace-body,.workspace-footer{padding-left:16px;padding-right:16px}.header-main,.workspace-footer{align-items:flex-start;flex-direction:column}.summary-grid,.material-summary,.frozen-grid,.source-card{grid-template-columns:1fr}.footer-actions{width:100%;flex-wrap:wrap}.footer-actions .primary{flex:1}.production-plan-workspace{padding-bottom:130px}}

.production-plan-workspace{--ink:#111827;--muted:#65748b;--line:#e1e7ed;--green:#238653;--green-soft:#ecf7f1;--paper:#fff;padding-bottom:0;display:flex;flex-direction:column}
.replan-button{margin-top:12px;border:0;background:none;color:var(--green);font-weight:750;cursor:pointer;padding:0}.replan-state{display:flex;justify-content:space-between;align-items:center;margin-top:12px;padding:8px 10px;border-radius:7px;background:#fff0df;color:#92590c;font-size:12px;font-weight:750}.replan-state button{border:0;background:none;color:var(--green);font-weight:750;cursor:pointer}
.workspace-header{position:relative;box-shadow:none;padding-bottom:0;border-bottom:0}
.workspace-body{padding-top:18px;width:100%;box-sizing:border-box}
.summary-grid{background:var(--green-soft);border-radius:8px;gap:0}
.summary-grid article{border:0;background:transparent;border-radius:0}
.content-section{box-shadow:none;border-radius:8px;padding:18px}
.readiness-panel{box-shadow:none;border:0;border-left:1px solid var(--line);border-radius:0;top:16px;padding:0 0 0 20px}
.task-grid{grid-template-columns:1fr}
.task-card{border-radius:7px;background:#fff;padding:14px}
.trace-row b{font-size:11px}
.task-meta span{background:#f3f5f7}
.issue-list article{background:#fff;border-color:var(--line);border-radius:7px}
.text-button,.back-button,.issue-list button{color:#2563a5}
.workspace-footer{position:static;bottom:auto;box-shadow:0 -3px 16px #14263408;margin-top:auto}
.header-main p.draft-edit-label{color:var(--green);margin:6px 0;font-weight:650}
.workspace-notice{margin:16px 28px 0;padding:11px 14px;border:1px solid #c7e3d2;background:#f0faf4;color:#23734d;border-radius:7px;font-size:13px}
.workspace-error{display:flex;gap:12px;align-items:center;flex-wrap:wrap;padding:16px 28px;text-align:left}
.workspace-error small{width:100%}
.more-actions{position:relative;display:block;align-self:flex-start}
.more-actions summary{display:flex;align-items:center;gap:8px;list-style:none;border:1px solid #cdd6df;border-radius:7px;padding:9px 13px;cursor:pointer;font-size:13px;white-space:nowrap}
.more-actions summary::-webkit-details-marker{display:none}
.more-menu{position:absolute;right:0;top:42px;z-index:25;display:grid;min-width:190px;padding:6px;background:#fff;border:1px solid var(--line);box-shadow:0 8px 24px #10203018;border-radius:8px}
.more-menu button{border:0;background:none;text-align:left;padding:10px;font-size:13px;cursor:pointer;border-radius:4px}
.more-menu button:hover{background:#f4f7f8}
.more-menu button.danger-link{color:#ad3f30}
@media(max-width:1100px){.readiness-panel{order:0;border-left:0;border-top:1px solid var(--line);padding:18px 0 0}.workspace-header{position:relative}}
@media(max-width:720px){.workspace-notice{margin-left:16px;margin-right:16px}.header-main{flex-direction:row;gap:10px}.title-row h1{font-size:21px}.title-row{flex-wrap:wrap}.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.header-main p.draft-edit-label{font-size:12px}.section-heading{flex-direction:column;gap:10px}.section-heading>.text-button{align-self:flex-start;white-space:nowrap}}
</style>

<style scoped>
.preparation-guidance{margin:-6px 0 18px;color:#657771;font-size:13px;line-height:1.55}
.plan-groups{display:grid;gap:12px}
.plan-task-group{border:1px solid #dfe7e3;border-radius:9px;background:#fff;overflow:hidden}
.plan-group-heading{display:grid;grid-template-columns:minmax(0,1fr) auto auto;gap:16px;align-items:center;padding:14px 16px;background:#f7faf8;border-bottom:1px solid #e7eeea}
.plan-group-heading>div>span,.plan-group-heading>small{font-size:12px;color:#657771}
.plan-group-heading h4{margin:4px 0 0;font-size:16px}
.plan-group-heading>strong{font-size:18px;color:#1e6a50;white-space:nowrap}
.group-preparation{padding:12px 16px 14px}
.group-preparation-heading{display:flex;align-items:center;gap:9px;margin-bottom:9px}
.group-preparation-heading>strong{font-size:13px}
.group-preparation-heading>span{font-size:11px;color:#657771}
.group-preparation-list{display:grid;gap:7px}
.group-preparation-row{display:grid;grid-template-columns:minmax(130px,1fr) minmax(300px,2fr) auto;gap:12px;align-items:center;padding:10px 12px;border:1px solid #e4ebe7;border-radius:7px;background:#fcfdfc}
.group-preparation-row.shortage{border-color:#e8b88d;background:#fffaf5}
.group-preparation-name{display:grid;gap:4px}
.group-preparation-name>span{font-size:11px;color:#657771}
.group-preparation-name>strong{font-size:13px;overflow-wrap:anywhere}
.group-preparation-name>small{font-size:11px;color:#657771}
.group-preparation-row dl{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:8px;margin:0}
.group-preparation-row dt{font-size:10px;color:#657771}
.group-preparation-row dd{margin:4px 0 0;font-size:12px;font-weight:700;white-space:nowrap}
.group-preparation-row dd.missing{color:#a34a1a}
.group-preparation-state{padding:5px 8px;border-radius:16px;background:#eaf5ef;color:#196145;font-size:11px;white-space:nowrap}
.group-preparation-state.shortage{background:#fff1e2;color:#9e501a}
.group-preparation-origin{grid-column:1/-1;color:#657771;font-size:11px}
.group-task-details{border-top:1px solid #e8eeeb;background:#fff}
.group-task-details>summary{padding:10px 16px;color:#2563a5;font-size:12px;font-weight:700;cursor:pointer;overflow-wrap:anywhere}
.group-task-details>.task-grid{padding:0 14px 14px}
.group-task-details .task-card{min-width:0}
.group-task-details .preparation{margin-top:12px;border-top:1px solid #eef1ef;padding-top:10px}
.legacy-source-list{display:grid;gap:7px;margin-top:12px}
.legacy-source-row,.unlinked-source-row{display:grid;grid-template-columns:minmax(0,1fr) minmax(200px,1fr);gap:10px;align-items:center;padding:10px;border:1px solid #e4ebe7;border-radius:7px;background:#fafcfb}
.legacy-source-row>div{display:grid;gap:4px}
.legacy-source-row small{color:#657771;font-size:11px}
.unlinked-preparation{margin-top:16px;padding:14px;border:1px solid #e9cfaa;border-radius:8px;background:#fffaf3}
.unlinked-preparation h3{margin:0 0 5px;font-size:14px}
.unlinked-preparation>p{margin:0 0 10px;color:#785c34;font-size:12px;line-height:1.5}
.unlinked-preparation .unlinked-source-row{margin-top:6px;background:#fff;border-color:#efe2d0;grid-template-columns:repeat(4,minmax(0,1fr));font-size:12px}
@media(max-width:850px){
  .plan-group-heading{grid-template-columns:1fr auto;gap:6px 12px}
  .plan-group-heading>small{grid-column:1/-1}
  .group-preparation-row{grid-template-columns:minmax(0,1fr) auto}
  .group-preparation-row dl{grid-column:1/-1;grid-row:2;grid-template-columns:repeat(3,minmax(0,1fr))}
  .group-preparation-state{grid-column:2;grid-row:1}
  .group-preparation-origin{grid-column:1/-1}
  .unlinked-preparation .unlinked-source-row{grid-template-columns:repeat(2,minmax(0,1fr))}
}
@media(max-width:520px){
  .plan-group-heading{padding:12px}
  .plan-group-heading>strong{font-size:16px}
  .group-preparation{padding:11px 12px}
  .group-preparation-row dl{gap:6px;grid-template-columns:repeat(2,minmax(0,1fr))}
  .group-preparation-row dd{white-space:nowrap}
  .group-preparation-state{font-size:10px;white-space:normal;text-align:center}
  .group-task-details>.task-grid{padding:0 10px 10px}
  .legacy-source-row,.unlinked-preparation .unlinked-source-row{grid-template-columns:1fr}
}
</style>
