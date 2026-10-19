import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { useCustomerOrderDraftStore } from './customerOrderDraft'

describe('customer order draft handoff', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('isolates stock, on-demand and topup drafts without overwriting ordinary purchases', () => {
    const store = useCustomerOrderDraftStore()
    const draft = { order_date: '2026-10-19', recipient: null, note: 'original', lines: [], idempotency_key: 'original' }
    store.saveDraft(9, 'direct_ship', { ...draft, fulfillment_mode: 'stock' })
    store.saveDraft(9, 'product_order', draft)
    store.saveDraft(9, 'direct_ship', { ...draft, fulfillment_mode: 'on_demand', note: 'demand' }, 'on_demand')
    store.saveDraft(9, 'stock_purchase', { ...draft, return_to_direct_ship: true, note: 'topup' })
    expect(store.restoreDraft(9, 'direct_ship', 'stock')?.note).toBe('original')
    expect(store.restoreDraft(9, 'direct_ship', 'on_demand')?.note).toBe('demand')
    expect(store.restoreDraft(9, 'product_order')?.note).toBe('original')
    expect(store.restoreDraft(9, 'stock_purchase')?.note).toBe('topup')
    expect(store.restoreDraft(10, 'stock_purchase')).toBeNull()
  })

  it('keeps one customer and order mode isolated while a recipient page is open', () => {
    const store = useCustomerOrderDraftStore()
    store.saveDraft(9, 'direct_ship', {
      order_date: '2026-09-15',
      recipient: { recipient_name: '张三', phone: '13800138000', detail_address: '咖啡路 88 号' },
      note: '下午发货',
      lines: [{ key: 'line-1', product_family_key: '0:91:0', product_id: 91, product_name: '小菠萝', spec_g: 0, spec_label: '227g 袋装', qty: 2 }],
      idempotency_key: 'mini-ds-1',
    })
    store.stageRecipient(9, 'direct_ship', { recipient_name: '李四', phone: '13900139000', detail_address: '茶城 1 号' })

    expect(store.restoreDraft(9, 'direct_ship')?.lines[0].spec_label).toBe('227g 袋装')
    expect(store.consumeRecipient(9, 'product_order')).toBeNull()
    expect(store.consumeRecipient(9, 'direct_ship')?.recipient_name).toBe('李四')
    expect(store.consumeRecipient(9, 'direct_ship')).toBeNull()
  })
})
