import {freightLabel} from './freight.js'
const text = (value) => String(value ?? '').trim()
const legacyTypes = { roasted: '熟豆', roasted_bean: '熟豆', green_bean: '生豆', drip_bag: '挂耳', instant: '速溶咖啡', instant_coffee: '速溶咖啡' }

export function orderTypeLabels(row = {}) {
  const types = Array.isArray(row.product_type_labels) ? row.product_type_labels : text(row.product_kind_summary).split(',').filter(Boolean).map(kind => legacyTypes[text(kind)] || '未分类')
  return [...new Set([text(row.order_type), row.is_sample_order ? '样品' : '', ...types.map(text), ...(!types.length ? ['未分类'] : [])].filter(Boolean))]
}

export function inlineMoney(value) {
  if (value == null || text(value) === '' || !Number.isFinite(Number(value))) return '—'
  return `¥${Number(value).toFixed(2)}`
}

function specLabel(item) {
  const storedName = text(item.bom_spec_name)
  if (storedName) return /^\d+(\.\d+)?$/.test(storedName) ? `${storedName}g` : storedName
  // Legacy orders can still contain a frozen conversion snapshot.
  let source = item.price_source_json
  try { if (typeof source === 'string') source = JSON.parse(source) } catch { source = null }
  const converted = item.spec_conversion || source?.spec_conversion
  if (text(converted?.target_spec_name)) return text(converted.target_spec_name)
  if (text(item.unit_conversion_label)) return text(item.unit_conversion_label)
  const spec = text(item.spec)
  return spec ? (/^\d+(\.\d+)?$/.test(spec) ? `${spec}g` : spec) : '—'
}

export function inlineOrderItems(order = {}) {
  return (order.items || []).map((item, index) => ({
    key: item.item_id || index,
    name: text(item.customer_product_display_name_snapshot) || text(item.product_name_snapshot) || text(item.product_name) || '—',
    spec: specLabel(item), quantity: [text(item.qty) || '—', text(item.unit)].filter(Boolean).join(' '),
    price: inlineMoney(item.unit_price), subtotal: inlineMoney(item.line_total), note: text(item.note),
  }))
}

export function inlineOrderTotals(order = {}) {
  const lines = [{ label: order.freight?'运费状态':'运费', value: order.freight?freightLabel(order.freight):inlineMoney(order.shipping_amount) }]
  if (Number(order.discount_amount)) lines.push({ label: '优惠', value: `−${inlineMoney(order.discount_amount)}` })
  if (Number(order.outsource_total_fee)) lines.push({ label: '委外费用', value: inlineMoney(order.outsource_total_fee) })
  // express_fee is a legacy note/cost field, not an additional customer receivable.
  if (text(order.express_fee) && text(order.express_fee) !== '0' && Number(order.express_fee) !== 0) lines.push({ label: '快递费备注', value: text(order.express_fee) })
  lines.push({ label: '总价', value: inlineMoney(order.grand_total), emphasized: true })
  return lines
}

export function shouldToggleOrderRow(event, selection = '') {
  if (event.defaultPrevented || (event.button != null && event.button !== 0) || text(selection)) return false
  return !event.target?.closest?.('a, button, input, select, textarea, label, summary, [role="button"], [role="link"], [contenteditable]')
}

// The caller supplies reactive state in Vue; plain state also makes races testable.
export function createOrderInlineDetails(fetchDetail, state = {}) {
  let generation = 0, token = 0
  const cache = new Map(), pending = new Map()
  const clear = () => Object.assign(state, { id: 0, detail: null, error: '', loading: false })
  clear()
  async function open(id) {
    const requestToken = ++token, epoch = generation
    Object.assign(state, { id, detail: cache.get(id) || null, error: '', loading: !cache.has(id) })
    if (cache.has(id)) return
    if (!pending.has(id)) {
      const request = Promise.resolve().then(() => fetchDetail(id, epoch)).then(data => {
        if (!data?.edit_data) throw Error('订单详情不可用，请重试')
        if (epoch === generation) cache.set(id, data.edit_data)
        return data.edit_data
      }).finally(() => { if (epoch === generation) pending.delete(id) })
      pending.set(id, request)
    }
    try {
      const detail = await pending.get(id)
      if (epoch === generation && requestToken === token) state.detail = detail
    } catch (err) {
      if (epoch === generation && requestToken === token) state.error = err.message || '加载失败，请重试'
    } finally {
      if (epoch === generation && requestToken === token) state.loading = false
    }
  }
  return {
    state,
    toggle(id) {
      id = Number(id)
      if (!Number.isSafeInteger(id) || id <= 0) return
      if (state.id === id) { ++token; clear(); return }
      return open(id)
    },
    retry() { if (state.id) return open(state.id) },
    reset() { ++generation; ++token; cache.clear(); pending.clear(); clear() },
  }
}
