import test from 'node:test'
import assert from 'node:assert/strict'
import { clearRecordedOutputReceiptAttempt, readRecordedOutputReceiptAttempt, recordedOutputReceiptPayload, saveRecordedOutputReceiptAttempt } from './recorded-output-receipt.js'

test('direct recorded-output receipt sends only the version and stable request id',()=>{
  assert.deepEqual(recordedOutputReceiptPayload('version-1','request-1'),{
    expected_version:'version-1',
    request_id:'request-1',
  })
})

test('direct receipt requires a fresh version and request id',()=>{
  assert.throws(()=>recordedOutputReceiptPayload('','request-1'),/刷新/)
  assert.throws(()=>recordedOutputReceiptPayload('version-1',''),/请求标识/)
})

test('direct receipt keeps the same request across page reloads after a lost response',()=>{
  const values=new Map()
  const storage={getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value),removeItem:key=>values.delete(key)}
  const first=recordedOutputReceiptPayload('version-1','request-1')
  saveRecordedOutputReceiptAttempt(storage,116,first)
  assert.deepEqual(readRecordedOutputReceiptAttempt(storage,116),first)
  clearRecordedOutputReceiptAttempt(storage,116)
  assert.equal(readRecordedOutputReceiptAttempt(storage,116),null)
})
