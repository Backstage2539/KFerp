import test from 'node:test'
import assert from 'node:assert/strict'
import { groupRemainderSuggestion, validateGroupRemainders } from './packaging-group-remainder.js'
const items=[{job_card_id:1,packaging_available_g:10000,packaging_standard_g_per_piece:454},{job_card_id:2,packaging_available_g:5000,packaging_standard_g_per_piece:454}]
const allocations=[{job_card_id:1,qty:10},{job_card_id:2,qty:5}]
test('suggests each task remaining grams separately',()=>{assert.equal(groupRemainderSuggestion(items[0],10),5460);assert.equal(groupRemainderSuggestion(items[1],5),2730)})
test('rounds used weight before calculating remainder',()=>{assert.equal(groupRemainderSuggestion({packaging_available_g:10,packaging_standard_g_per_piece:1.5},1),8)})
test('requires individual weighed confirmation and reason for discrepancy',()=>{assert.match(validateGroupRemainders(items,allocations,{1:5460,2:2730},{1:true},''),/称重/);assert.match(validateGroupRemainders(items,allocations,{1:5000,2:2730},{1:true,2:true},''),/原因/);assert.equal(validateGroupRemainders(items,allocations,{1:5000,2:2730},{1:true,2:true},'装袋差异'),'');assert.match(validateGroupRemainders(items,allocations,{1:-1,2:2730},{1:true,2:true},'差异'),/余料/)})
