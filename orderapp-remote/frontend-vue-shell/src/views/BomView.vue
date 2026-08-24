<template>
  <div class="page">
    <div v-if="bomReturnNavigation" class="bom-return-banner">
      <button class="secondary bom-return-button" type="button" @click="returnToProductConfig">{{ bomReturnLabel }}</button>
      <span>完成 BOM 明细维护后可回到来源操作界面。</span>
    </div>
    <section class="panel">
      <div class="panel-head">
        <h2>生产 BOM（制造主档）</h2>
        <div class="panel-actions">
          <button class="secondary" type="button" @click="openSpecTemplateDrawer" :disabled="loading">BOM 规格模板</button>
          <button class="secondary" type="button" @click="loadAll" :disabled="loading || productionBomCategoryMoveActive">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <p class="muted left">普通生产 BOM 统一声明产出对象（商品或物料）、产出数量和组件清单；制造阶段由所选 BOM 决定。</p>
    </section>

    <div v-if="bomDrawerOpen" class="drawer-mask" @click.self="closeBomDrawer">
      <aside class="drawer bom-settings-drawer" data-bom-settings-drawer>
        <div class="drawer-head">
          <div>
            <h3>{{ bomFormTitle }}</h3>
            <p class="muted left">声明商品或物料产出，并在同一设置抽屉中维护版本、配方、工艺和损耗。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeBomDrawer">关闭</button>
        </div>
        <!-- compatibility: bomForm.mode !== 'edit' && (!Number(bomForm.spec_template_version_id) || !Number(bomForm.main_input_material_id)) -->
        <form class="inline-form bom-record-form" @submit.prevent="saveProductionBomRecord">
          <label>
            <span>BOM名称</span>
            <input v-model.trim="bomForm.name" placeholder="例如 精品拼配" @input="markBomWorkspaceDirty" />
          </label>
          <label>
            <span>产出对象类型</span>
            <select v-model="bomForm.output_type" :disabled="bomForm.mode === 'edit' && !canEditBomFormOutputBasis" @change="syncBomOutputType">
              <option value="product">商品</option>
              <option value="material">物料</option>
            </select>
          </label>
          <label>
            <span>产出对象</span>
            <SearchableSelect
              v-model="bomForm.output_id"
              :options="outputTargetOptions"
              :option-label="outputTargetOptionLabel"
              :option-meta="outputTargetOptionMeta"
              :option-value="optionNumericValue"
              :placeholder="bomForm.output_type === 'material' ? '选择产出物料' : '选择产出商品'"
              :empty-text="bomForm.output_type === 'material' ? '没有可用物料档案' : '没有可用商品档案'"
              @update:model-value="markBomWorkspaceDirty" />
          </label>
          <label v-if="bomForm.output_type === 'material'">
            <span>产出数量</span>
            <input v-model.number="bomForm.output_qty" type="number" min="0.001" step="0.001" placeholder="例如 1" :disabled="!canEditBomFormOutputBasis" @input="markBomWorkspaceDirty" />
          </label>
          <label v-if="bomForm.output_type === 'material'">
            <span>产出单位</span>
            <input :value="outputUnitDisplay" disabled />
            <small>{{ outputUnitSourceHint }}</small>
            <small v-if="outputUnitMismatchWarning" class="warn">{{ outputUnitMismatchWarning }}</small>
          </label>
          <label v-if="bomForm.output_type === 'product' && (bomForm.mode !== 'edit' || !bomVariants.length)" class="bom-spec-template-field">
            <span>BOM 规格模板</span>
            <select v-model.number="bomForm.spec_template_version_id">
              <option :value="0">选择已发布模板版本</option>
              <option v-for="row in specTemplateVersionOptions" :key="row.version_id" :value="row.version_id">{{ row.label }}</option>
            </select>
            <small>创建时复制为 BOM 专属规格组；后续模板修改不影响本 BOM。</small>
          </label>
          <label v-if="bomForm.output_type === 'product' && (bomForm.mode !== 'edit' || !bomVariants.length)">
            <span>规格主体物料</span>
            <SearchableSelect
              v-model="bomForm.main_input_material_id"
              :options="materialComponentOptions"
              :option-label="materialOptionLabel"
              :option-value="optionNumericValue"
              placeholder="选择规格主体物料"
              empty-text="没有匹配物料" />
            <small>模板中的规格用量占位符会复制为该物料；每个规格仍可继续编辑完整配方。</small>
          </label>
          <label v-if="bomForm.mode === 'edit'">
            <span>状态</span>
            <select v-model="bomForm.status" @change="markBomWorkspaceDirty">
              <option value="active">启用</option>
              <option value="inactive">已失效</option>
            </select>
          </label>
          <div class="bom-record-form-action">
            <span class="bom-record-form-action-spacer" aria-hidden="true">操作</span>
            <button v-if="['create', 'copy'].includes(bomForm.mode)" class="primary" type="submit" :disabled="loading || !bomForm.name || !Number(bomForm.output_id || 0) || (bomForm.output_type === 'product' && !bomVariants.length && (!Number(bomForm.spec_template_version_id || 0) || !Number(bomForm.main_input_material_id || 0)))">{{ bomForm.mode === 'copy' ? '复制 BOM' : '保存 BOM 草稿' }}</button>
          </div>
        </form>
        <div id="bom-settings-detail-target" class="bom-settings-detail-target" aria-label="BOM 明细、BOM版本、配方明细"></div>
        <div v-if="bomForm.mode === 'edit'" class="bom-draft-footer">
          <span :class="bomWorkspaceDirty ? 'warn' : 'muted'">{{ bomWorkspaceDirty ? '未保存改动' : '草稿已保存' }}</span>
          <button class="primary" type="button" :disabled="loading || !bomWorkspaceDirty" @click="saveProductionBomRecord">保存 BOM 草稿</button>
        </div>
      </aside>
    </div>

    <div v-if="specTemplateDrawerOpen" class="drawer-mask" @click.self="closeSpecTemplateDrawer">
      <aside class="drawer spec-template-drawer" aria-label="BOM 规格模板管理">
        <div class="drawer-head">
          <div>
            <h3>BOM 规格模板</h3>
            <p class="muted left">模板是可复用蓝图；商品 BOM 创建时复制成专属规格组，发布后的 BOM 不随模板变化。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeSpecTemplateDrawer">关闭</button>
        </div>
        <div class="spec-template-workspace">
          <section class="spec-template-list">
            <form class="inline-form" @submit.prevent="createSpecTemplate">
              <label class="wide-field">
                <span>新模板名称</span>
                <input v-model.trim="specTemplateNewName" placeholder="例如 熟豆袋装规格组" />
              </label>
              <button class="primary" type="submit" :disabled="loading || !specTemplateNewName">新建模板</button>
            </form>
            <button
              v-for="row in productionBomSpecTemplates"
              :key="row.id"
              :class="['spec-template-list-row', { active: Number(selectedSpecTemplateID) === Number(row.id) }]"
              type="button"
              @click="selectSpecTemplate(row.id)">
              <strong>{{ row.name }}</strong>
              <small>{{ specTemplateSummary(row) }}</small>
            </button>
            <p v-if="!productionBomSpecTemplates.length" class="empty">暂无 BOM 规格模板</p>
          </section>
          <section v-if="selectedSpecTemplateDetail" class="spec-template-detail">
            <div class="section-title-row">
              <div>
                <h3>{{ selectedSpecTemplateDetail.name }}</h3>
                <p class="muted left">每个版本整组发布；任何一个规格无效都会回滚整组。</p>
              </div>
              <button class="secondary" type="button" :disabled="loading" @click="createSpecTemplateDraft">新增草稿版本</button>
            </div>
            <div class="version-row-list">
              <button
                v-for="version in selectedSpecTemplateDetail.versions || []"
                :key="version.id"
                :class="['secondary', { active: Number(selectedSpecTemplateVersionID) === Number(version.id) }]"
                type="button"
                @click="selectSpecTemplateVersion(version.id)">
                {{ version.version_no || `V${version.id}` }} · {{ productionBomVersionStatusLabel(version.status) }}
              </button>
            </div>
            <div v-if="selectedSpecTemplateVersion" class="spec-template-version-editor">
              <div class="section-title-row">
                <div>
                  <h3>规格配方（{{ templateDraftVariants.length }} 个）</h3>
                  <p class="muted left">每个规格固定产出 1 × 该规格库存单位；规格用量和包材均在本规格内维护。</p>
                </div>
                <div class="inline-actions">
                  <button v-if="selectedSpecTemplateVersion.status === 'draft'" class="secondary" type="button" @click="addTemplateVariant">添加规格</button>
                  <button v-if="selectedSpecTemplateVersion.status === 'draft'" class="primary" type="button" :disabled="loading" @click="saveSpecTemplateDraft">保存整组草稿</button>
                  <button v-if="selectedSpecTemplateVersion.status === 'draft'" class="secondary" type="button" :disabled="loading" @click="publishSpecTemplateVersion">整组发布</button>
                </div>
              </div>
              <article v-for="(variant, variantIndex) in templateDraftVariants" :key="variant.local_key || variant.spec_key || variantIndex" class="spec-variant-card">
                <div class="spec-variant-grid">
                  <label><span>规格名称</span><input v-model.trim="variant.name" :disabled="selectedSpecTemplateVersion.status !== 'draft'" placeholder="227g 袋装" /></label>
                  <label><span>库存单位</span><select v-model="variant.inventory_unit" :disabled="selectedSpecTemplateVersion.status !== 'draft'"><option value="">请选择</option><option v-for="unit in activeUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option></select></label>
                  <label><span>排序</span><input v-model.number="variant.sort_order" type="number" min="1" :disabled="selectedSpecTemplateVersion.status !== 'draft'" /></label>
                  <label class="checkbox-row compact-checkbox"><input v-model="variant.is_default" type="checkbox" :disabled="selectedSpecTemplateVersion.status !== 'draft'" @change="setTemplateDefaultVariant(variantIndex)" /><span>默认规格</span></label>
                  <label><span>工艺路线</span><select v-model.number="variant.process_route_id" :disabled="selectedSpecTemplateVersion.status !== 'draft'"><option :value="0">未配置</option><option v-for="route in processRoutes" :key="route.id" :value="route.id">{{ route.name }}</option></select></label>
                  <label><span>规格用量</span><input v-model.number="variant.main_input_qty" type="number" min="0.001" step="0.001" :disabled="selectedSpecTemplateVersion.status !== 'draft'" /></label>
                </div>
                <div class="spec-variant-components">
                  <strong>包材与其他组件</strong>
                  <p :class="['recipe-mode-hint', { warn: templateVariantRecipeMode(variant) === 'mixed_legacy' }]">
                    本规格配方模式：{{ templateVariantRecipeModeLabel(variant) }}。商品规格组件只能使用固定用量；物料固定用量必须使用该物料的库存单位。
                  </p>
                  <div v-for="(item, itemIndex) in variant.items" :key="item.local_key || itemIndex" class="spec-component-row">
                    <select v-model="item.component_type" :disabled="selectedSpecTemplateVersion.status !== 'draft'" @change="syncTemplateVariantItemComponentType(variantIndex, itemIndex)">
                      <option value="material">物料</option>
                      <option value="product" :disabled="templateVariantProductComponentDisabled(variant, item)">商品规格组件</option>
                    </select>
                    <SearchableSelect
                      v-if="item.component_type === 'product'"
                      v-model="item.component_product_id"
                      :options="productComponentOptions"
                      :option-label="optionLabel"
                      :option-meta="optionMeta"
                      :option-value="optionNumericValue"
                      placeholder="选择商品"
                      empty-text="没有可用商品组件"
                      :disabled="selectedSpecTemplateVersion.status !== 'draft'"
                      @update:model-value="loadTemplateComponentProductSpecs(variantIndex, itemIndex, $event)" />
                    <SearchableSelect
                      v-else
                      v-model="item.material_id"
                      :options="materialComponentOptions"
                      :option-label="materialOptionLabel"
                      :option-value="optionNumericValue"
                      placeholder="选择物料"
                      empty-text="没有匹配物料"
                      :disabled="selectedSpecTemplateVersion.status !== 'draft'"
                      @update:model-value="syncTemplateVariantItemMaterial(variantIndex, itemIndex)" />
                    <select
                      v-if="item.component_type === 'product'"
                      v-model.number="item.component_bom_spec_id"
                      :disabled="selectedSpecTemplateVersion.status !== 'draft' || !item.component_product_id || item.component_bom_spec_loading"
                      @change="syncTemplateVariantItemBomSpec(variantIndex, itemIndex)">
                      <option :value="0">请选择已发布规格</option>
                      <option v-for="spec in item.component_bom_spec_options" :key="spec.bom_spec_id" :value="spec.bom_spec_id">{{ componentBomSpecOptionLabel(spec) }}</option>
                    </select>
                    <select
                      v-model="item.consume_unit"
                      :disabled="selectedSpecTemplateVersion.status !== 'draft' || templateVariantItemConsumeUnitLocked(variant, item)"
                      @change="syncTemplateVariantItemConsumeMode(variantIndex, itemIndex)">
                      <option v-for="unit in templateVariantItemConsumeUnitOptions(variant, item)" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                    </select>
                    <input v-if="item.consume_unit === 'ratio_pct'" v-model.number="item.ratio_pct" type="number" min="0.01" max="100" step="0.01" :disabled="selectedSpecTemplateVersion.status !== 'draft'" />
                    <input v-else v-model.number="item.qty_per_unit" type="number" min="0.001" step="0.001" :disabled="selectedSpecTemplateVersion.status !== 'draft'" />
                    <button v-if="selectedSpecTemplateVersion.status === 'draft'" class="text-button danger-text" type="button" @click="removeTemplateVariantItem(variantIndex, itemIndex)">删除</button>
                  </div>
                  <button v-if="selectedSpecTemplateVersion.status === 'draft'" class="secondary compact-action" type="button" @click="addTemplateVariantItem(variantIndex)">添加组件</button>
                </div>
                <button v-if="selectedSpecTemplateVersion.status === 'draft'" class="text-button danger-text" type="button" @click="removeTemplateVariant(variantIndex)">删除该规格</button>
              </article>
              <p v-if="!templateDraftVariants.length" class="empty">请至少添加一个规格</p>
            </div>
          </section>
        </div>
      </aside>
    </div>

    <div class="grid">
      <section class="panel list-panel">
        <div class="panel-head bom-list-head" data-pr442-bom-business-groups>
          <div class="panel-title compact-title">生产 BOM列表</div>
        </div>
        <BusinessGroupInlineWorkspace
          v-model:collapsed-keys="collapsedProductionBomGroups"
          class="bom-business-group-inline-workspace"
          :groups="productionBomDisplayGroups"
          :move-active="productionBomCategoryMoveActive"
          :selected-count="selectedBomRecordsForMove.length"
          :can-move="canBeginProductionBomCategoryMove"
          :loading="loading || productionBomGroupFeatureSelectionSaving"
          count-unit="个 BOM"
          @move="beginProductionBomCategoryMove"
          @cancel="cancelProductionBomCategoryMove"
          @target="handleProductionBomCategoryMoveTarget"
          @manage="openBusinessGroupManagement"
          @configure="openProductionBomGroupFeatureSelectionDrawer">
          <template #toolbar-extra>
            <button class="primary compact-action" type="button" :disabled="loading || productionBomCategoryMoveActive" @click="openNewProductionBomRecord">新建生产 BOM</button>
          </template>
          <template #filters>
            <div class="bom-list-filters">
              <label>
                <span>状态</span>
                <select v-model="filters.status">
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
          </template>
          <template #group="{ group }">
            <template v-if="productionBomGroupShowsTable(group)">
              <div class="table-wrap bom-list-panel-scroll">
                <table data-auto-pagination="off">
                  <thead>
                    <tr>
                      <th class="select-col">
                        <input
                          type="checkbox"
                          :checked="isAllVisibleBomsSelected(group)"
                          :indeterminate.prop="isSomeVisibleBomsSelected(group)"
                          @change="toggleAllVisibleBoms($event, group.rows)" />
                      </th>
                      <th>BOM名称</th>
                      <th>状态</th>
                      <th>组件数</th>
                      <th>产出对象</th>
                      <th>更新时间</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="row in group.rows"
                      :key="`${group.key}-${bomRowKey(row)}`"
                      :class="{ active: bomRowKey(row) === activeBomRowKey }">
                      <td class="select-col">
                        <input
                          v-model="selectedBomRowKeys"
                          type="checkbox"
                          :value="bomRowKey(row)"
                          :disabled="productionBomCategoryMoveActive || !isMovableBomRow(row)"
                          @click.stop />
                      </td>
                      <td>
                        <button class="text-button bom-name-button" type="button" :disabled="productionBomCategoryMoveActive" @click.stop="openBomRowPrimary(row)">
                          {{ productionBomListName(row) }}
                        </button>
                        <small v-if="productionBomVersionWarning(row)" class="bom-version-warning" data-warning-prefix="当前引用">{{ productionBomVersionWarning(row) }}</small>
                      </td>
                      <td><span :class="['status-pill', row.status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.status) }}</span></td>
                      <td>{{ row.item_count || row.material_count || 0 }}</td>
                      <td>{{ productionBomOutputLabel(row) }}</td>
                      <td>{{ row.updated_at }}</td>
                      <td>
                        <button class="text-button" type="button" :disabled="productionBomCategoryMoveActive || !Number(row.production_bom_id || row.id || 0)" @click.stop="copyProductionBomRecord(bomRecordFromRow(row))">复制</button>
                        <button class="text-button danger-text" type="button" :disabled="productionBomCategoryMoveActive || !Number(row.production_bom_id || row.id || 0) || row.status === 'inactive'" @click.stop="deactivateProductionBomRecord(bomRecordFromRow(row))">失效</button>
                      </td>
                    </tr>
                    <tr v-if="!group.rows.length">
                      <td colspan="7" class="muted">{{ productionBomRows.length ? '当前分类暂无 BOM' : '暂无配方档案' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <PaginationControls
                v-if="group.total > 0"
                :page="group.page"
                :page-size="group.pageSize"
                :total="group.total"
                :disabled="loading || productionBomCategoryMoveActive"
                @change="handleProductionBomGroupPaginationChange(group.key, $event)" />
            </template>
          </template>
        </BusinessGroupInlineWorkspace>
      </section>

      <Teleport v-if="bomDrawerOpen && bomForm.mode === 'edit'" to="#bom-settings-detail-target">
      <section class="bom-settings-detail">
        <div class="panel-title">BOM 明细</div>
        <div v-if="detail" class="summary">
          <div><span>生产 BOM</span><strong>{{ currentProductionBomLabel }}</strong></div>
          <div><span>产出对象</span><strong>{{ productionBomOutputLabel(detail) }}</strong></div>
          <div><span>产出数量</span><strong>{{ currentOutputBasisLabel }}</strong></div>
          <div v-if="outputUnitMismatchWarning"><span>单位提示</span><strong class="warn">{{ outputUnitMismatchWarning }}</strong></div>
          <div><span>多层展开</span><strong>{{ usedByBoms.length ? `${usedByBoms.length} 个上层 BOM` : '可作为库存件' }}</strong></div>
          <div v-if="currentProductionBomWarning"><span>版本提示</span><strong class="warn">{{ currentProductionBomWarning }}</strong></div>
          <div><span>工艺参数</span><strong>{{ detail.roast_level || '-' }}</strong></div>
          <div><span>状态</span><strong :class="{ warn: detail.status === 'inactive' }">{{ bomStatusLabel(detail.status) }}</strong></div>
        </div>
        <div v-if="detail" class="linked-processes bom-default-actions">
          <button class="secondary compact-action" type="button" :disabled="!canSetCurrentBomAsDefault || loading" @click="setCurrentProductionBomAsDefault">设为产出对象默认 BOM</button>
          <span class="status-pill readonly">使用版本 {{ currentProductionBomDefaultVersion?.version_no || '-' }}</span>
        </div>
        <div v-if="detail && referencedProducts.length" class="linked-processes referenced-products">
          <button
            v-for="product in referencedProducts"
            :key="referencedProductKey(product)"
            type="button"
            class="status-pill referenced-product-button"
            @click="openReferencedProductConfig(product)">
            {{ product.product_name || `商品 #${product.product_id}` }}<template v-if="product.product_code"> · {{ product.product_code }}</template>
          </button>
        </div>
        <div v-if="detail && usedByBoms.length" class="detail-subpanel compact-subpanel">
          <div class="section-title-row">
            <div>
              <h3>被哪些 BOM 使用</h3>
              <p class="muted left">这些上层 BOM 会把当前产出对象作为组件消耗。</p>
            </div>
          </div>
          <div class="table-wrap compact">
            <table>
              <thead>
                <tr>
                  <th>上层 BOM</th>
                  <th>上层产出对象</th>
                  <th>组件用量</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in usedByBoms" :key="`${row.bom_id}:${row.bom_version_id}:${row.consume_unit}`">
                  <td>{{ row.bom_code }} {{ row.bom_name }}<small>{{ row.bom_version_no }}</small></td>
                  <td>{{ productionBomOutputLabel(row) }}</td>
                  <td>{{ qty(row.qty_per_unit) }} {{ consumeUnitLabel(row.consume_unit) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
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
                  <th>组件数</th>
                  <th>产出基准</th>
                  <th>工艺路线</th>
                  <th>最新可用</th>
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
                  <td>{{ qty(version.output_qty || 1) }} {{ version.output_unit || 'unit' }}</td>
                  <td>{{ version.process_route_name || '未配置' }}</td>
                  <td>{{ version.is_latest_usable ? '是' : '-' }}</td>
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
                  <td colspan="9" class="muted">暂无版本</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="currentOutputIdentity.type === 'product' && (bomVariants.length || canEditCurrentBomItems)" class="bom-spec-group-panel">
            <div class="section-title-row">
              <div>
                <h3>规格组</h3>
                <p class="muted left">当前版本包含 {{ bomVariants.length }} 个 BOM 专属规格；规格、路线和配方统一保存，发布仍单独操作。</p>
              </div>
              <div v-if="canEditCurrentBomItems" class="inline-actions">
                <button class="secondary compact-action" type="button" :disabled="loading" @click="addProductionBomDraftVariant">添加规格</button>
                <button class="text-button danger-text" type="button" :disabled="loading || bomVariants.length <= 1 || !selectedBomVariant" @click="removeProductionBomDraftVariant">删除该规格</button>
              </div>
            </div>
            <div v-if="canEditCurrentBomItems" class="inline-form bom-spec-template-reapply-form">
              <label>
                <span>重新套用已发布模板</span>
                <select v-model.number="reapplySpecTemplateVersionID">
                  <option :value="0">选择已发布模板版本</option>
                  <option v-for="row in specTemplateVersionOptions" :key="row.version_id" :value="row.version_id">{{ row.label }}</option>
                </select>
                <small>会用模板整组替换当前草稿；同规格且单位未变时继续沿用原规格身份。</small>
              </label>
              <label>
                <span>规格主体物料</span>
                <SearchableSelect
                  v-model="reapplyMainInputMaterialID"
                  :options="materialComponentOptions"
                  :option-label="materialOptionLabel"
                  :option-value="optionNumericValue"
                  placeholder="选择规格主体物料"
                  empty-text="没有匹配物料" />
                <small>模板中的规格用量占位符会复制为该物料。</small>
              </label>
              <div class="reapply-action-cell">
                <span class="reapply-action-spacer" aria-hidden="true">操作</span>
                <button class="secondary" type="button" :disabled="loading || !Number(reapplySpecTemplateVersionID || 0) || !Number(reapplyMainInputMaterialID || 0)" @click="reapplyProductionBomSpecTemplate">重新套用模板</button>
              </div>
            </div>
            <div class="bom-variant-tabs">
              <button
                v-for="variant in bomVariants"
                :key="variant.local_key || variant.bom_variant_id || variant.id || variant.spec_key"
                :class="['secondary', { active: Number(selectedBomVariant?.bom_variant_id || selectedBomVariant?.id || 0) === Number(variant.bom_variant_id || variant.id || 0) }]"
                type="button"
                @click="selectBomVariant(variant)">
                {{ variant.name || variant.spec_name || variant.spec_key }} · 1 {{ variant.inventory_unit }}<template v-if="variant.is_default"> · 默认</template>
              </button>
            </div>
            <div v-if="selectedBomVariant" class="bom-spec-identity-form">
              <label>
                <span>规格编码（自动）</span>
                <input :value="selectedBomVariant.code || '保存后生成'" disabled />
              </label>
              <label>
                <span>规格名称</span>
                <input v-model.trim="selectedBomVariant.name" :disabled="!canEditCurrentBomItems" @input="markBomWorkspaceDirty" />
              </label>
              <label>
                <span>新条码（可选）</span>
                <input v-model.trim="selectedBomVariant.barcode" :disabled="!canEditCurrentBomItems" placeholder="不沿用旧子 SKU 条码" @input="markBomWorkspaceDirty" />
              </label>
              <label>
                <span>库存单位</span>
                <select v-model="selectedBomVariant.inventory_unit" :disabled="!canEditCurrentBomItems" @change="markBomWorkspaceDirty">
                  <option value="">请选择</option>
                  <option v-for="unit in activeUnitDefinitions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option>
                </select>
                <small>规格一经发布，库存单位不可修改；变更单位请新增规格。</small>
              </label>
              <label>
                <span>排序</span>
                <input v-model.number="selectedBomVariant.sort_order" type="number" min="0" step="1" :disabled="!canEditCurrentBomItems" @input="markBomWorkspaceDirty" />
              </label>
              <div class="identity-action-cell">
                <span class="identity-action-spacer" aria-hidden="true">操作</span>
                <button v-if="!selectedBomVariant.is_default" class="secondary compact-action" type="button" :disabled="!canEditCurrentBomItems" @click="makeSelectedBomVariantDefault">设为默认规格</button>
                <span v-else class="status-pill readonly">默认规格</span>
              </div>
            </div>
          </div>
              <div class="version-recipe-panel">
                <!-- 合计比例卡片已删除；历史“保存组件”入口改为本地添加到当前配方。 -->
                <div class="section-title-row">
              <div>
                <h3>{{ selectedBomVariant ? `${selectedBomVariant.name || selectedBomVariant.spec_name || selectedBomVariant.spec_key} · 完整配方` : '配方明细' }}</h3>
                <p class="muted left">当前编辑版本：{{ selectedProductionBomVersion?.version_no || '-' }} · {{ productionBomVersionStatusLabel(selectedProductionBomVersion?.status || '') }}</p>
              </div>
            </div>
            <div class="inline-form version-route-form">
              <label class="wide-field">
                <span>工艺路线</span>
                <select
                  :value="currentRecipeTarget ? Number(currentRecipeTarget.process_route_id || 0) : 0"
                  :disabled="!canEditCurrentBomItems"
                  @change="setSelectedProductionBomRouteID($event.target.value)">
                  <option value="0">未配置</option>
                  <option v-for="route in processRoutes" :key="route.id" :value="route.id">{{ route.name }}</option>
                </select>
                <small>{{ currentRecipeTarget?.process_route_name || '未配置路线' }}<template v-if="selectedProductionBomVersion?.is_latest_usable"> · 最新可用</template></small>
              </label>
              <div v-if="isMaterialOutputBom" class="material-loss-control bom-version-loss-control">
                <label class="checkbox-row compact-checkbox">
                  <input v-model="versionMaterialLossRateEnabled" type="checkbox" :disabled="!canEditCurrentBomItems" @change="handleVersionMaterialLossToggle" />
                  <span>原料损耗比</span>
                </label>
                <label v-if="versionMaterialLossRateEnabled" class="material-loss-rate-field">
                  <span>损耗比例 %</span>
                  <input v-model.number="versionMaterialLossRatePct" type="number" min="0" max="99.9999" step="0.01" :disabled="!canEditCurrentBomItems" />
                  <small>开启损耗后，所有组件必须是物料并使用比例 %；损耗比例必须大于 0。</small>
                </label>
              </div>
            </div>
            <p :class="['recipe-mode-hint', { warn: recipeConsumeMode === 'mixed_legacy' }]">
              配方模式：{{ recipeConsumeModeLabel }}。新配方使用固定用量；历史比例配方保持只读兼容。
            </p>
            <form class="inline-form" @submit.prevent="saveItem">
              <label>
                <span>组件来源</span>
                <select v-model="itemForm.component_type" :disabled="!detail || !canEditCurrentBomItems" @change="syncComponentTypeDefaults">
                  <option value="material">物料</option>
                  <option value="product" :disabled="recipeConsumeMode === 'ratio'">商品规格组件</option>
                </select>
              </label>
              <label>
                <span>{{ itemForm.component_type === 'product' ? '商品组件' : '物料' }}</span>
                <SearchableSelect
                  v-if="itemForm.component_type === 'product'"
                  v-model="itemForm.component_product_id"
                  :options="productComponentOptions"
                  :option-label="optionLabel"
                  :option-meta="optionMeta"
                  :option-value="optionNumericValue"
                  placeholder="选择半成品或成品商品"
                  empty-text="没有可用商品组件"
                  :disabled="!detail || !canEditCurrentBomItems"
                  @update:model-value="loadComponentProductSpecs" />
                <SearchableSelect
                  v-else
                  v-model="itemForm.material_id"
                  :options="materialComponentOptions"
                  :option-label="materialOptionLabel"
                  :option-value="optionNumericValue"
                  placeholder="选择物料"
                  empty-text="没有匹配物料"
                  :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <label v-if="itemForm.component_type === 'product'">
                <span>商品 BOM 规格</span>
                <select v-model.number="itemForm.component_bom_spec_id" :disabled="!detail || !canEditCurrentBomItems || !itemForm.component_product_id">
                  <option :value="0">请选择已发布规格</option>
                  <option v-for="spec in componentBomSpecOptions" :key="spec.bom_spec_id" :value="spec.bom_spec_id">
                    {{ componentBomSpecOptionLabel(spec) }}
                  </option>
                </select>
                <small>商品组件必须引用该商品默认已发布 BOM 中的明确规格。</small>
              </label>
              <label>
                <span>消耗单位</span>
                <select v-model="itemForm.consume_unit" :disabled="!detail || !canEditCurrentBomItems">
                  <option v-for="unit in currentConsumeUnitOptions" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                </select>
                <small>组件库存单位：{{ componentStockUnitLabel }}</small>
              </label>
              <label v-if="itemForm.consume_unit === 'ratio_pct'">
                <span>比例 %</span>
                <input v-model.number="itemForm.ratio_pct" type="number" min="0.01" max="100" step="0.01" :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <label v-else>
                <span>用量</span>
                <input v-model.number="itemForm.qty_per_unit" type="number" min="0.001" step="0.001" :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <button class="primary" type="submit" :disabled="!detail || loading || !canEditCurrentBomItems">添加到当前配方</button>
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
                  <tr v-for="(item, itemIndex) in detailItems" :key="productionBomDraftItemKey(item, itemIndex)">
                    <td>{{ componentTypeLabel(item.component_type) }}</td>
                    <td>{{ componentItemName(item) }}</td>
                    <td>{{ itemQuantityDisplay(item) }}</td>
                    <td><button class="text-button" type="button" :disabled="!canEditCurrentBomItems" @click="deleteItem(productionBomDraftItemKey(item, itemIndex))">删除</button></td>
                  </tr>
                  <tr v-if="!detailItems.length">
                    <td colspan="4" class="muted">暂无组件</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

      </section>
      </Teleport>
    </div>

    <div v-if="productionBomGroupFeatureDrawerOpen" class="drawer-mask" @click.self="productionBomGroupFeatureDrawerOpen = false">
      <aside class="drawer">
        <div class="drawer-head">
          <h3>生产 BOM 分组模板</h3>
          <button class="secondary compact-action" type="button" @click="productionBomGroupFeatureDrawerOpen = false">关闭</button>
        </div>
        <div class="feature-group-selection" data-feature-key="production_bom">
          <div class="feature-group-selection-copy">
            <strong>生产 BOM 使用的分组模板</strong>
            <small>可多选；保存后只有已选模板可用于 BOM 分类和移动。取消全部后按 BOM 平铺展示。</small>
          </div>
          <div class="feature-group-selection-options">
            <label v-for="template in selectableProductionBomGroupTemplates" :key="template.id" class="feature-group-selection-option">
              <input
                v-model="productionBomGroupFeatureSelectionDraft"
                type="checkbox"
                :value="Number(template.id || 0)"
                :disabled="productionBomGroupFeatureSelectionSaving || loading" />
              <span>{{ template.label }}</span>
            </label>
            <span v-if="!selectableProductionBomGroupTemplates.length" class="muted left">暂无可选分组模板，请先维护模板。</span>
          </div>
          <div class="feature-group-selection-actions">
            <button class="secondary compact-action" type="button" :disabled="productionBomGroupFeatureSelectionSaving || loading" @click="openBusinessGroupManagement">维护分组模板</button>
            <button class="primary compact-action" type="button" :disabled="productionBomGroupFeatureSelectionSaving || loading || !productionBomGroupFeatureSelectionHasChanges" @click="saveProductionBomFeatureSelection">
              {{ productionBomGroupFeatureSelectionSaving ? '保存中' : '保存模板选择' }}
            </button>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import BusinessGroupInlineWorkspace from '../components/BusinessGroupInlineWorkspace.vue'
import PaginationControls from '../components/PaginationControls.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import { assignVariantSpecKeys, bomProductOptionLabel, filterProductionBomCatalog, isBomProductCandidate, isProductionBomOutputProductCandidate, materialOptionLabel, nextSpecKey, productionBomDetailAsRecipeDetail, productionBomDraftItemKey, productionBomLabel, productionBomListName, productionBomOutputIdentity, productionBomOutputLabel, productionBomOutputPayload, productionBomVersionWarning, removeProductionBomDraftItem } from '../lib/bom'
import {
  businessGroupControlOptions,
  businessGroupFeatureSelectionIDs,
  businessGroupFeatureSelectionPayload,
  businessGroupInlineListState,
  businessGroupMoveAssignmentPayload,
  businessGroupRowsForFeatureSelection,
  businessGroupVisibleRows,
  groupRowsByBusinessGroupTemplates,
} from '../lib/business-grouping'
import { componentTypeLabel } from '../lib/drip-product'
import { FORM_DRAFT_SCOPES, readFormDraft, saveFormDraft } from '../lib/form-draft-cache'
import { isSystemDefaultBusinessGroup } from '../lib/product-settings'
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
const productUnitDefinitions = ref([])
const versions = ref([])
const processRoutes = ref([])
const productionBomBusinessGroups = ref([])
const productionBomDetail = ref(null)
const selectedProductionBomRecord = ref(null)
const detail = ref(null)
const selectedProductId = ref(0)
const selectedProductionBomVersionID = ref(0)
const productionBomCategoryMoveActive = ref(false)
const productionBomGroupFeatureSelectionTemplateIDs = ref([])
const productionBomGroupFeatureSelectionDraft = ref([])
const productionBomGroupFeatureSelectionSaving = ref(false)
const productionBomGroupFeatureDrawerOpen = ref(false)
const productionBomSpecTemplates = ref([])
const componentBomSpecOptions = ref([])
const specTemplateDrawerOpen = ref(false)
const specTemplateNewName = ref('')
const selectedSpecTemplateID = ref(0)
const selectedSpecTemplateDetail = ref(null)
const selectedSpecTemplateVersionID = ref(0)
const templateDraftVariants = ref([])
const selectedBomVariantID = ref(0)
const reapplySpecTemplateVersionID = ref(0)
const reapplyMainInputMaterialID = ref(0)
const selectedBomRowKeys = ref([])
const collapsedProductionBomGroups = ref([])
const productionBomPaginationByGroup = ref({})
const pendingProductionBomID = ref(0)
const bomReturnNavigation = computed(() => props.viewParams?.return_navigation || null)
const bomReturnProductID = computed(() => Number(bomReturnNavigation.value?.params?.open_product_config_id || 0))
const bomReturnLabel = computed(() => String(bomReturnNavigation.value?.label || '返回商品档案配置'))
const bomDrawerOpen = ref(false)
const filters = reactive({ status: 'active' })
const productionBomSearchQuery = ref('')
const loading = ref(false)
const error = ref('')
const ok = ref('')
const itemForm = reactive({
  component_type: 'material',
  material_id: 0,
  component_product_id: 0,
  component_bom_spec_id: 0,
  component_spec_g: 0,
  consume_unit: 'ratio_pct',
  qty_per_unit: '',
  ratio_pct: '',
})
const bomForm = reactive({ id: 0, source_id: 0, mode: 'create', name: '', output_type: 'product', output_id: 0, output_product_id: 0, output_material_id: 0, output_qty: 1, output_unit: 'unit', spec_template_version_id: 0, main_input_material_id: 0, status: 'active' })
const versionNote = ref('')
const bomWorkspaceDirty = ref(false)

const bomVariants = computed(() => Array.isArray(productionBomDetail.value?.variants) ? productionBomDetail.value.variants : [])
const selectedBomVariant = computed(() => bomVariants.value.find((variant) => Number(variant.bom_variant_id || variant.id || 0) === Number(selectedBomVariantID.value || 0)) || bomVariants.value[0] || null)
const currentRecipeTarget = computed(() => selectedBomVariant.value || selectedProductionBomVersion.value)
// productionBomDetail is the editable draft workspace; detail is only a
// projected/read-only compatibility view and must not own recipe mutations.
const detailItems = computed(() => selectedBomVariant.value?.items || productionBomDetail.value?.items || [])
const activeUnitDefinitions = computed(() => productUnitDefinitions.value.filter((unit) => unit.active !== false))
const selectedSpecTemplateVersion = computed(() => (selectedSpecTemplateDetail.value?.versions || []).find((version) => Number(version.id || 0) === Number(selectedSpecTemplateVersionID.value || 0)) || null)
const specTemplateVersionOptions = computed(() => {
  const rows = []
  for (const template of productionBomSpecTemplates.value || []) {
    const versions = Array.isArray(template.versions) ? template.versions : []
    for (const version of versions) {
      if (!['published', 'active'].includes(String(version.status || '').toLowerCase())) continue
      rows.push({
        version_id: Number(version.id || version.version_id || 0),
        template_id: Number(template.id || 0),
        label: `${template.name || '规格模板'} · ${version.version_no || `V${version.id || ''}`}`,
      })
    }
    const publishedVersionID = Number(template.published_version_id || template.latest_published_version_id || 0)
    if (publishedVersionID > 0 && !rows.some((row) => row.version_id === publishedVersionID)) {
      rows.push({ version_id: publishedVersionID, template_id: Number(template.id || 0), label: `${template.name || '规格模板'} · 已发布` })
    }
  }
  return rows
})
const recipeConsumeMode = computed(() => recipeModeForItems(detailItems.value))
const recipeConsumeModeLabel = computed(() => ({
  empty: '尚未选择（第一条组件决定）',
  ratio: '全部按比例 %',
  fixed: '全部按各组件库存单位固定用量',
  mixed_legacy: '历史混合配方，需拆分后才能保存或发布',
})[recipeConsumeMode.value] || '尚未选择')
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && Number(props.customerContextId || 0) > 0)
const selectableProductionBomGroupTemplates = computed(() => businessGroupControlOptions(productionBomBusinessGroups.value).templateOptions)
const productionBomGroupFeatureSelectionHasChanges = computed(() => (
  JSON.stringify(businessGroupFeatureSelectionIDs({ group_template_ids: productionBomGroupFeatureSelectionDraft.value }))
  !== JSON.stringify(businessGroupFeatureSelectionIDs({ group_template_ids: productionBomGroupFeatureSelectionTemplateIDs.value }))
))
const productionBomSelectedBusinessGroups = computed(() => businessGroupRowsForFeatureSelection(
  productionBomBusinessGroups.value,
  productionBomGroupFeatureSelectionTemplateIDs.value,
))

function filterProductionBomRows(rows = []) {
  return filterProductionBomCatalog(rows, {
    status: filters.status,
    query: productionBomSearchQuery.value,
  })
}

const productionBomRows = computed(() => {
  return filterProductionBomRows(productionBoms.value)
})
const fullProductionBomDisplayGroups = computed(() => groupRowsByBusinessGroupTemplates(productionBomRows.value, {
  templates: productionBomSelectedBusinessGroups.value,
  usageKey: 'production_bom',
  objectKey: 'production_bom',
  objectIDForRow: (row) => Number(row.production_bom_id || row.id || 0),
  rowAssignment: productionBomBusinessGroupAssignment,
}))
const productionBomInlineListState = computed(() => businessGroupInlineListState(
  fullProductionBomDisplayGroups.value,
  productionBomPaginationByGroup.value,
  { defaultPageSize: 10 },
))
const productionBomDisplayGroups = computed(() => productionBomInlineListState.value.groups)
const productionBomVisibleRows = computed(() => businessGroupVisibleRows(
  productionBomDisplayGroups.value,
  collapsedProductionBomGroups.value,
))
const selectedProduct = computed(() => productByID(selectedProductId.value))
const selectedBomOutput = computed(() => bomForm.output_type === 'material' ? materialByID(bomForm.output_id) : productByID(bomForm.output_id))
const outputUnitCode = computed(() => String(selectedBomOutput.value?.inventory_unit || selectedBomOutput.value?.unit || selectedProductionBomVersion.value?.output_unit || bomForm.output_unit || 'unit').trim() || 'unit')
const outputUnitDisplay = computed(() => unitLabel(outputUnitCode.value))
const rawReferencedProducts = computed(() => detail.value?.referenced_products || productionBomDetail.value?.referenced_products || [])
const referencedProducts = computed(() => {
  const seen = new Set()
  const rows = []
  for (const product of rawReferencedProducts.value || []) {
    if (!isActiveReferencedProduct(product)) continue
    const key = referencedProductKey(product)
    if (seen.has(key)) continue
    seen.add(key)
    rows.push(product)
  }
  return rows
})
const usedByBoms = computed(() => detail.value?.used_by_boms || productionBomDetail.value?.used_by_boms || [])
const referencedProductsLabel = computed(() => {
  const count = referencedProducts.value.length
  if (count > 0) return `${count} 个商品`
  if ((rawReferencedProducts.value || []).length > 0) return '未被商品引用'
  const summaryCount = Number(selectedProductionBomRecord.value?.reference_product_count || detail.value?.reference_product_count || productionBomDetail.value?.reference_product_count || 0)
  return summaryCount > 0 ? `${summaryCount} 个商品` : '未被商品引用'
})
const currentProductionBomLabel = computed(() => productionBomLabel(detail.value || selectedProductionBomRecord.value || {}))
const currentProductionBomWarning = computed(() => productionBomVersionWarning(detail.value || selectedProductionBomRecord.value || {}))
const currentProductionBomID = computed(() => Number(detail.value?.production_bom_id || selectedProductionBomRecord.value?.production_bom_id || selectedProductionBomRecord.value?.id || 0))
const currentOutputIdentity = computed(() => productionBomOutputIdentity(bomDrawerOpen.value && bomForm.mode === 'edit'
  ? { ...selectedProductionBomRecord.value, output_type: bomForm.output_type, output_id: bomForm.output_id, output_product_id: bomForm.output_type === 'product' ? bomForm.output_id : 0, output_material_id: bomForm.output_type === 'material' ? bomForm.output_id : 0 }
  : (detail.value || productionBomDetail.value || selectedProductionBomRecord.value || {})))
const currentOutputProductID = computed(() => Number(detail.value?.output_product_id || productionBomDetail.value?.output_product_id || selectedProductionBomRecord.value?.output_product_id || 0))
const currentOutputBasisLabel = computed(() => `${qty(selectedProductionBomVersion.value?.output_qty || 1)} ${selectedProductionBomVersion.value?.output_unit || 'unit'}`)
const currentOutputProduct = computed(() => currentOutputIdentity.value.type === 'product' ? productByID(currentOutputIdentity.value.id || currentOutputProductID.value) : null)
const outputUnitSourceHint = computed(() => {
  if (bomForm.output_type === 'material') return '来源：物料档案库存单位'
  return '来源：当前 BOM 规格组中各规格的库存单位'
})
const outputUnitMismatchWarning = computed(() => {
  const versionUnit = String(selectedProductionBomVersion.value?.output_unit || '').trim()
  const productUnit = String(currentOutputProduct.value?.inventory_unit || (bomForm.output_type === 'product' ? selectedBomOutput.value?.inventory_unit : '') || '').trim()
  if (!versionUnit || !productUnit || versionUnit === productUnit) return ''
  return `当前历史版本产出单位为 ${unitLabel(versionUnit)}，商品旧库存单位为 ${unitLabel(productUnit)}；历史版本不会自动回改`
})
const bomFormTitle = computed(() => ({
  create: '新建生产 BOM',
  edit: '编辑 BOM',
  copy: '复制 BOM',
})[bomForm.mode] || '生产 BOM')
const selectedProductionBomVersion = computed(() => versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0)) || null)
const currentProductionBomDefaultVersion = computed(() => {
  const published = (versions.value || []).filter((version) => {
    const status = String(version?.status || '').trim().toLowerCase()
    return status === 'published' || status === 'active'
  })
  if (!published.length) return null
  const latest = published.find((version) => version.is_latest === true || version.isLatest === true || version.is_latest_bom_version === true || version.isLatestBomVersion === true)
  if (latest) return latest
  return [...published].sort((a, b) => {
    const aTime = Date.parse(a.published_at || a.publishedAt || a.created_at || a.createdAt || '') || 0
    const bTime = Date.parse(b.published_at || b.publishedAt || b.created_at || b.createdAt || '') || 0
    return bTime - aTime || Number(b.id || 0) - Number(a.id || 0)
  })[0] || null
})
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
const canSetCurrentBomAsDefault = computed(() => currentProductionBomID.value > 0 && currentOutputIdentity.value.id > 0 && Number(currentProductionBomDefaultVersion.value?.id || 0) > 0)
const canEditBomFormOutputBasis = computed(() => bomForm.mode !== 'edit' || canEditCurrentBomItems.value)
const outputProductOptions = computed(() => products.value.filter(isProductionBomOutputProductCandidate))
const outputMaterialOptions = computed(() => materials.value.filter((row) => !isInactiveMarker(row.active) && !isInactiveMarker(row.status) && !row.deprecated_at))
const outputTargetOptions = computed(() => bomForm.output_type === 'material' ? outputMaterialOptions.value : outputProductOptions.value)
const productComponentOptions = computed(() => products.value.filter(isBomProductCandidate))
const materialComponentOptions = computed(() => materials.value.filter((material) => !(currentOutputIdentity.value.type === 'material' && Number(material.id || 0) === currentOutputIdentity.value.id)))
const selectedComponent = computed(() => itemForm.component_type === 'product'
  ? productByID(itemForm.component_product_id)
  : materials.value.find((material) => Number(material.id || 0) === Number(itemForm.material_id || 0)))
const selectedComponentBomSpec = computed(() => componentBomSpecOptions.value.find((spec) => Number(spec.bom_spec_id || 0) === Number(itemForm.component_bom_spec_id || 0)) || null)
const componentStockUnitCode = computed(() => String(
  (itemForm.component_type === 'product' ? selectedComponentBomSpec.value?.inventory_unit : '')
  || selectedComponent.value?.inventory_unit
  || selectedComponent.value?.unit
  || '',
).trim())
const componentStockUnitLabel = computed(() => unitLabel(componentStockUnitCode.value || (itemForm.component_type === 'product' ? 'unit' : 'kg')))
const ratioConsumeUnitOption = { value: 'ratio_pct', label: '比例 %' }
const materialLossRatioOnlyConsumeUnitOptions = [ratioConsumeUnitOption]
const legacyConsumeUnitLabels = {
  g_per_bag: '克/袋',
  unit_per_bag: '个/袋',
  unit_per_box: '个/盒',
  fixed_qty: '固定数量',
}
const unitDictionaryConsumeUnitOptions = computed(() => productUnitDefinitions.value
  .filter((unit) => unit.active !== false)
  .map((unit) => {
    const value = String(unit.code || '').trim()
    return value ? { value, label: unit.name ? `${unit.name}（${value}）` : value } : null
  })
  .filter(Boolean)
  .sort((a, b) => String(a.label || '').localeCompare(String(b.label || ''))))
const currentConsumeUnitOptions = computed(() => {
  if (versionMaterialLossRateEnabled.value || recipeConsumeMode.value === 'ratio') {
    return materialLossRatioOnlyConsumeUnitOptions
  }
  if (recipeConsumeMode.value === 'fixed' || itemForm.component_type === 'product') {
    return componentInventoryConsumeUnitOptions(itemForm.consume_unit)
  }
  return consumeUnitOptionsWithCurrent(
    itemForm.component_type !== 'product',
    itemForm.consume_unit,
  )
})
const isMaterialOutputBom = computed(() => currentOutputIdentity.value?.type === 'material')
const versionMaterialLossRateEnabled = ref(false)
const versionMaterialLossRatePct = ref('')
const selectedVersionMaterialLossRate = computed(() => {
  if (versionMaterialLossRateEnabled.value) return normalizedMaterialLossRateFromPercent(versionMaterialLossRatePct.value)
  if (isMaterialOutputBom.value) return 0
  return normalizedMaterialLossRateFromValue(currentRecipeTarget.value?.material_loss_rate)
})
const visibleMovableBomRows = computed(() => productionBomVisibleRows.value.filter(isMovableBomRow))
const selectedBomRows = computed(() => {
  const selected = new Set(selectedBomRowKeys.value)
  return productionBomVisibleRows.value.filter((row) => selected.has(bomRowKey(row)) && isMovableBomRow(row))
})
const selectedBomRecordsForMove = computed(() => {
  const byBomID = new Map()
  for (const row of selectedBomRows.value) {
    const record = bomRecordFromRow(row)
    if (record.id > 0 && !byBomID.has(record.id)) byBomID.set(record.id, record)
  }
  return [...byBomID.values()]
})
const canBeginProductionBomCategoryMove = computed(() => Boolean(
  productionBomSelectedBusinessGroups.value.length && selectedBomRecordsForMove.value.length,
))
const selectedActiveBomRecordsForDeactivate = computed(() => {
  const byBomID = new Map()
  for (const row of selectedBomRows.value) {
    const record = bomRecordFromRow(row)
    if (record.id > 0 && record.status !== 'inactive' && !byBomID.has(record.id)) byBomID.set(record.id, record)
  }
  return [...byBomID.values()]
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

function productionBomGroupID(row = {}) {
  return Number(row.business_group_id ?? row.group_id ?? row.production_bom_group_id ?? 0) || 0
}

function productionBomGroupItemID(row = {}) {
  return Number(row.group_item_id ?? row.business_group_item_id ?? row.group_category_id ?? row.production_bom_group_category_id ?? 0) || 0
}

function productionBomGroupShowsTable(group = {}) {
  if (group?.is_template_group) return false
  if (Number(group?.total || 0) > 0) return true
  const groupID = Number(group?.group_id || 0)
  const groupItemID = Number(group?.group_item_id || 0)
  if (!(groupID > 0) || !(groupItemID > 0)) return true
  return !fullProductionBomDisplayGroups.value.some((candidate) => (
    Number(candidate?.group_id || 0) === groupID
    && Number(candidate?.parent_group_item_id || 0) === groupItemID
  ))
}

function handleProductionBomGroupPaginationChange(groupKey, { page, pageSize } = {}) {
  const key = String(groupKey || '')
  if (!key) return
  productionBomPaginationByGroup.value = {
    ...productionBomPaginationByGroup.value,
    [key]: {
      page: Math.max(1, Number(page || 1)),
      pageSize: Math.max(1, Number(pageSize || 10)),
    },
  }
}

function resetProductionBomGroupPages() {
  productionBomPaginationByGroup.value = Object.fromEntries(Object.entries(productionBomPaginationByGroup.value).map(([key, pagination]) => [
    key,
    { page: 1, pageSize: Math.max(1, Number(pagination?.pageSize || 10)) },
  ]))
}

function movableBomRowsForGroup(group = {}) {
  return (Array.isArray(group?.rows) ? group.rows : []).filter(isMovableBomRow)
}

function isAllVisibleBomsSelected(group = {}) {
  const keys = movableBomRowsForGroup(group).map(bomRowKey)
  if (!keys.length) return false
  const selected = new Set(selectedBomRowKeys.value)
  return keys.every((key) => selected.has(key))
}

function isSomeVisibleBomsSelected(group = {}) {
  const keys = movableBomRowsForGroup(group).map(bomRowKey)
  if (!keys.length) return false
  const selected = new Set(selectedBomRowKeys.value)
  const count = keys.filter((key) => selected.has(key)).length
  return count > 0 && count < keys.length
}

function productionBomBusinessGroupAssignment(row = {}) {
  const groupID = productionBomGroupID(row)
  const group = productionBomBusinessGroups.value.find((item) => Number(item.id || 0) === groupID) || null
  if (!group || isSystemDefaultBusinessGroup(group)) {
    return { usage_key: 'production_bom', object_key: 'production_bom', object_id: Number(row.production_bom_id || row.id || 0), group_id: 0, group_item_id: 0 }
  }
  return {
    usage_key: 'production_bom',
    object_key: 'production_bom',
    object_id: Number(row.production_bom_id || row.id || 0),
    group_id: groupID,
    group_item_id: productionBomGroupItemID(row),
  }
}

function isInactiveMarker(value) {
  if (value === false || value === 0) return true
  const normalized = String(value ?? '').trim().toLowerCase()
  return ['inactive', 'disabled', 'deprecated', 'deactivated', 'archived', 'false', '0', '失效'].includes(normalized)
}

function isActiveReferencedProduct(product = {}) {
  return !isInactiveMarker(product.active) && !isInactiveMarker(product.status) && !isInactiveMarker(product.product_status)
}

function referencedProductKey(product = {}) {
  const productID = Number(product.product_id || product.id || 0)
  if (productID > 0) return `product:${productID}`
  const code = String(product.product_code || product.code || '').trim()
  const name = String(product.product_name || product.name || '').trim()
  return `product:${code}:${name}`
}

function isMovableBomRow(row = {}) {
  return Number(row.production_bom_id || 0) > 0
}

function optionLabel(option) {
  return bomProductOptionLabel(option)
}

function optionMeta(option) {
  const parts = []
  parts.push('商品档案')
  if (option?.number) parts.push(option.number)
  if (option?.roast_level) parts.push(option.roast_level)
  if (option?.inventory_unit) parts.push(`库存 ${unitLabel(option.inventory_unit)}`)
  return parts.join(' / ')
}

function outputTargetOptionLabel(option) {
  if (bomForm.output_type === 'material') {
    const code = String(option?.code || '').trim()
    const name = String(option?.name || '').trim()
    return [code, name].filter(Boolean).join(' ') || `物料 #${Number(option?.id || 0)}`
  }
  return bomProductOptionLabel(option)
}

function outputTargetOptionMeta(option) {
  if (bomForm.output_type === 'material') {
    return ['物料档案', option?.unit ? `库存 ${unitLabel(option.unit)}` : '', option?.is_semi_finished ? '半成品标识' : ''].filter(Boolean).join(' / ')
  }
  return optionMeta(option)
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
  versionNote.value = draft.versionNote || ''
  if (pendingProductionBomID.value > 0) {
    const pendingID = pendingProductionBomID.value
    pendingProductionBomID.value = 0
    const record = productionBoms.value.find((row) => Number(row.id || row.production_bom_id || 0) === pendingID)
      || { id: pendingID, production_bom_id: pendingID }
    await openEditProductionBomRecord(record)
  }
}

function normalizeBomProduct(product) {
  return {
    ...product,
    id: Number(product.id || 0),
    customer_id: Number(product.customer_id || 0),
    inventory_unit: String(product.inventory_unit || product.inventoryUnit || '').trim(),
    inventory_unit_explicit: Boolean(product.inventory_unit_explicit ?? product.inventoryUnitExplicit ?? false),
  }
}

function normalizeBomMaterial(material = {}) {
  return {
    ...material,
    id: Number(material.id || 0),
    code: String(material.product_code || material.code || '').trim(),
    name: String(material.name || '').trim(),
    unit: String(material.unit || material.inventory_unit || '').trim(),
    inventory_unit: String(material.inventory_unit || material.unit || '').trim(),
    is_semi_finished: Boolean(material.is_semi_finished),
    can_manufacture: Boolean(material.can_manufacture),
  }
}

function normalizeProductionBomRecord(row = {}) {
  const output = productionBomOutputIdentity(row)
  const groupID = Number(row.business_group_id ?? row.group_id ?? row.production_bom_group_id ?? 0) || 0
  const groupItemID = Number(row.group_item_id ?? row.business_group_item_id ?? row.group_category_id ?? row.production_bom_group_category_id ?? 0) || 0
  return {
    ...row,
    id: Number(row.id || row.production_bom_id || 0),
    production_bom_id: Number(row.production_bom_id || row.id || 0),
    output_product_id: Number(row.output_product_id || 0),
    output_product_name: row.output_product_name || '',
    output_product_code: row.output_product_code || '',
    output_type: output.type,
    output_id: output.id,
    output_name: output.name,
    output_code: output.code,
    output_unit: output.unit,
    output_material_id: output.type === 'material' ? output.id : 0,
    output_material_name: output.type === 'material' ? output.name : '',
    output_material_code: output.type === 'material' ? output.code : '',
    business_group_id: groupID,
    business_group_name: row.business_group_name || row.group_name || row.production_bom_group_name || '',
    group_id: groupID,
    production_bom_group_id: groupID,
    group_item_id: groupItemID,
    business_group_item_id: groupItemID,
    group_category_id: groupItemID,
    production_bom_group_category_id: groupItemID,
    group_item_name: row.group_item_name || row.group_category_name || row.production_bom_group_category_name || '',
    group_category_name: row.group_item_name || row.group_category_name || row.production_bom_group_category_name || '',
    production_bom_group_category_name: row.production_bom_group_category_name || row.group_item_name || row.group_category_name || '',
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

function materialByID(materialID) {
  const id = Number(materialID || 0)
  return materials.value.find((material) => Number(material.id || 0) === id) || null
}

function productHasExplicitInventoryUnit(product = {}) {
  if (Object.prototype.hasOwnProperty.call(product, 'inventory_unit_explicit') || Object.prototype.hasOwnProperty.call(product, 'inventoryUnitExplicit')) {
    return Boolean(product.inventory_unit_explicit ?? product.inventoryUnitExplicit)
  }
  const raw = product.unit_rule_override_json ?? product.unitRuleOverrideJSON
  if (typeof raw !== 'string' || !raw.trim()) return true
  try {
    const parsed = JSON.parse(raw)
    return Boolean(String(parsed?.inventory_unit || '').trim())
  } catch {
    return true
  }
}

function defaultDictionaryConsumeUnit() {
  return unitDictionaryConsumeUnitOptions.value[0]?.value || 'unit'
}

function unitLabel(unit) {
  const code = String(unit || '').trim()
  if (!code) return '-'
  const row = productUnitDefinitions.value.find((candidate) => String(candidate.code || '').trim() === code)
  return row?.name || code
}

function consumeUnitOptionsWithCurrent(includeRatio, currentUnit) {
  const options = includeRatio ? [ratioConsumeUnitOption, ...unitDictionaryConsumeUnitOptions.value] : [...unitDictionaryConsumeUnitOptions.value]
  const current = String(currentUnit || '').trim()
  if (current && current !== 'ratio_pct' && !options.some((option) => option.value === current)) {
    options.push({ value: current, label: legacyConsumeUnitLabels[current] || current })
  }
  if (!options.length) options.push({ value: 'unit', label: 'unit' })
  return options
}

function componentInventoryConsumeUnitOptions(currentUnit) {
  const inventoryUnit = componentStockUnitCode.value
  if (!inventoryUnit) return consumeUnitOptionsWithCurrent(false, currentUnit)
  const configured = unitDictionaryConsumeUnitOptions.value.find((option) => option.value === inventoryUnit)
  return [configured || { value: inventoryUnit, label: unitLabel(inventoryUnit) }]
}

function consumeUnitLabel(unit) {
  const value = String(unit || '').trim()
  if (value === ratioConsumeUnitOption.value) return ratioConsumeUnitOption.label
  return unitDictionaryConsumeUnitOptions.value.find((option) => option.value === value)?.label || legacyConsumeUnitLabels[value] || value || '-'
}

function componentItemName(item) {
  if (item?.component_type === 'product' || item?.component_type === 'finished_product') {
    const product = productByID(item.component_product_id)
    const productName = product?.name || product?.product_name || item.component_product_name || `商品 #${item.component_product_id}`
    const spec = (item.component_bom_spec_options || []).find((row) => Number(row.bom_spec_id || row.id || 0) === Number(item.component_bom_spec_id || 0))
    const specName = spec?.name || spec?.spec_name || item.component_bom_spec_name || item.component_spec_name || ''
    return specName ? `${productName} · ${specName}` : productName
  }
  const material = materialByID(item?.material_id)
  return material?.name || material?.material_name || item?.material_name || `物料 #${item?.material_id || 0}`
}

function componentBomSpecOptionLabel(spec = {}) {
  const name = String(spec.name || spec.spec_name || spec.spec_key || `规格 #${spec.bom_spec_id || 0}`).trim()
  const unit = String(spec.inventory_unit || '').trim()
  return unit ? `${name} · ${unit}` : name
}

async function loadComponentProductSpecs(productID, { preserve = false } = {}) {
  const id = Number(productID || 0)
  if (!id) {
    componentBomSpecOptions.value = []
    itemForm.component_bom_spec_id = 0
    return
  }
  try {
    const response = await apiGet(`/api/products/${id}/bom-spec-options`)
    const rows = Array.isArray(response?.rows) ? response.rows : (Array.isArray(response) ? response : [])
    componentBomSpecOptions.value = rows
      .filter((row) => Number(row.bom_spec_id || row.id || 0) > 0)
      .map((row) => ({ ...row, bom_spec_id: Number(row.bom_spec_id || row.id || 0), bom_variant_id: Number(row.bom_variant_id || 0) }))
      .sort((left, right) => Number(left.sort_order || 0) - Number(right.sort_order || 0) || componentBomSpecOptionLabel(left).localeCompare(componentBomSpecOptionLabel(right)))
    const selected = Number(itemForm.component_bom_spec_id || 0)
    if (!preserve || !componentBomSpecOptions.value.some((row) => row.bom_spec_id === selected)) {
      itemForm.component_bom_spec_id = Number(componentBomSpecOptions.value.find((row) => row.is_default === true)?.bom_spec_id || 0)
    }
  } catch (err) {
    componentBomSpecOptions.value = []
    itemForm.component_bom_spec_id = 0
    error.value = err.message || '商品 BOM 规格加载失败'
  }
}

function itemQuantityDisplay(item) {
  if ((item?.consume_unit || 'ratio_pct') === 'ratio_pct') {
    const lossText = materialLossRateDisplay(item)
    return lossText ? `${ratio(item.ratio_pct)} · ${lossText}` : ratio(item.ratio_pct)
  }
  return `${qty(item.qty_per_unit)} ${consumeUnitLabel(item.consume_unit)}`
}

function normalizedMaterialLossRateFromValue(value) {
  const rate = Number(value || 0)
  if (!Number.isFinite(rate) || rate <= 0 || rate >= 1) return 0
  return rate
}

function normalizedMaterialLossRateFromPercent(value) {
  return normalizedMaterialLossRateFromValue(Number(value || 0) / 100)
}

function syncVersionMaterialLossRateFromSelectedVersion() {
  const rate = isMaterialOutputBom.value
    ? normalizedMaterialLossRateFromValue(currentRecipeTarget.value?.material_loss_rate)
    : 0
  versionMaterialLossRateEnabled.value = rate > 0
  versionMaterialLossRatePct.value = rate > 0 ? Number((rate * 100).toFixed(4)) : ''
  syncItemFormToRecipeMode()
}

function selectBomVariant(variant = {}) {
  selectedBomVariantID.value = Number(variant.bom_variant_id || variant.id || 0)
  syncVersionMaterialLossRateFromSelectedVersion()
  resetItemForm()
}

function syncSelectedBomVariant() {
  if (!bomVariants.value.length) {
    selectedBomVariantID.value = 0
    return
  }
  const existing = bomVariants.value.find((variant) => Number(variant.bom_variant_id || variant.id || 0) === Number(selectedBomVariantID.value || 0))
  const selected = existing || bomVariants.value.find((variant) => variant.is_default === true) || bomVariants.value[0]
  selectedBomVariantID.value = Number(selected?.bom_variant_id || selected?.id || 0)
}

function recipeModeForItems(items = []) {
  let hasRatio = false
  let hasFixed = false
  for (const item of items || []) {
    if (String(item?.consume_unit || 'ratio_pct') === 'ratio_pct') hasRatio = true
    else hasFixed = true
  }
  if (hasRatio && hasFixed) return 'mixed_legacy'
  if (hasRatio) return 'ratio'
  if (hasFixed) return 'fixed'
  return 'empty'
}

function validateRecipeMode(items = [], lossRate = 0) {
  const mode = recipeModeForItems(items)
  if (mode === 'mixed_legacy') throw new Error('同一配方不能混合使用比例 % 和固定用量')
  if (lossRate > 0) {
    const invalid = (items || []).some((item) => (
      (item?.component_type || 'material') !== 'material'
      || String(item?.consume_unit || '') !== 'ratio_pct'
    ))
    if (invalid) throw new Error('历史比例配方中所有组件消耗单位必须为比例 %')
  }
}

function syncItemFormToRecipeMode() {
  if (versionMaterialLossRateEnabled.value || recipeConsumeMode.value === 'ratio') {
    itemForm.component_type = 'material'
    itemForm.component_product_id = 0
    itemForm.component_bom_spec_id = 0
    componentBomSpecOptions.value = []
    itemForm.component_spec_g = 0
    itemForm.consume_unit = 'ratio_pct'
    itemForm.qty_per_unit = ''
    return
  }
  if (recipeConsumeMode.value === 'fixed' || itemForm.component_type === 'product') {
    itemForm.consume_unit = componentStockUnitCode.value || defaultDictionaryConsumeUnit()
    itemForm.ratio_pct = ''
  }
}

function handleVersionMaterialLossToggle() {
  if (!versionMaterialLossRateEnabled.value) {
    versionMaterialLossRatePct.value = ''
    syncItemFormToRecipeMode()
    return
  }
  if (recipeConsumeMode.value === 'fixed' || recipeConsumeMode.value === 'mixed_legacy') {
    versionMaterialLossRateEnabled.value = false
    versionMaterialLossRatePct.value = ''
    error.value = '已有固定用量组件，不能开启原料损耗比；请先删除固定组件或拆分 BOM'
    return
  }
  if (!(Number(versionMaterialLossRatePct.value || 0) > 0)) versionMaterialLossRatePct.value = 1
  syncItemFormToRecipeMode()
}

function materialLossRateDisplay(item = {}) {
  const lossRate = normalizedMaterialLossRateFromValue(item.material_loss_rate)
  if (!lossRate || (item.component_type !== 'material') || ((item.consume_unit || 'ratio_pct') !== 'ratio_pct')) return ''
  const effectiveRatio = Number(item.ratio_pct || 0) / (1 - lossRate)
  return `原料损耗 ${ratio(lossRate * 100)}，损耗后用量占比 ${ratio(effectiveRatio)}（配方比例 ÷ (1 - 原料损耗率)）`
}

function productionBomDraftItemFromItem(item = {}, index = 0) {
  const componentType = (item.component_type === 'product' || item.component_type === 'finished_product') ? 'product' : 'material'
  const consumeUnit = item.consume_unit || 'ratio_pct'
  const product = productByID(item.component_product_id)
  const material = materialByID(item.material_id)
  const spec = (item.component_bom_spec_options || []).find((row) => Number(row.bom_spec_id || row.id || 0) === Number(item.component_bom_spec_id || 0))
  return {
    id: Number(item.id || 0),
    local_key: String(item.local_key || '').trim() || productionBomDraftItemKey({ ...item, local_key: `bom-item-draft-${index}` }),
    bom_variant_id: Number(item.bom_variant_id || 0),
    material_id: Number(item.material_id || 0),
    component_type: componentType,
    component_product_id: Number(item.component_product_id || 0),
    component_bom_spec_id: Number(item.component_bom_spec_id || 0),
    component_spec_g: Number(item.component_spec_g || 0),
    consume_unit: consumeUnit,
    qty_per_unit: Number(item.qty_per_unit || 0),
    ratio_pct: Number(item.ratio_pct || 0),
    material_name: material?.name || material?.material_name || item.material_name || '',
    component_product_name: product?.name || product?.product_name || item.component_product_name || '',
    component_bom_spec_name: spec?.name || spec?.spec_name || item.component_bom_spec_name || item.component_spec_name || '',
    consume_unit_label: consumeUnitLabel(consumeUnit),
    material_loss_rate: componentType === 'material' && consumeUnit === 'ratio_pct'
      ? normalizedMaterialLossRateFromValue(item.material_loss_rate)
      : 0,
  }
}

function productionBomDraftItemPayloadFromItem(item = {}, index = 0) {
  const pageItem = productionBomDraftItemFromItem(item, index)
  const payload = { ...pageItem }
  delete payload.material_name
  delete payload.component_product_name
  delete payload.component_bom_spec_name
  delete payload.consume_unit_label
  delete payload.local_key
  return payload
}

function productionBomDraftVariantPayloadFromVariant(variant = {}, overrideItems = null) {
  const variantID = Number(variant.bom_variant_id || variant.id || 0)
  const lossRate = normalizedMaterialLossRateFromValue(variant.material_loss_rate)
  const items = (overrideItems || variant.items || []).map((item, index) => {
    const payload = productionBomDraftItemPayloadFromItem(item, index)
    payload.material_loss_rate = lossRate > 0 && payload.component_type === 'material' && payload.consume_unit === 'ratio_pct'
      ? lossRate
      : 0
    return payload
  })
  validateRecipeMode(items, lossRate)
  return {
    bom_variant_id: variantID,
    bom_spec_id: Number(variant.bom_spec_id || 0),
    barcode: String(variant.barcode || '').trim(),
    spec_key: String(variant.spec_key || '').trim() || nextSpecKey(bomVariants.value.map((row) => String(row.spec_key || '').trim()).filter(Boolean)),
    name: String(variant.name || variant.spec_name || '').trim(),
    inventory_unit: String(variant.inventory_unit || '').trim(),
    is_default: variant.is_default === true,
    sort_order: Number(variant.sort_order || 0),
    material_loss_rate: lossRate,
    process_route_id: Number(variant.process_route_id || 0),
    items,
  }
}

function makeSelectedBomVariantDefault() {
  const selectedID = Number(selectedBomVariant.value?.bom_variant_id || selectedBomVariant.value?.id || 0)
  const selectedSpecID = Number(selectedBomVariant.value?.bom_spec_id || 0)
  for (const variant of bomVariants.value) {
    const sameVariant = selectedID !== 0
      ? Number(variant.bom_variant_id || variant.id || 0) === selectedID
      : Number(variant.bom_spec_id || 0) === selectedSpecID
    variant.is_default = sameVariant
  }
}

function validatedProductionBomDraftVariantPayloads(variants = bomVariants.value) {
  if (!Array.isArray(variants) || !variants.length) throw new Error('商品 BOM 规格组至少需要一个规格')
  assignVariantSpecKeys(variants)
  let defaultCount = 0
  const payloads = variants.map((variant) => {
    const payload = productionBomDraftVariantPayloadFromVariant(variant)
    if (!payload.name || !payload.inventory_unit) throw new Error('请填写每个规格的名称和库存单位')
    if (payload.is_default) defaultCount += 1
    return payload
  })
  if (defaultCount !== 1) throw new Error('商品 BOM 规格组必须且只能设置一个默认规格')
  return payloads
}

function addProductionBomDraftVariant() {
  if (!canEditCurrentBomItems.value || !productionBomDetail.value) return
  const localID = Math.min(0, ...bomVariants.value.map((variant) => Number(variant.bom_variant_id || variant.id || 0))) - 1
  const inventoryUnit = String(selectedBomVariant.value?.inventory_unit || activeUnitDefinitions.value[0]?.code || '').trim()
  const variant = {
    bom_variant_id: localID,
    bom_spec_id: 0,
    local_key: `bom-variant-new-${Date.now()}-${Math.abs(localID)}`,
    code: '',
    spec_key: nextSpecKey(bomVariants.value.map((variant) => String(variant.spec_key || '').trim()).filter(Boolean)),
    name: '',
    barcode: '',
    inventory_unit: inventoryUnit,
    is_default: bomVariants.value.length === 0,
    sort_order: bomVariants.value.length + 1,
    material_loss_rate: 0,
    process_route_id: 0,
    process_route_name: '',
    items: [],
  }
  bomVariants.value.push(variant)
  selectBomVariant(variant)
  bomWorkspaceDirty.value = true
}

async function saveProductionBomDraftVariantGroup() {
  if (!canEditCurrentBomItems.value) return
  const draftVersionID = Number(selectedProductionBomDraftVersion.value?.id || 0)
  if (!draftVersionID) return
  await mutate(async () => {
    await apiSend(`/api/production-bom-versions/${draftVersionID}/draft`, {
      method: 'PUT',
      body: { variants: validatedProductionBomDraftVariantPayloads() },
    })
    await loadProductionBomDetailForVersion(currentProductionBomID.value, draftVersionID)
    ok.value = '已更新商品 BOM 规格草稿'
  })
}

async function removeProductionBomDraftVariant() {
  if (!canEditCurrentBomItems.value || bomVariants.value.length <= 1 || !selectedBomVariant.value) return
  const selectedID = Number(selectedBomVariant.value.bom_variant_id || selectedBomVariant.value.id || 0)
  const selectedSpecID = Number(selectedBomVariant.value.bom_spec_id || 0)
  const remaining = bomVariants.value
    .filter((variant) => {
      const variantID = Number(variant.bom_variant_id || variant.id || 0)
      if (selectedID !== 0) return variantID !== selectedID
      return Number(variant.bom_spec_id || 0) !== selectedSpecID
    })
    .map((variant) => ({ ...variant, items: (variant.items || []).map((item) => ({ ...item })) }))
  if (!remaining.some((variant) => variant.is_default === true)) remaining[0].is_default = true
  productionBomDetail.value.variants = remaining
  selectedBomVariantID.value = 0
  bomWorkspaceDirty.value = true
  ok.value = '已从当前草稿删除规格；点击“保存 BOM 草稿”后生效'
}

async function reapplyProductionBomSpecTemplate() {
  if (!canEditCurrentBomItems.value) return
  const draftVersionID = Number(selectedProductionBomDraftVersion.value?.id || 0)
  const specTemplateVersionID = Number(reapplySpecTemplateVersionID.value || 0)
  const mainInputMaterialID = Number(reapplyMainInputMaterialID.value || 0)
  if (!draftVersionID || !specTemplateVersionID || !mainInputMaterialID) {
    error.value = '请选择已发布规格模板版本和规格主体物料'
    return
  }
  await mutate(async () => {
    await apiSend(`/api/production-bom-versions/${draftVersionID}/spec-template`, {
      body: {
        spec_template_version_id: specTemplateVersionID,
        main_input_material_id: mainInputMaterialID,
      },
    })
    reapplySpecTemplateVersionID.value = 0
    reapplyMainInputMaterialID.value = 0
    selectedBomVariantID.value = 0
    await loadProductionBomDetailForVersion(currentProductionBomID.value, draftVersionID)
    ok.value = '已重新套用规格模板；同规格且单位未变的规格身份已保留'
  })
}

function productionBomDraftItemFromForm() {
  const componentType = itemForm.component_type === 'product' ? 'product' : 'material'
  const consumeUnit = itemForm.consume_unit || 'ratio_pct'
  const selectedProduct = componentType === 'product' ? productByID(itemForm.component_product_id) : null
  const selectedMaterial = componentType === 'material' ? materialByID(itemForm.material_id) : null
  const selectedSpec = componentType === 'product' ? selectedComponentBomSpec.value : null
  return {
    local_key: `bom-item-new-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    material_id: Number(itemForm.material_id || 0),
    component_type: componentType,
    component_product_id: Number(itemForm.component_product_id || 0),
    component_bom_spec_id: Number(itemForm.component_bom_spec_id || 0),
    component_spec_g: Number(itemForm.component_spec_g || 0),
    consume_unit: consumeUnit,
    qty_per_unit: Number(itemForm.qty_per_unit || 0),
    ratio_pct: Number(itemForm.ratio_pct || 0),
    material_name: selectedMaterial?.name || selectedMaterial?.material_name || '',
    component_product_name: selectedProduct?.name || selectedProduct?.product_name || '',
    component_bom_spec_name: selectedSpec?.name || selectedSpec?.spec_name || selectedSpec?.spec_key || '',
    consume_unit_label: consumeUnitLabel(consumeUnit),
    material_loss_rate: versionMaterialLossRateEnabled.value && componentType === 'material' && consumeUnit === 'ratio_pct'
      ? selectedVersionMaterialLossRate.value
      : 0,
  }
}

async function saveProductionBomDraftItems(items, basis = {}) {
  const draftVersionID = Number(selectedProductionBomDraftVersion.value?.id || 0)
  if (!draftVersionID) throw new Error('请先复制为新版草稿后再编辑配方明细')
  const lossRate = selectedVersionMaterialLossRate.value
  if (versionMaterialLossRateEnabled.value && !(lossRate > 0)) throw new Error('开启原料损耗比后，损耗比例必须大于 0')
  validateRecipeMode(items, lossRate)
  const body = bomVariants.value.length
    ? {
        variants: bomVariants.value.map((variant) => productionBomDraftVariantPayloadFromVariant(
          variant,
          Number(variant.bom_variant_id || variant.id || 0) === Number(selectedBomVariant.value?.bom_variant_id || selectedBomVariant.value?.id || 0) ? items : null,
        )),
      }
    : {
        output_qty: Number(basis.output_qty || selectedProductionBomVersion.value?.output_qty || 1),
        output_unit: String(basis.output_unit || selectedProductionBomVersion.value?.output_unit || 'unit').trim() || 'unit',
        process_route_id: Number(selectedProductionBomVersion.value?.process_route_id || 0),
        material_loss_rate: selectedVersionMaterialLossRate.value,
        items,
      }
  await apiSend(`/api/production-bom-versions/${draftVersionID}/draft`, { method: 'PUT', body })
}

function markBomWorkspaceDirty() {
  bomWorkspaceDirty.value = true
  ok.value = '已修改本地 BOM 草稿；点击“保存 BOM 草稿”后生效'
}

function setSelectedProductionBomRouteID(routeID) {
  const target = currentRecipeTarget.value
  if (!target || !canEditCurrentBomItems.value) return
  const nextRouteID = Number(routeID || 0)
  target.process_route_id = nextRouteID
  const route = processRoutes.value.find((row) => Number(row.id || 0) === nextRouteID)
  target.process_route_name = route?.name || ''
  markBomWorkspaceDirty()
}

function productionBomVersionStatusLabel(status) {
  if (status === 'draft') return '草稿'
  if (status === 'published' || status === 'active') return '已发布'
  if (status === 'archived') return '已归档'
  return status || '-'
}

function bomRecordFromRow(row = {}) {
  const output = productionBomOutputIdentity(row)
  const groupID = productionBomGroupID(row)
  const groupItemID = productionBomGroupItemID(row)
  return {
    id: Number(row.production_bom_id || row.id || 0),
    code: row.production_bom_code || row.code || '',
    name: row.production_bom_name || row.name || row.product || '',
    output_product_id: Number(row.output_product_id || 0),
    output_product_name: row.output_product_name || '',
    output_product_code: row.output_product_code || '',
    output_type: output.type,
    output_id: output.id,
    output_name: output.name,
    output_code: output.code,
    output_unit: output.unit,
    output_material_id: output.type === 'material' ? output.id : 0,
    output_material_name: output.type === 'material' ? output.name : '',
    output_material_code: output.type === 'material' ? output.code : '',
    business_group_id: groupID,
    business_group_name: row.business_group_name || row.production_bom_group_name || row.group_name || '',
    group_id: groupID,
    group_name: row.business_group_name || row.production_bom_group_name || row.group_name || '',
    group_item_id: groupItemID,
    group_category_id: groupItemID,
    group_item_name: row.group_item_name || row.production_bom_group_category_name || row.group_category_name || '',
    group_category_name: row.group_item_name || row.production_bom_group_category_name || row.group_category_name || '',
    status: row.status === 'inactive' ? 'inactive' : 'active',
  }
}

function toggleAllVisibleBoms(event, rows = []) {
  const shouldSelect = Boolean(event?.target?.checked)
  const next = new Set(selectedBomRowKeys.value)
  for (const row of (Array.isArray(rows) ? rows : []).filter(isMovableBomRow)) {
    const key = bomRowKey(row)
    if (shouldSelect) next.add(key)
    else next.delete(key)
  }
  selectedBomRowKeys.value = [...next]
}

async function selectProductionBomVersion(version, options = {}) {
  const versionID = Number(version?.id || version || 0)
  if (versionID > 0 && versionID !== Number(selectedProductionBomVersionID.value || 0) && !confirmDiscardBomWorkspaceChanges()) return
  if (versionID > 0 && versionID !== Number(selectedProductionBomVersionID.value || 0)) bomWorkspaceDirty.value = false
  selectedProductionBomVersionID.value = versionID
  syncVersionMaterialLossRateFromSelectedVersion()
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
  syncVersionMaterialLossRateFromSelectedVersion()
}

function resetBomForm() {
  bomWorkspaceDirty.value = false
  bomForm.id = 0
  bomForm.source_id = 0
  bomForm.mode = 'create'
  bomForm.name = ''
  bomForm.output_type = 'product'
  bomForm.output_id = 0
  bomForm.output_product_id = 0
  bomForm.output_material_id = 0
  bomForm.output_qty = 1
  bomForm.output_unit = 'unit'
  bomForm.spec_template_version_id = 0
  bomForm.main_input_material_id = 0
  bomForm.status = 'active'
}

function openNewProductionBomRecord() {
  resetBomForm()
  bomForm.mode = 'create'
  bomDrawerOpen.value = true
}

async function openEditProductionBomRecord(bom) {
  if (!confirmDiscardBomWorkspaceChanges()) return
  resetBomForm()
  const record = bomRecordFromRow(bom)
  bomForm.mode = 'edit'
  bomForm.id = record.id
  bomForm.name = record.name || ''
  bomForm.output_type = record.output_type || 'product'
  bomForm.output_id = Number(record.output_id || record.output_product_id || record.output_material_id || 0)
  bomForm.output_product_id = Number(record.output_product_id || 0)
  bomForm.output_material_id = Number(record.output_material_id || 0)
  bomForm.status = record.status === 'inactive' ? 'inactive' : 'active'
  bomDrawerOpen.value = true
  if (record.id > 0 && Number(currentProductionBomID.value || 0) !== record.id) {
    await selectUnboundProductionBom(record)
  }
  bomForm.output_qty = Number(selectedProductionBomVersion.value?.output_qty || 1)
  bomForm.output_unit = selectedProductionBomVersion.value?.output_unit || defaultDictionaryConsumeUnit()
}

function copyProductionBomRecord(bom) {
  resetBomForm()
  bomForm.mode = 'copy'
  bomForm.source_id = Number(bom?.id || 0)
  bomForm.name = `${bom?.name || '生产 BOM'} 副本`
  const output = productionBomOutputIdentity(bom || {})
  bomForm.output_type = output.type
  bomForm.output_id = output.id
  bomForm.output_product_id = Number(bom?.output_product_id || 0)
  bomForm.output_material_id = Number(bom?.output_material_id || 0)
  bomForm.output_qty = Number(selectedProductionBomVersion.value?.output_qty || 1)
  bomForm.output_unit = selectedProductionBomVersion.value?.output_unit || 'unit'
  bomForm.status = 'active'
  bomDrawerOpen.value = true
}

function syncBomOutputType() {
  if (bomForm.mode === 'edit') bomWorkspaceDirty.value = true
  const outputChangedToMaterial = bomForm.mode === 'edit'
    && bomForm.output_type === 'material'
    && String(selectedProductionBomRecord.value?.output_type || '').toLowerCase() === 'product'
  bomForm.output_id = 0
  bomForm.output_product_id = 0
  bomForm.output_material_id = 0
  bomForm.output_unit = bomForm.output_type === 'material' ? 'kg' : defaultDictionaryConsumeUnit()
  bomForm.spec_template_version_id = 0
  bomForm.main_input_material_id = 0
  if (outputChangedToMaterial && bomVariants.value.length > 0) {
    // The old client used variants: [] as the cleanup marker. Keep the
    // 规格组将随保存一起删除；the unified save payload is now mutually
    // exclusive and sends only items for material output.
    productionBomDetail.value.variants = []
    productionBomDetail.value.items = []
    if (detail.value) detail.value.items = []
    selectedBomVariantID.value = 0
    bomWorkspaceDirty.value = true
    error.value = '改为物料产出后，规格组和规格配方已清空；保存 BOM 草稿后生效'
  }
}

function confirmDiscardBomWorkspaceChanges() {
  if (!bomWorkspaceDirty.value) return true
  if (typeof window === 'undefined' || typeof window.confirm !== 'function') return false
  return window.confirm('当前 BOM 草稿有未保存改动，确认放弃？')
}

function closeBomDrawer() {
  if (!confirmDiscardBomWorkspaceChanges()) return
  bomDrawerOpen.value = false
  bomWorkspaceDirty.value = false
  resetBomForm()
  clearSelectedProductionBom()
}

function syncComponentTypeDefaults() {
  if (itemForm.component_type === 'product') {
    if (recipeConsumeMode.value === 'ratio') {
      itemForm.component_type = 'material'
      itemForm.component_product_id = 0
      itemForm.consume_unit = 'ratio_pct'
      error.value = '比例模式不允许商品规格组件'
      return
    }
    itemForm.material_id = 0
    itemForm.consume_unit = componentStockUnitCode.value || defaultDictionaryConsumeUnit()
    itemForm.ratio_pct = ''
    return
  }
  itemForm.component_product_id = 0
  itemForm.component_bom_spec_id = 0
  componentBomSpecOptions.value = []
  itemForm.component_spec_g = 0
  if (recipeConsumeMode.value === 'fixed') {
    itemForm.consume_unit = componentStockUnitCode.value || defaultDictionaryConsumeUnit()
    itemForm.ratio_pct = ''
    return
  }
  itemForm.consume_unit = 'ratio_pct'
  itemForm.qty_per_unit = ''
}

function resetItemForm() {
  itemForm.component_type = 'material'
  itemForm.material_id = 0
  itemForm.component_product_id = 0
  itemForm.component_bom_spec_id = 0
  componentBomSpecOptions.value = []
  itemForm.component_spec_g = 0
  itemForm.consume_unit = 'ratio_pct'
  itemForm.qty_per_unit = ''
  itemForm.ratio_pct = ''
  syncItemFormToRecipeMode()
}

function clearSelectedProductionBom() {
  selectedProductId.value = 0
  detail.value = null
  productionBomDetail.value = null
  selectedProductionBomRecord.value = null
  versions.value = []
  selectedProductionBomVersionID.value = 0
  selectedBomVariantID.value = 0
    updateUrl()
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'productionConfig')
  url.searchParams.set('tab', 'bom')
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
        key: 'productionConfig',
        label: `返回BOM编辑：${currentProductionBomLabel.value || '生产 BOM'}`,
        params: bomID > 0 ? { tab: 'bom', production_bom_id: bomID } : { tab: 'bom' },
        source_label: `商品档案配置：${labelProductName}`,
        targetKey: 'productMaster',
      },
    },
  }))
}

async function loadProductUnitDefinitions() {
  const rows = await apiGet('/api/product-settings/units')
  return Array.isArray(rows) ? rows : []
}

async function loadProductionBomSpecTemplates() {
  const response = await apiGet('/api/production-bom-spec-templates')
  const rows = response?.rows || response || []
  return Promise.all((Array.isArray(rows) ? rows : []).map(async (row) => {
    const detail = await apiGet(`/api/production-bom-spec-templates/${Number(row.id || 0)}`)
    return { ...row, ...detail }
  }))
}

function specTemplateSummary(row = {}) {
  const versions = Array.isArray(row.versions) ? row.versions : []
  const published = versions.filter((version) => ['published', 'active'].includes(String(version.status || '').toLowerCase())).length
  return `${versions.length || Number(row.version_count || 0)} 个版本 · ${published || Number(row.published_version_count || 0)} 个已发布`
}

function templateVariantDraftItemFromAPI(item = {}, index = 0) {
  const componentType = ['product', 'finished_product'].includes(String(item.component_type || '').trim()) ? 'product' : 'material'
  return {
    ...item,
    local_key: `template-item-${item.id || index}-${Date.now()}-${index}`,
    component_type: componentType,
    material_id: componentType === 'material' ? Number(item.material_id || 0) : 0,
    component_product_id: componentType === 'product' ? Number(item.component_product_id || 0) : 0,
    component_bom_spec_id: componentType === 'product' ? Number(item.component_bom_spec_id || 0) : 0,
    component_spec_g: 0,
    component_bom_spec_options: [],
    component_bom_spec_loading: false,
    consume_unit: String(item.consume_unit || '').trim(),
    qty_per_unit: Number(item.qty_per_unit || 0),
    ratio_pct: Number(item.ratio_pct || 0),
  }
}

function templateVariantExplicitRecipeMode(items = []) {
  let hasRatio = false
  let hasFixed = false
  for (const item of items || []) {
    const componentType = ['product', 'finished_product'].includes(String(item?.component_type || '').trim()) ? 'product' : 'material'
    const consumeUnit = String(item?.consume_unit || '').trim()
    if (componentType === 'product') hasFixed = true
    else if (consumeUnit === 'ratio_pct') hasRatio = true
    else if (consumeUnit) hasFixed = true
  }
  if (hasRatio && hasFixed) return 'mixed_legacy'
  if (hasRatio) return 'ratio'
  if (hasFixed) return 'fixed'
  return 'empty'
}

function templateVariantRecipeMode(variant = {}) {
  if (normalizedMaterialLossRateFromPercent(variant.material_loss_rate_pct) > 0) return 'ratio'
  const itemMode = templateVariantExplicitRecipeMode(variant.items)
  if (itemMode !== 'empty') return itemMode
  const mainUnit = String(variant.main_input_consume_unit || '').trim()
  if (!mainUnit) return 'empty'
  return mainUnit === 'ratio_pct' ? 'ratio' : 'fixed'
}

function templateVariantRecipeModeLabel(variant = {}) {
  return ({
    empty: '尚未选择（第一条组件决定）',
    ratio: '全部按比例 %',
    fixed: '全部按各组件库存单位固定用量',
    mixed_legacy: '历史混合配方，需拆分后才能保存或发布',
  })[templateVariantRecipeMode(variant)] || '尚未选择'
}

function templateVariantItemInventoryUnit(item = {}) {
  if (item.component_type === 'product') {
    const selected = (item.component_bom_spec_options || []).find((spec) => Number(spec.bom_spec_id || 0) === Number(item.component_bom_spec_id || 0))
    return String(selected?.inventory_unit || '').trim()
  }
  const material = materialByID(item.material_id)
  return String(material?.inventory_unit || material?.unit || '').trim()
}

function templateVariantInventoryUnitOption(item = {}) {
  const value = templateVariantItemInventoryUnit(item)
  return value ? { value, label: unitLabel(value) } : null
}

function templateVariantItemConsumeUnitOptions(variant = {}, item = {}) {
  const mode = templateVariantRecipeMode(variant)
  if (normalizedMaterialLossRateFromPercent(variant.material_loss_rate_pct) > 0 || mode === 'ratio') {
    return item.component_type === 'product' ? [] : [ratioConsumeUnitOption]
  }
  const inventoryOption = templateVariantInventoryUnitOption(item)
  if (item.component_type === 'product' || mode === 'fixed') return inventoryOption ? [inventoryOption] : []
  if (mode === 'mixed_legacy') {
    const current = String(item.consume_unit || '').trim()
    if (!current) return []
    return [{ value: current, label: consumeUnitLabel(current) }]
  }
  return inventoryOption ? [ratioConsumeUnitOption, inventoryOption] : [ratioConsumeUnitOption]
}

function templateVariantItemConsumeUnitLocked(variant = {}, item = {}) {
  return item.component_type === 'product' || templateVariantRecipeMode(variant) !== 'empty'
}

function templateVariantProductComponentDisabled(variant = {}) {
  return normalizedMaterialLossRateFromPercent(variant.material_loss_rate_pct) > 0 || templateVariantRecipeMode(variant) === 'ratio'
}

function applyTemplateVariantRecipeMode(variant = {}, mode = templateVariantRecipeMode(variant)) {
  if (mode === 'ratio') {
    variant.main_input_consume_unit = 'ratio_pct'
    for (const item of variant.items || []) {
      if (item.component_type === 'product') continue
      item.consume_unit = 'ratio_pct'
      item.qty_per_unit = 0
    }
    return
  }
  if (mode === 'fixed') {
    variant.main_input_consume_unit = 'main_input_unit'
    for (const item of variant.items || []) {
      const inventoryUnit = templateVariantItemInventoryUnit(item)
      item.consume_unit = inventoryUnit
      item.ratio_pct = 0
    }
    return
  }
  if (mode === 'empty') variant.main_input_consume_unit = ''
}

function templateVariantOtherItemMode(variant = {}, itemIndex = -1) {
  return templateVariantExplicitRecipeMode((variant.items || []).filter((_, index) => index !== itemIndex))
}

function syncTemplateVariantItemConsumeMode(variantIndex, itemIndex) {
  const variant = templateDraftVariants.value[variantIndex]
  const item = variant?.items?.[itemIndex]
  if (!variant || !item) return
  const chosenMode = item.consume_unit === 'ratio_pct' ? 'ratio' : 'fixed'
  const otherMode = templateVariantOtherItemMode(variant, itemIndex)
  const lossEnabled = normalizedMaterialLossRateFromPercent(variant.material_loss_rate_pct) > 0
  if ((lossEnabled && chosenMode !== 'ratio') || (otherMode !== 'empty' && otherMode !== chosenMode)) {
    const fallbackMode = lossEnabled ? 'ratio' : otherMode
    applyTemplateVariantRecipeMode(variant, fallbackMode)
    error.value = '同一规格配方不能混合使用比例 % 和固定用量'
    return
  }
  applyTemplateVariantRecipeMode(variant, chosenMode)
}

function syncTemplateVariantItemMaterial(variantIndex, itemIndex) {
  const variant = templateDraftVariants.value[variantIndex]
  const item = variant?.items?.[itemIndex]
  if (!variant || !item) return
  item.component_type = 'material'
  item.component_product_id = 0
  item.component_bom_spec_id = 0
  item.component_spec_g = 0
  item.component_bom_spec_options = []
  const mode = templateVariantRecipeMode(variant)
  if (mode === 'ratio') {
    item.consume_unit = 'ratio_pct'
    item.qty_per_unit = 0
  } else if (mode === 'fixed') {
    item.consume_unit = templateVariantItemInventoryUnit(item)
    item.ratio_pct = 0
  }
}

function syncTemplateVariantItemBomSpec(variantIndex, itemIndex) {
  const variant = templateDraftVariants.value[variantIndex]
  const item = variant?.items?.[itemIndex]
  if (!variant || !item || item.component_type !== 'product') return
  item.consume_unit = templateVariantItemInventoryUnit(item)
  item.ratio_pct = 0
  applyTemplateVariantRecipeMode(variant, 'fixed')
}

async function loadTemplateComponentProductSpecs(variantIndex, itemIndex, productID, { preserve = false } = {}) {
  const variant = templateDraftVariants.value[variantIndex]
  const item = variant?.items?.[itemIndex]
  if (!variant || !item) return
  const id = Number(productID || item.component_product_id || 0)
  item.component_product_id = id
  item.material_id = 0
  item.component_spec_g = 0
  if (!id) {
    item.component_bom_spec_id = 0
    item.component_bom_spec_options = []
    item.consume_unit = ''
    return
  }
  const localKey = item.local_key
  item.component_bom_spec_loading = true
  try {
    const response = await apiGet(`/api/products/${id}/bom-spec-options`)
    const rows = Array.isArray(response?.rows) ? response.rows : (Array.isArray(response) ? response : [])
    const current = templateDraftVariants.value[variantIndex]?.items?.[itemIndex]
    if (!current || current.local_key !== localKey || Number(current.component_product_id || 0) !== id) return
    current.component_bom_spec_options = rows
      .filter((row) => Number(row.bom_spec_id || row.id || 0) > 0)
      .map((row) => ({ ...row, bom_spec_id: Number(row.bom_spec_id || row.id || 0), bom_variant_id: Number(row.bom_variant_id || 0) }))
      .sort((left, right) => Number(left.sort_order || 0) - Number(right.sort_order || 0) || componentBomSpecOptionLabel(left).localeCompare(componentBomSpecOptionLabel(right)))
    const selectedID = Number(current.component_bom_spec_id || 0)
    if (preserve && selectedID > 0 && !current.component_bom_spec_options.some((row) => Number(row.bom_spec_id || 0) === selectedID)) {
      current.component_bom_spec_options.push({
        bom_spec_id: selectedID,
        name: current.component_bom_spec_name || current.component_spec_name || `历史规格 #${selectedID}`,
        inventory_unit: String(current.consume_unit || '').trim(),
      })
    } else if (!preserve || selectedID <= 0) {
      current.component_bom_spec_id = Number(current.component_bom_spec_options.find((row) => row.is_default === true)?.bom_spec_id || current.component_bom_spec_options[0]?.bom_spec_id || 0)
    }
    if (!preserve || !current.consume_unit) syncTemplateVariantItemBomSpec(variantIndex, itemIndex)
  } catch (err) {
    const current = templateDraftVariants.value[variantIndex]?.items?.[itemIndex]
    if (current?.local_key === localKey) {
      current.component_bom_spec_options = []
      if (!preserve) current.component_bom_spec_id = 0
    }
    error.value = err.message || '商品 BOM 规格加载失败'
  } finally {
    const current = templateDraftVariants.value[variantIndex]?.items?.[itemIndex]
    if (current?.local_key === localKey) current.component_bom_spec_loading = false
  }
}

function syncTemplateVariantItemComponentType(variantIndex, itemIndex) {
  const variant = templateDraftVariants.value[variantIndex]
  const item = variant?.items?.[itemIndex]
  if (!variant || !item) return
  if (item.component_type === 'product') {
    if (templateVariantProductComponentDisabled(variant, item)) {
      item.component_type = 'material'
      error.value = '比例模式不允许商品规格组件'
      return
    }
    item.material_id = 0
    item.component_product_id = 0
    item.component_bom_spec_id = 0
    item.component_spec_g = 0
    item.component_bom_spec_options = []
    item.consume_unit = ''
    item.ratio_pct = 0
    applyTemplateVariantRecipeMode(variant, 'fixed')
    return
  }
  item.component_type = 'material'
  item.component_product_id = 0
  item.component_bom_spec_id = 0
  item.component_spec_g = 0
  item.component_bom_spec_options = []
  const mode = templateVariantRecipeMode(variant)
  if (mode === 'ratio') item.consume_unit = 'ratio_pct'
  else if (mode === 'fixed') item.consume_unit = templateVariantItemInventoryUnit(item)
  else item.consume_unit = ''
}

function templateVariantDraftFromAPI(variant = {}, index = 0) {
  const allItems = Array.isArray(variant.items) ? variant.items : []
  const mainItem = allItems.find((item) => item.is_main_input === true) || {}
  return {
    ...variant,
    local_key: `template-variant-${variant.spec_key || index}-${Date.now()}`,
    spec_key: String(variant.spec_key || '').trim(),
    name: String(variant.name || variant.spec_name || '').trim(),
    inventory_unit: String(variant.inventory_unit || '').trim(),
    is_default: variant.is_default === true,
    sort_order: Number(variant.sort_order || index + 1),
    process_route_id: Number(variant.process_route_id || 0),
    material_loss_rate_pct: Number((normalizedMaterialLossRateFromValue(variant.material_loss_rate) * 100).toFixed(4)) || 0,
    main_input_qty: Number(mainItem.ratio_pct || mainItem.qty_per_unit || variant.main_input_qty || 1),
    main_input_consume_unit: String(mainItem.consume_unit || '').trim(),
    items: allItems.filter((item) => item.is_main_input !== true).map(templateVariantDraftItemFromAPI),
  }
}

function templateVariantPayload(variant = {}) {
  const lossRate = normalizedMaterialLossRateFromPercent(variant.material_loss_rate_pct)
  const otherItems = (variant.items || []).map((item) => {
    const componentType = item.component_type === 'product' ? 'product' : 'material'
    const consumeUnit = String(item.consume_unit || '').trim()
    const inventoryUnit = templateVariantItemInventoryUnit(item)
    if (componentType === 'product') {
      const selectedSpecExists = (item.component_bom_spec_options || []).some((spec) => Number(spec.bom_spec_id || 0) === Number(item.component_bom_spec_id || 0))
      if (Number(item.component_product_id || 0) <= 0 || Number(item.component_bom_spec_id || 0) <= 0 || !selectedSpecExists) {
        throw new Error('商品规格组件必须选择明确的已发布 BOM 规格')
      }
      if (consumeUnit === 'ratio_pct') throw new Error('商品规格组件只能使用固定用量')
      if (!inventoryUnit || consumeUnit !== inventoryUnit) throw new Error('商品规格组件消耗单位必须使用所选规格库存单位')
    } else if (consumeUnit !== 'ratio_pct') {
      if (!inventoryUnit || consumeUnit !== inventoryUnit) throw new Error('物料固定用量必须使用该物料的库存单位')
    }
    return {
      component_type: componentType,
      material_id: componentType === 'material' ? Number(item.material_id || 0) : 0,
      component_product_id: componentType === 'product' ? Number(item.component_product_id || 0) : 0,
      component_bom_spec_id: componentType === 'product' ? Number(item.component_bom_spec_id || 0) : 0,
      component_spec_g: 0,
      consume_unit: consumeUnit,
      qty_per_unit: consumeUnit === 'ratio_pct' ? 0 : Number(item.qty_per_unit || 0),
      ratio_pct: consumeUnit === 'ratio_pct' ? Number(item.ratio_pct || 0) : 0,
      is_main_input: false,
    }
  })
  const inferredMode = templateVariantRecipeMode(variant)
  const mainConsumeUnit = lossRate > 0 || inferredMode === 'ratio' ? 'ratio_pct' : 'main_input_unit'
  const mainItem = {
    is_main_input: true,
    component_type: 'material',
    material_id: 0,
    consume_unit: mainConsumeUnit,
    qty_per_unit: mainConsumeUnit === 'ratio_pct' ? 0 : Number(variant.main_input_qty || 0),
    ratio_pct: mainConsumeUnit === 'ratio_pct' ? Number(variant.main_input_qty || 0) : 0,
  }
  validateRecipeMode([mainItem, ...otherItems], lossRate)
  return {
    spec_key: String(variant.spec_key || '').trim() || nextSpecKey(templateDraftVariants.value.map((row) => String(row.spec_key || '').trim()).filter(Boolean)),
    name: String(variant.name || '').trim(),
    inventory_unit: String(variant.inventory_unit || '').trim(),
    is_default: variant.is_default === true,
    sort_order: Number(variant.sort_order || 0),
    material_loss_rate: lossRate,
    process_route_id: Number(variant.process_route_id || 0),
    items: [mainItem, ...otherItems],
  }
}

function setTemplateDetail(response = {}, versionID = 0) {
  selectedSpecTemplateDetail.value = response
  const versions = Array.isArray(response.versions) ? response.versions : []
  const requested = versions.find((version) => Number(version.id || 0) === Number(versionID || 0))
  const selected = requested || versions.find((version) => version.status === 'draft') || versions.find((version) => ['published', 'active'].includes(String(version.status || '').toLowerCase())) || versions[0] || null
  selectedSpecTemplateVersionID.value = Number(selected?.id || 0)
  templateDraftVariants.value = (response.variants || selected?.variants || []).map(templateVariantDraftFromAPI)
  templateDraftVariants.value.forEach((variant, variantIndex) => {
    variant.items.forEach((item, itemIndex) => {
      if (item.component_type === 'product' && item.component_product_id > 0) {
        void loadTemplateComponentProductSpecs(variantIndex, itemIndex, item.component_product_id, { preserve: true })
      }
    })
  })
}

function openSpecTemplateDrawer() {
  specTemplateDrawerOpen.value = true
  if (!selectedSpecTemplateID.value && productionBomSpecTemplates.value.length) selectSpecTemplate(productionBomSpecTemplates.value[0].id)
}

function closeSpecTemplateDrawer() {
  specTemplateDrawerOpen.value = false
}

async function createSpecTemplate() {
  const name = String(specTemplateNewName.value || '').trim()
  if (!name) return
  await mutate(async () => {
    const created = await apiSend('/api/production-bom-spec-templates', { body: { name } })
    specTemplateNewName.value = ''
    productionBomSpecTemplates.value = await loadProductionBomSpecTemplates()
    await selectSpecTemplate(created?.id || created?.template_id)
    ok.value = '已新建 BOM 规格模板'
  })
}

async function selectSpecTemplate(templateID) {
  const id = Number(templateID || 0)
  if (!id) return
  selectedSpecTemplateID.value = id
  const response = await apiGet(`/api/production-bom-spec-templates/${id}`)
  setTemplateDetail(response)
}

async function selectSpecTemplateVersion(versionID) {
  const id = Number(versionID || 0)
  if (!selectedSpecTemplateID.value || !id) return
  const response = await apiGet(`/api/production-bom-spec-templates/${selectedSpecTemplateID.value}?version_id=${id}`)
  setTemplateDetail(response, id)
}

async function createSpecTemplateDraft() {
  if (!selectedSpecTemplateID.value) return
  await mutate(async () => {
    const created = await apiSend(`/api/production-bom-spec-templates/${selectedSpecTemplateID.value}/versions`, {
      body: { source_version_id: Number(selectedSpecTemplateVersionID.value || 0) },
    })
    await selectSpecTemplateVersion(created?.id || created?.version_id)
    ok.value = '已创建规格模板草稿版本'
  })
}

function addTemplateVariant() {
  const specKey = nextSpecKey(templateDraftVariants.value.map((variant) => String(variant.spec_key || '').trim()).filter(Boolean))
  templateDraftVariants.value.push(templateVariantDraftFromAPI({ sort_order: templateDraftVariants.value.length + 1, main_input_qty: 1, spec_key: specKey }, templateDraftVariants.value.length))
}

function removeTemplateVariant(index) {
  templateDraftVariants.value.splice(index, 1)
}

function setTemplateDefaultVariant(index) {
  if (!templateDraftVariants.value[index]?.is_default) return
  templateDraftVariants.value.forEach((variant, current) => { variant.is_default = current === index })
}

function addTemplateVariantItem(index) {
  const variant = templateDraftVariants.value[index]
  if (!variant) return
  const mode = templateVariantRecipeMode(variant)
  variant.items.push({
    local_key: `template-item-new-${Date.now()}-${variant.items.length}`,
    component_type: 'material',
    material_id: 0,
    component_product_id: 0,
    component_bom_spec_id: 0,
    component_spec_g: 0,
    component_bom_spec_options: [],
    component_bom_spec_loading: false,
    consume_unit: mode === 'ratio' ? 'ratio_pct' : '',
    qty_per_unit: 1,
    ratio_pct: 0,
  })
}

function removeTemplateVariantItem(variantIndex, itemIndex) {
  const variant = templateDraftVariants.value[variantIndex]
  if (!variant) return
  variant.items.splice(itemIndex, 1)
  const mode = templateVariantExplicitRecipeMode(variant.items)
  applyTemplateVariantRecipeMode(variant, mode)
}

function validatedTemplateVariantPayloads() {
  if (!templateDraftVariants.value.length) throw new Error('BOM 规格模板至少需要一个规格')
  assignVariantSpecKeys(templateDraftVariants.value)
  let defaultCount = 0
  const variants = templateDraftVariants.value.map((variant) => {
    const payload = templateVariantPayload(variant)
    if (!payload.name || !payload.inventory_unit) throw new Error('请填写每个规格的名称和库存单位')
    if (payload.is_default) defaultCount += 1
    return payload
  })
  if (defaultCount !== 1) throw new Error('BOM 规格模板必须且只能设置一个默认规格')
  return variants
}

async function saveSpecTemplateDraft() {
  const versionID = Number(selectedSpecTemplateVersionID.value || 0)
  if (!versionID || selectedSpecTemplateVersion.value?.status !== 'draft') return
  await mutate(async () => {
    await apiSend(`/api/production-bom-spec-template-versions/${versionID}/draft`, { method: 'PUT', body: { variants: validatedTemplateVariantPayloads() } })
    await selectSpecTemplateVersion(versionID)
    productionBomSpecTemplates.value = await loadProductionBomSpecTemplates()
    ok.value = '已保存 BOM 规格模板整组草稿'
  })
}

async function publishSpecTemplateVersion() {
  const versionID = Number(selectedSpecTemplateVersionID.value || 0)
  if (!versionID || selectedSpecTemplateVersion.value?.status !== 'draft') return
  await mutate(async () => {
    await apiSend(`/api/production-bom-spec-template-versions/${versionID}/draft`, { method: 'PUT', body: { variants: validatedTemplateVariantPayloads() } })
    await apiSend(`/api/production-bom-spec-template-versions/${versionID}/publish`, { body: {} })
    productionBomSpecTemplates.value = await loadProductionBomSpecTemplates()
    await selectSpecTemplateVersion(versionID)
    ok.value = '已整组发布 BOM 规格模板'
  })
}

async function loadAll({ strict = false } = {}) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [productData, materialData, unitData, processRouteData, productionGroupData, productionGroupSelectionData, productionBomData, specTemplateData] = await Promise.all([
      apiGet('/api/bom/products'),
      apiGet('/api/bom/materials'),
      loadProductUnitDefinitions(),
      apiGet('/api/process-routes?status=active'),
      apiGet('/api/business-groups'),
      apiGet('/api/business-group-feature-selections/production_bom'),
      apiGet('/api/production-boms?status=all'),
      loadProductionBomSpecTemplates(),
    ])

    products.value = (productData || []).map(normalizeBomProduct)
    materials.value = (materialData || []).map(normalizeBomMaterial)
    productUnitDefinitions.value = unitData || []
    processRoutes.value = processRouteData?.rows || processRouteData || []
    productionBomBusinessGroups.value = Array.isArray(productionGroupData?.rows) ? productionGroupData.rows : (Array.isArray(productionGroupData) ? productionGroupData : [])
    productionBomGroupFeatureSelectionTemplateIDs.value = businessGroupFeatureSelectionIDs(productionGroupSelectionData)
    productionBomGroupFeatureSelectionDraft.value = [...productionBomGroupFeatureSelectionTemplateIDs.value]
    productionBoms.value = (productionBomData.rows || productionBomData || []).map(normalizeProductionBomRecord)
    productionBomSpecTemplates.value = Array.isArray(specTemplateData) ? specTemplateData : []
    if (pendingProductionBomID.value > 0) {
      const pendingID = pendingProductionBomID.value
      pendingProductionBomID.value = 0
      const pendingRecord = productionBoms.value.find((row) => Number(row.id || row.production_bom_id || 0) === pendingID)
        || { id: pendingID, production_bom_id: pendingID }
      await openEditProductionBomRecord(pendingRecord)
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
    if (strict) throw err
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
  syncSelectedBomVariant()
  if (Number(versionID || 0) > 0) {
    selectedProductionBomVersionID.value = Number(versionID || 0)
    syncVersionMaterialLossRateFromSelectedVersion()
  }
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
  const currentID = Number(currentProductionBomID.value || 0)
  if (record.id !== currentID && !confirmDiscardBomWorkspaceChanges()) return
  if (record.id !== currentID) bomWorkspaceDirty.value = false
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

async function openBomRowPrimary(row) {
  await openEditProductionBomRecord(bomRecordFromRow(row))
}

function bomStatusLabel(status) {
  if (status === 'inactive') return '已失效'
  return '有效'
}

function productionBomRecordStatusLabel(status) {
  return status === 'inactive' ? '已失效' : '启用'
}

async function saveProductionBomVersionMeta() {
  if (!canEditCurrentBomItems.value) return
  await mutate(async () => {
    const versionID = Number(selectedProductionBomVersionID.value || 0)
    await saveProductionBomDraftItems(detailItems.value.map(productionBomDraftItemPayloadFromItem))
    ok.value = '已更新 BOM 草稿'
    await loadProductionBomDetailForVersion(currentProductionBomID.value, versionID)
  })
}

async function saveItem() {
  // 兼容历史验收标记“保存组件”：当前按钮文案为“添加到当前配方”，只写本地草稿。
  if (!canEditCurrentBomItems.value) return
  await mutate(async () => {
    if (itemForm.component_type === 'product' && Number(itemForm.component_bom_spec_id || 0) <= 0) {
      throw new Error('商品组件必须选择明确的已发布 BOM 规格')
    }
    const nextItems = detailItems.value.map(productionBomDraftItemFromItem)
    nextItems.push(productionBomDraftItemFromForm())
    if (selectedBomVariant.value) selectedBomVariant.value.items = nextItems
    else if (productionBomDetail.value) productionBomDetail.value.items = nextItems
    resetItemForm()
    markBomWorkspaceDirty()
  })
}

async function deleteItem(itemKey) {
  if (!canEditCurrentBomItems.value) return
  await mutate(async () => {
    const nextItems = removeProductionBomDraftItem(detailItems.value, itemKey)
    if (selectedBomVariant.value) selectedBomVariant.value.items = nextItems
    else if (productionBomDetail.value) productionBomDetail.value.items = nextItems
    markBomWorkspaceDirty()
  })
}

function openProductionBomGroupFeatureSelectionDrawer() {
  productionBomGroupFeatureSelectionDraft.value = [...productionBomGroupFeatureSelectionTemplateIDs.value]
  productionBomGroupFeatureDrawerOpen.value = true
}

function openBusinessGroupManagement() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'groupTemplates',
      returnNavigation: {
        key: 'productionConfig',
        label: '返回生产 BOM',
        params: { tab: 'bom' },
      },
    },
  }))
}

async function saveProductionBomFeatureSelection() {
  const payload = businessGroupFeatureSelectionPayload('production_bom', productionBomGroupFeatureSelectionDraft.value)
  productionBomGroupFeatureSelectionSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/business-group-feature-selections/production_bom', {
      method: 'PUT',
      body: payload,
    })
    productionBomGroupFeatureSelectionTemplateIDs.value = businessGroupFeatureSelectionIDs(result)
    productionBomGroupFeatureSelectionDraft.value = [...productionBomGroupFeatureSelectionTemplateIDs.value]
    productionBomCategoryMoveActive.value = false
    selectedBomRowKeys.value = []
    collapsedProductionBomGroups.value = []
    productionBomPaginationByGroup.value = {}
    productionBomGroupFeatureDrawerOpen.value = false
    ok.value = payload.group_template_ids.length
      ? `生产 BOM 已选择 ${payload.group_template_ids.length} 个分组模板`
      : '生产 BOM 已改为平铺展示'
  } catch (err) {
    error.value = err.message || '保存生产 BOM 分组模板失败'
  } finally {
    productionBomGroupFeatureSelectionSaving.value = false
  }
}

async function clearProductionBomBusinessGroupAssignment(bomID) {
  const id = Number(bomID || 0)
  if (!id) return
  const url = new URL('/api/business-group-assignments', window.location.origin)
  url.searchParams.set('usage_key', 'production_bom')
  url.searchParams.set('object_key', 'production_bom')
  url.searchParams.set('object_id', String(id))
  const data = await apiGet(url)
  const rows = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data?.assignments) ? data.assignments : [])
  await Promise.all(rows.map((row) => apiSend(`/api/business-group-assignments/${row.id}`, { method: 'DELETE' })))
}

function beginProductionBomCategoryMove() {
  if (!canBeginProductionBomCategoryMove.value || loading.value) return
  error.value = ''
  ok.value = ''
  productionBomCategoryMoveActive.value = true
}

function cancelProductionBomCategoryMove() {
  if (loading.value) return
  productionBomCategoryMoveActive.value = false
}

async function handleProductionBomCategoryMoveTarget(target) {
  if (!productionBomCategoryMoveActive.value || loading.value) return
  const completed = await moveSelectedProductBomsToGroup(target)
  if (completed) productionBomCategoryMoveActive.value = false
}

async function moveSelectedProductBomsToGroup(target = {}) {
  const targetGroupID = Number(target?.group_id || 0)
  const targetGroupItemID = Number(target?.group_item_id || 0)
  const moveToUnclassified = Boolean(target?.unclassified)
  const targetOption = moveToUnclassified
    ? null
    : { group_id: targetGroupID, group_item_id: targetGroupItemID }
  if (!moveToUnclassified && (!(targetGroupID > 0) || !(targetGroupItemID > 0))) return false
  const records = selectedBomRecordsForMove.value.filter((bom) => {
    if (!targetOption) return productionBomGroupID(bom) > 0 || productionBomGroupItemID(bom) > 0
    return productionBomGroupID(bom) !== Number(targetOption.group_id || 0) || productionBomGroupItemID(bom) !== Number(targetOption.group_item_id || 0)
  })
  if (!records.length) {
    error.value = selectedBomRecordsForMove.value.length
      ? '所选 BOM 已在该分类，请选择其他分类'
      : '请先勾选生产 BOM'
    return false
  }
  let completed = false
  await mutate(async () => {
    for (const bom of records) {
      if (!targetOption) {
        await clearProductionBomBusinessGroupAssignment(bom.id)
        continue
      }
      await apiSend('/api/business-group-assignments', {
        body: businessGroupMoveAssignmentPayload({
          usageKey: 'production_bom',
          objectKey: 'production_bom',
          objectID: Number(bom.id || 0),
          option: targetOption,
          sortOrder: 100,
        }),
      })
    }
    await loadAll({ strict: true })
    selectedBomRowKeys.value = []
    ok.value = `已移动 ${records.length} 个 BOM`
    completed = true
  })
  return completed
}

async function deactivateProductionBomRecords(records, successText) {
  await mutate(async () => {
    for (const bom of records) {
      await apiSend(`/api/production-boms/${bom.id}`, {
        method: 'PUT',
        body: {
          name: bom?.name || '',
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

async function saveProductionBomRecord() {
  const name = String(bomForm.name || '').trim()
  const outputID = Number(bomForm.output_id || 0)
  if (!name || outputID <= 0) return
  const binding = productionBomOutputPayload({ output_type: bomForm.output_type, output_id: outputID })
  const payload = {
    name,
    ...binding,
    output_qty: Number(bomForm.output_qty || 1),
    output_unit: outputUnitCode.value,
    spec_template_version_id: Number(bomForm.spec_template_version_id || 0),
    main_input_material_id: Number(bomForm.main_input_material_id || 0),
    status: bomForm.status === 'inactive' ? 'inactive' : 'active',
  }
	await mutate(async () => {
	  if (bomForm.mode === 'edit') {
	    const draftVersionID = Number(selectedProductionBomDraftVersion.value?.id || selectedProductionBomVersionID.value || 0)
	    if (!draftVersionID) throw new Error('请先选择可编辑的 BOM 草稿版本')
	    const isProductOutput = binding.output_type === 'product'
	    const workspaceRecipe = isProductOutput
	      ? { variants: validatedProductionBomDraftVariantPayloads() }
	      : { items: detailItems.value.map(productionBomDraftItemPayloadFromItem) }
	    await apiSend(`/api/production-boms/${bomForm.id}/draft-workspace`, {
	      method: 'PUT',
	      body: {
	        ...payload,
	        ...workspaceRecipe,
	        version_id: draftVersionID,
	        process_route_id: Number(currentRecipeTarget.value?.process_route_id || 0),
	        material_loss_rate: Number(selectedVersionMaterialLossRate.value || 0),
	      },
	    })
	    bomWorkspaceDirty.value = false
	    ok.value = '已保存 BOM 草稿；发布仍需单独操作'
	  } else if (bomForm.mode === 'copy') {
	      const copied = await apiSend(`/api/production-boms/${bomForm.source_id}/copy`, {
	        body: {
	          name: payload.name,
	          ...binding,
	          spec_template_version_id: payload.spec_template_version_id,
	          main_input_material_id: payload.main_input_material_id,
	        },
	      })
      ok.value = '已复制生产 BOM'
      pendingProductionBomID.value = Number(copied?.id || 0)
    } else {
      const created = await apiSend('/api/production-boms', { body: { name: payload.name, ...binding, output_qty: payload.output_qty, output_unit: payload.output_unit, spec_template_version_id: payload.spec_template_version_id, main_input_material_id: payload.main_input_material_id } })
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
  if (bomWorkspaceDirty.value) {
    error.value = '当前 BOM 草稿有未保存改动，请先保存 BOM 草稿后再发布'
    return
  }
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

async function setCurrentProductionBomAsDefault() {
  const output = currentOutputIdentity.value
  const outputID = output.id
  const bomID = currentProductionBomID.value
  const version = currentProductionBomDefaultVersion.value
  const versionID = Number(version?.id || 0)
  if (!outputID || !bomID || !versionID) {
    error.value = '当前生产 BOM 没有可用的发布版本'
    return
  }
  await mutate(async () => {
    const endpoint = output.type === 'material'
      ? `/api/materials/${outputID}/default-production-bom`
      : `/api/products/${outputID}/default-production-bom`
    await apiSend(endpoint, {
      method: 'PUT',
      body: {
        default_production_bom_id: bomID,
      },
    })
    ok.value = `已设为产出对象默认 BOM：${version?.version_no || versionID}`
    await loadAll()
    await loadProductionBomDetailForVersion(bomID, versionID)
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

watch([() => filters.status, productionBomSearchQuery], () => {
  selectedBomRowKeys.value = []
  resetProductionBomGroupPages()
})

watch(() => productionBomInlineListState.value.pagination, (pagination) => {
  if (JSON.stringify(pagination) === JSON.stringify(productionBomPaginationByGroup.value)) return
  productionBomPaginationByGroup.value = pagination
}, { deep: true, immediate: true })

watch([productionBomVisibleRows, productionBomCategoryMoveActive], () => {
  if (productionBomCategoryMoveActive.value) return
  const visibleKeys = new Set(visibleMovableBomRows.value.map(bomRowKey))
  const next = selectedBomRowKeys.value.filter((key) => visibleKeys.has(key))
  if (next.length !== selectedBomRowKeys.value.length) selectedBomRowKeys.value = next
})

watch(componentStockUnitCode, (unit) => {
  if (!unit || recipeConsumeMode.value === 'ratio') return
  itemForm.consume_unit = unit
  itemForm.ratio_pct = ''
})

function outputProductLabel(row = {}) {
  const productID = Number(row.output_product_id || 0)
  const name = row.output_product_name || productByID(productID)?.name || ''
  const code = row.output_product_code || ''
  if (name && code) return `${name} / ${code}`
  if (name) return name
  return productID > 0 ? `商品 #${productID}` : '-'
}
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
.grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; align-items: stretch; }
.list-panel { display: flex; flex-direction: column; }
.bom-business-group-inline-workspace { flex: 1 1 auto; min-height: 0; }
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
.bom-search-field input { min-width: min(340px, 100%); }
.bom-product-filter { min-width: min(280px, 100%); }
.bom-list-head { align-items: flex-start; }
.bom-list-filters { display: flex; align-items: flex-end; gap: 12px; flex-wrap: wrap; margin: 6px 0 12px; padding-top: 10px; border-top: 1px solid #eee8df; }
.bom-batch-deactivate-action { margin-left: auto; }
.feature-group-selection { display: grid; grid-template-columns: minmax(190px, .9fr) minmax(260px, 1.5fr) auto; gap: 10px; align-items: center; margin: 8px 0; padding: 10px; border: 1px solid #d9e2ec; border-radius: 8px; background: #f8fbff; }
.feature-group-selection-copy { display: grid; gap: 3px; }
.feature-group-selection-copy small { color: #607086; line-height: 1.4; }
.feature-group-selection-options, .feature-group-selection-actions { display: flex; align-items: center; gap: 8px 12px; flex-wrap: wrap; }
.feature-group-selection-actions { justify-content: flex-end; }
.feature-group-selection-option { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
.feature-group-selection-option input { width: auto; min-width: 0; height: auto; }
.bom-list-panel-scroll { min-width: 0; min-height: 0; overflow: auto; }
.bom-name-button { height: auto; min-height: 30px; text-align: left; font-weight: 700; }
.bom-record-form { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); align-items: start; gap: 12px; }
.bom-record-form > label { min-width: 0; }
.bom-record-form > label > input,
.bom-record-form > label > select { width: 100%; min-width: 0; }
.bom-record-form > label > small { display: block; margin-top: 5px; line-height: 1.35; }
.bom-record-form > .bom-spec-template-field { grid-column: 1; }
.bom-record-form-action { min-width: 0; }
.bom-record-form-action-spacer { display: block; margin-bottom: 5px; color: #666; font-size: 12px; visibility: hidden; }
.bom-record-form-action > button { justify-self: start; }
.bom-focus-filter { align-self: stretch; display: inline-flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid #dbeafe; border-radius: 8px; background: #eff6ff; color: #1d4ed8; font-size: 13px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 640px; border-collapse: collapse; }
.compact table { min-width: 520px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.select-col { width: 42px; text-align: center; }
.select-col input { min-width: 0; width: 16px; height: 16px; }
tbody tr.active { background: #f3f7fb; }
.list-panel tbody tr { cursor: default; }
.classification-group-row td { background: #f8f7f5; color: #333; border-top: 1px solid #e6e0d8; padding-left: var(--classification-group-indent, 16px); }
.classification-group-row strong { margin: 0 8px; }
.classification-template-row td { background: #ece6dc; border-top: 2px solid #d3c6b0; border-bottom: 1px solid #d3c6b0; color: #2f2820; padding-left: 8px; }
.classification-template-row strong { margin: 0 8px; font-size: 16px; }
.classification-template-row small { color: #6b5f4f; }
.classification-template-collapsed td { border-bottom: 2px solid #d3c6b0; }
.classification-group-toggle { height: 28px; border: 0; background: transparent; color: #1f4f82; padding: 0 4px; }
.classification-item-row td:first-child + td { padding-left: var(--classification-item-indent, 18px); }
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
.version-recipe-panel { border: 1px solid #e6e0d8; border-radius: 8px; margin-top: 12px; padding: 12px; background: #fbfaf8; }
.material-loss-control { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; padding: 8px 10px; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; }
.recipe-mode-hint { margin: 10px 0 4px; color: #5f5a54; font-size: 13px; line-height: 1.45; }
.checkbox-row.compact-checkbox { display: inline-flex; align-items: center; gap: 6px; min-height: 38px; margin: 0; }
.checkbox-row.compact-checkbox input { min-width: 0; width: 16px; height: 16px; }
.checkbox-row.compact-checkbox span { margin: 0; color: #333; font-size: 13px; font-weight: 700; }
.material-loss-rate-field small { display: block; max-width: 360px; color: #666; font-size: 12px; line-height: 1.35; margin-top: 4px; }
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
.bom-settings-drawer { width: min(1180px, 96vw); }
.spec-template-drawer { width: min(1240px, 98vw); }
.spec-template-workspace { display: grid; grid-template-columns: minmax(240px, .7fr) minmax(0, 2fr); gap: 16px; align-items: start; }
.spec-template-list { display: grid; gap: 8px; position: sticky; top: 0; }
.spec-template-list-row { display: grid; gap: 4px; width: 100%; height: auto; min-height: 54px; padding: 9px 11px; text-align: left; background: #fff; border-color: #d8d0c7; }
.spec-template-list-row small { color: #666; }
.spec-template-list-row.active { border-color: #1f4f82; background: #eef6ff; }
.spec-template-detail { min-width: 0; }
.version-row-list, .bom-variant-tabs { display: flex; flex-wrap: wrap; gap: 8px; margin: 8px 0 14px; }
.bom-spec-identity-form { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; align-items: start; margin: 10px 0 14px; padding: 12px; border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; }
.bom-spec-identity-form label { min-width: 0; }
.bom-spec-identity-form input, .bom-spec-identity-form select { width: 100%; min-width: 0; }
.bom-spec-identity-form small { display: block; margin-top: 4px; line-height: 1.35; }
.identity-action-spacer { display: block; margin-bottom: 5px; color: #666; font-size: 12px; visibility: hidden; }
.legacy-loss-display { margin: 0; align-self: center; }
.spec-template-version-editor, .bom-spec-group-panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fbfaf8; padding: 12px; margin: 12px 0; }
.bom-settings-detail { padding: 12px; border: 1px solid #d8d0c7; border-radius: 10px; background: #fff; }
.bom-spec-template-reapply-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)) auto; gap: 10px; align-items: start; padding: 10px; border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; }
.bom-spec-template-reapply-form label { min-width: 0; }
.bom-spec-template-reapply-form select { width: 100%; min-width: 0; }
.bom-spec-template-reapply-form small { display: block; margin-top: 4px; line-height: 1.35; }
.reapply-action-spacer { display: block; margin-bottom: 5px; color: #666; font-size: 12px; visibility: hidden; }
.spec-variant-card { border: 1px solid #d8d0c7; border-radius: 8px; background: #fff; padding: 12px; margin: 10px 0; }
.spec-variant-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; align-items: end; }
.spec-variant-grid label { min-width: 0; }
.spec-variant-grid input, .spec-variant-grid select { width: 100%; min-width: 0; }
.spec-variant-grid .checkbox-row input { width: 16px; height: 16px; min-width: 0; padding: 0; flex: 0 0 auto; }
.spec-variant-grid .checkbox-row span { flex: 1; }
.spec-variant-components { display: grid; gap: 8px; margin: 12px 0; padding-top: 10px; border-top: 1px solid #eee8df; }
.spec-component-row { display: grid; grid-template-columns: minmax(120px, .55fr) minmax(200px, 1.25fr) minmax(160px, 1fr) minmax(130px, .75fr) minmax(110px, .55fr) auto; gap: 8px; align-items: center; }
.spec-component-row select, .spec-component-row input { width: 100%; min-width: 0; }
.bom-settings-detail-target { min-width: 0; margin-top: 18px; padding-top: 16px; border-top: 1px solid #e6e0d8; }
.bom-settings-detail { min-width: 0; }
.drawer-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 14px; }
@media (max-width: 1100px) { .grid, .feature-group-selection { grid-template-columns: 1fr; } .feature-group-selection-actions { justify-content: flex-start; } }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .attrs-grid { grid-template-columns: 1fr; }
  .bom-record-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .spec-template-workspace { grid-template-columns: 1fr; }
  .spec-template-list { position: static; }
  .spec-variant-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .bom-spec-identity-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .bom-spec-template-reapply-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .bom-spec-template-reapply-form > button { justify-self: start; }
  .spec-component-row { grid-template-columns: 1fr 1fr; }
  table { min-width: 620px; }
}
@media (max-width: 600px) {
  .bom-record-form { grid-template-columns: 1fr; }
  .bom-record-form-action-spacer { display: none; }
  .bom-record-form-action > button { width: 100%; }
  .spec-variant-grid, .spec-component-row, .bom-spec-identity-form, .bom-spec-template-reapply-form { grid-template-columns: 1fr; }
  .bom-spec-template-reapply-form > button { width: 100%; justify-self: stretch; }
}
</style>
