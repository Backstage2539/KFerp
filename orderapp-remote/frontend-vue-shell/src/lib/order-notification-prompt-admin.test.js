import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom = new JSDOM('<div id="app"></div>', { url: 'http://erp.test/app/vue-shell', pretendToBeVisual: true })
for (const key of ['window', 'document', 'Node', 'Element', 'HTMLElement', 'SVGElement', 'Event']) globalThis[key] = dom.window[key]
const { createApp, nextTick } = await import('vue')
const file = new URL('../components/OrderNotificationSettings.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
let source = compileScript(descriptor, { id: file.pathname, inlineTemplate: true }).content
const apiFile = new URL('../api/message-center.js', file)
const apiSource = fs.readFileSync(apiFile, 'utf8').replace("'./client'", JSON.stringify(new URL('./client.js', apiFile).href))
const apiURL = `data:text/javascript;base64,${Buffer.from(apiSource).toString('base64')}`
source = source.replace("'../api/message-center.js'", JSON.stringify(apiURL))
source = source.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
const Component = (await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`)).default
const flush = async () => { for (let i = 0; i < 5; i++) { await nextTick(); await new Promise(resolve => setImmediate(resolve)) } }
const response = data => ({ ok: true, status: 200, json: async () => data })

test('后台重选提醒须确认，取消不写入，确认后发起且不改用户选择', async () => {
  const calls = [], originalFetch = globalThis.fetch
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(String(url), window.location.origin).pathname
    calls.push({ path, method: options.method || 'GET', body: options.body })
    return response(path.endsWith('/prompt/reselect') ? { version: 2 } : { config: {}, devices: [], deliveries: [], official_ready: true })
  }
  const app = createApp(Component)
  try {
    app.mount(document.querySelector('#app')); await flush()
    const findButton = () => [...document.querySelectorAll('button')].find(b => b.textContent.trim() === '提醒用户重新选择通知')
    assert.ok(findButton(), '管理员应能发起重选提醒')
    window.confirm = () => false
    findButton().click(); await flush()
    assert.equal(calls.filter(c => c.path.endsWith('/prompt/reselect')).length, 0)
    window.confirm = () => true
    findButton().click(); await flush()
    const writes = calls.filter(c => c.path.endsWith('/prompt/reselect'))
    assert.equal(writes.length, 1)
    assert.equal(writes[0].method, 'POST')
    assert.deepEqual(JSON.parse(writes[0].body), {})
    assert.match(document.body.textContent, /下次进入小程序.*重新选择/)
  } finally { app.unmount(); globalThis.fetch = originalFetch }
})
