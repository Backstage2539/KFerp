export function priceListPricingRuleTrialRequestsForRows(sourceRows = [], options = {}) {
  const {
    customerID = 0,
    cache = {},
    payloadForRow,
    cacheKeyForPayload,
  } = options
  if (typeof payloadForRow !== 'function' || typeof cacheKeyForPayload !== 'function') return []

  const requests = []
  const seen = new Set()
  ;(Array.isArray(sourceRows) ? sourceRows : []).forEach((row) => {
    const payload = payloadForRow(row, { customerID })
    if (!payload) return
    const key = cacheKeyForPayload(payload)
    if (!key || seen.has(key)) return
    const cached = cache[key]
    if (cached?.status === 'loading' || cached?.status === 'success' || cached?.status === 'error') return
    seen.add(key)
    requests.push({ row, key, payload })
  })
  return requests
}

export function dedupePriceListFlatRows(sourceRows = []) {
  const rows = Array.isArray(sourceRows) ? sourceRows : []
  const seen = new Map()
  const out = []
  rows.forEach((row) => {
    const key = duplicateTierTemplateFlatRowKey(row)
    if (!key) {
      out.push(row)
      return
    }
    const existingIndex = seen.get(key)
    if (existingIndex === undefined) {
      seen.set(key, out.length)
      out.push(row)
      return
    }
    if (row?.manual_adjusted === true && out[existingIndex]?.manual_adjusted !== true) {
      out[existingIndex] = row
    }
  })
  return out
}

function duplicateTierTemplateFlatRowKey(row = {}) {
  if (String(row?.pricing_mode || row?.pricingMode || '').trim() !== 'tier_template') return ''
  const productKey = flatRowProductKey(row)
  const templateID = Number(row?.tier_template_id || row?.tierTemplateID || 0)
  const pricingRuleID = Number(row?.tier_pricing_rule_id || row?.tierPricingRuleID || row?.pricing_rule_id || row?.pricingRuleID || 0)
  const priceUnit = String(row?.price_unit || row?.priceUnit || '').trim()
  if (!productKey || templateID <= 0 || pricingRuleID <= 0 || !priceUnit) return ''
  return `${productKey}:tier-template:${templateID}:pricing-rule:${pricingRuleID}:unit:${priceUnit}`
}

function flatRowProductKey(row = {}) {
  const skuID = Number(row?.sku_id || row?.skuID || row?.skuId || 0)
  if (skuID > 0) return `sku:${skuID}`
  const productID = Number(row?.product_id || row?.productID || row?.productId || 0)
  if (productID > 0) return `id:${productID}`
  const productKey = String(row?.product_key || row?.productKey || '').trim()
  if (productKey) return `key:${productKey}`
  const productName = String(row?.product_name || row?.productName || row?.name || '').trim()
  return productName ? `name:${productName}` : ''
}
