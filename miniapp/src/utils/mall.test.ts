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

  it('registers the mall page and keeps customers on the home main tab', () => {
    const pages = fs.readFileSync(path.join(currentDir, '..', 'pages.json'), 'utf8')
    const home = fs.readFileSync(path.join(currentDir, '..', 'pages', 'home', 'home.vue'), 'utf8')
    expect(pages).toContain('"path": "pages/mall/mall"')
    expect(home).toContain('MainTabBar')
    expect(home).toContain('current="home"')
    expect(home).not.toContain("session.entryMode === 'mall'")
    expect(home).not.toContain("uni.redirectTo({ url: '/pages/mall/mall' })")
  })

  it('keeps order history reachable from the bottom order entry', () => {
    const tabBar = fs.readFileSync(path.join(currentDir, '..', 'components', 'MainTabBar.vue'), 'utf8')
    expect(tabBar).toContain('/pages/service/service?key=orders')
    expect(tabBar).toContain('订单')
    expect(tabBar).toContain('uni.reLaunch')
  })

  it('preserves mall entry mode when mall customers open service pages', () => {
    const servicePage = fs.readFileSync(path.join(currentDir, '..', 'pages', 'service', 'service.vue'), 'utf8')
    const api = fs.readFileSync(path.join(currentDir, '..', 'api', 'customerPortal.ts'), 'utf8')
    expect(api).toContain('miniapp_entry_mode?: MiniappEntryMode | string')
    expect(servicePage).toContain('miniapp_entry_mode: page.value.miniapp_entry_mode || session.entryMode')
  })

  it('validates direct ship batch row count before submitting', () => {
    const servicePage = fs.readFileSync(path.join(currentDir, '..', 'pages', 'service', 'service.vue'), 'utf8')
    expect(servicePage).toContain('const totalRows = Number(directShipForm.value.total_rows) || 0')
    expect(servicePage).toContain('if (totalRows <= 0)')
    expect(servicePage).toContain('订单行数必须大于 0')
  })

  it('uses customer-facing pickers instead of raw system ID fields for service order forms', () => {
    const servicePage = fs.readFileSync(path.join(currentDir, '..', 'pages', 'service', 'service.vue'), 'utf8')
    expect(servicePage).toContain('processingInputOptions')
    expect(servicePage).toContain('processingTargetProductOptions')
    expect(servicePage).toContain('fulfillmentProductOptions')
    expect(servicePage).toContain('setProcessingInputMaterial')
    expect(servicePage).toContain('setProcessingTargetProduct')
    expect(servicePage).toContain('setFulfillmentProduct')
    expect(servicePage).toContain('<picker mode="selector" :range="processingInputLabels"')
    expect(servicePage).toContain('<picker mode="selector" :range="fulfillmentProductLabels"')
    expect(servicePage).not.toContain('placeholder="生豆物料ID"')
    expect(servicePage).not.toContain('placeholder="目标产品ID"')
    expect(servicePage).not.toContain('placeholder="产品ID"')
  })

  it('keeps fulfillment order prices server-authoritative', () => {
    const servicePage = fs.readFileSync(path.join(currentDir, '..', 'pages', 'service', 'service.vue'), 'utf8')
    const api = fs.readFileSync(path.join(currentDir, '..', 'api', 'customerPortal.ts'), 'utf8')
    expect(servicePage).not.toContain('placeholder="单价，可不填"')
    expect(servicePage).not.toContain('unit_price: Number(fulfillmentForm.value.unit_price)')
    expect(api).not.toContain('unit_price?: number')
  })
})
