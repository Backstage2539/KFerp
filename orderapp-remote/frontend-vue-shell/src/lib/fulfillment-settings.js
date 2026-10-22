export function explicitMoneyCents(value) {
  const raw = String(value ?? '').trim()
  if (!raw) throw new Error('费用必须明确填写，免费请填写 0')
  if (!/^\d+(\.\d{1,2})?$/.test(raw)) throw new Error('费用需为非负金额，最多两位小数')
  const cents = Math.round(Number(raw) * 100)
  if (!Number.isSafeInteger(cents) || cents > 999999999999) throw new Error('费用超出可处理范围')
  return cents
}
const money = cents => (Number(cents || 0) / 100).toFixed(2)
export function fulfillmentSettingsForm(data = {}) {
  return { customer_id: Number(data.customer_id || 0), revision: Number(data.revision || 0), modes: ['stock', 'on_demand'].map(mode => {
    const row = (data.modes || []).find(r => r.mode === mode) || {}
    const policy = row.shipping_policy
    return { mode, enabled: row.enabled === true, warehouse_code: row.warehouse_code || '', shipping_free: policy ? policy.free_shipping === true : row.shipping_free === true, shipping_template_id: Number(policy ? policy.template_id || 0 : row.shipping_template_id || 0), shipping_policy_revision: Number(policy?.revision || 0), surcharge: money(policy?.surcharge_cents), fixed_fee: row.fee_configured ? money(row.fixed_fee_cents) : '', per_package_fee: row.fee_configured ? money(row.per_package_fee_cents) : '' }
  }) }
}
export function fulfillmentSettingsPayload(form) {
  return { customer_id: form.customer_id, expected_revision: form.revision, modes: form.modes.map(row => {
    if (!row.enabled) return { mode: row.mode, enabled: false }
    if (!row.shipping_template_id && !row.shipping_free && explicitMoneyCents(row.surcharge) !== 0) throw new Error('继承运费配置时不能单独设置附加运费；请选模板后设置')
    if (row.mode === 'stock' && !row.warehouse_code) throw new Error('库存代发必须绑定此客户的成品仓')
    return { mode: row.mode, enabled: true, warehouse_code: row.mode === 'stock' ? row.warehouse_code : '', shipping_free: row.shipping_free === true, shipping_template_id: Number(row.shipping_template_id || 0), fee_configured: true, fixed_fee_cents: explicitMoneyCents(row.fixed_fee), per_package_fee_cents: explicitMoneyCents(row.per_package_fee), shipping_policy: { template_id: Number(row.shipping_template_id || 0), surcharge_cents: explicitMoneyCents(row.surcharge), free_shipping: row.shipping_free === true, expected_revision: Number(row.shipping_policy_revision || 0) } }
  }) }
}
export function customerFinishedWarehouses(rows, customerID) {
  if (Number(customerID || 0) <= 0) return []
  return (rows || []).filter(r => r.active !== false && Number(r.customer_id) === Number(customerID) && ['finished', 'customer_finished', 'customer_processing', 'customer'].includes(r.kind))
}
export function shippingTemplateForm(data = {}) {
  const rule = r => ({ province: r.province || '*', city: r.city || '', district: r.district || '', first_weight_g: Number(r.first_weight_g || 1000), additional_weight_g: Number(r.additional_weight_g || 1000), first_fee: r.first_fee_cents === undefined ? '' : money(r.first_fee_cents), additional_fee: r.additional_fee_cents === undefined ? '' : money(r.additional_fee_cents) })
  return { id: Number(data.id || 0), revision: Number(data.revision || 0), name: data.name || '', active: data.active !== false, packaging_weight_g: Number(data.packaging_weight_g || 0), rules: data.rules?.length ? data.rules.map(rule) : [rule({})] }
}
export function shippingTemplatePayload(form) {
  if (!form.name.trim() || !Number.isInteger(Number(form.packaging_weight_g)) || Number(form.packaging_weight_g) < 0 || !form.rules.length) throw new Error('请填写名称、有效包装附加重及地区规则')
  const seen = new Set()
  return { id: form.id, expected_revision: form.revision, name: form.name.trim(), active: form.active, packaging_weight_g: Number(form.packaging_weight_g), rules: form.rules.map(r => {
    const province = r.province.trim(), city = r.city.trim(), district = r.district.trim()
    if (!province || !Number.isInteger(Number(r.first_weight_g)) || Number(r.first_weight_g) <= 0 || !Number.isInteger(Number(r.additional_weight_g)) || Number(r.additional_weight_g) <= 0 || district && !city) throw new Error('请填写地区及正整数首重、续重，区县需同时填写城市')
    const key = `${province}|${city}|${district}`
    if (seen.has(key)) throw new Error('地区规则重复')
    seen.add(key)
    return { province, city, district, first_weight_g: Number(r.first_weight_g), additional_weight_g: Number(r.additional_weight_g), first_fee_cents: explicitMoneyCents(r.first_fee), additional_fee_cents: explicitMoneyCents(r.additional_fee) }
  }) }
}
