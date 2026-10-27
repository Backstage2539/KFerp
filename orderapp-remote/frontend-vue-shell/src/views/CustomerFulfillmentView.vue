<template>
  <div class="page customer-fulfillment">
    <ProductionReturnLink :source="viewParams.return_navigation" />
    <section class="workspace-header">
      <div class="workspace-heading"><div><p class="eyebrow">客户管理 / 履约运营</p><h2>客户履约运营台</h2><p class="workspace-subtitle">{{ overview.customer_name || selectedCustomerLabel || '选择客户，开始处理履约业务' }}</p></div>
        <div class="head-actions"><button class="secondary icon-button" type="button" @click="openManual"><IconBook :size="16" />使用手册</button><button class="secondary icon-button" type="button" @click="loadAll" :disabled="loading || !normalizedCustomerId"><IconRefresh :size="16" />刷新</button><button class="secondary icon-button" type="button" @click="settingsDrawerOpen = true" :disabled="!normalizedCustomerId"><IconSettings :size="16" />客户设置</button></div>
      </div>
      <div class="customer-context"><label class="customer-picker-field"><span>当前客户</span><SearchableSelect v-model="customerId" :options="customerOptions" :option-label="customerFulfillmentCustomerOptionLabel" :option-meta="customerFulfillmentCustomerOptionMeta" :option-value="optionNumericValue" placeholder="搜索并切换客户" empty-text="没有匹配客户" :disabled="loading" @select="selectCustomer" /></label>
        <div class="customer-service-tags"><span v-for="mode in effectiveSettings?.modes?.filter(row => row.enabled) || []" :key="mode.mode" class="service-tag">{{ businessLabel(mode.mode) }}</span><span class="configuration-caption">{{ configurationCaption }}</span></div>
      </div>
    </section>
    <nav class="module-tabs" role="tablist" aria-label="客户履约模块"><button v-for="tab in moduleTabs" :key="tab.key" type="button" role="tab" :aria-selected="activeTab === tab.key" :class="{ active: activeTab === tab.key }" @click="changeTab(tab.key)">{{ tab.label }}</button></nav>
    <div v-if="error" class="error workspace-message" role="alert">{{ error }}</div><div v-if="ok" class="ok workspace-message" role="status">{{ ok }}</div>
    <div v-if="!normalizedCustomerId" class="workspace-empty"><IconUsers :size="36" /><h3>先选择一个履约客户</h3><p>查看客户待办、库存、订单和费用。</p></div>
    <template v-else>
      <div v-if="loading && !overview.customer_id" class="workspace-empty" role="status">正在加载客户工作台…</div>
      <FulfillmentOperationsOverview v-else-if="activeTab === 'overview'" :totals="overview.operations || {}" @navigate="openTodo" />
      <div v-if="activeTab !== 'overview'" class="module-toolbar"><div><h3>{{ activeModule.label }}管理</h3><p>{{ activeModule.description }}</p></div><div class="head-actions">
        <button v-if="activeTab === 'inventory'" class="secondary" type="button" @click="activeAction = activeAction === 'inventory' ? '' : 'inventory'" :disabled="!workbenchSections.inventory" :title="!workbenchSections.inventory ? '未开通库存托管，仍可查看历史库存' : ''">调整库存</button>
        <button v-if="activeTab === 'orders'" class="secondary" type="button" @click="activeAction = activeAction === 'processing' ? '' : 'processing'" :disabled="!workbenchSections.processing">提交加工工单</button>
        <button v-if="activeTab === 'dispatch'" class="primary icon-button" type="button" @click="activeAction = activeAction === 'directShip' ? '' : 'directShip'" :disabled="!workbenchSections.directShip"><IconPlus :size="17" />新建代发</button>
        <button v-if="activeTab === 'fees' && billingView === 'unbilled'" class="primary" type="button" @click="openBillingPreview" :disabled="!selectedBillingSources.size || billingPreviewLoading">出账{{ selectedBillingSources.size ? `（${selectedBillingSources.size}）` : '' }}</button>
        <button class="secondary" type="button" @click="activeAction = activeAction === 'import' ? '' : 'import'" :disabled="!workbenchSections.imports">导入数据</button>
      </div></div>
      <p v-if="activeTab !== 'overview' && !moduleCanCreate" class="readonly-notice">此客户未开通当前业务，历史数据仍可查看；如需新建，请先调整客户能力与履约设置。</p>
      <FulfillmentInventory v-if="activeTab === 'inventory'" :key="normalizedCustomerId" :rows="overview.assets || []" />
      <div v-if="activeTab === 'fees'" class="fee-summary-grid billing-summary-grid">
        <div><span>合计未付</span><strong>¥ {{ moneyFromCents(billingTotals.total_unpaid_cents || 0) }}</strong></div>
        <div><span>未出账</span><strong>¥ {{ moneyFromCents(billingTotals.unbilled_cents || 0) }}</strong></div>
        <div><span>已出账</span><strong>¥ {{ moneyFromCents(billingTotals.issued_cents || 0) }}</strong></div>
      </div>
    </template>
    <section v-if="activeAction" class="panel control-panel action-panel"><div class="panel-head"><h3>{{ actionTitles[activeAction] }}</h3><button class="secondary" @click="activeAction = ''">收起</button></div>
      <div v-if="activeAction === 'import' && workbenchSections.imports" class="import-row">
        <div class="segmented">
          <button
            v-for="option in visibleImportTypes"
            :key="option.value"
            type="button"
            :class="{ active: selectedImportType === option.value }"
            @click="selectedImportType = option.value">
            {{ option.label }}
          </button>
        </div>
        <label class="file-picker">
          <span>Excel 文件</span>
          <input type="file" accept=".xlsx,.xls" @change="onFileChange" />
          <strong>{{ selectedFileName || '未选择文件' }}</strong>
        </label>
        <button class="primary" type="button" @click="parseImport" :disabled="loading || !normalizedCustomerId || !selectedFile">解析导入</button>
        <button class="secondary" type="button" @click="applyLatest" :disabled="loading || !selectedParsedBatchId">应用当前类型最新批次</button>
        <span v-if="!normalizedCustomerId" class="muted import-hint">先选择客户再上传 Excel</span>
      </div>

      <div class="ops-grid">
        <div v-if="activeAction === 'processing' && workbenchSections.processing" class="ops-panel">
          <h3>提交加工工单</h3>
          <div class="ops-form">
            <label>
              <span>成品名称</span>
              <SearchableSelect
                v-model="processingProductValue"
                :options="customerSKUOptions"
                :option-label="productOptionLabel"
                :option-meta="productOptionMeta"
                :option-value="productOptionValue"
                empty-value=""
                placeholder="搜索该客户 SKU"
                empty-text="没有匹配 SKU"
                :disabled="loading || !normalizedCustomerId"
                @select="selectProcessingProduct" />
            </label>
            <label>
              <span>原料名称</span>
              <SearchableSelect
                v-model="processingRawBeanValue"
                :options="rawBeanOptions"
                :option-label="custodyOptionLabel"
                :option-meta="custodyOptionMeta"
                :option-value="custodyOptionValue"
                empty-value=""
                placeholder="搜索托管原料"
                empty-text="没有匹配原料"
                :disabled="loading || !normalizedCustomerId"
                @select="selectProcessingRawBean" />
            </label>
            <label>
              <span>投豆克重</span>
              <input v-model.number="processingForm.input_quantity_g" type="number" min="1" />
            </label>
            <label>
              <span>计划产量</span>
              <input v-model.number="processingForm.planned_output_units" type="number" min="1" />
            </label>
            <label>
              <span>期望日期</span>
              <input v-model="processingForm.expected_date" type="date" />
            </label>
            <label class="wide-field">
              <span>备注</span>
              <input v-model.trim="processingForm.note" placeholder="加工要求" />
            </label>
            <button class="primary" type="button" @click="submitProcessingWorkOrder" :disabled="loading || !normalizedCustomerId">提交工单</button>
          </div>
        </div>

        <div v-if="activeTab === 'dispatch' && activeAction === 'directShip' && workbenchSections.directShip" class="ops-panel direct-ship-panel">
          <h3>{{ submitCopy.formTitle }}</h3>
          <CustomerDirectShipForm
            :customer-id="customerId"
            :customer-label="selectedCustomerLabel"
            @saved="onFulfillmentOrderSaved" />
        </div>

        <div v-if="activeAction === 'inventory' && workbenchSections.inventory" class="ops-panel">
          <h3>库存手动调整</h3>
          <div class="ops-form">
            <label>
              <span>类型</span>
              <select v-model="adjustment.item_type">
                <option value="raw_bean">生豆</option>
                <option value="packaging">包材</option>
                <option value="product">成品</option>
              </select>
            </label>
            <label>
              <span>名称</span>
              <SearchableSelect
                v-model="adjustmentItemValue"
                :options="adjustmentItemOptions"
                :option-label="custodyOptionLabel"
                :option-meta="custodyOptionMeta"
                :option-value="custodyOptionValue"
                empty-value=""
                placeholder="搜索已有库存"
                empty-text="没有匹配库存"
                :disabled="loading || !normalizedCustomerId"
                @select="selectAdjustmentItem" />
            </label>
            <label>
              <span>规格</span>
              <input v-model.trim="adjustment.spec" placeholder="可选" />
            </label>
            <label>
              <span>克重增减</span>
              <input v-model.number="adjustment.quantity_g_delta" type="number" />
            </label>
            <label>
              <span>件数增减</span>
              <input v-model.number="adjustment.quantity_units_delta" type="number" />
            </label>
            <label class="wide-field">
              <span>备注</span>
              <input v-model.trim="adjustment.note" placeholder="手工调整原因" />
            </label>
            <button class="secondary" type="button" @click="adjustCustody" :disabled="loading || !normalizedCustomerId">保存调整</button>
          </div>
        </div>

      </div>


    </section>

    <section v-if="activeAction === 'import' && selectedParsedBatch" class="panel apply-panel">
      <div class="panel-head">
        <div>
          <h3>当前可应用批次</h3>
          <p>{{ selectedParsedBatchLabel }}</p>
        </div>
        <button class="secondary" type="button" @click="loadApplyPreview" :disabled="loading || !selectedParsedBatchId">刷新预览</button>
      </div>
      <div class="batch-line">
        <span>ID {{ selectedParsedBatch.id }}</span>
        <span>{{ importTypeLabel(selectedParsedBatch.import_type) }}</span>
        <span>{{ rowStatusLabel(selectedParsedBatch.status) }}</span>
        <span>有效 {{ selectedParsedBatch.summary?.valid_rows || 0 }}</span>
        <span>错误 {{ selectedParsedBatch.summary?.invalid_rows || 0 }}</span>
      </div>
      <div v-if="applyPreviewEffects.length" class="preview-grid">
        <div v-for="effect in applyPreviewEffects" :key="effect.label" class="preview-item">
          <span>{{ effect.label }}</span>
          <strong>{{ effect.value }}</strong>
        </div>
      </div>
      <div v-if="applyPreview?.warning" class="muted">{{ applyPreview.warning }}</div>
    </section>

    <section ref="resultAnchor" v-if="activeAction === 'import' && summaryCards.length" class="metric-grid">
      <div v-for="card in summaryCards" :key="card.label" class="metric">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </div>
    </section>

    <section v-if="activeAction === 'import' && (invalidRows.length || latestInvalidCount)" class="panel">
      <div class="panel-head">
        <h3>错误行</h3>
      </div>
      <div v-if="latestInvalidCount && !invalidRows.length" class="muted">最近批次有 {{ latestInvalidCount }} 行错误，请在导入批次中查看源文件。</div>
      <div v-if="invalidRowGroups.length" class="error-groups">
        <div v-for="group in invalidRowGroups" :key="group.key">
          <strong>{{ group.count }}</strong>
          <span>{{ group.sheet_name }} / {{ group.row_type }} / {{ group.error }}</span>
        </div>
      </div>
      <table v-if="invalidRows.length">
        <thead>
          <tr><th>表</th><th>行号</th><th>类型</th><th>错误</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in invalidRows" :key="`${row.sheet_name}-${row.row_no}-${row.row_type}`">
            <td>{{ row.sheet_name }}</td>
            <td>{{ row.row_no }}</td>
            <td>{{ row.row_type }}</td>
            <td>{{ row.error }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <section class="grid-2">
      <DataPanel v-if="activeAction === 'import'" title="导入批次（最近 20 批）" :rows="visibleImports" empty="暂无导入批次">
        <table>
          <thead>
            <tr><th>ID</th><th>类型</th><th>文件</th><th>状态</th><th>有效/错误</th><th>操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in visibleImports" :key="row.id">
              <td>{{ row.id }}</td>
              <td>{{ importTypeLabel(row.import_type) }}</td>
              <td>{{ row.source_filename }}</td>
              <td>{{ rowStatusLabel(row.status) }}</td>
              <td>{{ row.summary?.valid_rows || 0 }} / {{ row.summary?.invalid_rows || 0 }}</td>
              <td>
                <button class="link-button" type="button" @click="loadInvalidRows(row)" :disabled="loading || !(row.summary?.invalid_rows > 0)">查看错误行</button>
              </td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel v-if="activeTab === 'orders'" title="加工工单（最近 100 单）" :rows="overview.processing_orders" empty="暂无加工工单">
        <table>
          <thead>
            <tr><th>工单号</th><th>产品</th><th>状态</th><th>投豆</th><th>产量</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.processing_orders || []" :key="row.work_order_no">
              <td>{{ row.work_order_no }}</td>
              <td>{{ row.product_name }}</td>
              <td>{{ businessLabel(row.status) }}</td>
              <td>{{ row.quantity_g || 0 }}</td>
              <td>{{ row.units || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <div v-if="activeTab === 'fees'" class="panel billing-workspace">
        <nav class="billing-tabs" aria-label="费用账务视图">
          <button type="button" :class="{ active: billingView === 'unbilled' }" @click="switchBillingView('unbilled')">未出账</button>
          <button type="button" :class="{ active: billingView === 'bills' }" @click="switchBillingView('bills')">已出账</button>
        </nav>

        <div v-if="billingView === 'unbilled'" class="billing-filters">
          <label><span>费用类型</span><select v-model="billingFilter.fee_type"><option v-for="option in billingFilterOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
          <div class="billing-period-picker" role="group" aria-label="费用日期">
            <button v-for="period in billingDateOptions" :key="period.value" type="button" :class="{ active: billingDateMode === period.value }" @click="setBillingPeriod(period.value)">{{ period.label }}</button>
          </div>
          <template v-if="billingDateMode === 'custom'">
            <label><span>开始日期</span><input v-model="billingFilter.date_from" type="date" /></label>
            <label><span>结束日期</span><input v-model="billingFilter.date_to" type="date" /></label>
          </template>
          <span class="muted">共 {{ billingTotal }} 笔可出账费用</span>
        </div>

        <div v-if="billingView === 'unbilled'" class="table-scroll">
          <table class="billing-table">
            <thead><tr>
              <th><input type="checkbox" aria-label="全选当前页" :checked="billingPageSelection.checked" :indeterminate="billingPageSelection.indeterminate" @change="toggleBillingPage($event.target.checked)" /></th>
              <th>发生日期</th><th>费用类型</th><th>来源单号</th><th>费用名称</th><th>应收</th><th>已收</th><th>未收</th><th>确认状态</th>
            </tr></thead>
            <tbody>
              <tr v-for="row in unbilledCharges" :key="row.id">
                <td><input type="checkbox" :aria-label="`选择${row.source_no || row.description || row.id}`" :checked="selectedBillingSources.has(row.id)" :disabled="row.confirmation_status === 'pending_billing' || Number(row.due_cents) <= 0" @change="toggleBillingRow(row,$event.target.checked)" /></td>
                <td>{{ row.occurred_at }}</td><td>{{ chargeTypeLabel(row.charge_type) }}</td>
                <td><button v-if="row.source_type === 'order'" class="link-button" type="button" @click="openSalesOrderDrawer({ id: row.source_id })">{{ row.source_no }}</button><span v-else>{{ row.source_no || `${businessLabel(row.source_type)} #${row.source_id}` }}</span></td>
                <td>{{ row.description || chargeTypeLabel(row.charge_type) }}</td><td>{{ moneyFromCents(row.receivable_cents) }}</td><td>{{ moneyFromCents(row.received_cents) }}</td><td>{{ moneyFromCents(row.due_cents) }}</td><td>{{ confirmationLabel(row.confirmation_status) }}</td>
              </tr>
              <tr v-if="!billingLoading && !unbilledCharges.length"><td colspan="9" class="muted">没有符合条件的未出账费用</td></tr>
            </tbody>
          </table>
        </div>

        <div v-if="billingView === 'bills'" class="billing-filters">
          <label><span>收款状态</span><select v-model="billStatusFilter" @change="loadBillingWorkspace(1)"><option value="all">全部账单</option><option value="unpaid">尚有未收</option><option value="paid">已收清</option></select></label>
          <span class="muted">共 {{ billTotal }} 张账单</span>
        </div>
        <div v-if="billingView === 'bills'" class="table-scroll">
          <table class="billing-table"><thead><tr><th>账单号</th><th>出账日期</th><th>费用期间</th><th>出账金额</th><th>已收</th><th>未收</th><th>状态</th><th>操作</th></tr></thead>
            <tbody><tr v-for="bill in issuedBills" :key="bill.batch_id" @dblclick="openIssuedBill(bill.batch_id)">
              <td>{{ bill.settlement_no }}</td><td>{{ bill.issued_at || '—' }}</td><td>{{ bill.period_from }} 至 {{ bill.period_to }}</td><td>{{ moneyFromCents(bill.total_amount_cents) }}</td><td>{{ moneyFromCents(bill.current_received_cents) }}</td><td>{{ moneyFromCents(bill.current_due_amount_cents) }}</td><td>{{ bill.current_due_amount_cents > 0 ? '未收清' : '已收清' }}</td><td><button class="link-button" type="button" @click="openIssuedBill(bill.batch_id)">费用明细</button></td>
            </tr><tr v-if="!billingLoading && !issuedBills.length"><td colspan="8" class="muted">暂无已出账账单</td></tr></tbody>
          </table>
        </div>

        <div v-if="billingView === 'unbilled'" class="billing-pagination"><span>第 {{ billingPage }} 页 / {{ Math.max(1, Math.ceil(billingTotal / billingLimit)) }} 页</span><button class="secondary" :disabled="billingLoading || billingPage <= 1" @click="loadBillingWorkspace(billingPage - 1)">上一页</button><button class="secondary" :disabled="billingLoading || billingPage * billingLimit >= billingTotal" @click="loadBillingWorkspace(billingPage + 1)">下一页</button></div>
        <div v-if="billingView === 'bills'" class="billing-pagination"><span>第 {{ billPage }} 页 / {{ Math.max(1, Math.ceil(billTotal / billingLimit)) }} 页</span><button class="secondary" :disabled="billingLoading || billPage <= 1" @click="loadBillingWorkspace(billPage - 1)">上一页</button><button class="secondary" :disabled="billingLoading || billPage * billingLimit >= billTotal" @click="loadBillingWorkspace(billPage + 1)">下一页</button></div>

        <div v-if="billingPreview" class="billing-preview">
          <div class="panel-head"><div><h4>确认出账</h4><p>{{ billingPreview.customer_name }} · {{ billingPreview.period_from }} 至 {{ billingPreview.period_to }} · {{ billingPreview.fee_items }} 笔</p></div><strong>合计 ¥{{ moneyFromCents(billingPreview.total_amount_cents) }}</strong></div>
          <div class="billing-category-totals"><span v-for="item in billingPreviewCategories" :key="item.type">{{ chargeTypeLabel(item.type) }}：¥{{ moneyFromCents(item.amount_cents) }}</span></div>
          <div class="table-scroll"><table class="billing-table"><thead><tr><th>发生日期</th><th>类型</th><th>来源单号</th><th>费用明细</th><th>应收</th><th>出账前已收</th><th>本次出账</th></tr></thead><tbody>
            <tr v-for="line in billingPreview.lines" :key="`${line.charge_type}-${line.source_type}-${line.source_id}`"><td>{{ line.occurred_at }}</td><td>{{ chargeTypeLabel(line.charge_type) }}</td><td>{{ line.source_no || line.source_id }}</td><td>{{ line.description }}</td><td>{{ moneyFromCents(line.receivable_cents) }}</td><td>{{ moneyFromCents(line.issued_received_cents || line.received_cents) }}</td><td>{{ moneyFromCents(line.amount_cents) }}</td></tr>
          </tbody></table></div>
          <div class="head-actions"><button class="secondary" type="button" :disabled="billingExportLoading" @click="exportBillingPreview">{{ billingExportLoading ? '导出中…' : '导出预览 Excel' }}</button><button class="secondary" type="button" :disabled="billingIssueLoading" @click="billingPreview = null">取消</button><button class="primary" type="button" :disabled="billingIssueLoading" @click="confirmBillingIssue">{{ billingIssueLoading ? '出账中…' : '确认出账' }}</button></div>
        </div>
      </div>
    </section>

    <section v-if="normalizedCustomerId && ['orders', 'dispatch'].includes(activeTab)" class="panel fulfillment-orders-panel">
      <div class="panel-head">
        <div>
          <h3>{{ activeTab === 'dispatch' ? '代发工作队列' : '履约客户订单' }}</h3>
          <p>{{ activeTab === 'dispatch' ? '与订单共用同一份数据，点击订单查看收件信息、费用和发货进度。' : '商品下单、留厂补购与代发订单统一查看。' }}</p>
        </div>
        <div class="head-actions">
          <span class="muted">共 {{ fulfillmentOrdersSummary.orders || 0 }} 单</span>
          <button class="secondary" type="button" @click="loadFulfillmentOrders(fulfillmentOrdersPage)" :disabled="fulfillmentOrdersLoading || !normalizedCustomerId">
            刷新
          </button>
        </div>
      </div>

      <div class="order-filters"><div class="queue-tabs"><button v-for="filter in orderFilters" :key="filter.key" :class="{active: orderQueue === filter.key}" @click="setOrderQueue(filter.key)">{{ filter.label }}</button></div><form class="order-search" @submit.prevent="loadFulfillmentOrders(1)"><input v-model="orderSearch" placeholder="订单号、客户或运单" aria-label="搜索履约订单" /><button class="secondary" type="submit">搜索</button></form></div>
      <div class="table-wrap">
        <table class="fulfillment-orders-table">
          <thead><tr><th>订单</th><th>交付信息</th><th class="amount-column">应收金额</th><th>收款</th><th>履约进度</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in fulfillmentOrders" :key="row.id" :class="orderBillingRowClass(row)">
              <td><button class="order-link" type="button" @click="openFulfillmentOrderDetail(row)">{{ row.order_no }}</button><div class="cell-meta">{{ businessLabel(row.fulfillment_mode || (row.delivery_mode === 'customer_stock' ? row.delivery_mode : row.portal_service_code) || row.order_type) }} · {{ row.order_date || '-' }}</div></td>
              <td><div v-if="isCustomerStockOrder(row)" class="cell-meta">留厂库存 · 无需发货</div><div v-else class="stacked-text receiver-cell"><strong>{{ row.receiver_name || '待补收件人' }} <span>{{ row.receiver_phone || '' }}</span></strong><span class="receiver-address" :title="row.receiver_address">{{ row.ship_tracking_no || row.receiver_address || '待补收件信息' }}</span></div></td>
              <td class="amount-column"><strong>¥ {{ row.grand_total || '0.00' }}</strong></td>
              <td><span class="order-status-label" :class="{'status-unpaid': !isOrderFullyPaid(row)}">{{ row.pay_status || '待核对' }}</span></td>
              <td><span class="order-status-label" :class="{'status-complete': isCustomerStockOrder(row) ? row.delivery_status === 'received' : isShipped(row)}">{{ orderProgressLabel(row) }}</span><div class="cell-meta">{{ row.confirmation_status && row.confirmation_status !== 'accepted' ? orderConfirmationLabel(row.confirmation_status) : '' }}</div></td>
              <td><div class="actions-inline compact-order-actions"><button class="link-button" type="button" @click="openFulfillmentOrderDetail(row)">详情</button><button class="link-button" type="button" @click="openSalesOrderDrawer(row)">销售单</button><button v-if="!isCustomerStockOrder(row)" class="link-button" type="button" @click="openDeliveryNoteDrawer(row)" :disabled="!isShipped(row)">出库单</button></div></td>
            </tr>
            <tr v-if="!fulfillmentOrders.length"><td colspan="6" class="muted empty-row">{{ fulfillmentOrdersLoading ? '加载中...' : '暂无履约订单' }}</td></tr>
          </tbody>
        </table>
      </div>

      <PaginationControls
        :page="fulfillmentOrdersPage"
        :page-size="fulfillmentOrdersLimit"
        :total="Number(fulfillmentOrdersSummary.orders || 0)"
        :disabled="fulfillmentOrdersLoading"
        @change="handleFulfillmentOrdersPagination"
      />
    </section>

    <div v-if="settingsDrawerOpen" class="fulfillment-settings-mask" @click.self="settingsDrawerOpen = false" @keydown.esc="settingsDrawerOpen = false"><aside ref="settingsDrawerElement" tabindex="-1" class="fulfillment-settings-drawer" role="dialog" aria-modal="true" aria-label="客户履约设置"><div class="drawer-head"><div><p>{{ overview.customer_name || selectedCustomerLabel }}</p><h3>履约设置</h3></div><button class="secondary" @click="settingsDrawerOpen = false" aria-label="关闭履约设置"><IconX :size="18" /></button></div><p class="settings-intro">多数客户跟随默认，特殊约定只修改例外项。仓库和商品授权按客户单独管理。</p><div class="settings-switch"><button :class="{active: !editingDefaults}" @click="editingDefaults = false">此客户配置</button><button :class="{active: editingDefaults}" @click="editingDefaults = true">共享默认配置</button></div><CustomerFulfillmentSettings v-if="!editingDefaults" :key="normalizedCustomerId" :customer-id="normalizedCustomerId" embedded @settings-changed="handleSettingsChanged" /><FulfillmentDefaultsEditor v-else @saved="handleDefaultsSaved" /><button class="text-link" @click="openCapabilitySettings">前往客户能力与价格授权</button></aside></div>

    <div v-if="orderDetailDrawerOpen" class="order-detail-drawer-mask" @click.self="closeOrderDetailDrawer">
      <aside class="order-detail-drawer" aria-label="履约订单详情">
        <div class="drawer-head">
          <div>
            <h3>{{ activeOrderSummary?.order_no || '订单详情' }}</h3>
            <p>{{ activeOrderSummary?.customer || '-' }} · {{ activeOrderSummary?.order_date || '-' }}</p>
          </div>
          <button class="secondary" type="button" @click="closeOrderDetailDrawer">关闭</button>
        </div>

        <div v-if="orderDetailError" class="error">{{ orderDetailError }}</div>

        <div v-if="activeOrderSummary" class="drawer-body">
 <OrderConfirmationPanel :order-id="Number(activeOrderSummary.id)" :customer-id="Number(customerId)" @updated="refreshConfirmationOrder" />
          <section v-if="!isCustomerStockOrder(activeOrderDetail || activeOrderSummary)" class="drawer-section">
            <h4>收件与快递</h4>
            <div class="detail-grid">
              <span>收件人：{{ activeOrderSummary.receiver_name || '-' }}</span>
              <span>电话：{{ activeOrderSummary.receiver_phone || '-' }}</span>
              <span class="wide-item">地址：{{ activeOrderSummary.receiver_address || '-' }}</span>
              <span>寄件人：{{ activeOrderSummary.sender_label || activeOrderSummary.sender_name || '-' }}</span>
              <span>运单号：{{ activeOrderSummary.ship_tracking_no || '-' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <h4>订单状态</h4>
            <div class="detail-grid">
              <span>收款：{{ activeOrderSummary.pay_status || '-' }}</span>
              <span v-if="!isCustomerStockOrder(activeOrderDetail || activeOrderSummary)">发货：{{ activeOrderSummary.ship_status || '-' }}</span>
              <span>履约：{{ orderProgressLabel({ ...activeOrderSummary, ...activeOrderDetail }) }}</span>
              <span>发票：{{ activeOrderSummary.invoice_status || '-' }}</span>
              <span v-if="activeOrderDetail?.delivery_mode === 'customer_stock' || activeOrderSummary.delivery_mode === 'customer_stock'">交付：购货留厂 · {{ activeOrderDetail?.delivery_warehouse_code || activeOrderSummary.delivery_warehouse_code || '客户仓' }} · {{ customerStockProgressLabel({ ...activeOrderSummary, ...activeOrderDetail }) }} · 无需发货</span>
              <span v-if="activeOrderDetail?.fulfillment_mode || activeOrderSummary.fulfillment_mode">代发场景：{{ (activeOrderDetail?.fulfillment_mode || activeOrderSummary.fulfillment_mode) === 'stock' ? '库存代发（已购货款不重复收取）' : '无库存代发' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <h4>订单费用</h4>
            <div class="detail-grid">
              <span>商品金额：{{ activeOrderDetail?.total_amount || activeOrderSummary.total_amount || '0.00' }}</span>
              <span>{{activeOrderDetail?.freight?freightLabel(activeOrderDetail.freight):`运费：${activeOrderDetail?.shipping_amount || activeOrderSummary.shipping_amount || '0.00'}`}}</span><span v-if="activeOrderDetail?.freight?.service_fee_cents">代发服务费：¥{{(activeOrderDetail.freight.service_fee_cents/100).toFixed(2)}}（另见账单）</span>
              <span>优惠：{{ activeOrderDetail?.discount_amount || activeOrderSummary.discount_amount || '0.00' }}</span>
              <span>应收：{{ activeOrderDetail?.grand_total || activeOrderSummary.grand_total || '0.00' }}</span>
            </div>
            <FulfillmentFeeAdjustment v-if="activeOrderSummary.portal_service_code === 'direct_ship'" :customer-id="Number(customerId)" :order-id="Number(activeOrderSummary.id)" @saved="refreshConfirmationOrder" />
          </section>

          <section class="drawer-section">
            <h4>订单信息</h4>
            <div class="detail-grid">
              <span>类型：{{ activeOrderSummary.order_type || '-' }}</span>
              <span>来源：{{ businessLabel(activeOrderSummary.portal_service_code) }}</span>
              <span>负责人：{{ activeOrderSummary.responsible_name || '-' }}</span>
              <span>录入：{{ activeOrderSummary.created_by_employee || '-' }}</span>
              <span class="wide-item">备注：{{ activeOrderDetail?.notes || activeOrderSummary.notes || '-' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <div class="drawer-actions">
              <OrderDetailsCopyButton :order-id="activeOrderSummary.id" :customer-id="customerId" />
              <DirectShipCancelButton v-if="activeOrderSummary.fulfillment_mode && !activeOrderSummary.is_void" :customer-id="Number(customerId)" :order-id="Number(activeOrderSummary.id)" @cancelled="onDirectShipCancelled" />
              <button class="secondary" type="button" @click="openSalesOrderDrawer(activeOrderSummary)">销售单</button>
              <button v-if="!isCustomerStockOrder(activeOrderDetail || activeOrderSummary)" class="secondary" type="button" @click="openDeliveryNoteDrawer(activeOrderSummary)" :disabled="!isShipped(activeOrderSummary)">出库单</button>
            </div>
          </section>

          <section class="drawer-section">
            <h4>商品明细</h4>
            <div v-if="orderDetailLoading" class="muted">订单明细加载中...</div>
            <div v-else-if="!activeOrderDetail?.items?.length" class="muted">暂无商品明细</div>
            <div v-else class="table-wrap drawer-table-wrap">
              <table class="drawer-table">
                <thead>
                  <tr><th>商品</th><th>规格</th><th>数量</th><th>单价</th><th>小计</th><th>备注</th></tr>
                </thead>
                <tbody>
                  <tr v-for="(item, idx) in activeOrderDetail?.items || []" :key="`${activeOrderSummary.id}-${idx}`">
                    <td>{{ item.product_name || '-' }}<OrderItemWeights :item="item" /></td>
                    <td>{{ item.spec || '-' }}</td>
                    <td>{{ item.qty || '-' }}{{ item.unit || '' }}</td>
                    <td>{{ item.unit_price || '-' }}</td>
                    <td>{{ item.line_total || '-' }}</td>
                    <td>{{ item.note || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </aside>
    </div>

    <div v-if="activeBill" class="sales-order-drawer-mask billing-bill-mask" @click.self="activeBill = null">
      <aside class="sales-order-drawer billing-bill-drawer" aria-label="已出账费用明细">
        <header class="panel-head"><div><h3>{{ activeBill.settlement_no }}</h3><p>{{ activeBill.period_from }} 至 {{ activeBill.period_to }} · {{ activeBill.customer_name }}</p></div><button class="secondary" type="button" @click="activeBill = null">关闭</button></header>
        <div class="billing-bill-summary"><span>出账 ¥{{ moneyFromCents(activeBill.total_amount_cents) }}</span><span>已收 ¥{{ moneyFromCents(activeBill.current_received_cents) }}</span><span>未收 ¥{{ moneyFromCents(activeBill.current_due_amount_cents) }}</span><span>{{ activeBill.issued_at }}</span></div>
        <div class="head-actions"><button class="secondary" type="button" :disabled="billingExportLoading" @click="downloadBillFile('xlsx')">导出费用 Excel</button><button class="secondary" type="button" :disabled="billPdfLoading" @click="downloadStampedBillPDF">{{ billPdfLoading ? 'PDF生成中…' : '下载费用 PDF' }}</button></div>
        <div class="table-scroll"><table class="billing-table"><thead><tr><th>发生日期</th><th>费用类型</th><th>来源单号</th><th>费用名称</th><th>应收</th><th>出账</th><th>出账前已收</th><th>出账后已收</th><th>当前未收</th></tr></thead><tbody>
          <tr v-for="line in activeBill.lines || []" :key="line.id || `${line.charge_type}-${line.source_type}-${line.source_id}`"><td>{{ line.occurred_at }}</td><td>{{ chargeTypeLabel(line.charge_type) }}</td><td><button v-if="line.source_type === 'order'" class="link-button" type="button" @click="openSalesOrderDrawer({ id: line.source_id })">{{ line.source_no }}</button><span v-else>{{ line.source_no || line.source_id }}</span></td><td>{{ line.description }}</td><td>{{ moneyFromCents(line.receivable_cents) }}</td><td>{{ moneyFromCents(line.amount_cents) }}</td><td>{{ moneyFromCents(line.issued_received_cents || 0) }}</td><td>{{ moneyFromCents(line.received_cents) }}</td><td>{{ moneyFromCents(line.due_cents) }}</td></tr>
        </tbody></table></div>
        <section class="bill-pdf-settings"><h4>PDF 公章</h4><div class="billing-filters"><label><span>系统公章</span><select v-model.number="billPDFSettings.seal_asset_id" @change="onBillingSealChanged"><option :value="0">不加盖公章</option><option v-for="seal in billSeals" :key="seal.id" :value="Number(seal.id)">{{ seal.filename || `公章 ${seal.id}` }}</option></select></label><button class="secondary" type="button" :disabled="billPdfLoading" @click="saveBillingPDFSettings">保存公章设置</button></div>
          <div v-if="billPDFSettings.seal_asset_id && billPDFPages.length" class="billing-seal-pages" aria-label="选择盖章页"><label v-for="page in billPDFPages" :key="page.pageNumber"><input type="checkbox" :checked="hasBillingSealPage(page.pageNumber)" @change="toggleBillingSealPage(page,$event.target.checked)" />第 {{ page.pageNumber }} 页</label></div>
          <PDFStampPreview v-if="activeBill" :key="`bill-pdf-${activeBill.batch_id}`" :pdf-url="customerBillExportURL(normalizedCustomerId, activeBill.batch_id, 'pdf')" :placements="billPDFSettings.placements || []" :seal-url="selectedBillSeal?.url || ''" seal-label="公章" :editable="Boolean(selectedBillSeal)" @loaded="onBillPDFLoaded" @placement-change="updateBillPDFPlacement" @placement-commit="saveBillingPDFSettings" />
        </section>
      </aside>
    </div>

    <div v-if="salesOrderDrawerOpen" class="sales-order-drawer-mask" @click.self="closeSalesOrderDrawer">
      <aside class="sales-order-drawer" aria-label="销售单">
        <SalesOrderView :order-id="activeSalesOrderID" embedded @close="closeSalesOrderDrawer" />
      </aside>
    </div>

    <div v-if="deliveryNoteDrawerOpen" class="delivery-note-drawer-mask" @click.self="closeDeliveryNoteDrawer">
      <aside class="delivery-note-drawer" aria-label="出库单">
        <DeliveryNoteView :order-id="activeDeliveryNoteID" embedded @close="closeDeliveryNoteDrawer" />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { IconBook, IconRefresh, IconSettings, IconUsers, IconPlus, IconX } from '@tabler/icons-vue'
import FulfillmentOperationsOverview from '../components/FulfillmentOperationsOverview.vue'
import FulfillmentInventory from '../components/FulfillmentInventory.vue'
import CustomerFulfillmentSettings from '../components/CustomerFulfillmentSettings.vue'
import FulfillmentDefaultsEditor from '../components/FulfillmentDefaultsEditor.vue'
import { useDialogFocus } from '../lib/dialog-focus.js'
import { apiFetch, apiGet, appURL } from '../api/client'
import { fulfillmentModuleTabs, fulfillmentBusinessLabel as businessLabel } from '../lib/fulfillment-operations.js'
import { settlementChargeOptions as buildSettlementChargeOptions, settlementPeriodRange } from '../lib/customer-settlement.js'
import { customerBillingRowClass, isCustomerOrderFullyPaid, pageSelectionState, togglePageSelection } from '../lib/customer-billing.js'
import { createStampedContractPDF, defaultContractStampPlacement } from '../lib/contract-stamp.js'
import PDFStampPreview from '../components/PDFStampPreview.vue'
import {freightLabel} from '../lib/freight'
import OrderDetailsCopyButton from '../components/OrderDetailsCopyButton.vue'
import OrderItemWeights from '../components/OrderItemWeights.vue'
import OrderConfirmationPanel from '../components/OrderConfirmationPanel.vue'
import { orderConfirmationLabel, visibleOrderRefresh } from '../lib/order-confirmation.js'
import { onBeforeUnmount as onConfirmationUnmount, onMounted as onConfirmationMount } from 'vue'

import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import DataPanel from '../components/DataPanel.vue'
import PaginationControls from '../components/PaginationControls.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import DeliveryNoteView from './DeliveryNoteView.vue'
import SalesOrderView from './SalesOrderView.vue'
import CustomerDirectShipForm from '../components/CustomerDirectShipForm.vue'
import FulfillmentFeeAdjustment from '../components/FulfillmentFeeAdjustment.vue'
import DirectShipCancelButton from '../components/DirectShipCancelButton.vue'
import {
  applyCustomerFulfillmentImport,
  adjustCustomerFulfillmentCustodyInventory,
  fetchCustomerFulfillmentCustomers,
  fetchCustomerFulfillmentOrderDetail,
  fetchCustomerFulfillmentOrders,
  fetchCustomerFulfillmentOptions,
  fetchCustomerFulfillmentImportPreview,
  fetchCustomerFulfillmentImportRows,
  fetchCustomerFulfillmentImports,
  fetchCustomerFulfillmentOverview,
  parseCustomerFulfillmentImport,
  submitCustomerFulfillmentProcessingWorkOrder,
} from '../api/customer-fulfillment'
import {
  customerBillExportURL,
  downloadCustomerBill,
  fetchCustomerBillingSummary,
  fetchCustomerBill,
  fetchCustomerBillPDFSettings,
  fetchCustomerIssuedBills,
  fetchCustomerUnbilledCharges,
  issueCustomerBill,
  previewCustomerBill,
  previewCustomerBillExcel,
  saveCustomerBillPDFSettings,
} from '../api/customer-billing.js'
import {
  activeCustomerFulfillmentCustomers,
  buildImportPreviewEffects,
  customerFulfillmentCustomerOptionLabel,
  customerFulfillmentCustomerOptionMeta,
  customerFulfillmentOrderFees,
  customerFulfillmentSubmitCopy,
  customerFulfillmentWorkbenchSections,
  groupInvalidImportRows,
  importSummaryCards,
  importTypeOptions,
  latestParsedBatchForType,
  rowStatusLabel,
  visibleCustomerFulfillmentImports,
} from '../lib/customer-fulfillment'
import { isCustomerStockOrder, customerStockProgressLabel, orderProgressLabel } from '../lib/customer-stock-entry'
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { toNumber } from '../lib/order-entry'
import { normalizePageSize } from '../lib/pagination'
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const moduleTabs = fulfillmentModuleTabs()
const activeTab = ref('overview'), activeAction = ref(''), orderQueue = ref(''), orderSearch = ref('')
const settingsDrawerOpen = ref(false), editingDefaults = ref(false), effectiveSettings = ref(null)
const settingsDrawerElement=ref(null)
useDialogFocus(settingsDrawerOpen,settingsDrawerElement,()=>{settingsDrawerOpen.value=false})
const activeModule = computed(() => moduleTabs.find(tab => tab.key === activeTab.value) || moduleTabs[0])
const actionTitles = { inventory: '调整托管库存', processing: '提交加工工单', directShip: '新建代发', import: '导入数据' }
const moduleCanCreate = computed(() => ({inventory: workbenchSections.value.inventory, orders: workbenchSections.value.orders, dispatch: workbenchSections.value.directShip, fees: workbenchSections.value.settlement})[activeTab.value])
const configurationCaption = computed(() => {
 if (!effectiveSettings.value) return '配置摘要未加载'
 const modes = (effectiveSettings.value.modes || []).filter(row => row.enabled)
 if (!modes.length) return '尚未开启代发场景'
 const count = modes.reduce((n, mode) => n + 5 - (mode.inherited_fields || []).length, 0)
 return count ? `自定义 ${count} 项 · 其余跟随默认` : '使用默认配置'
})
const orderFilters = computed(() => activeTab.value === 'dispatch' ? [{key:'dispatch',label:'全部代发'},{key:'stock_waiting',label:'待备货'},{key:'pending_shipment',label:'待发货'},{key:'missing_recipient',label:'待补信息'}] : [{key:'',label:'全部订单'},{key:'pending_confirmation',label:'待确认'},{key:'pending_payment',label:'待收款'}])
let customerLoadSequence = 0, orderLoadSequence = 0, orderDetailSequence = 0
function changeTab(key) { activeTab.value=key; activeAction.value=''; if (['orders','dispatch'].includes(key)) {orderQueue.value=key === 'dispatch' ? 'dispatch' : ''; loadFulfillmentOrders(1)} if(key==='fees') loadBillingWorkspace(1) }
function openTodo(item) { activeTab.value=item.tab; activeAction.value=''; orderQueue.value=item.queue || ''; if(['orders','dispatch'].includes(item.tab)) loadFulfillmentOrders(1); if(item.tab==='fees') loadBillingWorkspace(1) }
function setOrderQueue(queue) { orderQueue.value=queue; loadFulfillmentOrders(1) }
function handleSettingsChanged(data) { if (Number(data.customerId) !== normalizedCustomerId.value) return; effectiveSettings.value = {...effectiveSettings.value, modes: data.modes}; if(data.saved) loadAll() }
function handleDefaultsSaved() { loadAll() }
function openCapabilitySettings() { window.dispatchEvent(new CustomEvent('kferp:navigate-view',{detail:{key:'customerPortalSettings',params:{customer_id:normalizedCustomerId.value},customerContext:{customerID:normalizedCustomerId.value,customerName:overview.value.customer_name || selectedCustomerLabel.value},returnNavigation:{key:'customerFulfillment',label:'返回履约运营台',params:{customer_id:normalizedCustomerId.value}}}})) }
const customerId = ref(0)
const customerOptions = ref([])
const selectedImportType = ref('processing_workbook')
const selectedFile = ref(null)
const latestSummary = ref(null)
const latestBatch = ref(null)
const applyPreview = ref(null)
const imports = ref([])
const overview = ref({})
const fulfillmentOptions = ref({})
const invalidRows = ref([])
const resultAnchor = ref(null)
const loading = ref(false)
const error = ref('')
const ok = ref('')
const fulfillmentOrders = ref([])
const fulfillmentOrdersSummary = ref({})
const fulfillmentOrdersPage = ref(1)
const fulfillmentOrdersLimit = ref(10)
const fulfillmentOrdersHasPrev = ref(false)
const fulfillmentOrdersHasNext = ref(false)
const fulfillmentOrdersLoading = ref(false)
const orderDetailDrawerOpen = ref(false)
const activeOrderSummary = ref(null)
const activeOrderDetail = ref(null)
const orderDetailLoading = ref(false)
const orderDetailError = ref('')
const salesOrderDrawerOpen = ref(false)
const activeSalesOrderID = ref(0)
const deliveryNoteDrawerOpen = ref(false)
const activeDeliveryNoteID = ref(0)
const billingView = ref('unbilled')
const billingTotals = ref({ total_unpaid_cents: 0, unbilled_cents: 0, issued_cents: 0 })
const unbilledCharges = ref([]), issuedBills = ref([])
const billingPage = ref(1), billingLimit = ref(25), billingTotal = ref(0), billingLoading = ref(false)
const billPage = ref(1), billTotal = ref(0), billStatusFilter = ref('all')
const billingDateMode = ref('all')
const billingDateOptions = [{ value: 'all', label: '全部日期' }, { value: 'previous', label: '上月' }, { value: 'current', label: '当月' }, { value: 'custom', label: '自定义' }]
const billingFilter = reactive({ fee_type: '', date_from: '', date_to: '' })
const selectedBillingSources = ref(new Map())
const billingPreview = ref(null), billingPreviewLoading = ref(false), billingIssueLoading = ref(false), billingExportLoading = ref(false)
const activeBill = ref(null), billPDFSettings = ref({ seal_asset_id: 0, placements: [] }), billSeals = ref([]), billPDFPages = ref([]), billPdfLoading = ref(false)
const billingIdempotencyKey = ref('')
let billingLoadSequence = 0
const processingProductValue = ref('')
const processingRawBeanValue = ref('')
const processingForm = reactive({
  product_id: 0,
  product_name: '',
  raw_bean_item_id: 0,
  raw_bean_name: '',
  input_quantity_g: '',
  planned_output_units: '',
  expected_date: '',
  note: '',
})
const adjustmentItemValue = ref('')
const adjustment = reactive({
  item_type: 'raw_bean',
  item_name: '',
  spec: '',
  quantity_g_delta: 0,
  quantity_units_delta: 0,
  note: '',
})
const normalizedCustomerId = computed(() => Number(customerId.value || 0))
const selectedCustomer = computed(() => customerOptions.value.find((row) => Number(row.id) === normalizedCustomerId.value) || null)
const selectedCustomerLabel = computed(() => selectedCustomer.value ? customerFulfillmentCustomerOptionLabel(selectedCustomer.value) : '')
const selectedFileName = computed(() => selectedFile.value?.name || '')
const summaryCards = computed(() => importSummaryCards(latestSummary.value || latestBatch.value?.summary || {}))
const latestInvalidCount = computed(() => Number((latestSummary.value || latestBatch.value?.summary || {}).invalid_rows || 0))
const invalidRowGroups = computed(() => groupInvalidImportRows(invalidRows.value))
const enabledCapabilities = computed(() => Array.isArray(overview.value?.capabilities) ? overview.value.capabilities : [])
const submitCopy = computed(() => customerFulfillmentSubmitCopy(enabledCapabilities.value))
const workbenchSections = computed(() => customerFulfillmentWorkbenchSections(enabledCapabilities.value))
const visibleImportTypes = computed(() => importTypeOptions(enabledCapabilities.value))
const visibleImports = computed(() => imports.value)
const selectedParsedBatch = computed(() => latestParsedBatchForType(visibleImports.value, latestBatch.value, selectedImportType.value))
const selectedParsedBatchId = computed(() => visibleImportTypes.value.some(o => o.value === selectedParsedBatch.value?.import_type) ? Number(selectedParsedBatch.value?.id || selectedParsedBatch.value?.batch_id || 0) : 0)
const selectedParsedBatchLabel = computed(() => {
  const batch = selectedParsedBatch.value
  if (!batch) return ''
  return `${batch.source_filename || '未命名文件'} / ${importTypeLabel(batch.import_type)}`
})
const customerSKUOptions = computed(() => fulfillmentOptions.value?.customer_skus || [])
const custodyItemOptions = computed(() => fulfillmentOptions.value?.custody_items || [])
const rawBeanOptions = computed(() => custodyItemOptions.value.filter((row) => row.item_type === 'raw_bean'))
const adjustmentItemOptions = computed(() => custodyItemOptions.value.filter((row) => row.item_type === adjustment.item_type))
const settlementChargeOptions = computed(() => buildSettlementChargeOptions(overview.value.fees || []))
const billingPageSelection = computed(() => pageSelectionState(unbilledCharges.value, new Set(selectedBillingSources.value.keys())))
const billingFilterOptions = computed(() => [{ value: '', label: '全部费用类型' }, ...settlementChargeOptions.value])
const billingPreviewCategories = computed(() => {
  const totals = new Map()
  for (const line of billingPreview.value?.lines || []) totals.set(line.charge_type, (totals.get(line.charge_type) || 0) + Number(line.amount_cents || 0))
  return [...totals.entries()].map(([type, amount_cents]) => ({ type, amount_cents }))
})
const selectedBillSeal = computed(() => {
  const seal = billSeals.value.find(row => Number(row.id) === Number(billPDFSettings.value.seal_asset_id)) || null
  return seal ? { ...seal, url: appURL(seal.url || '') } : null
})
const applyPreviewEffects = computed(() => {
  if (Array.isArray(applyPreview.value?.effects) && applyPreview.value.effects.length) return applyPreview.value.effects
  return buildImportPreviewEffects(selectedParsedBatch.value?.summary || {})
})

watch(selectedImportType, async () => {
  invalidRows.value = []
  await loadApplyPreview()
})

watch(visibleImportTypes, (options) => {
  if (options.some((option) => option.value === selectedImportType.value)) return
  selectedImportType.value = options[0]?.value || ''
}, { immediate: true })

watch(selectedParsedBatchId, async () => {
  await loadApplyPreview()
})

watch(() => [billingFilter.fee_type, billingFilter.date_from, billingFilter.date_to], () => {
  selectedBillingSources.value = new Map()
  billingPreview.value = null
  billingIdempotencyKey.value = ''
  billingPage.value = 1
  if (activeTab.value === 'fees' && billingView.value === 'unbilled' && normalizedCustomerId.value) loadBillingWorkspace(1)
})

onMounted(async () => {
  await loadCustomerOptions()
  const params = new URL(window.location.href).searchParams
  customerId.value = Number(props.customerContextId || props.viewParams.customer_id || params.get('customer_id') || 0)
  if (customerId.value) await loadAll()
})

function onFileChange(event) {
  selectedFile.value = event.target.files?.[0] || null
}

async function loadCustomerOptions(query = '') {
  try {
    const data = await fetchCustomerFulfillmentCustomers(query, 200)
    customerOptions.value = activeCustomerFulfillmentCustomers(data)
  } catch (err) {
    if (!customerOptions.value.length) error.value = err.message || '加载客户列表失败'
  }
}

async function selectCustomer(customer) {
  customerId.value = Number(customer?.id || 0)
  fulfillmentOrdersPage.value = 1
  resetCustomerWorkspace()
  closeOrderDetailDrawer()
  notifyWorkspaceCustomerChanged(customerId.value)
  if (customerId.value) await loadAll()
}

async function applyWorkspaceCustomerContext() {
  const nextCustomerID = Number(props.customerContextId || 0)
  if (nextCustomerID <= 0 || nextCustomerID === Number(customerId.value || 0)) return
  customerId.value = nextCustomerID
  fulfillmentOrdersPage.value = 1
  resetCustomerWorkspace()
  closeOrderDetailDrawer()
  await loadAll()
}

function notifyWorkspaceCustomerChanged(nextCustomerID) {
  if (props.workspaceMode !== CUSTOMER_WORKSPACE_MODE || Number(nextCustomerID || 0) <= 0) return
  if (Number(nextCustomerID || 0) === Number(props.customerContextId || 0)) return
  window.dispatchEvent(workspaceCustomerChangeEvent(nextCustomerID))
}

watch(() => props.customerContextId, () => {
  applyWorkspaceCustomerContext()
})

function resetCustomerWorkspace() {
 customerLoadSequence++; orderLoadSequence++; overview.value={}; imports.value=[]; fulfillmentOptions.value={}; fulfillmentOrders.value=[]; fulfillmentOrdersSummary.value={}; effectiveSettings.value=null
 processingProductValue.value='';processingRawBeanValue.value='';Object.assign(processingForm,{product_id:0,product_name:'',raw_bean_item_id:0,raw_bean_name:'',input_quantity_g:'',planned_output_units:'',expected_date:'',note:''});adjustmentItemValue.value='';Object.assign(adjustment,{item_type:'raw_bean',item_name:'',spec:'',quantity_g_delta:0,quantity_units_delta:0,note:''})
 activeTab.value='overview';activeAction.value='';orderQueue.value='';orderSearch.value='';settingsDrawerOpen.value=false;editingDefaults.value=false;latestBatch.value=null;latestSummary.value=null;selectedFile.value=null;invalidRows.value=[]
 billingView.value='unbilled';billingTotals.value={total_unpaid_cents:0,unbilled_cents:0,issued_cents:0};unbilledCharges.value=[];issuedBills.value=[];selectedBillingSources.value=new Map();billingPreview.value=null;activeBill.value=null;billingFilter.fee_type='';billingFilter.date_from='';billingFilter.date_to='';billingDateMode.value='all';billingPage.value=1;billPage.value=1
 billingIdempotencyKey.value='';billPDFSettings.value={seal_asset_id:0,placements:[]};billSeals.value=[];billPDFPages.value=[]
}
async function loadAll() {
  const id=normalizedCustomerId.value, sequence=++customerLoadSequence
  if (!id) return
  loading.value = true; error.value = ''; ok.value = ''
  try {
    const [overviewData, importData, optionsData, settingsData] = await Promise.all([
      fetchCustomerFulfillmentOverview(id), fetchCustomerFulfillmentImports(id), fetchCustomerFulfillmentOptions(id), apiGet(`/api/customer-fulfillment/${id}/settings`).catch(() => null),
    ])
    if (sequence!==customerLoadSequence || id!==normalizedCustomerId.value) return
    overview.value=overviewData || {};fulfillmentOptions.value=optionsData || {};effectiveSettings.value=settingsData
    rememberOverviewCustomer(overviewData);imports.value=importData?.imports || overviewData?.imports || []
    if (['orders','dispatch'].includes(activeTab.value)) await loadFulfillmentOrders(fulfillmentOrdersPage.value)
    if (activeTab.value === 'fees') await loadBillingWorkspace(billingView.value === 'unbilled' ? billingPage.value : billPage.value)
  } catch (err) { if(sequence===customerLoadSequence) error.value=err.message || '加载客户履约账户失败' }
  finally { if(sequence===customerLoadSequence) loading.value=false }
}

async function loadBillingWorkspace(page = 1) {
  const customerID = normalizedCustomerId.value
  if (!customerID) return
  const sequence = ++billingLoadSequence
  billingLoading.value = true
  try {
    const [totals, listing] = await Promise.all([
      fetchCustomerBillingSummary(customerID),
      billingView.value === 'unbilled'
        ? fetchCustomerUnbilledCharges(customerID, { ...billingFilter, page, limit: billingLimit.value })
        : fetchCustomerIssuedBills(customerID, { status: billStatusFilter.value === 'all' ? '' : billStatusFilter.value, page, limit: billingLimit.value }),
    ])
    if (sequence !== billingLoadSequence || customerID !== normalizedCustomerId.value) return
    billingTotals.value = totals || { total_unpaid_cents: 0, unbilled_cents: 0, issued_cents: 0 }
    if (billingView.value === 'unbilled') {
      unbilledCharges.value = listing?.rows || []
      billingTotal.value = Number(listing?.total || 0)
      billingPage.value = Number(listing?.page || page)
    } else {
      issuedBills.value = listing?.rows || []
      billTotal.value = Number(listing?.total || 0)
      billPage.value = Number(listing?.page || page)
    }
  } catch (err) {
    if (sequence === billingLoadSequence) error.value = err.message || '加载费用账单失败'
  } finally {
    if (sequence === billingLoadSequence) billingLoading.value = false
  }
}

function switchBillingView(view) {
  if (!['unbilled', 'bills'].includes(view) || billingView.value === view) return
  billingView.value = view
  selectedBillingSources.value = new Map()
  billingPreview.value = null
  billingIdempotencyKey.value = ''
  loadBillingWorkspace(view === 'unbilled' ? billingPage.value : billPage.value)
}

function setBillingPeriod(mode) {
  billingDateMode.value = mode
  if (mode === 'custom') {
    const range = settlementPeriodRange('current')
    if (!billingFilter.date_from || !billingFilter.date_to) {
      billingFilter.date_from = range.period_from
      billingFilter.date_to = range.period_to
    }
    return
  }
  if (mode === 'all') {
    billingFilter.date_from = ''
    billingFilter.date_to = ''
    return
  }
  const range = settlementPeriodRange(mode)
  billingFilter.date_from = range.period_from
  billingFilter.date_to = range.period_to
}

function toggleBillingPage(checked) {
  const ids = togglePageSelection(unbilledCharges.value, new Set(selectedBillingSources.value.keys()), checked)
  const next = new Map(selectedBillingSources.value)
  for (const row of unbilledCharges.value) {
    if (ids.has(String(row.id))) next.set(String(row.id), row)
    else next.delete(String(row.id))
  }
  selectedBillingSources.value = next
  billingPreview.value = null
  billingIdempotencyKey.value = ''
}

function toggleBillingRow(row, checked) {
  const next = new Map(selectedBillingSources.value)
  if (checked) next.set(String(row.id), row)
  else next.delete(String(row.id))
  selectedBillingSources.value = next
  billingPreview.value = null
  billingIdempotencyKey.value = ''
}

function billingSources() {
  return [...selectedBillingSources.value.values()].map(row => ({
    charge_type: row.charge_type,
    source_type: row.source_type,
    source_id: Number(row.source_id),
  }))
}

async function openBillingPreview() {
  if (!normalizedCustomerId.value || !selectedBillingSources.value.size) return
  billingPreviewLoading.value = true
  error.value = ''
  ok.value = ''
  try {
    billingPreview.value = await previewCustomerBill(normalizedCustomerId.value, { sources: billingSources() })
    billingIdempotencyKey.value = createBillingIdempotencyKey()
  } catch (err) {
    error.value = billingErrorMessage(err.message)
    await loadBillingWorkspace(billingPage.value)
  } finally {
    billingPreviewLoading.value = false
  }
}

async function exportBillingPreview() {
  if (!billingPreview.value) return
  billingExportLoading.value = true
  try {
    const blob = await previewCustomerBillExcel(normalizedCustomerId.value, { sources: billingSources() }, billingPreview.value.preview_version)
    saveBillingBlob(blob, '出账预览.xlsx')
  } catch (err) {
    error.value = err.message || '出账预览 Excel 导出失败'
  } finally {
    billingExportLoading.value = false
  }
}

async function confirmBillingIssue() {
  if (!billingPreview.value || billingIssueLoading.value) return
  billingIssueLoading.value = true
  error.value = ''
  try {
    const bill = await issueCustomerBill(normalizedCustomerId.value, {
      sources: billingSources(),
      preview_version: billingPreview.value.preview_version,
      idempotency_key: billingIdempotencyKey.value || createBillingIdempotencyKey(),
    })
    billingPreview.value = null
    selectedBillingSources.value = new Map()
    billingIdempotencyKey.value = ''
    activeTab.value = 'fees'
    billingView.value = 'bills'
    await loadBillingWorkspace(1)
    ok.value = `已出账 ${bill.settlement_no}，${bill.fee_items || bill.lines?.length || 0} 笔，合计 ¥${moneyFromCents(bill.total_amount_cents || 0)}`
    await openIssuedBill(bill.batch_id)
  } catch (err) {
    error.value = billingErrorMessage(err.message)
    if (String(err.message || '').includes('preview changed')) billingPreview.value = null
  } finally {
    billingIssueLoading.value = false
  }
}

async function openIssuedBill(batchID) {
  const customerID = normalizedCustomerId.value
  if (!customerID || !Number(batchID)) return
  billPdfLoading.value = true
  error.value = ''
  try {
    const [bill, settings, sealData] = await Promise.all([
      fetchCustomerBill(customerID, batchID),
      fetchCustomerBillPDFSettings(customerID, batchID),
      apiGet('/api/settings/sales-order/seals'),
    ])
    activeBill.value = bill
    billPDFSettings.value = { seal_asset_id: Number(settings?.seal_asset_id || 0), placements: settings?.placements || [] }
    billSeals.value = sealData?.rows || []
    billPDFPages.value = []
  } catch (err) {
    error.value = err.message || '读取账单明细失败'
  } finally {
    billPdfLoading.value = false
  }
}

function onBillPDFLoaded(pages) {
  billPDFPages.value = pages || []
  if (!billPDFSettings.value.seal_asset_id || billPDFSettings.value.placements?.length || !billPDFPages.value.length) return
  const page = billPDFPages.value[0]
  const placement = defaultContractStampPlacement({ pageNumber: page.pageNumber, pageWidth: page.pageWidth, pageHeight: page.pageHeight })
  billPDFSettings.value = { ...billPDFSettings.value, placements: [{ ...placement, kind: 'seal', resizable: true, lock_aspect_ratio: true, min_width: 36, min_height: 36 }] }
}

function hasBillingSealPage(pageNumber) {
  return (billPDFSettings.value.placements || []).some(row => Number(row.page_number) === Number(pageNumber))
}

function toggleBillingSealPage(page, checked) {
  const placements = [...(billPDFSettings.value.placements || [])]
  const index = placements.findIndex(row => Number(row.page_number) === Number(page.pageNumber))
  if (checked && index < 0) {
    const placement = defaultContractStampPlacement({ pageNumber: page.pageNumber, pageWidth: page.pageWidth, pageHeight: page.pageHeight })
    placements.push({ ...placement, kind: 'seal', resizable: true, lock_aspect_ratio: true, min_width: 36, min_height: 36 })
  } else if (!checked && index >= 0) placements.splice(index, 1)
  billPDFSettings.value = { ...billPDFSettings.value, placements }
}

function updateBillPDFPlacement(placement) {
  const placements = [...(billPDFSettings.value.placements || [])]
  const index = placements.findIndex(row => Number(row.page_number) === Number(placement.page_number))
  if (index >= 0) placements[index] = { ...placements[index], ...placement }
  else placements.push({ ...placement, kind: 'seal', resizable: true, lock_aspect_ratio: true, min_width: 36, min_height: 36 })
  billPDFSettings.value = { ...billPDFSettings.value, placements }
}

async function onBillingSealChanged() {
  billPDFSettings.value = { ...billPDFSettings.value, placements: [] }
  if (billPDFSettings.value.seal_asset_id && billPDFPages.value.length) onBillPDFLoaded(billPDFPages.value)
  await saveBillingPDFSettings()
}

async function saveBillingPDFSettings() {
  if (!activeBill.value) return
  try {
    billPDFSettings.value = await saveCustomerBillPDFSettings(normalizedCustomerId.value, activeBill.value.batch_id, billPDFSettings.value)
    ok.value = '公章设置已保存'
  } catch (err) {
    error.value = err.message || '公章设置保存失败'
  }
}

async function downloadBillFile(format) {
  if (!activeBill.value) return
  billingExportLoading.value = true
  try {
    const blob = await downloadCustomerBill(normalizedCustomerId.value, activeBill.value.batch_id, format)
    saveBillingBlob(blob, `${activeBill.value.settlement_no}.${format}`)
  } catch (err) {
    error.value = err.message || '账单导出失败'
  } finally {
    billingExportLoading.value = false
  }
}

async function downloadStampedBillPDF() {
  if (!activeBill.value) return
  billPdfLoading.value = true
  try {
    const pdfBlob = await downloadCustomerBill(normalizedCustomerId.value, activeBill.value.batch_id, 'pdf')
    let output = pdfBlob
    if (selectedBillSeal.value && billPDFSettings.value.placements?.length) {
      const [pdfBytes, sealResponse] = await Promise.all([pdfBlob.arrayBuffer(), apiFetch(selectedBillSeal.value.url)])
      if (!sealResponse.ok) throw new Error('公章图片下载失败')
      const sealBytes = await sealResponse.arrayBuffer()
      const stamped = await createStampedContractPDF({ pdfBytes, sealBytes, sealContentType: selectedBillSeal.value.content_type, placements: billPDFSettings.value.placements })
      output = new Blob([stamped], { type: 'application/pdf' })
    }
    saveBillingBlob(output, `${activeBill.value.settlement_no}.pdf`)
  } catch (err) {
    error.value = err.message || '费用 PDF 导出失败'
  } finally {
    billPdfLoading.value = false
  }
}

function saveBillingBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

function createBillingIdempotencyKey() {
  return globalThis.crypto?.randomUUID?.() || `bill-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function billingErrorMessage(message) {
  const text = String(message || '')
  if (text.includes('preview changed')) return '费用发生变化，请刷新并重新核对出账明细'
  if (text.includes('changed or unavailable')) return '所选费用已变化或不可出账，请刷新后重新选择'
  if (text.includes('at least one billing source')) return '请选择至少一笔可出账费用'
  if (text.includes('permission') || text.includes('capability')) return '当前客户未开通出账权限'
  return text || '出账失败，请刷新后重试'
}

async function parseImport() {
  if (!normalizedCustomerId.value || !selectedFile.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await parseCustomerFulfillmentImport(normalizedCustomerId.value, selectedImportType.value, selectedFile.value)
    latestBatch.value = data.batch || data
    latestSummary.value = data.summary || data.batch?.summary || {}
    invalidRows.value = data.invalid_rows || []
    ok.value = `已解析批次 ${data.batch_id || latestBatch.value?.id || ''}`
    await loadAll()
    await loadApplyPreview()
    await nextTick()
    resultAnchor.value?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
  } catch (err) {
    error.value = err.message || '解析失败'
  } finally {
    loading.value = false
  }
}

async function applyLatest() {
  const batch = selectedParsedBatch.value
  const batchId = selectedParsedBatchId.value
  if (!batch || !batchId) return
  const confirmed = window.confirm(`确认应用 ${importTypeLabel(batch.import_type)} 批次 ${batchId}？\n文件：${batch.source_filename || '-'}\n有效行：${batch.summary?.valid_rows || 0}\n错误行：${batch.summary?.invalid_rows || 0}`)
  if (!confirmed) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await applyCustomerFulfillmentImport(batchId)
    ok.value = `已应用 ${result.applied_rows || 0} 行`
    await loadAll()
  } catch (err) {
    error.value = err.message || '应用失败'
  } finally {
    loading.value = false
  }
}

async function submitProcessingWorkOrder() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await submitCustomerFulfillmentProcessingWorkOrder(normalizedCustomerId.value, {
      product_id: Number(processingForm.product_id || 0),
      product_name: processingForm.product_name,
      raw_bean_item_id: Number(processingForm.raw_bean_item_id || 0),
      raw_bean_name: processingForm.raw_bean_name,
      input_quantity_g: Number(processingForm.input_quantity_g || 0),
      planned_output_units: Number(processingForm.planned_output_units || 0),
      expected_date: processingForm.expected_date,
      note: processingForm.note,
    })
    ok.value = `已提交工单 ${row.work_order_no || ''}`
    processingProductValue.value = ''
    processingRawBeanValue.value = ''
    processingForm.product_id = 0
    processingForm.product_name = ''
    processingForm.raw_bean_item_id = 0
    processingForm.raw_bean_name = ''
    processingForm.input_quantity_g = ''
    processingForm.planned_output_units = ''
    processingForm.expected_date = ''
    processingForm.note = ''
    await loadAll()
  } catch (err) {
    error.value = err.message || '提交工单失败'
  } finally {
    loading.value = false
  }
}

async function adjustCustody() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await adjustCustomerFulfillmentCustodyInventory(normalizedCustomerId.value, {
      item_type: adjustment.item_type,
      item_name: adjustment.item_name,
      spec: adjustment.spec,
      quantity_g_delta: Number(adjustment.quantity_g_delta || 0),
      quantity_units_delta: Number(adjustment.quantity_units_delta || 0),
      note: adjustment.note,
    })
    ok.value = `已调整库存：${row.item_name || adjustment.item_name}`
    adjustment.quantity_g_delta = 0
    adjustment.quantity_units_delta = 0
    adjustment.note = ''
    await loadAll()
  } catch (err) {
    error.value = err.message || '库存调整失败'
  } finally {
    loading.value = false
  }
}

async function loadInvalidRows(batch) {
  const batchId = Number(batch?.id || 0)
  if (!batchId) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await fetchCustomerFulfillmentImportRows(batchId, { status: 'invalid', limit: 200 })
    invalidRows.value = data?.rows || []
    latestBatch.value = batch
    latestSummary.value = batch?.summary || {}
    ok.value = `已载入批次 ${batchId} 的错误行`
    await nextTick()
    resultAnchor.value?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
  } catch (err) {
    error.value = err.message || '加载错误行失败'
  } finally {
    loading.value = false
  }
}

async function loadApplyPreview() {
  const batchId = selectedParsedBatchId.value
  if (!batchId) {
    applyPreview.value = null
    return
  }
  try {
    applyPreview.value = await fetchCustomerFulfillmentImportPreview(batchId)
  } catch {
    applyPreview.value = {
      batch: selectedParsedBatch.value,
      effects: buildImportPreviewEffects(selectedParsedBatch.value?.summary || {}),
      warning: '应用预览暂时不可用，请按批次摘要核对后再应用。',
    }
  }
}

async function loadFulfillmentOrders(page=1) {
 const id=normalizedCustomerId.value, sequence=++orderLoadSequence
 if(!id)return
 fulfillmentOrdersLoading.value=true;error.value=''
 try { const data=await loadFulfillmentOrdersData(id,page);if(sequence===orderLoadSequence && id===normalizedCustomerId.value)assignFulfillmentOrders(data) }
 catch(err){if(sequence===orderLoadSequence)error.value=err.message || '加载履约客户订单失败'}
 finally{if(sequence===orderLoadSequence)fulfillmentOrdersLoading.value=false}
}

function loadFulfillmentOrdersData(customerId, page = 1) {
  return fetchCustomerFulfillmentOrders(customerId, { page, limit: fulfillmentOrdersLimit.value, q: orderSearch.value, queue: orderQueue.value })
}

function handleFulfillmentOrdersPagination({ page, pageSize }) {
  fulfillmentOrdersLimit.value = normalizePageSize(pageSize)
  loadFulfillmentOrders(page)
}

function assignFulfillmentOrders(data = {}) {
  fulfillmentOrders.value = Array.isArray(data?.rows) ? data.rows : []
  fulfillmentOrdersSummary.value = data?.summary || {}
  fulfillmentOrdersPage.value = Number(data?.page || fulfillmentOrdersPage.value || 1)
  fulfillmentOrdersLimit.value = normalizePageSize(data?.limit || fulfillmentOrdersLimit.value)
  fulfillmentOrdersHasPrev.value = Boolean(data?.has_prev)
  fulfillmentOrdersHasNext.value = Boolean(data?.has_next)
  const currentID = Number(activeOrderSummary.value?.id || 0)
  if (currentID > 0) {
    const refreshed = fulfillmentOrders.value.find((row) => Number(row.id) === currentID)
    if (refreshed) activeOrderSummary.value = { ...refreshed }
  }
}

async function openFulfillmentOrderDetail(row) {
  const orderId = Number(row?.id || 0), customerID=normalizedCustomerId.value, sequence=++orderDetailSequence
  if (!orderId) return
  activeOrderSummary.value = { ...row }
  activeOrderDetail.value = null
  orderDetailError.value = ''
  orderDetailDrawerOpen.value = true
  orderDetailLoading.value = true
  try {
    const data = await fetchCustomerFulfillmentOrderDetail(orderId)
    if(sequence!==orderDetailSequence || customerID!==normalizedCustomerId.value)return
    activeOrderDetail.value = data?.edit_data || null
  } catch (err) {
    if(sequence===orderDetailSequence)orderDetailError.value = err.message || '加载订单明细失败'
  } finally {
    if(sequence===orderDetailSequence)orderDetailLoading.value = false
  }
}

function closeOrderDetailDrawer() {
  orderDetailSequence++
  orderDetailDrawerOpen.value = false
  activeOrderSummary.value = null
  activeOrderDetail.value = null
  orderDetailError.value = ''
  orderDetailLoading.value = false
}

function openSalesOrderDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeSalesOrderID.value = id
  salesOrderDrawerOpen.value = true
}

function closeSalesOrderDrawer() {
  salesOrderDrawerOpen.value = false
  activeSalesOrderID.value = 0
}

function openDeliveryNoteDrawer(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  activeDeliveryNoteID.value = id
  deliveryNoteDrawerOpen.value = true
}

function closeDeliveryNoteDrawer() {
  deliveryNoteDrawerOpen.value = false
  activeDeliveryNoteID.value = 0
}

function chargeTypeLabel(value) {
  return settlementChargeOptions.value.find(row => row.value === value)?.label || businessLabel(value)
}

function confirmationLabel(value) {
  if (value === 'confirmed') return '已确认'
  if (value === 'pending_confirmation') return '待确认'
  if (value === 'pending_billing') return '待出账'
  return businessLabel(value)
}

function selectProcessingProduct(option) {
  processingForm.product_id = Number(option?.product_id || 0)
  processingForm.product_name = String(option?.product_name || '').trim()
}

function selectProcessingRawBean(option) {
  processingForm.raw_bean_item_id = Number(option?.item_id || 0)
  processingForm.raw_bean_name = String(option?.item_name || '').trim()
}

function selectAdjustmentItem(option) {
  adjustment.item_type = option?.item_type || adjustment.item_type
  adjustment.item_name = String(option?.item_name || '').trim()
  adjustment.spec = String(option?.spec || '').trim()
}

function productOptionLabel(option) {
  return option?.product_name || ''
}

function productOptionMeta(option) {
  return [
    option?.sku_code,
    option?.spec,
    option?.roast_degree,
    option?.warehouse,
    option?.quantity_units ? `${option.quantity_units}件` : '',
  ].filter(Boolean).join(' / ')
}

function productOptionValue(option) {
  if (Number(option?.product_id || 0) > 0) {
    return `product:${option.product_id}:${option?.spec || ''}:${option?.warehouse || ''}`
  }
  return `product:${option?.product_name || ''}:${option?.spec || ''}`
}

function custodyOptionLabel(option) {
  return option?.item_name || ''
}

function custodyOptionMeta(option) {
  return [
    custodyTypeLabel(option?.item_type),
    option?.spec,
    option?.quantity_g ? `${option.quantity_g}g` : '',
    option?.quantity_units ? `${option.quantity_units}件` : '',
  ].filter(Boolean).join(' / ')
}

function custodyOptionValue(option) {
  return `custody:${option?.item_type || ''}:${option?.item_id || option?.item_name || ''}:${option?.spec || ''}`
}

function recipientOptionLabel(option) {
  const label = [option?.receiver_name, option?.receiver_phone].filter(Boolean).join(' ')
  return label || option?.receiver_address || ''
}

function recipientOptionMeta(option) {
  return [option?.receiver_address, option?.last_order_no, option?.last_used_at].filter(Boolean).join(' / ')
}

function recipientOptionValue(option) {
  return [option?.receiver_phone, option?.receiver_address, option?.last_order_no].filter(Boolean).join('|')
}

const recipientOptionsForOrderEntry = computed(() => {
  return (recipientOptions.value || []).map(r => ({
    key: `${r.receiver_phone}|${r.receiver_address}`,
    label: r.receiver_name || '',
    phone: r.receiver_phone || '',
    address: r.receiver_address || '',
  }))
})

function onFulfillmentOrderSaved(data) {
  ok.value = `订单已创建：${data.order_no || data.order_id || ''}`
  loadAll()
}

async function onDirectShipCancelled() { closeOrderDetailDrawer(); ok.value = '代发已取消，预订与未结算费用已释放'; await loadAll() }

function importTypeLabel(value) {
  return importTypeOptions(enabledCapabilities.value).find((option) => option.value === value)?.label || value
}

function orderFeeLines(row) {
  return customerFulfillmentOrderFees(row)
}

function isShipped(row) {
  return String(row?.ship_status || '').includes('已发货')
}

function isOrderFullyPaid(row) { return isCustomerOrderFullyPaid(row?.pay_status) }
function orderBillingRowClass(row) { return customerBillingRowClass(row, isShipped(row)) }

function custodyTypeLabel(value) {
  return { raw_bean: '生豆', packaging: '包材', product: '产品' }[value] || value
}

function moneyFromCents(value) {
  return (Number(value || 0) / 100).toFixed(2)
}

function money(value) {
  return Number(value || 0).toFixed(2)
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function rememberOverviewCustomer(data) {
  const id = Number(data?.customer_id || 0)
  if (!id || !data?.customer_name) return
  if (customerOptions.value.some((row) => Number(row.id) === id)) return
  customerOptions.value = [
    { id, name: data.customer_name, active: true },
    ...customerOptions.value,
  ]
}

function openManual() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'customerFulfillmentManual' },
  }))
}

async function refreshConfirmationOrder() { const id=activeOrderSummary.value?.id; await loadFulfillmentOrders(fulfillmentOrdersPage.value); if(id){ const row=fulfillmentOrders.value.find(row=>Number(row.id)===Number(id)); if(row) await openFulfillmentOrderDetail(row) } }
let stopConfirmationRefresh
onConfirmationMount(() => { stopConfirmationRefresh=visibleOrderRefresh(async () => {
 if(!normalizedCustomerId.value || loading.value)return
 if(['orders','dispatch'].includes(activeTab.value))return loadFulfillmentOrders(fulfillmentOrdersPage.value)
 const id=normalizedCustomerId.value,sequence=customerLoadSequence
 try {const data=await fetchCustomerFulfillmentOverview(id);if(id===normalizedCustomerId.value && sequence===customerLoadSequence)overview.value=data || {}} catch { /* keep the last successful view; manual refresh exposes an error */ }
}) })
onConfirmationUnmount(() => stopConfirmationRefresh?.())
</script>

<style scoped>
.customer-fulfillment {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.panel {
  background: #fff;
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 14px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.panel-head h2,
.panel-head h3 {
  margin: 0;
}

.panel-head p {
  margin: 4px 0 0;
  color: #64748b;
}

.head-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.toolbar,
.import-row,
.settlement-row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 10px;
  margin-top: 10px;
}

.settlement-period-picker { display:flex; gap:6px; }
.settlement-period-picker button.active { background:#e8f6ed; border-color:#b9dfc4; color:#278456; }
.settlement-charge-types { flex:1 1 100%; display:flex; flex-wrap:wrap; gap:8px 16px; border:1px solid #e1e7ee; border-radius:8px; margin:0; padding:10px 12px; }
.settlement-charge-types legend { color:#718097; font-size:12px; padding:0 5px; }
.settlement-charge-types label { display:flex; align-items:center; gap:6px; font-size:12px; color:#526174; }
.settlement-charge-types input { width:auto; margin:0; }
.settlement-order-preview { flex:1 1 100%; padding:10px 12px; border:1px solid #e1e7ee; border-radius:8px; overflow-x:auto; }
.settlement-order-preview p { margin:0 0 8px; color:#64748b; font-size:12px; }
.fee-confirmed { color:#16834a; font-weight:600; }
.fee-unconfirmed { color:#a66b00; }

.customer-picker-field {
  flex: 1 1 340px;
  max-width: 520px;
}

.file-picker {
  min-width: 260px;
}

.file-picker input {
  max-width: 260px;
}

.file-picker strong {
  color: #0f172a;
  font-weight: 600;
  max-width: 260px;
  overflow-wrap: anywhere;
}

.import-hint {
  align-self: center;
}

.account-hint {
  margin: 10px 0 0;
}

label {
  display: grid;
  gap: 4px;
  color: #475569;
  font-size: 13px;
}

input,
textarea {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 8px;
  font: inherit;
}

textarea {
  resize: vertical;
}

select {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 8px;
  background: #fff;
  font: inherit;
}

button {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 12px;
  background: #fff;
  cursor: pointer;
}

button:disabled {
  cursor: not-allowed;
  opacity: .6;
}

.primary {
  border-color: #0f766e;
  background: #0f766e;
  color: #fff;
}

.secondary {
  background: #f8fafc;
}

.direct-ship-form {
  display: grid;
  gap: 12px;
}

.direct-ship-recipient {
  display: grid;
  grid-template-columns: minmax(220px, 1.4fr) auto repeat(3, minmax(130px, 1fr));
  gap: 10px;
  align-items: end;
}

.recipient-paste {
  grid-column: 1 / span 1;
}

.recipient-address {
  grid-column: 1 / -1;
}

.parse-button {
  min-width: 120px;
}

.direct-ship-items {
  display: grid;
  gap: 8px;
  position: relative;
  z-index: 5;
  overflow: visible;
}

.direct-ship-items-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.order-lines-table {
  min-width: 1080px;
  table-layout: fixed;
}

.order-lines-table th:nth-child(1) { width: 260px; }
.order-lines-table th:nth-child(2) { width: 100px; }
.order-lines-table th:nth-child(3) { width: 90px; }
.order-lines-table th:nth-child(4) { width: 130px; }
.order-lines-table th:nth-child(5) { width: 170px; }
.order-lines-table th:nth-child(6) { width: 100px; }
.order-lines-table th:nth-child(7) { width: 240px; }
.order-lines-table th:nth-child(8) { width: 80px; }

.order-lines-table td {
  vertical-align: top;
}

.order-lines-table td :deep(.searchable-select) {
  width: 100%;
}

.order-lines-table td input {
  width: 100%;
}

.price-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.price-cell small {
  color: #64748b;
  font-size: 12px;
  white-space: nowrap;
}

.discount-cell {
  display: grid;
  gap: 6px;
}

.discount-cell select,
.discount-cell input {
  width: 100%;
}

.line-total {
  display: grid;
  gap: 2px;
  color: #475569;
  font-size: 12px;
}

.line-total strong {
  color: #0f172a;
  font-size: 14px;
}

.subtotal-cell {
  font-weight: 700;
  color: #0f172a;
}

.tier-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tier-chip {
  border: 1px solid #cbd5e1;
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 12px;
  color: #475569;
  background: #fff;
}

.tier-chip.active {
  border-color: #0f766e;
  color: #0f766e;
  font-weight: 600;
}

.direct-ship-footer {
  display: grid;
  grid-template-columns: minmax(140px, 220px) minmax(120px, 180px) minmax(280px, 1fr);
  gap: 12px;
  align-items: end;
}

.grand-total {
  align-self: center;
}

.note-field {
  min-width: 0;
}

.danger {
  border-color: #fecaca;
  color: #b91c1c;
}

.link-button {
  min-height: 28px;
  border: 0;
  padding: 0;
  color: #0f766e;
  background: transparent;
  font-weight: 600;
}

.segmented {
  display: inline-flex;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  overflow: hidden;
}

.segmented button {
  border: 0;
  border-radius: 0;
}

.segmented button + button {
  border-left: 1px solid #cbd5e1;
}

.segmented .active {
  background: #0f766e;
  color: #fff;
}

.ops-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 12px;
  margin-top: 12px;
}

.ops-panel {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 12px;
  background: #f8fafc;
}
.direct-ship-panel { grid-column:1/-1;order:-1; }

.ops-panel h3 {
  margin: 0 0 10px;
  font-size: 16px;
}

.ops-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 8px;
  align-items: end;
}

.external-user-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  align-items: end;
  margin-bottom: 12px;
}

.external-users-table {
  min-width: 860px;
}

.password-row {
  display: grid;
  grid-template-columns: minmax(120px, 1fr) 72px;
  gap: 6px;
}

.wide-field {
  grid-column: span 2;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.metric {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 12px;
  background: #f8fafc;
}

.metric span {
  display: block;
  color: #64748b;
  font-size: 13px;
}

.metric strong {
  display: block;
  margin-top: 6px;
  font-size: 22px;
}

.apply-panel {
  display: grid;
  gap: 10px;
}

.batch-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.batch-line span {
  border: 1px solid #d8dee4;
  border-radius: 999px;
  padding: 4px 8px;
  background: #f8fafc;
  color: #334155;
  font-size: 12px;
}

.preview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 8px;
}

.preview-item {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 10px;
  background: #fff;
}

.preview-item span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.preview-item strong {
  display: block;
  margin-top: 4px;
  color: #0f172a;
}

.error-groups {
  display: grid;
  gap: 8px;
  margin-bottom: 10px;
}

.error-groups div {
  display: flex;
  align-items: start;
  gap: 8px;
  border: 1px solid #fecaca;
  border-radius: 8px;
  padding: 8px;
  background: #fef2f2;
  color: #991b1b;
}

.error-groups strong {
  min-width: 28px;
}

.grid-2 {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
  align-items: start;
  gap: 14px;
}

.grid-2 :deep(.data-panel) {
  min-width: 0;
  overflow-x: auto;
}

.table-wrap {
  overflow: visible;
}

table {
  width: 100%;
  min-width: 560px;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 13px;
}

th,
td {
  border-bottom: 1px solid #e2e8f0;
  height: 40px;
  padding: 8px 10px;
  text-align: left;
  vertical-align: top;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

th {
  color: #475569;
  background: #f8fafc;
  font-weight: 600;
}

td {
  color: #1f2937;
}

.muted {
  color: #64748b;
}

.empty {
  padding: 12px 0;
}

.empty-row {
  text-align: center;
}

.fulfillment-orders-table {
  min-width: 1120px;
}

.order-link {
  border: 0;
  padding: 0;
  min-height: 0;
  background: transparent;
  color: #0f766e;
  font-weight: 700;
}

.cell-meta {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.stacked-text {
  display: grid;
  gap: 4px;
  white-space: normal;
}

.stacked-text strong {
  color: #0f172a;
  font-size: 13px;
}

.stacked-text span {
  color: #64748b;
  font-size: 12px;
  word-break: break-all;
}

.receiver-cell span {
  word-break: break-word;
}

.fee-stack {
  display: grid;
  gap: 4px;
}

.fee-line {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  color: #475569;
  font-size: 12px;
}

.fee-line strong {
  color: #0f172a;
}

.fee-line.emphasized {
  font-weight: 700;
}

.status-stack {
  display: grid;
  gap: 4px;
  white-space: normal;
  color: #475569;
  font-size: 12px;
}

.actions-inline {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.order-detail-drawer-mask,
.sales-order-drawer-mask,
.delivery-note-drawer-mask {
  position: fixed;
  inset: 0;
  z-index: 40;
  display: flex;
  justify-content: flex-end;
  background: rgba(15, 23, 42, 0.28);
}

.order-detail-drawer,
.sales-order-drawer,
.delivery-note-drawer {
  width: min(1120px, 100vw);
  height: 100%;
  overflow-y: auto;
  background: #fff;
  box-shadow: -18px 0 44px rgba(15, 23, 42, 0.18);
}

.order-detail-drawer {
  width: min(840px, 100vw);
  padding: 18px;
}

.drawer-head {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.drawer-head h3 {
  margin: 0;
}

.drawer-head p {
  margin: 4px 0 0;
  color: #64748b;
}

.drawer-body {
  display: grid;
  gap: 14px;
}

.drawer-section {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 12px;
  background: #f8fafc;
}

.drawer-section h4 {
  margin: 0 0 10px;
  font-size: 15px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 12px;
  color: #334155;
  font-size: 13px;
}

.wide-item {
  grid-column: 1 / -1;
}

.drawer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.drawer-table-wrap {
  margin-top: 8px;
}

.drawer-table {
  min-width: 640px;
}

.error,
.ok {
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: 6px;
}

.error {
  background: #fef2f2;
  color: #b91c1c;
}

.ok {
  background: #ecfdf5;
  color: #047857;
}

@media (max-width: 520px) {
  .grid-2 {
    grid-template-columns: minmax(0, 1fr);
  }

  .wide-field {
    grid-column: span 1;
  }

  .detail-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .direct-ship-recipient,
  .direct-ship-footer {
    grid-template-columns: 1fr;
  }

  .recipient-paste,
  .recipient-address {
    grid-column: auto;
  }

  .direct-ship-items-head {
    flex-direction: column;
    align-items: stretch;
  }
}
/* Product Creator workspace surface: quiet hierarchy, stable modules, tables first. */
.customer-fulfillment{padding:24px;background:#f7f9fb;color:#142033;gap:0;min-height:calc(100vh - 120px);box-sizing:border-box}.workspace-header{padding:4px 0 22px}.workspace-heading{display:flex;justify-content:space-between;gap:20px;align-items:center}.eyebrow{font-size:12px;color:#718097;margin:0 0 10px}.workspace-heading h2{font-size:27px;letter-spacing:-.7px;line-height:1.25;margin:0;color:#142033}.workspace-subtitle{font-size:14px;color:#718097;margin:10px 0 0}.head-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.icon-button{display:inline-flex;align-items:center;justify-content:center;gap:7px;font-size:12px}.customer-context{display:flex;align-items:end;gap:20px;margin-top:24px;flex-wrap:wrap}.customer-picker-field{width:300px;min-width:240px}.customer-picker-field>span{font-size:11px;color:#718097;margin-bottom:3px}.customer-service-tags{display:flex;align-items:center;gap:8px;flex-wrap:wrap;min-height:36px}.service-tag{border-radius:5px;padding:5px 8px;background:#e8f6ed;color:#278456;font-size:11px}.configuration-caption{font-size:11px;color:#718097;margin-left:5px}.module-tabs{display:flex;gap:26px;border-bottom:1px solid #e1e7ee;margin:0 0 4px}.module-tabs button{border:0;border-bottom:2px solid transparent;background:none;border-radius:0;color:#718097;padding:12px 4px;font-size:14px;min-width:50px}.module-tabs button.active{border-bottom-color:#278456;color:#278456;font-weight:650}.module-toolbar{display:flex;justify-content:space-between;align-items:center;gap:16px;margin:24px 0 18px}.module-toolbar h3{font-size:20px;margin:0 0 7px}.module-toolbar p{font-size:12px;color:#718097;margin:0}.workspace-empty{display:flex;align-items:center;flex-direction:column;padding:100px 20px;color:#718097;text-align:center}.workspace-empty h3{font-size:18px;color:#142033;font-weight:600}.workspace-empty p{font-size:13px}.workspace-message{margin-top:16px}.readonly-notice{font-size:12px;color:#718097;background:#eef1f5;border-radius:7px;padding:12px;margin:0 0 16px}.panel{border-color:#e1e7ee;padding:20px;border-radius:10px}.primary{background:#278456;border-color:#278456}.secondary{color:#526174;border-color:#e1e7ee}.action-panel{margin:18px 0}.ops-grid{grid-template-columns:1fr}.ops-panel{background:#fff;border:0;padding:0}.ops-panel>h3{display:none}.grid-2{margin:18px 0 0;gap:16px}.grid-2:empty{display:none}.fulfillment-orders-panel{margin-top:18px;order:0}.fulfillment-orders-table th{background:#f7f9fb;font-weight:500;color:#718097}.fulfillment-orders-table td{border-color:#edf0f4;padding-top:16px;padding-bottom:16px}.order-link,.link-button,.text-link{color:#278456}.order-filters{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:18px 0;flex-wrap:wrap}.queue-tabs{display:flex;gap:4px;flex-wrap:wrap}.queue-tabs button{background:transparent;border:0;color:#718097;font-size:12px}.queue-tabs button.active{background:#e8f6ed;color:#278456}.order-search{display:flex;gap:6px}.order-search input{min-width:160px;font-size:12px}.fee-summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}.fee-summary-grid>div{background:#fff;border:1px solid #e1e7ee;border-radius:9px;padding:18px}.fee-summary-grid span{font-size:12px;color:#718097}.fee-summary-grid strong{display:block;font-size:23px;margin:14px 0;font-weight:600}.fee-summary-grid small{font-size:11px;color:#718097}td>small{display:block;color:#718097;font-size:11px;margin-top:5px}.fulfillment-settings-mask{position:fixed;inset:0;z-index:1000;background:#14203340;display:flex;justify-content:flex-end}.fulfillment-settings-drawer{width:min(660px,96vw);height:100%;background:white;padding:28px;box-sizing:border-box;overflow-y:auto;box-shadow:-8px 0 40px #14203312}.fulfillment-settings-drawer h3{margin:4px 0;font-size:23px}.fulfillment-settings-drawer .drawer-head{padding:0 0 18px;border-bottom:1px solid #e1e7ee}.fulfillment-settings-drawer .drawer-head p{margin:0;font-size:12px;color:#718097}.settings-intro{font-size:12px;color:#718097;line-height:1.8;margin:20px 0}.settings-switch{display:flex;background:#f7f9fb;border-radius:7px;padding:4px;gap:4px;margin-bottom:20px}.settings-switch button{flex:1;border:0;background:none;color:#718097;font-size:12px}.settings-switch .active{background:white;color:#278456;box-shadow:0 1px 3px #14203312}.text-link{background:none;border:0;margin-top:20px;padding:0;font-size:12px}button:focus-visible,input:focus-visible{outline:2px solid #278456;outline-offset:3px}@media(max-width:1000px){.workspace-heading,.module-toolbar{align-items:flex-start;flex-direction:column}.fee-summary-grid{grid-template-columns:repeat(2,1fr)}.grid-2{grid-template-columns:1fr}}@media(max-width:650px){.customer-fulfillment{padding:16px}.module-tabs{gap:12px}.customer-picker-field{width:100%}.workspace-heading h2{font-size:24px}.fulfillment-settings-drawer{padding:20px}}
.fulfillment-orders-table{min-width:860px}.fulfillment-orders-table .amount-column{text-align:right;white-space:nowrap}.fulfillment-orders-table td{vertical-align:middle;padding-top:14px;padding-bottom:14px}.fulfillment-orders-table .receiver-cell{max-width:270px}.receiver-address{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:270px}.receiver-cell strong>span{font-weight:400;color:#718097;margin-left:6px}.order-status-label{display:inline-block;background:#f1f4f7;color:#526174;border-radius:4px;padding:4px 7px;font-size:11px;white-space:nowrap}.order-status-label.status-complete{color:#278456;background:#e8f6ed}.compact-order-actions{flex-wrap:nowrap;gap:12px}.cell-meta{margin-top:7px;font-size:11px;color:#718097}.fulfillment-orders-panel>.panel-head p{font-size:12px}.fulfillment-orders-panel>.panel-head h3{font-size:17px}.fulfillment-orders-panel{margin-top:10px}
 .billing-summary-grid{grid-template-columns:repeat(3,minmax(0,1fr));margin:0 0 18px}.billing-summary-grid>div{padding:16px 18px}.billing-summary-grid strong{margin:9px 0 0}.billing-workspace{display:grid;gap:14px;margin:0 0 18px}.billing-tabs,.billing-period-picker{display:flex;align-items:center;gap:6px;flex-wrap:wrap}.billing-tabs{border-bottom:1px solid #e1e7ee;padding-bottom:10px}.billing-tabs button,.billing-period-picker button{background:transparent;border:0;color:#718097;border-radius:6px;padding:8px 12px;font-size:12px}.billing-tabs button.active,.billing-period-picker button.active{background:#e8f6ed;color:#278456;font-weight:600}.billing-filters{display:flex;align-items:end;gap:12px;flex-wrap:wrap}.billing-filters label{min-width:160px}.billing-filters label span{display:block;color:#718097;font-size:11px;margin-bottom:4px}.billing-table{min-width:920px;width:100%;border-collapse:collapse}.billing-table th{background:#f7f9fb;color:#718097;font-size:11px;font-weight:550;white-space:nowrap}.billing-table td,.billing-table th{border-bottom:1px solid #edf0f4;padding:10px 9px;text-align:left}.billing-table td{font-size:12px;vertical-align:middle}.billing-table input[type=checkbox]{width:auto}.billing-pagination{display:flex;justify-content:flex-end;align-items:center;gap:8px;color:#718097;font-size:12px}.billing-preview{border:1px solid #d8e4dc;border-radius:9px;background:#fbfefc;padding:16px;display:grid;gap:12px}.billing-preview>.panel-head{margin:0}.billing-preview h4,.bill-pdf-settings h4{margin:0;font-size:15px}.billing-category-totals{display:flex;gap:8px;flex-wrap:wrap}.billing-category-totals span{background:#eef6f0;color:#267a4e;border-radius:5px;padding:5px 8px;font-size:11px}.billing-seal-pages{display:flex;gap:10px;flex-wrap:wrap;color:#526174;font-size:12px}.billing-seal-pages label{display:inline-flex;align-items:center;gap:5px}.billing-seal-pages input{width:auto}.billing-bill-mask{z-index:1200}.billing-bill-drawer{width:min(1160px,98vw);max-width:1160px}.billing-bill-summary{display:flex;gap:16px;flex-wrap:wrap;padding:12px 0;color:#526174;font-size:12px}.billing-bill-summary span:first-child{font-weight:650;color:#142033}.billing-bill-drawer>.table-scroll{max-height:42vh;overflow:auto;margin:12px 0 18px}.bill-pdf-settings{border-top:1px solid #e1e7ee;padding-top:16px;display:grid;gap:12px}.order-row-unpaid{background:#fff1f0;color:#b42318}.order-row-unpaid td{border-color:#f7d4d1}.order-row-unpaid .cell-meta,.order-row-unpaid .receiver-cell strong>span{color:#b42318}.order-row-unpaid .order-status-label.status-unpaid{color:#b42318;background:#fee4e2}.order-row-complete{background:#e8f6ed;color:#278456}.order-row-complete td{border-color:#d8efdf}.order-status-label.status-unpaid{color:#b42318;background:#fee4e2}
@media(max-width:800px){.billing-summary-grid{grid-template-columns:1fr}.billing-filters{align-items:stretch}.billing-bill-drawer{padding:16px}.billing-bill-summary{gap:9px}}
</style>
