import { describe, expect, it } from 'vitest'
import {
  buildMallOrderPath,
  buildMallPagePath,
  buildBeanListAckPath,
  buildBeanListPDFPath,
  buildBeanListPNGPath,
  buildMiniLoginPayload,
  buildPasswordLoginPath,
  buildCustomerProductsPath,
  buildCustomerProductCategoriesPath,
  buildCustomerProductCategoryPath,
  buildCustomerProductCategoryMovePath,
  buildCustomerProductCategoryAssignPath,
  buildServicePagePath,
  buildSwitchCustomerPath,
} from './customerPortal'
import type { CreateFulfillmentOrderPayload, ProductSummary } from './customerPortal'

describe('customer portal API helpers', () => {
  it('encodes service page filters into the mini service path', () => {
    expect(
      buildServicePagePath('orders', {
        q: '乌拉嘎 上海',
        date_from: '2026-05-01',
        date_to: '2026-05-03',
        process_status: '生产中',
        pay_status: '已收款',
        ship_status: '待发货',
      }),
    ).toBe('/api/mini/services/orders?q=%E4%B9%8C%E6%8B%89%E5%98%8E%20%E4%B8%8A%E6%B5%B7&date_from=2026-05-01&date_to=2026-05-03&process_status=%E7%94%9F%E4%BA%A7%E4%B8%AD&pay_status=%E5%B7%B2%E6%94%B6%E6%AC%BE&ship_status=%E5%BE%85%E5%8F%91%E8%B4%A7')
  })

  it('does not add a query string when no filters are set', () => {
    expect(buildServicePagePath('beanList', {})).toBe('/api/mini/services/beanList')
  })

  it('exposes stable mini mall API paths', () => {
    expect(buildMallPagePath()).toBe('/api/mini/mall')
    expect(buildMallOrderPath()).toBe('/api/mini/mall/orders')
  })

  it('builds phone verify login payloads', () => {
    expect(buildMiniLoginPayload('phone_verify', { code: 'wx-code', phoneCode: 'phone-code', nickname: '客户A' })).toEqual({
      mode: 'phone_verify',
      code: 'wx-code',
      phone_code: 'phone-code',
      nickname: '客户A',
    })
  })

  it('exposes the current-customer switch API path', () => {
    expect(buildSwitchCustomerPath()).toBe('/api/mini/current-customer')
  })

  it('exposes the ERP password login API path', () => {
    expect(buildPasswordLoginPath()).toBe('/api/mini/login/password')
  })

  it('exposes customer products and category management mini API paths', () => {
    expect(buildCustomerProductsPath()).toBe('/api/mini/customer-products')
    expect(buildCustomerProductCategoriesPath()).toBe('/api/mini/customer-products/categories')
    expect(buildCustomerProductCategoryPath(31)).toBe('/api/mini/customer-products/categories/31')
    expect(buildCustomerProductCategoryMovePath(31)).toBe('/api/mini/customer-products/categories/31/move')
    expect(buildCustomerProductCategoryAssignPath(501)).toBe('/api/mini/customer-products/501/category')
    expect(buildBeanListPDFPath(11)).toBe('/api/mini/bean-lists/11.pdf')
    expect(buildBeanListPNGPath(11)).toBe('/api/mini/bean-lists/11.png')
  })

  it('types mini product and fulfillment payload drip unit metadata', () => {
    const product: ProductSummary = {
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
    }
    const payload: CreateFulfillmentOrderPayload = {
      service_code: 'product_order',
      recipient_name: '张三',
      recipient_phone: '13800138000',
      recipient_address: '上海市',
      product_id: product.id,
      spec_g: 120,
      qty: 3,
      sales_unit: 'box',
      unit_bag_count: 12,
      unit_bean_g: 10,
    }

    expect(product.sales_units).toEqual(['bag', 'box'])
    expect(payload).toMatchObject({ sales_unit: 'box', unit_bag_count: 12, unit_bean_g: 10 })
  })
})
