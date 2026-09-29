import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const designer = readFileSync(new URL('../views/ProductCreatorView.vue', import.meta.url), 'utf8')
const runtime = readFileSync(new URL('../views/ProductCreatorRunView.vue', import.meta.url), 'utf8')

test('V3 hides industry material and product categories while keeping V2 legacy editors', () => {
  assert.match(designer, /v-if="templateWorkflowVersion < 3" class="pc-field-label">物料类别与取得方式/)
  assert.match(designer, /v-if="templateWorkflowVersion < 3" class="pc-control"[^>]*selectedNode\.data\.config\.kind/)
  assert.match(runtime, /v-if="workflowVersion < 3" class="pc-row-field"><span>物料类别<\/span>/)
  assert.match(runtime, /v-if="workflowVersion < 3"><span>商品类型<\/span>/)
  assert.match(runtime, /v-if="workflowVersion < 3"><span>物料类别<\/span>/)
})
