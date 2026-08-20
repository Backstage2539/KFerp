<template>
  <div class="page">
    <div v-if="productReturnNavigation" class="product-return-banner">
      <button class="secondary product-return-button" type="button" @click="returnToPreviousView">{{ productReturnLabel }}</button>
      <span>完成商品档案配置后可回到来源操作界面。</span>
    </div>
    <section class="panel sku-page-summary">
      <div class="panel-head">
        <div>
          <h2>{{ productSectionTitle }}</h2>
          <p>商品档案维护商品族信息、行业字段、客户引用和 BOM 使用摘要；已切换商品的规格、库存单位和配方统一由默认已发布 BOM 提供，旧销售规格模板与派生子 SKU 仅供尚未切换的商品过渡使用。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading || productCategoryMoveActive">刷新</button>
      </div>
    </section>

    <section class="settings-workbench">
      <div v-if="false" class="sku-category-management-workspace" data-section-mode="groupTemplatesRetired">
        <section class="panel category-management-panel">
          <div class="panel-title sku-panel-title">
            <span>模板分类维护已迁移</span>
            <div class="filter-actions">
              <button class="secondary compact-action" type="button" @click="createPrimaryCategoryInline">新增大类</button>
              <button
                class="secondary compact-action danger-outline"
                type="button"
                :class="{ active: primaryDeleteMode }"
                @click="togglePrimaryDeleteMode">
                {{ primaryDeleteMode ? '退出停用大类' : '停用大类' }}
              </button>
            </div>
          </div>
          <p class="muted">分组是泛化主数据能力，可被商品档案归类、商品价格表选品、物料档案归类、生产 BOM 分组和报表分组复用。分组项不参与 BOM、库存、销售单位；商品价格表引用分组时只用于选品、筛选、分组展示和模板继承。</p>
          <label class="category-search">
            <span>搜索分类/商品</span>
            <input v-model.trim="categorySearchQuery" placeholder="搜索大类、小类或商品名称" />
          </label>
          <div class="category-scroll-list">
            <div class="category-tree">
              <article
                v-for="primary in visibleCategoryManagementTreeForSkuContext"
                :key="primary.id"
                :data-primary-id="primary.id"
                :class="['primary-category', { collapsed: isPrimaryCategoryCollapsed(primary) }]"
                @dragover.prevent="handlePrimaryCategoryDragOver($event, primary)"
                @drop.prevent="dropCategoryOnCurrentTarget(primary)">
                <div class="category-head primary-category-head">
                  <div class="primary-category-left">
                    <button class="category-collapse-button" type="button" @click="togglePrimaryCategoryCollapse(primary)">
                      {{ isPrimaryCategoryCollapsed(primary) ? '展开' : '收起' }}
                    </button>
                    <form v-if="editingCategoryId === Number(primary.id)" class="category-name-form" @submit.prevent="saveCategoryName(primary)">
                      <input v-model.trim="editingCategoryName" />
                      <div class="form-actions">
                        <button class="primary compact-action" type="submit">保存</button>
                        <button class="secondary compact-action" type="button" @click="cancelCategoryNameEdit">取消</button>
                      </div>
                    </form>
                    <button v-else class="category-name-button primary-name-button" type="button" :disabled="!canEditCategory(primary)" @click="startCategoryEdit(primary)">
                      <strong>{{ primary.name }} <small>{{ primary.children?.length || 0 }} 个小类</small></strong>
                    </button>
                  </div>
                  <div class="category-row-actions primary-category-right">
                    <span class="category-sort-pill">
                      <button class="category-action-button compact" type="button" :disabled="isFirstPrimaryCategory(primary) || isCategorySearchActive || !canEditCategory(primary)" @click="movePrimaryCategory(primary, -1)">↑</button>
                      <button class="category-action-button compact" type="button" :disabled="isLastPrimaryCategory(primary) || isCategorySearchActive || !canEditCategory(primary)" @click="movePrimaryCategory(primary, 1)">↓</button>
                    </span>
                    <button class="category-action-button" type="button" :disabled="!canEditCategory(primary)" @click="createSecondaryCategoryInline(primary)">+</button>
                    <button v-if="primaryDeleteMode" class="category-delete-button" type="button" :disabled="!canEditCategory(primary)" @click="deleteCategory(primary)">×</button>
                    <button class="category-action-button danger-toggle" type="button" :class="{ active: secondaryDeleteModeFor === Number(primary.id) }" :disabled="!canEditCategory(primary)" @click="toggleSecondaryDeleteMode(primary)">-</button>
                  </div>
                </div>
                <div v-if="!isPrimaryCategoryCollapsed(primary)" class="category-children">
                  <div v-if="primary.products?.length" class="uncategorized">
                    <span class="muted">未分到小类</span>
                    <button
                      v-for="product in primary.products"
                      :key="`primary-product-${primary.id}-${product.id}`"
                      class="product-chip"
                      type="button"
                      :draggable="canDragSkuRow(product)"
                      @dragstart="startProductDrag(product)"
                      @dragend="scheduleClearDrag">
                      {{ product.name || product.sku || product.id }}
                    </button>
                  </div>
                  <template v-for="(secondary, index) in primary.children" :key="secondary.id">
                    <div :class="['category-drop-line', { active: isCategoryDropTarget(primary, index + 1) }]"></div>
                    <div
                      :data-secondary-id="secondary.id"
                      :class="['secondary-category', { dragging: isDraggingCategory(secondary), 'pointer-dragging': isPointerDraggingCategory(secondary) }]"
                      :draggable="canEditCategory(secondary) && !isCategorySearchActive"
                      @pointerdown="startCategoryPointerDrag($event, primary, index + 1, secondary)"
                      @dragstart="startCategoryDrag(secondary)"
                      @dragover.prevent="handleSecondaryCategoryDragOver($event, primary, index + 1)"
                      @drop.prevent="dropCategoryOrProductOnSecondary(primary, index + 1, secondary)"
                      @dragend="scheduleClearDrag">
                      <div class="secondary-head">
                        <button class="category-collapse-button secondary-collapse" type="button" @click="toggleSecondaryCategoryCollapse(secondary)">
                          {{ isSecondaryCategoryCollapsed(secondary) ? '+' : '-' }}
                        </button>
                        <form v-if="editingCategoryId === Number(secondary.id)" class="category-name-form secondary-name-form" @submit.prevent="saveCategoryName(secondary)">
                          <input v-model.trim="editingCategoryName" />
                          <div class="form-actions">
                            <button class="primary compact-action" type="submit">保存</button>
                            <button class="secondary compact-action" type="button" @click="cancelCategoryNameEdit">取消</button>
                          </div>
                        </form>
                        <button v-else class="category-name-button secondary-name-button" type="button" :disabled="!canEditCategory(secondary)" @click="startCategoryEdit(secondary)">
                          <b>{{ secondary.name }}</b>
                          <small>{{ secondary.products?.length || 0 }} 款商品</small>
                        </button>
                        <div class="secondary-category-actions">
                          <button v-if="secondaryDeleteModeFor === Number(primary.id)" class="category-delete-button" type="button" :disabled="!canEditCategory(secondary)" @click="deleteCategory(secondary)">×</button>
                        </div>
                      </div>
                      <div v-if="!isSecondaryCategoryCollapsed(secondary)" class="product-chip-list">
                        <button
                          v-for="product in secondary.products"
                          :key="`secondary-product-${secondary.id}-${product.id}`"
                          class="product-chip"
                          type="button"
                          :draggable="canDragSkuRow(product)"
                          @dragstart="startProductDrag(product)"
                          @dragend="scheduleClearDrag">
                          {{ product.name || product.sku || product.id }}
                        </button>
                        <span v-if="!secondary.products?.length" class="muted">暂无商品</span>
                      </div>
                    </div>
                  </template>
                  <div :class="['category-drop-line', { active: isCategoryDropTarget(primary, (primary.children?.length || 0) + 1) }]"></div>
                </div>
              </article>
              <p v-if="!visibleCategoryManagementTreeForSkuContext.length" class="muted">暂无商品分类。</p>
            </div>
          </div>
        </section>
      </div>

      <div v-show="currentSettingsSection === 'master'" class="sku-master-workspace" data-section-mode="productMaster">
        <div class="master-data-layout">
      <div class="panel product-panel">
        <div class="panel-title sku-panel-title">
          <span>商品档案 · {{ selectedSkuContextLabel }}</span>
          <button class="toggle-section" type="button" @click="productsCollapsed = !productsCollapsed">
            {{ productsCollapsed ? '展开' : '收起' }}
          </button>
        </div>
        <div v-show="!productsCollapsed">
          <BusinessGroupInlineWorkspace
            v-model:collapsed-keys="collapsedProductClassificationGroups"
            :groups="displaySkuGroups"
            :move-active="productCategoryMoveActive"
            :selected-count="selectedProductIds.length"
            :can-move="canMoveSelectedProductsToBusinessGroup"
            :loading="loading"
            count-unit="款"
            data-pr442-product-group-assignments
            data-pr442-business-group-items-api="/api/business-group-items"
            @manage="openProductBusinessGroupManagement"
            @configure="openProductGroupTemplateDrawer"
            @move="productCategoryMoveActive = true"
            @cancel="productCategoryMoveActive = false"
            @target="handleProductCategoryMoveTarget">
            <template #filters>
              <div v-if="!productCatalogBusinessGroups.length" class="classification-view-toolbar product-business-group-empty">
                <span>商品档案尚未选择分组模板，当前按全部商品平铺展示。</span>
              </div>
              <div class="sku-filters product-filter-row">
                <label>
                  <span>搜索</span>
                  <input v-model.trim="skuFilters.query" placeholder="搜索商品名称/类型/备注" />
                </label>
                <label>
                  <span>状态</span>
                  <select v-model="skuFilters.active">
                    <option value="all">全部</option>
                    <option value="active">有效</option>
                    <option value="inactive">已失效</option>
                  </select>
                </label>
                <div class="filter-actions sku-list-actions">
                  <button class="primary compact-action" type="button" @click="openProductDrawer">创建新商品档案</button>
                  <button class="secondary compact-action danger-outline" type="button" @click="deactivateProducts(selectedProductIds)" :disabled="!selectedProductIds.length || loading">
                    失效商品
                  </button>
                </div>
              </div>
            </template>

            <template #group="{ group }">
              <div class="table-wrap sku-table-wrap product-inline-group-table">
                <table :key="`${skuTableKey}:${group.key}`" class="sku-table" data-auto-pagination="off">
                  <thead>
                    <tr>
                      <th class="select-col">
                        <input
                          type="checkbox"
                          :checked="areProductGroupRowsSelected(group)"
                          :disabled="!editableProductGroupRows(group).length"
                          @change="toggleProductGroupRows(group, $event.target.checked)" />
                      </th>
                      <th class="sku-name-cell">商品名</th>
                      <th>商品编号</th>
                      <th>行业字段</th>
                      <th>归属</th>
                      <th class="action-cell">新增动作</th>
                      <th>库存单位</th>
                      <th>整数库存</th>
                      <th>价格摘要</th>
                      <th>商品状态</th>
                      <th>处理</th>
                      <th class="remark-cell">备注</th>
                    </tr>
                  </thead>
                  <tbody>
                    <template v-for="row in group.rows" :key="`${group.key}-${row.id}`">
                      <tr :class="[{ 'inactive-sku': row.active === false, 'sku-highlight': row.id === highlightedSkuId }, 'classification-item-row']">
                        <td class="select-col">
                          <input type="checkbox" :checked="isProductSelected(row)" :disabled="!canEditSkuRow(row) || row.active === false" @change="toggleProductSelection(row, $event.target.checked)" />
                        </td>
                        <td class="sku-name-cell">
                          <button class="text-button sku-name-button" type="button" :disabled="row.active === false" @click="openProductProductionConfig(row)">{{ row.name || '未命名商品' }}</button>
                          <details v-if="row.sku_rows?.length" class="product-spec-skus">
                            <summary>{{ row.sku_rows.length }} 个规格 SKU</summary>
                            <div class="product-spec-sku-list">
                              <button
                                v-for="sku in row.sku_rows"
                                :key="`product-spec-sku-${sku.id}`"
                                class="product-spec-sku-item"
                                type="button"
                                :disabled="sku.active === false"
                                @click.stop="openProductProductionConfig(sku)">
                                <span>{{ sku.sku_name || sku.spec_label || sku.name }}</span>
                                <small>{{ productCodeLabel(sku) }}</small>
                              </button>
                            </div>
                          </details>
                        </td>
                        <td>{{ productCodeLabel(row) }}</td>
                        <td class="industry-field-cell">
                          <span>{{ industryFieldSummary(productionConfigPriceListFields(row)) }}</span>
                          <button class="text-button" type="button" :disabled="row.active === false" @click="openProductProductionConfig(row)">设置</button>
                        </td>
                        <td>{{ productOwnerLabel(row) }}</td>
                        <td class="action-cell">
                          <button class="text-button" type="button" @click="copyProductArchive(row)">复制为商品档案</button>
                        </td>
                        <td>{{ productInventoryUnitLabel(row) }}</td>
                        <td>{{ productIntegerInventoryLabel(row) }}</td>
                        <td class="price-summary-cell">{{ productPriceSummaryLabel(row) }}</td>
                        <td>
                          <span :class="['status-pill', row.active === false ? 'inactive' : '']">{{ skuStatusLabel(row) }}</span>
                        </td>
                        <td>
                          <button class="text-button danger-text" type="button" :disabled="!canEditSkuRow(row) || row.active === false" @click="deactivateProducts([row.id])">停用</button>
                        </td>
                        <td>
                          <textarea
                            class="remark-input"
                            v-model.trim="row.remark"
                            rows="2"
                            :disabled="!canEditSkuRow(row) || row.active === false"
                            @change="saveProductBasics(row, 'SKU备注已保存')"></textarea>
                        </td>
                      </tr>
                    </template>
                    <tr v-if="!group.rows.length">
                      <td :colspan="12" class="muted">当前分类暂无商品档案</td>
                    </tr>
                  </tbody>
                </table>
                <PaginationControls
                  v-if="group.needsPagination"
                  :key="`${group.key}-pagination-${group.pageSize}-${group.total}`"
                  :page="group.page"
                  :page-size="group.pageSize"
                  :total="group.total"
                  :disabled="loading || productCategoryMoveActive"
                  @change="handleProductGroupPaginationChange(group.key, $event)"
                />
              </div>
            </template>
          </BusinessGroupInlineWorkspace>
        </div>
      </div>
        </div>
      </div>

      <div v-show="currentSettingsSection === 'aliases'" class="customer-alias-workspace">
        <section class="panel customer-alias-panel">
          <div class="panel-title">
            <span>客户商品 · {{ aliasCustomerLabel }}</span>
          </div>
          <p class="muted">客户商品只维护对外名称、编号、重命名和价格表展示；生产结构回到生产 BOM 维护，商品档案只提供库存对象和反查入口。</p>
          <div class="alias-filters alias-filter-row">
            <label>
              <span>客户</span>
              <SearchableSelect
                v-model="selectedAliasCustomerID"
                :options="customerSkuCustomers"
                :option-label="customerOptionLabel"
                :option-meta="customerOptionMeta"
                :option-value="optionNumericValue"
                placeholder="选择客户"
                empty-text="暂无客户" />
            </label>
            <label>
              <span>搜索</span>
              <input v-model.trim="aliasFilters.query" placeholder="客户商品/编号/绑定商品" />
            </label>
            <label>
              <span>状态</span>
              <select v-model="aliasFilters.active">
                <option value="active">启用</option>
                <option value="inactive">停用</option>
                <option value="all">全部</option>
              </select>
            </label>
            <div class="filter-actions">
              <button class="primary compact-action" type="button" :disabled="!selectedAliasCustomerID" @click="openCustomerAliasCreateDrawer">新建客户商品</button>
              <button class="secondary compact-action danger-outline" type="button" :disabled="!selectedAliasIds.length || aliasSaving" @click="batchDisableCustomerProductAliases">批量失效</button>
            </div>
          </div>
          <div class="classification-view-toolbar alias-classification-tabs" aria-label="客户商品分类模板视图">
            <div class="classification-tabs">
              <button
                v-for="tab in aliasClassificationTabs"
                :key="tab.key"
                :class="['classification-tab', { active: tab.key === activeAliasClassificationTab }]"
                type="button"
                @click="activeAliasClassificationTab = tab.key">
                {{ tab.label }}
              </button>
            </div>
            <div class="classification-select-row alias-classification-selects">
              <SearchableSelect
                v-model="selectedAliasClassificationTemplateID"
                :options="aliasAddClassificationOptions"
                :option-label="classificationTemplateOptionLabel"
                :option-meta="classificationTemplateOptionMeta"
                :option-value="optionNumericValue"
                placeholder="增加分类"
                empty-text="没有可增加的分类"
                :disabled="!selectedAliasCustomerID"
                @select="confirmAliasClassificationTemplateUsage" />
              <SearchableSelect
                v-model="selectedAliasClassificationMoveID"
                :options="aliasMoveClassificationOptions"
                :option-label="classificationMoveOptionLabel"
                :option-meta="classificationMoveOptionMeta"
                :option-value="optionNumericValue"
                placeholder="移动到分类"
                empty-text="没有可移动的分类"
                :disabled="!selectedAliasIds.length"
                @select="confirmSelectedAliasClassificationMove" />
            </div>
          </div>
          <div class="table-wrap">
            <table class="customer-alias-table">
              <thead>
                <tr>
                  <th class="select-col">
                    <input type="checkbox" :checked="allAliasRowsSelected" :disabled="!visibleCustomerProductAliases.length" @change="toggleAllAliasRows($event.target.checked)" />
                  </th>
                  <th>客户商品</th>
                  <th>客户商品编号</th>
                  <th>绑定商品档案</th>
                  <th>价格摘要</th>
                  <th>当前归类</th>
                  <th>客户行业字段</th>
                  <th>进入价格表</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="group in visibleCustomerAliasGroups" :key="group.key">
                  <tr v-if="!group.all" class="classification-group-row">
                    <td colspan="10">
                      <button class="classification-group-toggle" type="button" @click="toggleAliasClassificationGroup(group.key)">
                        {{ isAliasClassificationGroupCollapsed(group.key) ? '展开' : '收起' }}
                      </button>
                      <strong>{{ group.label }}</strong>
                      <small>{{ group.rows.length }} 款</small>
                    </td>
                  </tr>
                  <template v-if="!isAliasClassificationGroupCollapsed(group.key)">
                <tr v-for="alias in group.rows" :key="`${group.key}-${alias.id}`" class="classification-item-row">
                  <td class="select-col">
                    <input type="checkbox" :checked="isAliasSelected(alias)" :disabled="alias.active === false" @change="toggleAliasSelection(alias, $event.target.checked)" />
                  </td>
                  <td>
                    <button class="text-button" type="button" @click="openCustomerProductAliasEditor(alias)">{{ customerAliasEffectiveDisplayName(alias) }}</button>
                  </td>
                  <td>{{ alias.customer_item_code || '-' }}</td>
                  <td :class="{ 'invalid-product-reference': alias.product_active === false }">
                    <span>{{ alias.product_code || alias.product_id }} · {{ alias.product_name || productName(alias.product_id) }}</span>
                    <small v-if="alias.product_active === false" class="inactive-product-warning">绑定商品已失效</small>
                  </td>
                  <td class="price-summary-cell">{{ aliasPriceSummaryLabel(alias) }}</td>
                  <td>
                    <div>{{ aliasClassificationLabel(alias) }}</div>
                    <small v-for="warning in classificationWarningsForAlias(alias)" :key="warning" class="bom-version-warning">{{ warning }}</small>
                  </td>
                  <td class="industry-field-cell">
                    <span>{{ industryFieldSummary(alias.industry_fields || []) }}</span>
                    <button class="text-button" type="button" :disabled="alias.active === false" @click="openAliasIndustryFieldDrawer(alias)">设置</button>
                  </td>
                  <td>{{ alias.include_in_price_list ? '是' : '否' }}</td>
                  <td><span :class="['status-pill', alias.active === false ? 'inactive' : '']">{{ alias.active === false ? '停用' : '启用' }}</span></td>
                  <td class="table-actions">
                    <button class="text-button danger-text" type="button" :disabled="alias.active === false" @click="disableCustomerProductAlias(alias)">停用</button>
                  </td>
                </tr>
                  </template>
                </template>
                <tr v-if="!visibleCustomerProductAliases.length">
                  <td colspan="10" class="muted">当前客户暂无客户商品。</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <div v-show="currentSettingsSection === 'templates'" class="sku-template-workspace">
        <div class="template-workspace-stack">
          <div v-if="currentSettingsSection === 'templates' && !forcedConfigTemplateSection" class="config-template-tabs" role="tablist" aria-label="商品配置模板类型">
            <button
              type="button"
              :class="['config-template-tab', { active: activeConfigTemplateSection === 'product-config' }]"
              @click="activeConfigTemplateSection = 'product-config'">
              商品配置模板
            </button>
            <button
              type="button"
              :class="['config-template-tab', { active: activeConfigTemplateSection === 'product-price-management' }]"
              @click="activeConfigTemplateSection = 'product-price-management'">
              商品价格管理
            </button>
          </div>
      <div v-show="showGradientTemplatePane" class="panel gradient-template-panel gradient-template-pane">
        <div class="panel-title">
          <span>阶梯价模板</span>
          <button class="secondary compact-action" type="button" @click="resetGradientTemplateForm">新建模板</button>
        </div>
        <div v-if="selectedCustomerSkuCustomerID" class="customer-copy-panel">
          <label class="checkline switchline">
            <input :checked="customerUsesPublicGradientTemplates" type="checkbox" :disabled="publicUsageSaving" @change="savePublicGradientTemplateUsageForCustomer" />
            <span>是否使用公共梯度模板</span>
          </label>
        </div>
        <div class="gradient-template-layout">
          <div class="template-list">
            <div
              v-for="template in gradientTemplatesForContext"
              :key="template.id"
              :class="['template-row', { active: Number(template.id) === Number(templateForm.id), inactive: template.active === false }]">
              <button class="template-row-main" type="button" @click="startGradientTemplateEdit(template)">
                <strong>{{ template.name }}</strong>
                <small>{{ gradientTemplateLabel(template) }} · {{ gradientDisplayUnitLabel(template.display_unit) }} · {{ template.tiers.length }} 档</small>
              </button>
              <button
                v-if="canDeriveGradientTemplate(template)"
                class="text-button template-copy-action"
                type="button"
                :disabled="templateSaving"
                @click.stop="deriveGradientTemplateForCustomer(template)">
                复制为客户模板
              </button>
            </div>
            <p v-if="!gradientTemplatesForContext.length" class="muted">暂无梯度模板</p>
          </div>
          <form class="template-editor" @submit.prevent="saveGradientTemplate">
            <p v-if="!canEditCurrentTemplate" class="muted">公共模板需复制到客户后修改。</p>
            <div class="template-editor-grid">
              <label>
                <span>模板名称</span>
                <input v-model.trim="templateForm.name" :disabled="!canEditCurrentTemplate" placeholder="如 工厂量单模板" />
              </label>
              <label>
                <span>展示单位</span>
                <select v-model="templateForm.display_unit" :disabled="!canEditCurrentTemplate">
                  <option v-for="unit in gradientDisplayUnitOptions" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                </select>
              </label>
              <label class="checkline">
                <input v-model="templateForm.allow_customer_resale" type="checkbox" :disabled="!canEditCurrentTemplate" />
                <span>允许客户转售豆单使用</span>
              </label>
            </div>
            <div class="template-tier-head">
              <strong>梯度档位</strong>
              <button class="secondary compact-action" type="button" :disabled="!canEditCurrentTemplate" @click="addGradientTemplateTier">新增档位</button>
            </div>
            <div class="template-tier-list">
              <div v-for="(tier, index) in templateForm.tiers" :key="`tier-${index}`" class="template-tier-row">
                <label>
                  <span>区间名</span>
                  <input v-model.trim="tier.label" :disabled="!canEditCurrentTemplate" placeholder="24-49kg" />
                </label>
                <label>
                  <span>最小数量（{{ gradientDisplayQuantityUnitLabel(templateForm.display_unit) }}）</span>
                  <input v-model.number="tier.min_display_qty" type="number" min="0" :step="gradientDisplayQuantityStep(templateForm.display_unit)" :disabled="!canEditCurrentTemplate" />
                </label>
                <label>
                  <span>最大数量（{{ gradientDisplayQuantityUnitLabel(templateForm.display_unit) }}）</span>
                  <input v-model="tier.max_display_qty" type="number" min="0" :step="gradientDisplayQuantityStep(templateForm.display_unit)" :disabled="!canEditCurrentTemplate" placeholder="无上限" />
                </label>
                <label>
                  <span>利润率</span>
                  <input v-model.number="tier.margin_rate" type="number" min="0" step="0.001" :disabled="!canEditCurrentTemplate" />
                </label>
                <button class="text-button danger-text" type="button" :disabled="!canEditCurrentTemplate" @click="removeGradientTemplateTier(index)">删除</button>
              </div>
            </div>
            <div class="form-actions">
              <button class="primary" type="submit" :disabled="templateSaving || !canEditCurrentTemplate">保存模板</button>
              <button v-if="templateForm.id" class="secondary" type="button" :disabled="templateSaving || !canEditCurrentTemplate" @click="deactivateGradientTemplate(templateForm.id)">停用模板</button>
            </div>
          </form>
        </div>
      </div>

      <div v-show="showUnitTemplatePane" class="panel unit-template-panel unit-template-pane">
        <div class="panel-title">
          <span>旧商品销售规格模板（迁移期）</span>
          <button class="secondary compact-action" type="button" @click="openGlobalUnitDictionaryDrawer">全局单位字典</button>
        </div>
        <p class="muted unit-template-note">这里维护库存单位和销售规格换算，仅供尚未切换到 BOM 规格的旧商品及历史派生子 SKU；已切换商品请到 BOM 维护规格。</p>
        <div class="unit-template-layout">
          <section class="unit-template-card unit-template-list-panel">
            <div class="field-group-head">
              <strong>销售规格模板</strong>
              <small>点击模板在右侧编辑。</small>
            </div>
            <div class="template-list compact-template-list">
              <div
                v-for="unitTemplate in visibleProductUnitTemplates"
                :key="unitTemplate.id"
                :class="['template-row', { active: Number(unitTemplate.id) === Number(productUnitTemplateForm.id), inactive: unitTemplate.active === false }]">
                <button class="template-row-main" type="button" @click="startProductUnitTemplateEdit(unitTemplate)">
                  <strong>{{ unitTemplate.name }}</strong>
                  <small>{{ productUnitTemplateSummary(unitTemplate) }}</small>
                </button>
                <button class="text-button danger-text" type="button" :disabled="unitTemplate.active === false" @click="deleteProductUnitTemplate(unitTemplate)">删除</button>
              </div>
              <p v-if="!visibleProductUnitTemplates.length" class="muted">暂无销售规格模板</p>
            </div>
          </section>
          <section class="unit-template-card unit-template-editor-panel">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>{{ productUnitTemplateForm.id ? '编辑销售规格模板' : '新增销售规格模板' }}</strong>
                <small>保存后刷新列表并回到空白表单。</small>
              </div>
              <button class="secondary compact-action" type="button" @click="resetProductUnitTemplateForm">新增销售规格模板</button>
            </div>
            <form class="unit-template-form" @submit.prevent="saveProductUnitTemplate">
              <label class="wide-field">
                <span>销售规格模板名称</span>
                <input v-model.trim="productUnitTemplateForm.name" placeholder="咖啡豆袋装规格" />
              </label>
              <div class="template-editor-grid">
                <label class="wide-field">
                  <span>库存单位</span>
                  <select v-model="productUnitTemplateForm.inventory_unit" :disabled="productUnitTemplateInventoryUnitLocked" @change="syncSalesSpecTemplateInventoryUnit(productUnitTemplateForm)">
                    <option v-for="unit in activeProductUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option>
                  </select>
                  <small v-if="productUnitTemplateInventoryUnitLocked">库存单位保存后不可修改；如需调整，请新建销售规格模板。</small>
                </label>
              </div>
              <div class="sales-spec-editor">
                <div class="field-group-head">
                  <span>销售规格明细</span>
                  <button class="secondary compact-action" type="button" @click="addSalesSpecRow(productUnitTemplateForm)">新增规格</button>
                </div>
                <div v-for="(row, rowIndex) in productUnitTemplateForm.sales_spec_rows" :key="`sales-spec-${row.spec_key || rowIndex}`" class="sales-spec-row">
                  <span class="conversion-prefix">1</span>
                  <input v-model.trim="row.spec_name" class="sales-spec-name-input" placeholder="227g袋装" @input="syncSalesSpecTemplateDefaults(productUnitTemplateForm)" />
                  <span class="conversion-equals">=</span>
                  <input v-model.number="row.net_content_qty" class="sales-spec-qty-input" type="number" min="0" step="0.0001" placeholder="0.227" />
                  <span class="inventory-unit-chip">{{ productUnitName(productUnitTemplateForm.inventory_unit) }}</span>
                  <button
                    type="button"
                    :class="['default-spec-toggle', { active: row.default }]"
                    :aria-pressed="row.default ? 'true' : 'false'"
                    @click="setSalesSpecDefault(productUnitTemplateForm, rowIndex)"
                  >默认规格</button>
                  <button class="text-button danger-text" type="button" @click="removeSalesSpecRow(productUnitTemplateForm, rowIndex)">删除</button>
                </div>
                <small>例如：1 227g袋装 = 0.227 {{ productUnitName(productUnitTemplateForm.inventory_unit || 'kg') }}。保存商品档案后会按模板规格派生子 SKU。</small>
              </div>
              <div class="form-actions">
                <button class="primary" type="submit" :disabled="productUnitSaving">
                  {{ productUnitSaving ? '保存中' : (productUnitTemplateForm.id ? '保存' : '新增') }}
                </button>
              </div>
            </form>
          </section>
        </div>
      </div>

      <div v-show="currentSettingsSection === 'templates' && effectiveConfigTemplateSection === 'product-config'" class="panel product-config-panel product-config-template-pane">
        <div class="panel-title">
          <span>商品配置模板</span>
          <button class="secondary compact-action" type="button" @click="resetProductConfigTemplateForm">新建商品配置</button>
        </div>
        <div class="product-config-layout">
          <div class="template-list product-config-list">
            <div
              v-for="config in productConfigTemplatesForContext"
              :key="config.id"
              :class="['template-row', 'product-config-row', { active: Number(config.id || 0) === Number(productConfigTemplateForm.id || 0), inactive: config.active === false }]"
              role="button"
              tabindex="0"
              @click="startProductConfigTemplateEdit(config)"
              @keydown.enter.prevent="startProductConfigTemplateEdit(config)">
              <span class="template-row-main product-config-row-main">
                <span class="product-config-row-title">
                  <strong>{{ config.name }}</strong>
                  <span class="template-state-pill">{{ productConfigTemplateLabel(config) }}</span>
                </span>
                <span class="product-config-row-subtitle">{{ productConfigUnitTemplateName(config.unit_template_id) }}</span>
                <span class="template-meta-chips" aria-label="商品配置单位摘要">
                  <span v-for="chip in productConfigUnitChips(config.unit_template_id)" :key="chip" class="template-meta-chip">{{ chip }}</span>
                </span>
              </span>
              <button
                v-if="canDeriveProductConfigTemplate(config)"
                class="text-button"
                type="button"
                :disabled="productConfigSaving"
                @click.stop="deriveProductConfigTemplateForCustomer(config)">
                复制为客户配置
              </button>
            </div>
            <p v-if="!productConfigTemplatesForContext.length" class="muted">暂无商品配置</p>
          </div>

          <form class="product-config-editor" @submit.prevent="saveProductConfigTemplate">
            <p v-if="!canEditCurrentProductConfigTemplate" class="muted">公共商品配置需复制到客户后修改。</p>
            <label>
              <span>配置名称</span>
              <input v-model.trim="productConfigTemplateForm.name" :disabled="!canEditCurrentProductConfigTemplate" placeholder="如 盒装速溶配置" />
            </label>
            <label>
              <span>工序模板ID</span>
              <input v-model.number="productConfigTemplateForm.operation_template_id" :disabled="!canEditCurrentProductConfigTemplate" type="number" min="0" step="1" placeholder="0 表示未绑定" />
            </label>
            <div class="rule-config-block price-rule-grid">
              <div class="field-group-title">价格表生成规则</div>
              <label class="rule-config-field">
                <span>计价方式</span>
                <select v-model="productConfigTemplateForm.price_rule_pricing_mode" :disabled="!canEditCurrentProductConfigTemplate">
                  <option v-for="option in priceListRulePricingModeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </label>
              <label v-if="productConfigTemplateNeedsGradientTemplate(productConfigTemplateForm)" class="rule-config-field">
                <span>阶梯价模板</span>
                <select v-model.number="productConfigTemplateForm.gradient_template_id" :disabled="!canEditCurrentProductConfigTemplate">
                  <option value="0">未绑定模板</option>
                  <option v-for="template in selectableGradientTemplatesForProductConfig" :key="template.id" :value="template.id">{{ template.name }} · {{ gradientDisplayUnitLabel(template.display_unit) }}</option>
                </select>
              </label>
              <label v-if="productConfigTemplateForm.price_rule_pricing_mode === 'fixed_unit_price'" class="rule-config-field">
                <span class="field-label-with-help">
                  <span>固定单价</span>
                  <button type="button" class="field-help-icon" aria-label="固定单价说明">?</button>
                  <span class="field-help-tooltip" role="tooltip">按阶梯价模板单位填写最终售价。</span>
                </span>
                <input v-model.number="productConfigTemplateForm.price_rule_fixed_unit_price" :disabled="!canEditCurrentProductConfigTemplate" type="number" min="0" step="0.01" placeholder="按阶梯价模板单位" />
              </label>
              <label v-if="productConfigTemplateForm.price_rule_pricing_mode === 'cost_plus'" class="rule-config-field">
                <span class="field-label-with-help">
                  <span>成本加成%</span>
                  <button type="button" class="field-help-icon" aria-label="成本加成说明">?</button>
                  <span class="field-help-tooltip" role="tooltip">按成本乘以加成比例生成售价。</span>
                </span>
                <input v-model.number="productConfigTemplateForm.price_rule_cost_plus_percent" :disabled="!canEditCurrentProductConfigTemplate" type="number" min="0" step="0.01" placeholder="如 25" />
              </label>
              <label class="rule-config-field">
                <span>取整规则</span>
                <select v-model="productConfigTemplateForm.price_rule_rounding" :disabled="!canEditCurrentProductConfigTemplate">
                  <option v-for="option in priceListRuleRoundingOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </label>
              <label class="checkline">
                <input v-model="productConfigTemplateForm.price_rule_tax_included" :disabled="!canEditCurrentProductConfigTemplate" type="checkbox" />
                <span>含税价格</span>
              </label>
            </div>
            <div class="rule-config-block">
              <div class="field-group-title">销售规格</div>
              <small class="unit-impact-help">商品规格统一由默认制造 BOM 的规格组提供；配置模板不再单独引用规格模板。</small>
            </div>
            <div class="form-actions">
              <button class="primary" type="submit" :disabled="productConfigSaving || !canEditCurrentProductConfigTemplate">保存商品配置</button>
              <button v-if="productConfigTemplateForm.id" class="secondary" type="button" :disabled="productConfigSaving || !canEditCurrentProductConfigTemplate" @click="deactivateProductConfigTemplate(productConfigTemplateForm.id)">停用配置</button>
              <button v-if="productConfigTemplateForm.id" class="secondary danger-outline" type="button" :disabled="productConfigSaving || !canEditCurrentProductConfigTemplate" @click="deleteProductConfigTemplate(productConfigTemplateForm.id)">删除配置</button>
            </div>
          </form>
        </div>
      </div>

      <div v-show="showProductPriceManagementPane" class="panel product-price-management-pane">
        <div class="panel-title">
          <span>商品价格管理</span>
          <div class="panel-actions">
            <button class="secondary compact-action" type="button" @click="openPricingRuleTrial()">价格试算</button>
            <button class="secondary compact-action" type="button" @click="resetPricingRuleForm">新建价格计算模板</button>
          </div>
        </div>
        <div class="product-price-management-layout">
          <section class="product-price-records-panel pricing-rule-management-panel">
            <div class="field-group-head">
              <strong>价格计算模板 / Pricing Rule</strong>
              <small>模板只负责基础成本、其他成本、加价率、税费和取整公式；不绑定商品，不保存数量档位和最终成交价。商品价格表引用模板后生成平铺价格行。</small>
            </div>
            <div class="table-wrap compact-table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>模板</th>
                    <th>公式版本</th>
                    <th>基础成本</th>
                    <th>加价率</th>
                    <th>税率</th>
                    <th>取整规则</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="rule in pricingRules" :key="rule.id" :class="['pricing-rule-row', { inactive: rule.active === false }]">
                    <td>
                      <button class="text-button pricing-rule-name-button" type="button" @click="startPricingRuleEdit(rule)">
                        {{ rule.name || rule.code || `Pricing Rule #${rule.id}` }}
                      </button>
                      <small v-if="pricingRuleNeedsMarkupConfirmation(rule)" class="pricing-rule-migration-warning">旧方式待确认，请新建加价率模板</small>
                    </td>
                    <td>{{ rule.formula_version || 'v1' }}</td>
                    <td>{{ pricingRuleCostSourceLabel(rule.cost_source_mode) }}</td>
                    <td>{{ percentDisplay(rule.margin_rate) }}</td>
                    <td>{{ percentDisplay(rule.tax_rate) }}</td>
                    <td>{{ pricingRuleRoundingLabel(rule.rounding_mode) }}</td>
                    <td><span :class="['status-pill', rule.active === false ? 'inactive' : '']">{{ rule.active === false ? '停用' : '启用' }}</span></td>
                    <td class="table-actions">
                      <button
                        class="secondary compact-action pricing-rule-copy-action"
                        type="button"
                        :disabled="productPriceSaving || pricingRuleNeedsMarkupConfirmation(rule)"
                        :title="pricingRuleNeedsMarkupConfirmation(rule) ? '旧价格方式无法安全换算，请新建加价率模板' : '复制价格计算模板'"
                        @click="copyPricingRule(rule)">复制</button>
                      <button v-if="rule.active !== false" class="secondary compact-action danger-outline" type="button" :disabled="productPriceSaving" @click="deactivatePricingRule(rule)">失效</button>
                    </td>
                  </tr>
                  <tr v-if="!pricingRules.length">
                    <td colspan="8" class="muted">暂无价格计算模板。可先新建模板，再在商品价格表生成时引用。</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
        <p class="muted price-list-flat-row-note" aria-label="父商品 > 所在分类 > 上级分类逐级向上 > 价格表">商品价格表按分组勾选商品后生成平铺价格行；计价模式继承规则为：父商品 &gt; 所在分类 &gt; 上级分类逐级向上 &gt; 价格表，最近设置优先。阶梯模板在商品价格表维护。</p>
      </div>
        </div>
      </div>
    </section>

    <div v-if="pricingRuleEditorDrawerOpen" class="settings-drawer-mask" @click.self="closePricingRuleEditor" @keydown.esc.stop.prevent="closePricingRuleEditor">
      <aside
        ref="pricingRuleEditorDrawer"
        class="settings-drawer pricing-rule-editor-drawer"
        role="dialog"
        aria-modal="true"
        aria-label="价格计算模板编辑"
        tabindex="-1"
        @keydown.tab="trapPricingRuleEditorFocus">
        <div class="drawer-head">
          <div>
            <h3>{{ pricingRuleForm.id ? '编辑价格计算模板' : '新建价格计算模板' }}</h3>
            <p>统一按加价率计算。80% 请填写 0.8；税前价 = 成本基数 × (1 + 加价率)，最终售价再计算税额和取整。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closePricingRuleEditor">关闭</button>
        </div>
        <div class="drawer-body">
          <div v-if="pricingRuleNeedsMarkupConfirmation(pricingRuleForm)" class="error pricing-rule-migration-alert" role="alert">
            <strong>旧价格方式无法安全换算；请新建加价率模板。</strong>
            <span>该模板仅供核对，不能复制或直接保存。原方式：{{ pricingRuleLegacyMethodLabel(pricingRuleForm) }}；原参数：{{ pricingRuleLegacyValueLabel(pricingRuleForm) }}。</span>
          </div>
          <form class="template-editor pricing-rule-form" @submit.prevent="savePricingRule">
            <div class="template-editor-grid">
              <label>
                <span>模板名称</span>
                <input v-model.trim="pricingRuleForm.name" placeholder="如 成本加成含税" />
              </label>
              <label>
                <span>模板编号</span>
                <input v-model.trim="pricingRuleForm.code" placeholder="如 PR-COST-PLUS" />
              </label>
              <label>
                <span>基础成本</span>
                <select v-model="pricingRuleForm.cost_source_mode">
                  <option value="bom_current_cost">生产 BOM 成本（物料+工序）</option>
                </select>
              </label>
              <label>
                <span>公式版本</span>
                <input v-model.trim="pricingRuleForm.formula_version" placeholder="v1" />
              </label>
            </div>
            <div class="pricing-rule-form-section">
              <div class="pricing-rule-section-head">
                <strong>其他成本</strong>
                <button class="secondary compact-action" type="button" @click="addPricingRuleOtherCostRow">新增其他成本</button>
              </div>
              <small class="muted">生产 BOM 成本已包含物料采购成本和已选择工序成本；货币使用全局币种配置，当前不在价格模板中单独设置。</small>
              <div class="pricing-rule-other-cost-list">
                <div v-for="(row, index) in pricingRuleForm.other_cost_rows" :key="index" class="pricing-rule-other-cost-row">
                  <label>
                    <span>成本名</span>
                    <input v-model.trim="row.key" placeholder="如 包装贴标" />
                  </label>
                  <label>
                    <span>成本价格</span>
                    <input v-model.number="row.value" type="number" min="0" step="0.0001" placeholder="0" />
                  </label>
                  <button class="secondary compact-action" type="button" @click="removePricingRuleOtherCostRow(index)">删除</button>
                </div>
              </div>
            </div>
            <div class="template-editor-grid">
              <label>
                <span>加价率（80%=0.8）</span>
                <input v-model.number="pricingRuleForm.margin_rate" type="number" min="0" step="0.0001" placeholder="如 0.8" />
                <small>计算公式：税前价 = 成本基数 × (1 + 加价率)；最终售价再计算税额和取整</small>
              </label>
              <label>
                <span>最低毛利率（仅预警）</span>
                <input v-model.number="pricingRuleForm.minimum_margin_rate" type="number" min="0" step="0.0001" placeholder="0.18" />
                <small>只比较试算结果，不参与售价计算</small>
              </label>
              <label>
                <span>税费方式</span>
                <select v-model="pricingRuleForm.tax_mode">
                  <option value="tax_included">含税</option>
                  <option value="tax_excluded">未税</option>
                  <option value="none">不计税</option>
                </select>
              </label>
              <label>
                <span>税率</span>
                <input v-model.number="pricingRuleForm.tax_rate" type="number" min="0" step="0.0001" placeholder="0.06" />
              </label>
              <label>
                <span>取整规则</span>
                <select v-model="pricingRuleForm.rounding_mode">
                  <option value="none">不取整</option>
                  <option value="jiao">保留到角</option>
                  <option value="yuan">保留到元</option>
                </select>
              </label>
            </div>
            <label class="wide-field">
              <span>备注</span>
              <textarea v-model.trim="pricingRuleForm.remark" rows="2" placeholder="说明 BOM/耗材/工艺成本如何参与试算"></textarea>
            </label>
            <label class="wide-field">
              <span>试算说明</span>
              <textarea v-model.trim="pricingRuleForm.trial_note" rows="2" placeholder="例如：选择商品、销售单位后按生产 BOM 成本试算"></textarea>
            </label>
            <div class="form-actions">
              <button class="primary" type="submit" :disabled="productPriceSaving || pricingRuleNeedsMarkupConfirmation(pricingRuleForm)">保存价格计算模板</button>
              <button v-if="pricingRuleForm.id && pricingRuleForm.active !== false" class="secondary danger-outline" type="button" :disabled="productPriceSaving" @click="deactivatePricingRule(pricingRuleForm)">失效</button>
            </div>
          </form>
        </div>
      </aside>
    </div>

    <div v-if="pricingRuleTrialDrawerOpen" class="settings-drawer-mask" @click.self="closePricingRuleTrial">
      <aside class="settings-drawer pricing-rule-trial-drawer" aria-label="价格计算模板试算">
        <div class="drawer-head">
          <div>
            <h3>价格计算模板试算</h3>
            <p>{{ pricingRuleTrialRule?.name || pricingRuleTrialRule?.code || '价格计算模板' }} · {{ pricingRuleTrialRule?.formula_version || 'v1' }}</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closePricingRuleTrial">关闭</button>
        </div>
        <div class="drawer-body">
          <section class="drawer-section pricing-rule-trial-summary">
            <div>
              <strong>{{ pricingRuleTrialRule?.name || pricingRuleTrialRule?.code || '未命名模板' }}</strong>
              <small>{{ pricingRuleTrialRule ? '启用模板，可将调试参数更新回当前模板' : '请先选择启用的价格计算模板' }}</small>
            </div>
            <div class="pricing-rule-trial-rule-grid">
              <span>基础成本：{{ pricingRuleCostSourceLabel(pricingRuleTrialRule?.cost_source_mode) }}</span>
              <span>计价方式：加价率</span>
              <span>取整规则：{{ pricingRuleRoundingLabel(pricingRuleTrialRule?.rounding_mode) }}（来自价格计算模板）</span>
              <span>税率：{{ pricingRuleTrialTaxRateSummary(pricingRuleTrialResult, pricingRuleTrialRule) }}</span>
            </div>
          </section>

          <section class="drawer-section pricing-rule-trial-form-section">
            <div class="template-editor-grid pricing-rule-trial-grid">
              <label class="wide-field">
                <span>试算模板</span>
                <select v-model.number="pricingRuleTrialForm.pricing_rule_id" @change="handlePricingRuleTrialRuleChange">
                  <option :value="0">请选择启用的价格计算模板</option>
                  <option v-for="rule in activePricingRuleTrialOptions" :key="rule.id" :value="Number(rule.id || 0)">
                    {{ pricingRuleOptionLabel(rule) }}
                  </option>
                </select>
                <small v-if="!activePricingRuleTrialOptions.length" class="muted">暂无启用的价格计算模板，可先新建或复制停用模板。</small>
              </label>
              <label class="wide-field">
                <span>试算商品</span>
                <SearchableSelect
                  :key="`pricing-rule-trial-product-picker:${activePricingRuleTrialProductKindFilter}`"
                  v-model="pricingRuleTrialForm.parent_product_id"
                  :options="pricingRuleTrialProductOptions"
                  :option-label="productOptionLabel"
                  :option-meta="productOptionMeta"
                  :option-value="optionNumericValue"
                  placeholder="选择商品档案"
                  empty-text="暂无可试算商品">
                  <template #menu-header>
                    <div
                      v-if="pricingRuleTrialProductKindFilterOptions.length > 1"
                      class="product-kind-filter"
                      role="group"
                      aria-label="商品分类">
                      <button
                        v-for="option in pricingRuleTrialProductKindFilterOptions"
                        :key="option.value || 'all'"
                        type="button"
                        class="product-kind-filter-option"
                        :class="{ active: activePricingRuleTrialProductKindFilter === option.value }"
                        :aria-pressed="activePricingRuleTrialProductKindFilter === option.value"
                        @mousedown.prevent
                        @click.stop="setPricingRuleTrialProductKindFilter(option.value)">
                        {{ option.label }}
                      </button>
                    </div>
                  </template>
                  <template #option="{ option }">
                    <strong>
                      {{ productOptionLabel(option) }}
                      <span class="kind-badge" :class="productKindBadgeClass(option)">{{ productKindLabel(option) }}</span>
                    </strong>
                    <small v-if="productOptionMeta(option)">{{ productOptionMeta(option) }}</small>
                  </template>
                </SearchableSelect>
              </label>
              <div class="pricing-rule-trial-bom-field">
                <label>
                  <span>试算BOM版本</span>
                  <select v-model.number="pricingRuleTrialForm.bom_version_id" :disabled="!pricingRuleTrialBomVersionOptions.length">
                    <option :value="0">按默认版本</option>
                    <option v-for="option in pricingRuleTrialBomVersionOptions" :key="option.version_id" :value="Number(option.version_id || 0)">
                      {{ pricingRuleTrialBomVersionOptionLabel(option) }}
                    </option>
                  </select>
                </label>
                <button class="secondary compact-action" type="button" :disabled="!selectedPricingRuleTrialBomVersion?.bom_id" @click="navigatePricingRuleTrialBom">配置BOM</button>
              </div>
              <label>
                <span>工艺路线</span>
                <select v-model.number="pricingRuleTrialForm.process_route_id" :disabled="!pricingRuleTrialProcessRouteOptions.length">
                  <option :value="0">按 BOM/商品默认工艺路线</option>
                  <option v-for="option in pricingRuleTrialProcessRouteOptions" :key="option.id" :value="Number(option.id || 0)">
                    {{ pricingRuleTrialProcessRouteOptionLabel(option) }}
                  </option>
                </select>
              </label>
              <label>
                <span>客户范围（可选）</span>
                <select v-model.number="pricingRuleTrialForm.customer_id">
                  <option value="0">公共商品档案</option>
                  <option v-for="customer in customers" :key="customer.id" :value="Number(customer.id || 0)">{{ customer.name || `客户 #${customer.id}` }}</option>
                </select>
              </label>
              <label>
                <span>销售规格</span>
                <select v-model.number="pricingRuleTrialForm.product_id" :disabled="!pricingRuleTrialForm.parent_product_id || !pricingRuleTrialSalesSpecOptions.length">
                  <option :value="0">{{ pricingRuleTrialForm.parent_product_id ? '请选择销售规格' : '请先选择试算商品' }}</option>
                  <option v-for="spec in pricingRuleTrialSalesSpecOptions" :key="Number(spec.sku_id || spec.id || 0)" :value="Number(spec.sku_id || spec.id || 0)">
                    {{ pricingRuleTrialProductSpecLabel(spec) }}
                  </option>
                </select>
                <small v-if="pricingRuleTrialForm.parent_product_id && !pricingRuleTrialSalesSpecOptions.length" class="muted">当前商品暂无可试算的有效销售规格，请先在商品档案维护规格。</small>
              </label>
              <label>
                <span>临时加价率</span>
                <input v-model.number="pricingRuleTrialForm.margin_rate" type="number" min="0" step="0.0001" />
              </label>
              <label>
                <span>临时税率</span>
                <input v-model.number="pricingRuleTrialForm.tax_rate" type="number" min="0" step="0.0001" />
              </label>
            </div>

            <div class="pricing-rule-section-head">
              <strong>其他成本</strong>
              <button class="secondary compact-action" type="button" @click="addPricingRuleTrialOtherCostRow">新增其他成本</button>
            </div>
            <div class="pricing-rule-other-cost-list">
              <div v-for="(row, index) in pricingRuleTrialForm.other_cost_rows" :key="index" class="pricing-rule-other-cost-row">
                <label>
                  <span>成本名</span>
                  <input v-model.trim="row.key" placeholder="如 包装贴标" />
                </label>
                <label>
                  <span>成本价格</span>
                  <input v-model.number="row.value" type="number" min="0" step="0.0001" placeholder="0" />
                </label>
                <button class="secondary compact-action" type="button" @click="removePricingRuleTrialOtherCostRow(index)">删除</button>
              </div>
            </div>

            <div v-if="pricingRuleTrialError" class="error">{{ pricingRuleTrialError }}</div>
            <div class="form-actions">
              <span v-if="pricingRuleTrialLoading" class="muted">试算中...</span>
              <button class="secondary" type="button" @click="closePricingRuleTrial">关闭</button>
            </div>
          </section>

          <section v-if="pricingRuleTrialResult" class="drawer-section pricing-rule-trial-result">
            <div class="pricing-rule-trial-waterfall">
              <button
                :class="['pricing-rule-trial-waterfall-card', 'interactive', { active: pricingRuleTrialActiveExplanation === 'base_cost', warning: pricingRuleTrialBaseCostMissing(pricingRuleTrialResult) }]"
                type="button" @click="openPricingRuleTrialExplanation('base_cost')"
                aria-label="点击查看试算说明：标准制造成本">
                <small>标准制造成本 <span v-if="pricingRuleTrialBaseCostMissing(pricingRuleTrialResult)" title="该商品暂无可试算的标准制造成本">!</span></small>
                <strong>{{ trialMoneyDisplay(pricingRuleTrialResult.standard_manufacturing_unit_cost ?? pricingRuleTrialResult.base_cost, pricingRuleTrialResult.quote_unit) }}</strong>
                <em>BOM物料成本 {{ trialMoneyDisplay(pricingRuleTrialResult.material_unit_cost ?? pricingRuleTrialResult.bom_cost_total, pricingRuleTrialResult.quote_unit) }} / 标准工序成本 {{ trialMoneyDisplay(pricingRuleTrialResult.operation_unit_cost ?? pricingRuleTrialResult.operation_cost_total, pricingRuleTrialResult.quote_unit) }}</em>
              </button>
              <span class="pricing-rule-trial-operator">+</span>
              <button
                :class="['pricing-rule-trial-waterfall-card', 'interactive', { active: pricingRuleTrialActiveExplanation === 'other_cost' }]"
                type="button" @click="openPricingRuleTrialExplanation('other_cost')"
                aria-label="点击查看试算说明：其他成本">
                <small>其他成本</small>
                <strong>{{ trialMoneyDisplay(pricingRuleTrialResult.other_cost_total, pricingRuleTrialResult.quote_unit) }}</strong>
                <em>成本基数 {{ trialMoneyDisplay(pricingRuleTrialResult.cost_base_total, pricingRuleTrialResult.quote_unit) }}</em>
              </button>
              <span class="pricing-rule-trial-operator">+</span>
              <button
                :class="['pricing-rule-trial-waterfall-card', 'interactive', { active: pricingRuleTrialActiveExplanation === 'profit_markup' }]"
                type="button" @click="openPricingRuleTrialExplanation('profit_markup')"
                aria-label="点击查看试算说明：加价增加">
                <small>加价增加</small>
                <strong>{{ trialMoneyDisplay(pricingRuleTrialResult.profit_markup_amount, pricingRuleTrialResult.quote_unit) }}</strong>
                <em>加价后价格 {{ trialMoneyDisplay(pricingRuleTrialResult.pre_tax_price, pricingRuleTrialResult.quote_unit) }}</em>
              </button>
              <span class="pricing-rule-trial-operator">+</span>
              <div class="pricing-rule-trial-waterfall-card">
                <small>税额</small>
                <strong>{{ trialMoneyDisplay(pricingRuleTrialTaxInPriceAmount(pricingRuleTrialResult), pricingRuleTrialResult.quote_unit) }}</strong>
                <em>{{ pricingRuleTrialTaxWaterfallNote(pricingRuleTrialResult) }}；税率来源：{{ pricingRuleTrialTaxSourceLabel(pricingRuleTrialResult) }}</em>
              </div>
              <template v-if="pricingRuleTrialHasRoundingAdjustment(pricingRuleTrialResult)">
                <span class="pricing-rule-trial-operator">+</span>
                <div class="pricing-rule-trial-waterfall-card">
                  <small>取整调整</small>
                  <strong>{{ trialMoneyDisplay(pricingRuleTrialResult.rounding_adjustment, pricingRuleTrialResult.quote_unit) }}</strong>
                  <em>{{ pricingRuleTrialRoundingSourceLabel(pricingRuleTrialResult) }}</em>
                </div>
              </template>
              <span class="pricing-rule-trial-operator equals">=</span>
              <div class="pricing-rule-trial-waterfall-card final">
                <small>试算单价</small>
                <strong>{{ trialMoneyDisplay(pricingRuleTrialResult.final_unit_price, pricingRuleTrialResult.quote_unit) }}</strong>
                <em>试算结果只读</em>
              </div>
            </div>
            <div v-if="pricingRuleTrialActiveExplanation" class="pricing-rule-trial-explanation-panel">
              <div class="pricing-rule-trial-explanation-head">
                <strong>试算说明：{{ pricingRuleTrialExplanationTitle(pricingRuleTrialActiveExplanation) }}</strong>
                <button class="secondary compact-action" type="button" @click="closePricingRuleTrialExplanation">关闭</button>
              </div>
              <template v-if="pricingRuleTrialActiveExplanation === 'base_cost'">
                <p>标准制造成本来自当前试算商品和 BOM 版本；BOM物料成本已包含原料损耗，标准工序成本来自发布 BOM 冻结的工序成本快照，并按当前试算单位换算。</p>
                <div class="table-wrap compact-table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>类型</th>
                        <th>名称</th>
                        <th>BOM组成</th>
                        <th>原料损耗</th>
                        <th>损耗后用量</th>
                        <th>成本单价</th>
                        <th>折算成本</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="row in pricingRuleTrialBaseCostRows(pricingRuleTrialResult, 'all')" :key="row.key || `${row.type}-${row.name}`">
                        <td>{{ row.type_label || pricingRuleTrialBaseCostTypeLabel(row.type) }}</td>
                        <td>
                          <span>{{ row.name || '-' }}</span>
                          <small v-if="row.description">{{ row.description }}</small>
                        </td>
                        <td>{{ pricingRuleTrialBaseCostRecipeUsage(row) }}</td>
                        <td>{{ pricingRuleTrialBaseCostLossRate(row) }}</td>
                        <td>{{ pricingRuleTrialBaseCostEffectiveUsage(row) }}</td>
                        <td>{{ trialMoneyDisplay(pricingRuleTrialBaseCostUnitCostValue(row), pricingRuleTrialBaseCostUnit(row, pricingRuleTrialResult)) }}</td>
                        <td>{{ trialMoneyDisplay(row.amount, row.unit || pricingRuleTrialResult.quote_unit) }}</td>
                      </tr>
                      <tr v-if="!pricingRuleTrialBaseCostRows(pricingRuleTrialResult, 'all').length">
                        <td colspan="7" class="muted">暂无可展开的标准制造成本明细。</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </template>
              <template v-else-if="pricingRuleTrialActiveExplanation === 'other_cost'">
                <p>其他成本设置位置：价格计算模板编辑区「其他成本」。本次试算抽屉「其他成本」有临时值时，会优先使用本次试算抽屉的明细。</p>
                <div class="table-wrap compact-table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>成本名</th>
                        <th>金额</th>
                        <th>来源</th>
                        <th>设置位置</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="row in pricingRuleTrialOtherCostRows(pricingRuleTrialResult)" :key="row.name">
                        <td>{{ row.name || '-' }}</td>
                        <td>{{ trialMoneyDisplay(row.amount, row.unit || pricingRuleTrialResult.quote_unit) }}</td>
                        <td>{{ pricingRuleTrialSourceDisplay(row.source) }}</td>
                        <td>{{ row.setting_location || pricingRuleTrialOtherCostSettingLocation(row.source) }}</td>
                      </tr>
                      <tr v-if="!pricingRuleTrialOtherCostRows(pricingRuleTrialResult).length">
                        <td colspan="4" class="muted">暂无其他成本；可在价格计算模板编辑区「其他成本」维护，或在本次试算抽屉「其他成本」临时录入。</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </template>
              <template v-else-if="pricingRuleTrialActiveExplanation === 'profit_markup'">
                <p>加价率设置位置：价格计算模板编辑抽屉「加价率」。80% 填写 0.8，按“税前价 = 成本基数 × (1 + 加价率)”计算，最终售价再计算税额和取整；本次试算抽屉「临时加价率」有值时，会覆盖本次计算使用的加价率。</p>
                <dl class="pricing-rule-trial-explanation-grid">
                  <div>
                    <dt>加价方式</dt>
                    <dd>{{ pricingRuleTrialProfitExplanation(pricingRuleTrialResult).method_label || '-' }}</dd>
                  </div>
                  <div>
                    <dt>参数</dt>
                    <dd>{{ pricingRuleTrialProfitRateDisplay(pricingRuleTrialProfitExplanation(pricingRuleTrialResult), pricingRuleTrialResult.quote_unit) }}</dd>
                  </div>
                  <div>
                    <dt>来源</dt>
                    <dd>{{ pricingRuleTrialSourceDisplay(pricingRuleTrialProfitExplanation(pricingRuleTrialResult).source) }}</dd>
                  </div>
                  <div>
                    <dt>加价基数</dt>
                    <dd>{{ trialMoneyDisplay(pricingRuleTrialProfitExplanation(pricingRuleTrialResult).cost_after_yield, pricingRuleTrialResult.quote_unit) }}</dd>
                  </div>
                  <div>
                    <dt>加价增加</dt>
                    <dd>{{ trialMoneyDisplay(pricingRuleTrialProfitExplanation(pricingRuleTrialResult).markup_amount, pricingRuleTrialResult.quote_unit) }}</dd>
                  </div>
                  <div>
                    <dt>加价后价格</dt>
                    <dd>{{ trialMoneyDisplay(pricingRuleTrialProfitExplanation(pricingRuleTrialResult).pre_tax_price, pricingRuleTrialResult.quote_unit) }}</dd>
                  </div>
                </dl>
                <p class="pricing-rule-trial-explanation-formula">{{ pricingRuleTrialProfitExplanation(pricingRuleTrialResult).formula || '暂无加价公式说明。' }}</p>
              </template>
            </div>
            <div class="pricing-rule-trial-result-meta">
              <span>BOM版本：{{ pricingRuleTrialResult.bom_version_no || pricingRuleTrialResult.bom_version_id || '-' }}</span>
              <span>工艺路线：{{ pricingRuleTrialResult.process_route_name || pricingRuleTrialResult.process_route_id || '-' }}</span>
              <span>毛利率：{{ percentDisplay(pricingRuleTrialResult.gross_margin_rate) }}</span>
            </div>
            <div v-if="pricingRuleTrialResult.warnings?.length" class="pricing-rule-trial-warnings">
              <strong>试算警告</strong>
              <ul>
                <li v-for="warning in pricingRuleTrialResult.warnings" :key="warning">{{ warning }}</li>
              </ul>
            </div>
            <div v-if="pricingRuleTrialResult.formula_expression || pricingRuleTrialResult.formula_expression_lines?.length" class="pricing-rule-trial-formula">
              <strong>计算公式</strong>
              <p v-if="pricingRuleTrialResult.formula_expression" class="pricing-rule-trial-formula-main">{{ pricingRuleTrialResult.formula_expression }}</p>
              <ol v-if="pricingRuleTrialResult.formula_expression_lines?.length">
                <li v-for="line in pricingRuleTrialResult.formula_expression_lines" :key="line">{{ line }}</li>
              </ol>
            </div>
            <div class="pricing-rule-trial-base-detail">
              <div class="field-group-head">
                <strong>标准制造成本折算明细</strong>
                <small>BOM物料成本 {{ trialMoneyDisplay(pricingRuleTrialResult.material_unit_cost ?? pricingRuleTrialResult.bom_cost_total, pricingRuleTrialResult.quote_unit) }}；标准工序成本 {{ trialMoneyDisplay(pricingRuleTrialResult.operation_unit_cost ?? pricingRuleTrialResult.operation_cost_total, pricingRuleTrialResult.quote_unit) }}；标准制造成本 {{ trialMoneyDisplay(pricingRuleTrialResult.standard_manufacturing_unit_cost ?? pricingRuleTrialResult.base_cost, pricingRuleTrialResult.quote_unit) }}</small>
              </div>
              <div class="pricing-rule-trial-detail-group">
                <strong>物料成本明细</strong>
                <div class="table-wrap compact-table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>类型</th>
                        <th>名称</th>
                        <th>BOM组成</th>
                        <th>原料损耗</th>
                        <th>损耗后用量</th>
                        <th>成本单价</th>
                        <th>折算成本</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="row in pricingRuleTrialBaseCostRows(pricingRuleTrialResult, 'material')" :key="row.key || row.name">
                        <td>{{ row.type_label || pricingRuleTrialBaseCostTypeLabel(row.type) }}</td>
                        <td>
                          <span>{{ row.name || '-' }}</span>
                          <small v-if="row.description">{{ row.description }}</small>
                        </td>
                        <td>{{ pricingRuleTrialBaseCostRecipeUsage(row) }}</td>
                        <td>{{ pricingRuleTrialBaseCostLossRate(row) }}</td>
                        <td>{{ pricingRuleTrialBaseCostEffectiveUsage(row) }}</td>
                        <td>{{ trialMoneyDisplay(pricingRuleTrialBaseCostUnitCostValue(row), pricingRuleTrialBaseCostUnit(row, pricingRuleTrialResult)) }}</td>
                        <td>{{ trialMoneyDisplay(row.amount, row.unit || pricingRuleTrialResult.quote_unit) }}</td>
                      </tr>
                      <tr v-if="!pricingRuleTrialBaseCostRows(pricingRuleTrialResult, 'material').length">
                        <td colspan="7" class="muted">暂无物料成本明细。</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
              <div class="pricing-rule-trial-detail-group">
                <strong>工序成本明细</strong>
                <div class="table-wrap compact-table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>类型</th>
                        <th>名称</th>
                        <th>成本来源</th>
                        <th>计费口径</th>
                        <th>成本率</th>
                        <th>金额</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="row in pricingRuleTrialBaseCostRows(pricingRuleTrialResult, 'operation')" :key="row.key || row.name">
                        <td>{{ row.type_label || pricingRuleTrialBaseCostTypeLabel(row.type) }}</td>
                        <td>
                          <span>{{ row.name || '-' }}</span>
                          <small v-if="row.description">{{ row.description }}</small>
                          <small v-if="row.warning" class="warning-text">{{ row.warning }}</small>
                        </td>
                        <td>{{ pricingRuleTrialCapacitySourceLabel(row) }}</td>
                        <td>{{ pricingRuleTrialBaseCostUsage(row) }}</td>
                        <td>{{ trialMoneyDisplay(row.unit_cost, row.unit || pricingRuleTrialResult.quote_unit) }}</td>
                        <td>{{ trialMoneyDisplay(row.amount, row.unit || pricingRuleTrialResult.quote_unit) }}</td>
                      </tr>
                      <tr v-if="!pricingRuleTrialBaseCostRows(pricingRuleTrialResult, 'operation').length">
                        <td colspan="6" class="muted">暂无工序成本明细。</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
            <div class="field-group-head">
              <strong>公式步骤</strong>
              <small>试算结果不写入商品价格表、发布快照或订单。</small>
            </div>
            <div class="table-wrap compact-table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>步骤</th>
                    <th>说明</th>
                    <th>数值</th>
                    <th>临时覆盖</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="step in pricingRuleTrialResult.steps || []" :key="step.key">
                    <td>{{ step.label || step.key }}</td>
                    <td>{{ pricingRuleTrialStepSourceDisplay(step) }}</td>
                    <td>{{ pricingRuleTrialStepDisplay(step) }}</td>
                    <td>{{ step.changed ? '是' : '否' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
        <div class="drawer-footer pricing-rule-trial-footer">
          <div>
            <strong v-if="pricingRuleTrialUpdateMessage" class="pricing-rule-trial-update-message">{{ pricingRuleTrialUpdateMessage }}</strong>
            <small>只更新加价率、已填写税率和其他成本；商品、BOM、路线与销售规格不写入模板。</small>
          </div>
          <button class="primary" type="button" :disabled="productPriceSaving || !pricingRuleTrialForm.pricing_rule_id" @click="updatePricingRuleFromTrial">更新参数到价格计算模板</button>
        </div>
      </aside>
    </div>

    <div v-if="productDrawerOpen" class="settings-drawer-mask" @click.self="closeProductDrawer">
      <aside class="settings-drawer product-editor-drawer" aria-label="创建新商品档案">
        <div class="drawer-head">
          <div>
            <h3>创建新商品档案</h3>
            <p>当前归属：{{ selectedSkuContextLabel }}。配方、包装、生产方式、库存对象或成本口径变化时使用。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeProductDrawer">关闭</button>
        </div>
        <div class="drawer-body">
          <form class="sku-create-form product-create-form product-drawer-form" @submit.prevent="createSku">
            <label class="wide-field">
              <span>商品名称</span>
              <input v-model.trim="skuForm.name" placeholder="如 盒装速溶 10条/盒" />
            </label>
            <label class="wide-field">
              <span>备注</span>
              <textarea v-model.trim="skuForm.remark" rows="2" placeholder="如 原料规格、包装说明或客户要求"></textarea>
            </label>
            <p class="muted wide-field new-product-bom-spec-hint">建档后请到 BOM 创建至少一个规格，并维护该规格的库存单位与完整配方；商品档案不再维护销售规格模板或派生子 SKU。</p>
            <div class="form-actions">
              <button class="primary" type="submit" :disabled="skuSaving">创建新商品档案</button>
            </div>
          </form>
        </div>
      </aside>
    </div>

    <div v-if="customerAliasCreateDrawerOpen" class="settings-drawer-mask" @click.self="closeCustomerAliasCreateDrawer">
      <aside class="settings-drawer customer-alias-create-drawer" aria-label="新建客户商品">
        <div class="drawer-head">
          <div>
            <h3>新建客户商品</h3>
            <p>{{ aliasCustomerLabel }}</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeCustomerAliasCreateDrawer">关闭</button>
        </div>
        <div class="drawer-body">
          <div class="customer-alias-create-mode-tabs">
            <button :class="['secondary', 'compact-action', { active: customerAliasCreateMode === 'single' }]" type="button" @click="customerAliasCreateMode = 'single'">单个新增</button>
            <button :class="['secondary', 'compact-action', { active: customerAliasCreateMode === 'batch' }]" type="button" @click="customerAliasCreateMode = 'batch'">批量添加商品档案</button>
          </div>
          <form v-if="customerAliasCreateMode === 'single'" class="customer-alias-form customer-alias-create-form" @submit.prevent="saveCustomerProductAlias">
            <label class="span-2">
              <span>绑定商品档案</span>
              <SearchableSelect
                v-model="customerProductAliasForm.product_id"
                :options="aliasProductOptions"
                :option-label="productOptionLabel"
                :option-meta="productOptionMeta"
                :option-value="optionNumericValue"
                placeholder="选择商品档案"
                empty-text="暂无商品档案" />
            </label>
            <label>
              <span>客户商品</span>
              <input v-model.trim="customerProductAliasForm.display_name" required placeholder="客户对外展示名称" />
            </label>
            <label>
              <span>重命名</span>
              <input v-model.trim="customerProductAliasForm.brand_name" placeholder="留空则使用客户商品" />
            </label>
            <label>
              <span>排序</span>
              <input v-model.number="customerProductAliasForm.sort_order" type="number" min="0" step="1" />
            </label>
            <label class="checkbox-row">
              <input v-model="customerProductAliasForm.active" type="checkbox" />
              <span>启用</span>
            </label>
            <label class="span-2">
              <span>备注</span>
              <textarea v-model.trim="customerProductAliasForm.remark" rows="2" placeholder="例如贴牌、客户命名、展示用途"></textarea>
            </label>
            <div class="form-actions span-2">
              <button class="primary" type="submit" :disabled="aliasSaving || loading">保存客户商品</button>
            </div>
          </form>
          <div v-else class="customer-alias-batch-mode">
            <div class="alias-batch-list-filters">
              <label>
                <span>搜索</span>
                <input v-model.trim="aliasBatchFilters.query" placeholder="商品名称/编号" />
              </label>
            </div>
            <div class="alias-batch-toolbar">
              <span class="muted">已选 {{ selectedAliasBatchProductIds.length }} 个；同客户同商品档案已存在时自动跳过。</span>
              <button class="secondary compact-action" type="button" @click="toggleAllAliasBatchProducts(true)">全选当前筛选</button>
              <button class="secondary compact-action" type="button" @click="toggleAllAliasBatchProducts(false)">清空选择</button>
            </div>
            <div class="table-wrap">
              <table class="customer-alias-table alias-batch-table">
                <thead>
                  <tr>
                    <th class="select-col">选择</th>
                    <th>商品档案</th>
                    <th>商品编号</th>
                    <th>状态</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="product in aliasBatchCandidateRows" :key="product.id">
                    <td class="select-col">
                      <input type="checkbox" :checked="isAliasBatchProductSelected(product)" :disabled="aliasBatchProductExists(product)" @change="toggleAliasBatchProduct(product, $event.target.checked)" />
                    </td>
                    <td>{{ product.name }}</td>
                    <td>{{ product.number || product.id }}</td>
                    <td>{{ aliasBatchProductExists(product) ? '已存在' : '可添加' }}</td>
                  </tr>
                  <tr v-if="!aliasBatchCandidateRows.length">
                    <td colspan="4" class="muted">没有匹配的商品档案。</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div v-if="customerAliasCreateMode === 'batch'" class="drawer-footer">
          <span class="muted">批量创建时客户商品=商品档案名称，客户商品编号由系统生成；需要客户侧改名后再点客户商品填写“重命名”。</span>
          <button class="primary" type="button" :disabled="aliasBatchSaving || !selectedAliasBatchProductIds.length" @click="saveCustomerAliasBatch">
            {{ aliasBatchSaving ? '添加中' : '批量创建客户商品' }}
          </button>
        </div>
      </aside>
    </div>

    <div v-if="aliasIndustryFieldDrawerOpen" class="settings-drawer-mask" @click.self="closeAliasIndustryFieldDrawer">
      <aside class="settings-drawer alias-industry-field-drawer" aria-label="客户行业字段">
        <div class="drawer-head">
          <div>
            <h3>客户行业字段</h3>
            <p>{{ aliasIndustryFieldAlias?.display_name || '客户商品' }}</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeAliasIndustryFieldDrawer">关闭</button>
        </div>
        <div class="drawer-body product-drawer-form">
          <label v-for="field in aliasIndustryFieldForm.fields" :key="field.field_key">
            <span>{{ field.label || field.field_key }}</span>
            <select v-if="String(field.field_type || '').toLowerCase() === 'select'" v-model="field.value_text">
              <option value="">继承商品档案值</option>
              <option v-for="option in industryFieldOptions(field)" :key="option" :value="option">{{ option }}</option>
            </select>
            <input v-else v-model.trim="field.value_text" placeholder="留空则继承商品档案值" />
          </label>
          <p v-if="!aliasIndustryFieldForm.fields.length" class="muted span-2">绑定商品档案暂无行业字段模板。</p>
        </div>
        <div class="drawer-footer">
          <button class="primary" type="button" :disabled="aliasIndustryFieldSaving" @click="saveAliasIndustryFields">{{ aliasIndustryFieldSaving ? '保存中' : '保存客户行业字段' }}</button>
        </div>
      </aside>
    </div>

    <div v-if="productProductionConfigDrawerOpen" class="settings-drawer-mask" @click.self="closeProductProductionConfigDrawer">
      <aside class="settings-drawer product-production-config-drawer" aria-label="商品档案配置">
        <div class="drawer-head">
          <div>
            <h3>商品档案配置</h3>
            <p>{{ productProductionConfigProduct?.name || '商品档案' }}</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeProductProductionConfigDrawer">关闭</button>
        </div>
        <div class="drawer-body product-production-config-body">
          <section v-if="!Number(productProductionConfigProduct?.parent_product_id || 0)" class="drawer-section bom-spec-migration-panel">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>BOM 规格迁移</strong>
                <small>只迁入旧规格元数据供核对，不自动生成配方，也不会自动切换商品。</small>
              </div>
              <span class="template-meta-chip">{{ productBomSpecMigrationStateLabel }}</span>
            </div>
            <div class="inline-actions migration-actions">
              <button v-if="productBomSpecMigrationState === 'legacy'" class="secondary compact-action" type="button" :disabled="productSpecMigrationSaving" @click="prepareProductBomSpecMigration">迁入规格元数据</button>
              <button v-if="productBomSpecMigrationState === 'preparing' || productBomSpecMigrationState === 'ready'" class="secondary compact-action" type="button" :disabled="productSpecMigrationSaving" @click="assessProductBomSpecMigration">重新检查切换条件</button>
              <button v-if="productBomSpecMigrationState === 'ready'" class="primary compact-action" type="button" :disabled="productSpecMigrationSaving" @click="cutoverProductBomSpecs">确认切换到 BOM 规格</button>
              <button class="text-button" type="button" @click="navigateProductBom(productProductionDefaultBomUsageRow || {})">到 BOM 重建并发布完整配方</button>
            </div>
            <div v-if="productBomSpecMigrationBlockers.length" class="warning-banner migration-blockers">
              <strong>当前不能切换</strong>
              <ul>
                <li v-for="blocker in productBomSpecMigrationBlockers" :key="blocker.code">{{ blocker.message || blocker.code }}（{{ blocker.count }}）</li>
              </ul>
            </div>
            <div v-if="productBomSpecMigrationMappings.length" class="table-wrap compact">
              <table>
                <thead><tr><th>旧规格</th><th>BOM 规格映射</th><th>历史配置对照</th></tr></thead>
                <tbody>
                  <tr v-for="mapping in productBomSpecMigrationMappings" :key="mapping.id || mapping.legacy_child_product_id">
                    <td>{{ mapping.legacy_spec_name || mapping.legacy_spec_key || `旧规格 #${mapping.legacy_child_product_id}` }}</td>
                    <td>{{ Number(mapping.bom_spec_id || 0) > 0 ? `已匹配 BOM 规格 #${mapping.bom_spec_id}` : '待在 BOM 中重建并按规格键匹配' }}</td>
                    <td><details><summary>查看旧损耗、备注、BOM 与路线</summary><pre>{{ productBomSpecMigrationSnapshot(mapping) }}</pre></details></td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
          <section class="drawer-section">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>基础信息</strong>
                <small>商品档案是库存、成本、生产和成品批次对象；客户对外名称在客户引用维护。</small>
              </div>
            </div>
            <div class="production-config-grid">
              <label>
                <span>商品名</span>
                <input v-model.trim="productProductionConfigForm.name" :disabled="!canEditSkuRow(productProductionConfigProduct || {})" placeholder="商品档案名称" />
              </label>
              <label class="wide-field">
                <span>备注</span>
                <textarea v-model.trim="productProductionConfigForm.remark" rows="2" placeholder="商品档案备注"></textarea>
              </label>
              <label class="wide-field">
                <span>商品规格</span>
                <input :value="productProductionBomSpecsSummary" disabled />
                <small>规格统一来自默认制造 BOM 的规格组，到 BOM 维护；商品档案不再单独选择规格模板。</small>
              </label>
            </div>
            <div class="sales-spec-template-detail bom-spec-readonly-panel">
              <div class="sales-spec-template-detail-head">
                <div>
                  <strong>BOM 规格（只读）</strong>
                  <small>规格、单位和配方统一在默认制造 BOM 的规格组维护；商品档案直接引用并展示。</small>
                </div>
                <button class="secondary compact-action" type="button" @click="navigateProductBom(productProductionDefaultBomUsageRow || {})">到 BOM 维护规格</button>
              </div>
              <article v-for="row in productProductionBomSpecs" :key="`bom-spec-${row.bom_spec_id}`" class="child-sku-row compact-derived-sku-row">
                <div>
                  <strong>{{ row.name || `规格 #${row.bom_spec_id}` }}</strong>
                  <small>规格编码：{{ row.code || '-' }}</small>
                  <small v-if="row.barcode">条码：{{ row.barcode }}</small>
                  <small>生产单位：{{ row.unit || '-' }}</small>
                </div>
                <span v-if="row.is_default" class="template-meta-chip default-spec-chip">默认规格</span>
              </article>
              <p v-if="!productProductionBomSpecs.length" class="muted">{{ productProductionDefaultBomUsageRow ? '默认 BOM 暂无可用规格，请到 BOM 检查规格组和发布版本。' : '该商品尚未绑定默认制造 BOM；请到 生产配置 -> 生产 BOM 创建规格组并设为默认 BOM。' }}</p>
            </div>
          </section>

          <section class="drawer-section">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>可生产该商品的 BOM</strong>
                <small>这里展示产出该商品的生产 BOM；默认 BOM 按商品保存，生产计划、试算和新工单会优先读取。</small>
              </div>
            </div>
            <div class="production-config-grid reverse-bom-grid single-column">
              <div class="readonly-link-list">
                <div
                  v-for="row in productProductionConfigProduceBomRows"
                  :key="bomUsageRowKey(row)"
                  class="bom-default-row">
                  <button
                    class="text-button readonly-link-button"
                    type="button"
                    @click="navigateProductBom({ production_bom_id: bomUsageBomID(row), id: bomUsageBomID(row), name: row.bom_name })">
                    <span>{{ bomUsageRelationLabel(row) }}</span>
                    <small :class="['bom-usage-status', bomUsageStatusClass(row)]">BOM状态：{{ bomUsageStatusLabel(row) }}</small>
                    <small>BOM版本：{{ bomUsageVersionLabel(row) }}</small>
                  </button>
                  <button
                    class="secondary compact-action"
                    type="button"
                    :disabled="row.is_default || !row.can_set_default || productProductionConfigSaving"
                    @click="setDefaultProductionBom(row)">
                    {{ row.is_default ? '默认 BOM' : '设为默认' }}
                  </button>
                </div>
                <small v-if="!productProductionConfigProduceBomRows.length" class="muted">暂无可生产该商品的 BOM</small>
              </div>
            </div>
          </section>

          <section class="drawer-section">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>作为组件被哪些 BOM 使用</strong>
                <small>这里只读查看把当前商品当作组件消耗的生产 BOM，不参与当前商品默认 BOM 设置。</small>
              </div>
            </div>
            <div class="production-config-grid reverse-bom-grid single-column">
              <div class="readonly-link-list">
                <button
                  v-for="row in productProductionConfigUsedByBomRows"
                  :key="bomUsageRowKey(row)"
                  class="text-button readonly-link-button"
                  type="button"
                  @click="navigateProductBom({ production_bom_id: bomUsageBomID(row), id: bomUsageBomID(row), name: row.bom_name })">
                  <span>{{ bomUsageRelationLabel(row) }}</span>
                  <small :class="['bom-usage-status', bomUsageStatusClass(row)]">BOM状态：{{ bomUsageStatusLabel(row) }}</small>
                </button>
                <small v-if="!productProductionConfigUsedByBomRows.length" class="muted">暂无 BOM 把该商品作为组件</small>
              </div>
            </div>
          </section>

          <section class="drawer-section production-config-fields">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>行业字段</strong>
                <small>字段定义来自行业字段模板；这里只填写字段值和是否在价格表展示。</small>
              </div>
              <div class="field-group-actions industry-template-selector">
                <label v-for="template in productProductionConfigIndustryTemplateOptions" :key="template.id" class="checkline industry-template-option" :class="{ unavailable: template.unavailable }">
                  <input
                    v-model="productProductionConfigForm.industry_field_template_ids"
                    type="checkbox"
                    :value="Number(template.id || 0)"
                    @change="applyIndustryFieldTemplateToProductionConfig" />
                  <span>{{ industryFieldTemplateOptionLabel(template) }}</span>
                </label>
                <small v-if="productProductionConfigIndustryTemplateOptions.length" class="muted industry-template-priority-hint">勾选顺序决定同名字段优先级；取消后重新勾选可调整顺序。</small>
                <small v-if="!productProductionConfigIndustryTemplateOptions.length" class="muted">暂无可用行业字段模板</small>
              </div>
            </div>
            <div v-for="(field, index) in productProductionConfigForm.fields" :key="field.local_id" class="production-config-field-row">
              <div class="field-definition-readonly">
                <strong>{{ field.label || field.field_key }}</strong>
                <small>{{ fieldTypeLabel(field.field_type) }}{{ field.unit ? ` / ${field.unit}` : '' }}</small>
              </div>
              <label v-if="field.field_type === 'number' || field.field_type === 'ratio'">
                <span>字段值</span>
                <input v-model.number="field.value_number" type="number" step="0.0001" />
              </label>
              <label v-else-if="field.field_type === 'checkbox'" class="checkline field-bool-value">
                <input v-model="field.value_bool" type="checkbox" />
                <span>字段值</span>
              </label>
              <label v-else-if="field.field_type === 'select'">
                <span>字段值</span>
                <select v-model="field.value_text">
                  <option value="">未选择</option>
                  <option v-for="option in fieldOptions(field)" :key="option" :value="option">{{ option }}</option>
                </select>
              </label>
              <label v-else-if="field.field_type === 'textarea'" class="wide-field">
                <span>字段值</span>
                <textarea v-model.trim="field.value_text" rows="2" placeholder="字段值"></textarea>
              </label>
              <label v-else-if="field.field_type === 'date'">
                <span>字段值</span>
                <input v-model="field.value_text" type="date" />
              </label>
              <label v-else>
                <span>字段值</span>
                <input v-model.trim="field.value_text" placeholder="字段值" />
              </label>
              <label class="checkline">
                <input v-model="field.show_in_price_list" type="checkbox" />
                <span>价格表展示</span>
              </label>
              <label>
                <span>排序</span>
                <input v-model.number="field.sort_order" type="number" min="0" step="1" />
              </label>
            </div>
            <p v-if="!productProductionConfigForm.fields.length" class="muted">暂无产品信息字段。请选择行业字段模板后填写字段值。</p>
          </section>
        </div>
        <div class="drawer-footer">
          <span class="muted">保存后价格表、录单、生产计划和新工单会读取这份商品档案配置。</span>
          <button class="primary" type="button" :disabled="productProductionConfigSaving" @click="saveProductProductionConfig">
            {{ productProductionConfigSaving ? '保存中' : '保存商品档案配置' }}
          </button>
        </div>
      </aside>
    </div>

    <div v-if="globalUnitDrawerOpen" class="settings-drawer-mask" @click.self="closeGlobalUnitDictionaryDrawer">
      <aside class="settings-drawer global-unit-dictionary-drawer" aria-label="全局单位字典设置">
        <div class="drawer-head">
          <div>
            <h3>全局单位字典</h3>
            <p>维护 kg、盒、箱等基础单位；销售规格模板会引用这些单位作为库存单位和规格换算单位。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeGlobalUnitDictionaryDrawer">关闭</button>
        </div>
        <div class="drawer-body global-unit-drawer-body">
          <section class="unit-template-card global-unit-list-panel">
            <div class="field-group-head">
              <strong>基础单位</strong>
              <small>点击单位后在右侧编辑。</small>
            </div>
            <div class="unit-chip-list global-unit-chip-list">
              <button
                v-for="unit in visibleProductUnitDefinitions"
                :key="unit.code"
                class="unit-chip global-unit-chip"
                :class="{ inactive: unit.active === false }"
                type="button"
                @click="startGlobalUnitDefinitionEdit(unit)">
                <strong>{{ unit.name || unit.code }}</strong>
                <small>{{ unit.code }} · {{ unitTypeLabel(unit.unit_type) }} · {{ unit.allow_decimal ? '允许小数' : '整数优先' }}</small>
              </button>
              <p v-if="!visibleProductUnitDefinitions.length" class="muted">暂无单位，直接在右侧表单填写并保存。</p>
            </div>
          </section>

          <section class="unit-template-card global-unit-editor-panel">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>{{ globalUnitEditingCode ? '编辑基础单位' : '新增基础单位' }}</strong>
                <small>保存后刷新列表并回到空白表单。</small>
              </div>
              <button class="secondary compact-action" type="button" @click="resetGlobalUnitDefinitionForm">新增基础单位</button>
            </div>
            <form class="unit-definition-form global-unit-definition-form" @submit.prevent="saveGlobalUnitDefinitionFromDrawer">
              <label>
                <span>单位编码</span>
                <input v-model.trim="globalUnitForm.code" :disabled="Boolean(globalUnitEditingCode)" placeholder="box" />
              </label>
              <label>
                <span>单位名称</span>
                <input v-model.trim="globalUnitForm.name" placeholder="盒" />
              </label>
              <label>
                <span>单位类型</span>
                <select v-model="globalUnitForm.unit_type">
                  <option value="weight">重量</option>
                  <option value="package">包装</option>
                  <option value="count">数量</option>
                  <option value="other">其他</option>
                </select>
              </label>
              <label class="checkline">
                <input v-model="globalUnitForm.allow_decimal" type="checkbox" />
                <span>允许小数</span>
              </label>
              <label class="checkline">
                <input v-model="globalUnitForm.active" type="checkbox" />
                <span>启用</span>
              </label>
              <div class="form-actions">
                <button v-if="globalUnitEditingCode" class="text-button danger-text" type="button" :disabled="globalUnitSaving || loading" @click="deleteGlobalUnitDefinitionFromDrawer">删除</button>
                <button class="primary" type="submit" :disabled="globalUnitSaving || loading">
                  {{ globalUnitSaving ? '保存中' : (globalUnitEditingCode ? '保存' : '新增') }}
                </button>
              </div>
            </form>
          </section>
        </div>
      </aside>
    </div>

    <div v-if="productGroupTemplateDrawerOpen" class="settings-drawer-mask" @click.self="closeProductGroupTemplateDrawer" @keydown.esc.stop.prevent="closeProductGroupTemplateDrawer">
      <aside class="settings-drawer product-group-template-drawer" aria-label="商品档案分组模板设置">
        <div class="drawer-head">
          <div>
            <strong>分组模板设置</strong>
            <p>选择商品档案引用的分组模板，已包含商品的模板不可取消。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeProductGroupTemplateDrawer">关闭</button>
        </div>
        <div class="drawer-body">
          <div v-if="deletedProductGroupTemplateWarnings.length" class="product-group-template-deleted-warnings">
            <div v-for="warn in deletedProductGroupTemplateWarnings" :key="warn.id" class="warning-banner">
              <strong>{{ warn.name }}</strong> 的分组或分类已删除，相关商品已自动移至未分类。
            </div>
          </div>
          <div v-if="selectableProductGroupTemplates.length" class="product-group-template-list">
            <label v-for="template in selectableProductGroupTemplates" :key="template.id" class="checkline product-group-template-option" :class="{ 'has-items': productGroupTemplateHasItems(Number(template.id || 0)) }">
              <input
                type="checkbox"
                :checked="productGroupFeatureSelectionDraft.includes(Number(template.id || 0))"
                :disabled="productGroupFeatureSelectionSaving || loading || productGroupTemplateHasItems(Number(template.id || 0))"
                @change="toggleProductGroupTemplate(Number(template.id || 0))" />
              <span>{{ businessGroupDisplayName(template) }}</span>
              <small v-if="productGroupTemplateHasItems(Number(template.id || 0))" class="muted">已包含商品，不可取消</small>
            </label>
          </div>
          <div v-else class="muted">暂无可选分组模板，请先新增模板。</div>
          <div class="drawer-actions">
            <button class="secondary compact-action" type="button" :disabled="productGroupFeatureSelectionSaving || loading" @click="openProductBusinessGroupManagement">维护分组模板</button>
            <button class="primary compact-action" type="button" :disabled="productGroupFeatureSelectionSaving || loading || !productGroupFeatureSelectionHasChanges" @click="saveAndCloseProductGroupTemplateDrawer">
              {{ productGroupFeatureSelectionSaving ? '保存中' : '保存' }}
            </button>
          </div>
        </div>
      </aside>
    </div>

  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import BusinessGroupInlineWorkspace from '../components/BusinessGroupInlineWorkspace.vue'
import PaginationControls from '../components/PaginationControls.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import {
  businessGroupFeatureSelectionIDs,
  businessGroupFeatureSelectionPayload,
  businessGroupControlOptions,
  businessGroupInlineListState,
  businessGroupRowsForFeatureSelection,
  businessGroupMoveAssignmentPayload,
  businessGroupVisibleRows,
  groupRowsByBusinessGroupTemplates,
} from '../lib/business-grouping'
import { FORM_DRAFT_SCOPES, readFormDraft, saveFormDraft } from '../lib/form-draft-cache'
import {
  buildGradientTemplatePayload,
  gradientDisplayQuantityStep,
  gradientDisplayQuantityUnitLabel,
  gradientDisplayUnitOptions as baseGradientDisplayUnitOptions,
  gradientDisplayUnitLabel,
  normalizeGradientTemplate,
  validateGradientTemplate,
} from '../lib/gradient-templates'
import {
  buildCustomerPublicUsagePayload,
  buildCustomerProductAliasIndustryFieldPayload,
  buildCustomerProductAliasBatchPayload,
  buildCustomerProductAliasPayload,
  customerAliasEffectiveDisplayName,
  activeProductionBomOptions,
  buildClassificationTemplateUsagePayload,
  classificationAssignmentForRow,
  classificationAssignmentLabel,
  classificationTemplateUnitPriceWarnings,
  classificationTemplateTabs,
  buildCustomerProductRuleBindingPayload,
  buildCustomerProductRuleOverridePayload,
  buildCustomerProductRuleTemplatePayload,
  buildBusinessGroupAssignmentPayload,
  businessGroupItemsTree,
  businessGroupVisibleName,
  buildCustomProductCreatePayload,
  buildProductCategoryConfigPayload,
  buildProductConfigTemplatePayload,
  buildPriceTierTemplatePayload,
  buildProductPriceRecordPayload,
  buildProductTierPriceSchemePayload,
  buildPricingRulePayload,
  buildPricingRuleCopyPayload,
  buildPricingRuleUpdateFromTrial,
  buildPricingRuleTrialPayload,
  buildProductProductionConfigField,
  buildProductProductionConfigForm,
  industryFieldTemplateIDsFromConfig,
  industryFieldTemplateOptionsForConfig,
  productProductionConfigFieldsFromTemplates,
  buildProductProductionConfigBasicsPayload,
  buildProductUnitDefinitionPayload,
  buildProductUnitTemplatePayload,
  buildProductBasicsPayload,
  buildProductCreatePayload,
  buildAssignCategoryPayload,
  buildChildSkuCreatePayload,
  buildSkuContextCategoryTree,
  categoryBelongsToSkuContext as categoryBelongsToContext,
  categoryDisplayState,
  customerSkuCustomerOptions,
  customerProductAliasRowsForCustomer,
  filterSkuRows,
  groupRowsByClassificationCategory,
  industryFieldSummary,
  gradientTemplateBelongsToSkuContext,
  integerUnitModeOptions,
  inferProductKindFromProductTypeCategory,
  isPublicReferenceRow,
  nextSkuContextCustomerID,
  normalizePricingRuleCostSourceMode,
  normalizeVisibleSkuFilters,
  normalizedProductKind,
  priceListRuleFormFromJSON,
  priceListRulePricingModeOptions,
  priceListRuleRoundingOptions,
  productBelongsToSkuContext as productBelongsToContext,
  productConfigTemplateBelongsToSkuContext,
  productConfigTemplateNeedsGradientTemplate,
  productCategoryAssignmentLabel,
  productDisplayState,
  productPriceRecordLabel,
  productKindSupportsBomParams,
  productCodeLabel,
  primaryCategoryOptions,
  pricingRuleTrialDefaultProductSpecID,
  pricingRuleTrialMainProductOptions,
  pricingRuleTrialProductSpecLabel,
  pricingRuleTrialProductSpecOptions,
  pricingRuleTrialProductSpecUnit,
  productSkuRowsForParent,
  productArchiveRowsWithSkus,
  productionBomOptionLabel,
  resolveCreatedProductForConfig,
  salesSpecRowsFromTemplate,
  secondaryCategoryOptions,
  selectedSkuRowIDsAfterVisibleToggle,
  storePricingRuleTrialReturnState,
  takePricingRuleTrialReturnState,
  productSubtypeCategoryOptionsForType,
  specialAttrValuesFromJSON,
  sortRowsForCustomerSkuPriority,
  skuTypeLabel,
  skuTypeOptions,
  unitConversionRowsFromJSON,
  unitRuleFormFromJSON,
  visibleNonDeletedRows,
} from '../lib/product-settings'
import { orderProductFamilyOptions, orderProductKindFilterOptions } from '../lib/order-entry'
import { normalizePageSize } from '../lib/pagination'
import {
  isProductBomSpecCutover,
  normalizeProductBomSpecs,
  productSpecMigrationState,
  visibleRowsForProductSpecMigration,
} from '../lib/product-spec-cutover'
import { replaceHistoryURL } from '../lib/url-state'
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'

const props = defineProps({
  sectionMode: { type: String, default: '' },
  workspaceMode: { type: String, default: '' },
  viewContext: { type: Object, default: () => ({}) },
  viewParams: { type: Object, default: () => ({}) },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})
const SKU_SETTINGS_FORM_DRAFT_SCOPE = FORM_DRAFT_SCOPES.skuSettings
const UNCLASSIFIED_CATEGORY_MOVE_ID = -999999
const DEFAULT_SKU_GROUP_PAGE_SIZE = 10
let restoringProductSettingsDraft = false

const categories = ref([])
const products = ref([])
const gradientTemplates = ref([])
const productConfigTemplates = ref([])
const productClassificationTemplates = ref([])
const productClassificationTemplateUsages = ref([])
const aliasClassificationTemplateUsages = ref([])
const businessGroups = ref([])
const businessGroupAssignments = ref([])
const productGroupFeatureSelectionIDs = ref([])
const productGroupFeatureSelectionDraft = ref([])
const productProductionConfigs = ref([])
const industryFieldTemplates = ref([])
const productUnitDefinitions = ref([])
const productUnitTemplates = ref([])
const productPriceGroups = ref([])
const productPriceRecords = ref([])
const productTierPriceSchemes = ref([])
const pricingRules = ref([])
const priceTierTemplates = ref([])
const pricingRuleEditorDrawerOpen = ref(false)
const pricingRuleEditorDrawer = ref(null)
const pricingRuleTrialDrawerOpen = ref(false)
const pricingRuleTrialLoading = ref(false)
const pricingRuleTrialRule = ref(null)
const pricingRuleTrialForm = ref(defaultPricingRuleTrialForm())
const activePricingRuleTrialProductKindFilter = ref('')
const pricingRuleTrialResult = ref(null)
const pricingRuleTrialActiveExplanation = ref('')
const pricingRuleTrialError = ref('')
const pricingRuleTrialUpdateMessage = ref('')
let pricingRuleTrialAutoRunTimer = null
let pricingRuleTrialRunID = 0
let restoringPricingRuleTrialReturnState = false
let pricingRuleEditorReturnFocus = null
const customerPublicUsages = ref([])
const customerProductAliases = ref([])
const customerProductRuleTemplates = ref([])
const customerProductRuleOverrides = ref([])
const customerProductRuleBindings = ref([])
const productionBoms = ref([])
const productionBomDetails = ref({})
const productBomUsageByProductID = ref({})
const productSpecMigrationByProductID = ref({})
const processRoutes = ref([])
const customers = ref([])
const loading = ref(false)
const skuSaving = ref(false)
const productSaving = skuSaving
const customSaving = skuSaving
const templateSaving = ref(false)
const productConfigSaving = ref(false)
const productUnitSaving = ref(false)
const productPriceSaving = ref(false)
const classificationTemplateSaving = ref(false)
const globalUnitSaving = ref(false)
const customerRuleSaving = ref(false)
const aliasSaving = ref(false)
const aliasBatchSaving = ref(false)
const error = ref('')
const ok = ref('')
const dragging = ref(null)
const pointerDrag = ref(null)
const categoryDropTarget = ref(null)
const editingCategoryId = ref(0)
const editingCategoryName = ref('')
const categoryCollapsed = ref(false)
const collapsedPrimaryCategoryIds = ref([])
const collapsedSecondaryCategoryIds = ref([])
const productsCollapsed = ref(false)
const activeSettingsSection = ref('master')
const activeConfigTemplateSection = ref('product-config')
const PRODUCT_SECTION_MODES = {
  productMaster: 'master',
  master: 'master',
  customerProductAliases: 'aliases',
  aliases: 'aliases',
  productConfigTemplates: 'templates',
  productPriceManagement: 'templates',
  productPrices: 'templates',
  templates: 'templates',
  pricingGradientTemplates: 'templates',
  gradient: 'templates',
  productUnitTemplates: 'templates',
  unitTemplate: 'templates',
}
const forcedConfigTemplateSection = computed(() => {
  if (props.sectionMode === 'pricingGradientTemplates' || props.sectionMode === 'gradient') return 'gradient'
  if (props.sectionMode === 'productUnitTemplates' || props.sectionMode === 'unitTemplate') return 'unit-template'
  if (props.sectionMode === 'productPriceManagement' || props.sectionMode === 'productPrices') return 'product-price-management'
  return ''
})
const effectiveConfigTemplateSection = computed(() => forcedConfigTemplateSection.value || activeConfigTemplateSection.value)
const currentSettingsSection = computed(() => PRODUCT_SECTION_MODES[props.sectionMode] || activeSettingsSection.value)
const productReturnNavigation = computed(() => props.viewParams?.return_navigation || null)
const productReturnLabel = computed(() => String(productReturnNavigation.value?.label || '返回上一步'))
const productSectionTitle = computed(() => {
  if (currentSettingsSection.value === 'aliases') return '客户商品'
  if (forcedConfigTemplateSection.value === 'gradient') return '阶梯价模板'
  if (forcedConfigTemplateSection.value === 'unit-template') return '销售规格模板'
  if (forcedConfigTemplateSection.value === 'product-price-management') return '商品价格管理'
  if (currentSettingsSection.value === 'templates') return '商品配置模板'
  return '商品档案'
})
const showGradientTemplatePane = computed(() => currentSettingsSection.value === 'templates' && effectiveConfigTemplateSection.value === 'gradient')
const showUnitTemplatePane = computed(() => currentSettingsSection.value === 'templates' && effectiveConfigTemplateSection.value === 'unit-template')
const showProductPriceManagementPane = computed(() => currentSettingsSection.value === 'templates' && effectiveConfigTemplateSection.value === 'product-price-management')
const productDrawerOpen = ref(false)
const productProductionConfigDrawerOpen = ref(false)
let productProductionConfigOpenGeneration = 0
const customerAliasCreateDrawerOpen = ref(false)
const customerAliasCreateMode = ref('single')
const classificationTemplateCreateDrawerOpen = ref(false)
const globalUnitDrawerOpen = ref(false)
const categorySearchQuery = ref('')
const primaryDeleteMode = ref(false)
const secondaryDeleteModeFor = ref(0)
const selectedCustomerSkuCustomerID = ref(0)
const selectedAliasCustomerID = ref(0)
const selectedProductIds = ref([])
const selectedAliasIds = ref([])
const selectedAliasBatchProductIds = ref([])
const activeProductClassificationTab = ref('all')
const activeAliasClassificationTab = ref('all')
const selectedProductClassificationTemplateID = ref(0)
const selectedAliasClassificationTemplateID = ref(0)
const selectedProductClassificationMoveID = ref(0)
const selectedAliasClassificationMoveID = ref(0)
const selectedProductClassificationCategoryID = ref(0)
const selectedAliasClassificationCategoryID = ref(0)
const selectedProductGroupTemplateID = ref(0)
const productCategoryMoveActive = ref(false)
const productGroupFeatureSelectionSaving = ref(false)
const productGroupTemplateDrawerOpen = ref(false)
const collapsedProductClassificationGroups = ref([])
const collapsedAliasClassificationGroups = ref([])
const skuFilters = ref(defaultSkuFilters())
const aliasFilters = ref({ query: '', active: 'active' })
const skuGroupPagination = ref({})
const publicUsageSaving = ref(false)
const skuForm = ref(defaultSkuForm())
const productForm = skuForm
const customForm = ref(defaultCustomForm())
const highlightedSkuId = ref(0)
const templateForm = ref(defaultGradientTemplateForm())
const productUnitTemplateForm = ref(defaultProductUnitTemplateForm())
const productUnitTemplateInventoryUnitLocked = computed(() => Number(productUnitTemplateForm.value?.id || 0) > 0)
const productPriceRecordForm = ref(defaultProductPriceRecordForm())
const productTierPriceSchemeForm = ref(defaultProductTierPriceSchemeForm())
const pricingRuleForm = ref(defaultPricingRuleForm())
const priceTierTemplateForm = ref(defaultPriceTierTemplateForm())
const globalUnitForm = ref(defaultProductUnitDefinitionForm())
const globalUnitEditingCode = ref('')
const customerRuleTemplateForm = ref(defaultCustomerProductRuleTemplateForm())
const customerRuleOverrideForm = ref(defaultCustomerProductRuleOverrideForm())
const customerProductAliasForm = ref(defaultCustomerProductAliasForm())
const aliasBatchForm = ref(defaultCustomerProductAliasBatchForm())
const aliasBatchFilters = ref(defaultAliasBatchFilters())
const classificationTemplateCreateForm = ref(defaultClassificationTemplateForm())
const classificationCategoryForm = ref(defaultClassificationCategoryForm())
const productProductionConfigProduct = ref(null)
const productProductionConfigForm = ref(defaultProductProductionConfigForm())
const productProductionConfigSaving = ref(false)
const productSpecMigrationSaving = ref(false)
const aliasIndustryFieldDrawerOpen = ref(false)
const aliasIndustryFieldSaving = ref(false)
const aliasIndustryFieldAlias = ref(null)
const aliasIndustryFieldForm = ref({ fields: [] })

const skuContextCustomerID = computed(() => Number(selectedCustomerSkuCustomerID.value || 0))
const productConfigTemplateForm = ref(defaultProductConfigTemplateForm())
const classificationTemplateForm = ref(defaultClassificationTemplateForm())
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && Number(props.customerContextId || 0) > 0)
const selectedSkuContextLabel = computed(() => {
  const customerID = skuContextCustomerID.value
  if (!customerID) return '全部商品'
  return `${customerName(customerID) || `客户 #${customerID}`} 商品`
})
const flatPublicCategories = computed(() => flattenCategoryNodes(categories.value).filter((category) => Number(category.customer_id || 0) === 0))
const flatCustomerCategories = computed(() => flattenCategoryNodes(categories.value).filter((category) => Number(category.customer_id || 0) === skuContextCustomerID.value))
const publicProducts = computed(() => products.value.filter((product) => Number(product.customer_id || 0) === 0))
const customerProductsForContext = computed(() => products.value.filter((product) => Number(product.customer_id || 0) === skuContextCustomerID.value))
const customerGradientTemplatesForContext = computed(() => gradientTemplates.value.filter((template) => Number(template.customer_id || 0) === skuContextCustomerID.value))
const customerProductConfigTemplatesForContext = computed(() => visibleProductConfigTemplates.value.filter((template) => Number(template.customer_id || 0) === skuContextCustomerID.value))
const selectedCustomerPublicUsage = computed(() => {
  const customerID = skuContextCustomerID.value
  return customerPublicUsages.value.find((row) => Number(row.customer_id || 0) === customerID) || {
    customer_id: customerID,
    use_public_sku: false,
    use_public_categories: false,
    use_public_gradient_templates: false,
  }
})
const selectedProductProductionConfigBomDetail = computed(() => productionBomDetails.value[String(productProductionConfigForm.value.production_bom_id || 0)] || null)
const productProductionConfigParentProductID = computed(() => {
  const current = productProductionConfigProduct.value || {}
  return Number(current.parent_product_id || current.parentProductID || productProductionConfigForm.value.parent_product_id || productProductionConfigForm.value.product_id || current.id || 0)
})
const productProductionConfigParentProduct = computed(() => {
  const parentID = productProductionConfigParentProductID.value
  return products.value.find((product) => Number(product.id || 0) === parentID) || productProductionConfigProduct.value || {}
})
const productProductionConfigSkuRows = computed(() => productSkuRowsForParent(products.value, productProductionConfigParentProductID.value))
const productProductionConfigVersionOptions = computed(() => (selectedProductProductionConfigBomDetail.value?.versions || [])
  .filter((version) => version.status === 'published')
  .sort((a, b) => String(b.version_no || '').localeCompare(String(a.version_no || ''))))
const customerUsesPublicCategories = computed(() => Boolean(
  selectedCustomerSkuCustomerID.value && selectedCustomerPublicUsage.value.use_public_categories,
))
const customerUsesPublicSku = computed(() => false)
const customerUsesPublicGradientTemplates = computed(() => Boolean(
  selectedCustomerSkuCustomerID.value && selectedCustomerPublicUsage.value.use_public_gradient_templates,
))
const categoryTreeForSkuContext = computed(() => buildSkuContextCategoryTree(categories.value, {
  customerID: skuContextCustomerID.value,
  usePublicCategories: customerUsesPublicCategories.value,
  usePublicSku: false,
  usePublicSkuInCategoryTree: false,
  publicCategories: flatPublicCategories.value,
  customerCategories: flatCustomerCategories.value,
  publicProducts: publicProducts.value,
  customerProducts: customerProductsForContext.value,
}))
const categoryManagementTreeForSkuContext = computed(() => buildProductCatalogBusinessGroupTree())
const isCategorySearchActive = computed(() => Boolean(categorySearchQuery.value.trim()))
const visibleCategoryTreeForSkuContext = computed(() => filterCategoryTreeByQuery(categoryTreeForSkuContext.value, categorySearchQuery.value))
const visibleCategoryManagementTreeForSkuContext = computed(() => filterCategoryTreeByQuery(categoryManagementTreeForSkuContext.value, categorySearchQuery.value))
const productTypeCategoryOptions = computed(() => categoryTreeForSkuContext.value
  .map((category) => ({
    id: Number(category.id || 0),
    name: category.name || '',
    customer_id: Number(category.customer_id || 0),
    source_category_id: Number(category.source_category_id || 0),
    template_state: category.template_state || '',
  }))
  .filter((category) => category.id > 0 && category.name))
function productSubtypeCategoryOptions(productTypeCategoryID) {
  return productSubtypeCategoryOptionsForType(categoryTreeForSkuContext.value, productTypeCategoryID)
}
const productRows = computed(() => {
  const rows = []
  for (const primary of categoryTreeForSkuContext.value) {
    for (const product of primary.products || []) {
      rows.push(product)
    }
    for (const secondary of primary.children || []) {
      for (const product of secondary.products || []) {
        rows.push({ ...product, primary_name: primary.name, secondary_name: secondary.name })
      }
    }
  }
  for (const product of uncategorizedProducts.value) {
    rows.push(product)
  }
  return rows
})
const activePricingRuleTrialOptions = computed(() => pricingRules.value
  .filter((rule) => rule && rule.active !== false && Number(rule.id || 0) > 0)
  .slice()
  .sort((a, b) => pricingRuleOptionLabel(a).localeCompare(pricingRuleOptionLabel(b))))
const pricingRuleTrialCatalogProducts = computed(() => products.value.filter(skuContextProductFilter))
const pricingRuleTrialMainProducts = computed(() => pricingRuleTrialMainProductOptions(
  pricingRuleTrialCatalogProducts.value,
)
  .slice()
  .sort((a, b) => productOptionLabel(a).localeCompare(productOptionLabel(b))))
const pricingRuleTrialProductKindFilterOptions = computed(() => orderProductKindFilterOptions(pricingRuleTrialMainProducts.value))
const pricingRuleTrialProductOptions = computed(() => orderProductFamilyOptions(
  pricingRuleTrialMainProducts.value,
  '',
  activePricingRuleTrialProductKindFilter.value,
))
const selectedPricingRuleTrialProduct = computed(() => pricingRuleTrialMainProducts.value.find((product) => Number(product.id || 0) === Number(pricingRuleTrialForm.value.parent_product_id || 0)) || null)
const pricingRuleTrialSalesSpecOptions = computed(() => pricingRuleTrialProductSpecOptions(
  pricingRuleTrialCatalogProducts.value,
  pricingRuleTrialForm.value.parent_product_id,
))
const selectedPricingRuleTrialProductSpec = computed(() => pricingRuleTrialSalesSpecOptions.value.find((product) => (
  Number(product?.sku_id || product?.id || 0) === Number(pricingRuleTrialForm.value.product_id || 0)
)) || null)
const pricingRuleTrialBomVersionOptions = computed(() => Array.isArray(pricingRuleTrialResult.value?.bom_version_options) ? pricingRuleTrialResult.value.bom_version_options : [])
const selectedPricingRuleTrialBomVersion = computed(() => {
  const versionID = Number(pricingRuleTrialForm.value.bom_version_id || pricingRuleTrialResult.value?.bom_version_id || 0)
  return pricingRuleTrialBomVersionOptions.value.find((option) => Number(option?.version_id || 0) === versionID) || null
})
const pricingRuleTrialProcessRouteOptions = computed(() => Array.isArray(pricingRuleTrialResult.value?.process_route_options) ? pricingRuleTrialResult.value.process_route_options : [])
const pricingRuleTrialOperationTemplateOptions = computed(() => Array.isArray(pricingRuleTrialResult.value?.operation_template_options) ? pricingRuleTrialResult.value.operation_template_options : [])
const pricingRuleTrialAutoRunSignature = computed(() => JSON.stringify({
  open: pricingRuleTrialDrawerOpen.value,
  pricing_rule_id: pricingRuleTrialForm.value.pricing_rule_id,
  parent_product_id: pricingRuleTrialForm.value.parent_product_id,
  product_id: pricingRuleTrialForm.value.product_id,
  customer_id: pricingRuleTrialForm.value.customer_id,
  bom_version_id: pricingRuleTrialForm.value.bom_version_id,
  process_route_id: pricingRuleTrialForm.value.process_route_id,
  operation_template_id: pricingRuleTrialForm.value.operation_template_id,
  quote_unit: pricingRuleTrialForm.value.quote_unit,
  margin_rate: pricingRuleTrialForm.value.margin_rate,
  tax_rate: pricingRuleTrialForm.value.tax_rate,
  other_cost_rows: pricingRuleTrialForm.value.other_cost_rows,
}))

const contextCategorizedProductIDs = computed(() => {
  const ids = new Set()
  for (const primary of categoryTreeForSkuContext.value) {
    for (const product of primary.products || []) ids.add(Number(product.id))
    for (const secondary of primary.children || []) {
      for (const product of secondary.products || []) ids.add(Number(product.id))
    }
  }
  return ids
})

const uncategorizedProducts = computed(() => products.value
  .filter(skuContextProductFilter)
  .filter((product) => !contextCategorizedProductIDs.value.has(Number(product.id)))
  .slice()
  .sort((a, b) => ownerLabel(a).localeCompare(ownerLabel(b)) || a.name.localeCompare(b.name)))

function skuTableCategoryMeta(categoriesForTable = []) {
  const byProductID = new Map()
  const byCategoryID = new Map()
  function visit(nodes = [], primaryName = '', parentNumber = '') {
    for (const [index, category] of (nodes || []).entries()) {
      const categoryID = Number(category?.id || 0)
      const categoryNumber = parentNumber ? `${parentNumber}.${index + 1}` : String(index + 1)
      const nextPrimaryName = primaryName || category?.name || ''
      const secondaryName = primaryName ? category?.name || '' : ''
      const categoryMeta = {
        number: categoryNumber,
        product_category_position: Number(category?.position || index + 1),
        primary_name: nextPrimaryName,
        secondary_name: secondaryName,
      }
      if (categoryID) byCategoryID.set(categoryID, categoryMeta)
      for (const [productIndex, product] of (category?.products || []).entries()) {
        const productID = Number(product?.id || 0)
        if (!productID) continue
        byProductID.set(productID, {
          ...categoryMeta,
          number: product?.number || productIndex + 1,
          product_category_position: Number(product?.product_category_position || productIndex + 1),
        })
      }
      visit(category?.children || [], nextPrimaryName, categoryNumber)
    }
  }
  visit(categoriesForTable)
  return { byProductID, byCategoryID }
}

function skuTableRowsFromFlatProducts(sourceProducts = [], sourceCategories = [], filterFn = () => true) {
  const { byProductID, byCategoryID } = skuTableCategoryMeta(sourceCategories)
  return visibleRowsForProductSpecMigration(sourceProducts || [], productSpecMigrationByProductID.value)
    .filter((product) => {
      try {
        return filterFn(product)
      } catch (_) {
        return false
      }
    })
    .map((product, index) => ({
      ...product,
      ...(byProductID.get(Number(product?.id || 0))
        || byCategoryID.get(Number(product?.product_category_id || 0))
        || { number: index + 1, primary_name: '', secondary_name: '' }),
    }))
}

const baseProducts = computed(() => products.value.filter((product) => Number(product.customer_id || 0) === 0 && productVisibility(product) === 'public'))
const customBaseProducts = computed(() => baseProducts.value.filter((product) => normalizedProductKind(product) === customForm.value.product_kind))
const publicSkuRowsRaw = computed(() => sortRowsForCustomerSkuPriority(
  skuTableRowsFromFlatProducts(products.value, categories.value, (product) => Number(product.customer_id || 0) === 0),
  0,
))
const publicSkuRows = computed(() => productArchiveRowsWithSkus(publicSkuRowsRaw.value))
const customerSkuCustomers = computed(() => customerSkuCustomerOptions(customers.value))
const aliasCustomerLabel = computed(() => {
  const customerID = Number(selectedAliasCustomerID.value || 0)
  if (!customerID) return '请选择客户'
  return customerName(customerID) || `客户 #${customerID}`
})
const aliasProductOptions = computed(() => products.value
  .filter((product) => product.active !== false)
  .slice()
  .sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')) || Number(a.id || 0) - Number(b.id || 0)))
const productProductionConfigIndustryTemplateOptions = computed(() => industryFieldTemplateOptionsForConfig(
  industryFieldTemplates.value,
  productProductionConfigForm.value,
))
const productProductionConfigActiveBomOptions = computed(() => activeProductionBomOptions(productionBoms.value))
const productProductionConfigBomUsageRows = computed(() => {
  const productID = Number(productProductionConfigProduct.value?.id || productProductionConfigForm.value.product_id || 0)
  const seen = new Set()
  const rows = []
  for (const row of productBomUsageByProductID.value[String(productID)] || []) {
    if (!isActiveBomUsageRow(row)) continue
    const normalized = { relation_type: 'component', ...row }
    const key = bomUsageRowKey(normalized)
    if (seen.has(key)) continue
    seen.add(key)
    rows.push(normalized)
  }
  return rows
})
const productProductionConfigProduceBomRows = computed(() => productProductionConfigBomUsageRows.value
  .filter((row) => String(row.relation_type || '') === 'output'))
const productProductionConfigUsedByBomRows = computed(() => productProductionConfigBomUsageRows.value
  .filter((row) => String(row.relation_type || '') === 'component'))
const productProductionConfigMigration = computed(() => {
  const productID = Number(productProductionConfigProduct.value?.id || productProductionConfigForm.value.product_id || 0)
  return productSpecMigrationByProductID.value[String(productID)] || productProductionConfigProduct.value || {}
})
const productBomSpecMigrationState = computed(() => productSpecMigrationState(productProductionConfigMigration.value))
const productBomSpecMigrationStateLabel = computed(() => ({
  legacy: '旧规格模式',
  preparing: '准备中',
  ready: '可切换',
  cutover: '已切换 BOM 规格',
}[productBomSpecMigrationState.value] || '旧规格模式'))
const productBomSpecMigrationBlockers = computed(() => productProductionConfigMigration.value?.readiness?.blockers || [])
const productBomSpecMigrationMappings = computed(() => productProductionConfigMigration.value?.mappings || [])
const productProductionConfigUsesBomSpecs = computed(() => isProductBomSpecCutover(productProductionConfigMigration.value))
const productProductionDefaultBomUsageRow = computed(() => productProductionConfigProduceBomRows.value.find((row) => row.is_default === true || row.isDefault === true)
  || productProductionConfigProduceBomRows.value.find((row) => bomUsageBomID(row) === Number(productProductionConfigForm.value.production_bom_id || 0))
  || null)
const productProductionBomSpecs = computed(() => {
  const bomID = bomUsageBomID(productProductionDefaultBomUsageRow.value || {})
    || Number(productProductionConfigForm.value.production_bom_id || 0)
  return normalizeProductBomSpecs(productionBomDetails.value[String(bomID)] || {})
})
const productProductionBomSpecsSummary = computed(() => {
  const specs = productProductionBomSpecs.value
  if (!specs.length) return productProductionDefaultBomUsageRow.value ? '默认 BOM 暂无可用规格' : '尚未绑定默认制造 BOM'
  const names = specs.map((row) => `${row.name || row.code || `规格 #${row.bom_spec_id}`}${row.is_default ? '（默认）' : ''}`)
  return names.length > 3 ? `${names.slice(0, 3).join('、')} 等 ${names.length} 个规格` : names.join('、')
})
const aliasDisplayCategoryOptions = computed(() => flattenCategoryNodes(categories.value).map((category) => ({
  id: Number(category.id || 0),
  label: `${Number(category.customer_id || 0) > 0 ? `${customerName(category.customer_id) || '客户'} / ` : ''}${category.name || ''}`,
})).filter((category) => category.id > 0 && category.label))
const visibleCustomerProductAliases = computed(() => customerProductAliasRowsForCustomer(customerProductAliases.value, selectedAliasCustomerID.value, {
  active: aliasFilters.value.active,
  query: aliasFilters.value.query,
}))
const aliasBatchRows = computed(() => skuTableRowsFromFlatProducts(aliasProductOptions.value, categories.value, () => true))
const aliasBatchFilteredRows = computed(() => filterAliasBatchRows(aliasBatchRows.value, aliasBatchFilters.value))
const aliasBatchCandidateRows = computed(() => aliasBatchFilteredRows.value.slice(0, 300))

function productionConfigForProduct(product) {
  const productID = Number(product?.id || product?.product_id || 0)
  return productProductionConfigs.value.find((row) => Number(row.product_id || 0) === productID) || {}
}

function productionConfigPriceListFields(product) {
  const config = productionConfigForProduct(product)
  return productProductionConfigFieldsFromTemplates(config.fields || [], industryFieldTemplatesForConfig(config))
    .filter((field) => field.show_in_price_list)
}

const customerSkuRowsRaw = computed(() => {
  const customerID = skuContextCustomerID.value
  return sortRowsForCustomerSkuPriority(
    skuTableRowsFromFlatProducts(products.value, categories.value, (product) => customerID > 0 && skuContextProductFilter(product)),
    customerID,
  )
})
const customerSkuRows = computed(() => productArchiveRowsWithSkus(customerSkuRowsRaw.value))
const currentSkuSourceRows = computed(() => (
  skuContextCustomerID.value > 0 ? customerSkuRows.value : publicSkuRows.value
).slice())
const normalizedSkuFilters = computed(() => normalizeVisibleSkuFilters(skuFilters.value, currentSkuSourceRows.value))
const filteredSkuRows = computed(() => filterSkuRows(currentSkuSourceRows.value, normalizedSkuFilters.value))
const skuPrimaryCategoryOptions = computed(() => primaryCategoryOptions(currentSkuSourceRows.value))
const skuSecondaryCategoryOptions = computed(() => secondaryCategoryOptions(currentSkuSourceRows.value, normalizedSkuFilters.value.primaryCategory))
const hasActiveSkuFilters = computed(() => Boolean(
  normalizedSkuFilters.value.query
    || normalizedSkuFilters.value.primaryCategory
    || normalizedSkuFilters.value.secondaryCategory,
))
const skuDisplayKey = computed(() => [
  skuContextCustomerID.value,
  filteredSkuRows.value.length,
  normalizedSkuFilters.value.query || '',
  normalizedSkuFilters.value.primaryCategory || '',
  normalizedSkuFilters.value.secondaryCategory || '',
].join(':'))
const skuTableKey = computed(() => `${skuDisplayKey.value}:table`)
const fullDisplaySkuGroups = computed(() => groupRowsByBusinessGroupTemplates(filteredSkuRows.value, {
  templates: productCatalogBusinessGroups.value,
  assignments: businessGroupAssignments.value,
  usageKey: 'product_catalog',
  objectKey: 'product',
  objectIDForRow: (row) => Number(row.id || 0),
}))
const productInlineGroupState = computed(() => businessGroupInlineListState(fullDisplaySkuGroups.value, skuGroupPagination.value, {
  defaultPageSize: DEFAULT_SKU_GROUP_PAGE_SIZE,
}))
const displaySkuGroups = computed(() => productInlineGroupState.value.groups)
const displaySkuRows = computed(() => productInlineGroupState.value.visibleRows)
const visibleDisplaySkuRows = computed(() => businessGroupVisibleRows(displaySkuGroups.value, collapsedProductClassificationGroups.value))
const allAliasRowsSelected = computed(() => visibleCustomerProductAliases.value.length > 0 && visibleCustomerProductAliases.value.every((row) => row.active === false || selectedAliasIds.value.includes(Number(row.id))))
const activeGradientTemplates = computed(() => gradientTemplates.value
  .filter((template) => template.active !== false)
  .filter((template) => gradientTemplateBelongsToSkuContext(template, {
    customerID: skuContextCustomerID.value,
    usePublicGradientTemplates: customerUsesPublicGradientTemplates.value,
    customerTemplates: customerGradientTemplatesForContext.value,
  })))
const gradientTemplatesForContext = computed(() => activeGradientTemplates.value)
const activeGradientTemplatesForContext = computed(() => activeGradientTemplates.value)
const selectableGradientTemplatesForProductConfig = computed(() => gradientTemplates.value
  .filter((template) => template.active !== false)
  .filter((template) => {
    const templateCustomerID = Number(template.customer_id || 0)
    const customerID = skuContextCustomerID.value
    if (!customerID) return templateCustomerID === 0
    return templateCustomerID === 0 || templateCustomerID === customerID
  }))
const visibleProductUnitDefinitions = computed(() => visibleNonDeletedRows(productUnitDefinitions.value))
const visibleProductUnitTemplates = computed(() => visibleNonDeletedRows(productUnitTemplates.value))
const visibleProductConfigTemplates = computed(() => visibleNonDeletedRows(productConfigTemplates.value))
const visibleProductClassificationTemplates = computed(() => visibleNonDeletedRows(productClassificationTemplates.value))
const activeProductUnitDefinitions = computed(() => visibleProductUnitDefinitions.value.filter((unit) => unit.active !== false))
const activeProductUnitTemplates = computed(() => visibleProductUnitTemplates.value.filter((template) => template.active !== false))
const gradientDisplayUnitOptions = computed(() => {
  const out = baseGradientDisplayUnitOptions.map((unit) => ({ ...unit }))
  const seen = new Set(out.map((unit) => unit.value))
  for (const unit of activeProductUnitDefinitions.value) {
    const code = String(unit.code || '').trim()
    if (!code || seen.has(code)) continue
    out.push({ value: code, label: `元/${code}`, quantityLabel: code, specG: 1 })
    seen.add(code)
  }
  return out
})
const productConfigTemplatesForContext = computed(() => visibleProductConfigTemplates.value
  .filter((template) => template.active !== false)
  .filter((template) => productConfigTemplateBelongsToSkuContext(template, {
    customerID: skuContextCustomerID.value,
    customerTemplates: customerProductConfigTemplatesForContext.value,
  })))
const activeProductClassificationTemplates = computed(() => visibleProductClassificationTemplates.value
  .filter((template) => template.active !== false)
  .slice()
  .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || String(a.name || '').localeCompare(String(b.name || ''))))
const productClassificationTabs = computed(() => classificationTemplateTabs(activeProductClassificationTemplates.value, productClassificationTemplateUsages.value, { allLabel: '全部商品', unclassifiedLabel: '未分类商品' }))
const aliasClassificationUsagesForCustomer = computed(() => aliasClassificationTemplateUsages.value.filter((row) => Number(row.customer_id || 0) === Number(selectedAliasCustomerID.value || 0)))
const aliasClassificationTabs = computed(() => classificationTemplateTabs(activeProductClassificationTemplates.value, aliasClassificationUsagesForCustomer.value, { allLabel: '全部客户商品', unclassifiedLabel: '未分类客户商品' }))
const currentProductClassificationTab = computed(() => productClassificationTabs.value.find((tab) => tab.key === activeProductClassificationTab.value) || productClassificationTabs.value[0])
const currentAliasClassificationTab = computed(() => aliasClassificationTabs.value.find((tab) => tab.key === activeAliasClassificationTab.value) || aliasClassificationTabs.value[0])
const currentProductClassificationTemplate = computed(() => currentProductClassificationTab.value?.template || null)
const currentAliasClassificationTemplate = computed(() => currentAliasClassificationTab.value?.template || null)
const isProductAllOrUnclassifiedTab = computed(() => Boolean(currentProductClassificationTab.value?.all || currentProductClassificationTab.value?.unclassified))
const isAliasAllOrUnclassifiedTab = computed(() => Boolean(currentAliasClassificationTab.value?.all || currentAliasClassificationTab.value?.unclassified))
const productMovableClassificationTabs = computed(() => productClassificationTabs.value.filter((tab) => !tab.all && !tab.unclassified))
const aliasMovableClassificationTabs = computed(() => aliasClassificationTabs.value.filter((tab) => !tab.all && !tab.unclassified))
const productAddClassificationOptions = computed(() => {
  const enabled = new Set(productMovableClassificationTabs.value.map((tab) => Number(tab.id || tab.template?.id || 0)).filter(Boolean))
  return activeProductClassificationTemplates.value.filter((template) => !enabled.has(Number(template.id || 0)))
})
const aliasAddClassificationOptions = computed(() => {
  const enabled = new Set(aliasMovableClassificationTabs.value.map((tab) => Number(tab.id || tab.template?.id || 0)).filter(Boolean))
  return activeProductClassificationTemplates.value.filter((template) => !enabled.has(Number(template.id || 0)))
})
const productMoveClassificationOptions = computed(() => {
  if (isProductAllOrUnclassifiedTab.value) return productMovableClassificationTabs.value.map((tab) => ({ ...tab, move_type: 'template' }))
  return [{ id: UNCLASSIFIED_CATEGORY_MOVE_ID, category_id: 0, name: '未分类', move_type: 'category' }, ...productClassificationCategories.value.map((category) => ({ ...category, category_id: Number(category.id || 0), move_type: 'category' }))]
})
const selectableProductGroupTemplates = computed(() => businessGroupControlOptions(businessGroups.value).templateOptions.map((option) => option.group))
const productGroupFeatureSelectionHasChanges = computed(() => (
  JSON.stringify(businessGroupFeatureSelectionIDs({ group_template_ids: productGroupFeatureSelectionDraft.value }))
  !== JSON.stringify(businessGroupFeatureSelectionIDs({ group_template_ids: productGroupFeatureSelectionIDs.value }))
))
const productCatalogBusinessGroups = computed(() => productCatalogBusinessGroupRows())
const productBusinessGroupControls = computed(() => businessGroupControlOptions(productCatalogBusinessGroups.value, {
  selectedTemplateID: selectedProductGroupTemplateID.value,
  usageKey: 'product_catalog',
}))
const selectedProductGroupTemplate = computed(() => productBusinessGroupControls.value.selectedTemplate)
const canMoveSelectedProductsToBusinessGroup = computed(() => Boolean(productCatalogBusinessGroups.value.length && selectedProductIds.value.length))
const aliasMoveClassificationOptions = computed(() => {
  if (isAliasAllOrUnclassifiedTab.value) return aliasMovableClassificationTabs.value.map((tab) => ({ ...tab, move_type: 'template' }))
  return [{ id: UNCLASSIFIED_CATEGORY_MOVE_ID, category_id: 0, name: '未分类', move_type: 'category' }, ...aliasClassificationCategories.value.map((category) => ({ ...category, category_id: Number(category.id || 0), move_type: 'category' }))]
})
const classificationTemplateEditorTemplate = computed(() => visibleProductClassificationTemplates.value.find((template) => Number(template.id || 0) === Number(classificationTemplateForm.value.id || 0)) || null)
const classificationTemplateEditorCategories = computed(() => (classificationTemplateEditorTemplate.value?.categories || [])
  .filter((category) => category.active !== false)
  .slice()
  .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0)))
const productClassificationCategories = computed(() => (currentProductClassificationTemplate.value?.categories || []).filter((category) => category.active !== false).slice().sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0)))
const aliasClassificationCategories = computed(() => (currentAliasClassificationTemplate.value?.categories || []).filter((category) => category.active !== false).slice().sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0)))
const visibleCustomerAliasGroups = computed(() => {
  const tab = currentAliasClassificationTab.value
  if (!tab || tab.all) return [{ key: 'all-aliases', label: '全部客户商品', rows: visibleCustomerProductAliases.value, all: true }]
  if (tab.unclassified) {
    return [{
      key: 'unclassified-aliases',
      label: '未分类客户商品',
      rows: visibleCustomerProductAliases.value.filter((row) => !classificationAssignmentForRow(row, productClassificationTemplates.value, { assignmentType: 'alias' })),
      all: true,
    }]
  }
  return groupRowsByClassificationCategory(visibleCustomerProductAliases.value, tab.template, {
    idKey: 'id',
    assignmentKey: 'alias_id',
    assignmentsKey: 'customer_alias_assignments',
    onlyAssigned: true,
  })
})
const activeProductConfigTemplates = computed(() => productConfigTemplatesForContext.value.filter((template) => template.active !== false))
const customerProductRuleTemplatesForContext = computed(() => customerProductRuleTemplates.value
  .filter((template) => Number(template.customer_id || 0) === 0 || Number(template.customer_id || 0) === skuContextCustomerID.value)
  .filter((template) => template.active !== false))
const customerProductRuleOverridesForContext = computed(() => customerProductRuleOverrides.value
  .filter((row) => Number(row.customer_id || 0) === skuContextCustomerID.value && row.active !== false))
const currentCustomerRuleBinding = computed(() => customerProductRuleBindings.value
  .find((row) => Number(row.customer_id || 0) === skuContextCustomerID.value) || null)
const currentCustomerRuleTemplateID = computed(() => Number(currentCustomerRuleBinding.value?.template_id || 0))
const productSubtypeRuleOptions = computed(() => flattenCategoryNodes(categoryTreeForSkuContext.value)
  .filter((category) => Number(category.level || 0) === 2 || Number(category.parent_id || 0) > 0))
const selectedTemplateRow = computed(() => gradientTemplates.value.find((template) => Number(template.id || 0) === Number(templateForm.value.id || 0)) || null)
const canEditCurrentTemplate = computed(() => {
  if (!skuContextCustomerID.value) return true
  if (!templateForm.value.id) return true
  return Number(selectedTemplateRow.value?.customer_id || 0) === skuContextCustomerID.value
})
const selectedProductConfigTemplateRow = computed(() => productConfigTemplates.value.find((template) => Number(template.id || 0) === Number(productConfigTemplateForm.value.id || 0)) || null)
const canEditCurrentProductConfigTemplate = computed(() => {
  if (!skuContextCustomerID.value) return true
  if (!productConfigTemplateForm.value.id) return true
  return Number(selectedProductConfigTemplateRow.value?.customer_id || 0) === skuContextCustomerID.value
})
const selectedProductRowsAlreadyInCurrentCategory = computed(() => {
  const templateID = Number(currentProductClassificationTemplate.value?.id || 0)
  if (!templateID || !selectedProductIds.value.length) return false
  const categoryID = Number(selectedProductClassificationCategoryID.value || 0)
  const selected = new Set(selectedProductIds.value.map((id) => Number(id || 0)))
  const assignments = currentProductClassificationTemplate.value?.product_assignments || []
  return [...selected].every((productID) => assignments.some((assignment) => Number(assignment.product_id || 0) === productID && Number(assignment.template_id || templateID) === templateID && Number(assignment.category_id || 0) === categoryID))
})
const selectedAliasRowsAlreadyInCurrentCategory = computed(() => {
  const templateID = Number(currentAliasClassificationTemplate.value?.id || 0)
  if (!templateID || !selectedAliasIds.value.length) return false
  const categoryID = Number(selectedAliasClassificationCategoryID.value || 0)
  const selected = new Set(selectedAliasIds.value.map((id) => Number(id || 0)))
  const assignments = currentAliasClassificationTemplate.value?.customer_alias_assignments || []
  return [...selected].every((aliasID) => assignments.some((assignment) => Number(assignment.alias_id || 0) === aliasID && Number(assignment.template_id || templateID) === templateID && Number(assignment.category_id || 0) === categoryID))
})
function defaultSkuFilters() {
  return normalizeVisibleSkuFilters()
}

function normalizeSkuFiltersForCurrentRows(filters = skuFilters.value) {
  return normalizeVisibleSkuFilters(filters, currentSkuSourceRows.value)
}

function syncVisibleSkuTableState() {
  const normalizedFilters = normalizeSkuFiltersForCurrentRows()
  if (JSON.stringify(normalizedFilters) !== JSON.stringify(skuFilters.value)) {
    skuFilters.value = normalizedFilters
  }
}

function syncSkuGroupPaginationState() {
  const normalizedPagination = productInlineGroupState.value.pagination
  if (JSON.stringify(normalizedPagination) !== JSON.stringify(skuGroupPagination.value)) {
    skuGroupPagination.value = normalizedPagination
  }
}

function resetSkuGroupPages() {
  if (restoringProductSettingsDraft) return
  skuGroupPagination.value = Object.fromEntries(
    Object.entries(skuGroupPagination.value).map(([key, value]) => [key, {
      page: 1,
      pageSize: normalizePageSize(value?.pageSize || DEFAULT_SKU_GROUP_PAGE_SIZE),
    }]),
  )
}

function defaultSkuForm() {
  const unitTemplateID = defaultProductUnitTemplateID()
  return {
    name: '',
    remark: '',
    unit_template_id: unitTemplateID,
    unit_rule_override_enabled: false,
    inventory_unit: 'kg',
    integer_inventory_unit: false,
    default_sales_unit: 'kg',
    unit_conversion_rows: [{ from_qty: 1, from_unit: 'kg', to_qty: 1, to_unit: 'kg', integer_sales_unit: false }],
    product_config_template_id: 0,
    special_attr_values: {},
    active: true,
  }
}

function defaultCustomerProductAliasForm() {
  return {
    id: 0,
    customer_id: Number(selectedAliasCustomerID.value || 0),
    product_id: 0,
    display_name: '',
    customer_item_code: '',
    brand_name: '',
    display_category_id: 0,
    product_config_template_id: 0,
    sort_order: 0,
    include_in_price_list: true,
    active: true,
    remark: '',
  }
}

function defaultCustomerProductAliasBatchForm() {
  return {
    customer_id: Number(selectedAliasCustomerID.value || 0),
    include_in_price_list: true,
    brand_name: '',
    display_category_id: 0,
  }
}

function defaultAliasBatchFilters() {
  return {
    query: '',
  }
}

function defaultClassificationCategoryForm() {
  return { id: 0, name: '', sort_order: 100, product_config_template_id: 0, gradient_template_id: 0, unit_template_id: 0 }
}

function defaultProductProductionConfigField(row = {}, index = 0) {
  return buildProductProductionConfigField(row, index)
}

function defaultProductProductionConfigForm(config = {}, product = {}) {
  return buildProductProductionConfigForm(config, product, industryFieldTemplatesForConfig(config))
}

function isAliasClassificationGroupCollapsed(key) {
  return collapsedAliasClassificationGroups.value.includes(String(key || ''))
}

function toggleAliasClassificationGroup(key) {
  const groupKey = String(key || '')
  if (!groupKey) return
  collapsedAliasClassificationGroups.value = isAliasClassificationGroupCollapsed(groupKey)
    ? collapsedAliasClassificationGroups.value.filter((item) => item !== groupKey)
    : [...collapsedAliasClassificationGroups.value, groupKey]
}

function isAliasSelected(alias) {
  return selectedAliasIds.value.includes(Number(alias?.id || 0))
}

function toggleAliasSelection(alias, checked) {
  const id = Number(alias?.id || 0)
  if (!id) return
  selectedAliasIds.value = checked
    ? Array.from(new Set([...selectedAliasIds.value, id]))
    : selectedAliasIds.value.filter((item) => item !== id)
}

function toggleAllAliasRows(checked) {
  if (!checked) {
    selectedAliasIds.value = []
    return
  }
  selectedAliasIds.value = visibleCustomerProductAliases.value
    .filter((row) => row.active !== false)
    .map((row) => Number(row.id || 0))
    .filter(Boolean)
}

function defaultProductForm() {
  const unitTemplateID = defaultProductUnitTemplateID()
  return {
    name: '',
    product_type_category_id: 0,
    product_subtype_category_id: 0,
    product_kind: 'roasted',
    remark: '',
    unit_template_id: unitTemplateID,
    unit_rule_override_enabled: false,
    inventory_unit: 'kg',
    integer_inventory_unit: false,
    special_attr_values: {},
  }
}

function productKindLabel(row = {}) {
  const kind = normalizedProductKind(row)
  if (kind === 'green_bean') return '生豆'
  if (kind === 'drip_bag') return '挂耳'
  if (kind === 'instant_coffee') return '速溶咖啡'
  return '熟豆'
}

function productKindBadgeClass(row = {}) {
  const kind = normalizedProductKind(row)
  if (kind === 'green_bean') return 'kind-green'
  if (kind === 'drip_bag') return 'kind-drip'
  if (kind === 'instant_coffee') return 'kind-instant'
  return 'kind-roasted'
}

function categoryLabel(row, level) {
  return level === 1 ? row.primary_name || '' : row.secondary_name || ''
}

function defaultCustomForm() {
  return {
    customer_id: 0,
    base_product_id: 0,
    name: '',
    remark: '',
    product_type_category_id: 0,
    product_subtype_category_id: 0,
    product_kind: 'roasted',
    special_attr_values: {},
    custom_type: 'public_sku_alias',
    copy_bom: true,
    copy_price_tiers: true,
  }
}

function defaultGradientTemplateForm() {
  return normalizeGradientTemplate({
    name: '',
    display_unit: 'lb',
    unit_template_id: 0,
    tiers: [
      { label: '2磅-13磅', min_display_qty: 2, max_display_qty: 13, margin_rate: 0.5421052631578949, position: 1 },
    ],
  })
}

function defaultProductConfigTemplateForm(template = {}, options = {}) {
  const inventoryUnit = template.inventory_unit || 'kg'
  const quoteUnit = template.quote_unit || inventoryUnit
  const hasCustomerID = Object.prototype.hasOwnProperty.call(template, 'customer_id')
  const defaultCustomerID = Object.prototype.hasOwnProperty.call(options, 'customerID') ? options.customerID : skuContextCustomerID.value
  return {
    id: Number(template.id || 0),
    customer_id: Number(hasCustomerID ? template.customer_id || 0 : defaultCustomerID || 0),
    source_template_id: Number(template.source_template_id || 0),
    template_state: template.template_state || '',
    name: template.name || '',
    gradient_template_id: Number(template.gradient_template_id || 0),
    operation_template_id: Number(template.operation_template_id || 0),
    unit_template_id: Number(template.unit_template_id || 0),
    price_list_rule_json: template.price_list_rule_json || '{}',
    ...priceListRuleFormFromJSON(template.price_list_rule_json || '{}'),
    inventory_unit: inventoryUnit,
    quote_unit: quoteUnit,
    order_unit: template.order_unit || quoteUnit,
    unit_conversion_json: template.unit_conversion_json || '{}',
    unit_conversion_rows: unitConversionRowsFromJSON(template.unit_conversion_json || '{}'),
    integer_unit: Boolean(template.integer_unit),
    active: template.active !== false,
  }
}

function defaultClassificationTemplateForm(template = {}) {
  return {
    id: Number(template.id || 0),
    customer_id: 0,
    source_template_id: Number(template.source_template_id || 0),
    name: template.name || '',
    remark: template.remark || '',
    product_config_template_id: Number(template.product_config_template_id || 0),
    gradient_template_id: Number(template.gradient_template_id || 0),
    unit_template_id: Number(template.unit_template_id || 0),
    sort_order: Number(template.sort_order || 100),
    active: template.active !== false,
  }
}

function defaultProductUnitDefinitionForm(unit = {}) {
  return {
    code: unit.code || '',
    name: unit.name || '',
    unit_type: unit.unit_type || 'package',
    allow_decimal: Boolean(unit.allow_decimal),
    active: unit.active !== false,
  }
}

function unitTypeLabel(value) {
  return {
    weight: '重量',
    package: '包装',
    count: '数量',
    other: '其他',
  }[value] || '其他'
}

function defaultProductUnitTemplateForm(template = {}) {
  const inventoryUnit = template.inventory_unit || 'kg'
  const originalInventoryUnit = template.original_inventory_unit || (Number(template.id || 0) > 0 ? inventoryUnit : '')
  const salesSpecRows = salesSpecRowsFromTemplate(template, inventoryUnit)
  if (!salesSpecRows.length) {
    salesSpecRows.push({
      spec_key: `spec-${Date.now()}`,
      spec_name: '',
      sales_unit: '',
      net_content_qty: 0,
      net_content_unit: inventoryUnit,
      default: true,
      active: true,
      derived_sku_code: '',
      derived_spec_status: 'active',
    })
  }
  const defaultSpec = salesSpecRows.find((row) => row.default) || salesSpecRows[0]
  const defaultSalesUnit = defaultSpec?.sales_unit || defaultSpec?.spec_name || template.default_sales_unit || template.order_unit || template.quote_unit || template.sales_unit || ''
  return {
    id: Number(template.id || 0),
    name: template.name || '',
    inventory_unit: inventoryUnit,
    original_inventory_unit: originalInventoryUnit,
    sales_unit: defaultSalesUnit,
    default_sales_unit: defaultSalesUnit,
    sales_units: Array.isArray(template.sales_units) ? template.sales_units : [],
    quote_unit: defaultSalesUnit,
    order_unit: defaultSalesUnit,
    unit_conversion_json: template.unit_conversion_json || '{}',
    unit_conversion_rows: [],
    sales_spec_rows: salesSpecRows,
    default_spec_key: defaultSpec?.spec_key || '',
    integer_unit: Boolean(template.integer_unit),
    active: template.active !== false,
  }
}

function defaultProductPriceRecordForm(record = {}) {
  return {
    id: Number(record.id || 0),
    product_id: Number(record.product_id || 0),
    customer_product_alias_id: Number(record.customer_product_alias_id || 0),
    final_unit_price: Number(record.final_unit_price || 0),
    price_unit: record.price_unit || 'kg',
    currency: record.currency || 'CNY',
    price_group_id: Number(record.price_group_id || 0),
    price_group_name: record.price_group_name || productPriceGroupName(record.price_group_id) || '',
    inventory_unit: record.inventory_unit || 'kg',
    inventory_conversion_json: typeof record.inventory_conversion_json === 'object'
      ? JSON.stringify(record.inventory_conversion_json)
      : (record.inventory_conversion_json || '{"kg":{"kg":1}}'),
    status: record.status || 'draft',
    remark: record.remark || '',
    active: record.active !== false,
  }
}

function defaultProductTierPriceSchemeForm(scheme = {}) {
  return {
    id: Number(scheme.id || 0),
    name: scheme.name || '',
    product_id: Number(scheme.product_id || 0),
    customer_product_alias_id: Number(scheme.customer_product_alias_id || 0),
    price_group_id: Number(scheme.price_group_id || 0),
    active: scheme.active !== false,
    remark: scheme.remark || '',
    tiers: Array.isArray(scheme.tiers) && scheme.tiers.length
      ? scheme.tiers.map((tier, index) => defaultProductTierPriceSchemeTier(tier, index))
      : [defaultProductTierPriceSchemeTier({}, 0)],
  }
}

function defaultProductTierPriceSchemeTier(tier = {}, index = 0) {
  return {
    label: tier.label || '',
    min_qty: Number(tier.min_qty || 0),
    max_qty: tier.max_qty === null || typeof tier.max_qty === 'undefined' ? '' : tier.max_qty,
    source_price_record_id: Number(tier.source_price_record_id || 0),
    position: Number(tier.position || 0) || index + 1,
  }
}

function defaultPricingRuleForm(rule = {}) {
  const calculation = pricingRuleCalculationFromRule(rule)
  return {
    id: Number(rule.id || 0),
    name: rule.name || '',
    code: rule.code || '',
    cost_source_mode: normalizePricingRuleCostSourceMode(rule.cost_source_mode),
    margin_rate: Number(rule.margin_rate || 0),
    tax_rate: Number(rule.tax_rate || 0),
    rounding_mode: rule.rounding_mode || 'none',
    formula_version: rule.formula_version || 'v1',
    calculation_json: calculation,
    other_cost_rows: pricingRuleOtherCostRowsFromCalculation(calculation),
    profit_method: 'markup',
    tax_mode: calculation.tax_mode || 'tax_included',
    minimum_margin_rate: Number(calculation.minimum_margin_rate || 0),
    trial_note: calculation.trial_note || '',
    active: rule.active !== false,
    remark: rule.remark || '',
  }
}

function pricingRuleOtherCostRowsFromCalculation(calculation = {}) {
  return pricingRuleCostRowsFromMap(calculation.other_costs ?? calculation.otherCosts ?? {}, defaultPricingRuleOtherCostRow)
}

function pricingRuleCostRowsFromMap(raw = {}, defaultRow) {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return [defaultRow()]
  const rows = Object.entries(raw)
    .map(([key, value]) => ({
      key: String(key || '').trim(),
      value: Number(value || 0),
    }))
    .filter((row) => row.key)
  return rows.length ? rows : [defaultRow()]
}

function defaultPricingRuleOtherCostRow() {
  return { key: '', value: 0 }
}

function defaultPricingRuleTrialForm(rule = {}) {
  const form = defaultPricingRuleForm(rule || {})
  return {
    pricing_rule_id: Number(form.id || 0),
    parent_product_id: 0,
    product_id: 0,
    customer_id: 0,
    bom_version_id: 0,
    process_route_id: 0,
    operation_template_id: 0,
    quote_unit: '',
    margin_rate: form.margin_rate,
    tax_rate: '',
    other_cost_rows: form.other_cost_rows.map((row) => ({ ...row })),
  }
}

function defaultPricingRuleTrialOtherCostRow() {
  return { key: '', value: 0 }
}

function pricingRuleCalculationFromRule(rule = {}) {
  const raw = rule.calculation_json ?? rule.calculationJSON ?? {}
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) return raw
  if (typeof raw === 'string' && raw.trim()) {
    try {
      const parsed = JSON.parse(raw)
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
    } catch {
      return {}
    }
  }
  return {}
}

function defaultPriceTierTemplateForm(template = {}) {
  return {
    id: Number(template.id || 0),
    name: template.name || '',
    active: template.active !== false,
    remark: template.remark || '',
    tiers: Array.isArray(template.tiers) && template.tiers.length
      ? template.tiers.map((tier, index) => defaultPriceTierTemplateTier(tier, index))
      : [defaultPriceTierTemplateTier({}, 0)],
  }
}

function defaultPriceTierTemplateTier(tier = {}, index = 0) {
  return {
    label: tier.label || '',
    min_qty: Number(tier.min_qty || 0),
    max_qty: tier.max_qty === null || typeof tier.max_qty === 'undefined' ? '' : tier.max_qty,
    quantity_unit: tier.quantity_unit || 'kg',
    position: Number(tier.position || 0) || index + 1,
    active: tier.active !== false,
    remark: tier.remark || '',
  }
}

function defaultCustomerProductRuleTemplateForm() {
  return {
    id: 0,
    customer_id: Number(selectedCustomerSkuCustomerID.value || 0),
    name: '',
    active: true,
    items: [defaultCustomerProductRuleTemplateItem()],
  }
}

function defaultCustomerProductRuleTemplateItem() {
  return {
    product_subtype_category_id: 0,
    gradient_template_id: 0,
    operation_template_id: 0,
    price_list_rule_json: '{}',
    unit_rule_json: '{}',
    ...priceListRuleFormFromJSON('{}'),
    ...unitRuleFormFromJSON('{}'),
    active: true,
  }
}

function defaultCustomerProductRuleOverrideForm() {
  return {
    id: 0,
    customer_id: Number(selectedCustomerSkuCustomerID.value || 0),
    product_subtype_category_id: 0,
    gradient_template_id: 0,
    operation_template_id: 0,
    price_list_rule_json: '{}',
    unit_rule_json: '{}',
    ...priceListRuleFormFromJSON('{}'),
    ...unitRuleFormFromJSON('{}'),
    active: true,
  }
}

function productSettingsDraftKey() {
  const workspace = props.workspaceMode || 'factory'
  const customerID = Number(props.customerContextId || 0)
  return `${SKU_SETTINGS_FORM_DRAFT_SCOPE}:${workspace}:${customerID || 'all'}`
}

function saveProductSettingsDraft() {
  saveFormDraft(productSettingsDraftKey(), {
    selectedCustomerSkuCustomerID: selectedCustomerSkuCustomerID.value,
    selectedAliasCustomerID: selectedAliasCustomerID.value,
    customerProductAliasForm: customerProductAliasForm.value,
    skuForm: skuForm.value,
    templateForm: templateForm.value,
    productConfigTemplateForm: productConfigTemplateForm.value,
    productUnitTemplateForm: productUnitTemplateForm.value,
    productPriceRecordForm: productPriceRecordForm.value,
    productTierPriceSchemeForm: productTierPriceSchemeForm.value,
    editingCategoryId: editingCategoryId.value,
    editingCategoryName: editingCategoryName.value,
    categoryCollapsed: categoryCollapsed.value,
    collapsedPrimaryCategoryIds: collapsedPrimaryCategoryIds.value,
    collapsedSecondaryCategoryIds: collapsedSecondaryCategoryIds.value,
    productsCollapsed: productsCollapsed.value,
    activeSettingsSection: activeSettingsSection.value,
    activeConfigTemplateSection: activeConfigTemplateSection.value,
    categorySearchQuery: categorySearchQuery.value,
    skuFilters: skuFilters.value,
    skuGroupPagination: skuGroupPagination.value,
    selectedProductGroupTemplateID: selectedProductGroupTemplateID.value,
  })
}

watch([
  publicSkuRows,
  customerSkuRows,
  skuFilters,
  selectedCustomerSkuCustomerID,
], syncVisibleSkuTableState, { deep: true, immediate: true })

watch(fullDisplaySkuGroups, syncSkuGroupPaginationState, { deep: true, immediate: true })

async function restoreProductSettingsDraft() {
  const draft = readFormDraft(productSettingsDraftKey())
  if (!draft) return
  restoringProductSettingsDraft = true
  selectedCustomerSkuCustomerID.value = Number(draft.selectedCustomerSkuCustomerID || 0)
  selectedAliasCustomerID.value = Number(draft.selectedAliasCustomerID || selectedCustomerSkuCustomerID.value || 0)
  syncSelectedCustomerSkuCustomer()
  applyWorkspaceCustomerContext()
  skuForm.value = { ...defaultSkuForm(), ...(draft.skuForm || draft.productForm || {}) }
  customerProductAliasForm.value = { ...defaultCustomerProductAliasForm(), ...(draft.customerProductAliasForm || {}) }
  templateForm.value = normalizeGradientTemplate(draft.templateForm || defaultGradientTemplateForm())
  productConfigTemplateForm.value = defaultProductConfigTemplateForm(draft.productConfigTemplateForm || {})
  productUnitTemplateForm.value = defaultProductUnitTemplateForm(draft.productUnitTemplateForm || {})
  productPriceRecordForm.value = defaultProductPriceRecordForm(draft.productPriceRecordForm || {})
  productTierPriceSchemeForm.value = defaultProductTierPriceSchemeForm(draft.productTierPriceSchemeForm || {})
  const draftProductGroupTemplateID = Number(draft.selectedProductGroupTemplateID || 0)
  if (draftProductGroupTemplateID && productCatalogBusinessGroupRows().some((group) => Number(group.id || 0) === draftProductGroupTemplateID)) {
    selectedProductGroupTemplateID.value = draftProductGroupTemplateID
  }
  editingCategoryId.value = Number(draft.editingCategoryId || 0)
  editingCategoryName.value = draft.editingCategoryName || ''
  categoryCollapsed.value = Boolean(draft.categoryCollapsed)
  collapsedPrimaryCategoryIds.value = normalizeCategoryIdList(draft.collapsedPrimaryCategoryIds)
  collapsedSecondaryCategoryIds.value = normalizeCategoryIdList(draft.collapsedSecondaryCategoryIds)
  productsCollapsed.value = Boolean(draft.productsCollapsed)
  activeSettingsSection.value = ['master', 'templates', 'aliases'].includes(draft.activeSettingsSection) ? draft.activeSettingsSection : 'master'
  activeConfigTemplateSection.value = ['product-config', 'product-price-management'].includes(draft.activeConfigTemplateSection) ? draft.activeConfigTemplateSection : 'product-config'
  categorySearchQuery.value = draft.categorySearchQuery || ''
  skuFilters.value = normalizeSkuFiltersForCurrentRows(draft.skuFilters || {})
  skuGroupPagination.value = draft.skuGroupPagination && typeof draft.skuGroupPagination === 'object'
    ? draft.skuGroupPagination
    : {}
  ensureProductTypeCategorySelected(skuForm.value)
  await nextTick()
  syncVisibleSkuTableState()
  restoringProductSettingsDraft = false
}

function decorateProduct(product) {
  const productKind = normalizedProductKind(product)
  const marginRateOverride = normalizeBackendMarginRateOverride(product.margin_rate_override)
  return {
    ...product,
    product_code: product.product_code || productCodeLabel(product),
    remark: product.remark || '',
    product_kind: productKind,
    roast_level: product.roast_level || '',
    special_attrs_json: product.special_attrs_json || '{}',
    special_attr_values: {
      ...(product.roast_level ? { roast_level: product.roast_level } : {}),
      ...specialAttrValuesFromJSON(product.special_attrs_json || '{}'),
    },
    default_price: Number(product.default_price || 0),
    margin_rate_override: marginRateOverride,
    margin_rate_override_input: marginRateOverride === null ? '' : marginRateOverride,
    classification_template_id: Number(product.classification_template_id || 0),
    customer_id: Number(product.customer_id || 0),
    base_product_id: Number(product.base_product_id || 0),
    visibility: productVisibility(product),
    custom_type: product.custom_type || '',
    bom_item_count: Number(product.bom_item_count || 0),
    bom_status: product.bom_status || '',
  }
}

function normalizeBackendMarginRateOverride(value) {
  if (value === null || typeof value === 'undefined' || value === '') return null
  const numberValue = Number(value)
  return Number.isFinite(numberValue) && numberValue >= 0 ? numberValue : null
}

function decorateCategory(category) {
  const inventoryUnit = category.inventory_unit || 'kg'
  const quoteUnit = category.quote_unit || inventoryUnit
  return {
    ...category,
    customer_id: Number(category.customer_id || 0),
    source_category_id: Number(category.source_category_id || 0),
    template_state: category.template_state || '',
    product_config_template_id: Number(category.product_config_template_id || 0),
    gradient_template_id: Number(category.gradient_template_id || 0),
    operation_template_id: Number(category.operation_template_id || 0),
    price_list_rule_json: category.price_list_rule_json || '{}',
    inventory_unit: inventoryUnit,
    quote_unit: quoteUnit,
    order_unit: category.order_unit || quoteUnit,
    unit_conversion_json: category.unit_conversion_json || '{}',
    integer_unit: Boolean(category.integer_unit),
    children: (category.children || []).map(decorateCategory),
    products: (category.products || []).map(decorateProduct),
  }
}

function decorateCustomerProductAlias(alias = {}) {
  return {
    ...alias,
    id: Number(alias.id || 0),
    customer_id: Number(alias.customer_id || 0),
    product_id: Number(alias.product_id || 0),
    display_name: alias.display_name || '',
    customer_item_code: alias.customer_item_code || '',
    brand_name: alias.brand_name || '',
    display_category_id: Number(alias.display_category_id || 0),
    classification_template_id: Number(alias.classification_template_id || 0),
    product_config_template_id: Number(alias.product_config_template_id || 0),
    gradient_template_id: Number(alias.gradient_template_id || 0),
    unit_template_id: Number(alias.unit_template_id || 0),
    sort_order: Number(alias.sort_order || 0),
    include_in_price_list: alias.include_in_price_list !== false,
    active: alias.active !== false,
    remark: alias.remark || '',
    product_code: alias.product_code || '',
    product_name: alias.product_name || '',
    product_active: alias.product_active !== false,
    display_category_name: alias.display_category_name || '',
    industry_fields: (alias.industry_fields || []).map((field, index) => defaultProductProductionConfigField(field, index)),
  }
}

function decorateProductClassificationTemplate(template = {}) {
  return {
    ...template,
    id: Number(template.id || 0),
    customer_id: Number(template.customer_id || 0),
    source_template_id: Number(template.source_template_id || 0),
    template_state: template.template_state || '',
    name: template.name || '',
    product_config_template_id: Number(template.product_config_template_id || 0),
    gradient_template_id: Number(template.gradient_template_id || 0),
    unit_template_id: Number(template.unit_template_id || 0),
    active: template.active !== false,
    sort_order: Number(template.sort_order || 100),
    categories: (template.categories || []).map((category) => ({
      ...category,
      id: Number(category.id || 0),
      template_id: Number(category.template_id || template.id || 0),
      parent_id: Number(category.parent_id || 0),
      name: category.name || '',
      level: Number(category.level || 1),
      sort_order: Number(category.sort_order || 100),
      product_config_template_id: Number(category.product_config_template_id || 0),
      gradient_template_id: Number(category.gradient_template_id || 0),
      unit_template_id: Number(category.unit_template_id || 0),
      active: category.active !== false,
    })),
    product_assignments: (template.product_assignments || []).map((assignment) => ({
      product_id: Number(assignment.product_id || 0),
      template_id: Number(assignment.template_id || template.id || 0),
      category_id: Number(assignment.category_id || 0),
      sort_order: Number(assignment.sort_order || 100),
    })),
    customer_alias_assignments: (template.customer_alias_assignments || []).map((assignment) => ({
      alias_id: Number(assignment.alias_id || 0),
      template_id: Number(assignment.template_id || template.id || 0),
      category_id: Number(assignment.category_id || 0),
      sort_order: Number(assignment.sort_order || 100),
    })),
  }
}

function decorateClassificationTemplateUsage(row = {}) {
  return {
    classification_template_id: Number(row.classification_template_id || 0),
    active: row.active !== false,
    sort_order: Number(row.sort_order || 100),
  }
}

function decorateAliasClassificationTemplateUsage(row = {}) {
  return {
    customer_id: Number(row.customer_id || 0),
    classification_template_id: Number(row.classification_template_id || 0),
    active: row.active !== false,
    sort_order: Number(row.sort_order || 100),
  }
}

function decorateCustomerProductRuleTemplate(template) {
  return {
    ...template,
    id: Number(template.id || 0),
    customer_id: Number(template.customer_id || 0),
    active: template.active !== false,
    items: (template.items || []).map(decorateCustomerProductRuleItem),
  }
}

function decorateProductConfigTemplate(template) {
  return defaultProductConfigTemplateForm(template, { customerID: 0 })
}

function decorateProductUnitDefinition(unit) {
  return defaultProductUnitDefinitionForm(unit)
}

function decorateProductUnitTemplate(template) {
  return defaultProductUnitTemplateForm(template)
}

function decorateProductPriceGroup(group = {}) {
  return {
    id: Number(group.id || 0),
    name: group.name || '',
    sort_order: Number(group.sort_order || 100),
    active: group.active !== false,
  }
}

function decorateProductPriceRecord(record = {}) {
  return {
    ...record,
    id: Number(record.id || 0),
    product_id: Number(record.product_id || 0),
    customer_product_alias_id: Number(record.customer_product_alias_id || 0),
    final_unit_price: Number(record.final_unit_price || 0),
    price_group_id: Number(record.price_group_id || 0),
    price_group_name: record.price_group_name || productPriceGroupName(record.price_group_id) || '',
    inventory_conversion_json: record.inventory_conversion_json || '{"kg":{"kg":1}}',
    active: record.active !== false,
  }
}

function decorateProductTierPriceScheme(scheme = {}) {
  return {
    ...scheme,
    id: Number(scheme.id || 0),
    product_id: Number(scheme.product_id || 0),
    customer_product_alias_id: Number(scheme.customer_product_alias_id || 0),
    price_group_id: Number(scheme.price_group_id || 0),
    active: scheme.active !== false,
    tiers: (scheme.tiers || []).map((tier, index) => ({
      ...tier,
      min_qty: Number(tier.min_qty || 0),
      source_price_record_id: Number(tier.source_price_record_id || 0),
      position: Number(tier.position || 0) || index + 1,
    })),
  }
}

function decorateCustomerProductRuleItem(item = {}) {
  return {
    ...defaultCustomerProductRuleTemplateItem(),
    ...item,
    product_subtype_category_id: Number(item.product_subtype_category_id || 0),
    gradient_template_id: Number(item.gradient_template_id || 0),
    operation_template_id: Number(item.operation_template_id || 0),
    price_list_rule_json: item.price_list_rule_json || '{}',
    unit_rule_json: item.unit_rule_json || '{}',
    ...priceListRuleFormFromJSON(item.price_list_rule_json || '{}'),
    ...unitRuleFormFromJSON(item.unit_rule_json || '{}'),
    active: item.active !== false,
  }
}

function decorateCustomerProductRuleOverride(row) {
  return {
    ...defaultCustomerProductRuleOverrideForm(),
    ...row,
    id: Number(row.id || 0),
    customer_id: Number(row.customer_id || 0),
    product_subtype_category_id: Number(row.product_subtype_category_id || 0),
    gradient_template_id: Number(row.gradient_template_id || 0),
    operation_template_id: Number(row.operation_template_id || 0),
    price_list_rule_json: row.price_list_rule_json || '{}',
    unit_rule_json: row.unit_rule_json || '{}',
    ...priceListRuleFormFromJSON(row.price_list_rule_json || '{}'),
    ...unitRuleFormFromJSON(row.unit_rule_json || '{}'),
    active: row.active !== false,
  }
}

async function loadAll({ strict = false } = {}) {
  loading.value = true
  error.value = ''
  try {
    const [data, customerData, aliasData, industryData, productUsageData, aliasUsageData, productGroupSelectionData] = await Promise.all([
      apiGet('/api/product-settings'),
      apiGet('/api/customer-fulfillment/customers?limit=200'),
      apiGet('/api/customer-product-aliases?active=all'),
      apiGet('/api/industry-field-templates'),
      apiGet('/api/product-classification-template-usages/products'),
      apiGet('/api/product-classification-template-usages/customer-aliases'),
      apiGet('/api/business-group-feature-selections/product_catalog'),
    ])
    categories.value = (data.categories || []).map(decorateCategory)
    products.value = (data.products || []).map(decorateProduct)
    gradientTemplates.value = (data.gradient_templates || []).map(normalizeGradientTemplate)
    productConfigTemplates.value = (data.product_config_templates || []).map(decorateProductConfigTemplate)
    productClassificationTemplates.value = (data.product_classification_templates || []).map(decorateProductClassificationTemplate)
    productClassificationTemplateUsages.value = (productUsageData.rows || data.product_classification_template_usages || []).map(decorateClassificationTemplateUsage)
    aliasClassificationTemplateUsages.value = (aliasUsageData.rows || []).map(decorateAliasClassificationTemplateUsage)
    businessGroups.value = Array.isArray(data.business_groups) ? data.business_groups : []
    businessGroupAssignments.value = Array.isArray(data.business_group_assignments) ? data.business_group_assignments : []
    productGroupFeatureSelectionIDs.value = businessGroupFeatureSelectionIDs(productGroupSelectionData)
    productGroupFeatureSelectionDraft.value = [...productGroupFeatureSelectionIDs.value]
    const referencedProductGroups = productCatalogBusinessGroupRows()
    if (!referencedProductGroups.some((group) => Number(group.id || 0) === Number(selectedProductGroupTemplateID.value || 0))) {
      selectedProductGroupTemplateID.value = Number(referencedProductGroups[0]?.id || 0)
    }
    productProductionConfigs.value = data.product_production_configs || []
    industryFieldTemplates.value = industryData?.rows || []
    productUnitDefinitions.value = (data.product_unit_definitions || []).map(decorateProductUnitDefinition)
    productUnitTemplates.value = (data.product_unit_templates || []).map(decorateProductUnitTemplate)
    productPriceGroups.value = (data.product_price_groups || []).map(decorateProductPriceGroup)
    productPriceRecords.value = (data.product_price_records || []).map(decorateProductPriceRecord)
    productTierPriceSchemes.value = (data.product_tier_price_schemes || []).map(decorateProductTierPriceScheme)
    pricingRules.value = (data.product_pricing_rules || data.pricing_rules || []).map((rule) => defaultPricingRuleForm(rule))
    priceTierTemplates.value = (data.price_tier_templates || []).map((template) => defaultPriceTierTemplateForm(template))
    customerProductRuleTemplates.value = (data.customer_product_rule_templates || []).map(decorateCustomerProductRuleTemplate)
    customerProductRuleOverrides.value = (data.customer_product_rule_overrides || []).map(decorateCustomerProductRuleOverride)
    customerProductRuleBindings.value = (data.customer_product_rule_bindings || []).map((row) => ({
      customer_id: Number(row.customer_id || 0),
      template_id: Number(row.template_id || 0),
    }))
    customerPublicUsages.value = (data.customer_public_usages || []).map((row) => ({
      customer_id: Number(row.customer_id || 0),
      use_public_sku: Boolean(row.use_public_sku),
      use_public_categories: Boolean(row.use_public_categories),
      use_public_gradient_templates: Boolean(row.use_public_gradient_templates),
    }))
    customerProductAliases.value = (aliasData.rows || []).map(decorateCustomerProductAlias)
    customers.value = customerSkuCustomerOptions(customerData)
    syncSelectedCustomerSkuCustomer()
    syncSelectedAliasCustomer()
    applyWorkspaceCustomerContext()
    syncVisibleSkuTableState()
    pruneSelectedProducts(filteredSkuRows.value)
  } catch (err) {
    error.value = err.message || '加载失败'
    if (strict) throw err
  } finally {
    loading.value = false
  }
}

function resetGradientTemplateForm() {
  templateForm.value = defaultGradientTemplateForm()
}

function startGradientTemplateEdit(template) {
  templateForm.value = normalizeGradientTemplate(JSON.parse(JSON.stringify(template)))
}

function syncGradientDisplayUnitFromUnitTemplate() {
  const unitTemplate = findProductUnitTemplate(templateForm.value.unit_template_id)
  if (!unitTemplate) return
  templateForm.value.display_unit = unitTemplate.quote_unit || unitTemplate.order_unit || unitTemplate.inventory_unit || templateForm.value.display_unit
}

function addGradientTemplateTier() {
  templateForm.value.tiers.push({
    id: 0,
    label: '',
    min_display_qty: 0,
    max_display_qty: null,
    min_weight_g: 0,
    max_weight_g: null,
    margin_rate: 0,
    position: templateForm.value.tiers.length + 1,
  })
}

function removeGradientTemplateTier(index) {
  templateForm.value.tiers.splice(index, 1)
  templateForm.value.tiers.forEach((tier, tierIndex) => {
    tier.position = tierIndex + 1
  })
}

async function saveGradientTemplate() {
  if (!canEditCurrentTemplate.value) {
    error.value = '公共模板需复制到客户后修改'
    return
  }
  const payload = buildGradientTemplatePayload(templateForm.value)
  if (!payload.id && skuContextCustomerID.value) {
    payload.customer_id = skuContextCustomerID.value
  }
  const errors = validateGradientTemplate(payload)
  if (errors.length) {
    error.value = errors[0]
    return
  }
  templateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/pricing-gradient-templates/${payload.id}` : '/api/pricing-gradient-templates'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '梯度模板已保存'
    resetGradientTemplateForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存梯度模板失败'
  } finally {
    templateSaving.value = false
  }
}

async function deactivateGradientTemplate(id) {
  if (!id) return
  templateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/pricing-gradient-templates/${id}/deactivate`)
    ok.value = '梯度模板已停用'
    resetGradientTemplateForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '停用梯度模板失败'
  } finally {
    templateSaving.value = false
  }
}

async function bindCategoryGradientTemplate(category, templateID) {
  if (!canEditCategory(category)) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const resolvedTemplateID = await resolveGradientTemplateForCategory(category, Number(templateID || 0))
    await apiSend(`/api/product-settings/categories/${category.id}/gradient-template`, {
      body: { gradient_template_id: resolvedTemplateID },
    })
    ok.value = '分类梯度模板已更新，未发布预览会自动按新模板更新'
    await loadAll()
  } catch (err) {
    error.value = err.message || '绑定梯度模板失败'
  } finally {
    loading.value = false
  }
}

async function resolveGradientTemplateForCategory(category, templateID) {
  if (!templateID) return 0
  const template = gradientTemplates.value.find((row) => Number(row.id || 0) === Number(templateID))
  if (!template) return templateID
  const customerID = skuContextCustomerID.value
  if (!customerID || Number(category.customer_id || 0) !== customerID || Number(template.customer_id || 0) !== 0) {
    return templateID
  }
  const response = await apiSend('/api/product-settings/customer-gradient-templates/derive', {
    body: {
      customer_id: customerID,
      source_template_id: templateID,
      name: `${customerName(customerID) || '客户'} - ${template.name}`,
    },
  })
  return Number(response?.template?.id || templateID)
}

function canDeriveGradientTemplate(template) {
  return skuContextCustomerID.value > 0
    && Number(template?.customer_id || 0) === 0
    && template?.active !== false
}

async function deriveGradientTemplateForCustomer(template) {
  const customerID = skuContextCustomerID.value
  if (!customerID) {
    error.value = '请选择履约客户'
    return
  }
  if (!canDeriveGradientTemplate(template)) return
  templateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const response = await apiSend('/api/product-settings/customer-gradient-templates/derive', {
      body: {
        customer_id: customerID,
        source_template_id: Number(template.id || 0),
        name: `${customerName(customerID) || '客户'} - ${template.name}`,
      },
    })
    const derivedID = Number(response?.template?.id || 0)
    ok.value = '公共梯度模板已复制为客户模板，可改名和调整档位'
    await loadAll()
    const derived = gradientTemplates.value.find((row) => Number(row.id || 0) === derivedID)
      || gradientTemplates.value.find((row) => Number(row.customer_id || 0) === customerID && Number(row.source_template_id || 0) === Number(template.id || 0))
    if (derived) startGradientTemplateEdit(derived)
  } catch (err) {
    error.value = err.message || '复制公共梯度模板失败'
  } finally {
    templateSaving.value = false
  }
}

function resetProductConfigTemplateForm() {
  productConfigTemplateForm.value = defaultProductConfigTemplateForm({}, { customerID: skuContextCustomerID.value })
}

function startProductConfigTemplateEdit(template) {
  productConfigTemplateForm.value = defaultProductConfigTemplateForm(JSON.parse(JSON.stringify(template || {})))
}

function validateProductConfigTemplatePayload(payload) {
  if (!String(payload.name || '').trim()) return '请填写商品配置名称'
  const rule = parseJSONSafe(payload.price_list_rule_json)
  if (rule.pricing_mode === 'fixed_unit_price' && !(Number(rule.fixed_unit_price) > 0)) return '固定单价模式必须填写固定单价'
  if (rule.pricing_mode === 'cost_plus' && !Object.prototype.hasOwnProperty.call(rule, 'cost_plus_rate')) return '成本加成模式必须填写加成比例'
  return ''
}

function parseJSONSafe(value) {
  try {
    const parsed = JSON.parse(String(value || '{}'))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

async function saveProductConfigTemplate() {
  if (!canEditCurrentProductConfigTemplate.value) {
    error.value = '公共商品配置需复制到客户后修改'
    return
  }
  const payload = buildProductConfigTemplatePayload({
    ...productConfigTemplateForm.value,
    unit_template_id: productConfigTemplateForm.value.unit_template_id,
  })
  if (!payload.id && skuContextCustomerID.value) {
    payload.customer_id = skuContextCustomerID.value
  }
  const validation = validateProductConfigTemplatePayload(payload)
  if (validation) {
    error.value = validation
    return
  }
  productConfigSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/product-settings/product-config-templates/${payload.id}` : '/api/product-settings/product-config-templates'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '商品配置模板已保存，引用该模板的商品档案会同步使用新规则'
    resetProductConfigTemplateForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存商品配置失败'
  } finally {
    productConfigSaving.value = false
  }
}

async function deactivateProductConfigTemplate(id) {
  if (!id) return
  productConfigTemplateForm.value.active = false
  await saveProductConfigTemplate()
}

async function deleteProductConfigTemplate(id) {
  const templateID = Number(id || 0)
  if (!templateID) return
  if (typeof window !== 'undefined' && !window.confirm(`确认删除商品配置模板「${productConfigTemplateForm.value.name || templateID}」？已引用该模板的历史商品档案和价格表不会回改。`)) return
  productConfigSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/product-config-templates/${templateID}`, { method: 'DELETE' })
    ok.value = '商品配置模板已删除，新的商品档案和客户商品不再引用该模板'
    resetProductConfigTemplateForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '删除商品配置模板失败'
  } finally {
    productConfigSaving.value = false
  }
}

function resetProductPriceRecordForm() {
  productPriceRecordForm.value = defaultProductPriceRecordForm()
}

function startProductPriceRecordEdit(record) {
  productPriceRecordForm.value = defaultProductPriceRecordForm(JSON.parse(JSON.stringify(record || {})))
}

function resetPricingRuleForm() {
  pricingRuleForm.value = defaultPricingRuleForm()
  error.value = ''
  ok.value = ''
  openPricingRuleEditorDrawer()
}

function startPricingRuleEdit(rule) {
  pricingRuleForm.value = defaultPricingRuleForm(JSON.parse(JSON.stringify(rule || {})))
  error.value = ''
  ok.value = ''
  openPricingRuleEditorDrawer()
}

function openPricingRuleEditorDrawer() {
  if (typeof document !== 'undefined' && !pricingRuleEditorDrawerOpen.value) {
    pricingRuleEditorReturnFocus = document.activeElement
  }
  pricingRuleEditorDrawerOpen.value = true
  nextTick(() => {
    const firstField = pricingRuleEditorDrawer.value?.querySelector('input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled])')
    ;(firstField || pricingRuleEditorDrawer.value)?.focus?.()
  })
}

function closePricingRuleEditor() {
  pricingRuleEditorDrawerOpen.value = false
  error.value = ''
  ok.value = ''
  const returnFocus = pricingRuleEditorReturnFocus
  pricingRuleEditorReturnFocus = null
  nextTick(() => returnFocus?.focus?.())
}

function trapPricingRuleEditorFocus(event) {
  const drawer = pricingRuleEditorDrawer.value
  if (!drawer) return
  const focusable = [...drawer.querySelectorAll('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [href], [tabindex]:not([tabindex="-1"])')]
    .filter((element) => element.getAttribute('aria-hidden') !== 'true')
  if (!focusable.length) {
    event.preventDefault()
    drawer.focus()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

function pricingRuleNeedsMarkupConfirmation(rule = {}) {
  const calculation = pricingRuleCalculationFromRule(rule)
  const rawMethod = String(calculation.profit_method ?? rule.profit_method ?? '').trim().toLowerCase()
  return Boolean(
    String(calculation.legacy_profit_method || '').trim()
    || String(calculation.migration_warning || '').trim()
    || (rawMethod && !['markup', 'gross_margin'].includes(rawMethod))
  )
}

function pricingRuleLegacyMethodLabel(rule = {}) {
  const calculation = pricingRuleCalculationFromRule(rule)
  return String(calculation.legacy_profit_method || calculation.profit_method || '未知').trim() || '未知'
}

function pricingRuleLegacyValueLabel(rule = {}) {
  const calculation = pricingRuleCalculationFromRule(rule)
  const value = calculation.legacy_margin_rate ?? calculation.legacy_fixed_amount ?? rule.margin_rate
  return value === null || typeof value === 'undefined' || String(value).trim() === '' ? '未记录' : String(value)
}

function pricingRuleOptionLabel(rule = {}) {
  const name = String(rule.name || rule.code || `Pricing Rule #${rule.id || ''}`).trim()
  const version = String(rule.formula_version || 'v1').trim()
  return version ? `${name} / ${version}` : name
}

function activePricingRuleTrialOptionByID(id) {
  const ruleID = Number(id || 0)
  return activePricingRuleTrialOptions.value.find((rule) => Number(rule.id || 0) === ruleID) || null
}

function openPricingRuleTrial(rule = null) {
  const normalized = rule && rule.active !== false ? defaultPricingRuleForm(JSON.parse(JSON.stringify(rule || {}))) : null
  pricingRuleTrialRule.value = normalized
  pricingRuleTrialForm.value = defaultPricingRuleTrialForm(normalized || {})
  activePricingRuleTrialProductKindFilter.value = ''
  pricingRuleTrialResult.value = null
  pricingRuleTrialActiveExplanation.value = ''
  pricingRuleTrialError.value = ''
  pricingRuleTrialUpdateMessage.value = ''
  pricingRuleTrialDrawerOpen.value = true
}

function setPricingRuleTrialProductKindFilter(value) {
  const next = String(value || '').trim()
  activePricingRuleTrialProductKindFilter.value = next
  const selected = selectedPricingRuleTrialProduct.value
  if (!selected || !next || normalizedProductKind(selected) === next) return
  pricingRuleTrialForm.value.parent_product_id = 0
  pricingRuleTrialForm.value.product_id = 0
  pricingRuleTrialForm.value.quote_unit = ''
  pricingRuleTrialForm.value.bom_version_id = 0
  pricingRuleTrialForm.value.process_route_id = 0
  pricingRuleTrialForm.value.operation_template_id = 0
  pricingRuleTrialResult.value = null
  pricingRuleTrialActiveExplanation.value = ''
}

function handlePricingRuleTrialRuleChange() {
  const selected = activePricingRuleTrialOptionByID(pricingRuleTrialForm.value.pricing_rule_id)
  const previous = { ...pricingRuleTrialForm.value }
  pricingRuleTrialRule.value = selected ? defaultPricingRuleForm(JSON.parse(JSON.stringify(selected))) : null
  const next = defaultPricingRuleTrialForm(pricingRuleTrialRule.value || {})
  next.parent_product_id = Number(previous.parent_product_id || 0)
  next.product_id = Number(previous.product_id || 0)
  next.customer_id = Number(previous.customer_id || 0)
  next.quote_unit = String(previous.quote_unit || '')
  next.bom_version_id = 0
  next.process_route_id = 0
  next.operation_template_id = 0
  pricingRuleTrialForm.value = next
  pricingRuleTrialResult.value = null
  pricingRuleTrialActiveExplanation.value = ''
  pricingRuleTrialError.value = ''
  pricingRuleTrialUpdateMessage.value = ''
}

function closePricingRuleTrial() {
  pricingRuleTrialDrawerOpen.value = false
  if (pricingRuleTrialAutoRunTimer) {
    clearTimeout(pricingRuleTrialAutoRunTimer)
    pricingRuleTrialAutoRunTimer = null
  }
  pricingRuleTrialRunID++
  pricingRuleTrialLoading.value = false
  pricingRuleTrialRule.value = null
  pricingRuleTrialResult.value = null
  pricingRuleTrialActiveExplanation.value = ''
  pricingRuleTrialError.value = ''
  pricingRuleTrialUpdateMessage.value = ''
}

function openPricingRuleTrialExplanation(kind) {
  pricingRuleTrialActiveExplanation.value = String(kind || '').trim()
}

function closePricingRuleTrialExplanation() {
  pricingRuleTrialActiveExplanation.value = ''
}

function addPricingRuleTrialOtherCostRow() {
  pricingRuleTrialForm.value.other_cost_rows.push(defaultPricingRuleTrialOtherCostRow())
}

function removePricingRuleTrialOtherCostRow(index) {
  pricingRuleTrialForm.value.other_cost_rows.splice(index, 1)
  if (!pricingRuleTrialForm.value.other_cost_rows.length) {
    addPricingRuleTrialOtherCostRow()
  }
}

function syncPricingRuleTrialProductionSelections(result = {}) {
  if (!result || Number(result.product_id || 0) !== Number(pricingRuleTrialForm.value.product_id || 0)) return
  const bomOptions = Array.isArray(result.bom_version_options) ? result.bom_version_options : []
  const selectedBomID = Number(result.bom_version_id || 0)
  if (selectedBomID > 0 && !Number(pricingRuleTrialForm.value.bom_version_id || 0)) {
    pricingRuleTrialForm.value.bom_version_id = selectedBomID
  } else if (Number(pricingRuleTrialForm.value.bom_version_id || 0) > 0 && !bomOptions.some((option) => Number(option.version_id || 0) === Number(pricingRuleTrialForm.value.bom_version_id || 0))) {
    pricingRuleTrialForm.value.bom_version_id = selectedBomID || 0
  }
  const routeOptions = Array.isArray(result.process_route_options) ? result.process_route_options : []
  const selectedRouteID = Number(result.process_route_id || 0)
  if (selectedRouteID > 0 && !Number(pricingRuleTrialForm.value.process_route_id || 0)) {
    pricingRuleTrialForm.value.process_route_id = selectedRouteID
  } else if (Number(pricingRuleTrialForm.value.process_route_id || 0) > 0 && !routeOptions.some((option) => Number(option.id || 0) === Number(pricingRuleTrialForm.value.process_route_id || 0))) {
    pricingRuleTrialForm.value.process_route_id = selectedRouteID || 0
  }
  const operationOptions = Array.isArray(result.operation_template_options) ? result.operation_template_options : []
  const selectedOperationID = Number(result.operation_template_id || 0)
  if (selectedOperationID > 0 && !Number(pricingRuleTrialForm.value.operation_template_id || 0)) {
    pricingRuleTrialForm.value.operation_template_id = selectedOperationID
  } else if (Number(pricingRuleTrialForm.value.operation_template_id || 0) > 0 && !operationOptions.some((option) => Number(option.id || 0) === Number(pricingRuleTrialForm.value.operation_template_id || 0))) {
    pricingRuleTrialForm.value.operation_template_id = selectedOperationID || 0
  }
}

function pricingRuleTrialBomVersionOptionLabel(option = {}) {
  const code = String(option.bom_code || '').trim()
  const name = String(option.bom_name || '').trim()
  const version = String(option.version_no || '').trim()
  const defaultText = option.is_default ? ' 默认' : ''
  const statusText = String(option.status || '').trim().toLowerCase() === 'draft' ? ' 草稿，仅供试算' : ''
  return `${[code, name].filter(Boolean).join(' ')}${version ? ` / ${version}` : ''}${defaultText}${statusText}`.trim() || `BOM版本 #${option.version_id || ''}`
}

function navigatePricingRuleTrialBom() {
  const option = selectedPricingRuleTrialBomVersion.value
  const bomID = Number(option?.bom_id || 0)
  if (!bomID) {
    pricingRuleTrialError.value = '当前试算版本缺少可配置的 BOM'
    return
  }
  const returnKey = storePricingRuleTrialReturnState({
    form: pricingRuleTrialForm.value,
    product_kind_filter: activePricingRuleTrialProductKindFilter.value,
  })
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'bom',
      params: { production_bom_id: bomID },
      returnNavigation: {
        key: 'productPriceManagement',
        label: '返回价格试算',
        params: { pricing_rule_trial_return_key: returnKey },
      },
    },
  }))
}

function pricingRuleTrialProcessRouteOptionLabel(option = {}) {
  const name = String(option.name || '').trim() || `工艺路线 #${option.id || ''}`
  return option.is_default ? `${name} 默认` : name
}

function pricingRuleTrialOperationTemplateOptionLabel(option = {}) {
  const name = String(option.name || '').trim() || `工序模板 #${option.id || ''}`
  return option.is_default ? `${name} 默认` : name
}

function schedulePricingRuleTrial() {
  if (restoringPricingRuleTrialReturnState) return
  pricingRuleTrialRunID++
  pricingRuleTrialLoading.value = false
  if (pricingRuleTrialAutoRunTimer) {
    clearTimeout(pricingRuleTrialAutoRunTimer)
    pricingRuleTrialAutoRunTimer = null
  }
  if (!pricingRuleTrialDrawerOpen.value) return
  const payload = buildPricingRuleTrialPayload(pricingRuleTrialForm.value)
  if (!payload.pricing_rule_id || !payload.product_id || !String(payload.quote_unit || '').trim()) {
    pricingRuleTrialResult.value = null
    pricingRuleTrialActiveExplanation.value = ''
    return
  }
  pricingRuleTrialAutoRunTimer = setTimeout(() => {
    pricingRuleTrialAutoRunTimer = null
    runPricingRuleTrial()
  }, 250)
}

async function runPricingRuleTrial() {
  const payload = buildPricingRuleTrialPayload(pricingRuleTrialForm.value)
  if (!payload.pricing_rule_id) {
    pricingRuleTrialError.value = '请选择价格计算模板'
    return
  }
  if (!Number(pricingRuleTrialForm.value.parent_product_id || 0)) {
    pricingRuleTrialError.value = '请选择试算商品'
    return
  }
  if (!payload.product_id) {
    pricingRuleTrialError.value = '请选择销售规格'
    return
  }
  if (!String(payload.quote_unit || '').trim()) {
    pricingRuleTrialError.value = '所选销售规格缺少有效销售单位，请先在商品档案维护规格'
    return
  }
  const runID = ++pricingRuleTrialRunID
  pricingRuleTrialLoading.value = true
  pricingRuleTrialError.value = ''
  try {
    const result = await apiSend('/api/costing/pricing-rule-trial', { method: 'POST', body: payload })
    if (runID === pricingRuleTrialRunID) {
      pricingRuleTrialResult.value = result
      pricingRuleTrialActiveExplanation.value = ''
      syncPricingRuleTrialProductionSelections(result)
    }
  } catch (err) {
    if (runID === pricingRuleTrialRunID) {
      pricingRuleTrialResult.value = null
      pricingRuleTrialActiveExplanation.value = ''
      pricingRuleTrialError.value = err.message || '价格计算模板试算失败'
    }
  } finally {
    if (runID === pricingRuleTrialRunID) pricingRuleTrialLoading.value = false
  }
}

async function updatePricingRuleFromTrial() {
  const ruleID = Number(pricingRuleTrialForm.value.pricing_rule_id || 0)
  const rule = activePricingRuleTrialOptionByID(ruleID) || pricingRuleTrialRule.value
  if (!rule || ruleID <= 0) {
    pricingRuleTrialError.value = '请选择价格计算模板'
    return
  }
  const confirmed = typeof window === 'undefined' || window.confirm('确认将本次试算的临时加价率、已填写临时税率和其他成本更新到价格计算模板？商品、BOM、工艺路线和销售规格不会写入模板，已发布价格表不会自动重算。')
  if (!confirmed) return
  const payload = buildPricingRuleUpdateFromTrial(rule, pricingRuleTrialForm.value)
  productPriceSaving.value = true
  pricingRuleTrialError.value = ''
  pricingRuleTrialUpdateMessage.value = ''
  try {
    const result = await apiSend(`/api/product-pricing-rules/${payload.id}`, { method: 'PUT', body: payload })
    const row = defaultPricingRuleForm(result.rule || payload)
    pricingRules.value = [
      row,
      ...pricingRules.value.filter((item) => Number(item.id || 0) !== Number(row.id || 0)),
    ]
    pricingRuleTrialRule.value = defaultPricingRuleForm(JSON.parse(JSON.stringify(row)))
    pricingRuleTrialUpdateMessage.value = '价格计算模板参数已更新；当前试算上下文已保留'
    schedulePricingRuleTrial()
  } catch (err) {
    pricingRuleTrialError.value = err.message || '更新价格计算模板失败'
  } finally {
    productPriceSaving.value = false
  }
}

function addPricingRuleOtherCostRow() {
  pricingRuleForm.value.other_cost_rows.push(defaultPricingRuleOtherCostRow())
}

function removePricingRuleOtherCostRow(index) {
  pricingRuleForm.value.other_cost_rows.splice(index, 1)
  if (!pricingRuleForm.value.other_cost_rows.length) {
    addPricingRuleOtherCostRow()
  }
}

function resetPriceTierTemplateForm() {
  priceTierTemplateForm.value = defaultPriceTierTemplateForm()
}

function startPriceTierTemplateEdit(template) {
  priceTierTemplateForm.value = defaultPriceTierTemplateForm(JSON.parse(JSON.stringify(template || {})))
}

function addPriceTierTemplateTier() {
  priceTierTemplateForm.value.tiers.push(defaultPriceTierTemplateTier({}, priceTierTemplateForm.value.tiers.length))
}

function removePriceTierTemplateTier(index) {
  priceTierTemplateForm.value.tiers.splice(index, 1)
  if (!priceTierTemplateForm.value.tiers.length) {
    addPriceTierTemplateTier()
  }
}

function pricingRuleCostSourceLabel() {
  return '生产 BOM 成本（物料+工序）'
}

function pricingRuleRoundingLabel(value) {
  return {
    none: '不取整',
    jiao: '保留到角',
    yuan: '保留到元',
  }[String(value || '')] || '不取整'
}

function percentDisplay(value) {
  const n = Number(value || 0)
  return n > 0 ? `${(n * 100).toFixed(2).replace(/\.?0+$/, '')}%` : '-'
}

function trialMoneyDisplay(value, unit = '') {
  const n = Number(value)
  if (!Number.isFinite(n)) return '-'
  const amount = n.toFixed(2).replace(/\.?0+$/, '')
  const normalizedUnit = String(unit || '').trim()
  return normalizedUnit ? `${amount}/${normalizedUnit}` : amount
}

function pricingRuleTrialStepDisplay(step = {}) {
  const n = Number(step.value)
  if (!Number.isFinite(n)) return '-'
  if (step.unit === 'ratio') return percentDisplay(n)
  return trialMoneyDisplay(n, step.unit)
}

function pricingRuleTrialStepSourceDisplay(step = {}) {
  const source = String(step.source || '').trim()
  if (!source) return '-'
  const sourceMap = {
    standard_manufacturing_cost: '标准制造成本',
    product_bom_operation_cost: '当前商品 BOM/工序成本',
    pricing_rule: '价格计算模板',
    trial_override: '本次临时录入',
    finance_settings: '财务设置',
    tax_disabled: '不计税',
    formula: '公式计算',
    temporary_override: '本次临时录入',
    temporary_other_costs: '本次临时录入',
    temporary_post_markup_costs: '本次临时录入',
    pricing_rule_other_costs: '价格计算模板',
    pricing_rule_post_markup_costs: '价格计算模板',
    bom_operation_snapshot: 'BOM工序成本快照',
    bom_operation_snapshot_missing: 'BOM工序成本快照缺失',
    operation_master: '工序列表（历史）',
    override: '本次临时录入',
    product_bom: '当前商品 BOM',
    default: '系统默认',
  }
  if (sourceMap[source]) return sourceMap[source]
  if (/^[a-z0-9_:. -]+$/i.test(source)) return '-'
  return source
}

function pricingRuleTrialBaseCostRows(result = {}, kind = 'material') {
  const rows = Array.isArray(result?.base_cost_details) ? result.base_cost_details : []
  if (kind === 'all') return rows
  if (kind === 'operation') return rows.filter((row) => String(row?.type || '') === 'operation')
  return rows.filter((row) => String(row?.type || '') !== 'operation')
}

function pricingRuleTrialCapacitySourceLabel(row = {}) {
  const source = String(row?.capacity_selection_source || '').trim()
  if (!source) return '-'
  return pricingRuleTrialSourceDisplay(source)
}

function pricingRuleTrialOtherCostRows(result = {}) {
  return Array.isArray(result?.other_cost_details) ? result.other_cost_details : []
}

function pricingRuleTrialProfitExplanation(result = {}) {
  return result?.profit_explanation && typeof result.profit_explanation === 'object' ? result.profit_explanation : {}
}

function pricingRuleTrialHasRoundingAdjustment(result = {}) {
  const amount = Number(result?.rounding_adjustment || 0)
  return Number.isFinite(amount) && Math.abs(amount) > 0.000001
}

function pricingRuleTrialExplanationTitle(kind = '') {
  return {
    base_cost: '标准制造成本',
    other_cost: '其他成本',
    profit_markup: '加价增加',
  }[String(kind || '').trim()] || '试算说明'
}

function pricingRuleTrialSourceDisplay(source = '') {
  return pricingRuleTrialStepSourceDisplay({ source })
}

function pricingRuleTrialOtherCostSettingLocation(source = '') {
  return String(source || '').trim() === 'temporary_override'
    ? '本次试算抽屉「其他成本」'
    : '价格计算模板编辑区「其他成本」'
}

function pricingRuleTrialProfitRateDisplay(explanation = {}, unit = '') {
  const method = String(explanation.method || '').trim()
  const rate = Number(explanation.rate)
  if (!Number.isFinite(rate)) return '-'
  if (method === 'fixed_add') return trialMoneyDisplay(rate, unit)
  return percentDisplay(rate)
}

function pricingRuleTrialBaseCostTypeLabel(type = '') {
  const value = String(type || '').trim()
  if (value === 'operation') return '工序'
  if (value === 'component_product' || value === 'product' || value === 'finished_product') return '成品组件'
  if (value === 'temporary_override') return '临时覆盖'
  return '物料'
}

function pricingRuleTrialBaseCostUsage(row = {}) {
  const consumeUnit = String(row.consume_unit || '').trim()
  const quantity = Number(row.quantity || 0)
  const ratioPct = Number(row.ratio_pct || 0)
  if (consumeUnit === 'ratio_pct') return `比例 ${percentDisplay(ratioPct / 100)}`
  if (quantity > 0) return `${pricingRuleTrialConsumeUnitLabel(consumeUnit)} ${quantity.toFixed(4).replace(/\.?0+$/, '')}`
  return pricingRuleTrialConsumeUnitLabel(consumeUnit) || '-'
}

function pricingRuleTrialBaseCostRecipeUsage(row = {}) {
  const consumeUnit = String(row.consume_unit || '').trim()
  if (consumeUnit !== 'ratio_pct') return pricingRuleTrialBaseCostUsage(row)
  const explicitRecipeRatio = Number(row.recipe_ratio_pct)
  const ratioPct = Number(row.ratio_pct || 0)
  const lossRate = Number(row.material_loss_rate || 0)
  const fallbackRecipeRatio = lossRate > 0 && lossRate < 1 ? ratioPct * (1 - lossRate) : ratioPct
  const recipeRatio = Number.isFinite(explicitRecipeRatio) && explicitRecipeRatio > 0 ? explicitRecipeRatio : fallbackRecipeRatio
  return recipeRatio > 0 ? `原比例 ${percentDisplay(recipeRatio / 100)}` : '-'
}

function pricingRuleTrialBaseCostLossRate(row = {}) {
  if (String(row.consume_unit || '').trim() !== 'ratio_pct') return '-'
  const lossRate = Number(row.material_loss_rate || 0)
  return lossRate > 0 ? percentDisplay(lossRate) : '-'
}

function pricingRuleTrialBaseCostEffectiveUsage(row = {}) {
  const consumeUnit = String(row.consume_unit || '').trim()
  if (consumeUnit !== 'ratio_pct') return pricingRuleTrialBaseCostUsage(row)
  const explicitEffectiveRatio = Number(row.effective_ratio_pct)
  const ratioPct = Number(row.ratio_pct || 0)
  const effectiveRatio = Number.isFinite(explicitEffectiveRatio) && explicitEffectiveRatio > 0 ? explicitEffectiveRatio : ratioPct
  return effectiveRatio > 0 ? `有效比例 ${percentDisplay(effectiveRatio / 100)}` : '-'
}

function pricingRuleTrialBaseCostUnitCostValue(row = {}) {
  const costUnitCost = Number(row.cost_unit_cost)
  if (String(row.cost_unit || '').trim() && Number.isFinite(costUnitCost)) return costUnitCost
  return Number(row.unit_cost || 0)
}

function pricingRuleTrialBaseCostUnit(row = {}, result = {}) {
  const costUnit = String(row.cost_unit || '').trim()
  if (costUnit) return costUnit
  return row.unit || result.quote_unit
}

function pricingRuleTrialConsumeUnitLabel(value = '') {
  return {
    ratio_pct: '比例',
    g_per_bag: '每袋克数',
    unit_per_bag: '每袋数量',
    unit_per_box: '每盒数量',
    fixed_qty: '固定数量',
    unit: '数量',
    g: '克',
    kg: '千克',
    length: '长度',
    area: '面积',
    per_kg: '每 kg',
    per_kg_output: '每 kg 成品',
    per_finished_kg: '每 kg 成品',
    fixed: '固定',
    per_unit: '每单位',
    per_quote_unit: '每销售单位',
    process_route: '工艺路线计划成本',
    standard_operation: '标准工序成本',
  }[String(value || '').trim()] || String(value || '').trim()
}

function pricingRuleTrialBaseCostMissing(result = {}) {
  return Number(result?.base_cost || 0) <= 0
}

function pricingRuleTrialTaxInPriceAmount(result = {}) {
  if (Object.prototype.hasOwnProperty.call(result || {}, 'tax_in_price_amount')) return Number(result.tax_in_price_amount || 0)
  return Number(result?.tax_amount || 0)
}

function pricingRuleTrialTaxWaterfallNote(result = {}) {
  const unit = result?.quote_unit
  const taxAmount = Number(result?.tax_amount || 0)
  const taxInPriceAmount = pricingRuleTrialTaxInPriceAmount(result)
  if (taxAmount > 0 && Math.abs(taxAmount - taxInPriceAmount) > 0.0001) {
    return `税额另计 ${trialMoneyDisplay(taxAmount, unit)}；取整前 ${trialMoneyDisplay(result?.final_before_rounding, unit)}`
  }
  return `取整前 ${trialMoneyDisplay(result?.final_before_rounding, unit)}`
}

function pricingRuleTrialTaxSourceLabel(result = {}) {
  return pricingRuleTrialSourceDisplay(result?.tax_rate_source || '')
}

function pricingRuleTrialRoundingSourceLabel(result = {}) {
  const source = pricingRuleTrialSourceDisplay(result?.rounding_rule_source || 'pricing_rule')
  return `按模板取整（${source}）`
}

function pricingRuleTrialTaxRateSummary(result = null, rule = null) {
  if (result?.tax_rate_source) {
    const step = (Array.isArray(result.steps) ? result.steps : []).find((row) => row?.key === 'tax_rate')
    const rateText = step ? pricingRuleTrialStepDisplay(step) : '-'
    return `${rateText}（${pricingRuleTrialTaxSourceLabel(result)}）`
  }
  const rate = Number(rule?.tax_rate || 0)
  if (rate > 0) return `${percentDisplay(rate)}（来自价格计算模板）`
  return '按财务设置'
}

async function savePricingRule() {
  if (pricingRuleNeedsMarkupConfirmation(pricingRuleForm.value)) {
    error.value = '旧价格方式无法安全换算；请新建加价率模板'
    return
  }
  const payload = buildPricingRulePayload(pricingRuleForm.value)
  if (!payload.name) {
    error.value = '请填写价格计算模板名称'
    return
  }
  productPriceSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/product-pricing-rules/${payload.id}` : '/api/product-pricing-rules'
    const method = payload.id ? 'PUT' : 'POST'
    const result = await apiSend(url, { method, body: payload })
    const row = defaultPricingRuleForm(result.rule || payload)
    pricingRules.value = [
      row,
      ...pricingRules.value.filter((item) => Number(item.id || 0) !== Number(row.id || 0)),
    ]
    pricingRuleForm.value = row
    ok.value = '价格计算模板已保存'
  } catch (err) {
    error.value = err.message || '保存价格计算模板失败'
  } finally {
    productPriceSaving.value = false
  }
}

async function copyPricingRule(rule) {
  if (pricingRuleNeedsMarkupConfirmation(rule)) {
    startPricingRuleEdit(rule)
    return
  }
  const payload = buildPricingRuleCopyPayload(rule || {}, pricingRules.value)
  if (!payload.name) {
    error.value = '请选择可复制的价格计算模板'
    return
  }
  productPriceSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/product-pricing-rules', { method: 'POST', body: payload })
    const row = defaultPricingRuleForm(result.rule || payload)
    pricingRules.value = [
      row,
      ...pricingRules.value.filter((item) => Number(item.id || 0) !== Number(row.id || 0)),
    ]
    pricingRuleForm.value = row
    ok.value = '价格计算模板已复制'
    openPricingRuleEditorDrawer()
  } catch (err) {
    error.value = err.message || '复制价格计算模板失败'
  } finally {
    productPriceSaving.value = false
  }
}

async function deactivatePricingRule(rule) {
  const form = defaultPricingRuleForm(JSON.parse(JSON.stringify(rule || {})))
  if (!form.id) return
  form.active = false
  const payload = buildPricingRulePayload(form)
  productPriceSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend(`/api/product-pricing-rules/${payload.id}`, { method: 'PUT', body: payload })
    const row = defaultPricingRuleForm(result.rule || payload)
    pricingRules.value = [
      row,
      ...pricingRules.value.filter((item) => Number(item.id || 0) !== Number(row.id || 0)),
    ]
    pricingRuleForm.value = row
    ok.value = '价格计算模板已失效'
  } catch (err) {
    error.value = err.message || '价格计算模板失效失败'
  } finally {
    productPriceSaving.value = false
  }
}

async function savePriceTierTemplate() {
  const payload = buildPriceTierTemplatePayload(priceTierTemplateForm.value)
  if (!payload.name) {
    error.value = '请填写阶梯模板名称'
    return
  }
  if (!payload.tiers.length) {
    error.value = '请至少维护一个档位'
    return
  }
  productPriceSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/price-tier-templates/${payload.id}` : '/api/price-tier-templates'
    const method = payload.id ? 'PUT' : 'POST'
    const result = await apiSend(url, { method, body: payload })
    const row = defaultPriceTierTemplateForm(result.template || payload)
    priceTierTemplates.value = [
      row,
      ...priceTierTemplates.value.filter((item) => Number(item.id || 0) !== Number(row.id || 0)),
    ]
    priceTierTemplateForm.value = row
    ok.value = '阶梯模板已保存'
  } catch (err) {
    error.value = err.message || '阶梯模板保存失败'
  } finally {
    productPriceSaving.value = false
  }
}

function validateProductPriceRecordPayload(payload) {
  if (Number(payload.product_id || 0) <= 0 && Number(payload.customer_product_alias_id || 0) <= 0) return '请选择商品或客户商品'
  if (!(Number(payload.final_unit_price || 0) > 0)) return '请填写最终单价'
  if (!String(payload.price_unit || '').trim()) return '请选择价格单位'
  return ''
}

async function saveProductPriceRecord() {
  const payload = buildProductPriceRecordPayload(productPriceRecordForm.value)
  const validation = validateProductPriceRecordPayload(payload)
  if (validation) {
    error.value = validation
    return
  }
  productPriceSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/product-price-records/${payload.id}` : '/api/product-price-records'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '商品价格记录已保存'
    await loadAll()
    resetProductPriceRecordForm()
  } catch (err) {
    error.value = err.message || '保存商品价格记录失败'
  } finally {
    productPriceSaving.value = false
  }
}

function resetProductTierPriceSchemeForm() {
  productTierPriceSchemeForm.value = defaultProductTierPriceSchemeForm()
}

function startProductTierPriceSchemeEdit(scheme) {
  productTierPriceSchemeForm.value = defaultProductTierPriceSchemeForm(JSON.parse(JSON.stringify(scheme || {})))
}

function addProductTierPriceSchemeTier() {
  productTierPriceSchemeForm.value.tiers.push(defaultProductTierPriceSchemeTier({}, productTierPriceSchemeForm.value.tiers.length))
}

function removeProductTierPriceSchemeTier(index) {
  productTierPriceSchemeForm.value.tiers.splice(index, 1)
  if (!productTierPriceSchemeForm.value.tiers.length) {
    addProductTierPriceSchemeTier()
  }
}

function validateProductTierPriceSchemePayload(payload) {
  if (!String(payload.name || '').trim()) return '请填写阶梯方案名称'
  if (Number(payload.product_id || 0) <= 0 && Number(payload.customer_product_alias_id || 0) <= 0) return '请选择商品或客户商品'
  if (!payload.tiers.length) return '请至少添加一个档位'
  if (payload.tiers.some((tier) => Number(tier.source_price_record_id || 0) <= 0)) return '每个档位都要引用价格记录'
  return ''
}

async function saveProductTierPriceScheme() {
  const payload = buildProductTierPriceSchemePayload(productTierPriceSchemeForm.value)
  const validation = validateProductTierPriceSchemePayload(payload)
  if (validation) {
    error.value = validation
    return
  }
  productPriceSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/product-tier-price-schemes/${payload.id}` : '/api/product-tier-price-schemes'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '阶梯价格方案已保存'
    await loadAll()
    resetProductTierPriceSchemeForm()
  } catch (err) {
    error.value = err.message || '保存阶梯价格方案失败'
  } finally {
    productPriceSaving.value = false
  }
}

function resetClassificationTemplateForm() {
  classificationTemplateForm.value = defaultClassificationTemplateForm()
  classificationCategoryForm.value = defaultClassificationCategoryForm()
}

function openClassificationTemplateCreateDrawer() {
  classificationTemplateCreateForm.value = defaultClassificationTemplateForm()
  classificationTemplateCreateDrawerOpen.value = true
}

function closeClassificationTemplateCreateDrawer() {
  classificationTemplateCreateDrawerOpen.value = false
}

function startClassificationTemplateEdit(template) {
  classificationTemplateForm.value = defaultClassificationTemplateForm(JSON.parse(JSON.stringify(template || {})))
  classificationCategoryForm.value = defaultClassificationCategoryForm()
}

function classificationTemplatePayload(form) {
  return {
    customer_id: 0,
    source_template_id: Number(form.source_template_id || 0),
    name: String(form.name || '').trim(),
    remark: String(form.remark || '').trim(),
    product_config_template_id: Number(form.product_config_template_id || 0),
    gradient_template_id: 0,
    unit_template_id: 0,
    sort_order: Number(form.sort_order || 100),
    active: form.active !== false,
  }
}

async function saveClassificationTemplate() {
  const payload = classificationTemplatePayload(classificationTemplateForm.value)
  if (!payload.name) {
    error.value = '请填写分类模板名称'
    return
  }
  const id = Number(classificationTemplateForm.value.id || 0)
  classificationTemplateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(id ? `/api/product-classification-templates/${id}` : '/api/product-classification-templates', {
      method: id ? 'PUT' : 'POST',
      body: payload,
    })
    ok.value = '分类模板已保存'
    resetClassificationTemplateForm()
    await refreshClassificationTemplates()
  } catch (err) {
    error.value = err.message || '保存分类模板失败'
  } finally {
    classificationTemplateSaving.value = false
  }
}

async function saveClassificationTemplateCreate() {
  const payload = classificationTemplatePayload(classificationTemplateCreateForm.value)
  if (!payload.name) {
    error.value = '请填写分类模板名称'
    return
  }
  classificationTemplateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/product-classification-templates', {
      method: 'POST',
      body: payload,
    })
    ok.value = '分类模板已创建'
    classificationTemplateCreateDrawerOpen.value = false
    classificationTemplateCreateForm.value = defaultClassificationTemplateForm()
    await refreshClassificationTemplates()
    if (result?.template) startClassificationTemplateEdit(result.template)
  } catch (err) {
    error.value = err.message || '创建分类模板失败'
  } finally {
    classificationTemplateSaving.value = false
  }
}

async function deleteClassificationTemplate(id) {
  const templateID = Number(id || 0)
  if (!templateID) return
  if (!window.confirm('删除分类模板？商品档案和客户商品的历史记录不会回改。')) return
  classificationTemplateSaving.value = true
  error.value = ''
  try {
    await apiSend(`/api/product-classification-templates/${templateID}`, { method: 'DELETE' })
    ok.value = '分类模板已删除'
    resetClassificationTemplateForm()
    await refreshClassificationTemplates()
  } catch (err) {
    error.value = err.message || '删除分类模板失败'
  } finally {
    classificationTemplateSaving.value = false
  }
}

function openGlobalUnitDictionaryDrawer() {
  globalUnitDrawerOpen.value = true
}

function closeGlobalUnitDictionaryDrawer() {
  globalUnitDrawerOpen.value = false
}

function resetGlobalUnitDefinitionForm() {
  globalUnitForm.value = defaultProductUnitDefinitionForm()
  globalUnitEditingCode.value = ''
}

function startGlobalUnitDefinitionEdit(unit) {
  globalUnitForm.value = defaultProductUnitDefinitionForm(JSON.parse(JSON.stringify(unit || {})))
  globalUnitEditingCode.value = String(unit?.code || '')
}

function validateGlobalUnitDefinitionPayload(payload) {
  if (!String(payload.code || '').trim()) return '请填写单位编码'
  if (!String(payload.name || '').trim()) return '请填写单位名称'
  return ''
}

async function saveGlobalUnitDefinitionFromDrawer() {
  const payload = buildProductUnitDefinitionPayload(globalUnitForm.value)
  const validation = validateGlobalUnitDefinitionPayload(payload)
  if (validation) {
    error.value = validation
    return
  }
  globalUnitSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const editingCode = globalUnitEditingCode.value
    const url = editingCode ? `/api/product-settings/units/${encodeURIComponent(editingCode)}` : '/api/product-settings/units'
    const method = editingCode ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '全局单位已保存，可在销售规格模板中引用'
    await loadAll()
    resetGlobalUnitDefinitionForm()
  } catch (err) {
    error.value = err.message || '保存全局单位失败'
  } finally {
    globalUnitSaving.value = false
  }
}

async function deleteGlobalUnitDefinitionFromDrawer() {
  const editingCode = globalUnitEditingCode.value
  if (!editingCode) return
  if (typeof window !== 'undefined' && !window.confirm(`确认删除全局单位「${globalUnitForm.value.name || editingCode}」？已引用该单位的历史配置不会被物理删除。`)) return
  globalUnitSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/units/${encodeURIComponent(editingCode)}`, { method: 'DELETE' })
    ok.value = '全局单位已删除，新的销售规格模板将不再引用该单位'
    await loadAll()
    resetGlobalUnitDefinitionForm()
  } catch (err) {
    error.value = err.message || '删除全局单位失败'
  } finally {
    globalUnitSaving.value = false
  }
}

function startProductUnitTemplateEdit(template) {
  productUnitTemplateForm.value = defaultProductUnitTemplateForm(JSON.parse(JSON.stringify(template || {})))
}

function resetProductUnitTemplateForm() {
  productUnitTemplateForm.value = defaultProductUnitTemplateForm()
}

function validateProductUnitTemplatePayload(payload) {
  if (!String(payload.name || '').trim()) return '请填写销售规格模板名称'
  if (Array.isArray(payload.sales_specs)) {
    if (!String(payload.inventory_unit || '').trim()) return '请选择库存单位'
    if (!payload.sales_specs.length) return '请至少添加一条销售规格'
    const badSpec = payload.sales_specs.find((row) => !String(row.spec_name || '').trim() || Number(row.net_content_qty || 0) <= 0)
    if (badSpec) return '请填写每条销售规格的规格名称和库存数量'
    const mismatchedUnit = payload.sales_specs.find((row) => String(row.net_content_unit || '').trim() !== String(payload.inventory_unit || '').trim())
    if (mismatchedUnit) return `销售规格「${mismatchedUnit.spec_name || '未命名'}」的换算单位不能自动转换为库存单位 ${payload.inventory_unit}，请调整库存单位或重新填写该规格`
    return ''
  }
  if (!String(payload.inventory_unit || '').trim()) return '请选择库存单位'
  if (!String(payload.default_sales_unit || payload.sales_unit || '').trim()) return '请选择默认销售单位'
  const salesUnits = Array.isArray(payload.sales_units) ? payload.sales_units : []
  if (!salesUnits.includes(payload.default_sales_unit || payload.sales_unit)) return '默认销售单位必须来自历史销售单位配置'
  const conversion = JSON.parse(payload.unit_conversion_json || '{}')
  if (!conversion[payload.default_sales_unit || payload.sales_unit]) return '默认销售单位必须配置历史换算'
  return ''
}

function productUnitTemplateSalesSpecRows(idOrTemplate) {
  const template = typeof idOrTemplate === 'object' ? idOrTemplate : findProductUnitTemplate(idOrTemplate)
  return salesSpecRowsFromTemplate(template || {})
}

function syncSalesSpecTemplateDefaults(target) {
  if (!target || !Array.isArray(target.sales_spec_rows)) return
  let defaultIndex = -1
  const defaultSpecKey = String(target.default_spec_key || '').trim()
  target.sales_spec_rows.forEach((row, index) => {
    if (defaultSpecKey && String(row.spec_key || '').trim() === defaultSpecKey) defaultIndex = index
    if (defaultIndex < 0 && row.default === true) defaultIndex = index
  })
  if (defaultIndex < 0) defaultIndex = target.sales_spec_rows.findIndex((row) => row.active !== false)
  if (defaultIndex < 0) defaultIndex = 0
  target.sales_spec_rows.forEach((row, index) => {
    row.default = index === defaultIndex
    row.active = true
    row.sales_unit = String(row.spec_name || '').trim()
    row.net_content_unit = target.inventory_unit || row.net_content_unit || 'kg'
  })
  const defaultRow = target.sales_spec_rows[defaultIndex]
  target.default_spec_key = defaultRow?.spec_key || ''
  target.default_sales_unit = defaultRow?.sales_unit || defaultRow?.spec_name || target.default_sales_unit || ''
  target.sales_unit = target.default_sales_unit
  target.quote_unit = target.default_sales_unit
  target.order_unit = target.default_sales_unit
}

function syncSalesSpecTemplateInventoryUnit(target) {
  if (!target || !Array.isArray(target.sales_spec_rows)) return
  const inventoryUnit = target.inventory_unit || 'kg'
  target.sales_spec_rows = salesSpecRowsFromTemplate({
    inventory_unit: inventoryUnit,
    sales_specs: target.sales_spec_rows,
  }, inventoryUnit)
  syncSalesSpecTemplateDefaults(target)
}

function addSalesSpecRow(target) {
  if (!target) return
  if (!Array.isArray(target.sales_spec_rows)) target.sales_spec_rows = []
  const nextIndex = target.sales_spec_rows.length + 1
  target.sales_spec_rows.push({
    spec_key: `spec-${Date.now()}-${nextIndex}`,
    spec_name: '',
    sales_unit: '',
    net_content_qty: 0,
    net_content_unit: target.inventory_unit || 'kg',
    default: target.sales_spec_rows.length === 0,
    active: true,
  })
  if (target.sales_spec_rows.length === 1) target.default_spec_key = target.sales_spec_rows[0].spec_key
  syncSalesSpecTemplateDefaults(target)
}

function removeSalesSpecRow(target, index) {
  if (!target || !Array.isArray(target.sales_spec_rows)) return
  const removedWasDefault = target.sales_spec_rows[index]?.default === true ||
    String(target.sales_spec_rows[index]?.spec_key || '').trim() === String(target.default_spec_key || '').trim()
  target.sales_spec_rows.splice(index, 1)
  if (!target.sales_spec_rows.length) {
    addSalesSpecRow(target)
    return
  }
  if (removedWasDefault || !target.sales_spec_rows.some((row) => String(row.spec_key || '').trim() === String(target.default_spec_key || '').trim())) {
    const fallback = target.sales_spec_rows.find((row) => row.active !== false) || target.sales_spec_rows[0]
    target.default_spec_key = fallback?.spec_key || ''
    target.sales_spec_rows.forEach((row) => { row.default = row === fallback })
  }
  syncSalesSpecTemplateDefaults(target)
}

function setSalesSpecDefault(target, index) {
  if (!target || !Array.isArray(target.sales_spec_rows)) return
  target.sales_spec_rows.forEach((row, rowIndex) => { row.default = rowIndex === index })
  target.default_spec_key = target.sales_spec_rows[index]?.spec_key || ''
  syncSalesSpecTemplateDefaults(target)
}

function salesSpecNetContentLabel(row = {}) {
  const qty = Number(row.net_content_qty || row.netContentQty || 0)
  const unit = productUnitName(row.net_content_unit || row.netContentUnit || '')
  if (!qty || !unit) return '未填写净含量'
  return `${qty}${unit}`
}

function derivedSpecStatusLabel(status = '') {
  return {
    active: '已派生',
    template_disabled: '模板已停用',
    template_removed: '模板已移除',
  }[String(status || '').trim()] || '已派生'
}

async function saveProductUnitTemplate() {
  const payload = buildProductUnitTemplatePayload(productUnitTemplateForm.value)
  if (productUnitTemplateInventoryUnitLocked.value) {
    payload.inventory_unit = productUnitTemplateForm.value.original_inventory_unit || productUnitTemplateForm.value.inventory_unit || payload.inventory_unit
  }
  const validation = validateProductUnitTemplatePayload(payload)
  if (validation) {
    error.value = validation
    return
  }
  productUnitSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/product-settings/unit-templates/${payload.id}` : '/api/product-settings/unit-templates'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '销售规格模板已保存，商品配置可直接引用'
    await loadAll()
    resetProductUnitTemplateForm()
  } catch (err) {
    error.value = err.message || '保存销售规格模板失败'
  } finally {
    productUnitSaving.value = false
  }
}

async function deleteProductUnitTemplate(template) {
  const templateID = Number(template?.id || 0)
  if (templateID <= 0) return
  if (typeof window !== 'undefined' && !window.confirm(`确认删除销售规格模板「${template?.name || templateID}」？已绑定的历史商品配置不会被物理删除。`)) return
  productUnitSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/unit-templates/${templateID}`, { method: 'DELETE' })
    ok.value = '销售规格模板已删除，新的商品配置不再引用该模板'
    await loadAll()
    if (Number(productUnitTemplateForm.value.id || 0) === templateID) resetProductUnitTemplateForm()
  } catch (err) {
    error.value = err.message || '删除销售规格模板失败'
  } finally {
    productUnitSaving.value = false
  }
}

function canDeriveProductConfigTemplate(template) {
  return skuContextCustomerID.value > 0
    && Number(template?.customer_id || 0) === 0
    && template?.active !== false
}

async function deriveProductConfigTemplateForCustomer(template) {
  const customerID = skuContextCustomerID.value
  if (!customerID) {
    error.value = '请选择履约客户'
    return
  }
  if (!canDeriveProductConfigTemplate(template)) return
  productConfigSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const response = await apiSend('/api/product-settings/product-config-templates/derive', {
      body: {
        customer_id: customerID,
        source_template_id: Number(template.id || 0),
        name: `${customerName(customerID) || '客户'} - ${template.name}`,
      },
    })
    const derivedID = Number(response?.template?.id || 0)
    ok.value = '公共商品配置已复制为客户配置'
    await loadAll()
    selectedCustomerSkuCustomerID.value = customerID
    activeSettingsSection.value = 'templates'
    activeConfigTemplateSection.value = 'product-config'
    const derived = productConfigTemplates.value.find((row) => Number(row.id || 0) === derivedID)
      || productConfigTemplates.value.find((row) => Number(row.customer_id || 0) === customerID && Number(row.source_template_id || 0) === Number(template.id || 0))
    if (derived) startProductConfigTemplateEdit(derived)
  } catch (err) {
    error.value = err.message || '复制公共商品配置失败'
  } finally {
    productConfigSaving.value = false
  }
}

function resetCustomerProductRuleForms() {
  customerRuleTemplateForm.value = defaultCustomerProductRuleTemplateForm()
  customerRuleOverrideForm.value = defaultCustomerProductRuleOverrideForm()
}

function resetCustomerProductRuleTemplateForm() {
  customerRuleTemplateForm.value = defaultCustomerProductRuleTemplateForm()
}

function startCustomerProductRuleTemplateEdit(template) {
  customerRuleTemplateForm.value = {
    ...defaultCustomerProductRuleTemplateForm(),
    ...JSON.parse(JSON.stringify(template || {})),
    customer_id: Number(template?.customer_id || skuContextCustomerID.value || 0),
    items: (template?.items || []).map(decorateCustomerProductRuleItem),
  }
}

function addCustomerProductRuleTemplateItem() {
  customerRuleTemplateForm.value.items.push(defaultCustomerProductRuleTemplateItem())
}

function removeCustomerProductRuleTemplateItem(index) {
  if (customerRuleTemplateForm.value.items.length <= 1) {
    customerRuleTemplateForm.value.items = [defaultCustomerProductRuleTemplateItem()]
    return
  }
  customerRuleTemplateForm.value.items.splice(index, 1)
}

function addUnitConversionRow(target) {
  if (!target) return
  if (!Array.isArray(target.unit_conversion_rows)) target.unit_conversion_rows = []
  const inventoryUnit = target.inventory_unit || 'kg'
  const existingUnits = new Set(target.unit_conversion_rows.map((row) => String(row?.from_unit || '').trim()).filter(Boolean))
  const nextUnit = activeProductUnitDefinitions.value.find((unit) => unit.code && !existingUnits.has(unit.code))?.code || inventoryUnit
  target.unit_conversion_rows.push({
    from_qty: 1,
    from_unit: nextUnit,
    to_qty: 1,
    to_unit: inventoryUnit,
    integer_sales_unit: false,
  })
  syncProductUnitTemplateInventoryUnit(target)
}

function removeUnitConversionRow(target, index) {
  if (!target || !Array.isArray(target.unit_conversion_rows)) return
  if (isInventoryUnitConversionRow(target.unit_conversion_rows[index], target)) return
  target.unit_conversion_rows.splice(index, 1)
  const options = productUnitTemplateSalesUnitOptions(target)
  if (!options.includes(target.default_sales_unit)) {
    target.default_sales_unit = options[0] || target.inventory_unit || 'kg'
  }
}

function ensureProductUnitTemplateInventoryConversionRow(rows = [], inventoryUnit = 'kg') {
  const normalizedInventoryUnit = String(inventoryUnit || 'kg').trim() || 'kg'
  const nextRows = (Array.isArray(rows) ? rows : [])
    .map((row) => ({
      from_qty: Number(row?.from_qty || 1) || 1,
      from_unit: String(row?.from_unit || '').trim(),
      to_qty: Number(row?.to_qty || 0) || 0,
      to_unit: String(row?.to_unit || normalizedInventoryUnit).trim() || normalizedInventoryUnit,
      integer_sales_unit: Boolean(row?.integer_sales_unit || row?.integer_unit),
    }))
    .filter((row) => row.from_unit)
  const hasInventoryRow = nextRows.some((row) => row.from_unit === normalizedInventoryUnit)
  if (!hasInventoryRow) {
    nextRows.unshift({ from_qty: 1, from_unit: normalizedInventoryUnit, to_qty: 1, to_unit: normalizedInventoryUnit, integer_sales_unit: false })
  }
  return nextRows.map((row) => row.from_unit === normalizedInventoryUnit
    ? { ...row, from_qty: 1, to_qty: 1, to_unit: normalizedInventoryUnit }
    : { ...row, from_qty: row.from_qty || 1, to_unit: normalizedInventoryUnit })
}

function syncProductUnitTemplateInventoryUnit(target) {
  if (!target) return
  const inventoryUnit = String(target.inventory_unit || 'kg').trim() || 'kg'
  target.inventory_unit = inventoryUnit
  target.unit_conversion_rows = ensureProductUnitTemplateInventoryConversionRow(target.unit_conversion_rows, inventoryUnit)
  const options = productUnitTemplateSalesUnitOptions(target)
  if (!options.includes(target.default_sales_unit)) {
    target.default_sales_unit = options.includes(inventoryUnit) ? inventoryUnit : options[0] || inventoryUnit
  }
  target.sales_unit = target.default_sales_unit
  target.quote_unit = target.default_sales_unit
  target.order_unit = target.default_sales_unit
}

function isInventoryUnitConversionRow(row, form) {
  const inventoryUnit = String(form?.inventory_unit || 'kg').trim() || 'kg'
  return String(row?.from_unit || '').trim() === inventoryUnit
}

function productUnitTemplateSalesUnitOptions(form = {}) {
  const units = []
  const push = (value) => {
    const unit = String(value || '').trim()
    if (unit && !units.includes(unit)) units.push(unit)
  }
  push(form.inventory_unit || 'kg')
  push(form.default_sales_unit || form.sales_unit || form.order_unit || form.quote_unit)
  if (Array.isArray(form.sales_units)) form.sales_units.forEach(push)
  if (Array.isArray(form.unit_conversion_rows)) {
    form.unit_conversion_rows.forEach((row) => push(row?.from_unit))
  }
  return units
}

function startCustomerProductRuleOverrideEdit(row) {
  customerRuleOverrideForm.value = decorateCustomerProductRuleOverride({
    ...JSON.parse(JSON.stringify(row || {})),
    customer_id: Number(row?.customer_id || skuContextCustomerID.value || 0),
  })
}

function validateCustomerProductRulePayload(payload, requireName = false) {
  // DEV-352/DEV-354 compatibility marker: 产品类型 / 产品子类型.
  if (Number(payload.customer_id || 0) <= 0) return '请选择履约客户'
  if (requireName && !String(payload.name || '').trim()) return '请填写规则模板名称'
  const items = requireName ? payload.items || [] : [payload]
  if (!items.length) return '至少维护一个产品子类型规则'
  if (items.some((item) => Number(item.product_subtype_category_id || 0) <= 0)) return '请选择产品子类型'
  return ''
}

async function saveCustomerProductRuleTemplate() {
  const customerID = skuContextCustomerID.value
  const payload = buildCustomerProductRuleTemplatePayload({
    ...customerRuleTemplateForm.value,
    customer_id: customerID,
  })
  const validation = validateCustomerProductRulePayload(payload, true)
  if (validation) {
    error.value = validation
    return
  }
  customerRuleSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/product-settings/customer-rule-templates/${payload.id}` : '/api/product-settings/customer-rule-templates'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '商品配置兼容模板已保存'
    resetCustomerProductRuleTemplateForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存商品配置兼容模板失败'
  } finally {
    customerRuleSaving.value = false
  }
}

async function saveCustomerProductRuleOverride() {
  const customerID = skuContextCustomerID.value
  const payload = buildCustomerProductRuleOverridePayload({
    ...customerRuleOverrideForm.value,
    customer_id: customerID,
  })
  const validation = validateCustomerProductRulePayload(payload, false)
  if (validation) {
    error.value = validation
    return
  }
  customerRuleSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/product-settings/customer-rule-overrides/${payload.id}` : '/api/product-settings/customer-rule-overrides'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '商品配置兼容覆盖已保存'
    customerRuleOverrideForm.value = defaultCustomerProductRuleOverrideForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存商品配置兼容覆盖失败'
  } finally {
    customerRuleSaving.value = false
  }
}

async function bindCustomerProductRuleTemplate(templateID) {
  const customerID = skuContextCustomerID.value
  if (!customerID) {
    error.value = '请选择履约客户'
    return
  }
  customerRuleSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/customers/${customerID}/rule-template`, {
      body: buildCustomerProductRuleBindingPayload(customerID, templateID),
    })
    ok.value = '商品配置兼容绑定已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '绑定商品配置兼容模板失败'
  } finally {
    customerRuleSaving.value = false
  }
}

function productSubtypeName(id) {
  const categoryID = Number(id || 0)
  return productSubtypeRuleOptions.value.find((category) => Number(category.id || 0) === categoryID)?.name || `子类型 #${categoryID || '-'}`
}

function categoryRuleLabel(category) {
  const parent = flatPublicCategories.value.find((row) => Number(row.id || 0) === Number(category.parent_id || 0))
    || flatCustomerCategories.value.find((row) => Number(row.id || 0) === Number(category.parent_id || 0))
  return parent?.name ? `${parent.name} / ${category.name}` : category.name || ''
}

function productVisibility(product) {
  const customerID = Number(product?.customer_id || 0)
  return product?.visibility || (customerID > 0 ? 'customer_only' : 'public')
}

function flattenCategoryNodes(nodes = []) {
  const out = []
  for (const node of nodes || []) {
    out.push(node)
    out.push(...flattenCategoryNodes(node.children || []))
  }
  return out
}

function normalizedSearch(value) {
  return String(value || '').trim().toLowerCase()
}

function categorySearchText(category = {}) {
  return [
    category.name,
    category.number,
    category.template_state,
    ...(category.products || []).flatMap((product) => [product.name, product.number, product.remark]),
  ].map((value) => String(value || '').toLowerCase()).join(' ')
}

function filterCategoryTreeByQuery(tree = [], query = '') {
  const q = normalizedSearch(query)
  if (!q) return tree
  return (tree || [])
    .map((primary) => {
      const primaryMatches = categorySearchText(primary).includes(q)
      const children = (primary.children || []).filter((secondary) => (
        primaryMatches || categorySearchText(secondary).includes(q)
      ))
      if (!primaryMatches && !children.length) return null
      return { ...primary, children }
    })
    .filter(Boolean)
}

function categoryBelongsToCurrentSkuContext(category) {
  return categoryBelongsToContext(category, {
    customerID: skuContextCustomerID.value,
    usePublicCategories: customerUsesPublicCategories.value,
    publicCategories: flatPublicCategories.value,
    customerCategories: flatCustomerCategories.value,
    publicProducts: publicProducts.value,
  })
}

function skuContextProductFilter(product) {
  return productBelongsToContext(product, {
    customerID: skuContextCustomerID.value,
    usePublicSku: false,
    publicProducts: publicProducts.value,
    customerProducts: customerProductsForContext.value,
  })
}

function canEditCategory(category) {
  if (category?.business_group_item) return true
  return !isPublicReferenceRow(category, { customerID: skuContextCustomerID.value })
}

function canEditSkuRow(row) {
  return !isPublicReferenceRow(row, { customerID: skuContextCustomerID.value })
}

function openProductDrawer() {
  ensureProductTypeCategorySelected(skuForm.value)
  productDrawerOpen.value = true
  productsCollapsed.value = false
}

function closeProductDrawer() {
  productDrawerOpen.value = false
}

function canDragSkuRow(row) {
  return skuContextProductFilter(row)
}

function customerName(id) {
  return customers.value.find((customer) => Number(customer.id) === Number(id))?.name || ''
}

function productName(id) {
  return products.value.find((product) => Number(product.id) === Number(id))?.name || ''
}

function productPriceGroupName(id) {
  return productPriceGroups.value.find((group) => Number(group.id) === Number(id))?.name || ''
}

function priceRecordTargetLabel(record = {}) {
  const aliasID = Number(record.customer_product_alias_id || 0)
  if (aliasID > 0) {
    const alias = customerProductAliases.value.find((row) => Number(row.id || 0) === aliasID)
    return alias ? customerAliasEffectiveDisplayName(alias) : `客户商品 #${aliasID}`
  }
  const productID = Number(record.product_id || 0)
  return productID > 0 ? productName(productID) || `商品 #${productID}` : '-'
}

function productPriceRecordStatusLabel(record = {}) {
  if (record.active === false || record.status === 'inactive') return '停用'
  if (record.status === 'published') return '已发布'
  return '草稿'
}

function categoryName(id) {
  return flattenCategoryNodes(categories.value).find((category) => Number(category.id) === Number(id))?.name || ''
}

function customerOptionLabel(customer) {
  return customer?.name || ''
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

function notifyKferp(type, title, message = '') {
  window.dispatchEvent(new CustomEvent('kferp:notify', {
    detail: {
      type,
      title,
      message,
    },
  }))
}

function classificationTemplateOptionLabel(template) {
  return template?.label || template?.name || ''
}

function classificationTemplateOptionMeta(template) {
  const count = Array.isArray(template?.categories) ? template.categories.length : 0
  return count ? `${count} 个分类项` : '暂无分类项'
}

function classificationMoveOptionLabel(option) {
  return option?.label || option?.name || ''
}

function classificationMoveOptionMeta(option) {
  if (option?.move_type === 'template') return '移动到大类，子类为未分类'
  if (currentProductClassificationTemplate.value && productMoveClassificationOptions.value.includes(option)) return currentProductClassificationTemplate.value.name || ''
  if (currentAliasClassificationTemplate.value && aliasMoveClassificationOptions.value.includes(option)) return currentAliasClassificationTemplate.value.name || ''
  return ''
}

function classificationMoveCategoryID(option) {
  if (!option || option.move_type === 'template') return 0
  if (Object.prototype.hasOwnProperty.call(option, 'category_id')) return Number(option.category_id || 0)
  return Number(option.id || 0)
}

function baseProductOptionLabel(product) {
  return product?.name || ''
}

function baseProductOptionMeta(product) {
  const parts = []
  if (product?.number) parts.push(`编号 ${product.number}`)
  for (const value of Object.values(product?.special_attr_values || {})) {
    if (value) parts.push(value)
  }
  return parts.join(' / ')
}

function productOptionLabel(product) {
  return product?.name || ''
}

function productOptionMeta(product) {
  const parts = []
  parts.push(`SKU-${String(Number(product?.id || 0)).padStart(6, '0')}`)
  const owner = ownerLabel(product)
  if (owner) parts.push(owner)
  return parts.join(' / ')
}

function ownerLabel(row) {
  if (Number(row.customer_id || 0) <= 0) return '公共'
  return customerName(row.customer_id) || `客户 #${row.customer_id}`
}

function categoryStateLabel(category) {
  return categoryDisplayState(category, { customerID: skuContextCustomerID.value }).label
}

function productOwnerLabel(product) {
  if (!skuContextCustomerID.value) return ownerLabel(product)
  return productDisplayState(product, { customerID: skuContextCustomerID.value }).label
}

function gradientTemplateLabel(template) {
  if (Number(template.customer_id || 0) <= 0) return '公共模板'
  if (Number(template.source_template_id || 0) > 0 || template.template_state === 'derived_from_public') return '来自公共模板'
  return ownerLabel(template)
}

function productConfigTemplateLabel(template) {
  if (Number(template.customer_id || 0) <= 0) return '公共配置'
  if (Number(template.source_template_id || 0) > 0 || template.template_state === 'derived_from_public') return '来自公共配置'
  return ownerLabel(template)
}

function productUnitName(code) {
  const normalized = String(code || '').trim()
  if (!normalized) return '-'
  return visibleProductUnitDefinitions.value.find((unit) => unit.code === normalized)?.name || normalized
}

function productInventoryUnitLabel(row = {}) {
  return productUnitName(row.inventory_unit || row.stock_unit || 'kg')
}

function productIntegerInventoryLabel(row = {}) {
  return row.integer_inventory_unit || row.integer_unit || row.stock_integer_unit ? '只允许整数' : '允许小数'
}

function normalizePriceSummary(summary = {}) {
  const source = summary && typeof summary === 'object' ? summary : {}
  const price = source.final_price ?? source.unit_price ?? source.price
  const unit = source.price_unit || source.unit || source.uom || ''
  const version = source.price_table_version || source.version || source.version_no || ''
  const tier = source.tier_label || source.tier || source.quantity_tier || ''
  const updatedAt = source.updated_at || source.published_at || ''
  return {
    price,
    unit,
    version,
    tier,
    updatedAt,
  }
}

function priceSummaryLabel(summary = {}) {
  const normalized = normalizePriceSummary(summary)
  const priceNumber = Number(normalized.price)
  if (!Number.isFinite(priceNumber) || priceNumber <= 0) return '暂无价格表价格'
  const unit = normalized.unit ? `/${productUnitName(normalized.unit)}` : ''
  const version = normalized.version ? ` · ${normalized.version}` : ''
  const tier = normalized.tier ? ` · ${normalized.tier}` : ''
  const updated = normalized.updatedAt ? ` · ${String(normalized.updatedAt).slice(0, 16)}` : ''
  return `¥${priceNumber.toFixed(2)}${unit}${tier}${version}${updated}`
}

function productPriceSummaryLabel(row = {}) {
  return priceSummaryLabel(row.price_summary || row.current_price_summary || row.latest_price_snapshot || {})
}

function aliasPriceSummaryLabel(row = {}) {
  return priceSummaryLabel(row.price_summary || row.current_price_summary || row.latest_price_snapshot || {})
}

function findProductUnitTemplate(id) {
  const templateID = Number(id || 0)
  if (!templateID) return null
  return visibleProductUnitTemplates.value.find((template) => Number(template.id || 0) === templateID) || null
}

function hasProductUnitRuleOverride(form = {}) {
  try {
    const raw = form.unit_rule_override_json || form.unitRuleOverrideJSON || '{}'
    const rule = typeof raw === 'object' && !Array.isArray(raw) ? raw : JSON.parse(String(raw || '{}'))
    if (!rule || typeof rule !== 'object' || Array.isArray(rule)) return false
    return [
      'inventory_unit',
      'integer_inventory_unit',
      'integer_unit',
      'default_sales_unit',
      'quote_unit',
      'order_unit',
      'unit_conversion_json',
      'conversion_json',
      'sales_unit_rules',
    ].some((key) => Object.prototype.hasOwnProperty.call(rule, key))
  } catch (_) {
    return false
  }
}

function defaultProductUnitTemplateID() {
  const row = (productUnitTemplates.value || []).find((template) => template && template.active !== false && !template.deleted_at && !template.deleted)
  return Number(row?.id || 0)
}

function findGradientTemplate(id) {
  const templateID = Number(id || 0)
  if (!templateID) return null
  return gradientTemplates.value.find((template) => Number(template.id || 0) === templateID) || null
}

function productConfigTemplateForAlias(alias = {}) {
  const override = productConfigTemplateByID(alias.product_config_template_id)
  if (override) return { template: override, inherited: false }
  const product = products.value.find((row) => Number(row.id || 0) === Number(alias.product_id || 0)) || null
  const inherited = productConfigTemplateByID(product?.product_config_template_id)
  return { template: inherited, inherited: true }
}

function aliasPricingTemplateLabel(alias = {}) {
  const { template, inherited } = productConfigTemplateForAlias(alias)
  if (template) return `商品配置：${template.name}${inherited ? '（继承商品档案）' : ''}`
  return '商品配置：继承商品档案配置'
}

function aliasUnitTemplateLabel(alias = {}) {
  const { template } = productConfigTemplateForAlias(alias)
  if (!template) return '计价/单位：继承商品档案配置'
  const pricingMode = priceListRuleFormFromJSON(template.price_list_rule_json || '{}').price_rule_pricing_mode
  const pricingLabel = priceListRulePricingModeOptions.find((option) => option.value === pricingMode)?.label || '计价方式'
  return `${pricingLabel} · ${productConfigUnitTemplateName(template.unit_template_id)}`
}

function productionBomOptionMeta(row = {}) {
  const parts = []
  if (row.group_name) parts.push(row.group_name)
  if (row.latest_version_status) parts.push(row.latest_version_status === 'published' ? '已发布' : row.latest_version_status)
  return parts.join(' / ')
}

function productUnitTemplateSummary(idOrTemplate) {
  const template = typeof idOrTemplate === 'object' ? idOrTemplate : findProductUnitTemplate(idOrTemplate)
  if (!template) return '未绑定销售规格模板'
  const specs = salesSpecRowsFromTemplate(template)
  const defaultSpec = specs.find((row) => row.default) || specs[0]
  const activeCount = specs.filter((row) => row.active !== false).length
  const defaultLabel = defaultSpec ? (defaultSpec.spec_name || '默认规格') : '未配置规格'
  return `${template.name || '销售规格模板'} · 默认 ${defaultLabel} · 规格 ${activeCount}`
}

function productUnitTemplateConversionRows(template) {
  if (!template) return []
  const inventoryUnit = template.inventory_unit || 'kg'
  const salesUnit = template.default_sales_unit || template.order_unit || template.quote_unit || template.sales_unit || inventoryUnit
  const rows = ensureProductUnitTemplateInventoryConversionRow(unitConversionRowsFromJSON(template.unit_conversion_json || '{}', inventoryUnit), inventoryUnit)
  if (rows.length) {
    return rows.map((row) => ({
      ...row,
      integer_sales_unit: Boolean(template.integer_unit),
    }))
  }
  return [{ from_qty: 1, from_unit: salesUnit, to_qty: 1, to_unit: inventoryUnit, integer_sales_unit: Boolean(template.integer_unit) }]
}

function applyProductUnitTemplateToForm(form) {
  if (!form) return
  const template = findProductUnitTemplate(form.unit_template_id)
  if (!template) return
  const specs = salesSpecRowsFromTemplate(template)
  const defaultSpec = specs.find((row) => row.default) || specs[0]
  form.default_sales_unit = defaultSpec?.sales_unit || template.default_sales_unit || template.order_unit || template.quote_unit || template.sales_unit || form.inventory_unit || 'kg'
  form.unit_conversion_rows = []
}

function applySkuUnitTemplateDefaults(form) {
  if (!form) return
  if (!Number(form.unit_template_id || 0)) {
    form.unit_rule_override_enabled = false
    return
  }
  form.unit_rule_override_enabled = false
  applyProductUnitTemplateToForm(form)
}

function clearProductConfigUnitOverride() {
  productProductionConfigForm.value.unit_rule_override_enabled = false
  applyProductUnitTemplateToForm(productProductionConfigForm.value)
}

function productConfigUnitTemplateName(idOrTemplate) {
  const template = typeof idOrTemplate === 'object' ? idOrTemplate : findProductUnitTemplate(idOrTemplate)
  return template?.name || '未绑定销售规格模板'
}

function productConfigUnitChips(idOrTemplate) {
  const template = typeof idOrTemplate === 'object' ? idOrTemplate : findProductUnitTemplate(idOrTemplate)
  if (!template) return ['单位未绑定']
  const salesUnit = template.default_sales_unit || template.order_unit || template.quote_unit || template.sales_unit || template.inventory_unit
  const chips = [
    `库存 ${productUnitName(template.inventory_unit)}`,
    `默认销售 ${productUnitName(salesUnit)}`,
  ]
  if (template.integer_unit) chips.push('整数单位')
  return chips
}

function productConfigSummary(templateID) {
  const config = productConfigTemplates.value.find((row) => Number(row.id || 0) === Number(templateID || 0))
  if (!config) return '未绑定商品配置；会继续保留子类型当前默认规则。'
  return `阶梯价模板 ${config.gradient_template_id || '未绑定'} · 工序 ${config.operation_template_id || '未绑定'} · ${productUnitTemplateSummary(config.unit_template_id)}`
}

function bomStatusLabel(value) {
  if (value === 'inactive') return '已失效'
  if (value === 'missing') return '未维护'
  return '有效'
}

function skuStatusLabel(row) {
  if (row.active === false) return '已失效'
  return '启用'
}

function pruneSelectedProducts(sourceProducts) {
  const validIDs = new Set((sourceProducts || []).map((product) => Number(product.id || 0)).filter(Boolean))
  selectedProductIds.value = selectedProductIds.value.filter((id) => validIDs.has(Number(id)))
}

function syncSelectedCustomerSkuCustomer() {
  if (!selectedCustomerSkuCustomerID.value) return
  if (!customers.value.some((customer) => Number(customer.id || 0) === Number(selectedCustomerSkuCustomerID.value))) {
    selectedCustomerSkuCustomerID.value = 0
  }
}

function syncSelectedAliasCustomer() {
  const current = Number(selectedAliasCustomerID.value || 0)
  if (current > 0 && customers.value.some((customer) => Number(customer.id || 0) === current)) return
  const contextCustomerID = Number(selectedCustomerSkuCustomerID.value || props.customerContextId || 0)
  if (contextCustomerID > 0 && customers.value.some((customer) => Number(customer.id || 0) === contextCustomerID)) {
    selectedAliasCustomerID.value = contextCustomerID
    return
  }
  selectedAliasCustomerID.value = Number(customers.value[0]?.id || 0)
}

function applyWorkspaceCustomerContext() {
  const nextCustomerID = nextSkuContextCustomerID(selectedCustomerSkuCustomerID.value, {
    workspaceMode: props.workspaceMode,
    customerContextID: props.customerContextId,
  })
  if (Number(selectedCustomerSkuCustomerID.value || 0) !== nextCustomerID) {
    selectedCustomerSkuCustomerID.value = nextCustomerID
  }
  if (props.workspaceMode === CUSTOMER_WORKSPACE_MODE && nextCustomerID > 0) {
    selectedAliasCustomerID.value = nextCustomerID
    if (activeSettingsSection.value === 'master') {
      activeSettingsSection.value = 'aliases'
    }
  }
}

function notifyWorkspaceCustomerChanged(customerID) {
  if (props.workspaceMode !== CUSTOMER_WORKSPACE_MODE || Number(customerID || 0) <= 0) return
  if (Number(customerID || 0) === Number(props.customerContextId || 0)) return
  window.dispatchEvent(workspaceCustomerChangeEvent(customerID))
}

function isProductSelected(row) {
  return selectedProductIds.value.includes(Number(row.id))
}

function toggleProductSelection(row, checked) {
  if (!canEditSkuRow(row)) return
  const id = Number(row.id || 0)
  if (!id) return
  const current = selectedProductIds.value
  selectedProductIds.value = checked
    ? Array.from(new Set([...current, id]))
    : current.filter((item) => item !== id)
}

function editableProductGroupRows(group = {}) {
  return (Array.isArray(group?.rows) ? group.rows : []).filter(canEditSkuRow)
}

function areProductGroupRowsSelected(group = {}) {
  const rows = editableProductGroupRows(group)
  return rows.length > 0 && rows.every((row) => selectedProductIds.value.includes(Number(row.id || 0)))
}

function toggleProductGroupRows(group, checked) {
  selectedProductIds.value = selectedSkuRowIDsAfterVisibleToggle(
    selectedProductIds.value,
    editableProductGroupRows(group),
    checked,
  )
}

function handleProductGroupPaginationChange(groupKey, { page, pageSize }) {
  const key = String(groupKey || '')
  if (!key) return
  skuGroupPagination.value = {
    ...skuGroupPagination.value,
    [key]: {
      page: Number(page || 1),
      pageSize: normalizePageSize(pageSize || DEFAULT_SKU_GROUP_PAGE_SIZE),
    },
  }
}

function selectedBaseProduct() {
  return products.value.find((product) => Number(product.id) === Number(customForm.value.base_product_id)) || null
}

function productTypeCategoryByID(categoryID) {
  return productTypeCategoryOptions.value.find((category) => Number(category.id) === Number(categoryID)) || null
}

function productSubtypeCategoryByID(categoryID) {
  const id = Number(categoryID || 0)
  if (!id) return null
  return productSubtypeRuleOptions.value.find((category) => Number(category.id || 0) === id) || null
}

function productConfigTemplateByID(templateID) {
  const id = Number(templateID || 0)
  if (!id) return null
  return productConfigTemplates.value.find((template) => Number(template.id || 0) === id) || null
}

function flattenBusinessGroupItemsForView(items = [], parent = null, out = []) {
  for (const item of Array.isArray(items) ? items : []) {
    const row = {
      ...item,
      parent_id: Number(item.parent_id || parent?.id || 0),
      parent_name: parent?.name || '',
    }
    out.push(row)
    flattenBusinessGroupItemsForView(item.children || [], row, out)
  }
  return out
}

function productCatalogBusinessGroupRows() {
  return businessGroupRowsForFeatureSelection(businessGroups.value, productGroupFeatureSelectionIDs.value)
}

function selectedProductCatalogBusinessGroup() {
  return selectedProductGroupTemplate.value || productCatalogBusinessGroupRows()[0] || null
}

function businessGroupDisplayName(group = {}) {
  return businessGroupVisibleName(group) || String(group.name || '').trim() || `分组模板 #${Number(group.id || 0)}`
}

function productCatalogProductAssignmentsForGroup(groupID) {
  const normalizedGroupID = Number(groupID || 0)
  return (businessGroupAssignments.value || []).filter((row) => (
    Number(row.group_id || 0) === normalizedGroupID
    && Number(row.group_item_id || 0) > 0
    && String(row.usage_key || '') === 'product_catalog'
    && String(row.object_key || '') === 'product'
  ))
}

function productGroupTemplateHasItems(templateID) {
  const normalizedID = Number(templateID || 0)
  if (!normalizedID) return false
  return (businessGroupAssignments.value || []).some((row) => (
    Number(row.group_id || 0) === normalizedID
    && Number(row.group_item_id || 0) > 0
    && String(row.usage_key || '') === 'product_catalog'
    && String(row.object_key || '') === 'product'
  ))
}

const deletedProductGroupTemplateWarnings = computed(() => {
  const availableIDs = new Set(selectableProductGroupTemplates.value.map((t) => Number(t.id || 0)))
  return productGroupFeatureSelectionIDs.value
    .filter((id) => !availableIDs.has(Number(id)))
    .map((id) => ({ id, name: `分组模板 #${Number(id)}` }))
})

function openProductGroupTemplateDrawer() {
  productGroupFeatureSelectionDraft.value = [...productGroupFeatureSelectionIDs.value]
  productGroupTemplateDrawerOpen.value = true
}

function closeProductGroupTemplateDrawer() {
  productGroupTemplateDrawerOpen.value = false
}

function toggleProductGroupTemplate(templateID) {
  const id = Number(templateID || 0)
  if (!id) return
  if (productGroupTemplateHasItems(id)) return
  const draft = [...productGroupFeatureSelectionDraft.value]
  const idx = draft.indexOf(id)
  if (idx >= 0) {
    draft.splice(idx, 1)
  } else {
    draft.push(id)
  }
  productGroupFeatureSelectionDraft.value = draft
}

async function saveAndCloseProductGroupTemplateDrawer() {
  await saveProductGroupFeatureSelection()
  if (!error.value) {
    productGroupTemplateDrawerOpen.value = false
  }
}


function buildProductCatalogBusinessGroupTree() {
  const group = selectedProductCatalogBusinessGroup()
  const groupID = Number(group?.id || 0)
  if (!groupID) return []
  const productsByID = new Map(products.value.map((product) => [Number(product.id || 0), product]))
  const productsByItemID = new Map()
  for (const assignment of productCatalogProductAssignmentsForGroup(groupID)) {
    const product = productsByID.get(Number(assignment.object_id || 0))
    if (!product || !skuContextProductFilter(product)) continue
    const itemID = Number(assignment.group_item_id || 0)
    if (!productsByItemID.has(itemID)) productsByItemID.set(itemID, [])
    productsByItemID.get(itemID).push({
      ...product,
      product_category_position: Number(assignment.sort_order || 100),
    })
  }
  for (const rows of productsByItemID.values()) {
    rows.sort((a, b) => Number(a.product_category_position || 0) - Number(b.product_category_position || 0) || String(a.name || '').localeCompare(String(b.name || ''), 'zh-Hans-CN'))
  }
  const project = (item = {}, parent = null, index = 0) => {
    const itemID = Number(item.id || 0)
    const parentID = Number(item.parent_id || parent?.id || 0)
    const primaryName = parent ? parent.name || '' : item.name || ''
    const secondaryName = parent ? item.name || '' : ''
    const itemProducts = (productsByItemID.get(itemID) || []).map((product, productIndex) => ({
      ...product,
      number: productIndex + 1,
      primary_name: primaryName,
      secondary_name: secondaryName,
    }))
    return {
      ...item,
      id: itemID,
      group_id: groupID,
      parent_id: parentID,
      customer_id: 0,
      position: Number(item.sort_order || index + 1),
      number: index + 1,
      business_group_item: true,
      products: itemProducts,
      children: (item.children || []).map((child, childIndex) => project(child, { ...item, id: itemID }, childIndex)),
    }
  }
  return businessGroupItemsTree(group.items || []).map((item, index) => project(item, null, index))
}

function aliasClassificationLabel(row) {
  return classificationAssignmentLabel(row, productClassificationTemplates.value, { assignmentType: 'alias' })
}

function classificationWarningsForProduct(row) {
  const found = classificationAssignmentForRow(row, productClassificationTemplates.value, { assignmentType: 'product' })
  if (!found) return []
  return classificationTemplateUnitPriceWarnings({
    productConfigTemplate: productConfigTemplateByID(row.product_config_template_id),
    classificationTemplate: found.template,
    classificationCategory: found.category,
  })
}

function classificationWarningsForAlias(row) {
  const found = classificationAssignmentForRow(row, productClassificationTemplates.value, { assignmentType: 'alias' })
  if (!found) return []
  const product = products.value.find((item) => Number(item.id || 0) === Number(row.product_id || 0)) || {}
  const aliasConfig = productConfigTemplateByID(row.product_config_template_id)
  return classificationTemplateUnitPriceWarnings({
    productConfigTemplate: aliasConfig || productConfigTemplateByID(product.product_config_template_id),
    classificationTemplate: found.template,
    classificationCategory: found.category,
  })
}

function productCategoryByID(categoryID) {
  const id = Number(categoryID || 0)
  if (!id) return null
  return flattenCategoryNodes(categoryTreeForSkuContext.value).find((category) => Number(category.id || 0) === id)
    || flattenCategoryNodes(categories.value).find((category) => Number(category.id || 0) === id)
    || null
}

function handleProductTypeCategoryChange(form) {
  if (!form) return
  form.product_subtype_category_id = 0
  form.special_attr_values = {}
  syncProductKindFromProductTypeCategory(form)
}

function syncProductKindFromProductTypeCategory(form) {
  if (!form) return
  const category = productTypeCategoryByID(form.product_type_category_id)
  form.product_kind = inferProductKindFromProductTypeCategory(category)
}

function syncProductTypeFromProductSubtype(form) {
  if (!form) return
  const subtype = productSubtypeCategoryByID(form.product_subtype_category_id)
  if (subtype?.parent_id) {
    form.product_type_category_id = Number(subtype.parent_id || 0)
  }
  syncProductKindFromProductTypeCategory(form)
}

function handleCustomProductTypeCategoryChange() {
  const previousKind = customForm.value.product_kind
  handleProductTypeCategoryChange(customForm.value)
  if (customForm.value.product_kind !== previousKind) {
    handleCustomProductKindChange()
  }
}

function ensureProductTypeCategorySelected(form) {
  if (!form) return
  if (Number(form.product_type_category_id || 0) && !productTypeCategoryByID(form.product_type_category_id)) {
    form.product_type_category_id = 0
  }
  if (Number(form.product_subtype_category_id || 0) && !productSubtypeCategoryByID(form.product_subtype_category_id)) {
    form.product_subtype_category_id = 0
  }
  syncProductKindFromProductTypeCategory(form)
}

async function assignCreatedSkuToSelectedProductSubtype(product, form) {
  const productID = Number(product?.id || 0)
  const categoryID = Number(form?.product_subtype_category_id || 0)
  if (!productID || !categoryID) return false
  const category = productSubtypeCategoryByID(categoryID) || { id: categoryID, customer_id: 0 }
  await apiSend(`/api/product-settings/products/${productID}/category`, {
    body: buildAssignCategoryPayload({
      product,
      category,
      customerID: Number(product?.customer_id || skuContextCustomerID.value || 0),
      position: 0,
    }),
  })
  return true
}

function baseProductName(id) {
  return products.value.find((product) => Number(product.id) === Number(id))?.name || '-'
}

function fillCustomProductName() {
  if (customForm.value.name) return
  const customer = customerName(selectedCustomerSkuCustomerID.value || customForm.value.customer_id)
  const base = selectedBaseProduct()
  if (!customer || !base) return
  const attrSuffix = Object.values(customForm.value.special_attr_values || {}).find(Boolean)
  customForm.value.name = attrSuffix ? `${customer}-${base.name}-${attrSuffix}` : `${customer}-${base.name}`
}

function syncCustomFormFromBaseProduct(product) {
  if (!product) return
  const kind = normalizedProductKind(product)
  customForm.value.product_kind = kind
  customForm.value.special_attr_values = { ...specialAttrValuesFromJSON(product.special_attrs_json || '{}') }
  if (kind !== 'roasted') customForm.value.copy_bom = false
}

function handleCustomProductKindChange() {
  customForm.value.base_product_id = 0
  customForm.value.name = ''
  customForm.value.copy_bom = customForm.value.product_kind === 'roasted' && customForm.value.custom_type !== 'custom_roast'
  customForm.value.copy_price_tiers = customForm.value.product_kind !== 'green_bean' && customForm.value.custom_type !== 'custom_roast'
}

function navigateProductBom(row) {
  const productID = Number(row?.product_id || productProductionConfigProduct.value?.id || productProductionConfigForm.value.product_id || row?.id || 0)
  const bomID = Number(row?.production_bom_id || row?.bom_id || productProductionConfigForm.value.production_bom_id || 0)
  const productName = productProductionConfigForm.value.name || row?.name || productProductionConfigProduct.value?.name || ''
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'bom',
      params: bomID > 0 ? { production_bom_id: bomID } : {},
      returnNavigation: productID > 0 ? {
        label: productName ? `返回商品档案配置：${productName}` : '返回商品档案配置',
        key: 'productMaster',
        params: { open_product_config_id: productID },
      } : null,
    },
  }))
}

function isInactiveMarker(value) {
  if (value === false || value === 0) return true
  const normalized = String(value ?? '').trim().toLowerCase()
  return ['inactive', 'disabled', 'deprecated', 'deactivated', 'archived', 'false', '0', '失效'].includes(normalized)
}

function isActiveBomUsageRow(row = {}) {
  return !isInactiveMarker(row.status) && !isInactiveMarker(row['bom_status']) && !isInactiveMarker(row.active)
}

function bomUsageBomID(row = {}) {
  return Number(row.bom_id || row.production_bom_id || row.id || 0)
}

function bomUsageRowKey(row = {}) {
  const bomID = bomUsageBomID(row)
  if (bomID > 0) return `bom:${bomID}`
  const code = String(row.bom_code || row.production_bom_code || row.code || '').trim()
  const name = String(row.bom_name || row.production_bom_name || row.name || '').trim()
  return `bom:${code}:${name}`
}

function bomUsageRelationLabel(row = {}) {
  const bomID = bomUsageBomID(row)
  const code = String(row.bom_code || row.production_bom_code || row.code || '').trim()
  const name = String(row.bom_name || row.production_bom_name || row.name || '').trim()
  const label = [code, name].filter(Boolean).join(' ') || (bomID > 0 ? `BOM #${bomID}` : '生产 BOM')
  if (row.relation_type === 'output') return `${label} · 产出商品`
  if (row.relation_type === 'component') return `${label} · 作为组件`
  return label
}

function bomUsageStatusLabel(row = {}) {
  if (row.is_default === true || row.isDefault === true) return '默认状态'
  if (isInactiveMarker(row.bom_status ?? row.status ?? row.active)) return '失效状态'
  return '启用状态'
}

function bomUsageStatusClass(row = {}) {
  if (row.is_default === true || row.isDefault === true) return 'default'
  if (isInactiveMarker(row.bom_status ?? row.status ?? row.active)) return 'inactive'
  return 'active'
}

function bomUsageVersionLabel(row = {}) {
  const version = String(row.current_published_version_no || row.currentPublishedVersionNo || row.latest_bom_version_no || row.latestBomVersionNo || row.latest_version_no || row.latestVersionNo || row.production_bom_version_no || row.productionBomVersionNo || row.bom_version_no || row.bomVersionNo || '').trim()
  return version || '-'
}

function navigateCurrentProductBom() {
  navigateProductBom({ id: productProductionConfigForm.value.product_id || productProductionConfigProduct.value?.id || 0 })
}

function returnToPreviousView() {
  const navigation = productReturnNavigation.value
  if (!navigation?.key) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: String(navigation.key),
      params: navigation.params || {},
    },
  }))
}

async function loadProductionBomCatalog() {
  if (productionBoms.value.length) return
  productionBoms.value = await apiGet('/api/production-boms') || []
}

async function loadProcessRoutes() {
  if (processRoutes.value.length) return
  const data = await apiGet('/api/process-routes?status=published')
  processRoutes.value = data?.rows || []
}

async function ensureProductionBomDetail(bomID) {
  const id = Number(bomID || 0)
  if (!id || productionBomDetails.value[String(id)]) return
  const detail = await apiGet(`/api/production-boms/${id}`)
  productionBomDetails.value = { ...productionBomDetails.value, [String(id)]: detail }
}

async function ensureProductBomUsage(productID) {
  const id = Number(productID || 0)
  if (!id || productBomUsageByProductID.value[String(id)]) return
  const rows = await apiGet(`/api/production-bom-product-usage/${id}`)
  productBomUsageByProductID.value = { ...productBomUsageByProductID.value, [String(id)]: rows || [] }
}

async function loadProductSpecMigration(productID) {
  const id = Number(productID || 0)
  if (!id || Object.prototype.hasOwnProperty.call(productSpecMigrationByProductID.value, String(id))) return
  let migration
  try {
    migration = await apiGet(`/api/products/${productID}/bom-spec-migration`)
  } catch (err) {
    if (Number(err?.status || 0) !== 404) throw err
    migration = { product_id: id, state: 'legacy', migration_state: 'legacy' }
  }
  productSpecMigrationByProductID.value = {
    ...productSpecMigrationByProductID.value,
    [String(id)]: { ...migration, migration_state: migration?.migration_state || migration?.state || 'legacy' },
  }
}

function normalizeProductSpecMigrationResponse(row = {}, productID = 0) {
  return {
    ...row,
    product_id: Number(row?.product_id || productID || 0),
    migration_state: row?.migration_state || row?.state || 'legacy',
  }
}

function productBomSpecMigrationSnapshot(mapping = {}) {
  const raw = mapping?.metadata_snapshot ?? mapping?.metadataSnapshot ?? {}
  let snapshot = raw
  if (typeof raw === 'string') {
    try {
      snapshot = JSON.parse(raw || '{}')
    } catch (_) {
      snapshot = { historical_configuration: raw }
    }
  }
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) snapshot = {}
  const labels = {
    production_bom_id: '旧 BOM',
    production_bom_version_id: '旧 BOM 版本',
    process_route_id: '旧工艺路线',
    expected_loss_rate: '旧损耗率',
    note: '旧生产备注',
    industry_field_template_id: '旧行业字段模板',
  }
  const formatConfig = (title, config = {}) => Object.entries(labels)
    .filter(([key]) => config?.[key] !== undefined && config?.[key] !== null && config?.[key] !== '')
    .map(([key, label]) => `${title}${label}：${config[key]}`)
  const rows = [
    ...formatConfig('', snapshot),
    ...formatConfig('该旧规格 · ', snapshot.legacy_production_config),
    ...formatConfig('父商品 · ', snapshot.parent_production_config),
  ]
  return rows.length ? rows.join('\n') : '无单独旧生产配置；请按旧规格名称和单位核对新配方。'
}

async function mutateProductBomSpecMigration(action, successMessage) {
  const productID = Number(productProductionConfigProduct.value?.id || productProductionConfigForm.value.product_id || 0)
  if (!productID) {
    error.value = '请选择商品档案'
    return null
  }
  productSpecMigrationSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await apiSend(`/api/products/${productID}/bom-spec-migration/${action}`, { method: 'POST' })
    const migration = normalizeProductSpecMigrationResponse(row, productID)
    productSpecMigrationByProductID.value = {
      ...productSpecMigrationByProductID.value,
      [String(productID)]: migration,
    }
    ok.value = successMessage
    return migration
  } catch (err) {
    error.value = err.message || '更新 BOM 规格迁移状态失败'
    return null
  } finally {
    productSpecMigrationSaving.value = false
  }
}

async function prepareProductBomSpecMigration() {
  await mutateProductBomSpecMigration('prepare', '旧规格元数据已迁入；请到 BOM 重建并发布完整规格配方')
}

async function assessProductBomSpecMigration() {
  const migration = await mutateProductBomSpecMigration('readiness', '切换条件已重新检查')
  if (migration?.readiness?.ready) ok.value = '切换条件已满足；确认后才会停用旧子 SKU'
}

async function cutoverProductBomSpecs() {
  if (!window.confirm('确认切换到默认已发布 BOM 的规格组？切换后旧子 SKU 将成为只读历史记录，新订单、价格、库存和生产只接受父商品 + BOM 规格。')) return
  const migration = await mutateProductBomSpecMigration('cutover', '商品已切换到 BOM 规格；旧子 SKU 仅保留历史追溯')
  if (!migration) return
  const productID = Number(migration.product_id || productProductionConfigProduct.value?.id || 0)
  products.value = products.value.map((row) => Number(row.id || 0) === productID
    ? { ...row, migration_state: 'cutover', bom_spec_migration_state: 'cutover' }
    : row)
  const nextUsage = { ...productBomUsageByProductID.value }
  delete nextUsage[String(productID)]
  productBomUsageByProductID.value = nextUsage
  await ensureProductBomUsage(productID)
  const defaultBomID = bomUsageBomID(productProductionDefaultBomUsageRow.value || {})
    || Number(productProductionConfigForm.value.production_bom_id || 0)
  if (defaultBomID > 0) {
    const nextDetails = { ...productionBomDetails.value }
    delete nextDetails[String(defaultBomID)]
    productionBomDetails.value = nextDetails
    await ensureProductionBomDetail(defaultBomID)
  }
}

async function setDefaultProductionBom(row = {}) {
  const productID = Number(productProductionConfigProduct.value?.id || productProductionConfigForm.value.product_id || 0)
  const bomID = bomUsageBomID(row)
  const versionID = Number(row.current_published_version_id || row.production_bom_version_id || row.bom_version_id || 0)
  if (!productID || !bomID) {
    error.value = '请选择可生产该商品的生产 BOM'
    return
  }
  productProductionConfigSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const saved = await apiSend(`/api/products/${productID}/default-production-bom`, {
      method: 'PUT',
      body: {
        default_production_bom_id: Number(row.bom_id || row.production_bom_id || row.id || 0),
      },
    })
    productProductionConfigForm.value.production_bom_id = Number(saved?.production_bom_id || bomID)
    productProductionConfigForm.value.production_bom_version_id = Number(saved?.production_bom_version_id || versionID)
    const existingConfig = productProductionConfigByProductID(productID) || { product_id: productID }
    productProductionConfigs.value = [
      ...productProductionConfigs.value.filter((config) => Number(config.product_id || 0) !== productID),
      {
        ...existingConfig,
        production_bom_id: productProductionConfigForm.value.production_bom_id,
        production_bom_version_id: productProductionConfigForm.value.production_bom_version_id,
      },
    ]
    const key = String(productID)
    const nextUsage = { ...productBomUsageByProductID.value }
    delete nextUsage[key]
    productBomUsageByProductID.value = nextUsage
    await ensureProductBomUsage(productID)
    ok.value = '默认生产 BOM 已更新'
  } catch (err) {
    error.value = err.message || '设置默认生产 BOM 失败'
  } finally {
    productProductionConfigSaving.value = false
  }
}

function productProductionConfigByProductID(productID) {
  const id = Number(productID || 0)
  return productProductionConfigs.value.find((row) => Number(row.product_id || 0) === id) || null
}

function normalizeProductProductionConfigFieldForSave(field = {}, index = 0) {
  const label = String(field.label || '').trim()
  const fieldKey = String(field.field_key || '').trim()
    || label.toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '_').replace(/^_+|_+$/g, '')
    || `field_${index + 1}`
  const fieldType = ['text', 'textarea', 'number', 'ratio', 'select', 'checkbox', 'date'].includes(String(field.field_type || '').trim()) ? String(field.field_type || '').trim() : 'text'
  const usesText = ['text', 'textarea', 'select', 'date'].includes(fieldType)
  const usesNumber = ['number', 'ratio'].includes(fieldType)
  const optionsJSON = normalizeFieldOptionsJSON(field.options_json)
  return {
    id: Number(field.id || 0),
    field_key: fieldKey,
    template_field_key: String(field.template_field_key || fieldKey).trim(),
    label,
    field_type: fieldType,
    unit: String(field.unit || '').trim(),
    value_text: usesText ? String(field.value_text || '').trim() : '',
    value_number: usesNumber && field.value_number !== null && typeof field.value_number !== 'undefined' && field.value_number !== '' ? Number(field.value_number) : null,
    value_bool: fieldType === 'checkbox' ? Boolean(field.value_bool) : null,
    required: Boolean(field.required),
    options_json: optionsJSON,
    show_in_price_list: field.show_in_price_list !== false,
    sort_order: Number(field.sort_order || index + 1),
  }
}

function normalizeFieldOptionsJSON(value) {
  if (Array.isArray(value)) return JSON.stringify(value)
  const text = String(value || '').trim()
  if (!text) return '[]'
  try {
    const parsed = JSON.parse(text)
    return JSON.stringify(Array.isArray(parsed) ? parsed : [])
  } catch (_) {
    return JSON.stringify(text.split(/[,\n]/).map((item) => item.trim()).filter(Boolean))
  }
}

function fieldOptions(field = {}) {
  try {
    const parsed = JSON.parse(field.options_json || '[]')
    return Array.isArray(parsed) ? parsed.map((item) => String(item || '').trim()).filter(Boolean) : []
  } catch (_) {
    return []
  }
}

function fieldTypeLabel(type) {
  return ({
    text: '文本',
    textarea: '长文本',
    number: '数字',
    ratio: '比例',
    select: '选项',
    checkbox: '勾选',
    date: '日期',
  })[String(type || '').trim()] || '文本'
}

function industryFieldTemplateOptionLabel(template = {}) {
  const name = String(template.name || '').trim() || `行业字段模板 #${Number(template.id || 0)}`
  const priority = Number(template.selected_order || 0) > 0 ? `优先级 ${Number(template.selected_order)} · ` : ''
  if (!template.unavailable) return `${priority}${name}`
  return `${priority}${name}${String(template.status || '') === 'missing' ? '（不可用，可取消）' : '（已停用，可取消）'}`
}

async function selectProductProductionConfigBom(bom) {
  const bomID = Number((typeof bom === 'object' && bom !== null ? bom.id : bom) || 0)
  productProductionConfigForm.value.production_bom_id = bomID
  productProductionConfigForm.value.production_bom_version_id = 0
  await ensureProductionBomDetail(bomID)
  const latest = productProductionConfigVersionOptions.value[0]
  if (latest) productProductionConfigForm.value.production_bom_version_id = Number(latest.id || 0)
}

function isCurrentProductProductionConfigOpen(generation, productID) {
  const currentProductID = Number(productProductionConfigProduct.value?.id || productProductionConfigForm.value.product_id || 0)
  return generation === productProductionConfigOpenGeneration
    && productProductionConfigDrawerOpen.value
    && currentProductID === Number(productID || 0)
}

function isCurrentProductProductionConfigIndustryProjection(generation, productID, industryFieldTemplateSignature) {
  return isCurrentProductProductionConfigOpen(generation, productID)
    && industryFieldTemplateIDsFromConfig(productProductionConfigForm.value).join(',') === String(industryFieldTemplateSignature || '')
}

async function openProductProductionConfig(row) {
  const openGeneration = ++productProductionConfigOpenGeneration
  const config = productProductionConfigByProductID(row?.id)
  const productID = Number(row?.id || config?.product_id || 0)
  const industryFieldTemplateIDs = industryFieldTemplateIDsFromConfig(config)
  const industryFieldTemplateSignature = industryFieldTemplateIDs.join(',')
  const industryFieldTemplatesAvailableAtOpen = industryFieldTemplatesForConfig(config).length === industryFieldTemplateIDs.length
  productProductionConfigProduct.value = row || null
  productProductionConfigForm.value = defaultProductProductionConfigForm(config, row)
  const parentID = Number(row?.parent_product_id || row?.parentProductID || row?.id || 0)
  const parentProduct = products.value.find((product) => Number(product.id || 0) === parentID) || row || {}
  if (Number(productProductionConfigForm.value.unit_template_id || 0) > 0 && !productProductionConfigForm.value.unit_rule_override_enabled) {
    applyProductUnitTemplateToForm(productProductionConfigForm.value)
  }
  productProductionConfigDrawerOpen.value = true
  error.value = ''
  try {
    let industryFieldTemplatesPromise = loadIndustryFieldTemplates()
    if (!industryFieldTemplatesAvailableAtOpen && industryFieldTemplateIDs.length) {
      industryFieldTemplatesPromise = industryFieldTemplatesPromise.then(() => {
        if (!isCurrentProductProductionConfigIndustryProjection(openGeneration, productID, industryFieldTemplateSignature)) return
        productProductionConfigForm.value.fields = productProductionConfigFieldsFromTemplates(
          config?.fields || [],
          industryFieldTemplatesForConfig(config),
        )
      })
    }
    await Promise.all([
      loadProductionBomCatalog(),
      loadProcessRoutes(),
      loadProductSpecMigration(productID),
      industryFieldTemplatesPromise,
    ])
    if (!isCurrentProductProductionConfigOpen(openGeneration, productID)) return
    await ensureProductBomUsage(productID)
    if (!isCurrentProductProductionConfigOpen(openGeneration, productID)) return
    {
      const defaultBomID = bomUsageBomID(productProductionDefaultBomUsageRow.value || {})
        || Number(productProductionConfigForm.value.production_bom_id || 0)
      if (defaultBomID > 0) await ensureProductionBomDetail(defaultBomID)
    }
    if (productProductionConfigForm.value.production_bom_id) await ensureProductionBomDetail(productProductionConfigForm.value.production_bom_id)
    if (!isCurrentProductProductionConfigOpen(openGeneration, productID)) return
    if (productProductionConfigForm.value.production_bom_id && !productProductionConfigForm.value.production_bom_version_id) {
      const latest = productProductionConfigVersionOptions.value[0]
      if (latest) productProductionConfigForm.value.production_bom_version_id = Number(latest.id || 0)
    }
  } catch (err) {
    if (!isCurrentProductProductionConfigOpen(openGeneration, productID)) return
    error.value = err.message || '加载商品生产配置失败'
  }
}

async function loadIndustryFieldTemplates() {
  if (industryFieldTemplates.value.length) return
  const data = await apiGet('/api/industry-field-templates')
  industryFieldTemplates.value = data?.rows || []
}

function industryFieldTemplatesForConfig(config = {}) {
  const templatesByID = new Map(industryFieldTemplates.value.map((template) => [Number(template.id || 0), template]))
  return industryFieldTemplateIDsFromConfig(config)
    .map((id) => templatesByID.get(Number(id || 0)))
    .filter(Boolean)
}

function applyIndustryFieldTemplateToProductionConfig() {
  const industryFieldTemplateIDs = industryFieldTemplateIDsFromConfig(productProductionConfigForm.value)
  productProductionConfigForm.value.industry_field_template_ids = industryFieldTemplateIDs
  productProductionConfigForm.value.industry_field_template_id = Number(industryFieldTemplateIDs[0] || 0)
  productProductionConfigForm.value.fields = productProductionConfigFieldsFromTemplates(
    productProductionConfigForm.value.fields || [],
    industryFieldTemplatesForConfig(productProductionConfigForm.value),
  )
}

function closeProductProductionConfigDrawer() {
  productProductionConfigOpenGeneration += 1
  productProductionConfigDrawerOpen.value = false
  productProductionConfigProduct.value = null
  productProductionConfigForm.value = defaultProductProductionConfigForm()
}

async function refreshClassificationTemplates() {
  const data = await apiGet('/api/product-classification-templates')
  productClassificationTemplates.value = (data.rows || []).map(decorateProductClassificationTemplate)
}

function editClassificationCategory(category) {
  classificationCategoryForm.value = {
    id: Number(category.id || 0),
    name: category.name || '',
    sort_order: Number(category.sort_order || 100),
    product_config_template_id: Number(category.product_config_template_id || 0),
    gradient_template_id: Number(category.gradient_template_id || 0),
    unit_template_id: Number(category.unit_template_id || 0),
  }
}

async function saveClassificationCategory() {
  const templateID = Number(classificationTemplateForm.value.id || 0)
  if (!templateID) return
  const form = classificationCategoryForm.value
  const id = Number(form.id || 0)
  await apiSend(id ? `/api/product-classification-template-categories/${id}` : '/api/product-classification-template-categories', {
    method: id ? 'PUT' : 'POST',
    body: {
      template_id: templateID,
      parent_id: 0,
      name: String(form.name || '').trim(),
      level: 1,
      sort_order: Number(form.sort_order || 100),
      product_config_template_id: Number(form.product_config_template_id || 0),
      gradient_template_id: 0,
      unit_template_id: 0,
    },
  })
  classificationCategoryForm.value = defaultClassificationCategoryForm()
  await refreshClassificationTemplates()
}

async function moveClassificationCategory(category, delta) {
  const nextSort = Math.max(1, Number(category.sort_order || 100) + (Number(delta || 0) * 10))
  classificationCategoryForm.value = {
    id: Number(category.id || 0),
    name: category.name || '',
    sort_order: nextSort,
    product_config_template_id: Number(category.product_config_template_id || 0),
    gradient_template_id: Number(category.gradient_template_id || 0),
    unit_template_id: Number(category.unit_template_id || 0),
  }
  await saveClassificationCategory()
}

async function deleteClassificationCategory(category) {
  const templateID = Number(classificationTemplateForm.value.id || 0)
  if (!templateID || !category?.id) return
  if (!window.confirm(`删除分类「${category.name}」？该分类下对象会回到未分类。`)) return
  await apiSend(`/api/product-classification-template-categories/${category.id}?template_id=${templateID}`, { method: 'DELETE' })
  await refreshClassificationTemplates()
}

async function saveProductClassificationTemplateUsage() {
  const templateID = Number(selectedProductClassificationTemplateID.value || 0)
  if (!templateID) return
  const payload = buildClassificationTemplateUsagePayload({
    classification_template_id: templateID,
    sort_order: productClassificationTabs.value.length * 10 + 100,
  })
  await apiSend('/api/product-classification-template-usages/products', { body: payload })
  selectedProductClassificationTemplateID.value = 0
  await loadAll()
  activeProductClassificationTab.value = `template-${templateID}`
}

async function confirmProductClassificationTemplateUsage(template) {
  const templateID = Number(template?.id || 0)
  if (!templateID) return
  selectedProductClassificationTemplateID.value = templateID
  if (!window.confirm(`增加分类「${template?.name || templateID}」？`)) {
    selectedProductClassificationTemplateID.value = 0
    return
  }
  await saveProductClassificationTemplateUsage()
}

async function saveAliasClassificationTemplateUsage() {
  const templateID = Number(selectedAliasClassificationTemplateID.value || 0)
  const customerID = Number(selectedAliasCustomerID.value || 0)
  if (!templateID || !customerID) return
  const payload = buildClassificationTemplateUsagePayload({
    customer_id: customerID,
    classification_template_id: templateID,
    sort_order: aliasClassificationTabs.value.length * 10 + 100,
  })
  await apiSend('/api/product-classification-template-usages/customer-aliases', { body: payload })
  selectedAliasClassificationTemplateID.value = 0
  await loadAll()
  activeAliasClassificationTab.value = `template-${templateID}`
}

async function confirmAliasClassificationTemplateUsage(template) {
  const templateID = Number(template?.id || 0)
  if (!templateID) return
  selectedAliasClassificationTemplateID.value = templateID
  if (!window.confirm(`为当前客户增加分类「${template?.name || templateID}」？`)) {
    selectedAliasClassificationTemplateID.value = 0
    return
  }
  await saveAliasClassificationTemplateUsage()
}

async function saveSelectedProductClassificationAssignment() {
  const templateID = isProductAllOrUnclassifiedTab.value
    ? Number(selectedProductClassificationMoveID.value || 0)
    : Number(currentProductClassificationTemplate.value?.id || 0)
  if (!templateID || !selectedProductIds.value.length) return
  const categoryID = isProductAllOrUnclassifiedTab.value ? 0 : Number(selectedProductClassificationCategoryID.value || 0)
  for (const productID of selectedProductIds.value) {
    await apiSend('/api/product-classification-assignments/products', {
      body: {
        product_id: Number(productID || 0),
        template_id: templateID,
        category_id: categoryID,
        sort_order: 100,
      },
    })
  }
  selectedProductIds.value = []
  selectedProductClassificationTemplateID.value = 0
  selectedProductClassificationMoveID.value = 0
  selectedProductClassificationCategoryID.value = 0
  await refreshClassificationTemplates()
  activeProductClassificationTab.value = `template-${templateID}`
}

async function saveProductGroupFeatureSelection() {
  const payload = businessGroupFeatureSelectionPayload('product_catalog', productGroupFeatureSelectionDraft.value)
  productGroupFeatureSelectionSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/business-group-feature-selections/product_catalog', {
      method: 'PUT',
      body: payload,
    })
    productGroupFeatureSelectionIDs.value = businessGroupFeatureSelectionIDs(result)
    productGroupFeatureSelectionDraft.value = [...productGroupFeatureSelectionIDs.value]
    if (!productCatalogBusinessGroupRows().some((group) => Number(group.id || 0) === Number(selectedProductGroupTemplateID.value || 0))) {
      selectedProductGroupTemplateID.value = Number(productCatalogBusinessGroupRows()[0]?.id || 0)
    }
    productCategoryMoveActive.value = false
    ok.value = payload.group_template_ids.length
      ? `商品档案已选择 ${payload.group_template_ids.length} 个分组模板`
      : '商品档案已改为平铺展示'
  } catch (err) {
    error.value = err.message || '保存商品档案分组模板失败'
  } finally {
    productGroupFeatureSelectionSaving.value = false
  }
}

async function saveSelectedProductBusinessGroupAssignment(target = {}) {
  const unclassified = Boolean(target?.unclassified)
  const option = unclassified ? null : {
    group_id: Number(target?.group_id || 0),
    group_item_id: Number(target?.group_item_id || 0),
  }
  if (!unclassified && (!(option.group_id > 0) || !(option.group_item_id > 0))) return false
  if (!productCatalogBusinessGroups.value.length || !selectedProductIds.value.length) return false
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    for (const productID of selectedProductIds.value) {
      if (!option) {
        await clearProductBusinessGroupAssignment(productID)
        continue
      }
      await apiSend('/api/business-group-assignments', {
        body: businessGroupMoveAssignmentPayload({
          usageKey: 'product_catalog',
          objectKey: 'product',
          objectID: Number(productID || 0),
          option,
          sortOrder: 100,
        }),
      })
    }
    const movedCount = selectedProductIds.value.length
    await loadAll({ strict: true })
    selectedProductIds.value = []
    ok.value = `已移动 ${movedCount} 个商品到分类`
    return true
  } catch (err) {
    error.value = err.message || '移动商品分类失败'
    return false
  } finally {
    loading.value = false
  }
}

async function handleProductCategoryMoveTarget(target) {
  const moved = await saveSelectedProductBusinessGroupAssignment(target)
  if (moved) productCategoryMoveActive.value = false
}

async function clearProductBusinessGroupAssignment(productID) {
  const id = Number(productID || 0)
  if (!id) return
  const url = new URL('/api/business-group-assignments', window.location.origin)
  url.searchParams.set('usage_key', 'product_catalog')
  url.searchParams.set('object_key', 'product')
  url.searchParams.set('object_id', String(id))
  const data = await apiGet(url)
  const rows = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data?.assignments) ? data.assignments : [])
  await Promise.all(rows.map((row) => apiSend(`/api/business-group-assignments/${row.id}`, { method: 'DELETE' })))
}

function openProductBusinessGroupManagement() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'groupTemplates',
      returnNavigation: {
        key: 'productMaster',
        label: '返回商品档案',
      },
    },
  }))
}

async function confirmSelectedProductClassificationMove(option) {
  if (!selectedProductIds.value.length) {
    error.value = '请先勾选商品'
    return
  }
  const label = classificationMoveOptionLabel(option)
  if (isProductAllOrUnclassifiedTab.value) {
    selectedProductClassificationMoveID.value = Number(option?.id || 0)
  } else {
    selectedProductClassificationCategoryID.value = classificationMoveCategoryID(option)
  }
  if (selectedProductRowsAlreadyInCurrentCategory.value) {
    selectedProductClassificationMoveID.value = 0
    selectedProductClassificationCategoryID.value = 0
    return
  }
  if (!window.confirm(`移动选中商品到「${label || '未分类'}」？`)) {
    selectedProductClassificationMoveID.value = 0
    selectedProductClassificationCategoryID.value = 0
    return
  }
  await saveSelectedProductClassificationAssignment()
}

async function saveSelectedAliasClassificationAssignment() {
  const templateID = isAliasAllOrUnclassifiedTab.value
    ? Number(selectedAliasClassificationMoveID.value || 0)
    : Number(currentAliasClassificationTemplate.value?.id || 0)
  if (!templateID || !selectedAliasIds.value.length) return
  const categoryID = isAliasAllOrUnclassifiedTab.value ? 0 : Number(selectedAliasClassificationCategoryID.value || 0)
  for (const aliasID of selectedAliasIds.value) {
    await apiSend('/api/product-classification-assignments/customer-aliases', {
      body: {
        alias_id: Number(aliasID || 0),
        template_id: templateID,
        category_id: categoryID,
        sort_order: 100,
      },
    })
  }
  selectedAliasIds.value = []
  selectedAliasClassificationTemplateID.value = 0
  selectedAliasClassificationMoveID.value = 0
  selectedAliasClassificationCategoryID.value = 0
  await refreshClassificationTemplates()
  activeAliasClassificationTab.value = `template-${templateID}`
}

async function confirmSelectedAliasClassificationMove(option) {
  if (!selectedAliasIds.value.length) {
    error.value = '请先勾选客户商品'
    return
  }
  const label = classificationMoveOptionLabel(option)
  if (isAliasAllOrUnclassifiedTab.value) {
    selectedAliasClassificationMoveID.value = Number(option?.id || 0)
  } else {
    selectedAliasClassificationCategoryID.value = classificationMoveCategoryID(option)
  }
  if (selectedAliasRowsAlreadyInCurrentCategory.value) {
    selectedAliasClassificationMoveID.value = 0
    selectedAliasClassificationCategoryID.value = 0
    return
  }
  if (!window.confirm(`移动选中客户商品到「${label || '未分类'}」？`)) {
    selectedAliasClassificationMoveID.value = 0
    selectedAliasClassificationCategoryID.value = 0
    return
  }
  await saveSelectedAliasClassificationAssignment()
}

async function saveProductProductionConfig() {
  const productID = Number(productProductionConfigProduct.value?.id || productProductionConfigForm.value.product_id || 0)
  if (!productID) {
    error.value = '请选择商品档案'
    return
  }
  const industryFieldTemplateIDs = industryFieldTemplateIDsFromConfig(productProductionConfigForm.value)
  const fields = productProductionConfigFieldsFromTemplates(
    productProductionConfigForm.value.fields || [],
    industryFieldTemplatesForConfig(productProductionConfigForm.value),
  )
    .map((field, index) => normalizeProductProductionConfigFieldForSave(field, index))
    .filter((field) => field.label || field.value_text || field.value_number !== null || field.value_bool !== null)
  productProductionConfigSaving.value = true
  error.value = ''
  ok.value = ''
  try {
	    const originalProduct = productProductionConfigProduct.value || {}
	    await apiSend(`/api/products/${productID}`, {
	      method: 'PUT',
	      body: buildProductProductionConfigBasicsPayload(originalProduct, productProductionConfigForm.value),
	    })
    const result = await apiSend(`/api/product-production-configs/${productID}`, {
      method: 'PUT',
      body: {
        production_bom_id: Number(productProductionConfigForm.value.production_bom_id || 0),
        production_bom_version_id: Number(productProductionConfigForm.value.production_bom_version_id || 0),
        process_route_id: Number(productProductionConfigForm.value.process_route_id || 0),
        industry_field_template_ids: industryFieldTemplateIDs,
        industry_field_template_id: Number(industryFieldTemplateIDs[0] || 0),
        note: String(productProductionConfigForm.value.note || '').trim(),
        fields,
      },
    })
    const saved = result?.config
    if (saved) {
      productProductionConfigs.value = [
        ...productProductionConfigs.value.filter((row) => Number(row.product_id || 0) !== productID),
        saved,
      ]
    }
    ok.value = '商品档案配置已保存'
    closeProductProductionConfigDrawer()
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存商品档案配置失败'
  } finally {
    productProductionConfigSaving.value = false
  }
}

async function createSku() {
  if (!skuForm.value.name) {
    error.value = '请填写商品名称'
    return
  }
  skuSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/product-settings/products', {
      body: buildProductCreatePayload(skuForm.value),
    })
    ok.value = '商品档案已创建'
    skuForm.value = defaultSkuForm()
    closeProductDrawer()
    await loadAll()
    const createdProductForConfig = resolveCreatedProductForConfig(result, products.value)
    if (createdProductForConfig) {
      await openProductProductionConfig(createdProductForConfig)
    }
  } catch (err) {
    error.value = err.message || '创建商品档案失败'
  } finally {
    skuSaving.value = false
  }
}

async function createProduct() {
  ensureProductTypeCategorySelected(productForm.value)
  if (!productForm.value.name) {
    error.value = '请填写商品名称'
    return
  }
  productSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/product-settings/products', {
      body: buildProductCreatePayload(productForm.value),
    })
    const assigned = await assignCreatedSkuToSelectedProductSubtype(result?.product, productForm.value)
    ok.value = assigned ? '公共产品已创建并挂到产品子类型' : '公共产品已创建，未选择产品子类型，已进入停车场'
    productForm.value = defaultProductForm()
    closeProductDrawer()
    await loadAll()
  } catch (err) {
    error.value = err.message || '创建公共产品失败'
  } finally {
    productSaving.value = false
  }
}

async function createCustomProduct() {
  const customerID = Number(selectedCustomerSkuCustomerID.value || customForm.value.customer_id || 0)
  if (!customerID) {
    error.value = '请选择客户'
    return
  }
  ensureProductTypeCategorySelected(customForm.value)
  if (customForm.value.product_kind !== 'green_bean' && customForm.value.custom_type !== 'custom_roast' && !customForm.value.base_product_id) {
    error.value = '请选择基础产品'
    return
  }
  if (customForm.value.product_kind === 'green_bean' || customForm.value.custom_type === 'custom_roast') {
    customForm.value.base_product_id = 0
    customForm.value.copy_bom = false
    customForm.value.copy_price_tiers = false
  }
  if (!customForm.value.name) {
    fillCustomProductName()
  }
  if (!customForm.value.name) {
    error.value = '请填写专属 SKU 名称'
    return
  }
  customSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/product-settings/custom-products', {
      body: buildCustomProductCreatePayload(customerID, customForm.value),
    })
    const assigned = await assignCreatedSkuToSelectedProductSubtype(result?.product, customForm.value)
    ok.value = assigned ? '客户专属 SKU 已创建并挂到产品子类型' : '客户专属 SKU 已创建，未选择产品子类型，已进入停车场'
    customForm.value = { ...defaultCustomForm(), customer_id: customerID }
    closeProductDrawer()
    await loadAll()
  } catch (err) {
    error.value = err.message || '创建专属 SKU 失败'
  } finally {
    customSaving.value = false
  }
}

async function savePublicCategoryUsageForCustomer(event) {
  const checked = Boolean(event?.target?.checked)
  await saveCustomerPublicUsage({
    use_public_categories: checked,
    use_public_sku: customerUsesPublicSku.value,
    use_public_gradient_templates: customerUsesPublicGradientTemplates.value,
  }, checked ? '已开启公共商品分类引用' : '已关闭公共商品分类引用')
}

async function savePublicGradientTemplateUsageForCustomer(event) {
  const checked = Boolean(event?.target?.checked)
  await saveCustomerPublicUsage({
    use_public_sku: customerUsesPublicSku.value,
    use_public_categories: customerUsesPublicCategories.value,
    use_public_gradient_templates: checked,
  }, checked ? '已开启公共梯度模板引用' : '已关闭公共梯度模板引用')
}

async function saveCustomerPublicUsage(options, successMessage) {
  if (!selectedCustomerSkuCustomerID.value) {
    error.value = '请选择履约客户'
    return
  }
  publicUsageSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const payload = buildCustomerPublicUsagePayload(selectedCustomerSkuCustomerID.value, {
      use_public_sku: Boolean(options?.use_public_sku),
      use_public_categories: Boolean(options?.use_public_categories),
      use_public_gradient_templates: Boolean(options?.use_public_gradient_templates),
    })
    await apiSend('/api/product-settings/customer-public-usage', { body: payload })
    ok.value = successMessage || '公共引用设置已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存公共引用设置失败'
  } finally {
    publicUsageSaving.value = false
  }
}

function resetCustomerProductAliasForm() {
  customerProductAliasForm.value = defaultCustomerProductAliasForm()
}

function openCustomerAliasCreateDrawer(mode = 'single') {
  if (!selectedAliasCustomerID.value) {
    error.value = '请选择客户'
    return
  }
  customerAliasCreateMode.value = mode === 'batch' ? 'batch' : 'single'
  customerProductAliasForm.value = {
    ...defaultCustomerProductAliasForm(),
    customer_id: Number(selectedAliasCustomerID.value || 0),
  }
  aliasBatchForm.value = {
    ...defaultCustomerProductAliasBatchForm(),
    customer_id: Number(selectedAliasCustomerID.value || 0),
  }
  aliasBatchFilters.value = defaultAliasBatchFilters()
  selectedAliasBatchProductIds.value = []
  customerAliasCreateDrawerOpen.value = true
}

function openCustomerProductAliasEditor(alias = {}) {
  const customerID = Number(alias.customer_id || selectedAliasCustomerID.value || 0)
  if (!customerID) {
    error.value = '请选择客户'
    return
  }
  selectedAliasCustomerID.value = customerID
  customerAliasCreateMode.value = 'single'
  customerProductAliasForm.value = {
    ...defaultCustomerProductAliasForm(),
    ...alias,
    id: Number(alias.id || 0),
    customer_id: customerID,
    product_id: Number(alias.product_id || 0),
    display_name: String(alias.display_name || '').trim(),
    brand_name: String(alias.brand_name || '').trim(),
    product_config_template_id: Number(alias.product_config_template_id || 0),
    sort_order: Number(alias.sort_order || 0),
    active: alias.active !== false,
    remark: String(alias.remark || '').trim(),
  }
  customerAliasCreateDrawerOpen.value = true
}

function closeCustomerAliasCreateDrawer() {
  customerAliasCreateDrawerOpen.value = false
  selectedAliasBatchProductIds.value = []
  resetCustomerProductAliasForm()
}

function aliasBatchProductExists(product) {
  const productID = Number(product?.id || 0)
  const customerID = Number(selectedAliasCustomerID.value || 0)
  return visibleCustomerProductAliases.value.some((alias) => Number(alias.customer_id || 0) === customerID && Number(alias.product_id || 0) === productID && alias.active !== false)
}

function isAliasBatchProductSelected(product) {
  return selectedAliasBatchProductIds.value.includes(Number(product?.id || 0))
}

function toggleAliasBatchProduct(product, checked) {
  const productID = Number(product?.id || 0)
  if (!productID || aliasBatchProductExists(product)) return
  if (checked) {
    if (!selectedAliasBatchProductIds.value.includes(productID)) selectedAliasBatchProductIds.value = [...selectedAliasBatchProductIds.value, productID]
    return
  }
  selectedAliasBatchProductIds.value = selectedAliasBatchProductIds.value.filter((id) => Number(id) !== productID)
}

function toggleAllAliasBatchProducts(checked) {
  if (!checked) {
    selectedAliasBatchProductIds.value = []
    return
  }
  const ids = aliasBatchCandidateRows.value
    .filter((product) => !aliasBatchProductExists(product))
    .map((product) => Number(product.id || 0))
    .filter(Boolean)
  selectedAliasBatchProductIds.value = Array.from(new Set([...selectedAliasBatchProductIds.value, ...ids]))
}

function filterAliasBatchRows(rows = [], filters = {}) {
  const query = String(filters.query || '').trim().toLowerCase()
  return (rows || []).filter((row) => {
    if (query) {
      const haystack = `${row.name || ''} ${row.number || ''} ${row.id || ''}`.toLowerCase()
      if (!haystack.includes(query)) return false
    }
    return row.active !== false
  })
}

async function saveCustomerAliasBatch() {
  const payload = buildCustomerProductAliasBatchPayload({
    ...aliasBatchForm.value,
    customer_id: selectedAliasCustomerID.value || aliasBatchForm.value.customer_id,
    product_ids: selectedAliasBatchProductIds.value,
  })
  if (!payload.customer_id) {
    error.value = '请选择客户'
    return
  }
  if (!payload.product_ids.length) {
    error.value = '请选择商品档案'
    return
  }
  aliasBatchSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/customer-product-aliases/batch', { body: payload })
    ok.value = `客户商品批量添加完成：创建 ${Number(result?.created_count || 0)} 个，跳过 ${Number(result?.skipped_count || 0)} 个`
    closeCustomerAliasCreateDrawer()
    await loadAll()
  } catch (err) {
    error.value = err.message || '批量添加客户商品失败'
  } finally {
    aliasBatchSaving.value = false
  }
}

function openCustomerAliasSection() {
  if (skuContextCustomerID.value > 0) {
    selectedAliasCustomerID.value = skuContextCustomerID.value
  }
  activeSettingsSection.value = 'aliases'
  resetCustomerProductAliasForm()
}

async function saveCustomerProductAlias() {
  const payload = buildCustomerProductAliasPayload({
    ...customerProductAliasForm.value,
    customer_id: selectedAliasCustomerID.value || customerProductAliasForm.value.customer_id,
  })
  if (!payload.customer_id) {
    error.value = '请选择客户'
    return
  }
  if (!payload.product_id) {
    error.value = '请选择绑定商品档案'
    return
  }
  if (!payload.display_name) {
    error.value = '请填写客户商品'
    return
  }
  aliasSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/customer-product-aliases/${payload.id}` : '/api/customer-product-aliases'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '客户商品已保存'
    closeCustomerAliasCreateDrawer()
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存客户商品失败'
  } finally {
    aliasSaving.value = false
  }
}

async function disableCustomerProductAlias(alias) {
  const id = Number(alias?.id || 0)
  if (!id) return
  aliasSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/customer-product-aliases/${alias.id}/disable`)
    ok.value = '客户商品已停用'
    await loadAll()
  } catch (err) {
    error.value = err.message || '停用客户商品失败'
  } finally {
    aliasSaving.value = false
  }
}

async function batchDisableCustomerProductAliases() {
  const ids = selectedAliasIds.value.map((id) => Number(id || 0)).filter((id) => id > 0)
  if (!ids.length) return
  aliasSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/customer-product-aliases/batch-disable', { body: { ids } })
    ok.value = `批量停用完成：停用 ${Number(result?.disabled_count || 0)} 个，跳过 ${Number(result?.skipped_count || 0)} 个`
    selectedAliasIds.value = []
    await loadAll()
  } catch (err) {
    error.value = err.message || '批量停用客户商品失败'
  } finally {
    aliasSaving.value = false
  }
}

function industryFieldOptions(field = {}) {
  try {
    const parsed = JSON.parse(String(field.options_json || '[]'))
    return Array.isArray(parsed) ? parsed.map((item) => String(item || '').trim()).filter(Boolean) : []
  } catch (_) {
    return []
  }
}

async function openAliasIndustryFieldDrawer(alias) {
  const id = Number(alias?.id || 0)
  if (!id) return
  aliasIndustryFieldAlias.value = alias
  aliasIndustryFieldDrawerOpen.value = true
  aliasIndustryFieldSaving.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/customer-product-aliases/${id}/industry-fields`)
    aliasIndustryFieldForm.value = {
      fields: (data.fields || []).map((field, index) => ({
        ...defaultProductProductionConfigField(field, index),
        value_text: String(field.value_text || '').trim(),
      })),
    }
  } catch (err) {
    error.value = err.message || '加载客户行业字段失败'
    aliasIndustryFieldForm.value = { fields: alias.industry_fields || [] }
  } finally {
    aliasIndustryFieldSaving.value = false
  }
}

function closeAliasIndustryFieldDrawer() {
  aliasIndustryFieldDrawerOpen.value = false
  aliasIndustryFieldAlias.value = null
  aliasIndustryFieldForm.value = { fields: [] }
}

async function saveAliasIndustryFields() {
  const id = Number(aliasIndustryFieldAlias.value?.id || 0)
  if (!id) return
  aliasIndustryFieldSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend(`/api/customer-product-aliases/${id}/industry-fields`, {
      method: 'PUT',
      body: buildCustomerProductAliasIndustryFieldPayload(aliasIndustryFieldForm.value),
    })
    const updatedFields = data.fields || []
    customerProductAliases.value = customerProductAliases.value.map((row) => Number(row.id || 0) === id
      ? { ...row, industry_fields: updatedFields }
      : row)
    if (aliasIndustryFieldAlias.value) {
      aliasIndustryFieldAlias.value = { ...aliasIndustryFieldAlias.value, industry_fields: updatedFields }
    }
    ok.value = '客户行业字段已保存'
    closeAliasIndustryFieldDrawer()
  } catch (err) {
    error.value = err.message || '保存客户行业字段失败'
  } finally {
    aliasIndustryFieldSaving.value = false
  }
}

function nextCategoryName(baseName, siblings = []) {
  const existingNames = new Set((siblings || []).map((category) => String(category?.name || '').trim()))
  if (!existingNames.has(baseName)) return baseName
  let index = 2
  while (existingNames.has(`${baseName} ${index}`)) index += 1
  return `${baseName} ${index}`
}

function togglePrimaryDeleteMode() {
  primaryDeleteMode.value = !primaryDeleteMode.value
  if (primaryDeleteMode.value) secondaryDeleteModeFor.value = 0
}

function toggleSecondaryDeleteMode(primary) {
  const id = Number(primary?.id || 0)
  if (!id || !canEditCategory(primary)) return
  secondaryDeleteModeFor.value = secondaryDeleteModeFor.value === id ? 0 : id
  if (secondaryDeleteModeFor.value) primaryDeleteMode.value = false
}

function isFirstPrimaryCategory(primary) {
  return Number(primary?.number || 0) <= 1
}

function isLastPrimaryCategory(primary) {
  return Number(primary?.number || 0) >= categoryManagementTreeForSkuContext.value.length
}

function normalizeCategoryIdList(values = []) {
  const source = Array.isArray(values) ? values : []
  return [...new Set(source.map((value) => Number(value || 0)).filter((value) => value > 0))]
}

function toggleCategoryId(listRef, id) {
  const normalizedID = Number(id || 0)
  if (!normalizedID) return
  if (listRef.value.includes(normalizedID)) {
    listRef.value = listRef.value.filter((value) => Number(value) !== normalizedID)
    return
  }
  listRef.value = [...listRef.value, normalizedID]
}

function expandPrimaryCategory(id) {
  const normalizedID = Number(id || 0)
  if (!normalizedID) return
  collapsedPrimaryCategoryIds.value = collapsedPrimaryCategoryIds.value.filter((value) => Number(value) !== normalizedID)
}

function expandSecondaryCategory(id) {
  const normalizedID = Number(id || 0)
  if (!normalizedID) return
  collapsedSecondaryCategoryIds.value = collapsedSecondaryCategoryIds.value.filter((value) => Number(value) !== normalizedID)
}

function isPrimaryCategoryCollapsed(primary) {
  if (isCategorySearchActive.value) return false
  return collapsedPrimaryCategoryIds.value.includes(Number(primary?.id || 0))
}

function isSecondaryCategoryCollapsed(secondary) {
  if (isCategorySearchActive.value) return false
  return collapsedSecondaryCategoryIds.value.includes(Number(secondary?.id || 0))
}

function togglePrimaryCategoryCollapse(primary) {
  toggleCategoryId(collapsedPrimaryCategoryIds, Number(primary?.id || 0))
}

function toggleSecondaryCategoryCollapse(secondary) {
  toggleCategoryId(collapsedSecondaryCategoryIds, Number(secondary?.id || 0))
}

async function focusCategoryAfterCreate(category) {
  const id = Number(category?.id || 0)
  if (!id) return
  const parentID = Number(category?.parent_id || 0)
  categoryCollapsed.value = false
  categorySearchQuery.value = ''
  if (parentID) expandPrimaryCategory(parentID)
  expandPrimaryCategory(id)
  expandSecondaryCategory(id)
  await nextTick()
  if (typeof document === 'undefined') return
  const selector = parentID ? `[data-secondary-id="${id}"]` : `[data-primary-id="${id}"]`
  const element = document.querySelector(selector)
  element?.scrollIntoView?.({ block: 'center', behavior: 'smooth' })
  const input = element?.querySelector?.('.category-name-form input')
  input?.focus?.()
  input?.select?.()
}

async function ensureProductCatalogBusinessGroupForEdit() {
  const current = selectedProductCatalogBusinessGroup()
  if (current) return current
  const result = await apiSend('/api/business-groups', {
    body: {
      name: '默认分组模板',
      code: 'product_catalog',
      remark: '商品档案归组分组集',
      active: true,
      sort_order: 10,
      items: [],
    },
  })
  const created = result?.group || result || {}
  const createdID = Number(created.id || 0)
  if (createdID > 0) {
    await apiSend('/api/business-group-feature-selections/product_catalog', {
      method: 'PUT',
      body: businessGroupFeatureSelectionPayload('product_catalog', [...productGroupFeatureSelectionIDs.value, createdID]),
    })
  }
  await loadAll()
  return createdID > 0 ? created : selectedProductCatalogBusinessGroup()
}

async function saveProductCatalogBusinessGroupItem(body, successMessage = '分组已保存') {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const group = body.group_id ? null : await ensureProductCatalogBusinessGroupForEdit()
    const id = Number(body.id || 0)
    const payload = {
      id,
      group_id: Number(body.group_id || group?.id || 0),
      parent_id: Number(body.parent_id || 0),
      name: String(body.name || '').trim(),
      code: String(body.code || '').trim(),
      remark: String(body.remark || '').trim(),
      active: body.active !== false,
      sort_order: Number(body.sort_order || body.position || 100),
    }
    const result = await apiSend(id ? `/api/business-group-items/${id}` : '/api/business-group-items', {
      method: id ? 'PUT' : 'POST',
      body: payload,
    })
    ok.value = successMessage
    await loadAll()
    return result?.item || null
  } catch (err) {
    error.value = err.message || '保存分组失败'
    return null
  } finally {
    loading.value = false
  }
}

async function moveProductCatalogBusinessGroupItem(itemID, parentID, position, successMessage = '分组顺序已保存') {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend(`/api/business-group-items/${itemID}/move`, {
      body: {
        parent_id: Number(parentID || 0),
        position: Number(position || 1),
      },
    })
    ok.value = successMessage
    await loadAll()
    return result?.item || null
  } catch (err) {
    error.value = err.message || '移动分组失败'
    return null
  } finally {
    loading.value = false
  }
}

async function createPrimaryCategoryInline() {
  const name = nextCategoryName('新大类', categoryManagementTreeForSkuContext.value)
  const category = await saveProductCatalogBusinessGroupItem({
    name,
    parent_id: 0,
    sort_order: (categoryManagementTreeForSkuContext.value.length + 1) * 10,
  }, '大类已保存')
  const id = Number(category?.id || 0)
  if (id) {
    editingCategoryId.value = id
    editingCategoryName.value = category.name || name
    await focusCategoryAfterCreate({ ...category, id, parent_id: 0 })
  }
}

async function createSecondaryCategoryInline(primary) {
  if (!canEditCategory(primary)) return
  const name = nextCategoryName('新小类', primary.children || [])
  const category = await saveProductCatalogBusinessGroupItem({
    group_id: Number(primary.group_id || selectedProductCatalogBusinessGroup()?.id || 0),
    name,
    parent_id: Number(primary.id),
    sort_order: (Number(primary.children?.length || 0) + 1) * 10,
  }, '小类已保存')
  const id = Number(category?.id || 0)
  if (id) {
    editingCategoryId.value = id
    editingCategoryName.value = category.name || name
    await focusCategoryAfterCreate({ ...category, id, parent_id: Number(primary.id || 0) })
  }
}

async function movePrimaryCategory(category, direction) {
  if (isCategorySearchActive.value) return
  if (!canEditCategory(category)) return
  const currentPosition = Number(category.number || category.position || 0)
  const position = currentPosition + Number(direction || 0)
  if (position < 1 || position > categoryManagementTreeForSkuContext.value.length) return
  await moveProductCatalogBusinessGroupItem(Number(category.id || 0), 0, position, '大类顺序已保存')
}

function startCategoryEdit(category) {
  if (!canEditCategory(category)) return
  editingCategoryId.value = Number(category.id)
  editingCategoryName.value = category.name || ''
}

function cancelCategoryNameEdit() {
  editingCategoryId.value = 0
  editingCategoryName.value = ''
}

async function saveCategoryName(category) {
  if (!canEditCategory(category)) return
  if (editingCategoryId.value !== Number(category.id)) return
  const name = String(editingCategoryName.value || '').trim()
  if (!name) return
  if (name === String(category.name || '').trim()) {
    cancelCategoryNameEdit()
    return
  }
  const saved = await saveProductCatalogBusinessGroupItem({
    id: Number(category.id || 0),
    group_id: Number(category.group_id || selectedProductCatalogBusinessGroup()?.id || 0),
    name,
    parent_id: Number(category.parent_id || 0),
    code: category.code || '',
    remark: category.remark || '',
    active: category.active !== false,
    sort_order: Number(category.position || category.sort_order || category.number || 1),
  }, '分组已保存')
  if (saved) {
    cancelCategoryNameEdit()
  }
}

async function deleteCategory(category) {
  if (!canEditCategory(category)) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/business-group-items/${category.id}`, { method: 'DELETE' })
    ok.value = '分组项已停用，相关对象已回到未分组'
    if (Number(category.parent_id || 0) === 0) {
      primaryDeleteMode.value = false
    } else {
      secondaryDeleteModeFor.value = 0
    }
    await loadAll()
  } catch (err) {
    error.value = err.message || '停用分组失败'
  } finally {
    loading.value = false
  }
}

function startCategoryDrag(category) {
  if (isCategorySearchActive.value) return
  if (!canEditCategory(category)) return
  dragging.value = {
    type: 'category',
    id: Number(category.id),
    parentID: Number(category.parent_id || 0),
    position: Number(category.number || category.position || 0),
  }
  categoryDropTarget.value = null
}

function startCategoryPointerDrag(event, primary, visualPosition, category) {
  if (isCategorySearchActive.value) return
  if (!canEditCategory(primary) || !canEditCategory(category)) return
  if (event.button !== 0 || isInteractiveDragSource(event.target)) return
  pointerDrag.value = {
    pointerID: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    primaryID: Number(primary.id),
    visualPosition: Number(visualPosition),
    category,
    active: false,
    element: event.currentTarget,
  }
  event.currentTarget.setPointerCapture?.(event.pointerId)
  event.currentTarget.addEventListener('pointermove', handleCategoryPointerMove)
  event.currentTarget.addEventListener('pointerup', handleCategoryPointerUp)
  event.currentTarget.addEventListener('pointercancel', cancelCategoryPointerDrag)
}

function isInteractiveDragSource(target) {
  return Boolean(target?.closest?.('button,input,select,textarea,a,.product-chip'))
}

function handleCategoryPointerMove(event) {
  const state = pointerDrag.value
  if (!state || state.pointerID !== event.pointerId) return
  const moved = Math.abs(event.clientX - state.startX) + Math.abs(event.clientY - state.startY)
  if (!state.active) {
    if (moved < 5) return
    state.active = true
    startCategoryDrag(state.category)
  }
  event.preventDefault()
  const target = resolveCategoryPointerTarget(event.clientX, event.clientY, state.primaryID)
  if (!target) return
  categoryDropTarget.value = { parentID: Number(target.primary.id), position: Number(target.position) }
}

async function handleCategoryPointerUp(event) {
  const state = pointerDrag.value
  if (!state || state.pointerID !== event.pointerId) return
  const target = categoryDropTarget.value
  const active = state.active
  cleanupCategoryPointerDrag()
  if (!active) {
    clearDrag()
    return
  }
  event.preventDefault()
  const primary = categoryManagementTreeForSkuContext.value.find((item) => Number(item.id) === Number(target?.parentID))
  if (!primary || !target) {
    clearDrag()
    return
  }
  await dropCategoryAtPosition(primary, target.position)
}

function cancelCategoryPointerDrag() {
  cleanupCategoryPointerDrag()
  clearDrag()
}

function cleanupCategoryPointerDrag() {
  const state = pointerDrag.value
  if (state?.element) {
    state.element.removeEventListener('pointermove', handleCategoryPointerMove)
    state.element.removeEventListener('pointerup', handleCategoryPointerUp)
    state.element.removeEventListener('pointercancel', cancelCategoryPointerDrag)
  }
  pointerDrag.value = null
}

function resolveCategoryPointerTarget(clientX, clientY, fallbackPrimaryID) {
  const elements = document.elementsFromPoint(clientX, clientY)
  const primaryElement = elements.find((item) => item.classList?.contains('primary-category'))
    || document.querySelector(`[data-primary-id="${fallbackPrimaryID}"]`)
  if (!primaryElement) return null
  const primaryID = Number(primaryElement.dataset.primaryId || fallbackPrimaryID)
  const primary = categoryManagementTreeForSkuContext.value.find((item) => Number(item.id) === primaryID)
  if (!primary) return null
  const secondaryElements = Array.from(primaryElement.querySelectorAll('.secondary-category'))
  if (!secondaryElements.length) return { primary, position: 1 }
  for (let index = 0; index < secondaryElements.length; index += 1) {
    const rect = secondaryElements[index].getBoundingClientRect()
    if (clientY < rect.top + rect.height / 2) {
      return { primary, position: index + 1 }
    }
  }
  return { primary, position: secondaryElements.length + 1 }
}

function startProductDrag(product) {
  if (!canDragSkuRow(product)) {
    error.value = '只能拖拽当前 SKU 归属下的商品'
    clearDrag()
    return
  }
  dragging.value = { type: 'product', id: Number(product.id) }
  categoryDropTarget.value = null
}

function clearDrag() {
  dragging.value = null
  categoryDropTarget.value = null
}

function scheduleClearDrag() {
  window.setTimeout(clearDrag, 0)
}

function isDraggingCategory(category) {
  return dragging.value?.type === 'category' && dragging.value.id === Number(category.id)
}

function isPointerDraggingCategory(category) {
  return pointerDrag.value?.active && Number(pointerDrag.value.category?.id) === Number(category.id)
}

function setCategoryDropTarget(primary, position) {
  if (isCategorySearchActive.value) return
  if (dragging.value?.type !== 'category') return
  if (!canEditCategory(primary)) return
  categoryDropTarget.value = { parentID: Number(primary.id), position: Number(position) }
}

function handlePrimaryCategoryDragOver(event, primary) {
  if (isCategorySearchActive.value) return
  if (dragging.value?.type !== 'category') return
  if (!canEditCategory(primary)) return
  const target = resolveCategoryPointerTarget(event.clientX, event.clientY, Number(primary.id))
  if (target) {
    categoryDropTarget.value = { parentID: Number(target.primary.id), position: Number(target.position) }
  }
}

function handleSecondaryCategoryDragOver(event, primary, visualPosition) {
  if (isCategorySearchActive.value) return
  if (dragging.value?.type !== 'category') return
  if (!canEditCategory(primary)) return
  const rect = event.currentTarget.getBoundingClientRect()
  const position = event.clientY > rect.top + rect.height / 2 ? Number(visualPosition) + 1 : Number(visualPosition)
  categoryDropTarget.value = { parentID: Number(primary.id), position }
}

function isCategoryDropTarget(primary, position) {
  return dragging.value?.type === 'category'
    && categoryDropTarget.value?.parentID === Number(primary.id)
    && categoryDropTarget.value?.position === Number(position)
}

async function dropCategoryAtPosition(primary, visualPosition) {
  if (isCategorySearchActive.value) {
    clearDrag()
    return
  }
  const drag = dragging.value
  if (drag?.type !== 'category') return
  if (!canEditCategory(primary)) {
    clearDrag()
    return
  }
  const parentID = Number(primary.id)
  let position = Number(visualPosition)
  if (drag.parentID === parentID && drag.position > 0 && drag.position < visualPosition) {
    position -= 1
  }
  if (position <= 0) position = 1
  try {
    await moveProductCatalogBusinessGroupItem(Number(drag.id || 0), parentID, position, '分组顺序已保存')
  } catch (err) {
    error.value = err.message || '移动分组失败'
  } finally {
    clearDrag()
  }
}

async function dropCategoryOnPrimary(primary) {
  if (isCategorySearchActive.value) {
    clearDrag()
    return
  }
  if (dragging.value?.type !== 'category') return
  await dropCategoryAtPosition(primary, Number(primary.children?.length || 0) + 1)
}

async function dropCategoryOnCurrentTarget(primary) {
  if (dragging.value?.type !== 'category') return
  const parentID = Number(primary.id)
  const position = categoryDropTarget.value?.parentID === parentID
    ? categoryDropTarget.value.position
    : Number(primary.children?.length || 0) + 1
  await dropCategoryAtPosition(primary, position)
}

async function dropCategoryOrProductOnSecondary(primary, visualPosition, secondary) {
  const drag = dragging.value
  if (drag?.type === 'category') {
    const parentID = Number(primary.id)
    const position = categoryDropTarget.value?.parentID === parentID
      ? categoryDropTarget.value.position
      : visualPosition
    await dropCategoryAtPosition(primary, position)
    return
  }
  await dropProductOnSecondary(secondary)
}

async function dropProductOnSecondary(secondary) {
  const drag = dragging.value
  if (drag?.type !== 'product') return
  const product = products.value.find((item) => Number(item.id) === Number(drag.id))
  if (!product || !canDragSkuRow(product)) {
    error.value = '只能调整当前 SKU 归属下的商品分类'
    clearDrag()
    return
  }
  const categoryID = Number(secondary.id || 0)
  if (categoryID > 0 && Number(secondary.customer_id || 0) !== skuContextCustomerID.value && Number(secondary.customer_id || 0) !== 0) {
    error.value = '只能移动到当前分组模板'
    clearDrag()
    return
  }
  try {
    await apiSend('/api/business-group-assignments', {
      body: buildBusinessGroupAssignmentPayload({
        usage_key: 'product_catalog',
        object_key: 'product',
        object_id: Number(product.id || drag.id || 0),
        group_id: Number(secondary.group_id || selectedProductCatalogBusinessGroup()?.id || 0),
        group_item_id: Number(secondary.id || 0),
        sort_order: Number(secondary.products?.length || 0) + 1,
      }),
    })
    ok.value = '分类已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '移动分类失败'
  } finally {
    clearDrag()
  }
}

async function saveProductBasics(row, successMessage = '商品基础信息已保存') {
  if (!canEditSkuRow(row)) {
    error.value = '公共商品档案为引用，请回到商品档案维护'
    return
  }
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/products/${row.id}`, {
      method: 'PUT',
      body: buildProductBasicsPayload(row),
    })
    ok.value = successMessage
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存商品失败'
  } finally {
    loading.value = false
  }
}

async function deactivateProducts(productIds) {
  const ids = Array.from(new Set((productIds || []).map((id) => Number(id || 0)).filter((id) => id > 0)))
  if (!ids.length) return
  const message = ids.length > 1
    ? `确认失效选中的 ${ids.length} 个产品？对应 BOM 会同步失效，历史配方明细会保留。`
    : '确认失效该产品？对应 BOM 会同步失效，历史配方明细会保留。'
  if (!window.confirm(message)) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend('/api/product-settings/products/deactivate', { body: { product_ids: ids } })
    selectedProductIds.value = []
    ok.value = ids.length > 1 ? '选中产品已失效，对应 BOM 已同步失效' : '产品已失效，对应 BOM 已同步失效'
    await loadAll()
  } catch (err) {
    error.value = err.message || '产品失效失败'
  } finally {
    loading.value = false
  }
}

async function copyProductArchive(row) {
  if (!row || !row.id) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend(`/api/product-settings/products/${row.id}/copy`, { method: 'POST' })
    ok.value = `已复制商品档案「${row.name}」`
    await loadAll()
    if (result?.product?.id) {
      highlightedSkuId.value = Number(result.product.id || 0)
      setTimeout(() => { highlightedSkuId.value = 0 }, 3000)
    }
  } catch (err) {
    error.value = err.message || '复制商品档案失败'
  } finally {
    loading.value = false
  }
}

watch(selectedCustomerSkuCustomerID, (customerID) => {
  if (restoringProductSettingsDraft) {
    pruneSelectedProducts(filteredSkuRows.value)
    return
  }
  skuForm.value = defaultSkuForm()
  resetProductConfigTemplateForm()
  resetCustomerProductRuleForms()
  skuFilters.value = defaultSkuFilters()
  skuGroupPagination.value = {}
  collapsedPrimaryCategoryIds.value = []
  collapsedSecondaryCategoryIds.value = []
  if (Number(customerID || 0) > 0) {
    selectedAliasCustomerID.value = Number(customerID || 0)
  }
  pruneSelectedProducts(filteredSkuRows.value)
  notifyWorkspaceCustomerChanged(customerID)
})

watch(selectedAliasCustomerID, (customerID) => {
  if (!customerProductAliasForm.value.id) {
    customerProductAliasForm.value.customer_id = Number(customerID || 0)
  }
})

watch(ok, (message) => {
  if (message) notifyKferp('success', String(message))
})

watch(error, (message) => {
  if (message) notifyKferp('error', String(message))
})

watch(() => [props.workspaceMode, props.customerContextId], applyWorkspaceCustomerContext, { immediate: true })

watch(productTypeCategoryOptions, () => {
  ensureProductTypeCategorySelected(skuForm.value)
}, { deep: true })

watch(() => customForm.value.base_product_id, () => {
  if (customForm.value.product_kind === 'green_bean' || customForm.value.custom_type === 'custom_roast') return
  syncCustomFormFromBaseProduct(selectedBaseProduct())
})

watch(() => customForm.value.custom_type, () => {
  if (customForm.value.custom_type === 'custom_roast') {
    customForm.value.base_product_id = 0
    customForm.value.copy_bom = false
    customForm.value.copy_price_tiers = false
  } else {
    handleCustomProductKindChange()
  }
})

watch(skuFilters, resetSkuGroupPages, { deep: true })

watch(() => skuFilters.value.primaryCategory, () => {
  if (!skuSecondaryCategoryOptions.value.includes(skuFilters.value.secondaryCategory)) {
    skuFilters.value.secondaryCategory = ''
  }
})

watch(() => pricingRuleTrialForm.value.parent_product_id, () => {
  if (restoringPricingRuleTrialReturnState) return
  pricingRuleTrialForm.value.product_id = 0
  pricingRuleTrialForm.value.quote_unit = ''
  pricingRuleTrialForm.value.bom_version_id = 0
  pricingRuleTrialForm.value.process_route_id = 0
  pricingRuleTrialForm.value.operation_template_id = 0
  pricingRuleTrialResult.value = null
  pricingRuleTrialActiveExplanation.value = ''
  const defaultSpecID = pricingRuleTrialDefaultProductSpecID(pricingRuleTrialSalesSpecOptions.value)
  if (defaultSpecID > 0) pricingRuleTrialForm.value.product_id = defaultSpecID
})

watch(() => pricingRuleTrialForm.value.product_id, () => {
  if (restoringPricingRuleTrialReturnState) return
  const product = selectedPricingRuleTrialProductSpec.value
  pricingRuleTrialForm.value.quote_unit = product ? pricingRuleTrialProductSpecUnit(product) : ''
  pricingRuleTrialForm.value.bom_version_id = 0
  pricingRuleTrialForm.value.process_route_id = 0
  pricingRuleTrialForm.value.operation_template_id = 0
  pricingRuleTrialResult.value = null
  pricingRuleTrialActiveExplanation.value = ''
})

watch(() => pricingRuleTrialForm.value.customer_id, () => {
  if (restoringPricingRuleTrialReturnState) return
  activePricingRuleTrialProductKindFilter.value = ''
  pricingRuleTrialForm.value.parent_product_id = 0
  pricingRuleTrialForm.value.product_id = 0
  pricingRuleTrialForm.value.bom_version_id = 0
  pricingRuleTrialForm.value.process_route_id = 0
  pricingRuleTrialForm.value.operation_template_id = 0
  pricingRuleTrialForm.value.quote_unit = ''
  pricingRuleTrialResult.value = null
  pricingRuleTrialActiveExplanation.value = ''
})

watch(pricingRuleTrialProductKindFilterOptions, (options) => {
  const selected = String(activePricingRuleTrialProductKindFilter.value || '').trim()
  if (selected && !(options || []).some((option) => option.value === selected)) {
    activePricingRuleTrialProductKindFilter.value = ''
  }
})

watch(pricingRuleTrialMainProducts, (products) => {
  if (restoringPricingRuleTrialReturnState) return
  const selectedParentID = Number(pricingRuleTrialForm.value.parent_product_id || 0)
  if (selectedParentID > 0 && !(products || []).some((product) => Number(product?.id || 0) === selectedParentID)) {
    pricingRuleTrialForm.value.parent_product_id = 0
  }
})

watch(() => pricingRuleTrialAutoRunSignature.value, () => {
  if (restoringPricingRuleTrialReturnState) return
  schedulePricingRuleTrial()
})

watch(currentSkuSourceRows, () => {
  const normalized = normalizeSkuFiltersForCurrentRows()
  if (JSON.stringify(normalized) !== JSON.stringify(skuFilters.value)) {
    skuFilters.value = normalized
  }
}, { deep: true })

watch(filteredSkuRows, (rows) => {
  pruneSelectedProducts(rows)
})

watch(selectedProductGroupTemplateID, () => {
  if (restoringProductSettingsDraft) return
  saveProductSettingsDraft()
})

async function applyProductSettingsViewParams(params = {}) {
  if (await restorePricingRuleTrialReturnState(params)) return
  const productID = Number(params.open_product_config_id || params.return_product_id || 0)
  if (!productID) return
  const row = products.value.find((product) => Number(product.id || 0) === productID)
  if (row) {
    await openProductProductionConfig(row)
  }
}

function clearPricingRuleTrialReturnKeyFromURL() {
  const url = new URL(window.location.href)
  url.searchParams.delete('pricing_rule_trial_return_key')
  replaceHistoryURL(url)
}

async function restorePricingRuleTrialReturnState(params = {}) {
  const returnKey = String(params.pricing_rule_trial_return_key || '').trim()
  if (!returnKey) return false
  const state = takePricingRuleTrialReturnState(returnKey)
  clearPricingRuleTrialReturnKeyFromURL()
  if (!state?.form) return false
  const formState = state.form || {}
  const rule = activePricingRuleTrialOptionByID(formState.pricing_rule_id)
  if (!rule) {
    pricingRuleTrialError.value = '原价格计算模板已不可用，请重新选择'
    return false
  }
  restoringPricingRuleTrialReturnState = true
  try {
    pricingRuleTrialRule.value = defaultPricingRuleForm(JSON.parse(JSON.stringify(rule)))
    pricingRuleTrialForm.value = {
      ...defaultPricingRuleTrialForm(pricingRuleTrialRule.value),
      ...formState,
      other_cost_rows: Array.isArray(formState.other_cost_rows)
        ? formState.other_cost_rows.map((row) => ({ ...row }))
        : [defaultPricingRuleTrialOtherCostRow()],
    }
    activePricingRuleTrialProductKindFilter.value = String(state.product_kind_filter || '')
    pricingRuleTrialResult.value = null
    pricingRuleTrialActiveExplanation.value = ''
    pricingRuleTrialError.value = ''
    pricingRuleTrialUpdateMessage.value = ''
    pricingRuleTrialDrawerOpen.value = true
    await nextTick()
  } finally {
    restoringPricingRuleTrialReturnState = false
  }
  schedulePricingRuleTrial()
  return true
}

watch(() => props.viewParams, (params) => {
  applyProductSettingsViewParams(params || {})
}, { deep: true })

onMounted(async () => {
  await loadAll()
  await applyProductSettingsViewParams(props.viewParams || {})
  await restoreProductSettingsDraft()
})

onBeforeUnmount(saveProductSettingsDraft)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.panel-head h2 { margin: 0 0 4px; font-size: 20px; }
.panel-head p { margin: 0; color: #666; font-size: 13px; }
.sku-page-summary { padding: 8px 12px; }
.sku-page-summary .panel-head { align-items: center; margin-bottom: 0; }
.sku-page-summary .panel-head h2 { margin: 0 0 2px; font-size: 18px; }
.sku-page-summary .ok, .sku-page-summary .error { margin-top: 8px; padding: 7px 10px; font-size: 13px; }
.panel-title { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-weight: 700; margin-bottom: 10px; }
.panel-actions, .row-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.sub-title { width: 100%; font-weight: 700; font-size: 13px; }
button, input, select, textarea { font: inherit; min-height: 36px; border-radius: 6px; }
input, select, textarea { border: 1px solid #cfc8bf; padding: 7px 9px; background: #fff; width: 100%; }
textarea { resize: vertical; line-height: 1.4; }
button { border: 1px solid #1f1f1f; background: #fff; padding: 0 12px; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #111; color: #fff; }
.secondary, .toggle-section { background: #fff; color: #111; }
.toggle-section { min-height: 30px; padding: 0 10px; }
.compact-action { min-height: 30px; padding: 0 10px; font-size: 12px; }
.text-button { border: 0; background: transparent; color: #1f4f82; padding: 0; min-height: 28px; }
.danger-text { color: #a33; }
.settings-workbench { display: grid; gap: 10px; align-items: start; }
.product-section-tabs-legacy { display: inline-flex; align-items: center; gap: 4px; width: fit-content; border: 1px solid #e6e0d8; border-radius: 8px; background: #fbfaf8; padding: 4px; }
.workspace-tab { min-height: 32px; border: 0; border-radius: 6px; background: transparent; color: #333; padding: 0 14px; font-weight: 700; }
.workspace-tab.active { background: #111; color: #fff; }
.sku-master-workspace, .sku-template-workspace { display: grid; gap: 14px; min-width: 0; }
.master-data-layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; align-items: start; min-width: 0; }
.template-workspace-stack { display: grid; gap: 14px; min-width: 0; }
.config-template-tabs { display: inline-flex; align-items: center; gap: 4px; width: fit-content; border: 1px solid #e6e0d8; border-radius: 8px; background: #fbfaf8; padding: 4px; }
.config-template-tab { min-height: 32px; border: 0; border-radius: 6px; background: transparent; color: #333; padding: 0 14px; font-weight: 700; }
.config-template-tab.active { background: #111; color: #fff; }
.customer-rule-panel { grid-column: 1 / -1; }
.customer-rule-binding { display: grid; grid-template-columns: minmax(260px, 360px) minmax(0, 1fr); align-items: end; gap: 12px; margin-bottom: 12px; }
.customer-rule-layout { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(280px, .8fr); gap: 14px; align-items: start; }
.customer-rule-editor { display: grid; gap: 10px; }
.customer-rule-items, .customer-rule-overrides, .customer-rule-template-list { display: grid; gap: 8px; }
.customer-rule-item { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(160px, .7fr) minmax(120px, .5fr) auto; gap: 8px; align-items: end; border: 1px solid #eee; border-radius: 8px; padding: 10px; background: #fafafa; }
.customer-rule-editor label span, .customer-rule-binding label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.rule-config-block { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 8px; align-items: end; min-width: 0; border: 1px solid #eee8df; border-radius: 8px; padding: 10px; background: #fff; }
.field-group-title { grid-column: 1 / -1; color: #3f3328; font-weight: 700; font-size: 13px; }
.field-group-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; font-weight: 700; color: #3f3328; }
.field-group-copy { display: grid; gap: 2px; }
.field-group-copy small { color: #7b746c; font-weight: 400; }
.unit-conversion-editor { grid-column: 1 / -1; display: grid; gap: 8px; min-width: 0; }
.unit-impact-help { grid-column: 1 / -1; color: #7b746c; line-height: 1.45; }
.unit-conversion-row { display: grid; grid-template-columns: minmax(72px, .45fr) minmax(82px, .7fr) auto minmax(72px, .45fr) minmax(82px, .7fr) minmax(118px, .7fr) auto; gap: 6px; align-items: center; min-width: 0; }
.unit-conversion-row span { color: #666; text-align: center; }
.price-rule-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: start; column-gap: 10px; row-gap: 10px; }
.price-rule-grid .rule-config-field { min-width: 0; align-self: start; display: grid; grid-template-rows: 22px auto; gap: 4px; }
.price-rule-grid .rule-config-field select { width: 100%; }
.price-rule-grid .checkline { grid-column: 1 / -1; min-height: 24px; padding-top: 0; }
.price-rule-grid .rule-config-field > span, .field-label-with-help { min-height: 22px; }
.field-label-with-help { display: inline-flex; align-items: center; gap: 6px; width: fit-content; color: #333; }
.field-help-wrap { position: relative; display: inline-flex; align-items: center; }
.field-help-icon { width: 16px; min-width: 16px; height: 16px; min-height: 16px; flex: 0 0 16px; display: inline-flex; align-items: center; justify-content: center; border: 0; border-radius: 999px; padding: 0; background: #111; color: #fff; font-size: 11px; font-weight: 800; line-height: 1; cursor: help; outline: none; }
.field-help-icon:focus-visible { box-shadow: 0 0 0 3px rgba(17, 17, 17, .16); }
.field-help-tooltip { position: absolute; left: 50%; bottom: calc(100% + 8px); transform: translateX(-50%); display: none; width: min(240px, 70vw); padding: 8px 10px; border: 1px solid #d8d2ca; border-radius: 6px; background: #fff; color: #3f3328; font-size: 12px; font-weight: 400; line-height: 1.45; box-shadow: 0 8px 22px rgba(35, 28, 20, .16); z-index: 20; }
.field-help-wrap:hover .field-help-tooltip, .field-help-wrap:focus-within .field-help-tooltip { display: block; }
.product-create-form { display: grid; grid-template-columns: repeat(2, minmax(140px, 1fr)); gap: 10px; align-items: end; }
.custom-product-panel { grid-column: 1 / -1; }
.custom-product-form { display: grid; grid-template-columns: repeat(4, minmax(160px, 1fr)); gap: 10px; align-items: end; }
.product-drawer-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; align-items: end; }
.product-create-form label, .custom-product-form label { display: grid; gap: 5px; font-size: 13px; }
.product-create-form small, .custom-product-form small { color: #7b746c; line-height: 1.35; }
.product-create-form .wide-field, .custom-product-form .wide-field { grid-column: span 2; }
.gradient-template-panel { grid-column: 1 / -1; }
.gradient-template-layout { display: grid; grid-template-columns: minmax(220px, 280px) minmax(0, 1fr); gap: 12px; align-items: start; }
.unit-template-panel { grid-column: 1 / -1; }
.unit-template-layout { display: grid; grid-template-columns: minmax(280px, 340px) minmax(520px, 1fr); gap: 12px; align-items: stretch; }
.unit-template-card { display: grid; gap: 10px; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; padding: 12px; min-width: 0; align-content: start; }
.unit-template-list-panel { display: flex; flex-direction: column; min-height: 0; }
.unit-template-list-panel .template-list { flex: 1 1 auto; min-height: 0; }
.unit-chip-list { display: flex; gap: 6px; flex-wrap: wrap; }
.unit-chip { min-height: 30px; display: inline-flex; align-items: center; gap: 5px; border-color: #d9d2c8; background: #fff; padding: 0 9px; font-size: 12px; }
.unit-chip small { color: #777; }
.unit-chip.inactive { opacity: .55; }
.unit-definition-form, .unit-template-form { display: grid; gap: 10px; }
.unit-definition-form { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: end; }
.unit-definition-form label, .unit-template-form label { display: grid; gap: 5px; font-size: 13px; }
.compact-template-list { max-height: none; overflow: auto; padding-right: 2px; }
.template-list { display: grid; gap: 8px; }
.template-row { min-height: 50px; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: center; text-align: left; border: 1px solid #e2ddd6; background: #fbfaf8; padding: 8px 10px; }
.template-row.active { border-color: #1f4f82; background: #eef6ff; }
.template-row.inactive { opacity: .58; }
.template-row small { color: #666; font-size: 12px; }
.template-row-main { min-height: 0; border: 0; background: transparent; padding: 0; color: inherit; text-align: left; display: grid; gap: 3px; }
.product-config-row { position: relative; cursor: pointer; border-radius: 8px; background: #fff; box-shadow: 0 1px 2px rgba(29, 24, 18, .04); }
.product-config-row:hover { border-color: #c9beb1; background: #fffdf9; }
.product-config-row.active { border-color: #111; background: #f7f7f5; box-shadow: inset 4px 0 0 #111; }
.product-config-row-main { gap: 6px; }
.product-config-row-title { display: flex; align-items: center; gap: 6px; min-width: 0; }
.product-config-row-title strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.template-state-pill { flex: 0 0 auto; border: 1px solid #ded8cf; border-radius: 999px; background: #fbfaf8; padding: 2px 7px; color: #6b6156; font-size: 11px; font-weight: 700; }
.product-config-row-subtitle { color: #3f3328; font-size: 12px; font-weight: 600; }
.template-meta-chips { display: flex; flex-wrap: wrap; gap: 4px; }
.template-meta-chip { border: 1px solid #e4ded6; border-radius: 999px; background: #fbfaf8; color: #5f5a52; padding: 2px 7px; font-size: 11px; line-height: 1.35; }
.template-copy-action { white-space: nowrap; }
.template-editor, .product-config-editor { border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; padding: 12px; }
.pricing-rule-form { display: grid; gap: 10px; }
.template-editor-grid { display: grid; grid-template-columns: minmax(0, 1fr) 160px; gap: 10px; }
.template-editor label, .product-config-editor label { display: grid; gap: 5px; font-size: 13px; }
.unit-template-form .template-editor-grid { grid-template-columns: minmax(0, 1fr); }
.sales-spec-editor { display: grid; gap: 8px; min-width: 0; }
.sales-spec-row { display: grid; grid-template-columns: auto minmax(180px, 1fr) auto minmax(110px, 140px) auto auto auto; gap: 8px; align-items: center; border: 1px solid #e2ddd6; border-radius: 8px; background: #fff; padding: 8px; min-width: 0; }
.sales-spec-row input { min-width: 0; }
.sales-spec-row .conversion-prefix, .sales-spec-row .conversion-equals { color: #6d665c; font-weight: 700; text-align: center; }
.sales-spec-name-input { font-weight: 700; }
.inventory-unit-chip { min-height: 34px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid #e4ded6; border-radius: 6px; background: #fbfaf8; color: #3f3328; padding: 0 10px; font-size: 13px; font-weight: 700; white-space: nowrap; }
.default-spec-toggle { min-height: 34px; border: 1px solid #d7d0c6; border-radius: 6px; background: #fff; color: #5a5146; padding: 0 10px; font: inherit; font-size: 12px; font-weight: 700; white-space: nowrap; cursor: pointer; }
.default-spec-toggle.active { border-color: #111; background: #111; color: #fff; }
.pricing-rule-form-section { display: grid; gap: 8px; padding: 10px; border: 1px solid #ead8c4; background: #fffaf4; border-radius: 6px; }
.pricing-rule-section-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.pricing-rule-other-cost-list { display: grid; gap: 8px; }
.pricing-rule-other-cost-row { display: grid; grid-template-columns: minmax(160px, 1fr) minmax(120px, 180px) auto; gap: 8px; align-items: end; }
.checkbox-line { display: inline-flex; align-items: center; gap: 6px; min-width: 0; font-size: 13px; color: #3f3328; }
.checkbox-line input { width: 16px; height: 16px; min-height: 16px; flex: 0 0 auto; }
.product-config-layout { display: grid; grid-template-columns: minmax(220px, 280px) minmax(0, 1fr); gap: 12px; align-items: start; }
.product-price-management-layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; align-items: start; }
.product-price-management-layout section { display: grid; gap: 10px; min-width: 0; }
.product-price-record-form, .product-tier-price-scheme-form { display: grid; gap: 10px; }
.product-price-record-form .template-editor-grid, .product-tier-price-scheme-form .template-editor-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.product-tier-price-row { grid-template-columns: minmax(110px, .8fr) minmax(100px, .65fr) minmax(100px, .65fr) minmax(180px, 1.2fr) auto; }
.compact-table-wrap table { min-width: 720px; }
.product-config-editor { display: grid; gap: 10px; }
.template-tier-head { display: flex; justify-content: space-between; align-items: center; gap: 10px; margin: 12px 0 8px; }
.template-tier-list { display: grid; gap: 8px; }
.template-tier-row { display: grid; grid-template-columns: minmax(130px, 1.1fr) minmax(130px, .9fr) minmax(130px, .9fr) minmax(100px, .7fr) auto; gap: 8px; align-items: end; border: 1px solid #e2ddd6; border-radius: 8px; background: #fff; padding: 10px; }
.template-select { width: min(280px, 100%); min-height: 30px; padding: 4px 8px; font-size: 12px; }
.sku-panel-title { align-items: flex-start; }
.sku-panel-actions { flex: 1; }
.sku-customer-select { min-width: 220px; max-width: 320px; flex: 1 1 220px; font-weight: 400; }
.sku-filters { display: grid; grid-template-columns: minmax(220px, 1fr) 160px auto; gap: 8px; margin-bottom: 10px; align-items: end; }
.sku-filters label { display: grid; gap: 5px; font-size: 12px; color: #333; }
.filter-actions { display: inline-flex; justify-content: flex-end; align-items: flex-end; gap: 8px; flex-wrap: wrap; }
.checkline { display: flex !important; align-items: center; gap: 8px; min-height: 36px; }
.checkline input { width: auto; min-height: 0; }
.customer-copy-panel { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; border: 1px solid #e6e0d8; border-radius: 8px; background: #fbfaf8; padding: 8px 10px; margin-bottom: 10px; }
.switchline { min-height: 30px; color: #333; font-size: 13px; }
.form-actions { display: flex; justify-content: flex-end; }
.inline-form { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; margin-bottom: 10px; }
.sub-form { margin: 8px 0; }
.category-search { display: grid; gap: 5px; margin-bottom: 10px; font-size: 12px; color: #333; }
.category-action-pill { display: inline-flex; align-items: center; gap: 2px; width: fit-content; border: 1px solid #e3ddd4; border-radius: 999px; background: #fff; padding: 3px; box-shadow: 0 1px 2px rgba(25, 20, 15, .04); }
.category-inline-toolbar { display: flex; align-items: center; margin: -2px 0 10px; }
.category-action-button { width: 28px; height: 28px; min-height: 28px; display: inline-grid; place-items: center; border: 0; border-radius: 999px; background: transparent; color: #4a4037; padding: 0; font-size: 18px; font-weight: 700; line-height: 1; cursor: pointer; }
.category-action-button.compact { width: 24px; height: 24px; min-height: 24px; font-size: 13px; }
.category-action-button:hover:not(:disabled) { background: #f4f0ea; }
.category-action-button.danger-toggle.active { background: #fff1f0; color: #b42318; }
.category-action-button:disabled { cursor: not-allowed; opacity: .36; }
.category-collapse-button { min-width: 52px; height: 26px; min-height: 26px; display: inline-grid; place-items: center; flex: 0 0 auto; border: 1px solid #d8d0c6; border-radius: 999px; background: #fff; color: #4a4037; padding: 0 10px; font-size: 12px; font-weight: 700; line-height: 1; cursor: pointer; }
.category-collapse-button:hover { background: #f4f0ea; }
.category-collapse-button.secondary-collapse { min-width: 24px; width: 24px; height: 24px; min-height: 24px; padding: 0; font-size: 13px; }
.category-scroll-list { max-height: min(640px, calc(100vh - 280px)); overflow: auto; display: grid; gap: 10px; padding-right: 2px; }
.category-tree { display: grid; gap: 10px; min-width: 0; }
.primary-category { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; background: #fbfaf8; min-width: 0; }
.primary-category.collapsed { padding: 0; border-color: transparent; background: transparent; }
.category-head, .secondary-head, .category-actions { display: flex; align-items: center; gap: 8px; justify-content: space-between; }
.primary-category-head { align-items: flex-start; justify-content: space-between; gap: 10px; }
.primary-category.collapsed .primary-category-head { align-items: center; width: 100%; box-sizing: border-box; padding: 10px 12px; border: 1px solid #d8d0c6; border-radius: 8px; background: #fff; box-shadow: 0 1px 2px rgba(25, 20, 15, .05); }
.primary-category-left { display: flex; align-items: flex-start; gap: 8px; flex: 1 1 auto; min-width: 0; }
.primary-category-right { justify-content: flex-end; padding-top: 2px; }
.category-row-actions { flex: 0 0 auto; display: flex; align-items: center; gap: 6px; margin-left: auto; padding-top: 1px; }
.category-sort-pill { display: inline-flex; align-items: center; gap: 2px; border: 1px solid #e3ddd4; border-radius: 999px; background: #fff; padding: 2px; }
.category-delete-button { width: 24px; height: 24px; min-height: 24px; display: inline-grid; place-items: center; flex: 0 0 auto; border: 1px solid #d92d20; border-radius: 999px; background: #fff1f0; color: #b42318; padding: 0; font-size: 14px; font-weight: 800; line-height: 1; cursor: pointer; }
.category-delete-button:hover { background: #ffe4e2; }
.category-title-stack { flex: 1 1 auto; min-width: 0; display: grid; gap: 6px; }
.category-title-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; min-width: 0; }
.category-name-button { border: 0; background: transparent; color: #3f3328; padding: 3px 0; text-align: left; min-width: 0; max-width: 100%; cursor: pointer; }
.category-name-button:disabled { color: #6d665f; cursor: default; }
.primary-name-button strong { display: flex; align-items: baseline; flex-wrap: wrap; gap: 4px; min-width: 0; overflow-wrap: anywhere; }
.primary-name-button small { color: #6d665f; font-weight: 400; }
.category-name-form { flex: 1 1 min(260px, 100%); min-width: min(260px, 100%); max-width: 100%; }
.category-name-form input { width: 100%; min-height: 32px; box-sizing: border-box; }
.category-child-toolbar { display: inline-flex; align-items: center; }
.secondary-head { flex-wrap: wrap; justify-content: flex-start; }
.secondary-head b { min-width: 120px; }
.secondary-category { border: 1px solid #ddd; border-radius: 8px; padding: 9px; margin-left: 34px; background: #fff; cursor: grab; user-select: none; touch-action: none; min-width: 0; overflow: hidden; }
.secondary-category.dragging { opacity: .45; }
.secondary-category.pointer-dragging { cursor: grabbing; }
.secondary-head span { display: inline-flex; width: 24px; height: 24px; align-items: center; justify-content: center; border: 1px solid #ddd; border-radius: 6px; }
.secondary-name-button { display: inline-flex; align-items: center; gap: 8px; min-width: min(190px, 100%); }
.secondary-name-button b { min-width: 0; overflow-wrap: anywhere; }
.secondary-name-form { flex: 1 1 190px; min-width: min(190px, 100%); }
.secondary-head small { color: #666; }
.secondary-category-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; margin-left: auto; flex-wrap: wrap; }
.subtype-config-form { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 8px; width: 100%; max-width: 100%; min-width: 0; box-sizing: border-box; overflow: hidden; margin-top: 10px; padding: 10px; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; cursor: default; user-select: text; }
.subtype-config-title { grid-column: 1 / -1; font-weight: 700; color: #3f3328; }
.subtype-config-help { grid-column: 1 / -1; margin: 0; color: #5f5a52; font-size: 12px; line-height: 1.55; }
.subtype-config-form label { display: grid; gap: 5px; min-width: 0; font-size: 12px; color: #444; }
.subtype-config-form label span { color: #666; font-weight: 600; }
.subtype-config-form input, .subtype-config-form select, .subtype-config-form textarea { min-width: 0; width: 100%; box-sizing: border-box; }
.subtype-config-form input[type="checkbox"] { width: auto; min-height: 0; }
.subtype-config-form small { color: #7b746c; line-height: 1.35; }
.wide-subtype-field { grid-column: 1 / -1; }
.subtype-integer-field { align-self: end; }
.subtype-checkbox-row { display: inline-flex; align-items: center; gap: 8px; }
.subtype-config-actions { grid-column: 1 / -1; gap: 8px; flex-wrap: wrap; }
.category-drop-line { height: 16px; border-top: 2px solid transparent; margin: 2px 0; transition: border-color .12s ease, background .12s ease; }
.category-drop-line.active { border-top-color: #1f4f82; background: #edf5ff; }
.unit-template-note { margin: -4px 0 10px; }
.product-chip-list, .uncategorized { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
.uncategorized .muted { width: 100%; margin: 0; }
.product-chip { border: 1px solid #ddd; border-radius: 8px; padding: 5px 8px; background: #fff; font-size: 12px; cursor: grab; }
.table-wrap { overflow: auto; }
.sku-table-wrap { width: 100%; overflow-x: auto; overflow-y: visible; -webkit-overflow-scrolling: touch; }
table { width: 100%; min-width: 1400px; border-collapse: collapse; }
.sku-table { width: max-content; min-width: 1600px; table-layout: auto; }
.compact-table table { min-width: 760px; }
th, td { border-bottom: 1px solid #eee8df; padding: 8px; text-align: left; font-size: 13px; vertical-align: middle; }
.sku-table th, .sku-table td { white-space: nowrap; }
th { background: #fbfaf8; position: sticky; top: 0; }
.select-col { width: 42px; text-align: center; }
.select-col input { width: 16px; min-height: 16px; }
.sku-category-cell { min-width: 112px; max-width: 220px; white-space: nowrap; }
    .bom-version-warning { display: block; margin-top: 3px; color: #9a5b13; font-size: 12px; white-space: nowrap; }
.sku-name-cell { min-width: 300px; }
.action-cell { min-width: 150px; }
.remark-cell { min-width: 220px; }
.roast-select { min-width: 92px; }
.green-type-select { min-width: 92px; }
.bom-product-select { min-width: 220px; }
.green-bean-detail-row td { background: #f7fbf8; padding-top: 6px; padding-bottom: 12px; }
.green-bean-detail-fields { display: flex; align-items: flex-end; gap: 14px; padding-left: 296px; }
.green-bean-detail-fields label { display: grid; gap: 5px; }
.green-bean-detail-fields label span { color: #667085; font-size: 12px; font-weight: 600; }
.green-bom-field { min-width: 360px; }
.yield-editor { display: flex; align-items: center; gap: 6px; max-width: 130px; }
.yield-input { width: 90px; }
.kind-badge { display: inline-flex; align-items: center; min-height: 20px; padding: 1px 7px; border-radius: 4px; font-size: 12px; font-weight: 600; }
.kind-roasted { color: #8a4b12; background: #fff3df; border: 1px solid #f3c67c; }
.kind-green { color: #12613a; background: #e8f7ee; border: 1px solid #8bd4a6; }
.kind-drip { color: #1f4b7a; background: #eaf3ff; border: 1px solid #9bc4ef; }
.kind-instant { color: #6b3f16; background: #f5efe6; border: 1px solid #cba77d; }
.product-kind-filter { position: sticky; top: -6px; z-index: 1; display: flex; flex-wrap: wrap; gap: 6px; margin: -6px -6px 4px; padding: 8px 6px 6px; border-bottom: 1px solid #edf0f5; background: #fff; }
.product-kind-filter-option { min-height: 28px; padding: 4px 10px; border: 1px solid #d7dbe3; border-radius: 999px; background: #fff; color: #475467; font: inherit; font-size: 12px; line-height: 1.2; cursor: pointer; }
.product-kind-filter-option:hover { background: #f3f6fb; }
.product-kind-filter-option.active { border-color: #2563eb; background: #eff6ff; color: #1d4ed8; font-weight: 600; }
.margin-input { width: 150px; }
.sku-name-input { min-width: 240px; }
.sku-table .sku-name-input { min-width: 300px; }
.product-spec-skus { margin-top: 5px; color: var(--muted); }
.product-spec-skus summary { width: fit-content; cursor: pointer; color: #8a5a24; font-size: 12px; }
.product-spec-sku-list { display: grid; gap: 4px; margin-top: 5px; padding-left: 12px; }
.product-spec-sku-item { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; padding: 4px 8px; border: 1px solid #eadfce; border-radius: 7px; background: #fffaf2; color: #4d3927; text-align: left; }
.product-spec-sku-item small { color: var(--muted); }
.product-spec-sku-item:disabled { opacity: 0.5; cursor: not-allowed; }
.remark-input { width: 180px; min-height: 46px; resize: vertical; }
.status-pill { display: inline-flex; align-items: center; min-height: 24px; border: 1px solid #cfd8cf; border-radius: 999px; padding: 2px 8px; color: #27602e; background: #f2fbf2; white-space: nowrap; }
.status-pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
.pricing-rule-row.inactive td:not(.table-actions):not(:first-child) { opacity: 0.42; }
.pricing-rule-row.inactive td:first-child .pricing-rule-name-button { opacity: 0.42; }
.pricing-rule-row.inactive .pricing-rule-copy-action { opacity: 1; }
.pricing-rule-name-button { text-align: left; font-weight: 700; line-height: 1.25; white-space: normal; overflow-wrap: anywhere; }
.pricing-rule-migration-warning { display: block; margin-top: 4px; color: #a14618; font-weight: 700; line-height: 1.35; }
.pricing-rule-migration-alert { display: grid; gap: 4px; margin: 0 0 14px; }
.product-return-banner { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 10px; padding: 10px 12px; border: 1px solid #dbeafe; border-radius: 8px; background: #eff6ff; color: #1e3a8a; }
.product-return-banner span { font-size: 13px; color: #31577f; }
.product-return-button { border-color: #1d4ed8; color: #1d4ed8; background: #fff; }
.customer-alias-workspace { display: grid; gap: 14px; min-width: 0; }
.customer-alias-panel { display: grid; gap: 12px; }
.customer-alias-form label { display: grid; gap: 5px; min-width: 0; font-size: 13px; }
.customer-alias-form label span { color: #5f5a52; font-weight: 600; }
.customer-alias-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; align-items: end; }
.customer-alias-form .span-2 { grid-column: 1 / -1; }
.alias-filters { display: grid; grid-template-columns: minmax(220px, 260px) minmax(220px, 1fr) 160px auto; gap: 10px; align-items: end; padding: 10px; margin: 10px 0; border: 1px solid #eee8df; border-radius: 8px; background: #fff; }
.alias-filters label { display: grid; gap: 5px; font-size: 13px; }
.alias-filters label span { color: #5f5a52; font-weight: 600; }
.checkbox-row { display: inline-flex !important; grid-template-columns: auto 1fr; align-items: center; gap: 8px !important; }
.checkbox-row input { width: auto; min-height: 0; }
.customer-alias-table { min-width: 980px; }
.table-actions { white-space: nowrap; }
.invalid-product-reference { color: #9d2626; font-weight: 700; }
.inactive-product-warning { display: block; margin-top: 3px; color: #9d2626; font-size: 12px; font-weight: 700; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.muted { color: #666; font-size: 12px; }
.settings-drawer-mask { position: fixed; inset: 0; z-index: 60; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .22); }
.settings-drawer { width: min(760px, 94vw); height: 100%; overflow: auto; background: #fff; box-shadow: -12px 0 32px rgba(0, 0, 0, .16); padding: 16px; display: grid; grid-template-rows: auto 1fr; gap: 12px; }
    .product-editor-drawer { width: min(820px, 94vw); }
    .production-bom-binding-drawer { width: min(680px, 94vw); grid-template-rows: auto 1fr auto; }
    .product-production-config-drawer { width: min(980px, 96vw); grid-template-rows: auto 1fr auto; }
.category-settings-drawer { width: min(920px, 96vw); }
.global-unit-dictionary-drawer { width: min(760px, 94vw); }
.pricing-rule-editor-drawer { width: min(860px, 96vw); }
.drawer-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; border-bottom: 1px solid #eee8df; padding-bottom: 12px; }
.drawer-head h3 { margin: 0 0 4px; font-size: 18px; }
.drawer-head p { margin: 0; color: #666; font-size: 12px; }
.drawer-body { display: grid; gap: 12px; align-content: start; min-width: 0; }
.drawer-footer { border-top: 1px solid #eee8df; padding-top: 10px; display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.picker-head, .product-picker-category-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.picker-actions { display: flex; align-items: center; gap: 8px; }
.check-line { display: inline-flex; align-items: center; gap: 8px; }
.check-line input { width: auto; min-height: 0; }
.product-picker-list { display: grid; gap: 10px; max-height: min(620px, calc(100vh - 260px)); overflow: auto; padding-right: 2px; }
.product-picker-category, .product-picker-subcategory { border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; padding: 10px; display: grid; gap: 8px; }
.product-picker-subcategory { background: #fff; }
.subtype-head { font-size: 13px; }
.product-picker-row { min-height: 38px; border: 1px solid #eee8df; border-radius: 8px; background: #fff; padding: 7px 9px; justify-content: space-between; }
.product-picker-row small { color: #667085; }
.product-picker-row.overwrite small { color: #8a4b12; }
.product-picker-row.inactive { opacity: .5; }
.global-unit-drawer-body { grid-template-columns: minmax(220px, 280px) minmax(0, 1fr); align-items: start; }
.global-unit-chip-list { display: grid; gap: 8px; }
.global-unit-chip { min-height: 50px; justify-content: flex-start; text-align: left; }
.drawer-section { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; background: #fbfaf8; }
.child-sku-list { display: grid; gap: 8px; }
.sales-spec-template-detail-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.sales-spec-default-actions { display: flex; align-items: center; justify-content: flex-end; gap: 6px; flex-wrap: wrap; }
.default-spec-chip { border-color: #9fc5ff; background: #edf5ff; color: #1658a5; font-weight: 700; }
.sales-spec-history-toggle { margin: 0; }
.child-sku-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; border: 1px solid #eee8df; border-radius: 8px; background: #fff; padding: 8px 10px; min-width: 0; }
.child-sku-row.inactive { background: #fbfaf8; opacity: .78; }
.child-sku-row div { min-width: 0; display: grid; gap: 2px; }
.child-sku-row strong, .child-sku-row small { overflow-wrap: anywhere; }
.pricing-rule-trial-drawer { width: min(940px, 96vw); grid-template-rows: auto minmax(0, 1fr) auto; overflow: hidden; }
.pricing-rule-trial-drawer > .drawer-body { overflow: auto; padding-right: 2px; }
.pricing-rule-trial-summary { display: grid; gap: 10px; }
.pricing-rule-trial-summary strong { display: block; margin-bottom: 3px; }
.pricing-rule-trial-rule-grid, .pricing-rule-trial-metrics, .pricing-rule-trial-source { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.pricing-rule-trial-rule-grid span, .pricing-rule-trial-source span { min-width: 0; border: 1px solid #eee8df; border-radius: 6px; background: #fff; padding: 7px 8px; color: #4f453b; font-size: 12px; overflow-wrap: anywhere; }
.pricing-rule-trial-form-section { display: grid; gap: 12px; }
.pricing-rule-trial-grid .wide-field { grid-column: 1 / -1; }
.pricing-rule-trial-bom-field { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: end; }
.pricing-rule-trial-bom-field label { min-width: 0; }
.pricing-rule-trial-footer > div { display: grid; gap: 3px; }
.pricing-rule-trial-footer small { color: #666; line-height: 1.4; }
.pricing-rule-trial-update-message { color: #1f6a3f; }
.pricing-rule-trial-result { display: grid; gap: 12px; }
.pricing-rule-trial-waterfall { display: flex; align-items: stretch; gap: 8px; flex-wrap: wrap; }
.pricing-rule-trial-waterfall-card { min-width: 118px; flex: 1 1 132px; border: 1px solid #e2dacd; border-radius: 8px; background: #fff; padding: 10px; display: grid; gap: 4px; align-content: start; }
.pricing-rule-trial-waterfall-card.interactive { appearance: none; cursor: pointer; text-align: left; font: inherit; color: inherit; transition: border-color .15s ease, box-shadow .15s ease, transform .15s ease; }
.pricing-rule-trial-waterfall-card.interactive:hover, .pricing-rule-trial-waterfall-card.interactive:focus-visible { border-color: #8db6e8; box-shadow: 0 0 0 3px rgba(43, 116, 204, .12); outline: 0; }
.pricing-rule-trial-waterfall-card.interactive:active { transform: translateY(1px); }
.pricing-rule-trial-waterfall-card.active { border-color: #2f7ed8; background: #f5f9ff; }
.pricing-rule-trial-waterfall-card small { color: #6d665c; }
.pricing-rule-trial-waterfall-card small span { display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; margin-left: 4px; border-radius: 999px; background: #fff0d9; color: #8a5400; font-weight: 800; }
.pricing-rule-trial-waterfall-card strong { font-size: 18px; color: #2f2a25; overflow-wrap: anywhere; }
.pricing-rule-trial-waterfall-card em { font-style: normal; font-size: 12px; color: #7c7064; line-height: 1.35; }
.pricing-rule-trial-waterfall-card.warning { border-color: #f1c27d; background: #fffaf1; }
.pricing-rule-trial-waterfall-card.final { border-color: #b8d0f0; background: #f2f7ff; }
.pricing-rule-trial-operator { flex: 0 0 18px; min-height: 76px; display: inline-flex; align-items: center; justify-content: center; color: #6d665c; font-weight: 800; }
.pricing-rule-trial-operator.equals { color: #25568d; }
.pricing-rule-trial-explanation-panel { border: 1px solid #c7d8ef; border-radius: 8px; background: #f7fbff; padding: 12px; display: grid; gap: 10px; color: #2f3d4d; }
.pricing-rule-trial-explanation-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.pricing-rule-trial-explanation-head strong { color: #233a56; }
.pricing-rule-trial-explanation-panel p { margin: 0; line-height: 1.55; font-size: 13px; }
.pricing-rule-trial-explanation-grid { margin: 0; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
.pricing-rule-trial-explanation-grid div { min-width: 0; border: 1px solid #dde8f6; border-radius: 6px; background: #fff; padding: 8px; }
.pricing-rule-trial-explanation-grid dt { color: #6d7480; font-size: 12px; margin-bottom: 3px; }
.pricing-rule-trial-explanation-grid dd { margin: 0; color: #253142; font-weight: 700; overflow-wrap: anywhere; }
.pricing-rule-trial-explanation-formula { border: 1px solid #dde8f6; border-radius: 6px; background: #fff; padding: 8px; overflow-wrap: anywhere; }
.pricing-rule-trial-result-meta { display: flex; flex-wrap: wrap; gap: 8px 14px; color: #5f5a52; font-size: 13px; }
.pricing-rule-trial-base-detail { display: grid; gap: 10px; border: 1px solid #eee8df; border-radius: 8px; padding: 10px; background: #fff; }
.pricing-rule-trial-detail-group { display: grid; gap: 8px; }
.pricing-rule-trial-detail-group > strong { color: #2f2a25; }
.pricing-rule-trial-detail-group td span { display: block; }
.pricing-rule-trial-detail-group td small { display: block; margin-top: 2px; color: #7c7064; line-height: 1.35; }
.pricing-rule-trial-metrics > div { min-width: 0; border: 1px solid #e2dacd; border-radius: 8px; background: #fff; padding: 10px; display: grid; gap: 4px; }
.pricing-rule-trial-metrics small { color: #6d665c; }
.pricing-rule-trial-metrics strong { font-size: 18px; color: #2f2a25; overflow-wrap: anywhere; }
.pricing-rule-trial-metrics .final { border-color: #b8d0f0; background: #f2f7ff; }
.pricing-rule-trial-warnings { border: 1px solid #f1c27d; border-radius: 8px; background: #fff8ec; padding: 10px; color: #7a4a08; }
.pricing-rule-trial-warnings ul { margin: 6px 0 0; padding-left: 18px; }
.warning-text { color: #9a3412; }
.pricing-rule-trial-formula { border: 1px solid #d7e3d5; border-radius: 8px; background: #f7fbf6; padding: 10px 12px; color: #2f4631; display: grid; gap: 8px; }
.pricing-rule-trial-formula strong { color: #213d25; }
.pricing-rule-trial-formula-main { margin: 0; font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.pricing-rule-trial-formula ol { margin: 0; padding-left: 18px; display: grid; gap: 4px; font-size: 12px; line-height: 1.5; }
.product-production-config-body { gap: 14px; }
.bom-spec-migration-panel { display: grid; gap: 10px; }
.migration-actions { align-items: center; flex-wrap: wrap; }
.migration-blockers { display: grid; gap: 6px; }
.migration-blockers ul { margin: 0; padding-left: 20px; }
.bom-spec-migration-panel details { min-width: 180px; }
.bom-spec-migration-panel summary { cursor: pointer; color: #25568d; }
.bom-spec-migration-panel pre { max-width: 420px; margin: 8px 0 0; padding: 8px; border: 1px solid #e2dacd; border-radius: 6px; background: #fbfaf8; color: #3f3a33; font: inherit; font-size: 12px; line-height: 1.5; white-space: pre-wrap; overflow-wrap: anywhere; }
.production-config-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; align-items: end; }
.production-config-grid label, .production-config-field-row label { display: grid; gap: 5px; min-width: 0; font-size: 13px; }
.production-config-grid label span, .production-config-field-row label span { color: #5f5a52; font-weight: 600; }
.production-config-grid .wide-field { grid-column: 1 / -1; }
.readonly-link-button { display: inline-flex; align-items: center; gap: 8px; flex-wrap: wrap; text-align: left; }
.bom-default-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 6px 0; border-bottom: 1px solid #f0e7d8; }
.bom-default-row:last-child { border-bottom: 0; }
.bom-default-row .readonly-link-button { flex: 1; min-width: 0; }
.bom-usage-status { display: inline-flex; align-items: center; min-height: 22px; padding: 1px 7px; border: 1px solid #d8cbb8; border-radius: 999px; background: #fffaf2; color: #755116; font-size: 12px; line-height: 1.4; white-space: nowrap; }
.bom-usage-status.default { border-color: #b8d0f0; background: #f2f7ff; color: #25568d; }
.bom-usage-status.active { border-color: #cddfc9; background: #f5fbf3; color: #2f6c2f; }
.bom-usage-status.inactive { border-color: #e1b6b6; background: #fff0f0; color: #8a1f1f; }
.production-config-fields { display: grid; gap: 10px; }
.production-config-field-row { display: grid; grid-template-columns: minmax(150px, 1fr) minmax(150px, 1fr) minmax(112px, .7fr) minmax(76px, .5fr); gap: 8px; align-items: end; padding: 10px; border: 1px solid #eee8df; border-radius: 8px; background: #fff; }
.production-config-field-row .checkline { align-self: center; }
.field-bool-value { min-height: 58px; align-content: end; }
.field-definition-readonly { display: grid; gap: 4px; min-width: 0; }
.field-definition-readonly strong { font-size: 13px; }
.field-definition-readonly small { color: #6d665c; }
.inline-field-action { display: grid; gap: 5px; min-width: 0; font-size: 13px; align-content: end; }
.inline-field-action > span { color: #5f5a52; font-weight: 600; }
.drawer-head-actions { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; justify-content: flex-end; }
.classification-config-body { display: grid; gap: 12px; }
.classification-category-editor { display: grid; gap: 10px; padding-top: 10px; border-top: 1px solid #eee8df; }
.classification-category-form { display: grid; grid-template-columns: minmax(180px, 1fr) 110px minmax(320px, 1.4fr) auto; gap: 8px; align-items: end; }
.classification-category-template-row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.classification-category-list, .classification-assignment-list { display: grid; gap: 8px; }
.classification-category-row, .classification-assignment-row { display: grid; grid-template-columns: minmax(160px, 1fr) auto auto auto auto; gap: 8px; align-items: center; padding: 8px; border: 1px solid #eee8df; border-radius: 8px; background: #fff; }
.classification-assignment-row { grid-template-columns: minmax(180px, 1fr) minmax(180px, 240px); }
.product-group-feature-selection { display: grid; grid-template-columns: minmax(210px, .8fr) minmax(320px, 1.6fr) auto; gap: 12px; align-items: center; padding: 12px; margin: 10px 0; border: 1px solid #d9e2ec; border-radius: 8px; background: #f8fbff; }
.product-group-feature-selection-copy { display: grid; gap: 3px; }
.product-group-feature-selection-copy small { color: #607086; line-height: 1.4; }
.product-group-feature-selection-options { display: flex; align-items: center; gap: 8px 14px; flex-wrap: wrap; }
.product-group-feature-selection-option { white-space: nowrap; }
.product-group-feature-selection-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
.classification-view-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px; margin: 10px 0; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; flex-wrap: wrap; }
.industry-template-selector { display: flex; align-items: center; justify-content: flex-end; gap: 8px 14px; flex-wrap: wrap; }
.industry-template-option { white-space: nowrap; }
.industry-template-option.unavailable { color: #9a3412; }
.industry-template-priority-hint { flex-basis: 100%; text-align: right; }
.classification-tabs { display: flex; flex-wrap: wrap; gap: 8px; }
.classification-tab { height: 32px; border-color: #d8cec2; background: #fff; color: #2f2a25; }
.classification-tab.active { border-color: #1f1f1f; background: #1f1f1f; color: #fff; }
.classification-select-row { display: grid; grid-template-columns: minmax(160px, 210px) minmax(170px, 230px); gap: 8px; margin-left: auto; align-items: end; }
.product-classification-selects { grid-template-columns: minmax(240px, 360px); }
.classification-template-actions-bottom { justify-content: flex-end; margin-top: 14px; padding-top: 12px; border-top: 1px solid #eee8df; }
.industry-field-cell { max-width: 260px; }
.industry-field-cell span { display: block; line-height: 1.35; color: #3f3a33; }
.classification-group-row td { background: #f6f1ea; border-top: 1px solid #e5ded4; border-bottom: 1px solid #e5ded4; color: #3b332a; padding-left: var(--classification-group-indent, 16px); }
.classification-subgroup-row td { background: #fbf7f1; }
.classification-group-row strong { margin: 0 8px; }
.classification-group-row small { color: #7c7064; }
.classification-template-row td { background: #ece3d6; border-top: 2px solid #d3c6b0; border-bottom: 1px solid #d3c6b0; color: #2f2820; padding-left: 8px; }
.classification-template-row strong { margin: 0 8px; font-size: 16px; }
.classification-template-row small { color: #6b5f4f; }
.classification-template-collapsed td { border-bottom: 2px solid #d3c6b0; }
.product-business-group-empty { justify-content: space-between; color: #6b6258; font-size: 13px; }
.classification-group-toggle { height: 28px; border: 0; background: transparent; color: #1f4f82; padding: 0 4px; }
.classification-item-row td:first-child + td { padding-left: var(--classification-item-indent, 18px); }
.classification-pagination-row td { background: #fbf9f5; padding: 8px 16px 12px; }
.classification-pagination-row :deep(.list-pagination-controls) { margin-top: 0; }
@media (max-width: 1100px) {
  .custom-product-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .customer-rule-item { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .master-data-layout { grid-template-columns: 1fr; }
  .production-config-field-row { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .product-group-feature-selection { grid-template-columns: 1fr; }
  .product-group-feature-selection-actions { justify-content: flex-start; }
}
.sku-table .inactive-sku td { opacity: 0.4; }
.sku-table .inactive-sku td input, .sku-table .inactive-sku td select, .sku-table .inactive-sku td textarea { pointer-events: none; }
@media (max-width: 900px) {
	  .pricing-rule-trial-bom-field { grid-template-columns: 1fr; }
  .page { padding: 12px; }
	  .inline-form, .product-create-form, .custom-product-form, .gradient-template-layout, .product-config-layout, .product-price-management-layout, .unit-template-layout, .global-unit-drawer-body, .unit-definition-form, .template-editor-grid, .template-tier-row, .product-tier-price-row, .pricing-rule-other-cost-row, .pricing-rule-trial-rule-grid, .pricing-rule-trial-metrics, .pricing-rule-trial-source, .product-price-record-form .template-editor-grid, .product-tier-price-scheme-form .template-editor-grid, .sku-filters, .customer-rule-binding, .customer-rule-layout, .customer-rule-item, .subtype-config-form, .rule-config-block, .unit-conversion-row, .sales-spec-row, .customer-alias-form, .production-config-grid, .production-config-field-row { grid-template-columns: 1fr; }
  .product-section-tabs-legacy { width: 100%; }
  .workspace-tab { flex: 1; }
  .panel-actions { justify-content: flex-start; }
  .sku-panel-actions { width: 100%; }
  .sku-customer-select { max-width: none; }
  .product-create-form .wide-field, .custom-product-form .wide-field { grid-column: auto; }
  .template-select { width: 100%; }
  table { min-width: 1400px; }
  .sku-table .inactive-sku td { opacity: 0.4; }
.sku-table .inactive-sku td input, .sku-table .inactive-sku td select, .sku-table .inactive-sku td textarea { pointer-events: none; }
.sku-highlight { animation: sku-flash 3s ease-out; }
@keyframes sku-flash {
  0% { background-color: #fef08a; }
  100% { background-color: transparent; }
}
}

.product-group-template-drawer { width: min(520px, 92vw); }
.product-group-template-list { display: grid; gap: 8px; }
.product-group-template-option { display: flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid #eee8df; border-radius: 6px; }
.product-group-template-option.has-items { background: #faf8f3; }
.product-group-template-option small { margin-left: auto; }
.product-group-template-deleted-warnings { display: grid; gap: 8px; margin-bottom: 4px; }
.warning-banner { padding: 8px 12px; border-radius: 6px; background: #fff4e6; border: 1px solid #ffcc80; color: #8a5a00; font-size: 13px; }
.drawer-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 12px; border-top: 1px solid #eee8df; }
</style>
