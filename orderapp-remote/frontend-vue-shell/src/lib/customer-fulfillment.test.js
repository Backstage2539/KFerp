import test from 'node:test'
import assert from 'node:assert/strict'

import {
  activeCustomerFulfillmentCustomers,
  customerFulfillmentSubmitCopy,
  customerFulfillmentOrderFees,
  customerFulfillmentPublicSKUOrderMode,
  customerFulfillmentWorkbenchSections,
  customerFulfillmentCustomerOptionLabel,
  customerFulfillmentCustomerOptionMeta,
} from './customer-fulfillment.js'

test('customer fulfillment workbench follows enabled customer capabilities', () => {
  const publicSkuCapabilities = ['product_order', 'direct_ship', 'settlement']

  assert.deepEqual(customerFulfillmentWorkbenchSections(publicSkuCapabilities), {
    processing: false,
    directShip: true,
    inventory: false,
    settlement: true,
    orders: true,
  })

})

test('customer fulfillment submit copy distinguishes public sku order mode and one-piece direct-ship mode', () => {
  assert.equal(customerFulfillmentPublicSKUOrderMode(['product_order', 'direct_ship']), true)
  assert.deepEqual(customerFulfillmentSubmitCopy(['product_order', 'direct_ship']), {
    formTitle: '提交订单信息',
    submitButton: '提交订单',
    successPrefix: '已提交订单',
    errorFallback: '提交订单失败',
    notePlaceholder: '订单要求',
  })
  assert.equal(customerFulfillmentPublicSKUOrderMode(['direct_ship']), false)
  assert.deepEqual(customerFulfillmentSubmitCopy(['direct_ship']), {
    formTitle: '提交代发信息',
    submitButton: '提交代发',
    successPrefix: '已提交代发',
    errorFallback: '提交代发失败',
    notePlaceholder: '发货要求',
  })
})

test('customer fulfillment customer selector labels active customer options for humans', () => {
  const rows = activeCustomerFulfillmentCustomers({
    customers: [
      { id: 147, name: '誉观山', company_name: '誉观山咖啡', contact: '王总', phone: '13800138075', active: true },
      { id: 148, name: '停用客户', active: false },
    ],
  })

  assert.equal(rows.length, 1)
  assert.equal(customerFulfillmentCustomerOptionLabel(rows[0]), '誉观山')
  assert.equal(customerFulfillmentCustomerOptionMeta(rows[0]), '誉观山咖啡 / 王总 / 13800138075')
})

test('customer fulfillment order fees keep ERP-style fee breakdown for the bottom order list', () => {
  assert.deepEqual(customerFulfillmentOrderFees({
    total_amount: '128.00',
    shipping_amount: '12.00',
    discount_amount: '5.00',
    grand_total: '135.00',
  }), [
    { label: '商品', value: '128.00' },
    { label: '运费', value: '12.00' },
    { label: '优惠', value: '5.00' },
    { label: '应收', value: '135.00', emphasized: true },
  ])
})
