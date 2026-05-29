<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>产品价格表</h2>
        <div class="actions">
          <button class="secondary" type="button" :disabled="loading" @click="loadBeanList">刷新</button>
          <button class="secondary" type="button" @click="settingsOpen = true">参数设置</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div v-if="inactiveBomWarningCount" class="warning-banner">BOM已失效：{{ inactiveBomWarningCount }} 款产品依赖的 BOM 已失效，发布价格表前请先重新启用 BOM。</div>
      <div class="metrics">
        <div>
          <span>商品数</span>
          <strong>{{ customerScopedSkuCount }}</strong>
        </div>
      </div>
      <div class="bean-list-global-scope">
        <div>
          <span>豆单范围</span>
          <strong>{{ publicationScopeLabel(versionListScope) }}</strong>
        </div>
        <label v-if="!isWorkspaceCustomerLocked" class="scope-select">
          <select v-model="versionListScope" aria-label="豆单范围">
            <option value="official">公共豆单</option>
            <option v-for="customer in customers" :key="`version-scope-${customer.id}`" :value="`customer:${customer.id}`">
              {{ customerOptionLabel(customer) }}
            </option>
          </select>
        </label>
        <div v-else class="scope-select locked-scope">{{ publicationScopeLabel(versionListScope) }}</div>
      </div>
    </section>

    <section class="panel bean-list-version-panel">
      <div class="section-bar bean-list-version-head">
        <div>
          <div class="section-title">已发布价格表</div>
          <p class="muted">查看当前范围下的已发布价格表、生成新版和撤回。</p>
        </div>
        <div class="actions">
          <button class="secondary compact" type="button" :disabled="beanListVersionListLoading" @click="refreshBeanListVersionList">刷新版本</button>
        </div>
      </div>

      <div class="version-controls">
        <label>
          <span>产品类型</span>
          <select v-model.number="selectedProductTypeCategoryID" :disabled="!productPriceListTypeOptions.length">
            <option v-for="type in productPriceListTypeOptions" :key="type.key" :value="type.id">
              {{ type.label }}（{{ type.itemCount }}款）
            </option>
          </select>
        </label>
        <div class="version-summary">
          <span>当前发布</span>
          <strong>{{ versionListCurrentPublication?.version || '暂无' }}</strong>
        </div>
        <div class="version-summary">
          <span>版本数</span>
          <strong>{{ currentScopePublicationRows.length }}</strong>
        </div>
      </div>

      <div v-if="currentScopePublicationRows.length" class="version-table-wrap">
        <table class="version-table">
          <thead>
            <tr>
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
            <tr v-for="row in currentScopePublicationRows" :key="`bean-list-version-${row.id}`">
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
      </div>
      <div v-else class="muted empty">
        当前{{ publicationScopeLabel(versionListScope) }}暂无{{ selectedProductPriceListLabel }}价格表版本。
      </div>
    </section>

    <section class="panel">
      <div class="bean-list-generate-bar">
        <div>
          <div class="section-title">生成价格表</div>
          <p class="muted">按当前豆单范围和 SKU 设置里的产品类型生成公共或客户产品价格表。</p>
        </div>
        <button class="primary" type="button" :disabled="loading || !visibleCostingItems.length || !productPriceListTypeOptions.length" @click="openBeanListDrawer()">生成价格表</button>
      </div>
    </section>

    <section v-for="section in productPriceListPreviewSections" :key="section.key" class="panel collapsible-bean-section">
      <div class="collapsible-bean-head">
        <button class="section-toggle" type="button" :aria-expanded="!section.collapsed" @click="toggleBeanListPreviewSection(section.key)">
          <span>
            <b>{{ section.label }}产品价格表</b>
            <small>{{ section.groups.length }} 类 · {{ section.itemCount }} 款</small>
          </span>
          <span>{{ section.collapsed ? '展开' : '收起' }}</span>
        </button>
      </div>
      <div v-show="!section.collapsed && section.listType === 'green'" class="green-price-save-bar">
        <p class="muted">梯度按 KG，单价按元/KG；这里修改的是草稿价，生成并发布新版豆单后，录单才会使用新价格。</p>
        <button class="secondary compact" type="button" :disabled="beanListPublishing || !section.groups.length || !customerScopeReady" @click="saveGreenBeanPriceDraftForSection(section)">
          {{ beanListPublishing ? '保存中' : '保存生豆价格' }}
        </button>
      </div>
      <div v-show="!section.collapsed" class="bean-groups">
        <section v-for="group in section.groups" :key="`${section.key}-${group.category}`" class="bean-group">
          <h3>{{ group.category }}</h3>
          <div class="bean-grid">
            <article v-for="item in group.items" :key="`${section.key}-${item.product_id || item.name}`">
              <div class="bean-heading">
                <span class="bean-code">{{ beanMeta(item, section.metaKey).code }}</span>
                <div>
                  <div class="bean-title">{{ beanName(item, section.metaKey) }}</div>
                  <div v-if="beanMeta(item, section.metaKey).recommended_use" class="bean-use">
                    {{ beanMeta(item, section.metaKey).recommended_use }}
                  </div>
                </div>
              </div>
              <div v-if="itemWarnings(item).length" class="bean-warning-list">
                <span v-for="warning in itemWarnings(item)" :key="`${section.key}-warning-${item.product_id || item.name}-${warning}`" class="warning-chip">{{ warning }}</span>
              </div>
              <div v-if="itemProductAttributeLines(item).length" class="bean-attrs">
                <span v-for="line in itemProductAttributeLines(item)" :key="`${section.key}-attr-${item.product_id || item.name}-${line}`">{{ line }}</span>
              </div>
              <div v-if="beanFlavor(item, section.metaKey)" class="bean-note">{{ beanFlavor(item, section.metaKey) }}</div>
              <div v-if="beanDescription(item, section.metaKey)" class="bean-desc">{{ beanDescription(item, section.metaKey) }}</div>
              <template v-if="section.listType === 'drip'">
                <div class="bean-row" v-for="tier in dripDisplayTiers(item)" :key="`drip-tier-${tier.label}-${tier.sales_unit}`">
                  <span>{{ tier.label }} / {{ dripTierUnit(tier) }}</span>
                  <strong>
                    {{ price(tier.price_per_unit) }}
                    <button v-if="item.drip_price_template" class="source-button" type="button" @click="openDripPriceExplanation(item, tier)">来源</button>
                  </strong>
                </div>
              </template>
              <template v-else-if="section.listType === 'retail'">
                <div class="bean-row" v-for="tier in item.retail_bean_tiers || []" :key="`retail-tier-${tier.label}`">
                  <span>{{ tier.label }}</span><strong>{{ price(tier.price_per_unit) }}</strong>
                </div>
                <div class="bean-row"><span>挂耳10袋</span><strong>{{ price(item.retail_drip_10_bag_price) }}</strong></div>
              </template>
              <template v-else-if="section.listType === 'green'">
                <div class="bean-row green-inline-price-editor" v-for="tier in greenTierPriceRows(item)" :key="`green-tier-${greenTierOverrideKey(tier)}`">
                  <span>{{ tier.label }}</span>
                  <label>
                    <input type="number" min="0" step="0.01" :value="greenTierPriceValue(itemProductID(item), tier)" @input="setGreenBeanTierPrice(itemProductID(item), tier, $event.target.value)" />
                    <small>/{{ greenTierPriceUnit(tier) }}</small>
                  </label>
                </div>
              </template>
              <template v-else>
                <div class="bean-row" v-for="tier in item.commercial_wholesale_tiers || []" :key="tier.label">
                  <span>{{ tier.label }}</span>
                  <strong>
                    {{ price(tierPriceValue(tier)) }}/{{ tierUnit(tier) }}
                    <button v-if="item.gradient_template" class="source-button" type="button" @click="openPriceExplanation(item, tier)">来源</button>
                  </strong>
                </div>
              </template>
            </article>
          </div>
        </section>
        <div v-if="!section.groups.length" class="muted empty-card">暂无产品价格表数据</div>
      </div>
    </section>

    <div v-if="settingsOpen" class="drawer-backdrop" @click.self="settingsOpen = false">
      <aside class="settings-drawer" aria-label="快速成本参数设置">
        <div class="drawer-head">
          <div>
            <h3>快速成本参数设置</h3>
            <p>保存单个参数后，豆单数据会自动刷新。</p>
          </div>
          <button class="secondary" type="button" @click="settingsOpen = false">关闭</button>
        </div>
        <CostingSettingsPanel compact :show-header="false" @saved="handleSettingSaved" />
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
          <div v-if="isDripExplanation">
            <span>袋价</span>
            <strong v-if="isDripExplanation">{{ price(priceExplanation.packed_price_per_bag) }}/袋</strong>
          </div>
          <div v-if="isDripExplanation">
            <span>盒价</span>
            <strong>{{ price(priceExplanation.packed_price_per_box) }}/盒</strong>
          </div>
          <div v-if="!isDripExplanation">
            <span>当前试算</span>
            <strong>{{ price(priceExplanation.saved_final_price) }}/{{ gradientDisplayUnitLabel(priceExplanation.display_unit).replace('元/', '') }}</strong>
          </div>
          <div v-if="!isDripExplanation">
            <span>临时试算</span>
            <strong>{{ price(priceExplanation.preview_final_price) }}/{{ gradientDisplayUnitLabel(priceExplanation.display_unit).replace('元/', '') }}</strong>
          </div>
        </div>
        <div v-if="priceExplanation && !isDripExplanation" class="explanation-form">
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
        <p class="muted">{{ isDripExplanation ? '挂耳价格来源只展示当前公式步骤；交易价格仍需发布价格表后生效。' : '这里的参数只做临时试算；保存请回到快速成本参数或梯度模板，交易价格仍需发布后生效。' }}</p>
      </aside>
    </div>

    <div v-if="pdfDrawerOpen" class="drawer-backdrop" @click.self="pdfDrawerOpen = false">
      <aside class="settings-drawer pdf-drawer" aria-label="生成价格表 PDF">
        <div class="drawer-head">
          <div>
            <h3>生成价格表</h3>
            <p>按手机查看宽度预览，发布后保留版本记录，也可在浏览器打印窗口保存为 PDF。</p>
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
          <p v-if="actorLoaded && !isBeanListAdmin" class="muted">客户账号只能保存修改和下载豆单，发布由管理员执行。</p>
        </div>

        <div class="copy-config-box bean-list-publish-reminder">
          <div>
            <strong>发布提醒</strong>
            <p v-if="pdfTheme.listType === 'green'">梯度按 KG，单价按元/KG；生成并发布新版价格表后，录单才会使用新价格。</p>
            <p v-else>生成并发布新版豆单后，录单和客户侧才会使用新价格。</p>
          </div>
        </div>

        <div class="copy-config-box" v-if="(publicationScope === 'mine' || publicationScope === 'customer') && officialPriceSourcePublications.length">
          <div>
            <strong>复制官方价格来源</strong>
            <p>复制棵凡已发布价格表的报价、风味、特点和出品建议，作为本次客户价格表的锁定内容快照。</p>
          </div>
          <div class="copy-config-actions">
            <select v-model="selectedPriceSourcePublicationID">
              <option value="">选择官方豆单价格</option>
              <option v-for="row in officialPriceSourcePublications" :key="`price-source-${row.id}`" :value="String(row.id)">
                {{ beanListPublicationLabel(row) }}
              </option>
            </select>
            <button class="secondary" type="button" :disabled="!selectedPriceSourcePublication" @click="applyCopiedBeanListPriceSource()">复制价格</button>
          </div>
        </div>

        <div class="pdf-form">
          <label>
            <span>产品类型</span>
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

        <div class="pdf-picker productSelection">
          <div class="picker-head">
            <strong>选择分类和产品</strong>
            <span class="muted">{{ pdfSelectedProductIDs.length }}/{{ pdfAvailableItems.length }} 款</span>
            <div class="picker-actions">
              <button class="secondary compact" type="button" @click="setAllPdfProducts(true)">全选</button>
              <button class="secondary compact" type="button" @click="setAllPdfProducts(false)">清空</button>
            </div>
          </div>
          <div class="product-picker-list categoryProductGroups">
            <section v-for="category in categoryProductGroups" :key="`pick-cat-${category.code}`" class="product-picker-category">
              <div class="product-picker-category-head">
                <label class="check-line">
                  <input type="checkbox" :checked="isPdfCategorySelected(category.code)" @change="togglePdfCategoryProducts(category.code, $event.target.checked)" />
                  <span>{{ category.label }}</span>
                </label>
                <span class="muted">{{ selectedCountForCategory(category.code) }}/{{ category.items.length }} 款</span>
              </div>
              <article v-for="row in category.items" :key="`pick-${itemProductID(row)}`" class="product-picker-row">
                <label class="check-line">
                  <input type="checkbox" :checked="isPdfProductSelected(itemProductID(row))" @change="togglePdfProduct(itemProductID(row), $event.target.checked)" />
                  <span>{{ beanMeta(row, metaKeyForListType(pdfTheme.listType)).code }} {{ beanName(row, metaKeyForListType(pdfTheme.listType)) }}</span>
                </label>
                <div class="customizer-row">
                  <select :value="customizerField(itemProductID(row), 'badge')" @change="setCustomizerField(itemProductID(row), 'badge', $event.target.value)">
                    <option value="">无标签</option>
                    <option value="new">NEW 上新</option>
                    <option value="thumb">👍 推荐</option>
                    <option value="medal">🏅 推荐</option>
                  </select>
                  <input :value="customizerField(itemProductID(row), 'highlightTerms')" placeholder="标红词，用逗号分隔" @input="setCustomizerField(itemProductID(row), 'highlightTerms', $event.target.value)" />
                </div>
                <div v-if="pdfTheme.listType === 'green' && greenTierPriceRows(row).length" class="green-tier-price-editor">
                  <label v-for="tier in greenTierPriceRows(row)" :key="`green-price-${itemProductID(row)}-${greenTierOverrideKey(tier)}`">
                    <span>{{ tier.label }}</span>
                    <input type="number" min="0" step="0.01" :value="greenTierPriceValue(itemProductID(row), tier)" @input="setGreenBeanTierPrice(itemProductID(row), tier, $event.target.value)" />
                    <small>/{{ greenTierPriceUnit(tier) }}</small>
                  </label>
                </div>
              </article>
            </section>
          </div>
        </div>

        <div class="pdf-preview-title">
          <strong>预览</strong>
          <span>{{ pdfTotalItems }} 款</span>
          <div class="pdf-actions">
            <button v-if="isBeanListAdmin" class="secondary" type="button" :disabled="beanListWithdrawing || !currentBeanListPublication" @click="withdrawBeanList()">撤回发布</button>
            <button v-if="isBeanListAdmin" class="primary" type="button" :disabled="beanListPublishing || !pdfGroups.length || !pdfTheme.version || !customerScopeReady" @click="publishBeanList">发布价格表</button>
            <button v-else class="primary" type="button" :disabled="beanListPublishing || !pdfGroups.length || !pdfTheme.version || !customerScopeReady" @click="saveBeanListDraft">保存修改</button>
            <button class="secondary" type="button" :disabled="beanListPdfGenerating || !pdfGroups.length" @click="generateBeanListPdf">{{ beanListPdfGenerating ? '生成中' : '生成 PDF' }}</button>
          </div>
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
                    <div v-if="item.flavor" class="pdf-table-line">
                      <span v-for="(part, idx) in highlightedParts(item.flavor, item)" :key="`pf-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </div>
                    <div v-if="item.description" class="pdf-table-line">
                      <span v-for="(part, idx) in highlightedParts(item.description, item)" :key="`pd-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </div>
                    <div v-for="line in item.attributeLines || []" :key="`pa-${item.code}-${line}`" class="pdf-table-line"><b>属性</b> {{ line }}</div>
                    <div v-if="item.recommendedUse" class="pdf-table-line">
                      <b>出品</b>
                      <span v-for="(part, idx) in highlightedParts(item.recommendedUse, item)" :key="`pu-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </div>
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
                      <p v-if="item.recommendedUse" class="pdf-meta-line">
                        <b>出品建议</b>
                        <span v-for="(part, idx) in highlightedParts(item.recommendedUse, item)" :key="`cu-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                      </p>
                    </div>
                  </div>
                  <p v-if="item.flavor" class="pdf-flavor">
                    <b>风味</b>
                    <span>
                      <span v-for="(part, idx) in highlightedParts(item.flavor, item)" :key="`cf-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </span>
                  </p>
                  <p v-if="item.description" class="pdf-desc">
                    <b>特点</b>
                    <span>
                      <span v-for="(part, idx) in highlightedParts(item.description, item)" :key="`cd-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </span>
                  </p>
                  <p v-for="line in item.attributeLines || []" :key="`preview-attr-${item.code}-${line}`" class="pdf-meta-line"><b>属性</b><span>{{ line }}</span></p>
                  <div class="pdf-price-block">
                    <div class="pdf-section-label">报价</div>
                    <div class="pdf-price-list">
                      <div v-for="priceRow in item.prices" :key="`preview-${item.code}-${priceRow.label}`" class="pdf-price">
                        <span class="pdf-price-label" :class="{ 'pdf-red': priceRow.red }">
                          <span v-for="(part, idx) in priceLabelParts(priceRow, item)" :key="`cpl-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                        </span>
                        <strong class="pdf-price-value" :class="priceValueClass(priceRow, item)">
                          <span v-for="(part, idx) in priceValueParts(priceRow, item)" :key="`cpv-${item.code}-${priceRow.label}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
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
                  <div v-if="item.flavor" class="pdf-table-line">
                    <span v-for="(part, idx) in highlightedParts(item.flavor, item)" :key="`pf-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                  </div>
                  <div v-if="item.description" class="pdf-table-line">
                    <span v-for="(part, idx) in highlightedParts(item.description, item)" :key="`pd-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                  </div>
                  <div v-for="line in item.attributeLines || []" :key="`pa-print-${item.code}-${line}`" class="pdf-table-line"><b>属性</b> {{ line }}</div>
                  <div v-if="item.recommendedUse" class="pdf-table-line">
                    <b>出品</b>
                    <span v-for="(part, idx) in highlightedParts(item.recommendedUse, item)" :key="`pu-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                  </div>
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
                    <p v-if="item.recommendedUse" class="pdf-meta-line">
                      <b>出品建议</b>
                      <span v-for="(part, idx) in highlightedParts(item.recommendedUse, item)" :key="`cu-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                    </p>
                  </div>
                </div>
                <p v-if="item.flavor" class="pdf-flavor">
                  <b>风味</b>
                  <span>
                    <span v-for="(part, idx) in highlightedParts(item.flavor, item)" :key="`cf-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                  </span>
                </p>
                <p v-if="item.description" class="pdf-desc">
                  <b>特点</b>
                  <span>
                    <span v-for="(part, idx) in highlightedParts(item.description, item)" :key="`cd-print-${item.code}-${idx}`" :class="{ 'pdf-red': part.red }">{{ part.text }}</span>
                  </span>
                </p>
                <p v-for="line in item.attributeLines || []" :key="`pdf-attr-${item.code}-${line}`" class="pdf-meta-line"><b>属性</b><span>{{ line }}</span></p>
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
import CostingSettingsPanel from '../components/CostingSettingsPanel.vue'
import {
  DEFAULT_BEAN_LIST_PDF_VERSION,
  beanListPublicationPdfOptions,
  buildBeanListPdfGroups,
  buildBeanListPdfSubtitle,
  buildBeanListPdfTitle,
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
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'

const props = defineProps({
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const loading = ref(false)
const beanListPublishing = ref(false)
const beanListWithdrawing = ref(false)
const beanListVersionListLoading = ref(false)
const beanListPdfGenerating = ref(false)
const settingsOpen = ref(false)
const priceExplanationOpen = ref(false)
const priceExplanationLoading = ref(false)
const pdfDrawerOpen = ref(false)
const pdfPrinting = ref(false)
const versionListScope = ref('official')
const publicationScope = ref('official')
const selectedBeanListCustomerID = ref(0)
const actorLoaded = ref(false)
const currentActor = ref(null)
const selectedPriceSourcePublicationID = ref('')
const selectedProductTypeCategoryID = ref(0)
const downloadSourcePublication = ref(null)
const error = ref('')
const message = ref('')
const priceExplanationError = ref('')
const parameters = ref(null)
const items = ref([])
const priceExplanation = ref(null)
const explanationItem = ref(null)
const explanationTier = ref(null)
const explanationMode = ref('commercial')
const explanationOverrides = ref({
  green_bean_cost_per_kg: '',
  yield_rate: '',
  margin_rate: '',
})
const customers = ref([])
const customerPublicUsages = ref([])
const beanListPublications = ref({
  official: {},
  mine: {},
  customer: {},
})
const beanListPreviewCollapsed = ref({
  commercial: false,
  drip: true,
  retail: true,
  green: true,
})
const priceSourcePublicationByType = ref({})
const styleSourcePublicationIDByType = ref({})
const selectedProductIDsByType = ref({})
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

const normalizedCustomerContextID = computed(() => Number(props.customerContextId || 0))
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && normalizedCustomerContextID.value > 0)
const activeBeanListCustomerID = computed(() => normalizedCustomerContextID.value || versionListScopeCustomerID(versionListScope.value) || Number(selectedBeanListCustomerID.value || 0))
const activeCostingScope = computed(() => {
  return activeBeanListCustomerID.value > 0 ? 'customer' : 'official'
})
const activeCustomerPublicUsage = computed(() => {
  const customerID = activeBeanListCustomerID.value
  return customerPublicUsages.value.find((row) => Number(row.customer_id || 0) === customerID) || {
    customer_id: customerID,
    use_public_categories: false,
  }
})
const visibleCostingItems = computed(() => filterBeanListItemsForPriceTableScope(items.value, activeCostingScope.value, activeBeanListCustomerID.value))
const customerScopedSkuCount = computed(() => {
  const customerID = Number(activeBeanListCustomerID.value || 0)
  if (!customerID || activeCostingScope.value !== 'customer') return visibleCostingItems.value.length
  return (Array.isArray(items.value) ? items.value : []).filter((item) => {
    return Number(item?.customer_id ?? item?.customerID ?? 0) === customerID
  }).length
})
const productPriceListTypeOptions = computed(() => buildProductPriceListTypeOptions(visibleCostingItems.value))
const selectedProductPriceListType = computed(() => {
  const selectedID = Number(selectedProductTypeCategoryID.value || 0)
  return productPriceListTypeOptions.value.find((type) => Number(type.id || 0) === selectedID) || productPriceListTypeOptions.value[0] || null
})
const activeProductTypeCategoryID = computed(() => Number(selectedProductPriceListType.value?.id || selectedProductTypeCategoryID.value || 0))
const selectedProductPriceListLabel = computed(() => selectedProductPriceListType.value?.label || beanListTypeLabel(pdfTheme.value.listType))
const activePriceListTypeKey = computed(() => productPriceListTypeKey(selectedProductPriceListType.value, pdfTheme.value.listType))
const productPriceListPreviewSections = computed(() => productPriceListTypeOptions.value.map((type, index) => {
  const listType = normalizeBeanListType(type.listType)
  const key = productPriceListTypeKey(type, listType)
  const groups = productGroupsForType(listType, type.id)
  const collapsed = Object.prototype.hasOwnProperty.call(beanListPreviewCollapsed.value, key)
    ? Boolean(beanListPreviewCollapsed.value[key])
    : index > 0
  return {
    key,
    id: Number(type.id || 0),
    label: type.label || beanListTypeLabel(listType),
    listType,
    metaKey: metaKeyForListType(listType),
    groups,
    itemCount: beanListGroupItemCount(groups),
    collapsed,
  }
}))
const pdfTheme = computed(() => sanitizeBeanListPdfTheme(pdfOptions.value))
const pdfAvailableItems = computed(() => beanListItemsForType(pdfTheme.value.listType, activeProductTypeCategoryID.value))
const pdfCategoryOptions = computed(() => beanListCategoryOptions(pdfTheme.value.listType, activeProductTypeCategoryID.value))
const pdfSelectedProductIDs = computed(() => selectedProductIDsByType.value[activePriceListTypeKey.value] || [])
const pdfVisibleCategoryCodes = computed(() => visibleCategoryCodesByType.value[activePriceListTypeKey.value] || [])
const categoryProductGroups = computed(() => productGroupsForType(pdfTheme.value.listType, activeProductTypeCategoryID.value))
const pdfGenerationOptions = computed(() => ({
  selectedProductIDs: pdfSelectedProductIDs.value,
  showCategoryNumbers: pdfOptions.value.showCategoryNumbers,
  visibleCategoryCodes: pdfVisibleCategoryCodes.value,
  customizers: pdfCustomizers.value,
}))
const currentPriceSourcePublication = computed(() => (publicationScope.value === 'mine' || publicationScope.value === 'customer' ? priceSourcePublicationByType.value[activePriceListTypeKey.value] : null))
const pdfContentSourcePublication = computed(() => downloadSourcePublication.value || currentPriceSourcePublication.value)
const pdfGroups = computed(() => {
  if (pdfContentSourcePublication.value?.content?.groups) {
    return copyBeanListPublicationContentGroups(pdfContentSourcePublication.value, {
      listType: pdfTheme.value.listType,
      customizers: pdfCustomizers.value,
    })
  }
  return buildBeanListPdfGroups(pdfAvailableItems.value, pdfTheme.value.listType, pdfGenerationOptions.value)
})
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
const currentScopePublicationRows = computed(() => publicationRows(versionListScope.value, pdfTheme.value.listType, activeProductTypeCategoryID.value))
const versionListCurrentPublication = computed(() => currentScopePublicationRows.value.find((row) => row.status === 'published') || null)
const publicationScopeRows = computed(() => publicationRows(publicationScope.value, pdfTheme.value.listType, activeProductTypeCategoryID.value))
const currentBeanListPublication = computed(() => publicationScopeRows.value.find((row) => row.status === 'published') || null)
const officialPriceSourcePublications = computed(() => publicationRows('official', pdfTheme.value.listType, activeProductTypeCategoryID.value).filter((row) => row.status === 'published'))
const selectedPriceSourcePublication = computed(() => officialPriceSourcePublications.value.find((row) => String(row.id) === String(selectedPriceSourcePublicationID.value)) || null)
const currentPublicationOwnerLabel = computed(() => publicationScopeLabel(publicationScope.value))
const currentPublicationScopeDescription = computed(() => {
  if (publicationScope.value === 'customer') return '生成和发布会保存到当前豆单范围对应的履约客户。'
  if (publicationScope.value === 'mine') return '当前客户账号保存自己的豆单修改。'
  return '生成和发布会保存到公共豆单。'
})
const publicBeanListURL = computed(() => {
  if (publicationScope.value !== 'official' || !currentBeanListPublication.value) return ''
  const params = new URLSearchParams()
  const productTypeCategoryID = Number(currentBeanListPublication.value?.product_type_category_id || activeProductTypeCategoryID.value || 0)
  if (productTypeCategoryID > 0) params.set('product_type_category_id', String(productTypeCategoryID))
  const query = params.toString()
  return `${window.location.origin}/public/bean-list/${pdfTheme.value.listType}${query ? `?${query}` : ''}`
})
const inactiveBomWarningCount = computed(() => visibleCostingItems.value.filter((item) => itemWarnings(item).length).length)
const isDripExplanation = computed(() => explanationMode.value === 'drip')
const pdfPageStyle = computed(() => {
  const bg = pdfTheme.value.backgroundImage
  return {
    color: pdfTheme.value.fontColor,
    backgroundColor: pdfTheme.value.backgroundColor,
    backgroundImage: bg ? `linear-gradient(rgba(255,255,255,.74), rgba(255,255,255,.74)), url(${bg})` : 'none',
  }
})

watch(productPriceListTypeOptions, (options) => {
  if (!options.length) {
    selectedProductTypeCategoryID.value = 0
    return
  }
  if (!options.some((type) => Number(type.id || 0) === Number(selectedProductTypeCategoryID.value || 0))) {
    selectedProductTypeCategoryID.value = Number(options[0].id || 0)
    return
  }
  syncPdfListTypeFromSelectedProductType()
}, { immediate: true })

watch(selectedProductTypeCategoryID, () => {
  selectedPriceSourcePublicationID.value = ''
  syncPdfListTypeFromSelectedProductType()
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  loadBeanListPublications(pdfTheme.value.listType, versionListScope.value, activeProductTypeCategoryID.value)
  loadBeanListPublications(pdfTheme.value.listType, 'official', activeProductTypeCategoryID.value)
  loadBeanListPublications(pdfTheme.value.listType, 'mine', activeProductTypeCategoryID.value)
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(pdfTheme.value.listType, 'customer', activeProductTypeCategoryID.value)
  }
})

watch(() => pdfOptions.value.listType, (listType) => {
  selectedPriceSourcePublicationID.value = ''
  initializePdfDefaultsForType(listType, activeProductTypeCategoryID.value)
  loadBeanListPublications(listType, versionListScope.value, activeProductTypeCategoryID.value)
  loadBeanListPublications(listType, 'official', activeProductTypeCategoryID.value)
  loadBeanListPublications(listType, 'mine', activeProductTypeCategoryID.value)
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(listType, 'customer', activeProductTypeCategoryID.value)
  }
})

watch(versionListScope, (scope) => {
  syncPublicationScopeFromPageContext()
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  loadBeanList()
  loadBeanListPublications(pdfTheme.value.listType, scope, activeProductTypeCategoryID.value)
})

watch(activeBeanListCustomerID, () => {
  loadBeanList()
})

watch(publicationScope, (scope) => {
  loadBeanListPublications(pdfTheme.value.listType, scope, activeProductTypeCategoryID.value)
  initializePdfDefaultsForType(pdfTheme.value.listType, activeProductTypeCategoryID.value)
})

watch(selectedBeanListCustomerID, () => {
  beanListPublications.value = {
    ...beanListPublications.value,
    customer: {},
  }
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(pdfTheme.value.listType, 'customer', activeProductTypeCategoryID.value)
  }
  notifyWorkspaceCustomerChanged(selectedBeanListCustomerID.value)
})

watch(isBeanListAdmin, (canPublish) => {
  void canPublish
  syncPublicationScopeFromPageContext()
})

watch(() => props.customerContextId, syncPublicationScopeFromPageContext, { immediate: true })

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
    loadBeanListPublications(pdfTheme.value.listType, 'customer', activeProductTypeCategoryID.value)
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

function firstFiniteNumber(...values) {
  for (const value of values) {
    const n = Number(value)
    if (Number.isFinite(n)) return n
  }
  return 0
}

function dripDisplayTiers(item) {
  const tiers = Array.isArray(item?.drip_wholesale_tiers) ? item.drip_wholesale_tiers : []
  return tiers.flatMap((tier) => {
    if (tier?.sales_unit === 'box') {
      return [{
        ...tier,
        price_per_unit: firstFiniteNumber(tier.price_per_unit, tier.packed_price_per_box, 0),
        unit_bag_count: Number(tier.unit_bag_count || tier.box_bag_count || item?.drip_box_bag_count || 10),
      }]
    }
    const boxBagCount = Number(tier?.unit_bag_count || tier?.box_bag_count || item?.drip_box_bag_count || 10) || 10
    const bagPrice = firstFiniteNumber(tier?.price_per_unit, tier?.packed_price_per_bag, tier?.price_per_lb, 0)
    const rows = [{
      ...tier,
      sales_unit: 'bag',
      unit_bag_count: 1,
      price_per_unit: bagPrice,
    }]
    if (boxBagCount > 1) {
      rows.push({
        ...tier,
        sales_unit: 'box',
        unit_bag_count: boxBagCount,
        price_per_unit: firstFiniteNumber(tier?.packed_price_per_box, bagPrice * boxBagCount),
      })
    }
    return rows
  })
}

function dripTierUnit(tier) {
  if (tier?.sales_unit === 'box') return `盒(${Number(tier.unit_bag_count || 10)}袋)`
  return '袋'
}

function beanListGroupItemCount(groups) {
  return (Array.isArray(groups) ? groups : []).reduce((sum, group) => sum + (Array.isArray(group.items) ? group.items.length : 0), 0)
}

function toggleBeanListPreviewSection(section) {
  beanListPreviewCollapsed.value = {
    ...beanListPreviewCollapsed.value,
    [section]: !beanListPreviewCollapsed.value[section],
  }
}

function customerOptionLabel(customer) {
  return customer?.name || ''
}

function buildProductPriceListTypeOptions(sourceItems = []) {
  const seen = new Map()
  ;(Array.isArray(sourceItems) ? sourceItems : []).forEach((item) => {
    const id = productTypeCategoryIDOfItem(item)
    const listType = priceListRenderTypeForItem(item)
    const fallbackID = fallbackProductTypeID(listType)
    const key = id > 0 ? `product-type:${id}` : `legacy:${listType}`
    const label = productTypeNameOfItem(item) || beanListTypeLabel(listType)
    const current = seen.get(key) || {
      id: id > 0 ? id : fallbackID,
      categoryID: id,
      key,
      label,
      listType,
      position: productTypePositionOfItem(item),
      itemCount: 0,
    }
    current.itemCount += 1
    current.position = Math.min(current.position || 999999, productTypePositionOfItem(item))
    seen.set(key, current)
  })
  return Array.from(seen.values())
    .sort((a, b) => {
      const positionDelta = Number(a.position || 999999) - Number(b.position || 999999)
      if (positionDelta !== 0) return positionDelta
      return String(a.label || '').localeCompare(String(b.label || ''), 'zh-Hans-CN')
    })
}

function productTypeCategoryIDOfItem(item) {
  return Number(item?.product_type_category_id || item?.productTypeCategoryID || 0)
}

function productTypePositionOfItem(item) {
  return Number(item?.category_primary_position || item?.product_type_position || item?.productTypePosition || 999999)
}

function productTypeNameOfItem(item) {
  return String(item?.product_type_name || item?.productTypeName || item?.category_primary_name || item?.primary_name || '').trim()
}

function priceListRenderTypeForItem(item) {
  const subtypeName = String(item?.product_subtype_name || item?.productSubtypeName || '').trim()
  const typeName = String(item?.product_type_name || item?.productTypeName || '').trim()
  const kind = String(item?.product_kind || item?.productKind || '').trim().toLowerCase()

  // 按产品子类型/产品类型名称推断豆单渲染模式
  const categoryHint = (subtypeName + typeName).toLowerCase()
  if (categoryHint.includes('生豆') || categoryHint.includes('green')) return 'green'
  if (categoryHint.includes('挂耳') || categoryHint.includes('drip')) return 'drip'
  if (categoryHint.includes('零售') || categoryHint.includes('retail')) return 'retail'

  // fallback to product_kind for backward compatibility
  if (kind === 'green_bean') return 'green'
  if (kind === 'drip_bag') return 'drip'
  return 'commercial'
}

function fallbackProductTypeID(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return -2
  if (normalized === 'drip') return -3
  if (normalized === 'retail') return -4
  return -1
}

function productPriceListTypeKey(type, listType = 'commercial') {
  const id = Number(type?.categoryID || type?.id || 0)
  if (id > 0) return `product-type:${id}`
  return `legacy:${normalizeBeanListType(type?.listType || listType)}`
}

function beanListPublicationTypeKey(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
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
  if (label) return `${brand}${label}产品价格表`
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
  const warnings = Array.isArray(item?.warnings) ? item.warnings.filter(Boolean) : []
  if (item?.bom_status === 'missing_green_bean_template' && !warnings.some((warning) => String(warning).includes('未挂到带生豆模板的分类'))) {
    return ['未挂到带生豆模板的分类，无法生成生豆价格。请在 SKU设置 里把该生豆 SKU 移到带生豆模板的生豆分类。', ...warnings]
  }
  if (item?.bom_status === 'inactive' && !warnings.some((warning) => String(warning).includes('BOM已失效'))) {
    return ['BOM已失效：请重新启用 BOM 后再发布价格表', ...warnings]
  }
  return warnings
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
  return String(item?.product_id ?? item?.productID ?? item?.id ?? item?.name ?? '')
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

function beanListItemsForType(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  return scopedBeanListItems(publicationScope.value, listType)
    .filter((item) => matchesProductTypeCategory(item, productTypeCategoryID))
    .filter((item) => beanMetaForItem(item).code)
    .slice()
    .sort((a, b) => compareBeanCodes(beanMetaForItem(a).code, beanMetaForItem(b).code))
}

function customerBeanListItems(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  return scopedBeanListItems('customer', listType)
    .filter((item) => matchesProductTypeCategory(item, productTypeCategoryID))
    .filter((item) => beanMetaForItem(item).code)
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

function matchesProductTypeCategory(item, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const id = Number(productTypeCategoryID || 0)
  if (id <= 0) return true
  return productTypeCategoryIDOfItem(item) === id
}

function scopedBeanListItems(scope, listType) {
  void listType
  const customerID = selectedBeanListCustomerID.value
  return filterBeanListItemsForPriceTableScope(items.value, scope, customerID)
}

function beanListCategoryOptions(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const seen = new Map()
  beanListItemsForType(listType, productTypeCategoryID).forEach((item) => {
    const meta = beanMetaForItem(item)
    const code = String(meta.code || '').split('.')[0]
    if (!seen.has(code)) {
      seen.set(code, { code, label: meta.category || '未分类' })
    }
  })
  return Array.from(seen.values())
}

function productGroupsForType(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  return beanListCategoryOptions(listType, productTypeCategoryID).map((category) => ({
    ...category,
    items: beanListItemsForType(listType, productTypeCategoryID).filter((item) => categoryCodeOfItem(item, listType) === category.code),
  }))
}

function categoryCodeOfItem(item, listType = pdfTheme.value.listType) {
  return String(beanMetaForItem(item).code || '').split('.')[0]
}

function publicationRows(scope, listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const cacheKey = beanListPublicationCacheKey(scope)
  const typeKey = beanListPublicationTypeKey(listType, productTypeCategoryID)
  return beanListPublications.value?.[cacheKey]?.[typeKey] || []
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
  selectedProductIDsByType.value = {}
  visibleCategoryCodesByType.value = {}
  productSelectionInitialized.value = {}
  categorySelectionInitialized.value = {}
}

function initializePdfDefaultsForType(listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const cacheKey = beanListPublicationTypeKey(listType, productTypeCategoryID)
  const validIDs = beanListItemsForType(listType, productTypeCategoryID).map((item) => itemProductID(item))
  const validCategories = beanListCategoryOptions(listType, productTypeCategoryID).map((item) => item.code)
  if (!productSelectionInitialized.value[cacheKey]) {
    selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [cacheKey]: validIDs }
    productSelectionInitialized.value = { ...productSelectionInitialized.value, [cacheKey]: true }
  } else {
    const current = selectedProductIDsByType.value[cacheKey] || []
    selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [cacheKey]: current.filter((id) => validIDs.includes(id)) }
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
  loadBeanListPublications(resolvedListType, 'official', activeProductTypeCategoryID.value)
  loadBeanListPublications(resolvedListType, 'mine', activeProductTypeCategoryID.value)
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(resolvedListType, 'customer', activeProductTypeCategoryID.value)
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
  default:
    return row?.status || '未知'
  }
}

function beanListPublicationStatusClass(row) {
  const status = String(row?.status || '').trim()
  if (status === 'published') return 'status-published'
  if (status === 'draft') return 'status-draft'
  if (status === 'withdrawn') return 'status-withdrawn'
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
  if (scope === 'official') return '公共豆单'
  if (scope === 'mine') return '我的客户豆单'
  if (scope === 'customer') return '指定客户豆单'
  return '棵凡官方豆单'
}

function beanListPublicationOwnerLabel(row) {
  if (row?.owner_type === 'customer') {
    const customerID = Number(row.owner_key || 0)
    const customer = customers.value.find((item) => Number(item?.id || 0) === customerID)
    return customer ? customerOptionLabel(customer) : `客户 ${row.owner_key || '-'}`
  }
  if (row?.owner_type === 'actor') return '我的客户豆单'
  return '棵凡官方豆单'
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
  const productTypeCategoryID = activePublicationProductTypeCategoryID(row?.product_type_category_id || activeProductTypeCategoryID.value)
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
  const key = beanListPublicationTypeKey(listType, row.product_type_category_id || activeProductTypeCategoryID.value)
  downloadSourcePublication.value = null
  priceSourcePublicationByType.value = { ...priceSourcePublicationByType.value, [key]: row }
  selectedPriceSourcePublicationID.value = String(row.id)
  pdfOptions.value = { ...pdfOptions.value, listType, version: defaultBeanListVersionForScope(listType, row.product_type_category_id || activeProductTypeCategoryID.value) }
  message.value = `已复制${beanListPublicationLabel(row)}价格来源，发布后会锁定为客户豆单快照`
}

function beanListTypeLabel(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return '生豆'
  if (normalized === 'drip') return '挂耳'
  return normalized === 'retail' ? '零售' : '商用'
}

function beanListPublicationTypeLabel(row) {
  const productTypeName = String(row?.product_type_name || '').trim()
  if (productTypeName) return productTypeName
  const productTypeCategoryID = Number(row?.product_type_category_id || 0)
  if (productTypeCategoryID > 0) {
    const option = productPriceListTypeOptions.value.find((type) => Number(type.categoryID || type.id || 0) === productTypeCategoryID)
    if (option?.label) return option.label
  }
  return beanListTypeLabel(row?.list_type)
}

function selectProductTypeFromPublication(row) {
  const productTypeCategoryID = Number(row?.product_type_category_id || 0)
  if (productTypeCategoryID > 0) {
    selectedProductTypeCategoryID.value = productTypeCategoryID
    return
  }
  const listType = normalizeBeanListType(row?.list_type || pdfTheme.value.listType)
  const option = productPriceListTypeOptions.value.find((type) => type.listType === listType)
  if (option) selectedProductTypeCategoryID.value = Number(option.id || 0)
}

function isPdfProductSelected(id) {
  return pdfSelectedProductIDs.value.includes(String(id))
}

function togglePdfProduct(id, checked) {
  const key = activePriceListTypeKey.value
  const value = String(id)
  const current = selectedProductIDsByType.value[key] || []
  const next = checked ? Array.from(new Set([...current, value])) : current.filter((item) => item !== value)
  selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [key]: next }
  syncCategoryVisibilityFromSelectedProducts(pdfTheme.value.listType, next, activeProductTypeCategoryID.value)
}

function setAllPdfProducts(selected) {
  const key = activePriceListTypeKey.value
  const next = selected ? pdfAvailableItems.value.map((item) => itemProductID(item)) : []
  selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [key]: next }
  syncCategoryVisibilityFromSelectedProducts(pdfTheme.value.listType, next, activeProductTypeCategoryID.value)
}

function isPdfCategoryVisible(code) {
  return pdfVisibleCategoryCodes.value.includes(String(code))
}

function isPdfCategorySelected(code) {
  const ids = productIDsForCategory(code)
  return ids.length > 0 && ids.every((id) => pdfSelectedProductIDs.value.includes(id))
}

function selectedCountForCategory(code) {
  const ids = productIDsForCategory(code)
  return ids.filter((id) => pdfSelectedProductIDs.value.includes(id)).length
}

function productIDsForCategory(code, listType = pdfTheme.value.listType, productTypeCategoryID = activeProductTypeCategoryID.value) {
  return beanListItemsForType(listType, productTypeCategoryID)
    .filter((item) => categoryCodeOfItem(item, listType) === String(code))
    .map((item) => itemProductID(item))
}

function togglePdfCategoryProducts(code, checked) {
  const key = activePriceListTypeKey.value
  const categoryIDs = productIDsForCategory(code, pdfTheme.value.listType, activeProductTypeCategoryID.value)
  const current = selectedProductIDsByType.value[key] || []
  const next = checked
    ? Array.from(new Set([...current, ...categoryIDs]))
    : current.filter((id) => !categoryIDs.includes(id))
  selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [key]: next }
  syncCategoryVisibilityFromSelectedProducts(pdfTheme.value.listType, next, activeProductTypeCategoryID.value)
}

function setAllPdfCategories(selected) {
  setAllPdfProducts(selected)
}

function syncCategoryVisibilityFromSelectedProducts(listType, selectedIDs, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const key = beanListPublicationTypeKey(listType, productTypeCategoryID)
  const selectedSet = new Set(selectedIDs.map((id) => String(id)))
  const next = beanListCategoryOptions(listType, productTypeCategoryID)
    .filter((category) => productIDsForCategory(category.code, listType, productTypeCategoryID).some((id) => selectedSet.has(id)))
    .map((category) => category.code)
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
    const data = await apiGet(beanListURLForCustomerRules())
    parameters.value = data.parameters
    items.value = Array.isArray(data.items) ? data.items : []
    syncSelectedProductTypeCategoryFromOptions()
    initializePdfDefaults()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
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
  } catch (err) {
    customers.value = []
  }
}

async function loadCustomerPublicUsages() {
  try {
    const data = await apiGet('/api/product-settings')
    customerPublicUsages.value = (data.customer_public_usages || []).map((row) => ({
      customer_id: Number(row.customer_id || 0),
      use_public_categories: Boolean(row.use_public_categories),
    }))
  } catch (err) {
    customerPublicUsages.value = []
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

async function loadBeanListPublications(listType = pdfTheme.value.listType, scope = publicationScope.value, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const cacheKey = beanListPublicationCacheKey(scope)
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
    const data = await apiGet(beanListPublicationURL(listType, scope, productTypeCategoryID))
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

async function refreshBeanListVersionList() {
  beanListVersionListLoading.value = true
  error.value = ''
  try {
    await loadBeanListPublications(pdfTheme.value.listType, versionListScope.value, activeProductTypeCategoryID.value)
  } finally {
    beanListVersionListLoading.value = false
  }
}

function beanListPublicationURL(listType, scope, productTypeCategoryID = activeProductTypeCategoryID.value) {
  const requestScope = beanListPublicationRequestScope(scope)
  const params = new URLSearchParams({ list_type: listType, scope: requestScope })
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

function beanListPublicationCacheKey(scope = publicationScope.value) {
  return String(scope || 'official')
}

async function handleSettingSaved() {
  await loadBeanList()
  message.value = '成本参数已保存，豆单数据已刷新'
}

async function openPriceExplanation(item, tier) {
  explanationMode.value = 'commercial'
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

async function openDripPriceExplanation(item, tier) {
  explanationMode.value = 'drip'
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
    if (explanationMode.value === 'drip') {
      priceExplanation.value = await loadDripPriceExplanation({
        product: explanationItem.value,
        tier_label: explanationTier.value?.label || '',
      })
    } else {
      const payload = buildPriceExplanationRequest(
        explanationItem.value,
        explanationTier.value,
        cleanExplanationOverrides(),
      )
      priceExplanation.value = await apiSend('/api/costing/price-explanation', { body: payload })
    }
  } catch (err) {
    priceExplanation.value = null
    priceExplanationError.value = err.message || '加载价格来源失败'
  } finally {
    priceExplanationLoading.value = false
  }
}

async function loadDripPriceTemplates() {
  return apiGet('/api/drip-price-templates')
}

async function loadDripPriceExplanation(payload) {
  return apiSend('/api/costing/drip-price-explanation', { body: payload })
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
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID)
    await loadBeanListPublications(listType, versionListScope.value, productTypeCategoryID)
  } catch (err) {
    error.value = err.message || '生成价格表 PDF 失败'
  } finally {
    beanListPdfGenerating.value = false
  }
}

async function publishBeanList() {
  if (!pdfGroups.value.length) return
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
    const row = await apiSend('/api/costing/bean-list/publications', { body: beanListPublicationPayload() })
    message.value = publicationScope.value === 'official'
      ? `已发布${selectedProductPriceListLabel.value}产品价格表 ${row.version}，客户访问链接已生成`
      : `已发布${selectedProductPriceListLabel.value}客户产品价格表 ${row.version}，内容和价格已锁定为快照`
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID)
  } catch (err) {
    error.value = err.message || '发布价格表失败'
  } finally {
    beanListPublishing.value = false
  }
}

async function saveBeanListDraft() {
  if (!pdfGroups.value.length) return
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
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID)
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
    await loadBeanListPublications('green', publicationScope.value, activeProductTypeCategoryID.value)
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
  const productTypeCategoryID = activePublicationProductTypeCategoryID(activeProductTypeCategoryID.value)
  return {
    list_type: listType,
    product_type_category_id: productTypeCategoryID,
    product_type_name: selectedProductPriceListLabel.value,
    version: pdfTheme.value.version,
    scope: publicationScope.value,
    customer_id: Number(selectedBeanListCustomerID.value || 0),
    price_source_publication_id: Number(currentPriceSourcePublication.value?.id || 0),
    style_source_publication_id: Number(styleSourcePublicationIDByType.value[activePriceListTypeKey.value] || 0),
    source_version: currentPriceSourcePublication.value?.version || '',
    config: {
      ...pdfTheme.value,
      selectedProductIDs: pdfSelectedProductIDs.value,
      showCategoryNumbers: pdfOptions.value.showCategoryNumbers,
      visibleCategoryCodes: pdfVisibleCategoryCodes.value,
      customizers: pdfCustomizers.value,
    },
    content: {
      title: pdfTitle.value,
      subtitle: pdfSubtitle.value,
      totalItems: pdfTotalItems.value,
      groups: pdfGroups.value,
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
    await loadBeanListPublications(listType, publicationScope.value, productTypeCategoryID)
    await loadBeanListPublications(listType, versionListScope.value, productTypeCategoryID)
  } catch (err) {
    error.value = err.message || '撤回豆单失败'
  } finally {
    beanListWithdrawing.value = false
  }
}

function beanListWithdrawScopeParams(row) {
  if (row?.owner_type === 'customer') {
    return new URLSearchParams({ scope: 'customer', customer_id: String(row.owner_key || selectedBeanListCustomerID.value || 0) })
  }
  if (row?.owner_type === 'actor') {
    return new URLSearchParams({ scope: 'mine' })
  }
  const scope = publicationScope.value === 'mine' ? 'mine' : 'official'
  return new URLSearchParams({ scope })
}

onMounted(() => {
  loadCurrentActor()
  loadBeanList()
  loadCustomers()
  loadCustomerPublicUsages()
  loadBeanListPublications(pdfTheme.value.listType, 'official', activeProductTypeCategoryID.value)
  loadBeanListPublications(pdfTheme.value.listType, 'mine', activeProductTypeCategoryID.value)
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
.collapsible-bean-section { display: grid; gap: 10px; }
.collapsible-bean-head { display: flex; align-items: stretch; }
.section-toggle { width: 100%; min-height: 48px; display: flex; align-items: center; justify-content: space-between; gap: 12px; border: 1px solid #ddd; background: #fafafa; color: #111; text-align: left; }
.section-toggle span:first-child { display: grid; gap: 3px; min-width: 0; }
.section-toggle b { font-size: 15px; line-height: 1.25; }
.section-toggle small { color: #666; font-size: 12px; line-height: 1.25; }
.green-price-save-bar { display: flex; align-items: center; justify-content: space-between; gap: 10px; border: 1px solid #e7edf3; border-radius: 8px; background: #f7fbff; padding: 8px 10px; }
.green-price-save-bar p { margin: 0; line-height: 1.45; }
.bean-list-version-panel { display: grid; gap: 12px; }
.bean-list-version-head { align-items: flex-start; }
.bean-list-version-head p { margin: 4px 0 0; line-height: 1.45; }
.version-controls { display: grid; grid-template-columns: minmax(170px, .9fr) minmax(110px, .55fr) minmax(90px, .45fr); gap: 10px; align-items: end; }
.version-controls label span, .version-summary span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.version-controls select { width: 100%; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
.version-control-customer { grid-column: span 2; }
.version-summary { min-height: 38px; border: 1px solid #eee; border-radius: 8px; background: #fafafa; padding: 8px 10px; box-sizing: border-box; }
.version-summary strong { display: block; overflow-wrap: anywhere; font-size: 14px; line-height: 1.2; }
.version-table-wrap { overflow: auto; }
.version-table { min-width: 1040px; }
.version-table th, .version-table td { text-align: left; white-space: normal; vertical-align: top; }
.version-main strong { display: block; line-height: 1.25; }
.version-main small { display: block; margin-top: 3px; color: #777; font-size: 12px; }
.version-note { max-width: 260px; line-height: 1.45; color: #333; overflow-wrap: anywhere; }
.version-actions { display: flex; flex-wrap: wrap; gap: 6px; justify-content: flex-start; }
.status-pill { display: inline-flex; align-items: center; border-radius: 999px; border: 1px solid #d4d4d4; padding: 3px 8px; font-size: 12px; font-weight: 700; line-height: 1.2; white-space: nowrap; }
.status-published { border-color: #9fd0a4; background: #ecfaee; color: #176528; }
.status-draft { border-color: #c8d4e1; background: #f7fbff; color: #1f4f82; }
.status-withdrawn { border-color: #e0b4b4; background: #fff1f1; color: #8b1e1e; }
.status-unknown { background: #f5f5f5; color: #555; }
.metrics { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
.metrics > div { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fafafa; }
.metrics span, .muted { color: #666; font-size: 12px; }
.metrics span { display: block; margin-bottom: 6px; }
.metrics strong { font-size: 18px; }
.bean-list-global-scope { display: flex; align-items: end; justify-content: space-between; gap: 12px; margin-top: 12px; padding-top: 12px; border-top: 1px solid #eee; }
.bean-list-global-scope span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.bean-list-global-scope strong { display: block; font-size: 18px; line-height: 1.2; }
.scope-select { min-width: min(280px, 100%); margin: 0; }
.scope-select select { width: 100%; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
.table-wrap { overflow: auto; margin-top: 10px; }
table { width: 100%; border-collapse: collapse; min-width: 1100px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 9px 10px; text-align: right; white-space: nowrap; }
th:first-child, td:first-child { text-align: left; }
th { color: #555; background: #fafafa; font-weight: 700; }
.name { font-weight: 650; }
.item-warning-list, .bean-warning-list { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 6px; }
.warning-chip { display: inline-flex; align-items: center; max-width: 100%; border: 1px solid #e8c28f; border-radius: 999px; background: #fff8eb; color: #8a4b00; padding: 2px 7px; font-size: 12px; font-weight: 650; line-height: 1.35; white-space: normal; }
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
.pdf-form input, .pdf-form select, .pdf-form textarea, .product-picker-row input, .product-picker-row select { width: 100%; min-height: 38px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
.pdf-form textarea { resize: vertical; line-height: 1.45; }
.pdf-form input[type="color"] { padding: 4px; }
.pdf-form .wide, .pdf-actions { grid-column: 1 / -1; }
.pdf-actions { display: flex; justify-content: flex-end; gap: 8px; }
.check-line { display: flex; align-items: center; gap: 7px; color: #333; font-size: 12px; }
.check-line input { width: auto; min-height: auto; }
.pdf-picker { margin-top: 12px; border: 1px solid #e4e4e4; border-radius: 8px; background: #fff; padding: 10px; }
.picker-head { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.picker-actions { margin-left: auto; display: flex; gap: 6px; }
.checkbox-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px 10px; }
.product-picker-list { display: grid; gap: 10px; max-height: 420px; overflow: auto; }
.product-picker-category { display: grid; gap: 8px; border: 1px solid #ddd; border-radius: 8px; padding: 10px; background: #fff; }
.product-picker-category-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding-bottom: 6px; border-bottom: 1px solid #eee; }
.product-picker-row { display: grid; gap: 7px; border: 1px solid #eee; border-radius: 8px; padding: 9px; background: #fafafa; }
.customizer-row { display: grid; grid-template-columns: 120px minmax(0, 1fr); gap: 7px; }
.green-tier-price-editor { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 7px; }
.green-tier-price-editor label { display: grid; grid-template-columns: minmax(0, 1fr) 82px; align-items: center; gap: 6px; font-size: 12px; color: #555; }
.green-tier-price-editor input { min-width: 0; }
.green-inline-price-editor label { display: grid; grid-template-columns: minmax(82px, 110px) auto; align-items: center; gap: 5px; margin: 0; }
.green-inline-price-editor input { width: 100%; min-width: 0; min-height: 32px; border: 1px solid #ddd; border-radius: 7px; padding: 5px 7px; font: inherit; text-align: right; box-sizing: border-box; }
.green-inline-price-editor small { color: #666; font-size: 12px; white-space: nowrap; }
.pdf-preview-title { max-width: 760px; margin: 16px auto 8px; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; color: #555; font-size: 12px; }
.pdf-preview-title strong { color: #111; font-size: 14px; }
.pdf-preview-phone { max-width: 430px; min-height: 360px; max-height: 72vh; overflow: auto; margin: 0 auto; border: 1px solid #ded6c9; border-radius: 8px; box-shadow: 0 10px 28px rgba(0,0,0,.12); }
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
.pdf-card-row { display: grid; gap: 9px; align-items: stretch; }
.pdf-card-row > .pdf-item { min-width: 0; height: 100%; }
.pdf-item { display: flex; flex-direction: column; break-inside: avoid; page-break-inside: avoid; border: 1px solid rgba(0,0,0,.16); border-radius: 8px; padding: 10px 10px 18px; margin-bottom: 0; background: rgba(255,255,255,.76); }
.pdf-item-head { display: grid; grid-template-columns: auto 1fr; gap: 8px; align-items: start; }
.pdf-item-head > span { min-width: 32px; height: 26px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid currentColor; border-radius: 6px; font-size: 12px; font-weight: 700; }
.pdf-item h3 { margin: 0; font-size: 20px; line-height: 1.18; letter-spacing: 0; }
.pdf-meta-line { margin: 4px 0 0; font-size: 12px; line-height: 1.45; white-space: pre-line; }
.pdf-flavor, .pdf-desc { display: grid; grid-template-columns: 44px 1fr; gap: 6px; margin: 7px 0 0; font-size: 12px; line-height: 1.45; }
.pdf-card-row.cards-2 .pdf-item-head, .pdf-card-row.cards-3 .pdf-item-head { min-height: 58px; }
.pdf-card-row.cards-2 .pdf-flavor, .pdf-card-row.cards-3 .pdf-flavor { min-height: 44px; }
.pdf-card-row.cards-2 .pdf-desc, .pdf-card-row.cards-3 .pdf-desc { min-height: 62px; }
.pdf-flavor { font-weight: 650; }
.pdf-desc { opacity: .82; }
.pdf-meta-line b, .pdf-flavor b, .pdf-desc b, .pdf-section-label { color: inherit; opacity: .62; font-weight: 650; }
.pdf-meta-line b { margin-right: 6px; }
.pdf-price-block { margin-top: auto; padding-top: 9px; }
.pdf-section-label { margin-bottom: 5px; font-size: 12px; }
.pdf-price-list { display: grid; grid-template-columns: 1fr; gap: 6px; }
.pdf-card-row.cards-1 .pdf-price-list { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.pdf-price { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 8px; min-height: 42px; border: 1px solid rgba(0,0,0,.12); border-radius: 6px; padding: 6px 7px; background: #dff5d9; font-size: 12px; }
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

@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: flex-start; flex-direction: column; }
  .actions { justify-content: flex-start; }
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .bean-list-global-scope { align-items: stretch; flex-direction: column; }
  .scope-select { width: 100%; }
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
  .checkbox-grid, .customizer-row { grid-template-columns: 1fr; }
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
