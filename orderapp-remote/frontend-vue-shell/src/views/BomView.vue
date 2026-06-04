<template>
  <div class="page">
    <div v-if="bomReturnNavigation" class="bom-return-banner">
      <button class="secondary bom-return-button" type="button" @click="returnToProductConfig">{{ bomReturnLabel }}</button>
      <span>完成 BOM 明细维护后可回到来源操作界面。</span>
    </div>
    <section class="panel">
      <div class="panel-head">
        <h2>生产 BOM（配方库）</h2>
        <div class="panel-actions">
          <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <p class="muted left">生产 BOM 是可分组、可复制、可复用的配方档案；商品档案在生产配置中引用某个 BOM 版本。</p>
    </section>

    <div class="grid">
      <section class="panel list-panel">
        <div class="panel-head bom-list-head">
          <div>
            <div class="panel-title compact-title">生产 BOM列表</div>
            <p class="muted left">生产 BOM 是独立配方档案；商品引用后会在 BOM 详情里展示。</p>
          </div>
        </div>
        <div class="bom-list-tabs-row">
          <div class="bom-list-tabs">
            <button
              :class="['secondary', 'compact-action', { active: selectedProductionBomGroupID === 0 }]"
              type="button"
              @click="selectProductionBomGroup(0)">
              全部分组
            </button>
            <button
              :class="['secondary', 'compact-action', { active: selectedProductionBomGroupID === -1 }]"
              type="button"
              @click="selectProductionBomGroup(-1)">
              未分类
            </button>
            <button
              v-for="group in productionBomGroups"
              :key="group.id"
              :class="['secondary', 'compact-action', { active: selectedProductionBomGroupID === Number(group.id || 0) }]"
              type="button"
              @click="selectProductionBomGroup(Number(group.id || 0))">
              {{ group.name }}
            </button>
          </div>
          <button class="primary compact-action" type="button" @click="openNewProductionBomRecord">新建生产 BOM</button>
        </div>
        <div class="bom-list-toolbar">
          <button class="secondary compact-action" type="button" @click="openGroupDrawer">增加分组</button>
          <label>
            <span>目标分组</span>
            <select v-model.number="selectedBomMoveGroupID">
              <option :value="0">未分类</option>
              <option v-for="group in productionBomGroups" :key="group.id" :value="Number(group.id || 0)">{{ group.name }}</option>
            </select>
          </label>
          <button class="secondary compact-action" type="button" :disabled="!canMoveSelectedBoms || loading" @click="moveSelectedProductBomsToGroup">
            移动到分组
          </button>
          <template v-if="isCustomProductionBomGroupSelected">
            <span class="toolbar-divider">组内分类</span>
            <div class="group-category-move-controls">
              <button class="secondary compact-action" type="button" @click="openGroupCategoryDrawer">新增小分类</button>
              <label>
                <span>目标小分类</span>
                <select v-model.number="selectedProductionBomGroupCategoryID">
                  <option :value="0">未分类</option>
                  <option v-for="category in productionBomGroupCategories" :key="category.id" :value="Number(category.id || 0)">{{ category.name }}</option>
                </select>
              </label>
            </div>
            <button class="secondary compact-action" type="button" :disabled="!canMoveSelectedBomsToGroupCategory || loading" @click="moveSelectedProductBomsToGroupCategory">
              移动到小分类
            </button>
          </template>
          <span class="muted left">已选 {{ selectedBomRecordsForMove.length }} 个可移动 BOM</span>
        </div>
        <div class="bom-list-filters">
          <label>
            <span>状态</span>
            <select v-model="productionBomStatusFilter">
              <option value="active">启用</option>
              <option value="inactive">已失效</option>
              <option value="all">全部</option>
            </select>
          </label>
          <label class="bom-search-field">
            <span>搜索 BOM</span>
            <input v-model.trim="productionBomSearchQuery" placeholder="按 BOM 名称或编号搜索" />
          </label>
          <button class="secondary compact-action danger-outline bom-batch-deactivate-action" type="button" :disabled="!canDeactivateSelectedBoms || loading" @click="deactivateSelectedProductionBoms">
            批量失效
          </button>
        </div>
        <div class="table-wrap bom-list-panel-scroll">
          <table>
            <thead>
              <tr>
                <th class="select-col"><input type="checkbox" :checked="isAllVisibleBomsSelected" :indeterminate.prop="isSomeVisibleBomsSelected" @change="toggleAllVisibleBoms" /></th>
                <th>BOM名称</th>
                <th>分组</th>
                <th>状态</th>
                <th>物料数</th>
                <th>引用商品</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody v-if="isCustomProductionBomGroupSelected">
              <template v-for="group in groupProductionBomRowsByInnerCategory" :key="group.key">
                <tr class="classification-group-row">
                  <td colspan="8">
                    <button class="text-button category-toggle" type="button" @click.stop="toggleBomGroupCategory(group.key)">
                      {{ isBomGroupCategoryCollapsed(group.key) ? '展开' : '收起' }}
                    </button>
                    <strong>{{ group.name }}</strong>
                    <span class="muted left">{{ group.rows.length }} 个 BOM</span>
                    <template v-if="group.category_id > 0">
                      <button class="text-button" type="button" @click.stop="openGroupCategoryDrawer(group.category)">改名</button>
                      <button class="text-button danger-text" type="button" @click.stop="deleteProductionBomGroupCategory(group.category)">删除</button>
                    </template>
                  </td>
                </tr>
                <template v-if="!isBomGroupCategoryCollapsed(group.key)">
                  <tr
                    v-for="row in group.rows"
                    :key="bomRowKey(row)"
                    :class="{ active: bomRowKey(row) === activeBomRowKey }"
                    @click="selectBomRow(row)">
                    <td class="select-col">
                      <input
                        v-model="selectedBomRowKeys"
                        type="checkbox"
                        :value="bomRowKey(row)"
                        :disabled="!isMovableBomRow(row)"
                        @click.stop />
                    </td>
                    <td>
                      <button class="text-button bom-name-button" type="button" @click.stop="openBomRowPrimary(row)">
                        {{ productionBomLabel(row) }}
                      </button>
                      <small v-if="productionBomVersionWarning(row)" class="bom-version-warning" data-warning-prefix="当前引用">{{ productionBomVersionWarning(row) }}</small>
                    </td>
                    <td>{{ bomGroupLabel(row) }}</td>
                    <td><span :class="['status-pill', row.status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.status) }}</span></td>
                    <td>{{ row.item_count || row.material_count || 0 }}</td>
                    <td>{{ row.reference_product_count || 0 }}</td>
                    <td>{{ row.updated_at }}</td>
                    <td>
                      <button class="text-button" type="button" :disabled="!Number(row.production_bom_id || row.id || 0)" @click.stop="copyProductionBomRecord(bomRecordFromRow(row))">复制</button>
                      <button class="text-button danger-text" type="button" :disabled="!Number(row.production_bom_id || row.id || 0) || row.status === 'inactive'" @click.stop="deactivateProductionBomRecord(bomRecordFromRow(row))">失效</button>
                    </td>
                  </tr>
                </template>
              </template>
              <tr v-if="!productionBomRows.length">
                <td colspan="8" class="muted">暂无配方档案</td>
              </tr>
            </tbody>
            <tbody v-else>
              <tr
                v-for="row in productionBomRows"
                :key="bomRowKey(row)"
                :class="{ active: bomRowKey(row) === activeBomRowKey }"
                @click="selectBomRow(row)">
                <td class="select-col">
                  <input
                    v-model="selectedBomRowKeys"
                    type="checkbox"
                    :value="bomRowKey(row)"
                    :disabled="!isMovableBomRow(row)"
                    @click.stop />
                </td>
                <td>
                  <button class="text-button bom-name-button" type="button" @click.stop="openBomRowPrimary(row)">
                    {{ productionBomLabel(row) }}
                  </button>
                  <small v-if="productionBomVersionWarning(row)" class="bom-version-warning" data-warning-prefix="当前引用">{{ productionBomVersionWarning(row) }}</small>
                </td>
                <td>{{ bomGroupLabel(row) }}</td>
                <td><span :class="['status-pill', row.status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.status) }}</span></td>
                <td>{{ row.item_count || row.material_count || 0 }}</td>
                <td>{{ row.reference_product_count || 0 }}</td>
                <td>{{ row.updated_at }}</td>
                <td>
                  <button class="text-button" type="button" :disabled="!Number(row.production_bom_id || row.id || 0)" @click.stop="copyProductionBomRecord(bomRecordFromRow(row))">复制</button>
                  <button class="text-button danger-text" type="button" :disabled="!Number(row.production_bom_id || row.id || 0) || row.status === 'inactive'" @click.stop="deactivateProductionBomRecord(bomRecordFromRow(row))">失效</button>
                </td>
              </tr>
              <tr v-if="!productionBomRows.length">
                <td colspan="8" class="muted">暂无配方档案</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel detail-panel">
        <div class="panel-title">配方明细</div>
        <div v-if="detail" class="summary">
          <div><span>生产 BOM</span><strong>{{ currentProductionBomLabel }}</strong></div>
          <div><span>引用商品</span><strong>{{ referencedProductsLabel }}</strong></div>
          <div v-if="currentProductionBomWarning"><span>版本提示</span><strong class="warn">{{ currentProductionBomWarning }}</strong></div>
          <div><span>工艺参数</span><strong>{{ detail.roast_level || '-' }}</strong></div>
          <div><span>状态</span><strong :class="{ warn: detail.status === 'inactive' }">{{ bomStatusLabel(detail.status) }}</strong></div>
          <div><span>关联工艺</span><strong>{{ linkedProcessTemplates.length ? `${linkedProcessTemplates.length} 个模板` : '-' }}</strong></div>
        </div>
        <div v-if="detail && linkedProcessTemplates.length" class="linked-processes">
          <span v-for="template in linkedProcessTemplates" :key="template.id" :class="['status-pill', template.status === 'inactive' ? 'inactive' : '']">
            {{ template.name }} · {{ processStatusLabel(template.status) }}
          </span>
        </div>
        <div v-if="detail && referencedProducts.length" class="linked-processes referenced-products">
          <button
            v-for="product in referencedProducts"
            :key="product.product_id"
            type="button"
            :class="['status-pill', 'referenced-product-button', product.active === false ? 'inactive' : '']"
            @click="openReferencedProductConfig(product)">
            {{ product.product_name || `商品 #${product.product_id}` }}<template v-if="product.product_code"> · {{ product.product_code }}</template><template v-if="product.bom_version_no"> · {{ product.bom_version_no }}</template>
          </button>
        </div>
        <div v-if="detail?.status === 'inactive'" class="warning-banner">当前 BOM 已失效，历史配方明细会保留；重新保存或启用版本后可恢复为有效 BOM。</div>
        <div v-if="!detail" class="muted empty">请选择生产 BOM</div>

        <div v-if="detail" class="detail-subpanel bom-version-panel">
          <div class="section-title-row">
            <div>
              <h3>BOM版本</h3>
              <p class="muted left">{{ currentProductionBomLabel }}</p>
            </div>
            <div class="inline-actions">
              <input v-model.trim="versionNote" placeholder="版本备注，例如 2026 春季豆单" :disabled="!currentProductionBomID || !canEditCurrentBomProduct" />
              <button class="primary compact-action" type="button" @click="copyVersionAsDraft()" :disabled="loading || !canCopyCurrentVersionAsDraft">复制为新版草稿</button>
            </div>
          </div>
          <div v-if="selectedProductionBomVersion?.status === 'published'" class="warning-banner">已发布版本只读，复制为新版草稿后编辑</div>
          <div class="table-wrap compact">
            <table>
              <thead>
                <tr>
                  <th>版本</th>
                  <th>状态</th>
                  <th>物料数</th>
                  <th>备注</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="version in versions"
                  :key="version.id"
                  :class="{ active: Number(version.id || 0) === Number(selectedProductionBomVersionID || 0) }"
                  @click="selectProductionBomVersion(version, { reload: true })">
                  <td>{{ version.version_no }}</td>
                  <td>{{ productionBomVersionStatusLabel(version.status) }}</td>
                  <td>{{ version.item_count }}</td>
                  <td>{{ version.note }}</td>
                  <td>{{ version.published_at || version.created_at }}</td>
                  <td>
                    <button
                      v-if="version.status === 'draft'"
                      class="text-button"
                      type="button"
                      @click.stop="activateVersion(version.id)"
                      :disabled="!canEditCurrentBomProduct">
                      发布版本
                    </button>
                    <button
                      v-else-if="version.status === 'published'"
                      class="text-button"
                      type="button"
                      @click.stop="copyVersionAsDraft(version)"
                      :disabled="!canEditCurrentBomProduct">
                      复制为新版草稿
                    </button>
                    <span v-else class="muted left">只读</span>
                  </td>
                </tr>
                <tr v-if="!versions.length">
                  <td colspan="6" class="muted">暂无版本</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="version-recipe-panel">
            <div class="section-title-row">
              <div>
                <h3>配方明细</h3>
                <p class="muted left">当前编辑版本：{{ selectedProductionBomVersion?.version_no || '-' }} · {{ productionBomVersionStatusLabel(selectedProductionBomVersion?.status || '') }}</p>
              </div>
              <div class="version-ratio-box">
                <span>合计比例</span>
                <strong :class="{ warn: detail.total_ratio > 100 }">{{ ratio(detail.total_ratio) }}</strong>
              </div>
            </div>
            <form class="inline-form" @submit.prevent="saveItem">
              <label>
                <span>组件类型</span>
                <select v-model="itemForm.component_type" :disabled="!detail || !canEditCurrentBomItems" @change="syncComponentTypeDefaults">
                  <option value="material">物料</option>
                  <option value="finished_product">成品</option>
                </select>
              </label>
              <label>
                <span>{{ itemForm.component_type === 'finished_product' ? '熟豆成品' : '物料' }}</span>
                <SearchableSelect
                  v-if="itemForm.component_type === 'finished_product'"
                  v-model="itemForm.component_product_id"
                  :options="roastedBeanProducts"
                  :option-label="optionLabel"
                  :option-meta="optionMeta"
                  :option-value="optionNumericValue"
                  placeholder="选择熟豆成品"
                  empty-text="没有可用熟豆成品"
                  :disabled="!detail || !canEditCurrentBomItems" />
                <SearchableSelect
                  v-else
                  v-model="itemForm.material_id"
                  :options="materials"
                  :option-label="optionLabel"
                  :option-value="optionNumericValue"
                  placeholder="选择物料"
                  empty-text="没有匹配物料"
                  :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <label>
                <span>消耗单位</span>
                <select v-model="itemForm.consume_unit" :disabled="!detail || !canEditCurrentBomItems || itemForm.component_type === 'finished_product'">
                  <option v-for="unit in currentConsumeUnitOptions" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                </select>
              </label>
              <label v-if="itemForm.consume_unit === 'ratio_pct'">
                <span>比例 %</span>
                <input v-model.number="itemForm.ratio_pct" type="number" min="0.01" max="100" step="0.01" :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <label v-else>
                <span>用量</span>
                <input v-model.number="itemForm.qty_per_unit" type="number" min="0.001" step="0.001" :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <button class="primary" type="submit" :disabled="!detail || loading || !canEditCurrentBomItems">保存组件</button>
            </form>

            <div class="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>类型</th>
                    <th>组件</th>
                    <th>用量</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in detailItems" :key="item.id">
                    <td>{{ componentTypeLabel(item.component_type) }}</td>
                    <td>{{ componentItemName(item) }}</td>
                    <td>{{ itemQuantityDisplay(item) }}</td>
                    <td><button class="text-button" type="button" :disabled="!canEditCurrentBomItems" @click="deleteItem(item.id)">删除</button></td>
                  </tr>
                  <tr v-if="!detailItems.length">
                    <td colspan="4" class="muted">暂无组件</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div v-if="detail && !isWorkspaceCustomerLocked" class="detail-subpanel bag-spec-mapping-panel">
          <div class="section-title-row">
            <div>
              <h3>全局规格袋材映射</h3>
              <p class="muted left">规格袋材映射在 BOM 详情中维护，但数据对所有 BOM 共用。</p>
            </div>
          </div>
          <form class="inline-form compact-form" @submit.prevent="saveMapping">
            <label>
              <span>规格 g</span>
              <input v-model.number="mappingForm.spec_g" type="number" min="1" step="1" />
            </label>
            <label>
              <span>袋材物料</span>
              <select v-model.number="mappingForm.material_id">
                <option :value="0">选择物料</option>
                <option v-for="material in materials" :key="material.id" :value="material.id">{{ material.name }}</option>
              </select>
            </label>
            <button class="primary compact-action" type="submit" :disabled="loading">保存映射</button>
          </form>
          <div class="table-wrap compact">
            <table>
              <thead>
                <tr>
                  <th>规格</th>
                  <th>袋材物料</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="mapping in mappings" :key="mapping.spec_g">
                  <td>{{ mapping.spec_g }}g</td>
                  <td>{{ mapping.material_name }}</td>
                  <td><button class="text-button" type="button" @click="deleteMapping(mapping.spec_g)">删除</button></td>
                </tr>
                <tr v-if="!mappings.length">
                  <td colspan="3" class="muted">暂无映射</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

      </section>
    </div>

    <div v-if="bomDrawerOpen" class="drawer-mask" @click.self="closeBomDrawer">
      <aside class="drawer">
        <div class="drawer-head">
          <div>
            <h3>{{ bomFormTitle }}</h3>
            <p class="muted left">只维护配方库档案。版本和配方明细在右侧“BOM版本”和“配方明细”中处理。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeBomDrawer">关闭</button>
        </div>
        <form class="inline-form bom-record-form" @submit.prevent="saveProductionBomRecord">
          <label>
            <span>BOM名称</span>
            <input v-model.trim="bomForm.name" placeholder="例如 精品拼配" />
          </label>
          <label>
            <span>分组</span>
            <select v-model.number="bomForm.group_id" @change="bomForm.group_category_id = 0">
              <option :value="0">未分类</option>
              <option v-for="group in productionBomGroups" :key="group.id" :value="Number(group.id || 0)">{{ group.name }}</option>
            </select>
          </label>
          <label v-if="Number(bomForm.group_id || 0) > 0">
            <span>组内分类</span>
            <select v-model.number="bomForm.group_category_id">
              <option :value="0">未分类</option>
              <option v-for="category in bomFormGroupCategories" :key="category.id" :value="Number(category.id || 0)">{{ category.name }}</option>
            </select>
          </label>
          <label v-if="bomForm.mode === 'edit'">
            <span>状态</span>
            <select v-model="bomForm.status">
              <option value="active">启用</option>
              <option value="inactive">已失效</option>
            </select>
          </label>
          <button class="primary" type="submit" :disabled="loading || !bomForm.name">{{ bomForm.mode === 'copy' ? '复制 BOM' : '保存 BOM' }}</button>
        </form>
      </aside>
    </div>

    <div v-if="groupCategoryDrawerOpen" class="drawer-mask" @click.self="closeGroupCategoryDrawer">
      <aside class="drawer">
        <div class="drawer-head">
          <div>
            <h3>{{ categoryForm.id ? '编辑组内分类' : '新增组内分类' }}</h3>
            <p class="muted left">当前大组：{{ selectedProductionBomGroup?.name || '-' }}。组内分类只用于当前大组下的 BOM 归类。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeGroupCategoryDrawer">关闭</button>
        </div>
        <form class="inline-form" @submit.prevent="saveProductionBomGroupCategory">
          <label>
            <span>分类名称</span>
            <input v-model.trim="categoryForm.name" placeholder="例如 浅烘 / 深烘 / 常规" />
          </label>
          <label>
            <span>排序</span>
            <input v-model.number="categoryForm.sort_order" type="number" min="0" step="1" />
          </label>
          <button class="primary" type="submit" :disabled="loading || !categoryForm.name || !selectedProductionBomGroup">保存小分类</button>
        </form>
      </aside>
    </div>

    <div v-if="groupDrawerOpen" class="drawer-mask" @click.self="closeGroupDrawer">
      <aside class="drawer">
        <div class="drawer-head">
          <div>
            <h3>管理分组</h3>
            <p class="muted left">分组只用于配方库归类。删除分组时，组内 BOM 会回到未分类。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeGroupDrawer">关闭</button>
        </div>
        <form class="inline-form" @submit.prevent="saveProductionBomGroup">
          <label>
            <span>分组名称</span>
            <input v-model.trim="groupForm.name" placeholder="例如 常用配方" />
          </label>
          <label>
            <span>排序</span>
            <input v-model.number="groupForm.sort_order" type="number" min="0" step="1" />
          </label>
          <button class="primary" type="submit" :disabled="loading || !groupForm.name">{{ groupForm.id ? '保存分组' : '新增分组' }}</button>
          <button class="secondary" type="button" @click="resetGroupForm">清空</button>
        </form>
        <div class="table-wrap compact">
          <table>
            <thead>
              <tr>
                <th>分组</th>
                <th>排序</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="group in managedProductionBomGroups" :key="group.id">
                <td>{{ group.name }}</td>
                <td>{{ group.sort_order }}</td>
                <td>
                  <button class="text-button" type="button" @click="editProductionBomGroup(group)">编辑</button>
                  <button class="text-button" type="button" @click="moveProductionBomGroup(group, -1)">上移</button>
                  <button class="text-button" type="button" @click="moveProductionBomGroup(group, 1)">下移</button>
                  <button
                    class="text-button danger-text"
                    type="button"
                    @click="deleteProductionBomGroup(group)">
                    DELETE
                  </button>
                </td>
              </tr>
              <tr v-if="!managedProductionBomGroups.length">
                <td colspan="3" class="muted">暂无分组</td>
              </tr>
            </tbody>
          </table>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import { filterProductionBomCatalog, productionBomDetailAsRecipeDetail, productionBomLabel, productionBomVersionWarning } from '../lib/bom'
import { componentTypeLabel, isDripProduct } from '../lib/drip-product'
import { FORM_DRAFT_SCOPES, readFormDraft, saveFormDraft } from '../lib/form-draft-cache'
import { replaceHistoryURL } from '../lib/url-state'
import { CUSTOMER_WORKSPACE_MODE } from '../lib/workspace-mode'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})
const BOM_FORM_DRAFT_SCOPE = FORM_DRAFT_SCOPES.bom

const productionBoms = ref([])
const products = ref([])
const materials = ref([])
const mappings = ref([])
const versions = ref([])
const processTemplates = ref([])
const productionBomGroups = ref([])
const productionBomDetail = ref(null)
const selectedProductionBomRecord = ref(null)
const detail = ref(null)
const selectedProductId = ref(0)
const selectedProductionBomGroupID = ref(0)
const selectedProductionBomVersionID = ref(0)
const selectedBomMoveGroupID = ref(0)
const selectedProductionBomGroupCategoryID = ref(0)
const selectedBomRowKeys = ref([])
const collapsedBomGroupCategoryKeys = ref([])
const pendingProductionBomID = ref(0)
const bomReturnNavigation = computed(() => props.viewParams?.return_navigation || null)
const bomReturnProductID = computed(() => Number(bomReturnNavigation.value?.params?.open_product_config_id || 0))
const bomReturnLabel = computed(() => String(bomReturnNavigation.value?.label || '返回商品档案配置'))
const groupDrawerOpen = ref(false)
const groupCategoryDrawerOpen = ref(false)
const bomDrawerOpen = ref(false)
const productionBomStatusFilter = ref('active')
const productionBomSearchQuery = ref('')
const managedProductionBomGroups = ref([])
const loading = ref(false)
const error = ref('')
const ok = ref('')
const itemForm = reactive({
  component_type: 'material',
  material_id: 0,
  component_product_id: 0,
  component_spec_g: 0,
  consume_unit: 'ratio_pct',
  qty_per_unit: '',
  ratio_pct: '',
})
const mappingForm = reactive({ spec_g: 227, material_id: 0 })
const groupForm = reactive({ id: 0, name: '', sort_order: 100 })
const categoryForm = reactive({ id: 0, name: '', sort_order: 100 })
const bomForm = reactive({ id: 0, source_id: 0, mode: 'create', name: '', group_id: 0, group_category_id: 0, status: 'active' })
const versionNote = ref('')

const detailItems = computed(() => detail.value?.items || [])
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && Number(props.customerContextId || 0) > 0)
const productionBomRows = computed(() => {
  return filterProductionBomCatalog(productionBoms.value, {
    status: productionBomStatusFilter.value,
    query: productionBomSearchQuery.value,
    groupID: Number(selectedProductionBomGroupID.value || 0),
  })
})
const selectedProductionBomGroup = computed(() => {
  const groupID = Number(selectedProductionBomGroupID.value || 0)
  return productionBomGroups.value.find((group) => Number(group.id || 0) === groupID) || null
})
const isCustomProductionBomGroupSelected = computed(() => Number(selectedProductionBomGroupID.value || 0) > 0)
const productionBomGroupCategories = computed(() => Array.isArray(selectedProductionBomGroup.value?.categories) ? selectedProductionBomGroup.value.categories : [])
const bomFormGroupCategories = computed(() => {
  const groupID = Number(bomForm.group_id || 0)
  return productionBomGroups.value.find((group) => Number(group.id || 0) === groupID)?.categories || []
})
const groupProductionBomRowsByInnerCategory = computed(() => {
  if (!isCustomProductionBomGroupSelected.value) return []
  const buckets = new Map()
  const ensureBucket = (categoryID, name, category = null) => {
    const key = `category:${Number(categoryID || 0)}`
    if (!buckets.has(key)) buckets.set(key, { key, category_id: Number(categoryID || 0), name, category, rows: [] })
    return buckets.get(key)
  }
  ensureBucket(0, '未分类')
  for (const category of productionBomGroupCategories.value) {
    ensureBucket(Number(category.id || 0), category.name || '未命名分类', category)
  }
  for (const row of productionBomRows.value) {
    const categoryID = Number(row.group_category_id || row.production_bom_group_category_id || 0)
    const bucket = buckets.get(`category:${categoryID}`) || buckets.get('category:0')
    bucket.rows.push(row)
  }
  return [...buckets.values()].filter((bucket) => bucket.category_id === 0 || bucket.rows.length > 0)
})
const linkedProcessTemplates = computed(() => processTemplates.value.filter((template) => Number(template.product_id || 0) === Number(selectedProductId.value || 0)))
const selectedProduct = computed(() => productByID(selectedProductId.value))
const referencedProducts = computed(() => detail.value?.referenced_products || productionBomDetail.value?.referenced_products || [])
const referencedProductsLabel = computed(() => {
  const count = referencedProducts.value.length
  if (count > 0) return `${count} 个商品`
  const summaryCount = Number(selectedProductionBomRecord.value?.reference_product_count || detail.value?.reference_product_count || productionBomDetail.value?.reference_product_count || 0)
  return summaryCount > 0 ? `${summaryCount} 个商品` : '未被商品引用'
})
const currentProductionBomLabel = computed(() => productionBomLabel(detail.value || selectedProductionBomRecord.value || {}))
const currentProductionBomWarning = computed(() => productionBomVersionWarning(detail.value || selectedProductionBomRecord.value || {}))
const currentProductionBomID = computed(() => Number(detail.value?.production_bom_id || selectedProductionBomRecord.value?.production_bom_id || selectedProductionBomRecord.value?.id || 0))
const bomFormTitle = computed(() => ({
  create: '新建生产 BOM',
  edit: '编辑 BOM',
  copy: '复制 BOM',
})[bomForm.mode] || '生产 BOM')
const selectedProductionBomVersion = computed(() => versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0)) || null)
const activeBomRowKey = computed(() => {
  if (selectedProductionBomRecord.value) return `bom:${Number(selectedProductionBomRecord.value.id || selectedProductionBomRecord.value.production_bom_id || 0)}`
  return ''
})
const canEditCurrentBomProduct = computed(() => {
  if (selectedProductionBomRecord.value) return true
  if (!selectedProductId.value) return true
  if (detail.value?.can_edit_bom === false) return false
  return true
})
const selectedProductionBomDraftVersion = computed(() => versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0) && version.status === 'draft') || null)
const canEditCurrentBomItems = computed(() => canEditCurrentBomProduct.value && Number(selectedProductionBomDraftVersion.value?.id || 0) > 0)
const canCopyCurrentVersionAsDraft = computed(() => canEditCurrentBomProduct.value && currentProductionBomID.value > 0 && selectedProductionBomVersion.value?.status === 'published')
const roastedBeanProducts = computed(() => products.value.filter((product) => {
  if (Number(product.id || 0) === Number(selectedProductId.value || 0)) return false
  return (product.product_kind || 'roasted_bean') === 'roasted_bean'
}))
const materialConsumeUnitOptions = [
  { value: 'ratio_pct', label: '比例 %' },
  { value: 'g_per_bag', label: '克/袋' },
  { value: 'unit_per_bag', label: '个/袋' },
  { value: 'unit_per_box', label: '个/盒' },
]
const finishedProductConsumeUnitOptions = [
  { value: 'g_per_bag', label: '克/袋' },
]
const currentConsumeUnitOptions = computed(() => itemForm.component_type === 'finished_product'
  ? finishedProductConsumeUnitOptions
  : materialConsumeUnitOptions)
const visibleMovableBomRows = computed(() => productionBomRows.value.filter(isMovableBomRow))
const isAllVisibleBomsSelected = computed(() => {
  const keys = visibleMovableBomRows.value.map(bomRowKey)
  if (!keys.length) return false
  const selected = new Set(selectedBomRowKeys.value)
  return keys.every((key) => selected.has(key))
})
const isSomeVisibleBomsSelected = computed(() => {
  const keys = visibleMovableBomRows.value.map(bomRowKey)
  if (!keys.length) return false
  const selected = new Set(selectedBomRowKeys.value)
  const count = keys.filter((key) => selected.has(key)).length
  return count > 0 && count < keys.length
})
const selectedBomRows = computed(() => {
  const selected = new Set(selectedBomRowKeys.value)
  return productionBoms.value.filter((row) => selected.has(bomRowKey(row)) && isMovableBomRow(row))
})
const selectedBomRecordsForMove = computed(() => {
  const byBomID = new Map()
  for (const row of selectedBomRows.value) {
    const record = bomRecordFromRow(row)
    if (record.id > 0 && !byBomID.has(record.id)) byBomID.set(record.id, record)
  }
  return [...byBomID.values()]
})
const selectedActiveBomRecordsForDeactivate = computed(() => {
  const byBomID = new Map()
  for (const row of selectedBomRows.value) {
    const record = bomRecordFromRow(row)
    if (record.id > 0 && record.status !== 'inactive' && !byBomID.has(record.id)) byBomID.set(record.id, record)
  }
  return [...byBomID.values()]
})
const canMoveSelectedBoms = computed(() => {
  const targetGroupID = Number(selectedBomMoveGroupID.value || 0)
  return selectedBomRecordsForMove.value.some((bom) => Number(bom.group_id || 0) !== targetGroupID)
})
const canMoveSelectedBomsToGroupCategory = computed(() => {
  if (!isCustomProductionBomGroupSelected.value) return false
  const targetGroupID = Number(selectedProductionBomGroupID.value || 0)
  const targetCategoryID = Number(selectedProductionBomGroupCategoryID.value || 0)
  return selectedBomRecordsForMove.value.some((bom) => {
    return Number(bom.group_id || 0) !== targetGroupID || Number(bom.group_category_id || 0) !== targetCategoryID
  })
})
const canDeactivateSelectedBoms = computed(() => selectedActiveBomRecordsForDeactivate.value.length > 0)

function ratio(value) {
  const n = Number(value || 0)
  return `${n.toFixed(2)}%`
}

function qty(value) {
  const n = Number(value || 0)
  if (!n) return '-'
  return Number.isInteger(n) ? String(n) : n.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}

function bomRowKey(row) {
  const bomID = Number(row?.production_bom_id || row?.id || 0)
  if (bomID > 0) return `bom:${bomID}`
  return `product:${Number(row?.product_id || 0)}`
}

function isMovableBomRow(row = {}) {
  return Number(row.production_bom_id || 0) > 0
}

function optionLabel(option) {
  return option?.name || ''
}

function optionMeta(option) {
  const parts = []
  parts.push('商品档案')
  if (option?.number) parts.push(option.number)
  if (option?.roast_level) parts.push(option.roast_level)
  return parts.join(' / ')
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function bomFormDraftKey() {
  const workspace = props.workspaceMode || 'factory'
  const customerID = Number(props.customerContextId || 0)
  return `${BOM_FORM_DRAFT_SCOPE}:${workspace}:${customerID || 'all'}`
}

function saveBomFormDraft() {
	saveFormDraft(bomFormDraftKey(), {
		selectedProductionBomID: currentProductionBomID.value,
		itemForm: { ...itemForm },
		mappingForm: { ...mappingForm },
		versionNote: versionNote.value,
	})
}

async function restoreBomFormDraft() {
  const params = new URL(window.location.href).searchParams
  if (Number(params.get('product_id') || 0) > 0) return
  if (Number(params.get('production_bom_id') || params.get('bom_id') || 0) > 0) return
  const draft = readFormDraft(bomFormDraftKey())
  if (!draft) return
  pendingProductionBomID.value = Number(draft.selectedProductionBomID || 0)
  Object.assign(itemForm, {
    component_type: 'material',
    material_id: 0,
    component_product_id: 0,
    component_spec_g: 0,
    consume_unit: 'ratio_pct',
    qty_per_unit: '',
    ratio_pct: '',
	}, draft.itemForm || {})
	Object.assign(mappingForm, { spec_g: 227, material_id: 0 }, draft.mappingForm || {})
	versionNote.value = draft.versionNote || ''
	if (pendingProductionBomID.value > 0) {
    await selectProductionBomRecordByID(pendingProductionBomID.value)
  }
}

function normalizeBomProduct(product) {
  return {
    ...product,
    id: Number(product.id || 0),
    customer_id: Number(product.customer_id || 0),
  }
}

function normalizeProductionBomRecord(row = {}) {
  return {
    ...row,
    id: Number(row.id || row.production_bom_id || 0),
    production_bom_id: Number(row.production_bom_id || row.id || 0),
    group_id: Number(row.group_id || 0),
    production_bom_group_id: Number(row.production_bom_group_id ?? row.group_id ?? 0),
    group_category_id: Number(row.group_category_id || row.production_bom_group_category_id || 0),
    production_bom_group_category_id: Number(row.production_bom_group_category_id ?? row.group_category_id ?? 0),
    group_category_name: row.group_category_name || row.production_bom_group_category_name || '',
    production_bom_group_category_name: row.production_bom_group_category_name || row.group_category_name || '',
    item_count: Number(row.item_count || row.material_count || 0),
    reference_product_count: Number(row.reference_product_count || 0),
    latest_version_status: row.latest_version_status || '',
    status: row.status === 'inactive' ? 'inactive' : 'active',
  }
}

function productByID(productId) {
  const id = Number(productId || 0)
  return products.value.find((product) => Number(product.id || 0) === id) || null
}

function consumeUnitLabel(unit) {
  return materialConsumeUnitOptions.find((option) => option.value === unit)?.label || unit || '-'
}

function componentItemName(item) {
  if (item?.component_type === 'finished_product') return item.component_product_name || `成品 #${item.component_product_id}`
  return item?.material_name || `物料 #${item?.material_id || 0}`
}

function itemQuantityDisplay(item) {
  if ((item?.consume_unit || 'ratio_pct') === 'ratio_pct') return ratio(item.ratio_pct)
  return `${qty(item.qty_per_unit)} ${consumeUnitLabel(item.consume_unit)}`
}

function productionBomDraftItemFromItem(item = {}) {
  return {
    material_id: Number(item.material_id || 0),
    component_type: item.component_type === 'finished_product' ? 'finished_product' : 'material',
    component_product_id: Number(item.component_product_id || 0),
    component_spec_g: Number(item.component_spec_g || 0),
    consume_unit: item.consume_unit || 'ratio_pct',
    qty_per_unit: Number(item.qty_per_unit || 0),
    ratio_pct: Number(item.ratio_pct || 0),
  }
}

function productionBomDraftItemFromForm() {
  return {
    material_id: Number(itemForm.material_id || 0),
    component_type: itemForm.component_type === 'finished_product' ? 'finished_product' : 'material',
    component_product_id: Number(itemForm.component_product_id || 0),
    component_spec_g: Number(itemForm.component_spec_g || 0),
    consume_unit: itemForm.consume_unit || 'ratio_pct',
    qty_per_unit: Number(itemForm.qty_per_unit || 0),
    ratio_pct: Number(itemForm.ratio_pct || 0),
  }
}

async function saveProductionBomDraftItems(items) {
  const draftVersionID = Number(selectedProductionBomDraftVersion.value?.id || 0)
  if (!draftVersionID) throw new Error('请先复制为新版草稿后再编辑配方明细')
  await apiSend(`/api/production-bom-versions/${draftVersionID}/draft`, {
    method: 'PUT',
    body: {
      items,
    },
  })
}

function productionBomVersionStatusLabel(status) {
  if (status === 'draft') return '草稿'
  if (status === 'published' || status === 'active') return '已发布'
  if (status === 'archived') return '已归档'
  return status || '-'
}

function selectProductionBomGroup(groupID) {
  selectedProductionBomGroupID.value = Number(groupID || 0)
  const groupOnlyRows = filterProductionBomCatalog(productionBoms.value, {
    status: 'all',
    query: '',
    groupID: Number(selectedProductionBomGroupID.value || 0),
  })
  const visibleBomIDs = new Set(groupOnlyRows.map((bom) => Number(bom.production_bom_id || bom.id || 0)))
  const selectedBomID = Number(selectedProductionBomRecord.value?.id || selectedProductionBomRecord.value?.production_bom_id || detail.value?.production_bom_id || 0)
  if (selectedBomID && !visibleBomIDs.has(selectedBomID)) {
    clearSelectedProductionBom()
  }
}

function bomGroupLabel(row) {
  const groupName = String(row?.production_bom_group_name || row?.group_name || '').trim() || '未分类'
  const categoryName = String(row?.production_bom_group_category_name || row?.group_category_name || '').trim()
  if (categoryName && groupName !== '未分类') return `${groupName} / ${categoryName}`
  return groupName
}

function bomRecordFromRow(row = {}) {
  return {
    id: Number(row.production_bom_id || row.id || 0),
    code: row.production_bom_code || row.code || '',
    name: row.production_bom_name || row.name || row.product || '',
    group_id: Number(row.production_bom_group_id ?? row.group_id ?? 0),
    group_name: row.production_bom_group_name || row.group_name || '',
    group_category_id: Number(row.production_bom_group_category_id ?? row.group_category_id ?? 0),
    group_category_name: row.production_bom_group_category_name || row.group_category_name || '',
    status: row.status === 'inactive' ? 'inactive' : 'active',
  }
}

function toggleAllVisibleBoms(event) {
  const shouldSelect = Boolean(event?.target?.checked)
  const next = new Set(selectedBomRowKeys.value)
  for (const row of visibleMovableBomRows.value) {
    const key = bomRowKey(row)
    if (shouldSelect) next.add(key)
    else next.delete(key)
  }
  selectedBomRowKeys.value = [...next]
}

async function selectProductionBomVersion(version, options = {}) {
  const versionID = Number(version?.id || version || 0)
  selectedProductionBomVersionID.value = versionID
  if (options.reload && currentProductionBomID.value > 0 && versionID > 0) {
    await loadProductionBomDetailForVersion(currentProductionBomID.value, versionID)
  }
}

function syncSelectedProductionBomVersion() {
  if (!versions.value.length) {
    selectedProductionBomVersionID.value = 0
    return
  }
  const existing = versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0))
  const selected = existing || versions.value.find((version) => version.status === 'draft') || versions.value.find((version) => version.is_latest) || versions.value[0]
  selectedProductionBomVersionID.value = Number(selected?.id || 0)
}

function resetGroupForm() {
  groupForm.id = 0
  groupForm.name = ''
  groupForm.sort_order = 100
}

function resetCategoryForm() {
  categoryForm.id = 0
  categoryForm.name = ''
  categoryForm.sort_order = 100
}

function resetBomForm() {
  bomForm.id = 0
  bomForm.source_id = 0
  bomForm.mode = 'create'
  bomForm.name = ''
  bomForm.group_id = Number(selectedProductionBomGroupID.value || 0) > 0 ? Number(selectedProductionBomGroupID.value || 0) : 0
  bomForm.group_category_id = 0
  bomForm.status = 'active'
}

function openNewProductionBomRecord() {
  resetBomForm()
  bomForm.mode = 'create'
  bomDrawerOpen.value = true
}

function openEditProductionBomRecord(bom) {
  resetBomForm()
  bomForm.mode = 'edit'
  bomForm.id = Number(bom?.id || 0)
  bomForm.name = bom?.name || ''
  bomForm.group_id = Number(bom?.group_id || 0)
  bomForm.group_category_id = Number(bom?.group_category_id || 0)
  bomForm.status = bom?.status === 'inactive' ? 'inactive' : 'active'
  bomDrawerOpen.value = true
}

function copyProductionBomRecord(bom) {
  resetBomForm()
  bomForm.mode = 'copy'
  bomForm.source_id = Number(bom?.id || 0)
  bomForm.name = `${bom?.name || '生产 BOM'} 副本`
  bomForm.group_id = Number(bom?.group_id || 0)
  bomForm.group_category_id = Number(bom?.group_category_id || 0)
  bomForm.status = 'active'
  bomDrawerOpen.value = true
}

function closeBomDrawer() {
  bomDrawerOpen.value = false
  resetBomForm()
}

function openGroupCategoryDrawer(category = null) {
  if (!selectedProductionBomGroup.value) return
  if (category?.id) {
    categoryForm.id = Number(category.id || 0)
    categoryForm.name = category.name || ''
    categoryForm.sort_order = Number(category.sort_order || 100)
  } else {
    resetCategoryForm()
  }
  groupCategoryDrawerOpen.value = true
}

function closeGroupCategoryDrawer() {
  groupCategoryDrawerOpen.value = false
  resetCategoryForm()
}

function toggleBomGroupCategory(key) {
  const value = String(key || '')
  if (!value) return
  const next = new Set(collapsedBomGroupCategoryKeys.value)
  if (next.has(value)) next.delete(value)
  else next.add(value)
  collapsedBomGroupCategoryKeys.value = [...next]
}

function isBomGroupCategoryCollapsed(key) {
  return collapsedBomGroupCategoryKeys.value.includes(String(key || ''))
}

function editProductionBomGroup(group) {
  groupForm.id = Number(group?.id || 0)
  groupForm.name = group?.name || ''
  groupForm.sort_order = Number(group?.sort_order || 0)
}

function syncComponentTypeDefaults() {
  if (itemForm.component_type === 'finished_product') {
    itemForm.material_id = 0
    itemForm.consume_unit = 'g_per_bag'
    itemForm.ratio_pct = ''
    if (!Number(itemForm.qty_per_unit || 0) && isDripProduct(selectedProduct.value)) {
      itemForm.qty_per_unit = Number(selectedProduct.value.drip_bag_grams || 0) || ''
    }
    return
  }
  itemForm.component_product_id = 0
  itemForm.component_spec_g = 0
  itemForm.consume_unit = 'ratio_pct'
  itemForm.qty_per_unit = ''
}

function resetItemForm() {
  itemForm.component_type = 'material'
  itemForm.material_id = 0
  itemForm.component_product_id = 0
  itemForm.component_spec_g = 0
  itemForm.consume_unit = 'ratio_pct'
  itemForm.qty_per_unit = ''
  itemForm.ratio_pct = ''
}

function clearSelectedProductionBom() {
  selectedProductId.value = 0
	detail.value = null
  productionBomDetail.value = null
  selectedProductionBomRecord.value = null
	versions.value = []
  selectedProductionBomVersionID.value = 0
	updateUrl()
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'bom')
  const bomID = currentProductionBomID.value
  if (bomID) url.searchParams.set('production_bom_id', String(bomID))
  else url.searchParams.delete('production_bom_id')
  url.searchParams.delete('product_id')
  url.searchParams.delete('bom_filter_product_id')
  replaceHistoryURL(url)
}

function returnToProductConfig() {
  const navigation = bomReturnNavigation.value
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: String(navigation?.key || 'productMaster'),
      params: navigation?.params || (bomReturnProductID.value > 0 ? { open_product_config_id: bomReturnProductID.value } : {}),
    },
  }))
}

function openReferencedProductConfig(product) {
  const productID = Number(product?.product_id || product?.id || 0)
  if (!productID) return
  const bomID = currentProductionBomID.value
  const labelProductName = product?.product_name || `商品 #${productID}`
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'productMaster',
      params: { open_product_config_id: productID },
      returnNavigation: {
        key: 'bom',
        label: `返回BOM编辑：${currentProductionBomLabel.value || '生产 BOM'}`,
        params: bomID > 0 ? { production_bom_id: bomID } : {},
        source_label: `商品档案配置：${labelProductName}`,
        targetKey: 'productMaster',
      },
    },
  }))
}

async function loadAll() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [productData, materialData, mappingData, processData, productionGroupData, productionBomData] = await Promise.all([
      apiGet('/api/bom/products'),
      apiGet('/api/bom/materials'),
      apiGet('/api/bom/bag-spec-mappings'),
      apiGet('/api/process-templates'),
      apiGet('/api/production-bom-groups'),
      apiGet('/api/production-boms?status=all'),
    ])

    products.value = (productData || []).map(normalizeBomProduct)
    materials.value = materialData || []
    mappings.value = mappingData || []
    processTemplates.value = processData.rows || []
    productionBomGroups.value = productionGroupData || []
    productionBoms.value = (productionBomData.rows || productionBomData || []).map(normalizeProductionBomRecord)
    if (pendingProductionBomID.value > 0) {
      const pendingID = pendingProductionBomID.value
      pendingProductionBomID.value = 0
      await selectProductionBomRecordByID(pendingID)
      return
    }
    const selectedBomID = currentProductionBomID.value
    if (selectedBomID > 0) {
      const current = productionBoms.value.find((bom) => Number(bom.id || bom.production_bom_id || 0) === selectedBomID)
      if (current) await selectUnboundProductionBom(current)
      else clearSelectedProductionBom()
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadProductionBomVersions(bomID) {
  const id = Number(bomID || 0)
  if (!id) {
    productionBomDetail.value = null
    versions.value = []
    selectedProductionBomVersionID.value = 0
    return
  }
  productionBomDetail.value = await apiGet(`/api/production-boms/${id}`)
  versions.value = productionBomDetail.value?.versions || []
  syncSelectedProductionBomVersion()
}

async function loadProductionBomDetailForVersion(bomID, versionID = 0, fallbackRow = null) {
  const id = Number(bomID || 0)
  if (!id) return
  const query = Number(versionID || 0) > 0 ? `?version_id=${Number(versionID || 0)}` : ''
  productionBomDetail.value = await apiGet(`/api/production-boms/${id}${query}`)
  versions.value = productionBomDetail.value?.versions || []
  if (Number(versionID || 0) > 0) selectedProductionBomVersionID.value = Number(versionID || 0)
  else syncSelectedProductionBomVersion()
  const row = fallbackRow || selectedProductionBomRecord.value || {}
  detail.value = productionBomDetailAsRecipeDetail(productionBomDetail.value || {}, row)
  selectedProductionBomRecord.value = {
    ...bomRecordFromRow({ ...row, ...productionBomDetail.value }),
    id: detail.value.production_bom_id,
    production_bom_id: detail.value.production_bom_id,
    production_bom_code: detail.value.production_bom_code,
    production_bom_name: detail.value.production_bom_name,
    production_bom_group_id: detail.value.production_bom_group_id,
    production_bom_group_name: detail.value.production_bom_group_name,
    production_bom_group_category_id: detail.value.production_bom_group_category_id,
    production_bom_group_category_name: detail.value.production_bom_group_category_name,
    latest_bom_version_no: detail.value.latest_bom_version_no,
    production_bom_version_no: detail.value.production_bom_version_no,
    reference_product_count: productionBomDetail.value?.reference_product_count || row.reference_product_count || 0,
  }
}

async function selectProductionBomRecordByID(bomID) {
  const id = Number(bomID || 0)
  if (!id) return
  const record = productionBoms.value.find((row) => Number(row.id || row.production_bom_id || 0) === id) || { id, production_bom_id: id }
  await selectUnboundProductionBom(record)
}

async function selectUnboundProductionBom(row) {
  const record = bomRecordFromRow(row)
  if (!record.id) return
  selectedProductId.value = 0
  error.value = ''
  ok.value = ''
  selectedProductionBomRecord.value = {
    ...record,
    production_bom_id: record.id,
    production_bom_code: record.code,
    production_bom_name: record.name,
    production_bom_group_id: record.group_id,
    production_bom_group_name: record.group_name,
    production_bom_group_category_id: record.group_category_id,
    production_bom_group_category_name: record.group_category_name,
    latest_bom_version_no: row.latest_bom_version_no || row.latest_version_no || '',
    production_bom_version_no: row.production_bom_version_no || '',
  }
  try {
    await loadProductionBomDetailForVersion(record.id, 0, row)
  } catch (err) {
    detail.value = null
    productionBomDetail.value = null
    versions.value = []
    selectedProductionBomVersionID.value = 0
    error.value = err.message || '加载生产 BOM 配方失败'
  } finally {
    updateUrl()
  }
}

async function selectBomRow(row) {
  await selectUnboundProductionBom(row)
}

async function openBomRowPrimary(row) {
  await selectUnboundProductionBom(row)
}

function bomStatusLabel(status) {
  if (status === 'inactive') return '已失效'
  return '有效'
}

function productionBomRecordStatusLabel(status) {
  return status === 'inactive' ? '已失效' : '启用'
}

function processStatusLabel(status) {
  if (status === 'draft') return '草稿'
  if (status === 'active') return '已发布'
  if (status === 'inactive') return '已停用'
  return status || '-'
}

async function saveItem() {
  if (!canEditCurrentBomItems.value) return
  await mutate(async () => {
    const versionID = Number(selectedProductionBomVersionID.value || 0)
    const nextItems = detailItems.value.map(productionBomDraftItemFromItem)
    nextItems.push(productionBomDraftItemFromForm())
    await saveProductionBomDraftItems(nextItems)
    resetItemForm()
    ok.value = '已保存'
    await loadProductionBomDetailForVersion(currentProductionBomID.value, versionID)
  })
}

async function deleteItem(id) {
  if (!canEditCurrentBomItems.value) return
  await mutate(async () => {
    const versionID = Number(selectedProductionBomVersionID.value || 0)
    const deleteID = Number(id || 0)
    const nextItems = detailItems.value
      .filter((item) => Number(item.id || 0) !== deleteID)
      .map(productionBomDraftItemFromItem)
    await saveProductionBomDraftItems(nextItems)
    ok.value = '已删除'
    await loadProductionBomDetailForVersion(currentProductionBomID.value, versionID)
  })
}

async function saveMapping() {
  await mutate(async () => {
    await apiSend('/api/bom/bag-spec-mappings/save', {
      body: {
        spec_g: Number(mappingForm.spec_g || 0),
        material_id: Number(mappingForm.material_id || 0),
      },
    })
    mappingForm.material_id = 0
    ok.value = '已保存映射'
    await loadAll()
  })
}

async function deleteMapping(specG) {
  await mutate(async () => {
    await apiSend('/api/bom/bag-spec-mappings/delete', { body: { spec_g: specG } })
    ok.value = '已删除映射'
    await loadAll()
  })
}

async function loadProductionBomGroupsForManagement() {
  managedProductionBomGroups.value = await apiGet('/api/production-bom-groups') || []
}

async function openGroupDrawer() {
  groupDrawerOpen.value = true
  resetGroupForm()
  await mutate(async () => {
    await loadProductionBomGroupsForManagement()
  })
}

function closeGroupDrawer() {
  groupDrawerOpen.value = false
  resetGroupForm()
}

async function saveProductionBomGroup() {
  const payload = { name: groupForm.name, sort_order: Number(groupForm.sort_order || 0) }
  await mutate(async () => {
    if (groupForm.id) {
      await apiSend(`/api/production-bom-groups/${groupForm.id}`, { method: 'PUT', body: payload })
      ok.value = '已保存分组'
    } else {
      await apiSend('/api/production-bom-groups', { body: payload })
      ok.value = '已新增分组'
    }
    resetGroupForm()
    await Promise.all([loadProductionBomGroupsForManagement(), loadAll()])
  })
}

async function saveProductionBomGroupCategory() {
  const groupID = Number(selectedProductionBomGroupID.value || 0)
  const payload = { name: categoryForm.name, sort_order: Number(categoryForm.sort_order || 0) }
  if (!groupID || !payload.name) return
  await mutate(async () => {
    if (categoryForm.id) {
      await apiSend(`/api/production-bom-group-categories/${categoryForm.id}`, { method: 'PUT', body: payload })
      ok.value = '已保存小分类'
    } else {
      await apiSend(`/api/production-bom-groups/${groupID}/categories`, { body: payload })
      ok.value = '已新增小分类'
    }
    closeGroupCategoryDrawer()
    await loadAll()
  })
}

async function deleteProductionBomGroupCategory(category) {
  const categoryID = Number(category?.id || 0)
  if (!categoryID) return
  const okToDelete = window.confirm(`确认删除组内分类「${category?.name || categoryID}」？分类下 BOM 会回到该大组的未分类。`)
  if (!okToDelete) return
  await mutate(async () => {
    await apiSend(`/api/production-bom-group-categories/${categoryID}`, { method: 'DELETE' })
    ok.value = '已删除小分类'
    if (Number(selectedProductionBomGroupCategoryID.value || 0) === categoryID) selectedProductionBomGroupCategoryID.value = 0
    await loadAll()
  })
}

async function deleteProductionBomGroup(group) {
  const groupID = Number(group?.id || 0)
  if (!groupID) return
  const okToDelete = window.confirm(`确认删除分组「${group?.name || groupID}」？分组下 BOM 会移到未分类，配方和商品绑定不受影响。`)
  if (!okToDelete) return
  await mutate(async () => {
    await apiSend(`/api/production-bom-groups/${group.id}`, { method: 'DELETE' })
    ok.value = '已删除分组'
    if (selectedProductionBomGroupID.value === groupID) selectedProductionBomGroupID.value = 0
    await Promise.all([loadProductionBomGroupsForManagement(), loadAll()])
  })
}

async function moveSelectedProductBomsToGroup() {
  const targetGroupID = Number(selectedBomMoveGroupID.value || 0)
  const records = selectedBomRecordsForMove.value.filter((bom) => Number(bom.group_id || 0) !== targetGroupID)
  if (!records.length) return
  await mutate(async () => {
    for (const bom of records) {
      await apiSend(`/api/production-boms/${bom.id}`, {
        method: 'PUT',
        body: {
          name: bom.name,
          group_id: targetGroupID,
          group_category_id: 0,
          status: bom.status === 'inactive' ? 'inactive' : 'active',
        },
      })
    }
    ok.value = `已移动 ${records.length} 个 BOM`
    selectedBomRowKeys.value = []
    await loadAll()
  })
}

async function moveSelectedProductBomsToGroupCategory() {
  const targetGroupID = Number(selectedProductionBomGroupID.value || 0)
  const targetCategoryID = Number(selectedProductionBomGroupCategoryID.value || 0)
  if (!targetGroupID) return
  const records = selectedBomRecordsForMove.value.filter((bom) => {
    return Number(bom.group_id || 0) !== targetGroupID || Number(bom.group_category_id || 0) !== targetCategoryID
  })
  if (!records.length) return
  await mutate(async () => {
    for (const bom of records) {
      await apiSend(`/api/production-boms/${bom.id}`, {
        method: 'PUT',
        body: {
          name: bom.name,
          group_id: targetGroupID,
          group_category_id: targetCategoryID,
          status: bom.status === 'inactive' ? 'inactive' : 'active',
        },
      })
    }
    ok.value = `已移动 ${records.length} 个 BOM 到小分类`
    selectedBomRowKeys.value = []
    await loadAll()
  })
}

async function deactivateProductionBomRecords(records, successText) {
  await mutate(async () => {
    for (const bom of records) {
      await apiSend(`/api/production-boms/${bom.id}`, {
        method: 'PUT',
        body: {
          name: bom?.name || '',
          group_id: Number(bom?.group_id || 0),
          group_category_id: Number(bom?.group_category_id || 0),
          status: 'inactive',
        },
      })
    }
    ok.value = successText
    selectedBomRowKeys.value = []
    await loadAll()
    if (currentProductionBomID.value) await selectProductionBomRecordByID(currentProductionBomID.value)
  })
}

async function deactivateSelectedProductionBoms() {
  const records = selectedActiveBomRecordsForDeactivate.value
  if (!records.length) return
  await deactivateProductionBomRecords(records, `已失效 ${records.length} 个生产 BOM`)
}

async function moveProductionBomGroup(group, direction) {
  const groupID = Number(group?.id || 0)
  if (!groupID) return
  await mutate(async () => {
    await apiSend(`/api/production-bom-groups/${groupID}/move`, {
      body: { sort_order: Math.max(0, Number(group?.sort_order || 0) + Number(direction || 0)) },
    })
    ok.value = '已调整分组顺序'
    await Promise.all([loadProductionBomGroupsForManagement(), loadAll()])
  })
}

async function saveProductionBomRecord() {
  const name = String(bomForm.name || '').trim()
  if (!name) return
  const payload = {
    name,
    group_id: Number(bomForm.group_id || 0),
    group_category_id: Number(bomForm.group_category_id || 0),
    status: bomForm.status === 'inactive' ? 'inactive' : 'active',
  }
  await mutate(async () => {
    if (bomForm.mode === 'edit') {
      await apiSend(`/api/production-boms/${bomForm.id}`, { method: 'PUT', body: payload })
      ok.value = '已保存生产 BOM'
    } else if (bomForm.mode === 'copy') {
      const copied = await apiSend(`/api/production-boms/${bomForm.source_id}/copy`, { body: { name: payload.name, group_id: payload.group_id, group_category_id: payload.group_category_id } })
      ok.value = '已复制生产 BOM'
      pendingProductionBomID.value = Number(copied?.id || 0)
    } else {
      const created = await apiSend('/api/production-boms', { body: { name: payload.name, group_id: payload.group_id, group_category_id: payload.group_category_id } })
      ok.value = '已新建生产 BOM'
      pendingProductionBomID.value = Number(created?.id || 0)
    }
    closeBomDrawer()
    await loadAll()
  })
}

async function deactivateProductionBomRecord(bom) {
  const bomID = Number(bom?.id || 0)
  if (!bomID || bom?.status === 'inactive') return
  await deactivateProductionBomRecords([bom], '已失效生产 BOM')
}

async function createVersion() {
  await copyVersionAsDraft()
}

async function copyVersionAsDraft(version = selectedProductionBomVersion.value) {
  if (!canEditCurrentBomProduct.value) return
  await mutate(async () => {
    const bomID = currentProductionBomID.value
    const sourceVersionID = Number(version?.id || 0)
    if (bomID) {
      const created = await apiSend(`/api/production-boms/${bomID}/versions`, { body: { note: versionNote.value, source_version_id: sourceVersionID } })
      selectedProductionBomVersionID.value = Number(created?.id || 0)
    } else {
      await apiSend('/api/bom/versions', { body: { product_id: selectedProductId.value, note: versionNote.value } })
    }
    versionNote.value = ''
    ok.value = bomID ? '已复制为新版草稿' : '已保存版本'
    if (bomID) await loadProductionBomDetailForVersion(bomID, selectedProductionBomVersionID.value)
  })
}

async function activateVersion(id) {
  if (!canEditCurrentBomProduct.value) return
  await mutate(async () => {
    const bomID = currentProductionBomID.value
    if (currentProductionBomID.value) {
      await apiSend(`/api/production-bom-versions/${id}/publish`, { body: {} })
      ok.value = '已发布版本'
    } else {
      await apiSend(`/api/bom/versions/${id}/activate`, { body: {} })
      ok.value = '已启用版本'
    }
    await loadAll()
    if (bomID) await loadProductionBomDetailForVersion(bomID, id)
  })
}

async function mutate(action) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await action()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  const params = new URL(window.location.href).searchParams
  pendingProductionBomID.value = Number(props.viewParams?.production_bom_id || props.viewParams?.bom_id || params.get('production_bom_id') || params.get('bom_id') || 0)
  await loadAll()
  await restoreBomFormDraft()
})

onBeforeUnmount(saveBomFormDraft)

watch(selectedProductionBomGroupID, (groupID) => {
  selectedBomRowKeys.value = []
  selectedBomMoveGroupID.value = Number(groupID || 0) > 0 ? Number(groupID || 0) : 0
  selectedProductionBomGroupCategoryID.value = 0
})

watch([productionBomStatusFilter, productionBomSearchQuery], () => {
  selectedBomRowKeys.value = []
})

watch(selectedProductId, () => {
  if (itemForm.component_type === 'finished_product' && isDripProduct(selectedProduct.value)) {
    itemForm.qty_per_unit = Number(selectedProduct.value.drip_bag_grams || 0) || ''
  }
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.bom-return-banner { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; padding: 10px 12px; border: 1px solid #c7d2fe; border-radius: 8px; background: #eef2ff; color: #1e3a8a; }
.bom-return-banner span { font-size: 13px; }
.bom-return-button { border-color: #1e40af; color: #1e40af; font-weight: 700; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters, .inline-form, .summary { display: flex; align-items: end; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; align-items: center; margin-bottom: 12px; }
.panel-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.compact-title { margin-bottom: 4px; }
h2 { margin: 0; font-size: 20px; }
h3 { margin: 2px 0 4px; font-size: 18px; }
.grid { display: grid; grid-template-columns: minmax(360px, 0.9fr) minmax(420px, 1.1fr); gap: 14px; align-items: start; }
label span, .summary span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; min-width: 180px; }
input, select { height: 38px; }
textarea { width: 100%; min-height: 148px; resize: vertical; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 13px; line-height: 1.45; }
textarea[readonly] { background: #f8f7f5; color: #555; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.secondary.active { background: #1f1f1f; color: #fff; }
.compact-action { height: 32px; padding: 0 10px; font-size: 12px; }
.danger-outline { border-color: #9d2626; color: #9d2626; }
.danger-text { color: #9d2626; margin-left: 10px; }
.text-button { height: 30px; border: 0; background: transparent; color: #1f4f82; padding: 0; }
.text-button + .text-button { margin-left: 10px; }
.bom-action-row { display: flex; align-items: flex-end; gap: 12px; flex-wrap: wrap; }
.bom-search-field input { min-width: min(340px, 100%); }
.bom-product-filter { min-width: min(280px, 100%); }
.bom-list-head { align-items: flex-start; }
.bom-list-filters { display: flex; align-items: flex-end; gap: 12px; flex-wrap: wrap; margin: 6px 0 12px; padding-top: 10px; border-top: 1px solid #eee8df; }
.bom-batch-deactivate-action { margin-left: auto; }
.bom-list-tabs-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin: 8px 0; }
.bom-list-toolbar { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; padding: 10px; margin: 8px 0; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; }
.toolbar-divider { align-self: center; color: #666; font-size: 12px; font-weight: 700; padding-left: 8px; border-left: 1px solid #ddd6ce; }
.group-category-move-controls { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; }
.bom-list-tabs { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.bom-list-panel-scroll { max-height: min(62vh, 720px); overflow: auto; }
.bom-name-button { height: auto; min-height: 30px; text-align: left; font-weight: 700; }
.bom-record-form { align-items: flex-end; }
.bom-group-strip { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; width: 100%; }
.bom-group-strip > span { color: #666; font-size: 12px; font-weight: 700; }
.bom-focus-filter { align-self: stretch; display: inline-flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid #dbeafe; border-radius: 8px; background: #eff6ff; color: #1d4ed8; font-size: 13px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 640px; border-collapse: collapse; }
.compact table { min-width: 520px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.select-col { width: 42px; text-align: center; }
.select-col input { min-width: 0; width: 16px; height: 16px; }
tbody tr.active { background: #f3f7fb; }
.list-panel tbody tr { cursor: pointer; }
.classification-group-row td { background: #f8f7f5; color: #333; border-top: 1px solid #e6e0d8; }
.classification-group-row strong { margin: 0 8px; }
.category-toggle { font-size: 12px; }
.summary { align-items: stretch; margin-bottom: 12px; }
.summary div { min-width: 120px; border: 1px solid #eee8df; border-radius: 6px; padding: 9px; }
.summary strong { font-size: 16px; }
.linked-processes { display: flex; flex-wrap: wrap; gap: 8px; margin: -4px 0 12px; }
.referenced-product-button { cursor: pointer; font: inherit; }
.referenced-product-button:hover, .referenced-product-button:focus-visible { border-color: #1f4f82; color: #1f4f82; background: #eef6ff; outline: none; }
.version-attrs-panel { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; margin-top: 12px; background: #fbfaf8; }
.section-title-row { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 12px; }
.detail-subpanel { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; margin: 12px 0; background: #fbfaf8; }
.detail-subpanel h3 { margin: 0 0 4px; font-size: 16px; }
.version-recipe-panel { border-top: 1px solid #e6e0d8; margin-top: 14px; padding-top: 14px; }
.version-ratio-box { min-width: 120px; border: 1px solid #eee8df; border-radius: 6px; padding: 8px 10px; background: #fff; }
.version-ratio-box span { display: block; color: #666; font-size: 12px; margin-bottom: 4px; }
.version-ratio-box strong { font-size: 16px; }
.inline-actions { display: flex; gap: 8px; align-items: flex-end; flex-wrap: wrap; justify-content: flex-end; }
.inline-actions input { min-width: min(280px, 100%); }
.compact-form { align-items: end; }
.compact-action { min-height: 34px; }
.attrs-grid { display: grid; grid-template-columns: repeat(2, minmax(260px, 1fr)); gap: 12px; align-items: start; }
.warning-banner { border: 1px solid #e8c28f; border-radius: 6px; background: #fff8eb; color: #8a4b00; padding: 9px; margin-bottom: 12px; }
.bom-source-banner { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.inline-form { margin: 12px 0; }
.muted { color: #666; text-align: center; }
.muted.left { text-align: left; margin: 0; font-size: 13px; }
.empty { padding: 22px; border: 1px dashed #d8d0c7; border-radius: 8px; }
.warn { color: #a13b00; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.status-pill { display: inline-flex; align-items: center; min-height: 24px; border: 1px solid #cfd8cf; border-radius: 999px; padding: 2px 8px; color: #27602e; background: #f2fbf2; white-space: nowrap; }
.status-pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
.status-pill.readonly { border-color: #d6d0c7; color: #5f5a54; background: #f8f7f5; }
.drawer-mask { position: fixed; inset: 0; z-index: 80; background: rgba(20, 20, 20, .28); display: flex; justify-content: flex-end; }
.drawer { width: min(560px, 100vw); height: 100%; background: #fff; box-shadow: -8px 0 28px rgba(0,0,0,.16); padding: 18px; overflow: auto; }
.drawer-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 14px; }
@media (max-width: 1100px) { .grid { grid-template-columns: 1fr; } }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .attrs-grid { grid-template-columns: 1fr; }
  table { min-width: 620px; }
}
</style>
