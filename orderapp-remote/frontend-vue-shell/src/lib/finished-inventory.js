import { buildProductSpecWriteIdentity, isProductBomSpecCutover } from './product-spec-cutover.js'

function quantity(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}

export function buildFinishedInventoryAdjustmentPayload(form = {}) {
  if (!isProductBomSpecCutover(form)) return { ...form }
  const identity = buildProductSpecWriteIdentity({ ...form, parent_product_id: form.product_id, qty: form.units })
  return {
    product_id: quantity(identity.product_id),
    bom_spec_id: quantity(identity.bom_spec_id),
    bom_variant_id: quantity(identity.bom_variant_id),
    units: quantity(form.units),
  }
}

export function finishedInventoryRowQuantity(row = {}) {
  if (quantity(row.bom_spec_id) > 0) {
    return `${quantity(row.units)} ${String(row.inventory_unit || row.unit || '').trim() || '件'}`
  }
  return `${quantity(row.units)} 件 + ${quantity(row.loose_g)}g（${quantity(row.total_g)}g）`
}
