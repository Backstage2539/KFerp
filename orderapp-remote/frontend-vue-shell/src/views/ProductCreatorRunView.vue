<template>
  <section class="pc-run-page">
    <header class="pc-run-header">
      <button class="pc-run-back" type="button" aria-label="返回模板列表" @click="$emit('back')"><IconArrowLeft :size="19" /></button>
      <div class="pc-run-title"><span>使用已发布模板 · V{{ run.template_version }}</span><h1>{{ templateName }}</h1><p>填写本次需要新建或引用的商品数据，预览通过后再提交。</p></div>
      <div class="pc-run-actions">
        <template v-if="run.status === 'draft'">
          <button class="pc-run-secondary" type="button" :disabled="busy" @click="$emit('save', cloneRunDraft())"><IconDeviceFloppy :size="16" /> 保存草稿</button>
          <button class="pc-run-secondary" type="button" :disabled="busy" @click="$emit('preview', cloneRunDraft())"><IconEye :size="16" /> 业务预览</button>
          <button v-if="run.preview?.valid" class="pc-run-primary" type="button" :disabled="busy" @click="$emit('commit', cloneRunDraft())"><IconSend :size="16" /> 提交创建</button>
        </template>
        <span v-else class="pc-run-committed-badge"><IconCircleCheck :size="16" /> {{ run.status === 'in_progress' ? '配置已提交 · 后续步骤待处理' : '配置已完成' }}</span>
      </div>
    </header>

    <div v-if="error" class="pc-run-alert"><IconAlertTriangle :size="16" /> {{ error }}</div>
    <div v-if="optionError" class="pc-run-alert"><IconAlertTriangle :size="16" /> {{ optionError }}</div>

    <section v-if="run.status !== 'draft'" class="pc-run-result-card">
      <header><IconCircleCheck :size="19" /><div><strong>正式业务档案已创建</strong><span>配置结果已保存；重复刷新或恢复不会再次创建。</span></div></header>
      <div v-if="resultRows.length" class="pc-run-created-objects">
        <div v-for="row in resultRows" :key="`${row.nodeID}-${row.type}-${row.name}`"><span>{{ row.stepName }} · {{ row.typeName }}</span><strong>{{ row.name }}</strong><small v-if="row.code">{{ row.code }}</small></div>
      </div>
      <div v-if="workflowVersion >= 5 && bomResultNodes.length" class="pc-run-bom-route-results">
        <article v-for="node in bomResultNodes" :key="node.id">
          <strong>{{ node.data.label || node.data.module.name }}</strong>
          <span>{{ processRouteResultSourceLabel(bomExecutionResult(node.id).process_route?.source) }} · {{ bomExecutionResult(node.id).process_route?.name || '规格模板各规格默认工艺' }}</span>
          <small v-for="route in bomExecutionResult(node.id).effective_specification_routes || []" :key="route.spec_key">{{ route.spec_key }} · {{ route.route_name || '未设置工艺' }}</small>
        </article>
      </div>
      <div v-for="node in purchaseNodes" :key="node.id" class="pc-run-followup">
        <div class="pc-run-followup-heading"><strong>{{ node.data.label || node.data.module.name }}</strong><span v-if="purchaseReceipt(node.id)" class="pc-run-followup-done">已完成收货</span><span v-else-if="purchaseOrder(node.id)" class="pc-run-followup-waiting">采购单已创建 · 等待到货</span><span v-else class="pc-run-followup-waiting">待创建采购单</span></div>
        <div v-if="purchaseOrder(node.id)" class="pc-run-purchase-order"><span>采购单 {{ purchaseOrder(node.id).order_no }}</span><span>状态：{{ purchaseOrder(node.id).status }}</span></div>
        <button v-else class="pc-run-primary" type="button" :disabled="busy" @click="$emit('execute', node.id, 'create_purchase_order', {})"><IconShoppingCart :size="16" /> 创建采购单</button>
        <div v-if="purchaseOrder(node.id) && !purchaseReceipt(node.id)" class="pc-run-receipt-form">
          <label><span>本次实收数量</span><input v-model.number="followupValues(node.id).quantity" type="number" min="0.001" step="0.001" /></label>
          <label><span>实际采购单价</span><input v-model.number="followupValues(node.id).unit_price" type="number" min="0" step="0.01" /></label>
          <button class="pc-run-primary" type="button" :disabled="busy || followupValues(node.id).quantity <= 0" @click="$emit('execute', node.id, 'confirm_receipt', cloneValue(followupValues(node.id)))"><IconCircleCheck :size="16" /> 确认收货并入库</button>
          <small>确认后才会更新库存批次和实际采购成本；该动作不会随配置提交自动发生。</small>
        </div>
        <div v-if="purchaseReceipt(node.id)" class="pc-run-purchase-order"><span>收货单 {{ purchaseReceipt(node.id).receipt_no }}</span><span>入库批次 {{ purchaseReceipt(node.id).stock_batch_code }}</span></div>
      </div>
      <div v-for="node in pricingNodes" :key="node.id" class="pc-run-followup">
        <div class="pc-run-followup-heading"><strong>{{ node.data.label || node.data.module.name }}</strong><span v-if="publishedPrice(node.id)" class="pc-run-followup-done">价格已发布 {{ publishedPrice(node.id).version }}</span><span v-else-if="priceDraft(node.id)" class="pc-run-followup-waiting">草稿已保存 · 待发布</span><span v-else class="pc-run-followup-waiting">待试算</span></div>
        <div v-if="pricingPreview(node.id)" class="pc-run-price-preview"><div v-for="row in pricingPreview(node.id).prices || []" :key="row.spec_row_id"><span>{{ row.spec_name }} · {{ row.unit }}</span><strong>¥{{ Number(row.price || 0).toFixed(2) }}</strong></div></div>
        <div class="pc-run-price-actions" v-if="stepResult(node.id).status !== 'skipped'">
          <button v-if="!pricingPreview(node.id)" class="pc-run-secondary" type="button" :disabled="busy" @click="$emit('execute', node.id, 'preview_pricing', {})"><IconEye :size="16" /> 试算价格</button>
          <button v-if="pricingPreview(node.id) && !priceDraft(node.id)" class="pc-run-primary" type="button" :disabled="busy" @click="$emit('execute', node.id, 'save_price_draft', {})"><IconDeviceFloppy :size="16" /> 保存价格草稿</button>
          <button v-if="priceDraft(node.id) && !publishedPrice(node.id)" class="pc-run-primary" type="button" :disabled="busy" @click="$emit('execute', node.id, 'publish_price', {})"><IconTags :size="16" /> 发布价格</button>
        </div>
        <div v-if="publishedPrice(node.id)" class="pc-run-purchase-order"><span>{{ publishedPrice(node.id).table_name }}</span><span>{{ publishedPrice(node.id).status }}</span></div>
        <small class="pc-run-help">本次会复制目标价格表生成新版本，保留表内其他商品；只有明确发布后才进入销售使用。</small>
      </div>
      <p v-if="run.status === 'in_progress' && !purchaseNodes.length && !pricingNodes.length">后续步骤已记录；请按模板配置完成其他待处理动作。</p>
    </section>

    <div class="pc-run-layout">
      <main class="pc-run-form">
        <fieldset class="pc-run-readonly" :disabled="run.status !== 'draft'">
        <div class="pc-run-section-heading"><div><h2>创建信息</h2><p>带 <b>*</b> 的字段需要补齐后才能预览。</p></div><span>运行草稿 #{{ run.id }} · 修订 {{ run.revision }}</span></div>
        <section v-if="workflowVersion >= 3 && usedWorkflowVariables.length" class="pc-run-variables">
          <header><div><strong>本次变量</strong><small>每个变量只需填写一次，名称会自动带入对应物料和商品。</small></div></header>
          <label v-for="variable in usedWorkflowVariables" :key="variable.id"><span>{{ variable.name }}<small v-if="variable.default_value">默认：{{ variable.default_value }}</small></span><input v-model="variableValues[variable.id]" :placeholder="variable.default_value || `填写${variable.name}`" /></label>
        </section>
        <section v-for="(node, index) in orderedNodes" :key="node.id" class="pc-run-step" :class="{ 'has-issue': issueForNode(node.id) }">
          <header class="pc-run-step-heading">
            <span class="pc-run-step-number" :class="`kind-${node.data.module.kind}`">{{ index + 1 }}</span>
            <div><h3>{{ node.data.label || node.data.module.name }}</h3><p>{{ node.data.module.kind === 'bom' ? bomIdentity(node) : node.data.module.description }}</p></div>
            <span class="pc-run-step-state" :class="stepStatusClass(node.id)">{{ stepStatusLabel(node.id) }}</span>
          </header>
          <div class="pc-run-step-body">
            <div v-if="workflowVersion >= 2" class="pc-bom-run-fields">
              <template v-if="node.data.module.kind === 'material' && node.data.config.data_role !== 'output'">
                <div class="pc-repeater-caption">配方物料 <small>搜索已有物料，或直接在当前行新建</small></div>
                <div v-for="row in ensureMaterialRows(node.id)" :key="row.row_id" class="pc-material-row">
                  <label class="pc-row-field"><span>处理方式</span><select v-model="row.action" @change="handleMaterialAction(node.id, row)"><option value="reuse">引用已有</option><option value="create">新建物料</option></select></label>
                  <template v-if="row.action === 'reuse'">
                    <label class="pc-row-field pc-row-wide"><span>搜索物料名称、编码或规格 *</span><input :value="materialSearchText[materialSearchKey(node.id, row.row_id)] || selectedMaterialLabel(row)" placeholder="输入关键词，例如：云南水洗豆" @input="searchMaterialForRow(node.id, row, $event.target.value)" /></label>
                    <div v-if="materialSearchResults[materialSearchKey(node.id, row.row_id)]?.length" class="pc-material-search-results pc-row-wide">
                      <button v-for="option in materialSearchResults[materialSearchKey(node.id, row.row_id)]" :key="option.id" type="button" @click="selectMaterialForRow(node.id, row, option)"><strong>{{ option.name }}</strong><span>{{ option.code || '无编码' }} · {{ option.unit || '无单位' }} · {{ option.owner_label }}</span></button>
                      <button v-if="materialSearchHasMore[materialSearchKey(node.id, row.row_id)]" class="pc-material-search-more" type="button" @click="searchMoreMaterials(node.id, row)">查看更多</button>
                    </div>
                    <small v-if="row.material_id" class="pc-run-help pc-row-wide">已选择：{{ selectedMaterialLabel(row) }} · 库存单位 {{ row.unit }}</small>
                  </template>
                  <template v-else>
                    <label class="pc-row-field"><span>物料名称 *</span><input v-model.trim="row.name" placeholder="例如：云南水洗豆" /></label>
                    <label v-if="workflowVersion < 3" class="pc-row-field"><span>物料类别</span><select v-model="row.kind"><option value="bean">原料</option><option value="pack">包材</option><option value="other">其他</option></select></label>
                    <label class="pc-row-field"><span>取得方式</span><select v-model="row.supply_mode"><option value="purchase">外购</option><option value="manufacture">自制</option></select></label>
                    <label class="pc-row-field"><span>库存单位 *</span><select v-model="row.unit"><option value="">选择单位</option><option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option></select></label>
                    <label class="pc-row-field"><span>归属</span><select v-model="row.owner_type"><option value="factory">本公司</option><option value="customer">客户</option></select></label>
                    <label v-if="row.owner_type === 'customer'" class="pc-row-field pc-row-wide"><span>归属客户 *</span><select v-model.number="row.owner_customer_id"><option :value="0">选择客户</option><option v-for="option in customerOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                  </template>
                  <button class="pc-run-icon-button" type="button" aria-label="删除物料行" @click="removeMaterialRow(node.id, row.row_id)"><IconTrash :size="17" /></button>
                </div>
                <button class="pc-run-add" type="button" @click="addRepeaterRow(node.id, 'rows')"><IconPlus :size="16" /> 添加配方物料</button>
              </template>

              <template v-else-if="node.data.module.kind === 'material' && node.data.config.data_role === 'output'">
                <label class="pc-run-field-inline"><span>产出物料处理</span><select v-model="valuesFor(node.id).action" :disabled="isNodeFieldFixed(node, 'action')"><option value="create">自动新建物料</option><option value="reuse">使用时选择已有物料</option></select></label>
                <template v-if="valuesFor(node.id).action === 'reuse'">
                  <label class="pc-run-field-inline"><span>搜索已有物料 *</span><input :value="materialSearchText[materialSearchKey(node.id, 'output')] || selectedMaterialLabel(valuesFor(node.id))" placeholder="输入物料名称或编码" @input="searchMaterialForOutput(node, $event.target.value)" /></label>
                  <div v-if="materialSearchResults[materialSearchKey(node.id, 'output')]?.length" class="pc-material-search-results">
                    <button v-for="option in materialSearchResults[materialSearchKey(node.id, 'output')]" :key="option.id" type="button" @click="selectMaterialOutput(node, option)"><strong>{{ option.name }}</strong><span>{{ option.code || '无编码' }} · {{ option.unit }} · {{ option.owner_label }}</span></button>
                  </div>
                </template>
                <label v-else class="pc-run-field-inline"><span>产出名称 * <button v-if="workflowVersion >= 3 && valuesFor(node.id).name_mode === 'manual'" class="pc-reset-name" type="button" @click.prevent="restoreVariableName(node)">恢复变量默认值</button></span><input v-model.trim="valuesFor(node.id).name" :placeholder="generatedOutputName(node) || '输入半成品名称'" :disabled="isNodeFieldFixed(node, 'name')" @input="markNameManual(node)" /></label>
                <div class="pc-run-field-inline"><span>库存单位</span><strong>{{ outputMaterialUnit(node) || '请在 BOM 默认配置中选择产出单位' }}</strong></div>
                <div class="pc-run-field-pair">
                  <label v-if="workflowVersion < 3"><span>物料类别</span><select v-model="valuesFor(node.id).kind" :disabled="isNodeFieldFixed(node, 'kind')"><option value="other">通用物料（含半成品）</option><option value="pack">包装物料</option><option value="bean">原料</option></select></label>
                  <label><span>取得方式</span><select v-model="valuesFor(node.id).supply_mode" :disabled="isNodeFieldFixed(node, 'supply_mode')"><option value="manufacture">自制</option><option value="purchase">外购</option></select></label>
                </div>
                <small class="pc-run-help">该物料由上游 BOM 生成；物料档案保持一个库存规格。</small>
              </template>

              <template v-else-if="node.data.module.kind === 'product' && node.data.config.data_role === 'output'">
                <label class="pc-run-field-inline"><span>产出商品处理</span><select v-model="valuesFor(node.id).action" :disabled="isNodeFieldFixed(node, 'action')"><option value="create">自动新建商品</option><option value="reuse">使用时选择已有商品</option></select></label>
                <label v-if="valuesFor(node.id).action === 'reuse'" class="pc-run-field-inline"><span>已有商品 *</span><select v-model.number="valuesFor(node.id).product_id" @change="setProductReferenceOwner(node.id)"><option :value="0">选择商品</option><option v-for="option in productOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                <label v-else class="pc-run-field-inline"><span>商品名称 * <button v-if="workflowVersion >= 3 && valuesFor(node.id).name_mode === 'manual'" class="pc-reset-name" type="button" @click.prevent="restoreVariableName(node)">恢复变量默认值</button></span><input v-model.trim="valuesFor(node.id).name" :placeholder="generatedOutputName(node) || '输入成品名称'" :disabled="isNodeFieldFixed(node, 'name')" @input="markNameManual(node)" /></label>
                <div v-if="valuesFor(node.id).action !== 'reuse'" class="pc-run-field-pair">
                  <label v-if="workflowVersion < 3"><span>商品类型</span><select v-model="valuesFor(node.id).product_kind" :disabled="isNodeFieldFixed(node, 'product_kind')"><option value="generic">通用商品／装配件</option><option value="roasted">熟豆</option><option value="green_bean">生豆</option><option value="drip_bag">挂耳</option><option value="instant_coffee">速溶咖啡</option></select></label>
                  <label><span>归属</span><select v-model="valuesFor(node.id).owner" :disabled="isNodeFieldFixed(node, 'owner')"><option value="factory">本公司</option><option value="customer">客户</option></select></label>
                  <label v-if="valuesFor(node.id).owner === 'customer'"><span>归属客户 *</span><select v-model.number="valuesFor(node.id).customer_id" :disabled="isNodeFieldFixed(node, 'customer_id')"><option :value="0">选择客户</option><option v-for="option in customerOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                </div>
                <small class="pc-run-help">商品规格和包装由上游 BOM 引用的规格模板生成。</small>
              </template>

              <template v-else-if="node.data.module.kind === 'product'">
                <label class="pc-run-field-inline"><span>选择已有商品 *</span><select v-model.number="valuesFor(node.id).product_id" @change="loadProductSpecs(node.id)"><option :value="0">搜索或选择商品</option><option v-for="option in productOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                <label class="pc-run-field-inline"><span>商品规格 *</span><select v-model.number="valuesFor(node.id).bom_spec_id" @change="syncProductInputComponents(node.id)"><option :value="0">选择商品的已发布规格</option><option v-for="option in productSpecOptions[node.id] || []" :key="option.bom_spec_id" :value="option.bom_spec_id">{{ option.label }}</option></select></label>
                <small class="pc-run-help">商品作为 BOM 组件时，必须引用具体的已发布规格。</small>
              </template>

              <template v-else-if="node.data.module.kind === 'process'">
                <label class="pc-run-field-inline"><span>采用工艺路线</span><select v-model.number="valuesFor(node.id).route_id" :disabled="isNodeFieldFixed(node, 'route_id')"><option :value="0">选择有效工艺路线</option><option v-for="route in routeOptions" :key="route.id" :value="route.id">{{ route.label }}</option></select></label>
              </template>

              <template v-else-if="isV4ProductBOM(node)">
                <label class="pc-run-field-inline"><span>规格主体候选 *</span>
                  <select :value="mainInputSelection(node.id)" @change="selectMainInputCandidate(node, $event.target.value)">
                    <option value="">选择一个已连接的物料或商品规格</option>
                    <option v-for="candidate in mainInputCandidates(node)" :key="candidate.value" :value="candidate.value">{{ candidate.label }}</option>
                  </select>
                </label>
                <small v-if="issueForField(node.id, 'main_input_source_row_id')" class="pc-run-field-error">{{ issueForField(node.id, 'main_input_source_row_id') }}</small>
                <small v-if="issueForField(node.id, 'main_input_source_node_id')" class="pc-run-field-error">{{ issueForField(node.id, 'main_input_source_node_id') }}</small>
                <div v-if="workflowVersion >= 5" class="pc-bom-run-defaults pc-bom-route-override">
                  <label><span>工艺路线 · 可单独改选</span><select v-model.number="valuesFor(node.id).route_override_id"><option :value="0">跟随默认 · {{ defaultBOMRouteLabel(node) }}</option><option v-for="route in routeOptions" :key="route.id" :value="route.id">本次采用 · {{ route.label }}</option></select></label>
                </div>
                <section v-if="specificationTemplateForBOM(node.id)" class="pc-template-run-preview">
                  <header><strong>{{ specificationTemplateForBOM(node.id).name }}</strong><span>已发布 {{ specificationTemplateForBOM(node.id).selected_version?.version_no || '' }} · {{ specificationTemplateForBOM(node.id).variants?.length || 0 }} 个规格</span></header>
                  <article v-for="variant in specificationTemplateForBOM(node.id).variants || []" :key="variant.spec_key">
                    <div class="pc-template-run-variant-heading"><strong>{{ variant.name }}</strong><span>{{ variant.inventory_unit }}<b v-if="variant.is_default">默认规格</b></span></div>
                    <div class="pc-template-run-meta">主体用量 {{ mainTemplateInput(variant)?.qty_per_unit || 0 }} {{ mainTemplateInput(variant)?.consume_unit || '' }} · 工艺 {{ routeLabel(effectiveTemplateVariantRouteID(node, variant)) }} · {{ processRouteSourceLabel(node) }} · 损耗 {{ (Number(variant.material_loss_rate || 0) * 100).toFixed(2) }}%</div>
                    <div v-for="item in (variant.items || []).filter((row) => !row.is_main_input)" :key="`${variant.spec_key}-${item.sort_order}-${item.component_bom_spec_id || item.material_id}`" class="pc-template-run-item">
                      <span>{{ item.component_name || item.component_spec_name || '规格模板包材' }}<small v-if="item.component_spec_name && item.component_name"> · {{ item.component_spec_name }}</small></span>
                      <strong>{{ item.qty_per_unit || item.ratio_pct }} {{ item.consume_unit }}<small v-if="item.component_spec_unit"> / {{ item.component_spec_unit }}</small></strong>
                    </div>
                  </article>
                </section>
                <div v-else class="pc-template-run-empty">{{ specificationTemplateLoadError || '正在读取规格模板详情…' }}</div>
                <small class="pc-run-help">整组规格、包材、工艺和损耗均来自所选的已发布规格模板；未选作主体的候选来源不会进入本 BOM。</small>
              </template>

              <template v-else-if="node.data.module.kind === 'bom'">
                <div class="pc-bom-run-defaults">
                  <label><span>产出基准数量</span><input v-model.number="valuesFor(node.id).output_qty" type="number" min="0.001" step="0.001" :disabled="isNodeFieldFixed(node, 'output_qty')" /></label>
                  <label><span>产出单位</span><select :value="valuesFor(node.id).output_unit" :disabled="isNodeFieldFixed(node, 'output_unit')" @change="setBOMOutputUnit(node.id, $event.target.value)"><option value="">采用物料／默认规格单位</option><option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option></select></label>
                  <label><span>工艺路线</span><select v-if="workflowVersion >= 5" v-model.number="valuesFor(node.id).route_override_id"><option :value="0">跟随默认 · {{ defaultBOMRouteLabel(node) }}</option><option v-for="route in routeOptions" :key="route.id" :value="route.id">本次采用 · {{ route.label }}</option></select><select v-else v-model.number="valuesFor(node.id).route_id"><option :value="0">采用模板连线工艺</option><option v-for="route in routeOptions" :key="route.id" :value="route.id">{{ route.label }}</option></select></label>
                  <label><span>比例配方损耗 %</span><input :value="Number(valuesFor(node.id).material_loss_rate || 0) * 100" type="number" min="0" max="99.99" step="0.01" :disabled="isNodeFieldFixed(node, 'material_loss_rate')" @input="setBOMLossPercent(node.id, $event.target.value)" /></label>
                </div>
                <div v-if="node.data.config.output_type === 'product'" class="pc-bom-variant-list">
                  <div class="pc-repeater-caption">商品规格 <small>各规格分别维护用量，可共用同一配方行</small></div>
                  <div v-for="variant in ensureVariants(node)" :key="variant.row_id" class="pc-bom-variant-row">
                    <input v-model.trim="variant.name" placeholder="规格名称，例如：200g" :disabled="isNodeFieldFixed(node, 'variants')" />
                    <select v-model="variant.unit" :disabled="isNodeFieldFixed(node, 'variants')"><option value="">选择单位</option><option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option></select>
                    <label class="pc-default-variant"><input v-model="variant.is_default" type="radio" :name="`default-${node.id}`" :value="true" :disabled="isNodeFieldFixed(node, 'variants')" @change="setDefaultVariant(node.id, variant.row_id)" /> 默认</label>
                    <button class="pc-run-icon-button" type="button" aria-label="删除规格" :disabled="isNodeFieldFixed(node, 'variants')" @click="removeRow(node.id, 'variants', variant.row_id)"><IconTrash :size="16" /></button>
                  </div>
                  <button class="pc-run-add" type="button" :disabled="isNodeFieldFixed(node, 'variants')" @click="addRepeaterRow(node.id, 'variants')"><IconPlus :size="15" /> 添加商品规格</button>
                </div>
                <div class="pc-bom-components">
                  <div class="pc-repeater-caption">{{ bomIdentity(node) }} · 配方 <small>只需选择物料及规格，再填写用量和消耗单位</small></div>
                  <div v-for="component in ensureComponents(node)" :key="component.row_id" class="pc-component-row">
                    <div class="pc-component-source"><IconLink :size="15" /><span>{{ sourceNodeName(component.source_node_id) }} · {{ sourceRowLabel(component.source_node_id, component.source_row_id, node.id) }}</span></div>
                    <label><span>配方对象 *</span><select v-model="component.source_row_id"><option value="">选择连接对象</option><option v-for="option in sourceRows(component.source_node_id, node.id)" :key="option.row_id" :value="option.row_id">{{ option.label }}</option></select></label>
                    <label><span>用量／比例 *</span><input v-model.number="component.quantity" type="number" min="0.001" step="0.001" placeholder="填写本规格用量" :disabled="isNodeFieldFixed(node, 'components')" /></label>
                    <label><span>消耗单位 *</span><select v-model="component.unit" :disabled="isNodeFieldFixed(node, 'components')"><option value="">选择单位</option><option value="ratio_pct">比例 %</option><option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.name || unit.code }}</option></select></label>
                    <label v-if="node.data.config.output_type === 'product' && ensureVariants(node).length > 1"><span>适用产出规格</span><select v-model="component.variant_row_id" :disabled="isNodeFieldFixed(node, 'components')"><option value="">全部规格</option><option v-for="variant in ensureVariants(node)" :key="variant.row_id" :value="variant.row_id">{{ variant.name }}</option></select></label>
                    <div class="pc-component-actions">
                      <button class="pc-run-icon-button" type="button" aria-label="复制配方行" :disabled="isNodeFieldFixed(node, 'components')" @click="duplicateComponentRow(node.id, component)"><IconCopy :size="16" /></button>
                      <button class="pc-run-icon-button" type="button" aria-label="删除配方行" :disabled="isNodeFieldFixed(node, 'components')" @click="removeRow(node.id, 'components', component.row_id)"><IconTrash :size="16" /></button>
                    </div>
                  </div>
                  <button class="pc-run-add" type="button" :disabled="isNodeFieldFixed(node, 'components')" @click="addComponentFromEdge(node)"><IconPlus :size="15" /> 添加已连接物料行</button>
                </div>
              </template>

              <template v-else-if="node.data.module.kind === 'purchase'">
                <div class="pc-run-field-pair">
                  <label><span>采购物料 *</span><select v-model="valuesFor(node.id).material_source_row_id"><option value="">选择连接的物料</option><option v-for="option in purchaseMaterialRows(node.id)" :key="option.row_id" :value="option.row_id">{{ option.label }}</option></select></label>
                  <label><span>供应商 *</span><select v-model.number="valuesFor(node.id).supplier_id"><option :value="0">选择供应商</option><option v-for="option in supplierOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                  <label><span>预计数量 *</span><input v-model.number="valuesFor(node.id).quantity" type="number" min="0.001" step="0.001" /></label>
                  <label><span>目标仓库 *</span><select v-model="valuesFor(node.id).warehouse"><option value="">选择仓库</option><option v-for="option in warehouseOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                  <label><span>采购单价 *</span><input v-model.number="valuesFor(node.id).unit_price" type="number" min="0" step="0.01" /></label>
                </div>
              </template>
            </div>
            <template v-else>
            <div v-for="field in visibleFields(node)" :key="field.key" class="pc-run-field" :class="{ wide: field.type === 'record' || field.type === 'repeater' }">
              <label :for="`run-${node.id}-${field.key}`">{{ field.label }}<b v-if="field.required">*</b></label>
              <template v-if="field.type === 'repeater'">
                <div v-if="node.data.module.kind === 'material'" class="pc-run-repeater">
                  <div v-for="row in ensureMaterialRows(node.id)" :key="row.row_id" class="pc-material-row">
                    <label class="pc-row-field"><span>处理方式</span><select v-model="row.action"><option value="create">新建物料</option><option value="reuse">引用已有</option></select></label>
                    <label v-if="row.action === 'reuse'" class="pc-row-field pc-row-wide"><span>选择已有物料 *</span><select v-model.number="row.material_id" @change="setMaterialReferenceOwner(row)"><option :value="0">选择物料名称</option><option v-for="option in materialOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                    <template v-else>
                      <label class="pc-row-field"><span>物料名称 *</span><input v-model.trim="row.name" placeholder="例如：云南水洗豆" /></label>
                      <label v-if="workflowVersion < 3" class="pc-row-field"><span>物料类别</span><select v-model="row.kind"><option value="bean">原料</option><option value="pack">包材</option><option value="other">其他</option></select></label>
                      <label class="pc-row-field"><span>取得方式</span><select v-model="row.supply_mode"><option value="purchase">外购</option><option value="manufacture">自制</option></select></label>
                      <label class="pc-row-field"><span>库存单位 *</span><input v-model.trim="row.unit" placeholder="kg / 个 / 袋" /></label>
                      <label class="pc-row-field"><span>归属</span><select v-model="row.owner_type"><option value="factory">本公司</option><option value="customer">客户</option></select></label>
                      <label v-if="row.owner_type === 'customer'" class="pc-row-field pc-row-wide"><span>归属客户 *</span><select v-model.number="row.owner_customer_id"><option :value="0">选择客户</option><option v-for="option in customerOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select></label>
                    </template>
                    <button class="pc-run-icon-button" type="button" aria-label="删除物料行" @click="removeRow(node.id, 'rows', row.row_id)"><IconTrash :size="17" /></button>
                  </div>
                  <button class="pc-run-add" type="button" @click="addRepeaterRow(node.id, 'rows')"><IconPlus :size="16" /> 添加物料</button>
                </div>
                <div v-else-if="node.data.module.kind === 'bom' && field.key === 'variants'" class="pc-run-repeater">
                  <div class="pc-bom-variant-list">
                    <div class="pc-repeater-caption">商品规格 <small>同一行标识用于匹配该规格的包材用量</small></div>
                    <div v-for="variant in ensureVariants(node)" :key="variant.row_id" class="pc-bom-variant-row">
                      <input v-model.trim="variant.name" placeholder="规格名称，例如：250g" />
                      <input v-model.trim="variant.unit" placeholder="库存单位" />
                      <label class="pc-default-variant"><input v-model="variant.is_default" type="radio" :name="`default-${node.id}`" :value="true" @change="setDefaultVariant(node.id, variant.row_id)" /> 默认</label>
                      <button class="pc-run-icon-button" type="button" aria-label="删除规格" @click="removeRow(node.id, 'variants', variant.row_id)"><IconTrash :size="16" /></button>
                    </div>
                    <button class="pc-run-add" type="button" @click="addRepeaterRow(node.id, 'variants')"><IconPlus :size="16" /> 添加规格</button>
                  </div>
                </div>
                <div v-else-if="node.data.module.kind === 'bom' && field.key === 'components'" class="pc-run-repeater">
                  <div class="pc-bom-components">
                    <div class="pc-repeater-caption">BOM 组件 <small>连接线决定组件来自哪个步骤</small></div>
                    <div v-for="component in ensureComponents(node)" :key="component.row_id" class="pc-component-row">
                      <div class="pc-component-source"><IconLink :size="15" /><span>{{ sourceNodeName(component.source_node_id) || '选择连接的业务步骤' }}</span></div>
                      <label><span>引用对象 *</span><select v-model="component.source_row_id"><option value="">选择对象</option><option v-for="option in sourceRows(component.source_node_id, node.id)" :key="option.row_id" :value="option.row_id">{{ option.label }}</option></select></label>
                      <label><span>用量 *</span><input v-model.number="component.quantity" type="number" min="0" step="0.001" placeholder="0" /></label>
                      <label><span>单位 *</span><input v-model.trim="component.unit" placeholder="g / 个" /></label>
                      <label><span>损耗率 %</span><input v-model.number="component.loss_rate" type="number" min="0" step="0.01" placeholder="0" /></label>
                      <label v-if="ensureVariants(node).length > 1"><span>适用规格</span><select v-model="component.variant_row_id"><option value="">全部规格</option><option v-for="variant in ensureVariants(node)" :key="variant.row_id" :value="variant.row_id">{{ variant.name }}</option></select></label>
                      <button class="pc-run-icon-button" type="button" aria-label="删除 BOM 组件" @click="removeRow(node.id, 'components', component.row_id)"><IconTrash :size="16" /></button>
                    </div>
                    <button class="pc-run-add" type="button" @click="addComponentFromEdge(node)"><IconPlus :size="15" /> 补齐已连接来源</button>
                    <small class="pc-run-help">每条连接可展开为多条物料或规格行；未显示的新行可点此补齐。</small>
                  </div>
                </div>
                <div v-else-if="node.data.module.kind === 'pricing' && field.key === 'prices'" class="pc-run-repeater">
                  <div class="pc-price-row" v-for="price in ensurePricingRows(node)" :key="price.row_id">
                    <div class="pc-price-spec"><IconTags :size="16" /><div><strong>{{ pricingSpecLabel(node.id, price.spec_row_id) }}</strong><small>按当前已发布 BOM 规格计价</small></div></div>
                    <label><span>定价方式</span><select v-model="price.pricing_mode"><option value="fixed">固定售价</option><option value="rule">成本规则试算</option></select></label>
                    <label v-if="price.pricing_mode === 'fixed'"><span>每{{ pricingSpecUnit(node.id, price.spec_row_id) }}售价</span><input v-model.number="price.price" type="number" min="0.01" step="0.01" placeholder="0.00" /></label>
                    <label v-else><span>定价规则</span><select v-model.number="price.pricing_rule_id"><option :value="0">选择有效规则</option><option v-for="rule in pricingRuleOptions" :key="rule.id" :value="rule.id">{{ rule.name }} · {{ rule.formula_version || '当前版本' }}</option></select></label>
                  </div>
                  <small class="pc-run-help">规则试算使用已发布 BOM 与当前成本；若连接了收货步骤，需先完成收货后再试算。</small>
                </div>
              </template>
              <template v-else-if="field.type === 'choice'">
                <select :id="`run-${node.id}-${field.key}`" v-model="valuesFor(node.id)[field.key]" @change="handleRunFieldChange(node, field.key)">
                  <option v-for="option in choicesFor(node, field.key)" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </template>
              <template v-else-if="field.type === 'owner'">
                <select :id="`run-${node.id}-${field.key}`" v-model="valuesFor(node.id)[field.key]"><option value="factory">本公司</option><option value="customer">客户</option></select>
              </template>
              <template v-else-if="node.data.module.kind === 'product' && field.key === 'customer_id'">
                <select :id="`run-${node.id}-${field.key}`" v-model.number="valuesFor(node.id)[field.key]"><option :value="0">选择归属客户</option><option v-for="option in customerOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select>
              </template>
              <template v-else-if="node.data.module.kind === 'purchase' && field.key === 'material_source_row_id'">
                <select :id="`run-${node.id}-${field.key}`" v-model="valuesFor(node.id).material_source_row_id"><option value="">选择已连接物料</option><option v-for="option in purchaseMaterialRows(node.id)" :key="option.row_id" :value="option.row_id">{{ option.label }}</option></select>
              </template>
              <template v-else-if="field.type === 'reference' && field.key === 'warehouse'">
                <select :id="`run-${node.id}-${field.key}`" v-model="valuesFor(node.id)[field.key]">
                  <option value="">选择目标仓库</option>
                  <option v-for="option in referenceOptions(field.key)" :key="option.id" :value="option.id">{{ option.label }}</option>
                </select>
              </template>
              <template v-else-if="node.data.module.kind === 'bom' && field.key === 'output_source_row_id'">
                <select :id="`run-${node.id}-${field.key}`" v-model="valuesFor(node.id).output_source_row_id">
                  <option value="">选择连接步骤中的产出对象</option>
                  <option v-for="option in outputSourceRows(node.id)" :key="option.row_id" :value="option.row_id">{{ option.label }}</option>
                </select>
              </template>
              <template v-else-if="field.type === 'reference'">
                <select :id="`run-${node.id}-${field.key}`" v-model.number="valuesFor(node.id)[field.key]" @change="handleRunFieldChange(node, field.key)">
                  <option :value="0">{{ referencePlaceholder(field.key) }}</option>
                  <option v-for="option in referenceOptions(field.key)" :key="option.id" :value="option.id">{{ option.label }}</option>
                </select>
              </template>
              <template v-else-if="field.type === 'quantity' || field.type === 'money'">
                <input :id="`run-${node.id}-${field.key}`" v-model.number="valuesFor(node.id)[field.key]" type="number" min="0" step="0.001" :placeholder="field.type === 'money' ? '0.00' : '0'" />
              </template>
              <template v-else-if="field.type === 'boolean'">
                <label class="pc-run-check"><input :id="`run-${node.id}-${field.key}`" v-model="valuesFor(node.id)[field.key]" type="checkbox" /> {{ field.label }}</label>
              </template>
              <template v-else-if="field.type === 'record'">
                <textarea :id="`run-${node.id}-${field.key}`" v-model="valuesFor(node.id)[field.key]" rows="3" placeholder="按字段名称填写，每行一项"></textarea>
              </template>
              <template v-else>
                <input :id="`run-${node.id}-${field.key}`" v-model.trim="valuesFor(node.id)[field.key]" :placeholder="field.description || `填写${field.label}`" />
              </template>
              <small v-if="fieldHelp(node, field.key)" class="pc-run-help">{{ fieldHelp(node, field.key) }}</small>
              <small v-if="issueForField(node.id, field.key)" class="pc-run-error">{{ issueForField(node.id, field.key) }}</small>
              <div v-if="node.data.module.kind === 'product' && field.key === 'action' && valuesFor(node.id).action === 'reuse'" class="pc-run-reference-inline">
                <label>选择已有商品 *</label>
                <select v-model.number="valuesFor(node.id).product_id" @change="setProductReferenceOwner(node.id)"><option :value="0">选择商品名称</option><option v-for="option in productOptions" :key="option.id" :value="option.id">{{ option.label }}</option></select>
              </div>
            </div>
            </template>
          </div>
          <div v-if="dependencySummary(node).length" class="pc-run-dependencies"><IconLink :size="14" /> 数据来源：{{ dependencySummary(node).join('、') }}</div>
        </section>

        <section v-if="run.preview" class="pc-run-preview-result" :class="{ invalid: !run.preview.valid }">
          <header><component :is="run.preview.valid ? IconCircleCheck : IconAlertTriangle" :size="19" /><div><strong>{{ run.preview.valid ? '业务预览通过' : '还有信息需要补齐' }}</strong><span>预览只做校验，不会创建正式商品、物料、BOM 或单据。</span></div></header>
          <div class="pc-run-preview-steps"><div v-for="step in run.preview.steps" :key="step.node_id"><span>{{ step.name }}</span><strong>{{ step.action }}</strong><small v-if="step.details?.specification_template">规格模板：{{ step.details.specification_template.name }} · {{ step.details.specification_template.version_no }}（{{ step.details.specification_template.variant_count }} 个规格）<template v-if="step.details.main_input?.source_node_name"> · 主体来源：{{ step.details.main_input.source_node_name }}</template></small></div></div>
          <div v-for="issue in run.preview.issues" :key="`${issue.node_id}-${issue.field}-${issue.code}`" class="pc-run-preview-issue">{{ issue.message }}</div>
        </section>
        </fieldset>
      </main>

      <aside class="pc-run-progress">
        <div class="pc-progress-heading"><h2>流程进度</h2><span>模板 V{{ run.template_version }}</span></div>
        <div class="pc-run-flow-wrap">
          <VueFlow
            :nodes="progressNodes"
            :edges="progressEdges"
            :node-types="nodeTypes"
            :nodes-draggable="false"
            :nodes-connectable="false"
            :elements-selectable="false"
            fit-view-on-init>
            <Background pattern-color="#e1e8ef" :gap="18" :size="1" />
          </VueFlow>
        </div>
        <div class="pc-progress-legend"><span><i class="complete"></i> 已完成</span><span><i class="current"></i> 本次填写</span><span><i class="waiting"></i> 等待前置</span></div>
        <div class="pc-run-safety"><IconInfoCircle :size="16" /><p>提交前会重新检查关联对象与权限。采购收货和价格发布会在配置完成后分别确认。</p></div>
      </aside>
    </div>
  </section>
</template>

<script setup>
import { computed, markRaw, onMounted, ref, watch } from 'vue'
import { Background } from '@vue-flow/background'
import { VueFlow } from '@vue-flow/core'
import {
  IconAlertTriangle,
  IconArrowLeft,
  IconCircleCheck,
  IconCopy,
  IconDeviceFloppy,
  IconEye,
  IconInfoCircle,
  IconLink,
  IconPackage,
  IconPlus,
  IconRoute,
  IconSend,
  IconShoppingCart,
  IconSitemap,
  IconTags,
  IconTrash,
} from '@tabler/icons-vue'
import { apiGet } from '../api/client.js'
import { getProductionBomSpecTemplateVersion, listProductionBomSpecTemplates } from '../api/product-creator.js'
import { canonicalInputPortID, cloneValue, makeNodeId, toCanvasGraph } from '../lib/product-creator-graph.js'
import { appendDuplicatedBOMComponentRow } from '../lib/product-creator-component-rows.js'
import { renderNameParts } from '../lib/product-creator-variables.js'
import ProductCreatorNode from './ProductCreatorNode.vue'

const props = defineProps({
  run: { type: Object, required: true },
  modules: { type: Array, default: () => [] },
  busy: { type: Boolean, default: false },
  error: { type: String, default: '' },
})
defineEmits(['back', 'save', 'preview', 'commit', 'execute'])

const nodeTypes = { business: markRaw(ProductCreatorNode) }
const inputValues = ref({})
const variableValues = ref({})
const materialOptions = ref([])
const productOptions = ref([])
const customerOptions = ref([])
const supplierOptions = ref([])
const routeOptions = ref([])
const unitOptions = ref([])
const warehouseOptions = ref([])
const priceListOptions = ref([])
const pricingRuleOptions = ref([])
const optionError = ref('')
const originalBomVariants = ref({})
let activeRunID = 0
let priceTargetLoadRevision = 0
const bomOptions = ref([])
const templateName = ref('商品创建模板')
const materialSearchText = ref({})
const materialSearchResults = ref({})
const materialSearchOffsets = ref({})
const materialSearchHasMore = ref({})
const materialSearchLoading = ref({})
const productSpecOptions = ref({})
const specificationTemplateDetails = ref({})
const specificationTemplateVersionCache = ref({})
const specificationTemplateLoadError = ref('')
let specificationTemplateCatalog = null
let specificationTemplateLoadRevision = 0
const materialSearchTimers = new Map()
const materialSearchRevisions = new Map()
const workflowVersion = computed(() => Number(props.run.workflow?.version || 1))
const usedWorkflowVariables = computed(() => {
  const used = new Set()
  for (const node of props.run.workflow?.nodes || []) {
    for (const part of node.config?.name_parts || []) if (part.type === 'variable' && part.variable_id) used.add(part.variable_id)
  }
  return (props.run.workflow?.variables || []).filter((variable) => used.has(variable.id))
})

const graph = computed(() => toCanvasGraph(props.run.workflow || { nodes: [], edges: [] }, props.modules))
const progressNodes = computed(() => graph.value.nodes.map((node) => ({
  ...node,
  position: { ...node.position },
  data: { ...node.data, countLabel: statusFor(node.id) === 'complete' ? '已完成' : '' },
})))
const progressEdges = computed(() => graph.value.edges.map((edge) => ({ ...edge })))
const orderedNodes = computed(() => {
  const incoming = new Map(graph.value.nodes.map((node) => [node.id, 0]))
  const outgoing = new Map(graph.value.nodes.map((node) => [node.id, []]))
  for (const edge of graph.value.edges) {
    if (!incoming.has(edge.source) || !incoming.has(edge.target)) continue
    outgoing.get(edge.source).push(edge.target)
    incoming.set(edge.target, incoming.get(edge.target) + 1)
  }
  const ready = [...incoming.entries()].filter(([, n]) => n === 0).map(([id]) => id).sort()
  const order = []
  while (ready.length) {
    const id = ready.shift()
    order.push(id)
    for (const target of outgoing.get(id) || []) {
      incoming.set(target, incoming.get(target) - 1)
      if (!incoming.get(target)) ready.push(target)
    }
    ready.sort()
  }
  const byID = new Map(graph.value.nodes.map((node) => [node.id, node]))
  return (order.length === byID.size ? order : [...byID.keys()]).map((id) => byID.get(id)).filter(Boolean)
})
const resultRows = computed(() => {
  const outputs = props.run.business_results?.objects || {}
  const nodeNames = new Map((props.run.workflow?.nodes || []).map((node) => [node.id, node.name || '业务步骤']))
  const labels = { product: '商品', material: '物料', bom: 'BOM', spec: '规格', route: '工艺路线' }
  const rows = []
  for (const [nodeID, entries] of Object.entries(outputs)) {
    for (const entry of Array.isArray(entries) ? entries : []) {
      if (!entry?.type || entry.type === 'route') continue
      rows.push({ nodeID, stepName: nodeNames.get(nodeID), typeName: labels[entry.type] || '业务档案', name: entry.name || labels[entry.type] || '已创建', code: entry.code || '' })
    }
  }
  return rows
})
const purchaseNodes = computed(() => orderedNodes.value.filter((node) => node.data.module.kind === 'purchase'))
const pricingNodes = computed(() => orderedNodes.value.filter((node) => node.data.module.kind === 'pricing'))
const bomResultNodes = computed(() => orderedNodes.value.filter((node) => node.data.module.kind === 'bom'))
const followupInputValues = ref({})

watch(() => pricingNodes.value.map((node) => pricingOwnerKey(node.id)).join('|'), () => loadPriceListOptions(), { immediate: true })

watch(() => props.run, (run) => {
  if (!run) return
  if (activeRunID !== Number(run.id)) {
    activeRunID = Number(run.id)
    originalBomVariants.value = {}
    specificationTemplateDetails.value = {}
    optionError.value = ''
  }
  const saved = cloneValue(run.inputs || {})
  inputValues.value = saved
  variableValues.value = { ...Object.fromEntries((run.workflow?.variables || []).map((variable) => [variable.id, variable.default_value || ''])), ...(run.variable_values || {}) }
  for (const node of run.workflow?.nodes || []) {
    const initial = makeInitialValues(node)
    inputValues.value[node.id] = { ...initial, ...(inputValues.value[node.id] || {}) }
    if (workflowVersion.value >= 2) applyNodeDefaults(node, inputValues.value[node.id])
  }
  applyVariableNames()
  for (const node of run.workflow?.nodes || []) {
    const values = inputValues.value[node.id]
    if (node.kind === 'bom') {
      if (workflowVersion.value < 2 && !Object.prototype.hasOwnProperty.call(values, 'output_source_row_id')) values.output_source_row_id = initialOutputSourceRow(node.id)
      if (!(workflowVersion.value >= 4 && node.config?.output_type === 'product') && (!Array.isArray(values.components) || (workflowVersion.value >= 2 && values.components.length === 0))) values.components = initialComponentRows(node.id)
    }
    if (node.kind === 'purchase' && !Object.prototype.hasOwnProperty.call(values, 'material_source_row_id')) values.material_source_row_id = purchaseMaterialRows(node.id)[0]?.row_id || ''
    if (node.kind === 'pricing' && !Array.isArray(values.prices)) values.prices = initialPricingRows(node.id)
  }
  void loadSpecificationTemplatesForRun(run)
  for (const node of run.workflow?.nodes || []) {
    if (node.kind !== 'purchase' || followupInputValues.value[node.id]) continue
    const values = inputValues.value[node.id] || {}
    followupInputValues.value[node.id] = { quantity: Number(values.quantity || 0), unit_price: Number(values.unit_price || 0) }
  }
}, { immediate: true, deep: true })

watch(variableValues, () => applyVariableNames(), { deep: true })

onMounted(async () => {
  await loadOptions()
  for (const node of graph.value.nodes) {
    if (workflowVersion.value >= 2 && node.data.module.kind === 'product' && node.data.config.data_role !== 'output' && Number(valuesFor(node.id).product_id || 0) > 0) {
      await loadProductSpecs(node.id)
    }
    if (node.data.module.kind === 'bom' && valuesFor(node.id).action === 'reuse' && Number(valuesFor(node.id).bom_id) > 0) {
      await loadExistingBomVariants(node.id)
    }
  }
  const templateID = Number(props.run.template_id || 0)
  if (templateID) {
    try {
      const response = await apiGet(`/api/product-creator/templates/${templateID}/versions`)
      const version = response.rows?.find((row) => row.version === props.run.template_version)
      if (version?.name) templateName.value = version.name
    } catch { /* The pinned run snapshot remains usable when version labels cannot be loaded. */ }
  }
})

function makeInitialValues(node) {
  if (workflowVersion.value >= 2) {
    if (node.kind === 'material' && node.config?.data_role !== 'output') {
      return { rows: [{ row_id: makeNodeId(), name: '', action: 'reuse', kind: 'other', supply_mode: 'purchase', unit: '', owner_type: 'factory', owner_customer_id: 0 }] }
    }
    if (node.kind === 'material') {
      return { action: node.config?.object_action || 'create', material_id: 0, name: '', name_mode: 'automatic', unit: node.config?.unit || outputBOMUnitForDataNode(node.id), kind: 'other', supply_mode: node.config?.supply_mode || 'manufacture', owner_type: node.config?.owner || 'factory', owner_customer_id: 0 }
    }
    if (node.kind === 'product' && node.config?.data_role === 'output') {
      return { action: node.config?.object_action || 'create', product_id: 0, name: '', name_mode: 'automatic', product_kind: 'generic', owner: node.config?.owner || 'factory', customer_id: Number(node.config?.customer_id || 0), industry_fields: '' }
    }
    if (node.kind === 'product') return { product_id: 0, bom_spec_id: 0 }
    if (node.kind === 'bom') {
      if (workflowVersion.value >= 4 && node.config?.output_type === 'product') {
        return { main_input_source_node_id: '', main_input_source_row_id: '', ...(workflowVersion.value >= 5 ? { route_override_id: 0 } : {}) }
      }
      return {
        output_qty: node.config?.output_qty ?? 1,
        output_unit: node.config?.output_unit || '',
        route_id: Number(node.config?.route_id || 0),
        ...(workflowVersion.value >= 5 ? { route_override_id: 0 } : {}),
        material_loss_rate: Number(node.config?.material_loss_rate || 0),
        variants: (node.config?.variants || []).map((row) => ({ ...row, row_id: row.row_id || makeNodeId() })),
        components: [],
      }
    }
    if (node.kind === 'process') return { route_id: Number(node.config?.route_id || 0) }
    if (node.kind === 'purchase') return { material_source_row_id: '', supplier_id: 0, quantity: '', warehouse: '', unit_price: '' }
  }
  const values = {}
  const defaults = {
    product: { action: 'create', product_kind: 'generic', owner: 'factory', customer_id: 0, industry_fields: '' },
    material: { rows: [{ row_id: makeNodeId(), name: '', action: 'create', kind: 'other', supply_mode: 'purchase', unit: '', owner_type: 'factory', owner_customer_id: 0 }] },
    bom: {
      action: 'create',
      output_source_row_id: '',
      variants: (node.config?.variants || []).map((row) => ({ ...row, row_id: row.row_id || makeNodeId() })),
      components: (node.config?.components?.length ? node.config.components : null)?.map((row) => ({ ...row, row_id: row.row_id || makeNodeId() })),
    },
    process: { route_id: 0 },
    publish: { set_default: true },
    purchase: { material_source_row_id: '', supplier_id: 0, quantity: '', warehouse: '', unit_price: '' },
    pricing: { price_list_id: 0, prices: [] },
  }
  return { ...(defaults[node.kind] || {}) }
}

function applyNodeDefaults(node, values) {
  const defaults = { ...(node.config?.defaults || {}) }
  for (const key of ['action', 'object_action', 'product_kind', 'owner', 'customer_id', 'kind', 'supply_mode', 'unit', 'name', 'name_pattern', 'output_type', 'output_qty', 'output_unit', 'route_id', 'material_loss_rate', 'rows', 'variants', 'components']) {
    if (Object.prototype.hasOwnProperty.call(node.config || {}, key)) defaults[key] = node.config[key]
  }
  if (defaults.object_action && !defaults.action) defaults.action = defaults.object_action
  const fixed = new Set(workflowVersion.value >= 3 ? [] : (node.config?.fixed_fields || []))
  for (const [key, value] of Object.entries(defaults)) {
    const current = values[key]
    const blank = current === undefined || current === null || current === '' || current === 0 || (Array.isArray(current) && current.length === 0)
    if (fixed.has(key) || blank) values[key] = cloneValue(value)
  }
}

async function loadSpecificationTemplatesForRun(run) {
  if (Number(run?.workflow?.version || 1) < 4) return
  const revision = ++specificationTemplateLoadRevision
  specificationTemplateLoadError.value = ''
  try {
    if (!specificationTemplateCatalog) {
      const response = await listProductionBomSpecTemplates()
      specificationTemplateCatalog = Array.isArray(response) ? response : response.rows || []
    }
    const publishedVersions = new Map()
    for (const template of specificationTemplateCatalog) {
      if (template.active === false) continue
      for (const version of template.versions || []) if (version.status === 'published') publishedVersions.set(String(version.id), { template_id: template.id, ...version })
    }
    const details = {}
    for (const node of run.workflow?.nodes || []) {
      if (node.kind !== 'bom' || node.config?.output_type !== 'product') continue
      const versionID = Number(node.config?.spec_template_version_id || 0)
      if (versionID <= 0) continue
      let detail = specificationTemplateVersionCache.value[String(versionID)]
      if (!detail) {
        const version = publishedVersions.get(String(versionID))
        if (!version) throw new Error(`${node.name || '商品 BOM'}引用的规格模板版本不再可用，请升级模板`)
        detail = await getProductionBomSpecTemplateVersion(version.template_id, versionID)
        detail.selected_version = version
        specificationTemplateVersionCache.value = { ...specificationTemplateVersionCache.value, [String(versionID)]: detail }
      }
      details[node.id] = detail
    }
    if (revision === specificationTemplateLoadRevision && Number(run.id) === activeRunID) specificationTemplateDetails.value = details
  } catch (error) {
    if (revision === specificationTemplateLoadRevision) specificationTemplateLoadError.value = error.message || '规格模板详情读取失败'
  }
}

function isV4ProductBOM(node) {
  return workflowVersion.value >= 4 && node.data.module.kind === 'bom' && node.data.config.output_type === 'product'
}

function specificationTemplateForBOM(nodeID) {
  return specificationTemplateDetails.value[nodeID] || null
}

function mainInputCandidates(node) {
  const candidates = []
  const edges = graph.value.edges.filter((edge) => edge.target === node.id && canonicalInputPortID(edge.targetHandle) === 'components')
  for (const edge of edges) {
    for (const row of sourceRows(edge.source, node.id)) {
      candidates.push({
        source_node_id: edge.source,
        source_row_id: row.row_id,
        value: JSON.stringify([edge.source, row.row_id]),
        label: `${sourceNodeName(edge.source)} · ${row.label}${row.unit ? ` · ${row.unit}` : ''}`,
      })
    }
  }
  return candidates
}

function mainInputSelection(nodeID) {
  const values = valuesFor(nodeID)
  if (!values.main_input_source_node_id || !values.main_input_source_row_id) return ''
  return JSON.stringify([values.main_input_source_node_id, values.main_input_source_row_id])
}

function selectMainInputCandidate(node, value) {
  const values = valuesFor(node.id)
  if (!value) {
    values.main_input_source_node_id = ''
    values.main_input_source_row_id = ''
    return
  }
  try {
    const [sourceNodeID, sourceRowID] = JSON.parse(value)
    values.main_input_source_node_id = String(sourceNodeID || '')
    values.main_input_source_row_id = String(sourceRowID || '')
  } catch {
    values.main_input_source_node_id = ''
    values.main_input_source_row_id = ''
  }
}

function mainTemplateInput(variant) {
  return (variant?.items || []).find((item) => item.is_main_input) || null
}

function routeLabel(routeID) {
  return routeOptions.value.find((route) => Number(route.id) === Number(routeID || 0))?.label || (Number(routeID || 0) > 0 ? `工艺路线 ${routeID}` : '未设置')
}

function connectedBOMRouteID(node) {
  const edge = graph.value.edges.find((item) => item.target === node.id && item.targetHandle === 'route' && item.data?.kind === 'data')
  if (!edge) return 0
  const source = (props.run.workflow?.nodes || []).find((item) => item.id === edge.source)
  return Number(valuesFor(edge.source).route_id || source?.config?.route_id || 0)
}

function defaultBOMRouteLabel(node) {
  const connectedRoute = connectedBOMRouteID(node)
  if (connectedRoute > 0) return routeLabel(connectedRoute)
  if (node.data.config.output_type === 'product') {
    const variants = specificationTemplateForBOM(node.id)?.variants || []
    const ids = [...new Set(variants.map((variant) => Number(variant.process_route_id || 0)))]
    if (ids.length > 1) return '各规格沿用模板工艺'
    if (ids.length === 1) return routeLabel(ids[0])
    return '规格模板未设工艺'
  }
  return routeLabel(node.data.config.route_id || 0)
}

function effectiveTemplateVariantRouteID(node, variant) {
  const selected = Number(valuesFor(node.id).route_override_id || 0)
  if (workflowVersion.value >= 5 && selected > 0) return selected
  const connected = workflowVersion.value >= 5 ? connectedBOMRouteID(node) : 0
  return connected || Number(variant.process_route_id || 0)
}

function processRouteSourceLabel(node) {
  if (Number(valuesFor(node.id).route_override_id || 0) > 0) return '本次改选'
  if (connectedBOMRouteID(node) > 0) return '跟随连线工艺'
  return '沿用规格模板'
}

function valuesFor(nodeID) {
  if (!inputValues.value[nodeID]) inputValues.value[nodeID] = {}
  return inputValues.value[nodeID]
}

function visibleFields(node) {
  const fields = node.data.module.fields || []
  const values = valuesFor(node.id)
  if (node.data.module.kind === 'product' && values.action === 'reuse') return fields.filter((field) => field.key === 'action')
  if (node.data.module.kind === 'product' && values.owner !== 'customer') return fields.filter((field) => field.key !== 'customer_id')
  if (node.data.module.kind === 'bom' && values.action === 'reuse') return fields.filter((field) => field.key === 'action' || field.key === 'bom_id')
  return fields
}

function choicesFor(node, key) {
  if (node.data.module.kind === 'product' && key === 'action') return [{ value: 'create', label: '新建商品' }, { value: 'reuse', label: '引用已有商品' }]
  if (node.data.module.kind === 'product' && key === 'product_kind') return [
    { value: 'generic', label: '通用商品 / 装配件' },
    { value: 'roasted', label: '熟豆' },
    { value: 'green_bean', label: '生豆' },
    { value: 'drip_bag', label: '挂耳' },
    { value: 'instant_coffee', label: '速溶咖啡' },
  ]
  if (node.data.module.kind === 'bom' && key === 'action') return [{ value: 'create', label: '新建 BOM' }, { value: 'copy', label: '复制为新 BOM' }, { value: 'reuse', label: '引用已有 BOM' }]
  return []
}

function handleRunFieldChange(node, key) {
  if (node.data.module.kind !== 'bom') return
  const values = valuesFor(node.id)
  if (key === 'action') {
    if (values.action === 'reuse') {
      if (!Object.prototype.hasOwnProperty.call(originalBomVariants.value, node.id)) {
        originalBomVariants.value[node.id] = cloneValue(values.variants || [])
      }
      if (Number(values.bom_id) > 0) loadExistingBomVariants(node.id)
      return
    }
    if (Object.prototype.hasOwnProperty.call(originalBomVariants.value, node.id)) {
      values.variants = originalBomVariants.value[node.id]
      delete originalBomVariants.value[node.id]
      refreshComponentRowsForSource(node.id)
    }
  } else if (key === 'bom_id' && values.action === 'reuse' && Number(values.bom_id) > 0) {
    loadExistingBomVariants(node.id)
  }
}

async function loadExistingBomVariants(nodeID) {
  const values = valuesFor(nodeID)
  const bomID = Number(values.bom_id || 0)
  if (values.action !== 'reuse' || bomID <= 0) return
  optionError.value = ''
  try {
    const summary = await apiGet(`/api/production-boms/${bomID}`)
    const publishedVersion = (summary.versions || [])
      .filter((version) => version.status === 'published')
      .sort((a, b) => String(b.published_at || '').localeCompare(String(a.published_at || '')) || Number(b.id) - Number(a.id))[0]
    if (!publishedVersion) throw new Error('所选 BOM 没有有效的已发布版本')
    const detail = await apiGet(`/api/production-boms/${bomID}?version_id=${publishedVersion.id}`)
    if (valuesFor(nodeID).action !== 'reuse' || Number(valuesFor(nodeID).bom_id) !== bomID) return
    const variants = (detail.variants || []).map((variant) => ({
      row_id: variant.spec_key,
      name: variant.name,
      unit: variant.inventory_unit,
      is_default: Boolean(variant.is_default),
    })).filter((variant) => variant.row_id)
    if (detail.output_type === 'product' && variants.length === 0) throw new Error('所选商品 BOM 没有可用规格')
    valuesFor(nodeID).variants = variants
    refreshComponentRowsForSource(nodeID)
  } catch (error) {
    optionError.value = error.message || '读取所选 BOM 规格失败'
  }
}

function refreshComponentRowsForSource(sourceNodeID) {
  for (const edge of graph.value.edges.filter((item) => item.source === sourceNodeID && canonicalInputPortID(item.targetHandle) === 'components')) {
    const targetValues = valuesFor(edge.target)
    const existing = Array.isArray(targetValues.components) ? targetValues.components : []
    const sourceOptions = sourceRows(sourceNodeID, edge.target)
    const replacements = sourceOptions.map((source) => existing.find((row) => row.source_node_id === sourceNodeID && row.source_row_id === source.row_id)
      || makeComponentRow(edge.target, sourceNodeID, source.row_id, source))
    targetValues.components = [...existing.filter((row) => row.source_node_id !== sourceNodeID), ...replacements]
  }
}

function ensureMaterialRows(nodeID) {
  const values = valuesFor(nodeID)
  if (!Array.isArray(values.rows)) values.rows = []
  return values.rows
}

function ensureVariants(node) {
  const values = valuesFor(node.id)
  if (!Array.isArray(values.variants)) values.variants = makeInitialValues({ kind: 'bom', config: node.data.config }).variants
  return values.variants
}

function ensureComponents(node) {
  const values = valuesFor(node.id)
  if (!Array.isArray(values.components)) values.components = initialComponentRows(node.id)
  return values.components
}

function addRepeaterRow(nodeID, field) {
  const rows = valuesFor(nodeID)[field] || (valuesFor(nodeID)[field] = [])
  if (field === 'rows') {
    rows.push({ row_id: makeNodeId(), name: '', action: workflowVersion.value >= 2 ? 'reuse' : 'create', kind: 'other', supply_mode: 'purchase', unit: '', owner_type: 'factory', owner_customer_id: 0 })
    if (workflowVersion.value >= 2) refreshComponentRowsForSource(nodeID)
  } else if (field === 'variants') {
    rows.push({ row_id: makeNodeId(), name: '', unit: '', is_default: false })
  }
}

function addComponentFromEdge(node) {
  const connected = graph.value.edges.filter((edge) => edge.target === node.id && canonicalInputPortID(edge.targetHandle) === 'components')
  const values = ensureComponents(node)
  for (const edge of connected) {
    const sourceRowsForEdge = sourceRows(edge.source, node.id)
    for (const source of sourceRowsForEdge) {
      if (values.some((row) => row.source_node_id === edge.source && row.source_row_id === source.row_id)) continue
      values.push(makeComponentRow(node.id, edge.source, source.row_id, source))
    }
  }
  if (!connected.length && !values.length) values.push({ row_id: makeNodeId(), source_node_id: '', quantity: '', unit: '', loss_rate: 0, variant_row_id: '' })
}

function removeRow(nodeID, field, rowID) {
  const values = inputValues.value[nodeID]
  if (!values?.[field]) return
  values[field] = values[field].filter((row) => row.row_id !== rowID)
}

function duplicateComponentRow(nodeID, component) {
  valuesFor(nodeID).components = appendDuplicatedBOMComponentRow(ensureComponents(nodeID), component, makeNodeId())
}

function removeMaterialRow(nodeID, rowID) {
  removeRow(nodeID, 'rows', rowID)
  refreshComponentRowsForSource(nodeID)
}

function setDefaultVariant(nodeID, rowID) {
  const variants = valuesFor(nodeID).variants || []
  for (const row of variants) row.is_default = row.row_id === rowID
}

function sourceNodeName(id) {
  return props.run.workflow?.nodes?.find((node) => node.id === id)?.name || ''
}

function sourceRows(sourceNodeID, targetNodeID, targetHandle = 'components') {
  const edge = graph.value.edges.find((item) => item.source === sourceNodeID && item.target === targetNodeID && canonicalInputPortID(item.targetHandle) === canonicalInputPortID(targetHandle))
  const sourceNode = graph.value.nodes.find((item) => item.id === sourceNodeID)
  if (!edge || !sourceNode) return []
  const values = valuesFor(sourceNodeID)
  if (workflowVersion.value >= 2) {
    if (sourceNode.data.module.kind === 'material') {
      if (sourceNode.data.config.data_role === 'output') {
        return [{ row_id: 'output', label: values.name || generatedOutputName(sourceNode) || sourceNode.data.label || '新物料', unit: values.unit || sourceNode.data.config.unit || '' }]
      }
      return ensureMaterialRows(sourceNodeID).map((row) => ({ row_id: row.row_id, label: row.name || selectedMaterialLabel(row) || '配方物料', unit: row.unit || '' }))
    }
    if (sourceNode.data.module.kind === 'product' && sourceNode.data.config.data_role === 'output') {
      if (edge.sourceHandle === 'specs') {
        const bomNodeID = graph.value.edges.find((item) => item.target === sourceNodeID && item.targetHandle === 'from_bom')?.source
        const template = specificationTemplateDetails.value[bomNodeID || '']
        if (workflowVersion.value >= 4 && template) return (template.variants || []).map((variant) => ({ row_id: variant.spec_key, label: `${variant.name || '商品规格'}${variant.inventory_unit ? ` · ${variant.inventory_unit}` : ''}`, unit: variant.inventory_unit || '' }))
        return (valuesFor(bomNodeID || '').variants || []).map((variant) => ({ row_id: variant.row_id, label: `${variant.name || '商品规格'}${variant.unit ? ` · ${variant.unit}` : ''}`, unit: variant.unit || '' }))
      }
      return [{ row_id: 'output', label: values.name || sourceNode.data.label || '商品', unit: '' }]
    }
    if (sourceNode.data.module.kind === 'product' && edge.sourceHandle === 'specs') {
      const selectedID = Number(values.bom_spec_id || 0)
      return (productSpecOptions.value[sourceNodeID] || [])
        .filter((spec) => selectedID <= 0 || spec.bom_spec_id === selectedID)
        .map((spec) => ({ row_id: spec.row_id, label: spec.label, unit: spec.unit }))
    }
  }
  if (sourceNode.data.module.kind === 'material') {
    return ensureMaterialRows(sourceNodeID).map((row) => ({ row_id: row.row_id, label: row.name || (row.action === 'reuse' ? materialOptions.value.find((item) => item.id === Number(row.material_id))?.label : '') || '新物料', unit: row.unit || (row.action === 'reuse' ? materialOptions.value.find((item) => item.id === Number(row.material_id))?.unit : '') || '' }))
  }
  if (sourceNode.data.module.kind === 'product') {
    const name = values.action === 'reuse' ? productOptions.value.find((item) => item.id === Number(values.product_id))?.label : values.name
    return [{ row_id: 'product', label: name || sourceNode.data.label || '商品', unit: '' }]
  }
  if (sourceNode.data.module.kind === 'bom' && edge.sourceHandle === 'output') {
    return [{ row_id: 'output', label: values.output_name || sourceNode.data.label || '产出对象', unit: '' }]
  }
  if (sourceNode.data.module.kind === 'publish' || (sourceNode.data.module.kind === 'bom' && edge.sourceHandle === 'specs')) {
    if (sourceNode.data.module.kind === 'publish' && edge.sourceHandle === 'output') return [{ row_id: 'output', label: sourceNode.data.label || '已绑定产出对象' }]
    const bomNodeID = sourceNode.data.module.kind === 'publish'
      ? graph.value.edges.find((item) => item.target === sourceNodeID && item.targetHandle === 'bom')?.source
      : sourceNodeID
    const variants = valuesFor(bomNodeID || '').variants || []
    return variants.map((variant) => ({ row_id: variant.row_id, label: `${variant.name || '商品规格'}${variant.unit ? ` · ${variant.unit}` : ''}`, unit: variant.unit || '' }))
  }
  return []
}

function initialComponentRows(nodeID) {
  const edges = graph.value.edges.filter((edge) => edge.target === nodeID && canonicalInputPortID(edge.targetHandle) === 'components')
  return edges.flatMap((edge) => sourceRows(edge.source, nodeID).map((source) => makeComponentRow(nodeID, edge.source, source.row_id, source)))
}

function makeComponentRow(targetNodeID, sourceNodeID, sourceRowID, source) {
  const defaults = workflowVersion.value >= 2
    ? (graph.value.nodes.find((node) => node.id === targetNodeID)?.data.config?.components || []).find((row) => row.source_node_id === sourceNodeID && (!row.source_row_id || row.source_row_id === sourceRowID))
      || (graph.value.nodes.find((node) => node.id === targetNodeID)?.data.config?.components || []).find((row) => row.source_node_id === sourceNodeID)
    : null
  return {
    row_id: makeNodeId(), source_node_id: sourceNodeID, source_row_id: sourceRowID,
    quantity: defaults?.quantity ?? '', unit: defaults?.unit || source.unit || '',
    loss_rate: defaults?.loss_rate || 0, variant_row_id: defaults?.variant_row_id || '',
  }
}

function sourceRowLabel(sourceNodeID, sourceRowID, targetNodeID) {
  return sourceRows(sourceNodeID, targetNodeID).find((row) => row.row_id === sourceRowID)?.label || '待选择物料'
}

function pricingSpecRows(nodeID) {
  const edge = graph.value.edges.find((item) => item.target === nodeID && item.targetHandle === 'product')
  return edge ? sourceRows(edge.source, nodeID, 'product') : []
}

function initialPricingRows(nodeID) {
  return pricingSpecRows(nodeID).map((spec) => ({ row_id: `price-${spec.row_id}`, spec_row_id: spec.row_id, pricing_mode: 'fixed', price: '', pricing_rule_id: 0 }))
}

function ensurePricingRows(node) {
  const values = valuesFor(node.id)
  const current = Array.isArray(values.prices) ? values.prices : []
  const currentIDs = current.map((row) => row.spec_row_id).join('|')
  const specs = pricingSpecRows(node.id)
  const specIDs = specs.map((row) => row.row_id).join('|')
  if (currentIDs !== specIDs) {
    const previous = new Map(current.map((row) => [row.spec_row_id, row]))
    values.prices = specs.map((spec) => previous.get(spec.row_id) || { row_id: `price-${spec.row_id}`, spec_row_id: spec.row_id, pricing_mode: 'fixed', price: '', pricing_rule_id: 0 })
  }
  return values.prices
}

function pricingSpecLabel(nodeID, specRowID) {
  return pricingSpecRows(nodeID).find((row) => row.row_id === specRowID)?.label || '未连接规格'
}

function pricingSpecUnit(nodeID, specRowID) {
  const name = pricingSpecLabel(nodeID, specRowID)
  return name.includes(' · ') ? name.split(' · ').at(-1) : name
}

function purchaseMaterialRows(nodeID) {
  const edge = graph.value.edges.find((item) => item.target === nodeID && item.targetHandle === 'material')
  return edge ? sourceRows(edge.source, nodeID, 'material') : []
}

function followupValues(nodeID) {
  if (!followupInputValues.value[nodeID]) {
    const values = valuesFor(nodeID)
    followupInputValues.value[nodeID] = { quantity: Number(values.quantity || 0), unit_price: Number(values.unit_price || 0) }
  }
  return followupInputValues.value[nodeID]
}

function stepResult(nodeID) {
  const result = props.run.business_results?.steps?.[nodeID]
  return result && typeof result === 'object' ? result : {}
}

function bomExecutionResult(nodeID) {
  const result = stepResult(nodeID)
  return result.bom && typeof result.bom === 'object' ? result.bom : result
}

function processRouteResultSourceLabel(source) {
  return ({ run_override: '本次改选', connected_node: '跟随连线工艺', bom_default: 'BOM 默认工艺', specification_template: '规格模板工艺' })[source] || '工艺路线'
}

function purchaseOrder(nodeID) { return stepResult(nodeID).purchase_order || null }
function purchaseReceipt(nodeID) { return stepResult(nodeID).purchase_receipt || null }
function pricingPreview(nodeID) { return stepResult(nodeID).pricing_preview || null }
function priceDraft(nodeID) { return stepResult(nodeID).price_draft || null }
function publishedPrice(nodeID) { return stepResult(nodeID).published_price || null }

function outputSourceRows(nodeID) {
  const edge = graph.value.edges.find((item) => item.target === nodeID && item.targetHandle === 'output')
  return edge ? sourceRows(edge.source, nodeID, 'output') : []
}

function initialOutputSourceRow(nodeID) {
  const edge = graph.value.edges.find((item) => item.target === nodeID && item.targetHandle === 'output')
  return edge ? sourceRows(edge.source, nodeID, 'output')[0]?.row_id || '' : ''
}

function setProductReferenceOwner(nodeID) {
  const values = valuesFor(nodeID)
  const option = productOptions.value.find((item) => Number(item.id) === Number(values.product_id))
  if (!option) return
  values.owner = Number(option.customer_id || 0) > 0 ? 'customer' : 'factory'
  values.customer_id = Number(option.customer_id || 0)
}

function setMaterialReferenceOwner(row) {
  const option = materialOptions.value.find((item) => Number(item.id) === Number(row.material_id))
  if (!option) return
  row.owner_type = Number(option.owner_customer_id || 0) > 0 ? 'customer' : 'factory'
  row.owner_customer_id = Number(option.owner_customer_id || 0)
}

function materialSearchKey(nodeID, rowID) {
  return `${nodeID}:${rowID}`
}

function selectedMaterialLabel(record) {
  const materialID = Number(record?.material_id || 0)
  if (materialID <= 0) return ''
  const option = (materialSearchResults.value[Object.keys(materialSearchResults.value).find((key) => materialSearchResults.value[key]?.some((row) => Number(row.id) === materialID))] || [])
    .find((row) => Number(row.id) === materialID)
  const fallback = materialOptions.value.find((row) => Number(row.id) === materialID)
  return record.material_name || option?.name || fallback?.name || fallback?.label || `物料 #${materialID}`
}

function normalizedMaterialOption(row) {
  const ownerCustomerID = Number(row.owner_customer_id || 0)
  return {
    id: Number(row.id), name: row.name || '', code: row.code || '', unit: row.unit || row.inventory_unit || '',
    owner_customer_id: ownerCustomerID, owner_type: row.owner_type || (ownerCustomerID > 0 ? 'customer' : 'factory'),
    owner_name: row.owner_name || '', owner_label: row.owner_name || (ownerCustomerID > 0 ? `客户 ${ownerCustomerID}` : '本公司'),
  }
}

function searchMaterialForRow(nodeID, row, query) {
  queueMaterialSearch(nodeID, row.row_id, query)
}

function searchMaterialForOutput(node, query) {
  queueMaterialSearch(node.id, 'output', query)
}

function queueMaterialSearch(nodeID, rowID, query) {
  const key = materialSearchKey(nodeID, rowID)
  materialSearchText.value[key] = query
  const revision = (materialSearchRevisions.get(key) || 0) + 1
  materialSearchRevisions.set(key, revision)
  if (materialSearchTimers.has(key)) clearTimeout(materialSearchTimers.get(key))
  if (String(query || '').trim().length < 2) {
    materialSearchResults.value[key] = []
    materialSearchHasMore.value[key] = false
    materialSearchOffsets.value[key] = 0
    materialSearchLoading.value[key] = false
    return
  }
  materialSearchLoading.value[key] = true
  materialSearchTimers.set(key, setTimeout(() => fetchMaterialSearch(nodeID, rowID, query, 0, false, revision), 240))
}

async function fetchMaterialSearch(nodeID, rowID, query, offset, append, revision) {
  const key = materialSearchKey(nodeID, rowID)
  try {
    const params = new URLSearchParams({ q: String(query || '').trim(), active: 'active', limit: '50', offset: String(offset) })
    const response = await apiGet(`/api/materials?${params.toString()}`)
    if (materialSearchRevisions.get(key) !== revision || materialSearchText.value[key] !== query) return
    const rows = (response.rows || []).map(normalizedMaterialOption)
    materialSearchResults.value[key] = append ? [...(materialSearchResults.value[key] || []), ...rows] : rows
    materialSearchOffsets.value[key] = offset
    materialSearchHasMore.value[key] = rows.length === 50
  } catch (error) {
    optionError.value = error.message || '搜索物料失败'
    materialSearchResults.value[key] = []
  } finally {
    if (materialSearchRevisions.get(key) === revision) materialSearchLoading.value[key] = false
  }
}

function searchMoreMaterials(nodeID, row) {
  const key = materialSearchKey(nodeID, row.row_id)
  const query = materialSearchText.value[key]
  if (!query || materialSearchLoading.value[key] || !materialSearchHasMore.value[key]) return
  const offset = Number(materialSearchOffsets.value[key] || 0) + (materialSearchResults.value[key] || []).length
  const revision = materialSearchRevisions.get(key) || 0
  materialSearchLoading.value[key] = true
  fetchMaterialSearch(nodeID, row.row_id, query, offset, true, revision)
}

function selectMaterialForRow(nodeID, row, rawOption) {
  const option = normalizedMaterialOption(rawOption)
  row.material_id = option.id
  row.material_name = option.name
  row.material_code = option.code
  row.unit = option.unit
  row.owner_type = option.owner_type
  row.owner_customer_id = option.owner_customer_id
  const key = materialSearchKey(nodeID, row.row_id)
  materialSearchResults.value[key] = []
  materialSearchText.value[key] = ''
  refreshComponentRowsForSource(nodeID)
}

function selectMaterialOutput(node, rawOption) {
  const option = normalizedMaterialOption(rawOption)
  const values = valuesFor(node.id)
  values.material_id = option.id
  values.material_name = option.name
  values.material_code = option.code
  values.name = option.name
  values.unit = option.unit
  values.owner_type = option.owner_type
  values.owner_customer_id = option.owner_customer_id
  const key = materialSearchKey(node.id, 'output')
  materialSearchResults.value[key] = []
  materialSearchText.value[key] = ''
}

function handleMaterialAction(nodeID, row) {
  if (row.action === 'create') {
    row.material_id = 0
    row.material_name = ''
    row.material_code = ''
    row.unit = ''
  } else {
    row.name = ''
  }
  const key = materialSearchKey(nodeID, row.row_id)
  materialSearchResults.value[key] = []
  materialSearchText.value[key] = ''
  refreshComponentRowsForSource(nodeID)
}

function generatedOutputName(node) {
  const config = node.data.config || {}
  if (workflowVersion.value >= 3 && Array.isArray(config.name_parts)) return renderNameParts(config.name_parts, props.run.workflow?.variables || [], variableValues.value).value
  let pattern = String(config.name_pattern || config.defaults?.name_pattern || '')
  if (!pattern) return ''
  const productNode = graph.value.nodes.find((candidate) => candidate.data.module.kind === 'product' && candidate.data.config.data_role === 'output')
  const productValues = productNode ? valuesFor(productNode.id) : {}
  const productName = String(productValues.name || productNode?.data.config.name || productNode?.data.label || '')
  for (const token of ['{{商品名称}}', '{{商品}}', '{商品名称}', '{商品}']) pattern = pattern.replaceAll(token, productName)
  return pattern.trim()
}

async function loadProductSpecs(nodeID) {
  const productID = Number(valuesFor(nodeID).product_id || 0)
  if (productID <= 0) {
    productSpecOptions.value[nodeID] = []
    valuesFor(nodeID).bom_spec_id = 0
    syncProductInputComponents(nodeID)
    return
  }
  try {
    const response = await apiGet(`/api/production-bom-product-specs/${productID}`)
    if (Number(valuesFor(nodeID).product_id || 0) !== productID) return
    const rows = Array.isArray(response) ? response : (response.rows || [])
    productSpecOptions.value[nodeID] = rows.map((row) => ({
      row_id: String(row.bom_spec_id), bom_spec_id: Number(row.bom_spec_id), spec_key: row.spec_key,
      label: `${row.name || row.spec_key} · ${row.inventory_unit}${row.bom_name ? ` · ${row.bom_name}` : ` · BOM ${row.bom_id}`}`,
      unit: row.inventory_unit || '', is_default: Boolean(row.is_default),
    }))
    const options = productSpecOptions.value[nodeID]
    const currentID = Number(valuesFor(nodeID).bom_spec_id || 0)
    if (!options.some((row) => row.bom_spec_id === currentID)) {
      const defaults = options.filter((row) => row.is_default)
      valuesFor(nodeID).bom_spec_id = defaults.length === 1 ? defaults[0].bom_spec_id : options.length === 1 ? options[0].bom_spec_id : 0
    }
    syncProductInputComponents(nodeID)
  } catch (error) {
    optionError.value = error.message || '读取商品已发布规格失败'
    productSpecOptions.value[nodeID] = []
  }
}

function syncProductInputComponents(nodeID) {
  const selectedID = Number(valuesFor(nodeID).bom_spec_id || 0)
  const options = productSpecOptions.value[nodeID] || []
  if (selectedID > 0 && !options.some((row) => row.bom_spec_id === selectedID)) valuesFor(nodeID).bom_spec_id = 0
  refreshComponentRowsForSource(nodeID)
}

function setBOMLossPercent(nodeID, value) {
  const percent = Number(value)
  valuesFor(nodeID).material_loss_rate = Number.isFinite(percent) ? Math.max(0, Math.min(99.99, percent)) / 100 : 0
}

function setBOMOutputUnit(nodeID, unit) {
  valuesFor(nodeID).output_unit = unit
  for (const edge of graph.value.edges.filter((item) => item.source === nodeID && item.sourceHandle === 'assembly')) {
    const output = graph.value.nodes.find((node) => node.id === edge.target && node.data.config.data_role === 'output' && node.data.module.kind === 'material')
    if (output) valuesFor(output.id).unit = unit
  }
}

function outputBOMUnitForDataNode(nodeID) {
  const edge = graph.value.edges.find((item) => item.target === nodeID && item.targetHandle === 'from_bom')
  const bom = graph.value.nodes.find((node) => node.id === edge?.source)
  return String(bom?.data.config.output_unit || valuesFor(bom?.id || '').output_unit || '')
}

function outputMaterialUnit(node) {
  return valuesFor(node.id).unit || outputBOMUnitForDataNode(node.id) || node.data.config.unit || ''
}

function hasConnectedRoute(nodeID) {
  return graph.value.edges.some((edge) => edge.target === nodeID && edge.targetHandle === 'route')
}

function isNodeFieldFixed(node, key) {
  if (workflowVersion.value >= 3) return false
  return (node?.data.config?.fixed_fields || []).includes(key)
}

function applyVariableNames() {
  if (workflowVersion.value < 3) return
  for (const node of graph.value.nodes) {
    const parts = node.data.config?.name_parts
    if (!Array.isArray(parts) || !parts.length) continue
    const values = valuesFor(node.id)
    if (values.name_mode === 'manual') continue
    values.name = renderNameParts(parts, props.run.workflow?.variables || [], variableValues.value).value
    values.name_mode = 'automatic'
  }
}

function markNameManual(node) {
  if (workflowVersion.value >= 3) valuesFor(node.id).name_mode = 'manual'
}

function restoreVariableName(node) {
  valuesFor(node.id).name_mode = 'automatic'
  const parts = node.data.config?.name_parts || []
  valuesFor(node.id).name = renderNameParts(parts, props.run.workflow?.variables || [], variableValues.value).value
}

function cloneRunDraft() {
  const draft = { inputs: cloneValue(inputValues.value) }
  if (workflowVersion.value >= 3) draft.variable_values = cloneValue(variableValues.value)
  return draft
}

function bomIdentity(node) {
  const outputEdge = graph.value.edges.find((edge) => edge.source === node.id && edge.sourceHandle === 'assembly')
  const outputNode = graph.value.nodes.find((candidate) => candidate.id === outputEdge?.target)
  if (!outputNode) return node.data.label || 'BOM组装'
  const outputName = valuesFor(outputNode.id).name || generatedOutputName(outputNode) || outputNode.data.label || '待填写'
  const typeName = outputNode.data.module.kind === 'product' ? '商品' : '物料'
  return `${node.data.label || 'BOM组装'} · 产出${typeName}：${outputName}`
}

function issueForNode(nodeID) {
  return props.run.preview?.issues?.find((issue) => issue.node_id === nodeID)
}

function issueForField(nodeID, field) {
  return props.run.preview?.issues?.find((issue) => issue.node_id === nodeID && (issue.field === field || issue.field?.startsWith(`${field}.`)))?.message || ''
}

function statusFor(nodeID) {
  const execution = props.run.business_results?.steps?.[nodeID]
  if (execution?.status === 'succeeded' || execution?.status === 'skipped') return 'complete'
  if (props.run.status === 'config_committed' || props.run.status === 'completed') return 'complete'
  if (props.run.status === 'in_progress' && execution?.status === 'ready') return 'current'
  const preview = props.run.preview
  if (preview?.steps?.some((step) => step.node_id === nodeID && step.status === 'ready')) return 'current'
  return 'waiting'
}

function stepStatusClass(nodeID) {
  return `run-step-${statusFor(nodeID)}`
}

function stepStatusLabel(nodeID) {
  if (props.run.business_results?.steps?.[nodeID]?.status === 'skipped') return '按条件跳过'
  if (props.run.status === 'in_progress' && props.run.business_results?.steps?.[nodeID]?.status === 'ready') return '待处理'
  return ({ complete: '已完成', current: '待提交', waiting: '待填写' })[statusFor(nodeID)]
}

function dependencySummary(node) {
  return graph.value.edges.filter((edge) => edge.target === node.id).map((edge) => {
    const from = sourceNodeName(edge.source)
    return edge.data?.kind === 'prerequisite' ? `${from}完成` : from
  })
}

function fieldHelp(node, key) {
  if (node.data.module.kind === 'purchase' && key === 'quantity') return '采购单不会改变库存；到货后需确认收货。'
  if (node.data.module.kind === 'pricing' && key === 'prices') return '价格表会以新版本发布；目标表中的其他商品与价格保持原样。'
  if (node.data.module.kind === 'material' && key === 'rows') return '引用已有物料不会复制库存或下级 BOM。'
  return ''
}

function referenceOptions(key) {
  if (key === 'route_id') return routeOptions.value
  if (key === 'supplier_id') return supplierOptions.value
  if (key === 'warehouse') return warehouseOptions.value
  if (key === 'price_list_id') return priceListOptions.value
  if (key === 'bom_id') return bomOptions.value
  return []
}

function referencePlaceholder(key) {
  if (key === 'route_id') return '选择工艺路线'
  if (key === 'supplier_id') return '选择供应商'
  if (key === 'price_list_id') return '选择目标价格表'
  if (key === 'bom_id') return '选择已有 BOM'
  return '选择'
}

function pricingOwnerKey(nodeID) {
  const specEdge = graph.value.edges.find((edge) => edge.target === nodeID && edge.targetHandle === 'product')
  const publishID = specEdge?.source
  const bomID = graph.value.edges.find((edge) => edge.target === publishID && edge.targetHandle === 'bom')?.source
  const outputEdge = graph.value.edges.find((edge) => edge.target === bomID && edge.targetHandle === 'output')
  const productNode = graph.value.nodes.find((node) => node.id === outputEdge?.source && node.data.module.kind === 'product')
  if (!productNode) return `official:${nodeID}`
  const productValues = valuesFor(productNode.id)
  if (productValues.action === 'reuse') {
    const option = productOptions.value.find((row) => Number(row.id) === Number(productValues.product_id))
    return Number(option?.customer_id || 0) > 0 ? `customer:${Number(option.customer_id)}` : `official:${nodeID}`
  }
  return productValues.owner === 'customer' && Number(productValues.customer_id) > 0
    ? `customer:${Number(productValues.customer_id)}`
    : `official:${nodeID}`
}

async function loadPriceListOptions() {
  const revision = ++priceTargetLoadRevision
  const owners = [...new Set(pricingNodes.value.map((node) => pricingOwnerKey(node.id)))]
  const results = await Promise.allSettled(owners.map((owner) => {
    const [scope, customerID] = owner.split(':')
    const params = new URLSearchParams({ list_type: 'commercial', publication_purpose: 'factory_supply', scope, view: 'summary', status: 'published', page_size: '100' })
    if (scope === 'customer' && customerID) params.set('customer_id', customerID)
    return apiGet(`/api/costing/bean-list/publications?${params.toString()}`)
  }))
  if (revision !== priceTargetLoadRevision) return
  priceListOptions.value = results.flatMap((result) => result.status === 'fulfilled' ? (result.value.rows || []).map((row) => ({
    id: Number(row.id),
    label: `${row.table_name || row.product_type_name || '商品价格表'} · ${row.version}${row.status !== 'published' ? ' · 非发布版本' : ''}`,
    owner_type: row.owner_type,
    owner_key: row.owner_key || '',
  })) : []).filter((row) => row.id > 0)
}

async function loadOptions() {
  const results = await Promise.allSettled([
    workflowVersion.value >= 2 ? Promise.resolve({ rows: [] }) : apiGet('/api/materials?active=active&limit=500'),
    apiGet('/api/products/options?limit=500'),
    apiGet('/api/purchase/suppliers'),
    apiGet('/api/process-routes?status=active'),
    apiGet('/api/product-settings/units'),
    apiGet('/api/stock/warehouses'),
    apiGet('/api/production-boms?status=all'),
    apiGet('/api/customers?limit=500&active=true'),
    apiGet('/api/product-pricing-rules'),
  ])
  const rows = (result) => result.status === 'fulfilled' ? (result.value.rows || result.value.items || result.value.options || []) : []
  materialOptions.value = rows(results[0]).map((item) => ({ id: item.id, label: `${item.name}${item.code ? ` · ${item.code}` : ''}`, owner_customer_id: item.owner_customer_id || 0, unit: item.unit || item.inventory_unit || '' }))
  productOptions.value = rows(results[1]).map((item) => ({ id: item.id, label: `${item.name}${item.product_code ? ` · ${item.product_code}` : ''}`, customer_id: item.customer_id || 0 }))
  supplierOptions.value = rows(results[2]).filter((item) => item.active !== false).map((item) => ({ id: item.id, label: item.name }))
  routeOptions.value = rows(results[3]).map((item) => ({ id: item.id, label: item.name }))
  unitOptions.value = (results[4].status === 'fulfilled' ? (Array.isArray(results[4].value) ? results[4].value : results[4].value.rows || results[4].value.units || []) : []).filter((item) => item.active !== false)
  warehouseOptions.value = rows(results[5]).filter((item) => item.active !== false).map((item) => ({ id: item.code || item.id, label: item.name || item.code }))
  bomOptions.value = rows(results[6]).filter((item) => item.active !== false).map((item) => ({ id: item.id, label: `${item.name || item.code}${item.code ? ` · ${item.code}` : ''}` }))
  customerOptions.value = rows(results[7]).filter((item) => item.active !== false && Number(item.id) > 0).map((item) => ({ id: item.id, label: item.name || `客户 ${item.code || ''}` }))
  pricingRuleOptions.value = rows(results[8]).filter((item) => item.active !== false && Number(item.id) > 0)
  loadPriceListOptions()
}

function cloneInputs() {
  return cloneValue(inputValues.value)
}
</script>

<style scoped>
.pc-run-page { min-height: calc(100vh - 88px); padding: 0 24px 35px; color: #18273c; background: #f7f9fb; }
.pc-run-header { display: flex; align-items: center; gap: 14px; border-bottom: 1px solid #e3e8ee; margin: 0 -24px 17px; padding: 15px 24px; background: white; }
.pc-run-back { display: grid; place-items: center; flex: 0 0 38px; width: 38px; height: 38px; border: 1px solid #dbe3ed; border-radius: 7px; color: #32435a; background: white; cursor: pointer; }
.pc-run-title { flex: 1; min-width: 0; }
.pc-run-title > span { color: #268051; font-size: 11px; }
.pc-run-title h1 { margin: 2px 0 3px; font-size: 21px; }
.pc-run-title p { margin: 0; color: #78859a; font-size: 12px; }
.pc-run-actions { display: flex; gap: 8px; }
.pc-run-primary, .pc-run-secondary { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 37px; border: 1px solid #d8e0e9; border-radius: 6px; padding: 0 12px; color: #2a3a50; background: white; font: inherit; font-size: 12px; font-weight: 600; cursor: pointer; }
.pc-run-primary { border-color: #288252; color: white; background: #288252; }
.pc-run-primary:disabled, .pc-run-secondary:disabled { opacity: .55; cursor: wait; }
.pc-run-alert { display: flex; align-items: center; gap: 8px; border: 1px solid #f3cac7; border-radius: 6px; margin-bottom: 14px; padding: 9px 12px; color: #a62d25; background: #fff6f5; font-size: 12px; }
.pc-run-result-card { max-width: 1402px; border: 1px solid #c8e5d1; border-radius: 8px; margin: 0 auto 15px; padding: 13px 15px; color: #246f47; background: #f4fbf6; }
.pc-run-result-card > header { display: flex; align-items: center; gap: 9px; }
.pc-run-result-card > header strong, .pc-run-result-card > header span { display: block; }
.pc-run-result-card > header strong { font-size: 13px; }
.pc-run-result-card > header span, .pc-run-result-card > p { margin: 3px 0 0; color: #71839a; font-size: 10px; }
.pc-run-created-objects { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 11px; }
.pc-run-created-objects > div { display: grid; gap: 3px; min-width: 150px; border: 1px solid #dcebe1; border-radius: 6px; padding: 8px 10px; background: white; }
.pc-run-created-objects span, .pc-run-created-objects small { color: #758399; font-size: 9px; }
.pc-run-created-objects strong { color: #263950; font-size: 11px; }
.pc-run-bom-route-results { display: grid; gap: 7px; margin-top: 10px; }
.pc-run-bom-route-results article { display: flex; flex-wrap: wrap; align-items: baseline; gap: 5px 12px; border: 1px solid #dcebe1; border-radius: 6px; padding: 7px 9px; background: white; }
.pc-run-bom-route-results strong { color: #34465b; font-size: 10px; }
.pc-run-bom-route-results span, .pc-run-bom-route-results small { color: #71839a; font-size: 9px; }
.pc-run-followup { border-top: 1px solid #dcebe1; margin-top: 11px; padding-top: 11px; }
.pc-run-followup-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; color: #34465b; font-size: 11px; }
.pc-run-followup-waiting, .pc-run-followup-done { border-radius: 10px; padding: 3px 7px; color: #8a5b20; background: #fff2dc; font-size: 9px; }
.pc-run-followup-done { color: #267349; background: #e5f4ea; }
.pc-run-purchase-order { display: flex; gap: 16px; margin: 7px 0; color: #607089; font-size: 10px; }
.pc-run-price-preview { display: flex; flex-wrap: wrap; gap: 7px; margin: 8px 0; }
.pc-run-price-preview > div { display: grid; gap: 3px; min-width: 115px; border: 1px solid #e0e9e3; border-radius: 6px; padding: 7px 9px; background: white; }
.pc-run-price-preview span { color: #77869a; font-size: 9px; }
.pc-run-price-preview strong { color: #267349; font-size: 12px; }
.pc-run-price-actions { display: flex; gap: 8px; margin: 8px 0; }
.pc-run-receipt-form { display: flex; align-items: end; flex-wrap: wrap; gap: 9px; margin-top: 8px; }
.pc-run-receipt-form label { display: grid; gap: 4px; min-width: 130px; color: #617189; font-size: 9px; }
.pc-run-receipt-form input { min-height: 34px; width: 150px; border: 1px solid #dce4ed; border-radius: 5px; padding: 6px 8px; font: inherit; font-size: 11px; box-sizing: border-box; }
.pc-run-receipt-form small { flex-basis: 100%; color: #7b889a; font-size: 9px; }
.pc-run-layout { display: grid; grid-template-columns: minmax(530px, 1.4fr) minmax(300px, .8fr); align-items: start; gap: 16px; max-width: 1450px; margin: 0 auto; }
.pc-run-form { min-width: 0; }
.pc-run-section-heading { display: flex; align-items: flex-end; justify-content: space-between; margin: 1px 0 12px; }
.pc-run-section-heading h2, .pc-progress-heading h2 { margin: 0 0 4px; font-size: 16px; }
.pc-run-section-heading p { margin: 0; color: #77859a; font-size: 11px; }
.pc-run-section-heading p b { color: #cf3e35; }
.pc-run-section-heading > span { color: #7a8799; font-size: 10px; }
.pc-run-variables { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 10px; border: 1px solid #dce9e1; border-radius: 8px; margin: 0 0 13px; padding: 12px; background: #f8fcf9; }
.pc-run-variables > header { grid-column: 1 / -1; }
.pc-run-variables > header strong, .pc-run-variables > header small { display: block; }
.pc-run-variables > header strong { color: #2e6044; font-size: 12px; }
.pc-run-variables > header small { margin-top: 3px; color: #768a7d; font-size: 10px; }
.pc-run-variables > label { display: grid; gap: 5px; min-width: 0; color: #44566a; font-size: 10px; }
.pc-run-variables > label span small { margin-left: 6px; color: #8694a3; font-size: 9px; font-weight: 400; }
.pc-run-variables input, .pc-bom-run-fields input:not([type=radio]), .pc-bom-run-fields select { width: 100%; min-height: 35px; border: 1px solid #dce4ed; border-radius: 6px; padding: 7px 9px; outline: 0; color: #26364b; background: white; font: inherit; font-size: 11px; box-sizing: border-box; }
.pc-run-variables input:focus, .pc-bom-run-fields input:focus, .pc-bom-run-fields select:focus { border-color: #65b583; box-shadow: 0 0 0 2px #d9f0e1; }
.pc-bom-run-fields { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; grid-column: 1 / -1; width: 100%; min-width: 0; }
.pc-run-field-inline { display: grid; gap: 5px; min-width: 0; color: #38485f; font-size: 10px; font-weight: 600; }
.pc-run-field-inline > span { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 5px; }
.pc-run-field-inline > strong { display: flex; align-items: center; min-height: 35px; border: 1px solid #e2e8ef; border-radius: 6px; padding: 7px 9px; color: #52647a; background: #f8fafc; font-size: 11px; font-weight: 500; }
.pc-run-field-pair { display: grid; grid-template-columns: repeat(auto-fit, minmax(165px, 1fr)); gap: 10px; min-width: 0; }
.pc-run-field-pair label, .pc-bom-run-defaults label { display: grid; gap: 5px; min-width: 0; color: #64738a; font-size: 10px; }
.pc-bom-run-defaults { display: grid; grid-template-columns: repeat(auto-fit, minmax(155px, 1fr)); gap: 10px; border: 1px solid #e8edf2; border-radius: 7px; padding: 11px; background: #fbfcfd; }
.pc-reset-name { border: 0; padding: 0; color: #2874b6; background: transparent; font: inherit; font-size: 9px; font-weight: 500; cursor: pointer; }
.pc-run-step { border: 1px solid #e1e7ee; border-radius: 8px; margin: 0 0 12px; background: white; box-shadow: 0 1px 2px #28394c09; }
.pc-run-step.has-issue { border-color: #e8aaa6; }
.pc-run-step-heading { display: flex; align-items: center; gap: 11px; border-bottom: 1px solid #edf0f4; padding: 12px 14px; }
.pc-run-step-heading > div { flex: 1; min-width: 0; }
.pc-run-step-number { display: grid; place-items: center; flex: 0 0 28px; width: 28px; height: 28px; border-radius: 7px; color: #236f48; background: #e7f5ec; font-size: 12px; font-weight: 700; }
.pc-run-step-number.kind-material { color: #2f62ad; background: #e8efff; }
.pc-run-step-number.kind-purchase { color: #b7611c; background: #fff0e1; }
.pc-run-step-number.kind-pricing { color: #7a4cab; background: #f1e9fb; }
.pc-run-step-heading h3 { overflow: hidden; margin: 0; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.pc-run-step-heading p { overflow: hidden; margin: 3px 0 0; color: #7b889b; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.pc-run-step-state { border-radius: 12px; padding: 4px 8px; color: #62738a; background: #f1f4f7; font-size: 10px; white-space: nowrap; }
.run-step-current { color: #206e43; background: #eaf6ee; }
.run-step-complete { color: #fff; background: #2b8654; }
.pc-run-step-body { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 15px; padding: 14px; }
.pc-run-field { min-width: 0; }
.pc-run-field.wide { grid-column: 1 / -1; }
.pc-run-field > label { display: block; margin-bottom: 6px; color: #38485f; font-size: 11px; font-weight: 600; }
.pc-run-field > label b { margin-left: 3px; color: #cf3e35; }
.pc-run-field input:not([type=checkbox]), .pc-run-field select, .pc-run-field textarea,
.pc-row-field input, .pc-row-field select, .pc-bom-variant-row input, .pc-component-row input, .pc-component-row select, .pc-run-reference-inline select {
  width: 100%; min-height: 35px; border: 1px solid #dce4ed; border-radius: 5px; padding: 7px 9px; outline: 0; color: #26364b; background: white; font: inherit; font-size: 11px; box-sizing: border-box;
}
.pc-run-field input:focus, .pc-run-field select:focus, .pc-run-field textarea:focus, .pc-row-field input:focus, .pc-row-field select:focus { border-color: #65b583; box-shadow: 0 0 0 2px #d9f0e1; }
.pc-run-field textarea { resize: vertical; }
.pc-run-help, .pc-run-error { display: block; margin-top: 5px; font-size: 10px; }
.pc-run-help { color: #7d8a9d; }
.pc-run-error { color: #b42318; }
.pc-run-check { display: flex; align-items: center; gap: 7px; min-height: 34px; color: #34445a; font-size: 11px; }
.pc-run-check input { accent-color: #278456; }
.pc-run-repeater { display: grid; gap: 10px; }
.pc-price-row { display: grid; grid-template-columns: minmax(190px, 1.4fr) minmax(135px, .8fr) minmax(135px, .8fr); align-items: end; gap: 10px; border: 1px solid #e8edf2; border-radius: 7px; padding: 10px; background: #fbfcfd; }
.pc-price-row label { display: grid; gap: 5px; color: #718097; font-size: 10px; }
.pc-price-row select, .pc-price-row input { width: 100%; min-height: 34px; border: 1px solid #dce4ed; border-radius: 5px; padding: 6px 8px; color: #26364b; background: white; font: inherit; font-size: 11px; box-sizing: border-box; }
.pc-price-spec { display: flex; align-items: center; gap: 8px; min-width: 0; color: #288052; }
.pc-price-spec > div { display: grid; gap: 3px; min-width: 0; }
.pc-price-spec strong { overflow: hidden; color: #304156; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.pc-price-spec small { color: #8090a3; font-size: 9px; }
.pc-material-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)) 30px; align-items: end; gap: 8px; border: 1px solid #e8edf2; border-radius: 7px; padding: 9px; background: #fbfcfd; }
.pc-row-field { min-width: 0; }
.pc-row-field > span { display: block; overflow: hidden; margin-bottom: 5px; color: #718097; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.pc-row-wide { grid-column: 1 / -2; }
.pc-run-icon-button { display: grid; place-items: center; width: 29px; height: 32px; border: 1px solid transparent; border-radius: 5px; color: #718097; background: transparent; cursor: pointer; }
.pc-run-icon-button:hover { color: #b42318; background: #fff0ef; }
.pc-run-add { display: inline-flex; align-items: center; gap: 5px; width: fit-content; border: 0; padding: 3px 1px; color: #2874b6; background: transparent; font: inherit; font-size: 11px; cursor: pointer; }
.pc-bom-variant-list, .pc-bom-components { border: 1px solid #e8edf2; border-radius: 7px; padding: 10px; background: #fcfdfe; }
.pc-bom-components { margin-top: 8px; }
.pc-repeater-caption { margin-bottom: 8px; color: #36465b; font-size: 11px; font-weight: 700; }
.pc-repeater-caption small { margin-left: 6px; color: #8a96a6; font-size: 10px; font-weight: 400; }
.pc-bom-variant-row { display: grid; grid-template-columns: minmax(0, 1fr) 92px 55px 30px; gap: 7px; margin-bottom: 8px; }
.pc-default-variant { display: flex; align-items: center; gap: 3px; color: #607088; font-size: 10px; white-space: nowrap; }
.pc-default-variant input { accent-color: #268252; }
.pc-component-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(135px, 1fr)) 62px; align-items: end; gap: 8px; margin-bottom: 10px; }
.pc-template-run-preview { display: grid; gap: 8px; border: 1px solid #e2eaf0; border-radius: 8px; padding: 10px; background: #fbfcfd; }
.pc-template-run-preview > header { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 5px; color: #34465d; font-size: 11px; }
.pc-template-run-preview > header span { color: #728198; font-size: 10px; }
.pc-template-run-preview article { border: 1px solid #e8edf2; border-radius: 6px; padding: 8px; background: white; }
.pc-template-run-variant-heading, .pc-template-run-item { display: flex; justify-content: space-between; gap: 8px; color: #40536b; font-size: 10px; }
.pc-template-run-variant-heading span { display: flex; gap: 8px; color: #718096; }
.pc-template-run-variant-heading b { color: #2b8051; font-weight: 600; }
.pc-template-run-meta { margin: 5px 0; color: #728198; font-size: 9px; line-height: 1.5; }
.pc-template-run-item { border-top: 1px solid #f0f2f5; margin-top: 4px; padding-top: 5px; }
.pc-template-run-item small { color: #7c8999; font-size: 9px; font-weight: 400; }
.pc-template-run-empty { border: 1px dashed #e0c7a0; border-radius: 6px; padding: 11px; color: #8a6b3c; background: #fffaf0; font-size: 10px; }
.pc-run-field-error { color: #b33e36; font-size: 10px; }
.pc-component-actions { display: flex; align-items: center; justify-content: flex-end; gap: 2px; }
.pc-component-source { display: flex; align-items: center; gap: 5px; min-width: 0; min-height: 34px; overflow: hidden; border: 1px solid #dce7df; border-radius: 5px; padding: 0 7px; color: #33724f; background: #f5fbf7; font-size: 10px; }
.pc-component-source span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pc-component-row label span { display: block; margin-bottom: 4px; color: #728096; font-size: 9px; }
.pc-component-row input, .pc-component-row select { min-height: 32px; padding: 5px 6px; }
.pc-run-reference-inline { margin-top: 9px; }
.pc-run-reference-inline label { display: block; margin-bottom: 5px; color: #42526a; font-size: 10px; }
.pc-run-dependencies { display: flex; align-items: center; gap: 5px; border-top: 1px solid #eff2f5; padding: 8px 14px; color: #62748b; font-size: 10px; }
.pc-run-progress { position: sticky; top: 12px; overflow: hidden; border: 1px solid #e1e7ee; border-radius: 8px; background: white; }
.pc-progress-heading { display: flex; align-items: center; justify-content: space-between; padding: 13px 14px 10px; }
.pc-progress-heading h2 { margin: 0; }
.pc-progress-heading span { color: #7e8a9c; font-size: 10px; }
.pc-run-flow-wrap { height: 367px; margin: 0 11px; overflow: hidden; border: 1px solid #edf0f4; border-radius: 6px; background: #fafbfd; }
.pc-run-flow-wrap :deep(.vue-flow) { width: 100%; height: 100%; }
.pc-run-flow-wrap :deep(.creator-node) { transform: scale(.83); transform-origin: center; }
.pc-run-flow-wrap :deep(.vue-flow__handle) { opacity: .6; }
.pc-run-progress .pc-run-safety { display: flex; align-items: flex-start; gap: 8px; border-top: 1px solid #edf0f4; margin-top: 10px; padding: 12px 14px; color: #66809a; }
.pc-run-safety p { margin: 0; font-size: 10px; line-height: 1.55; }
.pc-progress-legend { display: flex; gap: 13px; padding: 9px 14px 1px; color: #748197; font-size: 9px; }
.pc-progress-legend span { display: flex; align-items: center; gap: 5px; }
.pc-progress-legend i { width: 8px; height: 8px; border-radius: 50%; background: #d8e0e8; }
.pc-progress-legend i.complete { background: #2b8654; }
.pc-progress-legend i.current { background: #acdaba; }
.pc-progress-legend i.waiting { background: #d8e0e8; }
.pc-run-preview-result { border: 1px solid #b9dec6; border-radius: 8px; margin-top: 13px; padding: 13px; color: #206d42; background: #f4fbf6; }
.pc-run-preview-result.invalid { border-color: #efc4c1; color: #a2332c; background: #fff8f7; }
.pc-run-preview-result header { display: flex; align-items: center; gap: 9px; }
.pc-run-preview-result header strong, .pc-run-preview-result header span { display: block; }
.pc-run-preview-result header strong { font-size: 12px; }
.pc-run-preview-result header span { margin-top: 3px; color: #73839a; font-size: 10px; }
.pc-run-preview-steps > div { display: flex; justify-content: space-between; gap: 10px; border-top: 1px solid #e6eee8; margin-top: 8px; padding-top: 8px; font-size: 10px; }
.pc-run-preview-steps > div > small { flex-basis: 100%; color: #708099; font-size: 9px; text-align: right; }
.pc-run-preview-steps strong { color: #34475e; font-weight: 600; text-align: right; }
.pc-run-preview-issue { margin-top: 7px; color: #af352d; font-size: 10px; }
@media (max-width: 1050px) {
  .pc-run-layout { grid-template-columns: minmax(460px, 1.3fr) minmax(270px, .8fr); }
  .pc-material-row { grid-template-columns: repeat(2, minmax(0, 1fr)) 30px; }
  .pc-material-row .pc-run-icon-button { grid-column: 3; grid-row: 1; }
  .pc-component-row { grid-template-columns: repeat(2, minmax(0, 1fr)) 62px; }
  .pc-component-source { grid-column: 1 / -1; }
  .pc-component-row label:nth-of-type(n+3) { grid-row: auto; }
}
@media (max-width: 900px) {
  .pc-run-page { padding: 0 12px 25px; }
  .pc-run-header { flex-wrap: wrap; margin: 0 -12px 13px; padding: 12px; }
  .pc-run-title { flex-basis: calc(100% - 55px); }
  .pc-run-actions { width: 100%; justify-content: flex-end; }
  .pc-run-layout { display: flex; flex-direction: column; }
  .pc-run-form, .pc-run-progress { width: 100%; }
  .pc-run-progress { position: static; order: 1; }
  .pc-run-flow-wrap { height: 250px; }
  .pc-run-section-heading > span { display: none; }
}
@media (max-width: 520px) {
  .pc-run-step-body { grid-template-columns: 1fr; }
  .pc-run-field.wide { grid-column: 1; }
  .pc-bom-variant-row { grid-template-columns: minmax(0, 1fr) 68px 48px 28px; gap: 4px; }
  .pc-bom-variant-row input { padding-inline: 5px; }
  .pc-component-row { grid-template-columns: repeat(2, minmax(0, 1fr)) 62px; }
}
</style>
