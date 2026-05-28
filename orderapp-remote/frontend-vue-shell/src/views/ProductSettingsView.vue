<template>
  <div class="page">
    <section class="panel sku-page-summary">
      <div class="panel-head">
        <div>
          <h2>SKU设置</h2>
          <p>维护公共 SKU、客户专属 SKU、商品分类和商品配置；豆单生成请进入产品价格表。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section class="settings-workbench">
      <div class="panel sku-context-panel compact-sku-context">
        <div class="sku-context-main">
          <div class="sku-context-title-line">
            <div class="context-eyebrow">SKU归属</div>
            <h3>{{ selectedSkuContextLabel }}</h3>
            <div class="context-stats">
              <span>公共SKU {{ publicSkuRows.length }}</span>
              <span>当前SKU {{ skuDisplayTotal }}</span>
              <span>商品分类 {{ categoryTreeForSkuContext.length }}</span>
            </div>
          </div>
          <div v-if="!isWorkspaceCustomerLocked" class="sku-context-controls">
            <button class="secondary compact-action" type="button" @click="selectedCustomerSkuCustomerID = 0" :disabled="!selectedCustomerSkuCustomerID">
              公共SKU
            </button>
            <SearchableSelect
              class="sku-customer-select"
              v-model="selectedCustomerSkuCustomerID"
              :options="customerSkuCustomers"
              :option-label="customerOptionLabel"
              :option-meta="customerOptionMeta"
              :option-value="optionNumericValue"
              placeholder="选择履约客户"
              empty-text="暂无履约客户" />
          </div>
          <p v-else class="muted context-lock-note">客户账户模式下由顶部当前客户控制。</p>
        </div>
      </div>

      <div class="sku-workspace-tabs" role="tablist" aria-label="SKU设置工作区">
        <button
          type="button"
          :class="['workspace-tab', { active: activeSettingsSection === 'master' }]"
          @click="activeSettingsSection = 'master'">
          商品资料
        </button>
        <button
          type="button"
          :class="['workspace-tab', { active: activeSettingsSection === 'templates' }]"
          @click="activeSettingsSection = 'templates'">
          商品配置
        </button>
      </div>

      <div v-show="activeSettingsSection === 'master'" class="sku-master-workspace">
        <div class="master-data-layout">
      <div class="panel product-panel">
        <div class="panel-title sku-panel-title">
          <span>客户SKU列表 · {{ selectedSkuContextLabel }}</span>
          <div class="panel-actions sku-panel-actions">
            <button class="primary compact-action" type="button" @click="openProductDrawer">新增SKU</button>
            <button class="secondary compact-action" type="button" @click="openSkuCopyDrawer">SKU复制</button>
            <button class="secondary compact-action" type="button" @click="deactivateProducts(selectedProductIds)" :disabled="!selectedProductIds.length || loading">
              失效选中产品
            </button>
            <button class="toggle-section" type="button" @click="productsCollapsed = !productsCollapsed">
              {{ productsCollapsed ? '展开' : '收起' }}
            </button>
          </div>
        </div>
        <div v-show="!productsCollapsed">
          <div class="table-wrap sku-table-wrap">
          <div class="sku-filters">
            <label>
              <span>搜索</span>
              <input v-model.trim="skuFilters.query" placeholder="搜索商品名称/类型/备注" />
            </label>
            <label>
              <span>产品类型</span>
              <select v-model="skuFilters.primaryCategory">
                <option value="">全部产品类型</option>
                <option v-for="name in skuPrimaryCategoryOptions" :key="name" :value="name">{{ name }}</option>
              </select>
            </label>
            <label>
              <span>产品子类型</span>
              <select v-model="skuFilters.secondaryCategory">
                <option value="">全部产品子类型</option>
                <option v-for="name in skuSecondaryCategoryOptions" :key="name" :value="name">{{ name }}</option>
              </select>
            </label>
          </div>
          <table :key="skuTableKey" class="sku-table" data-auto-pagination="off">
            <thead>
              <tr>
                <th class="select-col">
                  <input type="checkbox" :checked="allProductRowsSelected" :disabled="!editableDisplaySkuRows.length" @change="toggleAllProductRows($event.target.checked)" />
                </th>
                <th class="sku-col-product-type">产品类型</th>
                <th class="sku-col-product-subtype">产品子类型</th>
                <th>商品编号</th>
                <th class="sku-name-cell">商品</th>
                <th>归属</th>
                <th class="special-attrs-cell">特殊属性</th>
                <th>BOM预期产出率</th>
                <th>利润率覆盖</th>
                <th>BOM状态</th>
                <th>BOM</th>
                <th>处理</th>
                <th class="remark-cell">备注</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="row in displaySkuRows" :key="row.id">
                <tr>
                  <td class="select-col">
                    <input type="checkbox" :checked="isProductSelected(row)" :disabled="!canEditSkuRow(row)" @change="toggleProductSelection(row, $event.target.checked)" />
                  </td>
                  <td class="sku-category-cell">{{ categoryLabel(row, 1) }}</td>
                  <td class="sku-category-cell">{{ categoryLabel(row, 2) }}</td>
                  <td>{{ row.number || '' }}</td>
                  <td class="sku-name-cell">
                    <input
                      class="sku-name-input"
                      v-model.trim="row.name"
                      :disabled="!canEditSkuRow(row)"
                      @change="saveProductBasics(row, 'SKU名称已保存')" />
                  </td>
                  <td>{{ productOwnerLabel(row) }}</td>
                  <td class="special-attrs-cell">
                    <div v-if="specialAttrSchemaForProduct(row).length" class="special-attr-editor compact">
                      <label v-for="attr in specialAttrSchemaForProduct(row)" :key="`row-attr-${row.id}-${attr.key}`">
                        <span>{{ attr.label }}</span>
                        <select v-if="attr.value_type === 'select'" v-model="row.special_attr_values[attr.key]" :disabled="!canEditSkuRow(row)" @change="saveProductBasics(row)">
                          <option value="">未填写</option>
                          <option v-for="option in attr.options" :key="`${attr.key}-${option}`" :value="option">{{ option }}</option>
                        </select>
                        <input v-else v-model.trim="row.special_attr_values[attr.key]" :type="attr.value_type === 'number' ? 'number' : 'text'" :disabled="!canEditSkuRow(row)" @change="saveProductBasics(row)" />
                      </label>
                    </div>
                    <div v-else class="sku-empty-special-attrs">
                      <span class="muted">未配置字段</span>
                      <button class="text-button" type="button" @click="openSpecialAttrConfigForProduct(row)">配置字段</button>
                    </div>
                  </td>
                  <td>
                    <div v-if="productKindSupportsBomParams(row)" class="yield-editor">
                      <input
                        class="yield-input"
                        v-model.number="row.yield_percent"
                        type="number"
                        min="1"
                        max="100"
                        step="0.01"
                        :disabled="!canEditSkuRow(row)"
                        @change="saveProductBasics(row)" />
                      <span>%</span>
                    </div>
                    <span v-else class="muted">-</span>
                  </td>
                  <td>
                    <input
                      class="margin-input"
                      v-model="row.margin_rate_override_input"
                      type="number"
                      min="0"
                      step="0.001"
                      placeholder="留空继承分类模板"
                      :disabled="!canEditSkuRow(row)"
                      @change="saveProductMarginOverride(row)" />
                  </td>
                  <td>
                    <span :class="['status-pill', row.bom_status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.bom_status) }}</span>
                  </td>
                  <td>
                    <button class="text-button" type="button" :disabled="!canEditSkuRow(row)" @click="openProductBom(row)">维护 BOM</button>
                  </td>
                  <td>
                    <button class="text-button danger-text" type="button" :disabled="!canEditSkuRow(row)" @click="deactivateProducts([row.id])">停用</button>
                  </td>
                  <td>
                    <textarea
                      class="remark-input"
                      v-model.trim="row.remark"
                      rows="2"
                      :disabled="!canEditSkuRow(row)"
                      @change="saveProductBasics(row, 'SKU备注已保存')"></textarea>
                  </td>
                </tr>
                <tr v-if="row.product_kind === 'green_bean'" class="green-bean-detail-row">
                  <td :colspan="13">
                    <div class="green-bean-detail-fields">
                      <label>
                        <span>生豆属性</span>
                        <select
                          class="green-type-select"
                          v-model="row.green_bean_type"
                          :disabled="!canEditSkuRow(row)"
                          @change="saveProductBasics(row)">
                          <option v-for="option in greenBeanTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                        </select>
                      </label>
                      <label class="green-bom-field">
                        <span>绑定熟豆</span>
                        <SearchableSelect
                          class="bom-product-select"
                          v-model="row.green_bean_bom_product_id"
                          :options="roastedBomProductsForRow(row)"
                          :option-label="baseProductOptionLabel"
                          :option-meta="baseProductOptionMeta"
                          :option-value="optionNumericValue"
                          :disabled="!canEditSkuRow(row)"
                          placeholder="选择熟豆"
                          empty-text="暂无熟豆产品"
                          @select="saveProductBasics(row)" />
                      </label>
                    </div>
                  </td>
                </tr>
                <tr v-if="row.product_kind === 'drip_bag'" class="green-bean-detail-row">
                  <td :colspan="13">
                    <div class="green-bean-detail-fields">
                      <label>
                        <span>每袋克重</span>
                        <input v-model.number="row.drip_bag_grams" type="number" min="0.01" step="0.01" :disabled="!canEditSkuRow(row)" @change="saveProductBasics(row)" />
                      </label>
                      <label>
                        <span>每盒袋数</span>
                        <input v-model.number="row.drip_box_bag_count" type="number" min="1" step="1" :disabled="!canEditSkuRow(row)" @change="saveProductBasics(row)" />
                      </label>
                    </div>
                  </td>
                </tr>
              </template>
              <tr v-if="!displaySkuRows.length">
                <td :colspan="13" class="muted">{{ selectedCustomerSkuCustomerID ? '暂无客户SKU' : '暂无公共SKU' }}</td>
              </tr>
            </tbody>
          </table>
          </div>
        </div>
        <PaginationControls
          :key="skuPaginationKey"
          :page="skuPage"
          :page-size="skuPageSize"
          :total="skuDisplayTotal"
          :disabled="loading"
          @change="handleSkuPaginationChange"
        />
      </div>
        </div>
      </div>

      <div v-show="activeSettingsSection === 'templates'" class="sku-template-workspace">
        <div class="template-workspace-stack">
          <div class="config-template-tabs" role="tablist" aria-label="商品配置模板类型">
            <button
              type="button"
              :class="['config-template-tab', { active: activeConfigTemplateSection === 'product-config' }]"
              @click="activeConfigTemplateSection = 'product-config'">
              商品配置模板
            </button>
            <button
              type="button"
              :class="['config-template-tab', { active: activeConfigTemplateSection === 'unit-template' }]"
              @click="activeConfigTemplateSection = 'unit-template'">
              单位模板
            </button>
            <button
              type="button"
              :class="['config-template-tab', { active: activeConfigTemplateSection === 'gradient' }]"
              @click="activeConfigTemplateSection = 'gradient'">
              阶梯价模板
            </button>
            <button
              type="button"
              :class="['config-template-tab', { active: activeConfigTemplateSection === 'category-management' }]"
              @click="activeConfigTemplateSection = 'category-management'">
              商品分类管理
            </button>
          </div>
      <div v-if="activeSettingsSection === 'templates' && activeConfigTemplateSection === 'category-management'" class="category-panel category-drawer-panel category-management-panel">
        <div class="panel-title">
          <span>商品分类 · {{ selectedSkuContextLabel }}</span>
          <div class="panel-actions">
            <button class="toggle-section" type="button" @click="categoryCollapsed = !categoryCollapsed">
              {{ categoryCollapsed ? '展开' : '收起' }}
            </button>
          </div>
        </div>
        <div v-show="!categoryCollapsed">
          <div v-if="selectedCustomerSkuCustomerID" class="customer-copy-panel">
            <label class="checkline switchline">
              <input :checked="customerUsesPublicCategories" type="checkbox" :disabled="publicUsageSaving" @change="savePublicCategoryUsageForCustomer" />
              <span>是否使用公共商品分类</span>
            </label>
          </div>

          <label class="category-search">
            <span>搜索商品分类</span>
            <input v-model.trim="categorySearchQuery" placeholder="搜索产品类型、子类型或 SKU" />
          </label>

          <div class="category-action-pill category-inline-toolbar" aria-label="产品类型操作">
            <button class="category-action-button" type="button" aria-label="新增产品类型" title="新增产品类型" :disabled="loading" @click="createPrimaryCategoryInline">+</button>
            <button
              class="category-action-button danger-toggle"
              :class="{ active: primaryDeleteMode }"
              type="button"
              aria-label="切换删除产品类型"
              title="删除产品类型"
              :disabled="loading || !categoryTreeForSkuContext.length"
              @click="togglePrimaryDeleteMode">
              -
            </button>
          </div>

          <div class="category-scroll-list">
          <div class="category-tree">
            <div
              v-for="primary in visibleCategoryTreeForSkuContext"
              :key="primary.id"
              class="primary-category"
              :data-primary-id="primary.id"
              @dragover.prevent="handlePrimaryCategoryDragOver($event, primary)"
              @drop.prevent="dropCategoryOnCurrentTarget(primary)">
              <div class="category-head primary-category-head">
                <div class="primary-category-left">
                  <div class="category-sort-pill category-sort-buttons" aria-label="产品类型排序">
                    <button class="category-action-button compact" type="button" aria-label="上移产品类型" title="上移产品类型" :disabled="loading || isCategorySearchActive || !canEditCategory(primary) || isFirstPrimaryCategory(primary)" @click.stop="movePrimaryCategory(primary, -1)">↑</button>
                    <button class="category-action-button compact" type="button" aria-label="下移产品类型" title="下移产品类型" :disabled="loading || isCategorySearchActive || !canEditCategory(primary) || isLastPrimaryCategory(primary)" @click.stop="movePrimaryCategory(primary, 1)">↓</button>
                  </div>
                  <div class="category-title-stack">
                  <div class="category-title-row">
                    <form v-if="editingCategoryId === Number(primary.id)" class="category-name-form" @submit.prevent="saveCategoryName(primary)">
                      <input
                        v-model.trim="editingCategoryName"
                        placeholder="产品类型名称"
                        @keyup.enter.prevent="saveCategoryName(primary)"
                        @keyup.esc.prevent="cancelCategoryNameEdit"
                        @blur="saveCategoryName(primary)" />
                    </form>
                    <button v-else class="category-name-button primary-name-button" type="button" :disabled="!canEditCategory(primary)" @click="startCategoryEdit(primary)">
                      <strong>{{ primary.number }}. {{ primary.name }}<small v-if="skuContextCustomerID">（{{ categoryStateLabel(primary) }}）</small></strong>
                    </button>
                    <div v-if="!canEditCategory(primary) && skuContextCustomerID" class="category-actions">
                      <button class="text-button" type="button" @click="deriveCategoryTemplate(primary)">复制为客户分类</button>
                    </div>
                  </div>
                  <div v-if="canEditCategory(primary)" class="category-action-pill category-child-toolbar" aria-label="产品子类型操作">
                    <button class="category-action-button compact" type="button" aria-label="新增产品子类型" title="新增产品子类型" :disabled="loading" @click="createSecondaryCategoryInline(primary)">+</button>
                    <button
                      class="category-action-button compact danger-toggle"
                      :class="{ active: secondaryDeleteModeFor === Number(primary.id) }"
                      type="button"
                      aria-label="切换删除产品子类型"
                      title="删除产品子类型"
                      :disabled="loading || !primary.children.length"
                      @click="toggleSecondaryDeleteMode(primary)">
                      -
                    </button>
                  </div>
                  </div>
                </div>
                <div class="category-row-actions primary-category-right">
                  <button
                    class="category-collapse-button"
                    type="button"
                    :aria-expanded="!isPrimaryCategoryCollapsed(primary)"
                    :title="isPrimaryCategoryCollapsed(primary) ? '展开产品类型' : '折叠产品类型'"
                    @click.stop="togglePrimaryCategoryCollapse(primary)">
                    {{ isPrimaryCategoryCollapsed(primary) ? '›' : '⌄' }}
                  </button>
                  <button v-if="primaryDeleteMode && canEditCategory(primary)" class="category-delete-button" type="button" aria-label="删除产品类型" title="删除产品类型" @click.stop="deleteCategory(primary)">-</button>
                </div>
              </div>

              <template v-if="!isPrimaryCategoryCollapsed(primary)">
                <div
                  v-if="!isCategorySearchActive"
                  class="category-drop-line"
                  :class="{ active: isCategoryDropTarget(primary, 1) }"
                  @dragover.prevent.stop="setCategoryDropTarget(primary, 1)"
                  @drop.prevent.stop="dropCategoryAtPosition(primary, 1)">
                </div>

                <template v-for="(secondary, index) in primary.children" :key="secondary.id">
                  <div
                    class="secondary-category"
                    :class="{ dragging: isDraggingCategory(secondary), 'pointer-dragging': isPointerDraggingCategory(secondary) }"
                    :data-secondary-id="secondary.id"
                    :data-secondary-position="index + 1"
                    @pointerdown="startCategoryPointerDrag($event, primary, index + 1, secondary)"
                    @dragenter.prevent.stop="handleSecondaryCategoryDragOver($event, primary, index + 1)"
                    @dragover.prevent.stop="handleSecondaryCategoryDragOver($event, primary, index + 1)"
                    @drop.prevent.stop="dropCategoryOrProductOnSecondary(primary, index + 1, secondary)">
                    <div class="secondary-head">
                      <button
                        class="category-collapse-button secondary-collapse"
                        type="button"
                        :aria-expanded="!isSecondaryCategoryCollapsed(secondary)"
                        :title="isSecondaryCategoryCollapsed(secondary) ? '展开产品子类型' : '折叠产品子类型'"
                        @pointerdown.stop
                        @click.stop="toggleSecondaryCategoryCollapse(secondary)">
                        {{ isSecondaryCategoryCollapsed(secondary) ? '›' : '⌄' }}
                      </button>
                      <form v-if="editingCategoryId === Number(secondary.id)" class="category-name-form secondary-name-form" @submit.prevent="saveCategoryName(secondary)" @pointerdown.stop>
                        <input
                          v-model.trim="editingCategoryName"
                          placeholder="产品子类型名称"
                          @keyup.enter.prevent="saveCategoryName(secondary)"
                          @keyup.esc.prevent="cancelCategoryNameEdit"
                          @blur="saveCategoryName(secondary)" />
                      </form>
                      <button v-else class="category-name-button secondary-name-button" type="button" :disabled="!canEditCategory(secondary)" @click.stop="startCategoryEdit(secondary)">
                        <span>{{ secondary.number }}</span>
                        <b>{{ secondary.name }}</b>
                      </button>
                      <small v-if="skuContextCustomerID">{{ categoryStateLabel(secondary) }}</small>
                      <small>{{ secondary.products.length }} 款</small>
                      <select
                        class="template-select"
                        :value="secondary.product_config_template_id || 0"
                        :disabled="!canEditCategory(secondary)"
                        @pointerdown.stop
                        @change.stop="bindProductConfigTemplateToSubtype(secondary, $event.target.value)">
                        <option value="0">未绑定商品配置</option>
                        <option v-for="config in activeProductConfigTemplates" :key="config.id" :value="config.id">
                          {{ config.name }} · {{ config.quote_unit || 'kg' }}/{{ config.order_unit || config.quote_unit || 'kg' }}
                        </option>
                      </select>
                      <div class="secondary-category-actions">
                        <button v-if="!canEditCategory(secondary) && skuContextCustomerID" class="text-button" type="button" @click="deriveCategoryTemplate(secondary)">复制为客户分类</button>
                        <button v-if="secondaryDeleteModeFor === Number(primary.id) && canEditCategory(secondary)" class="category-delete-button" type="button" aria-label="删除产品子类型" title="删除产品子类型" @click.stop="deleteCategory(secondary)">-</button>
                      </div>
                    </div>
                    <div v-show="!isSecondaryCategoryCollapsed(secondary)" class="product-chip-list">
                      <span
                        v-for="product in secondary.products"
                        :key="product.id"
                        class="product-chip"
                        :draggable="canDragSkuRow(product)"
                        @dragstart.stop="startProductDrag(product)"
                        @dragend="scheduleClearDrag">
                        {{ product.number }}. {{ product.name }}
                      </span>
                    </div>
                  </div>
                  <div
                    v-if="!isCategorySearchActive"
                    class="category-drop-line"
                    :class="{ active: isCategoryDropTarget(primary, index + 2) }"
                    @dragover.prevent.stop="setCategoryDropTarget(primary, index + 2)"
                    @drop.prevent.stop="dropCategoryAtPosition(primary, index + 2)">
                  </div>
                </template>
              </template>
            </div>
          </div>
          <p v-if="!visibleCategoryTreeForSkuContext.length" class="muted category-empty">没有匹配的商品分类</p>

          <div class="uncategorized" @dragover.prevent @drop="dropProductOnSecondary({ id: 0 })">
            <div class="sub-title">停车场（待归类 SKU）</div>
            <p class="muted">未选择产品子类型创建的 SKU 会先进入停车场；拖入产品子类型后才参与产品价格表生成。</p>
            <span
              v-for="product in uncategorizedProducts"
              :key="product.id"
              class="product-chip"
              :draggable="canDragSkuRow(product)"
              @dragstart="startProductDrag(product)"
              @dragend="scheduleClearDrag">
              {{ product.name }}
            </span>
            <p v-if="!uncategorizedProducts.length" class="muted">停车场暂无 SKU</p>
          </div>
          </div>
        </div>
      </div>
      <div v-show="activeConfigTemplateSection === 'gradient'" class="panel gradient-template-panel gradient-template-pane">
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
              v-for="template in gradientTemplates"
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
            <p v-if="!gradientTemplates.length" class="muted">暂无梯度模板</p>
          </div>
          <form class="template-editor" @submit.prevent="saveGradientTemplate">
            <p v-if="!canEditCurrentTemplate" class="muted">公共模板需复制到客户后修改。</p>
            <div class="template-editor-grid">
              <label>
                <span>模板名称</span>
                <input v-model.trim="templateForm.name" :disabled="!canEditCurrentTemplate" placeholder="如 工厂量单模板" />
              </label>
              <label>
                <span>单位模板</span>
                <select v-model.number="templateForm.unit_template_id" :disabled="!canEditCurrentTemplate" @change="syncGradientDisplayUnitFromUnitTemplate">
                  <option value="0">未绑定单位模板</option>
                  <option v-for="unitTemplate in activeProductUnitTemplates" :key="unitTemplate.id" :value="unitTemplate.id">{{ productUnitTemplateSummary(unitTemplate) }}</option>
                </select>
              </label>
              <label>
                <span>展示单位</span>
                <select v-model="templateForm.display_unit" :disabled="!canEditCurrentTemplate">
                  <option v-for="unit in gradientDisplayUnitOptions" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                </select>
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

      <div v-show="activeConfigTemplateSection === 'unit-template'" class="panel unit-template-panel unit-template-pane">
        <div class="panel-title">
          <span>单位模板</span>
          <button class="secondary compact-action" type="button" @click="openGlobalUnitDictionaryDrawer">全局单位字典</button>
        </div>
        <p class="muted unit-template-note">基础单位在“全局设置”维护；这里配置成品库存单位、报价单位、录单单位之间的换算模板。</p>
        <div class="unit-template-layout">
          <section class="unit-template-card unit-template-list-panel">
            <div class="field-group-head">
              <strong>单位换算模板</strong>
              <small>点击模板在右侧编辑。</small>
            </div>
            <div class="template-list compact-template-list">
              <div
                v-for="unitTemplate in productUnitTemplates"
                :key="unitTemplate.id"
                :class="['template-row', { active: Number(unitTemplate.id) === Number(productUnitTemplateForm.id), inactive: unitTemplate.active === false }]">
                <button class="template-row-main" type="button" @click="startProductUnitTemplateEdit(unitTemplate)">
                  <strong>{{ unitTemplate.name }}</strong>
                  <small>{{ productUnitTemplateSummary(unitTemplate) }}</small>
                </button>
              </div>
              <p v-if="!productUnitTemplates.length" class="muted">暂无单位模板</p>
            </div>
          </section>
          <section class="unit-template-card unit-template-editor-panel">
            <div class="field-group-head">
              <div class="field-group-copy">
                <strong>{{ productUnitTemplateForm.id ? '编辑单位模板' : '新增单位模板' }}</strong>
                <small>保存后刷新列表并回到空白表单。</small>
              </div>
              <button class="secondary compact-action" type="button" @click="resetProductUnitTemplateForm">新增单位模板</button>
            </div>
            <form class="unit-template-form" @submit.prevent="saveProductUnitTemplate">
              <label class="wide-field">
                <span>模板名称</span>
                <input v-model.trim="productUnitTemplateForm.name" placeholder="盒装200g" />
              </label>
              <div class="template-editor-grid">
                <label>
                  <span>成品库存单位</span>
                  <select v-model="productUnitTemplateForm.inventory_unit">
                    <option v-for="unit in activeProductUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option>
                  </select>
                </label>
                <label>
                  <span>报价单位</span>
                  <select v-model="productUnitTemplateForm.quote_unit">
                    <option v-for="unit in activeProductUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option>
                  </select>
                </label>
                <label>
                  <span>录单单位</span>
                  <select v-model="productUnitTemplateForm.order_unit">
                    <option v-for="unit in activeProductUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option>
                  </select>
                </label>
                <label class="checkline">
                  <input v-model="productUnitTemplateForm.integer_unit" type="checkbox" />
                  <span>整数单位（录单）</span>
                </label>
              </div>
              <div class="unit-conversion-editor">
                <div class="field-group-head">
                  <span>单位换算</span>
                  <button class="secondary compact-action" type="button" @click="addUnitConversionRow(productUnitTemplateForm)">新增换算</button>
                </div>
                <div v-for="(row, rowIndex) in productUnitTemplateForm.unit_conversion_rows" :key="`unit-template-conversion-${rowIndex}`" class="unit-conversion-row">
                  <input v-model.number="row.from_qty" type="number" min="0.0001" step="0.0001" />
                  <select v-model="row.from_unit">
                    <option v-for="unit in activeProductUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option>
                  </select>
                  <span>=</span>
                  <input v-model.number="row.to_qty" type="number" min="0.0001" step="0.0001" />
                  <select v-model="row.to_unit">
                    <option v-for="unit in activeProductUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option>
                  </select>
                  <button class="text-button danger-text" type="button" @click="removeUnitConversionRow(productUnitTemplateForm, rowIndex)">删除</button>
                </div>
                <small v-if="!productUnitTemplateForm.unit_conversion_rows.length">例如 1 盒 = 0.2 kg；模板保存后可被商品配置和阶梯价引用。</small>
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

      <div v-show="activeConfigTemplateSection === 'product-config'" class="panel product-config-panel product-config-template-pane">
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
              <span>阶梯价模板</span>
              <select v-model.number="productConfigTemplateForm.gradient_template_id" :disabled="!canEditCurrentProductConfigTemplate">
                <option value="0">未绑定模板</option>
                <option v-for="template in activeGradientTemplates" :key="template.id" :value="template.id">{{ template.name }} · {{ gradientDisplayUnitLabel(template.display_unit) }}</option>
              </select>
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
              <div class="field-group-title">单位模板</div>
              <label>
                <span>引用模板</span>
                <select v-model.number="productConfigTemplateForm.unit_template_id" :disabled="!canEditCurrentProductConfigTemplate">
                  <option value="0">请选择单位模板</option>
                  <option v-for="unitTemplate in activeProductUnitTemplates" :key="unitTemplate.id" :value="unitTemplate.id">{{ productUnitTemplateSummary(unitTemplate) }}</option>
                </select>
              </label>
              <small>{{ productUnitTemplateSummary(selectedProductConfigUnitTemplate) }}</small>
              <small class="unit-impact-help">单位模板会影响产品价格表单位、录单默认单位和库存/生产折算；已发布价格表和历史订单不会被回改。</small>
            </div>
            <div class="rule-config-block special-attrs-schema-editor">
              <div class="field-group-title">
                <span title="特殊KV定义">产品信息字段（特殊属性KV）</span>
                <button class="secondary compact-action" type="button" :disabled="!canEditCurrentProductConfigTemplate" @click="addSpecialAttrSchemaRow">新增KV</button>
              </div>
              <p class="muted special-attrs-config-guide">配置入口：商品配置 → 商品配置模板 → 产品信息字段。这里定义字段，SKU列表特殊属性列填写具体值；勾选展示到价格表/PDF后，价格表页面、发布快照和 PDF 均展示。</p>
              <div v-for="(attr, index) in productConfigTemplateForm.special_attrs_schema_rows" :key="`special-attr-${index}`" class="special-attr-schema-row">
                <label>
                  <span>Key</span>
                  <input v-model.trim="attr.key" :disabled="!canEditCurrentProductConfigTemplate" placeholder="如 roast_level" />
                </label>
                <label>
                  <span>名称</span>
                  <input v-model.trim="attr.label" :disabled="!canEditCurrentProductConfigTemplate" placeholder="如 烘焙度" />
                </label>
                <label>
                  <span>类型</span>
                  <select v-model="attr.value_type" :disabled="!canEditCurrentProductConfigTemplate" @change="handleSpecialAttrSchemaTypeChange(attr)">
                    <option value="text">文本</option>
                    <option value="select">下拉</option>
                    <option value="number">数字</option>
                  </select>
                </label>
                <label class="wide-field">
                  <span>下拉选项</span>
                  <textarea v-model.trim="attr.options_text" aria-label="下拉选项" :disabled="!canEditCurrentProductConfigTemplate || attr.value_type !== 'select'" placeholder="每行一个选项，也支持用逗号分隔，例如：浅烘，中烘，中深烘，深烘"></textarea>
                </label>
                <label class="checkline">
                  <input v-model="attr.required" :disabled="!canEditCurrentProductConfigTemplate" type="checkbox" />
                  <span>必填</span>
                </label>
                <label class="checkline">
                  <input v-model="attr.show_in_price_list" :disabled="!canEditCurrentProductConfigTemplate" type="checkbox" />
                  <span>展示到价格表/PDF</span>
                </label>
                <button class="danger compact-action" type="button" :disabled="!canEditCurrentProductConfigTemplate" @click="removeSpecialAttrSchemaRow(index)">删除</button>
              </div>
              <p v-if="!productConfigTemplateForm.special_attrs_schema_rows.length" class="muted">暂无特殊KV。</p>
            </div>
            <div class="form-actions">
              <button class="primary" type="submit" :disabled="productConfigSaving || !canEditCurrentProductConfigTemplate">保存商品配置</button>
              <button v-if="productConfigTemplateForm.id" class="secondary" type="button" :disabled="productConfigSaving || !canEditCurrentProductConfigTemplate" @click="deactivateProductConfigTemplate(productConfigTemplateForm.id)">停用配置</button>
            </div>
          </form>
        </div>
      </div>
        </div>
      </div>
    </section>

    <div v-if="productDrawerOpen" class="settings-drawer-mask" @click.self="closeProductDrawer">
      <aside class="settings-drawer product-editor-drawer" aria-label="新增SKU">
        <div class="drawer-head">
          <div>
            <h3>新增SKU</h3>
            <p>当前归属：{{ selectedSkuContextLabel }}。只填写名称、备注、产品类别和产品子类型。</p>
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
            <label>
              <span>产品类别</span>
              <select v-model.number="skuForm.product_type_category_id" @change="handleProductTypeCategoryChange(skuForm)">
                <option value="0">选择产品类别</option>
                <option v-for="category in productTypeCategoryOptions" :key="category.id" :value="category.id">{{ category.name }}</option>
              </select>
            </label>
            <label>
              <span>产品子类型</span>
              <select v-model.number="skuForm.product_subtype_category_id" @change="syncProductTypeFromProductSubtype(skuForm)">
                <option value="0">不选择，进停车场</option>
                <option v-for="category in productSubtypeCategoryOptions(skuForm.product_type_category_id)" :key="category.id" :value="category.id">{{ category.name }}</option>
              </select>
              <small>只有选中产品子类型才会挂入分类；未选会进入停车场。</small>
            </label>
            <div class="form-actions">
              <button class="primary" type="submit" :disabled="skuSaving">新增SKU</button>
            </div>
          </form>
        </div>
      </aside>
    </div>

    <div v-if="skuCopyDrawerOpen" class="settings-drawer-mask" @click.self="closeSkuCopyDrawer">
      <aside class="settings-drawer sku-copy-drawer" aria-label="SKU复制">
        <div class="drawer-head">
          <div>
            <h3>SKU复制</h3>
            <p>从公共 SKU 或其他客户复制到当前归属；同名 SKU 会覆盖资料但保留目标 SKU ID。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeSkuCopyDrawer">关闭</button>
        </div>
        <div class="drawer-body">
          <label class="sku-copy-source">
            <span>复制来源</span>
            <select v-model.number="copySourceCustomerID" @change="loadSkuCopyOptions">
              <option
                v-for="source in skuCopySourceOptions"
                :key="source.id"
                :value="source.id">
                {{ source.name }}
              </option>
            </select>
          </label>

          <section class="pdf-picker productSelection sku-copy-selection">
            <div class="picker-head">
              <label class="check-line">
                <input type="checkbox" :checked="allCopySkusSelected" :disabled="!skuCopyTotalCount" @change="setAllCopySkus($event.target.checked)" />
                <strong>选择分类和产品</strong>
              </label>
              <span>{{ copySelectedCount }}/{{ skuCopyTotalCount }} 款</span>
              <div class="picker-actions">
                <button class="secondary compact-action" type="button" :disabled="!skuCopyTotalCount" @click="setAllCopySkus(true)">全选</button>
                <button class="secondary compact-action" type="button" :disabled="!copySelectedCount" @click="setAllCopySkus(false)">清空</button>
              </div>
            </div>
            <div class="product-picker-list">
              <div v-for="group in skuCopyGroups" :key="`copy-type-${group.id}`" class="product-picker-category">
                <div class="product-picker-category-head">
                  <label class="check-line">
                    <input type="checkbox" :checked="isCopyTypeSelected(group)" @change="toggleCopyType(group, $event.target.checked)" />
                    <strong>{{ group.name }}</strong>
                  </label>
                  <span>{{ selectedCopyCountForType(group) }}/{{ copyCountForType(group) }} 款</span>
                </div>
                <div v-for="subtype in group.children" :key="`copy-subtype-${subtype.id}`" class="product-picker-subcategory">
                  <div class="product-picker-category-head subtype-head">
                    <label class="check-line">
                      <input type="checkbox" :checked="isCopySubtypeSelected(subtype)" @change="toggleCopySubtype(subtype, $event.target.checked)" />
                      <span>{{ subtype.name }}</span>
                    </label>
                    <span>{{ selectedCopyCountForSubtype(subtype) }}/{{ copyCountForSubtype(subtype) }} 款</span>
                  </div>
                  <label
                    v-for="sku in subtype.products"
                    :key="`copy-sku-${sku.id}`"
                    class="product-picker-row check-line"
                    :class="{ inactive: sku.copy_state === 'inactive', overwrite: sku.copy_state === 'will_overwrite' }">
                    <input type="checkbox" :checked="isCopySkuSelected(sku)" :disabled="sku.copy_state === 'inactive'" @change="toggleCopySku(sku, $event.target.checked)" />
                    <span>{{ sku.name }}</span>
                    <small>{{ sku.copy_state === 'will_overwrite' ? '覆盖同名' : (sku.copy_state === 'inactive' ? '已停用' : '新增') }}</small>
                  </label>
                </div>
              </div>
              <p v-if="!skuCopyGroups.length" class="muted">当前来源暂无可复制 SKU。</p>
            </div>
          </section>
        </div>
        <div class="drawer-footer sku-copy-footer">
          <span>已选 {{ copySelectedCount }} 款</span>
          <button class="primary" type="button" :disabled="skuCopySaving || !copySelectedCount" @click="copySelectedSkus">复制SKU</button>
        </div>
      </aside>
    </div>

    <div v-if="globalUnitDrawerOpen" class="settings-drawer-mask" @click.self="closeGlobalUnitDictionaryDrawer">
      <aside class="settings-drawer global-unit-dictionary-drawer" aria-label="全局单位字典设置">
        <div class="drawer-head">
          <div>
            <h3>全局单位字典</h3>
            <p>维护 kg、盒、箱等基础单位；单位模板引用这些单位配置换算关系。</p>
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
                v-for="unit in productUnitDefinitions"
                :key="unit.code"
                class="unit-chip global-unit-chip"
                :class="{ inactive: unit.active === false }"
                type="button"
                @click="startGlobalUnitDefinitionEdit(unit)">
                <strong>{{ unit.name || unit.code }}</strong>
                <small>{{ unit.code }} · {{ unitTypeLabel(unit.unit_type) }} · {{ unit.allow_decimal ? '允许小数' : '整数优先' }}</small>
              </button>
              <p v-if="!productUnitDefinitions.length" class="muted">暂无单位，直接在右侧表单填写并保存。</p>
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
                <button class="primary" type="submit" :disabled="globalUnitSaving || loading">
                  {{ globalUnitSaving ? '保存中' : (globalUnitEditingCode ? '保存' : '新增') }}
                </button>
              </div>
            </form>
          </section>
        </div>
      </aside>
    </div>

  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import PaginationControls from '../components/PaginationControls.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
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
  buildCustomerProductRuleBindingPayload,
  buildCustomerProductRuleOverridePayload,
  buildCustomerProductRuleTemplatePayload,
  buildCustomProductCreatePayload,
  buildProductCategoryConfigPayload,
  buildProductConfigTemplatePayload,
  buildProductUnitDefinitionPayload,
  buildProductUnitTemplatePayload,
  buildProductBasicsPayload,
  buildProductBomURL,
  buildProductCreatePayload,
  buildAssignCategoryPayload,
  buildSkuCopyPayload,
  buildSkuCreatePayload,
  buildSkuContextCategoryTree,
  categoryBelongsToSkuContext as categoryBelongsToContext,
  categoryDisplayState,
  customerSkuCustomerOptions,
  gradientTemplateBelongsToSkuContext,
  greenBeanTypeOptions,
  integerUnitModeOptions,
  inferProductKindFromProductTypeCategory,
  isPublicReferenceRow,
  nextSkuContextCustomerID,
  normalizeVisibleSkuFilters,
  normalizedProductKind,
  priceListRuleFormFromJSON,
  priceListRulePricingModeOptions,
  priceListRuleRoundingOptions,
  productBelongsToSkuContext as productBelongsToContext,
  productConfigTemplateBelongsToSkuContext,
  productDisplayState,
  productKindSupportsBomParams,
  productSubtypeCategoryOptionsForType,
  roastedBomProductOptions,
  specialAttrSchemaRowsFromJSON,
  specialAttrValuesFromJSON,
  skuTableState,
  sortRowsForCustomerSkuPriority,
  skuTypeLabel,
  skuTypeOptions,
  unitConversionRowsFromJSON,
  unitRuleFormFromJSON,
} from '../lib/product-settings'
import { normalizePageSize } from '../lib/pagination'
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'

const props = defineProps({
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})
const SKU_SETTINGS_FORM_DRAFT_SCOPE = FORM_DRAFT_SCOPES.skuSettings
let restoringProductSettingsDraft = false

const categories = ref([])
const products = ref([])
const gradientTemplates = ref([])
const productConfigTemplates = ref([])
const productUnitDefinitions = ref([])
const productUnitTemplates = ref([])
const customerPublicUsages = ref([])
const customerProductRuleTemplates = ref([])
const customerProductRuleOverrides = ref([])
const customerProductRuleBindings = ref([])
const customers = ref([])
const loading = ref(false)
const skuSaving = ref(false)
const productSaving = skuSaving
const customSaving = skuSaving
const skuCopyLoading = ref(false)
const skuCopySaving = ref(false)
const templateSaving = ref(false)
const productConfigSaving = ref(false)
const productUnitSaving = ref(false)
const globalUnitSaving = ref(false)
const customerRuleSaving = ref(false)
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
const productDrawerOpen = ref(false)
const skuCopyDrawerOpen = ref(false)
const globalUnitDrawerOpen = ref(false)
const categorySearchQuery = ref('')
const primaryDeleteMode = ref(false)
const secondaryDeleteModeFor = ref(0)
const selectedCustomerSkuCustomerID = ref(0)
const selectedProductIds = ref([])
const skuFilters = ref(defaultSkuFilters())
const skuPage = ref(1)
const skuPageSize = ref(10)
const publicUsageSaving = ref(false)
const skuForm = ref(defaultSkuForm())
const productForm = skuForm
const customForm = ref(defaultCustomForm())
const skuCopyOptions = ref(defaultSkuCopyOptions())
const copySourceCustomerID = ref(0)
const copySkuSelection = ref([])
const templateForm = ref(defaultGradientTemplateForm())
const productUnitTemplateForm = ref(defaultProductUnitTemplateForm())
const globalUnitForm = ref(defaultProductUnitDefinitionForm())
const globalUnitEditingCode = ref('')
const customerRuleTemplateForm = ref(defaultCustomerProductRuleTemplateForm())
const customerRuleOverrideForm = ref(defaultCustomerProductRuleOverrideForm())

const skuContextCustomerID = computed(() => Number(selectedCustomerSkuCustomerID.value || 0))
const productConfigTemplateForm = ref(defaultProductConfigTemplateForm())
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && Number(props.customerContextId || 0) > 0)
const selectedSkuContextLabel = computed(() => {
  const customerID = skuContextCustomerID.value
  if (!customerID) return '公共SKU'
  return `${customerName(customerID) || `客户 #${customerID}`} SKU`
})
const flatPublicCategories = computed(() => flattenCategoryNodes(categories.value).filter((category) => Number(category.customer_id || 0) === 0))
const flatCustomerCategories = computed(() => flattenCategoryNodes(categories.value).filter((category) => Number(category.customer_id || 0) === skuContextCustomerID.value))
const publicProducts = computed(() => products.value.filter((product) => Number(product.customer_id || 0) === 0))
const customerProductsForContext = computed(() => products.value.filter((product) => Number(product.customer_id || 0) === skuContextCustomerID.value))
const customerGradientTemplatesForContext = computed(() => gradientTemplates.value.filter((template) => Number(template.customer_id || 0) === skuContextCustomerID.value))
const customerProductConfigTemplatesForContext = computed(() => productConfigTemplates.value.filter((template) => Number(template.customer_id || 0) === skuContextCustomerID.value))
const selectedCustomerPublicUsage = computed(() => {
  const customerID = skuContextCustomerID.value
  return customerPublicUsages.value.find((row) => Number(row.customer_id || 0) === customerID) || {
    customer_id: customerID,
    use_public_sku: false,
    use_public_categories: false,
    use_public_gradient_templates: false,
  }
})
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
const isCategorySearchActive = computed(() => Boolean(categorySearchQuery.value.trim()))
const visibleCategoryTreeForSkuContext = computed(() => filterCategoryTreeByQuery(categoryTreeForSkuContext.value, categorySearchQuery.value))
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
  return (sourceProducts || [])
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
const publicSkuRows = computed(() => sortRowsForCustomerSkuPriority(
  skuTableRowsFromFlatProducts(products.value, categories.value, (product) => Number(product.customer_id || 0) === 0),
  0,
))
const customerSkuCustomers = computed(() => customerSkuCustomerOptions(customers.value))
const skuCopySourceCustomers = computed(() => customerSkuCustomers.value.filter((customer) => Number(customer.id || 0) !== skuContextCustomerID.value))
const skuCopySourceOptions = computed(() => {
  const targetCustomerID = skuContextCustomerID.value
  const options = []
  if (targetCustomerID !== 0) {
    options.push({ id: 0, name: '公共SKU' })
  }
  for (const customer of skuCopySourceCustomers.value) {
    options.push({ id: Number(customer.id || 0), name: customer.name })
  }
  return options
})
const customerSkuRows = computed(() => {
  const customerID = skuContextCustomerID.value
  return sortRowsForCustomerSkuPriority(
    skuTableRowsFromFlatProducts(products.value, categories.value, (product) => customerID > 0 && skuContextProductFilter(product)),
    customerID,
  )
})
const currentSkuSourceRows = computed(() => (
  skuContextCustomerID.value > 0 ? customerSkuRows.value : publicSkuRows.value
).slice())
const skuVisibleTableState = computed(() => skuTableState(currentSkuSourceRows.value, skuFilters.value, {
  page: skuPage.value,
  pageSize: skuPageSize.value,
}))
const normalizedSkuFilters = computed(() => skuVisibleTableState.value.filters)
const skuDisplayTotal = computed(() => skuVisibleTableState.value.total)
const displaySkuRows = computed(() => skuVisibleTableState.value.rows)
const skuPrimaryCategoryOptions = computed(() => skuVisibleTableState.value.primaryOptions)
const skuSecondaryCategoryOptions = computed(() => skuVisibleTableState.value.secondaryOptions)
const hasActiveSkuFilters = computed(() => Boolean(
  normalizedSkuFilters.value.query
    || normalizedSkuFilters.value.primaryCategory
    || normalizedSkuFilters.value.secondaryCategory,
))
const skuDisplayKey = computed(() => [
  skuContextCustomerID.value,
  skuDisplayTotal.value,
  skuPage.value,
  skuPageSize.value,
  normalizedSkuFilters.value.query || '',
  normalizedSkuFilters.value.primaryCategory || '',
  normalizedSkuFilters.value.secondaryCategory || '',
].join(':'))
const skuTableKey = computed(() => `${skuDisplayKey.value}:table`)
const skuPaginationKey = computed(() => `${skuDisplayKey.value}:pagination`)
const editableDisplaySkuRows = computed(() => displaySkuRows.value.filter(canEditSkuRow))
const allProductRowsSelected = computed(() => editableDisplaySkuRows.value.length > 0 && editableDisplaySkuRows.value.every((row) => selectedProductIds.value.includes(Number(row.id))))
const activeGradientTemplates = computed(() => gradientTemplates.value
  .filter((template) => template.active !== false)
  .filter((template) => gradientTemplateBelongsToSkuContext(template, {
    customerID: skuContextCustomerID.value,
    usePublicGradientTemplates: customerUsesPublicGradientTemplates.value,
    customerTemplates: customerGradientTemplatesForContext.value,
  })))
const activeProductUnitDefinitions = computed(() => productUnitDefinitions.value.filter((unit) => unit.active !== false))
const activeProductUnitTemplates = computed(() => productUnitTemplates.value.filter((template) => template.active !== false))
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
const productConfigTemplatesForContext = computed(() => productConfigTemplates.value
  .filter((template) => template.active !== false)
  .filter((template) => productConfigTemplateBelongsToSkuContext(template, {
    customerID: skuContextCustomerID.value,
    customerTemplates: customerProductConfigTemplatesForContext.value,
  })))
const selectedProductConfigUnitTemplate = computed(() => findProductUnitTemplate(productConfigTemplateForm.value.unit_template_id))
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
const skuCopyGroups = computed(() => skuCopyOptions.value?.groups || [])
const copyableSkuIDs = computed(() => {
  const ids = []
  for (const group of skuCopyGroups.value) {
    for (const subtype of group.children || []) {
      for (const sku of subtype.products || []) {
        if (sku.copy_state === 'inactive') continue
        const id = Number(sku.id || 0)
        if (id) ids.push(id)
      }
    }
  }
  return ids
})
const skuCopyTotalCount = computed(() => copyableSkuIDs.value.length)
const copySelectedCount = computed(() => copySkuSelection.value.length)
const allCopySkusSelected = computed(() => skuCopyTotalCount.value > 0 && copyableSkuIDs.value.every((id) => copySkuSelection.value.includes(id)))
const publicRoastedBomProducts = computed(() => roastedBomProductOptions(products.value))
const customRoastedBomProducts = computed(() => roastedBomProductOptions(products.value, {
  customerID: selectedCustomerSkuCustomerID.value,
}))

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
  const tableState = skuVisibleTableState.value
  if (Number(skuPage.value || 1) !== tableState.page) {
    skuPage.value = tableState.page
  }
}

function defaultSkuForm() {
  return {
    name: '',
    remark: '',
    product_type_category_id: 0,
    product_subtype_category_id: 0,
    special_attr_values: {},
    active: true,
  }
}

function defaultSkuCopyOptions() {
  return {
    title: '选择分类和产品',
    target_customer_id: 0,
    source_customer_id: 0,
    total_count: 0,
    groups: [],
  }
}

function defaultProductForm() {
  return {
    name: '',
    product_type_category_id: 0,
    product_subtype_category_id: 0,
    product_kind: 'roasted',
    remark: '',
    green_bean_type: 'single_origin',
    green_bean_bom_product_id: 0,
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    special_attr_values: {},
    yield_percent: 80,
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
    green_bean_type: 'single_origin',
    green_bean_bom_product_id: 0,
    drip_bag_grams: 10,
    drip_box_bag_count: 10,
    special_attr_values: {},
    yield_percent: 80,
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
    special_attrs_schema_json: template.special_attrs_schema_json || '[]',
    special_attrs_schema_rows: specialAttrSchemaRowsFromJSON(template.special_attrs_schema_json || '[]'),
    inventory_unit: inventoryUnit,
    quote_unit: quoteUnit,
    order_unit: template.order_unit || quoteUnit,
    unit_conversion_json: template.unit_conversion_json || '{}',
    unit_conversion_rows: unitConversionRowsFromJSON(template.unit_conversion_json || '{}'),
    integer_unit: Boolean(template.integer_unit),
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
  const quoteUnit = template.quote_unit || inventoryUnit
  return {
    id: Number(template.id || 0),
    name: template.name || '',
    inventory_unit: inventoryUnit,
    quote_unit: quoteUnit,
    order_unit: template.order_unit || quoteUnit,
    unit_conversion_json: template.unit_conversion_json || '{}',
    unit_conversion_rows: unitConversionRowsFromJSON(template.unit_conversion_json || '{}'),
    integer_unit: Boolean(template.integer_unit),
    active: template.active !== false,
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
    skuForm: skuForm.value,
    templateForm: templateForm.value,
    productConfigTemplateForm: productConfigTemplateForm.value,
    productUnitTemplateForm: productUnitTemplateForm.value,
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
    skuPage: skuPage.value,
    skuPageSize: skuPageSize.value,
  })
}

watch([
  publicSkuRows,
  customerSkuRows,
  skuFilters,
  skuPage,
  skuPageSize,
  selectedCustomerSkuCustomerID,
], syncVisibleSkuTableState, { deep: true, immediate: true })

async function restoreProductSettingsDraft() {
  const draft = readFormDraft(productSettingsDraftKey())
  if (!draft) return
  restoringProductSettingsDraft = true
  selectedCustomerSkuCustomerID.value = Number(draft.selectedCustomerSkuCustomerID || 0)
  syncSelectedCustomerSkuCustomer()
  applyWorkspaceCustomerContext()
  skuForm.value = { ...defaultSkuForm(), ...(draft.skuForm || draft.productForm || {}) }
  templateForm.value = normalizeGradientTemplate(draft.templateForm || defaultGradientTemplateForm())
  productConfigTemplateForm.value = defaultProductConfigTemplateForm(draft.productConfigTemplateForm || {})
  productUnitTemplateForm.value = defaultProductUnitTemplateForm(draft.productUnitTemplateForm || {})
  editingCategoryId.value = Number(draft.editingCategoryId || 0)
  editingCategoryName.value = draft.editingCategoryName || ''
  categoryCollapsed.value = Boolean(draft.categoryCollapsed)
  collapsedPrimaryCategoryIds.value = normalizeCategoryIdList(draft.collapsedPrimaryCategoryIds)
  collapsedSecondaryCategoryIds.value = normalizeCategoryIdList(draft.collapsedSecondaryCategoryIds)
  productsCollapsed.value = Boolean(draft.productsCollapsed)
  activeSettingsSection.value = ['master', 'templates'].includes(draft.activeSettingsSection) ? draft.activeSettingsSection : 'master'
  activeConfigTemplateSection.value = ['product-config', 'unit-template', 'gradient', 'category-management'].includes(draft.activeConfigTemplateSection) ? draft.activeConfigTemplateSection : 'product-config'
  categorySearchQuery.value = draft.categorySearchQuery || ''
  skuFilters.value = normalizeSkuFiltersForCurrentRows(draft.skuFilters || {})
  skuPage.value = Number(draft.skuPage || 1)
  skuPageSize.value = normalizePageSize(draft.skuPageSize)
  ensureProductTypeCategorySelected(skuForm.value)
  await nextTick()
  syncVisibleSkuTableState()
  restoringProductSettingsDraft = false
}

function decorateProduct(product) {
  const yieldRate = Number(product.yield_rate || 0.8)
  const productKind = normalizedProductKind(product)
  const marginRateOverride = normalizeBackendMarginRateOverride(product.margin_rate_override)
  return {
    ...product,
    remark: product.remark || '',
    product_kind: productKind,
    green_bean_type: product.green_bean_type || 'single_origin',
    green_bean_bom_product_id: Number(product.green_bean_bom_product_id || 0),
    drip_bag_grams: Number(product.drip_bag_grams || 10),
    drip_box_bag_count: Number(product.drip_box_bag_count || 10),
    roast_level: product.roast_level || '',
    special_attrs_json: product.special_attrs_json || '{}',
    special_attr_values: {
      ...(product.roast_level ? { roast_level: product.roast_level } : {}),
      ...specialAttrValuesFromJSON(product.special_attrs_json || '{}'),
    },
    yield_rate: productKindSupportsBomParams(productKind) ? yieldRate : 0,
    yield_percent: productKindSupportsBomParams(productKind) ? Number((yieldRate * 100).toFixed(2)) : 0,
    default_price: Number(product.default_price || 0),
    margin_rate_override: marginRateOverride,
    margin_rate_override_input: marginRateOverride === null ? '' : marginRateOverride,
    customer_id: Number(product.customer_id || 0),
    base_product_id: Number(product.base_product_id || 0),
    visibility: productVisibility(product),
    custom_type: product.custom_type || '',
    bom_item_count: Number(product.bom_item_count || 0),
    bom_status: product.bom_status || (Number(product.bom_item_count || 0) > 0 ? 'active' : 'missing'),
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

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [data, customerData] = await Promise.all([
      apiGet('/api/product-settings'),
      apiGet('/api/customer-fulfillment/customers?limit=200'),
    ])
    categories.value = (data.categories || []).map(decorateCategory)
    products.value = (data.products || []).map(decorateProduct)
    gradientTemplates.value = (data.gradient_templates || []).map(normalizeGradientTemplate)
    productConfigTemplates.value = (data.product_config_templates || []).map(decorateProductConfigTemplate)
    productUnitDefinitions.value = (data.product_unit_definitions || []).map(decorateProductUnitDefinition)
    productUnitTemplates.value = (data.product_unit_templates || []).map(decorateProductUnitTemplate)
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
    customers.value = customerSkuCustomerOptions(customerData)
    syncSelectedCustomerSkuCustomer()
    applyWorkspaceCustomerContext()
    syncVisibleSkuTableState()
    pruneSelectedProducts(displaySkuRows.value)
  } catch (err) {
    error.value = err.message || '加载失败'
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

async function bindProductConfigTemplateToSubtype(category, templateID) {
  if (!canEditCategory(category)) return
  const payload = buildProductCategoryConfigPayload({
    ...category,
    product_config_template_id: Number(templateID || 0),
  })
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/categories/${payload.id}`, {
      method: 'PUT',
      body: payload,
    })
    ok.value = '子类型商品配置已更新'
    await loadAll()
  } catch (err) {
    error.value = err.message || '绑定商品配置失败'
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

function addSpecialAttrSchemaRow() {
  productConfigTemplateForm.value.special_attrs_schema_rows.push({
    key: '',
    label: '',
    value_type: 'text',
    options_text: '',
    required: false,
    show_in_price_list: false,
    position: productConfigTemplateForm.value.special_attrs_schema_rows.length + 1,
  })
}

function removeSpecialAttrSchemaRow(index) {
  productConfigTemplateForm.value.special_attrs_schema_rows.splice(index, 1)
}

function handleSpecialAttrSchemaTypeChange(attr) {
  if (!attr || attr.value_type === 'select') return
  attr.options = []
  attr.options_text = ''
}

function validateProductConfigTemplatePayload(payload) {
  if (!String(payload.name || '').trim()) return '请填写商品配置名称'
  if (Number(payload.unit_template_id || 0) <= 0) return '请选择单位模板'
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
    ok.value = '商品配置已保存，绑定的产品子类型会同步更新'
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
    ok.value = '全局单位已保存，可在单位模板中引用'
    await loadAll()
    resetGlobalUnitDefinitionForm()
  } catch (err) {
    error.value = err.message || '保存全局单位失败'
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
  if (!String(payload.name || '').trim()) return '请填写单位模板名称'
  if (!String(payload.inventory_unit || '').trim()) return '请选择成品库存单位'
  if (!String(payload.quote_unit || '').trim()) return '请选择报价单位'
  if (!String(payload.order_unit || '').trim()) return '请选择录单单位'
  return ''
}

async function saveProductUnitTemplate() {
  const payload = buildProductUnitTemplatePayload(productUnitTemplateForm.value)
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
    ok.value = '单位模板已保存，商品配置可直接引用'
    await loadAll()
    resetProductUnitTemplateForm()
  } catch (err) {
    error.value = err.message || '保存单位模板失败'
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
  target.unit_conversion_rows.push({
    from_qty: 1,
    from_unit: target.order_unit || target.quote_unit || '',
    to_qty: 1,
    to_unit: target.inventory_unit || target.quote_unit || '',
  })
}

function removeUnitConversionRow(target, index) {
  if (!target || !Array.isArray(target.unit_conversion_rows)) return
  target.unit_conversion_rows.splice(index, 1)
}

function startCustomerProductRuleOverrideEdit(row) {
  customerRuleOverrideForm.value = decorateCustomerProductRuleOverride({
    ...JSON.parse(JSON.stringify(row || {})),
    customer_id: Number(row?.customer_id || skuContextCustomerID.value || 0),
  })
}

function validateCustomerProductRulePayload(payload, requireName = false) {
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

async function openSkuCopyDrawer() {
  ensureSkuCopySource()
  skuCopyDrawerOpen.value = true
  await loadSkuCopyOptions()
}

function closeSkuCopyDrawer() {
  skuCopyDrawerOpen.value = false
}

async function loadSkuCopyOptions() {
  ensureSkuCopySource()
  skuCopyLoading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      target_customer_id: String(skuContextCustomerID.value),
      source_customer_id: String(copySourceCustomerID.value || 0),
    })
    skuCopyOptions.value = await apiGet(`/api/product-settings/skus/copy-options?${params.toString()}`)
    pruneCopySkuSelection()
  } catch (err) {
    error.value = err.message || '加载 SKU 复制选项失败'
    skuCopyOptions.value = defaultSkuCopyOptions()
  } finally {
    skuCopyLoading.value = false
  }
}

function ensureSkuCopySource() {
  const options = skuCopySourceOptions.value
  if (!options.length) {
    copySourceCustomerID.value = 0
    return
  }
  if (!options.some((source) => Number(source.id || 0) === Number(copySourceCustomerID.value || 0))) {
    copySourceCustomerID.value = Number(options[0].id || 0)
  }
}

function pruneCopySkuSelection() {
  const valid = new Set(copyableSkuIDs.value)
  copySkuSelection.value = copySkuSelection.value.filter((id) => valid.has(Number(id)))
}

function isCopySkuSelected(sku) {
  return copySkuSelection.value.includes(Number(sku?.id || 0))
}

function toggleCopySku(sku, checked) {
  if (sku?.copy_state === 'inactive') return
  const id = Number(sku?.id || 0)
  if (!id) return
  copySkuSelection.value = checked
    ? Array.from(new Set([...copySkuSelection.value, id]))
    : copySkuSelection.value.filter((item) => item !== id)
}

function copyIDsForSubtype(subtype = {}) {
  return (subtype.products || [])
    .filter((sku) => sku.copy_state !== 'inactive')
    .map((sku) => Number(sku.id || 0))
    .filter(Boolean)
}

function copyIDsForType(group = {}) {
  return (group.children || []).flatMap(copyIDsForSubtype)
}

function copyCountForSubtype(subtype = {}) {
  return copyIDsForSubtype(subtype).length
}

function copyCountForType(group = {}) {
  return copyIDsForType(group).length
}

function selectedCopyCountForSubtype(subtype = {}) {
  const selected = new Set(copySkuSelection.value)
  return copyIDsForSubtype(subtype).filter((id) => selected.has(id)).length
}

function selectedCopyCountForType(group = {}) {
  const selected = new Set(copySkuSelection.value)
  return copyIDsForType(group).filter((id) => selected.has(id)).length
}

function isCopySubtypeSelected(subtype = {}) {
  const ids = copyIDsForSubtype(subtype)
  return ids.length > 0 && ids.every((id) => copySkuSelection.value.includes(id))
}

function isCopyTypeSelected(group = {}) {
  const ids = copyIDsForType(group)
  return ids.length > 0 && ids.every((id) => copySkuSelection.value.includes(id))
}

function setCopyIDs(ids = [], checked = false) {
  const target = new Set(copySkuSelection.value)
  for (const id of ids) {
    if (checked) target.add(id)
    else target.delete(id)
  }
  copySkuSelection.value = Array.from(target)
}

function toggleCopySubtype(subtype, checked) {
  setCopyIDs(copyIDsForSubtype(subtype), checked)
}

function toggleCopyType(group, checked) {
  setCopyIDs(copyIDsForType(group), checked)
}

function setAllCopySkus(checked) {
  copySkuSelection.value = checked ? copyableSkuIDs.value.slice() : []
}

async function copySelectedSkus() {
  if (!copySelectedCount.value) return
  skuCopySaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/product-settings/skus/copy', {
      body: buildSkuCopyPayload({
        target_customer_id: skuContextCustomerID.value,
        source_customer_id: copySourceCustomerID.value,
        source_sku_ids: copySkuSelection.value,
      }),
    })
    ok.value = `SKU复制完成：新增 ${Number(result?.created_count || 0)} 款，覆盖 ${Number(result?.overwritten_count || 0)} 款`
    copySkuSelection.value = []
    await loadAll()
    await loadSkuCopyOptions()
  } catch (err) {
    error.value = err.message || '复制 SKU 失败'
  } finally {
    skuCopySaving.value = false
  }
}

function customerName(id) {
  return customers.value.find((customer) => Number(customer.id) === Number(id))?.name || ''
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
  return productUnitDefinitions.value.find((unit) => unit.code === normalized)?.name || normalized
}

function findProductUnitTemplate(id) {
  const templateID = Number(id || 0)
  if (!templateID) return null
  return productUnitTemplates.value.find((template) => Number(template.id || 0) === templateID) || null
}

function productUnitTemplateSummary(idOrTemplate) {
  const template = typeof idOrTemplate === 'object' ? idOrTemplate : findProductUnitTemplate(idOrTemplate)
  if (!template) return '未绑定单位模板'
  return `${template.name || '单位模板'} · 成品库存 ${productUnitName(template.inventory_unit)} · 报价 ${productUnitName(template.quote_unit)} · 录单 ${productUnitName(template.order_unit)}${template.integer_unit ? ' · 整数' : ''}`
}

function productConfigUnitTemplateName(idOrTemplate) {
  const template = typeof idOrTemplate === 'object' ? idOrTemplate : findProductUnitTemplate(idOrTemplate)
  return template?.name || '未绑定单位模板'
}

function productConfigUnitChips(idOrTemplate) {
  const template = typeof idOrTemplate === 'object' ? idOrTemplate : findProductUnitTemplate(idOrTemplate)
  if (!template) return ['单位未绑定']
  const chips = [
    `成品库存 ${productUnitName(template.inventory_unit)}`,
    `报价 ${productUnitName(template.quote_unit)}`,
    `录单 ${productUnitName(template.order_unit)}`,
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

function applyWorkspaceCustomerContext() {
  const nextCustomerID = nextSkuContextCustomerID(selectedCustomerSkuCustomerID.value, {
    workspaceMode: props.workspaceMode,
    customerContextID: props.customerContextId,
  })
  if (Number(selectedCustomerSkuCustomerID.value || 0) !== nextCustomerID) {
    selectedCustomerSkuCustomerID.value = nextCustomerID
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

function toggleAllProductRows(checked) {
  selectedProductIds.value = checked ? editableDisplaySkuRows.value.map((row) => Number(row.id)).filter(Boolean) : []
}

function handleSkuPaginationChange({ page, pageSize }) {
  skuPageSize.value = normalizePageSize(pageSize)
  skuPage.value = page
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

function productCategoryByID(categoryID) {
  const id = Number(categoryID || 0)
  if (!id) return null
  return flattenCategoryNodes(categoryTreeForSkuContext.value).find((category) => Number(category.id || 0) === id)
    || flattenCategoryNodes(categories.value).find((category) => Number(category.id || 0) === id)
    || null
}

function specialAttrSchemaForCategory(categoryID) {
  const category = productCategoryByID(categoryID)
  const template = productConfigTemplateByID(category?.product_config_template_id)
  return specialAttrSchemaRowsFromJSON(template?.special_attrs_schema_json || '[]')
}

function specialAttrSchemaForProduct(row = {}) {
  return specialAttrSchemaForCategory(row.product_category_id)
}

function openSpecialAttrConfigForProduct(row = {}) {
  error.value = ''
  activeSettingsSection.value = 'templates'
  activeConfigTemplateSection.value = 'product-config'
  const category = productCategoryByID(row.product_category_id)
  const template = productConfigTemplateByID(category?.product_config_template_id)
  if (template) {
    startProductConfigTemplateEdit(template)
  } else {
    resetProductConfigTemplateForm()
  }
  ok.value = '特殊属性字段在商品配置模板中定义；SKU列表特殊属性列填写具体值，勾选展示到价格表/PDF后，价格表页面、发布快照和 PDF 均展示'
}

function specialAttrSchemaForForm(form = {}) {
  return specialAttrSchemaForCategory(form.product_subtype_category_id)
}

function syncSpecialAttrValuesForForm(form = {}) {
  if (!form.special_attr_values || typeof form.special_attr_values !== 'object') {
    form.special_attr_values = {}
  }
  for (const attr of specialAttrSchemaForForm(form)) {
    if (!Object.prototype.hasOwnProperty.call(form.special_attr_values, attr.key)) {
      form.special_attr_values[attr.key] = ''
    }
  }
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
  syncSpecialAttrValuesForForm(form)
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

function roastedBomProductsForRow(row) {
  return roastedBomProductOptions(products.value, { customerID: Number(row?.customer_id || 0) })
}

function fillCustomProductName() {
  if (customForm.value.name) return
  const customer = customerName(selectedCustomerSkuCustomerID.value || customForm.value.customer_id)
  const base = customForm.value.product_kind === 'green_bean'
    ? products.value.find((product) => Number(product.id) === Number(customForm.value.green_bean_bom_product_id))
    : selectedBaseProduct()
  if (!customer || !base) return
  const attrSuffix = Object.values(customForm.value.special_attr_values || {}).find(Boolean)
  customForm.value.name = attrSuffix ? `${customer}-${base.name}-${attrSuffix}` : `${customer}-${base.name}`
}

function syncCustomFormFromBaseProduct(product) {
  if (!product) return
  const kind = normalizedProductKind(product)
  customForm.value.product_kind = kind
  customForm.value.special_attr_values = { ...specialAttrValuesFromJSON(product.special_attrs_json || '{}') }
  customForm.value.yield_percent = productKindSupportsBomParams(kind) ? Number(((Number(product.yield_rate || 0.8)) * 100).toFixed(2)) : 0
  customForm.value.green_bean_type = product.green_bean_type || 'single_origin'
  customForm.value.green_bean_bom_product_id = Number(product.green_bean_bom_product_id || 0)
  customForm.value.drip_bag_grams = Number(product.drip_bag_grams || 10)
  customForm.value.drip_box_bag_count = Number(product.drip_box_bag_count || 10)
  if (kind !== 'roasted') customForm.value.copy_bom = false
}

function handleCustomProductKindChange() {
  customForm.value.base_product_id = 0
  customForm.value.name = ''
  customForm.value.copy_bom = customForm.value.product_kind === 'roasted' && customForm.value.custom_type !== 'custom_roast'
  customForm.value.copy_price_tiers = customForm.value.product_kind !== 'green_bean' && customForm.value.custom_type !== 'custom_roast'
}

function openProductBom(row) {
  window.location.href = buildProductBomURL(window.location.href, row).toString()
}

async function createSku() {
  ensureProductTypeCategorySelected(skuForm.value)
  if (!skuForm.value.name) {
    error.value = '请填写 SKU 名称'
    return
  }
  skuSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend('/api/product-settings/skus', {
      body: buildSkuCreatePayload(skuContextCustomerID.value, skuForm.value),
    })
    ok.value = Number(skuForm.value.product_subtype_category_id || 0) > 0
      ? 'SKU已创建并挂到产品子类型'
      : 'SKU已创建，未选择产品子类型，已进入停车场'
    skuForm.value = defaultSkuForm()
    closeProductDrawer()
    await loadAll()
  } catch (err) {
    error.value = err.message || '新增 SKU 失败'
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
  const yieldPercent = Number(productForm.value.yield_percent || 0)
  if (productKindSupportsBomParams(productForm.value.product_kind) && (yieldPercent <= 0 || yieldPercent > 100)) {
    error.value = 'BOM预期产出率必须在 1% 到 100% 之间'
    return
  }
  if (productForm.value.product_kind === 'green_bean' && Number(productForm.value.green_bean_bom_product_id || 0) <= 0) {
    error.value = '生豆 SKU 必须绑定对应熟豆 BOM'
    return
  }
  if (productForm.value.product_kind === 'drip_bag' && Number(productForm.value.drip_bag_grams || 0) <= 0) {
    error.value = '挂耳每袋克重必须大于 0'
    return
  }
  if (productForm.value.product_kind === 'drip_bag' && Number(productForm.value.drip_box_bag_count || 0) <= 0) {
    error.value = '挂耳每盒袋数必须大于 0'
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
  if (customForm.value.product_kind === 'green_bean' && Number(customForm.value.green_bean_bom_product_id || 0) <= 0) {
    error.value = '生豆 SKU 必须绑定对应熟豆 BOM'
    return
  }
  if (customForm.value.product_kind === 'drip_bag' && Number(customForm.value.drip_bag_grams || 0) <= 0) {
    error.value = '挂耳每袋克重必须大于 0'
    return
  }
  if (customForm.value.product_kind === 'drip_bag' && Number(customForm.value.drip_box_bag_count || 0) <= 0) {
    error.value = '挂耳每盒袋数必须大于 0'
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
  return Number(primary?.number || 0) >= categoryTreeForSkuContext.value.length
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

async function createPrimaryCategoryInline() {
  const name = nextCategoryName('新产品类型', categoryTreeForSkuContext.value)
  const category = await saveCategory({
    name,
    parent_id: 0,
    customer_id: selectedCustomerSkuCustomerID.value,
    position: categoryTreeForSkuContext.value.length + 1,
  })
  const id = Number(category?.id || 0)
  if (id) {
    editingCategoryId.value = id
    editingCategoryName.value = category.name || name
    await focusCategoryAfterCreate({ ...category, id, parent_id: 0 })
  }
}

async function createSecondaryCategoryInline(primary) {
  if (!canEditCategory(primary)) return
  const name = nextCategoryName('新产品子类型', primary.children || [])
  const category = await saveCategory({
    name,
    parent_id: Number(primary.id),
    customer_id: selectedCustomerSkuCustomerID.value,
    position: Number(primary.children?.length || 0) + 1,
  })
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
  if (position < 1 || position > categoryTreeForSkuContext.value.length) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/categories/${category.id}/move`, {
      body: { parent_id: 0, position },
    })
    ok.value = '产品类型顺序已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '移动产品类型失败'
  } finally {
    loading.value = false
  }
}

async function saveCategory(body) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/product-settings/categories', { body })
    ok.value = '分类已保存'
    await loadAll()
    return result?.category || null
  } catch (err) {
    error.value = err.message || '保存分类失败'
    return null
  } finally {
    loading.value = false
  }
}

async function deriveCategoryTemplate(category) {
  const customerID = skuContextCustomerID.value
  if (!customerID || Number(category.customer_id || 0) !== 0) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend('/api/product-settings/customer-categories/derive', {
      body: {
        customer_id: customerID,
        source_category_id: Number(category.id || 0),
      },
    })
    ok.value = '公共分类已复制为客户分类，可改名、排序和绑定客户梯度模板'
    await loadAll()
  } catch (err) {
    error.value = err.message || '复制公共分类失败'
  } finally {
    loading.value = false
  }
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
  const payload = buildProductCategoryConfigPayload({
    ...category,
    name,
    parent_id: Number(category.parent_id || 0),
    customer_id: Number(category.customer_id || selectedCustomerSkuCustomerID.value || 0),
    position: Number(category.position || category.number || 1),
  })
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/categories/${category.id}`, {
      method: 'PUT',
      body: payload,
    })
    cancelCategoryNameEdit()
    ok.value = '分类已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存分类失败'
  } finally {
    loading.value = false
  }
}

async function deleteCategory(category) {
  if (!canEditCategory(category)) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/categories/${category.id}`, { method: 'DELETE' })
    ok.value = '分类已删除，相关商品已回到未分类'
    if (Number(category.parent_id || 0) === 0) {
      primaryDeleteMode.value = false
    } else {
      secondaryDeleteModeFor.value = 0
    }
    await loadAll()
  } catch (err) {
    error.value = err.message || '删除分类失败'
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
  const primary = categoryTreeForSkuContext.value.find((item) => Number(item.id) === Number(target?.parentID))
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
  const primary = categoryTreeForSkuContext.value.find((item) => Number(item.id) === primaryID)
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
    await apiSend(`/api/product-settings/categories/${drag.id}/move`, {
      body: { parent_id: parentID, position },
    })
    ok.value = '分类顺序已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '移动分类失败'
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
    error.value = '只能移动到当前客户自己的商品分类'
    clearDrag()
    return
  }
  try {
    await apiSend(`/api/product-settings/products/${drag.id}/category`, {
      body: buildAssignCategoryPayload({
        product,
        category: secondary,
        customerID: skuContextCustomerID.value,
        position: Number(secondary.products?.length || 0) + 1,
      }),
    })
    ok.value = '商品分类已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '移动商品失败'
  } finally {
    clearDrag()
  }
}

function normalizeMarginRateOverride(row) {
  const raw = row.margin_rate_override_input
  if (raw === '' || raw === null || typeof raw === 'undefined') return { ok: true, value: null }
  const value = Number(raw)
  if (!Number.isFinite(value) || value < 0) return { ok: false, value: null }
  return { ok: true, value: Number(value.toFixed(6)) }
}

async function saveProductMarginOverride(row) {
  await saveProductBasics(row, '产品级利润率覆盖已保存')
}

async function saveProductBasics(row, successMessage = '商品基础信息已保存') {
  if (!canEditSkuRow(row)) {
    error.value = '公共 SKU 为引用，请回到公共SKU归属维护'
    return
  }
  const productKind = normalizedProductKind(row)
  const yieldPercent = Number(row.yield_percent || 0)
  if (productKindSupportsBomParams(productKind) && (yieldPercent <= 0 || yieldPercent > 100)) {
    error.value = 'BOM预期产出率必须在 1% 到 100% 之间'
    return
  }
  if (productKind === 'green_bean' && Number(row.green_bean_bom_product_id || 0) <= 0) {
    error.value = '生豆 SKU 必须绑定对应熟豆 BOM'
    return
  }
  if (productKind === 'drip_bag' && Number(row.drip_bag_grams || 0) <= 0) {
    error.value = '挂耳每袋克重必须大于 0'
    return
  }
  if (productKind === 'drip_bag' && Number(row.drip_box_bag_count || 0) <= 0) {
    error.value = '挂耳每盒袋数必须大于 0'
    return
  }
  const marginOverride = normalizeMarginRateOverride(row)
  if (!marginOverride.ok) {
    error.value = '利润率覆盖必须为 0 或正数'
    return
  }
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/products/${row.id}`, {
      method: 'PUT',
      body: buildProductBasicsPayload(row, marginOverride.value),
    })
    row.margin_rate_override = marginOverride.value
    row.margin_rate_override_input = marginOverride.value === null ? '' : marginOverride.value
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

watch(selectedCustomerSkuCustomerID, (customerID) => {
  if (restoringProductSettingsDraft) {
    pruneSelectedProducts(displaySkuRows.value)
    return
  }
  skuForm.value = defaultSkuForm()
  copySkuSelection.value = []
  skuCopyOptions.value = defaultSkuCopyOptions()
  resetProductConfigTemplateForm()
  resetCustomerProductRuleForms()
  skuFilters.value = defaultSkuFilters()
  skuPage.value = 1
  collapsedPrimaryCategoryIds.value = []
  collapsedSecondaryCategoryIds.value = []
  pruneSelectedProducts(displaySkuRows.value)
  notifyWorkspaceCustomerChanged(customerID)
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

watch(skuFilters, () => {
  skuPage.value = 1
}, { deep: true })

watch(() => skuFilters.value.primaryCategory, () => {
  if (!skuSecondaryCategoryOptions.value.includes(skuFilters.value.secondaryCategory)) {
    skuFilters.value.secondaryCategory = ''
  }
})

watch(currentSkuSourceRows, () => {
  const normalized = normalizeSkuFiltersForCurrentRows()
  if (JSON.stringify(normalized) !== JSON.stringify(skuFilters.value)) {
    skuFilters.value = normalized
  }
}, { deep: true })

watch(displaySkuRows, (rows) => {
  pruneSelectedProducts(rows)
})

onMounted(async () => {
  await loadAll()
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
.sku-page-summary { padding: 10px 12px; }
.sku-page-summary .panel-head { align-items: center; }
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
.sku-workspace-tabs { display: inline-flex; align-items: center; gap: 4px; width: fit-content; border: 1px solid #e6e0d8; border-radius: 8px; background: #fbfaf8; padding: 4px; }
.workspace-tab { min-height: 32px; border: 0; border-radius: 6px; background: transparent; color: #333; padding: 0 14px; font-weight: 700; }
.workspace-tab.active { background: #111; color: #fff; }
.sku-master-workspace, .sku-template-workspace { display: grid; gap: 14px; min-width: 0; }
.master-data-layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; align-items: start; min-width: 0; }
.template-workspace-stack { display: grid; gap: 14px; min-width: 0; }
.config-template-tabs { display: inline-flex; align-items: center; gap: 4px; width: fit-content; border: 1px solid #e6e0d8; border-radius: 8px; background: #fbfaf8; padding: 4px; }
.config-template-tab { min-height: 32px; border: 0; border-radius: 6px; background: transparent; color: #333; padding: 0 14px; font-weight: 700; }
.config-template-tab.active { background: #111; color: #fff; }
.sku-context-panel { grid-column: 1 / -1; display: grid; gap: 10px; }
.compact-sku-context { padding: 10px 12px; }
.customer-rule-panel { grid-column: 1 / -1; }
.customer-rule-binding { display: grid; grid-template-columns: minmax(260px, 360px) minmax(0, 1fr); align-items: end; gap: 12px; margin-bottom: 12px; }
.customer-rule-layout { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(280px, .8fr); gap: 14px; align-items: start; }
.customer-rule-editor { display: grid; gap: 10px; }
.customer-rule-items, .customer-rule-overrides, .customer-rule-template-list { display: grid; gap: 8px; }
.customer-rule-item { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(160px, .7fr) minmax(120px, .5fr) auto; gap: 8px; align-items: end; border: 1px solid #eee; border-radius: 8px; padding: 10px; background: #fafafa; }
.customer-rule-editor label span, .customer-rule-binding label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.rule-config-block { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 8px; align-items: end; min-width: 0; border: 1px solid #eee8df; border-radius: 8px; padding: 10px; background: #fff; }
.special-attrs-schema-editor { gap: 10px; }
.special-attr-schema-row { grid-column: 1 / -1; display: grid; grid-template-columns: minmax(120px, 1fr) minmax(120px, 1fr) minmax(96px, .7fr) minmax(180px, 1.2fr) auto auto auto; gap: 8px; align-items: end; padding: 9px; border: 1px solid #eee; border-radius: 8px; background: #fff; }
.special-attr-schema-row label { min-width: 0; }
.special-attr-schema-row input, .special-attr-schema-row select { min-width: 0; }
.special-attr-editor { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 8px; }
.special-attr-editor.compact { grid-template-columns: minmax(150px, 1fr); min-width: 180px; }
.special-attr-editor label { display: grid; gap: 4px; min-width: 0; }
.special-attr-editor span { font-size: 12px; color: #666; }
.special-attr-editor input, .special-attr-editor select { min-width: 0; min-height: 32px; border: 1px solid #ddd; border-radius: 7px; padding: 5px 7px; font: inherit; box-sizing: border-box; }
.field-group-title { grid-column: 1 / -1; color: #3f3328; font-weight: 700; font-size: 13px; }
.field-group-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; font-weight: 700; color: #3f3328; }
.field-group-copy { display: grid; gap: 2px; }
.field-group-copy small { color: #7b746c; font-weight: 400; }
.unit-conversion-editor { grid-column: 1 / -1; display: grid; gap: 8px; min-width: 0; }
.unit-impact-help { grid-column: 1 / -1; color: #7b746c; line-height: 1.45; }
.unit-conversion-row { display: grid; grid-template-columns: minmax(72px, .45fr) minmax(82px, .7fr) auto minmax(72px, .45fr) minmax(82px, .7fr) auto; gap: 6px; align-items: center; min-width: 0; }
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
.sku-context-main { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.sku-context-main h3 { margin: 0; font-size: 17px; }
.sku-context-title-line { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; min-width: 0; }
.context-eyebrow { color: #7a4d1a; font-size: 12px; font-weight: 700; }
.sku-context-controls { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; min-width: min(460px, 100%); }
.context-stats { display: flex; flex-wrap: wrap; gap: 6px; }
.context-stats span { border: 1px solid #e6e0d8; border-radius: 999px; padding: 4px 9px; background: #fbfaf8; color: #333; font-size: 12px; }
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
.unit-template-layout { display: grid; grid-template-columns: minmax(220px, 280px) minmax(0, 1fr); gap: 12px; align-items: start; }
.unit-template-card { display: grid; gap: 10px; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; padding: 12px; min-width: 0; }
.unit-chip-list { display: flex; gap: 6px; flex-wrap: wrap; }
.unit-chip { min-height: 30px; display: inline-flex; align-items: center; gap: 5px; border-color: #d9d2c8; background: #fff; padding: 0 9px; font-size: 12px; }
.unit-chip small { color: #777; }
.unit-chip.inactive { opacity: .55; }
.unit-definition-form, .unit-template-form { display: grid; gap: 10px; }
.unit-definition-form { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: end; }
.unit-definition-form label, .unit-template-form label { display: grid; gap: 5px; font-size: 13px; }
.compact-template-list { max-height: 220px; overflow: auto; padding-right: 2px; }
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
.template-editor-grid { display: grid; grid-template-columns: minmax(0, 1fr) 160px; gap: 10px; }
.template-editor label, .product-config-editor label { display: grid; gap: 5px; font-size: 13px; }
.product-config-layout { display: grid; grid-template-columns: minmax(220px, 280px) minmax(0, 1fr); gap: 12px; align-items: start; }
.product-config-editor { display: grid; gap: 10px; }
.template-tier-head { display: flex; justify-content: space-between; align-items: center; gap: 10px; margin: 12px 0 8px; }
.template-tier-list { display: grid; gap: 8px; }
.template-tier-row { display: grid; grid-template-columns: minmax(130px, 1.1fr) minmax(130px, .9fr) minmax(130px, .9fr) minmax(100px, .7fr) auto; gap: 8px; align-items: end; border: 1px solid #e2ddd6; border-radius: 8px; background: #fff; padding: 10px; }
.template-select { width: min(280px, 100%); min-height: 30px; padding: 4px 8px; font-size: 12px; }
.sku-panel-title { align-items: flex-start; }
.sku-panel-actions { flex: 1; }
.sku-customer-select { min-width: 220px; max-width: 320px; flex: 1 1 220px; font-weight: 400; }
.sku-filters { display: grid; grid-template-columns: 150px 170px minmax(200px, 1fr) 180px 180px; gap: 8px; margin-bottom: 10px; align-items: end; }
.sku-filters label { display: grid; gap: 5px; font-size: 12px; color: #333; }
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
.category-collapse-button { width: 24px; height: 24px; min-height: 24px; display: inline-grid; place-items: center; flex: 0 0 auto; border: 1px solid #e3ddd4; border-radius: 999px; background: #fff; color: #4a4037; padding: 0; font-size: 15px; font-weight: 700; line-height: 1; cursor: pointer; }
.category-collapse-button:hover { background: #f4f0ea; }
.category-collapse-button.secondary-collapse { width: 22px; height: 22px; min-height: 22px; font-size: 13px; }
.category-scroll-list { max-height: min(640px, calc(100vh - 280px)); overflow: auto; display: grid; gap: 10px; padding-right: 2px; }
.category-tree { display: grid; gap: 10px; min-width: 0; }
.primary-category { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; background: #fbfaf8; min-width: 0; }
.category-head, .secondary-head, .category-actions { display: flex; align-items: center; gap: 8px; justify-content: space-between; }
.primary-category-head { align-items: flex-start; justify-content: space-between; gap: 10px; }
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
.secondary-category { border: 1px solid #ddd; border-radius: 8px; padding: 9px; background: #fff; cursor: grab; user-select: none; touch-action: none; min-width: 0; overflow: hidden; }
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
.sku-name-cell { min-width: 300px; }
.special-attrs-cell { min-width: 220px; }
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
.margin-input { width: 150px; }
.sku-name-input { min-width: 240px; }
.sku-table .sku-name-input { min-width: 300px; }
.sku-empty-special-attrs { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }
.remark-input { width: 180px; min-height: 46px; resize: vertical; }
.status-pill { display: inline-flex; align-items: center; min-height: 24px; border: 1px solid #cfd8cf; border-radius: 999px; padding: 2px 8px; color: #27602e; background: #f2fbf2; white-space: nowrap; }
.status-pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.muted { color: #666; font-size: 12px; }
.settings-drawer-mask { position: fixed; inset: 0; z-index: 60; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .22); }
.settings-drawer { width: min(760px, 94vw); height: 100%; overflow: auto; background: #fff; box-shadow: -12px 0 32px rgba(0, 0, 0, .16); padding: 16px; display: grid; grid-template-rows: auto 1fr; gap: 12px; }
.product-editor-drawer { width: min(820px, 94vw); }
.sku-copy-drawer { width: min(940px, 96vw); grid-template-rows: auto 1fr auto; }
.category-settings-drawer { width: min(920px, 96vw); }
.global-unit-dictionary-drawer { width: min(760px, 94vw); }
.drawer-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; border-bottom: 1px solid #eee8df; padding-bottom: 12px; }
.drawer-head h3 { margin: 0 0 4px; font-size: 18px; }
.drawer-head p { margin: 0; color: #666; font-size: 12px; }
.drawer-body { display: grid; gap: 12px; align-content: start; min-width: 0; }
.drawer-footer { border-top: 1px solid #eee8df; padding-top: 10px; display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.sku-copy-source { display: grid; gap: 5px; font-size: 13px; }
.sku-copy-selection { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 12px; display: grid; gap: 10px; }
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
@media (max-width: 1100px) {
  .custom-product-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .customer-rule-item { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .special-attr-schema-row { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .master-data-layout { grid-template-columns: 1fr; }
}
@media (max-width: 900px) {
  .page { padding: 12px; }
  .inline-form, .product-create-form, .custom-product-form, .gradient-template-layout, .product-config-layout, .unit-template-layout, .global-unit-drawer-body, .unit-definition-form, .template-editor-grid, .template-tier-row, .sku-filters, .customer-rule-binding, .customer-rule-layout, .customer-rule-item, .subtype-config-form, .rule-config-block, .unit-conversion-row { grid-template-columns: 1fr; }
  .sku-context-main { display: grid; }
  .sku-context-controls { justify-content: flex-start; min-width: 0; }
  .sku-workspace-tabs { width: 100%; }
  .workspace-tab { flex: 1; }
  .panel-actions { justify-content: flex-start; }
  .sku-panel-actions { width: 100%; }
  .sku-customer-select { max-width: none; }
  .product-create-form .wide-field, .custom-product-form .wide-field { grid-column: auto; }
  .template-select { width: 100%; }
  table { min-width: 1400px; }
}
</style>
