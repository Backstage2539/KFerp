import { ref } from 'vue'
import type {
  EmployeeOrderDetail, EmployeeOrderDetailResponse, EmployeeOrderStatusOptions, EmployeeOrderStatusPayload,
} from '../api/customerPortal'

export const employeeReceiptMethods = ['微信支付', '支付宝', '银行转账', '对公银行', '现金', 'POS刷卡', '其他']

type StatusDraft = {
  pay_status_id: number
  ship_status_id: number
  payment_method: string
  prepayment_amount: string
  payment_goods_amount: string
  payment_shipping_amount: string
  payment_voucher_asset_id: number
}

type StatusEditorOptions = {
  order: () => EmployeeOrderDetail | undefined
  canUpdate: () => boolean
  options: () => EmployeeOrderStatusOptions
  request: (payload: EmployeeOrderStatusPayload) => Promise<unknown>
  refresh: () => Promise<EmployeeOrderDetailResponse>
  onSaved: (response: EmployeeOrderDetailResponse) => void
  onError?: (cause: unknown) => void
}

export function useEmployeeOrderStatus(options: StatusEditorOptions) {
  const editing = ref(false)
  const saving = ref(false)
  const error = ref('')
  const draft = ref<StatusDraft>({ pay_status_id: 0, ship_status_id: 0, payment_method: '', prepayment_amount: '0.00', payment_goods_amount: '', payment_shipping_amount: '', payment_voucher_asset_id: 0 })
  let revision = ''

  function start() {
    const order = options.order()
    if (!order || !options.canUpdate() || order.is_void || saving.value) return
    draft.value = {
      pay_status_id: Number(order.pay_status_id || 0),
      ship_status_id: Number(order.ship_status_id || 0),
      payment_method: order.payment_method || '',
      prepayment_amount: order.prepayment_amount || '0.00',
      payment_goods_amount: order.payment_goods_amount || '',
      payment_shipping_amount: order.payment_shipping_amount || '',
      payment_voucher_asset_id: Number(order.payment_voucher_asset_id || 0),
    }
    revision = order.edit_revision || ''
    error.value = ''
    editing.value = true
  }

  function cancel() {
    if (saving.value) return
    editing.value = false
    error.value = ''
  }

  async function save() {
    const order = options.order()
    if (!editing.value || saving.value || !order || !options.canUpdate() || order.is_void) return
    error.value = ''
    const statusOptions = options.options()
    const selectedPay = statusOptions.pay_statuses.find(status => status.id === draft.value.pay_status_id)
    const noPayment = order.pay_status === '无需收款'
    if (!selectedPay && !noPayment) { error.value = '请选择有效的付款状态'; return }
    if (draft.value.ship_status_id !== Number(order.ship_status_id || 0) && !statusOptions.ship_statuses.some(status => status.id === draft.value.ship_status_id)) {
      error.value = '请选择有效的发货状态'; return
    }
    const paid = !noPayment && /预付款|已付|已收|已支付/.test(selectedPay?.name || '')
    if (paid && !draft.value.payment_method.trim()) { error.value = '请选择收款方式'; return }
    const prepayment = Number(draft.value.prepayment_amount)
    if (draft.value.prepayment_amount.trim() && !/^\d+(\.\d{1,2})?$/.test(draft.value.prepayment_amount.trim())) {
      error.value = '预付款金额必须为非负金额，最多两位小数'; return
    }
    if (!Number.isFinite(prepayment) || prepayment < 0 || prepayment > Number(order.grand_total || 0)) {
      error.value = '预付款金额须在 0 和订单应收金额之间'; return
    }
    if (!noPayment && selectedPay?.name.includes('预付款') && (prepayment <= 0 || prepayment >= Number(order.grand_total || 0))) {
      error.value = '预付款须大于零且小于应收金额；收齐款项请选择已付款'; return
    }
    if (!noPayment && prepayment > 0 && !selectedPay?.name.includes('预付款') && !/已付款|已收款|已支付/.test(selectedPay?.name || '')) {
      error.value = '已登记预付款，请选择预付款状态或已付款'; return
    }
    const needsReceipt = !noPayment && selectedPay?.name.includes('已收款')
    if (needsReceipt) {
      const shipping = Number(draft.value.payment_shipping_amount)
      const goods = Number(draft.value.payment_goods_amount)
      const zeroGoodsSample = order.is_sample_order && Number(order.total_amount) === 0 && shipping > 0
      if (!/^\d+(\.\d{1,2})?$/.test(draft.value.payment_goods_amount.trim()) || (goods <= 0 && !zeroGoodsSample)) {
        error.value = '请输入有效的实收货款金额，最多两位小数'; return
      }
      if (!/^\d+(\.\d{1,2})?$/.test(draft.value.payment_shipping_amount.trim())) {
        error.value = '请输入有效的实收运费金额，无运费请填 0'; return
      }
      if (!draft.value.payment_voucher_asset_id) { error.value = '请上传收款凭证'; return }
    }
    saving.value = true
    try {
      await options.request({
        edit_revision: revision,
        pay_status_id: noPayment ? Number(order.pay_status_id || 0) : draft.value.pay_status_id,
        ship_status_id: draft.value.ship_status_id,
        payment_method: noPayment ? (order.payment_method || '') : draft.value.payment_method.trim(),
        prepayment_amount: noPayment ? Number(order.prepayment_amount || 0) : prepayment,
        ...(needsReceipt ? {
          payment_goods_amount: Number(draft.value.payment_goods_amount),
          payment_shipping_amount: Number(draft.value.payment_shipping_amount),
          payment_voucher_asset_id: draft.value.payment_voucher_asset_id,
        } : {}),
      })
      editing.value = false
      try {
        options.onSaved(await options.refresh())
      } catch (cause) {
        error.value = '状态已保存，详情刷新失败，请刷新查看最新状态'
        options.onError?.(cause)
      }
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : '状态保存失败，请重试'
      options.onError?.(cause)
    } finally {
      saving.value = false
    }
  }

  return { editing, saving, error, draft, start, cancel, save }
}
