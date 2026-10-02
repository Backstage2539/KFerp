import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import { createRenderer, h, nextTick, reactive } from 'vue'
const filename = new URL('../components/ProductCreatorNameEditor.vue', import.meta.url)
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
function mount(t) {
 const state=reactive({parts:[],variables:[{id:'name',name:'商品名',default_value:'樱桃'}],samples:{}})
 const root=element('root')
 const app=renderer.createApp({render:()=>h(Editor,{...state,onUpdate:(parts)=>state.parts=parts,onCreateVariable:({name,index})=>{state.variables.push({id:'new',name,default_value:'新品'});state.parts[index]={type:'variable',variable_id:'new'}}})})
 app.mount(root);t.after(()=>app.unmount())
 const find=(label)=>all(root).find(n=>n.props['aria-label']===label)
 const click=async(label)=>{const el=find(label);assert.ok(el,`missing ${label}`);el.props.onClick();await nextTick()}
 const input=async(label,value)=>{find(label).props.onInput({target:{value}});await nextTick()}
 return {state,root,find,click,input}
}
test('two text actions append exactly in sequence and preview updates while typing',async(t)=>{
 const ui=mount(t)
 assert.equal(all(ui.root).filter(n=>n.type==='input').length,0)
 await ui.click('添加文本'); await ui.input('文本 1','前缀-')
 await ui.click('添加文本'); await ui.input('文本 2','尾部')
 assert.equal(ui.find('生成名称预览').text,'前缀-尾部')
 await ui.click('移除片段 1');assert.equal(ui.find('生成名称预览').text,'尾部')
})
test('variable action searches existing definitions or creates a shared variable at its position',async(t)=>{
 const ui=mount(t)
 await ui.click('添加变量'); await ui.input('搜索变量 1','商品')
 await ui.click('选择变量 商品名');assert.equal(ui.find('生成名称预览').text,'樱桃')
 await ui.click('添加文本');await ui.input('文本 2','-')
 await ui.click('添加变量');await ui.input('搜索变量 3','批次名')
 await ui.click('新建变量 批次名');assert.equal(ui.find('生成名称预览').text,'樱桃-新品')
 assert.deepEqual(ui.state.parts.map(p=>p.type),['variable','text','variable'])
 ui.state.samples.name='演示';await nextTick();assert.equal(ui.find('生成名称预览').text,'演示-新品')
 assert.equal(ui.state.variables[0].default_value,'樱桃')
})
