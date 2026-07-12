import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('../views/GroupTemplatesView.vue', import.meta.url), 'utf8')

test('group template categories use delete instead of deactivate', () => {
  for (const forbidden of [
    '确认停用分类',
    '分类已停用',
    'groupTemplateCategoryForm.active',
    '<span>启用</span>',
  ]) {
    assert.doesNotMatch(source, new RegExp(forbidden.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }

  assert.match(source, /deleteGroupTemplateCategory\(primary\)/)
  assert.match(source, /deleteGroupTemplateCategory\(child\)/)
  assert.match(source, /确认删除大类/)
  assert.match(source, /确认删除小类/)
  assert.match(source, /自动归入未分类/)
  assert.match(source, /分类已删除/)
  assert.match(source, /method:\s*'DELETE'/)
})
