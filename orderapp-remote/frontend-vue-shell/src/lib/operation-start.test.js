import test from 'node:test'
import assert from 'node:assert/strict'
import {startDraft,startPayload} from './operation-start.js'
const p={version:'v1',input_qty:5,input_unit:'kg',output_unit:'kg',recipe_ratio_pct:100,materials:[{key:'a',unit:'kg',qty:1.5,ratio_pct:30},{key:'b',unit:'kg',qty:3.5,ratio_pct:70}]}
test('start keeps frozen identities and requires reasons for adjusted quantities',()=>{
 const d=startDraft(p);assert.equal(d.actual_input_qty,5)
 const payload=startPayload(p,d,'r');assert.equal(payload.materials[0].key,'a');assert.equal(payload.expected_version,'v1')
 d.materials[0].qty=2;d.materials[1].qty=3
 assert.throws(()=>startPayload(p,d,'r'),/原因/)
 d.difference_reason='实称调整';assert.equal(startPayload(p,d,'r').materials[0].qty,2)
 d.materials[1].qty=4;assert.throws(()=>startPayload(p,d,'r'),/合计/)
})
test('ratio rounding conserves one gram and converts one pound to gram precision',()=>{
 const p={version:'v',input_qty:1,input_unit:'g',recipe_ratio_pct:100,materials:[{key:'a',unit:'kg',ratio_pct:50,qty:.001},{key:'b',unit:'kg',ratio_pct:50,qty:0}]}
 const d=startDraft(p);assert.equal(startPayload(p,d,'r').materials.reduce((sum,m)=>sum+Math.round(m.qty*1000),0),1)
 p.input_unit='lb';p.materials[0].qty=.227;p.materials[1].qty=.227
 assert.equal(startPayload(p,startDraft(p),'r').materials.reduce((sum,m)=>sum+Math.round(m.qty*1000),0),454)
})
test('changing the actual batch input requires a reason even with unchanged proportions',()=>{
 const p={version:'v',input_qty:10,input_unit:'kg',recipe_ratio_pct:100,materials:[{key:'a',unit:'kg',ratio_pct:100,qty:10}]},d=startDraft(p)
 d.actual_input_qty=8;d.materials[0].qty=8
 assert.throws(()=>startPayload(p,d,'r'),/原因/)
 d.difference_reason='本锅少投';assert.equal(startPayload(p,d,'r').actual_input_qty,8)
})
