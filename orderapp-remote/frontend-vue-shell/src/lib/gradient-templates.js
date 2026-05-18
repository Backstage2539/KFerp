function toNumber(value) {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

export function normalizeGradientDisplayUnit(unit) {
  return unit === 'kg' ? 'kg' : 'lb'
}

export function gradientDisplayUnitLabel(unit) {
  return normalizeGradientDisplayUnit(unit) === 'kg' ? '元/kg' : '元/磅'
}

export function normalizeGradientTemplate(template = {}) {
  const tiers = (template.tiers || [])
    .map((tier, index) => {
      const hasMax = !(tier.max_weight_g === '' || tier.max_weight_g == null)
      const max = hasMax ? toNumber(tier.max_weight_g) : null
      return {
        id: Number(tier.id || 0),
        label: String(tier.label || '').trim(),
        min_weight_g: toNumber(tier.min_weight_g),
        max_weight_g: max,
        margin_rate: toNumber(tier.margin_rate),
        position: Number(tier.position || index + 1),
      }
    })
    .sort((a, b) => a.position - b.position || a.min_weight_g - b.min_weight_g)

  return {
    id: Number(template.id || 0),
    name: String(template.name || '').trim(),
    display_unit: normalizeGradientDisplayUnit(template.display_unit),
    active: template.active !== false,
    tiers,
  }
}

export function validateGradientTemplate(template = {}) {
  const row = normalizeGradientTemplate(template)
  const errors = []
  if (!row.name) errors.push('请填写模板名称')
  row.tiers.forEach((tier, index) => {
    const label = `第 ${index + 1} 档`
    if (!tier.label) errors.push(`${label}请填写区间名`)
    if (tier.min_weight_g <= 0) errors.push(`${label}最小总克重必须大于 0`)
    if (tier.max_weight_g != null && tier.max_weight_g <= tier.min_weight_g) errors.push(`${label}最大总克重必须大于最小总克重`)
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
