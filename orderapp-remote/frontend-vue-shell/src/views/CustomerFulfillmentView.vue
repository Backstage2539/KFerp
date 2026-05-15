<template>
  <div class="page customer-fulfillment">
    <section class="panel control-panel">
      <div class="panel-head">
        <div>
          <h2>客户履约运营台</h2>
          <p>内部运营 · {{ overview.customer_name || selectedCustomerLabel || '未选择客户' }}</p>
        </div>
        <div class="head-actions">
          <button class="secondary" type="button" @click="openManual">客户履约手册</button>
          <button class="secondary" type="button" @click="loadAll" :disabled="loading || !normalizedCustomerId">刷新</button>
        </div>
      </div>

      <div class="toolbar">
        <label class="customer-picker-field">
          <span>选择客户</span>
          <SearchableSelect
            v-model="customerId"
            :options="customerOptions"
            :option-label="customerFulfillmentCustomerOptionLabel"
            :option-meta="customerFulfillmentCustomerOptionMeta"
            :option-value="optionNumericValue"
            placeholder="搜索客户名/公司/联系人/电话"
            empty-text="没有匹配客户"
            :disabled="loading"
            @select="selectCustomer" />
        </label>
        <button class="primary" type="button" @click="loadAll" :disabled="loading || !normalizedCustomerId">载入账户</button>
      </div>

      <p class="muted account-hint">外部用户配置已移到“门户客户配置”；配置手机号、密码并启用登录后，该客户会出现在这里。</p>

      <div v-if="workbenchSections.imports" class="import-row">
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

      <div v-if="workbenchSections.settlement" class="settlement-row">
        <label>
          <span>结算开始</span>
          <input v-model="settlement.period_from" type="date" />
        </label>
        <label>
          <span>结算结束</span>
          <input v-model="settlement.period_to" type="date" />
        </label>
        <button class="secondary" type="button" @click="createSettlement" :disabled="loading || !normalizedCustomerId">生成月结</button>
      </div>

      <div class="ops-grid">
        <div v-if="workbenchSections.processing" class="ops-panel">
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

        <div v-if="workbenchSections.directShip" class="ops-panel">
          <h3>{{ submitCopy.formTitle }}</h3>
          <div class="ops-form">
            <label class="wide-field">
              <span>粘贴收件信息</span>
              <textarea v-model="recipientPasteText" rows="2" placeholder="粘贴姓名、电话、地址" @paste.prevent="pasteRecipientInfo"></textarea>
            </label>
            <button class="secondary" type="button" @click="applyRecipientParse()" :disabled="loading">解析收件信息</button>
            <label>
              <span>历史收件信息</span>
              <SearchableSelect
                v-model="recipientHistoryValue"
                :options="recipientOptions"
                :option-label="recipientOptionLabel"
                :option-meta="recipientOptionMeta"
                :option-value="recipientOptionValue"
                empty-value=""
                placeholder="搜索姓名/电话/地址"
                empty-text="没有历史收件信息"
                :disabled="loading || !normalizedCustomerId"
                @select="selectRecipientHistory" />
            </label>
            <label>
              <span>收件人</span>
              <input v-model.trim="directShipForm.receiver_name" />
            </label>
            <label>
              <span>电话</span>
              <input v-model.trim="directShipForm.receiver_phone" />
            </label>
            <label class="wide-field">
              <span>地址</span>
              <input v-model.trim="directShipForm.receiver_address" />
            </label>
            <label>
              <span>商品</span>
              <SearchableSelect
                v-model="directShipProductValue"
                :options="directShipProductOptions"
                :option-label="productOptionLabel"
                :option-meta="productOptionMeta"
                :option-value="productOptionValue"
                empty-value=""
                placeholder="搜索客户 SKU/成品库存"
                empty-text="没有匹配商品"
                :disabled="loading || !normalizedCustomerId"
                @select="selectDirectShipProduct" />
            </label>
            <label>
              <span>规格</span>
              <input v-model.trim="directShipForm.spec" placeholder="100g" />
            </label>
            <label>
              <span>数量</span>
              <input v-model.number="directShipForm.quantity_units" type="number" min="1" />
            </label>
            <label class="wide-field">
              <span>备注</span>
              <input v-model.trim="directShipForm.note" :placeholder="submitCopy.notePlaceholder" />
            </label>
            <button class="primary" type="button" @click="submitDirectShipOrder" :disabled="loading || !normalizedCustomerId">{{ submitCopy.submitButton }}</button>
          </div>
        </div>

        <div v-if="workbenchSections.inventory" class="ops-panel">
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

      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section v-if="selectedParsedBatch" class="panel apply-panel">
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

    <section ref="resultAnchor" v-if="summaryCards.length" class="metric-grid">
      <div v-for="card in summaryCards" :key="card.label" class="metric">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </div>
    </section>

    <section v-if="invalidRows.length || latestInvalidCount" class="panel">
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
      <DataPanel v-if="workbenchSections.imports" title="导入批次" :rows="visibleImports" empty="暂无导入批次">
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

      <DataPanel v-if="workbenchSections.inventory" title="托管库存" :rows="overview.custody_balances" empty="暂无托管库存">
        <table>
          <thead>
            <tr><th>类型</th><th>名称</th><th>规格</th><th>克重</th><th>件数</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.custody_balances || []" :key="`${row.item_type}-${row.item_name}-${row.spec}`">
              <td>{{ custodyTypeLabel(row.item_type) }}</td>
              <td>{{ row.item_name }}</td>
              <td>{{ row.spec || '-' }}</td>
              <td>{{ row.quantity_g || 0 }}</td>
              <td>{{ row.quantity_units || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel v-if="workbenchSections.processing" title="加工工单" :rows="overview.processing_orders" empty="暂无加工工单">
        <table>
          <thead>
            <tr><th>工单号</th><th>产品</th><th>状态</th><th>投豆</th><th>产量</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.processing_orders || []" :key="row.work_order_no">
              <td>{{ row.work_order_no }}</td>
              <td>{{ row.product_name }}</td>
              <td>{{ row.status || '-' }}</td>
              <td>{{ row.quantity_g || 0 }}</td>
              <td>{{ row.units || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel v-if="workbenchSections.settlement" title="费用明细" :rows="overview.fees" empty="暂无费用">
        <table>
          <thead>
            <tr><th>类型</th><th>名称</th><th>金额</th><th>来源</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.fees || []" :key="`${row.fee_type}-${row.fee_name}-${row.amount_cents}`">
              <td>{{ row.fee_type }}</td>
              <td>{{ row.fee_name }}</td>
              <td>{{ moneyFromCents(row.amount_cents) }}</td>
              <td>{{ row.source || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel v-if="workbenchSections.settlement" title="结算批次" :rows="overview.settlements" empty="暂无结算">
        <table>
          <thead>
            <tr><th>ID</th><th>期间</th><th>状态</th><th>金额</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.settlements || []" :key="row.batch_id">
              <td>{{ row.batch_id }}</td>
              <td>{{ row.period_from }} 至 {{ row.period_to }}</td>
              <td>{{ row.status }}</td>
              <td>{{ moneyFromCents(row.total_amount_cents) }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>
    </section>

    <section v-if="workbenchSections.orders" class="panel fulfillment-orders-panel">
      <div class="panel-head">
        <div>
          <h3>履约客户订单</h3>
          <p>按当前履约客户读取 ERP 订单列表，订单费用、销售单和出库单都在这里核对。</p>
        </div>
        <div class="head-actions">
          <span class="muted">共 {{ fulfillmentOrdersSummary.orders || 0 }} 单</span>
          <button class="secondary" type="button" @click="loadFulfillmentOrders(fulfillmentOrdersPage)" :disabled="fulfillmentOrdersLoading || !normalizedCustomerId">
            刷新
          </button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="fulfillment-orders-table">
          <thead>
            <tr>
              <th>订单号</th>
              <th>日期</th>
              <th>客户 / 类型</th>
              <th>收件信息</th>
              <th>订单费用</th>
              <th>快递信息</th>
              <th>订单状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in fulfillmentOrders" :key="row.id">
              <td>
                <button class="order-link" type="button" @click="openFulfillmentOrderDetail(row)">{{ row.order_no }}</button>
                <div class="cell-meta">{{ row.portal_service_code || row.order_type || '-' }}</div>
              </td>
              <td>{{ row.order_date || '-' }}</td>
              <td>
                <div class="stacked-text">
                  <strong>{{ row.customer || '-' }}</strong>
                  <span>{{ row.order_type || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="stacked-text receiver-cell">
                  <strong>{{ row.receiver_name || '-' }} {{ row.receiver_phone || '' }}</strong>
                  <span>{{ row.receiver_address || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="fee-stack">
                  <div v-for="fee in orderFeeLines(row)" :key="`${row.id}-${fee.label}`" class="fee-line" :class="{ emphasized: fee.emphasized }">
                    <span>{{ fee.label }}</span>
                    <strong>{{ fee.value }}</strong>
                  </div>
                </div>
              </td>
              <td>
                <div class="stacked-text">
                  <strong>{{ row.sender_label || row.sender_name || '-' }}</strong>
                  <span>{{ row.ship_tracking_no || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="status-stack">
                  <span>收款：{{ row.pay_status || '-' }}</span>
                  <span>发货：{{ row.ship_status || '-' }}</span>
                  <span>生产：{{ row.process_status || '-' }}</span>
                  <span>发票：{{ row.invoice_status || '-' }}</span>
                </div>
              </td>
              <td>
                <div class="actions-inline">
                  <button class="link-button" type="button" @click="openSalesOrderDrawer(row)">销售单</button>
                  <button class="link-button" type="button" @click="openDeliveryNoteDrawer(row)" :disabled="!isShipped(row)">出库单</button>
                </div>
              </td>
            </tr>
            <tr v-if="!fulfillmentOrders.length">
              <td colspan="8" class="muted empty-row">{{ fulfillmentOrdersLoading ? '加载中...' : '暂无履约订单' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="pager">
        <button class="secondary" type="button" @click="loadFulfillmentOrders(fulfillmentOrdersPage - 1)" :disabled="!fulfillmentOrdersHasPrev || fulfillmentOrdersLoading">上一页</button>
        <span>第 {{ fulfillmentOrdersPage }} 页</span>
        <button class="secondary" type="button" @click="loadFulfillmentOrders(fulfillmentOrdersPage + 1)" :disabled="!fulfillmentOrdersHasNext || fulfillmentOrdersLoading">下一页</button>
      </div>
    </section>

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
          <section class="drawer-section">
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
              <span>发货：{{ activeOrderSummary.ship_status || '-' }}</span>
              <span>生产：{{ activeOrderSummary.process_status || '-' }}</span>
              <span>发票：{{ activeOrderSummary.invoice_status || '-' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <h4>订单费用</h4>
            <div class="detail-grid">
              <span>商品金额：{{ activeOrderDetail?.total_amount || activeOrderSummary.total_amount || '0.00' }}</span>
              <span>运费：{{ activeOrderDetail?.shipping_amount || activeOrderSummary.shipping_amount || '0.00' }}</span>
              <span>优惠：{{ activeOrderDetail?.discount_amount || activeOrderSummary.discount_amount || '0.00' }}</span>
              <span>应收：{{ activeOrderDetail?.grand_total || activeOrderSummary.grand_total || '0.00' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <h4>订单信息</h4>
            <div class="detail-grid">
              <span>类型：{{ activeOrderSummary.order_type || '-' }}</span>
              <span>来源：{{ activeOrderSummary.portal_service_code || '-' }}</span>
              <span>负责人：{{ activeOrderSummary.responsible_name || '-' }}</span>
              <span>录入：{{ activeOrderSummary.created_by_employee || '-' }}</span>
              <span class="wide-item">备注：{{ activeOrderDetail?.notes || activeOrderSummary.notes || '-' }}</span>
            </div>
          </section>

          <section class="drawer-section">
            <div class="drawer-actions">
              <button class="secondary" type="button" @click="openSalesOrderDrawer(activeOrderSummary)">销售单</button>
              <button class="secondary" type="button" @click="openDeliveryNoteDrawer(activeOrderSummary)" :disabled="!isShipped(activeOrderSummary)">出库单</button>
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
                    <td>{{ item.product_name || '-' }}</td>
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
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import DataPanel from '../components/DataPanel.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import DeliveryNoteView from './DeliveryNoteView.vue'
import SalesOrderView from './SalesOrderView.vue'
import {
  applyCustomerFulfillmentImport,
  adjustCustomerFulfillmentCustodyInventory,
  createCustomerFulfillmentSettlement,
  fetchCustomerFulfillmentCustomers,
  fetchCustomerFulfillmentOrderDetail,
  fetchCustomerFulfillmentOrders,
  fetchCustomerFulfillmentOptions,
  fetchCustomerFulfillmentImportPreview,
  fetchCustomerFulfillmentImportRows,
  fetchCustomerFulfillmentImports,
  fetchCustomerFulfillmentOverview,
  parseCustomerFulfillmentImport,
  submitCustomerFulfillmentDirectShipOrder,
  submitCustomerFulfillmentProcessingWorkOrder,
} from '../api/customer-fulfillment'
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
import { parseRecipientText } from '../lib/customer-recipient'

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
const settlement = reactive({
  period_from: '',
  period_to: '',
})
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
const directShipProductValue = ref('')
const recipientHistoryValue = ref('')
const recipientPasteText = ref('')
const directShipForm = reactive({
  receiver_name: '',
  receiver_phone: '',
  receiver_address: '',
  product_id: 0,
  product_name: '',
  spec: '',
  quantity_units: 1,
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
const workbenchSections = computed(() => customerFulfillmentWorkbenchSections(enabledCapabilities.value))
const visibleImportTypes = computed(() => importTypeOptions(enabledCapabilities.value))
const visibleImports = computed(() => visibleCustomerFulfillmentImports(imports.value, enabledCapabilities.value))
const submitCopy = computed(() => customerFulfillmentSubmitCopy(enabledCapabilities.value))
const selectedParsedBatch = computed(() => latestParsedBatchForType(visibleImports.value, latestBatch.value, selectedImportType.value))
const selectedParsedBatchId = computed(() => Number(selectedParsedBatch.value?.id || selectedParsedBatch.value?.batch_id || 0))
const selectedParsedBatchLabel = computed(() => {
  const batch = selectedParsedBatch.value
  if (!batch) return ''
  return `${batch.source_filename || '未命名文件'} / ${importTypeLabel(batch.import_type)}`
})
const customerSKUOptions = computed(() => fulfillmentOptions.value?.customer_skus || [])
const custodyItemOptions = computed(() => fulfillmentOptions.value?.custody_items || [])
const rawBeanOptions = computed(() => custodyItemOptions.value.filter((row) => row.item_type === 'raw_bean'))
const adjustmentItemOptions = computed(() => custodyItemOptions.value.filter((row) => row.item_type === adjustment.item_type))
const finishedGoodsProductOptions = computed(() => (overview.value?.finished_goods || []).map((row) => ({
  product_id: row.product_id,
  product_name: row.product_name,
  spec: row.spec_g ? `${row.spec_g}g` : '',
  warehouse: row.warehouse,
  quantity_units: row.quantity_units,
  quantity_g: row.quantity_g,
  source: 'finished_goods',
})))
const directShipProductOptions = computed(() => uniqueProductOptions([
  ...customerSKUOptions.value,
  ...finishedGoodsProductOptions.value,
]))
const recipientOptions = computed(() => fulfillmentOptions.value?.recipients || [])
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

onMounted(async () => {
  await loadCustomerOptions()
  const params = new URL(window.location.href).searchParams
  customerId.value = Number(params.get('customer_id') || 0)
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
  closeOrderDetailDrawer()
  if (customerId.value) await loadAll()
}

async function loadAll() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  fulfillmentOrdersLoading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [overviewData, importData, optionsData, orderData] = await Promise.all([
      fetchCustomerFulfillmentOverview(normalizedCustomerId.value),
      fetchCustomerFulfillmentImports(normalizedCustomerId.value),
      fetchCustomerFulfillmentOptions(normalizedCustomerId.value),
      loadFulfillmentOrdersData(normalizedCustomerId.value, fulfillmentOrdersPage.value),
    ])
    overview.value = overviewData || {}
    fulfillmentOptions.value = optionsData || {}
    rememberOverviewCustomer(overviewData)
    imports.value = importData?.imports || overviewData?.imports || []
    assignFulfillmentOrders(orderData)
  } catch (err) {
    error.value = err.message || '加载客户履约账户失败'
  } finally {
    loading.value = false
    fulfillmentOrdersLoading.value = false
  }
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

async function submitDirectShipOrder() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await submitCustomerFulfillmentDirectShipOrder(normalizedCustomerId.value, {
      receiver_name: directShipForm.receiver_name,
      receiver_phone: directShipForm.receiver_phone,
      receiver_address: directShipForm.receiver_address,
      product_id: Number(directShipForm.product_id || 0),
      product_name: directShipForm.product_name,
      spec: directShipForm.spec,
      quantity_units: Number(directShipForm.quantity_units || 0),
      note: directShipForm.note,
    })
    ok.value = `${submitCopy.value.successPrefix} ${row.order_no || ''}`
    directShipProductValue.value = ''
    recipientHistoryValue.value = ''
    recipientPasteText.value = ''
    directShipForm.receiver_name = ''
    directShipForm.receiver_phone = ''
    directShipForm.receiver_address = ''
    directShipForm.product_id = 0
    directShipForm.product_name = ''
    directShipForm.spec = ''
    directShipForm.quantity_units = 1
    directShipForm.note = ''
    fulfillmentOrdersPage.value = 1
    await loadAll()
  } catch (err) {
    error.value = err.message || submitCopy.value.errorFallback
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

async function loadFulfillmentOrders(page = fulfillmentOrdersPage.value) {
  if (!normalizedCustomerId.value) return
  fulfillmentOrdersLoading.value = true
  try {
    const data = await loadFulfillmentOrdersData(normalizedCustomerId.value, page)
    assignFulfillmentOrders(data)
  } catch (err) {
    error.value = err.message || '加载履约客户订单失败'
  } finally {
    fulfillmentOrdersLoading.value = false
  }
}

function loadFulfillmentOrdersData(customerId, page = 1) {
  return fetchCustomerFulfillmentOrders(customerId, { page, limit: 10 })
}

function assignFulfillmentOrders(data = {}) {
  fulfillmentOrders.value = Array.isArray(data?.rows) ? data.rows : []
  fulfillmentOrdersSummary.value = data?.summary || {}
  fulfillmentOrdersPage.value = Number(data?.page || fulfillmentOrdersPage.value || 1)
  fulfillmentOrdersHasPrev.value = Boolean(data?.has_prev)
  fulfillmentOrdersHasNext.value = Boolean(data?.has_next)
  const currentID = Number(activeOrderSummary.value?.id || 0)
  if (currentID > 0) {
    const refreshed = fulfillmentOrders.value.find((row) => Number(row.id) === currentID)
    if (refreshed) activeOrderSummary.value = { ...refreshed }
  }
}

async function openFulfillmentOrderDetail(row) {
  const orderId = Number(row?.id || 0)
  if (!orderId) return
  activeOrderSummary.value = { ...row }
  activeOrderDetail.value = null
  orderDetailError.value = ''
  orderDetailDrawerOpen.value = true
  orderDetailLoading.value = true
  try {
    const data = await fetchCustomerFulfillmentOrderDetail(orderId)
    activeOrderDetail.value = data?.edit_data || null
  } catch (err) {
    orderDetailError.value = err.message || '加载订单明细失败'
  } finally {
    orderDetailLoading.value = false
  }
}

function closeOrderDetailDrawer() {
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

async function createSettlement() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await createCustomerFulfillmentSettlement(normalizedCustomerId.value, settlement)
    ok.value = `已生成结算 ${result.batch_id || ''}`
    await loadAll()
  } catch (err) {
    error.value = err.message || '生成结算失败'
  } finally {
    loading.value = false
  }
}

function selectProcessingProduct(option) {
  processingForm.product_id = Number(option?.product_id || 0)
  processingForm.product_name = String(option?.product_name || '').trim()
}

function selectProcessingRawBean(option) {
  processingForm.raw_bean_item_id = Number(option?.item_id || 0)
  processingForm.raw_bean_name = String(option?.item_name || '').trim()
}

function selectDirectShipProduct(option) {
  directShipForm.product_id = Number(option?.product_id || 0)
  directShipForm.product_name = String(option?.product_name || '').trim()
  if (option?.spec) directShipForm.spec = String(option.spec).trim()
}

function selectAdjustmentItem(option) {
  adjustment.item_type = option?.item_type || adjustment.item_type
  adjustment.item_name = String(option?.item_name || '').trim()
  adjustment.spec = String(option?.spec || '').trim()
}

function pasteRecipientInfo(event) {
  const text = event?.clipboardData?.getData('text') || ''
  recipientPasteText.value = text
  applyRecipientParse(text)
}

function applyRecipientParse(text = recipientPasteText.value) {
  const parsed = parseRecipientText(text)
  applyRecipientFields(parsed)
}

function selectRecipientHistory(option) {
  const snapshot = [option?.receiver_name, option?.receiver_phone, option?.receiver_address].filter(Boolean).join(' ')
  recipientPasteText.value = snapshot
  const parsed = parseRecipientText(snapshot)
  applyRecipientFields({
    recipient_name: option?.receiver_name || parsed.recipient_name,
    phone: option?.receiver_phone || parsed.phone,
    address: option?.receiver_address || parsed.address,
  })
}

function applyRecipientFields(parsed) {
  if (parsed?.recipient_name) directShipForm.receiver_name = parsed.recipient_name
  if (parsed?.phone) directShipForm.receiver_phone = parsed.phone
  if (parsed?.address) directShipForm.receiver_address = parsed.address
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

function uniqueProductOptions(rows) {
  const out = []
  const seen = new Set()
  for (const row of rows || []) {
    const name = String(row?.product_name || '').trim()
    if (!name) continue
    const normalized = { ...row, product_name: name, spec: String(row?.spec || '').trim() }
    const key = `${normalized.product_id || 0}|${normalized.product_name}|${normalized.spec}|${normalized.warehouse || ''}`
    if (seen.has(key)) continue
    seen.add(key)
    out.push(normalized)
  }
  return out
}

function importTypeLabel(value) {
  return importTypeOptions(enabledCapabilities.value).find((option) => option.value === value)?.label || value
}

function orderFeeLines(row) {
  return customerFulfillmentOrderFees(row)
}

function isShipped(row) {
  return String(row?.ship_status || '').includes('已发货')
}

function custodyTypeLabel(value) {
  return { raw_bean: '生豆', packaging: '包材', product: '产品' }[value] || value
}

function moneyFromCents(value) {
  return (Number(value || 0) / 100).toFixed(2)
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
  overflow-x: auto;
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

.pager {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 12px;
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
}
</style>
