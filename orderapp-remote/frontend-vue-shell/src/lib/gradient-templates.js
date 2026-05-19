function toNumber(value) {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

function hasValue(value) {
  return !(value === '' || value == null)
}

function roundQuantity(value) {
  const n = Number(value)
  return Number.isFinite(n) ? Number(n.toFixed(3)) : 0
}

export const gradientDisplayUnitOptions = [
  { value: 'lb', label: '元/磅', quantityLabel: '磅', specG: 454 },
  { value: 'kg', label: '元/kg', quantityLabel: 'kg', specG: 1000 },
  { value: 'g227', label: '元/227g', quantityLabel: '227g', specG: 227 },
  { value: 'g100', label: '元/100g', quantityLabel: '100g', specG: 100 },
  { value: 'g250', label: '元/250g', quantityLabel: '250g', specG: 250 },
]

const gradientDisplayUnitByValue = new Map(gradientDisplayUnitOptions.map((unit) => [unit.value, unit]))

export function normalizeGradientDisplayUnit(unit) {
  const value = String(unit || '').trim()
  return gradientDisplayUnitByValue.has(value) ? value : 'lb'
}

export function gradientDisplayUnitLabel(unit) {
  return gradientDisplayUnitByValue.get(normalizeGradientDisplayUnit(unit))?.label || '元/磅'
}

export function gradientDisplayQuantityUnitLabel(unit) {
  return gradientDisplayUnitByValue.get(normalizeGradientDisplayUnit(unit))?.quantityLabel || '磅'
}

export function gradientDisplayUnitSpecG(unit) {
  return gradientDisplayUnitByValue.get(normalizeGradientDisplayUnit(unit))?.specG || 454
}

export function gradientDisplayQuantityStep(unit) {
  const normalized = normalizeGradientDisplayUnit(unit)
  return normalized === 'kg' || normalized === 'lb' ? '0.01' : '1'
}

export function gradientDisplayQtyToWeightG(qty, unit) {
  return roundQuantity(toNumber(qty) * gradientDisplayUnitSpecG(unit))
}

export function gradientWeightGToDisplayQty(weightG, unit) {
  return roundQuantity(toNumber(weightG) / gradientDisplayUnitSpecG(unit))
}

export function normalizeGradientTemplate(template = {}) {
  const displayUnit = normalizeGradientDisplayUnit(template.display_unit)
  const tiers = (template.tiers || [])
    .map((tier, index) => {
      const minDisplayQty = hasValue(tier.min_display_qty)
        ? toNumber(tier.min_display_qty)
        : gradientWeightGToDisplayQty(tier.min_weight_g, displayUnit)
      const hasMaxDisplay = hasValue(tier.max_display_qty)
      const hasMaxWeight = hasValue(tier.max_weight_g)
      const maxDisplayQty = hasMaxDisplay
        ? toNumber(tier.max_display_qty)
        : (hasMaxWeight ? gradientWeightGToDisplayQty(tier.max_weight_g, displayUnit) : null)
      return {
        id: Number(tier.id || 0),
        label: String(tier.label || '').trim(),
        min_display_qty: minDisplayQty,
        max_display_qty: maxDisplayQty,
        min_weight_g: gradientDisplayQtyToWeightG(minDisplayQty, displayUnit),
        max_weight_g: maxDisplayQty == null ? null : gradientDisplayQtyToWeightG(maxDisplayQty, displayUnit),
        margin_rate: toNumber(tier.margin_rate),
        position: Number(tier.position || index + 1),
      }
    })
    .sort((a, b) => a.position - b.position || a.min_weight_g - b.min_weight_g)

  return {
    id: Number(template.id || 0),
    name: String(template.name || '').trim(),
    customer_id: Number(template.customer_id || 0),
    source_template_id: Number(template.source_template_id || 0),
    template_state: String(template.template_state || '').trim(),
    display_unit: displayUnit,
    active: template.active !== false,
    tiers,
  }
}

export function buildGradientTemplatePayload(template = {}) {
  const row = normalizeGradientTemplate(template)
  return {
    id: row.id,
    customer_id: row.customer_id,
    name: row.name,
    display_unit: row.display_unit,
    active: row.active,
    tiers: row.tiers.map((tier) => ({
      id: tier.id,
      label: tier.label,
      min_display_qty: tier.min_display_qty,
      max_display_qty: tier.max_display_qty,
      min_weight_g: tier.min_weight_g,
      max_weight_g: tier.max_weight_g,
      margin_rate: tier.margin_rate,
      position: tier.position,
    })),
  }
}

export function validateGradientTemplate(template = {}) {
  const row = normalizeGradientTemplate(template)
  const errors = []
  if (!row.name) errors.push('请填写模板名称')
  row.tiers.forEach((tier, index) => {
    const label = `第 ${index + 1} 档`
    if (!tier.label) errors.push(`${label}请填写区间名`)
    if (tier.min_display_qty <= 0) errors.push(`${label}最小数量必须大于 0`)
    if (tier.max_display_qty != null && tier.max_display_qty <= tier.min_display_qty) errors.push(`${label}最大数量必须大于最小数量`)
    if (tier.margin_rate < 0) errors.push(`${label}利润率不能为负数`)
  })
  if (!row.tiers.length) errors.push('至少添加一个梯度档位')
  return errors
}

export function buildPriceExplanationRequest(item = {}, tier = {}, overrides = {}) {
  const normalizedOverrides = {}
  for (const [key, value] of Object.entries(overrides || {})) {
    if (value === '' || value == null) continue
    const n = Number(value)
    if (Number.isFinite(n)) normalizedOverrides[key] = n
  }
  return {
    product: {
      product_id: Number(item.product_id || 0),
      name: item.name || '',
      customer_id: Number(item.customer_id || 0),
      base_product_id: Number(item.base_product_id || 0),
      visibility: item.visibility || '',
      custom_type: item.custom_type || '',
      product_category_id: Number(item.product_category_id || 0),
      green_bean_cost_per_kg: Number(item.green_bean_cost_per_kg || 0),
      yield_rate: Number(item.yield_rate || 0),
      bom_status: item.bom_status || '',
      warnings: item.warnings || [],
      gradient_template: item.gradient_template || null,
    },
    tier_label: tier.label || '',
    overrides: normalizedOverrides,
  }
}
