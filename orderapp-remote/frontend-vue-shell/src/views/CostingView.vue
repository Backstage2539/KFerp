<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>价格与豆单</h2>
        <div class="actions">
          <button class="secondary" type="button" :disabled="loading" @click="loadBeanList">刷新</button>
          <button class="secondary" type="button" @click="settingsOpen = true">参数设置</button>
          <button class="primary" type="button" :disabled="saving || loading || !visibleCostingItems.length" @click="createRun">保存试算</button>
          <button class="danger" type="button" :disabled="publishing || !runId" @click="publishRun">发布价格</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div v-if="inactiveBomWarningCount" class="warning-banner">BOM已失效：{{ inactiveBomWarningCount }} 款产品依赖的 BOM 已失效，发布价格策略或豆单前请先重新启用 BOM。</div>
      <div class="metrics">
        <div>
          <span>商品数</span>
          <strong>{{ visibleCostingItems.length }}</strong>
        </div>
        <div>
          <span>烘焙率</span>
          <strong>{{ percent(parameters?.roast_yield_rate) }}</strong>
        </div>
        <div>
          <span>kg/lb</span>
          <strong>{{ fixed(parameters?.kg_to_lb_factor, 3) }}</strong>
        </div>
        <div>
          <span>试算批次</span>
          <strong>{{ runId || '-' }}</strong>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-bar">
        <div class="section-title">价格试算</div>
        <button class="secondary compact" type="button" @click="pricingCollapsed = !pricingCollapsed">
          {{ pricingCollapsed ? '展开' : '收起' }}
        </button>
      </div>
      <div v-show="!pricingCollapsed" class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>生豆/kg</th>
              <th>熟豆成本/kg</th>
              <th>商用批发梯度</th>
              <th>零售豆单价</th>
              <th>挂耳/袋</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in visibleCostingItems" :key="item.product_id || item.name">
              <td class="name">
                <div>{{ item.name }}</div>
                <div v-if="itemWarnings(item).length" class="item-warning-list">
                  <span v-for="warning in itemWarnings(item)" :key="warning" class="warning-chip">{{ warning }}</span>
                </div>
              </td>
              <td>{{ costMoney(item.green_bean_cost_per_kg) }}</td>
              <td>{{ costMoney(item.small_batch_cost_per_kg) }}</td>
              <td class="tiers-cell">
                <div class="tier-list">
                  <span v-for="tier in item.commercial_wholesale_tiers || []" :key="tier.label" class="tier-chip">
                    <b>{{ tier.label }}</b>{{ price(tierPriceValue(tier)) }}/{{ tierUnit(tier) }}
                    <button v-if="item.gradient_template" class="source-button" type="button" @click="openPriceExplanation(item, tier)">来源</button>
                  </span>
                </div>
              </td>
              <td class="tiers-cell">
                <div class="tier-list">
                  <span v-for="tier in item.retail_bean_tiers || []" :key="tier.label" class="tier-chip">
                    <b>{{ tier.label }}</b>{{ price(tier.price_per_unit) }}
                  </span>
                </div>
              </td>
              <td>{{ price(first(item.wholesale_drip_bag_prices)) }}</td>
            </tr>
            <tr v-if="!loading && !visibleCostingItems.length">
              <td colspan="6" class="muted empty">暂无可试算商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="bean-list-generate-bar">
        <div>
          <div class="section-title">商用批发豆单</div>
          <p class="muted">生成豆单前可选择产品、分级、样式、标签和标红内容。</p>
        </div>
        <button class="primary" type="button" :disabled="loading || !visibleCostingItems.length" @click="openBeanListDrawer('commercial')">生成豆单</button>
      </div>
      <div class="bean-groups">
        <section v-for="group in commercialGroups" :key="group.category" class="bean-group">
          <h3>{{ group.category }}</h3>
          <div class="bean-grid">
            <article v-for="item in group.items" :key="item.product_id || item.name">
              <div class="bean-heading">
                <span class="bean-code">{{ beanMeta(item, 'commercial_bean_list').code }}</span>
                <div>
                  <div class="bean-title">{{ beanName(item, 'commercial_bean_list') }}</div>
                  <div v-if="beanMeta(item, 'commercial_bean_list').recommended_use" class="bean-use">
                    {{ beanMeta(item, 'commercial_bean_list').recommended_use }}
                  </div>
                </div>
              </div>
              <div v-if="itemWarnings(item).length" class="bean-warning-list">
                <span v-for="warning in itemWarnings(item)" :key="`commercial-warning-${item.product_id || item.name}-${warning}`" class="warning-chip">{{ warning }}</span>
              </div>
              <div v-if="beanFlavor(item, 'commercial_bean_list')" class="bean-note">{{ beanFlavor(item, 'commercial_bean_list') }}</div>
              <div v-if="beanDescription(item, 'commercial_bean_list')" class="bean-desc">{{ beanDescription(item, 'commercial_bean_list') }}</div>
              <div class="bean-row" v-for="tier in item.commercial_wholesale_tiers || []" :key="tier.label">
                <span>{{ tier.label }}</span>
                <strong>
                  {{ price(tierPriceValue(tier)) }}/{{ tierUnit(tier) }}
                  <button v-if="item.gradient_template" class="source-button" type="button" @click="openPriceExplanation(item, tier)">来源</button>
                </strong>
              </div>
            </article>
          </div>
        </section>
        <div v-if="!commercialGroups.length" class="muted empty-card">暂无豆单数据</div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">零售豆单</div>
      <div class="bean-groups">
        <section v-for="group in retailGroups" :key="group.category" class="bean-group">
          <h3>{{ group.category }}</h3>
          <div class="bean-grid">
            <article v-for="item in group.items" :key="`retail-${item.product_id || item.name}`">
              <div class="bean-heading">
                <span class="bean-code">{{ beanMeta(item, 'retail_bean_list').code }}</span>
                <div>
                  <div class="bean-title">{{ beanName(item, 'retail_bean_list') }}</div>
                  <div v-if="beanMeta(item, 'retail_bean_list').recommended_use" class="bean-use">
                    {{ beanMeta(item, 'retail_bean_list').recommended_use }}
                  </div>
                </div>
              </div>
              <div v-if="itemWarnings(item).length" class="bean-warning-list">
                <span v-for="warning in itemWarnings(item)" :key="`retail-warning-${item.product_id || item.name}-${warning}`" class="warning-chip">{{ warning }}</span>
              </div>
              <div v-if="beanFlavor(item, 'retail_bean_list')" class="bean-note">{{ beanFlavor(item, 'retail_bean_list') }}</div>
              <div v-if="beanDescription(item, 'retail_bean_list')" class="bean-desc">{{ beanDescription(item, 'retail_bean_list') }}</div>
              <div class="bean-row" v-for="tier in item.retail_bean_tiers || []" :key="`retail-tier-${tier.label}`">
                <span>{{ tier.label }}</span><strong>{{ price(tier.price_per_unit) }}</strong>
              </div>
              <div class="bean-row"><span>挂耳10袋</span><strong>{{ price(item.retail_drip_10_bag_price) }}</strong></div>
            </article>
          </div>
        </section>
        <div v-if="!retailGroups.length" class="muted empty-card">暂无豆单数据</div>
      </div>
    </section>

    <div v-if="settingsOpen" class="drawer-backdrop" @click.self="settingsOpen = false">
      <aside class="settings-drawer" aria-label="快速成本参数设置">
        <div class="drawer-head">
          <div>
            <h3>快速成本参数设置</h3>
            <p>保存单个参数后，当前成本试算会自动刷新。</p>
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
          <div>
            <span>当前试算</span>
            <strong>{{ price(priceExplanation.saved_final_price) }}/{{ gradientDisplayUnitLabel(priceExplanation.display_unit).replace('元/', '') }}</strong>
          </div>
          <div>
            <span>临时试算</span>
            <strong>{{ price(priceExplanation.preview_final_price) }}/{{ gradientDisplayUnitLabel(priceExplanation.display_unit).replace('元/', '') }}</strong>
          </div>
        </div>
        <div class="explanation-form">
          <label>
            <span>临时生豆成本 元/kg</span>
            <input v-model="explanationOverrides.green_bean_cost_per_kg" type="number" min="0" step="0.01" placeholder="不改" />
          </label>
          <label>
            <span>临时出成率</span>
            <input v-model="explanationOverrides.yield_rate" type="number" min="0" max="1" step="0.001" placeholder="不改" />
          </label>
          <label>
            <span>临时利润率</span>
            <input v-model="explanationOverrides.margin_rate" type="number" min="0" step="0.001" placeholder="不改" />
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
        <p class="muted">这里的参数只做临时试算；保存请回到快速成本参数或梯度模板，交易价格仍需发布后生效。</p>
      </aside>
    </div>

    <div v-if="pdfDrawerOpen" class="drawer-backdrop" @click.self="pdfDrawerOpen = false">
      <aside class="settings-drawer pdf-drawer" aria-label="生成豆单 PDF">
        <div class="drawer-head">
          <div>
            <h3>生成豆单</h3>
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

        <div class="copy-config-box">
          <div>
            <strong>发布归属</strong>
            <p>官方豆单用于棵凡公开链接；我的客户豆单发布后锁定快照，不会跟随官方自动变价。</p>
          </div>
          <div class="copy-config-actions">
            <select v-model="publicationScope" :disabled="!isBeanListAdmin">
              <option value="official">棵凡官方豆单</option>
              <option value="mine">我的客户豆单</option>
              <option value="customer">指定客户豆单</option>
            </select>
          </div>
          <p v-if="actorLoaded && !isBeanListAdmin" class="muted">客户账号只能保存修改和下载豆单，发布由管理员执行。</p>
        </div>

        <div class="copy-config-box" v-if="publicationScope === 'customer'">
          <div>
            <strong>客户</strong>
            <p>客户专属 SKU 只会出现在对应客户的豆单里；如果其他客户需要同款 SKU，请先复制一份给该客户。</p>
          </div>
          <div class="copy-config-actions">
            <SearchableSelect
              v-model="selectedBeanListCustomerID"
              :options="customers"
              :option-label="customerOptionLabel"
              :option-meta="customerOptionMeta"
              :option-value="optionNumericValue"
              placeholder="选择客户"
              empty-text="没有匹配客户" />
          </div>
        </div>

        <div class="copy-config-box" v-if="(publicationScope === 'mine' || publicationScope === 'customer') && officialPriceSourcePublications.length">
          <div>
            <strong>复制官方价格来源</strong>
            <p>复制棵凡已发布豆单的报价、风味、特点和出品建议，作为本次客户豆单的锁定内容快照。</p>
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

        <div class="copy-config-box" v-if="copyableBeanListPublications.length">
          <div>
            <strong>复制已有豆单配置</strong>
            <p>选择历史发布记录，只复制样式、选择、标签和标红词；客户豆单可配合官方价格来源生成独立快照。</p>
          </div>
          <div class="copy-config-actions">
            <select v-model="selectedCopyPublicationID">
              <option value="">选择历史豆单</option>
              <option v-for="row in copyableBeanListPublications" :key="`copy-pub-${row.id}`" :value="String(row.id)">
                {{ beanListPublicationLabel(row) }}
              </option>
            </select>
            <button class="secondary" type="button" :disabled="!selectedCopyPublication" @click="applyCopiedBeanListPublicationConfig()">复制配置</button>
          </div>
        </div>

        <div class="pdf-form">
          <label>
            <span>豆单类型</span>
            <select v-model="pdfOptions.listType">
              <option value="commercial">商用批发豆单</option>
              <option value="retail">零售豆单</option>
              <option value="green">生豆豆单</option>
            </select>
          </label>
          <label>
            <span>版本号</span>
            <input v-model.trim="pdfOptions.version" placeholder="V3.0.5" />
          </label>
          <label>
            <span>品牌名字</span>
            <input v-model.trim="pdfOptions.brandName" placeholder="棵凡咖啡" />
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
              </article>
            </section>
          </div>
        </div>

        <div class="pdf-preview-title">
          <strong>预览</strong>
          <span>{{ pdfTotalItems }} 款</span>
          <div class="pdf-actions">
            <button v-if="isBeanListAdmin" class="secondary" type="button" :disabled="beanListWithdrawing || !currentBeanListPublication" @click="withdrawBeanList">撤回发布</button>
            <button v-if="isBeanListAdmin" class="primary" type="button" :disabled="beanListPublishing || !pdfGroups.length || !pdfTheme.version || !customerScopeReady" @click="publishBeanList">发布豆单</button>
            <button v-else class="primary" type="button" :disabled="beanListPublishing || !pdfGroups.length || !pdfTheme.version || !customerScopeReady" @click="saveBeanListDraft">保存修改</button>
            <button class="secondary" type="button" :disabled="!pdfGroups.length" @click="generateBeanListPdf">生成 PDF</button>
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
            <div class="pdf-badge">{{ beanListTypeLabel(pdfTheme.listType) }}</div>
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
          <div class="pdf-badge">{{ beanListTypeLabel(pdfTheme.listType) }}</div>
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
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchCurrentActor } from '../api/auth'
import { apiGet, apiSend } from '../api/client'
import CostingSettingsPanel from '../components/CostingSettingsPanel.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import {
  DEFAULT_BEAN_LIST_PDF_VERSION,
  buildBeanListPdfGroups,
  buildBeanListPdfSubtitle,
  buildBeanListPdfTitle,
  copyBeanListPublicationContentGroups,
  copyBeanListPublicationConfig,
  filterBeanListItemsForScope,
  sanitizeBeanListPdfTheme,
  splitHighlightedText,
} from '../lib/bean-list-pdf'
import {
  buildPriceExplanationRequest,
  gradientDisplayUnitLabel,
} from '../lib/gradient-templates'

const props = defineProps({
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const loading = ref(false)
const saving = ref(false)
const publishing = ref(false)
const beanListPublishing = ref(false)
const beanListWithdrawing = ref(false)
const settingsOpen = ref(false)
const priceExplanationOpen = ref(false)
const priceExplanationLoading = ref(false)
const pdfDrawerOpen = ref(false)
const pdfPrinting = ref(false)
const pricingCollapsed = ref(true)
const publicationScope = ref('official')
const selectedBeanListCustomerID = ref(0)
const actorLoaded = ref(false)
const currentActor = ref(null)
const selectedCopyPublicationID = ref('')
const selectedPriceSourcePublicationID = ref('')
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
const runId = ref(null)
const beanListPublications = ref({
  official: { commercial: [], retail: [], green: [] },
  mine: { commercial: [], retail: [], green: [] },
  customer: { commercial: [], retail: [], green: [] },
})
const priceSourcePublicationByType = ref({ commercial: null, retail: null, green: null })
const styleSourcePublicationIDByType = ref({ commercial: 0, retail: 0, green: 0 })
const selectedProductIDsByType = ref({ commercial: [], retail: [], green: [] })
const visibleCategoryCodesByType = ref({ commercial: [], retail: [], green: [] })
const productSelectionInitialized = ref({ commercial: false, retail: false, green: false })
const categorySelectionInitialized = ref({ commercial: false, retail: false, green: false })
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
const activeCostingScope = computed(() => normalizedCustomerContextID.value > 0 ? 'customer' : 'official')
const visibleCostingItems = computed(() => filterBeanListItemsForScope(items.value, activeCostingScope.value, normalizedCustomerContextID.value))
const commercialGroups = computed(() => groupBeanItems('commercial_bean_list'))
const retailGroups = computed(() => groupBeanItems('retail_bean_list'))
const pdfTheme = computed(() => sanitizeBeanListPdfTheme(pdfOptions.value))
const pdfAvailableItems = computed(() => beanListItemsForType(pdfTheme.value.listType))
const pdfCategoryOptions = computed(() => beanListCategoryOptions(pdfTheme.value.listType))
const pdfSelectedProductIDs = computed(() => selectedProductIDsByType.value[pdfTheme.value.listType] || [])
const pdfVisibleCategoryCodes = computed(() => visibleCategoryCodesByType.value[pdfTheme.value.listType] || [])
const categoryProductGroups = computed(() => productGroupsForType(pdfTheme.value.listType))
const pdfGenerationOptions = computed(() => ({
  selectedProductIDs: pdfSelectedProductIDs.value,
  showCategoryNumbers: pdfOptions.value.showCategoryNumbers,
  visibleCategoryCodes: pdfVisibleCategoryCodes.value,
  customizers: pdfCustomizers.value,
}))
const currentPriceSourcePublication = computed(() => (publicationScope.value === 'mine' || publicationScope.value === 'customer' ? priceSourcePublicationByType.value[pdfTheme.value.listType] : null))
const pdfGroups = computed(() => {
  if (currentPriceSourcePublication.value?.content?.groups) {
    return copyBeanListPublicationContentGroups(currentPriceSourcePublication.value)
  }
  return buildBeanListPdfGroups(pdfAvailableItems.value, pdfTheme.value.listType, pdfGenerationOptions.value)
})
const pdfTotalItems = computed(() => pdfGroups.value.reduce((sum, group) => sum + group.items.length, 0))
const pdfTitle = computed(() => buildBeanListPdfTitle(pdfTheme.value.listType, pdfTheme.value.brandName))
const pdfSubtitle = computed(() => buildBeanListPdfSubtitle(pdfTheme.value.listType))
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
const currentScopePublicationRows = computed(() => publicationRows(publicationScope.value, pdfTheme.value.listType))
const currentBeanListPublication = computed(() => currentScopePublicationRows.value.find((row) => row.status === 'published') || null)
const copyableBeanListPublications = computed(() => currentScopePublicationRows.value)
const officialPriceSourcePublications = computed(() => publicationRows('official', pdfTheme.value.listType).filter((row) => row.status === 'published'))
const selectedCopyPublication = computed(() => copyableBeanListPublications.value.find((row) => String(row.id) === String(selectedCopyPublicationID.value)) || null)
const selectedPriceSourcePublication = computed(() => officialPriceSourcePublications.value.find((row) => String(row.id) === String(selectedPriceSourcePublicationID.value)) || null)
const publicBeanListURL = computed(() => {
  if (publicationScope.value !== 'official' || !currentBeanListPublication.value) return ''
  return `${window.location.origin}/public/bean-list/${pdfTheme.value.listType}`
})
const inactiveBomWarningCount = computed(() => visibleCostingItems.value.filter((item) => itemWarnings(item).length).length)
const pdfPageStyle = computed(() => {
  const bg = pdfTheme.value.backgroundImage
  return {
    color: pdfTheme.value.fontColor,
    backgroundColor: pdfTheme.value.backgroundColor,
    backgroundImage: bg ? `linear-gradient(rgba(255,255,255,.74), rgba(255,255,255,.74)), url(${bg})` : 'none',
  }
})

watch(() => pdfOptions.value.listType, (listType) => {
  selectedCopyPublicationID.value = ''
  selectedPriceSourcePublicationID.value = ''
  initializePdfDefaultsForType(listType)
  loadBeanListPublications(listType, 'official')
  loadBeanListPublications(listType, 'mine')
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(listType, 'customer')
  }
})

watch(publicationScope, (scope) => {
  if (actorLoaded.value && !isBeanListAdmin.value && scope !== 'mine') {
    publicationScope.value = 'mine'
    return
  }
  selectedCopyPublicationID.value = ''
  loadBeanListPublications(pdfTheme.value.listType, scope)
  initializePdfDefaultsForType(pdfTheme.value.listType)
})

watch(selectedBeanListCustomerID, () => {
  beanListPublications.value = {
    ...beanListPublications.value,
    customer: { commercial: [], retail: [], green: [] },
  }
  selectedCopyPublicationID.value = ''
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(pdfTheme.value.listType, 'customer')
  }
})

watch(isBeanListAdmin, (canPublish) => {
  if (actorLoaded.value && !canPublish && publicationScope.value !== 'mine') {
    publicationScope.value = 'mine'
    return
  }
  syncCustomerContext()
})

watch(() => props.customerContextId, syncCustomerContext, { immediate: true })

function syncCustomerContext() {
  const normalizedCustomerID = Number(props.customerContextId || 0)
  if (normalizedCustomerID > 0) {
    selectedBeanListCustomerID.value = normalizedCustomerID
    if (isBeanListAdmin.value) {
      publicationScope.value = 'customer'
    }
    resetPdfSelectionDefaults()
    initializePdfDefaultsIfItemsLoaded()
    if (publicationScope.value === 'customer') {
      loadBeanListPublications(pdfTheme.value.listType, 'customer')
    }
    return
  }
  if (publicationScope.value === 'customer') {
    publicationScope.value = isBeanListAdmin.value ? 'official' : 'mine'
  }
  selectedBeanListCustomerID.value = 0
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
}

function first(values) {
  return Array.isArray(values) && values.length ? Number(values[0] || 0) : 0
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
  if (item?.bom_status === 'inactive' && !warnings.some((warning) => String(warning).includes('BOM已失效'))) {
    return ['BOM已失效：请重新启用 BOM 后再发布价格策略', ...warnings]
  }
  return warnings
}

function itemProductID(item) {
  return String(item?.product_id ?? item?.productID ?? item?.id ?? item?.name ?? '')
}

function metaKeyForListType(listType) {
  if (listType === 'green') return 'green_bean_list'
  return listType === 'retail' ? 'retail_bean_list' : 'commercial_bean_list'
}

function tierKeyForListType(listType) {
  if (listType === 'green') return 'green_bean_sale_tiers'
  return listType === 'retail' ? 'retail_bean_tiers' : 'commercial_wholesale_tiers'
}

function beanListItemsForType(listType) {
  const key = metaKeyForListType(listType)
  return scopedBeanListItems(publicationScope.value, listType)
    .filter((item) => beanMeta(item, key).code)
    .slice()
    .sort((a, b) => compareBeanCodes(beanMeta(a, key).code, beanMeta(b, key).code))
}

function customerBeanListItems(listType) {
  const key = metaKeyForListType(listType)
  return scopedBeanListItems('customer', listType)
    .filter((item) => beanMeta(item, key).code)
    .slice()
    .sort((a, b) => compareBeanCodes(beanMeta(a, key).code, beanMeta(b, key).code))
}

function scopedBeanListItems(scope, listType) {
  void listType
  return filterBeanListItemsForScope(items.value, scope, selectedBeanListCustomerID.value)
}

function beanListCategoryOptions(listType) {
  const key = metaKeyForListType(listType)
  const seen = new Map()
  beanListItemsForType(listType).forEach((item) => {
    const meta = beanMeta(item, key)
    const code = String(meta.code || '').split('.')[0]
    if (!seen.has(code)) {
      seen.set(code, { code, label: meta.category || '未分类' })
    }
  })
  return Array.from(seen.values())
}

function productGroupsForType(listType) {
  return beanListCategoryOptions(listType).map((category) => ({
    ...category,
    items: beanListItemsForType(listType).filter((item) => categoryCodeOfItem(item, listType) === category.code),
  }))
}

function categoryCodeOfItem(item, listType = pdfTheme.value.listType) {
  return String(beanMeta(item, metaKeyForListType(listType)).code || '').split('.')[0]
}

function publicationRows(scope, listType) {
  return beanListPublications.value?.[scope]?.[listType] || []
}

function initializePdfDefaults() {
  initializePdfDefaultsForType('commercial')
  initializePdfDefaultsForType('retail')
  initializePdfDefaultsForType('green')
}

function initializePdfDefaultsIfItemsLoaded() {
  if (!items.value.length) return
  initializePdfDefaults()
}

function resetPdfSelectionDefaults() {
  selectedProductIDsByType.value = { commercial: [], retail: [], green: [] }
  visibleCategoryCodesByType.value = { commercial: [], retail: [], green: [] }
  productSelectionInitialized.value = { commercial: false, retail: false, green: false }
  categorySelectionInitialized.value = { commercial: false, retail: false, green: false }
}

function initializePdfDefaultsForType(listType) {
  const validIDs = beanListItemsForType(listType).map((item) => itemProductID(item))
  const validCategories = beanListCategoryOptions(listType).map((item) => item.code)
  if (!productSelectionInitialized.value[listType]) {
    selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [listType]: validIDs }
    productSelectionInitialized.value = { ...productSelectionInitialized.value, [listType]: true }
  } else {
    const current = selectedProductIDsByType.value[listType] || []
    selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [listType]: current.filter((id) => validIDs.includes(id)) }
  }
  if (!categorySelectionInitialized.value[listType]) {
    visibleCategoryCodesByType.value = { ...visibleCategoryCodesByType.value, [listType]: validCategories }
    categorySelectionInitialized.value = { ...categorySelectionInitialized.value, [listType]: true }
  } else {
    const current = visibleCategoryCodesByType.value[listType] || []
    visibleCategoryCodesByType.value = { ...visibleCategoryCodesByType.value, [listType]: current.filter((code) => validCategories.includes(code)) }
  }
}

function openBeanListDrawer(listType = 'commercial') {
  pdfOptions.value = { ...pdfOptions.value, listType }
  initializePdfDefaultsForType(listType)
  loadBeanListPublications(listType, 'official')
  loadBeanListPublications(listType, 'mine')
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(listType, 'customer')
  }
  pdfDrawerOpen.value = true
}

function beanListPublicationLabel(row) {
  const status = row?.status === 'published' ? '已发布' : '已撤回'
  const time = row?.published_at || row?.created_at || ''
  return [row?.version || '未命名版本', status, time].filter(Boolean).join(' · ')
}

function applyCopiedBeanListPublicationConfig(row = selectedCopyPublication.value) {
  if (!row) return
  const listType = normalizeBeanListType(row.list_type)
  const copied = copyBeanListPublicationConfig(row, pdfOptions.value, {
    productIDs: beanListItemsForType(listType).map((item) => itemProductID(item)),
    categoryCodes: beanListCategoryOptions(listType).map((item) => item.code),
  })
  pdfOptions.value = { ...pdfOptions.value, ...copied.options, listType }
  selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [listType]: copied.selectedProductIDs }
  visibleCategoryCodesByType.value = { ...visibleCategoryCodesByType.value, [listType]: copied.visibleCategoryCodes }
  productSelectionInitialized.value = { ...productSelectionInitialized.value, [listType]: true }
  categorySelectionInitialized.value = { ...categorySelectionInitialized.value, [listType]: true }
  pdfCustomizers.value = copied.customizers
  styleSourcePublicationIDByType.value = { ...styleSourcePublicationIDByType.value, [listType]: Number(row.id || 0) }
  message.value = `已复制${beanListPublicationLabel(row)}配置，可继续修改后生成`
}

function applyCopiedBeanListPriceSource(row = selectedPriceSourcePublication.value) {
  if (!row) return
  const listType = normalizeBeanListType(row.list_type)
  priceSourcePublicationByType.value = { ...priceSourcePublicationByType.value, [listType]: row }
  selectedPriceSourcePublicationID.value = String(row.id)
  message.value = `已复制${beanListPublicationLabel(row)}价格来源，发布后会锁定为客户豆单快照`
}

function normalizeBeanListType(listType) {
  if (listType === 'retail') return 'retail'
  if (listType === 'green' || listType === 'green_bean') return 'green'
  return 'commercial'
}

function beanListTypeLabel(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return '生豆'
  return normalized === 'retail' ? '零售' : '商用'
}

function isPdfProductSelected(id) {
  return pdfSelectedProductIDs.value.includes(String(id))
}

function togglePdfProduct(id, checked) {
  const key = pdfTheme.value.listType
  const value = String(id)
  const current = selectedProductIDsByType.value[key] || []
  const next = checked ? Array.from(new Set([...current, value])) : current.filter((item) => item !== value)
  selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [key]: next }
  syncCategoryVisibilityFromSelectedProducts(key, next)
}

function setAllPdfProducts(selected) {
  const key = pdfTheme.value.listType
  const next = selected ? pdfAvailableItems.value.map((item) => itemProductID(item)) : []
  selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [key]: next }
  syncCategoryVisibilityFromSelectedProducts(key, next)
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

function productIDsForCategory(code, listType = pdfTheme.value.listType) {
  return beanListItemsForType(listType)
    .filter((item) => categoryCodeOfItem(item, listType) === String(code))
    .map((item) => itemProductID(item))
}

function togglePdfCategoryProducts(code, checked) {
  const key = pdfTheme.value.listType
  const categoryIDs = productIDsForCategory(code, key)
  const current = selectedProductIDsByType.value[key] || []
  const next = checked
    ? Array.from(new Set([...current, ...categoryIDs]))
    : current.filter((id) => !categoryIDs.includes(id))
  selectedProductIDsByType.value = { ...selectedProductIDsByType.value, [key]: next }
  syncCategoryVisibilityFromSelectedProducts(key, next)
}

function setAllPdfCategories(selected) {
  setAllPdfProducts(selected)
}

function syncCategoryVisibilityFromSelectedProducts(listType, selectedIDs) {
  const selectedSet = new Set(selectedIDs.map((id) => String(id)))
  const next = beanListCategoryOptions(listType)
    .filter((category) => productIDsForCategory(category.code, listType).some((id) => selectedSet.has(id)))
    .map((category) => category.code)
  visibleCategoryCodesByType.value = { ...visibleCategoryCodesByType.value, [listType]: next }
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

function groupBeanItems(key) {
  const groups = new Map()
  visibleCostingItems.value
    .filter((item) => beanMeta(item, key).code)
    .slice()
    .sort((a, b) => compareBeanCodes(beanMeta(a, key).code, beanMeta(b, key).code))
    .forEach((item) => {
      const meta = beanMeta(item, key)
      const category = meta.category || '未分类'
      if (!groups.has(category)) {
        groups.set(category, { category, items: [] })
      }
      groups.get(category).items.push(item)
    })
  return Array.from(groups.values())
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

function money(value) {
  return fixed(value, 2)
}

function price(value) {
  return fixed(value, 0)
}

function costMoney(value) {
  return fixed(value, 2)
}

function percent(value) {
  return `${fixed(Number(value || 0) * 100, 1)}%`
}

async function loadBeanList() {
  loading.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiGet('/api/costing/bean-list')
    parameters.value = data.parameters
    items.value = Array.isArray(data.items) ? data.items : []
    initializePdfDefaults()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadCustomers() {
  try {
    const data = await apiGet('/api/customers?limit=200')
    customers.value = (data.rows || []).filter((row) => row.active !== false)
  } catch (err) {
    customers.value = []
  }
}

async function loadCurrentActor() {
  try {
    currentActor.value = await fetchCurrentActor()
  } catch (err) {
    currentActor.value = null
  } finally {
    actorLoaded.value = true
    if (!isBeanListAdmin.value) {
      publicationScope.value = 'mine'
    }
  }
}

async function loadBeanListPublications(listType = pdfTheme.value.listType, scope = publicationScope.value) {
  if (scope === 'customer' && !selectedBeanListCustomerID.value) {
    beanListPublications.value = {
      ...beanListPublications.value,
      customer: {
        ...(beanListPublications.value.customer || {}),
        [listType]: [],
      },
    }
    return
  }
  try {
    const data = await apiGet(beanListPublicationURL(listType, scope))
    const rows = Array.isArray(data.rows) ? data.rows : []
    beanListPublications.value = {
      ...beanListPublications.value,
      [scope]: {
        ...(beanListPublications.value[scope] || {}),
        [listType]: rows,
      },
    }
  } catch (err) {
    error.value = err.message || '加载豆单发布记录失败'
  }
}

function beanListPublicationURL(listType, scope) {
  const params = new URLSearchParams({ list_type: listType, scope })
  if (scope === 'customer') {
    params.set('customer_id', String(selectedBeanListCustomerID.value || 0))
  }
  return `/api/costing/bean-list/publications?${params.toString()}`
}

async function createRun() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend('/api/costing/runs')
    runId.value = data.id
    if (Array.isArray(data.items)) items.value = data.items
    message.value = `已保存试算批次 ${data.id}`
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function publishRun() {
  if (!runId.value) return
  publishing.value = true
  error.value = ''
  message.value = ''
  try {
    await apiSend(`/api/costing/runs/${runId.value}/publish`)
    message.value = `已发布试算批次 ${runId.value}`
  } catch (err) {
    error.value = err.message || '发布失败'
  } finally {
    publishing.value = false
  }
}

async function handleSettingSaved() {
  await loadBeanList()
  message.value = '成本参数已保存，当前试算已刷新'
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
  document.body.classList.remove('bean-list-pdf-printing')
}

function generateBeanListPdf() {
  if (!pdfGroups.value.length) return
  pdfPrinting.value = true
  document.body.classList.add('bean-list-pdf-printing')
  window.setTimeout(() => window.print(), 80)
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
  try {
    const row = await apiSend('/api/costing/bean-list/publications', { body: beanListPublicationPayload() })
    message.value = publicationScope.value === 'official'
      ? `已发布${beanListTypeLabel(listType)}豆单 ${row.version}，客户访问链接已生成`
      : `已发布${beanListTypeLabel(listType)}客户豆单 ${row.version}，内容和价格已锁定为快照`
    await loadBeanListPublications(listType, publicationScope.value)
  } catch (err) {
    error.value = err.message || '发布豆单失败'
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
  try {
    const row = await apiSend('/api/costing/bean-list/drafts', { body: beanListPublicationPayload() })
    message.value = `已保存${beanListTypeLabel(listType)}豆单修改 ${row.version}，可继续生成 PDF 下载`
    await loadBeanListPublications(listType, publicationScope.value)
  } catch (err) {
    error.value = err.message || '保存豆单修改失败'
  } finally {
    beanListPublishing.value = false
  }
}

function beanListPublicationPayload() {
  const listType = pdfTheme.value.listType
  return {
    list_type: listType,
    version: pdfTheme.value.version,
    scope: publicationScope.value,
    customer_id: Number(selectedBeanListCustomerID.value || 0),
    price_source_publication_id: Number(currentPriceSourcePublication.value?.id || 0),
    style_source_publication_id: Number(styleSourcePublicationIDByType.value[listType] || 0),
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

async function withdrawBeanList() {
  const row = currentBeanListPublication.value
  if (!row?.id) return
  beanListWithdrawing.value = true
  error.value = ''
  message.value = ''
  const listType = pdfTheme.value.listType
  try {
    const params = new URLSearchParams({ scope: publicationScope.value })
    if (publicationScope.value === 'customer') {
      params.set('customer_id', String(selectedBeanListCustomerID.value || 0))
    }
    await apiSend(`/api/costing/bean-list/publications/${row.id}/withdraw?${params.toString()}`)
    message.value = `已撤回${beanListTypeLabel(listType)}豆单 ${row.version}`
    await loadBeanListPublications(listType, publicationScope.value)
  } catch (err) {
    error.value = err.message || '撤回豆单失败'
  } finally {
    beanListWithdrawing.value = false
  }
}

onMounted(() => {
  loadCurrentActor()
  loadBeanList()
  loadCustomers()
  loadBeanListPublications('commercial', 'official')
  loadBeanListPublications('commercial', 'mine')
  loadBeanListPublications('green', 'official')
  loadBeanListPublications('green', 'mine')
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
.metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.metrics > div { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fafafa; }
.metrics span, .muted { color: #666; font-size: 12px; }
.metrics span { display: block; margin-bottom: 6px; }
.metrics strong { font-size: 18px; }
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
.explanation-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-bottom: 12px; }
.explanation-summary > div { border: 1px solid #ddd; border-radius: 8px; background: #fff; padding: 10px; }
.explanation-summary span { display: block; margin-bottom: 5px; color: #666; font-size: 12px; }
.explanation-summary strong { display: block; overflow-wrap: anywhere; font-size: 16px; }
.explanation-form { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)) auto; gap: 10px; align-items: end; border: 1px solid #ddd; border-radius: 8px; background: #fff; padding: 10px; margin-bottom: 12px; }
.explanation-form label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.explanation-form input { width: 100%; min-height: 36px; border: 1px solid #ddd; border-radius: 8px; padding: 7px 9px; background: #fff; font: inherit; box-sizing: border-box; }
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
  .bean-grid { grid-template-columns: 1fr; }
  .settings-drawer { width: 100vw; }
  .explanation-summary, .explanation-form, .formula-step { grid-template-columns: 1fr; }
  .formula-step strong { text-align: left; }
  .copy-config-box, .copy-config-actions { grid-template-columns: 1fr; }
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
