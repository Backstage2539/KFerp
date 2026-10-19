import type { CustomerAssetInventory, CustomerFulfillmentSettings, CustomerInventorySummary, DirectShipCatalog, DirectShipPreview, EmployeeOrderProductSpec, FulfillmentMode } from '../api/customerPortal'
import type { DirectShipDraftLine } from './directShipDraft'

export function enabledDirectShipModes(settings?: CustomerFulfillmentSettings): FulfillmentMode[] {
  return (settings?.modes || []).filter(row => row.enabled).map(row => row.mode)
}

export function initialDirectShipMode(settings?: CustomerFulfillmentSettings, preferred: FulfillmentMode | '' = ''): FulfillmentMode | '' {
  const enabled = enabledDirectShipModes(settings)
  return preferred && enabled.includes(preferred) ? preferred : enabled.includes('stock') ? 'stock' : enabled[0] || ''
}

export function purchaseDeliveryMode(settings?: CustomerFulfillmentSettings): 'ship' | 'customer_stock' {
  return enabledDirectShipModes(settings).includes('stock') ? 'customer_stock' : 'ship'
}

export function stockItemKey(item: { product_id?: number; sku_id?: number; bom_spec_id?: number; bom_variant_id?: number; spec_g?: number }): string {
  const product = Number(item.sku_id || item.product_id || 0)
  return Number(item.bom_spec_id || 0) > 0 ? `${product}:bom:${item.bom_spec_id}:${item.bom_variant_id || 0}` : `${product}:g:${item.spec_g || 0}`
}

export function stockOverviewRows(catalog: DirectShipCatalog, lines: DirectShipDraftLine[]) {
  const rows = new Map<string, { key: string; name: string; spec: string; unit: string; physical: number; pendingAvailable: number; reserved: number; available: number; requested: number; remaining: number; gap: number }>()
  for (const family of catalog.product_families) for (const spec of family.specs) {
    const key = stockItemKey(spec)
    const metrics = stockScenarioMetrics(spec)
    const pendingAvailable = Math.max(0, metrics.pending - metrics.reserved)
    const available = metrics.physical + pendingAvailable
    if (!rows.has(key)) rows.set(key, { key, name: family.customer_product_display_name || family.name || family.parent_product_name || '', spec: spec.spec_label || spec.sku_name || '', unit: spec.inventory_unit || spec.sales_unit || '件', physical: metrics.physical, pendingAvailable, reserved: metrics.reserved, available, requested: 0, remaining: available, gap: 0 })
  }
  for (const line of lines) {
    if (!(Number(line.product_id) > 0)) continue
    const key = stockItemKey(line)
    if (!rows.has(key)) rows.set(key, { key, name: line.product_name, spec: line.spec_label || '', unit: line.inventory_unit || line.sales_unit || '件', physical: 0, pendingAvailable: 0, reserved: 0, available: 0, requested: 0, remaining: 0, gap: 0 })
    const row = rows.get(key)!
    const qty = Number(line.qty)
    row.requested += Number.isFinite(qty) ? Math.max(0, qty) : 0
    row.remaining = Math.max(0, row.available - row.requested)
    row.gap = Math.max(0, row.requested - row.available)
  }
  return [...rows.values()].filter(row => row.available > 0 || row.requested > 0)
}

export function stockTopupFromCatalog(lines: DirectShipDraftLine[], catalog: DirectShipCatalog): DirectShipDraftLine[] {
  return stockOverviewRows(catalog, lines).flatMap(row => {
    const original = lines.find(line => stockItemKey(line) === row.key)
    return original && row.gap > 0 ? [{ ...original, qty: row.gap }] : []
  })
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
