export type OrderDetailsText = { order_id: number; text: string }
type CopyResult = { status: 'busy' | 'cancelled' | 'copied' | 'error'; message?: string }
type CopyOptions = { fetchDetails: (id: number) => Promise<OrderDetailsText>; writeText: (text: string) => Promise<unknown>; context: () => string }
export function createOrderDetailsCopyAction({ fetchDetails, writeText, context }: CopyOptions) {
  let generation = 0
  let busy = false
  return {
    get busy() { return busy },
    invalidate() { generation++; busy = false },
    async copy(id: number): Promise<CopyResult> {
      if (busy) return { status: 'busy' }
      if (!Number.isSafeInteger(id) || id <= 0) return { status: 'error', message: '订单编号无效' }
      const run = ++generation
      const scope = context()
      const current = () => generation === run && context() === scope
      busy = true
      try {
        const payload = await fetchDetails(id)
        if (!current()) return { status: 'cancelled' }
        if (Number(payload?.order_id) !== id || !String(payload?.text || '').trim()) throw new Error('订单详情不完整，请重试')
        try { await writeText(payload.text) } catch { throw new Error('复制失败，请重试') }
        return current() ? { status: 'copied' } : { status: 'cancelled' }
      } catch (cause) {
        return current() ? { status: 'error', message: cause instanceof Error ? cause.message : '读取订单详情失败，请重试' } : { status: 'cancelled' }
      } finally {
        if (generation === run) busy = false
      }
    },
  }
}
export function writeOrderDetailsClipboard(text: string): Promise<void> {
  return new Promise((resolve, reject) => uni.setClipboardData({ data: text, showToast: false, success: () => resolve(), fail: () => reject(new Error('复制失败，请重试')) }))
}
