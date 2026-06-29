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

export function priceListFlatRowsReady(sourceRows = []) {
  const rows = Array.isArray(sourceRows) ? sourceRows : []
  return rows.length > 0 && rows.every((row) => priceListFlatRowErrors(row).length === 0)
}

export function priceListFlatRowErrors(row = {}) {
  const title = priceListFlatRowDisplayTitle(row)
  const errors = []
  const mode = String(row?.pricing_mode || row?.pricingMode || '').trim()
  if (!mode) {
    errors.push(`${title}：缺少计价模式`)
  } else if (mode === 'tier_template') {
    const pricingRuleID = Number(row?.pricing_rule_id || row?.pricingRuleID || row?.tier_pricing_rule_id || row?.tierPricingRuleID || 0)
    const pricingRuleVersion = String(row?.pricing_rule_version || row?.pricingRuleVersion || row?.tier_pricing_rule_version || row?.tierPricingRuleVersion || '').trim()
    if (Number(row?.tier_template_id || row?.tierTemplateID || 0) <= 0) errors.push(`${title}：缺少阶梯模板`)
    if (Number(row?.template_tier_id || row?.templateTierID || 0) <= 0) errors.push(`${title}：缺少阶梯档位`)
    if (pricingRuleID <= 0) errors.push(`${title}：缺少计算模板`)
    else if (!pricingRuleVersion) errors.push(`${title}：缺少计算模板版本`)
  } else if (mode === 'pricing_rule') {
    if (Number(row?.pricing_rule_id || row?.pricingRuleID || 0) <= 0) errors.push(`${title}：缺少计算模板`)
    if (!String(row?.pricing_rule_version || row?.pricingRuleVersion || '').trim()) errors.push(`${title}：缺少计算模板版本`)
  } else if (mode === 'fixed_price') {
    if (Number(row?.fixed_unit_price || row?.fixedUnitPrice || 0) <= 0) errors.push(`${title}：缺少固定价`)
  } else {
    errors.push(`${title}：计价模式无效`)
  }

  if (Number(row?.final_unit_price || row?.finalUnitPrice || 0) <= 0) {
    errors.push(`${title}：最终价必须大于 0`)
  }

  const priceUnit = String(row?.price_unit || row?.priceUnit || '').trim()
  const inventoryUnit = String(row?.inventory_unit || row?.inventoryUnit || '').trim()
  if (!priceUnit) errors.push(`${title}：缺少价格单位`)
  if (!inventoryUnit) errors.push(`${title}：缺少库存单位`)
  if (priceUnit && inventoryUnit && !flatRowHasInventoryConversion(row, priceUnit, inventoryUnit)) {
    errors.push(`${title}：缺少 ${priceUnit} 到 ${inventoryUnit} 的换算`)
  }

  if (Object.keys(parsePlainObject(row?.group_snapshot ?? row?.groupSnapshot)).length === 0) {
    errors.push(`${title}：缺少价格表分组快照`)
  }
  if (Object.keys(parsePlainObject(row?.cost_source_snapshot ?? row?.costSourceSnapshot)).length === 0) {
    errors.push(`${title}：缺少成本来源快照`)
  }
  return errors
}

export function priceListFlatRowDisplayTitle(row = {}) {
  const productName = String(row?.product_name || row?.productName || row?.name || '').trim()
  const spec = priceListFlatRowSpecLabel(row)
  if (!productName) return spec || '未命名商品'
  if (!spec || productName.includes(spec)) return productName
  return `${productName}（${spec}）`
}

export function priceListFlatRowPriceUnitLabel(row = {}) {
  const priceUnit = String(row?.price_unit || row?.priceUnit || '').trim()
  const spec = priceListFlatRowUnitSpecLabel(row)
  if (spec && (priceUnit === '袋' || priceUnit === '包' || priceUnit === '个' || priceUnit === '盒' || priceUnit === 'unit')) {
    return spec
  }
  const compact = compactSpecLabelFromText(priceUnit)
  if (compact) return compact
  return priceUnit || '-'
}

function priceListFlatRowSpecLabel(row = {}) {
  const snapshot = parsePlainObject(row?.sku_snapshot ?? row?.skuSnapshot)
  const candidates = [
    row?.sku_name,
    row?.skuName,
    snapshot?.sku_name,
    snapshot?.skuName,
    row?.derived_spec_name,
    row?.derivedSpecName,
    row?.spec_label,
    row?.specLabel,
    snapshot?.spec_label,
    snapshot?.specLabel,
    netContentLabel(row),
    netContentLabel(snapshot),
  ]
  for (const candidate of candidates) {
    const text = String(candidate || '').trim()
    if (text) return text
  }
  return ''
}

function priceListFlatRowUnitSpecLabel(row = {}) {
  const snapshot = parsePlainObject(row?.sku_snapshot ?? row?.skuSnapshot)
  const candidates = [
    row?.spec_label,
    row?.specLabel,
    snapshot?.spec_label,
    snapshot?.specLabel,
    compactSpecLabelFromText(row?.derived_spec_name),
    compactSpecLabelFromText(row?.derivedSpecName),
    compactSpecLabelFromText(row?.sku_name),
    compactSpecLabelFromText(row?.skuName),
    compactSpecLabelFromText(snapshot?.sku_name),
    compactSpecLabelFromText(snapshot?.skuName),
    row?.derived_spec_name,
    row?.derivedSpecName,
    row?.sku_name,
    row?.skuName,
    snapshot?.sku_name,
    snapshot?.skuName,
    netContentLabel(row),
    netContentLabel(snapshot),
  ]
  for (const candidate of candidates) {
    const text = String(candidate || '').trim()
    if (text) return text
  }
  return ''
}

function compactSpecLabelFromText(value = '') {
  const text = String(value || '').trim()
  if (!text) return ''
  const match = text.match(/(\d+(?:\.\d+)?)\s*(kg|g|克|千克|公斤|lb|lbs|磅)/i)
  if (!match) return ''
  const qty = formatNetContentQty(Number(match[1]))
  const unit = normalizeCompactSpecUnit(match[2])
  return qty && unit ? `${qty}${unit}` : ''
}

function normalizeCompactSpecUnit(unit = '') {
  const value = String(unit || '').trim().toLowerCase()
  if (value === '克') return 'g'
  if (value === '千克' || value === '公斤') return 'kg'
  if (value === 'lbs' || value === '磅') return 'lb'
  return value
}

function netContentLabel(row = {}) {
  const qty = Number(row?.net_content_qty ?? row?.netContentQty ?? 0)
  const unit = String(row?.net_content_unit ?? row?.netContentUnit ?? '').trim()
  if (!Number.isFinite(qty) || qty <= 0 || !unit) return ''
  return `${formatNetContentQty(qty)}${unit}`
}

function formatNetContentQty(qty) {
  if (Number.isInteger(qty)) return String(qty)
  return String(Number(qty.toFixed(4))).replace(/\.?0+$/, '')
}

function flatRowHasInventoryConversion(row = {}, priceUnit = '', inventoryUnit = '') {
  if (!priceUnit || !inventoryUnit) return false
  if (priceUnit === inventoryUnit) return true
  const conversion = parsePlainObject(row?.inventory_conversion_json ?? row?.inventoryConversionJSON)
  const targets = conversion?.[priceUnit]
  if (!targets || typeof targets !== 'object' || Array.isArray(targets)) return false
  const factor = Number(targets[inventoryUnit] || 0)
  return Number.isFinite(factor) && factor > 0
}

function parsePlainObject(raw = {}) {
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) return raw
  if (typeof raw === 'string' && raw.trim()) {
    try {
      const parsed = JSON.parse(raw)
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
    } catch {
      return {}
    }
  }
  return {}
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
