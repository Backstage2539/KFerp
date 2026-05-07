import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  manualDocNameForView,
  operationManualsByView,
  parseManualMarkdown,
} from './operation-manuals.js'

test('operation manual view keys map to deployed OP_MANUAL docs', () => {
  assert.deepEqual(
    Object.fromEntries(Object.entries(operationManualsByView).map(([key, item]) => [key, item.doc])),
    {
      orderSalesManual: 'OP_MANUAL_ORDER_SALES.md',
      productionManual: 'OP_MANUAL_PRODUCTION.md',
      inventoryMaterialsManual: 'OP_MANUAL_INVENTORY_MATERIALS.md',
      costingManual: 'OP_MANUAL_COSTING.md',
      financeManual: 'OP_MANUAL_FINANCE.md',
      settingsAuditManual: 'OP_MANUAL_SETTINGS_AUDIT.md',
      customerPortalManual: 'OP_MANUAL_CUSTOMER_PORTAL.md',
      customerFulfillmentManual: 'OP_MANUAL_CUSTOMER_FULFILLMENT.md',
      requirementsManual: 'OP_MANUAL_REQUIREMENTS.md',
    },
  )
  assert.equal(manualDocNameForView('orderSalesManual'), 'OP_MANUAL_ORDER_SALES.md')
  assert.equal(manualDocNameForView('orders'), '')
})

test('manual markdown parser keeps headings, lists and quoted notes readable', () => {
  const blocks = parseManualMarkdown(`# 标题

> 重要说明

## 入口
- 录单
- 订单列表

## 标准操作
1. 选择客户
2. 保存订单

普通说明。`)

  assert.deepEqual(blocks, [
    { type: 'h1', text: '标题' },
    { type: 'quote', text: '重要说明' },
    { type: 'h2', text: '入口' },
    { type: 'ul', items: ['录单', '订单列表'] },
    { type: 'h2', text: '标准操作' },
    { type: 'ol', items: ['选择客户', '保存订单'] },
    { type: 'p', text: '普通说明。' },
  ])
})
