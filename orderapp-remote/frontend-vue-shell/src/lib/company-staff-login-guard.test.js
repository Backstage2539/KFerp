import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../views/CompanyStaffView.vue', import.meta.url), 'utf8')

test('employee maintenance loads the current actor before rendering login controls', () => {
  assert.match(source, /fetchCurrentActor/)
  assert.match(source, /fetchCurrentActor\(\)\.catch\(\(\) => null\)/)
  assert.match(source, /currentEmployeeID/)
  assert.match(source, /function isCurrentAccount\(employeeId\)/)
})

test('current employee login switch is disabled with an actionable explanation', () => {
  assert.match(source, /:disabled="isCurrentAccount\(row\.id\)"/)
  assert.match(source, /当前账号不能关闭自己的登录/)
})

test('setEnabled keeps a defensive client guard while leaving other employee switches available', () => {
  assert.match(source, /if \(isCurrentAccount\(employeeId\) && !loginEnabled\)/)
  assert.match(source, /@change="setEnabled\(row\.id, \$event\.target\.checked\)"/)
})
