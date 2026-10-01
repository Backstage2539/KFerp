import test from 'node:test'
import assert from 'node:assert/strict'
import { productCreatorResultRows } from './product-creator-results.js'

test('object aliases merge by identity, never by name, with current and creation names', () => {
 const run={workflow:{nodes:[{id:'input',name:'投入原料'},{id:'output',name:'半成品'},{id:'bom',name:'烘焙BOM'}]},business_results:{objects:{output:[{type:'material',id:167,name:'半成品',code:'M167'}],bom:[{type:'material',id:167,name:'半成品',bom_id:9},{type:'bom',id:9,name:'半成品'}],input:[{type:'material',id:166,name:'误填半成品'},{type:'material',id:168,name:'半成品'}]}}}
 const rows=productCreatorResultRows(run)
 assert.equal(rows.length,4)
 assert.deepEqual(rows.find(r=>r.type==='material'&&r.id===167).source_node_ids,['output','bom'])
 assert.equal(rows.find(r=>r.id===167).code,'M167')
 run.current_objects=[{type:'material',id:166,name:'生豆',creation_name:'误填半成品',source_node_ids:['input'],bom_ids:[]}]
 const current=productCreatorResultRows(run)
 assert.equal(current[0].name,'生豆')
 assert.equal(current[0].creation_name,'误填半成品')
})
