import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(here, '../views/CostingView.vue'), 'utf8')

test('product bean-list view exposes publication versions without pricing trial workspace', () => {
  const versionListIndex = viewSource.indexOf('豆单版本列表')

  assert.ok(versionListIndex > -1, 'missing visible bean-list version list section')
  assert.equal(viewSource.indexOf('价格试算'), -1, '产品豆单 should not expose the pricing trial workspace')
  assert.equal(viewSource.indexOf('pricingCollapsed'), -1, 'pricing trial collapse state should be removed from 产品豆单')

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
