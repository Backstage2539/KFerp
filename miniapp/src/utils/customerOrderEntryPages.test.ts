import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = (path: string) => readFileSync(resolve(path), 'utf8')

describe('customer order entry pages', () => {
  it('uses compact employee-style order rows and dedicated recipient maintenance', () => {
    const panel = source('src/components/CustomerDirectShipPanel.vue')
    const addresses = source('src/pages/customer-addresses/customer-addresses.vue')
    expect(panel).toContain('订单日期')
    expect(panel).toContain('收件客户')
    expect(panel).toContain('class="spec-choice-list"')
    expect(panel).toContain('@tap="chooseInlineSpec(line, spec)"')
    expect(panel).not.toContain('ProductSpecPickerSheet')
    expect(panel).not.toContain('全部可选规格')
    expect(panel).toContain('商品估算合计')
    expect(panel).toContain('查看价格表')
    expect(panel).toContain("Number(item.bom_variant_id || 0) === Number(line.bom_variant_id || 0)")
    expect(panel).not.toContain('v-model="pastedRecipient"')
    expect(panel).not.toContain('v-model="recipientName"')
    expect(addresses).toContain('仅本单使用')
    expect(addresses).toContain('保存并使用')
  })

  it('registers a read-only price-table preview with no selection control', () => {
    const pages = source('src/pages.json')
    const preview = source('src/pages/order-price-table-preview/order-price-table-preview.vue')
    expect(pages).toContain('pages/order-price-table-preview/order-price-table-preview')
    expect(preview).toContain('fetchOrderPriceTablePreview')
    expect(preview).toContain('数量档位')
    expect(preview).toContain('priceTableSpecLabel(row)')
    expect(preview).not.toContain("row.spec_name || '默认规格'")
    expect(preview).not.toContain('<picker')
    expect(preview).not.toContain('@tap="select')
  })
})
