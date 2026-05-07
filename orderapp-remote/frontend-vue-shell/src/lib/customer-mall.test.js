import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import {
  createBlankMallProduct,
  formatMallPrice,
  mallStatusOptions,
  mallTemplateOptions,
  normalizeMallProduct,
  normalizeMallProductStatus,
  normalizeMallTemplateKey,
} from './customer-mall.js'

const currentDir = path.dirname(fileURLToPath(import.meta.url))

test('mall product form normalizes template, status, sort, spec and price', () => {
  assert.equal(normalizeMallTemplateKey('compact'), 'compact')
  assert.equal(normalizeMallTemplateKey('unknown'), 'hero')
  assert.equal(normalizeMallProductStatus('published'), 'published')
  assert.equal(normalizeMallProductStatus('offline'), 'draft')

  const row = normalizeMallProduct({
    id: 8,
    product_id: 3,
    title: '',
    spec_g: 0,
    unit_price: '68.6',
    template_key: 'wide',
    status: 'published',
    sort_order: 0,
  }, [{ id: 3, name: '乌拉嘎', default_price: 88 }])
  assert.equal(row.title, '乌拉嘎')
  assert.equal(row.spec_g, 454)
  assert.equal(row.unit_price, 68.6)
  assert.equal(row.template_key, 'wide')
  assert.equal(row.status, 'published')
  assert.equal(row.sort_order, 100)
})

test('blank mall product starts from the first selectable ERP product', () => {
  const row = createBlankMallProduct([{ id: 7, name: '瑰夏', default_price: 128 }])
  assert.equal(row.product_id, 7)
  assert.equal(row.title, '瑰夏')
  assert.equal(row.unit_price, 128)
  assert.equal(row.spec_g, 454)
  assert.equal(row.template_key, 'hero')
  assert.equal(row.status, 'draft')
})

test('mall admin page is wired to list, save, image upload and live preview', () => {
  assert.deepEqual(mallTemplateOptions.map((item) => item.key), ['hero', 'compact', 'wide'])
  assert.deepEqual(mallStatusOptions.map((item) => item.key), ['draft', 'published'])
  assert.equal(formatMallPrice(68), '¥68.00')

  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'MallSettingsView.vue'), 'utf8')
  assert.match(source, /apiGet\('\/api\/customer-portal\/admin\/mall-products'\)/)
  assert.match(source, /apiSend\('\/api\/customer-portal\/admin\/mall-products'/)
  assert.match(source, /`\/api\/customer-portal\/admin\/mall-products\/\$\{current\.value\.id\}\/image`/)
  assert.match(source, /商城预览/)
  assert.match(source, /v-if="preview\.image_url"/)
})
