import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

async function portalSettings() {
  const file = new URL('../views/CustomerPortalSettingsView.vue', import.meta.url)
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
  let code = compileScript(descriptor, { id: 'customer-portal-refresh' }).content
  code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
  let vm
  await renderToString(createSSRApp({ setup() {
    vm = View.setup({ workspaceMode: '', customerContextId: 0, customerContextLabel: '' }, { expose() {} })
    return () => h('div')
  } }))
  return vm
}

const response = capabilities => ({ ok: true, status: 200, json: async () => ({ capabilities }) })
const automatic = [{ code: 'product_order', enabled: true, config: { required_by_fulfillment: true } }]
const ordinary = [{ code: 'product_order', enabled: false, config: {} }]

test('fulfillment refresh fetches a fresh summary and ignores the older response without losing portal drafts', async () => {
  const vm = await portalSettings()
  const row = vm.createPortalRow({ id: 97101, capability_template_key: 'ordinary', display_name: '原名称' })
  Object.assign(row.form, { display_name: '未保存名称', default_sender_id: 17, enabled: false, capability_template_key: 'draft-template' })
  const form = row.form
  const draft = { ...form }
  const bindings = row.bindings = [{ id: 23 }]
  vm.capabilityTemplates.value = [{ key: 'draft-template', capabilities: ordinary }]
  const originalFetch = globalThis.fetch
  const pending = []
  globalThis.fetch = url => new Promise(resolve => pending.push({ url, resolve }))
  const calls = []
  try {
    calls.push(vm.refreshRowCapabilities(row))
    calls.push(vm.refreshRowCapabilities(row))
    assert.equal(pending.length, 2, 'saving fulfillment must not reuse the pre-save in-flight GET')
    assert.notEqual(pending[0].url, pending[1].url)
    pending[1].resolve(response(ordinary))
    await calls[1]
    pending[0].resolve(response(automatic))
    await calls[0]
    assert.deepEqual(row.capabilities, ordinary)
    assert.deepEqual(vm.enabledTemplateCapabilities(row), [])
    assert.equal(row.form, form)
    assert.deepEqual(row.form, draft)
    assert.equal(row.bindings, bindings)
  } finally {
    for (const request of pending) request.resolve(response(automatic))
    await Promise.all(calls)
    globalThis.fetch = originalFetch
  }
})

test('capability refresh requests for different customers both remain current', async () => {
  const vm = await portalSettings()
  const rows = [97102, 97103].map(id => vm.createPortalRow({ id }))
  const originalFetch = globalThis.fetch
  const pending = []
  globalThis.fetch = url => new Promise(resolve => pending.push({ url, resolve }))
  const calls = rows.map(row => vm.refreshRowCapabilities(row))
  try {
    assert.equal(pending.length, 2)
    pending[1].resolve(response(ordinary))
    await calls[1]
    pending[0].resolve(response(automatic))
    await calls[0]
    assert.deepEqual(rows[0].capabilities, automatic)
    assert.deepEqual(rows[1].capabilities, ordinary)
  } finally {
    for (const request of pending) request.resolve(response([]))
    await Promise.all(calls)
    globalThis.fetch = originalFetch
  }
})

test('a failed latest refresh keeps the current summary and rejects a late older success', async () => {
  const vm = await portalSettings()
  const row = vm.createPortalRow({ id: 97104 })
  row.capabilities = ordinary
  const originalFetch = globalThis.fetch
  const pending = []
  globalThis.fetch = url => new Promise(resolve => pending.push({ url, resolve }))
  const calls = [vm.refreshRowCapabilities(row), vm.refreshRowCapabilities(row)]
  try {
    assert.equal(pending.length, 2)
    pending[1].resolve({ ok: false, status: 503, json: async () => ({ error: '暂时不可用' }) })
    await calls[1]
    pending[0].resolve(response(automatic))
    await calls[0]
    assert.equal(row.capabilities, ordinary)
  } finally {
    for (const request of pending) request.resolve(response([]))
    await Promise.all(calls)
    globalThis.fetch = originalFetch
  }
})
