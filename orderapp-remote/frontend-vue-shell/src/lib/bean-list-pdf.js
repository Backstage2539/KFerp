export const DEFAULT_BEAN_LIST_PDF_VERSION = 'V3.0.5'

export function sanitizeBeanListPdfTheme(input = {}) {
  const listType = input.listType === 'retail' ? 'retail' : 'commercial'
  return {
    listType,
    version: String(input.version || DEFAULT_BEAN_LIST_PDF_VERSION).trim() || DEFAULT_BEAN_LIST_PDF_VERSION,
    backgroundColor: normalizeColor(input.backgroundColor, '#f8f1e5'),
    fontColor: normalizeColor(input.fontColor, '#171717'),
    backgroundImage: String(input.backgroundImage || '').trim(),
  }
}

export function buildBeanListPdfTitle(listType) {
  return listType === 'retail' ? '棵凡咖啡零售豆单' : '棵凡咖啡批发豆单'
}

export function buildBeanListPdfSubtitle(listType) {
  return listType === 'retail' ? '报价含税运' : '报价不含税、不含运'
}

export function buildBeanListPdfGroups(items = [], listType = 'commercial') {
  const metaKey = listType === 'retail' ? 'retail_bean_list' : 'commercial_bean_list'
  const tierKey = listType === 'retail' ? 'retail_bean_tiers' : 'commercial_wholesale_tiers'
  const groups = new Map()
  items
    .filter((item) => item?.[metaKey]?.code)
    .slice()
    .sort((a, b) => compareBeanCodes(a[metaKey].code, b[metaKey].code))
    .forEach((item) => {
      const meta = item[metaKey] || {}
      const category = meta.category || '未分类'
      if (!groups.has(category)) {
        groups.set(category, { category, items: [] })
      }
      groups.get(category).items.push({
        code: meta.code || '',
        name: meta.display_name || item.name || '',
        recommendedUse: meta.recommended_use || '',
        flavor: meta.flavor || item.flavor || '',
        description: meta.description || item.bean_list_note || '',
        prices: (Array.isArray(item[tierKey]) ? item[tierKey] : []).map((tier) => ({
          label: tier.label || '',
          price: Number(tier.price_per_unit || tier.price_per_lb || 0),
          unit: listType === 'retail' ? '' : priceUnit(tier),
        })),
      })
    })
  return Array.from(groups.values())
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
  const specG = Number(tier.spec_g || 454)
  if (specG === 1000) return 'kg'
  if (specG === 227) return '227g'
  return '包'
}

function normalizeColor(value, fallback) {
  const v = String(value || '').trim()
  return /^#[0-9a-fA-F]{6}$/.test(v) ? v : fallback
}
