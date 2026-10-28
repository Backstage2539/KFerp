import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../components/CustomerDirectShipForm.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'customer-direct-ship-runtime' }).content
code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)

test('客户侧代发使用本人范围接口并复用新代发表单', async () => {
  let editor
  await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 307, customerLabel: '喜沁咖啡', customerPortal: true }, { expose() {}, emit() {} }); return () => h('div') } }))
  assert.equal(editor.baseURL.value, '/api/customer-processing/portal')
  assert.equal(editor.settingsURL.value, '/api/customer-processing/portal/direct-ship/settings')
  assert.equal(editor.addressURL.value, '/api/customer-processing/portal/recipient-addresses')
  const portalSource = fs.readFileSync(new URL('../views/CustomerProcessingPortalView.vue', import.meta.url), 'utf8')
  assert.match(portalSource, /<CustomerDirectShipForm[\s\S]*?:customer-portal="customerAccountActor"/)
})

test('ERP真实代发表单保留地址地区、库存场景和已录商品', async () => {
  let editor
  await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 77, customerLabel: '喜沁咖啡' }, { expose() {}, emit() {} }); return () => h('div') } }))
  editor.mode.value = 'stock'
  const family = { parent_product_id: 10, name: '曜石 2.0', specs: [{ product_id: 11, bom_spec_id: 12, bom_variant_id: 13, spec_name: '454g', inventory_unit: '袋', physical_available_qty: 4, pending_inbound_qty: 20, pending_reserved_qty: 0 }] }
  editor.catalog.value = { product_families: [family] }
  editor.selectFamily(editor.lines.value[0], family)
  editor.lines.value[0].qty = 12
  editor.selectRecipient({ receiver_name: '测试收件人', receiver_phone: '13800000000', receiver_address: '云南省大理市测试路1号', receiver_company: '测试门店' }, { province: '云南省', city: '大理市', district: '', detail_address: '测试路1号' })
  const payload = editor.command()
  assert.equal(payload.fulfillment_mode, 'stock')
  assert.equal(payload.province, '云南省')
  assert.equal(payload.city, '大理市')
  assert.equal(payload.detail_address, '测试路1号')
  assert.equal(payload.items[0].qty, 12)
  assert.equal(payload.items[0].bom_variant_id, 13)
  assert.equal(editor.quantityFor(editor.lines.value[0]).gap, 0)
})

test('补购仅带服务端确认的权益缺口，并保留原代发数量', async () => {
  let editor
  await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 78, customerLabel: '喜沁咖啡' }, { expose() {}, emit() {} }); return () => h('div') } }))
  editor.mode.value = 'stock'
  editor.settings.value = { modes: [{ mode: 'stock', enabled: true, warehouse_code: 'XI-QIN' }] }
  const family = { parent_product_id: 10, name: '曜石 2.0', specs: [{ product_id: 11, bom_spec_id: 12, bom_variant_id: 13, spec_name: '454g', inventory_unit: '袋' }] }
  editor.catalog.value = { product_families: [family] }
  editor.selectFamily(editor.lines.value[0], family)
  editor.lines.value[0].qty = 14
  editor.preview.value = { needs_topup: true, shortages: [{ product_id: 11, bom_spec_id: 12, bom_variant_id: 13, qty: 14, available_qty: 12 }] }
  editor.startTopup()
  assert.equal(editor.topup.value.warehouse_code, 'XI-QIN')
  assert.equal(editor.topup.value.items[0].qty, 2)
  assert.equal(editor.lines.value[0].qty, 14)
})

test('库存代发可随时打开留厂购货，保留原代发草稿', async () => {
  let editor
  await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 78, customerLabel: '喜沁咖啡' }, { expose() {}, emit() {} }); return () => h('div') } }))
  editor.mode.value = 'stock'
  editor.settings.value = { modes: [{ mode: 'stock', enabled: true, warehouse_code: 'XI-QIN' }] }
  editor.form.receiver_name = '测试收件人'
  editor.startStockPurchase()
  assert.deepEqual(editor.topup.value, { warehouse_code: 'XI-QIN', items: [], kind: 'purchase' })
  assert.equal(editor.form.receiver_name, '测试收件人')
  assert.match(descriptor.template.content, /v-if="mode === 'stock'"[^>]*@click="startStockPurchase"/)
})

test('工作台直接选择代发模式，只能使用地址簿中已保存的收件地址', async () => {
  let editor
  await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 79, customerLabel: '测试客户', customerPortal: false, externalMode: 'stock' }, { expose() {}, emit() {} }); return () => h('div') } }))
  editor.mode.value = 'stock'
  editor.settings.value = { modes: [{ mode: 'stock', enabled: true, warehouse_code: 'TEST' }] }
  Object.assign(editor.form, { receiver_name: '旧草稿', receiver_phone: '13800000000', receiver_address: '旧地址' })
  assert.match(editor.validation(), /地址簿/)
  editor.selectRecipient({ receiver_name: '收件人', receiver_phone: '13800000000', receiver_company: '门店' }, { id: 8, province: '云南省', city: '大理市', district: '大理市', detail_address: '测试路1号' })
  assert.match(editor.recipientLine.value, /收件人.*13800000000.*门店.*云南省.*测试路1号/)
  assert.doesNotMatch(editor.validation(), /地址簿/)
  editor.recipientEdited()
  assert.match(editor.validation(), /地址簿/)
  const html = descriptor.template.content
  assert.doesNotMatch(html, /核对库存与费用/)
  assert.match(html, /:require-saved="!customerPortal"/)
  assert.match(html, /recipientLine/)
})

test('履约运营台直接展示代发场景并删除旧导入模块', () => {
  const source = fs.readFileSync(new URL('../views/CustomerFulfillmentView.vue', import.meta.url), 'utf8')
  assert.doesNotMatch(source, /导入数据|parseImport|applyLatest|fetchCustomerFulfillmentImports/)
  assert.match(source, /库存代发/)
  assert.match(source, /无库存代发/)
  assert.match(source, /:external-mode="dispatchMode"/)
})

test('提交时自动核价并使用报价凭证，连续点击只创建一次', async () => {
  let editor
  await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 81, customerPortal: false, externalMode: 'stock' }, { expose() {}, emit() {} }); return () => h('div') } }))
  editor.mode.value = 'stock'
  editor.settings.value = { modes: [{ mode: 'stock', enabled: true, warehouse_code: 'TEST' }] }
  const family = { parent_product_id: 10, name: '商品', specs: [{ product_id: 11, bom_spec_id: 12, bom_variant_id: 13, inventory_unit: '袋' }] }
  editor.catalog.value = { product_families: [family] }
  editor.selectFamily(editor.lines.value[0], family)
  editor.selectRecipient({ receiver_name: '收件人', receiver_phone: '13800000000' }, { id: 8, province: '云南省', city: '大理市', detail_address: '测试路1号' })
  const originalFetch = globalThis.fetch, requests = []
  let release
  const gate = new Promise(resolve => { release = resolve })
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url: String(url), init })
    if (String(url).endsWith('/preview')) { await gate; return new Response(JSON.stringify({ can_submit: true, price_quote_token: 'price-token', fee_quote_token: 'fee-token' })) }
    if (String(url).endsWith('/requests')) return new Response(JSON.stringify({ order_no: 'TEST-ORDER' }))
    return new Response(JSON.stringify({ product_families: [] }))
  }
  try {
    const first = editor.submit(), second = editor.submit()
    release(); await Promise.all([first, second])
    const previews = requests.filter(row => row.url.endsWith('/preview'))
    const creates = requests.filter(row => row.url.endsWith('/requests'))
    assert.equal(previews.length, 1); assert.equal(creates.length, 1)
    const body = JSON.parse(creates[0].init.body)
    assert.equal(body.price_quote_token, 'price-token'); assert.equal(body.fee_quote_token, 'fee-token')
    assert.equal(body.province, '云南省'); assert.equal(body.detail_address, '测试路1号')
  } finally { globalThis.fetch = originalFetch }
})

test('库存不足时只返回补购提示，不创建代发单', async () => {
  let editor
  await renderToString(createSSRApp({ setup() { editor = View.setup({ customerId: 82, customerPortal: false, externalMode: 'stock' }, { expose() {}, emit() {} }); return () => h('div') } }))
  editor.mode.value = 'stock'
  editor.settings.value = { modes: [{ mode: 'stock', enabled: true, warehouse_code: 'TEST' }] }
  const family = { parent_product_id: 10, name: '商品', specs: [{ product_id: 11, bom_spec_id: 12, bom_variant_id: 13, inventory_unit: '袋' }] }
  editor.catalog.value = { product_families: [family] }
  editor.selectFamily(editor.lines.value[0], family)
  editor.selectRecipient({ receiver_name: '收件人', receiver_phone: '13800000000' }, { id: 8, detail_address: '测试路1号' })
  const originalFetch = globalThis.fetch, requests = []
  globalThis.fetch = async url => { requests.push(String(url)); return new Response(JSON.stringify({ can_submit: false, needs_topup: true, purchase_gap_qty: 1 })) }
  try { await editor.submit(); assert.equal(requests.length, 1); assert.match(requests[0], /\/preview$/); assert.match(editor.error.value, /补购/); assert.equal(editor.preview.value.needs_topup, true) }
  finally { globalThis.fetch = originalFetch }
})

test('从当前录单新增地址时保留省市区和详细地址，供下一次地区运费核算', async () => {
  const pickerFilename = new URL('../components/CustomerRecipientPicker.vue', import.meta.url)
  const parsed = parse(fs.readFileSync(pickerFilename, 'utf8'), { filename: pickerFilename.pathname })
  let pickerCode = compileScript(parsed.descriptor, { id: 'recipient-picker-runtime' }).content
  pickerCode = pickerCode.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, pickerFilename).href : import.meta.resolve(path))}`)
  const { default: Picker } = await import(`data:text/javascript;base64,${Buffer.from(pickerCode).toString('base64')}`)
  let picker
  await renderToString(createSSRApp({ setup() { picker = Picker.setup({ customerId: 77, currentRecipient: { receiver_name: '测试收件人', receiver_phone: '13800000000', receiver_address: '测试路1号', province: '云南省', city: '大理市', district: '大理古城' } }, { expose() {}, emit() {} }); return () => h('div') } }))
  picker.createFromOrder()
  assert.equal(picker.form.province, '云南省')
  assert.equal(picker.form.city, '大理市')
  assert.equal(picker.form.district, '大理古城')
  assert.equal(picker.form.detail_address, '测试路1号')
  const selected = []
  let restricted
  await renderToString(createSSRApp({ setup() { restricted = Picker.setup({ customerId: 77, requireSaved: true }, { expose() {}, emit(...args) { selected.push(args) } }); return () => h('div') } }))
  restricted.use({ recipient_name: '收件人', phone: '13800000000', detail_address: '测试路1号' })
  assert.match(restricted.error.value, /保存/)
  assert.equal(selected.length, 0)
  restricted.use({ id: 8, recipient_name: '收件人', phone: '13800000000', detail_address: '测试路1号' })
  assert.equal(selected.length, 1)
})
