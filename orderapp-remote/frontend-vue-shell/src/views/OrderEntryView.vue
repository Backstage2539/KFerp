<template>
  <div class="page" :class="{ embedded: props.embedded }">
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
          <input v-model.trim="form.document_date" type="date" />
        </label>

        <label>
          <span>订单日期</span>
          <input v-model.trim="form.order_date" type="date" />
        </label>

        <label class="customer-combobox combobox" :class="{ open: customerOpen }">
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

        <label class="readonly-field">
          <span>客户负责人</span>
          <input :value="selectedCustomerResponsibleLabel" placeholder="先在客户资料指定负责人" readonly />
        </label>

        <div class="customer-profile-summary full-span">
          <div v-for="item in selectedCustomerProfileSummary" :key="item.key" class="profile-summary-item" :class="{ missing: item.missing }">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </div>
        </div>

        <label>
          <span>付款状态</span>
          <select v-model.number="form.pay_status_id">
            <option v-for="item in payStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>

        <label :class="{ 'field-invalid': hasFieldError('payment_method') }" data-error-field="payment_method">
          <span>收款方式</span>
          <select v-model.trim="form.payment_method" :disabled="!paymentMethodRequired">
            <option value="">选择收款方式</option>
            <option v-for="item in orderReceiptMethodOptions" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>

        <label>
          <span>发货状态</span>
          <select v-model.number="form.ship_status_id">
            <option v-for="item in shipStatuses" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>

        <label>
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

        <div v-if="paymentReceiptRequired" class="conditional-panel full-span">
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
            <span>运费金额</span>
            <div class="amount-suggestion-wrap">
              <input v-model.trim="form.payment_shipping_amount" type="number" min="0" step="0.01" />
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
          <button class="secondary" type="button" @click="openBeanListDrawer" :disabled="!canOpenBeanListDrawer">选择豆单</button>
          <div class="bean-list-summary-list">
            <small v-for="item in selectedBeanListSummaryItems" :key="item.type" class="bean-list-summary">
              <span class="bean-list-summary-label">{{ item.label }}：</span>
              <span class="bean-list-summary-value">{{ item.versionLabel }}</span>
            </small>
          </div>
        </div>
      </div>
      <div class="line-list">
        <article v-for="(row, idx) in rows" :key="row.key" class="line-item">
          <label class="product-combobox combobox product-cell" :class="{ open: row.product_open }">
            <span>商品</span>
            <input
              v-model.trim="row.product_query"
              type="search"
              placeholder="选择商品"
              autocomplete="off"
              @focus="row.product_open = true"
              @input="clearProduct(row)"
              @keydown.down.prevent="row.product_open = true"
            />
            <div v-if="row.product_open" class="combo-menu">
              <button
                v-for="product in productOptions(row)"
                :key="product.id"
                type="button"
                class="combo-option"
                @mousedown.prevent="chooseProduct(row, product)"
              >
                <strong>{{ product.name }} <span class="kind-badge" :class="productKindBadgeClass(product)">{{ productKindLabel(product) }}</span></strong>
                <small v-if="product.tiers?.length">{{ product.tiers.length }} 个价格梯度</small>
              </button>
              <div v-if="!productOptions(row).length" class="combo-empty">没有匹配商品</div>
            </div>
          </label>

          <label>
            <span>{{ isDripRow(row) ? '单位' : '规格' }}</span>
            <div class="spec-control">
              <select v-if="isDripRow(row)" v-model="row.sales_unit" @change="onDripUnitChange(row)">
                <option v-for="option in dripUnitOptionsForRow(row)" :key="option.value" :value="option.value">
                  {{ option.spec }}
                </option>
              </select>
              <select v-else v-model="row.spec_mode" @change="syncPrice(row)">
                <option value="">选择规格</option>
                <option v-for="option in specOptions(row)" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
              <input
                v-if="row.spec_mode === CUSTOM_SPEC_VALUE"
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
            <span>数量</span>
            <input v-model.number="row.qty" type="number" min="1" step="1" @input="syncPrice(row)" />
          </label>

          <label>
            <span>{{ retailOrder ? '单价' : `单价（${priceUnitLabel(row)}）` }}</span>
            <div class="price-control">
              <input
                v-model.trim="row.unit_price"
                type="number"
                min="0"
                step="0.01"
                placeholder="0.00"
                @input="markManualPrice(row)"
              />
              <button class="icon-button" type="button" title="恢复自动价格" @click="resetAutoPrice(row)">↺</button>
            </div>
          </label>

          <label class="discount-control">
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
            <small v-if="row.tier_below_min" class="tier-warning">低于最低梯度，已按最低档 {{ row.tier_price_label || '价格' }} 计价</small>
            <small
              v-if="row.product_id && row.bean_list_version_no"
              class="bean-list-version-meta"
              :class="{ stale: isRowBeanListVersionStale(row), open: row.bean_list_version_tip_open }"
            >
              <span>豆单版本：{{ row.bean_list_version_no }}</span>
              <button
                v-if="isRowBeanListVersionStale(row)"
                class="bean-list-version-warning"
                type="button"
                aria-label="非新版本豆单"
                title="非新版本豆单"
                @click.stop="toggleBeanListVersionTip(row)"
              >!</button>
              <span v-if="isRowBeanListVersionStale(row)" class="bean-list-version-tip" role="tooltip">非新版本豆单</span>
            </small>
          </div>

          <button class="secondary danger" type="button" @click="removeRow(idx)" :disabled="rows.length === 1">删除</button>

          <label class="line-note">
            <span>条目备注</span>
            <input v-model.trim="row.item_note" placeholder="如贴标、磨粉、特殊包装" />
          </label>

          <div v-if="tierRows(row).length" class="tier-prices">
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
      <div class="section-title">费用</div>
      <div class="form-grid compact">
        <label>
          <span>运费</span>
          <input v-model.trim="form.shipping_amount" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>优惠</span>
          <input v-model.trim="form.discount_amount" type="number" min="0" step="0.01" />
        </label>
        <label>
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
            v-if="backfillMode && canUseBackfillMode"
            class="secondary"
            type="button"
            @click="save({ continueBackfill: false })"
            :disabled="saving"
          >
            保存并查看订单
          </button>
          <button
            v-if="backfillMode && canUseBackfillMode"
            class="primary"
            type="button"
            @click="save({ continueBackfill: true })"
            :disabled="saving"
          >
            保存并继续补录
          </button>
          <button
            v-else
            class="primary"
            type="button"
            @click="save({ continueBackfill: false })"
            :disabled="saving"
          >
            保存订单
          </button>
        </div>
      </div>
      <details class="manual">
        <summary>录单手册</summary>
        <ul>
          <li>客户和商品输入框支持名称、拼音和首字母搜索。</li>
          <li>录单时可点“新增客户”打开右侧抽屉，粘贴收件信息后可解析姓名、联系电话和地址。</li>
          <li>选择客户后会带入客户档案中的默认来源和订单类型。</li>
          <li>客户有历史订单时，商品下拉会把常用商品排在最前面。</li>
          <li>来源、客户类型和订单类型只在客户资料维护，录单选择客户后只读展示。</li>
          <li>商品明细区点击“选择豆单”可切换熟豆、生豆、挂耳已发布豆单；客户没有自定义豆单时使用公共豆单。</li>
          <li>常用规格：36g、80g、100g、227g、454g、500g、1000g、2.5kg。</li>
          <li>挂耳产品可按袋或盒录单，盒价会按发布的挂耳价格梯度自动匹配。</li>
          <li>新订单默认已付款、未发货；商品单价会随规格和数量匹配价格梯度。</li>
          <li>需要临时改价时直接修改单价，点击 ↺ 恢复自动梯度价。</li>
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
          <h3>豆单选择</h3>
          <button class="secondary" type="button" @click="closeBeanListDrawer">关闭</button>
        </div>
        <p class="drawer-help">可分别选择熟豆、生豆、挂耳已发布豆单。客户没有自定义豆单时，系统使用对应公共豆单。</p>
        <div class="bean-list-picker-list">
          <label v-for="item in orderBeanListTypes" :key="item.type">
            <span>{{ item.label }}</span>
            <select
              v-if="customerBeanListVersionOptionsByType(item.type).length"
              :value="form[beanListVersionField(item.type)]"
              @change="setBeanListVersion(item.type, $event.target.value)"
            >
              <option v-for="option in customerBeanListVersionOptionsByType(item.type)" :key="option.id" :value="option.id">
                {{ beanListVersionLabel(option) }}
              </option>
            </select>
            <input v-else value="暂无已发布豆单" readonly />
            <small>{{ beanListDrawerHint(item.type) }}</small>
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
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { clearFormDraft, FORM_DRAFT_SCOPES, readFormDraft, saveFormDraft } from '../lib/form-draft-cache'
import {
  CUSTOM_SPEC_VALUE,
  beanListVersionOptionsForCustomer,
  buildOrderPayload,
  defaultWholesaleSpec,
  defaultStatusID,
  dripSalesUnitSpec,
  dripTierPriceRows,
  filterProductsForCustomer,
  filterOptions,
  isOrderTierActive,
  lineTotal,
  needsTrailingBlankOrderLine,
  normalizeSpecG,
  orderRowPriceUnit,
  orderReceiptMethodOptions,
  orderTotalPreview,
  productKindBadgeClass,
  productKindLabel,
  requiresOrderPaymentMethod,
  resolveWholesaleTierPrice,
  retailPackagePrice,
  retailSpecOptions,
  rowUsesStaleBeanListPublication,
  sortProductsByCustomerUsage,
  syncDripTierPrice,
  toInt,
  toNumber,
  wholesaleTierPriceRows,
  wholesaleSpecOptions,
} from '../lib/order-entry'
import { parseRecipientText } from '../lib/customer-recipient'
import { dripUnitOptions, isDripProduct } from '../lib/drip-product'
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'

const props = defineProps({
  editId: { type: [Number, String], default: 0 },
  copyId: { type: [Number, String], default: 0 },
  embedded: { type: Boolean, default: false },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const emit = defineEmits(['close', 'saved'])
const ORDER_ENTRY_DRAFT_SCOPE = FORM_DRAFT_SCOPES.orderEntry
let orderEntryDraftDisabled = false

const loading = ref(false)
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
const employees = ref([])
const logisticsCompanies = ref([])
const beanListVersionOptions = ref([])
const customerPublicUsages = ref([])
const customerProductUsages = ref([])
const rows = ref([newRow()])
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
const effectiveCopyID = ref(0)
const backfillMode = ref(false)
const customerTypeOptions = [
  { value: 'retail', label: '零售客户' },
  { value: 'ecommerce', label: '电商客户' },
  { value: 'wholesale', label: '批发客户' },
]

const form = reactive({
  edit_id: 0,
  document_date: '',
  order_date: '',
  customer_id: 0,
  source_id: 0,
  order_type_id: 0,
  pay_status_id: 0,
  payment_method: '',
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
})

function newRow() {
  return {
    key: `${Date.now()}-${Math.random()}`,
    product_query: '',
    product_open: false,
    product_id: 0,
    product_name: '',
    product_kind: 'roasted_bean',
    tier_id: 'auto',
    bean_list_publication_id: 0,
    bean_list_version_no: '',
    bean_list_version_tip_open: false,
    unit_price: '',
    price_unit: '',
    price_unit_suffix: '',
    price_unit_g: 0,
    tier_price_label: '',
    tier_below_min: false,
    manual_price: false,
    spec_mode: '',
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
  return `${ORDER_ENTRY_DRAFT_SCOPE}:${workspace}:${customerID || 'all'}:new`
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
  if (!draft) return
  Object.assign(form, draft.form || {})
  rows.value = Array.isArray(draft.rows) && draft.rows.length
    ? draft.rows.map((row) => ({ ...newRow(), ...row, product_open: false }))
    : [newRow()]
  customerQuery.value = draft.customerQuery || ''
  customerDrawerOpen.value = Boolean(draft.customerDrawerOpen)
  customerPaste.value = draft.customerPaste || ''
  Object.assign(customerForm, emptyCustomerForm(), draft.customerForm || {})
  backfillMode.value = Boolean(draft.backfillMode)
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
  itemsTotal: itemsTotal.value,
  shippingAmount: form.shipping_amount,
  discountAmount: form.discount_amount,
  roundToInt: form.round_to_int,
}))
const orderTotalHintText = computed(() => `货款 ${money(orderTotalPreviewValue.value.goodsAmount)} · 物流 ${money(orderTotalPreviewValue.value.logisticsAmount)}`)
const filteredCustomers = computed(() => filterOptions(customers.value, customerQuery.value).slice(0, 20))
const paymentMethodRequired = computed(() => requiresOrderPaymentMethod(form, payStatuses.value))
const selectedPayStatusName = computed(() => optionName(payStatuses.value, form.pay_status_id))
const selectedShipStatusName = computed(() => optionName(shipStatuses.value, form.ship_status_id))
const paymentReceiptRequired = computed(() => {
  const name = selectedPayStatusName.value
  return name.includes('已收款') || name.includes('已付款') || name.includes('已支付')
})
const paymentGoodsAmountSuggestion = computed(() => money(itemsTotal.value))
const paymentShippingAmountSuggestion = computed(() => money(toNumber(form.shipping_amount)))
const showPaymentGoodsAmountSuggestion = computed(() => paymentReceiptRequired.value && itemsTotal.value > 0)
const showPaymentShippingAmountSuggestion = computed(() => paymentReceiptRequired.value)
const logisticsRequired = computed(() => selectedShipStatusName.value.includes('已发货'))
const selectedLogisticsProducts = computed(() => {
  const company = logisticsCompanies.value.find((item) => Number(item.id || 0) === Number(form.logistics_company_id || 0))
  return (company?.products || []).filter((item) => item.active !== false)
})
const copyMode = computed(() => Number(props.copyId || effectiveCopyID.value || 0) > 0)
const canUseBackfillMode = computed(() => !props.embedded && !form.edit_id && !copyMode.value)
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
const customerBeanListVersionOptions = computed(() => {
  return beanListVersionOptionsForCustomer(beanListVersionOptions.value, form.customer_id)
})
const canOpenBeanListDrawer = computed(() => {
  return Number(form.customer_id || 0) > 0 || customerBeanListVersionOptions.value.length > 0
})
const selectedBeanListSummaryItems = computed(() => orderBeanListTypes
  .map((item) => {
    const selected = selectedBeanListVersionOptionByType(item.type)
    return {
      type: item.type,
      label: item.label,
      versionLabel: selected ? beanListVersionLabel(selected) : '暂无',
    }
  }))

function customerTypeLabel(value) {
  return customerTypeOptions.find((item) => item.value === String(value || '').trim())?.label || ''
}

function selectedCustomerMissingProfileLabels() {
  if (!selectedCustomer.value) return []
  return selectedCustomerProfileSummary.value.filter((item) => item.missing).map((item) => item.label)
}
function productByID(id) {
  return products.value.find((item) => Number(item.id) === Number(id)) || null
}

function isDripRow(row) {
  return row?.product_kind === 'drip_bag' || isDripProduct(productByID(row?.product_id))
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
  if (String(form.payment_shipping_amount || '').trim() === '') {
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
  if (fieldKey === 'payment_goods_amount') return !paymentReceiptRequired.value || toNumber(form.payment_goods_amount) > 0
  if (fieldKey === 'payment_shipping_amount') return !paymentReceiptRequired.value || String(form.payment_shipping_amount || '').trim() !== ''
  if (fieldKey === 'payment_voucher_asset_id') return !paymentReceiptRequired.value || Number(form.payment_voucher_asset_id || 0) > 0
  if (fieldKey === 'product_items') return hasValidProductLine()
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
  syncOrderHeaderFromCustomer(item)
  syncBeanListVersionForCustomer({ force: true })
  notifyWorkspaceCustomerChanged(form.customer_id)
}

function syncOrderHeaderFromCustomer(customer = selectedCustomer.value, options = {}) {
  const nextSourceID = Number(customer?.default_source_id || 0)
  const nextOrderTypeID = Number(customer?.default_order_type_id || 0)
  const orderTypeChanged = Number(form.order_type_id || 0) !== nextOrderTypeID
  form.source_id = nextSourceID
  form.order_type_id = nextOrderTypeID
  if (orderTypeChanged && options.syncRows !== false) syncRowsForType()
}

function beanListVersionLabel(item) {
  if (!item) return ''
  const owner = item.is_customer_owned ? '客户豆单' : '公共豆单'
  const version = item.version_no || item.label || `#${item.id}`
  const time = item.published_at ? ` · ${item.published_at}` : ''
  return `${owner} ${version}${time}`
}

function beanListVersionField(listType) {
  if (listType === 'green') return 'green_bean_list_publication_id'
  if (listType === 'drip') return 'drip_bean_list_publication_id'
  return 'commercial_bean_list_publication_id'
}

function orderBeanListTypeForProductKind(productKind) {
  if (productKind === 'green_bean') return 'green'
  if (productKind === 'drip_bag') return 'drip'
  return 'commercial'
}

function customerBeanListVersionOptionsByType(listType) {
  return customerBeanListVersionOptions.value.filter((item) => String(item.list_type || 'commercial') === listType)
}

function selectedBeanListPublicationIDsByType() {
  const out = {}
  for (const beanListType of orderBeanListTypes) {
    const item = selectedBeanListVersionOptionByType(beanListType.type)
    if (!item) continue
    const id = Number(item.id || 0)
    if (id <= 0) continue
    const listType = String(item.list_type || beanListType.type || 'commercial')
    if (!out[listType]) out[listType] = []
    out[listType].push(id)
  }
  return out
}

function customerOwnedBeanListPublicationIDsByType() {
  return selectedBeanListPublicationIDsByType()
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

function setBeanListVersion(listType, value) {
  const field = beanListVersionField(listType)
  form[field] = Number(value || 0)
  if (listType === 'commercial') {
    form.bean_list_publication_id = form[field]
  }
  syncRowsForType()
}

function openBeanListDrawer() {
  if (!canOpenBeanListDrawer.value) {
    raiseSaveError('暂无可选择的已发布豆单')
    return
  }
  beanListDrawerOpen.value = true
}

function closeBeanListDrawer() {
  beanListDrawerOpen.value = false
}

function beanListDrawerHint(listType) {
  const rows = customerBeanListVersionOptionsByType(listType)
  if (!rows.length) return '没有可用的已发布豆单'
  if (rows.some((item) => item.is_customer_owned)) return '可选择客户自定义豆单版本'
  return '当前使用公共豆单'
}

function syncBeanListVersionForCustomer(options = {}) {
  for (const item of orderBeanListTypes) {
    const rows = customerBeanListVersionOptionsByType(item.type)
    const field = beanListVersionField(item.type)
    if (!rows.length) {
      form[field] = 0
      if (item.type === 'commercial') form.bean_list_publication_id = 0
      continue
    }
    const currentID = Number(form[field] || 0)
    if (!options.force && rows.some((row) => Number(row.id) === currentID)) continue
    const selected = rows.find((row) => row.is_default) || rows[0]
    form[field] = Number(selected?.id || 0)
    if (item.type === 'commercial') form.bean_list_publication_id = form[field]
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
    customer_type: customer.customer_type || 'retail',
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
      customer_type: customerForm.customer_type,
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

function productOptions(row) {
  return sortProductsByCustomerUsage(
    filterOptions(
      filterProductsForCustomer(
        products.value,
        form.customer_id,
        customerOwnedBeanListPublicationIDsByType(),
        customerPublicUsages.value,
      ),
      row.product_query,
    ),
    form.customer_id,
    customerProductUsages.value,
  ).slice(0, 30)
}

function clearProduct(row) {
  row.product_open = true
  row.product_id = 0
  row.product_name = ''
  row.product_kind = 'roasted_bean'
  row.tier_id = 'auto'
  row.bean_list_publication_id = 0
  row.bean_list_version_no = ''
  row.bean_list_version_tip_open = false
  row.unit_price = ''
  clearWholesalePriceMetadata(row)
  row.manual_price = false
  row.spec_mode = ''
  row.custom_spec_g = ''
  row.sales_unit = ''
  row.unit_bag_count = 0
  row.unit_bean_g = ''
  row.unit = '件'
}

function chooseProduct(row, product) {
  row.product_id = Number(product?.id || 0)
  row.product_name = product?.name || ''
  row.product_query = product?.name || ''
  row.product_open = false
  row.manual_price = false
  row.product_kind = product?.product_kind || 'roasted_bean'
  if (isDripProduct(product)) {
    row.sales_unit = 'bag'
    row.unit_bean_g = Number(product?.drip_bag_grams || 10)
    row.unit_bag_count = 1
    row.unit = '袋'
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
  const product = productByID(row.product_id)
  if (isDripProduct(product)) return []
  if (retailOrder.value) return retailSpecOptions(product, true)
  return wholesaleSpecOptions(product)
}

function syncRowsForType() {
  rows.value.forEach((row) => {
    if (!row.product_id) return
    row.manual_price = false
    if (isDripRow(row)) {
      syncPrice(row, { force: true })
      return
    }
    const options = specOptions(row)
    if (!options.some((option) => option.value === row.spec_mode)) {
      row.spec_mode = retailOrder.value ? (options[0]?.value || '') : defaultWholesaleSpec(productByID(row.product_id))
      row.custom_spec_g = ''
    }
    syncPrice(row, { force: true })
  })
}

function syncPrice(row, options = {}) {
  const product = productByID(row.product_id)
  if (!product) {
    row.unit_price = ''
    clearWholesalePriceMetadata(row)
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
    row.manual_price = false
    return
  }
  if (row.manual_price && !options.force) return
  if (retailOrder.value) {
    syncRowBeanListVersionFromSelection(row)
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
  row.price_unit = ''
  row.price_unit_suffix = ''
  row.price_unit_g = 0
  row.tier_price_label = ''
  row.tier_below_min = false
}

function applyResolvedWholesalePrice(row, price) {
  row.tier_id = price.tierID
  row.unit_price = price.unitPrice
  row.price_unit = price.priceUnit?.label || ''
  row.price_unit_suffix = price.priceUnit?.suffix || ''
  row.price_unit_g = Number(price.priceUnit?.unitG || 0)
  row.tier_price_label = price.tierPriceLabel || ''
  row.tier_below_min = Boolean(price.belowMinTier)
  ensureRowBeanListVersion(row, price)
}

function ensureRowBeanListVersion(row, price = {}) {
  const listType = orderBeanListTypeForProductKind(row.product_kind)
  const selected = selectedBeanListVersionOptionByType(listType)
  const publicationID = Number(price.beanListPublicationID || row.bean_list_publication_id || selected?.id || 0)
  row.bean_list_publication_id = publicationID
  row.bean_list_version_no = String(price.beanListVersionNo || row.bean_list_version_no || selected?.version_no || '').trim()
  if (!isRowBeanListVersionStale(row)) row.bean_list_version_tip_open = false
}

function syncRowBeanListVersionFromSelection(row) {
  const listType = orderBeanListTypeForProductKind(row.product_kind)
  const selected = selectedBeanListVersionOptionByType(listType)
  row.bean_list_publication_id = Number(selected?.id || 0)
  row.bean_list_version_no = String(selected?.version_no || '').trim()
  if (!isRowBeanListVersionStale(row)) row.bean_list_version_tip_open = false
}

function isRowBeanListVersionStale(row) {
  const listType = orderBeanListTypeForProductKind(row?.product_kind)
  return rowUsesStaleBeanListPublication(row, customerBeanListVersionOptionsByType(listType), listType)
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
  syncPrice(row, { force: true })
}

function dripUnitOptionsForRow(row) {
  return dripUnitOptions(productByID(row.product_id))
}

function markManualPrice(row) {
  row.manual_price = true
  row.tier_id = 'manual'
}

function resetAutoPrice(row) {
  row.manual_price = false
  syncPrice(row, { force: true })
}

function tierRows(row) {
  if (retailOrder.value) return []
  if (isDripRow(row)) return dripTierPriceRows(productByID(row.product_id), row)
  return wholesaleTierPriceRows(productByID(row.product_id), row)
}

function selectTier(row, tier) {
  if (isDripRow(row)) {
    row.manual_price = false
    row.tier_id = tier.id || 'auto'
    row.unit_price = String(tier.unitPrice || '')
    return
  }
  row.spec_mode = String(tier.specG || '')
  row.custom_spec_g = ''
  row.manual_price = false
  syncPrice(row, { force: true })
}

function isTierActive(row, tier) {
  return isOrderTierActive(row, tier)
}

function autoPriceLabel(row) {
  if (isDripRow(row)) return row.sales_unit === 'box' ? '挂耳盒价' : '挂耳袋价'
  if (retailOrder.value) return '零售价'
  if (row.tier_price_label) return `梯度 ${row.tier_price_label}`
  if (row.tier_id && row.tier_id !== 'auto' && row.tier_id !== 'manual') return `梯度 ${row.tier_id}`
  return '自动价'
}

function priceUnitLabel(row) {
  if (isDripRow(row)) return row.sales_unit === 'box' ? '元/盒' : '元/袋'
  return orderRowPriceUnit(row).label
}

function rowTotal(row) {
  return lineTotal(productByID(row.product_id), row, retailOrder.value)
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
  if (!data) return
  const editItems = Array.isArray(data.items) ? data.items : []
  const itemPublicationIDByType = (listType) => {
    const item = editItems.find((row) => orderBeanListTypeForProductKind(row.product_kind) === listType && Number(row.bean_list_publication_id || 0) > 0)
    return Number(item?.bean_list_publication_id || 0)
  }
  Object.assign(form, {
    edit_id: Number(data.edit_id || form.edit_id || 0),
    document_date: data.document_date || data.order_date || form.document_date,
    order_date: data.order_date || form.order_date,
    customer_id: Number(data.customer_id || 0),
    source_id: Number(data.source_id || 0),
    order_type_id: Number(data.order_type_id || 0),
    pay_status_id: Number(data.pay_status_id || 0),
    payment_method: data.payment_method || '',
    ship_status_id: Number(data.ship_status_id || 0),
    ship_method: data.ship_method || '',
    ship_tracking_no: data.ship_tracking_no || '',
    logistics_company_id: Number(data.logistics_company_id || 0),
    logistics_product_id: Number(data.logistics_product_id || 0),
    payment_goods_amount: data.payment_goods_amount || '',
    payment_shipping_amount: data.payment_shipping_amount || '',
    payment_voucher_asset_id: Number(data.payment_voucher_asset_id || 0),
    bean_list_publication_id: Number(data.bean_list_publication_id || 0),
    commercial_bean_list_publication_id: Number(data.commercial_bean_list_publication_id || data.bean_list_publication_id || itemPublicationIDByType('commercial') || 0),
    green_bean_list_publication_id: Number(data.green_bean_list_publication_id || itemPublicationIDByType('green') || 0),
    drip_bean_list_publication_id: Number(data.drip_bean_list_publication_id || itemPublicationIDByType('drip') || 0),
    notes: data.notes || '',
    shipping_amount: data.shipping_amount || '',
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
  syncOrderHeaderFromCustomer(selectedCustomer.value, { syncRows: false })
  paymentVoucher.value = data.payment_voucher || null
  paymentVoucherCollapsed.value = Boolean(paymentVoucher.value)
  customerQuery.value = optionName(customers.value, form.customer_id)
  rows.value = editItems.map((item) => {
    const spec = String(item.spec || '').replace(/g$/i, '')
    const product = productByID(item.product_id)
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
    return {
      ...newRow(),
      product_id: Number(item.product_id || 0),
      product_name: item.product_name || '',
      product_query: item.product_name || '',
      product_kind: productKind,
      tier_id: item.tier_id || 'auto',
      bean_list_publication_id: Number(item.bean_list_publication_id || 0),
      bean_list_version_no: item.bean_list_version_no || '',
      unit_price: item.unit_price || '',
      price_unit: '',
      price_unit_suffix: '',
      price_unit_g: 0,
      tier_price_label: '',
      tier_below_min: false,
      manual_price: item.tier_id === 'manual',
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
  })
  if (!rows.value.length) rows.value = [newRow()]
  for (const row of rows.value) {
    const publicationID = Number(row.bean_list_publication_id || 0)
    if (publicationID <= 0) continue
    const listType = orderBeanListTypeForProductKind(row.product_kind)
    const field = beanListVersionField(listType)
    if (!Number(form[field] || 0)) form[field] = publicationID
  }
  form.bean_list_publication_id = Number(form.commercial_bean_list_publication_id || form.bean_list_publication_id || 0)
  for (const row of rows.value) {
    if (!row.product_id || row.manual_price) {
      ensureRowBeanListVersion(row)
      continue
    }
    const product = productByID(row.product_id)
    if (!product || isDripProduct(product) || retailOrder.value) {
      ensureRowBeanListVersion(row)
      continue
    }
    applyResolvedWholesalePrice(row, resolveWholesaleTierPrice(product, row))
  }
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = new URL('/api/order/form', window.location.origin)
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
    customers.value = data.customers || []
    sources.value = data.sources || []
    shipStatuses.value = data.ship_statuses || []
    payStatuses.value = data.pay_statuses || []
    orderTypes.value = data.order_types || []
    products.value = data.products || []
    employees.value = data.employees || []
    logisticsCompanies.value = data.logistics_companies || []
    beanListVersionOptions.value = data.bean_list_version_options || []
    customerPublicUsages.value = data.customer_public_usages || []
    customerProductUsages.value = data.customer_product_usages || []
    applyDefaultSelections(data)
    if (data.edit_mode) {
      const editData = { ...data.edit_data, edit_id: copyID ? 0 : data.edit_id }
      if (copyID) {
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
      syncBeanListVersionForCustomer({ force: !!copyID })
    } else {
      applyCustomerContextToNewOrder()
      syncBeanListVersionForCustomer({ force: true })
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function resetForBackfillContinuation() {
  form.edit_id = 0
  form.ship_tracking_no = ''
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
  const continueBackfill = Boolean(options?.continueBackfill && canUseBackfillMode.value)
  saving.value = true
  error.value = ''
  ok.value = ''
  stockBatchNotice.value = ''
  clearAllFieldErrors()
  try {
    const payload = buildOrderPayload({ form, rows: rows.value })
    if (!payload.customer_id) {
      raiseSaveError('请选择客户', 'customer_id')
      return
    }
    const missingCustomerProfile = selectedCustomerMissingProfileLabels()
    if (missingCustomerProfile.length) {
      raiseSaveError(`请先在客户资料维护${missingCustomerProfile.join('、')}`, 'customer_id')
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
      if (toNumber(payload.payment_goods_amount) <= 0) {
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
    const stockDecision = await previewStockBatchesBeforeSave(payload)
    if (stockDecision) payload.stock_batch_decision = stockDecision
    const data = await apiSend('/api/order', { body: payload })
    ok.value = data.order_no || '成功'
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
    if (props.embedded) emit('saved', data)
    if (!props.embedded && data.redirect_url) window.location.href = data.redirect_url
  } catch (err) {
    raiseSaveError(err.message || '保存失败')
  } finally {
    saving.value = false
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
  await load()
  restoreOrderEntryDraft()
})

onBeforeUnmount(saveOrderEntryDraft)

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

watch(canUseBackfillMode, (canUse) => {
  if (!canUse) backfillMode.value = false
})

watch(
  paymentMethodRequired,
  (required) => {
    if (!required) form.payment_method = ''
    clearFieldErrorIfValid('payment_method')
  },
)

watch(logisticsRequired, ensureLogisticsDefaults)
watch(() => form.customer_id, () => clearFieldErrorIfValid('customer_id'))
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
watch(itemsTotal, () => {
  clearFieldErrorIfValid('payment_goods_amount')
})
watch(() => form.shipping_amount, () => {
  if (paymentReceiptRequired.value && String(form.payment_shipping_amount || '').trim() === '') {
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
.page { min-height: 100%; max-width: 100%; overflow-x: hidden; padding: 18px; display: grid; gap: 14px; background: #f6f7f9; color: #15171a; box-sizing: border-box; }
.page * { box-sizing: border-box; }
.page.embedded { min-height: auto; padding: 0; background: transparent; }
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
.kind-badge { display: inline-flex; align-items: center; min-height: 18px; padding: 1px 6px; border-radius: 4px; font-size: 12px; font-weight: 600; margin-left: 4px; }
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
.line-total small.tier-warning { color: #b42318; font-weight: 700; }
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
