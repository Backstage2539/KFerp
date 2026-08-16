import { describe, expect, it } from 'vitest'
import type { CustomerInventorySummary, EmployeeOrderProductFamily } from '../api/customerPortal'
import {
  customerInventorySelectionItems,
  customerInventoryDetailPath,
  customerInventoryItemKey,
  matchesCustomerInventoryIdentity,
  normalizeProcessingPrefillItems,
  resolveProcessingPrefillLines,
  toggleCustomerInventorySelection,
} from './customerInventory'

const rows: CustomerInventorySummary[] = [
  {
    product_id: 551,
    product_name: '乌拉嘎 227g',
    sku_code: 'WLG-227',
    spec_g: 227,
    available_qty: 4,
    reserved_qty: 0,
    total_qty: 4,
    warehouses: ['客户成品仓 A'],
  },
  {
    product_id: 552,
    product_name: '乌拉嘎 454g',
    sku_code: 'WLG-454',
    spec_g: 454,
    available_qty: 5,
    reserved_qty: 1,
    total_qty: 6,
    warehouses: ['客户成品仓 B'],
  },
  {
    product_id: 911,
    product_name: '萨其姆 生豆',
    sku_code: 'SKU-000911',
    spec_g: 1000,
    available_qty: 2,
    reserved_qty: 0,
    total_qty: 2,
    warehouses: ['客户成品仓 A'],
  },
]

describe('customer inventory helpers', () => {
  it('keeps selected SKU snapshots when the visible search page changes', () => {
    let selected = toggleCustomerInventorySelection({}, rows[0])
    selected = toggleCustomerInventorySelection(selected, rows[2])
    expect(customerInventorySelectionItems(selected)).toEqual([rows[0], rows[2]])

    selected = toggleCustomerInventorySelection(selected, rows[0])
    expect(customerInventorySelectionItems(selected)).toEqual([rows[2]])
  })

  it('uses a SKU and specification key and builds a separate detail-page route', () => {
    expect(customerInventoryItemKey(rows[0])).toBe('551:227')
    expect(customerInventoryDetailPath(rows[0])).toBe(
      '/pages/customer-inventory-detail/customer-inventory-detail?product_id=551&spec_g=227',
    )
  })

  it('deduplicates multi-select prefills without requiring a positive quantity', () => {
    expect(normalizeProcessingPrefillItems([rows[0], rows[0], rows[1]])).toEqual([
      { product_id: 551, spec_g: 227, product_name: '乌拉嘎 227g', sku_code: 'WLG-227' },
      { product_id: 552, spec_g: 454, product_name: '乌拉嘎 454g', sku_code: 'WLG-454' },
    ])
  })

  it('resolves every available BOM catalog SKU and reports unavailable prefills explicitly', () => {
    const families: EmployeeOrderProductFamily[] = [{
      parent_product_id: 550,
      name: '乌拉嘎',
      customer_id: 19,
      specs: [
        { product_id: 551, sku_id: 551, sku_name: '乌拉嘎 227g', sku_code: 'WLG-227', spec_label: '227g' },
        { product_id: 552, sku_id: 552, sku_name: '乌拉嘎 454g', sku_code: 'WLG-454', spec_label: '454g' },
      ],
    }]

    const result = resolveProcessingPrefillLines([
      rows[0],
      rows[1],
      { ...rows[2], product_id: 999, product_name: '旧库存商品' },
    ], families)

    expect(result.lines).toEqual([
      { product_id: 551, product_name: '乌拉嘎 227g', spec_g: 227, spec_label: '227g', qty: 0 },
      { product_id: 552, product_name: '乌拉嘎 454g', spec_g: 454, spec_label: '454g', qty: 0 },
    ])
    expect(result.unavailable).toEqual([
      { product_id: 999, spec_g: 1000, product_name: '旧库存商品', sku_code: 'SKU-000911' },
    ])
  })

  it('preserves a cutover inventory BOM specification through selection, detail routing and processing prefill', () => {
    const canonical = {
      product_id: 550,
      parent_product_id: 550,
      bom_spec_id: 91,
      bom_variant_id: 191,
      bom_spec_name: '227g袋',
      inventory_unit: '袋',
      product_name: '乌拉嘎',
      sku_code: 'BSP-227',
      spec_g: 0,
      available_qty: 4,
      reserved_qty: 0,
      total_qty: 4,
      warehouses: ['客户成品仓 A'],
    }
    const families: EmployeeOrderProductFamily[] = [{
      parent_product_id: 550,
      name: '乌拉嘎',
      migration_state: 'cutover',
      default_bom_spec_id: 91,
      specs: [{
        product_id: 550,
        migration_state: 'cutover',
        bom_spec_id: 91,
        bom_variant_id: 191,
        spec_name: '227g袋',
        spec_label: '227g袋',
        inventory_unit: '袋',
        is_default: true,
      }],
    }]

    expect(customerInventoryItemKey(canonical)).toBe('550:bom_spec:91')
    expect(customerInventoryDetailPath(canonical)).toBe(
      '/pages/customer-inventory-detail/customer-inventory-detail?product_id=550&bom_spec_id=91&bom_variant_id=191&inventory_unit=%E8%A2%8B',
    )
    expect(normalizeProcessingPrefillItems([canonical, canonical])).toEqual([{
      product_id: 550,
      bom_spec_id: 91,
      bom_variant_id: 191,
      spec_g: 0,
      spec_name: '227g袋',
      inventory_unit: '袋',
      product_name: '乌拉嘎',
      sku_code: 'BSP-227',
    }])
    expect(resolveProcessingPrefillLines([canonical], families).lines).toEqual([{
      product_id: 550,
      bom_spec_id: 91,
      bom_variant_id: 191,
      product_name: '乌拉嘎',
      spec_g: 0,
      spec_label: '227g袋',
      inventory_unit: '袋',
      qty: 0,
    }])

    expect(customerInventoryItemKey({ ...canonical, bom_variant_id: 192 })).toBe('550:bom_spec:91')
    expect(matchesCustomerInventoryIdentity({ ...canonical, bom_variant_id: 192 }, canonical)).toBe(true)
    expect(matchesCustomerInventoryIdentity({ ...canonical, bom_spec_id: 92, bom_variant_id: 192 }, canonical)).toBe(false)
  })
})
