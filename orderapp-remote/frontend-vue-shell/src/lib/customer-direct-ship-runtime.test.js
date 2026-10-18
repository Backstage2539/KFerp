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
})
