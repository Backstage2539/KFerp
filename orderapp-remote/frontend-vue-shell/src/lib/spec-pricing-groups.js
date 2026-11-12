export function buildSpecPricingGroupPayload(form = {}) {
  return {
    id: Number(form.id || 0),
    name: String(form.name || '').trim(),
    spec_template_id: Number(form.spec_template_id || 0),
    spec_template_version_id: Number(form.spec_template_version_id || 0),
    revision: Number(form.revision || 0),
    active: form.active !== false,
    items: (form.items || []).map((item, index) => ({
      spec_key: String(item.spec_key || ''),
      inventory_unit: String(item.inventory_unit || ''),
      pricing_rule_id: Number(item.pricing_rule_id || 0),
      sort_order: index + 1,
    })),
  }
}

export function validateSpecPricingGroup(form = {}, templates = [], rules = []) {
  const group = buildSpecPricingGroupPayload(form)
  if (!group.name) return '请输入计价组名称'
  const template = templates.find((row) => Number(row.id) === group.spec_template_id)
  const version = template?.versions?.find((row) => Number(row.id) === group.spec_template_version_id)
  if (!version) return '请选择有效的已发布规格模板版本'
  if (!group.items.length) return '请至少选择一个规格'
  const seen = new Set()
  for (const item of group.items) {
    const key = `${item.spec_key.trim().toLowerCase()}\u0000${item.inventory_unit.trim().toLowerCase()}`
    if (seen.has(key)) return '规格不能重复'
    seen.add(key)
    if (!version.variants?.some((row) => String(row.spec_key || '').trim().toLowerCase() === item.spec_key.trim().toLowerCase() && String(row.inventory_unit || '').trim().toLowerCase() === item.inventory_unit.trim().toLowerCase())) return '规格不属于所选模板版本'
    if (!rules.some((row) => Number(row.id) === item.pricing_rule_id && row.active !== false)) return '请为每个规格选择有效价格模板'
  }
  return ''
}

export function canDisableUnchangedSpecPricingGroup(form = {}, saved = null) {
  if (!saved || Number(form.id) <= 0 || Number(form.id) !== Number(saved.id) || form.active !== false || saved.active === false || !String(form.name || '').trim()) return false
  const current = buildSpecPricingGroupPayload(form)
  const previous = buildSpecPricingGroupPayload(saved)
  return current.spec_template_id === previous.spec_template_id && current.spec_template_version_id === previous.spec_template_version_id && JSON.stringify(current.items) === JSON.stringify(previous.items)
}

const navigationDrafts = new Map()
let navigationSequence = 0
export function saveSpecPricingGroupReturnDraft(form) {
  const key = `spec-pricing-group-${++navigationSequence}`
  navigationDrafts.set(key, JSON.parse(JSON.stringify(form)))
  return key
}
export function consumeSpecPricingGroupReturnDraft(key) {
  const form = navigationDrafts.get(key)
  navigationDrafts.delete(key)
  return form || null
}
