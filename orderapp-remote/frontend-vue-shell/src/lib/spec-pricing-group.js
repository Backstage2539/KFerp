function token(value) {
  return String(value ?? '').trim().toLowerCase()
}

// Match only explicit manufacturing identities; names and weights are not identities.
export function resolveSpecPricingGroup(group, specs = []) {
  const result = { items: [], errors: [] }
  if (!group || group.active === false) return { ...result, errors: ['规格计价组不存在或已停用'] }
  if (group.template_available === false) return { ...result, errors: ['规格模板已停用或发布版本不可用'] }
  if (!Array.isArray(group.items) || !group.items.length) return { ...result, errors: ['规格计价组没有配置规格'] }
  for (const item of group.items) {
    const matches = specs.filter((spec) =>
      Number(spec.source_spec_template_id || 0) === Number(group.spec_template_id || 0) &&
      token(spec.source_spec_template_key) === token(item.spec_key) &&
      token(spec.inventory_unit) === token(item.inventory_unit))
    if (!(Number(group.spec_template_id) > 0) || !token(item.spec_key) || !token(item.inventory_unit) || matches.length !== 1 || !(Number(item.pricing_rule_id) > 0)) {
      result.errors.push(`规格 ${item.spec_name || item.spec_key || '未命名'} 不匹配或价格模板无效`)
      continue
    }
    result.items.push({ spec: matches[0], item })
  }
  return result
}
