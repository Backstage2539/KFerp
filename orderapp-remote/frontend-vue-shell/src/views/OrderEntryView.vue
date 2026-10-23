<template>
  <div class="page" :class="{ embedded: props.embedded }">
    <section v-if="editorSnapshotPending" class="panel" aria-label="加载订单编辑">
      <div class="section-row"><h2>编辑订单</h2><button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button></div>
      <p v-if="loading || !error" role="status">正在读取订单及商品、费用信息…</p>
      <p v-else class="notice error" role="alert">{{ error }} <button class="secondary" type="button" @click="load" :disabled="loading">重新加载</button></p>
    </section>
    <template v-else>
    <section class="order-hero">
      <div>
        <p class="eyebrow">订单销售</p>
        <h2>{{ backfillMode && canUseBackfillMode ? '订单补录' : (copyMode ? '复制订单' : (form.edit_id ? '编辑订单' : '录单')) }}</h2>
      </div>
      <div class="hero-actions">
        <div class="total-pill">
          <span>订单合计</span>
          <strong>{{ money(orderTotalPreviewValue.grandTotal) }}</strong>
          <small>{{ orderTotalHintText }}</small>
        </div>
        <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="error" class="global-error-toast notice error" role="alert">
      <div class="toast-text">
        <strong>操作失败</strong>
        <span>{{ error }}</span>
      </div>
      <button class="toast-close" type="button" aria-label="关闭错误提示" @click="error = ''">×</button>
    </div>
    <div v-if="ok" class="notice ok">订单已保存：{{ ok }}</div>
    <div v-if="copyMode" class="notice warn">复制订单会生成一张新订单，不会修改原订单。</div>
    <div v-if="sampleOrderEditing" class="notice">正在编辑样品销售订单：已确认的零元手动价和样品规格会保留。生产及发货后的编辑限制继续生效。</div>
    <div v-if="stockBatchNotice" class="notice warn">{{ stockBatchNotice }}</div>

    <section class="panel order-fields">
      <div class="section-title">订单信息</div>
      <div class="form-grid">
        <div class="backfill-hint" :class="{ active: backfillMode && canUseBackfillMode }">
          <div class="backfill-copy">
            <strong>订单补录</strong>
            <span>单据日期用于制单和订单编号，订单日期用于客户真实下单时间。</span>
          </div>
          <label v-if="canUseBackfillMode" class="backfill-toggle">
            <input v-model="backfillMode" type="checkbox" />
            <span>连续补录</span>
          </label>
        </div>
        <label>
          <span>单据日期</span>
          <input v-model.trim="form.document_date" type="date" :disabled="fulfillmentOrderEditing" />
        </label>

        <label>
          <span>订单日期</span>
          <input v-model.trim="form.order_date" type="date" />
        </label>

        <label v-if="!props.fulfillmentMode && !fulfillmentOrderEditing" class="customer-combobox combobox" :class="{ open: customerOpen }">
          <div class="label-row">
            <span>客户</span>
            <span class="label-actions">
              <button class="text-button" type="button" @click="openCustomerDrawer">新增客户</button>
              <button class="text-button" type="button" @click="openCustomerEditDrawer" :disabled="!form.customer_id">编辑客户</button>
            </span>
          </div>
          <div class="field-shell" :class="{ 'field-invalid': hasFieldError('customer_id') }" data-error-field="customer_id">
            <input
              v-model.trim="customerQuery"
              type="search"
              placeholder="输入客户名/拼音"
              autocomplete="off"
              @focus="customerOpen = true"
              @input="form.customer_id = 0; customerOpen = true"
              @keydown.down.prevent="customerOpen = true"
            />
          </div>
          <div v-if="customerOpen" class="combo-menu">
            <button
              v-for="item in filteredCustomers"
              :key="item.id"
              type="button"
              class="combo-option"
              @mousedown.prevent="chooseCustomer(item)"
            >
              <strong>{{ item.name }}</strong>
              <small v-if="item.customer_type || item.default_source_id || item.default_order_type_id">
                {{ customerTypeLabel(item.customer_type) || '未设置客户类型' }} / {{ optionName(sources, item.default_source_id) || '未设置来源' }} / {{ optionName(orderTypes, item.default_order_type_id) || '未设置订单类型' }}
              </small>
            </button>
            <div v-if="!filteredCustomers.length" class="combo-empty">没有匹配客户</div>
          </div>
        </label>

        <Teleport :to="props.recipientTarget || 'body'" :disabled="!props.recipientTarget" defer>
        <div v-if="!isCustomerStockOrder(form) && (props.fulfillmentMode || form.edit_id)" class="fulfillment-recipient-section full-span">
          <CustomerRecipientPicker v-if="!props.customerPortal" :customer-id="form.customer_id" :current-recipient="form" @select="applyAddressBookRecipient" />
          <label class="full-span"><span>粘贴收件信息</span><textarea v-model="fulfillmentRecipientPaste" rows="2" placeholder="姓名、电话、地址"></textarea><button type="button" @click="parseFulfillmentRecipient">解析收件信息</button></label>
          <label v-if="props.fulfillmentMode" class="readonly-field">
            <span>履约客户</span>
            <input :value="props.customerContextLabel || `客户 #${props.customerContextId}`" readonly />
          </label>
          <label>
            <span>收件人</span>
            <input v-model.trim="form.receiver_name" />
          </label>
          <label>
            <span>电话</span>
            <input v-model.trim="form.receiver_phone" />
          </label>
          <label><span>公司（选填）</span><input v-model.trim="form.receiver_company" /></label>
          <label class="full-span" data-error-field="receiver_address" :class="{ 'field-invalid': hasFieldError('receiver_address') }">
            <span>地址</span>
            <input v-model.trim="form.receiver_address" />
          </label>
          <label v-if="props.recipientInfo?.historyOptions?.length" class="full-span">
            <span>历史收件信息</span>
            <select @change="applyRecipientHistory($event)">
              <option value="">选择历史收件信息...</option>
              <option v-for="h in props.recipientInfo.historyOptions" :key="h.key" :value="h.key">{{ h.label }} ({{ h.phone }}) {{ h.address }}</option>
            </select>
          </label>
        </div>
        </Teleport>

        <div v-if="!props.customerPortal && !sampleOrderEditing" class="delivery-choice full-span">
          <label><span>交付方式</span><select v-model="form.delivery_mode" :disabled="fulfillmentOrderEditing"><option value="ship">正常发货</option><option value="customer_stock">购货留厂（进入客户库存）</option></select></label>
          <label v-if="form.delivery_mode === 'customer_stock'"><span>目标客户成品仓</span><select v-model="form.delivery_warehouse_code" :disabled="deliveryWarehousesLoading"><option value="">请选择客户仓库</option><option v-for="warehouse in deliveryWarehouseOptions" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label>
          <p v-if="form.delivery_mode === 'customer_stock'" class="muted">货款按本次销售订单结算，形成待入库权益并按订单排产。实际入客户仓后才能发货；后续代发只收服务费和运费。</p>
          <p v-if="deliveryWarehouseError" class="error">{{ deliveryWarehouseError }}</p>
        </div>

        <label v-if="fulfillmentOrderEditing" class="readonly-field"><span>客户</span><input :value="selectedCustomer?.name || props.customerContextLabel || `客户 #${form.customer_id}`" readonly /></label>
        <label v-if="!props.customerPortal" class="readonly-field">
          <span>客户负责人</span>
          <input :value="selectedCustomerResponsibleLabel" placeholder="先在客户资料指定负责人" readonly />
        </label>

        <div class="customer-profile-summary full-span">
          <div v-for="item in selectedCustomerProfileSummary" :key="item.key" class="profile-summary-item" :class="{ missing: item.missing }">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </div>
        </div>

        <Teleport :to="props.workflowTarget || 'body'" :disabled="!props.workflowTarget" defer>
          <div :class="props.workflowTarget ? 'form-grid' : 'workflow-fields-inline'">
          <template v-if="!fulfillmentOrderEditing">
        <label v-if="!props.customerPortal">
          <span>付款状态</span>
          <strong v-if="freeSampleSettlement" class="no-payment">无需收款 · 已结清</strong>
          <select v-else v-model.number="form.pay_status_id">
            <option v-for="item in payStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>

        <div v-if="!props.customerPortal && !freeSampleSettlement" class="prepayment-editor" data-prepayment-editor>
          <label>
            <span>已支付预付款（元）</span>
            <input v-model="form.prepayment_amount" type="number" min="0" step="0.01" placeholder="填写实际已收金额" @input="onManualPrepayment" />
          </label>
          <div class="prepayment-presets" aria-label="预付款比例">
            <button v-for="rate in prepaymentPresets" :key="rate" type="button" :class="{ selected: prepaymentRate === rate }" @click="applyPrepaymentPreset(rate)">{{ rate }}%</button>
          </div>
          <small>按应收合计 ¥{{ money(orderTotalPreviewValue.grandTotal) }} 计算{{ isCustomerStockOrder(form) ? '' : '（含运费）' }}；可修改实际已收金额。</small>
        </div>

        <label v-if="!props.customerPortal && !freeSampleSettlement" :class="{ 'field-invalid': hasFieldError('payment_method') }" data-error-field="payment_method">
          <span>收款方式</span>
          <select v-model.trim="form.payment_method" :disabled="!paymentMethodRequired">
            <option value="">选择收款方式</option>
            <option v-for="item in orderReceiptMethodOptions" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>

        <label v-if="!props.customerPortal && !isCustomerStockOrder(form)">
          <span>配送方式</span>
          <select v-model="deliveryMode">
            <option value="express">快递</option>
            <option value="pickup">自提</option>
            <option value="local_delivery">本地送货</option>
          </select>
        </label>
        <label v-if="!props.customerPortal && !props.fulfillmentMode && !isCustomerStockOrder(form)">
          <span>交付状态</span>
          <select v-model.number="form.ship_status_id">
            <option v-for="item in shipStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>

        <label v-if="!props.customerPortal && !props.fulfillmentMode && !isCustomerStockOrder(form) && !isNonCourierShipMethod(form.ship_method)">
          <span>快递单号（可多个）</span>
          <textarea v-model.trim="form.ship_tracking_no" rows="2" placeholder="多个单号可用换行、逗号或分号分隔"></textarea>
        </label>

        <div v-if="logisticsRequired" class="conditional-panel full-span">
          <div class="condition-title">发货物流</div>
          <label :class="{ 'field-invalid': hasFieldError('logistics_company_id') }" data-error-field="logistics_company_id">
            <span>物流公司</span>
            <select v-model.number="form.logistics_company_id" @change="syncLogisticsProduct">
              <option :value="0">选择物流公司</option>
              <option v-for="item in logisticsCompanies" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label :class="{ 'field-invalid': hasFieldError('logistics_product_id') }" data-error-field="logistics_product_id">
            <span>物流产品</span>
            <select v-model.number="form.logistics_product_id">
              <option :value="0">选择物流产品</option>
              <option v-for="item in selectedLogisticsProducts" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
        </div>

        <div v-if="paymentReceiptVisible" class="conditional-panel full-span">
          <div class="condition-title">收款凭证</div>
          <label :class="{ 'field-invalid': hasFieldError('payment_goods_amount') }" data-error-field="payment_goods_amount">
            <span>货款金额</span>
            <div class="amount-suggestion-wrap">
              <input v-model.trim="form.payment_goods_amount" type="number" min="0" step="0.01" />
              <button
                v-if="showPaymentGoodsAmountSuggestion"
                class="amount-suggestion-popover"
                type="button"
                @click="applyPaymentGoodsAmountSuggestion">
                实时价格提示 货款 {{ paymentGoodsAmountSuggestion }}
              </button>
            </div>
          </label>
          <label :class="{ 'field-invalid': hasFieldError('payment_shipping_amount') }" data-error-field="payment_shipping_amount">
            <span>实收运费（元）</span>
            <div class="amount-suggestion-wrap">
              <input :disabled="!form.edit_id && (!form.freight_state?.manual || form.freight_state?.free)" v-model.trim="form.payment_shipping_amount" type="number" min="0" step="0.01" />
              <button
                v-if="showPaymentShippingAmountSuggestion"
                class="amount-suggestion-popover"
                type="button"
                @click="applyPaymentShippingAmountSuggestion">
                实时价格提示 运费 {{ paymentShippingAmountSuggestion }}
              </button>
            </div>
          </label>
          <div class="voucher-field" :class="{ 'field-invalid': hasFieldError('payment_voucher_asset_id') }" data-error-field="payment_voucher_asset_id">
            <span>收款凭证</span>
            <div v-if="paymentVoucher && paymentVoucherCollapsed" class="voucher-collapsed">
              <button class="voucher-summary" type="button" @click="openPaymentVoucherPreview">
                <strong>{{ paymentVoucher.filename || '已上传凭证' }}</strong>
                <small>点击查看大图</small>
              </button>
              <button class="text-button" type="button" @click="paymentVoucherCollapsed = false">更换凭证</button>
            </div>
            <label v-else class="file-upload-control">
              <input type="file" accept="image/*,.pdf" @change="handlePaymentVoucherFile" />
              <span class="file-button">选择文件</span>
              <span class="file-name">{{ paymentVoucherFile?.name || paymentVoucher?.filename || '未选择文件' }}</span>
            </label>
            <small v-if="uploadingVoucher">上传中...</small>
            <small v-else-if="paymentVoucher && !paymentVoucherCollapsed">{{ paymentVoucher.filename || '已上传凭证' }}</small>
          </div>
        </div>
          <p v-if="freeSampleSettlement" class="full-span no-payment">商品和运费均为零，无需填写收款方式或凭证。</p>
          </template>
          <p v-else class="full-span fulfillment-explanation">{{ fulfillmentEditContext?.fulfillment_mode === 'stock' ? '库存一件代发：从本客户全部已购库存及待入库权益选择商品；货款在原购货单结算，本代发单不重复收取货款。' : '无库存一件代发：原单商品沿用确认价格，新增或更换规格按当前授权代发固定价，另计服务费和运费。' }}</p>
          <p v-if="editBlockReason" class="full-span notice warn">{{ editBlockReason }}</p>
          </div>
        </Teleport>
      </div>

      <label class="notes">
        <span>备注</span>
        <textarea v-model.trim="form.notes" rows="2"></textarea>
      </label>
    </section>

    <section class="panel" :class="{ 'field-invalid': hasFieldError('product_items') }" data-error-field="product_items">
      <div class="section-row">
        <div class="section-title">商品明细</div>
        <div class="section-actions">
          <button v-if="!fulfillmentOrderEditing" class="secondary" type="button" @click="openBeanListDrawer" :disabled="!canOpenBeanListDrawer">选择价格表</button>
          <div v-if="!fulfillmentOrderEditing" class="bean-list-summary-list">
            <small v-for="item in selectedBeanListSummaryItems" :key="item.type" class="bean-list-summary">
              <span class="bean-list-summary-label">{{ item.label }}：</span>
              <span class="bean-list-summary-value">{{ item.versionLabel }}</span>
            </small>
          </div>
        </div>
      </div>
      <p v-if="fulfillmentOrderEditing && fulfillmentEditContext?.fulfillment_mode === 'stock'" class="quote-source-summary">商品来源：原购货库存/权益</p>
      <p v-else-if="quoteSourceSummary.length" class="quote-source-summary">报价来源：
        <span v-for="source in quoteSourceSummary" :key="source.key">{{ source.label }}</span>
      </p>
      <div class="line-list">
        <article v-for="(row, idx) in rows" :key="row.key" class="line-item">
          <label
            class="product-combobox combobox product-cell"
            :class="{ open: row.product_open }"
            :data-product-combobox-key="row.key"
          >
            <span>商品</span>
            <input
              v-model.trim="row.product_query"
              type="search"
              placeholder="选择商品"
              autocomplete="off"
              @focus="openProductDropdown(row)"
              @input="clearProduct(row)"
              @keydown.down.prevent="openProductDropdown(row)"
            />
            <div v-if="row.product_open" class="combo-menu">
              <div
                v-if="productKindFilterOptions(row).length > 1"
                class="product-kind-filter"
                role="group"
                aria-label="商品分类"
              >
                <button
                  v-for="option in productKindFilterOptions(row)"
                  :key="option.value || 'all'"
                  type="button"
                  class="product-kind-filter-option"
                  :class="{ active: activeProductKindFilter(row) === option.value }"
                  :aria-pressed="activeProductKindFilter(row) === option.value"
                  @mousedown.prevent
                  @click.stop="row.product_kind_filter = option.value"
                >
                  {{ option.label }}
                </button>
              </div>
              <button
                v-for="product in productOptions(row)"
                :key="productOptionKey(product)"
                type="button"
                class="combo-option"
                @mousedown.prevent="chooseProduct(row, product)"
              >
                <strong>{{ product.name }} <span class="kind-badge" :class="orderProductCategoryBadgeClass(product)">{{ orderProductCategoryLabel(product) }}</span></strong>
                <small v-if="productSpecCountForCurrentList(product)">{{ productSpecCountForCurrentList(product) }} 个可售规格</small>
                <small v-else-if="!product.__order_concrete_price_family && product.tiers?.length">{{ product.tiers.length }} 个价格梯度</small>
              </button>
              <div v-if="!productOptions(row).length" class="combo-empty">{{ !props.customerPortal && staffCatalog.loading ? '商品目录加载中…' : (!props.customerPortal && staffCatalog.error ? staffCatalog.error : '没有匹配商品') }}</div>
            </div>
          </label>

          <label>
            <span>{{ !isConcretePriceListRow(row) && isDripRow(row) ? '单位' : '规格' }}</span>
            <div class="spec-control">
              <select v-if="isConcretePriceListRow(row)" v-model="row.spec_mode" @change="onSpecChange(row)">
                <option value="">选择规格</option>
                <option
                  v-if="row.historical_spec_readonly && !specOptions(row).some((option) => option.value === row.spec_mode)"
                  :value="row.spec_mode"
                  disabled
                >
                  {{ row.spec_label || '历史规格' }}{{ preservesSampleSpec(row, sampleOrderEditing) ? '（已存样品规格）' : '（历史规格，当前价格表不可用）' }}
                </option>
                <option v-for="option in specOptions(row)" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
              <select v-else-if="isDripRow(row)" v-model="row.sales_unit" @change="onDripUnitChange(row)">
                <option v-for="option in dripUnitOptionsForRow(row)" :key="option.value" :value="option.value">
                  {{ option.spec }}
                </option>
              </select>
              <select v-else v-model="row.spec_mode" @change="onLegacySpecChange(row)">
                <option value="">选择规格</option>
                <option v-for="option in specOptions(row)" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
              <input
                v-if="!isConcretePriceListRow(row) && row.spec_mode === CUSTOM_SPEC_VALUE"
                v-model.number="row.custom_spec_g"
                type="number"
                min="1"
                step="1"
                placeholder="克数"
                @input="syncPrice(row)"
              />
            </div>
          </label>

          <label>
            <span>{{ quantityLabel(row) }}</span>
            <input v-model.number="row.qty" type="number" min="1" step="1" @input="syncPrice(row)" />
          </label>

          <label>
            <span>{{ retailOrder ? '单价' : `单价（${priceUnitLabel(row)}）` }}</span>
            <div class="price-control">
              <input
                v-model.trim="row.unit_price"
                :readonly="fulfillmentOrderEditing || props.customerPortal || isFrozenConversion(row)"
                type="number"
                min="0"
                step="0.01"
                placeholder="0.00"
                @input="markManualPrice(row)"
              />
              <button v-if="!fulfillmentOrderEditing && !props.customerPortal && !isFrozenConversion(row)" class="icon-button" type="button" title="恢复自动价格" @click="resetAutoPrice(row)">↺</button>
            </div>
          </label>

          <label v-if="!fulfillmentOrderEditing && !props.customerPortal" class="discount-control">
            <span>优惠</span>
            <div class="discount-inputs">
              <select v-model="row.discount_type" @change="onRowDiscountTypeChange(row)">
                <option value="">无优惠</option>
                <option value="amount">减免数额</option>
                <option value="unit_amount">单价优惠</option>
                <option value="percent">折扣</option>
                <option value="free">免费</option>
              </select>
              <input
                v-if="row.discount_type === 'amount' || row.discount_type === 'unit_amount' || row.discount_type === 'percent'"
                v-model.number="row.discount_value"
                type="number"
                min="0"
                step="0.01"
                :placeholder="discountValuePlaceholder(row)" />
            </div>
          </label>

          <div class="line-total">
            <span>小计</span>
            <strong>{{ money(rowTotal(row)) }}</strong>
            <small>{{ row.manual_price ? '手动价' : autoPriceLabel(row) }}</small>
            <small v-if="fulfillmentOrderEditing && fulfillmentLineState(row).requiresDiscountReview" class="price-missing-warning">该商品包含历史行折扣，修改数量前请先通过财务流程核对折扣</small>
            <small v-else-if="fulfillmentOrderEditing && fulfillmentLineState(row).discountAmount">原单行优惠 ¥{{ money(fulfillmentLineState(row).discountAmount) }}，已计入小计</small>
            <small v-if="rowHasBlockingPrice(row)" class="price-missing-warning">{{ rowPriceBlockingMessage(row) }}</small>
            <small
              v-if="row.product_id && row.bean_list_version_no"
              class="bean-list-version-meta"
              :class="{ stale: isRowBeanListVersionStale(row), open: row.bean_list_version_tip_open }"
            >
              <span v-if="isRowBeanListVersionStale(row)">历史报价</span>
              <button
                v-if="isRowBeanListVersionStale(row)"
                class="bean-list-version-warning"
                type="button"
                aria-label="非最新价格表"
                title="非最新价格表"
                @click.stop="toggleBeanListVersionTip(row)"
              >!</button>
              <span v-if="isRowBeanListVersionStale(row)" class="bean-list-version-tip" role="tooltip">非最新价格表</span>
            </small>
          </div>

          <button class="secondary danger" type="button" @click="removeRow(idx)" :disabled="rows.length === 1">删除</button>

          <p v-if="fulfillmentOrderEditing && fulfillmentEditContext?.fulfillment_mode === 'stock'" class="line-note fulfillment-supply">{{ fulfillmentSupplyLabel(row) }}</p>
          <label v-if="!fulfillmentOrderEditing" class="line-note">
            <span>条目备注</span>
            <input v-model.trim="row.item_note" placeholder="如贴标、磨粉、特殊包装" />
          </label>

          <div v-if="!fulfillmentOrderEditing && !props.customerPortal && !isFrozenConversion(row) && tierRows(row).length" class="tier-prices">
            <small class="tier-choice-hint">手动选择价格档（不受数量门槛限制）</small>
            <button
              v-for="tier in tierRows(row)"
              :key="tier.id || `${tier.specG}-${tier.rangeLabel}`"
              type="button"
              class="tier-price-chip"
              :class="{ active: isTierActive(row, tier) }"
              @click="selectTier(row, tier)"
            >
              <span>{{ tier.specLabel }} {{ tier.rangeLabel }}</span>
              <strong>{{ unitPriceMoney(tier.unitPrice) }}{{ tier.priceUnit?.suffix || '/磅' }}</strong>
            </button>
          </div>
        </article>
      </div>
      <div class="line-actions">
        <button class="secondary" type="button" @click="addRow">新增明细</button>
      </div>
    </section>

    <section class="panel footer-panel">
      <template v-if="fulfillmentOrderEditing">
        <div class="section-title">代发费用</div>
        <template v-if="!props.feesInSummary">
          <FulfillmentOrderFees :fees="fulfillmentCurrentFees" :freight-view="orderFreight || {}" :goods-cents="fulfillmentGoodsCents" :mode="fulfillmentEditContext?.fulfillment_mode" />
          <OrderPaymentSummary :order="fulfillmentEditData || {}" />
        </template>
        <p v-else class="muted">费用明细统一显示在上方“订单状态与费用明细”；重新核算后同步更新。</p>
        <div class="form-grid compact fulfillment-regions">
          <label><span>收件省份</span><input v-model.trim="form.shipping_province" @input="fulfillmentManualDestination = true" /></label><label><span>收件城市</span><input v-model.trim="form.shipping_city" @input="fulfillmentManualDestination = true" /></label><label><span>收件区县</span><input v-model.trim="form.shipping_district" @input="fulfillmentManualDestination = true" /></label>
        </div>
        <details class="fulfillment-adjustment"><summary>本单服务费调整</summary><label class="checkline"><input v-model="fulfillmentServiceAdjustment.enabled" type="checkbox" />人工调整本单服务费</label><template v-if="fulfillmentServiceAdjustment.enabled"><label><span>服务费（元）</span><input v-model="fulfillmentServiceAdjustment.amount" type="number" min="0" step="0.01" /></label><label><span>费用调整原因</span><textarea v-model.trim="fulfillmentServiceAdjustment.reason" rows="2" /></label></template></details>
        <button class="secondary" type="button" :disabled="saving || fulfillmentPreviewLoading || !!editBlockReason" @click="previewFulfillmentEdit">{{ fulfillmentPreviewLoading ? '核算中…' : '重新核算代发费用' }}</button>
        <p v-if="fulfillmentPreview" class="muted">已按本次商品和收件地区核算；保存时会再次核对。</p>
      </template>
      <OrderFreightEstimate v-model="form.freight_state" :order-id="Number(form.edit_id||0)" :expected-token="orderFreight?.edit_token??orderFreight?.token??''" :locked="!!orderFreight?.edit_locked" :stored-quote="orderFreight" @amount-change="form.shipping_amount=String($event)" v-if="!fulfillmentOrderEditing && form.delivery_mode!=='customer_stock' && (!form.edit_id || !props.customerPortal)" ref="freightEstimator" :customer-id="form.customer_id" :items="freightItems" :saved-destination="{province:form.shipping_province,city:form.shipping_city,district:form.shipping_district}" @destination-change="Object.assign(form,$event)" :address="form.receiver_address || selectedCustomer?.address || selectedCustomer?.company_address || ''" :customer-portal="props.customerPortal" />
      <div v-if="!fulfillmentOrderEditing && !props.customerPortal" class="section-title">费用</div>
      <div v-if="!fulfillmentOrderEditing && !props.customerPortal" class="form-grid compact">
        <label>
          <span>优惠</span>
          <input v-model.trim="form.discount_amount" type="number" min="0" step="0.01" />
        </label>
        <label v-if="!isCustomerStockOrder(form)">
          <span>快递费备注</span>
          <input v-model.trim="form.express_fee" />
        </label>
        <label class="checkline">
          <input v-model="form.round_to_int" type="checkbox" />
          <span>合计取整</span>
        </label>
      </div>
      <div class="save-row">
        <div class="grand-line">
          <span>订单合计</span>
          <strong>{{ money(orderTotalPreviewValue.grandTotal) }}</strong>
          <small>{{ orderTotalHintText }}</small>
        </div>
        <div class="save-actions">
          <button
            v-if="(backfillMode || props.customerPortal) && canUseBackfillMode"
            class="secondary"
            type="button"
            @click="save({ continueBackfill: false })"
            :disabled="saving || loading || !!editBlockReason || hasUnpricedPublishedRow"
          >
            保存并查看订单
          </button>
          <button
            v-if="(backfillMode || props.customerPortal) && canUseBackfillMode"
            class="primary"
            type="button"
            @click="save({ continueBackfill: true })"
            :disabled="saving || loading || !!editBlockReason || hasUnpricedPublishedRow"
          >
            {{ backfillMode ? '保存并继续补录' : '保存并继续' }}
          </button>
          <button
            v-else
            class="primary"
            type="button"
            @click="save({ continueBackfill: false })"
            :disabled="saving || loading || !!editBlockReason || hasUnpricedPublishedRow"
          >
            保存订单
          </button>
        </div>
      </div>
      <details class="manual">
        <summary>录单手册</summary>
        <p>售价和数量档位沿用所选已发布价格表；生产配置、物流重量使用当前 BOM。更新配方、成本或重量不需要重新发布价格表。新增规格或改变库存单位需单独发布报价，删除规格不可新下单。若保存时配置已更新，系统刷新规格及运费，请核对后再次保存。</p>
        <ul>
          <li>已有订单在详情顶部核对收件信息及付款与交付，在最底部点击“保存订单”保存全部修改。代发费用统一在上方“订单状态与费用明细”查看，重新核算后同步更新。订单地址是本单快照，修改客户档案不会覆盖。</li>
          <li>整单免费样品自动显示“无需收款”，不用填写收款方式或凭证。仅收运费时货款填 0，按实际运费登记并上传凭证；免费改收费先保存费用，再登记实际收款。</li>
          <li>客户和商品输入框支持名称、拼音和首字母搜索。</li>
          <li>录单时可点“新增客户”打开右侧抽屉，粘贴收件信息后可解析姓名、联系电话和地址。</li>
          <li>选择客户后会带入客户档案中的默认来源和订单类型。</li>
          <li>客户有历史订单时，商品下拉会把常用商品排在最前面。</li>
          <li>商品下拉按当前已选择并发布的商品价格表真实分类过滤，例如烘焙咖啡豆、咖啡生豆、咖啡挂耳、速溶咖啡和好玩的产品；还可输入名称或拼音。</li>
          <li>来源、客户类型和订单类型只在客户资料维护，录单选择客户后只读展示。</li>
          <li>商品明细区点击“选择价格表”可切换熟豆、生豆、挂耳已发布价格表；客户没有自定义价格表时使用公共价格表。</li>
          <li>同一价格表类型已有按商品分类发布的价格表时，新订单只自动启用当前分类价格表；旧全局价格表默认不参与商品候选，需要时可手工启用。仅有旧全局价格表时仍按历史规则自动使用。</li>
          <li>商品按父商品展示，商品名不再拼接规格；规格列和规格搜索只使用当前已选已发布价格表中有价的规格，价格表没有的商品档案规格不会出现在新订单候选中。</li>
          <li>每个商品类型默认带入当前版本的默认价格表，可以切换同版本内其他表。切表后保留仍可用的规格并重新匹配价格；不在新表中的规格必须重选后提交。</li>
          <li>零售客户也由录单人手动选择对应价格表。客户类型和订单类型不改变商品范围或自动切换报价；发布新版后重新打开商品选择器会刷新同名表，并提示核对已有明细。</li>
          <li>新版挂耳商品同样从当前价格表选择袋或盒规格；历史挂耳数据仍保留旧单位选择兼容。</li>
          <li v-if="props.customerPortal">商品与规格来自所选价格表，单价随数量匹配阶梯。补录可以暂缺收件信息，补齐后才能发货。</li>
          <li v-else>新订单默认已付款、未发货；商品单价会随规格和数量匹配价格梯度。</li>
          <li>按规格件数计价时，档位范围和单价单位使用所选 BOM 规格库存单位；例如 50 袋会匹配 48 袋以上档，不显示无意义的 0g。</li>
          <li>数量低于起订量、高于有限最高量或落在档位空隙时不猜测最低价；页面提示“当前数量无已发布价格，不能保存”，请调整数量、补齐已发布价格表档位或按授权流程输入手动价。</li>
          <li>需要临时改价时直接修改单价，点击 ↺ 恢复自动梯度价。表外 BOM 规格不能下单；历史换算订单仅保留原规格和冻结价格，复制时须重新选择当前价格表内的规格。</li>
          <li>每条商品明细可选择减免数额、单价优惠、折扣或免费，保存后会计入订单优惠。</li>
          <li>每条商品明细可填写“条目备注”，会随该商品带入销售单和出库单。</li>
          <li>库存充足时保存前会提示成品批次；历史库存没有 FP 批次时会提示库存余额，确认使用后进入库存待发货。</li>
        </ul>
      </details>
    </section>

    <div v-if="customerDrawerOpen" class="drawer-mask" @click.self="closeCustomerDrawer">
      <aside class="drawer">
        <div class="drawer-head">
          <h3>{{ customerDrawerMode === 'edit' ? '编辑客户' : '新增客户' }}</h3>
          <button class="secondary" type="button" @click="closeCustomerDrawer">关闭</button>
        </div>
        <label v-if="customerDrawerMode !== 'edit'" class="wide-field">
          <span>粘贴收件信息</span>
          <textarea v-model.trim="customerPaste" rows="4" placeholder="张三 13800138000 云南省普洱市思茅区咖啡路 88 号"></textarea>
        </label>
        <button v-if="customerDrawerMode !== 'edit'" class="secondary parse-button" type="button" @click="applyRecipientParse">地址解析</button>
        <div v-if="customerError" class="notice error">{{ customerError }}</div>
        <div v-if="customerNotice" class="notice ok">{{ customerNotice }}</div>
        <div class="drawer-grid">
          <label>
            <span>客户名</span>
            <input v-model.trim="customerForm.name" />
          </label>
          <label>
            <span>公司名称</span>
            <input v-model.trim="customerForm.company_name" placeholder="不填则默认客户名" />
          </label>
          <label>
            <span>公司电话</span>
            <input v-model.trim="customerForm.company_phone" />
          </label>
          <label>
            <span>联系人</span>
            <input v-model.trim="customerForm.contact" />
          </label>
          <label>
            <span>联系电话</span>
            <input v-model.trim="customerForm.phone" />
          </label>
          <label>
            <span>客户类型</span>
            <select v-model="customerForm.customer_type">
              <option v-for="item in customerTypeOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </label>
          <label>
            <span>来源</span>
            <select v-model.number="customerForm.default_source_id">
              <option v-for="item in sources" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label>
            <span>订单类型</span>
            <select v-model.number="customerForm.default_order_type_id">
              <option v-for="item in orderTypes" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label>
            <span>负责人</span>
            <select v-model.number="customerForm.responsible_employee_id">
              <option :value="0">选择员工</option>
              <option v-for="employee in activeEmployees" :key="employee.id" :value="employee.id">{{ employee.name }}</option>
            </select>
          </label>
          <label class="wide-field">
            <span>地址</span>
            <textarea v-model.trim="customerForm.address" rows="3"></textarea>
          </label>
          <label class="wide-field">
            <span>公司地址</span>
            <textarea v-model.trim="customerForm.company_address" rows="3"></textarea>
          </label>
          <label class="check-field">
            <input v-model="customerForm.portal_enabled" type="checkbox" />
            <span>开通客户门户/工作台</span>
          </label>
        </div>
        <div class="drawer-actions">
          <button class="primary" type="button" @click="saveCustomerFromDrawer" :disabled="customerSaving">
            {{ customerSaving ? '保存中' : (customerDrawerMode === 'edit' ? '保存客户信息' : '保存并选择') }}
          </button>
        </div>
      </aside>
    </div>

    <div v-if="beanListDrawerOpen" class="drawer-mask" @click.self="closeBeanListDrawer">
      <aside class="drawer bean-list-drawer">
        <div class="drawer-head">
          <h3>价格表选择</h3>
          <button class="secondary" type="button" @click="closeBeanListDrawer">关闭</button>
        </div>
        <p class="drawer-help">按商品分类选择已发布价格表。客户没有自定义版本时，系统使用同分类公共版本。</p>
        <div class="bean-list-picker-list">
          <label v-for="group in beanListVersionGroups" :key="group.key">
            <span>{{ group.label }}</span>
            <select
              v-if="group.options.length"
              :value="selectedBeanListPublicationIDs[group.key] || selectedBeanListVersionOptionByGroup(group)?.id || 0"
              @change="setBeanListVersion(group.key, $event.target.value)"
            >
              <option v-if="!group.autoSelect" :value="0">不使用该历史价格表</option>
              <option v-for="option in group.options" :key="option.id" :value="option.id">
                {{ beanListVersionLabel(option) }}
              </option>
            </select>
            <input v-else value="暂无已发布价格表" readonly />
            <small>{{ beanListDrawerHint(group) }}</small>
          </label>
        </div>
      </aside>
    </div>

    <div v-if="paymentVoucherPreviewOpen" class="voucher-preview-overlay" @click.self="paymentVoucherPreviewOpen = false">
      <div class="voucher-preview-dialog">
        <div class="drawer-head">
          <h3>{{ paymentVoucher?.filename || '收款凭证' }}</h3>
          <button class="secondary" type="button" @click="paymentVoucherPreviewOpen = false">关闭</button>
        </div>
        <img v-if="paymentVoucherImageURL && paymentVoucherIsImage" :src="paymentVoucherImageURL" alt="收款凭证" />
        <iframe v-else-if="paymentVoucherImageURL" :src="paymentVoucherImageURL" title="收款凭证"></iframe>
        <p v-else class="empty">暂无凭证预览</p>
      </div>
    </div>
    </template>
  </div>
</template>

<script setup>
import { convertedOrderRow, conversionSnapshot } from '../lib/spec-conversion.js'
function isFrozenConversion(row) { return ['converted_tier','bom_tier'].includes(row.price_selection_mode) && row.spec_conversion?.reprice === false && Number(form.edit_id) > 0 }

import { orderDeliveryMode, isNonCourierShipMethod, requiresCourierLogistics } from '../lib/order-shipping'
import { prepaymentPresets, prepaymentByRate } from '../lib/prepayment-presets.js'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { createOrderCatalogSession, orderCatalogKey, staffPriceTableGroups } from '../lib/order-catalog.js'
import { apiGet, apiSend } from '../api/client'
import FulfillmentOrderFees from '../components/FulfillmentOrderFees.vue'
import OrderPaymentSummary from '../components/OrderPaymentSummary.vue'
import { fulfillmentEditFamilies, fulfillmentSpecForRow, fulfillmentSpecRowPatch, fulfillmentEditCommand, fulfillmentFeeSummary, fulfillmentLinePreview, fulfillmentDraftGoodsCents } from '../lib/fulfillment-order-edit'
import { stockSupplySummary } from '../lib/customer-stock-entry'
import { isSampleOrderEdit, validManualOrderPrice, storedSampleSpecKey, preservesSampleSpec, sampleStoredItemIdentity, orderEditRecipientFields, isFreeSamplePreview, validSampleReceiptGoods } from '../lib/sample-order-edit.js'
import { resetCustomerContinuation } from '../lib/customer-workspace'
import { clearFormDraft, FORM_DRAFT_SCOPES, readFormDraft, saveFormDraft } from '../lib/form-draft-cache'
import {
  activeBeanListPublicationIDsByType,
  activeCustomerOwnedBeanListPublicationIDsByType,
  CUSTOM_SPEC_VALUE,
  beanListVersionGroupIdentity,
  beanListVersionGroupForPublicationID,
  beanListVersionOptionGroups,
  beanListVersionOptionForGroup,
  beanListVersionOptionForProductGroups,
  beanListVersionOptionsForCustomer,
  filterBeanListVersionOptionsToCurrentTypes,
  buildOrderPayload,
  closeOrderProductDropdowns,
  defaultDripSalesUnitSpec,
  defaultWholesaleSpec,
  defaultStatusID,
  dripSalesUnitSpec,
  dripTierPriceRows,
  filterProductsForCustomer,
  filterOptions,
  isOrderProductFamily,
  isOrderTierActive,
  lineTotal,
  latestBeanListVersionOption,
  needsTrailingBlankOrderLine,
  normalizeSpecG,
  normalizeOrderProductFamilies,
  orderFamilyDefaultSpec,
  orderFamilySearchScopeForPublication,
  orderFamilySpecOptions,
  orderFamilySpecProduct,
  orderFamilyHydratedSpecRowPatch,
  orderFamilySpecRowPatch,
  orderFamilySpecForStoredItem,
  orderProductFamilyForStoredItem,
  orderQuoteSourceSummary,
  orderFamilySpecsForPublication,
  orderLegacyProductForPublication,
  orderProductPublicationMode,
  orderProductFamilyOptions,
  orderProductFamilyForContext,
  orderProductFamilyIdentity,
  orderProductClassificationFilterOptions,
  orderProductClassificationForFamily,
  orderProductFamiliesForClassification,
  orderProductKindFilterOptions,
  orderSpecSelectionAfterPublicationChange,
  orderRowPriceUnit,
  orderReceiptMethodOptions,
  orderTotalPreview,
  productBeanListType,
  productKindBadgeClass,
  productKindLabel,
  requiresOrderPaymentMethod,
  requiresOrderPaymentReceipt,
  resolveWholesaleTierPrice,
  retailPackagePrice,
  retailSpecOptions,
  rowUsesStaleBeanListPublication,
  shouldKeepFrozenOrderPublication,
  sortProductsByCustomerUsage,
  syncDripTierPrice,
  toInt,
  toNumber,
  wholesaleTierPriceRows,
  wholesaleSpecOptions,
} from '../lib/order-entry'
import { parseRecipientText } from '../lib/customer-recipient'
import OrderFreightEstimate from '../components/OrderFreightEstimate.vue'
import {emptyFreightState,freightStateFromOrder,freightAmount} from '../lib/order-freight'
import CustomerRecipientPicker from '../components/CustomerRecipientPicker.vue'
import { customerStockWarehouses, isCustomerStockOrder } from '../lib/customer-stock-entry'
import { businessGroupFeatureSelectionIDs, businessGroupRowsForFeatureSelection } from '../lib/business-grouping'
import { buildProductCatalogTemplatePriceListTypeOptions } from '../lib/product-price-list-types'
import { customerTypeLabel, customerTypeOptions, normalizeCustomerType } from '../lib/customer-types'
import { dripUnitOptions, isDripProduct } from '../lib/drip-product'
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'

const props = defineProps({
  editId: { type: [Number, String], default: 0 },
  copyId: { type: [Number, String], default: 0 },
  embedded: { type: Boolean, default: false },
  recipientTarget: { type: String, default: '' },
  workflowTarget: { type: String, default: '' },
  feesInSummary: { type: Boolean, default: false },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
  fulfillmentMode: { type: Boolean, default: false },
  customerPortal: { type: Boolean, default: false },
  portalService: { type: String, default: 'direct_ship' },
  recipientInfo: { type: Object, default: null },
  deliveryPrefill: { type: Object, default: null },
})

const emit = defineEmits(['close', 'saved', 'editability', 'fee-summary', 'delivery-mode-change'])
const fulfillmentRecipientPaste=ref('')
function parseFulfillmentRecipient(){const result=parseRecipientText(fulfillmentRecipientPaste.value);if(result.recipient_name)form.receiver_name=result.recipient_name;if(result.phone)form.receiver_phone=result.phone;if(result.address)form.receiver_address=result.address}
const ORDER_ENTRY_DRAFT_SCOPE = FORM_DRAFT_SCOPES.orderEntry
let orderEntryDraftDisabled = false

const loading = ref(false)
const sampleOrderEditing = ref(false)
const fulfillmentEditData = ref(null)
const effectiveCopyID = ref(0)
const copyMode = computed(() => Number(props.copyId || effectiveCopyID.value || 0) > 0)
const fulfillmentOrderEditing = computed(() => !props.customerPortal && !copyMode.value
  && Number(form.edit_id) > 0 && fulfillmentEditData.value?.portal_service_code === 'direct_ship'
  && ['stock', 'on_demand'].includes(fulfillmentEditData.value?.fulfillment_mode))
const fulfillmentEditContext = ref(null)
const fulfillmentPreview = ref(null)
const fulfillmentPreviewLoading = ref(false)
const fulfillmentServiceAdjustment = reactive({ enabled: false, amount: '0.00', reason: '' })
const fulfillmentManualDestination = ref(false)
let fulfillmentPreviewVersion = 0
const fulfillmentFamilies = computed(() => fulfillmentEditFamilies(fulfillmentEditContext.value?.catalog, fulfillmentEditContext.value?.fulfillment_mode))
const fulfillmentCurrentFees = computed(() => fulfillmentPreview.value?.fees || fulfillmentEditContext.value?.fees || {})
const fulfillmentGoodsCents = computed(() => fulfillmentDraftGoodsCents(fulfillmentEditContext.value || {}, rows.value))
const loadedEditorOrderID = ref(0)
const requestedEditorOrderID = computed(() => Number(props.copyId || props.editId || (typeof window === 'undefined' ? 0 : new URL(window.location.href).searchParams.get('copy_id') || new URL(window.location.href).searchParams.get('edit_id')) || 0))
const editorSnapshotPending = computed(() => requestedEditorOrderID.value > 0 && loadedEditorOrderID.value !== requestedEditorOrderID.value)
let editorLoadVersion = 0
let editorAlive = true
const editBlockReason = ref('')
const saving = ref(false)
const error = ref('')
const ok = ref('')
const stockBatchNotice = ref('')
const customers = ref([])
const sources = ref([])
const shipStatuses = ref([])
const payStatuses = ref([])
const orderTypes = ref([])
const products = ref([])
const productFamilies = ref([])
const employees = ref([])
const logisticsCompanies = ref([])
const beanListVersionOptions = ref([])
const customerPublicUsages = ref([])
const customerProductUsages = ref([])
const orderProductCatalogGroups = ref([])
const orderProductCatalogAssignments = ref([])
const orderProductCatalogFeatureSelection = ref({ feature_key: 'product_catalog', group_template_ids: [] })
const orderProductCatalogFeatureSelectionLoaded = ref(false)
const selectedBeanListPublicationIDs = reactive({})
const staffCatalog = reactive({})
const staffCatalogSession = createOrderCatalogSession({ state: staffCatalog, load: (customerID, ids) => apiGet(`/api/order/catalog?customer_id=${customerID}&selected_price_table_ids=${ids.join(',')}`) })
function selectedStaffTableIDs() { return [...new Set(Object.values(selectedBeanListPublicationIDs).map(Number).filter(id => id > 0))] }
function preservesStoredOrderPrice(row) {
  return isFrozenConversion(row) || preservesSampleSpec(row, sampleOrderEditing.value)
}
function staffCatalogRequestTableIDs() {
  const historicalIDs = new Set()
  const liveIDs = new Set()
  for (const row of rows.value) {
    if (Number(row.product_id || row.parent_product_id || 0) <= 0) continue
    const target = preservesStoredOrderPrice(row) ? historicalIDs : liveIDs
    target.add(Number(row.bean_list_publication_id || 0))
  }
  const currentIDs = new Set(Number(staffCatalog.customerID) === Number(form.customer_id) && staffCatalog.key
    ? (staffCatalog.options || []).map(table => Number(table.id)) : [])
  return selectedStaffTableIDs().filter(id => !historicalIDs.has(id) || liveIDs.has(id) || currentIDs.has(id))
}
async function refreshStaffOrderCatalog() {
  if (props.customerPortal || fulfillmentOrderEditing.value) return true
  const customerID = Number(form.customer_id || 0)
  const previousKey = staffCatalog.key
  const data = await staffCatalogSession.refresh(customerID, staffCatalogRequestTableIDs())
  if (!data) { if (staffCatalog.error) error.value = staffCatalog.error; return false }
  for (const key of Object.keys(selectedBeanListPublicationIDs)) delete selectedBeanListPublicationIDs[key]
  for (const group of staffPriceTableGroups(data.price_table_options || [], data.selected_price_table_ids || [])) {
    const selected = group.options.find(row => data.selected_price_table_ids?.includes(Number(row.id)))
    if (selected) selectedBeanListPublicationIDs[group.key] = Number(selected.id)
  }
  syncBeanListVersionForCustomer()
  if (!data.catalog_updated) {
    if (previousKey && previousKey !== staffCatalog.key) syncRowsForType({ priceListChanged: true })
    else repriceHydratedRows()
  }
  if (data.catalog_updated) {
    ok.value = '价格表已更新，请重新选择规格并核对原有商品价格。'
    for (const row of rows.value) if (Number(row.product_id) > 0 && !preservesStoredOrderPrice(row)) { row.spec_invalid_message = '价格表已更新，请重新选择规格并核对价格'; row.price_missing = true }
  }
  return true
}
function staffCatalogFamilies() { return normalizeOrderProductFamilies(staffCatalog.families || []) }
function currentOrderProductFamilies() {
  if (fulfillmentOrderEditing.value) return fulfillmentFamilies.value
  if (props.customerPortal || Number(staffCatalog.customerID) !== Number(form.customer_id)) return productFamilies.value
  const current = staffCatalogFamilies()
  const keys = new Set(current.map(orderProductFamilyIdentity))
  return [...current, ...productFamilies.value.filter(row => !keys.has(orderProductFamilyIdentity(row)))]
}

const rows = ref([newRow()])
const orderFreight=ref(null)
const freightEstimator=ref(null)
const freightItems=computed(()=>rows.value.filter(r=>Number(r.product_id)>0&&Number(r.qty)>0).map(r=>({product_id:Number(r.product_id),bom_spec_id:Number(r.bom_spec_id||0),bom_variant_id:Number(r.bom_variant_id||0),qty:Number(r.qty)})))

const hasUnpricedPublishedRow = computed(() => rows.value.some(rowHasBlockingPrice))
const paymentVoucher = ref(null)
const paymentVoucherFile = ref(null)
const uploadingVoucher = ref(false)
const paymentVoucherCollapsed = ref(false)
const paymentVoucherPreviewOpen = ref(false)
const customerQuery = ref('')
const customerOpen = ref(false)
const customerDrawerOpen = ref(false)
const customerDrawerMode = ref('create')
const beanListDrawerOpen = ref(false)
const customerSaving = ref(false)
const customerError = ref('')
const customerNotice = ref('')
const customerPaste = ref('')
const customerForm = reactive(emptyCustomerForm())
const fieldErrors = reactive({})
const backfillMode = ref(false)
const deliveryWarehouses = ref([])
const deliveryWarehousesLoading = ref(false)
const deliveryWarehouseError = ref('')
const deliveryWarehouseOptions = computed(() => customerStockWarehouses(deliveryWarehouses.value, form.customer_id))
let deliveryWarehouseLoadVersion = 0
function applyAddressBookRecipient(snapshot) { Object.assign(form, snapshot); saveOrderEntryDraft() }
async function loadDeliveryWarehouses() {
  const version = ++deliveryWarehouseLoadVersion
  deliveryWarehouses.value = []
  deliveryWarehouseError.value = ''
  if (props.customerPortal || Number(form.customer_id) <= 0) return
  deliveryWarehousesLoading.value = true
  try { const data = await apiGet(`/api/stock/warehouses?customer_id=${Number(form.customer_id)}`); if (version === deliveryWarehouseLoadVersion) deliveryWarehouses.value = data.rows || [] }
  catch (cause) { if (version === deliveryWarehouseLoadVersion) deliveryWarehouseError.value = cause.message || '客户仓库加载失败' }
  finally { if (version === deliveryWarehouseLoadVersion) deliveryWarehousesLoading.value = false }
}

const form = reactive({
  edit_id: 0,
 edit_revision: "",
  document_date: '',
  order_date: '',
  customer_id: 0,
  delivery_mode: 'ship',
  delivery_warehouse_code: '',
  source_id: 0,
  order_type_id: 0,
  pay_status_id: 0,
  payment_method: '',
  prepayment_amount: '0',
  ship_status_id: 0,
  ship_method: '',
  ship_tracking_no: '',
  logistics_company_id: 0,
  logistics_product_id: 0,
  payment_goods_amount: '',
  payment_shipping_amount: '',
  payment_voucher_asset_id: 0,
  bean_list_publication_id: 0,
  commercial_bean_list_publication_id: 0,
  green_bean_list_publication_id: 0,
  drip_bean_list_publication_id: 0,
  notes: '',
  freight_state: emptyFreightState(),
  shipping_amount: '',
  discount_amount: '',
  round_to_int: false,
  express_fee: '',
  outsource_material_fee: '',
  outsource_roast_fee: '',
  outsource_packaging_fee: '',
  outsource_manual_fee: '',
  outsource_tax_fee: '',
  outsource_other_fee: '',
  receiver_name: '',
  receiver_phone: '',
  receiver_address: '',
  receiver_company: '',
  settlement_reason: '',
})

function newRow() {
  return {
    key: `${Date.now()}-${Math.random()}`,
    item_id: 0,
    product_query: '',
    product_open: false,
    product_kind_filter: '',
    product_family_key: '',
    parent_product_id: 0,
    parent_product_name: '',
    product_id: 0,
    product_name: '',
    product_code: '',
    product_record_name: '',
    customer_product_alias_id: 0,
    customer_product_display_name: '',
    customer_item_code: '',
    brand_name: '',
    product_kind: 'roasted_bean',
    product_type_category_id: 0,
    product_type_name: '',
    tier_id: 'auto',
    price_selection_mode: 'auto',
    selected_price_row_key: '',
    quantity_basis: '',
    price_source_json: '',
    bean_list_publication_id: 0,
    bean_list_version_no: '',
    bean_list_version_tip_open: false,
    unit_price: '',
    price_unit: '',
    price_unit_suffix: '',
    price_unit_g: 0,
    tier_price_label: '',
    tier_below_min: false,
    price_missing: false,
    manual_price: false,
    spec_mode: '',
    spec_source: '',
    spec_label: '',
    spec_g: 0,
    historical_spec_readonly: false,
    spec_invalid_message: '',
    custom_spec_g: '',
    sales_unit: '',
    unit_bag_count: 0,
    unit_bean_g: '',
    qty: 1,
    unit: '件',
    item_note: '',
    discount_type: '',
    discount_value: '',
  }
}

function emptyCustomerForm() {
  return {
    id: 0,
    name: '',
    raw_name: '',
    customer_type: 'retail',
    company_name: '',
    company_address: '',
    company_phone: '',
    contact: '',
    phone: '',
    address: '',
    default_source_id: 0,
    default_order_type_id: 0,
    responsible_employee_id: 0,
    active: true,
    portal_enabled: false,
  }
}

function currentUrlNumberParam(name) {
  return Number(new URL(window.location.href).searchParams.get(name) || 0)
}

function orderEntryDraftKey() {
  const editID = Number(props.editId || currentUrlNumberParam('edit_id') || 0)
  const copyID = Number(props.copyId || currentUrlNumberParam('copy_id') || 0)
  if (props.embedded || editID || copyID) return ''
  const workspace = props.workspaceMode || 'factory'
  const customerID = Number(props.customerContextId || 0)
  return `${ORDER_ENTRY_DRAFT_SCOPE}:${workspace}:${customerID || 'all'}:${props.customerPortal ? props.portalService : 'staff'}:new`
}

function closeTransientRowMenus(row) {
  return { ...row, product_open: false }
}

function saveOrderEntryDraft() {
  if (orderEntryDraftDisabled) return
  const key = orderEntryDraftKey()
  if (!key) return
  saveFormDraft(key, {
    form: { ...form },
    selectedPriceTableIDs: { ...selectedBeanListPublicationIDs },
    rows: rows.value.map(closeTransientRowMenus),
    customerQuery: customerQuery.value,
    customerDrawerOpen: customerDrawerOpen.value,
    customerPaste: customerPaste.value,
    customerForm: { ...customerForm },
    backfillMode: backfillMode.value,
  })
}

function restoreOrderEntryDraft() {
  const draft = readFormDraft(orderEntryDraftKey())
  if (!draft) return false
  Object.assign(form, draft.form || {})
  for (const key of Object.keys(selectedBeanListPublicationIDs)) delete selectedBeanListPublicationIDs[key]
  Object.assign(selectedBeanListPublicationIDs, draft.selectedPriceTableIDs || {})
  rows.value = Array.isArray(draft.rows) && draft.rows.length
    ? draft.rows.map((row) => ({ ...newRow(), ...row, product_open: false }))
    : [newRow()]
  customerQuery.value = draft.customerQuery || ''
  customerDrawerOpen.value = Boolean(draft.customerDrawerOpen)
  customerPaste.value = draft.customerPaste || ''
  Object.assign(customerForm, emptyCustomerForm(), draft.customerForm || {})
  backfillMode.value = Boolean(draft.backfillMode)
  return true
}

function selectedOrderType() {
  return orderTypes.value.find((item) => Number(item.id) === Number(form.order_type_id)) || null
}

const retailOrder = computed(() => {
  const name = selectedOrderType()?.name || ''
  return name.includes('零售') || name.toLowerCase().includes('retail')
})

const itemsTotal = computed(() => rows.value.reduce((sum, row) => sum + rowTotal(row), 0))
const orderTotalPreviewValue = computed(() => orderTotalPreview({
  itemsTotal: fulfillmentOrderEditing.value ? fulfillmentGoodsCents.value / 100 : itemsTotal.value,
  shippingAmount: fulfillmentOrderEditing.value ? (fulfillmentFeeSummary(fulfillmentCurrentFees.value).service + fulfillmentFeeSummary(fulfillmentCurrentFees.value).shipping) / 100 : form.shipping_amount,
  discountAmount: fulfillmentOrderEditing.value ? 0 : form.discount_amount,
  roundToInt: fulfillmentOrderEditing.value ? false : form.round_to_int,
}))
const orderTotalHintText = computed(() => fulfillmentOrderEditing.value ? '商品货款＋代发服务费＋本次运费应收，预估运费单独展示' : `货款 ${money(orderTotalPreviewValue.value.goodsAmount)} · 物流 ${money(orderTotalPreviewValue.value.logisticsAmount)}`)
const filteredCustomers = computed(() => filterOptions(customers.value, customerQuery.value).slice(0, 20))
const freeSampleSettlement = computed(() => isFreeSamplePreview(sampleOrderEditing.value, rows.value, form, orderTotalPreviewValue.value.grandTotal))
const zeroGoodsSample = computed(() => sampleOrderEditing.value && itemsTotal.value === 0)
const paymentMethodRequired = computed(() => !freeSampleSettlement.value && !props.customerPortal && requiresOrderPaymentMethod(form, payStatuses.value))
const prepaymentRate = ref(0)
function selectPrepaymentStatus() {
  if (Number(form.prepayment_amount) <= 0) return
  const status = payStatuses.value.find(item => String(item.name).includes('预付款'))
  if (status) form.pay_status_id = Number(status.id)
}
function applyPrepaymentPreset(rate) {
  prepaymentRate.value = rate
  form.prepayment_amount = prepaymentByRate(orderTotalPreviewValue.value.grandTotal, rate)
  selectPrepaymentStatus()
}
function onManualPrepayment() {
  prepaymentRate.value = 0
  selectPrepaymentStatus()
}
watch(() => orderTotalPreviewValue.value.grandTotal, () => {
  if (prepaymentRate.value) applyPrepaymentPreset(prepaymentRate.value)
})
const selectedPayStatusName = computed(() => optionName(payStatuses.value, form.pay_status_id))
const selectedShipStatusName = computed(() => optionName(shipStatuses.value, form.ship_status_id))
const paymentReceiptVisible = computed(() => {
  if (props.customerPortal || freeSampleSettlement.value) return false
  const name = selectedPayStatusName.value
  return name.includes('已收款') || name.includes('已付款') || name.includes('已支付')
})
const paymentReceiptRequired = computed(() => !freeSampleSettlement.value && !props.customerPortal && requiresOrderPaymentReceipt(form, payStatuses.value))
const paymentGoodsAmountSuggestion = computed(() => money(itemsTotal.value))
const paymentShippingAmountSuggestion = computed(() => money(toNumber(form.shipping_amount)))
const showPaymentGoodsAmountSuggestion = computed(() => paymentReceiptVisible.value && itemsTotal.value > 0)
const showPaymentShippingAmountSuggestion = computed(() => paymentReceiptVisible.value)
const logisticsRequired = computed(() => form.delivery_mode !== 'customer_stock' && !props.customerPortal && requiresCourierLogistics(form.ship_method, selectedShipStatusName.value))
const deliveryMode = computed({
  get: () => orderDeliveryMode(form.ship_method),
  set: (value) => {
    form.ship_method = value === 'express' ? '' : value
    if (value !== 'express') {
      form.ship_tracking_no = ''
      form.logistics_company_id = 0
      form.logistics_product_id = 0
    }
  },
})
const quoteSourceSummary = computed(() => orderQuoteSourceSummary(rows.value.filter(row => Number(row.product_id) > 0), beanListVersionOptions.value))
const selectedLogisticsProducts = computed(() => {
  const company = logisticsCompanies.value.find((item) => Number(item.id || 0) === Number(form.logistics_company_id || 0))
  return (company?.products || []).filter((item) => item.active !== false)
})
const canUseBackfillMode = computed(() => (!props.embedded || props.customerPortal) && !form.edit_id && !copyMode.value)
const activeEmployees = computed(() => employees.value.filter((employee) => employee.active !== false))
const selectedCustomer = computed(() => customers.value.find((item) => Number(item.id || 0) === Number(form.customer_id || 0)) || null)
const selectedCustomerResponsibleLabel = computed(() => selectedCustomer.value?.responsible_employee_name || '')
const selectedCustomerProfileSummary = computed(() => [
  {
    key: 'customer_type',
    label: '客户类型',
    value: customerTypeLabel(selectedCustomer.value?.customer_type) || '未设置',
    missing: !customerTypeLabel(selectedCustomer.value?.customer_type),
  },
  {
    key: 'source',
    label: '来源',
    value: optionName(sources.value, selectedCustomer.value?.default_source_id) || '未设置',
    missing: !Number(selectedCustomer.value?.default_source_id || 0),
  },
  {
    key: 'order_type',
    label: '订单类型',
    value: optionName(orderTypes.value, selectedCustomer.value?.default_order_type_id) || '未设置',
    missing: !Number(selectedCustomer.value?.default_order_type_id || 0),
  },
])
const orderBeanListTypes = [
  { type: 'commercial', label: '熟豆豆单' },
  { type: 'green', label: '生豆豆单' },
  { type: 'drip', label: '挂耳豆单' },
]
const orderCurrentProductCatalogGroupTemplates = computed(() => businessGroupRowsForFeatureSelection(
  orderProductCatalogGroups.value,
  businessGroupFeatureSelectionIDs(orderProductCatalogFeatureSelection.value),
))
const orderCurrentProductPriceListTypes = computed(() => {
  if (!orderProductCatalogFeatureSelectionLoaded.value) return []
  return buildProductCatalogTemplatePriceListTypeOptions(products.value, {
    templates: orderCurrentProductCatalogGroupTemplates.value,
    assignments: orderProductCatalogAssignments.value,
  })
})
const customerBeanListVersionOptions = computed(() => {
  if (!props.customerPortal && Number(staffCatalog.customerID) === Number(form.customer_id) && staffCatalog.key) return staffCatalog.options
  const rows = beanListVersionOptionsForCustomer(beanListVersionOptions.value, form.customer_id)
  return filterBeanListVersionOptionsToCurrentTypes(rows, orderCurrentProductPriceListTypes.value)
})
const beanListVersionGroups = computed(() => props.customerPortal
  ? beanListVersionOptionGroups(customerBeanListVersionOptions.value)
  : staffPriceTableGroups(customerBeanListVersionOptions.value, selectedStaffTableIDs()))
const canOpenBeanListDrawer = computed(() => {
  return Number(form.customer_id || 0) > 0 || customerBeanListVersionOptions.value.length > 0
})
const selectedBeanListSummaryItems = computed(() => beanListVersionGroups.value
  .map((group) => {
    const selected = selectedBeanListVersionOptionByGroup(group)
    return {
      type: group.key,
      label: group.label,
      versionLabel: selected ? beanListVersionLabel(selected) : '暂无',
    }
  })
  .filter((item) => item.versionLabel !== '暂无'))

function selectedCustomerMissingProfileLabels() {
  if (!selectedCustomer.value) return []
  return selectedCustomerProfileSummary.value.filter((item) => item.missing).map((item) => item.label)
}
function preferredProductFamily(candidates, reference = {}) {
  return orderProductFamilyForContext(candidates, {
    customer_id: form.customer_id,
    ...reference,
  })
}

function productFamilyBySKU(id, reference = {}) {
  const productID = Number(id || 0)
  if (productID <= 0) return null
  const candidates = currentOrderProductFamilies().filter((family) => (family.specs || [])
    .some((spec) => Number(spec.sku_id || 0) === productID))
  return preferredProductFamily(candidates, reference)
}

function productByID(id, reference = {}) {
  const productID = Number(id || 0)
  const family = productFamilyBySKU(productID, reference)
  if (family) {
    const spec = (family.specs || []).find((item) => Number(item.sku_id || 0) === productID)
    if (spec) return orderFamilySpecProduct(family, spec, 0)
  }
  return products.value.find((item) => Number(item.id) === productID) || null
}

function productFamilyByParentID(id, reference = {}) {
  const parentID = Number(id || 0)
  if (parentID <= 0) return null
  const candidates = currentOrderProductFamilies().filter((item) => Number(item.parent_product_id || item.id || 0) === parentID)
  return preferredProductFamily(candidates, reference)
}

function productFamilyForRow(row) {
  if (row?.__order_legacy_price_product || row?.spec_source === 'legacy_price_list') return null
  return productFamilyByParentID(row?.parent_product_id, row)
    || productFamilyBySKU(row?.product_id, row)
    || null
}

function isConcretePriceListRow(row) {
  if (row?.__order_legacy_price_product || row?.spec_source === 'legacy_price_list') return false
  const family = productFamilyForRow(row)
  return row?.spec_source === 'price_list_sku'
    || isOrderProductFamily(family)
}

function productForRow(row) {
  if (fulfillmentOrderEditing.value) {
    const { family, spec } = fulfillmentSpecForRow(fulfillmentFamilies.value, row)
    return spec ? { ...family, ...spec, tiers: [] } : null
  }
  if (row?.__order_legacy_price_product || row?.spec_source === 'legacy_price_list') {
    return products.value.find((item) => Number(item.id || 0) === Number(row?.product_id || 0)) || null
  }
  const family = productFamilyForRow(row)
  if (!family || !isConcretePriceListRow(row)) return productByID(row?.product_id, row)
  const selected = selectedBeanListVersionOptionForProduct(family)
  const publicationID = Number(selected?.id || row?.bean_list_publication_id || 0)
  const spec = orderFamilySpecForStoredItem(family, row, publicationID)
  if (!spec) return null
  return orderFamilySpecProduct(family, spec, publicationID)
}

function isDripRow(row) {
  return row?.product_kind === 'drip_bag' || isDripProduct(productForRow(row))
}

function optionName(options, id) {
  return (options || []).find((item) => Number(item.id) === Number(id))?.name || ''
}

function defaultOptionID(options, labels) {
  const row = (options || []).find((item) => labels.some((label) => String(item.name || '').includes(label)))
  return Number(row?.id || options?.[0]?.id || 0)
}

function defaultSourceID() {
  return defaultOptionID(sources.value, ['微信', 'Wechat', 'wechat'])
}

function defaultOrderTypeID() {
  return defaultOptionID(orderTypes.value, ['批发', 'Wholesale', 'wholesale'])
}

function syncLogisticsProduct() {
  const products = selectedLogisticsProducts.value
  if (!products.some((item) => Number(item.id || 0) === Number(form.logistics_product_id || 0))) {
    form.logistics_product_id = Number(products[0]?.id || 0)
  }
}

function ensureLogisticsDefaults() {
  if (!logisticsRequired.value) return
  if (!form.logistics_company_id && logisticsCompanies.value.length) {
    form.logistics_company_id = Number(logisticsCompanies.value[0]?.id || 0)
  }
  syncLogisticsProduct()
}

function ensurePaymentDefaults() {
  if (!paymentReceiptRequired.value) return
  if (String(form.payment_shipping_amount ?? '').trim() === '') {
    form.payment_shipping_amount = money(toNumber(form.shipping_amount))
  }
}

function applyPaymentGoodsAmountSuggestion() {
  form.payment_goods_amount = paymentGoodsAmountSuggestion.value
  clearFieldErrorIfValid('payment_goods_amount')
}

function applyPaymentShippingAmountSuggestion() {
  form.payment_shipping_amount = paymentShippingAmountSuggestion.value
  clearFieldErrorIfValid('payment_shipping_amount')
}

function hasFieldError(fieldKey) {
  return Boolean(fieldErrors[fieldKey])
}

function clearFieldError(fieldKey) {
  delete fieldErrors[fieldKey]
}

function clearAllFieldErrors() {
  Object.keys(fieldErrors).forEach((fieldKey) => clearFieldError(fieldKey))
}

function hasValidProductLine() {
  return rows.value.some((row) => Number(row.product_id || 0) > 0)
}

function fieldIsValid(fieldKey) {
  if (fieldKey === 'customer_id') return Number(form.customer_id || 0) > 0
  if (fieldKey === 'payment_method') return !paymentMethodRequired.value || String(form.payment_method || '').trim() !== ''
  if (fieldKey === 'logistics_company_id') return !logisticsRequired.value || Number(form.logistics_company_id || 0) > 0
  if (fieldKey === 'logistics_product_id') return !logisticsRequired.value || Number(form.logistics_product_id || 0) > 0
  if (fieldKey === 'payment_goods_amount') return !paymentReceiptRequired.value || validSampleReceiptGoods(form.payment_goods_amount, zeroGoodsSample.value, form.payment_shipping_amount)
  if (fieldKey === 'payment_shipping_amount') return !paymentReceiptRequired.value || String(form.payment_shipping_amount ?? '').trim() !== ''
  if (fieldKey === 'payment_voucher_asset_id') return !paymentReceiptRequired.value || Number(form.payment_voucher_asset_id || 0) > 0
  if (fieldKey === 'product_items') return hasValidProductLine() && !hasUnpricedPublishedRow.value
  return false
}

function clearFieldErrorIfValid(fieldKey) {
  if (fieldIsValid(fieldKey)) clearFieldError(fieldKey)
}

function focusErrorField(fieldKey) {
  if (!fieldKey || typeof document === 'undefined') return
  nextTick(() => {
    const target = document.querySelector(`[data-error-field="${fieldKey}"]`)
    if (!target) return
    target.scrollIntoView({ behavior: 'smooth', block: 'center' })
    const input = target.matches?.('input,select,textarea,button')
      ? target
      : target.querySelector?.('input,select,textarea,button')
    input?.focus?.({ preventScroll: true })
  })
}

function raiseSaveError(message, fieldKey = '') {
  error.value = message
  if (fieldKey) {
    fieldErrors[fieldKey] = message
    focusErrorField(fieldKey)
  }
}

async function handlePaymentVoucherFile(event) {
  const file = event?.target?.files?.[0]
  paymentVoucherFile.value = file || null
  if (!file) return
  await uploadPaymentVoucher()
}

async function uploadPaymentVoucher() {
  if (!paymentVoucherFile.value) throw new Error('请选择收款凭证')
  uploadingVoucher.value = true
  try {
    const body = new FormData()
    body.append('file', paymentVoucherFile.value)
    const data = await apiSend('/api/order/payment-vouchers', { body })
    paymentVoucher.value = data.asset || null
    form.payment_voucher_asset_id = Number(data.asset?.id || 0)
    paymentVoucherCollapsed.value = Boolean(paymentVoucher.value)
  } finally {
    uploadingVoucher.value = false
  }
}

const paymentVoucherImageURL = computed(() => {
  const voucher = paymentVoucher.value || {}
  if (voucher.url) return voucher.url
  if (voucher.object_key) return `/assets/${voucher.object_key}`
  return ''
})

const paymentVoucherIsImage = computed(() => String(paymentVoucher.value?.content_type || '').startsWith('image/'))

function openPaymentVoucherPreview() {
  if (!paymentVoucher.value) return
  paymentVoucherPreviewOpen.value = true
}

function chooseCustomer(item) {
  form.customer_id = Number(item.id || 0)
  customerQuery.value = item.name || ''
  customerOpen.value = false
  syncOrderHeaderFromCustomer(item, { syncRows: false })
  syncBeanListVersionForCustomer({ force: true })
  syncRowsForType({ priceListChanged: true })
  notifyWorkspaceCustomerChanged(form.customer_id)
}

function applyRecipientHistory(event) {
  const key = event.target.value
  if (!key || !props.recipientInfo?.historyOptions) return
  const found = props.recipientInfo.historyOptions.find(h => h.key === key)
  if (!found) return
  form.receiver_name = found.name || ''
  form.receiver_phone = found.phone || ''
  form.receiver_address = found.address || ''
}

function syncOrderHeaderFromCustomer(customer = selectedCustomer.value, options = {}) {
  const nextSourceID = Number(customer?.default_source_id || 0)
  const nextOrderTypeID = Number(customer?.default_order_type_id || 0)
  const orderTypeChanged = Number(form.order_type_id || 0) !== nextOrderTypeID
  form.source_id = nextSourceID
  form.order_type_id = nextOrderTypeID
  if (orderTypeChanged && options.syncRows !== false) syncRowsForType()
}

function orderLinePriceTableLabel(row) {
  let name = ''
  try { name = JSON.parse(row.price_source_json || '{}').price_table_name || '' } catch {}
  if (!name) name = beanListVersionOptions.value.find(table => Number(table.id) === Number(row.bean_list_publication_id))?.table_name || '价格表'
  return `${name} · ${row.bean_list_version_no || ''}`
}

function beanListVersionLabel(item) {
  if (!item) return ''
  const owner = item.is_customer_owned ? '客户价格表' : '公共价格表'
  const version = item.version_no || item.label || `#${item.id}`
  const time = item.published_at ? ` · ${item.published_at}` : ''
  return `${item.table_name || owner} · ${version}${item.is_default ? ' · 默认' : ''}${time}`
}

function beanListVersionField(listType) {
  if (listType === 'retail') return 'bean_list_publication_id'
  if (listType === 'green') return 'green_bean_list_publication_id'
  if (listType === 'drip') return 'drip_bean_list_publication_id'
  return 'commercial_bean_list_publication_id'
}

function orderBeanListTypeForProductKind(productKind) {
  return productBeanListType({ product_kind: productKind })
}

function currentOrderBeanListTypeForProductKind(productKind) {
  if (String(productKind || '').trim() === 'drip_bag') return 'drip'
  if (props.customerPortal && retailOrder.value) return 'retail'
  return orderBeanListTypeForProductKind(productKind)
}

function currentPriceListTypeForProduct(productOrRow) {
  if (productOrRow?.__order_concrete_price_family || isConcretePriceListRow(productOrRow)) {
    if (props.customerPortal && retailOrder.value && productBeanListType(productOrRow) === 'commercial') return 'retail'
    return productBeanListType(productOrRow)
  }
  return currentOrderBeanListTypeForProductKind(productOrRow?.product_kind)
}

function customerBeanListVersionOptionsByType(listType) {
  return customerBeanListVersionOptions.value.filter((item) => String(item.list_type || 'commercial') === listType)
}

function beanListGroupKeyForProduct(productOrRow) {
  return beanListVersionGroupForProduct(productOrRow)?.key
    || beanListVersionGroupIdentity({
      ...productOrRow,
      list_type: currentPriceListTypeForProduct(productOrRow),
    }).key
}

function selectedBeanListPublicationIDsByType() {
  return activeBeanListPublicationIDsByType(beanListVersionGroups.value, selectedBeanListPublicationIDs)
}

function customerOwnedBeanListPublicationIDsByType() {
  return activeCustomerOwnedBeanListPublicationIDsByType(
    beanListVersionGroups.value,
    selectedBeanListPublicationIDs,
    form.customer_id,
  )
}

function showBeanListVersionPickerByType(listType) {
  const rows = customerBeanListVersionOptionsByType(listType)
  return rows.length > 1 || rows.some((item) => item.is_customer_owned)
}

function selectedBeanListVersionOptionByType(listType) {
  const rows = customerBeanListVersionOptionsByType(listType)
  if (!rows.length) return null
  const selected = Number(form[beanListVersionField(listType)] || 0)
  return rows.find((item) => Number(item.id) === selected)
    || rows.find((item) => item.is_default)
    || rows[0]
}

function selectedBeanListVersionOptionByGroup(group) {
  return beanListVersionOptionForGroup(group, selectedBeanListPublicationIDs[group?.key])
}

function productPublicationIDs(product = {}) {
  const ids = new Set()
  const currentID = Number(product?.bean_list_publication_id || 0)
  if (currentID > 0) ids.add(currentID)
  for (const tier of product?.tiers || []) {
    const id = Number(tier?.publication_id || 0)
    if (id > 0) ids.add(id)
  }
  return ids
}

function beanListVersionGroupForProduct(product = {}) {
  const publicationIDs = productPublicationIDs(product)
  const currentID = Number(product?.bean_list_publication_id || 0)
  if (currentID > 0) {
    const exact = beanListVersionGroups.value.find((group) => (
      (group?.options || []).some((item) => Number(item?.id || 0) === currentID)
    ))
    if (exact) return exact
  }
  const selectedMatch = beanListVersionGroups.value.find((group) => {
    const selected = selectedBeanListVersionOptionByGroup(group)
    return Number(selected?.id || 0) > 0 && publicationIDs.has(Number(selected.id))
  })
  if (selectedMatch) return selectedMatch
  const identity = beanListVersionGroupIdentity({
    ...product,
    list_type: currentPriceListTypeForProduct(product),
  })
  return beanListVersionGroups.value.find((group) => group.key === identity.key)
    || beanListVersionGroups.value.find((group) => (
      (group?.options || []).some((item) => publicationIDs.has(Number(item?.id || 0)))
    ))
    || null
}

function selectedBeanListVersionOptionForProduct(productOrRow) {
  const legacyProduct = productOrRow?.__order_legacy_price_product || productOrRow?.spec_source === 'legacy_price_list'
    ? (products.value.find((item) => Number(item.id || 0) === Number(productOrRow?.product_id || productOrRow?.id || 0)) || productOrRow)
    : null
  const product = productFamilyForRow(productOrRow)
    || legacyProduct
    || (productOrRow?.product_id ? (productByID(productOrRow.product_id, productOrRow) || productOrRow) : productOrRow)
  const selected = beanListVersionOptionForProductGroups(
    beanListVersionGroups.value,
    selectedBeanListPublicationIDs,
    product,
    productOrRow?.bean_list_publication_id,
  )
  if (selected) return selected
  if (beanListVersionGroupForPublicationID(beanListVersionGroups.value, productOrRow?.bean_list_publication_id)) return null
  return latestBeanListVersionOption(
    customerBeanListVersionOptions.value,
    currentPriceListTypeForProduct(product),
  )
}

function setBeanListVersion(groupKey, value) {
  const group = beanListVersionGroups.value.find((item) => item.key === groupKey)
  if (!group) return
  const selectedID = Number(value || 0)
  selectedBeanListPublicationIDs[group.key] = selectedID
  const legacyKey = group.key.startsWith('legacy:')
  const listType = group.listType || 'commercial'
  if (legacyKey) {
    const field = beanListVersionField(listType)
    form[field] = selectedID
  }
  if (listType === 'commercial' || listType === 'retail') {
    const field = beanListVersionField(listType)
    if (legacyKey) form[field] = selectedID
    form.bean_list_publication_id = selectedID
  }
  if (props.customerPortal) syncRowsForType({ priceListChanged: true })
}

function openBeanListDrawer() {
  if (!canOpenBeanListDrawer.value) {
    raiseSaveError('暂无可选择的已发布价格表')
    return
  }
  beanListDrawerOpen.value = true
}

function closeBeanListDrawer() {
  beanListDrawerOpen.value = false
}

function beanListDrawerHint(group) {
  const rows = group?.options || []
  if (!rows.length) return '没有可用的已发布价格表'
  if (!group?.autoSelect && !selectedBeanListVersionOptionByGroup(group)) return '历史兼容价格表，默认不参与新订单；需要时可手工启用'
  if (rows.some((item) => item.is_customer_owned)) return '可选择客户自定义价格表版本'
  return '当前使用公共价格表'
}

function syncBeanListVersionForCustomer(options = {}) {
  const previousIDs = selectedStaffTableIDs()
  const validGroups = new Set(beanListVersionGroups.value.map(group => group.key))
  for (const key of Object.keys(selectedBeanListPublicationIDs)) if (!validGroups.has(key)) delete selectedBeanListPublicationIDs[key]
  for (const group of beanListVersionGroups.value) {
    const currentID = Number(selectedBeanListPublicationIDs[group.key] || 0)
    if (!options.force && group.options.some(row => Number(row.id) === currentID)) continue
    const previous = !options.force && group.options.find(row => previousIDs.includes(Number(row.id)))
    const selected = previous || beanListVersionOptionForGroup(group, 0)
    selectedBeanListPublicationIDs[group.key] = Number(selected?.id || 0)
  }
  for (const type of ['commercial', 'green', 'drip']) form[beanListVersionField(type)] = 0
  form.bean_list_publication_id = 0
  for (const group of beanListVersionGroups.value) {
    const selected = selectedBeanListVersionOptionByGroup(group)
    if (!selected) continue
    const type = String(selected.list_type || 'commercial')
    form[beanListVersionField(type)] = Number(selected.id)
    if (type === 'commercial' || type === 'retail') form.bean_list_publication_id = Number(selected.id)
  }
}

function resetCustomerDrawerForm() {
  Object.assign(customerForm, emptyCustomerForm(), {
    default_source_id: defaultSourceID(),
    default_order_type_id: defaultOrderTypeID(),
  })
  customerPaste.value = ''
  customerError.value = ''
  customerNotice.value = ''
}

function openCustomerDrawer() {
  customerDrawerMode.value = 'create'
  resetCustomerDrawerForm()
  customerDrawerOpen.value = true
}

function assignCustomerDrawerForm(customer = {}) {
  Object.assign(customerForm, emptyCustomerForm(), {
    id: Number(customer.id || 0),
    name: customer.name || '',
    raw_name: customer.raw_name || '',
    customer_type: normalizeCustomerType(customer.customer_type),
    company_name: customer.company_name || '',
    company_address: customer.company_address || '',
    company_phone: customer.company_phone || customer.phone || '',
    contact: customer.contact || '',
    phone: customer.phone || '',
    address: customer.address || '',
    default_source_id: Number(customer.default_source_id || defaultSourceID()),
    default_order_type_id: Number(customer.default_order_type_id || defaultOrderTypeID()),
    responsible_employee_id: Number(customer.responsible_employee_id || 0),
    active: customer.active !== false,
    portal_enabled: customer.portal_enabled === true,
  })
}

async function openCustomerEditDrawer() {
  if (!Number(form.customer_id || 0)) {
    raiseSaveError('请先选择客户', 'customer_id')
    return
  }
  customerDrawerMode.value = 'edit'
  customerError.value = ''
  customerNotice.value = ''
  customerPaste.value = ''
  customerSaving.value = true
  try {
    const data = await apiGet(`/api/customers/${form.customer_id}`)
    assignCustomerDrawerForm(data.customer || {})
    employees.value = data.employees || employees.value
    customerDrawerOpen.value = true
  } catch (err) {
    raiseSaveError(err.message || '加载客户资料失败', 'customer_id')
  } finally {
    customerSaving.value = false
  }
}

function closeCustomerDrawer() {
  customerDrawerOpen.value = false
}

function applyRecipientParse() {
  const parsed = parseRecipientText(customerPaste.value)
  if (parsed.recipient_name) {
    customerForm.contact = parsed.recipient_name
    if (!customerForm.name) customerForm.name = parsed.recipient_name
  }
  if (parsed.phone) customerForm.phone = parsed.phone
  if (parsed.address) customerForm.address = parsed.address
}

async function saveCustomerFromDrawer() {
  customerSaving.value = true
  customerError.value = ''
  customerNotice.value = ''
  try {
    if (!customerForm.name && customerForm.contact) customerForm.name = customerForm.contact
    if (!customerForm.name) throw new Error('请填写客户名')
    if (!customerTypeLabel(customerForm.customer_type)) throw new Error('请选择客户类型')
    if (!Number(customerForm.default_source_id || 0)) throw new Error('请选择客户来源')
    if (!Number(customerForm.default_order_type_id || 0)) throw new Error('请选择客户订单类型')
    if (!Number(customerForm.responsible_employee_id || 0)) throw new Error('请选择客户负责人')
    const customerPayload = {
      name: customerForm.name,
      raw_name: customerForm.raw_name || '',
      customer_type: normalizeCustomerType(customerForm.customer_type),
      company_name: customerForm.company_name || '',
      company_address: customerForm.company_address || '',
      company_phone: customerForm.phone,
      contact: customerForm.contact,
      phone: customerForm.phone,
      address: customerForm.address,
      default_source_id: customerForm.default_source_id || null,
      default_order_type_id: customerForm.default_order_type_id || null,
      responsible_employee_id: customerForm.responsible_employee_id || null,
      active: customerForm.active !== false,
      portal_enabled: customerForm.portal_enabled === true,
    }
    if (customerForm.company_phone) customerPayload.company_phone = customerForm.company_phone
    const data = await apiSend(customerDrawerMode.value === 'edit' ? `/api/customers/${customerForm.id}` : '/api/customers', {
      method: customerDrawerMode.value === 'edit' ? 'PUT' : 'POST',
      body: customerPayload,
    })
    const saved = data.customer
    if (!saved?.id) throw new Error('客户保存失败')
    customers.value = [saved, ...customers.value.filter((item) => Number(item.id) !== Number(saved.id))]
    chooseCustomer(saved)
    if (customerDrawerMode.value === 'edit') {
      assignCustomerDrawerForm(saved)
      customerNotice.value = '客户信息已保存'
    } else {
      closeCustomerDrawer()
    }
  } catch (err) {
    customerError.value = err.message || '保存客户失败'
  } finally {
    customerSaving.value = false
  }
}

function scopedOrderProductOptions() {
  if (fulfillmentOrderEditing.value) return fulfillmentFamilies.value
  if (!props.customerPortal) {
    if (staffCatalog.loading || staffCatalog.error || Number(staffCatalog.customerID) !== Number(form.customer_id)) return []
    return staffCatalogFamilies()
  }
  return filterProductsForCustomer(
    productFamilies.value,
    form.customer_id,
    selectedBeanListPublicationIDsByType(),
    customerPublicUsages.value,
    customerOwnedBeanListPublicationIDsByType(),
    true,
  ).flatMap((family) => {
    if (!isOrderProductFamily(family)) return [family]
    const selected = selectedBeanListVersionOptionForProduct(family)
    if (Number(selected?.id || 0) <= 0) {
      return family?.__order_concrete_price_family ? [] : [family]
    }
    const scoped = orderFamilySearchScopeForPublication(family, selected.id)
    return scoped.specs.length > 0 ? [scoped] : []
  })
}

function productKindFilterOptions() {
  const families = scopedOrderProductOptions()
  const classified = orderProductClassificationFilterOptions(families, beanListVersionGroups.value, selectedBeanListPublicationIDs)
  return classified.length ? classified : orderProductKindFilterOptions(families)
}

function activeProductKindFilter(row) {
  const selected = String(row?.product_kind_filter || '').trim()
  return productKindFilterOptions(row).some((option) => option.value === selected) ? selected : ''
}

function productOptions(row) {
  const families = scopedOrderProductOptions()
  const classified = orderProductClassificationFilterOptions(families, beanListVersionGroups.value, selectedBeanListPublicationIDs)
  const selected = activeProductKindFilter(row)
  const scopedFamilies = classified.length
    ? orderProductFamiliesForClassification(families, selected, beanListVersionGroups.value, selectedBeanListPublicationIDs)
    : families
  return sortProductsByCustomerUsage(
    orderProductFamilyOptions(scopedFamilies, row.product_query, classified.length ? '' : selected),
    form.customer_id,
    customerProductUsages.value,
  ).slice(0, 30)
}

function orderProductCategoryLabel(product = {}) {
  return orderProductClassificationForFamily(product, beanListVersionGroups.value, selectedBeanListPublicationIDs)?.label
    || productKindLabel(product)
}

function orderProductCategoryBadgeClass(product = {}) {
  return orderProductClassificationForFamily(product, beanListVersionGroups.value, selectedBeanListPublicationIDs)
    ? 'kind-price-list'
    : productKindBadgeClass(product)
}

function handleOrderProductPointerDown(event) {
  const combobox = event?.target?.closest?.('[data-product-combobox-key]')
  closeOrderProductDropdowns(rows.value, combobox?.dataset?.productComboboxKey || '')
}

function openProductDropdown(row) {
  if (!props.customerPortal && !staffCatalog.loading) void refreshStaffOrderCatalog()
  closeOrderProductDropdowns(rows.value, row?.key)
  if (row) row.product_open = true
}

function productOptionKey(product) {
  if (product?.__order_legacy_price_product) return `legacy:${Number(product.id || 0)}`
  return `family:${orderProductFamilyIdentity(product)}`
}

function productSpecCountForCurrentList(family) {
  if (fulfillmentOrderEditing.value) return family?.specs?.length || 0
  if (!isOrderProductFamily(family)) return 0
  const selected = selectedBeanListVersionOptionForProduct(family)
  if (Number(selected?.id || 0) <= 0) return 0
  return orderFamilySpecOptions(family, selected.id).length
}

function clearProduct(row) {
  row.spec_conversion = null
  row.price_selection_mode = "auto"
  row.selected_price_row_key = ""
  row.product_open = true
  row.parent_product_id = 0
  row.product_family_key = ''
  row.parent_product_name = ''
  row.product_id = 0
  row.product_name = ''
  row.product_code = ''
  row.product_record_name = ''
  row.customer_product_reference_id = 0
  row.bom_spec_id = 0
  row.bom_variant_id = 0
  row.migration_state = 'legacy'
  row.customer_product_alias_id = 0
  row.customer_product_display_name = ''
  row.customer_item_code = ''
  row.brand_name = ''
  row.product_kind = 'roasted_bean'
  row.product_type_category_id = 0
  row.product_type_name = ''
  row.tier_id = 'auto'
  row.bean_list_publication_id = 0
  row.bean_list_version_no = ''
  row.bean_list_version_tip_open = false
  row.unit_price = ''
  clearWholesalePriceMetadata(row)
  row.manual_price = false
  row.spec_mode = ''
  row.spec_source = ''
  row.spec_label = ''
  row.spec_g = 0
  row.historical_spec_readonly = false
  row.spec_invalid_message = ''
  row.custom_spec_g = ''
  row.sales_unit = ''
  row.unit_bag_count = 0
  row.unit_bean_g = ''
  row.unit = '件'
}

function assignProductFamilyHeader(row, family) {
  row.product_family_key = orderProductFamilyIdentity(family)
  row.parent_product_id = Number(family?.parent_product_id || family?.id || 0)
  row.parent_product_name = family?.parent_product_name || family?.product_name_snapshot || family?.name || ''
  row.product_name = family?.name || row.parent_product_name
  row.product_query = row.product_name
  row.product_record_name = row.parent_product_name
  row.customer_product_alias_id = Number(family?.customer_product_alias_id || 0)
  row.customer_product_display_name = family?.customer_product_display_name || row.product_name
  row.customer_item_code = family?.customer_item_code || ''
  row.brand_name = family?.brand_name || ''
  row.product_kind = family?.product_kind || 'roasted_bean'
  row.product_type_category_id = Number(family?.product_type_category_id || 0)
  row.product_type_name = family?.product_type_name || ''
}

function applyPriceListSpecToRow(row, family, spec, publicationID) {
  row.spec_conversion = null
  const qty = row.qty
  const itemNote = row.item_note
  const discountType = row.discount_type
  const discountValue = row.discount_value
  clearWholesalePriceMetadata(row)
  Object.assign(row, orderFamilySpecRowPatch(family, spec, publicationID), {
    tier_id: 'auto',
    price_selection_mode: 'auto',
    selected_price_row_key: '',
    unit_price: '',
    manual_price: false,
    qty,
    item_note: itemNote,
    discount_type: discountType,
    discount_value: discountValue,
  })
  if (Number(publicationID || 0) > 0 && orderFamilySpecsForPublication({ specs: [spec] }, publicationID).length === 0) {
    row.spec_invalid_message = '当前价格表未发布该商品规格，不能保存；请补齐价格表或选择有价规格。'
  }
}

function invalidatePriceListSpecRow(row, message = '所选价格表不包含该商品当前规格，请重新选择规格。') {
  const family = productFamilyForRow(row)
  if (family) assignProductFamilyHeader(row, family)
  const selected = family ? selectedBeanListVersionOptionForProduct(family) : null
  row.product_id = 0
  row.product_code = ''
  row.spec_source = 'price_list_sku'
  row.spec_mode = ''
  row.spec_label = ''
  row.spec_g = 0
  row.custom_spec_g = ''
  row.sales_unit = ''
  row.unit_bag_count = 0
  row.unit_bean_g = ''
  row.unit = '件'
  row.tier_id = 'auto'
  row.unit_price = ''
  row.manual_price = false
  row.historical_spec_readonly = false
  row.spec_invalid_message = message
  row.bean_list_publication_id = Number(selected?.id || 0)
  row.bean_list_version_no = String(selected?.version_no || '').trim()
  clearWholesalePriceMetadata(row)
  row.price_missing = true
}

function fulfillmentOriginalItem(row) {
  return fulfillmentEditContext.value?.items?.find(item => Number(item.item_id) > 0 && Number(item.item_id) === Number(row.item_id)) || null
}

function chooseProduct(row, product) {
  if (fulfillmentOrderEditing.value) {
    const spec = product.specs?.find(entry => entry.is_default || entry.is_default_sku) || product.specs?.[0]
    if (!spec) return
    Object.assign(row, fulfillmentSpecRowPatch(product, spec, fulfillmentEditContext.value.fulfillment_mode, fulfillmentOriginalItem(row)), { product_open: false })
    ensureTrailingBlankRow()
    return
  }
  row.spec_conversion = null
  row.price_selection_mode = 'auto'
  row.selected_price_row_key = ''
  row.manual_price = false
  const family = product?.__order_legacy_price_product
    ? null
    : (product?.__order_product_family ? product : productFamilyByParentID(product?.parent_product_id || product?.id, product))
  if (isOrderProductFamily(family)) {
    assignProductFamilyHeader(row, family)
    row.product_open = false
    const selected = selectedBeanListVersionOptionForProduct(family)
    const selectedPublicationID = Number(selected?.id || 0)
    if (selectedPublicationID <= 0) {
      invalidatePriceListSpecRow(row, '当前商品没有可用价格表，请先发布价格表后重试。')
      return
    }
    const defaultSpec = orderFamilyDefaultSpec(family, selectedPublicationID)
    const spec = orderSpecSelectionAfterPublicationChange(
      family,
      defaultSpec?.sku_id,
      selectedPublicationID,
    )
    if (!spec) {
      invalidatePriceListSpecRow(row, '当前价格表没有该商品的可售规格，请先维护并发布价格后重试。')
      return
    }
    applyPriceListSpecToRow(row, family, spec, selectedPublicationID)
    syncPrice(row, { force: true })
    ensureTrailingBlankRow()
    return
  }
  row.product_family_key = ''
  row.parent_product_id = product?.__order_legacy_price_product ? 0 : Number(product?.parent_product_id || 0)
  row.parent_product_name = product?.parent_product_name || product?.product_name_snapshot || product?.name || ''
  row.product_id = Number(product?.id || 0)
  row.product_name = product?.name || ''
  row.product_query = product?.name || ''
  row.product_code = product?.product_code || (row.product_id ? `SKU-${row.product_id}` : '')
  row.product_record_name = product?.product_name_snapshot || product?.product_record_name || product?.source_product_name || product?.name || ''
  row.customer_product_alias_id = Number(product?.customer_product_alias_id || 0)
  row.customer_product_display_name = product?.customer_product_display_name || product?.name || ''
  row.customer_item_code = product?.customer_item_code || ''
  row.brand_name = product?.brand_name || ''
  row.product_open = false
  row.manual_price = false
  row.product_kind = product?.product_kind || 'roasted_bean'
  row.product_type_category_id = Number(product?.product_type_category_id || 0)
  row.product_type_name = product?.product_type_name || ''
  row.spec_source = product?.__order_legacy_price_product ? 'legacy_price_list' : ''
  row.spec_label = ''
  row.spec_g = 0
  row.historical_spec_readonly = false
  row.spec_invalid_message = ''
  if (isDripProduct(product)) {
    const defaultSpec = defaultDripSalesUnitSpec(product)
    row.sales_unit = defaultSpec.salesUnit
    row.unit_bean_g = defaultSpec.unitBeanG
    row.unit_bag_count = defaultSpec.unitBagCount
    row.unit = defaultSpec.unitLabel
    row.spec_mode = ''
    row.custom_spec_g = ''
    syncPrice(row, { force: true })
    ensureTrailingBlankRow()
    return
  }
  row.sales_unit = ''
  row.unit_bag_count = 0
  row.unit_bean_g = ''
  row.unit = '件'
  if (retailOrder.value) {
    const options = retailSpecOptions(product, true)
    row.spec_mode = options[0]?.value || CUSTOM_SPEC_VALUE
  } else {
    row.spec_mode = defaultWholesaleSpec(product)
  }
  syncPrice(row, { force: true })
  ensureTrailingBlankRow()
}

function specOptions(row) {
  if (fulfillmentOrderEditing.value) {
    const { family } = fulfillmentSpecForRow(fulfillmentFamilies.value, row)
    return (family?.specs || []).map(spec => ({ value: String(spec.bom_spec_id || spec.sku_id || spec.product_id), label: spec.spec_label || spec.spec_name }))
  }
  const family = productFamilyForRow(row)
  if (isOrderProductFamily(family) || row?.spec_source === 'price_list_sku') {
    const selected = selectedBeanListVersionOptionForProduct(family || row)
    return orderFamilySpecOptions(family || {}, selected?.id || 0)
  }
  const product = productForRow(row)
  if (isDripProduct(product)) return []
  if (retailOrder.value) return retailSpecOptions(product, true)
  return wholesaleSpecOptions(product)
}

function onSpecChange(row) {
  if (fulfillmentOrderEditing.value) {
    const { family } = fulfillmentSpecForRow(fulfillmentFamilies.value, row)
    const spec = family?.specs?.find(spec => String(spec.bom_spec_id || spec.sku_id || spec.product_id) === String(row.spec_mode))
    if (spec) Object.assign(row, fulfillmentSpecRowPatch(family, spec, fulfillmentEditContext.value.fulfillment_mode, fulfillmentOriginalItem(row)))
    else Object.assign(row, { bom_spec_id: 0, bom_variant_id: 0, spec_label: '', unit_price: '', price_missing: true, historical_spec_readonly: false, spec_invalid_message: '请重新选择商品规格' })
    return
  }
  const family = productFamilyForRow(row)
  if (!family) return
  const selected = selectedBeanListVersionOptionForProduct(family)
  const spec = orderSpecSelectionAfterPublicationChange(
    family,
    Number(row.spec_mode || 0),
    selected?.id || 0,
  )
  if (!spec) {
    invalidatePriceListSpecRow(row)
    return
  }
  applyPriceListSpecToRow(row, family, spec, selected?.id || 0)
  syncPrice(row, { force: true })
}

function onLegacySpecChange(row) {
  row.price_selection_mode = 'auto'
  row.selected_price_row_key = ''
  row.manual_price = false
  syncPrice(row, { force: true })
}

function syncRowsForType(options = {}) {
  if (fulfillmentOrderEditing.value) return
  rows.value.forEach((row) => {
    if (preservesStoredOrderPrice(row)) return
    if (options.priceListChanged) {
      row.price_selection_mode = 'auto'
      row.selected_price_row_key = ''
      row.manual_price = false
    }
    if (row.spec_source === 'legacy_price_list' && options.priceListChanged) {
      const legacyProduct = products.value.find((item) => Number(item.id || 0) === Number(row.product_id || 0)) || null
      const selected = selectedBeanListVersionOptionForProduct(row)
      const selectedPublicationID = Number(selected?.id || 0)
      const publicationMode = orderProductPublicationMode(legacyProduct || row, selectedPublicationID)
      if (publicationMode !== 'legacy') {
        const family = productFamilyBySKU(row.product_id, row)
          || productFamilyByParentID(legacyProduct?.parent_product_id, row)
        if (family) assignProductFamilyHeader(row, family)
        else {
          row.parent_product_id = Number(legacyProduct?.parent_product_id || 0)
          row.parent_product_name = legacyProduct?.parent_product_name || legacyProduct?.product_name_snapshot || ''
        }
        row.spec_source = 'price_list_sku'
        invalidatePriceListSpecRow(
          row,
          publicationMode === 'concrete'
            ? '已切换到具体规格价格表，请重新选择价格表中的规格。'
            : '所选价格表不包含该商品当前规格，请重新选择规格。',
        )
        if (!family) {
          row.bean_list_publication_id = selectedPublicationID
          row.bean_list_version_no = String(selected?.version_no || '').trim()
        }
        return
      }
    }
    const family = productFamilyForRow(row)
    if (family?.__order_concrete_price_family || row.spec_source === 'price_list_sku') {
      if (row.historical_spec_readonly && !options.priceListChanged) return
      const selected = selectedBeanListVersionOptionForProduct(family || row)
      const spec = family && Number(selected?.id || 0) > 0
        ? orderFamilySpecForStoredItem(family, row, selected.id)
        : null
      if (!spec) {
        invalidatePriceListSpecRow(row)
        return
      }
      applyPriceListSpecToRow(row, family, spec, selected?.id || 0)
      syncPrice(row, { force: true })
      return
    }
    if (!row.product_id) return
    row.manual_price = false
    if (isDripRow(row)) {
      syncPrice(row, { force: true })
      return
    }
    const specChoices = specOptions(row)
    if (!specChoices.some((option) => option.value === row.spec_mode)) {
      row.spec_mode = retailOrder.value ? (specChoices[0]?.value || '') : defaultWholesaleSpec(productForRow(row))
      row.custom_spec_g = ''
    }
    syncPrice(row, { force: true })
  })
}

function syncPrice(row, options = {}) {
  if (fulfillmentOrderEditing.value) return
  if (isFrozenConversion(row)) return
  if (preservesSampleSpec(row, sampleOrderEditing.value) && row.manual_price && !options.force) return
  const product = productForRow(row)
  if (!product) {
    row.unit_price = ''
    clearWholesalePriceMetadata(row)
    return
  }
  if (row.price_selection_mode === 'tier') {
    const selectedTier = tierRows(row).find((tier) => String(tier.priceRowKey || '') === String(row.selected_price_row_key || ''))
    if (!selectedTier || !(Number(selectedTier.unitPrice) > 0)) {
      row.unit_price = ''
      row.tier_id = 'auto'
      row.price_missing = true
      row.tier_below_min = false
      row.tier_price_label = '所选价格档已失效，请重新选择'
      row.price_source_json = ''
      return
    }
    row.manual_price = false
    row.tier_id = 'auto'
    row.unit_price = String(selectedTier.unitPrice)
    row.price_source_json = String(selectedTier.priceSourceJSON || '')
    row.price_unit = selectedTier.priceUnit?.label || ''
    row.price_unit_suffix = selectedTier.priceUnit?.suffix || ''
    row.price_unit_g = Number(selectedTier.priceUnit?.unitG || 0)
    row.tier_price_label = String(selectedTier.tierLabel || selectedTier.rangeLabel || '')
    row.price_missing = false
    row.tier_below_min = false
    ensureRowBeanListVersion(row, {
      beanListPublicationID: selectedTier.publicationID,
      beanListVersionNo: selectedTier.versionNo,
    })
    return
  }
  if (isConcretePriceListRow(row)) {
    if (row.manual_price && !options.force) return
    syncRowBeanListVersionFromSelection(row)
    applyResolvedWholesalePrice(row, resolveWholesaleTierPrice(product, row))
    row.manual_price = false
    return
  }
  if (isDripProduct(product)) {
    syncRowBeanListVersionFromSelection(row)
    applyDripUnit(row, product)
    clearWholesalePriceMetadata(row)
    ensureRowBeanListVersion(row)
    if (row.manual_price && !options.force) return
    const price = syncDripTierPrice(product, row)
    row.tier_id = price.tierID
    row.unit_price = price.unitPrice
    row.price_missing = price.unitPrice === ''
    row.manual_price = false
    return
  }
  if (row.manual_price && !options.force) return
  if (retailOrder.value) {
    syncRowBeanListVersionFromSelection(row)
    const publishedPrice = resolveWholesaleTierPrice(product, row)
    if (publishedPrice.quantityBasis === 'sales_spec_count') {
      applyResolvedWholesalePrice(row, publishedPrice)
      row.manual_price = false
      return
    }
    row.tier_id = 'auto'
    row.unit_price = String(retailPackagePrice(product, normalizeSpecG(row)) || '')
    clearWholesalePriceMetadata(row)
    ensureRowBeanListVersion(row)
    row.manual_price = false
    return
  }
  syncRowBeanListVersionFromSelection(row)
  applyResolvedWholesalePrice(row, resolveWholesaleTierPrice(product, row))
  row.manual_price = false
}

function clearWholesalePriceMetadata(row) {
  row.quantity_basis = ''
  row.price_source_json = ''
  row.price_unit = ''
  row.price_unit_suffix = ''
  row.price_unit_g = 0
  row.tier_price_label = ''
  row.tier_below_min = false
  row.price_missing = false
}

function applyResolvedWholesalePrice(row, price) {
  row.tier_id = price.tierID
  row.unit_price = price.unitPrice
  row.quantity_basis = String(price.quantityBasis || '')
  row.price_source_json = String(price.priceSourceJSON || '')
  row.price_unit = price.priceUnit?.label || ''
  row.price_unit_suffix = price.priceUnit?.suffix || ''
  row.price_unit_g = Number(price.priceUnit?.unitG || 0)
  row.tier_price_label = price.tierPriceLabel || ''
  row.tier_below_min = Boolean(price.belowMinTier)
  row.price_missing = Boolean(price.priceMissing)
  ensureRowBeanListVersion(row, price)
}

function ensureRowBeanListVersion(row, price = {}) {
  if (preservesSampleSpec(row, sampleOrderEditing.value) && !Number(row.bean_list_publication_id || 0) && !Number(price.beanListPublicationID || 0)) return
  const selected = selectedBeanListVersionOptionForProduct(row)
  const publicationID = Number(price.beanListPublicationID || row.bean_list_publication_id || selected?.id || 0)
  row.bean_list_publication_id = publicationID
  row.bean_list_version_no = String(price.beanListVersionNo || row.bean_list_version_no || selected?.version_no || '').trim()
  if (!isRowBeanListVersionStale(row)) row.bean_list_version_tip_open = false
}

function syncRowBeanListVersionFromSelection(row) {
  const selected = selectedBeanListVersionOptionForProduct(row)
  row.bean_list_publication_id = Number(selected?.id || 0)
  row.bean_list_version_no = String(selected?.version_no || '').trim()
  if (!isRowBeanListVersionStale(row)) row.bean_list_version_tip_open = false
}

function isRowBeanListVersionStale(row) {
  return rowUsesStaleBeanListPublication(
    row,
    customerBeanListVersionOptions.value,
    currentPriceListTypeForProduct(row),
  )
}

function toggleBeanListVersionTip(row) {
  row.bean_list_version_tip_open = !row.bean_list_version_tip_open
}

function applyDripUnit(row, product) {
  row.product_kind = 'drip_bag'
  const spec = dripSalesUnitSpec(product, row)
  row.sales_unit = spec.salesUnit
  row.unit_bean_g = spec.unitBeanG
  row.unit_bag_count = spec.unitBagCount
  row.unit = spec.unitLabel
}

function onDripUnitChange(row) {
  row.manual_price = false
  row.price_selection_mode = 'auto'
  row.selected_price_row_key = ''
  syncPrice(row, { force: true })
}

function dripUnitOptionsForRow(row) {
  return dripUnitOptions(productForRow(row))
}

function markManualPrice(row) {
  if (isFrozenConversion(row)) return
  row.manual_price = true
  row.tier_id = 'manual'
  row.price_selection_mode = 'manual'
  row.selected_price_row_key = ''
}

function hasValidManualPrice(row) {
  return validManualOrderPrice(row, preservesSampleSpec(row, sampleOrderEditing.value) && row.sample_stored_manual === true)
}

function rowHasBlockingPrice(row) {
  if (Number(row?.product_id || row?.parent_product_id || 0) <= 0) return false
  if (row?.spec_invalid_message) return true
  if (row?.manual_price) return !hasValidManualPrice(row)
  return Boolean(row?.price_missing)
}

function rowPriceBlockingMessage(row) {
  if (row?.spec_invalid_message) return row.spec_invalid_message
  if (row?.manual_price && !hasValidManualPrice(row)) {
    if (preservesSampleSpec(row, sampleOrderEditing.value) && row.sample_stored_manual === true) return '样品手动价必须为有效非负金额，不能留空。'
    return '手动价必须大于0，不能保存；请输入有效手动价或恢复自动价。'
  }
  return '当前数量无已发布价格，不能保存；请调整数量、补齐价格表档位或输入手动价。'
}

function resetAutoPrice(row) {
  if (isFrozenConversion(row)) return
  row.spec_conversion = null
  row.manual_price = false
  row.price_selection_mode = 'auto'
  row.selected_price_row_key = ''
  syncPrice(row, { force: true })
}

function tierRows(row) {
  const product = productForRow(row)
  if (!isConcretePriceListRow(row) && isDripRow(row)) return dripTierPriceRows(product, row)
  const specG = normalizeSpecG(row)
  return wholesaleTierPriceRows(product, row).filter((tier) => Number(tier.specG || 0) === specG)
}

function selectTier(row, tier) {
  if (props.customerPortal || !String(tier?.priceRowKey || '').trim()) return
  row.manual_price = false
  row.price_selection_mode = 'tier'
  row.selected_price_row_key = String(tier.priceRowKey)
  row.tier_id = 'auto'
  syncPrice(row, { force: true })
}

function isTierActive(row, tier) {
  if (row?.price_selection_mode === 'tier') return String(row.selected_price_row_key || '') === String(tier?.priceRowKey || '')
  return isOrderTierActive(row, tier)
}

function autoPriceLabel(row) {
  if (fulfillmentOrderEditing.value) return fulfillmentEditContext.value?.fulfillment_mode === 'stock' ? '已购库存，货款不重复收取' : '授权代发价格表固定价'
  if (isFrozenConversion(row)) return "历史订单冻结价格"
  if (row?.price_selection_mode === 'tier') return `手动选档 ${row.tier_price_label || ''}`.trim()
  if (row?.price_selection_mode === 'manual' || row?.manual_price) return '手动输入单价'
  if (isDripRow(row)) return row.sales_unit === 'box' ? '挂耳盒价' : '挂耳袋价'
  if (props.customerPortal && retailOrder.value) return '零售价'
  if (row.tier_price_label) return `梯度 ${row.tier_price_label}`
  if (row.tier_id && row.tier_id !== 'auto' && row.tier_id !== 'manual') return `梯度 ${row.tier_id}`
  return '自动价'
}

function priceUnitLabel(row) {
  if (fulfillmentOrderEditing.value) return `元/${row.unit || '件'}`
  if (isDripRow(row)) return row.sales_unit === 'box' ? '元/盒' : '元/袋'
  return orderRowPriceUnit(row).label
}

function quantityLabel(row) {
  const unit = String(row?.unit || '').trim()
  return unit ? `数量（${unit}）` : '数量'
}

function fulfillmentLineState(row) {
  return fulfillmentLinePreview(row, fulfillmentOriginalItem(row), fulfillmentEditContext.value?.fulfillment_mode)
}

function rowTotal(row) {
  if (fulfillmentOrderEditing.value) return fulfillmentLineState(row).amount
  return lineTotal(productForRow(row), row, retailOrder.value)
}

function fulfillmentSupplyLabel(row) {
  const { spec } = fulfillmentSpecForRow(fulfillmentFamilies.value, row)
  if (!spec) return '原单商品快照；请重新读取当前客户权益后核对'
  const supply = stockSupplySummary(spec, row.qty)
  return `实物可用 ${supply.physical} · 待入库 ${supply.pending} · 待入库已预订 ${supply.reserved} · 本单原占用 ${Number(spec.own_reserved_qty || 0)} · 本单待补购 ${supply.gap}；可用量已计入本单原占用，待入库权益可等待入库`
}

function onRowDiscountTypeChange(row) {
  if (row?.discount_type === 'free' || row?.discount_type === '') {
    row.discount_value = ''
  }
}

function discountValuePlaceholder(row) {
  if (row?.discount_type === 'percent') return '90=9折'
  if (row?.discount_type === 'unit_amount') return `每${priceUnitLabel(row).replace(/^元\//, '')}优惠金额`
  return '减免金额'
}

function money(value) {
  return Number(value || 0).toFixed(2)
}

function unitPriceMoney(value) {
  const n = Number(value || 0)
  if (!Number.isFinite(n)) return '0'
  return Number.isInteger(n) ? String(n) : n.toFixed(2)
}

function addRow() {
  rows.value.push(newRow())
}

function ensureTrailingBlankRow() {
  if (!needsTrailingBlankOrderLine(rows.value)) return
  rows.value.push(newRow())
}

function removeRow(idx) {
  if (rows.value.length <= 1) return
  rows.value.splice(idx, 1)
}

function applyDefaultSelections(data) {
  if (!form.pay_status_id) {
    form.pay_status_id = defaultStatusID(payStatuses.value, ['已付款', '已收款']) || Number(payStatuses.value[0]?.id || 0)
  }
  if (!form.ship_status_id) {
    form.ship_status_id = defaultStatusID(shipStatuses.value, ['未发货']) || Number(shipStatuses.value[0]?.id || 0)
  }
  form.document_date = form.document_date || data.today || ''
  form.order_date = form.order_date || data.today || ''
}

function applyCustomerContextToNewOrder() {
  const customerID = Number(props.customerContextId || 0)
  if (customerID <= 0 || form.edit_id || copyMode.value) return
  const customer = customers.value.find((item) => Number(item.id || 0) === customerID)
  if (customer) {
    chooseCustomer(customer)
    return
  }
  form.customer_id = customerID
  customerQuery.value = props.customerContextLabel || `客户 #${customerID}`
  syncBeanListVersionForCustomer({ force: true })
}

function notifyWorkspaceCustomerChanged(customerID) {
  if (props.workspaceMode !== CUSTOMER_WORKSPACE_MODE || Number(customerID || 0) <= 0) return
  if (Number(customerID || 0) === Number(props.customerContextId || 0)) return
  window.dispatchEvent(workspaceCustomerChangeEvent(customerID))
}

function applyEditData(data) {
  prepaymentRate.value = 0
  if (!data) return
  fulfillmentEditData.value = data
  orderFreight.value=copyMode.value?null:(data.freight||null)
  form.freight_state=freightStateFromOrder(data)
  sampleOrderEditing.value = isSampleOrderEdit(data, {copyMode:copyMode.value,customerPortal:props.customerPortal})
  const editItems = Array.isArray(data.items) ? data.items : []
  const itemPublicationIDByType = (listType) => {
    const item = editItems.find((row) => orderBeanListTypeForProductKind(row.product_kind) === listType && Number(row.bean_list_publication_id || 0) > 0)
    return Number(item?.bean_list_publication_id || 0)
  }
  editBlockReason.value = data.can_edit === false ? (data.edit_block_reason || '订单当前状态不能编辑') : ''
  if (!editBlockReason.value && !fulfillmentOrderEditing.value && orderFreight.value?.edit_locked) {
    editBlockReason.value = '运费已出账或已确认实际费用，整单编辑已锁定；登记收款请使用员工小程序订单详情顶部“编辑付款与发货状态”。'
  }
  emit('editability', !editBlockReason.value)
  Object.assign(form, {
    ...orderEditRecipientFields(data),
    settlement_reason: data.settlement_reason || '',
    edit_id: Number(data.edit_id || form.edit_id || 0),
 edit_revision: data.edit_revision || "",
    document_date: data.document_date || data.order_date || form.document_date,
    order_date: data.order_date || form.order_date,
    customer_id: Number(data.customer_id || 0),
    delivery_mode: data.delivery_mode === 'customer_stock' ? 'customer_stock' : 'ship',
    delivery_warehouse_code: data.delivery_warehouse_code || '',
    source_id: Number(data.source_id || 0),
    order_type_id: Number(data.order_type_id || 0),
    pay_status_id: Number(data.pay_status_id || 0),
    payment_method: data.payment_method || '',
    prepayment_amount: data.prepayment_amount || '0',
    ship_status_id: Number(data.ship_status_id || 0),
    ship_method: data.ship_method || '',
    ship_tracking_no: data.ship_tracking_no || '',
    logistics_company_id: Number(data.logistics_company_id || 0),
    logistics_product_id: Number(data.logistics_product_id || 0),
    payment_goods_amount: data.payment_goods_amount ?? '',
    payment_shipping_amount: data.payment_shipping_amount ?? '',
    payment_voucher_asset_id: Number(data.payment_voucher_asset_id || 0),
    bean_list_publication_id: Number(data.bean_list_publication_id || 0),
    commercial_bean_list_publication_id: Number(data.commercial_bean_list_publication_id || data.bean_list_publication_id || itemPublicationIDByType('commercial') || 0),
    green_bean_list_publication_id: Number(data.green_bean_list_publication_id || itemPublicationIDByType('green') || 0),
    drip_bean_list_publication_id: Number(data.drip_bean_list_publication_id || itemPublicationIDByType('drip') || 0),
    notes: data.notes || '',
    shipping_amount: data.shipping_amount ?? '',
    discount_amount: data.discount_amount || '',
    round_to_int: !!data.round_to_int,
    express_fee: data.express_fee || '',
    outsource_material_fee: data.outsource_material_fee || '',
    outsource_roast_fee: data.outsource_roast_fee || '',
    outsource_packaging_fee: data.outsource_packaging_fee || '',
    outsource_manual_fee: data.outsource_manual_fee || '',
    outsource_tax_fee: data.outsource_tax_fee || '',
    outsource_other_fee: data.outsource_other_fee || '',
  })
  if (fulfillmentOrderEditing.value) {
    rows.value = editItems.map(item => {
      const { family } = fulfillmentSpecForRow(fulfillmentFamilies.value, item)
      return { ...newRow(), ...item, parent_product_id: Number(item.parent_product_id || item.product_id),
        product_query: family?.name || item.product_name || '', product_name: family?.name || item.product_name || '',
        spec_source: 'price_list_sku', spec_mode: String(item.bom_spec_id || item.product_id || ''),
        spec_label: item.spec_name || item.spec_label || item.bom_spec_name || item.spec || '',
        unit: item.inventory_unit || item.unit || item.sales_unit || '件',
        unit_price: data.fulfillment_mode === 'stock' ? '0.00' : item.unit_price,
        bean_list_publication_id: Number(item.publication_id || item.bean_list_publication_id || 0),
        historical_spec_readonly: true, spec_invalid_message: '', price_missing: false }
    })
    return
  }
  syncOrderHeaderFromCustomer(selectedCustomer.value, { syncRows: false })
  paymentVoucher.value = data.payment_voucher || null
  paymentVoucherCollapsed.value = Boolean(paymentVoucher.value)
  customerQuery.value = optionName(customers.value, form.customer_id)
  rows.value = editItems.map((item) => {
    const spec = String(item.spec || '').replace(/g$/i, '')
    const flatProduct = products.value.find((entry) => Number(entry.id || 0) === Number(item.product_id || 0)) || null
    const familyReference = {
      customer_id: form.customer_id,
      customer_product_alias_id: item.customer_product_alias_id,
      customer_product_reference_id: item.customer_product_reference_id,
    }
    const family = orderProductFamilyForStoredItem(productFamilies.value, item, familyReference)
    const storedSpec = family ? orderFamilySpecForStoredItem(family, item, item.bean_list_publication_id) : null
    const product = storedSpec ? orderFamilySpecProduct(family, storedSpec, item.bean_list_publication_id) : flatProduct
    const productKind = item.product_kind || product?.product_kind || 'roasted_bean'
    const salesUnit = item.sales_unit || (item.unit === '盒' ? 'box' : 'bag')
    const unitBagCount = salesUnit === 'box'
      ? (toInt(item.unit_bag_count) || toInt(product?.drip_box_bag_count) || 10)
      : 1
    const specNumber = toNumber(spec)
    const unitBeanG = toNumber(item.unit_bean_g)
      || toNumber(product?.drip_bag_grams)
      || (salesUnit === 'box' && unitBagCount > 0 ? specNumber / unitBagCount : specNumber)
      || 10
    const retailSpecs = (product?.retail_specs || []).map(toInt)
    const shouldUseCustomSpec = productKind !== 'drip_bag' && retailOrder.value && !retailSpecs.includes(toInt(spec))
    const hydrated = {
      ...newRow(),
      item_id: copyMode.value ? 0 : Number(item.item_id || 0),
      bom_spec_id: Number(item.bom_spec_id || 0),
      bom_variant_id: Number(item.bom_variant_id || 0),
      migration_state: Number(item.bom_spec_id || 0) > 0 ? 'cutover' : 'legacy',
      customer_product_reference_id: Number(item.customer_product_reference_id || 0),
      product_family_key: family ? orderProductFamilyIdentity(family) : '',
      parent_product_id: Number(family?.parent_product_id || product?.parent_product_id || 0),
      parent_product_name: family?.parent_product_name || product?.parent_product_name || '',
      product_id: Number(item.product_id || 0),
      product_name: item.product_name || '',
      product_query: item.product_name || '',
      product_code: item.product_code_snapshot || (item.product_id ? `SKU-${item.product_id}` : ''),
      product_record_name: item.product_name_snapshot || product?.product_name_snapshot || product?.name || item.product_name || '',
      customer_product_alias_id: Number(item.customer_product_alias_id || 0),
      customer_product_display_name: item.customer_product_display_name_snapshot || item.product_name || '',
      customer_item_code: item.customer_item_code_snapshot || '',
      brand_name: item.brand_name_snapshot || '',
      product_kind: productKind,
      product_type_category_id: Number(product?.product_type_category_id || 0),
      product_type_name: product?.product_type_name || '',
      tier_id: item.tier_id || 'auto',
      price_selection_mode: item.price_selection_mode || (item.price_override === true || item.tier_id === 'manual' ? 'manual' : 'auto'),
      selected_price_row_key: item.selected_price_row_key || '',
      quantity_basis: '',
      price_source_json: item.price_source_json || '',
      bean_list_publication_id: Number(item.bean_list_publication_id || 0),
      bean_list_version_no: item.bean_list_version_no || '',
      unit_price: item.unit_price ?? '',
      price_unit: '',
      price_unit_suffix: '',
      price_unit_g: 0,
      tier_price_label: '',
      tier_below_min: false,
      manual_price: item.price_selection_mode === 'manual' || item.price_override === true || item.tier_id === 'manual',
      spec_mode: productKind === 'drip_bag' ? '' : (shouldUseCustomSpec ? CUSTOM_SPEC_VALUE : spec),
      custom_spec_g: shouldUseCustomSpec ? spec : '',
      sales_unit: productKind === 'drip_bag' ? salesUnit : '',
      unit_bag_count: productKind === 'drip_bag' ? unitBagCount : 0,
      unit_bean_g: productKind === 'drip_bag' ? unitBeanG : '',
      qty: Number(item.qty || 1),
      unit: productKind === 'drip_bag' ? (salesUnit === 'box' ? '盒' : '袋') : (item.unit || '件'),
      item_note: item.note || '',
      discount_type: item.discount_type || '',
      discount_value: item.discount_value || '',
    }
    const conversion = conversionSnapshot(item)
    if (!copyMode.value && ['converted_tier','bom_tier'].includes(item.price_selection_mode) && conversion) {
      const result = convertedOrderRow(hydrated, { ...conversion, unit_price: item.unit_price, price_source_json: item.price_source_json })
      result.spec_conversion.reprice = false
      result.historical_spec_readonly = true
      return result
    }
    if (copyMode.value && ['converted_tier','bom_tier'].includes(item.price_selection_mode)) {
      hydrated.spec_conversion = null
      hydrated.price_selection_mode = 'auto'
      hydrated.selected_price_row_key = ''
      hydrated.manual_price = false
      hydrated.unit_price = ''
      hydrated.price_source_json = ''
      hydrated.price_missing = true
    }
    const publicationMode = orderProductPublicationMode(
      family || product || flatProduct || hydrated,
      item.bean_list_publication_id,
    )
    const keepSampleSpec = sampleOrderEditing.value && Number(item.bom_spec_id || 0) > 0
    const keepFrozenPublication = keepSampleSpec || shouldKeepFrozenOrderPublication(
      beanListVersionGroups.value,
      item.bean_list_publication_id,
      copyMode.value,
    )
    if (publicationMode === 'legacy') hydrated.spec_source = 'legacy_price_list'
    if (keepFrozenPublication) hydrated.historical_spec_readonly = true
    const hasConcreteSpecIdentity = Boolean(family?.__order_concrete_price_family)
      || Number(item.bom_spec_id || item.parent_product_id || flatProduct?.parent_product_id || 0) > 0
    if (!hasConcreteSpecIdentity) return hydrated

    const publicationID = Number(item.bean_list_publication_id || 0)
    const pricedSpec = orderFamilySpecForStoredItem(family || {}, item, publicationID)
    if (family) {
      assignProductFamilyHeader(hydrated, family)
    } else {
      hydrated.parent_product_id = Number(item.parent_product_id || (item.bom_spec_id ? item.product_id : flatProduct?.parent_product_id) || 0)
      hydrated.parent_product_name = flatProduct?.parent_product_name || item.product_name_snapshot || item.product_name || ''
      hydrated.product_name = item.customer_product_display_name_snapshot || item.product_name || hydrated.parent_product_name
      hydrated.product_query = hydrated.product_name
      hydrated.product_record_name = hydrated.parent_product_name
    }
    hydrated.product_id = Number(item.product_id || 0)
    hydrated.product_code = item.product_code_snapshot || pricedSpec?.product_code || pricedSpec?.sku_name || `SKU-${item.product_id}`
    hydrated.spec_source = publicationMode === 'legacy' ? 'legacy_price_list' : 'price_list_sku'
    hydrated.spec_mode = String(item.bom_spec_id || item.product_id || '')
    hydrated.spec_label = pricedSpec?.spec_label || item.spec_label || item.effective_sales_spec || (specNumber > 0 ? `${specNumber}g` : '历史规格')
    hydrated.spec_g = specNumber
    hydrated.custom_spec_g = ''
    hydrated.historical_spec_readonly = !pricedSpec || keepFrozenPublication
    hydrated.spec_invalid_message = !pricedSpec && !keepFrozenPublication ? '原商品规格在此报价表中不可用，请重新选择有效规格。' : ''
    if (pricedSpec && family) {
      Object.assign(hydrated, orderFamilyHydratedSpecRowPatch(
        family,
        pricedSpec,
        publicationID,
        {
          bom_spec_id: Number(item.bom_spec_id || 0),
          bom_variant_id: Number(item.bom_variant_id || 0),
          tier_id: item.tier_id || 'auto',
          price_selection_mode: item.price_selection_mode || (item.price_override === true || item.tier_id === 'manual' ? 'manual' : 'auto'),
          selected_price_row_key: item.selected_price_row_key || '',
          unit_price: item.unit_price ?? '',
          price_source_json: item.price_source_json || '',
          bean_list_publication_id: publicationID,
          bean_list_version_no: item.bean_list_version_no || hydrated.bean_list_version_no,
          manual_price: item.price_selection_mode === 'manual' || item.price_override === true || item.tier_id === 'manual',
          qty: Number(item.qty || 1),
          item_note: item.note || '',
          discount_type: item.discount_type || '',
          discount_value: item.discount_value || '',
        },
        keepFrozenPublication,
      ))
    }
    if (keepSampleSpec) {
      Object.assign(hydrated, sampleStoredItemIdentity(item))
      hydrated.sample_stored_manual = item.price_override === true || item.price_selection_mode === 'manual' || item.tier_id === 'manual'
      hydrated.sample_stored_spec_key = storedSampleSpecKey(hydrated)
    }
    return hydrated
  })
  if (!rows.value.length) rows.value = [newRow()]
  for (const row of rows.value) {
    const publicationID = Number(row.bean_list_publication_id || 0)
    if (publicationID <= 0) continue
    const listType = orderBeanListTypeForProductKind(row.product_kind)
    const field = beanListVersionField(listType)
    if (!Number(form[field] || 0)) form[field] = publicationID
    const group = beanListVersionGroupForPublicationID(beanListVersionGroups.value, publicationID)
    if (group?.options?.some((entry) => Number(entry.id || 0) === publicationID)) {
      selectedBeanListPublicationIDs[group.key] = publicationID
    }
  }
  form.bean_list_publication_id = Number(form.commercial_bean_list_publication_id || form.bean_list_publication_id || 0)
  repriceHydratedRows()
}

function repriceHydratedRows() {
  if (fulfillmentOrderEditing.value) return
  for (const row of rows.value) {
    if (preservesStoredOrderPrice(row)) continue
    if (row.historical_spec_readonly) {
      ensureRowBeanListVersion(row)
      continue
    }
    const family = productFamilyForRow(row)
    if (family?.__order_concrete_price_family || row.spec_source === 'price_list_sku') {
      const selected = selectedBeanListVersionOptionForProduct(family || row)
      const spec = family
        ? orderFamilySpecForStoredItem(family, row, selected?.id || row.bean_list_publication_id || 0)
        : null
      if (!spec) {
        invalidatePriceListSpecRow(row)
        continue
      }
      const frozen = {
        unit_price: row.unit_price,
        tier_id: row.tier_id,
        price_selection_mode: row.price_selection_mode,
        selected_price_row_key: row.selected_price_row_key,
        manual_price: row.manual_price,
        price_source_json: row.price_source_json,
        qty: row.qty,
        item_note: row.item_note,
        discount_type: row.discount_type,
        discount_value: row.discount_value,
      }
      Object.assign(row, orderFamilySpecRowPatch(family, spec, selected?.id || row.bean_list_publication_id || 0), frozen)
    }
    if (!row.product_id || row.manual_price) {
      ensureRowBeanListVersion(row)
      continue
    }
    const product = productForRow(row)
    if (!product || (retailOrder.value && !isConcretePriceListRow(row))) {
      ensureRowBeanListVersion(row)
      continue
    }
    syncPrice(row, { force: true })
  }
}

async function loadOrderProductCatalogTypes() {
  if (props.customerPortal) return
  try {
    const [groupsData, assignmentsData, featureSelectionData] = await Promise.all([
      apiGet('/api/business-groups'),
      apiGet('/api/business-group-assignments?usage_key=product_catalog&object_key=product'),
      apiGet('/api/business-group-feature-selections/product_catalog'),
    ])
    orderProductCatalogGroups.value = Array.isArray(groupsData?.rows)
      ? groupsData.rows
      : (Array.isArray(groupsData?.groups) ? groupsData.groups : (Array.isArray(groupsData) ? groupsData : []))
    orderProductCatalogAssignments.value = Array.isArray(assignmentsData?.rows)
      ? assignmentsData.rows
      : (Array.isArray(assignmentsData?.assignments) ? assignmentsData.assignments : (Array.isArray(assignmentsData) ? assignmentsData : []))
    orderProductCatalogFeatureSelection.value = featureSelectionData && typeof featureSelectionData === 'object'
      ? featureSelectionData
      : { feature_key: 'product_catalog', group_template_ids: [] }
  } catch (err) {
    orderProductCatalogGroups.value = []
    orderProductCatalogAssignments.value = []
    orderProductCatalogFeatureSelection.value = { feature_key: 'product_catalog', group_template_ids: [] }
  } finally {
    orderProductCatalogFeatureSelectionLoaded.value = true
  }
}

async function load() {
  const requestVersion = ++editorLoadVersion
  loading.value = true
  sampleOrderEditing.value = false
  editBlockReason.value = ''
  form.settlement_reason = ''
  error.value = ''
  ok.value = ''
  try {
    const url = new URL(props.customerPortal ? '/api/customer-processing/portal/order/form' : '/api/order/form', window.location.origin)
    if (props.customerPortal) url.searchParams.set('service', props.portalService)
    const contextCustomerID = Number(props.customerContextId || 0)
    if (contextCustomerID > 0) url.searchParams.set('customer_id', String(contextCustomerID))
    const propCopyID = Number(props.copyId || 0)
    const urlCopyID = new URL(window.location.href).searchParams.get('copy_id')
    const copyID = propCopyID > 0 ? String(propCopyID) : urlCopyID
    effectiveCopyID.value = Number(copyID || 0)
    const propEditID = Number(props.editId || 0)
    const urlEditID = new URL(window.location.href).searchParams.get('edit_id')
    const editID = copyID || (propEditID > 0 ? String(propEditID) : urlEditID)
    if (editID) url.searchParams.set('edit_id', editID)
    const data = await apiGet(url)
    if (requestVersion !== editorLoadVersion) return
    customers.value = data.customers || []
    sources.value = data.sources || []
    shipStatuses.value = data.ship_statuses || []
    payStatuses.value = data.pay_statuses || []
    orderTypes.value = data.order_types || []
    products.value = data.products || []
    productFamilies.value = normalizeOrderProductFamilies(
      data.product_families || [],
      products.value,
      data.product_bom_spec_options || [],
    )
    employees.value = data.employees || []
    logisticsCompanies.value = data.logistics_companies || []
    beanListVersionOptions.value = data.bean_list_version_options || []
    customerPublicUsages.value = data.customer_public_usages || []
    customerProductUsages.value = data.customer_product_usages || []
    await loadOrderProductCatalogTypes()
    if (requestVersion !== editorLoadVersion) return
    applyDefaultSelections(data)
    if (data.edit_mode) {
      const editData = { ...data.edit_data, edit_id: copyID ? 0 : data.edit_id }
      if (copyID) {
        editData.prepayment_amount = '0'
        editData.pay_status_id = defaultStatusID(payStatuses.value, ['未付款'])
        editData.payment_method = ''
        editData.ship_tracking_no = ''
        editData.ship_status_id = defaultStatusID(shipStatuses.value, ['未发货']) || editData.ship_status_id
        editData.logistics_company_id = 0
        editData.logistics_product_id = 0
        editData.payment_goods_amount = ''
        editData.payment_shipping_amount = ''
        editData.payment_voucher_asset_id = 0
        editData.payment_voucher = null
        editData.bean_list_publication_id = 0
        editData.commercial_bean_list_publication_id = 0
        editData.green_bean_list_publication_id = 0
        editData.drip_bean_list_publication_id = 0
      }
      form.edit_id = Number(editData.edit_id || 0)
      applyEditData(editData)
      if (fulfillmentOrderEditing.value) {
        const context = await apiGet(`/api/orders/${Number(editID)}/fulfillment`)
        if (requestVersion !== editorLoadVersion) return
        applyFulfillmentEditContext(context)
      } else {
        fulfillmentEditContext.value = null
      }
      if (!sampleOrderEditing.value && !fulfillmentOrderEditing.value) syncBeanListVersionForCustomer({ force: !!copyID })
      if (copyID) syncRowsForType({ priceListChanged: true })
    } else {
      fulfillmentEditData.value = null
      fulfillmentEditContext.value = null
      applyCustomerContextToNewOrder()
      syncBeanListVersionForCustomer({ force: true })
      if (props.deliveryPrefill) {
        applyEditData({ ...form, customer_id: Number(props.customerContextId || form.customer_id), delivery_mode: 'customer_stock', delivery_warehouse_code: props.deliveryPrefill.warehouse_code || '', items: props.deliveryPrefill.items || [] })
        syncRowsForType({ priceListChanged: true })
      }
    }
    loadedEditorOrderID.value = Number(editID || 0)
  } catch (err) {
    if (requestVersion === editorLoadVersion) error.value = err.message || '加载失败'
  } finally {
    if (requestVersion === editorLoadVersion) loading.value = false
  }
}

function applyFulfillmentEditContext(context) {
  fulfillmentEditContext.value = context
  const combined = { ...fulfillmentEditData.value, ...context, edit_id: Number(context.order_id || form.edit_id), portal_service_code: 'direct_ship' }
  applyEditData(combined)
  const destination = context.fees?.freight?.destination || context.fees || {}
  form.shipping_province = context.province || destination.province || ''
  form.shipping_city = context.city || destination.city || ''
  form.shipping_district = context.district || destination.district || ''
  fulfillmentManualDestination.value = false
  Object.assign(fulfillmentServiceAdjustment, { enabled: false, amount: (Number(context.fees?.service_fee_cents || 0) / 100).toFixed(2), reason: '' })
  fulfillmentPreview.value = null
}

async function previewFulfillmentEdit() {
  if (!fulfillmentEditContext.value || editBlockReason.value) return null
  let version = ++fulfillmentPreviewVersion
  fulfillmentPreviewLoading.value = true
  error.value = ''
  try {
    if (form.receiver_address !== fulfillmentEditContext.value.receiver_address && !fulfillmentManualDestination.value) {
      const address = form.receiver_address
      const resolved = await apiSend('/api/order/shipping-destination/resolve', { body: { address } })
      if (address !== form.receiver_address) return null
      if (resolved?.status !== 'resolved') throw new Error('无法唯一识别收件地区，请填写省、市、区县后重新核算')
      form.shipping_province = resolved.destination.province
      form.shipping_city = resolved.destination.city
      form.shipping_district = resolved.destination.district
      await nextTick()
      version = ++fulfillmentPreviewVersion
      fulfillmentPreviewLoading.value = true
    }
    const command = fulfillmentEditCommand(fulfillmentEditContext.value, form, rows.value, fulfillmentServiceAdjustment)
    const preview = await apiSend(`/api/orders/${Number(form.edit_id)}/fulfillment/preview`, { body: command })
    if (version !== fulfillmentPreviewVersion) return null
    fulfillmentPreview.value = preview
    return { preview, command }
  } catch (err) {
    if (version === fulfillmentPreviewVersion) error.value = err.message || '代发费用核算失败'
    return null
  } finally {
    if (version === fulfillmentPreviewVersion) fulfillmentPreviewLoading.value = false
  }
}

async function saveFulfillmentEdit() {
  saving.value = true
  ok.value = ''
  try {
    const checked = await previewFulfillmentEdit()
    if (!checked) return
    const data = await apiSend(`/api/orders/${Number(form.edit_id)}/fulfillment`, { method: 'PATCH', body: { ...checked.command, preview_token: checked.preview.preview_token } })
    await load()
    ok.value = '代发订单已保存'
    emit('saved', data)
  } catch (err) {
    error.value = err.message || '代发订单保存失败'
  } finally {
    saving.value = false
  }
}

function resetForBackfillContinuation() {
  if (props.fulfillmentMode) {resetCustomerContinuation(form);form.customer_request_id='';fulfillmentRecipientPaste.value=''}
  prepaymentRate.value = 0
  orderFreight.value=null
  form.freight_state=emptyFreightState()
  form.shipping_amount='0'
  form.edit_id = 0
  form.ship_tracking_no = ''
  form.prepayment_amount = '0'
  form.payment_goods_amount = ''
  form.payment_shipping_amount = ''
  form.payment_voucher_asset_id = 0
  form.notes = ''
  form.discount_amount = ''
  form.outsource_material_fee = ''
  form.outsource_roast_fee = ''
  form.outsource_packaging_fee = ''
  form.outsource_manual_fee = ''
  form.outsource_tax_fee = ''
  form.outsource_other_fee = ''
  rows.value = [newRow()]
  paymentVoucher.value = null
  paymentVoucherFile.value = null
  paymentVoucherCollapsed.value = false
  paymentVoucherPreviewOpen.value = false
  customerOpen.value = false
  beanListDrawerOpen.value = false
  clearAllFieldErrors()
  saveOrderEntryDraft()
}

async function save(options = {}) {
  if (saving.value || loading.value || editBlockReason.value) return
  if (fulfillmentOrderEditing.value) return saveFulfillmentEdit()
  if (!props.customerPortal && (staffCatalog.loading || staffCatalog.error)) { error.value = staffCatalog.error || '商品目录加载中，请稍后重试'; return }
  const continueBackfill = Boolean(options?.continueBackfill && canUseBackfillMode.value)
  saving.value = true
  error.value = ''
  ok.value = ''
  stockBatchNotice.value = ''
  clearAllFieldErrors()
  const saveVersion = editorLoadVersion
  try {
    const payload = buildOrderPayload({ form, rows: rows.value })
    if (form.delivery_mode!=='customer_stock' && !orderFreight.value?.edit_locked && (!form.edit_id || !props.customerPortal)) {
      const q=await freightEstimator.value?.preview();if(!q){error.value='请完善收件地区及运费配置后重试';return}
      Object.assign(payload,freightEstimator.value.payload())
      if(q.carrier_id){payload.logistics_company_id=q.carrier_id;payload.logistics_product_id=q.logistics_product_id;form.logistics_company_id=q.carrier_id;form.logistics_product_id=q.logistics_product_id}
    }

    payload.selected_price_table_ids = [...new Set([...beanListVersionGroups.value.map(group => Number(selectedBeanListVersionOptionByGroup(group)?.id || 0)), ...rows.value.filter(isFrozenConversion).map(row => Number(row.bean_list_publication_id))].filter(id => id > 0))]
    if (sampleOrderEditing.value) payload.selected_price_table_ids = [...new Set(rows.value.map(row => Number(row.bean_list_publication_id || 0)).filter(id => id > 0))]
    if (!payload.customer_id) {
      raiseSaveError('请选择客户', 'customer_id')
      return
    }
    if (payload.delivery_mode === 'customer_stock' && !deliveryWarehouseOptions.value.some(row => row.code === payload.delivery_warehouse_code)) {
      error.value = '请选择属于当前客户的启用成品仓库'; return
    }
    const missingCustomerProfile = selectedCustomerMissingProfileLabels()
    if (missingCustomerProfile.length) {
      raiseSaveError(`请先在客户资料维护${missingCustomerProfile.join('、')}`, 'customer_id')
      return
    }
    if (payload.delivery_mode!=='customer_stock' && (sampleOrderEditing.value || props.fulfillmentMode) && (!payload.receiver_name || !payload.receiver_phone || !payload.receiver_address)) {
      raiseSaveError('请在顶部收件信息补齐收件人、电话和地址', 'receiver_address')
      return
    }
    if (paymentMethodRequired.value && !payload.payment_method) {
      raiseSaveError('请选择收款方式', 'payment_method')
      return
    }
    if (logisticsRequired.value) {
      if (!payload.logistics_company_id) {
        raiseSaveError('请选择物流公司', 'logistics_company_id')
        return
      }
      if (!payload.logistics_product_id) {
        raiseSaveError('请选择物流产品', 'logistics_product_id')
        return
      }
    }
    if (paymentReceiptRequired.value) {
      if (!validSampleReceiptGoods(payload.payment_goods_amount, zeroGoodsSample.value, payload.payment_shipping_amount)) {
        raiseSaveError('请输入货款金额', 'payment_goods_amount')
        return
      }
      if (String(payload.payment_shipping_amount || '').trim() === '') {
        raiseSaveError('请输入运费金额', 'payment_shipping_amount')
        return
      }
      if (!payload.payment_voucher_asset_id) {
        raiseSaveError('请上传收款凭证', 'payment_voucher_asset_id')
        return
      }
    }
    if (!payload.product_id.length) {
      raiseSaveError('请至少录入一条有效明细', 'product_items')
      return
    }
    const missingPublishedPriceRowIndex = rows.value.findIndex(rowHasBlockingPrice)
    if (missingPublishedPriceRowIndex >= 0) {
      raiseSaveError(`第 ${missingPublishedPriceRowIndex + 1} 行：${rowPriceBlockingMessage(rows.value[missingPublishedPriceRowIndex])}`, 'product_items')
      return
    }
    if (props.customerPortal) {
      form.customer_request_id ||= crypto.randomUUID()
      saveOrderEntryDraft()
      Object.assign(payload,{request_id:form.customer_request_id,backfill_mode:backfillMode.value,portal_service_code:props.portalService,pay_status_id:0,ship_status_id:0,payment_method:'',payment_goods_amount:'',payment_shipping_amount:'',payment_voucher_asset_id:0,prepayment_amount:'0',shipping_amount:'0',discount_amount:'0',round_to_int:'',ship_tracking_no:'',source_warehouse:''})
      payload.unit_price=payload.unit_price.map(()=> '')
      payload.tier_id=payload.tier_id.map(()=> 'auto')
      payload.price_selection_mode=payload.price_selection_mode.map(()=> 'auto')
      payload.selected_price_row_key=payload.selected_price_row_key.map(()=> '')
      payload.discount_value=payload.discount_value?.map(()=> '')
      payload.discount_type=payload.discount_type?.map(()=> '')
    }
    if (props.fulfillmentMode) { form.customer_request_id ||= crypto.randomUUID(); payload.request_id=form.customer_request_id; payload.portal_service_code=props.portalService; payload.orders_scope='fulfillment' }
    payload.edit_revision=form.edit_revision
    const stockDecision = (props.customerPortal || props.fulfillmentMode || payload.delivery_mode === 'customer_stock') ? '' : await previewStockBatchesBeforeSave(payload)
    if (stockDecision) payload.stock_batch_decision = stockDecision
    const data = await apiSend(props.customerPortal ? '/api/customer-processing/portal/order' : props.fulfillmentMode ? '/api/fulfillment/order' : '/api/order', { body: payload })
    ok.value = data.confirmation_status === 'pending' ? `${data.order_no} 已保存，待确认` : data.order_no || '成功'
    if (data.stock_batch_used) {
      stockBatchNotice.value = '已使用成品批次，订单状态已进入“库存待发货”。'
    } else if (stockDecision === 'produce') {
      stockBatchNotice.value = '已选择不使用库存批次，订单会进入生产计划的库存不足/待生产流程。'
    }
    if (continueBackfill) {
      ok.value = `${data.order_no || '成功'}，可继续补录下一单`
      orderEntryDraftDisabled = false
      clearFormDraft(orderEntryDraftKey())
      resetForBackfillContinuation()
      return
    }
    orderEntryDraftDisabled = true
    clearFormDraft(orderEntryDraftKey())
    if (props.embedded && !props.customerPortal) {
      form.customer_request_id = ''
      if (!copyMode.value && editorAlive && saveVersion === editorLoadVersion) {
        await refreshSavedOrder(Number(data.order_id))
      }
      if (editorAlive) emit('saved', data)
    }
    if (props.customerPortal) {form.customer_request_id='';emit('saved',data);return}
    if (!props.embedded && data.redirect_url) window.location.href = data.redirect_url
  } catch (err) {
    if (!props.customerPortal && String(err.message || '').includes('bom_configuration_updated')) {
      const refreshed = await refreshStaffOrderCatalog()
      if (refreshed) { await nextTick(); await freightEstimator.value?.preview() }
      raiseSaveError(refreshed ? '商品生产配置已更新，已刷新规格及运费。商品、数量和报价选择已保留，请核对后重新保存。' : '商品生产配置已更新，目录刷新失败，请刷新后核对规格及运费再保存。')
    } else raiseSaveError(err.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function refreshSavedOrder(orderID) {
  const version = ++editorLoadVersion
  loading.value = true
  try {
    // Do not share an in-flight pre-save GET: the server may have changed
    // payment status/deposit and both the order and freight revisions.
    const data = await apiGet(`/api/order/form?edit_id=${orderID}&saved=${Date.now()}-${version}`)
    if (!editorAlive || version !== editorLoadVersion) return
    if (!data.edit_mode || Number(data.edit_id) !== orderID || !data.edit_data?.edit_revision) throw new Error('缺少最新订单版本')
    applyEditData({ ...data.edit_data, edit_id: orderID })
    loadedEditorOrderID.value = orderID
    paymentVoucherFile.value = null
    await nextTick()
  } catch (err) {
    if (!editorAlive || version !== editorLoadVersion) return
    // The write already committed. Do not call it a failed save or allow the
    // old form to submit again when only the read-back failed.
    editBlockReason.value = '订单已保存，但最新数据读取失败，请关闭后重新打开订单再编辑。'
    error.value = editBlockReason.value
  } finally {
    if (version === editorLoadVersion) loading.value = false
  }
}

async function previewStockBatchesBeforeSave(payload) {
  const preview = await apiSend('/api/order/stock-batch-preview', { body: payload })
  if (!preview?.sufficient || !preview?.has_batch_choices) return ''
  const message = stockBatchConfirmText(preview)
  const useBatch = window.confirm(message)
  return useBatch ? 'use_batch' : 'produce'
}

function stockBatchConfirmText(preview) {
  const lines = (preview.lines || []).map((line) => {
    const batches = (line.allocations || [])
      .map((batch) => `${batch.created_at === '库存余额' ? '库存余额 ' : ''}${batch.batch_code} 使用${batch.allocated_g}g`)
      .join('，')
    return `${line.product_name || '商品'} ${line.spec_g}g x ${line.need_units}件：${batches}`
  })
  return [
    '库存充足，可直接使用以下成品批次或库存余额发货。',
    ...lines,
    '确定使用以上批次吗？',
    '确定：订单状态改为“库存待发货”；取消：不使用库存，进入库存不足/待生产流程。',
  ].join('\n')
}

onMounted(async () => {
  document.addEventListener('pointerdown', handleOrderProductPointerDown)
  await load()
  if (!editorAlive) return
  const draftRestored = restoreOrderEntryDraft()
  if (draftRestored) {
    syncBeanListVersionForCustomer()
    syncRowsForType({ priceListChanged: true })
  } else {
    repriceHydratedRows()
  }
  await refreshStaffOrderCatalog()
})

onBeforeUnmount(saveOrderEntryDraft)
onBeforeUnmount(() => { editorAlive = false; editorLoadVersion++; fulfillmentPreviewVersion++ })
onBeforeUnmount(() => document.removeEventListener('pointerdown', handleOrderProductPointerDown))

watch(
  () => props.editId,
  (next, prev) => {
    if (!props.embedded || Number(next || 0) === Number(prev || 0)) return
    load()
  },
)

watch(
  () => props.copyId,
  (next, prev) => {
    if (!props.embedded || Number(next || 0) === Number(prev || 0)) return
    load()
  },
)

watch(
  () => props.customerContextId,
  () => applyCustomerContextToNewOrder(),
)

watch(() => props.recipientInfo, (info) => {
  if (!info || !props.fulfillmentMode) return
  if (info.name) form.receiver_name = info.name
  if (info.phone) form.receiver_phone = info.phone
  if (info.address) form.receiver_address = info.address
}, { immediate: true })

watch(canUseBackfillMode, (canUse) => {
  if (!canUse) backfillMode.value = false
})

watch(freeSampleSettlement, (free, previous) => {
 if (free) {
  form.pay_status_id = defaultStatusID(payStatuses.value, ['已收款','已付款']) || form.pay_status_id
 } else if (previous || form.settlement_reason === 'free_sample') {
  form.pay_status_id = defaultStatusID(payStatuses.value, ['未收款','未付款']) || form.pay_status_id
  form.settlement_reason = ''
 }
})

watch(
  paymentMethodRequired,
  (required) => {
    if (!required) form.payment_method = ''
    clearFieldErrorIfValid('payment_method')
  },
)

watch(logisticsRequired, ensureLogisticsDefaults)

watch(() => orderCatalogKey(form.customer_id, selectedStaffTableIDs()), key => {
  if (!props.customerPortal && !loading.value && key !== staffCatalog.key) void refreshStaffOrderCatalog()
})
watch(() => form.customer_id, () => clearFieldErrorIfValid('customer_id'))
watch(() => form.customer_id, (next, previous) => { if (!loading.value && previous && Number(next) !== Number(previous)) form.delivery_warehouse_code = ''; loadDeliveryWarehouses() })
watch(() => form.delivery_mode, (mode) => { if (mode !== 'customer_stock') form.delivery_warehouse_code = ''; else if (!deliveryWarehouses.value.length) loadDeliveryWarehouses() })
watch(() => form.payment_method, () => clearFieldErrorIfValid('payment_method'))
watch(() => form.logistics_company_id, () => {
  syncLogisticsProduct()
  clearFieldErrorIfValid('logistics_company_id')
})
watch(() => form.logistics_product_id, () => clearFieldErrorIfValid('logistics_product_id'))
watch(paymentReceiptRequired, (required) => {
  ensurePaymentDefaults()
  if (!required) {
    clearFieldErrorIfValid('payment_goods_amount')
    clearFieldErrorIfValid('payment_shipping_amount')
    clearFieldErrorIfValid('payment_voucher_asset_id')
  }
})
watch(() => form.payment_goods_amount, () => clearFieldErrorIfValid('payment_goods_amount'))
watch(() => form.payment_shipping_amount, () => clearFieldErrorIfValid('payment_shipping_amount'))
watch(() => form.payment_voucher_asset_id, () => clearFieldErrorIfValid('payment_voucher_asset_id'))
watch(() => form.receiver_address, address => {
  if (!fulfillmentOrderEditing.value || !fulfillmentEditContext.value) return
  fulfillmentManualDestination.value = false
  const original = address === fulfillmentEditContext.value.receiver_address ? fulfillmentEditContext.value : {}
  const destination = original.fees?.freight?.destination || original.fees || {}
  form.shipping_province = original.province || destination.province || ''
  form.shipping_city = original.city || destination.city || ''
  form.shipping_district = original.district || destination.district || ''
})
watch(() => [form.receiver_address, form.shipping_province, form.shipping_city, form.shipping_district, form.notes, form.order_date,
  rows.value.map(row => [row.product_id, row.bom_spec_id, row.bom_variant_id, row.qty]), { ...fulfillmentServiceAdjustment }], () => {
  if (!fulfillmentOrderEditing.value) return
  fulfillmentPreviewVersion++
  fulfillmentPreview.value = null
  fulfillmentPreviewLoading.value = false
}, { deep: true })
watch(() => [fulfillmentCurrentFees.value, fulfillmentGoodsCents.value, orderFreight.value], () => {
  if (!fulfillmentOrderEditing.value || !fulfillmentEditContext.value) return
  emit('fee-summary', { order_id: Number(form.edit_id), fees: fulfillmentCurrentFees.value, goods_cents: fulfillmentGoodsCents.value, freight: orderFreight.value || {} })
}, { immediate: true })
watch(itemsTotal, () => {
  clearFieldErrorIfValid('payment_goods_amount')
})
watch(() => [form.delivery_mode, form.edit_id, loadedEditorOrderID.value], () => {
  if (editorSnapshotPending.value) return
  emit('delivery-mode-change', { order_id: Number(props.copyId || form.edit_id || 0), delivery_mode: form.delivery_mode })
})
watch(()=>form.delivery_mode,mode=>{if(mode==='customer_stock'){form.shipping_amount='0'}else{try{form.shipping_amount=String(freightAmount(form.freight_state))}catch{}}})
watch(() => form.shipping_amount, (next, previous) => {
  if (paymentReceiptRequired.value && (String(form.payment_shipping_amount ?? '').trim() === '' || !form.edit_id && Number(form.payment_shipping_amount)===Number(previous||0))) {
    form.payment_shipping_amount = money(toNumber(form.shipping_amount))
  }
  clearFieldErrorIfValid('payment_shipping_amount')
})
watch(rows, () => clearFieldErrorIfValid('product_items'), { deep: true })

watch(
  () => form.commercial_bean_list_publication_id,
  (publicationID) => {
    form.bean_list_publication_id = Number(publicationID || 0)
  },
)
</script>

<style scoped>
.workflow-fields-inline { display: contents; }
.no-payment { color:#15803d; }

.quote-source-summary { display: flex; flex-wrap: wrap; gap: 6px 14px; font-size: 13px; color: #626d81; }
.prepayment-editor{display:flex;flex-direction:column;gap:6px}.prepayment-presets{display:flex;gap:8px}.prepayment-presets button{padding:5px 16px;color:#2563eb;background:#eff6ff;border:1px solid #bfdbfe;border-radius:6px}.prepayment-presets button.selected{background:#2563eb;color:white}.prepayment-editor small{color:#64748b;line-height:1.5}
.page { min-height: 100%; max-width: 100%; overflow-x: hidden; padding: 18px; display: grid; gap: 14px; background: #f6f7f9; color: #15171a; box-sizing: border-box; }
.page * { box-sizing: border-box; }
.page.embedded { min-height: auto; padding: 0; background: transparent; }
.fulfillment-recipient-section { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 10px; }
.delivery-choice { display:flex;flex-wrap:wrap;gap:12px;padding:14px;border:1px solid #d8e7dc;border-radius:8px;background:#f7fbf8; }
.delivery-choice label { flex:1;min-width:200px; }
.delivery-choice p { flex-basis:100%;margin:0; }
.fulfillment-recipient-section .full-span { grid-column: 1 / -1; }
.order-hero, .panel { background: #fff; border: 1px solid #e7e9ee; border-radius: 8px; box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04); }
.order-hero { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 20px; }
.eyebrow { margin: 0 0 4px; color: #6b7280; font-size: 12px; }
.order-hero h2, .section-title { margin: 0; font-size: 20px; font-weight: 700; letter-spacing: 0; }
.section-title { font-size: 17px; }
.hero-actions, .section-row, .save-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.section-actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; flex-wrap: wrap; min-width: 0; }
.bean-list-summary-list { min-width: min(420px, 100%); max-width: 720px; flex: 1 1 420px; display: grid; gap: 3px; }
.bean-list-summary { min-width: 0; display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 2px; color: #667085; line-height: 1.35; }
.bean-list-summary-label { color: #667085; font-size: 12px; }
.bean-list-summary-value { min-width: 0; color: #667085; font-size: 12px; overflow-wrap: anywhere; }
.total-pill { display: grid; gap: 2px; min-width: 132px; padding: 8px 12px; border: 1px solid #e5e7eb; border-radius: 8px; background: #fafafa; }
.total-pill span, .grand-line span, label span, .line-total span, .combo-option small { color: #667085; font-size: 12px; }
.total-pill strong, .grand-line strong { font-size: 20px; }
.total-pill small, .grand-line small { color: #667085; font-size: 12px; }
.panel { padding: 16px; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(190px, 1fr)); gap: 14px; }
.form-grid.compact { grid-template-columns: repeat(4, minmax(150px, 1fr)); }
.full-span { grid-column: 1 / -1; }
.backfill-hint { grid-column: 1 / -1; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; min-height: 38px; border: 1px solid #cfe3d5; border-radius: 6px; padding: 8px 10px; background: #f3faf5; color: #24533a; }
.backfill-hint.active { border-color: #8fbc8f; background: #edf8ef; }
.backfill-copy { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; min-width: 0; }
.backfill-hint strong { font-size: 14px; }
.backfill-hint span { color: #42624e; font-size: 12px; }
.backfill-toggle { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; width: auto; min-height: 32px; padding: 5px 8px; border: 1px solid #bfd9c6; border-radius: 6px; background: #fff; cursor: pointer; }
.backfill-toggle input { width: auto; min-height: auto; }
.save-actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; flex-wrap: wrap; }
.conditional-panel { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)); gap: 12px; align-items: end; padding: 12px; border: 1px solid #e4e7ec; border-radius: 8px; background: #fafbfc; }
.customer-profile-summary { display: grid; grid-template-columns: repeat(3, minmax(150px, 1fr)); gap: 10px; padding: 10px 12px; border: 1px solid #e4e7ec; border-radius: 8px; background: #fafbfc; }
.profile-summary-item { display: grid; gap: 3px; min-width: 0; }
.profile-summary-item span { color: #667085; font-size: 12px; }
.profile-summary-item strong { min-width: 0; font-size: 14px; color: #111827; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.profile-summary-item.missing strong { color: #be123c; }
.order-hero, .panel, .form-grid, .conditional-panel, .line-item { min-width: 0; }
.form-grid > *, .conditional-panel > *, .line-item > * { min-width: 0; }
.condition-title { align-self: center; font-weight: 700; color: #1f2937; }
.voucher-field { position: relative; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.voucher-field > span { color: #667085; font-size: 12px; }
.voucher-field small { color: #667085; font-size: 12px; }
.voucher-collapsed { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: center; }
.voucher-summary { min-width: 0; display: grid; gap: 2px; text-align: left; border: 1px solid #d7dbe3; background: #fff; color: #111827; }
.voucher-summary strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-upload-control { position: relative; width: 100%; min-height: 38px; display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 8px; border: 1px solid #d7dbe3; border-radius: 6px; padding: 6px 8px; background: #fff; cursor: pointer; }
.file-upload-control input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; pointer-events: none; }
.file-button { min-height: 26px; display: inline-flex; align-items: center; justify-content: center; border-radius: 999px; padding: 2px 12px; background: #eef0f3; color: #111827; font-size: 13px; white-space: nowrap; }
.file-name { min-width: 0; color: #344054; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
label { position: relative; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
input, select, textarea, button { font: inherit; }
input, select, textarea { width: 100%; border: 1px solid #d7dbe3; border-radius: 6px; padding: 8px 10px; min-height: 38px; background: #fff; box-sizing: border-box; }
input:focus, select:focus, textarea:focus { outline: 2px solid #9cc2ff; border-color: #4f8df7; }
textarea { resize: vertical; }
button { border-radius: 7px; padding: 8px 12px; cursor: pointer; white-space: nowrap; }
button:disabled { cursor: not-allowed; opacity: 0.5; }
.primary { border: 1px solid #111827; background: #111827; color: #fff; }
.secondary { border: 1px solid #c9ced8; background: #fff; color: #111827; }
.text-button { border: 0; background: transparent; color: #174ea6; padding: 0; min-height: 0; font-size: 12px; text-decoration: underline; }
.danger { color: #9f1239; }
.label-row { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.label-actions { display: inline-flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.field-shell { min-width: 0; }
.amount-suggestion-wrap { position: relative; min-width: 0; }
.amount-suggestion-popover { position: absolute; left: 8px; top: calc(100% + 6px); z-index: 6; min-height: 30px; padding: 5px 10px; border: 1px solid #b7d3ff; background: #eef5ff; color: #174ea6; box-shadow: 0 10px 24px rgba(23, 78, 166, 0.14); font-size: 12px; }
.field-invalid input, .field-invalid select, .field-invalid textarea, .field-invalid .file-upload-control { border-color: #f43f5e; box-shadow: 0 0 0 2px rgba(244, 63, 94, 0.12); }
.field-invalid > span:first-child, .field-invalid .label-row > span:first-child, .field-invalid.voucher-field > span:first-child { color: #be123c; }
.panel.field-invalid { border-color: #fda4af; box-shadow: 0 0 0 2px rgba(244, 63, 94, 0.08); }
.readonly-field input { background: #f8fafc; color: #4b5563; }
.notes { margin-top: 14px; }
.notice { border-radius: 8px; padding: 10px 12px; }
.notice.ok { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.notice.ok a { color: #0f3d99; font-weight: 700; text-decoration: none; }
.global-error-toast { position: fixed; top: 18px; right: 18px; z-index: 80; width: min(520px, calc(100vw - 36px)); display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; box-shadow: 0 18px 44px rgba(159, 18, 57, 0.18); }
.toast-text { display: grid; gap: 2px; min-width: 0; }
.toast-text strong { font-size: 13px; }
.toast-text span { overflow-wrap: anywhere; }
.toast-close { width: 28px; height: 28px; padding: 0; display: inline-grid; place-items: center; flex: 0 0 auto; border: 0; background: transparent; color: inherit; font-size: 20px; line-height: 1; }
.error { background: #fff1f2; border: 1px solid #fecdd3; color: #9f1239; }
.ok { background: #ecfdf3; border: 1px solid #bbf7d0; color: #166534; }
.warn { background: #fffbeb; border: 1px solid #fde68a; color: #92400e; }
.combobox { z-index: 2; }
.combobox.open { z-index: 30; }
.combo-menu { position: absolute; top: calc(100% + 4px); left: 0; right: 0; z-index: 20; max-height: 280px; overflow: auto; border: 1px solid #d7dbe3; border-radius: 8px; background: #fff; box-shadow: 0 14px 30px rgba(15, 23, 42, 0.16); padding: 6px; }
.combo-option { width: 100%; display: grid; gap: 2px; text-align: left; border: 0; background: transparent; padding: 8px; border-radius: 6px; }
.combo-option:hover { background: #f3f6fb; }
.product-kind-filter { position: sticky; top: -6px; z-index: 1; display: flex; flex-wrap: wrap; gap: 6px; margin: -6px -6px 4px; padding: 8px 6px 6px; border-bottom: 1px solid #edf0f5; background: #fff; }
.product-kind-filter-option { min-height: 28px; padding: 4px 10px; border: 1px solid #d7dbe3; border-radius: 999px; background: #fff; color: #475467; font: inherit; font-size: 12px; line-height: 1.2; cursor: pointer; }
.product-kind-filter-option:hover { background: #f3f6fb; }
.product-kind-filter-option.active { border-color: #2563eb; background: #eff6ff; color: #1d4ed8; font-weight: 600; }
.kind-badge { display: inline-flex; align-items: center; min-height: 18px; padding: 1px 6px; border-radius: 4px; font-size: 12px; font-weight: 600; margin-left: 4px; }
.kind-price-list { color: #1d4ed8; background: #eff6ff; border: 1px solid #bfdbfe; }
.kind-roasted { color: #8a4b12; background: #fff3df; border: 1px solid #f3c67c; }
.kind-green { color: #12613a; background: #e8f7ee; border: 1px solid #8bd4a6; }
.kind-drip { color: #1f4b7a; background: #eaf3ff; border: 1px solid #9bc4ef; }
.kind-instant { color: #6b3f16; background: #f5efe6; border: 1px solid #cba77d; }
.combo-empty { padding: 12px; color: #667085; font-size: 13px; }
.line-list { display: grid; gap: 10px; margin-top: 12px; }
.line-actions { display: flex; justify-content: flex-start; padding-top: 12px; }
.line-item { display: grid; grid-template-columns: minmax(240px, 1.35fr) minmax(160px, 0.85fr) minmax(90px, 0.45fr) minmax(145px, 0.7fr) minmax(150px, 0.75fr) minmax(100px, 0.5fr) auto; align-items: end; gap: 12px; padding: 12px; border: 1px solid #edf0f5; border-radius: 8px; background: #fcfcfd; }
.product-cell { z-index: 3; }
.spec-control, .price-control { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: center; }
.discount-inputs { display: grid; gap: 6px; }
.spec-control input { min-width: 86px; }
.icon-button { width: 38px; height: 38px; padding: 0; display: inline-grid; place-items: center; border: 1px solid #c9ced8; background: #fff; }
.line-total { display: grid; gap: 3px; padding-bottom: 2px; }
.line-total strong { font-size: 18px; }
.line-total small { color: #667085; font-size: 12px; }
.line-total small.price-missing-warning { color: #b42318; font-weight: 700; }
.bean-list-version-meta { position: relative; display: inline-flex; align-items: center; gap: 5px; width: fit-content; }
.bean-list-version-meta.stale { color: #b42318; font-weight: 700; }
.bean-list-version-warning { width: 18px; height: 18px; padding: 0; display: inline-grid; place-items: center; border: 1px solid #fda29b; border-radius: 50%; background: #fff1f0; color: #b42318; font-size: 12px; font-weight: 800; line-height: 1; }
.bean-list-version-tip { position: absolute; left: 0; top: calc(100% + 4px); z-index: 8; display: none; white-space: nowrap; border-radius: 5px; background: #111827; color: #fff; padding: 5px 8px; font-size: 12px; font-weight: 600; box-shadow: 0 8px 20px rgba(15, 23, 42, 0.18); }
.bean-list-version-meta:hover .bean-list-version-tip,
.bean-list-version-meta.open .bean-list-version-tip,
.bean-list-version-warning:focus + .bean-list-version-tip { display: inline-block; }
.line-note { grid-column: 1 / -1; }
.tier-prices { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 8px; padding-top: 2px; }
.tier-price-chip { display: grid; grid-template-columns: auto auto; align-items: center; gap: 8px; min-height: 32px; border: 1px solid #d7dbe3; background: #fff; color: #344054; border-radius: 7px; padding: 6px 8px; font-size: 12px; }
.tier-price-chip strong { color: #111827; }
.tier-price-chip.active { border-color: #4f8df7; background: #eef5ff; color: #174ea6; }
.checkline { flex-direction: row; align-items: center; gap: 8px; padding-top: 25px; }
.checkline input { width: auto; min-height: auto; }
.footer-panel { display: grid; gap: 12px; }
.grand-line { display: grid; gap: 2px; }
.manual { border-top: 1px solid #edf0f5; padding-top: 10px; color: #4b5563; font-size: 13px; }
.manual summary { cursor: pointer; font-weight: 700; color: #111827; }
.manual ul { margin: 8px 0 0; padding-left: 18px; }
.drawer-mask { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.28); display: flex; justify-content: flex-end; z-index: 40; }
.drawer { width: min(480px, 100%); height: 100%; overflow: auto; background: #fff; border-left: 1px solid #d7dbe3; padding: 16px; box-shadow: -12px 0 28px rgba(15, 23, 42, 0.12); }
.drawer-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
.drawer-head h3 { margin: 0; font-size: 18px; }
.drawer-grid { display: grid; gap: 12px; margin-top: 12px; }
.wide-field textarea, .drawer-grid input, .drawer-grid select { width: 100%; }
.check-field { display: flex; align-items: center; gap: 8px; }
.check-field input { width: auto; }
.parse-button { margin-top: 8px; }
.drawer-actions { display: flex; justify-content: flex-end; margin-top: 14px; }
.drawer-help { margin: 0 0 14px; color: #667085; font-size: 13px; }
.bean-list-picker-list { display: grid; gap: 14px; }
.bean-list-picker-list small { color: #667085; font-size: 12px; }
.voucher-preview-overlay { position: fixed; inset: 0; z-index: 90; display: grid; place-items: center; padding: 20px; background: rgba(15, 23, 42, 0.42); }
.voucher-preview-dialog { width: min(920px, 100%); max-height: min(86vh, 860px); display: grid; gap: 12px; overflow: auto; border-radius: 10px; border: 1px solid #d7dbe3; background: #fff; padding: 16px; box-shadow: 0 24px 80px rgba(15, 23, 42, 0.28); }
.voucher-preview-dialog img, .voucher-preview-dialog iframe { width: 100%; max-height: 72vh; border: 0; object-fit: contain; background: #f8fafc; }
.voucher-preview-dialog iframe { min-height: 70vh; }

@media (max-width: 1100px) {
  .line-item { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 760px) {
  .page { padding: 12px; overflow-x: hidden; }
  .order-hero, .section-row, .save-row { align-items: stretch; flex-direction: column; }
  .form-grid, .form-grid.compact, .line-item { grid-template-columns: 1fr; }
  .customer-profile-summary { grid-template-columns: 1fr; }
  .section-actions { width: 100%; justify-content: stretch; }
  .section-actions button { width: 100%; }
  .bean-list-summary-list { width: 100%; min-width: 0; max-width: none; flex-basis: 100%; }
  .save-actions { width: 100%; justify-content: stretch; }
  .save-actions button { flex: 1 1 180px; }
  .conditional-panel { grid-template-columns: 1fr; align-items: stretch; padding: 12px; }
  .global-error-toast { --notice-stack-offset: var(--kferp-notice-stack-space, 0px); top: calc(max(12px, env(safe-area-inset-top)) + var(--notice-stack-offset)); left: max(12px, env(safe-area-inset-left)); right: max(12px, env(safe-area-inset-right)); width: auto; max-width: none; }
  .hero-actions { width: 100%; }
  .file-upload-control { grid-template-columns: auto minmax(0, 1fr); }
  .voucher-collapsed { grid-template-columns: 1fr; }
  .voucher-preview-overlay { align-items: start; padding: max(12px, env(safe-area-inset-top)) max(12px, env(safe-area-inset-right)) max(12px, env(safe-area-inset-bottom)) max(12px, env(safe-area-inset-left)); }
  .tier-price-chip { max-width: 100%; grid-template-columns: minmax(0, 1fr) auto; }
  .tier-price-chip span { overflow-wrap: anywhere; }
}
</style>
