export const DEFAULT_BEAN_LIST_PDF_VERSION = 'V3.0.5'

export function sanitizeBeanListPdfTheme(input = {}) {
  const listType = input.listType === 'retail' || input.listType === 'drip' ? input.listType : 'commercial'
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
  if (listType === 'drip') return `${brand}挂耳豆单`
  return listType === 'retail' ? `${brand}零售豆单` : `${brand}批发豆单`
}

export function filterBeanListItemsForScope(items = [], scope = 'official', customerID = 0) {
  const selectedCustomerID = Number(customerID || 0)
  return (Array.isArray(items) ? items : []).filter((item) => {
    const itemCustomerID = Number(item?.customer_id ?? item?.customerID ?? 0)
    if (scope === 'customer') {
      return itemCustomerID <= 0 || (selectedCustomerID > 0 && itemCustomerID === selectedCustomerID)
    }
    return itemCustomerID <= 0
  })
}

export function buildBeanListPdfSubtitle(listType) {
  if (listType === 'drip') return '挂耳供应价，报价按袋/盒快照发布'
  return listType === 'retail' ? '报价含税运' : '报价不含税、不含运'
}

export function buildBeanListPdfGroups(items = [], listType = 'commercial', options = {}) {
  const metaKey = listType === 'retail' ? 'retail_bean_list' : listType === 'drip' ? 'drip_bean_list' : 'commercial_bean_list'
  const tierKey = listType === 'retail' ? 'retail_bean_tiers' : listType === 'drip' ? 'drip_wholesale_tiers' : 'commercial_wholesale_tiers'
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
      items: sourceRows.map((item, index) => buildPdfItem(item, metaKey, tierKey, listType, String(index + 1), customizers)),
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
    groups.get(key).items.push(buildPdfItem(item, metaKey, tierKey, listType, renumberItemCode(meta.code, categoryCode), customizers))
  })
  return Array.from(groups.values())
}

function buildPdfItem(item, metaKey, tierKey, listType, code, customizers) {
  const meta = item[metaKey] || {}
  const customizer = customizerFor(item, customizers)
  const highlightTerms = normalizeStringList(customizer.highlightTerms)
  const badge = normalizeBadge(customizer.badge)
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
    prices: (Array.isArray(item[tierKey]) ? item[tierKey] : []).map((tier) => {
      const label = tier.label || ''
      return {
        label,
        price: Number(tier.price_per_unit || tier.price_per_lb || 0),
        unit: listType === 'retail' ? '' : listType === 'drip' ? dripPriceUnit(tier) : priceUnit(tier),
        red: false,
      }
    })
  }
}

function dripPriceUnit(tier = {}) {
  if (tier.sales_unit === 'box') return `盒(${Number(tier.unit_bag_count || 10)}袋)`
  return '袋'
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

export function copyBeanListPublicationContentGroups(publication = {}) {
  const groups = publication?.content?.groups
  if (!Array.isArray(groups)) return []
  return JSON.parse(JSON.stringify(groups))
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
    }
  })
  return out
}

function productIDOf(item) {
  return String(item?.product_id ?? item?.productID ?? item?.id ?? item?.name ?? '')
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
