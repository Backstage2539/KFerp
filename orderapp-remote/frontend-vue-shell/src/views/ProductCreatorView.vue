<template>
  <section class="product-creator">
    <div v-if="screen === 'list'" class="pc-list-page">
      <div class="pc-list-heading">
        <div>
          <div class="pc-eyebrow">商品 · 工作流模板</div>
          <h1>商品创建器</h1>
          <p>把商品、物料、配方和后续业务拼成模板，一次设计，重复使用。</p>
        </div>
        <button class="pc-primary" type="button" @click="newTemplate">
          <IconPlus :size="17" /> 新建模板
        </button>
      </div>

      <div class="pc-summary-row">
        <div class="pc-summary-card"><span>可用模板</span><strong>{{ publishedCount }}</strong><small>已发布，可直接创建</small></div>
        <div class="pc-summary-card"><span>我的草稿</span><strong>{{ draftCount }}</strong><small>未发布的流程设计</small></div>
        <div class="pc-summary-card"><span>最近创建</span><strong>{{ recentRuns.length }}</strong><small>{{ recentRuns[0] ? `${recentRuns[0].template_name} · ${runStatusLabel(recentRuns[0].status)}` : '暂无运行记录' }}</small></div>
        <div class="pc-list-actions"><label class="pc-search"><IconSearch :size="17" /><input v-model="search" placeholder="搜索模板" /></label></div>
      </div>

      <div v-if="loadError" class="pc-alert pc-alert-error">{{ loadError }}</div>
      <div v-else-if="loading" class="pc-loading">正在加载模板…</div>
      <div v-else-if="filteredTemplates.length" class="pc-template-grid">
        <article v-for="item in filteredTemplates" :key="item.id" class="pc-template-card">
          <div class="pc-card-top">
            <div class="pc-template-symbol"><IconSitemap :size="24" /></div>
            <span class="pc-state" :class="`state-${item.status}`">{{ statusLabel(item.status) }}</span>
          </div>
          <h2>{{ item.name }}</h2>
          <p class="pc-template-description">{{ item.description || '尚未添加模板说明' }}</p>
          <div class="pc-template-meta">
            <span>版本 {{ item.published_version ? `V${item.published_version}` : '未发布' }}</span>
            <span>{{ item.draft?.nodes?.length || 0 }} 个步骤</span>
            <span>{{ formatDate(item.updated_at) }}</span>
          </div>
          <div class="pc-card-footer">
            <button v-if="item.status === 'published'" class="pc-primary pc-small" type="button" @click="startRun(item)"><IconPlayerPlay :size="15" /> 使用模板</button>
            <button v-else class="pc-secondary pc-small" type="button" @click="editTemplate(item)"><IconPencil :size="15" /> 继续设计</button>
            <button class="pc-icon-button" type="button" aria-label="编辑模板" title="编辑模板" @click="editTemplate(item)"><IconPencil :size="17" /></button>
            <button class="pc-icon-button" type="button" aria-label="复制模板" title="复制模板" @click="copyTemplate(item)"><IconCopy :size="17" /></button>
            <button class="pc-icon-button" type="button" :aria-label="historyTemplateId === item.id ? '收起记录' : '查看版本和创建记录'" :title="historyTemplateId === item.id ? '收起记录' : '版本和创建记录'" @click="toggleHistory(item)"><IconClock :size="17" /></button>
            <button v-if="item.status !== 'disabled'" class="pc-icon-button danger" type="button" aria-label="停用模板" title="停用模板" @click="disableTemplate(item)"><IconArchive :size="17" /></button>
          </div>
          <div v-if="historyTemplateId === item.id" class="pc-template-history">
            <strong>创建记录</strong>
            <p v-if="historyLoading">正在读取记录…</p>
            <p v-else-if="!(templateHistory[item.id]?.runs || []).length">暂无创建记录</p>
            <button v-for="record in templateHistory[item.id]?.runs || []" :key="record.id" class="pc-history-run" type="button" :disabled="['completed', 'failed'].includes(record.status)" :title="record.status === 'draft' ? '继续填写此草稿' : ['config_committed', 'in_progress'].includes(record.status) ? '继续处理未完成的采购或价格步骤' : '已完成的运行记录为只读'" @click="continueRun(record)">
              <span>运行 #{{ record.id }} · V{{ record.template_version }}</span><b>{{ runStatusLabel(record.status) }}</b><small>{{ formatDateTime(record.updated_at) }}</small>
            </button>
            <strong class="pc-history-version-title">模板版本</strong>
            <p v-if="!(templateHistory[item.id]?.versions || []).length">尚无已发布版本</p>
            <div v-for="version in templateHistory[item.id]?.versions || []" :key="version.id" class="pc-history-version">V{{ version.version }} · {{ formatDateTime(version.published_at) }} · {{ version.published_by }}</div>
          </div>
        </article>
      </div>
      <div v-else class="pc-empty-state">
        <div class="pc-empty-icon"><IconSitemap :size="28" /></div>
        <h2>还没有商品流程模板</h2>
        <p>从空白流程开始，按你的业务添加商品、物料、BOM 和其他步骤。</p>
        <button class="pc-primary" type="button" @click="newTemplate"><IconPlus :size="17" /> 新建第一个模板</button>
      </div>
    </div>

    <div v-else-if="screen === 'designer'" class="pc-designer-page">
      <header class="pc-designer-header">
        <button class="pc-back" type="button" aria-label="返回模板列表" @click="leaveDesigner"><IconArrowLeft :size="20" /></button>
        <div class="pc-title-area">
          <div class="pc-title-line">
            <h1>商品创建器</h1>
            <input v-model="template.name" aria-label="模板名称" class="pc-template-name" placeholder="输入模板名称" />
            <span class="pc-state" :class="`state-${template.status || 'draft'}`">{{ statusLabel(template.status || 'draft') }}</span>
          </div>
          <p class="pc-saved-indicator"><IconCircleCheck :size="15" /> {{ dirty ? '有未保存的更改' : '已保存' }}<span v-if="template.revision"> · 修订 {{ template.revision }}</span></p>
        </div>
        <div class="pc-header-actions">
          <button class="pc-secondary" type="button" :disabled="saving" @click="saveTemplate">保存草稿</button>
          <button class="pc-secondary" type="button" @click="designerTab = designerTab === 'flow' ? 'form' : 'flow'">{{ designerTab === 'flow' ? '表单预览' : '流程设计' }}</button>
          <button class="pc-primary" type="button" :disabled="saving || nodes.length === 0" @click="publishTemplate"><IconSend :size="16" /> 发布模板</button>
        </div>
      </header>

      <div v-if="errorMessage" class="pc-alert pc-alert-error"><IconAlertTriangle :size="16" /> {{ errorMessage }}</div>

      <div v-if="designerTab === 'flow'" class="pc-designer-layout">
        <aside class="pc-module-library">
          <h2>业务模块</h2>
          <label class="pc-search"><IconSearch :size="16" /><input v-model="moduleSearch" placeholder="搜索模块" /></label>
          <div v-for="group in filteredModuleGroups" :key="group.name" class="pc-module-group">
            <h3>{{ group.name }}</h3>
            <button
              v-for="module in group.modules"
              :key="module.kind"
              class="pc-module-item"
              type="button"
              draggable="true"
              @dragstart="startModuleDrag($event, module.kind)"
              @click="addModule(module)">
              <component :is="moduleIcon(module.kind)" :size="19" />
              <span>{{ module.name }}</span>
              <IconGripVertical class="pc-module-grip" :size="16" />
            </button>
          </div>
          <div class="pc-library-tip"><IconInfoCircle :size="16" /> 拖入画布，连接业务步骤。</div>
        </aside>

        <div v-if="canvasVisible" class="pc-flow-area" @dragover.prevent @drop.prevent="dropModule">
          <div class="pc-flow-mode">
            <span>连线方式</span>
            <button type="button" :class="{ active: edgeMode === 'data' }" @click="edgeMode = 'data'"><IconLink :size="15" /> 数据来源</button>
            <button v-if="templateWorkflowVersion < 2" type="button" :class="{ active: edgeMode === 'prerequisite' }" @click="edgeMode = 'prerequisite'"><IconClock :size="15" /> 等待完成</button>
          </div>
          <VueFlow
            v-model:nodes="nodes"
            v-model:edges="edges"
            class="creator-vue-flow"
            :node-types="nodeTypes"
            :nodes-connectable="true"
            :nodes-draggable="true"
            :elements-selectable="true"
            :is-valid-connection="isValidConnection"
            :default-edge-options="edgeDefaults"
            fit-view-on-init
            @viewport-change="updateZoom"
            @connect="connectNodes"
            @edges-delete="reconcileDeletedEdges"
            @node-click="selectNode"
            @edge-click="selectEdge"
            @pane-click="clearSelection"
            @node-drag-start="startDragHistory"
            @node-drag-stop="finishDragHistory">
            <Background pattern-color="#d7e1eb" :gap="19" :size="1" />
            <MiniMap pannable zoomable :node-color="miniMapNodeColor" />
          </VueFlow>
          <div class="pc-canvas-toolbar">
            <button type="button" aria-label="撤销" title="撤销" :disabled="historyIndex <= 0" @click="undo"><IconArrowBackUp :size="18" /></button>
            <button type="button" aria-label="重做" title="重做" :disabled="historyIndex >= history.length - 1" @click="redo"><IconArrowForwardUp :size="18" /></button>
            <i></i>
            <button type="button" aria-label="缩小" title="缩小" @click="zoomOut"><IconMinus :size="18" /></button>
            <span>{{ zoomLabel }}</span>
            <button type="button" aria-label="放大" title="放大" @click="zoomIn"><IconPlus :size="18" /></button>
            <i></i>
            <button type="button" @click="autoArrange"><IconWand :size="17" /> 自动整理</button>
            <button v-if="templateWorkflowVersion >= 3" type="button" @click="variableManagerOpen = true">变量 · {{ workflowVariables.length }}</button>
            <button v-if="selectedNode" type="button" class="pc-delete-selection" @click="deleteSelected"><IconTrash :size="17" /> 删除</button>
          </div>
        </div>

        <div v-if="templateWorkflowVersion >= 3 && workflowUpgradeNotice" class="pc-upgrade-notice">旧模板已转换为 V3 草稿。历史发布版本和运行记录保持原样；保存或发布后才会应用新版变量与字段规则。</div>

        <aside class="pc-node-inspector">
          <template v-if="selectedNode">
            <div class="pc-inspector-heading">
              <h2>节点配置</h2>
              <button class="pc-icon-button" type="button" aria-label="取消选中" @click="clearSelection"><IconX :size="18" /></button>
            </div>
            <div class="pc-inspector-module">
              <div class="pc-inspector-icon" :class="`kind-${selectedNode.data.module.kind}`"><component :is="moduleIcon(selectedNode.data.module.kind)" :size="23" /></div>
              <div><strong>{{ selectedNode.data.label || selectedNode.data.module.name }}</strong><span>{{ selectedNode.data.module.description }}</span></div>
            </div>
            <label class="pc-field-label">节点名称</label>
            <input class="pc-control" :value="selectedNode.data.label" @input="updateNodeLabel($event.target.value)" />
            <template v-if="templateWorkflowVersion < 2">
            <label class="pc-field-label">执行动作</label>
            <select class="pc-control" :value="selectedNode.data.config.action || ''" @change="updateNodeConfig({ action: $event.target.value })">
              <option value="">选择处理方式</option>
              <option v-for="action in selectedNode.data.module.actions" :key="action" :value="action">{{ action }}</option>
            </select>
            </template>
            <section class="pc-inspector-section">
              <h3>数据来源</h3>
              <div v-if="incomingEdges.length" class="pc-source-list">
                <div v-for="edge in incomingEdges" :key="edge.id" class="pc-source-row">
                  <span>{{ edge.label || (edge.data?.kind === 'prerequisite' ? '等待步骤完成' : '业务数据') }}</span>
                  <strong>{{ edgeSourceLabel(edge) }}</strong>
                </div>
              </div>
              <p v-else class="pc-muted">本次填写或引用现有对象</p>
            </section>
            <section v-if="templateWorkflowVersion >= 2 && ['material', 'product'].includes(selectedNode.data.module.kind)" class="pc-inspector-section">
              <h3>对象用途</h3>
              <label class="pc-field-label">节点角色</label>
              <select class="pc-control" :value="selectedNode.data.config.data_role || 'input'" @change="updateNodeConfig({ data_role: $event.target.value })">
                <option value="input">配方输入／引用来源</option>
                <option value="output">接收 BOM 产出对象</option>
              </select>
              <template v-if="selectedNode.data.config.data_role === 'output'">
                <label class="pc-field-label">产出处理</label>
                <select class="pc-control" :value="selectedNode.data.config.object_action || 'create'" @change="updateNodeConfig({ object_action: $event.target.value })">
                  <option value="create">自动生成新档案</option>
                  <option value="reuse">使用时选择已有档案</option>
                </select>
                <label class="pc-field-label">命名默认值</label>
                <div class="pc-name-parts-editor">
                  <template v-for="(part, index) in selectedNameParts" :key="`${selectedNode.id}-${index}`">
                    <span v-if="part.type === 'variable'" class="pc-name-variable-chip">{{ variableName(part.variable_id) }}<button type="button" :aria-label="`移除变量${variableName(part.variable_id)}`" @click="removeNamePart(index)"><IconX :size="13" /></button></span>
                    <input v-else class="pc-control pc-name-literal" :value="part.value" placeholder="固定文字" @change="updateNamePart(index, { value: $event.target.value })" />
                  </template>
                  <button class="pc-text-action" type="button" @click="appendNameText"><IconPlus :size="14" />固定文字</button>
                </div>
                <div class="pc-variable-picker">
                  <input v-model="variablePickerQuery" class="pc-control" placeholder="搜索或新建命名变量" />
                  <button v-for="variable in filteredNameVariables" :key="variable.id" class="pc-variable-option" type="button" @click="appendNameVariable(variable.id)">{{ variable.name }}</button>
                  <button v-if="variablePickerQuery.trim() && !filteredNameVariables.some((variable) => variable.name.toLocaleLowerCase() === variablePickerQuery.trim().toLocaleLowerCase())" class="pc-variable-option create" type="button" @click="createAndAppendVariable">＋ 新建“{{ variablePickerQuery.trim() }}”</button>
                </div>
                <p class="pc-muted">使用时填写变量，名称会自动生成；填写者仍可直接修改名称。</p>
                <template v-if="selectedNode.data.module.kind === 'material'">
                  <p class="pc-muted">库存单位由连接的 BOM 产出单位带入，物料档案保持一个规格。</p>
                  <label v-if="templateWorkflowVersion < 3" class="pc-field-label">物料类别与取得方式</label>
                  <div class="pc-inline-controls">
                    <select v-if="templateWorkflowVersion < 3" class="pc-control" :value="selectedNode.data.config.kind || 'other'" @change="updateNodeConfig({ kind: $event.target.value })"><option value="other">通用物料（含半成品）</option><option value="pack">包装物料</option><option value="bean">原料</option></select>
                    <select class="pc-control" :value="selectedNode.data.config.supply_mode || 'manufacture'" @change="updateNodeConfig({ supply_mode: $event.target.value })"><option value="manufacture">自制</option><option value="purchase">外购</option></select>
                  </div>
                  <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('kind')" @change="setFieldEditable('kind', $event.target.checked)" /> 使用时允许修改物料类别</label>
                  <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('supply_mode')" @change="setFieldEditable('supply_mode', $event.target.checked)" /> 使用时允许修改取得方式</label>
                </template>
                <template v-else>
                  <label class="pc-field-label">默认归属</label>
                  <select class="pc-control" :value="selectedNode.data.config.owner || 'factory'" @change="updateNodeConfig({ owner: $event.target.value })"><option value="factory">本公司</option><option value="customer">客户</option></select>
                  <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('owner')" @change="setFieldEditable('owner', $event.target.checked)" /> 使用时允许修改归属</label>
                  <template v-if="selectedNode.data.config.owner === 'customer'">
                    <label class="pc-field-label">默认归属客户</label>
                    <select class="pc-control" :value="selectedNode.data.config.customer_id || 0" @change="updateNodeConfig({ customer_id: Number($event.target.value) })"><option :value="0">由使用者选择</option><option v-for="customer in customerOptions" :key="customer.id" :value="customer.id">{{ customer.name }}</option></select>
                    <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('customer_id')" @change="setFieldEditable('customer_id', $event.target.checked)" /> 使用时允许更换客户</label>
                  </template>
                  <p class="pc-muted">商品规格与默认规格由连接的 BOM 一处维护，并随 BOM 一起发布。</p>
                </template>
              </template>
            </section>
            <section v-if="templateWorkflowVersion >= 2 && selectedNode.data.module.kind === 'process'" class="pc-inspector-section">
              <h3>工艺默认配置</h3>
              <label class="pc-field-label">有效工艺路线</label>
              <select class="pc-control" :value="selectedNode.data.config.route_id || 0" @change="updateNodeConfig({ route_id: Number($event.target.value) })"><option :value="0">选择有效工艺路线</option><option v-for="route in processOptions" :key="route.id" :value="route.id">{{ route.name || route.route_name }}</option></select>
              <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('route_id')" @change="setFieldEditable('route_id', $event.target.checked)" /> 使用时允许更换路线</label>
            </section>
            <section v-if="templateWorkflowVersion >= 2 && selectedNode.data.module.kind === 'bom'" class="pc-inspector-section">
              <h3>BOM 默认配置</h3>
              <label class="pc-field-label">产出类型</label>
              <select class="pc-control" :value="selectedNode.data.config.output_type || 'material'" @change="changeBOMOutputType($event.target.value)"><option value="material">物料／半成品</option><option value="product">成品商品</option></select>
              <div class="pc-inline-controls">
                <label class="pc-field-label">产出数量<input class="pc-control" type="number" min="0.001" step="0.001" :value="selectedNode.data.config.output_qty ?? 1" @input="updateNodeConfig({ output_qty: Number($event.target.value) })" /></label>
                <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('output_qty')" @change="setFieldEditable('output_qty', $event.target.checked)" /> 使用时允许调整</label>
                <label class="pc-field-label">产出单位<select class="pc-control" :value="selectedNode.data.config.output_unit || ''" @change="updateNodeConfig({ output_unit: $event.target.value })"><option value="">按产出物料或默认规格</option><option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.name || unit.label || unit.code }}</option></select></label>
                <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('output_unit')" @change="setFieldEditable('output_unit', $event.target.checked)" /> 使用时允许调整</label>
              </div>
              <label class="pc-field-label">默认工艺路线</label>
              <select class="pc-control" :value="selectedNode.data.config.route_id || 0" :disabled="hasIncomingRoute(selectedNode.id)" @change="updateNodeConfig({ route_id: Number($event.target.value) })"><option :value="0">优先使用连线工艺</option><option v-for="route in processOptions" :key="route.id" :value="route.id">{{ route.name }}</option></select>
              <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('route_id')" @change="setFieldEditable('route_id', $event.target.checked)" /> 使用时允许调整工艺</label>
              <label class="pc-field-label">物料损耗率 %<input class="pc-control" type="number" min="0" max="99.99" step="0.01" :value="Number(selectedNode.data.config.material_loss_rate || 0) * 100" @input="updateNodeConfig({ material_loss_rate: Number($event.target.value || 0) / 100 })" /></label>
              <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('material_loss_rate')" @change="setFieldEditable('material_loss_rate', $event.target.checked)" /> 使用时允许调整损耗</label>
              <template v-if="selectedNode.data.config.output_type === 'product'">
                <h3>商品规格</h3>
                <div v-for="variant in selectedVariants" :key="variant.row_id" class="pc-variant-row">
                  <input class="pc-control" v-model="variant.name" placeholder="规格名称" @change="touchGraph" />
                  <select class="pc-control" v-model="variant.unit" @change="touchGraph"><option value="">选择单位</option><option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.name || unit.label || unit.code }}</option></select>
                  <label class="pc-default-variant"><input v-model="variant.is_default" type="radio" :name="`default-${selectedNode.id}`" :value="true" @change="setDefaultVariant(variant.row_id)" /> 默认</label>
                  <button class="pc-icon-button danger" type="button" aria-label="删除规格" @click="removeVariant(variant.row_id)"><IconTrash :size="16" /></button>
                </div>
                <button class="pc-text-action" type="button" @click="addVariant"><IconPlus :size="16" /> 添加规格</button>
                <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('variants')" @change="setFieldEditable('variants', $event.target.checked)" /> 使用时允许调整规格</label>
              </template>
              <template v-if="selectedNode.data.config.components?.length">
                <h3>配方默认值</h3>
                <div v-for="component in selectedBOMComponents" :key="component.row_id" class="pc-template-component-row">
                  <strong>{{ nodeName(component.source_node_id) }}</strong>
                  <div><label><span>默认用量</span><input class="pc-control" type="number" min="0" step="0.001" :value="component.quantity" placeholder="使用时填写" @input="updateBOMComponent(component.row_id, { quantity: Number($event.target.value || 0) })" /></label><label><span>消耗单位</span><select class="pc-control" :value="component.unit || ''" @change="updateBOMComponent(component.row_id, { unit: $event.target.value })"><option value="">使用时选择</option><option value="ratio_pct">比例 %</option><option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.name || unit.label || unit.code }}</option></select></label></div>
                </div>
                <label v-if="templateWorkflowVersion < 3" class="pc-edit-toggle"><input type="checkbox" :checked="!isFieldFixed('components')" @change="setFieldEditable('components', $event.target.checked)" /> 使用时允许调整配方行、用量和单位</label>
              </template>
            </section>
            <section v-if="templateWorkflowVersion < 2 && selectedNode.data.module.kind === 'bom'" class="pc-inspector-section">
              <h3>规格设置</h3>
              <div v-for="variant in selectedVariants" :key="variant.row_id" class="pc-variant-row">
                <input class="pc-control" v-model="variant.name" placeholder="规格名称" @change="touchGraph" />
                <input class="pc-control" v-model="variant.unit" placeholder="单位" @change="touchGraph" />
                <button class="pc-icon-button danger" type="button" aria-label="删除规格" @click="removeVariant(variant.row_id)"><IconTrash :size="16" /></button>
              </div>
              <button class="pc-text-action" type="button" @click="addVariant"><IconPlus :size="16" /> 添加规格</button>
            </section>
            <details v-if="templateWorkflowVersion < 2" class="pc-advanced">
              <summary>高级设置 · 执行条件</summary>
              <label class="pc-field-label">满足条件时执行</label>
              <select class="pc-control" :value="conditionMode" @change="setConditionMode($event.target.value)">
                <option value="always">始终执行</option>
                <option value="equals">上一步字段等于指定值</option>
                <option value="not_equals">上一步字段不等于指定值</option>
                <option value="not_empty">上一步字段不为空</option>
                <option value="empty">上一步字段为空</option>
              </select>
              <template v-if="conditionMode !== 'always'">
                <select class="pc-control" :value="selectedNode.data.condition?.node_id || ''" @change="setConditionSource($event.target.value)">
                  <option value="">选择前置步骤</option>
                  <option v-for="node in conditionSources" :key="node.id" :value="node.id">{{ node.data.label }}</option>
                </select>
                <select class="pc-control" :value="selectedNode.data.condition?.field || ''" @change="updateCondition({ field: $event.target.value })">
                  <option value="">选择判断字段</option>
                  <option v-for="field in conditionFieldOptions" :key="field.key" :value="field.key">{{ field.label }}</option>
                </select>
                <input v-if="conditionMode === 'equals' || conditionMode === 'not_equals'" class="pc-control" :value="selectedNode.data.condition?.value || ''" placeholder="条件值" @input="updateCondition({ value: $event.target.value })" />
              </template>
            </details>
            <div class="pc-inspector-actions">
              <button class="pc-secondary" type="button" @click="copySelected"><IconCopy :size="16" /> 复制节点</button>
              <button class="pc-danger-button" type="button" @click="deleteSelected"><IconTrash :size="16" /> 删除节点</button>
            </div>
          </template>
          <div v-else class="pc-inspector-empty">
            <div class="pc-inspector-empty-icon"><IconAdjustmentsHorizontal :size="24" /></div>
            <h3>节点配置</h3>
            <p>选择画布中的节点，配置动作、数据来源和执行条件。</p>
            <span>添加业务模块后，连接点会显示其可传递的数据。</span>
          </div>
        </aside>
        <footer class="pc-flow-status" :class="{ invalid: graphIssues.length }">
          <IconCircleCheck v-if="!graphIssues.length" :size="18" />
          <IconAlertTriangle v-else :size="18" />
          <strong>{{ graphIssues.length ? `发现 ${graphIssues.length} 个流程问题` : '流程检查通过' }}</strong>
          <span>{{ graphIssues[0]?.message || '连接点用于关联业务数据。' }}</span>
        </footer>
      </div>

      <div v-else class="pc-form-design-preview">
        <div class="pc-preview-heading"><div><h2>执行表单预览</h2><p>发布后，使用者会按此模板填写本次差异。</p></div><span>{{ nodes.length }} 个步骤 · {{ formFieldCount }} 个填写区域</span></div>
        <div v-if="!nodes.length" class="pc-empty-small">先在「流程设计」中添加业务模块。</div>
        <article v-for="node in orderedNodes" :key="node.id" class="pc-preview-step">
          <header><span class="pc-step-number">{{ stepNumber(node.id) }}</span><div><strong>{{ node.data.label }}</strong><small>{{ node.data.module.description }}</small></div></header>
          <div class="pc-preview-fields">
            <label v-for="field in node.data.module.fields" :key="field.key"><span>{{ field.label }}<b v-if="field.required">*</b></span><div>{{ field.type === 'repeater' ? '可添加多行' : field.description || '按模板设置填写' }}</div></label>
          </div>
        </article>
      </div>

      <div v-if="variableManagerOpen" class="pc-modal-backdrop" @click.self="variableManagerOpen = false">
        <section class="pc-variable-manager" role="dialog" aria-modal="true" aria-labelledby="pc-variable-title">
          <header><div><h2 id="pc-variable-title">模板命名变量</h2><p>变量在本模板的所有节点共用，每次运行单独填写。</p></div><button class="pc-icon-button" type="button" aria-label="关闭变量管理" @click="variableManagerOpen = false"><IconX :size="18" /></button></header>
          <div v-if="!workflowVariables.length" class="pc-empty-small">还没有变量。在节点命名配置中可直接创建。</div>
          <div v-for="variable in workflowVariables" :key="variable.id" class="pc-variable-row">
            <label><span>变量名称</span><input class="pc-control" :value="variable.name" @change="updateWorkflowVariable(variable.id, { name: $event.target.value })" /></label>
            <label><span>默认值</span><input class="pc-control" :value="variable.default_value || ''" @change="updateWorkflowVariable(variable.id, { default_value: $event.target.value })" /></label>
            <div><small>{{ variableReferences(variable.id) || '未被引用' }}</small><button class="pc-icon-button danger" type="button" :disabled="hasVariableReferences(variable.id)" :title="hasVariableReferences(variable.id) ? '先解除节点命名引用后才能删除' : '删除变量'" @click="removeWorkflowVariable(variable.id)"><IconTrash :size="16" /></button></div>
          </div>
          <footer><button class="pc-secondary" type="button" @click="addManagedVariable"><IconPlus :size="15" /> 添加变量</button><button class="pc-primary" type="button" @click="variableManagerOpen = false">完成</button></footer>
        </section>
      </div>
    </div>

    <ProductCreatorRunView
      v-else
      :run="run"
      :modules="modules"
      :busy="saving"
      :error="errorMessage"
      @back="leaveRun"
      @save="saveRunInputs"
      @preview="previewRun"
      @commit="commitRun"
      @execute="executeRunStep" />
  </section>
</template>

<script setup>
import { computed, markRaw, onMounted, ref, watch, nextTick } from 'vue'
import { Background } from '@vue-flow/background'
import { MiniMap } from '@vue-flow/minimap'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import {
  IconAdjustmentsHorizontal,
  IconAlertTriangle,
  IconArchive,
  IconArrowBackUp,
  IconArrowForwardUp,
  IconArrowLeft,
  IconBox,
  IconClock,
  IconCircleCheck,
  IconCopy,
  IconGripVertical,
  IconInfoCircle,
  IconLink,
  IconMinus,
  IconPackage,
  IconPencil,
  IconPlayerPlay,
  IconPlus,
  IconRoute,
  IconSearch,
  IconSend,
  IconShoppingCart,
  IconSitemap,
  IconTags,
  IconTrash,
  IconWand,
  IconX,
} from '@tabler/icons-vue'
import ProductCreatorNode from './ProductCreatorNode.vue'
import ProductCreatorRunView from './ProductCreatorRunView.vue'
import { apiGet } from '../api/client.js'
import {
  appendGraphSnapshot,
  cloneValue,
  autoLayout,
  canonicalInputPortID,
  connectionIsValid,
  makeEdgeId,
  makeNodeId,
  connectionRoleUpdates,
  moduleFieldLabel,
  moduleForNode,
  recipePortForEdge,
  toCanvasGraph,
  toWorkflowGraph,
} from '../lib/product-creator-graph.js'
import { filterWorkflowVariables, upgradeWorkflowToV3 } from '../lib/product-creator-variables.js'
import {
  copyProductCreatorTemplate,
  commitProductCreatorRun,
  executeProductCreatorRunStep,
  disableProductCreatorTemplate,
  getProductCreatorModules,
  listProductCreatorTemplates,
  listProductCreatorRuns,
  listProductCreatorVersions,
  listRecentProductCreatorRuns,
  getProductCreatorRun,
  previewProductCreatorRun,
  publishProductCreatorTemplate,
  saveProductCreatorRunDraft,
  saveProductCreatorTemplate,
  startProductCreatorRun,
  validateProductCreatorTemplate,
} from '../api/product-creator.js'

const nodeTypes = { business: markRaw(ProductCreatorNode) }
const edgeDefaults = { type: 'smoothstep', animated: false, style: { stroke: '#75839b', strokeWidth: 1.4 }, markerEnd: { type: 'arrowclosed', color: '#75839b' } }
const { fitView, zoomIn, zoomOut, screenToFlowCoordinate, getViewport, addEdges } = useVueFlow()
const screen = ref('list')
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const loadError = ref('')
const errorMessage = ref('')
const search = ref('')
const moduleSearch = ref('')
const templates = ref([])
const modules = ref([])
const recentRuns = ref([])
const historyTemplateId = ref(0)
const historyLoading = ref(false)
const templateHistory = ref({})
const template = ref(emptyTemplate())
const workflowVariables = ref([])
const variableManagerOpen = ref(false)
const workflowUpgradeNotice = ref(false)
const variablePickerQuery = ref('')
const run = ref(null)
const designerTab = ref('flow')
const nodes = ref([])
const edges = ref([])
const selectedNodeId = ref('')
const selectedEdgeId = ref('')
const edgeMode = ref('data')
const processOptions = ref([])
const unitOptions = ref([])
const customerOptions = ref([])
const graphIssues = ref([])
const history = ref([{ nodes: [], edges: [] }])
const historyIndex = ref(0)
const zoomLabel = ref('100%')
const canvasVisible = ref(true)
let draggedKind = ''

const selectedNode = computed(() => nodes.value.find((node) => node.id === selectedNodeId.value) || null)
const selectedVariants = computed(() => selectedNode.value?.data.config?.variants || [])
const selectedBOMComponents = computed(() => selectedNode.value?.data.config?.components || [])
const selectedNameParts = computed(() => selectedNode.value?.data.config?.name_parts || [])
const filteredNameVariables = computed(() => filterWorkflowVariables(variablePickerQuery.value, workflowVariables.value))
const templateWorkflowVersion = computed(() => Number(template.value.draft?.version || 1))
const incomingEdges = computed(() => edges.value.filter((edge) => edge.target === selectedNodeId.value))
const conditionSources = computed(() => nodes.value.filter((node) => node.id !== selectedNodeId.value))
const conditionSourceNode = computed(() => nodes.value.find((node) => node.id === selectedNode.value?.data.condition?.node_id))
const conditionFieldOptions = computed(() => conditionSourceNode.value?.data.module.fields || [])
const publishedCount = computed(() => templates.value.filter((item) => item.status === 'published').length)
const draftCount = computed(() => templates.value.filter((item) => item.status === 'draft').length)
const filteredTemplates = computed(() => templates.value.filter((item) => `${item.name} ${item.description}`.toLowerCase().includes(search.value.trim().toLowerCase())))
const groupedModules = computed(() => {
  const groups = new Map()
  const available = modules.value.filter((module) => templateWorkflowVersion.value >= 2
    ? module.palette_visible && Number(module.workflow_version) === templateWorkflowVersion.value
    : Number(module.workflow_version || 1) === 1)
  for (const module of available) {
    if (!groups.has(module.category)) groups.set(module.category, [])
    groups.get(module.category).push(module)
  }
  return [...groups.entries()].map(([name, rows]) => ({ name, modules: rows }))
})
const filteredModuleGroups = computed(() => groupedModules.value
  .map((group) => ({ ...group, modules: group.modules.filter((module) => `${module.name} ${module.description}`.toLowerCase().includes(moduleSearch.value.trim().toLowerCase())) }))
  .filter((group) => group.modules.length))
const orderedNodes = computed(() => {
  const order = topologyIds()
  return order.map((id) => nodes.value.find((node) => node.id === id)).filter(Boolean)
})
const formFieldCount = computed(() => nodes.value.reduce((count, node) => count + (node.data.module.fields?.length || 0), 0))

onMounted(load)

watch([nodes, edges], () => {
  if (screen.value === 'designer') dirty.value = true
}, { deep: true })

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const [moduleResult, templateResult, recentResult, routeResult, unitResult, customerResult] = await Promise.all([
      getProductCreatorModules(), listProductCreatorTemplates(), listRecentProductCreatorRuns(5), apiGet('/api/process-routes?status=active'), apiGet('/api/product-settings/units'), apiGet('/api/customers?limit=500&active=true'),
    ])
    modules.value = moduleResult.modules || []
    templates.value = templateResult.rows || []
    recentRuns.value = recentResult.rows || []
    processOptions.value = (routeResult.rows || routeResult.routes || []).filter((route) => route.status === undefined || route.status === 'active')
    unitOptions.value = (Array.isArray(unitResult) ? unitResult : unitResult.rows || unitResult.units || []).filter((unit) => unit.active !== false)
    customerOptions.value = (customerResult.rows || customerResult.items || []).filter((customer) => customer.active !== false && Number(customer.id) > 0)
  } catch (error) {
    loadError.value = error.message || '商品创建器加载失败'
  } finally {
    loading.value = false
  }
}

function emptyTemplate() {
  return { id: 0, revision: 0, name: '', description: '', status: 'draft', published_version: 0, draft: { version: 3, variables: [], nodes: [], edges: [] } }
}

function newTemplate() {
  errorMessage.value = ''
  template.value = emptyTemplate()
  workflowVariables.value = []
  workflowUpgradeNotice.value = false
  edgeMode.value = 'data'
  setCanvasGraph({ nodes: [], edges: [] })
  designerTab.value = 'flow'
  screen.value = 'designer'
  dirty.value = false
  resetHistory()
}

function editTemplate(item) {
  errorMessage.value = ''
  const sourceVersion = Number(item.draft?.version || 1)
  const upgradedWorkflow = upgradeWorkflowToV3(item.draft || { nodes: [], edges: [] })
  template.value = { ...cloneValue(item), draft: upgradedWorkflow }
  workflowVariables.value = cloneValue(upgradedWorkflow.variables || [])
  workflowUpgradeNotice.value = sourceVersion >= 2 && sourceVersion < 3
  edgeMode.value = 'data'
  setCanvasGraph(toCanvasGraph(upgradedWorkflow, modules.value))
  designerTab.value = 'flow'
  screen.value = 'designer'
  dirty.value = workflowUpgradeNotice.value
  resetHistory()
  nextTick(() => fitView({ padding: 0.22 }))
}

function setCanvasGraph(graph) {
  nodes.value = graph.nodes || []
  edges.value = graph.edges || []
  selectedNodeId.value = ''
  selectedEdgeId.value = ''
  checkGraph()
}

async function saveTemplate() {
  if (!template.value.name.trim()) {
    errorMessage.value = '请先填写模板名称。'
    return
  }
  saving.value = true
  errorMessage.value = ''
  try {
    const saved = await saveProductCreatorTemplate({
      id: template.value.id,
      revision: template.value.revision,
      name: template.value.name.trim(),
      description: template.value.description || '',
      workflow: toWorkflowGraph(nodes.value, edges.value, templateWorkflowVersion.value, workflowVariables.value),
    })
    template.value = { ...template.value, ...saved, draft: saved.draft || toWorkflowGraph(nodes.value, edges.value, templateWorkflowVersion.value, workflowVariables.value) }
    workflowVariables.value = cloneValue(template.value.draft.variables || [])
    workflowUpgradeNotice.value = false
    dirty.value = false
    await loadTemplatesOnly()
  } catch (error) {
    errorMessage.value = error.message || '模板保存失败'
    graphIssues.value = error.issues || []
  } finally {
    saving.value = false
  }
}

async function publishTemplate() {
  if (dirty.value || !template.value.id) await saveTemplate()
  if (!template.value.id || errorMessage.value) return
  saving.value = true
  errorMessage.value = ''
  try {
    const result = await validateProductCreatorTemplate(template.value.id)
    graphIssues.value = result.issues || []
    if (!result.valid) {
      errorMessage.value = '流程还有未完成的配置，请检查画布和节点提示。'
      return
    }
    const version = await publishProductCreatorTemplate(template.value.id, template.value.revision)
    template.value.status = 'published'
    template.value.published_version = version.version
    await loadTemplatesOnly()
    template.value = templates.value.find((item) => item.id === template.value.id) || template.value
    dirty.value = false
  } catch (error) {
    errorMessage.value = error.message || '发布模板失败'
    graphIssues.value = error.issues || []
  } finally {
    saving.value = false
  }
}

async function loadTemplatesOnly() {
  const result = await listProductCreatorTemplates()
  templates.value = result.rows || []
}

async function copyTemplate(item) {
  const name = window.prompt('新模板名称', `${item.name}（副本）`)
  if (!name?.trim()) return
  try {
    const copied = await copyProductCreatorTemplate(item.id, name.trim())
    await loadTemplatesOnly()
    editTemplate(copied)
  } catch (error) {
    loadError.value = error.message || '复制模板失败'
  }
}

async function disableTemplate(item) {
  if (!window.confirm(`停用「${item.name}」？已创建的运行记录仍会保留。`)) return
  try {
    await disableProductCreatorTemplate(item.id, item.revision)
    await loadTemplatesOnly()
  } catch (error) {
    loadError.value = error.message || '停用模板失败'
  }
}

async function startRun(item) {
  saving.value = true
  errorMessage.value = ''
  try {
    run.value = await startProductCreatorRun(item.id)
    screen.value = 'run'
  } catch (error) {
    errorMessage.value = error.message || '无法启动模板'
  } finally {
    saving.value = false
  }
}

async function toggleHistory(item) {
  if (historyTemplateId.value === item.id) {
    historyTemplateId.value = 0
    return
  }
  historyTemplateId.value = item.id
  historyLoading.value = true
  try {
    const [runs, versions] = await Promise.all([listProductCreatorRuns(item.id, 10), listProductCreatorVersions(item.id)])
    templateHistory.value = { ...templateHistory.value, [item.id]: { runs: runs.rows || [], versions: versions.rows || [] } }
  } catch (error) {
    errorMessage.value = error.message || '读取模板记录失败'
  } finally {
    historyLoading.value = false
  }
}

async function continueRun(record) {
  saving.value = true
  errorMessage.value = ''
  try {
    run.value = await getProductCreatorRun(record.id)
    screen.value = 'run'
  } catch (error) {
    errorMessage.value = error.message || '无法恢复此运行记录'
  } finally {
    saving.value = false
  }
}

function unpackRunDraft(payload) {
  if (payload && typeof payload === 'object' && payload.inputs) return payload
  return { inputs: payload || {}, variable_values: null }
}

async function saveRunInputs(payload) {
  if (!run.value) return
  const draft = unpackRunDraft(payload)
  saving.value = true
  errorMessage.value = ''
  try {
    run.value = await saveProductCreatorRunDraft(run.value.id, run.value.revision, draft.inputs, draft.variable_values)
  } catch (error) {
    errorMessage.value = error.message || '草稿保存失败'
  } finally {
    saving.value = false
  }
}

async function previewRun(payload) {
  if (!run.value) return
  const draft = payload ? unpackRunDraft(payload) : null
  saving.value = true
  errorMessage.value = ''
  try {
    if (draft) run.value = await saveProductCreatorRunDraft(run.value.id, run.value.revision, draft.inputs, draft.variable_values)
    run.value = await previewProductCreatorRun(run.value.id, run.value.revision)
    if (!run.value.preview?.valid) errorMessage.value = '请根据步骤提示补齐字段后再预览。'
  } catch (error) {
    errorMessage.value = error.message || '预览失败'
  } finally {
    saving.value = false
  }
}

async function commitRun(payload) {
  if (!run.value) return
  const draft = payload ? unpackRunDraft(payload) : null
  saving.value = true
  errorMessage.value = ''
  try {
    if (draft) run.value = await saveProductCreatorRunDraft(run.value.id, run.value.revision, draft.inputs, draft.variable_values)
    run.value = await previewProductCreatorRun(run.value.id, run.value.revision)
    if (!run.value.preview?.valid) {
      errorMessage.value = '请根据步骤提示补齐字段后再提交。'
      return
    }
    const idempotencyKey = `product-creator-${run.value.id}-revision-${run.value.revision}`
    try {
      run.value = await commitProductCreatorRun(run.value.id, run.value.revision, idempotencyKey)
    } catch (error) {
      // A lost response can occur after the server has committed. Refreshing
      // the run first makes recovery safe and avoids creating another set.
      const current = await getProductCreatorRun(run.value.id).catch(() => null)
      if (current && current.status !== 'draft') run.value = current
      else throw error
    }
  } catch (error) {
    errorMessage.value = error.message || '配置提交失败，草稿已保留，可修正后继续。'
    if (error.issues?.length) {
      const first = error.issues[0]
      const nodeName = run.value.workflow?.nodes?.find((node) => node.id === first.node_id)?.name || first.node_id
      errorMessage.value = `${nodeName ? `${nodeName}：` : ''}${first.message || errorMessage.value}`
    }
  } finally {
    saving.value = false
  }
}

async function executeRunStep(nodeID, action, inputs = {}) {
  if (!run.value) return
  saving.value = true
  errorMessage.value = ''
  const runID = run.value.id
  const revision = run.value.revision
  const idempotencyKey = `pc-run-${runID}-${nodeID}-${action}-rev-${revision}`
  try {
    run.value = await executeProductCreatorRunStep(runID, nodeID, revision, action, inputs, idempotencyKey)
  } catch (error) {
    const current = await getProductCreatorRun(runID).catch(() => null)
    const step = current?.business_results?.steps?.[nodeID]
    const completed = action === 'create_purchase_order' ? Boolean(step?.purchase_order)
      : action === 'confirm_receipt' ? Boolean(step?.purchase_receipt)
        : action === 'preview_pricing' ? Boolean(step?.pricing_preview)
          : action === 'save_price_draft' ? Boolean(step?.price_draft)
            : action === 'publish_price' ? Boolean(step?.published_price)
              : false
    if (completed) run.value = current
    else {
      if (current) run.value = current
      errorMessage.value = error.message || '后续业务步骤失败，已保留配置，可修正后重试。'
    }
  } finally {
    saving.value = false
  }
}

function leaveDesigner() {
  if (dirty.value && !window.confirm('当前模板有未保存更改，确定返回吗？')) return
  screen.value = 'list'
  errorMessage.value = ''
  loadTemplatesOnly().catch(() => {})
}

function leaveRun() {
  screen.value = 'list'
  run.value = null
  errorMessage.value = ''
}

function moduleIcon(kind) {
  return ({ product: IconBox, material: IconPackage, bom: IconSitemap, process: IconRoute, publish: IconSend, purchase: IconShoppingCart, pricing: IconTags })[kind] || IconBox
}

function startModuleDrag(event, kind) {
  draggedKind = kind
  event.dataTransfer?.setData('application/x-product-creator-module', kind)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'copy'
}

function dropModule(event) {
  const kind = event.dataTransfer?.getData('application/x-product-creator-module') || draggedKind
  const module = modules.value.find((item) => item.kind === kind && Number(item.workflow_version || 1) === templateWorkflowVersion.value && (templateWorkflowVersion.value < 2 || item.palette_visible))
  if (!module) return
  const position = screenToFlowCoordinate({ x: event.clientX, y: event.clientY })
  addModule(module, position)
  draggedKind = ''
}

function addModule(module, position = null) {
  rememberHistory()
  const count = nodes.value.filter((node) => node.data.module.kind === module.kind).length
  const config = module.kind === 'material' && templateWorkflowVersion.value >= 2
    ? { data_role: 'input', rows: [] }
    : module.kind === 'product' && templateWorkflowVersion.value >= 2
      ? { data_role: 'output', object_action: 'create', action: 'create', owner: 'factory' }
      : module.kind === 'process' && templateWorkflowVersion.value >= 2
        ? { route_id: 0 }
      : module.kind === 'bom' && templateWorkflowVersion.value >= 2
        ? { output_type: 'material', output_qty: 1, output_unit: '', route_id: 0, material_loss_rate: 0, variants: [], components: [] }
        : { action: '', variants: module.kind === 'bom' ? [{ row_id: makeNodeId(), name: '默认规格', unit: '件', is_default: true }] : [] }
  if (module.kind === 'bom' && templateWorkflowVersion.value >= 2) {
    config.variants = [{ row_id: makeNodeId(), name: '默认规格', unit: '', is_default: true }]
  }
  const resolvedModule = moduleForNode({ kind: module.kind, config }, modules.value, templateWorkflowVersion.value) || module
  const node = {
    id: makeNodeId(),
    type: 'business',
    position: position || { x: 85 + (nodes.value.length % 3) * 230, y: 85 + Math.floor(nodes.value.length / 3) * 185 },
    data: {
      module: resolvedModule,
      label: module.kind === 'bom' ? `BOM组装 ${count + 1}` : module.name,
      config,
    },
  }
  nodes.value.push(node)
  selectedNodeId.value = node.id
  selectedEdgeId.value = ''
  finishGraphChange()
}

function isValidConnection(connection) {
  return connectionIsValid(connection, nodes.value, modules.value, edgeMode.value, edges.value)
}

function connectNodes(connection) {
  if (!isValidConnection(connection)) {
    errorMessage.value = '这两个步骤的数据类型不匹配，请连接相同业务类型的端口。'
    return
  }
  rememberHistory()
  const source = nodes.value.find((node) => node.id === connection.source)
  const target = nodes.value.find((node) => node.id === connection.target)
  const edgeID = makeEdgeId()
  const canonicalTarget = canonicalInputPortID(connection.targetHandle)
  const label = edgeMode.value === 'prerequisite'
    ? '等待步骤完成'
    : `${moduleFieldLabel(source.data.module, connection.sourceHandle)} → ${moduleFieldLabel(target.data.module, canonicalTarget)}`
  const color = edgeMode.value === 'prerequisite' ? '#97a6b9' : '#71829a'
  const routedConnection = edgeMode.value === 'prerequisite'
    ? { ...connection, sourceHandle: '__prerequisite', targetHandle: '__prerequisite' }
    : { ...connection, targetHandle: target.data.module.kind === 'bom' && canonicalTarget === 'components' ? recipePortForEdge(edgeID) : connection.targetHandle }
  const edge = {
    id: edgeID, ...routedConnection, type: 'smoothstep', label,
    data: { kind: edgeMode.value },
    style: { stroke: color, strokeWidth: edgeMode.value === 'data' ? 1.55 : 1.3, ...(edgeMode.value === 'prerequisite' ? { strokeDasharray: '6 5' } : {}) },
    markerEnd: { type: 'arrowclosed', color },
  }
  addEdges([edge])
  if (edgeMode.value === 'data' && target.data.module.kind === 'bom' && canonicalTarget === 'route') {
    target.data = { ...target.data, config: { ...target.data.config, route_id: 0 } }
  }
  if (edgeMode.value === 'data' && target.data.module.kind === 'bom' && canonicalTarget === 'components') {
    const components = [...(target.data.config.components || [])]
    if (!components.some((row) => row.source_node_id === source.id && (row.source_handle || 'material') === connection.sourceHandle)) {
      components.push({ row_id: makeNodeId(), edge_id: edgeID, source_node_id: source.id, source_handle: connection.sourceHandle, quantity: '', unit: '', loss_rate: 0, variant_row_id: '' })
      target.data = { ...target.data, config: { ...target.data.config, components } }
    }
    refreshRecipeInputs(target.id)
  }
  for (const [nodeId, role] of Object.entries(connectionRoleUpdates(connection, nodes.value, edgeMode.value, edges.value))) {
    const node = nodes.value.find((item) => item.id === nodeId)
    if (node) node.data = { ...node.data, config: { ...node.data.config, data_role: role } }
  }
  selectedNodeId.value = target.id
  selectedEdgeId.value = ''
  errorMessage.value = ''
  finishGraphChange()
}

function selectNode({ node }) {
  selectedNodeId.value = node.id
  selectedEdgeId.value = ''
}

function selectEdge({ edge }) {
  selectedEdgeId.value = edge.id
  selectedNodeId.value = ''
}

function clearSelection() {
  selectedNodeId.value = ''
  selectedEdgeId.value = ''
}

function updateNodeLabel(label) {
  updateSelectedNode({ data: { ...selectedNode.value.data, label } })
}

function updateNodeConfig(patch) {
  const config = { ...selectedNode.value.data.config, ...patch }
  const nextModule = moduleForNode({ kind: selectedNode.value.data.module.kind, config }, modules.value, templateWorkflowVersion.value)
  updateSelectedNode({ data: { ...selectedNode.value.data, module: nextModule || selectedNode.value.data.module, config } })
}

function updateNamePart(index, patch) {
  const parts = selectedNameParts.value.map((part, partIndex) => partIndex === index ? { ...part, ...patch } : { ...part })
  updateNodeConfig({ name_parts: parts })
}

function appendNameText() {
  updateNodeConfig({ name_parts: [...selectedNameParts.value, { type: 'text', value: '' }] })
}

function appendNameVariable(variableID) {
  updateNodeConfig({ name_parts: [...selectedNameParts.value, { type: 'variable', variable_id: variableID }] })
  variablePickerQuery.value = ''
}

function removeNamePart(index) {
  updateNodeConfig({ name_parts: selectedNameParts.value.filter((_, partIndex) => partIndex !== index) })
}

function variableName(variableID) {
  return workflowVariables.value.find((variable) => variable.id === variableID)?.name || '缺失变量'
}

function upsertWorkflowVariable(name, defaultValue = '') {
  const trimmed = String(name || '').trim()
  if (!trimmed) return null
  const existing = workflowVariables.value.find((variable) => variable.name.toLocaleLowerCase() === trimmed.toLocaleLowerCase())
  if (existing) return existing
  rememberHistory()
  const variable = { id: makeNodeId(), name: trimmed, default_value: String(defaultValue || '') }
  workflowVariables.value = [...workflowVariables.value, variable]
  finishGraphChange()
  return variable
}

function createAndAppendVariable() {
  const variable = upsertWorkflowVariable(variablePickerQuery.value)
  if (variable) appendNameVariable(variable.id)
  variablePickerQuery.value = ''
}

function addManagedVariable() {
  const name = window.prompt('变量名称')
  if (name?.trim()) upsertWorkflowVariable(name)
}

function updateWorkflowVariable(variableID, patch) {
  rememberHistory()
  const nextValue = { ...patch }
  if (typeof nextValue.name === 'string') {
    nextValue.name = nextValue.name.trim()
    if (!nextValue.name || workflowVariables.value.some((variable) => variable.id !== variableID && variable.name.toLocaleLowerCase() === nextValue.name.toLocaleLowerCase())) {
      errorMessage.value = '变量名称不能为空或与其他变量重复。'
      return
    }
  }
  workflowVariables.value = workflowVariables.value.map((variable) => variable.id === variableID ? { ...variable, ...nextValue } : variable)
  finishGraphChange()
}

function variableReferences(variableID) {
  return nodes.value.filter((node) => (node.data.config?.name_parts || []).some((part) => part.type === 'variable' && part.variable_id === variableID)).map((node) => node.data.label).join('、')
}

function hasVariableReferences(variableID) {
  return nodes.value.some((node) => (node.data.config?.name_parts || []).some((part) => part.type === 'variable' && part.variable_id === variableID))
}

function removeWorkflowVariable(variableID) {
  if (hasVariableReferences(variableID)) {
    errorMessage.value = `变量仍被使用：${variableReferences(variableID)}。请先解除这些命名引用。`
    return
  }
  rememberHistory()
  workflowVariables.value = workflowVariables.value.filter((variable) => variable.id !== variableID)
  finishGraphChange()
}

function refreshRecipeInputs(nodeID) {
  const target = nodes.value.find((node) => node.id === nodeID)
  if (!target || target.data.module.kind !== 'bom') return
  const connected = edges.value.filter((edge) => edge.target === nodeID && canonicalInputPortID(edge.targetHandle) === 'components')
  target.data = { ...target.data, recipeInputs: [
    ...connected.map((edge) => {
      const source = nodes.value.find((node) => node.id === edge.source)
      return { id: edge.targetHandle, label: `${source?.data.label || source?.data.module.name || '配方来源'}${edge.sourceHandle === 'specs' ? ' · 商品规格' : ''}`, edgeId: edge.id }
    }),
    { id: 'components:add', label: '＋配方输入', add: true },
  ] }
}

function isFieldFixed(key) {
  return (selectedNode.value?.data.config?.fixed_fields || []).includes(key)
}

function setFieldEditable(key, editable) {
  const fixed = new Set(selectedNode.value?.data.config?.fixed_fields || [])
  if (editable) fixed.delete(key)
  else fixed.add(key)
  updateNodeConfig({ fixed_fields: [...fixed] })
}

function updateBOMComponent(rowID, patch) {
  const components = selectedBOMComponents.value.map((row) => row.row_id === rowID ? { ...row, ...patch } : { ...row })
  updateNodeConfig({ components })
}

function hasIncomingRoute(nodeID) {
  return edges.value.some((edge) => edge.target === nodeID && edge.targetHandle === 'route' && edge.data?.kind === 'data')
}

function nodeName(nodeID) {
  return nodes.value.find((node) => node.id === nodeID)?.data.label || '连接的配方来源'
}

function updateCondition(patch) {
  const current = selectedNode.value.data.condition || { node_id: '', field: 'action', operator: 'equals', value: '' }
  updateSelectedNode({ data: { ...selectedNode.value.data, condition: { ...current, ...patch } } })
}

function setConditionSource(nodeID) {
  const source = nodes.value.find((node) => node.id === nodeID)
  const field = source?.data.module.fields?.[0]?.key || ''
  updateCondition({ node_id: nodeID, field })
}

function setConditionMode(mode) {
  if (mode === 'always') {
    const { condition, ...data } = selectedNode.value.data
    updateSelectedNode({ data })
    return
  }
  const operator = mode === 'equals' ? 'equals' : mode
  updateCondition({ operator })
}

const conditionMode = computed(() => {
  const condition = selectedNode.value?.data.condition
  return condition?.operator || 'always'
})

function updateSelectedNode(patch) {
  if (!selectedNode.value) return
  rememberHistory()
  const index = nodes.value.findIndex((node) => node.id === selectedNodeId.value)
  nodes.value[index] = { ...nodes.value[index], ...patch }
  finishGraphChange()
}

function addVariant() {
  if (!selectedNode.value) return
  rememberHistory()
  const variants = [...selectedVariants.value, { row_id: makeNodeId(), name: '', unit: '', is_default: false }]
  updateNodeConfig({ variants })
}

function changeBOMOutputType(outputType) {
  const variants = selectedVariants.value.length
    ? selectedVariants.value
    : [{ row_id: makeNodeId(), name: '默认规格', unit: '', is_default: true }]
  if (outputType === 'product' && !variants.some((variant) => variant.is_default)) variants[0].is_default = true
  updateNodeConfig({ output_type: outputType, variants })
}

function setDefaultVariant(rowID) {
  if (!selectedNode.value) return
  updateNodeConfig({ variants: selectedVariants.value.map((variant) => ({ ...variant, is_default: variant.row_id === rowID })) })
}

function removeVariant(rowId) {
  if (!selectedNode.value) return
  rememberHistory()
  updateNodeConfig({ variants: selectedVariants.value.filter((variant) => variant.row_id !== rowId) })
}

function deleteSelected() {
  rememberHistory()
  if (selectedNodeId.value) {
    const deletedID = selectedNodeId.value
    const removedEdges = edges.value.filter((edge) => edge.source === deletedID || edge.target === deletedID)
    nodes.value = nodes.value.filter((node) => node.id !== deletedID)
    edges.value = edges.value.filter((edge) => edge.source !== deletedID && edge.target !== deletedID)
    pruneBOMRows(removedEdges)
  } else if (selectedEdgeId.value) {
    const removedEdges = edges.value.filter((edge) => edge.id === selectedEdgeId.value)
    edges.value = edges.value.filter((edge) => edge.id !== selectedEdgeId.value)
    pruneBOMRows(removedEdges)
  }
  clearSelection()
  finishGraphChange()
}

function reconcileDeletedEdges(deletedEdges) {
  const ids = new Set((deletedEdges || []).map((edge) => edge.id))
  const removed = deletedEdges || []
  edges.value = edges.value.filter((edge) => !ids.has(edge.id))
  pruneBOMRows(removed)
  finishGraphChange()
}

function pruneBOMRows(removedEdges) {
  const byTarget = new Map()
  for (const edge of removedEdges) {
    if (canonicalInputPortID(edge.targetHandle) !== 'components') continue
    byTarget.set(edge.target, [...(byTarget.get(edge.target) || []), edge])
  }
  for (const [targetID, edgesToRemove] of byTarget) {
    const target = nodes.value.find((node) => node.id === targetID)
    if (!target) continue
    const edgeIDs = new Set(edgesToRemove.map((edge) => edge.id))
    target.data = { ...target.data, config: { ...target.data.config, components: (target.data.config.components || []).filter((row) => !edgeIDs.has(row.edge_id) && !edgesToRemove.some((edge) => !row.edge_id && row.source_node_id === edge.source)) } }
    refreshRecipeInputs(targetID)
  }
}

function copySelected() {
  if (!selectedNode.value) return
  rememberHistory()
  const copy = cloneValue(selectedNode.value)
  copy.id = makeNodeId()
  copy.position = { x: copy.position.x + 45, y: copy.position.y + 46 }
  copy.data.label = `${copy.data.label} 副本`
  if (copy.data.condition) copy.data.condition.node_id = ''
  nodes.value.push(copy)
  selectedNodeId.value = copy.id
  finishGraphChange()
}

function edgeSourceLabel(edge) {
  const source = nodes.value.find((node) => node.id === edge.source)
  const port = source?.data.module.outputs?.find((item) => item.id === edge.sourceHandle)
  return `${source?.data.label || '未知步骤'}${port ? ` · ${port.label}` : ''}`
}

function miniMapNodeColor(node) {
  const colors = { product: '#d8f3e4', material: '#e0e9ff', bom: '#c9f2dc', process: '#e2e8f0', publish: '#dcf6ea', purchase: '#ffead5', pricing: '#f3e8ff' }
  return colors[node.data?.module?.kind] || '#e2e8f0'
}

function autoArrange() {
  rememberHistory()
  nodes.value = autoLayout(nodes.value, edges.value)
  finishGraphChange()
  nextTick(() => fitView({ padding: 0.22, duration: 250 }))
}

function topologyIds() {
  const incoming = new Map(nodes.value.map((node) => [node.id, 0]))
  const outgoing = new Map(nodes.value.map((node) => [node.id, []]))
  for (const edge of edges.value) {
    if (!incoming.has(edge.source) || !incoming.has(edge.target)) continue
    outgoing.get(edge.source).push(edge.target)
    incoming.set(edge.target, incoming.get(edge.target) + 1)
  }
  const ready = [...incoming.entries()].filter(([, count]) => count === 0).map(([id]) => id).sort()
  const order = []
  while (ready.length) {
    const id = ready.shift()
    order.push(id)
    for (const next of outgoing.get(id) || []) {
      incoming.set(next, incoming.get(next) - 1)
      if (incoming.get(next) === 0) ready.push(next)
    }
    ready.sort()
  }
  return order.length === nodes.value.length ? order : nodes.value.map((node) => node.id)
}

function stepNumber(id) {
  return Math.max(1, orderedNodes.value.findIndex((node) => node.id === id) + 1)
}

function checkGraph() {
  graphIssues.value = []
  if (!nodes.value.length) return
  const ids = new Set(nodes.value.map((node) => node.id))
  for (const node of nodes.value) {
    for (const port of node.data.module.inputs || []) {
      if (port.required && !edges.value.some((edge) => edge.target === node.id && canonicalInputPortID(edge.targetHandle) === port.id && (edge.data?.kind || 'data') === 'data')) {
        graphIssues.value.push({ node_id: node.id, field: port.id, message: `${port.label}需要连接一个数据来源。` })
      }
    }
    const condition = node.data.condition
    if (condition) {
      const source = nodes.value.find((candidate) => candidate.id === condition.node_id)
      if (!source || !source.data.module.fields?.some((field) => field.key === condition.field)) {
        graphIssues.value.push({ node_id: node.id, field: 'condition', message: '执行条件需要选择有效的前置步骤和判断字段。' })
      }
    }
  }
  for (const edge of edges.value) {
    if (!ids.has(edge.source) || !ids.has(edge.target)) graphIssues.value.push({ node_id: edge.target, message: '连线引用了已删除的步骤。' })
    if ((edge.data?.kind || 'data') !== 'data') continue
    const source = nodes.value.find((node) => node.id === edge.source)
    const target = nodes.value.find((node) => node.id === edge.target)
    const out = source?.data.module.outputs?.find((port) => port.id === edge.sourceHandle)
    const input = target?.data.module.inputs?.find((port) => port.id === canonicalInputPortID(edge.targetHandle))
    if (!out || !input || !out.types?.some((type) => input.types?.includes(type) || input.types?.includes('*'))) {
      graphIssues.value.push({ node_id: edge.target, edge_id: edge.id, message: '连线两端的数据类型不兼容。' })
    }
  }
  const order = topologyIds()
  if (order.length !== nodes.value.length) graphIssues.value.push({ code: 'cycle', message: '流程中存在循环依赖，请检查连线。' })
}

function touchGraph() {
  dirty.value = true
  checkGraph()
}

function finishGraphChange() {
  touchGraph()
  rememberHistory()
}

function graphSnapshot() {
  return cloneValue({ nodes: nodes.value, edges: edges.value, variables: workflowVariables.value })
}

function rememberHistory() {
  const next = appendGraphSnapshot(history.value, historyIndex.value, graphSnapshot())
  history.value = next.history
  historyIndex.value = next.index
}

function resetHistory() {
  history.value = [graphSnapshot()]
  historyIndex.value = 0
}

function restoreHistory(index) {
  if (index < 0 || index >= history.value.length) return
  const snapshot = cloneValue(history.value[index])
  nodes.value = snapshot.nodes
  edges.value = snapshot.edges
  workflowVariables.value = snapshot.variables || []
  historyIndex.value = index
  dirty.value = true
  clearSelection()
  checkGraph()
}

function undo() { restoreHistory(historyIndex.value - 1) }
function redo() { restoreHistory(historyIndex.value + 1) }
function startDragHistory() { rememberHistory() }
function finishDragHistory() { finishGraphChange() }

function statusLabel(status) {
  return ({ draft: '草稿', published: '已发布', disabled: '已停用' })[status] || '草稿'
}

function formatDate(value) {
  if (!value) return '刚刚更新'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '刚刚更新' : date.toLocaleDateString('zh-CN')
}

function formatDateTime(value) {
  if (!value) return '时间未知'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '时间未知' : date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function runStatusLabel(status) {
  return ({ draft: '草稿', config_committed: '配置完成', in_progress: '处理中', completed: '已完成', failed: '有失败步骤' })[status] || status || '未知状态'
}

function updateZoom() {
  const viewport = getViewport()
  zoomLabel.value = `${Math.round((viewport?.zoom || 1) * 100)}%`
}
</script>

<style>
@import '@vue-flow/core/dist/style.css';
@import '@vue-flow/core/dist/theme-default.css';
</style>

<style scoped>
.product-creator { min-height: calc(100vh - 88px); color: #142033; background: #f7f9fb; }
.pc-list-page { padding: 26px 30px 42px; }
.pc-list-heading, .pc-summary-row, .pc-card-top, .pc-card-footer, .pc-designer-header, .pc-title-line, .pc-header-actions, .pc-preview-heading, .pc-inspector-heading { display: flex; align-items: center; }
.pc-list-heading { justify-content: space-between; margin: 3px 0 25px; }
.pc-eyebrow { color: #73819a; font-size: 12px; letter-spacing: .04em; }
.pc-list-heading h1 { margin: 6px 0 6px; font-size: 28px; font-weight: 700; letter-spacing: -.02em; }
.pc-list-heading p { margin: 0; color: #748197; font-size: 14px; }
.pc-primary, .pc-secondary, .pc-danger-button { min-height: 40px; display: inline-flex; align-items: center; justify-content: center; gap: 8px; border-radius: 7px; padding: 0 16px; font: inherit; font-size: 14px; font-weight: 600; cursor: pointer; transition: background .15s, box-shadow .15s; }
.pc-primary { border: 1px solid #258153; color: white; background: #278456; box-shadow: 0 1px 2px #102a2414; }
.pc-primary:hover:not(:disabled) { background: #1f7047; }
.pc-primary:disabled, .pc-secondary:disabled { opacity: .48; cursor: not-allowed; }
.pc-secondary { border: 1px solid #d7dfe9; color: #26354b; background: white; }
.pc-secondary:hover:not(:disabled) { border-color: #a7b7c9; background: #f9fbfd; }
.pc-danger-button { border: 1px solid #f3c9c9; color: #b42318; background: #fff8f8; }
.pc-small { min-height: 34px; padding: 0 12px; font-size: 13px; }
.pc-summary-row { gap: 14px; margin-bottom: 20px; }
.pc-summary-card { width: 183px; min-height: 99px; border: 1px solid #e5eaf0; border-radius: 9px; padding: 13px 16px; background: white; box-shadow: 0 1px 2px #1c293712; }
.pc-summary-card span, .pc-summary-card small { display: block; color: #718097; font-size: 12px; }
.pc-summary-card strong { display: block; margin: 6px 0 4px; color: #1b2b40; font-size: 23px; line-height: 1; }
.pc-list-actions { display: flex; flex: 1; justify-content: flex-end; }
.pc-search { height: 38px; display: flex; align-items: center; gap: 9px; border: 1px solid #d8e0ea; border-radius: 7px; padding: 0 11px; color: #7c899c; background: white; }
.pc-search input { width: 170px; border: 0; outline: 0; color: #213049; background: transparent; font: inherit; font-size: 13px; }
.pc-template-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(275px, 1fr)); gap: 16px; }
.pc-template-card { min-width: 0; border: 1px solid #e1e7ee; border-radius: 10px; padding: 17px 17px 13px; background: white; box-shadow: 0 2px 5px #20354b08; }
.pc-card-top { justify-content: space-between; }
.pc-template-symbol, .pc-empty-icon, .pc-inspector-empty-icon { display: grid; place-items: center; width: 44px; height: 44px; border-radius: 9px; color: #228050; background: #e4f5eb; }
.pc-state { border: 1px solid #dce4ee; border-radius: 5px; padding: 3px 8px; color: #617089; background: #f8fafc; font-size: 11px; line-height: 1.35; white-space: nowrap; }
.state-published { border-color: #cdebd8; color: #24754a; background: #edf8f1; }
.state-disabled { color: #768399; background: #f0f2f5; }
.pc-template-card h2 { margin: 15px 0 6px; overflow: hidden; font-size: 17px; text-overflow: ellipsis; white-space: nowrap; }
.pc-template-description { min-height: 40px; margin: 0; color: #718097; font-size: 13px; line-height: 1.55; }
.pc-template-meta { display: flex; flex-wrap: wrap; gap: 8px 15px; border-top: 1px solid #eff2f6; margin-top: 12px; padding-top: 11px; color: #7c899c; font-size: 11px; }
.pc-template-history { display: grid; gap: 7px; border-top: 1px solid #edf0f4; margin-top: 12px; padding-top: 11px; color: #5e6d82; font-size: 11px; }
.pc-template-history > strong { color: #33445b; font-size: 11px; }
.pc-template-history > p { margin: 0; color: #8290a2; }
.pc-history-run { display: grid; grid-template-columns: 1fr auto; gap: 2px 7px; border: 1px solid #e7ebf0; border-radius: 5px; padding: 7px 8px; color: #3c4a60; background: #fbfcfd; text-align: left; cursor: pointer; }
.pc-history-run:disabled { cursor: default; opacity: .8; }
.pc-history-run small { grid-column: 1 / -1; color: #8995a5; }
.pc-history-run b { font-weight: 600; }
.pc-history-version-title { margin-top: 6px; }
.pc-history-version { color: #728095; }
.pc-card-footer { gap: 5px; margin-top: 13px; }
.pc-card-footer > .pc-primary, .pc-card-footer > .pc-secondary { margin-right: auto; }
.pc-icon-button { display: grid; place-items: center; width: 32px; height: 32px; border: 1px solid transparent; border-radius: 6px; color: #66758b; background: transparent; cursor: pointer; }
.pc-icon-button:hover { border-color: #e0e6ee; background: #f5f8fb; }
.pc-icon-button.danger:hover { color: #b42318; background: #fff3f2; }
.pc-empty-state { display: grid; justify-items: center; padding: 74px 24px; border: 1px dashed #d5dee9; border-radius: 12px; background: #ffffffb8; text-align: center; }
.pc-empty-icon { width: 54px; height: 54px; margin-bottom: 8px; }
.pc-empty-state h2 { margin: 8px 0; font-size: 19px; }
.pc-empty-state p { max-width: 460px; margin: 0 0 18px; color: #718097; font-size: 14px; }
.pc-loading { color: #75839a; padding: 40px 0; }
.pc-alert { display: flex; align-items: center; gap: 8px; border-radius: 7px; padding: 10px 13px; font-size: 13px; }
.pc-alert-error { border: 1px solid #f5c9c7; color: #9a261f; background: #fff6f5; }
.pc-designer-page { min-height: calc(100vh - 88px); background: white; }
.pc-designer-header { min-height: 89px; gap: 15px; border-bottom: 1px solid #e4e8ee; padding: 11px 20px; }
.pc-back { display: grid; flex: 0 0 39px; place-items: center; width: 39px; height: 39px; border: 1px solid #dbe3ed; border-radius: 7px; color: #23354c; background: white; cursor: pointer; }
.pc-title-area { flex: 1; min-width: 180px; }
.pc-title-line { gap: 12px; }
.pc-title-line h1 { margin: 0; font-size: 23px; white-space: nowrap; }
.pc-template-name { width: min(330px, 34vw); height: 35px; border: 0; border-bottom: 1px solid transparent; outline: 0; color: #27364c; background: transparent; font: inherit; font-size: 17px; }
.pc-template-name:focus { border-color: #7ebd98; }
.pc-saved-indicator { display: flex; align-items: center; gap: 5px; margin: 6px 0 0 1px; color: #318056; font-size: 12px; }
.pc-header-actions { gap: 8px; }
.pc-designer-layout { display: grid; grid-template-columns: 220px minmax(440px, 1fr) 337px; grid-template-rows: minmax(570px, calc(100vh - 223px)) 49px; min-height: 620px; }
.pc-module-library { grid-row: 1 / span 2; border-right: 1px solid #e3e8ee; padding: 17px 13px 14px; background: white; }
.pc-module-library h2, .pc-inspector-heading h2 { margin: 0 0 13px; font-size: 17px; }
.pc-module-library .pc-search { height: 35px; margin-bottom: 17px; }
.pc-module-library .pc-search input { width: 100%; }
.pc-module-group { margin: 12px 0 19px; }
.pc-module-group h3 { margin: 0 0 6px; color: #718097; font-size: 12px; font-weight: 600; }
.pc-module-item { display: flex; align-items: center; gap: 10px; width: 100%; min-height: 43px; border: 0; border-radius: 6px; padding: 0 8px; color: #18263b; background: transparent; text-align: left; cursor: grab; }
.pc-module-item:hover { background: #f1f7f4; color: #1c7447; }
.pc-module-item span { flex: 1; font-size: 13px; }
.pc-module-grip { color: #97a4b5; }
.pc-library-tip { display: flex; align-items: flex-start; gap: 8px; border-top: 1px solid #edf0f4; margin-top: 22px; padding-top: 13px; color: #76849a; font-size: 11px; line-height: 1.5; }
.pc-flow-area { position: relative; min-width: 0; min-height: 0; overflow: hidden; background: #f8fafc; }
.creator-vue-flow { width: 100%; height: 100%; background: radial-gradient(#e7edf3 .8px, transparent .8px); background-size: 20px 20px; }
.pc-flow-mode { position: absolute; z-index: 6; top: 12px; left: 14px; display: flex; align-items: center; gap: 4px; border: 1px solid #e1e7ee; border-radius: 7px; padding: 4px; background: #ffffffed; box-shadow: 0 2px 7px #25354b0c; }
.pc-flow-mode span { padding: 0 7px; color: #728097; font-size: 11px; }
.pc-flow-mode button { display: flex; align-items: center; gap: 5px; min-height: 29px; border: 0; border-radius: 5px; padding: 0 8px; color: #65748a; background: transparent; font: inherit; font-size: 11px; cursor: pointer; }
.pc-flow-mode button.active { color: #24784c; background: #e8f6ed; }
.pc-canvas-toolbar { position: absolute; z-index: 5; bottom: 14px; left: 15px; display: flex; align-items: center; gap: 5px; border: 1px solid #dce4ed; border-radius: 7px; padding: 4px; background: #fffffff2; box-shadow: 0 2px 8px #26364c12; }
.pc-canvas-toolbar button { display: inline-flex; align-items: center; justify-content: center; gap: 5px; height: 30px; min-width: 30px; border: 0; border-radius: 4px; padding: 0 7px; color: #34455d; background: transparent; font: inherit; font-size: 11px; cursor: pointer; }
.pc-canvas-toolbar button:hover:not(:disabled) { background: #f0f4f8; }
.pc-canvas-toolbar button:disabled { color: #b8c2cf; }
.pc-canvas-toolbar span { min-width: 39px; color: #34455d; font-size: 11px; text-align: center; }
.pc-canvas-toolbar i { width: 1px; height: 20px; margin: 0 2px; background: #e3e8ee; }
.pc-canvas-toolbar .pc-delete-selection { color: #b42318; }
.pc-node-inspector { overflow: auto; border-left: 1px solid #e3e8ee; padding: 17px 19px; background: white; }
.pc-inspector-heading { justify-content: space-between; }
.pc-inspector-heading h2 { margin-bottom: 10px; }
.pc-inspector-module { display: flex; align-items: center; gap: 12px; border-bottom: 1px solid #e9edf2; margin-bottom: 15px; padding: 4px 0 14px; }
.pc-inspector-icon { display: grid; place-items: center; flex: 0 0 44px; width: 44px; height: 44px; border-radius: 8px; color: #21804f; background: #e5f5eb; }
.pc-inspector-module strong, .pc-inspector-module span { display: block; }
.pc-inspector-module strong { font-size: 14px; }
.pc-inspector-module span { margin-top: 3px; color: #728097; font-size: 11px; line-height: 1.4; }
.pc-field-label { display: block; margin: 12px 0 6px; color: #36455b; font-size: 12px; font-weight: 600; }
.pc-edit-toggle { display: flex; align-items: center; gap: 6px; margin: 5px 0 9px; color: #78869a; font-size: 10px; font-weight: 500; }
.pc-template-component-row { display: grid; grid-template-columns: minmax(0, 1fr); gap: 7px; border: 1px solid #edf0f4; border-radius: 7px; margin: 8px 0; padding: 9px; background: #fbfcfd; }
.pc-template-component-row > strong { align-self: center; overflow: hidden; color: #40516a; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.pc-template-component-row > div { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 9px; }
.pc-template-component-row > div label { display: grid; gap: 4px; min-width: 0; }
.pc-template-component-row label span { display: block; margin-bottom: 4px; color: #78869a; font-size: 9px; }
.pc-template-component-row .pc-control { min-height: 32px; padding: 5px 6px; font-size: 10px; }
.pc-name-parts-editor { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin: 6px 0; }
.pc-name-literal { flex: 1 1 125px; min-width: 100px; }
.pc-name-variable-chip { display: inline-flex; align-items: center; gap: 5px; border: 1px solid #cbdcf5; border-radius: 14px; padding: 5px 8px; color: #315d98; background: #f0f5ff; font-size: 11px; }
.pc-name-variable-chip button { display: grid; place-items: center; border: 0; padding: 0; color: inherit; background: transparent; cursor: pointer; }
.pc-variable-picker { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, max-content)); gap: 5px; margin: 7px 0; }
.pc-variable-picker > input { grid-column: 1 / -1; }
.pc-variable-option { border: 1px solid #e0e7f0; border-radius: 5px; padding: 5px 8px; color: #53647d; background: white; text-align: left; font: inherit; font-size: 10px; cursor: pointer; }
.pc-variable-option.create { color: #226f49; background: #f4fbf6; }
.pc-upgrade-notice { grid-column: 1 / -1; border: 1px solid #d7e5f4; border-radius: 6px; margin: 0 12px; padding: 8px 11px; color: #526e91; background: #f4f8fd; font-size: 11px; }
.pc-modal-backdrop { position: fixed; inset: 0; z-index: 30; display: grid; place-items: center; padding: 20px; background: #15223866; }
.pc-variable-manager { width: min(720px, 100%); max-height: min(80vh, 680px); overflow: auto; border: 1px solid #e2e8f0; border-radius: 10px; padding: 18px; background: white; box-shadow: 0 18px 55px #15223833; }
.pc-variable-manager > header, .pc-variable-manager > footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.pc-variable-manager h2 { margin: 0 0 4px; font-size: 17px; }
.pc-variable-manager header p { margin: 0; color: #77859a; font-size: 11px; }
.pc-variable-row { display: grid; grid-template-columns: minmax(140px, .8fr) minmax(160px, 1fr) auto; align-items: end; gap: 10px; border-bottom: 1px solid #eef1f5; padding: 12px 0; }
.pc-variable-row label { display: grid; gap: 5px; color: #69788e; font-size: 10px; }
.pc-variable-row > div { display: flex; align-items: center; gap: 8px; }
.pc-variable-row small { color: #8290a2; font-size: 9px; }
.pc-variable-manager > footer { justify-content: flex-end; margin-top: 14px; }
@media (max-width: 620px) { .pc-variable-row { grid-template-columns: 1fr 1fr; } .pc-variable-row > div { grid-column: 1 / -1; justify-content: space-between; } }
.pc-control { width: 100%; min-height: 35px; border: 1px solid #dbe3ec; border-radius: 6px; padding: 7px 9px; outline: 0; color: #24334a; background: white; font: inherit; font-size: 12px; }
.pc-control:focus { border-color: #53a878; box-shadow: 0 0 0 2px #ccebd8; }
.pc-inspector-section { border-top: 1px solid #edf0f4; margin-top: 18px; padding-top: 14px; }
.pc-inspector-section h3 { margin: 0 0 10px; font-size: 13px; }
.pc-source-row { display: grid; grid-template-columns: 83px minmax(0, 1fr); align-items: center; gap: 8px; margin-bottom: 7px; color: #697890; font-size: 11px; }
.pc-source-row strong { overflow: hidden; border: 1px solid #dde5ed; border-radius: 5px; padding: 7px 8px; color: #34445b; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.pc-muted { margin: 0; color: #8491a3; font-size: 11px; }
.pc-variant-row { display: grid; grid-template-columns: minmax(0, 1fr) 74px 30px; gap: 5px; margin-bottom: 6px; }
.pc-variant-row .pc-control { min-height: 32px; padding: 5px 7px; }
.pc-text-action { display: inline-flex; align-items: center; gap: 4px; border: 0; padding: 6px 0; color: #2875b8; background: transparent; font: inherit; font-size: 12px; cursor: pointer; }
.pc-advanced { border-top: 1px solid #edf0f4; margin-top: 17px; padding-top: 13px; }
.pc-advanced summary { color: #35455b; font-size: 12px; font-weight: 600; cursor: pointer; }
.pc-advanced .pc-control { margin-bottom: 6px; }
.pc-inspector-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 20px; }
.pc-inspector-actions .pc-secondary, .pc-inspector-actions .pc-danger-button { min-height: 35px; padding: 0 8px; font-size: 11px; }
.pc-inspector-empty { display: grid; justify-items: center; padding: 110px 5px 30px; color: #617087; text-align: center; }
.pc-inspector-empty-icon { width: 44px; height: 44px; color: #5e738e; background: #edf2f6; }
.pc-inspector-empty h3 { margin: 12px 0 5px; color: #29384c; font-size: 14px; }
.pc-inspector-empty p { margin: 0 0 7px; font-size: 12px; line-height: 1.5; }
.pc-inspector-empty span { color: #8b97a7; font-size: 11px; }
.pc-flow-status { grid-column: 2 / span 2; display: flex; align-items: center; gap: 8px; border-top: 1px solid #e3e8ee; padding: 0 16px; color: #278152; background: white; font-size: 12px; }
.pc-flow-status.invalid { color: #b5473f; }
.pc-flow-status span { color: #748197; }
.pc-form-design-preview { max-width: 1000px; margin: 0 auto; padding: 27px; }
.pc-preview-heading { justify-content: space-between; margin-bottom: 18px; }
.pc-preview-heading h2 { margin: 0 0 5px; font-size: 19px; }
.pc-preview-heading p, .pc-preview-heading > span { margin: 0; color: #748197; font-size: 12px; }
.pc-preview-step { border: 1px solid #e2e8ef; border-radius: 8px; margin: 12px 0; background: white; }
.pc-preview-step header { display: flex; align-items: center; gap: 11px; border-bottom: 1px solid #edf0f4; padding: 13px 15px; }
.pc-step-number { display: grid; place-items: center; width: 25px; height: 25px; border-radius: 50%; color: #24764a; background: #e8f5ed; font-size: 12px; font-weight: 700; }
.pc-preview-step header strong, .pc-preview-step header small { display: block; }
.pc-preview-step header strong { font-size: 13px; }
.pc-preview-step header small { margin-top: 3px; color: #77849a; font-size: 11px; }
.pc-preview-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 12px; padding: 14px 15px; }
.pc-preview-fields label span, .pc-preview-fields label div { display: block; }
.pc-preview-fields label span { margin-bottom: 6px; color: #39475c; font-size: 11px; font-weight: 600; }
.pc-preview-fields label b { margin-left: 3px; color: #d13c32; }
.pc-preview-fields label div { border: 1px solid #e3e8ef; border-radius: 5px; padding: 9px; color: #8793a4; font-size: 11px; }
.pc-empty-small { border: 1px dashed #d5dee8; border-radius: 8px; padding: 45px; color: #748197; text-align: center; }
:deep(.vue-flow__node-business) { border: 0; background: transparent; box-shadow: none; }
:deep(.vue-flow__node.selected .creator-node) { border-color: #19804d; box-shadow: 0 0 0 2px #ccebd8, 0 3px 10px #20384b12; }
:deep(.vue-flow__handle) { width: 11px; height: 11px; border: 2px solid #7d8ca2; background: white; }
:deep(.vue-flow__handle:hover) { border-color: #278456; background: #e9f6ee; }
:deep(.creator-node-prerequisite-handle) { border-color: #91a0b2; background: #eef2f6; }
:deep(.vue-flow__edge-textbg) { fill: white; fill-opacity: .94; }
:deep(.vue-flow__edge-text) { fill: #64748b; font-size: 10px; }
:deep(.vue-flow__minimap) { right: 15px; bottom: 14px; overflow: hidden; border: 1px solid #d7e0e9; border-radius: 5px; background: #fffffff0; }
@media (max-width: 1120px) {
  .pc-designer-layout { grid-template-columns: 190px minmax(360px, 1fr) 295px; }
  .pc-module-library { padding-inline: 10px; }
  .pc-node-inspector { padding-inline: 14px; }
  .pc-title-line h1 { font-size: 20px; }
  .pc-template-name { width: 220px; }
}
@media (max-width: 860px) {
  .pc-list-page { padding: 18px 14px 30px; }
  .pc-summary-row { flex-wrap: wrap; }
  .pc-list-actions { flex-basis: 100%; justify-content: stretch; }
  .pc-list-actions .pc-search { flex: 1; }
  .pc-list-actions .pc-search input { width: 100%; }
  .pc-designer-header { flex-wrap: wrap; gap: 9px; padding: 12px; }
  .pc-title-area { min-width: 65%; }
  .pc-header-actions { width: 100%; justify-content: flex-end; }
  .pc-designer-layout { grid-template-columns: 155px minmax(250px, 1fr); grid-template-rows: minmax(480px, 62vh) auto 44px; }
  .pc-module-library { grid-row: 1 / span 3; }
  .pc-node-inspector { grid-column: 2; grid-row: 2; border-top: 1px solid #e3e8ee; border-left: 0; }
  .pc-inspector-empty { padding: 22px 8px; }
  .pc-flow-status { grid-column: 2; grid-row: 3; }
}
@media (max-width: 560px) {
  .pc-list-heading { align-items: flex-start; gap: 12px; }
  .pc-list-heading h1 { font-size: 24px; }
  .pc-list-heading p { max-width: 280px; font-size: 12px; }
  .pc-list-heading .pc-primary { padding: 0 10px; font-size: 12px; }
  .pc-summary-card { width: calc(50% - 7px); flex: 1 1 calc(50% - 7px); }
  .pc-designer-layout { display: flex; flex-direction: column; min-height: 0; }
  .pc-module-library { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; overflow-x: auto; border-right: 0; border-bottom: 1px solid #e3e8ee; padding: 8px; }
  .pc-module-library h2, .pc-module-library .pc-search, .pc-module-group h3, .pc-library-tip { display: none; }
  .pc-module-group { display: contents; }
  .pc-module-item { flex: 0 0 auto; width: auto; min-height: 36px; gap: 6px; border: 1px solid #e2e8ef; padding: 0 8px; }
  .pc-module-item span { font-size: 11px; white-space: nowrap; }
  .pc-module-grip { display: none; }
  .pc-flow-area { flex: 0 0 57vh; min-height: 365px; }
  .pc-node-inspector { overflow: visible; }
  .pc-flow-status { min-height: 44px; flex-wrap: wrap; padding: 6px 10px; font-size: 10px; }
  .pc-flow-status span { flex-basis: 100%; }
  .pc-template-name { width: min(48vw, 220px); font-size: 14px; }
  .pc-title-line h1 { font-size: 17px; }
  .pc-header-actions > button { min-height: 34px; padding: 0 9px; font-size: 11px; }
}
</style>
