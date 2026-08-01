import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

test('audit view exposes employee order drafts as a business object filter', () => {
  const source = readFileSync(new URL('../views/AuditView.vue', import.meta.url), 'utf8')

  assert.match(source, /<option value="employee_order_draft">订单草稿<\/option>/)
  assert.match(source, /操作者\/对象类型\/字段\/内容/)
})
