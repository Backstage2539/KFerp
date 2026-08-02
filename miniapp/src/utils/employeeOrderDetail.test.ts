import { describe, expect, it } from 'vitest'
import {
  employeeOrderDocumentAsset,
  employeeOrderFeeLines,
  employeeOrderItemDisplayName,
  employeeOrderItemPriceSourceLabel,
  employeeOrderItemSpecLabel,
  employeeOrderInvoiceStatusLabel,
  employeeOrderDetailPagePath,
  employeeOrderNavigationRows,
  employeeOrderTraceSourceLines,
  employeeOrderListQuery,
  rememberEmployeeOrderListQuery,
} from './employeeOrderDetail'

describe('employee order detail presentation', () => {
  it('builds a safe native detail-page path from an order id', () => {
    expect(employeeOrderDetailPagePath(42)).toBe('/pages/employee-order-detail/employee-order-detail?id=42')
    expect(employeeOrderDetailPagePath('42')).toBe('/pages/employee-order-detail/employee-order-detail?id=42')
    expect(employeeOrderDetailPagePath(0)).toBe('')
    expect(employeeOrderDetailPagePath(-1)).toBe('')
    expect(employeeOrderDetailPagePath('not-an-id')).toBe('')
  })

  it('prepares a native detail-page url without dropping malformed order rows', () => {
    expect(employeeOrderNavigationRows([
      { id: 42, order_no: 'SO-42' },
      { id: 0, order_no: 'SO-BAD' },
      { order_no: 'SO-MISSING' },
    ])).toEqual([
      {
        id: 42,
        order_no: 'SO-42',
        detail_url: '/pages/employee-order-detail/employee-order-detail?id=42',
      },
      { id: 0, order_no: 'SO-BAD', detail_url: '' },
      { order_no: 'SO-MISSING', detail_url: '' },
    ])
  })

  it('maps document availability from the detail response without inventing a version', () => {
    const documents = {
      sales_order: {
        pdf: { available: true, version_no: 3, filename: 'SO-42-v3.pdf' },
        png: { available: false },
      },
      delivery_note: {},
    }

    expect(employeeOrderDocumentAsset(documents, 'sales-order', 'pdf')).toMatchObject({
      available: true,
      version_no: 3,
    })
    expect(employeeOrderDocumentAsset(documents, 'sales-order', 'png')).toEqual({ available: false })
    expect(employeeOrderDocumentAsset(documents, 'delivery-note', 'pdf')).toBeUndefined()
  })

  it('accepts the backend flat document keys used by the employee detail contract', () => {
    const documents = {
      sales_order_pdf: { available: true, path: '/sales.pdf', content_type: 'application/pdf' },
      sales_order_png: { available: true, path: '/sales.png', content_type: 'image/png' },
      delivery_note_pdf: { available: false, path: '', content_type: 'application/pdf' },
      delivery_note_png: { available: true, path: '/delivery.png', content_type: 'image/png' },
    }

    expect(employeeOrderDocumentAsset(documents, 'sales-order', 'png')?.path).toBe('/sales.png')
    expect(employeeOrderDocumentAsset(documents, 'delivery-note', 'pdf')?.available).toBe(false)
    expect(employeeOrderDocumentAsset(documents, 'delivery-note', 'png')?.path).toBe('/delivery.png')
  })

  it('uses order snapshots for item name, spec and price-table version', () => {
    const item = {
      product_name: '当前商品名',
      customer_product_display_name_snapshot: '客户商品名',
      product_name_snapshot: '下单商品名',
      product_kind: 'drip_bag',
      spec: '100',
      sales_unit: 'box',
      unit_bag_count: 10,
      unit_conversion_label: '10袋/盒',
      bean_list_version_no: 'V2.3',
      price_source_json: JSON.stringify({ source: '商用价格表', version: 'V2.3' }),
    }

    expect(employeeOrderItemDisplayName(item)).toBe('客户商品名')
    expect(employeeOrderItemSpecLabel(item)).toBe('10袋/盒')
    expect(employeeOrderItemPriceSourceLabel(item)).toBe('商用价格表 · V2.3')
  })

  it('keeps zero-valued core fees and exposes non-zero outsource fees', () => {
    const lines = employeeOrderFeeLines({
      total_amount: '100.00',
      shipping_amount: '0.00',
      discount_amount: '5.00',
      grand_total: '95.00',
      express_fee: '顺丰到付',
      outsource_total_fee: '3.00',
      outsource_packaging_fee: '3.00',
      outsource_roast_fee: '0.00',
    })

    expect(lines.map((line) => line.label)).toEqual([
      '商品金额', '运费', '优惠', '应收', '快递费', '委外合计', '委外包装',
    ])
    expect(lines.find((line) => line.label === '运费')?.value).toBe('¥0.00')
    expect(lines.find((line) => line.label === '快递费')?.value).toBe('顺丰到付')
  })

  it('formats quotation and production sources into mobile-readable rows', () => {
    expect(employeeOrderTraceSourceLines({
      price_list_version: 'V8',
      final_unit_price: '48.00',
      price_unit: '袋',
      manual_adjusted: true,
      source_label: '客户价格表',
    }, 'quote')).toEqual(['价格表：V8', '最终价：48.00/袋', '人工调整', '客户价格表'])

    expect(employeeOrderTraceSourceLines({
      bom_version_no: 'BOM-V3',
      process_route_name: '标准烘焙',
      work_order_no: 'WO-9',
    }, 'production')).toEqual(['BOM：BOM-V3', '工艺：标准烘焙', '工单：WO-9'])
  })

  it('retains the list query while navigating to a detail page and back', () => {
    rememberEmployeeOrderListQuery('金色山脉')
    expect(employeeOrderListQuery()).toBe('金色山脉')
    rememberEmployeeOrderListQuery('')
    expect(employeeOrderListQuery()).toBe('')
  })

  it('uses the same Chinese invoice status labels as the web order detail', () => {
    expect(employeeOrderInvoiceStatusLabel('requested')).toBe('已申请')
    expect(employeeOrderInvoiceStatusLabel('uploaded')).toBe('已上传')
    expect(employeeOrderInvoiceStatusLabel('')).toBe('未申请')
  })
})
