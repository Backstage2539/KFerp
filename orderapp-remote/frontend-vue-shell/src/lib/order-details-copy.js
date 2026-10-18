export function createOrderDetailsCopyAction({ fetchDetails, writeText, context }) {
  let generation = 0
  let busy = false
  return {
    get busy() { return busy },
    invalidate() { generation++; busy = false },
    async copy(id) {
      if (busy) return { status: 'busy' }
      if (!Number.isSafeInteger(Number(id)) || Number(id) <= 0) return { status: 'error', message: '订单编号无效' }
      const run = ++generation
      const scope = context()
      const current = () => generation === run && context() === scope
      busy = true
      try {
        const payload = await fetchDetails(Number(id))
        if (!current()) return { status: 'cancelled' }
        if (Number(payload?.order_id) !== Number(id) || !String(payload?.text || '').trim()) throw new Error('订单详情不完整，请重试')
        try {
          await writeText(payload.text)
          return current() ? { status: 'copied' } : { status: 'cancelled' }
        } catch {
          return current() ? { status: 'manual', text: payload.text } : { status: 'cancelled' }
        }
      } catch (cause) {
        return current() ? { status: 'error', message: cause?.message || '读取订单详情失败，请重试' } : { status: 'cancelled' }
      } finally {
        if (generation === run) busy = false
      }
    },
  }
}
