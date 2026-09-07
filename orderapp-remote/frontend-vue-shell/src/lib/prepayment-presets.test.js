import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {prepaymentPresets, prepaymentByRate} from './prepayment-presets.js'
test('preset deposits use receivable amount, with cent rounding',()=>{
 assert.deepEqual(prepaymentPresets,[30,50,70])
 assert.equal(prepaymentByRate(616,30),'184.80')
 assert.equal(prepaymentByRate(99.99,50),'50.00')
 assert.equal(prepaymentByRate(-1,30),'0.00')
 assert.equal(prepaymentByRate(100,101),'0.00')
})
test('web entry always exposes deposit input and clickable presets',()=>{
 const source=readFileSync(new URL('../views/OrderEntryView.vue',import.meta.url),'utf8')
 assert.ok(source.includes('data-prepayment-editor'))
 assert.ok(source.includes('applyPrepaymentPreset'))
 assert.ok(!source.includes(`<label v-if="selectedPayStatusName.includes('预付款')`))
})

test('web preset uses the order total including freight and follows freight changes',()=>{
 const source=readFileSync(new URL('../views/OrderEntryView.vue',import.meta.url),'utf8')
 const body=source.match(/function applyPrepaymentPreset\(rate\) \{([\s\S]*?)\n\}/)[1]
 const form={prepayment_amount:'0.00'}
 const total={value:{goodsAmount:80,grandTotal:100}}
 const apply=new Function('rate','prepaymentRate','form','prepaymentByRate','orderTotalPreviewValue','selectPrepaymentStatus',body)
 apply(50,{value:0},form,prepaymentByRate,total,()=>{})
 assert.equal(form.prepayment_amount,'50.00')
 total.value.grandTotal=120
 apply(50,{value:50},form,prepaymentByRate,total,()=>{})
 assert.equal(form.prepayment_amount,'60.00')
 assert.match(source,/watch\(\(\) => orderTotalPreviewValue.value.grandTotal,/)
 assert.ok(!source.includes('不含运费'))
})
