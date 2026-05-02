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

test('wechat share sends the generated file instead of a share link', async () => {
  class TestFile extends Blob {
    constructor(parts, name, options = {}) {
      super(parts, options)
      this.name = name
    }
  }

  const fetched = []
  const shared = []
  const result = await shareResourceToWechat({
    title: '销售单 SO-1',
    share_url: '/share/token-1',
    file_url: '/share/token-1/file',
    filename: 'sales-order-SO-1.pdf',
    content_type: 'application/pdf',
  }, {
    origin: 'https://erp.example.com',
    File: TestFile,
    fetch: async (url) => {
      fetched.push(url)
      return {
        ok: true,
        blob: async () => new Blob(['pdf-data'], { type: 'application/pdf' }),
      }
    },
    navigator: {
      canShare: (payload) => payload.files?.[0]?.name === 'sales-order-SO-1.pdf',
      share: async (payload) => shared.push(payload),
    },
  })

  assert.equal(result, 'file-shared')
  assert.deepEqual(fetched, ['https://erp.example.com/share/token-1/file'])
  assert.equal(shared.length, 1)
  assert.equal(shared[0].title, '销售单 SO-1')
  assert.equal(shared[0].text, '销售单 SO-1')
  assert.equal(shared[0].url, undefined)
  assert.equal(shared[0].files.length, 1)
  assert.equal(shared[0].files[0].name, 'sales-order-SO-1.pdf')
  assert.equal(shared[0].files[0].type, 'application/pdf')
})

test('wechat share reports unsupported when the browser cannot share files and does not copy a link', async () => {
  class TestFile extends Blob {
    constructor(parts, name, options = {}) {
      super(parts, options)
      this.name = name
    }
  }

  const copied = []
  const shared = []
  const result = await shareResourceToWechat({
    title: '销售单 SO-2',
    share_url: 'https://erp.example.com/share/token-2',
    file_url: 'https://erp.example.com/share/token-2/file',
    filename: 'sales-order-SO-2.png',
    content_type: 'image/png',
  }, {
    origin: 'https://erp.example.com',
    File: TestFile,
    fetch: async () => ({
      ok: true,
      blob: async () => new Blob(['png-data'], { type: 'image/png' }),
    }),
    navigator: {
      canShare: () => false,
      share: async (payload) => shared.push(payload),
      clipboard: { writeText: async (text) => copied.push(text) },
    },
  })

  assert.equal(result, 'unsupported')
  assert.deepEqual(shared, [])
  assert.deepEqual(copied, [])
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
