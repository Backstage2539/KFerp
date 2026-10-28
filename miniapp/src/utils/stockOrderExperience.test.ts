import { describe, expect, it } from 'vitest'
import { stockOverviewRows, stockTopupFromCatalog, purchaseDeliveryMode } from './directShipScenario'
import { createDirectShipDraftLine } from './directShipDraft'
import type { DirectShipCatalog, CustomerFulfillmentSettings } from '../api/customerPortal'

const settings: CustomerFulfillmentSettings = { customer_id: 9, revision: 1, modes: [{ mode: 'stock', enabled: true, warehouse_code: 'C9', fixed_fee_cents: 0, per_package_fee_cents: 0, fee_configured: true, shipping_template_id: 1 }] }
const catalog = { current_customer_id: 9, product_families: [{ name: '曜石', parent_product_id: 5, specs: [{ product_id: 5, sku_id: 5, bom_spec_id: 10, bom_variant_id: 2, spec_label: '454g', inventory_unit: '袋', purchased_qty: 20, received_qty: 8, physical_available_qty: 4, pending_inbound_qty: 42, pending_reserved_qty: 26 }] }] } as DirectShipCatalog
const line = { ...createDirectShipDraftLine(), product_id: 5, bom_spec_id: 10, bom_variant_id: 2, spec_label: '454g', product_name: '曜石', inventory_unit: '袋', qty: 12 }

describe('库存概览与购货留厂', () => {
  it('全部库存均显示，同规格多行合计扣算，不修改目录或草稿', () => {
    const rows = stockOverviewRows(catalog, [line, { ...line, key: 'two', qty: 10 }])
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({ available: 20, requested: 22, remaining: 0, gap: 2, physical: 4, pendingAvailable: 16, reserved: 26, purchased: 20, received: 8, unit: '袋' })
    expect(stockOverviewRows(catalog, [])[0]).toMatchObject({ available: 20, requested: 0, remaining: 20 })
    expect(stockOverviewRows(catalog, [line, { ...line, key: 'different-version', bom_variant_id: 99, qty: 10 }])[0].gap).toBe(2)
    expect(line.qty).toBe(12)
  })
  it('刷新后只带准确缺口，无缺口或尚未选商品时不预填购货数量', () => {
    expect(stockTopupFromCatalog([line, { ...line, key: 'two', qty: 10 }], catalog)).toEqual([{ ...line, qty: 2 }])
    expect(stockTopupFromCatalog([line], catalog)).toEqual([])
    expect(stockTopupFromCatalog([], catalog)).toEqual([])
    expect(stockTopupFromCatalog([{ ...line, qty: 22 }], { ...catalog, product_families: [] })[0].qty).toBe(22)
  })
  it('库存代发客户自动留厂，其他客户保持正常发货', () => {
    expect(purchaseDeliveryMode(settings)).toBe('customer_stock')
    expect(purchaseDeliveryMode({ ...settings, modes: [{ ...settings.modes[0], enabled: false }] })).toBe('ship')
    expect(purchaseDeliveryMode()).toBe('ship')
  })
})
