import test from 'node:test'
import assert from 'node:assert/strict'
import { salesOrderPageUrl, salesOrderDownloadUrl, salesOrderImageDownloadUrl } from './sales-order.js'
import { buildShareResourcePayload, shareResourceToWechat } from './external-share.js'
import { beginSalesOrderSealDrag, moveSalesOrderSealDrag } from './sales-order-seal.js'

test('salesOrderPageUrl keeps order id in vue shell', () => {
  assert.equal(salesOrderPageUrl(12), '/vue-shell?view=salesOrder&order_id=12')
})

test('salesOrderDownloadUrl points to latest pdf', () => {
  assert.equal(salesOrderDownloadUrl(12), '/orders/12/sales-order-latest.pdf')
})

test('salesOrderImageDownloadUrl points to latest png image', () => {
  assert.equal(salesOrderImageDownloadUrl(12), '/orders/12/sales-order-image-latest.png')
})

test('sales order uses the shared external share payload for pdf and image resources', () => {
  assert.deepEqual(buildShareResourcePayload('sales_order_pdf', 12), {
    resource_type: 'sales_order_pdf',
    order_id: 12,
    latest: true,
  })
  assert.deepEqual(buildShareResourcePayload('sales_order_image', 12, { documentID: 99, latest: false }), {
    resource_type: 'sales_order_image',
    order_id: 12,
    document_id: 99,
    latest: false,
  })
})

test('wechat share uses native share when available and falls back to copying the share link', async () => {
  const shared = []
  const result = await shareResourceToWechat({
    title: '销售单 SO-1',
    share_url: '/share/token-1',
    share_text: '销售单 SO-1\n/share/token-1',
  }, {
    origin: 'https://erp.example.com',
    navigator: { share: async (payload) => shared.push(payload) },
  })

  assert.equal(result, 'shared')
  assert.deepEqual(shared, [{
    title: '销售单 SO-1',
    text: '销售单 SO-1\nhttps://erp.example.com/share/token-1',
    url: 'https://erp.example.com/share/token-1',
  }])

  const copied = []
  const fallback = await shareResourceToWechat({
    title: '销售单 SO-2',
    share_url: 'https://erp.example.com/share/token-2',
  }, {
    origin: 'https://erp.example.com',
    navigator: { clipboard: { writeText: async (text) => copied.push(text) } },
  })

  assert.equal(fallback, 'copied')
  assert.deepEqual(copied, ['https://erp.example.com/share/token-2'])
})

test('sales order seal drag keeps the clicked offset instead of snapping the seal away', () => {
  const drag = beginSalesOrderSealDrag({
    seal: { x_mm: 32, y_mm: 22, width_mm: 42 },
    clientX: 146.3,
    clientY: 83.4,
    scale: 2.2,
  })

  assert.deepEqual(moveSalesOrderSealDrag(drag, { clientX: 146.3, clientY: 83.4 }), {
    x_mm: 32,
    y_mm: 22,
    width_mm: 42,
  })
  assert.deepEqual(moveSalesOrderSealDrag(drag, { clientX: 168.3, clientY: 105.4 }), {
    x_mm: 42,
    y_mm: 32,
    width_mm: 42,
  })
})
