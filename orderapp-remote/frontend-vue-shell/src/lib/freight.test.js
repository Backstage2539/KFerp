import { test } from 'node:test'
import assert from 'node:assert/strict'
import { freightTemplateForm, freightTemplatePayload, searchFreightRegions, remainingCoverage, freightLabel } from './freight.js'
test('groups preserve tiers and reject duplicate regions and blank money',()=>{
 const f=freightTemplateForm();f.name='test';f.carrier_id=1;f.logistics_product_id=2;f.groups[0].name='一区';f.groups[0].region_codes=['530000'];f.groups[0].tiers.forEach((t,i)=>t.fee=String(i+1));f.groups[0].additional_fee='2';
 const p=freightTemplatePayload(f);assert.equal(p.groups[0].tiers[4].fee_cents,500);assert.deepEqual(p.rules,[]);f.groups.push(structuredClone(f.groups[0]));assert.throws(()=>freightTemplatePayload(f),/地区重复/)
})
test('searches names alias pinyin initials code and reports coverage',()=>{
 const rows=[{code:'530000',name:'云南省',alias:'云南',pinyin:'yunnansheng',initials:'yns',level:1},{code:'540000',name:'西藏自治区',level:1}];for(const q of ['云南','yunnan','yn','5300'])assert.equal(searchFreightRegions(rows,q)[0].code,'530000');assert.deepEqual(remainingCoverage(rows,[{region_codes:['530000']}]).map(x=>x.code),['540000']);assert.equal(freightLabel({status:'pending_actual',estimated_total_cents:2000}),'预估运费 ¥20.00 · 待出账');assert.equal(freightLabel({status:'free',estimate_known:false}),'包邮 · 快递成本待核')
})
test('free-shipping cost stops being pending after an actual bill, including zero actual cost',()=>{for(const actual_cost_cents of [0,1200])assert.equal(freightLabel({status:'free',estimate_known:false,actual_cost_cents}),'包邮')})
