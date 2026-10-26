import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source=readFileSync(new URL('../views/WorkOrdersView.vue',import.meta.url),'utf8')

test('work order list posts recorded output directly without a quantity or material form',()=>{
  assert.match(source,/async function receiveRecordedOutput\(row\)/)
  assert.match(source,/\/api\/production\/legacy-review\/preview/)
  assert.match(source,/\/api\/produce\/work-orders\/\$\{id\}\/receive-completed-output/)
  assert.doesNotMatch(source,/WorkOrderReceiptForm|实际完成数量|本单用料/)
  assert.match(source,/正在入库/)
})
