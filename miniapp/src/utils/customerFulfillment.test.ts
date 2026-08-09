import { describe, expect, it } from 'vitest'
import type { EmployeeOrderProductFamily } from '../api/customerPortal'
import {
  canShowFactoryProductLinks,
  directShipStatusLabel,
  mergeProcessingTargetLines,
  productionStatusLabel,
  productionSubmissionBlockReason,
  scopedFulfillmentProductFamilies,
} from './customerFulfillment'

const families: EmployeeOrderProductFamily[] = [
  {
    parent_product_id: 10,
    name: '萨其姆生豆',
    code: 'P-001',
    py: 'saqimu',
    product_type_name: '生豆',
    product_kind: 'green_bean',
    customer_id: 31,
    specs: [{ product_id: 911, sku_id: 911, sku_code: 'SKU-000911', sku_name: '萨其姆-生豆', spec_label: '60kg' }],
  },
  {
    parent_product_id: 20,
    name: '耶加雪菲熟豆',
    code: 'P-002',
    py: 'yegaxuefei',
    product_type_name: '熟豆',
    product_kind: 'roasted_bean',
    customer_id: 31,
    specs: [{ product_id: 912, sku_id: 912, sku_code: 'SKU-000912', sku_name: '耶加雪菲-454g', spec_label: '454g' }],
  },
]

describe('customer fulfillment helpers', () => {
  it('reuses employee-order category and fuzzy search behavior for customer SKU selection', () => {
    expect(scopedFulfillmentProductFamilies(families, 31, 'SKU-000911', 'green_bean')).toEqual([families[0]])
    expect(scopedFulfillmentProductFamilies(families, 31, 'saqimu', 'all')).toEqual([families[0]])
    expect(scopedFulfillmentProductFamilies(families, 31, '耶加', 'roasted')).toEqual([families[1]])
  })

  it('merges duplicate target SKU and specification rows without sharing quantities across SKUs', () => {
    expect(mergeProcessingTargetLines([
      { product_id: 911, spec_g: 60000, qty: 1 },
      { product_id: 911, spec_g: 60000, qty: 2 },
      { product_id: 912, spec_g: 454, qty: 4 },
    ])).toEqual([
      { product_id: 911, spec_g: 60000, qty: 3 },
      { product_id: 912, spec_g: 454, qty: 4 },
    ])
  })

  it('blocks production submission when any aggregated BOM material has a shortage', () => {
    expect(productionSubmissionBlockReason({
      complete: true,
      materials: [{ material_id: 8, required_g: 1000, available_g: 900, shortage_g: 100 }],
    })).toBe('物料库存不足，无法提交生产工单')
    expect(productionSubmissionBlockReason({ complete: false, materials: [] })).toBe('当前目标商品没有可用 BOM 配置')
    expect(productionSubmissionBlockReason({ complete: true, canSubmit: false, materials: [] })).toBe('当前生产配置无法提交')
    expect(productionSubmissionBlockReason({ complete: true, materials: [] })).toBe('')
  })

  it('requires both spot-order and bean-list capabilities for factory product links', () => {
    expect(canShowFactoryProductLinks([
      { code: 'processing', enabled: true },
      { code: 'bean_list', enabled: true },
    ])).toBe(false)
    expect(canShowFactoryProductLinks([
      { code: 'product_order', enabled: true },
      { code: 'bean_list', enabled: true },
    ])).toBe(true)
  })

  it('shows customer-facing production and shipment status labels', () => {
    expect(productionStatusLabel('awaiting_schedule')).toBe('待排产')
    expect(productionStatusLabel('partially_completed')).toBe('部分完成')
    expect(directShipStatusLabel('reserved')).toBe('待发货')
    expect(directShipStatusLabel('partially_shipped')).toBe('部分发货')
  })
})
