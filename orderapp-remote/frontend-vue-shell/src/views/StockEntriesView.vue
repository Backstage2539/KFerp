<template>
  <div class="stock-entry-page" :class="{ embedded: props.embedded }">
    <ProductionReturnLink :source="props.viewParams?.return_navigation" :disabled="saving" :before-navigate="requestReturnToSource" />
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>库存单据</h2>
          <p>发料、转仓和生产库存动作统一形成 SE 单据；采购收货请前往采购入库，草稿不会改变库存。</p>
        </div>
        <div class="head-actions">
          <button class="secondary" type="button" @click="openWipInventory">查看 WIP 库存</button>
          <button class="primary" type="button" @click="openNewDrawer()">新建库存单据</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="filters.q" placeholder="SE 单号 / 备注" @keyup.enter="load" /></label>
        <label>
          <span>单据目的</span>
          <select v-model="filters.purpose">
            <option value="">全部</option>
            <option v-for="option in entryTypeOptions" :key="option.value" :value="normalizedPurpose(option.value)">{{ option.label }}</option>
          </select>
        </label>
        <label>
          <span>状态</span>
          <select v-model="filters.status">
            <option value="">全部</option>
            <option value="draft">草稿</option>
            <option value="submitted">已提交</option>
            <option value="cancelled">已取消</option>
          </select>
        </label>
        <button class="secondary" type="button" @click="load" :disabled="loading">查询</button>
      </div>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead><tr><th>单号</th><th>目的</th><th>工单</th><th>数量</th><th>状态</th><th>操作人</th><th>时间</th><th>备注</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td><strong>{{ row.entry_no }}</strong><small v-if="row.legacy">历史单据</small></td>
            <td>{{ documentPurposeLabel(row) }}</td>
            <td>{{ row.work_order_no || (row.work_order_id ? '已绑定工单' : '-') }}</td>
            <td>{{ quantityLabel(row) }}<small>{{ row.item_count || 0 }} 行</small></td>
            <td><span class="status" :class="row.status">{{ statusLabel(row.status) }}</span></td>
            <td>{{ row.operator || '-' }}</td>
            <td>{{ row.created_at || '-' }}</td>
            <td>{{ row.note || '-' }}</td>
            <td class="row-actions">
              <button class="link" type="button" @click="openExisting(row)">{{ row.purpose === 'manufacture' ? '查看结果' : '查看' }}</button>
              <span v-if="row.legacy" class="legacy-readonly">只读</span>
              <button v-if="row.status === 'cancelled'" class="link disabled" type="button" disabled>已取消</button>
              <button v-else-if="(row.status === 'submitted' && !row.legacy && row.purpose !== 'material_receipt') || (row.status === 'draft' && row.source_type === 'production_plan_material_transfer')" class="danger-link" type="button" :disabled="saving" @click="cancelDocument(row)">取消</button>
            </td>
          </tr>
          <tr v-if="!rows.length"><td colspan="9" class="muted">暂无库存单据</td></tr>
        </tbody>
      </table>
    </section>

    <div v-if="drawerOpen" class="drawer-mask" @click.self="closeDrawer">
      <aside class="drawer" aria-label="库存单据抽屉">
        <ProductionReturnLink :source="props.viewParams?.return_navigation" :disabled="saving" :before-navigate="requestReturnToSource" />
        <div class="drawer-head">
          <div>
            <h3>{{ form.id ? `${form.entry_no} · ${statusLabel(form.status)}` : '新建库存单据' }}</h3>
            <p v-if="form.work_order_id">工单号：{{ form.work_order_no || '加载中' }}</p>
          </div>
          <button class="secondary" type="button" @click="closeDrawer">关闭</button>
        </div>
        <div v-if="drawerError" class="error">{{ drawerError }}</div>
        <div v-if="isRetiredReceiptDraft" class="warning-list">
          <p>历史原料入库草稿已停用，不能继续提交。请前往采购入库重建采购单并确认收货。</p>
          <button class="secondary" type="button" @click="openPurchase">前往采购入库</button>
        </div>
        <div v-if="drawerWarnings.length" class="warning-list" role="status">
          <p v-for="warning in drawerWarnings" :key="warning">{{ warning }}</p>
        </div>
        <div v-if="stockPreviewState.message" class="stock-preview-state" :class="stockPreviewState.availability" role="status">
          <strong>{{ stockPreviewState.availability === 'ready' ? '本次任务已齐套' : '本次暂无可生成的领料明细' }}</strong>
          <p>{{ stockPreviewState.message }}</p>
        </div>
        <section v-if="completionResultLoading || completionResult" class="completion-result" aria-label="完工入库结果">
          <div class="completion-result-head">
            <div><span>完工入库结果</span><h4>{{ completionResult?.stockEntryNo || form.entry_no }}</h4><p>入库、客户货权、成品占用和发货状态使用同一组后台数据。</p></div>
            <span class="status submitted">{{ completionResultLoading ? '正在核对' : '已入库' }}</span>
          </div>
          <div v-if="completionResultLoading" class="completion-result-loading">正在核对客户生产申请、库存批次和订单预订…</div>
          <template v-else-if="completionResult">
            <div class="completion-metrics">
              <div><span>本次实际入库</span><strong>{{ numberLabel(completionResult.receiptQty) }} 件</strong></div>
              <div><span>库存总量</span><strong>{{ numberLabel(completionResult.inventoryTotalQty) }} 件</strong></div>
              <div><span>订单占用</span><strong>{{ numberLabel(completionResult.occupiedQty) }} 件</strong></div>
              <div><span>现货可用</span><strong>{{ numberLabel(completionResult.availableQty) }} 件</strong></div>
            </div>
            <div class="completion-trace-grid">
              <div><span>质检结果</span><strong>{{ completionResult.qualityLabel }}</strong></div>
              <div><span>客户货权</span><strong>{{ completionResult.customerName || (completionResult.customerID ? `客户 #${completionResult.customerID}` : '-') }}</strong></div>
              <div><span>目标仓库</span><strong>{{ warehouseName(completionResult.warehouse) }}</strong></div>
              <div><span>来源工单</span><strong>{{ completionResult.workOrderNo || '-' }}</strong></div>
              <div><span>客户生产申请</span><strong>{{ completionResult.requestNo || '-' }}</strong></div>
              <div><span>成品批次</span><strong>{{ completionResult.batchCode || '暂无批次编号' }}</strong></div>
            </div>
            <div class="completion-conversions">
              <strong>本次预订转换与订单状态</strong>
              <p v-if="!completionResult.convertedOrders.length">本次没有需要转换的客户订单预订。</p>
              <article v-for="order in completionResult.convertedOrders" :key="order.order_id">
                <div><strong>{{ order.order_no }}</strong><span>已转成品占用 {{ numberLabel(order.converted_qty - (order.released_qty || 0)) }} 件 · {{ orderFulfillmentLabel(order) }}</span></div>
                <button class="secondary" type="button" @click="openCompletionOrder(order)">查看订单</button>
              </article>
            </div>
            <div class="completion-actions">
              <button class="secondary" type="button" @click="openCompletionWorkOrder">查看来源工单</button>
              <button v-if="completionResult.requestNo" class="secondary" type="button" @click="openCompletionRequest">查看客户申请</button>
              <button class="primary" type="button" @click="openCompletionInventory">查看库存批次</button>
            </div>
          </template>
        </section>
        <template v-if="stockPreviewState.availability === 'actionable' || !stockPreviewState.message">
        <fieldset v-if="isPlanMaterialTransfer" disabled class="plan-transfer-readonly">
          <p class="production-issue-hint">此调拨单由生产计划生成。若需修改，请取消草稿后返回生产计划重新生成。</p>
          <label><span>备注</span><input :value="form.note" /></label>
          <h4>单据明细</h4>
          <article v-for="item in form.items" :key="item.id || item.local_key" class="item-card">
            <div class="item-grid">
              <label><span>物料</span><input :value="item.item_name || '-'" /></label>
              <label><span>出库仓</span><input :value="warehouseName(item.from_warehouse)" /></label>
              <label><span>入库仓</span><input :value="warehouseName(item.to_warehouse)" /></label>
              <label><span>货主</span><input :value="item.owner_customer_id ? `客户 #${item.owner_customer_id}` : '工厂'" /></label>
              <label><span>数量（{{ item.inventory_unit || '件' }}）</span><input :value="item.quantity" /></label>
              <label><span>来源批次</span><input :value="item.batch_code || '-'" /></label>
            </div>
          </article>
        </fieldset>
        <template v-else>
        <div class="document-form">
          <label>
            <span>单据目的</span>
            <select v-model="form.purpose_key" :disabled="!isDraft || isBoundProductionDocument" @change="applyEntryDefaults">
              <option v-for="option in entryTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label v-if="isMaterialProductionReceipt"><span>本次入库</span><select v-model="receiptMode" :disabled="!isDraft"><option value="partial">本次部分入库</option><option value="final">最后一次入库</option></select></label>
          <label v-if="isMaterialProductionReceipt"><span>本次实际投入（kg）</span><input v-model.number="receiptInputKg" type="number" min="0" step="0.001" :disabled="!isDraft" placeholder="按本次实际投料填写" /></label>
          <label class="wide"><span>备注</span><input v-model.trim="form.note" :disabled="!isDraft" /></label>
        </div>

        <div class="line-head">
          <h4>单据明细</h4>
          <button v-if="isDraft && !isBoundProductionDocument" class="secondary" type="button" @click="addItem">新增明细</button>
        </div>
        <p v-if="showsWIPIssueSuggestion" class="production-issue-hint" role="note">工单建议领用量仅用于默认填充，不限制实际领料；超出工单当前需求的部分将保留为可用 WIP 库存，生产消耗仍需另行记录。来源仓与批次按已冻结安排，实际可领量受合格库存及其他工单预留限制。</p>
        <div v-if="usesCompactProductionItemRows" class="compact-production-items">
          <div class="compact-production-items-head" aria-hidden="true">
            <span>物料</span>
            <span>出库仓</span>
            <span>入库仓</span>
            <span>{{ productionQuantityLabel }}</span>
            <span>库存单位</span>
            <span>指定批次</span>
            <span>操作</span>
          </div>
          <div
            v-for="(item, index) in form.items"
            :key="item.local_key || item.id || index"
            class="compact-production-item"
          >
            <div class="compact-production-item-grid">
              <label class="compact-material-name">
                <span class="mobile-field-label">物料</span>
                <input :value="item.item_name || '-'" aria-label="物料" disabled />
              </label>
              <label>
                <span class="mobile-field-label">出库仓</span>
                <select v-model="item.from_warehouse" aria-label="出库仓" disabled>
                  <option value="">无</option>
                  <option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option>
                </select>
                <small v-if="item.from_warehouse && item.material_id" class="source-balance">来源仓账面库存 {{ item.source_balance?.book_qty || 0 }} {{ item.inventory_unit || '' }}；可用库存 {{ item.source_balance?.available_qty || 0 }} {{ item.inventory_unit || '' }}<template v-if="Number(item.source_balance?.frozen_qty || 0) > 0">；冻结库存 {{ item.source_balance.frozen_qty }} {{ item.inventory_unit || '' }}</template></small>
              </label>
              <label>
                <span class="mobile-field-label">入库仓</span>
                <select v-model="item.to_warehouse" aria-label="入库仓" disabled>
                  <option value="">无</option>
                  <option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option>
                </select>
              </label>
              <label>
                <span class="mobile-field-label">{{ productionQuantityLabel }}</span>
                <input v-model.number="item.quantity" type="number" min="0" :step="itemUsesCount(item) ? 1 : 'any'" :aria-label="productionQuantityLabel" :disabled="!isDraft" />
              </label>
              <label>
                <span class="mobile-field-label">库存单位</span>
                <output class="readonly-value" aria-label="库存单位">{{ item.inventory_unit || '-' }}</output>
              </label>
              <label>
                <span class="mobile-field-label">指定批次（可选）</span>
                <input v-model.trim="item.batch_code" aria-label="指定批次（可选）" :disabled="!isDraft || item.frozen_picking" placeholder="不填按 FIFO" />
              </label>
              <button
                v-if="isDraft && form.items.length > 1"
                class="danger-link compact-delete"
                type="button"
                :aria-label="`删除${item.item_name || '物料'}明细`"
                @click="form.items.splice(index, 1)"
              >
                删除
              </button>
            </div>
            <div v-if="item.allocations?.length" class="allocations compact-allocations">
              <span v-for="allocation in item.allocations" :key="`${allocation.material_batch_id}-${allocation.batch_code}`">
                {{ allocation.batch_code }}：{{ allocationQuantityLabel(allocation, item.inventory_unit) }}
              </span>
            </div>
          </div>
        </div>
        <div v-else v-for="(item, index) in form.items" :key="item.local_key || item.id || index" class="item-card">
          <div class="item-grid">
            <template v-if="isDraft && isOrdinaryTransfer()">
              <label><span>类型</span><select v-model="item.item_type" @change="resetTransferSelection(item)"><option value="material">物料</option><option value="finished_product">商品 / SKU</option></select></label>
              <label><span>出库仓</span><select v-model="item.from_warehouse" @change="changeTransferWarehouse(item)"><option value="">请选择</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label>
              <label class="wide"><span>{{ item.item_type === 'material' ? '物料' : '商品 / SKU' }}</span>
                <SearchableSelect :model-value="selectedObjectID(item)" :options="transferObjectOptions(item)" :option-label="itemOptionLabel" :max-options="100000" :empty-text="transferChoiceEmptyText(item)" placeholder="输入名称 / 编号" @focus="ensureChoiceInventory(item.from_warehouse)" @open="ensureChoiceInventory(item.from_warehouse)" @update:model-value="selectTransferObject(item, $event)" />
                <button v-if="transferChoices[item.from_warehouse]?.error" class="secondary" type="button" @click="ensureChoiceInventory(item.from_warehouse)">重试加载库存</button>
              </label>
              <label><span>规格</span><select v-model="item.transfer_spec" @change="changeTransferSpec(item)"><option value="">请选择</option><option v-for="spec in transferItemSpecs(item)" :key="spec.key" :value="spec.key">{{ spec.label }}</option></select><small v-if="selectedTransferSpec(item)" class="source-balance">该规格库存 {{ transferSpecBalance(item, false) }}；可转 {{ transferSpecBalance(item, true) }}</small></label>
              <label class="wide"><span>来源批次（必选）</span><select v-model="item.batch_code" @change="changeTransferBatch(item)"><option value="">请选择批次</option><option v-for="row in transferItemBatches(item)" :key="row.batch_id" :value="row.batch_code" :disabled="Boolean(row.unavailable_reason) || transferRowAmount(row, item, true) <= 0">{{ row.batch_code }} · 可转 {{ transferRowAmount(row, item, true) }} {{ selectedTransferSpec(item)?.unit }}{{ row.unavailable_reason ? ` · ${row.unavailable_reason}` : '' }}</option></select><small v-if="selectedTransferBatch(item)" class="source-balance">该批次库存 {{ transferRowAmount(selectedTransferBatch(item), item, false) }}；本行最多可转 {{ remainingTransferBatch(item) }} {{ selectedTransferSpec(item)?.unit }}</small></label>
              <label><span>数量（{{ selectedTransferSpec(item)?.unit || '件' }}）</span><input v-model.number="item.quantity" type="number" min="0" :step="selectedTransferSpec(item)?.mode === 'weight' ? 'any' : 1" /><small v-if="selectedTransferSpec(item)?.mode === 'loose'" class="source-balance">当前按克计量，填写 100 即转 100 克</small><small v-else-if="selectedTransferSpec(item)?.mode === 'canonical_weight'" class="source-balance">当前按 {{ selectedTransferSpec(item)?.unit }} 计量</small></label>
              <label><span>入库仓</span><select v-model="item.to_warehouse" @change="item.target_owner_customer_id = 0"><option value="">请选择</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label>
              <label v-if="ownerTransferCustomerID(item)" class="wide"><span>货权变更</span><span><input type="checkbox" :checked="Number(item.target_owner_customer_id || 0) === ownerTransferCustomerID(item)" @change="item.target_owner_customer_id = $event.target.checked ? ownerTransferCustomerID(item) : 0" /> 确认将本次转入的公司成品变更为{{ warehouseName(item.to_warehouse) }}所属客户的库存</span></label>
            </template>
            <template v-else>
            <label>
              <span>类型</span>
              <select v-model="item.item_type" :disabled="!isDraft || isBoundProductionDocument" @change="resetItemObject(item)">
                <option value="material">物料</option>
                <option value="finished_product">商品 / SKU</option>
              </select>
            </label>
            <label class="wide">
              <span>{{ item.item_type === 'material' ? '物料' : '商品 / SKU' }}</span>
              <SearchableSelect
                v-if="isDraft && !isBoundProductionDocument"
                :model-value="selectedObjectID(item)"
                :options="item.item_type === 'material' ? stockEntryMaterialOptions : productOptionsForItem(item)"
                :option-label="itemOptionLabel"
                :remote="item.item_type === 'finished_product' && !isFinishedTransfer(item)"
                :max-options="isFinishedTransfer(item) ? 100000 : (item.item_type === 'finished_product' ? 20 : 80)"
                :empty-text="isFinishedTransfer(item) ? transferOptionsEmptyText(item) : (item.item_type === 'finished_product' ? (productOptionsLoading ? '正在加载产品…' : '没有匹配产品') : (baseOptionsLoading ? '正在加载物料…' : '没有匹配物料'))"
                :disabled="baseOptionsLoading && item.item_type === 'material'"
                placeholder="输入名称 / 编号"
                @focus="item.item_type === 'finished_product' && handleItemProductOptionsFocus(item)"
                @open="item.item_type === 'finished_product' && handleItemProductOptionsFocus(item)"
                @input="item.item_type === 'finished_product' && !isFinishedTransfer(item) && handleProductSearch($event)"
                @update:model-value="selectItemObject(item, $event)">
                <template #menu-footer>
                  <div v-if="item.item_type === 'finished_product' && !isFinishedTransfer(item) && productOptionsLoading" class="select-loading">正在加载…</div>
                  <button v-else-if="item.item_type === 'finished_product' && !isFinishedTransfer(item) && productOptionsHasNext" type="button" class="select-load-more" @mousedown.prevent @click="loadMoreProductOptions">加载更多</button>
                </template>
              </SearchableSelect>
              <input v-else :value="item.item_name || '-'" disabled />
            </label>
            <label v-if="item.item_type === 'finished_product' && itemUsesBomSpecs(item)">
              <span>BOM 规格</span>
              <select v-model.number="item.bom_spec_id" :disabled="!isDraft" @change="selectItemBomSpec(item)">
                <option :value="0">请选择</option>
                <option v-for="spec in itemBomSpecs(item)" :key="spec.bom_spec_id" :value="spec.bom_spec_id">{{ spec.name }}（{{ spec.unit }}）</option>
              </select>
            </label>
            <label v-else-if="item.item_type === 'finished_product'"><span>规格(g)</span><input v-model.number="item.spec_g" type="number" min="1" :disabled="!isDraft" /></label>
            <label><span>出库仓</span><select v-model="item.from_warehouse" :disabled="!isDraft || isBoundProductionDocument" @change="handleSourceWarehouseChange(item)"><option value="">无</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select><small v-if="item.from_warehouse && item.material_id" class="source-balance">来源仓账面库存 {{ item.source_balance?.book_qty || 0 }} {{ item.inventory_unit || '' }}；可用库存 {{ item.source_balance?.available_qty || 0 }} {{ item.inventory_unit || '' }}<template v-if="Number(item.source_balance?.frozen_qty || 0) > 0">；冻结库存 {{ item.source_balance.frozen_qty }} {{ item.inventory_unit || '' }}</template></small></label>
            <label><span>入库仓</span><select v-model="item.to_warehouse" :disabled="!isDraft || isBoundProductionDocument"><option value="">无</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label>
            <label v-if="usesSingleQuantity(item)">
              <span>{{ productionQuantityLabel }}</span>
              <input v-model.number="item.quantity" type="number" min="0" :step="itemUsesCount(item) ? 1 : 'any'" :disabled="!isDraft" />
            </label>
            <label v-if="usesSingleQuantity(item)"><span>库存单位</span><div class="readonly-value">{{ item.inventory_unit || '-' }}</div></label>
            <label v-if="!usesSingleQuantity(item)"><span>数量(g)</span><input v-model.number="item.qty_g" type="number" min="0" :disabled="!isDraft" /></label>
            <label v-if="!usesSingleQuantity(item)"><span>数量(件)</span><input v-model.number="item.qty_units" type="number" min="0" :disabled="!isDraft" /></label>
            <label v-if="!isDraft && isOrdinaryTransfer()"><span>{{ item.item_type === 'finished_product' ? '目标批次' : '来源批次' }}</span><input :value="item.batch_code || '-'" disabled /></label>
            <label v-else><span>指定批次（可选）</span><input v-model.trim="item.batch_code" :disabled="!isDraft || item.frozen_picking" placeholder="不填按 FIFO" /></label>
            <label v-if="isReceipt"><span>单位成本</span><input v-if="item.cost_unavailable" value="待核算" type="text" readonly /><input v-else v-model.number="item.unit_cost" type="number" min="0" step="0.0001" :disabled="!isDraft" /></label>
            <template v-if="isReceipt && item.item_type === 'material'">
              <label><span>供应商</span><input v-model.trim="item.supplier" :disabled="!isDraft" /></label>
              <label><span>产季</span><input v-model.trim="item.crop_season" :disabled="!isDraft" /></label>
              <label><span>产地</span><input v-model.trim="item.origin" :disabled="!isDraft" /></label>
              <label class="wide"><span>产家风味描述</span><input v-model.trim="item.producer_flavor_description" :disabled="!isDraft" /></label>
            </template>
            </template>
          </div>
          <div v-if="item.allocations?.length" class="allocations">
            <span v-for="allocation in item.allocations" :key="`${allocation.material_batch_id}-${allocation.batch_code}`">
              {{ allocation.batch_code }}：{{ allocationQuantityLabel(allocation, item.inventory_unit) }}
            </span>
          </div>
          <div v-if="item.transfer_batch_moves?.length" class="allocations"><span v-for="move in item.transfer_batch_moves" :key="`${move.source_batch_code}-${move.target_batch_code}`">来源批次 {{ move.source_batch_code }} → 目标批次 {{ move.target_batch_code }}</span></div>
          <button v-if="isDraft && form.items.length > 1" class="danger-link" type="button" @click="form.items.splice(index, 1)">删除明细</button>
        </div>
        </template>
        <div class="drawer-actions">
          <button v-if="isDraft && !isRetiredReceiptDraft && !isPlanMaterialTransfer" class="secondary" type="button" @click="saveDraft" :disabled="saving || productDetailsLoading || !form.items.length">保存草稿</button>
          <button v-if="isDraft && !isRetiredReceiptDraft" class="primary" type="button" @click="submitDocument" :disabled="saving || productDetailsLoading || !form.items.length">提交并过账</button>
        </div>
        </template>
      </aside>
    </div>
    <div v-if="returnDialogOpen" class="return-dialog-mask">
      <section ref="returnDialog" class="return-dialog" role="dialog" aria-modal="true" aria-labelledby="stock-return-title" tabindex="-1" @keydown="handleReturnDialogKeydown">
        <h3 id="stock-return-title">库存单据有未保存的修改</h3>
        <p>可以先保存草稿再返回，库存不会过账。</p>
        <p v-if="drawerError" class="error" role="alert">{{ drawerError }}</p>
        <div class="return-dialog-actions">
          <button class="primary" type="button" :disabled="saving" @click="saveDraftAndReturn">保存草稿并返回</button>
          <button class="secondary" type="button" :disabled="saving" @click="discardDraftAndReturn">放弃修改并返回</button>
          <button class="secondary" type="button" :disabled="saving" @click="continueEditing">继续编辑</button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { isSemiFinishedMaterial, selectableStockEntryMaterials } from '../lib/material-receipts'
import { stockEntryEndpoint, stockEntryTypeLabel, stockEntryTypeOptions } from '../lib/manufacturing-execution'
import { inventoryUnitWeightInGrams, productionStockDocumentPreviewAction, stockCanonicalQuantity, stockDocumentPositiveItems, stockQuantityUsesCount } from '../lib/production-execution-hub'
import { customerProcessingReceiptResult } from '../lib/customer-processing-trace'
import { loadTransferInventory, transferProductOptions, transferProductSpecs } from '../lib/stock-transfer-options'
import { loadTransferOptions, transferSpecs, transferBatches, transferAmount, transferWeightFactor } from '../lib/stock-transfer-selector'
import {
  buildProductSpecWriteIdentity,
  isProductBomSpecCutover,
  normalizeProductBomSpecs,
} from '../lib/product-spec-cutover'

const props = defineProps({
  embedded: { type: Boolean, default: false },
  viewParams: { type: Object, default: () => ({}) },
})

const entryTypeOptions = stockEntryTypeOptions()
const rows = ref([])
const materials = ref([])
const products = ref([])
const warehouses = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const drawerError = ref('')
const drawerWarnings = ref([])
const stockPreviewState = reactive({ availability: '', message: '' })
const drawerOpen = ref(false)
const completionResult = ref(null)
const completionResultLoading = ref(false)
const filters = reactive({ q: '', purpose: '', status: '', work_order_id: 0 })
let localKey = 0
const form = reactive(emptyDocument())
let materialBalanceRequest = 0
const baseOptionsLoading = ref(false)
const baseOptionsLoaded = ref(false)
const productOptionsLoading = ref(false)
const productOptionsLoaded = ref(false)
const productOptionsPage = ref(1)
const productOptionsHasNext = ref(false)
const productSearchQuery = ref('')
const productDetailsLoading = ref(0)
let productSearchTimer = 0
let productOptionsRequest = 0
let baseOptionsRequest = null
const productDetailRequests = new Map()
const transferInventory = reactive({})
const transferChoices = reactive({})
const transferChoiceRequests = new Map()
let transferChoiceGeneration = 0
const transferInventoryRequests = new Map()
let transferInventoryGeneration = 0

const receiptMode = ref('final')
const receiptInputKg = ref(0)
const savedDraftFingerprint = ref(draftFingerprint())
const returnDialogOpen = ref(false)
const returnDialog = ref(null)
const pendingReturnNavigation = ref(null)
let returnFocusElement = null
const isMaterialProductionReceipt = computed(() => isBoundProductionDocument.value && form.purpose_key === 'manufacture' && form.items.length === 1 && form.items[0].item_type === 'material')
const isDraft = computed(() => !form.id || form.status === 'draft')
const isPlanMaterialTransfer = computed(() => String(form.source_type || '') === 'production_plan_material_transfer')
const hasUnsavedDraft = computed(() => isDraft.value && (
  draftFingerprint() !== savedDraftFingerprint.value
  || (!form.id && form.items.some(item => Number(item.quantity || item.qty_g || item.qty_units || 0) > 0))
))
const isReceipt = computed(() => form.purpose_key === 'material_receipt')
const isRetiredReceiptDraft = computed(() => Boolean(form.id && form.status === 'draft' && isReceipt.value))
const stockEntryMaterialOptions = computed(() => selectableStockEntryMaterials(materials.value, form.purpose_key))
const isBoundProductionDocument = computed(() => Number(form.work_order_id || 0) > 0)
const showsWIPIssueSuggestion = computed(() => (
  isBoundProductionDocument.value && form.purpose_key === 'material_transfer_for_manufacture'
))
const usesCompactProductionItemRows = computed(() => (
  isBoundProductionDocument.value
  && [
    'material_transfer_for_manufacture',
    'material_return_from_manufacture',
    'material_consumption_for_manufacture',
  ].includes(form.purpose_key)
))
const productionQuantityLabel = computed(() => {
  if (form.purpose_key === 'material_transfer_for_manufacture') return '领用数量'
  if (form.purpose_key === 'material_return_from_manufacture') return '退回数量'
  if (form.purpose_key === 'material_consumption_for_manufacture') return '消耗数量'
  if (form.purpose_key === 'manufacture') return '入库数量'
  return '数量'
})

function emptyItem() {
  return {
    local_key: ++localKey, material_id: 0, product_id: 0, item_type: 'material', item_name: '',
    migration_state: 'legacy', bom_spec_id: 0, bom_variant_id: 0,
    spec_g: 0, inventory_unit: '', from_warehouse: 'raw_materials', to_warehouse: 'wip',
    quantity: 0, quantity_basis: '', canonical_qty_per_unit: 0, transfer_spec: '',
    required_qty: 0, remaining_qty: null, remembered_qty: 0, default_qty: 0,
    qty_g: 0, qty_units: 0, batch_code: '', owner_customer_id: 0, target_owner_customer_id: 0, unit_cost: 0, supplier: '', crop_season: '',
    origin: '', producer_flavor_description: '', product_detail_ready: false, allocations: [], source_balance: { book_qty: 0, available_qty: 0, frozen_qty: 0 },
  }
}

function emptyDocument() {
  return {
    id: 0, entry_no: '', status: 'draft', purpose_key: 'material_transfer',
    work_order_id: 0, work_order_no: '', job_card_id: 0, running_item_id: 0, source_type: '', source_id: 0,
    return_source: '', note: '', items: [emptyItem()],
  }
}

function resetForm(next = emptyDocument()) {
  Object.keys(form).forEach((key) => delete form[key])
  Object.assign(form, next)
}

function normalizedPurpose(value) {
  if (value === 'material_return_from_manufacture') return 'material_transfer_for_manufacture'
  return value
}

function draftFingerprint() {
  const keys = ['material_id', 'product_id', 'item_type', 'bom_spec_id', 'bom_variant_id', 'spec_g', 'from_warehouse', 'to_warehouse', 'owner_customer_id', 'target_owner_customer_id', 'quantity', 'qty_g', 'qty_units', 'batch_code', 'unit_cost', 'supplier', 'crop_season', 'origin', 'producer_flavor_description']
  return JSON.stringify({
    purpose: form.purpose_key, note: form.note,
    items: form.items.map(item => Object.fromEntries(keys.map(key => [key, item[key] ?? '']))),
  })
}

function requestReturnToSource() {
  const source = props.viewParams?.return_navigation
  if (saving.value || !source?.key) return false
  pendingReturnNavigation.value = { ...source, params: { ...(source.params || {}) } }
  if (hasUnsavedDraft.value) {
    returnFocusElement = globalThis.document?.activeElement
    returnDialogOpen.value = true
    nextTick(() => returnDialog.value?.focus())
  } else {
    returnToSource()
  }
  return false
}

function returnToSource() {
  const source = pendingReturnNavigation.value
  if (!source?.key) return
  returnDialogOpen.value = false
  pendingReturnNavigation.value = null
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: source.key, params: source.params || {}, ...(source.viewContext ? { viewContext: source.viewContext } : {}) } }))
}

function continueEditing() {
  if (saving.value) return
  returnDialogOpen.value = false
  pendingReturnNavigation.value = null
  nextTick(() => returnFocusElement?.focus())
}

function handleReturnDialogKeydown(event) {
  if (event.key === 'Escape') {
    event.preventDefault()
    continueEditing()
  }
  if (event.key !== 'Tab') return
  const buttons = [...event.currentTarget.querySelectorAll('button:not([disabled])')]
  const first = buttons[0], last = buttons.at(-1)
  if (!first) { event.preventDefault(); return }
  if (event.shiftKey && (event.target === first || event.target === event.currentTarget)) {
    event.preventDefault(); last.focus()
  } else if (!event.shiftKey && event.target === last) {
    event.preventDefault(); first.focus()
  }
}

function discardDraftAndReturn() {
  if (!saving.value) returnToSource()
}

async function saveDraftAndReturn() {
  if (saving.value || !pendingReturnNavigation.value) return
  const draft = await saveDraft()
  if (draft?.id) returnToSource()
}

function documentPurposeLabel(row) {
  if (row.is_return) return '退回未用原料'
  return stockEntryTypeLabel(row.purpose || row.entry_type)
}

function statusLabel(value) {
  return ({ draft: '草稿', submitted: '已提交', cancelled: '已取消' })[String(value || '')] || value || '-'
}

function quantityLabel(row) {
  if (Number(row.total_qty_g || 0) > 0) return `${Number(row.total_qty_g).toLocaleString('zh-CN')}g`
  if (Number(row.total_qty_units || 0) > 0) return `${Number(row.total_qty_units).toLocaleString('zh-CN')}件`
  return '-'
}

function numberLabel(value) {
  return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 })
}

function warehouseName(code) {
  return warehouses.value.find((row) => row.code === code)?.name || code || '-'
}

function orderFulfillmentLabel(order = {}) {
  if (Number(order.shortfall_qty || 0) > 0) return `仍缺 ${numberLabel(order.shortfall_qty)} 件`
  return Number(order.converted_qty || 0) >= Number(order.reserved_qty || 0) ? '待发货' : '待生产'
}

function itemUsesCount(item = {}) {
  return stockQuantityUsesCount(item)
}

function usesSingleQuantity(item = {}) {
  if (!isBoundProductionDocument.value || item.item_type !== 'material') return false
  return [
    'material_transfer_for_manufacture',
    'material_return_from_manufacture',
    'material_consumption_for_manufacture',
    'manufacture',
  ].includes(form.purpose_key)
}

function quantityFromCanonical(item = {}) {
  for (const value of [item.quantity, item.default_qty, item.remembered_qty]) {
    const explicit = Number(value || 0)
    if (explicit > 0) return explicit
  }
  if (itemUsesCount(item)) return Number(item.qty_units || 0)
  const factor = Number(item.canonical_qty_per_unit || 0) || inventoryUnitWeightInGrams(item.inventory_unit)
  return factor > 0 ? Number(item.qty_g || 0) / factor : 0
}

function normalizedItem(item = {}) {
  const next = { ...emptyItem(), ...item }
  next.item_name = next.item_name || next.material_name || next.product_name || ''
  const preferredQuantity = Number(next.quantity || next.default_qty || next.remembered_qty || 0)
  if (!itemUsesCount(next) && preferredQuantity > 0 && Number(next.qty_g || 0) > 0) {
    next.canonical_qty_per_unit = Number(next.qty_g) / preferredQuantity
  }
  next.quantity = quantityFromCanonical(next)
  return next
}

function canonicalQuantity(item = {}) {
  const quantity = stockCanonicalQuantity(item)
  if (Number(item.quantity || 0) > 0 && quantity.qty_g <= 0 && quantity.qty_units <= 0) {
    throw new Error(`${item.item_name || '物料'}缺少库存单位换算`)
  }
  return quantity
}

function allocationQuantityLabel(allocation = {}, inventoryUnit = '') {
  if (Number(allocation.qty_units || 0) > 0) return `${Number(allocation.qty_units).toLocaleString('zh-CN')} ${inventoryUnit || '件'}`
  const factor = inventoryUnitWeightInGrams(inventoryUnit)
  if (!(factor > 0)) return `${Number(allocation.qty_g || 0).toLocaleString('zh-CN')} g`
  return `${(Number(allocation.qty_g || 0) / factor).toLocaleString('zh-CN')} ${inventoryUnit}`
}

function itemOptionLabel(row) {
  const name = String(row?.name || row?.Name || '').trim()
  const code = String(row?.code || row?.Code || '').trim()
  return code ? `${name} (${code})` : name || `#${row?.id || ''}`
}

function selectedObjectID(item) {
  return Number(item.item_type === 'material' ? item.material_id : item.product_id) || 0
}

function selectedItemProduct(item) {
  return products.value.find((row) => Number(row.id || row.product_id || 0) === Number(item.product_id || 0)) || null
}

function itemUsesBomSpecs(item) {
  return item?.migration_state === 'cutover' || isProductBomSpecCutover(selectedItemProduct(item) || {})
}

function itemBomSpecs(item) {
  const specs = normalizeProductBomSpecs(selectedItemProduct(item) || item || {})
  const inventory = transferInventory[item.from_warehouse]
  return isFinishedTransfer(item) && inventory?.loaded
    ? transferProductSpecs(specs, inventory.rows, item.product_id)
    : specs
}

function selectItemBomSpec(item) {
  const selected = itemBomSpecs(item).find((row) => Number(row.bom_spec_id) === Number(item.bom_spec_id))
  item.bom_variant_id = Number(selected?.bom_variant_id || 0)
  item.inventory_unit = String(selected?.unit || item.inventory_unit || '')
}

function mergeProductOption(row) {
  const id = Number(row?.id || 0)
  if (!id) return row
  const current = products.value.find((candidate) => Number(candidate.id || 0) === id)
  const merged = current ? { ...current, ...row } : row
  products.value = current
    ? products.value.map((candidate) => Number(candidate.id || 0) === id ? merged : candidate)
    : [...products.value, merged]
  return merged
}

function applySelectedProduct(item, selected) {
  if (!selected) return
  item.product_detail_ready = Boolean(selected.product_detail_loaded)
  item.migration_state = isProductBomSpecCutover(selected) ? 'cutover' : 'legacy'
  const specs = itemBomSpecs(item)
  const current = specs.find((row) => Number(row.bom_spec_id) === Number(item.bom_spec_id))
  item.bom_spec_id = Number(current?.bom_spec_id || specs.find((row) => row.is_default)?.bom_spec_id || specs[0]?.bom_spec_id || 0)
  item.bom_variant_id = 0
  item.item_name = selected.name || selected.Name || item.item_name || ''
  item.inventory_unit = selected.inventory_unit || selected.unit || item.inventory_unit || ''
  if (item.item_type === 'finished_product') item.spec_g = Number(selected.spec_g || item.spec_g || 0)
  if (item.migration_state === 'cutover') selectItemBomSpec(item)
}

async function loadProductDetail(productID) {
  const id = Number(productID || 0)
  if (!id) return null
  const existing = productDetailRequests.get(id)
  if (existing) return existing
  productDetailsLoading.value += 1
  const request = apiGet(`/api/products/${id}`)
    .then((data) => {
      const product = data?.product || data
      return mergeProductOption({ ...product, product_detail_loaded: true })
    })
    .finally(() => {
      productDetailsLoading.value = Math.max(0, productDetailsLoading.value - 1)
      productDetailRequests.delete(id)
    })
  productDetailRequests.set(id, request)
  return request
}

async function ensureProductDetailForItem(item) {
  const id = Number(item?.product_id || 0)
  if (!id) return
  const selected = products.value.find((row) => Number(row.id || 0) === id)
  if (selected?.product_detail_loaded) {
    applySelectedProduct(item, selected)
    return
  }
  item.product_detail_ready = false
  try {
    const detail = await loadProductDetail(id)
    if (Number(item.product_id || 0) === id) applySelectedProduct(item, detail)
  } catch (err) {
    drawerError.value = err.message || '加载产品规格失败，请重试'
  }
}

function selectItemObject(item, id) {
  const value = Number(id || 0)
  const options = item.item_type === 'material' ? materials.value : productOptionsForItem(item)
  const selected = options.find((row) => Number(row.id || 0) === value)
  item.material_id = item.item_type === 'material' ? value : 0
  item.product_id = item.item_type === 'finished_product' ? value : 0
  item.item_name = selected?.name || selected?.Name || ''
  if (item.item_type === 'material') {
    item.product_detail_ready = false
    item.inventory_unit = selected?.unit || selected?.inventory_unit || ''
  } else if (value > 0) {
    item.product_detail_ready = false
    void ensureProductDetailForItem(item)
  } else {
    item.product_detail_ready = false
  }
  loadMaterialBalances()
}

function resetItemObject(item) {
  item.material_id = 0
  item.product_id = 0
  item.migration_state = 'legacy'
  item.bom_spec_id = 0
  item.bom_variant_id = 0
  item.item_name = ''
  item.product_detail_ready = false
  item.spec_g = item.item_type === 'finished_product' ? 454 : 0
}

function addItem() {
  const item = emptyItem()
  form.items.push(item)
  applyItemDefaults(item)
  if (isOrdinaryTransfer()) void ensureChoiceInventory(item.from_warehouse)
}

function applyItemDefaults(item) {
  const defaults = {
    material_issue: ['raw_materials', '', 'material'],
    material_transfer: ['raw_materials', 'wip', item.item_type || 'material'],
    material_transfer_for_manufacture: ['raw_materials', 'wip', 'material'],
    material_return_from_manufacture: ['wip', 'raw_materials', 'material'],
    material_consumption_for_manufacture: ['wip', '', 'material'],
    manufacture: ['', 'finished_goods', 'finished_product'],
  }[form.purpose_key] || ['raw_materials', 'wip', 'material']
  item.from_warehouse = defaults[0]
  item.to_warehouse = defaults[1]
  item.item_type = defaults[2]
}

function applyEntryDefaults() {
  form.items.forEach(applyItemDefaults)
  if (isOrdinaryTransfer()) for (const item of form.items) void ensureChoiceInventory(item.from_warehouse)
}

function openNewDrawer(purpose = '') {
  clearTransferInventory()
  clearTransferChoices()
  completionResult.value = null
  resetForm(emptyDocument())
  if (purpose) form.purpose_key = purpose
  applyEntryDefaults()
  savedDraftFingerprint.value = draftFingerprint()
  drawerError.value = ''
  drawerWarnings.value = []
  Object.assign(stockPreviewState, { availability: '', message: '' })
  drawerOpen.value = true
  void loadOptions().catch(() => {})
  if (isOrdinaryTransfer()) for (const item of form.items) void ensureChoiceInventory(item.from_warehouse)
}

function closeDrawer() {
  completionResult.value = null
  drawerOpen.value = false
  drawerError.value = ''
  drawerWarnings.value = []
  Object.assign(stockPreviewState, { availability: '', message: '' })
}

function requestBody() {
  if (!isOrdinaryTransfer() && form.items.some((item) => item.item_type === 'finished_product' && Number(item.product_id || 0) > 0 && !item.product_detail_ready)) {
    throw new Error('正在加载产品完整规格，请稍候再提交')
  }
  if (isOrdinaryTransfer()) for (const [index, item] of form.items.entries()) {
    if (!selectedObjectID(item) || !selectedTransferSpec(item) || !item.batch_code || !selectedTransferBatch(item)) throw new Error(`第 ${index + 1} 行请选择来源商品、规格和批次`)
    if (!item.from_warehouse || !item.to_warehouse || item.from_warehouse === item.to_warehouse) throw new Error(`第 ${index + 1} 行请选择不同的出库仓和入库仓`)
    if (!(Number(item.quantity) > 0) || Number(item.quantity) > remainingTransferBatch(item)) throw new Error(`第 ${index + 1} 行数量无效或超过批次可转库存`)
    if (ownerTransferCustomerID(item) && Number(item.target_owner_customer_id || 0) !== ownerTransferCustomerID(item)) throw new Error(`第 ${index + 1} 行请确认本次货权转移`)
  }
  const requestItems = stockDocumentPositiveItems(form.items, (item) => (
    isOrdinaryTransfer() ? transferQuantity(item) : usesSingleQuantity(item)
      ? canonicalQuantity(item)
      : { qty_g: Number(item.qty_g || 0), qty_units: Number(item.qty_units || 0) }
  ))
  return {
    purpose: normalizedPurpose(form.purpose_key),
    is_return: form.purpose_key === 'material_return_from_manufacture',
    work_order_id: Number(form.work_order_id || 0),
    job_card_id: Number(form.job_card_id || 0),
    running_item_id: Number(form.running_item_id || 0),
    source_type: form.source_type || '',
    source_id: Number(form.source_id || 0),
    return_source: form.return_source || '',
    note: form.note || '',
    items: requestItems.map(({ item, quantity }) => {
      const identity = isOrdinaryTransfer() ? { product_id: item.product_id, bom_spec_id: item.bom_spec_id, bom_variant_id: item.bom_variant_id, qty: item.quantity, unit: item.inventory_unit } : buildProductSpecWriteIdentity({ ...item, parent_product_id: item.product_id, qty: Number(item.quantity || quantity.qty_units || quantity.qty_g || 0), unit: item.inventory_unit })
      return {
        material_id: Number(item.material_id || 0), product_id: item.item_type === 'finished_product' ? Number(identity.product_id || 0) : 0,
        ...(item.item_type === 'finished_product' && (isOrdinaryTransfer() ? item.bom_spec_id > 0 : itemUsesBomSpecs(item)) ? {
          bom_spec_id: Number(identity.bom_spec_id || 0),
          bom_variant_id: Number(identity.bom_variant_id || 0),
          qty: Number(identity.qty || 0),
          unit: String(identity.unit || item.inventory_unit || ''),
        } : {}),
        item_type: item.item_type, item_name: item.item_name || '', spec_g: Number(item.spec_g || 0),
        inventory_unit: item.inventory_unit || '', quantity_basis: item.quantity_basis || '',
        from_warehouse: item.from_warehouse || '', to_warehouse: item.to_warehouse || '',
        qty_g: quantity.qty_g, qty_units: quantity.qty_units,
        batch_code: item.batch_code || '', owner_customer_id: Number(item.owner_customer_id || 0), target_owner_customer_id: Number(item.target_owner_customer_id || 0), unit_cost: isReceipt.value ? Number(item.unit_cost || 0) : 0,
        supplier: item.supplier || '', crop_season: item.crop_season || '', origin: item.origin || '',
        producer_flavor_description: item.producer_flavor_description || '',
      }
    }),
  }
}

async function saveDraft() {
	if (isPlanMaterialTransfer.value) { drawerError.value = '请返回生产计划修改调拨安排'; return null }
  saving.value = true
  drawerError.value = ''
  try {
    const data = await apiSend(form.id ? `${stockEntryEndpoint()}/${form.id}` : stockEntryEndpoint(), {
      method: form.id ? 'PUT' : 'POST',
      body: requestBody(),
    })
    applyDocument(data)
    await load()
    return data
  } catch (err) {
    drawerError.value = err.message || '保存草稿失败'
    return null
  } finally {
    saving.value = false
  }
}

async function submitDocument() {
  if (saving.value) return
  const receiptOptions = isMaterialProductionReceipt.value ? { completion_mode: receiptMode.value, consumed_input_g: Math.round(Number(receiptInputKg.value || 0) * 1000) } : {}
  const draft = isPlanMaterialTransfer.value && form.id ? { id: form.id } : await saveDraft()
  if (!draft?.id) return
  saving.value = true
  drawerError.value = ''
  try {
    const data = await apiSend(`${stockEntryEndpoint()}/${draft.id}/submit`, { body: receiptOptions })
    applyDocument(data)
    await loadCompletionResult(data)
    await load()
  } catch (err) {
    drawerError.value = err.message || '提交过账失败'
  } finally {
    saving.value = false
  }
}

async function loadCompletionResult(document = {}) {
  completionResult.value = null
  const isFinishedReceipt = String(document.status || '') === 'submitted'
    && String(document.purpose || document.entry_type || '') === 'manufacture'
    && Number(document.work_order_id || 0) > 0
    && (document.items || []).some((item) => item.item_type === 'finished_product')
  if (!isFinishedReceipt) return
  completionResultLoading.value = true
  try {
    const workOrder = await apiGet(`/api/produce/work-orders/${Number(document.work_order_id)}`)
    const hub = workOrder.execution_hub || {}
    const header = hub.header || workOrder.work_order || {}
    const workOrderRow = workOrder.work_order || {}
    const customerID = Number(header.customer_id || workOrderRow.customer_id || document.items?.[0]?.owner_customer_id || 0)
    const processingRequestItemID = Number(header.processing_request_item_id || workOrderRow.processing_request_item_id || 0)
    let request = null
    if (customerID > 0 && processingRequestItemID > 0) {
      const requestData = await apiGet(`/api/customer-processing/internal/${customerID}/processing-requests?limit=100`)
      request = (requestData.rows || []).find((candidate) => (candidate.items || []).some((item) => Number(item.id || 0) === processingRequestItemID)) || null
    }
    const summary = customerProcessingReceiptResult({ document, workOrder, request: request || {}, processingRequestItemID })
    let inventoryTotalQty = summary.inboundQty
    const productID = Number(header.output_product_id || workOrderRow.output_product_id || workOrderRow.product_id || document.items?.[0]?.product_id || 0)
    if (productID > 0 && summary.warehouse) {
      const inventoryURL = new URL('/api/stock/warehouse-inventory', window.location.origin)
      inventoryURL.searchParams.set('warehouse', summary.warehouse)
      inventoryURL.searchParams.set('item_type', 'finished_product')
      inventoryURL.searchParams.set('q', String(header.output_name || workOrderRow.output_name || workOrderRow.product_name || document.items?.[0]?.item_name || ''))
      inventoryURL.searchParams.set('limit', '500')
      if (customerID > 0) inventoryURL.searchParams.set('customer_id', String(customerID))
      const inventory = await apiGet(inventoryURL)
      const bomSpecID = Number(workOrderRow.bom_spec_id || document.items?.[0]?.bom_spec_id || 0)
      const specG = Number(workOrderRow.spec_g || document.items?.[0]?.spec_g || 0)
      inventoryTotalQty = (inventory.rows || [])
        .filter((row) => Number(row.item_id || 0) === productID)
        .filter((row) => bomSpecID > 0 ? Number(row.bom_spec_id || 0) === bomSpecID : Number(row.spec_g || 0) === specG)
        .reduce((sum, row) => sum + Number(row.qty_units || 0), 0)
    }
    completionResult.value = {
      ...summary,
      customerID,
      customerName: request?.customer_name || '',
      inventoryTotalQty,
      availableQty: Math.max(0, inventoryTotalQty - summary.occupiedQty),
    }
  } catch (err) {
    drawerWarnings.value = [...drawerWarnings.value, err.message || '完工入库结果核对失败，请从工单或库存详情重试']
  } finally {
    completionResultLoading.value = false
  }
}

async function cancelDocument(row) {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    await apiSend(`${stockEntryEndpoint()}/${row.id}/cancel`, { body: {} })
    await load()
  } catch (err) {
    error.value = err.message || '取消失败'
  } finally {
    saving.value = false
  }
}

function applyDocument(data = {}, warnings = [], previewState = {}) {
  clearTransferInventory()
  clearTransferChoices()
  const fallbackWorkOrderNo = String(data.work_order_no || form.work_order_no || props.viewParams?.work_order_no || '').trim()
  resetForm({
    ...emptyDocument(), ...data,
    work_order_no: fallbackWorkOrderNo,
    purpose_key: data.is_return ? 'material_return_from_manufacture' : (data.purpose || data.entry_type || 'material_transfer'),
    items: (data.items || []).map(normalizedItem),
  })
  savedDraftFingerprint.value = draftFingerprint()
  drawerWarnings.value = Array.isArray(warnings) ? warnings.filter(Boolean) : []
  Object.assign(stockPreviewState, { availability: previewState.availability || 'actionable', message: previewState.message || '' })
  drawerOpen.value = true
  for (const item of form.items) {
    if (isPlanMaterialTransfer.value) continue
    if (isOrdinaryTransfer()) { if (data.status === 'draft') void ensureChoiceInventory(item.from_warehouse); continue }
    if (item.item_type === 'finished_product' && Number(item.product_id || 0) > 0) {
      if (isFinishedTransfer(item)) void ensureTransferInventory(item.from_warehouse)
      void ensureProductDetailForItem(item)
    }
  }
  loadMaterialBalances()
}

async function loadMaterialBalances() {
  const request = ++materialBalanceRequest
  const groups = new Map()
  for (const item of form.items) {
    item.source_balance = { book_qty: 0, available_qty: 0, frozen_qty: 0 }
    if (item.item_type !== 'material' || !Number(item.material_id || 0) || !item.from_warehouse) continue
    const ids = groups.get(item.from_warehouse) || new Set()
    ids.add(Number(item.material_id))
    groups.set(item.from_warehouse, ids)
  }
  try {
    for (const [warehouse, ids] of groups.entries()) {
      const url = new URL('/api/stock/material-balances', window.location.origin)
      url.searchParams.set('warehouse', warehouse)
      url.searchParams.set('material_ids', [...ids].join(','))
      const data = await apiGet(url)
      if (request !== materialBalanceRequest) return
      const byID = new Map((data.rows || []).map((row) => [Number(row.material_id || 0), row]))
      for (const item of form.items) {
        if (item.from_warehouse === warehouse && item.item_type === 'material') item.source_balance = byID.get(Number(item.material_id || 0)) || item.source_balance
      }
    }
  } catch (err) {
    if (request !== materialBalanceRequest) return
    drawerError.value = err.message || '加载来源仓库存失败'
  }
}

async function openExisting(row) {
  drawerError.value = ''
  try {
    await loadOptions()
    const action = productionStockDocumentPreviewAction(row)
    const workOrderID = Number(row.work_order_id || 0)
    if (row.status === 'draft' && workOrderID > 0 && action) {
      const preview = await apiSend(`/api/produce/work-orders/${workOrderID}/stock-document-preview`, {
        body: { action, stock_document_id: Number(row.id || 0), return_source: 'stock_document_list' },
      })
      applyDocument({
        ...preview.document,
        work_order_no: preview.document?.work_order_no || preview.work_order?.work_order_no || row.work_order_no || '',
        status: 'draft',
      }, preview.warnings)
      Object.assign(stockPreviewState, { availability: preview.availability || 'actionable', message: preview.message || '' })
      return
    }
    const data = await apiGet(`${stockEntryEndpoint()}/${row.id}`)
    applyDocument(data)
    await loadCompletionResult(data)
  } catch (err) {
    error.value = err.message || '加载单据失败'
  }
}

function queryURL() {
  const url = new URL(stockEntryEndpoint(), window.location.origin)
  if (filters.q) url.searchParams.set('q', filters.q)
  if (filters.purpose) url.searchParams.set('purpose', filters.purpose)
  if (filters.status) url.searchParams.set('status', filters.status)
  if (Number(filters.work_order_id || 0) > 0) url.searchParams.set('work_order_id', String(filters.work_order_id))
  return url
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(queryURL())
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  if (baseOptionsLoaded.value) return
  if (baseOptionsRequest) return baseOptionsRequest
  baseOptionsLoading.value = true
  baseOptionsRequest = (async () => {
    try {
      const [materialData, warehouseData] = await Promise.all([
        apiGet('/api/materials?limit=500'), apiGet('/api/stock/warehouses'),
      ])
      materials.value = materialData.rows || []
      warehouses.value = warehouseData.rows || []
      baseOptionsLoaded.value = true
    } catch (err) {
      error.value = err.message || '基础资料加载失败'
      throw err
    } finally {
      baseOptionsLoading.value = false
      baseOptionsRequest = null
    }
  })()
  return baseOptionsRequest
}

function mergeProductRows(rows = [], keep = []) {
  const byID = new Map()
  for (const row of [...keep, ...products.value, ...rows]) {
    const id = Number(row?.id || 0)
    if (!id) continue
    const current = byID.get(id)
    byID.set(id, current
      ? (current.product_detail_loaded && !row.product_detail_loaded ? current : { ...current, ...row })
      : row)
  }
  return [...byID.values()]
}

async function loadProductOptions(query = productSearchQuery.value, page = 1, append = false) {
  const requestID = ++productOptionsRequest
  productOptionsLoading.value = true
  const url = new URL('/api/products/options', window.location.origin)
  if (query) url.searchParams.set('q', query)
  url.searchParams.set('page', String(page))
  url.searchParams.set('limit', '20')
  try {
    const data = await apiGet(url)
    if (requestID !== productOptionsRequest) return
    const selected = form.items
      .filter((item) => item.item_type === 'finished_product' && Number(item.product_id || 0) > 0)
      .map((item) => products.value.find((row) => Number(row.id || 0) === Number(item.product_id || 0)))
      .filter(Boolean)
    products.value = append ? mergeProductRows(data.rows || []) : mergeProductRows(data.rows || [], selected)
    productOptionsLoaded.value = true
    productSearchQuery.value = query
    productOptionsPage.value = Number(data.page || page)
    productOptionsHasNext.value = Boolean(data.has_next)
  } catch (err) {
    if (requestID === productOptionsRequest) error.value = err.message || '产品选项加载失败'
  } finally {
    if (requestID === productOptionsRequest) productOptionsLoading.value = false
  }
}

function isFinishedTransfer(item) {
  return form.purpose_key === 'material_transfer' && item.item_type === 'finished_product'
}

function isOrdinaryTransfer() { return form.purpose_key === 'material_transfer' && !isBoundProductionDocument.value }
function clearTransferChoices() {
  transferChoiceGeneration += 1
  transferChoiceRequests.clear()
  for (const key of Object.keys(transferChoices)) delete transferChoices[key]
}
async function ensureChoiceInventory(warehouse) {
  if (!warehouse || transferChoices[warehouse]?.loaded) return
  if (transferChoiceRequests.has(warehouse)) return transferChoiceRequests.get(warehouse)
  const generation = transferChoiceGeneration
  transferChoices[warehouse] = { rows: [], loading: true, loaded: false, error: false }
  const request = loadTransferOptions(warehouse, apiGet).then(rows => {
    if (generation !== transferChoiceGeneration) return
    transferChoices[warehouse] = { rows, loading: false, loaded: true, error: false }
    for (const item of form.items) if (item.from_warehouse === warehouse && item.batch_code && !item.transfer_spec) hydrateTransferDraft(item)
  }).catch(err => {
    if (generation !== transferChoiceGeneration) return
    transferChoices[warehouse] = { rows: [], loading: false, loaded: false, error: true }
    drawerError.value = err.message || '来源仓库存加载失败'
  }).finally(() => { if (transferChoiceRequests.get(warehouse) === request) transferChoiceRequests.delete(warehouse) })
  transferChoiceRequests.set(warehouse, request)
  return request
}
function transferChoiceRows(item) { return transferChoices[item.from_warehouse]?.rows || [] }
function transferObjectOptions(item) {
  const map = new Map()
  for (const row of transferChoiceRows(item)) if (row.item_type === item.item_type && !map.has(row.item_id)) map.set(row.item_id, { id: row.item_id, name: row.item_name, code: row.item_code || String(row.item_id) })
  return [...map.values()]
}
function transferChoiceEmptyText(item) {
  if (!item.from_warehouse) return '请先选择出库仓'
  const state = transferChoices[item.from_warehouse]
  if (state?.loading) return '正在加载来源仓库存…'
  if (state?.error) return '库存加载失败，请重试'
  return '来源仓没有匹配库存'
}
function transferItemSpecs(item) { return transferSpecs(transferChoiceRows(item), item.item_type, selectedObjectID(item)) }
function selectedTransferSpec(item) { return transferItemSpecs(item).find(spec => spec.key === item.transfer_spec) }
function transferItemBatches(item) { return transferBatches(transferChoiceRows(item), item.item_type, selectedObjectID(item), item.transfer_spec) }
function selectedTransferBatch(item) { return transferItemBatches(item).find(row => row.batch_code === item.batch_code) }
function ownerTransferCustomerID(item) {
  if (item.item_type !== 'finished_product' || Number(item.bom_spec_id || 0) <= 0 || Number(item.owner_customer_id || 0) !== 0) return 0
  return Number(warehouses.value.find(row => row.code === item.to_warehouse)?.customer_id || 0)
}
function transferRowAmount(row, item, available) { const spec = selectedTransferSpec(item); return row && spec ? transferAmount(row, spec, available ? 'available' : 'book').amount : 0 }
function transferSpecBalance(item, available) {
  const spec = selectedTransferSpec(item)
  if (!spec) return '-'
  return `${numberLabel(transferItemBatches(item).reduce((sum, row) => sum + transferRowAmount(row, item, available), 0))} ${spec.unit}`
}
function remainingTransferBatch(item) {
  const row = selectedTransferBatch(item)
  if (!row) return 0
  const used = form.items.filter(other => other !== item && other.from_warehouse === item.from_warehouse && other.item_type === item.item_type && selectedObjectID(other) === selectedObjectID(item) && other.transfer_spec === item.transfer_spec && other.batch_code === item.batch_code).reduce((sum, other) => sum + Number(other.quantity || 0), 0)
  return Math.max(0, transferRowAmount(row, item, true) - used)
}
function resetTransferSelection(item) {
  resetItemObject(item)
  item.transfer_spec = ''; item.batch_code = ''; item.quantity = 0; item.owner_customer_id = 0; item.target_owner_customer_id = 0
}
function changeTransferWarehouse(item) { resetTransferSelection(item); void ensureChoiceInventory(item.from_warehouse) }
function selectTransferObject(item, id) {
  const selected = transferObjectOptions(item).find(row => Number(row.id) === Number(id))
  resetTransferSelection(item)
  if (item.item_type === 'material') item.material_id = Number(id || 0)
  else item.product_id = Number(id || 0)
  item.item_name = selected?.name || ''
  const specs = transferItemSpecs(item)
  if (specs.length === 1) { item.transfer_spec = specs[0].key; changeTransferSpec(item) }
}
function changeTransferSpec(item) {
  const spec = selectedTransferSpec(item)
  item.batch_code = ''; item.quantity = 0; item.owner_customer_id = 0; item.target_owner_customer_id = 0
  item.bom_spec_id = spec?.bomSpecID || 0; item.bom_variant_id = spec?.variantID || 0
  item.spec_g = spec?.specG || 0; item.inventory_unit = spec?.inventoryUnit || spec?.unit || ''
}
function changeTransferBatch(item) { item.quantity = 0; item.owner_customer_id = Number(selectedTransferBatch(item)?.owner_customer_id || 0); item.target_owner_customer_id = 0 }
function hydrateTransferDraft(item) {
  const specs = transferItemSpecs(item)
  const spec = specs.find(s => item.bom_spec_id > 0 ? s.bomSpecID === Number(item.bom_spec_id) && s.variantID === Number(item.bom_variant_id) : item.item_type === 'material' ? true : s.specG === Number(item.spec_g) && s.mode === (Number(item.qty_units) > 0 ? 'count' : 'loose'))
  if (!spec) return
  item.transfer_spec = spec.key
  item.quantity = spec.mode === 'weight' ? Number(item.qty_g || 0) / transferWeightFactor(spec.unit) : spec.mode === 'loose' ? Number(item.qty_g || 0) : Number(item.qty_units || 0)
}
function transferQuantity(item) {
  const spec = selectedTransferSpec(item); const quantity = Number(item.quantity || 0)
  if (!spec || !Number.isFinite(quantity) || quantity <= 0) throw new Error('请填写有效数量')
  if (spec.mode !== 'weight' && !Number.isInteger(quantity)) throw new Error('件数和克数必须为整数')
  if (spec.mode === 'count' || spec.mode === 'canonical_weight') return { qty_g: 0, qty_units: quantity }
  const grams = spec.mode === 'loose' ? quantity : quantity * transferWeightFactor(spec.unit)
  if (Math.round(grams) <= 0) throw new Error('数量精度不能小于 1 克')
  return { qty_g: Math.round(grams), qty_units: 0 }
}

function productOptionsForItem(item) {
  if (!isFinishedTransfer(item)) return products.value
  const rows = transferInventory[item.from_warehouse]?.rows || []
  return transferProductOptions(rows).map((option) => ({
    ...option,
    ...(products.value.find((row) => Number(row.id) === option.id) || {}),
  }))
}

function transferOptionsEmptyText(item) {
  if (!item.from_warehouse) return '请先选择出库仓'
  const state = transferInventory[item.from_warehouse]
  if (state?.loading) return '正在加载来源仓库存…'
  if (state?.error) return '库存加载失败，请重新打开选择框重试'
  return '来源仓没有匹配的商品库存'
}

function clearTransferInventory() {
  transferInventoryGeneration += 1
  transferInventoryRequests.clear()
  for (const warehouse of Object.keys(transferInventory)) delete transferInventory[warehouse]
}

async function ensureTransferInventory(warehouse) {
  if (!warehouse || transferInventory[warehouse]?.loaded) return
  if (transferInventoryRequests.has(warehouse)) return transferInventoryRequests.get(warehouse)
  transferInventory[warehouse] = { rows: [], loading: true, loaded: false, error: false }
  const generation = transferInventoryGeneration
  const request = loadTransferInventory(warehouse, apiGet).then((rows) => {
    if (generation !== transferInventoryGeneration) return
    transferInventory[warehouse] = { rows, loading: false, loaded: true, error: false }
  }).catch((err) => {
    if (generation !== transferInventoryGeneration) return
    transferInventory[warehouse] = { rows: [], loading: false, loaded: false, error: true }
    drawerError.value = err.message || '来源仓库存加载失败'
  }).finally(() => {
    if (transferInventoryRequests.get(warehouse) === request) transferInventoryRequests.delete(warehouse)
  })
  transferInventoryRequests.set(warehouse, request)
  return request
}

function handleItemProductOptionsFocus(item) {
  if (isFinishedTransfer(item)) void ensureTransferInventory(item.from_warehouse)
  else handleProductOptionsFocus()
}

function handleSourceWarehouseChange(item) {
  if (!isFinishedTransfer(item)) return
  resetItemObject(item)
  void ensureTransferInventory(item.from_warehouse)
}

function handleProductOptionsFocus() {
  if (!productOptionsLoaded.value || productSearchQuery.value) {
    void loadProductOptions(productSearchQuery.value, 1, false)
  }
}

function handleProductSearch(query) {
  productSearchQuery.value = String(query || '').trim()
  if (productSearchTimer) window.clearTimeout(productSearchTimer)
  productSearchTimer = window.setTimeout(() => {
    productSearchTimer = 0
    void loadProductOptions(productSearchQuery.value, 1, false)
  }, 250)
}

function loadMoreProductOptions() {
  if (productOptionsLoading.value || !productOptionsHasNext.value) return
  void loadProductOptions(productSearchQuery.value, productOptionsPage.value + 1, true)
}

async function applyViewParams(params = {}) {
  receiptMode.value = params.receipt_mode === 'partial' ? 'partial' : 'final'
  receiptInputKg.value = 0
  const workOrderID = Number(params.work_order_id || 0)
  const stockEntryID = Number(params.stock_entry_id || 0)
  filters.work_order_id = workOrderID
  if (stockEntryID > 0) {
    const row = rows.value.find((candidate) => Number(candidate.id || 0) === stockEntryID) || { id: stockEntryID }
    await openExisting(row)
    return
  }
  let action = String(params.action || '').trim()
  if (!action && (params.tab === 'wip' || Number(params.shortage_g || 0) > 0)) action = 'issue'
  if (action === 'receipt') {
    openPurchase()
    return
  }
  if (!workOrderID || !['issue', 'supplement', 'return', 'consume', 'finish'].includes(action)) return
  await loadOptions().catch(() => {})
  drawerError.value = ''
  try {
    const preview = await apiSend(`/api/produce/work-orders/${workOrderID}/stock-document-preview`, {
      body: {
        action, job_card_id: Number(params.job_card_id || 0), return_source: params.return_source || 'work_order',
      },
    })
    applyDocument({
      ...preview.document,
      work_order_no: preview.document?.work_order_no || preview.work_order?.work_order_no || params.work_order_no || '',
      status: 'draft',
    }, preview.warnings)
    Object.assign(stockPreviewState, { availability: preview.availability || 'actionable', message: preview.message || '' })
  } catch (err) {
    openNewDrawer(action === 'return' ? 'material_return_from_manufacture' : 'material_transfer_for_manufacture')
    form.work_order_id = workOrderID
    form.work_order_no = String(params.work_order_no || '').trim()
    form.items = []
    Object.assign(stockPreviewState, { availability: 'unavailable', message: err.message || '工单库存预填失败' })
  }
}

function openWipInventory() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'warehouseInventory', params: { warehouse: 'wip', item_type: 'material' } },
  }))
}

function navigateFromCompletion(key, params = {}) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key,
      params,
      returnNavigation: { key: 'stockOperations', label: '返回完工入库结果', params: { tab: 'stockEntries', work_order_id: completionResult.value?.workOrderID, stock_entry_id: Number(form.id || 0) } },
    },
  }))
}

function openCompletionWorkOrder() {
  navigateFromCompletion('workOrders', { work_order_id: completionResult.value?.workOrderID })
}

function openCompletionRequest() {
  navigateFromCompletion('customerProcessing', { customer_id: completionResult.value?.customerID, processing_request_no: completionResult.value?.requestNo })
}

function openCompletionOrder(order) {
  navigateFromCompletion('customerOrders', { customer_id: completionResult.value?.customerID, q: order.order_no, order_id: order.order_id })
}

function openCompletionInventory() {
  navigateFromCompletion('warehouseInventory', { warehouse: completionResult.value?.warehouse, item_type: 'finished_product', customer_id: completionResult.value?.customerID })
}

function openPurchase() {
  closeDrawer()
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'purchase' } }))
}

watch(() => props.viewParams, (params) => applyViewParams(params || {}), { deep: true })
watch(() => form.purpose_key, (purpose) => {
  if (purpose !== 'material_receipt') return
  for (const item of form.items) {
    const selectedMaterial = materials.value.find((row) => Number(row.id || 0) === Number(item.material_id || 0))
    if (isSemiFinishedMaterial(selectedMaterial)) resetItemObject(item)
  }
})
watch(() => form.items.map((item) => `${item.item_type}:${item.material_id}:${item.from_warehouse}`).join('|'), loadMaterialBalances)
onMounted(async () => {
  await load()
  await applyViewParams(props.viewParams || {})
})
</script>

<style scoped>
.plan-transfer-readonly{border:0;padding:0;margin:0;min-width:0;display:grid;gap:12px}.plan-transfer-readonly h4{margin:0}.plan-transfer-readonly .item-card{margin:0}
.return-dialog-mask{position:fixed;inset:0;z-index:90;display:grid;place-items:center;background:rgba(17,24,39,.45);padding:16px}.return-dialog{width:min(540px,100%);box-sizing:border-box;background:#fff;border-radius:12px;padding:22px;box-shadow:0 16px 48px #0003;outline:none}.return-dialog h3{margin:0 0 12px}.return-dialog p{line-height:1.6}.return-dialog-actions{display:flex;flex-wrap:wrap;gap:10px}.return-dialog-actions button{flex:1 1 130px}
.stock-entry-page,.stock-entry-page *{box-sizing:border-box}.stock-entry-page{padding:16px;display:grid;gap:16px}.stock-entry-page.embedded{padding:0}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head,.drawer-head,.line-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.panel-head{margin-bottom:12px}.panel-head h2,.drawer-head h3,.line-head h4{margin:0 0 4px}.panel-head p,.drawer-head p{margin:0;color:#6b7280;font-size:13px}.head-actions,.row-actions,.drawer-actions{display:flex;gap:8px;flex-wrap:wrap}.filters{display:grid;grid-template-columns:minmax(200px,1.5fr) repeat(2,minmax(130px,1fr)) auto;gap:10px;align-items:end}label{min-width:0}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,input,button{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #d1d5db;padding:7px 9px}.readonly-value{display:block;min-height:36px;border:1px solid #d1d5db;border-radius:6px;background:#f9fafb;padding:7px 9px;color:#374151}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.link,.danger-link{border:0;background:transparent;color:#1d4ed8;padding:0;min-height:0}.danger-link{color:#b91c1c}.disabled{color:#9ca3af}.legacy-readonly{color:#6b7280}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.warning-list{background:#fffbeb;border:1px solid #fcd34d;border-radius:8px;padding:8px 10px;color:#92400e}.warning-list p{margin:0}.warning-list p+p{margin-top:4px}.production-issue-hint{margin:0;padding:6px 8px;border-radius:6px;background:#eff6ff;color:#1e40af;font-size:12px}.source-balance{display:block;margin-top:4px;color:#1e40af;font-size:11px;line-height:1.35}.table-wrap{overflow:auto}table{width:100%;min-width:1050px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px}.status.submitted{border-color:#86efac;background:#f0fdf4;color:#15803d}.status.draft{border-color:#fcd34d;background:#fffbeb;color:#a16207}.muted{text-align:center;color:#666}.drawer-mask{position:fixed;inset:0;background:rgba(17,24,39,.35);z-index:80;display:flex;justify-content:flex-end}.drawer{width:min(980px,96vw);height:100%;overflow:auto;background:#fff;padding:18px;box-shadow:-12px 0 32px rgba(15,23,42,.2);display:grid;align-content:start;gap:16px}.drawer-head{border-bottom:1px solid #e5e7eb;padding-bottom:12px}.document-form,.item-grid{display:grid;grid-template-columns:repeat(4,minmax(130px,1fr));gap:10px}.wide{grid-column:span 2}.line-head{align-items:center}.item-card{border:1px solid #e5e7eb;border-radius:8px;padding:12px;display:grid;gap:10px}.compact-production-items{display:grid;gap:4px}.compact-production-items-head,.compact-production-item-grid{display:grid;grid-template-columns:minmax(150px,2fr) minmax(90px,1fr) minmax(90px,1fr) minmax(90px,.8fr) minmax(64px,.55fr) minmax(120px,1.2fr) 52px;gap:6px;align-items:end}.compact-production-items-head{padding:0 6px;color:#6b7280;font-size:12px;font-weight:600}.compact-production-item{border:1px solid #e5e7eb;border-radius:6px;padding:4px 6px}.compact-production-item-grid>label{margin:0}.compact-production-item-grid .mobile-field-label{display:none}.compact-production-item-grid select,.compact-production-item-grid input,.compact-production-item-grid .readonly-value{min-height:32px;height:32px;padding:4px 6px}.compact-delete{align-self:center;justify-self:center}.compact-allocations{margin-top:4px}.allocations{display:flex;flex-wrap:wrap;gap:6px}.allocations span{background:#eff6ff;border:1px solid #bfdbfe;border-radius:999px;padding:3px 8px;font-size:12px}.drawer-actions{justify-content:flex-end;border-top:1px solid #e5e7eb;padding-top:12px}.select-loading,.select-load-more{display:block;width:100%;padding:8px 10px;text-align:center;color:#6b7280;font-size:12px;border:0;background:#f9fafb}.select-load-more{color:#1d4ed8;cursor:pointer}
.stock-preview-state{border:1px solid #e4d3ad;border-radius:9px;background:#fff9eb;padding:16px;color:#815418}.stock-preview-state.ready{border-color:#b9ddc5;background:#eff9f2;color:#237146}.stock-preview-state p{margin:6px 0 12px;line-height:1.6}
.completion-result{border:1px solid #cbded2;border-radius:12px;background:linear-gradient(135deg,#f2f8f4,#fff);padding:16px;display:grid;gap:14px}.completion-result-head{display:flex;justify-content:space-between;gap:14px;align-items:flex-start}.completion-result-head span,.completion-trace-grid span,.completion-metrics span{color:#65746b;font-size:12px}.completion-result-head h4{margin:3px 0;font-size:20px;color:#183b2b}.completion-result-head p{margin:0;color:#65746b;font-size:13px}.completion-result-loading{padding:18px;text-align:center;color:#65746b}.completion-metrics{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.completion-metrics>div{border:1px solid #dce9e1;border-radius:10px;background:#fff;padding:12px;display:grid;gap:5px}.completion-metrics strong{font-size:20px;color:#236844}.completion-trace-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.completion-trace-grid>div{display:grid;gap:4px}.completion-conversions{border-top:1px solid #dce9e1;padding-top:12px;display:grid;gap:8px}.completion-conversions>p{margin:0;color:#65746b}.completion-conversions article{display:flex;align-items:center;justify-content:space-between;gap:12px;border:1px solid #e2e9e5;border-radius:9px;background:#fff;padding:10px}.completion-conversions article div{display:grid;gap:3px}.completion-conversions article span{font-size:12px;color:#65746b}.completion-actions{display:flex;justify-content:flex-end;gap:8px;flex-wrap:wrap}
@media(max-width:900px){.stock-entry-page{padding:12px}.panel-head,.drawer-head{display:grid}.filters,.document-form,.item-grid{grid-template-columns:1fr}.wide{grid-column:auto}.drawer{width:100%}.compact-production-items-head{display:none}.compact-production-item-grid{grid-template-columns:1fr 1fr;align-items:end}.compact-production-item-grid .mobile-field-label{display:block}.compact-production-item-grid .compact-material-name{grid-column:1/-1}.compact-delete{justify-self:start;margin-top:4px}.completion-metrics{grid-template-columns:repeat(2,minmax(0,1fr))}.completion-trace-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:560px){.compact-production-item-grid{grid-template-columns:1fr}.compact-production-item-grid .compact-material-name{grid-column:auto}}
</style>
