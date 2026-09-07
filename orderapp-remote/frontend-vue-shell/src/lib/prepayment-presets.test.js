import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {prepaymentPresets, prepaymentByRate} from './prepayment-presets.js'
test('preset deposits use discounted goods amount, with cent rounding',()=>{
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
