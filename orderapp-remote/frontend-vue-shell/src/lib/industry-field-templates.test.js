import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

const source = fs.readFileSync(new URL('../views/IndustryFieldTemplatesView.vue', import.meta.url), 'utf8')
const template = source.split('<script setup>')[0] || source

test('industry field template page removes the standalone calculation preview', () => {
  for (const removed of [
    'calculator-panel',
    '计算预览',
    '业务预设',
    '需求产出(g)',
    '原料单价(元/kg)',
    '工序分钟',
    '工时费(元/小时)',
    'calculatorDraft',
    'calculatorPreview',
    'runCalculatorPreview',
    '/api/industry-calculators/preview',
  ]) {
    assert.doesNotMatch(source, new RegExp(removed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }

  assert.match(template, /字段定义/)
  assert.match(template, /新增字段/)
  assert.match(template, /保存模板/)
  assert.match(source, /apiSend\('\/api\/industry-field-templates'/)
})
