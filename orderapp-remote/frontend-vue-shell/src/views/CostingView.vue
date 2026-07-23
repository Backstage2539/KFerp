<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>商品价格表</h2>
        <div class="actions">
          <button class="secondary" type="button" :disabled="loading" @click="loadBeanList">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div class="price-list-top-toolbar">
        <div class="price-list-toolbar-stat">
          <span>商品数</span>
          <strong>{{ customerScopedSkuCount }}</strong>
        </div>
        <label v-if="!isWorkspaceCustomerLocked" class="price-list-toolbar-scope">
          <span>价格表归属</span>
          <select v-model="versionListScope" aria-label="价格表归属">
            <option value="official">公共价格表</option>
            <option v-for="customer in customers" :key="`version-scope-${customer.id}`" :value="`customer:${customer.id}`">
              {{ customerOptionLabel(customer) }}
            </option>
          </select>
        </label>
        <div v-else class="price-list-toolbar-scope locked-scope">
          <span>价格表归属</span>
          <strong>{{ publicationScopeLabel(versionListScope) }}</strong>
        </div>
        <button class="secondary price-list-tier-template-button" type="button" @click="openTierTemplateDrawer()">管理阶梯模板</button>
      </div>
    </section>

    <section class="panel bean-list-version-panel">
      <div class="section-bar bean-list-version-head">
        <div class="bean-list-version-title-row">
          <button
            class="publication-list-collapse-toggle"
            type="button"
            :aria-label="publicationListCollapsed ? '展开已发布价格表' : '收起已发布价格表'"
            :title="publicationListCollapsed ? '展开已发布价格表' : '收起已发布价格表'"
            @click="publicationListCollapsed = !publicationListCollapsed"
          >
            <span aria-hidden="true">{{ publicationListCollapsed ? '⇊' : '⇈' }}</span>
          </button>
          <div class="section-title">已发布价格表</div>
        </div>
        <div class="actions">
          <button class="secondary compact" type="button" @click="publicationArchiveListCollapsed = !publicationArchiveListCollapsed">归档列表 {{ publicationArchiveListCollapsed ? `(${currentScopeArchivedPublicationRows.length})` : '收起' }}</button>
        </div>
      </div>

      <div v-show="!publicationListCollapsed" class="version-controls">
        <label>
          <span>商品类型</span>
          <select v-model.number="selectedProductTypeCategoryID" :disabled="!productPriceListTypeOptions.length">
            <option v-for="type in productPriceListTypeOptions" :key="type.key" :value="type.id">
              {{ type.label }}（{{ type.itemCount }}款）
            </option>
          </select>
        </label>
        <label>
          <span>搜索</span>
          <input v-model.trim="publicationListSearch" type="search" placeholder="搜索版本/客户/说明" />
        </label>
        <div class="version-summary">
          <span>当前发布</span>
          <strong>{{ versionListCurrentPublication?.version || '暂无' }}</strong>
        </div>
        <div class="version-summary">
          <span>版本数</span>
          <strong>{{ publicationListState.total }} / {{ currentScopePublicationRows.length }}</strong>
        </div>
        <div class="version-summary">
          <span>已归档</span>
          <strong>{{ currentScopeArchivedPublicationRows.length }}</strong>
        </div>
      </div>

      <div v-if="!publicationListCollapsed && isBeanListAdmin && currentScopePublicationRows.length" class="version-bulk-actions">
        <label class="table-select-all">
          <input
            type="checkbox"
            :checked="currentPagePublicationArchiveAllSelected"
            :aria-checked="currentPagePublicationArchiveSomeSelected && !currentPagePublicationArchiveAllSelected ? 'mixed' : String(currentPagePublicationArchiveAllSelected)"
            :disabled="!archiveSelectableCurrentPagePublicationRows.length || beanListArchiving"
            @change="toggleCurrentPagePublicationArchiveSelection($event.target.checked)"
          />
          <span>当前页可归档</span>
        </label>
        <button class="secondary compact" type="button" :disabled="!selectedPublicationArchiveIDs.length || beanListArchiving" @click="archiveSelectedBeanListPublications">
          归档选中
        </button>
        <span class="muted">已选 {{ selectedPublicationArchiveIDs.length }} 个；当前发布版本不可归档。</span>
      </div>

      <div v-if="publicationListCollapsed && currentScopePublicationRows.length" class="muted empty">
        已收起 {{ currentScopePublicationRows.length }} 个价格表版本。
      </div>

      <div v-else-if="paginatedCurrentScopePublicationRows.length" class="version-table-wrap">
        <table class="version-table">
          <thead>
            <tr>
              <th class="select-col">选择</th>
              <th>版本号</th>
              <th>类型</th>
              <th>归属</th>
              <th>状态</th>
              <th>时间</th>
              <th>更新说明</th>
              <th>来源</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in paginatedCurrentScopePublicationRows" :key="`bean-list-version-${row.id}`">
              <td class="select-col">
                <input
                  v-if="isBeanListAdmin"
                  type="checkbox"
                  :checked="isPublicationArchiveSelected(row)"
                  :disabled="!canArchiveBeanListPublication(row) || beanListArchiving"
                  :aria-label="`选择归档 ${row.version || row.id}`"
                  @change="togglePublicationArchiveSelection(row)"
                />
              </td>
              <td class="version-main">
                <strong>{{ row.version || '未命名版本' }}</strong>
                <small>#{{ row.id }}</small>
              </td>
              <td>{{ beanListPublicationTypeLabel(row) }}</td>
              <td>{{ beanListPublicationOwnerLabel(row) }}</td>
              <td>
                <span :class="['status-pill', beanListPublicationStatusClass(row)]">
                  {{ beanListPublicationStatusLabel(row) }}
                </span>
              </td>
              <td>{{ beanListPublicationTime(row) || '-' }}</td>
              <td class="version-note">{{ row.changelog || '无更新说明' }}</td>
              <td class="version-note">{{ beanListPublicationSourceLabel(row) }}</td>
              <td>
                <div class="version-actions">
                  <button class="secondary compact" type="button" :disabled="!beanListPublicationHasContent(row)" @click="downloadBeanListPublication(row)">下载 PDF</button>
                  <button class="secondary compact" type="button" @click="startBeanListFromPublication(row)">生成新版</button>
                  <button v-if="isBeanListAdmin && row.status === 'published'" class="danger compact" type="button" :disabled="beanListWithdrawing" @click="withdrawBeanList(row)">撤回</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        <PaginationControls
          v-model:page="publicationListPage"
          v-model:pageSize="publicationListPageSize"
          :total="publicationListState.total"
          :page-size-options="[5, 10, 20, 50, 100]"
        />
      </div>
      <div v-else-if="currentScopePublicationRows.length" class="muted empty">
        当前搜索没有匹配的价格表版本。
      </div>
      <div v-else class="muted empty">
        当前{{ publicationScopeLabel(versionListScope) }}暂无{{ selectedProductPriceListLabel }}价格表版本。
      </div>

      <div v-if="!publicationArchiveListCollapsed" class="version-archive-panel">
        <div class="section-bar">
          <div>
            <div class="section-title">归档列表</div>
            <p class="muted">归档价格表不在已发布价格表列表展示；需要恢复时点击移出归档。</p>
          </div>
        </div>
        <div v-if="paginatedArchivedPublicationRows.length" class="version-table-wrap">
          <table class="version-table">
            <thead>
              <tr>
                <th>版本号</th>
                <th>类型</th>
                <th>归属</th>
                <th>状态</th>
                <th>时间</th>
                <th>更新说明</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in paginatedArchivedPublicationRows" :key="`bean-list-archived-version-${row.id}`">
                <td class="version-main">
                  <strong>{{ row.version || '未命名版本' }}</strong>
                  <small>#{{ row.id }}</small>
                </td>
                <td>{{ beanListPublicationTypeLabel(row) }}</td>
                <td>{{ beanListPublicationOwnerLabel(row) }}</td>
                <td>
                  <span :class="['status-pill', beanListPublicationStatusClass(row)]">
                    {{ beanListPublicationStatusLabel(row) }}
                  </span>
                </td>
                <td>{{ beanListPublicationTime(row) || '-' }}</td>
                <td class="version-note">{{ row.changelog || '无更新说明' }}</td>
                <td>
                  <div class="version-actions">
                    <button class="secondary compact" type="button" :disabled="!beanListPublicationHasContent(row)" @click="downloadBeanListPublication(row)">下载 PDF</button>
                    <button v-if="isBeanListAdmin" class="secondary compact" type="button" :disabled="beanListArchiving" @click="restoreArchivedBeanListPublication(row)">移出归档</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
          <PaginationControls
            v-model:page="publicationArchiveListPage"
            v-model:pageSize="publicationArchiveListPageSize"
            :total="archivedPublicationListState.total"
            :page-size-options="[5, 10, 20, 50, 100]"
            :disabled="beanListArchiving"
          />
        </div>
        <div v-else class="muted empty">
          当前没有归档价格表版本。
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="bean-list-generate-bar">
        <div>
          <div class="section-title">生成价格表</div>
          <p class="muted">商品价格表是 Price List / Item Price 平铺价格行。生成时选择分组并勾选分组项选品；父商品只设置一次计价模式，所选规格共同继承。</p>
        </div>
        <div class="generate-actions">
          <button class="secondary" type="button" @click="priceListRulesDialogOpen = true">计价模式规则</button>
          <button class="primary" type="button" :disabled="loading || !visibleCostingItems.length || !productPriceListTypeOptions.length" @click="openBeanListDrawer()">价格表配置</button>
        </div>
      </div>
    </section>

    <section class="panel price-list-page-config">
      <div class="pdf-picker price-list-template-builder" data-pr440-price-list-model>
        <div class="picker-head">
          <strong>计价规则</strong>
          <span class="muted">商品 &gt; 子类 &gt; 父类 &gt; 价格表</span>
        </div>
        <div class="template-default-grid">
          <label>
            <span>价格表计价模式</span>
            <select :value="priceListTemplateDefaults.pricing_mode" @change="setPriceListDefaultTemplate('pricing_mode', $event.target.value)">
              <option v-for="mode in priceTablePricingModeOptions" :key="`default-mode-${mode.value}`" :value="mode.value">{{ mode.label }}</option>
            </select>
          </label>
          <label v-if="priceListTemplateDefaults.pricing_mode === 'tier_template'">
            <span>价格表阶梯模板</span>
            <select :value="priceListTemplateDefaults.tier_template_id" @change="setPriceListDefaultTemplate('tier_template_id', $event.target.value)">
              <option :value="0">请选择阶梯模板</option>
              <option v-for="template in priceTierTemplates" :key="`default-tier-${template.id}`" :value="template.id">{{ priceTierTemplateLabel(template) }}</option>
            </select>
          </label>
          <label v-else-if="priceListTemplateDefaults.pricing_mode === 'pricing_rule'">
            <span>价格表价格计算模板</span>
            <select :value="priceListTemplateDefaults.pricing_rule_id" @change="setPriceListDefaultTemplate('pricing_rule_id', $event.target.value)">
              <option :value="0">请选择价格计算模板</option>
              <option v-for="rule in pricingRules" :key="`default-rule-${rule.id}`" :value="rule.id">{{ pricingRuleLabel(rule) }}</option>
            </select>
          </label>
          <div v-else class="inline-price-config-note">
            <strong>价格表固定价</strong>
            <span>固定价金额按具体规格分别录入；此处只继承计价方式。</span>
          </div>
        </div>
        <p class="muted inline-pricing-config-note">分类和父商品计价直接在下方选品位置处理；商品的全部已选规格继承同一种计价方式，固定价金额仍按规格分别录入。</p>
        <div v-if="priceListLegacyPricingConflicts.length" class="product-spec-selection-warning price-list-legacy-pricing-warning">
          <strong>旧草稿存在规格级计价冲突，发布已阻止。</strong>
          <span v-for="conflict in priceListLegacyPricingConflicts" :key="`legacy-pricing-${conflict.parent_product_id}`">{{ conflict.message }}</span>
        </div>
      </div>

      <div class="pdf-picker productSelection">
        <div class="picker-head">
          <strong>选择分类和产品</strong>
          <span class="muted" aria-label="X款/Y规格">已选 {{ pdfProductSpecSelectionCounts.productCount }} 款 / {{ pdfProductSpecSelectionCounts.specCount }} 规格，共 {{ pdfAvailableItems.length }} 款</span>
          <div class="picker-actions">
            <button class="secondary compact" type="button" @click="setAllPdfProducts(true)">全选</button>
            <button class="secondary compact" type="button" @click="setAllPdfProducts(false)">清空</button>
          </div>
        </div>
        <div class="product-picker-list categoryProductGroups">
          <template v-for="category in categoryProductGroups" :key="`pick-cat-${category.code}`">
            <section
              v-if="!isProductPickerCategoryHiddenByCollapsedAncestor(category)"
              :class="['product-picker-category', { collapsed: isProductPickerCategoryCollapsed(category) }]"
              :style="productPickerCategoryStyle(category)"
            >
            <div class="product-picker-category-head">
              <button
                type="button"
                class="secondary compact category-collapse-toggle"
                aria-label="收起或展开分类"
                @click.stop="toggleProductPickerCategoryCollapse(category)"
              >
                {{ isProductPickerCategoryCollapsed(category) ? '展开' : '收起' }}
              </button>
              <label class="check-line">
                <input type="checkbox" :checked="isPdfCategorySelected(category.code)" :indeterminate.prop="isPdfCategoryPartiallySelected(category.code)" @change="togglePdfCategoryProducts(category.code, $event.target.checked)" />
                <span>{{ category.label }}</span>
              </label>
              <span class="muted">{{ selectedCountForCategory(category.code) }}/{{ productIDsForCategory(category.code).length }} 款 · {{ selectedSpecCountForCategory(category.code) }} 规格</span>
              <div class="category-pricing-summary">
                <button
                  type="button"
                  :class="['price-list-summary-button', { active: isPriceListPricingPopoverOpen('category', category), overridden: priceListCategoryPricingHasOverride(category) }]"
                  @click.stop="openPriceListPricingPopover('category', category)"
                >
                  <span>计价</span>
                  <strong>{{ priceListCategoryPricingSummary(category) }}</strong>
                  <small v-if="priceListCategoryPricingHasOverride(category)">已覆盖</small>
                </button>
                <div v-if="isPriceListPricingPopoverOpen('category', category)" class="price-list-pricing-popover" @click.stop>
                  <div class="price-list-pricing-popover-title">
                    <strong>{{ priceListPricingPopover.group?.group_item_name || '当前分类' }}</strong>
                    <button type="button" class="secondary compact" @click="closePriceListPricingPopover">关闭</button>
                  </div>
                  <div class="price-list-pricing-options">
                    <button
                      v-for="option in priceListPricingPopoverOptions"
                      :key="`category-pricing-option-${option.value || 'inherit'}`"
                      type="button"
                      :class="{ active: priceListActivePricingSelection().pricing_mode === option.value }"
                      @click="setPriceListPricingPopoverMode(option.value)"
                    >
                      {{ option.label }}
                    </button>
                  </div>
                  <label v-if="priceListActivePricingSelection().pricing_mode === 'tier_template'" class="inline-price-config">
                    <span>阶梯模板</span>
                    <select :value="priceListActivePricingSelection().tier_template_id" @change="setPriceListPricingPopoverField('tier_template_id', $event.target.value)">
                      <option :value="0">请选择阶梯模板</option>
                      <option v-for="template in priceTierTemplates" :key="`category-pop-tier-${template.id}`" :value="template.id">{{ priceTierTemplateLabel(template) }}</option>
                    </select>
                  </label>
                  <label v-else-if="priceListActivePricingSelection().pricing_mode === 'pricing_rule'" class="inline-price-config">
                    <span>价格计算模板</span>
                    <select :value="priceListActivePricingSelection().pricing_rule_id" @change="setPriceListPricingPopoverField('pricing_rule_id', $event.target.value)">
                      <option :value="0">请选择价格计算模板</option>
                      <option v-for="rule in pricingRules" :key="`category-pop-rule-${rule.id}`" :value="rule.id">{{ pricingRuleLabel(rule) }}</option>
                    </select>
                  </label>
                  <p v-else-if="priceListActivePricingSelection().pricing_mode === 'fixed_price'" class="muted inline-pricing-config-note">
                    固定价金额按具体规格分别录入；分类只继承计价方式。
                  </p>
                </div>
              </div>
            </div>
            <article
              v-for="row in category.items"
              v-if="!isProductPickerCategoryCollapsed(category)"
              :key="`pick-${itemProductID(row)}`"
              class="product-picker-row"
              :style="productPickerRowStyle(category)"
            >
              <div class="product-picker-row-head">
                <label class="check-line">
                  <input type="checkbox" :checked="isPdfProductSelected(priceListParentProductID(row))" @change="togglePdfProduct(row, $event.target.checked)" />
                  <span>{{ beanMeta(row, metaKeyForListType(pdfTheme.listType)).code }} {{ beanName(row, metaKeyForListType(pdfTheme.listType)) }}</span>
                </label>
                <div v-if="isPdfProductSelected(priceListParentProductID(row))" class="product-compact-status">
                  <button
                    type="button"
                    :class="['price-list-summary-button', { active: isPriceListPricingPopoverOpen('product', priceListParentProductPricingRow(row)), overridden: priceListProductPricingHasOverride(priceListParentProductPricingRow(row)) }]"
                    @click.stop="openPriceListPricingPopover('product', priceListParentProductPricingRow(row))"
                  >
                    <span>商品计价</span>
                    <strong>{{ priceListProductPricingSummary(priceListParentProductPricingRow(row)) }}</strong>
                    <small v-if="priceListProductPricingHasOverride(priceListParentProductPricingRow(row))">已覆盖</small>
                  </button>
                  <button
                    type="button"
                    :class="['price-list-summary-button', { active: isPriceListProductDisplayDialogOpen(priceListParentProductID(row)), overridden: priceListProductDisplayHasOverride(priceListParentProductID(row)) }]"
                    @click.stop="openPriceListProductDisplayDialog(priceListParentProductID(row))"
                  >
                    <span>展示</span>
                    <strong>{{ priceListProductDisplaySummary(priceListParentProductID(row)) }}</strong>
                    <small v-if="priceListProductDisplayHasOverride(priceListParentProductID(row))">已设置</small>
                  </button>
                  <div v-if="isPriceListPricingPopoverOpen('product', priceListParentProductPricingRow(row))" class="price-list-pricing-popover" @click.stop>
                    <div class="price-list-pricing-popover-title">
                      <strong>{{ priceListPricingPopover.productRow?.parent_product_name }} · 商品计价</strong>
                      <button type="button" class="secondary compact" @click="closePriceListPricingPopover">关闭</button>
                    </div>
                    <div class="price-list-pricing-options">
                      <button
                        v-for="option in priceListPricingPopoverOptions"
                        :key="`parent-product-pricing-option-${priceListParentProductID(row)}-${option.value || 'inherit'}`"
                        type="button"
                        :class="{ active: priceListActivePricingSelection().pricing_mode === option.value }"
                        @click="setPriceListPricingPopoverMode(option.value)"
                      >
                        {{ option.label }}
                      </button>
                    </div>
                    <label v-if="priceListActivePricingSelection().pricing_mode === 'tier_template'" class="inline-price-config">
                      <span>阶梯模板</span>
                      <select :value="priceListActivePricingSelection().tier_template_id" @change="setPriceListPricingPopoverField('tier_template_id', $event.target.value)">
                        <option :value="0">请选择阶梯模板</option>
                        <option v-for="template in priceTierTemplates" :key="`parent-product-pop-tier-${template.id}`" :value="template.id">{{ priceTierTemplateLabel(template) }}</option>
                      </select>
                      <small>模板按件数继承；档位统一显示为 1件、10件，规格单独展示。</small>
                    </label>
                    <label v-else-if="priceListActivePricingSelection().pricing_mode === 'pricing_rule'" class="inline-price-config">
                      <span>价格计算模板</span>
                      <select :value="priceListActivePricingSelection().pricing_rule_id" @change="setPriceListPricingPopoverField('pricing_rule_id', $event.target.value)">
                        <option :value="0">请选择价格计算模板</option>
                        <option v-for="rule in pricingRules" :key="`parent-product-pop-rule-${rule.id}`" :value="rule.id">{{ pricingRuleLabel(rule) }}</option>
                      </select>
                      <small>所选规格分别按自身销售规格试算。</small>
                    </label>
                    <div v-else-if="priceListActivePricingSelection().pricing_mode === 'fixed_price'" class="parent-product-fixed-prices">
                      <label v-for="spec in selectedSpecsForProduct(priceListPricingPopover.productRow)" :key="`parent-fixed-${priceListSkuID(spec)}`" class="inline-price-config">
                        <span>固定价（元/{{ priceListProductSpecLabel(spec) }}）</span>
                        <input
                          type="number"
                          min="0"
                          step="0.01"
                          :value="priceListProductFixedPrice(priceListPricingPopover.productRow, spec)"
                          @input="setPriceListProductFixedPrice(priceListPricingPopover.productRow, spec, $event.target.value)"
                        />
                      </label>
                      <p v-if="!selectedSpecsForProduct(priceListPricingPopover.productRow).length" class="muted inline-pricing-config-note">请先选择至少一个规格，再分别录入固定价。</p>
                    </div>
                  </div>
                </div>
              </div>
              <div v-if="productSpecSelectionIssueForFamily(row)" class="product-spec-selection-warning">
                <strong>{{ productSpecSelectionIssueForFamily(row).message }}</strong>
                <div class="product-spec-selection-warning-actions">
                  <button
                    v-if="productSpecSelectionIssueForFamily(row).type === 'default_changed'"
                    class="secondary compact"
                    type="button"
                    @click.stop="resolveProductSpecSelectionIssue(row, 'keep')">
                    保留原规格
                  </button>
                  <button class="secondary compact" type="button" @click.stop="resolveProductSpecSelectionIssue(row, 'switch')">切换当前默认规格</button>
                </div>
              </div>
              <div class="product-spec-options">
                <div v-for="spec in row.sku_options" :key="`product-spec-${priceListSkuID(spec)}`" :class="['product-spec-option', { selected: isPdfProductSpecSelected(row, spec) }]">
                  <label class="check-line product-spec-check">
                    <input type="checkbox" :checked="isPdfProductSpecSelected(row, spec)" @change="togglePdfProductSpec(row, spec, $event.target.checked)" />
                    <span>{{ priceListProductSpecLabel(spec) }}</span>
                    <small v-if="priceListSkuID(spec) === Number(row.default_sku_id || 0)">默认规格</small>
                  </label>
                  <div v-if="priceListProductTierTemplateWarning(spec)" class="product-picker-tier-warning">
                    <strong>{{ priceListProductTierTemplateWarning(spec) }}</strong>
                  </div>
                </div>
              </div>
              <div v-if="visibleItemWarnings(row).length" class="item-warning-list">
                <span v-for="warning in visibleItemWarnings(row)" :key="warning" class="warning-icon-wrap">
                  <button class="warning-icon" type="button" aria-label="查看商品警示">!</button>
                  <span class="warning-tooltip">{{ warningTooltip(warning) }}</span>
                </span>
              </div>
              <div
                v-if="itemBomWarning(row)"
                class="product-picker-bom-warning"
                :data-bom-warning-product-id="itemProductID(row)"
              >
                <div>
                  <strong>BOM已失效：{{ itemBomProblemLabel(row) }}</strong>
                  <span>{{ itemBomWarning(row) }}</span>
                </div>
                <button class="secondary compact" type="button" @click.stop="openProductArchiveForBom(row)">去商品档案重新选择 BOM</button>
              </div>
              <template v-for="spec in selectedSpecsForProduct(row)" :key="`green-spec-${priceListSkuID(spec)}`">
                <div v-if="pdfTheme.listType === 'green' && greenTierPriceRows(spec).length" class="green-tier-price-editor">
                  <label v-for="tier in greenTierPriceRows(spec)" :key="`green-price-${priceListSkuID(spec)}-${greenTierOverrideKey(tier)}`">
                    <span>{{ tier.label }}</span>
                    <input type="number" min="0" step="0.01" :value="greenTierPriceValue(priceListSkuID(spec), tier)" @input="setGreenBeanTierPrice(priceListSkuID(spec), tier, $event.target.value)" />
                    <small>/{{ greenTierPriceUnit(tier) }}</small>
                  </label>
                </div>
              </template>
            </article>
            </section>
          </template>
        </div>
      </div>

      <div v-if="priceListConfigDialog.open" class="price-list-config-dialog-backdrop" @click.self="closePriceListConfigDialog">
        <section class="price-list-config-dialog" role="dialog" aria-modal="true">
          <div class="price-list-config-dialog-head">
            <strong>{{ priceListConfigDialogTitle }}</strong>
            <button type="button" class="secondary compact" @click="closePriceListConfigDialog">关闭</button>
          </div>
          <template v-if="priceListConfigDialog.type === 'product-display'">
            <p class="muted">商品展示</p>
            <div class="customizer-row">
              <select :value="customizerField(priceListConfigDialog.productId, 'badge')" @change="setCustomizerField(priceListConfigDialog.productId, 'badge', $event.target.value)">
                <option value="">无标签</option>
                <option value="new">NEW 上新</option>
                <option value="thumb">推荐</option>
                <option value="medal">推荐</option>
              </select>
              <input :value="customizerField(priceListConfigDialog.productId, 'highlightTerms')" placeholder="标红词，用逗号分隔" @input="setCustomizerField(priceListConfigDialog.productId, 'highlightTerms', $event.target.value)" />
            </div>
          </template>
        </section>
      </div>

      <div v-if="priceListFlatRows.length" class="pdf-picker flat-price-row-editor">
        <div class="picker-head">
          <strong>平铺价格行</strong>
          <span class="muted">{{ priceListFlatRows.length }} 行，发布快照固化分组、模板来源、Pricing Rule 版本、成本来源和客户引用</span>
          <div v-if="priceListPricingRuleTrialFailedCount" class="picker-actions">
            <button class="secondary compact" type="button" @click="retryPriceListPricingRuleTrials">重新试算失败项（{{ priceListPricingRuleTrialFailedCount }}）</button>
          </div>
        </div>
        <div class="flat-price-table" v-if="priceListFlatRows.length">
          <div class="flat-price-head">
            <span>商品 / 档位</span>
            <span>计价来源</span>
            <span>最终价</span>
            <span>快照</span>
          </div>
          <div v-for="row in priceListFlatRows" :key="row.row_key" :class="['flat-price-row', { invalid: hasPriceListFlatRowError(row), loading: priceListFlatRowPricingTrialStatus(row) === 'loading' }]">
            <div>
              <strong>{{ priceListFlatRowDisplayTitle(row) }}</strong>
              <span>{{ priceListFlatRowSpecDescription(row) }}</span>
              <span>{{ row.group_snapshot.group_item_name || '-' }} · {{ row.tier_label || '-' }} · {{ row.group_source === 'price_list' ? '价格表覆盖' : '商品档案分组' }}</span>
            </div>
            <div>
              <span>{{ priceTablePricingModeLabel(row.pricing_mode) }}：{{ priceListSourceLabel(row.pricing_mode_source) }}</span>
              <span v-if="row.tier_template_id">阶梯模板：{{ priceListSourceLabel(row.tier_template_source) }}</span>
              <span v-if="row.pricing_rule_id">计算模板：{{ priceListSourceLabel(row.pricing_rule_source) }}</span>
            </div>
            <label>
              <input type="number" min="0" step="0.01" :value="row.final_unit_price" :disabled="priceListFlatRowHasLegacyUnitMismatch(row)" @input="setPriceListFlatRowPrice(row, $event.target.value)" />
              <small>/{{ priceListFlatRowPriceUnitLabel(row) }}</small>
            </label>
            <div>
              <span>{{ row.pricing_rule_version || (row.pricing_mode === 'fixed_price' ? '固定价' : '未选择 Pricing Rule') }}</span>
              <span :class="{ adjusted: row.manual_adjusted }">{{ row.manual_adjusted ? '人工调整' : (priceListFlatRowPricingTrialStatus(row) === 'loading' ? '价格计算中…' : (priceListFlatRowPricingTrialStatus(row) === 'error' ? '计算失败' : '自动计算')) }}</span>
              <span>{{ priceListFlatRowUnitSummary(row) }}</span>
            </div>
            <ul v-if="priceListFlatRowVisibleErrors(row).length" class="flat-price-row-error-list">
              <li v-for="msg in priceListFlatRowVisibleErrors(row)" :key="msg">{{ msg }}</li>
            </ul>
          </div>
        </div>
      </div>

      <section class="price-list-preview">
        <div class="pdf-preview-title">
          <strong>预览</strong>
          <span>{{ pdfTotalItems }} 款</span>
          <div class="pdf-actions">
            <button v-if="isBeanListAdmin" class="secondary" type="button" :disabled="beanListWithdrawing || !currentBeanListPublication" @click="withdrawBeanList()">撤回发布</button>
            <button v-if="isBeanListAdmin" class="primary" type="button" :disabled="beanListPublishing" @click="publishBeanList">发布价格表</button>
            <button v-else class="primary" type="button" :disabled="beanListPublishing || !pdfGroups.length || !pdfTheme.version || !customerScopeReady" @click="saveBeanListDraft">保存修改</button>
            <button class="secondary" type="button" :disabled="beanListPdfGenerating || !pdfGroups.length" @click="generateBeanListPdf">{{ beanListPdfGenerating ? '生成中' : '生成 PDF' }}</button>
          </div>
          <p v-if="error" class="error price-list-publish-feedback">{{ error }}</p>
          <p v-if="message" class="ok price-list-publish-feedback">{{ message }}</p>
        </div>
        <div class="pdf-preview-phone bean-list-pdf-surface" :style="pdfPageStyle">
        <header class="pdf-cover">
          <div>
            <img v-if="pdfTheme.logoImage" class="pdf-logo" :src="pdfTheme.logoImage" alt="logo" />
            <p v-if="pdfTheme.showVersion" class="pdf-version">{{ pdfTheme.version }}</p>
            <h1>{{ pdfTitle }}</h1>
            <p>{{ pdfSubtitle }}</p>
            <p v-if="pdfTheme.brandIntro" class="pdf-brand-intro">{{ pdfTheme.brandIntro }}</p>
          </div>
          <div class="pdf-badge">{{ selectedProductPriceListLabel }}</div>
        </header>

        <section v-for="group in pdfGroups" :key="`preview-${group.category}`" class="pdf-group">
          <h2 v-if="group.showCategory && pdfOptions.showCategoryNumbers">{{ group.category }}</h2>

          <table v-if="pdfTheme.layoutStyle === 'table'" class="pdf-compact-table">
            <tbody>
              <tr v-for="item in group.items" :key="`preview-table-${group.category}-${item.code}`">
                <td class="pdf-code-cell">{{ item.code }}</td>
                <td>
                  <div class="pdf-table-name">
                    <span v-for="(part, idx) in highlightedParts(item.name, item)" :key="`pn-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    <span v-if="item.badgeLabel" :class="badgeClass(item.badge)">{{ item.badgeLabel }}</span>
                  </div>
                  <div v-for="line in item.attributeLines || []" :key="`pa-${item.code}-${line}`" class="pdf-table-line"><b>属性</b> {{ line }}</div>
                </td>
                <td class="pdf-table-prices">
                  <div v-for="priceRow in item.prices" :key="`preview-table-price-${item.code}-${priceRow.label}`">
                    <span :class="{ 'pdf-red': priceRow.red }">
                      <span v-for="(part, idx) in priceLabelParts(priceRow, item)" :key="`ptl-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </span>
                    <strong :class="priceValueClass(priceRow, item)">
                      <span v-for="(part, idx) in priceValueParts(priceRow, item)" :key="`ptv-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </strong>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>

          <div v-else class="pdf-card-grid">
            <div v-for="(row, rowIndex) in cardRows(group)" :key="`preview-row-${group.category}-${rowIndex}`" :class="['pdf-card-row', `cards-${row.columns}`]" :style="cardRowStyle(row)">
              <article v-for="item in row.items" :key="`preview-${group.category}-${item.code}`" class="pdf-item">
                <div class="pdf-item-head">
                  <span>{{ item.code }}</span>
                  <div>
                    <h3>
                      <span v-for="(part, idx) in highlightedParts(item.name, item)" :key="`cn-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                      <span v-if="item.badgeLabel" :class="badgeClass(item.badge)">{{ item.badgeLabel }}</span>
                    </h3>
                  </div>
                </div>
                <div class="pdf-meta-block">
                  <p v-for="line in item.attributeLines || []" :key="`card-attr-${item.code}-${line}`" class="pdf-meta-line"><b>属性</b> {{ line }}</p>
                </div>
                <div class="pdf-price-block">
                  <div class="pdf-section-label">报价</div>
                  <div class="pdf-price-list">
                    <div v-for="priceRow in item.prices" :key="`preview-price-${item.code}-${priceRow.label}`" class="pdf-price">
                      <span :class="{ 'pdf-red': priceRow.red }">
                        <span v-for="(part, idx) in priceLabelParts(priceRow, item)" :key="`pl-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                      </span>
                      <strong :class="priceValueClass(priceRow, item)">
                        <span v-for="(part, idx) in priceValueParts(priceRow, item)" :key="`pv-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                      </strong>
                    </div>
                  </div>
                </div>
              </article>
            </div>
          </div>
        </section>

        <div v-if="pdfTheme.showChangelog && pdfTheme.changelog" class="pdf-changelog pdf-bottom-changelog">
          <strong>更新</strong>
          <span>{{ pdfTheme.changelog }}</span>
        </div>
        <footer class="pdf-footer">
          <span>{{ pdfTheme.brandName || '棵凡咖啡' }}</span>
          <span>{{ pdfTheme.version }}</span>
        </footer>
        </div>
      </section>
    </section>

    <div v-if="priceListRulesDialogOpen" class="price-list-config-dialog-backdrop" @click.self="priceListRulesDialogOpen = false">
      <section class="price-list-config-dialog price-list-rules-dialog" role="dialog" aria-modal="true" aria-label="计价模式规则">
        <div class="price-list-config-dialog-head">
          <strong>计价模式规则</strong>
          <button type="button" class="secondary compact" @click="priceListRulesDialogOpen = false">关闭</button>
        </div>
        <p class="muted">计价模式按 <b>父商品 &gt; 子类 &gt; 父类 &gt; 价格表</b> 解析。父商品只选择一次计价模式、阶梯模板或价格计算模板，全部已选规格共同继承；固定价金额按规格分别录入。生成价格表默认使用商品档案分组，本次覆盖只写入价格表快照 group_source=price_list，不回写商品档案分组。</p>
        <table class="price-list-rule-table">
          <thead>
            <tr>
              <th>层级</th>
              <th>用途</th>
              <th>发布快照</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>价格表</td>
              <td>价格表默认计价模式、阶梯模板、价格计算模板或固定价；分组来源可为商品档案分组或价格表覆盖</td>
              <td rowspan="4">固化计价模式、最终价、价格单位、库存换算、分组快照、模板来源和 Pricing Rule 版本</td>
            </tr>
            <tr>
              <td>父类 / 子类</td>
              <td>按分组项覆盖计价模式，用于批量选品和分组展示</td>
            </tr>
            <tr>
              <td>商品行</td>
              <td>父商品统一覆盖计价模式、阶梯模板或价格计算模板；固定价金额按所选规格分别录入，不支持规格级模板覆盖</td>
            </tr>
            <tr>
              <td>录单</td>
              <td>订单单位和价格只读取已发布价格表快照</td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>

    <div v-if="tierTemplateDrawerOpen" class="drawer-backdrop" @click.self="closeTierTemplateDrawer">
      <aside class="settings-drawer tier-template-drawer" aria-label="阶梯模板">
        <div class="drawer-head">
          <div>
            <h3>阶梯模板</h3>
            <p>阶梯模板只定义销售规格件数。每个档位选择一个价格计算模板，档位显示为 1件、10件；227g、454g 等规格单独展示。</p>
          </div>
          <button class="secondary" type="button" @click="closeTierTemplateDrawer">关闭</button>
        </div>
        <div class="tier-template-drawer-body">
          <section class="tier-template-list">
            <div class="picker-head">
              <strong>模板列表</strong>
              <button class="secondary compact" type="button" @click="resetPriceTierTemplateForm">新建阶梯模板</button>
            </div>
            <button
              v-for="template in priceTierTemplates"
              :key="`drawer-tier-template-${template.id}`"
              :class="['template-row-main tier-template-list-row', { active: Number(template.id || 0) === Number(priceTierTemplateForm.id || 0) }]"
              type="button"
              @click="startPriceTierTemplateEdit(template)">
              <strong>{{ template.name }}</strong>
              <small>{{ template.tiers?.length || 0 }} 档 · {{ template.active === false ? '已停用' : '启用' }}</small>
            </button>
            <p v-if="!priceTierTemplates.length" class="muted">暂无阶梯模板</p>
          </section>
          <form class="template-editor tier-template-form" @submit.prevent="savePriceTierTemplate">
            <div class="template-editor-grid">
              <label>
                <span>模板名称</span>
                <input v-model.trim="priceTierTemplateForm.name" placeholder="如 批发阶梯" />
              </label>
              <label class="check-line">
                <input v-model="priceTierTemplateForm.active" type="checkbox" />
                <span>启用</span>
              </label>
            </div>
            <div class="template-tier-head">
              <strong>档位</strong>
              <button class="secondary compact-action" type="button" @click="addPriceTierTemplateTier">新增档位</button>
            </div>
            <div class="template-tier-list">
              <div v-for="(tier, index) in priceTierTemplateForm.tiers" :key="`drawer-price-tier-template-${index}`" class="template-tier-row price-list-tier-template-row">
                <label>
                  <span>档位名</span>
                  <input v-model.trim="tier.label" placeholder="如 1件、10件、100件" />
                </label>
                <label>
                  <span>最小件数</span>
                  <input v-model.number="tier.min_qty" type="number" min="0" step="0.0001" />
                </label>
                <label>
                  <span>最大件数</span>
                  <input v-model="tier.max_qty" type="number" min="0" step="0.0001" placeholder="无上限" />
                </label>
                <label>
                  <span>价格计算模板</span>
                  <select v-model.number="tier.pricing_rule_id">
                    <option :value="0">请选择</option>
                    <option v-for="rule in pricingRules" :key="`tier-template-rule-${index}-${rule.id}`" :value="rule.id">{{ pricingRuleLabel(rule) }}</option>
                  </select>
                </label>
                <button class="text-button danger-text" type="button" @click="removePriceTierTemplateTier(index)">删除</button>
              </div>
            </div>
            <label class="wide-field">
              <span>备注</span>
              <textarea v-model.trim="priceTierTemplateForm.remark" rows="2"></textarea>
            </label>
            <div class="form-actions">
              <button class="danger" type="button" :disabled="tierTemplateSaving || !priceTierTemplateForm.id" @click="deletePriceTierTemplate">删除阶梯模板</button>
              <button class="primary" type="submit" :disabled="tierTemplateSaving">{{ tierTemplateSaving ? '保存中' : '保存阶梯模板' }}</button>
            </div>
          </form>
        </div>
      </aside>
    </div>

    <div v-if="priceExplanationOpen" class="drawer-backdrop" @click.self="priceExplanationOpen = false">
      <aside class="settings-drawer explanation-drawer" aria-label="价格来源解释">
        <div class="drawer-head">
          <div>
            <h3>价格来源</h3>
            <p>{{ explanationItem?.name || '-' }} · {{ explanationTier?.label || '-' }}</p>
          </div>
          <button class="secondary" type="button" @click="priceExplanationOpen = false">关闭</button>
        </div>
        <div v-if="priceExplanationError" class="error">{{ priceExplanationError }}</div>
        <div v-if="priceExplanation" class="explanation-summary">
          <div>
            <span>模板</span>
            <strong>{{ priceExplanation.template_name || '-' }}</strong>
          </div>
          <div>
            <span>当前试算</span>
            <strong>{{ price(priceExplanation.saved_final_price) }}/{{ gradientDisplayUnitLabel(priceExplanation.display_unit).replace('元/', '') }}</strong>
          </div>
          <div>
            <span>临时试算</span>
            <strong>{{ price(priceExplanation.preview_final_price) }}/{{ gradientDisplayUnitLabel(priceExplanation.display_unit).replace('元/', '') }}</strong>
          </div>
        </div>
        <div v-if="priceExplanation" class="explanation-form">
          <label>
            <span>临时生豆成本 元/kg</span>
            <input v-model="explanationOverrides.green_bean_cost_per_kg" type="number" step="0.01" placeholder="不填则沿用当前" />
          </label>
          <label>
            <span>临时预期产出率</span>
            <input v-model="explanationOverrides.yield_rate" type="number" step="0.001" placeholder="如 0.82" />
          </label>
          <label>
            <span>临时利润率</span>
            <input v-model="explanationOverrides.margin_rate" type="number" step="0.001" placeholder="如 0.28" />
          </label>
          <button class="secondary" type="button" :disabled="priceExplanationLoading" @click="loadPriceExplanation">重新试算</button>
        </div>
        <div v-if="priceExplanation" class="formula-steps">
          <div v-for="step in priceExplanation.steps || []" :key="step.key" :class="['formula-step', { changed: step.changed }]">
            <span>{{ step.label }}</span>
            <strong>{{ fixed(step.value, step.unit === 'ratio' ? 3 : 2) }} {{ step.unit || '' }}</strong>
            <small>{{ step.source }}</small>
          </div>
        </div>
        <p class="muted">这里的参数只做临时试算，不会写入物料档案、商品生产配置或价格计算模板；交易价格仍需发布后生效。</p>
      </aside>
    </div>

    <div v-if="pdfDrawerOpen" class="drawer-backdrop" @click.self="pdfDrawerOpen = false">
      <aside class="settings-drawer pdf-drawer" aria-label="价格表配置">
        <div class="drawer-head">
          <div>
            <h3>价格表配置</h3>
            <p>维护版本、样式、归属和价格来源；生成规则、选品、平铺价格行和预览已在主页面展示。</p>
            <p v-if="currentBeanListPublication" class="publish-state">当前已发布：{{ currentBeanListPublication.version }} · {{ currentBeanListPublication.published_at }}</p>
            <p v-else class="publish-state">当前暂无已发布版本</p>
            <div v-if="publicBeanListURL" class="public-link-box">
              <span>客户访问链接</span>
              <a :href="publicBeanListURL" target="_blank" rel="noopener">{{ publicBeanListURL }}</a>
              <button class="secondary compact" type="button" @click="copyPublicBeanListURL">复制链接</button>
            </div>
          </div>
          <button class="secondary" type="button" @click="pdfDrawerOpen = false">关闭</button>
        </div>

        <div class="copy-config-box publication-context-box">
          <div>
            <strong>当前归属</strong>
            <p>{{ currentPublicationScopeDescription }}</p>
          </div>
          <div class="current-owner-pill">{{ currentPublicationOwnerLabel }}</div>
          <p v-if="actorLoaded && !isBeanListAdmin" class="muted">客户账号只能保存修改和下载价格表，发布由管理员执行。</p>
        </div>

        <div class="copy-config-box bean-list-publish-reminder">
          <div>
            <strong>发布提醒</strong>
            <p v-if="pdfTheme.listType === 'green'">梯度按 KG，单价按元/KG；生成并发布新版价格表后，录单才会使用新价格。</p>
            <p v-else>生成并发布新版价格表后，录单和客户侧才会使用新价格。</p>
          </div>
        </div>

        <div class="copy-config-box" v-if="(publicationScope === 'mine' || publicationScope === 'customer') && officialPriceSourcePublications.length">
          <div>
            <strong>复制官方价格来源</strong>
            <p>复制棵凡已发布价格表的报价和商品展示快照，作为本次客户价格表的锁定内容快照。</p>
          </div>
          <div class="copy-config-actions">
            <select v-model="selectedPriceSourcePublicationID">
              <option value="">选择官方价格表价格</option>
              <option v-for="row in officialPriceSourcePublications" :key="`price-source-${row.id}`" :value="String(row.id)">
                {{ beanListPublicationLabel(row) }}
              </option>
            </select>
            <button class="secondary" type="button" :disabled="!selectedPriceSourcePublication" @click="applyCopiedBeanListPriceSource()">复制价格</button>
          </div>
        </div>

        <div class="pdf-form">
          <label>
            <span>商品类型</span>
            <select v-model.number="selectedProductTypeCategoryID" :disabled="!productPriceListTypeOptions.length">
              <option v-for="type in productPriceListTypeOptions" :key="`drawer-${type.key}`" :value="type.id">
                {{ type.label }}（{{ type.itemCount }}款）
              </option>
            </select>
          </label>
          <label>
            <span>版本号</span>
            <input v-model.trim="pdfOptions.version" placeholder="V3.0.5" />
          </label>
          <label>
            <span>品牌名字</span>
            <input v-model.trim="pdfOptions.brandName" :placeholder="activeCostingScope === 'customer' ? '' : '棵凡咖啡'" />
          </label>
          <label>
            <span>豆单样式</span>
            <select v-model="pdfOptions.layoutStyle">
              <option value="card">豆卡样式</option>
              <option value="table">紧密表格样式</option>
            </select>
          </label>
          <label>
            <span>每行豆卡</span>
            <input v-model.number="pdfOptions.cardsPerRow" type="number" min="1" max="4" :disabled="pdfOptions.layoutStyle === 'table'" />
          </label>
          <label>
            <span>背景颜色</span>
            <input v-model="pdfOptions.backgroundColor" type="color" />
          </label>
          <label>
            <span>字体颜色</span>
            <input v-model="pdfOptions.fontColor" type="color" />
          </label>
          <label class="wide">
            <span>背景上传</span>
            <input type="file" accept="image/*" @change="handlePdfBackgroundUpload" />
          </label>
          <label class="wide">
            <span>上传logo</span>
            <input type="file" accept="image/*" @change="handlePdfLogoUpload" />
          </label>
          <label class="wide">
            <span>品牌介绍</span>
            <textarea v-model.trim="pdfOptions.brandIntro" rows="3" placeholder="例如：棵凡咖啡专注精品咖啡烘焙与稳定出品。"></textarea>
          </label>
          <label class="wide">
            <span>历史更新日志</span>
            <textarea v-model.trim="pdfOptions.changelog" rows="3" placeholder="例如：V3.0.5 调整庄园精品豆报价，新增上新标签。"></textarea>
          </label>
          <label class="check-line">
            <input v-model="pdfOptions.showVersion" type="checkbox" />
            <span>展示版本号</span>
          </label>
          <label class="check-line">
            <input v-model="pdfOptions.showChangelog" type="checkbox" />
            <span>展示历史更新日志</span>
          </label>
          <label class="check-line wide">
            <input v-model="pdfOptions.showCategoryNumbers" type="checkbox" />
            <span>展示分级编号</span>
          </label>
          <div class="wide pdf-actions">
            <button class="secondary" type="button" @click="clearPdfBackground" :disabled="!pdfOptions.backgroundImage">清除背景图</button>
            <button class="secondary" type="button" @click="clearPdfLogo" :disabled="!pdfOptions.logoImage">清除logo</button>
          </div>
        </div>
      </aside>
    </div>

    <Teleport to="body">
      <section v-if="pdfPrinting" class="bean-list-pdf-page bean-list-pdf-surface" :style="pdfPageStyle">
        <header class="pdf-cover">
          <div>
            <img v-if="pdfTheme.logoImage" class="pdf-logo" :src="pdfTheme.logoImage" alt="logo" />
            <p v-if="pdfTheme.showVersion" class="pdf-version">{{ pdfTheme.version }}</p>
            <h1>{{ pdfTitle }}</h1>
            <p>{{ pdfSubtitle }}</p>
            <p v-if="pdfTheme.brandIntro" class="pdf-brand-intro">{{ pdfTheme.brandIntro }}</p>
          </div>
          <div class="pdf-badge">{{ selectedProductPriceListLabel }}</div>
        </header>

        <section v-for="group in pdfGroups" :key="`pdf-${group.category}`" class="pdf-group">
          <h2 v-if="group.showCategory && pdfOptions.showCategoryNumbers">{{ group.category }}</h2>

          <table v-if="pdfTheme.layoutStyle === 'table'" class="pdf-compact-table">
            <tbody>
              <tr v-for="item in group.items" :key="`pdf-table-${group.category}-${item.code}`">
                <td class="pdf-code-cell">{{ item.code }}</td>
                <td>
                  <div class="pdf-table-name">
                    <span v-for="(part, idx) in highlightedParts(item.name, item)" :key="`pn-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    <span v-if="item.badgeLabel" :class="badgeClass(item.badge)">{{ item.badgeLabel }}</span>
                  </div>
                  <div v-for="line in item.attributeLines || []" :key="`pa-print-${item.code}-${line}`" class="pdf-table-line"><b>属性</b> {{ line }}</div>
                </td>
                <td class="pdf-table-prices">
                  <div v-for="priceRow in item.prices" :key="`pdf-table-price-${item.code}-${priceRow.label}`">
                    <span :class="{ 'pdf-red': priceRow.red }">
                      <span v-for="(part, idx) in priceLabelParts(priceRow, item)" :key="`ftl-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </span>
                    <strong :class="priceValueClass(priceRow, item)">
                      <span v-for="(part, idx) in priceValueParts(priceRow, item)" :key="`ftv-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </strong>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>

          <div v-else class="pdf-card-grid">
            <div v-for="(row, rowIndex) in cardRows(group)" :key="`pdf-row-${group.category}-${rowIndex}`" :class="['pdf-card-row', `cards-${row.columns}`]" :style="cardRowStyle(row)">
              <article v-for="item in row.items" :key="`pdf-${group.category}-${item.code}`" class="pdf-item">
                <div class="pdf-item-head">
                  <span>{{ item.code }}</span>
                  <div>
                    <h3>
                      <span v-for="(part, idx) in highlightedParts(item.name, item)" :key="`cn-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                      <span v-if="item.badgeLabel" :class="badgeClass(item.badge)">{{ item.badgeLabel }}</span>
                    </h3>
                  </div>
                </div>
                <div class="pdf-meta-block">
                  <p v-for="line in item.attributeLines || []" :key="`pdf-attr-${item.code}-${line}`" class="pdf-meta-line"><b>属性</b><span>{{ line }}</span></p>
                </div>
                <div class="pdf-price-block">
                  <div class="pdf-section-label">报价</div>
                  <div class="pdf-price-list">
                    <div v-for="priceRow in item.prices" :key="`${item.code}-${priceRow.label}`" class="pdf-price">
                      <span class="pdf-price-label" :class="{ 'pdf-red': priceRow.red }">
                        <span v-for="(part, idx) in priceLabelParts(priceRow, item)" :key="`fpl-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                      </span>
                      <strong class="pdf-price-value" :class="priceValueClass(priceRow, item)">
                        <span v-for="(part, idx) in priceValueParts(priceRow, item)" :key="`fpv-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                      </strong>
                    </div>
                  </div>
                </div>
              </article>
            </div>
          </div>
        </section>

        <div v-if="pdfTheme.showChangelog && pdfTheme.changelog" class="pdf-changelog pdf-bottom-changelog">
          <b>更新</b>
          <span>{{ pdfTheme.changelog }}</span>
        </div>

        <footer class="pdf-footer">
          <span>{{ pdfTheme.brandName }}</span>
          <span>联系电话：15302787466</span>
        </footer>
      </section>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchCurrentActor } from '../api/auth'
import { apiFetch, apiGet, apiSend } from '../api/client'
import PaginationControls from '../components/PaginationControls.vue'
import {
  DEFAULT_BEAN_LIST_PDF_VERSION,
  applyCustomerProductAliasesToBeanListItems,
  beanListPublicationPdfOptions,
  buildPriceListGenerationSnapshot,
  buildBeanListPdfGroups,
  buildBeanListPdfGroupsFromCategoryRows,
  buildBeanListPdfSubtitle,
  buildBeanListPdfTitle,
  applyPriceListFlatRowsToBeanListPdfGroups,
  copyBeanListPublicationContentGroups,
  defaultBeanListDraftVersion,
  filterBeanListItemsForPriceTableScope,
  filterBeanListItemsForScope,
  sanitizeBeanListPdfTheme,
  splitHighlightedText,
} from '../lib/bean-list-pdf'
import {
  buildPriceExplanationRequest,
  gradientDisplayUnitLabel,
} from '../lib/gradient-templates'
import {
  UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID,
  buildClassificationPriceListTypeOptions as buildClassificationPriceListTypeOptionsFromItems,
  buildProductCatalogPriceListTypeOptions,
  classificationCategoryIDOfItem as currentClassificationCategoryIDOfItem,
  classificationCategoryNameOfItem as currentClassificationCategoryNameOfItem,
  classificationTemplateIDOfItem as currentClassificationTemplateIDOfItem,
  classificationTemplateIDOfPublication as currentClassificationTemplateIDOfPublication,
  classificationTemplateNameOfItem as currentClassificationTemplateNameOfItem,
  classificationTemplateNameOfPublication as currentClassificationTemplateNameOfPublication,
  matchesPublicationProductType as matchesCurrentPublicationProductType,
  matchesProductCatalogPriceListType,
  matchesProductTypeCategory as matchesCurrentProductTypeCategory,
  publicationVersionListState,
  priceListRenderTypeForItem as currentPriceListRenderTypeForItem,
  priceListSelectionStateKey,
  productTypeCategoryIDOfItem as currentProductTypeCategoryIDOfItem,
  productTypeNameOfItem as currentProductTypeNameOfItem,
} from '../lib/product-price-list-types'
import {
  buildPriceListProductFamilies,
  defaultPriceListProductSpecSelections,
  normalizePriceListProductSpecSelections,
  priceListCategoryCodesForSelectedProducts,
  priceListCategoryHiddenByCollapsedAncestor,
  priceListCategoryProductIDs,
  priceListParentProductID,
  priceListProductSpecLabel,
  priceListProductSpecSelectionIssue,
  priceListProductSpecSelectionCounts,
  priceListSelectedSkuCategoryRows,
  priceListSkuID,
  resolvePriceListProductSpecSelectionIssue,
  setPriceListCategorySpecSelection,
  togglePriceListProductSpecSelection,
  priceListVisibleCategoryRows,
} from '../lib/product-price-list-selection'
import {
  businessGroupControlOptions,
  businessGroupRowsForUsage,
  groupRowsByBusinessGroupTemplate,
} from '../lib/business-grouping'
import {
  normalizeParentSharedPriceListProductOverrides,
  priceListGenerationDraftKey,
  readPriceListGenerationDraft,
  savePriceListGenerationDraft,
} from '../lib/product-price-list-draft'
import {
  applyPricingRuleTrialToPriceTableRow,
  buildPriceTierTemplatePayload,
  productCurrentSalesSpecUnit,
  priceTierTemplateRowKey,
  priceTierTemplateUnitCompatibility,
  priceTablePricingRuleTrialCacheKey,
  priceTablePricingRuleTrialPayload,
  priceTablePricingModeOptions,
  resolvePriceTableTemplateInheritance,
} from '../lib/product-settings'
import {
  dedupePriceListFlatRows,
  executePriceListPricingRuleTrialBatches,
  priceListFlatRowDisplayTitle,
  priceListFlatRowErrors,
  priceListFlatRowPriceUnitLabel,
  priceListFlatRowSpecDescription,
  priceListFlatRowsReady as arePriceListFlatRowsReady,
  priceListSalesSpecCountTierLabel,
  priceListPricingRuleTrialCacheForRetry,
  priceListPricingRuleTrialRequestsForRows as buildPriceListPricingRuleTrialRequests,
} from '../lib/costing-price-list-workflow.js'
import { FORM_DRAFT_SCOPES, readFormDraft } from '../lib/form-draft-cache'
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'
import {
  PRICE_LIST_PAGE_PREFERENCES_KEY,
  readPriceListPagePreferences,
  resolvePriceListScopePreference,
  resolveProductTypePreference,
  writePriceListPagePreferences,
} from '../lib/price-list-page-preferences'

const props = defineProps({
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const SKU_SETTINGS_FORM_DRAFT_SCOPE = FORM_DRAFT_SCOPES.skuSettings
const FACTORY_SUPPLY_PUBLICATION_PURPOSE = 'factory_supply'
const initialPriceListPagePreferences = readPriceListPagePreferences()

const loading = ref(false)
const beanListPublishing = ref(false)
const beanListWithdrawing = ref(false)
const beanListArchiving = ref(false)
const beanListPdfGenerating = ref(false)
const priceExplanationOpen = ref(false)
const priceExplanationLoading = ref(false)
const pdfDrawerOpen = ref(false)
const pdfPrinting = ref(false)
const versionListScope = ref(initialPriceListPagePreferences.scope)
const publicationListSearch = ref('')
const publicationListPage = ref(1)
const publicationListPageSize = ref(10)
const publicationListCollapsed = ref(false)
const publicationArchiveListPage = ref(1)
const publicationArchiveListPageSize = ref(10)
const publicationArchiveListCollapsed = ref(true)
const selectedPublicationArchiveIDs = ref([])
const publicationScope = ref('official')
const selectedBeanListCustomerID = ref(0)
const actorLoaded = ref(false)
const currentActor = ref(null)
const selectedPriceSourcePublicationID = ref('')
const selectedProductTypeCategoryID = ref(initialPriceListPagePreferences.productTypeCategoryID)
const downloadSourcePublication = ref(null)
const error = ref('')
const message = ref('')
const priceExplanationError = ref('')
const parameters = ref(null)
const items = ref([])
const priceExplanation = ref(null)
const explanationItem = ref(null)
const explanationTier = ref(null)
const explanationOverrides = ref({
  green_bean_cost_per_kg: '',
  yield_rate: '',
  margin_rate: '',
})
const customers = ref([])
const customerProductAliases = ref([])
const beanListPublications = ref({
  official: {},
  mine: {},
  customer: {},
})
const priceSourcePublicationByType = ref({})
const styleSourcePublicationIDByType = ref({})
const productSpecSelectionsByType = ref({})
const visibleCategoryCodesByType = ref({})
const productSelectionInitialized = ref({})
const categorySelectionInitialized = ref({})
const pdfCustomizers = ref({})
const pdfOptions = ref({
  listType: 'commercial',
  version: DEFAULT_BEAN_LIST_PDF_VERSION,
  brandName: '棵凡咖啡',
  backgroundColor: '#f8f1e5',
  fontColor: '#171717',
  backgroundImage: '',
  logoImage: '',
  brandIntro: '',
  layoutStyle: 'card',
  cardsPerRow: 2,
  showCategoryNumbers: true,
  showVersion: true,
  showChangelog: true,
  changelog: '',
})
const priceTierTemplates = ref([])
const pricingRules = ref([])
const priceListTemplateDefaults = ref(defaultPriceListTemplateSelection({ pricing_mode: 'tier_template' }))
const priceListParentTemplateSelections = ref({})
const priceListGroupTemplateSelections = ref({})
const priceListProductTemplateOverrides = ref({})
const priceListLegacyPricingConflicts = ref([])
const priceListFlatRowOverrides = ref({})
const priceListPricingRuleTrialCache = ref({})
const priceListProductBusinessGroups = ref([])
const priceListProductBusinessGroupAssignments = ref([])
const selectedProductCatalogGroupTemplateID = ref(0)
const priceListConfigDialog = ref(defaultPriceListConfigDialog())
const priceListPricingPopover = ref(defaultPriceListPricingPopover())
const productPickerCollapsedCategories = ref({})
const priceListRulesDialogOpen = ref(false)
const tierTemplateDrawerOpen = ref(false)
const tierTemplateSaving = ref(false)
const priceTierTemplateForm = ref(defaultPriceTierTemplateForm())
let priceListPricingRuleTrialRefreshScheduled = false
const priceListPricingPopoverOptions = [
  { value: '', label: '继承分类' },
  { value: 'tier_template', label: '按阶梯模板价计算' },
  { value: 'pricing_rule', label: '按价格模板计算' },
  { value: 'fixed_price', label: '固定价' },
]

const normalizedCustomerContextID = computed(() => Number(props.customerContextId || 0))
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && normalizedCustomerContextID.value > 0)
const activeBeanListCustomerID = computed(() => normalizedCustomerContextID.value || versionListScopeCustomerID(versionListScope.value) || Number(selectedBeanListCustomerID.value || 0))
const activeCostingScope = computed(() => {
  return activeBeanListCustomerID.value > 0 ? 'customer' : 'official'
})
const activePriceListCustomerAliases = computed(() => {
  const customerID = Number(activeBeanListCustomerID.value || 0)
  if (customerID <= 0) return []
  return customerProductAliases.value.filter((row) => {
    return Number(row?.customer_id || 0) === customerID && row?.active !== false && row?.include_in_price_list !== false
  })
})
const visibleCostingItems = computed(() => {
  const scoped = filterBeanListItemsForPriceTableScope(items.value, activeCostingScope.value, activeBeanListCustomerID.value)
  if (activeCostingScope.value !== 'customer') return scoped
  return applyCustomerProductAliasesToBeanListItems(scoped, activePriceListCustomerAliases.value, activeBeanListCustomerID.value)
})
const customerScopedSkuCount = computed(() => {
  const customerID = Number(activeBeanListCustomerID.value || 0)
  if (!customerID || activeCostingScope.value !== 'customer') return visibleCostingItems.value.length
  return activePriceListCustomerAliases.value.length
})
const productPriceListTypeOptions = computed(() => {
  const productCatalogOptions = buildProductCatalogPriceListTypeOptions(visibleCostingItems.value, {
    template: selectedProductCatalogGroupTemplate.value,
    assignments: priceListProductBusinessGroupAssignments.value,
  })
  return productCatalogOptions.length ? productCatalogOptions : buildClassificationPriceListTypeOptions(visibleCostingItems.value)
})
const selectedProductPriceListType = computed(() => {
  const selectedID = Number(selectedProductTypeCategoryID.value || 0)
  return productPriceListTypeOptions.value.find((type) => Number(type.id || 0) === selectedID) || productPriceListTypeOptions.value[0] || null
})
const activeProductTypeCategoryID = computed(() => Number(selectedProductPriceListType.value?.id || selectedProductTypeCategoryID.value || 0))
const selectedProductPriceListLabel = computed(() => selectedProductPriceListType.value?.label || beanListTypeLabel(pdfTheme.value.listType))
const activePriceListTypeKey = computed(() => productPriceListTypeKey(selectedProductPriceListType.value, pdfTheme.value.listType))
const productCatalogBusinessGroupControls = computed(() => businessGroupControlOptions(productCatalogBusinessGroupRowsForPriceList(), {
  selectedTemplateID: selectedProductCatalogGroupTemplateID.value,
  usageKey: 'product_catalog',
}))
const selectedProductCatalogGroupTemplate = computed(() => (
  productCatalogBusinessGroupControls.value.selectedTemplate ||
  productCatalogBusinessGroupControls.value.templateOptions[0]?.group ||
  null
))
const selectedProductCatalogGroupItemIDs = computed(() => new Set(
  productCatalogBusinessGroupControls.value.moveOptions
    .map((option) => Number(option.group_item_id || 0))
    .filter(Boolean),
))
const pdfTheme = computed(() => sanitizeBeanListPdfTheme(pdfOptions.value))
const pdfAvailableItems = computed(() => priceListProductFamiliesForType(pdfTheme.value.listType, activeProductTypeCategoryID.value))
const pdfCategoryOptions = computed(() => beanListCategoryOptions(pdfTheme.value.listType, activeProductTypeCategoryID.value))
const pdfProductSpecSelections = computed(() => productSpecSelectionsByType.value[activePriceListTypeKey.value] || [])
const pdfSelectedProductIDs = computed(() => pdfProductSpecSelections.value.map((row) => String(row.sku_id || '')).filter(Boolean))
const pdfProductSpecSelectionCounts = computed(() => priceListProductSpecSelectionCounts(pdfProductSpecSelections.value))
const pdfProductSpecSelectionIssues = computed(() => pdfAvailableItems.value
  .map((family) => ({ family, issue: priceListProductSpecSelectionIssue(family, pdfProductSpecSelections.value) }))
  .filter((row) => row.issue))
const priceListProductSpecSelectionBlockedReason = computed(() => String(pdfProductSpecSelectionIssues.value[0]?.issue?.message || '').trim())
const pdfVisibleCategoryCodes = computed(() => visibleCategoryCodesByType.value[activePriceListTypeKey.value] || [])
const categoryProductGroups = computed(() => productGroupsForType(pdfTheme.value.listType, activeProductTypeCategoryID.value))
const selectedSkuCategoryProductGroups = computed(() => priceListSelectedSkuCategoryRows(categoryProductGroups.value, pdfProductSpecSelections.value))
const pdfGenerationCustomizers = computed(() => {
  const out = { ...pdfCustomizers.value }
  categoryProductGroups.value.forEach((category) => {
    ;(category.items || []).forEach((family) => {
      const parentCustomizer = pdfCustomizers.value[String(priceListParentProductID(family))] || {}
      ;(family.sku_options || []).forEach((spec) => {
        const skuKey = String(priceListSkuID(spec))
        out[skuKey] = { ...parentCustomizer, ...(pdfCustomizers.value[skuKey] || {}) }
      })
    })
  })
  return out
})
const pdfGenerationOptions = computed(() => ({
  selectedProductIDs: pdfSelectedProductIDs.value,
  showCategoryNumbers: pdfOptions.value.showCategoryNumbers,
  visibleCategoryCodes: pdfVisibleCategoryCodes.value,
  customizers: pdfGenerationCustomizers.value,
}))
const currentPriceSourcePublication = computed(() => (publicationScope.value === 'mine' || publicationScope.value === 'customer' ? priceSourcePublicationByType.value[activePriceListTypeKey.value] : null))
const basePdfGroups = computed(() => {
  if (downloadSourcePublication.value?.content?.groups) {
    return copyBeanListPublicationContentGroups(downloadSourcePublication.value, {
      listType: pdfTheme.value.listType,
      customizers: pdfCustomizers.value,
    })
  }
  return buildBeanListPdfGroupsFromCategoryRows(selectedSkuCategoryProductGroups.value, pdfTheme.value.listType, pdfGenerationOptions.value)
})
const priceListGroupTemplateRows = computed(() => priceListTemplateGroupRows(categoryProductGroups.value))
const priceListFlatRows = computed(() => dedupePriceListFlatRows(priceListFlatRowsFromGroups(basePdfGroups.value)))
const pdfGroups = computed(() => applyPriceListFlatRowsToBeanListPdfGroups(basePdfGroups.value, priceListFlatRows.value, pdfTheme.value.listType))
const priceListTierUnitBlockedReason = computed(() => String(
  priceListFlatRows.value.find((row) => priceListFlatRowHasLegacyUnitMismatch(row))?.tier_unit_compatibility_error || '',
).trim())
const priceListFlatRowsReady = computed(() => arePriceListFlatRowsReady(priceListFlatRows.value, {
  trialStatusForRow: priceListFlatRowPricingTrialStatus,
}))
const priceListLegacyPricingBlockedReason = computed(() => {
  if (!priceListLegacyPricingConflicts.value.length) return ''
  return '旧草稿中同一商品存在不同的规格级计价配置，请在“商品计价”重新选择一次后继续。'
})
const priceListPricingRuleTrialRequests = computed(() => currentPriceListPricingRuleTrialRequests(priceListFlatRows.value))
const priceListPricingRuleTrialFailedCount = computed(() => priceListFlatRows.value.filter((row) => (
  priceListFlatRowPricingTrialStatus(row) === 'error'
  && priceListFlatRowVisibleErrors(row).some((message) => message.includes('价格计算失败'))
)).length)

function currentPriceListPricingRuleTrialRequests(sourceRows = []) {
  return buildPriceListPricingRuleTrialRequests(sourceRows, {
    customerID: activeBeanListCustomerID.value,
    cache: priceListPricingRuleTrialCache.value,
    payloadForRow: priceTablePricingRuleTrialPayload,
    cacheKeyForPayload: priceTablePricingRuleTrialCacheKey,
  })
}

function schedulePriceListPricingRuleTrialRefresh() {
  if (priceListPricingRuleTrialRefreshScheduled) return
  priceListPricingRuleTrialRefreshScheduled = true
  nextTick(() => {
    priceListPricingRuleTrialRefreshScheduled = false
    clearPriceListPricingRuleTrialErrorCache(priceListFlatRows.value)
    const requests = currentPriceListPricingRuleTrialRequests(priceListFlatRows.value)
    if (requests.length) {
      loadPriceListPricingRuleTrials(requests)
    }
  })
}

function retryPriceListPricingRuleTrials() {
  clearPriceListPricingRuleTrialErrorCache(priceListFlatRows.value)
  schedulePriceListPricingRuleTrialRefresh()
}

const pdfTotalItems = computed(() => pdfGroups.value.reduce((sum, group) => sum + group.items.length, 0))
const pdfTitle = computed(() => buildProductPriceListTitle(pdfTheme.value.brandName, selectedProductPriceListLabel.value, pdfTheme.value.listType))
const pdfSubtitle = computed(() => buildProductPriceListSubtitle(selectedProductPriceListLabel.value, pdfTheme.value.listType))
const isBeanListAdmin = computed(() => {
  const actor = currentActor.value || {}
  const roles = Array.isArray(actor.roles) ? actor.roles : []
  const permissions = Array.isArray(actor.permissions) ? actor.permissions : []
  return Boolean(
    actor.basic_auth_admin ||
    roles.some((role) => String(role?.code || '').trim().toLowerCase() === 'admin') ||
    permissions.includes('auth.manage'),
  )
})
const customerScopeReady = computed(() => publicationScope.value !== 'customer' || Number(selectedBeanListCustomerID.value || 0) > 0)
const currentScopeAllPublicationRows = computed(() => publicationRows(versionListScope.value, pdfTheme.value.listType, activeProductTypeCategoryID.value, FACTORY_SUPPLY_PUBLICATION_PURPOSE))
const currentScopeActivePublicationRows = computed(() => currentScopeAllPublicationRows.value.filter((row) => row.status !== 'archived'))
const currentScopeArchivedPublicationRows = computed(() => currentScopeAllPublicationRows.value.filter((row) => row.status === 'archived'))
const currentScopePublicationRows = computed(() => currentScopeActivePublicationRows.value)
const publicationListState = computed(() => publicationVersionListState(currentScopePublicationRows.value, {
  query: publicationListSearch.value,
  page: publicationListPage.value,
  pageSize: publicationListPageSize.value,
  collapsed: publicationListCollapsed.value,
}))
const paginatedCurrentScopePublicationRows = computed(() => publicationListState.value.rows)
const versionListCurrentPublication = computed(() => currentScopePublicationRows.value.find((row) => row.status === 'published') || null)
const archivedPublicationListState = computed(() => publicationVersionListState(currentScopeArchivedPublicationRows.value, {
  query: publicationListSearch.value,
  page: publicationArchiveListPage.value,
  pageSize: publicationArchiveListPageSize.value,
  collapsed: publicationArchiveListCollapsed.value,
}))
const paginatedArchivedPublicationRows = computed(() => archivedPublicationListState.value.rows)
const archiveSelectableCurrentPagePublicationRows = computed(() => paginatedCurrentScopePublicationRows.value.filter((row) => canArchiveBeanListPublication(row)))
const currentPagePublicationArchiveAllSelected = computed(() => archiveSelectableCurrentPagePublicationRows.value.length > 0 && archiveSelectableCurrentPagePublicationRows.value.every((row) => isPublicationArchiveSelected(row)))
const currentPagePublicationArchiveSomeSelected = computed(() => archiveSelectableCurrentPagePublicationRows.value.some((row) => isPublicationArchiveSelected(row)))
const publicationScopeRows = computed(() => publicationRows(publicationScope.value, pdfTheme.value.listType, activeProductTypeCategoryID.value, 'factory_supply'))
const currentBeanListPublication = computed(() => publicationScopeRows.value.find((row) => row.status === 'published') || null)
const officialPriceSourcePublications = computed(() => publicationRows('official', pdfTheme.value.listType, activeProductTypeCategoryID.value, 'factory_supply').filter((row) => row.status === 'published'))
const selectedPriceSourcePublication = computed(() => officialPriceSourcePublications.value.find((row) => String(row.id) === String(selectedPriceSourcePublicationID.value)) || null)
const currentPublicationOwnerLabel = computed(() => publicationScopeLabel(publicationScope.value))
const currentPublicationScopeDescription = computed(() => {
  if (publicationScope.value === 'customer') return '生成和发布会保存到当前价格表归属对应的履约客户。'
  if (publicationScope.value === 'mine') return '当前客户账号保存自己的价格表修改。'
  return '生成和发布会保存到公共价格表。'
})
const publicBeanListURL = computed(() => {
  if (publicationScope.value !== 'official' || !currentBeanListPublication.value) return ''
  const params = new URLSearchParams()
  const productTypeCategoryID = currentClassificationTemplateIDOfPublication(currentBeanListPublication.value)
  if (productTypeCategoryID > 0) params.set('product_type_category_id', String(productTypeCategoryID))
  const query = params.toString()
  return `${window.location.origin}/public/bean-list/${pdfTheme.value.listType}${query ? `?${query}` : ''}`
})
const priceListPublishBlockedReason = computed(() => {
  if (priceListProductSpecSelectionBlockedReason.value) return priceListProductSpecSelectionBlockedReason.value
  if (priceListLegacyPricingBlockedReason.value) return priceListLegacyPricingBlockedReason.value
  if (!pdfGroups.value.length) return '暂无可发布的价格表预览'
  if (!String(pdfTheme.value.version || '').trim()) return '请填写价格表版本号'
  if (!customerScopeReady.value) return '请选择客户'
  if (priceListTierUnitBlockedReason.value) return priceListTierUnitBlockedReason.value
  if (!priceListFlatRowsReady.value) return '平铺价格行存在未完成项目，请按红色行提示补齐。'
  return ''
})
const pdfPageStyle = computed(() => {
  const bg = pdfTheme.value.backgroundImage
  return {
    color: pdfTheme.value.fontColor,
    backgroundColor: pdfTheme.value.backgroundColor,
    backgroundImage: bg ? `linear-gradient(rgba(255,255,255,.74), rgba(255,255,255,.74)), url(${bg})` : 'none',
  }
})

watch(productPriceListTypeOptions, (options) => {
  const resolvedID = resolveProductTypePreference(selectedProductTypeCategoryID.value, options)
  if (resolvedID !== Number(selectedProductTypeCategoryID.value || 0)) {
    selectedProductTypeCategoryID.value = resolvedID
    return
  }
  if (!options.length) return
  syncPdfListTypeFromSelectedProductType()
}, { immediate: true })

watch(selectedProductTypeCategoryID, () => {
  writePriceListPagePreferences({ productTypeCategoryID: selectedProductTypeCategoryID.value })
  selectedPriceSourcePublicationID.value = ''
  syncPdfListTypeFromSelectedProductType()
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  restorePriceListGenerationDraftForActiveType()
  loadBeanListPublications(pdfTheme.value.listType, versionListScope.value, activeProductTypeCategoryID.value)
  loadBeanListPublications(pdfTheme.value.listType, 'official', activeProductTypeCategoryID.value, 'factory_supply')
  loadBeanListPublications(pdfTheme.value.listType, 'mine', activeProductTypeCategoryID.value, 'factory_supply')
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(pdfTheme.value.listType, 'customer', activeProductTypeCategoryID.value, 'factory_supply')
  }
})

watch(() => pdfOptions.value.listType, (listType) => {
  selectedPriceSourcePublicationID.value = ''
  initializePdfDefaultsForType(listType, activeProductTypeCategoryID.value)
  restorePriceListGenerationDraftForActiveType()
  loadBeanListPublications(listType, versionListScope.value, activeProductTypeCategoryID.value)
  loadBeanListPublications(listType, 'official', activeProductTypeCategoryID.value, 'factory_supply')
  loadBeanListPublications(listType, 'mine', activeProductTypeCategoryID.value, 'factory_supply')
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(listType, 'customer', activeProductTypeCategoryID.value, 'factory_supply')
  }
})

watch(versionListScope, (scope) => {
  if (!isWorkspaceCustomerLocked.value) writePriceListPagePreferences({ scope })
  syncPublicationScopeFromPageContext()
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  loadBeanList()
  loadBeanListPublications(pdfTheme.value.listType, scope, activeProductTypeCategoryID.value)
})

watch([versionListScope, selectedProductTypeCategoryID, publicationListSearch], () => {
  publicationListPage.value = 1
  publicationArchiveListPage.value = 1
  selectedPublicationArchiveIDs.value = []
})

watch(activeBeanListCustomerID, () => {
  loadBeanList()
})

watch(publicationScope, (scope) => {
  loadBeanListPublications(pdfTheme.value.listType, scope, activeProductTypeCategoryID.value, 'factory_supply')
  initializePdfDefaultsForType(pdfTheme.value.listType, activeProductTypeCategoryID.value)
  restorePriceListGenerationDraftForActiveType()
})

watch(selectedBeanListCustomerID, () => {
  beanListPublications.value = {
    ...beanListPublications.value,
    customer: {},
  }
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  restorePriceListGenerationDraftForActiveType()
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(pdfTheme.value.listType, 'customer', activeProductTypeCategoryID.value, 'factory_supply')
  }
  notifyWorkspaceCustomerChanged(selectedBeanListCustomerID.value)
})

watch(isBeanListAdmin, (canPublish) => {
  void canPublish
  syncPublicationScopeFromPageContext()
})

watch(() => props.customerContextId, syncPublicationScopeFromPageContext, { immediate: true })

watch(priceListPricingRuleTrialRequests, (requests) => {
  if (!requests.length) return
  loadPriceListPricingRuleTrials(requests)
}, { deep: true, immediate: true })

function syncPublicationScopeFromPageContext() {
  const pageCustomerID = Number(props.customerContextId || 0) || versionListScopeCustomerID(versionListScope.value)
  if (pageCustomerID > 0) {
    selectedBeanListCustomerID.value = pageCustomerID
    versionListScope.value = `customer:${pageCustomerID}`
    publicationScope.value = 'customer'
  } else {
    selectedBeanListCustomerID.value = 0
    publicationScope.value = 'official'
  }
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  if (publicationScope.value === 'customer') {
    loadBeanListPublications(pdfTheme.value.listType, 'customer', activeProductTypeCategoryID.value, 'factory_supply')
  }
}

function notifyWorkspaceCustomerChanged(customerID) {
  if (props.workspaceMode !== CUSTOMER_WORKSPACE_MODE || Number(customerID || 0) <= 0) return
  if (Number(customerID || 0) === Number(props.customerContextId || 0)) return
  window.dispatchEvent(workspaceCustomerChangeEvent(customerID))
}

function tierPriceValue(tier) {
  return Number(tier?.price_per_unit || tier?.price_per_lb || 0)
}

function tierUnit(tier) {
  if (tier?.display_unit) return gradientDisplayUnitLabel(tier.display_unit).replace('元/', '')
  const specG = Number(tier?.spec_g || 454)
  if (specG === 1000) return 'kg'
  if (specG === 227) return '227g'
  if (specG === 250) return '250g'
  if (specG === 100) return '100g'
  return '包'
}

function customerOptionLabel(customer) {
  return customer?.name || ''
}

function buildClassificationPriceListTypeOptions(sourceItems = []) {
  return buildClassificationPriceListTypeOptionsFromItems(sourceItems)
}

function buildProductPriceListTypeOptions(sourceItems = []) {
  return buildClassificationPriceListTypeOptions(sourceItems)
}

function classificationTemplateIDOfItem(item) {
  return currentClassificationTemplateIDOfItem(item)
}

function classificationCategoryIDOfItem(item) {
  return currentClassificationCategoryIDOfItem(item)
}

function classificationTemplateNameOfItem(item) {
  return currentClassificationTemplateNameOfItem(item)
}

function classificationCategoryNameOfItem(item) {
  return currentClassificationCategoryNameOfItem(item)
}

function productTypeCategoryIDOfItem(item) {
  return currentProductTypeCategoryIDOfItem(item)
}

function productTypePositionOfItem(item) {
  return Number(item?.category_primary_position || item?.product_type_position || item?.productTypePosition || 999999)
}

function productTypeNameOfItem(item) {
  return currentProductTypeNameOfItem(item)
}

function priceListRenderTypeForItem(item) {
  return currentPriceListRenderTypeForItem(item)
}

function fallbackProductTypeID(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return -2
  if (normalized === 'drip') return -3
  if (normalized === 'retail') return -4
  return -1
}

function productPriceListTypeKey(type, listType = 'commercial') {
  const explicitKey = String(type?.key || '')
  if (explicitKey.startsWith('product-catalog:')) return explicitKey
  const id = Number(type?.categoryID || type?.id || 0)
  if (id === UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID) return 'classification:unclassified'
  if (id > 0) return `product-type:${id}`
  return `legacy:${normalizeBeanListType(type?.listType || listType)}`
}

function priceListSelectionKey(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  return priceListSelectionStateKey(productPriceListTypeOptions.value, listType, productTypeCategoryID)
}

function priceListGenerationDraftStorageKey() {
  return priceListGenerationDraftKey({
    workspace: props.workspaceMode || 'factory',
    scope: publicationScope.value || activeCostingScope.value || 'official',
    customerID: activeBeanListCustomerID.value,
    typeKey: activePriceListTypeKey.value,
  })
}

function productCatalogBusinessGroupRowsForPriceList() {
  return businessGroupRowsForUsage(priceListProductBusinessGroups.value, 'product_catalog')
}

function normalizePriceListSelectionDraftMap(rows = {}, options = {}) {
  const includeProductMeta = Boolean(options.includeProductMeta)
  const out = {}
  for (const [key, value] of Object.entries(rows || {})) {
    const selection = defaultPriceListTemplateSelection(value || {})
    if (!priceListTemplateHasOverride(selection)) continue
    out[key] = includeProductMeta
      ? {
        ...selection,
        product_id: Number(value?.product_id ?? value?.productID ?? 0) || 0,
        sku_id: Number(value?.sku_id ?? value?.skuID ?? value?.product_id ?? value?.productID ?? 0) || 0,
        parent_product_id: Number(value?.parent_product_id ?? value?.parentProductID ?? 0) || 0,
        scope: String(value?.scope ?? value?.override_scope ?? value?.overrideScope ?? '').trim(),
        product_key: String(value?.product_key ?? value?.productKey ?? '').trim(),
        product_name: String(value?.product_name ?? value?.productName ?? '').trim(),
        parent_product_name: String(value?.parent_product_name ?? value?.parentProductName ?? '').trim(),
        sku_name: String(value?.sku_name ?? value?.skuName ?? '').trim(),
      }
      : selection
  }
  return out
}

function normalizePriceListFlatRowOverrides(rows = {}) {
  const out = {}
  for (const [key, value] of Object.entries(rows || {})) {
    const price = Number(value || 0)
    if (key && Number.isFinite(price) && price > 0) out[key] = price
  }
  return out
}

function savePriceListGenerationDraftForActiveType() {
  savePriceListGenerationDraft(priceListGenerationDraftStorageKey(), {
    defaults: priceListTemplateDefaults.value,
    parentSelections: priceListParentTemplateSelections.value,
    groupSelections: priceListGroupTemplateSelections.value,
    productOverrides: priceListProductTemplateOverrides.value,
    flatRowOverrides: priceListFlatRowOverrides.value,
    product_spec_selections: pdfProductSpecSelections.value,
  })
}

function restorePriceListGenerationDraftForActiveType() {
  priceListLegacyPricingConflicts.value = []
  const draft = readPriceListGenerationDraft(priceListGenerationDraftStorageKey())
  if (!draft) return false
  priceListTemplateDefaults.value = {
    ...priceListTemplateDefaults.value,
    ...defaultPriceListTemplateSelection(draft.defaults || {}),
  }
  priceListParentTemplateSelections.value = normalizePriceListSelectionDraftMap(draft.parentSelections)
  priceListGroupTemplateSelections.value = normalizePriceListSelectionDraftMap(draft.groupSelections)
  const migratedProductOverrides = normalizeParentSharedPriceListProductOverrides(draft.productOverrides, {
    productSpecSelections: draft.product_spec_selections,
  })
  priceListProductTemplateOverrides.value = normalizePriceListSelectionDraftMap(migratedProductOverrides.overrides, { includeProductMeta: true })
  priceListLegacyPricingConflicts.value = migratedProductOverrides.conflicts
  priceListFlatRowOverrides.value = normalizePriceListFlatRowOverrides(draft.flatRowOverrides)
  if (Object.prototype.hasOwnProperty.call(draft, 'product_spec_selections')) {
    const key = activePriceListTypeKey.value
    productSpecSelectionsByType.value = {
      ...productSpecSelectionsByType.value,
      [key]: normalizePriceListProductSpecSelections(draft.product_spec_selections, pdfAvailableItems.value, { fallbackInvalid: true }),
    }
    syncCategoryVisibilityFromSelectedProducts(pdfTheme.value.listType, pdfSelectedProductIDs.value, activeProductTypeCategoryID.value)
  }
  schedulePriceListPricingRuleTrialRefresh()
  return true
}

function beanListPublicationTypeKey(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  if (Number(productTypeCategoryID || 0) === UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID) return 'classification:unclassified'
  const id = activePublicationProductTypeCategoryID(productTypeCategoryID)
  if (id > 0) return `product-type:${id}`
  return `legacy:${normalizeBeanListType(listType)}`
}

function activePublicationProductTypeCategoryID(productTypeCategoryID = activeProductTypeCategoryID.value) {
  const id = Number(productTypeCategoryID || 0)
  return id > 0 ? id : 0
}

function syncPdfListTypeFromSelectedProductType() {
  const selected = selectedProductPriceListType.value
  if (!selected?.listType) return
  if (pdfOptions.value.listType !== selected.listType) {
    pdfOptions.value = { ...pdfOptions.value, listType: selected.listType }
  }
}

function syncSelectedProductTypeCategoryFromOptions() {
  const options = productPriceListTypeOptions.value
  if (!options.length) {
    selectedProductTypeCategoryID.value = 0
    return
  }
  if (!options.some((type) => Number(type.id || 0) === Number(selectedProductTypeCategoryID.value || 0))) {
    selectedProductTypeCategoryID.value = Number(options[0].id || 0)
  }
  syncPdfListTypeFromSelectedProductType()
}

function buildProductPriceListTitle(brandName, productTypeLabel, listType) {
  const brand = String(brandName || '棵凡咖啡').trim() || '棵凡咖啡'
  const label = String(productTypeLabel || '').trim()
  if (label) return `${brand}${label}商品价格表`
  return buildBeanListPdfTitle(listType, brand)
}

function buildProductPriceListSubtitle(productTypeLabel, listType) {
  const label = String(productTypeLabel || '').trim()
  if (normalizeBeanListType(listType) === 'green') return `${label || '生豆'}销售报价`
  if (label) return `${label}产品报价`
  return buildBeanListPdfSubtitle(listType)
}

function customerOptionMeta(customer) {
  const parts = []
  if (customer?.company_name && customer.company_name !== customer?.name) parts.push(customer.company_name)
  if (customer?.contact) parts.push(customer.contact)
  if (customer?.phone || customer?.company_phone) parts.push(customer.phone || customer.company_phone)
  return parts.join(' / ')
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
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

function itemWarnings(item) {
  const warnings = Array.isArray(item?.warnings)
    ? item.warnings.filter((warning) => warning && !isInactiveBomWarningText(warning))
    : []
  if (item?.bom_status === 'missing_green_bean_template' && !warnings.some((warning) => String(warning).includes('未挂到带生豆模板的分类'))) {
    return ['未挂到带生豆模板的分类，无法生成生豆价格。请在商品管理里把该生豆商品移到带生豆模板的生豆分类。', ...warnings]
  }
  return warnings
}

function visibleItemWarnings(item) {
  return itemWarnings(item).filter((warning) => {
    const text = String(warning || '').trim()
    if (text === '未设置计价方式' && itemHasResolvedPriceListPricingMethod(item)) return false
    return true
  })
}

function priceListResolvedTemplateForItem(item = {}) {
  const representative = selectedSpecsForProduct(item)[0]
  const source = representative || item
  const groupRow = priceListGroupForItem(source)
  const sourceProductID = Number(source?.product_id || source?.productID || source?.productId || source?.id || itemProductID(source) || 0)
  const skuID = itemSkuID(source, sourceProductID)
  const productID = skuID || sourceProductID
  const product = {
    id: productID,
    product_id: productID,
    sku_id: skuID,
    parent_product_id: itemParentProductID(source, skuID) || priceListParentProductID(item),
    group_item_id: groupRow.group_item_id,
    parent_group_item_id: groupRow.parent_group_item_id,
  }
  return resolvePriceTableTemplateInheritance({
    defaults: priceListTemplateDefaults.value,
    groupAssignments: priceListTemplateAssignments(),
    productOverrides: priceListProductOverridesForSnapshot(),
    product,
  })
}

function itemHasResolvedPriceListPricingMethod(item) {
  const resolved = priceListResolvedTemplateForItem(item)
  const mode = String(resolved.pricing_mode || '').trim()
  return Boolean(
    (mode === 'pricing_rule' && Number(resolved.pricing_rule_id || 0) > 0) ||
    (mode === 'tier_template' && Number(resolved.tier_template_id || 0) > 0) ||
    (mode === 'fixed_price' && Number(resolved.fixed_unit_price || 0) > 0)
  )
}

function isInactiveBomWarningText(warning) {
  const text = String(warning || '')
  return text.includes('BOM已失效') || text.includes('BOM失效')
}

function itemHasInactiveBomWarning(item) {
  const warnings = Array.isArray(item?.warnings) ? item.warnings : []
  return item?.bom_status === 'inactive' || warnings.some(isInactiveBomWarningText)
}

function itemBomWarning(item) {
  if (!itemHasInactiveBomWarning(item)) return ''
  return '请在商品档案重新选择可用 BOM。失效 BOM 不能重新启用；如需沿用旧结构，请先在生产 BOM 复制成新 BOM 后再选择。'
}

function itemBomProblemLabel(item = {}) {
  const name = String(
    item.production_bom_name ||
    item.productionBomName ||
    item.bom_name ||
    item.bomName ||
    ''
  ).trim()
  const version = String(
    item.production_bom_version_no ||
    item.productionBomVersionNo ||
    item.source_bom_version_no ||
    item.sourceBomVersionNo ||
    item.bom_version_no ||
    item.bomVersionNo ||
    item.latest_bom_version_no ||
    item.latestBomVersionNo ||
    ''
  ).trim()
  const label = [name, version].filter(Boolean).join(' / ')
  return label || '当前绑定 BOM'
}

function warningTooltip(warning) {
  const text = String(warning || '').trim()
  if (text === '未设置计价方式') {
    return '未设置计价方式。请到 商品与配方 → 商品价格表 → 生成价格表，在价格表、父类、子类或商品行选择按阶梯模板计算、按价格计算模板计算或固定价。'
  }
  return text
}

function itemProductAttributeLines(item) {
  const rows = Array.isArray(item?.product_attributes)
    ? item.product_attributes
    : (Array.isArray(item?.productAttributes) ? item.productAttributes : [])
  return rows
    .map((row) => {
      const label = String(row?.label || row?.key || '').trim()
      const value = String(row?.value || '').trim()
      if (!label || !value) return ''
      return `${label}：${value}`
    })
    .filter(Boolean)
}

function itemProductID(item) {
  return String(item?.sku_id ?? item?.skuID ?? item?.skuId ?? item?.product_id ?? item?.productID ?? item?.productId ?? item?.id ?? item?.name ?? '')
}

function itemSkuID(item = {}, fallback = 0) {
  return Number(item?.sku_id || item?.skuID || item?.skuId || fallback || item?.product_id || item?.productID || item?.productId || item?.id || 0)
}

function itemParentProductID(item = {}, skuID = 0) {
  const explicit = Number(item?.parent_product_id || item?.parentProductID || item?.parentProductId || 0)
  if (explicit > 0) return explicit
  const effective = Number(item?.effective_parent_product_id || item?.effectiveParentProductID || item?.effectiveParentProductId || 0)
  return effective > 0 && effective !== Number(skuID || 0) ? effective : 0
}

function itemSkuSnapshot(item = {}) {
  const snapshot = {}
  ;[
    ['sku_name', item?.sku_name ?? item?.skuName ?? item?.derived_spec_name ?? item?.derivedSpecName ?? item?.derived_sales_unit ?? item?.derivedSalesUnit],
    ['sku_code', item?.sku_code ?? item?.skuCode],
    ['barcode', item?.barcode],
    ['spec_label', item?.spec_label ?? item?.specLabel ?? item?.derived_spec_name ?? item?.derivedSpecName],
    ['net_content_unit', item?.net_content_unit ?? item?.netContentUnit],
  ].forEach(([key, raw]) => {
    const value = String(raw || '').trim()
    if (value) snapshot[key] = value
  })
  const qty = Number(item?.net_content_qty || item?.netContentQty || 0)
  if (Number.isFinite(qty) && qty > 0) snapshot.net_content_qty = qty
  return snapshot
}

function hasPriceListFlatRowError(row = {}) {
  return priceListFlatRowVisibleErrors(row).length > 0
}

async function scrollFirstInactiveBomWarningIntoView() {
  await nextTick()
  const node = document.querySelector('[data-bom-warning-product-id]')
  if (node && typeof node.scrollIntoView === 'function') {
    node.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }
}

function openProductArchiveForBom(item) {
  const productID = Number(item?.product_id || item?.productID || item?.productId || item?.id || 0)
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'productMaster',
      params: productID > 0 ? { open_product_config_id: productID } : {},
      returnNavigation: {
        key: 'costing',
        label: '返回商品价格表',
      },
    },
  }))
}

function metaKeyForListType(listType) {
  if (listType === 'green') return 'green_bean_list'
  if (listType === 'drip') return 'drip_bean_list'
  return listType === 'retail' ? 'retail_bean_list' : 'commercial_bean_list'
}

function tierKeyForListType(listType) {
  if (listType === 'green') return 'green_bean_sale_tiers'
  if (listType === 'drip') return 'drip_wholesale_tiers'
  return listType === 'retail' ? 'retail_bean_tiers' : 'commercial_wholesale_tiers'
}

function priceListTemplateGroupRows(groups = []) {
  return (Array.isArray(groups) ? groups : []).map((category, index) => {
    const firstItem = Array.isArray(category.items) ? (category.items[0] || {}) : {}
    const usesBusinessGroupTemplate = String(category.code || '').startsWith('business-group-') || Number(category.group_id || 0) > 0
    const groupID = Number(category.group_id || 0) || classificationTemplateIDOfItem(firstItem) || activeProductTypeCategoryID.value || 1
    const parentGroupItemID = Number(category.parent_group_item_id || 0) || (usesBusinessGroupTemplate ? 0 : (activeProductTypeCategoryID.value > 0 ? activeProductTypeCategoryID.value : groupID))
    const classificationCategoryID = classificationCategoryIDOfItem(firstItem)
    const groupItemID = Number(category.group_item_id || 0) || (classificationCategoryID > 0 ? classificationCategoryID : syntheticGroupItemID(category.code || category.label || index))
    const depth = Math.max(0, Number(category.depth || 0) || 0)
    return {
      key: String(category.code || groupItemID || index),
      label: category.label || category.category || '未分组',
      group_id: groupID,
      group_name: category.group_name || selectedProductCatalogGroupTemplate.value?.name || selectedProductPriceListLabel.value || '商品价格表分组',
      group_item_id: groupItemID,
      group_item_name: category.group_item_name || category.label || category.category || '未分组',
      parent_group_item_id: parentGroupItemID,
      parent_group_item_name: category.parent_group_item_name || selectedProductCatalogGroupTemplate.value?.name || selectedProductPriceListLabel.value || '父类',
      depth,
      unclassified: Boolean(category.unclassified) || String(category.code || '').includes('unclassified'),
      items: Array.isArray(category.items) ? category.items : [],
    }
  })
}

function syntheticGroupItemID(value) {
  const text = String(value || 'group')
  let hash = 0
  for (let i = 0; i < text.length; i += 1) hash = ((hash * 31) + text.charCodeAt(i)) % 900000
  return 100000 + hash
}

function priceListGroupForItem(item = {}) {
  const code = categoryCodeOfItem(item, pdfTheme.value.listType)
  const groupItemID = productCatalogGroupItemIDOfItem(item)
  return priceListGroupTemplateRows.value.find((row) => (
    row.key === String(code) ||
    (groupItemID > 0 && Number(row.group_item_id || 0) === groupItemID)
  )) || priceListGroupTemplateRows.value[0] || {
    key: 'default',
    group_id: Number(selectedProductCatalogGroupTemplate.value?.id || 0) || activeProductTypeCategoryID.value || 1,
    group_name: selectedProductCatalogGroupTemplate.value?.name || selectedProductPriceListLabel.value || '商品价格表分组',
    group_item_id: 1,
    group_item_name: '未分组',
    parent_group_item_id: Number(selectedProductCatalogGroupTemplate.value?.id || 0) || activeProductTypeCategoryID.value || 1,
    parent_group_item_name: selectedProductCatalogGroupTemplate.value?.name || selectedProductPriceListLabel.value || '父类',
  }
}

function priceListGroupConfigRow(group = {}) {
  const key = String(group.key || group.code || group.group_item_id || '')
  const groupItemID = Number(group.group_item_id || 0)
  return priceListGroupTemplateRows.value.find((row) => (
    String(row.key || '') === key ||
    (groupItemID > 0 && Number(row.group_item_id || 0) === groupItemID)
  )) || {
    key: key || 'default',
    group_id: Number(group.group_id || activeProductTypeCategoryID.value || 1),
    group_name: group.group_name || selectedProductPriceListLabel.value || '商品价格表分组',
    group_item_id: groupItemID || syntheticGroupItemID(group.code || group.label || 'default'),
    group_item_name: group.group_item_name || group.label || group.category || '未分组',
    parent_group_item_id: Number(group.parent_group_item_id || activeProductTypeCategoryID.value || 1),
    parent_group_item_name: group.parent_group_item_name || selectedProductPriceListLabel.value || '父类',
    depth: Math.max(0, Number(group.depth || 0) || 0),
    unclassified: Boolean(group.unclassified) || String(group.code || key || '').includes('unclassified'),
    items: Array.isArray(group.items) ? group.items : [],
  }
}

function productPickerCategoryDepth(category = {}) {
  const rowDepth = Number(category.depth ?? priceListGroupConfigRow(category).depth ?? 0)
  return Math.max(0, Number.isFinite(rowDepth) ? rowDepth : 0)
}

function productPickerCategoryCollapseKey(category = {}) {
  const row = priceListGroupConfigRow(category)
  return String(category.code || category.key || row.key || row.group_item_id || category.label || '')
}

function productPickerCategoryStyle(category = {}) {
  const indent = Math.min(productPickerCategoryDepth(category) * 24, 96)
  return { '--product-picker-category-indent': `${indent}px` }
}

function productPickerRowStyle(category = {}) {
  const indent = Math.min((productPickerCategoryDepth(category) + 1) * 24, 120)
  return { '--product-picker-row-indent': `${indent}px` }
}

function isProductPickerCategoryCollapsed(category = {}) {
  const key = productPickerCategoryCollapseKey(category)
  return Boolean(key && productPickerCollapsedCategories.value[key])
}

function isProductPickerCategoryHiddenByCollapsedAncestor(category = {}) {
  return priceListCategoryHiddenByCollapsedAncestor(categoryProductGroups.value, category, productPickerCollapsedCategories.value)
}

function toggleProductPickerCategoryCollapse(category = {}) {
  const key = productPickerCategoryCollapseKey(category)
  if (!key) return
  productPickerCollapsedCategories.value = {
    ...productPickerCollapsedCategories.value,
    [key]: !productPickerCollapsedCategories.value[key],
  }
}

function priceListProductRowForItem(item = {}) {
  const group = priceListGroupForItem(item)
  const skuID = itemSkuID(item, Number(itemProductID(item) || 0))
  return {
    product_id: skuID,
    sku_id: skuID,
    parent_product_id: itemParentProductID(item, skuID) || priceListParentProductID(item),
    product_key: `sku:${skuID}`,
    product_name: beanName(item, metaKeyForItem(item)) || item?.name || '',
    parent_product_name: String(item?.__price_list_parent_product_name || item?.parent_product_name || '').trim(),
    sku_name: priceListProductSpecLabel(item),
    group_item_id: group.group_item_id,
    parent_group_item_id: group.parent_group_item_id,
    price_unit: item?.price_unit ?? item?.priceUnit ?? item?.price_unit_snapshot ?? item?.priceUnitSnapshot ?? '',
    default_sales_unit: item?.default_sales_unit ?? item?.defaultSalesUnit ?? '',
    derived_sales_unit: item?.derived_sales_unit ?? item?.derivedSalesUnit ?? '',
    sales_unit: item?.sales_unit ?? item?.salesUnit ?? '',
    quote_unit: item?.quote_unit ?? item?.quoteUnit ?? '',
    order_unit: item?.order_unit ?? item?.orderUnit ?? '',
    spec_label: item?.spec_label ?? item?.specLabel ?? '',
  }
}

function priceListProductRowForSpec(family = {}, spec = {}) {
  return {
    ...priceListProductRowForItem(spec),
    scope: 'sku',
    parent_product_id: priceListParentProductID(family),
    parent_product_name: String(family?.name || family?.parent_product_name || '').trim(),
    product_name: String(family?.name || family?.parent_product_name || spec?.name || '').trim(),
    sku_name: priceListProductSpecLabel(spec),
  }
}

function priceListParentProductPricingRow(family = {}) {
  const parentProductID = priceListParentProductID(family)
  const name = String(family?.name || family?.parent_product_name || '').trim()
  return {
    scope: 'parent_product',
    product_id: parentProductID,
    sku_id: 0,
    parent_product_id: parentProductID,
    product_key: `parent:${parentProductID}`,
    product_name: name,
    parent_product_name: name,
    sku_name: '各销售规格',
    sku_options: Array.isArray(family?.sku_options) ? family.sku_options : [],
  }
}

function priceListTemplateResolutionForItem(item = {}) {
  const sourceProductID = Number(item?.product_id || item?.productId || item?.productID || item?.id || 0)
  const skuID = itemSkuID(item, sourceProductID)
  const productID = skuID || sourceProductID || Number(itemProductID(item) || 0)
  const groupRow = priceListGroupForItem(item)
  return resolvePriceTableTemplateInheritance({
    defaults: priceListTemplateDefaults.value,
    groupAssignments: priceListTemplateAssignments(),
    productOverrides: priceListProductOverridesForSnapshot(),
    product: {
      id: productID,
      product_id: productID,
      sku_id: skuID,
      parent_product_id: itemParentProductID(item, skuID) || priceListParentProductID(item),
      group_item_id: groupRow.group_item_id,
      parent_group_item_id: groupRow.parent_group_item_id,
    },
  })
}

function priceListEffectiveProductPricingSelection(row = {}) {
  const representative = selectedSpecsForProduct(row)[0]
  return priceListTemplateResolutionForItem(representative || row)
}

function priceListTierTemplateCompatibilityForItem(item = {}, template = null) {
  if (!template) {
    return {
      compatible: false,
      product_unit: '',
      template_units: [],
      message: '阶梯模板不可用：模板不存在、已停用或没有有效档位',
    }
  }
  return priceTierTemplateUnitCompatibility(item, template)
}

function priceListProductTierTemplateWarning(item = {}) {
  if (!isPdfProductSelected(itemProductID(item))) return ''
  const resolved = priceListTemplateResolutionForItem(item)
  if (String(resolved.pricing_mode || '').trim() !== 'tier_template') return ''
  const template = priceTierTemplateByID(resolved.tier_template_id)
  if (!template) return ''
  const compatibility = priceListTierTemplateCompatibilityForItem(item, template)
  return compatibility.compatible ? '' : compatibility.message
}

function priceListParentTemplateKey(group = {}) {
  const row = priceListGroupConfigRow(group)
  const parentGroupItemID = Number(row.parent_group_item_id || 0)
  const groupItemID = Number(row.group_item_id || 0)
  if (parentGroupItemID > 0) return String(parentGroupItemID)
  if (groupItemID > 0) return String(groupItemID)
  return String(row.key || '')
}

function priceListGroupTemplateKey(group = {}) {
  const row = priceListGroupConfigRow(group)
  return String(row.key || row.group_item_id || '')
}

function priceListCategoryTemplateTarget(group = {}) {
  const row = priceListGroupConfigRow(group)
  const parentGroupItemID = Number(row.parent_group_item_id || 0)
  const groupItemID = Number(row.group_item_id || 0)
  const depth = Math.max(0, Number(row.depth ?? group.depth ?? 0) || 0)
  const isUnclassified = Boolean(row.unclassified) || String(row.key || '').includes('unclassified')
  const isParentCategory = !isUnclassified && parentGroupItemID <= 0 && groupItemID > 0 && depth <= 0
  return {
    kind: isParentCategory ? 'parent' : 'group',
    key: isParentCategory ? priceListParentTemplateKey(row) : priceListGroupTemplateKey(row),
    row,
  }
}

function priceListParentTemplateSelection(group = {}) {
  const key = priceListParentTemplateKey(group)
  return priceListParentTemplateSelections.value[key] || defaultPriceListTemplateSelection()
}

function priceListGroupTemplateSelection(group = {}) {
  const key = priceListGroupTemplateKey(group)
  return priceListGroupTemplateSelections.value[key] || defaultPriceListTemplateSelection()
}

function priceListCategoryTemplateSelection(group = {}) {
  const target = priceListCategoryTemplateTarget(group)
  if (priceListCategoryTemplateTarget(group).kind === 'parent') {
    return priceListParentTemplateSelections.value[target.key] || defaultPriceListTemplateSelection()
  }
  return priceListGroupTemplateSelections.value[target.key] || defaultPriceListTemplateSelection()
}

function priceListProductTemplateOverrideScope(row = {}) {
  return String(row?.scope || row?.override_scope || row?.overrideScope || '').trim() === 'parent_product'
    ? 'parent_product'
    : 'sku'
}

function priceListProductTemplateOverrideKey(row = {}) {
  const scope = priceListProductTemplateOverrideScope(row)
  if (scope === 'parent_product') {
    const parentProductID = Number(row?.parent_product_id || row?.product_id || 0)
    return parentProductID > 0 ? `parent:${parentProductID}` : ''
  }
  const skuID = Number(row?.sku_id || row?.product_id || 0)
  return skuID > 0 ? `sku:${skuID}` : ''
}

function priceListProductTemplateOverride(row = {}) {
  const key = priceListProductTemplateOverrideKey(row)
  if (key && priceListProductTemplateOverrides.value[key]) return priceListProductTemplateOverrides.value[key]
  if (priceListProductTemplateOverrideScope(row) === 'sku') {
    const legacyKey = String(row?.product_id || row?.sku_id || '')
    if (legacyKey && priceListProductTemplateOverrides.value[legacyKey]) return priceListProductTemplateOverrides.value[legacyKey]
  }
  return defaultPriceListTemplateSelection()
}

function priceListProductFixedPrice(family = {}, spec = {}) {
  return Number(priceListProductTemplateOverride(priceListProductRowForSpec(family, spec)).fixed_unit_price || 0) || 0
}

function setPriceListProductFixedPrice(family = {}, spec = {}, value) {
  const row = priceListProductRowForSpec(family, spec)
  const key = priceListProductTemplateOverrideKey(row)
  if (!key) return
  const fixedUnitPrice = Number(value || 0) || 0
  const next = { ...priceListProductTemplateOverrides.value }
  if (fixedUnitPrice > 0) {
    next[key] = {
      ...defaultPriceListTemplateSelection({ fixed_unit_price: fixedUnitPrice }),
      scope: 'sku',
      product_id: Number(row.sku_id || row.product_id || 0),
      sku_id: Number(row.sku_id || row.product_id || 0),
      parent_product_id: Number(row.parent_product_id || 0),
      product_key: String(row.product_key || ''),
      product_name: row.product_name || '',
      parent_product_name: row.parent_product_name || row.product_name || '',
      sku_name: row.sku_name || '',
    }
  } else {
    delete next[key]
  }
  priceListProductTemplateOverrides.value = next
  savePriceListGenerationDraftForActiveType()
}

function defaultPriceListConfigDialog() {
  return {
    open: false,
    type: '',
    productId: '',
  }
}

function defaultPriceListPricingPopover() {
  return {
    open: false,
    type: '',
    key: '',
    group: null,
    productRow: null,
  }
}

function priceListProductDialogKey(row = {}) {
  return priceListProductTemplateOverrideKey(row) || String(row.product_key || row.product_id || '')
}

const priceListConfigDialogTitle = computed(() => {
  if (priceListConfigDialog.value.type === 'product-display') return '商品展示'
  return '展示配置'
})

function closePriceListConfigDialog() {
  priceListConfigDialog.value = defaultPriceListConfigDialog()
}

function openPriceListProductDisplayDialog(productId) {
  closePriceListPricingPopover()
  priceListConfigDialog.value = {
    ...defaultPriceListConfigDialog(),
    open: true,
    type: 'product-display',
    productId: String(productId || ''),
  }
}

function priceListPricingPopoverKey(type, payload = {}) {
  if (type === 'category') {
    const target = priceListCategoryTemplateTarget(payload)
    return `${type}:${target.kind}:${target.key}`
  }
  return `${type}:${priceListProductDialogKey(payload)}`
}

function openPriceListPricingPopover(type, payload = {}) {
  closePriceListConfigDialog()
  if (type === 'category') {
    const group = priceListGroupConfigRow(payload)
    priceListPricingPopover.value = {
      ...defaultPriceListPricingPopover(),
      open: true,
      type,
      key: priceListPricingPopoverKey(type, group),
      group,
    }
    return
  }
  const productRow = { ...payload }
  priceListPricingPopover.value = {
    ...defaultPriceListPricingPopover(),
    open: true,
    type: 'product',
    key: priceListPricingPopoverKey('product', productRow),
    productRow,
  }
}

function closePriceListPricingPopover() {
  priceListPricingPopover.value = defaultPriceListPricingPopover()
}

function isPriceListPricingPopoverOpen(type, payload = {}) {
  return priceListPricingPopover.value.open &&
    priceListPricingPopover.value.type === type &&
    priceListPricingPopover.value.key === priceListPricingPopoverKey(type, payload)
}

function isPriceListProductDisplayDialogOpen(productId) {
  return priceListConfigDialog.value.open &&
    priceListConfigDialog.value.type === 'product-display' &&
    String(priceListConfigDialog.value.productId || '') === String(productId || '')
}

function priceListCategoryPricingHasOverride(group = {}) {
  return priceListTemplateHasOverride(priceListCategoryTemplateSelection(group))
}

function priceListProductPricingHasOverride(row = {}) {
  return priceListTemplateHasOverride(priceListProductTemplateOverride(row))
}

function priceListTemplateSummary(selection = {}, fallback = '继承') {
  if (!priceListTemplateHasOverride(selection)) return fallback
  const mode = String(selection.pricing_mode || '').trim()
  if (mode === 'tier_template') {
    const templateID = Number(selection.tier_template_id || 0)
    const template = priceTierTemplates.value.find((row) => Number(row.id || 0) === templateID)
    return template ? priceTierTemplateLabel(template) : priceTablePricingModeLabel(mode)
  }
  if (mode === 'pricing_rule') {
    const ruleID = Number(selection.pricing_rule_id || 0)
    const rule = pricingRules.value.find((row) => Number(row.id || 0) === ruleID)
    return rule ? pricingRuleLabel(rule) : priceTablePricingModeLabel(mode)
  }
  if (mode === 'fixed_price') {
    const price = Number(selection.fixed_unit_price || 0)
    return price > 0 ? `固定价 ${price}` : priceTablePricingModeLabel(mode)
  }
  return priceTablePricingModeLabel(mode)
}

function priceListCategoryPricingSummary(group = {}) {
  const summary = priceListTemplateSummary(priceListCategoryTemplateSelection(group), '')
  if (summary) return summary
  return '继承分类'
}

function priceListProductPricingSummary(row = {}) {
  return priceListTemplateSummary(priceListProductTemplateOverride(row), '继承分类')
}

function priceListBadgeLabel(value) {
  switch (String(value || '').trim()) {
  case 'new':
    return 'NEW 上新'
  case 'thumb':
  case 'medal':
    return '推荐'
  default:
    return ''
  }
}

function priceListProductDisplayHasOverride(id) {
  return Boolean(String(customizerField(id, 'badge') || '').trim() || String(customizerField(id, 'highlightTerms') || '').trim())
}

function priceListProductDisplaySummary(id) {
  const parts = []
  const badge = priceListBadgeLabel(customizerField(id, 'badge'))
  const highlight = String(customizerField(id, 'highlightTerms') || '').trim()
  if (badge) parts.push(badge)
  if (highlight) parts.push('标红词')
  return parts.length ? parts.join(' / ') : '无标签'
}

function setPriceListDefaultTemplate(field, value) {
  priceListTemplateDefaults.value = {
    ...priceListTemplateDefaults.value,
    [field]: priceListTemplateFieldValue(field, value),
  }
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function setPriceListParentTemplate(group = {}, field, value) {
  const key = priceListParentTemplateKey(group)
  if (field === 'pricing_mode' && !String(value || '').trim()) {
    const next = { ...priceListParentTemplateSelections.value }
    delete next[key]
    priceListParentTemplateSelections.value = next
    savePriceListGenerationDraftForActiveType()
    schedulePriceListPricingRuleTrialRefresh()
    return
  }
  priceListParentTemplateSelections.value = {
    ...priceListParentTemplateSelections.value,
    [key]: {
      ...priceListParentTemplateSelection(group),
      [field]: priceListTemplateFieldValue(field, value),
    },
  }
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function setPriceListGroupTemplate(group = {}, field, value) {
  const key = priceListGroupTemplateKey(group)
  if (field === 'pricing_mode' && !String(value || '').trim()) {
    const next = { ...priceListGroupTemplateSelections.value }
    delete next[key]
    priceListGroupTemplateSelections.value = next
    savePriceListGenerationDraftForActiveType()
    schedulePriceListPricingRuleTrialRefresh()
    return
  }
  priceListGroupTemplateSelections.value = {
    ...priceListGroupTemplateSelections.value,
    [key]: {
      ...priceListGroupTemplateSelection(group),
      [field]: priceListTemplateFieldValue(field, value),
    },
  }
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function clearPriceListCategoryTemplate(group = {}) {
  const target = priceListCategoryTemplateTarget(group)
  if (target.kind === 'parent') {
    const next = { ...priceListParentTemplateSelections.value }
    delete next[target.key]
    priceListParentTemplateSelections.value = next
    savePriceListGenerationDraftForActiveType()
    schedulePriceListPricingRuleTrialRefresh()
    return
  }
  const next = { ...priceListGroupTemplateSelections.value }
  delete next[target.key]
  priceListGroupTemplateSelections.value = next
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function setPriceListCategoryTemplate(group = {}, field, value) {
  const target = priceListCategoryTemplateTarget(group)
  if (field === 'pricing_mode' && !String(value || '').trim()) {
    clearPriceListCategoryTemplate(group)
    return
  }
  const selection = field === 'pricing_mode'
    ? defaultPriceListTemplateSelection({ pricing_mode: value })
    : {
      ...priceListCategoryTemplateSelection(group),
      [field]: priceListTemplateFieldValue(field, value),
    }
  if (target.kind === 'parent') {
    priceListParentTemplateSelections.value = {
      ...priceListParentTemplateSelections.value,
      [target.key]: selection,
    }
    savePriceListGenerationDraftForActiveType()
    schedulePriceListPricingRuleTrialRefresh()
    return
  }
  priceListGroupTemplateSelections.value = {
    ...priceListGroupTemplateSelections.value,
    [target.key]: selection,
  }
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function setPriceListProductTemplate(row = {}, field, value) {
  if (priceListProductTemplateOverrideScope(row) !== 'parent_product') return
  const key = priceListProductTemplateOverrideKey(row)
  if (!key) return
  if (field === 'pricing_mode' && !String(value || '').trim()) {
    clearPriceListProductTemplate(row)
    return
  }
  const selection = field === 'pricing_mode'
    ? defaultPriceListTemplateSelection({ pricing_mode: value })
    : {
      ...priceListProductTemplateOverride(row),
      [field]: priceListTemplateFieldValue(field, value),
    }
  priceListProductTemplateOverrides.value = {
    ...priceListProductTemplateOverrides.value,
    [key]: {
      ...selection,
      scope: 'parent_product',
      product_id: Number(row.parent_product_id || row.product_id || 0),
      sku_id: 0,
      parent_product_id: Number(row.parent_product_id || row.product_id || 0),
      product_key: String(row.product_key || ''),
      product_name: row.product_name || '',
      parent_product_name: row.parent_product_name || row.product_name || '',
      sku_name: row.sku_name || '',
    },
  }
  clearPriceListLegacyPricingConflict(row.parent_product_id || row.product_id)
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function clearPriceListLegacyPricingConflict(parentProductID) {
  const target = Number(parentProductID || 0)
  if (!(target > 0)) return
  priceListLegacyPricingConflicts.value = priceListLegacyPricingConflicts.value.filter((conflict) => (
    Number(conflict.parent_product_id || 0) !== target
  ))
}

function clearPriceListProductTemplate(row = {}) {
  const key = priceListProductTemplateOverrideKey(row)
  if (!key) return
  const next = { ...priceListProductTemplateOverrides.value }
  delete next[key]
  if (priceListProductTemplateOverrideScope(row) === 'sku') {
    delete next[String(row.product_id || row.sku_id || '')]
  }
  priceListProductTemplateOverrides.value = next
  if (priceListProductTemplateOverrideScope(row) === 'parent_product') {
    clearPriceListLegacyPricingConflict(row.parent_product_id || row.product_id)
  }
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function priceListActivePricingSelection() {
  if (priceListPricingPopover.value.type === 'category') {
    return priceListCategoryTemplateSelection(priceListPricingPopover.value.group || {})
  }
  if (priceListPricingPopover.value.type === 'product') {
    return priceListEffectiveProductPricingSelection(priceListPricingPopover.value.productRow || {})
  }
  return defaultPriceListTemplateSelection()
}

function setPriceListPricingPopoverMode(value) {
  if (priceListPricingPopover.value.type === 'category') {
    setPriceListCategoryTemplate(priceListPricingPopover.value.group || {}, 'pricing_mode', value)
    return
  }
  if (priceListPricingPopover.value.type === 'product') {
    setPriceListProductTemplate(priceListPricingPopover.value.productRow || {}, 'pricing_mode', value)
  }
}

function setPriceListPricingPopoverField(field, value) {
  if (priceListPricingPopover.value.type === 'category') {
    setPriceListCategoryTemplate(priceListPricingPopover.value.group || {}, field, value)
    return
  }
  if (priceListPricingPopover.value.type === 'product') {
    setPriceListProductTemplate(priceListPricingPopover.value.productRow || {}, field, value)
  }
}

function defaultPriceListTemplateSelection(overrides = {}) {
  return {
    pricing_mode: String(overrides.pricing_mode ?? overrides.pricingMode ?? '').trim(),
    tier_template_id: Number(overrides.tier_template_id ?? overrides.tierTemplateID ?? 0) || 0,
    pricing_rule_id: Number(overrides.pricing_rule_id ?? overrides.pricingRuleID ?? 0) || 0,
    fixed_unit_price: Number(overrides.fixed_unit_price ?? overrides.fixedUnitPrice ?? 0) || 0,
  }
}

function priceListTemplateFieldValue(field, value) {
  if (field === 'pricing_mode') return String(value || '').trim()
  return Number(value || 0) || 0
}

function priceListTemplateHasOverride(selection = {}) {
  return Boolean(
    String(selection.pricing_mode || '').trim() ||
    Number(selection.tier_template_id || 0) > 0 ||
    Number(selection.pricing_rule_id || 0) > 0 ||
    Number(selection.fixed_unit_price || 0) > 0
  )
}

function priceListTemplateAssignments() {
  const rows = []
  const seenParents = new Set()
  priceListGroupTemplateRows.value.forEach((group) => {
    const parentKey = priceListParentTemplateKey(group)
    const parentSelection = priceListParentTemplateSelection(group)
    if (parentKey && !seenParents.has(parentKey) && priceListTemplateHasOverride(parentSelection)) {
      seenParents.add(parentKey)
      const parentGroupItemID = Number(group.parent_group_item_id || 0) || Number(group.group_item_id || 0)
      rows.push({
        group_id: group.group_id,
        group_name: group.group_name,
        group_item_id: Number(group.group_item_id || 0) > 0 && Number(group.parent_group_item_id || 0) <= 0 ? Number(group.group_item_id || 0) : parentGroupItemID,
        group_item_name: Number(group.parent_group_item_id || 0) > 0 ? group.parent_group_item_name : group.group_item_name,
        parent_group_item_id: 0,
        parent_group_item_name: '',
        level: 1,
        ...parentSelection,
      })
    }
    const groupKey = priceListGroupTemplateKey(group)
    const selection = priceListGroupTemplateSelection(group)
    rows.push({
      group_id: group.group_id,
      group_name: group.group_name,
      group_item_id: Number(group.group_item_id || 0),
      group_item_name: group.group_item_name,
      parent_group_item_id: group.parent_group_item_id,
      parent_group_item_name: group.parent_group_item_name,
      group_key: groupKey,
      level: 2,
      ...selection,
    })
  })
  return rows
}

function priceListProductOverridesForSnapshot() {
  const selectedSkuIDs = new Set(pdfProductSpecSelections.value.map((row) => Number(row.sku_id || 0)).filter((id) => id > 0))
  const selectedParentIDs = new Set(pdfProductSpecSelections.value.map((row) => Number(row.parent_product_id || 0)).filter((id) => id > 0))
  return Object.values(priceListProductTemplateOverrides.value || {})
    .filter((row) => priceListProductTemplateOverrideScope(row) === 'parent_product'
      ? selectedParentIDs.has(Number(row.parent_product_id || row.product_id || 0))
      : selectedSkuIDs.has(Number(row.sku_id || row.product_id || 0)))
    .filter((row) => (Number(row?.product_id || 0) > 0 || String(row?.product_key || '').trim()) && priceListTemplateHasOverride(row))
}

function priceListFlatRowsFromGroups(groups = []) {
  const tierKey = tierKeyForListType(pdfTheme.value.listType)
  const rows = []
  ;(Array.isArray(groups) ? groups : []).forEach((group) => {
    ;(Array.isArray(group?.items) ? group.items : []).forEach((item) => {
      const sourceProductID = Number(item?.product_id || item?.productId || item?.productID || item?.id || 0)
      const skuID = itemSkuID(item, sourceProductID)
      const productID = skuID || sourceProductID || Number(itemProductID(item) || 0)
      const groupRow = priceListGroupConfigRow(group)
      const product = {
        id: productID,
        product_id: productID,
        sku_id: skuID,
        parent_product_id: itemParentProductID(item, skuID) || priceListParentProductID(item),
        group_item_id: groupRow.group_item_id,
        parent_group_item_id: groupRow.parent_group_item_id,
      }
      const resolved = resolvePriceTableTemplateInheritance({
        defaults: priceListTemplateDefaults.value,
        groupAssignments: priceListTemplateAssignments(),
        productOverrides: priceListProductOverridesForSnapshot(),
        product,
      })
      const tiers = Array.isArray(item?.[tierKey]) ? item[tierKey] : []
      const mode = String(resolved.pricing_mode || 'tier_template').trim()
      if (mode === 'tier_template') {
        const template = priceTierTemplateByID(resolved.tier_template_id)
        const templateCompatibility = priceListTierTemplateCompatibilityForItem(item, template)
        if (!template || !templateCompatibility.compatible) {
          const templateTiers = (Array.isArray(template?.tiers) ? template.tiers : []).filter((tier) => tier?.active !== false)
          const incompatibleTier = templateTiers.find((tier) => (
            !priceListTierTemplateCompatibilityForItem(item, { tiers: [tier] }).compatible
          )) || templateTiers[0] || {}
          const incompatibleTierIndex = Math.max(0, templateTiers.indexOf(incompatibleTier))
          const sourceTier = tierForTemplateTier(incompatibleTier, tiers, incompatibleTierIndex)
          const pricingRule = pricingRuleByID(incompatibleTier.pricing_rule_id)
          rows.push(priceListFlatRowFromSource({
            item,
            groupRow,
            productID,
            sourceTier,
            rowKey: priceTierTemplateRowKey({
              productID: skuID || productID || itemProductID(item),
              templateID: resolved.tier_template_id,
              tierID: incompatibleTier.id || incompatibleTier.label || incompatibleTierIndex,
              product: item,
              tier: incompatibleTier,
              suffix: 'unit-incompatible',
            }),
            tierLabel: '模板不可用',
            minQty: incompatibleTier.min_qty ?? incompatibleTier.minQty ?? 0,
            maxQty: incompatibleTier.max_qty ?? incompatibleTier.maxQty ?? null,
            resolved,
            pricingRule,
            tierTemplateID: resolved.tier_template_id,
            tierTemplateName: template?.name || '已失效阶梯模板',
            tierTemplateSource: resolved.tier_template_source,
            templateTierID: Number(incompatibleTier.id || 0),
            tierQuantityUnit: incompatibleTier.quantity_unit ?? incompatibleTier.quantityUnit ?? '',
            tierUnitCompatibility: templateCompatibility,
            pricingRuleID: Number(incompatibleTier.pricing_rule_id || 0),
            pricingRuleSource: resolved.tier_template_source,
            tierPricingRuleID: Number(incompatibleTier.pricing_rule_id || 0),
          }))
          return
        }
        ;(Array.isArray(template?.tiers) ? template.tiers : []).forEach((templateTier, tierIndex) => {
          const sourceTier = tierForTemplateTier(templateTier, tiers, tierIndex)
          const pricingRule = pricingRuleByID(templateTier.pricing_rule_id)
          rows.push(priceListFlatRowFromSource({
            item,
            groupRow,
            productID,
            sourceTier,
            rowKey: priceTierTemplateRowKey({
              productID: skuID || productID || itemProductID(item),
              templateID: resolved.tier_template_id,
              tierID: templateTier.id || templateTier.label || tierIndex,
              product: item,
              tier: templateTier,
            }),
            tierLabel: priceListSalesSpecCountTierLabel(templateTier) || templateTier.label || sourceTier?.label || '',
            minQty: templateTier.min_qty ?? templateTier.minQty ?? sourceTier?.min_qty ?? sourceTier?.minQty ?? 0,
            maxQty: templateTier.max_qty ?? templateTier.maxQty ?? sourceTier?.max_qty ?? sourceTier?.maxQty ?? null,
            resolved,
            pricingRule,
            tierTemplateID: resolved.tier_template_id,
            tierTemplateName: template?.name || '',
            tierTemplateSource: resolved.tier_template_source,
            templateTierID: Number(templateTier.id || 0),
            tierQuantityUnit: productCurrentSalesSpecUnit(item) || priceListProductSpecLabel(item),
            tierUnitCompatibility: templateCompatibility,
            pricingRuleID: Number(templateTier.pricing_rule_id || 0),
            pricingRuleSource: resolved.tier_template_source,
            tierPricingRuleID: Number(templateTier.pricing_rule_id || 0),
          }))
        })
      } else {
        const sourceTier = firstPriceSourceTier(tiers)
        const pricingRule = mode === 'pricing_rule' ? pricingRuleByID(resolved.pricing_rule_id) : null
        rows.push(priceListFlatRowFromSource({
          item,
          groupRow,
          productID,
          sourceTier,
          rowKey: `${skuID || productID || itemProductID(item)}:${mode}`,
          tierLabel: mode === 'fixed_price' ? '固定价' : '基础价',
          minQty: 0,
          maxQty: null,
          resolved,
          pricingRule,
          tierTemplateID: 0,
          tierTemplateSource: '',
          templateTierID: 0,
          pricingRuleID: mode === 'pricing_rule' ? resolved.pricing_rule_id : 0,
          pricingRuleSource: mode === 'pricing_rule' ? resolved.pricing_rule_source : '',
          tierPricingRuleID: 0,
          fixedUnitPrice: mode === 'fixed_price' ? resolved.fixed_unit_price : 0,
        }))
      }
    })
  })
  return rows
}

function priceListFlatRowFromSource({
  item = {},
  groupRow = {},
  productID = 0,
  sourceTier = {},
  rowKey = '',
  tierLabel = '',
  minQty = 0,
  maxQty = null,
  resolved = {},
  pricingRule = null,
  tierTemplateID = 0,
  tierTemplateName = '',
  tierTemplateSource = '',
  templateTierID = 0,
  tierQuantityUnit = '',
  tierUnitCompatibility = null,
  pricingRuleID = 0,
  pricingRuleSource = '',
  tierPricingRuleID = 0,
  fixedUnitPrice = 0,
} = {}) {
  const mode = String(resolved.pricing_mode || '').trim()
  const tierUnitIncompatible = tierUnitCompatibility?.compatible === false
  const originalPrice = tierUnitIncompatible ? 0 : (mode === 'fixed_price' ? Number(fixedUnitPrice || 0) : tierFlatFinalPrice(sourceTier))
  const override = Number(priceListFlatRowOverrides.value[rowKey])
  const hasOverride = !tierUnitIncompatible && Number.isFinite(override) && override > 0
  const finalPrice = hasOverride ? override : originalPrice
  const priceUnit = flatRowPriceUnit(sourceTier, item)
  const inventoryUnit = String(item?.inventory_unit || item?.inventoryUnit || sourceTier?.inventory_unit || 'kg').trim() || 'kg'
  const ruleVersion = pricingRuleVersion(pricingRule)
  const skuID = itemSkuID(item, productID)
  const parentProductID = itemParentProductID(item, skuID)
  const skuSnapshot = itemSkuSnapshot(item)
  const row = {
    row_key: rowKey,
    product_id: productID,
    sku_id: skuID,
    parent_product_id: parentProductID,
    sku_snapshot: skuSnapshot,
    sku_name: skuSnapshot.sku_name || '',
    sku_code: skuSnapshot.sku_code || '',
    barcode: skuSnapshot.barcode || '',
    spec_label: skuSnapshot.spec_label || '',
    net_content_qty: Number(skuSnapshot.net_content_qty || 0) || 0,
    net_content_unit: skuSnapshot.net_content_unit || '',
    product_key: itemProductID(item),
    product_name: item.product_name_snapshot || item.product_name || item.__price_list_product_name || item.name || item.display_name_snapshot || '',
    group_snapshot: priceListGroupSnapshot(groupRow),
    group_source: 'product_catalog',
    pricing_mode: mode,
    pricing_mode_source: resolved.pricing_mode_source || 'default',
    tier_label: tierLabel,
    min_qty: Number(minQty || 0) || 0,
    max_qty: maxQty === undefined || maxQty === null || maxQty === '' ? null : Number(maxQty),
    quantity_basis: 'sales_spec_count',
    price_unit: priceUnit,
    final_unit_price: finalPrice,
    original_final_unit_price: originalPrice,
    currency: sourceTier?.currency || 'CNY',
    inventory_unit: inventoryUnit,
    inventory_conversion_json: flatRowInventoryConversion(sourceTier, priceUnit, inventoryUnit, item),
    source_price_record_id: Number(sourceTier?.source_price_record_id || sourceTier?.sourcePriceRecordID || 0),
    tier_template_id: Number(tierTemplateID || 0),
    ...(Number(tierTemplateID || 0) > 0 ? {
      tier_template_name: String(tierTemplateName || '').trim(),
      tier_quantity_unit: String(tierQuantityUnit || priceUnit || '').trim(),
      product_sales_spec_unit: String(tierUnitCompatibility?.product_unit || priceUnit || '').trim(),
      tier_unit_compatible: !tierUnitIncompatible,
      tier_unit_compatibility_error: String(tierUnitCompatibility?.message || '').trim(),
    } : {}),
    tier_template_source: tierTemplateSource,
    template_tier_id: Number(templateTierID || 0),
    pricing_rule_id: Number(pricingRuleID || 0),
    pricing_rule_source: pricingRuleSource,
	    pricing_rule_version: ruleVersion,
	    tier_pricing_rule_id: Number(tierPricingRuleID || 0),
	    tier_pricing_rule_version: tierPricingRuleID ? ruleVersion : '',
	    customer_product_alias_id: Number(item.customer_product_alias_id || item.customerProductAliasID || 0),
	    fixed_unit_price: Number(fixedUnitPrice || 0) || 0,
	    cost_source_snapshot: costSourceSnapshotForPriceRow(item, sourceTier, pricingRule, mode),
	    customer_reference_snapshot: customerReferenceSnapshotForPriceRow(item),
    manual_adjusted: hasOverride && Math.abs(override - originalPrice) > 0.005,
  }
  const trial = mode === 'pricing_rule' || mode === 'tier_template' ? priceListPricingRuleTrialResultForRow(row) : null
  return trial ? applyPricingRuleTrialToPriceTableRow(row, trial) : row
}

function priceListPricingRuleTrialResultForRow(row = {}) {
  const cached = priceListPricingRuleTrialCacheEntryForRow(row)
  return cached?.status === 'success' ? cached.result : null
}

function priceListPricingRuleTrialCacheEntryForRow(row = {}) {
  const payload = priceTablePricingRuleTrialPayload(row, { customerID: activeBeanListCustomerID.value })
  const key = priceTablePricingRuleTrialCacheKey(payload)
  if (!key) return null
  return priceListPricingRuleTrialCache.value[key] || null
}

function priceListFlatRowPricingTrialStatus(row = {}) {
  return String(priceListPricingRuleTrialCacheEntryForRow(row)?.status || '').trim()
}

function priceListFlatRowPricingTrialError(row = {}) {
  return String(priceListPricingRuleTrialCacheEntryForRow(row)?.error || '').trim()
}

function priceListFlatRowVisibleErrors(row = {}) {
  return priceListFlatRowErrors(row, {
    trialStatus: priceListFlatRowPricingTrialStatus(row),
    trialError: priceListFlatRowPricingTrialError(row),
  })
}

function priceListFlatRowHasLegacyUnitMismatch(row = {}) {
  const quantityBasis = String(row?.quantity_basis || row?.quantityBasis || '').trim()
  return quantityBasis !== 'sales_spec_count' && (row?.tier_unit_compatible === false || row?.tierUnitCompatible === false)
}

function mergePriceListPricingRuleTrialCache(entries = {}) {
  if (!entries || typeof entries !== 'object' || Array.isArray(entries) || !Object.keys(entries).length) return
  priceListPricingRuleTrialCache.value = {
    ...priceListPricingRuleTrialCache.value,
    ...entries,
  }
}

function clearPriceListPricingRuleTrialErrorCache(rows = []) {
  const keys = new Set()
  ;(Array.isArray(rows) ? rows : []).forEach((row) => {
    const payload = priceTablePricingRuleTrialPayload(row, { customerID: activeBeanListCustomerID.value })
    const key = priceTablePricingRuleTrialCacheKey(payload)
    if (key) keys.add(key)
  })
  priceListPricingRuleTrialCache.value = priceListPricingRuleTrialCacheForRetry(
    priceListPricingRuleTrialCache.value,
    [...keys],
  )
}

async function loadPriceListPricingRuleTrials(requests = []) {
  const pending = (Array.isArray(requests) ? requests : []).filter(({ key }) => {
    const cached = priceListPricingRuleTrialCache.value[key]
    return key && cached?.status !== 'loading' && cached?.status !== 'success' && cached?.status !== 'error'
  })
  if (!pending.length) return
  mergePriceListPricingRuleTrialCache(Object.fromEntries(pending.map(({ key }) => [key, { status: 'loading' }])))
  const completed = await executePriceListPricingRuleTrialBatches(pending, {
    chunkSize: 100,
    timeoutMs: 30000,
    sendBatch: (payloads, { signal }) => (
      apiSend('/api/costing/pricing-rule-trials', {
        method: 'POST',
        body: { requests: payloads },
        signal,
      })
    ),
  })
  mergePriceListPricingRuleTrialCache(completed)
}

function priceListGroupSnapshot(group = {}) {
  return {
    group_id: Number(group.group_id || 0),
    group_name: group.group_name || '商品价格表分组',
    group_item_id: Number(group.group_item_id || 0),
    group_item_name: group.group_item_name || '未分组',
    parent_group_item_id: Number(group.parent_group_item_id || 0),
    parent_group_item_name: group.parent_group_item_name || '',
    classification_snapshot: selectedProductPriceListLabel.value || '',
  }
}

function tierFlatFinalPrice(tier = {}) {
  return firstPositiveNumber(tier.final_unit_price, tier.finalUnitPrice, tier.price_per_unit, tier.pricePerUnit, tier.price_per_kg, tier.pricePerKg, tier.price_per_lb, tier.pricePerLb, tier.packed_price_per_bag, tier.packedPricePerBag, tier.packed_price_per_box, tier.packedPricePerBox)
}

function priceTierTemplateByID(id) {
  return priceTierTemplates.value.find((row) => Number(row.id || 0) === Number(id || 0)) || null
}

function firstPriceSourceTier(tiers = []) {
  return (Array.isArray(tiers) ? tiers : []).find((tier) => tierFlatFinalPrice(tier) > 0) || (Array.isArray(tiers) ? tiers[0] : null) || {}
}

function tierForTemplateTier(templateTier = {}, sourceTiers = [], index = 0) {
  const label = String(templateTier.label || '').trim()
  const rows = Array.isArray(sourceTiers) ? sourceTiers : []
  return rows.find((tier) => String(tier?.label || '').trim() === label) || rows[index] || firstPriceSourceTier(rows)
}

function flatRowPriceUnit(tier = {}, item = {}) {
  const raw = productCurrentSalesSpecUnit(item) || String(tier.price_unit || tier.priceUnit || tier.display_unit || tier.displayUnit || item.inventory_unit || item.inventoryUnit || '').trim()
  if (raw === '磅') return 'lb'
  if (raw === '公斤' || raw === '千克') return 'kg'
  if (raw) return raw
  return Number(tier.spec_g || tier.specG || 0) === 1000 ? 'kg' : 'lb'
}

function flatRowInventoryConversion(tier = {}, priceUnit = '', inventoryUnit = '', item = {}) {
  const source = String(priceUnit || '').trim()
  const target = String(inventoryUnit || '').trim()
  const itemConversion = normalizeFlatRowConversion(item.unit_conversion_json ?? item.unitConversionJSON)
  const itemSnapshot = flatRowConversionForUnits(itemConversion, source, target)
  if (Object.keys(itemSnapshot).length) return itemSnapshot
  const tierConversion = normalizeFlatRowConversion(tier.inventory_conversion_json ?? tier.inventoryConversionJSON)
  const tierSnapshot = flatRowConversionForUnits(tierConversion, source, target)
  if (Object.keys(tierSnapshot).length) return tierSnapshot
  if (Object.keys(tierConversion).length) return tierConversion
  if (!source || !target) return {}
  if (source === target) return { [source]: { [target]: 1 } }
  if ((source === 'lb' || source === '磅') && target === 'kg') return { lb: { kg: 0.454 } }
  if (source === 'kg' && (target === 'lb' || target === '磅')) return { kg: { lb: 2.20462 } }
  return {}
}

function normalizeFlatRowConversion(raw = {}) {
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) return raw
  if (typeof raw === 'string' && raw.trim()) {
    try {
      const parsed = JSON.parse(raw)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return parsed
    } catch {
      return {}
    }
  }
  return {}
}

function flatRowConversionForUnits(conversion = {}, priceUnit = '', inventoryUnit = '') {
  const source = String(priceUnit || '').trim()
  const target = String(inventoryUnit || '').trim()
  if (!source || !target) return {}
  const targets = conversion?.[source]
  if (!targets || typeof targets !== 'object' || Array.isArray(targets)) return {}
  const factor = Number(targets[target] || 0)
  if (!Number.isFinite(factor) || factor <= 0) return {}
  return { [source]: { [target]: Number(factor.toFixed(8)) } }
}

function priceListFlatRowUnitSummary(row = {}) {
  const priceUnit = String(row.price_unit || '').trim() || '-'
  const inventoryUnit = String(row.inventory_unit || '').trim() || '-'
  const conversion = normalizeFlatRowConversion(row.inventory_conversion_json)
  const factor = Number(conversion?.[priceUnit]?.[inventoryUnit] || 0)
  if (Number.isFinite(factor) && factor > 0) {
    return `商品档案单位：1 ${priceUnit} = ${Number(factor.toFixed(8))} ${inventoryUnit}`
  }
  if (priceUnit && inventoryUnit && priceUnit === inventoryUnit) {
    return `商品档案单位：${priceUnit} = ${inventoryUnit}`
  }
  return `商品档案单位：${priceUnit} -> ${inventoryUnit}`
}

function pricingRuleByID(id) {
  return pricingRules.value.find((row) => Number(row.id || 0) === Number(id || 0)) || null
}

function pricingRuleVersion(rule = null) {
  if (!rule) return ''
  return String(rule.code || rule.version || rule.name || `PR-${rule.id || 0}`).trim()
}

function costSourceSnapshotForPriceRow(item = {}, tier = {}, pricingRule = null, pricingMode = '') {
  return {
    cost_source_mode: pricingMode === 'fixed_price' ? 'fixed_price' : (pricingRule?.cost_source_mode || pricingRule?.costSourceMode || 'pricing_rule'),
    pricing_mode: pricingMode || '',
    pricing_rule_id: Number(pricingRule?.id || 0),
    pricing_rule_code: pricingRule?.code || '',
    pricing_rule_name: pricingRule?.name || '',
    pricing_rule_formula_version: pricingRule?.formula_version || pricingRule?.formulaVersion || '',
    pricing_rule_calculation: pricingRuleCalculationSnapshot(pricingRule),
    bom_version_id: Number(item.bom_version_id_snapshot || item.bom_version_id || item.bomVersionID || 0),
    bom_version_no: item.bom_version_no_snapshot || item.bom_version_no || item.bomVersionNo || '',
    bom_usage_mode: item.bom_usage_mode_snapshot || item.bom_usage_mode || item.bomUsageMode || '',
    process_route_name: item.process_route_name || item.processRouteName || item.operation_template_name || item.operationTemplateName || '',
    tier_label: tier.label || '',
  }
}

function pricingRuleCalculationSnapshot(rule = null) {
  if (!rule) return {}
  const raw = rule.calculation_json ?? rule.calculationJSON ?? {}
  let parsed = {}
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
    parsed = raw
  } else if (typeof raw === 'string' && raw.trim()) {
    try {
      const decoded = JSON.parse(raw)
      parsed = decoded && typeof decoded === 'object' && !Array.isArray(decoded) ? decoded : {}
    } catch {
      parsed = {}
    }
  }
  return stripPricingRuleQuantityFields(parsed)
}

function stripPricingRuleQuantityFields(value) {
  if (Array.isArray(value)) return value.map((item) => stripPricingRuleQuantityFields(item))
  if (!value || typeof value !== 'object') return value
  const forbidden = new Set(['min_qty', 'minQty', 'max_qty', 'maxQty', 'tier_label', 'tierLabel', 'tier_name', 'tierName', 'tiers', 'quantity_unit', 'quantityUnit', 'position', 'final_unit_price', 'finalUnitPrice', 'customer_tiers', 'customerTiers'])
  return Object.fromEntries(Object.entries(value)
    .filter(([key]) => !forbidden.has(String(key || '').trim()))
    .map(([key, child]) => [key, stripPricingRuleQuantityFields(child)]))
}

function customerReferenceSnapshotForPriceRow(item = {}) {
  return {
    customer_product_alias_id: Number(item.customer_product_alias_id || item.customerProductAliasID || 0),
    customer_id: Number(item.customer_id || item.customerID || 0),
    customer_display_name: item.display_name_snapshot || item.customer_product_display_name || item.customerProductDisplayName || '',
    customer_item_code: item.customer_item_code_snapshot || item.customer_item_code || item.customerItemCode || '',
    brand_name: item.brand_name_snapshot || item.brand_name || item.brandName || '',
  }
}

function setPriceListFlatRowPrice(row = {}, value) {
  const key = String(row.row_key || '')
  if (!key) return
  const numeric = Number(value)
  const next = { ...priceListFlatRowOverrides.value }
  if (Number.isFinite(numeric) && numeric > 0) next[key] = numeric
  else delete next[key]
  priceListFlatRowOverrides.value = next
}

function defaultPriceTierTemplateForm(template = {}) {
  const hasPersistedTemplate = Number(template.id || 0) > 0
  return {
    id: Number(template.id || 0),
    name: String(template.name || '').trim(),
    active: Boolean(template.active ?? true),
    remark: String(template.remark || '').trim(),
    tiers: Array.isArray(template.tiers) && template.tiers.length
      ? template.tiers.map((tier, index) => defaultPriceTierTemplateTier(tier, index))
      : (hasPersistedTemplate ? [] : [defaultPriceTierTemplateTier({}, 0)]),
  }
}

function defaultPriceTierTemplateTier(tier = {}, index = 0) {
  return {
    id: Number(tier.id || 0),
    label: String(tier.label || '').trim(),
    min_qty: Number(tier.min_qty ?? tier.minQty ?? 0) || 0,
    max_qty: tier.max_qty === null || tier.max_qty === undefined ? '' : tier.max_qty,
    quantity_unit: 'sales_spec_count',
    pricing_rule_id: Number(tier.pricing_rule_id || tier.pricingRuleID || 0),
    position: Number(tier.position || index + 1),
    active: Boolean(tier.active ?? true),
    remark: String(tier.remark || '').trim(),
  }
}

function openTierTemplateDrawer() {
  tierTemplateDrawerOpen.value = true
  if (!priceTierTemplateForm.value?.id && priceTierTemplates.value.length) {
    startPriceTierTemplateEdit(priceTierTemplates.value[0])
  }
}

function closeTierTemplateDrawer() {
  tierTemplateDrawerOpen.value = false
}

function resetPriceTierTemplateForm() {
  priceTierTemplateForm.value = defaultPriceTierTemplateForm()
}

function startPriceTierTemplateEdit(template = {}) {
  priceTierTemplateForm.value = defaultPriceTierTemplateForm(JSON.parse(JSON.stringify(template || {})))
}

function addPriceTierTemplateTier() {
  priceTierTemplateForm.value.tiers.push(defaultPriceTierTemplateTier({}, priceTierTemplateForm.value.tiers.length))
}

function removePriceTierTemplateTier(index) {
  priceTierTemplateForm.value.tiers.splice(index, 1)
  if (!priceTierTemplateForm.value.tiers.length) addPriceTierTemplateTier()
}

async function savePriceTierTemplate() {
  const payload = buildPriceTierTemplatePayload(priceTierTemplateForm.value)
  if (!payload.name) {
    error.value = '请填写阶梯模板名称'
    return
  }
  if (!payload.tiers.length || payload.tiers.some((tier) => !tier.label || !(Number(tier.pricing_rule_id || 0) > 0))) {
    error.value = '每个档位都需要档位名和价格计算模板'
    return
  }
  tierTemplateSaving.value = true
  error.value = ''
  try {
    const url = payload.id ? `/api/price-tier-templates/${payload.id}` : '/api/price-tier-templates'
    const result = await apiSend(url, {
      method: payload.id ? 'PUT' : 'POST',
      body: payload,
    })
    const row = defaultPriceTierTemplateForm(result.template || payload)
    const next = priceTierTemplates.value.filter((template) => Number(template.id || 0) !== Number(row.id || 0))
    next.push(row)
    priceTierTemplates.value = next.filter((template) => template.active !== false).sort((a, b) => Number(a.id || 0) - Number(b.id || 0))
    priceTierTemplateForm.value = row
    message.value = '阶梯模板已保存'
  } catch (err) {
    error.value = err.message || '保存阶梯模板失败'
  } finally {
    tierTemplateSaving.value = false
  }
}

async function deletePriceTierTemplate() {
  const id = Number(priceTierTemplateForm.value.id || 0)
  if (!id) return
  tierTemplateSaving.value = true
  error.value = ''
  try {
    await apiSend(`/api/price-tier-templates/${id}`, { method: 'DELETE' })
    priceTierTemplates.value = priceTierTemplates.value.filter((template) => Number(template.id || 0) !== id)
    resetPriceTierTemplateForm()
    message.value = '阶梯模板已删除'
  } catch (err) {
    error.value = err.message || '删除阶梯模板失败'
  } finally {
    tierTemplateSaving.value = false
  }
}

function priceTierTemplateLabel(template = {}) {
  return template.name || `阶梯模板 ${template.id || ''}`
}

function pricingRuleLabel(rule = {}) {
  const code = rule.code ? `${rule.code} · ` : ''
  return `${code}${rule.name || `Pricing Rule ${rule.id || ''}`}`
}

function priceTablePricingModeLabel(mode) {
  return priceTablePricingModeOptions.find((item) => item.value === String(mode || '').trim())?.label || '计价模式'
}

function priceListSourceLabel(source) {
  switch (String(source || '').trim()) {
  case 'sku':
    return '规格'
  case 'parent_product':
    return '商品'
  case 'product':
    return '规格（历史）'
  case 'subgroup':
    return '子类'
  case 'parent_group':
    return '父类'
  case 'default':
    return '价格表'
  case 'tier_template':
    return '阶梯模板'
  default:
    return source || '-'
  }
}

function beanListItemsForType(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  void listType
  return priceListScopedItems()
    .filter((item) => matchesProductTypeCategory(item, productTypeCategoryID))
    .slice()
    .sort((a, b) => compareBeanCodes(beanMetaForItem(a).code, beanMetaForItem(b).code))
}

function priceListProductFamiliesForType(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  void listType
  return buildPriceListProductFamilies(priceListScopedItems())
    .filter((family) => matchesProductTypeCategory(family.parent_item || family, productTypeCategoryID))
    .slice()
    .sort((a, b) => compareBeanCodes(beanMetaForItem(a).code, beanMetaForItem(b).code))
}

function customerBeanListItems(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  void listType
  return priceListScopedItems()
    .filter((item) => matchesProductTypeCategory(item, productTypeCategoryID))
    .slice()
    .sort((a, b) => compareBeanCodes(beanMetaForItem(a).code, beanMetaForItem(b).code))
}

function beanMetaForItem(item) {
  const listType = priceListRenderTypeForItem(item)
  const key = metaKeyForListType(listType)
  const meta = beanMeta(item, key)
  if (meta && meta.code) return meta

  // fallback: 产品分类为生豆但 product_kind 非 green_bean 时，green_bean_list meta 可能为空
  // 尝试从缓存或已发布版本中获取 code
  if (listType === 'green') {
    const fallback = beanMeta(item, 'commercial_bean_list')
    if (fallback && fallback.code) return fallback
  }
  return meta || {}
}

function metaKeyForItem(item) {
  return metaKeyForListType(priceListRenderTypeForItem(item))
}

function matchesProductTypeCategory(item, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const selectedType = productPriceListTypeOptions.value.find((type) => Number(type.id || 0) === Number(productTypeCategoryID || 0))
  if (selectedType?.productCatalogGroupID) {
    return matchesProductCatalogPriceListType(item, selectedType, {
      assignments: priceListProductBusinessGroupAssignments.value,
    })
  }
  return matchesCurrentProductTypeCategory(item, productTypeCategoryID)
}

function priceListScopedItems() {
  return visibleCostingItems.value
}

function scopedBeanListItems(scope, listType) {
  void listType
  const customerID = selectedBeanListCustomerID.value
  return filterBeanListItemsForPriceTableScope(items.value, scope, customerID)
}

function beanListCategoryOptions(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  return productGroupsForType(listType, productTypeCategoryID).map((group) => ({
    code: group.code,
    category: group.category,
    label: group.label,
  }))
}

function productGroupsForType(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const template = selectedProductCatalogGroupTemplate.value
  const groups = groupRowsByBusinessGroupTemplate(priceListProductFamiliesForType(listType, productTypeCategoryID), {
    template,
    assignments: priceListProductBusinessGroupAssignments.value,
    usageKey: 'product_catalog',
    objectKey: 'product',
    objectIDForRow: (item) => priceListParentProductID(item),
    unclassifiedLabel: '未分类',
  }).map((group) => {
    const label = group.label || group.path_label || '未分类'
    const pathLabel = group.path_label || label
    const parentName = pathLabel.includes(' / ') ? pathLabel.split(' / ')[0] : ''
    return {
      code: String(group.key || `business-group-${group.group_id || 0}-${group.group_item_id || 0}`),
      category: label,
      label,
      path_label: pathLabel,
      depth: Number(group.depth || 0),
      group_id: Number(group.group_id || template?.id || 0),
      group_name: template?.name || selectedProductPriceListLabel.value || '商品分组',
      group_item_id: Number(group.group_item_id || 0),
      group_item_name: label,
      parent_group_item_id: Number(group.parent_group_item_id || 0),
      parent_group_item_name: parentName,
      items: Array.isArray(group.rows) ? group.rows : [],
      unclassified: Boolean(group.unclassified),
    }
  })
  return priceListVisibleCategoryRows(groups)
}

function categoryCodeOfItem(item, listType = pdfTheme.value.listType) {
  void listType
  const annotatedCode = String(item?.__price_list_category_code || '').trim()
  if (annotatedCode) return annotatedCode
  const groupItemID = productCatalogGroupItemIDOfItem(item)
  const templateID = Number(selectedProductCatalogGroupTemplate.value?.id || 0)
  if (templateID > 0 && groupItemID > 0) return `business-group-${templateID}-${groupItemID}`
  return 'business-group-unclassified'
}

function productCatalogGroupItemIDOfItem(item = {}) {
  const annotatedGroupItemID = Number(item?.__price_list_group_item_id || 0)
  if (annotatedGroupItemID > 0) return annotatedGroupItemID
  const templateID = Number(selectedProductCatalogGroupTemplate.value?.id || 0)
  if (!templateID) return 0
  const objectID = priceListParentProductID(item)
  if (!objectID) return 0
  const assignment = (priceListProductBusinessGroupAssignments.value || []).find((row) => (
    String(row?.usage_key ?? row?.usageKey ?? '') === 'product_catalog' &&
    String(row?.object_key ?? row?.objectKey ?? '') === 'product' &&
    Number(row?.object_id ?? row?.objectID ?? 0) === objectID
  ))
  if (Number(assignment?.group_id ?? assignment?.groupID ?? 0) !== templateID) return 0
  const groupItemID = Number(assignment?.group_item_id ?? assignment?.groupItemID ?? 0)
  return selectedProductCatalogGroupItemIDs.value.has(groupItemID) ? groupItemID : 0
}

function publicationRows(scope, listType, productTypeCategoryID = activeProductTypeCategoryID.value, purpose = FACTORY_SUPPLY_PUBLICATION_PURPOSE) {
  const cacheKey = beanListPublicationCacheKey(scope, purpose)
  const typeKey = beanListPublicationTypeKey(listType, productTypeCategoryID)
  const rows = beanListPublications.value?.[cacheKey]?.[typeKey] || []
  return rows.filter((row) => matchesCurrentPublicationProductType(row, productTypeCategoryID))
}

function initializePdfDefaults() {
  productPriceListTypeOptions.value.forEach((type) => {
    initializePdfDefaultsForType(type.listType, type.id)
  })
}

function initializePdfDefaultsIfItemsLoaded() {
  if (!items.value.length) return
  initializePdfDefaults()
}

function resetPdfSelectionDefaults() {
  productSpecSelectionsByType.value = {}
  visibleCategoryCodesByType.value = {}
  productSelectionInitialized.value = {}
  categorySelectionInitialized.value = {}
}

function initializePdfDefaultsForType(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const cacheKey = priceListSelectionKey(listType, productTypeCategoryID)
  const families = priceListProductFamiliesForType(listType, productTypeCategoryID)
  const validCategories = beanListCategoryOptions(listType, productTypeCategoryID).map((item) => item.code)
  if (!productSelectionInitialized.value[cacheKey]) {
    productSpecSelectionsByType.value = {
      ...productSpecSelectionsByType.value,
      [cacheKey]: defaultPriceListProductSpecSelections(families),
    }
    productSelectionInitialized.value = { ...productSelectionInitialized.value, [cacheKey]: true }
  } else {
    const current = productSpecSelectionsByType.value[cacheKey] || []
    productSpecSelectionsByType.value = {
      ...productSpecSelectionsByType.value,
      [cacheKey]: normalizePriceListProductSpecSelections(current, families, { fallbackInvalid: true }),
    }
  }
  if (!categorySelectionInitialized.value[cacheKey]) {
    visibleCategoryCodesByType.value = { ...visibleCategoryCodesByType.value, [cacheKey]: validCategories }
    categorySelectionInitialized.value = { ...categorySelectionInitialized.value, [cacheKey]: true }
  } else {
    const current = visibleCategoryCodesByType.value[cacheKey] || []
    visibleCategoryCodesByType.value = { ...visibleCategoryCodesByType.value, [cacheKey]: current.filter((code) => validCategories.includes(code)) }
  }
}

function openBeanListDrawer(listType = selectedProductPriceListType.value?.listType || 'commercial') {
  downloadSourcePublication.value = null
  syncPublicationScopeFromPageContext()
  const selected = selectedProductPriceListType.value
  const resolvedListType = selected?.listType || normalizeBeanListType(listType)
  const isCustomerScope = activeCostingScope.value === 'customer'
  pdfOptions.value = {
    ...pdfOptions.value,
    listType: resolvedListType,
    version: defaultBeanListVersionForScope(resolvedListType, activeProductTypeCategoryID.value),
    brandName: isCustomerScope ? '' : '棵凡咖啡',
  }
  initializePdfDefaultsForType(resolvedListType, activeProductTypeCategoryID.value)
  restorePriceListGenerationDraftForActiveType()
  loadBeanListPublications(resolvedListType, 'official', activeProductTypeCategoryID.value, 'factory_supply')
  loadBeanListPublications(resolvedListType, 'mine', activeProductTypeCategoryID.value, 'factory_supply')
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(resolvedListType, 'customer', activeProductTypeCategoryID.value, 'factory_supply')
  }
  pdfDrawerOpen.value = true
}

function defaultBeanListVersionForScope(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const key = beanListPublicationTypeKey(listType, productTypeCategoryID)
  const source = priceSourcePublicationByType.value[key]
  return defaultBeanListDraftVersion(publicationRows(publicationScope.value, listType, productTypeCategoryID), source)
}

function beanListPublicationLabel(row) {
  const status = beanListPublicationStatusLabel(row)
  const time = beanListPublicationTime(row)
  return [row?.version || '未命名版本', status, time].filter(Boolean).join(' · ')
}

function normalizeBeanListType(value) {
  if (value === 'green' || value === 'green_bean') return 'green'
  if (value === 'drip') return 'drip'
  if (value === 'retail') return 'retail'
  return 'commercial'
}

function beanListPublicationStatusLabel(row) {
  switch (String(row?.status || '').trim()) {
  case 'published':
    return '已发布'
  case 'withdrawn':
    return '已撤回'
  case 'draft':
    return '草稿'
  case 'archived':
    return '已归档'
  default:
    return row?.status || '未知'
  }
}

function beanListPublicationStatusClass(row) {
  const status = String(row?.status || '').trim()
  if (status === 'published') return 'status-published'
  if (status === 'draft') return 'status-draft'
  if (status === 'withdrawn') return 'status-withdrawn'
  if (status === 'archived') return 'status-archived'
  return 'status-unknown'
}

function beanListPublicationTime(row) {
  return row?.published_at || row?.created_at || ''
}

function publicationScopeLabel(scope) {
  const customerID = versionListScopeCustomerID(scope) || (scope === 'customer' ? Number(selectedBeanListCustomerID.value || 0) : 0)
  if (customerID > 0) {
    const customer = customers.value.find((item) => Number(item?.id || 0) === customerID)
    return customer ? customerOptionLabel(customer) : `客户 ${customerID}`
  }
  if (scope === 'official') return '公共价格表'
  if (scope === 'mine') return '我的客户价格表'
  if (scope === 'customer') return '指定客户价格表'
  return '棵凡官方价格表'
}

function beanListPublicationOwnerLabel(row) {
  if (row?.owner_type === 'customer') {
    const customerID = Number(row.owner_key || 0)
    const customer = customers.value.find((item) => Number(item?.id || 0) === customerID)
    return customer ? customerOptionLabel(customer) : `客户 ${row.owner_key || '-'}`
  }
  if (row?.owner_type === 'actor') return '我的客户价格表'
  return '棵凡官方价格表'
}

function beanListPublicationSourceLabel(row) {
  const parts = []
  if (row?.source_version) parts.push(`价格源 ${row.source_version}`)
  if (Number(row?.price_source_publication_id || 0) > 0 && !row?.source_version) {
    parts.push(`价格源 #${row.price_source_publication_id}`)
  }
  if (Number(row?.style_source_publication_id || 0) > 0) {
    parts.push(`样式源 #${row.style_source_publication_id}`)
  }
  return parts.length ? parts.join(' / ') : '本版本配置'
}

function startBeanListFromPublication(row) {
  if (!row) return
  setPublicationScopeFromOwner(row)
  selectProductTypeFromPublication(row)
  openBeanListDrawer(normalizeBeanListType(row.list_type))
}

function beanListPublicationIsCurrent(row) {
  return Number(row?.id || 0) > 0 && Number(versionListCurrentPublication.value?.id || 0) === Number(row.id || 0)
}

function canArchiveBeanListPublication(row) {
  return isBeanListAdmin.value && row?.status !== 'archived' && !beanListPublicationIsCurrent(row)
}

function isPublicationArchiveSelected(row) {
  const id = Number(row?.id || 0)
  return id > 0 && selectedPublicationArchiveIDs.value.includes(id)
}

function togglePublicationArchiveSelection(row) {
  if (!canArchiveBeanListPublication(row)) return
  const id = Number(row?.id || 0)
  if (id <= 0) return
  if (selectedPublicationArchiveIDs.value.includes(id)) {
    selectedPublicationArchiveIDs.value = selectedPublicationArchiveIDs.value.filter((value) => value !== id)
    return
  }
  selectedPublicationArchiveIDs.value = [...selectedPublicationArchiveIDs.value, id]
}

function toggleCurrentPagePublicationArchiveSelection(checked) {
  const ids = archiveSelectableCurrentPagePublicationRows.value.map((row) => Number(row.id || 0)).filter((id) => id > 0)
  if (!ids.length) return
  const selected = new Set(selectedPublicationArchiveIDs.value)
  ids.forEach((id) => {
    if (checked) selected.add(id)
    else selected.delete(id)
  })
  selectedPublicationArchiveIDs.value = [...selected]
}

function selectedPublicationArchiveRows() {
  const selected = new Set(selectedPublicationArchiveIDs.value)
  return currentScopePublicationRows.value.filter((row) => selected.has(Number(row.id || 0)) && canArchiveBeanListPublication(row))
}

function publicationArchiveRefreshProductTypeIDs(row = {}, fallbackProductTypeCategoryID = activeProductTypeCategoryID.value) {
  return Array.from(new Set([
    activeProductTypeCategoryID.value,
    fallbackProductTypeCategoryID,
    row?.product_type_category_id,
    row?.productTypeCategoryID,
    currentClassificationTemplateIDOfPublication(row),
  ].map((id) => Number(id || 0)).filter((id) => Number.isFinite(id))))
}

async function reloadBeanListPublicationsAfterArchiveChange(listType, scope, row, fallbackProductTypeCategoryID, purpose = FACTORY_SUPPLY_PUBLICATION_PURPOSE) {
  for (const refreshProductTypeID of publicationArchiveRefreshProductTypeIDs(row, fallbackProductTypeCategoryID)) {
    await loadBeanListPublications(listType, scope, refreshProductTypeID, purpose)
  }
}

function beanListPublicationArchivedFromStatus(row = {}) {
  const config = row?.config || row?.config_json || row?.configJson || {}
  return String(config.archived_from_status || config.archivedFromStatus || '').trim() || 'published'
}

function setBeanListPublicationStatusInCache(ids = [], status = '') {
  const idSet = new Set((Array.isArray(ids) ? ids : [])
    .map((id) => Number(id || 0))
    .filter((id) => id > 0))
  const nextStatus = String(status || '').trim()
  if (!idSet.size || !nextStatus) return
  const next = {}
  Object.entries(beanListPublications.value || {}).forEach(([scopeKey, scopeRows]) => {
    next[scopeKey] = {}
    Object.entries(scopeRows || {}).forEach(([typeKey, rows]) => {
      next[scopeKey][typeKey] = Array.isArray(rows)
        ? rows.map((row) => (idSet.has(Number(row?.id || 0)) ? { ...row, status: nextStatus } : row))
        : rows
    })
  })
  beanListPublications.value = next
}

function beanListPublicationHasContent(row) {
  return Array.isArray(row?.content?.groups) && row.content.groups.length > 0
}

async function downloadBeanListPublication(row) {
  if (!beanListPublicationHasContent(row)) {
    error.value = '该豆单版本没有可下载内容'
    return
  }
  error.value = ''
  message.value = ''
  try {
    const params = beanListPublicationDownloadParams(row)
    const document = await apiSend(`/api/costing/bean-list/publications/${row.id}/pdf?${params.toString()}`)
    await downloadBeanListPublicationPDF(document)
    message.value = `已生成并下载${beanListPublicationTypeLabel(row)}价格表 ${row.version || '未命名版本'} PDF`
  } catch (err) {
    error.value = err.message || '下载豆单 PDF 失败'
  } finally {
    downloadSourcePublication.value = null
  }
}

function beanListPublicationDownloadParams(row) {
  const params = beanListWithdrawScopeParams(row)
  params.set('list_type', normalizeBeanListType(row?.list_type || pdfTheme.value.listType))
  params.set('publication_purpose', row?.publication_purpose || 'factory_supply')
  const productTypeCategoryID = currentClassificationTemplateIDOfPublication(row)
  if (productTypeCategoryID > 0) params.set('product_type_category_id', String(productTypeCategoryID))
  return params
}

async function downloadBeanListPublicationPDF(document) {
  if (!document?.download_url) {
    throw new Error('豆单 PDF 下载地址缺失')
  }
  const response = await apiFetch(document.download_url)
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || '下载豆单 PDF 失败')
  }
  const blob = await response.blob()
  const href = URL.createObjectURL(blob)
  const anchor = window.document.createElement('a')
  anchor.href = href
  anchor.download = document.filename || 'bean-list.pdf'
  window.document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(href)
}

function setPublicationScopeFromOwner(row) {
  if (row?.owner_type === 'customer') {
    selectedBeanListCustomerID.value = Number(row.owner_key || 0)
    publicationScope.value = 'customer'
    versionListScope.value = `customer:${Number(row.owner_key || 0)}`
    return
  }
  if (row?.owner_type === 'actor') {
    publicationScope.value = 'mine'
    return
  }
  publicationScope.value = 'official'
  versionListScope.value = 'official'
}

function beanListTypeName(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return '生豆'
  if (normalized === 'drip') return '挂耳'
  return normalized === 'retail' ? '零售' : '商用'
}

function applyCopiedBeanListPriceSource(row = selectedPriceSourcePublication.value) {
  if (!row) return
  const listType = normalizeBeanListType(row.list_type)
  selectProductTypeFromPublication(row)
  const keyProductTypeID = currentClassificationTemplateIDOfPublication(row) || UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID
  const key = beanListPublicationTypeKey(listType, keyProductTypeID)
  downloadSourcePublication.value = null
  priceSourcePublicationByType.value = { ...priceSourcePublicationByType.value, [key]: row }
  selectedPriceSourcePublicationID.value = String(row.id)
  pdfOptions.value = { ...pdfOptions.value, listType, version: defaultBeanListVersionForScope(listType, keyProductTypeID) }
  message.value = `已复制${beanListPublicationLabel(row)}价格来源，发布后会锁定为客户价格表快照`
}

function beanListTypeLabel(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return '生豆'
  if (normalized === 'drip') return '挂耳'
  return normalized === 'retail' ? '零售' : '商用'
}

function beanListPublicationTypeLabel(row) {
  const classificationName = currentClassificationTemplateNameOfPublication(row)
  if (classificationName) return classificationName
  const classificationID = currentClassificationTemplateIDOfPublication(row)
  if (classificationID > 0) {
    const option = productPriceListTypeOptions.value.find((type) => Number(type.categoryID || type.id || 0) === classificationID)
    if (option?.label) return option.label
  }
  const activeTypeID = Number(activeProductTypeCategoryID.value || 0)
  if (activeTypeID !== 0 && matchesCurrentPublicationProductType(row, activeTypeID)) {
    return selectedProductPriceListLabel.value || '商品价格表'
  }
  return '未分类商品'
}

function selectProductTypeFromPublication(row) {
  const classificationTemplateID = currentClassificationTemplateIDOfPublication(row)
  if (classificationTemplateID > 0) {
    selectedProductTypeCategoryID.value = classificationTemplateID
    return
  }
  selectedProductTypeCategoryID.value = UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID
}

function isPdfProductSelected(parentProductID) {
  const normalizedParentID = Number(parentProductID || 0)
  return pdfProductSpecSelections.value.some((row) => Number(row.parent_product_id || 0) === normalizedParentID)
}

function selectedSpecsForProduct(family = {}) {
  const parentProductID = priceListParentProductID(family)
  const selectedSkuIDs = new Set(pdfProductSpecSelections.value
    .filter((row) => Number(row.parent_product_id || 0) === parentProductID)
    .map((row) => Number(row.sku_id || 0)))
  return (Array.isArray(family.sku_options) ? family.sku_options : []).filter((spec) => selectedSkuIDs.has(priceListSkuID(spec)))
}

function isPdfProductSpecSelected(family = {}, spec = {}) {
  const parentProductID = priceListParentProductID(family)
  const skuID = priceListSkuID(spec)
  return pdfProductSpecSelections.value.some((row) => (
    Number(row.parent_product_id || 0) === parentProductID && Number(row.sku_id || 0) === skuID
  ))
}

function productSpecSelectionIssueForFamily(family = {}) {
  return priceListProductSpecSelectionIssue(family, pdfProductSpecSelections.value)
}

function resolveProductSpecSelectionIssue(family = {}, action = 'switch') {
  updatePdfProductSpecSelections(resolvePriceListProductSpecSelectionIssue(
    pdfProductSpecSelections.value,
    family,
    action,
  ))
}

function updatePdfProductSpecSelections(next = []) {
  const key = activePriceListTypeKey.value
  const normalized = normalizePriceListProductSpecSelections(next, pdfAvailableItems.value)
  productSpecSelectionsByType.value = { ...productSpecSelectionsByType.value, [key]: normalized }
  const selectedSkuIDs = normalized.map((row) => String(row.sku_id || '')).filter(Boolean)
  syncCategoryVisibilityFromSelectedProducts(pdfTheme.value.listType, selectedSkuIDs, activeProductTypeCategoryID.value)
  savePriceListGenerationDraftForActiveType()
  schedulePriceListPricingRuleTrialRefresh()
}

function togglePdfProduct(family = {}, checked) {
  const parentProductID = priceListParentProductID(family)
  const current = pdfProductSpecSelections.value
  const next = checked
    ? (isPdfProductSelected(parentProductID) ? current : [...current, ...defaultPriceListProductSpecSelections([family])])
    : current.filter((row) => Number(row.parent_product_id || 0) !== parentProductID)
  if (!checked && priceListPricingPopover.value.type === 'product' && Number(priceListPricingPopover.value.productRow?.parent_product_id || 0) === parentProductID) {
    closePriceListPricingPopover()
  }
  updatePdfProductSpecSelections(next)
}

function togglePdfProductSpec(family = {}, spec = {}, checked) {
  const skuID = priceListSkuID(spec)
  const next = togglePriceListProductSpecSelection(pdfProductSpecSelections.value, family, skuID, checked)
  if (!checked && priceListPricingPopover.value.type === 'product' && Number(priceListPricingPopover.value.productRow?.sku_id || 0) === skuID) {
    closePriceListPricingPopover()
  }
  updatePdfProductSpecSelections(next)
}

function setAllPdfProducts(selected) {
  const rows = [{ code: '__all__', items: pdfAvailableItems.value }]
  const next = selected
    ? setPriceListCategorySpecSelection(rows, '__all__', pdfProductSpecSelections.value, true)
    : []
  if (!selected) closePriceListPricingPopover()
  updatePdfProductSpecSelections(next)
}

function isPdfCategoryVisible(code) {
  return pdfVisibleCategoryCodes.value.includes(String(code))
}

function isPdfCategorySelected(code) {
  const ids = productIDsForCategory(code)
  return ids.length > 0 && ids.every((id) => isPdfProductSelected(id))
}

function isPdfCategoryPartiallySelected(code) {
  const ids = productIDsForCategory(code)
  const selectedCount = ids.filter((id) => isPdfProductSelected(id)).length
  return selectedCount > 0 && selectedCount < ids.length
}

function selectedCountForCategory(code) {
  const ids = productIDsForCategory(code)
  return ids.filter((id) => isPdfProductSelected(id)).length
}

function selectedSpecCountForCategory(code) {
  const parentIDs = new Set(productIDsForCategory(code).map((id) => Number(id || 0)))
  return pdfProductSpecSelections.value.filter((row) => parentIDs.has(Number(row.parent_product_id || 0))).length
}

function productIDsForCategory(code, listType = pdfTheme.value.listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  void listType
  void productTypeCategoryID
  return priceListCategoryProductIDs(categoryProductGroups.value, code)
}

function togglePdfCategoryProducts(code, checked) {
  const next = setPriceListCategorySpecSelection(categoryProductGroups.value, code, pdfProductSpecSelections.value, checked)
  if (!checked) {
    const categoryIDs = new Set(productIDsForCategory(code).map((id) => Number(id || 0)))
    if (priceListPricingPopover.value.type === 'product' && categoryIDs.has(Number(priceListPricingPopover.value.productRow?.parent_product_id || 0))) {
      closePriceListPricingPopover()
    }
  }
  updatePdfProductSpecSelections(next)
}

function setAllPdfCategories(selected) {
  setAllPdfProducts(selected)
}

function syncCategoryVisibilityFromSelectedProducts(listType, selectedIDs, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const key = priceListSelectionKey(listType, productTypeCategoryID)
  const next = priceListCategoryCodesForSelectedProducts(categoryProductGroups.value, selectedIDs)
  visibleCategoryCodesByType.value = { ...visibleCategoryCodesByType.value, [key]: next }
}

function customizerField(id, field) {
  return pdfCustomizers.value[String(id)]?.[field] || ''
}

function setCustomizerField(id, field, value) {
  const key = String(id)
  pdfCustomizers.value = {
    ...pdfCustomizers.value,
    [key]: {
      ...(pdfCustomizers.value[key] || {}),
      [field]: value,
    },
  }
}

function greenTierPriceRows(item) {
  return Array.isArray(item?.green_bean_sale_tiers) ? item.green_bean_sale_tiers : []
}

function greenTierOverrideKey(tier) {
  return String(tier?.template_tier_id || tier?.templateTierID || tier?.label || '')
}

function greenTierPriceValue(id, tier) {
  const key = String(id)
  const tierKey = greenTierOverrideKey(tier)
  const overrides = pdfCustomizers.value[key]?.greenPriceOverrides || {}
  const overridden = Number(overrides[tierKey])
  if (Number.isFinite(overridden) && overridden > 0) return overridden
  return greenTierDisplayPrice(tier)
}

function greenTierPriceUnit(tier = {}) {
  const unit = greenTierPriceUnitCode(tier)
  if (unit === 'kg') return 'kg'
  if (unit === 'g100') return '100g'
  if (unit === 'g227') return '227g'
  if (unit === 'g250') return '250g'
  return '磅'
}

function greenTierDisplayPrice(tier = {}) {
  const unit = greenTierPriceUnitCode(tier)
  const pricePerKg = firstPositiveNumber(
    tier?.price_per_kg,
    tier?.pricePerKg,
    unit === 'kg' ? tier?.price_per_unit : undefined,
    unit === 'kg' ? tier?.pricePerUnit : undefined,
    normalizeGreenTierUnit(tier?.display_unit) === 'kg' ? tier?.price_per_unit : undefined,
    normalizeGreenTierUnit(tier?.displayUnit) === 'kg' ? tier?.pricePerUnit : undefined,
    firstPositiveNumber(tier?.price_per_lb, tier?.pricePerLb) / 0.454,
  )
  if (unit === 'kg') return Number(pricePerKg.toFixed(2))
  if (unit === 'lb') {
    return Number(firstPositiveNumber(
      tier?.price_per_lb,
      tier?.pricePerLb,
      normalizeGreenTierUnit(tier?.price_unit) === 'lb' ? tier?.price_per_unit : undefined,
      normalizeGreenTierUnit(tier?.priceUnit) === 'lb' ? tier?.pricePerUnit : undefined,
      pricePerKg * 0.454,
    ).toFixed(2))
  }
  const unitG = greenTierPriceUnitG(unit, tier)
  if (pricePerKg > 0) return Number((pricePerKg * unitG / 1000).toFixed(2))
  return firstPositiveNumber(tier?.price_per_unit, tier?.pricePerUnit)
}

function greenTierPriceUnitCode(tier = {}) {
  const explicit = normalizeGreenTierUnit(tier?.price_unit || tier?.priceUnit)
  const display = normalizeGreenTierUnit(tier?.display_unit || tier?.displayUnit)
  return explicit || display || 'lb'
}

function normalizeGreenTierUnit(unit) {
  const value = String(unit || '').trim().toLowerCase()
  return ['kg', 'lb', 'g100', 'g227', 'g250'].includes(value) ? value : ''
}

function greenTierPriceUnitG(unit, tier = {}) {
  switch (normalizeGreenTierUnit(unit)) {
    case 'kg':
      return 1000
    case 'lb':
      return 454
    case 'g100':
      return 100
    case 'g227':
      return 227
    case 'g250':
      return 250
    default:
      return Number(tier?.spec_g || tier?.specG || 454) || 454
  }
}

function firstPositiveNumber(...values) {
  for (const value of values) {
    const n = Number(value)
    if (Number.isFinite(n) && n > 0) return n
  }
  return 0
}

function setGreenBeanTierPrice(id, tier, value) {
  const key = String(id)
  const tierKey = greenTierOverrideKey(tier)
  if (!tierKey) return
  const current = pdfCustomizers.value[key] || {}
  const currentOverrides = current.greenPriceOverrides || {}
  const nextOverrides = { ...currentOverrides }
  const numeric = Number(value)
  if (Number.isFinite(numeric) && numeric > 0) {
    nextOverrides[tierKey] = numeric
  } else {
    delete nextOverrides[tierKey]
  }
  pdfCustomizers.value = {
    ...pdfCustomizers.value,
    [key]: {
      ...current,
      greenPriceOverrides: nextOverrides,
    },
  }
}

function highlightedParts(text, item) {
  return splitHighlightedText(text, item?.highlightTerms || [])
}

function priceDisplay(priceRow) {
  return `${price(priceRow?.price)}${priceRow?.unit ? `/${priceRow.unit}` : ''}`
}

function priceLabelParts(priceRow, item) {
  return splitHighlightedText(priceRow?.label || '', item?.highlightTerms || [])
}

function priceValueParts(priceRow, item) {
  return splitHighlightedText(priceDisplay(priceRow), item?.highlightTerms || [])
}

function priceValueClass(priceRow, item) {
  return { 'pdf-red': Boolean(priceRow?.red) || priceValueParts(priceRow, item).some((part) => part.red) }
}

function cardRows(group) {
  const source = Array.isArray(group?.items) ? group.items : []
  const maxColumns = Math.max(1, Math.min(Number(pdfTheme.value.cardsPerRow || 1), 4))
  const rows = []
  for (let i = 0; i < source.length; i += maxColumns) {
    const rowItems = source.slice(i, i + maxColumns)
    rows.push({ items: rowItems, columns: Math.max(1, Math.min(maxColumns, rowItems.length)) })
  }
  return rows
}

function cardRowStyle(row) {
  const columns = Math.max(1, Math.min(Number(row?.columns || 1), 4))
  return { gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }
}

function badgeClass(badge) {
  return badge ? `pdf-product-badge badge-${badge}` : 'pdf-product-badge'
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

function price(value) {
  return fixed(value, 0)
}

function percent(value) {
  return `${fixed(Number(value || 0) * 100, 1)}%`
}

async function loadBeanList() {
  loading.value = true
  error.value = ''
  message.value = ''
  try {
    const [data] = await Promise.all([
      apiGet(beanListURLForCustomerRules()),
      loadPriceListProductBusinessGroups(),
    ])
    parameters.value = data.parameters
    items.value = Array.isArray(data.items) ? data.items : []
    syncSelectedProductTypeCategoryFromOptions()
    initializePdfDefaults()
    restorePriceListGenerationDraftForActiveType()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadPriceListProductBusinessGroups() {
  try {
    const [groupsData, assignmentsData] = await Promise.all([
      apiGet('/api/business-groups'),
      apiGet('/api/business-group-assignments?usage_key=product_catalog&object_key=product'),
    ])
    priceListProductBusinessGroups.value = Array.isArray(groupsData?.rows)
      ? groupsData.rows
      : (Array.isArray(groupsData?.groups) ? groupsData.groups : (Array.isArray(groupsData) ? groupsData : []))
    priceListProductBusinessGroupAssignments.value = Array.isArray(assignmentsData?.rows)
      ? assignmentsData.rows
      : (Array.isArray(assignmentsData?.assignments) ? assignmentsData.assignments : (Array.isArray(assignmentsData) ? assignmentsData : []))
    const templateOptions = productCatalogBusinessGroupControls.value.templateOptions
    if (!templateOptions.length) {
      selectedProductCatalogGroupTemplateID.value = 0
      return
    }
    const preferredTemplateID = productSettingsSelectedProductGroupTemplateID()
    if (preferredTemplateID && templateOptions.some((option) => Number(option.id || 0) === preferredTemplateID)) {
      selectedProductCatalogGroupTemplateID.value = preferredTemplateID
      return
    }
    const selectedTemplateID = Number(selectedProductCatalogGroupTemplateID.value || 0)
    const selectedTemplateValid = templateOptions.some((option) => Number(option.id || 0) === selectedTemplateID)
    const assignedTemplate = templateOptions.find((option) => priceListProductBusinessGroupAssignments.value.some((row) => (
      Number(row?.group_id ?? row?.groupID ?? 0) === Number(option.id || 0) &&
      String(row?.usage_key ?? row?.usageKey ?? '') === 'product_catalog' &&
      String(row?.object_key ?? row?.objectKey ?? '') === 'product'
    )))
    const selectedHasAssignments = priceListProductBusinessGroupAssignments.value.some((row) => (
      Number(row?.group_id ?? row?.groupID ?? 0) === selectedTemplateID &&
      String(row?.usage_key ?? row?.usageKey ?? '') === 'product_catalog' &&
      String(row?.object_key ?? row?.objectKey ?? '') === 'product'
    ))
    if (!selectedTemplateValid || (!selectedHasAssignments && assignedTemplate)) {
      selectedProductCatalogGroupTemplateID.value = Number(assignedTemplate?.id || templateOptions[0].id || 0)
    }
  } catch (err) {
    priceListProductBusinessGroups.value = []
    priceListProductBusinessGroupAssignments.value = []
    selectedProductCatalogGroupTemplateID.value = 0
  }
}

function productSettingsDraftKeyForPriceList() {
  const workspace = props.workspaceMode || 'factory'
  const customerID = Number(props.customerContextId || 0)
  return `${SKU_SETTINGS_FORM_DRAFT_SCOPE}:${workspace}:${customerID || 'all'}`
}

function productSettingsSelectedProductGroupTemplateID() {
  const draft = readFormDraft(productSettingsDraftKeyForPriceList())
  return Number(draft?.selectedProductGroupTemplateID || 0)
}

function beanListURLForCustomerRules() {
  const customerID = Number(activeBeanListCustomerID.value || 0)
  if (customerID <= 0) return '/api/costing/bean-list'
  const params = new URLSearchParams()
  params.set('customer_id', String(customerID))
  return `/api/costing/bean-list?${params.toString()}`
}

async function loadCustomers() {
  try {
    const data = await apiGet('/api/customer-fulfillment/customers?limit=200')
    const rows = Array.isArray(data.customers) ? data.customers : (data.rows || [])
    customers.value = rows.filter((row) => row.active !== false)
    if (!isWorkspaceCustomerLocked.value) {
      versionListScope.value = resolvePriceListScopePreference(versionListScope.value, customers.value)
    }
  } catch (err) {
    customers.value = []
  }
}

async function loadCustomerProductAliases() {
  try {
    const data = await apiGet('/api/customer-product-aliases?active=all')
    customerProductAliases.value = Array.isArray(data.rows) ? data.rows : []
  } catch (err) {
    customerProductAliases.value = []
  }
}

async function loadPriceListTemplateOptions() {
  try {
    const [tierData, ruleData] = await Promise.all([
      apiGet('/api/price-tier-templates'),
      apiGet('/api/product-pricing-rules'),
    ])
    priceTierTemplates.value = (tierData.templates || tierData.rows || [])
      .filter((row) => row?.active !== false)
      .map((row) => defaultPriceTierTemplateForm(row))
    pricingRules.value = (ruleData.rules || ruleData.rows || []).filter((row) => row?.active !== false)
    if (!priceListTemplateDefaults.value.tier_template_id && priceTierTemplates.value.length) {
      priceListTemplateDefaults.value = {
        ...priceListTemplateDefaults.value,
        pricing_mode: priceListTemplateDefaults.value.pricing_mode || 'tier_template',
        tier_template_id: Number(priceTierTemplates.value[0].id || 0),
      }
    }
    if (!priceListTemplateDefaults.value.pricing_rule_id && pricingRules.value.length) {
      priceListTemplateDefaults.value = {
        ...priceListTemplateDefaults.value,
        pricing_rule_id: Number(pricingRules.value[0].id || 0),
      }
    }
    restorePriceListGenerationDraftForActiveType()
  } catch (err) {
    priceTierTemplates.value = []
    pricingRules.value = []
  }
}

async function loadCurrentActor() {
  try {
    currentActor.value = await fetchCurrentActor()
  } catch (err) {
    currentActor.value = null
  } finally {
    actorLoaded.value = true
    syncPublicationScopeFromPageContext()
  }
}

async function loadBeanListPublications(listType = pdfTheme.value.listType, scope = publicationScope.value, productTypeCategoryID = activeProductTypeCategoryID.value, purpose = FACTORY_SUPPLY_PUBLICATION_PURPOSE) {
  const cacheKey = beanListPublicationCacheKey(scope, purpose)
  const typeKey = beanListPublicationTypeKey(listType, productTypeCategoryID)
  const requestScope = beanListPublicationRequestScope(scope)
  const customerID = beanListPublicationCustomerID(scope)
  if (requestScope === 'customer' && !customerID) {
    beanListPublications.value = {
      ...beanListPublications.value,
      [cacheKey]: {
        ...(beanListPublications.value[cacheKey] || {}),
        [typeKey]: [],
      },
    }
    return
  }
  try {
    const data = await apiGet(beanListPublicationURL(listType, scope, productTypeCategoryID, purpose))
    const rows = Array.isArray(data.rows) ? data.rows : []
    beanListPublications.value = {
      ...beanListPublications.value,
      [cacheKey]: {
        ...(beanListPublications.value[cacheKey] || {}),
        [typeKey]: rows,
      },
    }
  } catch (err) {
    error.value = err.message || '加载豆单发布记录失败'
  }
}

function beanListPublicationURL(listType, scope, productTypeCategoryID = activeProductTypeCategoryID.value, purpose = FACTORY_SUPPLY_PUBLICATION_PURPOSE) {
  const requestScope = beanListPublicationRequestScope(scope)
  const params = new URLSearchParams({ list_type: listType, scope: requestScope })
  params.set('publication_purpose', purpose || FACTORY_SUPPLY_PUBLICATION_PURPOSE)
  const categoryID = activePublicationProductTypeCategoryID(productTypeCategoryID)
  if (categoryID > 0) params.set('product_type_category_id', String(categoryID))
  const customerID = beanListPublicationCustomerID(scope)
  if (requestScope === 'customer') {
    params.set('customer_id', String(customerID || 0))
  }
  return `/api/costing/bean-list/publications?${params.toString()}`
}

function versionListScopeCustomerID(scope = versionListScope.value) {
  const match = String(scope || '').match(/^customer:(\d+)$/)
  return match ? Number(match[1] || 0) : 0
}

function beanListPublicationRequestScope(scope = publicationScope.value) {
  if (versionListScopeCustomerID(scope) > 0) return 'customer'
  if (scope === 'customer' || scope === 'mine') return scope
  return 'official'
}

function beanListPublicationCustomerID(scope = publicationScope.value) {
  const versionCustomerID = versionListScopeCustomerID(scope)
  if (versionCustomerID > 0) return versionCustomerID
  return scope === 'customer' ? Number(selectedBeanListCustomerID.value || 0) : 0
}

function beanListPublicationCacheKey(scope = publicationScope.value, purpose = FACTORY_SUPPLY_PUBLICATION_PURPOSE) {
  return `${String(scope || 'official')}:${purpose || FACTORY_SUPPLY_PUBLICATION_PURPOSE}`
}

async function openPriceExplanation(item, tier) {
  explanationItem.value = item
  explanationTier.value = tier
  explanationOverrides.value = {
    green_bean_cost_per_kg: '',
    yield_rate: '',
    margin_rate: '',
  }
  priceExplanation.value = null
  priceExplanationError.value = ''
  priceExplanationOpen.value = true
  await loadPriceExplanation()
}

function cleanExplanationOverrides() {
  const out = {}
  for (const [key, value] of Object.entries(explanationOverrides.value || {})) {
    if (value === '' || value == null) continue
    const n = Number(value)
    if (Number.isFinite(n)) out[key] = n
  }
  return out
}

async function loadPriceExplanation() {
  if (!explanationItem.value || !explanationTier.value) return
  priceExplanationLoading.value = true
  priceExplanationError.value = ''
  try {
    const payload = buildPriceExplanationRequest(
      explanationItem.value,
      explanationTier.value,
      cleanExplanationOverrides(),
    )
    priceExplanation.value = await apiSend('/api/costing/price-explanation', { body: payload })
  } catch (err) {
    priceExplanation.value = null
    priceExplanationError.value = err.message || '加载价格来源失败'
  } finally {
    priceExplanationLoading.value = false
  }
}

function handlePdfBackgroundUpload(event) {
  const file = event.target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    pdfOptions.value = { ...pdfOptions.value, backgroundImage: String(reader.result || '') }
  }
  reader.readAsDataURL(file)
}

function handlePdfLogoUpload(event) {
  const file = event.target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    pdfOptions.value = { ...pdfOptions.value, logoImage: String(reader.result || '') }
  }
  reader.readAsDataURL(file)
}

function clearPdfBackground() {
  pdfOptions.value = { ...pdfOptions.value, backgroundImage: '' }
}

function clearPdfLogo() {
  pdfOptions.value = { ...pdfOptions.value, logoImage: '' }
}

function clearPdfPrintMode() {
  pdfPrinting.value = false
  downloadSourcePublication.value = null
  document.body.classList.remove('bean-list-pdf-printing')
}

async function generateBeanListPdf() {
  if (!pdfGroups.value.length) return
  if (priceListLegacyPricingBlockedReason.value) {
    message.value = ''
    error.value = priceListLegacyPricingBlockedReason.value
    return
  }
  if (priceListProductSpecSelectionBlockedReason.value) {
    message.value = ''
    error.value = priceListProductSpecSelectionBlockedReason.value
    return
  }
  if (priceListTierUnitBlockedReason.value) {
    message.value = ''
    error.value = priceListTierUnitBlockedReason.value
    return
  }
  if (!customerScopeReady.value) {
    error.value = '请选择客户'
    return
  }
  beanListPdfGenerating.value = true
  error.value = ''
  message.value = ''
  const listType = pdfTheme.value.listType
  const productTypeCategoryID = activeProductTypeCategoryID.value
  try {
    const row = await apiSend('/api/costing/bean-list/drafts', { body: beanListPublicationPayload() })
    const params = beanListPublicationDownloadParams(row)
    const document = await apiSend(`/api/costing/bean-list/publications/${row.id}/pdf?${params.toString()}`)
    await downloadBeanListPublicationPDF(document)
    message.value = `已生成并下载${selectedProductPriceListLabel.value}价格表 ${row.version || '未命名版本'} PDF`
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID, 'factory_supply')
    await loadBeanListPublications(listType, versionListScope.value, productTypeCategoryID)
  } catch (err) {
    error.value = err.message || '生成价格表 PDF 失败'
  } finally {
    beanListPdfGenerating.value = false
  }
}

async function publishBeanList() {
  const blockedReason = priceListPublishBlockedReason.value
  if (blockedReason) {
    message.value = ''
    error.value = blockedReason
    return
  }
  beanListPublishing.value = true
  error.value = ''
  message.value = ''
  const listType = pdfTheme.value.listType
  const productTypeCategoryID = activeProductTypeCategoryID.value
  try {
    const row = await apiSend('/api/costing/bean-list/publications', { body: beanListPublicationPayload() })
    message.value = publicationScope.value === 'official'
      ? `已发布${selectedProductPriceListLabel.value}商品价格表 ${row.version}，客户访问链接已生成`
      : `已发布${selectedProductPriceListLabel.value}客户商品价格表 ${row.version}，内容和价格已锁定为快照`
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID, 'factory_supply')
  } catch (err) {
    error.value = err.message || '发布价格表失败'
  } finally {
    beanListPublishing.value = false
  }
}

async function saveBeanListDraft() {
  if (!pdfGroups.value.length) return
  if (priceListLegacyPricingBlockedReason.value) {
    message.value = ''
    error.value = priceListLegacyPricingBlockedReason.value
    return
  }
  if (priceListProductSpecSelectionBlockedReason.value) {
    message.value = ''
    error.value = priceListProductSpecSelectionBlockedReason.value
    return
  }
  if (priceListTierUnitBlockedReason.value) {
    message.value = ''
    error.value = priceListTierUnitBlockedReason.value
    return
  }
  if (!customerScopeReady.value) {
    error.value = '请选择客户'
    return
  }
  beanListPublishing.value = true
  error.value = ''
  message.value = ''
  const listType = pdfTheme.value.listType
  const productTypeCategoryID = activeProductTypeCategoryID.value
  try {
    const row = await apiSend('/api/costing/bean-list/drafts', { body: beanListPublicationPayload() })
    message.value = `已保存${selectedProductPriceListLabel.value}价格表修改 ${row.version}，可继续生成 PDF 下载`
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID, 'factory_supply')
  } catch (err) {
    error.value = err.message || '保存豆单修改失败'
  } finally {
    beanListPublishing.value = false
  }
}

async function saveGreenBeanPriceDraft() {
  syncPublicationScopeFromPageContext()
  pdfOptions.value = { ...pdfOptions.value, listType: 'green', version: defaultBeanListVersionForScope('green', activeProductTypeCategoryID.value) }
  initializePdfDefaultsForType('green', activeProductTypeCategoryID.value)
  if (!pdfGroups.value.length) return
  if (priceListLegacyPricingBlockedReason.value) {
    message.value = ''
    error.value = priceListLegacyPricingBlockedReason.value
    return
  }
  if (priceListProductSpecSelectionBlockedReason.value) {
    message.value = ''
    error.value = priceListProductSpecSelectionBlockedReason.value
    return
  }
  if (priceListTierUnitBlockedReason.value) {
    message.value = ''
    error.value = priceListTierUnitBlockedReason.value
    return
  }
  if (!customerScopeReady.value) {
    error.value = '请选择客户'
    return
  }
  beanListPublishing.value = true
  error.value = ''
  message.value = ''
  try {
    const row = await apiSend('/api/costing/bean-list/drafts', { body: beanListPublicationPayload() })
    message.value = `已保存生豆价格草稿 ${row.version}，可继续在生成价格表中发布`
    await loadBeanListPublications('green', publicationScope.value, activeProductTypeCategoryID.value, 'factory_supply')
    await loadBeanListPublications('green', versionListScope.value, activeProductTypeCategoryID.value)
  } catch (err) {
    error.value = err.message || '保存生豆价格失败'
  } finally {
    beanListPublishing.value = false
  }
}

async function saveGreenBeanPriceDraftForSection(section) {
  selectedProductTypeCategoryID.value = Number(section?.id || 0)
  await nextTick()
  await saveGreenBeanPriceDraft()
}

function beanListPublicationPayload() {
  const listType = pdfTheme.value.listType
  const selectedProductTypeCategoryID = activeProductTypeCategoryID.value
  const productTypeCategoryID = activePublicationProductTypeCategoryID(selectedProductTypeCategoryID)
  const selectedItems = beanListItemsForType(listType, selectedProductTypeCategoryID)
  const firstClassifiedItem = selectedItems.find((item) => classificationTemplateIDOfItem(item) > 0 || classificationCategoryIDOfItem(item) > 0) || {}
  const generationSnapshot = buildPriceListGenerationSnapshot({
    defaults: priceListTemplateDefaults.value,
    groupSelections: priceListTemplateAssignments(),
    productOverrides: priceListProductOverridesForSnapshot(),
    rows: priceListFlatRows.value,
  })
  return {
    list_type: listType,
    publication_purpose: 'factory_supply',
    product_type_category_id: productTypeCategoryID,
    product_type_name: selectedProductPriceListLabel.value,
    classification_template_id: classificationTemplateIDOfItem(firstClassifiedItem) || productTypeCategoryID,
    classification_template_name: classificationTemplateNameOfItem(firstClassifiedItem) || selectedProductPriceListLabel.value,
    classification_category_id: classificationCategoryIDOfItem(firstClassifiedItem),
    classification_category_name: classificationCategoryNameOfItem(firstClassifiedItem),
    version: pdfTheme.value.version,
    scope: publicationScope.value,
    customer_id: Number(selectedBeanListCustomerID.value || 0),
    price_source_publication_id: Number(currentPriceSourcePublication.value?.id || 0),
    style_source_publication_id: Number(styleSourcePublicationIDByType.value[activePriceListTypeKey.value] || 0),
    source_version: currentPriceSourcePublication.value?.version || '',
    config: {
      ...pdfTheme.value,
      selectedProductIDs: pdfSelectedProductIDs.value,
      product_spec_selections: pdfProductSpecSelections.value,
      showCategoryNumbers: pdfOptions.value.showCategoryNumbers,
      visibleCategoryCodes: pdfVisibleCategoryCodes.value,
      customizers: pdfCustomizers.value,
      ...generationSnapshot.config,
    },
    content: {
      title: pdfTitle.value,
      subtitle: pdfSubtitle.value,
      totalItems: pdfTotalItems.value,
      groups: pdfGroups.value,
      ...generationSnapshot.content,
    },
    changelog: pdfOptions.value.changelog,
  }
}

async function copyPublicBeanListURL() {
  if (!publicBeanListURL.value) return
  try {
    await navigator.clipboard.writeText(publicBeanListURL.value)
    message.value = '客户访问链接已复制'
  } catch (err) {
    error.value = '复制失败，请手动复制客户访问链接'
  }
}

async function withdrawBeanList(row = currentBeanListPublication.value) {
  if (!row?.id) return
  beanListWithdrawing.value = true
  error.value = ''
  message.value = ''
  const listType = normalizeBeanListType(row.list_type || pdfTheme.value.listType)
  const productTypeCategoryID = activePublicationProductTypeCategoryID(row?.product_type_category_id || activeProductTypeCategoryID.value)
  try {
    const params = beanListWithdrawScopeParams(row)
    await apiSend(`/api/costing/bean-list/publications/${row.id}/withdraw?${params.toString()}`)
    message.value = `已撤回${beanListPublicationTypeLabel(row)}价格表 ${row.version}`
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID, row?.publication_purpose || 'factory_supply')
    await loadBeanListPublications(listType, versionListScope.value, productTypeCategoryID)
  } catch (err) {
    error.value = err.message || '撤回豆单失败'
  } finally {
    beanListWithdrawing.value = false
  }
}

async function archiveSelectedBeanListPublications() {
  const rows = selectedPublicationArchiveRows()
  if (!rows.length) {
    error.value = '请选择要归档的价格表版本'
    return
  }
  const first = rows[0]
  beanListArchiving.value = true
  error.value = ''
  message.value = ''
  const listType = normalizeBeanListType(first.list_type || pdfTheme.value.listType)
  const productTypeCategoryID = activePublicationProductTypeCategoryID(first?.product_type_category_id || activeProductTypeCategoryID.value)
  try {
    const params = beanListPublicationDownloadParams(first)
    await apiSend('/api/costing/bean-list/publications/archive' + `?${params.toString()}`, {
      body: { ids: rows.map((row) => Number(row.id || 0)).filter((id) => id > 0) },
    })
    setBeanListPublicationStatusInCache(rows.map((row) => Number(row.id || 0)), 'archived')
    message.value = `已归档 ${rows.length} 个价格表版本，可在归档列表移出归档`
    selectedPublicationArchiveIDs.value = []
    await reloadBeanListPublicationsAfterArchiveChange(listType, versionListScope.value, first, productTypeCategoryID, first?.publication_purpose || 'factory_supply')
  } catch (err) {
    error.value = err.message || '归档价格表失败'
  } finally {
    beanListArchiving.value = false
  }
}

async function restoreArchivedBeanListPublication(row) {
  if (!row?.id) return
  beanListArchiving.value = true
  error.value = ''
  message.value = ''
  const listType = normalizeBeanListType(row.list_type || pdfTheme.value.listType)
  const productTypeCategoryID = activePublicationProductTypeCategoryID(row?.product_type_category_id || activeProductTypeCategoryID.value)
  try {
    const params = beanListPublicationDownloadParams(row)
    await apiSend('/api/costing/bean-list/publications/unarchive' + `?${params.toString()}`, {
      body: { ids: [Number(row.id || 0)] },
    })
    setBeanListPublicationStatusInCache([Number(row.id || 0)], beanListPublicationArchivedFromStatus(row))
    message.value = `已将价格表 ${row.version || row.id} 移出归档`
    await reloadBeanListPublicationsAfterArchiveChange(listType, versionListScope.value, row, productTypeCategoryID, row?.publication_purpose || 'factory_supply')
  } catch (err) {
    error.value = err.message || '移出归档失败'
  } finally {
    beanListArchiving.value = false
  }
}

function beanListWithdrawScopeParams(row) {
  const publicationPurpose = row?.publication_purpose || FACTORY_SUPPLY_PUBLICATION_PURPOSE
  if (row?.owner_type === 'customer') {
    return new URLSearchParams({ scope: 'customer', customer_id: String(row.owner_key || selectedBeanListCustomerID.value || 0), publication_purpose: publicationPurpose })
  }
  if (row?.owner_type === 'actor') {
    return new URLSearchParams({ scope: 'mine', publication_purpose: publicationPurpose })
  }
  const scope = publicationScope.value === 'mine' ? 'mine' : 'official'
  return new URLSearchParams({ scope, publication_purpose: publicationPurpose })
}

onMounted(() => {
  loadCurrentActor()
  loadBeanList()
  loadCustomers()
  loadCustomerProductAliases()
  loadPriceListTemplateOptions()
  loadBeanListPublications(pdfTheme.value.listType, 'official', activeProductTypeCategoryID.value, 'factory_supply')
  loadBeanListPublications(pdfTheme.value.listType, 'mine', activeProductTypeCategoryID.value, 'factory_supply')
  window.addEventListener('afterprint', clearPdfPrintMode)
})

onBeforeUnmount(() => {
  window.removeEventListener('afterprint', clearPdfPrintMode)
  clearPdfPrintMode()
})
</script>

<style scoped>
.page { padding: 18px; color: #171717; display: grid; gap: 16px; }
.panel { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fff; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.section-bar, .bean-list-generate-bar { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.bean-list-generate-bar p { margin: 4px 0 0; }
.price-list-page-config { display: grid; grid-template-columns: minmax(0, 1fr); min-width: 0; gap: 12px; }
.price-list-page-config .pdf-picker:first-child { margin-top: 0; }
.bean-list-version-panel { display: grid; gap: 12px; }
.bean-list-version-head { align-items: center; }
.bean-list-version-title-row { display: flex; align-items: center; gap: 8px; min-width: 0; }
.publication-list-collapse-toggle { width: 34px; height: 34px; display: inline-flex; align-items: center; justify-content: center; flex: 0 0 auto; border: 1px solid #cfc8bf; border-radius: 8px; background: #fff; color: #222; font: inherit; font-size: 18px; line-height: 1; cursor: pointer; }
.publication-list-collapse-toggle:hover { border-color: #888; background: #f7f7f7; }
.publication-list-collapse-toggle:focus-visible { outline: 2px solid #111; outline-offset: 2px; }
.version-controls { display: grid; grid-template-columns: minmax(110px, .55fr) minmax(170px, .9fr) repeat(3, minmax(90px, .45fr)); gap: 10px; align-items: end; }
.version-controls label span, .version-summary span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.version-controls input, .version-controls select { width: 100%; height: 38px; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
.version-control-customer { grid-column: span 2; }
.version-summary { min-height: 38px; border: 1px solid #eee; border-radius: 8px; background: #fafafa; padding: 8px 10px; box-sizing: border-box; }
.version-summary strong { display: block; overflow-wrap: anywhere; font-size: 14px; line-height: 1.2; }
.version-bulk-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; border: 1px dashed #ddd; border-radius: 8px; background: #fcfcfc; padding: 8px 10px; }
.table-select-all { display: inline-flex; align-items: center; gap: 6px; color: #333; font-size: 13px; font-weight: 650; }
.version-archive-panel { display: grid; gap: 10px; border-top: 1px solid #eee; padding-top: 12px; }
.version-table-wrap { overflow: auto; }
.version-table { min-width: 1040px; }
.version-table th, .version-table td { text-align: left; white-space: normal; vertical-align: top; }
.version-table .select-col { width: 56px; text-align: center; white-space: nowrap; }
.version-table .select-col input { width: 18px; height: 18px; }
.version-main strong { display: block; line-height: 1.25; }
.version-main small { display: block; margin-top: 3px; color: #777; font-size: 12px; }
.version-note { max-width: 260px; line-height: 1.45; color: #333; overflow-wrap: anywhere; }
.version-actions { display: flex; flex-wrap: wrap; gap: 6px; justify-content: flex-start; }
.status-pill { display: inline-flex; align-items: center; border-radius: 999px; border: 1px solid #d4d4d4; padding: 3px 8px; font-size: 12px; font-weight: 700; line-height: 1.2; white-space: nowrap; }
.status-published { border-color: #9fd0a4; background: #ecfaee; color: #176528; }
.status-draft { border-color: #c8d4e1; background: #f7fbff; color: #1f4f82; }
.status-withdrawn { border-color: #e0b4b4; background: #fff1f1; color: #8b1e1e; }
.status-archived { border-color: #c8c8c8; background: #f4f4f4; color: #666; }
.status-unknown { background: #f5f5f5; color: #555; }
.price-list-top-toolbar { display: grid; grid-template-columns: minmax(120px, .35fr) minmax(260px, 1fr) auto; align-items: end; gap: 12px; }
.price-list-toolbar-stat, .price-list-toolbar-scope { min-height: 62px; border: 1px solid #eee; border-radius: 8px; background: #fafafa; padding: 10px 12px; box-sizing: border-box; }
.price-list-toolbar-stat span, .price-list-toolbar-scope span { display: block; margin-bottom: 5px; color: #666; font-size: 12px; }
.price-list-toolbar-stat strong, .price-list-toolbar-scope strong { display: block; font-size: 18px; line-height: 1.2; }
.price-list-toolbar-scope { margin: 0; }
.price-list-toolbar-scope select { width: 100%; height: 38px; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
.price-list-tier-template-button { min-height: 38px; align-self: center; }
.table-wrap { overflow: auto; margin-top: 10px; }
table { width: 100%; border-collapse: collapse; min-width: 1100px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 9px 10px; text-align: right; white-space: nowrap; }
th:first-child, td:first-child { text-align: left; }
th { color: #555; background: #fafafa; font-weight: 700; }
.name { font-weight: 650; }
.item-warning-list, .bean-warning-list { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 6px; }
.warning-icon-wrap { position: relative; display: inline-flex; align-items: center; }
.warning-icon { width: 22px; height: 22px; min-width: 22px; border: 1px solid #e8c28f; border-radius: 999px; background: #fff8eb; color: #8a4b00; padding: 0; font-size: 14px; font-weight: 800; line-height: 20px; text-align: center; }
.warning-tooltip { position: absolute; left: 0; bottom: calc(100% + 7px); z-index: 10; display: none; width: min(320px, 72vw); border: 1px solid #e8c28f; border-radius: 8px; background: #fff8eb; color: #5f3400; padding: 8px 10px; font-size: 12px; font-weight: 600; line-height: 1.45; box-shadow: 0 8px 24px rgba(0,0,0,.12); white-space: normal; }
.warning-icon-wrap:hover .warning-tooltip,
.warning-icon:focus + .warning-tooltip { display: block; }
.tiers-cell { min-width: 360px; text-align: left; white-space: normal; }
.tier-list { display: flex; flex-wrap: wrap; gap: 6px; justify-content: flex-start; }
.tier-chip { border: 1px solid #ddd; border-radius: 8px; background: #fff; padding: 5px 7px; color: #222; font-size: 12px; line-height: 1.2; }
.tier-chip b { margin-right: 5px; font-weight: 700; color: #111; }
.source-button { min-height: 22px; margin-left: 6px; border: 1px solid #c8d4e1; border-radius: 6px; background: #f7fbff; color: #1f4f82; padding: 0 6px; font-size: 11px; line-height: 20px; vertical-align: middle; }
.empty { text-align: center !important; padding: 18px; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; font: inherit; }
button:disabled { opacity: .45; cursor: not-allowed; }
.compact { padding: 5px 8px; font-size: 12px; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.danger { border: 1px solid #8b1e1e; background: #8b1e1e; color: #fff; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.warning-banner { border: 1px solid #e8c28f; border-radius: 8px; background: #fff8eb; color: #8a4b00; padding: 10px; margin-bottom: 12px; }
.drawer-backdrop { position: fixed; inset: 0; z-index: 80; background: rgba(0,0,0,.25); display: flex; justify-content: flex-end; }
.settings-drawer { width: min(620px, 100vw); height: 100vh; overflow: auto; background: #f7f7f7; border-left: 1px solid #d9d9d9; padding: 14px; box-shadow: -18px 0 36px rgba(0,0,0,.18); }
.explanation-drawer { width: min(680px, 100vw); }
.drawer-head { position: sticky; top: 0; z-index: 2; background: #f7f7f7; display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; padding-bottom: 12px; margin-bottom: 4px; }
.drawer-head h3 { margin: 0; font-size: 18px; }
.drawer-head p { margin: 4px 0 0; color: #666; font-size: 12px; line-height: 1.45; }
.explanation-summary { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-bottom: 12px; }
.explanation-summary > div { border: 1px solid #ddd; border-radius: 8px; background: #fff; padding: 10px; }
.explanation-summary span { display: block; margin-bottom: 5px; color: #666; font-size: 12px; }
.explanation-summary strong { display: block; overflow-wrap: anywhere; font-size: 16px; }
.explanation-form { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)) auto; gap: 8px; align-items: end; margin: 12px 0; }
.explanation-form label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.explanation-form input { width: 100%; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
.formula-steps { display: grid; gap: 8px; margin: 12px 0; }
.formula-step { display: grid; grid-template-columns: minmax(110px, .7fr) minmax(130px, .6fr) minmax(0, 1fr); align-items: center; gap: 10px; border: 1px solid #ddd; border-radius: 8px; background: #fff; padding: 9px 10px; }
.formula-step.changed { border-color: #d29b42; background: #fff8eb; }
.formula-step span { color: #333; font-weight: 650; }
.formula-step strong { text-align: right; }
.formula-step small { min-width: 0; color: #666; overflow-wrap: anywhere; }
.publish-state { color: #333 !important; }
.public-link-box { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 8px; margin-top: 8px; border: 1px solid #ddd; border-radius: 8px; background: #fff; padding: 8px; font-size: 12px; }
.public-link-box span { color: #666; font-weight: 700; }
.public-link-box a { min-width: 0; color: #111; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.copy-config-box { display: grid; grid-template-columns: minmax(0, 1fr) minmax(280px, .8fr); align-items: end; gap: 12px; border: 1px solid #ddd; border-radius: 10px; background: #fafafa; padding: 12px; margin-bottom: 12px; }
.copy-config-box strong { display: block; margin-bottom: 4px; }
.copy-config-box p { margin: 0; color: #666; font-size: 12px; line-height: 1.45; }
.publication-context-box { align-items: center; }
.current-owner-pill { justify-self: end; min-height: 38px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid #c8d4e1; border-radius: 999px; background: #f7fbff; color: #1f4f82; padding: 0 12px; font-size: 13px; font-weight: 700; }
.copy-config-actions { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; }
.copy-config-actions select { min-width: 0; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; }
.pdf-drawer { width: min(760px, 100vw); }
.pdf-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.pdf-form label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.pdf-form input, .pdf-form select, .pdf-form textarea, .product-picker-row input, .product-picker-row select, .category-inline-pricing-config select, .category-inline-pricing-config input, .price-list-config-dialog input, .price-list-config-dialog select { width: 100%; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
.pdf-form textarea { resize: vertical; line-height: 1.45; }
.pdf-form input[type="color"] { padding: 4px; }
.pdf-form .wide, .pdf-actions { grid-column: 1 / -1; }
.pdf-actions { display: flex; justify-content: flex-end; gap: 8px; }
.check-line { display: flex; align-items: center; gap: 7px; color: #333; font-size: 12px; }
.check-line input { width: auto; min-height: auto; }
.pdf-picker { margin-top: 12px; border: 1px solid #e4e4e4; border-radius: 8px; background: #fff; padding: 10px; }
.picker-head { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.picker-actions { margin-left: auto; display: flex; gap: 6px; }
.template-default-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-bottom: 10px; }
.inline-pricing-config-note { margin: 4px 0 0; }
.template-default-grid span, .template-table span, .flat-price-table span { color: #666; font-size: 12px; line-height: 1.35; }
.template-default-grid select, .template-default-grid input, .template-table select, .template-table input, .flat-price-row input, .template-editor-grid input, .template-editor-grid select, .template-tier-row input, .template-tier-row select { width: 100%; min-height: 34px; border: 1px solid #ddd; border-radius: 7px; padding: 6px 8px; background: #fff; font: inherit; box-sizing: border-box; }
.template-editor-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.template-editor-grid span, .template-tier-row span, .wide-field span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.template-tier-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-top: 4px; }
.template-tier-list { display: grid; gap: 8px; }
.template-tier-row { display: grid; gap: 8px; align-items: end; border: 1px solid #eee; border-radius: 8px; background: #fafafa; padding: 8px; }
.wide-field textarea { width: 100%; min-height: 64px; border: 1px solid #ddd; border-radius: 7px; padding: 8px; font: inherit; box-sizing: border-box; resize: vertical; }
.template-table { display: grid; gap: 6px; margin-top: 10px; }
.template-table-head, .template-table-row { display: grid; grid-template-columns: minmax(120px, .8fr) minmax(0, 1fr) minmax(0, 1fr); gap: 8px; align-items: center; }
.template-table-head { border-bottom: 1px solid #eee; padding-bottom: 5px; font-weight: 700; }
.template-table-row { border: 1px solid #eee; border-radius: 7px; background: #fafafa; padding: 8px; }
.template-table-row strong { min-width: 0; overflow-wrap: anywhere; font-size: 13px; }
.template-select-pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
.flat-price-table { display: grid; gap: 6px; max-height: 360px; overflow: auto; }
.flat-price-head, .flat-price-row { display: grid; grid-template-columns: minmax(150px, 1.1fr) minmax(130px, .8fr) minmax(110px, .55fr) minmax(150px, .9fr); gap: 8px; align-items: center; }
.flat-price-head { border-bottom: 1px solid #eee; padding-bottom: 5px; font-weight: 700; }
.flat-price-row { border: 1px solid #eee; border-radius: 7px; background: #fafafa; padding: 8px; }
.flat-price-row.invalid { border-color: #d93025; background: #fff7f6; }
.flat-price-row > div { display: grid; gap: 3px; min-width: 0; }
.flat-price-row strong { min-width: 0; overflow-wrap: anywhere; font-size: 13px; }
.flat-price-row label { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 5px; margin: 0; }
.flat-price-row small { color: #666; font-size: 12px; }
.flat-price-row .adjusted { color: #8a4b00; font-weight: 700; }
.flat-price-row-error-list { grid-column: 1 / -1; margin: 0; padding-left: 18px; color: #b3261e; font-size: 12px; display: grid; gap: 2px; }
.flat-price-row-error-list li { overflow-wrap: anywhere; }
.checkbox-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px 10px; }
.product-picker-list { display: grid; gap: 10px; max-height: 420px; overflow: auto; }
.product-picker-category { display: grid; gap: 8px; margin-left: var(--product-picker-category-indent, 0); border: 1px solid #ddd; border-radius: 8px; padding: 10px; background: #fff; }
.product-picker-category-head { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 8px 10px; align-items: start; padding-bottom: 8px; border-bottom: 1px solid #eee; }
.category-collapse-toggle { min-width: 54px; justify-self: start; }
.product-picker-category.collapsed .product-picker-category-head { padding-bottom: 0; border-bottom: 0; }
.category-pricing-summary, .product-compact-status { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 6px; min-width: 0; }
.category-pricing-summary { margin-left: calc(54px + 10px); }
.price-list-summary-button { min-height: 34px; border: 1px solid #ddd; border-radius: 7px; background: #fff; color: inherit; padding: 5px 8px; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 6px; text-align: left; cursor: pointer; min-width: min(100%, 240px); max-width: 100%; }
.price-list-summary-button span { color: #666; font-size: 12px; line-height: 1.2; }
.price-list-summary-button strong { color: #222; font-size: 12px; line-height: 1.25; font-weight: 700; overflow-wrap: anywhere; }
.price-list-summary-button small { border: 1px solid #f0d6a8; border-radius: 999px; background: #fff7e8; color: #8a4b00; padding: 2px 6px; font-size: 11px; font-weight: 700; line-height: 1.1; white-space: nowrap; }
.price-list-summary-button.active { border-color: #86afe8; background: #f5f9ff; }
.price-list-summary-button.overridden:not(.active) { border-color: #f0d6a8; background: #fffaf0; }
.price-list-pricing-popover { flex: 1 1 340px; min-width: min(100%, 280px); max-width: min(100%, 520px); border: 1px solid #d9d9d9; border-radius: 8px; background: #fff; box-shadow: 0 12px 32px rgba(0,0,0,.14); padding: 10px; display: grid; gap: 9px; }
.price-list-pricing-popover-title { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.price-list-pricing-popover-title strong { min-width: 0; overflow-wrap: anywhere; font-size: 13px; }
.price-list-pricing-popover-context { display: grid; gap: 2px; min-width: 0; }
.price-list-pricing-popover-context span { color: #666; font-size: 12px; overflow-wrap: anywhere; }
.price-list-pricing-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
.price-list-pricing-options button { min-height: 34px; border: 1px solid #ddd; border-radius: 7px; background: #fff; padding: 6px 8px; font: inherit; text-align: left; cursor: pointer; }
.price-list-pricing-options button.active { border-color: #111; background: #111; color: #fff; }
.price-list-pricing-popover select,
.price-list-pricing-popover input { width: 100%; min-height: 34px; border: 1px solid #ddd; border-radius: 7px; padding: 6px 8px; background: #fff; font: inherit; box-sizing: border-box; }
.price-list-config-dialog-backdrop { position: fixed; inset: 0; z-index: 120; display: flex; justify-content: flex-end; align-items: flex-end; background: rgba(0,0,0,.18); padding: 14px; box-sizing: border-box; }
.price-list-config-dialog { width: min(520px, calc(100vw - 28px)); max-height: min(70vh, 560px); overflow: auto; border: 1px solid #d9d9d9; border-radius: 8px; background: #fff; box-shadow: 0 18px 44px rgba(0,0,0,.22); padding: 12px; display: grid; gap: 10px; }
.price-list-config-dialog-head { display: flex; justify-content: space-between; align-items: center; gap: 10px; }
.price-list-config-dialog-head strong { font-size: 15px; }
.price-list-config-dialog .inline-price-config { border: 1px solid #eee; border-radius: 8px; background: #fafafa; padding: 9px; }
.price-list-config-dialog .inline-price-config-controls { display: grid; gap: 7px; }
.price-list-rules-dialog { width: min(720px, calc(100vw - 28px)); }
.price-list-rule-table { width: 100%; min-width: 0; border-collapse: collapse; background: #fff; font-size: 13px; }
.price-list-rule-table th, .price-list-rule-table td { border: 1px solid #eee; padding: 8px 10px; text-align: left; vertical-align: top; white-space: normal; }
.price-list-rule-table th { background: #f7f7f7; color: #555; font-weight: 650; }
.price-list-rule-table td:first-child { width: 110px; font-weight: 650; color: #111; }
.category-inline-pricing-config { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.inline-price-config { display: grid; gap: 5px; margin: 0; min-width: 0; }
.inline-price-config > span, .product-inline-pricing-config > span { color: #666; font-size: 12px; line-height: 1.35; }
.inline-price-config-controls { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; min-width: 0; }
.product-picker-row { display: grid; gap: 7px; margin-left: var(--product-picker-row-indent, 0); border: 1px solid #eee; border-radius: 8px; padding: 9px; background: #fafafa; }
.product-picker-row-head { display: grid; gap: 7px; min-width: 0; }
.product-spec-options { display: flex; flex-wrap: wrap; align-items: flex-start; gap: 7px; padding-left: 18px; }
.product-spec-option { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; min-width: 0; border: 1px solid #e5e5e5; border-radius: 8px; padding: 6px 8px; background: #fff; }
.product-spec-option.selected { border-color: #9fc2f6; background: #f7faff; }
.product-spec-selection-warning { display: flex; align-items: center; justify-content: space-between; gap: 10px; border: 1px solid #d89a2b; border-radius: 8px; background: #fff8e8; color: #7b4e00; padding: 9px 10px; }
.product-spec-selection-warning-actions { display: flex; gap: 6px; flex-wrap: wrap; flex: none; }
.product-spec-check { min-width: 0; }
.product-spec-check span { min-width: 0; overflow-wrap: anywhere; font-weight: 600; }
.product-spec-check small { border-radius: 999px; padding: 2px 7px; background: #e8f1ff; color: #205da8; white-space: nowrap; }
.parent-product-fixed-prices { display: grid; gap: 8px; margin-top: 10px; }
.parent-product-fixed-prices .inline-price-config { grid-template-columns: minmax(130px, 1fr) minmax(120px, 180px); align-items: center; }
.price-list-legacy-pricing-warning { display: grid; gap: 4px; margin-top: 10px; }
.product-spec-option .product-picker-tier-warning { flex: 1 0 100%; margin-top: 0; }
.product-picker-bom-warning { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; border: 1px solid #f0b7b7; border-radius: 8px; background: #fff1f1; color: #7d1616; padding: 8px 10px; font-size: 12px; line-height: 1.45; }
.product-picker-bom-warning div { display: grid; gap: 2px; min-width: 0; }
.product-picker-bom-warning strong { font-size: 13px; color: #5f0f0f; }
.product-picker-bom-warning button { flex-shrink: 0; }
.product-picker-tier-warning { margin-top: 8px; border: 1px solid #d93025; border-radius: 8px; background: #fff7f6; color: #a61b12; padding: 8px 10px; font-size: 12px; line-height: 1.45; }
.inline-tier-template-warning { margin: 0; font-size: 12px; line-height: 1.45; }
.product-inline-pricing-config { display: grid; grid-template-columns: minmax(82px, .45fr) minmax(0, .8fr) minmax(0, .8fr); gap: 6px; align-items: center; min-width: 0; }
.customizer-row { display: grid; grid-template-columns: 120px minmax(0, 1fr); gap: 7px; }
.green-tier-price-editor { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 7px; }
.green-tier-price-editor label { display: grid; grid-template-columns: minmax(0, 1fr) 82px; align-items: center; gap: 6px; font-size: 12px; color: #555; }
.green-tier-price-editor input { min-width: 0; }
.price-list-preview { grid-column: 1 / -1; display: grid; gap: 8px; width: 100%; min-width: 0; }
.pdf-preview-title { width: 100%; max-width: none; margin: 16px 0 0; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; color: #555; font-size: 12px; box-sizing: border-box; }
.pdf-preview-title strong { color: #111; font-size: 14px; }
.price-list-publish-feedback { flex-basis: 100%; margin: -4px 0 0; }
.pdf-preview-phone { max-width: 430px; min-height: 360px; max-height: 72vh; overflow: auto; margin: 0 auto; border: 1px solid #ded6c9; border-radius: 8px; box-shadow: 0 10px 28px rgba(0,0,0,.12); }
.price-list-preview .pdf-preview-phone { width: 100%; max-width: none; margin: 0; box-sizing: border-box; }
.bean-list-pdf-surface { box-sizing: border-box; padding: 16px; background-size: cover; background-position: center; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
.pdf-cover { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; border-bottom: 2px solid currentColor; padding-bottom: 12px; margin-bottom: 14px; }
.pdf-cover h1 { margin: 2px 0 6px; font-size: 26px; line-height: 1.12; letter-spacing: 0; }
.pdf-cover p { margin: 0; font-size: 12px; line-height: 1.45; }
.pdf-version { color: inherit; opacity: .72; }
.pdf-logo { max-width: 96px; max-height: 44px; object-fit: contain; display: block; margin-bottom: 5px; }
.pdf-brand-intro { max-width: 330px; margin-top: 6px !important; white-space: pre-line; }
.pdf-changelog { display: grid; grid-template-columns: 38px 1fr; gap: 7px; border: 1px solid rgba(0,0,0,.12); border-radius: 8px; padding: 8px; margin-bottom: 12px; background: rgba(255,255,255,.62); font-size: 12px; line-height: 1.45; white-space: pre-line; }
.pdf-bottom-changelog { margin-top: 14px; margin-bottom: 8px; }
.pdf-badge { border: 1px solid currentColor; border-radius: 999px; padding: 4px 9px; font-size: 12px; white-space: nowrap; }
.pdf-group { margin: 14px 0; }
.pdf-group h2 { margin: 0 0 8px; padding: 7px 9px; background: rgba(255,255,255,.62); border-left: 4px solid currentColor; font-size: 15px; line-height: 1.25; }
.pdf-card-grid { display: grid; gap: 18px; }
.pdf-card-row { display: grid; column-gap: 9px; row-gap: 0; align-items: stretch; grid-template-rows: auto auto auto; }
.pdf-card-row > .pdf-item { min-width: 0; height: auto; display: grid; grid-template-rows: subgrid; grid-row: span 3; align-content: start; }
.pdf-item { break-inside: avoid; page-break-inside: avoid; border: 1px solid rgba(0,0,0,.16); border-radius: 8px; padding: 10px; margin-bottom: 0; background: rgba(255,255,255,.76); }
.pdf-item-head { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 8px; align-items: start; }
.pdf-item-head > div { min-width: 0; }
.pdf-item-head > span { min-width: 32px; height: 26px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid currentColor; border-radius: 6px; font-size: 12px; font-weight: 700; }
.pdf-item h3 { margin: 0; font-size: 20px; line-height: 1.18; letter-spacing: 0; overflow-wrap: anywhere; word-break: break-word; }
.pdf-meta-block { min-width: 0; display: grid; gap: 4px; align-content: start; margin-top: 4px; }
.pdf-meta-line { margin: 0; font-size: 12px; line-height: 1.45; white-space: pre-line; }
.pdf-flavor, .pdf-desc { display: grid; grid-template-columns: 44px 1fr; gap: 6px; margin: 7px 0 0; font-size: 12px; line-height: 1.45; }
.pdf-card-row.cards-2 .pdf-flavor, .pdf-card-row.cards-3 .pdf-flavor { min-height: 44px; }
.pdf-card-row.cards-2 .pdf-desc, .pdf-card-row.cards-3 .pdf-desc { min-height: 62px; }
.pdf-flavor { font-weight: 650; }
.pdf-desc { opacity: .82; }
.pdf-meta-line b, .pdf-flavor b, .pdf-desc b, .pdf-section-label { color: inherit; opacity: .62; font-weight: 650; }
.pdf-meta-line b { margin-right: 6px; }
.pdf-price-block { margin-top: 8px; padding-top: 0; align-self: start; }
.pdf-section-label { margin-bottom: 4px; font-size: 12px; }
.pdf-price-list { display: grid; grid-template-columns: 1fr; gap: 6px; }
.pdf-card-row.cards-1 .pdf-price-list { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.pdf-price { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 8px; min-height: 34px; border: 1px solid rgba(0,0,0,.12); border-radius: 6px; padding: 5px 7px; background: #dff5d9; font-size: 12px; }
.pdf-price:nth-child(even) { background: #dbeaf7; }
.pdf-price-label { min-width: 0; line-height: 1.25; overflow-wrap: anywhere; }
.pdf-price-value { justify-self: end; white-space: nowrap; font-size: 15px; line-height: 1.15; text-align: right; }
.pdf-red { color: #c51616 !important; font-weight: 750; }
.pdf-product-badge { display: inline-flex; align-items: center; justify-content: center; margin-left: 6px; border-radius: 999px; border: 1px solid currentColor; padding: 1px 5px; font-size: 11px; line-height: 1.2; vertical-align: middle; }
.badge-new { color: #c51616; }
.badge-thumb { color: #0b6bcb; }
.badge-medal { color: #8a6200; }
.pdf-compact-table { width: 100%; min-width: 0; border-collapse: collapse; font-size: 10px; background: rgba(255,255,255,.72); }
.pdf-compact-table td { border: 1px solid rgba(0,0,0,.16); padding: 5px 6px; white-space: normal; vertical-align: top; text-align: left; }
.pdf-code-cell { width: 32px; font-weight: 800; text-align: center !important; white-space: nowrap !important; }
.pdf-table-name { font-weight: 800; font-size: 12px; line-height: 1.25; }
.pdf-table-line { margin-top: 3px; color: #444; line-height: 1.35; }
.pdf-table-line b { margin-right: 4px; color: #777; }
.pdf-table-prices { width: 108px; }
.pdf-table-prices div { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 5px; line-height: 1.35; }
.pdf-table-prices strong { font-size: 11px; white-space: nowrap; }
.pdf-footer { display: flex; justify-content: space-between; gap: 12px; border-top: 1px solid currentColor; padding-top: 10px; margin-top: 16px; font-size: 11px; }
.bean-groups { display: grid; gap: 14px; margin-top: 10px; }
.bean-group { display: grid; gap: 8px; }
.bean-group h3 { margin: 0; padding: 8px 10px; border-left: 4px solid #111; background: #f5f5f5; font-size: 15px; }
.bean-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
article, .empty-card { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fafafa; }
.bean-heading { display: grid; grid-template-columns: auto 1fr; gap: 8px; align-items: start; min-height: 52px; margin-bottom: 8px; }
.bean-code { display: inline-flex; align-items: center; justify-content: center; min-width: 34px; height: 28px; border: 1px solid #ddd; border-radius: 8px; background: #fff; font-size: 12px; font-weight: 700; color: #333; }
.bean-title { font-weight: 700; line-height: 1.25; }
.bean-use { color: #111; font-size: 12px; line-height: 1.35; margin-top: 3px; white-space: pre-line; }
.bean-attrs { display: flex; flex-wrap: wrap; gap: 6px; margin: 8px 0 4px; }
.bean-attrs span { border: 1px solid #e7dfd5; border-radius: 999px; background: #fff; color: #5f554b; font-size: 12px; line-height: 1.3; padding: 4px 8px; }
.bean-note { color: #555; font-size: 12px; min-height: 18px; margin: 0 0 8px; line-height: 1.45; }
.bean-desc { color: #777; font-size: 12px; line-height: 1.45; margin: 0 0 8px; }
.bean-row { display: flex; justify-content: space-between; align-items: center; gap: 8px; border-top: 1px solid #eee; padding: 7px 0; }
.bean-row span { color: #666; font-size: 12px; }
.bean-row strong { font-size: 13px; }
.bean-list-pdf-page { display: none; }
.generate-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
.tier-template-drawer { width: min(1040px, 96vw); }
.tier-template-drawer-body { display: grid; grid-template-columns: minmax(220px, .75fr) minmax(0, 1.6fr); gap: 14px; align-items: start; }
.tier-template-list { display: grid; grid-template-columns: minmax(0, 1fr); min-width: 0; gap: 8px; border: 1px solid #eee; border-radius: 8px; padding: 10px; background: #fafafa; }
.tier-template-list-row { display: grid; grid-template-columns: minmax(0, 1fr); gap: 3px; width: 100%; min-width: 0; box-sizing: border-box; text-align: left; white-space: normal; border: 1px solid #eee; border-radius: 8px; padding: 9px 10px; background: #fff; }
.tier-template-list-row strong, .tier-template-list-row small { display: block; min-width: 0; white-space: normal; overflow-wrap: anywhere; }
.tier-template-list-row.active { border-color: #111; }
.tier-template-form { display: grid; gap: 10px; }
.price-list-tier-template-row { grid-template-columns: minmax(100px, .8fr) minmax(90px, .65fr) minmax(90px, .65fr) minmax(180px, 1.2fr) auto; }
.template-select-pair input { width: 100%; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: flex-start; flex-direction: column; }
  .actions { justify-content: flex-start; }
  .price-list-top-toolbar { grid-template-columns: 1fr; align-items: stretch; }
  .price-list-tier-template-button { justify-self: start; }
  .bean-grid { grid-template-columns: 1fr; }
  .settings-drawer { width: 100vw; }
  .explanation-summary, .formula-step { grid-template-columns: 1fr; }
  .explanation-form { grid-template-columns: 1fr; }
  .formula-step strong { text-align: left; }
  .section-bar.bean-list-version-head { align-items: flex-start; flex-direction: column; }
  .version-controls { grid-template-columns: 1fr; }
  .version-control-customer { grid-column: auto; }
  .copy-config-box, .copy-config-actions { grid-template-columns: 1fr; }
  .current-owner-pill { justify-self: start; }
  .pdf-form { grid-template-columns: 1fr; }
  .template-default-grid,
  .category-inline-pricing-config,
  .inline-price-config-controls,
  .product-inline-pricing-config,
  .price-list-pricing-options,
  .template-table-head,
  .template-table-row,
  .template-select-pair,
  .tier-template-drawer-body,
  .price-list-tier-template-row,
  .flat-price-head,
  .flat-price-row { grid-template-columns: 1fr; }
  .product-picker-bom-warning { flex-direction: column; }
  .product-spec-options { padding-left: 0; }
  .parent-product-fixed-prices .inline-price-config { grid-template-columns: 1fr; }
  .generate-actions { justify-content: flex-start; }
  .checkbox-grid, .customizer-row { grid-template-columns: 1fr; }
  .category-pricing-summary { margin-left: 0; }
  .bean-list-generate-bar { align-items: flex-start; flex-direction: column; }
}

@media print {
  @page { size: 108mm 192mm; margin: 0; }
  :global(body.bean-list-pdf-printing #app) { display: none !important; }
  :global(body.bean-list-pdf-printing) { background: #fff !important; }
  :global(body.bean-list-pdf-printing .bean-list-pdf-page) { display: block !important; }
  .bean-list-pdf-page {
    display: block;
    width: 100%;
    max-width: 108mm;
    min-height: 192mm;
    margin: 0 auto;
  }
  .pdf-group { break-inside: auto; page-break-inside: auto; }
  .pdf-item { break-inside: avoid; page-break-inside: avoid; }
}
</style>
