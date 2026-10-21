import { describe, expect, it, vi } from 'vitest'
import type { EmployeeOrderDetail } from '../api/customerPortal'
import { useEmployeeOrderStatus } from './useEmployeeOrderStatus'

const order = {
  id: 42, edit_revision: 'revision-1', pay_status_id: 1, pay_status: '未付款',
  payment_method: '', prepayment_amount: '0.00', ship_status_id: 3, grand_total: '100.00',
} as EmployeeOrderDetail
const statuses = {
  pay_statuses: [{ id: 1, name: '未付款' }, { id: 2, name: '已付款' }, { id: 4, name: '已付预付款' }, { id: 6, name: '已收款' }],
  ship_statuses: [{ id: 3, name: '待发货' }, { id: 5, name: '已发货' }],
}

function setup(allowed = true, current = order) {
  const onSaved = vi.fn()
  const request = vi.fn().mockResolvedValue({ order_id: current.id })
  const refresh = vi.fn().mockResolvedValue({ order: { ...current, edit_revision: 'revision-2' } })
  const editor = useEmployeeOrderStatus({
    order: () => current, canUpdate: () => allowed, options: () => statuses,
    request, refresh, onSaved,
  })
  return { editor, request, refresh, onSaved }
}

describe('employee order status editor', () => {
  it('allows payment updates even when whole-order editing is unavailable', () => {
    const { editor } = setup(true, { ...order, can_edit: false })
    editor.start()
    expect(editor.editing.value).toBe(true)
    expect(editor.draft.value).toMatchObject({ pay_status_id: 1, payment_method: '', prepayment_amount: '0.00', ship_status_id: 3 })
  })

  it('preserves an unset shipment while updating payment on a historical order', async () => {
    const { editor, request } = setup(true, { ...order, ship_status_id: 0 })
    editor.start()
    editor.draft.value.pay_status_id = 2
    editor.draft.value.payment_method = '微信支付'
    await editor.save()
    expect(request).toHaveBeenCalledWith(expect.objectContaining({ ship_status_id: 0, pay_status_id: 2 }))
  })

  it('refuses editing without status permission or for a void order', async () => {
    for (const state of [setup(false), setup(true, { ...order, is_void: true })]) {
      state.editor.start()
      await state.editor.save()
      expect(state.editor.editing.value).toBe(false)
      expect(state.request).not.toHaveBeenCalled()
    }
  })

  it('requires a receipt method for paid and prepayment statuses', async () => {
    const { editor, request } = setup()
    editor.start()
    editor.draft.value.pay_status_id = 2
    await editor.save()
    expect(editor.error.value).toContain('收款方式')
    expect(request).not.toHaveBeenCalled()
    expect(editor.editing.value).toBe(true)
  })

  it('validates prepayment and option identity before sending', async () => {
    const { editor, request } = setup()
    editor.start()
    editor.draft.value.payment_method = '微信支付'
    editor.draft.value.pay_status_id = 4
    editor.draft.value.prepayment_amount = '101'
    await editor.save()
    expect(editor.error.value).toContain('预付款')
    editor.draft.value.prepayment_amount = '30'
    editor.draft.value.ship_status_id = 999
    await editor.save()
    expect(editor.error.value).toContain('发货状态')
    expect(request).not.toHaveBeenCalled()
  })

  it('preserves the entered draft after a failed save', async () => {
    const { editor, request, onSaved } = setup()
    request.mockRejectedValueOnce(new Error('订单已更新，请刷新后重试'))
    editor.start()
    editor.draft.value.ship_status_id = 5
    await editor.save()
    expect(editor.error.value).toContain('订单已更新')
    expect(editor.editing.value).toBe(true)
    expect(editor.draft.value.ship_status_id).toBe(5)
    expect(editor.saving.value).toBe(false)
    expect(onSaved).not.toHaveBeenCalled()
  })

  it('sends only status fields and the original revision, blocks duplicate saves and applies fresh detail', async () => {
    const { editor, request, refresh, onSaved } = setup()
    let finish!: (response: unknown) => void
    request.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    editor.start()
    editor.draft.value.pay_status_id = 2
    editor.draft.value.payment_method = '微信支付'
    editor.draft.value.ship_status_id = 5
    const first = editor.save()
    await editor.save()
    expect(request).toHaveBeenCalledTimes(1)
    expect(request).toHaveBeenCalledWith({
      edit_revision: 'revision-1', pay_status_id: 2, payment_method: '微信支付',
      prepayment_amount: 0, ship_status_id: 5,
    })
    const refreshed = { order: { ...order, pay_status_id: 2, pay_status: '已付款', edit_revision: 'revision-2' } }
    refresh.mockResolvedValue(refreshed)
    finish({ order_id: 42 })
    await first
    expect(onSaved).toHaveBeenCalledWith(refreshed)
    expect(editor.editing.value).toBe(false)
    expect(editor.saving.value).toBe(false)
  })

  it('distinguishes a successful save from a failed detail refresh', async () => {
    const { editor, request, refresh } = setup()
    refresh.mockRejectedValueOnce(new Error('网络中断'))
    editor.start()
    await editor.save()
    expect(editor.editing.value).toBe(false)
    expect(editor.error.value).toContain('状态已保存')
    await editor.save()
    expect(request).toHaveBeenCalledTimes(1)
  })

  it('keeps no-payment sample settlement immutable while allowing shipment updates', async () => {
    const { editor, request } = setup(true, { ...order, pay_status: '无需收款', grand_total: '0.00' })
    editor.start()
    editor.draft.value.ship_status_id = 5
    await editor.save()
    expect(request).toHaveBeenCalledWith(expect.objectContaining({ pay_status_id: 1, ship_status_id: 5 }))
  })

  it('collects actual receipt amounts and voucher for received status while preserving existing values', async () => {
    const { editor, request } = setup(true, { ...order, payment_goods_amount: '80.00', payment_shipping_amount: '20.00', payment_voucher_asset_id: 77 })
    editor.start()
    editor.draft.value.pay_status_id = 6
    editor.draft.value.payment_method = '微信支付'
    expect(editor.draft.value.payment_goods_amount).toBe('80.00')
    expect(editor.draft.value.payment_shipping_amount).toBe('20.00')
    expect(editor.draft.value.payment_voucher_asset_id).toBe(77)
    await editor.save()
    expect(request).toHaveBeenCalledWith(expect.objectContaining({ payment_goods_amount: 80, payment_shipping_amount: 20, payment_voucher_asset_id: 77 }))
  })

  it('requires actual receipt values and voucher before received status can save', async () => {
    const { editor, request } = setup()
    editor.start()
    editor.draft.value.pay_status_id = 6
    editor.draft.value.payment_method = '微信支付'
    await editor.save()
    expect(editor.error.value).toContain('货款')
    editor.draft.value.payment_goods_amount = '100.00'
    await editor.save()
    expect(editor.error.value).toContain('运费')
    editor.draft.value.payment_shipping_amount = '0.00'
    await editor.save()
    expect(editor.error.value).toContain('凭证')
    expect(request).not.toHaveBeenCalled()
  })

  it('rejects fully paid amounts under prepayment status and positive prepayments under unpaid status', async () => {
    const { editor, request } = setup()
    editor.start()
    editor.draft.value.payment_method = '微信支付'
    editor.draft.value.pay_status_id = 4
    editor.draft.value.prepayment_amount = '100'
    await editor.save()
    expect(editor.error.value).toContain('已付款')
    editor.draft.value.pay_status_id = 1
    editor.draft.value.prepayment_amount = '30.001'
    await editor.save()
    expect(editor.error.value).toContain('两位')
    editor.draft.value.prepayment_amount = '30'
    await editor.save()
    expect(editor.error.value).toContain('预付款状态')
    expect(request).not.toHaveBeenCalled()
  })
})
