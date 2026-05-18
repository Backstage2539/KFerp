import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(here, '../views/CostingView.vue'), 'utf8')

test('costing view exposes bean-list publication versions before pricing trial', () => {
  const versionListIndex = viewSource.indexOf('豆单版本列表')
  const pricingIndex = viewSource.indexOf('价格试算')

  assert.ok(versionListIndex > -1, 'missing visible bean-list version list section')
  assert.ok(pricingIndex > -1, 'missing pricing trial section')
  assert.ok(versionListIndex < pricingIndex, 'bean-list version list should be shown in 产品豆单 before 价格试算')

  for (const expected of [
    'v-model="publicationScope"',
    'v-model="pdfOptions.listType"',
    'publicationScope === \'customer\'',
    'currentScopePublicationRows',
    'applyCopiedBeanListPublicationConfig(row)',
    'withdrawBeanList(row)',
  ]) {
    assert.ok(viewSource.includes(expected), `missing version list behavior: ${expected}`)
  }
})
