import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createRenderer, createSSRApp, h, nextTick, reactive, ref } from 'vue'
import { renderToString } from '@vue/server-renderer'

async function component(name, render = false) {
  const file = new URL(`../components/${name}.vue`, import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'), { filename: file.pathname })
  const script = compileScript(descriptor, { id: `weight-${name}` })
  let code = script.content
  if (render) {
    code = code.replace('export default', 'const component =')
    const template = compileTemplate({ id: `weight-${name}`, source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
    assert.deepEqual(template.errors, [])
    code += '\n' + template.code.replace('export function render', 'function render') + '\ncomponent.render = render; export default component;'
  }
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  return (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
}

test('logistics editor preserves grams when switching unit and clears invalid values before publication', async () => {
  const Editor = await component('LogisticsWeightEditor')
  const props = reactive({ modelValue: 100, inventoryUnit: '盒', disabled: false })
  let editor
  await renderToString(createSSRApp({ setup() {
    editor = Editor.setup(props, { expose() {}, emit(event, value) { if (event === 'update:modelValue') props.modelValue = value } })
    return () => h('div')
  } }))
  assert.equal(editor.quantity.value, 0.1)
  editor.changeUnit('g')
  assert.equal(editor.quantity.value, 100)
  editor.updateQuantity('227.125')
  assert.equal(props.modelValue, 227.125)
  editor.updateQuantity('0')
  assert.equal(props.modelValue, null)
  await nextTick()
  assert.equal(editor.quantity.value, '0')
  assert.match(editor.error.value, /物流重量/)
  editor.updateQuantity('')
  assert.equal(props.modelValue, null)
  assert.equal(editor.error.value, '')
  editor.changeUnit('kg')
  editor.updateQuantity('0.1')
  assert.equal(props.modelValue, 100)
})

test('saved weight component renders decimal snapshots, unknown calculations and historical absence', async () => {
  const Weights = await component('OrderItemWeights', true)
  const html = await renderToString(createSSRApp(Weights, { item: { material_unit_weight_g: '90.125', material_item_weight_g: '180.25', logistics_unit_weight_g: '100', logistics_item_weight_g: '200' } }))
  assert.match(html, /用料重量（参考）/)
  assert.match(html, /单件 90.125g · 本项 180.25g/)
  assert.match(html, /物流重量/)
  assert.match(html, /单件 100g · 本项 200g/)
  const unknown = await renderToString(createSSRApp(Weights, { item: { weight_snapshot_json: '{"version":1}' } }))
  assert.match(unknown, /未计算/)
  assert.doesNotMatch(unknown, /0g/)
  const historical = await renderToString(createSSRApp(Weights, { item: {} }))
  assert.match(historical, /未记录/)
})

test('switching BOM specifications discards another specifications invalid weight draft', async () => {
  const bom = readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const tag = bom.match(/<LogisticsWeightEditor[^>]*v-model="selectedBomVariant\.logistics_weight_g"[^>]*\/>/)?.[0] || ''
  const keyExpression = tag.match(/:key="([^"]+)"/)?.[1] || 'undefined'
  const keyFor = new Function('selectedBomVariant', `return ${keyExpression}`)
  for (const nextWeight of [100, null]) {
    const Editor = { ...await component('LogisticsWeightEditor') }
    const setup = Editor.setup
    let activeEditor
    Editor.setup = (props, context) => { activeEditor = setup(props, context); return () => h('div') }
    const selected = ref({ bom_variant_id: 101, logistics_weight_g: 250 })
    const renderer = createRenderer({
      createElement: () => ({ children: [] }), createText: text => ({ text }), createComment: text => ({ text }),
      setElementText: (node, text) => { node.text = text }, setText: (node, text) => { node.text = text },
      patchProp() {}, insert(node, parent) { node.parent = parent; parent.children.push(node) },
      remove(node) { node.parent.children = node.parent.children.filter(child => child !== node) },
      parentNode: node => node.parent, nextSibling: () => null,
    })
    const app = renderer.createApp({ render: () => h(Editor, {
      key: keyFor(selected.value), modelValue: selected.value.logistics_weight_g, inventoryUnit: '盒', disabled: false,
      'onUpdate:modelValue': value => { selected.value.logistics_weight_g = value },
    }) })
    app.mount({ children: [] })
    activeEditor.updateQuantity('0')
    await nextTick()
    assert.match(activeEditor.error.value, /物流重量/)
    selected.value = { bom_variant_id: 102, logistics_weight_g: nextWeight }
    await nextTick()
    assert.equal(activeEditor.error.value, '', `next specification ${nextWeight} clears the previous error`)
    assert.equal(activeEditor.quantity.value, nextWeight == null ? '' : 0.1)
    app.unmount()
  }
})
