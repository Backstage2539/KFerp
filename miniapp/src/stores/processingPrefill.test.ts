import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { useProcessingPrefillStore } from './processingPrefill'

describe('processing prefill handoff store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('stages deduplicated items in memory and consumes them exactly once', () => {
    const store = useProcessingPrefillStore()
    store.stage(19, [
      { product_id: 551, spec_g: 227, product_name: '乌拉嘎 227g' },
      { product_id: 551, spec_g: 227, product_name: '乌拉嘎 227g' },
      { product_id: 552, spec_g: 454, product_name: '乌拉嘎 454g' },
    ])

    expect(store.consume(19)).toHaveLength(2)
    expect(store.consume(19)).toEqual([])
    expect(store.items).toEqual([])
  })

  it('does not leak staged inventory selections to another customer', () => {
    const store = useProcessingPrefillStore()
    store.stage(19, [{ product_id: 551, spec_g: 227, product_name: '乌拉嘎' }])

    expect(store.consume(20)).toEqual([])
    expect(store.items).toEqual([])
  })

  it('fails closed when either staged or consuming customer is unknown', () => {
    const store = useProcessingPrefillStore()
    store.stage(0, [{ product_id: 551, spec_g: 227, product_name: '乌拉嘎' }])
    expect(store.items).toEqual([])

    store.stage(19, [{ product_id: 551, spec_g: 227, product_name: '乌拉嘎' }])
    expect(store.consume(0)).toEqual([])
    expect(store.items).toEqual([])
  })

  it('keeps cutover BOM specification identity intact when staging inventory selections', () => {
    const store = useProcessingPrefillStore()
    store.stage(19, [{
      product_id: 550,
      bom_spec_id: 91,
      bom_variant_id: 191,
      spec_g: 0,
      spec_name: '227g袋',
      inventory_unit: '袋',
      product_name: '乌拉嘎',
    }])

    expect(store.consume(19)).toEqual([{
      product_id: 550,
      bom_spec_id: 91,
      bom_variant_id: 191,
      spec_g: 0,
      spec_name: '227g袋',
      inventory_unit: '袋',
      product_name: '乌拉嘎',
      sku_code: undefined,
    }])
  })
})
