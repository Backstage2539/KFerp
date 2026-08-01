import { describe, expect, it } from 'vitest'
import {
  buildMallOrderPath,
  buildMallPagePath,
  buildBeanListAckPath,
  buildBeanListPDFPath,
  buildBeanListPNGPath,
  buildMiniLoginPayload,
  buildPasswordLoginPath,
  buildEmployeeOrderFormPath,
  buildEmployeeOrderDetailPath,
  buildEmployeeOrderDocumentPath,
  buildEmployeeOrdersPath,
  buildEmployeeCustomersPath,
  buildEmployeeCustomerPath,
  buildEmployeeOrderDraftPath,
  buildCustomerProductsPath,
  buildCustomerProductCategoriesPath,
  buildCustomerProductCategoryPath,
  buildCustomerProductCategoryMovePath,
  buildCustomerProductCategoryAssignPath,
  buildServicePagePath,
  buildSwitchCustomerPath,
} from './customerPortal'
import type {
  CreateFulfillmentOrderPayload,
  EmployeeCustomersResponse,
  EmployeeOrderDetailResponse,
  EmployeeOrderDraftPayload,
  ProductSummary,
} from './customerPortal'

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

  it('exposes employee ERP order API paths', () => {
    expect(buildEmployeeOrderFormPath()).toBe('/api/mini/employee/order-form')
    expect(buildEmployeeOrdersPath()).toBe('/api/mini/employee/orders')
    expect(buildEmployeeOrderDetailPath(42)).toBe('/api/mini/employee/orders/42')
    expect(buildEmployeeOrderDocumentPath(42, 'sales-order', 'pdf')).toBe('/api/mini/employee/orders/42/documents/sales-order.pdf')
    expect(buildEmployeeOrderDocumentPath(42, 'sales-order', 'png')).toBe('/api/mini/employee/orders/42/documents/sales-order.png')
    expect(buildEmployeeOrderDocumentPath(42, 'delivery-note', 'pdf')).toBe('/api/mini/employee/orders/42/documents/delivery-note.pdf')
    expect(buildEmployeeOrderDocumentPath(42, 'delivery-note', 'png')).toBe('/api/mini/employee/orders/42/documents/delivery-note.png')
    expect(buildEmployeeCustomersPath()).toBe('/api/mini/employee/customers')
    expect(buildEmployeeCustomersPath({ q: '上海 客户', page: 2, limit: 100 })).toBe('/api/mini/employee/customers?q=%E4%B8%8A%E6%B5%B7%20%E5%AE%A2%E6%88%B7&page=2&limit=100')
    expect(buildEmployeeCustomerPath(31)).toBe('/api/mini/employee/customers/31')
    expect(buildEmployeeOrderDraftPath()).toBe('/api/mini/employee/order-draft')
  })

  it('types scoped employee customers and multi-item server drafts', () => {
    const customers: EmployeeCustomersResponse = {
      rows: [{ id: 8, name: '客户A', can_maintain: true }],
      sources: [{ id: 1, name: '小程序' }],
      order_types: [{ id: 2, name: '销售订单' }],
      employees: [{ id: 3, name: '销售A' }],
      customer_type_options: [{ value: 'wholesale', label: '批发客户' }],
      is_admin: false,
      total: 1,
      has_next: false,
    }
    const draft: EmployeeOrderDraftPayload = {
      order_date: '2026-08-01',
      customer_id: 8,
      source_id: 1,
      order_type_id: 2,
      pay_status_id: 0,
      ship_status_id: 0,
      receiver_name: '',
      receiver_phone: '',
      receiver_address: '',
      receiver_company: '',
      notes: '',
      items: [
        {
          key: 'line-1',
          product_family_key: '0:10:0',
          product_family_id: 10,
          customer_product_alias_id: 0,
          product_id: 11,
          product_name: '商品A',
          product_kind: 'roasted_bean',
          spec_label: '227g',
          spec_g: 227,
          sales_unit: '袋',
          unit_bag_count: 0,
          unit_bean_g: 0,
          qty: 2,
          unit_price: 48,
        },
        {
          key: 'line-2',
          product_family_key: '0:20:0',
          product_family_id: 20,
          customer_product_alias_id: 0,
          product_id: 21,
          product_name: '商品B',
          product_kind: 'roasted_bean',
          spec_label: '454g',
          spec_g: 454,
          sales_unit: '袋',
          unit_bag_count: 0,
          unit_bean_g: 0,
          qty: 1,
          unit_price: 80,
        },
      ],
    }

    expect(customers.rows[0]?.can_maintain).toBe(true)
    expect(draft.items).toHaveLength(2)
  })

  it('types the complete employee order detail and four authenticated document outputs', () => {
    const response: EmployeeOrderDetailResponse = {
      order: {
        id: 42,
        order_no: 'SO-42',
        document_date: '2026-08-01',
        order_date: '2026-08-01',
        customer: '客户A',
        grand_total: '96.00',
        pay_status: '待收款',
        ship_status: '待发货',
        process_status: '待生产',
        responsible_name: '销售甲',
        created_by_employee: '录单乙',
        items: [{
          item_id: 1,
          product_id: 8,
          product_name: '商品A',
          spec: '227',
          qty: '2',
          unit: '袋',
          unit_price: '48.00',
          line_total: '96.00',
          bean_list_version_no: 'V3',
        }],
      },
      documents: {
        sales_order_pdf: { available: true, path: '/api/mini/employee/orders/42/documents/sales-order.pdf', content_type: 'application/pdf' },
        sales_order_png: { available: true, path: '/api/mini/employee/orders/42/documents/sales-order.png', content_type: 'image/png' },
        delivery_note_pdf: { available: false },
        delivery_note_png: { available: false },
      },
    }

    expect(response.order.items[0]?.line_total).toBe('96.00')
    expect(response.documents?.sales_order_png?.content_type).toBe('image/png')
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
