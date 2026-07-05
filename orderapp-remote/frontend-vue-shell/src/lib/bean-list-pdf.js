export const DEFAULT_BEAN_LIST_PDF_VERSION = 'V3.0.5'

export function nextBeanListVersion(version, fallback = DEFAULT_BEAN_LIST_PDF_VERSION) {
  const source = String(version || '').trim()
  if (!source) return fallback
  const dotted = source.match(/^(.*\.)(\d+)$/)
  if (dotted) {
    const [, prefix, segment] = dotted
    const next = String(Number(segment) + 1).padStart(segment.length, '0')
    return `${prefix}${next}`
  }
  const trailingNumber = source.match(/\d+$/)
  if (trailingNumber) return `${source}.01`
  return `${source}.01`
}

export function defaultBeanListDraftVersion(publications = [], sourcePublication = null, fallback = DEFAULT_BEAN_LIST_PDF_VERSION) {
  const current = highestBeanListPublicationVersion(
    (Array.isArray(publications) ? publications : []).filter((row) => String(row?.status || '').trim() === 'published'),
  )
  const source = current || beanListPublicationVersion(sourcePublication)
  return nextBeanListVersion(source, fallback)
}

function highestBeanListPublicationVersion(publications = []) {
  return publications
    .map((row) => beanListPublicationVersion(row))
    .filter(Boolean)
    .reduce((max, version) => (compareBeanListVersions(version, max) > 0 ? version : max), '')
}

function beanListPublicationVersion(row = null) {
  return String(row?.version || row?.version_no || row?.versionNo || '').trim()
}

function compareBeanListVersions(left = '', right = '') {
  const leftStandard = isStandardBeanListVersion(left)
  const rightStandard = isStandardBeanListVersion(right)
  if (leftStandard && !rightStandard) return 1
  if (!leftStandard && rightStandard) return -1
  const leftNumbers = beanListVersionNumbers(left)
  const rightNumbers = beanListVersionNumbers(right)
  if (leftNumbers.length && rightNumbers.length) {
    const length = Math.max(leftNumbers.length, rightNumbers.length)
    for (let index = 0; index < length; index += 1) {
      const diff = (leftNumbers[index] || 0) - (rightNumbers[index] || 0)
      if (diff !== 0) return diff
    }
  }
  return String(left || '').localeCompare(String(right || ''), 'zh-Hans-CN')
}

function isStandardBeanListVersion(version = '') {
  return /^v\d+(?:\.\d+)*$/i.test(String(version || '').trim())
}

function beanListVersionNumbers(version = '') {
  const matches = String(version || '').match(/\d+/g)
  return matches ? matches.map((value) => Number(value)) : []
}

export function sanitizeBeanListPdfTheme(input = {}) {
  const listType = normalizeBeanListType(input.listType)
  const rawBrand = input.brandName
  const hasBrand = Object.prototype.hasOwnProperty.call(input, 'brandName') && String(input.brandName ?? '').trim() !== ''
  return {
    listType,
    version: String(input.version || DEFAULT_BEAN_LIST_PDF_VERSION).trim() || DEFAULT_BEAN_LIST_PDF_VERSION,
    brandName: hasBrand ? String(rawBrand || '').trim() : '棵凡咖啡',
    backgroundColor: normalizeColor(input.backgroundColor, '#f8f1e5'),
    fontColor: normalizeColor(input.fontColor, '#171717'),
    backgroundImage: String(input.backgroundImage || '').trim(),
    logoImage: String(input.logoImage || '').trim(),
    brandIntro: String(input.brandIntro || '').trim(),
    layoutStyle: input.layoutStyle === 'table' ? 'table' : 'card',
    cardsPerRow: clampInt(input.cardsPerRow, 2, 1, 4),
    showVersion: input.showVersion !== false,
    showChangelog: input.showChangelog !== false,
    changelog: String(input.changelog || '').trim(),
  }
}

export function buildBeanListPdfTitle(listType, brandName = '棵凡咖啡') {
  const brand = String(brandName ?? '').trim()
  const displayBrand = brand || '棵凡咖啡'
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return brand ? `${brand}生豆产品价格表` : '生豆产品价格表'
  if (normalized === 'drip') return brand ? `${brand}挂耳产品价格表` : '挂耳产品价格表'
  return normalized === 'retail' ? (brand ? `${brand}零售产品价格表` : '零售产品价格表') : (brand ? `${brand}批发产品价格表` : '批发产品价格表')
}

export function filterBeanListItemsForScope(items = [], scope = 'official', customerID = 0, options = {}) {
  const selectedCustomerID = Number(customerID || 0)
  const usePublicCategories = options.usePublicCategories ?? options.use_public_categories ?? true
  return (Array.isArray(items) ? items : []).filter((item) => {
    const itemCustomerID = Number(item?.customer_id ?? item?.customerID ?? 0)
    if (scope === 'customer') {
      if (itemCustomerID <= 0) return Boolean(usePublicCategories)
      return selectedCustomerID > 0 && itemCustomerID === selectedCustomerID
    }
    return itemCustomerID <= 0
  })
}

export function filterBeanListItemsForPriceTableScope(items = [], scope = 'official', customerID = 0) {
  const selectedCustomerID = Number(customerID || 0)
  return (Array.isArray(items) ? items : []).filter((item) => {
    const itemCustomerID = Number(item?.customer_id ?? item?.customerID ?? 0)
    if (scope === 'customer') {
      return itemCustomerID > 0 && itemCustomerID === selectedCustomerID
    }
    return itemCustomerID <= 0
  })
}

export function applyCustomerProductAliasesToBeanListItems(items = [], aliases = [], customerID = 0) {
  const selectedCustomerID = Number(customerID || 0)
  if (selectedCustomerID <= 0) return Array.isArray(items) ? items : []
  const aliasByProduct = new Map()
  ;(Array.isArray(aliases) ? aliases : []).forEach((alias) => {
    const aliasCustomerID = Number(alias?.customer_id ?? alias?.customerID ?? 0)
    const productID = Number(alias?.product_id ?? alias?.productID ?? 0)
    if (aliasCustomerID !== selectedCustomerID || productID <= 0) return
    if (alias?.active === false || alias?.include_in_price_list === false || alias?.includeInPriceList === false) return
    if (!aliasByProduct.has(productID)) aliasByProduct.set(productID, alias)
  })
  return (Array.isArray(items) ? items : [])
    .map((item) => {
      const productID = Number(item?.product_id ?? item?.productID ?? item?.productId ?? item?.id ?? 0)
      const alias = aliasByProduct.get(productID)
      if (!alias) return null
      return customerAliasBeanListItem(item, alias, selectedCustomerID)
    })
    .filter(Boolean)
}

export function buildBeanListPdfSubtitle(listType) {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return '生豆销售报价'
  if (normalized === 'drip') return '挂耳供应价，报价按袋/盒快照发布'
  return normalized === 'retail' ? '报价含税运' : '报价不含税、不含运'
}

export function buildBeanListPdfGroups(items = [], listType = 'commercial', options = {}) {
  const normalizedListType = normalizeBeanListType(listType)
  const metaKey = normalizedListType === 'green' ? 'green_bean_list' : normalizedListType === 'retail' ? 'retail_bean_list' : normalizedListType === 'drip' ? 'drip_bean_list' : 'commercial_bean_list'
  const tierKey = normalizedListType === 'green' ? 'green_bean_sale_tiers' : normalizedListType === 'retail' ? 'retail_bean_tiers' : normalizedListType === 'drip' ? 'drip_wholesale_tiers' : 'commercial_wholesale_tiers'
  const selectedIDs = normalizeStringSet(options.selectedProductIDs)
  const hasProductFilter = Object.prototype.hasOwnProperty.call(options, 'selectedProductIDs')
  const visibleCategoryCodes = normalizeStringSet(options.visibleCategoryCodes)
  const hasCategoryFilter = Object.prototype.hasOwnProperty.call(options, 'visibleCategoryCodes')
  const showCategoryNumbers = options.showCategoryNumbers !== false
  const customizers = options.customizers && typeof options.customizers === 'object' ? options.customizers : {}
  const sourceRows = items
    .filter((item) => item?.[metaKey]?.code)
    .filter((item) => !hasProductFilter || selectedIDs.has(productIDOf(item)))
    .filter((item) => !hasCategoryFilter || visibleCategoryCodes.has(firstCodePart(item[metaKey].code)))
    .slice()
    .sort((a, b) => compareBeanCodes(a[metaKey].code, b[metaKey].code))

  if (!showCategoryNumbers) {
    return [{
      category: '全部产品',
      categoryCode: '',
      originalCategoryCode: '',
      showCategory: false,
      items: sourceRows.map((item, index) => buildPdfItem(item, metaKey, tierKey, normalizedListType, String(index + 1), customizers)),
    }]
  }

  const categoryOrder = []
  sourceRows.forEach((item) => {
    const code = firstCodePart(item[metaKey].code)
    if (!categoryOrder.includes(code)) categoryOrder.push(code)
  })
  const categoryRemap = new Map(categoryOrder.map((code, index) => [code, hasCategoryFilter ? String(index + 1) : code]))
  const groups = new Map()
  sourceRows.forEach((item) => {
    const meta = item[metaKey] || {}
    const originalCategoryCode = firstCodePart(meta.code)
    const categoryCode = categoryRemap.get(originalCategoryCode) || originalCategoryCode
    const category = renumberCategory(meta.category || '未分类', categoryCode)
    const key = originalCategoryCode || category
    if (!groups.has(key)) {
      groups.set(key, { category, categoryCode, originalCategoryCode, showCategory: true, items: [] })
    }
    groups.get(key).items.push(buildPdfItem(item, metaKey, tierKey, normalizedListType, renumberItemCode(meta.code, categoryCode), customizers))
  })
  return Array.from(groups.values())
}

export function buildBeanListPdfGroupsFromCategoryRows(categoryRows = [], listType = 'commercial', options = {}) {
  const normalizedListType = normalizeBeanListType(listType)
  const metaKey = normalizedListType === 'green' ? 'green_bean_list' : normalizedListType === 'retail' ? 'retail_bean_list' : normalizedListType === 'drip' ? 'drip_bean_list' : 'commercial_bean_list'
  const tierKey = normalizedListType === 'green' ? 'green_bean_sale_tiers' : normalizedListType === 'retail' ? 'retail_bean_tiers' : normalizedListType === 'drip' ? 'drip_wholesale_tiers' : 'commercial_wholesale_tiers'
  const selectedIDs = normalizeStringSet(options.selectedProductIDs)
  const hasProductFilter = Object.prototype.hasOwnProperty.call(options, 'selectedProductIDs')
  const visibleCategoryCodes = normalizeStringSet(options.visibleCategoryCodes)
  const hasCategoryFilter = Object.prototype.hasOwnProperty.call(options, 'visibleCategoryCodes')
  const showCategoryNumbers = options.showCategoryNumbers !== false
  const customizers = options.customizers && typeof options.customizers === 'object' ? options.customizers : {}
  const rows = (Array.isArray(categoryRows) ? categoryRows : [])
    .map((row, index) => {
      const categoryCode = String(row?.code || row?.key || row?.group_item_id || index + 1).trim()
      const categoryLabel = String(row?.label || row?.category || row?.group_item_name || row?.path_label || '未分类').trim() || '未分类'
      const sourceItems = Array.isArray(row?.items) ? row.items : (Array.isArray(row?.rows) ? row.rows : [])
      const items = sourceItems
        .filter((item) => item?.[metaKey]?.code)
        .filter((item) => !hasProductFilter || selectedIDs.has(productIDOf(item)))
      return {
        categoryCode,
        categoryLabel,
        items,
      }
    })
    .filter((row) => (!hasCategoryFilter || visibleCategoryCodes.has(row.categoryCode)) && row.items.length > 0)

  if (!showCategoryNumbers) {
    let itemIndex = 0
    const items = rows.flatMap((row) => row.items.map((item) => {
      itemIndex += 1
      return buildPdfItem(item, metaKey, tierKey, normalizedListType, String(itemIndex), customizers, { includeMarketingFields: false })
    }))
    if (!items.length) return []
    return [{
      category: '全部产品',
      categoryCode: '',
      originalCategoryCode: '',
      showCategory: false,
      items,
    }]
  }

  return rows.map((row, index) => {
    const displayCategoryCode = String(index + 1)
    return {
      category: renumberCategory(row.categoryLabel, displayCategoryCode),
      categoryCode: row.categoryCode,
      originalCategoryCode: row.categoryCode,
      showCategory: true,
      items: row.items.map((item, itemIndex) => buildPdfItem(item, metaKey, tierKey, normalizedListType, `${displayCategoryCode}.${itemIndex + 1}`, customizers, { includeMarketingFields: false })),
    }
  })
}

export function applyPriceListFlatRowsToBeanListPdfGroups(groups = [], rows = [], listType = 'commercial') {
  const normalizedRows = (Array.isArray(rows) ? rows : [])
    .map(normalizePriceListFlatRow)
    .filter((row) => row.final_unit_price > 0)
  if (!normalizedRows.length) return JSON.parse(JSON.stringify(Array.isArray(groups) ? groups : []))
  const tierKey = priceListTierKeyForType(listType)
  return (Array.isArray(groups) ? groups : []).map((group) => ({
    ...group,
    items: (Array.isArray(group?.items) ? group.items : []).map((item) => {
      const itemRows = flatRowsForPdfItem(item, normalizedRows)
      if (!itemRows.length) return { ...item }
      const prices = itemRows.map((row) => ({
        label: row.tier_label || (row.pricing_mode === 'fixed_price' ? '固定价' : '基础价'),
        price: row.final_unit_price,
        unit: priceUnitLabelForFlatRow(row, listType),
        red: false,
      }))
      const tierSnapshots = itemRows.map((row) => flatRowTierSnapshot(row))
      return {
        ...item,
        prices,
        price_unit_snapshot: itemRows[0]?.price_unit || item.price_unit_snapshot || '',
        tiers_snapshot: tierSnapshots,
        [tierKey]: tierSnapshots,
      }
    }),
  }))
}

function normalizeBeanListType(listType) {
  if (listType === 'retail') return 'retail'
  if (listType === 'drip') return 'drip'
  if (listType === 'green' || listType === 'green_bean') return 'green'
  return 'commercial'
}

function buildPdfItem(item, metaKey, tierKey, listType, code, customizers, options = {}) {
  const meta = item[metaKey] || {}
  const customizer = customizerFor(item, customizers)
  const highlightTerms = normalizeStringList(customizer.highlightTerms)
  const badge = normalizeBadge(customizer.badge)
  const beanListQuality = normalizeBeanListQuality(item.bean_list_quality || item.beanListQuality)
  const productAttributes = normalizeProductAttributes(item.product_attributes || item.productAttributes)
  const tierSnapshots = attachProductPriceSnapshotsToTiers(item, pdfTierSnapshots(item, tierKey, listType, customizer), listType)
  const prices = pdfPriceRows(item, tierKey, listType, customizer, tierSnapshots)
  const displayName = stringField(item.customer_product_display_name ?? item.customerProductDisplayName ?? meta.display_name ?? item.name)
  const productID = firstNumber(item.product_id, item.productID, item.productId, item.id)
  const includeMarketingFields = options.includeMarketingFields !== false
  return {
    productId: productID || null,
    product_id: productID || null,
    sku_id: firstNumber(item.sku_id, item.skuID, item.skuId),
    parent_product_id: firstNumber(item.parent_product_id, item.parentProductID, item.parentProductId),
    effective_parent_product_id: firstNumber(item.effective_parent_product_id, item.effectiveParentProductID, item.effectiveParentProductId),
    sku_name: stringField(item.sku_name ?? item.skuName),
    sku_code: stringField(item.sku_code ?? item.skuCode),
    barcode: stringField(item.barcode),
    spec_label: stringField(item.spec_label ?? item.specLabel),
    net_content_qty: firstNumber(item.net_content_qty, item.netContentQty),
    net_content_unit: stringField(item.net_content_unit ?? item.netContentUnit),
    inventory_unit: stringField(item.inventory_unit ?? item.inventoryUnit),
    default_sales_unit: stringField(item.default_sales_unit ?? item.defaultSalesUnit),
    quote_unit: stringField(item.quote_unit ?? item.quoteUnit),
    order_unit: stringField(item.order_unit ?? item.orderUnit),
    unit_conversion_json: parseJSONObject(item.unit_conversion_json ?? item.unitConversionJSON),
    sales_units: Array.isArray(item.sales_units) ? item.sales_units.slice() : (Array.isArray(item.salesUnits) ? item.salesUnits.slice() : []),
    customer_product_alias_id: firstNumber(item.customer_product_alias_id, item.customerProductAliasID),
    customer_id: firstNumber(item.customer_id, item.customerID),
    code: code || meta.code || '',
    originalCode: meta.code || '',
    name: displayName || item.name || '',
    display_name_snapshot: displayName || item.name || '',
    customer_item_code_snapshot: stringField(item.customer_item_code ?? item.customerItemCode),
    brand_name_snapshot: stringField(item.brand_name ?? item.brandName),
    display_category_snapshot: stringField(item.display_category_name ?? item.displayCategoryName ?? meta.category),
    product_code_snapshot: stringField(item.product_code ?? item.productCode),
    product_name_snapshot: stringField(item.product_name ?? item.productName ?? item.name),
    bom_version_id_snapshot: firstNumber(item.bom_version_id, item.bomVersionID),
    bom_version_no_snapshot: stringField(item.bom_version_no ?? item.bomVersionNo),
    bom_usage_mode_snapshot: stringField(item.bom_usage_mode ?? item.bomUsageMode),
    price_unit_snapshot: stringField(prices[0]?.unit || tierSnapshots[0]?.display_unit || tierSnapshots[0]?.price_unit || item.quote_unit || item.quoteUnit),
    tiers_snapshot: tierSnapshots,
    special_attrs_snapshot: productAttributes,
    price_source_json: beanListItemPriceSource(item, listType, tierKey),
    recommendedUse: includeMarketingFields ? (meta.recommended_use || '') : '',
    flavor: includeMarketingFields ? (meta.flavor || item.flavor || '') : '',
    description: includeMarketingFields ? (meta.description || item.bean_list_note || '') : '',
    ...(productAttributes.length ? { productAttributes, attributeLines: productAttributes.map((attr) => `${attr.label}：${attr.value}`) } : {}),
    badge,
    badgeLabel: badgeLabel(badge),
    highlightTerms,
    ...(beanListQuality ? { beanListQuality, qualityLines: beanListQualityLines(beanListQuality) } : {}),
    ...(tierSnapshots.length ? { [tierKey]: tierSnapshots } : {}),
    prices,
  }
}

function normalizeProductAttributes(value = []) {
  if (!Array.isArray(value)) return []
  const seen = new Set()
  const rows = []
  value.forEach((row) => {
    const key = String(row?.key || '').trim()
    const label = productAttributeDisplayLabel(key, row?.label)
    const attr = {
      key,
      label,
      value: String(row?.value || '').trim(),
    }
    if (!attr.key || !attr.label || !attr.value) return
    const dedupeKey = productAttributeDedupeKey(attr)
    if (seen.has(dedupeKey)) return
    seen.add(dedupeKey)
    rows.push(attr)
  })
  return rows
}

function productAttributeDisplayLabel(key, label) {
  const rawKey = String(key || '').trim()
  const rawLabel = String(label || '').trim()
  const normalized = normalizeProductAttributeToken(rawLabel || rawKey)
  if (normalized === 'roast_level') return '烘焙度'
  return rawLabel || rawKey
}

function productAttributeDedupeKey(attr = {}) {
  const normalizedKey = normalizeProductAttributeToken(attr.key)
  const normalizedLabel = normalizeProductAttributeToken(attr.label)
  if (normalizedKey === 'roast_level' || normalizedLabel === 'roast_level') return 'roast_level'
  return normalizedKey || normalizedLabel
}

function normalizeProductAttributeToken(value) {
  const text = String(value || '').trim().toLowerCase()
  if (['roast_level', 'roastlevel', 'roast_degree', 'roastdegree', '烘焙度'].includes(text)) return 'roast_level'
  return text
}

function pdfTierSnapshots(item, tierKey, listType, customizer = {}) {
  const tiers = Array.isArray(item[tierKey]) ? item[tierKey] : []
  return tiers.map((tier) => {
    const next = { ...tier }
    if (listType === 'green') {
      const override = greenTierPriceOverride(tier, customizer)
      if (override != null) applyGreenTierPrice(next, override)
    }
    return next
  })
}

function pdfPriceRows(item, tierKey, listType, customizer = {}, tierSnapshots = null) {
  const tiers = Array.isArray(tierSnapshots) ? tierSnapshots : pdfTierSnapshots(item, tierKey, listType, customizer)
  if (listType === 'drip') {
    return tiers.flatMap((tier) => dripPdfPriceRows(tier, item))
  }
  if (listType === 'green') {
    return tiers.map((tier) => ({
      label: tier.label || '',
      price: greenDisplayPrice(tier),
      unit: greenPriceUnitLabel(tier),
      red: false,
    }))
  }
  return tiers.map((tier) => {
    const label = tier.label || ''
    return {
      label,
      price: firstNumber(tier.price_per_unit, tier.price_per_lb, 0),
      unit: listType === 'retail' ? '' : priceUnit(tier),
      red: false,
    }
  })
}

function attachProductPriceSnapshotsToTiers(item, tiers = [], listType = 'commercial') {
  if (!Array.isArray(tiers) || !tiers.length) return []
  return tiers.map((tier) => {
    const snapshot = productPriceSnapshotForTier(item, tier, listType)
    if (!snapshot) return { ...tier }
    const priceUnit = stringField(snapshot.price_unit || tier.price_unit || tier.display_unit)
    const inventoryUnit = stringField(snapshot.inventory_unit || item.inventory_unit || item.inventoryUnit || 'kg')
    return {
      ...tier,
      source_price_record_id: firstNumber(snapshot.source_price_record_id, snapshot.id),
      final_unit_price: firstNumber(snapshot.final_unit_price, tier.final_unit_price, tier.price_per_unit),
      price_unit: priceUnit || stringField(tier.price_unit || tier.display_unit),
      currency: stringField(snapshot.currency) || 'CNY',
      inventory_unit: inventoryUnit,
      inventory_conversion_json: inventoryConversionSnapshot(snapshot, priceUnit, inventoryUnit),
    }
  })
}

function productPriceSnapshotForTier(item, tier = {}, listType = 'commercial') {
  const snapshots = productPriceSnapshotsForItem(item)
  if (!snapshots.length) return null
  const tierPrice = tierFinalPrice(tier, listType)
  const tierUnit = normalizeComparableUnit(tier.price_unit || tier.display_unit || (listType === 'green' ? greenPriceUnitLabel(tier) : priceUnit(tier)))
  const productID = firstNumber(item.product_id, item.productID, item.productId, item.id)
  const aliasID = firstNumber(item.customer_product_alias_id, item.customerProductAliasID)
  const candidates = snapshots.filter((snapshot) => {
    const snapshotAliasID = firstNumber(snapshot.customer_product_alias_id, snapshot.customerProductAliasID)
    const snapshotProductID = firstNumber(snapshot.product_id, snapshot.productID, snapshot.productId)
    if (aliasID > 0 && snapshotAliasID > 0) return snapshotAliasID === aliasID
    return snapshotProductID <= 0 || productID <= 0 || snapshotProductID === productID
  })
  return candidates.find((snapshot) => {
    const price = firstNumber(snapshot.final_unit_price, snapshot.finalUnitPrice)
    const unit = normalizeComparableUnit(snapshot.price_unit || snapshot.priceUnit)
    return pricesClose(price, tierPrice) && (!tierUnit || !unit || unit === tierUnit)
  }) || candidates.find((snapshot) => pricesClose(firstNumber(snapshot.final_unit_price, snapshot.finalUnitPrice), tierPrice)) || null
}

function productPriceSnapshotsForItem(item = {}) {
  const rows = item.product_price_snapshots || item.productPriceSnapshots || item.price_records_snapshot || item.priceRecordsSnapshot || []
  return Array.isArray(rows) ? rows : []
}

function tierFinalPrice(tier = {}, listType = 'commercial') {
  if (listType === 'green') return greenDisplayPrice(tier)
  return firstNumber(tier.final_unit_price, tier.finalUnitPrice, tier.price_per_unit, tier.pricePerUnit, tier.packed_price_per_bag, tier.packedPricePerBag, tier.packed_price_per_box, tier.packedPricePerBox, tier.price_per_kg, tier.pricePerKg, tier.price_per_lb, tier.pricePerLb)
}

function pricesClose(a, b) {
  return Number.isFinite(Number(a)) && Number.isFinite(Number(b)) && Math.abs(Number(a) - Number(b)) < 0.005
}

function inventoryConversionSnapshot(snapshot = {}, priceUnit = '', inventoryUnit = '') {
  const parsed = parseJSONObject(snapshot.inventory_conversion_json ?? snapshot.inventoryConversionJSON)
  if (Object.keys(parsed).length) return parsed
  const source = stringField(priceUnit)
  const target = stringField(inventoryUnit)
  if (source && target && source === target) return { [source]: { [target]: 1 } }
  return {}
}

export function buildPriceListGenerationSnapshot(input = {}) {
  const defaults = normalizeTemplateSelection(input.defaults)
  const groupSelections = (Array.isArray(input.groupSelections) ? input.groupSelections : [])
    .map(normalizeGroupTemplateSelection)
  const productOverrides = (Array.isArray(input.productOverrides) ? input.productOverrides : [])
    .map(normalizeProductTemplateOverride)
    .filter((row) => row.product_id > 0 || row.product_key)
  const priceRows = (Array.isArray(input.rows) ? input.rows : [])
    .map(normalizePriceListFlatRow)
    .filter((row) => row.product_id > 0 || row.product_key || row.product_name)
  return {
    config: {
      price_list_template_selection: {
        defaults,
        group_selections: groupSelections,
        product_overrides: productOverrides,
      },
    },
    content: {
      price_rows: priceRows,
    },
  }
}

function normalizeTemplateSelection(value = {}) {
  return {
    pricing_mode: stringField(value.pricing_mode ?? value.pricingMode),
    tier_template_id: firstNumber(value.tier_template_id, value.tierTemplateID),
    pricing_rule_id: firstNumber(value.pricing_rule_id, value.pricingRuleID),
    fixed_unit_price: firstNumber(value.fixed_unit_price, value.fixedUnitPrice),
  }
}

function normalizeGroupTemplateSelection(row = {}) {
  return {
    group_id: firstNumber(row.group_id, row.groupID),
    group_name: stringField(row.group_name ?? row.groupName),
    group_item_id: firstNumber(row.group_item_id, row.groupItemID),
    group_item_name: stringField(row.group_item_name ?? row.groupItemName),
    parent_group_item_id: firstNumber(row.parent_group_item_id, row.parentGroupItemID),
    parent_group_item_name: stringField(row.parent_group_item_name ?? row.parentGroupItemName),
    level: firstNumber(row.level),
    ...normalizeTemplateSelection(row),
  }
}

function normalizeProductTemplateOverride(row = {}) {
  return {
    product_id: firstNumber(row.product_id, row.productID, row.productId),
    product_key: stringField(row.product_key ?? row.productKey),
    product_name: stringField(row.product_name ?? row.productName),
    ...normalizeTemplateSelection(row),
    final_unit_price: firstNumber(row.final_unit_price, row.finalUnitPrice),
  }
}

function normalizePriceListFlatRow(row = {}) {
  const finalUnitPrice = firstNumber(row.final_unit_price, row.finalUnitPrice, row.price_per_unit, row.pricePerUnit)
  const originalFinalUnitPrice = firstNumber(row.original_final_unit_price, row.originalFinalUnitPrice, row.source_final_unit_price, row.sourceFinalUnitPrice, finalUnitPrice)
  const manualAdjusted = row.manual_adjusted === true || row.manualAdjusted === true || !pricesClose(finalUnitPrice, originalFinalUnitPrice)
  return {
    product_id: firstNumber(row.product_id, row.productID, row.productId),
    sku_id: firstNumber(row.sku_id, row.skuID, row.skuId),
    parent_product_id: firstNumber(row.parent_product_id, row.parentProductID, row.parentProductId),
    sku_snapshot: parseJSONObject(row.sku_snapshot ?? row.skuSnapshot),
    sku_name: stringField(row.sku_name ?? row.skuName),
    sku_code: stringField(row.sku_code ?? row.skuCode),
    barcode: stringField(row.barcode),
    spec_label: stringField(row.spec_label ?? row.specLabel),
    net_content_qty: firstNumber(row.net_content_qty, row.netContentQty),
    net_content_unit: stringField(row.net_content_unit ?? row.netContentUnit),
    product_key: stringField(row.product_key ?? row.productKey),
    product_name: stringField(row.product_name ?? row.productName ?? row.name),
    group_snapshot: parseJSONObject(row.group_snapshot ?? row.groupSnapshot),
    group_source: stringField(row.group_source ?? row.groupSource) || 'product_catalog',
    pricing_mode: stringField(row.pricing_mode ?? row.pricingMode),
    pricing_mode_source: stringField(row.pricing_mode_source ?? row.pricingModeSource),
    tier_label: stringField(row.tier_label ?? row.tierLabel ?? row.label),
    min_qty: firstNumber(row.min_qty, row.minQty),
    max_qty: firstNumber(row.max_qty, row.maxQty),
    price_unit: stringField(row.price_unit ?? row.priceUnit),
    final_unit_price: finalUnitPrice,
    original_final_unit_price: originalFinalUnitPrice,
    currency: stringField(row.currency) || 'CNY',
    inventory_unit: stringField(row.inventory_unit ?? row.inventoryUnit),
    inventory_conversion_json: inventoryConversionSnapshot(row, row.price_unit ?? row.priceUnit, row.inventory_unit ?? row.inventoryUnit),
    source_price_record_id: firstNumber(row.source_price_record_id, row.sourcePriceRecordID),
    tier_template_id: firstNumber(row.tier_template_id, row.tierTemplateID),
    tier_template_source: stringField(row.tier_template_source ?? row.tierTemplateSource),
    template_tier_id: firstNumber(row.template_tier_id, row.templateTierID),
    pricing_rule_id: firstNumber(row.pricing_rule_id, row.pricingRuleID),
    pricing_rule_source: stringField(row.pricing_rule_source ?? row.pricingRuleSource),
    pricing_rule_version: stringField(row.pricing_rule_version ?? row.pricingRuleVersion),
    tier_pricing_rule_id: firstNumber(row.tier_pricing_rule_id, row.tierPricingRuleID),
    tier_pricing_rule_version: stringField(row.tier_pricing_rule_version ?? row.tierPricingRuleVersion),
    fixed_unit_price: firstNumber(row.fixed_unit_price, row.fixedUnitPrice),
    cost_source_snapshot: parseJSONObject(row.cost_source_snapshot ?? row.costSourceSnapshot),
    customer_reference_snapshot: parseJSONObject(row.customer_reference_snapshot ?? row.customerReferenceSnapshot),
    manual_adjusted: manualAdjusted,
    manual_adjustment_label: manualAdjusted ? '人工调整' : '',
  }
}

function parseJSONObject(value) {
  if (!value) return {}
  if (typeof value === 'object' && !Array.isArray(value)) return value
  try {
    const parsed = JSON.parse(String(value))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function normalizeComparableUnit(unit) {
  const value = stringField(unit)
  const lower = value.toLowerCase()
  if (['kg', 'lb', 'g100', 'g227', 'g250'].includes(lower)) return lower
  if (value === '100g') return 'g100'
  if (value === '227g') return 'g227'
  if (value === '250g') return 'g250'
  if (value === '磅') return 'lb'
  return value
}

function priceListTierKeyForType(listType = 'commercial') {
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return 'green_bean_sale_tiers'
  if (normalized === 'drip') return 'drip_wholesale_tiers'
  if (normalized === 'retail') return 'retail_bean_tiers'
  return 'commercial_wholesale_tiers'
}

function flatRowsForPdfItem(item = {}, rows = []) {
  const skuID = firstNumber(item.sku_id, item.skuID, item.skuId)
  const productID = firstNumber(item.product_id, item.productID, item.productId, item.id)
  const productKey = stringField(item.product_key ?? item.productKey)
  const productName = stringField(item.product_name_snapshot ?? item.productNameSnapshot ?? item.name)
  return rows.filter((row) => {
    if (skuID > 0 && row.sku_id > 0) return row.sku_id === skuID
    if (productID > 0 && row.product_id > 0) return row.product_id === productID
    if (productKey && row.product_key) return row.product_key === productKey
    return productName && row.product_name === productName
  })
}

function priceUnitLabelForFlatRow(row = {}, listType = 'commercial') {
  if (normalizeBeanListType(listType) === 'retail') return ''
  const unit = stringField(row.price_unit ?? row.priceUnit)
  if (unit === 'lb') return '磅'
  if (unit === 'g100') return '100g'
  if (unit === 'g227') return '227g'
  if (unit === 'g250') return '250g'
  return unit
}

function flatRowTierSnapshot(row = {}) {
  return {
    label: row.tier_label,
    min_qty: row.min_qty,
    max_qty: row.max_qty,
    final_unit_price: row.final_unit_price,
    original_final_unit_price: row.original_final_unit_price,
    price_per_unit: row.final_unit_price,
    price_unit: row.price_unit,
    display_unit: row.price_unit,
    currency: row.currency,
    inventory_unit: row.inventory_unit,
    inventory_conversion_json: row.inventory_conversion_json,
    source_price_record_id: row.source_price_record_id,
    tier_template_id: row.tier_template_id,
    tier_template_source: row.tier_template_source,
    template_tier_id: row.template_tier_id,
    pricing_mode: row.pricing_mode,
    pricing_rule_id: row.pricing_rule_id,
    pricing_rule_source: row.pricing_rule_source,
    pricing_rule_version: row.pricing_rule_version,
    tier_pricing_rule_id: row.tier_pricing_rule_id,
    tier_pricing_rule_version: row.tier_pricing_rule_version,
    manual_adjusted: row.manual_adjusted,
  }
}

function applyGreenTierPrice(tier, price) {
  const unitPrice = roundTo(price, 2)
  const unit = greenTierPriceUnit(tier, true)
  const unitG = greenPriceUnitG(unit, tier)
  const pricePerKg = unitG === 1000 ? unitPrice : unitPrice * 1000 / unitG
  tier.price_unit = unit
  tier.price_per_unit = unitPrice
  tier.price_per_kg = roundTo(pricePerKg, 2)
  tier.price_per_lb = roundTo(pricePerKg * 0.454, 2)
}

function greenPriceUnitLabel(tier = {}) {
  const unit = greenTierPriceUnit(tier)
  if (unit === 'kg') return 'kg'
  if (unit === 'g100') return '100g'
  if (unit === 'g227') return '227g'
  if (unit === 'g250') return '250g'
  return '磅'
}

function greenDisplayPrice(tier = {}) {
  const unit = greenTierPriceUnit(tier)
  const pricePerKg = firstNumber(tier.price_per_kg, tier.display_unit === 'kg' ? tier.price_per_unit : '', tier.price_per_lb ? Number(tier.price_per_lb) / 0.454 : '')
  if (unit === 'kg') return roundTo(pricePerKg || firstNumber(tier.price_per_unit, 0), 2)
  if (unit === 'lb') return roundTo(firstNumber(tier.price_per_lb, tier.price_unit === 'lb' ? tier.price_per_unit : '', pricePerKg ? pricePerKg * 0.454 : 0), 2)
  const unitG = greenPriceUnitG(unit, tier)
  if (tier.price_unit === unit) return roundTo(firstNumber(tier.price_per_unit, 0), 2)
  if (pricePerKg > 0) return roundTo(pricePerKg * unitG / 1000, 2)
  return roundTo(firstNumber(tier.price_per_unit, tier.price_per_lb, 0), 2)
}

function greenTierPriceUnit(tier = {}, preferDisplay = false) {
  const displayUnit = normalizeGreenPriceUnit(tier.display_unit)
  const explicitUnit = normalizeGreenPriceUnit(tier.price_unit)
  return (preferDisplay ? (displayUnit || explicitUnit) : (explicitUnit || displayUnit)) || 'lb'
}

function normalizeGreenPriceUnit(unit) {
  const value = String(unit || '').trim().toLowerCase()
  return ['kg', 'lb', 'g100', 'g227', 'g250'].includes(value) ? value : ''
}

function greenPriceUnitG(unit, tier = {}) {
  switch (normalizeGreenPriceUnit(unit)) {
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
      return Number(tier.spec_g || 454) || 454
  }
}

function greenTierPriceOverride(tier = {}, customizer = {}) {
  const overrides = customizer.greenPriceOverrides && typeof customizer.greenPriceOverrides === 'object' ? customizer.greenPriceOverrides : {}
  for (const key of greenTierOverrideKeys(tier)) {
    const value = Number(overrides[key])
    if (Number.isFinite(value) && value > 0) return value
  }
  return undefined
}

function greenTierOverrideKeys(tier = {}) {
  return [
    tier.template_tier_id,
    tier.templateTierID,
    tier.label,
  ].filter((value) => value !== undefined && value !== null && String(value).trim() !== '').map((value) => String(value))
}

function dripPdfPriceRows(tier = {}, item = {}) {
  if (tier.sales_unit === 'box') {
    return [{
      label: tier.label || '',
      price: firstNumber(tier.price_per_unit, tier.packed_price_per_box, 0),
      unit: dripPriceUnit(tier),
      red: false,
    }]
  }
  const boxBagCount = positiveInteger(tier.unit_bag_count, tier.box_bag_count, item.drip_box_bag_count, 10)
  const bagPrice = firstNumber(tier.price_per_unit, tier.packed_price_per_bag, tier.price_per_lb, 0)
  const rows = [{
    label: tier.label || '',
    price: bagPrice,
    unit: '袋',
    red: false,
  }]
  if (boxBagCount > 1) {
    rows.push({
      label: tier.label || '',
      price: firstNumber(tier.packed_price_per_box, bagPrice * boxBagCount),
      unit: `盒(${boxBagCount}袋)`,
      red: false,
    })
  }
  return rows
}

function dripPriceUnit(tier = {}) {
  if (tier.sales_unit === 'box') return `盒(${positiveInteger(tier.unit_bag_count, tier.box_bag_count, 10)}袋)`
  return '袋'
}

function firstNumber(...values) {
  for (const value of values) {
    if (value === null || value === undefined || value === '') continue
    const n = Number(value)
    if (Number.isFinite(n)) return n
  }
  return 0
}

function roundTo(value, precision = 2) {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  const pow = 10 ** precision
  return Math.round(n * pow) / pow
}

function positiveInteger(...values) {
  for (const value of values) {
    const n = Number.parseInt(String(value ?? ''), 10)
    if (Number.isFinite(n) && n > 0) return n
  }
  return 10
}

function normalizeBeanListQuality(input) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return null
  const candidates = {
    factoryFlavorDescription: input.factory_flavor_description ?? input.factoryFlavorDescription,
    moisture: input.moisture,
    density: input.density,
    inspectionCreatedAt: input.inspection_created_at ?? input.inspectionCreatedAt,
    inspectionReferenceNo: input.inspection_reference_no ?? input.inspectionReferenceNo,
  }
  const out = {}
  Object.entries(candidates).forEach(([key, value]) => {
    const normalized = stringField(value)
    if (normalized) out[key] = normalized
  })
  return Object.keys(out).length > 0 ? out : null
}

function beanListQualityLines(quality = {}) {
  return [
    { label: '工厂风味', value: stringField(quality.factoryFlavorDescription) },
    { label: '水分', value: stringField(quality.moisture) },
    { label: '密度', value: stringField(quality.density) },
    { label: '质检时间', value: stringField(quality.inspectionCreatedAt) },
    { label: '质检单号', value: stringField(quality.inspectionReferenceNo) },
  ].filter((line) => line.value)
}

export function compareBeanCodes(a, b) {
  const aa = String(a || '').split('.').map((v) => Number(v) || 0)
  const bb = String(b || '').split('.').map((v) => Number(v) || 0)
  const len = Math.max(aa.length, bb.length)
  for (let i = 0; i < len; i += 1) {
    if ((aa[i] || 0) !== (bb[i] || 0)) return (aa[i] || 0) - (bb[i] || 0)
  }
  return String(a || '').localeCompare(String(b || ''))
}

export function priceUnit(tier = {}) {
  switch (tier.display_unit) {
    case 'kg':
      return 'kg'
    case 'lb':
      return '磅'
    case 'g227':
      return '227g'
    case 'g100':
      return '100g'
    case 'g250':
      return '250g'
    default:
      break
  }
  const displayUnit = String(tier.display_unit || '').trim()
  if (displayUnit) return displayUnit
  const specG = Number(tier.spec_g || 454)
  if (specG === 1000) return 'kg'
  if (specG === 227) return '227g'
  if (specG === 100) return '100g'
  if (specG === 250) return '250g'
  return '包'
}

export function splitHighlightedText(text, terms = []) {
  const source = String(text || '')
  const needles = normalizeStringList(terms).sort((a, b) => b.length - a.length)
  if (!source || needles.length === 0) return [{ text: source, red: false }]
  const parts = []
  let index = 0
  while (index < source.length) {
    let match = null
    let matchIndex = source.length
    for (const term of needles) {
      const i = source.indexOf(term, index)
      if (i >= 0 && i < matchIndex) {
        matchIndex = i
        match = term
      }
    }
    if (!match) {
      parts.push({ text: source.slice(index), red: false })
      break
    }
    if (matchIndex > index) {
      parts.push({ text: source.slice(index, matchIndex), red: false })
    }
    parts.push({ text: match, red: true })
    index = matchIndex + match.length
  }
  return parts.filter((part) => part.text)
}

export function copyBeanListPublicationConfig(publication = {}, currentOptions = {}, available = {}) {
  const config = publication.config && typeof publication.config === 'object' ? publication.config : {}
  const listType = config.listType === 'drip' || publication.list_type === 'drip' ? 'drip' : config.listType === 'retail' || publication.list_type === 'retail' ? 'retail' : 'commercial'
  const options = {
    ...sanitizeBeanListPdfTheme({
      ...currentOptions,
      ...config,
      listType,
      changelog: config.changelog || publication.changelog || currentOptions.changelog || '',
    }),
    showCategoryNumbers: config.showCategoryNumbers !== false,
  }
  const productIDs = copySelectedValues(config, 'selectedProductIDs', available.productIDs)
  const categoryCodes = copySelectedValues(config, 'visibleCategoryCodes', available.categoryCodes)
  return {
    options,
    selectedProductIDs: productIDs,
    visibleCategoryCodes: categoryCodes,
    customizers: copyCustomizers(config.customizers, new Set(productIDs)),
  }
}

export function copyBeanListPublicationContentGroups(publication = {}, options = {}) {
  const groups = publication?.content?.groups
  if (!Array.isArray(groups)) return []
  const copied = JSON.parse(JSON.stringify(groups))
  const listType = normalizeBeanListType(options.listType || publication.list_type)
  if (listType === 'green') {
    applyGreenOverridesToCopiedGroups(copied, options.customizers)
  }
  return copied
}

export function beanListPublicationPdfOptions(publication = {}, currentOptions = {}) {
  const config = publication.config && typeof publication.config === 'object' ? publication.config : {}
  const listType = normalizeBeanListType(config.listType || publication.list_type || publication.listType)
  const version = String(publication.version || publication.version_no || publication.versionNo || config.version || currentOptions.version || DEFAULT_BEAN_LIST_PDF_VERSION).trim()
  const changelog = String(config.changelog || publication.changelog || currentOptions.changelog || '').trim()
  const showCategoryNumbers = Object.prototype.hasOwnProperty.call(config, 'showCategoryNumbers')
    ? config.showCategoryNumbers !== false
    : currentOptions.showCategoryNumbers !== false
  return {
    ...sanitizeBeanListPdfTheme({
      ...currentOptions,
      ...config,
      listType,
      version,
      changelog,
    }),
    showCategoryNumbers,
  }
}

function applyGreenOverridesToCopiedGroups(groups = [], customizers = {}) {
  groups.forEach((group) => {
    const items = Array.isArray(group?.items) ? group.items : []
    items.forEach((item) => {
      if (!Array.isArray(item?.green_bean_sale_tiers)) return
      const customizer = customizerFor(item, customizers && typeof customizers === 'object' ? customizers : {})
      item.green_bean_sale_tiers = pdfTierSnapshots(item, 'green_bean_sale_tiers', 'green', customizer)
      item.prices = pdfPriceRows(item, 'green_bean_sale_tiers', 'green', customizer)
    })
  })
}

function normalizeColor(value, fallback) {
  const v = String(value || '').trim()
  return /^#[0-9a-fA-F]{6}$/.test(v) ? v : fallback
}

function clampInt(value, fallback, min, max) {
  const n = Number.parseInt(String(value ?? ''), 10)
  if (!Number.isFinite(n)) return fallback
  return Math.max(min, Math.min(max, n))
}

function normalizeStringSet(values) {
  return new Set(normalizeStringList(values))
}

function normalizeStringList(values) {
  const raw = Array.isArray(values) ? values : String(values || '').split(/[\n,，]/)
  return raw.map((value) => String(value ?? '').trim()).filter(Boolean)
}

function stringField(value) {
  return String(value ?? '').trim()
}

function customerAliasBeanListItem(item, alias, customerID) {
  const productID = Number(item?.product_id ?? item?.productID ?? item?.productId ?? item?.id ?? alias?.product_id ?? 0)
  const displayName = stringField(alias?.brand_name ?? alias?.brandName) || stringField(alias?.display_name ?? alias?.displayName ?? item?.name)
  const productName = stringField(item?.product_name ?? item?.productName ?? item?.name)
  return {
    ...item,
    customer_id: customerID,
    customer_product_alias_id: Number(alias?.id || 0),
    customer_product_display_name: displayName,
    customer_item_code: stringField(alias?.customer_item_code ?? alias?.customerItemCode),
    brand_name: stringField(alias?.brand_name ?? alias?.brandName),
    display_category_id: Number(alias?.display_category_id ?? alias?.displayCategoryID ?? 0),
    display_category_name: stringField(alias?.display_category_name ?? alias?.displayCategoryName ?? alias?.category_name ?? alias?.categoryName),
    product_id: productID,
    product_code: stringField(item?.product_code ?? item?.productCode) || (productID > 0 ? `SKU-${productID}` : ''),
    product_name: productName,
    name: displayName || productName || item?.name || '',
    commercial_bean_list: aliasBeanListMeta(item?.commercial_bean_list ?? item?.commercialBeanList, displayName),
    retail_bean_list: aliasBeanListMeta(item?.retail_bean_list ?? item?.retailBeanList, displayName),
    drip_bean_list: aliasBeanListMeta(item?.drip_bean_list ?? item?.dripBeanList, displayName),
    green_bean_list: aliasBeanListMeta(item?.green_bean_list ?? item?.greenBeanList, displayName),
  }
}

function aliasBeanListMeta(meta, displayName) {
  if (!meta || typeof meta !== 'object' || Array.isArray(meta)) return meta || {}
  return {
    ...meta,
    ...(displayName ? { display_name: displayName } : {}),
  }
}

function beanListItemPriceSource(item, listType, tierKey) {
  return {
    product_id: firstNumber(item.product_id, item.productID, item.productId, item.id),
    customer_product_alias_id: firstNumber(item.customer_product_alias_id, item.customerProductAliasID),
    customer_id: firstNumber(item.customer_id, item.customerID),
    list_type: normalizeBeanListType(listType),
    tier_key: tierKey,
    gradient_template_id: firstNumber(item.gradient_template?.id, item.gradientTemplate?.id),
    drip_price_template_id: firstNumber(item.drip_price_template?.id, item.dripPriceTemplate?.id),
    bom_version_id: firstNumber(item.bom_version_id, item.bomVersionID),
    bom_usage_mode: stringField(item.bom_usage_mode ?? item.bomUsageMode),
    yield_rate: firstNumber(item.yield_rate, item.yieldRate),
    bom_cost_per_unit: firstNumber(item.bom_cost_per_unit, item.bomCostPerUnit),
    operation_cost_per_unit: firstNumber(item.operation_cost_per_unit, item.operationCostPerUnit),
    operation_cost_per_kg: firstNumber(item.operation_cost_per_kg, item.operationCostPerKg),
    green_bean_cost_per_kg: firstNumber(item.green_bean_cost_per_kg, item.greenBeanCostPerKg),
  }
}

function copySelectedValues(config, key, validValues = []) {
  const valid = normalizeStringList(validValues)
  const validSet = new Set(valid)
  if (!Object.prototype.hasOwnProperty.call(config, key)) return valid
  return normalizeStringList(config[key]).filter((value) => validSet.has(value))
}

function copyCustomizers(value, validProductIDs) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const out = {}
  Object.entries(value).forEach(([key, customizer]) => {
    const id = String(key)
    if (!validProductIDs.has(id) || !customizer || typeof customizer !== 'object' || Array.isArray(customizer)) return
    const badge = normalizeBadge(customizer.badge)
    const highlightTerms = Array.isArray(customizer.highlightTerms)
      ? normalizeStringList(customizer.highlightTerms)
      : String(customizer.highlightTerms || '').trim()
    out[id] = {
      ...(badge ? { badge } : {}),
      ...(Array.isArray(highlightTerms) ? { highlightTerms } : highlightTerms ? { highlightTerms } : {}),
      ...copyGreenPriceOverrides(customizer.greenPriceOverrides),
    }
  })
  return out
}

function productIDOf(item) {
  return String(item?.product_id ?? item?.productID ?? item?.productId ?? item?.id ?? item?.name ?? '')
}

function copyGreenPriceOverrides(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const overrides = {}
  Object.entries(value).forEach(([key, price]) => {
    const value = Number(price)
    if (String(key).trim() && Number.isFinite(value) && value > 0) {
      overrides[String(key)] = value
    }
  })
  return Object.keys(overrides).length ? { greenPriceOverrides: overrides } : {}
}

function firstCodePart(code) {
  return String(code || '').split('.')[0].trim()
}

function renumberItemCode(code, categoryCode) {
  const parts = String(code || '').split('.')
  if (parts.length <= 1) return categoryCode || String(code || '')
  return [categoryCode, ...parts.slice(1)].join('.')
}

function renumberCategory(category, categoryCode) {
  const text = String(category || '').trim()
  if (!categoryCode) return text
  const matched = text.match(/^\d+([、.．\-\s]+)(.*)$/)
  if (matched) return `${categoryCode}${matched[1]}${matched[2]}`
  return `${categoryCode}、${text}`
}

function customizerFor(item, customizers) {
  const key = productIDOf(item)
  return customizers[key] || customizers[item?.name] || {}
}

function normalizeBadge(value) {
  const badge = String(value || '').trim()
  return ['new', 'thumb', 'medal'].includes(badge) ? badge : ''
}

function badgeLabel(badge) {
  if (badge === 'new') return 'NEW'
  if (badge === 'thumb') return '👍'
  if (badge === 'medal') return '🏅'
  return ''
}
