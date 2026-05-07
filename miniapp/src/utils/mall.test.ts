import { describe, expect, it } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  addMallCartItem,
  buildMallOrderPayload,
  mallCartTotal,
  normalizeMallProduct,
  updateMallCartQty,
} from './mall'

const currentDir = path.dirname(fileURLToPath(import.meta.url))

describe('mini mall helpers', () => {
  it('normalizes product cards for display and cart calculations', () => {
    const product = normalizeMallProduct({
      id: 11,
      product_id: 7,
      title: '',
      subtitle: '柑橘莓果',
      spec_g: 0,
      unit_price: '68.5',
      template_key: 'wide',
      status: 'published',
    })

    expect(product.id).toBe(11)
    expect(product.title).toBe('商品')
    expect(product.spec_g).toBe(454)
    expect(product.unit_price).toBe(68.5)
    expect(product.template_key).toBe('wide')
  })

  it('merges cart lines and keeps only positive quantities', () => {
    const product = normalizeMallProduct({ id: 11, title: '乌拉嘎', unit_price: 68 })
    const cart = addMallCartItem(addMallCartItem([], product, 1), product, 2)
    expect(cart).toEqual([{ mall_product_id: 11, title: '乌拉嘎', unit_price: 68, qty: 3 }])
    expect(updateMallCartQty(cart, 11, 0)).toEqual([])
    expect(mallCartTotal(cart)).toBe(204)
  })

  it('builds the backend order payload from recipient form and cart lines', () => {
    const payload = buildMallOrderPayload(
      { name: '张三', phone: '13800138000', address: '上海市', note: '周末前发' },
      [{ mall_product_id: 11, title: '乌拉嘎', unit_price: 68, qty: 2 }],
    )

    expect(payload).toEqual({
      recipient_name: '张三',
      recipient_phone: '13800138000',
      recipient_address: '上海市',
      note: '周末前发',
      items: [{ mall_product_id: 11, qty: 2 }],
    })
  })

  it('registers the mall page and lets mall entry-mode customers land there', () => {
    const pages = fs.readFileSync(path.join(currentDir, '..', 'pages.json'), 'utf8')
    const home = fs.readFileSync(path.join(currentDir, '..', 'pages', 'home', 'home.vue'), 'utf8')
    expect(pages).toContain('"path": "pages/mall/mall"')
    expect(home).toContain("session.entryMode === 'mall'")
    expect(home).toContain("uni.redirectTo({ url: '/pages/mall/mall' })")
  })
})
