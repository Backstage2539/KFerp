import { priceTablePricingRuleTrialPayload, buildPricingRulePayload } from './product-settings.js'

function positiveID(...values) {
  for (const value of values) {
    const id = Number(value || 0)
    if (Number.isFinite(id) && id > 0) return id
  }
  return 0
}

function candidateIdentity(row = {}) {
  const parentID = positiveID(row.parent_product_id, row.parentProductID, row.product_id, row.productID)
  const bomID = positiveID(row.bom_id, row.bomID)
  const versionID = positiveID(row.bom_version_id, row.bomVersionID)
  const specID = positiveID(row.bom_spec_id, row.bomSpecID, row.default_bom_spec_id, row.defaultBOMSpecID)
  const variantID = positiveID(row.bom_variant_id, row.bomVariantID, row.default_bom_variant_id, row.defaultBOMVariantID)
  const skuID = positiveID(row.sku_id, row.skuID, row.skuId, row.product_id, row.productID)
  if (parentID && (specID || variantID)) return JSON.stringify(['bom', parentID, bomID, versionID, specID, variantID])
  const specKey = String(row.spec_key ?? row.specKey ?? row.spec_name ?? row.specName ?? row.sku_name ?? row.skuName ?? '').trim()
  if (parentID && (skuID || specKey)) return JSON.stringify(['sku', parentID, skuID, specKey, String(row.inventory_unit ?? row.inventoryUnit ?? '')])
  return ''
}

function effectiveRuleID(row = {}) {
  return positiveID(row.tier_pricing_rule_id, row.tierPricingRuleID, row.pricing_rule_id, row.pricingRuleID)
}

export function priceTemplateTrialCandidates(sourceRows = []) {
  const byKey = new Map()
  for (const row of Array.isArray(sourceRows) ? sourceRows : []) {
    const key = candidateIdentity(row)
    if (!key) continue
    const current = byKey.get(key)
    const ruleID = effectiveRuleID(row)
    if (current) {
      if (ruleID && !current.ruleIDs.includes(ruleID)) current.ruleIDs.push(ruleID)
      continue
    }
    byKey.set(key, {
      key,
      row,
      ruleIDs: ruleID ? [ruleID] : [],
      parentProductID: positiveID(row.parent_product_id, row.parentProductID, row.product_id, row.productID),
      skuID: positiveID(row.sku_id, row.skuID, row.skuId, row.product_id, row.productID),
      productName: String(row.parent_product_name ?? row.parentProductName ?? row.product_name ?? row.productName ?? '').trim(),
      specName: String(row.spec_name ?? row.specName ?? row.sku_name ?? row.skuName ?? row.spec_key ?? row.specKey ?? '').trim(),
    })
  }
  return [...byKey.values()]
}

export function selectPriceTemplateTrialCandidate(candidates = [], selectedKey = '', pricingRuleID = 0) {
  const rows = Array.isArray(candidates) ? candidates : []
  const existing = rows.find((row) => row.key === String(selectedKey || ''))
  if (existing) return existing
  const preferred = Number(pricingRuleID || 0) > 0
    ? rows.find((row) => row.ruleIDs.includes(Number(pricingRuleID)))
    : null
  return preferred || rows[0] || null
}

export function priceTemplateTrialPayload(candidate, form = {}, customerID = 0) {
  if (!candidate?.row) return null
  const rule = buildPricingRulePayload(form)
  if (!(rule.id > 0)) return null
  const row = {
    ...candidate.row,
    pricing_mode: 'pricing_rule',
    pricing_rule_id: rule.id,
    tier_pricing_rule_id: 0,
    tier_pricing_mode: 'pricing_rule',
  }
  const payload = priceTablePricingRuleTrialPayload(row, { customerID })
  if (!payload) return null
  payload.pricing_rule_id = rule.id
  payload.pricing_rule_draft = rule
  delete payload.overrides
  return payload
}

export function createPriceTemplateTrialRunner({ request, onState = () => {}, setTimer = setTimeout, clearTimer = clearTimeout } = {}) {
  let timer = null
  let generation = 0
  let state = { loading: false, result: null, error: '' }
  const publish = (next) => { state = next; onState({ ...state }) }
  const cancelTimer = () => {
    if (timer !== null) clearTimer(timer)
    timer = null
  }
  return {
    schedule(payload) {
      generation += 1
      const current = generation
      cancelTimer()
      publish({ loading: false, result: null, error: '' })
      if (!payload || typeof request !== 'function') return
      publish({ loading: true, result: null, error: '' })
      timer = setTimer(async () => {
        timer = null
        try {
          const result = await request(payload)
          if (current === generation) publish({ loading: false, result, error: '' })
        } catch (error) {
          if (current === generation) publish({ loading: false, result: null, error: error?.message || '价格试算失败' })
        }
      }, 250)
    },
    cancel() {
      generation += 1
      cancelTimer()
      publish({ loading: false, result: null, error: '' })
    },
  }
}
