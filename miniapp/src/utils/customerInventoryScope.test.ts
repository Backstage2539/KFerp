import { describe, expect, it } from 'vitest'
import { customerInventoryTypeOptions } from './customerInventory'
import { customerOrderCatalogError, customerPriceTableEmptyMessage } from './customerOrderEntry'

describe('customer inventory scope and price table loading', () => {
  it('shows only authorized categories even when the authorized finished warehouse has no stock', () => {
    expect(customerInventoryTypeOptions(['finished_product'], [])).toEqual([{ key: '', label: '全部' }, { key: 'finished_product', label: '成品' }])
    expect(customerInventoryTypeOptions([], [{ inventory_type: 'green_bean' }])).toEqual([{ key: '', label: '全部' }])
  })

  it('does not invent category authorization when older responses omit scope metadata', () => {
    expect(customerInventoryTypeOptions(undefined, [])).toEqual([{ key: '', label: '全部' }])
    expect(customerInventoryTypeOptions(undefined, [{ inventory_type: 'packaging' }])).toEqual([{ key: '', label: '全部' }, { key: 'packaging', label: '包材' }])
    expect(customerInventoryTypeOptions(['packaging', 'green_bean', 'packaging'], []).map(row => row.key)).toEqual(['', 'green_bean', 'packaging'])
  })

  it('does not label pending or failed price-table loading as an unbound policy', () => {
    expect(customerPriceTableEmptyMessage({ loaded: false, loading: true, failed: false })).toBe('价格表加载中…')
    expect(customerPriceTableEmptyMessage({ loaded: false, loading: false, failed: true })).toBe('价格表暂时无法加载，请重试')
    expect(customerPriceTableEmptyMessage({ loaded: false, loading: false, failed: true, error: 'capability not enabled' })).toBe('当前下单服务尚未开通，无法加载价格表')
    expect(customerOrderCatalogError(new Error('capability not enabled'), true)).toBe('当前客户尚未开通商品下单，请联系工厂开通')
    expect(customerOrderCatalogError(new Error('会话已失效'), true)).toBe('会话已失效')
    expect(customerPriceTableEmptyMessage({ loaded: true, loading: false, failed: false })).toBe('ERP 尚未绑定可用价格表')
    expect(customerPriceTableEmptyMessage({ loaded: true, loading: false, failed: false, stock: true })).toBe('库存代发使用已购库存，不重复收取商品货款')
  })
})
