import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function source(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('customer inventory miniapp pages', () => {
  it('keeps the inventory list searchable, paginated and multi-selectable across filters', () => {
    const panel = source('src/components/CustomerInventoryPanel.vue')

    expect(panel).toContain('q: query.value')
    expect(panel).toContain('page: page.value')
    expect(panel).toContain('limit: pageSize.value')
    expect(panel).toContain('搜索商品名称')
    expect(panel).toContain('@tap.stop="toggleSelection(item)"')
    expect(panel).toContain("`生成生产工单（${selectedItems.length}）`")
    expect(panel).toContain('共 {{ totalRows }} 条')
    expect(panel).toContain('共 {{ totalPages }} 页')
    expect(panel).toContain('上一页')
    expect(panel).toContain('下一页')
    expect(panel).toContain('跳转')
    expect(panel).toContain('每页')
    expect(panel).toContain('processingPrefill.stage')
    expect(panel).toContain('selectedByKey')
    expect(panel).toContain('let loadVersion = 0')
    expect(panel).toContain('if (version !== loadVersion) return')
    expect(panel).toContain('if (navigating.value) return')
    expect(panel).toContain(':value="Math.max(0, pageSizeOptions.indexOf(pageSize))"')
    expect(panel).toContain('customerInventoryDetailPath')
    expect(panel).not.toContain('fetchCustomerInventoryBatches')
    expect(panel).not.toContain('v-if="selected"')
  })

  it('registers a separate inventory detail page with native stack navigation', () => {
    const pages = source('src/pages.json')
    const detail = source('src/pages/customer-inventory-detail/customer-inventory-detail.vue')

    expect(pages).toContain('pages/customer-inventory-detail/customer-inventory-detail')
    expect(detail).toContain('EnvironmentBadge')
    expect(detail).toContain('fetchCustomerInventory')
    expect(detail).toContain('fetchCustomerInventoryBatches')
    expect(detail).toContain('历史库存，暂无生产日期')
    expect(detail).toContain('添加生产工单')
    expect(detail).toContain('processingPrefill.stage')
    expect(detail).toContain('if (navigating.value) return')
    expect(detail).toContain('let loadVersion = 0')
    expect(detail).toContain('version !== loadVersion')
    expect(detail).toContain('customerID !== session.currentCustomerID')
    expect(detail).toContain('当前库存已变化，请返回库存列表刷新')
    expect(detail).toContain('batches.value = []')
    expect(detail).toContain('v-if="summary"')
    expect(detail).toContain("uni.navigateTo({")
    expect(detail).not.toContain('uni.redirectTo')
    expect(detail).not.toContain('uni.reLaunch')
  })

  it('consumes multi-SKU prefills once and preserves the legacy single-SKU query', () => {
    const service = source('src/pages/service/service.vue')
    const processing = source('src/components/CustomerProcessingPanel.vue')

    expect(service).toContain('processingPrefill.consume')
    expect(service).toContain("query?.product_id")
    expect(service).toContain("query?.spec_g")
    expect(service).toContain(':prefill-items="processingPrefillItems"')
    expect(service).toContain('@prefill-consumed="clearProcessingPrefill"')
    expect(processing).toContain('resolveProcessingPrefillLines')
    expect(processing).toContain('prefillApplied')
    expect(processing).toContain("emit('prefillConsumed')")
    expect(processing).toContain('未配置有效 BOM')
  })
})
