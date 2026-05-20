<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>产品豆单</h2>
        <div class="actions">
          <button class="secondary" type="button" :disabled="loading" @click="loadBeanList">刷新</button>
          <button class="secondary" type="button" @click="settingsOpen = true">参数设置</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div v-if="inactiveBomWarningCount" class="warning-banner">BOM已失效：{{ inactiveBomWarningCount }} 款产品依赖的 BOM 已失效，发布豆单前请先重新启用 BOM。</div>
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
      </div>
      <div class="bean-list-global-scope">
        <div>
          <span>豆单范围</span>
          <strong>{{ publicationScopeLabel(versionListScope) }}</strong>
        </div>
        <label class="scope-select">
          <select v-model="versionListScope" aria-label="豆单范围">
            <option value="official">公共豆单</option>
            <option v-for="customer in customers" :key="`version-scope-${customer.id}`" :value="`customer:${customer.id}`">
              {{ customerOptionLabel(customer) }}
            </option>
          </select>
        </label>
      </div>
    </section>

    <section class="panel bean-list-version-panel">
      <div class="section-bar bean-list-version-head">
        <div>
          <div class="section-title">豆单版本列表</div>
          <p class="muted">查看当前豆单范围下的版本、生成新版和撤回。</p>
        </div>
        <div class="actions">
          <button class="secondary compact" type="button" :disabled="beanListVersionListLoading" @click="refreshBeanListVersionList">刷新版本</button>
        </div>
      </div>

      <div class="version-controls">
        <label>
          <span>豆单类型</span>
          <select v-model="pdfOptions.listType">
            <option value="commercial">商用批发豆单</option>
            <option value="drip">挂耳豆单</option>
            <option value="retail">零售豆单</option>
            <option value="green">生豆豆单</option>
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
              <td>{{ beanListTypeLabel(row.list_type) }}</td>
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
                  <button class="secondary compact" type="button" @click="startBeanListFromPublication(row)">生成新版</button>
                  <button v-if="isBeanListAdmin && row.status === 'published'" class="danger compact" type="button" :disabled="beanListWithdrawing" @click="withdrawBeanList(row)">撤回</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="muted empty">
        当前{{ publicationScopeLabel(versionListScope) }}暂无{{ beanListTypeLabel(pdfTheme.listType) }}豆单版本。
      </div>
    </section>

    <section class="panel">
      <div class="bean-list-generate-bar">
        <div>
          <div class="section-title">生成豆单</div>
          <p class="muted">按当前豆单范围生成公共或客户豆单；商用、挂耳、零售、生豆在抽屉中切换。</p>
        </div>
        <button class="primary" type="button" :disabled="loading || !visibleCostingItems.length" @click="openBeanListDrawer(pdfTheme.listType)">生成豆单</button>
      </div>
    </section>

    <section class="panel collapsible-bean-section">
      <div class="collapsible-bean-head">
        <button class="section-toggle" type="button" :aria-expanded="!beanListPreviewCollapsed.commercial" @click="toggleBeanListPreviewSection('commercial')">
          <span>
            <b>商用批发豆单</b>
            <small>{{ commercialGroups.length }} 类 · {{ commercialPreviewItemCount }} 款</small>
          </span>
          <span>{{ beanListPreviewCollapsed.commercial ? '展开' : '收起' }}</span>
        </button>
      </div>
      <div v-show="!beanListPreviewCollapsed.commercial" class="bean-groups">
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

    <section class="panel collapsible-bean-section">
      <div class="collapsible-bean-head">
        <button class="section-toggle" type="button" :aria-expanded="!beanListPreviewCollapsed.drip" @click="toggleBeanListPreviewSection('drip')">
          <span>
            <b>挂耳豆单</b>
            <small>{{ dripGroups.length }} 类 · {{ dripPreviewItemCount }} 款</small>
          </span>
          <span>{{ beanListPreviewCollapsed.drip ? '展开' : '收起' }}</span>
        </button>
      </div>
      <div v-show="!beanListPreviewCollapsed.drip" class="bean-groups">
        <section v-for="group in dripGroups" :key="`drip-${group.category}`" class="bean-group">
          <h3>{{ group.category }}</h3>
          <div class="bean-grid">
            <article v-for="item in group.items" :key="`drip-${item.product_id || item.name}`">
              <div class="bean-heading">
                <span class="bean-code">{{ beanMeta(item, 'drip_bean_list').code }}</span>
                <div>
                  <div class="bean-title">{{ beanName(item, 'drip_bean_list') }}</div>
                  <div v-if="beanMeta(item, 'drip_bean_list').recommended_use" class="bean-use">
                    {{ beanMeta(item, 'drip_bean_list').recommended_use }}
                  </div>
                </div>
              </div>
              <div v-if="beanFlavor(item, 'drip_bean_list')" class="bean-note">{{ beanFlavor(item, 'drip_bean_list') }}</div>
              <div v-if="beanDescription(item, 'drip_bean_list')" class="bean-desc">{{ beanDescription(item, 'drip_bean_list') }}</div>
              <div class="bean-row" v-for="tier in dripDisplayTiers(item)" :key="`drip-tier-${tier.label}-${tier.sales_unit}`">
                <span>{{ tier.label }} / {{ dripTierUnit(tier) }}</span>
                <strong>
                  {{ price(tier.price_per_unit) }}
                  <button v-if="item.drip_price_template" class="source-button" type="button" @click="openDripPriceExplanation(item, tier)">来源</button>
                </strong>
              </div>
            </article>
          </div>
        </section>
        <div v-if="!dripGroups.length" class="muted empty-card">暂无挂耳豆单数据</div>
      </div>
    </section>

    <section class="panel collapsible-bean-section">
      <div class="collapsible-bean-head">
        <button class="section-toggle" type="button" :aria-expanded="!beanListPreviewCollapsed.retail" @click="toggleBeanListPreviewSection('retail')">
          <span>
            <b>零售豆单</b>
            <small>{{ retailGroups.length }} 类 · {{ retailPreviewItemCount }} 款</small>
          </span>
          <span>{{ beanListPreviewCollapsed.retail ? '展开' : '收起' }}</span>
        </button>
      </div>
      <div v-show="!beanListPreviewCollapsed.retail" class="bean-groups">
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

    <section class="panel collapsible-bean-section">
      <div class="collapsible-bean-head">
        <button class="section-toggle" type="button" :aria-expanded="!beanListPreviewCollapsed.green" @click="toggleBeanListPreviewSection('green')">
          <span>
            <b>生豆豆单</b>
            <small>{{ greenGroups.length }} 类 · {{ greenPreviewItemCount }} 款</small>
          </span>
          <span>{{ beanListPreviewCollapsed.green ? '展开' : '收起' }}</span>
        </button>
      </div>
      <div v-show="!beanListPreviewCollapsed.green" class="green-price-save-bar">
        <p class="muted">生豆价格改动会先保存为当前豆单范围下的生豆豆单草稿，正式报价仍需在生成豆单中发布。</p>
        <button class="secondary compact" type="button" :disabled="beanListPublishing || !greenGroups.length || !customerScopeReady" @click="saveGreenBeanPriceDraft">
          {{ beanListPublishing ? '保存中' : '保存生豆价格' }}
        </button>
      </div>
      <div v-show="!beanListPreviewCollapsed.green" class="bean-groups">
        <section v-for="group in greenGroups" :key="`green-${group.category}`" class="bean-group">
          <h3>{{ group.category }}</h3>
          <div class="bean-grid">
            <article v-for="item in group.items" :key="`green-${item.product_id || item.name}`">
              <div class="bean-heading">
                <span class="bean-code">{{ beanMeta(item, 'green_bean_list').code }}</span>
                <div>
                  <div class="bean-title">{{ beanName(item, 'green_bean_list') }}</div>
                  <div v-if="beanMeta(item, 'green_bean_list').recommended_use" class="bean-use">
                    {{ beanMeta(item, 'green_bean_list').recommended_use }}
                  </div>
                </div>
              </div>
              <div v-if="itemWarnings(item).length" class="bean-warning-list">
                <span v-for="warning in itemWarnings(item)" :key="`green-warning-${item.product_id || item.name}-${warning}`" class="warning-chip">{{ warning }}</span>
              </div>
              <div v-if="beanFlavor(item, 'green_bean_list')" class="bean-note">{{ beanFlavor(item, 'green_bean_list') }}</div>
              <div v-if="beanDescription(item, 'green_bean_list')" class="bean-desc">{{ beanDescription(item, 'green_bean_list') }}</div>
              <div class="bean-row green-inline-price-editor" v-for="tier in greenTierPriceRows(item)" :key="`green-tier-${greenTierOverrideKey(tier)}`">
                <span>{{ tier.label }}</span>
                <label>
                  <input type="number" min="0" step="0.01" :value="greenTierPriceValue(itemProductID(item), tier)" @input="setGreenBeanTierPrice(itemProductID(item), tier, $event.target.value)" />
                  <small>/{{ tierUnit(tier) }}</small>
                </label>
              </div>
            </article>
          </div>
        </section>
        <div v-if="!greenGroups.length" class="muted empty-card">暂无生豆豆单数据</div>
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
            <span>临时出成率</span>
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
        <p class="muted">{{ isDripExplanation ? '挂耳价格来源只展示当前公式步骤；交易价格仍需发布豆单后生效。' : '这里的参数只做临时试算；保存请回到快速成本参数或梯度模板，交易价格仍需发布后生效。' }}</p>
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

        <div class="copy-config-box publication-context-box">
          <div>
            <strong>当前归属</strong>
            <p>{{ currentPublicationScopeDescription }}</p>
          </div>
          <div class="current-owner-pill">{{ currentPublicationOwnerLabel }}</div>
          <p v-if="actorLoaded && !isBeanListAdmin" class="muted">客户账号只能保存修改和下载豆单，发布由管理员执行。</p>
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

        <div class="pdf-form">
          <label>
            <span>豆单类型</span>
            <select v-model="pdfOptions.listType">
              <option value="commercial">商用批发豆单</option>
              <option value="drip">挂耳豆单</option>
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
                <div v-if="pdfTheme.listType === 'green' && greenTierPriceRows(row).length" class="green-tier-price-editor">
                  <label v-for="tier in greenTierPriceRows(row)" :key="`green-price-${itemProductID(row)}-${greenTierOverrideKey(tier)}`">
                    <span>{{ tier.label }}</span>
                    <input type="number" min="0" step="0.01" :value="greenTierPriceValue(itemProductID(row), tier)" @input="setGreenBeanTierPrice(itemProductID(row), tier, $event.target.value)" />
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
import {
  DEFAULT_BEAN_LIST_PDF_VERSION,
  buildBeanListPdfGroups,
  buildBeanListPdfSubtitle,
  buildBeanListPdfTitle,
  copyBeanListPublicationContentGroups,
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
const beanListPublishing = ref(false)
const beanListWithdrawing = ref(false)
const beanListVersionListLoading = ref(false)
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
const beanListPublications = ref({
  official: { commercial: [], drip: [], retail: [], green: [] },
  mine: { commercial: [], drip: [], retail: [], green: [] },
  customer: { commercial: [], drip: [], retail: [], green: [] },
})
const beanListPreviewCollapsed = ref({
  commercial: false,
  drip: true,
  retail: true,
  green: true,
})
const priceSourcePublicationByType = ref({ commercial: null, drip: null, retail: null, green: null })
const styleSourcePublicationIDByType = ref({ commercial: 0, drip: 0, retail: 0, green: 0 })
const selectedProductIDsByType = ref({ commercial: [], drip: [], retail: [], green: [] })
const visibleCategoryCodesByType = ref({ commercial: [], drip: [], retail: [], green: [] })
const productSelectionInitialized = ref({ commercial: false, drip: false, retail: false, green: false })
const categorySelectionInitialized = ref({ commercial: false, drip: false, retail: false, green: false })
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
const activeBeanListCustomerID = computed(() => normalizedCustomerContextID.value || versionListScopeCustomerID(versionListScope.value) || Number(selectedBeanListCustomerID.value || 0))
const activeCostingScope = computed(() => {
  return activeBeanListCustomerID.value > 0 ? 'customer' : 'official'
})
const visibleCostingItems = computed(() => filterBeanListItemsForScope(items.value, activeCostingScope.value, activeBeanListCustomerID.value))
const commercialGroups = computed(() => groupBeanItems('commercial_bean_list'))
const dripGroups = computed(() => groupBeanItems('drip_bean_list'))
const retailGroups = computed(() => groupBeanItems('retail_bean_list'))
const greenGroups = computed(() => groupBeanItems('green_bean_list'))
const commercialPreviewItemCount = computed(() => beanListGroupItemCount(commercialGroups.value))
const dripPreviewItemCount = computed(() => beanListGroupItemCount(dripGroups.value))
const retailPreviewItemCount = computed(() => beanListGroupItemCount(retailGroups.value))
const greenPreviewItemCount = computed(() => beanListGroupItemCount(greenGroups.value))
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
const currentScopePublicationRows = computed(() => publicationRows(versionListScope.value, pdfTheme.value.listType))
const versionListCurrentPublication = computed(() => currentScopePublicationRows.value.find((row) => row.status === 'published') || null)
const publicationScopeRows = computed(() => publicationRows(publicationScope.value, pdfTheme.value.listType))
const currentBeanListPublication = computed(() => publicationScopeRows.value.find((row) => row.status === 'published') || null)
const officialPriceSourcePublications = computed(() => publicationRows('official', pdfTheme.value.listType).filter((row) => row.status === 'published'))
const selectedPriceSourcePublication = computed(() => officialPriceSourcePublications.value.find((row) => String(row.id) === String(selectedPriceSourcePublicationID.value)) || null)
const currentPublicationOwnerLabel = computed(() => publicationScopeLabel(publicationScope.value))
const currentPublicationScopeDescription = computed(() => {
  if (publicationScope.value === 'customer') return '生成和发布会保存到当前豆单范围对应的履约客户。'
  if (publicationScope.value === 'mine') return '当前客户账号保存自己的豆单修改。'
  return '生成和发布会保存到公共豆单。'
})
const publicBeanListURL = computed(() => {
  if (publicationScope.value !== 'official' || !currentBeanListPublication.value) return ''
  return `${window.location.origin}/public/bean-list/${pdfTheme.value.listType}`
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

watch(() => pdfOptions.value.listType, (listType) => {
  selectedPriceSourcePublicationID.value = ''
  initializePdfDefaultsForType(listType)
  loadBeanListPublications(listType, versionListScope.value)
  loadBeanListPublications(listType, 'official')
  loadBeanListPublications(listType, 'mine')
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(listType, 'customer')
  }
})

watch(versionListScope, (scope) => {
  syncPublicationScopeFromPageContext()
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  loadBeanListPublications(pdfTheme.value.listType, scope)
})

watch(publicationScope, (scope) => {
  loadBeanListPublications(pdfTheme.value.listType, scope)
  initializePdfDefaultsForType(pdfTheme.value.listType)
})

watch(selectedBeanListCustomerID, () => {
  beanListPublications.value = {
    ...beanListPublications.value,
    customer: { commercial: [], drip: [], retail: [], green: [] },
  }
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  if (publicationScope.value === 'customer' && selectedBeanListCustomerID.value) {
    loadBeanListPublications(pdfTheme.value.listType, 'customer')
  }
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
    publicationScope.value = 'customer'
  } else {
    selectedBeanListCustomerID.value = 0
    publicationScope.value = 'official'
  }
  resetPdfSelectionDefaults()
  initializePdfDefaultsIfItemsLoaded()
  if (publicationScope.value === 'customer') {
    loadBeanListPublications(pdfTheme.value.listType, 'customer')
  }
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
    return ['BOM已失效：请重新启用 BOM 后再发布豆单', ...warnings]
  }
  return warnings
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
  const cacheKey = beanListPublicationCacheKey(scope)
  return beanListPublications.value?.[cacheKey]?.[listType] || []
}

function initializePdfDefaults() {
  initializePdfDefaultsForType('commercial')
  initializePdfDefaultsForType('drip')
  initializePdfDefaultsForType('retail')
  initializePdfDefaultsForType('green')
}

function initializePdfDefaultsIfItemsLoaded() {
  if (!items.value.length) return
  initializePdfDefaults()
}

function resetPdfSelectionDefaults() {
  selectedProductIDsByType.value = { commercial: [], drip: [], retail: [], green: [] }
  visibleCategoryCodesByType.value = { commercial: [], drip: [], retail: [], green: [] }
  productSelectionInitialized.value = { commercial: false, drip: false, retail: false, green: false }
  categorySelectionInitialized.value = { commercial: false, drip: false, retail: false, green: false }
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
  syncPublicationScopeFromPageContext()
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
  openBeanListDrawer(normalizeBeanListType(row.list_type))
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
  priceSourcePublicationByType.value = { ...priceSourcePublicationByType.value, [listType]: row }
  selectedPriceSourcePublicationID.value = String(row.id)
  message.value = `已复制${beanListPublicationLabel(row)}价格来源，发布后会锁定为客户豆单快照`
}

function beanListTypeLabel(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return '生豆'
  if (normalized === 'drip') return '挂耳'
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
  return Number(tier?.price_per_unit || tier?.pricePerUnit || 0)
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
    const data = await apiGet('/api/customer-fulfillment/customers?limit=200')
    const rows = Array.isArray(data.customers) ? data.customers : (data.rows || [])
    customers.value = rows.filter((row) => row.active !== false)
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
    syncPublicationScopeFromPageContext()
  }
}

async function loadBeanListPublications(listType = pdfTheme.value.listType, scope = publicationScope.value) {
  const cacheKey = beanListPublicationCacheKey(scope)
  const requestScope = beanListPublicationRequestScope(scope)
  const customerID = beanListPublicationCustomerID(scope)
  if (requestScope === 'customer' && !customerID) {
    beanListPublications.value = {
      ...beanListPublications.value,
      [cacheKey]: {
        ...(beanListPublications.value[cacheKey] || {}),
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
      [cacheKey]: {
        ...(beanListPublications.value[cacheKey] || {}),
        [listType]: rows,
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
    await loadBeanListPublications(pdfTheme.value.listType, versionListScope.value)
  } finally {
    beanListVersionListLoading.value = false
  }
}

function beanListPublicationURL(listType, scope) {
  const requestScope = beanListPublicationRequestScope(scope)
  const params = new URLSearchParams({ list_type: listType, scope: requestScope })
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
      ? `已发布${beanListTypeName(listType)}豆单 ${row.version}，客户访问链接已生成`
      : `已发布${beanListTypeName(listType)}客户豆单 ${row.version}，内容和价格已锁定为快照`
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
    message.value = `已保存${beanListTypeName(listType)}豆单修改 ${row.version}，可继续生成 PDF 下载`
    await loadBeanListPublications(listType, publicationScope.value)
  } catch (err) {
    error.value = err.message || '保存豆单修改失败'
  } finally {
    beanListPublishing.value = false
  }
}

async function saveGreenBeanPriceDraft() {
  syncPublicationScopeFromPageContext()
  pdfOptions.value = { ...pdfOptions.value, listType: 'green' }
  initializePdfDefaultsForType('green')
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
    message.value = `已保存生豆价格草稿 ${row.version}，可继续在生成豆单中发布`
    await loadBeanListPublications('green', publicationScope.value)
    await loadBeanListPublications('green', versionListScope.value)
  } catch (err) {
    error.value = err.message || '保存生豆价格失败'
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

async function withdrawBeanList(row = currentBeanListPublication.value) {
  if (!row?.id) return
  beanListWithdrawing.value = true
  error.value = ''
  message.value = ''
  const listType = normalizeBeanListType(row.list_type || pdfTheme.value.listType)
  try {
    const params = beanListWithdrawScopeParams(row)
    await apiSend(`/api/costing/bean-list/publications/${row.id}/withdraw?${params.toString()}`)
    message.value = `已撤回${beanListTypeLabel(listType)}豆单 ${row.version}`
    await loadBeanListPublications(listType, publicationScope.value)
    await loadBeanListPublications(listType, versionListScope.value)
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
