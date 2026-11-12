import test from 'node:test'
import assert from 'node:assert/strict'
import { workstationFormCache, clearWorkstationFormCache } from './workstation-form-cache.js'

test('task input and uncertain request survive switching away and reset after success', () => {
 const first=workstationFormCache('start',123); first.draft.qty=12
 const attempt=first.retry.begin({qty:12},()=> 'same-id');first.retry.fail(new Error('timeout'))
 const next=workstationFormCache('start',123)
 assert.equal(next.draft.qty,12);assert.equal(next.retry.uncertain,true)
 assert.equal(next.retry.begin({qty:99},()=> 'other-id').request_id,attempt.request_id)
 assert.notEqual(workstationFormCache('completion',123),next)
 clearWorkstationFormCache('start',123)
 assert.equal(workstationFormCache('start',123).draft.qty,undefined)
})
