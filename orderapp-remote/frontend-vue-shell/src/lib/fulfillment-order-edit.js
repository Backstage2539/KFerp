const number = value => Number.isFinite(Number(value)) ? Number(value) : 0
const id = value => Math.max(0, Math.trunc(number(value)))

export function fulfillmentEditFamilies(catalog = {}, mode = 'stock') {
  return (catalog.product_families || []).map(family => {
    const parent = id(family.parent_product_id || family.product_id || family.id)
    const name = family.customer_product_display_name || family.name || family.parent_product_name || ''
    return {
      ...family, id: parent, parent_product_id: parent, parent_product_name: family.parent_product_name || name,
      name, __order_product_family: true, __order_concrete_price_family: true,
      specs: (family.specs || []).map(spec => ({
        ...spec, parent_product_id: parent, product_id: id(spec.product_id || parent),
        sku_id: id(spec.bom_spec_id || spec.sku_id || spec.product_id),
        spec_label: spec.spec_label || spec.spec_name || spec.sku_name || '',
        unit_price: mode === 'stock' ? 0 : number(spec.unit_price ?? spec.price_tiers?.[0]?.unit_price),
        publication_id: id(spec.publication_id || spec.price_tiers?.[0]?.publication_id),
      })),
    }
  })
}

export function fulfillmentSpecForRow(families = [], row = {}) {
  const parent = id(row.parent_product_id || row.product_id)
  const family = families.find(entry => id(entry.parent_product_id) === parent)
  if (!family) return { family: null, spec: null }
  const spec = family.specs.find(entry => id(row.bom_spec_id) > 0
    ? id(entry.bom_spec_id) === id(row.bom_spec_id)
    : id(entry.product_id || entry.sku_id) === id(row.product_id)) || null
  return { family, spec }
}

export function fulfillmentSpecRowPatch(family = {}, spec = {}, mode = 'stock', originalItem = null) {
  const original = originalItem && id(originalItem.product_id) === id(spec.product_id || family.parent_product_id) &&
    id(originalItem.bom_spec_id) === id(spec.bom_spec_id) ? originalItem : null
  const name = family.name || family.parent_product_name || ''
  return {
    parent_product_id: id(family.parent_product_id), parent_product_name: family.parent_product_name || name,
    product_id: id(spec.product_id || family.parent_product_id), product_name: name, product_query: name,
    product_record_name: family.parent_product_name || name, product_kind: spec.product_kind || family.product_kind || 'roasted_bean',
    product_code: spec.sku_code || spec.product_code || '', bom_spec_id: id(spec.bom_spec_id), bom_variant_id: id(original?.bom_variant_id || spec.bom_variant_id),
    migration_state: id(spec.bom_spec_id) > 0 ? 'cutover' : 'legacy',
    spec_source: 'price_list_sku', spec_mode: String(id(spec.bom_spec_id || spec.sku_id || spec.product_id)),
    spec_label: spec.spec_label || spec.spec_name || '', spec_g: number(spec.spec_g),
    unit: spec.inventory_unit || spec.sales_unit || '件', sales_unit: spec.inventory_unit || spec.sales_unit || '件',
    unit_price: mode === 'stock' ? 0 : number(original?.unit_price ?? spec.unit_price),
    bean_list_publication_id: mode === 'stock' ? 0 : id(original?.publication_id ?? original?.bean_list_publication_id ?? spec.publication_id), price_selection_mode: 'auto', selected_price_row_key: '',
    tier_id: 'auto', manual_price: false, price_missing: mode !== 'stock' && !(number(spec.unit_price) > 0),
    historical_spec_readonly: false, spec_invalid_message: '', spec_conversion: null,
    line_amount: mode === 'stock' ? 0 : original?.line_amount, base_line_amount: mode === 'stock' ? 0 : original?.base_line_amount,
    discount_type: mode === 'stock' ? '' : original?.discount_type || '', discount_value: mode === 'stock' ? 0 : number(original?.discount_value), discount_amount: mode === 'stock' ? 0 : number(original?.discount_amount),
  }
}

function sameFulfillmentLine(row, original) {
  return original && id(row.item_id) > 0 && id(row.item_id) === id(original.item_id) &&
    id(row.product_id) === id(original.product_id) && id(row.bom_spec_id) === id(original.bom_spec_id) &&
    (id(row.bom_variant_id) === 0 || id(row.bom_variant_id) === id(original.bom_variant_id))
}

export function fulfillmentLinePreview(row = {}, original = null, mode = 'on_demand') {
  const same = sameFulfillmentLine(row, original)
  const sameQty = same && number(row.qty) === number(original.qty)
  const discount = same && (number(original.discount_amount) !== 0 || number(original.discount_value) !== 0)
  return {
    amount: mode === 'stock' ? 0 : sameQty && original.line_amount != null ? number(original.line_amount) : number(row.unit_price) * number(row.qty),
    discountAmount: mode === 'stock' || !same ? 0 : number(original.discount_amount),
    requiresDiscountReview: mode !== 'stock' && Boolean(discount && !sameQty),
  }
}

export function fulfillmentDraftGoodsCents(context = {}, rows = []) {
  const originals = context.items || []
  const originalNet = originals.reduce((sum, item) => sum + number(item.line_amount ?? number(item.unit_price) * number(item.qty)), 0)
  const currentNet = rows.reduce((sum, row) => sum + fulfillmentLinePreview(row, originals.find(item => id(item.item_id) === id(row.item_id)), context.fulfillment_mode).amount, 0)
  return Math.round(number(context.goods_total_cents ?? Math.round(originalNet * 100)) + (currentNet - originalNet) * 100)
}

export function fulfillmentSavedGoodsCents(order = {}) {
  const saved = order.fulfillment_goods_total_cents ?? order.goods_total_cents
  if (saved != null) return number(saved)
  const shipping = number(order.shipping_amount)
  if (order.grand_total != null && (number(order.grand_total) !== 0 || number(order.discount_amount) !== 0 || shipping !== 0 || number(order.rounding_amount) !== 0 || number(order.total_amount) === 0)) return Math.round((number(order.grand_total) - shipping) * 100)
  return Math.round((number(order.total_amount) - number(order.discount_amount)) * 100)
}

export function mergeFulfillmentDetailItems(details = [], canonical = []) {
  return canonical.map(item => {
    const original = details.find(detail => id(item.item_id) > 0 && id(detail.item_id) === id(item.item_id)) || {}
    return { ...original, ...item, spec: item.spec_name || item.spec_label || item.spec || original.spec || '', unit: item.inventory_unit || item.sales_unit || item.unit || original.unit || '' }
  })
}

export function fulfillmentEditCommand(context, form, rows, adjustment = {}) {
  const items = rows.filter(row => id(row.product_id) > 0).map(row => ({
    item_id: id(row.item_id), product_id: id(row.product_id), bom_spec_id: id(row.bom_spec_id),
    bom_variant_id: id(row.bom_variant_id), qty: number(row.qty),
    spec_g: id(row.bom_spec_id) > 0 ? 0 : number(row.spec_g),
    ...(row.unit || row.sales_unit ? { sales_unit: String(row.unit || row.sales_unit) } : {}),
    publication_id: id(row.bean_list_publication_id),
    price_selection_mode: 'auto', selected_price_row_key: '',
  }))
  if (!items.length || items.some(item => item.bom_spec_id <= 0 || !Number.isInteger(item.qty) || item.qty <= 0)) throw new Error('请完整选择商品规格，数量须为正整数')
  if (!String(form.receiver_name || '').trim() || !String(form.receiver_phone || '').trim() || !String(form.receiver_address || '').trim()) throw new Error('请填写收件人、电话和地址')
  const command = {
    edit_revision: context.edit_revision, order_date: form.order_date,
    receiver_name: String(form.receiver_name || '').trim(), receiver_phone: String(form.receiver_phone || '').trim(),
    receiver_address: String(form.receiver_address || '').trim(), receiver_company: String(form.receiver_company || '').trim(),
    province: String(form.shipping_province || '').trim(), city: String(form.shipping_city || '').trim(), district: String(form.shipping_district || '').trim(),
    notes: String(form.notes || '').trim(), items,
  }
  if (adjustment.enabled) {
    const service = Number(adjustment.amount)
    if (!Number.isFinite(service) || service < 0) throw new Error('请输入有效的非负服务费')
    if (!String(adjustment.reason || '').trim()) throw new Error('请填写费用调整原因')
    command.service_fee_cents = Math.round(service * 100)
    command.adjustment_reason = String(adjustment.reason).trim()
  }
  return command
}

export function fulfillmentFeeSummary(fees = {}, goodsCents = 0, freightView = {}) {
  const freight = fees.freight || {}
  const known = freight.estimate_known !== false
  const estimated = freight.estimated_total_cents ?? freight.estimate?.base_cents ?? fees.calculated_shipping_fee_cents
  const shipping = freight.current_receivable_cents ?? fees.shipping_fee_cents ?? 0
  const settledService = freightView?.service_fee_cents == null ? null : Math.max(0, number(freightView.service_fee_cents) - number(freightView.service_due_cents))
  const serviceDue = freightView?.service_due_cents == null ? null : settledService == null ? number(freightView.service_due_cents) : Math.max(0, number(fees.service_fee_cents) - settledService)
  return {
    goods: number(goodsCents), service: number(fees.service_fee_cents), shipping: number(shipping),
    serviceDue,
    estimated: known && estimated != null ? number(estimated) : null,
    actual: (freightView?.actual_cost_cents ?? freight.actual_cost_cents ?? freight.actual_cents) == null ? null : number(freightView?.actual_cost_cents ?? freight.actual_cost_cents ?? freight.actual_cents),
    billed: (freightView?.billed_cents ?? freight.billed_cents) == null ? null : number(freightView?.billed_cents ?? freight.billed_cents),
    total: number(goodsCents) + number(fees.service_fee_cents) + number(shipping),
  }
}

export function fulfillmentItemProductName(item = {}) {
  const name = String(item.product_name || item.item_name || '').trim()
  const spec = String(item.spec_name || item.spec_label || item.bom_spec_name || item.spec || '').trim()
  return spec && name.endsWith(spec) && name.length > spec.length ? name.slice(0, -spec.length).trim() : name
}

export function fulfillmentListFeeLines(order = {}) {
  const view = order.freight || {}
  const money = cents => (number(cents) / 100).toFixed(2)
  return [
    { label: '商品货款', value: (fulfillmentSavedGoodsCents(order) / 100).toFixed(2) },
    { label: '代发服务费', value: view.service_fee_cents == null ? '待读取' : money(view.service_fee_cents) },
    ...(view.service_due_cents == null ? [] : [{ label: '服务费结算', value: number(view.service_due_cents) > 0 ? `待收 ${money(view.service_due_cents)}` : '已结清' }]),
    { label: '预估运费', value: view.estimate_known === false || view.estimated_total_cents == null ? '待核算' : money(view.estimated_total_cents) },
    { label: '实际运费', value: view.actual_cost_cents == null ? '待快递账单' : money(view.actual_cost_cents) },
    ...(view.actual_cost_cents == null || view.billed_cents == null ? [] : [{ label: '运费已出账', value: money(view.billed_cents) }]),
  ]
}
