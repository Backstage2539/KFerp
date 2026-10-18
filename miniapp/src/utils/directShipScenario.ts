import type { CustomerAssetInventory, CustomerFulfillmentSettings, CustomerInventorySummary, DirectShipPreview, EmployeeOrderProductSpec, FulfillmentMode } from '../api/customerPortal'
import type { DirectShipDraftLine } from './directShipDraft'

export function enabledDirectShipModes(settings?: CustomerFulfillmentSettings): FulfillmentMode[] {
  return (settings?.modes || []).filter(row => row.enabled).map(row => row.mode)
}

export function initialDirectShipMode(settings?: CustomerFulfillmentSettings, preferred: FulfillmentMode | '' = ''): FulfillmentMode | '' {
  const enabled = enabledDirectShipModes(settings)
  return preferred && enabled.includes(preferred) ? preferred : enabled.length === 1 ? enabled[0] : ''
}

export function retainedStockStatus(status = ''): string {
  return ({ pending_receipt: '待入客户仓', partially_received: '部分入客户仓', received: '已入客户仓' } as Record<string, string>)[status] || '待入客户仓'
}

export function stockScenarioMetrics(spec?: Partial<EmployeeOrderProductSpec>, requested = 0) {
  const physical = Math.max(0, Number(spec?.physical_available_qty ?? spec?.stock_available_qty ?? 0))
  const pending = Math.max(0, Number(spec?.pending_inbound_qty || 0))
  const reserved = Math.max(0, Number(spec?.pending_reserved_qty || 0))
  return { physical, pending, reserved, gap: Math.max(0, Number(requested || 0) - physical - Math.max(0, pending - reserved)) }
}

export function directShipTopupLines(lines: DirectShipDraftLine[], preview: DirectShipPreview): DirectShipDraftLine[] {
  const seen = new Set<string>()
  return lines.flatMap(line => {
    const key = `${line.product_id}:${line.bom_spec_id || 0}:${line.bom_variant_id || 0}:${line.spec_g || 0}`
    if (seen.has(key)) return []
    seen.add(key)
    const shortage = (preview.shortages || []).find(row => Number(row.product_id) === Number(line.product_id)
      && (Number(line.bom_spec_id || 0) > 0
        ? Number(row.bom_spec_id || 0) === Number(line.bom_spec_id) && Number(row.bom_variant_id || 0) === Number(line.bom_variant_id || 0)
        : Number(row.spec_g || 0) === Number(line.spec_g || 0)))
    const gap = shortage ? Math.max(0, Number(shortage.qty) - Number(shortage.available_qty || 0)) : 0
    return gap > 0 ? [{ ...line, qty: gap }] : []
  })
}

export function mergeCustomerStockAssets(assets: CustomerAssetInventory[], stock: CustomerInventorySummary[]): CustomerAssetInventory[] {
  const rows = assets.map(row => ({ ...row }))
  for (const item of stock) {
    const existing = rows.find(row => row.inventory_type === 'finished_product' && Number(row.product_id || row.item_id) === Number(item.product_id)
      && Number(row.bom_spec_id || 0) === Number(item.bom_spec_id || 0) && Number(row.bom_variant_id || 0) === Number(item.bom_variant_id || 0) && (Number(item.bom_spec_id || 0) > 0 || Number(row.spec_g || 0) === Number(item.spec_g || 0)))
    const metrics = { physical_available_qty: item.physical_available_qty ?? item.available_qty, pending_inbound_qty: item.pending_inbound_qty || 0, pending_reserved_qty: item.pending_reserved_qty || 0 }
    if (existing) Object.assign(existing, metrics)
    else if (Number(item.pending_inbound_qty || 0) > 0) rows.push({ ...metrics, inventory_type: 'finished_product', item_id: item.product_id, product_id: item.product_id, bom_spec_id: item.bom_spec_id, bom_variant_id: item.bom_variant_id, spec_g: item.spec_g, item_code: item.sku_code, item_name: item.product_name, spec: item.bom_spec_name || `${item.spec_g}g`, unit: item.inventory_unit || '件', total_qty: item.total_qty || 0, available_qty: metrics.physical_available_qty, occupied_qty: item.reserved_qty || 0, in_production_qty: 0, quality_status: '', warehouses: item.warehouses.map(code => ({ warehouse_code: code, warehouse_name: code, available_qty: 0, occupied_qty: 0, in_production_qty: 0 })), batches: [] })
  }
  return rows
}
