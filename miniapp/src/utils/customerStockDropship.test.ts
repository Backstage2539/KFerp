import { describe, expect, it } from 'vitest'
import { buildDirectShipCatalogPath } from '../api/customerPortal'
import { directShipTopupLines, initialDirectShipMode, mergeCustomerStockAssets, stockScenarioMetrics } from './directShipScenario'
import { createDirectShipDraftLine } from './directShipDraft'

describe('代发场景目录请求', () => {
  it('向服务端声明库存场景，避免读取无库存商品和价格', () => {
    expect(buildDirectShipCatalogPath('stock')).toBe('/api/mini/direct-ship/catalog?fulfillment_mode=stock')
  })
  it('兼容未切换场景的原有入口', () => {
    expect(buildDirectShipCatalogPath()).toBe('/api/mini/direct-ship/catalog')
  })
})

describe('库存权益与补购草稿', () => {
  it('旧入口双开时默认库存，独立入口保持明确场景', () => {
    const settings = { customer_id: 77, revision: 1, modes: [{ mode: 'stock' as const, enabled: true, fixed_fee_cents: 0, per_package_fee_cents: 0, fee_configured: true, shipping_template_id: 1 }, { mode: 'on_demand' as const, enabled: true, fixed_fee_cents: 0, per_package_fee_cents: 0, fee_configured: true, shipping_template_id: 1 }] }
    expect(initialDirectShipMode(settings)).toBe('stock')
    expect(initialDirectShipMode(settings, 'on_demand')).toBe('on_demand')
    expect(initialDirectShipMode({ ...settings, modes: [settings.modes[0]] })).toBe('stock')
  })
  it('待入库权益扣除已有预订，不混入实物数', () => {
    expect(stockScenarioMetrics({ physical_available_qty: 4, pending_inbound_qty: 20, pending_reserved_qty: 12 }, 14)).toEqual({ physical: 4, pending: 20, reserved: 12, gap: 2 })
  })
  it('按服务端缺口生成同规格补购；不修改原代发草稿', () => {
    const line = { ...createDirectShipDraftLine(), product_id: 5, bom_spec_id: 10, bom_variant_id: 2, qty: 14 }
    const preview = { can_submit: false, stock_ready: false, total_amount: 0, price_quote_token: '', warehouses: [], shortages: [{ product_id: 5, bom_spec_id: 10, bom_variant_id: 2, spec_g: 0, qty: 14, available_qty: 12, stock_available_qty: 4, production_available_qty: 8, blocking: true }] }
    expect(directShipTopupLines([line], preview)[0].qty).toBe(2)
    expect(directShipTopupLines([line, { ...line, key: 'second' }], preview)).toHaveLength(1)
    expect(line.qty).toBe(14)
    expect(directShipTopupLines([{ ...line, bom_variant_id: 3 }], preview)).toEqual([{ ...line, bom_variant_id: 3, qty: 2 }])
  })
  it('尚未入库的24包也显示在客户库存列表，实物数保持0', () => {
    const rows = mergeCustomerStockAssets([], [{ product_id: 5, product_name: '曜石2.0', bom_spec_id: 10, bom_variant_id: 2, inventory_unit: '袋', spec_g: 0, physical_available_qty: 0, pending_inbound_qty: 24, pending_reserved_qty: 12, available_qty: 12, total_qty: 0, reserved_qty: 0, warehouses: ['XI-QIN'] }])
    expect(rows).toHaveLength(1)
    expect(rows[0].available_qty).toBe(0)
    expect(rows[0].pending_inbound_qty).toBe(24)
    expect(rows[0].pending_reserved_qty).toBe(12)
  })
})
