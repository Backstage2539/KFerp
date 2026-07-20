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

export async function executePriceListPricingRuleTrialBatches(sourceRequests = [], options = {}) {
  const requests = Array.isArray(sourceRequests) ? sourceRequests : []
  const sendBatch = options?.sendBatch
  if (!requests.length) return {}
  if (typeof sendBatch !== 'function') throw new Error('sendBatch required')

  const requestedChunkSize = Math.floor(Number(options?.chunkSize || 100))
  const chunkSize = requestedChunkSize > 0 ? requestedChunkSize : 100
  const timeoutMs = Math.max(1, Number(options?.timeoutMs || 30000))
  const createAbortController = typeof options?.createAbortController === 'function'
    ? options.createAbortController
    : () => new AbortController()
  const scheduleTimeout = typeof options?.scheduleTimeout === 'function'
    ? options.scheduleTimeout
    : (callback, delay) => globalThis.setTimeout(callback, delay)
  const cancelTimeout = typeof options?.cancelTimeout === 'function'
    ? options.cancelTimeout
    : (timerID) => globalThis.clearTimeout(timerID)
  const timeoutError = String(options?.timeoutError || '价格计算超时，请重新试算').trim()
  const fallbackError = String(options?.fallbackError || 'pricing rule trial failed').trim()
  const completed = {}

  for (let start = 0; start < requests.length; start += chunkSize) {
    const chunk = requests.slice(start, start + chunkSize)
    const controller = createAbortController()
    const timeoutID = scheduleTimeout(() => controller.abort(), timeoutMs)
    try {
      const response = await sendBatch(chunk.map(({ payload }) => payload), { signal: controller.signal })
      const responseRows = Array.isArray(response?.rows) ? response.rows : []
      const responseByIndex = new Map(responseRows.map((row, index) => [row?.index === undefined ? index : Number(row.index), row]))
      chunk.forEach(({ key }, index) => {
        const row = responseByIndex.get(index)
        const result = row?.result
        const finalUnitPrice = Number(result?.final_unit_price ?? result?.finalUnitPrice)
        if (result && Number.isFinite(finalUnitPrice) && finalUnitPrice > 0) {
          completed[key] = { status: 'success', result }
          return
        }
        const warnings = Array.isArray(result?.warnings)
          ? result.warnings.map((warning) => String(warning || '').trim()).filter(Boolean)
          : []
        completed[key] = {
          status: 'error',
          error: row?.error || warnings.join('；') || (result ? '试算未生成有效价格' : fallbackError),
        }
      })
    } catch (err) {
      chunk.forEach(({ key }) => {
        completed[key] = {
          status: 'error',
          error: err?.name === 'AbortError' ? timeoutError : (err?.message || fallbackError),
        }
      })
    } finally {
      cancelTimeout(timeoutID)
    }
  }
  return completed
}

export function priceListPricingRuleTrialCacheForRetry(sourceCache = {}, sourceKeys = []) {
  const cache = sourceCache && typeof sourceCache === 'object' && !Array.isArray(sourceCache) ? sourceCache : {}
  const next = { ...cache }
  ;(Array.isArray(sourceKeys) ? sourceKeys : []).forEach((rawKey) => {
    const key = String(rawKey || '').trim()
    if (key && next[key]?.status === 'error') delete next[key]
  })
  return next
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
    if (priceListFlatRowIsManualAdjusted(row) && !priceListFlatRowIsManualAdjusted(out[existingIndex])) {
      out[existingIndex] = row
    }
  })
  return out
}

export function priceListFlatRowsReady(sourceRows = [], options = {}) {
  const rows = Array.isArray(sourceRows) ? sourceRows : []
  const trialStatusForRow = typeof options?.trialStatusForRow === 'function' ? options.trialStatusForRow : null
  return rows.length > 0 && rows.every((row) => {
    if (priceListFlatRowErrors(row).length > 0) return false
    if (!trialStatusForRow || !priceListFlatRowUsesLiveTrial(row) || priceListFlatRowIsManualAdjusted(row)) return true
    return String(trialStatusForRow(row) || '').trim() === 'success'
  })
}

export function priceListSalesSpecCountTierLabel(tier = {}) {
  const minQty = Number(tier?.min_qty ?? tier?.minQty ?? 0)
  const maxRaw = tier?.max_qty ?? tier?.maxQty
  const maxQty = maxRaw === null || maxRaw === undefined || maxRaw === '' ? null : Number(maxRaw)
  if (!Number.isFinite(minQty) || minQty < 0) return ''
  const formatQty = (value) => Number.isInteger(value) ? String(value) : String(Number(value.toFixed(4)))
  if (Number.isFinite(maxQty) && maxQty > minQty) return `${formatQty(minQty)}-${formatQty(maxQty)}件`
  if (Number.isFinite(maxQty)) return `${formatQty(minQty)}件`
  return `${formatQty(minQty)}件+`
}

export function priceListFlatRowErrors(row = {}, options = {}) {
  const title = priceListFlatRowContextLabel(row)
  if (!priceListFlatRowUsesSalesSpecCount(row) && (row?.tier_unit_compatible === false || row?.tierUnitCompatible === false)) {
    const detail = String(row?.tier_unit_compatibility_error || row?.tierUnitCompatibilityError || '').trim()
    const productUnit = String(row?.product_sales_spec_unit || row?.productSalesSpecUnit || row?.price_unit || row?.priceUnit || '-').trim() || '-'
    const tierUnit = String(row?.tier_quantity_unit || row?.tierQuantityUnit || '-').trim() || '-'
    return [`${title}：${detail || `阶梯模板不可用：商品规格“${productUnit}”与阶梯规格“${tierUnit}”不匹配`}`]
  }
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

  if (Number(row?.final_unit_price || row?.finalUnitPrice || 0) <= 0 && String(options?.trialStatus || '').trim() !== 'loading') {
    errors.push(`${title}：最终价必须大于 0`)
  }
  if (priceListFlatRowUsesLiveTrial(row) && !priceListFlatRowIsManualAdjusted(row) && String(options?.trialStatus || '').trim() === 'error') {
    const detail = String(options?.trialError || '').trim()
    errors.push(`${title}：价格计算失败${detail ? `：${detail}` : ''}`)
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

function priceListFlatRowUsesLiveTrial(row = {}) {
  if (!priceListFlatRowUsesSalesSpecCount(row) && (row?.tier_unit_compatible === false || row?.tierUnitCompatible === false)) return false
  const mode = String(row?.pricing_mode || row?.pricingMode || '').trim()
  return mode === 'pricing_rule' || mode === 'tier_template'
}

function priceListFlatRowUsesSalesSpecCount(row = {}) {
  return String(row?.quantity_basis || row?.quantityBasis || '').trim() === 'sales_spec_count'
}

export function priceListFlatRowDisplayTitle(row = {}) {
  const productName = String(row?.product_name || row?.productName || row?.name || '').trim()
  return productName || '未命名商品'
}

export function priceListFlatRowSpecDescription(row = {}) {
  const spec = priceListFlatRowPriceUnitLabel(row)
  return `规格：${spec || '-'}`
}

export function priceListFlatRowPriceUnitLabel(row = {}) {
  const priceUnit = String(row?.price_unit || row?.priceUnit || '').trim()
  const spec = priceListFlatRowUnitSpecLabel(row)
  if (spec && priceListFlatRowUsesSalesSpecCount(row)) return compactSpecLabelFromText(spec) || spec
  if (spec && (priceUnit === '袋' || priceUnit === '包' || priceUnit === '个' || priceUnit === '盒' || priceUnit === 'unit')) {
    return spec
  }
  const compact = compactSpecLabelFromText(priceUnit)
  if (compact) return compact
  return priceUnit || '-'
}

export function priceListFlatRowContextLabel(row = {}) {
  const productName = priceListFlatRowDisplayTitle(row)
  const spec = priceListFlatRowPriceUnitLabel(row)
  if (!spec || spec === '-') return productName
  return `${productName} / 规格：${spec}`
}

function priceListFlatRowUnitSpecLabel(row = {}) {
  const snapshot = parsePlainObject(row?.sku_snapshot ?? row?.skuSnapshot)
  const effectiveSpec = parsePlainObject(row?.effective_sales_spec ?? row?.effectiveSalesSpec)
  const candidates = [
    effectiveSpec?.spec_label,
    effectiveSpec?.specLabel,
    effectiveSpec?.spec_name,
    effectiveSpec?.specName,
    row?.spec_label,
    row?.specLabel,
    snapshot?.spec_label,
    snapshot?.specLabel,
    row?.tier_quantity_unit,
    row?.tierQuantityUnit,
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
  const templateTierID = Number(row?.template_tier_id || row?.templateTierID || 0)
  const pricingRuleID = Number(row?.tier_pricing_rule_id || row?.tierPricingRuleID || row?.pricing_rule_id || row?.pricingRuleID || 0)
  const priceUnit = String(row?.price_unit || row?.priceUnit || '').trim()
  if (!productKey || templateID <= 0 || pricingRuleID <= 0 || !priceUnit) return ''
  const tierKey = templateTierID > 0
    ? `id:${templateTierID}`
    : `range:${String(row?.tier_label || row?.tierLabel || '').trim()}:${Number(row?.min_qty ?? row?.minQty ?? 0)}:${nullableTierLimit(row?.max_qty ?? row?.maxQty)}`
  return `${productKey}:tier-template:${templateID}:tier:${tierKey}:pricing-rule:${pricingRuleID}:unit:${priceUnit}`
}

function nullableTierLimit(value) {
  if (value === undefined || value === null || value === '') return 'open'
  const number = Number(value)
  return Number.isFinite(number) ? String(number) : String(value).trim()
}

function priceListFlatRowIsManualAdjusted(row = {}) {
  if (row?.manual_adjusted === true || row?.manualAdjusted === true) return true
  const finalRaw = row?.final_unit_price ?? row?.finalUnitPrice
  const originalRaw = row?.original_final_unit_price ?? row?.originalFinalUnitPrice
  if (finalRaw === undefined || finalRaw === null || finalRaw === '' || originalRaw === undefined || originalRaw === null || originalRaw === '') return false
  const finalPrice = Number(finalRaw)
  const originalPrice = Number(originalRaw)
  return Number.isFinite(finalPrice) && Number.isFinite(originalPrice) && Math.abs(finalPrice - originalPrice) > 0.005
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
