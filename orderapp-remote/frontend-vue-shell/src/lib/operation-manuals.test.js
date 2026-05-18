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
      greenBeanSalesManual: 'OP_MANUAL_GREEN_BEAN_SALES.md',
      financeManual: 'OP_MANUAL_FINANCE.md',
      settingsAuditManual: 'OP_MANUAL_SETTINGS_AUDIT.md',
      notificationManual: 'OP_MANUAL_NOTIFICATIONS.md',
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

test('manual markdown parser converts mermaid flowcharts into flowchart blocks', () => {
  const blocks = parseManualMarkdown(`## 流程图

\`\`\`mermaid
flowchart TD
  A["接收订单"] --> B["检查库存"]
  B{"库存是否足够"} -->|是| C["直接发货"]
  B -->|否| D["进入生产计划"]
\`\`\`

后续说明。`)

  assert.deepEqual(blocks, [
    { type: 'h2', text: '流程图' },
    {
      type: 'flowchart',
      direction: 'TD',
      source: 'flowchart TD\n  A["接收订单"] --> B["检查库存"]\n  B{"库存是否足够"} -->|是| C["直接发货"]\n  B -->|否| D["进入生产计划"]',
      nodes: {
        A: { id: 'A', label: '接收订单', shape: 'step' },
        B: { id: 'B', label: '库存是否足够', shape: 'decision' },
        C: { id: 'C', label: '直接发货', shape: 'step' },
        D: { id: 'D', label: '进入生产计划', shape: 'step' },
      },
      edges: [
        { from: 'A', to: 'B', label: '' },
        { from: 'B', to: 'C', label: '是' },
        { from: 'B', to: 'D', label: '否' },
      ],
    },
    { type: 'p', text: '后续说明。' },
  ])
})
