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
  const current = (Array.isArray(publications) ? publications : []).find((row) => String(row?.status || '').trim() === 'published')
  const row = current || sourcePublication || null
  return nextBeanListVersion(row?.version || row?.version_no || row?.versionNo || '', fallback)
}

export function sanitizeBeanListPdfTheme(input = {}) {
  const listType = normalizeBeanListType(input.listType)
  return {
    listType,
    version: String(input.version || DEFAULT_BEAN_LIST_PDF_VERSION).trim() || DEFAULT_BEAN_LIST_PDF_VERSION,
    brandName: String(input.brandName || '棵凡咖啡').trim() || '棵凡咖啡',
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
  const brand = String(brandName || '棵凡咖啡').trim() || '棵凡咖啡'
  const normalized = normalizeBeanListType(listType)
  if (normalized === 'green') return `${brand}生豆产品价格表`
  if (normalized === 'drip') return `${brand}挂耳产品价格表`
  return normalized === 'retail' ? `${brand}零售产品价格表` : `${brand}批发产品价格表`
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

function normalizeBeanListType(listType) {
  if (listType === 'retail') return 'retail'
  if (listType === 'drip') return 'drip'
  if (listType === 'green' || listType === 'green_bean') return 'green'
  return 'commercial'
}

function buildPdfItem(item, metaKey, tierKey, listType, code, customizers) {
  const meta = item[metaKey] || {}
  const customizer = customizerFor(item, customizers)
  const highlightTerms = normalizeStringList(customizer.highlightTerms)
  const badge = normalizeBadge(customizer.badge)
  const beanListQuality = normalizeBeanListQuality(item.bean_list_quality || item.beanListQuality)
  const tierSnapshots = pdfTierSnapshots(item, tierKey, listType, customizer)
  return {
    productId: item.product_id || item.productID || item.id || null,
    code: code || meta.code || '',
    originalCode: meta.code || '',
    name: meta.display_name || item.name || '',
    recommendedUse: meta.recommended_use || '',
    flavor: meta.flavor || item.flavor || '',
    description: meta.description || item.bean_list_note || '',
    badge,
    badgeLabel: badgeLabel(badge),
    highlightTerms,
    ...(beanListQuality ? { beanListQuality, qualityLines: beanListQualityLines(beanListQuality) } : {}),
    ...(tierSnapshots.length ? { [tierKey]: tierSnapshots } : {}),
    prices: pdfPriceRows(item, tierKey, listType, customizer)
  }
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

function pdfPriceRows(item, tierKey, listType, customizer = {}) {
  const tiers = pdfTierSnapshots(item, tierKey, listType, customizer)
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
