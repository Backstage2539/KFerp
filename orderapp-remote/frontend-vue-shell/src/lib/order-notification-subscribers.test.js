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

test('按姓名或手机号查询订阅名单并对指定账号提醒，不能把未绑定当作已关注', async () => {
  const calls = [], originalFetch = globalThis.fetch
  globalThis.fetch = async (url, options = {}) => {
    const parsed = new URL(String(url), window.location.origin)
    calls.push({ path: parsed.pathname, q: parsed.searchParams.get('q'), method: options.method || 'GET', body: options.body })
    if (parsed.pathname.endsWith('/subscribers')) return response({items:[
      {mini_user_id:12, customer_id:3, employee_id:0, name:'测试客户', identity_type:'customer', identity_active:true, choice_recorded:true, choice_enabled:true, customer_phone:'13300000001', account_phone:'13300000009', has_binding:true, binding_count:3, active_binding_count:2, subscribed_binding_count:1, enabled_binding_count:1, active:true, subscribed:true, notifications_enabled:true, events:['shipment_created'], prompt_required:false},
      {mini_user_id:0, customer_id:4, employee_id:0, name:'待绑定客户', identity_type:'customer', identity_active:true, customer_phone:'13300000002', has_binding:false, active:false, subscribed:false, notifications_enabled:false, events:[], prompt_required:false},
      {mini_user_id:21, customer_id:0, employee_id:9, name:'测试销售', identity_type:'employee', identity_active:false, phone:'13300000003', active:false, subscribed:false, notifications_enabled:false, events:[], prompt_required:false},
      {mini_user_id:22, customer_id:7, employee_id:0, name:'已选择待绑定', identity_type:'customer', identity_active:true, choice_recorded:true, choice_enabled:true, has_binding:false, active:false, subscribed:false, notifications_enabled:false, events:['shipment_created'], prompt_required:false},
    ]})
    if (parsed.pathname.endsWith('/subscribers/reselect')) return response({ok:true})
    return response({ config:{}, devices:[], deliveries:[], official_ready:true })
  }
  const app = createApp(Component)
  try {
    app.mount(document.querySelector('#app')); await flush()
    const field = document.querySelector('[aria-label="搜索客户、员工或手机号"]')
    assert.ok(field, '应提供客户、员工和手机号搜索')
    field.value='13300000001'; field.dispatchEvent(new Event('input', {bubbles:true}))
    document.querySelector('[data-testid="subscriber-search"]').click(); await flush()
    assert.ok(calls.some(c => c.path.endsWith('/subscribers') && c.q==='13300000001'))
    const rows=[...document.querySelectorAll('[data-testid="subscriber-row"]')]
    assert.equal(rows.length,4)
    assert.match(rows[0].textContent,/当前绑定 2/)
    assert.match(rows[0].textContent,/已关注 1/)
    assert.match(rows[0].textContent,/关联记录 3/)
    assert.match(rows[0].textContent,/订单发货/)
    assert.match(rows[0].textContent,/客户档案：13300000001/)
    assert.match(rows[0].textContent,/账号手机号：13300000009/)
    assert.doesNotMatch(rows[0].textContent,/已验证/)
    assert.match(rows[1].textContent,/未绑定/)
    assert.doesNotMatch(rows[1].textContent,/已关注/)
    assert.ok(rows[1].querySelector('button')?.disabled)
    assert.match(rows[2].textContent,/员工/)
    assert.match(rows[2].textContent,/身份或关联已失效/)
    assert.ok(rows[2].querySelector('button')?.disabled)
    assert.match(rows[3].textContent,/用户选择：接收所选通知/)
    assert.match(rows[3].textContent,/绑定通知开启：0/)
    let confirmations=0
    window.confirm=()=>{confirmations++;return false}
    rows[0].querySelector('button').click();await flush()
    assert.equal(calls.filter(c=>c.path.endsWith('/subscribers/reselect')).length,0)
    window.confirm=()=>{confirmations++;return true}
    rows[0].querySelector('button').click();await flush()
    const writes=calls.filter(c=>c.path.endsWith('/subscribers/reselect'))
    assert.equal(confirmations,2)
    assert.equal(writes.length,1)
    assert.deepEqual(JSON.parse(writes[0].body),{mini_user_id:12,customer_id:3,employee_id:0})
    assert.match(document.body.textContent,/下次进入小程序/)
  } finally {app.unmount();globalThis.fetch=originalFetch}
})

test('服务号未就绪时不能对个人发起无法出现的提醒', async () => {
  const originalFetch=globalThis.fetch
  globalThis.fetch=async url=>{
    const path=new URL(String(url),window.location.origin).pathname
    return response(path.endsWith('/subscribers')?{items:[{mini_user_id:12,customer_id:3,employee_id:0,name:'测试客户',identity_type:'customer',identity_active:true,events:[]}]}:{config:{},devices:[],deliveries:[],official_ready:false})
  }
  const app=createApp(Component)
  try {
    app.mount(document.querySelector('#app'));await flush()
    assert.equal(document.querySelector('[data-testid="subscriber-row"] button').disabled,true)
  } finally {app.unmount();globalThis.fetch=originalFetch}
})
