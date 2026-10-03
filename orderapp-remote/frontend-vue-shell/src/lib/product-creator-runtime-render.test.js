import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
const filename = new URL('../views/ProductCreatorRunView.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'pc-runtime-render', inlineTemplate: true }).content
// The canvas is irrelevant to form rendering; keep the real form and helpers.
code = code.replace(/import \{ (Background|VueFlow) \} from '[^']+'/g, (_, name) => `const ${name} = {render: () => null}`)
code = code.replace(/import BusinessGroupClassificationPicker from '[^']+'/g, 'const BusinessGroupClassificationPicker = {render: () => null}')
code = code.replace(/import ProductCreatorNode from '[^']+'/g, 'const ProductCreatorNode = {render: () => null}')
code = code.replace(/from (['"])([^'"]+)\1/g, (match, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(path, filename).href : import.meta.resolve(path))}`)
const { default: Run } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
for (const version of [6, 7]) {
 test(`V${version} runtime renders variable-named outputs and existing-material input without errors`, async () => {
  const workflow = {version, variables:[{id:'name',name:'商品名',default_value:'樱桃'}],nodes:[
   {id:'raw',kind:'material',name:'配方原料候选',config:{data_role:'input',supply_mode:'purchase'}},
   {id:'output',kind:'product',name:'产出商品',config:{data_role:'output',name_parts:[{type:'variable',variable_id:'name'},{type:'text',value:'-成品'}]}}
  ],edges:[]}
  const modules = ['material','product'].map(kind=>({kind,name:kind,workflow_version:version,inputs:[],outputs:[],fields:[]}))
  const app = createSSRApp({render:()=>h(Run,{run:{id:1,status:'draft',workflow,inputs:{}},modules})})
  const html = await renderToString(app)
  assert.match(html,/樱桃-成品/)
  assert.match(html,/商品名/)
  if(version===7) {
   assert.match(html, /搜索物料名称、编码或规格/)
   assert.doesNotMatch(html,/默认物料行/)
  }
 })
}

test('V8 reuse disables dedicated upstream forms and their variables, retaining the selection control', async () => {
 const workflow = {version:8,variables:[{id:'upstream',name:'仅上游名称'}],nodes:[
  {id:'raw',kind:'material',name:'生豆',config:{data_role:'input',supply_mode:'purchase'}},
  {id:'roast',kind:'bom',name:'烘焙 BOM',config:{output_type:'material',route_id:1}},
  {id:'semi',kind:'material',name:'烘焙半成品',config:{data_role:'output',supply_mode:'manufacture',name_parts:[{type:'variable',variable_id:'upstream'}]}}
 ],edges:[{id:'a',source:'raw',source_handle:'material',target:'roast',target_handle:'components',kind:'data'},{id:'b',source:'roast',source_handle:'assembly',target:'semi',target_handle:'from_bom',kind:'data'}]}
 const modules=['material','bom'].map(kind=>({kind,name:kind,workflow_version:8,inputs:[],outputs:[],fields:[]}))
 const html=await renderToString(createSSRApp({render:()=>h(Run,{run:{id:8,status:'draft',workflow,inputs:{semi:{action:'reuse'}}},modules})}))
 assert.match(html,/已使用现有烘焙半成品，本步骤本次不执行/)
 assert.match(html,/pc-run-step-body[^>]* disabled/)
 assert.match(html,/搜索已有物料/)
 assert.doesNotMatch(html,/仅上游名称/)
})

test('completed run separates reused archives from created results',async()=>{
 const workflow={version:8,nodes:[],edges:[]}
 const current_objects=[{type:'material',id:140,name:'卡蒂姆5T中深烘',origin:'reused'},{type:'product',id:957,name:'坚果浓香挂耳-盒装',origin:'created'}]
 const html=await renderToString(createSSRApp({render:()=>h(Run,{run:{id:16,status:'config_committed',workflow,inputs:{},current_objects},modules:[]})}))
 assert.match(html,/aria-label="本次新建"[^]*坚果浓香挂耳-盒装[^]*aria-label="引用已有（未新建）"[^]*卡蒂姆5T中深烘/)
 assert.doesNotMatch(html,/正式业务档案已创建/)
})
