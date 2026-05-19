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
    'function beanListPublicationStatusLabel',
    'function beanListPublicationStatusClass',
    'function beanListPublicationTime',
    'function publicationScopeLabel',
    'function beanListPublicationOwnerLabel',
    'function beanListPublicationSourceLabel',
    'function startBeanListFromPublication',
    'applyCopiedBeanListPublicationConfig(row)',
    'withdrawBeanList(row)',
  ]) {
    assert.ok(viewSource.includes(expected), `missing version list behavior: ${expected}`)
  }
})

test('product bean-list version scope selector only offers public and all fulfillment customer lists', () => {
  const versionListStart = viewSource.indexOf('<section class="panel bean-list-version-panel">')
  const versionListEnd = viewSource.indexOf('<section class="panel">', versionListStart)
  assert.ok(versionListStart > -1 && versionListEnd > versionListStart, 'missing bean-list version panel')
  const versionListSource = viewSource.slice(versionListStart, versionListEnd)

  assert.match(versionListSource, /v-model="versionListScope"/)
  assert.match(versionListSource, /<option value="official">公共豆单<\/option>/)
  assert.match(versionListSource, /<option value="fulfillment_customers">所有履约客户豆单<\/option>/)
  assert.doesNotMatch(versionListSource, /棵凡官方豆单/)
  assert.doesNotMatch(versionListSource, /我的客户豆单/)
  assert.doesNotMatch(versionListSource, /指定客户豆单/)
  assert.doesNotMatch(versionListSource, /version-control-customer/)
  assert.match(viewSource, /const versionListCurrentPublication = computed/)
  assert.match(viewSource, /const publicationScopeRows = computed/)
  assert.match(viewSource, /const copyableBeanListPublications = computed\(\(\) => publicationScopeRows\.value\)/)
})

test('product bean-list view maps every bean-list type to its own metadata and tier fields', () => {
  for (const expected of [
    "if (listType === 'green') return 'green_bean_list'",
    "if (listType === 'green') return 'green_bean_sale_tiers'",
    'official: { commercial: [], drip: [], retail: [], green: [] }',
    'selectedProductIDsByType.value = { commercial: [], drip: [], retail: [], green: [] }',
    "if (normalized === 'drip') return '挂耳'",
  ]) {
    assert.ok(viewSource.includes(expected), `missing bean-list type mapping: ${expected}`)
  }
})
