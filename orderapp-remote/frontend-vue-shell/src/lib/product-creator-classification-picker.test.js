import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import { createRenderer, h, nextTick, reactive } from 'vue'
const filename = new URL('../components/BusinessGroupClassificationPicker.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'creator-name-editor', inlineTemplate: true }).content
code = code.replace(/from (['"])([^'"]+)\1/g, (match, quote, path) => `from ${JSON.stringify(path === 'vue' ? import.meta.resolve('vue') : new URL(path, filename).href)}`)
const { default: Editor } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
function element(type, text = '') {
  return { type, text, props: {}, children: [], parent: null, scrollTop: 0, scrollIntoView() {}, querySelector() { return null } }
}
const renderer = createRenderer({
  createElement: element,
  createText: (text) => element('text', text),
  createComment: (text) => element('comment', text),
  setText: (node, text) => { node.text = text },
  setElementText: (node, text) => { node.text = text; node.children = [] },
  patchProp: (node, key, previous, next) => { node.props[key] = next },
  parentNode: (node) => node.parent,
  nextSibling: (node) => node.parent?.children[node.parent.children.indexOf(node) + 1],
  insert(node, parent, anchor = null) {
    if (node.parent) node.parent.children.splice(node.parent.children.indexOf(node), 1)
    node.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    if (index < 0) parent.children.push(node)
    else parent.children.splice(index, 0, node)
  },
  remove(node) { node.parent?.children.splice(node.parent.children.indexOf(node), 1); node.parent = null },
})

function all(node) { return [node,...node.children.flatMap(all)] }
test('category picker expands, collapses, searches and selects roots or children without changing identity',async(t)=>{
 const root=element('root'), selected=[]
 const groups=[{id:30,name:'咖啡豆',active:true,items:[{id:40,name:'烘焙',children:[{id:41,name:'中深烘'}]}]}]
 const app=renderer.createApp({render:()=>h(Editor,{groups,selectedTemplateIDs:[30],usageKey:'material_catalog',value:{classification_group_id:30,classification_item_id:0},onSelect:row=>selected.push(row)})})
 app.mount(root);t.after(()=>app.unmount())
 const find=label=>all(root).find(n=>n.props['aria-label']===label)
 const click=async label=>{assert.ok(find(label),label);find(label).props.onClick();await nextTick()}
 assert.equal(find('选择 咖啡豆').props['aria-pressed'],true)
 await click('全部收缩分类');assert.equal(find('选择 咖啡豆 / 烘焙'),undefined)
 await click('展开咖啡豆');assert.ok(find('选择 咖啡豆 / 烘焙'));assert.equal(find('选择 咖啡豆 / 烘焙 / 中深烘'),undefined)
 await click('全部展开分类');assert.ok(find('选择 咖啡豆 / 烘焙 / 中深烘'))
 await click('全部收缩分类');find('搜索分类').props.onInput({target:{value:'中深烘'}});await nextTick()
 assert.ok(find('选择 咖啡豆'));assert.ok(find('选择 咖啡豆 / 烘焙 / 中深烘'))
 await click('选择 咖啡豆 / 烘焙 / 中深烘');assert.equal(selected[0].group_item_id,41)
 await click('选择 咖啡豆');assert.equal(selected[1].group_item_id,0)
 assert.equal(selected[1].group_id,30)
})
