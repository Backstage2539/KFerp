import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../components/DirectShipCancelButton.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'direct-ship-cancel-runtime' }).content
code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)

test('取消通过客户范围专用API，成功后才刷新；已发或已结拒绝时保留错误', async () => {
  const originalWindow = globalThis.window
  const originalFetch = globalThis.fetch
  const events = []
  let editor
  try {
    globalThis.window = { confirm: () => true, location: { origin: 'https://example.test', pathname: '/app/vue-shell' } }
    await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 77, orderId: 125 }, { expose() {}, emit(...args) { events.push(args) } }); return () => h('div') } }))
    globalThis.fetch = async (url, options) => {
      assert.equal(new URL(url).pathname, '/app/api/customer-fulfillment/77/direct-ship/orders/125/cancel')
      assert.equal(options.method, 'POST')
      return new Response(JSON.stringify({ error: '已发货订单不能取消' }), { status: 409 })
    }
    await editor.cancel()
    assert.deepEqual(events, [])
    assert.equal(editor.error.value, '已发货订单不能取消')
    globalThis.fetch = async () => new Response(JSON.stringify({ status: 'cancelled' }), { status: 200 })
    await editor.cancel()
    assert.deepEqual(events, [['cancelled', 125]])
    assert.equal(editor.cancelling.value, false)
  } finally { globalThis.window = originalWindow; globalThis.fetch = originalFetch }
})
