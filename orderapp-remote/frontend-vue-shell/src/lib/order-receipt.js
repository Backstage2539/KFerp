import { parseCombinedMoneyCents } from './combined-order-payment.js'

export function initialReceiptDateRequired(form, { customerPortal = false, freeSample = false, retainedNew = false, backfill = false, pendingConfirmation = false } = {}) {
  return !customerPortal && !freeSample && !retainedNew && !backfill && !pendingConfirmation && Number(form?.edit_id || 0) === 0 && (Number(form?.prepayment_amount || 0) > 0 || Number(form?.payment_goods_amount || 0) > 0 || Number(form?.payment_shipping_amount || 0) > 0)
}

export function validateInitialReceiptBreakdown({prepayment,goods,shipping,total,freight,status}) {
 if (String(goods ?? '').trim()==='' || String(shipping ?? '').trim()==='') return '请明确填写实收货款和运费分项，没有到账的项目请填 0'
 try {
  const paid=parseCombinedMoneyCents(prepayment),goodsCents=parseCombinedMoneyCents(goods),shippingCents=parseCombinedMoneyCents(shipping),parts=goodsCents+shippingCents,totalCents=parseCombinedMoneyCents(total),freightCents=parseCombinedMoneyCents(freight)
  if (!Number.isSafeInteger(parts) || paid!==parts) return '实收货款与运费分项合计必须等于已收总额'
  if (paid<=0) return '实际收款须大于零'
  if (freightCents>totalCents || shippingCents>freightCents || goodsCents>totalCents-freightCents) return '实收货款或运费不能超过对应应收金额'
  if (/(已付款|已收款|已支付)/.test(String(status || ''))) { if(paid!==totalCents) return '付款状态为已付款时，实收总额必须等于应收总额' }
  else if (!String(status || '').includes('预付款') || paid>=totalCents) return '部分收款请选择预付款状态，实收须小于应收总额'
  return ''
 } catch(err) {return err.message || '实收金额无效'}
}
export function initialReceiptPrepayment({prepayment,goods,shipping,status}) {
 if (!/(已付款|已收款|已支付)/.test(String(status || '')) || Number(prepayment || 0)!==0 || String(goods ?? '').trim()==='' || String(shipping ?? '').trim()==='') return String(prepayment || '0')
 const cents=parseCombinedMoneyCents(goods)+parseCombinedMoneyCents(shipping)
 if (!Number.isSafeInteger(cents)) throw new Error('实际收款金额过大')
 return `${Math.floor(cents/100)}.${String(cents%100).padStart(2,'0')}`
}

export function receiptActorIdentity(actor) {
  const id = Number(actor?.employee_id)
  if (Number.isSafeInteger(id) && id > 0) return `employee-${id}`
  // Basic authentication uses the server's single configured administrator.
  if (actor?.basic_auth_admin === true && String(actor?.name || '').trim()) return 'basic-admin'
  return ''
}
export function receiptPendingKey(identity, orderId) {
  if (!/^(employee-[1-9]\d*|basic-admin)$/.test(String(identity || '')) || !Number.isSafeInteger(Number(orderId)) || Number(orderId) <= 0) throw new Error('无法确认当前账号身份，不能安全登记收款')
  return `kferp.order-receipt.pending.${identity}.order-${Number(orderId)}`
}

export function orderCreatePendingKey(identity) {
  if (!/^employee-[1-9]\d*$/.test(String(identity || ''))) throw new Error('无法确认员工身份，不能安全保存订单')
  return `kferp.order-create.pending.${identity}`
}
export function validPendingOrderCreate(value) {
  return ['/api/order','/api/fulfillment/order'].includes(value?.route) && String(value?.payload?.request_id || '').length >= 16 && Number(value?.payload?.customer_id) > 0
}
export function readPendingOrderCreate(storage, identity) {
  const raw=storage.getItem(orderCreatePendingKey(identity)); if(!raw)return null
  const pending=JSON.parse(raw); if(!validPendingOrderCreate(pending))throw new Error('本机待确认订单记录异常，请联系管理员核对');return pending
}
export function savePendingOrderCreate(storage, identity, pending) {
  if(!validPendingOrderCreate(pending))throw new Error('订单请求标识无效，无法安全保存')
  storage.setItem(orderCreatePendingKey(identity),JSON.stringify(pending))
}

// Only the original response may prove that this request did not commit.
export function receiptKeepPending(status, notCommitted, firstAttempt) {
  return !(firstAttempt === true && notCommitted === true && (status === 400 || status === 409))
}

export function orderReceiptRequest({ orderId, revision, dueCents, amount, date, method, key, voucherID = 0 }) {
  const actual = parseCombinedMoneyCents(amount)
  if (!Number.isSafeInteger(Number(orderId)) || Number(orderId) <= 0 || !String(revision || '').trim()) throw new Error('订单版本无效，请重新读取')
  if (!Number.isSafeInteger(dueCents) || dueCents <= 0 || actual <= 0 || actual > dueCents) throw new Error('本次收款须大于零且不超过待收金额')
  if (!/^\d{4}-\d{2}-\d{2}$/.test(String(date || ''))) throw new Error('请选择实际到账日期')
  if (!String(method || '').trim() || !String(key || '').trim()) throw new Error('请选择收款方式并生成操作标识')
  return { edit_revision: String(revision), expected_due_cents: dueCents, actual_cents: actual, receipt_date: String(date), payment_method: String(method).trim(), payment_voucher_asset_id: Number(voucherID || 0), idempotency_key: String(key) }
}
