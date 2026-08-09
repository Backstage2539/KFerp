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
    expect(serviceTitle('processing')).toBe('生产工单')
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

  it('does not expose sales orders as fee-center bill rows', () => {
    const sections = visibleServiceSections({
      key: 'settlement',
      title: '结算中心',
      orders: [{ order_no: 'SO-YAN-BILL', grand_total: '4559.00' }],
      settlement_batches: [{ settlement_no: 'SET-1', total_amount: '4559.00' }],
    })

    expect(sections.map((section) => section.title)).toEqual(['账单'])
  })

  it('loads fee-center bills independently from sales-order filters', () => {
    const servicePage = readSource('src/pages/service/service.vue')
    const billsPanel = readSource('src/components/CustomerBillsPanel.vue')

    expect(servicePage).toContain("serviceKey.value === 'settlement'")
    expect(billsPanel).toContain('fetchCustomerBills')
    expect(servicePage).not.toContain("serviceKey.value === 'settlement' ? buildOrderServiceFilters")
    expect(servicePage).not.toContain('账期筛选')
  })

  it('only exposes the retained sections for direct ship and production work orders', () => {
    expect(visibleServiceSections({
      key: 'directShip',
      title: '一件代发',
      products: [{}],
      orders: [{}],
      direct_ship_batches: [{}],
    })).toEqual([])

    expect(visibleServiceSections({
      key: 'processing',
      title: '生产工单',
      products: [{}],
      orders: [{}],
      inventory: [{}],
      processing_requests: [{ request_no: 'PR-1' }],
    })).toEqual([{ title: '生产工单', count: 1 }])
  })

  it('keeps the retained spot-order form defaults behind the serviceForms factory', () => {
    const servicePage = readSource('src/pages/service/service.vue')

    expect(servicePage).toContain('../../utils/serviceForms')
    expect(servicePage).toContain('emptyFulfillmentForm()')
    expect(servicePage).toContain('emptyOrderSearch()')
    expect(servicePage).not.toContain("const directShipForm = ref({ source_name: '', total_rows: 0, note: '' })")
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
