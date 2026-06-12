import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

test('IndustryFieldTemplatesView exposes configurable industry calculator preview', () => {
  const source = readFileSync(new URL('../views/IndustryFieldTemplatesView.vue', import.meta.url), 'utf8')
  for (const marker of [
    '/api/industry-calculators/preview',
    '计算预览',
    '咖啡烘焙',
    '包装盒',
    '童装',
    '需求产出',
    '计划投入',
    '预计损耗',
  ]) {
    assert.ok(source.includes(marker), `IndustryFieldTemplatesView.vue should include ${marker}`)
  }
})
