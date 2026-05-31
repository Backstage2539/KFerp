import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { menuGroups, primaryMenuKeys } from './menu-ia.js'
import {
  CUSTOMER_VIEW_CONTEXT,
  EXTERNAL_CUSTOMER_VIEW_CONTEXT,
  FACTORY_VIEW_CONTEXT,
  ORDER_VIEW_CONTEXT,
  currentViewLabel,
  customerIDForViewContext,
  externalCustomerViewContext,
  legacyWorkspaceModeForViewContext,
  menuGroupsForViewContext,
  normalizeViewContext,
  viewContextFromURL,
  viewContextToURLParams,
  viewContextViewParams,
} from './view-context.js'

test('view context parses factory, legacy customer URL and order URL', () => {
  assert.deepEqual(
    normalizeViewContext(viewContextFromURL('https://erp.example/app/?view=orders')),
    { type: FACTORY_VIEW_CONTEXT },
  )

  const legacyCustomer = normalizeViewContext(
    viewContextFromURL('https://erp.example/app/?workspace=customer&customer_id=18'),
  )
  assert.equal(legacyCustomer.type, CUSTOMER_VIEW_CONTEXT)
  assert.equal(legacyCustomer.customerID, 18)
  assert.equal(legacyWorkspaceModeForViewContext(legacyCustomer), 'customer')
  assert.equal(customerIDForViewContext(legacyCustomer), 18)

  const order = normalizeViewContext(
    viewContextFromURL('https://erp.example/app/?view_context=order&order_id=99&order_no=SO-20260531-001&customer_id=18&customer_name=Karen'),
  )
  assert.equal(order.type, ORDER_VIEW_CONTEXT)
  assert.equal(order.orderID, 99)
  assert.equal(order.orderNo, 'SO-20260531-001')
  assert.equal(order.customerID, 18)
  assert.equal(order.customerName, 'Karen')
  assert.equal(legacyWorkspaceModeForViewContext(order), 'customer')
  assert.equal(customerIDForViewContext(order), 18)
})

test('view context writes canonical URL params while preserving legacy customer compatibility', () => {
  assert.deepEqual(viewContextToURLParams({ type: FACTORY_VIEW_CONTEXT }), {})
  assert.deepEqual(
    viewContextToURLParams({ type: CUSTOMER_VIEW_CONTEXT, customerID: 18 }),
    { view_context: 'customer', workspace: 'customer', customer_id: '18' },
  )
  assert.deepEqual(
    viewContextToURLParams({
      type: ORDER_VIEW_CONTEXT,
      orderID: 99,
      orderNo: 'SO-20260531-001',
      customerID: 18,
    }),
    {
      view_context: 'order',
      order_id: '99',
      order_no: 'SO-20260531-001',
      customer_id: '18',
    },
  )
})

test('view context injects customer and order params into routed pages', () => {
  assert.deepEqual(
    viewContextViewParams({ scope: 'all' }, { type: CUSTOMER_VIEW_CONTEXT, customerID: 18 }),
    { scope: 'all', customer_id: '18' },
  )
  assert.deepEqual(
    viewContextViewParams(
      { scope: 'fulfillment' },
      { type: ORDER_VIEW_CONTEXT, orderID: 99, orderNo: 'SO-20260531-001', customerID: 18 },
    ),
    { scope: 'fulfillment', customer_id: '18', order_id: '99', order_no: 'SO-20260531-001' },
  )
})

test('external customer view context is fixed to the bound customer', () => {
  const ctx = externalCustomerViewContext({ customer_id: 18, customer_name: 'Karen' })

  assert.equal(ctx.type, EXTERNAL_CUSTOMER_VIEW_CONTEXT)
  assert.equal(ctx.customerID, 18)
  assert.equal(ctx.customerName, 'Karen')
  assert.equal(legacyWorkspaceModeForViewContext(ctx), 'customer')
  assert.equal(currentViewLabel(ctx), '客户：Karen')
})

test('view context reuses existing permission-filtered menus without introducing product model names', () => {
  const factory = menuGroupsForViewContext(menuGroups, { type: FACTORY_VIEW_CONTEXT })
  const customer = menuGroupsForViewContext(menuGroups, { type: CUSTOMER_VIEW_CONTEXT, customerID: 18 })
  const order = menuGroupsForViewContext(menuGroups, { type: ORDER_VIEW_CONTEXT, orderID: 99, customerID: 18 })

  assert.equal(primaryMenuKeys(factory).includes('customerFulfillment'), false)
  assert.deepEqual(customer.map((group) => group.name), ['客户账户', '客户商品与配方', '客户财务'])
  assert.deepEqual(order.map((group) => group.name), ['客户账户', '客户商品与配方', '客户财务'])
  assert.equal(primaryMenuKeys(customer).includes('producePlan'), false)
  assert.equal(primaryMenuKeys(order).includes('producePlan'), false)
})

test('vue shell exposes current view selector and passes view context to pages', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  for (const marker of [
    '当前视图',
    'view-context-switcher',
    'view-context-label',
    'showViewContextSelector',
    'currentViewContext',
    ':view-context="currentViewContext"',
    '/api/view-context/options?type=customer',
    '/api/view-context/options?type=order',
    '/api/view-context/presets',
    '保存当前视图',
    '停用视图',
    '恢复默认视图',
    'external_customer',
    'view_context',
    'workspace=customer',
  ]) {
    assert.ok(source.includes(marker), `App.vue should include ${marker}`)
  }
})
