function numericID(value) {
  const parsed = Number(value || 0)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

function numberValue(value) {
  const parsed = Number(value || 0)
  return Number.isFinite(parsed) ? parsed : 0
}

export function isCanonicalWarehouseInventoryRow(row = {}) {
  return numericID(row.bom_spec_id ?? row.bomSpecID) > 0
}

export function warehouseInventoryItemKey(row = {}) {
  const itemType = String(row.item_type || '')
  const itemID = numericID(row.item_id)
  const bomSpecID = numericID(row.bom_spec_id ?? row.bomSpecID)
  if (bomSpecID > 0) return `${itemType}:${itemID}:bom_spec:${bomSpecID}`
  return `${itemType}:${itemID}:${numberValue(row.spec_g)}`
}

export function warehouseInventoryRowKey(row = {}) {
  const batchIdentity = row.batch_id || row.batch_code || 'summary'
  if (isCanonicalWarehouseInventoryRow(row)) {
    return `${row.warehouse || ''}-${warehouseInventoryItemKey(row)}-${batchIdentity}`
  }
  return `${row.warehouse || ''}-${row.item_type || ''}-${numericID(row.item_id)}-${numberValue(row.spec_g)}-${batchIdentity}`
}

export function warehouseInventorySpecLabel(row = {}) {
  if (isCanonicalWarehouseInventoryRow(row)) {
    return String(
      row.bom_spec_name
      || row.spec_name
      || row.spec_name_snapshot
      || row.bomSpecName
      || '',
    ).trim() || `BOM规格 #${numericID(row.bom_spec_id ?? row.bomSpecID)}`
  }
  const specG = numberValue(row.spec_g)
  return specG > 0 ? `${specG}g` : '-'
}

export function warehouseInventoryUnitLabel(row = {}) {
  if (!isCanonicalWarehouseInventoryRow(row)) return ''
  return String(row.inventory_unit || row.inventoryUnit || row.output_unit || '').trim()
}

export function warehouseInventoryQuantityLabel(row = {}) {
  const qtyG = numberValue(row.qty_g)
  const qtyUnits = numberValue(row.qty_units)
  if (isCanonicalWarehouseInventoryRow(row)) {
    const unit = warehouseInventoryUnitLabel(row) || '库存单位'
    if (qtyUnits && qtyG) return `${qtyUnits.toLocaleString('zh-CN')} ${unit} / ${qtyG.toLocaleString('zh-CN')}g`
    if (qtyUnits) return `${qtyUnits.toLocaleString('zh-CN')} ${unit}`
    if (qtyG) return `${qtyG.toLocaleString('zh-CN')}g`
    return `0 ${unit}`
  }
  if (qtyUnits && qtyG) return `${qtyUnits.toLocaleString('zh-CN')} 件 / ${qtyG.toLocaleString('zh-CN')}g`
  if (qtyUnits) return `${qtyUnits.toLocaleString('zh-CN')} ${row?.item_type === 'finished_product' ? '件' : '库存单位'}`
  if (qtyG) return `${qtyG.toLocaleString('zh-CN')}g`
  return '-'
}
