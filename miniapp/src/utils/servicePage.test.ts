import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  buildFulfillmentOrderPayload,
  fulfillmentSalesUnitOptions,
  serviceCapability,
  serviceTitle,
  visibleServiceSections,
} from './servicePage'

describe('service page helpers', () => {
  function readSource(path: string): string {
    return readFileSync(resolve(path), 'utf8')
  }

  it('maps mini service keys to capability codes and titles', () => {
    expect(serviceCapability('directShip')).toBe('direct_ship')
    expect(serviceCapability('orders')).toBe('product_order')
    expect(serviceCapability('processing')).toBe('processing')
    expect(serviceTitle('orders')).toBe('订单中心')
    expect(serviceTitle('settlement')).toBe('费用中心')
  })

  it('maps legacy logistics service links to the order service', () => {
    expect(serviceCapability('shipping')).toBe('product_order')
    expect(serviceTitle('shipping')).toBe('订单中心')
  })

  it('returns visible data sections for a service payload', () => {
    const sections = visibleServiceSections({
      key: 'orders',
      title: '我的订单',
      orders: [{ order_no: 'SO-1', ship_tracking_no: 'SF123' }],
      fee_items: [{ fee_type: 'shipping', amount: '12.00', status: 'unsettled' }],
    })

    expect(sections.map((section) => section.title)).toEqual(['我的订单', '费用明细'])
  })

  it('labels settlement orders as bill rows', () => {
    const sections = visibleServiceSections({
      key: 'settlement',
      title: '结算中心',
      orders: [{ order_no: 'SO-YAN-BILL', grand_total: '4559.00' }],
    })

    expect(sections.map((section) => section.title)).toEqual(['订单账单'])
  })

  it('keeps settlement billing unfiltered by default so older unpaid orders stay in the summary', () => {
    const servicePage = readSource('src/pages/service/service.vue')

    expect(servicePage).toContain("serviceKey.value === 'settlement'")
    expect(servicePage).toContain('buildOrderServiceFilters(orderSearch.value)')
    expect(servicePage).toContain('账期筛选')
    expect(servicePage).not.toContain('applyBillingDefaultPeriod')
  })

  it('maps drip fulfillment products to bag and box options', () => {
    const options = fulfillmentSalesUnitOptions({
      id: 8,
      name: '耶加雪菲挂耳',
      roast_level: '',
      default_price: '0.00',
      retail_price_100g: '0.00',
      retail_price_200g: '0.00',
      retail_price_227g: '0.00',
      retail_price_250g: '0.00',
      product_kind: 'drip_bag',
      sales_units: ['bag', 'box'],
      drip_bag_grams: 10,
      drip_box_bag_count: 12,
    })

    expect(options).toEqual([
      { sales_unit: 'bag', label: '袋', unit_bag_count: 1, unit_bean_g: 10, spec_g: 10, quantity_label: '袋数' },
      { sales_unit: 'box', label: '盒', unit_bag_count: 12, unit_bean_g: 10, spec_g: 120, quantity_label: '盒数' },
    ])
  })

  it('builds drip fulfillment submit payload with the selected unit snapshot', () => {
    const payload = buildFulfillmentOrderPayload('product_order', {
      recipient_name: '张三',
      recipient_phone: '13800138000',
      recipient_address: '上海市',
      recipient_company: '',
      product_id: 8,
      product_name: '耶加雪菲挂耳',
      spec_g: 120,
      qty: 3,
      sales_unit: 'box',
      unit_bag_count: 12,
      unit_bean_g: 10,
      note: '周末前发',
    })

    expect(payload).toEqual({
      service_code: 'product_order',
      recipient_name: '张三',
      recipient_phone: '13800138000',
      recipient_address: '上海市',
      recipient_company: '',
      product_id: 8,
      product_name: '耶加雪菲挂耳',
      spec_g: 120,
      qty: 3,
      sales_unit: 'box',
      unit_bag_count: 12,
      unit_bean_g: 10,
      note: '周末前发',
    })
  })
})
