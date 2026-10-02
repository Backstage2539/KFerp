import test from 'node:test'
import assert from 'node:assert/strict'
import { moduleForNode, connectionValidation } from './product-creator-graph.js'
import * as variables from './product-creator-variables.js'
const modules = ['material', 'bom', 'product'].map(kind => ({kind, workflow_version:7, inputs:[], outputs:kind === 'bom' ? [{id:'assembly',types:['bom.output']}] : []}))
test('editing legacy output makes implicit manufacture explicit and removes input presets only', () => {
 const original={version:6,nodes:[{id:'raw',kind:'material',config:{data_role:'input',default_rows:[{row_id:'one',name:'preset'}],name_parts:[{type:'text',value:'raw'}]}},{id:'semi',kind:'material',config:{data_role:'output',name_parts:[{type:'variable',variable_id:'name'},{type:'text',value:'-半成品'}]}}],edges:[]}
 assert.equal(typeof variables.upgradeWorkflowToV7,'function')
 const draft=variables.upgradeWorkflowToV7(original)
 assert.equal(draft.version,7)
 assert.equal(draft.nodes[0].config.supply_mode,'purchase')
 assert.equal(draft.nodes[0].config.default_rows,undefined)
 assert.equal(draft.nodes[0].config.name_parts,undefined)
 assert.equal(draft.nodes[1].config.supply_mode,'manufacture')
 assert.deepEqual(draft.nodes[1].config.name_parts,original.nodes[1].config.name_parts)
 assert.equal(original.nodes[0].config.default_rows.length,1)
})
test('purchased materials have no input; manufactured materials accept exactly one generating BOM',()=>{
 const config={supply_mode:'purchase',data_role:'input'}
 assert.deepEqual(moduleForNode({kind:'material',config},modules,7).inputs,[])
 const output={supply_mode:'manufacture',data_role:'output'}
 const target={id:'semi',data:{config:output,module:moduleForNode({kind:'material',config:output},modules,7)}}
 const bom={id:'bom',data:{module:modules[1]}}
 const connection={source:'bom',sourceHandle:'assembly',target:'semi',targetHandle:'from_bom'}
 assert.equal(connectionValidation(connection,[bom,target],modules).valid,true)
 assert.equal(connectionValidation(connection,[bom,target],modules,'data',[{id:'existing',source:'another',sourceHandle:'assembly',target:'semi',targetHandle:'from_bom',data:{kind:'data'}}]).code,'multiple_material_producers')
})
