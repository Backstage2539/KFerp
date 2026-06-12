<template>
  <div
    class="page"
    @pointerdown="startTableScrollDrag"
    @pointermove="moveTableScrollDrag"
    @pointerup="stopTableScrollDrag"
    @pointercancel="stopTableScrollDrag"
  >
    <section class="panel">
      <div class="panel-head">
        <h2>生产计划</h2>
        <button class="secondary" type="button" @click="load(false)" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="stockTip" class="ok direct-ship-tip">
        <div>
          <strong>{{ stockTip }}</strong>
          <span>库存充足订单在录单保存时确认是否使用成品批次；确认使用后进入“库存待发货”，可直接发货。</span>
        </div>
        <button class="secondary" type="button" @click="openShipReadyOrders">去订单列表直接发货</button>
      </div>
      <div class="filters">
        <label>
          <span>开始日期</span>
          <input v-model.trim="filters.from" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="filters.to" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>客户ID</span>
          <input v-model.trim="filters.customer_id" placeholder="例如 123" />
        </label>
        <label>
          <span>需求状态</span>
          <select v-model="demandStatusFilter" aria-label="需求状态：全部 / 待计划 / 生产中 / 生产完成" @change="load(false)">
            <option v-for="option in demandStatusOptions" :key="option.value || 'all'" :value="option.value">{{ option.label }}</option>
          </select>
        </label>
      </div>
    </section>

    <section :class="['planning-workbench', { 'demand-collapsed': demandPanelCollapsed, 'current-plan-collapsed': currentPlanPanelCollapsed }]">
      <section :class="['panel demand-panel', { 'is-collapsed': demandPanelCollapsed }]">
        <div class="panel-head">
          <div class="section-title section-title-with-checkbox">
            <input
              ref="insufficientHeaderCheckbox"
              class="bulk-checkbox"
              type="checkbox"
              :checked="allInsufficientSelected"
              :disabled="!insufficientSelection.total"
              :aria-checked="insufficientSelection.indeterminate ? 'mixed' : String(insufficientSelection.checked)"
              aria-label="全选库存不足商品"
              @change="toggleAllInsufficient($event.target.checked)"
            />
            <span>待生产需求</span>
          </div>
          <div class="panel-head-actions">
            <span class="muted">已选 {{ insufficientSelection.selectedCount }} / {{ insufficientSelection.total }}</span>
            <button class="secondary compact collapse-button" type="button" @click="toggleDemandPanelCollapsed">
              {{ demandPanelCollapsed ? '展开待生产需求' : '收起待生产需求' }}
            </button>
          </div>
        </div>
        <div v-if="demandPanelCollapsed" class="collapsed-panel-summary">待生产需求已收起，展开后可继续勾选商品。</div>
        <div v-else class="table-wrap drag-scroll-wrap" aria-label="待生产需求横向滚动表格">
          <table class="demand-table">
            <thead>
              <tr>
                <th>选择</th>
                <th>商品</th>
                <th>订单号</th>
                <th>规格(g)</th>
                <th>需求(件)</th>
                <th>需求(g)</th>
                <th>库存(件)</th>
                <th>库存合计(g)</th>
                <th>缺口(g)</th>
                <th>状态</th>
                <th>关联计划</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in stockInsufficientRows" :key="rowKey(row)">
                <td>
                  <input
                    class="bulk-checkbox"
                    type="checkbox"
                    :checked="!!selected[rowKey(row)]"
                    :disabled="!productionDemandSelectable(row)"
                    :title="productionDemandSelectable(row) ? '选择生成生产计划' : '已进入生产计划的需求不可重复生成计划'"
                    @change="toggleInsufficientRow(row, $event.target.checked)"
                  />
                </td>
                <td>{{ row.product }}</td>
                <td class="muted">{{ row.order_nos }}</td>
                <td>{{ row.spec_g }}</td>
                <td>{{ row.need_units }}</td>
                <td>{{ row.need_g }}</td>
                <td>{{ row.inv_units }}</td>
                <td>{{ row.inv_g }}</td>
                <td><strong>{{ row.gap_g }}</strong></td>
                <td><span :class="['status', `status-demand-${productionDemandStatusTone(row.demand_status)}`]">{{ productionDemandStatusLabel(row.demand_status) }}</span></td>
                <td class="muted">{{ row.production_plan_no || '-' }}</td>
              </tr>
              <tr v-if="!stockInsufficientRows.length">
                <td colspan="11" class="muted">暂无待生产需求</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section :class="['panel current-plan-panel', { 'is-collapsed': currentPlanPanelCollapsed }]">
        <div class="panel-head">
          <h2>当前生产计划</h2>
          <div class="panel-head-actions">
            <span v-if="currentPlan" :class="['status', `status-${productionPlanStatusTone(currentPlan.status)}`]">{{ productionPlanStatusLabel(currentPlan.status) }}</span>
            <button class="secondary compact collapse-button" type="button" @click="toggleCurrentPlanPanelCollapsed">
              {{ currentPlanPanelCollapsed ? '展开当前生产计划' : '收起当前生产计划' }}
            </button>
          </div>
        </div>
        <div v-if="currentPlanPanelCollapsed" class="collapsed-panel-summary">当前生产计划已收起，展开后可查看预览、拆分和提交动作。</div>
        <template v-else>
          <div v-if="previewError" class="error">{{ previewError }}</div>
          <div v-if="!hasSelectedRows && !currentPlan" class="empty-state">
            <strong>勾选库存不足商品后生成计划预览</strong>
            <span>在左侧选择要生产的商品后，这里会集中显示 BOM、工艺路线和物料需求。</span>
          </div>
          <template v-else>
            <div v-if="currentPlan" class="ok plan-result">
              <strong>{{ currentPlan.plan_no }}</strong>
              <span>计划行 {{ currentPlan.items?.length || computedPlanRows.length || 0 }} 条</span>
              <span v-if="currentPlan.submitted_at">提交 {{ currentPlan.submitted_at }}</span>
            </div>
            <div v-if="previewLoading" class="muted preview-loading">正在生成计划预览...</div>
            <div v-if="planReady" class="current-plan-content">
              <div>
                <div class="section-title">计划预览（缺口 &gt; 0）</div>
                <div class="table-wrap drag-scroll-wrap" aria-label="计划预览横向滚动表格">
                  <table class="plan-preview-table">
                    <thead>
                      <tr>
                        <th>商品</th>
                        <th>订单号</th>
                        <th>规格(g)</th>
                        <th>需求(g)</th>
                        <th>库存(g)</th>
                        <th>缺口(g)</th>
                        <th>BOM摘要</th>
                        <th>计划投料(g)</th>
                        <th>工艺路线摘要</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="row in computedPlanRows" :key="rowKey(row)">
                        <td>{{ row.product }}</td>
                        <td class="muted">{{ row.order_nos }}</td>
                        <td>{{ row.spec_g }}</td>
                        <td>{{ row.need_g }}</td>
                        <td>{{ row.inv_g }}</td>
                        <td><strong>{{ row.gap_g }}</strong></td>
                        <td>默认 BOM / 预期产出率 {{ percent(row.bom_yield_rate) }}</td>
                        <td>{{ row.input_g }}</td>
                        <td>{{ productionRouteSummary(row) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
              <div>
                <div class="section-title">物料需求汇总（预计消耗）</div>
                <div class="table-wrap drag-scroll-wrap" aria-label="物料需求横向滚动表格">
                  <table class="materials-table">
                    <thead>
                      <tr>
                        <th>物料</th>
                        <th>预计消耗数量</th>
                        <th>单位</th>
                        <th>WIP可用(g)</th>
                        <th>建议领到WIP(g)</th>
                        <th>原料仓(g)</th>
                        <th>采购建议(g)</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="item in computedMaterials" :key="`${item.name}-${item.unit}`">
                        <td>{{ item.name }}</td>
                        <td>{{ item.qty }}</td>
                        <td>{{ item.unit }}</td>
                        <td>{{ item.available_g || 0 }}</td>
                        <td><strong>{{ item.wip_transfer_suggestion_g || 0 }}</strong></td>
                        <td>{{ item.raw_g || 0 }}</td>
                        <td>{{ item.purchase_suggestion_g || 0 }}</td>
                      </tr>
                      <tr v-if="!computedMaterials.length">
                        <td colspan="7" class="muted">暂无物料汇总</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
              <div v-if="!currentPlan" class="operation-split-placeholder">
                <div class="section-title">工序产能拆分</div>
                <strong>创建草稿生产计划后可填写工序产能拆分</strong>
                <span>先点创建生产计划，生成草稿后再选择工位产能和承担产量；拆分会在提交生成工单前保存。</span>
              </div>
              <div v-if="currentPlan" class="operation-split-panel">
                <div class="panel-head compact-head">
                  <div class="section-title">工序产能拆分</div>
                  <button class="secondary compact" type="button" @click="saveCurrentPlanOperationSplits" :disabled="saving || !currentPlanDraft">保存拆分</button>
                </div>
                <div v-if="operationSplitError" class="error">{{ operationSplitError }}</div>
                <div v-if="!currentPlanDraft" class="muted section-hint">已提交计划只展示已冻结拆分。</div>
                <div v-for="row in currentPlanOperationRows" :key="`${row.item.id}-${row.operation.seq || row.operation.sequence_no || row.operation.operation}`" class="split-operation-block">
                  <div class="split-operation-head">
                    <strong>{{ row.item.product_name || '-' }}</strong>
                    <span>{{ row.operation.seq || row.operation.sequence_no || '-' }}. {{ row.operation.operation || '工序' }}</span>
                    <button class="secondary compact" type="button" @click="addOperationSplit(row.item, row.operation)" :disabled="!currentPlanDraft">添加拆分</button>
                  </div>
                  <div class="split-row" v-for="(split, splitIndex) in splitRowsForOperation(row.item, row.operation)" :key="split.local_key || split.id || `${row.item.id}-${splitIndex}`">
                    <label>
                      <span>工位产能</span>
                      <select v-model.number="split.workstation_capacity_id" :disabled="!currentPlanDraft" @change="applySplitCapacity(split)">
                        <option value="0">选择工位产能，例如 布勒 18kg / 智烘 4kg</option>
                        <option v-for="capacity in activeWorkstationCapacities" :key="capacity.id" :value="capacity.id">{{ capacityOptionLabel(capacity) }}</option>
                      </select>
                    </label>
                    <label>
                      <span>承担产量{{ splitQuantityUnit(split) }}</span>
                      <input v-model.number="split.planned_qty" type="number" min="0" :step="splitQuantityStep(split)" :disabled="!currentPlanDraft" />
                    </label>
                    <div class="split-metric">
                      <span>自动批次数</span>
                      <strong>{{ plannedCapacitySplitMetrics(split).planned_batch_count || 0 }}</strong>
                    </div>
                    <div class="split-metric">
                      <span>计划数量</span>
                      <strong>{{ plannedCapacitySplitMetrics(split).planned_qty_g || 0 }}g</strong>
                    </div>
                    <div class="split-metric">
                      <span>计划分钟</span>
                      <strong>{{ plannedCapacitySplitMetrics(split).planned_minutes || 0 }}</strong>
                    </div>
                    <div class="split-metric">
                      <span>计划工序成本</span>
                      <strong>{{ plannedCapacitySplitMetrics(split).planned_operation_cost || 0 }}</strong>
                    </div>
                    <button class="secondary compact danger-text" type="button" @click="removeOperationSplit(split)" :disabled="!currentPlanDraft">删除</button>
                    <div v-if="splitBatchCards(split).length" class="split-batch-cards" aria-label="自动批次卡片">
                      <div
                        v-for="batch in splitBatchCards(split)"
                        :key="`${split.local_key || split.id || splitIndex}-${batch.label}`"
                        class="split-batch-card"
                        :class="{ underfilled: batch.underfilled }"
                      >
                        <strong>{{ batch.label }}</strong>
                        <span>{{ batch.workstation_capacity_name || split.workstation_capacity_name || '工位产能' }}</span>
                        <small>单批标准 {{ splitQtyText(batch.batch_size_qty, batch.batch_size_unit) }}</small>
                        <small>本批计划 {{ splitQtyText(batch.planned_qty, batch.batch_size_unit) }}</small>
                        <small>计划分钟 {{ batch.planned_minutes || 0 }}</small>
                        <em v-if="batch.underfilled">不足标准批量</em>
                      </div>
                    </div>
                  </div>
                  <div v-if="!splitRowsForOperation(row.item, row.operation).length" class="muted section-hint">暂无拆分</div>
                </div>
                <div v-if="!currentPlanOperationRows.length" class="muted section-hint">暂无工序快照</div>
              </div>
            </div>
            <div v-else-if="hasSelectedRows && !previewLoading" class="muted empty-state">已选择商品，等待计划预览。</div>
            <div class="actions current-plan-actions">
              <button v-if="!currentPlan" class="primary" type="button" @click="createProductionPlan" :disabled="saving || previewLoading || !planReady">创建生产计划</button>
              <button v-else class="primary" type="button" @click="submitCurrentProductionPlan" :disabled="saving || !currentPlanDraft">提交当前计划生成工单</button>
              <span v-if="currentPlan && !currentPlanDraft" class="muted">当前计划状态为 {{ productionPlanStatusLabel(currentPlan.status) }}，无需重复提交。</span>
            </div>
          </template>
        </template>
      </section>
    </section>

    <section class="panel">
      <div class="section-title">库存充足（只提示）</div>
      <div class="muted section-hint">以下订单已有成品库存覆盖，不进入生产计划；录单时确认使用批次后会进入库存待发货。</div>
      <div class="table-wrap drag-scroll-wrap" aria-label="库存充足商品横向滚动表格">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>订单号</th>
              <th>规格(g)</th>
              <th>需求(件)</th>
              <th>需求(g)</th>
              <th>库存(件)</th>
              <th>库存散装(g)</th>
              <th>库存合计(g)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in stockSufficientRows" :key="rowKey(row)">
              <td>{{ row.product }}</td>
              <td class="muted">{{ row.order_nos }}</td>
              <td>{{ row.spec_g }}</td>
              <td>{{ row.need_units }}</td>
              <td>{{ row.need_g }}</td>
              <td>{{ row.inv_units }}</td>
              <td>{{ row.inv_loose_g }}</td>
              <td>{{ row.inv_g }}</td>
            </tr>
            <tr v-if="!stockSufficientRows.length">
              <td colspan="8" class="muted">暂无库存充足商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2>生产计划单据</h2>
        <button class="secondary" type="button" @click="loadProductionPlans" :disabled="loading">刷新单据</button>
      </div>
      <div class="filters production-plan-filters">
        <label>
          <span>状态</span>
          <select v-model="productionPlanFilters.status" @change="loadProductionPlans">
            <option value="">全部</option>
            <option value="draft">草稿</option>
            <option value="submitted">已提交工单</option>
            <option value="in_progress">生产中</option>
            <option value="completed">已完成</option>
            <option value="cancelled">已取消</option>
          </select>
        </label>
        <label>
          <span>时间类型</span>
          <select v-model="productionPlanFilters.time_field" @change="loadProductionPlans">
            <option value="created_at">创建时间</option>
            <option value="submitted_at">提交时间</option>
            <option value="completed_at">完成时间</option>
          </select>
        </label>
        <label>
          <span>开始日期</span>
          <input v-model.trim="productionPlanFilters.from" type="date" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="productionPlanFilters.to" type="date" />
        </label>
        <button class="secondary filter-action" type="button" @click="loadProductionPlans" :disabled="loading">过滤</button>
      </div>
      <div class="actions plan-list-actions">
        <button class="primary" type="button" @click="submitSelectedProductionPlans" :disabled="saving || !hasSelectedProductionPlans">提交生成工单</button>
      </div>
      <div class="table-wrap drag-scroll-wrap" aria-label="生产计划单据横向滚动表格">
        <table>
          <thead>
            <tr>
              <th>
                <input
                  ref="productionPlanHeaderCheckbox"
                  class="bulk-checkbox"
                  type="checkbox"
                  :checked="allProductionPlansSelected"
                  :disabled="!productionPlanSelection.total"
                  :aria-checked="productionPlanSelection.indeterminate ? 'mixed' : String(productionPlanSelection.checked)"
                  aria-label="全选草稿生产计划"
                  @change="toggleAllProductionPlans($event.target.checked)"
                />
              </th>
              <th>计划号</th>
              <th>来源</th>
              <th>状态</th>
              <th>行数</th>
              <th>创建人</th>
              <th>提交人</th>
              <th>时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="plan in productionPlans" :key="plan.id">
              <td>
                <input
                  class="bulk-checkbox"
                  type="checkbox"
                  :checked="!!selectedProductionPlans[String(plan.id)]"
                  :disabled="!productionPlanSelectable(plan)"
                  :aria-label="`选择生产计划 ${plan.plan_no}`"
                  @change="toggleProductionPlan(plan, $event.target.checked)"
                />
              </td>
              <td><button class="link-button plan-no-button" type="button" @click="openProductionPlanDetail(plan)">{{ plan.plan_no }}</button></td>
              <td>{{ plan.source_type || '-' }}</td>
              <td><span :class="['status', `status-${productionPlanStatusTone(plan.status)}`]">{{ productionPlanStatusLabel(plan.status) }}</span></td>
              <td>{{ plan.item_count || 0 }}</td>
              <td>{{ plan.created_by || '-' }}</td>
              <td>{{ plan.submitted_by || '-' }}</td>
              <td><small>建 {{ plan.created_at || '-' }}</small><small>交 {{ plan.submitted_at || '-' }}</small><small>完 {{ plan.completed_at || '-' }}</small></td>
              <td><button class="secondary compact" type="button" @click="openProductionPlanDetail(plan)">详情</button></td>
            </tr>
            <tr v-if="!productionPlans.length">
              <td colspan="9" class="muted">暂无生产计划单据</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <div v-if="productionPlanDetail" class="drawer-backdrop" @click.self="closeProductionPlanDetail">
      <aside class="production-plan-detail-drawer" aria-label="生产计划单据详情">
        <div class="drawer-head">
          <div>
            <div class="muted">生产计划单据</div>
            <h2>{{ productionPlanDetail.plan_no || '-' }}</h2>
          </div>
          <div class="drawer-head-actions">
            <span :class="['status', `status-${productionPlanStatusTone(productionPlanDetail.status)}`]">{{ productionPlanStatusLabel(productionPlanDetail.status) }}</span>
            <button class="secondary compact" type="button" @click="closeProductionPlanDetail">关闭</button>
          </div>
        </div>
        <div v-if="productionPlanDetailError" class="error">{{ productionPlanDetailError }}</div>
        <div v-if="productionPlanDetailLoading" class="muted drawer-loading">正在加载单据详情...</div>
        <template v-else>
          <section class="detail-section">
            <div class="section-title">单据头</div>
            <dl class="detail-grid">
              <div><dt>计划号</dt><dd>{{ productionPlanDetail.plan_no || '-' }}</dd></div>
              <div><dt>来源</dt><dd>{{ productionPlanDetail.source_type || '-' }}</dd></div>
              <div><dt>状态</dt><dd>{{ productionPlanStatusLabel(productionPlanDetail.status) }}</dd></div>
              <div><dt>创建</dt><dd>{{ productionPlanDetail.created_by || '-' }} / {{ productionPlanDetail.created_at || '-' }}</dd></div>
              <div><dt>提交</dt><dd>{{ productionPlanDetail.submitted_by || '-' }} / {{ productionPlanDetail.submitted_at || '-' }}</dd></div>
              <div><dt>完成</dt><dd>{{ productionPlanDetail.completed_at || '-' }}</dd></div>
            </dl>
          </section>

          <section class="detail-section">
            <div class="section-title">计划行</div>
            <div class="table-wrap drag-scroll-wrap" aria-label="计划行横向滚动表格">
              <table class="detail-table">
                <thead>
                  <tr>
                    <th>商品</th>
                    <th>来源订单</th>
                    <th>规格/单位</th>
                    <th>生产缺口</th>
                    <th>计划投料</th>
                    <th>计划产出</th>
                    <th>BOM</th>
                    <th>工艺路线摘要</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in productionPlanDetail.items || []" :key="item.id">
                    <td>{{ item.product_name || '-' }}</td>
                    <td class="muted">{{ item.order_nos || '-' }}</td>
                    <td>{{ planItemSpecLabel(item) }}</td>
                    <td>{{ item.gap_g || 0 }}</td>
                    <td>{{ item.planned_g || 0 }}</td>
                    <td>{{ item.planned_output_g || 0 }}</td>
                    <td>{{ planItemBomLabel(item) }}</td>
                    <td>{{ planItemRouteSummary(item) }}</td>
                  </tr>
                  <tr v-if="!(productionPlanDetail.items || []).length">
                    <td colspan="8" class="muted">暂无计划行</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-title">物料需求汇总</div>
            <div class="table-wrap drag-scroll-wrap" aria-label="物料需求汇总横向滚动表格">
              <table class="detail-table compact-table">
                <thead>
                  <tr>
                    <th>物料</th>
                    <th>需求数量</th>
                    <th>单位</th>
                    <th>类型</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in productionPlanDetail.material_summary || []" :key="`${item.name}-${item.unit}-${item.component_type || ''}`">
                    <td>{{ item.name }}</td>
                    <td>{{ item.qty }}</td>
                    <td>{{ item.unit || '-' }}</td>
                    <td>{{ materialTypeLabel(item) }}</td>
                  </tr>
                  <tr v-if="!(productionPlanDetail.material_summary || []).length">
                    <td colspan="4" class="muted">暂无物料需求汇总</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-title">工艺路线摘要</div>
            <div v-for="item in productionPlanDetail.items || []" :key="`route-${item.id}`" class="route-block">
              <div class="route-title">{{ item.product_name || '-' }} · {{ planItemRouteSummary(item) }}</div>
              <div v-if="processOperations(item).length" class="operation-list">
                <span v-for="op in processOperations(item)" :key="`${item.id}-${op.seq || op.sequence_no || op.operation}`" class="operation-pill">
                  {{ op.seq || op.sequence_no || '-' }}. {{ op.operation || '工序' }} / {{ op.workstation || '工位' }}
                </span>
              </div>
              <div v-else class="muted">暂无工序快照</div>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-title">工艺参数 / 商品生产配置快照</div>
            <div v-for="item in productionPlanDetail.items || []" :key="`config-${item.id}`" class="route-block">
              <div class="route-title">{{ item.product_name || '-' }}</div>
              <div v-if="productionConfigFields(item).length" class="operation-list">
                <span v-for="field in productionConfigFields(item)" :key="`${item.id}-${field.field_key || field.label}`" class="operation-pill">
                  {{ field.label || field.field_key }}：{{ productionConfigValue(field) }}
                </span>
              </div>
              <div v-else class="muted">暂无商品生产配置快照</div>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-title">生成结果</div>
            <div class="result-summary">
              <span>工单 {{ (productionPlanDetail.related_work_orders || []).length }} 张</span>
              <span>工序卡 {{ productionPlanDetail.job_card_count || 0 }} 张</span>
              <button class="secondary compact" type="button" @click="navigateProductionView('workOrders')">进入工单</button>
              <button class="secondary compact" type="button" @click="navigateProductionView('jobCards')">进入工序卡</button>
            </div>
            <div class="table-wrap drag-scroll-wrap" aria-label="生成结果横向滚动表格">
              <table class="detail-table">
                <thead>
                  <tr>
                    <th>工单号</th>
                    <th>商品</th>
                    <th>计划投料</th>
                    <th>计划产出</th>
                    <th>状态</th>
                    <th>工序卡</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="wo in productionPlanDetail.related_work_orders || []" :key="wo.id">
                    <td>{{ wo.work_order_no }}</td>
                    <td>{{ wo.product_name || '-' }}</td>
                    <td>{{ wo.planned_g || 0 }}</td>
                    <td>{{ wo.planned_output_g || 0 }}</td>
                    <td>{{ workOrderStatusLabel(wo.status) }}</td>
                    <td>{{ wo.job_card_count || 0 }}</td>
                  </tr>
                  <tr v-if="!(productionPlanDetail.related_work_orders || []).length">
                    <td colspan="6" class="muted">尚未生成工单</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </template>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch, watchEffect } from 'vue'
import { apiGet, apiSend } from '../api/client'
import {
  buildCurrentProductionPlanSubmitPayload,
  buildInsufficientSelection,
  buildProductionPlanBatchSubmitPayload,
  buildProductionPlanCreatePayload,
  buildProductionPlanListQuery,
  buildProductionPlanOperationSplitPayload,
  buildProductionPlanSelection,
  buildProductionDemandSelection,
  buildProductionDemandSummaryQuery,
  plannedCapacitySplitMetrics,
  productionDemandSelectable,
  productionDemandSelectionState,
  productionDemandStatusLabel,
  productionDemandStatusOptions,
  productionDemandStatusTone,
  productionPlanSplitBatchCards,
  productionPlanBatchSubmitEndpoint,
  productionPlanDetailEndpoint,
  productionPlanOperationSplitsEndpoint,
  productionPlanSelectable,
  productionPlanSelectionState,
  productionPlanStatusLabel,
  productionPlanStatusTone,
  producePlanKey,
} from '../lib/produce-plan'
import { replaceHistoryURL } from '../lib/url-state'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const loading = ref(false)
const saving = ref(false)
const previewLoading = ref(false)
const error = ref('')
const previewError = ref('')
const stockTip = ref('')
const rows = ref([])
const planRows = ref([])
const initialMaterials = ref([])
const productionPlans = ref([])
const currentPlan = ref(null)
const workstationCapacities = ref([])
const operationSplits = ref([])
const productionPlanDetail = ref(null)
const productionPlanDetailLoading = ref(false)
const productionPlanDetailError = ref('')
const operationSplitError = ref('')
const insufficientHeaderCheckbox = ref(null)
const productionPlanHeaderCheckbox = ref(null)
const demandPanelCollapsed = ref(false)
const currentPlanPanelCollapsed = ref(false)
const selected = reactive({})
const selectedProductionPlans = reactive({})
let previewTimer = 0
let previewRequestSeq = 0
const tableScrollDrag = {
  element: null,
  pointerId: 0,
  startX: 0,
  startY: 0,
  scrollLeft: 0,
  scrollTop: 0,
  dragging: false,
}

const filters = reactive({
  from: '',
  to: '',
  customer_id: '',
  demand_status: '',
})

const demandStatusOptions = productionDemandStatusOptions()
const demandStatusFilter = computed({
  get: () => filters.demand_status,
  set: (value) => { filters.demand_status = value },
})

const productionPlanFilters = reactive({
  status: '',
  time_field: 'created_at',
  from: '',
  to: '',
  limit: 50,
})

function rowKey(row) {
  return producePlanKey(row.product_id, row.spec_g)
}

function percent(v) {
  return `${(Number(v || 0) * 100).toFixed(2)}%`
}

const planReady = computed(() => planRows.value.length > 0)
const computedPlanRows = computed(() => planRows.value || [])
const computedMaterials = computed(() => initialMaterials.value || [])
const hasSelectedRows = computed(() => selectedKeys().length > 0)
const stockInsufficientRows = computed(() => rows.value.filter((row) => Number(row.gap_g || 0) > 0 || String(row.demand_status || 'unplanned') !== 'unplanned'))
const stockSufficientRows = computed(() => rows.value.filter((row) => Number(row.gap_g || 0) <= 0 && String(row.demand_status || 'unplanned') === 'unplanned'))
const insufficientSelection = computed(() => productionDemandSelectionState(stockInsufficientRows.value, selected))
const allInsufficientSelected = computed(() => insufficientSelection.value.checked)
const productionPlanSelection = computed(() => productionPlanSelectionState(productionPlans.value, selectedProductionPlans))
const allProductionPlansSelected = computed(() => productionPlanSelection.value.checked)
const hasSelectedProductionPlans = computed(() => productionPlanSelection.value.selectedCount > 0)
const currentPlanDraft = computed(() => productionPlanSelectable(currentPlan.value))
const selectedSignature = computed(() => selectedKeys().join('|'))
const activeWorkstationCapacities = computed(() => workstationCapacities.value.filter((row) => String(row.status || 'active') === 'active'))
const currentPlanOperationRows = computed(() => {
  const rows = []
  for (const item of currentPlan.value?.items || []) {
    for (const operation of processOperations(item)) {
      rows.push({ item, operation })
    }
  }
  return rows
})

watchEffect(() => {
  if (insufficientHeaderCheckbox.value) {
    insufficientHeaderCheckbox.value.indeterminate = insufficientSelection.value.indeterminate
  }
  if (productionPlanHeaderCheckbox.value) {
    productionPlanHeaderCheckbox.value.indeterminate = productionPlanSelection.value.indeterminate
  }
})

function selectedKeys() {
  return Object.keys(selected).filter((key) => selected[key])
}

function replaceSelected(nextSelected) {
  Object.keys(selected).forEach((key) => delete selected[key])
  Object.assign(selected, nextSelected)
}

function replaceSelectedProductionPlans(nextSelected) {
  Object.keys(selectedProductionPlans).forEach((key) => delete selectedProductionPlans[key])
  Object.assign(selectedProductionPlans, nextSelected)
}

function updateUrl(plan) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'producePlan')
  if (filters.from) url.searchParams.set('from', filters.from)
  else url.searchParams.delete('from')
  if (filters.to) url.searchParams.set('to', filters.to)
  else url.searchParams.delete('to')
  if (filters.customer_id) url.searchParams.set('customer_id', filters.customer_id)
  else url.searchParams.delete('customer_id')
  if (filters.demand_status) url.searchParams.set('demand_status', filters.demand_status)
  else url.searchParams.delete('demand_status')
  const keys = selectedKeys()
  if (plan && keys.length) {
    url.searchParams.set('plan', '1')
    url.searchParams.set('selected', keys.join(','))
  } else {
    url.searchParams.delete('plan')
    url.searchParams.delete('selected')
  }
  replaceHistoryURL(url)
}

function buildUnproducedURL(plan, keys = selectedKeys()) {
  return new URL(buildProductionDemandSummaryQuery(filters, plan, keys), window.location.origin)
}

function applyUnproducedData(data, plan) {
  rows.value = data.rows || []
  stockTip.value = data.stock_tip || ''
  planRows.value = data.plan_rows || []
  initialMaterials.value = data.materials || []
  if (data.selected) {
    Object.keys(selected).forEach((key) => delete selected[key])
    for (const key of Object.keys(data.selected)) {
      if (data.selected[key]) selected[key] = true
    }
  }
  pruneSufficientSelections()
  updateUrl(plan)
}

async function load(plan) {
  loading.value = true
  error.value = ''
  previewError.value = ''
  try {
    const data = await apiGet(buildUnproducedURL(plan))
    applyUnproducedData(data, plan)
    if (!plan) currentPlan.value = null
    if (!plan && selectedKeys().length) schedulePlanPreview()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadSelectedPlanPreview() {
  const keys = selectedKeys()
  previewError.value = ''
  if (!keys.length) {
    planRows.value = []
    initialMaterials.value = []
    updateUrl(false)
    return
  }
  const requestID = ++previewRequestSeq
  previewLoading.value = true
  try {
    const data = await apiGet(buildUnproducedURL(true, keys))
    if (requestID !== previewRequestSeq) return
    applyUnproducedData(data, true)
  } catch (err) {
    if (requestID !== previewRequestSeq) return
    planRows.value = []
    initialMaterials.value = []
    previewError.value = err.message || '生成计划预览失败'
  } finally {
    if (requestID === previewRequestSeq) previewLoading.value = false
  }
}

function schedulePlanPreview() {
  if (previewTimer) clearTimeout(previewTimer)
  previewTimer = setTimeout(() => {
    previewTimer = 0
    loadSelectedPlanPreview()
  }, 250)
}

async function loadProductionPlans() {
  try {
    const data = await apiGet(buildProductionPlanListQuery(productionPlanFilters))
    productionPlans.value = data.rows || []
    pruneProductionPlanSelections()
  } catch (err) {
    error.value = err.message || '加载生产计划失败'
  }
}

function resetCurrentPlanForSelection() {
  currentPlan.value = null
  operationSplits.value = []
  operationSplitError.value = ''
  previewError.value = ''
}

function toggleAllInsufficient(checked) {
  replaceSelected(buildProductionDemandSelection(stockInsufficientRows.value, checked))
  resetCurrentPlanForSelection()
}

function toggleInsufficientRow(row, checked) {
  const key = rowKey(row)
  if (!productionDemandSelectable(row)) {
    delete selected[key]
    return
  }
  if (checked) selected[key] = true
  else delete selected[key]
  resetCurrentPlanForSelection()
}

function toggleDemandPanelCollapsed() {
  demandPanelCollapsed.value = !demandPanelCollapsed.value
  if (demandPanelCollapsed.value && currentPlanPanelCollapsed.value) {
    currentPlanPanelCollapsed.value = false
  }
}

function toggleCurrentPlanPanelCollapsed() {
  currentPlanPanelCollapsed.value = !currentPlanPanelCollapsed.value
  if (currentPlanPanelCollapsed.value && demandPanelCollapsed.value) {
    demandPanelCollapsed.value = false
  }
}

function startTableScrollDrag(event) {
  if (event.button !== 0) return
  const target = event.target
  const interactive = target?.closest?.('button,input,select,textarea,a,label')
  if (interactive) return
  const wrap = target?.closest?.('.drag-scroll-wrap')
  if (!wrap || (wrap.scrollWidth <= wrap.clientWidth && wrap.scrollHeight <= wrap.clientHeight)) return
  tableScrollDrag.element = wrap
  tableScrollDrag.pointerId = event.pointerId
  tableScrollDrag.startX = event.clientX
  tableScrollDrag.startY = event.clientY
  tableScrollDrag.scrollLeft = wrap.scrollLeft
  tableScrollDrag.scrollTop = wrap.scrollTop
  tableScrollDrag.dragging = false
  wrap.classList.add('is-dragging-scroll')
  wrap.setPointerCapture?.(event.pointerId)
}

function moveTableScrollDrag(event) {
  const wrap = tableScrollDrag.element
  if (!wrap || event.pointerId !== tableScrollDrag.pointerId) return
  const deltaX = event.clientX - tableScrollDrag.startX
  const deltaY = event.clientY - tableScrollDrag.startY
  if (Math.abs(deltaX) > 3 || Math.abs(deltaY) > 3) tableScrollDrag.dragging = true
  if (!tableScrollDrag.dragging) return
  wrap.scrollLeft = tableScrollDrag.scrollLeft - deltaX
  wrap.scrollTop = tableScrollDrag.scrollTop - deltaY
  event.preventDefault()
}

function stopTableScrollDrag(event) {
  const wrap = tableScrollDrag.element
  if (!wrap) return
  wrap.classList.remove('is-dragging-scroll')
  if (event?.pointerId === tableScrollDrag.pointerId) {
    wrap.releasePointerCapture?.(event.pointerId)
  }
  tableScrollDrag.element = null
  tableScrollDrag.pointerId = 0
  tableScrollDrag.dragging = false
}

function toggleAllProductionPlans(checked) {
  replaceSelectedProductionPlans(buildProductionPlanSelection(productionPlans.value, checked))
}

function toggleProductionPlan(plan, checked) {
  const key = String(Number(plan?.id || 0))
  if (!productionPlanSelectable(plan)) {
    delete selectedProductionPlans[key]
    return
  }
  if (checked) selectedProductionPlans[key] = true
  else delete selectedProductionPlans[key]
}

function pruneSufficientSelections() {
  const allowed = new Set(stockInsufficientRows.value.filter(productionDemandSelectable).map((row) => rowKey(row)))
  for (const key of Object.keys(selected)) {
    if (!allowed.has(key)) delete selected[key]
  }
}

function pruneProductionPlanSelections() {
  const allowed = new Set(productionPlans.value.filter(productionPlanSelectable).map((plan) => String(Number(plan.id))))
  for (const key of Object.keys(selectedProductionPlans)) {
    if (!allowed.has(key)) delete selectedProductionPlans[key]
  }
}

function productionRouteSummary(row) {
  const templateID = Number(row?.operation_template_id || 0)
  if (templateID > 0) return `工艺模板 #${templateID}`
  return '按商品默认工艺路线'
}

function parseJSONSnapshot(raw, fallback) {
  if (raw && typeof raw === 'object') return raw
  const text = String(raw || '').trim()
  if (!text) return fallback
  try {
    return JSON.parse(text)
  } catch (_) {
    return fallback
  }
}

function planItemSpecLabel(item) {
  const spec = Number(item?.spec_g || 0)
  if (spec > 0) return `${spec}g`
  return '按商品单位'
}

function planItemBomLabel(item) {
  const id = Number(item?.bom_version_id || 0)
  return id > 0 ? `BOM版本 #${id}` : '默认 BOM'
}

function planItemRouteSummary(item) {
  const snapshot = parseJSONSnapshot(item?.process_snapshot_json, {})
  const name = snapshot.route_name || snapshot.name || ''
  if (name) return name
  const routeID = Number(item?.process_route_id || 0)
  if (routeID > 0) return `工艺路线 #${routeID}`
  const templateID = Number(item?.operation_template_id || 0)
  if (templateID > 0) return `工艺模板 #${templateID}`
  return '按商品默认工艺路线'
}

function processOperations(item) {
  const snapshot = parseJSONSnapshot(item?.process_snapshot_json, {})
  return Array.isArray(snapshot.operations) ? snapshot.operations : []
}

function operationIdentity(operation) {
  return {
    seq: Number(operation?.seq || operation?.sequence_no || 0),
    id: Number(operation?.operation_id || 0),
    name: String(operation?.operation || '').trim(),
  }
}

function splitMatchesOperation(split, operation) {
  const identity = operationIdentity(operation)
  if (identity.seq > 0 && Number(split?.operation_seq || 0) === identity.seq) return true
  if (identity.id > 0 && Number(split?.operation_id || 0) === identity.id) return true
  return !identity.seq && identity.name && String(split?.operation || '').trim() === identity.name
}

function splitRowsForOperation(item, operation) {
  const itemID = Number(item?.id || 0)
  return operationSplits.value.filter((split) => Number(split.production_plan_item_id || 0) === itemID && splitMatchesOperation(split, operation))
}

function splitQuantityUnit(split) {
  const unit = String(split?.batch_size_unit || '').trim()
  return unit ? `（${unit}）` : ''
}

function splitQuantityStep(split) {
  const unit = String(split?.batch_size_unit || '').trim().toLowerCase()
  if (unit === 'g' || unit === '克') return '1'
  return '0.001'
}

function splitQtyText(qty, unit) {
  const n = Math.max(0, Number(qty || 0))
  const value = n ? n.toLocaleString('zh-CN', { maximumFractionDigits: 3 }) : '0'
  return `${value}${String(unit || '').trim()}`
}

function splitBatchCards(split) {
  return productionPlanSplitBatchCards(split)
}

function qtyFromGForSplitUnit(qtyG, unit) {
  const value = Math.max(0, Number(qtyG || 0))
  const normalized = String(unit || '').trim().toLowerCase()
  if (normalized === 'kg' || normalized === '千克' || normalized === '公斤') return Number((value / 1000).toFixed(3))
  if (normalized === 'g' || normalized === '克') return Math.round(value)
  return 0
}

function productionPlanItemTargetG(item) {
  return Math.max(0, Number(item?.planned_g || item?.planned_output_g || item?.gap_g || 0))
}

function currentPlanItemForSplit(split) {
  const itemID = Number(split?.production_plan_item_id || 0)
  return (currentPlan.value?.items || []).find((item) => Number(item?.id || 0) === itemID) || null
}

function splitSameOperation(left, right) {
  if (Number(left?.production_plan_item_id || 0) !== Number(right?.production_plan_item_id || 0)) return false
  const leftSeq = Number(left?.operation_seq || 0)
  const rightSeq = Number(right?.operation_seq || 0)
  if (leftSeq > 0 || rightSeq > 0) return leftSeq === rightSeq
  const leftID = Number(left?.operation_id || 0)
  const rightID = Number(right?.operation_id || 0)
  if (leftID > 0 || rightID > 0) return leftID === rightID
  return String(left?.operation || '').trim() === String(right?.operation || '').trim()
}

function defaultPlannedQtyForSplit(split) {
  const item = currentPlanItemForSplit(split)
  const targetG = productionPlanItemTargetG(item)
  if (targetG <= 0) return 0
  const usedG = operationSplits.value.reduce((sum, row) => {
    if (row === split || !splitSameOperation(row, split)) return sum
    return sum + (plannedCapacitySplitMetrics(row).planned_qty_g || 0)
  }, 0)
  return qtyFromGForSplitUnit(Math.max(0, targetG - usedG), split.batch_size_unit)
}

function capacityOptionLabel(capacity) {
  const parts = [capacity?.name || `#${capacity?.id || ''}`]
  if (Number(capacity?.batch_size_qty || 0) > 0) parts.push(`${capacity.batch_size_qty}${capacity.batch_size_unit || ''}`)
  if (Number(capacity?.standard_minutes || 0) > 0) parts.push(`${capacity.standard_minutes}分钟/批`)
  if (Number(capacity?.hourly_rate || 0) > 0) parts.push(`${capacity.hourly_rate}/小时`)
  return parts.filter(Boolean).join(' · ')
}

function normalizeOperationSplit(row = {}) {
  return {
    local_key: row.local_key || `split-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    id: Number(row.id || 0),
    production_plan_id: Number(row.production_plan_id || currentPlan.value?.id || 0),
    production_plan_item_id: Number(row.production_plan_item_id || 0),
    operation_seq: Number(row.operation_seq || 0),
    operation_id: Number(row.operation_id || 0),
    operation: row.operation || '',
    workstation_id: Number(row.workstation_id || 0),
    workstation: row.workstation || '',
    workstation_capacity_id: Number(row.workstation_capacity_id || 0),
    workstation_capacity_name: row.workstation_capacity_name || '',
    batch_size_qty: Number(row.batch_size_qty || 0),
    batch_size_unit: row.batch_size_unit || '',
    standard_minutes: Number(row.standard_minutes || 0),
    hourly_rate: Number(row.hourly_rate || 0),
    planned_batch_count: Number(row.planned_batch_count || 0),
    planned_qty: Number(row.planned_qty || qtyFromGForSplitUnit(Number(row.planned_qty_g || 0), row.batch_size_unit)),
    planned_qty_g: Number(row.planned_qty_g || 0),
    planned_minutes: Number(row.planned_minutes || 0),
    planned_operation_cost: Number(row.planned_operation_cost || 0),
    note: row.note || '',
  }
}

function addOperationSplit(item, operation) {
  const identity = operationIdentity(operation)
  operationSplits.value.push(normalizeOperationSplit({
    production_plan_id: currentPlan.value?.id || 0,
    production_plan_item_id: item?.id || 0,
    operation_seq: identity.seq,
    operation_id: identity.id,
    operation: identity.name,
    planned_qty: 0,
  }))
}

function removeOperationSplit(split) {
  operationSplits.value = operationSplits.value.filter((row) => row !== split)
}

function applySplitCapacity(split) {
  const capacity = workstationCapacities.value.find((row) => Number(row.id || 0) === Number(split.workstation_capacity_id || 0))
  if (!capacity) return
  split.workstation_id = Number(capacity.workstation_id || 0)
  split.workstation = capacity.workstation || ''
  split.workstation_capacity_name = capacity.name || ''
  split.batch_size_qty = Number(capacity.batch_size_qty || 0)
  split.batch_size_unit = capacity.batch_size_unit || ''
  split.standard_minutes = Number(capacity.standard_minutes || 0)
  split.hourly_rate = Number(capacity.hourly_rate || 0)
  if (Number(split.planned_qty || 0) <= 0) {
    split.planned_qty = defaultPlannedQtyForSplit(split)
  }
}

async function loadWorkstationCapacities() {
  const data = await apiGet('/api/manufacturing-workstation-capacities')
  workstationCapacities.value = data.rows || []
}

async function loadProductionPlanOperationSplits(plan = currentPlan.value) {
  if (!productionPlanOperationSplitsEndpoint(plan)) {
    operationSplits.value = []
    return
  }
  const data = await apiGet(productionPlanOperationSplitsEndpoint(plan))
  operationSplits.value = (data.rows || []).map(normalizeOperationSplit)
}

async function saveCurrentPlanOperationSplits() {
  if (!productionPlanOperationSplitsEndpoint(currentPlan.value) || !currentPlanDraft.value) return
  operationSplitError.value = ''
  const payload = buildProductionPlanOperationSplitPayload(operationSplits.value)
  const data = await apiSend(productionPlanOperationSplitsEndpoint(currentPlan.value), { body: payload })
  operationSplits.value = (data.rows || []).map(normalizeOperationSplit)
}

function productionConfigFields(item) {
  const snapshot = parseJSONSnapshot(item?.production_config_snapshot_json, {})
  return Array.isArray(snapshot.fields) ? snapshot.fields : []
}

function productionConfigValue(field) {
  if (field?.value_text) return field.value_text
  if (field?.value_number !== undefined && field.value_number !== null) return field.value_number
  if (field?.value_bool !== undefined && field.value_bool !== null) return field.value_bool ? '是' : '否'
  return '-'
}

function materialTypeLabel(item) {
  const type = String(item?.component_type || '').trim()
  if (type === 'finished_product') return '半成品/成品组件'
  if (type === 'packaging') return '包材'
  if (type === 'material' || type === 'bom') return '物料'
  return type || '-'
}

function workOrderStatusLabel(status) {
  const map = {
    draft: '草稿',
    released: '待开工',
    running: '生产中',
    completed: '已完成',
    cancelled: '已取消',
  }
  return map[String(status || '').trim()] || status || '-'
}

async function openProductionPlanDetail(plan) {
  if (!productionPlanDetailEndpoint(plan)) return
  productionPlanDetail.value = { ...plan, items: [], material_summary: [], related_work_orders: [], job_card_count: 0 }
  productionPlanDetailLoading.value = true
  productionPlanDetailError.value = ''
  try {
    productionPlanDetail.value = await apiGet(productionPlanDetailEndpoint(plan))
  } catch (err) {
    productionPlanDetailError.value = err.message || '加载生产计划单据详情失败'
  } finally {
    productionPlanDetailLoading.value = false
  }
}

function closeProductionPlanDetail() {
  productionPlanDetail.value = null
  productionPlanDetailError.value = ''
}

function navigateProductionView(key) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key } }))
}

async function createProductionPlan() {
  let keys = selectedKeys()
  if (!keys.length) {
    window.alert('请先选择产品后再创建生产计划')
    return
  }
  saving.value = true
  error.value = ''
  previewError.value = ''
  try {
    if (!planReady.value) {
      await loadSelectedPlanPreview()
      keys = selectedKeys()
      if (!keys.length) {
        window.alert('请先选择产品后再创建生产计划')
        return
      }
      if (!planReady.value) {
        if (!previewError.value) previewError.value = '没有可创建的生产计划，请检查库存缺口或订单商品绑定'
        return
      }
    }
    const payload = buildProductionPlanCreatePayload(filters, keys)
    currentPlan.value = await apiSend('/api/production-plans', { body: payload })
    await loadProductionPlanOperationSplits(currentPlan.value)
    await loadProductionPlans()
  } catch (err) {
    previewError.value = err.message || '创建生产计划失败'
  } finally {
    saving.value = false
  }
}

async function submitCurrentProductionPlan() {
  const payload = buildCurrentProductionPlanSubmitPayload(currentPlan.value)
  if (!payload.ids.length) return
  saving.value = true
  previewError.value = ''
  try {
    await saveCurrentPlanOperationSplits()
    const result = await apiSend(productionPlanBatchSubmitEndpoint(), { body: payload })
    const firstSuccess = Array.isArray(result.success) ? result.success[0] : null
    if (firstSuccess?.plan) currentPlan.value = firstSuccess.plan
    await loadProductionPlans()
    if (Array.isArray(result.failed) && result.failed.length) {
      previewError.value = `当前生产计划提交失败：${result.failed.map((item) => `${item.id}: ${item.error}`).join('；')}`
    }
  } catch (err) {
    previewError.value = err.message || '提交生成工单失败'
  } finally {
    saving.value = false
  }
}

async function submitSelectedProductionPlans() {
  const payload = buildProductionPlanBatchSubmitPayload(selectedProductionPlans)
  if (!payload.ids.length) return
  saving.value = true
  error.value = ''
  try {
    const result = await apiSend(productionPlanBatchSubmitEndpoint(), { body: payload })
    const firstSuccess = Array.isArray(result.success) ? result.success[0] : null
    currentPlan.value = firstSuccess?.plan || currentPlan.value
    replaceSelectedProductionPlans({})
    await loadProductionPlans()
    if (Array.isArray(result.failed) && result.failed.length) {
      error.value = `部分生产计划提交失败：${result.failed.map((item) => `${item.id}: ${item.error}`).join('；')}`
    }
  } catch (err) {
    error.value = err.message || '提交生成工单失败'
  } finally {
    saving.value = false
  }
}

function openShipReadyOrders() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'orders', params: { ship_ready: '1' } },
  }))
}

onMounted(async () => {
  const url = new URL(window.location.href)
  filters.from = url.searchParams.get('from') || ''
  filters.to = url.searchParams.get('to') || ''
  filters.customer_id = String(props.viewParams?.customer_id || props.customerContextId || url.searchParams.get('customer_id') || '')
  const selectedCsv = url.searchParams.get('selected') || ''
  if (selectedCsv) {
    for (const key of selectedCsv.split(',')) {
      if (key) selected[key] = true
    }
  }
  await load(url.searchParams.get('plan') === '1')
  await loadWorkstationCapacities()
  await loadProductionPlans()
})

watch(() => [props.viewParams?.customer_id, props.customerContextId], async () => {
  const nextCustomerID = String(props.viewParams?.customer_id || props.customerContextId || '')
  if (String(filters.customer_id || '') === nextCustomerID) return
  filters.customer_id = nextCustomerID
  await load(false)
})

watch(selectedSignature, () => {
  resetCurrentPlanForSelection()
  schedulePlanPreview()
})

watch(() => [filters.from, filters.to, filters.customer_id, filters.demand_status], () => {
  if (hasSelectedRows.value) {
    resetCurrentPlanForSelection()
    schedulePlanPreview()
  }
})

onBeforeUnmount(() => {
  if (previewTimer) clearTimeout(previewTimer)
  stopTableScrollDrag()
})
</script>

<style scoped>
.page { padding: 16px; display: grid; gap: 16px; }
.panel { min-width: 0; box-sizing: border-box; border: 1px solid #eee; border-radius: 10px; padding: 12px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.planning-workbench { display: grid; grid-template-columns: minmax(300px, .95fr) minmax(0, 1.35fr); gap: 16px; align-items: start; }
.planning-workbench.demand-collapsed { grid-template-columns: minmax(150px, 180px) minmax(0, 1fr); }
.planning-workbench.current-plan-collapsed { grid-template-columns: minmax(0, 1fr) minmax(150px, 180px); }
.demand-panel, .current-plan-panel { min-width: 0; }
.demand-panel.is-collapsed, .current-plan-panel.is-collapsed { min-height: 116px; }
.current-plan-content { display: grid; gap: 18px; min-width: 0; }
.current-plan-content > div { min-width: 0; }
.compact-head { margin-bottom: 8px; }
.empty-state { border: 1px dashed #d1d5db; border-radius: 8px; padding: 14px; display: grid; gap: 6px; background: #fafafa; }
.collapsed-panel-summary { border: 1px dashed #d1d5db; border-radius: 8px; padding: 10px; color: #666; background: #fafafa; font-size: 13px; }
.panel-head-actions { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 8px; }
.collapse-button { white-space: nowrap; }
.preview-loading { margin-bottom: 10px; }
.filters { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.production-plan-filters { grid-template-columns: repeat(4, minmax(0, 1fr)) auto; align-items: end; }
.filters label, .actions { display: flex; gap: 8px; }
.filters label { flex-direction: column; }
.filters span { font-size: 12px; color: #666; }
.actions { margin-top: 12px; flex-wrap: wrap; }
.plan-list-actions { margin: 12px 0; }
.current-plan-actions { align-items: center; }
.filter-action { min-height: 42px; }
.section-title-with-checkbox { display: inline-flex; align-items: center; gap: 8px; }
.section-hint { margin: 6px 0 10px; }
input, select, button { font: inherit; }
input { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 8px; }
select { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 8px; background: #fff; }
button { border-radius: 8px; padding: 10px 12px; cursor: pointer; }
input.bulk-checkbox { width: 18px; min-width: 18px; height: 18px; padding: 0; cursor: pointer; }
input.bulk-checkbox:disabled { cursor: not-allowed; opacity: 0.45; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.compact { min-height: 30px; padding: 5px 10px; }
.link-button { border: 0; background: transparent; color: #111; padding: 0; text-align: left; text-decoration: underline; text-underline-offset: 3px; }
.plan-no-button { font-weight: 700; }
.status { display: inline-flex; border: 1px solid #d1d5db; border-radius: 999px; padding: 2px 8px; background: #f9fafb; color: #374151; white-space: nowrap; }
.status-draft { border-color: #d1d5db; background: #f3f4f6; color: #374151; }
.status-submitted { border-color: #93c5fd; background: #eff6ff; color: #1d4ed8; }
.status-in-progress { border-color: #fdba74; background: #fff7ed; color: #c2410c; }
.status-completed { border-color: #86efac; background: #f0fdf4; color: #15803d; }
.status-cancelled { border-color: #fca5a5; background: #fef2f2; color: #b91c1c; }
.status-unknown { border-color: #e5e7eb; background: #f9fafb; color: #4b5563; }
.status-demand-unplanned { border-color: #d1d5db; background: #f9fafb; color: #374151; }
.status-demand-in-production { border-color: #fdba74; background: #fff7ed; color: #c2410c; }
.status-demand-completed { border-color: #86efac; background: #f0fdf4; color: #15803d; }
.plan-result { margin-top: 12px; display: flex; gap: 12px; align-items: center; }
.table-wrap { overflow: auto; max-width: 100%; }
.drag-scroll-wrap { cursor: grab; overscroll-behavior: auto; scrollbar-gutter: stable; -webkit-overflow-scrolling: touch; }
.drag-scroll-wrap.is-dragging-scroll { cursor: grabbing; user-select: none; }
table { width: 100%; border-collapse: collapse; min-width: 980px; }
.demand-table { min-width: 860px; }
.materials-table { min-width: 760px; }
.plan-preview-table { min-width: 1160px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 10px 8px; text-align: left; vertical-align: top; }
td small { display: block; color: #666; line-height: 1.6; }
.muted { color: #666; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.direct-ship-tip { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.direct-ship-tip div { display: grid; gap: 4px; }
.direct-ship-tip span { color: #28633b; font-size: 13px; }
.drawer-backdrop { position: fixed; inset: 0; z-index: 40; background: rgba(17, 24, 39, 0.28); display: flex; justify-content: flex-end; }
.production-plan-detail-drawer { width: min(980px, 100vw); height: 100vh; overflow: auto; background: #fff; box-shadow: -12px 0 28px rgba(15, 23, 42, 0.18); padding: 18px; display: grid; align-content: start; gap: 16px; }
.drawer-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; border-bottom: 1px solid #eee; padding-bottom: 12px; }
.drawer-head h2 { margin: 4px 0 0; font-size: 22px; }
.drawer-head-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.drawer-loading { padding: 12px 0; }
.detail-section { display: grid; gap: 10px; }
.detail-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin: 0; }
.detail-grid div { border: 1px solid #eee; border-radius: 8px; padding: 10px; min-width: 0; }
.detail-grid dt { font-size: 12px; color: #666; margin-bottom: 4px; }
.detail-grid dd { margin: 0; overflow-wrap: anywhere; }
.detail-table { min-width: 860px; }
.compact-table { min-width: 560px; }
.route-block { border: 1px solid #eee; border-radius: 8px; padding: 10px; display: grid; gap: 8px; }
.route-title { font-weight: 700; }
.operation-list { display: flex; flex-wrap: wrap; gap: 8px; }
.operation-pill { border: 1px solid #e5e7eb; border-radius: 999px; padding: 4px 8px; background: #f9fafb; color: #374151; }
.result-summary { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.result-summary span { border: 1px solid #e5e7eb; border-radius: 999px; padding: 4px 8px; background: #f9fafb; }
.operation-split-panel { display: grid; gap: 10px; }
.operation-split-placeholder { border: 1px dashed #d1d5db; border-radius: 8px; padding: 12px; display: grid; gap: 6px; background: #fafafa; }
.operation-split-placeholder span { color: #666; }
.split-operation-block { border-top: 1px solid #eee; padding-top: 10px; display: grid; gap: 8px; }
.split-operation-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.split-operation-head span { color: #374151; }
.split-row { display: grid; grid-template-columns: minmax(220px, 1.4fr) 120px repeat(4, minmax(96px, .65fr)) auto; gap: 8px; align-items: end; }
.split-row label { display: grid; gap: 5px; }
.split-row label span, .split-metric span { font-size: 12px; color: #666; }
.split-metric { min-height: 42px; border: 1px solid #eee; border-radius: 8px; padding: 6px 8px; display: grid; gap: 2px; background: #fafafa; }
.split-batch-cards { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(auto-fill, minmax(132px, 1fr)); gap: 8px; }
.split-batch-card { border: 1px solid #dbe3ef; border-radius: 8px; padding: 8px; display: grid; gap: 3px; background: #f8fbff; min-width: 0; }
.split-batch-card strong { font-size: 13px; color: #111827; }
.split-batch-card span, .split-batch-card small { overflow-wrap: anywhere; }
.split-batch-card span { font-size: 12px; color: #374151; }
.split-batch-card small { font-size: 12px; color: #6b7280; }
.split-batch-card em { font-style: normal; font-size: 12px; color: #b45309; }
.split-batch-card.underfilled { border-color: #f59e0b; background: #fffbeb; }
.danger-text { color: #a33; border-color: #d8b4b4; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .planning-workbench,
  .planning-workbench.demand-collapsed,
  .planning-workbench.current-plan-collapsed { grid-template-columns: 1fr; }
  .filters, .production-plan-filters { grid-template-columns: 1fr; }
  .direct-ship-tip { align-items: stretch; flex-direction: column; }
  .production-plan-detail-drawer { width: 100vw; padding: 14px; }
  .drawer-head { flex-direction: column; }
  .detail-grid { grid-template-columns: 1fr; }
  .split-row { grid-template-columns: 1fr; }
}
</style>
