import { isProductBomSpecCutover, normalizeProductBomSpecs } from './product-spec-cutover.js'

function number(value) {
  const result = Number(value || 0)
  return Number.isFinite(result) ? result : 0
}

function text(value) {
  return String(value || '').trim()
}

export function currentBOMSpecs(product = {}) {
  return normalizeProductBomSpecs(product)
}

export function defaultBOMSpecID(product = {}) {
  const specs = currentBOMSpecs(product)
  return number(specs.find((row) => row.is_default)?.bom_spec_id || specs[0]?.bom_spec_id)
}

export function selectedBOMSpec(product = {}, bomSpecID = 0) {
  const specs = currentBOMSpecs(product)
  const selectedID = number(bomSpecID) || defaultBOMSpecID(product)
  return specs.find((row) => number(row.bom_spec_id) === selectedID) || null
}

export function usesCurrentBOMSpecs(product = {}) {
  return isProductBomSpecCutover(product)
}

export function buildFinishedTransferPayload(form = {}, product = {}) {
  if (!usesCurrentBOMSpecs(product)) {
    return {
      product_id: number(form.product_id),
      spec_g: number(form.spec_g),
      from_warehouse: text(form.from_warehouse),
      to_warehouse: text(form.to_warehouse),
      qty_units: number(form.qty_units),
      qty_loose_g: number(form.qty_loose_g),
      note: text(form.note),
    }
  }
  const spec = selectedBOMSpec(product, form.bom_spec_id)
  return {
    product_id: number(form.product_id),
    bom_spec_id: number(spec?.bom_spec_id),
    unit_code: text(spec?.unit),
    from_warehouse: text(form.from_warehouse),
    to_warehouse: text(form.to_warehouse),
    qty_units: number(form.qty_units),
    note: text(form.note),
  }
}

export function buildFinishedAdjustmentPayload(form = {}, product = {}) {
  if (!usesCurrentBOMSpecs(product)) {
    return {
      adjustment_type: 'quantity',
      item_type: 'finished_product',
      item_id: number(form.item_id),
      spec_g: number(form.spec_g),
      warehouse: text(form.warehouse),
      target_g: number(form.target_g),
      target_units: number(form.target_units),
      reason: text(form.reason),
    }
  }
  const spec = selectedBOMSpec(product, form.bom_spec_id)
  return {
    adjustment_type: 'quantity',
    item_type: 'finished_product',
    item_id: number(form.item_id),
    bom_spec_id: number(spec?.bom_spec_id),
    unit_code: text(spec?.unit),
    warehouse: text(form.warehouse),
    target_units: number(form.target_units),
    reason: text(form.reason),
  }
}
